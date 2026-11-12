/*
|--------------------------------------------------------------------------
| Cheap Verdicts
|--------------------------------------------------------------------------
|
| Some questions only need a word back, and the workspace's cheapest
| model answers them: the agent's classifier model on its connection,
| or the answer model when no classifier is set. Each question is its
| own run, of its own kind, so its tokens and cost show beside the
| agent's and count on the month's spend in the same transaction.
|
| Every answer is one JSON object a schema forces on the model, never
| free text read for its meaning. A workspace without a model that keeps
| to a schema, or whose month reached its spend cap, gets no verdict at
| all, and the caller falls back to rules that need no model. A question
| never waits longer than its timeout.
|
*/

package verdicts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/ai/pipeline"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/managers"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
Why a question got no verdict: the workspace has no model to ask, or
its month reached the spend cap.
*/
var (
	ErrNoModel = errors.New("verdicts: no model to ask")
	ErrCapped  = errors.New("verdicts: spend cap reached")
)

/*
defaultTimeout is how long a question waits when none is set.
*/
const defaultTimeout = 3 * time.Second

/*
Verdicts asks the cheap model. Timeout bounds each question.
*/
type Verdicts struct {
	DB      *database.DB
	Clock   clock.Clock
	LLM     *managers.LLMManager
	Prices  *aiprices.Table
	Timeout time.Duration
}

/*
question is one question to ask: the workspace, the kind of run it
records, the conversation it is about when there is one, the
instructions, the untrusted text, and the JSON Schema the answer must
match, under its name.
*/
type question struct {
	workspaceID    ids.UUID
	kind           string
	conversationID *ids.UUID
	system         string
	text           string
	schemaName     string
	schema         json.RawMessage
}

/*
asked is the model a question goes to, its connection, the agent it
spends for, and the run it records.
*/
type asked struct {
	agent models.AiAgent
	key   models.AiProviderKey
	model string
	llm   contracts.LLM
	run   models.AiRun
}

/*
candidate is a model a question may go to, on its connection.
*/
type candidate struct {
	key   models.AiProviderKey
	model string
}

/*
ask asks the question and decodes the model's answer, one JSON object
the schema forces, into out.
*/
func (v *Verdicts) ask(ctx context.Context, q question, out any) error {
	// Callers such as workflow runs may carry no workspace; row security
	// shows the question's workspace only inside its own scope.
	ctx = tenancy.With(ctx, q.workspaceID.String())

	a, err := v.choose(ctx, q)
	if err != nil {
		return err
	}

	if a.run, err = v.start(ctx, q, a); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, v.timeout())
	defer cancel()

	resp, err := a.llm.Chat(ctx, contracts.ChatRequest{
		Model: a.model, System: []contracts.SystemBlock{{Text: q.system}}, MaxOutputTokens: 200,
		Schema: &contracts.ResponseSchema{Name: q.schemaName, Schema: q.schema},
		Messages: []contracts.ChatMessage{{
			Role: contracts.ChatRoleUser, Parts: []contracts.ContentPart{{Text: untrusted(q.text)}},
		}},
	})
	if errors.Is(err, contracts.ErrSchemaUnsupported) {
		return errors.Join(ErrNoModel, v.finish(context.WithoutCancel(ctx), a, contracts.LLMUsage{}, err))
	}

	if err != nil {
		return v.finish(context.WithoutCancel(ctx), a, contracts.LLMUsage{}, err)
	}

	err = json.Unmarshal([]byte(resp.Text), out)
	if err != nil {
		err = fmt.Errorf("read verdict: %w", err)
	}

	return errors.Join(err, v.finish(context.WithoutCancel(ctx), a, resp.Usage, err))
}

/*
timeout is how long a question waits.
*/
func (v *Verdicts) timeout() time.Duration {
	if v.Timeout > 0 {
		return v.Timeout
	}

	return defaultTimeout
}

/*
choose picks the first of the workspace's models that keeps to a
schema: the agent's classifier model, then its answer model. A
workspace with none, or over its spend cap, has no model to ask.
*/
func (v *Verdicts) choose(ctx context.Context, q question) (asked, error) {
	var (
		agent      models.AiAgent
		candidates []candidate
	)

	err := v.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		m := models.New(tx)

		var err error

		if agent, candidates, err = candidatesOf(ctx, m, q.workspaceID); err != nil {
			return err
		}

		capped, err := pipeline.SpendCapped(ctx, m, v.Clock.Now(), agent)
		if err == nil && capped {
			return ErrCapped
		}

		return err
	})
	if err != nil {
		return asked{}, err
	}

	for _, c := range candidates {
		llm, err := v.LLM.ForKey(ctx, c.key)
		if err == nil && llm.Capabilities(c.model).JSONSchema {
			return asked{agent: agent, key: c.key, model: c.model, llm: llm}, nil
		}
	}

	return asked{}, ErrNoModel
}

/*
start opens the question's run.
*/
func (v *Verdicts) start(ctx context.Context, q question, a asked) (models.AiRun, error) {
	var run models.AiRun

	err := v.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		now := v.Clock.Now()

		var err error

		run, err = models.New(tx).CreateRun(ctx, models.CreateRunParams{
			WorkspaceID: q.workspaceID, AgentID: a.agent.ID, Kind: q.kind, ConversationID: q.conversationID,
			IncludedMessageIds: []ids.UUID{}, Status: "running", PromptHashes: json.RawMessage("{}"),
			ConfigSnapshot: json.RawMessage(`{"model":` + quoted(a.model) + `}`), StartedAt: &now, Now: now,
		})
		if err != nil {
			return fmt.Errorf("start %s run: %w", q.kind, err)
		}

		return nil
	})

	return run, err
}

/*
candidatesOf returns the workspace's agent and the models a question may
go to, classifier first.
*/
func candidatesOf(ctx context.Context, q *models.Queries, workspaceID ids.UUID) (
	models.AiAgent, []candidate, error,
) {
	agent, err := q.FindAgent(ctx, workspaceID)
	if errors.Is(err, database.ErrNotFound) {
		return agent, nil, ErrNoModel
	}

	if err != nil {
		return agent, nil, fmt.Errorf("find agent: %w", err)
	}

	var out []candidate

	roles := []struct {
		keyID *ids.UUID
		model *string
	}{{agent.ClassifierKeyID, agent.ClassifierModel}, {agent.AnswerKeyID, agent.AnswerModel}}

	for _, role := range roles {
		keyID, model := role.keyID, role.model
		if keyID == nil || model == nil || *model == "" {
			continue
		}

		key, err := q.FindProviderKey(ctx, models.FindProviderKeyParams{WorkspaceID: workspaceID, ID: *keyID})
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			return agent, nil, fmt.Errorf("find provider key: %w", err)
		}

		if err == nil {
			out = append(out, candidate{key: key, model: *model})
		}
	}

	return agent, out, nil
}

/*
finish records how the run went and adds its cost to the month.
*/
func (v *Verdicts) finish(ctx context.Context, a asked, usage contracts.LLMUsage, failure error) error {
	now := v.Clock.Now()
	outcome, status := "answered", "completed"
	params := models.FinishRunParams{
		InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens,
		OutputTokens: usage.OutputTokens, CostMicros: v.cost(a, usage), Retrieval: json.RawMessage("[]"),
		Flags: []string{}, CompletedAt: &now, Now: now, WorkspaceID: a.run.WorkspaceID, ID: a.run.ID,
	}

	if failure != nil {
		outcome, status = "error", "failed"
		params.ErrorMessage = new(failure.Error())
	}

	params.Status, params.Outcome = status, &outcome

	err := v.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		if _, err := models.New(tx).FinishRun(ctx, params); err != nil {
			return fmt.Errorf("finish run: %w", err)
		}

		return pipeline.AddTaskSpend(ctx, tx, a.agent, pipeline.TaskSpend{
			At: now, Cost: params.CostMicros, Input: usage.InputTokens, Cached: usage.CachedInputTokens,
			Output: usage.OutputTokens,
		})
	})

	return errors.Join(failure, err)
}

/*
cost prices the model call, zero when the price is unknown.
*/
func (v *Verdicts) cost(a asked, u contracts.LLMUsage) int64 {
	if v.Prices == nil {
		return 0
	}

	c := v.Prices.Cost(a.key.Provider, a.model, aiprices.TokenCounts{
		Input: u.InputTokens, CachedInput: u.CachedInputTokens, CacheWrite: u.CacheWriteTokens, Output: u.OutputTokens,
	})
	if c == nil {
		return 0
	}

	return *c
}

/*
untrusted wraps text a customer or teammate wrote, so the model reads
it as data and never as instructions.
*/
func untrusted(text string) string {
	text = strings.ReplaceAll(text, "</untrusted>", "")

	return "<untrusted>\n" + text + "\n</untrusted>"
}

/*
quoted returns the text as a JSON string.
*/
func quoted(text string) string {
	raw, _ := json.Marshal(text)

	return string(raw)
}
