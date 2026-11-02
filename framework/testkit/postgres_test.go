package testkit_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"strconv"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
connect opens a pool of two connections to the URL, closed when the
test ends.
*/
func connect(t *testing.T, url string) *pgxpool.Pool {
	t.Helper()

	cfg, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)

	cfg.MaxConns = 2

	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	require.NoError(t, err)

	t.Cleanup(pool.Close)

	return pool
}

/*
exec runs statements on a fresh connection to the URL.
*/
func exec(ctx context.Context, url string, statements ...string) error {
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		return err
	}

	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()

	for _, statement := range statements {
		if _, err := conn.Exec(ctx, statement); err != nil {
			return err
		}
	}

	return nil
}

func TestTable(t *testing.T) {
	t.Parallel()

	db := testkit.NewDatabase(t)
	conn := connect(t, db.URL)

	require.NoError(t, exec(t.Context(), db.URL,
		"CREATE SCHEMA audit",
		"CREATE TABLE audit.items (id int PRIMARY KEY, name text, color text)",
		"INSERT INTO audit.items VALUES (1, 'a', 'red'), (2, 'b', NULL), (3, 'c', 'red')",
		"CREATE TABLE empty (id int)",
	))

	t.Run("finds rows by the values that matter", func(t *testing.T) {
		t.Parallel()

		testkit.AssertDatabaseHas(t, conn, "audit.items", testkit.Row{"id": 1, "name": "a"})
		testkit.AssertDatabaseHas(t, conn, "audit.items", testkit.Row{"name": "b", "color": nil})
		testkit.AssertDatabaseMissing(t, conn, "audit.items", testkit.Row{"name": "a", "color": nil})
		testkit.AssertDatabaseCount(t, conn, "audit.items", 3)
		testkit.Table(t, conn, "audit.items").
			AssertCount(2, testkit.Row{"color": "red"}).
			AssertCount(1, testkit.Row{"color": "red"}, testkit.Row{"name": "c"})
	})

	t.Run("reports what it found instead", func(t *testing.T) {
		t.Parallel()

		rec := record(t, func(tb *recorder) {
			testkit.Table(tb, conn, "audit.items").
				AssertHas(testkit.Row{"name": "z"}).
				AssertMissing(testkit.Row{"color": "red"}).
				AssertCount(5)
			testkit.AssertDatabaseHas(tb, conn, "empty", testkit.Row{"id": 1})
			testkit.AssertDatabaseHas(tb, conn, "audit.items", testkit.Row{"name": func() {}})
		})

		failures := rec.Failures()
		require.Len(t, failures, 5)
		assert.Contains(t, failures[0], `no row in audit.items matches {"name":"z"}; found: {"id":1,"name":"a"`)
		assert.Equal(t, `2 rows in audit.items match {"color":"red"}`, failures[1])
		assert.Equal(t, "audit.items has 3 rows matching {}, want 5", failures[2])
		assert.Contains(t, failures[3], "found: no rows")
		assert.Contains(t, failures[4], "count rows in audit.items")
	})

	t.Run("stops on a table that does not exist", func(t *testing.T) {
		t.Parallel()

		rec := record(t, func(tb *recorder) {
			testkit.AssertDatabaseCount(tb, conn, "missing", 0)
		})

		assert.Contains(t, rec.Failures()[0], "count rows in missing")
	})
}

func TestNewMigratedDatabase(t *testing.T) {
	t.Parallel()

	version := testkit.UniqueName("version")
	runs := atomic.Int32{}
	migrate := func(ctx context.Context, url string) error {
		runs.Add(1)

		return exec(ctx, url, "CREATE TABLE items (id int PRIMARY KEY)", "ALTER TABLE items ENABLE ROW LEVEL SECURITY")
	}

	t.Cleanup(func() { dropTemplate(t, version) })

	t.Run("migrates the template once for parallel tests", func(t *testing.T) {
		t.Parallel()

		t.Run("copies", func(t *testing.T) {
			for i := range 4 {
				t.Run(strconv.Itoa(i), func(t *testing.T) {
					t.Parallel()
					testkit.NewMigratedDatabase(t, version, migrate)
				})
			}
		})

		assert.Equal(t, int32(1), runs.Load())
	})

	t.Run("hands each copy to the test's own role", func(t *testing.T) {
		t.Parallel()

		db := testkit.NewMigratedDatabase(t, version, migrate)
		conn := connect(t, db.URL)

		var owner, user string
		require.NoError(t, conn.QueryRow(t.Context(),
			"SELECT tableowner, current_user FROM pg_tables WHERE tablename = 'items'").Scan(&owner, &user))
		_, err := conn.Exec(t.Context(), "INSERT INTO items VALUES (1)")

		require.NoError(t, err)
		assert.Equal(t, user, owner)
	})

	t.Run("rebuilds a template whose build did not finish", func(t *testing.T) {
		t.Parallel()

		stale := testkit.UniqueName("stale")
		t.Cleanup(func() { dropTemplate(t, stale) })

		first := record(t, func(tb *recorder) {
			testkit.NewMigratedDatabase(tb, stale, func(context.Context, string) error { return assert.AnError })
		})
		db := testkit.NewMigratedDatabase(t, stale, func(ctx context.Context, url string) error {
			return exec(ctx, url, "CREATE TABLE items (id int PRIMARY KEY)")
		})

		assert.Contains(t, first.Failures()[0], "migrate template")
		require.NoError(t, exec(t.Context(), db.URL, "INSERT INTO items VALUES (1)"))
	})
}

/*
dropTemplate removes the template built for the version, which no later
run uses.
*/
func dropTemplate(t *testing.T, version string) {
	t.Helper()

	ctx := context.WithoutCancel(t.Context())
	admin, err := pgx.Connect(ctx, testkit.Env(t, "DATABASE_URL"))
	require.NoError(t, err)

	defer func() { _ = admin.Close(ctx) }()

	sum := sha256.Sum256([]byte(version))
	name := "tpl_" + hex.EncodeToString(sum[:8])

	for _, statement := range []string{
		"ALTER DATABASE " + name + " IS_TEMPLATE false",
		"DROP DATABASE " + name + " WITH (FORCE)",
		"DROP ROLE " + name,
	} {
		_, err := admin.Exec(ctx, statement)
		assert.NoError(t, err)
	}
}

/*
brokenFS is a file system that cannot be read.
*/
type brokenFS struct{}

func (brokenFS) Open(string) (fs.File, error) {
	return nil, fs.ErrPermission
}

func TestFingerprint(t *testing.T) {
	t.Parallel()

	t.Run("changes with any file", func(t *testing.T) {
		t.Parallel()

		base := fstest.MapFS{"1_init.sql": {Data: []byte("a")}}
		edited := fstest.MapFS{"1_init.sql": {Data: []byte("b")}}
		renamed := fstest.MapFS{"2_init.sql": {Data: []byte("a")}}

		assert.Equal(t, testkit.Fingerprint(t, base), testkit.Fingerprint(t, base))
		assert.NotEqual(t, testkit.Fingerprint(t, base), testkit.Fingerprint(t, edited))
		assert.NotEqual(t, testkit.Fingerprint(t, base), testkit.Fingerprint(t, renamed))
		assert.NotEqual(t, testkit.Fingerprint(t, base), testkit.Fingerprint(t, base, edited))
	})

	t.Run("stops on a file system it cannot read", func(t *testing.T) {
		t.Parallel()

		rec := record(t, func(tb *recorder) {
			testkit.Fingerprint(tb, brokenFS{})
		})

		assert.NotEmpty(t, rec.Failures())
	})
}
