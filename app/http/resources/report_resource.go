/*
|--------------------------------------------------------------------------
| Report Resources
|--------------------------------------------------------------------------
|
| How a report reads over the API, whatever it is about: the range and
| the range compared with, the headline numbers and the charts. A number
| carries its unit, its value, its value before, and for a duration the
| 90th percentile and the mean, in seconds; drill names the metric that
| lists its conversations at /reports/{report}/conversations.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/app/actions/reports"
)

/*
ReportType is one report the caller may read.
*/
type ReportType struct {
	Object  string `json:"object"`
	Key     string `json:"key"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Live    bool   `json:"live"`
}

/*
NewReportType shapes a report's description.
*/
func NewReportType(d reports.Description) ReportType {
	return ReportType{Object: "report_type", Key: d.Key, Title: d.Title, Summary: d.Summary, Live: d.Live}
}

/*
ReportResult is one report as asked for. From and to are days, to included;
the previous ones are null without a comparison.
*/
type ReportResult struct {
	Object        string         `json:"object"`
	Key           string         `json:"key"`
	Title         string         `json:"title"`
	From          string         `json:"from"`
	To            string         `json:"to"`
	PreviousFrom  *string        `json:"previous_from"`
	PreviousTo    *string        `json:"previous_to"`
	Timezone      string         `json:"timezone"`
	BusinessHours bool           `json:"business_hours"`
	Live          bool           `json:"live"`
	Metrics       []ReportMetric `json:"metrics"`
	Charts        []ReportChart  `json:"charts"`
}

/*
ReportMetric is a headline number.
*/
type ReportMetric struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Unit     string   `json:"unit"     enum:"count,percent,seconds,micros,score"`
	Value    *float64 `json:"value"`
	Previous *float64 `json:"previous"`
	P90      *float64 `json:"p90"`
	Mean     *float64 `json:"mean"`
	Sample   int64    `json:"sample"`
	Drill    *string  `json:"drill"`
}

/*
NewReport shapes a report.
*/
func NewReport(r reports.Report) ReportResult {
	out := ReportResult{
		Object: "report", Key: r.Key, Title: r.Title, From: day(r.Start), To: day(r.End.AddDate(0, 0, -1)),
		Timezone: r.Timezone, BusinessHours: r.BusinessHours, Live: r.Live,
		Metrics: make([]ReportMetric, len(r.Metrics)), Charts: make([]ReportChart, len(r.Charts)),
	}
	if r.PreviousStart != nil && r.PreviousEnd != nil {
		out.PreviousFrom, out.PreviousTo = new(day(*r.PreviousStart)), new(day(r.PreviousEnd.AddDate(0, 0, -1)))
	}

	for i, m := range r.Metrics {
		out.Metrics[i] = ReportMetric{
			Key: m.Key, Label: m.Label, Unit: m.Unit, Value: m.Value, Previous: m.Previous, P90: m.P90,
			Mean: m.Mean, Sample: m.Sample, Drill: orNull(m.Drill),
		}
	}

	for i, c := range r.Charts {
		out.Charts[i] = newReportChart(c)
	}

	return out
}

/*
day writes a moment's day in its own location.
*/
func day(t time.Time) string {
	return t.Format(time.DateOnly)
}

/*
orNull returns nil for an empty string.
*/
func orNull(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}
