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

import (
	"strings"

	"github.com/stretchr/testify/require"
)

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

	_, err := tc.operatorDB().Exec(tc.T.Context(), "DROP TABLE "+table+" CASCADE")
	require.NoError(tc.T, err)
}

/*
FailWrites makes every statement of the kinds, such as UPDATE or
DELETE, fail on the table from now on, while reads keep working, the
way a lost write permission or a full disk would. It fires once per
statement, so even a statement that touches no row fails.
*/
func (tc *TestCase) FailWrites(table string, kinds ...string) {
	tc.T.Helper()

	name := "fail_writes_" + table + "_" + strings.ToLower(strings.Join(kinds, "_"))
	for _, statement := range []string{
		"CREATE FUNCTION " + name + "() RETURNS trigger LANGUAGE plpgsql AS " +
			"$$ BEGIN RAISE EXCEPTION 'writes to % fail', TG_TABLE_NAME; END $$",
		"CREATE TRIGGER " + name + " BEFORE " + strings.Join(kinds, " OR ") + " ON " + table +
			" FOR EACH STATEMENT EXECUTE FUNCTION " + name + "()",
	} {
		_, err := tc.operatorDB().Exec(tc.T.Context(), statement)
		require.NoError(tc.T, err)
	}
}
