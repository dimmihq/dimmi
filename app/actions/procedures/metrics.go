/*
|--------------------------------------------------------------------------
| Procedure Scorecard
|--------------------------------------------------------------------------
|
| How one procedure does over the last 7, 30 or 90 days, for all its
| versions or one: the conversations it ran in, how many dimmi resolved
| (by the same definition as the dimmi agent report, so the numbers
| reconcile), how many went to the team and why, how customers rated
| them, how often its guardrails stepped in, how far conversations got
| through each step and which side each branch took, and how its actions
| fared. The runs behind the numbers are listed for drilling in. The
| report lists every procedure that ran in a range, side by side.
|
*/

package procedures

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
quiet is how long a customer stays silent after dimmi's last answer for
the conversation to count as resolved, as in the dimmi agent report.
*/
const quiet = 24 * time.Hour

/*
Metrics are a procedure's numbers over a range.
*/
type Metrics struct {
	Days        int
	Version     int32
	Figures     models.ProcedureConversationFiguresRow
	Handoffs    []models.ProcedureHandoffReasonsRow
	Guards      []models.ProcedureGuardEventsRow
	Steps       []models.ProcedureStepFiguresRow
	Actions     []models.ProcedureActionFiguresRow
	Runs        []models.ProcedureRunFactsRow
	ActionNames map[ids.UUID]string
}

/*
MetricsScope is the range of a scorecard: the last days, and one
version or, with zero, all.
*/
type MetricsScope struct {
	Days    int
	Version int32
}

/*
Metrics reads a procedure's scorecard over the last days, 7, 30 or 90,
for one version or, with zero, all.
*/
func (p *Procedures) Metrics(ctx context.Context, m authz.Membership, id ids.UUID, scope MetricsScope,
) (Metrics, error) {
	days, version := scope.Days, scope.Version
	if !m.Can("ai_actions:read") {
		return Metrics{}, exceptions.ErrForbidden
	}

	if !slices.Contains([]int{7, 30, 90}, days) {
		days = 30
	}

	now := p.Clock.Now()
	out := Metrics{Days: days, Version: version, ActionNames: map[ids.UUID]string{}}
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		return readMetrics(ctx, q, metricsRange{
			ws: m.Workspace.ID, id: id, since: now.Add(-time.Duration(days) * 24 * time.Hour),
			until:   now.Add(time.Second),
			version: version, quietBefore: now.Add(-quiet),
		}, &out)
	})

	return out, err
}

/*
metricsRange is what one scorecard reads.
*/
type metricsRange struct {
	ws, id       ids.UUID
	since, until time.Time
	version      int32
	quietBefore  time.Time
}

/*
readMetrics fills a scorecard.
*/
func readMetrics(ctx context.Context, q *models.Queries, r metricsRange, out *Metrics) error {
	var err error
	if out.Figures, err = q.ProcedureConversationFigures(ctx, models.ProcedureConversationFiguresParams{
		WorkspaceID: r.ws, Since: r.since, Until: r.until, ProcedureID: r.id, Version: r.version,
		QuietBefore: &r.quietBefore,
	}); err != nil {
		return fmt.Errorf("count procedure conversations: %w", err)
	}

	if out.Handoffs, err = q.ProcedureHandoffReasons(ctx, models.ProcedureHandoffReasonsParams{
		WorkspaceID: r.ws, Since: r.since, Until: r.until, ProcedureID: r.id, Version: r.version,
	}); err != nil {
		return fmt.Errorf("count procedure handovers: %w", err)
	}

	if out.Guards, err = q.ProcedureGuardEvents(ctx, models.ProcedureGuardEventsParams{
		WorkspaceID: r.ws, Since: r.since, Until: r.until, ProcedureID: r.id, Version: r.version,
	}); err != nil {
		return fmt.Errorf("count procedure guardrails: %w", err)
	}

	if out.Steps, err = q.ProcedureStepFigures(ctx, models.ProcedureStepFiguresParams{
		WorkspaceID: r.ws, Since: r.since, Until: r.until, ProcedureID: r.id, Version: r.version,
	}); err != nil {
		return fmt.Errorf("count procedure steps: %w", err)
	}

	if out.Actions, err = q.ProcedureActionFigures(ctx, models.ProcedureActionFiguresParams{
		WorkspaceID: r.ws, Since: r.since, Until: r.until, ProcedureID: r.id, Version: r.version,
	}); err != nil {
		return fmt.Errorf("count procedure actions: %w", err)
	}

	for _, a := range out.Actions {
		if action, err := q.FindAction(ctx, models.FindActionParams{WorkspaceID: r.ws, ID: a.ActionID}); err == nil {
			out.ActionNames[a.ActionID] = action.Name
		}
	}

	out.Runs, err = q.ProcedureRunFacts(ctx, models.ProcedureRunFactsParams{
		WorkspaceID: r.ws, Since: r.since, Until: r.until, ProcedureID: r.id, RowLimit: 50,
	})

	return wrap("list procedure runs", err)
}

/*
Report lists every procedure that ran in a range with its numbers.
*/
func (p *Procedures) Report(ctx context.Context, m authz.Membership, from, to time.Time,
) ([]models.ReportProceduresRow, error) {
	if !m.Can("reports:read") && !m.Can("ai_actions:read") {
		return nil, exceptions.ErrForbidden
	}

	now := p.Clock.Now()
	if to.IsZero() || to.After(now) {
		to = now.Add(time.Second)
	}

	if from.IsZero() || !from.Before(to) {
		from = to.Add(-30 * 24 * time.Hour)
	}

	var out []models.ReportProceduresRow
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = models.New(tx).ReportProcedures(ctx, models.ReportProceduresParams{
			WorkspaceID: m.Workspace.ID, Since: from, Until: to, QuietBefore: new(now.Add(-quiet)),
		})

		return wrap("report procedures", err)
	})

	return out, err
}

/*
Anomaly is a run that followed a procedure and went wrong somewhere:
guardrails stepped in, an action failed, the conversation went to the
team, the customer rated it 1 or 2, or it was reopened.
*/
type Anomaly struct {
	Run     models.ProcedureRunFactsRow
	Reasons []string
}

/*
Anomalies lists a procedure's recent runs that went wrong, newest
first.
*/
func (p *Procedures) Anomalies(ctx context.Context, m authz.Membership, id ids.UUID, days int) ([]Anomaly, error) {
	if !m.Can("ai_actions:read") {
		return nil, exceptions.ErrForbidden
	}

	days = min(max(days, 1), 90)
	now := p.Clock.Now()
	var rows []models.ProcedureRunFactsRow
	var failed map[ids.UUID]bool
	err := p.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		var err error
		rows, err = q.ProcedureRunFacts(ctx, models.ProcedureRunFactsParams{
			WorkspaceID: m.Workspace.ID, Since: now.Add(-time.Duration(days) * 24 * time.Hour),
			Until: now.Add(time.Second), ProcedureID: id, RowLimit: 200,
		})
		if err != nil {
			return wrap("list procedure runs", err)
		}

		failed, err = failedRuns(ctx, q, m.Workspace.ID, rows)

		return err
	})
	if err != nil {
		return nil, err
	}

	var out []Anomaly
	for _, r := range rows {
		if reasons := anomalies(r, failed); len(reasons) > 0 {
			out = append(out, Anomaly{Run: r, Reasons: reasons})
		}
	}

	return out, nil
}

/*
failedRuns marks the runs one of whose action calls failed.
*/
func failedRuns(ctx context.Context, q *models.Queries, ws ids.UUID, rows []models.ProcedureRunFactsRow,
) (map[ids.UUID]bool, error) {
	out := map[ids.UUID]bool{}
	for _, r := range rows {
		calls, err := q.ListRunActionCalls(ctx, models.ListRunActionCallsParams{WorkspaceID: ws, RunID: &r.ID})
		if err != nil {
			return nil, fmt.Errorf("list run action calls: %w", err)
		}
		for _, c := range calls {
			if c.State == "failed" {
				out[r.ID] = true
			}
		}
	}

	return out, nil
}

/*
anomalies names what went wrong in one run.
*/
func anomalies(r models.ProcedureRunFactsRow, failed map[ids.UUID]bool) []string {
	var out []string
	var trace struct {
		Guards []struct {
			Type string `json:"type"`
		} `json:"guard_events"`
	}
	_ = json.Unmarshal(r.ProcedureTrace, &trace)
	for _, g := range trace.Guards {
		if g.Type != "awaiting_input" && !slices.Contains(out, g.Type) {
			out = append(out, g.Type)
		}
	}

	if failed[r.ID] {
		out = append(out, "action_error")
	}

	if r.Outcome != nil && *r.Outcome == "handoff" {
		out = append(out, "handed_off")
	}

	if r.Rating != nil && *r.Rating <= 2 {
		out = append(out, "low_rating")
	}

	if r.Reopens > 0 {
		out = append(out, "reopened")
	}

	return out
}
