/*
|--------------------------------------------------------------------------
| What The Team's Slack Work Claims
|--------------------------------------------------------------------------
|
| The team's share of Slack's deliveries: reactions that act on a
| conversation, channels the auto-join rules may want, the home tab, a
| stopped agent, and the buttons and forms of triage cards, drafts,
| discussions and feedback. And the inbox changes the team's places in
| Slack follow: a new conversation is triaged and may get its
| auto-reply, every move of it updates its card, a teammate's reply is
| compared with a waiting draft, and a note reaches the team's threads.
|
*/

package channels

import (
	"context"
	"time"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
)

/*
claimChannelsSeen registers the auto-join check ahead of the channel's
own handling of the same events, which it never takes from them.
*/
func (s *Slack) claimChannelsSeen(r *Router) {
	for _, kind := range []string{"channel_created", "channel_rename", "channel_shared"} {
		r.OnEvent(kind, onConnection(s.onChannelSeen))
	}
}

/*
claimTeam registers the team's handlers.
*/
func (s *Slack) claimTeam(r *Router) {
	r.OnEvent("reaction_added", onConnection(s.onEmojiAction))
	r.OnEvent("reaction_removed", onConnection(s.onEmojiAction))
	r.OnEvent("app_home_opened", onConnection(s.onHomeOpened))
	r.OnEvent("agent_session_stopped", onConnection(s.onSessionStopped))
	r.OnAction(slackdriver.TriagePrefix(), pressOnConnection(s.onTriageAction))
	r.OnAction(slackdriver.DraftPrefix(), pressOnConnection(s.onDraftAction))
	r.OnAction(slackdriver.FeedbackPrefix(), pressOnConnection(s.onFeedback))
	r.OnAction(slackdriver.DiscussionPrefix(), pressOnConnection(s.onDiscussionStart))
	r.OnView(slackdriver.DraftEditorID, pressOnConnection(s.onDraftEdited))
}

/*
listenTeam follows the inbox changes the team's places show.
*/
func (s *Slack) listenTeam(d *events.Dispatcher) {
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.MessageCreated) error {
		return s.followMessage(ctx, e)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationCreated) error {
		return s.followCreated(ctx, e)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationAssigned) error {
		return s.refreshCard(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationClosed) error {
		return s.refreshCard(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationReopened) error {
		return s.refreshCard(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationSnoozed) error {
		return s.refreshCard(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.ConversationPriorityChanged) error {
		return s.refreshCard(ctx, e.WorkspaceID, e.ConversationID)
	})
	events.Listen(d, events.AfterCommit, func(ctx context.Context, e appevents.AiHandoffRequested) error {
		return s.refreshCard(ctx, e.WorkspaceID, e.ConversationID)
	})
}

/*
followCreated triages a new conversation on Slack and schedules its
auto-reply.
*/
func (s *Slack) followCreated(ctx context.Context, e appevents.ConversationCreated) error {
	ctx = tenancy.With(ctx, e.WorkspaceID.String())

	onSlack, err := s.onSlack(ctx, e.WorkspaceID, e.ConversationID)
	if err != nil || !onSlack {
		return err
	}

	args := effectOf(e.WorkspaceID, e.ConversationID, nil, jobs.SlackEffectTriage)
	if err := s.Queue.Push(ctx, args); err != nil {
		return err
	}

	return s.scheduleAutoReply(ctx, args)
}

/*
followMessage queues what a teammate's new message changes on the
team's side: a reply updates the card and is compared with a waiting
draft, a note goes to the team's threads.
*/
func (s *Slack) followMessage(ctx context.Context, e appevents.MessageCreated) error {
	member := e.AuthorType == string(enums.AuthorTypeMember)
	note := e.Type == string(enums.MessageTypeNote)

	if !member || (!note && e.Type != string(enums.MessageTypeMessage)) {
		return nil
	}

	ctx = tenancy.With(ctx, e.WorkspaceID.String())

	onSlack, err := s.onSlack(ctx, e.WorkspaceID, e.ConversationID)
	if err != nil || !onSlack {
		return err
	}

	if note {
		return s.Queue.Push(ctx, effectOf(e.WorkspaceID, e.ConversationID, &e.MessageID, jobs.SlackEffectNote))
	}

	err = s.Queue.Push(ctx, effectOf(e.WorkspaceID, e.ConversationID, &e.MessageID, jobs.SlackEffectJudge))
	if err != nil {
		return err
	}

	return s.refreshCard(ctx, e.WorkspaceID, e.ConversationID)
}

/*
refreshCard queues an update of the conversation's triage card, a
moment later, so a burst of changes updates it once.
*/
func (s *Slack) refreshCard(ctx context.Context, workspaceID, conversationID ids.UUID) error {
	ctx = tenancy.With(ctx, workspaceID.String())

	onSlack, err := s.onSlack(ctx, workspaceID, conversationID)
	if err != nil || !onSlack {
		return err
	}

	return s.Queue.Push(ctx, effectOf(workspaceID, conversationID, nil, jobs.SlackEffectTriage),
		queue.At(s.Clock.Now().Add(effectDebounce*time.Second)), queue.Unique(queue.UniqueOpts{
			ByArgs: true, ByState: []queue.State{
				queue.StatePending, queue.StateScheduled, queue.StateAvailable, queue.StateRunning,
			},
		}))
}
