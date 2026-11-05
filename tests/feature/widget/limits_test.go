package widget_test

import (
	"net/url"
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

func TestWidgetLimits(t *testing.T) {
	t.Parallel()

	t.Run("refuses a message over 64 KB or with more than ten files", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
			"client_id": ids.New().String(), "body": strings.Repeat("a", 65<<10),
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("body")

		files := make([]string, 11)
		for i := range files {
			files[i] = "att_01m6m7de33ffpazt4mjzygtdjz"
		}

		v.client().PostJSON("/widget/v1/conversations", testkit.JSON{
			"client_id": ids.New().String(), "body": "see", "attachment_ids": files,
		}).AssertStatus(fhttp.StatusUnprocessableEntity).AssertValidationError("attachment_ids")
	})

	t.Run("forgets a time zone that does not exist", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := tc.Client().PostJSON("/widget/v1/sessions", testkit.JSON{
			"app_id": appID(t, tc), "timezone": "Mars/Olympus",
		}).AssertCreated()

		assert.Nil(t, (&visitor{tc: tc, id: res.String("data.id")}).session(t).Timezone)
	})

	t.Run("polling hears that a conversation closed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		id := v.startConversation(t, "hello")
		cursor := v.client().GetJSON("/widget/v1/updates").AssertOK().String("meta.cursor")

		tc.Clock.Travel(time.Second)
		owner(tc).PatchJSON("/api/v1/conversations/"+id, testkit.JSON{"status": "closed"}).AssertOK()

		res := v.client().GetJSON("/widget/v1/updates?since=" + url.QueryEscape(cursor)).AssertOK()
		assert.Contains(t, string(res.Body), `"event":"conversation.closed"`)
	})

	t.Run("a session token opens no stream", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)

		require.NotNil(t, dialStream(t, streamURL(t, tc), map[string]any{"token": v.token}).next().Disconnect)
	})

	t.Run("a stream whose session cannot be read is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		v := startVisitor(t, tc)
		token := v.client().PostJSON("/widget/v1/realtime-tokens", nil).AssertCreated().String("data.token")
		tc.BreakTable("widget_sessions")

		require.NotNil(t, dialStream(t, streamURL(t, tc), map[string]any{"token": token}).next().Disconnect)
	})

	t.Run("moving to the inbox it already uses changes nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		inbox := owner(tc).GetJSON(settingsPath(t, tc)).AssertOK().String("data.inbox.id")

		patch(t, tc, testkit.JSON{"inbox_id": inbox}).AssertJSONPath("data.inbox.id", inbox)
	})
}
