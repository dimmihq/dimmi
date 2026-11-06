//go:build dev

/*
|--------------------------------------------------------------------------
| Test Leftovers Sweep
|--------------------------------------------------------------------------
|
| Tests make a database and a role of their own on the development
| server and drop both when they end, but a run killed half way, by a
| timeout or by hand, never gets to. This command clears what such runs
| left: databases and roles named the way the test kit names them, and
| nothing else, so the server's real databases and roles are never at
| risk.
|
| A database is only dropped once nobody is connected to it and it is
| older than the given age, so a test run going on at the same time
| keeps its databases. A role is only dropped once it owns no database,
| nobody is signed in as it and it holds no privilege anywhere, which the
| server itself checks.
|
*/

package foundation

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/dimmihq/dimmi/framework/console"
)

/*
Patterns of the names the test kit gives the databases and roles it
creates: a test's own database and role, a migrated template and its
role, and the application roles older tests made.
*/
const (
	sweptDatabases = `^(t_[a-z2-7]{10}|tpl_[0-9a-f]{16})$`
	sweptRoles     = `^(t_[a-z2-7]{10}|tpl_[0-9a-f]{16}|app_[a-z2-7]{10})$`
)

/*
staleDatabases lists the test databases nobody is connected to that
were created more than the given minutes ago. A database's version file
is written when the database is created, so its time is the database's
age. A database another run drops while the list is read has no file
left, and is skipped rather than failing the list.
*/
const staleDatabases = `
SELECT d.datname::text FROM pg_database AS d
WHERE d.datname ~ $1
  AND NOT EXISTS (SELECT 1 FROM pg_stat_activity AS a WHERE a.datid = d.oid)
  AND (pg_stat_file('base/' || d.oid || '/PG_VERSION', true)).modification < now() - make_interval(mins => $2)
ORDER BY 1`

/*
idleRoles lists the test roles that own no database and that nobody is
signed in as.
*/
const idleRoles = `
SELECT r.rolname::text FROM pg_roles AS r
WHERE r.rolname ~ $1
  AND NOT EXISTS (SELECT 1 FROM pg_database AS d WHERE d.datdba = r.oid)
  AND NOT EXISTS (SELECT 1 FROM pg_stat_activity AS a WHERE a.usesysid = r.oid)
ORDER BY 1`

/*
testSweepCommand drops the databases and roles test runs left behind on
the configured database's server. The configured user must be allowed
to drop them, as the development superuser is.
*/
func (k *kernel) testSweepCommand() console.Command {
	return command{
		name:        "test:sweep",
		description: "Drop the databases and roles killed test runs left on the database server",
		flags: func(flags *console.Flags) {
			flags.Int("minutes", 60, "only drop databases created at least this many minutes ago")
		},
		handle: func(ctx context.Context, in *console.Input, out *console.Output) error {
			cfg, err := k.config(ctx)
			if err != nil {
				return err
			}

			serverURL, err := cfg.Database.SessionURL()
			if err != nil {
				return err
			}

			conn, err := pgx.Connect(ctx, serverURL)
			if err != nil {
				return fmt.Errorf("connect to the database server: %w", err)
			}

			defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

			databases, err := sweep(ctx, conn, leftovers{
				query: staleDatabases, args: []any{sweptDatabases, in.Int("minutes")}, drop: dropDatabase,
			})
			if err != nil {
				return err
			}

			roles, err := sweep(ctx, conn, leftovers{query: idleRoles, args: []any{sweptRoles}, drop: dropRole})
			if err != nil {
				return err
			}

			out.Info("Dropped %d databases and %d roles.", len(databases), len(roles))

			for _, name := range append(databases, roles...) {
				out.Line("  %s", name)
			}

			return nil
		},
	}
}

/*
leftovers is one kind of thing to sweep: the query that lists their
names, with its arguments, and the statements that drop one.
*/
type leftovers struct {
	query string
	args  []any
	drop  func(name string) []string
}

/*
sweep lists the leftovers and drops each, returning the names it
dropped. One the server refuses to drop, because it is in use or still
holds something, is left for a later sweep.
*/
func sweep(ctx context.Context, conn *pgx.Conn, l leftovers) ([]string, error) {
	rows, err := conn.Query(ctx, l.query, l.args...)
	if err != nil {
		return nil, fmt.Errorf("list test leftovers: %w", err)
	}

	names, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("list test leftovers: %w", err)
	}

	dropped := make([]string, 0, len(names))

	for _, name := range names {
		if dropAll(ctx, conn, l.drop(name)) {
			dropped = append(dropped, name)
		}
	}

	return dropped, nil
}

/*
dropAll runs the statements in order and reports whether all of them
ran.
*/
func dropAll(ctx context.Context, conn *pgx.Conn, statements []string) bool {
	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement); err != nil {
			return false
		}
	}

	return true
}

/*
dropDatabase returns the statements that drop a test database. A
template has to stop being one before it can be dropped.
*/
func dropDatabase(name string) []string {
	quoted := pgx.Identifier{name}.Sanitize()
	statements := []string{"DROP DATABASE " + quoted}

	if strings.HasPrefix(name, "tpl_") {
		statements = append([]string{"ALTER DATABASE " + quoted + " IS_TEMPLATE false"}, statements...)
	}

	return statements
}

/*
dropRole returns the statement that drops a test role.
*/
func dropRole(name string) []string {
	return []string{"DROP ROLE " + pgx.Identifier{name}.Sanitize()}
}
