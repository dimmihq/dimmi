package widget_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

/*
workspaceHeader names the workspace a dashboard client works in.
*/
const workspaceHeader = "Dimmi-Workspace"

/*
workspaceOf returns the public id of the test case's workspace.
*/
func workspaceOf(tc *tests.TestCase) string {
	return models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID)
}

/*
owner returns the owner's dashboard client inside the workspace.
*/
func owner(tc *tests.TestCase) *testkit.Client {
	return tc.ActingAsOwner().WithHeader(workspaceHeader, workspaceOf(tc))
}

/*
inWorkspace runs a read or write inside the workspace.
*/
func inWorkspace(t *testing.T, tc *tests.TestCase, ws ids.UUID, fn func(ctx context.Context, q *models.Queries) error) {
	t.Helper()

	scoped := tenancy.With(t.Context(), ws.String())
	err := tc.App.DB.Transaction(scoped, func(ctx context.Context, tx *database.Tx) error {
		return fn(ctx, models.New(tx))
	})
	require.NoError(t, err)
}

/*
defaultWidget returns the widget a new workspace comes with.
*/
func defaultWidget(t *testing.T, tc *tests.TestCase) models.WidgetSetting {
	t.Helper()

	var out models.WidgetSetting

	inWorkspace(t, tc, tc.Workspace.Workspace.ID, func(ctx context.Context, q *models.Queries) error {
		rows, err := q.ListWidgetSettings(ctx, tc.Workspace.Workspace.ID)
		if err == nil {
			require.Len(t, rows, 1)
			out = rows[0]
		}

		return err
	})

	return out
}

/*
appID returns the public id of the workspace's first widget.
*/
func appID(t *testing.T, tc *tests.TestCase) string {
	t.Helper()

	return models.PublicID(models.PrefixWidget, defaultWidget(t, tc).ID)
}

/*
settingsPath returns the dashboard's path of the workspace's first widget.
*/
func settingsPath(t *testing.T, tc *tests.TestCase) string {
	t.Helper()

	return "/api/v1/widget-settings/" + appID(t, tc)
}

/*
patch changes the workspace's first widget, failing the test if refused.
*/
func patch(t *testing.T, tc *tests.TestCase, body testkit.JSON) *testkit.Response {
	t.Helper()

	return owner(tc).PatchJSON(settingsPath(t, tc), body).AssertOK()
}

/*
memberClient adds a user with the role and returns them with a client.
*/
func memberClient(t *testing.T, tc *tests.TestCase, role enums.Role) (models.WorkspaceMember, *testkit.Client) {
	t.Helper()

	user := tc.CreateUser()
	member, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return member, tc.ActingAs(user).WithHeader(workspaceHeader, workspaceOf(tc))
}
