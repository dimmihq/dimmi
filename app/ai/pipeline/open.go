/*
|--------------------------------------------------------------------------
| Opening A Run
|--------------------------------------------------------------------------
|
| The first short transaction: load the world, drop the runs that
| should not exist at all, open the run row and pass the gate. Every
| customer message queues its own job, so a burst of messages ends in
| one answer: each job but the newest finds a newer message and steps
| aside, and the newest answers all of them. A retried job finds the
| run that already decided its messages and steps aside too.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
run is one run in flight: its environment, recorder and what the
stages produced.
*/
type run struct {
	env           *env
	rec           *Recorder
	args          Args
	gate          *decision
	c             classification
	sources       []source
	flags         []string
	retrieval     json.RawMessage
	skipRetrieval bool
	noSources     bool
}

/*
open loads the world and opens the run. It returns a finished result
instead of a run when there is nothing to do.
*/
func (r *Runner) open(ctx context.Context, args Args) (*run, *Result, error) {
	var st *run
	var early *Result

	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		e, err := r.loadEnv(ctx, tx, args)
		if errors.Is(err, errTriggerMissing) {
			early = &Result{Outcome: "skipped", SkipReason: "trigger_missing"}
			return nil
		}
		if err != nil {
			return err
		}
		if e == nil {
			early = &Result{Outcome: "skipped", SkipReason: "agent_missing"}
			return nil
		}

		if reason, err := r.stepAside(ctx, tx, e); err != nil || reason != "" {
			early = &Result{Outcome: "skipped", SkipReason: reason}
			return err
		}

		st, early, err = r.start(ctx, tx, e, args)
		return err
	})
	return st, early, err
}

/*
stepAside names why this job should not run at all: its message is
not the newest pending one, it was already answered, or a run already
decided it.
*/
func (r *Runner) stepAside(ctx context.Context, tx *database.Tx, e *env) (string, error) {
	if e.Trigger.AuthorType != string(enums.AuthorTypeContact) {
		return "not_customer_message", nil
	}

	at := slices.IndexFunc(e.Pending, func(m models.Message) bool { return m.ID == e.Trigger.ID })
	switch {
	case at >= 0 && at < len(e.Pending)-1:
		return "debounced", nil
	case at < 0 && e.ReplyAfter == string(enums.AuthorTypeAgent):
		return "already_answered", nil
	case at < 0 && e.ReplyAfter == "":
		return "trigger_stale", nil
	}

	if len(e.Pending) == 0 {
		return "", nil
	}

	done, err := models.New(tx).RunFinishedForMessage(ctx, models.RunFinishedForMessageParams{
		WorkspaceID: e.Conversation.WorkspaceID, ConversationID: &e.Conversation.ID,
		MessageID: e.Pending[len(e.Pending)-1].ID,
	})
	if err != nil || !done {
		return "", err
	}
	return "duplicate", nil
}

/*
start opens the run row and runs the gate. A gate skip finishes the
run right here; a gate pass hands the conversation to the agent.
*/
func (r *Runner) start(ctx context.Context, tx *database.Tx, e *env, args Args) (*run, *Result, error) {
	row, err := r.createRun(ctx, tx, e, args)
	if err != nil {
		return nil, nil, err
	}

	st := &run{env: e, rec: r.newRecorder(row), args: args}
	st.gate = r.gate(e)

	if st.gate != nil && st.gate.outcome == outcomeSkipped {
		if err := r.finishRun(ctx, tx, st, st.gate); err != nil {
			return nil, nil, err
		}
		return nil, st.gate.result(row.ID), nil
	}

	if e.Conversation.AiState == aiStateOff {
		if err := models.New(tx).ActivateConversationAi(ctx, models.ActivateConversationAiParams{
			WorkspaceID: e.Conversation.WorkspaceID, ID: e.Conversation.ID, Now: e.now,
		}); err != nil {
			return nil, nil, err
		}
		e.Conversation.AiState = aiStateActive
	}
	return st, nil, nil
}

/*
createRun writes the run row with the prompt version actually loaded,
its file hashes and a snapshot of the settings that shaped the run.
*/
func (r *Runner) createRun(ctx context.Context, tx *database.Tx, e *env, args Args) (models.AiRun, error) {
	hashes := json.RawMessage("{}")
	if r.Prompts != nil {
		if fp, err := r.Prompts.Fingerprint(e.Version, r.family(e.AnswerKey, e.Agent.AnswerModel),
			[]string{"answer.system.md", "classifier.system.md"}); err == nil {
			hashes, _ = json.Marshal(fp.Hashes)
		}
	}

	snapshot, _ := json.Marshal(map[string]any{
		"answer_model": strv(e.Agent.AnswerModel), "classifier_model": r.classifierModel(e),
		"tone": e.Agent.Tone, "grounding_mode": e.Agent.GroundingMode,
		"require_citations": e.Agent.RequireCitations, "pii_redaction": e.Agent.PiiRedaction,
		"unknown_behavior": e.Agent.UnknownBehavior, "max_ai_replies": e.Agent.MaxAiReplies,
	})

	kind := args.Kind
	if kind == "" {
		kind = "conversation"
	}
	now := r.Clock.Now()
	return models.New(tx).CreateRun(ctx, models.CreateRunParams{
		WorkspaceID: e.Workspace.ID, AgentID: e.Agent.ID, Kind: kind,
		ConversationID: &e.Conversation.ID, TriggerMessageID: &e.Trigger.ID,
		IncludedMessageIds: e.included(), ActorType: new("contact"), ActorID: &e.Conversation.ContactID,
		ChannelType: new(e.ChannelType), Status: "running",
		PromptVersion: e.Version, PromptHashes: hashes, ConfigSnapshot: snapshot,
		StartedAt: &now, Now: now,
	})
}
