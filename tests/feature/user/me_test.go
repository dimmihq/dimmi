package user_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/auth/token"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/config"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

func TestMeShow(t *testing.T) {
	t.Parallel()

	t.Run("shows a browser its user and workspaces when it names none", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsOwner().GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.email", tc.Owner.Email).
			AssertJSONPath("data.workspace", nil).
			AssertJSONPath("data.member", nil).
			AssertJSONPath("data.auth.abilities", []string{"*"}).
			AssertJSONPath("data.workspaces.0.id", workspaceID(tc)).
			AssertJSONPath("data.instance.edition", string(config.BuiltEdition))
	})

	t.Run("shows a member's permissions in the workspace the browser names", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		_, err := join(t, tc, user, enums.RoleMember)
		require.NoError(t, err)

		res := tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspaceID(tc)).GetJSON("/api/v1/me").AssertOK()

		res.AssertJSONPath("data.workspace.id", workspaceID(tc)).AssertJSONPath("data.member.role", "member")
		assert.Contains(t, res.String("data.member.permissions"), "conversations:write")
		assert.NotContains(t, res.String("data.member.permissions"), "settings:write")
		assert.NotContains(t, res.String("data.auth.effective_abilities"), "billing:read")
	})

	t.Run("does not find a workspace the user is not in, or one named some other way", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		stranger := tc.ActingAs(tc.CreateUser())

		for _, header := range []string{workspaceID(tc), tc.Workspace.Workspace.Slug, "ws_nope"} {
			stranger.WithHeader("Dimmi-Workspace", header).GetJSON("/api/v1/me").
				AssertNotFound().
				AssertJSONPath("code", "workspace_not_found")
		}
	})

	t.Run("refuses a workspace where the membership is deactivated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		member, err := join(t, tc, user, enums.RoleAdmin)
		require.NoError(t, err)
		require.NoError(t, factories.Deactivate(t.Context(), tc.App, member))

		tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspaceID(tc)).GetJSON("/api/v1/me").
			AssertForbidden().
			AssertJSONPath("code", "membership_deactivated")
	})

	t.Run("shows a token its own workspace and no list to switch to", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "conversations:read").GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.auth.type", "personal_access_token").
			AssertJSONPath("data.auth.abilities", []string{"conversations:read"}).
			AssertJSONPath("data.auth.effective_abilities", []string{"conversations:read"}).
			AssertJSONPath("data.member.permissions", []string{"conversations:read"}).
			AssertJSONPath("data.workspace.id", workspaceID(tc)).
			AssertJSONPath("data.workspaces", nil).
			AssertJSONMissing("data.auth.auth_methods").
			AssertJSONMissing("data.auth.session_id")
	})

	t.Run("never lets a token do more than its member's role", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		member, err := join(t, tc, user, enums.RoleMember)
		require.NoError(t, err)

		res := tc.ActingAsToken(member, "*").GetJSON("/api/v1/me").AssertOK()

		assert.NotContains(t, res.String("data.member.permissions"), "settings:write")
		assert.NotContains(t, res.String("data.auth.effective_abilities"), "billing:read")
	})

	t.Run("accepts a token of a test workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, plain, err := tc.App.Tokens.Test.Create(t.Context(), token.Token{
			OwnerID: tc.Workspace.Owner.ID, TenantID: tc.Workspace.Workspace.ID.String(), Name: "Sandbox",
			Abilities: []string{"members:read"},
		})
		require.NoError(t, err)

		assert.True(t, strings.HasPrefix(plain, "dmi_test_"))
		tc.Client().WithHeader("Authorization", "Bearer "+plain).GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.member.permissions", []string{"members:read"})
	})

	t.Run("refuses a token naming another workspace", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		other := tests.New(t)

		tc.ActingAsToken(tc.Workspace.Owner, "*").WithHeader("Dimmi-Workspace", workspaceID(other)).
			GetJSON("/api/v1/me").
			AssertForbidden().
			AssertJSONPath("code", "workspace_mismatch")
	})

	t.Run("refuses a mistyped token without looking it up", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		_, plain := tc.MintToken(tc.Workspace.Owner, "*")
		typo := plain[:len(plain)-1] + string(plain[len(plain)-1]^1)

		for _, bad := range []string{typo, plain[:20], "dmi_oat_anything", ""} {
			tc.Client().WithHeader("Authorization", "Bearer "+bad).GetJSON("/api/v1/me").
				AssertUnauthorized().
				AssertJSONPath("code", "invalid_token")
		}
	})

	t.Run("refuses a revoked or expired token, saying which", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		revoked, revokedPlain := tc.MintToken(tc.Workspace.Owner, "*")
		require.NoError(t, tc.App.Tokens.Live.Revoke(t.Context(), revoked.ID))
		_, expiringPlain, err := tc.App.Tokens.Live.Create(t.Context(), token.Token{
			OwnerID: tc.Workspace.Owner.ID, TenantID: tc.Workspace.Workspace.ID.String(), Name: "CI",
			Abilities: []string{"*"}, ExpiresAt: tests.Epoch().Add(time.Hour),
		})
		require.NoError(t, err)
		tc.Clock.Advance(time.Hour)

		tc.Client().WithHeader("Authorization", "Bearer "+revokedPlain).GetJSON("/api/v1/me").
			AssertUnauthorized().AssertJSONPath("code", "token_revoked")
		tc.Client().WithHeader("Authorization", "Bearer "+expiringPlain).GetJSON("/api/v1/me").
			AssertUnauthorized().AssertJSONPath("code", "token_expired")
	})

	t.Run("stops a token the moment its member is deactivated", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		member, err := join(t, tc, user, enums.RoleAdmin)
		require.NoError(t, err)
		client := tc.ActingAsToken(member, "*")
		client.GetJSON("/api/v1/me").AssertOK()

		require.NoError(t, factories.Deactivate(t.Context(), tc.App, member))

		client.GetJSON("/api/v1/me").AssertUnauthorized().AssertJSONPath("code", "invalid_token")
	})

	t.Run("lets the token win when a request carries a cookie too", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		user := tc.CreateUser()
		member, err := join(t, tc, user, enums.RoleMember)
		require.NoError(t, err)
		_, plain := tc.MintToken(member, "*")

		tc.ActingAsOwner().WithHeader("Authorization", "Bearer "+plain).GetJSON("/api/v1/me").AssertOK().
			AssertJSONPath("data.user.email", user.Email)
	})
}

/*
workspaceID returns the public id of the test case's workspace.
*/
func workspaceID(tc *tests.TestCase) string {
	return models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
}

/*
join adds the user to the test case's workspace with the role.
*/
func join(t *testing.T, tc *tests.TestCase, user models.User, role enums.Role) (models.WorkspaceMember, error) {
	t.Helper()

	return factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
}
