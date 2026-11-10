package channels_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
pressed is a button press on the answer by the Slack user.
*/
func pressed(answer models.Message, control, user string) map[string]any {
	return map[string]any{
		"type": "block_actions", "api_app_id": appID, "trigger_id": "trig-" + control + "-" + user,
		"team": map[string]any{"id": installTeam}, "user": map[string]any{"id": user, "team_id": customerTeam},
		"channel":   map[string]any{"id": supportRoom},
		"container": map[string]any{"type": "message", "channel_id": supportRoom, "thread_ts": "1793000000.000100"},
		"actions": []map[string]any{{
			"action_id": slackdriver.ControlActionID(models.PublicID(models.PrefixMessage, answer.ID), control),
			"action_ts": "1793000100.000100",
		}},
	}
}

func TestSlackInteractions(t *testing.T) {
	t.Parallel()

	t.Run("solved from the person who asked closes the conversation", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		answer := sc.answerAsAgent(conv)

		sc.interact(pressed(answer, "solved", "U0CUSTOMER"))

		assert.Equal(t, string(enums.ConversationStatusClosed), sc.conversations()[0].Status)
		assert.Len(t, sc.slack.called("chat.update"), 1)
	})

	t.Run("solved from anyone else is refused", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		answer := sc.answerAsAgent(conv)

		sc.interact(pressed(answer, "solved", "U0OTHER"))

		assert.Equal(t, string(enums.ConversationStatusOpen), sc.conversations()[0].Status)
		assert.Len(t, sc.slack.called("chat.postEphemeral"), 1)
	})

	t.Run("talk to a human hands the conversation to the team once", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		answer := sc.answerAsAgent(conv)

		sc.interact(pressed(answer, "handoff", "U0OTHER"))
		assert.Equal(t, "handed_off", sc.conversations()[0].AiState)
	})

	t.Run("a button naming nothing or another conversation's channel does nothing", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		first := sc.messages(conv)[0]

		sc.interact(pressed(first, "solved", "U0CUSTOMER"))
		sc.interact(map[string]any{
			"type": "block_actions", "api_app_id": appID, "trigger_id": "x", "team": map[string]any{"id": installTeam},
			"actions": []map[string]any{{"action_id": "c:msg_garbage:solved", "action_ts": "1"}},
		})

		assert.Equal(t, string(enums.ConversationStatusOpen), sc.conversations()[0].Status)
	})

	t.Run("the shortcut opens a conversation from a customer's message, once", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "off")

		shortcut := map[string]any{
			"type": "message_action", "callback_id": slackdriver.ShortcutCreate, "api_app_id": appID,
			"trigger_id": "trig-1", "team": map[string]any{"id": installTeam},
			"user":    map[string]any{"id": "U0TEAMMATE", "team_id": installTeam},
			"channel": map[string]any{"id": supportRoom, "name": "acme-support"},
			"message": map[string]any{
				"type": "message", "user": "U0CUSTOMER", "ts": "1793000000.000100",
				"text": "The export fails",
			},
		}
		sc.interact(shortcut)
		shortcut["trigger_id"] = "trig-2"
		sc.interact(shortcut)

		require.Len(t, sc.conversations(), 1)
		ephemerals := sc.slack.called("chat.postEphemeral")
		require.Len(t, ephemerals, 2)
		assert.Contains(t, ephemerals[0].Form.Get("text"), "Opened a dimmi conversation")
		assert.Contains(t, ephemerals[1].Form.Get("text"), "Already in")
	})

	t.Run("the shortcut on a teammate's message or an unwatched channel explains why not", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		shortcut := map[string]any{
			"type": "message_action", "callback_id": slackdriver.ShortcutCreate, "api_app_id": appID,
			"trigger_id": "trig-1", "team": map[string]any{"id": installTeam},
			"user":    map[string]any{"id": "U0TEAMMATE", "team_id": installTeam},
			"channel": map[string]any{"id": supportRoom},
			"message": map[string]any{"type": "message", "user": "U0TEAMMATE", "ts": "1793000000.000100", "text": "x"},
		}
		sc.interact(shortcut)
		shortcut["trigger_id"], shortcut["channel"] = "trig-2", map[string]any{"id": "C0UNKNOWN"}
		sc.interact(shortcut)

		assert.Empty(t, sc.conversations())
		assert.Len(t, sc.slack.called("chat.postEphemeral"), 2)
	})
}
