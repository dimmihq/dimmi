package engine

import (
	"context"
	"errors"

	"github.com/dimmihq/dimmi/cli/format"
)

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
	run := verifyRun{migrationID: st.MigrationID, inv: inv, mig: mig, sample: sample}

	for _, kind := range e.Source.Kinds() {
		if st.Skips(kind) {
			continue
		}

		check, err := e.checkKind(ctx, run, kind)
		if err != nil {
			return out, err
		}

		out.OK = out.OK && check.OK
		out.Kinds = append(out.Kinds, check)

		e.Report.Report(Event{Event: "verified", Kind: kind, Counts: map[string]int{
			"source": check.Source, "dimmi": check.Dimmi, "sampled": check.Sampled, "missing": len(check.Missing),
		}, Data: check})
	}

	return out, nil
}

/*
verifyRun is what verify compares each kind against: what the source
holds, what dimmi holds, and how many records to sample.
*/
type verifyRun struct {
	migrationID string
	inv         Inventory
	mig         Migration
	sample      int
}

/*
checkKind compares one kind's counts and looks a sample of its records
up in dimmi.
*/
func (e *Engine) checkKind(ctx context.Context, run verifyRun, kind format.Kind) (KindCheck, error) {
	count, _ := run.inv.Count(kind)
	counts := run.mig.Counts[kind]
	check := KindCheck{Kind: kind, Source: count.Count, Dimmi: counts.Total(), Failed: counts.Failed}

	ids, err := e.sampleIDs(ctx, kind, run.sample)
	if err != nil {
		return check, err
	}

	check.Sampled = len(ids)

	if len(ids) > 0 {
		found, err := e.Dest.Lookup(ctx, run.migrationID, kind, ids)

		switch {
		case errors.Is(err, ErrNoLookup):
			check.Sampled = 0
		case err != nil:
			return check, err
		default:
			check.Missing = missing(ids, found)
		}
	}

	check.OK = len(check.Missing) == 0 && (count.Estimate || check.Dimmi >= check.Source)

	return check, nil
}

/*
missing lists the ids dimmi didn't find.
*/
func missing(ids []string, found map[string]bool) []string {
	var out []string

	for _, id := range ids {
		if !found[id] {
			out = append(out, id)
		}
	}

	return out
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
			// A read gives records of earlier kinds too, such as the
			// teammates conversations reveal; they aren't this kind's.
			if r.Kind != "" && r.Kind != kind {
				continue
			}

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
