package helpcenters_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

/*
form posts a sign-in form of the help center.
*/
func (c center) form(path string, values url.Values) *testkit.Response {
	return c.d.TC.Client().WithHeader("Content-Type", "application/x-www-form-urlencoded").
		WithHeader("Accept", "text/html").Send("POST", c.address(path), values.Encode())
}

/*
proPage keeps an article to customers on the pro plan and returns its
path.
*/
func (c center) proPage(t *testing.T) string {
	t.Helper()
	pro := c.audience(t, "Pro", testkit.JSON{"rules": testkit.JSON{
		"field": "contact.custom.plan", "op": "eq", "value": "pro",
	}})
	id := c.published(t, testkit.JSON{"title": "Priority support", "body_markdown": "Call us any time."})
	c.d.Owner.PatchJSON("/api/v1/articles/"+id+"/audience", testkit.JSON{"audience_id": pro}).AssertOK()

	return c.local("/articles/priority-support")
}

/*
customer has the team's product sign in a pro customer with the address
once, as it would through the messenger.
*/
func (c center) customer(t *testing.T, sub, email string) {
	t.Helper()
	token := c.signed(t, sub, map[string]any{
		"aud": "dimmi-help", "email": email, "custom_attributes": map[string]any{"plan": "pro"},
	})
	c.signIn(token, "").AssertStatus(fhttp.StatusSeeOther)
}

/*
code asks for a code at the address and returns the one mailed.
*/
func (c center) code(t *testing.T, email, next string) string {
	t.Helper()
	fake := c.d.TC.Mail()
	page := c.form("/sso/codes", url.Values{"email": {email}, "next": {next}}).AssertOK()
	assert.Contains(t, string(page.Body), "Check your email")
	mailed, _ := tests.LastSent[appmail.ReaderCodeMail](c.d.TC, fake)
	require.Len(t, mailed.Code, 6)

	return mailed.Code
}

func wrongCode(right string) string {
	if right == "000000" {
		return "111111"
	}

	return "000000"
}

func TestHelpCenterEmailSignIn(t *testing.T) {
	t.Parallel()

	t.Run("signs a reader in with an emailed code as the customer with that address", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		next := c.proPage(t)
		c.customer(t, "user_ada", "ada@example.com")
		assert.Contains(t, string(c.page("").AssertOK().Body), c.local("/sso/sign-in?next="))
		form := string(c.page("/sso/sign-in?next=" + url.QueryEscape(next)).AssertOK().Body)
		assert.Contains(t, form, "Email me a code")
		assert.NotContains(t, form, "Continue with your account")

		code := c.code(t, "Ada@Example.com", next)
		wrong := c.form("/sso/codes/check", url.Values{"email": {"Ada@Example.com"}, "code": {wrongCode(code)}})
		wrong.AssertStatus(fhttp.StatusUnprocessableEntity)
		assert.Contains(t, string(wrong.Body), "That code didn")
		assert.Contains(t, string(wrong.Body), "a***@example.com")

		res := c.form("/sso/codes/check", url.Values{"email": {"Ada@Example.com"}, "code": {code}, "next": {next}})
		res.AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", next)
		cookie := res.Cookie("dimmi_help_reader")
		require.NotNil(t, cookie)
		assert.Contains(t, string(c.as(cookie, "/articles/priority-support").AssertOK().Body), "Call us any time.")

		again := c.form("/sso/codes/check", url.Values{"email": {"ada@example.com"}, "code": {code}})
		again.AssertStatus(fhttp.StatusUnprocessableEntity)
		assert.Contains(t, string(again.Body), "That code expired")
	})

	t.Run("makes a stranger a verified lead that the audience does not open", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.proPage(t)
		code := c.code(t, "bob@example.com", "")
		cookie := c.form("/sso/codes/check", url.Values{"email": {"bob@example.com"}, "code": {code}}).
			AssertStatus(fhttp.StatusSeeOther).Cookie("dimmi_help_reader")
		require.NotNil(t, cookie)
		c.as(cookie, "/articles/priority-support").AssertNotFound()

		var kind string
		var verified bool
		require.NoError(t, c.d.TC.Operator().QueryRow(t.Context(),
			"SELECT kind, email_verified FROM contacts WHERE workspace_id = $1 AND email = 'bob@example.com'",
			c.d.TC.Workspace.Workspace.ID).Scan(&kind, &verified))
		assert.Equal(t, "lead", kind)
		assert.True(t, verified)
	})

	t.Run("burns a code after five wrong tries and throttles sending", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		code := c.code(t, "eve@example.com", "")
		for range 5 {
			c.form("/sso/codes/check", url.Values{"email": {"eve@example.com"}, "code": {wrongCode(code)}}).
				AssertStatus(fhttp.StatusUnprocessableEntity)
		}
		locked := c.form("/sso/codes/check", url.Values{"email": {"eve@example.com"}, "code": {code}})
		locked.AssertStatus(fhttp.StatusTooManyRequests)
		assert.Contains(t, string(locked.Body), "Too many wrong codes")

		resent := c.form("/sso/codes", url.Values{"email": {"eve@example.com"}, "resend": {"1"}})
		resent.AssertStatus(fhttp.StatusTooManyRequests)
		assert.Contains(t, string(resent.Body), "Check your email")

		bad := c.form("/sso/codes", url.Values{"email": {"not an address"}})
		bad.AssertStatus(fhttp.StatusUnprocessableEntity)
		assert.Contains(t, string(bad.Body), "Enter an email address")
	})

	t.Run("keeps a reader for seven days and renews them as they come back", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.proPage(t)
		c.customer(t, "user_kim", "kim@example.com")
		code := c.code(t, "kim@example.com", "")
		first := c.form("/sso/codes/check", url.Values{"email": {"kim@example.com"}, "code": {code}}).
			Cookie("dimmi_help_reader")
		require.NotNil(t, first)
		assert.Empty(t, c.as(first, "/articles/priority-support").AssertOK().Header.Values("Set-Cookie"))

		c.d.TC.Clock.Travel(3 * 24 * time.Hour)
		renewed := c.as(first, "/articles/priority-support").AssertOK().Cookie("dimmi_help_reader")
		require.NotNil(t, renewed)

		c.d.TC.Clock.Travel(5 * 24 * time.Hour)
		c.as(first, "/articles/priority-support").AssertNotFound()
		c.as(renewed, "/articles/priority-support").AssertOK()
	})

	t.Run("takes no code when the team turns email sign-in off", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.d.Owner.PatchJSON("/api/v1/help-centers/"+c.id, testkit.JSON{
			"settings": testkit.JSON{"email_sign_in": false},
		}).AssertOK()
		c.page("/sso/sign-in").AssertNotFound()
		off := c.form("/sso/codes", url.Values{"email": {"ada@example.com"}})
		off.AssertStatus(fhttp.StatusUnprocessableEntity)
		assert.Contains(t, string(off.Body), "does not take email sign-in")
	})
}

/*
messengerVisitor starts a messenger session the team's product signed
in for a pro customer, and returns its bearer token.
*/
func (c center) messengerVisitor(t *testing.T, sub string) string {
	t.Helper()
	token := c.signed(t, sub, map[string]any{"custom_attributes": map[string]any{"plan": "pro"}})

	return c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": c.app(), "user_jwt": token}).
		AssertCreated().AssertJSONPath("data.kind", "identified").String("data.token")
}

func (c center) app() string {
	return c.d.Owner.GetJSON("/api/v1/widget-settings").AssertOK().String("data.0.id")
}

/*
passOf takes the pass out of an address the messenger opens.
*/
func passOf(t *testing.T, address string) (string, string) {
	t.Helper()
	page, fragment, ok := strings.Cut(address, "#pass=")
	require.True(t, ok, address)
	u, err := url.Parse(page)
	require.NoError(t, err)

	return fragment, u.Query().Get("next")
}

func TestHelpCenterMessengerPass(t *testing.T) {
	t.Parallel()

	t.Run("opens an article signed in from the messenger, once", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		next := c.proPage(t)
		bearer := c.messengerVisitor(t, "user_pro")
		visitor := c.d.TC.Client().WithHeader("Authorization", "Bearer "+bearer)

		mine := visitor.GetJSON("/widget/v1/help").AssertOK()
		assert.Contains(t, string(mine.Body), "Call us any time.")
		assert.Contains(t, mine.String("data.url"), c.link(""))
		public := c.d.TC.Client().GetJSON("/widget/v1/apps/" + c.app() + "/help").AssertOK()
		assert.NotContains(t, string(public.Body), "Call us any time.")

		res := visitor.PostJSON("/widget/v1/help-passes", testkit.JSON{"article": "priority-support"}).AssertCreated()
		pass, back := passOf(t, res.String("data.url"))
		assert.Equal(t, next, back)

		in := c.form("/sso", url.Values{"pass": {pass}, "next": {back}})
		in.AssertStatus(fhttp.StatusSeeOther).AssertHeader("Location", next)
		cookie := in.Cookie("dimmi_help_reader")
		require.NotNil(t, cookie)
		c.as(cookie, "/articles/priority-support").AssertOK()

		spent := c.form("/sso", url.Values{"pass": {pass}, "next": {back}})
		spent.AssertStatus(fhttp.StatusUnauthorized)
		assert.Contains(t, string(spent.Body), "That link expired")
	})

	t.Run("gives anyone else the plain address and lets a proven email read as its customer", func(t *testing.T) {
		t.Parallel()
		c := newCenter(t)
		c.proPage(t)
		c.customer(t, "user_lee", "lee@example.com")
		started := c.d.TC.Client().PostJSON("/widget/v1/sessions", testkit.JSON{"app_id": c.app()}).AssertCreated()
		visitor := c.d.TC.Client().WithHeader("Authorization", "Bearer "+started.String("data.token"))

		plain := visitor.PostJSON("/widget/v1/help-passes", testkit.JSON{"article": "priority-support"}).AssertCreated()
		assert.True(t, strings.HasSuffix(plain.String("data.url"), c.link("/articles/priority-support")))
		assert.NotContains(t, plain.String("data.url"), "#pass=")

		fake := c.d.TC.Mail()
		visitor.PostJSON("/widget/v1/email-verifications", testkit.JSON{"email": "lee@example.com"}).
			AssertStatus(fhttp.StatusAccepted)
		mailed, _ := tests.LastSent[appmail.VisitorCodeMail](c.d.TC, fake)
		visitor.PostJSON("/widget/v1/email-verifications/checks", testkit.JSON{"code": mailed.Code}).AssertOK()
		assert.Contains(t, string(visitor.GetJSON("/widget/v1/help").AssertOK().Body), "Call us any time.")

		res := visitor.PostJSON("/widget/v1/help-passes", testkit.JSON{"article": "priority-support"}).AssertCreated()
		pass, back := passOf(t, res.String("data.url"))
		cookie := c.form("/sso", url.Values{"pass": {pass}, "next": {back}}).
			AssertStatus(fhttp.StatusSeeOther).Cookie("dimmi_help_reader")
		require.NotNil(t, cookie)
		c.as(cookie, "/articles/priority-support").AssertOK()
	})
}
