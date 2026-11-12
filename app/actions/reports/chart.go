/*
|--------------------------------------------------------------------------
| Charts
|--------------------------------------------------------------------------
|
| A chart is series of points along days, hours of the week or groups,
| or a table of rows under columns. Every chart can be read as a CSV
| file the same way: one row per point or table row.
|
*/

package reports

/*
The kinds of chart.
*/
const (
	ChartLine      = "line"
	ChartBar       = "bar"
	ChartStacked   = "stacked_bar"
	ChartHeatmap   = "heatmap"
	ChartHistogram = "histogram"
	ChartTable     = "table"
)

/*
The axes a chart runs along.
*/
const (
	AxisDay      = "day"
	AxisHour     = "hour"
	AxisCategory = "category"
)

/*
Chart is a chart: series of points along days, hours or groups, or a
table of rows under columns.
*/
type Chart struct {
	Key     string
	Title   string
	Kind    string
	Unit    string
	Axis    string
	Drill   string
	Series  []Series
	Columns []Column
	Rows    []Row
}

/*
Series is one line or set of bars. Drill is the metric its points list,
empty to use the chart's.
*/
type Series struct {
	Key    string
	Label  string
	Drill  string
	Points []Point
}

/*
Point is a value at a day, an hour or a group; nil is no data.
*/
type Point struct {
	X string
	Y *float64
}

/*
Column is one column of a table.
*/
type Column struct {
	Key   string
	Label string
	Unit  string
}

/*
Row is one row of a table: its values in the columns' order, and the
filter that narrows a report to it, such as {"member": "mem_..."}.
*/
type Row struct {
	Key    string
	Label  string
	Values []*float64
	Filter map[string]string
}
