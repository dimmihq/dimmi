package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
openTenantNotes returns a pool on a table that row-level security limits
to the current tenant, with one note for each of tenants a and b.
*/
func openTenantNotes(t *testing.T) *database.DB {
	t.Helper()

	db := open(t)
	mustExec(t, db, `CREATE TABLE notes (tenant text NOT NULL, body text NOT NULL);
		ALTER TABLE notes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE notes FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation ON notes
			USING (tenant = current_setting('app.tenant_id', true))
			WITH CHECK (tenant = current_setting('app.tenant_id', true))`)

	for _, tenant := range []string{"a", "b"} {
		err := db.Transaction(tenancy.With(t.Context(), tenant), func(ctx context.Context, tx *database.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO notes VALUES ($1, $2)", tenant, "note "+tenant)

			return err
		})
		require.NoError(t, err)
	}

	return db
}

func countNotes(ctx context.Context, t *testing.T, db *database.DB) int {
	t.Helper()

	var n int
	require.NoError(t, db.QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&n))

	return n
}

func TestTenantScoping(t *testing.T) {
	t.Parallel()

	t.Run("limits a transaction and its savepoints to the tenant", func(t *testing.T) {
		t.Parallel()
		db := openTenantNotes(t)

		err := db.Transaction(tenancy.With(t.Context(), "a"), func(ctx context.Context, _ *database.Tx) error {
			assert.Equal(t, 1, countNotes(ctx, t, db))

			return db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
				var body string
				require.NoError(t, tx.QueryRow(ctx, "SELECT body FROM notes").Scan(&body))
				assert.Equal(t, "note a", body)

				return nil
			})
		})

		require.NoError(t, err)
	})

	t.Run("shows tenant rows to no one outside a transaction", func(t *testing.T) {
		t.Parallel()
		db := openTenantNotes(t)

		assert.Zero(t, countNotes(tenancy.With(t.Context(), "a"), t, db))
		assert.Zero(t, countNotes(t.Context(), t, db))
	})

	t.Run("rejects writes for another tenant", func(t *testing.T) {
		t.Parallel()
		db := openTenantNotes(t)

		err := db.Transaction(tenancy.With(t.Context(), "a"), func(ctx context.Context, tx *database.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO notes VALUES ('b', 'sneaky')")

			return err
		})

		require.ErrorContains(t, err, "row-level security")
	})

	t.Run("reports a failure to set the tenant", func(t *testing.T) {
		t.Parallel()
		db, err := database.Open(t.Context(), database.Config{
			URL:           testkit.NewDatabase(t).URL,
			MaxConns:      2,
			PoolMode:      database.PoolModeSession,
			TenantSetting: "not a valid setting name",
		})
		require.NoError(t, err)
		t.Cleanup(db.Close)

		err = db.Transaction(tenancy.With(t.Context(), "a"), func(context.Context, *database.Tx) error { return nil })

		require.ErrorContains(t, err, "set tenant")
	})
}
