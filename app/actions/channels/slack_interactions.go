/*
|--------------------------------------------------------------------------
| Slack Buttons And Shortcuts
|--------------------------------------------------------------------------
|
| The buttons under an agent's answer are the customer's way out.
| Solved closes the conversation, but only for the person who asked, so
| someone else in a public thread cannot close another's question. Talk
| to a human hands the conversation to the team, for anyone, once. A
| click names the answer it came from; one naming an answer of another
| connection, or of nothing, does nothing. Either way the buttons go,
| and the press is kept on the run that wrote the answer.
|
| The message shortcut opens a conversation from any customer message,
| whatever the room's trigger. Using it on a message already in one
| points to that conversation instead of opening a second.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
answered is an answer a control was pressed on, with its conversation.
*/
type answered struct {
	message models.Message
	conv    models.Conversation
}

/*
onControl handles a press of an answer control.
*/
func (s *Slack) onControl(ctx context.Context, in SlackInteraction) (bool, error) {
	publicID, control, ok := slackdriver.ParseControlActionID(in.Action.ActionID)
	if !ok {
		return false, nil
	}

	found, ok, err := s.answerOf(ctx, in.Connection, publicID)
	if err != nil || !ok {
		return false, err
	}

	switch control {
	case contracts.ControlSolved:
		err = s.solve(ctx, in, found)
	case contracts.ControlHandoff:
		err = s.handOff(ctx, found.conv)
	default:
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if err := s.feedbackOn(ctx, found.message, controlFeedback(control)); err != nil {
		return false, err
	}

	return true, s.clearControls(ctx, in.Connection, found.message)
}

/*
answerOf finds the answer a control names, on the connection, reporting
false for any other.
*/
func (s *Slack) answerOf(ctx context.Context, conn models.ChannelConnection, publicID string) (answered, bool, error) {
	messageID, decodeErr := s.IDs.Decode(string(models.PrefixMessage), publicID)
	if decodeErr != nil {
		return answered{}, false, ignoreUnreadable(decodeErr)
	}

	var found answered

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		found.message, err = q.FindWorkspaceMessage(ctx, models.FindWorkspaceMessageParams{
			WorkspaceID: conn.WorkspaceID, ID: messageID,
		})
		if err != nil {
			return err
		}

		found.conv, err = q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: conn.WorkspaceID, ID: found.message.ConversationID,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return answered{}, false, nil
	}

	if err != nil {
		return answered{}, false, fmt.Errorf("find answer: %w", err)
	}

	mine := found.conv.ChannelConnectionID == conn.ID && found.message.AuthorType == string(enums.AuthorTypeAgent)

	return found, mine, nil
}

/*
solve closes the conversation for the person who asked, and tells
anyone else only they can.
*/
func (s *Slack) solve(ctx context.Context, in SlackInteraction, found answered) error {
	clicker := slackdriver.UserIdentity(in.Interaction.User.TeamID, in.Interaction.User.ID)

	asked, err := s.askedBy(ctx, found.conv, clicker)
	if err != nil {
		return err
	}

	if !asked {
		s.tell(ctx, in, "Only the person who asked can mark this solved.")

		return nil
	}

	if found.conv.Status == string(enums.ConversationStatusClosed) {
		return nil
	}

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w := &conversations.Writer{
			Q: models.New(tx), Tx: tx, Events: s.Events, Now: s.Clock.Now(),
			Actor: conversations.Actor{Type: string(enums.AuthorTypeSystem), ID: ids.Nil()},
		}

		_, _, err := w.ChangeStatus(ctx, found.conv, conversations.StatusChange{
			Status: string(enums.ConversationStatusClosed), Reason: conversations.ReasonContactSolved,
		})

		return err
	})
}

/*
askedBy reports whether the Slack user is the conversation's customer.
*/
func (s *Slack) askedBy(ctx context.Context, conv models.Conversation, identity string) (bool, error) {
	var asked bool

	err := s.read(ctx, func(q *models.Queries) error {
		contact, err := q.FindContactByIdentity(ctx, models.FindContactByIdentityParams{
			WorkspaceID: conv.WorkspaceID, Type: string(enums.IdentityTypeSlackUser), Value: identity,
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		asked = err == nil && (contact.ID == conv.ContactID || (contact.MergedIntoID != nil &&
			*contact.MergedIntoID == conv.ContactID))

		return err
	})

	return asked, err
}

/*
handOff hands the conversation to the team, once.
*/
func (s *Slack) handOff(ctx context.Context, conv models.Conversation) error {
	if conv.AiState == "handed_off" {
		return nil
	}

	now := s.Clock.Now()

	return s.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		err := models.New(tx).HandoffConversationAi(ctx, models.HandoffConversationAiParams{
			WorkspaceID: conv.WorkspaceID, ID: conv.ID, Reason: new(string(enums.AIHandoffCustomerRequested)),
			Now: &now,
		})
		if err != nil {
			return fmt.Errorf("hand the conversation off: %w", err)
		}

		return s.Events.Dispatch(ctx, tx, appevents.AiHandoffRequested{
			Base: appevents.Base{
				ID: ids.New(), WorkspaceID: conv.WorkspaceID, OccurredAt: now,
				Actor: appevents.Actor{Type: string(enums.AuthorTypeContact), ID: conv.ContactID},
			},
			ConversationRef: appevents.ConversationRef{ConversationID: conv.ID},
			Reason:          string(enums.AIHandoffCustomerRequested),
		})
	})
}

/*
clearControls takes the buttons off the answer and records it.
*/
func (s *Slack) clearControls(ctx context.Context, conn models.ChannelConnection, msg models.Message) error {
	if msg.ChannelMessageID == nil {
		return nil
	}

	body := ""
	if msg.Body != nil {
		body = *msg.Body
	}

	answer := contracts.ChatAnswer{
		Body: body, Sources: contracts.SourcesOf(models.ReadAnswerMetadata(msg.Metadata).Sources),
	}

	err := s.Driver.ClearControls(ctx, conn, contracts.ChatMessageRef{ChannelMessageID: *msg.ChannelMessageID}, answer)
	if err != nil {
		return err
	}

	return s.read(ctx, func(q *models.Queries) error {
		return q.MarkControlsCleared(ctx, models.MarkControlsClearedParams{
			Now: s.Clock.Now(), WorkspaceID: msg.WorkspaceID, ID: msg.ID,
		})
	})
}

/*
tell shows the person who acted a message only they see.
*/
func (s *Slack) tell(ctx context.Context, in SlackInteraction, text string) {
	api, err := s.api(ctx, in.Connection)
	if err != nil {
		return
	}

	_ = api.Ephemeral(ctx, slackdriver.Notice{
		Channel: in.Interaction.ChannelID(), User: in.Interaction.User.ID, ThreadTS: in.Interaction.Container.ThreadTS,
		Text: text,
	})
}

/*
onCreateShortcut opens a conversation from the message the shortcut was
used on, or points to the one it is already in.
*/
func (s *Slack) onCreateShortcut(ctx context.Context, in SlackInteraction) (bool, error) {
	msg := in.Interaction.Message
	if msg == nil || msg.User == "" || msg.BotID != "" {
		return false, nil
	}

	room, ok, err := s.watchedRoom(ctx, in.Connection, in.Interaction.ChannelID())
	if err != nil || !ok || room.Mode == string(enums.RoomModeInternal) {
		s.tell(ctx, in, "dimmi does not watch this channel. Invite the app to it first.")

		return false, err
	}

	a, err := s.classify(ctx, in.Connection, &room, msg.User)
	if err != nil || a.kind != authorContact {
		s.tell(ctx, in, "Only a customer's message can become a conversation.")

		return false, err
	}

	ev := *msg
	ev.ChannelRaw = quotedChannel(in.Interaction.ChannelID())

	result, ok, err := s.ingest(ctx, ingestion{
		conn: in.Connection, room: &room, ev: ev, author: a, trigger: enums.RoomTriggerShortcut,
		threadKey: slackdriver.RoomThreadKey(room.ID, ev.RootTS()),
	})
	if err != nil || !ok {
		return false, err
	}

	verb := "Opened"
	if result.Duplicate {
		verb = "Already in"
	}

	s.tell(ctx, in, verb+" a dimmi conversation: "+s.conversationLink(result.ConversationID))

	return true, nil
}
