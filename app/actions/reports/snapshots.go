/*
|--------------------------------------------------------------------------
| Backlog Snapshots
|--------------------------------------------------------------------------
|
| Once a workspace's day has ended in its own time zone, the backlog is
| counted as it stands: the conversations still open or snoozed, by
| inbox and team, how many nobody holds and how long they have waited.
| What waited on a past day cannot be worked out later, so each day is
| kept.
|
*/

package reports

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
day is twenty-four hours, the unit backlog ages are counted in.
*/
const day = 24 * time.Hour

/*
Snapshot counts the backlog of every workspace whose day has ended.
*/
func (m *Maintenance) Snapshot(ctx context.Context) error {
	now := m.Builder.Clock.Now()

	due, err := models.New(m.Builder.DB).ReportSnapshotsDue(ctx, now)
	if err != nil {
		return fmt.Errorf("find due snapshots: %w", err)
	}

	var errs []error
	for _, d := range due {
		errs = append(errs, m.SnapshotDay(ctx, d.WorkspaceID, d.Day))
	}

	return errors.Join(errs...)
}

/*
SnapshotDay records the workspace's backlog as it stands, under the
day named "2006-01-02".
*/
func (m *Maintenance) SnapshotDay(ctx context.Context, ws ids.UUID, date string) error {
	now := m.Builder.Clock.Now()

	return m.read(ctx, ws, func(q *models.Queries) error {
		err := q.SnapshotBacklog(ctx, models.SnapshotBacklogParams{
			WorkspaceID: ws, SnapshotDay: date, DayAgo: now.Add(-day), DaysAgo: now.Add(-3 * day),
			WeekAgo: now.Add(-7 * day), TakenAt: now,
		})
		if err != nil {
			return fmt.Errorf("snapshot backlog: %w", err)
		}

		return q.SetReportSnapshotDay(ctx, models.SetReportSnapshotDayParams{
			WorkspaceID: ws, TakenAt: now, SnapshotDay: date,
		})
	})
}
