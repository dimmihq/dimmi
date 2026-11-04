package authz_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/app"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/auth/authz"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/tests"
)

/*
memberRows is what every member may do, as the permissions matrix lists
it.
*/
func memberRows() []string {
	return []string{
		"conversations:read", "conversations:write", "contacts:read", "contacts:write", "articles:read",
		"articles:write", "members:read", "notifications:read", "notifications:write", "widgets:read",
		"identity:read", "email:read", "email_signatures:manage_own", "help_centers:read_settings", "knowledge:sync",
		"github:read", "ai:read", "ai:write", "ai_actions:read", "ai_action_calls:approve", "changelog:read",
		"changelog:write", "feedback:read", "feedback:write", "engage:read", "engage:write",
	}
}

/*
adminRows is what admins may do beyond members, as the matrix lists it.
*/
func adminRows() []string {
	return []string{
		"settings:read", "settings:write", "audit_logs:read", "members:write", "tokens:manage_any", "widgets:manage",
		"identity:manage", "conversations:delete", "conversations:moderate", "saved_replies:manage_shared",
		"contacts:erase", "contacts:manage_external_id", "companies:delete", "inboxes:manage", "tags:manage",
		"email:read_raw", "email:manage", "email_suppressions:manage", "email_signatures:manage_any",
		"channels:manage", "channels:inspect", "notification_destinations:manage", "notification_alerts:receive",
		"ai_runs:read_payloads", "ai_agent:manage", "ai:keys", "ai_actions:manage", "help_centers:manage",
		"knowledge:manage", "github:manage", "docs_sync:manage", "oauth_grants:manage_any", "agent_access:manage",
		"feedback:delete", "feedback:manage", "engage:manage", "imports:read", "imports:manage", "imports:rollback",
		"webhooks:read", "webhooks:write", "billing:read", "billing:write", "members:reset_two_factor",
		"members:import",
	}
}

/*
ownerRows is what only owners may do, as the matrix lists it.
*/
func ownerRows() []string {
	return []string{
		"workspace:delete", "members:manage_owners", "email:rotate_dkim", "email:resume_sending",
		"email_suppressions:lift_complaint", "help_centers:delete", "billing:cap_unrestricted", "exports:read",
		"exports:write",
	}
}

func sorted(lists ...[]string) []string {
	all := slices.Concat(lists...)
	slices.Sort(all)

	return all
}

func TestRoles(t *testing.T) {
	t.Parallel()

	tc := tests.New(t)

	t.Run("gives each role exactly the permissions of the matrix, each role holding the one below", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, sorted(memberRows()), appauth.RolePermissions(enums.RoleMember))
		assert.Equal(t, sorted(memberRows(), adminRows()), appauth.RolePermissions(enums.RoleAdmin))
		assert.Equal(t, sorted(memberRows(), adminRows(), ownerRows()), appauth.RolePermissions(enums.RoleOwner))
		assert.Empty(t, appauth.RolePermissions(enums.Role("guest")))
	})

	t.Run("grants running the server to no workspace role", func(t *testing.T) {
		t.Parallel()

		for _, role := range enums.Roles() {
			assert.NotContains(t, appauth.RolePermissions(role), "system:write")
		}
		assert.True(t, tc.App.Permissions.Has("system:write"))
	})

	t.Run("narrows a token to the groups it holds, the wildcard reaching every group", func(t *testing.T) {
		t.Parallel()
		catalog := tc.App.Permissions

		narrowed := authz.EffectivePermissions(catalog, enums.RoleAdmin, []string{"settings:read"})
		everything := authz.EffectivePermissions(catalog, enums.RoleAdmin, []string{"*"})

		assert.Contains(t, narrowed, "audit_logs:read")
		assert.NotContains(t, narrowed, "settings:write")
		assert.Equal(t, appauth.RolePermissions(enums.RoleAdmin), everything)
		assert.Contains(t, authz.EffectiveAbilities(catalog, everything), "ai:actions")
		assert.Empty(t, authz.EffectivePermissions(catalog, enums.RoleMember, []string{"not:a-group"}))
	})

	t.Run("answers whether a principal holds a permission, never one nobody registered", func(t *testing.T) {
		t.Parallel()
		member := authz.Principal{Role: enums.RoleMember, Permissions: appauth.RolePermissions(enums.RoleMember)}

		assert.True(t, authz.Can(member, "conversations:write"))
		assert.False(t, authz.Can(member, "settings:write"))
		assert.False(t, authz.Can(member, "made:up"))
	})
}

func TestPermissionCatalog(t *testing.T) {
	t.Parallel()

	for name, change := range map[string]func(a *app.App){
		"refuses to boot with a permission in no known group": func(a *app.App) {
			a.Permissions.Register("widgets:explode", "widgets:write")
		},
		"refuses to boot with a permission the role table lacks": func(a *app.App) {
			a.Permissions.Register("widgets:explode", "settings:write")
		},
		"refuses to boot with a permission named the wrong way": func(a *app.App) {
			a.Permissions.Register("Explode", "settings:write")
		},
		"refuses to boot when the role table names an unregistered permission": func(a *app.App) {
			a.Permissions = &appauth.Catalog{}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, err := tests.Boot(t, tests.Before(change))

			require.Error(t, err)
		})
	}

	t.Run("refuses to boot with a trusted origin that is not one", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv("DIMMI_HTTP__TRUSTED_ORIGINS=not an origin"))

		require.Error(t, err)
	})

	t.Run("refuses to boot when passwords cannot be hashed", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t, tests.WithEnv("DIMMI_HASHING__DEFAULT=scrypt"))

		require.Error(t, err)
	})

	t.Run("boots with the catalog as shipped", func(t *testing.T) {
		t.Parallel()

		_, _, err := tests.Boot(t)

		require.NoError(t, err)
	})
}

func TestRegisteredRule(t *testing.T) {
	t.Parallel()

	tc := tests.New(t)

	t.Run("accepts registered values and refuses the rest", func(t *testing.T) {
		t.Parallel()
		type input struct {
			Method string `json:"method" validate:"registered=auth_methods"`
		}
		type unknownRegistry struct {
			Method string `json:"method" validate:"registered=colors"`
		}

		require.NoError(t, tc.App.Validator.Struct(t.Context(), input{Method: "github"}))
		require.Error(t, tc.App.Validator.Struct(t.Context(), input{Method: "carrier_pigeon"}))
		require.Error(t, tc.App.Validator.Struct(t.Context(), unknownRegistry{Method: "github"}))
	})
}
