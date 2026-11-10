/*
|--------------------------------------------------------------------------
| The Hourly Sweep
|--------------------------------------------------------------------------
|
| Once an hour every scheduled or sending broadcast gets its job back in
| case it was lost, which costs nothing when it was not, since each job
| runs once per broadcast at a time. The sweep also checks goals: a
| recipient who did what the broadcast hoped for, within the goal's
| window after it reached them, counts as converted.
|
*/

package broadcasts

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Goal is what a broadcast hopes recipients do: an event they send, within
a number of days of getting it.
*/
type Goal struct {
	Event string `json:"event"`
	Days  int    `json:"days"`
}

/*
sweep restarts the workspace's active broadcasts and checks goals.
*/
func (b *Broadcasts) sweep(ctx context.Context, ws ids.UUID) error {
	var active []models.Broadcast

	err := b.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		var err error

		active, err = q.ListActiveBroadcasts(ctx, ws)

		return wrap("list active broadcasts", err)
	})
	if err != nil {
		return err
	}

	for _, row := range active {
		if err := b.restart(ctx, row); err != nil {
			return err
		}
	}

	return b.convert(ctx, ws)
}

/*
convert counts the recipients who reached their broadcast's goal, for
every broadcast whose goal window is still open.
*/
func (b *Broadcasts) convert(ctx context.Context, ws ids.UUID) error {
	return b.inWorkspace(ctx, ws, func(ctx context.Context, q *models.Queries) error {
		rows, err := q.ListGoalBroadcasts(ctx, models.ListGoalBroadcastsParams{
			WorkspaceID: ws, Since: b.Clock.Now().Add(-maxGoalWindow),
		})
		if err != nil {
			return wrap("list goal broadcasts", err)
		}

		for _, row := range rows {
			if err := b.convertOne(ctx, q, row); err != nil {
				return err
			}
		}

		return nil
	})
}

/*
convertOne marks the recipients of one broadcast who reached its goal.
*/
func (b *Broadcasts) convertOne(ctx context.Context, q *models.Queries, row models.Broadcast) error {
	g, ok := goalOf(row)
	if !ok {
		return nil
	}

	n, err := q.ConvertBroadcastRecipients(ctx, models.ConvertBroadcastRecipientsParams{
		WorkspaceID: row.WorkspaceID, BroadcastID: row.ID, Event: g.Event, Days: int32Of(min(g.Days, 90)),
		Now: b.Clock.Now(),
	})
	if err != nil || n == 0 {
		return wrap("convert recipients", err)
	}

	return b.recount(ctx, row.WorkspaceID, row.ID)
}

/*
restart pushes the job a scheduled or sending broadcast waits on.
*/
func (b *Broadcasts) restart(ctx context.Context, row models.Broadcast) error {
	if row.Status == StatusScheduled && row.ScheduledAt != nil {
		args := jobs.StartBroadcastArgs{WorkspaceID: row.WorkspaceID, BroadcastID: row.ID}

		return b.Queue.Push(ctx, args, queue.At(startAt(row, b.Clock.Now())))
	}

	return b.Queue.Push(ctx, jobs.DispatchBroadcastArgs{WorkspaceID: row.WorkspaceID, BroadcastID: row.ID})
}

/*
goalOf reads a broadcast's goal; ok is false when it has none.
*/
func goalOf(row models.Broadcast) (Goal, bool) {
	var g Goal
	if len(row.Goal) == 0 || json.Unmarshal(row.Goal, &g) != nil || g.Event == "" {
		return g, false
	}

	if g.Days <= 0 {
		g.Days = 7
	}

	return g, true
}

/*
maxGoalWindow is the longest a goal waits after a broadcast started.
*/
const maxGoalWindow = 91 * 24 * time.Hour
