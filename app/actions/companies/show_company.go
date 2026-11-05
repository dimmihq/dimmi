/*
|--------------------------------------------------------------------------
| Show Company
|--------------------------------------------------------------------------
|
| One company with its tags, for its page and the customer panel. A
| deleted company is not found.
|
*/

package companies

import (
	"context"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ShowInput is who asks about which company.
*/
type ShowInput struct {
	Membership authz.Membership
	CompanyID  ids.UUID
}

/*
ShowCompany returns one company.

Rules:
  - The caller needs contacts:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ShowCompany struct {
	DB *database.DB
}

/*
Handle returns the company.
*/
func (a *ShowCompany) Handle(ctx context.Context, in ShowInput) (Detail, error) {
	if !in.Membership.Can("contacts:read") {
		return Detail{}, exceptions.ErrForbidden
	}

	var out Detail

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		c, err := find(ctx, q, in.Membership.Workspace.ID, in.CompanyID)
		if err != nil {
			return err
		}

		out, err = LoadDetail(ctx, q, c)

		return err
	})

	return out, err
}
