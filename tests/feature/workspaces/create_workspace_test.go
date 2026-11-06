package workspaces_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestCreateWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("creates the organization first and the owner with the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		created, err := tc.App.Actions.CreateWorkspace.Handle(t.Context(), workspaces.CreateWorkspaceInput{
			Owner: tc.Owner, Name: "  Ana\u200blytical  ", JoinedVia: appauth.JoinedViaCreated,
		})
		require.NoError(t, err)

		assert.Equal(t, "Analytical", created.Workspace.Name)
		assert.Equal(t, tc.Owner.ID, created.Owner.UserID)
		tc.Table("organizations").AssertHas(testkit.Row{"id": created.Workspace.OrganizationID, "name": "Analytical"})
	})

	t.Run("falls back to a random suffix once every numbered slug is taken", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		var slug string
		for range 100 {
			created, err := tc.App.Actions.CreateWorkspace.Handle(t.Context(), workspaces.CreateWorkspaceInput{
				Owner: tc.Owner, Name: "Busy", JoinedVia: appauth.JoinedViaCreated,
			})
			require.NoError(t, err)

			slug = created.Workspace.Slug
		}

		assert.Regexp(t, regexp.MustCompile(`^busy-[a-z0-9]{4}$`), slug)
		tc.Table("workspaces").AssertHas(testkit.Row{"slug": "busy-99"})
	})

	t.Run("creates nothing when its owner cannot be added", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		tc.BreakTable("workspace_members")

		_, err := tc.App.Actions.CreateWorkspace.Handle(t.Context(), workspaces.CreateWorkspaceInput{
			Owner: tc.Owner, Name: "Orphan", JoinedVia: appauth.JoinedViaCreated,
		})

		require.Error(t, err)
		tc.Table("organizations").AssertMissing(testkit.Row{"name": "Orphan"})
	})
}

func TestWorkspacesStore(t *testing.T) {
	t.Parallel()

	t.Run("lets someone without a workspace create one they own", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		users, err := factories.User(t.Context(), tc.App)
		require.NoError(t, err)
		user, err := users.CreateOne(t.Context())
		require.NoError(t, err)
		client := tc.ActingAs(user)

		created := client.PostJSON("/api/v1/workspaces", testkit.JSON{"name": "  Acme  "}).AssertCreated()

		created.AssertJSONPath("data.name", "Acme")
		workspace := created.String("data.id")
		client.GetJSON("/api/v1/me").AssertOK().AssertJSONPath("data.workspaces.0.id", workspace)
		client.WithHeader("Dimmi-Workspace", workspace).GetJSON("/api/v1/members/me").
			AssertOK().AssertJSONPath("data.role", "owner")
	})

	t.Run("refuses a missing or too long name and a guest", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAs(tc.Owner).PostJSON("/api/v1/workspaces", testkit.JSON{"name": ""}).AssertStatus(422)
		tc.ActingAs(tc.Owner).PostJSON("/api/v1/workspaces", testkit.JSON{"name": strings.Repeat("a", 81)}).
			AssertStatus(422)
		tc.Client().PostJSON("/api/v1/workspaces", testkit.JSON{"name": "Acme"}).AssertStatus(401)
	})
}
