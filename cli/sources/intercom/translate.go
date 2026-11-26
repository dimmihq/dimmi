package intercom

import (
	"encoding/json"
	"html"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/dom"

	"github.com/dimmihq/dimmi/cli/format"
)

/*
text is Intercom's text with its HTML escapes undone: names and emails
arrive escaped unless asked otherwise.
*/
func text(s string) string { return strings.TrimSpace(html.UnescapeString(s)) }

func toMarkdown(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}

	md, err := dom.ToMarkdown(body)
	if err != nil {
		return strings.TrimSpace(dom.ToText(body))
	}

	return strings.TrimSpace(md)
}

func when(t Time) *time.Time { return format.Unix(int64(t)) }

func at(t Time) time.Time {
	if p := when(t); p != nil {
		return *p
	}

	return time.Time{}
}

func base(id ID, updated Time) format.Base {
	return format.Base{SourceID: string(id), SourceUpdatedAt: when(updated)}
}

/*
translateAdmin turns a teammate. Current marks those GET /admins lists.
*/
func translateAdmin(a admin, current bool) format.Record {
	t := &format.Teammate{
		Base: base(a.ID, 0), Email: text(a.Email), Name: text(a.Name), AvatarURL: a.Avatar.ImageURL, Current: &current,
	}

	if a.Role != nil {
		t.Role = a.Role.Name
	}

	return format.NewRecord(format.KindTeammate, t.SourceID, t)
}

func translateTeam(t team) format.Record {
	return format.NewRecord(format.KindTeam, string(t.ID), &format.Team{
		Base: base(t.ID, 0), Name: text(t.Name), MemberRefs: t.AdminIDs.strings(),
	})
}

func translateTag(t tag) format.Record {
	return format.NewRecord(format.KindTag, string(t.ID), &format.Tag{Base: base(t.ID, 0), Name: text(t.Name)})
}

/*
attributeType maps Intercom's data types onto dimmi's.
*/
func attributeType(dataType string, options int) (string, bool) {
	switch dataType {
	case "string":
		if options > 0 {
			return format.TypeList, true
		}

		return format.TypeText, true
	case "integer":
		return format.TypeNumber, true
	case "float", "decimal":
		return format.TypeDecimal, true
	case "boolean":
		return format.TypeBoolean, true
	case "date", "datetime":
		return format.TypeDate, true
	case "list":
		return format.TypeList, true
	case "files":
		return format.TypeFiles, true
	case "relationship":
		return format.TypeRelation, true
	}

	return "", false
}

/*
translateDataAttribute turns a custom contact or company attribute.
Intercom's own fields, such as email, aren't custom and are skipped.
*/
func translateDataAttribute(a dataAttribute) (format.Record, bool) {
	if !a.Custom {
		return format.Record{}, false
	}

	typ, ok := attributeType(a.DataType, len(a.Options))
	if !ok || typ == format.TypeRelation {
		return format.Record{}, false
	}

	object := format.ObjectContact
	if a.Model == "company" {
		object = format.ObjectCompany
	}

	id := object + "_" + string(a.ID)

	return format.NewRecord(format.KindAttributeDefinition, id, &format.AttributeDefinition{
		Base: format.Base{SourceID: id, SourceUpdatedAt: when(a.UpdatedAt)}, Object: object, Key: attrKey(a.Name),
		Label: text(firstOf(a.Label, a.Name)), Type: typ, Options: a.Options, Description: text(a.Description),
		Archived: a.Archived,
	}), true
}

func translateConversationAttribute(a conversationAttribute) (format.Record, bool) {
	typ, ok := attributeType(a.DataType, len(a.Options))
	if !ok {
		return format.Record{}, false
	}

	var options []string

	for _, o := range a.Options {
		if label := firstOf(o.Label, o.Name, o.Value); label != "" && !o.Archived {
			options = append(options, text(label))
		}
	}

	id := "conversation_" + string(a.ID)

	return format.NewRecord(format.KindAttributeDefinition, id, &format.AttributeDefinition{
		Base: format.Base{SourceID: id, SourceUpdatedAt: when(a.UpdatedAt)}, Object: format.ObjectConversation,
		Key: attrKey(a.Name), Label: text(a.Name), Type: typ, Options: options, Description: text(a.Description),
		Archived: a.Archived, Multiline: a.Multiline, Required: a.Required,
	}), true
}

/*
attrKey turns an Intercom attribute name, such as "Beta tester", into a
dimmi attribute key, beta_tester. Contact, company and conversation
values are keyed the same way, so they land in their attributes.
*/
func attrKey(name string) string {
	var b strings.Builder

	under := false

	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)

			under = false
		case !under && b.Len() > 0:
			b.WriteByte('_')

			under = true
		}
	}

	key := strings.TrimRight(b.String(), "_")
	if key == "" || key[0] >= '0' && key[0] <= '9' {
		key = "attr_" + key
	}

	if reservedKey(key) {
		key = "custom_" + key
	}

	if len(key) > 64 {
		key = strings.TrimRight(key[:64], "_")
	}

	return key
}

/*
reservedKey reports whether the key is one of dimmi's standard fields,
which a custom attribute can't take; an Intercom attribute named like
one gets a custom_ prefix.
*/
func reservedKey(key string) bool {
	switch key {
	case "email", "name", "phone", "external_id", "kind", "locale", "timezone", "country", "region", "city",
		"created_at", "first_seen_at", "last_seen_at", "tags", "company", "domain", "id",
		"claimed_user_id", "test_member_id":
		return true
	}

	return false
}

/*
attrValues keys a record's custom attributes the way their definitions
are keyed.
*/
func attrValues(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]any, len(in))
	for k, v := range in {
		out[attrKey(k)] = v
	}

	return out
}

func firstOf(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}

	return ""
}

/*
translateCompany turns a company. Intercom's own company fields that
dimmi has no column for are kept as custom attributes.
*/
func translateCompany(c company) format.Record {
	attrs := map[string]any{}
	for k, v := range c.CustomAttrs {
		attrs[attrKey(k)] = v
	}

	var plan struct {
		Name string `json:"name"`
	}

	if len(c.Plan) > 0 && json.Unmarshal(c.Plan, &plan) == nil && plan.Name != "" {
		attrs["plan"] = text(plan.Name)
	}

	setNumber(attrs, "size", c.Size)
	setNumber(attrs, "monthly_spend", c.MonthlySpend)
	setNumber(attrs, "session_count", c.SessionCount)
	setNumber(attrs, "user_count", c.UserCount)

	if c.Industry != "" {
		attrs["industry"] = text(c.Industry)
	}

	if t := when(c.LastRequestAt); t != nil {
		attrs["last_request_at"] = t.Format(time.RFC3339)
	}

	if len(attrs) == 0 {
		attrs = nil
	}

	created := when(c.RemoteCreatedAt)
	if created == nil {
		created = when(c.CreatedAt)
	}

	var tags []string
	for _, t := range c.Tags.Tags {
		tags = append(tags, string(t.ID))
	}

	return format.NewRecord(format.KindCompany, string(c.ID), &format.Company{
		Base: base(c.ID, c.UpdatedAt), ExternalID: c.CompanyID, Name: firstOf(text(c.Name), c.CompanyID),
		Domain: domainOf(c.Website), CustomAttributes: attrs, TagRefs: tags, CreatedAt: created,
	})
}

func setNumber(m map[string]any, key string, v *float64) {
	if v != nil && *v != 0 {
		m[key] = *v
	}
}

func domainOf(website string) string {
	w := strings.TrimSpace(strings.ToLower(website))
	w = strings.TrimPrefix(strings.TrimPrefix(w, "https://"), "http://")
	w = strings.TrimPrefix(w, "www.")

	host, _, _ := strings.Cut(w, "/")

	return host
}

/*
translateContact turns a user or a lead. Their companies and tags come
in full: the contact's own lists stop at ten, so the reader fetches the
rest first.
*/
func translateContact(c contact, tags, companies []string) format.Record {
	kind := "user"
	if c.Role == "lead" {
		kind = "lead"
	}

	phone := c.Phone
	if phone == "" {
		phone = c.WhatsappNumber
	}

	var loc *format.Location
	if c.Location.City != "" || c.Location.Region != "" || c.Location.Country != "" {
		loc = &format.Location{City: c.Location.City, Region: c.Location.Region, Country: c.Location.Country}
	}

	first := when(c.SignedUpAt)
	if first == nil {
		first = when(c.CreatedAt)
	}

	var links []format.CompanyLink
	for i, id := range companies {
		links = append(links, format.CompanyLink{Ref: id, Default: i == 0})
	}

	attrs := attrValues(c.CustomAttrs)

	return format.NewRecord(format.KindContact, string(c.ID), &format.Contact{
		Base: base(c.ID, c.UpdatedAt), Kind: kind, ExternalID: c.ExternalID, Email: text(c.Email), Name: text(c.Name),
		Phone: phone, AvatarURL: avatarURL(c.Avatar), Locale: firstOf(c.LanguageOverride, c.BrowserLanguage),
		Location:    loc,
		FirstSeenAt: first, LastSeenAt: when(c.LastSeenAt), CreatedAt: when(c.CreatedAt), CustomAttributes: attrs,
		TagRefs: tags, Companies: links, EmailUnsubscribed: c.Unsubscribed,
	})
}

/*
avatarURL reads a contact's avatar, a URL in 2.18 and an object in
older versions.
*/
func avatarURL(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}

	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}

	var obj struct {
		ImageURL string `json:"image_url"`
	}

	_ = json.Unmarshal(raw, &obj)

	return obj.ImageURL
}

func translateNote(n note, contactID, companyID string) format.Record {
	a := format.Author{Type: format.AuthorSystem}
	if n.Author.ID != "" {
		a = format.Author{Type: format.AuthorTeammate, Ref: string(n.Author.ID)}
	}

	return format.NewRecord(format.KindContactNote, string(n.ID), &format.ContactNote{
		Base: base(n.ID, 0), ContactRef: contactID, CompanyRef: companyID, Author: a, Body: toMarkdown(n.Body),
		CreatedAt: at(n.CreatedAt),
	})
}

/*
ticketKind maps a ticket type's category.
*/
func ticketKind(category string) string {
	switch strings.ToLower(category) {
	case "back-office", "back_office":
		return format.TicketBackOffice
	case "tracker":
		return format.TicketTracker
	}

	return format.TicketCustomer
}

/*
The default ticket attributes Intercom names for the ticket's own title
and description.
*/
const (
	defaultTitle       = "_default_title_"
	defaultDescription = "_default_description_"
)

func translateTicketType(t ticketType) format.Record {
	tt := &format.TicketType{
		Base: base(t.ID, t.UpdatedAt), Kind: ticketKind(t.Category), Name: text(t.Name),
		Description: text(t.Description),
	}

	for _, a := range t.Attributes.Data {
		if a.Default || a.Name == defaultTitle || a.Name == defaultDescription || a.Archived {
			continue
		}

		var options []string
		for _, o := range a.Input.Options {
			if !o.Archived {
				options = append(options, text(o.Label))
			}
		}

		typ, ok := attributeType(a.DataType, len(options))
		if !ok {
			typ = format.TypeText
		}

		tt.Fields = append(tt.Fields, format.TicketField{
			SourceID: string(a.ID), Key: attrKey(a.Name), Label: text(a.Name), Type: typ, Options: options,
			CustomerVisible: a.VisibleToContacts, Required: a.RequiredToCreate, Multiline: a.Input.Multiline,
		})
	}

	states := slices.Clone(t.States.Data)
	order := map[string]int{"submitted": 0, "in_progress": 1, "waiting_on_customer": 2, "resolved": 3}
	sort.SliceStable(states, func(i, j int) bool { return order[states[i].Category] < order[states[j].Category] })

	for _, s := range states {
		tt.States = append(tt.States, format.TicketState{
			SourceID: string(s.ID), Group: stateGroup(s.Category),
			Name:          text(firstOf(s.InternalLabel, s.ExternalLabel, s.Category)),
			CustomerLabel: text(s.ExternalLabel),
		})
	}

	return format.NewRecord(format.KindTicketType, tt.SourceID, tt)
}

/*
stateGroup maps Intercom's ticket state categories onto dimmi's groups.
*/
func stateGroup(category string) string {
	if category == "waiting_on_customer" {
		return "waiting_customer"
	}

	return category
}
