package workspaces_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

/*
graceAhead puts the purge of the test case's trashed workspace a month
past the database's clock, which the command line's clock follows.
*/
func graceAhead(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	exec(t, tc, `UPDATE workspaces SET purge_after = now() + interval '30 days' WHERE id = $1`,
		tc.Workspace.Workspace.ID)
}

func TestWorkspaceListCommand(t *testing.T) {
	t.Parallel()

	t.Run("lists live workspaces as a table and every workspace as json", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		slug := tc.Workspace.Workspace.Slug

		code, out := tc.Command("workspace:list")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, slug)
		assert.Contains(t, out, "active")

		trash(tc).AssertOK()

		code, out = tc.Command("workspace:list")
		require.Equal(t, 0, code, out)
		assert.NotContains(t, out, slug)

		code, out = tc.Command("workspace:list", "--with-deleted", "--json")
		require.Equal(t, 0, code, out)

		var listed []map[string]any
		require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &listed))
		require.Len(t, listed, 1)
		assert.Equal(t, slug, listed[0]["slug"])
		assert.Equal(t, "deleted", listed[0]["status"])
		assert.Equal(t, workspaceID(tc), listed[0]["id"])
	})
}

func TestWorkspaceSuspendCommands(t *testing.T) {
	t.Parallel()

	t.Run("suspends with a reason, blocks changes, and unsuspends", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace

		code, out := tc.Command("workspace:suspend", ws.Slug, "--reason", "unpaid invoice")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "unpaid invoice")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID, "suspended_reason": "unpaid invoice"})
		tc.Table("audit_logs").AssertHas(testkit.Row{
			"workspace_id": ws.ID, "event": "workspace.suspended", "actor_type": "system", "via_kind": "cli",
		})

		in(tc, tc.Owner).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).
			AssertForbidden().AssertJSONPath("code", "workspace_suspended")
		in(tc, tc.Owner).GetJSON("/api/v1/workspace").AssertOK()

		code, out = tc.Command("workspace:list", "--json")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, `"status":"suspended"`)

		code, out = tc.Command("workspace:unsuspend", ws.Slug)
		require.Equal(t, 0, code, out)
		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID, "suspended_at": nil, "suspended_reason": nil})
		tc.Table("audit_logs").AssertCount(1, testkit.Row{"workspace_id": ws.ID, "event": "workspace.unsuspended"})
		in(tc, tc.Owner).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Renamed"}).AssertOK()

		code, out = tc.Command("workspace:unsuspend", ws.Slug)
		require.Equal(t, 0, code, out)
		tc.Table("audit_logs").AssertCount(1, testkit.Row{"workspace_id": ws.ID, "event": "workspace.unsuspended"})
	})

	t.Run("refuses a suspension without a reason or of an unknown workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, out := tc.Command("workspace:suspend", tc.Workspace.Workspace.Slug)
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "--reason")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID, "suspended_at": nil})

		code, out = tc.Command("workspace:suspend", "nowhere", "--reason", "abuse")
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "Workspace not found")

		code, _ = tc.Command("workspace:unsuspend", "nowhere")
		assert.Equal(t, 1, code)
	})
}

func TestWorkspaceRestoreCommand(t *testing.T) {
	t.Parallel()

	t.Run("takes a workspace out of the trash and tells its owners", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		fake := tc.Mail()
		ws := tc.Workspace.Workspace
		trash(tc).AssertOK()
		graceAhead(t, tc)

		code, out := tc.Command("workspace:restore", ws.Slug)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "restored")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID, "deleted_at": nil, "purge_after": nil})
		tc.Table("audit_logs").AssertHas(testkit.Row{
			"workspace_id": ws.ID, "event": "workspace.restored", "actor_type": "system",
		})
		tc.Drain()
		restored, _ := tests.LastSent[appmail.WorkspaceRestoredMail](tc, fake)
		assert.Equal(t, "System", restored.RestoredBy)
		in(tc, tc.Owner).GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("refuses a live, unknown or purging workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace

		code, out := tc.Command("workspace:restore", ws.Slug)
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "not in the trash")

		code, _ = tc.Command("workspace:restore", "nowhere")
		assert.Equal(t, 1, code)

		trash(tc).AssertOK()
		exec(t, tc, `UPDATE workspaces SET purge_after = now() - interval '1 minute' WHERE id = $1`, ws.ID)

		code, out = tc.Command("workspace:restore", ws.Slug)
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "can no longer be restored")
		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": ws.ID, "deleted_at": nil})
	})
}

func TestWorkspacePurgeCommand(t *testing.T) {
	t.Parallel()

	t.Run("deletes a workspace in the trash for good", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace
		disk, path := storeFile(tc, ws.ID)
		trash(tc).AssertOK()
		graceAhead(t, tc)

		code, out := tc.Command("workspace:purge", ws.Slug, "--force")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, "deleted for good")
		tc.Table("workspaces").AssertCount(0, testkit.Row{"id": ws.ID})
		tc.Table("workspace_members").AssertCount(0, testkit.Row{"workspace_id": ws.ID})

		exists, err := disk.Exists(t.Context(), path)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("refuses without force, a live workspace and an unknown one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		ws := tc.Workspace.Workspace

		code, out := tc.Command("workspace:purge", ws.Slug, "--force")
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "not in the trash")

		trash(tc).AssertOK()

		code, out = tc.Command("workspace:purge", ws.Slug)
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "--force")

		code, _ = tc.Command("workspace:purge", "nowhere", "--force")
		assert.Equal(t, 1, code)
		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID})
	})

	t.Run("leaves a workspace the retention guard holds in the trash", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = legalHold{} }))
		ws := tc.Workspace.Workspace
		trash(tc).AssertOK()

		code, out := tc.Command("workspace:purge", ws.Slug, "--force")
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "retention guard")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID})
	})

	t.Run("reports a purge that fails", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, tests.Before(func(a *app.App) { a.Retention = failingGuard{} }))
		ws := tc.Workspace.Workspace
		trash(tc).AssertOK()

		code, out := tc.Command("workspace:purge", ws.Slug, "--force")
		assert.Equal(t, 1, code)
		assert.Contains(t, out, "retention service down")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": ws.ID})
	})
}
