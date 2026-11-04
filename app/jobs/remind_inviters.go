/*
|--------------------------------------------------------------------------
| Remind Inviters
|--------------------------------------------------------------------------
|
| An invitation that waits three days without an answer was likely lost
| in a spam folder, so whoever sent it hears about it, once. The sweep
| runs daily across every workspace. Each reminder is marked in the same
| transaction that queues its mail, so a sweep that runs twice, or on two
| nodes at once, never mails anyone twice. A sender who left the
| workspace is not reminded.
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

	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long an invitation waits before its sender hears, how many one pass
reads, and how often the job is tried.
*/
const (
	reminderAfter    = 3 * 24 * time.Hour
	reminderBatch    = 200
	reminderAttempts = 3
)

/*
RemindInvitersArgs carries nothing: every run sweeps everything due.
*/
type RemindInvitersArgs struct{}

/*
Kind names the job.
*/
func (RemindInvitersArgs) Kind() string {
	return "remind_inviters"
}

/*
Options try the job a few times; the next day's run catches up anyway.
*/
func (RemindInvitersArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: reminderAttempts}
}

/*
RemindInviters reminds senders of invitations nobody answered.
*/
type RemindInviters struct {
	DB    *database.DB
	Clock clock.Clock
	Mail  *mail.Manager
	Links appauth.Links
}

/*
Handle reminds every sender due a reminder, a batch at a time.
*/
func (w *RemindInviters) Handle(ctx context.Context, _ *queue.Job[RemindInvitersArgs]) error {
	for {
		now := w.Clock.Now()

		due, err := models.New(w.DB).ListInvitationsToRemind(ctx, models.ListInvitationsToRemindParams{
			SentBefore: now.Add(-reminderAfter), Now: now, MaxRows: reminderBatch,
		})
		if err != nil {
			return fmt.Errorf("list invitations to remind about: %w", err)
		}

		for _, inv := range due {
			if err := w.remind(ctx, inv); err != nil {
				return err
			}
		}

		if len(due) < reminderBatch {
			return nil
		}
	}
}

/*
remind marks the invitation reminded about and queues its sender's
mail, both or neither.
*/
func (w *RemindInviters) remind(ctx context.Context, inv models.ListInvitationsToRemindRow) error {
	return w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		marked, err := models.New(tx).MarkInvitationReminded(ctx, models.MarkInvitationRemindedParams{
			InvitationID: inv.ID, Now: w.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("mark invitation reminded: %w", err)
		}

		if marked == 0 {
			return nil
		}

		pending := w.Mail.To(mail.Address{Name: inv.InviterName, Email: inv.InviterEmail})
		if inv.InviterLocale != "" {
			pending.Locale(inv.InviterLocale)
		}

		err = pending.QueueTx(ctx, tx, appmail.InvitationPendingMail{
			Name: inv.InviterName, Workspace: inv.WorkspaceName, Email: inv.Email,
			SentOn: inv.SentAt.UTC().Format("2 January 2006"), MembersURL: w.Links.Members(),
		})
		if err != nil {
			return fmt.Errorf("queue invitation pending mail: %w", err)
		}

		return nil
	})
}
