package widget_test

import (
	"net/url"
	"strings"
	"testing"

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
