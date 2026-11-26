package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-migrate/format"
)

/*
fakeSource serves records per kind in pages of two, and fails once at a
chosen page to test resume.
*/
type fakeSource struct {
	mu      sync.Mutex
	records map[format.Kind][]format.Record
	failAt  map[format.Kind]int
	opened  int
	sinces  map[format.Kind]time.Time
	cursors map[format.Kind][]string
}

func (s *fakeSource) Name() string { return "fake" }
func (s *fakeSource) Account(context.Context) (Account, error) {
	return Account{ID: "acct1", Name: "Acme"}, nil
}
func (s *fakeSource) Kinds() []format.Kind {
	return []format.Kind{format.KindTag, format.KindContact, format.KindConversation}
}

func (s *fakeSource) Discover(context.Context) (Inventory, error) {
	inv := Inventory{}
	for _, k := range s.Kinds() {
		inv.Kinds = append(inv.Kinds, KindCount{Kind: k, Count: len(s.records[k])})
	}

	return inv, nil
}

func (s *fakeSource) Read(_ context.Context, kind format.Kind, opts ReadOptions, emit func(Page) error) error {
	s.mu.Lock()
	if s.sinces == nil {
		s.sinces = map[format.Kind]time.Time{}
		s.cursors = map[format.Kind][]string{}
	}
	s.sinces[kind] = opts.Since
	s.cursors[kind] = append(s.cursors[kind], opts.Cursor)
	s.mu.Unlock()

	recs := s.records[kind]
	start := 0
	if opts.Cursor != "" {
		fmt.Sscanf(opts.Cursor, "%d", &start)
	}

	for i := start; i < len(recs); i += 2 {
		if at, ok := s.failAt[kind]; ok && at == i {
			delete(s.failAt, kind)

			return errors.New("source went away")
		}

		end := min(i+2, len(recs))
		cursor := ""
		if end < len(recs) {
			cursor = fmt.Sprint(end)
		}

		if err := emit(Page{Records: recs[i:end], Cursor: cursor}); err != nil {
			return err
		}
	}

	return nil
}

func (s *fakeSource) Open(context.Context, *format.File) (io.ReadCloser, error) {
	s.mu.Lock()
	s.opened++
	s.mu.Unlock()

	return io.NopCloser(strings.NewReader("file bytes")), nil
}

/*
fakeDest keeps records by kind and source id, like the server does.
*/
type fakeDest struct {
	mu        sync.Mutex
	held      map[format.Kind]map[string]string
	files     map[string]bool
	uploads   int
	calls     int
	refuse    map[string]bool
	maxBatch  int
	rolled    bool
	startedAs Start
}

func newDest() *fakeDest { return &fakeDest{held: map[format.Kind]map[string]string{}} }

func (d *fakeDest) Start(_ context.Context, s Start) (Migration, error) {
	d.startedAs = s

	return Migration{ID: "mig_1"}, nil
}

func (d *fakeDest) Get(context.Context, string) (Migration, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	m := Migration{ID: "mig_1", Counts: map[format.Kind]Counts{}}
	for k, held := range d.held {
		m.Counts[k] = Counts{Created: len(held)}
	}

	return m, nil
}

func (d *fakeDest) Upsert(_ context.Context, _ string, kind format.Kind, recs []format.Record) ([]Result, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.calls++

	if d.maxBatch > 0 && len(recs) > d.maxBatch {
		return nil, ErrTooLarge
	}

	if d.held[kind] == nil {
		d.held[kind] = map[string]string{}
	}

	var out []Result
	for _, r := range recs {
		if d.refuse[r.SourceID] {
			out = append(out, Result{SourceID: r.SourceID, Status: StatusFailed, Error: &RecordError{Code: "invalid", Message: "refused"}})

			continue
		}

		for _, f := range r.Files {
			if !d.files[f.SourceID] {
				return nil, fmt.Errorf("record %s sent before its file was uploaded", r.SourceID)
			}
		}

		status := StatusCreated
		if _, ok := d.held[kind][r.SourceID]; ok {
			status = StatusUpdated
		}

		d.held[kind][r.SourceID] = "x"
		out = append(out, Result{SourceID: r.SourceID, Status: status, ID: "id_" + r.SourceID})
	}

	return out, nil
}

func (d *fakeDest) Upload(_ context.Context, _ string, u Upload) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	raw, _ := io.ReadAll(u.Body)
	if string(raw) != "file bytes" {
		return "", errors.New("upload body changed")
	}

	d.uploads++
	if d.files == nil {
		d.files = map[string]bool{}
	}
	d.files[u.SourceID] = true

	return fmt.Sprintf("file_%d", d.uploads), nil
}

func (d *fakeDest) Lookup(_ context.Context, _ string, kind format.Kind, ids []string) (map[string]bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := map[string]bool{}
	for _, id := range ids {
		_, out[id] = d.held[kind][id]
	}

	return out, nil
}

func (d *fakeDest) Rollback(context.Context, string) (Migration, error) {
	d.rolled = true
	d.held = map[format.Kind]map[string]string{}

	return Migration{ID: "mig_1", Status: "rolled_back"}, nil
}

func tags(n int) []format.Record {
	var out []format.Record
	for i := range n {
		id := fmt.Sprint("t", i)
		out = append(out, format.NewRecord(format.KindTag, id, &format.Tag{Base: format.Base{SourceID: id}, Name: fmt.Sprint("tag ", i)}))
	}

	return out
}

func conversations() []format.Record {
	a := &format.File{SourceID: format.FileID("https://files.example.com/a.png?sig=1", "a.png", 10), Name: "a.png", Source: "https://files.example.com/a.png?sig=1"}
	b := &format.File{SourceID: format.FileID("https://files.example.com/b.pdf", "b.pdf", 10), Name: "b.pdf", Source: "https://files.example.com/b.pdf"}

	return []format.Record{
		format.NewRecord(format.KindConversation, "c1", &format.Conversation{Base: format.Base{SourceID: "c1"}, Entries: []format.Entry{
			{SourceID: "p1", Type: format.EntryMessage, Attachments: []string{a.SourceID}},
		}}, a),
		format.NewRecord(format.KindConversation, "c2", &format.Conversation{Base: format.Base{SourceID: "c2"}, Entries: []format.Entry{
			{SourceID: "p2", Type: format.EntryReply, Attachments: []string{a.SourceID}},
			{SourceID: "p3", Type: format.EntryNote, Attachments: []string{b.SourceID}},
		}}, a, b),
	}
}

func newEngine(t *testing.T, src *fakeSource, dest *fakeDest) *Engine {
	t.Helper()

	store, err := OpenStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = store.Close() })

	return &Engine{Source: src, Dest: dest, Store: store, BatchSize: 3}
}

func TestRunImportsInOrderAndRerunSendsNothing(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{
		format.KindTag: tags(7), format.KindConversation: conversations(),
	}}
	dest := newDest()
	e := newEngine(t, src, dest)

	if err := e.Run(context.Background(), RunOptions{}); err != nil {
		t.Fatal(err)
	}

	if len(dest.held[format.KindTag]) != 7 || len(dest.held[format.KindConversation]) != 2 {
		t.Fatalf("held %v", dest.held)
	}

	if dest.uploads != 2 {
		t.Fatalf("uploaded %d files, want 2: one per distinct file", dest.uploads)
	}

	calls := dest.calls

	if err := e.Run(context.Background(), RunOptions{Sync: true}); err != nil {
		t.Fatal(err)
	}

	if dest.calls != calls || dest.uploads != 2 {
		t.Fatalf("a sync of unchanged records sent %d batches and %d files", dest.calls-calls, dest.uploads-2)
	}
}

func TestRunResumesFromTheCheckpoint(t *testing.T) {
	src := &fakeSource{
		records: map[format.Kind][]format.Record{format.KindTag: tags(6)},
		failAt:  map[format.Kind]int{format.KindTag: 4},
	}
	dest := newDest()
	e := newEngine(t, src, dest)

	if err := e.Run(context.Background(), RunOptions{}); err == nil {
		t.Fatal("expected the run to stop")
	}

	if len(dest.held[format.KindTag]) != 4 {
		t.Fatalf("held %d before the failure", len(dest.held[format.KindTag]))
	}

	if err := e.Run(context.Background(), RunOptions{}); err != nil {
		t.Fatal(err)
	}

	got := src.cursors[format.KindTag]
	if len(got) != 2 || got[1] != "4" {
		t.Fatalf("cursors %q, want the second read to resume at 4", got)
	}

	if len(dest.held[format.KindTag]) != 6 {
		t.Fatalf("held %d", len(dest.held[format.KindTag]))
	}
}

func TestSyncReadsFromTheLastRead(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{format.KindTag: tags(2)}}
	e := newEngine(t, src, newDest())

	first := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	e.Now = func() time.Time { return first }

	if err := e.Run(context.Background(), RunOptions{}); err != nil {
		t.Fatal(err)
	}

	if !src.sinces[format.KindTag].IsZero() {
		t.Fatal("the first run must read everything")
	}

	e.Now = func() time.Time { return first.Add(time.Hour) }

	if err := e.Run(context.Background(), RunOptions{Sync: true}); err != nil {
		t.Fatal(err)
	}

	if want := first.Add(-10 * time.Minute); !src.sinces[format.KindTag].Equal(want) {
		t.Fatalf("sync read from %v, want %v", src.sinces[format.KindTag], want)
	}
}

func TestChangedRecordsAreSentAgain(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{format.KindTag: tags(3)}}
	dest := newDest()
	e := newEngine(t, src, dest)

	if err := e.Run(context.Background(), RunOptions{}); err != nil {
		t.Fatal(err)
	}

	src.records[format.KindTag][1] = format.NewRecord(format.KindTag, "t1", &format.Tag{Base: format.Base{SourceID: "t1"}, Name: "renamed"})
	calls := dest.calls

	if err := e.Run(context.Background(), RunOptions{Sync: true}); err != nil {
		t.Fatal(err)
	}

	if dest.calls != calls+1 {
		t.Fatalf("sent %d batches, want 1", dest.calls-calls)
	}
}

func TestFailedRecordsAreKeptAndRetried(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{format.KindTag: tags(3)}}
	dest := newDest()
	dest.refuse = map[string]bool{"t2": true}
	e := newEngine(t, src, dest)

	if err := e.Run(context.Background(), RunOptions{}); err != nil {
		t.Fatal(err)
	}

	st, _ := e.Store.Load()
	if len(st.Failures) != 1 || st.Failures[0].SourceID != "t2" || st.Kinds[format.KindTag].Failed != 1 {
		t.Fatalf("failures %+v", st.Failures)
	}

	dest.refuse = nil

	if err := e.Run(context.Background(), RunOptions{Sync: true}); err != nil {
		t.Fatal(err)
	}

	st, _ = e.Store.Load()
	if len(st.Failures) != 0 || len(dest.held[format.KindTag]) != 3 {
		t.Fatalf("failures %+v held %d", st.Failures, len(dest.held[format.KindTag]))
	}
}

func TestTooLargeBatchesAreSplit(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{format.KindTag: tags(2)}}
	dest := newDest()
	dest.maxBatch = 1
	e := newEngine(t, src, dest)

	if err := e.Run(context.Background(), RunOptions{}); err != nil {
		t.Fatal(err)
	}

	if len(dest.held[format.KindTag]) != 2 {
		t.Fatalf("held %d", len(dest.held[format.KindTag]))
	}
}

func TestSkippedKindsAreLeftOut(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{format.KindTag: tags(2), format.KindConversation: conversations()}}
	dest := newDest()
	e := newEngine(t, src, dest)

	if err := e.Run(context.Background(), RunOptions{Skip: []format.Kind{format.KindConversation}}); err != nil {
		t.Fatal(err)
	}

	if len(dest.held[format.KindConversation]) != 0 {
		t.Fatal("a skipped kind was imported")
	}
}

func TestVerifyAndRollback(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{format.KindTag: tags(3)}}
	dest := newDest()
	e := newEngine(t, src, dest)

	if err := e.Run(context.Background(), RunOptions{}); err != nil {
		t.Fatal(err)
	}

	v, err := e.Verify(context.Background(), 50)
	if err != nil || !v.OK {
		t.Fatalf("verify %+v %v", v, err)
	}

	delete(dest.held[format.KindTag], "t0")

	v, err = e.Verify(context.Background(), 50)
	if err != nil || v.OK || v.Kinds[0].Missing[0] != "t0" {
		t.Fatalf("verify after a loss %+v %v", v, err)
	}

	if _, err := e.Rollback(context.Background()); err != nil || !dest.rolled {
		t.Fatal(err)
	}

	if st, _ := e.Store.Load(); st != nil {
		t.Fatal("rollback kept the checkpoint")
	}
}

func TestACheckpointForAnotherAccountIsRefused(t *testing.T) {
	src := &fakeSource{records: map[format.Kind][]format.Record{}}
	e := newEngine(t, src, newDest())

	if err := e.Store.Save(&State{MigrationID: "m", Source: "fake", Account: Account{ID: "other"}}); err != nil {
		t.Fatal(err)
	}

	if err := e.Run(context.Background(), RunOptions{}); err == nil || !strings.Contains(err.Error(), "other") {
		t.Fatalf("err %v", err)
	}
}

func TestFileIDIgnoresSignedQueryStrings(t *testing.T) {
	if format.FileID("https://x/a.png?sig=1", "a.png", 3) != format.FileID("https://x/a.png?sig=2", "a.png", 3) {
		t.Fatal("a new signature changed the file id")
	}

	if format.FileID("https://x/a.png", "a.png", 3) == format.FileID("https://x/b.png", "a.png", 3) {
		t.Fatal("two files share an id")
	}
}
