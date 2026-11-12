/*
|--------------------------------------------------------------------------
| Ticket Link Requests
|--------------------------------------------------------------------------
|
| The inputs of links: a new link from a ticket, removing one, a page of
| a tracker's conversations, turning a conversation into a ticket, and
| back-office work escalated from a customer ticket.
|
*/

package requests

/*
StoreTicketLinkRequest is the input of POST /tickets/{ticket}/links.
*/
type StoreTicketLinkRequest struct {
	Ticket string `path:"ticket"`
	Body   TicketLinkBody
}

/*
TicketLinkRequest names a link in the path.
*/
type TicketLinkRequest struct {
	Link string `path:"link"`
}

/*
TicketConversationsRequest pages a tracker's conversations.
*/
type TicketConversationsRequest struct {
	Ticket string `path:"ticket"`
	Cursor string `query:"cursor" maxLength:"64"`
}

/*
ConvertConversationRequest is the input of POST
/conversations/{conversation}/ticket. Ai_fill asks the AI to propose
field values, kept as suggestions.
*/
type ConvertConversationRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		TypeID  string         `json:"type_id"            maxLength:"64"  validate:"required"`
		StateID *string        `json:"state_id,omitempty" maxLength:"64"`
		Title   string         `json:"title,omitempty"    maxLength:"200"`
		Fields  map[string]any `json:"fields,omitempty"`
		AIFill  bool           `json:"ai_fill,omitempty"`
	}
}

/*
StoreTicketEscalationRequest is the input of POST
/tickets/{ticket}/escalations: back-office work for a customer ticket.
*/
type StoreTicketEscalationRequest struct {
	Ticket string `path:"ticket"`
	Body   struct {
		TypeID      string         `json:"type_id"               maxLength:"64"    validate:"required"`
		Title       string         `json:"title"                 maxLength:"200"   validate:"required"`
		Description string         `json:"description,omitempty" maxLength:"20000"`
		Fields      map[string]any `json:"fields,omitempty"`
		TeamID      *string        `json:"team_id,omitempty"     maxLength:"64"`
		AssigneeID  *string        `json:"assignee_id,omitempty" maxLength:"64"`
	}
}
