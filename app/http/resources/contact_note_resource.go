/*
|--------------------------------------------------------------------------
| Contact Note Resources
|--------------------------------------------------------------------------
|
| How a note about a customer or a company reads over the API: its words, whether it
| is pinned, whether the reader may change it, and the teammate who
| wrote it, null when nobody did, as for a note written with an API
| token.
|
*/

package resources

import (
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/contacts"
	"github.com/dimmihq/dimmi/app/models"
)

/*
ContactNoteAuthor is the teammate who wrote a note.
*/
type ContactNoteAuthor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

/*
ContactNote is one note about a customer or a company: contact_id or
company_id names which, and the other is null.
*/
type ContactNote struct {
	ID        string             `json:"id"`
	Object    string             `json:"object"`
	ContactID *string            `json:"contact_id"`
	CompanyID *string            `json:"company_id"`
	Body      string             `json:"body"`
	Pinned    bool               `json:"pinned"`
	Editable  bool               `json:"editable"`
	Author    *ContactNoteAuthor `json:"author"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
}

/*
NewContactNote shapes a note for the member reading it: editable says
whether they may rewrite or delete it, which its author may, and anyone
when it has no author.
*/
func NewContactNote(n contacts.Note, viewer ids.UUID) ContactNote {
	out := ContactNote{
		ID: models.PublicID(models.PrefixContactNote, n.ID), Object: "contact_note",
		ContactID: broadcastRef(models.PrefixContact, n.ContactID),
		CompanyID: broadcastRef(models.PrefixCompany, n.CompanyID), Body: n.Body, Pinned: n.PinnedAt != nil,
		Editable:  n.AuthorID == nil || *n.AuthorID == viewer,
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt,
	}
	if n.AuthorID != nil {
		out.Author = &ContactNoteAuthor{ID: models.PublicID(models.PrefixMember, *n.AuthorID), Name: n.AuthorName}
	}

	return out
}
