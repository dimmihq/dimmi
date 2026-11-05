package workspaces_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/tests"
)

/*
labelInitializer gives every workspace a default label in its
settings, the way a real initializer adds a default row, and fails
for a workspace named "Broken".
*/
type labelInitializer struct{}

func (labelInitializer) Name() string {
	return "default_label"
}

func (labelInitializer) Initialize(ctx context.Context, tx *database.Tx, w models.Workspace) error {
	if w.Name == "Broken" {
		return errors.New("label storage unavailable")
	}

	_, err := tx.Exec(ctx, `UPDATE workspaces SET settings = settings || '{"label": "default"}' WHERE id = $1`, w.ID)

	return err
}

/*
withLabels registers the label initializer before the application
boots.
*/
func withLabels() tests.Option {
	return tests.Before(func(a *app.App) { a.WorkspaceInitializers.Register(labelInitializer{}) })
}

func TestWorkspaceInitializers(t *testing.T) {
	t.Parallel()

	t.Run("sets up every new workspace in the creating transaction", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withLabels())

		var label string
		require.NoError(t, tc.Operator().QueryRow(t.Context(),
			"SELECT settings->>'label' FROM workspaces WHERE id = $1", tc.Workspace.Workspace.ID).Scan(&label))
		assert.Equal(t, "default", label)
	})

	t.Run("creates nothing, organization included, when an initializer fails", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withLabels())

		_, err := tc.App.Actions.CreateWorkspace.Handle(t.Context(), workspaces.CreateWorkspaceInput{
			Owner: tc.Owner, Name: "Broken", JoinedVia: appauth.JoinedViaCreated,
		})

		require.ErrorContains(t, err, "initialize default_label")
		tc.Table("workspaces").AssertMissing(testkit.Row{"name": "Broken"})
		tc.Table("organizations").AssertMissing(testkit.Row{"name": "Broken"})
	})
}

func TestWorkspaceRepair(t *testing.T) {
	t.Parallel()

	t.Run("restores missing defaults and a deleted organization, then changes nothing", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, withLabels())
		w := tc.Workspace.Workspace
		exec(t, tc, "UPDATE workspaces SET settings = settings - 'label' WHERE id = $1", w.ID)
		exec(t, tc, "UPDATE organizations SET deleted_at = now(), name = 'Stale' WHERE id = $1", w.OrganizationID)

		code, out := tc.Command("workspace:repair", w.Slug)
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, w.Slug+": defaults checked, organization restored")
		tc.Table("organizations").AssertHas(testkit.Row{"id": w.OrganizationID, "name": w.Name, "deleted_at": nil})
		var label string
		require.NoError(t, tc.Operator().QueryRow(t.Context(),
			"SELECT settings->>'label' FROM workspaces WHERE id = $1", w.ID).Scan(&label))
		assert.Equal(t, "default", label)

		code, out = tc.Command("workspace:repair")
		require.Equal(t, 0, code, out)
		assert.Contains(t, out, w.Slug+": defaults checked, organization in order")
	})

	t.Run("names an unknown workspace and a failing one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		code, out := tc.Command("workspace:repair", "nowhere")
		assert.NotEqual(t, 0, code)
		assert.Contains(t, out, "Workspace not found")

		tc.BreakTable("organizations")
		_, err := tc.App.Actions.RepairWorkspaces.Handle(t.Context(), "")
		require.ErrorContains(t, err, "repair workspace "+tc.Workspace.Workspace.Slug)
	})
}
