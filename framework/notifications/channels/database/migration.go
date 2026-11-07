/*
|--------------------------------------------------------------------------
| Notifications Table
|--------------------------------------------------------------------------
|
| The table's name comes from configuration, so its schema is created by
| a migration step rather than a fixed SQL file. The step only creates
| what is missing, columns added later included, which keeps it safe to
| run on every boot and on tables made by an older release.
|
| Rows are read newest first for one recipient, by the time they last
| changed. A grouped notification keeps one unread row per recipient and
| group, which a unique index enforces.
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
schemaSQL creates the table, its columns and its indexes; {table},
{index}, {recent} and {group} stand for their quoted names.
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
ALTER TABLE {table} ADD COLUMN IF NOT EXISTS tenant_id text;
ALTER TABLE {table} ADD COLUMN IF NOT EXISTS group_key text;
ALTER TABLE {table} ADD COLUMN IF NOT EXISTS count integer NOT NULL DEFAULT 1;
ALTER TABLE {table} ADD COLUMN IF NOT EXISTS last_delivery_id text;
ALTER TABLE {table} ADD COLUMN IF NOT EXISTS seen_at timestamptz;
ALTER TABLE {table} ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS {index} ON {table} (notifiable_type, notifiable_id, created_at DESC);
CREATE INDEX IF NOT EXISTS {recent} ON {table} (notifiable_type, notifiable_id, updated_at DESC, id DESC);
CREATE UNIQUE INDEX IF NOT EXISTS {group} ON {table} (notifiable_type, notifiable_id, group_key)
	WHERE read_at IS NULL AND group_key IS NOT NULL`

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

	name := func(suffix string) string { return pgx.Identifier{table + suffix}.Sanitize() }

	return strings.NewReplacer(
		"{table}", quoted, "{index}", name("_notifiable_index"), "{recent}", name("_recent_index"),
		"{group}", name("_group_key"),
	).Replace(schemaSQL), nil
}
