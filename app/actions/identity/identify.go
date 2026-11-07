/*
|--------------------------------------------------------------------------
| Identify Visitor
|--------------------------------------------------------------------------
|
| A verified token names a customer by the host app's own id, and that id
| is one contact across every widget of the workspace. The first sign-in
| makes the contact, or, when an anonymous lead was already talking, turns
| that lead into the customer so the conversation carries on. A lead who
| signs in as a customer we already know is merged into them: their
| threads move over and the lead stays behind as a pointer. A customer is
| never merged into another customer, whatever their addresses say.
| Everything runs in the caller's transaction, and a token that changes
| nothing writes nothing.
|
*/

package identity

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
Visitor is who to identify: the verified profile, and the anonymous
lead the visitor was before signing in, nil when there was none.
*/
type Visitor struct {
	WorkspaceID ids.UUID
	Profile     Profile
	Lead        *models.Contact
	Now         time.Time
}

/*
Identified is the customer the token named, whether they were just
added, and the sessions of a merged lead that ended.
*/
type Identified struct {
	Contact models.Contact
	Created bool
	Ended   []Ended
}

/*
Identify finds or makes the contact the profile names and writes what
the token says about them.

Events: [appevents.ContactCreated] or [appevents.ContactUpdated].
*/
func Identify(ctx context.Context, tx *database.Tx, dispatcher *events.Dispatcher, v Visitor,
) (Identified, error) {
	q := models.New(tx)
	if err := q.LockContactExternalID(ctx, models.LockContactExternalIDParams{
		WorkspaceID: v.WorkspaceID.String(), ExternalID: v.Profile.ExternalID,
	}); err != nil {
		return Identified{}, fmt.Errorf("lock customer: %w", err)
	}
	known, found, err := findCustomer(ctx, q, v)
	if err != nil {
		return Identified{}, err
	}
	run := &identifyRun{q: q, tx: tx, v: v, dispatcher: dispatcher}
	switch {
	case found && v.Lead != nil && v.Lead.ID != known.ID && v.Lead.Kind == string(enums.ContactKindLead):
		return run.merge(ctx, known, *v.Lead)
	case found:
		return run.apply(ctx, known)
	case v.Lead != nil && v.Lead.Kind == string(enums.ContactKindLead) && v.Lead.ExternalID == nil:
		return run.apply(ctx, *v.Lead)
	}
	return run.create(ctx)
}

/*
findCustomer returns the live contact holding the external id.
*/
func findCustomer(ctx context.Context, q *models.Queries, v Visitor) (models.Contact, bool, error) {
	c, err := q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
		WorkspaceID: v.WorkspaceID, ExternalID: &v.Profile.ExternalID,
	})
	if errors.Is(err, database.ErrNotFound) {
		return c, false, nil
	}
	if err != nil {
		return c, false, fmt.Errorf("find customer: %w", err)
	}
	survivor, err := contacts.Survivor(ctx, q, v.WorkspaceID, c.ID)
	return survivor, err == nil, err
}

/*
identifyRun is one identification in its transaction, and whether it
added the contact.
*/
type identifyRun struct {
	q          *models.Queries
	tx         *database.Tx
	v          Visitor
	dispatcher *events.Dispatcher
	created    bool
}

/*
create adds the customer.
*/
func (r *identifyRun) create(ctx context.Context) (Identified, error) {
	c, err := r.q.CreateContact(ctx, models.CreateContactParams{
		WorkspaceID: r.v.WorkspaceID, Kind: string(enums.ContactKindUser), ExternalID: &r.v.Profile.ExternalID,
		CustomAttributes: json.RawMessage(`{}`), Now: r.v.Now,
	})
	if err != nil {
		return Identified{}, fmt.Errorf("create customer: %w", err)
	}
	r.created = true
	return r.apply(ctx, c)
}

/*
merge moves a lead's conversations to the customer it signed in as and
leaves the lead pointing at them. The lead's other open messengers end,
since an unproven browser must never see a customer's history.
*/
func (r *identifyRun) merge(ctx context.Context, into, lead models.Contact) (Identified, error) {
	ws, now := r.v.WorkspaceID, r.v.Now
	if err := r.q.MoveLeadConversations(ctx, models.MoveLeadConversationsParams{
		IntoContactID: into.ID, Now: now, WorkspaceID: ws, ContactID: lead.ID,
	}); err != nil {
		return Identified{}, fmt.Errorf("move lead conversations: %w", err)
	}
	if err := r.q.MoveLeadParticipations(ctx, models.MoveLeadParticipationsParams{
		IntoContactID: into.ID, WorkspaceID: ws, ContactID: lead.ID,
	}); err != nil {
		return Identified{}, fmt.Errorf("move lead participations: %w", err)
	}
	if err := r.q.MoveLeadActionCalls(ctx, models.MoveLeadActionCallsParams{
		IntoContactID: &into.ID, WorkspaceID: ws, ContactID: &lead.ID,
	}); err != nil {
		return Identified{}, fmt.Errorf("move lead action calls: %w", err)
	}
	if err := r.q.MarkContactMerged(ctx, models.MarkContactMergedParams{
		IntoContactID: &into.ID, Now: now, WorkspaceID: ws, ID: lead.ID,
	}); err != nil {
		return Identified{}, fmt.Errorf("mark lead merged: %w", err)
	}
	rows, err := r.q.RevokeWidgetSessionsForContact(ctx, models.RevokeWidgetSessionsForContactParams{
		Now: now, Reason: string(enums.WidgetRevokeContactMerged), WorkspaceID: ws, ContactID: lead.ID,
	})
	if err != nil {
		return Identified{}, fmt.Errorf("end merged lead sessions: %w", err)
	}
	out, err := r.apply(ctx, into)
	for _, row := range rows {
		out.Ended = append(out.Ended, Ended{SessionID: row.ID, ContactID: row.ContactID})
	}
	return out, err
}

/*
apply writes the token's claims over the contact and records its
verified identities.
*/
func (r *identifyRun) apply(ctx context.Context, c models.Contact) (Identified, error) {
	p := r.v.Profile
	ch := change{before: c, params: paramsOf(c, r.v.Now)}
	ch.externalID(p.ExternalID)
	ch.text("name", &ch.params.Name, p.Name)
	ch.text("phone", &ch.params.Phone, p.Phone)
	ch.email(p)
	if err := r.company(ctx, &ch, p.Company); err != nil {
		return Identified{}, err
	}
	ch.attributes(p.Attributes)
	contact, err := r.write(ctx, ch)
	if err != nil {
		return Identified{}, err
	}
	if contact, err = r.profile(ctx, contact, &ch); err != nil {
		return Identified{}, err
	}
	if err := r.identities(ctx, contact); err != nil {
		return Identified{}, err
	}
	return Identified{Contact: contact, Created: r.created}, r.announce(ctx, contact, ch)
}

/*
company finds or adds the company the token names and links it.
*/
func (r *identifyRun) company(ctx context.Context, ch *change, f *companies.Fields) error {
	if f == nil {
		return nil
	}
	company, _, err := companies.Identify(ctx, r.tx, companies.Identity{
		WorkspaceID: r.v.WorkspaceID, Fields: *f, Now: r.v.Now,
	})
	if err != nil {
		return fmt.Errorf("identify company: %w", err)
	}
	if ch.params.CompanyID == nil || *ch.params.CompanyID != company.ID {
		ch.params.CompanyID = &company.ID
		ch.changes = append(ch.changes, "company_id")
	}
	return nil
}

/*
write stores the contact when anything changed.
*/
func (r *identifyRun) write(ctx context.Context, ch change) (models.Contact, error) {
	if len(ch.changes) == 0 {
		return ch.before, nil
	}
	c, err := r.q.UpdateContact(ctx, ch.params)
	if err != nil {
		return c, fmt.Errorf("update customer: %w", err)
	}
	return c, nil
}

/*
profile writes the avatar, language and time zone the token gives.
*/
func (r *identifyRun) profile(ctx context.Context, c models.Contact, ch *change) (models.Contact, error) {
	p := r.v.Profile
	moved := differs(c.AvatarUrl, p.AvatarURL) || differs(c.Locale, p.Locale) || differs(c.Timezone, p.Timezone)
	if !moved {
		return c, nil
	}
	out, err := r.q.ApplyContactProfile(ctx, models.ApplyContactProfileParams{
		AvatarUrl: p.AvatarURL, Locale: p.Locale, Timezone: p.Timezone, Now: r.v.Now,
		WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return c, fmt.Errorf("update customer profile: %w", err)
	}
	ch.changes = append(ch.changes, "profile")
	return out, nil
}

/*
identities records the external id, and the email when the token
proves it, as verified identities of the contact.
*/
func (r *identifyRun) identities(ctx context.Context, c models.Contact) error {
	type proven struct {
		kind  enums.IdentityType
		value string
	}
	values := []proven{{enums.IdentityTypeExternalID, r.v.Profile.ExternalID}}
	if r.v.Profile.EmailVerified && r.v.Profile.Email != nil {
		values = append(values, proven{enums.IdentityTypeEmail, *r.v.Profile.Email})
	}
	for _, p := range values {
		if err := r.q.UpsertContactIdentity(ctx, models.UpsertContactIdentityParams{
			WorkspaceID: c.WorkspaceID, ContactID: c.ID, Type: string(p.kind), Value: p.value, Now: r.v.Now,
		}); err != nil {
			return fmt.Errorf("record %s identity: %w", p.kind, err)
		}
	}
	return nil
}

/*
announce tells the rest of the product the contact was added or
changed.
*/
func (r *identifyRun) announce(ctx context.Context, c models.Contact, ch change) error {
	if r.created {
		return r.dispatcher.Dispatch(ctx, r.tx, appevents.ContactCreated{
			WorkspaceID: c.WorkspaceID, ContactID: c.ID, ActorID: ids.Nil(),
		})
	}
	if len(ch.changes) == 0 {
		return nil
	}
	return r.dispatcher.Dispatch(ctx, r.tx, appevents.ContactUpdated{
		WorkspaceID: c.WorkspaceID, ContactID: c.ID, ActorID: ids.Nil(), Changes: ch.changes,
		ChangedAttributeKeys: ch.attributeKeys,
	})
}

/*
change is a contact's update being built: what it was, the new values
and the fields that moved.
*/
type change struct {
	before        models.Contact
	params        models.UpdateContactParams
	changes       []string
	attributeKeys []string
}

/*
externalID makes the contact the signed-in customer: a user with the
host app's id.
*/
func (c *change) externalID(id string) {
	if c.params.ExternalID != nil && *c.params.ExternalID == id && c.params.Kind == string(enums.ContactKindUser) {
		return
	}
	c.params.ExternalID, c.params.Kind = &id, string(enums.ContactKindUser)
	c.changes = append(c.changes, "external_id")
}

func (c *change) text(field string, current **string, value *string) {
	if value == nil || !differs(*current, value) {
		return
	}
	*current = value
	c.changes = append(c.changes, field)
}

/*
email takes the token's address, proven only when the token says so.
*/
func (c *change) email(p Profile) {
	if p.Email == nil {
		return
	}
	if differs(c.params.Email, p.Email) {
		c.params.Email, c.params.EmailVerified = p.Email, p.EmailVerified
		c.changes = append(c.changes, "email")
		return
	}
	if p.EmailVerified && !c.params.EmailVerified {
		c.params.EmailVerified = true
		c.changes = append(c.changes, "email_verified")
	}
}

/*
attributes merges the token's custom attributes and marks each one it
wrote as trusted; a removed one stops being trusted.
*/
func (c *change) attributes(sent map[string]any) {
	if len(sent) == 0 {
		return
	}
	merged, changed, err := attributes.Merge(c.params.CustomAttributes, sent)
	if err != nil || len(changed) == 0 {
		return
	}
	trusted := slices.Clone(c.params.TrustedAttributeKeys)
	for _, key := range changed {
		trusted = slices.DeleteFunc(trusted, func(k string) bool { return k == key })
		if sent[key] != nil {
			trusted = append(trusted, key)
		}
	}
	slices.Sort(trusted)
	c.params.CustomAttributes, c.params.TrustedAttributeKeys = merged, trusted
	c.changes = append(c.changes, "custom_attributes")
	c.attributeKeys = changed
}

/*
paramsOf returns the contact's stored values as an update's parameters.
*/
func paramsOf(c models.Contact, now time.Time) models.UpdateContactParams {
	return models.UpdateContactParams{
		Kind: c.Kind, ExternalID: c.ExternalID, Email: c.Email, Name: c.Name, Phone: c.Phone,
		CustomAttributes: c.CustomAttributes, TrustedAttributeKeys: c.TrustedAttributeKeys, BlockedAt: c.BlockedAt,
		Notes: c.Notes, CompanyID: c.CompanyID, EmailVerified: c.EmailVerified, Now: now,
		WorkspaceID: c.WorkspaceID, ID: c.ID,
	}
}

/*
differs reports whether a sent value is new; nothing sent never is.
*/
func differs(current, sent *string) bool {
	if sent == nil {
		return false
	}
	return current == nil || *current != *sent
}
