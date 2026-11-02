/*
|--------------------------------------------------------------------------
| Failure Injection
|--------------------------------------------------------------------------
|
| Some promises only show when something underneath fails: a sign-in
| that cannot be logged must not start a session, and counters that
| cannot be read must refuse rather than let attempts through. These
| helpers break one piece of the test's own application for the rest of
| the test, the way an outage or a lost permission would, without
| touching any other test.
|
*/

package tests

import "github.com/stretchr/testify/require"

/*
BreakValkey closes the application's Valkey client, so every counter,
lock and cache read fails from now on.
*/
func (tc *TestCase) BreakValkey() {
	tc.App.Redis.Close()
}

/*
BreakTable drops the table from the test's own database, so every
statement that touches it fails from now on.
*/
func (tc *TestCase) BreakTable(table string) {
	tc.T.Helper()

	_, err := tc.App.DB.Exec(tc.T.Context(), "DROP TABLE "+table+" CASCADE")
	require.NoError(tc.T, err)
}
