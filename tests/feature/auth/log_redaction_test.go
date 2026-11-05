package auth_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestLogRedaction(t *testing.T) {
	t.Parallel()

	t.Run("never writes a token or a session cookie to the logs", func(t *testing.T) {
		t.Parallel()
		logs := &syncBuffer{}
		tc := tests.New(t, tests.WithLogs(logs), tests.WithEnv("DIMMI_LOG__LEVEL=debug", "DIMMI_LOG__DRIVER=json"))
		workspace := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)

		signIn := tc.Client().PostJSON("/api/v1/sessions", testkit.JSON{
			"grant": "password", "email": tc.Owner.Email, "password": factories.Password,
		}).AssertCreated()
		cookie := signIn.Cookie(tc.App.Session.CookieName())
		browser := tc.Client().WithCookie(cookie).WithHeader("Dimmi-Workspace", workspace)

		plain := browser.PostJSON("/api/v1/tokens", testkit.JSON{"name": "CI", "abilities": []string{"*"}}).
			AssertCreated().
			String("data.plain_text_token")
		require.NotEmpty(t, plain)

		tc.Client().WithHeader("Authorization", "Bearer "+plain).GetJSON("/api/v1/me").AssertOK()
		tc.Client().WithHeader("Authorization", "Bearer "+plain+"x").GetJSON("/api/v1/me").AssertUnauthorized()
		browser.Delete("/api/v1/sessions/current").AssertNoContent()
		tc.Drain()

		written := logs.String()
		assert.NotEmpty(t, written)
		assert.NotContains(t, written, plain)
		assert.NotContains(t, written, plain[len(plain)-12:])
		assert.NotContains(t, written, cookie.Value)
	})
}
