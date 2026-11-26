package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/cli/format"
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

	st = &State{MigrationID: mig.ID, Source: e.Source.Name(), Account: acct, Kinds: make(map[format.Kind]*KindState)}

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

	e.Report.Report(Event{
		Event:   "started",
		Message: fmt.Sprintf("Migration %s from %s account %s", st.MigrationID, st.Source, st.Account.ID),
		Data:    map[string]any{"migration_id": st.MigrationID, "sync": opts.Sync},
	})

	for _, kind := range e.Source.Kinds() {
		if !e.ready(st, kind, opts) {
			continue
		}

		if err := e.runKind(ctx, st, kind); err != nil {
			_ = e.Store.Save(st)

			return err
		}
	}

	if err := e.settleWaits(ctx, st); err != nil {
		_ = e.Store.Save(st)

		return err
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
ready reports whether the run reads the kind, and marks when its read
began: a kind the plan skips or the run leaves out is not read, and a
finished kind is read again only by a sync.
*/
func (e *Engine) ready(st *State, kind format.Kind, opts RunOptions) bool {
	if st.Skips(kind) || (len(opts.Only) > 0 && !slices.Contains(opts.Only, kind)) {
		return false
	}

	ks := st.Kind(kind)

	switch {
	case ks.Done && !opts.Sync:
		return false
	case ks.Done:
		ks.Done, ks.Cursor, ks.StartedAt = false, "", e.Now()
	case ks.StartedAt.IsZero():
		ks.StartedAt = e.Now()
	}

	return true
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
		if err := e.page(ctx, kindRun{st: st, kind: kind, counts: counts}, page.Records); err != nil {
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
	maps.Copy(out, m)

	return out
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
