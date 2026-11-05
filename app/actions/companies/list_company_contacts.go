/*
|--------------------------------------------------------------------------
| Company People
|--------------------------------------------------------------------------
|
| The people who work at a company, newest first, a page at a time. Only
| a live company has people to list.
|
*/

package companies

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactsInput is who asks for which page of a company's people.
*/
type ContactsInput struct {
	Membership authz.Membership
	CompanyID  ids.UUID
	AfterAt    *time.Time
	AfterID    *ids.UUID
	Limit      int
}

/*
ContactsPage is a page of a company's people and whether more follow.
*/
type ContactsPage struct {
	Contacts []models.Contact
	More     bool
}

/*
ListCompanyContacts lists a company's people, newest first.

Rules:
  - The caller needs contacts:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type ListCompanyContacts struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListCompanyContacts) Handle(ctx context.Context, in ContactsInput) (ContactsPage, error) {
	if !in.Membership.Can("contacts:read") {
		return ContactsPage{}, exceptions.ErrForbidden
	}

	var page ContactsPage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q, ws := models.New(tx), in.Membership.Workspace.ID

		if _, err := find(ctx, q, ws, in.CompanyID); err != nil {
			return err
		}

		rows, err := q.ListCompanyContacts(ctx, models.ListCompanyContactsParams{
			WorkspaceID: ws, CompanyID: in.CompanyID, BeforeID: in.AfterID, BeforeCreatedAt: in.AfterAt,
			RowLimit: int64(in.Limit) + 1,
		})
		if err != nil {
			return fmt.Errorf("list company contacts: %w", err)
		}

		if len(rows) > in.Limit {
			rows, page.More = rows[:in.Limit], true
		}

		page.Contacts = rows

		return nil
	})

	return page, err
}
