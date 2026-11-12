/*
|--------------------------------------------------------------------------
| Ticket State Requests
|--------------------------------------------------------------------------
|
| The inputs of a workspace's ticket states: a new state in a group, a
| change to one, their order, and removing one with the state its
| tickets move to.
|
*/

package requests

/*
TicketStateBody is a state's settings; fields left out stay as they are.
The group is read only when the state is added.
*/
type TicketStateBody struct {
	Group          *string `json:"group,omitempty"           enum:"submitted,in_progress,waiting_customer,waiting_team,resolved"` //nolint:lll // one enum
	Name           *string `json:"name,omitempty"            maxLength:"60"`
	CustomerLabel  *string `json:"customer_label,omitempty"  maxLength:"80"`
	NotifyCustomer *bool   `json:"notify_customer,omitempty"`
}

/*
StoreTicketStateRequest is the input of POST /ticket-states.
*/
type StoreTicketStateRequest struct {
	Body TicketStateBody
}

/*
UpdateTicketStateRequest is the input of PATCH /ticket-states/{state}.
*/
type UpdateTicketStateRequest struct {
	State string `path:"state"`
	Body  TicketStateBody
}

/*
ReorderTicketStatesRequest is the live states in their new order.
*/
type ReorderTicketStatesRequest struct {
	Body struct {
		StateIDs []string `json:"state_ids" maxItems:"100" validate:"required,min=1"`
	}
}

/*
DestroyTicketStateRequest is the input of DELETE
/ticket-states/{state}: move_to names the state of the same group its
tickets move to.
*/
type DestroyTicketStateRequest struct {
	State  string `path:"state"`
	MoveTo string `query:"move_to" maxLength:"60"`
}
