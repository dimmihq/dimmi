package migrate_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/testkit"
)

func TestStatus(t *testing.T) {
	t.Parallel()

	t.Run("lists applied and pending migrations in order", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		framework := fstest.MapFS{"001_users.sql": migration("CREATE TABLE users (id int);", "")}
		_, err := migrate.Runner{URL: url, Framework: framework}.Up(t.Context())
		require.NoError(t, err)

		migrations, err := migrate.Runner{URL: url, Framework: framework, App: fstest.MapFS{
			"001_posts.sql": migration("CREATE TABLE posts (id int);", ""),
		}}.Status(t.Context())

		require.NoError(t, err)
		require.Len(t, migrations, 2)
		assert.Equal(t, "framework", migrations[0].Set)
		assert.True(t, migrations[0].Applied)
		assert.False(t, migrations[0].AppliedAt.IsZero())
		assert.Equal(t, migrate.Migration{Set: "app", Version: 1, Name: "001_posts.sql"}, migrations[1])
	})

	t.Run("reports a failed connection", func(t *testing.T) {
		t.Parallel()

		_, err := migrate.Runner{URL: "postgres://%zz"}.Status(t.Context())

		require.ErrorContains(t, err, "parse migration url")
	})
}

func TestPlan(t *testing.T) {
	t.Parallel()

	t.Run("lists pending migrations with their sql and findings", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		app := fstest.MapFS{
			"001_users.sql": migration("CREATE TABLE users (id int, legacy text);", "DROP TABLE users;"),
			"002_drop.sql": {Data: []byte("-- migrate: phase=contract\n-- +goose Up\n" +
				"ALTER TABLE users DROP COLUMN legacy;\n-- +goose Down\nSELECT 1;\n")},
			"003_index.sql": {Data: []byte("-- migrate: phase=expand\n-- +goose NO TRANSACTION\n-- +goose Up\n" +
				"CREATE INDEX users_id ON users (id);\n")},
		}
		_, err := migrate.Runner{URL: url, App: fstest.MapFS{"001_users.sql": app["001_users.sql"]}}.Up(t.Context())
		require.NoError(t, err)

		plan, err := migrate.Runner{URL: url, App: app}.Plan(t.Context())

		require.NoError(t, err)
		require.Len(t, plan, 2)
		assert.Equal(t, migrate.Pending{
			Set: "app", Version: 2, Name: "002_drop.sql", Phase: "contract", Transactional: true,
			SQL: "-- migrate: phase=contract\n-- +goose Up\nALTER TABLE users DROP COLUMN legacy;",
		}, plan[0])
		assert.False(t, plan[1].Transactional)
		require.Len(t, plan[1].Findings, 1)
		assert.Equal(t, migrate.RuleBlockingIndex, plan[1].Findings[0].Rule)
	})

	t.Run("reports a migration it cannot read", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		file := filepath.Join(dir, "001_secret.sql")
		require.NoError(t, os.WriteFile(file, migration("SELECT 1;", "").Data, 0o600))
		require.NoError(t, os.Chmod(file, 0))

		_, err := migrate.Runner{URL: testkit.NewDatabase(t).URL, App: os.DirFS(dir)}.Plan(t.Context())

		require.ErrorContains(t, err, "read app migration 001_secret.sql")
	})

	t.Run("reports a failed connection", func(t *testing.T) {
		t.Parallel()

		_, err := migrate.Runner{URL: "postgres://%zz"}.Plan(t.Context())

		require.ErrorContains(t, err, "parse migration url")
	})
}

func TestRollback(t *testing.T) {
	t.Parallel()

	t.Run("is refused outside development", func(t *testing.T) {
		t.Parallel()

		_, err := migrate.Runner{URL: testkit.NewDatabase(t).URL}.Rollback(t.Context())

		require.ErrorIs(t, err, migrate.ErrRollbackDisabled)
	})

	t.Run("undoes the newest migration of the last set first", func(t *testing.T) {
		t.Parallel()
		url := testkit.NewDatabase(t).URL
		runner := migrate.Runner{
			URL:           url,
			AllowRollback: true,
			Framework: fstest.MapFS{
				"001_users.sql": migration("CREATE TABLE users (id int);", "DROP TABLE users;"),
			},
			App: fstest.MapFS{
				"001_posts.sql": migration("CREATE TABLE posts (id int);", "DROP TABLE posts;"),
				"002_tags.sql":  migration("CREATE TABLE tags (id int);", "DROP TABLE tags;"),
			},
		}
		_, err := runner.Up(t.Context())
		require.NoError(t, err)

		var undone []string
		for range 3 {
			result, err := runner.Rollback(t.Context())
			require.NoError(t, err)
			undone = append(undone, result.Set+"/"+result.Name)
		}

		assert.Equal(t, []string{"app/002_tags.sql", "app/001_posts.sql", "framework/001_users.sql"}, undone)
		_, err = runner.Rollback(t.Context())
		require.ErrorIs(t, err, migrate.ErrNothingToRollback)
		assert.False(t, tableExists(t, connect(t, url), "users"))
	})

	t.Run("reports a failing down migration", func(t *testing.T) {
		t.Parallel()
		runner := migrate.Runner{
			URL:           testkit.NewDatabase(t).URL,
			AllowRollback: true,
			App:           fstest.MapFS{"001_a.sql": migration("SELECT 1;", "DROP TABLE missing;")},
		}
		_, err := runner.Up(t.Context())
		require.NoError(t, err)

		_, err = runner.Rollback(t.Context())

		require.ErrorContains(t, err, "roll back app")
	})
}
