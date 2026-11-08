package helpcenters_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/jwt"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
token mints the identity token the team's product would sign for the
customer, with a fresh nonce.
*/
func (c center) token(t *testing.T, sub string, attrs map[string]any) string {
	t.Helper()
	secret := c.d.Owner.PostJSON("/api/v1/identity-secrets", testkit.JSON{"name": "Product", "algorithm": "HS256"}).
		AssertCreated()
	key, err := jwt.NewKey(secret.String("data.id"), jwt.HS256, []byte(secret.String("data.secret")))
	require.NoError(t, err)

	now := c.d.TC.Clock.Now()
	claims := jwt.Claims{
		Subject: sub, IssuedAt: now, Expiry: now.Add(5 * time.Minute),
		Extra: map[string]any{"aud": "dimmi-help", "nonce": sub + now.String(), "custom_attributes": attrs},
	}
	signed, err := jwt.Sign(claims, key)
	require.NoError(t, err)

	return signed
}

/*
signIn posts a token to the help center's sign-in, as its page does.
*/
func (c center) signIn(token, next string) *testkit.Response {
	form := url.Values{"token": {token}, "next": {next}}

	return c.d.TC.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
		Send("POST", "/hc/"+c.subdomain+"/sso", form.Encode())
}

/*
as opens a page as the reader the cookie remembers.
*/
func (c center) as(cookie *fhttp.Cookie, path string) *testkit.Response {
	return c.d.TC.Client().WithHeader("Accept", "text/html").WithCookie(cookie).
		Send("GET", "/hc/"+c.subdomain+path, nil)
}

func TestHelpCenterReaders(t *testing.T) {
	t.Parallel()

	t.Run("offers sign-in only when the help center has a login address", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		assert.NotContains(t, string(c.page("").AssertOK().Body), ">Sign in<")

		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{
			"settings": testkit.JSON{"login_url": "https://app.acme.dev/login"},
		}).AssertOK().AssertJSONPath("data.settings.login_url", "https://app.acme.dev/login")
		body := string(c.page("/articles/anything").Body)
		assert.Contains(t, body, "https://app.acme.dev/login?return_to=")
		assert.Contains(t, body, url.QueryEscape("/hc/"+c.subdomain+"/sso?next="))

		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{
			"settings": testkit.JSON{"login_url": "http://app.acme.dev/login"},
		}).AssertStatus(422).AssertJSONPath("code", "login_url_invalid")
	})

	t.Run("shows a signed-in reader what their audiences open, and never caches it", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		pro := c.audience(t, "Pro", testkit.JSON{"rules": testkit.JSON{
			"field": "contact.custom.plan", "op": "eq", "value": "pro",
		}})
		id := c.published(t, testkit.JSON{"title": "Priority support", "body_markdown": "Call us any time."})
		c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": pro}).AssertOK()
		c.page("/articles/priority-support").AssertNotFound()

		next := "/hc/" + c.subdomain + "/articles/priority-support"
		res := c.signIn(c.token(t, "user_9", map[string]any{"plan": "pro"}), next)
		res.AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", next)
		cookie := res.Cookie("dimmi_help_reader")
		require.NotNil(t, cookie)
		assert.True(t, cookie.HttpOnly)

		page := c.as(cookie, "/articles/priority-support").AssertOK()
		assert.Contains(t, string(page.Body), "Call us any time.")
		assert.Equal(t, "private, no-store", page.Header.Get("Cache-Control"))
		assert.Contains(t, string(c.as(cookie, "/search?q=call").AssertOK().Body), "Priority support")
		c.page("/articles/priority-support").AssertNotFound()
	})

	t.Run("does not let a reader in without the audience, a forged cookie or an old one", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		pro := c.audience(t, "Pro", testkit.JSON{"rules": testkit.JSON{
			"field": "contact.custom.plan", "op": "eq", "value": "pro",
		}})
		id := c.published(t, testkit.JSON{"title": "Priority support", "body_markdown": "Call us any time."})
		c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": pro}).AssertOK()

		free := c.signIn(c.token(t, "user_free", map[string]any{"plan": "free"}), "").Cookie("dimmi_help_reader")
		require.NotNil(t, free)
		c.as(free, "/articles/priority-support").AssertNotFound()

		paying := c.signIn(c.token(t, "user_pro", map[string]any{"plan": "pro"}), "").Cookie("dimmi_help_reader")
		require.NotNil(t, paying)
		forged := &fhttp.Cookie{
			Name: paying.Name, Value: paying.Value[:len(paying.Value)-2] + "xx", HttpOnly: true, Secure: true,
			SameSite: fhttp.SameSiteLaxMode,
		}
		c.as(forged, "/articles/priority-support").AssertNotFound()

		c.d.TC.Clock.Travel(2 * time.Hour)
		c.as(paying, "/articles/priority-support").AssertNotFound()
	})

	t.Run("refuses a bad token and sends readers back only within the help center", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)

		c.signIn("not-a-token", "").AssertStatus(401)
		c.signIn(c.token(t, "user_1", nil), "https://evil.example/").
			AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", "/hc/"+c.subdomain)
	})
}
