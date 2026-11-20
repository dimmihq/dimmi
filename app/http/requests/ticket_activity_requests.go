/*
|--------------------------------------------------------------------------
| Ticket Activity Requests
|--------------------------------------------------------------------------
|
| The inputs of a ticket's history: a page of one ticket's log or the
| workspace's, and an internal note.
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
