package migrate_test

import (
	"context"
	"errors"
	"hash/fnv"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
migration returns a goose SQL file with the given up and down sections
and an expand header.
*/
func migration(up, down string) *fstest.MapFile {
	text := "-- migrate: phase=expand\n-- +goose Up\n" + up + "\n-- +goose Down\n" + down + "\n"

	return &fstest.MapFile{Data: []byte(text)}
}

/*
connect opens a plain connection to the test database, for setting up
and inspecting what the runner did.
*/
func connect(t *testing.T, url string) *pgx.Conn {
	t.Helper()

	conn, err := pgx.Connect(t.Context(), url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.WithoutCancel(t.Context())) })

	return conn
}

func exec(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()

	_, err := conn.Exec(t.Context(), sql, args...)
	require.NoError(t, err)
}

func tableExists(t *testing.T, conn *pgx.Conn, name string) bool {
	t.Helper()

	var exists bool
	require.NoError(t, conn.QueryRow(t.Context(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&exists))

	return exists
}

/*
lockID mirrors how the runner derives its advisory lock id from the key,
so a test can hold the lock the way another server would.
*/
func lockID(key string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))

	return int64(h.Sum64() >> 1)
}

func names(results []migrate.Result) []string {
	out := make([]string, 0, len(results))
	for _, r := range results {
		out = append(out, r.Set+"/"+r.Name)
	}

	return out
}

func TestUp(t *testing.T) {
	t.Parallel()

	t.Run("applies the framework, extra and app sets in order", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		runner := migrate.Runner{
			URL: url,
			Framework: fstest.MapFS{
				"001_users.sql": migration("CREATE TABLE users (id int PRIMARY KEY);", "DROP TABLE users;"),
			},
			Extra: []migrate.Set{{Name: "jobs", FS: fstest.MapFS{
				"001_jobs.sql": migration("CREATE TABLE jobs (id int);", "DROP TABLE jobs;"),
			}}},
			App: fstest.MapFS{
				"001_posts.sql": migration("CREATE TABLE posts (user_id int REFERENCES users);", "DROP TABLE posts;"),
				"002_tags.sql":  migration("CREATE TABLE tags (id int);", "DROP TABLE tags;"),
			},
		}

		results, err := runner.Up(t.Context())

		require.NoError(t, err)
		assert.Equal(t, []string{
			"framework/001_users.sql", "jobs/001_jobs.sql", "app/001_posts.sql", "app/002_tags.sql",
		}, names(results))
		conn := connect(t, url)
		for _, table := range []string{"framework_migrations", "jobs_migrations", "app_migrations", "tags"} {
			assert.True(t, tableExists(t, conn, table), table)
		}

		again, err := runner.Up(t.Context())
		require.NoError(t, err)
		assert.Empty(t, again, "nothing left to apply")
	})

	t.Run("applies a migration older than the newest one already applied", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		newer := migration("CREATE TABLE newer (id int);", "DROP TABLE newer;")
		first := migrate.Runner{URL: url, App: fstest.MapFS{"002_newer.sql": newer}}
		_, err := first.Up(t.Context())
		require.NoError(t, err)

		results, err := migrate.Runner{URL: url, App: fstest.MapFS{
			"001_older.sql": migration("CREATE TABLE older (id int);", "DROP TABLE older;"),
			"002_newer.sql": newer,
		}}.Up(t.Context())

		require.NoError(t, err)
		assert.Equal(t, []string{"app/001_older.sql"}, names(results))
		assert.True(t, tableExists(t, connect(t, url), "older"))
	})

	t.Run("runs steps under the lock before the sets", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		step := migrate.Step{Name: "library", Up: func(ctx context.Context, pool *pgxpool.Pool) error {
			var timeout string
			if err := pool.QueryRow(ctx, "SHOW lock_timeout").Scan(&timeout); err != nil {
				return err
			}

			_, err := pool.Exec(ctx, "CREATE TABLE IF NOT EXISTS library_items (id int PRIMARY KEY, timeout text)")
			if err != nil {
				return err
			}

			_, err = pool.Exec(ctx, "INSERT INTO library_items VALUES (1, $1) ON CONFLICT DO NOTHING", timeout)

			return err
		}}
		runner := migrate.Runner{
			URL:   url,
			Steps: []migrate.Step{step},
			App: fstest.MapFS{
				"001_uses.sql": migration("CREATE TABLE uses (item_id int REFERENCES library_items);", ""),
			},
			LockTimeout: 2 * time.Second,
		}

		results, err := runner.Up(t.Context())
		require.NoError(t, err)
		assert.Equal(t, []string{"app/001_uses.sql"}, names(results))

		_, err = runner.Up(t.Context())
		require.NoError(t, err)

		var timeout string
		require.NoError(t, connect(t, url).QueryRow(t.Context(), "SELECT timeout FROM library_items").Scan(&timeout))
		assert.Equal(t, "2s", timeout)
	})

	t.Run("stops at a failing step", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		broken := errors.New("broken")
		runner := migrate.Runner{
			URL: url,
			Steps: []migrate.Step{{Name: "library", Up: func(context.Context, *pgxpool.Pool) error {
				return broken
			}}},
			App: fstest.MapFS{"001_a.sql": migration("CREATE TABLE a (id int);", "")},
		}

		results, err := runner.Up(t.Context())

		require.ErrorIs(t, err, broken)
		require.ErrorContains(t, err, "migrate library")
		assert.Empty(t, results)
		assert.False(t, tableExists(t, connect(t, url), "a"))
	})

	t.Run("skips sets without migrations", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		runner := migrate.Runner{
			URL:       url,
			Framework: fstest.MapFS{},
			Extra: []migrate.Set{{
				Name: "custom", Table: "custom_versions", FS: fstest.MapFS{"001_a.sql": migration("SELECT 1;", "")},
			}},
		}

		results, err := runner.Up(t.Context())

		require.NoError(t, err)
		assert.Equal(t, []string{"custom/001_a.sql"}, names(results))
		conn := connect(t, url)
		assert.True(t, tableExists(t, conn, "custom_versions"))
		assert.False(t, tableExists(t, conn, "framework_migrations"))
	})

	t.Run("returns what applied before a failing migration", func(t *testing.T) {
		t.Parallel()
		runner := migrate.Runner{URL: testkit.NewDatabase(t).URL, App: fstest.MapFS{
			"001_ok.sql":     migration("CREATE TABLE ok (id int);", ""),
			"002_broken.sql": migration("CREATE TABLE broken (;", ""),
		}}

		results, err := runner.Up(t.Context())

		require.ErrorContains(t, err, "migrate app")
		assert.Equal(t, []string{"app/001_ok.sql"}, names(results))
	})

	t.Run("retries a migration that waits too long for a table lock", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		blocker := connect(t, url)
		exec(t, blocker, "CREATE TABLE busy (id int)")
		exec(t, blocker, "BEGIN; LOCK TABLE busy IN ACCESS EXCLUSIVE MODE")

		released := make(chan struct{})
		go func() {
			defer close(released)
			clock.System().Sleep(300 * time.Millisecond)
			_, _ = blocker.Exec(context.WithoutCancel(t.Context()), "COMMIT")
		}()

		runner := migrate.Runner{
			URL:         url,
			App:         fstest.MapFS{"001_busy.sql": migration("ALTER TABLE busy ADD COLUMN name text;", "")},
			LockTimeout: 50 * time.Millisecond,
			RetryDelay:  50 * time.Millisecond,
		}

		results, err := runner.Up(t.Context())
		<-released

		require.NoError(t, err)
		assert.Equal(t, []string{"app/001_busy.sql"}, names(results))
	})

	t.Run("gives up after the lock retries", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		blocker := connect(t, url)
		exec(t, blocker, "CREATE TABLE busy (id int)")
		exec(t, blocker, "BEGIN; LOCK TABLE busy IN ACCESS EXCLUSIVE MODE")

		runner := migrate.Runner{
			URL:         url,
			App:         fstest.MapFS{"001_busy.sql": migration("ALTER TABLE busy ADD COLUMN name text;", "")},
			LockTimeout: 20 * time.Millisecond,
			LockRetries: 2,
			RetryDelay:  10 * time.Millisecond,
		}

		_, err := runner.Up(t.Context())

		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		assert.Equal(t, "55P03", pgErr.Code)
	})

	t.Run("stops retrying when the context ends", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		blocker := connect(t, url)
		exec(t, blocker, "CREATE TABLE busy (id int)")
		exec(t, blocker, "BEGIN; LOCK TABLE busy IN ACCESS EXCLUSIVE MODE")
		ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()

		runner := migrate.Runner{
			URL:         url,
			App:         fstest.MapFS{"001_busy.sql": migration("ALTER TABLE busy ADD COLUMN name text;", "")},
			LockTimeout: 20 * time.Millisecond,
			RetryDelay:  time.Minute,
		}

		_, err := runner.Up(ctx)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.ErrorContains(t, err, "wait to retry migration")
	})

	t.Run("rejects sets goose cannot load", func(t *testing.T) {
		t.Parallel()
		runner := migrate.Runner{URL: testkit.NewDatabase(t).URL, App: fstest.MapFS{
			"001_a.sql": migration("SELECT 1;", ""),
			"001_b.sql": migration("SELECT 2;", ""),
		}}

		_, err := runner.Up(t.Context())

		require.ErrorContains(t, err, "load app migrations")
	})

	t.Run("rejects a malformed url", func(t *testing.T) {
		t.Parallel()

		_, err := migrate.Runner{URL: "postgres://%zz"}.Up(t.Context())

		require.ErrorContains(t, err, "parse migration url")
	})

	t.Run("fails when the server cannot be reached", func(t *testing.T) {
		t.Parallel()

		_, err := migrate.Runner{URL: "postgres://nobody@127.0.0.1:1/none?connect_timeout=1"}.Up(t.Context())

		require.ErrorContains(t, err, "connect for migrations")
	})
}

func TestMigrationLock(t *testing.T) {
	t.Parallel()

	t.Run("waits for the lock and reports who holds it", func(t *testing.T) {
		t.Parallel()
		database := testkit.NewDatabase(t)
		holder := connect(t, database.URL)
		exec(t, holder, "SET application_name = 'other-server'")
		exec(t, holder, "SELECT pg_advisory_lock($1)", lockID("migrate"))

		var mu sync.Mutex
		var reports []migrate.Holder
		runner := migrate.Runner{
			URL: database.URL,
			App: fstest.MapFS{"001_a.sql": migration("CREATE TABLE a (id int);", "")},
			OnWait: func(ctx context.Context, h migrate.Holder) {
				mu.Lock()
				defer mu.Unlock()
				reports = append(reports, h)
				_, _ = holder.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID("migrate"))
			},
		}

		results, err := runner.Up(t.Context())

		require.NoError(t, err)
		assert.Len(t, results, 1)
		mu.Lock()
		defer mu.Unlock()
		require.Len(t, reports, 1)
		assert.Equal(t, "other-server", reports[0].Application)
		assert.NotEmpty(t, reports[0].Address)
		assert.Positive(t, reports[0].PID)
	})

	t.Run("gives up when the lock stays taken", func(t *testing.T) {
		t.Parallel()
		database := testkit.NewDatabase(t)
		holder := connect(t, database.URL)
		exec(t, holder, "SELECT pg_advisory_lock($1)", lockID("deploy"))

		_, err := migrate.Runner{URL: database.URL, LockKey: "deploy", LockWait: time.Second}.Up(t.Context())

		require.ErrorIs(t, err, migrate.ErrLockTimeout)
	})

	t.Run("stops waiting when the context ends", func(t *testing.T) {
		t.Parallel()
		database := testkit.NewDatabase(t)
		holder := connect(t, database.URL)
		exec(t, holder, "SELECT pg_advisory_lock($1)", lockID("migrate"))
		ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
		defer cancel()

		_, err := migrate.Runner{URL: database.URL}.Up(ctx)

		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.False(t, errors.Is(err, migrate.ErrLockTimeout))
	})
}
