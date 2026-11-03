package auth_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestOAuthFailures(t *testing.T) {
	t.Parallel()

	t.Run("sends the person back when the provider cannot be reached to start", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_ID=google-client",
			"DIMMI_SOCIALITE__PROVIDERS__GOOGLE__ISSUER=http://127.0.0.1:1",
			"DIMMI_HTTP_CLIENT__ALLOW_PRIVATE_NETWORKS=true",
		))

		res := tc.Client().Send(fhttp.MethodGet, "/auth/google/redirect", nil)

		assertLands(t, res, "/login?error=oauth_unavailable")
	})

	t.Run("signs nobody in while identities cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.BreakTable("oauth_identities")

		assertServerError(tc.SignInWith("github", person("583231", "grace@example.com")))
		tc.Table("sessions").AssertCount(0)
	})

	t.Run("signs nobody in while accounts cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("583231", "grace@example.com"))
		tc.BreakTable("users")

		assertServerError(tc.SignInWith("github", person("583231", "grace@example.com")))
		assertServerError(tc.SignInWith("github", person("1", "ada@example.com")))
	})

	t.Run("creates nothing when the account or its link cannot be written", func(t *testing.T) {
		t.Parallel()

		for _, table := range []string{"users", "oauth_identities", "river_job"} {
			tc := tests.New(t, withProviders())
			tc.FailWrites(table, "INSERT")
			grace := person("583231", "grace@example.com")
			grace.AvatarURL = "https://avatars.example.com/u/583231"

			assertServerError(tc.SignInWith("github", grace))
			tc.Table("users").AssertCount(0, testkit.Row{"email": "grace@example.com"})
		}
	})

	t.Run("refreshes nothing and signs nobody in when a linked identity cannot be updated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("583231", "grace@example.com"))
		tc.FailWrites("oauth_identities", "UPDATE")

		assertServerError(tc.SignInWith("github", person("583231", "grace@example.com")))
	})

	t.Run("keeps a deleted account's identity when it cannot be removed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.SignInWith("github", person("583231", "grace@example.com"))
		_, err := tc.App.DB.Exec(t.Context(), "UPDATE users SET deleted_at = now() WHERE email = 'grace@example.com'")
		require.NoError(t, err)
		tc.FailWrites("oauth_identities", "DELETE")

		assertServerError(tc.SignInWith("github", person("583231", "grace@example.com")))
		tc.Table("oauth_identities").AssertCount(1)
	})

	t.Run("links nothing to an account whose address cannot be marked proven", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		unverified := tc.CreateUser(factories.Unverified, factories.WithoutPassword)
		squatted := tc.CreateUser(factories.Unverified)
		tc.FailWrites("users", "UPDATE")

		assertServerError(tc.SignInWith("github", person("1", unverified.Email)))
		assertServerError(tc.SignInWith("github", person("2", squatted.Email)))
		tc.Table("oauth_identities").AssertCount(0)
	})

	t.Run("signs nobody up while invitations cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		token := inviteGrace(t, tc, 0)
		tc.BreakTable("invitations")
		tc.Social()

		assertServerError(tc.SignInWith("github", person("583231", "grace@example.com")))
		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect?invitation="+token)
		assertServerError(tc.FinishOAuth(tc.Client(), attempt, person("583231", "grace@example.com")))
	})

	t.Run("signs up nobody with an invitation token that matches none", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders(), tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE=invite_only"))
		tc.Social()

		attempt := tc.StartOAuth(tc.Client(), "/auth/github/redirect?invitation=unknown")

		assertLands(t, tc.FinishOAuth(tc.Client(), attempt, person("583231", "grace@example.com")),
			"/login?error=signup_closed")
	})
}

func TestOAuthLinkFailures(t *testing.T) {
	t.Parallel()

	t.Run("links nothing while the account or its identities cannot be read", func(t *testing.T) {
		t.Parallel()

		for _, table := range []string{"oauth_identities", "users"} {
			tc := tests.New(t, withProviders())
			tc.Social()
			browser := tc.ActingAsOwner()
			attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=link")
			tc.BreakTable(table)

			assertServerError(tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com")))
		}
	})

	t.Run("links nothing when the link cannot be logged", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.ActingAsOwner()
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?intent=link")
		tc.FailWrites("security_events", "INSERT")

		assertServerError(tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com")))
		tc.Table("oauth_identities").AssertCount(0)
	})
}
