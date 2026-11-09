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

	"github.com/dimmihq/dimmi/framework/clock"
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
numbers and external ids. TagIDs keeps contacts carrying every tag,
Custom is a JSON object of custom attributes they must hold, and
Conditions is a checked list of conditions contact_matches applies, and
Audiences keeps people in (or out of) saved audiences.
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
	Conditions json.RawMessage
	Audiences  *AudienceFilter
	Search     string
	Sort       string
	Offset     int
	Limit      int
	Count      bool
}

/*
ContactPage is a page of contacts, whether more follow and, when asked,
how many match in all, up to TotalCap.
*/
type ContactPage struct {
	Contacts []Detail
	More     bool
	Total    *int64
}

/*
TotalCap is the most a contact count goes up to; a list that reaches it
says only that there are at least that many.
*/
const TotalCap = 10_000

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
	DB    *database.DB
	Clock clock.Clock
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
		if in.Audiences != nil {
			var err error
			page, err = a.inAudiences(ctx, q, in)

			return err
		}

		rows, err := q.ListContacts(ctx, params(in, in.Offset, in.Limit+1))
		if err != nil {
			return fmt.Errorf("list contacts: %w", err)
		}

		if len(rows) > in.Limit {
			rows, page.More = rows[:in.Limit], true
		}

		if page.Contacts, err = LoadDetails(ctx, q, rows); err != nil || !in.Count {
			return err
		}

		page.Total, err = count(ctx, q, in)

		return err
	})

	return page, err
}

/*
inAudiences lists the page of people in the input's audiences: every
other filter narrows the list in the database, then the people are
judged in order and the page is cut from those kept.
*/
func (a *ListContacts) inAudiences(ctx context.Context, q *models.Queries, in ListContactsInput) (ContactPage, error) {
	rows, err := q.ListContacts(ctx, params(in, 0, AudienceScanCap))
	if err != nil {
		return ContactPage{}, fmt.Errorf("list contacts: %w", err)
	}

	kept, err := InAudiences(ctx, q, Judging{
		WorkspaceID: in.Membership.Workspace.ID, Now: a.Clock.Now(), Rows: rows, Filter: *in.Audiences,
	})
	if err != nil {
		return ContactPage{}, err
	}

	var page ContactPage
	if in.Count {
		total := int64(min(len(kept), TotalCap))
		page.Total = &total
	}

	start := min(in.Offset, len(kept))
	end := min(start+in.Limit, len(kept))
	page.More = end < len(kept)
	page.Contacts, err = LoadDetails(ctx, q, kept[start:end])

	return page, err
}

/*
params writes the input as the list query's parameters.
*/
func params(in ListContactsInput, offset, limit int) models.ListContactsParams {
	return models.ListContactsParams{
		WorkspaceID: in.Membership.Workspace.ID, Kind: in.Kind, Email: NormalizeEmail(in.Email),
		ExternalID: in.ExternalID, Blocked: in.Blocked, CompanyID: in.CompanyID, TagIds: in.TagIDs,
		LastSeenAfter: in.Seen.From, LastSeenBefore: in.Seen.To, CreatedAfter: in.Created.From,
		CreatedBefore: in.Created.To, Custom: in.Custom, Conditions: in.Conditions, Search: LikePattern(in.Search),
		Terms: searchquery.Parse(in.Search).TSQuery, Sort: in.Sort, RowOffset: int64(offset),
		RowLimit: int64(limit),
	}
}

/*
count counts the contacts the input lists, up to TotalCap.
*/
func count(ctx context.Context, q *models.Queries, in ListContactsInput) (*int64, error) {
	n, err := q.CountContacts(ctx, models.CountContactsParams{
		WorkspaceID: in.Membership.Workspace.ID, Kind: in.Kind, Email: NormalizeEmail(in.Email),
		ExternalID: in.ExternalID, Blocked: in.Blocked, CompanyID: in.CompanyID, TagIds: in.TagIDs,
		LastSeenAfter: in.Seen.From, LastSeenBefore: in.Seen.To, CreatedAfter: in.Created.From,
		CreatedBefore: in.Created.To, Custom: in.Custom, Conditions: in.Conditions, Search: LikePattern(in.Search),
		Terms: searchquery.Parse(in.Search).TSQuery, RowLimit: TotalCap,
	})
	if err != nil {
		return nil, fmt.Errorf("count contacts: %w", err)
	}

	return &n, nil
}

/*
LikePattern escapes the characters LIKE reads as wildcards, so a search
matches what was typed.
*/
func LikePattern(search string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(search))
}
