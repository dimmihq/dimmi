/*
|--------------------------------------------------------------------------
| Run Recorder
|--------------------------------------------------------------------------
|
| Every step lands on ai_runs and ai_run_steps as it happens, so a
| trace exists even when the run dies mid-way. Costs are summed into
| the period tally at finish; payloads are redacted before they are
| written.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
Recorder writes one run and its steps.
*/
type Recorder struct {
	db         *database.DB
	clock      clock.Clock
	prices     *aiprices.Table
	Run        models.AiRun
	step       int
	tokens     tokens
	costMicros int64
}

/*
tokens are the run's usage sums.
*/
type tokens struct {
	input  int64
	cached int64
	write  int64
	output int64
}

/*
newRecorder creates the run row.
*/
func (r *Runner) newRecorder(run models.AiRun) *Recorder {
	return &Recorder{db: r.DB, clock: r.Clock, prices: r.Prices, Run: run}
}

/*
Step writes one completed step and returns its row.
*/
func (r *Recorder) Step(ctx context.Context, tx *database.Tx, in StepInput) (models.AiRunStep, error) {
	r.step++

	usageEstimated := !in.Usage.Reported && (in.Usage.InputTokens > 0 || in.Usage.OutputTokens > 0)
	cost := in.CostMicros
	if cost == 0 && r.prices != nil {
		if c := r.prices.Cost(in.Provider, in.Model, aiprices.TokenCounts{
			Input: in.Usage.InputTokens, CachedInput: in.Usage.CachedInputTokens,
			CacheWrite: in.Usage.CacheWriteTokens, Output: in.Usage.OutputTokens,
		}); c != nil {
			cost = *c
		}
	}

	r.tokens.input += in.Usage.InputTokens
	r.tokens.cached += in.Usage.CachedInputTokens
	r.tokens.write += in.Usage.CacheWriteTokens
	r.tokens.output += in.Usage.OutputTokens
	r.costMicros += cost

	var latency, firstToken *int32
	if in.Latency > 0 {
		v := sat32(in.Latency.Milliseconds())
		latency = &v
	}
	if in.FirstToken > 0 {
		v := sat32(in.FirstToken.Milliseconds())
		firstToken = &v
	}

	step, err := models.New(tx).CreateRunStep(ctx, models.CreateRunStepParams{
		WorkspaceID: r.Run.WorkspaceID, RunID: r.Run.ID, Seq: sat32(int64(r.step)),
		Kind: string(in.Kind), Provider: nilable(in.Provider), ProviderKeyID: in.ProviderKeyID,
		Model: nilable(in.Model), ResponseModel: nilable(in.ResponseModel), Attempt: sat32(int64(in.Attempt)),
		Status: in.Status, ErrorKind: nilable(in.ErrorKind), HttpStatus: nilableInt(in.HTTPStatus),
		ProviderRequestID: nilable(in.RequestID), Request: in.RequestPayload, Response: in.ResponsePayload,
		InputTokens: in.Usage.InputTokens, CachedInputTokens: in.Usage.CachedInputTokens,
		CacheWriteTokens: in.Usage.CacheWriteTokens, OutputTokens: in.Usage.OutputTokens,
		ReasoningTokens: in.Usage.ReasoningTokens, UsageEstimated: usageEstimated,
		CostMicros: cost, LatencyMs: latency, FirstTokenMs: firstToken,
		StartedAt: nilableTime(in.StartedAt), CompletedAt: nilableTime(in.CompletedAt),
	})
	return step, err
}

/*
StepInput is one step's record.
*/
type StepInput struct {
	Kind            enums.AIRunStepKind
	Provider        string
	ProviderKeyID   *ids.UUID
	Model           string
	ResponseModel   string
	Attempt         int
	Status          string
	ErrorKind       string
	HTTPStatus      int
	RequestID       string
	RequestPayload  json.RawMessage
	ResponsePayload json.RawMessage
	Usage           usageInput
	CostMicros      int64
	Latency         time.Duration
	FirstToken      time.Duration
	StartedAt       time.Time
	CompletedAt     time.Time
}

/*
usageInput is the contract's usage shape.
*/
type usageInput = contracts.LLMUsage

/*
Finish closes the run with its outcome and sums.
*/
func (r *Recorder) Finish(ctx context.Context, tx *database.Tx, in FinishInput) error {
	if len(in.Retrieval) == 0 {
		in.Retrieval = json.RawMessage("[]")
	}
	if in.Flags == nil {
		in.Flags = []string{}
	}
	now := r.clock.Now()
	latency := time.Duration(0)
	if r.Run.StartedAt != nil {
		latency = now.Sub(*r.Run.StartedAt)
	}

	var firstToken *int32
	if in.FirstToken > 0 {
		v := sat32(in.FirstToken.Milliseconds())
		firstToken = &v
	}
	lat := sat32(latency.Milliseconds())

	_, err := models.New(tx).FinishRun(ctx, models.FinishRunParams{
		WorkspaceID: r.Run.WorkspaceID, ID: r.Run.ID,
		Status: in.Status, Outcome: nilable(in.Outcome), SkipReason: nilable(in.SkipReason),
		HandoffReason: nilable(in.HandoffReason), Intent: nilable(in.Intent), Language: nilable(in.Language),
		QueryOriginal: nilable(in.QueryOriginal), QueryRewritten: nilable(in.QueryRewritten),
		Retrieval: in.Retrieval, AnswerMessageID: in.AnswerMessageID,
		Flags: in.Flags, InputTokens: r.tokens.input, CachedInputTokens: r.tokens.cached,
		CacheWriteTokens: r.tokens.write, OutputTokens: r.tokens.output,
		CostMicros: r.costMicros, PriceTableVersion: nilable(r.priceVersion()),
		FirstTokenMs: firstToken, LatencyMs: &lat, QueueWaitMs: nilableInt(in.QueueWaitMS),
		ErrorCode: nilable(in.ErrorCode), ErrorMessage: nilable(in.ErrorMessage),
		ActionCallsCount: sat32(int64(in.ActionCalls)), CompletedAt: &now, Now: now,
	})
	return err
}

/*
FinishInput is how a run ends.
*/
type FinishInput struct {
	Status          string
	Outcome         string
	SkipReason      string
	HandoffReason   string
	Intent          string
	Language        string
	QueryOriginal   string
	QueryRewritten  string
	Retrieval       json.RawMessage
	AnswerMessageID *ids.UUID
	Flags           []string
	FirstToken      time.Duration
	QueueWaitMS     int
	ErrorCode       string
	ErrorMessage    string
	ActionCalls     int
}

/*
Cost returns the run's cost so far.
*/
func (r *Recorder) Cost() int64 { return r.costMicros }

/*
priceVersion names the table in use.
*/
func (r *Recorder) priceVersion() string {
	if r.prices == nil {
		return ""
	}
	return r.prices.Version
}

/*
nilable maps an empty string to NULL.
*/
func nilable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

/*
nilableInt maps zero to NULL.
*/
func nilableInt(v int) *int32 {
	if v == 0 {
		return nil
	}
	i := sat32(int64(v))
	return &i
}

/*
nilableTime maps a zero time to NULL.
*/
func nilableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

/*
sat32 clamps a count into int32 for the columns that take one.
*/
func sat32(n int64) int32 {
	if n > 2147483647 {
		return 2147483647
	}
	if n < -2147483648 {
		return -2147483648
	}
	return int32(n)
}
