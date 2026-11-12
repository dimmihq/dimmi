/*
|--------------------------------------------------------------------------
| Ticket Events
|--------------------------------------------------------------------------
|
| What happens to a ticket: it is created, changed, moved to another
| state, resolved or reopened, assigned, given another type, linked to
| other work, sent to every linked customer, or trashed. Each names the
| ticket and, beside it, the conversation of a customer ticket, so
| workflows on that conversation hear it. Changes carries what moved,
| from and to. Every event is dispatched in the transaction that made
| the change.
|
*/

package events

import "github.com/dimmihq/dimmi/framework/ids"

/*
TicketRef names the ticket an event is about, its kind and the
conversation of a customer ticket, nil for the other kinds.
*/
type TicketRef struct {
	TicketID       ids.UUID
	Kind           string
	ConversationID *ids.UUID
}

/*
TicketChange is one moved value of a ticket event: what it was and
what it became.
*/
type TicketChange struct {
	From any `json:"from"`
	To   any `json:"to"`
}

/*
TicketChanged is the shape every ticket event shares.
*/
type TicketChanged struct {
	Base
	TicketRef
	Changes map[string]TicketChange
}

/*
Ticket returns the event's ticket reference, for listeners that hear
every ticket event.
*/
func (e TicketChanged) Ticket() TicketChanged { return e }

/*
TicketEvent is any ticket event.
*/
type TicketEvent interface {
	/*
		EventName returns the event's dotted name, such as ticket.created.
	*/
	EventName() string

	/*
		Ticket returns the shared shape: the ticket, its changes and actor.
	*/
	Ticket() TicketChanged
}

/*
TicketCreated says a ticket was created or a conversation converted.
*/
type TicketCreated struct{ TicketChanged }

/*
EventName names the event.
*/
func (TicketCreated) EventName() string { return "ticket.created" }

/*
EventName names the event.
*/
func (TicketUpdated) EventName() string { return "ticket.updated" }

/*
EventName names the event.
*/
func (TicketStateChanged) EventName() string { return "ticket.state_changed" }

/*
EventName names the event.
*/
func (TicketResolved) EventName() string { return "ticket.resolved" }

/*
EventName names the event.
*/
func (TicketReopened) EventName() string { return "ticket.reopened" }

/*
EventName names the event.
*/
func (TicketAssigned) EventName() string { return "ticket.assigned" }

/*
EventName names the event.
*/
func (TicketTypeChanged) EventName() string { return "ticket.type_changed" }

/*
EventName names the event.
*/
func (TicketLinked) EventName() string { return "ticket.linked" }

/*
EventName names the event.
*/
func (TicketUnlinked) EventName() string { return "ticket.unlinked" }

/*
EventName names the event.
*/
func (TicketIssueLinked) EventName() string { return "ticket.issue_linked" }

/*
EventName names the event.
*/
func (TicketIssueUpdated) EventName() string { return "ticket.issue_updated" }

/*
EventName names the event.
*/
func (TicketNoteCreated) EventName() string { return "ticket.note_created" }

/*
EventName names the event.
*/
func (TicketUpdateSent) EventName() string { return "ticket.update_sent" }

/*
EventName names the event.
*/
func (TicketPublicUpdatePosted) EventName() string { return "ticket.public_update_posted" }

/*
EventName names the event.
*/
func (TicketSLAWarning) EventName() string { return "ticket.sla_warning" }

/*
EventName names the event.
*/
func (TicketSLABreached) EventName() string { return "ticket.sla_breached" }

/*
EventName names the event.
*/
func (TicketDeleted) EventName() string { return "ticket.deleted" }

/*
EventName names the event.
*/
func (TicketRestored) EventName() string { return "ticket.restored" }

/*
EventName names the event.
*/
func (TicketCustomerReplied) EventName() string { return "ticket.customer_replied" }

/*
EventName names the event.
*/
func (IncidentSuggested) EventName() string { return "incident.suggested" }
