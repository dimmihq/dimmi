package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestMembershipsIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists the user's memberships, the one opened last first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		second, err := factories.Workspace(t.Context(), tc.App, tc.Owner)
		require.NoError(t, err)

		owner := tc.ActingAsOwner()
		owner.GetJSON("/api/v1/user/memberships").AssertOK().
			AssertJSONPath("data.0.object", "membership").
			AssertJSONPath("data.0.last_accessed_at", nil).
			AssertJSONPath("data.1.object", "membership")

		tc.Clock.Travel(time.Hour)
		secondID := models.PublicID(models.PrefixWorkspace, second.Workspace.ID)
		owner.WithHeader("Dimmi-Workspace", secondID).GetJSON("/api/v1/workspace").AssertOK()

		owner.GetJSON("/api/v1/user/memberships").AssertOK().
			AssertJSONPath("data.0.id", memberID(second.Owner)).
			AssertJSONPath("data.0.role", "owner").
			AssertJSONPath("data.0.joined_via", "created").
			AssertJSONPath("data.0.workspace.id", secondID).
			AssertJSONPath("data.0.workspace.deleted_at", nil).
			AssertJSONPath("data.1.workspace.id", workspaceID(tc))
	})

	t.Run("leaves out deactivated memberships and workspaces in the trash", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleAdmin)
		own, err := factories.Workspace(t.Context(), tc.App, user)
		require.NoError(t, err)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, member))
		exec(t, tc, "UPDATE workspaces SET deleted_at = now(), purge_after = now() + interval '30 days' WHERE id = $1",
			own.Workspace.ID)

		tc.ActingAs(user).GetJSON("/api/v1/user/memberships").AssertOK().AssertJSONPath("data", []any{})
	})

	t.Run("shows workspaces in the trash to their owners only, when asked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		exec(t, tc, "UPDATE workspaces SET deleted_at = now(), purge_after = now() + interval '30 days' WHERE id = $1",
			tc.Workspace.Workspace.ID)

		owner := tc.ActingAsOwner()
		owner.GetJSON("/api/v1/user/memberships?filter[trashed]=with").AssertOK().
			AssertJSONPath("data.0.workspace.id", workspaceID(tc)).
			AssertJSONPath("data.#", 1)
		owner.GetJSON("/api/v1/user/memberships?filter[trashed]=only").AssertOK().AssertJSONPath("data.#", 1)
		tc.ActingAs(admin).GetJSON("/api/v1/user/memberships?filter[trashed]=with").AssertOK().
			AssertJSONPath("data", []any{})
	})

	t.Run("refuses an unknown trash filter", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner := tc.ActingAsOwner()
		owner.GetJSON("/api/v1/user/memberships?filter[trashed]=maybe").AssertStatus(400)
		owner.GetJSON("/api/v1/user/memberships?filter[role]=owner").AssertStatus(400)
	})

	t.Run("is for the user themselves, not their tokens", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "*").GetJSON("/api/v1/user/memberships").
			AssertForbidden().
			AssertJSONPath("code", "first_party_only")
	})
}

func TestMembershipsDestroy(t *testing.T) {
	t.Parallel()

	t.Run("leaves the workspace, ending its tokens and access", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		_, plain := tc.MintToken(member, "*")
		deactivated := record[appevents.MemberDeactivated](tc)

		tc.ActingAs(user).Delete("/api/v1/user/memberships/" + memberID(member)).AssertNoContent()

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": member.ID, "status": "deactivated"})
		tc.Client().WithHeader("Authorization", "Bearer "+plain).GetJSON("/api/v1/me").AssertUnauthorized()
		in(tc, user).GetJSON("/api/v1/workspace").AssertJSONPath("code", "membership_deactivated")
		require.Len(t, deactivated.all(), 1)
		require.Equal(t, member.ID, deactivated.all()[0].DeactivatedBy)
	})

	t.Run("changes nothing when the membership is already deactivated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, member))
		deactivated := record[appevents.MemberDeactivated](tc)

		tc.ActingAs(user).Delete("/api/v1/user/memberships/" + memberID(member)).AssertNoContent()
		require.Empty(t, deactivated.all())
	})

	t.Run("does not find someone else's membership", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)

		client := tc.ActingAs(user)
		client.Delete("/api/v1/user/memberships/"+memberID(tc.Workspace.Owner)).
			AssertNotFound().
			AssertJSONPath("code", "not_found")
		client.Delete("/api/v1/user/memberships/mem_nope").AssertNotFound()
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": tc.Workspace.Owner.ID, "status": "active"})
	})

	t.Run("keeps the last owner from leaving, and lets one of two go", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		owner := tc.ActingAsOwner()
		owner.Delete("/api/v1/user/memberships/"+memberID(tc.Workspace.Owner)).
			AssertStatus(409).
			AssertJSONPath("code", "last_owner")

		join(t, tc, enums.RoleOwner)
		owner.Delete("/api/v1/user/memberships/" + memberID(tc.Workspace.Owner)).AssertNoContent()
	})
}

/*
membershipsOf returns the input listing the user's memberships.
*/
func membershipsOf(user models.User) workspaces.ListMembershipsInput {
	return workspaces.ListMembershipsInput{UserID: user.ID}
}
