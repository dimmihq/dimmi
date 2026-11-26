package intercom

import (
	"encoding/json"
	"fmt"
	"html"
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
	"github.com/dimmihq/dimmi/framework/dom"
)

/*
markdown turns Intercom's HTML into Markdown. Article bodies go through
the article converter, which keeps Intercom's rich blocks.
*/
var markdown = dom.ToMarkdown

/*
articleMarkdown converts an article body.
*/
var articleMarkdown = dom.ToMarkdown

/*
text is Intercom's text with its HTML escapes undone: names and emails
arrive escaped unless asked otherwise.
*/
func text(s string) string { return strings.TrimSpace(html.UnescapeString(s)) }

func toMarkdown(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}

	md, err := markdown(body)
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
		Label: text(firstOf(a.Label, a.Name)), Type: typ, Options: a.Options, Description: text(a.Description), Archived: a.Archived,
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

	if slices.Contains(reservedKeys, key) {
		key = "custom_" + key
	}

	if len(key) > 64 {
		key = strings.TrimRight(key[:64], "_")
	}

	return key
}

/*
reservedKeys are dimmi's standard fields, which a custom attribute can't
take; an Intercom attribute named like one gets a custom_ prefix.
*/
var reservedKeys = []string{
	"email", "name", "phone", "external_id", "kind", "locale", "timezone", "country", "region", "city",
	"created_at", "first_seen_at", "last_seen_at", "tags", "company", "domain", "id",
	"claimed_user_id", "test_member_id",
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
		Phone: phone, AvatarURL: avatarURL(c.Avatar), Locale: firstOf(c.LanguageOverride, c.BrowserLanguage), Location: loc,
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

func translateNote(n note, contactID string) format.Record {
	a := format.Author{Type: format.AuthorSystem}
	if n.Author.ID != "" {
		a = format.Author{Type: format.AuthorTeammate, Ref: string(n.Author.ID)}
	}

	return format.NewRecord(format.KindContactNote, string(n.ID), &format.ContactNote{
		Base: base(n.ID, 0), ContactRef: contactID, Author: a, Body: toMarkdown(n.Body), CreatedAt: at(n.CreatedAt),
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
		Base: base(t.ID, t.UpdatedAt), Kind: ticketKind(t.Category), Name: text(t.Name), Description: text(t.Description),
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
			SourceID: string(s.ID), Group: stateGroup(s.Category), Name: text(firstOf(s.InternalLabel, s.ExternalLabel, s.Category)),
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

func translateHelpCenter(h helpCenter, defaultLocale string) format.Record {
	locales := slices.Clone(h.Locales)
	if defaultLocale == "" && len(locales) > 0 {
		defaultLocale = locales[0]
	}

	return format.NewRecord(format.KindHelpCenter, string(h.ID), &format.HelpCenter{
		Base: base(h.ID, h.UpdatedAt), Name: text(firstOf(h.DisplayName, h.Identifier)), DefaultLocale: defaultLocale,
		Locales: locales, URL: h.URL,
	})
}

/*
translatedGroup reads a collection's translated_content: a map from
locale to name and description, with a "type" key among them.
*/
func translatedGroup(raw map[string]json.RawMessage) map[string]groupContent {
	out := map[string]groupContent{}

	for locale, v := range raw {
		if locale == "type" || string(v) == "null" {
			continue
		}

		var g groupContent
		if json.Unmarshal(v, &g) == nil && (g.Name != "" || g.Description != "") {
			out[locale] = g
		}
	}

	return out
}

func translateCollection(c collection, helpCenter string) format.Record {
	col := &format.Collection{
		Base: base(c.ID, c.UpdatedAt), HelpCenterRef: helpCenter, ParentRef: string(c.ParentID),
		Name: text(c.Name), Description: text(c.Description), Icon: c.Icon, Order: c.Order,
	}

	tr := translatedGroup(c.Translated)
	for _, locale := range sortedKeys(tr) {
		if locale == c.DefaultLocale {
			continue
		}

		g := tr[locale]
		col.Translations = append(col.Translations, format.Translation{
			Locale: locale, Name: text(g.Name), Description: text(g.Description),
		})
	}

	return format.NewRecord(format.KindCollection, col.SourceID, col)
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

/*
articleState maps Intercom's published and draft.
*/
func articleState(s string) string {
	if s == "published" {
		return "published"
	}

	return "draft"
}

func teammateAuthor(id ID) *format.Author {
	if id == "" || id == "0" {
		return nil
	}

	return &format.Author{Type: format.AuthorTeammate, Ref: string(id)}
}

/*
articleBody converts an article's HTML body, which is faithful to what
Intercom shows; body_markdown drops blocks such as buttons and videos.
*/
func articleBody(body, bodyMarkdown string) string {
	if strings.TrimSpace(body) == "" {
		return strings.TrimSpace(bodyMarkdown)
	}

	md, err := articleMarkdown(body)
	if err != nil {
		if bodyMarkdown != "" {
			return strings.TrimSpace(bodyMarkdown)
		}

		return strings.TrimSpace(dom.ToText(body))
	}

	return strings.TrimSpace(md)
}

/*
blockPatterns find the rich blocks an Intercom article body uses.
*/
var blockPatterns = map[string]*regexp.Regexp{
	"button":          regexp.MustCompile(`intercom-h2b-button`),
	"video":           regexp.MustCompile(`<iframe`),
	"callout":         regexp.MustCompile(`intercom-interblocks-callout`),
	"collapsible":     regexp.MustCompile(`<details|intercom-interblocks-collapsible`),
	"table":           regexp.MustCompile(`<table`),
	"code":            regexp.MustCompile(`<pre`),
	"image":           regexp.MustCompile(`<img`),
	"centered":        regexp.MustCompile(`intercom-align-center`),
	"horizontal_rule": regexp.MustCompile(`<hr`),
}

func blocksOf(bodies ...string) []string {
	var out []string

	for _, name := range sortedKeys(blockPatterns) {
		for _, b := range bodies {
			if blockPatterns[name].MatchString(b) {
				out = append(out, name)

				break
			}
		}
	}

	return out
}

/*
audience maps help_center_audience; anything narrower than everyone is
kept by name for dimmi's audiences.
*/
func audience(a string) string {
	switch a {
	case "", "everyone":
		return ""
	}

	return a
}

/*
redirect is the path of an article's old address, which dimmi's help
center redirects from.
*/
func redirect(u string) string {
	if u == "" {
		return ""
	}

	if i := strings.Index(u, "://"); i >= 0 {
		rest := u[i+3:]
		if j := strings.Index(rest, "/"); j >= 0 {
			return rest[j:]
		}

		return ""
	}

	return u
}

/*
images collects the pictures article bodies show. Each is uploaded as a
file and the body points at it as dimmi-file:<id>, which the engine
turns into the uploaded file's id and dimmi copies into the help
center, so nothing links back to Intercom's CDN.
*/
type images struct {
	files []*format.File
	seen  map[string]bool
}

var imgSrc = regexp.MustCompile(`(<img\b[^>]*?\ssrc=")([^"]+)(")`)

/*
rewrite points every picture of the HTML at its upload.
*/
func (im *images) rewrite(body string) string {
	return imgSrc.ReplaceAllStringFunc(body, func(m string) string {
		parts := imgSrc.FindStringSubmatch(m)
		src := html.UnescapeString(parts[2])

		if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
			return m
		}

		name := path.Base(strings.SplitN(src, "?", 2)[0])
		id := format.FileID(src, name, 0)

		if im.seen == nil {
			im.seen = map[string]bool{}
		}

		if !im.seen[id] {
			im.seen[id] = true
			im.files = append(im.files, &format.File{SourceID: id, Name: name, Source: src})
		}

		return parts[1] + "dimmi-file:" + id + parts[3]
	})
}

func translateArticle(a article, helpCenter, defaultLocale string) format.Record {
	im := &images{}
	a.Body = im.rewrite(a.Body)

	locale := firstOf(a.DefaultLocale, defaultLocale, "en")

	art := &format.Article{
		Base: base(a.ID, a.UpdatedAt), HelpCenterRef: helpCenter, Title: text(a.Title), Description: text(a.Description),
		Body: articleBody(a.Body, a.BodyMarkdown), State: articleState(a.State), Visibility: format.VisibilityPublic,
		Audience: audience(a.Audience), Author: teammateAuthor(a.AuthorID), CreatedAt: when(a.CreatedAt),
		UpdatedAt: when(a.UpdatedAt),
	}

	if a.ParentType == "collection" || a.ParentType == "" {
		if a.ParentID != "" {
			art.CollectionRef = string(a.ParentID)
		} else if len(a.ParentIDs) > 0 {
			art.CollectionRef = string(a.ParentIDs[0])
		}
	}

	if r := redirect(a.URL); r != "" {
		art.Redirects = append(art.Redirects, r)
	}

	bodies := []string{a.Body}

	for _, l := range sortedKeys(a.Translated) {
		if l == "type" || l == locale || string(a.Translated[l]) == "null" {
			continue
		}

		var c articleContent
		if json.Unmarshal(a.Translated[l], &c) != nil || (c.Title == "" && c.Body == "") {
			continue
		}

		bodies = append(bodies, c.Body)
		c.Body = im.rewrite(c.Body)
		tr := format.Translation{
			Locale: l, Title: text(c.Title), Description: text(c.Description), Body: articleBody(c.Body, c.BodyMarkdown),
			State: articleState(c.State), Author: teammateAuthor(c.AuthorID), CreatedAt: when(c.CreatedAt), UpdatedAt: when(c.UpdatedAt),
		}

		if r := redirect(c.URL); r != "" {
			tr.Redirects = []string{r}
		}

		art.Translations = append(art.Translations, tr)
	}

	art.Blocks = blocksOf(bodies...)

	for _, t := range a.Tags.Tags {
		art.TagRefs = append(art.TagRefs, string(t.ID))
	}

	return format.NewRecord(format.KindArticle, art.SourceID, art, im.files...)
}

/*
internalID keeps internal articles apart from public ones, whose ids
Intercom draws from another sequence.
*/
func internalID(id ID) string { return "internal_" + string(id) }

func translateInternalArticle(a internalArticle, helpCenter string) format.Record {
	id := internalID(a.ID)
	im := &images{}
	a.Body = im.rewrite(a.Body)

	return format.NewRecord(format.KindArticle, id, &format.Article{
		Base: format.Base{SourceID: id, SourceUpdatedAt: when(a.UpdatedAt)}, HelpCenterRef: helpCenter, Title: text(a.Title),
		Body: articleBody(a.Body, ""), State: "published", Visibility: format.VisibilityAgentOnly,
		Author: teammateAuthor(firstID(a.AuthorID, a.OwnerID)), CreatedAt: when(a.CreatedAt), UpdatedAt: when(a.UpdatedAt),
		Blocks: blocksOf(a.Body),
	}, im.files...)
}

func firstID(ids ...ID) ID {
	for _, id := range ids {
		if id != "" && id != "0" {
			return id
		}
	}

	return ""
}

func translateSnippet(s snippet, helpCenter string) format.Record {
	return format.NewRecord(format.KindSnippet, string(s.ID), &format.Snippet{
		Base: base(s.ID, s.UpdatedAt), HelpCenterRef: helpCenter, Title: text(s.Title), Body: strings.TrimSpace(s.BodyMarkdown),
		Locale: s.Locale, CreatedAt: when(s.CreatedAt),
	})
}

/*
placeholder finds an Intercom placeholder: {{ name }} or
{{ name | fallback: "x" }}.
*/
var placeholder = regexp.MustCompile(`\{\{\s*([a-zA-Z0-9_.]+)\s*(?:\|\s*fallback\s*:\s*("[^"]*"|'[^']*'))?\s*\}\}`)

/*
liquidNames maps Intercom's placeholder names onto dimmi's Liquid.
*/
var liquidNames = map[string]string{
	"first_name": "contact.first_name", "user.first_name": "contact.first_name", "contact.first_name": "contact.first_name",
	"name": "contact.name", "user.name": "contact.name", "full_name": "contact.name", "user.full_name": "contact.name",
	"contact.name": "contact.name",
	"email":        "contact.email", "user.email": "contact.email", "contact.email": "contact.email",
	"company.name": "company.name", "user.company.name": "company.name", "company_name": "company.name",
	"admin.first_name": "sender.first_name", "teammate.first_name": "sender.first_name",
	"current_admin.first_name": "sender.first_name", "conversation.admin_assignee.first_name": "sender.first_name",
	"admin.name": "sender.name", "teammate.name": "sender.name", "current_admin.name": "sender.name",
	"conversation.admin_assignee.name": "sender.name",
}

/*
rewritePlaceholders rewrites Intercom placeholders into dimmi's Liquid,
fallbacks becoming defaults. Names dimmi has no value for stay as they
were and are listed.
*/
func rewritePlaceholders(body string) (string, []string) {
	var unknown []string

	out := placeholder.ReplaceAllStringFunc(body, func(m string) string {
		parts := placeholder.FindStringSubmatch(m)
		name, fallback := parts[1], parts[2]

		liquid, ok := liquidNames[strings.ToLower(name)]
		if !ok {
			if !slices.Contains(unknown, name) {
				unknown = append(unknown, name)
			}

			return m
		}

		if fallback != "" {
			return fmt.Sprintf("{{ %s | default: %s }}", liquid, fallback)
		}

		return "{{ " + liquid + " }}"
	})

	return out, unknown
}

/*
macroBody converts a macro's HTML and rewrites its placeholders. They
are set aside while the HTML becomes Markdown, which would otherwise
escape their underscores.
*/
func macroBody(body string) (string, []string) {
	var kept []string

	held := placeholder.ReplaceAllStringFunc(body, func(m string) string {
		kept = append(kept, m)

		return fmt.Sprintf("DIMMIPLACEHOLDER%dX", len(kept)-1)
	})

	md := toMarkdown(held)

	for i, m := range kept {
		md = strings.Replace(md, fmt.Sprintf("DIMMIPLACEHOLDER%dX", i), html.UnescapeString(m), 1)
	}

	return rewritePlaceholders(md)
}

func translateMacro(m macro) format.Record {
	body, unknown := macroBody(firstOf(m.Body, m.BodyText))

	shared := m.VisibleTo != "specific_teams"
	r := &format.SavedReply{
		Base:  format.Base{SourceID: string(m.ID), SourceUpdatedAt: parseTime(m.UpdatedAt)},
		Title: text(m.Name), Body: body, Shared: &shared, Unknown: unknown,
	}

	if !shared {
		r.TeamRefs = slices.Clone(m.TeamIDs)
	}

	return format.NewRecord(format.KindSavedReply, r.SourceID, r)
}

func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return format.At(t)
		}
	}

	return nil
}

var weekdays = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

func clock(minute int) string {
	if minute >= 1440 {
		return "24:00"
	}

	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

/*
weekly splits intervals measured from Monday 00:00 into stretches of
each day; one that runs past midnight is split at it.
*/
func weekly(intervals []interval) []format.Hours {
	var out []format.Hours

	for _, iv := range intervals {
		start, end := iv.StartMinute, iv.EndMinute
		if end == 10079 {
			end = 10080
		}

		for start < end {
			day := start / 1440
			if day > 6 {
				break
			}

			dayEnd := min(end, (day+1)*1440)
			out = append(out, format.Hours{Day: weekdays[day], Start: clock(start - day*1440), End: clock(dayEnd - day*1440)})
			start = dayEnd
		}
	}

	return out
}

func translateSchedule(s schedule, exceptions []scheduleException, first bool) format.Record {
	sch := &format.Schedule{
		Base: base(s.ID, s.UpdatedAt), Name: text(s.Name), Timezone: s.TimeZone, Weekly: weekly(s.Intervals), Default: first,
	}

	if s.AlwaysOpen && len(sch.Weekly) == 0 {
		sch.Weekly = weekly([]interval{{StartMinute: 0, EndMinute: 10080}})
	}

	for _, e := range exceptions {
		if e.Type == "closed" {
			sch.Holidays = append(sch.Holidays, format.Holiday{Date: e.Date, Name: text(e.Name), Yearly: e.RecurringAnnually})

			continue
		}

		for _, iv := range e.Intervals {
			sch.Exceptions = append(sch.Exceptions, format.Exception{
				Date: e.Date, Name: text(e.Name), Start: clock(iv.StartMinute), End: clock(iv.EndMinute), Yearly: e.RecurringAnnually,
			})
		}
	}

	return format.NewRecord(format.KindSchedule, sch.SourceID, sch)
}
