/*
|--------------------------------------------------------------------------
| Widget Event Requests
|--------------------------------------------------------------------------
|
| The input of POST /widget/v1/events: what the visitor did on the page,
| up to 20 events at once. The session names the person, and the server
| decides when each happened. An id makes a retried event count once.
|
*/

package requests

/*
WidgetEventSent is one event the messenger sends.
*/
type WidgetEventSent struct {
	Name     string         `json:"name"               maxLength:"64"`
	Metadata map[string]any `json:"metadata,omitempty"`
	ID       string         `json:"id,omitempty"       maxLength:"64"`
}

/*
StoreWidgetEventsRequest is the input of POST /widget/v1/events.
*/
type StoreWidgetEventsRequest struct {
	Body struct {
		Events []WidgetEventSent `json:"events" minItems:"1" maxItems:"20"`
	}
}
