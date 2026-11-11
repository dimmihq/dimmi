/*
|--------------------------------------------------------------------------
| Series Report Resource
|--------------------------------------------------------------------------
|
| A series' numbers as the results view reads them: totals, why people
| left, entries and goals per day, and per step what it did. Message
| steps carry the counts of the broadcast they send through.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/series"
)

/*
SeriesReport is a series' numbers.
*/
type SeriesReport struct {
	Object   string                      `json:"object"`
	Entered  int64                       `json:"entered"`
	Active   int64                       `json:"active"`
	Finished int64                       `json:"finished"`
	Exited   int64                       `json:"exited"`
	Goal     int64                       `json:"goal"`
	Reasons  []SeriesCount               `json:"reasons"`
	Days     []SeriesDay                 `json:"days"`
	Steps    map[string]SeriesStepReport `json:"steps"`
}

/*
SeriesCount is a reason and how many.
*/
type SeriesCount struct {
	Reason string `json:"reason"`
	Count  int64  `json:"count"`
}

/*
SeriesDay is one day's entries and goals.
*/
type SeriesDay struct {
	Day     time.Time `json:"day"`
	Entered int64     `json:"entered"`
	Goal    int64     `json:"goal"`
}

/*
SeriesStepReport is one step's numbers.
*/
type SeriesStepReport struct {
	Reached  int64            `json:"reached"`
	Waiting  int64            `json:"waiting"`
	Paths    map[string]int64 `json:"paths"`
	Outcomes map[string]int64 `json:"outcomes"`
	Message  json.RawMessage  `json:"message,omitempty"`
	Variants []SeriesVariant  `json:"variants,omitempty"`
}

/*
SeriesVariant is one variant of a split and its goal.
*/
type SeriesVariant struct {
	Path    string `json:"path"`
	Entered int64  `json:"entered"`
	Goal    int64  `json:"goal"`
}

/*
NewSeriesReport shapes a series' report.
*/
func NewSeriesReport(r series.Report) SeriesReport {
	out := SeriesReport{
		Object: "series_report", Entered: r.Totals.Entered, Active: r.Totals.Active, Finished: r.Totals.Finished,
		Exited: r.Totals.Exited, Goal: r.Totals.Goal, Reasons: []SeriesCount{}, Days: []SeriesDay{},
		Steps: map[string]SeriesStepReport{},
	}

	for _, reason := range r.Reasons {
		out.Reasons = append(out.Reasons, SeriesCount{Reason: reason.Reason, Count: reason.N})
	}

	for _, d := range r.Days {
		out.Days = append(out.Days, SeriesDay{Day: d.Day, Entered: d.Entered, Goal: d.Goal})
	}

	for id, st := range r.Steps {
		step := SeriesStepReport{
			Reached: st.Reached, Waiting: st.Waiting, Paths: st.Paths, Outcomes: st.Outcomes, Message: st.Message,
		}

		for _, v := range st.Variants {
			step.Variants = append(step.Variants, SeriesVariant{Path: v.Path, Entered: v.Entered, Goal: v.Goal})
		}

		out.Steps[id] = step
	}

	return out
}
