/*
|--------------------------------------------------------------------------
| Test Case
|--------------------------------------------------------------------------
|
| Every business test runs against the application exactly as production
| wires it: the same bootstrap, the same providers, real PostgreSQL and
| real Valkey from compose.yaml. Each test gets a database of its own,
| copied in milliseconds from a template every migration already ran on,
| and a Valkey key prefix of its own, so tests run side by side without
| seeing each other. The clock is a fake the test can move, and password
| hashing is made cheap, which changes how long a hash takes but not
| what it does.
|
| A test case comes with a workspace and its owner, ready to act. Acting
| as someone goes through the real guards: the test case signs a real
| session in or mints a real token, and the request carries it.
|
*/

package tests

import (
	"cmp"
	"context"
	"io"
	"io/fs"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/foundation"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/mail"
	notifydb "github.com/dimmihq/dimmi/framework/notifications/channels/database"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/testkit"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/models"
	"github.com/dimmihq/dimmi/bootstrap"
	"github.com/dimmihq/dimmi/database/factories"
	"github.com/dimmihq/dimmi/database/migrations"
)

/*
Epoch returns the time every test case's clock starts at.
*/
func Epoch() time.Time {
	return time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
}

/*
TestCase is one test's running application with a workspace and its
owner. Database is the test's own database the application runs on.
*/
type TestCase struct {
	T            testing.TB
	App          *app.App
	Clock        *clock.Fake
	Owner        models.User
	Workspace    workspaces.CreatedWorkspace
	Database     testkit.Database
	handler      fhttp.Handler
	mail         *mail.Fake
	mailOnce     sync.Once
	social       *socialite.Fake
	oauthOnce    sync.Once
	role         *database.DB
	roleOnce     sync.Once
	operator     *database.DB
	operatorOnce sync.Once
	environ      []string
	before       []func(a *app.App)
}

/*
Option changes the application before it boots, such as its environment
or one of its fields.
*/
type Option func(o *options)

type options struct {
	env      []string
	before   []func(a *app.App)
	routes   []func(r *routing.Router)
	logs     io.Writer
	language fs.FS
}

/*
WithEnv adds environment entries, KEY=value, which win over the test
defaults.
*/
func WithEnv(entries ...string) Option {
	return func(o *options) { o.env = append(o.env, entries...) }
}

/*
Before changes the application before its providers run, such as
swapping its login policy.
*/
func Before(fn func(a *app.App)) Option {
	return func(o *options) { o.before = append(o.before, fn) }
}

/*
WithRoutes adds routes only the test serves, such as one behind a
middleware no product route uses yet, declared once the application's
own routes are.
*/
func WithRoutes(fn func(r *routing.Router)) Option {
	return func(o *options) { o.routes = append(o.routes, fn) }
}

/*
WithLanguage loads more message files over the product's own, one
directory per locale, such as a test-only language.
*/
func WithLanguage(files fs.FS) Option {
	return func(o *options) { o.language = files }
}

/*
WithLogs writes the application's log lines to the writer instead of
dropping them, so a test can read what was logged.
*/
func WithLogs(w io.Writer) Option {
	return func(o *options) { o.logs = w }
}

/*
New boots the application for the test, with a workspace and its owner,
and shuts it down when the test ends.
*/
func New(tb testing.TB, opts ...Option) *TestCase {
	tb.Helper()

	o := collect(opts)

	booted, err := start(tb, o)
	require.NoError(tb, err)

	application, fake := booted.app, booted.clock

	for _, fn := range o.routes {
		fn(application.Router)
	}

	ctx := tb.Context()
	users, err := factories.User(ctx, application)
	require.NoError(tb, err)

	owner, err := users.CreateOne(ctx)
	require.NoError(tb, err)

	workspace, err := factories.Workspace(ctx, application, owner)
	require.NoError(tb, err)

	routes, err := application.Router.Handler()
	require.NoError(tb, err)

	return &TestCase{
		T: tb, App: application, Clock: fake, Owner: owner, Workspace: workspace,
		Database: booted.database, handler: withLogger(application.Kernel.Handler(routes), application.Log),
		environ: booted.environ,
		before:  o.before,
	}
}

/*
Boot boots the application for the test and returns it with its clock,
or the error that stopped the boot. Most tests want New; Boot is for
tests about booting itself.
*/
func Boot(tb testing.TB, opts ...Option) (*app.App, *clock.Fake, error) {
	tb.Helper()

	booted, err := start(tb, collect(opts))

	return booted.app, booted.clock, err
}

/*
booted is a booted application with its clock, its database and the
environment it read.
*/
type booted struct {
	app      *app.App
	clock    *clock.Fake
	database testkit.Database
	environ  []string
}

/*
start boots the application with the options.
*/
func start(tb testing.TB, o *options) (booted, error) {
	tb.Helper()

	application, foundationOptions := bootstrap.Application()
	application.Frontend = Frontend()
	for _, fn := range o.before {
		fn(application)
	}

	fake := clock.NewFake(Epoch())
	db := MigratedDatabase(tb)
	foundationOptions.Environ = append(environment(tb, applicationURL(tb, db)), o.env...)
	foundationOptions.LogWriter = cmp.Or[io.Writer](o.logs, io.Discard)
	foundationOptions.Clock = fake

	if o.language != nil {
		merged, err := mergeLanguage(foundationOptions.Language, o.language)
		if err != nil {
			return booted{}, err
		}

		foundationOptions.Language = merged
	}

	fa, err := foundation.New(tb.Context(), foundationOptions)
	if err != nil {
		return booted{}, err
	}

	testkit.Cleanup(tb, func() { require.NoError(tb, fa.Shutdown(context.WithoutCancel(tb.Context()))) })

	return booted{app: application, clock: fake, database: db, environ: foundationOptions.Environ}, nil
}

/*
collect applies the options.
*/
func collect(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	return o
}

/*
withLogger gives every request the application's logger, as the server
does, so what a request logs goes where WithLogs points.
*/
func withLogger(next fhttp.Handler, logger log.Logger) fhttp.Handler {
	return fhttp.HandlerFunc(func(w fhttp.ResponseWriter, r *fhttp.Request) {
		next.ServeHTTP(w, r.WithContext(log.NewContext(r.Context(), logger)))
	})
}

/*
Client returns an HTTP client for the application that carries no
credential.
*/
func (tc *TestCase) Client() *testkit.Client {
	return testkit.NewClient(tc.T, tc.handler)
}

/*
Table returns assertions on a table, read outside any workspace, as an
operator would.
*/
func (tc *TestCase) Table(table string) *testkit.TableAssertions {
	return testkit.Table(tc.T, tc.Operator(), table)
}

/*
environment returns the settings that put the application on the
database, with its own Valkey prefix, data directory and key, sign-up
open, cheap hashing and the fake mail transport's log driver.
*/
func environment(tb testing.TB, databaseURL string) []string {
	tb.Helper()

	return []string{
		"DIMMI_APP__ENV=testing",
		"DIMMI_APP__KEY=" + encryption.GenerateAppKey(),
		"DIMMI_APP__DATA_DIR=" + tb.TempDir(),
		"DIMMI_DATABASE__URL=" + databaseURL,
		"DIMMI_DATABASE__MAX_CONNS=4",
		"DIMMI_MIGRATIONS__AUTO=false",
		"DIMMI_QUEUE__MAX_CONNS=2",
		"DIMMI_REDIS__URL=" + testkit.Env(tb, "REDIS_URL"),
		"DIMMI_REDIS__KEY_PREFIX=" + testkit.UniqueName("t") + ":",
		"DIMMI_HTTP__ADDR=127.0.0.1:0",
		"DIMMI_HTTP__ADMIN_ADDR=",
		"DIMMI_RUNTIME__MEMORY_LIMIT=off",
		"DIMMI_LOG__LEVEL=error",
		"DIMMI_HASHING__HASHERS__ARGON2ID__DRIVER=argon2id",
		"DIMMI_HASHING__HASHERS__ARGON2ID__MEMORY=64",
		"DIMMI_HASHING__HASHERS__ARGON2ID__ITERATIONS=1",
		"DIMMI_HASHING__HASHERS__BCRYPT__DRIVER=bcrypt",
		"DIMMI_HASHING__HASHERS__BCRYPT__COST=4",
		"DIMMI_AUTH__SIGNUP__MODE=open",
	}
}

/*
MigratedDatabase returns a database of the test's own, copied from a
template every framework and application migration ran on. Most tests
reach it through New; schema tests use it directly.
*/
func MigratedDatabase(tb testing.TB) testkit.Database {
	tb.Helper()

	version := testkit.Fingerprint(tb, foundation.FrameworkMigrations(), migrations.Files())

	return testkit.NewMigratedDatabase(tb, "dimmi-"+version, func(ctx context.Context, databaseURL string) error {
		_, err := Migrator(databaseURL).Up(ctx)

		return err
	})
}

/*
Migrator returns the migration runner the application runs, for the
database at the URL.
*/
func Migrator(databaseURL string) migrate.Runner {
	return migrate.Runner{
		URL:           databaseURL,
		Framework:     foundation.FrameworkMigrations(),
		App:           migrations.Files(),
		Steps:         []migrate.Step{queue.Migration(), notifydb.Migration("notifications")},
		AllowRollback: true,
	}
}
