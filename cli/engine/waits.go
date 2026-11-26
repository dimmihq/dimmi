package engine

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dimmihq/dimmi/cli/format"
)

/*
settleWaits sends the records dimmi still waits on. Records the source
never listed, such as contacts its lists leave out, are fetched by id
when the source can; records waiting on what was sent since are
resolved by dimmi as it arrives. What still waits after a few rounds is
reported, by kind, never by content.
*/
func (e *Engine) settleWaits(ctx context.Context, st *State) error {
	fetcher, canFetch := e.Source.(Fetcher)

	for round := 0; round < 3 && canFetch; round++ {
		need := e.unsent(st)
		if len(need) == 0 {
			break
		}

		for _, kind := range sortedKinds(need) {
			recs, err := fetcher.Fetch(ctx, kind, need[kind])
			if err != nil {
				return fmt.Errorf("fetch the %s dimmi waits on: %w", Label(kind), err)
			}

			if len(recs) == 0 {
				continue
			}

			counts := map[string]int{}
			if err := e.page(ctx, kindRun{st: st, kind: kind, counts: counts}, recs); err != nil {
				return err
			}
		}
	}

	e.reportWaits(st)

	return nil
}

/*
unsent lists, by kind, the records waited on that were never sent.
*/
func (e *Engine) unsent(st *State) map[format.Kind][]string {
	need := make(map[format.Kind][]string)

	for _, refs := range st.Waiting {
		for _, r := range refs {
			if !r.Kind.Valid() || e.Store.Has(r.Kind, r.SourceID) || slices.Contains(need[r.Kind], r.SourceID) {
				continue
			}

			need[r.Kind] = append(need[r.Kind], r.SourceID)
		}
	}

	for _, ids := range need {
		slices.Sort(ids)
	}

	return need
}

/*
reportWaits forgets the waits whose records were all sent, since dimmi
applies those records as they arrive, and reports the rest.
*/
func (e *Engine) reportWaits(st *State) {
	waiting := make(map[format.Kind]int)
	on := make(map[format.Kind]int)

	for key, refs := range st.Waiting {
		left := slices.DeleteFunc(slices.Clone(refs), func(r Ref) bool { return e.Store.Has(r.Kind, r.SourceID) })
		if len(left) == 0 {
			delete(st.Waiting, key)

			continue
		}

		kind, _, _ := strings.Cut(key, "\t")
		waiting[format.Kind(kind)]++

		for _, r := range left {
			on[r.Kind]++
		}
	}

	for _, kind := range sortedKinds(waiting) {
		e.Report.Report(Event{
			Event: "waiting", Kind: kind,
			Message: fmt.Sprintf("%d %s wait for records the source doesn't have: %s",
				waiting[kind], Label(kind), describe(on)),
			Data: map[string]any{"waiting": waiting[kind], "on": on},
		})
	}
}

func describe(on map[format.Kind]int) string {
	var parts []string
	for _, kind := range sortedKinds(on) {
		parts = append(parts, fmt.Sprintf("%d %s", on[kind], Label(kind)))
	}

	return strings.Join(parts, ", ")
}

func sortedKinds[V any](m map[format.Kind]V) []format.Kind {
	out := make([]format.Kind, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	slices.SortFunc(out, func(a, b format.Kind) int { return rank(a) - rank(b) })

	return out
}
