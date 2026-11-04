package user_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
withProviders configures GitHub and Google, and the extra settings.
*/
func withProviders(extra ...string) tests.Option {
	return tests.WithEnv(append([]string{
		"DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=github-client", "DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_ID=google-client",
	}, extra...)...)
}

/*
signUpGrace signs grace@example.com up with GitHub account 583231 and
returns the browser.
*/
func signUpGrace(tc *tests.TestCase) *testkit.Client {
	tc.T.Helper()

	return tc.Browser(tc.SignInWith("github", socialite.User{
		ID: "583231", Email: "grace@example.com", EmailVerified: true,
	}))
}

/*
identityIDs returns the public ids of the browser's linked accounts.
*/
func identityIDs(t *testing.T, browser *testkit.Client) map[string]string {
	t.Helper()

	var body struct {
		Data []struct {
			ID       string `json:"id"`
			Provider string `json:"provider"`
		} `json:"data"`
	}

	browser.GetJSON("/api/v1/user/identities").AssertOK().Decode(&body)

	ids := map[string]string{}
	for _, identity := range body.Data {
		ids[identity.Provider] = identity.ID
	}

	return ids
}

/*
linkWith links the provider account to the signed-in browser.
*/
func linkWith(tc *tests.TestCase, browser *testkit.Client, provider, id string) {
	tc.T.Helper()

	attempt := tc.StartOAuth(browser, "/auth/"+provider+"/redirect?intent=link")
	tc.FinishOAuth(browser, attempt, socialite.User{ID: id, Email: id + "@example.com", EmailVerified: true}).
		AssertStatus(fhttp.StatusFound)
}

func TestUserIdentities(t *testing.T) {
	t.Parallel()

	t.Run("lists the linked accounts without the provider's own id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		browser := signUpGrace(tc)

		res := browser.GetJSON("/api/v1/user/identities").AssertOK().
			AssertJSONPath("data.0.object", "oauth_identity").
			AssertJSONPath("data.0.provider", "github").
			AssertJSONPath("data.0.email", "grace@example.com").
			AssertJSONPath("data.0.last_login_at", tests.Epoch().Format(time.RFC3339))
		require.NotContains(t, string(res.Body), "583231")
	})

	t.Run("refuses access tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.ActingAsToken(tc.Workspace.Owner, "*").GetJSON("/api/v1/user/identities").
			AssertForbidden().AssertJSONPath("code", "first_party_only")
	})

	t.Run("unlinks an account of a user who keeps a password", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders("DIMMI_AUTH__MAGIC_LINK__ENABLED=false"))
		tc.Social()
		browser := tc.ActingAsOwner()
		linkWith(tc, browser, "github", "583231")

		browser.Delete("/api/v1/user/identities/" + identityIDs(t, browser)["github"]).AssertNoContent()

		tc.Table("oauth_identities").AssertCount(0)
		tc.Table("security_events").AssertHas(testkit.Row{"user_id": tc.Owner.ID, "type": "identity.unlinked"})
	})

	t.Run("asks for a recent confirmation before unlinking", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.ActingAsOwner()
		linkWith(tc, browser, "github", "583231")
		tc.Clock.Travel(tc.App.Config.Auth.PasswordConfirmation.TTL + time.Second)

		browser.Delete("/api/v1/user/identities/"+identityIDs(t, browser)["github"]).
			AssertStatus(fhttp.StatusLocked).AssertJSONPath("code", "password_confirmation_required")
		tc.Table("oauth_identities").AssertCount(1)
	})

	t.Run("unlinking the last way in is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders("DIMMI_AUTH__MAGIC_LINK__ENABLED=false"))
		browser := signUpGrace(tc)

		browser.Delete("/api/v1/user/identities/"+identityIDs(t, browser)["github"]).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "last_login_method")
		tc.Table("oauth_identities").AssertCount(1)
	})

	t.Run("lets a user without a password unlink while mail sign-in reaches their proven address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		browser := signUpGrace(tc)

		browser.Delete("/api/v1/user/identities/" + identityIDs(t, browser)["github"]).AssertNoContent()
	})

	t.Run("lets a user unlink one provider while another still signs them in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders("DIMMI_AUTH__MAGIC_LINK__ENABLED=false"))
		browser := signUpGrace(tc)
		linkWith(tc, browser, "google", "110169")
		linked := identityIDs(t, browser)

		browser.Delete("/api/v1/user/identities/" + linked["github"]).AssertNoContent()
		browser.Delete("/api/v1/user/identities/"+linked["google"]).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "last_login_method")
	})

	t.Run("does not count a provider the install no longer offers as a way in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders("DIMMI_AUTH__MAGIC_LINK__ENABLED=false"))
		browser := signUpGrace(tc)
		user := userByEmail(t, tc, "grace@example.com")
		_, err := tc.App.DB.Exec(t.Context(),
			"INSERT INTO oauth_identities (user_id, provider, provider_user_id) VALUES ($1, 'gitlab', '7')", user.ID)
		require.NoError(t, err)

		browser.Delete("/api/v1/user/identities/"+identityIDs(t, browser)["github"]).
			AssertStatus(fhttp.StatusConflict).AssertJSONPath("code", "last_login_method")
	})

	t.Run("answers not found for another user's identity and for a malformed id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		theirs := identityIDs(t, signUpGrace(tc))["github"]
		browser := tc.ActingAsOwner()

		browser.Delete("/api/v1/user/identities/" + theirs).AssertNotFound()
		browser.Delete("/api/v1/user/identities/ses_nope").AssertNotFound()
		tc.Table("oauth_identities").AssertCount(1)
	})
}

func TestUserIdentityFailures(t *testing.T) {
	t.Parallel()

	t.Run("lists and unlinks nothing while identities cannot be read", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		browser := signUpGrace(tc)
		id := identityIDs(t, browser)["github"]
		tc.BreakTable("oauth_identities")

		browser.GetJSON("/api/v1/user/identities").AssertStatus(fhttp.StatusInternalServerError)
		browser.Delete("/api/v1/user/identities/" + id).AssertStatus(fhttp.StatusInternalServerError)
	})

	t.Run("keeps the identity when it cannot be removed or the unlink cannot be logged", func(t *testing.T) {
		t.Parallel()

		for table, kind := range map[string]string{"oauth_identities": "DELETE", "security_events": "INSERT"} {
			tc := tests.New(t, withProviders())
			browser := signUpGrace(tc)
			id := identityIDs(t, browser)["github"]
			tc.FailWrites(table, kind)

			browser.Delete("/api/v1/user/identities/" + id).AssertStatus(fhttp.StatusInternalServerError)
			tc.Table("oauth_identities").AssertCount(1)
		}
	})
}

/*
userByEmail returns the live account with the address.
*/
func userByEmail(t *testing.T, tc *tests.TestCase, email string) models.User {
	t.Helper()

	user, err := models.New(tc.App.DB).FindUserByEmail(t.Context(), email)
	require.NoError(t, err)

	return user
}
