/*
|--------------------------------------------------------------------------
| Run Recorder
|--------------------------------------------------------------------------
|
| Every step lands on ai_run_steps the moment it completes, in its own
| short transaction, so a trace exists even when the run dies midway
| and no transaction waits on a model. Each step's usage is priced as
| it lands; when a provider reports no usage the tokens are estimated
| from the text and the step says so. Payloads are stored only when the
| install keeps them, and only as redacted as the agent's setting.
|
*/

package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/aiprices"
)

/*
payloadChars caps one stored payload.
*/
const payloadChars = 64_000

/*
Recorder tallies one run's steps and sums.
*/
type Recorder struct {
	clock      clock.Clock
	prices     *aiprices.Table
	Run        models.AiRun
	seq        int32
	tokens     tokens
	costMicros int64
	firstToken time.Duration
}

/*
tokens are the run's usage sums.
*/
type tokens struct {
	input, cached, write, output int64
}

/*
newRecorder starts the tally for a run row.
*/
func (r *Runner) newRecorder(row models.AiRun) *Recorder {
	return &Recorder{clock: r.Clock, prices: r.Prices, Run: row}
}

/*
stepRecord is one finished step: what ran, on which connection and
model, its text both ways, its usage, and its error if it failed. A
provider or cost set here wins over the driver's and the price table's.
*/
type stepRecord struct {
	kind          enums.AIRunStepKind
	llm           contracts.LLM
	key           *keyRef
	provider      string
	cost          *int64
	model         string
	responseModel string
	requestID     string
	attempt       int
	started       time.Time
	firstToken    time.Duration
	request       string
	response      string
	usage         contracts.LLMUsage
	err           error
}

/*
step prices and writes one step in its own transaction. A failed write
is dropped: the trace is best effort, the answer is not.
*/
func (r *Runner) step(ctx context.Context, st *run, in stepRecord) {
	rec := st.rec
	rec.seq++

	usage, estimated := in.usage, false
	if !usage.Reported && in.llm != nil && in.err == nil {
		usage.InputTokens, usage.OutputTokens = estimate(in.request), estimate(in.response)
		estimated = true
	}
	provider := providerOf(in)
	cost := rec.price(provider, in.model, usage)
	if in.cost != nil {
		cost = *in.cost
		rec.costMicros += cost
	}
	if in.kind == enums.AIRunStepKindGenerate && in.firstToken > 0 && rec.firstToken == 0 {
		rec.firstToken = in.firstToken
	}

	params := models.CreateRunStepParams{
		WorkspaceID: rec.Run.WorkspaceID, RunID: rec.Run.ID, Seq: rec.seq, Kind: string(in.kind),
		Provider: nilable(provider), Model: nilable(in.model), ResponseModel: nilable(in.responseModel),
		Attempt: max(sat32(int64(in.attempt)), 1), Status: "ok", ProviderRequestID: nilable(in.requestID),
		InputTokens: usage.InputTokens, CachedInputTokens: usage.CachedInputTokens,
		CacheWriteTokens: usage.CacheWriteTokens, OutputTokens: usage.OutputTokens,
		ReasoningTokens: usage.ReasoningTokens, UsageEstimated: estimated, CostMicros: cost,
		LatencyMs: millis(rec.clock.Now().Sub(in.started)), FirstTokenMs: millis(in.firstToken),
		StartedAt: nilableTime(in.started), CompletedAt: nilableTime(rec.clock.Now()),
	}
	if in.key != nil {
		params.ProviderKeyID = &in.key.id
	}
	if r.StorePayloads {
		params.Request, params.Response = payload(in.request), payload(in.response)
	}
	if in.err != nil {
		params.Status, params.ErrorKind, params.HttpStatus = errorFields(in.err)
	}

	_ = r.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		_, err := models.New(tx).CreateRunStep(ctx, params)
		return err
	})
}

/*
price adds the usage to the run's sums and returns its cost.
*/
func (rec *Recorder) price(provider, model string, u contracts.LLMUsage) int64 {
	rec.tokens.input += u.InputTokens
	rec.tokens.cached += u.CachedInputTokens
	rec.tokens.write += u.CacheWriteTokens
	rec.tokens.output += u.OutputTokens

	if rec.prices == nil || model == "" {
		return 0
	}
	c := rec.prices.Cost(provider, model, aiprices.TokenCounts{
		Input: u.InputTokens, CachedInput: u.CachedInputTokens,
		CacheWrite: u.CacheWriteTokens, Output: u.OutputTokens,
	})
	if c == nil {
		return 0
	}
	rec.costMicros += *c
	return *c
}

/*
providerOf names whose price list a step is charged on: the driver's
provider, which the price table is keyed by, else the connection's.
*/
func providerOf(in stepRecord) string {
	if in.provider != "" {
		return in.provider
	}
	if in.llm != nil {
		return string(in.llm.Provider())
	}
	if in.key != nil {
		return in.key.provider
	}
	return ""
}

/*
errorFields maps a failed call to the step's status, error kind and
HTTP status.
*/
func errorFields(err error) (string, *string, *int32) {
	status := "error"
	if errors.Is(err, context.Canceled) {
		status = "canceled"
	}
	llmErr, ok := errors.AsType[*contracts.LLMError](err)
	if !ok {
		return status, new("pipeline"), nil
	}
	var code *int32
	if llmErr.HTTPStatus != 0 {
		code = new(sat32(int64(llmErr.HTTPStatus)))
	}
	return status, nilable(string(llmErr.Kind)), code
}

/*
estimate guesses a token count from text, about four characters each.
*/
func estimate(text string) int64 {
	return int64(len(text)+3) / 4
}

/*
finishRun closes the run row with the decision and the run's sums.
*/
func (r *Runner) finishRun(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	rec := st.rec
	now := r.Clock.Now()
	retrieval := st.retrieval
	if len(retrieval) == 0 {
		retrieval = json.RawMessage("[]")
	}
	flags := st.flags
	if flags == nil {
		flags = []string{}
	}
	latency := millis(now.Sub(derefTime(rec.Run.StartedAt, now)))
	queueWait := millis(derefTime(rec.Run.StartedAt, now).Sub(st.args.QueuedAt))
	if st.args.QueuedAt.IsZero() {
		queueWait = nil
	}

	_, err := models.New(tx).FinishRun(ctx, models.FinishRunParams{
		WorkspaceID: rec.Run.WorkspaceID, ID: rec.Run.ID,
		Status: d.status(), Outcome: nilable(d.outcome), SkipReason: nilable(d.skipReason),
		HandoffReason: nilable(d.handoffReason), Intent: nilable(st.c.Intent), Language: nilable(st.c.Language),
		QueryOriginal:  nilable(r.redactFor(st.env, st.env.pendingText())),
		QueryRewritten: nilable(r.redactFor(st.env, st.query)),
		Retrieval:      retrieval, AnswerMessageID: d.answerID, Flags: flags,
		InputTokens: rec.tokens.input, CachedInputTokens: rec.tokens.cached,
		CacheWriteTokens: rec.tokens.write, OutputTokens: rec.tokens.output,
		CostMicros: rec.costMicros, PriceTableVersion: nilable(rec.priceVersion()),
		FirstTokenMs: millis(rec.firstToken), LatencyMs: latency, QueueWaitMs: queueWait,
		ErrorCode: nilable(d.errorCode), ErrorMessage: nilable(truncate(d.errorMessage, 500)),
		CompletedAt: &now, Now: now,
	})
	return err
}

/*
priceVersion names the price table in use.
*/
func (rec *Recorder) priceVersion() string {
	if rec.prices == nil {
		return ""
	}
	return rec.prices.Version
}

/*
payload stores a text as a JSON string, capped.
*/
func payload(text string) json.RawMessage {
	if text == "" {
		return nil
	}
	out, err := json.Marshal(truncate(text, payloadChars))
	if err != nil {
		return nil
	}
	return out
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
nilableTime maps a zero time to NULL.
*/
func nilableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

/*
derefTime reads a nullable time with a fallback.
*/
func derefTime(t *time.Time, fallback time.Time) time.Time {
	if t == nil {
		return fallback
	}
	return *t
}

/*
millis maps a positive duration to whole milliseconds, NULL otherwise.
*/
func millis(d time.Duration) *int32 {
	if d <= 0 {
		return nil
	}
	return new(sat32(d.Milliseconds()))
}

/*
sat32 clamps a count into int32 for the columns that take one.
*/
func sat32(n int64) int32 {
	return int32(max(min(n, 2147483647), -2147483648))
}
