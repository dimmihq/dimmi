/*
|--------------------------------------------------------------------------
| List Contacts
|--------------------------------------------------------------------------
|
| Every member sees the workspace's customers, a page at a time, most
| recently seen first, newest first or by name, narrowed by kind,
| address, external id, blocked state or a search of names, addresses,
| phone numbers and external ids. Contacts merged into another never
| show; their old ids lead to the one they were merged into.
|
*/

package contacts

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/searchquery"
)

/*
The orders of a contact list.
*/
const (
	SortLastSeen = "last_seen_at"
	SortCreated  = "created_at"
	SortName     = "name"
)

/*
ListContactsInput is who asks and which page of which contacts. The
pointers narrow only when set; Search matches names, addresses, phone
numbers and external ids. TagIDs keeps contacts carrying every tag, and
Custom is a JSON object of custom attributes they must hold.
*/
type ListContactsInput struct {
	Membership authz.Membership
	Kind       *string
	Email      *string
	ExternalID *string
	Blocked    *bool
	CompanyID  *ids.UUID
	TagIDs     []ids.UUID
	Seen       TimeRange
	Created    TimeRange
	Custom     json.RawMessage
	Search     string
	Sort       string
	Offset     int
	Limit      int
}

/*
ContactPage is a page of contacts and whether more follow.
*/
type ContactPage struct {
	Contacts []Detail
	More     bool
}

/*
TimeRange narrows a time to at or after From and before To, either nil
for no limit.
*/
type TimeRange struct {
	From *time.Time
	To   *time.Time
}

/*
ListContacts lists the workspace's contacts.

Rules:
  - The caller needs contacts:read.

Errors: [exceptions.ErrForbidden], or a wrapped database error.
*/
type ListContacts struct {
	DB *database.DB
}

/*
Handle returns the page.
*/
func (a *ListContacts) Handle(ctx context.Context, in ListContactsInput) (ContactPage, error) {
	if !in.Membership.Can("contacts:read") {
		return ContactPage{}, exceptions.ErrForbidden
	}

	var page ContactPage

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		rows, err := q.ListContacts(ctx, models.ListContactsParams{
			WorkspaceID: in.Membership.Workspace.ID, Kind: in.Kind, Email: NormalizeEmail(in.Email),
			ExternalID: in.ExternalID, Blocked: in.Blocked, CompanyID: in.CompanyID, TagIds: in.TagIDs,
			LastSeenAfter: in.Seen.From, LastSeenBefore: in.Seen.To, CreatedAfter: in.Created.From,
			CreatedBefore: in.Created.To, Custom: in.Custom, Search: LikePattern(in.Search),
			Terms: searchquery.Parse(in.Search).TSQuery, Sort: in.Sort, RowOffset: int64(in.Offset),
			RowLimit: int64(in.Limit + 1),
		})
		if err != nil {
			return fmt.Errorf("list contacts: %w", err)
		}

		if len(rows) > in.Limit {
			rows, page.More = rows[:in.Limit], true
		}

		page.Contacts, err = LoadDetails(ctx, q, rows)

		return err
	})

	return page, err
}

/*
LikePattern escapes the characters LIKE reads as wildcards, so a search
matches what was typed.
*/
func LikePattern(search string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(search))
}
