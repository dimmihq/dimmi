/*
|--------------------------------------------------------------------------
| Erasure Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that remove a customer: DELETE on a contact,
| which erases with erase=true and skips the grace period with
| immediate=true, and the endpoints on the erasure requests themselves,
| where the only change is to cancel.
|
*/

package requests

/*
RemoveContactRequest is the input of DELETE /contacts/{contact}.
*/
type RemoveContactRequest struct {
	Contact   string `path:"contact"`
	Erase     bool   `query:"erase"`
	Immediate bool   `query:"immediate"`
}

/*
ContactErasureRequest is the input of the endpoints that name only an
erasure.
*/
type ContactErasureRequest struct {
	Erasure string `path:"contact_erasure"`
}

/*
UpdateContactErasureRequest is the input of PATCH
/contact-erasures/{contact_erasure}.
*/
type UpdateContactErasureRequest struct {
	Erasure string `path:"contact_erasure"`
	Body    struct {
		Status string `json:"status" validate:"required,oneof=canceled"`
	}
}

/*
ContactExportRequest is the input of the endpoints on one export of a
contact.
*/
type ContactExportRequest struct {
	Contact string `path:"contact"`
	Export  string `path:"export"`
}

/*
StoreContactExportRequest is the input of POST
/contacts/{contact}/exports: whether the team's notes go in too. The
body may be left out.
*/
type StoreContactExportRequest struct {
	Contact string `path:"contact"`
	Body    *struct {
		IncludeNotes bool `json:"include_notes,omitempty"`
	}
}
