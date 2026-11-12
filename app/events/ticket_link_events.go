/*
|--------------------------------------------------------------------------
| Ticket Link Events
|--------------------------------------------------------------------------
|
| A ticket moved to another type, was linked to or unlinked from another
| ticket or a conversation, or an issue behind it was linked or moved.
|
*/

package events

/*
TicketTypeChanged says the type changed; Changes holds the dropped
field keys under dropped_fields.
*/
type TicketTypeChanged struct{ TicketChanged }

/*
TicketLinked says the ticket was linked to another ticket or a
conversation; Changes holds the link under link.
*/
type TicketLinked struct{ TicketChanged }

/*
TicketUnlinked says a link was removed.
*/
type TicketUnlinked struct{ TicketChanged }

/*
TicketIssueLinked says an issue or engineering incident was linked.
*/
type TicketIssueLinked struct{ TicketChanged }

/*
TicketIssueUpdated says a linked issue's status changed.
*/
type TicketIssueUpdated struct{ TicketChanged }
