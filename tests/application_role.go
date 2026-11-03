/*
|--------------------------------------------------------------------------
| Application Role
|--------------------------------------------------------------------------
|
| The test's application connects as the owner of its tables, and row
| level security never applies to a table's owner. In the cloud the
| application connects as a role that owns nothing, so every tenant
| policy filters what it reads and writes. Queries whose isolation
| matters run through this role in their tests, so a missing workspace
| filter or a broken policy fails there and not in production.
|
*/

package tests

import (
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
)

/*
ApplicationRole returns a pool connected to the test's database as a
role that owns nothing and may read and write every table and
sequence, the way the cloud's application role does, so every tenant
policy applies to it. The pool is made once per test case; it closes
and the role is dropped when the test ends.
*/
func (tc *TestCase) ApplicationRole() *database.DB {
	tc.T.Helper()

	tc.roleOnce.Do(func() { tc.role = tc.openApplicationRole() })

	return tc.role
}

/*
openApplicationRole creates the role, grants it the tables and opens a
pool as it.
*/
func (tc *TestCase) openApplicationRole() *database.DB {
	tc.T.Helper()

	ctx := tc.T.Context()
	role := tc.Database.NewRole(tc.T)

	admin, err := database.Open(ctx, database.Config{
		URL: tc.Database.AdminURL, MaxConns: 1, PoolMode: database.PoolModeSession, TenantSetting: "app.tenant_id",
	})
	require.NoError(tc.T, err)

	defer admin.Close()

	for _, statement := range []string{
		"GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO " + role.Name,
		"GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO " + role.Name,
	} {
		_, err := admin.Exec(ctx, statement)
		require.NoError(tc.T, err)
	}

	db, err := database.Open(ctx, database.Config{
		URL: role.URL, MaxConns: 2, PoolMode: database.PoolModeSession, TenantSetting: "app.tenant_id",
	})
	require.NoError(tc.T, err)
	tc.T.Cleanup(db.Close)

	return db
}
