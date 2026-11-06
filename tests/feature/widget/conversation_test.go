package widget_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
startConversation has the visitor start a conversation and returns its id.
*/
func (v *visitor) startConversation(t *testing.T, body string) string {
	t.Helper()

	return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
		"client_id": ids.New().String(), "body": body,
	}).AssertCreated().String("data.id")
}

/*
reply has the owner answer the conversation from the inbox.
*/
func reply(t *testing.T, tc *tests.TestCase, conversation, body string) {
	t.Helper()

	owner(tc).PostJSON("/api/v1/conversations/"+conversation+"/replies", testkit.JSON{"body": body}).AssertCreated()
}

func TestWidgetConversations(t *testing.T) {
	t.Parallel()

	t.Run("a first message starts a conversation the inbox sees with its page", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		client := ids.New().String()

		res := v.client().WithHeader("User-Agent", "Mozilla/5.0 (Macintosh) Firefox/120.0").
			PostJSON("/widget/v1/conversations", testkit.JSON{
				"client_id": client, "body": "  Hi, export fails\u0000 ",
				"page":     testkit.JSON{"url": "https://acme.com/app?session=x", "title": "App"},
				"metadata": testkit.JSON{"widget": testkit.JSON{"identity_verified": true}},
			}).AssertCreated()

		res.AssertJSONPath("data.object", "conversation")
		res.AssertJSONPath("data.status", "open")
		res.AssertJSONPath("data.customer_status", "with_team")
		res.AssertJSONPath("data.widget_id", appID(t, tc))
		res.AssertJSONPath("meta.message.body", "Hi, export fails")
		res.AssertJSONPath("meta.message.client_id", client)
		res.AssertJSONPath("meta.message.author.type", "contact")

		conv := conversationRow(t, tc, res.String("data.id"))
		assert.Contains(t, string(conv.Source), `"page_url": "https://acme.com/app"`)
		assert.Contains(t, string(conv.Source), `"session_id": "`+v.id+`"`)
		assert.False(t, conv.IsTest)
		assert.False(t, conv.Spam)
		tc.Table("messages").AssertCount(1, testkit.Row{"conversation_id": conv.ID, "type": "message"})

		owner(tc).GetJSON("/api/v1/conversations/" + res.String("data.id")).AssertOK()
	})

	t.Run("the same message sent twice lands once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		body := testkit.JSON{"client_id": ids.New().String(), "body": "hello"}

		first := v.client().PostJSON("/widget/v1/conversations", body).AssertCreated()
		second := v.client().PostJSON("/widget/v1/conversations", body).AssertOK()

		assert.Equal(t, first.String("data.id"), second.String("data.id"))
		assert.Equal(t, first.String("meta.message.id"), second.String("meta.message.id"))
		tc.Table("conversations").AssertCount(1, testkit.Row{})
	})

	t.Run("a bot's conversation lands as spam and the visitor never knows", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		res := tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appID(t, tc), "website": "x"})
		v := &visitor{tc: tc, token: res.String("data.token"), id: res.String("data.id")}

		id := v.startConversation(t, "buy now")

		assert.True(t, conversationRow(t, tc, id).Spam)
		v.client().GetJSON("/widget/v1/conversations/" + id).AssertOK().AssertJSONMissing("data.spam")
	})

	t.Run("an offline team asks a lead without email for one first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		patch(t, tc, testkit.JSON{"require_email_when_offline": true})
		v := startVisitor(t, tc)

		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{"client_id": ids.New().String(), "body": "hi"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertJSONPath("code", "email_required")
		tc.Table("conversations").AssertCount(0, testkit.Row{})

		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"email": "ada@example.com"}).AssertOK()
		v.startConversation(t, "hi")
	})

	t.Run("refuses a message without text or files, or without an id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{"client_id": ids.New().String(), "body": " "}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("body")
		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{"client_id": "nope", "body": "hi"}).
			AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("client_id")
	})
}

func TestWidgetMessages(t *testing.T) {
	t.Parallel()

	t.Run("the visitor sees the team's reply, waiting on them, but never a note", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")

		reply(t, tc, id, "Hi Ada, Kim here")
		owner(tc).PostJSON("/api/v1/conversations/"+id+"/notes", testkit.JSON{"body": "secret note"}).AssertCreated()

		res := v.client().GetJSON("/widget/v1/conversations/" + id + "/messages").AssertOK()
		assert.NotContains(t, string(res.Body), "secret note")
		res.AssertJSONPath("data.1.body", "Hi Ada, Kim here")
		res.AssertJSONPath("data.1.author.type", "member")
		assert.NotContains(t, string(res.Body), tc.Owner.Email)

		list := v.client().GetJSON("/widget/v1/conversations").AssertOK()
		list.AssertJSONPath("data.0.customer_status", "waiting_on_you")
		list.AssertJSONPath("data.0.unread_count", float64(1))
		list.AssertJSONPath("data.0.last_message.excerpt", "Hi Ada, Kim here")
		v.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.unread_count", float64(1))
	})

	t.Run("writing to a closed conversation reopens it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		owner(tc).PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		res := v.client().PostJSON("/widget/v1/conversations/"+id+"/messages", testkit.JSON{
			"client_id": ids.New().String(), "body": "one more thing",
		}).AssertCreated()

		res.AssertJSONPath("meta.reopened", true)
		res.AssertJSONPath("meta.conversation_status", "open")
		res.AssertJSONPath("data.conversation_id", id)
	})

	t.Run("writing past the reopen window starts a linked conversation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		owner(tc).PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()
		tc.Clock.Travel(tc.App.Config.Channels.Email.Inbound.ReopenWindow + time.Hour)

		res := v.client().PostJSON("/widget/v1/conversations/"+id+"/messages", testkit.JSON{
			"client_id": ids.New().String(), "body": "back again",
		}).AssertCreated()

		res.AssertJSONPath("meta.redirected_from", id)
		fresh := res.String("data.conversation_id")
		require.NotEqual(t, id, fresh)

		row := conversationRow(t, tc, fresh)
		require.NotNil(t, row.PreviousConversationID)
		assert.Equal(t, id, models.PublicID(models.PrefixConversation, *row.PreviousConversationID))
	})

	t.Run("another visitor's conversation is not found", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner, stranger := startVisitor(t, tc), startVisitor(t, tc)
		id := owner.startConversation(t, "mine")

		stranger.client().GetJSON("/widget/v1/conversations/"+id).
			AssertNotFound().AssertJSONPath("code", "conversation_not_found")
		stranger.client().GetJSON("/widget/v1/conversations/" + id + "/messages").AssertNotFound()
		stranger.client().PostJSON("/widget/v1/conversations/"+id+"/messages", testkit.JSON{
			"client_id": ids.New().String(), "body": "hijack",
		}).AssertNotFound()
		stranger.client().GetJSON("/widget/v1/conversations/nonsense").AssertNotFound()
		stranger.client().GetJSON("/widget/v1/conversations").AssertOK().AssertJSONPath("data", []any{})
	})

	t.Run("pages scroll back through older messages", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "m0")

		for _, body := range []string{"m1", "m2", "m3"} {
			tc.Clock.Travel(time.Second)
			v.client().PostJSON("/widget/v1/conversations/"+id+"/messages", testkit.JSON{
				"client_id": ids.New().String(), "body": body,
			}).AssertCreated()
		}

		first := v.client().GetJSON("/widget/v1/conversations/" + id + "/messages?limit=2").AssertOK()
		first.AssertJSONPath("data.0.body", "m2")
		first.AssertJSONPath("data.1.body", "m3")

		older := v.client().GetJSON("/widget/v1/conversations/" + id + "/messages?limit=2&before=" +
			first.String("meta.next_cursor")).AssertOK()
		older.AssertJSONPath("data.0.body", "m0")
		older.AssertJSONPath("data.1.body", "m1")

		held := first.String("data.0.id")
		newer := v.client().GetJSON("/widget/v1/conversations/" + id + "/messages?after=" + held).AssertOK()
		newer.AssertJSONPath("data.0.body", "m3")
		newer.AssertJSONMissing("data.1")

		base := "/widget/v1/conversations/" + id + "/messages?"
		v.client().GetJSON(base + "after=nonsense").AssertStatus(fhttp.StatusBadRequest)
		v.client().GetJSON(base + "before=nonsense").AssertStatus(fhttp.StatusBadRequest)
		v.client().GetJSON(base + "after=msg_01m6m7d1yjenvsa0zrhb0vv39z").AssertNotFound()
	})
}

/*
conversationRow reads a conversation by its public id.
*/
func conversationRow(t *testing.T, tc *tests.TestCase, public string) models.Conversation {
	t.Helper()

	id, err := tc.App.IDs.Decode(string(models.PrefixConversation), public)
	require.NoError(t, err)

	var out models.Conversation

	ws := tc.Workspace.Workspace.ID
	inWorkspace(t, tc, ws, func(ctx context.Context, q *models.Queries) error {
		var err error
		out, err = q.FindConversation(ctx, models.FindConversationParams{WorkspaceID: ws, ID: id})

		return err
	})

	return out
}
