/*
|--------------------------------------------------------------------------
| Draft Resource
|--------------------------------------------------------------------------
|
| How a draft reads over the API: what its writer typed, the files and
| recipients staged with it, and the moment their client last changed it,
| which a client compares with its own copy to keep the newest.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Draft is one of the caller's drafts. ConversationID is nil for a new
conversation's draft.
*/
type Draft struct {
	ID              string          `json:"id"`
	Object          string          `json:"object"`
	ConversationID  *string         `json:"conversation_id"`
	Mode            string          `json:"mode"`
	Body            string          `json:"body"`
	AttachmentIDs   []string        `json:"attachment_ids"`
	Recipients      json.RawMessage `json:"recipients"`
	ClientUpdatedAt time.Time       `json:"client_updated_at"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

/*
NewDraft shapes the draft.
*/
func NewDraft(d models.ConversationDraft) Draft {
	out := Draft{
		ID: models.PublicID(models.PrefixDraft, d.ID), Object: "draft", Mode: d.Mode, Body: d.Body,
		AttachmentIDs: publicIDs(models.PrefixAttachment, d.AttachmentIds), Recipients: d.Recipients,
		ClientUpdatedAt: d.ClientUpdatedAt, CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	}

	if d.ConversationID != nil {
		out.ConversationID = new(models.PublicID(models.PrefixConversation, *d.ConversationID))
	}

	return out
}

/*
NewDrafts shapes the drafts.
*/
func NewDrafts(rows []models.ConversationDraft) []Draft {
	out := make([]Draft, len(rows))
	for i, d := range rows {
		out[i] = NewDraft(d)
	}

	return out
}
