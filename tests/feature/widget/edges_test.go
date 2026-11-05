package widget_test

import (
	"encoding/json"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestWidgetEdges(t *testing.T) {
	t.Parallel()

	t.Run("a team that is there takes a message without an email", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		patch(t, tc, testkit.JSON{"require_email_when_offline": true})
		listener := tc.ListenAs(tc.Owner, tc.RealtimeURL())
		require.Nil(t, listener.Subscribe("workspace:"+workspaceOf(tc)).Error)
		v := startVisitor(t, tc)

		require.Eventually(t, func() bool {
			return v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
				"client_id": ids.New().String(), "body": "hi",
			}).Status == fhttp.StatusCreated
		}, 5*time.Second, 100*time.Millisecond)
	})

	t.Run("a conversation keeps the page, recent pages and context it started from", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		res := tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{
			"app_id": appID(t, tc), "page": testkit.JSON{"url": "https://acme.com/pricing", "title": "Pricing"},
			"pages": []testkit.JSON{{"url": "https://acme.com/"}},
		}).AssertCreated()
		v := &visitor{tc: tc, token: res.String("data.token"), id: res.String("data.id")}

		id := v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
			"client_id": ids.New().String(), "body": "hi", "context": testkit.JSON{"plan": "pro"},
		}).AssertCreated().String("data.id")

		var source map[string]any
		require.NoError(t, json.Unmarshal(conversationRow(t, tc, id).Source, &source))
		assert.Equal(t, "https://acme.com/pricing", source["page_url"])
		assert.NotEmpty(t, source["recent_pages"])
		assert.Equal(t, map[string]any{"plan": "pro"}, source["context"])
	})

	t.Run("a ping past ten pages forgets the oldest and cuts text on a character", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		pages := make([]testkit.JSON, 0, 10)
		for i := range 10 {
			pages = append(pages, testkit.JSON{"url": "https://acme.com/p" + strconv.Itoa(i)})
		}

		res := tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": appID(t, tc), "pages": pages}).
			AssertCreated()
		v := &visitor{tc: tc, token: res.String("data.token"), id: res.String("data.id")}

		v.client().PostJSON("/widget/v1/pings", testkit.JSON{
			"page": testkit.JSON{"url": "https://acme.com/last"}, "context": testkit.JSON{"note": "a" + strings.Repeat("é", 300)},
		}).AssertStatus(fhttp.StatusAccepted)

		row := v.session(t)

		var recent []struct{ URL string }
		require.NoError(t, json.Unmarshal(row.RecentPages, &recent))
		require.Len(t, recent, 10)
		assert.Equal(t, "https://acme.com/p1", recent[0].URL)
		assert.Equal(t, "https://acme.com/last", recent[9].URL)

		var context map[string]string
		require.NoError(t, json.Unmarshal(row.Context, &context))
		assert.Len(t, context["note"], 499)
	})

	t.Run("a profile change of nothing changes nothing and too many attributes are refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{}).AssertOK()

		attrs := testkit.JSON{}
		for i := range 21 {
			attrs["k"+strconv.Itoa(i)] = "v"
		}

		v.client().PatchJSON("/widget/v1/contact", testkit.JSON{"custom_attributes": attrs}).
			AssertStatus(fhttp.StatusUnprocessableEntity)
	})

	t.Run("a signed-in session must prove who it is to resume", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		_, err := tc.Operator().Exec(t.Context(), "UPDATE widget_sessions SET kind = 'identified'")
		require.NoError(t, err)

		v.client().GetJSON("/widget/v1/session").AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "identity_proof_required")
	})

	t.Run("deleting the widget ends its visitors' sessions", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		owner(tc).Delete(settingsPath(t, tc)).AssertNoContent()

		v.client().GetJSON("/widget/v1/session").AssertUnauthorized()
	})

	t.Run("lists conversations a page at a time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		v.startConversation(t, "first")
		tc.Clock.Travel(time.Second)
		v.startConversation(t, "second")

		page := v.client().GetJSON("/widget/v1/conversations?limit=1").AssertOK()
		page.AssertJSONPath("data.0.last_message.excerpt", "second")

		cursor := url.QueryEscape(page.String("meta.next_cursor"))
		next := v.client().GetJSON("/widget/v1/conversations?limit=1&cursor=" + cursor)
		next.AssertOK().AssertJSONPath("data.0.last_message.excerpt", "first")
	})

	t.Run("polling hands out at most a hundred changes at a time", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		cursor := v.client().GetJSON("/widget/v1/updates").AssertOK().String("meta.cursor")
		tc.Clock.Travel(time.Second)

		team := owner(tc)
		for i := range 101 {
			body := testkit.JSON{"body": "answer " + strconv.Itoa(i)}
			team.PostJSON("/api/v1/conversations/"+id+"/replies", body).AssertCreated()
		}

		first := v.client().GetJSON("/widget/v1/updates?since=" + url.QueryEscape(cursor)).AssertOK()
		first.AssertJSONPath("meta.has_more", true)

		next := url.QueryEscape(first.String("meta.cursor"))
		rest := v.client().GetJSON("/widget/v1/updates?since=" + next).AssertOK()
		rest.AssertJSONPath("meta.has_more", false)
		assert.Contains(t, string(rest.Body), "answer 100")
	})

	t.Run("reading after a message of someone else's conversation is not found", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v, other := startVisitor(t, tc), startVisitor(t, tc)
		mine := v.startConversation(t, "mine")
		theirs := other.startConversation(t, "theirs")
		held := other.client().GetJSON("/widget/v1/conversations/" + theirs + "/messages").AssertOK().
			String("data.0.id")

		v.client().GetJSON("/widget/v1/conversations/" + mine + "/messages?after=" + held).AssertNotFound()
	})

	t.Run("a fixed reply time and an automatic locale are kept", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		patch(t, tc, testkit.JSON{"reply_time": "day", "locale": "auto"})

		tc.Client().GetJSON("/widget/v1/apps/"+appID(t, tc)+"/availability").AssertOK().
			AssertJSONPath("data.reply_time", "day")
	})
}
