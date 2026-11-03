/*
|--------------------------------------------------------------------------
| Notify Expiring Tokens
|--------------------------------------------------------------------------
|
| A token that expires stops every script that uses it, so its owner is
| warned a week ahead, once. The sweep runs daily across every
| workspace. Each warning is marked in the same transaction that queues
| its mail, so a sweep that runs twice, or on two nodes at once, never
| mails anyone twice, and one that fails part way picks up where it
| stopped.
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
The warning window, how many tokens one pass reads, and how often the
job is tried.
*/
const (
	expiryWarningAhead = 7 * 24 * time.Hour
	expiryBatch        = 200
	expiryAttempts     = 3
)

/*
NotifyExpiringTokensArgs carries nothing: every run sweeps everything
due.
*/
type NotifyExpiringTokensArgs struct{}

/*
Kind names the job.
*/
func (NotifyExpiringTokensArgs) Kind() string {
	return "notify_expiring_tokens"
}

/*
Options try the job a few times; the next day's run catches up anyway.
*/
func (NotifyExpiringTokensArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: expiryAttempts}
}

/*
NotifyExpiringTokens warns owners of tokens that expire within a week.
*/
type NotifyExpiringTokens struct {
	DB    *database.DB
	Clock clock.Clock
	Mail  *mail.Manager
	Links appauth.Links
}

/*
Handle warns every owner due a warning, a batch at a time.
*/
func (w *NotifyExpiringTokens) Handle(ctx context.Context, _ *queue.Job[NotifyExpiringTokensArgs]) error {
	for {
		now := w.Clock.Now()

		due, err := models.New(w.DB).ListTokensExpiringSoon(ctx, models.ListTokensExpiringSoonParams{
			ExpiringAfter: now, ExpiringBefore: now.Add(expiryWarningAhead), MaxRows: expiryBatch,
		})
		if err != nil {
			return fmt.Errorf("list expiring tokens: %w", err)
		}

		for _, tok := range due {
			if err := w.warn(ctx, tok); err != nil {
				return err
			}
		}

		if len(due) < expiryBatch {
			return nil
		}
	}
}

/*
warn marks the token warned and queues its owner's mail, both or
neither.
*/
func (w *NotifyExpiringTokens) warn(ctx context.Context, tok models.ListTokensExpiringSoonRow) error {
	return w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		marked, err := models.New(tx).MarkTokenExpiryNotified(ctx, models.MarkTokenExpiryNotifiedParams{
			TokenID: tok.ID, Now: w.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("mark token warned: %w", err)
		}

		if marked == 0 {
			return nil
		}

		pending := w.Mail.To(mail.Address{Name: tok.UserName, Email: tok.UserEmail})
		if tok.UserLocale != "" {
			pending.Locale(tok.UserLocale)
		}

		err = pending.QueueTx(ctx, tx, appmail.TokenExpiringMail{
			Name: tok.UserName, Token: tok.Name, Workspace: tok.WorkspaceName,
			ExpiresOn: tok.ExpiresAt.UTC().Format("2 January 2006"), TokensURL: w.Links.Tokens(),
		})
		if err != nil {
			return fmt.Errorf("queue token expiring mail: %w", err)
		}

		return nil
	})
}
