package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dimmihq/dimmi/cmd/dimmi-cli/format"
)

/*
Engine runs migrations from a Source into a Destination, keeping its
checkpoint in Store.
*/
type Engine struct {
	Source Source
	Dest   Destination
	Store  *Store
	Report Reporter
	Name   string

	BatchSize   int
	BatchBytes  int
	Workers     int
	MaxFileSize int64
	Overlap     time.Duration
	Now         func() time.Time
}

/*
Defaults fills the settings left empty: batches of 100 records or 4MB,
4 parallel requests, files up to 100MB, and syncs that reread the 10
minutes before the last one, so nothing written while it ran is missed.
*/
func (e *Engine) defaults() {
	if e.BatchSize <= 0 {
		e.BatchSize = 100
	}

	if e.BatchBytes <= 0 {
		e.BatchBytes = 4 << 20
	}

	if e.Workers <= 0 {
		e.Workers = 4
	}

	if e.MaxFileSize <= 0 {
		e.MaxFileSize = 100 << 20
	}

	if e.Overlap <= 0 {
		e.Overlap = 10 * time.Minute
	}

	if e.Report == nil {
		e.Report = Discard{}
	}

	if e.Now == nil {
		e.Now = func() time.Time { return time.Now().UTC() }
	}
}

/*
Plan reads what the source holds without writing anything.
*/
func (e *Engine) Plan(ctx context.Context) (Inventory, error) {
	e.defaults()

	return e.Source.Discover(ctx)
}

/*
Begin loads the checkpoint, or starts the migration in dimmi and writes
a new one. A checkpoint for another source account is refused.
*/
func (e *Engine) Begin(ctx context.Context) (*State, error) {
	e.defaults()

	st, err := e.Store.Load()
	if err != nil {
		return nil, err
	}

	acct, err := e.Source.Account(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the %s account: %w", e.Source.Name(), err)
	}

	if st != nil {
		if st.Account.ID != acct.ID || st.Source != e.Source.Name() {
			return nil, fmt.Errorf("this checkpoint is for %s account %s, not %s", st.Source, st.Account.ID, acct.ID)
		}

		return st, nil
	}

	name := e.Name
	if name == "" {
		name = fmt.Sprintf("%s %s", e.Source.Name(), acct.Name)
	}

	mig, err := e.Dest.Start(ctx, Start{Source: e.Source.Name(), SourceAccount: acct.ID, Name: name})
	if err != nil {
		return nil, fmt.Errorf("start the migration in dimmi: %w", err)
	}

	st = &State{MigrationID: mig.ID, Source: e.Source.Name(), Account: acct, Kinds: map[format.Kind]*KindState{}}

	return st, e.Store.Save(st)
}

/*
RunOptions narrows a run. Skip leaves kinds out; Sync rereads kinds a
run already finished, from where the last read began.
*/
type RunOptions struct {
	Skip []format.Kind
	Only []format.Kind
	Sync bool
}

/*
Run imports every kind in order, resuming from the checkpoint. A sync
reads only what changed since the last read of each kind.
*/
func (e *Engine) Run(ctx context.Context, opts RunOptions) error {
	st, err := e.Begin(ctx)
	if err != nil {
		return err
	}

	for _, k := range opts.Skip {
		if !st.Skips(k) {
			st.Skip = append(st.Skip, k)
		}
	}

	e.Report.Report(Event{Event: "started", Message: fmt.Sprintf("Migration %s from %s account %s", st.MigrationID, st.Source, st.Account.ID),
		Data: map[string]any{"migration_id": st.MigrationID, "sync": opts.Sync}})

	for _, kind := range e.Source.Kinds() {
		if st.Skips(kind) || (len(opts.Only) > 0 && !slices.Contains(opts.Only, kind)) {
			continue
		}

		ks := st.Kind(kind)

		switch {
		case ks.Done && !opts.Sync:
			continue
		case ks.Done:
			ks.Done, ks.Cursor, ks.StartedAt = false, "", e.Now()
		case ks.StartedAt.IsZero():
			ks.StartedAt = e.Now()
		}

		if err := e.runKind(ctx, st, kind); err != nil {
			_ = e.Store.Save(st)

			return err
		}
	}

	if opts.Sync {
		st.SyncedAt = e.Now()
	}

	if err := e.Store.Save(st); err != nil {
		return err
	}

	e.Report.Report(Event{Event: "finished", Message: "Done.", Data: map[string]any{"migration_id": st.MigrationID}})

	return nil
}

/*
runKind reads one kind page by page, saving the cursor after each.
*/
func (e *Engine) runKind(ctx context.Context, st *State, kind format.Kind) error {
	ks := st.Kind(kind)
	counts := map[string]int{}

	opts := ReadOptions{Cursor: ks.Cursor}
	if !ks.ReadFrom.IsZero() {
		opts.Since = ks.ReadFrom.Add(-e.Overlap)
	}

	e.Report.Report(Event{Event: "kind_started", Kind: kind})

	err := e.Source.Read(ctx, kind, opts, func(page Page) error {
		if err := e.page(ctx, st, kind, page.Records, counts); err != nil {
			return err
		}

		ks.Cursor = page.Cursor
		if err := e.Store.Save(st); err != nil {
			return err
		}

		e.Report.Report(Event{Event: "progress", Kind: kind, Counts: clone(counts)})

		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", kind, err)
	}

	ks.Done, ks.Cursor, ks.ReadFrom = true, "", ks.StartedAt
	ks.Sent += counts["created"] + counts["updated"] + counts["unchanged"]
	ks.Skipped += counts["skipped"]
	ks.Failed += counts["failed"]

	e.Report.Report(Event{Event: "kind_finished", Kind: kind, Counts: clone(counts)})

	return e.Store.Save(st)
}

func clone(m map[string]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[k] = v
	}

	return out
}

/*
pending is a record waiting to be sent, with its fingerprint.
*/
type pending struct {
	rec  format.Record
	hash string
	size int
}

/*
page sends one page: records dimmi already holds unchanged are skipped,
files are uploaded, and the rest go in batches.
*/
func (e *Engine) page(ctx context.Context, st *State, kind format.Kind, records []format.Record, counts map[string]int) error {
	todo := map[format.Kind][]pending{}
	var order []format.Kind

	for _, rec := range records {
		if rec.Kind == "" {
			rec.Kind = kind
		}

		if !rec.Kind.Valid() || rank(rec.Kind) > rank(kind) || rec.SourceID == "" {
			return fmt.Errorf("source sent a %s record with id %q while reading %s", rec.Kind, rec.SourceID, kind)
		}

		key := "read"
		if rec.Kind != kind {
			key = "read_" + string(rec.Kind)
		}

		counts[key]++

		hash, err := Hash(rec)
		if err != nil {
			return err
		}

		if e.Store.Unchanged(rec.Kind, rec.SourceID, hash) {
			if rec.Kind == kind {
				counts["skipped"]++
			}

			continue
		}

		if _, ok := todo[rec.Kind]; !ok {
			order = append(order, rec.Kind)
		}

		todo[rec.Kind] = append(todo[rec.Kind], pending{rec: rec, hash: hash})
	}

	slices.SortFunc(order, func(a, b format.Kind) int { return rank(a) - rank(b) })

	for _, k := range order {
		if err := e.files(ctx, st, k, todo[k], counts); err != nil {
			return err
		}

		into := counts
		if k != kind {
			into = map[string]int{}
		}

		for i := range todo[k] {
			if err := linkFiles(&todo[k][i].rec, st.Files); err != nil {
				return err
			}
		}

		if err := e.send(ctx, st, k, todo[k], into); err != nil {
			return err
		}
	}

	return nil
}

/*
rank is a kind's place in sending order. A source may send records of
an earlier kind while reading a later one, such as a teammate only the
conversations they wrote in reveal; they go first.
*/
func rank(k format.Kind) int { return slices.Index(format.Kinds(), k) }

/*
files uploads every file the records name that dimmi doesn't have yet,
each once. A file that can't be read or is too big is reported and left
out; the record still goes, and dimmi shows the attachment as missing.
*/
func (e *Engine) files(ctx context.Context, st *State, kind format.Kind, todo []pending, counts map[string]int) error {
	if st.Files == nil {
		st.Files = map[string]string{}
	}

	var wanted []*format.File
	queued := map[string]bool{}

	for _, p := range todo {
		for _, f := range p.rec.Files {
			if f == nil || f.SourceID == "" || st.Files[f.SourceID] != "" || queued[f.SourceID] {
				continue
			}

			queued[f.SourceID] = true
			wanted = append(wanted, f)
		}
	}

	if len(wanted) == 0 {
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	var fatal error
	sem := make(chan struct{}, e.Workers)

	for _, f := range wanted {
		wg.Add(1)
		sem <- struct{}{}

		go func(f *format.File) {
			defer wg.Done()
			defer func() { <-sem }()

			id, err := e.upload(ctx, st.MigrationID, f)

			mu.Lock()
			defer mu.Unlock()

			var skip *skipFile
			switch {
			case errors.As(err, &skip):
				counts["files_skipped"]++
				e.Report.Report(Event{Event: "warning", Kind: kind, Code: "file_skipped", SourceID: f.SourceID, Message: skip.reason})
			case err != nil:
				if fatal == nil {
					fatal = err
					cancel()
				}
			default:
				counts["files"]++
				st.Files[f.SourceID] = id
			}
		}(f)
	}

	wg.Wait()

	return fatal
}

/*
skipFile is a file left out, with why.
*/
type skipFile struct{ reason string }

func (s *skipFile) Error() string { return s.reason }

/*
upload downloads one file to a private temporary file, then uploads it.
*/
func (e *Engine) upload(ctx context.Context, migration string, f *format.File) (string, error) {
	if f.Size > e.MaxFileSize {
		return "", &skipFile{fmt.Sprintf("a %d MB file is over the %d MB limit", f.Size>>20, e.MaxFileSize>>20)}
	}

	body, err := e.Source.Open(ctx, f)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		return "", &skipFile{fmt.Sprintf("a file could not be downloaded: %v", err)}
	}
	defer body.Close()

	tmp, err := os.CreateTemp(e.Store.Dir, "file-*")
	if err != nil {
		return "", err
	}

	defer os.Remove(tmp.Name())
	defer tmp.Close()

	n, err := io.Copy(tmp, io.LimitReader(body, e.MaxFileSize+1))
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		return "", &skipFile{fmt.Sprintf("a file download broke off: %v", err)}
	}

	if n > e.MaxFileSize {
		return "", &skipFile{fmt.Sprintf("a file is over the %d MB limit", e.MaxFileSize>>20)}
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	return e.Dest.Upload(ctx, migration, Upload{SourceID: f.SourceID, Name: f.Name, ContentType: f.ContentType, Size: n, Body: tmp})
}

/*
send splits the records into batches and sends them in parallel.
*/
func (e *Engine) send(ctx context.Context, st *State, kind format.Kind, todo []pending, counts map[string]int) error {
	var batches [][]pending
	var cur []pending
	size := 0

	for _, p := range todo {
		raw, err := json.Marshal(p.rec.Data)
		if err != nil {
			return err
		}

		p.size = len(raw)

		if len(cur) > 0 && (len(cur) >= e.BatchSize || size+p.size > e.BatchBytes) {
			batches = append(batches, cur)
			cur, size = nil, 0
		}

		cur = append(cur, p)
		size += p.size
	}

	if len(cur) > 0 {
		batches = append(batches, cur)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var fatal error
	sem := make(chan struct{}, e.Workers)

	for _, batch := range batches {
		wg.Add(1)
		sem <- struct{}{}

		go func(batch []pending) {
			defer wg.Done()
			defer func() { <-sem }()

			err := e.sendBatch(ctx, st, kind, batch, counts, &mu)
			if err != nil {
				mu.Lock()
				if fatal == nil {
					fatal = err
				}
				mu.Unlock()
			}
		}(batch)
	}

	wg.Wait()

	return fatal
}

/*
ErrTooLarge is what a Destination returns when a batch is too big; the
engine halves it and tries again.
*/
var ErrTooLarge = errors.New("batch too large")

func (e *Engine) sendBatch(ctx context.Context, st *State, kind format.Kind, batch []pending, counts map[string]int, mu *sync.Mutex) error {
	recs := make([]format.Record, len(batch))
	for i, p := range batch {
		recs[i] = p.rec
	}

	results, err := e.Dest.Upsert(ctx, st.MigrationID, kind, recs)
	if errors.Is(err, ErrTooLarge) && len(batch) > 1 {
		half := len(batch) / 2
		if err := e.sendBatch(ctx, st, kind, batch[:half], counts, mu); err != nil {
			return err
		}

		return e.sendBatch(ctx, st, kind, batch[half:], counts, mu)
	}

	if err != nil {
		return err
	}

	byID := make(map[string]Result, len(results))
	for _, r := range results {
		byID[r.SourceID] = r
	}

	mu.Lock()
	defer mu.Unlock()

	for _, p := range batch {
		r, ok := byID[p.rec.SourceID]
		if !ok {
			r = Result{SourceID: p.rec.SourceID, Status: StatusFailed, Error: &RecordError{Code: "no_result", Message: "dimmi returned no result for this record"}}
		}

		switch r.Status {
		case StatusCreated, StatusUpdated, StatusUnchanged, StatusDeferred:
			counts[r.Status]++

			if err := e.Store.Taken(kind, p.rec.SourceID, p.hash); err != nil {
				return err
			}

			st.Failures = dropFailure(st.Failures, kind, p.rec.SourceID)
		default:
			counts["failed"]++

			f := Failure{Kind: kind, SourceID: p.rec.SourceID, Code: "failed"}
			if r.Error != nil {
				f.Code, f.Message = r.Error.Code, r.Error.Message
			}

			st.Failures = append(dropFailure(st.Failures, kind, p.rec.SourceID), f)
			e.Report.Report(Event{Event: "record_failed", Kind: kind, SourceID: f.SourceID, Code: f.Code, Message: f.Message})
		}
	}

	return nil
}

func dropFailure(list []Failure, kind format.Kind, id string) []Failure {
	return slices.DeleteFunc(list, func(f Failure) bool { return f.Kind == kind && f.SourceID == id })
}

/*
KindCheck compares one kind between the source and dimmi. Missing lists
sampled source ids dimmi doesn't hold.
*/
type KindCheck struct {
	Kind    format.Kind `json:"kind"`
	Source  int         `json:"source"`
	Dimmi   int         `json:"dimmi"`
	Failed  int         `json:"failed"`
	Sampled int         `json:"sampled"`
	Missing []string    `json:"missing,omitempty"`
	OK      bool        `json:"ok"`
}

/*
Verification is the outcome of verify.
*/
type Verification struct {
	MigrationID string      `json:"migration_id"`
	Kinds       []KindCheck `json:"kinds"`
	OK          bool        `json:"ok"`
}

/*
Verify compares counts per kind between the source and dimmi, and checks
that a sample of each kind's records is in dimmi.
*/
func (e *Engine) Verify(ctx context.Context, sample int) (Verification, error) {
	e.defaults()

	st, err := e.Store.Load()
	if err != nil {
		return Verification{}, err
	}

	if st == nil {
		return Verification{}, errors.New("nothing to verify: run `dimmi migrate run` first")
	}

	inv, err := e.Source.Discover(ctx)
	if err != nil {
		return Verification{}, err
	}

	mig, err := e.Dest.Get(ctx, st.MigrationID)
	if err != nil {
		return Verification{}, err
	}

	out := Verification{MigrationID: st.MigrationID, OK: true}

	for _, kind := range e.Source.Kinds() {
		if st.Skips(kind) {
			continue
		}

		count, _ := inv.Count(kind)
		check := KindCheck{Kind: kind, Source: count.Count, Dimmi: mig.Counts[kind].Total(), Failed: mig.Counts[kind].Failed}

		ids, err := e.sampleIDs(ctx, kind, sample)
		if err != nil {
			return out, err
		}

		check.Sampled = len(ids)

		if len(ids) > 0 {
			found, err := e.Dest.Lookup(ctx, st.MigrationID, kind, ids)

			switch {
			case errors.Is(err, ErrNoLookup):
				check.Sampled = 0
			case err != nil:
				return out, err
			default:
				for _, id := range ids {
					if !found[id] {
						check.Missing = append(check.Missing, id)
					}
				}
			}
		}

		check.OK = len(check.Missing) == 0 && (count.Estimate || check.Dimmi >= check.Source)
		out.OK = out.OK && check.OK
		out.Kinds = append(out.Kinds, check)

		e.Report.Report(Event{Event: "verified", Kind: kind, Counts: map[string]int{
			"source": check.Source, "dimmi": check.Dimmi, "sampled": check.Sampled, "missing": len(check.Missing),
		}, Data: check})
	}

	return out, nil
}

/*
sampleIDs reads the first records of a kind.
*/
func (e *Engine) sampleIDs(ctx context.Context, kind format.Kind, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}

	var ids []string

	err := e.Source.Read(ctx, kind, ReadOptions{}, func(p Page) error {
		for _, r := range p.Records {
			ids = append(ids, r.SourceID)
			if len(ids) >= n {
				return ErrStop
			}
		}

		return nil
	})
	if err != nil && !errors.Is(err, ErrStop) {
		return nil, err
	}

	return ids, nil
}

/*
Status is the checkpoint with dimmi's own counts.
*/
type Status struct {
	State     *State     `json:"state"`
	Migration *Migration `json:"migration,omitempty"`
}

/*
Status reads the checkpoint and, when there is one, dimmi's counts.
*/
func (e *Engine) Status(ctx context.Context) (Status, error) {
	e.defaults()

	st, err := e.Store.Load()
	if err != nil || st == nil {
		return Status{State: st}, err
	}

	mig, err := e.Dest.Get(ctx, st.MigrationID)
	if err != nil {
		return Status{State: st}, err
	}

	return Status{State: st, Migration: &mig}, nil
}

/*
Rollback undoes everything the migration created in dimmi, then forgets
the checkpoint so the next run starts over.
*/
func (e *Engine) Rollback(ctx context.Context) (Migration, error) {
	e.defaults()

	st, err := e.Store.Load()
	if err != nil {
		return Migration{}, err
	}

	if st == nil {
		return Migration{}, errors.New("nothing to roll back: no migration has run from this machine")
	}

	mig, err := e.Dest.Rollback(ctx, st.MigrationID)
	if err != nil {
		return mig, err
	}

	return mig, e.Store.Reset()
}

/*
linkFiles points a record at the files dimmi holds: every file source id
the translator wrote becomes the id dimmi gave the upload, inside lists
and inside Markdown such as dimmi-file:<id>. A file that couldn't be
uploaded is dropped from lists, so the record still goes.
*/
func linkFiles(rec *format.Record, uploaded map[string]string) error {
	if len(rec.Files) == 0 {
		return nil
	}

	ids := map[string]string{}
	for _, f := range rec.Files {
		if f != nil && f.SourceID != "" {
			ids[f.SourceID] = uploaded[f.SourceID]
		}
	}

	raw, err := json.Marshal(rec.Data)
	if err != nil {
		return err
	}

	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return err
	}

	out, err := json.Marshal(swapFiles(v, ids))
	if err != nil {
		return err
	}

	rec.Data = json.RawMessage(out)

	return nil
}

func swapFiles(v any, ids map[string]string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			x[k] = swapFiles(child, ids)
		}

		return x
	case []any:
		out := x[:0]

		for _, child := range x {
			if s, ok := child.(string); ok {
				if id, known := ids[s]; known {
					if id != "" {
						out = append(out, id)
					}

					continue
				}
			}

			out = append(out, swapFiles(child, ids))
		}

		return out
	case string:
		if id, known := ids[x]; known {
			return id
		}

		for src, id := range ids {
			if id != "" && strings.Contains(x, src) {
				x = strings.ReplaceAll(x, src, id)
			}
		}

		return x
	}

	return v
}
