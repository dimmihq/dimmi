package accesspolicy_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appbroadcasting "github.com/dimmihq/dimmi/app/broadcasting"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

func TestClearAccessPolicyCommand(t *testing.T) {
	t.Parallel()

	t.Run("the operator command clears the allowlist and is recorded as the system", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		owner := from(in(tc, tc.Owner), "198.51.100.7")
		ruled(t, tc, map[string]any{
			"dashboard_ip_allowlist": allow1("203.0.113.0/24"), "session_max_age_minutes": 60,
		})
		owner.GetJSON("/api/v1/workspace").AssertForbidden()

		code, out := tc.Command("workspace:clear-access-policy", tc.Workspace.Workspace.Slug, "--only", "ip_allowlist")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "dashboard_ip_allowlist")

		owner.GetJSON("/api/v1/workspace").AssertOK()

		w, err := models.New(tc.App.DB).FindWorkspace(t.Context(), tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		require.NotNil(t, w.Security().SessionMaxAge)

		logs := owner.GetJSON("/api/v1/audit-logs?filter[event]=workspace.access_policy_changed").AssertOK()
		logs.AssertJSONPath("data.0.actor.type", "system")
	})

	t.Run("without a scope it clears every rule, and a second run has nothing to do", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ruled(t, tc, map[string]any{"session_max_age_minutes": 60, "allowed_sign_in_methods": []string{"password"}})

		code, out := tc.Command("workspace:clear-access-policy", tc.Workspace.Workspace.Slug)
		require.Equal(t, 0, code, out)

		w, err := models.New(tc.App.DB).FindWorkspace(t.Context(), tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		assert.Nil(t, w.Security().SessionMaxAge)
		assert.Empty(t, w.Security().AllowedSignInMethods)

		code, out = tc.Command("workspace:clear-access-policy", tc.Workspace.Workspace.Slug)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "Nothing to clear")
	})

	t.Run("an unknown workspace or scope is an error", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, _ := tc.Command("workspace:clear-access-policy", "no-such-workspace")
		assert.NotEqual(t, 0, code)

		code, _ = tc.Command("workspace:clear-access-policy", tc.Workspace.Workspace.Slug, "--only", "nonsense")
		assert.NotEqual(t, 0, code)
	})
}

func TestAccessPolicyRealtime(t *testing.T) {
	t.Parallel()

	t.Run("realtime subscribe is refused outside the list", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		ruled(t, tc, map[string]any{"dashboard_ip_allowlist": allow1("203.0.113.0/24")})

		c := tc.ListenAs(tc.Owner, tc.RealtimeURL())

		reply := c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID))
		require.NotNil(t, reply.Error)
	})

	t.Run("changing the policy closes workspace sockets", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member := join(t, tc, enums.RoleMember)
		c := tc.ListenAs(member, tc.RealtimeURL())
		require.Nil(t, c.Subscribe(appbroadcasting.WorkspaceChannel(tc.Workspace.Workspace.ID)).Error)

		in(tc, tc.Owner).PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 120}).AssertOK()

		assert.NotZero(t, c.Closed())
	})
}
