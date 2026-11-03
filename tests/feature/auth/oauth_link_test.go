package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

func TestOAuthLinking(t *testing.T) {
	t.Parallel()

	t.Run("links the provider to the signed-in user and returns to the security settings", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.ActingAsOwner()
		fake := tc.Mail()

		attempt := tc.StartOAuth(browser, "/auth/google/redirect?intent=link")
		res := tc.FinishOAuth(browser, attempt, person("110169", "someone-else@example.com"))

		assertLands(t, res, "/settings/security")
		tc.Table("oauth_identities").AssertHas(testkit.Row{
			"user_id": tc.Owner.ID, "provider": "google", "email": "someone-else@example.com", "last_login_at": nil,
		})
		mail, _ := tests.LastSent[appmail.IdentityLinkedMail](tc, fake)
		assert.Equal(t, "Google", mail.Provider)
	})

	t.Run("only refreshes an account already linked to the user and lands where asked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.ActingAsOwner()
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=link")
		tc.FinishOAuth(browser, attempt, person("583231", "old@example.com"))

		attempt = tc.StartOAuth(browser, "/auth/github/redirect?intent=link&redirect=/settings/profile")
		res := tc.FinishOAuth(browser, attempt, person("583231", "new@example.com"))

		assertLands(t, res, "/settings/profile")
		tc.Table("oauth_identities").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID, "email": "new@example.com"})
		tc.Table("security_events").AssertCount(1, testkit.Row{"user_id": tc.Owner.ID, "type": "identity.linked"})
	})

	t.Run("refuses a second account of a provider the user has linked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.ActingAsOwner()
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=link")
		tc.FinishOAuth(browser, attempt, person("1", "one@example.com"))

		attempt = tc.StartOAuth(browser, "/auth/github/redirect?intent=link")

		assertLands(t, tc.FinishOAuth(browser, attempt, person("2", "two@example.com")),
			"/settings/security?error=provider_already_linked")
	})

	t.Run("links nothing to an account deleted while the person was at the provider", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.ActingAsOwner()
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=link")
		_, err := tc.App.DB.Exec(t.Context(), "UPDATE users SET deleted_at = now() WHERE id = $1", tc.Owner.ID)
		require.NoError(t, err)

		assertLands(t, tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com")),
			"/settings/security?error=not_found")
		tc.Table("oauth_identities").AssertCount(0)
	})

	t.Run("refuses an account another user linked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("583231", "grace@example.com"))
		browser := tc.ActingAsOwner()

		attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=link")

		assertLands(t, tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com")),
			"/settings/security?error=identity_taken")
		tc.Table("oauth_identities").AssertCount(0, testkit.Row{"user_id": tc.Owner.ID})
	})

	t.Run("refuses a link finished in a browser signed in as someone else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		attempt := tc.StartOAuth(tc.ActingAsOwner(), "/auth/github/redirect?intent=link")

		res := tc.FinishOAuth(tc.ActingAs(tc.CreateUser()), attempt, person("583231", "grace@example.com"))

		assertLands(t, res, "/login?error=oauth_state")
		tc.Table("oauth_identities").AssertCount(0)
	})
}

func TestOAuthReauth(t *testing.T) {
	t.Parallel()

	t.Run("signs the session's own user in afresh on a new session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("583231", tc.Owner.Email))
		browser := tc.ActingAsOwner()
		before := browser.GetJSON("/api/v1/me").String("data.auth.session_id")
		tc.Clock.Travel(time.Minute)

		attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=reauth")
		res := tc.FinishOAuth(browser, attempt, person("583231", tc.Owner.Email))

		me := tc.Client().WithCookie(res.Cookie(tc.App.Session.CookieName())).GetJSON("/api/v1/me").AssertOK()
		assert.NotEqual(t, before, me.String("data.auth.session_id"))
		me.AssertJSONPath("data.auth.auth_methods", []any{"github"})
		browser.GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "login.succeeded"})
	})

	t.Run("changes nothing when the provider signs in someone else", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.ActingAsOwner()
		before := browser.GetJSON("/api/v1/me").String("data.auth.session_id")

		attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=reauth")
		res := tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com"))

		assertLands(t, res, "/login?error=reauth_user_mismatch")
		browser.GetJSON("/api/v1/me").AssertJSONPath("data.auth.session_id", before)
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
	})

	t.Run("is a plain sign-in for a browser without a session", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()

		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect?intent=reauth")

		assertLands(t, tc.FinishOAuth(tc.Client(), attempt, person("583231", "grace@example.com")), "/")
	})
}
