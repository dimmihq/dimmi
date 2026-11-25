/*
|--------------------------------------------------------------------------
| Delivery Events
|--------------------------------------------------------------------------
|
| A sending provider that reports bounces and complaints itself, rather
| than by mail, names the email by the id it gave it. The email is found
| by that id and the report recorded exactly as a mailed one: only the
| addresses the email went to are touched, a bounce the provider calls
| permanent stops mail to the address for good, one it gave up on stops
| it for a few days, and a complaint stops all mail to the complainer.
|
| Providers send the same event more than once, and the same failure
| can also arrive as mail. A recipient whose outcome is already on the
| email, recorded since it was last sent, is left alone, so nothing is
| suppressed or counted twice. An event about an email nobody here sent
| is dropped.
|
*/

package email

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeliveryEvent is a sending provider's report on one email it took.
Transport names the provider, as sent emails record it, and
ProviderMessageID the id it gave the email. A bounce is Permanent when
the provider says the address will never take mail.
*/
type DeliveryEvent struct {
	Transport         string
	ProviderMessageID string
	Complaint         bool
	Permanent         bool
	Recipients        []DeliveryEventRecipient
}

/*
DeliveryEventRecipient is one address a delivery event is about, with
the status code and the receiving server's words when the provider
passed them on.
*/
type DeliveryEventRecipient struct {
	Address    string
	Status     string
	Diagnostic string
}

/*
RecordDeliveryEvent records a sending provider's bounce or complaint on
the email it is about, then pauses the workspace's sending when the
report took it past a rate.

Rules:
  - An event about an email no workspace sent changes nothing.
  - Only addresses the email went to are recorded.
  - A recipient's outcome is recorded and counted once per send.

Errors: a wrapped database error; the provider should try again.
*/
type RecordDeliveryEvent struct {
	DB         *database.DB
	Clock      clock.Clock
	Events     *events.Dispatcher
	Reputation *Reputation
}

/*
Handle records the event.
*/
func (a *RecordDeliveryEvent) Handle(ctx context.Context, e DeliveryEvent) error {
	if e.ProviderMessageID == "" || len(e.Recipients) == 0 {
		return nil
	}

	workspaceID, err := models.New(a.DB).FindOutboundEmailWorkspace(ctx, models.FindOutboundEmailWorkspaceParams{
		Transport: e.Transport, ProviderMessageID: e.ProviderMessageID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("find delivery event workspace: %w", err)
	}

	ctx = inWorkspace(ctx, workspaceID)

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		sent, err := q.LockOutboundEmailByProviderMessageID(ctx, models.LockOutboundEmailByProviderMessageIDParams{
			WorkspaceID: workspaceID, Transport: e.Transport, ProviderMessageID: e.ProviderMessageID,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find delivery event email: %w", err)
		}

		run := reportRun{clock: a.Clock, events: a.Events, reputation: a.Reputation, tx: tx, q: q, sent: sent}

		for _, r := range e.Recipients {
			if err := a.record(ctx, run, e, r); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	_, err = a.Reputation.Judge(ctx, workspaceID)

	return err
}

/*
record records one recipient of the event, unless the email did not go
to it or its outcome is already known.
*/
func (a *RecordDeliveryEvent) record(
	ctx context.Context, run reportRun, e DeliveryEvent, r DeliveryEventRecipient,
) error {
	address := lower(r.Address)
	if !sentTo(run.sent, address) {
		return nil
	}

	if e.Complaint {
		if reported(run.sent, address, string(enums.OutboundEmailStatusComplained)) {
			return nil
		}

		return run.complained(ctx, address, e.Transport)
	}

	if reported(run.sent, address, string(enums.OutboundEmailStatusBounced)) {
		return nil
	}

	failed := outcome{
		address: address, source: e.Transport, reason: enums.EmailSuppressionReasonHardBounce,
		status: recipientStatus{Code: r.Status, Diagnostic: diagnostic(r.Diagnostic), At: a.Clock.Now()},
	}
	if !e.Permanent {
		failed.reason = enums.EmailSuppressionReasonSoftBounce
	}

	return run.undelivered(ctx, failed)
}

/*
reported reports whether the outcome is already recorded for the
recipient since the email was last handed to the provider. An outcome
from before that belongs to an earlier send of the same email.
*/
func reported(sent models.OutboundEmail, address, status string) bool {
	var known map[string]recipientStatus
	if json.Unmarshal(sent.RecipientStatuses, &known) != nil {
		return false
	}

	was, ok := known[address]

	return ok && was.Status == status && (sent.SentAt == nil || !was.At.Before(*sent.SentAt))
}
