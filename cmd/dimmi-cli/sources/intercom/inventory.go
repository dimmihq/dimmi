package intercom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
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
keep are the keys whose values are kinds, not content, and stay in
the scrubbed shapes.
*/
var keep = map[string]bool{
	"type": true, "part_type": true, "data_type": true, "model": true, "state": true, "role": true,
	"category": true, "delivered_as": true, "exception_type": true, "visible_to": true, "parent_type": true,
	"help_center_audience": true, "priority": true, "content_type": true, "trigger_type": true, "channel": true,
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
		if keep[key] {
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
Inventory reads the workspace and reports its shapes. When dir is set,
it writes one scrubbed sample per endpoint there. visitorID is an
anonymous visitor's intercom-id cookie to look for.
*/
func (s *Source) Inventory(ctx context.Context, dir, visitorID string) (Report, error) {
	r := Report{
		PartTypes: map[string]int{}, SourceTypes: map[string]int{}, AuthorTypes: map[string]int{},
		ArticleTags: map[string]int{}, ArticleClasses: map[string]int{}, MarkdownLosses: map[string]int{},
		Attachments: AttachmentReport{Hosts: map[string]int{}}, Errors: map[string]string{},
	}

	sample := func(name string, raw json.RawMessage) {
		if dir == "" || len(raw) == 0 {
			return
		}

		var v any
		if json.Unmarshal(raw, &v) != nil {
			return
		}

		out, _ := json.MarshalIndent(scrub(v, ""), "", "  ")
		path := filepath.Join(dir, name+".json")

		if err := os.WriteFile(path, append(out, '\n'), 0o600); err == nil {
			r.Shapes = append(r.Shapes, path)
		}
	}

	get := func(name, path string, q url.Values) json.RawMessage {
		var raw json.RawMessage
		if err := s.Client.get(ctx, path, q, &raw); err != nil {
			r.Errors[name] = err.Error()

			return nil
		}

		sample(name, raw)

		return raw
	}

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
	get("admins", "/admins", nil)
	get("teams", "/teams", nil)
	get("tags", "/tags", nil)
	get("data_attributes_contact", "/data_attributes", url.Values{"model": {"contact"}})
	get("data_attributes_company", "/data_attributes", url.Values{"model": {"company"}})
	get("conversation_attributes", "/conversations/attributes", nil)
	get("companies_scroll", "/companies/scroll", nil)
	get("ticket_types", "/ticket_types", nil)
	get("help_centers", "/help_center/help_centers", nil)
	get("collections", "/help_center/collections", nil)
	get("snippets", "/content_snippets", nil)
	get("macros", "/macros", nil)
	get("office_hours", "/office_hours_schedules", nil)
	get("internal_articles", "/internal_articles", nil)

	contactsRaw := get("contacts", "/contacts", url.Values{"per_page": {"150"}})
	s.inventoryArticles(ctx, &r, get("articles", "/articles", url.Values{"per_page": {"250"}}))

	former := map[string]bool{}

	var convs struct {
		Conversations []conversation `json:"conversations"`
	}

	if err := s.search(ctx, "/conversations/search", time.Time{}, "", &convs); err != nil {
		r.Errors["conversations"] = err.Error()
	}

	for i, c := range convs.Conversations {
		var raw json.RawMessage
		if err := s.Client.get(ctx, "/conversations/"+url.PathEscape(string(c.ID)), url.Values{"page": {"1"}, "per_page": {"500"}}, &raw); err != nil {
			r.Errors["conversation"] = err.Error()

			continue
		}

		if i == 0 {
			sample("conversation", raw)
		}

		var full conversation
		_ = json.Unmarshal(raw, &full)

		r.SourceTypes[full.Source.Type]++
		r.AuthorTypes[full.Source.Author.Type]++
		s.inventoryFiles(ctx, &r, full.Source.Attachments)

		for _, p := range full.Parts.Parts {
			r.PartTypes[p.PartType]++
			r.AuthorTypes[p.Author.Type]++

			if p.Author.Type == "admin" && !known[string(p.Author.ID)] {
				former[string(p.Author.ID)] = true
			}

			s.inventoryFiles(ctx, &r, p.Attachments)

			for _, src := range imgSrc.FindAllStringSubmatch(p.Body, -1) {
				s.inventoryFiles(ctx, &r, []attachment{{URL: src[2]}})
			}
		}
	}

	var tickets struct {
		Tickets []ticket `json:"tickets"`
	}

	if err := s.search(ctx, "/tickets/search", time.Time{}, "", &tickets); err == nil {
		for i, t := range tickets.Tickets {
			raw := get("ticket", "/tickets/"+url.PathEscape(string(t.ID)), nil)
			if i > 0 {
				r.Shapes = r.Shapes[:len(r.Shapes)-1]
			}

			var full ticket
			_ = json.Unmarshal(raw, &full)

			for _, p := range full.Parts.Parts {
				r.PartTypes["ticket:"+p.PartType]++
			}
		}
	}

	r.FormerAuthors = len(former)

	if visitorID != "" {
		r.AnonymousID = s.inventoryAnonymous(ctx, contactsRaw, visitorID)
	}

	if len(r.Errors) == 0 {
		r.Errors = nil
	}

	return r, nil
}

func (s *Source) inventoryArticles(ctx context.Context, r *Report, raw json.RawMessage) {
	var list struct {
		Data []article `json:"data"`
	}

	if json.Unmarshal(raw, &list) != nil {
		return
	}

	for _, a := range list.Data {
		bodies := map[string][2]string{"default": {a.Body, a.BodyMarkdown}}

		for l, v := range a.Translated {
			var c articleContent
			if l != "type" && json.Unmarshal(v, &c) == nil && c.Body != "" {
				bodies[l] = [2]string{c.Body, c.BodyMarkdown}
			}
		}

		for _, b := range bodies {
			for _, m := range htmlTag.FindAllStringSubmatch(b[0], -1) {
				r.ArticleTags[strings.ToLower(m[1])]++
			}

			for _, m := range icClass.FindAllString(b[0], -1) {
				r.ArticleClasses[m]++
			}

			for block, re := range blockPatterns {
				if re.MatchString(b[0]) && !markdownKeeps(block, b[1]) {
					r.MarkdownLosses[block]++
				}
			}

			for _, src := range imgSrc.FindAllStringSubmatch(b[0], -1) {
				s.inventoryFiles(ctx, r, []attachment{{URL: src[2]}})
			}
		}
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
inventoryFiles notes how each attachment is served: its host, size,
whether its link is signed and when it expires, and whether it downloads
without the token. It reads at most a few bytes.
*/
func (s *Source) inventoryFiles(ctx context.Context, r *Report, list []attachment) {
	for _, a := range list {
		u, err := url.Parse(a.URL)
		if err != nil || u.Host == "" {
			continue
		}

		r.Attachments.Count++
		r.Attachments.Hosts[u.Host]++
		r.Attachments.LargestBytes = max(r.Attachments.LargestBytes, a.Filesize)

		if m := expiring.FindAllStringSubmatch(u.RawQuery, -1); len(m) > 0 {
			r.Attachments.SignedLinks++

			for _, p := range m {
				name := strings.ToLower(p[1])
				if !contains(r.Attachments.ExpiryParams, name) {
					r.Attachments.ExpiryParams = append(r.Attachments.ExpiryParams, name)
				}

				if name == "expires" {
					if at, err := strconv.ParseInt(p[2], 10, 64); err == nil {
						r.Attachments.ExpiresInSeconds = append(r.Attachments.ExpiresInSeconds, at-time.Now().Unix())
					}
				}
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
		if err != nil {
			continue
		}

		req.Header.Set("Range", "bytes=0-15")

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64))
		_ = res.Body.Close()

		if res.StatusCode < 300 {
			r.Attachments.DownloadedNoAuth++

			if a.Filesize == 0 {
				if cr := res.Header.Get("Content-Range"); cr != "" {
					if i := strings.LastIndex(cr, "/"); i >= 0 {
						if n, err := strconv.ParseInt(cr[i+1:], 10, 64); err == nil {
							r.Attachments.LargestBytes = max(r.Attachments.LargestBytes, n)
						}
					}
				}
			}
		} else {
			r.Attachments.RefusedNoAuth++
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}

	return false
}

/*
inventoryAnonymous looks for a visitor's anonymous id in every lead's
fields, in GET /visitors?user_id= and in the lead's events.
*/
func (s *Source) inventoryAnonymous(ctx context.Context, contactsRaw json.RawMessage, visitorID string) *AnonymousReport {
	out := &AnonymousReport{}

	var list struct {
		Data []json.RawMessage `json:"data"`
	}

	_ = json.Unmarshal(contactsRaw, &list)

	var leadID string

	for _, raw := range list.Data {
		var c contact
		_ = json.Unmarshal(raw, &c)

		var full json.RawMessage
		if err := s.Client.get(ctx, "/contacts/"+url.PathEscape(string(c.ID)), nil, &full); err == nil {
			raw = full
		}

		if found := matches(raw, visitorID); len(found) > 0 {
			out.ContactFields = append(out.ContactFields, found...)
			leadID = string(c.ID)
		}
	}

	var visitor json.RawMessage

	err := s.Client.get(ctx, "/visitors", url.Values{"user_id": {visitorID}}, &visitor)

	switch {
	case err != nil:
		out.VisitorLookup = err.Error()
	default:
		out.VisitorLookup = "found"
		out.VisitorFields = matches(visitor, visitorID)
	}

	if leadID == "" {
		out.EventsLookup = "no lead carries the id"

		return out
	}

	var events json.RawMessage

	err = s.Client.get(ctx, "/events", url.Values{"type": {"user"}, "intercom_user_id": {leadID}}, &events)
	if err != nil {
		out.EventsLookup = err.Error()
	} else {
		out.EventsLookup = fmt.Sprintf("%d bytes", len(events))
		out.EventFields = matches(events, visitorID)
	}

	return out
}
