/*
|--------------------------------------------------------------------------
| Pipeline Stages
|--------------------------------------------------------------------------
|
| Each stage does one thing and records one or more steps: classify
| reads intent and language on the cheap model, retrieve asks the
| knowledge index, generate writes the answer on the strong model,
| and the commits write the outcome transactionally, after a last
| look for anything newer.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/linkguard"
	"github.com/dimmihq/dimmi/app/support/pii"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
classification is the run's decision state: no model fills it yet —
the dedicated decisions model is out of scope — so the pipeline
defaults it from the trigger and the agent's settings.
*/
type classification struct {
	Intent         string `json:"intent"`
	Language       string `json:"language"`
	QueryRewritten string `json:"query_rewritten"`
	Safety         string `json:"safety"`
	Summary        string `json:"summary"`
	handoff        string
	silent         bool
}

/*
retrieval is what the index returned plus the answerability verdict.
*/
type retrieval struct {
	Chunks     []contracts.KnowledgeChunk
	Query      string
	Answerable bool
}

/*
answer is the generated reply.
*/
type answer struct {
	Text       string
	Unknown    bool
	Handoff    string
	handoff    string
	unknown    bool
	FirstToken time.Duration
	Latency    time.Duration
}

/*
retrieve asks the knowledge index and decides answerability.
*/
func (r *Runner) retrieve(ctx context.Context, env *env, recorder *Recorder, c *classification) (*retrieval, error) {
	query := c.QueryRewritten
	if query == "" {
		query = triggerText(env.Trigger)
	}

	started := r.Clock.Now()
	res, err := r.Knowledge.Search(ctx, env.Workspace.ID, pii.Redact(query, env.Agent.PiiRedaction),
		contracts.KnowledgeSearchOptions{
			Limit:      6,
			Visibility: []contracts.KnowledgeVisibility{contracts.KnowledgeVisibilityPublic},
			Surface:    "agent",
		})
	if err != nil {
		r.stepErr(ctx, env, recorder, enums.AIRunStepKindRetrieve, "", started, err)
		return nil, fmt.Errorf("retrieve: %w", err)
	}

	chunks := res.Chunks
	answerable := len(chunks) > 0
	if f8, err := env.Agent.MinSimilarity.Float64Value(); err == nil && f8.Valid {
		min := f8.Float64
		answerable = false
		for _, ch := range chunks {
			if ch.Similarity == nil || *ch.Similarity >= min {
				answerable = true
				break
			}
		}
	}

	payload, _ := json.Marshal(chunks)
	if _, err := recorder.Step(ctx, env.tx, StepInput{
		Kind: enums.AIRunStepKindRetrieve, Status: "ok",
		Latency: r.Clock.Now().Sub(started), StartedAt: started, CompletedAt: r.Clock.Now(),
		RequestPayload: r.payload(env, query), ResponsePayload: r.payload(env, string(payload)),
	}); err != nil {
		return nil, fmt.Errorf("record retrieve step: %w", err)
	}

	return &retrieval{Chunks: chunks, Query: query, Answerable: answerable}, nil
}

/*
generate writes the answer on the answer model, streaming into a
buffer; the stream's first token is timed.
*/
func (r *Runner) generate(ctx context.Context, env *env, recorder *Recorder, c *classification, found *retrieval) (*answer, error) {
	llm, err := r.driverFor(ctx, env.AnswerKey, env.Agent.AnswerModel)
	if err != nil {
		return nil, err
	}
	model := strv(env.Agent.AnswerModel)

	system, sources, err := r.answerPrompt(env, c, found)
	if err != nil {
		return nil, err
	}

	started := r.Clock.Now()
	var buf strings.Builder
	var firstToken time.Duration
	var usage contracts.LLMUsage
	var stopReason contracts.StopReason
	var calls []contracts.ToolCall

	stream, err := llm.Stream(ctx, contracts.ChatRequest{
		Model:  model,
		System: []contracts.SystemBlock{{Text: system, Cacheable: true}},
		Messages: []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: sources}}},
		},
		MaxOutputTokens: int(env.Agent.MaxOutputTokens),
		CacheHint:       true,
	})
	if err != nil {
		r.stepErr(ctx, env, recorder, enums.AIRunStepKindGenerate, model, started, err)
		return nil, fmt.Errorf("generate: %w", err)
	}
	defer stream.Close()

	for {
		ev, err := stream.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			r.stepErr(ctx, env, recorder, enums.AIRunStepKindGenerate, model, started, err)
			return nil, fmt.Errorf("generate stream: %w", err)
		}
		switch ev.Kind {
		case contracts.StreamEventTextDelta:
			if firstToken == 0 {
				firstToken = r.Clock.Now().Sub(started)
			}
			buf.WriteString(ev.Text)
		case contracts.StreamEventToolCallDone:
			if ev.ToolCall != nil {
				calls = append(calls, *ev.ToolCall)
			}
		case contracts.StreamEventUsage:
			if ev.Usage != nil {
				usage = *ev.Usage
			}
		case contracts.StreamEventDone:
			stopReason = ev.StopReason
		}
	}

	latency := r.Clock.Now().Sub(started)
	text := strings.TrimSpace(buf.String())

	if _, err := recorder.Step(ctx, env.tx, StepInput{
		Kind: enums.AIRunStepKindGenerate, Provider: string(llm.Provider()),
		ProviderKeyID: keyID(env.AnswerKey), Model: model, Status: "ok",
		Usage: usage, Latency: latency, FirstToken: firstToken,
		StartedAt: started, CompletedAt: r.Clock.Now(),
		RequestPayload: r.payload(env, sources), ResponsePayload: r.payload(env, text),
	}); err != nil {
		return nil, fmt.Errorf("record generate step: %w", err)
	}

	out := &answer{Text: text, FirstToken: firstToken, Latency: latency}
	_ = stopReason
	_ = calls

	if saysUnknown(text) {
		out.unknown = true
	}

	return out, nil
}

/*
answerPrompt renders the family prompt and the numbered source list.
*/
func (r *Runner) answerPrompt(env *env, c *classification, found *retrieval) (string, string, error) {
	vars := map[string]string{
		"agent_name":   env.Agent.Name,
		"product_name": product(env),
		"guidance":     env.Agent.Guidance,
		"rules":        standingRules(env),
		"tone":         env.Agent.Tone,
		"language":     languageFor(env, c),
	}

	system, err := r.prompt(env, "answer.system.md", vars)
	if err != nil {
		return "", "", err
	}

	var b strings.Builder
	b.WriteString("SOURCES\n")
	for i, ch := range found.Chunks {
		fmt.Fprintf(&b, "[%d] %s\n%s\nURL: %s\n\n", i+1, ch.Title, ch.Text, ch.URL)
	}
	fmt.Fprintf(&b, "TRANSCRIPT\n%s", transcriptForLLM(env, 12))

	return system, b.String(), nil
}

/*
commitAnswer writes the reply and closes the run.
*/
func (r *Runner) commitAnswer(ctx context.Context, env *env, recorder *Recorder, c *classification, found *retrieval, a *answer) (*decision, error) {
	if reason := superseded(env); reason != "" {
		return &decision{name: "superseded", skipReason: reason}, nil
	}

	text := postProcess(env, a.Text, found)
	if text == "" {
		return r.commitUnknown(ctx, env, recorder, c)
	}

	writer := r.writer(env)
	msg, _, err := writer.Append(ctx, env.Conversation, conversations.NewMessage{
		Type: "message", Body: text, BodyFormat: str("markdown"),
		Metadata: aiMeta(recorder, found, c),
	})
	if err != nil {
		return nil, fmt.Errorf("write answer: %w", err)
	}

	if err := models.New(env.tx).BumpConversationAiAnswer(ctx, models.BumpConversationAiAnswerParams{
		WorkspaceID: env.Workspace.ID, ID: env.Conversation.ID,
		CostMicros: recorder.Cost(), Language: nilable(c.Language), Now: r.Clock.Now(),
	}); err != nil {
		return nil, fmt.Errorf("bump ai answer: %w", err)
	}

	return &decision{name: "answered", AnswerID: &msg.ID}, nil
}

/*
commitUnknown writes the honest no-answer and, when the setting asks,
hands off.
*/
func (r *Runner) commitUnknown(ctx context.Context, env *env, recorder *Recorder, c *classification) (*decision, error) {
	if reason := superseded(env); reason != "" {
		return &decision{name: "superseded", skipReason: reason}, nil
	}

	if env.Agent.UnknownBehavior == "handoff" {
		return r.commitHandoff(ctx, env, recorder, "unknown")
	}

	line := messageFor(env, "unknown_offer")
	writer := r.writer(env)
	msg, _, err := writer.Append(ctx, env.Conversation, conversations.NewMessage{
		Type: "message", Body: line, Metadata: aiMeta(recorder, nil, c),
	})
	if err != nil {
		return nil, fmt.Errorf("write unknown: %w", err)
	}

	return &decision{name: "unknown", AnswerID: &msg.ID}, nil
}

/*
commitHandoff writes the handoff line and marks the conversation.
*/
func (r *Runner) commitHandoff(ctx context.Context, env *env, recorder *Recorder, reason string) (*decision, error) {
	if s := superseded(env); s != "" {
		return &decision{name: "superseded", skipReason: s}, nil
	}

	line := messageFor(env, "handoff_line")
	writer := r.writer(env)
	_, _, err := writer.Append(ctx, env.Conversation, conversations.NewMessage{
		Type: "message", Body: line, Metadata: aiMeta(recorder, nil, nil),
	})
	if err != nil {
		return nil, fmt.Errorf("write handoff line: %w", err)
	}

	summary := r.handoffSummary(ctx, env, recorder, reason)

	if err := models.New(env.tx).HandoffConversationAi(ctx, models.HandoffConversationAiParams{
		WorkspaceID: env.Workspace.ID, ID: env.Conversation.ID,
		Reason: nilable(reason), Now: nilableTime(r.Clock.Now()),
	}); err != nil {
		return nil, fmt.Errorf("handoff conversation: %w", err)
	}

	if r.Events != nil {
		_ = r.Events.Dispatch(ctx, env.tx, appevents.AiHandoffRequested{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: env.Workspace.ID,
				OccurredAt: r.Clock.Now(), Actor: appevents.Actor{Type: "agent", ID: env.Agent.ID},
			},
			ConversationRef: appevents.ConversationRef{ConversationID: env.Conversation.ID},
			Reason:          reason,
			Summary:         summary,
		})
	}

	return &decision{name: "handoff", handoffReason: reason}, nil
}

/*
commitFastPath writes the deterministic answer for a fast path.
*/
func (r *Runner) commitFastPath(ctx context.Context, env *env, recorder *Recorder, outcome, reason string) (*decision, error) {
	if outcome == "handoff" {
		return r.commitHandoff(ctx, env, recorder, reason)
	}
	return &decision{name: "silent"}, nil
}

/*
superseded names what changed while the run worked, if anything did:
a newer customer word, a human reply, a handoff or a close.
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
func (r *Runner) handoffSummary(ctx context.Context, env *env, recorder *Recorder, reason string) string {
	llm, err := r.driverFor(ctx, env.AnswerKey, env.Agent.AnswerModel)
	if err != nil {
		return ""
	}
	model := strv(env.Agent.AnswerModel)

	started := r.Clock.Now()
	res, err := llm.Chat(ctx, contracts.ChatRequest{
		Model: model,
		Messages: []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text:
				"Summarize this conversation for a teammate in two or three sentences: " +
				"what the customer wants, what the assistant already said, and that it was " +
				"handed over for: " + reason + ".\n\n" + transcriptForLLM(env, 10)}}},
		},
		MaxOutputTokens: 300,
	})
	if err != nil {
		r.stepErr(ctx, env, recorder, enums.AIRunStepKindSummarize, model, started, err)
		return ""
	}

	text := strings.TrimSpace(res.Text)
	if _, err := recorder.Step(ctx, env.tx, StepInput{
		Kind: enums.AIRunStepKindSummarize, Provider: string(llm.Provider()),
		ProviderKeyID: keyID(env.AnswerKey), Model: model, Status: "ok",
		Usage: res.Usage, Latency: r.Clock.Now().Sub(started),
		StartedAt: started, CompletedAt: r.Clock.Now(),
		ResponsePayload: r.payload(env, text),
	}); err != nil {
		return ""
	}
	return text
}

/*
postProcess applies the deterministic rewrites: link allowlist and
the citation requirement.
*/
func postProcess(env *env, text string, found *retrieval) string {
	allowed := linkHosts(env)
	text = linkguard.Filter(text, allowed)
	return text
}

/*
linkHosts builds the allowlist from the index's public hosts plus the
agent's own.
*/
func linkHosts(env *env) []string {
	return append([]string{}, env.Agent.LinkAllowlist...)
}

/*
aiMeta is the message metadata every AI reply carries.
*/
func aiMeta(recorder *Recorder, found *retrieval, c *classification) []byte {
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
		meta["ai"].(map[string]any)["sources"] = sources
	}
	out, _ := json.Marshal(meta)
	return out
}

/*
writer builds the writer acting as the agent.
*/
func (r *Runner) writer(env *env) *conversations.Writer {
	return &conversations.Writer{
		Q: models.New(env.tx), Tx: env.tx, Events: r.Events,
		Now:   r.Clock.Now(),
		Actor: conversations.Actor{Type: "agent", ID: env.Agent.ID, Name: env.Agent.Name},
	}
}

/*
driverFor resolves a role's driver.
*/
func (r *Runner) driverFor(ctx context.Context, key *models.AiProviderKey, model *string) (llm, error) {
	if key == nil || model == nil {
		return nil, fmt.Errorf("model not configured")
	}
	return r.LLM.ForKey(ctx, *key)
}

/*
llm is the contracts interface.
*/
type llm = contracts.LLM

/*
prompt renders one file for the agent's family.
*/
func (r *Runner) prompt(env *env, name string, vars map[string]string) (string, error) {
	if r.Prompts == nil {
		return "", nil
	}
	family := enums.AIModelFamilyOpenWeight
	if env.Agent.AnswerModel != nil {
		family = familyForKey(env.AnswerKey, *env.Agent.AnswerModel)
	}
	file, err := r.Prompts.RenderFile(family, name, vars)
	if err != nil {
		return "", err
	}
	return file.Content, nil
}

/*
stepErr records a failed step.
*/
func (r *Runner) stepErr(ctx context.Context, env *env, recorder *Recorder, kind enums.AIRunStepKind, model string, started time.Time, err error) {
	status := "error"
	code := ""
	httpStatus := 0
	if llmErr, ok := err.(*contracts.LLMError); ok {
		code = string(llmErr.Kind)
		httpStatus = llmErr.HTTPStatus
	}
	_, _ = recorder.Step(ctx, env.tx, StepInput{
		Kind: kind, Model: model, Status: status, ErrorKind: code, HTTPStatus: httpStatus,
		Latency: r.Clock.Now().Sub(started), StartedAt: started, CompletedAt: r.Clock.Now(),
	})
}

/*
payload trims a stored request/response payload.
*/
func (r *Runner) payload(env *env, text string) []byte {
	if len(text) > 64_000 {
		text = text[:64_000]
	}
	out, _ := json.Marshal(text)
	return out
}

/*
keyID maps a key row to its id pointer.
*/
func keyID(key *models.AiProviderKey) *ids.UUID {
	if key == nil {
		return nil
	}
	return &key.ID
}

/*
product names the product the agent speaks for.
*/
func product(env *env) string {
	if env.Agent.ProductName != nil && *env.Agent.ProductName != "" {
		return *env.Agent.ProductName
	}
	return env.Workspace.Name
}

/*
standingRules is the workspace's rules block.
*/
func standingRules(env *env) string {
	return "Always offer a real person when the customer asks or when you cannot answer."
}

/*
languageFor picks the reply language.
*/
func languageFor(env *env, c *classification) string {
	if c != nil && c.Language != "" {
		return c.Language
	}
	return env.Agent.DefaultLanguage
}

/*
messageFor reads one configurable line, with its default.
*/
func messageFor(env *env, key string) string {
	defaults := map[string]string{
		"unknown_offer": "I don't have an answer for that. Want me to bring in someone from the team?",
		"handoff_line":  "I'm bringing in someone from the team.",
	}
	var msgs map[string]any
	if err := json.Unmarshal(env.Agent.Messages, &msgs); err == nil {
		if v, ok := msgs[key].(string); ok && v != "" {
			return v
		}
	}
	return defaults[key]
}

/*
saysUnknown reports whether the answer declines to answer.
*/
func saysUnknown(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "don't have an answer") ||
		strings.Contains(lower, "do not have an answer") ||
		strings.Contains(lower, "i don't know") ||
		strings.Contains(lower, "cannot find")
}

/*
transcriptForLLM renders the tail of the conversation.
*/
func transcriptForLLM(env *env, max int) string {
	msgs := env.Messages
	if len(msgs) > max {
		msgs = msgs[len(msgs)-max:]
	}

	var b strings.Builder
	for _, m := range msgs {
		who := "Customer"
		switch m.AuthorType {
		case "member":
			who = "Teammate"
		case "agent":
			who = env.Agent.Name
		case "system":
			who = "System"
		}
		text := triggerText(m)
		text = pii.Redact(text, env.Agent.PiiRedaction)
		if len(text) > 2000 {
			text = text[:2000]
		}
		fmt.Fprintf(&b, "%s: %s\n\n", who, text)
	}
	return b.String()
}

/*
jsonObject extracts the first JSON object from a reply.
*/
func jsonObject(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		return "{}"
	}
	return text[start : end+1]
}

/*
contains reports membership.
*/
func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

var _ = aimodels.FamilyOf
var _ = database.DB{}
