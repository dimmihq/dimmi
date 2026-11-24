/*
|--------------------------------------------------------------------------
| Workflow Analytics
|--------------------------------------------------------------------------
|
| How one workflow did over a range of days, for all its versions or
| one, so two versions can be put side by side. Only live runs that did
| something count. Totals say how many runs started, how many the
| customer wrote during, completed, still go, went to a person, failed
| and reached the workflow's goal. Per step: how many runs reached it,
| wait on it now, failed on it, ended on it without completing (the
| drop-off), and left by each path. Failures lists the runs where a
| call, an action, a classify step or the run itself failed, with the
| conversation to open.
|
*/

package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/workflowspec"
)

/*
maxRangeDays bounds how far back analytics read.
*/
const maxRangeDays = 365

/*
MetricsScope is the range analytics read: From up to To, the last 30
days when unset, and one version number or, with zero, all.
*/
type MetricsScope struct {
	From, To time.Time
	Version  int32
}

/*
Metrics are a workflow's numbers over a range.
*/
type Metrics struct {
	From, To time.Time
	Version  int32
	Goal     *workflowspec.Goal
	Figures  models.WorkflowRunFiguresRow
	Days     []models.WorkflowRunsByDayRow
	Steps    map[string]*StepMetrics
}

/*
StepMetrics is one step's numbers.
*/
type StepMetrics struct {
	Reached, Waiting, Failed, Dropped int32
	Paths                             map[string]int32
}

/*
Metrics reads a workflow's analytics over a range.
*/
func (w *Workflows) Metrics(ctx context.Context, m authz.Membership, id ids.UUID, scope MetricsScope,
) (Metrics, error) {
	if !m.Can("workflows:read") {
		return Metrics{}, exceptions.ErrForbidden
	}

	from, to := w.rangeOf(scope.From, scope.To)
	out := Metrics{From: from, To: to, Version: scope.Version, Steps: map[string]*StepMetrics{}}
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		wf, err := find(ctx, q, m.Workspace.ID, id)
		if err != nil {
			return err
		}

		out.Goal = goalOf(ctx, q, wf)

		return readMetrics(ctx, q, wf, &out)
	})

	return out, err
}

/*
readMetrics fills a workflow's numbers.
*/
func readMetrics(ctx context.Context, q *models.Queries, wf models.Workflow, out *Metrics) error {
	ws, id, since, until, version := wf.WorkspaceID, wf.ID, out.From, out.To, out.Version
	p := models.WorkflowRunFiguresParams{
		WorkspaceID: ws, WorkflowID: id, Since: since, Until: until, Version: version,
	}

	if g := out.Goal; g != nil {
		p.GoalKind, p.GoalMinRating = g.Kind, int32(g.MinRating) //nolint:gosec // checked to be 1 to 5
		p.GoalWindowSeconds = int32(g.WindowOf().Seconds())
	}

	var err error
	if out.Figures, err = q.WorkflowRunFigures(ctx, p); err != nil {
		return fmt.Errorf("count workflow runs: %w", err)
	}

	if out.Days, err = q.WorkflowRunsByDay(ctx, models.WorkflowRunsByDayParams{
		WorkspaceID: ws, WorkflowID: id, Since: since, Until: until, Version: version,
	}); err != nil {
		return fmt.Errorf("count workflow runs by day: %w", err)
	}

	steps, err := q.WorkflowStepFigures(ctx, models.WorkflowStepFiguresParams{
		WorkspaceID: ws, WorkflowID: id, Since: since, Until: until, Version: version,
	})
	if err != nil {
		return fmt.Errorf("count workflow steps: %w", err)
	}

	for _, s := range steps {
		st := out.step(s.StepID)
		st.Reached, st.Waiting, st.Failed = s.Reached, s.Waiting, s.Failed
	}

	paths, err := q.WorkflowStepPaths(ctx, models.WorkflowStepPathsParams{
		WorkspaceID: ws, WorkflowID: id, Since: since, Until: until, Version: version,
	})
	if err != nil {
		return fmt.Errorf("count workflow paths: %w", err)
	}

	for _, s := range paths {
		out.step(s.StepID).Paths[s.PathID] = s.Runs
	}

	drops, err := q.WorkflowStepDropoffs(ctx, models.WorkflowStepDropoffsParams{
		WorkspaceID: ws, WorkflowID: id, Since: since, Until: until, Version: version,
	})
	if err != nil {
		return fmt.Errorf("count workflow drop-offs: %w", err)
	}

	for _, s := range drops {
		out.step(s.StepID).Dropped = s.Dropped
	}

	return nil
}

/*
step returns the numbers of a step, made on first use.
*/
func (m *Metrics) step(id string) *StepMetrics {
	st, ok := m.Steps[id]
	if !ok {
		st = &StepMetrics{Paths: map[string]int32{}}
		m.Steps[id] = st
	}

	return st
}

/*
goalOf reads the goal of the live version, or of the working copy when
the workflow never went live.
*/
func goalOf(ctx context.Context, q *models.Queries, wf models.Workflow) *workflowspec.Goal {
	raw := wf.Draft
	if wf.LiveVersionID != nil {
		if v, err := q.FindWorkflowVersion(ctx, models.FindWorkflowVersionParams{
			WorkspaceID: wf.WorkspaceID, ID: *wf.LiveVersionID,
		}); err == nil {
			raw = v.Spec
		}
	}

	spec, err := workflowspec.Parse(raw)
	if err != nil {
		return nil
	}

	return spec.Goal
}

/*
rangeOf bounds a range to the last year, the last 30 days for none.
*/
func (w *Workflows) rangeOf(from, to time.Time) (time.Time, time.Time) {
	now := w.Clock.Now()
	if to.IsZero() || to.After(now) {
		to = now.Add(time.Second)
	}

	if from.IsZero() || !from.Before(to) {
		from = to.Add(-30 * 24 * time.Hour)
	}

	if oldest := to.Add(-maxRangeDays * 24 * time.Hour); from.Before(oldest) {
		from = oldest
	}

	return from, to
}

/*
FailuresScope narrows the failure list: a range, one step, and a page
before a run.
*/
type FailuresScope struct {
	From, To time.Time
	StepID   string
	Before   *ids.UUID
	Limit    int
}

/*
Failures lists a workflow's runs that failed, or one of whose steps
failed, newest first.
*/
func (w *Workflows) Failures(ctx context.Context, m authz.Membership, id ids.UUID, scope FailuresScope,
) ([]models.WorkflowFailuresRow, error) {
	if !m.Can("workflows:read") {
		return nil, exceptions.ErrForbidden
	}

	rows := int32(25)
	if scope.Limit > 0 && scope.Limit <= maxRunsListed {
		rows = int32(scope.Limit) //nolint:gosec // bounded above
	}

	from, to := w.rangeOf(scope.From, scope.To)
	var out []models.WorkflowFailuresRow
	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		if _, err := find(ctx, q, m.Workspace.ID, id); err != nil {
			return err
		}

		p := models.WorkflowFailuresParams{
			WorkspaceID: m.Workspace.ID, WorkflowID: id, Since: from, Until: to, Before: scope.Before, RowLimit: rows,
		}
		if scope.StepID != "" {
			p.StepID = &scope.StepID
		}

		var err error
		if out, err = q.WorkflowFailures(ctx, p); err != nil {
			return fmt.Errorf("list workflow failures: %w", err)
		}

		return nil
	})

	return out, err
}
