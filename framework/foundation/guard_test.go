package foundation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
)

/*
production returns the options of an application in production on a
database of its own.
*/
func production(t *testing.T) foundation.Options {
	t.Helper()

	return options(freshEnviron(t, "APP_APP__ENV=production"))
}

/*
migrated returns production options whose database is fully migrated.
*/
func migrated(t *testing.T) foundation.Options {
	t.Helper()

	opts := production(t)
	require.Equal(t, console.ExitSuccess, first(run(t, opts, "", "migrate", "--force")))

	return opts
}

func TestProductionGuard(t *testing.T) {
	t.Parallel()

	t.Run("refuses to migrate production without someone to confirm", func(t *testing.T) {
		t.Parallel()
		opts := production(t)

		code, _, stderr := run(t, opts, "", "migrate")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "--force")
		_, stdout, _ := run(t, opts, "", "migrate:plan")
		assert.Contains(t, stdout, "create_gadgets_table")
	})

	t.Run("migrates production once the operator confirms", func(t *testing.T) {
		t.Parallel()
		opts := production(t)

		code, stdout, _ := run(t, opts, "y", "migrate")

		require.Equal(t, console.ExitSuccess, code)
		assert.Regexp(t, `Migrating production database [^ ]+:15432/t`, stdout)
		assert.Contains(t, stdout, "create_gadgets_table")
		assert.Contains(t, stdout, "Migrated app")
	})

	t.Run("leaves production alone when the operator declines", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, production(t), "n", "migrate")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "not confirmed")
	})

	t.Run("migrates production with force and asks nothing when nothing is pending", func(t *testing.T) {
		t.Parallel()
		opts := migrated(t)

		code, stdout, _ := run(t, opts, "", "migrate")

		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Nothing to migrate.")
	})

	t.Run("refuses to roll back production without the version", func(t *testing.T) {
		t.Parallel()
		opts := migrated(t)

		code, _, stderr := run(t, opts, "", "migrate:rollback")
		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "--confirm-version")

		code, _, stderr = run(t, opts, "", "migrate:rollback", "--force", "--confirm-version", "20260101000000")
		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "not the version being rolled back")

		code, _, _ = run(t, opts, "20260101000000\n", "migrate:rollback")
		assert.Equal(t, console.ExitFailure, code)

		code, _, _ = run(t, opts, "\n", "migrate:rollback")
		assert.Equal(t, console.ExitFailure, code)
	})

	t.Run("rolls back production with force and the exact version", func(t *testing.T) {
		t.Parallel()
		opts := migrated(t)

		code, stdout, _ := run(t, opts, "", "migrate:rollback", "--force", "--confirm-version", "20261102160000")

		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "This can drop data.")
		assert.Contains(t, stdout, "Rolled back app 20261102160000")
	})

	t.Run("rolls back production once the operator types the version or its name", func(t *testing.T) {
		t.Parallel()
		opts := migrated(t)

		code, stdout, _ := run(t, opts, "20261102160000\n", "migrate:rollback")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Rolled back app 20261102160000")

		code, stdout, _ = run(t, opts, "20261201080004_add_sign_in_context_to_sessions.sql\n", "migrate:rollback")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Rolled back framework 20261201080004")
	})

	t.Run("takes production down only when confirmed", func(t *testing.T) {
		t.Parallel()
		opts := production(t)

		code, _, stderr := run(t, opts, "", "down")
		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "--force")

		code, _, _ = run(t, opts, "n", "down")
		assert.Equal(t, console.ExitFailure, code)

		code, stdout, _ := run(t, opts, "y", "down")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "turns every user of production away")

		code, _, _ = run(t, opts, "", "down", "--force")
		assert.Equal(t, console.ExitSuccess, code)
	})
}
