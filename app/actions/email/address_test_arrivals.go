/*
|--------------------------------------------------------------------------
| Forwarding Test Arrivals
|--------------------------------------------------------------------------
|
| A test mail coming back through the forward settles its own test: it
| took this long, the address is verified, and anything odd about the
| forward is noted. A copy of a test that is already over proves
| nothing, since anyone holding an old test mail can send it again, so
| it is dropped as stale and changes nothing.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
testArrived settles the open test the mail's token belongs to and marks
the address verified, or drops the mail as stale.
*/
func (a *ProcessInboundEmail) testArrived(ctx context.Context, mail *received, token string) error {
	reason := ignoreTestStale
	steps := testSteps{Clock: a.Clock, Events: a.Events}

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		test, err := q.FindEmailAddressTestByToken(ctx, models.FindEmailAddressTestByTokenParams{
			WorkspaceID: mail.address.WorkspaceID, EmailAddressID: mail.address.ID, Token: token,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find email address test: %w", err)
		}

		now := a.Clock.Now()
		latency := latencySince(test, now)

		settled, err := steps.settle(ctx, tx, test, testEnd{
			status: TestReceived, inboundID: &mail.row.ID, latency: &latency,
			findings: receivedFindings(now.Sub(test.CreatedAt), sentSubject(test), mail.parsed.Subject),
		})
		if err != nil || !settled {
			return err
		}

		reason = ignoreForwardingTest

		return q.RecordEmailAddressInbound(ctx, models.RecordEmailAddressInboundParams{
			Now: now, WorkspaceID: mail.address.WorkspaceID, ID: mail.address.ID,
		})
	})
	if err != nil {
		return fmt.Errorf("record forwarding test: %w", err)
	}

	return a.ignore(ctx, mail.row, reason)
}
