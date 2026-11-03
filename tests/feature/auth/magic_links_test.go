package auth_test

import (
	"bytes"
	"maps"
	"net/url"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
magicLink asks for a sign-in link for the email, with the extra body
fields, and returns the mail that came.
*/
func magicLink(t *testing.T, tc *tests.TestCase, email string, extra testkit.JSON) appmail.MagicLinkMail {
	t.Helper()

	fake := tc.Mail()
	body := testkit.JSON{"email": email}
	maps.Copy(body, extra)

	tc.Client().PostJSON("/api/v1/magic-links", body).AssertStatus(fhttp.StatusAccepted)
	sent, _ := tests.LastSent[appmail.MagicLinkMail](tc, fake)

	return sent
}

/*
withLink signs in with a mailed link's token.
*/
func withLink(client *testkit.Client, sent appmail.MagicLinkMail, tc *tests.TestCase) *testkit.Response {
	return client.PostJSON("/api/v1/sessions", testkit.JSON{
		"grant": "magic_link", "token": tc.Fragment(sent.URL).Get("token"),
	})
}

/*
withCode signs in with a mailed code typed for the email.
*/
func withCode(client *testkit.Client, email, code string) *testkit.Response {
	return client.PostJSON("/api/v1/sessions", testkit.JSON{"grant": "email_code", "email": email, "code": code})
}

func TestMagicLinkStore(t *testing.T) {
	t.Parallel()

	t.Run("mails a link and a six-digit code with where the request came from", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		sent := magicLink(t, tc, tc.Owner.Email, nil)

		assert.Contains(t, sent.URL, "/login/magic#token=")
		assert.Regexp(t, regexp.MustCompile(`^\d{3}-\d{3}$`), sent.Code)
		assert.Equal(t, "192.0.2.1", sent.IP)
		assert.Equal(t, 15, sent.Minutes)
	})

	t.Run("answers the same whether the address has an account, may sign up, or neither", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		fake := tc.Mail()

		known := tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		unknown := tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": "nobody@example.com"}).
			AssertStatus(fhttp.StatusAccepted)

		assert.True(t, bytes.Equal(known.Body, unknown.Body))
		known.AssertJSONPath("data.message", "If this email can sign in, a link is on its way.")
		_, messages := tests.Sent[appmail.MagicLinkMail](tc, fake)
		require.Len(t, messages, 1, "no link for an address that may not sign up")
		assert.Equal(t, []string{tc.Owner.Email}, tests.SentTo(messages[0]))
	})

	t.Run("drops a fourth request within ten minutes without saying so", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()

		for range 4 {
			tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).
				AssertStatus(fhttp.StatusAccepted)
		}

		sent, _ := tests.Sent[appmail.MagicLinkMail](tc, fake)
		assert.Len(t, sent, 3)
	})

	t.Run("keeps only the three newest links of an address working", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		first := magicLink(t, tc, tc.Owner.Email, nil)
		var newer appmail.MagicLinkMail
		for range 3 {
			tc.Clock.Advance(time.Second)
			require.NoError(t, tc.App.Limiter.Clear(t.Context(), "magic:email:"+tc.Owner.Email))
			newer = magicLink(t, tc, tc.Owner.Email, nil)
		}

		withLink(tc.Client(), first, tc).AssertStatus(fhttp.StatusGone)
		withLink(tc.Client(), newer, tc).AssertCreated()
	})

	t.Run("is not found where links are turned off", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__MAGIC_LINK__ENABLED=false"))

		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).AssertNotFound()
	})

	t.Run("asks the login policy before sending anything", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.LoginPolicy = ssoOnly{} }))

		tc.Client().PostJSON("/api/v1/magic-links", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusConflict).
			AssertJSONPath("code", "sso_required")
	})
}

func TestSessionStoreWithMagicLink(t *testing.T) {
	t.Parallel()

	t.Run("signs in with the link, which opening its address does not spend", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, tc.Owner.Email, nil)
		u, err := url.Parse(sent.URL)
		require.NoError(t, err)
		tc.Client().GetJSON(u.Path)

		res := withLink(tc.Client(), sent, tc).AssertCreated()

		res.AssertJSONPath("data.user.email", tc.Owner.Email).
			AssertJSONPath("data.auth.auth_methods", []string{"magic_link"})
		tc.Table("sessions").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "auth_method": "magic_link"})
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "magic_link.used"})
	})

	t.Run("signs in with the code typed on the device that asked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, tc.Owner.Email, nil)

		withCode(tc.Client(), tc.Owner.Email, sent.Code).AssertCreated().
			AssertJSONPath("data.auth.auth_methods", []string{"email_code"})
	})

	t.Run("works once", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, tc.Owner.Email, nil)
		withLink(tc.Client(), sent, tc).AssertCreated()

		withLink(tc.Client(), sent, tc).AssertStatus(fhttp.StatusGone).AssertJSONPath("code", "magic_link_expired")
		withCode(tc.Client(), tc.Owner.Email, sent.Code).AssertValidationError("code")
	})

	t.Run("stops working after fifteen minutes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, tc.Owner.Email, nil)
		tc.Clock.Advance(15 * time.Minute)

		withLink(tc.Client(), sent, tc).AssertStatus(fhttp.StatusGone)
		withCode(tc.Client(), tc.Owner.Email, sent.Code).AssertValidationError("code")
	})

	t.Run("burns every live link of an address after five wrong codes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, tc.Owner.Email, nil)

		for range appauth.MaxSecretAttempts {
			withCode(tc.Client(), tc.Owner.Email, "000-000").
				AssertValidationError("code").
				AssertJSONPath("code", "invalid_code")
		}

		withCode(tc.Client(), tc.Owner.Email, sent.Code).AssertValidationError("code")
		withLink(tc.Client(), sent, tc).AssertStatus(fhttp.StatusGone)
	})

	t.Run("refuses a code for an address that has no live link like a wrong one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		withCode(tc.Client(), "nobody@example.com", "123-456").
			AssertValidationError("code").
			AssertJSONPath("code", "invalid_code")
	})

	t.Run("signs a new person up through the link, verified and without a password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, "grace@example.com", testkit.JSON{"name": "Grace Hopper"})

		withLink(tc.Client(), sent, tc).AssertCreated().
			AssertJSONPath("data.user.name", "Grace Hopper").
			AssertJSONPath("data.user.email_verified", true).
			AssertJSONPath("data.user.has_password", false).
			AssertJSONPath("data.workspace", nil)
		tc.Table("users").AssertHas(testkit.Row{
			"email": "grace@example.com", "created_via": "magic_link", "password_hash": nil,
		})
	})

	t.Run("names a new account after its address when no name was given", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, "grace.hopper@example.com", nil)

		withCode(tc.Client(), "grace.hopper@example.com", sent.Code).AssertCreated().
			AssertJSONPath("data.user.name", "Grace Hopper")
	})

	t.Run("proves the inbox of an unverified account without a password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser(factories.Unverified, factories.WithoutPassword)
		sent := magicLink(t, tc, user.Email, nil)

		withLink(tc.Client(), sent, tc).AssertCreated().AssertJSONPath("data.user.email_verified", true)
	})

	t.Run("takes back an account someone registered with the address after the link was sent", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		sent := magicLink(t, tc, "grace@example.com", nil)
		squatter := tc.Browser(tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated())
		token := squatterToken(t, tc, "grace@example.com")
		fake := tc.Mail()

		withLink(tc.Client(), sent, tc).AssertCreated().
			AssertJSONPath("data.user.email_verified", true).
			AssertJSONPath("data.user.has_password", false)

		squatter.GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Client().WithHeader("Authorization", "Bearer "+token).GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Client().PostJSON("/api/v1/sessions", login("grace@example.com", factories.Password)).
			AssertValidationError("email")
		user := userByEmail(t, tc, "grace@example.com")
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": user.ID, "type": "sessions.revoked_others"})
		mail, _ := tests.LastSent[appmail.PasswordChangedMail](tc, fake)
		assert.Equal(t, appmail.PasswordWasRemoved, mail.Reason)
	})

	t.Run("re-authenticates the session's own user, and refuses a link for anyone else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		other := tc.CreateUser()
		before := browser.GetJSON("/api/v1/me").String("data.auth.session_id")
		theirs := magicLink(t, tc, other.Email, nil)
		mine := magicLink(t, tc, tc.Owner.Email, nil)

		browser.PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "magic_link", "token": tc.Fragment(theirs.URL).Get("token"), "reauth": true,
		}).AssertValidationError("email").AssertJSONPath("code", "reauth_user_mismatch")
		browser.GetJSON("/api/v1/me").AssertJSONPath("data.auth.session_id", before)

		res := browser.PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "email_code", "email": tc.Owner.Email, "code": mine.Code, "reauth": true,
		}).AssertCreated()
		assert.NotEqual(t, before, res.String("data.auth.session_id"))
	})

	t.Run("refuses a re-authentication through a sign-up link", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		browser := tc.ActingAsOwner()
		sent := magicLink(t, tc, "grace@example.com", nil)

		browser.PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "magic_link", "token": tc.Fragment(sent.URL).Get("token"), "reauth": true,
		}).AssertValidationError("email")
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
	})

	t.Run("refuses both grants where links are turned off", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__MAGIC_LINK__ENABLED=false"))

		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{"grant": "magic_link", "token": "x"}).
			AssertValidationError("grant").
			AssertJSONPath("code", "grant_not_allowed")
		withCode(tc.Client(), tc.Owner.Email, "123-456").AssertJSONPath("code", "grant_not_allowed")
	})

	t.Run("asks the login policy again when the link is used", func(t *testing.T) {
		t.Parallel()
		policy := &switchablePolicy{}
		tc := tests.New(t, tests.Before(func(a *app.App) { a.LoginPolicy = policy }))
		sent := magicLink(t, tc, tc.Owner.Email, nil)
		policy.ssoOnly.Store(true)

		withLink(tc.Client(), sent, tc).AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "sso_required")
		withCode(tc.Client(), tc.Owner.Email, sent.Code).AssertStatus(fhttp.StatusConflict)
	})

	t.Run("asks for the fields of each grant", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{"grant": "magic_link"}).AssertValidationError("token")
		tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{"grant": "email_code"}).
			AssertValidationError("email", "code")
	})

	t.Run("refuses an address that keeps trying links", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		for range 10 {
			withCode(tc.Client(), tc.Owner.Email, "000-000").AssertValidationError("code")
		}

		withCode(tc.Client(), tc.Owner.Email, "000-000").AssertStatus(fhttp.StatusTooManyRequests)
	})
}

/*
squatterToken mints an access token for the only membership of the
account with the email, as someone who registered it would.
*/
func squatterToken(t *testing.T, tc *tests.TestCase, email string) string {
	t.Helper()

	user := userByEmail(t, tc, email)
	member := membershipOf(t, tc, user)
	_, plain := tc.MintToken(member, "*")

	return plain
}
