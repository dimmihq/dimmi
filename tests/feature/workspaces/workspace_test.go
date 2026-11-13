package workspaces_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestWorkspaceShow(t *testing.T) {
	t.Parallel()

	t.Run("shows any member the workspace without its organization", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)

		in(tc, user).GetJSON("/api/v1/workspace").AssertOK().
			AssertJSONPath("data.name", tc.Workspace.Workspace.Name).
			AssertJSONPath("data.slug", tc.Workspace.Workspace.Slug).
			AssertJSONPath("data.timezone", "UTC").
			AssertJSONPath("data.test_mode", false).
			AssertJSONMissing("data.organization_id")
	})
}

func TestWorkspaceUpdate(t *testing.T) {
	t.Parallel()

	t.Run("renames the workspace and its organization, announcing what changed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		updated := record[appevents.WorkspaceUpdated](tc)

		in(tc, tc.Owner).PatchJSON("/api/v1/workspace", testkit.JSON{
			"name": "  Acme\u200b Support ", "timezone": "Europe/Lisbon", "locale": "en",
		}).AssertOK().
			AssertJSONPath("data.name", "Acme Support").
			AssertJSONPath("data.timezone", "Europe/Lisbon")

		tc.Table("organizations").AssertHas(testkit.Row{
			"id": tc.Workspace.Workspace.OrganizationID, "name": "Acme Support",
		})
		require.Len(t, updated.all(), 1)
		assert.Equal(t, []string{"name", "timezone"}, updated.all()[0].Changes)
	})

	t.Run("changes nothing and announces nothing for the current values", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		updated := record[appevents.WorkspaceUpdated](tc)
		w := tc.Workspace.Workspace

		in(tc, tc.Owner).PatchJSON("/api/v1/workspace", testkit.JSON{
			"name": w.Name, "slug": w.Slug, "timezone": w.Timezone, "test_mode": false,
		}).AssertOK()
		assert.Empty(t, updated.all())
	})

	t.Run("keeps the old slug for the workspace for thirty days", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		old := tc.Workspace.Workspace.Slug
		owner := in(tc, tc.Owner)
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": "acme-help"}).AssertOK().
			AssertJSONPath("data.slug", "acme-help")
		tc.Table("workspaces").AssertHas(testkit.Row{"id": tc.Workspace.Workspace.ID, "slug": "acme-help"})

		taken := other.Workspace.Slug
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": taken}).
			AssertJSONPath("code", "slug_taken").
			AssertValidationError("slug")

		created, err := tc.App.Actions.CreateWorkspace.Handle(t.Context(), workspaces.CreateWorkspaceInput{
			Owner: tc.Owner, Name: old, JoinedVia: appauth.JoinedViaCreated,
		})
		require.NoError(t, err)
		assert.NotEqual(t, old, created.Workspace.Slug)

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": old}).AssertOK().AssertJSONPath("data.slug", old)
	})

	t.Run("refuses a slug another workspace gave up, until its hold runs out", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		other, err := factories.Workspace(t.Context(), tc.App, user)
		require.NoError(t, err)
		given := other.Workspace.Slug

		tc.ActingAs(user).WithHeader("Dimmi-Workspace", publicWorkspace(other)).
			PatchJSON("/api/v1/workspace", testkit.JSON{"slug": "elsewhere"}).AssertOK()

		owner := in(tc, tc.Owner)
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": given}).AssertJSONPath("code", "slug_reserved")

		exec(t, tc, `UPDATE workspaces
			SET settings = jsonb_set(settings, '{previous_slugs,0,until}', '"2026-11-01T00:00:00Z"')
			WHERE id = $1`, other.Workspace.ID)
		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": given}).AssertOK()
	})

	t.Run("refuses reserved, malformed and blank values", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": "admin"}).
			AssertJSONPath("code", "slug_reserved")
		for _, page := range dashboardPages(t) {
			owner.PatchJSON("/api/v1/workspace", testkit.JSON{"slug": page}).
				AssertJSONPath("code", "slug_reserved")
		}
		for field, value := range map[string]any{
			"slug": "Not A Slug", "timezone": "America/Nowhere", "locale": "xx", "name": "\u200b ",
		} {
			owner.PatchJSON("/api/v1/workspace", testkit.JSON{field: value}).AssertValidationError(field)
		}

		owner.PatchJSON("/api/v1/workspace", testkit.JSON{"timezone": "Local"}).AssertValidationError("timezone")
	})

	t.Run("never changes test mode after creation", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		in(tc, tc.Owner).PatchJSON("/api/v1/workspace", testkit.JSON{"test_mode": true}).
			AssertJSONPath("code", "test_mode_immutable").
			AssertValidationError("test_mode")
	})

	t.Run("is for admins and owners only", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)
		admin, _ := join(t, tc, enums.RoleAdmin)

		in(tc, member).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}).
			AssertForbidden().
			AssertJSONPath("code", "forbidden")
		in(tc, admin).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Ours"}).AssertOK()
	})

	t.Run("keeps a separate organization name once it holds other workspaces", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		exec(t, tc, "UPDATE organizations SET name = 'Holding' WHERE id = $1", tc.Workspace.Workspace.OrganizationID)
		other, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)
		exec(t, tc, "UPDATE workspaces SET organization_id = $1 WHERE id = $2",
			tc.Workspace.Workspace.OrganizationID, other.Workspace.ID)

		in(tc, tc.Owner).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Branch"}).AssertOK()
		tc.Table("organizations").AssertHas(testkit.Row{
			"id": tc.Workspace.Workspace.OrganizationID, "name": "Holding",
		})
	})
}

/*
publicWorkspace returns the public id of a created workspace.
*/
func publicWorkspace(w workspaces.CreatedWorkspace) string {
	return models.PublicID(models.PrefixWorkspace, w.Workspace.ID)
}

/*
dashboardPages lists the first parts of dashboard addresses that are
pages, read from the dashboard's routes, which a slug may never take.
*/
func dashboardPages(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir("../../../resources/js/routes/d")
	require.NoError(t, err)

	var out []string

	for _, e := range entries {
		name := strings.TrimSuffix(e.Name(), ".tsx")
		if name != "index" && name != "route" && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}

	return out
}
