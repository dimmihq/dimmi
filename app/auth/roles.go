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
	}
}

/*
adminPermissions is what admins may do beyond members: run the
workspace's settings, channels, members and integrations.
*/
func adminPermissions() []string {
	return []string{
		"settings:read", "settings:write", "audit_logs:read", "members:write", "tokens:manage_any",
		"widgets:manage", "identity:manage", "conversations:delete", "conversations:moderate",
		"saved_replies:manage_shared", "contacts:erase", "contacts:manage_external_id", "companies:delete",
		"inboxes:manage", "tags:manage", "email:read_raw", "email:manage", "email_suppressions:manage",
		"email_signatures:manage_any", "channels:manage", "channels:inspect", "notification_destinations:manage",
		"notification_alerts:receive", "ai_runs:read_payloads", "ai_agent:manage", "ai:keys", "ai_actions:manage",
		"help_centers:manage", "knowledge:manage", "github:manage", "docs_sync:manage", "oauth_grants:manage_any",
		"agent_access:manage", "feedback:delete", "feedback:manage", "engage:manage", "imports:read",
		"imports:manage", "imports:rollback", "webhooks:read", "webhooks:write", "billing:read", "billing:write",
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
		"exports:write",
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
