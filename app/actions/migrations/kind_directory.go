/*
|--------------------------------------------------------------------------
| People, Teams, Tags And Fields
|--------------------------------------------------------------------------
|
| The records the rest name: teammates, teams, tags, custom field
| definitions, companies, contacts and the notes about them. Each is
| matched to what the workspace already has before anything is made, a
| teammate by email, a team by name, a tag by name, a field by key, a
| company by external id or domain, a contact by external id or email,
| so a workspace set up by hand, or a second migration, reuses them.
|
*/

package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/attributes"
	"github.com/dimmihq/dimmi/app/actions/companies"
	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/actions/tags"
	"github.com/dimmihq/dimmi/app/actions/teams"
	"github.com/dimmihq/dimmi/app/attributedefs"
	"github.com/dimmihq/dimmi/app/models"
	supportattributes "github.com/dimmihq/dimmi/app/support/attributes"
)

/*
kinds returns how each kind of record is read.
*/
func kinds() map[string]kindSpec {
	return map[string]kindSpec{
		KindTeammate:            decoder[Teammate](),
		KindTeam:                decoder[Team](),
		KindTag:                 decoder[Tag](),
		KindAttributeDefinition: decoder[AttributeDefinition](),
		KindCompany:             decoder[Company](),
		KindContact:             decoder[Contact](),
		KindContactNote:         decoder[ContactNote](),
		KindTicketType:          decoder[TicketType](),
		KindTicket:              decoder[Ticket](),
		KindHelpCenter:          decoder[HelpCenter](),
		KindCollection:          decoder[Collection](),
		KindArticle:             decoder[Article](),
		KindSnippet:             decoder[Snippet](),
		KindSavedReply:          decoder[SavedReply](),
		KindSchedule:            decoder[Schedule](),
		KindConversation:        decoder[Conversation](),
	}
}

/*
base returns the record's base.
*/
func (b *Base) base() Base { return *b }

/*
Teammates are matched by email to a member of the workspace, active or
not. One with no member here waits for imported teammates; until then
it is kept unsupported with its name, so its history is still shown as
written by them.
*/
func (t *Teammate) validate() error {
	t.Email = strings.TrimSpace(t.Email)
	if contacts.NormalizeEmail(&t.Email) == nil {
		return invalid("email", "A teammate has an email address.")
	}

	return oneOf("role", t.Role, "owner", "admin", "member")
}

func (t *Teammate) refs() []Ref { return nil }

func (t *Teammate) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Name}
	if out.label == "" {
		out.label = t.Email
	}

	m, err := s.q.FindWorkspaceMemberByEmail(ctx, models.FindWorkspaceMemberByEmailParams{
		WorkspaceID: s.ws(), Email: *contacts.NormalizeEmail(&t.Email),
	})
	if errors.Is(err, database.ErrNotFound) {
		return out, ErrNotSupportedYet
	}

	if err != nil {
		return out, fmt.Errorf("find teammate: %w", err)
	}

	out.id = m.ID

	return out, nil
}

/*
Teams are matched by name. Members are added when they are active
teammates here.
*/
func (t *Team) validate() error { return required("name", t.Name) }

func (t *Team) refs() []Ref { return refsOf(KindTeammate, t.MemberRefs...) }

func (t *Team) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Name}
	name := cut(strings.TrimSpace(t.Name), 80)

	team, err := s.q.FindLiveTeamByName(ctx, models.FindLiveTeamByNameParams{WorkspaceID: s.ws(), Name: name})

	switch {
	case err == nil:
		out.id = team.ID
	case errors.Is(err, database.ErrNotFound):
		made, err := s.kit().CreateTeam.Handle(ctx, teams.CreateTeamInput{Membership: s.runner.m, Name: name})
		if err != nil {
			return out, err
		}

		out.id, out.created = made.Team.ID, true
	default:
		return out, fmt.Errorf("find team: %w", err)
	}

	members := s.refs.ids(KindTeammate, t.MemberRefs)
	if len(members) < len(t.MemberRefs) {
		out.warn("member_refs", "teammate_unavailable", "Teammates who are not members here yet were left out.")
	}

	if len(members) == 0 {
		return out, nil
	}

	results, err := s.kit().AddTeamMembers.Handle(ctx, teams.AddTeamMembersInput{
		Membership: s.runner.m, TeamID: out.id, MemberIDs: members,
	})
	if err != nil {
		return out, err
	}

	for _, r := range results {
		if r == teams.ResultInactive {
			out.warn("member_refs", "teammate_inactive",
				"Teammates who are not active here were left out; add them once they are enabled.")

			break
		}
	}

	return out, nil
}

/*
Tags are matched by name, without regard to case.
*/
func (t *Tag) validate() error { return required("name", t.Name) }

func (t *Tag) refs() []Ref { return nil }

func (t *Tag) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: t.Name}
	name := strings.TrimSpace(t.Name)

	before, err := tagIDs(ctx, s, name, false)
	if err != nil {
		return out, err
	}

	if len(before) == 1 {
		out.id = before[0]

		return out, nil
	}

	made, err := tagIDs(ctx, s, name, true)
	if err != nil {
		return out, err
	}

	if len(made) != 1 {
		return out, invalid("name", "This tag could not be made.")
	}

	out.id, out.created = made[0], true

	return out, nil
}

/*
tagIDs finds the tag with the name, making it when asked.
*/
func tagIDs(ctx context.Context, s *step, name string, create bool) ([]ids.UUID, error) {
	if !create {
		row, err := s.q.FindTagByName(ctx, models.FindTagByNameParams{WorkspaceID: s.ws(), Name: name})
		if errors.Is(err, database.ErrNotFound) {
			return nil, nil
		}

		if err != nil {
			return nil, fmt.Errorf("find tag: %w", err)
		}

		return []ids.UUID{row.ID}, nil
	}

	return tags.Resolve(ctx, s.q, tags.Scope{WorkspaceID: s.ws(), Now: s.now}, tags.Refs{Names: []string{name}})
}

/*
Field definitions are matched by key. Conversation fields wait for
conversation attributes.
*/
func (a *AttributeDefinition) validate() error {
	if err := required("object", a.Object); err != nil {
		return err
	}

	if err := oneOf("object", a.Object, "contact", "company", "conversation"); err != nil {
		return err
	}

	if err := required("key", a.Key); err != nil {
		return err
	}

	return oneOf("type", a.Type, supportattributes.Types()...)
}

func (a *AttributeDefinition) refs() []Ref { return nil }

func (a *AttributeDefinition) apply(ctx context.Context, s *step) (outcome, error) {
	out := outcome{label: a.Label}
	if a.Object == "conversation" {
		return out, ErrNotSupportedYet
	}

	label := a.Label
	if strings.TrimSpace(label) == "" {
		label = a.Key
	}

	made, err := s.kit().Attributes.Create(ctx, attributes.CreateInput{
		Membership: s.runner.m, Object: a.Object, Key: a.Key, Label: cut(label, 80),
		Description: orNil(a.Description), DataType: a.Type, Options: a.Options,
	})
	if err == nil {
		out.id, out.created = made.ID, true

		return out, nil
	}

	if !errors.Is(err, attributes.ErrExists) {
		return out, err
	}

	defs, err := s.q.ListAttributeDefinitions(ctx, models.ListAttributeDefinitionsParams{
		WorkspaceID: s.ws(), Object: &a.Object,
	})
	if err != nil {
		return out, fmt.Errorf("list attribute definitions: %w", err)
	}

	for _, d := range defs {
		if d.Key == a.Key {
			out.id = d.ID

			return out, nil
		}
	}

	return out, attributes.ErrExists
}

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
	if s.prev != nil && s.prev.RecordID != nil {
		found, err := s.q.FindContact(ctx, models.FindContactParams{WorkspaceID: s.ws(), ID: *s.prev.RecordID})
		if err == nil && found.DeletedAt == nil {
			return found, false, nil
		}
	}

	if c.ExternalID != "" {
		found, err := s.q.FindContactByExternalID(ctx, models.FindContactByExternalIDParams{
			WorkspaceID: s.ws(), ExternalID: &c.ExternalID,
		})
		if err == nil && found.DeletedAt == nil {
			return found, false, nil
		}
	}

	if email := contacts.NormalizeEmail(&c.Email); email != nil {
		found, err := s.q.FindImportContactByEmail(ctx, models.FindImportContactByEmailParams{
			WorkspaceID: s.ws(), Email: *email,
		})
		if err == nil {
			return found, false, nil
		}
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

	var err error
	if n.ContactRef != "" {
		id, err := s.refs.must("contact_ref", KindContact, n.ContactRef)
		if err != nil {
			return out, err
		}

		params.ContactID = &id
	} else {
		id, err := s.refs.must("company_ref", KindCompany, n.CompanyRef)
		if err != nil {
			return out, err
		}

		params.CompanyID = &id
	}

	if member, ok := s.member(&n.Author); ok {
		params.AuthorMemberID = &member
	} else if n.Author.Type == AuthorTeammate {
		out.warn("author", "teammate_unavailable", "The teammate is not a member here yet, so the note has no author.")
	}

	out.id, err = s.q.CreateMigratedNote(ctx, params)
	if err != nil {
		return out, fmt.Errorf("add note: %w", err)
	}

	out.created = true

	return out, nil
}
