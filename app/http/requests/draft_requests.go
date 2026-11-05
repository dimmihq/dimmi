/*
|--------------------------------------------------------------------------
| Draft Requests
|--------------------------------------------------------------------------
|
| A save sends the draft whole with the moment the client last changed
| it, so the server can keep the newest. A reply or note draft names its
| conversation; a new conversation draft names none.
|
*/

package requests

import "time"

/*
DraftRecipientsBody is who a draft will go to, staged before sending.
*/
type DraftRecipientsBody struct {
	To                  []string `json:"to,omitempty"                    maxItems:"50"`
	Cc                  []string `json:"cc,omitempty"                    maxItems:"50"`
	Bcc                 []string `json:"bcc,omitempty"                   maxItems:"50"`
	Subject             string   `json:"subject,omitempty"               validate:"omitempty,max=998"`
	ChannelConnectionID string   `json:"channel_connection_id,omitempty" validate:"omitempty,max=64"`
}

/*
DraftContentBody is what a draft holds.
*/
type DraftContentBody struct {
	Body            string               `json:"body"`
	AttachmentIDs   []string             `json:"attachment_ids,omitempty" maxItems:"10"`
	Recipients      *DraftRecipientsBody `json:"recipients,omitempty"`
	ClientUpdatedAt time.Time            `json:"client_updated_at"`
}

/*
StoreDraftRequest is the input of POST /drafts. Upsert updates the
draft already there instead of refusing it.
*/
type StoreDraftRequest struct {
	Upsert bool `query:"upsert"`
	Body   struct {
		DraftContentBody

		ConversationID *string `json:"conversation_id,omitempty"`
		Mode           string  `json:"mode"                      validate:"required,oneof=reply note new"`
	}
}

/*
DraftRequest names one draft.
*/
type DraftRequest struct {
	Draft string `path:"draft"`
}

/*
UpdateDraftRequest is the input of PATCH /drafts/{draft}.
*/
type UpdateDraftRequest struct {
	Draft string `path:"draft"`
	Body  DraftContentBody
}
