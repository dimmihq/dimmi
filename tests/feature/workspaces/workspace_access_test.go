package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestWorkspaceAccess(t *testing.T) {
	t.Parallel()

	t.Run("works in the only workspace when a browser names none", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().GetJSON("/api/v1/workspace").AssertOK().
			AssertJSONPath("data.id", workspaceID(tc)).
			AssertJSONPath("data.object", "workspace")
	})

	t.Run("asks a user of several workspaces to name one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		tc.ActingAsOwner().GetJSON("/api/v1/workspace").
			AssertStatus(400).
			AssertJSONPath("code", "workspace_required")
	})

	t.Run("does not find a foreign, unknown, malformed or deleted workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		stranger := tc.CreateUser()
		other, err := factories.Workspace(t.Context(), tc.App, stranger)
		require.NoError(t, err)

		owner := tc.ActingAsOwner()
		for _, named := range []string{
			models.PublicID(models.PrefixWorkspace, other.Workspace.ID), tc.Workspace.Workspace.Slug, "ws_nope",
		} {
			owner.WithHeader("Dimmi-Workspace", named).GetJSON("/api/v1/members").
				AssertNotFound().
				AssertJSONPath("code", "workspace_not_found")
		}

		exec(t, tc, "UPDATE workspaces SET deleted_at = now() WHERE id = $1", tc.Workspace.Workspace.ID)
		owner.WithHeader("Dimmi-Workspace", workspaceID(tc)).GetJSON("/api/v1/workspace").AssertNotFound()
	})

	t.Run("refuses a member whose membership was deactivated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, member))

		in(tc, user).GetJSON("/api/v1/workspace").
			AssertForbidden().
			AssertJSONPath("code", "membership_deactivated")
	})

	t.Run("lets a suspended workspace be read but not changed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		exec(t, tc, "UPDATE workspaces SET suspended_at = now() WHERE id = $1", tc.Workspace.Workspace.ID)
		owner := in(tc, tc.Owner)

		owner.GetJSON("/api/v1/workspace").AssertOK()
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).
			AssertForbidden().
			AssertJSONPath("code", "workspace_suspended")
		tc.Table("workspaces").AssertMissing(testkit.Row{"name": "Renamed"})
	})

	t.Run("records when a member opened the workspace, at most every five minutes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		opened := tc.Clock.Now()

		owner.GetJSON("/api/v1/workspace").AssertOK()
		tc.Clock.Travel(time.Minute)
		owner.GetJSON("/api/v1/workspace").AssertOK()
		assert.Equal(t, opened.UTC(), lastAccess(t, tc).UTC())

		tc.Clock.Travel(5 * time.Minute)
		owner.GetJSON("/api/v1/workspace").AssertOK()
		assert.Equal(t, tc.Clock.Now().UTC(), lastAccess(t, tc).UTC())
	})

	t.Run("keeps a token in its own workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:read")

		token.GetJSON("/api/v1/members").AssertOK().AssertJSONPath("data.0.id", memberID(tc.Workspace.Owner))
		token.WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, other.Workspace.ID)).
			GetJSON("/api/v1/members").
			AssertForbidden().
			AssertJSONPath("code", "workspace_mismatch")
		token.WithHeader("Dimmi-Workspace", workspaceID(tc)).GetJSON("/api/v1/members").AssertOK()
	})

	t.Run("asks a token for the ability a route needs", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "conversations:read").GetJSON("/api/v1/members").
			AssertForbidden().
			AssertJSONPath("code", "missing_ability")
	})
}

/*
lastAccess returns when the test case's owner last opened the workspace.
*/
func lastAccess(t *testing.T, tc *tests.TestCase) time.Time {
	t.Helper()

	rows, err := tc.App.Actions.ListMemberships.Handle(t.Context(), membershipsOf(tc.Owner))
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	require.True(t, rows[0].Accessed)

	return rows[0].LastAccessedAt
}
