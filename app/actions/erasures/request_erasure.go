/*
|--------------------------------------------------------------------------
| Request an Erasure
|--------------------------------------------------------------------------
|
| A privacy request is answered at once and carried out later. The
| customer disappears from every list the moment it is asked, their
| addresses are freed so a new message from them starts afresh, and the
| wipe waits out a grace period so a mistake can be undone. The freed
| addresses are kept sealed with the workspace's keys until then.
| Skipping the grace period is for someone who just confirmed their
| password.
|
*/

package erasures

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
EraseArgs names the erasure a job carries out.
*/
type EraseArgs struct {
	WorkspaceID ids.UUID `json:"workspace_id"`
	ErasureID   ids.UUID `json:"erasure_id"`
}

/*
Kind names the job.
*/
func (EraseArgs) Kind() string { return "erase_contact" }

/*
Options run one job per erasure at a time, retried until every eraser
succeeds.
*/
func (EraseArgs) Options() queue.Options {
	return queue.Options{
		MaxAttempts: 25,
		Unique: queue.UniqueOpts{ByArgs: true, ByState: []queue.State{
			queue.StateAvailable, queue.StatePending, queue.StateRetryable, queue.StateRunning, queue.StateScheduled,
		}},
	}
}

/*
stashedIdentity is an identity as the stash keeps it.
*/
type stashedIdentity struct {
	Type     string `json:"type"`
	Value    string `json:"value"`
	Verified bool   `json:"verified"`
}

/*
RequestErasureInput is who asks to erase which customer, and whether
the grace period is skipped.
*/
type RequestErasureInput struct {
	Principal  *appauth.Principal
	Membership authz.Membership
	ContactID  ids.UUID
	Immediate  bool
}

/*
RequestErasure asks for a customer to be erased.

Rules:
  - The caller needs contacts:erase; skipping the grace period also needs
    a password confirmed within ConfirmWindow.
  - A customer already being erased is not found; two requests racing
    for one customer leave one of them refused.
  - The contact is hidden and its identities removed at once; the wipe
    runs once Grace has passed.

Events: [appevents.ContactErasureRequested].

Errors: [exceptions.ErrForbidden], [fauth.ErrConfirmationRequired],
[exceptions.ErrNotFound], [exceptions.ErrErasurePending], or a wrapped
error.
*/
type RequestErasure struct {
	DB            *database.DB
	Events        *events.Dispatcher
	Clock         clock.Clock
	Queue         *queue.Queue
	Keys          encryption.KeyManager
	Grace         time.Duration
	ConfirmWindow time.Duration
}

/*
Handle records the request and returns it.
*/
func (a *RequestErasure) Handle(ctx context.Context, in RequestErasureInput) (models.ContactErasure, error) {
	if !in.Membership.Can("contacts:erase") {
		return models.ContactErasure{}, exceptions.ErrForbidden
	}

	now := a.Clock.Now()
	if in.Immediate && !fauth.Confirmed(in.Principal, now, a.ConfirmWindow) {
		return models.ContactErasure{}, fauth.ErrConfirmationRequired
	}

	due := now.Add(a.Grace)
	if in.Immediate {
		due = now
	}

	var out models.ContactErasure

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		contact, err := a.hide(ctx, tx, in)
		if err != nil {
			return err
		}

		request := models.CreateContactErasureParams{RequestedByMemberID: &in.Membership.Member.ID, ScheduledFor: due}
		if out, err = a.record(ctx, tx, contact, request); err != nil {
			return err
		}

		var when []queue.Opt
		if !in.Immediate {
			when = append(when, queue.At(due))
		}

		err = a.Queue.PushTx(ctx, tx, EraseArgs{WorkspaceID: out.WorkspaceID, ErasureID: out.ID}, when...)
		if err != nil {
			return fmt.Errorf("schedule erasure: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactErasureRequested{
			Base: base(in.Membership, now), ErasureID: out.ID, ContactID: contact.ID, Immediate: in.Immediate,
		})
	})

	return out, err
}

/*
hide finds the live contact and hides it. A contact being erased, like
the placeholder, is hidden already, so it is not found.
*/
func (a *RequestErasure) hide(ctx context.Context, tx *database.Tx, in RequestErasureInput) (models.Contact, error) {
	q, ws := models.New(tx), in.Membership.Workspace.ID

	contact, err := contacts.Survivor(ctx, q, ws, in.ContactID)
	if err != nil {
		return contact, err
	}

	err = q.SoftDeleteContact(ctx, models.SoftDeleteContactParams{Now: a.Clock.Now(), WorkspaceID: ws, ID: contact.ID})
	if err != nil {
		return contact, fmt.Errorf("hide contact: %w", err)
	}

	return contact, nil
}

/*
record seals the contact's identities into the request, which names who
asked and when it is due, and frees them.
*/
func (a *RequestErasure) record(
	ctx context.Context, tx *database.Tx, contact models.Contact, request models.CreateContactErasureParams,
) (models.ContactErasure, error) {
	q := models.New(tx)

	stash, err := a.seal(ctx, q, contact)
	if err != nil {
		return models.ContactErasure{}, err
	}

	if _, err := q.DeleteContactIdentities(ctx, models.DeleteContactIdentitiesParams{
		WorkspaceID: contact.WorkspaceID, ContactID: contact.ID,
	}); err != nil {
		return models.ContactErasure{}, fmt.Errorf("free identities: %w", err)
	}

	request.WorkspaceID, request.ContactID, request.IdentitiesStash = contact.WorkspaceID, contact.ID, stash
	request.Now = a.Clock.Now()

	row, err := q.CreateContactErasure(ctx, request)
	if database.IsUniqueViolation(err, "contact_erasures_open_key") {
		return row, exceptions.ErrErasurePending
	}

	if err != nil {
		return row, fmt.Errorf("record erasure: %w", err)
	}

	return row, nil
}

/*
seal returns the contact's identities encrypted for the workspace.
*/
func (a *RequestErasure) seal(ctx context.Context, q *models.Queries, contact models.Contact) ([]byte, error) {
	rows, err := q.ListContactIdentities(ctx, models.ListContactIdentitiesParams{
		WorkspaceID: contact.WorkspaceID, ContactID: contact.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("list identities: %w", err)
	}

	stash := make([]stashedIdentity, 0, len(rows))
	for _, row := range rows {
		stash = append(stash, stashedIdentity{Type: row.Type, Value: row.Value, Verified: row.Verified})
	}

	plain, err := json.Marshal(stash)
	if err != nil {
		return nil, fmt.Errorf("encode identities: %w", err)
	}

	sealed, err := a.Keys.Encrypt(ctx, scope(contact.WorkspaceID), plain, stashData(contact.ID))
	if err != nil {
		return nil, fmt.Errorf("seal identities: %w", err)
	}

	return sealed, nil
}
