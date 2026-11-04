/*
|--------------------------------------------------------------------------
| Message Requests
|--------------------------------------------------------------------------
|
| Inputs of the endpoints that write to a conversation's timeline. A
| reply may say what the conversation does afterwards; a snooze comes
| only with snoozing, and then with exactly one way to say when it
| ends. A client message id makes a retried request find its first
| entry instead of writing a second one.
|
*/

package requests

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/validation"
)

/*
SnoozeBody is when a snooze ends: a time or a preset.
*/
type SnoozeBody struct {
	Until  *time.Time `json:"until,omitempty"`
	Preset *string    `json:"preset,omitempty"`
	Option *string    `json:"option,omitempty" validate:"omitempty,max=40"`
}

/*
ReplyRequest is the input of POST /conversations/{conversation}/replies.
*/
type ReplyRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		Body                  string      `json:"body"                               validate:"required,max=65536"`
		ClientMessageID       *string     `json:"client_message_id,omitempty"        validate:"omitempty,min=1,max=64"`
		ExpectedLastMessageID *string     `json:"expected_last_message_id,omitempty"`
		Then                  *string     `json:"then,omitempty"`
		Snooze                *SnoozeBody `json:"snooze,omitempty"`
	}
}

/*
Validate refuses a snooze without a time or preset, or one sent without
snoozing.
*/
func (r *ReplyRequest) Validate(ctx context.Context, _ *validation.Factory) error {
	snoozing := r.Body.Then != nil && *r.Body.Then == "snooze"
	s := r.Body.Snooze

	if err := choice(ctx, "then", r.Body.Then, "keep open close snooze"); err != nil {
		return err
	}

	if s != nil {
		if err := choice(ctx, "snooze.preset", s.Preset, snoozePresets); err != nil {
			return err
		}
	}

	if !snoozing && s != nil {
		return violation(ctx, "snooze", "prohibited", "validation.prohibited")
	}

	if snoozing && (s == nil || countSet(s.Until != nil, s.Preset != nil, s.Option != nil) != 1) {
		return violation(ctx, "snooze", "required", "validation.required")
	}

	return nil
}

/*
NoteRequest is the input of POST /conversations/{conversation}/notes.
*/
type NoteRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		Body            string  `json:"body"                        validate:"required,max=65536"`
		ClientMessageID *string `json:"client_message_id,omitempty" validate:"omitempty,min=1,max=64"`
	}
}

/*
ContactMessageRequest is the input of POST
/conversations/{conversation}/messages.
*/
type ContactMessageRequest struct {
	Conversation string `path:"conversation"`
	Body         struct {
		Body      string  `json:"body"                 validate:"required,max=65536"`
		DedupeKey *string `json:"dedupe_key,omitempty" validate:"omitempty,min=1,max=200"`
	}
}

/*
ConversationMessageRequest is the input of the endpoints that name a
message of a conversation.
*/
type ConversationMessageRequest struct {
	Conversation string `path:"conversation"`
	Message      string `path:"message"`
}
