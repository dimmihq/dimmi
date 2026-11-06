/*
|--------------------------------------------------------------------------
| Agent Usage
|--------------------------------------------------------------------------
|
| One month of the agent at a glance: what it cost and how much it did,
| from the running tally the cap reads, then day by day, by model and
| by how its runs ended, from the runs themselves. Months are counted
| in UTC, like the cap.
|
*/

package ai

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UsageInput names the month, as YYYY-MM; empty is the current month.
*/
type UsageInput struct {
	Membership authz.Membership
	Period     string
}

/*
UsageView is one month's usage: the tally, the daily series, the
spend by model and the runs by outcome, with the cap the month is held
to, nil when there is none.
*/
type UsageView struct {
	Period    models.AiSpendPeriod
	Daily     []models.ListDailySpendRow
	ByModel   []models.ListSpendByModelRow
	ByOutcome []models.ListSpendByOutcomeRow
	CapMicros *int64
}

/*
ShowUsage returns one month of the agent's usage.

Rules:
  - The caller needs ai:read.
  - A month with no runs answers zeros, never an error.

Errors: [exceptions.ErrForbidden], a [fault] for a period that is no
month, or a wrapped database error.
*/
type ShowUsage struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle returns the month's usage.
*/
func (a *ShowUsage) Handle(ctx context.Context, in UsageInput) (*UsageView, error) {
	if err := RequireAgentRead(in.Membership); err != nil {
		return nil, err
	}

	start, err := a.monthStart(in.Period)
	if err != nil {
		return nil, err
	}

	var view *UsageView

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		view, err = read(ctx, models.New(tx), in.Membership.Workspace.ID, start)
		return err
	})
	if err != nil {
		return nil, err
	}

	return view, nil
}

/*
monthStart parses the period, or takes the current month.
*/
func (a *ShowUsage) monthStart(period string) (time.Time, error) {
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
read builds the view from the tally and the runs of the month starting
at start.
*/
func read(ctx context.Context, q *models.Queries, workspaceID ids.UUID, start time.Time) (*UsageView, error) {
	view := &UsageView{}
	end := start.AddDate(0, 1, 0)

	period, err := q.GetSpendPeriod(ctx, models.GetSpendPeriodParams{
		WorkspaceID: workspaceID, PeriodStart: database.Date(start),
	})
	if err != nil {
		period = models.AiSpendPeriod{WorkspaceID: workspaceID}
	}
	period.PeriodStart = database.Date(start)
	view.Period = period

	if agent, err := q.FindAgent(ctx, workspaceID); err == nil {
		view.CapMicros = agent.MonthlySpendCapMicros
	}

	if view.Daily, err = q.ListDailySpend(ctx, models.ListDailySpendParams{
		WorkspaceID: workspaceID, PeriodStart: start, PeriodEnd: end,
	}); err != nil {
		return nil, fmt.Errorf("list daily spend: %w", err)
	}
	if view.ByModel, err = q.ListSpendByModel(ctx, models.ListSpendByModelParams{
		WorkspaceID: workspaceID, PeriodStart: start, PeriodEnd: end,
	}); err != nil {
		return nil, fmt.Errorf("list model spend: %w", err)
	}
	if view.ByOutcome, err = q.ListSpendByOutcome(ctx, models.ListSpendByOutcomeParams{
		WorkspaceID: workspaceID, PeriodStart: start, PeriodEnd: end,
	}); err != nil {
		return nil, fmt.Errorf("list outcome spend: %w", err)
	}

	return view, nil
}
