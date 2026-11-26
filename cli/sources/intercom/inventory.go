package intercom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

/*
Report is what an inventory found: shapes and counts that decide how
the translator reads a workspace, never the content itself.
*/
type Report struct {
	PartTypes      map[string]int    `json:"part_types"`
	SourceTypes    map[string]int    `json:"source_types"`
	AuthorTypes    map[string]int    `json:"author_types"`
	ArticleTags    map[string]int    `json:"article_html_tags"`
	ArticleClasses map[string]int    `json:"article_intercom_classes"`
	MarkdownLosses map[string]int    `json:"body_markdown_drops"`
	Attachments    AttachmentReport  `json:"attachments"`
	Admins         int               `json:"admins"`
	FormerAuthors  int               `json:"authors_missing_from_admins"`
	AnonymousID    *AnonymousReport  `json:"anonymous_id,omitempty"`
	Shapes         []string          `json:"shapes_written,omitempty"`
	Errors         map[string]string `json:"errors,omitempty"`
}

/*
AttachmentReport says how attachments are served.
*/
type AttachmentReport struct {
	Count            int            `json:"count"`
	LargestBytes     int64          `json:"largest_bytes"`
	Hosts            map[string]int `json:"hosts"`
	SignedLinks      int            `json:"signed_links"`
	ExpiryParams     []string       `json:"expiry_params,omitempty"`
	DownloadedNoAuth int            `json:"downloaded_without_token"`
	RefusedNoAuth    int            `json:"refused_without_token"`
	ExpiresInSeconds []int64        `json:"expires_in_seconds,omitempty"`
}

/*
AnonymousReport answers whether a visitor's anonymous id, the
intercom-id cookie, can be read through the API: the field paths whose
value equals it.
*/
type AnonymousReport struct {
	ContactFields []string `json:"contact_fields"`
	VisitorLookup string   `json:"visitors_lookup"`
	VisitorFields []string `json:"visitor_fields,omitempty"`
	EventsLookup  string   `json:"events_lookup"`
	EventFields   []string `json:"event_fields,omitempty"`
}

/*
keep reports whether the key's values are kinds, not content, and stay
in the scrubbed shapes.
*/
func keep(key string) bool {
	switch key {
	case "type", "part_type", "data_type", "model", "state", "role",
		"category", "delivered_as", "exception_type", "visible_to", "parent_type",
		"help_center_audience", "priority", "content_type", "trigger_type", "channel":
		return true
	}

	return false
}

/*
scrub replaces every value with its type, keeping kinds and the shape.
*/
func scrub(v any, key string) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, child := range x {
			out[k] = scrub(child, k)
		}

		return out
	case []any:
		out := make([]any, 0, min(len(x), 3))
		for i, child := range x {
			if i == 3 {
				break
			}

			out = append(out, scrub(child, key))
		}

		return out
	case string:
		if keep(key) {
			return x
		}

		if strings.HasPrefix(x, "<") {
			return "<html>"
		}

		return "<string>"
	case float64:
		return 0
	case bool:
		return x
	}

	return v
}

/*
flatten lists every string value of a JSON document by its path.
*/
func flatten(prefix string, v any, out map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			flatten(prefix+"."+k, child, out)
		}
	case []any:
		for i, child := range x {
			flatten(prefix+"["+strconv.Itoa(i)+"]", child, out)
		}
	case string:
		out[strings.TrimPrefix(prefix, ".")] = x
	case float64:
		out[strings.TrimPrefix(prefix, ".")] = strconv.FormatFloat(x, 'f', -1, 64)
	}
}

func matches(raw json.RawMessage, want string) []string {
	var v any
	if json.Unmarshal(raw, &v) != nil {
		return nil
	}

	all := map[string]string{}
	flatten("", v, all)

	var out []string

	for path, val := range all {
		if val == want || strings.Contains(val, want) {
			out = append(out, path)
		}
	}

	sort.Strings(out)

	return out
}

var (
	htmlTag  = regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9]*)\b`)
	icClass  = regexp.MustCompile(`intercom-[a-z0-9-]+`)
	expiring = regexp.MustCompile(`(?i)(expires|x-amz-expires|signature|x-amz-signature|token)=([^&]*)`)
)

/*
inventory is an inventory in progress: the report, and where samples
are written.
*/
type inventory struct {
	s   *Source
	dir string
	r   *Report
}

/*
sample writes one scrubbed sample of an answer.
*/
func (iv *inventory) sample(name string, raw json.RawMessage) {
	if iv.dir == "" || len(raw) == 0 {
		return
	}

	var v any
	if json.Unmarshal(raw, &v) != nil {
		return
	}

	out, _ := json.MarshalIndent(scrub(v, ""), "", "  ")
	path := filepath.Join(iv.dir, name+".json")

	if err := os.WriteFile(path, append(out, '\n'), 0o600); err == nil {
		iv.r.Shapes = append(iv.r.Shapes, path)
	}
}

/*
get reads one endpoint and samples it, noting an error by name.
*/
func (iv *inventory) get(ctx context.Context, name, path string, q url.Values) json.RawMessage {
	var raw json.RawMessage
	if err := iv.s.Client.get(ctx, path, q, &raw); err != nil {
		iv.r.Errors[name] = err.Error()

		return nil
	}

	iv.sample(name, raw)

	return raw
}

/*
Inventory reads the workspace and reports its shapes. When dir is set,
it writes one scrubbed sample per endpoint there. VisitorID is an
anonymous visitor's intercom-id cookie to look for.
*/
func (s *Source) Inventory(ctx context.Context, dir, visitorID string) (Report, error) {
	r := Report{
		PartTypes: map[string]int{}, SourceTypes: map[string]int{}, AuthorTypes: map[string]int{},
		ArticleTags: map[string]int{}, ArticleClasses: map[string]int{}, MarkdownLosses: map[string]int{},
		Attachments: AttachmentReport{Hosts: map[string]int{}}, Errors: map[string]string{},
	}

	iv := &inventory{s: s, dir: dir, r: &r}

	if dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return r, err
		}
	}

	known, err := s.knownAdmins(ctx)
	if err != nil {
		return r, err
	}

	r.Admins = len(known)

	for _, e := range inventoryEndpoints() {
		iv.get(ctx, e.name, e.path, e.query)
	}

	contactsRaw := iv.get(ctx, "contacts", "/contacts", url.Values{"per_page": {"150"}})
	s.inventoryArticles(ctx, &r, iv.get(ctx, "articles", "/articles", url.Values{"per_page": {"250"}}))

	r.FormerAuthors = s.inventoryConversations(ctx, iv, known)
	s.inventoryTickets(ctx, iv)

	if visitorID != "" {
		r.AnonymousID = s.inventoryAnonymous(ctx, contactsRaw, visitorID)
	}

	if len(r.Errors) == 0 {
		r.Errors = nil
	}

	return r, nil
}

/*
endpoint is a list an inventory samples.
*/
type endpoint struct {
	name  string
	path  string
	query url.Values
}

func inventoryEndpoints() []endpoint {
	return []endpoint{
		{"admins", "/admins", nil},
		{"teams", "/teams", nil},
		{"tags", "/tags", nil},
		{"data_attributes_contact", "/data_attributes", url.Values{"model": {"contact"}}},
		{"data_attributes_company", "/data_attributes", url.Values{"model": {"company"}}},
		{"conversation_attributes", "/conversations/attributes", nil},
		{"companies_scroll", "/companies/scroll", nil},
		{"ticket_types", "/ticket_types", nil},
		{"help_centers", "/help_center/help_centers", nil},
		{"collections", "/help_center/collections", nil},
		{"snippets", "/content_snippets", nil},
		{"macros", "/macros", nil},
		{"office_hours", "/office_hours_schedules", nil},
		{"internal_articles", "/internal_articles", nil},
	}
}

/*
inventoryConversations counts the part, source and author types of the
first page of conversations and probes their files. It returns how many
admin authors GET /admins doesn't list.
*/
func (s *Source) inventoryConversations(ctx context.Context, iv *inventory, known map[string]bool) int {
	r := iv.r
	former := map[string]bool{}

	var convs struct {
		Conversations []conversation `json:"conversations"`
	}

	if err := s.search(ctx, "/conversations/search", searchPage{}, &convs); err != nil {
		r.Errors["conversations"] = err.Error()
	}

	for i, c := range convs.Conversations {
		var raw json.RawMessage

		q := url.Values{"page": {"1"}, "per_page": {"500"}}
		if err := s.Client.get(ctx, "/conversations/"+url.PathEscape(string(c.ID)), q, &raw); err != nil {
			r.Errors["conversation"] = err.Error()

			continue
		}

		if i == 0 {
			iv.sample("conversation", raw)
		}

		var full conversation
		_ = json.Unmarshal(raw, &full)

		r.SourceTypes[full.Source.Type]++
		r.AuthorTypes[full.Source.Author.Type]++
		s.inventoryFiles(ctx, r, full.Source.Attachments)

		for _, p := range full.Parts.Parts {
			if p.Author.Type == "admin" && !known[string(p.Author.ID)] {
				former[string(p.Author.ID)] = true
			}

			s.inventoryPart(ctx, r, p)
		}
	}

	return len(former)
}

/*
inventoryPart counts a conversation part's type and author and probes
its files and pictures.
*/
func (s *Source) inventoryPart(ctx context.Context, r *Report, p part) {
	r.PartTypes[p.PartType]++
	r.AuthorTypes[p.Author.Type]++

	s.inventoryFiles(ctx, r, p.Attachments)

	for _, src := range imgSrc.FindAllStringSubmatch(p.Body, -1) {
		s.inventoryFiles(ctx, r, []attachment{{URL: src[2]}})
	}
}

/*
inventoryTickets counts the part types of the first page of tickets,
keeping one sample.
*/
func (s *Source) inventoryTickets(ctx context.Context, iv *inventory) {
	var tickets struct {
		Tickets []ticket `json:"tickets"`
	}

	if err := s.search(ctx, "/tickets/search", searchPage{}, &tickets); err != nil {
		return
	}

	for i, t := range tickets.Tickets {
		raw := iv.get(ctx, "ticket", "/tickets/"+url.PathEscape(string(t.ID)), nil)
		if i > 0 {
			iv.r.Shapes = iv.r.Shapes[:len(iv.r.Shapes)-1]
		}

		var full ticket
		_ = json.Unmarshal(raw, &full)

		for _, p := range full.Parts.Parts {
			iv.r.PartTypes["ticket:"+p.PartType]++
		}
	}
}

func (s *Source) inventoryArticles(ctx context.Context, r *Report, raw json.RawMessage) {
	var list struct {
		Data []article `json:"data"`
	}

	if json.Unmarshal(raw, &list) != nil {
		return
	}

	for _, a := range list.Data {
		for _, b := range articleBodies(a) {
			s.inventoryBody(ctx, r, b[0], b[1])
		}
	}
}

/*
articleBodies returns an article's HTML and Markdown bodies, its own and
each translation's, by locale.
*/
func articleBodies(a article) map[string][2]string {
	bodies := map[string][2]string{"default": {a.Body, a.BodyMarkdown}}

	for l, v := range a.Translated {
		var c articleContent
		if l != "type" && json.Unmarshal(v, &c) == nil && c.Body != "" {
			bodies[l] = [2]string{c.Body, c.BodyMarkdown}
		}
	}

	return bodies
}

/*
inventoryBody counts the tags, Intercom classes and blocks one article
body uses, the blocks its Markdown drops, and probes its pictures.
*/
func (s *Source) inventoryBody(ctx context.Context, r *Report, body, md string) {
	for _, m := range htmlTag.FindAllStringSubmatch(body, -1) {
		r.ArticleTags[strings.ToLower(m[1])]++
	}

	for _, m := range icClass.FindAllString(body, -1) {
		r.ArticleClasses[m]++
	}

	for _, b := range blocks() {
		if b.in(body) && !markdownKeeps(b.name, md) {
			r.MarkdownLosses[b.name]++
		}
	}

	for _, src := range imgSrc.FindAllStringSubmatch(body, -1) {
		s.inventoryFiles(ctx, r, []attachment{{URL: src[2]}})
	}
}

/*
markdownKeeps reports whether body_markdown still carries a block the
HTML has.
*/
func markdownKeeps(block, md string) bool {
	switch block {
	case "image":
		return strings.Contains(md, "![")
	case "table":
		return strings.Contains(md, "|")
	case "code":
		return strings.Contains(md, "```")
	case "horizontal_rule":
		return strings.Contains(md, "---")
	case "button", "video", "callout", "collapsible", "centered":
		return false
	}

	return true
}

/*
inventoryAnonymous looks for a visitor's anonymous id in every lead's
fields, in GET /visitors?user_id= and in the lead's events.
*/
func (s *Source) inventoryAnonymous(
	ctx context.Context, contactsRaw json.RawMessage, visitorID string,
) *AnonymousReport {
	out := &AnonymousReport{}

	var leadID string

	out.ContactFields, leadID = s.leadFields(ctx, contactsRaw, visitorID)

	var visitor json.RawMessage

	if err := s.Client.get(ctx, "/visitors", url.Values{"user_id": {visitorID}}, &visitor); err != nil {
		out.VisitorLookup = err.Error()
	} else {
		out.VisitorLookup = "found"
		out.VisitorFields = matches(visitor, visitorID)
	}

	if leadID == "" {
		out.EventsLookup = "no lead carries the id"

		return out
	}

	var events json.RawMessage

	err := s.Client.get(ctx, "/events", url.Values{"type": {"user"}, "intercom_user_id": {leadID}}, &events)
	if err != nil {
		out.EventsLookup = err.Error()
	} else {
		out.EventsLookup = fmt.Sprintf("%d bytes", len(events))
		out.EventFields = matches(events, visitorID)
	}

	return out
}

/*
leadFields reads each listed contact in full and returns the field paths
holding the visitor's id, and the last lead that carries it.
*/
func (s *Source) leadFields(ctx context.Context, contactsRaw json.RawMessage, visitorID string) ([]string, string) {
	var list struct {
		Data []json.RawMessage `json:"data"`
	}

	_ = json.Unmarshal(contactsRaw, &list)

	var fields []string
	var leadID string

	for _, raw := range list.Data {
		var c contact
		_ = json.Unmarshal(raw, &c)

		var full json.RawMessage
		if err := s.Client.get(ctx, "/contacts/"+url.PathEscape(string(c.ID)), nil, &full); err == nil {
			raw = full
		}

		if found := matches(raw, visitorID); len(found) > 0 {
			fields = append(fields, found...)
			leadID = string(c.ID)
		}
	}

	return fields, leadID
}
