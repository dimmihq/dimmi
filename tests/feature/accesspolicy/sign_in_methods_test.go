package accesspolicy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestSignInMethods(t *testing.T) {
	t.Parallel()

	t.Run("only allowed sign-in methods reach the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"allowed_sign_in_methods": []string{"google"}})

		owner.GetJSON("/api/v1/workspace").AssertForbidden().
			AssertJSONPath("code", "sign_in_method_required").
			AssertJSONPath("requirement", "sign_in_method").
			AssertJSONPath("action_url", "/login?reauth=1").
			AssertJSONPath("allowed_methods.0", "google")
	})

	t.Run("a second factor never counts as the method", func(t *testing.T) {
		t.Parallel()
		principal := &appauth.Principal{Kind: appauth.KindSession, SignIn: appauth.SignIn{
			Method: "password", Methods: []string{"password", "passkey"},
		}}

		check := appauth.AccessCheck{
			Principal: principal, Settings: models.SecuritySettings{AllowedSignInMethods: []string{"passkey"}},
		}
		require.Error(t, appauth.SignInMethodRule{}.Check(check))

		check.Settings.AllowedSignInMethods = []string{"password"}
		require.NoError(t, appauth.SignInMethodRule{}.Check(check))
	})

	t.Run("mailed codes count as magic links, resets as passwords, invitations as either", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			method  string
			allowed []string
			pass    bool
		}{
			{"email_code", []string{"magic_link"}, true},
			{"password_reset", []string{"password"}, true},
			{"password_reset", []string{"magic_link"}, false},
			{"invitation", []string{"password"}, true},
			{"setup", []string{"magic_link"}, true},
			{"invitation", []string{"google"}, false},
			{"github", []string{"github"}, true},
			{"sso", []string{"password"}, false},
		}

		for _, c := range cases {
			assert.Equal(t, c.pass, appauth.SignInAllowed(c.method, c.allowed), "%s under %v", c.method, c.allowed)
		}
	})

	t.Run("a token is not a sign-in and passes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		token := tc.ActingAsToken(tc.Workspace.Owner, "settings:read")
		ruled(t, tc, map[string]any{
			"allowed_sign_in_methods": []string{"google"}, "google_allowed_domains": []string{"a.example"},
		})

		token.GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("a bypass on anyone but an owner exempts nothing", func(t *testing.T) {
		t.Parallel()
		principal := &appauth.Principal{Kind: appauth.KindSession, SignIn: appauth.SignIn{
			Method: "password", Methods: []string{"password", appauth.MethodSSOBypass},
		}}
		settings := models.SecuritySettings{AllowedSignInMethods: []string{"sso"}}

		require.Error(t, appauth.SignInMethodRule{}.Check(appauth.AccessCheck{
			Principal: principal, Role: enums.RoleAdmin, Settings: settings,
		}))
		require.NoError(t, appauth.SignInMethodRule{}.Check(appauth.AccessCheck{
			Principal: principal, Role: enums.RoleOwner, Settings: settings,
		}))
	})

	t.Run("a caller cannot remove their own sign-in method", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__MAGIC_LINK__ENABLED=true"))
		owner := in(tc, tc.Owner)

		owner.PatchJSON(policyPath, testkit.JSON{"allowed_sign_in_methods": []string{"magic_link"}}).
			AssertStatus(422).AssertJSONPath("code", "sign_in_method_excludes_caller")
	})

	t.Run("a list with no owner able to sign in is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__MAGIC_LINK__ENABLED=true",
			"DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=id", "DIMMI_AUTH__OAUTH__GITHUB__CLIENT_SECRET=secret"))
		admin := join(t, tc, enums.RoleAdmin)
		_, err := tc.App.DB.Exec(t.Context(), `UPDATE users SET email_verified_at = NULL WHERE id = $1`, tc.Owner.ID)
		require.NoError(t, err)
		_, err = tc.App.DB.Exec(t.Context(), `UPDATE users SET email_verified_at = now() WHERE id = $1`, admin.ID)
		require.NoError(t, err)
		_, err = tc.App.DB.Exec(t.Context(), `UPDATE users SET password_hash = NULL WHERE id = $1`, tc.Owner.ID)
		require.NoError(t, err)
		browser := in(tc, admin)

		browser.PatchJSON(policyPath, testkit.JSON{"allowed_sign_in_methods": []string{"password", "magic_link"}}).
			AssertStatus(422).AssertJSONPath("code", "no_owner_can_sign_in")

		_, err = tc.App.DB.Exec(t.Context(), `INSERT INTO oauth_identities
			(id, user_id, provider, provider_user_id, email, email_verified, created_at, updated_at)
			VALUES (uuidv7(), $1, 'github', 'gh-1', $2, true, now(), now())`, tc.Owner.ID, tc.Owner.Email)
		require.NoError(t, err)

		browser.PatchJSON(policyPath, testkit.JSON{"allowed_sign_in_methods": []string{"password", "github"}}).
			AssertOK()
	})

	t.Run("a method the install does not offer is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.PatchJSON(policyPath, testkit.JSON{"allowed_sign_in_methods": []string{"password", "sso"}}).
			AssertValidationError("allowed_sign_in_methods")
		owner.PatchJSON(policyPath, testkit.JSON{"allowed_sign_in_methods": []string{"password", "google"}}).
			AssertValidationError("allowed_sign_in_methods")
	})

	t.Run("the policy shows what the install offers and the caller's address", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		owner := in(tc, tc.Owner)

		res := from(owner, "203.0.113.4").GetJSON(policyPath).AssertOK().
			AssertJSONPath("data.object", "access_policy").
			AssertJSONPath("data.your_ip", "203.0.113.4").
			AssertJSONPath("data.sso_enforced", false).
			AssertJSONPath("data.require_two_factor", false).
			AssertJSONPath("data.tokens_outside", 0.0).
			AssertJSONPath("data.available_sign_in_methods.0", "password")
		res.AssertJSONMissing("data.sso_connection_ids")
	})
}
