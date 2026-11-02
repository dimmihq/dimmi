package auth_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestAuthConfigShow(t *testing.T) {
	t.Parallel()

	t.Run("tells a signed-out app how to sign in on this server", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE="))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.object", "auth_config").
			AssertJSONPath("data.edition", "self_host").
			AssertJSONPath("data.region", "self").
			AssertJSONPath("data.server_version", "0.1.0-dev").
			AssertJSONPath("data.api_version", "2026-11-01").
			AssertJSONPath("data.signup_mode", "invite_only").
			AssertJSONPath("data.password_min_length", 10).
			AssertJSONPath("data.mail_configured", false).
			AssertJSONPath("data.providers", []any{}).
			AssertJSONPath("data.factors", []testkit.JSON{
				{"type": "totp", "name": "Authenticator app"},
				{"type": "recovery_code", "name": "Recovery codes"},
			}).
			AssertJSONPath("data.capabilities", []any{}).
			AssertJSONPath("data.min_client_versions", testkit.JSON{"ios": nil, "android": nil, "cli": nil})
	})

	t.Run("offers only the providers with a client id", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=gh-client"))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.providers", []testkit.JSON{
				{"key": "github", "name": "GitHub", "protocol": "oauth2", "redirect_url": "/auth/github/redirect"},
			})
	})

	t.Run("leaves out the factors the install disables", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__FACTORS__ENABLED=recovery_code"))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.factors", []testkit.JSON{{"type": "recovery_code", "name": "Recovery codes"}})
	})

	t.Run("reports the cloud's own defaults", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_APP__EDITION=cloud", "DIMMI_APP__REGION=us", "DIMMI_AUTH__SIGNUP__MODE=",
		))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.edition", "cloud").
			AssertJSONPath("data.region", "us").
			AssertJSONPath("data.signup_mode", "open")
	})
}
