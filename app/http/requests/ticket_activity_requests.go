/*
|--------------------------------------------------------------------------
| Ticket Activity Requests
|--------------------------------------------------------------------------
|
| The inputs of a ticket's history and messages: a page of one ticket's
| log or the workspace's, an internal note, a message to everyone a
| tracker links, a post to an incident's public log, and the AI's field
| suggestions for a conversation.
|
*/

package requests

/*
TicketEventsRequest pages a ticket's history.
*/
type TicketEventsRequest struct {
	Ticket string `path:"ticket"`
	Cursor string `query:"cursor" maxLength:"64"`
}

/*
WorkspaceTicketEventsRequest pages the workspace's ticket log after a
cursor, or since a time.
*/
type WorkspaceTicketEventsRequest struct {
	Cursor string `query:"cursor" maxLength:"64"`
	Since  string `query:"since"  maxLength:"40"`
}

/*
StoreTicketNoteRequest is the input of POST /tickets/{ticket}/notes.
*/
type StoreTicketNoteRequest struct {
	Ticket string `path:"ticket"`
	Body   struct {
		Body string `json:"body" maxLength:"20000" validate:"required"`
	}
}

/*
StoreTicketUpdateRequest is the input of POST /tickets/{ticket}/updates.
Dry_run only counts who it reaches and who would be held.
*/
type StoreTicketUpdateRequest struct {
	Ticket string `path:"ticket"`
	Body   struct {
		Body    string `json:"body"              maxLength:"20000" validate:"required"`
		Resolve bool   `json:"resolve,omitempty"`
		DryRun  bool   `json:"dry_run,omitempty"`
	}
}

/*
StorePublicUpdateRequest is the input of POST
/tickets/{ticket}/public-updates.
*/
type StorePublicUpdateRequest struct {
	Ticket string `path:"ticket"`
	Body   struct {
		Body         string  `json:"body"                     maxLength:"2000" validate:"required"`
		StateID      *string `json:"state_id,omitempty"       maxLength:"64"`
		SendToLinked bool    `json:"send_to_linked,omitempty"`
	}
}
