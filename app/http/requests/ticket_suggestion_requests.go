/*
|--------------------------------------------------------------------------
| Ticket Suggestion Requests
|--------------------------------------------------------------------------
|
| The inputs of what the panel of a conversation reads: its own ticket
| and the trackers it joined, and the AI's values for a type's fields.
|
*/

package requests

/*
ConversationTicketsRequest names a conversation in the path.
*/
type ConversationTicketsRequest struct {
	Conversation string `path:"conversation"`
}

/*
TicketSuggestionsRequest asks the AI's values for a type's fields from
a conversation.
*/
type TicketSuggestionsRequest struct {
	Conversation string `path:"conversation"`
	TypeID       string `query:"type_id" maxLength:"64" validate:"required"`
}
