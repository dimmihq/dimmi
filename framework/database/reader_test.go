package database_test

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/tenancy"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
replicated is a primary with an items table and a separate database
standing in as its read replica. Both hold a row naming which one it is,
so a read shows where it went.
*/
type replicated struct {
	db      *database.DB
	replica testkit.Database
}

func openReplicated(t *testing.T, maxLag time.Duration) replicated {
	t.Helper()

	primary := testkit.NewDatabase(t)
	replica := testkit.NewDatabase(t)
	seedOrigin(t, primary.URL, "primary")
	seedOrigin(t, replica.URL, "replica")

	db, err := database.Open(t.Context(), database.Config{
		URL:           primary.URL,
		ReadURLs:      []string{replica.URL},
		ReadMaxLag:    maxLag,
		MaxConns:      4,
		PoolMode:      database.PoolModeSession,
		TenantSetting: "app.tenant_id",
	})
	require.NoError(t, err)
	t.Cleanup(db.Close)

	return replicated{db: db, replica: replica}
}

func seedOrigin(t *testing.T, target, name string) {
	t.Helper()

	conn, err := pgx.Connect(t.Context(), target)
	require.NoError(t, err)
	defer func() { _ = conn.Close(t.Context()) }()

	_, err = conn.Exec(t.Context(), `CREATE TABLE origin (name text NOT NULL);
		CREATE TABLE notes (tenant text NOT NULL, body text NOT NULL);
		INSERT INTO notes VALUES ('a', 'note a'), ('b', 'note b');
		ALTER TABLE notes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE notes FORCE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation ON notes USING (tenant = current_setting('app.tenant_id', true))`)
	require.NoError(t, err)
	_, err = conn.Exec(t.Context(), "INSERT INTO origin VALUES ($1)", name)
	require.NoError(t, err)
}

func readOrigin(ctx context.Context, t *testing.T, q database.Querier) string {
	t.Helper()

	var name string
	require.NoError(t, q.QueryRow(ctx, "SELECT name FROM origin").Scan(&name))

	return name
}

/*
setConnections allows or refuses new connections to the test database
and ends the open ones, which is how a replica looks when it goes down.
*/
func setConnections(t *testing.T, db testkit.Database, allow bool) {
	t.Helper()

	u, err := url.Parse(db.URL)
	require.NoError(t, err)
	name := strings.TrimPrefix(u.Path, "/")

	admin, err := pgx.Connect(t.Context(), testkit.Env(t, "DATABASE_URL"))
	require.NoError(t, err)
	defer func() { _ = admin.Close(t.Context()) }()

	_, err = admin.Exec(t.Context(), fmt.Sprintf("ALTER DATABASE %s ALLOW_CONNECTIONS %t", name, allow))
	require.NoError(t, err)
	_, err = admin.Exec(t.Context(),
		"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1", name)
	require.NoError(t, err)
}

func TestReader(t *testing.T) {
	t.Parallel()

	t.Run("reads the primary when no replica is configured", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		mustExec(t, db, "INSERT INTO items VALUES (1, 'a')")

		var name string
		require.NoError(t, db.Reader(t.Context()).QueryRow(t.Context(), "SELECT name FROM items").Scan(&name))
		assert.Equal(t, "a", name)
	})

	t.Run("reads a replica once its lag check passes", func(t *testing.T) {
		t.Parallel()
		r := openReplicated(t, 0)

		assert.Eventually(t, func() bool {
			return readOrigin(t.Context(), t, r.db.Reader(t.Context())) == "replica"
		}, 5*time.Second, 10*time.Millisecond)
	})

	t.Run("keeps reads on the primary when the replica lags too far", func(t *testing.T) {
		t.Parallel()
		r := openReplicated(t, -time.Second)

		for range 20 {
			assert.Equal(t, "primary", readOrigin(t.Context(), t, r.db.Reader(t.Context())))
			clock.System().Sleep(15 * time.Millisecond)
		}
	})

	t.Run("keeps reads on the primary after the context wrote", func(t *testing.T) {
		t.Parallel()
		r := openReplicated(t, 10*time.Second)
		require.Eventually(t, func() bool {
			return readOrigin(t.Context(), t, r.db.Reader(t.Context())) == "replica"
		}, 5*time.Second, 10*time.Millisecond)

		untracked := t.Context()
		_, err := r.db.Exec(untracked, "UPDATE origin SET name = name")
		require.NoError(t, err)
		assert.Equal(t, "replica", readOrigin(untracked, t, r.db.Reader(untracked)), "untracked writes are unknown")

		execed := database.TrackWrites(t.Context())
		assert.Equal(t, "replica", readOrigin(execed, t, r.db.Reader(execed)), "no write yet")
		_, err = r.db.Exec(execed, "UPDATE origin SET name = name")
		require.NoError(t, err)
		assert.Equal(t, "primary", readOrigin(execed, t, r.db.Reader(execed)))

		transacted := database.TrackWrites(t.Context())
		require.NoError(t, r.db.Transaction(transacted, func(context.Context, *database.Tx) error { return nil }))
		assert.Equal(t, "primary", readOrigin(transacted, t, r.db.Reader(transacted)))
	})

	t.Run("reads inside a transaction through the transaction", func(t *testing.T) {
		t.Parallel()
		db := open(t)

		err := db.Transaction(t.Context(), func(ctx context.Context, tx *database.Tx) error {
			mustExecTx(ctx, t, tx, "INSERT INTO items VALUES (1, 'a')")
			assert.Same(t, tx, db.Reader(ctx))

			return nil
		})
		require.NoError(t, err)
	})

	t.Run("moves reads to the primary when the replica stops answering", func(t *testing.T) {
		t.Parallel()
		r := openReplicated(t, 10*time.Second)
		require.Eventually(t, func() bool {
			return readOrigin(t.Context(), t, r.db.Reader(t.Context())) == "replica"
		}, 5*time.Second, 10*time.Millisecond)

		setConnections(t, r.replica, false)

		reader := r.db.Reader(t.Context())
		assert.Equal(t, "primary", readOrigin(t.Context(), t, reader))
		assert.Equal(t, "primary", readOrigin(t.Context(), t, r.db.Reader(t.Context())), "skipped until checked")
	})

	t.Run("returns to a replica when a later check passes", func(t *testing.T) {
		t.Parallel()
		replica := testkit.NewDatabase(t)
		seedOrigin(t, replica.URL, "replica")
		setConnections(t, replica, false)
		primary := testkit.NewDatabase(t)
		seedOrigin(t, primary.URL, "primary")

		db, err := database.Open(t.Context(), database.Config{
			URL: primary.URL, ReadURLs: []string{replica.URL}, ReadMaxLag: time.Second,
			MaxConns: 2, PoolMode: database.PoolModeSession,
		})
		require.NoError(t, err)
		t.Cleanup(db.Close)
		assert.Equal(t, "primary", readOrigin(t.Context(), t, db.Reader(t.Context())))

		setConnections(t, replica, true)

		assert.Eventually(t, func() bool {
			return readOrigin(t.Context(), t, db.Reader(t.Context())) == "replica"
		}, 10*time.Second, 50*time.Millisecond)
	})

	t.Run("scopes reads to the tenant", func(t *testing.T) {
		t.Parallel()
		r := openReplicated(t, 10*time.Second)
		require.Eventually(t, func() bool {
			return readOrigin(t.Context(), t, r.db.Reader(t.Context())) == "replica"
		}, 5*time.Second, 10*time.Millisecond)

		var tenantRows, anonymousRows int
		ctx := tenancy.With(t.Context(), "a")
		require.NoError(t, r.db.Reader(ctx).QueryRow(ctx, "SELECT count(*) FROM notes").Scan(&tenantRows))
		anonymous := r.db.Reader(t.Context())
		require.NoError(t, anonymous.QueryRow(t.Context(), "SELECT count(*) FROM notes").Scan(&anonymousRows))

		assert.Equal(t, 1, tenantRows)
		assert.Zero(t, anonymousRows)
	})

	t.Run("runs queries and statements read-only", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		mustExec(t, db, "INSERT INTO items VALUES (1, 'a'), (2, 'b')")
		reader := db.Reader(t.Context())

		rows, err := reader.Query(t.Context(), "SELECT name FROM items ORDER BY id")
		require.NoError(t, err)
		names, err := pgx.CollectRows(rows, pgx.RowTo[string])
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, names)

		tag, err := reader.Exec(t.Context(), "SELECT 1")
		require.NoError(t, err)
		assert.Equal(t, "SELECT 1", tag.String())

		_, err = reader.Exec(t.Context(), "INSERT INTO items VALUES (3, 'c')")
		require.ErrorIs(t, err, database.ErrReadOnly)
		assert.Equal(t, 2, countItems(t.Context(), t, db))
	})

	t.Run("reports read errors", func(t *testing.T) {
		t.Parallel()
		db := open(t)
		reader := db.Reader(t.Context())

		err := reader.QueryRow(t.Context(), "SELECT name FROM items").Scan(new(string))
		require.ErrorIs(t, err, database.ErrNotFound)

		require.ErrorContains(t, queryError(t.Context(), reader, "SELECT * FROM missing"), "missing")

		canceled, cancel := context.WithCancel(t.Context())
		cancel()
		_, err = reader.Exec(canceled, "SELECT 1")
		require.ErrorContains(t, err, "begin read")
		require.ErrorIs(t, queryError(canceled, reader, "SELECT 1"), context.Canceled)
		require.ErrorIs(t, reader.QueryRow(canceled, "SELECT 1").Scan(new(int)), context.Canceled)
	})

	t.Run("reports a failure to set the tenant", func(t *testing.T) {
		t.Parallel()
		db, err := database.Open(t.Context(), database.Config{
			URL:           testkit.NewDatabase(t).URL,
			MaxConns:      2,
			PoolMode:      database.PoolModeSession,
			TenantSetting: "not a valid setting name",
		})
		require.NoError(t, err)
		t.Cleanup(db.Close)

		_, err = db.Reader(t.Context()).Exec(tenancy.With(t.Context(), "a"), "SELECT 1")

		require.ErrorContains(t, err, "set tenant")
	})

	t.Run("rejects a malformed read url", func(t *testing.T) {
		t.Parallel()

		_, err := database.Open(t.Context(), database.Config{
			URL:      testkit.NewDatabase(t).URL,
			ReadURLs: []string{"postgres://%zz"},
			MaxConns: 2,
			PoolMode: database.PoolModeSession,
		})

		require.ErrorContains(t, err, "read url 0")
	})
}
