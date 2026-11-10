/*
|--------------------------------------------------------------------------
| Cancel an Erasure
|--------------------------------------------------------------------------
|
| During its grace period a request can be withdrawn and the customer
| comes back as they were: visible again, with the addresses they had.
| An address someone else took meanwhile, such as a new lead who wrote
| from it, stays with them; the answer lists it so a teammate can merge
| the two by hand.
|
*/

package erasures

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
CancelErasureInput is who withdraws which request.
*/
type CancelErasureInput struct {
	Membership authz.Membership
	ErasureID  ids.UUID
}

/*
Canceled is a withdrawn request and the identities that could not be
given back, as type:value.
*/
type Canceled struct {
	Erasure    models.ContactErasure
	Conflicted []string
}

/*
CancelErasure withdraws a request during its grace period.

Rules:
  - The caller needs contacts:erase.
  - Only a pending or held request can be withdrawn.

Events: [appevents.ContactErasureCanceled].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrErasureClosed], or a wrapped error.
*/
type CancelErasure struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
	Keys   encryption.KeyManager
}

/*
Handle withdraws the request and restores the customer.
*/
func (a *CancelErasure) Handle(ctx context.Context, in CancelErasureInput) (Canceled, error) {
	if !in.Membership.Can("contacts:erase") {
		return Canceled{}, exceptions.ErrForbidden
	}

	var out Canceled

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws, now := models.New(tx), in.Membership.Workspace.ID, a.Clock.Now()

		row, err := q.LockContactErasure(ctx, models.LockContactErasureParams{WorkspaceID: ws, ID: in.ErasureID})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrNotFound
		}

		if err != nil {
			return fmt.Errorf("lock erasure: %w", err)
		}

		if !open(row) {
			return exceptions.ErrErasureClosed
		}

		if out.Conflicted, err = a.restore(ctx, q, row); err != nil {
			return err
		}

		if out.Erasure, err = q.CancelContactErasure(ctx, models.CancelContactErasureParams{
			Now: now, WorkspaceID: ws, ID: row.ID,
		}); err != nil {
			return fmt.Errorf("cancel erasure: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactErasureCanceled{
			Base: base(in.Membership, now), ErasureID: row.ID, ContactID: row.ContactID, Conflicted: out.Conflicted,
		})
	})

	return out, err
}

/*
restore shows the contact again and gives back the identities no one
else holds, returning those someone does.
*/
func (a *CancelErasure) restore(ctx context.Context, q *models.Queries, row models.ContactErasure) ([]string, error) {
	now := a.Clock.Now()

	err := q.RestoreErasedContact(ctx, models.RestoreErasedContactParams{
		Now: now, WorkspaceID: row.WorkspaceID, ID: row.ContactID,
	})
	if err != nil {
		return nil, fmt.Errorf("restore contact: %w", err)
	}

	stash, err := openStash(ctx, a.Keys, row)
	if err != nil {
		return nil, err
	}

	conflicted := []string{}

	for _, identity := range stash {
		_, err := q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
			WorkspaceID: row.WorkspaceID, Type: identity.Type, Value: identity.Value,
		})
		if err == nil {
			conflicted = append(conflicted, identity.Type+":"+identity.Value)

			continue
		}

		if !errors.Is(err, database.ErrNotFound) {
			return nil, fmt.Errorf("find identity: %w", err)
		}

		if _, err := q.CreateContactIdentity(ctx, models.CreateContactIdentityParams{
			WorkspaceID: row.WorkspaceID, ContactID: row.ContactID, Type: identity.Type, Value: identity.Value,
			Verified: identity.Verified, Now: now,
		}); err != nil {
			return nil, fmt.Errorf("restore identity: %w", err)
		}
	}

	return conflicted, nil
}
