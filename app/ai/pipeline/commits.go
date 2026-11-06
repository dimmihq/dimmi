/*
|--------------------------------------------------------------------------
| Pipeline Commits
|--------------------------------------------------------------------------
|
| The commits write the run's outcome: the reply, the honest
| no-answer, the handoff line plus its summary, and the deterministic
| fast paths. Each takes a last look for anything newer before it
| writes — a superseded run stays silent.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
commitAnswer writes the reply and closes the run.
*/
func (r *Runner) commitAnswer(ctx context.Context, f *flow, a *answer) (*decision, error) {
	if reason := superseded(f.env); reason != "" {
		return &decision{name: "superseded", skipReason: reason}, nil
	}

	text := postProcess(f.env, a.Text, f.found)
	if text == "" {
		return r.commitUnknown(ctx, f)
	}

	writer := r.writer(f.env)
	msg, _, err := writer.Append(ctx, f.env.Conversation, conversations.NewMessage{
		Type: "message", Body: text, BodyFormat: new("markdown"),
		Metadata: aiMeta(f.recorder, f.found, f.c),
	})
	if err != nil {
		return nil, fmt.Errorf("write answer: %w", err)
	}

	if err := models.New(f.env.tx).BumpConversationAiAnswer(ctx, models.BumpConversationAiAnswerParams{
		WorkspaceID: f.env.Workspace.ID, ID: f.env.Conversation.ID,
		CostMicros: f.recorder.Cost(), Language: nilable(f.c.Language), Now: r.Clock.Now(),
	}); err != nil {
		return nil, fmt.Errorf("bump ai answer: %w", err)
	}

	return &decision{name: "answered", AnswerID: &msg.ID}, nil
}

/*
commitUnknown writes the honest no-answer and, when the setting asks,
hands off.
*/
func (r *Runner) commitUnknown(ctx context.Context, f *flow) (*decision, error) {
	if reason := superseded(f.env); reason != "" {
		return &decision{name: "superseded", skipReason: reason}, nil
	}

	if f.env.Agent.UnknownBehavior == "handoff" {
		return r.commitHandoff(ctx, f, "unknown")
	}

	line := messageFor(f.env, "unknown_offer")
	msg, _, err := r.writer(f.env).Append(ctx, f.env.Conversation, conversations.NewMessage{
		Type: "message", Body: line, Metadata: aiMeta(f.recorder, nil, f.c),
	})
	if err != nil {
		return nil, fmt.Errorf("write unknown: %w", err)
	}

	return &decision{name: "unknown", AnswerID: &msg.ID}, nil
}

/*
commitHandoff writes the handoff line, generates the teammate brief
and marks the conversation.
*/
func (r *Runner) commitHandoff(ctx context.Context, f *flow, reason string) (*decision, error) {
	if s := superseded(f.env); s != "" {
		return &decision{name: "superseded", skipReason: s}, nil
	}

	line := messageFor(f.env, "handoff_line")
	if _, _, err := r.writer(f.env).Append(ctx, f.env.Conversation, conversations.NewMessage{
		Type: "message", Body: line, Metadata: aiMeta(f.recorder, nil, nil),
	}); err != nil {
		return nil, fmt.Errorf("write handoff line: %w", err)
	}

	summary := r.handoffSummary(ctx, f, reason)

	if err := models.New(f.env.tx).HandoffConversationAi(ctx, models.HandoffConversationAiParams{
		WorkspaceID: f.env.Workspace.ID, ID: f.env.Conversation.ID,
		Reason: nilable(reason), Now: nilableTime(r.Clock.Now()),
	}); err != nil {
		return nil, fmt.Errorf("handoff conversation: %w", err)
	}

	if r.Events != nil {
		_ = r.Events.Dispatch(ctx, f.env.tx, appevents.AiHandoffRequested{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: f.env.Workspace.ID,
				OccurredAt: r.Clock.Now(), Actor: appevents.Actor{Type: "agent", ID: f.env.Agent.ID},
			},
			ConversationRef: appevents.ConversationRef{ConversationID: f.env.Conversation.ID},
			Reason:          reason,
			Summary:         summary,
		})
	}

	return &decision{name: "handoff", handoffReason: reason}, nil
}

/*
commitFastPath writes the deterministic answer for a fast path.
*/
func (r *Runner) commitFastPath(ctx context.Context, f *flow, outcome, reason string) (*decision, error) {
	if outcome == "handoff" {
		return r.commitHandoff(ctx, f, reason)
	}
	return &decision{name: "silent"}, nil
}

/*
superseded names what changed while the run worked, if anything did:
a newer customer word or a human reply.
*/
func superseded(env *env) string {
	for _, m := range env.Messages {
		if m.ID.String() > env.Trigger.ID.String() && m.AuthorType == "contact" {
			return "newer_customer_message"
		}
		if m.ID.String() > env.Trigger.ID.String() && m.AuthorType == "member" {
			return "human_replied"
		}
	}
	return ""
}

/*
handoffSummary asks the answer model for the one-paragraph brief a
teammate reads first: what the customer wants, what was said, why it
was handed over. Its failure degrades to a reason-only handoff.
*/
func (r *Runner) handoffSummary(ctx context.Context, f *flow, reason string) string {
	llm, err := r.driverFor(ctx, f.env.AnswerKey, f.env.Agent.AnswerModel)
	if err != nil {
		return ""
	}
	model := strv(f.env.Agent.AnswerModel)

	started := r.Clock.Now()
	res, err := llm.Chat(ctx, contracts.ChatRequest{
		Model: model,
		Messages: []contracts.ChatMessage{
			{
				Role: contracts.ChatRoleUser,
				Parts: []contracts.ContentPart{{
					Text: "Summarize this conversation for a teammate in two or three sentences: " +
						"what the customer wants, what the assistant already said, and that it was " +
						"handed over for: " + reason + ".\n\n" + transcriptForLLM(f.env, 10),
				}},
			},
		},
		MaxOutputTokens: 300,
	})
	if err != nil {
		r.stepErr(ctx, f, stepError{kind: enums.AIRunStepKindSummarize, model: model, started: started, err: err})
		return ""
	}

	text := strings.TrimSpace(res.Text)
	if _, err := f.recorder.Step(ctx, f.env.tx, StepInput{
		Kind: enums.AIRunStepKindSummarize, Provider: string(llm.Provider()),
		ProviderKeyID: keyID(f.env.AnswerKey), Model: model, Status: "ok",
		Usage: res.Usage, Latency: r.Clock.Now().Sub(started),
		StartedAt: started, CompletedAt: r.Clock.Now(),
		ResponsePayload: r.payload(f.env, text),
	}); err != nil {
		return ""
	}
	return text
}

/*
aiMeta is the message metadata every AI reply carries.
*/
func aiMeta(recorder *Recorder, found *retrieval, _ *classification) []byte {
	meta := map[string]any{"ai": map[string]any{
		"run_id":    recorder.Run.ID.String(),
		"generated": true,
	}}
	if found != nil && len(found.Chunks) > 0 {
		var sources []map[string]string
		for _, ch := range found.Chunks {
			sources = append(sources, map[string]string{
				"title": ch.Title, "url": ch.URL,
			})
		}
		if ai, ok := meta["ai"].(map[string]any); ok {
			ai["sources"] = sources
		}
	}
	out, err := json.Marshal(meta)
	if err != nil {
		return nil
	}
	return out
}
