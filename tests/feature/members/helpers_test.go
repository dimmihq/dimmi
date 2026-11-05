package members_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	appmail "github.com/dimmihq/dimmi/app/mail"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
memberID returns the public id of the member.
*/
func memberID(m models.WorkspaceMember) string {
	return models.PublicID(models.PrefixMember, m.ID)
}

/*
join adds a new user to the test case's workspace with the role and
returns the user and their membership.
*/
func join(t *testing.T, tc *tests.TestCase, role enums.Role, states ...func(models.User) models.User) (
	models.User, models.WorkspaceMember,
) {
	t.Helper()

	user := tc.CreateUser(states...)
	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return user, member
}

/*
protected joins a new user with two-factor authentication on to the
test case's workspace with the role.
*/
func protected(t *testing.T, tc *tests.TestCase, role enums.Role, states ...func(models.User) models.User) (
	models.User, models.WorkspaceMember,
) {
	t.Helper()

	user, member := join(t, tc, role, states...)
	account, err := factories.WithTwoFactor(t.Context(), tc.App, user)
	require.NoError(t, err)

	return account.User, member
}

/*
in returns a client signed in as the user, working in the test case's
workspace.
*/
func in(tc *tests.TestCase, user models.User) *testkit.Client {
	return tc.ActingAs(user).
		WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID))
}

/*
resetPath is the path that mails the member a reset link.
*/
func resetPath(m models.WorkspaceMember) string {
	return "/api/v1/members/" + memberID(m) + "/two-factor-resets"
}

/*
mailReset has the admin mail the member a reset link and returns the
link's id and token, read from the mail.
*/
func mailReset(t *testing.T, tc *tests.TestCase, admin *testkit.Client, m models.WorkspaceMember) (string, string) {
	t.Helper()

	fake := tc.Mail()
	admin.PostJSON(resetPath(m), testkit.JSON{}).AssertCreated()
	sent, _ := tests.LastSent[appmail.TwoFactorResetMail](tc, fake)
	link := tc.Fragment(sent.URL)

	return link.Get("id"), link.Get("token")
}

/*
redeem uses the reset link with the password, if any.
*/
func redeem(client *testkit.Client, id string, body testkit.JSON) *testkit.Response {
	return client.PatchJSON("/api/v1/two-factor-resets/"+id, body)
}
