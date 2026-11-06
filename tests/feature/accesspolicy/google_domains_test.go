package accesspolicy_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/twofactor"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestGoogleDomains(t *testing.T) {
	t.Parallel()

	google := func(t *testing.T, tc *tests.TestCase, hd string) *testkit.Client {
		t.Helper()

		res := tc.SignInWith("google", socialite.User{
			ID: "google-" + tc.Owner.Email, Email: tc.Owner.Email, EmailVerified: true, Name: tc.Owner.Name,
			Raw: map[string]any{"hd": hd},
		})

		return working(tc, tc.Browser(res))
	}

	configured := func() tests.Option {
		return tests.WithEnv(
			"DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_ID=id", "DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_SECRET=secret",
			"DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=id", "DIMMI_AUTH__OAUTH__GITHUB__CLIENT_SECRET=secret",
		)
	}

	t.Run("a google session from another domain is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, configured())
		browser := google(t, tc, "other.example")
		ruled(t, tc, map[string]any{"google_allowed_domains": []string{"acme.example"}})

		browser.GetJSON("/api/v1/workspace").AssertForbidden().
			AssertJSONPath("code", "sign_in_method_required").
			AssertJSONPath("allowed_domains.0", "acme.example")
	})

	t.Run("a google session from a listed domain works", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, configured())
		browser := google(t, tc, "acme.example")
		ruled(t, tc, map[string]any{"google_allowed_domains": []string{"acme.example"}})

		browser.GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("a personal gmail session is refused when domains are set", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, configured())
		browser := google(t, tc, "")
		ruled(t, tc, map[string]any{"google_allowed_domains": []string{"acme.example"}})

		browser.GetJSON("/api/v1/workspace").AssertForbidden().AssertJSONPath("code", "sign_in_method_required")
	})

	t.Run("a password session is not judged by the google list", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		ruled(t, tc, map[string]any{"google_allowed_domains": []string{"acme.example"}})

		owner.GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("a google caller must keep their own domain on the list", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, configured())
		browser := google(t, tc, "acme.example")

		browser.PatchJSON(policyPath, testkit.JSON{"google_allowed_domains": []string{"other.example"}}).
			AssertStatus(422).AssertJSONPath("code", "sign_in_method_excludes_caller")
		browser.PatchJSON(policyPath, testkit.JSON{
			"google_allowed_domains": []string{"ACME.example", "other.example"},
		}).AssertOK().AssertJSONPath("data.google_allowed_domains.0", "acme.example")
	})

	t.Run("the domain survives a second factor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, configured())
		account, err := factories.WithTwoFactor(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		res := tc.SignInWith("google", socialite.User{
			ID: "g-1", Email: tc.Owner.Email, EmailVerified: true, Name: tc.Owner.Name,
			Raw: map[string]any{"hd": "acme.example"},
		})
		code, err := twofactor.Code(account.Secret, tc.Clock.Now())
		require.NoError(t, err)

		answered := tc.Client().WithCookie(res.Cookie(tc.App.Cookies.TwoFactorName())).
			PostJSON("/api/v1/sessions", testkit.JSON{"grant": "two_factor", "method": "totp", "code": code}).
			AssertCreated()

		ruled(t, tc, map[string]any{"google_allowed_domains": []string{"acme.example"}})
		working(tc, tc.Browser(answered)).GetJSON("/api/v1/workspace").AssertOK()
	})
}
