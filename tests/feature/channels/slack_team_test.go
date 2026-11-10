package channels_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
The triage channel the tests post cards to, and the timestamp every
post of the fake Slack gets.
*/
const (
	triageRoom = "C0TRIAGE"
	postedTS   = "1730000500.000200"
)

/*
setSetting writes one setting of the support room.
*/
func (sc *slackCase) setSetting(key string, value any) {
	sc.T.Helper()

	raw, err := json.Marshal(value)
	require.NoError(sc.T, err)

	_, err = sc.Operator().Exec(sc.T.Context(),
		"UPDATE channel_rooms SET settings = settings || jsonb_build_object($1::text, $2::jsonb) "+
			"WHERE channel_connection_id = $3 AND external_id = $4",
		key, string(raw), sc.conn.ID, supportRoom)
	require.NoError(sc.T, err)
}

/*
setConfig writes one key of the connection's config.
*/
func (sc *slackCase) setConfig(key string, value any) {
	sc.T.Helper()

	raw, err := json.Marshal(value)
	require.NoError(sc.T, err)

	_, err = sc.Operator().Exec(sc.T.Context(),
		"UPDATE channel_connections SET config = config || jsonb_build_object($1::text, $2::jsonb) WHERE id = $3",
		key, string(raw), sc.conn.ID)
	require.NoError(sc.T, err)
	sc.conn = sc.connection(sc.conn.ID)
}

/*
triaged is a test with a customer's conversation posted to the triage
channel.
*/
func triaged(t *testing.T) (*slackCase, models.Conversation) {
	t.Helper()

	sc := connectedCase(t)
	sc.setSetting("triage_channel_id", triageRoom)
	sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))

	convs := sc.conversations()
	require.Len(t, convs, 1)

	return sc, convs[0]
}

/*
teamPress is a press on a team button by the Slack user, with the
chosen option's value.
*/
func teamPress(actionID, user, value string) map[string]any {
	action := map[string]any{"action_id": actionID, "action_ts": "1793000100.000100"}
	if value != "" {
		action["selected_option"] = map[string]any{"value": value}
		action["value"] = value
	}

	return map[string]any{
		"type": "block_actions", "api_app_id": appID, "trigger_id": "trig-" + actionID + "-" + user,
		"team": map[string]any{"id": installTeam}, "user": map[string]any{"id": user, "team_id": installTeam},
		"channel":   map[string]any{"id": triageRoom},
		"container": map[string]any{"type": "message", "channel_id": triageRoom, "message_ts": postedTS},
		"actions":   []map[string]any{action},
	}
}

/*
posted returns the posts the fake Slack received in the channel.
*/
func (sc *slackCase) posted(channel string) []slackCall {
	var out []slackCall

	for _, call := range sc.slack.called("chat.postMessage") {
		if call.Form.Get("channel") == channel {
			out = append(out, call)
		}
	}

	return out
}

/*
refreshed reloads the conversation.
*/
func (sc *slackCase) refreshed(conv models.Conversation) models.Conversation {
	return query(sc, func(ctx context.Context, q *models.Queries) (models.Conversation, error) {
		return q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: conv.WorkspaceID, ID: conv.ID})
	})
}

func TestSlackTriage(t *testing.T) {
	t.Parallel()

	t.Run("a new conversation is posted once to the triage channel with its buttons", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)

		posts := sc.posted(triageRoom)
		require.Len(t, posts, 1)
		assert.Contains(t, posts[0].Form.Get("blocks"), models.PublicID(models.PrefixConversation, conv.ID)+":take")

		post := query(sc, func(ctx context.Context, q *models.Queries) (models.ChannelConversationPost, error) {
			return q.FindTriagePost(ctx, models.FindTriagePostParams{
				WorkspaceID: conv.WorkspaceID, ConversationID: conv.ID,
			})
		})
		assert.Equal(t, postedTS, post.ExternalTs)
	})

	t.Run("take it assigns the teammate, the card follows, a second taker is told", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)
		public := models.PublicID(models.PrefixConversation, conv.ID)
		take := slackdriver.TeamActionID(slackdriver.TriagePrefix(), public, slackdriver.TriageTake)

		sc.interact(teamPress(take, "U0TEAMMATE", ""))
		sc.DrainLater()

		got := sc.refreshed(conv)
		require.NotNil(t, got.AssigneeMemberID)
		assert.Equal(t, sc.Workspace.Owner.ID, *got.AssigneeMemberID)
		assert.NotEmpty(t, sc.slack.called("chat.update"))

		sc.interact(teamPress(take, "U0CUSTOMER", ""))
		notices := sc.slack.called("chat.postEphemeral")
		require.NotEmpty(t, notices)
		assert.Contains(t, notices[len(notices)-1].Form.Get("text"), "Link your Slack account")
	})

	t.Run("status, snooze, urgent and hand back act as the teammate", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)
		id := models.PublicID(models.PrefixConversation, conv.ID)

		sc.interact(teamPress(slackdriver.TeamActionID(slackdriver.TriagePrefix(), id, slackdriver.TriageUrgent),
			"U0TEAMMATE", ""))
		assert.Equal(t, enums.ConversationPriorityUrgent, enums.ConversationPriorityOf(sc.refreshed(conv).Priority))

		sc.interact(teamPress(slackdriver.TeamActionID(slackdriver.TriagePrefix(), id, slackdriver.TriageStatus),
			"U0TEAMMATE", "closed"))
		assert.Equal(t, "closed", sc.refreshed(conv).Status)

		_, err := sc.Operator().Exec(t.Context(), "UPDATE conversations SET ai_state = 'handed_off' WHERE id = $1",
			conv.ID)
		require.NoError(t, err)
		sc.interact(teamPress(slackdriver.TeamActionID(slackdriver.TriagePrefix(), id, slackdriver.TriageAIBack),
			"U0TEAMMATE", ""))
		assert.Equal(t, "active", sc.refreshed(conv).AiState)
	})

	t.Run("a thread reply is a note, and with >> a reply to the customer as the teammate", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)

		sc.deliver(reply(message(triageRoom, "U0TEAMMATE", "1793000020.000100", "Customer is on the old plan"),
			postedTS))
		sc.deliver(reply(message(triageRoom, "U0TEAMMATE", "1793000030.000100", ">> Try exporting again now"),
			postedTS))

		msgs := sc.messages(conv)
		require.Len(t, msgs, 3)
		assert.Equal(t, string(enums.MessageTypeNote), msgs[1].Type)
		assert.Equal(t, "Customer is on the old plan", body(msgs[1]))
		assert.Equal(t, string(enums.MessageTypeMessage), msgs[2].Type)
		assert.Equal(t, string(enums.AuthorTypeMember), msgs[2].AuthorType)
		assert.Equal(t, "Try exporting again now", body(msgs[2]))

		toCustomer := sc.posted(supportRoom)
		require.Len(t, toCustomer, 1)
		assert.Equal(t, rootTS, toCustomer[0].Form.Get("thread_ts"))
		assert.Len(t, sc.posted(triageRoom), 1, "a note from Slack never echoes back")
	})

	t.Run("a note written in dimmi reaches the triage thread", func(t *testing.T) {
		t.Parallel()
		sc, conv := triaged(t)

		sc.ActingAsOwner().PostJSON("/api/v1/conversations/"+models.PublicID(models.PrefixConversation, conv.ID)+
			"/notes", map[string]any{"body": "Escalated to billing"}).AssertCreated()
		sc.Drain()

		posts := sc.posted(triageRoom)
		require.Len(t, posts, 2)
		assert.Equal(t, postedTS, posts[1].Form.Get("thread_ts"))
	})
}

func TestSlackEmojiActions(t *testing.T) {
	t.Parallel()

	reaction := func(kind, user, name string) map[string]any {
		return map[string]any{
			"type": kind, "user": user, "reaction": name,
			"item": map[string]any{"type": "message", "channel": supportRoom, "ts": rootTS},
		}
	}

	t.Run("a teammate's check mark closes, taking it off reopens", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)

		sc.deliver(reaction("reaction_added", "U0TEAMMATE", "white_check_mark"))
		assert.Equal(t, "closed", sc.refreshed(conv).Status)

		sc.deliver(reaction("reaction_removed", "U0TEAMMATE", "white_check_mark"))
		assert.Equal(t, "open", sc.refreshed(conv).Status)
	})

	t.Run("eyes take the conversation and a siren makes it urgent", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)

		sc.deliver(reaction("reaction_added", "U0TEAMMATE", "eyes"))
		sc.deliver(reaction("reaction_added", "U0TEAMMATE", "rotating_light"))

		got := sc.refreshed(conv)
		require.NotNil(t, got.AssigneeMemberID)
		assert.Equal(t, sc.Workspace.Owner.ID, *got.AssigneeMemberID)
		assert.Equal(t, enums.ConversationPriorityUrgent, enums.ConversationPriorityOf(got.Priority))
	})

	t.Run("the customer's own reactions and the app's marks never act", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)

		sc.deliver(reaction("reaction_added", "U0CUSTOMER", "white_check_mark"))
		sc.deliver(reaction("reaction_added", botUser, "white_check_mark"))

		assert.Equal(t, "open", sc.refreshed(conv).Status)
	})
}

func TestSlackAutoReplyAndJoin(t *testing.T) {
	t.Parallel()

	t.Run("an unanswered conversation gets the room's auto-reply after its delay", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setSetting("auto_reply_after_seconds", 60)
		sc.setSetting("auto_reply_message", "We will get back to you shortly.")
		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))

		sc.DrainLater()

		msgs := sc.messages(sc.conversations()[0])
		require.Len(t, msgs, 2)
		assert.Equal(t, string(enums.AuthorTypeSystem), msgs[1].AuthorType)
		assert.Equal(t, "We will get back to you shortly.", body(msgs[1]))
		assert.Len(t, sc.posted(supportRoom), 1)
	})

	t.Run("a teammate answering first cancels the auto-reply", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setSetting("auto_reply_after_seconds", 60)
		sc.deliver(message(supportRoom, "U0CUSTOMER", rootTS, "The export fails"))
		sc.deliver(reply(message(supportRoom, "U0TEAMMATE", secondTS, "On it"), rootTS))

		sc.DrainLater()

		assert.Len(t, sc.messages(sc.conversations()[0]), 2)
		assert.Empty(t, sc.posted(supportRoom))
	})

	t.Run("a new channel matching a rule is joined and watched as the rule says", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setConfig("auto_join", []map[string]any{
			{"match": "prefix", "value": "ext-", "mode": "shared", "trigger": "ai"},
		})

		sc.deliver(map[string]any{
			"type": "channel_created", "channel": map[string]any{"id": "C0EXTACME", "name": "ext-acme"},
		})

		require.Len(t, sc.slack.called("conversations.join"), 1)
		room := sc.room("C0EXTACME")
		assert.Equal(t, "shared", room.Mode)
		assert.Equal(t, "ai", room.Trigger)
	})

	t.Run("a scan joins public matches and suggests inviting the app to private ones", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setConfig("auto_join", []map[string]any{{"match": "prefix", "value": "ext-", "mode": "shared"}})

		require.NoError(t, sc.App.Actions.Slack.Channel.ScanChannels(t.Context(), sc.conn.WorkspaceID, sc.conn.ID))
		sc.Drain()

		assert.Len(t, sc.slack.called("conversations.join"), 1)

		var config struct {
			Suggestions []map[string]string `json:"auto_join_suggestions"`
			Scan        map[string]any      `json:"auto_join_scan"`
		}
		require.NoError(t, json.Unmarshal(sc.connection(sc.conn.ID).Config, &config))
		require.Len(t, config.Suggestions, 1)
		assert.Equal(t, "C0EXTPRIV", config.Suggestions[0]["id"])
		assert.Equal(t, true, config.Scan["done"])
	})
}

func TestSlackHomeAndSessions(t *testing.T) {
	t.Parallel()

	t.Run("the home tab lists the requester's conversations", func(t *testing.T) {
		t.Parallel()
		sc, _ := opened(t)

		sc.deliver(map[string]any{"type": "app_home_opened", "user": "U0CUSTOMER", "tab": "home"})

		views := sc.slack.called("views.publish")
		require.Len(t, views, 1)
		assert.Contains(t, views[0].Body, "Your requests")
	})

	t.Run("a linked teammate's home tab lists what they hold instead", func(t *testing.T) {
		t.Parallel()
		sc, _ := opened(t)

		sc.deliver(map[string]any{"type": "app_home_opened", "user": "U0TEAMMATE", "tab": "home"})

		views := sc.slack.called("views.publish")
		require.Len(t, views, 1)
		assert.Contains(t, views[0].Body, "Assigned to you")
		assert.NotContains(t, views[0].Body, "Your requests")
	})

	t.Run("stopping the agent in Slack hands the conversation to the team", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)

		sc.deliver(map[string]any{
			"type": "agent_session_stopped", "channel": supportRoom, "thread_ts": rootTS, "user": "U0CUSTOMER",
		})

		assert.Equal(t, "handed_off", sc.refreshed(conv).AiState)
	})

	t.Run("a pasted conversation link in an internal room offers a discussion", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		sc.setRoom(communityRoom, "internal", "off")
		public := models.PublicID(models.PrefixConversation, conv.ID)
		link := sc.App.Application.Config.App.URL + "/d/inbox/all/" + public

		sc.deliver(message(communityRoom, "U0TEAMMATE", secondTS, "Can someone look at "+link))
		require.NotEmpty(t, sc.slack.called("chat.postEphemeral"))

		start := slackdriver.TeamActionID(slackdriver.DiscussionPrefix(), models.PublicID(models.PrefixConversation,
			conv.ID), "start")
		press := teamPress(start, "U0TEAMMATE", "")
		press["channel"] = map[string]any{"id": communityRoom}
		sc.interact(press)

		require.Len(t, sc.posted(communityRoom), 1)
		sc.deliver(reply(message(communityRoom, "U0TEAMMATE", "1793000040.000100", "Engineering here"), postedTS))
		msgs := sc.messages(conv)
		assert.Equal(t, string(enums.MessageTypeNote), msgs[len(msgs)-1].Type)
	})
}
