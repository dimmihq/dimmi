package accesspolicy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestAccessPolicyEndpoint(t *testing.T) {
	t.Parallel()

	t.Run("a member with settings read may read but not change the rules", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin := join(t, tc, enums.RoleAdmin)
		member := join(t, tc, enums.RoleMember)

		in(tc, admin).GetJSON(policyPath).AssertOK()
		in(tc, member).GetJSON(policyPath).AssertForbidden()
		in(tc, member).PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 60}).AssertForbidden()
		in(tc, admin).PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 60}).AssertOK()
	})

	t.Run("a token may read but never change the rules", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		token := tc.ActingAsToken(tc.Workspace.Owner, "settings:read", "settings:write")

		token.GetJSON(policyPath).AssertOK()
		token.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 60}).AssertForbidden().
			AssertJSONPath("code", "first_party_only")
	})

	t.Run("a change needs a recent confirmation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		tc.Clock.Advance(24 * time.Hour)

		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 60}).AssertStatus(423).
			AssertJSONPath("code", "password_confirmation_required")
	})

	t.Run("changes land in the audit log, tell the other owners and name no values there", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, proxyEnv())
		fake := tc.Mail()
		other := join(t, tc, enums.RoleOwner)
		owner := from(in(tc, tc.Owner), "203.0.113.4")

		owner.PatchJSON(policyPath, testkit.JSON{
			"dashboard_ip_allowlist": allow1("203.0.113.0/24"), "session_max_age_minutes": 120,
		}).AssertOK()
		tc.Drain()

		logs := owner.GetJSON("/api/v1/audit-logs?filter[event]=workspace.access_policy_changed").AssertOK()
		logs.AssertJSONPath("data.0.event", "workspace.access_policy_changed")
		assert.Contains(t, string(logs.Body), "203.0.113.0/24")

		sent, messages := tests.Sent[appmail.AccessPolicyChangedMail](tc, fake)
		require.Len(t, sent, 1)
		assert.Equal(t, []string{other.Email}, tests.SentTo(messages[0]))
	})

	t.Run("saving what is already set changes nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 120}).AssertOK()
		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 120}).AssertOK()
		owner.PatchJSON(policyPath, testkit.JSON{}).AssertOK()

		logs := owner.GetJSON("/api/v1/audit-logs?filter[event]=workspace.access_policy_changed").AssertOK()
		logs.AssertJSONMissing("data.1")
	})

	t.Run("every other setting of the workspace is kept", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		_, err := factories.WithTwoFactor(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		_, err = tc.App.DB.Exec(t.Context(),
			`UPDATE workspaces SET settings = '{"keep": 1, "security": {"require_two_factor": true}}' WHERE id = $1`,
			tc.Workspace.Workspace.ID)
		require.NoError(t, err)

		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 120}).AssertOK().
			AssertJSONPath("data.require_two_factor", true)

		w, err := models.New(tc.App.DB).FindWorkspace(t.Context(), tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		assert.JSONEq(t,
			`{"keep": 1, "security": {"require_two_factor": true, "session_max_age_minutes": 120}}`,
			string(w.Settings))
	})

	t.Run("a broken database answers with a server error and changes nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		owner.GetJSON("/api/v1/workspace").AssertOK()
		tc.FailWrites("audit_logs", "INSERT")

		owner.PatchJSON(policyPath, testkit.JSON{"session_max_age_minutes": 120}).AssertStatus(500)

		w, err := models.New(tc.App.DB).FindWorkspace(t.Context(), tc.Workspace.Workspace.ID)
		require.NoError(t, err)
		assert.Nil(t, w.Security().SessionMaxAge)
	})
}
