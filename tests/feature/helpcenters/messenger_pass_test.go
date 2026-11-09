package helpcenters_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
readerCookie signs a reader in to the help center with an emailed code
and returns the cookie that remembers them.
*/
func (c center) readerCookie(t *testing.T, email string) *fhttp.Cookie {
	t.Helper()
	code := c.code(t, email, "")
	cookie := c.form("/sso/codes/check", url.Values{"email": {email}, "code": {code}}).
		AssertStatus(fhttp.StatusSeeOther).Cookie("dimmi_help_reader")
	require.NotNil(t, cookie)
	return cookie
}

/*
messengerPass asks the help center who is signed in, as the messenger
on its pages does, and returns the pass it hands back.
*/
func (c center) messengerPass(cookie *fhttp.Cookie) *testkit.Response {
	client := c.d.TC.Client()
	if cookie != nil {
		client = client.WithCookie(cookie)
	}
	return client.GetJSON("/hc/" + c.subdomain + "/messenger-identity").AssertOK()
}

func TestHelpCenterMessengerSignIn(t *testing.T) {
	t.Parallel()

	t.Run("signs the messenger in as the help center's signed-in reader, once per pass", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		app := c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		cookie := c.readerCookie(t, "dana@example.org")

		c.messengerPass(nil).AssertJSONPath("token", nil)
		pass := c.messengerPass(cookie).String("token")
		require.NotEmpty(t, pass)
		assert.Contains(t, string(c.as(cookie, "").Body), `data-identity-url="/hc/`+c.subdomain+`/messenger-identity"`)

		started := c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": app, "user_jwt": pass}).
			AssertCreated()
		started.AssertJSONPath("data.kind", "identified")
		contact := c.d.Owner.GetJSON("/api/v1/contacts?query=dana%40example.org").AssertOK().String("data.0.id")
		started.AssertJSONPath("data.contact.id", contact)

		c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": app, "user_jwt": pass}).
			AssertUnauthorized().AssertJSONPath("code", "pass_invalid")
	})

	t.Run("brings the reader's anonymous chat along when they sign in mid-conversation", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		app := c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
		anonymous := c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": app}).AssertCreated()
		lead := anonymous.String("data.contact.id")
		cookie := c.readerCookie(t, "dana@example.org")

		signedIn := c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{
			"app_id": app, "user_jwt": c.messengerPass(cookie).String("token"),
			"previous_token": anonymous.String("data.token"),
		}).AssertCreated()
		assert.NotEqual(t, lead, signedIn.String("data.contact.id"))
		c.d.TC.Client().WithHeader("Authorization", "Bearer "+anonymous.String("data.token")).
			GetJSON("/widget/v1/session").AssertUnauthorized()
	})

	t.Run("refuses a pass on another workspace's messenger", func(t *testing.T) {
		t.Parallel()
		c, other := newCenter(t), newCenter(t)
		pass := c.messengerPass(c.readerCookie(t, "dana@example.org")).String("token")
		elsewhere := other.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")

		other.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": elsewhere, "user_jwt": pass}).
			AssertUnauthorized().AssertJSONPath("code", "pass_invalid")
	})
}
