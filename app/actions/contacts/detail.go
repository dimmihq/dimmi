/*
|--------------------------------------------------------------------------
| Contact Details
|--------------------------------------------------------------------------
|
| A contact is shown with the companies they belong to, the default one
| apart, and the tags that follow them across conversations. Both are read a page at a time, one
| query per kind of row, so a long directory costs the same few queries
| as a short one.
|
*/

package contacts

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Detail is a contact with what is shown beside it. Company is their
default company, nil when they have none. Companies are every company
they belong to, the default first.
*/
type Detail struct {
	Contact   models.Contact
	Company   *models.Company
	Companies []LinkedCompany
	Tags      []models.Tag
}

/*
LinkedCompany is one company a contact belongs to: whether it is their
default and when it was added.
*/
type LinkedCompany struct {
	Company   models.Company
	IsDefault bool
	LinkedAt  time.Time
}

/*
LoadDetails loads the contacts of one workspace, in the order given.
*/
func LoadDetails(ctx context.Context, q *models.Queries, rows []models.Contact) ([]Detail, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	ws := rows[0].WorkspaceID
	contactIDs := make([]ids.UUID, 0, len(rows))

	for _, c := range rows {
		contactIDs = append(contactIDs, c.ID)
	}

	tags, err := q.ListContactTags(ctx, models.ListContactTagsParams{WorkspaceID: ws, ContactIds: contactIDs})
	if err != nil {
		return nil, fmt.Errorf("read contact tags: %w", err)
	}

	links, err := q.ListContactCompanies(ctx, models.ListContactCompaniesParams{
		WorkspaceID: ws, ContactIds: contactIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("read contact companies: %w", err)
	}

	byContact := map[ids.UUID][]models.Tag{}
	for _, row := range tags {
		byContact[row.ContactID] = append(byContact[row.ContactID], row.Tag)
	}

	linked := map[ids.UUID][]LinkedCompany{}
	for _, row := range links {
		linked[row.ContactID] = append(linked[row.ContactID], LinkedCompany{
			Company: row.Company, IsDefault: row.IsDefault, LinkedAt: row.LinkedAt,
		})
	}

	out := make([]Detail, len(rows))
	for i, c := range rows {
		out[i] = Detail{Contact: c, Tags: byContact[c.ID], Companies: linked[c.ID]}
		for j := range out[i].Companies {
			if out[i].Companies[j].IsDefault {
				out[i].Company = &out[i].Companies[j].Company
			}
		}
	}

	return out, nil
}

/*
LoadDetail loads one contact.
*/
func LoadDetail(ctx context.Context, q *models.Queries, c models.Contact) (Detail, error) {
	details, err := LoadDetails(ctx, q, []models.Contact{c})
	if err != nil {
		return Detail{}, err
	}

	return details[0], nil
}

/*
DescribeContacts loads what is shown beside contacts another action
already read, such as a company's people, in a transaction of its own.
*/
type DescribeContacts struct {
	DB *database.DB
}

/*
Handle returns the contacts' details, in the order given.
*/
func (a *DescribeContacts) Handle(ctx context.Context, rows []models.Contact) ([]Detail, error) {
	var out []Detail

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		out, err = LoadDetails(ctx, models.New(tx), rows)

		return err
	})

	return out, err
}
