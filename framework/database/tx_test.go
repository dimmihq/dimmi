package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
)

var errStop = errors.New("stop")

func TestTransaction(t *testing.T) {
	t.Parallel()

	t.Run("commits when the function succeeds", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			_, err := tx.Exec(ctx, "INSERT INTO items VALUES (1, 'a')")

			return err
		})

		require.NoError(t, err)
		assert.Equal(t, 1, countItems(t.Context(), t, db))
	})

	t.Run("rolls back and returns the function's error", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			mustExecTx(ctx, t, tx, "INSERT INTO items VALUES (1, 'a')")

			return errStop
		})

		require.ErrorIs(t, err, errStop)
		assert.Equal(t, 0, countItems(t.Context(), t, db))
	})

	t.Run("rolls back and re-panics when the function panics", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		assert.PanicsWithValue(t, "boom", func() {
			_ = db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
				mustExecTx(ctx, t, tx, "INSERT INTO items VALUES (1, 'a')")
				panic("boom")
			})
		})
		assert.Equal(t, 0, countItems(t.Context(), t, db))
	})

	t.Run("routes db calls made with its context into the transaction", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		outside := t.Context()

		err := db.Transaction(outside, func(ctx context.Context, _ *database.Tx) error {
			_, err := db.Exec(ctx, "INSERT INTO items VALUES (1, 'a')")
			require.NoError(t, err)

			var inside int
			require.NoError(t, db.QueryRow(ctx, "SELECT count(*) FROM items").Scan(&inside))
			assert.Equal(t, 1, inside, "visible inside the transaction")
			assert.Equal(t, 0, countItems(outside, t, db), "invisible outside before commit")

			rows, err := db.Query(ctx, "SELECT id FROM items")
			require.NoError(t, err)
			rows.Close()

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 1, countItems(t.Context(), t, db))
	})

	t.Run("rolls back a failed savepoint and keeps the outer work", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			mustExecTx(ctx, t, tx, "INSERT INTO items VALUES (1, 'a')")

			nested := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
				mustExecTx(ctx, t, tx, "INSERT INTO items VALUES (2, 'b')")

				return errStop
			})
			require.ErrorIs(t, nested, errStop)

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, 1, countItems(t.Context(), t, db))
	})

	t.Run("commits a savepoint with the outer transaction", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, _ *database.Tx) error {
			return db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
				mustExecTx(ctx, t, tx, "INSERT INTO items VALUES (1, 'a')")

				return nil
			})
		})

		require.NoError(t, err)
		assert.Equal(t, 1, countItems(t.Context(), t, db))
	})

	t.Run("reports a failed begin", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := db.Transaction(ctx, func(context.Context, *database.Tx) error { return nil })

		require.ErrorIs(t, err, context.Canceled)
	})
}

func mustExecTx(ctx context.Context, t *testing.T, tx *database.Tx, sql string) {
	t.Helper()

	_, err := tx.Exec(ctx, sql)
	require.NoError(t, err)
}
