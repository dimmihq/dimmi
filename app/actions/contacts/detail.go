/*
|--------------------------------------------------------------------------
| Contact Details
|--------------------------------------------------------------------------
|
| A contact is shown with the company they work for and the tags that
| follow them across conversations. Both are read a page at a time, one
| query per kind of row, so a long directory costs the same few queries
| as a short one.
|
*/

package contacts

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Detail is a contact with what is shown beside it. Company is nil when
they have none.
*/
type Detail struct {
	Contact models.Contact
	Company *models.Company
	Tags    []models.Tag
}

/*
LoadDetails loads the contacts of one workspace, in the order given.
*/
func LoadDetails(ctx context.Context, q *models.Queries, rows []models.Contact) ([]Detail, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	ws := rows[0].WorkspaceID
	contactIDs, companyIDs := make([]ids.UUID, 0, len(rows)), []ids.UUID{}

	for _, c := range rows {
		contactIDs = append(contactIDs, c.ID)
		if c.CompanyID != nil {
			companyIDs = append(companyIDs, *c.CompanyID)
		}
	}

	tags, err := q.ListContactTags(ctx, models.ListContactTagsParams{WorkspaceID: ws, ContactIds: contactIDs})
	if err != nil {
		return nil, fmt.Errorf("read contact tags: %w", err)
	}

	companies, err := q.CompaniesByIDs(ctx, models.CompaniesByIDsParams{WorkspaceID: ws, Ids: companyIDs})
	if err != nil {
		return nil, fmt.Errorf("read contact companies: %w", err)
	}

	byContact := map[ids.UUID][]models.Tag{}
	for _, row := range tags {
		byContact[row.ContactID] = append(byContact[row.ContactID], row.Tag)
	}

	byID := map[ids.UUID]*models.Company{}
	for i := range companies {
		byID[companies[i].ID] = &companies[i]
	}

	out := make([]Detail, len(rows))
	for i, c := range rows {
		out[i] = Detail{Contact: c, Tags: byContact[c.ID]}
		if c.CompanyID != nil {
			out[i].Company = byID[*c.CompanyID]
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
