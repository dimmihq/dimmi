/*
|--------------------------------------------------------------------------
| Processing WhatsApp Deliveries
|--------------------------------------------------------------------------
|
| A kept change is processed by a job, one change of a customer at a
| time, so two messages land in the order they came. Its messages go to
| the inbox and its status reports move the business's messages along:
| sent, delivered, read, never backwards, or failed with Meta's reason.
| A failure is recorded, with no content, and tried again.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Process processes a kept change: claims it, waits its turn for its
customer, handles it and records the outcome.
*/
func (w *WhatsApp) Process(ctx context.Context, workspaceID, eventID ids.UUID) error {
	ctx = tenancy.With(ctx, workspaceID.String())

	var row models.ChannelInboundEvent

	err := w.read(ctx, func(q *models.Queries) error {
		var err error

		row, err = q.StartChannelInboundEvent(ctx, models.StartChannelInboundEventParams{
			WorkspaceID: workspaceID, ID: eventID, Now: w.Clock.Now(),
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("claim whatsapp delivery: %w", err)
	}

	if row.SerialKey != nil {
		held, release, err := w.Cache.Lock(ctx, "ch:serial:"+ownerOf(row)+":"+*row.SerialKey)
		if err != nil {
			return err
		}
		defer release()

		ctx = held
	}

	handled, failure := w.handle(ctx, row)

	return w.finish(ctx, row, handled, failure)
}

/*
handle hands the change's messages to the inbox and applies its status
reports, on its live connection.
*/
func (w *WhatsApp) handle(ctx context.Context, row models.ChannelInboundEvent) (bool, error) {
	if row.ChannelConnectionID == nil {
		return false, nil
	}

	conn, err := w.connection(ctx, row.WorkspaceID, *row.ChannelConnectionID)
	if err != nil {
		if errors.Is(err, exceptions.ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	if conn.Status == "disconnected" {
		return false, nil
	}

	value, ok := readChange(row.Payload)
	if !ok {
		return false, nil
	}

	handled := false

	for _, msg := range value.Messages {
		ok, err := w.ingest(ctx, conn, value, msg)
		if err != nil {
			return handled, err
		}

		handled = handled || ok
	}

	for _, status := range value.Statuses {
		ok, err := w.applyStatus(ctx, conn, status)
		if err != nil {
			return handled, err
		}

		handled = handled || ok
	}

	return handled, nil
}

/*
finish records how processing went: processed, ignored, or failed with
the error, which the job then retries.
*/
func (w *WhatsApp) finish(ctx context.Context, row models.ChannelInboundEvent, handled bool, failure error) error {
	status, reason := enums.ChannelEventIgnored, (*string)(nil)

	switch {
	case failure != nil:
		status, reason = enums.ChannelEventFailed, new(failure.Error())
	case handled:
		status = enums.ChannelEventProcessed
	}

	err := w.read(ctx, func(q *models.Queries) error {
		return q.FinishChannelInboundEvent(ctx, models.FinishChannelInboundEventParams{
			Status: string(status), Error: reason, Now: w.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
	})

	return errors.Join(failure, err)
}

/*
applyStatus moves the business's message as far as Meta reports it got.
Only the report of a reply's first part moves the reply itself; a late
report never moves it back.
*/
func (w *WhatsApp) applyStatus(ctx context.Context, conn models.ChannelConnection, s wadriver.Status) (bool, error) {
	status, ok := deliveryStatusOf(s.Status)
	if !ok || s.ID == "" {
		return false, nil
	}

	moved := false

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		ref, err := q.UpdateChannelMessageRefStatus(ctx, models.UpdateChannelMessageRefStatusParams{
			Status: string(status), Now: w.Clock.Now(), WorkspaceID: conn.WorkspaceID,
			ChannelConnectionID: conn.ID, ExternalMessageID: s.ID,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && ref.PartIndex != 0) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("record whatsapp status: %w", err)
		}

		moved = true

		return w.moveMessage(ctx, tx, ref.MessageID, delivered{workspace: conn.WorkspaceID, status: status, report: s})
	})

	return moved, err
}

/*
delivered is how far Meta reports a message of the workspace got, with
its report.
*/
type delivered struct {
	workspace ids.UUID
	status    enums.DeliveryStatus
	report    wadriver.Status
}

/*
moveMessage records on the message how far it got, with Meta's reason
for a failure and what Meta charges, and tells open screens.
*/
func (w *WhatsApp) moveMessage(ctx context.Context, tx *database.Tx, messageID ids.UUID, d delivered) error {
	params := models.UpdateMessageDeliveryParams{
		DeliveryStatus: new(string(d.status)), Now: w.Clock.Now(), WorkspaceID: d.workspace, ID: messageID,
	}

	reason := ""
	if d.status == enums.DeliveryStatusFailed && len(d.report.Errors) > 0 {
		reason = failureReason(d.report.Errors[0])
		params.DeliveryError = &reason
	}

	if d.report.Pricing != nil {
		params.Metadata, _ = json.Marshal(map[string]any{wadriver.Type: map[string]any{"pricing": d.report.Pricing}})
	}

	msg, err := models.New(tx).UpdateMessageDelivery(ctx, params)
	if err != nil {
		return fmt.Errorf("record whatsapp delivery: %w", err)
	}

	return w.Events.Dispatch(ctx, tx, appevents.MessageDeliveryUpdated{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: msg.WorkspaceID, OccurredAt: w.Clock.Now(),
			Actor: appevents.Actor{Type: string(enums.AuthorTypeSystem), ID: ids.Nil()},
		},
		MessageRef: appevents.MessageRef{
			ConversationRef: appevents.ConversationRef{ConversationID: msg.ConversationID}, MessageID: msg.ID,
		},
		Status: string(d.status), Final: d.status == enums.DeliveryStatusFailed, Reason: reason,
	})
}

/*
deliveryStatusOf reads Meta's status word, played voice notes counting
as read.
*/
func deliveryStatusOf(word string) (enums.DeliveryStatus, bool) {
	switch word {
	case "sent":
		return enums.DeliveryStatusSent, true
	case "delivered":
		return enums.DeliveryStatusDelivered, true
	case "read", "played":
		return enums.DeliveryStatusRead, true
	case "failed":
		return enums.DeliveryStatusFailed, true
	default:
		return "", false
	}
}

/*
failureReason says why Meta could not deliver, in the channel's words
for its code.
*/
func failureReason(e wadriver.StatusError) string {
	mapped := wadriver.MapError(&wadriver.GraphError{Code: e.Code, Message: e.Message})

	if refused, ok := errors.AsType[*contracts.SendError](mapped); ok && refused.Err != nil {
		return refused.Message
	}

	if e.Title != "" {
		return e.Title
	}

	return "WhatsApp could not deliver this message."
}

/*
readChange reads a kept change's messages, reporting false for one that
is not readable.
*/
func readChange(raw []byte) (wadriver.Messages, bool) {
	var change storedChange
	if json.Unmarshal(raw, &change) != nil {
		return wadriver.Messages{}, false
	}

	return readMessages(change.Value)
}
