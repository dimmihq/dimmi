/*
|--------------------------------------------------------------------------
| Tracker Requests
|--------------------------------------------------------------------------
|
| The inputs around trackers: the trackers suggested for a conversation,
| and a tracker's summary.
|
*/

package requests

/*
TrackerSuggestionsRequest names a conversation in the path.
*/
type TrackerSuggestionsRequest struct {
	Conversation string `path:"conversation"`
}

/*
TicketSummaryRequest names a tracker in the path.
*/
type TicketSummaryRequest struct {
	Ticket string `path:"ticket"`
}
