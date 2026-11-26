package intercom

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
specRoutes maps each endpoint the CLI reads onto its fixture in
testdata/spec. The fixtures come from Intercom's own OpenAPI
description of API 2.18 (github.com/intercom/Intercom-OpenAPI, MIT):
NAME.example.json is Intercom's example answer, NAME.schema.json an
answer built from the response schema with every property set.
*/
func specRoutes() map[string]string {
	return map[string]string{
		"GET /admins":                   "admins",
		"GET /teams":                    "teams",
		"GET /tags":                     "tags",
		"GET /data_attributes":          "data_attributes",
		"GET /conversations/attributes": "conversation_attributes",
		"GET /companies/scroll":         "companies_scroll",
		"GET /contacts":                 "contacts",
		"POST /contacts/search":         "contacts_search",
		"GET /contacts/{id}":            "contact",
		"GET /contacts/{id}/notes":      "contact_notes",
		"GET /companies/{id}/notes":     "company_notes",
		"GET /contacts/{id}/tags":       "contact_tags",
		"GET /contacts/{id}/companies":  "contact_companies",
		"GET /ticket_types":             "ticket_types",
		"GET /help_center/help_centers": "help_centers",
		"GET /help_center/collections":  "collections",
		"GET /articles":                 "articles",
		"GET /internal_articles":        "internal_articles",
		"GET /content_snippets":         "snippets",
		"GET /macros":                   "macros",
		"GET /office_hours_schedules":   "office_hours",
		"GET /office_hours_schedules/{id}/office_hours_exceptions": "office_hours_exceptions",
		"POST /conversations/search":                               "conversations_search",
		"GET /conversations/{id}":                                  "conversation",
		"POST /tickets/search":                                     "tickets_search",
		"GET /tickets/{id}":                                        "ticket",
		"GET /me":                                                  "me",
		"GET /export/workflows/{id}":                               "workflow",
	}
}

/*
perPageMax is the largest per_page Intercom's description allows on the
endpoints that cap it; searches cap pagination.per_page at 150.
*/
func perPageMax() map[string]int {
	return map[string]int{
		"GET /contacts": 150, "GET /articles": 250, "GET /help_center/collections": 250, "GET /macros": 150,
		"GET /contacts/{id}/companies": 60, "GET /conversations/{id}": 500, "POST /contacts/search": 150,
		"POST /conversations/search": 150, "POST /tickets/search": 150,
	}
}

/*
perPage reads the page size a request asks for, from its query or its
search body.
*/
func perPage(r *http.Request) int {
	if v := r.URL.Query().Get("per_page"); v != "" {
		n, _ := strconv.Atoi(v)

		return n
	}

	if r.Method != http.MethodPost {
		return 0
	}

	raw, _ := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(raw))

	var body struct {
		Pagination struct {
			PerPage int `json:"per_page"`
		} `json:"pagination"`
	}

	_ = json.Unmarshal(raw, &body)

	return body.Pagination.PerPage
}

/*
isList reports whether the key is one under which Intercom's answers
carry the items of a page.
*/
func isList(key string) bool {
	switch key {
	case "data", "conversations", "tickets", "conversation_parts", "ticket_parts":
		return true
	default:
		return false
	}
}

/*
emptied returns an answer with every page of items emptied and no next
page: what Intercom sends past the last page.
*/
func emptied(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}

		for k, child := range x {
			switch {
			case isList(k):
				if _, ok := child.([]any); ok {
					out[k] = []any{}

					continue
				}

				out[k] = emptied(child)
			case k == "next" || k == "scroll_param":
				out[k] = nil
			default:
				out[k] = emptied(child)
			}
		}

		return out
	case []any:
		out := make([]any, len(x))
		for i, child := range x {
			out[i] = emptied(child)
		}

		return out
	}

	return v
}

/*
laterPage reports whether a request asks for a page after the first.
*/
func laterPage(r *http.Request) bool {
	q := r.URL.Query()
	if q.Get("starting_after") != "" || q.Get("scroll_param") != "" || (q.Get("page") != "" && q.Get("page") != "1") {
		return true
	}

	if r.Method != http.MethodPost {
		return false
	}

	var body struct {
		Pagination struct {
			StartingAfter string `json:"starting_after"`
		} `json:"pagination"`
	}

	_ = json.NewDecoder(r.Body).Decode(&body)

	return body.Pagination.StartingAfter != ""
}

/*
specIntercom serves the fixtures of one set, example or schema. A route
asked for its first page more than a few times fails the test: a reader
that never stops.
*/
func specIntercom(t *testing.T, set string) *httptest.Server {
	t.Helper()

	firsts := &firstPages{calls: map[string]int{}}
	limits := perPageMax()
	fixtures := os.DirFS("testdata/spec")
	mux := http.NewServeMux()

	for route, name := range specRoutes() {
		raw, err := fs.ReadFile(fixtures, name+"."+set+".json")
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}

		if err != nil {
			t.Fatal(err)
		}

		var answer any
		if err := json.Unmarshal(raw, &answer); err != nil {
			t.Fatal(err)
		}

		last, _ := json.Marshal(emptied(answer))

		f := specFixture{route: route, raw: raw, last: last, limit: limits[route]}
		mux.HandleFunc(route, specAnswer(t, f, firsts))
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("the CLI called %s %s, which isn't in Intercom's 2.18 description", r.Method, r.URL.Path)
		http.NotFound(w, r)
	})

	return httptest.NewServer(mux)
}

/*
specFixture is one route's answers: its first page, the page past its
last, and the largest per_page Intercom allows on it, if any.
*/
type specFixture struct {
	route     string
	raw, last []byte
	limit     int
}

/*
firstPages counts how often each route's first page is read.
*/
type firstPages struct {
	mu    sync.Mutex
	calls map[string]int
}

/*
specAnswer answers one route from its fixture.
*/
func specAnswer(t *testing.T, f specFixture, firsts *firstPages) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Intercom-Version") != APIVersion {
			http.Error(w, "no version", http.StatusBadRequest)

			return
		}

		if f.limit > 0 && perPage(r) > f.limit {
			t.Errorf("%s asks for %d a page, over Intercom's %d", f.route, perPage(r), f.limit)
		}

		if laterPage(r) {
			_, _ = w.Write(f.last)

			return
		}

		firsts.mu.Lock()
		firsts.calls[f.route+" "+r.URL.Path]++
		n := firsts.calls[f.route+" "+r.URL.Path]
		firsts.mu.Unlock()

		if n > 50 {
			t.Errorf("%s read its first page %d times", r.URL.Path, n)
		}

		_, _ = w.Write(f.raw)
	}
}

/*
TestReadsIntercomsOwnAnswers reads every kind from answers shaped
exactly as Intercom's API description says: its examples, and answers
with every property of every schema set. Each kind must read without an
error and give records.
*/
func TestReadsIntercomsOwnAnswers(t *testing.T) {
	for _, set := range []string{"example", "schema"} {
		t.Run(set, func(t *testing.T) {
			srv := specIntercom(t, set)
			defer srv.Close()

			src := New(&Client{HTTP: srv.Client(), Base: srv.URL, Token: "tok"}, "us", []string{"1"})

			if _, err := src.Account(context.Background()); err != nil {
				t.Fatalf("account: %v", err)
			}

			for _, kind := range src.Kinds() {
				readsSpecKind(t, src, set, kind)
			}
		})
	}
}

/*
readsSpecKind reads one kind from a set's answers: it must read without
an error and give records.
*/
func readsSpecKind(t *testing.T, src *Source, set string, kind format.Kind) {
	t.Helper()

	var recs []format.Record

	err := src.Read(context.Background(), kind, engine.ReadOptions{}, func(p engine.Page) error {
		recs = append(recs, p.Records...)

		return nil
	})
	if err != nil {
		t.Errorf("%s: %v", kind, err)

		return
	}

	for _, r := range recs {
		if _, err := json.Marshal(r.Data); err != nil {
			t.Errorf("%s %s: %v", kind, r.SourceID, err)
		}
	}

	t.Logf("%-22s %d records", kind, len(recs))

	if len(recs) == 0 && !strings.Contains(emptyOK(set), string(kind)) {
		t.Errorf("%s: no records from Intercom's answer", kind)
	}
}

/*
emptyOK names the kinds a set may give no records for, with why.
*/
func emptyOK(set string) string {
	switch set {
	case "example":
		// Intercom's examples of these lists are empty.
		return "team contact contact_note help_center"
	default:
		return ""
	}
}
