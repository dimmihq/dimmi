package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
)

func TestTransactionHooks(t *testing.T) {
	t.Parallel()

	t.Run("runs after-commit hooks in order once committed", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		var calls []string

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			mustExecTx(ctx, t, tx, "INSERT INTO items VALUES (1, 'a')")
			tx.AfterCommit(func(ctx context.Context) {
				var n int
				require.NoError(t, db.QueryRow(ctx, "SELECT count(*) FROM items").Scan(&n))
				calls = append(calls, "first sees committed rows")
				assert.Equal(t, 1, n)
			})
			tx.AfterCommit(func(context.Context) { calls = append(calls, "second") })
			tx.AfterRollback(func(context.Context) { calls = append(calls, "rollback") })
			assert.Empty(t, calls, "nothing runs before commit")

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"first sees committed rows", "second"}, calls)
	})

	t.Run("runs after-rollback hooks and drops after-commit hooks on rollback", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		var calls []string

		err := db.Transaction(t.Context(), func(_ context.Context, tx *database.Tx) error {
			tx.AfterCommit(func(context.Context) { calls = append(calls, "commit") })
			tx.AfterRollback(func(context.Context) { calls = append(calls, "rollback") })

			return errStop
		})

		require.ErrorIs(t, err, errStop)
		assert.Equal(t, []string{"rollback"}, calls)
	})

	t.Run("drops a failed savepoint's after-commit hooks", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		var calls []string

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			tx.AfterCommit(func(context.Context) { calls = append(calls, "outer commit") })

			_ = db.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
				tx.AfterCommit(func(context.Context) { calls = append(calls, "inner commit") })
				tx.AfterRollback(func(context.Context) { calls = append(calls, "inner rollback") })

				return errStop
			})
			assert.Equal(t, []string{"inner rollback"}, calls, "savepoint rollback hooks run at once")

			return nil
		})

		require.NoError(t, err)
		assert.Equal(t, []string{"inner rollback", "outer commit"}, calls)
	})

	t.Run("defers a released savepoint's hooks to the outer transaction", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		var calls []string

		err := db.Transaction(t.Context(), func(ctx context.Context, _ *database.Tx) error {
			require.NoError(t, db.Transaction(ctx, func(_ context.Context, tx *database.Tx) error {
				tx.AfterCommit(func(context.Context) { calls = append(calls, "inner commit") })
				tx.AfterRollback(func(context.Context) { calls = append(calls, "inner rollback") })

				return nil
			}))
			assert.Empty(t, calls, "nothing runs at savepoint release")

			return errStop
		})

		require.ErrorIs(t, err, errStop)
		assert.Equal(t, []string{"inner rollback"}, calls, "outer rollback undoes the savepoint too")
	})

	t.Run("runs after-rollback hooks when the commit fails", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		mustExec(t, db, `CREATE TABLE parents (id int PRIMARY KEY);
			CREATE TABLE children (parent_id int REFERENCES parents DEFERRABLE INITIALLY DEFERRED)`)
		var calls []string

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			tx.AfterCommit(func(context.Context) { calls = append(calls, "commit") })
			tx.AfterRollback(func(context.Context) { calls = append(calls, "rollback") })
			_, err := tx.Exec(ctx, "INSERT INTO children VALUES (42)")

			return err
		})

		require.ErrorContains(t, err, "commit transaction")
		assert.Equal(t, []string{"rollback"}, calls)
	})
}
