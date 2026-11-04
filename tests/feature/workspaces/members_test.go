package workspaces_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestMembersIndex(t *testing.T) {
	t.Parallel()

	t.Run("lists every member with their account, by name", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)

		res := in(tc, user).GetJSON("/api/v1/members").AssertOK().
			AssertJSONPath("data.#", 2).
			AssertJSONPath("data.0.object", "member").
			AssertJSONPath("meta.next_cursor", nil)
		assert.Contains(t, res.String("data.#.id"), memberID(member))
		assert.Contains(t, res.String("data.#.user.email"), user.Email)
	})

	t.Run("narrows by status, role and search, and pages with a cursor", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		for range 3 {
			join(t, tc, enums.RoleAdmin)
		}

		tc.Clock.Travel(time.Minute)
		_, gone := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, gone))
		owner := in(tc, tc.Owner)

		owner.GetJSON("/api/v1/members?filter[status]=deactivated").AssertOK().
			AssertJSONPath("data.#", 1).
			AssertJSONPath("data.0.id", memberID(gone))
		owner.GetJSON("/api/v1/members?filter[role]=owner,member").AssertOK().AssertJSONPath("data.#", 2)
		owner.GetJSON("/api/v1/members?q="+tc.Owner.Email).AssertOK().AssertJSONPath("data.#", 1)

		first := owner.GetJSON("/api/v1/members?limit=3&sort=-created_at").AssertOK().AssertJSONPath("data.#", 3)
		assert.Equal(t, memberID(gone), first.String("data.0.id"))
		owner.GetJSON("/api/v1/members?limit=3&sort=-created_at&cursor="+first.String("meta.next_cursor")).
			AssertOK().
			AssertJSONPath("data.#", 2).
			AssertJSONPath("meta.next_cursor", nil)
	})

	t.Run("refuses filters no member can match and forged cursors", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.GetJSON("/api/v1/members?filter[status]=suspended").AssertStatus(400)
		owner.GetJSON("/api/v1/members?filter[team]=x").AssertStatus(400)
		owner.GetJSON("/api/v1/members?cursor=forged").AssertStatus(400)
	})
}

func TestMembersShow(t *testing.T) {
	t.Parallel()

	t.Run("shows a member, the caller as me, and nobody of another workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		other, err := factories.Workspace(t.Context(), tc.App, tc.CreateUser())
		require.NoError(t, err)
		client := in(tc, user)

		client.GetJSON("/api/v1/members/"+memberID(tc.Workspace.Owner)).AssertOK().
			AssertJSONPath("data.role", "owner").
			AssertJSONPath("data.user.email", tc.Owner.Email)
		client.GetJSON("/api/v1/members/me").AssertOK().AssertJSONPath("data.id", memberID(member))

		for _, id := range []string{memberID(other.Owner), "mem_nope"} {
			client.GetJSON("/api/v1/members/"+id).AssertNotFound().AssertJSONPath("code", "not_found")
		}
	})
}

func TestMembersUpdateRole(t *testing.T) {
	t.Parallel()

	t.Run("lets an admin move members between member and admin", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, _ := join(t, tc, enums.RoleAdmin)
		_, member := join(t, tc, enums.RoleMember)
		changed := record[appevents.MemberRoleChanged](tc)

		in(tc, admin).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "admin"}).AssertOK().
			AssertJSONPath("data.role", "admin")

		require.Len(t, changed.all(), 1)
		assert.Equal(t, "member", changed.all()[0].From)
		assert.Equal(t, "admin", changed.all()[0].To)
	})

	t.Run("answers the current value without an event", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		all := record[any](tc)

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{
			"role": "member", "status": "active",
		}).AssertOK()
		assert.Empty(t, all.all())
	})

	t.Run("keeps members from changing roles and admins from touching owners", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		member, _ := join(t, tc, enums.RoleMember)
		admin, _ := join(t, tc, enums.RoleAdmin)
		_, other := join(t, tc, enums.RoleMember)
		join(t, tc, enums.RoleOwner)
		owner := memberID(tc.Workspace.Owner)

		in(tc, member).PatchJSON("/api/v1/members/"+memberID(other), testkit.JSON{"role": "admin"}).
			AssertForbidden().
			AssertJSONPath("code", "forbidden")
		for _, body := range []testkit.JSON{{"role": "admin"}, {"status": "deactivated"}} {
			in(tc, admin).PatchJSON("/api/v1/members/"+owner, body).AssertForbidden()
		}

		in(tc, admin).PatchJSON("/api/v1/members/"+memberID(other), testkit.JSON{"role": "owner"}).
			AssertForbidden()
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": tc.Workspace.Owner.ID, "role": "owner"})
	})

	t.Run("keeps tokens away from owner changes", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)

		tc.ActingAsToken(tc.Workspace.Owner, "members:write").
			PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"role": "owner"}).
			AssertForbidden().
			AssertJSONPath("code", "first_party_only")
	})

	t.Run("asks an owner who signed in a while ago to confirm first", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, second := join(t, tc, enums.RoleOwner)
		owner := in(tc, tc.Owner)
		tc.Clock.Travel(tc.App.Config.Auth.PasswordConfirmation.TTL + 1)

		owner.PatchJSON("/api/v1/members/"+memberID(second), testkit.JSON{"role": "admin"}).
			AssertStatus(423).
			AssertJSONPath("code", "password_confirmation_required")
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": second.ID, "role": "owner"})
	})

	t.Run("lets an owner step down while another owner remains, never the last", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		me := memberID(tc.Workspace.Owner)

		owner.PatchJSON("/api/v1/members/"+me, testkit.JSON{"role": "admin"}).
			AssertStatus(409).
			AssertJSONPath("code", "last_owner")

		join(t, tc, enums.RoleOwner)
		owner.PatchJSON("/api/v1/members/me", testkit.JSON{"role": "admin"}).AssertOK().
			AssertJSONPath("data.role", "admin")
	})

	t.Run("lets only one of two owners demoting each other at once succeed", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		secondUser, second := join(t, tc, enums.RoleOwner)
		first := in(tc, tc.Owner)
		other := in(tc, secondUser)

		statuses := make([]int, 2)

		var wg sync.WaitGroup

		for i, demote := range []func() *testkit.Response{
			func() *testkit.Response {
				return first.PatchJSON("/api/v1/members/"+memberID(second), testkit.JSON{"role": "admin"})
			},
			func() *testkit.Response {
				return other.PatchJSON("/api/v1/members/"+memberID(tc.Workspace.Owner), testkit.JSON{"role": "admin"})
			},
		} {
			wg.Go(func() { statuses[i] = demote().Status })
		}

		wg.Wait()

		assert.ElementsMatch(t, []int{200, 409}, statuses)
		tc.Table("workspace_members").AssertCount(1, testkit.Row{"role": "owner", "status": "active"})
	})
}

func TestMembersUpdateStatus(t *testing.T) {
	t.Parallel()

	t.Run("withdraws what a deactivated member sent, so an invitation of theirs brings nobody in", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, mailServer())
		fake := tc.Mail()
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		in(tc, admin).PostJSON("/api/v1/invitations", testkit.JSON{
			"invitations": []testkit.JSON{{"email": "alt@example.com", "role": "admin"}},
		}).AssertOK()
		theirs, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		token := tc.Fragment(theirs.AcceptURL).Get("token")
		kept := mailedToken(tc, fake, "kept@example.com", "member")

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(adminMember), testkit.JSON{"status": "deactivated"}).
			AssertOK()

		res := tc.Client().PostJSON("/api/v1/users", testkit.JSON{
			"name": "Alt", "email": "alt@example.com", "password": factories.Password, "invitation_token": token,
		})
		assert.NotEqual(t, fhttp.StatusCreated, res.Status)
		tc.Table("workspace_members").AssertCount(0, testkit.Row{"role": "admin", "status": "active"})
		tc.Table("invitations").AssertHas(testkit.Row{"email": "alt@example.com", "status": "revoked"})
		tc.Table("invitations").AssertHas(testkit.Row{"email": "kept@example.com", "status": "pending"})
		tc.Table("audit_logs").AssertCount(1, testkit.Row{"event": "invitation.revoked"})
		tc.Client().WithHeader("Dimmi-Invitation-Token", kept).GetJSON("/api/v1/invitations/preview").AssertOK()
	})

	t.Run("withdraws what a member sent when they leave the workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t, mailServer())
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		in(tc, admin).PostJSON("/api/v1/invitations", testkit.JSON{
			"invitations": []testkit.JSON{{"email": "alt@example.com", "role": "member"}},
		}).AssertOK()

		tc.ActingAs(admin).Delete("/api/v1/user/memberships/" + memberID(adminMember)).AssertNoContent()

		tc.Table("invitations").AssertHas(testkit.Row{"email": "alt@example.com", "status": "revoked"})
	})

	t.Run("deactivates a member, revoking tokens and teams, with the reassignment hint", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, adminMember := join(t, tc, enums.RoleAdmin)
		_, member := join(t, tc, enums.RoleMember)
		_, plain := tc.MintToken(member, "*")
		team := addTeam(t, tc, member)
		deactivated := record[appevents.MemberDeactivated](tc)

		in(tc, admin).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{
			"status": "deactivated", "reassign_to_member_id": memberID(adminMember),
		}).AssertOK().AssertJSONPath("data.status", "deactivated")

		tc.Client().WithHeader("Authorization", "Bearer "+plain).GetJSON("/api/v1/me").AssertUnauthorized()
		tc.Table("team_members").AssertMissing(testkit.Row{"team_id": team, "workspace_member_id": member.ID})
		tc.Table("workspace_members").AssertHas(testkit.Row{
			"id": member.ID, "deactivated_by_member_id": adminMember.ID,
		})
		require.Len(t, deactivated.all(), 1)
		assert.Equal(t, adminMember.ID, deactivated.all()[0].ReassignTo)
	})

	t.Run("refuses a reassignment to someone who cannot take the work", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		_, gone := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, gone))
		owner := in(tc, tc.Owner)

		for _, target := range []string{memberID(gone), memberID(member), "mem_nope"} {
			owner.PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{
				"status": "deactivated", "reassign_to_member_id": target,
			}).AssertValidationError("reassign_to_member_id")
		}

		tc.Table("workspace_members").AssertHas(testkit.Row{"id": member.ID, "status": "active"})
	})

	t.Run("refuses side fields without the field they belong to, and unknown values", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		path := "/api/v1/members/" + memberID(member)
		owner := in(tc, tc.Owner)

		owner.PatchJSON(path, testkit.JSON{"reassign_to_member_id": memberID(member)}).
			AssertValidationError("reassign_to_member_id")
		owner.PatchJSON(path, testkit.JSON{"keep_owner": true, "role": "admin"}).AssertValidationError("keep_owner")
		owner.PatchJSON(path, testkit.JSON{"status": "suspended"}).AssertValidationError("status")
		owner.PatchJSON(path, testkit.JSON{"role": "owner", "status": "active"}).
			AssertJSONPath("code", "invalid_transition")
	})

	t.Run("refuses to let a member deactivate themselves", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		admin, member := join(t, tc, enums.RoleAdmin)

		in(tc, admin).PatchJSON("/api/v1/members/"+memberID(member), testkit.JSON{"status": "deactivated"}).
			AssertStatus(422).
			AssertJSONPath("code", "invalid_transition").
			AssertJSONPath("from", "active").
			AssertJSONPath("to", "deactivated")
	})

	t.Run("lets an owner deactivate another owner", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		secondUser, second := join(t, tc, enums.RoleOwner)

		in(tc, tc.Owner).PatchJSON("/api/v1/members/"+memberID(second), testkit.JSON{"status": "deactivated"}).
			AssertOK()
		in(tc, secondUser).GetJSON("/api/v1/workspace").AssertForbidden()
	})

	t.Run("reactivates an owner as an admin, or with the role an owner gives", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, second := join(t, tc, enums.RoleOwner)
		_, third := join(t, tc, enums.RoleMember)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, second))
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, third))
		all := record[any](tc)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/members/"+memberID(second), testkit.JSON{"status": "active"}).AssertOK().
			AssertJSONPath("data.status", "active").
			AssertJSONPath("data.role", "admin")
		owner.PatchJSON("/api/v1/members/"+memberID(third), testkit.JSON{"status": "active", "role": "admin"}).
			AssertOK().
			AssertJSONPath("data.role", "admin")

		assert.Equal(t, []string{"reactivated", "reactivated", "role_changed"}, kinds(all.all()))
	})

	t.Run("demotes and deactivates in one request, events in order, or not at all", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, admin := join(t, tc, enums.RoleAdmin)
		all := record[any](tc)
		owner := in(tc, tc.Owner)
		path := "/api/v1/members/" + memberID(admin)

		owner.PatchJSON(path, testkit.JSON{
			"role": "member", "status": "deactivated", "reassign_to_member_id": memberID(admin),
		}).AssertValidationError("reassign_to_member_id")
		tc.Table("workspace_members").AssertHas(testkit.Row{"id": admin.ID, "role": "admin", "status": "active"})

		owner.PatchJSON(path, testkit.JSON{"role": "member", "status": "deactivated"}).AssertOK()
		assert.Equal(t, []string{"role_changed", "deactivated"}, kinds(all.all()))
	})
}

/*
addTeam puts the member in a new team of their workspace and returns
the team's id.
*/
func addTeam(t *testing.T, tc *tests.TestCase, member models.WorkspaceMember) string {
	t.Helper()

	var team string

	require.NoError(t, tc.App.DB.QueryRow(t.Context(),
		"INSERT INTO teams (workspace_id, name) VALUES ($1, 'Support') RETURNING id::text", member.WorkspaceID,
	).Scan(&team))
	exec(t, tc, "INSERT INTO team_members (team_id, workspace_id, workspace_member_id) VALUES ($1, $2, $3)",
		team, member.WorkspaceID, member.ID)

	return team
}

/*
kinds names the member events in the order they went out.
*/
func kinds(seen []any) []string {
	var names []string

	for _, e := range seen {
		switch e.(type) {
		case appevents.MemberReactivated:
			names = append(names, "reactivated")
		case appevents.MemberRoleChanged:
			names = append(names, "role_changed")
		case appevents.MemberDeactivated:
			names = append(names, "deactivated")
		default:
			names = append(names, fmt.Sprintf("%T", e))
		}
	}

	return names
}
