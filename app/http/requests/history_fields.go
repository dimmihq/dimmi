/*
|--------------------------------------------------------------------------
| History Fields
|--------------------------------------------------------------------------
|
| A message or note brought in from another tool says when it was
| written, the id it had there and who wrote it. Sending any of them
| turns the request into an import.
|
*/

package requests

import "time"

/*
HistoryFields are what an entry brought in from another tool says about
itself: when it was written, the id it had there, and the member who
wrote it. Any of them makes the request an import.
*/
type HistoryFields struct {
	CreatedAt  *time.Time `json:"created_at,omitempty"`
	ExternalID *string    `json:"external_id,omitempty" validate:"omitempty,min=1,max=255"`
	AuthorID   *string    `json:"author_id,omitempty"`
}

/*
Importing reports whether any history field was sent.
*/
func (h HistoryFields) Importing() bool {
	return h.CreatedAt != nil || h.ExternalID != nil || h.AuthorID != nil
}
