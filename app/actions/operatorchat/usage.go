/*
|--------------------------------------------------------------------------
| Operator Usage
|--------------------------------------------------------------------------
|
| One month of Operator at a glance, for whoever looks after the agent:
| what it cost and how often it ran, day by day, why it ran, for whom,
| with which models, what became of the changes it proposed, and its
| latest runs. Only Operator's own runs count, so dimmi's work never
| shows here. Months are counted in UTC, like the spending cap both
| agents share.
|
*/

package operatorchat

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/ai"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
recentRuns is how many of the month's latest runs the view lists.
*/
const recentRuns = 25

/*
UsageInput names the month, as YYYY-MM; empty is the current month.
*/
type UsageInput struct {
	Membership authz.Membership
	Period     string
}

/*
UsageView is one month of Operator: its totals, the daily series, the
splits by purpose, teammate and model, the changesets by status, its
latest runs, and the agent's whole spend for the month with the cap it
is held to, nil when there is none.
*/
type UsageView struct {
	Start      time.Time
	Totals     models.OperatorUsageTotalsRow
	Daily      []models.OperatorUsageByDayRow
	ByPurpose  []models.OperatorUsageByPurposeRow
	ByMember   []models.OperatorUsageByMemberRow
	ByModel    []models.OperatorUsageByModelRow
	Changesets []models.OperatorChangesetsByStatusRow
	Recent     []models.ListRecentOperatorRunsRow
	AgentCost  int64
	CapMicros  *int64
}

/*
Usage returns one month of Operator's usage.

Rules:
  - The caller needs ai:read, like the agent's own usage.
  - A month with no runs answers zeros, never an error.

Errors: [exceptions.ErrForbidden], a [fault] for a period that is no
month, or a wrapped database error.
*/
type Usage struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns the month's usage.
*/
func (a *Usage) Handle(ctx context.Context, in UsageInput) (*UsageView, error) {
	if err := ai.RequireAgentRead(in.Membership); err != nil {
		return nil, err
	}
	start, err := a.monthStart(in.Period)
	if err != nil {
		return nil, err
	}
	view := &UsageView{Start: start}
	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		r := month{workspace: in.Membership.Workspace.ID, since: start, until: start.AddDate(0, 1, 0)}
		if err := r.totals(ctx, q, view); err != nil {
			return err
		}
		return r.splits(ctx, q, view)
	})
	if err != nil {
		return nil, err
	}
	return view, nil
}

/*
monthStart parses the period, or takes the current month.
*/
func (a *Usage) monthStart(period string) (time.Time, error) {
	if period == "" {
		now := a.Clock.Now().UTC()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return time.Time{}, fault.New(fault.Invalid, "period_invalid", "the period must be a month, as YYYY-MM").
			OnField("period")
	}
	return start, nil
}

/*
month is the workspace and range one view reads.
*/
type month struct {
	workspace    ids.UUID
	since, until time.Time
}

/*
totals reads Operator's totals and the agent's spend and cap.
*/
func (r month) totals(ctx context.Context, q *models.Queries, view *UsageView) error {
	var err error
	view.Totals, err = q.OperatorUsageTotals(ctx, models.OperatorUsageTotalsParams{
		WorkspaceID: r.workspace, Since: r.since, Until: r.until,
	})
	if err != nil {
		return fmt.Errorf("sum operator usage: %w", err)
	}
	period, err := q.GetSpendPeriod(ctx, models.GetSpendPeriodParams{
		WorkspaceID: r.workspace, PeriodStart: database.Date(r.since),
	})
	if err == nil {
		view.AgentCost = period.CostMicros
	}
	if agent, err := q.FindAgent(ctx, r.workspace); err == nil {
		view.CapMicros = agent.MonthlySpendCapMicros
	}
	return nil
}

/*
splits reads the daily series, the splits and the latest runs.
*/
func (r month) splits(ctx context.Context, q *models.Queries, view *UsageView) error {
	var err error
	if view.Daily, err = q.OperatorUsageByDay(ctx, models.OperatorUsageByDayParams{
		WorkspaceID: r.workspace, Since: r.since, Until: r.until,
	}); err != nil {
		return fmt.Errorf("list operator daily usage: %w", err)
	}
	if view.ByPurpose, err = q.OperatorUsageByPurpose(ctx, models.OperatorUsageByPurposeParams{
		WorkspaceID: r.workspace, Since: r.since, Until: r.until,
	}); err != nil {
		return fmt.Errorf("list operator usage by purpose: %w", err)
	}
	if view.ByMember, err = q.OperatorUsageByMember(ctx, models.OperatorUsageByMemberParams{
		WorkspaceID: r.workspace, Since: r.since, Until: r.until,
	}); err != nil {
		return fmt.Errorf("list operator usage by teammate: %w", err)
	}
	if view.ByModel, err = q.OperatorUsageByModel(ctx, models.OperatorUsageByModelParams{
		WorkspaceID: r.workspace, Since: r.since, Until: r.until,
	}); err != nil {
		return fmt.Errorf("list operator usage by model: %w", err)
	}
	if view.Changesets, err = q.OperatorChangesetsByStatus(ctx, models.OperatorChangesetsByStatusParams{
		WorkspaceID: r.workspace, Since: r.since, Until: r.until,
	}); err != nil {
		return fmt.Errorf("count operator changesets: %w", err)
	}
	if view.Recent, err = q.ListRecentOperatorRuns(ctx, models.ListRecentOperatorRunsParams{
		WorkspaceID: r.workspace, Since: r.since, Until: r.until, RowLimit: recentRuns,
	}); err != nil {
		return fmt.Errorf("list recent operator runs: %w", err)
	}
	return nil
}
