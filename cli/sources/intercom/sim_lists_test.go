package intercom

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
)

/*
cursor encodes the position after an item: its updated time and id, the
way Intercom's cursors are keyed.
*/
func cursor(at int64, id string) string {
	return base64.StdEncoding.EncodeToString([]byte(strconv.FormatInt(at, 10) + "|" + id))
}

func after(c string) (int64, string) {
	raw, err := base64.StdEncoding.DecodeString(c)
	if err != nil {
		return 0, ""
	}

	at, id, _ := strings.Cut(string(raw), "|")
	n, _ := strconv.ParseInt(at, 10, 64)

	return n, id
}

func less(at1 int64, id1 string, at2 int64, id2 string) bool {
	return at1 < at2 || (at1 == at2 && id1 < id2)
}

func (w *world) serveContacts(_ http.ResponseWriter, r *http.Request) any {
	per := intParam(r, "per_page", 10)
	if per > 150 {
		w.t.Errorf("/contacts per_page %d over 150", per)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	start := 0
	if c := r.URL.Query().Get("starting_after"); c != "" {
		_, id := after(c)
		start = slices.IndexFunc(w.contacts, func(s *simContact) bool { return s.id == id }) + 1
	}

	page := w.contacts[min(start, len(w.contacts)):min(start+per, len(w.contacts))]

	return w.contactList(page, len(w.contacts)-(start+per))
}

/*
contactList serves a page of contacts, with a cursor to the next page
when rest more follow it.
*/
func (w *world) contactList(page []*simContact, rest int) map[string]any {
	var items []any
	for _, c := range page {
		items = append(items, contactJSON(c))
	}

	pages := map[string]any{"type": "pages", "per_page": len(page)}
	if rest > 0 && len(page) > 0 {
		last := page[len(page)-1]
		pages["next"] = map[string]any{"starting_after": cursor(last.updated, last.id)}
	}

	return map[string]any{"type": "list", "data": items, "total_count": len(w.contacts), "pages": pages}
}

func contactJSON(c *simContact) map[string]any {
	shown := c.tags[:min(len(c.tags), 10)]

	var companies []any
	for _, co := range c.companies {
		companies = append(companies, map[string]any{"type": "company", "id": co})
	}

	var notes []any
	for i := range min(c.notes, 10) {
		notes = append(notes, map[string]any{"type": "note", "id": fmt.Sprintf("n-%s-%d", c.id, i)})
	}

	return map[string]any{
		"type": "contact", "id": c.id, "role": "user", "email": c.id + "@customer.test", "name": "Customer " + c.id,
		"created_at": c.created, "updated_at": c.updated, "custom_attributes": map[string]any{"plan_level": "pro"},
		"tags": map[string]any{
			"type": "list", "data": tagList(shown), "has_more": len(c.tags) > 10, "total_count": len(c.tags),
		},
		"companies": map[string]any{"type": "list", "data": companies, "total_count": len(companies)},
		"notes":     map[string]any{"type": "list", "data": notes, "total_count": c.notes, "has_more": c.notes > 10},
	}
}

/*
searchBody is the body of a search: a filter on updated_at, a page and
a sort, which must be oldest first.
*/
type searchBody struct {
	Query struct {
		Field    string `json:"field"`
		Operator string `json:"operator"`
		Value    int64  `json:"value"`
	} `json:"query"`
	Pagination struct {
		PerPage       int    `json:"per_page"`
		StartingAfter string `json:"starting_after"`
	} `json:"pagination"`
	Sort struct {
		Field string `json:"field"`
		Order string `json:"order"`
	} `json:"sort"`
}

func (w *world) readSearch(r *http.Request) searchBody {
	var b searchBody
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		w.t.Errorf("search body: %v", err)

		return b
	}

	if b.Query.Field != "updated_at" || b.Query.Operator != ">" || b.Pagination.PerPage > 150 {
		w.t.Errorf("search %s: %+v", r.URL.Path, b)
	}

	if b.Sort.Field != "updated_at" || b.Sort.Order != "ascending" {
		w.t.Errorf("search %s reads newest first", r.URL.Path)
	}

	return b
}

func (w *world) searchContacts(_ http.ResponseWriter, r *http.Request) any {
	b := w.readSearch(r)

	w.mu.Lock()
	defer w.mu.Unlock()

	var match []*simContact
	for _, c := range w.contacts {
		if c.updated > b.Query.Value {
			match = append(match, c)
		}
	}

	sort.Slice(match, func(i, j int) bool { return less(match[i].updated, match[i].id, match[j].updated, match[j].id) })

	page, rest := keyset(match, b, func(c *simContact) (int64, string) { return c.updated, c.id })

	return w.contactList(page, rest)
}

/*
keyset serves the page after the cursor, by updated time then id, so a
record that changes moves past the cursor instead of shifting others,
and how many follow the page.
*/
func keyset[T any](all []T, b searchBody, key func(T) (int64, string)) ([]T, int) {
	start := 0

	if b.Pagination.StartingAfter != "" {
		at, id := after(b.Pagination.StartingAfter)
		start = sort.Search(len(all), func(i int) bool {
			a, ai := key(all[i])

			return less(at, id, a, ai)
		})
	}

	per := max(b.Pagination.PerPage, 1)
	end := min(start+per, len(all))

	return all[start:end], len(all) - end
}

/*
search serves a search of conversations or of tickets, keyed as
Intercom keys their lists: "conversations" or "tickets".
*/
func (w *world) search(r *http.Request, key string) any {
	b := w.readSearch(r)
	tickets := key == "tickets"

	w.mu.Lock()
	defer w.mu.Unlock()

	var match []*simConv
	for _, c := range w.convs {
		if c.ticket == tickets && c.updated > b.Query.Value {
			match = append(match, c)
		}
	}

	sort.Slice(match, func(i, j int) bool { return less(match[i].updated, match[i].id, match[j].updated, match[j].id) })

	page, rest := keyset(match, b, func(c *simConv) (int64, string) { return c.updated, c.id })

	var items []any
	for _, c := range page {
		items = append(items, map[string]any{"type": "conversation", "id": c.id, "updated_at": c.updated})
	}

	pages := map[string]any{"type": "pages", "per_page": len(page)}
	if rest > 0 {
		last := page[len(page)-1]
		pages["next"] = map[string]any{"starting_after": cursor(last.updated, last.id)}
	}

	return map[string]any{"type": key, key: items, "total_count": len(match), "pages": pages}
}
