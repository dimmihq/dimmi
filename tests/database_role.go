/*
|--------------------------------------------------------------------------
| Running As The Application Role
|--------------------------------------------------------------------------
|
| In the cloud the application connects as a role that owns nothing, so
| row-level security applies to every query it makes, and only the
| migration task connects as the owner. Setting
| DIMMI_TEST_APPLICATION_ROLE=1 runs every test that way: migrations as
| the owner, the application, its queue and its jobs as a role with
| exactly the cloud role's grants. A job or query that silently sees
| nothing without a workspace fails here rather than in production.
|
*/

package tests

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
applicationRoleSwitch is the variable that turns the application role
on for the whole run.
*/
const applicationRoleSwitch = "DIMMI_TEST_APPLICATION_ROLE"

/*
asApplicationRole reports whether this run connects the application as
the application role.
*/
func asApplicationRole() bool {
	return testkit.Switch(applicationRoleSwitch)
}

/*
applicationURL returns the connection string the application runs with:
the owner's in a normal run, or a new role with the cloud role's grants
when the switch is on. The grants match what provisioning gives the
cloud role through default privileges.
*/
func applicationURL(tb testing.TB, db testkit.Database) string {
	tb.Helper()

	if !asApplicationRole() {
		return db.URL
	}

	role := db.NewRole(tb,
		"SELECT, INSERT, UPDATE, DELETE, MAINTAIN ON ALL TABLES IN SCHEMA public",
		"USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public",
		"EXECUTE ON ALL FUNCTIONS IN SCHEMA public",
	)

	return role.URL
}

/*
operatorDB returns a pool that reads every row, as an operator would:
the application's own pool in a normal run, or one connected as the
owner when the application runs as the application role.
*/
func (tc *TestCase) operatorDB() *database.DB {
	if !asApplicationRole() {
		return tc.App.DB
	}

	tc.operatorOnce.Do(func() {
		db, err := database.Open(tc.T.Context(), database.Config{
			URL: tc.Database.URL, MaxConns: 1, PoolMode: database.PoolModeSession, TenantSetting: "app.tenant_id",
		})
		require.NoError(tc.T, err)
		tc.T.Cleanup(db.Close)
		tc.operator = db
	})

	return tc.operator
}
