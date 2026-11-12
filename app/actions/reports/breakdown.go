/*
|--------------------------------------------------------------------------
| Breakdowns
|--------------------------------------------------------------------------
|
| A breakdown is a table of the same figures split by a group: channel,
| inbox, team, teammate or priority, largest first. Each row narrows the
| report to its group.
|
*/

package reports

import (
	"cmp"
	"context"
	"slices"
)

/*
breakdown describes a table: its key and title, the group, the
columns, how to read the rows and which values fill them.
*/
type breakdown struct {
	key, title string
	group      string
	drill      string
	columns    []Column
	read       groupRead
	pick       func(values) []*float64
}

/*
groupRead reads figures split by a group, or a single row when the
group is empty.
*/
type groupRead func(ctx context.Context, group string) ([]values, error)

/*
table builds the breakdown, rows sorted by their first value.
*/
func (b breakdown) table(ctx context.Context, n names) (Chart, error) {
	rows, err := b.read(ctx, b.group)
	if err != nil {
		return Chart{}, err
	}

	out := Chart{Key: b.key, Title: b.title, Kind: ChartTable, Axis: AxisCategory, Drill: b.drill, Columns: b.columns}
	for _, r := range rows {
		out.Rows = append(out.Rows, n.row(b.group, r.key, b.pick(r)))
	}

	slices.SortStableFunc(out.Rows, func(a, c Row) int { return cmp.Compare(first(c), first(a)) })

	return out, nil
}

/*
first returns a row's first value, 0 for none.
*/
func first(r Row) float64 {
	if len(r.Values) == 0 || r.Values[0] == nil {
		return 0
	}

	return *r.Values[0]
}

/*
only returns the single row of grouped values, empty ones when none.
*/
func only(rows []values, n int) values {
	if len(rows) == 0 {
		return values{nums: make([]*float64, n)}
	}

	return rows[0]
}

/*
durationColumns are the columns of a duration split by group.
*/
func durationColumns(label string) []Column {
	return []Column{
		{Key: "count", Label: label, Unit: UnitCount},
		{Key: "median", Label: "Median", Unit: UnitSeconds},
		{Key: "p90", Label: "90th percentile", Unit: UnitSeconds},
	}
}

/*
durationValues picks a duration's count, median and 90th percentile.
*/
func durationValues(i int) func(values) []*float64 {
	return func(v values) []*float64 { return []*float64{v.at(i), v.at(i + 1), v.at(i + 2)} }
}
