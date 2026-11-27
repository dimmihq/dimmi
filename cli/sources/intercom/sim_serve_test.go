package intercom

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"slices"
	"strconv"
	"time"
)

/*
serve answers like Intercom: it checks the token and version, rate
limits at random, refuses page sizes over the documented maximums, and
fails a few requests outright.
*/
func (w *world) serve(rng *rand.Rand) http.Handler {
	mux := http.NewServeMux()
	route := func(pattern string, fn func(http.ResponseWriter, *http.Request) any) {
		mux.HandleFunc(pattern, func(rw http.ResponseWriter, r *http.Request) {
			w.requests.Add(1)

			if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("Intercom-Version") != APIVersion {
				http.Error(rw, `{"errors":[{"code":"unauthorized"}]}`, http.StatusUnauthorized)

				return
			}

			w.mu.Lock()
			roll := rng.IntN(100)
			w.mu.Unlock()

			rw.Header().Set("X-RateLimit-Limit", "10000")
			rw.Header().Set("X-RateLimit-Remaining", "9000")
			rw.Header().Set("X-RateLimit-Reset", strconv.FormatInt(time.Now().Add(10*time.Second).Unix(), 10))

			switch {
			case roll < 4:
				w.limited.Add(1)
				rw.Header().Set("Retry-After", "1")
				http.Error(rw, `{"errors":[{"code":"rate_limit_exceeded"}]}`, http.StatusTooManyRequests)

				return
			case roll == 4:
				w.failed.Add(1)
				http.Error(rw, `{"errors":[{"code":"service_unavailable"}]}`, http.StatusServiceUnavailable)

				return
			}

			out := fn(rw, r)
			if out == nil {
				return
			}

			rw.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(rw).Encode(out)
		})
	}

	w.routes(route)

	mux.HandleFunc("GET /cdn/{name}", w.serveFile)
	mux.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		w.t.Errorf("called %s %s, which the simulation doesn't serve", r.Method, r.URL.Path)
		http.NotFound(rw, r)
	})

	return mux
}

type handler = func(http.ResponseWriter, *http.Request) any

func (w *world) routes(route func(string, handler)) {
	route("GET /me", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "admin", "id": "a1", "app": map[string]any{"id_code": "simapp", "name": "Sim"}}
	})
	route("GET /admins", func(http.ResponseWriter, *http.Request) any {
		var list []any
		for _, a := range w.admins {
			list = append(list, map[string]any{
				"type": "admin", "id": a, "name": "Admin " + a, "email": a + "@sim.test",
			})
		}

		return map[string]any{"type": "admin.list", "admins": list}
	})
	route("GET /teams", func(http.ResponseWriter, *http.Request) any {
		var list []any
		for _, t := range w.teams {
			list = append(list, map[string]any{"type": "team", "id": t, "name": "Team " + t, "admin_ids": w.admins[:2]})
		}

		return map[string]any{"type": "team.list", "teams": list}
	})
	route("GET /tags", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": tagList(w.tags)}
	})
	route("GET /data_attributes", func(_ http.ResponseWriter, r *http.Request) any {
		return map[string]any{"type": "list", "data": []any{map[string]any{
			"type": "data_attribute", "id": 1, "model": r.URL.Query().Get("model"), "name": "plan_level",
			"full_name": "custom_attributes.plan_level", "label": "Plan level", "data_type": "string", "custom": true,
		}}}
	})
	route("GET /conversations/attributes", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{}}
	})
	route("GET /companies/scroll", w.serveScroll)
	route("GET /contacts", w.serveContacts)
	route("POST /contacts/search", w.searchContacts)
	route("GET /contacts/{id}", func(_ http.ResponseWriter, r *http.Request) any {
		return contactJSON(w.contact(r.PathValue("id")))
	})
	route("GET /companies", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{}, "total_count": len(w.companies)}
	})
	route("GET /contacts/{id}/tags", func(_ http.ResponseWriter, r *http.Request) any {
		return map[string]any{"type": "list", "data": tagList(w.contact(r.PathValue("id")).tags)}
	})
	route("GET /contacts/{id}/companies", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{}, "pages": map[string]any{"page": 1, "total_pages": 1}}
	})
	route("GET /contacts/{id}/notes", func(_ http.ResponseWriter, r *http.Request) any {
		c := w.contact(r.PathValue("id"))
		if c.notes == 0 {
			w.t.Errorf("asked for the notes of %s, whose list says it has none", c.id)
		}

		return notePage(r, c.id, "contact", c.notes)
	})
	route("GET /companies/{id}/notes", func(_ http.ResponseWriter, r *http.Request) any {
		return notePage(r, r.PathValue("id"), "company", w.coNotes[r.PathValue("id")])
	})
	route("GET /ticket_types", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{simTicketType()}}
	})
	route("GET /help_center/help_centers", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{map[string]any{
			"id": "hc1", "identifier": "sim", "display_name": "Sim help", "default": true, "locales": []string{"en"},
		}}}
	})
	route("GET /help_center/collections", func(_ http.ResponseWriter, r *http.Request) any {
		var items []any
		for i := range 12 {
			items = append(items, map[string]any{
				"id": fmt.Sprintf("col%d", i+1), "name": fmt.Sprintf("Collection %d", i+1), "default_locale": "en",
				"help_center_id": "hc1", "order": i,
			})
		}

		return pageOf(r, items, 250)
	})
	route("GET /articles", w.serveArticles)
	route("GET /internal_articles", func(rw http.ResponseWriter, _ *http.Request) any {
		http.Error(rw, `{"errors":[{"code":"not_found"}]}`, http.StatusNotFound)

		return nil
	})
	route("GET /content_snippets", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{}, "page": 1, "per_page": 50, "total_pages": 0}
	})
	route("GET /macros", w.serveMacros)
	route("GET /office_hours_schedules", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{map[string]any{
			"id": "oh1", "name": "Office hours", "time_zone_name": "Europe/London",
			"time_intervals": []any{map[string]any{"start_minute": 540, "end_minute": 1020}},
		}}}
	})
	route("GET /office_hours_schedules/{id}/office_hours_exceptions", func(http.ResponseWriter, *http.Request) any {
		return map[string]any{"type": "list", "data": []any{}}
	})
	route("POST /conversations/search", func(_ http.ResponseWriter, r *http.Request) any {
		return w.search(r, "conversations")
	})
	route("POST /tickets/search", func(_ http.ResponseWriter, r *http.Request) any {
		return w.search(r, "tickets")
	})
	route("GET /conversations/{id}", w.serveConversation)
	route("GET /tickets/{id}", w.serveTicket)
}

func tagList(ids []string) []any {
	out := make([]any, 0, len(ids))
	for _, id := range ids {
		out = append(out, map[string]any{"type": "tag", "id": id, "name": "Tag " + id})
	}

	return out
}

func simTicketType() map[string]any {
	return map[string]any{
		"id": "tt1", "category": "Customer", "name": "Bug report",
		"ticket_type_attributes": map[string]any{"ticket_type_attributes": []any{
			map[string]any{"id": "ta1", "name": "_default_title_", "data_type": "string", "default": true},
			map[string]any{"id": "ta2", "name": "_default_description_", "data_type": "string", "default": true},
		}},
		"ticket_states": map[string]any{"data": []any{
			map[string]any{"id": "ts1", "category": "submitted", "internal_label": "Submitted"},
			map[string]any{"id": "ts2", "category": "in_progress", "internal_label": "In progress"},
			map[string]any{"id": "ts3", "category": "resolved", "internal_label": "Resolved"},
		}},
	}
}

func (w *world) contact(id string) *simContact {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, c := range append(slices.Clone(w.contacts), w.archived) {
		if c.id == id {
			return c
		}
	}

	w.t.Errorf("no contact %s", id)

	return &simContact{id: id}
}

func intParam(r *http.Request, name string, def int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil {
		return n
	}

	return def
}

/*
pageOf serves a page-numbered list, refusing pages over max.
*/
func pageOf(r *http.Request, items []any, maxPer int) map[string]any {
	per, page := intParam(r, "per_page", 50), intParam(r, "page", 1)
	if per > maxPer {
		per = maxPer
	}

	total := (len(items) + per - 1) / per
	from, to := min((page-1)*per, len(items)), min(page*per, len(items))

	return map[string]any{
		"type": "list", "data": items[from:to], "total_count": len(items),
		"pages": map[string]any{"type": "pages", "page": page, "per_page": per, "total_pages": total},
	}
}

func notePage(r *http.Request, owner, ownerType string, n int) map[string]any {
	var items []any
	for i := range n {
		author := map[string]any{"type": "admin", "id": "a1", "name": "Admin a1", "email": "a1@sim.test"}
		if owner == "co1" {
			author = map[string]any{"type": "admin", "id": "a98", "name": "Gone", "email": "a98@sim.test"}
		}

		items = append(items, map[string]any{
			"type": "note", "id": fmt.Sprintf("n-%s-%d", owner, i), "created_at": 1_700_000_000 + i,
			"body": "<p>Note</p>", "author": author,
			ownerType: map[string]any{"type": ownerType, "id": owner},
		})
	}

	return pageOf(r, items, 60)
}
