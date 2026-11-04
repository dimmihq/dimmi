package roles_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const rolesPath = "/api/v1/roles"

/*
in returns a browser of the user working in the test case's workspace.
*/
func in(tc *tests.TestCase, user models.User) *testkit.Client {
	workspace := models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)

	return tc.ActingAs(user).WithHeader("Dimmi-Workspace", workspace)
}

/*
join adds a person to the test case's workspace with the role.
*/
func join(t *testing.T, tc *tests.TestCase, role enums.Role) (models.User, models.WorkspaceMember) {
	t.Helper()

	user := tc.CreateUser()
	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return user, member
}

/*
role creates a custom role in the test case's workspace.
*/
func role(t *testing.T, tc *tests.TestCase, base enums.Role, permissions ...string) models.Role {
	t.Helper()

	created, err := factories.Role(t.Context(), tc.App, factories.RoleOf{
		Workspace: tc.Workspace.Workspace, Name: testkit.UniqueName("role"), BaseRole: base,
		Permissions: append([]string{"members:read"}, permissions...),
	})
	require.NoError(t, err)

	return created
}

/*
holding gives the member the custom role.
*/
func holding(t *testing.T, tc *tests.TestCase, m models.WorkspaceMember, r models.Role) models.WorkspaceMember {
	t.Helper()

	member, err := factories.WithCustomRole(t.Context(), tc.App, m, r)
	require.NoError(t, err)

	return member
}

/*
roleID returns the public id of the role.
*/
func roleID(r models.Role) string {
	return models.PublicID(models.PrefixRole, r.ID)
}

/*
memberID returns the public id of the member.
*/
func memberID(m models.WorkspaceMember) string {
	return models.PublicID(models.PrefixMember, m.ID)
}

/*
memberPath returns the path of the member.
*/
func memberPath(m models.WorkspaceMember) string {
	return "/api/v1/members/" + memberID(m)
}
