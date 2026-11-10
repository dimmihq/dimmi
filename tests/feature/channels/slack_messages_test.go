package channels_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	slackdriver "github.com/dimmihq/dimmi/app/drivers/channel/slack"
	"github.com/dimmihq/dimmi/app/enums"
)

func TestSlackMessages(t *testing.T) {
	t.Parallel()

	t.Run("a customer's top-level message opens a conversation in the room's thread", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The *export* keeps failing"))

		convs := sc.conversations()
		require.Len(t, convs, 1)

		room := sc.room(supportRoom)
		require.NotNil(t, convs[0].ExternalThreadKey)
		assert.Equal(t, slackdriver.RoomThreadKey(room.ID, "1793000000.000100"), *convs[0].ExternalThreadKey)

		var source map[string]any
		require.NoError(t, json.Unmarshal(convs[0].Source, &source))
		assert.Equal(t, true, source["public_room"])

		msgs := sc.messages(convs[0])
		require.Len(t, msgs, 1)
		assert.Equal(t, "The **export** keeps failing", body(msgs[0]))
		assert.Equal(t, string(enums.AuthorTypeContact), msgs[0].AuthorType)
		require.NotNil(t, msgs[0].ChannelMessageID)
		assert.Equal(t, supportRoom+":1793000000.000100", *msgs[0].ChannelMessageID)
	})

	t.Run("the same message delivered twice lands once", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		ev := message(supportRoom, "U0CUSTOMER", "1793000000.000100", "Hello, is anyone there?")
		sc.deliver(ev)
		sc.deliver(ev)

		convs := sc.conversations()
		require.Len(t, convs, 1)
		assert.Len(t, sc.messages(convs[0]), 1)
	})

	t.Run("a customer's messages within the window join their unanswered conversation", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000010.000100", "with error 500"))
		sc.deliver(message(supportRoom, "U0OTHER", "1793000020.000100", "Same problem for me"))

		convs := sc.conversations()
		require.Len(t, convs, 2)
		assert.Len(t, sc.messages(convs[0]), 2)
	})

	t.Run("every message opens its own conversation when the room asks for that", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "every_message")

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000010.000100", "with error 500"))

		assert.Len(t, sc.conversations(), 2)
	})

	t.Run("a mention room only opens conversations for messages that mention the app", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "mention")

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "just chatting"))
		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000010.000100", "<@U0BOT> the export fails"))

		convs := sc.conversations()
		require.Len(t, convs, 1)
		assert.Equal(t, "@dimmi the export fails", body(sc.messages(convs[0])[0]))
	})

	t.Run("rooms that are off or wait for an emoji open nothing at post time", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(supportRoom, "shared", "off")
		sc.setRoom(communityRoom, "community", "emoji")

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(message(communityRoom, "U0CUSTOMER", "1793000010.000100", "The import fails"))
		sc.deliver(reply(message(supportRoom, "U0CUSTOMER", "1793000020.000100", "still"), rootTS))

		assert.Empty(t, sc.conversations())
	})

	t.Run("thanks never opens a conversation and wakes nobody inside one", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "thanks! 🙏"))
		assert.Empty(t, sc.conversations())

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000010.000100", "The export fails"))
		sc.deliver(reply(message(supportRoom, "U0CUSTOMER", "1793000020.000100", "thank you :tada:"), secondTS))

		convs := sc.conversations()
		require.Len(t, convs, 1)

		msgs := sc.messages(convs[0])
		require.Len(t, msgs, 2)
		assert.False(t, msgs[1].Substantive)
	})

	t.Run("a customer's reply in a thread nobody tracked opens its conversation", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(reply(message(supportRoom, "U0CUSTOMER", "1793000020.000100", "Any update?"), rootTS))

		convs := sc.conversations()
		require.Len(t, convs, 1)
		room := sc.room(supportRoom)
		assert.Equal(t, slackdriver.RoomThreadKey(room.ID, "1793000000.000100"), *convs[0].ExternalThreadKey)
	})

	t.Run("bots, the app itself and channels nobody watches are never read", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		bot := message(supportRoom, "U0CUSTOMER", "1793000000.000100", "Deploy finished")
		bot["bot_id"] = "B0ZAPIER"
		sc.deliver(bot)
		sc.deliver(message(supportRoom, botUser, "1793000010.000100", "posted by the app"))
		sc.deliver(message("C0UNKNOWN", "U0CUSTOMER", "1793000020.000100", "The export fails"))
		joined := message(supportRoom, "U0CUSTOMER", "1793000030.000100", "")
		joined["subtype"] = "channel_join"
		sc.deliver(joined)

		assert.Empty(t, sc.conversations())
	})
}
