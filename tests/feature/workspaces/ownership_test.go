package workspaces_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestOwnershipTransfer(t *testing.T) {
	t.Parallel()

	t.Run("refuses a token that deactivates or reactivates an owner, whatever it holds", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		secondUser, second := join(t, tc, enums.RoleOwner)
		_, third := join(t, tc, enums.RoleOwner)
		owner := in(tc, tc.Owner)
		owner.PatchJSON("/api/v1/members/"+memberID(third), testkit.JSON{"status": "deactivated"}).AssertOK()
		token := tc.ActingAsToken(tc.Workspace.Owner, "members:write")

		token.PatchJSON("/api/v1/members/"+memberID(second), testkit.JSON{"status": "deactivated"}).
			AssertStatus(fhttp.StatusForbidden).AssertJSONPath("code", "first_party_only")
		token.PatchJSON("/api/v1/members/"+memberID(third), testkit.JSON{"status": "active"}).
			AssertStatus(fhttp.StatusForbidden).AssertJSONPath("code", "first_party_only")

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": second.ID, "status": "active"})
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": third.ID, "status": "deactivated"})
		in(tc, secondUser).GetJSON("/api/v1/workspace").AssertOK()
	})

	t.Run("asks a session gone idle to confirm before it deactivates or reactivates an owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, second := join(t, tc, enums.RoleOwner)
		_, third := join(t, tc, enums.RoleOwner)
		owner := in(tc, tc.Owner)
		owner.PatchJSON("/api/v1/members/"+memberID(third), testkit.JSON{"status": "deactivated"}).AssertOK()
		tc.Clock.Advance(2 * time.Hour)

		owner.PatchJSON("/api/v1/members/"+memberID(second), testkit.JSON{"status": "deactivated"}).
			AssertStatus(fhttp.StatusLocked).AssertJSONPath("code", "password_confirmation_required")
		owner.PatchJSON("/api/v1/members/"+memberID(third), testkit.JSON{"status": "active"}).
			AssertStatus(fhttp.StatusLocked).AssertJSONPath("code", "password_confirmation_required")

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": second.ID, "status": "active"})
	})

	t.Run("still lets a token deactivate a member who is no owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)

		tc.ActingAsToken(tc.Workspace.Owner, "members:write").
			PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).AssertOK()
	})

	t.Run("hands ownership over, the old owner stepping down to admin", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, admin := join(t, tc, enums.RoleAdmin)
		changed := record[appevents.MemberRoleChanged](tc)

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(admin), testkit.JSON{"role": "owner"}).AssertOK().
			AssertJSONPath("data.role", "owner")

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": tc.Workspace.Owner.ID, "role": "admin"})
		assert.Len(t, changed.all(), 2)
	})

	t.Run("keeps the caller an owner when asked", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		changed := record[appevents.MemberRoleChanged](tc)

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{
			"role": "owner", "keep_owner": true,
		}).AssertOK()

		tc.Table("workspace_members").AssertCount(2, testkit.Row{"role": "owner"})
		assert.Len(t, changed.all(), 1)
	})

	t.Run("hands ownership only to an active member who verified their email", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, unverified := join(t, tc, enums.RoleMember, factories.Unverified)
		_, gone := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/members/"+memberID(unverified), testkit.JSON{"role": "owner"}).
			AssertStatus(409).
			AssertJSONPath("code", "target_unverified")

		if err := factories.Deactivate(t.Context(), tc.App, gone); err != nil {
			t.Fatal(err)
		}

		owner.PatchJSON("/api/v1/members/"+memberID(gone), testkit.JSON{"role": "owner"}).
			AssertJSONPath("code", "invalid_transition")
		tc.Table("workspace_members").AssertCount(1, testkit.Row{"role": "owner"})
	})

	t.Run("hands ownership only to a member with two-factor where the workspace requires it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		exec(t, tc, `UPDATE workspaces SET settings = '{"security": {"require_two_factor": true}}' WHERE id = $1`,
			tc.Workspace.Workspace.ID)
		user, member := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)
		if _, err := factories.EnableTwoFactor(t.Context(), tc.App, tc.Owner); err != nil {
			t.Fatal(err)
		}

		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "owner"}).
			AssertStatus(409).
			AssertJSONPath("code", "target_two_factor_required")

		if _, err := factories.EnableTwoFactor(t.Context(), tc.App, user); err != nil {
			t.Fatal(err)
		}

		owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "owner"}).AssertOK()
	})
}
