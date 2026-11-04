package conversations_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
heldReply sends a reply asking for the undo window and returns its id.
*/
func heldReply(t *testing.T, d desk, path string, body testkit.JSON) string {
	t.Helper()

	body["delay"] = true

	return d.owner.PostJSON(path+"/replies", body).AssertCreated().String("data.id")
}

/*
sendDue runs the send job of the message as if its time had come, and
works every job ready.
*/
func sendDue(t *testing.T, d desk, path, message string) {
	t.Helper()

	conv := d.owner.GetJSON(path).AssertOK().String("data.id")
	require.NoError(t, d.tc.App.Queue.Push(t.Context(), jobs.SendChannelMessageArgs{
		WorkspaceID:    d.tc.Workspace.Workspace.ID,
		ConversationID: decode(t, d, models.PrefixConversation, conv),
		MessageID:      decode(t, d, models.PrefixMessage, message),
	}))
	d.tc.Drain()
}

func TestUndoSend(t *testing.T) {
	t.Parallel()

	t.Run("holds a dashboard reply on a pushing channel for the window, then sends it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)

		id := heldReply(t, d, path, testkit.JSON{"body": "Held"})
		d.tc.Drain()

		require.Len(t, driver.Sent(), 1)
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertJSONPath("data.1.delivery_status", "pending").
			AssertJSONPath("data.1.scheduled_send_at", d.tc.Clock.Now().Add(5*time.Second).Local().Format(time.RFC3339))

		d.tc.Clock.Travel(6 * time.Second)
		sendDue(t, d, path, id)

		require.Len(t, driver.Sent(), 2)
		assert.Equal(t, "Held", driver.Sent()[1].BodyMarkdown)
		d.owner.Send("DELETE", path+"/messages/"+id, nil).AssertStatus(409).AssertJSONPath("code", "already_sent")
	})

	t.Run("sends at once when not asked, from a token, or where replies show as written", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)
		token := d.tc.ActingAsToken(d.tc.Workspace.Owner, "conversations:write").
			WithHeader("Dimmi-Workspace", workspaceID(d.tc))

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Now"}).AssertCreated()
		token.PostJSON(path+"/replies", testkit.JSON{"body": "Program", "delay": true}).AssertCreated().
			AssertJSONPath("data.scheduled_send_at", nil)
		d.tc.Drain()
		require.Len(t, driver.Sent(), 3)

		api := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		heldReply(t, d, api, testkit.JSON{"body": "Shown"})
		d.owner.GetJSON(api+"/messages?filter[type]=message").AssertJSONPath("data.1.scheduled_send_at", nil)
	})

	t.Run("follows the member's own window, else the workspace's, none meaning at once", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)

		d.owner.PatchJSON("/api/v1/workspace", testkit.JSON{"undo_send_seconds": 7}).AssertValidationError(
			"undo_send_seconds",
		)
		d.owner.PatchJSON("/api/v1/workspace", testkit.JSON{"undo_send_seconds": 0}).AssertOK().
			AssertJSONPath("data.undo_send_seconds", float64(0))
		heldReply(t, d, path, testkit.JSON{"body": "At once"})
		d.tc.Drain()
		require.Len(t, driver.Sent(), 2)

		_, err := d.tc.App.DB.Exec(t.Context(), "UPDATE workspace_members SET undo_send_seconds = 20")
		require.NoError(t, err)
		heldReply(t, d, path, testkit.JSON{"body": "Mine"})
		in20 := d.tc.Clock.Now().Add(20 * time.Second).Local().Format(time.RFC3339)
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertJSONPath("data.2.scheduled_send_at", in20)
	})

	t.Run("undoes the reply, puts the conversation back and sends nothing", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": nil}).AssertOK()
		d.tc.Clock.Travel(time.Minute)

		id := heldReply(t, d, path, testkit.JSON{"body": "Oops", "then": "close"})
		d.owner.GetJSON(path).AssertJSONPath("data.status", "closed").AssertJSONPath("data.assignee.id", d.ownerID())

		d.owner.Send("DELETE", path+"/messages/"+id, nil).AssertOK().AssertJSONPath("data.undone", true).
			AssertJSONPath("data.body", "Oops").AssertJSONPath("meta.conversation.status", "open").
			AssertJSONPath("meta.conversation.assignee", nil)
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertJSONPath("data.#", 1)
		d.owner.Send("DELETE", path+"/messages/"+id, nil).AssertNotFound()

		d.tc.Clock.Travel(time.Minute)
		sendDue(t, d, path, id)
		require.Len(t, driver.Sent(), 1)
	})

	t.Run("keeps what teammates changed meanwhile", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, path := fakeConversation(t, d)
		_, bob := d.join(t, enums.RoleMember)
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": nil}).AssertOK()

		id := heldReply(t, d, path, testkit.JSON{"body": "Oops", "then": "close"})
		d.owner.PatchJSON(path, testkit.JSON{"assignee_id": memberID(bob), "status": "open"}).AssertOK()

		d.owner.Send("DELETE", path+"/messages/"+id, nil).AssertOK().
			AssertJSONPath("meta.conversation.status", "open").
			AssertJSONPath("meta.conversation.assignee.id", memberID(bob))
	})

	t.Run("lets only the writer undo, and only a reply still waiting", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, path := fakeConversation(t, d)
		admin, _ := d.join(t, enums.RoleAdmin)
		id := heldReply(t, d, path, testkit.JSON{"body": "Mine"})

		admin.Send("DELETE", path+"/messages/"+id, nil).AssertNotFound()
		d.owner.Send("DELETE", path+"/messages/msg_nonsense", nil).AssertNotFound()
		d.owner.Send("DELETE", "/api/v1/conversations/conv_nonsense/messages/"+id, nil).AssertNotFound()

		d.tc.Clock.Travel(5 * time.Second)
		d.owner.Send("DELETE", path+"/messages/"+id, nil).AssertStatus(409)
	})
}
