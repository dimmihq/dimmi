package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/dimmihq/dimmi/cli/format"
)

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
func (e *Engine) page(ctx context.Context, run kindRun, records []format.Record) error {
	st, kind, counts := run.st, run.kind, run.counts

	todo, order, err := e.collect(kind, records, counts)
	if err != nil {
		return err
	}

	for _, k := range order {
		if err := e.files(ctx, kindRun{st: st, kind: k, counts: counts}, todo[k]); err != nil {
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

		if err := e.send(ctx, kindRun{st: st, kind: k, counts: into}, todo[k]); err != nil {
			return err
		}
	}

	return nil
}

/*
collect groups a page's records to send by kind, in sending order,
counting what was read and leaving out what dimmi already holds.
*/
func (e *Engine) collect(
	kind format.Kind, records []format.Record, counts map[string]int,
) (map[format.Kind][]pending, []format.Kind, error) {
	todo := make(map[format.Kind][]pending)
	var order []format.Kind

	for _, rec := range records {
		p, send, err := e.check(kind, rec, counts)
		if err != nil {
			return nil, nil, err
		}

		if !send {
			continue
		}

		if _, ok := todo[p.rec.Kind]; !ok {
			order = append(order, p.rec.Kind)
		}

		todo[p.rec.Kind] = append(todo[p.rec.Kind], p)
	}

	slices.SortFunc(order, func(a, b format.Kind) int { return rank(a) - rank(b) })

	return todo, order, nil
}

/*
check counts one record read while reading the kind and fingerprints
it. It reports false for a record dimmi already holds unchanged.
*/
func (e *Engine) check(kind format.Kind, rec format.Record, counts map[string]int) (pending, bool, error) {
	if rec.Kind == "" {
		rec.Kind = kind
	}

	if !rec.Kind.Valid() || rank(rec.Kind) > rank(kind) || rec.SourceID == "" {
		return pending{}, false, fmt.Errorf(
			"source sent a %s record with id %q while reading %s", rec.Kind, rec.SourceID, kind,
		)
	}

	key := "read"
	if rec.Kind != kind {
		key = "read_" + string(rec.Kind)
	}

	counts[key]++

	hash, err := Hash(rec)
	if err != nil {
		return pending{}, false, err
	}

	if e.Store.Unchanged(rec.Kind, rec.SourceID, hash) {
		if rec.Kind == kind {
			counts["skipped"]++
		}

		return pending{}, false, nil
	}

	return pending{rec: rec, hash: hash}, true, nil
}

/*
rank is a kind's place in sending order. A source may send records of
an earlier kind while reading a later one, such as a teammate only the
conversations they wrote in reveal; they go first.
*/
func rank(k format.Kind) int { return slices.Index(format.Kinds(), k) }

/*
kindRun is what sending one kind's records works on: the checkpoint,
the kind and the counts it adds to.
*/
type kindRun struct {
	st     *State
	kind   format.Kind
	counts map[string]int
}

/*
batchRun is what every batch of one kind's send shares: the kind's run
and the lock that guards its counts and checkpoint.
*/
type batchRun struct {
	kindRun
	mu *sync.Mutex
}

/*
send splits the records into batches and sends them in parallel.
*/
func (e *Engine) send(ctx context.Context, kr kindRun, todo []pending) error {
	batches, err := e.batches(todo)
	if err != nil {
		return err
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	var fatal error
	sem := make(chan struct{}, e.Workers)
	run := batchRun{kindRun: kr, mu: &mu}

	for _, batch := range batches {
		wg.Add(1)
		sem <- struct{}{}

		go func(batch []pending) {
			defer wg.Done()
			defer func() { <-sem }()

			err := e.sendBatch(ctx, run, batch)
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
batches splits the records into batches of at most BatchSize records
and BatchBytes of data.
*/
func (e *Engine) batches(todo []pending) ([][]pending, error) {
	var batches [][]pending
	var cur []pending
	size := 0

	for _, p := range todo {
		raw, err := json.Marshal(p.rec.Data)
		if err != nil {
			return nil, err
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

	return batches, nil
}

/*
ErrTooLarge is what a Destination returns when a batch is too big; the
engine halves it and tries again.
*/
var ErrTooLarge = errors.New("batch too large")

/*
sendBatch sends one batch and records each record's outcome, halving
the batch when dimmi says it is too large.
*/
func (e *Engine) sendBatch(ctx context.Context, run batchRun, batch []pending) error {
	recs := make([]format.Record, len(batch))
	for i, p := range batch {
		recs[i] = p.rec
	}

	results, err := e.Dest.Upsert(ctx, run.st.MigrationID, run.kind, recs)
	if errors.Is(err, ErrTooLarge) && len(batch) > 1 {
		half := len(batch) / 2
		if err := e.sendBatch(ctx, run, batch[:half]); err != nil {
			return err
		}

		return e.sendBatch(ctx, run, batch[half:])
	}

	if err != nil {
		return err
	}

	byID := make(map[string]Result, len(results))
	for _, r := range results {
		byID[r.SourceID] = r
	}

	run.mu.Lock()
	defer run.mu.Unlock()

	for _, p := range batch {
		r, ok := byID[p.rec.SourceID]
		if !ok {
			r = Result{SourceID: p.rec.SourceID, Status: StatusFailed, Error: &RecordError{
				Code: "no_result", Message: "dimmi returned no result for this record",
			}}
		}

		if err := e.settle(run, p, r); err != nil {
			return err
		}
	}

	return nil
}

/*
settle records one record's outcome in the counts and the checkpoint.
The caller holds the run's lock.
*/
func (e *Engine) settle(run batchRun, p pending, r Result) error {
	st, kind := run.st, run.kind

	switch r.Status {
	case StatusCreated, StatusUpdated, StatusUnchanged, StatusDeferred:
		run.counts[r.Status]++

		if err := e.Store.Taken(kind, p.rec.SourceID, p.hash); err != nil {
			return err
		}

		if r.Status == StatusDeferred {
			st.wait(kind, p.rec.SourceID, r.WaitingFor)
		} else {
			st.wait(kind, p.rec.SourceID, nil)
		}

		st.Failures = dropFailure(st.Failures, kind, p.rec.SourceID)
	default:
		run.counts["failed"]++

		f := Failure{Kind: kind, SourceID: p.rec.SourceID, Code: "failed"}
		if r.Error != nil {
			f.Code, f.Message = r.Error.Code, r.Error.Message
		}

		st.Failures = append(dropFailure(st.Failures, kind, p.rec.SourceID), f)
		e.Report.Report(Event{
			Event: "record_failed", Kind: kind, SourceID: f.SourceID, Code: f.Code, Message: f.Message,
		})
	}

	return nil
}

func dropFailure(list []Failure, kind format.Kind, id string) []Failure {
	return slices.DeleteFunc(list, func(f Failure) bool { return f.Kind == kind && f.SourceID == id })
}
