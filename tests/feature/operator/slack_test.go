package operator_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/actions/operatorslack"
	"github.com/dimmihq/dimmi/app/enums"
)

func TestOperatorInSlack(t *testing.T) {
	t.Parallel()

	t.Run("answers a linked teammate's mention in an internal room in the thread, as them", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		s.roomMode(string(enums.RoomModeInternal))
		s.link()
		s.llm.Script(reply("Three refunds this week."), reply("All three were for March."))

		require.True(t, s.deliver(mention(ownerSlack, "how many refunds this week?", "")))

		posts := s.slack.called("chat.postMessage")
		require.Len(t, posts, 1)
		assert.Equal(t, askedTS, posts[0].Form.Get("thread_ts"))
		updates := s.slack.called("chat.update")
		require.Len(t, updates, 1)
		assert.Contains(t, updates[0].Form.Get("blocks"), "Three refunds this week.")
		sent := s.llm.Sent()
		require.Len(t, sent, 1)
		last := sent[0].Messages[len(sent[0].Messages)-1].Parts[0].Text
		assert.Contains(t, last, "how many refunds this week?")
		assert.NotContains(t, last, "<@"+slackBot+">")

		require.True(t, s.deliver(mention(ownerSlack, "and for which month?", askedTS)))
		assert.Equal(t, 1, s.count("SELECT count(*) FROM operator_slack_threads"))
		assert.Equal(t, 1, s.count("SELECT count(*) FROM operator_threads"))
		assert.Equal(t, 4, s.count("SELECT count(*) FROM operator_messages"))
	})

	t.Run("asks a Slack user who is not linked to connect, and runs nothing", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		s.roomMode(string(enums.RoomModeInternal))

		require.True(t, s.deliver(mention("U0STRANGER", "who is on call?", "")))

		ephemeral := s.slack.called("chat.postEphemeral")
		require.Len(t, ephemeral, 1)
		assert.Contains(t, ephemeral[0].Form.Get("text"), "/d/settings/integrations")
		assert.Empty(t, s.llm.Sent())
	})

	t.Run("never answers in a room customers read", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		s.roomMode(string(enums.RoomModeShared))
		s.link()

		assert.False(t, s.deliver(mention(ownerSlack, "what did Ana say?", "")))
		assert.Empty(t, s.slack.called("chat.postMessage"))
		assert.Empty(t, s.llm.Sent())
	})

	t.Run("answers a linked teammate's direct message", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		s.link()
		s.llm.Script(reply("Nothing is waiting on you."))

		require.True(t, s.deliver(map[string]any{
			"type": "message", "channel": "D0OWNER", "channel_type": "im", "user": ownerSlack, "team": slackTeam,
			"ts": askedTS, "text": "anything waiting on me?",
		}))

		updates := s.slack.called("chat.update")
		require.Len(t, updates, 1)
		assert.Contains(t, updates[0].Form.Get("blocks"), "Nothing is waiting on you.")
	})

	t.Run("says why when Operator cannot answer", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		s.roomMode(string(enums.RoomModeInternal))
		s.link()
		s.llm.Script(fakeFailure(), fakeFailure(), fakeFailure())

		require.True(t, s.deliver(mention(ownerSlack, "summarize the week", "")))

		updates := s.slack.called("chat.update")
		require.Len(t, updates, 1)
		assert.NotContains(t, updates[0].Form.Get("text"), "looking into it")
	})

	t.Run("proposes a change as a card that the teammate applies from Slack", func(t *testing.T) {
		t.Parallel()
		s := newSlackDesk(t)
		s.roomMode(string(enums.RoomModeInternal))
		s.link()
		conv := s.conversation("Charged twice.")
		s.llm.Script(tools(call("propose_changes", map[string]any{
			"title": "File as billing", "reason": "A double charge.",
			"operations": []map[string]any{{"kind": "conversation.priority", "priority": "high"}},
		})), reply("I proposed filing it under billing."))

		require.True(t, s.deliver(mention(ownerSlack, "file "+conv+" please", "")))

		posts := s.slack.called("chat.postMessage")
		require.Len(t, posts, 2)
		card := posts[1].Form.Get("blocks")
		assert.Contains(t, card, "File as billing")
		assert.Contains(t, card, operatorslack.ActionApply)

		s.press(ownerSlack, operatorslack.ActionApply, changesetID(t, card))

		s.d.Owner.GetJSON("/api/v1/conversations/"+conv).AssertOK().AssertJSONPath("data.priority", "high")
		updates := s.slack.called("chat.update")
		assert.Contains(t, updates[len(updates)-1].Form.Get("blocks"), operatorslack.ActionUndo)
	})
}

/*
changesetID reads the changeset id the card's Apply button carries.
*/
func changesetID(t *testing.T, raw string) string {
	t.Helper()

	var blocks []struct {
		Elements []struct {
			ActionID string `json:"action_id"`
			Value    string `json:"value"`
		} `json:"elements"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &blocks))

	for _, b := range blocks {
		for _, e := range b.Elements {
			if e.ActionID == operatorslack.ActionApply {
				return e.Value
			}
		}
	}

	t.Fatal("no apply button")

	return ""
}
