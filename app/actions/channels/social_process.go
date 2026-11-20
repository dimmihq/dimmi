/*
|--------------------------------------------------------------------------
| Processing Messenger And Instagram Events
|--------------------------------------------------------------------------
|
| A kept event is claimed, waits its turn among its customer's events,
| and is handled on its live connection. A customer's message, or a
| button they pressed, becomes an inbound message and marks when they
| last wrote. An edit corrects the message the inbox holds, and an
| unsent message is marked as deleted by its author, the way the inbox
| marks any message deleted where it was written. What the account itself sent from
| another place, such as Meta's own inbox, joins the conversation as the
| team's message. Reads, reactions and changes of which app answers are
| kept but change nothing yet.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
How long a change to a message the inbox does not hold yet waits for
the message, which may still be on its way in.
*/
const (
	socialChangeWait  = 2 * time.Minute
	socialChangeRetry = 10 * time.Second
)

/*
Process processes a kept event: claims it, waits its turn for its
customer, handles it and records the outcome.
*/
func (s *Social) Process(ctx context.Context, workspaceID, eventID ids.UUID) error {
	ctx = tenancy.With(ctx, workspaceID.String())

	var row models.ChannelInboundEvent

	err := s.read(ctx, func(q *models.Queries) error {
		var err error
		row, err = q.StartChannelInboundEvent(ctx, models.StartChannelInboundEventParams{
			WorkspaceID: workspaceID, ID: eventID, Now: s.Clock.Now(),
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("claim %s delivery: %w", s.Platform, err)
	}

	if row.SerialKey != nil {
		held, release, err := s.Cache.Lock(ctx, "ch:serial:"+ownerOf(row)+":"+*row.SerialKey)
		if err != nil {
			return err
		}

		defer release()

		ctx = held
	}

	handled, failure := s.handle(ctx, row)

	return s.finish(ctx, row, handled, failure)
}

/*
handle reads the event and hands it to what it concerns, on its live
connection.
*/
func (s *Social) handle(ctx context.Context, row models.ChannelInboundEvent) (bool, error) {
	if row.ChannelConnectionID == nil {
		return false, nil
	}

	conn, err := s.connection(ctx, row.WorkspaceID, *row.ChannelConnectionID)
	if errors.Is(err, exceptions.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	event, readable := readSocialEvent(row.Payload)
	if conn.Status == "disconnected" || !readable {
		return false, nil
	}

	switch {
	case event.Message != nil && event.Message.Deleted:
		return s.unsend(ctx, conn, event)
	case event.Message != nil:
		return s.ingest(ctx, conn, event)
	case event.Postback != nil:
		return s.pressed(ctx, conn, event)
	case event.Edit != nil:
		return s.edit(ctx, conn, event)
	default:
		return false, nil
	}
}

/*
readSocialEvent reads a kept event, reporting false for one that is not.
*/
func readSocialEvent(payload []byte) (social.Event, bool) {
	var event social.Event

	err := json.Unmarshal(payload, &event)

	return event, err == nil
}

/*
finish records how processing went: processed, ignored, or failed with
the error, which the job then retries.
*/
func (s *Social) finish(ctx context.Context, row models.ChannelInboundEvent, handled bool, failure error) error {
	status, reason := enums.ChannelEventIgnored, (*string)(nil)

	switch {
	case failure != nil:
		status, reason = enums.ChannelEventFailed, new(failure.Error())
	case handled:
		status = enums.ChannelEventProcessed
	}

	err := s.read(ctx, func(q *models.Queries) error {
		return q.FinishChannelInboundEvent(ctx, models.FinishChannelInboundEventParams{
			Status: string(status), Error: reason, Now: s.Clock.Now(), WorkspaceID: row.WorkspaceID, ID: row.ID,
		})
	})

	return errors.Join(failure, err)
}

/*
unsend marks a message the customer took back as deleted by them.
*/
func (s *Social) unsend(ctx context.Context, conn models.ChannelConnection, event social.Event) (bool, error) {
	_, err := s.Edits.MarkDeleted(ctx, contracts.InboundDeletion{
		ConnectionID: conn.ID, ChannelMessageID: event.Message.ID, Reason: conversations.ReasonSourceDeleted,
	})

	return s.changed(err, event)
}

/*
edit corrects a message the customer changed.
*/
func (s *Social) edit(ctx context.Context, conn models.ChannelConnection, event social.Event) (bool, error) {
	_, err := s.Edits.Edit(ctx, contracts.InboundEdit{
		ConnectionID: conn.ID, ChannelMessageID: event.Edit.MessageID, Body: event.Edit.Text, EditedAt: s.Clock.Now(),
	})

	return s.changed(err, event)
}

/*
changed reads a change's error. A message the inbox does not hold was
simply not handled, unless the change is moments old: then the message
may still be on its way in, and the change is tried again shortly.
*/
func (s *Social) changed(err error, event social.Event) (bool, error) {
	if !errors.Is(err, conversations.ErrInboundMessageNotFound) {
		return err == nil, err
	}

	if !event.At.IsZero() && s.Clock.Now().Sub(event.At) < socialChangeWait {
		return false, queue.RetryAfter(socialChangeRetry)
	}

	return false, nil
}
