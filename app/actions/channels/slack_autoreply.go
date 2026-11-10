/*
|--------------------------------------------------------------------------
| Slack Auto-Reply
|--------------------------------------------------------------------------
|
| A customer room may promise a reply when nobody answered for a while:
| a short message in the thread after the room's delay, always or only
| outside office hours. It is never sent when anyone of the team, or the
| agent, answered first, and never where the agent is going to answer
| anyway. It wakes nobody: it is not an answer, so the conversation
| still waits for one.
|
*/

package channels

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/queue"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/app/support/officehours"
)

/*
scheduleAutoReply queues the room's auto-reply for a new conversation,
after the room's delay.
*/
func (s *Slack) scheduleAutoReply(ctx context.Context, args jobs.ApplySlackEffectArgs) error {
	conv, err := s.findConversation(ctx, args.WorkspaceID, args.ConversationID)
	if err != nil {
		return ignoreMissing(err)
	}

	room, ok, err := s.roomOfConversation(ctx, conv)
	if err != nil || !ok || !customerRoom(&room) {
		return err
	}

	seconds := numberOf(&room, settingAutoReplyAfter)
	if seconds <= 0 {
		return nil
	}

	at := s.Clock.Now().Add(time.Duration(min(seconds, maxDelaySeconds)) * time.Second)
	args.Effect = jobs.SlackEffectAutoReply

	return s.Queue.Push(ctx, args, queue.At(at))
}

/*
autoReply posts the room's auto-reply when nobody answered yet.
*/
func (s *Slack) autoReply(ctx context.Context, conv models.Conversation) error {
	room, ok, err := s.roomOfConversation(ctx, conv)
	if err != nil || !ok || !customerRoom(&room) || conv.FirstReplyAt != nil ||
		conv.Status == string(enums.ConversationStatusClosed) {
		return err
	}

	due, err := s.autoReplyDue(ctx, conv, room)
	if err != nil || !due {
		return err
	}

	text := cmp.Or(settingOf(&room, settingAutoReplyText), defaultAutoReplyText)

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w := (&conversations.Writer{Q: models.New(tx), Tx: tx, Events: s.Events, Now: s.Clock.Now()}).AsSystem()

		msg, _, err := w.Append(ctx, conv, conversations.NewMessage{
			Type: string(enums.MessageTypeMessage), Body: text, BodyFormat: new(string(enums.BodyFormatMarkdown)),
			DeliveryStatus: new(string(enums.DeliveryStatusPending)), Automatic: true,
			Metadata: []byte(`{"slack_auto_reply": true}`),
		})
		if err != nil {
			return fmt.Errorf("post auto-reply: %w", err)
		}

		return s.Queue.PushTx(ctx, tx, jobs.SendChannelMessageArgs{
			WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID, MessageID: msg.ID,
		})
	})
}

/*
autoReplyDue reports whether the auto-reply goes out now: the agent is
not going to answer, and the room's hours allow it.
*/
func (s *Slack) autoReplyDue(ctx context.Context, conv models.Conversation, room models.ChannelRoom) (bool, error) {
	var agent models.AiAgent

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		agent, err = q.FindAgent(ctx, conv.WorkspaceID)

		return err
	})

	switch {
	case errors.Is(err, database.ErrNotFound):
		return settingOf(&room, settingAutoReplyWhen) != whenOutsideHours, nil
	case err != nil:
		return false, fmt.Errorf("find agent: %w", err)
	case agent.Enabled && room.AgentEnabled && conv.AiState != "handed_off":
		return false, nil
	case settingOf(&room, settingAutoReplyWhen) != whenOutsideHours:
		return true, nil
	}

	within, err := officehours.Within(agent.OfficeHours, s.Clock.Now())

	return err == nil && !within, nil
}
