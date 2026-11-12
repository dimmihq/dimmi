/*
|--------------------------------------------------------------------------
| Report Chart Resources
|--------------------------------------------------------------------------
|
| A chart reads as series of points along days ("2006-01-02"), hours of
| the week or groups, or as a table of rows under columns. A point's y
| is null for no data. A row's filter is the query filter that narrows
| the report to it, such as {"member": "mem_..."}.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/reports"

/*
ReportChart is one chart of a report.
*/
type ReportChart struct {
	Key     string         `json:"key"`
	Title   string         `json:"title"`
	Kind    string         `json:"kind"    enum:"line,bar,stacked_bar,heatmap,histogram,table"`
	Unit    string         `json:"unit"`
	Axis    string         `json:"axis"    enum:"day,hour,category"`
	Drill   *string        `json:"drill"`
	Series  []ReportSeries `json:"series"`
	Columns []ReportColumn `json:"columns"`
	Rows    []ReportRow    `json:"rows"`
}

/*
ReportSeries is one line or set of bars.
*/
type ReportSeries struct {
	Key    string        `json:"key"`
	Label  string        `json:"label"`
	Drill  *string       `json:"drill"`
	Points []ReportPoint `json:"points"`
}

/*
ReportPoint is a value at a day, an hour or a group.
*/
type ReportPoint struct {
	X string   `json:"x"`
	Y *float64 `json:"y"`
}

/*
ReportColumn is one column of a table.
*/
type ReportColumn struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Unit  string `json:"unit"`
}

/*
ReportRow is one row of a table.
*/
type ReportRow struct {
	Key    string            `json:"key"`
	Label  string            `json:"label"`
	Values []*float64        `json:"values"`
	Filter map[string]string `json:"filter"`
}

/*
newReportChart shapes a chart.
*/
func newReportChart(c reports.Chart) ReportChart {
	out := ReportChart{
		Key: c.Key, Title: c.Title, Kind: c.Kind, Unit: c.Unit, Axis: c.Axis, Drill: orNull(c.Drill),
		Series: make([]ReportSeries, len(c.Series)), Columns: make([]ReportColumn, len(c.Columns)),
		Rows: make([]ReportRow, len(c.Rows)),
	}

	for i, s := range c.Series {
		points := make([]ReportPoint, len(s.Points))
		for j, p := range s.Points {
			points[j] = ReportPoint{X: p.X, Y: p.Y}
		}

		out.Series[i] = ReportSeries{Key: s.Key, Label: s.Label, Drill: orNull(s.Drill), Points: points}
	}

	for i, col := range c.Columns {
		out.Columns[i] = ReportColumn{Key: col.Key, Label: col.Label, Unit: col.Unit}
	}

	for i, r := range c.Rows {
		filter := r.Filter
		if filter == nil {
			filter = map[string]string{}
		}

		out.Rows[i] = ReportRow{Key: r.Key, Label: r.Label, Values: r.Values, Filter: filter}
	}

	return out
}
