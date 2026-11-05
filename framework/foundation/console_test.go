package foundation_test

import (
	"context"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/foundation"
)

/*
whoami is an application command that prints the application's name.
*/
type whoami struct{}

func (whoami) Name() string { return "whoami" }

func (whoami) Description() string { return "Print the application name" }

func (whoami) Flags(*console.Flags) {}

func (whoami) Handle(_ context.Context, a *foundation.Application, _ *console.Input, out *console.Output) error {
	out.Line("name=%s", a.Name)

	return nil
}

func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("lists the release commands", func(t *testing.T) {
		t.Parallel()

		code, stdout, _ := run(t, options(nil), "")

		require.Equal(t, console.ExitSuccess, code)
		for _, name := range []string{
			"serve", "migrate", "migrate:status", "migrate:plan", "migrate:rollback", "down", "up", "health",
			"config:show", "route:list",
		} {
			assert.Contains(t, stdout, name)
		}
	})

	t.Run("shows the configuration with secrets masked", func(t *testing.T) {
		t.Parallel()
		env := environ(t)

		code, stdout, _ := run(t, options(env), "", "config:show")

		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "app.key = [redacted]")
		assert.Contains(t, stdout, "app.env = testing")
		assert.NotContains(t, stdout, env[1][len("APP_APP__KEY="):])
	})

	t.Run("prints a configuration that cannot work beside its problems", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, options([]string{"APP_APP__KEY=base64:secret"}), "", "config:show")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "database.url")
		assert.Contains(t, stderr, "app.key = [redacted]")
	})

	t.Run("migrates, reports status and plans", func(t *testing.T) {
		t.Parallel()
		opts := options(freshEnviron(t))

		code, stdout, _ := run(t, opts, "", "migrate:plan")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "create_gadgets_table")

		code, stdout, _ = run(t, opts, "", "migrate")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Migrated app")

		code, stdout, _ = run(t, opts, "", "migrate")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Nothing to migrate.")

		code, stdout, _ = run(t, opts, "", "migrate:plan")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Nothing to migrate.")

		code, stdout, _ = run(t, opts, "", "migrate:status")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "create_sessions_table")
		assert.Contains(t, stdout, "ran")
	})

	t.Run("rolls back outside production without asking", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_APP__ENV=local"))
		require.Equal(t, console.ExitSuccess, first(run(t, opts, "", "migrate")))

		code, stdout, _ := run(t, opts, "", "migrate:rollback")

		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Rolled back app 20261102160000")
	})

	t.Run("has nothing to roll back on an empty database", func(t *testing.T) {
		t.Parallel()

		code, stdout, _ := run(t, options(freshEnviron(t, "APP_APP__ENV=local")), "", "migrate:rollback")

		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "Nothing to roll back.")
	})

	t.Run("reports a migration that fails", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t))
		opts.Migrations = fstest.MapFS{"20261102170000_broken.sql": {Data: []byte(
			"-- migrate: phase=expand\n\n-- +goose Up\nCREATE TABLE;\n",
		)}}

		code, _, stderr := run(t, opts, "", "migrate")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "migrate")
	})

	t.Run("reports a database it cannot reach", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_DATABASE__URL=postgres://127.0.0.1:1/app"))

		for _, command := range []string{"migrate:status", "migrate:plan", "migrate:rollback"} {
			code, _, _ := run(t, opts, "", command)

			assert.Equal(t, console.ExitFailure, code, command)
		}
	})

	t.Run("takes the application down and up", func(t *testing.T) {
		t.Parallel()
		env := environ(t)
		app := newApp(t, options(env))

		code, stdout, _ := run(t, options(env), "", "down", "--message", "upgrading")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "down for maintenance")
		assert.Eventually(t, func() bool { return active(t, app) }, maintenanceWait, pollEvery)

		code, _, _ = run(t, options(env), "", "up")
		require.Equal(t, console.ExitSuccess, code)
		assert.Eventually(t, func() bool { return !active(t, app) }, maintenanceWait, pollEvery)
	})

	t.Run("reports valkey it cannot reach for maintenance", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_REDIS__URL=redis://127.0.0.1:1"))

		code, _, stderr := run(t, opts, "", "up")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "open valkey")
	})

	t.Run("lists routes and runs application commands on the booted application", func(t *testing.T) {
		t.Parallel()
		routes := provider{register: func(a *foundation.Application) error {
			a.Router.Handle(http.MethodGet, "/hello", "hello", http.NotFoundHandler()).Middleware("session")

			return nil
		}}
		opts := options(environ(t), routes)
		opts.Commands = []foundation.Command{whoami{}}

		code, stdout, _ := run(t, opts, "", "route:list")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "/hello")
		assert.Contains(t, stdout, "session")

		code, stdout, _ = run(t, opts, "", "whoami")
		require.Equal(t, console.ExitSuccess, code)
		assert.Contains(t, stdout, "name=app")
	})

	t.Run("reports an application that cannot boot for a command", func(t *testing.T) {
		t.Parallel()
		opts := options(environ(t, "APP_REDIS__URL=redis://127.0.0.1:1"))
		opts.Commands = []foundation.Command{whoami{}}

		for _, command := range []string{"route:list", "whoami"} {
			code, _, _ := run(t, opts, "", command)

			assert.Equal(t, console.ExitFailure, code, command)
		}
	})
}

func TestServeCommand(t *testing.T) {
	t.Parallel()

	t.Run("refuses a role it does not know", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, options(environ(t)), "", "serve", "--roles", "web,cron")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "cron")
	})

	t.Run("serves the roles it is given until interrupted", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(t.Context())
		started := provider{boot: func(_ context.Context, a *foundation.Application) error {
			go func() {
				assert.Eventually(t, func() bool { return a.Addr() != "" }, maintenanceWait, pollEvery)
				cancel()
			}()

			return nil
		}}
		opts := options(environ(t, "APP_HTTP__ADMIN_ADDR="), started)

		code := foundation.Run(ctx, opts, []string{"serve", "--roles", "web", "--no-migrate"}, console.IO{
			In: nil, Out: discard{}, Err: discard{},
		})

		assert.Equal(t, console.ExitSuccess, code)
	})

	t.Run("reports configuration it cannot load", func(t *testing.T) {
		t.Parallel()

		code, _, _ := run(t, options([]string{}), "", "serve")

		assert.Equal(t, console.ExitFailure, code)
	})

	t.Run("reports an application that cannot boot", func(t *testing.T) {
		t.Parallel()

		code, _, stderr := run(t, options(environ(t, "APP_REDIS__URL=redis://127.0.0.1:1")), "", "serve")

		assert.Equal(t, console.ExitFailure, code)
		assert.Contains(t, stderr, "open valkey")
	})
}

/*
discard is a writer that keeps nothing.
*/
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

/*
first returns the exit code of a run.
*/
func first(code int, _, _ string) int {
	return code
}

/*
active reports whether the application is down for maintenance.
*/
func active(t *testing.T, app *foundation.Application) bool {
	t.Helper()

	down, err := app.Maintenance.Active(t.Context())
	require.NoError(t, err)

	return down
}
