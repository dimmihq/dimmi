//go:build !cloud

package auth_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestSelfHostEdition(t *testing.T) {
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
				{"type": "passkey", "name": "Passkey"},
				{"type": "recovery_code", "name": "Recovery codes"},
			}).
			AssertJSONPath("data.magic_link", true).
			AssertJSONPath("data.terms", nil).
			AssertJSONPath("data.capabilities", []string{"magic_link", "email_code"}).
			AssertJSONPath("data.min_client_versions", testkit.JSON{"ios": nil, "android": nil, "cli": nil})
	})

	t.Run("stays self-hosted whatever the settings say", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_APP__EDITION=cloud"))

		tc.Client().GetJSON("/api/v1/auth/config").AssertOK().AssertJSONPath("data.edition", "self_host")
	})

	t.Run("starts closed to strangers", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_AUTH__SIGNUP__MODE="))

		tc.Client().PostJSON("/api/v1/users", registration("a@example.com")).AssertForbidden()
	})

	t.Run("asks no sign-up to accept terms, publishing none", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		silent := registration("alan@example.com")
		delete(silent, "terms_accepted")
		declined := registration("ada@example.com")
		declined["terms_accepted"] = false
		invited := invitee(inviteGrace(t, tc, time.Hour))
		delete(invited, "terms_accepted")

		tc.Client().PostJSON("/api/v1/users", silent).AssertCreated()
		tc.Client().PostJSON("/api/v1/users", declined).AssertCreated()
		tc.Client().PostJSON("/api/v1/users", invited).AssertCreated()

		assertNotAccepted(t, userByEmail(t, tc, "alan@example.com"))
		assertNotAccepted(t, userByEmail(t, tc, "ada@example.com"))
		assertNotAccepted(t, userByEmail(t, tc, "grace@example.com"))
	})

	t.Run("lets an unproven email take every step", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithRoutes(gatedStep))

		tc.ActingAs(tc.CreateUser(factories.Unverified)).PostJSON("/testing/gated", nil).AssertNoContent()
	})

	t.Run("logs unsent mail as a warning with its links on a self-hosted install", func(t *testing.T) {
		t.Parallel()
		logs := &syncBuffer{}
		tc := tests.New(t, tests.WithLogs(logs), tests.WithEnv("DIMMI_LOG__LEVEL=warn", "DIMMI_LOG__DRIVER=json"))

		tc.Client().PostJSON("/api/v1/password-resets", testkit.JSON{"email": tc.Owner.Email}).
			AssertStatus(fhttp.StatusAccepted)
		tc.Drain()

		line := fallbackLine(t, logs)
		assert.Equal(t, "mail logged instead of sent", line["msg"])
		assert.Equal(t, "WARN", line["level"])
		assert.Equal(t, tc.Owner.Email, line["recipients"])
		assert.Equal(t, "Reset your Dimmi password", line["subject"])
		assert.Contains(t, line["links"], "/reset-password#id=prs_")
		assert.NotContains(t, line, "message")
	})

	t.Run("records nothing for a provider sign-up that carries no acceptance", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withProviders())
		tc.Social()
		browser := tc.Client()
		attempt := tc.StartOAuth(browser, "/auth/github/redirect?terms_accepted=false")

		assertLands(t, tc.FinishOAuth(browser, attempt, person("583231", "grace@example.com")), "/")

		assertNotAccepted(t, userByEmail(t, tc, "grace@example.com"))
	})
}

/*
assertNotAccepted checks the account holds no acceptance at all.
*/
func assertNotAccepted(t *testing.T, user models.User) {
	t.Helper()

	assert.Nil(t, user.TermsVersion)
	assert.Nil(t, user.TermsAcceptedAt)
	assert.Nil(t, user.TermsAcceptedIp)
	assert.Nil(t, user.TermsAcceptedUserAgent)
}
