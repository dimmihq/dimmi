/*
|--------------------------------------------------------------------------
| Inbound Edits And Deletions
|--------------------------------------------------------------------------
|
| On a chat app the provider is the source of truth for what its users
| wrote. A customer who corrects a message there sees the correction in
| the inbox too, with the earlier words kept for the record; the edit
| never reopens anything and never asks the agent to answer again. A
| customer who deletes a message there keeps it here: the team still
| sees what was said, marked as deleted by its author. A conversation
| whose customer took back everything they said, and was never
| answered, closes, since nobody is waiting any more.
|
| Only what the customer or a teammate wrote natively on the channel can
| change this way; replies the team sent from the inbox never do.
|
*/

package conversations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
maxEdits is how many earlier versions of a message are kept.
*/
const maxEdits = 20

/*
ErrInboundMessageNotFound means the channel names a message the inbox
does not hold, or one it may not change.
*/
var ErrInboundMessageNotFound = errors.New("inbox: inbound message not found")

/*
InboundEdits applies edits and deletions made at a provider to the
messages it delivered.

Events: [appevents.MessageUpdated], and [appevents.ConversationClosed]
when a deletion leaves a conversation empty.
*/
type InboundEdits struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  clock.Clock
}

/*
Edit replaces the message's text, keeping the earlier one.
*/
func (a *InboundEdits) Edit(ctx context.Context, in contracts.InboundEdit) (models.Message, error) {
	var out models.Message

	err := a.within(ctx, in.ConnectionID, in.ChannelMessageID, func(
		ctx context.Context, w *Writer, msg models.Message,
	) error {
		if msg.Body != nil && *msg.Body == in.Body {
			out = msg

			return nil
		}

		metadata, err := withEdit(msg, in)
		if err != nil {
			return err
		}

		out, err = w.Q.ReplaceInboundMessageBody(ctx, models.ReplaceInboundMessageBodyParams{
			Body: &in.Body, Now: w.Now, Metadata: metadata, WorkspaceID: msg.WorkspaceID, ID: msg.ID,
		})
		if err != nil {
			return fmt.Errorf("edit inbound message: %w", err)
		}

		return w.Dispatch(ctx, appevents.MessageUpdated{
			Base: w.Base(msg.WorkspaceID), MessageRef: messageRef(msg),
		})
	})

	return out, err
}

/*
MarkDeleted records that the author deleted the message where they
wrote it, keeping its words and files, and closes a conversation whose
customer took back everything they said.
*/
func (a *InboundEdits) MarkDeleted(ctx context.Context, in contracts.InboundDeletion) (models.Message, error) {
	var out models.Message

	err := a.within(ctx, in.ConnectionID, in.ChannelMessageID, func(
		ctx context.Context, w *Writer, msg models.Message,
	) error {
		if sourceDeleted(msg) {
			out = msg

			return nil
		}

		mark, err := json.Marshal(map[string]any{"source_deleted": map[string]any{
			"by": msg.AuthorType, "reason": in.Reason, "at": w.Now,
		}})
		if err != nil {
			return fmt.Errorf("encode deletion: %w", err)
		}

		out, err = w.Q.MarkInboundMessageDeleted(ctx, models.MarkInboundMessageDeletedParams{
			Now: w.Now, Metadata: mark, WorkspaceID: msg.WorkspaceID, ID: msg.ID,
		})
		if err != nil {
			return fmt.Errorf("mark inbound message deleted: %w", err)
		}

		updated := appevents.MessageUpdated{Base: w.Base(msg.WorkspaceID), MessageRef: messageRef(msg)}
		if err := w.Dispatch(ctx, updated); err != nil {
			return err
		}

		return closeEmptied(ctx, w, msg)
	})

	return out, err
}

/*
within finds the message the connection knows by the id and runs the
change on it as the system, in one transaction. A message the team sent
from the inbox is not found.
*/
func (a *InboundEdits) within(
	ctx context.Context, connectionID ids.UUID, channelMessageID string,
	change func(ctx context.Context, w *Writer, msg models.Message) error,
) error {
	tenant, err := tenancy.Required(ctx)
	if err != nil {
		return err
	}

	workspaceID, err := ids.Parse(tenant)
	if err != nil {
		return fmt.Errorf("read workspace: %w", err)
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		msg, err := q.FindChannelMessage(ctx, models.FindChannelMessageParams{
			WorkspaceID: workspaceID, ChannelConnectionID: &connectionID, ChannelMessageID: &channelMessageID,
		})
		if errors.Is(err, database.ErrNotFound) || (err == nil && !nativeMessage(msg)) {
			return ErrInboundMessageNotFound
		}

		if err != nil {
			return fmt.Errorf("find inbound message: %w", err)
		}

		w := &Writer{Q: q, Tx: tx, Events: a.Events, Now: a.Clock.Now(), Actor: Actor{Type: "system", ID: ids.Nil()}}

		return change(ctx, w, msg)
	})
}

/*
nativeMessage reports whether the message was written on the channel
itself: by the customer, or by a teammate answering there.
*/
func nativeMessage(msg models.Message) bool {
	if msg.DeletedAt != nil || msg.Type != string(enums.MessageTypeMessage) {
		return false
	}

	if msg.AuthorType == string(enums.AuthorTypeContact) {
		return true
	}

	var meta struct {
		Echo bool `json:"echo"`
	}

	return msg.AuthorType == string(enums.AuthorTypeMember) && json.Unmarshal(msg.Metadata, &meta) == nil && meta.Echo
}

/*
withEdit returns the message's metadata with its current text added to
the earlier versions, the oldest dropped past the limit.
*/
func withEdit(msg models.Message, in contracts.InboundEdit) (json.RawMessage, error) {
	meta := map[string]any{}
	if len(msg.Metadata) > 0 {
		if err := json.Unmarshal(msg.Metadata, &meta); err != nil {
			return nil, fmt.Errorf("read message metadata: %w", err)
		}
	}

	previous, _ := meta["edits"].([]any)
	previous = append(previous, map[string]any{"body": deref(msg.Body), "edited_at": in.EditedAt})

	if len(previous) > maxEdits {
		previous = previous[len(previous)-maxEdits:]
	}

	meta["edits"], meta["edited_at"] = previous, in.EditedAt

	encoded, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("encode message metadata: %w", err)
	}

	return encoded, nil
}

/*
sourceDeleted reports whether the author already deleted the message
at the provider.
*/
func sourceDeleted(msg models.Message) bool {
	var meta struct {
		SourceDeleted map[string]any `json:"source_deleted"`
	}

	return json.Unmarshal(msg.Metadata, &meta) == nil && meta.SourceDeleted != nil
}

/*
closeEmptied closes the message's conversation when nothing the
customer said is left in it and nobody answered.
*/
func closeEmptied(ctx context.Context, w *Writer, msg models.Message) error {
	voices, err := w.Q.CountConversationVoices(ctx, models.CountConversationVoicesParams{
		WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID,
	})
	if err != nil {
		return fmt.Errorf("count conversation messages: %w", err)
	}

	if voices.ContactMessages > 0 || voices.Replies > 0 {
		return nil
	}

	conv, err := w.Q.FindConversation(ctx, models.FindConversationParams{
		WorkspaceID: msg.WorkspaceID, ID: msg.ConversationID,
	})
	if err != nil {
		return fmt.Errorf("find conversation: %w", err)
	}

	_, _, err = w.ChangeStatus(ctx, conv, StatusChange{
		Status: string(enums.ConversationStatusClosed), Reason: ReasonSourceDeleted,
	})

	return err
}

/*
messageRef names the message for its events.
*/
func messageRef(msg models.Message) appevents.MessageRef {
	return appevents.MessageRef{
		ConversationRef: appevents.ConversationRef{ConversationID: msg.ConversationID}, MessageID: msg.ID,
	}
}

/*
deref returns the text, or empty for none.
*/
func deref(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
