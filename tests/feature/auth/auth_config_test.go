package auth_test

import (
	"testing"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/tests"
)

func TestAuthConfigShow(t *testing.T) {
	t.Parallel()

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

	t.Run("leaves out signing in by mail where magic links are turned off", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__MAGIC_LINK__ENABLED=false"))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().
			AssertJSONPath("data.magic_link", false).
			AssertJSONPath("data.capabilities", []any{})
	})

	t.Run("reports mail as configured once a real mailer is named", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv(
			"DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=smtp", "DIMMI_MAIL__MAILERS__SYSTEM__HOST=localhost",
		))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().AssertJSONPath("data.mail_configured", true)
	})
}
