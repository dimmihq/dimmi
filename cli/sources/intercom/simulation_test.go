package intercom

import (
	"context"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dimmihq/dimmi/cli/engine"
	"github.com/dimmihq/dimmi/cli/format"
)

/*
The simulation runs a whole migration through the engine: a workspace
of thousands of records served the way Intercom's 2.18 description
says, into a strict fake dimmi. Along the way Intercom answers 429 and
503 at random, the workspace changes while it is read, the run is
killed and resumed after its file links expired, and a sync follows.
At the end dimmi must hold every record at its latest version, and no
record may have named one dimmi didn't hold yet.
*/

/*
simEngine builds the CLI's engine over the simulated Intercom, with the
rate limiter's waits cut short so the test runs fast.
*/
func simEngine(t *testing.T, srv *httptest.Server, dest *strictDimmi, dir string) *engine.Engine {
	t.Helper()

	lim := engine.NewLimiter(http.DefaultTransport, 0.3, 0)
	lim.MaxRetries = 30
	lim.Sleep = func(ctx context.Context, d time.Duration) error {
		return sleep(ctx, min(d, 2*time.Millisecond))
	}

	src := New(&Client{HTTP: &http.Client{Transport: lim}, Base: srv.URL, Token: "tok"}, "us", nil)

	store, err := engine.OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return &engine.Engine{Source: src, Dest: dest, Store: store, Name: "sim"}
}

func TestSimulatedMigrationOfAWholeWorkspace(t *testing.T) {
	if testing.Short() {
		t.Skip("a whole simulated migration")
	}

	wait := scrollWait
	scrollWait = 2 * scrollIdle

	t.Cleanup(func() { scrollWait = wait })

	w := newWorld(t)
	w.linkTTL = 400 * time.Millisecond
	srv := httptest.NewServer(w.serve(rand.New(rand.NewPCG(1, 2)))) //nolint:gosec // a fixed run, not a secret
	defer srv.Close()

	dir := t.TempDir()
	dest := newStrictDimmi(t)

	// First run: killed partway through the conversations, after the
	// workspace changed under it.
	var changed sync.Once

	w.onRead = func(n int32) {
		if n == 400 {
			changed.Do(w.change)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	dest.cancel, dest.crashAt = cancel, 60

	if err := simEngine(t, srv, dest, dir).Run(ctx, engine.RunOptions{}); err == nil {
		t.Fatal("the first run should have been killed")
	}

	// The file links read before the kill expire before the resume.
	time.Sleep(2 * w.linkTTL)

	dest.crashAt = 0
	w.linkTTL = time.Hour

	if err := simEngine(t, srv, dest, dir).Run(context.Background(), engine.RunOptions{}); err != nil {
		t.Fatalf("resume: %v", err)
	}

	// Then more changes, and a sync.
	w.onRead = nil
	w.change()

	if err := simEngine(t, srv, dest, dir).Run(context.Background(), engine.RunOptions{Sync: true}); err != nil {
		t.Fatalf("sync: %v", err)
	}

	checkWorld(t, w, dest)

	v, err := simEngine(t, srv, dest, dir).Verify(context.Background(), 50)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	if !v.OK {
		t.Errorf("verify found gaps: %+v", v)
	}
	t.Logf("requests %d, rate limited %d, failed %d, files downloaded %d, records sent in %d batches",
		w.requests.Load(), w.limited.Load(), w.failed.Load(), w.files.Load(), dest.upserts.Load())

	if w.limited.Load() == 0 || w.failed.Load() == 0 {
		t.Error("the run met no 429 or 503")
	}
}

/*
checkWorld compares what dimmi holds with the workspace as it ended.
*/
func checkWorld(t *testing.T, w *world, dest *strictDimmi) {
	t.Helper()

	dest.mu.Lock()
	defer dest.mu.Unlock()

	for key, w := range dest.waiting {
		var v any
		_ = json.Unmarshal(w.raw, &v)

		var gaps []string

		dest.missing(v, "", &gaps)
		t.Errorf("%s still waits for %v", key, gaps)
	}

	t.Logf("records put off along the way: %d", dest.deferred)

	notes, convs, tickets := 0, 0, 0
	for _, c := range w.contacts {
		notes += c.notes
	}

	for _, n := range w.coNotes {
		notes += n
	}

	for _, c := range w.convs {
		if c.ticket {
			tickets++
		} else {
			convs++
		}
	}

	want := map[format.Kind]int{ //nolint:exhaustive // attributes, snippets and workflows have no set count here
		// The removed teammates: one who wrote in conversations, one who
		// wrote notes; the article author nothing else names isn't one.
		format.KindTeammate: len(w.admins) + 2, format.KindTeam: len(w.teams), format.KindTag: len(w.tags),
		format.KindCompany: len(w.companies), format.KindContact: len(w.contacts) + 1,
		format.KindHelpCenter: 1, format.KindCollection: 12, format.KindArticle: w.articles,
		format.KindSavedReply: 130, format.KindSchedule: 1, format.KindTicketType: 1,
		format.KindContactNote: notes, format.KindConversation: convs, format.KindTicket: tickets,
	}

	for kind, n := range want {
		if got := len(dest.held[kind]); got != n {
			t.Errorf("%s: dimmi holds %d, Intercom has %d", kind, got, n)
		}
	}

	for _, c := range w.convs {
		checkConv(t, c, dest)
	}

	for _, c := range w.contacts {
		raw := dest.held[format.KindContact][c.id]

		var got struct {
			TagRefs []string `json:"tag_refs"`
		}

		_ = json.Unmarshal(raw, &got)

		if !slices.Equal(sorted(got.TagRefs), sorted(c.tags)) {
			t.Errorf("contact %s has tags %v, want %v", c.id, got.TagRefs, c.tags)
		}
	}
}

func checkConv(t *testing.T, c *simConv, dest *strictDimmi) {
	t.Helper()

	kind := format.KindConversation
	if c.ticket {
		kind = format.KindTicket
	}

	raw := dest.held[kind][c.id]
	if raw == nil {
		return
	}

	var got struct {
		Entries []struct {
			SourceID    string   `json:"source_id"`
			Attachments []string `json:"attachments"`
		} `json:"entries"`
	}

	_ = json.Unmarshal(raw, &got)

	ids := map[string]bool{}
	for _, e := range got.Entries {
		ids[strings.TrimSuffix(strings.TrimSuffix(e.SourceID, ":reply"), ":open")] = true
	}

	for _, p := range c.parts {
		if !ids[p.id] {
			t.Errorf("%s %s lacks part %s of %d", kind, c.id, p.id, len(c.parts))

			return
		}
	}
}

func sorted(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)

	return out
}
