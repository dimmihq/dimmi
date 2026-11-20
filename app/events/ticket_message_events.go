/*
|--------------------------------------------------------------------------
| Ticket Message Events
|--------------------------------------------------------------------------
|
| A ticket gained a note, or a customer wrote on their ticket.
|
*/

package events

/*
TicketNoteCreated says a teammate left an internal note.
*/
type TicketNoteCreated struct{ TicketChanged }

/*
TicketCustomerReplied says the customer wrote on a customer ticket's
conversation. It stays internal, for workflows.
*/
type TicketCustomerReplied struct{ TicketChanged }
