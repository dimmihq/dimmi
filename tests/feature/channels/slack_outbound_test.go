package channels_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
opened is a test with a customer's conversation open in the shared room.
*/
func opened(t *testing.T) (*slackCase, models.Conversation) {
	t.Helper()

	sc := connectedCase(t)
	sc.deliver(message(supportRoom, "U0CUSTOMER", "1793000000.000100", "The export fails"))

	convs := sc.conversations()
	require.Len(t, convs, 1)

	return sc, convs[0]
}

/*
replyAsOwner replies to the conversation from the inbox and runs the
send.
*/
func (sc *slackCase) replyAsOwner(conv models.Conversation, text string) string {
	sc.T.Helper()

	res := sc.ActingAsOwner().PostJSON(
		"/api/v1/conversations/"+models.PublicID(models.PrefixConversation, conv.ID)+"/replies",
		map[string]any{"body": text},
	)
	require.Less(sc.T, res.Status, 300, string(res.Body))
	sc.Drain()

	return res.String("data.id")
}

/*
answerAsAgent stores an agent's answer offering the controls and runs
its send.
*/
func (sc *slackCase) answerAsAgent(conv models.Conversation) models.Message {
	sc.T.Helper()

	var msg models.Message

	ctx := tenancy.With(sc.T.Context(), sc.Workspace.Workspace.ID.String())
	err := sc.App.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		w := &conversations.Writer{
			Q: models.New(tx), Tx: tx, Events: sc.App.Events, Now: sc.Clock.Now(),
			Actor: conversations.Actor{Type: string(enums.AuthorTypeAgent), ID: ids.New(), Name: "dimmi"},
		}

		var err error

		msg, _, err = w.Append(ctx, conv, conversations.NewMessage{
			Type: string(enums.MessageTypeMessage), Body: "Try exporting again.",
			DeliveryStatus: new(string(enums.DeliveryStatusPending)),
			Metadata:       json.RawMessage(`{"ai": {"actions": ["solved", "handoff"]}}`),
		})
		if err != nil {
			return err
		}

		return sc.App.Queue.PushTx(ctx, tx, jobs.SendChannelMessageArgs{
			WorkspaceID: msg.WorkspaceID, ConversationID: msg.ConversationID, MessageID: msg.ID,
		})
	})
	require.NoError(sc.T, err)
	sc.Drain()

	return query(sc, func(ctx context.Context, q *models.Queries) (models.Message, error) {
		return q.FindWorkspaceMessage(ctx, models.FindWorkspaceMessageParams{WorkspaceID: msg.WorkspaceID, ID: msg.ID})
	})
}

/*
lastReply returns the newest member reply of the first conversation.
*/
func (sc *slackCase) lastReply() models.Message {
	sc.T.Helper()

	var last models.Message

	for _, m := range sc.messages(sc.conversations()[0]) {
		if m.AuthorType == string(enums.AuthorTypeMember) && m.Type == string(enums.MessageTypeMessage) {
			last = m
		}
	}

	require.NotNil(sc.T, last.DeliveryStatus)

	return last
}

func TestSlackOutbound(t *testing.T) {
	t.Parallel()

	t.Run("a teammate's reply goes into the customer's thread under their name", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)

		sc.replyAsOwner(conv, "Fixed in <!channel> **v2**, see the table:\n\n| a | b |\n|---|---|\n| 1 | 2 |")

		posts := sc.slack.called("chat.postMessage")
		require.Len(t, posts, 1)

		form := posts[0].Form
		assert.Equal(t, supportRoom, form.Get("channel"))
		assert.Equal(t, "1793000000.000100", form.Get("thread_ts"))
		assert.Equal(t, sc.Owner.Name, form.Get("username"))
		assert.Contains(t, form.Get("markdown_text"), "&lt;!channel&gt;")
		assert.Contains(t, form.Get("markdown_text"), "```\n| a | b |")
		assert.NotContains(t, form.Get("markdown_text"), "<!channel>")

		sent := sc.lastReply()
		require.NotNil(t, sent.DeliveryStatus)
		assert.Equal(t, string(enums.DeliveryStatusSent), *sent.DeliveryStatus)
		assert.Equal(t, supportRoom+":1730000500.000200", *sent.ChannelMessageID)
	})

	t.Run("an agent's answer carries the buttons, taken off once a teammate replies", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)

		sc.answerAsAgent(conv)
		posts := sc.slack.called("chat.postMessage")
		require.Len(t, posts, 1)
		assert.Contains(t, posts[0].Form.Get("blocks"), `"action_id":"c:msg_`)
		assert.Contains(t, posts[0].Form.Get("blocks"), `"type":"markdown"`)

		sc.replyAsOwner(conv, "I took a look too.")

		assert.Len(t, sc.slack.called("chat.update"), 1)
	})

	t.Run("Slack asking to slow down postpones the reply without failing it", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		sc.slack.answer("chat.postMessage", slackAnswer{
			status: 429, body: `{"ok": false, "error": "ratelimited"}`, header: map[string]string{"Retry-After": "30"},
		})

		sc.replyAsOwner(conv, "On it")

		assert.Equal(t, string(enums.DeliveryStatusPending), *sc.lastReply().DeliveryStatus)

		sc.DrainLater()

		assert.Equal(t, string(enums.DeliveryStatusSent), *sc.lastReply().DeliveryStatus)
	})

	t.Run("a bot missing from the channel fails the reply with the fix", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		sc.slack.answer("chat.postMessage", slackAnswer{status: 200, body: `{"ok": false, "error": "not_in_channel"}`})

		sc.replyAsOwner(conv, "On it")

		assert.Equal(t, string(enums.DeliveryStatusFailed), *sc.lastReply().DeliveryStatus)
	})

	t.Run("closing marks the thread, reopening clears the mark", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		path := "/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conv.ID)

		sc.ActingAsOwner().PatchJSON(path, map[string]any{"status": "closed"}).AssertOK()
		sc.Drain()

		adds := sc.slack.called("reactions.add")
		require.Len(t, adds, 1)
		assert.Equal(t, "white_check_mark", adds[0].Form.Get("name"))
		assert.Equal(t, "1793000000.000100", adds[0].Form.Get("timestamp"))

		sc.ActingAsOwner().PatchJSON(path, map[string]any{"status": "open"}).AssertOK()
		sc.Drain()

		assert.Len(t, sc.slack.called("reactions.remove"), 1)
	})

	t.Run("deleting a sent reply in the inbox deletes it in Slack", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)

		id := sc.replyAsOwner(conv, "Wrong answer")
		sc.ActingAsOwner().Delete(
			"/api/v1/conversations/" + models.PublicID(models.PrefixConversation, conv.ID) + "/messages/" + id,
		)
		sc.Drain()

		deletes := sc.slack.called("chat.delete")
		require.Len(t, deletes, 1)
		assert.Equal(t, "1730000500.000200", deletes[0].Form.Get("ts"))
	})

	t.Run("the working sign follows the agent writing", func(t *testing.T) {
		t.Parallel()
		sc, conv := opened(t)
		first := sc.messages(conv)[0]

		sc.App.Actions.Slack.Channel.Started(sc.T.Context(), conv.WorkspaceID, conv.ID, first.ID)
		sc.App.Actions.Slack.Channel.Stopped(sc.T.Context(), conv.WorkspaceID, conv.ID, first.ID)

		require.Len(t, sc.slack.called("reactions.add"), 1)
		assert.Equal(t, "eyes", sc.slack.called("reactions.add")[0].Form.Get("name"))
		assert.Len(t, sc.slack.called("reactions.remove"), 1)
	})

	t.Run("each refusal from Slack fails the reply with what to do about it", func(t *testing.T) {
		t.Parallel()

		for code, want := range map[string]string{
			"invalid_auth": "Reconnect Slack", "channel_not_found": "archived or deleted",
			"msg_too_long": "refused the message", "restricted_action": "admin policy",
		} {
			sc, conv := opened(t)
			sc.slack.answer("chat.postMessage", slackAnswer{status: 200, body: refused(code)})

			sc.replyAsOwner(conv, "On it")

			reply := sc.lastReply()
			assert.Equal(t, string(enums.DeliveryStatusFailed), *reply.DeliveryStatus, code)
			require.NotNil(t, reply.DeliveryError, code)
			assert.Contains(t, *reply.DeliveryError, want, code)
		}
	})
}
