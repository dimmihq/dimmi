package workspaces_test

import (
	"regexp"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	appauth "github.com/dimmihq/dimmi/app/auth"
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

	t.Run("gives workspaces created at the same moment with one name their own slugs", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		name := "Rush " + tc.Owner.ID.String()[:8]

		var (
			wait  sync.WaitGroup
			mu    sync.Mutex
			slugs []string
			errs  []error
		)

		for range 8 {
			wait.Go(func() {
				created, err := tc.App.Actions.CreateWorkspace.Handle(t.Context(), workspaces.CreateWorkspaceInput{
					Owner: tc.Owner, Name: name, JoinedVia: appauth.JoinedViaCreated,
				})

				mu.Lock()
				defer mu.Unlock()

				slugs, errs = append(slugs, created.Workspace.Slug), append(errs, err)
			})
		}

		wait.Wait()

		for _, err := range errs {
			require.NoError(t, err)
		}

		assert.ElementsMatch(t, slugs, slices.Compact(slices.Sorted(slices.Values(slugs))))
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
