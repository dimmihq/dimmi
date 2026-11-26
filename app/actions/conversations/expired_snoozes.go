/*
|--------------------------------------------------------------------------
| Expired Snoozes
|--------------------------------------------------------------------------
|
| A snooze that runs out opens its conversation again on its own. Every
| minute a sweep reads each workspace's snoozes that ended, the earliest
| first, a batch at a time inside that workspace, and wakes them through
| the writer, so the timeline says so and conversation.unsnoozed goes
| out with the reason expired. Rows another worker holds are skipped, so
| two sweeps running at once never wake a conversation twice.
|
*/

package conversations

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ReasonExpired is why a conversation wakes when its snooze runs out.
*/
const ReasonExpired = "expired"

/*
snoozeActor is how the timeline names the sweep that wakes a snooze.
*/
const snoozeActor = "Snooze timer"

/*
wakeBatch is how many conversations one transaction wakes.
*/
const wakeBatch = 200

/*
WakeExpiredSnoozes opens the conversations whose snooze ran out, and
logs how many each workspace woke when Log is set.
*/
type WakeExpiredSnoozes struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Log    log.Logger
}

/*
Sweep wakes every workspace's expired snoozes.
*/
func (s *WakeExpiredSnoozes) Sweep(ctx context.Context) error {
	workspaces, err := models.New(s.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to wake snoozes: %w", err)
	}

	for _, ws := range workspaces {
		if err := s.wakeWorkspace(ctx, ws.ID); err != nil {
			return err
		}
	}

	return nil
}

/*
wakeWorkspace wakes the workspace's expired snoozes, a batch at a time,
until none is left.
*/
func (s *WakeExpiredSnoozes) wakeWorkspace(ctx context.Context, ws ids.UUID) error {
	ctx = tenancy.With(ctx, ws.String())
	total := 0

	for {
		woken, err := s.wakeBatch(ctx, ws)
		if err != nil {
			return fmt.Errorf("wake snoozes in %s: %w", ws, err)
		}

		total += woken
		if woken < wakeBatch {
			break
		}
	}

	if total > 0 && s.Log != nil {
		s.Log.InfoContext(ctx, "expired snoozes woke",
			log.String("workspace_id", ws.String()), log.Int("conversations", total))
	}

	return nil
}

/*
wakeBatch locks a batch of the workspace's expired snoozes and opens
them, in one transaction, and returns how many it woke.
*/
func (s *WakeExpiredSnoozes) wakeBatch(ctx context.Context, ws ids.UUID) (int, error) {
	var woken int

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w := &Writer{
			Q: models.New(tx), Tx: tx, Events: s.Events, Now: s.Clock.Now(),
			Actor: Actor{Type: string(enums.AuthorTypeSystem), Name: snoozeActor},
		}

		due, err := w.Q.LockExpiredSnoozes(ctx, models.LockExpiredSnoozesParams{
			WorkspaceID: ws, Now: w.Now, RowLimit: wakeBatch,
		})
		if err != nil {
			return fmt.Errorf("lock expired snoozes: %w", err)
		}

		for _, c := range due {
			if _, _, err := w.ChangeStatus(ctx, c, StatusChange{
				Status: string(enums.ConversationStatusOpen), Reason: ReasonExpired,
			}); err != nil {
				return err
			}
		}

		woken = len(due)

		return nil
	})

	return woken, err
}
