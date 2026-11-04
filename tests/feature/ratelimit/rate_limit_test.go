package ratelimit_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/support/entitlements"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const members = "/api/v1/members"

/*
limits starts the application with the API limits given as
environment entries, such as "TOKEN_PER_MINUTE=6".
*/
func limits(t *testing.T, entries ...string) *tests.TestCase {
	t.Helper()

	env := make([]string, 0, len(entries))
	for _, entry := range entries {
		env = append(env, "DIMMI_API__LIMITS__"+entry)
	}

	return tests.New(t, tests.WithEnv(env...))
}

func TestAPIRateLimits(t *testing.T) {
	t.Parallel()

	t.Run("limit headers come on every response", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "members:read").GetJSON(members).AssertOK().
			AssertHeader("X-RateLimit-Limit", "600").
			AssertHeader("X-RateLimit-Scope", "token")
		tc.ActingAsOwner().GetJSON("/api/v1/me").AssertOK().
			AssertHeader("X-RateLimit-Limit", "1200").
			AssertHeader("X-RateLimit-Scope", "user")
	})

	t.Run("a burst over ten seconds is refused while the minute limit remains", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "TOKEN_PER_MINUTE=12")
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:read")

		token.GetJSON(members).AssertOK().AssertHeader("X-RateLimit-Limit", "12")
		token.GetJSON(members).AssertOK()

		refused := token.GetJSON(members).AssertStatus(429).
			AssertJSONPath("code", "too_many_requests").
			AssertHeader("X-RateLimit-Scope", "burst")
		assert.NotEmpty(t, refused.Header.Get("Retry-After"))
		assert.NotEmpty(t, refused.Header.Get("X-RateLimit-Reset"))
	})

	t.Run("two tokens of one workspace share the workspace limit", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "WORKSPACE_PER_MINUTE=2")
		first := tc.ActingAsToken(tc.Workspace.Owner, "members:read")
		second := tc.ActingAsToken(tc.Workspace.Owner, "members:read")

		first.GetJSON(members).AssertOK()
		second.GetJSON(members).AssertOK()

		first.GetJSON(members).AssertStatus(429).AssertHeader("X-RateLimit-Scope", "workspace")
	})

	t.Run("a script hitting the workspace limit does not slow the dashboard", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "WORKSPACE_PER_MINUTE=1")
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:read")
		token.GetJSON(members).AssertOK()
		token.GetJSON(members).AssertStatus(429)

		tc.ActingAsOwner().GetJSON(members).AssertOK().AssertHeader("X-RateLimit-Scope", "user")
	})

	t.Run("a browser session has its own limit", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "USER_PER_MINUTE=2")
		owner := tc.ActingAsOwner()

		owner.GetJSON("/api/v1/me").AssertOK()
		owner.GetJSON("/api/v1/me").AssertOK()
		owner.GetJSON("/api/v1/me").AssertStatus(429).AssertHeader("X-RateLimit-Scope", "user")
	})

	t.Run("zero means no limit", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "TOKEN_PER_MINUTE=0", "WORKSPACE_PER_MINUTE=0")
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:read")

		for range 5 {
			token.GetJSON(members).AssertOK().AssertHeader("X-RateLimit-Limit", "")
		}
	})

	t.Run("an override raises the limit", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.WithEnv("DIMMI_API__LIMITS__TOKEN_PER_MINUTE=6"), tests.Before(func(a *app.App) {
			catalog := entitlements.SelfHost(0)
			catalog[entitlements.KeyAPIRateLimit] = 600
			a.Entitlements = entitlements.Driver{Catalog: catalog}
		}))
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:read")

		for range 3 {
			token.GetJSON(members).AssertOK().AssertHeader("X-RateLimit-Limit", "600")
		}
	})

	t.Run("refused by access policy costs no budget", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "TOKEN_PER_MINUTE=6")
		user := tc.CreateUser()
		member, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleAdmin,
		})
		require.NoError(t, err)
		_, err = tc.App.DB.Exec(t.Context(),
			`UPDATE workspaces SET settings = jsonb_set(settings, '{security}', '{"require_two_factor": true}')
			 WHERE id = $1`, tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		token := tc.ActingAsToken(member, "members:read")

		for range 3 {
			token.GetJSON(members).AssertForbidden().AssertHeader("X-RateLimit-Limit", "")
		}
	})

	t.Run("repeated bad tokens from one address are slowed", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "BAD_CREDENTIAL_PER_MINUTE=2")
		bad := tc.Client().WithHeader("Authorization", "Bearer dmi_live_not-a-real-token")

		bad.GetJSON(members).AssertUnauthorized()
		bad.GetJSON(members).AssertUnauthorized()
		bad.GetJSON(members).AssertStatus(429).AssertJSONPath("code", "too_many_requests")

		tc.Client().GetJSON("/api/v1/me").AssertUnauthorized()
	})

	t.Run("the workspace says its limits, from the catalog or the install's numbers", func(t *testing.T) {
		t.Parallel()
		tc := limits(t, "TOKEN_PER_MINUTE=0")

		tc.ActingAsOwner().GetJSON("/api/v1/workspace/entitlements").AssertOK().
			AssertJSONPath("data.object", "entitlements").
			AssertJSONPath("data.api.token_per_minute", 0).
			AssertJSONPath("data.api.user_per_minute", 1200).
			AssertJSONPath("data.api.workspace_per_minute", 6000)
		tc.Client().GetJSON("/api/v1/workspace/entitlements").AssertUnauthorized()
	})
}
