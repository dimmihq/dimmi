/*
|--------------------------------------------------------------------------
| Ticket Message Events
|--------------------------------------------------------------------------
|
| A ticket gained a note, a tracker's message to its customers finished,
| an incident posted publicly, a customer wrote on their ticket, or a
| spike of conversations suggested an incident.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TicketNoteCreated says a teammate left an internal note.
*/
type TicketNoteCreated struct{ TicketChanged }

/*
TicketUpdateSent says a mass message to the linked customers finished;
Changes holds its counts.
*/
type TicketUpdateSent struct{ TicketChanged }

/*
TicketPublicUpdatePosted says an incident posted to its public log.
*/
type TicketPublicUpdatePosted struct{ TicketChanged }

/*
TicketCustomerReplied says the customer wrote on a customer ticket's
conversation. It stays internal, for workflows.
*/
type TicketCustomerReplied struct{ TicketChanged }

/*
IncidentSuggested says a spike of conversations about one thing was
found; SuggestionID names it.
*/
type IncidentSuggested struct {
	Base
	SuggestionID ids.UUID
}
