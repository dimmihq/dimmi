package accesspolicy_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/tests"
)

const policyPath = "/api/v1/workspace/access-policy"

/*
proxyEnv makes the application believe the address the test client says
it forwards, the way one behind a trusted proxy would.
*/
func proxyEnv() tests.Option {
	return tests.WithEnv("DIMMI_HTTP__TRUSTED_PROXIES=192.0.2.1/32")
}

/*
from returns a copy of the client seen as coming from the address.
*/
func from(c *testkit.Client, address string) *testkit.Client {
	return c.WithHeader("X-Forwarded-For", address)
}

/*
in returns a browser of the user working in the test case's workspace.
*/
func in(tc *tests.TestCase, user models.User) *testkit.Client {
	return working(tc, tc.ActingAs(user))
}

/*
working returns the client sending the test case's workspace as the one
in play.
*/
func working(tc *tests.TestCase, c *testkit.Client) *testkit.Client {
	return c.WithHeader("Dimmi-Workspace", models.PublicID(models.PrefixWorkspace, tc.Workspace.Workspace.ID))
}

/*
join adds a person to the test case's workspace with the role.
*/
func join(t *testing.T, tc *tests.TestCase, role enums.Role) models.User {
	t.Helper()

	user := tc.CreateUser()
	_, err := factories.Member(t.Context(), tc.App, factories.Membership{
		Workspace: tc.Workspace.Workspace, User: user, Role: role,
	})
	require.NoError(t, err)

	return user
}

/*
ruled writes rules straight into the test case's workspace, as a past
change would have left them.
*/
func ruled(t *testing.T, tc *tests.TestCase, rules map[string]any) {
	t.Helper()

	raw, err := json.Marshal(rules)
	require.NoError(t, err)

	_, err = tc.Operator().Exec(t.Context(),
		`UPDATE workspaces SET settings = settings ||
			jsonb_build_object('security', COALESCE(settings->'security', '{}'::jsonb) || $2::jsonb)
		 WHERE id = $1`, tc.Workspace.Workspace.ID, string(raw))
	require.NoError(t, err)
}

/*
oneEntryAllowlist is a one-entry allowlist.
*/
func oneEntryAllowlist(cidr string) []map[string]string {
	return []map[string]string{{"cidr": cidr, "label": "Office"}}
}
