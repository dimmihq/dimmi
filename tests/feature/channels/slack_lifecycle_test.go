package channels_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
)

func TestSlackEditsAndDeletions(t *testing.T) {
	t.Parallel()

	t.Run("an edit in Slack replaces the text and keeps the first one", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(map[string]any{
			"type": "message", "subtype": "message_changed", "channel": supportRoom, "channel_type": "channel",
			"message": map[string]any{"user": "U0CUSTOMER", "ts": "1793000000.000100", "text": "The import fails"},
			"previous_message": map[string]any{
				"user": "U0CUSTOMER", "ts": "1793000000.000100", "text": "The export fails",
			},
		})

		msgs := sc.messages(sc.conversations()[0])
		assert.Equal(t, "The import fails", body(msgs[0]))

		var meta map[string]any
		require.NoError(t, json.Unmarshal(msgs[0].Metadata, &meta))
		edits, ok := meta["edits"].([]any)
		require.True(t, ok)
		require.Len(t, edits, 1)
		first, ok := edits[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "The export fails", first["body"])
	})

	t.Run("a deletion in Slack keeps the message and closes an unanswered conversation", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "my key is sk-123"))
		sc.deliver(map[string]any{
			"type": "message", "subtype": "message_deleted", "channel": supportRoom, "channel_type": "channel",
			"deleted_ts": "1793000000.000100",
		})

		conv := sc.conversations()[0]
		msgs := sc.messages(conv)
		assert.Equal(t, "my key is sk-123", body(msgs[0]))
		assert.Contains(t, string(msgs[0].Metadata), `"source_deleted"`)
		assert.Equal(t, string(enums.ConversationStatusClosed), conv.Status)
	})

	t.Run("a deletion in Slack keeps earlier versions and an open conversation", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(reply(message(supportRoom, "U0CUSTOMER", "1793000000.000200", "Since today"), "1793000000.000100"))
		sc.deliver(map[string]any{
			"type": "message", "subtype": "message_changed", "channel": supportRoom, "channel_type": "channel",
			"message": map[string]any{"user": "U0CUSTOMER", "ts": "1793000000.000100", "text": "The import fails"},
		})
		sc.deliver(map[string]any{
			"type": "message", "subtype": "message_deleted", "channel": supportRoom, "channel_type": "channel",
			"deleted_ts": "1793000000.000100",
		})

		conv := sc.conversations()[0]
		msgs := sc.messages(conv)
		assert.Equal(t, "The import fails", body(msgs[0]))
		assert.Contains(t, string(msgs[0].Metadata), `"edits"`)
		assert.Contains(t, string(msgs[0].Metadata), `"source_deleted"`)
		assert.Equal(t, string(enums.ConversationStatusOpen), conv.Status)
	})

	t.Run("edits and deletions of messages the inbox never held change nothing", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(map[string]any{
			"type": "message", "subtype": "message_changed", "channel": supportRoom, "channel_type": "channel",
			"message": map[string]any{"user": "U0CUSTOMER", "ts": "1793000000.000900", "text": "edited"},
		})
		sc.deliver(map[string]any{
			"type": "message", "subtype": "message_deleted", "channel": supportRoom, "channel_type": "channel",
			"deleted_ts": "1793000000.000900",
		})

		assert.Empty(t, sc.conversations())
	})
}

func TestSlackRooms(t *testing.T) {
	t.Parallel()

	t.Run("inviting the app watches the channel; renaming, moving and sharing follow it", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		sc.slack.answer("users.conversations", slackAnswer{status: 200, body: `{"ok": true, "channels": []}`})
		sc.conn = connectNow(sc)

		sc.deliver(map[string]any{"type": "member_joined_channel", "user": botUser, "channel": supportRoom})
		room := sc.room(supportRoom)
		assert.Equal(t, string(enums.RoomStatusActive), room.Status)
		assert.True(t, room.IsSharedExternal)

		sc.deliver(map[string]any{
			"type": "channel_rename", "channel": map[string]any{"id": supportRoom, "name": "beta"},
		})
		assert.Equal(t, "#beta", sc.room(supportRoom).Name)

		sc.deliver(map[string]any{"type": "channel_unshared", "channel": supportRoom, "is_ext_shared": false})
		assert.False(t, sc.room(supportRoom).IsSharedExternal)

		sc.deliver(map[string]any{
			"type": "channel_id_changed", "old_channel_id": supportRoom, "new_channel_id": "C0NEW",
		})
		assert.Equal(t, room.ID, sc.room("C0NEW").ID)

		sc.deliver(map[string]any{"type": "channel_archive", "channel": "C0NEW"})
		assert.Equal(t, string(enums.RoomStatusRemoved), sc.room("C0NEW").Status)
	})

	t.Run("someone else joining a channel watches nothing", func(t *testing.T) {
		t.Parallel()
		sc := newSlackCase(t)
		sc.slack.answer("users.conversations", slackAnswer{status: 200, body: `{"ok": true, "channels": []}`})
		sc.conn = connectNow(sc)

		sc.deliver(map[string]any{"type": "member_joined_channel", "user": "U0CUSTOMER", "channel": supportRoom})

		assert.Empty(t, sc.slack.called("conversations.info"))
	})

	t.Run("uninstalling the app disconnects the connection and every room", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(map[string]any{
			"type": "tokens_revoked", "tokens": map[string]any{"oauth": []string{"U0TEAMMATE"}},
		})
		assert.Equal(t, "active", sc.connection(sc.conn.ID).Status)

		sc.deliver(map[string]any{"type": "app_uninstalled"})

		assert.Equal(t, "disconnected", sc.connection(sc.conn.ID).Status)
		assert.Equal(t, string(enums.RoomStatusRemoved), sc.room(supportRoom).Status)
	})

	t.Run("a room sync removes rooms of channels the app left", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.slack.answer("users.conversations", slackAnswer{status: 200, body: `{"ok": true, "channels": []}`})

		slack := sc.App.Actions.Slack.Channel
		require.NoError(t, slack.SyncRooms(sc.T.Context(), sc.Workspace.Workspace.ID, sc.conn.ID))

		assert.Equal(t, string(enums.RoomStatusRemoved), sc.room(communityRoom).Status)
	})
}

func TestSlackTriggerEmoji(t *testing.T) {
	t.Parallel()

	t.Run("the trigger emoji on a customer's message opens one conversation", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "emoji")

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1730000300.000100", "The export keeps failing"))
		assert.Empty(t, sc.conversations())

		reaction := map[string]any{
			"type": "reaction_added", "user": "U0TEAMMATE", "reaction": "ticket",
			"item": map[string]any{"type": "message", "channel": supportRoom, "ts": "1730000300.000100"},
		}
		sc.deliver(reaction)
		sc.deliver(reaction)

		convs := sc.conversations()
		require.Len(t, convs, 1)
		assert.Equal(t, "The export **keeps** failing", body(sc.messages(convs[0])[0]))
	})

	t.Run("other reactions are left for other features", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "emoji")

		sc.deliver(map[string]any{
			"type": "reaction_added", "user": "U0TEAMMATE", "reaction": "eyes",
			"item": map[string]any{"type": "message", "channel": supportRoom, "ts": "1730000300.000100"},
		})

		assert.Empty(t, sc.conversations())
		assert.Empty(t, sc.slack.called("conversations.history"))
	})
}
