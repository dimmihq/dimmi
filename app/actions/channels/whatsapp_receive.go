/*
|--------------------------------------------------------------------------
| Receiving WhatsApp Deliveries
|--------------------------------------------------------------------------
|
| Meta wants an answer within seconds and retries for days otherwise, so
| a delivery is only checked, kept and queued here; the work happens in
| a job. Nothing is read before the signature is proven with the secret
| of the app it claims to come from: the install's own Meta app on the
| shared address, a workspace's own app on its connection's address.
| Before a webhook goes live Meta checks the address once, echoing a
| challenge when the verify token it carries is the right one.
|
| Each change in a delivery is kept on its own, for the connection whose
| number it reached; a change for a number nobody connected is answered
| with success and dropped, so Meta stops retrying something that can
| never be ours. Changes about one customer are processed one at a time,
| in the order they came.
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
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	wadriver "github.com/dimmihq/dimmi/app/drivers/channel/whatsapp"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WhatsAppDelivery is one request from Meta. ConnectionID names the
connection of a workspace's own app's address, nil on the shared one.
Body is the raw body Meta signed.
*/
type WhatsAppDelivery struct {
	ConnectionID *ids.UUID
	Signature    string
	Body         []byte
}

/*
WhatsAppCheck is Meta's check of a webhook address: the mode, the token
it carries and the challenge to echo.
*/
type WhatsAppCheck struct {
	ConnectionID *ids.UUID
	Mode         string
	Token        string
	Challenge    string
}

/*
Verify answers Meta's check of an address: the challenge when the token
is the one the address expects, false otherwise.
*/
func (w *WhatsApp) Verify(ctx context.Context, check WhatsAppCheck) (string, bool, error) {
	if check.Mode != "subscribe" || check.Token == "" || check.Challenge == "" {
		return "", false, nil
	}

	expected := w.Service.VerifyToken

	if check.ConnectionID != nil {
		r, found, err := w.route(ctx, check.ConnectionID, "")
		if err != nil || !found {
			return "", false, err
		}

		conn, err := w.connection(tenancy.With(ctx, r.workspaceID.String()), r.workspaceID, r.connectionID)
		if err != nil {
			return "", false, ignoreMissing(err)
		}

		secrets, err := w.open(ctx, conn)
		if err != nil {
			return "", false, err
		}

		expected = secrets.VerifyToken
	}

	if expected == "" || !hashing.Equal([]byte(expected), []byte(check.Token)) {
		return "", false, nil
	}

	return check.Challenge, true, nil
}

/*
Receive checks the delivery and keeps each change for the connection
whose number it reached.
*/
func (w *WhatsApp) Receive(ctx context.Context, d WhatsAppDelivery) (Outcome, error) {
	by, err := w.signerOf(ctx, d.ConnectionID)
	if err != nil || !by.found {
		return Unknown, err
	}

	if !wadriver.Verify(by.secret, d.Signature, d.Body) {
		return Unsigned, nil
	}

	delivery, ok := readDelivery(d.Body)
	if !ok {
		return Unreadable, nil
	}

	for _, entry := range delivery.Entry {
		for _, change := range entry.Changes {
			if err := w.keepChange(ctx, by.own, entry.ID, change); err != nil {
				return Accepted, err
			}
		}
	}

	return Accepted, nil
}

/*
signer is what an address's deliveries are checked with: the secret
they are signed with, the connection the address belongs to, nil on the
shared one, and whether the address leads anywhere.
*/
type signer struct {
	secret string
	own    *routed
	found  bool
}

/*
signerOf returns what the address's deliveries are checked with.
*/
func (w *WhatsApp) signerOf(ctx context.Context, connectionID *ids.UUID) (signer, error) {
	if connectionID == nil {
		return signer{secret: w.Service.AppSecret, found: w.Service.AppSecret != ""}, nil
	}

	r, found, err := w.route(ctx, connectionID, "")
	if err != nil || !found {
		return signer{}, err
	}

	conn, err := w.connection(tenancy.With(ctx, r.workspaceID.String()), r.workspaceID, r.connectionID)
	if err != nil {
		return signer{}, ignoreMissing(err)
	}

	secrets, err := w.open(ctx, conn)
	if err != nil {
		return signer{}, err
	}

	return signer{secret: secrets.AppSecret, own: &r, found: secrets.AppSecret != ""}, nil
}

/*
route finds the live WhatsApp connection by its id or by its number,
across every workspace.
*/
func (w *WhatsApp) route(ctx context.Context, connectionID *ids.UUID, phoneNumberID string) (routed, bool, error) {
	q := models.New(w.DB)

	var (
		found models.RouteChannelConnectionRow
		err   error
	)

	if connectionID != nil {
		var byID models.RouteChannelConnectionIDRow

		byID, err = q.RouteChannelConnectionID(ctx, models.RouteChannelConnectionIDParams{
			ConnectionType: wadriver.Type, ConnectionID: *connectionID,
		})
		found = models.RouteChannelConnectionRow(byID)
	} else {
		found, err = q.RouteChannelConnection(ctx, models.RouteChannelConnectionParams{
			ConnectionType: wadriver.Type, ProviderKey: phoneNumberID,
		})
	}

	if errors.Is(err, database.ErrNotFound) {
		return routed{}, false, nil
	}

	if err != nil {
		return routed{}, false, fmt.Errorf("route whatsapp delivery: %w", err)
	}

	return routed{
		workspaceID: found.WorkspaceID, connectionID: found.ChannelConnectionID, suspended: found.Suspended,
	}, true, nil
}

/*
keepChange keeps one change for the connection whose number it reached:
the address's own connection, or on the shared address the connection
holding the number. A change for a number of another connection than
the address's own is dropped.
*/
func (w *WhatsApp) keepChange(ctx context.Context, own *routed, accountID string, change wadriver.Change) error {
	if change.Field != wadriver.FieldMessages && change.Field != wadriver.FieldEchoes {
		return nil
	}

	value, ok := readMessages(change.Value)
	if !ok || value.Metadata.PhoneNumberID == "" {
		return nil
	}

	target, found, err := w.route(ctx, nil, value.Metadata.PhoneNumberID)
	if err != nil || !found {
		return err
	}

	if own != nil && own.connectionID != target.connectionID {
		return nil
	}

	payload, err := json.Marshal(storedChange{Account: accountID, Field: change.Field, Value: change.Value})
	if err != nil {
		return fmt.Errorf("encode whatsapp change: %w", err)
	}

	return w.keep(ctx, target, stored{
		transport: TransportWebhook, dedupeKey: change.Field + ":" + hashing.SHA256Hex(change.Value),
		serialKey: serialOf(value), kind: change.Field, payload: payload,
	})
}

/*
storedChange is a kept change: the WhatsApp account it came from, its
field and its value as Meta sent it.
*/
type storedChange struct {
	Account string          `json:"account"`
	Field   string          `json:"field"`
	Value   json.RawMessage `json:"value"`
}

/*
serialOf names the customer a change is about, so their changes are
processed in order; empty when it names none.
*/
func serialOf(v wadriver.Messages) string {
	key := v.Metadata.PhoneNumberID + ":"

	switch {
	case len(v.Contacts) > 0:
		return key + wadriver.ThreadKey(v.Contacts[0].UserID, v.Contacts[0].WaID)
	case len(v.Messages) > 0:
		return key + wadriver.ThreadKey(v.Messages[0].FromUserID, v.Messages[0].From)
	case len(v.Statuses) > 0:
		return key + wadriver.ThreadKey(v.Statuses[0].RecipientUserID, v.Statuses[0].RecipientID)
	default:
		return ""
	}
}

/*
keep keeps the change for the connection and queues its processing, a
suspended workspace's held until it is back.
*/
func (w *WhatsApp) keep(ctx context.Context, r routed, d stored) error {
	ctx = tenancy.With(ctx, r.workspaceID.String())
	seen := "ch:dedupe:whatsapp:" + r.workspaceID.String() + ":" + d.dedupeKey

	if _, err := w.Cache.Get(ctx, seen); err == nil {
		return nil
	}

	status := enums.ChannelEventPending
	if r.suspended {
		status = enums.ChannelEventHeld
	}

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).CreateChannelInboundEvent(ctx, models.CreateChannelInboundEventParams{
			WorkspaceID: r.workspaceID, ChannelConnectionID: &r.connectionID, Provider: wadriver.Type,
			Transport: d.transport, DedupeKey: d.dedupeKey, SerialKey: optional(d.serialKey), Kind: d.kind,
			Payload: json.RawMessage(d.payload), Status: string(status), Now: w.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("keep whatsapp delivery: %w", err)
		}

		if status == enums.ChannelEventHeld {
			return nil
		}

		return w.Queue.PushTx(ctx, tx, jobs.ProcessWhatsAppEventArgs{WorkspaceID: r.workspaceID, EventID: row.ID})
	})
	if err != nil {
		return err
	}

	_ = w.Cache.Set(ctx, seen, []byte{1}, w.Config.DedupeWindow)

	return nil
}

/*
readDelivery reads a delivery, reporting false for one that is not.
*/
func readDelivery(raw []byte) (wadriver.Delivery, bool) {
	d, err := wadriver.ParseDelivery(raw)

	return d, err == nil
}

/*
readMessages reads a messages change, reporting false for one that is
not.
*/
func readMessages(raw []byte) (wadriver.Messages, bool) {
	m, err := wadriver.ParseMessages(raw)

	return m, err == nil
}
