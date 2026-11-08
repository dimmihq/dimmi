/*
|--------------------------------------------------------------------------
| Event Requests
|--------------------------------------------------------------------------
|
| The inputs of tracking events and of their names. One event comes as
| the body itself, a batch of up to 100 under "events". Each event names
| its person by exactly one of contact_id, external_id and email.
|
*/

package requests

import "time"

/*
EventSent is one event as sent.
*/
type EventSent struct {
	Name           string         `json:"name,omitempty"            maxLength:"64"`
	ContactID      *string        `json:"contact_id,omitempty"      maxLength:"64"`
	ExternalID     *string        `json:"external_id,omitempty"     maxLength:"255"`
	Email          *string        `json:"email,omitempty"           maxLength:"320"`
	OccurredAt     *time.Time     `json:"occurred_at,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty" maxLength:"128"`
}

/*
TrackEventsRequest is the input of POST /contact-events.
*/
type TrackEventsRequest struct {
	Body struct {
		EventSent
		Events []EventSent `json:"events,omitempty" maxItems:"100"`
	}
}

/*
EventRequest names an event definition in the path.
*/
type EventRequest struct {
	Event string `path:"event"`
}

/*
UpdateEventRequest is the input of PATCH /event-definitions/{event}.
*/
type UpdateEventRequest struct {
	Event string `path:"event"`
	Body  struct {
		Label       *string `json:"label,omitempty"       maxLength:"80"`
		Description *string `json:"description,omitempty" maxLength:"500"`
		Status      *string `json:"status,omitempty"      enum:"active,archived"`
	}
}

/*
ContactEventsRequest names a contact whose events are read.
*/
type ContactEventsRequest struct {
	Contact string `path:"contact"`
}
