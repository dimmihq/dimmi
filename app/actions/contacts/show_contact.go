/*
|--------------------------------------------------------------------------
| Show Contact
|--------------------------------------------------------------------------
|
| A link to a merged contact keeps working: it lands on the contact it was
| merged into. A deleted contact is simply not found.
|
*/

package contacts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ShowContactInput is who asks about which contact.
*/
type ShowContactInput struct {
	Membership authz.Membership
	ContactID  ids.UUID
}

/*
ShowContact returns one contact, following a merged one to the contact
it was merged into.

Rules:
  - The caller needs contacts:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowContact struct {
	DB *database.DB
}

/*
Handle returns the contact.
*/
func (a *ShowContact) Handle(ctx context.Context, in ShowContactInput) (Detail, error) {
	if !in.Membership.Can("contacts:read") {
		return Detail{}, exceptions.ErrForbidden
	}

	var out Detail

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		contact, err := Survivor(ctx, q, in.Membership.Workspace.ID, in.ContactID)
		if err != nil {
			return err
		}

		out, err = LoadDetail(ctx, q, contact)

		return err
	})

	return out, err
}
