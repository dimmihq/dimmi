/*
|--------------------------------------------------------------------------
| Create Contact
|--------------------------------------------------------------------------
|
| Teammates and the API add customers. One the host app knows by its own
| id is a user; anyone else is a lead. An address added by a teammate or
| a trusted API call counts as proven, and becomes the contact's email
| identity unless another contact already holds it. An external id names
| one contact in a workspace, so a second contact with the same one is
| refused. Custom attributes a teammate or the API writes count as
| trusted.
|
| Starting a conversation names its customer the same way: by contact
| id, or by external id or email address, finding the contact those
| belong to and adding one when there is none. An address only finds a
| user or a contact whose address was proven, never a lead who typed it.
|
*/

package contacts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Fields are a contact's details as a request gives them, nil when left
out.
*/
type Fields struct {
	ExternalID       *string
	Email            *string
	Name             *string
	Phone            *string
	CustomAttributes map[string]any
}

/*
CreateContactInput is who adds which contact.
*/
type CreateContactInput struct {
	Membership authz.Membership
	Fields     Fields
}

/*
CreateContact adds a contact to the workspace.

Rules:
  - The caller needs contacts:write.
  - An external id another contact holds is refused.

Events: [appevents.ContactCreated].

Errors: [exceptions.ErrForbidden], [exceptions.ErrContactExists],
[exceptions.ErrInvalidAttributes], or a wrapped database error.
*/
type CreateContact struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle adds the contact and returns it.
*/
func (a *CreateContact) Handle(ctx context.Context, in CreateContactInput) (models.Contact, error) {
	if !in.Membership.Can("contacts:write") {
		return models.Contact{}, exceptions.ErrForbidden
	}

	var out models.Contact

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error

		n := NewContact{WorkspaceID: in.Membership.Workspace.ID, Fields: in.Fields}

		out, err = Create(ctx, tx, a.Clock.Now(), n)
		if err != nil {
			return err
		}

		return a.Events.Dispatch(ctx, tx, appevents.ContactCreated{
			WorkspaceID: out.WorkspaceID, ContactID: out.ID, ActorID: in.Membership.Member.ID,
		})
	})

	return out, err
}

/*
NewContact is a contact to add to the workspace.
*/
type NewContact struct {
	WorkspaceID ids.UUID
	Fields      Fields
}

/*
Create writes a new contact of the workspace with its email identity.
*/
func Create(ctx context.Context, tx *database.Tx, now time.Time, n NewContact) (models.Contact, error) {
	q, ws, f := models.New(tx), n.WorkspaceID, n.Fields

	if f.ExternalID != nil {
		_, err := q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: ws, ExternalID: f.ExternalID,
		})
		if err == nil {
			return models.Contact{}, exceptions.ErrContactExists
		}

		if !errors.Is(err, database.ErrNotFound) {
			return models.Contact{}, fmt.Errorf("find contact: %w", err)
		}
	}

	attrs, keys, err := MergeAttributes(nil, f.CustomAttributes)
	if err != nil {
		return models.Contact{}, err
	}

	kind := enums.ContactKindLead
	if f.ExternalID != nil {
		kind = enums.ContactKindUser
	}

	email := NormalizeEmail(f.Email)

	contact, err := q.CreateContact(ctx, models.CreateContactParams{
		WorkspaceID: ws, Kind: string(kind), ExternalID: f.ExternalID, Email: email, EmailVerified: email != nil,
		Name: f.Name, Phone: NormalizePhone(f.Phone), CustomAttributes: attrs, Now: now,
	})
	if err != nil {
		return contact, fmt.Errorf("create contact: %w", err)
	}

	if contact, err = trust(ctx, q, contact, keys); err != nil {
		return contact, err
	}

	return contact, claimEmail(ctx, q, contact, now)
}

/*
trust records the custom attributes written as coming from a trusted
source, since teammates and the API write them.
*/
func trust(ctx context.Context, q *models.Queries, c models.Contact, keys []string) (models.Contact, error) {
	if len(keys) == 0 {
		return c, nil
	}

	trusted, err := q.UpdateContact(ctx, models.UpdateContactParams{
		Kind: c.Kind, ExternalID: c.ExternalID, Email: c.Email, Name: c.Name, Phone: c.Phone,
		CustomAttributes: c.CustomAttributes, TrustedAttributeKeys: keys, BlockedAt: c.BlockedAt, Now: c.UpdatedAt,
		WorkspaceID: c.WorkspaceID, ID: c.ID,
	})
	if err != nil {
		return c, fmt.Errorf("trust custom attributes: %w", err)
	}

	return trusted, nil
}

/*
claimEmail records the contact's address as its email identity, unless
another contact of the workspace already holds it.
*/
func claimEmail(ctx context.Context, q *models.Queries, c models.Contact, now time.Time) error {
	if c.Email == nil {
		return nil
	}

	_, err := q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
		WorkspaceID: c.WorkspaceID, Type: "email", Value: *c.Email,
	})
	if err == nil {
		return nil
	}

	if !errors.Is(err, database.ErrNotFound) {
		return fmt.Errorf("find email identity: %w", err)
	}

	_, err = q.CreateContactIdentity(ctx, models.CreateContactIdentityParams{
		WorkspaceID: c.WorkspaceID, ContactID: c.ID, Type: "email", Value: *c.Email, Verified: true, Now: now,
	})
	if err != nil {
		return fmt.Errorf("create email identity: %w", err)
	}

	return nil
}

/*
Ref names a customer of the workspace: by ID, or by ExternalID or Email
with Name to use when they have to be added.
*/
type Ref struct {
	WorkspaceID ids.UUID
	ID          *ids.UUID
	ExternalID  *string
	Email       *string
	Name        *string
}

/*
Resolve returns the live contact the reference names, adding one when
an external id or email address finds none.
*/
func Resolve(ctx context.Context, tx *database.Tx, now time.Time, ref Ref) (models.Contact, error) {
	q, ws := models.New(tx), ref.WorkspaceID

	switch {
	case ref.ID != nil:
		return Survivor(ctx, q, ws, *ref.ID)
	case ref.ExternalID != nil:
		c, err := q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: ws, ExternalID: ref.ExternalID,
		})
		if err == nil {
			return Survivor(ctx, q, ws, c.ID)
		}

		if !errors.Is(err, database.ErrNotFound) {
			return c, fmt.Errorf("find contact: %w", err)
		}
	case ref.Email != nil:
		c, err := q.FindContactForEmail(ctx, models.FindContactForEmailParams{
			WorkspaceID: ws, Email: NormalizeEmail(ref.Email),
		})
		if err == nil {
			return c, nil
		}

		if !errors.Is(err, database.ErrNotFound) {
			return c, fmt.Errorf("find contact: %w", err)
		}
	default:
		return models.Contact{}, exceptions.ErrContactKeyRequired
	}

	return Create(ctx, tx, now, NewContact{WorkspaceID: ws, Fields: Fields{
		ExternalID: ref.ExternalID, Email: ref.Email, Name: ref.Name, CustomAttributes: map[string]any{},
	}})
}

/*
Survivor returns the live contact with the id, following a merged
contact to the one it was merged into.
*/
func Survivor(ctx context.Context, q *models.Queries, ws, id ids.UUID) (models.Contact, error) {
	for range maxMergeHops {
		c, err := q.FindContact(ctx, models.FindContactParams{WorkspaceID: ws, ID: id})
		if errors.Is(err, database.ErrNotFound) {
			return c, exceptions.ErrNotFound
		}

		if err != nil {
			return c, fmt.Errorf("find contact: %w", err)
		}

		if c.MergedIntoID == nil {
			if c.DeletedAt != nil {
				return c, exceptions.ErrNotFound
			}

			return c, nil
		}

		id = *c.MergedIntoID
	}

	return models.Contact{}, exceptions.ErrNotFound
}

/*
maxMergeHops is how many merges are followed before giving up, which
only a merge loop would ever reach.
*/
const maxMergeHops = 10
