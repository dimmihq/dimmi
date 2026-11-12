/*
|--------------------------------------------------------------------------
| Events From The Messenger
|--------------------------------------------------------------------------
|
| The messenger reports what the visitor did on the page, from their
| browser, where anyone can forge an event. So it may send only the names
| the team opened to it, each switched on in Settings, and nothing else,
| unless the workspace lets the messenger send any name. Then a name the
| workspace has not seen is defined on its first event, already open to
| the messenger, so it keeps working when the setting goes off again; a
| name the team closes stops. The visitor's session names the person:
| the browser never picks whose summary an event lands in. The time is
| always the server's, since a browser's clock cannot be trusted, and a
| retried event with the same id within 48 hours is counted once. Events
| land in the same summaries the server API feeds.
|
*/

package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxMessengerBatch is how many events the messenger sends at once.
*/
const MaxMessengerBatch = 20

/*
browser marks a batch sent by the messenger; open is the workspace's
setting that lets it send any name.
*/
type browser struct {
	open bool
}

/*
MessengerEvent is one event the visitor's browser sent. ID, when set,
makes a retry count once.
*/
type MessengerEvent struct {
	Name     string
	Metadata map[string]any
	ID       string
}

/*
Visitor is whose browser sent a batch: the session's workspace, its
contact and the session itself.
*/
type Visitor struct {
	WorkspaceID ids.UUID
	ContactID   ids.UUID
	SessionID   ids.UUID
}

/*
FromMessenger records the events a visitor's browser sent, on the
session's contact. A name the messenger may not send is reported as
not_allowed in its result rather than refused, so one stale name in a
batch does not cost the others.
*/
func (e *Events) FromMessenger(ctx context.Context, v Visitor, sent []MessengerEvent) ([]Result, error) {
	batch := messengerBatch(v, sent)

	if issues := check(batch); len(issues) > 0 {
		return nil, ErrInvalid.With("errors", issues)
	}

	fresh, err := e.firstSeen(ctx, v.WorkspaceID, batch)
	if err != nil {
		return nil, err
	}

	results := make([]Result, len(batch))
	err = e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)
		open, err := q.FindMessengerNewEvents(ctx, v.WorkspaceID)
		if err != nil {
			return fmt.Errorf("find messenger event setting: %w", err)
		}

		r := run{
			q: q, tx: tx, dispatcher: e.Dispatcher, ws: v.WorkspaceID, now: e.Clock.Now(),
			defs: map[string]*models.EventDefinition{}, browser: &browser{open: open},
		}
		for i, ev := range batch {
			if !fresh[i] {
				results[i] = Result{Status: StatusDuplicate}
				continue
			}

			if results[i], err = r.record(ctx, ev); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		e.forget(ctx, v.WorkspaceID, batch)
		return nil, err
	}

	return results, nil
}

/*
messengerBatch turns what the browser sent into events on the session's
contact, keyed for idempotency by the session and the browser's own id.
*/
func messengerBatch(v Visitor, sent []MessengerEvent) []Event {
	batch := make([]Event, len(sent))
	for i, s := range sent {
		contact := v.ContactID
		batch[i] = Event{Name: s.Name, ContactID: &contact, Metadata: s.Metadata}
		if s.ID != "" {
			batch[i].IdempotencyKey = "messenger:" + v.SessionID.String() + ":" + s.ID
		}
	}

	return batch
}

/*
MessengerSetting reports whether the messenger may send any event name.
*/
func (e *Events) MessengerSetting(ctx context.Context, m authz.Membership) (bool, error) {
	if !m.Can("contacts:read") {
		return false, exceptions.ErrForbidden
	}

	var open bool
	err := e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		open, err = models.New(tx).FindMessengerNewEvents(ctx, m.Workspace.ID)
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		return wrap("find messenger event setting", err)
	})

	return open, err
}

/*
SetMessengerSetting lets the messenger send any event name, or only the
names opened to it.
*/
func (e *Events) SetMessengerSetting(ctx context.Context, m authz.Membership, open bool) error {
	if !m.Can("attributes:manage") {
		return exceptions.ErrForbidden
	}

	return e.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		err := models.New(tx).SetMessengerNewEvents(ctx, models.SetMessengerNewEventsParams{
			Allowed: open, Now: e.Clock.Now(), WorkspaceID: m.Workspace.ID,
		})

		return wrap("set messenger event setting", err)
	})
}
