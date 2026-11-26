/*
|--------------------------------------------------------------------------
| Contact Companies
|--------------------------------------------------------------------------
|
| A contact can belong to several companies, and one of them is their
| default: the company a new conversation or ticket is about, and the
| one contacts.company_id names. The first company a contact gets
| becomes the default. Taking off the default makes the oldest
| remaining company the default. The database keeps the links and
| contacts.company_id in step (see the contact_companies migration), so
| these actions only say what should change.
|
*/

package contacts

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
MaxCompaniesPerContact is the most companies one contact may belong to.
*/
const MaxCompaniesPerContact = 300

/*
The ways to change a contact's companies.
*/
const (
	CompanyAdd     = "add"
	CompanyRemove  = "remove"
	CompanyDefault = "default"
)

/*
ChangeCompanyInput is who changes which contact's companies and how.
Op is CompanyAdd, CompanyRemove or CompanyDefault. MakeDefault, on an
add, also makes the company the default.
*/
type ChangeCompanyInput struct {
	Membership  authz.Membership
	ContactID   ids.UUID
	CompanyID   ids.UUID
	Op          string
	MakeDefault bool
}

/*
ChangeCompany adds a company to a contact, takes one off, or makes one
their default.

Rules:
  - The caller needs contacts:write.
  - The company must be a live one of the workspace.
  - A contact belongs to at most 300 companies.
  - Adding a company the contact has, or removing one they do not, changes
    nothing. Making a company the default adds it first when needed.

Events: [appevents.ContactUpdated] when anything changed, with
"companies" and, when the default moved, "company_id".

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrCompanyNotFound], [exceptions.ErrCompanyLimitReached],
or a wrapped database error.
*/
type ChangeCompany struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle applies the change and returns the contact as it now is.
*/
func (a *ChangeCompany) Handle(ctx context.Context, in ChangeCompanyInput) (Detail, error) {
	if !in.Membership.Can("contacts:write") {
		return Detail{}, exceptions.ErrForbidden
	}

	var out Detail

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := Survivor(ctx, q, in.Membership.Workspace.ID, in.ContactID)
		if err != nil {
			return err
		}

		if _, err := q.LockContact(ctx, models.LockContactParams{
			WorkspaceID: current.WorkspaceID, ID: current.ID,
		}); err != nil {
			return fmt.Errorf("lock contact: %w", err)
		}

		want, err := planOf(ctx, q, current, in)
		if err != nil {
			return err
		}

		want.Now = a.Clock.Now()

		contact, changes, err := SetCompanies(ctx, q, current, want)
		if err != nil {
			return err
		}

		if out, err = LoadDetail(ctx, q, contact); err != nil || len(changes) == 0 {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactUpdated{
			WorkspaceID: contact.WorkspaceID, ContactID: contact.ID, ActorID: in.Membership.Member.ID,
			Changes: changes,
		})
	})

	return out, err
}

/*
CompanySet is the companies a contact should belong to, in the order
they are added, and which of them is the default. A nil Default keeps
the current default when it stays, else takes the first company. Now
is when the links are made.
*/
type CompanySet struct {
	IDs     []ids.UUID
	Default *ids.UUID
	Now     time.Time
}

/*
planOf is the contact's companies after the change: one more, made the
default when asked, or one fewer.
*/
func planOf(ctx context.Context, q *models.Queries, c models.Contact, in ChangeCompanyInput) (CompanySet, error) {
	have, err := q.ContactCompanyIDs(ctx, models.ContactCompanyIDsParams{WorkspaceID: c.WorkspaceID, ContactID: c.ID})
	if err != nil {
		return CompanySet{}, fmt.Errorf("read contact companies: %w", err)
	}

	if in.Op == CompanyRemove {
		return CompanySet{IDs: slices.DeleteFunc(have, func(id ids.UUID) bool { return id == in.CompanyID })}, nil
	}

	set := CompanySet{IDs: have}
	if !slices.Contains(have, in.CompanyID) {
		set.IDs = append(set.IDs, in.CompanyID)
	}

	if in.Op == CompanyDefault || in.MakeDefault {
		set.Default = &in.CompanyID
	}

	return set, nil
}

/*
SetCompanies makes the contact belong to exactly the companies of the
set, with its default, and returns the contact as it now is and what
changed: "companies" when a company was added or taken off, and
"company_id" when the default moved. Companies must be live ones of the
contact's workspace. The caller holds the contact's lock.
*/
func SetCompanies(
	ctx context.Context, q *models.Queries, c models.Contact, want CompanySet,
) (models.Contact, []string, error) {
	ws := c.WorkspaceID
	wanted := uniqueIDs(want.IDs)

	if want.Default != nil && !slices.Contains(wanted, *want.Default) {
		wanted = append(wanted, *want.Default)
	}

	if len(wanted) > MaxCompaniesPerContact {
		return c, nil, exceptions.ErrCompanyLimitReached
	}

	if err := companiesLive(ctx, q, ws, wanted); err != nil {
		return c, nil, err
	}

	have, err := q.ContactCompanyIDs(ctx, models.ContactCompanyIDsParams{WorkspaceID: ws, ContactID: c.ID})
	if err != nil {
		return c, nil, fmt.Errorf("read contact companies: %w", err)
	}

	r := relinker{q: q, c: c, now: want.Now}

	linked, err := r.relink(ctx, have, wanted, defaultOf(c.CompanyID, want.Default, wanted))
	if err != nil {
		return c, nil, err
	}

	after, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: c.ID})
	if err != nil {
		return c, nil, fmt.Errorf("find contact: %w", err)
	}

	var changes []string
	if linked {
		changes = append(changes, "companies")
	}

	if !sameID(c.CompanyID, after.CompanyID) {
		changes = append(changes, "company_id")
	}

	return after, changes, nil
}

/*
relinker changes one contact's links at one time.
*/
type relinker struct {
	q   *models.Queries
	c   models.Contact
	now time.Time
}

/*
relink adds the companies wanted that the contact lacks, sets the
default, then takes off the companies no longer wanted, so the default
never falls back on its own. It reports whether any link changed.
*/
func (r relinker) relink(ctx context.Context, have, wanted []ids.UUID, next *ids.UUID) (bool, error) {
	q, c, now := r.q, r.c, r.now
	ws, linked := c.WorkspaceID, false

	for _, id := range wanted {
		if slices.Contains(have, id) {
			continue
		}

		if _, err := q.LinkContactCompany(ctx, models.LinkContactCompanyParams{
			WorkspaceID: ws, ContactID: c.ID, CompanyID: id, Now: now,
		}); err != nil {
			return linked, fmt.Errorf("link company: %w", err)
		}

		linked = true
	}

	if next != nil {
		if err := q.SetContactDefaultCompany(ctx, models.SetContactDefaultCompanyParams{
			WorkspaceID: ws, ID: c.ID, CompanyID: *next, Now: now,
		}); err != nil {
			return linked, fmt.Errorf("set default company: %w", err)
		}
	}

	for _, id := range have {
		if slices.Contains(wanted, id) {
			continue
		}

		if _, err := q.UnlinkContactCompany(ctx, models.UnlinkContactCompanyParams{
			WorkspaceID: ws, ContactID: c.ID, CompanyID: id,
		}); err != nil {
			return linked, fmt.Errorf("unlink company: %w", err)
		}

		linked = true
	}

	return linked, nil
}

/*
defaultOf picks the default company of a set: the one asked for, else
the current default when it stays, else the first company. Nil means
no company is left, which the database works out on its own as the
links go.
*/
func defaultOf(current, asked *ids.UUID, wanted []ids.UUID) *ids.UUID {
	switch {
	case asked != nil:
		return asked
	case current != nil && slices.Contains(wanted, *current):
		return current
	case len(wanted) > 0:
		return &wanted[0]
	}

	return nil
}

/*
companiesLive refuses any id that is not a live company of the
workspace.
*/
func companiesLive(ctx context.Context, q *models.Queries, ws ids.UUID, wanted []ids.UUID) error {
	if len(wanted) == 0 {
		return nil
	}

	found, err := q.CompaniesByIDs(ctx, models.CompaniesByIDsParams{WorkspaceID: ws, Ids: wanted})
	if err != nil {
		return fmt.Errorf("find companies: %w", err)
	}

	if len(found) != len(wanted) {
		return exceptions.ErrCompanyNotFound
	}

	return nil
}

/*
uniqueIDs drops repeated ids, keeping the first of each.
*/
func uniqueIDs(in []ids.UUID) []ids.UUID {
	out := make([]ids.UUID, 0, len(in))
	for _, id := range in {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}

	return out
}
