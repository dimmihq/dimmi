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
		CreatedAt           *time.Time `json:"created_at,omitempty"`
		ExternalID          *string    `json:"external_id,omitempty" validate:"omitempty,min=1,max=255"`
		Status              *string    `json:"status,omitempty"      validate:"omitempty,oneof=open closed"`
		ClosedAt            *time.Time `json:"closed_at,omitempty"`
		AuthorID            *string    `json:"author_id,omitempty"`
	}
	RawBody []byte
}

/*
Importing reports whether the request brings in a conversation from
another tool, which any of the import fields says.
*/
func (r *StoreConversationRequest) Importing() bool {
	b := r.Body

	return b.CreatedAt != nil || b.ExternalID != nil || b.Status != nil || b.ClosedAt != nil || b.AuthorID != nil
}

/*
Validate refuses a close time without closing, and a teammate author on
a message the customer wrote.
*/
func (r *StoreConversationRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	b := r.Body

	if b.ClosedAt != nil && (b.Status == nil || *b.Status != "closed") {
		return violation(ctx, "closed_at", "prohibited", "validation.prohibited")
	}

	if b.AuthorID != nil && b.Author != nil && *b.Author == "contact" {
		return violation(ctx, "author_id", "prohibited", "validation.prohibited")
	}

	return nil
}

/*
UpdateConversationRequest is the input of PATCH /conversations/{conversation}.
*/
type UpdateConversationRequest struct {
	Conversation string `path:"conversation"`
	Body         UpdateConversationRequestBody
	RawBody      []byte
}

/*
Validate refuses a side field without its field, and a snooze without
exactly one way to say when it ends.
*/
func (r *UpdateConversationRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	return r.Body.check(ctx)
}

/*
check refuses a side field without its field, and a snooze without
exactly one way to say when it ends.
*/
func (b UpdateConversationRequestBody) check(ctx context.Context) error {
	snoozing := b.Status != nil && *b.Status == "snoozed"

	if err := choice(ctx, "snooze_preset", b.SnoozePreset, snoozePresets); err != nil {
		return err
	}

	if !snoozing && b.SnoozedUntil != nil {
		return violation(ctx, "snoozed_until", "prohibited", "validation.prohibited")
	}

	if !snoozing && (b.SnoozePreset != nil || b.SnoozeOption != nil) {
		return violation(ctx, "snooze_preset", "prohibited", "validation.prohibited")
	}

	if snoozing && countSet(b.SnoozedUntil != nil, b.SnoozePreset != nil, b.SnoozeOption != nil) != 1 {
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

/*
countSet counts the fields sent.
*/
func countSet(sent ...bool) int {
	n := 0

	for _, s := range sent {
		if s {
			n++
		}
	}

	return n
}
