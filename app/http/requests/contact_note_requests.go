/*
|--------------------------------------------------------------------------
| Contact Note Requests
|--------------------------------------------------------------------------
|
| The inputs of the notes the team keeps about a customer or a company.
|
*/

package requests

/*
ContactNotesRequest names a contact in the path.
*/
type ContactNotesRequest struct {
	Contact string `path:"contact"`
}

/*
StoreContactNoteRequest is the input of POST /contacts/{contact}/notes.
*/
type StoreContactNoteRequest struct {
	Contact string `path:"contact"`
	Body    struct {
		Body string `json:"body" minLength:"1" maxLength:"10000"`
	}
}

/*
UpdateContactNoteRequest is the input of PATCH /contact-notes/{note}:
new words, whether the note is pinned, or both. A field left out is
left as it is.
*/
type UpdateContactNoteRequest struct {
	Note string `path:"note"`
	Body struct {
		Body   *string `json:"body,omitempty"   minLength:"1" maxLength:"10000"`
		Pinned *bool   `json:"pinned,omitempty"`
	}
}

/*
ContactNoteRequest names a note in the path.
*/
type ContactNoteRequest struct {
	Note string `path:"note"`
}
