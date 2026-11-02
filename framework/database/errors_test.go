package database_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
)

func TestErrors(t *testing.T) {
	t.Parallel()

	t.Run("maps a missing row to ErrNotFound", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.QueryRow(t.Context(), "SELECT name FROM items WHERE id = 1").Scan(new(string))
		require.ErrorIs(t, err, database.ErrNotFound)

		err = db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			return tx.QueryRow(ctx, "SELECT name FROM items WHERE id = 1").Scan(new(string))
		})
		require.ErrorIs(t, err, database.ErrNotFound)
	})

	t.Run("detects unique violations by constraint", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		mustExec(t, db, "INSERT INTO items VALUES (1, 'a')")

		_, err := db.Exec(t.Context(), "INSERT INTO items VALUES (2, 'a')")

		assert.True(t, database.IsUniqueViolation(err, "items_name_key"))
		assert.True(t, database.IsUniqueViolation(err, ""))
		assert.False(t, database.IsUniqueViolation(err, "items_pkey"))
		assert.False(t, database.IsSerialization(err))
	})

	t.Run("maps writes on a read-only server to ErrReadOnly", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			mustExecTx(ctx, t, tx, "SET TRANSACTION READ ONLY")
			_, err := tx.Exec(ctx, "INSERT INTO items VALUES (1, 'a')")

			return err
		})

		require.ErrorIs(t, err, database.ErrReadOnly)
		require.NoError(t, db.Ping(t.Context()), "the reset pool reconnects")
	})

	t.Run("detects serialization failures", func(t *testing.T) {
		t.Parallel()

		assert.True(t, database.IsSerialization(&pgconn.PgError{Code: "40001"}))
		assert.False(t, database.IsSerialization(&pgconn.PgError{Code: "23505"}))
		assert.False(t, database.IsUniqueViolation(errStop, ""))
	})
}

func TestAdvisoryLock(t *testing.T) {
	t.Parallel()

	t.Run("holds the lock until the transaction ends", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		outside := t.Context()

		err := db.Transaction(outside, func(ctx context.Context, tx *database.Tx) error {
			require.NoError(t, tx.AdvisoryLock(ctx, "report"))

			assert.False(t, tryLock(outside, t, db, "report"), "taken while held")
			assert.True(t, tryLock(outside, t, db, "other"), "other keys stay free")

			return nil
		})

		require.NoError(t, err)
		assert.True(t, tryLock(t.Context(), t, db, "report"), "free after commit")
	})

	t.Run("reports lock errors", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			mustExecTx(ctx, t, tx, "SET LOCAL lock_timeout = '1ms'")
			_, err := tx.Exec(ctx, "SELECT 1 / 0")
			require.Error(t, err)

			require.ErrorContains(t, tx.AdvisoryLock(ctx, "x"), "advisory lock")
			_, err = tx.TryAdvisoryLock(ctx, "x")
			require.ErrorContains(t, err, "try advisory lock")

			return nil
		})

		require.ErrorContains(t, err, "commit transaction")
	})
}

func tryLock(ctx context.Context, t *testing.T, db *database.DB, key string) bool {
	t.Helper()

	var locked bool

	err := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		var err error
		locked, err = tx.TryAdvisoryLock(ctx, key)

		return err
	})
	require.NoError(t, err)

	return locked
}
