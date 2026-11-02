package foundation_test

import (
	"context"
	"io"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/foundation"
	"github.com/dimmihq/dimmi/framework/httpclient"
	notifydb "github.com/dimmihq/dimmi/framework/notifications/channels/database"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/testkit"
)

/*
How long tests wait for something another goroutine or replica does,
and how often they look.
*/
const (
	maintenanceWait = 5 * time.Second
	pollEvery       = 20 * time.Millisecond
)

/*
appMigrations is the application's own migration set used by the
tests.
*/
func appMigrations() fstest.MapFS {
	return fstest.MapFS{
		"20261102160000_create_gadgets_table.sql": {Data: []byte(
			"-- migrate: phase=expand\n\n-- +goose Up\nCREATE TABLE gadgets (id int PRIMARY KEY);\n\n" +
				"-- +goose Down\nDROP TABLE gadgets;\n",
		)},
	}
}

/*
migratedDatabase returns a database of the test's own, copied from a
template that every framework and test migration already ran on.
*/
func migratedDatabase(t *testing.T) string {
	t.Helper()

	version := testkit.Fingerprint(t, foundation.FrameworkMigrations(), appMigrations())

	return testkit.NewMigratedDatabase(t, "foundation-"+version, func(ctx context.Context, url string) error {
		_, err := migrate.Runner{
			URL:       url,
			Framework: foundation.FrameworkMigrations(),
			App:       appMigrations(),
			Steps:     []migrate.Step{queue.Migration(), notifydb.Migration("notifications")},
		}.Up(ctx)

		return err
	}).URL
}

/*
environ returns the environment of a test application on a migrated
database of its own. Extra entries, KEY=value, are added last and win.
*/
func environ(t *testing.T, extra ...string) []string {
	t.Helper()

	return environOn(t, migratedDatabase(t), extra...)
}

/*
freshEnviron is environ on an empty database, for tests that watch
migrations run.
*/
func freshEnviron(t *testing.T, extra ...string) []string {
	t.Helper()

	return environOn(t, testkit.NewDatabase(t).URL, extra...)
}

/*
environOn returns the environment of a test application on the
database: a key prefix of its own, a data directory in a fresh
temporary directory, listeners on free loopback ports, small pools and
mail sent to Mailpit.
*/
func environOn(t *testing.T, databaseURL string, extra ...string) []string {
	t.Helper()

	smtp, err := url.Parse(testkit.Env(t, "MAIL_SMTP_URL"))
	require.NoError(t, err)

	env := []string{
		"APP_APP__ENV=testing",
		"APP_APP__KEY=" + encryption.GenerateAppKey(),
		"APP_APP__DATA_DIR=" + t.TempDir(),
		"APP_DATABASE__URL=" + databaseURL,
		"APP_DATABASE__MAX_CONNS=2",
		"APP_QUEUE__MAX_CONNS=3",
		"APP_QUEUE__POLL_INTERVAL=50ms",
		"APP_REDIS__URL=" + testkit.Env(t, "REDIS_URL"),
		"APP_REDIS__KEY_PREFIX=" + testkit.UniqueName("t") + ":",
		"APP_HTTP__ADDR=127.0.0.1:0",
		"APP_HTTP__ADMIN_ADDR=127.0.0.1:0",
		"APP_HTTP__SHUTDOWN_TIMEOUT=10s",
		"APP_HTTP_CLIENT__ALLOW_PRIVATE_NETWORKS=true",
		"APP_RUNTIME__MEMORY_LIMIT=off",
		"APP_TELEMETRY__METRICS=prometheus",
		"APP_LOG__LEVEL=error",
		"APP_MAIL__FROM__EMAIL=app@example.com",
		"APP_MAIL__MAILERS__SYSTEM__DRIVER=smtp",
		"APP_MAIL__MAILERS__SYSTEM__HOST=" + smtp.Hostname(),
		"APP_MAIL__MAILERS__SYSTEM__PORT=" + smtp.Port(),
		"APP_MAIL__MAILERS__SYSTEM__ALLOW_INSECURE=true",
	}

	return append(env, extra...)
}

/*
options returns the options of a test application with the environment.
*/
func options(env []string, providers ...foundation.Provider) foundation.Options {
	return foundation.Options{
		Name:       "app",
		Version:    "1.0.0",
		EnvPrefix:  "APP",
		Environ:    env,
		Migrations: appMigrations(),
		Providers:  providers,
		LogWriter:  io.Discard,
	}
}

/*
newApp boots an application with the options and shuts it down when the
test ends.
*/
func newApp(t *testing.T, opts foundation.Options) *foundation.Application {
	t.Helper()

	app, err := foundation.New(t.Context(), opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, app.Shutdown(context.WithoutCancel(t.Context()))) })

	return app
}

/*
get requests the URL through a client that may reach loopback.
*/
func get(t *testing.T, rawURL string) (*httpclient.Response, error) {
	t.Helper()

	client, err := httpclient.New(httpclient.Config{AllowPrivateNetworks: true, Timeout: 5 * time.Second})
	require.NoError(t, err)

	return client.New().Get(t.Context(), rawURL)
}

/*
mustGet requests the URL and fails the test when no response arrives.
*/
func mustGet(t *testing.T, rawURL string) *httpclient.Response {
	t.Helper()

	resp, err := get(t, rawURL)
	require.NoError(t, err)

	return resp
}

/*
provider is a provider made of functions, any of which may be nil.
*/
type provider struct {
	register func(a *foundation.Application) error
	boot     func(ctx context.Context, a *foundation.Application) error
	shutdown func(ctx context.Context) error
}

func (p provider) Register(a *foundation.Application) error {
	if p.register == nil {
		return nil
	}

	return p.register(a)
}

func (p provider) Boot(ctx context.Context, a *foundation.Application) error {
	if p.boot == nil {
		return nil
	}

	return p.boot(ctx, a)
}

/*
shutdownProvider is a provider with work to finish at shutdown.
*/
type shutdownProvider struct {
	provider
}

func (p shutdownProvider) Shutdown(ctx context.Context) error {
	return p.shutdown(ctx)
}

/*
run runs the command line with the options and arguments, answering
prompts from the input when interactive, and returns the exit code,
standard output and standard error.
*/
func run(t *testing.T, opts foundation.Options, input string, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr strings.Builder
	streams := console.IO{In: strings.NewReader(input), Out: &stdout, Err: &stderr, Interactive: input != ""}
	code := foundation.Run(t.Context(), opts, args, streams)

	return code, stdout.String(), stderr.String()
}
