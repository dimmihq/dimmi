/*
|--------------------------------------------------------------------------
| Companies, Contacts And Notes
|--------------------------------------------------------------------------
|
| A company is matched by external id, then domain, a contact by
| external id, then email; one with neither is the migration's own,
| found again by its ledger row. Notes keep their author and time.
|
*/

package migrations

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/models"
	supportattributes "github.com/dimmihq/dimmi/app/support/attributes"
)

/*
Companies are matched by external id, then domain; one with neither is
the migration's own, found again by its ledger row.
*/
func (c *Company) validate() error { return nil }

func (c *Company) refs() []Ref { return refsOf(KindTag, c.TagRefs...) }

func (c *Company) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: c.Name}
	fields := companies.Fields{
		ExternalID: orNil(c.ExternalID), Name: orNil(c.Name), Domain: orNil(c.Domain),
		CustomAttributes: c.CustomAttributes,
	}

	company, created, err := s.company(ctx, fields)
	if err != nil {
		return out, err
	}

	out.id, out.created = company.ID, created

	if c.CreatedAt != nil {
		if err := s.q.MigrateCompanyTimes(ctx, models.MigrateCompanyTimesParams{
			CreatedAt: c.CreatedAt.UTC(), WorkspaceID: s.ws(), ID: company.ID,
		}); err != nil {
			return out, fmt.Errorf("backdate company: %w", err)
		}
	}

	if tagged := s.refs.ids(KindTag, c.TagRefs); len(tagged) > 0 {
		if _, err := s.q.TagCompany(ctx, models.TagCompanyParams{
			WorkspaceID: s.ws(), CompanyID: company.ID, TagIds: tagged, CreatedByType: "system",
			CreatedByID: nil, Now: s.now,
		}); err != nil {
			return out, fmt.Errorf("tag company: %w", err)
		}
	}

	return out, nil
}

/*
company writes the company: the one this record made before, else the
one with its external id or domain, else a new one.
*/
func (s *step) company(ctx context.Context, f companies.Fields) (models.Company, bool, error) {
	if s.prev != nil && s.prev.RecordID != nil {
		current, err := s.q.FindCompany(ctx, models.FindCompanyParams{WorkspaceID: s.ws(), ID: *s.prev.RecordID})
		if err == nil && current.DeletedAt == nil {
			return s.overwriteCompany(ctx, current, f)
		}
	}

	if f.ExternalID != nil || companies.NormalizeDomain(f.Domain) != nil {
		c, created, err := companies.Identify(ctx, s.tx, companies.Identity{WorkspaceID: s.ws(), Fields: f, Now: s.now})

		return c, created, err
	}

	attrs, err := s.gate(ctx, attributedefs.ObjectCompany, f.CustomAttributes)
	if err != nil {
		return models.Company{}, false, err
	}

	merged, _, err := supportattributes.Merge(nil, attrs)
	if err != nil {
		return models.Company{}, false, err
	}

	name := "Imported company"
	if f.Name != nil {
		name = *f.Name
	}

	c, err := s.q.CreateCompany(ctx, models.CreateCompanyParams{
		WorkspaceID: s.ws(), Name: name, CustomAttributes: merged, Now: s.now,
	})
	if err != nil {
		return c, false, fmt.Errorf("create company: %w", err)
	}

	return c, true, nil
}

/*
overwriteCompany writes the fields sent over the company this record
made before.
*/
func (s *step) overwriteCompany(ctx context.Context, current models.Company, f companies.Fields,
) (models.Company, bool, error) {
	attrs, err := s.gate(ctx, attributedefs.ObjectCompany, f.CustomAttributes)
	if err != nil {
		return current, false, err
	}

	merged, _, err := supportattributes.Merge(current.CustomAttributes, attrs)
	if err != nil {
		return current, false, err
	}

	params := models.UpdateCompanyParams{
		ExternalID: current.ExternalID, Name: current.Name, Domain: current.Domain, CustomAttributes: merged,
		Notes: current.Notes, Now: s.now, WorkspaceID: current.WorkspaceID, ID: current.ID,
	}
	if f.ExternalID != nil {
		params.ExternalID = f.ExternalID
	}

	if f.Name != nil {
		params.Name = *f.Name
	}

	if d := companies.NormalizeDomain(f.Domain); d != nil {
		params.Domain = d
	}

	c, err := s.q.UpdateCompany(ctx, params)
	if err != nil {
		return current, false, fmt.Errorf("update company: %w", err)
	}

	return c, false, nil
}

/*
gate keeps the custom attributes the workspace's definitions allow,
defining new keys as the import trusts them.
*/
func (s *step) gate(ctx context.Context, object string, attrs map[string]any) (map[string]any, error) {
	if len(attrs) == 0 {
		return nil, nil
	}

	return attributedefs.Gate(ctx, s.q, attributedefs.Write{
		WorkspaceID: s.ws(), Object: object, Trusted: true, Changes: attrs, Now: s.now,
	})
}

/*
Contacts are matched by external id, then email; one with neither, an
anonymous visitor, is the migration's own, found again by its ledger
row.
*/
func (c *Contact) validate() error {
	if err := oneOf("kind", c.Kind, "user", "lead"); err != nil {
		return err
	}

	defaults := 0

	for i, link := range c.Companies {
		if link.Ref == "" {
			return invalid(fmt.Sprintf("companies[%d].ref", i), "A company link names its company in ref.")
		}

		if link.Default {
			defaults++
		}
	}

	if defaults > 1 {
		return invalid("companies", "At most one company is the contact's default.")
	}

	return nil
}

func (c *Contact) refs() []Ref {
	out := refsOf(KindTag, c.TagRefs...)
	for _, link := range c.Companies {
		out = append(out, refsOf(KindCompany, link.Ref)...)
	}

	return out
}

func (c *Contact) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: c.label()}

	contact, created, err := s.contact(ctx, c)
	if err != nil {
		return out, err
	}

	out.id, out.created = contact.ID, created

	company, others := c.defaultCompany(s)
	if others {
		out.unsupported("companies")
	}

	if err := s.writeContact(ctx, contact, c, company); err != nil {
		return out, err
	}

	if tagged := s.refs.ids(KindTag, c.TagRefs); len(tagged) > 0 {
		if _, err := s.q.TagContact(ctx, models.TagContactParams{
			WorkspaceID: s.ws(), ContactID: contact.ID, TagIds: tagged, CreatedByType: "system", Now: s.now,
		}); err != nil {
			return out, fmt.Errorf("tag contact: %w", err)
		}
	}

	return out, nil
}

/*
label returns what the contact is called, for records that name it.
*/
func (c *Contact) label() string {
	for _, v := range []string{c.Name, c.Email, c.ExternalID} {
		if v != "" {
			return v
		}
	}

	return "Visitor"
}

/*
defaultCompany returns the company the contact belongs to by default,
the first when none is marked, and reports whether it belongs to
others too, which waits for several companies per contact.
*/
func (c *Contact) defaultCompany(s *step) (*ids.UUID, bool) {
	if len(c.Companies) == 0 {
		return nil, false
	}

	pick := c.Companies[0]
	for _, link := range c.Companies {
		if link.Default {
			pick = link
		}
	}

	id, ok := s.refs.id(KindCompany, pick.Ref)
	if !ok {
		return nil, len(c.Companies) > 1
	}

	return &id, len(c.Companies) > 1
}

/*
contact finds the contact this record made before, else the one with
its external id or email, else adds one.
*/
func (s *step) contact(ctx context.Context, c *Contact) (models.Contact, bool, error) {
	if found, ok := s.matchContact(ctx, c); ok {
		return found, false, nil
	}

	kind := c.Kind
	if kind == "" {
		kind = "user"
		if c.ExternalID == "" && c.Email == "" {
			kind = "lead"
		}
	}

	at := timeOr(c.CreatedAt, timeOr(c.FirstSeenAt, s.now))

	made, err := s.q.CreateContact(ctx, models.CreateContactParams{
		WorkspaceID: s.ws(), Kind: kind, ExternalID: orNil(c.ExternalID), Email: contacts.NormalizeEmail(&c.Email),
		Name: orNil(c.Name), Phone: contacts.NormalizePhone(orNil(c.Phone)), CustomAttributes: []byte("{}"), Now: at,
	})
	if err != nil {
		return made, false, fmt.Errorf("add contact: %w", err)
	}

	return made, true, nil
}

/*
matchContact returns the live contact this record made before, else the
one with its external id, else the one with its email, and reports
whether there is one.
*/
func (s *step) matchContact(ctx context.Context, c *Contact) (models.Contact, bool) {
	if s.prev != nil && s.prev.RecordID != nil {
		found, err := s.q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.ws(), ID: *s.prev.RecordID})
		if err == nil && found.DeletedAt == nil {
			return found, true
		}
	}

	if c.ExternalID != "" {
		found, err := s.q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: s.ws(), ExternalID: &c.ExternalID,
		})
		if err == nil && found.DeletedAt == nil {
			return found, true
		}
	}

	email := contacts.NormalizeEmail(&c.Email)
	if email == nil {
		return models.Contact{}, false
	}

	found, err := s.q.FindImportContactByEmail(ctx, models.FindImportContactByEmailParams{
		WorkspaceID: s.ws(), Email: *email,
	})

	return found, err == nil
}

/*
writeContact writes every detail the record sent over the contact.
*/
func (s *step) writeContact(ctx context.Context, contact models.Contact, c *Contact, company *ids.UUID) error {
	attrs, err := s.gate(ctx, attributedefs.ObjectContact, c.CustomAttributes)
	if err != nil {
		return err
	}

	rawAttrs, err := json.Marshal(nonNilMap(attrs))
	if err != nil {
		return fmt.Errorf("encode attributes: %w", err)
	}

	var location json.RawMessage
	if c.Location != nil {
		location = mustJSON(c.Location)
	}

	kind := orNil(c.Kind)

	_, err = s.q.MigrateContact(ctx, models.MigrateContactParams{
		Kind: kind, ExternalID: externalIDFor(contact, c.ExternalID), Email: contacts.NormalizeEmail(&c.Email),
		Name: orNil(c.Name), Phone: contacts.NormalizePhone(orNil(c.Phone)), AvatarUrl: orNil(c.AvatarURL),
		Locale: orNil(c.Locale), Timezone: orNil(c.Timezone), Location: location, CompanyID: company,
		CustomAttributes: rawAttrs, FirstSeenAt: utc(c.FirstSeenAt), LastSeenAt: utc(c.LastSeenAt),
		CreatedAt: utc(c.CreatedAt), Now: s.now, WorkspaceID: s.ws(), ID: contact.ID,
	})
	if err != nil {
		return fmt.Errorf("write contact: %w", err)
	}

	return nil
}

/*
externalIDFor returns the external id to give the contact: the one
sent, unless the contact already holds another.
*/
func externalIDFor(contact models.Contact, sent string) *string {
	if sent == "" || contact.ExternalID != nil {
		return nil
	}

	return &sent
}

/*
utc returns the time in UTC, nil for nil.
*/
func utc(at *time.Time) *time.Time {
	if at == nil || at.IsZero() {
		return nil
	}

	return new(at.UTC())
}

/*
nonNilMap returns the map, empty instead of nil.
*/
func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}

	return m
}

/*
Notes about a contact or a company keep their author and time.
*/
func (n *ContactNote) validate() error {
	if (n.ContactRef == "") == (n.CompanyRef == "") {
		return invalid("contact_ref", "A note is about one contact or one company.")
	}

	if err := required("body", n.Body); err != nil {
		return err
	}

	if n.CreatedAt.IsZero() {
		return invalid("created_at", "created_at is required.")
	}

	return checkAuthor("author", n.Author)
}

func (n *ContactNote) refs() []Ref {
	return append(append(refsOf(KindContact, n.ContactRef), refsOf(KindCompany, n.CompanyRef)...),
		authorRefs(&n.Author)...)
}

func (n *ContactNote) apply(ctx context.Context, s *step) (outcome, error) {
	var out outcome

	body := cut(strings.TrimSpace(n.Body), 10000)

	if s.prev != nil && s.prev.RecordID != nil {
		rows, err := s.q.UpdateMigratedNote(ctx, models.UpdateMigratedNoteParams{
			Body: body, Now: s.now, WorkspaceID: s.ws(), ID: *s.prev.RecordID,
		})
		if err != nil {
			return out, fmt.Errorf("rewrite note: %w", err)
		}

		if rows == 1 {
			out.id = *s.prev.RecordID

			return out, nil
		}
	}

	params := models.CreateMigratedNoteParams{WorkspaceID: s.ws(), Body: body, CreatedAt: n.CreatedAt.UTC(), Now: s.now}

	if err := n.subject(s, &params); err != nil {
		return out, err
	}

	if member, ok := s.member(&n.Author); ok {
		params.AuthorMemberID = &member
	} else if n.Author.Type == AuthorTeammate {
		out.warn("author", "teammate_unavailable", "The teammate is not a member here yet, so the note has no author.")
	}

	id, err := s.q.CreateMigratedNote(ctx, params)
	if err != nil {
		return out, fmt.Errorf("add note: %w", err)
	}

	out.id, out.created = id, true

	return out, nil
}

/*
subject sets the contact or company the note is about.
*/
func (n *ContactNote) subject(s *step, params *models.CreateMigratedNoteParams) error {
	if n.ContactRef != "" {
		id, err := s.refs.must("contact_ref", KindContact, n.ContactRef)
		params.ContactID = &id

		return err
	}

	id, err := s.refs.must("company_ref", KindCompany, n.CompanyRef)
	params.CompanyID = &id

	return err
}
