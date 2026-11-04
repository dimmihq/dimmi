/*
|--------------------------------------------------------------------------
| Permission Catalog
|--------------------------------------------------------------------------
|
| A permission names one thing a member may do, as resource:action, and
| belongs to exactly one ability group, which is how tokens reach it.
| Policies check permissions, never role names, so a role can change
| what it holds without touching a single policy.
|
| The catalog is checked when the application boots: every permission
| sits in a known group, carries a well-formed name, and has a row in the
| role table, even when that row gives it to no role. A permission that
| slips through any of those would be granted or refused by accident.
|
*/

package auth

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/dimmihq/dimmi/app/enums"
)

/*
Errors a catalog check reports.

ErrUnknownGroup means a permission names an ability group that does not
exist. ErrInvalidPermission means a name is not resource:action.
ErrUnassignedPermission means the role table has no row for a
registered permission, and ErrUnregisteredPermission that it names a
permission nobody registered.
*/
var (
	ErrUnknownGroup           = errors.New("auth: unknown ability group")
	ErrInvalidPermission      = errors.New("auth: permission must be resource:action")
	ErrUnassignedPermission   = errors.New("auth: permission missing from the role table")
	ErrUnregisteredPermission = errors.New("auth: role table names an unregistered permission")
)

var permissionPattern = regexp.MustCompile(`^[a-z][a-z_]*:[a-z][a-z_]*$`)

/*
Catalog maps every permission to its ability group.
*/
type Catalog struct {
	groups map[string]string
}

/*
NewCatalog returns the catalog of every permission the product defines,
each in its group. Product areas add theirs with Register before boot
finishes.
*/
func NewCatalog() *Catalog {
	c := &Catalog{groups: map[string]string{}}

	for group, permissions := range permissionGroups() {
		for _, permission := range permissions {
			c.groups[permission] = group
		}
	}

	return c
}

/*
Register adds a permission to a group. The catalog is checked at boot,
so a bad name or group stops the application there.
*/
func (c *Catalog) Register(permission, group string) {
	c.groups[permission] = group
}

/*
Group returns the ability group the permission belongs to.
*/
func (c *Catalog) Group(permission string) (string, bool) {
	group, ok := c.groups[permission]

	return group, ok
}

/*
Has reports whether the permission is registered.
*/
func (c *Catalog) Has(permission string) bool {
	_, ok := c.groups[permission]

	return ok
}

/*
Permissions returns every registered permission, sorted.
*/
func (c *Catalog) Permissions() []string {
	return slices.Sorted(maps.Keys(c.groups))
}

/*
Check reports every way the catalog and the role table disagree.
*/
func (c *Catalog) Check() error {
	var errs []error

	for _, permission := range c.Permissions() {
		if !permissionPattern.MatchString(permission) {
			errs = append(errs, fmt.Errorf("%w: %q", ErrInvalidPermission, permission))
		}

		if !slices.Contains(AbilityGroups(), c.groups[permission]) {
			errs = append(errs, fmt.Errorf("%w: %q for %s", ErrUnknownGroup, c.groups[permission], permission))
		}

		if !assigned(permission) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrUnassignedPermission, permission))
		}
	}

	for _, permission := range roleTablePermissions() {
		if !c.Has(permission) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrUnregisteredPermission, permission))
		}
	}

	return errors.Join(errs...)
}

/*
permissionGroups lists the permissions of each ability group. A group
whose only permission is itself lists its own name.
*/
func permissionGroups() map[string][]string {
	return map[string][]string{
		"conversations:read": {"conversations:read", "conversations:export"},
		"conversations:write": {
			"conversations:write", "conversations:delete", "conversations:moderate", "saved_replies:manage_shared",
			"conversations:assign", "conversations:merge", "conversations:manage_notes", "views:manage_shared",
		},
		"contacts:read": {"contacts:read", "contacts:export"},
		"contacts:write": {
			"contacts:write", "contacts:erase", "contacts:manage_external_id", "companies:delete",
			"email_suppressions:manage", "email_suppressions:lift_complaint",
		},
		"articles:read":  {"articles:read"},
		"articles:write": {"articles:write"},
		"members:read":   {"members:read"},
		"members:write":  {"members:write", "members:manage_owners", "tokens:manage_any", "roles:manage"},
		"settings:read": {
			"settings:read", "audit_logs:read", "widgets:read", "identity:read", "email:read", "email:read_raw",
			"channels:inspect", "help_centers:read_settings", "github:read", "imports:read",
		},
		"settings:write":      settingsWritePermissions(),
		"webhooks:read":       {"webhooks:read"},
		"webhooks:write":      {"webhooks:write"},
		"notifications:read":  {"notifications:read", "notification_alerts:receive"},
		"notifications:write": {"notifications:write"},
		"ai:read":             {"ai:read", "ai_runs:read_payloads", "ai_actions:read"},
		"ai:write":            {"ai:write", "ai_agent:manage", "ai_action_calls:approve"},
		"ai:keys":             {"ai:keys"},
		"ai:actions":          {"ai_actions:manage"},
		"exports:read":        {"exports:read"},
		"exports:write":       {"exports:write"},
		"changelog:read":      {"changelog:read"},
		"changelog:write":     {"changelog:write"},
		"feedback:read":       {"feedback:read"},
		"feedback:write":      {"feedback:write", "feedback:delete"},
		"engage:read":         {"engage:read"},
		"engage:write":        {"engage:write", "engage:manage"},
		"billing:read":        {"billing:read"},
		"billing:write":       {"billing:write", "billing:cap_unrestricted"},
		"system:read":         {"system:read"},
		"system:write":        {"system:write"},
	}
}

/*
settingsWritePermissions lists the permissions reached through the
settings:write ability: changing how the workspace and its channels are
set up.
*/
func settingsWritePermissions() []string {
	return []string{
		"settings:write", "workspace:delete", "widgets:manage", "identity:manage", "inboxes:manage", "tags:manage",
		"email:manage", "email:rotate_dkim", "email:resume_sending", "email_signatures:manage_own",
		"email_signatures:manage_any", "channels:manage", "notification_destinations:manage", "help_centers:manage",
		"help_centers:delete", "knowledge:manage", "knowledge:sync", "github:manage", "docs_sync:manage",
		"oauth_grants:manage_any", "agent_access:manage", "feedback:manage", "imports:manage", "imports:rollback",
		"security_policy:manage", "sso:manage", "scim:manage",
	}
}

/*
Assignable reports whether a custom role may list the permission. What
only owners may do, and what no workspace role holds, stays out of reach:
a custom role narrows an admin or a member and never reaches past them.
*/
func Assignable(permission string) bool {
	return slices.Contains(RolePermissions(enums.RoleAdmin), permission)
}

/*
BaseRoles returns the built-in roles a custom role may narrow that hold
the permission, member first.
*/
func BaseRoles(permission string) []enums.Role {
	var roles []enums.Role

	for _, role := range CustomBaseRoles() {
		if slices.Contains(RolePermissions(role), permission) {
			roles = append(roles, role)
		}
	}

	return roles
}

/*
CustomBaseRoles returns the built-in roles a custom role may narrow.
*/
func CustomBaseRoles() []enums.Role {
	return []enums.Role{enums.RoleMember, enums.RoleAdmin}
}
