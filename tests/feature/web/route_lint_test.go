package web_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/tests"
)

func TestAPIRoutes(t *testing.T) {
	t.Parallel()

	t.Run("names every api route after its nouns and uses no verbs in paths", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		problems := routing.Lint(tc.App.Router.Routes(), routing.Rules{
			Prefixes:     []string{"/api/v1"},
			VerbDenylist: []string{"login", "logout", "verify", "resend", "enable", "disable", "confirm", "reset"},
			PutAllowlist: []string{"/api/v1/user/avatar"},
			Exceptions: []string{
				"/api/v1/conversation-views", "/api/v1/redaction-rules",
				"/api/v1/contacts/{contact}/exports/{export}/download",
			},
		})

		assert.Empty(t, problems, strings.Join(problems, "\n"))
	})

	t.Run("keeps none of the old account routes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		params := regexp.MustCompile(`\{[^}]+\}`)

		for _, route := range tc.App.Router.Routes() {
			key := route.Method() + " " + params.ReplaceAllString(route.Path(), "{}")
			assert.NotContains(t, oldAccountRoutes(), key)
			assert.False(t, strings.HasPrefix(route.Path(), "/api/v1/user/two-factor"), key)
		}
	})
}

/*
oldAccountRoutes are the account routes the API replaced, which must
never come back.
*/
func oldAccountRoutes() []string {
	routes := []string{
		"POST /auth/register", "POST /auth/login", "POST /auth/two-factor-challenge", "POST /auth/logout",
		"POST /auth/magic-link", "POST /auth/magic-link/consume", "POST /auth/forgot-password",
		"POST /auth/reset-password", "POST /auth/email/verification-notification", "POST /auth/email/verify",
		"GET /auth/confirmed-password-status", "POST /auth/confirm-password", "POST /auth/confirm-code",
		"POST /user/email", "POST /user/email/confirm", "POST /user/avatar", "GET /user/sessions",
		"DELETE /user/sessions/{}", "DELETE /user/sessions", "POST /user/two-factor", "POST /user/two-factor/confirm",
		"POST /user/two-factor/recovery-codes", "DELETE /user/two-factor", "GET /user/workspaces",
		"POST /invitations/accept", "POST /workspace/leave", "POST /workspaces/{}/restore",
		"POST /members/{}/deactivate", "POST /members/{}/reactivate", "POST /members/{}/transfer-ownership",
		"POST /invitations/{}/resend", "PATCH /members/me/availability",
	}

	out := make([]string, len(routes))
	for i, route := range routes {
		method, path, _ := strings.Cut(route, " ")
		out[i] = method + " /api/v1" + path
	}

	return out
}
