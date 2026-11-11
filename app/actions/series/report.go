/*
|--------------------------------------------------------------------------
| Series Report
|--------------------------------------------------------------------------
|
| The report says how many entered, are on the journey now, finished,
| left and why, and reached the goal; per day; and for every step how
| many reached it, wait on it now and left by each path. A message step
| adds what its broadcast counted: sent, opened, clicked, replied,
| bounced. A split adds, per variant, how many took it and reached the
| goal. Test journeys never count.
|
*/

package series

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Report is a series' numbers.
*/
type Report struct {
	Totals  models.SeriesTotalsRow
	Reasons []models.SeriesEndReasonsRow
	Days    []models.SeriesEntriesByDayRow
	Steps   map[string]*StepReport
}

/*
StepReport is one step's numbers: how many reached it and wait on it
now, how many left by each path and outcome, what its message did, and
for a split each variant's goal.
*/
type StepReport struct {
	Reached  int64
	Waiting  int64
	Paths    map[string]int64
	Outcomes map[string]int64
	Message  json.RawMessage
	Variants []models.SeriesVariantGoalsRow
}

/*
Report counts a series' journeys over the last days.
*/
func (s *Series) Report(ctx context.Context, m authz.Membership, id ids.UUID, days int) (Report, error) {
	if !m.Can("series:read") {
		return Report{}, exceptions.ErrForbidden
	}

	ws := m.Workspace.ID
	out := Report{Steps: map[string]*StepReport{}}

	err := s.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		row, err := find(ctx, q, ws, id)
		if err != nil {
			return err
		}

		if out.Totals, err = q.SeriesTotals(ctx, models.SeriesTotalsParams{WorkspaceID: ws, SeriesID: id}); err != nil {
			return wrap("count series", err)
		}

		if out.Reasons, err = q.SeriesEndReasons(ctx, models.SeriesEndReasonsParams{
			WorkspaceID: ws,
			SeriesID:    id,
		}); err != nil {
			return wrap("count reasons", err)
		}

		since := s.Clock.Now().Add(-time.Duration(max(min(days, 365), 1)) * day)
		if out.Days, err = q.SeriesEntriesByDay(ctx, models.SeriesEntriesByDayParams{
			WorkspaceID: ws, SeriesID: id, Since: since,
		}); err != nil {
			return wrap("count days", err)
		}

		return s.stepReports(ctx, q, row, &out)
	})

	return out, err
}

/*
stepReports fills the numbers of every step.
*/
func (s *Series) stepReports(ctx context.Context, q *models.Queries, row models.Series, out *Report) error {
	ws := row.WorkspaceID

	rows, err := q.SeriesNodeTotals(ctx, models.SeriesNodeTotalsParams{WorkspaceID: ws, SeriesID: row.ID})
	if err != nil {
		return wrap("count steps", err)
	}

	for _, r := range rows {
		st := out.step(r.NodeID)
		st.Reached += r.N

		if r.Status == "done" && r.PathID != "" {
			st.Paths[r.PathID] += r.N
		}

		if r.Outcome != "" {
			st.Outcomes[r.Outcome] += r.N
		}
	}

	waiting, err := q.CountSeriesTravelersByNode(ctx, models.CountSeriesTravelersByNodeParams{
		WorkspaceID: ws,
		SeriesID:    row.ID,
	})
	if err != nil {
		return wrap("count waiting", err)
	}

	for _, w := range waiting {
		out.step(w.NodeID).Waiting = w.N
	}

	return s.messageReports(ctx, q, row, out)
}

/*
messageReports adds what each message step's broadcast counted, and
each split's variants.
*/
func (s *Series) messageReports(ctx context.Context, q *models.Queries, row models.Series, out *Report) error {
	ws := row.WorkspaceID

	list, err := q.ListSeriesBroadcasts(ctx, models.ListSeriesBroadcastsParams{WorkspaceID: ws, SeriesID: &row.ID})
	if err != nil {
		return wrap("list step broadcasts", err)
	}

	for _, b := range list {
		if b.SeriesNodeID != nil {
			out.step(*b.SeriesNodeID).Message = b.Counts
		}
	}

	spec, err := SpecOf(row)
	if err != nil {
		return err
	}

	for _, n := range spec.Nodes {
		if n.Kind != KindSplit {
			continue
		}

		v, err := q.SeriesVariantGoals(ctx, models.SeriesVariantGoalsParams{
			WorkspaceID: ws, SeriesID: row.ID,
			NodeID: n.ID,
		})
		if err != nil {
			return wrap("count variants", err)
		}

		out.step(n.ID).Variants = v
	}

	return nil
}

/*
step returns the report of a step, made on first use.
*/
func (r *Report) step(id string) *StepReport {
	st, ok := r.Steps[id]
	if !ok {
		st = &StepReport{Paths: map[string]int64{}, Outcomes: map[string]int64{}}
		r.Steps[id] = st
	}

	return st
}

/*
WinnerInput picks Path as the winner of the split step Node of series
ID.
*/
type WinnerInput struct {
	ID   ids.UUID
	Node string
	Path string
}

/*
PickWinner sends everyone who reaches a split from now on down one
variant: it saves the winner on the working copy and publishes it.
*/
func (s *Series) PickWinner(ctx context.Context, m authz.Membership, in WinnerInput) (models.Series, error) {
	id, node, path := in.ID, in.Node, in.Path

	row, err := s.Show(ctx, m, id)
	if err != nil {
		return row, err
	}

	spec, err := SpecOf(row)
	if err != nil {
		return row, err
	}

	for i, n := range spec.Nodes {
		if n.ID != node || n.Kind != KindSplit {
			continue
		}

		settings := settingsOf[SplitSettings](n)
		settings.Winner = path
		spec.Nodes[i].Settings, _ = json.Marshal(settings)
	}

	save := CurrentSettings(row)
	save.Membership, save.Spec = m, &spec

	if _, err := s.Save(ctx, save); err != nil {
		return row, err
	}

	return s.Publish(ctx, PublishInput{Membership: m, ID: id, Note: "Picked the winner of " + node})
}
