/*
|--------------------------------------------------------------------------
| Backfill From Resend
|--------------------------------------------------------------------------
|
| Resend keeps the mail it received, so mail missed while the install was
| down is not lost. Feeding it through again is safe: a mail the install
| already holds is recognized by its id and kept once.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/mailbox"

	appconfig "github.com/dimmihq/dimmi/config"
)

/*
ErrNotResendSource means the source named resend runs another driver.
*/
var ErrNotResendSource = errors.New("email: resend source runs another driver")

/*
BackfillResend feeds through the mail Resend received since a time.
*/
type BackfillResend struct {
	Sources *mailbox.Manager
	Receive *ReceiveInboundEmail
}

/*
Handle walks Resend's mail and returns how many emails it fed through.

Errors: ErrNotResendSource, or a wrapped error from Resend or from
keeping a mail, saying how far the walk got.
*/
func (a *BackfillResend) Handle(ctx context.Context, since time.Time) (int, error) {
	receiver, err := a.Sources.Receiver(appconfig.InboundSourceResend)
	if err != nil {
		return 0, fmt.Errorf("resend source: %w", err)
	}

	source, ok := receiver.(mailbox.Backfiller)
	if !ok {
		return 0, ErrNotResendSource
	}

	count := 0

	err = source.Backfill(ctx, since, func(ctx context.Context, received mailbox.Received) error {
		if _, err := a.Receive.Handle(ctx, *received.Envelope); err != nil {
			return err
		}

		count++

		return nil
	})
	if err != nil {
		return count, fmt.Errorf("after %d emails: %w", count, err)
	}

	return count, nil
}
