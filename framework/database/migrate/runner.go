/*
|--------------------------------------------------------------------------
| Migration Runner
|--------------------------------------------------------------------------
|
| Every server boots the same binary and may try to migrate at the same
| moment. One wins a session lock in PostgreSQL and applies what is
| pending; the others wait and then find nothing left to do. Migrations
| run on a direct connection, because a session lock cannot survive a
| transaction pooler.
|
| Steps come first: libraries that ship their own migrator install their
| schema there, so every SQL set may rely on it. The framework's own
| tables come next, then any extra sets, then the application's. Each set
| keeps its own version table, so their version numbers never collide.
|
| A migration that cannot get a table lock quickly gives up instead of
| queueing behind live traffic, and is tried again a few times. Rolling
| back is for development only: production moves forward.
|
*/

package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
Errors the runner returns.

ErrLockTimeout means another server held the migration lock for longer
than LockWait. ErrRollbackDisabled refuses Rollback unless AllowRollback
is set. ErrNothingToRollback means no set has an applied migration.
*/
var (
	ErrLockTimeout       = errors.New("migrate: timed out waiting for the migration lock")
	ErrRollbackDisabled  = errors.New("migrate: rollback is disabled outside development")
	ErrNothingToRollback = errors.New("migrate: no applied migration to roll back")
)

/*
codeLockNotAvailable is the SQLSTATE of a statement that hit lock_timeout.
*/
const codeLockNotAvailable = "55P03"

/*
Set is one directory of goose SQL migrations with its own version table.
Table defaults to the name followed by "_migrations".
*/
type Set struct {
	Name  string
	Table string
	FS    fs.FS
}

/*
Step is schema work a library does with its own migrator, which keeps
its own version table. Up must apply only what is missing, since it
runs on every Up. It receives a pool of its own whose connections carry
the lock and statement timeouts. Steps only move forward: Rollback, Status
and Plan leave them out.
*/
type Step struct {
	Name string
	Up   func(ctx context.Context, pool *pgxpool.Pool) error
}

/*
Holder describes the server session holding the migration lock: its
backend process id, client address and application name.
*/
type Holder struct {
	PID         int32
	Address     string
	Application string
}

/*
Result describes one migration that was applied or rolled back: its
set, version, file name and how long it took.
*/
type Result struct {
	Set      string
	Version  int64
	Name     string
	Duration time.Duration
}

/*
Runner applies migration sets to the database at URL, which must reach
the primary directly rather than through a transaction pooler.

Steps run first. Framework and App are the two built-in sets, applied
in that order with Extra in between; any of them may be nil. LockKey
names the session lock (default "migrate"). LockWait bounds the wait for
it (default ten minutes); OnWait, when set, is told who holds it shortly
after waiting starts and every ten seconds after that.

LockTimeout and StatementTimeout limit every migration statement
(defaults five seconds and fifteen minutes). A migration that hits the
lock timeout is retried up to LockRetries times (default five), waiting
RetryDelay before the first retry (default one second) and twice as long
before each next one. AllowRollback enables Rollback.
*/
type Runner struct {
	URL              string
	Framework        fs.FS
	App              fs.FS
	Extra            []Set
	Steps            []Step
	LockKey          string
	LockWait         time.Duration
	LockTimeout      time.Duration
	StatementTimeout time.Duration
	LockRetries      int
	RetryDelay       time.Duration
	AllowRollback    bool
	OnWait           func(ctx context.Context, holder Holder)
}

/*
Up runs every step and then applies every pending migration, set by set,
under the migration lock. It returns what it applied, including what was
applied before a failure.
*/
func (r Runner) Up(ctx context.Context) ([]Result, error) {
	r = r.withDefaults()

	var results []Result

	err := r.locked(ctx, func(ctx context.Context, s *session) error {
		for _, step := range r.Steps {
			if err := r.runStep(ctx, step); err != nil {
				return err
			}
		}

		for _, set := range s.sets {
			applied, err := r.upSet(ctx, set)
			results = append(results, applied...)

			if err != nil {
				return err
			}
		}

		return nil
	})

	return results, err
}

/*
Rollback undoes the newest applied migration of the last set that has
one, under the migration lock. It returns ErrRollbackDisabled unless
AllowRollback is set.
*/
func (r Runner) Rollback(ctx context.Context) (Result, error) {
	if !r.AllowRollback {
		return Result{}, ErrRollbackDisabled
	}

	r = r.withDefaults()

	var result Result

	err := r.locked(ctx, func(ctx context.Context, s *session) error {
		for _, set := range slices.Backward(s.sets) {
			version, err := set.provider.GetDBVersion(ctx)
			if err != nil {
				return fmt.Errorf("read version of %s: %w", set.name, err)
			}

			if version == 0 {
				continue
			}

			undone, err := set.provider.Down(ctx)
			if err != nil {
				return fmt.Errorf("roll back %s: %w", set.name, err)
			}

			result = resultOf(set.name, undone)

			return nil
		}

		return ErrNothingToRollback
	})

	return result, err
}

func (r Runner) withDefaults() Runner {
	r.LockKey = defaultTo(r.LockKey, "migrate")
	r.LockWait = defaultTo(r.LockWait, 10*time.Minute)
	r.LockTimeout = defaultTo(r.LockTimeout, 5*time.Second)
	r.StatementTimeout = defaultTo(r.StatementTimeout, 15*time.Minute)
	r.LockRetries = defaultTo(r.LockRetries, 5)
	r.RetryDelay = defaultTo(r.RetryDelay, time.Second)

	return r
}

func defaultTo[T comparable](value, fallback T) T {
	var zero T
	if value == zero {
		return fallback
	}

	return value
}

/*
sets lists the configured sets in the order they apply, leaving out
those without a filesystem.
*/
func (r Runner) sets() []Set {
	all := slices.Concat(
		[]Set{{Name: "framework", FS: r.Framework}},
		r.Extra,
		[]Set{{Name: "app", FS: r.App}},
	)

	return slices.DeleteFunc(all, func(s Set) bool { return s.FS == nil })
}

/*
upSet applies the set's pending migrations, retrying when one hits the
lock timeout. Each retry resumes after the last migration that applied.
*/
func (r Runner) upSet(ctx context.Context, set migrationSet) ([]Result, error) {
	var results []Result

	delay := r.RetryDelay
	for attempt := 0; ; attempt++ {
		applied, err := set.provider.Up(ctx)
		results = append(results, resultsOf(set.name, applied, err)...)

		if err == nil {
			return results, nil
		}

		if !isLockTimeout(err) || attempt >= r.LockRetries {
			return results, fmt.Errorf("migrate %s: %w", set.name, err)
		}

		if err := sleep(ctx, delay); err != nil {
			return results, err
		}

		delay *= 2
	}
}

func isLockTimeout(err error) bool {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)

	return ok && pgErr.Code == codeLockNotAvailable
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("wait to retry migration: %w", ctx.Err())
	case <-clock.System().After(d):
		return nil
	}
}

/*
resultsOf converts what goose applied, including the migrations applied
before a partial failure.
*/
func resultsOf(set string, applied []*goose.MigrationResult, err error) []Result {
	if partial, ok := errors.AsType[*goose.PartialError](err); ok {
		applied = partial.Applied
	}

	results := make([]Result, 0, len(applied))
	for _, m := range applied {
		results = append(results, resultOf(set, m))
	}

	return results
}

func resultOf(set string, m *goose.MigrationResult) Result {
	return Result{Set: set, Version: m.Source.Version, Name: path.Base(m.Source.Path), Duration: m.Duration}
}
