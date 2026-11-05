package workspaces_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestCustomerFacingName(t *testing.T) {
	t.Parallel()

	t.Run("customer facing name falls back to the workspace name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.GetJSON("/api/v1/workspace").AssertOK().AssertJSONPath("data.customer_facing_name", nil)
		assert.Equal(t, tc.Workspace.Workspace.Name, workspace(t, tc).CustomerFacingName())

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"customer_facing_name": "  Acme Support  "}).AssertOK().
			AssertJSONPath("data.customer_facing_name", "Acme Support")
		assert.Equal(t, "Acme Support", workspace(t, tc).CustomerFacingName())

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).AssertOK().
			AssertJSONPath("data.customer_facing_name", "Acme Support")

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"customer_facing_name": nil}).AssertOK().
			AssertJSONPath("data.customer_facing_name", nil)
		assert.Equal(t, "Renamed", workspace(t, tc).CustomerFacingName())
	})

	t.Run("a change is announced and audited", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().PatchJSON("/api/v1/workspace", testkit.JSON{"customer_facing_name": "Acme Help"}).AssertOK()
		tc.ActingAsOwner().PatchJSON("/api/v1/workspace", testkit.JSON{"customer_facing_name": "Acme Help"}).AssertOK()

		tc.Drain()
		tc.Table("audit_logs").AssertCount(1, testkit.Row{
			"workspace_id": tc.Workspace.Workspace.ID, "event": "workspace.updated",
		})
	})

	t.Run("an empty or long name is refused", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := tc.ActingAsOwner()

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"customer_facing_name": " \u200b "}).
			AssertValidationError("customer_facing_name")
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"customer_facing_name": strings.Repeat("a", 81)}).
			AssertValidationError("customer_facing_name")
	})

	t.Run("a member without settings write cannot change it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		_, err := factories.Member(t.Context(), tc.App, factories.Membership{
			Workspace: tc.Workspace.Workspace, User: user, Role: enums.RoleMember,
		})
		require.NoError(t, err)

		tc.ActingAs(user).PatchJSON("/api/v1/workspace", testkit.JSON{"customer_facing_name": "Nope"}).
			AssertForbidden()
	})
}

/*
workspace reads the test's workspace as it is now stored.
*/
func workspace(t *testing.T, tc *tests.TestCase) models.Workspace {
	t.Helper()

	w, err := models.New(tc.App.DB).FindWorkspace(t.Context(), tc.Workspace.Workspace.ID)
	require.NoError(t, err)

	return w
}
