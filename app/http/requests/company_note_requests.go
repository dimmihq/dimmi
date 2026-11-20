/*
|--------------------------------------------------------------------------
| Company Note Requests
|--------------------------------------------------------------------------
|
| The inputs of the notes the team keeps about a company.
|
*/

package requests

/*
CompanyNotesRequest names a company in the path.
*/
type CompanyNotesRequest struct {
	Company string `path:"company"`
}

/*
StoreCompanyNoteRequest is the input of POST /companies/{company}/notes.
*/
type StoreCompanyNoteRequest struct {
	Company string `path:"company"`
	Body    struct {
		Body string `json:"body" minLength:"1" maxLength:"10000"`
	}
}
