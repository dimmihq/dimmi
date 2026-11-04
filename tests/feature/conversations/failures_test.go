package conversations_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

func TestInboxAbilities(t *testing.T) {
	t.Parallel()

	t.Run("refuses every inbox route to a token without its ability", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		_, m := d.join(t, enums.RoleMember)
		path := "/api/v1/conversations/" + d.customer(t, "ann", "Billing", "Hi")
		none := d.tc.ActingAsToken(m, "members:read")

		for _, get := range []string{
			path, path + "/messages", path + "/read-state", path + "/neighbors", "/api/v1/conversations",
			"/api/v1/conversation-counts", "/api/v1/search?q=a", "/api/v1/inboxes", "/api/v1/inboxes/" + d.inboxID(),
			"/api/v1/contacts",
		} {
			none.GetJSON(get).AssertForbidden()
		}

		for _, post := range []string{path + "/replies", path + "/notes", path + "/messages"} {
			none.PostJSON(post, testkit.JSON{"body": "x"}).AssertForbidden()
		}

		none.PatchJSON(path, testkit.JSON{"priority": "high"}).AssertForbidden()
		none.PatchJSON(path+"/read-state", testkit.JSON{}).AssertForbidden()
		none.PostJSON("/api/v1/conversations", testkit.JSON{"body": "x", "contact": testkit.JSON{}}).AssertForbidden()
		none.PostJSON("/api/v1/contacts", testkit.JSON{"name": "x"}).AssertForbidden()
	})
}

func TestInboxStorageFailures(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		breaks func(d desk)
		step   func(d desk, conv string) request
	}{
		{
			"lists when reads are gone", breakTable("conversation_reads"),
			fixed("GET", "/api/v1/conversations?filter[view]=all", nil),
		},
		{"lists when teams are gone", breakTable("teams"), fixed("GET", "/api/v1/conversations?filter[view]=all", nil)},
		{
			"lists when channels are gone", breakTable("channel_connections"),
			fixed("GET", "/api/v1/conversations?filter[view]=all", nil),
		},
		{
			"counts when conversations are gone", breakTable("conversations"),
			fixed("GET", "/api/v1/conversation-counts", nil),
		},
		{"lists inboxes when they are gone", breakTable("inboxes"), fixed("GET", "/api/v1/inboxes", nil)},
		{"lists contacts when they are gone", breakTable("contacts"), fixed("GET", "/api/v1/contacts", nil)},
		{"searches when messages are gone", breakTable("messages"), fixed("GET", "/api/v1/search?q=hello", nil)},
		{"reads the timeline when contacts are gone", breakTable("contacts"), on("GET", "/messages", nil)},
		{"shows the read state when reads are gone", breakTable("conversation_reads"), on("GET", "/read-state", nil)},
		{
			"marks read when reads cannot be written", failWrites("conversation_reads", "INSERT"),
			on("PATCH", "/read-state", testkit.JSON{}),
		},
		{
			"marks unread when reads are gone", breakTable("conversation_reads"),
			on("PATCH", "/read-state", testkit.JSON{"unread": true}),
		},
		{
			"changes status when conversations cannot be written", failWrites("conversations", "UPDATE"),
			on("PATCH", "", testkit.JSON{"status": "closed"}),
		},
		{
			"replies when messages cannot be written", failWrites("messages", "INSERT"),
			on("POST", "/replies", testkit.JSON{"body": "x"}),
		},
		{
			"adds notes when conversations cannot be written", failWrites("conversations", "UPDATE"),
			on("POST", "/notes", testkit.JSON{"body": "x"}),
		},
		{
			"starts conversations when numbers cannot be handed out", failWrites("workspaces", "UPDATE"),
			func(d desk, _ string) request {
				return request{"POST", "/api/v1/conversations", testkit.JSON{
					"inbox_id": d.inboxID(), "author": "contact", "body": "Hi",
					"contact": testkit.JSON{"email": "z@y.co"},
				}}
			},
		},
		{
			"adds inboxes when channels cannot be written", failWrites("channel_connections", "INSERT"),
			fixed("POST", "/api/v1/inboxes", testkit.JSON{"name": "Sales"}),
		},
		{
			"renames inboxes when they cannot be written", failWrites("inboxes", "UPDATE"),
			func(d desk, _ string) request {
				return request{"PATCH", "/api/v1/inboxes/" + d.inboxID(), testkit.JSON{"name": "Other"}}
			},
		},
		{
			"adds contacts when identities cannot be written", failWrites("contact_identities", "INSERT"),
			fixed("POST", "/api/v1/contacts", testkit.JSON{"email": "x@y.co"}),
		},
		{"reads the timeline when messages are gone", breakTable("messages"), on("GET", "/messages", nil)},
		{
			"relays when messages cannot be written", failWrites("messages", "INSERT"),
			on("POST", "/messages", testkit.JSON{"body": "x"}),
		},
		{
			"relays when channels are gone", breakTable("channel_connections"),
			on("POST", "/messages", testkit.JSON{"body": "x"}),
		},
		{"replies when contacts are gone", breakTable("contacts"), on("POST", "/replies", testkit.JSON{"body": "x"})},
		{
			"replies when channels are gone", breakTable("channel_connections"),
			on("POST", "/replies", testkit.JSON{"body": "x"}),
		},
		{
			"adds notes when messages cannot be written", failWrites("messages", "INSERT"),
			on("POST", "/notes", testkit.JSON{"body": "x"}),
		},
		{
			"assigns teams when teams are gone", breakTable("teams"),
			on("PATCH", "", testkit.JSON{"team_id": "team_01jbd0k3a8x9t2m4q5r6s7v8w9"}),
		},
		{
			"marks spam when conversations cannot be written", failWrites("conversations", "UPDATE"),
			on("PATCH", "", testkit.JSON{"spam": true}),
		},
		{
			"changes details when conversations cannot be written", failWrites("conversations", "UPDATE"),
			on("PATCH", "", testkit.JSON{"subject": "New"}),
		},
		{
			"moves inboxes when inboxes are gone", breakTable("inboxes"),
			on("PATCH", "", testkit.JSON{"inbox_id": "inbox_01jbd0k3a8x9t2m4q5r6s7v8w9"}),
		},
		{
			"marks unread when messages are gone", breakTable("messages"),
			on("PATCH", "", testkit.JSON{"unread": true}),
		},
		{"finds neighbors when conversations are gone", breakTable("conversations"), on("GET", "/neighbors", nil)},
		{"shows a conversation when contacts are gone", breakTable("contacts"), on("GET", "", nil)},
		{
			"shows a contact when contacts are gone", breakTable("contacts"),
			fixed("GET", "/api/v1/contacts/ctc_01jbd0k3a8x9t2m4q5r6s7v8w9", nil),
		},
		{"searches contacts when they are gone", breakTable("contacts"), fixed("GET", "/api/v1/search?q=zzz", nil)},
		{"shows an inbox when inboxes are gone", breakTable("inboxes"), func(d desk, _ string) request {
			return request{"GET", "/api/v1/inboxes/" + d.inboxID(), nil}
		}},
	} {
		t.Run("fails cleanly: "+tc.name, func(t *testing.T) {
			t.Parallel()
			d := open(t)
			conv := d.customer(t, "ann", "Billing", "hello there")
			s := tc.step(d, conv)
			tc.breaks(d)

			d.owner.Send(s.method, s.path, s.body).AssertStatus(500)
		})
	}
}

func TestSendChannelMessageJob(t *testing.T) {
	t.Parallel()

	t.Run("leaves alone messages that are gone or no longer pending", func(t *testing.T) {
		t.Parallel()
		d := open(t)
		driver, path := fakeConversation(t, d)
		conv := d.owner.GetJSON(path).AssertOK()
		msg := d.owner.GetJSON(path + "/messages?filter[type]=message").String("data.0.id")

		ws, cid, mid := d.tc.Workspace.Workspace.ID, decode(t, d, models.PrefixConversation, conv.String("data.id")),
			decode(t, d, models.PrefixMessage, msg)
		for _, args := range []jobs.SendChannelMessageArgs{
			{WorkspaceID: ws, ConversationID: cid, MessageID: mid},
			{WorkspaceID: ws, ConversationID: cid, MessageID: cid},
		} {
			require.NoError(t, d.tc.App.Queue.Push(t.Context(), args))
		}

		d.tc.Drain()
		require.Len(t, driver.Sent(), 1)
	})

	t.Run("sends nothing while what the reply needs cannot be read", func(t *testing.T) {
		t.Parallel()

		for name, breaks := range map[string]func(d desk){
			"message":      breakTable("messages"),
			"conversation": breakTable("conversations"),
			"channel":      breakTable("channel_connections"),
			"customer":     breakTable("contacts"),
			"author":       breakTable("workspace_members"),
			"driver": func(d desk) {
				_, err := d.tc.App.DB.Exec(d.tc.T.Context(), "UPDATE channel_connections SET type = 'retired'")
				require.NoError(d.tc.T, err)
			},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				d := open(t)
				driver, path := fakeConversation(t, d)
				d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Held"}).AssertCreated()
				breaks(d)

				d.tc.Drain()

				require.Len(t, driver.Sent(), 1)
			})
		}
	})

	t.Run("retries a reply whose delivery cannot be recorded", func(t *testing.T) {
		t.Parallel()

		for name, refusal := range map[string]error{"sent": nil, "refused": errors.New("timeout")} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				d := open(t)
				driver, path := fakeConversation(t, d)
				if refusal != nil {
					driver.FailNext(refusal)
				}
				d.owner.PostJSON(path+"/replies", testkit.JSON{"body": "Unrecorded"}).AssertCreated()
				d.tc.FailWrites("messages", "UPDATE")

				d.tc.Drain()

				d.tc.Table("river_job").
					AssertCount(1, testkit.Row{"kind": "send_channel_message", "state": "completed"})
			})
		}
	})
}

/*
request is one call a failure test makes.
*/
type request struct {
	method, path string
	body         any
}

/*
fixed returns a request at a fixed path.
*/
func fixed(method, path string, body any) func(desk, string) request {
	return func(desk, string) request { return request{method, path, body} }
}

/*
on returns a request on the test's conversation, at the path below it.
*/
func on(method, below string, body any) func(desk, string) request {
	return func(_ desk, conv string) request {
		return request{method, "/api/v1/conversations/" + conv + below, body}
	}
}

/*
breakTable drops the table for the rest of the test.
*/
func breakTable(table string) func(desk) {
	return func(d desk) { d.tc.BreakTable(table) }
}

/*
failWrites makes the kind of write to the table fail for the rest of the
test.
*/
func failWrites(table, kind string) func(desk) {
	return func(d desk) { d.tc.FailWrites(table, kind) }
}

/*
decode reads a public id of the kind.
*/
func decode(t *testing.T, d desk, prefix models.Prefix, id string) ids.UUID {
	t.Helper()

	uuid, err := d.tc.App.IDs.Decode(string(prefix), id)
	require.NoError(t, err, fmt.Sprintf("decode %s", id))

	return uuid
}
