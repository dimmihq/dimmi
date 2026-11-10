package channels_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

func TestSlackPeople(t *testing.T) {
	t.Parallel()

	t.Run("a teammate's native reply is stored as their answer and links them by email", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(reply(message(supportRoom, "U0TEAMMATE", "1793000010.000100", "Looking into it"), rootTS))

		convs := sc.conversations()
		require.Len(t, convs, 1)
		msgs := sc.messages(convs[0])
		require.Len(t, msgs, 2)

		answer := msgs[1]
		assert.Equal(t, string(enums.AuthorTypeMember), answer.AuthorType)
		assert.Equal(t, string(enums.MessageTypeMessage), answer.Type)
		require.NotNil(t, answer.DeliveryStatus)
		assert.Equal(t, string(enums.DeliveryStatusSent), *answer.DeliveryStatus)

		var meta map[string]any
		require.NoError(t, json.Unmarshal(answer.Metadata, &meta))
		assert.Equal(t, true, meta["echo"])
		assert.Empty(t, sc.slack.called("chat.postMessage"))

		link := query(sc, func(ctx context.Context, q *models.Queries) (models.FindChannelMemberLinkRow, error) {
			return q.FindChannelMemberLink(ctx, models.FindChannelMemberLinkParams{
				WorkspaceID: sc.Workspace.Workspace.ID, ChannelConnectionID: sc.conn.ID,
				ExternalUserID: installTeam + ":U0TEAMMATE",
			})
		})
		assert.Equal(t, "email_match", link.Source)

		conv := sc.conversations()[0]
		assert.NotNil(t, conv.FirstReplyAt)
	})

	t.Run("a teammate starting a thread of their own opens nothing", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0TEAMMATE", "1793000000.000100", "Release notes are out"))
		sc.deliver(reply(message(supportRoom, "U0TEAMMATE", "1793000010.000100", "and the blog"), rootTS))

		assert.Empty(t, sc.conversations())
	})

	t.Run("an unlinked teammate's reply stands the agent down and asks them to link", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(reply(message(supportRoom, "U0COLLEAGUE", "1793000010.000100", "On it"), rootTS))
		sc.deliver(reply(message(supportRoom, "U0COLLEAGUE", "1793000020.000100", "Still on it"), rootTS))

		conv := sc.conversations()[0]
		assert.Len(t, sc.messages(conv), 1)
		assert.Equal(t, "handed_off", conv.AiState)
		assert.Len(t, sc.slack.called("chat.postEphemeral"), 1)
	})

	t.Run("a customer mentioning a teammate asks for a human", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))
		sc.deliver(reply(message(supportRoom, "U0CUSTOMER", "1793000010.000100", "<@U0COLLEAGUE> help?"), rootTS))

		assert.Equal(t, "handed_off", sc.conversations()[0].AiState)
	})

	t.Run("guests of the installing team are customers in a shared room", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(supportRoom, "U0GUEST", "1793000000.000100", "Where is my invoice?"))

		assert.Len(t, sc.conversations(), 1)
	})

	t.Run("in a community room only linked teammates are the team", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		sc.deliver(message(communityRoom, "U0COLLEAGUE", "1793000000.000100", "How do I export?"))
		sc.deliver(message(communityRoom, "U0TEAMMATE", "1793000010.000100", "Team update"))

		assert.Len(t, sc.conversations(), 1)
	})

	t.Run("internal rooms never become conversations", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)
		sc.setRoom(communityRoom, "internal", "off")

		sc.deliver(message(communityRoom, "U0COLLEAGUE", "1793000000.000100", "Lunch?"))

		assert.Empty(t, sc.conversations())
	})

	t.Run("a direct message is one private conversation, a teammate's a test", func(t *testing.T) {
		t.Parallel()
		sc := connectedCase(t)

		dm := message("D0CAROL", "U0CUSTOMER", "1793000000.000100", "Hi, a private question")
		dm["channel_type"] = "im"
		sc.deliver(dm)
		dm["ts"], dm["text"] = "1793000010.000100", "and another"
		sc.deliver(dm)

		own := message("D0SAM", "U0TEAMMATE", "1793000020.000100", "Testing my bot")
		own["channel_type"] = "im"
		sc.deliver(own)

		convs := sc.conversations()
		require.Len(t, convs, 2)
		assert.Len(t, sc.messages(convs[0]), 2)

		var source map[string]any
		require.NoError(t, json.Unmarshal(convs[0].Source, &source))
		assert.Equal(t, false, source["public_room"])
		assert.True(t, convs[1].IsTest)
	})
}
