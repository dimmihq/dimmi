/*
|--------------------------------------------------------------------------
| Series Trace Resource
|--------------------------------------------------------------------------
|
| One journey read step by step: every step the traveler reached, the
| path they left by, what happened and when, so a teammate can answer
| why someone got a message or did not.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
SeriesStep is one step a journey took.
*/
type SeriesStep struct {
	NodeID    string          `json:"node_id"`
	Kind      string          `json:"kind"`
	Status    string          `json:"status"`
	PathID    *string         `json:"path_id"`
	Outcome   *string         `json:"outcome"`
	Detail    json.RawMessage `json:"detail"`
	EnteredAt time.Time       `json:"entered_at"`
	LeftAt    *time.Time      `json:"left_at"`
}

/*
SeriesTrace is a journey and the steps it took.
*/
type SeriesTrace struct {
	SeriesEnrollment
	Steps []SeriesStep `json:"steps"`
}

/*
NewSeriesTrace shapes a journey with its steps.
*/
func NewSeriesTrace(e models.SeriesEnrollment, steps []models.SeriesStep) SeriesTrace {
	out := SeriesTrace{SeriesEnrollment: NewSeriesEnrollment(e), Steps: make([]SeriesStep, len(steps))}
	for i, s := range steps {
		out.Steps[i] = SeriesStep{
			NodeID: s.NodeID, Kind: s.Kind, Status: s.Status, PathID: s.PathID, Outcome: s.Outcome, Detail: s.Detail,
			EnteredAt: s.EnteredAt, LeftAt: s.LeftAt,
		}
	}

	return out
}
