/*
|--------------------------------------------------------------------------
| Visitor's Agent Controls
|--------------------------------------------------------------------------
|
| The two buttons under every AI reply: "That helped" records that the
| agent's latest answer helped, and "Talk to a person" hands the
| conversation to the team. Both touch the visitor's own conversation
| only; any other is simply not found. Handing over happens once: a
| conversation already with the team stays as it is, and a visitor
| pressing the button over and over is slowed down.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/ratelimit"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/auth"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/models"
)

/*
handoffPerMinute is how many handoff requests one session may send a
minute.
*/
const handoffPerMinute = 5

/*
HelpInput names the conversation the visitor confirmed.
*/
type HelpInput struct {
	Auth         Authenticated
	Conversation ids.UUID
}

/*
ConfirmAiHelp records the visitor's "That helped" on the newest answer
the agent gave in their conversation. Pressing it again changes
nothing, and a conversation the agent never answered has nothing to
record.
*/
type ConfirmAiHelp struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle records the feedback.
*/
func (a *ConfirmAiHelp) Handle(ctx context.Context, in HelpInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, err := models.New(tx).FindWidgetConversation(ctx, scopeOf(in.Auth, in.Conversation))
		if err != nil {
			return exceptions.ErrWidgetConversationNotFound
		}

		_, err = models.New(tx).SetLatestAnswerFeedback(ctx, models.SetLatestAnswerFeedbackParams{
			WorkspaceID: found.Conversation.WorkspaceID, ConversationID: &found.Conversation.ID,
			Feedback: new("helped"), Now: a.Clock.Now(),
		})
		if err != nil {
			return fmt.Errorf("record answer feedback: %w", err)
		}
		return nil
	})
}

/*
HandoffInput names the conversation the visitor wants a person for.
*/
type HandoffInput struct {
	Auth         Authenticated
	Conversation ids.UUID
}

/*
RequestAiHandoff is the visitor's "talk to a person": the agent says
its one line, the conversation flips to handed_off and the team hears
about it. A conversation already handed off is left alone, so a double
click writes one line and one notice.
*/
type RequestAiHandoff struct {
	DB      *database.DB
	Clock   clock.Clock
	Events  *events.Dispatcher
	Limiter *ratelimit.Limiter
}

/*
Handle hands the conversation to the team.
*/
func (a *RequestAiHandoff) Handle(ctx context.Context, in HandoffInput) error {
	if err := a.throttle(ctx, in.Auth); err != nil {
		return err
	}

	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		q := models.New(tx)

		found, err := q.FindWidgetConversation(ctx, scopeOf(in.Auth, in.Conversation))
		if err != nil {
			return exceptions.ErrWidgetConversationNotFound
		}

		conv, err := q.LockConversation(ctx, models.LockConversationParams{
			WorkspaceID: found.Conversation.WorkspaceID, ID: found.Conversation.ID,
		})
		if err != nil {
			return exceptions.ErrWidgetConversationNotFound
		}
		if conv.AiState == "handed_off" {
			return nil
		}

		return a.handOff(ctx, tx, conv)
	})
}

/*
throttle counts the request against the session's minute.
*/
func (a *RequestAiHandoff) throttle(ctx context.Context, who Authenticated) error {
	if a.Limiter == nil {
		return nil
	}

	return auth.Throttle(ctx, a.Limiter, auth.Rate{
		Key: "widget-handoff:" + who.Session.ID.String(), Limit: handoffPerMinute, Window: time.Minute,
	})
}

/*
handOff writes the agent's handoff line, flips the conversation and
tells the team, in the caller's transaction.
*/
func (a *RequestAiHandoff) handOff(ctx context.Context, tx *database.Tx, conv models.Conversation) error {
	q := models.New(tx)
	now := a.Clock.Now()

	agent, err := q.FindAgent(ctx, conv.WorkspaceID)
	if err != nil {
		return exceptions.ErrWidgetConversationNotFound
	}

	writer := &conversations.Writer{
		Q: q, Tx: tx, Events: a.Events, Now: now,
		Actor: conversations.Actor{Type: "agent", ID: agent.ID, Name: agent.Name},
	}
	if _, _, err := writer.Append(ctx, conv, conversations.NewMessage{
		Type: "message", Body: handoffLine(agent.Messages),
	}); err != nil {
		return fmt.Errorf("write handoff line: %w", err)
	}

	if err := q.HandoffConversationAi(ctx, models.HandoffConversationAiParams{
		WorkspaceID: conv.WorkspaceID, ID: conv.ID, Reason: new("customer_requested"), Now: &now,
	}); err != nil {
		return fmt.Errorf("handoff conversation: %w", err)
	}

	return a.Events.Dispatch(ctx, tx, appevents.AiHandoffRequested{
		Base: appevents.Base{
			ID: ids.New(), WorkspaceID: conv.WorkspaceID,
			OccurredAt: now, Actor: appevents.Actor{Type: "contact", ID: conv.ContactID},
		},
		ConversationRef: appevents.ConversationRef{ConversationID: conv.ID},
		Reason:          "customer_requested",
	})
}

/*
handoffLine reads the agent's configured line, with its default.
*/
func handoffLine(raw json.RawMessage) string {
	line := "I'm bringing in someone from the team."
	var msgs map[string]any
	if err := json.Unmarshal(raw, &msgs); err == nil {
		if v, ok := msgs["handoff_line"].(string); ok && v != "" {
			return v
		}
	}
	return line
}
