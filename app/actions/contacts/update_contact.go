/*
|--------------------------------------------------------------------------
| Update Contact
|--------------------------------------------------------------------------
|
| Teammates correct a customer's details, add custom attributes, keep
| notes, tag and block or unblock them. Only sent fields change; a custom
| attribute set to null is removed. Attributes a teammate writes come
| from a trusted source, so their keys join the contact's trusted keys,
| and a removed key leaves them. The external id ties the contact to the
| host app's account, so changing or clearing it takes its own
| permission. A contact that gets an external id becomes a user.
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

	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/attributes"
)

/*
UpdateContactInput is who changes which contact and how. Fields holds
the details sent; ClearExternalID and ClearEmail and friends say a
field was sent as null. Blocked blocks or unblocks. AddTags and
RemoveTags change the tags following the contact. CompanySet says the
company was sent, CompanyID nil meaning none.
*/
type UpdateContactInput struct {
	Membership      authz.Membership
	ContactID       ids.UUID
	Fields          Fields
	ClearExternalID bool
	ClearEmail      bool
	ClearName       bool
	ClearPhone      bool
	Notes           *string
	ClearNotes      bool
	CompanySet      bool
	CompanyID       *ids.UUID
	AddTags         tags.Refs
	RemoveTags      []ids.UUID
	Blocked         *bool
}

/*
UpdateContact changes a contact.

Rules:
  - The caller needs contacts:write, and contacts:manage_external_id to
    change or clear the external id.
  - An external id another contact holds is refused.
  - A contact holds at most 50 tags.
  - Values equal to the current ones change nothing.

Events: [appevents.ContactUpdated] when anything changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrContactExists], [exceptions.ErrInvalidAttributes],
[exceptions.ErrTagLimitReached], [exceptions.ErrCompanyNotFound], or a
wrapped database error.
*/
type UpdateContact struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle applies the change and returns the contact as it now is.
*/
func (a *UpdateContact) Handle(ctx context.Context, in UpdateContactInput) (Detail, error) {
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

		lock := models.LockContactParams{WorkspaceID: current.WorkspaceID, ID: current.ID}
		if _, err := q.LockContact(ctx, lock); err != nil {
			return fmt.Errorf("lock contact: %w", err)
		}

		u := update{in: in, now: a.Clock.Now(), before: current}

		contact, err := u.write(ctx, q)
		if err != nil {
			return err
		}

		if out, err = LoadDetail(ctx, q, contact); err != nil || len(u.event.Changes) == 0 {
			return err
		}

		u.event.WorkspaceID, u.event.ContactID = contact.WorkspaceID, contact.ID
		u.event.ActorID = in.Membership.Member.ID

		return a.Events.Dispatch(ctx, tx, u.event)
	})

	return out, err
}

/*
update works out and writes one change to a contact, and the event
saying what changed.
*/
type update struct {
	in     UpdateContactInput
	now    time.Time
	before models.Contact
	params models.UpdateContactParams
	event  appevents.ContactUpdated
}

/*
paramsOf returns the contact's stored values as the parameters of an
update, so a change names only what differs.
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
write applies each field to the contact's current values and writes
them when something changed.
*/
func (u *update) write(ctx context.Context, q *models.Queries) (models.Contact, error) {
	b, f := u.before, u.in.Fields
	u.params = paramsOf(b, u.now)

	if err := u.externalID(ctx, q); err != nil {
		return b, err
	}

	u.text("email", &u.params.Email, sent(NormalizeEmail(f.Email), u.in.ClearEmail))

	if err := moveEmail(ctx, q, u); err != nil {
		return b, err
	}

	u.text("name", &u.params.Name, sent(f.Name, u.in.ClearName))
	u.text("phone", &u.params.Phone, sent(NormalizePhone(f.Phone), u.in.ClearPhone))
	u.text("notes", &u.params.Notes, sent(u.in.Notes, u.in.ClearNotes))

	if err := u.company(ctx, q); err != nil {
		return b, err
	}

	if err := u.attributes(); err != nil {
		return b, err
	}

	if err := u.tags(ctx, q); err != nil {
		return b, err
	}

	u.blocked()

	if len(u.event.Changes) == 0 {
		return b, nil
	}

	contact, err := q.UpdateContact(ctx, u.params)
	if err != nil {
		return b, fmt.Errorf("update contact: %w", err)
	}

	return contact, nil
}

/*
textField is what a request did to an optional text field: left it
out, sent a value, or cleared it.
*/
type textField struct {
	value   *string
	cleared bool
}

/*
sent returns what the request did to a field.
*/
func sent(value *string, cleared bool) textField {
	return textField{value: value, cleared: cleared}
}

/*
text applies one optional text field: the value sent, or nothing when
cleared.
*/
func (u *update) text(field string, current **string, f textField) {
	next := *current
	if f.value != nil {
		next = f.value
	}

	if f.cleared {
		next = nil
	}

	if sameText(*current, next) {
		return
	}

	*current = next
	u.event.Changes = append(u.event.Changes, field)
}

/*
externalID applies the external id, which takes its own permission and
must be free, and makes the contact a user when it gets one.
*/
func (u *update) externalID(ctx context.Context, q *models.Queries) error {
	before := u.params.ExternalID
	u.text("external_id", &u.params.ExternalID, sent(u.in.Fields.ExternalID, u.in.ClearExternalID))

	if sameText(before, u.params.ExternalID) {
		return nil
	}

	if !u.in.Membership.Can("contacts:manage_external_id") {
		return exceptions.ErrForbidden
	}

	if u.params.ExternalID == nil {
		return nil
	}

	u.params.Kind = string(enums.ContactKindUser)

	other, err := q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
		WorkspaceID: u.before.WorkspaceID, ExternalID: u.params.ExternalID,
	})
	if err == nil && other.ID != u.before.ID {
		return exceptions.ErrContactExists
	}

	return nil
}

/*
company applies the company, which must be a live one of the workspace.
*/
func (u *update) company(ctx context.Context, q *models.Queries) error {
	if !u.in.CompanySet || sameID(u.params.CompanyID, u.in.CompanyID) {
		return nil
	}

	if u.in.CompanyID != nil {
		c, err := q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: u.before.WorkspaceID, ID: *u.in.CompanyID})
		if errors.Is(err, database.ErrNotFound) || (err == nil && c.DeletedAt != nil) {
			return exceptions.ErrCompanyNotFound
		}

		if err != nil {
			return fmt.Errorf("find company: %w", err)
		}
	}

	u.params.CompanyID = u.in.CompanyID
	u.event.Changes = append(u.event.Changes, "company_id")

	return nil
}

/*
attributes merges the custom attributes sent and keeps the trusted keys
in step with them.
*/
func (u *update) attributes() error {
	if len(u.in.Fields.CustomAttributes) == 0 {
		return nil
	}

	merged, changed, err := attributes.Merge(u.before.CustomAttributes, u.in.Fields.CustomAttributes)
	if err != nil || len(changed) == 0 {
		return err
	}

	trusted := slices.Clone(u.params.TrustedAttributeKeys)
	for _, key := range changed {
		trusted = slices.DeleteFunc(trusted, func(k string) bool { return k == key })
		if u.in.Fields.CustomAttributes[key] != nil {
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
tags adds and removes the tags named, up to the limit on a contact.
*/
func (u *update) tags(ctx context.Context, q *models.Queries) error {
	in, ws := u.in, u.before.WorkspaceID

	var added []ids.UUID

	if !in.AddTags.Empty() {
		tagIDs, err := tags.Resolve(ctx, q, tags.Scope{WorkspaceID: ws, Now: u.now}, in.AddTags)
		if err != nil {
			return err
		}

		added, err = q.TagContact(ctx, models.TagContactParams{
			WorkspaceID: ws, ContactID: u.before.ID, TagIds: tagIDs, CreatedByType: "member",
			CreatedByID: &in.Membership.Member.ID, Now: u.now,
		})
		if err != nil {
			return fmt.Errorf("tag contact: %w", err)
		}
	}

	removed, err := q.UntagContact(ctx, models.UntagContactParams{
		WorkspaceID: ws, ContactID: u.before.ID, TagIds: in.RemoveTags,
	})
	if err != nil {
		return fmt.Errorf("untag contact: %w", err)
	}

	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	count, err := q.CountContactTags(ctx, models.CountContactTagsParams{WorkspaceID: ws, ContactID: u.before.ID})
	if err != nil {
		return fmt.Errorf("count contact tags: %w", err)
	}

	if count > tags.MaxOnRecord {
		return exceptions.ErrTagLimitReached
	}

	u.event.Changes = append(u.event.Changes, "tags")

	return nil
}

/*
blocked blocks or unblocks the contact.
*/
func (u *update) blocked() {
	if u.in.Blocked == nil || *u.in.Blocked == (u.before.BlockedAt != nil) {
		return
	}

	u.params.BlockedAt = nil
	if *u.in.Blocked {
		now := u.params.Now
		u.params.BlockedAt = &now
	}

	u.event.Changes = append(u.event.Changes, "blocked")
	u.event.Blocked, u.event.Unblocked = *u.in.Blocked, !*u.in.Blocked
}

/*
sameText reports whether two optional texts are equal.
*/
func sameText(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}

/*
sameID reports whether two optional ids are equal.
*/
func sameID(a, b *ids.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}
