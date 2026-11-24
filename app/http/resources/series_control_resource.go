/*
|--------------------------------------------------------------------------
| Series Control Group Resource
|--------------------------------------------------------------------------
|
| A series' control group against everyone else, as its report carries
| it: the share held out, and each group's entries and goals.
|
*/

package resources

import (
	"github.com/dimmihq/dimmi/app/actions/series"
)

/*
SeriesControl is a series' control group against everyone else: the
share held out, and each group's entries and goals.
*/
type SeriesControl struct {
	Percent int                  `json:"percent"`
	Groups  []SeriesControlGroup `json:"groups"`
}

/*
SeriesControlGroup is one group, control or treated, and its goal.
*/
type SeriesControlGroup struct {
	Group   string `json:"group"`
	Entered int64  `json:"entered"`
	Goal    int64  `json:"goal"`
}

/*
newSeriesControl shapes a control group report, nil for none.
*/
func newSeriesControl(c *series.ControlReport) *SeriesControl {
	if c == nil {
		return nil
	}

	out := &SeriesControl{Percent: c.Percent, Groups: []SeriesControlGroup{}}
	for _, g := range c.Groups {
		out.Groups = append(out.Groups, SeriesControlGroup{Group: g.Path, Entered: g.Entered, Goal: g.Goal})
	}

	return out
}
