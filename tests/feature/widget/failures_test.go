package widget_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

/*
failure is one request of a visitor that must fail cleanly once a piece
underneath it breaks: nothing half done, a server error answered.
*/
type failure struct {
	prepare func(t *testing.T, v *visitor) string
	breaks  func(tc *tests.TestCase)
	call    func(v *visitor, ref string) *testkit.Response
}

/*
failures lists, by what breaks, the visitor requests that must answer a
server error.
*/
func failures() map[string]failure {
	none := func(*testing.T, *visitor) string { return "" }
	conversation := func(t *testing.T, v *visitor) string {
		t.Helper()

		return v.startConversation(t, "hello")
	}
	upload := func(t *testing.T, v *visitor) string {
		t.Helper()

		return v.upload(t, "notes.txt", "text/plain", []byte("hi"))
	}

	return map[string]failure{
		"announcing a file while uploads cannot be written": {
			none, func(tc *tests.TestCase) { tc.FailWrites("attachments", "INSERT") },
			func(v *visitor, _ string) *testkit.Response { return v.announce("a.txt", "text/plain", 2) },
		},
		"finishing an upload that cannot be marked": {
			upload, func(tc *tests.TestCase) { tc.FailWrites("attachments", "UPDATE") },
			func(v *visitor, id string) *testkit.Response { return v.complete(id) },
		},
		"listing conversations whose messages cannot be read": {
			conversation, func(tc *tests.TestCase) { tc.BreakTable("attachments") },
			func(v *visitor, _ string) *testkit.Response { return v.client().GetJSON("/widget/v1/conversations") },
		},
		"reading messages that cannot be read": {
			conversation, func(tc *tests.TestCase) { tc.BreakTable("attachments") },
			func(v *visitor, id string) *testkit.Response {
				return v.client().GetJSON("/widget/v1/conversations/" + id + "/messages")
			},
		},
		"sending into a conversation that cannot be written": {
			conversation, func(tc *tests.TestCase) { tc.FailWrites("messages", "INSERT") },
			func(v *visitor, id string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/conversations/"+id+"/messages", testkit.JSON{
					"client_id": ids.New().String(), "body": "again",
				})
			},
		},
		"marking read while messages cannot be written": {
			func(t *testing.T, v *visitor) string {
				t.Helper()

				id := v.startConversation(t, "hello")
				reply(t, v.tc, id, "on it")
				last := v.client().GetJSON("/widget/v1/conversations/" + id + "/messages").AssertOK()

				return id + " " + last.String("data.1.id")
			},
			func(tc *tests.TestCase) { tc.FailWrites("messages", "UPDATE") },
			func(v *visitor, ref string) *testkit.Response {
				id, message, _ := strings.Cut(ref, " ")

				return v.client().PatchJSON("/widget/v1/conversations/"+id+"/read-state", testkit.JSON{
					"last_read_message_id": message,
				})
			},
		},
		"writing a profile that cannot be saved": {
			none, func(tc *tests.TestCase) { tc.FailWrites("contacts", "UPDATE") },
			func(v *visitor, _ string) *testkit.Response {
				return v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"name": "Grace"})
			},
		},
		"starting a session that cannot be stored": {
			none, func(tc *tests.TestCase) { tc.FailWrites("widget_sessions", "INSERT") },
			func(v *visitor, _ string) *testkit.Response {
				return v.tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appIDOf(v)})
			},
		},
		"starting a session whose lead cannot be stored": {
			none, func(tc *tests.TestCase) { tc.FailWrites("contacts", "INSERT") },
			func(v *visitor, _ string) *testkit.Response {
				return v.tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appIDOf(v)})
			},
		},
		"listing conversations that cannot be read": {
			conversation, func(tc *tests.TestCase) { tc.BreakTable("conversations") },
			func(v *visitor, _ string) *testkit.Response { return v.client().GetJSON("/widget/v1/conversations") },
		},
		"creating a widget whose channel cannot be stored": {
			none, func(tc *tests.TestCase) { tc.FailWrites("channel_connections", "INSERT") },
			func(v *visitor, _ string) *testkit.Response {
				return owner(v.tc).PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "Shop"})
			},
		},
		"resuming a session whose visit cannot be noted": {
			func(t *testing.T, v *visitor) string {
				t.Helper()
				v.tc.Clock.Travel(2 * time.Minute)

				return ""
			},
			func(tc *tests.TestCase) { tc.FailWrites("widget_sessions", "UPDATE") },
			func(v *visitor, _ string) *testkit.Response { return v.client().GetJSON("/widget/v1/session") },
		},
		"reading a profile that cannot be read": {
			none, func(tc *tests.TestCase) { tc.BreakTable("contacts") },
			func(v *visitor, _ string) *testkit.Response { return v.client().GetJSON("/widget/v1/contact") },
		},
		"leaving an email that cannot be kept": {
			none, func(tc *tests.TestCase) { tc.FailWrites("contact_identities", "INSERT", "UPDATE", "DELETE") },
			func(v *visitor, _ string) *testkit.Response {
				return v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"email": "grace@shop.example"})
			},
		},
		"starting a conversation that cannot be stored": {
			none, func(tc *tests.TestCase) { tc.FailWrites("conversations", "INSERT") },
			func(v *visitor, _ string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
					"client_id": ids.New().String(), "body": "hello",
				})
			},
		},
		"sending a file that cannot be linked": {
			func(t *testing.T, v *visitor) string {
				t.Helper()
				id := v.upload(t, "notes.txt", "text/plain", []byte("hi"))
				v.complete(id).AssertOK()

				return id
			},
			func(tc *tests.TestCase) { tc.FailWrites("attachments", "UPDATE") },
			func(v *visitor, id string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
					"client_id": ids.New().String(), "body": "see", "attachment_ids": []string{id},
				})
			},
		},
		"typing in a conversation that cannot be read": {
			conversation, func(tc *tests.TestCase) { tc.BreakTable("conversations") },
			func(v *visitor, id string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/conversations/"+id+"/typing-indicators",
					testkit.JSON{"state": "started"})
			},
		},
		"deleting a widget whose sessions cannot be ended": {
			none, func(tc *tests.TestCase) { tc.FailWrites("widget_sessions", "UPDATE") },
			func(v *visitor, _ string) *testkit.Response {
				return owner(v.tc).Delete("/api/v1/widget-settings/" + appIDOf(v))
			},
		},
		"creating a widget that cannot be stored": {
			none, func(tc *tests.TestCase) { tc.FailWrites("widget_settings", "INSERT") },
			func(v *visitor, _ string) *testkit.Response {
				return owner(v.tc).PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "Shop"})
			},
		},
		"using a session that cannot be read": {
			none, func(tc *tests.TestCase) { tc.BreakTable("widget_sessions") },
			func(v *visitor, _ string) *testkit.Response { return v.client().GetJSON("/widget/v1/session") },
		},
		"listing conversations whose latest messages cannot be read": {
			conversation, func(tc *tests.TestCase) { tc.BreakTable("messages") },
			func(v *visitor, _ string) *testkit.Response { return v.client().GetJSON("/widget/v1/conversations") },
		},
		"catching up while channels cannot be read": {
			func(t *testing.T, v *visitor) string {
				t.Helper()
				v.startConversation(t, "hello")

				return v.client().GetJSON("/widget/v1/updates").AssertOK().String("meta.cursor")
			},
			func(tc *tests.TestCase) { tc.BreakTable("channel_connections") },
			func(v *visitor, cursor string) *testkit.Response {
				return v.client().GetJSON("/widget/v1/updates?since=" + url.QueryEscape(cursor))
			},
		},
		"reading the team's widgets while inboxes cannot be read": {
			none, func(tc *tests.TestCase) { tc.BreakTable("inboxes") },
			func(v *visitor, _ string) *testkit.Response { return owner(v.tc).GetJSON("/api/v1/widget-settings") },
		},
		"creating a widget while inboxes cannot be read": {
			none, func(tc *tests.TestCase) { tc.BreakTable("inboxes") },
			func(v *visitor, _ string) *testkit.Response {
				return owner(v.tc).PostJSON("/api/v1/widget-settings", testkit.JSON{"name": "Shop"})
			},
		},
		"moving a widget to an inbox while channels cannot be written": {
			func(t *testing.T, v *visitor) string {
				t.Helper()

				return owner(v.tc).PostJSON("/api/v1/inboxes", testkit.JSON{"name": "Sales"}).AssertCreated().
					String("data.id")
			},
			func(tc *tests.TestCase) { tc.FailWrites("channel_connections", "UPDATE") },
			func(v *visitor, inbox string) *testkit.Response {
				return owner(v.tc).PatchJSON("/api/v1/widget-settings/"+appIDOf(v), testkit.JSON{"inbox_id": inbox})
			},
		},
		"reading replies whose authors cannot be read": {
			func(t *testing.T, v *visitor) string {
				t.Helper()
				id := v.startConversation(t, "hello")
				reply(t, v.tc, id, "on it")

				return id
			},
			func(tc *tests.TestCase) { tc.BreakTable("workspace_members") },
			func(v *visitor, id string) *testkit.Response {
				return v.client().GetJSON("/widget/v1/conversations/" + id + "/messages")
			},
		},
		"reading messages while messages cannot be read": {
			conversation, func(tc *tests.TestCase) { tc.BreakTable("messages") },
			func(v *visitor, id string) *testkit.Response {
				return v.client().GetJSON("/widget/v1/conversations/" + id + "/messages")
			},
		},
		"sending files while uploads cannot be read": {
			none, func(tc *tests.TestCase) { tc.BreakTable("attachments") },
			func(v *visitor, _ string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
					"client_id": ids.New().String(), "body": "see",
					"attachment_ids": []string{"att_01m6m7de33ffpazt4mjzygtdjz"},
				})
			},
		},
		"starting a conversation that cannot be marked": {
			none, func(tc *tests.TestCase) { tc.FailWrites("conversations", "UPDATE") },
			func(v *visitor, _ string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
					"client_id": ids.New().String(), "body": "hello",
				})
			},
		},
		"starting a conversation whose message cannot be stamped": {
			none, func(tc *tests.TestCase) { tc.FailWrites("messages", "UPDATE") },
			func(v *visitor, _ string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
					"client_id": ids.New().String(), "body": "hello",
				})
			},
		},
		"signing out while sessions cannot be written": {
			none, func(tc *tests.TestCase) { tc.FailWrites("widget_sessions", "UPDATE") },
			func(v *visitor, _ string) *testkit.Response { return v.client().Delete("/widget/v1/session") },
		},
		"pinging while the page cannot be kept": {
			none, func(tc *tests.TestCase) { tc.FailWrites("widget_sessions", "UPDATE") },
			func(v *visitor, _ string) *testkit.Response {
				return v.client().PostJSON("/widget/v1/pings", testkit.JSON{"context": testkit.JSON{}})
			},
		},
		"catching up while messages cannot be read": {
			func(t *testing.T, v *visitor) string {
				t.Helper()

				id := v.startConversation(t, "hello")
				cursor := v.client().GetJSON("/widget/v1/updates").AssertOK().String("meta.cursor")
				reply(t, v.tc, id, "late")

				return cursor
			},
			func(tc *tests.TestCase) { tc.BreakTable("attachments") },
			func(v *visitor, cursor string) *testkit.Response {
				return v.client().GetJSON("/widget/v1/updates?since=" + url.QueryEscape(cursor))
			},
		},
	}
}

func TestWidgetFailures(t *testing.T) {
	t.Parallel()

	for name, f := range failures() {
		t.Run("answers a server error "+name, func(t *testing.T) {
			t.Parallel()
			tc := tests.New(t)
			v := startVisitor(t, tc)
			ref := f.prepare(t, v)

			f.breaks(tc)

			f.call(v, ref).AssertStatus(fhttp.StatusInternalServerError)
		})
	}
}

/*
appIDOf returns the app id of the visitor's widget.
*/
func appIDOf(v *visitor) string {
	t, _ := v.tc.T.(*testing.T)

	return appID(t, v.tc)
}
