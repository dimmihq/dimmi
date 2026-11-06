/*
|--------------------------------------------------------------------------
| Pipeline Runner
|--------------------------------------------------------------------------
|
| One customer message in, one decision out. The runner loads the
| world, opens a run row, walks the stages — gate, fast path,
| classify, retrieve, generate, post-process — and only then commits
| what was written, after a last check nothing newer landed. Every
| step is traced; a skipped or failed run is a row, not a lost task.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	fevents "github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aimodels"
	"github.com/dimmihq/dimmi/app/support/aiprices"
	"github.com/dimmihq/dimmi/app/support/prompts"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/jackc/pgx/v5/pgtype"
)

/*
Runner runs the answer pipeline for one conversation.
*/
type Runner struct {
	DB       *database.DB
	Clock    clock.Clock
	LLM      *managers.LLMManager
	Knowledge contracts.Knowledge
	Prompts  *prompts.Loader
	Prices   *aiprices.Table
	Config   pipelineConfig
	Events   *fevents.Dispatcher
}

/*
pipelineConfig is the slice of config the pipeline reads.
*/
type pipelineConfig interface {
	DebounceMS() int
	MaxContextTokens() int
	StorePayloads() bool
}

/*
Args is what one run needs.
*/
type Args struct {
	WorkspaceID    ids.UUID
	ConversationID ids.UUID
	TriggerMessageID ids.UUID
	Kind           string
	QueuedAt       time.Time
}

/*
Result is how the run ended.
*/
type Result struct {
	Outcome       string
	SkipReason    string
	HandoffReason string
	AnswerID      *ids.UUID
	RunID         ids.UUID
}

/*
Run answers one trigger message, returning the outcome.
*/
func (r *Runner) Run(ctx context.Context, args Args) (*Result, error) {
	if r.Clock == nil {
		r.Clock = clock.System()
	}

	queueWait := int(r.Clock.Now().Sub(args.QueuedAt).Milliseconds())
	if queueWait < 0 {
		queueWait = 0
	}

	var result *Result

	err := r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		env, err := r.loadEnv(ctx, tx, args.WorkspaceID, args.ConversationID)
		if err != nil {
			return err
		}

		if env.Agent == nil {
			result = &Result{Outcome: "skipped", SkipReason: "agent_missing"}
			return nil
		}

		run, err := r.startRun(ctx, tx, env, args, queueWait)
		if err != nil {
			return err
		}
		recorder := newRecorder(ctx, r.DB, r.Clock, r.Prices, run)

		outcome, err := r.execute(ctx, env, recorder, args)
		if err != nil {
			if ferr := recorder.Finish(ctx, tx, FinishInput{
				Status: "failed", Outcome: "error",
				ErrorCode: errorCode(err), ErrorMessage: err.Error(), QueueWaitMS: queueWait,
			}); ferr != nil {
				return fmt.Errorf("finish run: %w", ferr)
			}
			result = &Result{Outcome: "error", RunID: run.ID}
			return nil
		}

		if outcome.AnswerID != nil {
			_ = recorder.Finish(ctx, tx, outcome.finish(queueWait))
			result = &Result{
				Outcome: outcome.name, SkipReason: outcome.skipReason,
				HandoffReason: outcome.handoffReason, AnswerID: outcome.AnswerID, RunID: run.ID,
			}
			return nil
		}

		_ = recorder.Finish(ctx, tx, outcome.finish(queueWait))
		result = &Result{
			Outcome: outcome.name, SkipReason: outcome.skipReason,
			HandoffReason: outcome.handoffReason, RunID: run.ID,
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pipeline: %w", err)
	}

	return result, nil
}

/*
startRun opens the run row with the prompt fingerprint and a config
snapshot.
*/
func (r *Runner) startRun(ctx context.Context, tx *database.Tx, env *env, args Args, queueWait int) (models.AiRun, error) {
	family := enums.AIModelFamilyOpenWeight
	if env.Agent.AnswerModel != nil {
		family = familyForKey(env.AnswerKey, *env.Agent.AnswerModel)
	}

	fp := map[string]string{}
	version := "v1"
	if r.Prompts != nil {
		if env.Agent.PromptVersion != nil && *env.Agent.PromptVersion != "" {
			version = *env.Agent.PromptVersion
		}
		if f, err := r.Prompts.Fingerprint(family, []string{"answer.system.md"}); err == nil {
			fp = f.Hashes
		}
	}

	snapshot, _ := json.Marshal(map[string]any{
		"model_family": family, "answer_model": strv(env.Agent.AnswerModel),
		"classifier_model": strv(env.Agent.ClassifierModel), "tone": env.Agent.Tone,
		"require_citations": env.Agent.RequireCitations, "pii_redaction": env.Agent.PiiRedaction,
	})
	hashes, _ := json.Marshal(fp)

	now := r.Clock.Now()
	return models.New(tx).CreateRun(ctx, models.CreateRunParams{
		WorkspaceID: env.Workspace.ID, AgentID: env.Agent.ID, Kind: args.Kind,
		ConversationID: &args.ConversationID, TriggerMessageID: &args.TriggerMessageID,
		IncludedMessageIds: messageIDs(env.Messages), ActorType: str("contact"),
		ChannelType: str(env.ChannelType), Status: "running",
		PromptVersion: version, PromptHashes: hashes, ConfigSnapshot: snapshot,
		StartedAt: &now, Now: now,
	})
}

/*
execute walks the stages.
*/
func (r *Runner) execute(ctx context.Context, env *env, recorder *Recorder, args Args) (*decision, error) {
	if s := r.gate(ctx, env); s != nil {
		return &decision{name: "skipped", skipReason: s.Reason}, nil
	}

	text := triggerText(env.Trigger)

	if outcome, reason := fastPath(text, env.pendingConfirm); outcome != "" {
		return r.commitFastPath(ctx, env, recorder, outcome, reason)
	}

	classified := &classification{
		Intent: "question", Language: env.Agent.DefaultLanguage,
		QueryRewritten: triggerText(env.Trigger),
	}

	found, err := r.retrieve(ctx, env, recorder, classified)
	if err != nil {
		return nil, err
	}

	if !found.Answerable {
		return r.commitUnknown(ctx, env, recorder, classified)
	}

	answer, err := r.generate(ctx, env, recorder, classified, found)
	if err != nil {
		return nil, err
	}

	if answer.handoff != "" {
		return r.commitHandoff(ctx, env, recorder, answer.handoff)
	}
	if answer.unknown {
		return r.commitUnknown(ctx, env, recorder, classified)
	}

	return r.commitAnswer(ctx, env, recorder, classified, found, answer)
}

/*
decision is what a stage resolved to.
*/
type decision struct {
	name          string
	skipReason    string
	handoffReason string
	AnswerID      *ids.UUID
}

/*
finish maps a decision to the run's finish input.
*/
func (d *decision) finish(queueWait int) FinishInput {
	out := FinishInput{Status: "completed", QueueWaitMS: queueWait}
	switch {
	case d.skipReason != "":
		out.Outcome = "skipped"
		out.SkipReason = d.skipReason
		out.Status = "skipped"
	case d.name == "":
		out.Outcome = "silent"
	default:
		out.Outcome = d.name
	}
	out.HandoffReason = d.handoffReason
	out.AnswerMessageID = d.AnswerID
	return out
}

/*
triggerText is the message's plain body.
*/
func triggerText(m models.Message) string {
	if m.Body != nil {
		return *m.Body
	}
	return ""
}

/*
messageIDs lists the transcript ids included.
*/
func messageIDs(msgs []models.Message) []ids.UUID {
	out := make([]ids.UUID, len(msgs))
	for i, m := range msgs {
		out[i] = m.ID
	}
	return out
}

/*
strv reads a nullable string.
*/
func strv(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

/*
str is a string pointer.
*/
func str(s string) *string { return &s }

/*
errorCode folds an error to a short code.
*/
func errorCode(err error) string {
	return "pipeline_error"
}

/*
recordSpend adds the run's cost and counts to the month's tally row.
*/
func (r *Runner) recordSpend(ctx context.Context, tx *database.Tx, env *env, recorder *Recorder, outcome string) {
	now := r.Clock.Now().UTC()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	nowT := r.Clock.Now()

	counts := map[string]int32{}
	counts[outcome] = 1

	_, err := models.New(tx).AddSpend(ctx, models.AddSpendParams{
		WorkspaceID: env.Workspace.ID, PeriodStart: pgtype.Date{Time: start, Valid: true},
		CostMicros: recorder.Cost(), InputTokens: recorder.tokens.input,
		CachedInputTokens: recorder.tokens.cached, OutputTokens: recorder.tokens.output,
		RunsCount: 1,
		AnsweredCount: counts["answered"], UnknownCount: counts["unknown"],
		HandoffCount: counts["handoff"], SilentCount: counts["silent"], Now: nowT,
	})
	if err != nil {
		return
	}
}

/*
familyForKey resolves a model's family from its connection.
*/
func familyForKey(key *models.AiProviderKey, model string) enums.AIModelFamily {
	provider := enums.AIProviderDimmi
	if key != nil {
		provider = enums.AIProvider(key.Provider)
	}
	return aimodels.FamilyOf(provider, model)
}


