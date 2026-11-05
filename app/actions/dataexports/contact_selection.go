/*
|--------------------------------------------------------------------------
| Contact Selections
|--------------------------------------------------------------------------
|
| A contacts export covers the customers a member picked or every
| customer the directory's filters match. Merged and deleted customers,
| and those someone asked to erase, never leave in an export.
|
*/

package dataexports

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/searchquery"
)

/*
ContactSelection names the customers of an export: the ids picked, or
the directory's filters when Filters is set.
*/
type ContactSelection struct {
	IDs     []ids.UUID      `json:"ids,omitempty"`
	Filters *ContactFilters `json:"filters,omitempty"`
}

/*
ContactFilters are the directory's filters, each narrowing only when
set. Search matches names, addresses, phone numbers and external ids;
Custom is a JSON object of custom attributes the customers must hold.
*/
type ContactFilters struct {
	Kind          *string         `json:"kind,omitempty"`
	Email         *string         `json:"email,omitempty"`
	ExternalID    *string         `json:"external_id,omitempty"`
	Blocked       *bool           `json:"blocked,omitempty"`
	CompanyID     *ids.UUID       `json:"company_id,omitempty"`
	TagIDs        []ids.UUID      `json:"tag_ids,omitempty"`
	SeenAfter     *time.Time      `json:"seen_after,omitempty"`
	SeenBefore    *time.Time      `json:"seen_before,omitempty"`
	CreatedAfter  *time.Time      `json:"created_after,omitempty"`
	CreatedBefore *time.Time      `json:"created_before,omitempty"`
	Custom        json.RawMessage `json:"custom,omitempty"`
	Search        string          `json:"search,omitempty"`
}

/*
contactRead is one read of a contact selection: in which workspace, and
how many rows at most.
*/
type contactRead struct {
	ws    ids.UUID
	sel   ContactSelection
	limit int
}

/*
countContacts returns how many customers the selection covers, up to
the count's cap.
*/
func countContacts(ctx context.Context, q *models.Queries, r contactRead) (int, error) {
	if r.sel.Filters == nil {
		rows, err := pickedContacts(ctx, q, r.ws, r.sel.IDs)

		return len(rows), err
	}

	f := r.sel.Filters

	n, err := q.CountContacts(ctx, models.CountContactsParams{
		WorkspaceID: r.ws, Kind: f.Kind, Email: contacts.NormalizeEmail(f.Email), ExternalID: f.ExternalID,
		Blocked: f.Blocked, CompanyID: f.CompanyID, TagIds: f.TagIDs, LastSeenAfter: f.SeenAfter,
		LastSeenBefore: f.SeenBefore, CreatedAfter: f.CreatedAfter, CreatedBefore: f.CreatedBefore,
		Custom: f.Custom, Search: contacts.LikePattern(f.Search), Terms: searchquery.Parse(f.Search).TSQuery,
		RowLimit: countCap,
	})
	if err != nil {
		return 0, fmt.Errorf("count contacts: %w", err)
	}

	return int(n), nil
}

/*
selectContacts returns up to limit customers of the selection, loaded,
most recently seen first.
*/
func selectContacts(ctx context.Context, q *models.Queries, r contactRead) ([]contacts.Detail, error) {
	rows, err := pickedContacts(ctx, q, r.ws, r.sel.IDs)
	if r.sel.Filters != nil {
		f := r.sel.Filters

		rows, err = q.ListContacts(ctx, models.ListContactsParams{
			WorkspaceID: r.ws, Kind: f.Kind, Email: contacts.NormalizeEmail(f.Email), ExternalID: f.ExternalID,
			Blocked: f.Blocked, CompanyID: f.CompanyID, TagIds: f.TagIDs, LastSeenAfter: f.SeenAfter,
			LastSeenBefore: f.SeenBefore, CreatedAfter: f.CreatedAfter, CreatedBefore: f.CreatedBefore,
			Custom: f.Custom, Search: contacts.LikePattern(f.Search), Terms: searchquery.Parse(f.Search).TSQuery,
			Sort: "last_seen_at", RowLimit: int64(r.limit),
		})
	}

	if err != nil {
		return nil, fmt.Errorf("select contacts: %w", err)
	}

	return contacts.LoadDetails(ctx, q, rows)
}

/*
pickedContacts returns the live customers among those picked.
*/
func pickedContacts(ctx context.Context, q *models.Queries, ws ids.UUID, picked []ids.UUID) ([]models.Contact, error) {
	if len(picked) == 0 {
		return nil, nil
	}

	rows, err := q.ExportContactsByIDs(ctx, models.ExportContactsByIDsParams{WorkspaceID: ws, Ids: picked})
	if err != nil {
		return nil, fmt.Errorf("find contacts: %w", err)
	}

	return rows, nil
}
