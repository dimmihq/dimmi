/*
|--------------------------------------------------------------------------
| Spend
|--------------------------------------------------------------------------
|
| Every run's cost and tokens land on the month's tally in the same
| transaction as the run row, so the spend cap the gate reads is never
| behind the runs that spent it. Crossing 50, 80 and 100 percent of a
| cap is marked once each; reaching 100 percent caps the month.
|
*/

package pipeline

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
spendDelta is what one write adds to the month: cost, tokens, and the
run, answered, unknown, handoff and silent counts.
*/
type spendDelta struct {
	cost, input, cached, output int64
	counts                      [5]int32
}

/*
recordSpend adds the run, its outcome and its sums to the month's
tally.
*/
func (r *Runner) recordSpend(ctx context.Context, tx *database.Tx, st *run, d *decision) error {
	rec := st.rec
	return r.addSpend(ctx, tx, st, spendDelta{
		cost: rec.costMicros, input: rec.tokens.input, cached: rec.tokens.cached, output: rec.tokens.output,
		counts: outcomeCounts(d.outcome),
	})
}

/*
addSpend writes one delta and checks the thresholds.
*/
func (r *Runner) addSpend(ctx context.Context, tx *database.Tx, st *run, delta spendDelta) error {
	e := st.env
	counts := delta.counts
	now := r.Clock.Now()
	period, err := models.New(tx).AddSpend(ctx, models.AddSpendParams{
		WorkspaceID: e.Workspace.ID, PeriodStart: database.Date(monthStart(now)),
		CostMicros: delta.cost, InputTokens: delta.input, CachedInputTokens: delta.cached,
		OutputTokens: delta.output, RunsCount: counts[0], AnsweredCount: counts[1],
		UnknownCount: counts[2], HandoffCount: counts[3], SilentCount: counts[4], Now: now,
	})
	if err != nil {
		return fmt.Errorf("add spend: %w", err)
	}
	return r.checkThresholds(ctx, tx, e, period)
}

/*
outcomeCounts are the run, answered, unknown, handoff and silent
counts one run adds.
*/
func outcomeCounts(outcome string) [5]int32 {
	c := [5]int32{1}
	switch outcome {
	case outcomeAnswered:
		c[1] = 1
	case outcomeUnknown:
		c[2] = 1
	case outcomeHandoff, outcomeError:
		c[3] = 1
	case outcomeSilent:
		c[4] = 1
	}
	return c
}

/*
checkThresholds marks the 50 and 80 percent notices and caps the
period at a hundred, each once. No cap, or a cap of zero, means no
limit.
*/
func (r *Runner) checkThresholds(ctx context.Context, tx *database.Tx, e *env, period models.AiSpendPeriod) error {
	capMicros := e.Agent.MonthlySpendCapMicros
	if capMicros == nil || *capMicros <= 0 {
		return nil
	}
	pct := period.CostMicros * 100 / *capMicros

	q := models.New(tx)
	for _, threshold := range []int32{50, 80, 100} {
		if pct < int64(threshold) {
			continue
		}
		if err := q.MarkSpendThreshold(ctx, models.MarkSpendThresholdParams{
			WorkspaceID: e.Workspace.ID, PeriodStart: period.PeriodStart, Threshold: threshold,
		}); err != nil {
			return fmt.Errorf("mark spend threshold: %w", err)
		}
	}
	if pct < 100 || period.CappedAt != nil {
		return nil
	}
	now := r.Clock.Now()
	if err := q.SetSpendCap(ctx, models.SetSpendCapParams{
		WorkspaceID: e.Workspace.ID, PeriodStart: period.PeriodStart, CappedAt: &now, Now: now,
	}); err != nil {
		return fmt.Errorf("cap spend period: %w", err)
	}
	return nil
}
