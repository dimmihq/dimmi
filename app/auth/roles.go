/*
|--------------------------------------------------------------------------
| Roles
|--------------------------------------------------------------------------
|
| This is the only place that says what a role may do. Each role holds
| everything the role below it holds, and more: an owner can do
| everything an admin can, an admin everything a member can. Policies ask
| for permissions and never name a role, so moving a permission between
| roles is an edit to this file alone.
|
| Some permissions belong to no role at all. Running the server itself
| is granted to instance administrators, never by a workspace role.
|
*/

package auth

import (
	"slices"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
RolePermissions returns the permissions the role holds, sorted. An
unknown role holds none.
*/
func RolePermissions(role enums.Role) []string {
	var held []string

	switch role {
	case enums.RoleOwner:
		held = slices.Concat(memberPermissions(), adminPermissions(), ownerPermissions())
	case enums.RoleAdmin:
		held = slices.Concat(memberPermissions(), adminPermissions())
	case enums.RoleMember:
		held = memberPermissions()
	}

	slices.Sort(held)

	return held
}

/*
memberPermissions is what every member may do: work conversations,
contacts and articles, and use the AI and engagement features.
*/
func memberPermissions() []string {
	return []string{
		"conversations:read", "conversations:write", "contacts:read", "contacts:write", "articles:read",
		"articles:write", "members:read", "notifications:read", "notifications:write", "widgets:read",
		"identity:read", "email:read", "email_signatures:manage_own", "help_centers:read_settings",
		"knowledge:sync", "github:read", "ai:read", "ai:write", "ai_actions:read", "ai_action_calls:approve",
		"changelog:read", "changelog:write", "feedback:read", "feedback:write", "engage:read", "engage:write",
		"conversations:assign", "conversations:merge", "workflows:read", "workflows:run", "audiences:read",
	}
}

/*
adminPermissions is what admins may do beyond members: run the
workspace's settings, channels, members and integrations.
*/
func adminPermissions() []string {
	return []string{
		"settings:read", "settings:write", "audit_logs:read", "members:write", "tokens:manage_any",
		"members:manage_access", "members:manage_availability",
		"widgets:manage", "identity:manage", "conversations:delete", "conversations:moderate",
		"saved_replies:manage_shared", "views:manage_shared", "contacts:erase", "contacts:manage_external_id",
		"companies:delete",
		"inboxes:manage", "tags:manage", "email:read_raw", "email:manage", "email_suppressions:manage",
		"email_signatures:manage_any", "channels:manage", "channels:inspect", "notification_destinations:manage",
		"notification_alerts:receive", "ai_runs:read_payloads", "ai_agent:manage", "ai:keys", "ai_actions:manage",
		"help_centers:manage", "knowledge:manage", "github:manage", "docs_sync:manage", "oauth_grants:manage_any",
		"agent_access:manage", "feedback:delete", "feedback:manage", "engage:manage", "imports:read",
		"imports:manage", "imports:rollback", "webhooks:read", "webhooks:write", "billing:read", "billing:write",
		"security_policy:manage", "roles:manage",
		"conversations:export", "conversations:manage_notes", "contacts:export",
		"members:reset_two_factor", "members:import",
		"redaction:manage", "workflows:manage", "schedules:manage", "audiences:manage",
	}
}

/*
ownerPermissions is what only owners may do: delete the workspace,
manage other owners, and take the steps that cannot be undone.
*/
func ownerPermissions() []string {
	return []string{
		"workspace:delete", "members:manage_owners", "email:rotate_dkim", "email:resume_sending",
		"email_suppressions:lift_complaint", "help_centers:delete", "billing:cap_unrestricted", "exports:read",
		"exports:write", "sso:manage", "scim:manage",
	}
}

/*
unassignedPermissions are held by no workspace role on purpose.
*/
func unassignedPermissions() []string {
	return []string{"system:read", "system:write"}
}

/*
roleTablePermissions returns every permission the role table mentions.
*/
func roleTablePermissions() []string {
	return slices.Concat(RolePermissions(enums.RoleOwner), unassignedPermissions())
}

/*
assigned reports whether the role table has a row for the permission.
*/
func assigned(permission string) bool {
	return slices.Contains(roleTablePermissions(), permission)
}

/*
IsOwnerRole reports whether the role owns its workspace. Changes to who
owns a workspace follow stricter rules than other role changes, and a
workspace must always keep an active owner, so the actions that change
roles ask this instead of naming the role.
*/
func IsOwnerRole(role enums.Role) bool {
	return role == enums.RoleOwner
}

/*
OwnerRole returns the role that owns a workspace.
*/
func OwnerRole() enums.Role {
	return enums.RoleOwner
}

/*
ManagerRole returns the role an owner keeps after handing ownership
over, and the role a deactivated owner comes back with.
*/
func ManagerRole() enums.Role {
	return enums.RoleAdmin
}

/*
RoleChanged reports whether a member's built-in role differs before and
after a change, so the change can be announced. It decides nothing about
what either role may do.
*/
func RoleChanged(from, to string) bool {
	return from != to
}

/*
RoleOutranks reports whether the built-in role gives more than the
other: owner over admin over member, so the strongest of several roles
a member is handed wins.
*/
func RoleOutranks(role, other enums.Role) bool {
	rank := func(r enums.Role) int {
		return slices.Index([]enums.Role{enums.RoleMember, enums.RoleAdmin, enums.RoleOwner}, r)
	}

	return rank(role) > rank(other)
}
