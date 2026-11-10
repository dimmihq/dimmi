/*
|--------------------------------------------------------------------------
| Slack Messages
|--------------------------------------------------------------------------
|
| Slack channels are shared places, so a message only becomes part of a
| conversation when it should. Bots, the app's own posts included, are
| never read as people. A room's mode says who the customer is; its
| trigger when a top-level message opens a conversation: every message,
| only messages that mention the app, only messages someone marks with
| the trigger emoji, the ones the request classifier calls requests, or,
| by default, a customer's messages grouped while nobody answered them.
| Thanks and other pleasantries never open one.
|
| Inside a thread a customer always continues, or opens, the thread's
| conversation; a thank you there is kept but wakes nobody. A linked
| teammate's reply there is their answer. A teammate who is not linked
| is not recorded, but the agent still steps back, since a human from
| the team spoke, and they are asked once a week to link their account.
| A customer mentioning a teammate asks for a human, so the agent steps
| back too.
|
| A direct message with the app is one private conversation at a time,
| and a teammate writing to their own app is a test. The team's own
| places, its internal rooms and the threads under its triage cards and
| discussions, follow their own rules.
|
*/

package channels

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Timing of the reminders to link an account and of the window a
follow-up joins its author's last conversation in.
*/
const (
	linkPromptEvery = 7 * 24 * time.Hour
	followupWindow  = 24 * time.Hour
)

/*
The confidence a request must reach, by how eagerly a room wants them.
*/
const (
	conservativeThreshold = 0.75
	aggressiveThreshold   = 0.45
)

/*
onMessage reads a message event: an edit or deletion of a message, or
a new message from a person.
*/
func (s *Slack) onMessage(ctx context.Context, ev SlackEvent) (bool, error) {
	e := ev.Event

	switch e.Subtype {
	case "message_changed":
		return s.onEdited(ctx, ev)
	case "message_deleted":
		return s.onDeleted(ctx, ev)
	case "", "file_share", "thread_broadcast", "me_message":
	default:
		return false, nil
	}

	if e.BotID != "" || e.User == "" || e.User == FactsOf(ev.Connection).BotUserID {
		return false, nil
	}

	if e.ChannelType == "im" {
		return s.onDirect(ctx, ev)
	}

	room, ok, err := s.watchedRoom(ctx, ev.Connection, e.ChannelID())
	if handled, done, teamErr := s.teamMessage(ctx, ev, room); err != nil || done || teamErr != nil || !ok {
		return handled, errors.Join(err, teamErr)
	}

	a, err := s.classify(ctx, ev.Connection, &room, e.User)
	if err != nil || a.kind == authorIgnored {
		return false, err
	}

	if e.InThread() {
		return s.onThreadReply(ctx, ev, room, a)
	}

	return s.onTopLevel(ctx, ev, room, a)
}

/*
watchedRoom returns the active room of the channel, reporting false for
a channel without one.
*/
func (s *Slack) watchedRoom(ctx context.Context, conn models.ChannelConnection, channel string) (
	models.ChannelRoom, bool, error,
) {
	var room models.ChannelRoom

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		room, err = q.FindChannelRoomByExternalID(ctx, models.FindChannelRoomByExternalIDParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalID: channel,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return room, false, nil
	}

	if err != nil {
		return room, false, fmt.Errorf("find room: %w", err)
	}

	return room, room.Status == string(enums.RoomStatusActive), nil
}

/*
onThreadReply reads a reply inside a thread of a room.
*/
func (s *Slack) onThreadReply(ctx context.Context, ev SlackEvent, room models.ChannelRoom, a author) (bool, error) {
	key := slackdriver.RoomThreadKey(room.ID, ev.Event.ThreadTS)
	in := ingestion{
		conn: ev.Connection, room: &room, ev: ev.Event, author: a, threadKey: key,
		trigger: enums.RoomTrigger(room.Trigger),
	}

	if a.kind == authorTeammate {
		return s.teammateReply(ctx, in)
	}

	if room.Trigger == string(enums.RoomTriggerOff) {
		if _, found, err := s.threadConversation(ctx, ev.Connection, key); err != nil || !found {
			return false, err
		}
	}

	in.quiet = s.Pleasantries.Is(ev.Event.Text)

	result, ok, err := s.ingest(ctx, in)
	if err != nil || !ok {
		return ok, err
	}

	if s.mentionsTeammate(ctx, ev.Connection, &room, ev.Event) {
		return true, s.standDown(ctx, ev.Connection.WorkspaceID, result.ConversationID)
	}

	return true, nil
}

/*
teammateReply records a linked teammate's native reply as their answer,
or, for one not linked, steps the agent back and asks them to link.
*/
func (s *Slack) teammateReply(ctx context.Context, in ingestion) (bool, error) {
	if in.author.memberID != nil {
		in.echo = true
		_, ok, err := s.ingest(ctx, in)
		s.explainSilence(ctx, in)

		return ok, err
	}

	conv, found, err := s.threadConversation(ctx, in.conn, in.threadKey)
	if err != nil || !found {
		return false, err
	}

	if err := s.standDown(ctx, in.conn.WorkspaceID, conv.ID); err != nil {
		return false, err
	}

	s.promptLink(ctx, in.conn, in.ev, in.author)

	return true, nil
}

/*
onTopLevel reads a top-level message of a room by the room's trigger.
*/
func (s *Slack) onTopLevel(ctx context.Context, ev SlackEvent, room models.ChannelRoom, a author) (bool, error) {
	if a.kind != authorContact || s.Pleasantries.Is(ev.Event.Text) {
		return false, nil
	}

	in := ingestion{
		conn: ev.Connection, room: &room, ev: ev.Event, author: a,
		threadKey: slackdriver.RoomThreadKey(room.ID, ev.Event.TS), trigger: enums.RoomTrigger(room.Trigger),
	}

	switch enums.RoomTrigger(room.Trigger) {
	case enums.RoomTriggerOff, enums.RoomTriggerEmoji:
		return false, nil
	case enums.RoomTriggerMention:
		if !slices.Contains(ev.Event.Mentions(), FactsOf(ev.Connection).BotUserID) {
			return false, nil
		}
	case enums.RoomTriggerEveryMessage, enums.RoomTriggerAuto, enums.RoomTriggerShortcut:
	case enums.RoomTriggerAI:
		return s.requestOrChatter(ctx, in)
	case enums.RoomTriggerAuthorWindow:
		if err := s.joinWindow(ctx, &in, s.Config.GroupWindow); err != nil {
			return false, err
		}
	}

	_, ok, err := s.ingest(ctx, in)

	return ok, err
}

/*
joinWindow points the message at its author's open conversation in the
room when they started one within the window and nobody answered it.
*/
func (s *Slack) joinWindow(ctx context.Context, in *ingestion, window time.Duration) error {
	return s.read(ctx, func(q *models.Queries) error {
		conv, err := q.FindWindowConversation(ctx, models.FindWindowConversationParams{
			WorkspaceID: in.conn.WorkspaceID, ChannelConnectionID: in.conn.ID,
			IdentityType: string(enums.IdentityTypeSlackUser), IdentityValue: in.author.identity,
			KeyPrefix: slackdriver.RoomThreadPrefix(in.room.ID), Since: s.Clock.Now().Add(-window),
		})
		if errors.Is(err, database.ErrNotFound) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find window conversation: %w", err)
		}

		if conv.ExternalThreadKey != nil {
			in.threadKey = *conv.ExternalThreadKey
		}

		return nil
	})
}

/*
requestOrChatter asks the request classifier about the message. Without
a classifier, or when it fails, the room groups by author instead.
*/
func (s *Slack) requestOrChatter(ctx context.Context, in ingestion) (bool, error) {
	window := s.Config.GroupWindow

	verdict, err := s.classifyRequest(ctx, in)
	switch {
	case err != nil || verdict.Kind == "":
		in.trigger = enums.RoomTriggerAuthorWindow
	case verdict.Kind == "chatter":
		return false, nil
	case verdict.Kind == "followup":
		window = followupWindow
	case verdict.Confidence < thresholdOf(in.room):
		return false, nil
	default:
		window = 0
	}

	if window > 0 {
		if err := s.joinWindow(ctx, &in, window); err != nil {
			return false, err
		}
	}

	_, ok, err := s.ingest(ctx, in)

	return ok, err
}

/*
classifyRequest returns the classifier's verdict, empty without one.
*/
func (s *Slack) classifyRequest(ctx context.Context, in ingestion) (RequestVerdict, error) {
	if s.Classifier == nil {
		return RequestVerdict{}, nil
	}

	return s.Classifier.Classify(ctx, ClassifyInput{
		WorkspaceID: in.conn.WorkspaceID, ConnectionID: in.conn.ID, RoomID: in.room.ID, Text: in.ev.Text,
		Sensitivity: settingOf(in.room, "request_sensitivity"),
	})
}

/*
thresholdOf is the confidence a request must reach in the room.
*/
func thresholdOf(room *models.ChannelRoom) float64 {
	if settingOf(room, "request_sensitivity") == "aggressive" {
		return aggressiveThreshold
	}

	return conservativeThreshold
}

/*
onDirect reads a direct message with the app: one private conversation
at a time, a teammate's own message being a test.
*/
func (s *Slack) onDirect(ctx context.Context, ev SlackEvent) (bool, error) {
	a, err := s.classify(ctx, ev.Connection, nil, ev.Event.User)
	if err != nil || a.kind == authorIgnored {
		return false, err
	}

	_, ok, err := s.ingest(ctx, ingestion{
		conn: ev.Connection, ev: ev.Event, author: a, threadKey: slackdriver.DirectThreadKey(ev.Event.ChannelID()),
		trigger: enums.RoomTriggerEveryMessage, quiet: s.Pleasantries.Is(ev.Event.Text), test: a.memberID != nil,
	})

	return ok, err
}

/*
threadConversation returns the conversation of the thread, reporting
false for a thread without one.
*/
func (s *Slack) threadConversation(ctx context.Context, conn models.ChannelConnection, key string) (
	models.Conversation, bool, error,
) {
	var conv models.Conversation

	err := s.read(ctx, func(q *models.Queries) error {
		var err error

		conv, err = q.FindThreadConversation(ctx, models.FindThreadConversationParams{
			WorkspaceID: conn.WorkspaceID, ChannelConnectionID: conn.ID, ExternalThreadKey: &key,
		})

		return err
	})
	if errors.Is(err, database.ErrNotFound) {
		return conv, false, nil
	}

	if err != nil {
		return conv, false, fmt.Errorf("find thread conversation: %w", err)
	}

	return conv, true, nil
}

/*
mentionsTeammate reports whether the message mentions someone of the
team: a linked teammate, or in a shared room anyone of the installing
team who is not a guest.
*/
func (s *Slack) mentionsTeammate(ctx context.Context, conn models.ChannelConnection, room *models.ChannelRoom,
	ev slackdriver.Event,
) bool {
	bot := FactsOf(conn).BotUserID

	for _, user := range ev.Mentions() {
		if user == bot {
			continue
		}

		if a, err := s.classify(ctx, conn, room, user); err == nil && a.kind == authorTeammate {
			return true
		}
	}

	return false
}

/*
standDown stops the agent in the conversation because a human of the
team spoke, without a word in the thread.
*/
func (s *Slack) standDown(ctx context.Context, workspaceID, conversationID ids.UUID) error {
	now := s.Clock.Now()

	return s.read(ctx, func(q *models.Queries) error {
		err := q.HandoffConversationAi(ctx, models.HandoffConversationAiParams{
			WorkspaceID: workspaceID, ID: conversationID, Reason: new(string(enums.AIHandoffHumanReplied)), Now: &now,
		})
		if err != nil {
			return fmt.Errorf("stand the agent down: %w", err)
		}

		return nil
	})
}

/*
promptLink asks a teammate who is not linked, at most once a week, to
link their Slack account so their replies are recorded.
*/
func (s *Slack) promptLink(ctx context.Context, conn models.ChannelConnection, ev slackdriver.Event, a author) {
	first, err := s.Cache.Add(ctx, "slack:link-prompt:"+conn.ID.String()+":"+a.identity, []byte{1}, linkPromptEvery)
	if err != nil || !first {
		return
	}

	api, err := s.api(ctx, conn)
	if err != nil {
		return
	}

	_ = api.Ephemeral(ctx, slackdriver.Notice{
		Channel: ev.ChannelID(), User: a.user.ID, ThreadTS: ev.ThreadTS,
		Text: "Link your Slack account in dimmi so your replies here are recorded on the conversation.",
	})
}
