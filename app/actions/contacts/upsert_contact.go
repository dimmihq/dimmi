/*
|--------------------------------------------------------------------------
| Upsert Identified Contact
|--------------------------------------------------------------------------
|
| A developer's server tells us who a customer is. The external id finds
| them first, then a proven address. A lead who only typed the address is
| never chosen and never merged, since typing proves nothing; they are
| named back as possible duplicates for a teammate to judge. An address
| on someone with a different external id makes a second contact with
| the same address, and the address stays with the first. The API is a
| trusted source, so what it writes counts as proven and its attributes
| as trusted. An existing external id never changes through this path.
|
*/

package contacts

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
UpsertInput is who identifies which customer. Cleared lists the fields
sent as null. Company, when given, is found or added and linked.
*/
type UpsertInput struct {
	Membership authz.Membership
	Fields     Fields
	Cleared    map[string]bool
	Company    *companies.Fields
}

/*
Upserted is the contact as it now is, whether it was just added, and
leads with the same address a teammate may want to merge.
*/
type Upserted struct {
	Detail             Detail
	Created            bool
	PossibleDuplicates []models.Contact
}

/*
UpsertIdentifiedContact finds or adds the customer a trusted caller
names.

Rules:
  - The caller needs contacts:write.
  - The external id matches first, following merges, then a proven
    address on a contact without an external id.
  - An existing external id never changes.

Events: [appevents.ContactCreated] or [appevents.ContactUpdated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrUpsertKeyRequired],
[exceptions.ErrInvalidAttributes], or a wrapped database error.
*/
type UpsertIdentifiedContact struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle finds or adds the contact and returns it.
*/
func (a *UpsertIdentifiedContact) Handle(ctx context.Context, in UpsertInput) (Upserted, error) {
	if !in.Membership.Can("contacts:write") {
		return Upserted{}, exceptions.ErrForbidden
	}

	if in.Fields.ExternalID == nil && NormalizeEmail(in.Fields.Email) == nil {
		return Upserted{}, exceptions.ErrUpsertKeyRequired
	}

	var out Upserted

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		run := upsertRun{a: a, in: in, tx: tx, q: models.New(tx), now: a.Clock.Now()}

		var err error
		run.in.Fields.CustomAttributes, err = attributedefs.Gate(ctx, run.q, attributedefs.Write{
			WorkspaceID: in.Membership.Workspace.ID, Object: attributedefs.ObjectContact, Trusted: true,
			Changes: in.Fields.CustomAttributes, Now: run.now,
		})
		if err != nil {
			return err
		}

		out, err = run.upsert(ctx)

		return err
	})

	return out, err
}

/*
upsertRun is one upsert in its transaction.
*/
type upsertRun struct {
	a   *UpsertIdentifiedContact
	in  UpsertInput
	tx  *database.Tx
	q   *models.Queries
	now time.Time
}

/*
upsert matches the contact, adds or changes it and loads the result.
*/
func (r upsertRun) upsert(ctx context.Context) (Upserted, error) {
	company, err := r.company(ctx)
	if err != nil {
		return Upserted{}, err
	}

	found, matched, err := r.match(ctx)
	if err != nil {
		return Upserted{}, err
	}

	var contact models.Contact

	if matched {
		contact, err = r.change(ctx, found, company)
	} else {
		contact, err = r.add(ctx, company)
	}

	if err != nil {
		return Upserted{}, err
	}

	detail, err := LoadDetail(ctx, r.q, contact)
	if err != nil {
		return Upserted{}, err
	}

	leads, err := r.leads(ctx, contact)

	return Upserted{Detail: detail, Created: !matched, PossibleDuplicates: leads}, err
}

/*
company finds or adds the company named, nil when none was.
*/
func (r upsertRun) company(ctx context.Context) (*ids.UUID, error) {
	if r.in.Company == nil {
		return nil, nil
	}

	c, _, err := companies.Identify(ctx, r.tx, companies.Identity{
		WorkspaceID: r.in.Membership.Workspace.ID, Fields: *r.in.Company, Now: r.now,
	})
	if err != nil {
		return nil, err
	}

	return &c.ID, nil
}

/*
match finds the contact the keys name, following merges.
*/
func (r upsertRun) match(ctx context.Context) (models.Contact, bool, error) {
	ws, f := r.in.Membership.Workspace.ID, r.in.Fields

	if f.ExternalID != nil {
		c, err := r.q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: ws, ExternalID: f.ExternalID,
		})
		if err == nil {
			survivor, err := Survivor(ctx, r.q, ws, c.ID)

			return survivor, err == nil, err
		}

		if !errors.Is(err, database.ErrNotFound) {
			return c, false, fmt.Errorf("find contact: %w", err)
		}
	}

	email := NormalizeEmail(f.Email)
	if email == nil {
		return models.Contact{}, false, nil
	}

	c, err := r.q.FindUpsertCandidateByEmail(ctx, models.FindUpsertCandidateByEmailParams{
		WorkspaceID: ws, Email: email,
	})
	if errors.Is(err, database.ErrNotFound) {
		return c, false, nil
	}

	if err != nil {
		return c, false, fmt.Errorf("find contact: %w", err)
	}

	return c, true, nil
}

/*
add creates the contact, proven since the caller is trusted.
*/
func (r upsertRun) add(ctx context.Context, company *ids.UUID) (models.Contact, error) {
	ws, m := r.in.Membership.Workspace.ID, r.in.Membership

	contact, err := Create(ctx, r.tx, r.now, NewContact{WorkspaceID: ws, Fields: r.in.Fields})
	if err != nil {
		return contact, err
	}

	if company != nil {
		params := paramsOf(contact, r.now)
		params.CompanyID = company

		if contact, err = r.q.UpdateContact(ctx, params); err != nil {
			return contact, fmt.Errorf("link company: %w", err)
		}
	}

	return contact, r.a.Events.Dispatch(ctx, r.tx, appevents.ContactCreated{
		WorkspaceID: ws, ContactID: contact.ID, ActorID: m.Member.ID,
	})
}

/*
change writes what was sent over the matched contact: only sent fields
change, null clears, and the external id is only ever set, never
replaced.
*/
func (r upsertRun) change(ctx context.Context, c models.Contact, company *ids.UUID) (models.Contact, error) {
	f := r.in.Fields
	if f.ExternalID != nil && c.ExternalID != nil && *c.ExternalID != *f.ExternalID {
		return c, exceptions.ErrExternalIDImmutable
	}

	u := update{in: UpdateContactInput{Membership: r.in.Membership}, now: r.now, before: c}
	u.params = paramsOf(c, r.now)

	if f.ExternalID != nil && c.ExternalID == nil {
		u.params.ExternalID, u.params.Kind = f.ExternalID, string(enums.ContactKindUser)
		u.params.EmailVerified = true
		u.event.Changes = append(u.event.Changes, "external_id")
	}

	u.text("email", &u.params.Email, sent(NormalizeEmail(f.Email), r.in.Cleared["email"]))
	u.text("name", &u.params.Name, sent(f.Name, r.in.Cleared["name"]))
	u.text("phone", &u.params.Phone, sent(NormalizePhone(f.Phone), r.in.Cleared["phone"]))

	if company != nil && !sameID(c.CompanyID, company) {
		u.params.CompanyID = company
		u.event.Changes = append(u.event.Changes, "company_id")
	}

	if err := r.attributes(&u); err != nil {
		return c, err
	}

	if len(u.event.Changes) == 0 {
		return c, nil
	}

	contact, err := r.q.UpdateContact(ctx, u.params)
	if err != nil {
		return c, fmt.Errorf("update contact: %w", err)
	}

	if err := claimEmail(ctx, r.q, contact, r.now); err != nil {
		return contact, err
	}

	u.event.WorkspaceID, u.event.ContactID = contact.WorkspaceID, contact.ID
	u.event.ActorID = r.in.Membership.Member.ID

	return contact, r.a.Events.Dispatch(ctx, r.tx, u.event)
}

/*
attributes merges the custom attributes sent and keeps the trusted keys
in step with them.
*/
func (r upsertRun) attributes(u *update) error {
	changes := r.in.Fields.CustomAttributes

	merged, changed, err := attributes.Merge(u.before.CustomAttributes, changes)
	if err != nil || len(changed) == 0 {
		return err
	}

	trusted := slices.Clone(u.params.TrustedAttributeKeys)
	for _, key := range changed {
		trusted = slices.DeleteFunc(trusted, func(k string) bool { return k == key })
		if changes[key] != nil {
			trusted = append(trusted, key)
		}
	}

	slices.Sort(trusted)

	u.params.CustomAttributes, u.params.TrustedAttributeKeys = merged, trusted
	u.event.Changes = append(u.event.Changes, "custom_attributes")
	u.event.ChangedAttributeKeys = changed

	return nil
}

/*
leads returns the leads who typed the contact's address.
*/
func (r upsertRun) leads(ctx context.Context, c models.Contact) ([]models.Contact, error) {
	if c.Email == nil {
		return nil, nil
	}

	rows, err := r.q.ListUnverifiedEmailLeads(ctx, models.ListUnverifiedEmailLeadsParams{
		WorkspaceID: c.WorkspaceID, Email: c.Email, ID: c.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("find leads: %w", err)
	}

	return rows, nil
}
