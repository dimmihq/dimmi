package intercom

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
write answers with a fixed body.
*/
func write(w http.ResponseWriter, body string) {
	_, _ = io.WriteString(w, body)
}

/*
fakeIntercom serves two conversations through two search pages, one of
them with 750 parts that only paging returns in full, and a company
scroll of two pages. It refuses a request without the version header.
*/
func fakeIntercom(t *testing.T, limited *atomic.Int32) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	check := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Intercom-Version") != APIVersion || r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, `{"errors":[{"code":"unauthorized"}]}`, http.StatusUnauthorized)

			return false
		}

		if limited != nil && limited.Add(-1) >= 0 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)

			return false
		}

		return true
	}

	guard := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if check(w, r) {
				h(w, r)
			}
		}
	}

	mux.HandleFunc("GET /admins", guard(func(w http.ResponseWriter, _ *http.Request) {
		write(w, `{"admins":[{"type":"admin","id":"1","name":"A","email":"a@x"}]}`)
	}))
	mux.HandleFunc("POST /conversations/search", guard(fakeSearch))
	mux.HandleFunc("GET /conversations/{id}", guard(func(w http.ResponseWriter, r *http.Request) {
		fakeConversation(t, w, r)
	}))
	mux.HandleFunc("GET /companies/scroll", guard(fakeCompanies))

	return httptest.NewServer(mux)
}

func fakeSearch(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pagination struct {
			StartingAfter string `json:"starting_after"`
		} `json:"pagination"`
	}

	_ = json.NewDecoder(r.Body).Decode(&body)

	if body.Pagination.StartingAfter == "" {
		write(w, `{"conversations":[{"id":"c1"}],"pages":{"next":{"starting_after":"cur2"}}}`)

		return
	}

	write(w, `{"conversations":[{"id":"c2"}],"pages":{}}`)
}

func fakeConversation(t *testing.T, w http.ResponseWriter, r *http.Request) {
	t.Helper()

	id := r.PathValue("id")
	total := 3

	if id == "c1" {
		total = 750
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page == 0 {
		t.Error("conversation read without page: only the latest 500 parts would come back")
	}

	var parts []map[string]any

	for i := (page - 1) * 500; i < min(page*500, total); i++ {
		parts = append(parts, map[string]any{
			"id": fmt.Sprintf("%s-p%d", id, i), "part_type": "comment", "body": "<p>x</p>", "created_at": 1000 + i,
			"author": map[string]any{"type": "admin", "id": "1"},
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": id, "created_at": 1000, "updated_at": 2000, "state": "open",
		"source": map[string]any{
			"type": "conversation", "id": "s" + id, "body": "<p>hi</p>",
			"author": map[string]any{"type": "user", "id": "u1"},
		},
		"contacts":           map[string]any{"contacts": []any{map[string]any{"id": "u1"}}},
		"conversation_parts": map[string]any{"conversation_parts": parts, "total_count": total},
	})
}

func fakeCompanies(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Query().Get("scroll_param") {
	case "":
		write(w, `{"data":[{"id":"co1","name":"One"}],"scroll_param":"s2"}`)
	case "s2":
		write(w, `{"data":[{"id":"co2","name":"Two"}],"scroll_param":"s3"}`)
	default:
		write(w, `{"data":[],"scroll_param":null}`)
	}
}

func testSource(t *testing.T, limited *atomic.Int32) *Source {
	t.Helper()

	srv := fakeIntercom(t, limited)
	t.Cleanup(srv.Close)

	hc := &http.Client{Transport: engine.NewLimiter(http.DefaultTransport, 1, 0)}

	return &Source{Client: &Client{HTTP: hc, Base: srv.URL, Token: "tok"}, Workers: 4}
}

func TestConversationsReadEveryPartThroughSearchPages(t *testing.T) {
	src := testSource(t, nil)

	var recs []format.Record
	var cursors []string

	err := src.Read(context.Background(), format.KindConversation, engine.ReadOptions{}, func(p engine.Page) error {
		recs = append(recs, p.Records...)
		cursors = append(cursors, p.Cursor)

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(recs) != 2 || cursors[0] != "cur2" || cursors[1] != "" {
		t.Fatalf("records %d cursors %q", len(recs), cursors)
	}

	c1, ok := recs[0].Data.(*format.Conversation)
	if !ok {
		t.Fatalf("record holds %T", recs[0].Data)
	}

	if len(c1.Entries) != 751 {
		t.Fatalf("c1 has %d entries, want the source and all 750 parts", len(c1.Entries))
	}

	if c1.Entries[1].SourceID != "c1-p0" || c1.Entries[750].SourceID != "c1-p749" {
		t.Fatal("parts out of order")
	}
}

func TestCompaniesScroll(t *testing.T) {
	src := testSource(t, nil)

	var ids []string

	err := src.Read(context.Background(), format.KindCompany, engine.ReadOptions{}, func(p engine.Page) error {
		for _, r := range p.Records {
			ids = append(ids, r.SourceID)
		}

		return nil
	})
	if err != nil || len(ids) != 2 {
		t.Fatalf("%v %v", ids, err)
	}
}

func TestReadsWaitOut429(t *testing.T) {
	var limited atomic.Int32
	limited.Store(2)

	src := testSource(t, &limited)

	if _, err := src.knownAdmins(context.Background()); err != nil {
		t.Fatalf("a 429 must be retried: %v", err)
	}
}

func TestRegions(t *testing.T) {
	hosts := map[string]string{
		"us": "https://api.intercom.io", "EU": "https://api.eu.intercom.io", "au": "https://api.au.intercom.io",
	}

	for region, host := range hosts {
		c, err := NewClient(nil, region, "t")
		if err != nil || c.Base != host {
			t.Errorf("%s: %v %v", region, c, err)
		}
	}

	if _, err := NewClient(nil, "mars", "t"); err == nil {
		t.Error("accepted an unknown region")
	}
}
