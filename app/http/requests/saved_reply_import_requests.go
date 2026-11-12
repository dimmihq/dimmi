/*
|--------------------------------------------------------------------------
| Saved Reply Import Requests
|--------------------------------------------------------------------------
|
| The input of importing saved replies: rows a spreadsheet gave.
|
*/

package requests

/*
ImportedSavedReply is one row of an import.
*/
type ImportedSavedReply struct {
	Title    string  `json:"title"              validate:"required,min=1,max=120"`
	Body     string  `json:"body"               validate:"required,min=1,max=65536"`
	Shortcut *string `json:"shortcut,omitempty" validate:"omitempty,max=40"`
	Shared   bool    `json:"shared,omitempty"`
}

/*
StoreSavedReplyImportRequest is the input of POST /saved-reply-imports.
*/
type StoreSavedReplyImportRequest struct {
	Body struct {
		Replies []ImportedSavedReply `json:"replies" validate:"required,min=1,max=500,dive"`
	}
}
