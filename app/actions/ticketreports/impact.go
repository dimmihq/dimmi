/*
|--------------------------------------------------------------------------
| Service Levels And Tracker Impact
|--------------------------------------------------------------------------
|
| How often the team kept its promises: for each policy and target, the
| clocks that ended in the range met or breached, and the share met. A
| clock counts on the day it ended. And which trackers and incidents
| matter most: ranked by the monthly revenue of the companies whose
| conversations they gather, then by how many conversations they hold.
|
*/

package ticketreports

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/reports"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/models"
)

/*
trackerLimit is how many trackers the ranking shows.
*/
const trackerLimit = 50

/*
Attainment is one target of one policy: clocks met and breached in the
range, the percent met, nil with none, how long met clocks took on
average in seconds, and the clocks running now.
*/
type Attainment struct {
	PolicyID   ids.UUID
	PolicyName string
	Target     string
	Met        int64
	Breached   int64
	Percent    *float64
	MetSeconds float64
	Running    int64
}

/*
SLA reports attainment per policy and target.
*/
func (r *Reports) SLA(ctx context.Context, m authz.Membership, rg Range) ([]Attainment, error) {
	var out []Attainment

	err := r.read(ctx, m, rg, func(ctx context.Context, q *models.Queries, rq reports.Query) error {
		var err error
		out, err = attainment(ctx, q, rq)

		return err
	})

	return out, err
}

/*
attainment counts the clocks that ended in the range.
*/
func attainment(ctx context.Context, q *models.Queries, rq reports.Query) ([]Attainment, error) {
	a := access(rq)

	rows, err := q.CountReportSLAClocks(ctx, models.CountReportSLAClocksParams{
		WorkspaceID: rq.Membership.Workspace.ID, RangeFrom: rq.Start, RangeTo: rq.End, AccessMode: a.Mode,
		AccessViewer: a.Viewer, AccessTeams: a.Teams, AccessUnassigned: a.Unassigned,
		AccessHiddenInboxes: a.HiddenInboxes,
	})
	if err != nil {
		return nil, fmt.Errorf("count sla clocks: %w", err)
	}

	out := make([]Attainment, 0, len(rows))
	for _, row := range rows {
		out = append(out, Attainment{
			PolicyID: row.PolicyID, PolicyName: row.PolicyName, Target: row.Target, Met: row.Met,
			Breached: row.Breached, Percent: percent(row.Met, row.Met+row.Breached), MetSeconds: row.MetSeconds,
		})
	}

	return withRunning(ctx, q, rq, out)
}

/*
withRunning adds the clocks running now to each target, and the targets
with only running clocks.
*/
func withRunning(ctx context.Context, q *models.Queries, rq reports.Query, out []Attainment) ([]Attainment, error) {
	a := access(rq)

	rows, err := q.CountReportSLARunning(ctx, models.CountReportSLARunningParams{
		WorkspaceID: rq.Membership.Workspace.ID, AccessMode: a.Mode, AccessViewer: a.Viewer, AccessTeams: a.Teams,
		AccessUnassigned: a.Unassigned, AccessHiddenInboxes: a.HiddenInboxes,
	})
	if err != nil {
		return nil, fmt.Errorf("count running sla clocks: %w", err)
	}

	for _, row := range rows {
		i := slices.IndexFunc(out, func(x Attainment) bool {
			return x.PolicyID == row.PolicyID && x.Target == row.Target
		})
		if i < 0 {
			out = append(out, Attainment{PolicyID: row.PolicyID, PolicyName: row.PolicyName, Target: row.Target})
			i = len(out) - 1
		}

		out[i].Running = row.Running
	}

	slices.SortStableFunc(out, func(x, y Attainment) int {
		return cmp.Or(cmp.Compare(x.PolicyName, y.PolicyName), cmp.Compare(x.Target, y.Target))
	})

	return out, nil
}

/*
percent returns the share the part is of the whole, nil without a
whole.
*/
func percent(part, whole int64) *float64 {
	if whole == 0 {
		return nil
	}

	v := float64(part*1000/whole) / 10

	return &v
}

/*
Tracker is one tracker or incident and what it reaches.
*/
type Tracker = models.ListReportTrackersRow

/*
Trackers ranks the trackers and incidents open in the range, or
resolved in it, by what they reach.
*/
func (r *Reports) Trackers(ctx context.Context, m authz.Membership, rg Range) ([]Tracker, error) {
	var out []Tracker

	err := r.read(ctx, m, rg, func(ctx context.Context, q *models.Queries, rq reports.Query) error {
		var err error
		out, err = trackers(ctx, q, rq)

		return err
	})

	return out, err
}

/*
trackers reads the ranking.
*/
func trackers(ctx context.Context, q *models.Queries, rq reports.Query) ([]Tracker, error) {
	a := access(rq)

	rows, err := q.ListReportTrackers(ctx, models.ListReportTrackersParams{
		RangeFrom: rq.Start, RangeTo: rq.End, WorkspaceID: rq.Membership.Workspace.ID, AccessMode: a.Mode,
		AccessViewer: a.Viewer, AccessTeams: a.Teams, AccessUnassigned: a.Unassigned,
		AccessHiddenInboxes: a.HiddenInboxes, RowLimit: trackerLimit,
	})
	if err != nil {
		return nil, fmt.Errorf("list report trackers: %w", err)
	}

	return rows, nil
}
