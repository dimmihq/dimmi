/*
|--------------------------------------------------------------------------
| Inbox Changes In Slack
|--------------------------------------------------------------------------
|
| What the team does in the inbox shows where the customer reads. A
| closed conversation's thread gets its check mark, taken off when it
| reopens. A teammate's reply takes the buttons off the agent's earlier
| answers, since a human now answers. A reply a teammate deletes in the
| inbox is deleted in Slack too; a teammate's own message typed in Slack
| stays theirs to delete there. While the agent writes, the customer's
| message carries the working sign.
|
| Each change is applied by a job after it committed, and only for
| conversations that live on Slack, so the inbox never waits on Slack
| and nothing else pays for it.
|
*/

package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Listen follows the inbox changes Slack shows.
*/
func (s *Slack) Listen(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationClosed) error {
		return s.follow(ctx, effectOf(e.WorkspaceID, e.ConversationID, nil, jobs.SlackEffectClosed))
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationReopened) error {
		return s.follow(ctx, effectOf(e.WorkspaceID, e.ConversationID, nil, jobs.SlackEffectReopened))
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MessageCreated) error {
		if e.AuthorType != string(enums.AuthorTypeMember) || e.Type != string(enums.MessageTypeMessage) {
			return nil
		}

		return s.follow(ctx, effectOf(e.WorkspaceID, e.ConversationID, nil, jobs.SlackEffectAnswered))
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MessageDeleted) error {
		return s.follow(ctx, effectOf(e.WorkspaceID, e.ConversationID, &e.MessageID, jobs.SlackEffectDeleted))
	})
}

/*
effectOf names a change for its job.
*/
func effectOf(workspaceID, conversationID ids.UUID, messageID *ids.UUID, effect string) jobs.ApplySlackEffectArgs {
	return jobs.ApplySlackEffectArgs{
		WorkspaceID: workspaceID, ConversationID: conversationID, MessageID: messageID, Effect: effect,
	}
}

/*
follow queues the change's job when the conversation lives on Slack.
*/
func (s *Slack) follow(ctx context.Context, args jobs.ApplySlackEffectArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	onSlack, err := s.onSlack(ctx, args.WorkspaceID, args.ConversationID)
	if err != nil || !onSlack {
		return err
	}

	return s.Queue.Push(ctx, args)
}

/*
onSlack reports whether the conversation lives on a Slack connection.
*/
func (s *Slack) onSlack(ctx context.Context, workspaceID, conversationID ids.UUID) (bool, error) {
	var kind string

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		kind, err = q.FindConversationChannelType(ctx, models.FindConversationChannelTypeParams{
			WorkspaceID: workspaceID, ID: conversationID,
		})

		return err
	})

	return kind == slackdriver.Type, ignoreMissing(err)
}

/*
Apply applies the change on Slack. A change Slack refuses for good,
such as deleting a message that is already gone, is not tried again.
*/
func (s *Slack) Apply(ctx context.Context, args jobs.ApplySlackEffectArgs) error {
	ctx = tenancy.With(ctx, args.WorkspaceID.String())

	var conv models.Conversation

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		conv, err = q.FindConversation(ctx, models.FindConversationParams{
			WorkspaceID: args.WorkspaceID, ID: args.ConversationID,
		})

		return err
	})
	if err != nil {
		return ignoreMissing(err)
	}

	conn, err := s.connection(ctx, conv.WorkspaceID, conv.ChannelConnectionID)
	if err != nil || conn.Type != slackdriver.Type || conn.Status == "disconnected" || conn.DeletedAt != nil {
		return err
	}

	err = s.applyOn(ctx, conn, conv, args)
	if errors.Is(err, contracts.ErrPermanent) {
		return queue.Cancel(err)
	}

	return err
}

/*
applyOn applies one change on the conversation's connection.
*/
func (s *Slack) applyOn(ctx context.Context, conn models.ChannelConnection, conv models.Conversation,
	args jobs.ApplySlackEffectArgs,
) error {
	key := ""
	if conv.ExternalThreadKey != nil {
		key = *conv.ExternalThreadKey
	}

	switch args.Effect {
	case jobs.SlackEffectClosed, jobs.SlackEffectReopened:
		return s.Driver.MarkState(ctx, conn, key, args.Effect == jobs.SlackEffectClosed)
	case jobs.SlackEffectAnswered:
		return s.clearAnswers(ctx, conn, conv)
	case jobs.SlackEffectDeleted:
		if args.MessageID == nil {
			return nil
		}

		return s.deleteSent(ctx, conn, *args.MessageID)
	default:
		return nil
	}
}

/*
clearAnswers takes the buttons off every answer of the conversation that
still shows them.
*/
func (s *Slack) clearAnswers(ctx context.Context, conn models.ChannelConnection, conv models.Conversation) error {
	var answers []models.Message

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		answers, err = q.ListControlledAnswers(ctx, models.ListControlledAnswersParams{
			WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID,
		})

		return err
	})
	if err != nil {
		return fmt.Errorf("list answers with controls: %w", err)
	}

	for _, answer := range answers {
		if err := s.clearControls(ctx, conn, answer); err != nil {
			return err
		}
	}

	return nil
}

/*
deleteSent deletes in Slack a reply the team sent from the inbox.
*/
func (s *Slack) deleteSent(ctx context.Context, conn models.ChannelConnection, messageID ids.UUID) error {
	var msg models.Message

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		msg, err = q.FindWorkspaceMessage(ctx, models.FindWorkspaceMessageParams{
			WorkspaceID: conn.WorkspaceID, ID: messageID,
		})

		return err
	})
	if err != nil {
		return ignoreMissing(err)
	}

	if msg.ChannelMessageID == nil || !teamSent(msg) {
		return nil
	}

	ref := contracts.ChatMessageRef{ChannelMessageID: *msg.ChannelMessageID}
	if msg.AuthorType == string(enums.AuthorTypeMember) {
		ref.AuthorMemberID = msg.AuthorID
	}

	return s.Driver.DeleteSent(ctx, conn, ref)
}

/*
teamSent reports whether the team sent the message from the inbox,
rather than a person typing it in Slack.
*/
func teamSent(msg models.Message) bool {
	if msg.AuthorType != string(enums.AuthorTypeMember) && msg.AuthorType != string(enums.AuthorTypeAgent) {
		return false
	}

	var meta struct {
		Echo bool `json:"echo"`
	}

	return json.Unmarshal(msg.Metadata, &meta) != nil || !meta.Echo
}

/*
Started puts the working sign on the customer's message when the agent
begins answering it on Slack. A failure only costs the sign.
*/
func (s *Slack) Started(ctx context.Context, workspaceID, conversationID, messageID ids.UUID) {
	ref, ok := s.workingOn(ctx, workingRef{
		workspaceID: workspaceID, conversationID: conversationID, messageID: messageID,
	})
	if ok {
		_ = s.Driver.StartWorking(ctx, ref.conn, ref.message)
	}
}

/*
Stopped takes the working sign off again.
*/
func (s *Slack) Stopped(ctx context.Context, workspaceID, conversationID, messageID ids.UUID) {
	ref, ok := s.workingOn(ctx, workingRef{
		workspaceID: workspaceID, conversationID: conversationID, messageID: messageID,
	})
	if ok {
		_ = s.Driver.StopWorking(ctx, ref.conn, ref.message)
	}
}

/*
workingRef names the customer's message the agent is answering.
*/
type workingRef struct {
	workspaceID    ids.UUID
	conversationID ids.UUID
	messageID      ids.UUID
}

/*
workingTarget is the Slack message the working sign goes on, and its
connection.
*/
type workingTarget struct {
	conn    models.ChannelConnection
	message contracts.ChatMessageRef
}

/*
workingOn finds the Slack message the working sign goes on, reporting
false for a message that is not on Slack.
*/
func (s *Slack) workingOn(ctx context.Context, ref workingRef) (workingTarget, bool) {
	ctx = tenancy.With(ctx, ref.workspaceID.String())

	if onSlack, err := s.onSlack(ctx, ref.workspaceID, ref.conversationID); err != nil || !onSlack {
		return workingTarget{}, false
	}

	var msg models.Message

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		msg, err = q.FindWorkspaceMessage(ctx, models.FindWorkspaceMessageParams{
			WorkspaceID: ref.workspaceID, ID: ref.messageID,
		})

		return err
	})
	if err != nil || msg.ChannelMessageID == nil || msg.ChannelConnectionID == nil {
		return workingTarget{}, false
	}

	conn, err := s.connection(ctx, ref.workspaceID, *msg.ChannelConnectionID)
	if err != nil {
		return workingTarget{}, false
	}

	return workingTarget{conn: conn, message: contracts.ChatMessageRef{ChannelMessageID: *msg.ChannelMessageID}}, true
}
