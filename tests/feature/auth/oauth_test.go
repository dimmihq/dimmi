package auth_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
withProviders configures GitHub and Google as an operator would.
*/
func withProviders() tests.Option {
	return tests.WithEnv(
		"DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=github-client", "DIMMI_AUTH__OAUTH__GITHUB__CLIENT_SECRET=github-secret",
		"DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_ID=google-client", "DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_SECRET=google-secret",
	)
}

/*
person is someone a provider signs in under its id, with a verified
address.
*/
func person(id, email string) socialite.User {
	return socialite.User{ID: id, Email: email, EmailVerified: true, Name: "Grace Hopper"}
}

/*
assertLands checks the response sends the browser to the location.
*/
func assertLands(t *testing.T, res *testkit.Response, location string) {
	t.Helper()

	res.AssertStatus(fhttp.StatusFound)
	assert.Equal(t, location, res.Header.Get("Location"))
}

/*
identityOf returns the identity of the provider's account.
*/
func identityOf(t *testing.T, tc *tests.TestCase, provider, subject string) models.OauthIdentity {
	t.Helper()

	identity, err := models.New(tc.App.DB).FindOAuthIdentity(t.Context(), models.FindOAuthIdentityParams{
		Provider: provider, ProviderUserID: subject,
	})
	require.NoError(t, err)

	return identity
}

func TestOAuthRedirect(t *testing.T) {
	t.Parallel()

	t.Run("sends the browser to the provider with a sealed attempt cookie", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()

		res := tc.Client().Send(fhttp.MethodGet, "/auth/github/redirect", nil).AssertStatus(fhttp.StatusFound)

		authURL, err := url.Parse(res.Header.Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "S256", authURL.Query().Get("code_challenge_method"))
		assert.Equal(t, "http://localhost:8080/auth/github/callback", authURL.Query().Get("redirect_uri"))
		cookie := res.Cookie(tc.App.Cookies.OAuthName())
		assert.True(t, cookie.HttpOnly)
		assert.Equal(t, fhttp.SameSiteLaxMode, cookie.SameSite)
		assert.NotContains(t, cookie.Value, authURL.Query().Get("state"))
		assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))
	})

	t.Run("answers not found for a provider without a client id or without a driver", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=github-client"))

		tc.Client().Send(fhttp.MethodGet, "/auth/google/redirect", nil).AssertNotFound()
		tc.Client().Send(fhttp.MethodGet, "/auth/microsoft/redirect", nil).AssertNotFound()
		tc.Client().Send(fhttp.MethodGet, "/auth/google/callback?code=x&state=y", nil).AssertNotFound()
	})

	t.Run("refuses an unknown intent", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())

		tc.Client().Send(fhttp.MethodGet, "/auth/github/redirect?intent=steal", nil).AssertNotFound()
	})

	t.Run("refuses to link without a browser session, even with a token", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		member := membershipOf(t, tc, tc.Owner)

		tc.Client().Send(fhttp.MethodGet, "/auth/github/redirect?intent=link", nil).AssertUnauthorized()
		tc.ActingAsToken(member, "*").Send(fhttp.MethodGet, "/auth/github/redirect?intent=link", nil).
			AssertUnauthorized()
	})

	t.Run("asks a session that signed in long ago to confirm before linking", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		browser := tc.ActingAsOwner()
		tc.Clock.Travel(tc.App.Config.Auth.PasswordConfirmation.TTL + 1)

		browser.Send(fhttp.MethodGet, "/auth/github/redirect?intent=link", nil).
			AssertStatus(fhttp.StatusLocked).AssertJSONPath("code", "password_confirmation_required")
	})
}

func TestOAuthSignIn(t *testing.T) {
	t.Parallel()

	t.Run("signs a new person up, verified, without a password, named by the provider", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())

		res := tc.SignInWith("github", person("583231", "grace@example.com"))

		assertLands(t, res, "/")
		tc.Browser(res).GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.email", "grace@example.com").
			AssertJSONPath("data.user.name", "Grace Hopper").
			AssertJSONPath("data.user.email_verified", true).
			AssertJSONPath("data.user.has_password", false).
			AssertJSONPath("data.auth.auth_methods", []any{"github"})
		user := userByEmail(t, tc, "grace@example.com")
		tc.Table("users").AssertHas(testkit.Row{"id": user.ID, "created_via": "github", "password_hash": nil})
		tc.Table("oauth_identities").AssertHas(testkit.Row{
			"user_id": user.ID, "provider": "github", "provider_user_id": "583231", "email_verified": true,
		})
	})

	t.Run("names a new person after their address when the provider gives no name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())

		res := tc.SignInWith("google", socialite.User{ID: "1", Email: "ada.king@example.com", EmailVerified: true})

		tc.Browser(res).GetJSON("/api/v1/me").AssertJSONPath("data.user.name", "Ada King")
	})

	t.Run("signs a linked person in by the provider's id even after their address changed there", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("583231", "grace@example.com"))
		tc.Clock.Travel(time.Minute)

		res := tc.SignInWith("github", person("583231", "grace@navy.mil"))

		tc.Browser(res).GetJSON("/api/v1/me").AssertJSONPath("data.user.email", "grace@example.com")
		identity := identityOf(t, tc, "github", "583231")
		assert.Equal(t, "grace@navy.mil", *identity.Email)
		assert.Equal(t, tc.Clock.Now(), identity.LastLoginAt.UTC())
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@navy.mil"})
	})

	t.Run("lands on the page the attempt asked for, and never on another site", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()

		for path, landing := range map[string]string{
			"/inbox?view=mine": "/inbox?view=mine", "//evil.example": "/", "/\\evil.example": "/",
			"https://evil.example/": "/",
		} {
			attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect?redirect="+url.QueryEscape(path))
			assertLands(t, tc.FinishOAuth(tc.Client(), attempt, person("583231", "grace@example.com")), landing)
		}
	})

	t.Run("refuses a first sign-in whose address the provider did not verify", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		unverified := person("583231", tc.Owner.Email)
		unverified.EmailVerified = false

		res := tc.SignInWith("github", unverified)

		assertLands(t, res, "/login?error=oauth_email_unverified")
		tc.Table("oauth_identities").AssertCount(0)
	})

	t.Run("sends an account protected by a second factor to answer it, holding the challenge", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		account, err := factories.WithTwoFactor(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)

		res := tc.SignInWith("github", person("583231", account.User.Email))

		assertLands(t, res, "/login/two-factor?redirect=%2F")
		assert.NotEmpty(t, res.Cookie(tc.App.Cookies.TwoFactorName()).Value)
		tc.Table("oauth_identities").AssertCount(1, testkit.Row{"user_id": account.User.ID})
		tc.Table("sessions").AssertCount(0, testkit.Row{"user_id": account.User.ID})
		tc.Table("one_time_tokens").AssertHas(testkit.Row{
			"user_id": account.User.ID, "purpose": "two_factor_challenge",
		})
	})

	t.Run("refuses an address the login policy sends to single sign-on", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.Before(func(a *app.App) { a.LoginPolicy = ssoOnly{} }))

		assertLands(t, tc.SignInWith("github", person("583231", "grace@example.com")), "/login?error=sso_required")
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
	})
}

func TestOAuthCallbackState(t *testing.T) {
	t.Parallel()

	t.Run("refuses a callback without the attempt cookie, as from another browser", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect")

		callback, err := tc.Social().Approve(attempt.AuthURL, person("583231", "grace@example.com"))
		require.NoError(t, err)

		assertLands(t, tc.Client().Send(fhttp.MethodGet, callback, nil), "/login?error=oauth_state")
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
	})

	t.Run("refuses the cookie of another attempt, a forged cookie and a second use", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		first := tc.StartOAuth(tc.Client(), "/auth/github/redirect")
		second := tc.StartOAuth(tc.Client(), "/auth/github/redirect")
		callback, err := tc.Social().Approve(first.AuthURL, person("583231", "grace@example.com"))
		require.NoError(t, err)

		assertLands(t, tc.Client().WithCookie(second.Cookie).Send(fhttp.MethodGet, callback, nil),
			"/login?error=oauth_state")
		forged := *first.Cookie
		forged.Value = "forged"
		assertLands(t, tc.Client().WithCookie(&forged).Send(fhttp.MethodGet, callback, nil), "/login?error=oauth_state")

		assertLands(t, tc.Client().WithCookie(first.Cookie).Send(fhttp.MethodGet, callback, nil), "/")
		assertLands(t, tc.Client().WithCookie(first.Cookie).Send(fhttp.MethodGet, callback, nil),
			"/login?error=oauth_state")
	})

	t.Run("refuses an attempt started for another provider", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		attempt := tc.StartOAuth(tc.Client(), "/auth/google/redirect")
		callback, err := tc.Social().Approve(attempt.AuthURL, person("583231", "grace@example.com"))
		require.NoError(t, err)
		query, err := url.Parse(callback)
		require.NoError(t, err)

		res := tc.Client().WithCookie(attempt.Cookie).
			Send(fhttp.MethodGet, "/auth/github/callback?"+query.RawQuery, nil)

		assertLands(t, res, "/login?error=oauth_state")
	})

	t.Run("tells the person they declined at the provider", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect")
		authURL, err := url.Parse(attempt.AuthURL)
		require.NoError(t, err)
		denied := "/auth/github/callback?error=access_denied&state=" + authURL.Query().Get("state")

		assertLands(t, tc.Client().WithCookie(attempt.Cookie).Send(fhttp.MethodGet, denied, nil),
			"/login?error=oauth_denied")
	})

	t.Run("tells the person the provider failed when the code is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect")
		authURL, err := url.Parse(attempt.AuthURL)
		require.NoError(t, err)
		forged := "/auth/github/callback?code=forged&state=" + authURL.Query().Get("state")

		assertLands(t, tc.Client().WithCookie(attempt.Cookie).Send(fhttp.MethodGet, forged, nil),
			"/login?error=oauth_unavailable")
	})
}

func TestOAuthAccountMatching(t *testing.T) {
	t.Parallel()

	t.Run("links the provider to the account with its verified address and mails the owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		fake := tc.Mail()

		res := tc.SignInWith("github", person("583231", tc.Owner.Email))

		tc.Browser(res).GetJSON("/api/v1/me").AssertJSONPath("data.user.id",
			models.PublicID(models.PrefixUser, tc.Owner.ID))
		tc.Table("oauth_identities").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "provider": "github"})
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "identity.linked"})
		mail, message := tests.LastSent[appmail.IdentityLinkedMail](tc, fake)
		assert.Equal(t, "GitHub", mail.Provider)
		assert.Equal(t, tc.Owner.Email, message.Message.To[0].Email)
	})

	t.Run("takes an unverified account back from whoever set its password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		squatter := tc.Browser(tc.Client().PostJSON("/api/v1/users", registration("grace@example.com")).AssertCreated())
		token := squatterToken(t, tc, "grace@example.com")
		fake := tc.Mail()

		res := tc.SignInWith("google", person("110169", "grace@example.com"))

		tc.Browser(res).GetJSON("/api/v1/me").
			AssertJSONPath("data.user.email_verified", true).
			AssertJSONPath("data.user.has_password", false)
		squatter.GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Client().WithHeader("Authorization", "Bearer "+token).GetJSON("/api/v1/me").AssertUnauthorized()
		user := userByEmail(t, tc, "grace@example.com")
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": user.ID, "type": "sessions.revoked_others"})
		mail, _ := tests.LastSent[appmail.PasswordChangedMail](tc, fake)
		assert.Equal(t, appmail.PasswordWasRemoved, mail.Reason)
	})

	t.Run("refuses a second account of a provider for a user who has one linked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("1", tc.Owner.Email))

		assertLands(t, tc.SignInWith("github", person("2", tc.Owner.Email)), "/login?error=provider_already_linked")
		tc.Table("oauth_identities").AssertCount(1)
	})

	t.Run("lets the address sign up again once the account it was linked to is deleted", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("583231", "grace@example.com"))
		old := userByEmail(t, tc, "grace@example.com")
		_, err := tc.Operator().Exec(t.Context(),
			"UPDATE users SET deleted_at = now(), email = 'deleted-' || id || '@invalid' WHERE id = $1", old.ID)
		require.NoError(t, err)

		res := tc.SignInWith("github", person("583231", "grace@example.com"))

		tc.Browser(res).GetJSON("/api/v1/me").AssertOK()
		user := userByEmail(t, tc, "grace@example.com")
		assert.NotEqual(t, old.ID, user.ID)
		tc.Table("oauth_identities").AssertCount(1, testkit.Row{"user_id": user.ID})
	})
}

func TestOAuthSignupRules(t *testing.T) {
	t.Parallel()

	t.Run("refuses to sign up a new person while signup is invite only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))

		assertLands(t, tc.SignInWith("github", person("583231", "grace@example.com")), "/login?error=signup_closed")
		tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
	})

	t.Run("signs up a person an invitation waits for while signup is invite only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		inviteGrace(t, tc, time.Hour)

		assertLands(t, tc.SignInWith("github", person("583231", "grace@example.com")), "/")
		tc.Table("users").AssertCount(1, testkit.Row{"email": "grace@example.com"})
	})

	t.Run("ignores an invitation that expired or was revoked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		token := inviteGrace(t, tc, time.Hour)
		_, err := tc.Operator().Exec(t.Context(), "UPDATE invitations SET status = 'revoked'")
		require.NoError(t, err)
		inviteGrace(t, tc, -time.Hour)
		tc.Social()
		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect?invitation="+token)

		assertLands(t, tc.FinishOAuth(tc.Client(), attempt, person("583231", "grace@example.com")),
			"/login?error=signup_closed")
	})

	t.Run("carries an invitation to the invitation page, or says it was for another address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		token := inviteGrace(t, tc, time.Hour)
		tc.Social()

		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect?invitation="+token)
		assertLands(t, tc.FinishOAuth(tc.Client(), attempt, person("583231", "grace@example.com")),
			"/invitation#token="+token)

		attempt = tc.StartOAuth(tc.Client(), "/auth/github/redirect?invitation="+token)
		assertLands(t, tc.FinishOAuth(tc.Client(), attempt, person("1", tc.Owner.Email)),
			"/invitation?error=email_mismatch#token="+token)
	})
}

/*
inviteGrace stores a pending invitation of the test's workspace to
grace@example.com, expiring after the duration, and returns its token.
*/
func inviteGrace(t *testing.T, tc *tests.TestCase, expiresIn time.Duration) string {
	t.Helper()

	token := testkit.UniqueName("invitation")
	_, err := tc.Operator().Exec(t.Context(),
		"INSERT INTO invitations (workspace_id, email, role, token_hash, expires_at, last_sent_at) "+
			"VALUES ($1, $2, 'member', sha256($3::bytea), $4, $5)",
		tc.Workspace.Workspace.ID, "grace@example.com", token, tc.Clock.Now().Add(expiresIn), tc.Clock.Now())
	require.NoError(t, err)

	return token
}
