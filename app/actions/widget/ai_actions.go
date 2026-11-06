/*
|--------------------------------------------------------------------------
| Visitor's Agent Controls
|--------------------------------------------------------------------------
|
| The two buttons under every AI reply: "That helped" confirms the
| answer resolved the conversation, and "Talk to a person" hands it
| to the team. Both touch the visitor's own conversation only.
|
*/

package widget

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
)

/*
errNotFound is the visitor-facing "no such conversation".
*/
var errNotFound = errors.New("not found")

/*
HelpInput names the conversation the visitor confirmed.
*/
type HelpInput struct {
	Auth         Authenticated
	Conversation ids.UUID
}

/*
ConfirmAiHelp marks the newest finished run of the visitor's
conversation as confirmed-resolved: the visitor said it helped.
*/
type ConfirmAiHelp struct {
	DB    *database.DB
	Clock clock.Clock
}

/*
Handle stamps the resolution on the latest run.
*/
func (a *ConfirmAiHelp) Handle(ctx context.Context, in HelpInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, err := models.New(tx).FindWidgetConversation(ctx, scopeOf(in.Auth, in.Conversation))
		if err != nil {
			return errNotFound
		}

		now := a.Clock.Now()
		_, err = models.New(tx).ResolveLatestRunOutcome(ctx, models.ResolveLatestRunOutcomeParams{
			WorkspaceID: found.Conversation.WorkspaceID, ConversationID: &found.Conversation.ID,
			Outcome: strPtr("resolved_confirmed"), Now: &now,
		})
		if err != nil {
			return fmt.Errorf("resolve run: %w", err)
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
about it.
*/
type RequestAiHandoff struct {
	DB     *database.DB
	Clock  clock.Clock
	Events *events.Dispatcher
}

/*
Handle hands the conversation to the team.
*/
func (a *RequestAiHandoff) Handle(ctx context.Context, in HandoffInput) error {
	return a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		found, err := models.New(tx).FindWidgetConversation(ctx, scopeOf(in.Auth, in.Conversation))
		if err != nil {
			return errNotFound
		}

		agent, err := models.New(tx).FindAgent(ctx, found.Conversation.WorkspaceID)
		if err != nil {
			return errNotFound
		}

		line := "I'm bringing in someone from the team."
		var msgs map[string]any
		if err := json.Unmarshal(agent.Messages, &msgs); err == nil {
			if v, ok := msgs["handoff_line"].(string); ok && v != "" {
				line = v
			}
		}

		writer := &conversations.Writer{
			Q: models.New(tx), Tx: tx, Events: a.Events, Now: a.Clock.Now(),
			Actor: conversations.Actor{Type: "agent", ID: agent.ID, Name: agent.Name},
		}

		conv, err := models.New(tx).FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: found.Conversation.WorkspaceID, ID: found.Conversation.ID,
		})
		if err != nil {
			return errNotFound
		}

		if _, _, err := writer.Append(ctx, conv, conversations.NewMessage{
			Type: "message", Body: line,
		}); err != nil {
			return fmt.Errorf("write handoff line: %w", err)
		}

		now := a.Clock.Now()
		if err := models.New(tx).HandoffConversationAi(ctx, models.HandoffConversationAiParams{
			WorkspaceID: found.Conversation.WorkspaceID, ID: found.Conversation.ID,
			Reason: strPtr("customer_requested"), Now: &now,
		}); err != nil {
			return fmt.Errorf("handoff conversation: %w", err)
		}

		return a.Events.Dispatch(ctx, tx, appevents.AiHandoffRequested{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: found.Conversation.WorkspaceID,
				OccurredAt: now, Actor: appevents.Actor{Type: "contact", ID: found.Conversation.ContactID},
			},
			ConversationRef: appevents.ConversationRef{ConversationID: found.Conversation.ID},
			Reason:          "customer_requested",
		})
	})
}

/*
strPtr is a string pointer.
*/
func strPtr(s string) *string { return &s }
