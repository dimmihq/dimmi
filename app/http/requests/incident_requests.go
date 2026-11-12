/*
|--------------------------------------------------------------------------
| Incident Requests
|--------------------------------------------------------------------------
|
| The inputs around incidents and trackers: deciding a suggested
| incident, the trackers suggested for a conversation, and a tracker's
| summary.
|
*/

package requests

/*
IncidentSuggestionsRequest is the input of GET /incident-suggestions.
*/
type IncidentSuggestionsRequest struct{}

/*
UpdateIncidentSuggestionRequest is the input of PATCH
/incident-suggestions/{suggestion}: accepted declares the incident,
dismissed mutes the group for an hour.
*/
type UpdateIncidentSuggestionRequest struct {
	Suggestion string `path:"suggestion"`
	Body       struct {
		Status string `json:"status" enum:"accepted,dismissed" validate:"required"`
	}
}

/*
TrackerSuggestionsRequest names a conversation in the path.
*/
type TrackerSuggestionsRequest struct {
	Conversation string `path:"conversation"`
}

/*
TicketSummaryRequest names a tracker or incident in the path.
*/
type TicketSummaryRequest struct {
	Ticket string `path:"ticket"`
}
