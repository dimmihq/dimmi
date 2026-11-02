/*
|--------------------------------------------------------------------------
| Migration Session
|--------------------------------------------------------------------------
|
| goose drives migrations through database/sql, so the runner opens its
| connections through pgx's database/sql adapter and never touches the
| application's pool. One connection holds the session lock for the whole
| run; goose applies the migrations on another. Every statement on them
| carries the lock and statement timeouts.
|
| A server that has to wait for the lock says who holds it, so operators
| can tell a slow migration from a stuck one.
|
*/

package migrate

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"io/fs"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/dimmihq/dimmi/framework/clock"
)

/*
waitReportInterval is how often a waiting server reports the lock
holder after its first report.
*/
const waitReportInterval = 10 * time.Second

/*
holderSQL finds another session holding the session advisory lock with
the given id in the current database.
*/
const holderSQL = `SELECT a.pid, coalesce(a.client_hostname, host(a.client_addr), 'local'), a.application_name
FROM pg_locks l JOIN pg_stat_activity a ON a.pid = l.pid
WHERE l.locktype = 'advisory' AND l.granted AND l.objsubid = 1
	AND l.database = (SELECT oid FROM pg_database WHERE datname = current_database())
	AND ((l.classid::bigint << 32) | l.objid::bigint) = $1
	AND l.pid <> $2
LIMIT 1`

/*
migrationSet is a set ready to run: its name, files and goose provider.
*/
type migrationSet struct {
	name     string
	fsys     fs.FS
	provider *goose.Provider
}

/*
session is an open migration connection. The lock functions act on the
connection that holds the lock, and close releases everything.
*/
type session struct {
	config *pgx.ConnConfig
	lockID int64
	pid    int32
	sets   []migrationSet
	lock   func(ctx context.Context) error
	unlock func(ctx context.Context) error
	close  func() error
}

/*
locked opens a session, takes the migration lock and runs the function.
The lock is released when the function returns; were the release to
fail, closing the connection still ends the session and its lock.
*/
func (r Runner) locked(ctx context.Context, fn func(ctx context.Context, s *session) error) error {
	return r.withSession(ctx, func(ctx context.Context, s *session) error {
		if err := r.acquire(ctx, s); err != nil {
			return err
		}
		defer func() { _ = s.unlock(context.WithoutCancel(ctx)) }()

		return fn(ctx, s)
	})
}

/*
withSession opens a session for the function and closes it afterwards.
*/
func (r Runner) withSession(ctx context.Context, fn func(ctx context.Context, s *session) error) error {
	s, err := r.open(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = s.close() }()

	return fn(ctx, s)
}

/*
open connects, remembers the locking connection's process id so the
wait report can skip it, and builds a provider per set.
*/
func (r Runner) open(ctx context.Context) (*session, error) {
	config, err := pgx.ParseConfig(r.URL)
	if err != nil {
		return nil, fmt.Errorf("parse migration url: %w", err)
	}

	r.limit(config.RuntimeParams)

	db := stdlib.OpenDB(*config.Copy())
	db.SetMaxOpenConns(2)

	conn, err := db.Conn(ctx)
	if err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("connect for migrations: %w", err)
	}

	s := &session{config: config, lockID: lockID(r.LockKey), close: func() error {
		return errors.Join(conn.Close(), db.Close())
	}}

	if err := conn.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&s.pid); err != nil {
		_ = s.close()

		return nil, fmt.Errorf("identify migration session: %w", err)
	}

	locker, err := lock.NewPostgresSessionLocker(lock.WithLockID(s.lockID), lock.WithLockTimeout(1, r.lockAttempts()))
	if err != nil {
		_ = s.close()

		return nil, fmt.Errorf("create migration lock: %w", err)
	}

	s.lock = func(ctx context.Context) error { return locker.SessionLock(ctx, conn) }
	s.unlock = func(ctx context.Context) error { return locker.SessionUnlock(ctx, conn) }

	s.sets, err = r.providers(func(fsys fs.FS, opts ...goose.ProviderOption) (*goose.Provider, error) {
		return goose.NewProvider(goose.DialectPostgres, db, fsys, opts...)
	})
	if err != nil {
		_ = s.close()

		return nil, err
	}

	return s, nil
}

/*
providers builds a goose provider for every set that has migrations.
*/
func (r Runner) providers(
	newProvider func(fsys fs.FS, opts ...goose.ProviderOption) (*goose.Provider, error),
) ([]migrationSet, error) {
	var sets []migrationSet

	for _, set := range r.sets() {
		table := defaultTo(set.Table, set.Name+"_migrations")

		provider, err := newProvider(set.FS, goose.WithTableName(table), goose.WithDisableGlobalRegistry(true))
		if errors.Is(err, goose.ErrNoMigrations) {
			continue
		}

		if err != nil {
			return nil, fmt.Errorf("load %s migrations: %w", set.Name, err)
		}

		sets = append(sets, migrationSet{name: set.Name, fsys: set.FS, provider: provider})
	}

	return sets, nil
}

/*
acquire takes the migration lock, reporting the holder while it waits.
*/
func (r Runner) acquire(ctx context.Context, s *session) error {
	watchCtx, stopWatching := context.WithCancel(ctx)
	watched := make(chan struct{})

	go func() {
		defer close(watched)
		r.reportWaits(watchCtx, s)
	}()

	err := s.lock(ctx)

	stopWatching()
	<-watched

	if err == nil {
		return nil
	}

	if ctx.Err() != nil {
		return fmt.Errorf("wait for migration lock: %w", ctx.Err())
	}

	return fmt.Errorf("%w after %s: %w", ErrLockTimeout, r.LockWait, err)
}

/*
reportWaits tells OnWait who holds the lock, first after one second of
waiting and then every waitReportInterval, until the context ends.
*/
func (r Runner) reportWaits(ctx context.Context, s *session) {
	if r.OnWait == nil {
		return
	}

	wait := time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-clock.System().After(wait):
		}

		wait = waitReportInterval
		if holder, ok := findHolder(ctx, s); ok {
			r.OnWait(ctx, holder)
		}
	}
}

/*
runStep runs the step on a small pool of its own, whose connections
carry the same timeouts as the session's, and closes it afterwards.
*/
func (r Runner) runStep(ctx context.Context, step Step) error {
	config, err := pgxpool.ParseConfig(r.URL)
	if err != nil {
		return fmt.Errorf("parse migration url: %w", err)
	}

	config.MaxConns = 2
	r.limit(config.ConnConfig.RuntimeParams)

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return fmt.Errorf("connect for %s: %w", step.Name, err)
	}
	defer pool.Close()

	if err := step.Up(ctx, pool); err != nil {
		return fmt.Errorf("migrate %s: %w", step.Name, err)
	}

	return nil
}

/*
limit sets the lock and statement timeouts in a connection's runtime
parameters.
*/
func (r Runner) limit(params map[string]string) {
	params["lock_timeout"] = milliseconds(r.LockTimeout)
	params["statement_timeout"] = milliseconds(r.StatementTimeout)
}

/*
findHolder looks up the other session holding the lock on a short-lived
connection. A failed lookup reports nothing; the wait goes on either way.
*/
func findHolder(ctx context.Context, s *session) (Holder, bool) {
	conn, err := pgx.ConnectConfig(ctx, s.config)
	if err != nil {
		return Holder{}, false
	}
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	var holder Holder

	err = conn.QueryRow(ctx, holderSQL, s.lockID, s.pid).Scan(&holder.PID, &holder.Address, &holder.Application)

	return holder, err == nil
}

/*
lockAttempts is how many one-second attempts fit in LockWait.
*/
func (r Runner) lockAttempts() uint64 {
	return uint64(max(1, math.Ceil(r.LockWait.Seconds())))
}

/*
lockID turns the lock key into a non-negative advisory lock id.
*/
func lockID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))

	return int64(h.Sum64() >> 1)
}

func milliseconds(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10)
}
