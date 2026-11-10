/*
|--------------------------------------------------------------------------
| Keeping And Processing Deliveries
|--------------------------------------------------------------------------
|
| A delivery is kept once per workspace: Slack's retries, and the same
| event over a socket and a webhook, hit the same key and are dropped,
| first in Valkey, then for good on the table's unique key. A suspended
| workspace's deliveries are kept but held, to be processed when it is
| back. Each kept delivery queues its job in the same transaction.
|
| Processing runs one delivery of a thread at a time, so two messages in
| one thread land in the order they came. What the router handles is
| processed; what nobody claims is ignored; a failure is recorded, with
| no content, and tried again.
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

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
stored is a delivery to keep: how it came, the key that recognizes it,
the thread it belongs to, its kind and its payload.
*/
type stored struct {
	transport string
	dedupeKey string
	serialKey string
	kind      string
	payload   []byte
}

/*
keep keeps the delivery for the connection and queues its processing.
*/
func (s *Slack) keep(ctx context.Context, r routed, d stored) error {
	ctx = tenancy.With(ctx, r.workspaceID.String())
	seen := "ch:dedupe:slack:" + r.workspaceID.String() + ":" + d.dedupeKey

	if _, err := s.Cache.Get(ctx, seen); err == nil {
		return nil
	}

	status := enums.ChannelEventPending
	if r.suspended {
		status = enums.ChannelEventHeld
	}

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).CreateChannelInboundEvent(ctx, models.CreateChannelInboundEventParams{
			WorkspaceID: r.workspaceID, ChannelConnectionID: r.connectionID, Provider: slackdriver.Type,
			Transport: d.transport, DedupeKey: d.dedupeKey, SerialKey: optional(d.serialKey), Kind: d.kind,
			Payload: json.RawMessage(d.payload), Status: string(status), Now: s.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("keep slack delivery: %w", err)
		}

		if status == enums.ChannelEventHeld {
			return nil
		}

		return s.Queue.PushTx(ctx, tx, jobs.ProcessChannelInboundEventArgs{WorkspaceID: r.workspaceID, EventID: row.ID})
	})
	if err != nil {
		return err
	}

	_ = s.Cache.Set(ctx, seen, []byte{1}, s.Config.DedupeWindow)

	return nil
}

/*
Process processes a kept delivery: claims it, waits its turn in its
thread, hands it to the router and records the outcome.
*/
func (s *Slack) Process(ctx context.Context, workspaceID, eventID ids.UUID) error {
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
		return fmt.Errorf("claim slack delivery: %w", err)
	}

	if row.SerialKey != nil {
		held, release, err := s.Cache.Lock(ctx, "ch:serial:"+row.ChannelConnectionID.String()+":"+*row.SerialKey)
		if err != nil {
			return err
		}
		defer release()

		ctx = held
	}

	handled, err := s.dispatch(ctx, row)

	return s.finish(ctx, row, handled, err)
}

/*
dispatch hands the delivery to the router, on its live connection.
*/
func (s *Slack) dispatch(ctx context.Context, row models.ChannelInboundEvent) (bool, error) {
	conn, err := s.connection(ctx, row.WorkspaceID, row.ChannelConnectionID)
	if err != nil {
		return false, err
	}

	if conn.DeletedAt != nil || conn.Status == "disconnected" {
		return false, nil
	}

	if row.Transport == "interaction" {
		in, ok := readInteraction(row.Payload)
		if !ok {
			return false, nil
		}

		return s.Router.DispatchInteraction(ctx, SlackInteraction{Connection: conn, Interaction: in})
	}

	env, ok := readEnvelope(row.Payload)
	if !ok {
		return false, nil
	}

	ev, ok := readEvent(env.Event)
	if !ok {
		return false, nil
	}

	return s.Router.DispatchEvent(ctx, SlackEvent{Connection: conn, Envelope: env, Event: ev})
}

/*
finish records how processing went: processed, ignored, or failed with
the error, which the job then retries.
*/
func (s *Slack) finish(ctx context.Context, row models.ChannelInboundEvent, handled bool, failure error) error {
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
readEnvelope reads an Events API delivery, reporting false for one that
is not.
*/
func readEnvelope(raw []byte) (slackdriver.Envelope, bool) {
	env, err := slackdriver.ParseEnvelope(raw)

	return env, err == nil
}

/*
readEvent reads an event, reporting false for one that is not.
*/
func readEvent(raw []byte) (slackdriver.Event, bool) {
	ev, err := slackdriver.ParseEvent(raw)

	return ev, err == nil
}

/*
readInteraction reads an interaction, reporting false for one that is
not.
*/
func readInteraction(raw []byte) (slackdriver.Interaction, bool) {
	in, err := slackdriver.ParseInteraction(raw)

	return in, err == nil
}

/*
optional returns a pointer to the text, nil when it is empty.
*/
func optional(text string) *string {
	if text == "" {
		return nil
	}

	return &text
}

/*
ignoreUnreadable drops an error about input that names nothing, such as
a malformed id in a button: there is nothing to act on.
*/
func ignoreUnreadable(error) error {
	return nil
}

/*
ignoreMissing returns nil for a row that is not there, and the error
otherwise.
*/
func ignoreMissing(err error) error {
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	return err
}
