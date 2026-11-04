/*
|--------------------------------------------------------------------------
| Duplicate Contacts
|--------------------------------------------------------------------------
|
| The same person often shows up twice: a lead who typed their address and
| the user they turn out to be. The finder offers likely pairs with the
| reasons it thinks so, and a teammate decides. Nothing is merged because
| of it. The same name at the same company is the weakest signal and is
| listed last.
|
*/

package contacts

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxDuplicates is the most possible duplicates listed for a contact.
*/
const maxDuplicates = 20

/*
The reasons two contacts look like one person.
*/
const (
	ReasonSameEmail       = "same_email"
	ReasonSamePhone       = "same_phone"
	ReasonSameWhatsApp    = "same_whatsapp"
	ReasonSameNameCompany = "same_name_company"
)

/*
Duplicate is a contact that looks like the same person, and why.
*/
type Duplicate struct {
	Contact models.Contact
	Reasons []string
}

/*
FindDuplicates returns the contacts that look like the same person as
the contact.
*/
func FindDuplicates(ctx context.Context, q *models.Queries, c models.Contact) ([]Duplicate, error) {
	rows, err := q.FindDuplicateContacts(ctx, models.FindDuplicateContactsParams{
		Email: c.Email, Phone: c.Phone, Name: c.Name, CompanyID: c.CompanyID, WorkspaceID: c.WorkspaceID, ID: c.ID,
		RowLimit: maxDuplicates,
	})
	if err != nil {
		return nil, fmt.Errorf("find duplicates: %w", err)
	}

	out := make([]Duplicate, 0, len(rows))

	for _, row := range rows {
		d := Duplicate{Contact: row.Contact}

		for _, signal := range []struct {
			hit    bool
			reason string
		}{
			{row.SameEmail, ReasonSameEmail},
			{row.SamePhone, ReasonSamePhone},
			{row.SameWhatsapp, ReasonSameWhatsApp},
			{row.SameNameCompany, ReasonSameNameCompany},
		} {
			if signal.hit {
				d.Reasons = append(d.Reasons, signal.reason)
			}
		}

		out = append(out, d)
	}

	return out, nil
}

/*
FindDuplicateContacts lists a contact's possible duplicates.

Rules:
  - The caller needs contacts:read.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound], or a
wrapped database error.
*/
type FindDuplicateContacts struct {
	DB *database.DB
}

/*
Handle returns the possible duplicates.
*/
func (a *FindDuplicateContacts) Handle(ctx context.Context, in ShowContactInput) ([]Duplicate, error) {
	if !in.Membership.Can("contacts:read") {
		return nil, exceptions.ErrForbidden
	}

	var out []Duplicate

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		contact, err := Survivor(ctx, q, in.Membership.Workspace.ID, in.ContactID)
		if err != nil {
			return err
		}

		out, err = FindDuplicates(ctx, q, contact)

		return err
	})

	return out, err
}
