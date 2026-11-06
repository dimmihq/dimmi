package ai_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/httpclient/httpclienttest"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
chatgptDesk is a desk on an install that offers the subscription
sign-in.
*/
func chatgptDesk(t *testing.T) tests.Desk {
	t.Helper()

	tc := tests.New(t, tests.WithEnv("DIMMI_AI__CHATGPT__ENABLED=true"))
	inbox, err := factories.DefaultInbox(t.Context(), tc.App, tc.Workspace.Workspace)
	if err != nil {
		t.Fatal(err)
	}

	return tests.Desk{TC: tc, Inbox: inbox, Owner: tc.In(tc.Owner)}
}

func TestChatGPTConnectAPI(t *testing.T) {
	t.Parallel()

	t.Run("starts a device sign-in and connects once it is approved", func(t *testing.T) {
		t.Parallel()
		d := chatgptDesk(t)
		fake := httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests()
		fake.Stub("POST https://auth.openai.com/api/accounts/deviceauth/usercode", jsonReply(fhttp.StatusOK,
			`{"device_auth_id":"dev_1","user_code":"ABCD-1234","interval":"5"}`))
		fake.StubSequence("POST https://auth.openai.com/api/accounts/deviceauth/token",
			jsonReply(fhttp.StatusForbidden, `{}`),
			jsonReply(fhttp.StatusOK, `{"authorization_code":"ac_1","code_verifier":"cv_1"}`))
		fake.Stub(exchangeURL, jsonReply(fhttp.StatusOK,
			`{"access_token":"at","refresh_token":"rt","id_token":"`+idToken("acct_9")+`","expires_in":3600}`))

		start := d.Owner.PostJSON("/api/v1/ai/chatgpt/connect", testkit.JSON{}).AssertCreated()
		start.AssertJSONPath("data.method", "device")
		start.AssertJSONPath("data.user_code", "ABCD-1234")
		start.AssertJSONPath("data.url", "https://auth.openai.com/codex/device")
		state := start.String("data.state")
		assert.NotEmpty(t, state)

		pending := d.Owner.PostJSON("/api/v1/ai/chatgpt/callback", testkit.JSON{"state": state}).AssertCreated()
		pending.AssertJSONPath("data.status", "pending")
		pending.AssertJSONMissing("data.key_id")

		d.TC.Clock.Travel(6 * time.Second)
		done := d.Owner.PostJSON("/api/v1/ai/chatgpt/callback", testkit.JSON{"state": state}).AssertCreated()
		done.AssertJSONPath("data.status", "connected")
		done.AssertJSONPath("data.account_id", "acct_9")
		assert.True(t, strings.HasPrefix(done.String("data.key_id"), "aik_"))
	})

	t.Run("hands out the browser address and asks for where it ended", func(t *testing.T) {
		t.Parallel()
		d := chatgptDesk(t)
		httpclienttest.New(d.TC.App.HTTP).PreventStrayRequests()

		start := d.Owner.PostJSON("/api/v1/ai/chatgpt/connect", testkit.JSON{"method": "browser"}).AssertCreated()
		start.AssertJSONPath("data.method", "browser")
		start.AssertJSONPath("data.redirect_uri", "http://localhost:1455/auth/callback")
		assert.True(t, strings.HasPrefix(start.String("data.url"), "https://auth.openai.com/oauth/authorize?"))

		d.Owner.PostJSON("/api/v1/ai/chatgpt/callback", testkit.JSON{"state": start.String("data.state")}).
			AssertStatus(fhttp.StatusUnprocessableEntity).
			AssertJSONPath("code", "oauth_code_required")
	})

	t.Run("refuses a method it does not know", func(t *testing.T) {
		t.Parallel()
		d := chatgptDesk(t)

		d.Owner.PostJSON("/api/v1/ai/chatgpt/connect", testkit.JSON{"method": "fax"}).
			AssertValidationError("method")
	})

	t.Run("says the sign-in is off when the install does not offer it", func(t *testing.T) {
		t.Parallel()
		d := tests.NewDesk(t)

		d.Owner.PostJSON("/api/v1/ai/chatgpt/connect", testkit.JSON{}).
			AssertStatus(fhttp.StatusUnprocessableEntity).
			AssertJSONPath("code", "chatgpt_unavailable")
	})
}
