/*
|--------------------------------------------------------------------------
| Ticket State Events
|--------------------------------------------------------------------------
|
| A ticket's title, fields, state or assignee changed, it resolved, or it
| reopened. Each carries what moved, from and to.
|
*/

package events

/*
TicketUpdated says the title, description or field values changed;
Changes names each changed key, fields under fields.key.
*/
type TicketUpdated struct{ TicketChanged }

/*
TicketStateChanged says the ticket moved to another state; Changes
holds state and group.
*/
type TicketStateChanged struct{ TicketChanged }

/*
TicketResolved says the ticket entered the resolved group.
*/
type TicketResolved struct{ TicketChanged }

/*
TicketReopened says the ticket left the resolved group.
*/
type TicketReopened struct{ TicketChanged }

/*
TicketAssigned says the teammate or team changed.
*/
type TicketAssigned struct{ TicketChanged }
