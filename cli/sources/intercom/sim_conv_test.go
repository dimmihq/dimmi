package intercom

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (w *world) conv(id string) *simConv {
	for _, c := range w.convs {
		if c.id == id {
			return c
		}
	}

	return nil
}

func (w *world) partJSON(p simPart, base string) map[string]any {
	out := map[string]any{
		"type": "conversation_part", "id": p.id, "part_type": p.typ, "created_at": p.at, "updated_at": p.at,
		"author": map[string]any{
			"type": p.authorType, "id": p.authorID, "name": "Person " + p.authorID, "email": p.authorID + "@sim.test",
		},
		"attachments": []any{},
	}

	switch p.typ {
	case "comment", "note":
		out["body"] = "<p>Message " + p.id + "</p>"
	case "assignment":
		out["assigned_to"] = map[string]any{"type": "admin", "id": "a2"}
	}

	if p.file != "" {
		expires := time.Now().Add(w.linkTTL).UnixMilli()
		out["attachments"] = []any{map[string]any{
			"type": "upload", "name": p.file, "content_type": "application/pdf", "filesize": len(simFile + p.file),
			"url": fmt.Sprintf("%s/cdn/%s?expires=%d&signature=sig", base, p.file, expires),
		}}
	}

	return out
}

func hostOf(r *http.Request) string { return "http://" + r.Host }

func (w *world) serveConversation(rw http.ResponseWriter, r *http.Request) any {
	n := w.convReads.Add(1)
	if w.onRead != nil {
		w.onRead(n)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	c := w.conv(r.PathValue("id"))
	if c == nil || c.ticket {
		http.Error(rw, `{"errors":[{"code":"not_found"}]}`, http.StatusNotFound)

		return nil
	}

	page, per := intParam(r, "page", 0), intParam(r, "per_page", 20)
	if per > 500 {
		w.t.Errorf("conversation per_page %d over 500", per)
	}

	parts := c.parts
	if page == 0 {
		parts = parts[max(0, len(parts)-500):] // The latest 500, as Intercom answers without page.
		page = 1
	} else {
		parts = parts[min((page-1)*per, len(parts)):min(page*per, len(parts))]
	}

	var items []any
	for _, p := range parts {
		items = append(items, w.partJSON(p, hostOf(r)))
	}

	out := w.convJSON(c)
	out["conversation_parts"] = map[string]any{
		"type": "conversation_part.list", "conversation_parts": items, "total_count": len(items),
		"pages": map[string]any{
			"type": "pages", "page": page, "per_page": per, "total_pages": (len(c.parts) + per - 1) / per,
		},
	}

	return out
}

func (w *world) convJSON(c *simConv) map[string]any {
	out := map[string]any{
		"type": "conversation", "id": c.id, "created_at": c.created, "updated_at": c.updated, "state": "open",
		"priority": "not_priority", "admin_assignee_id": c.assignee, "team_assignee_id": c.team,
		"source": map[string]any{
			"type": "conversation", "id": "src" + c.id, "delivered_as": "customer_initiated", "body": "<p>Hello</p>",
			"author": map[string]any{"type": "user", "id": c.contact},
		},
		"contacts": map[string]any{
			"type": "contact.list", "contacts": []any{map[string]any{"type": "contact", "id": c.contact}},
		},
		"tags": map[string]any{"type": "tag.list", "tags": tagList(c.tags)},
	}

	if c.company != "" {
		out["company"] = map[string]any{"type": "company", "id": c.company}
	}

	return out
}

func (w *world) serveTicket(rw http.ResponseWriter, r *http.Request) any {
	w.mu.Lock()
	defer w.mu.Unlock()

	c := w.conv(r.PathValue("id"))
	if c == nil || !c.ticket {
		http.Error(rw, `{"errors":[{"code":"not_found"}]}`, http.StatusNotFound)

		return nil
	}

	var items []any
	for _, p := range c.parts {
		items = append(items, w.partJSON(p, hostOf(r)))
	}

	return map[string]any{
		"type": "ticket", "id": c.id, "ticket_id": "T" + c.id, "category": "Customer",
		"ticket_attributes": map[string]any{
			"_default_title_": "Broken export " + c.id, "_default_description_": "<p>It fails</p>",
		},
		"ticket_state": map[string]any{"id": "ts2", "category": "in_progress"}, "ticket_type": simTicketType(),
		"contacts":          map[string]any{"contacts": []any{map[string]any{"type": "contact", "id": c.contact}}},
		"admin_assignee_id": c.assignee, "team_assignee_id": c.team, "created_at": c.created, "updated_at": c.updated,
		"open":         true,
		"ticket_parts": map[string]any{"type": "ticket_part.list", "ticket_parts": items, "total_count": len(items)},
	}
}

/*
serveScroll scrolls companies, keeping Intercom's rule of one scroll
open at a time, closed after it sits unused.
*/
func (w *world) serveScroll(rw http.ResponseWriter, r *http.Request) any {
	w.mu.Lock()
	defer w.mu.Unlock()

	param := r.URL.Query().Get("scroll_param")
	open := w.scroll != "" && time.Since(w.scrollUsed) < scrollIdle

	if param == "" && open {
		http.Error(rw, `{"errors":[{"code":"scroll_exists","message":"scroll already open"}]}`, http.StatusBadRequest)

		return nil
	}

	pos := 0

	if param != "" {
		name, at, _ := strings.Cut(param, ":")
		if !open || name != w.scroll {
			http.Error(rw, `{"errors":[{"code":"not_found"}]}`, http.StatusNotFound)

			return nil
		}

		pos, _ = strconv.Atoi(at)
	} else {
		w.scrollSeq++
		w.scroll = fmt.Sprintf("scroll%d", w.scrollSeq)
	}

	w.scrollUsed = time.Now()

	end := min(pos+50, len(w.companies))
	if pos >= len(w.companies) {
		w.scroll = ""

		return map[string]any{"type": "list", "data": []any{}, "scroll_param": nil}
	}

	var items []any
	for _, id := range w.companies[pos:end] {
		items = append(items, map[string]any{
			"type": "company", "id": id, "company_id": "ext-" + id, "name": "Company " + id,
			"created_at": 1_700_000_000, "updated_at": 1_700_000_000,
			"notes": map[string]any{"type": "note.list", "total_count": w.coNotes[id]},
		})
	}

	return map[string]any{"type": "list", "data": items, "scroll_param": fmt.Sprintf("%s:%d", w.scroll, end)}
}

func (w *world) serveArticles(_ http.ResponseWriter, r *http.Request) any {
	var items []any
	for i := range w.articles {
		items = append(items, map[string]any{
			"type": "article", "id": fmt.Sprintf("art%d", i+1), "title": fmt.Sprintf("Article %d", i+1),
			"body": "<p>How to</p>", "author_id": articleAuthor(i), "state": "published", "created_at": 1_700_000_000,
			"updated_at": 1_700_000_000, "parent_ids": []int{1 + i%12}, "default_locale": "en",
			"help_center_audience": "everyone",
		})
	}

	return pageOf(r, items, 250)
}

/*
articleAuthor is who wrote an article: a teammate, or for some one
removed from Intercom that nothing else names.
*/
func articleAuthor(i int) string {
	if i%50 == 0 {
		return "199"
	}

	return "a1"
}

func (w *world) serveMacros(_ http.ResponseWriter, r *http.Request) any {
	per := intParam(r, "per_page", 50)
	if per > 150 {
		w.t.Errorf("/macros per_page %d over 150", per)
	}

	start := 0
	if c := r.URL.Query().Get("starting_after"); c != "" {
		_, id := after(c)
		start, _ = strconv.Atoi(strings.TrimPrefix(id, "m"))
	}

	const total = 130

	var items []any
	for i := start; i < min(start+per, total); i++ {
		items = append(items, map[string]any{
			"type": "macro", "id": fmt.Sprintf("m%d", i+1), "name": fmt.Sprintf("Macro %d", i+1),
			"body": "<p>Hi {{first_name|fallback:\"there\"}}</p>", "visible_to": "everyone",
			"created_at": "2025-01-01T00:00:00Z", "updated_at": "2025-01-01T00:00:00Z",
		})
	}

	pages := map[string]any{"type": "pages", "per_page": per}
	if start+per < total {
		pages["next"] = map[string]any{"starting_after": cursor(0, fmt.Sprintf("m%d", start+per))}
	}

	return map[string]any{"type": "list", "data": items, "pages": pages}
}

/*
serveFile serves an attachment from the CDN, refusing a link whose
signature expired.
*/
func (w *world) serveFile(rw http.ResponseWriter, r *http.Request) {
	ms, _ := strconv.ParseInt(r.URL.Query().Get("expires"), 10, 64)
	if time.Now().UnixMilli() > ms {
		http.Error(rw, "expired", http.StatusForbidden)

		return
	}

	w.files.Add(1)
	_, _ = io.WriteString(rw, simFile+r.PathValue("name"))
}

/*
scrollIdle is how long the simulated Intercom keeps an unused scroll
open.
*/
const scrollIdle = 150 * time.Millisecond
