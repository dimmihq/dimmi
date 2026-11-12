/*
|--------------------------------------------------------------------------
| Report Registry
|--------------------------------------------------------------------------
|
| Every report is a source registered under its key: conversations'
| reports here, and any area with numbers of its own, such as tickets
| and their service levels, registers beside them. A source answers its
| headline numbers and its charts for a query, inside the workspace's
| transaction; the registry runs them, the numbers a second time for the
| period compared with.
|
*/

package reports

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/exceptions"
)

/*
Description says what a report is: its key, title and one line about
it, where it sorts among the others, whether it shows the present
instead of a range, and the permission reading it needs besides
conversations:read, empty for none.
*/
type Description struct {
	Key        string
	Title      string
	Summary    string
	Order      int
	Live       bool
	Permission string
}

/*
Source is one report.
*/
type Source interface {
	/*
		Describe says what the report is.
	*/
	Describe() Description
	/*
		Metrics returns the headline numbers for the query.
	*/
	Metrics(ctx context.Context, tx *database.Tx, q Query) ([]Metric, error)
	/*
		Charts returns the charts for the query.
	*/
	Charts(ctx context.Context, tx *database.Tx, q Query) ([]Chart, error)
}

/*
Reports runs the registered reports.
*/
type Reports struct {
	DB    *database.DB
	Clock clock.Clock

	mu      sync.RWMutex
	sources map[string]Source
}

/*
Register adds a report, replacing one under the same key.
*/
func (r *Reports) Register(s Source) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.sources == nil {
		r.sources = map[string]Source{}
	}

	r.sources[s.Describe().Key] = s
}

/*
List returns the reports the member may read, in order.
*/
func (r *Reports) List(m authz.Membership) []Description {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Description, 0, len(r.sources))
	for _, s := range r.sources {
		if d := s.Describe(); d.Permission == "" || m.Can(d.Permission) {
			out = append(out, d)
		}
	}

	slices.SortFunc(out, func(a, b Description) int { return a.Order - b.Order })

	return out
}

/*
source returns the report under the key the member may read.
*/
func (r *Reports) source(m authz.Membership, key string) (Source, error) {
	r.mu.RLock()
	s, ok := r.sources[key]
	r.mu.RUnlock()

	if !ok {
		return nil, exceptions.ErrNotFound
	}

	if p := s.Describe().Permission; p != "" && !m.Can(p) {
		return nil, exceptions.ErrForbidden
	}

	return s, nil
}

/*
Run answers the report for the member and filters.
*/
func (r *Reports) Run(ctx context.Context, m authz.Membership, key string, f Filters) (Report, error) {
	src, err := r.source(m, key)
	if err != nil {
		return Report{}, err
	}

	q, err := NewQuery(m, f, r.Clock.Now())
	if err != nil {
		return Report{}, err
	}

	d := src.Describe()
	out := Report{
		Key: d.Key, Title: d.Title, Start: q.Start, End: q.End, Timezone: q.Location.String(),
		BusinessHours: f.BusinessHours, Live: d.Live,
	}

	err = r.runParts(ctx, partRun{m: m, src: src, q: q}, &out)

	return out, err
}

/*
partRun is what a report's parts are read for: the member, the source
and the query.
*/
type partRun struct {
	m   authz.Membership
	src Source
	q   Query
}

/*
runParts reads the numbers, the numbers of the period before and the
charts at the same time, each in its own read, so a report takes as
long as its slowest part rather than all of them.
*/
func (r *Reports) runParts(ctx context.Context, run partRun, out *Report) error {
	m, src, q := run.m, run.src, run.q

	var (
		metrics, before []Metric
		wg              sync.WaitGroup
		errs            [3]error
	)

	compare := q.Filters.Compare && !src.Describe().Live
	prev := q.Previous()

	wg.Go(func() {
		errs[0] = r.read(ctx, m, func(ctx context.Context, tx *database.Tx) (err error) {
			metrics, err = src.Metrics(ctx, tx, q)

			return err
		})
	})
	wg.Go(func() {
		if compare {
			errs[1] = r.read(ctx, m, func(ctx context.Context, tx *database.Tx) (err error) {
				before, err = src.Metrics(ctx, tx, prev)

				return err
			})
		}
	})
	wg.Go(func() {
		out.Charts, errs[2] = r.charts(ctx, m, src, q)
	})
	wg.Wait()

	if err := errors.Join(errs[:]...); err != nil {
		return fmt.Errorf("report: %w", err)
	}

	if compare {
		mergePrevious(metrics, before)
		out.PreviousStart, out.PreviousEnd = &prev.Start, &prev.End
	}

	out.Metrics = metrics

	return nil
}

/*
fill runs the source's numbers, the previous period's and its charts
into the report.
*/
func (out *Report) fill(ctx context.Context, tx *database.Tx, src Source, q Query) error {
	metrics, err := src.Metrics(ctx, tx, q)
	if err != nil {
		return fmt.Errorf("report numbers: %w", err)
	}

	if q.Filters.Compare && !src.Describe().Live {
		prev := q.Previous()
		before, err := src.Metrics(ctx, tx, prev)
		if err != nil {
			return fmt.Errorf("report numbers before: %w", err)
		}

		mergePrevious(metrics, before)
		out.PreviousStart, out.PreviousEnd = &prev.Start, &prev.End
	}

	out.Metrics = metrics
	out.Charts, err = src.Charts(ctx, tx, q)
	if err != nil {
		return fmt.Errorf("report charts: %w", err)
	}

	return nil
}

/*
charts reads the source's charts, its parts at the same time when it
has parts.
*/
func (r *Reports) charts(ctx context.Context, m authz.Membership, src Source, q Query) ([]Chart, error) {
	parted, ok := src.(PartedSource)
	if !ok {
		var out []Chart

		err := r.read(ctx, m, func(ctx context.Context, tx *database.Tx) (err error) {
			out, err = src.Charts(ctx, tx, q)

			return err
		})

		return out, err
	}

	parts := parted.ChartParts()
	results := make([][]Chart, len(parts))
	errs := make([]error, len(parts))

	var wg sync.WaitGroup
	for i, part := range parts {
		wg.Go(func() {
			errs[i] = r.read(ctx, m, func(ctx context.Context, tx *database.Tx) (err error) {
				results[i], err = part(ctx, tx, q)

				return err
			})
		})
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	var out []Chart
	for _, charts := range results {
		out = append(out, charts...)
	}

	return out, nil
}

/*
mergePrevious sets each number's value in the period before.
*/
func mergePrevious(now, before []Metric) {
	byKey := make(map[string]*float64, len(before))
	for _, m := range before {
		byKey[m.Key] = m.Value
	}

	for i := range now {
		now[i].Previous = byKey[now[i].Key]
	}
}

/*
read runs reads inside the member's workspace.
*/
func (r *Reports) read(ctx context.Context, m authz.Membership, fn func(context.Context, *database.Tx) error) error {
	if !m.Can("conversations:read") {
		return exceptions.ErrForbidden
	}

	return r.DB.Transaction(tenancy.With(ctx, m.Workspace.ID.String()), fn)
}

/*
Describe returns what the report under the key is, refusing one the
member may not read.
*/
func (r *Reports) Describe(m authz.Membership, key string) (Description, error) {
	s, err := r.source(m, key)
	if err != nil {
		return Description{}, err
	}

	return s.Describe(), nil
}
