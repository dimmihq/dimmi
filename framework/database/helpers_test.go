package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
open returns a pool on a fresh database with an items table.
*/
func open(t *testing.T) *database.DB {
	t.Helper()

	db := openURL(t, testkit.NewDatabase(t).URL, database.PoolModeSession)
	mustExec(t, db, "CREATE TABLE items (id int PRIMARY KEY, name text NOT NULL CONSTRAINT items_name_key UNIQUE)")

	return db
}

func openURL(t *testing.T, url string, mode database.PoolMode) *database.DB {
	t.Helper()

	db, err := database.Open(t.Context(), database.Config{
		URL:           url,
		MaxConns:      4,
		PoolMode:      mode,
		TenantSetting: "app.tenant_id",
	})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return db
}

func mustExec(t *testing.T, db *database.DB, sql string) {
	t.Helper()

	_, err := db.Exec(t.Context(), sql)
	require.NoError(t, err)
}

/*
countItems counts committed rows from a context outside any transaction
under test.
*/
func countItems(ctx context.Context, t *testing.T, db *database.DB) int {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRow(ctx, "SELECT count(*) FROM items").Scan(&n))

	return n
}

/*
queryError runs a query that is expected to fail and returns its error,
closing the rows if it unexpectedly succeeds.
*/
func queryError(ctx context.Context, q database.Querier, sql string) error {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return err
	}
	defer rows.Close()

	return nil
}
