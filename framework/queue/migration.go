/*
|--------------------------------------------------------------------------
| Queue Tables
|--------------------------------------------------------------------------
|
| River owns the tables jobs live in and ships its own migrations, with a
| version table of its own. They run as a step of the migration runner,
| under its lock, so servers booting together never race to create them
| and every SQL migration may rely on them being there.
|
*/

package queue

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"github.com/dimmihq/dimmi/framework/database/migrate"
)

/*
Migration returns the migration step that creates and upgrades the
queue's tables.
*/
func Migration() migrate.Step {
	return migrate.Step{Name: "queue", Up: migrateUp}
}

/*
migrateUp applies River's pending migrations, each in a transaction of
its own.
*/
func migrateUp(ctx context.Context, pool *pgxpool.Pool) error {
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("create queue migrator: %w", err)
	}

	if _, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("apply queue migrations: %w", err)
	}

	return nil
}
