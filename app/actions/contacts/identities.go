/*
|--------------------------------------------------------------------------
| Contact Identities
|--------------------------------------------------------------------------
|
| An identity is how a customer appears on a channel: an address, a phone
| number, a chat user. A teammate can detach one attached by mistake, and
| the next message from it starts a new lead instead of landing in this
| customer's history.
|
*/

package contacts

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ListIdentities returns how a contact appears on channels.

Rules:
  - The caller needs contacts:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListIdentities struct {
	DB *database.DB
}

/*
Handle returns the identities.
*/
func (a *ListIdentities) Handle(ctx context.Context, in ShowContactInput) ([]models.ContactIdentity, error) {
	if !in.Membership.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []models.ContactIdentity

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		contact, err := Survivor(ctx, q, ws, in.ContactID)
		if err != nil {
			return err
		}

		out, err = q.ListContactIdentities(ctx, models.ListContactIdentitiesParams{
			WorkspaceID: ws, ContactID: contact.ID,
		})
		if err != nil {
			return fmt.Errorf("list identities: %w", err)
		}

		return nil
	})

	return out, err
}

/*
DetachIdentityInput is who detaches which identity from which contact.
*/
type DetachIdentityInput struct {
	Membership authz.Membership
	ContactID  ids.UUID
	IdentityID ids.UUID
}

/*
DetachIdentity detaches an identity from a contact.

Rules:
  - The caller needs contacts:write.

Events: [appevents.ContactUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type DetachIdentity struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle detaches the identity.
*/
func (a *DetachIdentity) Handle(ctx context.Context, in DetachIdentityInput) error {
	if !in.Membership.Can("contacts:write") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		contact, err := Survivor(ctx, q, ws, in.ContactID)
		if err != nil {
			return err
		}

		rows, err := q.DeleteContactIdentity(ctx, models.DeleteContactIdentityParams{
			WorkspaceID: ws, ContactID: contact.ID, ID: in.IdentityID,
		})
		if err != nil {
			return fmt.Errorf("detach identity: %w", err)
		}

		if rows == 0 {
			return exceptions.ErrNotFound
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactUpdated{
			WorkspaceID: ws, ContactID: contact.ID, ActorID: in.Membership.Member.ID,
			Changes: []string{"identities"},
		})
	})
}
