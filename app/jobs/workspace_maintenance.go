/*
|--------------------------------------------------------------------------
| Workspace Maintenance
|--------------------------------------------------------------------------
|
| The daily rounds that walk workspaces to their end. One hands every
| workspace whose time in the trash ran out to its own purge job. One
| reminds the owners of a workspace in the trash a few days before it is
| purged, once.
|
| Each round only looks and queues or mails, so a round that fails
| partway is simply run again the next time without harm.
|
*/

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
reminderNotice is how long before its purge the owners of a workspace
in the trash are reminded.
*/
const reminderNotice = 3 * 24 * time.Hour

/*
ScheduleWorkspacePurgesArgs starts the daily round that queues the
purges due.
*/
type ScheduleWorkspacePurgesArgs struct{}

/*
Kind names the job.
*/
func (ScheduleWorkspacePurgesArgs) Kind() string {
	return "schedule_workspace_purges"
}

/*
ScheduleWorkspacePurges queues a purge for every workspace whose time
in the trash ran out. A purge already queued for a workspace is not
queued twice, and one held by the retention guard is queued again on
the next round.
*/
type ScheduleWorkspacePurges struct {
	DB    *database.DB
	Clock clock.Clock
	Queue *queue.Queue
}

/*
Handle queues the purges.
*/
func (w *ScheduleWorkspacePurges) Handle(ctx context.Context, _ *queue.Job[ScheduleWorkspacePurgesArgs]) error {
	due, err := models.New(w.DB).ListWorkspacesDueForPurge(ctx, w.Clock.Now())
	if err != nil {
		return fmt.Errorf("list workspaces due for purge: %w", err)
	}

	for _, id := range due {
		if err := w.Queue.Push(ctx, PurgeWorkspaceArgs{WorkspaceID: id}); err != nil {
			return fmt.Errorf("queue purge: %w", err)
		}
	}

	return nil
}

/*
SendWorkspaceDeletionRemindersArgs starts the daily round that reminds
owners of workspaces about to be purged.
*/
type SendWorkspaceDeletionRemindersArgs struct{}

/*
Kind names the job.
*/
func (SendWorkspaceDeletionRemindersArgs) Kind() string {
	return "send_workspace_deletion_reminders"
}

/*
SendWorkspaceDeletionReminders mails the active owners of every
workspace in the trash that is purged within three days, once per
deletion.
*/
type SendWorkspaceDeletionReminders struct {
	DB    *database.DB
	Clock clock.Clock
	Mail  *mail.Manager
}

/*
Handle sends the reminders.
*/
func (w *SendWorkspaceDeletionReminders) Handle(
	ctx context.Context, _ *queue.Job[SendWorkspaceDeletionRemindersArgs],
) error {
	now := w.Clock.Now()

	due, err := models.New(w.DB).ListWorkspacesDueForReminder(ctx, models.ListWorkspacesDueForReminderParams{
		Now: now, NoticeUntil: now.Add(reminderNotice),
	})
	if err != nil {
		return fmt.Errorf("list workspaces due for a reminder: %w", err)
	}

	for _, workspace := range due {
		if err := w.remind(ctx, workspace); err != nil {
			return err
		}
	}

	return nil
}

/*
remind mails the workspace's active owners and records that they were
reminded, in one transaction.
*/
func (w *SendWorkspaceDeletionReminders) remind(ctx context.Context, ws models.Workspace) error {
	scoped := tenancy.With(ctx, ws.ID.String())

	return w.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		owners, err := q.ListActiveOwners(ctx, ws.ID)
		if err != nil {
			return fmt.Errorf("list owners: %w", err)
		}

		for _, owner := range owners {
			err := appmail.ToUser(w.Mail, owner.User).QueueTx(ctx, tx, appmail.WorkspaceDeletionReminderMail{
				Name: owner.User.Name, Workspace: ws.Name, PurgeAfter: *ws.PurgeAfter,
			})
			if err != nil {
				return fmt.Errorf("mail owner: %w", err)
			}
		}

		err = q.MarkWorkspaceReminded(ctx, models.MarkWorkspaceRemindedParams{ID: ws.ID, Now: w.Clock.Now()})
		if err != nil {
			return fmt.Errorf("record reminder: %w", err)
		}

		return nil
	})
}
