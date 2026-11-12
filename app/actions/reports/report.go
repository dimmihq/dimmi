/*
|--------------------------------------------------------------------------
| Reports
|--------------------------------------------------------------------------
|
| A report is headline numbers and charts, in one shape whatever it is
| about, so the dashboard, the API, the digest and the CSV files read
| every report the same way. A number names the metric whose
| conversations it counts, so any number opens the conversations behind
| it; a chart point or table row narrows the filters to its day or its
| group.
|
*/

package reports

import "time"

/*
The units a number comes in.
*/
const (
	UnitCount   = "count"
	UnitPercent = "percent"
	UnitSeconds = "seconds"
	UnitMicros  = "micros"
	UnitScore   = "score"
)

/*
Report is one report as asked for. Previous is the range compared with,
nil when none is.
*/
type Report struct {
	Key           string
	Title         string
	Start         time.Time
	End           time.Time
	PreviousStart *time.Time
	PreviousEnd   *time.Time
	Timezone      string
	BusinessHours bool
	Metrics       []Metric
	Charts        []Chart
	Live          bool
}

/*
Metric is a headline number: its value now and in the period compared
with, for a duration the median with the 90th percentile and the mean
beside it, how many conversations it rests on, and the metric to list
them by.
*/
type Metric struct {
	Key      string
	Label    string
	Unit     string
	Value    *float64
	Previous *float64
	P90      *float64
	Mean     *float64
	Sample   int64
	Drill    string
}

/*
num returns a number as a pointer.
*/
func num[T int | int64 | float64](v T) *float64 {
	f := float64(v)

	return &f
}

/*
ratio returns the percent the part is of the whole, nil without a
whole.
*/
func ratio(part, whole int64) *float64 {
	if whole == 0 {
		return nil
	}

	return num(float64(part) * 100 / float64(whole))
}
