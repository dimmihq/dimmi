/*
|--------------------------------------------------------------------------
| Delete Contact
|--------------------------------------------------------------------------
|
| A contact added by mistake can go as long as nothing hangs off it. A
| customer with conversations has a history the team relies on, and
| removing that is a privacy request with its own path, not a tidy-up.
| Identities leave with the contact so the address is free for whoever
| writes next.
|
*/

package contacts

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
DeleteContact removes a contact without conversations.

Rules:
  - The caller needs contacts:write.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrContactHasConversations], or a wrapped database error.
*/
type DeleteContact struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle removes the contact.
*/
func (a *DeleteContact) Handle(ctx context.Context, in ShowContactInput) error {
	if !in.Membership.Can("contacts:write") {
		return exceptions.ErrForbidden
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		contact, err := Survivor(ctx, q, ws, in.ContactID)
		if err != nil {
			return err
		}

		count, err := q.CountContactConversations(ctx, models.CountContactConversationsParams{
			WorkspaceID: ws, ContactID: contact.ID,
		})
		if err != nil {
			return fmt.Errorf("count conversations: %w", err)
		}

		if count > 0 {
			return exceptions.ErrContactHasConversations
		}

		if err := dropIdentities(ctx, q, contact); err != nil {
			return err
		}

		err = q.SoftDeleteContact(ctx, models.SoftDeleteContactParams{
			Now: a.Clock.Now(), WorkspaceID: ws, ID: contact.ID,
		})
		if err != nil {
			return fmt.Errorf("delete contact: %w", err)
		}

		return nil
	})
}

/*
dropIdentities removes every identity of the contact, freeing its
addresses.
*/
func dropIdentities(ctx context.Context, q *models.Queries, c models.Contact) error {
	identities, err := q.ListContactIdentities(ctx, models.ListContactIdentitiesParams{
		WorkspaceID: c.WorkspaceID, ContactID: c.ID,
	})
	if err != nil {
		return fmt.Errorf("list identities: %w", err)
	}

	for _, identity := range identities {
		if _, err := q.DeleteContactIdentity(ctx, models.DeleteContactIdentityParams{
			WorkspaceID: c.WorkspaceID, ContactID: c.ID, ID: identity.ID,
		}); err != nil {
			return fmt.Errorf("delete identity: %w", err)
		}
	}

	return nil
}
