/*
|--------------------------------------------------------------------------
| Run Record
|--------------------------------------------------------------------------
|
| Every Operator turn is an AI run of kind task, named after the
| teammate who asked and the conversation it is about, so its tokens,
| cost and steps show beside the customer agent's. Each model call and
| tool call is a step; a step that fails to save is dropped, since the
| trace is best effort and the answer is not.
|
*/

package operator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
runKind is the kind of run every Operator turn records.
*/
const runKind = "task"

/*
tally is a run's sums: tokens, cost, step count and the time to the
first token.
*/
type tally struct {
	seq                          int32
	input, cached, write, output int64
	cost                         int64
	firstToken                   time.Duration
}

/*
startRun opens the run row for the session.
*/
func (e *Engine) startRun(ctx context.Context, q *models.Queries, s *session) (models.AiRun, error) {
	now := e.Clock.Now()
	actor := "member"
	snapshot, _ := json.Marshal(map[string]any{"answer_model": s.model, "operator": true, "purpose": purposeOf(s)})
	run, err := q.CreateRun(ctx, models.CreateRunParams{
		WorkspaceID: s.m.Workspace.ID, AgentID: s.agent.ID, Kind: runKind,
		ConversationID: s.thread.ConversationID, IncludedMessageIds: []ids.UUID{},
		ActorType: &actor, ActorID: &s.m.Member.ID, Status: "running",
		PromptVersion: promptVersion, PromptHashes: json.RawMessage("{}"), ConfigSnapshot: snapshot,
		StartedAt: &now, Now: now,
	})
	if err != nil {
		return run, fmt.Errorf("start operator run: %w", err)
	}
	return run, nil
}

/*
The reasons an Operator run starts: a teammate's question, a brief on a
conversation, or a triage when a conversation arrives. Usage reports
split runs by them.
*/
const (
	PurposeAsk    = "ask"
	PurposeBrief  = "brief"
	PurposeTriage = "triage"
)

/*
purposeOf says why the session runs.
*/
func purposeOf(s *session) string {
	switch {
	case s.background:
		return PurposeTriage
	case s.question == strings.TrimSpace(BriefQuestion):
		return PurposeBrief
	default:
		return PurposeAsk
	}
}

/*
step is one finished step to record: its kind, the model or tool it
ran, its text both ways, its usage and its error.
*/
type step struct {
	kind              enums.AIRunStepKind
	name              string
	started           time.Time
	firstToken        time.Duration
	request, response string
	usage             contracts.LLMUsage
	err               error
}

/*
record prices and writes one step in its own transaction.
*/
func (e *Engine) record(ctx context.Context, s *session, in step) {
	s.tally.seq++
	cost := int64(0)
	if in.kind == enums.AIRunStepKindGenerate {
		cost = e.price(s, in.usage)
		if in.firstToken > 0 && s.tally.firstToken == 0 {
			s.tally.firstToken = in.firstToken
		}
	}
	now := e.Clock.Now()
	params := models.CreateRunStepParams{
		WorkspaceID: s.m.Workspace.ID, RunID: s.run.ID, Seq: s.tally.seq, Kind: string(in.kind),
		Model: &in.name, Attempt: 1, Status: "ok", InputTokens: in.usage.InputTokens,
		CachedInputTokens: in.usage.CachedInputTokens, CacheWriteTokens: in.usage.CacheWriteTokens,
		OutputTokens: in.usage.OutputTokens, ReasoningTokens: in.usage.ReasoningTokens, CostMicros: cost,
		LatencyMs: millis(now.Sub(in.started)), FirstTokenMs: millis(in.firstToken),
		StartedAt: &in.started, CompletedAt: &now,
	}
	if in.kind == enums.AIRunStepKindGenerate {
		provider := string(s.llm.Provider())
		params.Provider, params.ProviderKeyID = &provider, &s.key.ID
	}
	if e.StorePayloads {
		params.Request, params.Response = payload(in.request), payload(in.response)
	}
	if in.err != nil {
		failure(&params, in.err)
		s.failure = in.err
	}
	_ = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).CreateRunStep(ctx, params)
		return err
	})
}

/*
failure records why a step failed: canceled by the teammate, or the
provider's kind of error, status and code.
*/
func failure(p *models.CreateRunStepParams, err error) {
	p.Status = "error"
	if errors.Is(err, context.Canceled) {
		p.Status = "canceled"
		return
	}
	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	if !ok {
		p.ErrorKind = new("operator")
		return
	}
	p.ErrorKind, p.ProviderErrorCode = new(string(llmErr.Kind)), nilable(llmErr.ProviderCode)
	if llmErr.HTTPStatus != 0 {
		p.HttpStatus = new(int32(max(min(llmErr.HTTPStatus, 999), 0)))
	}
}

/*
nilable is nil for an empty text.
*/
func nilable(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}

/*
price adds a model call's usage to the run's sums and returns its cost.
*/
func (e *Engine) price(s *session, u contracts.LLMUsage) int64 {
	if !u.Reported {
		u.InputTokens = max(u.InputTokens, 1)
	}
	s.tally.input += u.InputTokens
	s.tally.cached += u.CachedInputTokens
	s.tally.write += u.CacheWriteTokens
	s.tally.output += u.OutputTokens
	if e.Prices == nil {
		return 0
	}
	c := e.Prices.Cost(string(s.llm.Provider()), s.model, aiprices.TokenCounts{
		Input: u.InputTokens, CachedInput: u.CachedInputTokens, CacheWrite: u.CacheWriteTokens, Output: u.OutputTokens,
	})
	if c == nil {
		return 0
	}
	s.tally.cost += *c
	return *c
}

/*
finishRun closes the run row with how the turn ended and its sums.
*/
func (e *Engine) finishRun(ctx context.Context, q *models.Queries, s *session, status string) error {
	now := e.Clock.Now()
	runStatus, outcome := "completed", "answered"
	var code *string
	if status != "done" {
		runStatus, outcome, code = "failed", "error", &status
	}
	var priceVersion *string
	if e.Prices != nil {
		v := e.Prices.Version
		priceVersion = &v
	}
	question := truncate(s.question, 2000)
	var message *string
	if s.failure != nil {
		message = new(truncate(s.failure.Error(), 500))
	}
	_, err := q.FinishRun(ctx, models.FinishRunParams{
		WorkspaceID: s.m.Workspace.ID, ID: s.run.ID, Status: runStatus, Outcome: &outcome,
		QueryOriginal: &question, Retrieval: json.RawMessage("[]"), Flags: []string{},
		InputTokens: s.tally.input, CachedInputTokens: s.tally.cached, CacheWriteTokens: s.tally.write,
		OutputTokens: s.tally.output, CostMicros: s.tally.cost, PriceTableVersion: priceVersion,
		FirstTokenMs: millis(s.tally.firstToken), LatencyMs: millis(now.Sub(s.run.CreatedAt)),
		ErrorCode: code, ErrorMessage: message, CompletedAt: &now, Now: now,
	})
	if err != nil {
		return fmt.Errorf("finish operator run: %w", err)
	}
	return nil
}

/*
payload stores step text as a JSON string, nil when there is none.
*/
func payload(text string) json.RawMessage {
	if text == "" {
		return nil
	}
	raw, _ := json.Marshal(truncate(text, 32<<10))
	return raw
}

/*
millis is a duration in whole milliseconds, nil when there is none.
*/
func millis(d time.Duration) *int32 {
	if d <= 0 {
		return nil
	}
	return new(int32(max(min(d.Milliseconds(), 2147483647), -2147483648)))
}

/*
truncate keeps at most n runes of a text.
*/
func truncate(text string, n int) string {
	runes := []rune(text)
	if len(runes) <= n {
		return text
	}
	return string(runes[:n])
}
