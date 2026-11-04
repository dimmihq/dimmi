//go:build dev

package foundation_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
existing reports which of the names exist on the server as a database
or a role.
*/
func existing(t *testing.T, admin *pgx.Conn, names ...string) []string {
	t.Helper()

	var found []string
	require.NoError(t, admin.QueryRow(t.Context(), `
		SELECT coalesce(array_agg(name ORDER BY name), '{}') FROM (
			SELECT datname::text AS name FROM pg_database WHERE datname = ANY ($1)
			UNION ALL
			SELECT rolname::text FROM pg_roles WHERE rolname = ANY ($1)
		) AS found`, names).Scan(&found))

	return found
}

func TestTestSweep(t *testing.T) {
	t.Parallel()

	t.Run("drops roles killed runs left and no database a run going on may still use", func(t *testing.T) {
		t.Parallel()
		server := testkit.Env(t, "DATABASE_URL")
		admin, err := pgx.Connect(t.Context(), server)
		require.NoError(t, err)
		t.Cleanup(func() { _ = admin.Close(context.WithoutCancel(t.Context())) })

		leftover, unrelated := testkit.UniqueName("app_"), testkit.UniqueName("keep_")
		for _, role := range []string{leftover, unrelated} {
			_, err := admin.Exec(t.Context(), "CREATE ROLE "+role)
			require.NoError(t, err)
		}

		t.Cleanup(func() { _, _ = admin.Exec(context.WithoutCancel(t.Context()), "DROP ROLE IF EXISTS "+unrelated) })
		db := testkit.NewDatabase(t)

		code, stdout, stderr := run(t, options(environOn(t, server)), "", "test:sweep", "--minutes", "10000000")

		require.Equal(t, console.ExitSuccess, code, stderr)
		assert.Contains(t, stdout, leftover)
		assert.NotContains(t, stdout, db.Name)
		assert.Equal(t, []string{unrelated, db.Name, db.Name}, existing(t, admin, leftover, unrelated, db.Name))
	})

	t.Run("reports a database server it cannot reach", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, options(environOn(t, "postgres://127.0.0.1:1/none")), "", "test:sweep")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "connect to the database server")
	})
}
