/*
|--------------------------------------------------------------------------
| Receiving Messenger And Instagram Deliveries
|--------------------------------------------------------------------------
|
| Meta posts every Page's and Instagram account's events to one address
| and signs each delivery with the app's secret. Nothing is read before
| the signature holds. Each event is then kept for the connection whose
| account it happened on and queued, one customer's events in order; an
| event for an account no workspace connected is dropped. Meta delivers
| an event again when it hears nothing within seconds, so the same event
| twice is kept once.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/drivers/channel/meta"
	"github.com/dimmihq/dimmi/app/drivers/channel/social"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
SocialDelivery is one request from Meta: the signature header and the
raw body Meta signed.
*/
type SocialDelivery struct {
	Signature string
	Body      []byte
}

/*
SocialCheck is Meta's check of the webhook address: the mode, the token
it carries and the challenge to echo.
*/
type SocialCheck struct {
	Mode      string
	Token     string
	Challenge string
}

/*
Verify answers Meta's check of the address: the challenge when the
token is the install's, false otherwise.
*/
func (s *Social) Verify(check SocialCheck) (string, bool) {
	expected := s.App.VerifyToken
	if check.Mode != "subscribe" || check.Challenge == "" || expected == "" ||
		!hashing.Equal([]byte(expected), []byte(check.Token)) {
		return "", false
	}

	return check.Challenge, true
}

/*
Receive checks the delivery and keeps each of the platform's events for
the connection whose account it happened on.
*/
func (s *Social) Receive(ctx context.Context, d SocialDelivery) (Outcome, error) {
	if s.App.AppSecret == "" {
		return Unknown, nil
	}

	if !meta.Verify(s.App.AppSecret, d.Signature, d.Body) {
		return Unsigned, nil
	}

	events, err := social.ParseDelivery(d.Body)
	if err != nil {
		return Unreadable, nil //nolint:nilerr // a body that is not a delivery is answered, not retried
	}

	for _, event := range events {
		if event.Platform != s.Platform {
			continue
		}

		if err := s.keepEvent(ctx, event); err != nil {
			return Accepted, err
		}
	}

	return Accepted, nil
}

/*
keepEvent keeps one event for the connection holding its account.
*/
func (s *Social) keepEvent(ctx context.Context, event social.Event) error {
	if event.Account == "" || event.Customer == "" {
		return nil
	}

	found, err := models.New(s.DB).RouteChannelConnection(ctx, models.RouteChannelConnectionParams{
		ConnectionType: string(s.Platform), ProviderKey: event.Account,
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("route %s delivery: %w", s.Platform, err)
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode %s event: %w", s.Platform, err)
	}

	return s.keep(ctx, routed{
		workspaceID: found.WorkspaceID, connectionID: found.ChannelConnectionID, suspended: found.Suspended,
	}, stored{
		transport: TransportWebhook, dedupeKey: hashing.SHA256Hex(payload), kind: eventKind(event), payload: payload,
		serialKey: event.Account + ":" + social.ThreadKey(event.Customer),
	})
}

/*
eventKind names what an event is, for reading kept deliveries.
*/
func eventKind(event social.Event) string {
	switch {
	case event.Message != nil:
		return "message"
	case event.Reaction != nil:
		return "reaction"
	case event.Read != nil:
		return "read"
	case event.Edit != nil:
		return "edit"
	case event.Postback != nil:
		return "postback"
	case event.Control != nil:
		return "control"
	default:
		return "other"
	}
}

/*
keep keeps the event for the connection and queues its processing, a
suspended workspace's held until it is back.
*/
func (s *Social) keep(ctx context.Context, r routed, d stored) error {
	ctx = tenancy.With(ctx, r.workspaceID.String())

	seen := "ch:dedupe:" + string(s.Platform) + ":" + r.workspaceID.String() + ":" + d.dedupeKey
	if _, err := s.Cache.Get(ctx, seen); err == nil {
		return nil
	}

	status := enums.ChannelEventPending
	if r.suspended {
		status = enums.ChannelEventHeld
	}

	err := s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).CreateChannelInboundEvent(ctx, models.CreateChannelInboundEventParams{
			WorkspaceID: r.workspaceID, ChannelConnectionID: &r.connectionID, Provider: string(s.Platform),
			Transport: d.transport, DedupeKey: d.dedupeKey, SerialKey: optional(d.serialKey), Kind: d.kind,
			Payload: json.RawMessage(d.payload), Status: string(status), Now: s.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("keep %s delivery: %w", s.Platform, err)
		}

		if status == enums.ChannelEventHeld {
			return nil
		}

		return s.Queue.PushTx(ctx, tx, jobs.ProcessSocialEventArgs{
			Platform: string(s.Platform), WorkspaceID: r.workspaceID, EventID: row.ID,
		})
	})
	if err != nil {
		return err
	}

	_ = s.Cache.Set(ctx, seen, []byte{1}, s.Config.DedupeWindow)

	return nil
}
