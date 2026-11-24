/*
|--------------------------------------------------------------------------
| Checking Series Proposals
|--------------------------------------------------------------------------
|
| A series edit is applied to the working copy, or to the suggestion
| open on it, and checked as publishing checks it. A new series may
| leave out only what a fresh, empty one misses too, such as who enters.
|
*/

package operator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/series"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
seriesOp checks an edit of a series' working copy.
*/
func (r resolver) seriesOp(ctx context.Context, a automationInput) (Operation, error) {
	op := Operation{Kind: KindSeriesSet}
	id, err := r.e.IDs.Decode(string(models.PrefixSeries), strings.TrimSpace(a.ID))
	if err != nil {
		return op, fmt.Errorf("%w: there is no series %q; \"series_id\" is the id list_series shows (srs_...)",
			errBadArguments, a.ID)
	}
	if !r.s.m.Can("series:write") {
		return op, exceptions.ErrForbidden
	}
	row, err := r.e.Services.Series.Show(ctx, r.s.m, id)
	if err != nil {
		return op, err
	}
	if row.Status == series.StatusArchived {
		return op, fmt.Errorf("%w: the series %q is archived and can no longer change", errBadArguments, row.Name)
	}
	base, err := series.SpecOf(row)
	if err != nil {
		return op, err
	}
	if suggested, ok := r.s.suggestedSeries(id); ok {
		base = suggested
	}
	next, err := nextSeries(base, a)
	if err != nil {
		return op, err
	}
	raw := rawJSON(next)
	if bytes.Equal(raw, rawJSON(base)) {
		return op, fmt.Errorf("%w: the edit leaves the series exactly as it is; do not send it again", errBadArguments)
	}
	problems := series.Check(next, row.Subject)
	if added := freshProblems(series.Check(base, row.Subject), problems); len(added) > 0 {
		return op, refusedSeries(added, next)
	}
	op.Series, op.Definition = &id, raw
	op.Label = fmt.Sprintf("Edit the working copy of the series “%s”", row.Name)
	op.Note = "\nThe series would read:\n" + seriesOutline(next) + problemText(problems)
	return op, nil
}

/*
newSeriesOp checks a new draft series. Only what a fresh, empty series
misses too, such as who enters, may stay for the teammate to fill in.
*/
func (r resolver) newSeriesOp(_ context.Context, a automationInput) (Operation, error) {
	op := Operation{Kind: KindSeriesNew}
	if !r.s.m.Can("series:write") {
		return op, exceptions.ErrForbidden
	}
	name := strings.TrimSpace(a.Name)
	if name == "" || len([]rune(name)) > 120 {
		return op, fmt.Errorf("%w: give the series a name of 1 to 120 characters", errBadArguments)
	}
	if a.Subject != series.SubjectContact && a.Subject != series.SubjectCompany {
		return op, fmt.Errorf("%w: subject is contact or company", errBadArguments)
	}
	if len(a.Spec) == 0 {
		return op, fmt.Errorf("%w: give the whole definition as spec", errBadArguments)
	}
	spec, err := readSeries(a.Spec)
	if err != nil {
		return op, err
	}
	problems := series.Check(spec, a.Subject)
	if added := freshProblems(series.Check(series.NewSpec(), a.Subject), problems); len(added) > 0 {
		return op, refusedSeries(added, spec)
	}
	op.Definition = rawJSON(newSeriesDraft{Name: name, Subject: a.Subject, Spec: rawJSON(spec)})
	op.Label = fmt.Sprintf("Draft a new series “%s”", name)
	op.Note = "\nThe series would read:\n" + seriesOutline(spec) + problemText(problems)
	return op, nil
}

/*
nextSeries is the working copy after the proposal.
*/
func nextSeries(base series.Spec, a automationInput) (series.Spec, error) {
	switch {
	case len(a.Spec) > 0 && len(a.Series) > 0:
		return base, fmt.Errorf("%w: send either edits or a whole spec, not both", errBadArguments)
	case len(a.Spec) > 0:
		return readSeries(a.Spec)
	case len(a.Series) > 0:
		return applySeriesEdits(base, a.Series)
	default:
		return base, fmt.Errorf("%w: \"edits\" is missing; send the changes as edits", errBadArguments)
	}
}

/*
readSeries reads a whole series spec the model sent.
*/
func readSeries(raw json.RawMessage) (series.Spec, error) {
	var spec series.Spec
	if err := json.Unmarshal(raw, &spec); err != nil {
		return spec, fmt.Errorf("%w: the spec does not fit its shape: %s", errBadArguments, err.Error())
	}
	return series.Normalize(spec), nil
}

/*
freshProblems are the problems of after that before did not have.
*/
func freshProblems(before, after []series.Problem) []series.Problem {
	had := map[series.Problem]bool{}
	for _, p := range before {
		had[p] = true
	}
	var out []series.Problem
	for _, p := range after {
		if !had[p] {
			out = append(out, p)
		}
	}
	return out
}

/*
refusedSeries refuses an edit with every problem it adds.
*/
func refusedSeries(problems []series.Problem, spec series.Spec) error {
	return fmt.Errorf("%w: publishing would refuse this, so nothing was suggested. Fix each problem below in "+
		"the same call and send it again:\n%s\nThe series would have read:\n%s", errBadArguments,
		problemText(problems), seriesOutline(spec))
}

/*
suggestedSeries is a series' working copy as the open suggestion would
leave it.
*/
func (s *session) suggestedSeries(id ids.UUID) (series.Spec, bool) {
	if s.viewed == nil {
		return series.Spec{}, false
	}
	for _, op := range Operations(s.viewed.Operations) {
		if op.Kind == KindSeriesSet && op.Series != nil && *op.Series == id {
			spec, err := readSeries(op.Definition)
			return spec, err == nil
		}
	}
	return series.Spec{}, false
}
