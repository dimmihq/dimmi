/*
|--------------------------------------------------------------------------
| Ticket State Moves
|--------------------------------------------------------------------------
|
| When a type stops using a state that still holds some of its tickets,
| the change says which of the type's remaining states they move to.
|
*/

package requests

/*
TicketStateMove says where the type's tickets in a state it no longer
uses go: one of the states state_ids keeps.
*/
type TicketStateMove struct {
	FromStateID string `json:"from_state_id" maxLength:"64" validate:"required"`
	ToStateID   string `json:"to_state_id"   maxLength:"64" validate:"required"`
}
