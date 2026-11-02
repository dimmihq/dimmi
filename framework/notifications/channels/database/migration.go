/*
|--------------------------------------------------------------------------
| Notifications Table
|--------------------------------------------------------------------------
|
| The table's name comes from configuration, so its schema is created by
| a migration step rather than a fixed SQL file. The step only creates
| what is missing, which keeps it safe to run on every boot.
|
| Rows are read newest first for one recipient, which is the one index
| the table needs.
|
*/

package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dimmihq/dimmi/framework/database/migrate"
)

/*
schemaSQL creates the table and its index; {table} and {index} stand
for their quoted names.
*/
const schemaSQL = `CREATE TABLE IF NOT EXISTS {table} (
	id uuid PRIMARY KEY,
	notifiable_type text NOT NULL,
	notifiable_id text NOT NULL,
	type text NOT NULL,
	data jsonb NOT NULL DEFAULT '{}',
	read_at timestamptz,
	created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS {index} ON {table} (notifiable_type, notifiable_id, created_at DESC)`

/*
Migration returns the migration step that creates the notifications
table under the given name, for the migration runner.
*/
func Migration(table string) migrate.Step {
	return migrate.Step{
		Name: "notifications",
		Up: func(ctx context.Context, pool *pgxpool.Pool) error {
			sql, err := schema(table)
			if err != nil {
				return err
			}

			if _, err := pool.Exec(ctx, sql); err != nil {
				return fmt.Errorf("create notifications table %s: %w", table, err)
			}

			return nil
		},
	}
}

/*
schema returns the SQL that creates the named table.
*/
func schema(table string) (string, error) {
	quoted, err := quote(table)
	if err != nil {
		return "", err
	}

	index := pgx.Identifier{table + "_notifiable_index"}.Sanitize()

	return strings.NewReplacer("{table}", quoted, "{index}", index).Replace(schemaSQL), nil
}
