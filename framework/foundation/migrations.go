/*
|--------------------------------------------------------------------------
| Framework Migrations
|--------------------------------------------------------------------------
|
| Every table the framework owns is created by one migration set with
| one version table, whichever packages the tables come from, and the
| libraries that keep their own schema, such as the queue, migrate as
| steps before it. The application's migrations follow in a set of their
| own. Every server runs the same migrations under one lock, so replicas
| booting together apply each migration exactly once.
|
*/

package foundation

import (
	"cmp"
	"context"
	"embed"
	"fmt"
	"io/fs"
	"slices"
	"time"

	"github.com/dimmihq/dimmi/framework/auth/token"
	"github.com/dimmihq/dimmi/framework/customdomain/storage"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/log"
	notifydb "github.com/dimmihq/dimmi/framework/notifications/channels/database"
	"github.com/dimmihq/dimmi/framework/oauth"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/session"
)

//go:embed *.sql
var ownMigrations embed.FS

/*
defaultNotificationTable is the table of the database notification
channel when its options name none.
*/
const defaultNotificationTable = "notifications"

/*
MigrationsConfig configures migrations. Auto applies pending migrations
at boot. LockTimeout bounds the wait for another server's migration
lock.
*/
type MigrationsConfig struct {
	Auto        bool          `default:"true" koanf:"auto"`
	LockTimeout time.Duration `default:"10m"  koanf:"lock_timeout"`
}

/*
FrameworkMigrations returns every migration of the framework's own
tables in one filesystem: sessions, access tokens, OAuth, certificate
storage and idempotency keys.
*/
func FrameworkMigrations() fs.FS {
	return mergedFS{session.Migrations(), token.Migrations(), oauth.Migrations(), storage.Migrations(), ownMigrations}
}

/*
newMigrator returns the runner for the configuration: the queue's step
and a step per notification table, then the framework set, then the
application's migrations. Rollback is allowed outside production only;
production rolls back through the guarded console command alone.
*/
func newMigrator(cfg *Config, app fs.FS, logger log.Logger) (migrate.Runner, error) {
	url, err := cfg.Database.SessionURL()
	if err != nil {
		return migrate.Runner{}, fmt.Errorf("migrations: %w", err)
	}

	steps := []migrate.Step{queue.Migration()}
	for _, table := range notificationTables(cfg) {
		steps = append(steps, notifydb.Migration(table))
	}

	return migrate.Runner{
		URL:           url,
		Framework:     FrameworkMigrations(),
		App:           app,
		Steps:         steps,
		LockWait:      cfg.Migrations.LockTimeout,
		AllowRollback: cfg.App.Env != EnvProduction,
		OnWait: func(ctx context.Context, holder migrate.Holder) {
			logger.InfoContext(ctx, "waiting for the migration lock",
				log.Int("pid", int(holder.PID)), log.String("address", holder.Address))
		},
	}, nil
}

/*
notificationTables lists the table of every database notification
channel, or the default table when none is configured, so the channel
works as soon as it is added.
*/
func notificationTables(cfg *Config) []string {
	var tables []string

	for _, section := range cfg.Notifications.Channels {
		if section.String("driver") != "database" {
			continue
		}

		var opts notifydb.Config
		if err := section.Decode(&opts); err != nil {
			continue
		}

		tables = append(tables, opts.Table)
	}

	if len(tables) == 0 {
		return []string{defaultNotificationTable}
	}

	slices.Sort(tables)

	return slices.Compact(tables)
}

/*
mergedFS shows several migration directories as one. Open finds a file
in the first directory that has it; Glob and ReadDir list every
directory's matches.
*/
type mergedFS []fs.FS

/*
Open opens the named file from the first directory holding it.
*/
func (m mergedFS) Open(name string) (fs.File, error) {
	for _, fsys := range m {
		file, err := fsys.Open(name)
		if err == nil {
			return file, nil
		}
	}

	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

/*
Glob returns the matching names of every directory, sorted.
*/
func (m mergedFS) Glob(pattern string) ([]string, error) {
	var names []string

	for _, fsys := range m {
		matches, err := fs.Glob(fsys, pattern)
		if err != nil {
			return nil, fmt.Errorf("glob %s: %w", pattern, err)
		}

		names = append(names, matches...)
	}

	slices.Sort(names)

	return names, nil
}

/*
ReadDir returns the entries of the named directory in every directory,
sorted by name.
*/
func (m mergedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	var entries []fs.DirEntry

	for _, fsys := range m {
		found, err := fs.ReadDir(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}

		entries = append(entries, found...)
	}

	slices.SortFunc(entries, func(a, b fs.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })

	return entries, nil
}
