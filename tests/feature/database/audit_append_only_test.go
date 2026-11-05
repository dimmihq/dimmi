package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/tests"
)

func TestAuditLogAppendOnly(t *testing.T) {
	t.Parallel()

	t.Run("the application role cannot change an entry but may delete one", func(t *testing.T) {
		t.Parallel()
		tc := tests.New(t)
		workspace := tc.Workspace.Workspace.ID
		ctx := tenancy.With(t.Context(), workspace.String())
		db := tc.ApplicationRole()
		run := func(sql string) (int64, error) {
			var affected int64
			err := db.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
				tag, err := tx.Exec(ctx, sql, workspace)
				affected = tag.RowsAffected()

				return err
			})

			return affected, err
		}

		_, err := run(`INSERT INTO audit_logs (workspace_id, event, actor_type, actor_label)
			VALUES ($1, 'test.recorded', 'system', 'System')`)
		require.NoError(t, err)

		_, err = run(`UPDATE audit_logs SET actor_label = 'Someone else' WHERE workspace_id = $1`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "audit log entries cannot be changed")

		deleted, err := run(`DELETE FROM audit_logs WHERE workspace_id = $1`)
		require.NoError(t, err)
		assert.Positive(t, deleted)
	})
}
