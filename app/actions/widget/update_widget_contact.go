/*
|--------------------------------------------------------------------------
| Update Widget Contact
|--------------------------------------------------------------------------
|
| An anonymous visitor may tell the team who they are: a name, an email to
| reply to, a phone number, a few attributes. None of it is proven, so none
| of it can take anything from anyone: an address another contact already
| holds is stored on this lead only and never claimed, and every attribute
| the widget writes stops being trusted, so nothing a visitor types is
| treated as what the customer's own app said. A signed-in customer's
| profile comes from their app and is never changed here.
|
*/

package widget

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/fault"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/support/str"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits on what a visitor writes about themselves.
*/
const (
	maxContactName      = 120
	maxContactEmail     = 254
	maxWidgetAttributes = 20
	contactFieldInvalid = "validation_failed"
)

/*
ContactChange is what a visitor writes about themselves; a nil field is
left as it is.
*/
type ContactChange struct {
	Name             *string
	Email            *string
	Phone            *string
	CustomAttributes map[string]any
}

/*
UpdateWidgetContact writes what a lead tells about themselves.

Events: [appevents.ContactUpdated].
*/
type UpdateWidgetContact struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle writes the change to the session's contact and returns it as it now
is.
*/
func (a *UpdateWidgetContact) Handle(
	ctx context.Context, session models.WidgetSession, change ContactChange,
) (models.Contact, error) {
	clean, err := cleanContactChange(change)
	if err != nil {
		return models.Contact{}, err
	}

	var out models.Contact

	err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		current, err := q.LockContact(ctx, models.LockContactParams{
			WorkspaceID: session.WorkspaceID, ID: session.ContactID,
		})
		if err != nil {
			return fmt.Errorf("lock widget contact: %w", err)
		}

		if current.Kind != string(enums.ContactKindLead) {
			return exceptions.ErrIdentityLocked
		}

		out, err = a.write(ctx, tx, current, clean)

		return err
	})

	return out, err
}

/*
write stores the change and tells who listens what changed.
*/
func (a *UpdateWidgetContact) write(
	ctx context.Context, tx *database.Tx, current models.Contact, change ContactChange,
) (models.Contact, error) {
	q, now := models.New(tx), a.Clock.Now()
	next, changes := applyContactChange(current, change)

	attrs, written, err := contacts.MergeAttributes(current.CustomAttributes, change.CustomAttributes)
	if err != nil {
		return current, err
	}

	if len(written) > 0 {
		changes = append(changes, "custom_attributes")
	}

	if len(changes) == 0 {
		return current, nil
	}

	updated, err := q.UpdateContact(ctx, models.UpdateContactParams{
		Kind: next.Kind, ExternalID: next.ExternalID, Email: next.Email, Name: next.Name, Phone: next.Phone,
		CustomAttributes: attrs, BlockedAt: next.BlockedAt, Now: now, WorkspaceID: next.WorkspaceID, ID: next.ID,
		TrustedAttributeKeys: slices.DeleteFunc(slices.Clone(current.TrustedAttributeKeys), func(k string) bool {
			return slices.Contains(written, k)
		}),
	})
	if err != nil {
		return current, fmt.Errorf("update widget contact: %w", err)
	}

	if slices.Contains(changes, "email") && updated.Email != nil {
		err := q.ReplaceWidgetContactEmailIdentity(ctx, models.ReplaceWidgetContactEmailIdentityParams{
			WorkspaceID: updated.WorkspaceID, ContactID: updated.ID, Email: *updated.Email, Now: now,
		})
		if err != nil {
			return updated, fmt.Errorf("record typed email: %w", err)
		}
	}

	return updated, a.Events.Dispatch(ctx, tx, appevents.ContactUpdated{
		WorkspaceID: updated.WorkspaceID, ContactID: updated.ID, ActorID: ids.Nil(), Changes: changes,
		ChangedAttributeKeys: written,
	})
}

/*
applyContactChange returns the contact with the change applied and the
fields that changed.
*/
func applyContactChange(c models.Contact, change ContactChange) (models.Contact, []string) {
	var changes []string

	if change.Name != nil && (c.Name == nil || *c.Name != *change.Name) {
		c.Name, changes = change.Name, append(changes, "name")
	}

	if change.Email != nil && (c.Email == nil || *c.Email != *change.Email) {
		c.Email, changes = change.Email, append(changes, "email")
	}

	if change.Phone != nil && (c.Phone == nil || *c.Phone != *change.Phone) {
		c.Phone, changes = change.Phone, append(changes, "phone")
	}

	return c, changes
}

/*
cleanContactChange returns the change in normal form, or the first field
it refuses.
*/
func cleanContactChange(change ContactChange) (ContactChange, error) {
	if change.Name != nil {
		name := str.Clean(*change.Name)
		if name == "" || utf8.RuneCountInString(name) > maxContactName {
			return change, contactField("name", "Use 1 to 120 characters.")
		}

		change.Name = &name
	}

	if change.Email != nil {
		email, ok := normalizeTypedEmail(*change.Email)
		if !ok {
			return change, contactField("email", "Use an email address.")
		}

		change.Email = &email
	}

	if change.Phone != nil {
		phone := strings.ReplaceAll(strings.TrimSpace(*change.Phone), " ", "")
		if !e164().MatchString(phone) {
			return change, contactField("phone", "Use the international format, such as +4930123456.")
		}

		change.Phone = &phone
	}

	if len(change.CustomAttributes) > maxWidgetAttributes {
		return change, contactField("custom_attributes", "Send at most 20 attributes at once.")
	}

	change.CustomAttributes = maps.Clone(change.CustomAttributes)

	return change, nil
}

/*
normalizeTypedEmail returns a bare address with its domain in ASCII form,
and false for anything else.
*/
func normalizeTypedEmail(raw string) (string, bool) {
	raw = strings.TrimSpace(raw)

	parsed, err := mail.ParseAddress(raw)
	if err != nil || parsed.Name != "" || parsed.Email != raw || len(raw) > maxContactEmail {
		return "", false
	}

	at := strings.LastIndex(raw, "@")

	domain, err := customdomain.Normalize(raw[at+1:])
	if err != nil {
		return "", false
	}

	return strings.ToLower(raw[:at]) + "@" + domain, true
}

/*
e164 matches a phone number in international format.
*/
func e164() *regexp.Regexp {
	return regexp.MustCompile(`^\+[1-9]\d{6,14}$`)
}

/*
contactField returns the refusal of one field.
*/
func contactField(field, message string) *fault.Error {
	return fault.New(fault.Invalid, contactFieldInvalid, message).OnField(field)
}
