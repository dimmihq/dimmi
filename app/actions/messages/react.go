/*
|--------------------------------------------------------------------------
| Note Reactions
|--------------------------------------------------------------------------
|
| Teammates react to notes with emoji, never to what the customer sees.
| Each teammate adds or takes off only their own reaction, and doing it
| twice changes nothing. A note holds at most 20 different emoji. A
| reaction is one emoji: short, with no letters, digits or spaces, so it
| cannot carry a message of its own. The note keeps a summary of its
| reactions for showing, and the conversation's screens hear about each
| change.
|
*/

package messages

import (
	"context"
	"errors"
	"fmt"
	"unicode"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Limits of reactions: the most different emoji on one note and the
longest emoji in bytes.
*/
const (
	MaxReactionEmoji = 20
	maxEmojiBytes    = 32
)

/*
ReactInput is who reacts to which note of which conversation, with
which emoji, and whether they take it off. MutationID is the id the
client gave the change.
*/
type ReactInput struct {
	Membership authz.Membership
	Ref        conversations.Ref
	MessageID  ids.UUID
	Emoji      string
	Remove     bool
	MutationID string
}

/*
ReactToNote adds the caller's emoji reaction to a note, or takes it off.

Rules:
  - The caller needs conversations:write and sees the conversation.
  - Only notes take reactions; a deleted one is not found.
  - At most MaxReactionEmoji different emoji per note.
  - Adding a reaction already there, or removing one that is not,
    changes nothing.

Events: [appevents.MessageReactionsChanged] when something changed.

Errors: [exceptions.ErrForbidden], [exceptions.ErrNotFound],
[exceptions.ErrMessageNotReactable], [exceptions.ErrReactionLimit],
[exceptions.ErrInvalidReaction], or a wrapped database error.
*/
type ReactToNote struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Handle changes the reaction and returns the note.
*/
func (a *ReactToNote) Handle(ctx context.Context, in ReactInput) (Written, error) {
	if err := conversations.CanWrite(in.Membership); err != nil {
		return Written{}, err
	}

	if !IsEmoji(in.Emoji) {
		return Written{}, exceptions.ErrInvalidReaction
	}

	var out Written

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		s, err := open(ctx, tx, conversations.Services{Events: a.Events, Clock: a.Clock}, target{
			in.Membership, in.Ref, in.MutationID,
		})
		if err != nil {
			return err
		}

		if err := s.react(ctx, in); err != nil {
			return err
		}

		out, err = s.written(ctx)

		return err
	})

	return out, err
}

/*
IsEmoji reports whether the text may be a reaction: at most 32 bytes,
with no ASCII letters or digits and no white space.
*/
func IsEmoji(text string) bool {
	if text == "" || len(text) > maxEmojiBytes {
		return false
	}

	for _, r := range text {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) || unicode.IsSpace(r) {
			return false
		}
	}

	return true
}

/*
react changes the caller's reaction on the note and, when something
changed, refreshes the note's summary and announces it.
*/
func (s *session) react(ctx context.Context, in ReactInput) error {
	msg, err := s.w.Q.LockMessage(ctx, models.LockMessageParams{
		WorkspaceID: s.c.WorkspaceID, ConversationID: s.c.ID, ID: in.MessageID,
	})
	if errors.Is(err, database.ErrNotFound) || (err == nil && msg.DeletedAt != nil) {
		return exceptions.ErrNotFound
	}

	if err != nil {
		return fmt.Errorf("find note: %w", err)
	}

	if msg.Type != string(enums.MessageTypeNote) {
		return exceptions.ErrMessageNotReactable
	}

	s.msg = msg

	changed, err := s.toggle(ctx, in)
	if err != nil || !changed {
		return err
	}

	if s.msg, err = s.w.Q.SummarizeMessageReactions(ctx, models.SummarizeMessageReactionsParams{
		Now: s.w.Now, WorkspaceID: s.c.WorkspaceID, ID: msg.ID,
	}); err != nil {
		return fmt.Errorf("summarize reactions: %w", err)
	}

	r := appevents.MessageRef{ConversationRef: appevents.ConversationRef{ConversationID: s.c.ID}, MessageID: msg.ID}

	return s.w.Dispatch(ctx, appevents.MessageReactionsChanged{Base: s.w.Base(s.c.WorkspaceID), MessageRef: r})
}

/*
toggle adds or removes the caller's emoji and reports whether that
changed anything.
*/
func (s *session) toggle(ctx context.Context, in ReactInput) (bool, error) {
	if in.Remove {
		n, err := s.w.Q.RemoveMessageReaction(ctx, models.RemoveMessageReactionParams{
			WorkspaceID: s.c.WorkspaceID, MessageID: s.msg.ID, MemberID: s.m.Member.ID, Emoji: in.Emoji,
		})
		if err != nil {
			return false, fmt.Errorf("remove reaction: %w", err)
		}

		return n > 0, nil
	}

	seen, err := s.w.Q.HasMessageReactionEmoji(ctx, models.HasMessageReactionEmojiParams{
		WorkspaceID: s.c.WorkspaceID, MessageID: s.msg.ID, Emoji: in.Emoji,
	})
	if err != nil {
		return false, fmt.Errorf("count reactions: %w", err)
	}

	if !seen.Present && seen.DistinctEmoji >= MaxReactionEmoji {
		return false, exceptions.ErrReactionLimit
	}

	n, err := s.w.Q.AddMessageReaction(ctx, models.AddMessageReactionParams{
		WorkspaceID: s.c.WorkspaceID, MessageID: s.msg.ID, MemberID: s.m.Member.ID, Emoji: in.Emoji, Now: s.w.Now,
	})
	if err != nil {
		return false, fmt.Errorf("add reaction: %w", err)
	}

	return n > 0, nil
}
