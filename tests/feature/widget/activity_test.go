package widget_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetReadState(t *testing.T) {
	t.Parallel()

	t.Run("reading marks the team's replies read and never moves back", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		reply(t, tc, id, "first")
		reply(t, tc, id, "second")

		messages := v.client().GetJSON("/widget/v1/conversations/" + id + "/messages").AssertOK()
		first, second := messages.String("data.1.id"), messages.String("data.2.id")

		res := v.client().PatchJSON("/widget/v1/conversations/"+id+"/read-state",
			testkit.JSON{"last_read_message_id": second}).AssertOK()
		res.AssertJSONPath("data.object", "widget_read_state")
		res.AssertJSONPath("data.unread_count", float64(0))

		v.client().PatchJSON("/widget/v1/conversations/"+id+"/read-state",
			testkit.JSON{"last_read_message_id": first}).AssertOK().AssertJSONPath("data.unread_count", float64(0))
		v.client().GetJSON("/widget/v1/session").AssertOK().AssertJSONPath("data.unread_count", float64(0))
	})

	t.Run("a message of another conversation is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		one, other := v.startConversation(t, "one"), v.startConversation(t, "two")
		foreign := v.client().GetJSON("/widget/v1/conversations/" + other + "/messages").AssertOK().String("data.0.id")

		v.client().PatchJSON("/widget/v1/conversations/"+one+"/read-state",
			testkit.JSON{"last_read_message_id": foreign}).AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}

func TestWidgetTyping(t *testing.T) {
	t.Parallel()

	t.Run("typing is accepted, never stored, and only on one's own conversation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v, stranger := startVisitor(t, tc), startVisitor(t, tc)
		id := v.startConversation(t, "hello")

		for range 3 {
			v.client().PostJSON("/widget/v1/conversations/"+id+"/typing-indicators",
				testkit.JSON{"state": "started"}).AssertStatus(fhttp.StatusAccepted)
		}

		v.client().PostJSON("/widget/v1/conversations/"+id+"/typing-indicators",
			testkit.JSON{"state": "stopped"}).AssertStatus(fhttp.StatusAccepted)
		tc.Table("messages").AssertCount(1, testkit.Row{})

		stranger.client().PostJSON("/widget/v1/conversations/"+id+"/typing-indicators",
			testkit.JSON{"state": "started"}).AssertNotFound()
		v.client().PostJSON("/widget/v1/conversations/"+id+"/typing-indicators",
			testkit.JSON{"state": "dancing"}).AssertStatus(fhttp.StatusUnprocessableEntity)
	})
}

func TestWidgetPings(t *testing.T) {
	t.Parallel()

	t.Run("records the page without secrets, once per ten seconds", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.client().PostJSON("/widget/v1/pings", testkit.JSON{
			"page":    testkit.JSON{"url": "https://acme.com/settings?token=abc&page=2", "title": "Settings"},
			"context": testkit.JSON{"app_version": "2.3.1", "nested": testkit.JSON{"no": true}},
		}).AssertStatus(fhttp.StatusAccepted)
		v.client().PostJSON("/widget/v1/pings", testkit.JSON{
			"page": testkit.JSON{"url": "https://acme.com/too-soon"},
		}).AssertStatus(fhttp.StatusAccepted)

		row := v.session(t)
		assert.Contains(t, string(row.CurrentPage), "https://acme.com/settings?page=2")
		assert.NotContains(t, string(row.RecentPages), "too-soon")
		assert.Contains(t, string(row.Context), "2.3.1")
		assert.NotContains(t, string(row.Context), "nested")
	})

	t.Run("refuses a context over four kilobytes by keeping the old one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		big := testkit.JSON{}

		for i := range 30 {
			big[string(rune('a'+i%26))+string(rune('a'+i/26))] = strings.Repeat("x", 400)
		}

		v.client().PostJSON("/widget/v1/pings", testkit.JSON{"context": big}).AssertStatus(fhttp.StatusAccepted)
		assert.JSONEq(t, "{}", string(v.session(t).Context))
	})
}
