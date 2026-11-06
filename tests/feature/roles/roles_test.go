package roles_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/tests"
)

func TestRoleRules(t *testing.T) {
	t.Parallel()

	t.Run("a custom role can only hold what its base role holds", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)

		owner.PostJSON(rolesPath, testkit.JSON{
			"name": "Agent", "base_role": "member", "permissions": []string{"members:read", "settings:write"},
		}).AssertStatus(422).AssertJSONPath("code", "permission_above_base_role").
			AssertJSONPath("permissions.0", "settings:write")

		owner.PostJSON(rolesPath, testkit.JSON{
			"name": "Boss", "base_role": "admin", "permissions": []string{"members:read", "workspace:delete"},
		}).AssertStatus(422).AssertJSONPath("code", "permission_not_assignable")

		owner.PostJSON(rolesPath, testkit.JSON{
			"name": "Agent", "base_role": "owner", "permissions": []string{"members:read"},
		}).AssertValidationError("base_role")

		owner.PostJSON(rolesPath, testkit.JSON{
			"name": "Agent", "base_role": "member", "permissions": []string{"conversations:read"},
		}).AssertStatus(422).AssertJSONPath("code", "permission_required")

		owner.PostJSON(rolesPath, testkit.JSON{
			"name": "Agent", "base_role": "member", "permissions": []string{"members:read", "no:such"},
		}).AssertStatus(422).AssertJSONPath("code", "permission_not_assignable")
	})

	t.Run("a caller cannot grant a permission they do not hold", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleAdmin)
		holding(t, tc, member, role(t, tc, enums.RoleAdmin, "roles:manage", "members:write"))
		manager := in(tc, user)

		manager.PostJSON(rolesPath, testkit.JSON{
			"name": "Billing", "base_role": "admin", "permissions": []string{"members:read", "billing:read"},
		}).AssertStatus(422).AssertJSONPath("code", "permission_not_held").
			AssertJSONPath("permissions.0", "billing:read")
		manager.PostJSON(rolesPath, testkit.JSON{
			"name": "Reader", "base_role": "member", "permissions": []string{"members:read"},
		}).AssertCreated()

		strong := role(t, tc, enums.RoleAdmin, "billing:read")
		_, other := join(t, tc, enums.RoleMember)
		manager.PatchJSON(memberPath(other), testkit.JSON{"custom_role_id": roleID(strong)}).
			AssertStatus(422).AssertJSONPath("code", "permission_not_held")
		manager.PatchJSON(rolesPath+"/"+roleID(strong), testkit.JSON{"name": "Mine"}).
			AssertStatus(422).AssertJSONPath("code", "permission_not_held")
		manager.Delete(rolesPath + "/" + roleID(strong)).AssertStatus(422)
	})

	t.Run("the base role cannot change and the name stays unique", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		existing := role(t, tc, enums.RoleMember)

		owner.PatchJSON(rolesPath+"/"+roleID(existing), testkit.JSON{"base_role": "admin"}).
			AssertValidationError("base_role")
		owner.PostJSON(rolesPath, testkit.JSON{
			"name": "Support", "base_role": "member", "permissions": []string{"members:read"},
		}).AssertCreated()
		owner.PostJSON(rolesPath, testkit.JSON{
			"name": "SUPPORT", "base_role": "member", "permissions": []string{"members:read"},
		}).AssertStatus(422).AssertJSONPath("code", "role_name_taken")
		owner.PatchJSON(rolesPath+"/"+roleID(existing), testkit.JSON{"name": "support"}).
			AssertStatus(422).AssertJSONPath("code", "role_name_taken")
	})

	t.Run("a workspace holds at most fifty custom roles", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		for range 50 {
			role(t, tc, enums.RoleMember)
		}

		in(tc, tc.Owner).PostJSON(rolesPath, testkit.JSON{
			"name": "One more", "base_role": "member", "permissions": []string{"members:read"},
		}).AssertStatus(429).AssertJSONPath("code", "entitlement_limit_reached")
	})

	t.Run("members without roles manage cannot shape roles, and tokens never", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, _ := join(t, tc, enums.RoleMember)
		body := testkit.JSON{"name": "Agent", "base_role": "member", "permissions": []string{"members:read"}}

		in(tc, user).PostJSON(rolesPath, body).AssertForbidden()
		tc.ActingAsToken(tc.Workspace.Owner, "members:write").PostJSON(rolesPath, body).AssertForbidden()
	})
}

func TestRoleAssignment(t *testing.T) {
	t.Parallel()

	t.Run("assigning a role narrows what the member can do", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		narrow := role(t, tc, enums.RoleAdmin, "settings:read")
		browser := in(tc, user)

		in(tc, tc.Owner).PatchJSON(memberPath(member), testkit.JSON{"custom_role_id": roleID(narrow)}).AssertOK().
			AssertJSONPath("data.role", "admin").
			AssertJSONPath("data.custom_role.id", roleID(narrow)).
			AssertJSONPath("data.custom_role.name", narrow.Name)

		browser.GetJSON("/api/v1/workspace").AssertOK()
		browser.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}).AssertForbidden()
		var me struct {
			Data struct {
				Member struct {
					Permissions []string `json:"permissions"`
				} `json:"member"`
			} `json:"data"`
		}
		browser.GetJSON("/api/v1/me").AssertOK().Decode(&me)
		assert.ElementsMatch(t, []string{"members:read", "settings:read"}, me.Data.Member.Permissions)
	})

	t.Run("a token of that member narrows too", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleAdmin)
		member = holding(t, tc, member, role(t, tc, enums.RoleAdmin, "settings:read"))
		token := tc.ActingAsToken(member, "*")

		token.GetJSON("/api/v1/workspace").AssertOK()
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}).AssertForbidden()
		token.GetJSON("/api/v1/members").AssertOK()
	})

	t.Run("a demoted token ceiling still applies", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		res := in(tc, user).PostJSON("/api/v1/tokens", testkit.JSON{"name": "Mine", "abilities": []string{"*"}}).
			AssertCreated()
		token := tc.Client().WithHeader("Authorization", "Bearer "+res.String("data.plain_text_token"))
		holding(t, tc, member, role(t, tc, enums.RoleAdmin, "settings:read", "settings:write"))

		in(tc, user).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "By role"}).AssertOK()
		token.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "By token"}).AssertForbidden()
	})

	t.Run("editing a role applies on the next request", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleAdmin)
		edited := role(t, tc, enums.RoleAdmin, "settings:read")
		holding(t, tc, member, edited)
		browser := in(tc, user)
		browser.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}).AssertForbidden()

		in(tc, tc.Owner).PatchJSON(rolesPath+"/"+roleID(edited), testkit.JSON{
			"permissions": []string{"members:read", "settings:read", "settings:write"},
		}).AssertOK().AssertJSONPath("data.member_count", 1.0)

		browser.PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}).AssertOK()
	})

	t.Run("owners cannot hold a custom role and nobody assigns their own", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, otherMember := join(t, tc, enums.RoleOwner)
		r := role(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)

		owner.PatchJSON(memberPath(otherMember), testkit.JSON{"custom_role_id": roleID(r)}).
			AssertStatus(422).AssertJSONPath("code", "invalid_transition")
		owner.PatchJSON(memberPath(tc.Workspace.Owner), testkit.JSON{"custom_role_id": roleID(r)}).
			AssertStatus(422).AssertJSONPath("code", "invalid_transition")

		admin, adminMember := join(t, tc, enums.RoleAdmin)
		in(tc, admin).PatchJSON(memberPath(adminMember), testkit.JSON{"custom_role_id": roleID(r)}).
			AssertStatus(422).AssertJSONPath("code", "invalid_transition")
	})

	t.Run("setting a built-in role clears the custom one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, member := join(t, tc, enums.RoleMember)
		member = holding(t, tc, member, role(t, tc, enums.RoleAdmin, "settings:read"))
		owner := in(tc, tc.Owner)

		owner.PatchJSON(memberPath(member), testkit.JSON{"role": "member"}).AssertOK().
			AssertJSONPath("data.role", "member").
			AssertJSONPath("data.custom_role", nil)
	})

	t.Run("taking a custom role away leaves the base role whole", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleAdmin)
		member = holding(t, tc, member, role(t, tc, enums.RoleAdmin, "settings:read"))
		owner := in(tc, tc.Owner)

		owner.PatchJSON(memberPath(member), testkit.JSON{"custom_role_id": nil}).AssertOK().
			AssertJSONPath("data.role", "admin").
			AssertJSONPath("data.custom_role", nil)
		owner.PatchJSON(memberPath(member), testkit.JSON{"custom_role_id": nil}).AssertOK()
		in(tc, user).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}).AssertOK()
		owner.PatchJSON(memberPath(member), testkit.JSON{"custom_role_id": nil, "role": "member"}).
			AssertValidationError("custom_role_id")
	})

	t.Run("a role of another workspace is not found", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tests.New(t)
		foreign := role(t, other, enums.RoleMember)
		_, member := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)

		owner.GetJSON(rolesPath + "/" + roleID(foreign)).AssertNotFound()
		owner.PatchJSON(rolesPath+"/"+roleID(foreign), testkit.JSON{"name": "Mine"}).AssertNotFound()
		owner.Delete(rolesPath + "/" + roleID(foreign)).AssertNotFound()
		owner.PatchJSON(memberPath(member), testkit.JSON{"custom_role_id": roleID(foreign)}).AssertNotFound()
		owner.GetJSON(rolesPath + "/rol_nope").AssertNotFound()
	})
}

func TestRoleLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("a role in use cannot be deleted until its holders move", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		used := role(t, tc, enums.RoleMember, "conversations:read")
		_, a := join(t, tc, enums.RoleMember)
		_, b := join(t, tc, enums.RoleMember)
		holding(t, tc, a, used)
		holding(t, tc, b, used)
		owner := in(tc, tc.Owner)

		owner.Delete(rolesPath+"/"+roleID(used)).AssertStatus(409).
			AssertJSONPath("code", "role_in_use").AssertJSONPath("member_count", 2.0)

		res := owner.PatchJSON("/api/v1/members", testkit.JSON{
			"ids":     []string{memberID(a), memberID(b), "mem_nope"},
			"changes": testkit.JSON{"custom_role_id": nil},
		}).AssertOK()
		res.AssertJSONPath("meta.updated", 2.0).AssertJSONPath("meta.failed", 1.0).
			AssertJSONPath("data.2.status", "failed").AssertJSONPath("data.2.error.code", "not_found")

		owner.Delete(rolesPath + "/" + roleID(used)).AssertNoContent()
		owner.GetJSON(rolesPath + "/" + roleID(used)).AssertNotFound()
	})

	t.Run("bulk changes skip members already there and refuse an unclear change", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		target := role(t, tc, enums.RoleMember, "conversations:read")
		_, a := join(t, tc, enums.RoleMember)
		holding(t, tc, a, target)
		_, b := join(t, tc, enums.RoleMember)
		owner := in(tc, tc.Owner)

		owner.PatchJSON("/api/v1/members", testkit.JSON{
			"ids":     []string{memberID(a), memberID(b)},
			"changes": testkit.JSON{"custom_role_id": roleID(target)},
		}).AssertOK().AssertJSONPath("meta.skipped", 1.0).AssertJSONPath("meta.updated", 1.0)
		owner.PatchJSON("/api/v1/members", testkit.JSON{
			"ids": []string{memberID(a)}, "changes": testkit.JSON{"role": "admin"},
		}).AssertOK().AssertJSONPath("meta.updated", 1.0)
		owner.PatchJSON("/api/v1/members", testkit.JSON{
			"ids": []string{memberID(a)}, "changes": testkit.JSON{},
		}).AssertValidationError("changes")
	})

	t.Run("the list shows built-in roles first and counts holders only for managers", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user, member := join(t, tc, enums.RoleMember)
		held := role(t, tc, enums.RoleMember, "conversations:read")
		holding(t, tc, member, held)

		in(tc, tc.Owner).GetJSON(rolesPath).AssertOK().
			AssertJSONPath("data.0.kind", "built_in").AssertJSONPath("data.0.id", nil).
			AssertJSONPath("data.0.base_role", "owner").
			AssertJSONPath("data.3.kind", "custom").AssertJSONPath("data.3.member_count", 1.0)
		in(tc, user).GetJSON(rolesPath).AssertOK().AssertJSONPath("data.3.member_count", nil)
		in(tc, user).GetJSON(rolesPath+"/"+roleID(held)).AssertOK().AssertJSONPath("data.member_count", nil)
		in(tc, tc.Owner).GetJSON(rolesPath+"/"+roleID(held)).AssertOK().AssertJSONPath("data.member_count", 1.0)
		in(tc, user).GetJSON("/api/v1/members").AssertOK()
	})

	t.Run("the catalog lists labels for every assignable permission", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		res := in(tc, tc.Owner).GetJSON("/api/v1/permissions").AssertOK()

		var catalog struct {
			Data []struct {
				Key         string `json:"key"`
				Label       string `json:"label"`
				Description string `json:"description"`
				Assignable  bool   `json:"assignable"`
			} `json:"data"`
		}
		res.Decode(&catalog)
		require.Len(t, catalog.Data, len(tc.App.Permissions.Permissions()))

		for _, p := range catalog.Data {
			assert.Equal(t, appauth.Assignable(p.Key), p.Assignable, p.Key)
			assert.NotEqual(t, "permissions."+p.Key+".label", p.Label, p.Key)
			assert.NotEqual(t, "permissions."+p.Key+".description", p.Description, p.Key)
		}

		res.AssertJSONPath(`data.#(key=="settings:write").base_roles.0`, "admin")
		res.AssertJSONPath(`data.#(key=="conversations:read").base_roles.#`, 2.0)
	})

	t.Run("changes land in the audit log", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		owner := in(tc, tc.Owner)
		_, member := join(t, tc, enums.RoleMember)

		created := owner.PostJSON(rolesPath, testkit.JSON{
			"name": "Agent", "base_role": "member", "description": "Answers chats",
			"permissions": []string{"members:read", "conversations:read"},
		}).AssertCreated()
		id := created.String("data.id")
		owner.PatchJSON(rolesPath+"/"+id, testkit.JSON{
			"permissions": []string{"members:read", "conversations:write"}, "description": "",
		}).AssertOK().AssertJSONPath("data.description", nil)
		owner.PatchJSON(memberPath(member), testkit.JSON{"custom_role_id": id}).AssertOK()
		owner.PatchJSON(memberPath(member), testkit.JSON{"custom_role_id": nil}).AssertOK()
		owner.Delete(rolesPath + "/" + id).AssertNoContent()

		for _, event := range []string{"role.created", "role.updated", "role.deleted", "member.custom_role_changed"} {
			owner.GetJSON("/api/v1/audit-logs?filter[event]="+event).AssertOK().AssertJSONPath("data.0.event", event)
		}

		updated := owner.GetJSON("/api/v1/audit-logs?filter[event]=role.updated").AssertOK()
		updated.AssertJSONPath("data.0.changes.permissions.added.0", "conversations:write")
		updated.AssertJSONPath("data.0.changes.permissions.removed.0", "conversations:read")
	})
}

func TestRoleInvitations(t *testing.T) {
	t.Parallel()

	t.Run("an invitee starts in the custom role the invitation names", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		offered := role(t, tc, enums.RoleAdmin, "settings:read")
		invitee := tc.CreateUser()
		fake := tc.Mail()

		in(tc, tc.Owner).PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{{
			"email": invitee.Email, "role": "member", "custom_role_id": roleID(offered),
		}}}).AssertOK()

		sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		token := tc.Fragment(sent.AcceptURL).Get("token")
		tc.ActingAs(invitee).PostJSON("/api/v1/user/memberships", testkit.JSON{"invitation_token": token}).
			AssertStatus(201)

		in(tc, invitee).GetJSON("/api/v1/members/me").AssertOK().
			AssertJSONPath("data.role", "admin").
			AssertJSONPath("data.custom_role.id", roleID(offered))
		in(tc, invitee).PatchJSON("/api/v1/workspace", testkit.JSON{"name": "Mine"}).AssertForbidden()
	})

	t.Run("only someone who could hold the role may offer it", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		strong := role(t, tc, enums.RoleAdmin, "billing:read")
		user, member := join(t, tc, enums.RoleAdmin)
		holding(t, tc, member, role(t, tc, enums.RoleAdmin, "members:write", "roles:manage"))
		admin, _ := join(t, tc, enums.RoleAdmin)
		body := testkit.JSON{"invitations": []testkit.JSON{{
			"email": "new@example.com", "role": "member", "custom_role_id": roleID(strong),
		}}}

		in(tc, user).PostJSON("/api/v1/invitations", body).
			AssertStatus(422).AssertJSONPath("code", "permission_not_held")
		tc.ActingAsToken(tc.Workspace.Owner, "members:write").PostJSON("/api/v1/invitations", testkit.JSON{
			"invitations": []testkit.JSON{{"email": "x@example.com", "role": "member", "custom_role_id": "rol_nope"}},
		}).AssertNotFound()
		in(tc, admin).PostJSON("/api/v1/invitations", body).AssertOK()
	})

	t.Run("a role deleted before acceptance leaves the invitation's own role", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		offered := role(t, tc, enums.RoleAdmin, "settings:read")
		invitee := tc.CreateUser()
		fake := tc.Mail()
		owner := in(tc, tc.Owner)
		owner.PostJSON("/api/v1/invitations", testkit.JSON{"invitations": []testkit.JSON{{
			"email": invitee.Email, "role": "member", "custom_role_id": roleID(offered),
		}}}).AssertOK()
		owner.Delete(rolesPath + "/" + roleID(offered)).AssertNoContent()

		sent, _ := tests.LastSent[appmail.InvitationMail](tc, fake)
		token := tc.Fragment(sent.AcceptURL).Get("token")
		tc.ActingAs(invitee).PostJSON("/api/v1/user/memberships", testkit.JSON{"invitation_token": token}).
			AssertStatus(201)

		in(tc, invitee).GetJSON("/api/v1/members/me").AssertOK().
			AssertJSONPath("data.role", "admin").AssertJSONPath("data.custom_role", nil)
	})
}
