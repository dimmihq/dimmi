package inbox_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
defaultInbox returns the inbox the workspace's settings name as its
default.
*/
func defaultInbox(t *testing.T, tc *tests.TestCase, workspace ids.UUID) models.Inbox {
	t.Helper()

	inbox, err := query(t, tc, workspace, func(ctx context.Context, q *models.Queries) (models.Inbox, error) {
		return q.FindDefaultInbox(ctx, workspace)
	})
	require.NoError(t, err)

	return inbox
}

/*
repair runs the workspace repair command on the test's workspace.
*/
func repair(t *testing.T, tc *tests.TestCase) {
	t.Helper()

	code, out := tc.Command("workspace:repair", tc.Workspace.Workspace.Slug)
	require.Equal(t, 0, code, out)
}

/*
exec runs a statement as the tables' owner.
*/
func exec(t *testing.T, tc *tests.TestCase, sql string, args ...any) {
	t.Helper()

	_, err := tc.App.DB.Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

func TestDefaultInbox(t *testing.T) {
	t.Parallel()

	t.Run("gives every new workspace a support inbox with an api connection", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID

		inbox := defaultInbox(t, tc, workspace)

		assert.Equal(t, "Support", inbox.Name)
		tc.Table("inboxes").AssertCount(1, testkit.Row{"workspace_id": workspace})
		tc.Table("channel_connections").
			AssertCount(1, testkit.Row{"workspace_id": workspace, "type": "api"}).
			AssertHas(testkit.Row{"inbox_id": inbox.ID, "type": "api", "name": "API", "status": "active"})
	})

	t.Run("changes nothing when the workspace is repaired", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		before := defaultInbox(t, tc, workspace)

		repair(t, tc)
		repair(t, tc)

		assert.Equal(t, before, defaultInbox(t, tc, workspace))
		tc.Table("inboxes").AssertCount(1, testkit.Row{"workspace_id": workspace})
		tc.Table("channel_connections").AssertCount(1, testkit.Row{"workspace_id": workspace, "type": "api"})
	})

	t.Run("recreates a deleted default inbox on repair and keeps the other settings", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		deleted := defaultInbox(t, tc, workspace)
		exec(t, tc, "UPDATE inboxes SET deleted_at = now() WHERE id = $1", deleted.ID)
		exec(t, tc, `UPDATE workspaces SET settings = settings || '{"theme": "dark"}' WHERE id = $1`, workspace)

		repair(t, tc)

		inbox := defaultInbox(t, tc, workspace)
		var settings json.RawMessage
		require.NoError(t, tc.App.DB.QueryRow(t.Context(),
			"SELECT settings FROM workspaces WHERE id = $1", workspace).Scan(&settings))

		assert.NotEqual(t, deleted.ID, inbox.ID)
		assert.Equal(t, "Support", inbox.Name)
		assert.JSONEq(t, `{"theme": "dark", "default_inbox_id": "`+inbox.ID.String()+`"}`, string(settings))
		tc.Table("channel_connections").AssertCount(1, testkit.Row{"inbox_id": inbox.ID, "type": "api"})
	})

	t.Run("connects the api to every live inbox missing one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		th := seedThread(t, tc, workspace)
		exec(t, tc, "UPDATE channel_connections SET deleted_at = now() WHERE type = 'api'")

		repair(t, tc)

		tc.Table("channel_connections").
			AssertCount(1, testkit.Row{"inbox_id": th.inbox.ID, "type": "api", "deleted_at": nil}).
			AssertCount(1, testkit.Row{"inbox_id": defaultInbox(t, tc, workspace).ID, "type": "api", "deleted_at": nil})
	})

	t.Run("names the step that failed and leaves the workspace as it was", func(t *testing.T) {
		t.Parallel()

		for name, tc := range map[string]struct {
			table, kind, want string
			deleteDefault     bool
		}{
			"inbox":      {table: "inboxes", kind: "INSERT", want: "create inbox", deleteDefault: true},
			"settings":   {table: "workspaces", kind: "UPDATE", want: "writes to workspaces fail", deleteDefault: true},
			"connection": {table: "channel_connections", kind: "INSERT", want: "connect the api to inbox"},
		} {
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				app := tests.New(t)
				exec(t, app, "UPDATE channel_connections SET deleted_at = now()")
				if tc.deleteDefault {
					exec(t, app, "UPDATE inboxes SET deleted_at = now()")
				}
				app.FailWrites(tc.table, tc.kind)

				_, err := app.App.Actions.RepairWorkspaces.Handle(t.Context(), app.Workspace.Workspace.Slug)

				require.ErrorContains(t, err, "initialize default_inbox")
				require.ErrorContains(t, err, tc.want)
			})
		}
	})

	t.Run("stops when the inboxes cannot be read", func(t *testing.T) {
		t.Parallel()

		for table, want := range map[string]string{
			"inboxes":             "ensure default inbox",
			"channel_connections": "list inboxes without an api connection",
		} {
			t.Run(table, func(t *testing.T) {
				t.Parallel()
				tc := tests.New(t)
				tc.BreakTable(table)

				_, err := tc.App.Actions.RepairWorkspaces.Handle(t.Context(), tc.Workspace.Workspace.Slug)

				require.ErrorContains(t, err, want)
			})
		}
	})
}
