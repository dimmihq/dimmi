/*
|--------------------------------------------------------------------------
| Away Reply
|--------------------------------------------------------------------------
|
| An address may answer a new conversation that arrives outside office
| hours, saying when the team is back. It goes once, on the email that
| opened the conversation, only to a person: never to bulk mail, an
| automatic reply or a teammate's forward, and never where an AI agent
| is going to answer anyway. It is marked automatic, so the customer's
| mail system does not answer it back, and it is not an answer: the
| conversation still waits for the team.
|
*/

package email

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/schedules"
	"github.com/dimmihq/dimmi/app/ai/agents"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
replyTimeTag is where the away reply says when the team is back.
*/
const replyTimeTag = "{{reply_time}}"

/*
awayReply answers a new conversation that arrived outside office hours,
when the address asks for it.
*/
func (a *ProcessInboundEmail) awayReply(ctx context.Context, p persisting) error {
	settings := models.ReadEmailAddressSettings(p.mail.address.Settings)
	if !settings.AwayReply || p.conv.MessageCount != 1 || p.plan.aiSkip != "" || p.plan.closeNew ||
		p.insp.forward != nil || a.Queue == nil {
		return nil
	}

	back, away, err := a.awayUntil(ctx, p)
	if err != nil || !away {
		return err
	}

	answers, err := agentAnswers(ctx, p.q, p.conv)
	if err != nil || answers {
		return err
	}

	text := strings.ReplaceAll(settings.AwayReplyText, replyTimeTag, back)
	w := (&conversations.Writer{Q: p.q, Tx: p.tx, Events: a.Events, Now: a.Clock.Now()}).AsSystem()

	msg, _, err := w.Append(ctx, p.conv, conversations.NewMessage{
		Type: string(enums.MessageTypeMessage), Body: text, BodyFormat: new(string(enums.BodyFormatMarkdown)),
		DeliveryStatus: new(string(enums.DeliveryStatusPending)), Automatic: true,
		Metadata: []byte(`{"email_away_reply": true}`),
	})
	if err != nil {
		return fmt.Errorf("write away reply: %w", err)
	}

	err = a.Queue.PushTx(ctx, p.tx, jobs.SendChannelMessageArgs{
		WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID, MessageID: msg.ID,
	})
	if err != nil {
		return fmt.Errorf("queue away reply: %w", err)
	}

	return nil
}

/*
awayUntil reports whether the conversation's inbox is closed now, and
in words when it opens again.
*/
func (a *ProcessInboundEmail) awayUntil(ctx context.Context, p persisting) (string, bool, error) {
	row, err := schedules.ForInbox(ctx, p.q, schedules.Place{Workspace: p.conv.WorkspaceID, Inbox: p.conv.InboxID},
		a.Clock)
	if err != nil {
		return "", false, err
	}

	sched, err := schedules.Read(row)
	if err != nil {
		return "", false, err
	}

	now := a.Clock.Now()
	if sched.IsOpen(now) {
		return "", false, nil
	}

	if next, err := sched.NextOpen(now); err == nil {
		return sched.BackAt(now, next), true, nil
	}

	return "as soon as we can", true, nil
}

/*
agentAnswers reports whether an AI agent is going to answer the
conversation.
*/
func agentAnswers(ctx context.Context, q *models.Queries, conv models.Conversation) (bool, error) {
	agent, err := agents.Resolve(ctx, q, agents.ForConversation(conv, nil))
	if errors.Is(err, agents.ErrNoAgent) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return agent.Enabled && conv.AiState != string(enums.AIStateHandedOff), nil
}
