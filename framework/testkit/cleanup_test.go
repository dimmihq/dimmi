package testkit_test

import (
	"context"
	"runtime"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
ownTest stands in for a test of its own: failures are recorded, and its
cleanups run, last first, once the test function returns or stops at a
fatal failure, as they would for a real test.
*/
type ownTest struct {
	*recorder
	cleanups []func()
}

func (o *ownTest) Cleanup(fn func()) {
	o.cleanups = append(o.cleanups, fn)
}

/*
runOwn runs fn as a test of its own on its own goroutine and returns it
once fn and every cleanup it registered have finished.
*/
func runOwn(t *testing.T, fn func(tb *ownTest)) *ownTest {
	t.Helper()

	own := &ownTest{recorder: &recorder{TB: t}}
	done := make(chan struct{})

	go func() {
		defer close(done)
		defer func() {
			for _, cleanup := range slices.Backward(own.cleanups) {
				cleanup()
			}
		}()

		fn(own)
	}()

	<-done

	return own
}

/*
leftovers returns which of the names still exist on the server as a
database or a role.
*/
func leftovers(t *testing.T, names ...string) []string {
	t.Helper()

	ctx := t.Context()
	admin, err := pgx.Connect(ctx, testkit.Env(t, "DATABASE_URL"))
	require.NoError(t, err)

	defer func() { _ = admin.Close(context.WithoutCancel(ctx)) }()

	var found []string
	require.NoError(t, admin.QueryRow(ctx, `
		SELECT coalesce(array_agg(name), '{}') FROM (
			SELECT datname::text AS name FROM pg_database WHERE datname = ANY ($1)
			UNION ALL
			SELECT rolname::text FROM pg_roles WHERE rolname = ANY ($1)
		) AS existing`, names).Scan(&found))

	return found
}

func TestCleanup(t *testing.T) {
	t.Parallel()

	t.Run("drops the database and every role of a test that passed", func(t *testing.T) {
		t.Parallel()

		var db testkit.Database
		var role testkit.Role

		own := runOwn(t, func(tb *ownTest) {
			db = testkit.NewDatabase(tb)
			role = db.NewRole(tb)
		})

		assert.Empty(t, own.Failures())
		assert.Empty(t, leftovers(t, db.Name, role.Name))
	})

	t.Run("drops them for a test that failed with connections still open", func(t *testing.T) {
		t.Parallel()

		var db testkit.Database
		var role testkit.Role

		own := runOwn(t, func(tb *ownTest) {
			db = testkit.NewDatabase(tb)
			role = db.NewRole(tb)

			conn, err := pgx.Connect(tb.Context(), role.URL)
			if err != nil {
				tb.Fatalf("connect as the role: %v", err)
			}

			tb.Cleanup(func() { _ = conn.Close(context.WithoutCancel(tb.Context())) })
			tb.Fatalf("the test failed")
		})

		assert.Equal(t, []string{"the test failed"}, own.Failures())
		assert.Empty(t, leftovers(t, db.Name, role.Name))
	})

	t.Run("drops a role granted privileges in a migrated copy", func(t *testing.T) {
		t.Parallel()

		version := testkit.UniqueName("version")
		t.Cleanup(func() { dropTemplate(t, version) })

		var db testkit.Database
		var role testkit.Role

		own := runOwn(t, func(tb *ownTest) {
			db = testkit.NewMigratedDatabase(tb, version, func(ctx context.Context, url string) error {
				return exec(ctx, url, "CREATE TABLE items (id int PRIMARY KEY)")
			})
			role = db.NewRole(tb, "SELECT ON items")

			runtime.Goexit()
		})

		assert.Empty(t, own.Failures())
		assert.Empty(t, leftovers(t, db.Name, role.Name))
	})
}

func TestNewRole(t *testing.T) {
	t.Parallel()

	t.Run("connects as a role that owns nothing, so row level security applies", func(t *testing.T) {
		t.Parallel()

		db := testkit.NewDatabase(t)
		require.NoError(t, exec(t.Context(), db.URL,
			"CREATE TABLE notes (owner text, body text)",
			"ALTER TABLE notes ENABLE ROW LEVEL SECURITY",
			"CREATE POLICY notes_mine ON notes USING (owner = current_user)",
			"INSERT INTO notes VALUES ('someone', 'hidden')",
		))
		role := db.NewRole(t, "SELECT, INSERT ON notes")
		require.NoError(t, exec(t.Context(), role.URL, "INSERT INTO notes VALUES (current_user, 'mine')"))

		var visible, all int
		require.NoError(t, connect(t, role.URL).QueryRow(t.Context(), "SELECT count(*) FROM notes").Scan(&visible))
		require.NoError(t, connect(t, db.URL).QueryRow(t.Context(), "SELECT count(*) FROM notes").Scan(&all))

		assert.Equal(t, 1, visible)
		assert.Equal(t, 2, all)
		assert.NotEqual(t, db.Name, role.Name)
	})

	t.Run("stops when the database cannot be reached", func(t *testing.T) {
		t.Parallel()

		db := testkit.Database{Name: "missing", AdminURL: "postgres://127.0.0.1:1/missing"}

		rec := record(t, func(tb *recorder) {
			db.NewRole(tb)
		})

		assert.Contains(t, rec.Failures()[0], "connect to missing")
	})
}
