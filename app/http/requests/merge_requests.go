/*
|--------------------------------------------------------------------------
| Merge And Bulk Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that change many conversations at once and
| that merge conversations or contacts. A bulk change carries either the
| fields of a change to one conversation or a move to the trash, never
| both. Its raw body is kept to tell an assignment to nobody from one
| left out.
|
*/

package requests

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
UpdateConversationRequestBody are the fields a change to a conversation may
carry, alone or together.
*/
type UpdateConversationRequestBody struct {
	Spam         *bool      `json:"spam,omitempty"`
	AssigneeID   *string    `json:"assignee_id,omitempty"`
	TeamID       *string    `json:"team_id,omitempty"`
	Status       *string    `json:"status,omitempty"        validate:"omitempty,oneof=open closed snoozed"`
	SnoozedUntil *time.Time `json:"snoozed_until,omitempty"`
	SnoozePreset *string    `json:"snooze_preset,omitempty"`
	SnoozeOption *string    `json:"snooze_option,omitempty" validate:"omitempty,max=40"`
	Note         *string    `json:"note,omitempty"          validate:"omitempty,min=1,max=65536"`
	Subject      *string    `json:"subject,omitempty"       validate:"omitempty,min=1,max=255"`
	Priority     *string    `json:"priority,omitempty"      validate:"omitempty,oneof=none low medium high urgent"`
	InboxID      *string    `json:"inbox_id,omitempty"`
	Unread       *bool      `json:"unread,omitempty"`
	AddTags      []string   `json:"add_tags,omitempty"      validate:"omitempty,max=50,dive,min=1,max=100"`
	RemoveTags   []string   `json:"remove_tags,omitempty"   validate:"omitempty,max=50"`
}

/*
BulkConversationChangesRequest is the input of POST
/conversation-bulk-changes.
*/
type BulkConversationChangesRequest struct {
	Body struct {
		ConversationIDs []string `json:"conversation_ids" validate:"required,min=1,max=200,dive,min=1,max=64"`

		Changes *UpdateConversationRequestBody `json:"changes,omitempty"`
		Trash   *bool                          `json:"trash,omitempty"`
	}
	RawBody []byte
}

/*
Validate refuses a body with neither changes nor a move to the trash,
or with both, and changes that would be refused on one conversation.
*/
func (r *BulkConversationChangesRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	b := r.Body
	trash := b.Trash != nil && *b.Trash

	if trash == (b.Changes != nil) {
		return violation(ctx, "changes", "required", "validation.required")
	}

	if b.Changes != nil {
		return b.Changes.check(ctx)
	}

	return nil
}

/*
SentChanges names the fields the body's changes carried.
*/
func (r *BulkConversationChangesRequest) SentChanges() map[string]bool {
	var body struct {
		Changes json.RawMessage `json:"changes"`
	}

	_ = json.Unmarshal(r.RawBody, &body)

	return Sent(body.Changes)
}

/*
MergeConversationsRequest is the input of POST
/conversations/{conversation}/merges: the conversations to fold into
the one the path names.
*/
type MergeConversationsRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		SourceIDs []string `json:"source_ids" validate:"required,min=1,max=20,dive,min=1,max=64"`
	}
}

/*
MergeContactRequest is the input of POST /contacts/{contact}/merges:
the contact to fold into the one the path names.
*/
type MergeContactRequest struct {
	Contact string `path:"contact"`
	Body    struct {
		SourceID string `json:"source_id" validate:"required,min=1,max=64"`
	}
}
