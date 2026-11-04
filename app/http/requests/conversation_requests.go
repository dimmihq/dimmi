/*
|--------------------------------------------------------------------------
| Conversation Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints on conversations and their timelines. A
| conversation is named by its public id or by its number. A change may
| carry several fields at once; side fields only make sense with the
| field they belong to, so a snooze time comes only with snoozing, and a
| closing note only with closing. An assignment sent as null hands the
| conversation to nobody, which is why changes also keep the raw body,
| to tell null from missing.
|
*/

package requests

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
ConversationRequest is the input of the endpoints that name only a
conversation.
*/
type ConversationRequest struct {
	Conversation string `path:"conversation"`
}

/*
ContactRef names the customer of a new conversation: by id, or by
external id or email with the name to use when they are new.
*/
type ContactRef struct {
	ID         *string `json:"id,omitempty"`
	ExternalID *string `json:"external_id,omitempty" validate:"omitempty,min=1,max=255"`
	Email      *string `json:"email,omitempty"       validate:"omitempty,email,max=320"`
	Name       *string `json:"name,omitempty"        validate:"omitempty,max=255"`
}

/*
StoreConversationRequest is the input of POST /conversations.
*/
type StoreConversationRequest struct {
	Body struct {
		InboxID             *string    `json:"inbox_id,omitempty"`
		ChannelConnectionID *string    `json:"channel_connection_id,omitempty"`
		Contact             ContactRef `json:"contact"`
		Author              *string    `json:"author,omitempty"      validate:"omitempty,oneof=contact member"`
		Subject             *string    `json:"subject,omitempty"     validate:"omitempty,min=1,max=255"`
		Body                string     `json:"body"                  validate:"required,max=65536"`
		AssigneeID          *string    `json:"assignee_id,omitempty"`
	}
	RawBody []byte
}

/*
UpdateConversationRequest is the input of PATCH /conversations/{conversation}.
*/
type UpdateConversationRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		Spam         *bool      `json:"spam,omitempty"`
		AssigneeID   *string    `json:"assignee_id,omitempty"`
		TeamID       *string    `json:"team_id,omitempty"`
		Status       *string    `json:"status,omitempty"        validate:"omitempty,oneof=open closed snoozed"`
		SnoozedUntil *time.Time `json:"snoozed_until,omitempty"`
		SnoozePreset *string    `json:"snooze_preset,omitempty"`
		Note         *string    `json:"note,omitempty"          validate:"omitempty,min=1,max=65536"`
		Subject      *string    `json:"subject,omitempty"       validate:"omitempty,min=1,max=255"`
		Priority     *bool      `json:"priority,omitempty"`
		InboxID      *string    `json:"inbox_id,omitempty"`
		Unread       *bool      `json:"unread,omitempty"`
		AddTags      []string   `json:"add_tags,omitempty"      validate:"omitempty,max=50,dive,min=1,max=100"`
		RemoveTags   []string   `json:"remove_tags,omitempty"   validate:"omitempty,max=50"`
	}
	RawBody []byte
}

/*
Validate refuses a side field without its field, and a snooze without
exactly one way to say when it ends.
*/
func (r *UpdateConversationRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	b := r.Body
	snoozing := b.Status != nil && *b.Status == "snoozed"

	if err := choice(ctx, "snooze_preset", b.SnoozePreset, snoozePresets); err != nil {
		return err
	}

	if !snoozing && b.SnoozedUntil != nil {
		return violation(ctx, "snoozed_until", "prohibited", "validation.prohibited")
	}

	if !snoozing && b.SnoozePreset != nil {
		return violation(ctx, "snooze_preset", "prohibited", "validation.prohibited")
	}

	if snoozing && (b.SnoozedUntil == nil) == (b.SnoozePreset == nil) {
		return violation(ctx, "snoozed_until", "required", "validation.required")
	}

	if b.Note != nil && (b.Status == nil || *b.Status != "closed") {
		return violation(ctx, "note", "prohibited", "validation.prohibited")
	}

	return nil
}

/*
snoozePresets are the presets a snooze may end at.
*/
const snoozePresets = "later_today tomorrow next_week one_week"

/*
choice refuses a value that is not one of the allowed ones, listed
with spaces between them.
*/
func choice(ctx context.Context, field string, value *string, allowed string) error {
	if value == nil || slices.Contains(strings.Fields(allowed), *value) {
		return nil
	}

	return &validation.Error{Violations: []validation.Violation{{
		Field: field, Rule: "oneof",
		Message: i18n.T(ctx, "validation.oneof", i18n.Args{"field": field, "param": allowed}),
	}}}
}

/*
ReadStateRequest is the input of PATCH
/conversations/{conversation}/read-state. An empty body reads to the
latest entry.
*/
type ReadStateRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		LastReadMessageID *string `json:"last_read_message_id,omitempty"`
		Unread            *bool   `json:"unread,omitempty"`
	} `required:"false"`
}

/*
Validate refuses a body that both reads up to an entry and sets unread.
*/
func (r *ReadStateRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	if r.Body.LastReadMessageID != nil && r.Body.Unread != nil {
		return violation(ctx, "unread", "prohibited", "validation.prohibited")
	}

	return nil
}
