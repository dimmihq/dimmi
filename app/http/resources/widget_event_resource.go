/*
|--------------------------------------------------------------------------
| Widget Event Resources
|--------------------------------------------------------------------------
|
| What became of the events a messenger sent, in the order sent. Only the
| status of each is told; the visitor's contact id never leaves the
| server.
|
*/

package resources

import "github.com/dimmihq/dimmi/app/actions/events"

/*
WidgetTrackedEvent is what became of one event the messenger sent.
*/
type WidgetTrackedEvent struct {
	Status string `json:"status"`
}

/*
WidgetTrackedEvents is what became of a batch the messenger sent.
*/
type WidgetTrackedEvents struct {
	Object   string               `json:"object"`
	Accepted int                  `json:"accepted"`
	Events   []WidgetTrackedEvent `json:"events"`
}

/*
NewWidgetTrackedEvents shapes the results of a messenger's batch.
*/
func NewWidgetTrackedEvents(results []events.Result) WidgetTrackedEvents {
	out := WidgetTrackedEvents{Object: "widget_event_batch", Events: make([]WidgetTrackedEvent, len(results))}
	for i, r := range results {
		out.Events[i] = WidgetTrackedEvent{Status: r.Status}
		if r.Status == events.StatusAccepted {
			out.Accepted++
		}
	}

	return out
}
