package conversations_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/drivers/channel/fake"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestConversationReplies(t *testing.T) {
	t.Parallel()

	t.Run("replies, takes the conversation and reads it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		created := record[appevents.MessageCreated](d.tc)

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Refunded, sorry!"}).AssertCreated().
			AssertJSONPath("data.object", "message").
			AssertJSONPath("data.type", "message").
			AssertJSONPath("data.body", "Refunded, sorry!").
			AssertJSONPath("data.author.type", "member").
			AssertJSONPath("data.author.id", d.ownerID()).
			AssertJSONPath("data.author.name", d.tc.Owner.Name).
			AssertJSONPath("data.delivery_status", "not_applicable").
			AssertJSONPath("meta.conversation.assignee.id", d.ownerID()).
			AssertJSONPath("meta.conversation.unread", false).
			AssertJSONPath("meta.conversation.message_count", 2)

		require.Len(t, created.all(), 1)
		assert.True(t, created.all()[0].Substantive)
		assertTime(t, d.owner.GetJSON(path).AssertOK(), "data.first_reply_at", d.tc.Clock.Now().Format(time.RFC3339))
	})

	t.Run("closes, reopens or snoozes after replying, as asked", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Done", "then": "close"}).AssertCreated().
			AssertJSONPath("meta.conversation.status", "closed")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "One more thing"}).AssertCreated().
			AssertJSONPath("meta.conversation.status", "closed")
		d.owner.PostJSON(path+"/replies", testkit.JSON{
			"body": "Later", "then": "snooze",
			"snooze": testkit.JSON{"preset": "tomorrow"},
		}).
			AssertStatus(422).AssertJSONPath("code", "invalid_transition")
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().AssertJSONPath("data.#", 3)

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Reopening", "then": "open"}).AssertCreated().
			AssertJSONPath("meta.conversation.status", "open")
		d.owner.PostJSON(path+"/replies", testkit.JSON{
			"body": "Later", "then": "snooze",
			"snooze": testkit.JSON{"until": d.tc.Clock.Now().Add(time.Hour)},
		}).AssertCreated().
			AssertJSONPath("meta.conversation.status", "snoozed")
		d.owner.GetJSON(path+"/messages?filter[type]=event").AssertOK().
			AssertJSONPath("data.2.event.reason", "reply")
	})

	t.Run("refuses a reply that missed a newer message, listing it", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		seen := d.owner.GetJSON(path + "/messages").String("data.0.id")
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Still waiting"}).AssertCreated()

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi", "expected_last_message_id": seen}).
			AssertStatus(409).
			AssertJSONPath("code", "new_messages_since").
			AssertJSONPath("data.0.excerpt", "Still waiting").
			AssertJSONPath("data.0.author.name", "ann")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi", "expected_last_message_id": "nope"}).
			AssertStatus(409)

		latest := d.owner.GetJSON(path + "/messages").String("data.1.id")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi", "expected_last_message_id": latest}).
			AssertCreated()
	})

	t.Run("answers a retried reply with the first one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		body := testkit.JSON{"body": "Once", "client_message_id": "c3f0e7c2"}

		first := d.owner.PostJSON(path+"/replies", body).AssertCreated()
		d.owner.PostJSON(path+"/replies", body).AssertOK().
			AssertHeader("Idempotent-Replayed", "true").
			AssertJSONPath("data.id", first.String("data.id"))
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().AssertJSONPath("data.#", 2)
	})

	t.Run("refuses replies on spam and unknown conversations, and empty bodies", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Buy now")
		d.owner.PatchJSON(path, testkit.JSON{"spam": true}).AssertOK()

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi"}).
			AssertStatus(409).AssertJSONPath("code", "conversation_spam")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": ""}).AssertStatus(422)
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi", "snooze": testkit.JSON{"preset": "tomorrow"}}).
			AssertValidationError("snooze")
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi", "then": "snooze"}).AssertValidationError("snooze")
		d.owner.PostJSON("/api/v1/conversations/conv_nope/replies", testkit.JSON{"body": "Hi"}).AssertNotFound()
		open(t).owner.PostJSON(path+"/replies", testkit.JSON{"body": "Hi"}).AssertNotFound()
	})
}

func TestConversationDelivery(t *testing.T) {
	t.Parallel()

	t.Run("sends a reply on a pushing channel once the request commits", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)

		reply := d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "On its way"}).AssertCreated().
			AssertJSONPath("data.delivery_status", "pending")
		d.tc.Drain()

		require.Len(t, driver.Sent(), 2)
		sent := driver.Sent()[1]
		assert.Equal(t, "On its way", sent.BodyMarkdown)
		assert.Equal(t, "ann@example.com", sent.Recipient.IdentityValue)
		assert.Equal(t, reply.String("data.id"), sent.IdempotencyKey)

		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().
			AssertJSONPath("data.1.delivery_status", "sent").
			AssertJSONPath("data.1.delivery_error", nil)
	})

	t.Run("fails a reply the channel refuses for good, and retries others", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)

		driver.FailNext(fmt.Errorf("bounce: %w", contracts.ErrRecipientUnreachable))
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Bounces"}).AssertCreated()
		d.tc.Drain()
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().
			AssertJSONPath("data.1.delivery_status", "failed").
			AssertJSONPath("data.1.delivery_error", "bounce: channel: recipient unreachable")

		driver.FailNext(errors.New("timeout"))
		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Retried"}).AssertCreated()
		d.tc.Drain()
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().
			AssertJSONPath("data.2.delivery_status", "pending").
			AssertJSONPath("data.2.delivery_error", "timeout")
	})

	t.Run("refuses a reply to a customer the channel cannot reach", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, err := d.tc.App.Channels.Fake()
		require.NoError(t, err)
		conn, err := factories.Connection(t.Context(), d.tc.App, d.inbox.Inbox, fake.Type)
		require.NoError(t, err)

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"channel_connection_id": models.PublicID(models.PrefixChannelConnection, conn.ID), "body": "Hello",
			"contact": testkit.JSON{"external_id": "no-email"},
		}).AssertStatus(422).AssertJSONPath("code", "contact_unreachable_on_channel")
	})
}

func TestConversationNotes(t *testing.T) {
	t.Parallel()

	t.Run("adds a note without touching status, holder or channel", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		d.owner.PatchJSON(path, testkit.JSON{"status": "closed", "spam": true}).AssertOK()
		body := testkit.JSON{"body": "Looks like fraud", "client_message_id": "n1"}

		note := d.owner.PostJSON(path+"/notes", body).AssertCreated().
			AssertJSONPath("data.type", "note").
			AssertJSONPath("data.substantive", false).
			AssertJSONPath("data.delivery_status", nil).
			AssertJSONPath("meta.conversation.status", "closed").
			AssertJSONPath("meta.conversation.assignee", nil).
			AssertJSONPath("meta.conversation.message_count", 1)
		d.owner.PostJSON(path+"/notes", body).AssertOK().AssertJSONPath("data.id", note.String("data.id"))
		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": ""}).AssertStatus(422)
	})

	t.Run("shows notes to teammates as unread", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, _ := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		bob.PatchJSON(path+"/read-state", testkit.JSON{}).AssertOK().AssertJSONPath("data.unread", false)

		d.owner.PostJSON(path+"/notes", testkit.JSON{"body": "FYI"}).AssertCreated()
		bob.GetJSON(path+"/read-state").AssertOK().AssertJSONPath("data.unread", true)
		d.owner.GetJSON(path+"/read-state").AssertOK().AssertJSONPath("data.unread", false)
	})
}

func TestConversationContactMessages(t *testing.T) {
	t.Parallel()

	t.Run("relays a customer message, reopening and waking the conversation, once per key", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		d.owner.PatchJSON(path, testkit.JSON{"status": "closed"}).AssertOK()
		body := testkit.JSON{"body": "Still broken", "dedupe_key": "evt_1"}

		first := d.owner.PostJSON(path+"/messages", body).AssertCreated().
			AssertJSONPath("data.author.type", "contact").
			AssertJSONPath("meta.conversation.status", "open")
		d.owner.PostJSON(path+"/messages", body).AssertOK().AssertJSONPath("data.id", first.String("data.id"))

		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "next_week"}).AssertOK()
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Hello?"}).AssertCreated().
			AssertJSONPath("meta.conversation.status", "open")

		events := sliceOf(d.owner.GetJSON(path+"/messages?filter[type]=event"), "data.#.event.name")
		assert.Equal(t, []string{
			"conversation.closed", "conversation.reopened", "conversation.snoozed", "conversation.unsnoozed",
		}, events)
	})

	t.Run("refuses conversations of other channels", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, path := fakeConversation(t, d)

		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "Hi"}).
			AssertStatus(409).AssertJSONPath("code", "channel_not_api")
	})
}

func TestConversationTimeline(t *testing.T) {
	t.Parallel()

	t.Run("pages back from the latest entries and forward again", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "m0")
		for i := 1; i < 5; i++ {
			d.tc.Clock.Travel(time.Second)
			d.owner.PostJSON(path+"/messages", testkit.JSON{"body": fmt.Sprintf("m%d", i)}).AssertCreated()
		}

		latest := d.owner.GetJSON(path + "/messages?limit=2").AssertOK()
		assert.Equal(t, []string{"m3", "m4"}, sliceOf(latest, "data.#.body"))

		older := d.owner.GetJSON(path + "/messages?limit=2&before=" + latest.String("meta.prev_cursor")).AssertOK()
		assert.Equal(t, []string{"m1", "m2"}, sliceOf(older, "data.#.body"))

		oldest := d.owner.GetJSON(path+"/messages?limit=2&before="+older.String("meta.prev_cursor")).AssertOK().
			AssertJSONPath("meta.prev_cursor", nil)
		assert.Equal(t, []string{"m0"}, sliceOf(oldest, "data.#.body"))

		newer := d.owner.GetJSON(path + "/messages?limit=3&after=" + oldest.String("meta.next_cursor")).AssertOK()
		assert.Equal(t, []string{"m1", "m2", "m3"}, sliceOf(newer, "data.#.body"))

		d.owner.GetJSON(path + "/messages?before=forged").AssertStatus(400)
		d.owner.GetJSON(path + "/messages?after=forged").AssertStatus(400)
		d.owner.GetJSON(path + "/messages?filter[type]=email").AssertStatus(400)
		d.owner.GetJSON(path + "/messages?filter[author]=x").AssertStatus(400)
		d.owner.GetJSON("/api/v1/conversations/conv_nope/messages").AssertNotFound()
	})

	t.Run("names deleted teammates and hides deleted text", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		bob, member := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Charged twice")
		bob.PostJSON(path+"/notes", testkit.JSON{"body": "Mine"}).AssertCreated()
		_, err := d.tc.App.DB.Exec(t.Context(), "DELETE FROM workspace_members WHERE id = $1", member.ID)
		require.NoError(t, err)
		_, err = d.tc.App.DB.Exec(t.Context(), "UPDATE messages SET deleted_at = now() WHERE type = 'message'")
		require.NoError(t, err)

		d.owner.GetJSON(path+"/messages").AssertOK().
			AssertJSONPath("data.0.body", nil).
			AssertJSONPath("data.0.deleted_at", d.owner.GetJSON(path+"/messages").String("data.0.deleted_at")).
			AssertJSONPath("data.1.author.name", "Former teammate").
			AssertJSONPath("data.1.author.id", nil)
	})
}

/*
fakeConversation starts a conversation on a fake channel the owner
wrote, to the customer ann, and returns the driver and its path.
*/
func fakeConversation(t *testing.T, d desk) (*fake.Driver, string) {
	t.Helper()

	driver, err := d.tc.App.Channels.Fake()
	require.NoError(t, err)

	conn, err := factories.Connection(t.Context(), d.tc.App, d.inbox.Inbox, fake.Type)
	require.NoError(t, err)

	res := d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
		"channel_connection_id": models.PublicID(models.PrefixChannelConnection, conn.ID), "body": "Hello",
		"subject": "Welcome", "contact": testkit.JSON{"email": "ann@example.com", "name": "Ann"},
	}).AssertCreated().AssertJSONPath("data.last_message.author.type", "member")
	d.tc.Drain()

	return driver, "/api/v1/conversations/" + res.String("data.id")
}
