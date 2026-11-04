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
	"fmt"
	"strings"

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
numbers and external ids.
*/
type ListContactsInput struct {
	Membership authz.Membership
	Kind       *string
	Email      *string
	ExternalID *string
	Blocked    *bool
	Search     string
	Sort       string
	Offset     int
	Limit      int
}

/*
ContactPage is a page of contacts and whether more follow.
*/
type ContactPage struct {
	Contacts []models.Contact
	More     bool
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

	var rows []models.Contact

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		rows, err = models.New(tx).ListContacts(ctx, models.ListContactsParams{
			WorkspaceID: in.Membership.Workspace.ID, Kind: in.Kind, Email: NormalizeEmail(in.Email),
			ExternalID: in.ExternalID, Blocked: in.Blocked, Search: LikePattern(in.Search),
			Terms: searchquery.Parse(in.Search).TSQuery, Sort: in.Sort, RowOffset: int64(in.Offset),
			RowLimit: int64(in.Limit + 1),
		})

		return err
	})
	if err != nil {
		return ContactPage{}, fmt.Errorf("list contacts: %w", err)
	}

	if len(rows) > in.Limit {
		return ContactPage{Contacts: rows[:in.Limit], More: true}, nil
	}

	return ContactPage{Contacts: rows}, nil
}

/*
LikePattern escapes the characters LIKE reads as wildcards, so a search
matches what was typed.
*/
func LikePattern(search string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.TrimSpace(search))
}

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
func (a *ShowContact) Handle(ctx context.Context, in ShowContactInput) (models.Contact, error) {
	if !in.Membership.Can("contacts:read") {
		return models.Contact{}, exceptions.ErrForbidden
	}

	var out models.Contact

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		out, err = Survivor(ctx, models.New(tx), in.Membership.Workspace.ID, in.ContactID)

		return err
	})

	return out, err
}
