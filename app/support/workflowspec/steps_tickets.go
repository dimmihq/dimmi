/*
|--------------------------------------------------------------------------
| Ticket Steps
|--------------------------------------------------------------------------
|
| Steps that work the conversation's customer ticket: make the
| conversation a ticket, with fields mapped from answers and
| variables; move it to a state; set its fields; escalate it to
| back-office work; and add the conversation to a tracker. Each is an
| action that runs in the background. A ticket that refuses a change,
| such as a required field left empty, stays as it was and the step is
| recorded as failed.
|
*/

package workflowspec

/*
The ticket step kinds.
*/
const (
	KindCreateTicket    = "create_ticket"
	KindSetTicketState  = "set_ticket_state"
	KindSetTicketFields = "set_ticket_fields"
	KindEscalate        = "escalate"
	KindAddToTracker    = "add_to_tracker"
)

/*
ticketKinds returns the ticket step kinds.
*/
func ticketKinds() []Kind {
	return []Kind{
		{Key: KindCreateTicket, Ready: true, Action: true},
		{Key: KindSetTicketState, Ready: true, Action: true},
		{Key: KindSetTicketFields, Ready: true, Action: true},
		{Key: KindEscalate, Ready: true, Action: true},
		{Key: KindAddToTracker, Ready: true, Action: true},
	}
}

/*
TicketValue sets one ticket field, by key, to a value that may hold
variables such as {{run.vars.order_number}}.
*/
type TicketValue struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

/*
CreateTicketSettings makes the conversation a ticket of the type, with
a title, a starting state and field values; empty ones take the type's
defaults and the conversation's subject.
*/
type CreateTicketSettings struct {
	TypeID  string        `json:"type_id"`
	Title   string        `json:"title,omitempty"`
	StateID string        `json:"state_id,omitempty"`
	Fields  []TicketValue `json:"fields,omitempty"`
}

/*
SetTicketStateSettings moves the ticket to a state.
*/
type SetTicketStateSettings struct {
	StateID string `json:"state_id"`
}

/*
SetTicketFieldsSettings sets field values.
*/
type SetTicketFieldsSettings struct {
	Fields []TicketValue `json:"fields"`
}

/*
EscalateSettings makes back-office work of the type for the ticket,
for a team when one is named.
*/
type EscalateSettings struct {
	TypeID string        `json:"type_id"`
	Title  string        `json:"title"`
	TeamID string        `json:"team_id,omitempty"`
	Fields []TicketValue `json:"fields,omitempty"`
}
