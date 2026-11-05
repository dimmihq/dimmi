package conversations_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/drivers/channel/fake"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
)

func TestInboxWriteFailures(t *testing.T) {
	t.Parallel()

	missing := "/api/v1/conversations/99"

	for _, tc := range []struct {
		name   string
		status int
		breaks func(d desk)
		step   func(d desk, conv string) request
	}{
		{
			"closing when the timeline cannot be written", 500, failWrites("messages", "INSERT"),
			on("PATCH", "", testkit.JSON{"status": "closed"}),
		},
		{
			"renaming when the timeline cannot be written", 500, failWrites("messages", "INSERT"),
			on("PATCH", "", testkit.JSON{"subject": "New"}),
		},
		{
			"prioritizing when the timeline cannot be written", 500, failWrites("messages", "INSERT"),
			on("PATCH", "", testkit.JSON{"priority": "high"}),
		},
		{
			"marking spam when the timeline cannot be written", 500, failWrites("messages", "INSERT"),
			on("PATCH", "", testkit.JSON{"spam": true}),
		},
		{
			"assigning when conversations cannot be written", 500, failWrites("conversations", "UPDATE"),
			on("PATCH", "", testkit.JSON{"assignee_id": "me"}),
		},
		{
			"counting unread when reads are gone", 500, breakTable("conversation_reads"),
			fixed("GET", "/api/v1/conversation-counts", nil),
		},
		{
			"listing when inboxes are gone", 500, breakTable("inboxes"),
			fixed("GET", "/api/v1/conversations?filter[view]=all", nil),
		},
		{
			"replying with a dedupe key when messages are gone", 500, breakTable("messages"),
			on("POST", "/replies", testkit.JSON{"body": "x", "client_message_id": "c1"}),
		},
		{
			"relaying when contacts are gone", 500, breakTable("contacts"),
			on("POST", "/messages", testkit.JSON{"body": "x"}),
		},
		{"starting when conversations cannot be written", 500, failWrites("conversations", "INSERT"), started},
		{"starting when messages cannot be written", 500, failWrites("messages", "INSERT"), started},
		{
			"starting on a channel when channels are gone", 500, breakTable("channel_connections"),
			func(desk, string) request {
				return request{"POST", "/api/v1/conversations", testkit.JSON{
					"channel_connection_id": "chn_01jbd0k3a8x9t2m4q5r6s7v8w9", "body": "Hi",
					"contact": testkit.JSON{"email": "z@y.co"},
				}}
			},
		},
		{"starting in an inbox when channels are gone", 500, breakTable("channel_connections"), started},
		{
			"adding a contact when contacts cannot be written", 500, failWrites("contacts", "INSERT"),
			fixed("POST", "/api/v1/contacts", testkit.JSON{"name": "Ann"}),
		},
		{
			"trusting attributes when contacts cannot be changed", 500, failWrites("contacts", "UPDATE"),
			fixed("POST", "/api/v1/contacts", testkit.JSON{"custom_attributes": testkit.JSON{"plan": "pro"}}),
		},
		{
			"adding a contact when identities are gone", 500, breakTable("contact_identities"),
			fixed("POST", "/api/v1/contacts", testkit.JSON{"email": "ann@x.dev"}),
		},
		{
			"changing a contact when contacts cannot be changed", 500, failWrites("contacts", "UPDATE"),
			func(d desk, conv string) request {
				id := d.owner.GetJSON("/api/v1/conversations/" + conv).String("data.contact.id")

				return request{"PATCH", "/api/v1/contacts/" + id, testkit.JSON{"name": "Bob"}}
			},
		},
		{"reading a conversation that is not there", 404, noBreak, fixed("GET", missing+"/messages", nil)},
		{
			"noting on a conversation that is not there", 404, noBreak,
			fixed("POST", missing+"/notes", testkit.JSON{"body": "x"}),
		},
		{
			"relaying to a conversation that is not there", 404, noBreak,
			fixed("POST", missing+"/messages", testkit.JSON{"body": "x"}),
		},
		{
			"changing a conversation that is not there", 404, noBreak,
			fixed("PATCH", missing, testkit.JSON{"priority": "high"}),
		},
		{"showing a read state that is not there", 404, noBreak, fixed("GET", missing+"/read-state", nil)},
		{
			"moving a read state that is not there", 404, noBreak,
			fixed("PATCH", missing+"/read-state", testkit.JSON{}),
		},
		{"starting for a contact that is not there", 404, noBreak, func(d desk, _ string) request {
			return request{"POST", "/api/v1/conversations", testkit.JSON{
				"inbox_id": d.inboxID(), "author": "contact", "body": "Hi",
				"contact": testkit.JSON{"id": "ctc_01jbd0k3a8x9t2m4q5r6s7v8w9"},
			}}
		}},
	} {
		t.Run("answers "+tc.name, func(t *testing.T) {
			t.Parallel()
			d := open(t)
			conv := d.customer(t, "ann", "Billing", "hello there")
			s := tc.step(d, conv)
			tc.breaks(d)

			d.owner.Send(s.method, s.path, s.body).AssertStatus(tc.status)
		})
	}
}

func TestInboxDeliveryFailures(t *testing.T) {
	t.Parallel()

	t.Run("keeps no reply when its delivery cannot be queued", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, path := fakeConversation(t, d)
		d.tc.FailWrites("river_job", "INSERT")

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "x"}).AssertStatus(500)
		d.owner.GetJSON(path+"/messages?filter[type]=message").AssertOK().AssertJSONPath("data.#", 1)
	})

	t.Run("starts no conversation when its first message cannot be queued", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, err := d.tc.App.Channels.Fake()
		require.NoError(t, err)
		conn, err := factories.Connection(t.Context(), d.tc.App, d.inbox.Inbox, fake.Type)
		require.NoError(t, err)
		d.tc.FailWrites("river_job", "INSERT")

		d.owner.PostJSON("/api/v1/conversations", testkit.JSON{
			"channel_connection_id": models.PublicID(models.PrefixChannelConnection, conn.ID), "body": "Hello",
			"contact": testkit.JSON{"email": "ann@example.com"},
		}).AssertStatus(500)
	})

	t.Run("refuses a reply on a channel this server cannot send on", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		conv := d.customer(t, "ann", "Billing", "Hi")
		sms, err := factories.Connection(t.Context(), d.tc.App, d.inbox.Inbox, "sms")
		require.NoError(t, err)
		_, err = d.tc.Operator().Exec(t.Context(), "UPDATE conversations SET channel_connection_id = $1", sms.ID)
		require.NoError(t, err)

		d.owner.PostJSON("/api/v1/conversations/"+conv+"/replies", testkit.JSON{"body": "x"}).
			AssertStatus(422).AssertJSONPath("code", "channel_unsupported")
	})

	t.Run("lists what arrived since even when it has no text", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Hi")
		seen := d.owner.GetJSON(path + "/messages").String("data.0.id")
		d.owner.PostJSON(path+"/messages", testkit.JSON{"body": "later"}).AssertCreated()
		_, err := d.tc.Operator().Exec(t.Context(), "UPDATE messages SET body = NULL WHERE body = 'later'")
		require.NoError(t, err)

		d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "x", "expected_last_message_id": seen}).
			AssertStatus(409).AssertJSONPath("data.0.excerpt", "")
	})

	t.Run("falls back to the workspace's time zone for an unknown one", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, err := d.tc.Operator().Exec(t.Context(), "UPDATE users SET timezone = 'Mars/Olympus'")
		require.NoError(t, err)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Hi")

		d.owner.PatchJSON(path, testkit.JSON{"status": "snoozed", "snooze_preset": "tomorrow"}).AssertOK()
	})

	t.Run("finds nothing for an id that cannot be read", func(t *testing.T) {
		t.Parallel()
		d := open(t)

		d.owner.GetJSON("/api/v1/search?q=conv_uuuuuuuuuuuuuuuuuuuuuuuuuu").AssertOK().AssertJSONPath("data.jump", nil)
	})
}

/*
started returns a request starting a conversation in the desk's inbox.
*/
func started(d desk, _ string) request {
	return request{"POST", "/api/v1/conversations", testkit.JSON{
		"inbox_id": d.inboxID(), "author": "contact", "body": "Hi", "contact": testkit.JSON{"email": "z@y.co"},
	}}
}

/*
noBreak leaves the application whole.
*/
func noBreak(desk) {}
