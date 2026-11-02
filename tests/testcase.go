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
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/foundation"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	notifydb "github.com/dimmihq/dimmi/framework/notifications/channels/database"
	"github.com/dimmihq/dimmi/framework/queue"
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
owner.
*/
type TestCase struct {
	T         testing.TB
	App       *app.App
	Clock     *clock.Fake
	Owner     models.User
	Workspace workspaces.CreatedWorkspace
	handler   fhttp.Handler
}

/*
Option changes the application before it boots, such as its environment
or one of its fields.
*/
type Option func(o *options)

type options struct {
	env    []string
	before []func(a *app.App)
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
New boots the application for the test, with a workspace and its owner,
and shuts it down when the test ends.
*/
func New(tb testing.TB, opts ...Option) *TestCase {
	tb.Helper()

	application, fake, err := Boot(tb, opts...)
	require.NoError(tb, err)

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
		handler: application.Kernel.Handler(routes),
	}
}

/*
Boot boots the application for the test and returns it with its clock,
or the error that stopped the boot. Most tests want New; Boot is for
tests about booting itself.
*/
func Boot(tb testing.TB, opts ...Option) (*app.App, *clock.Fake, error) {
	tb.Helper()

	o := &options{}
	for _, opt := range opts {
		opt(o)
	}

	application, foundationOptions := bootstrap.Application()
	for _, fn := range o.before {
		fn(application)
	}

	fake := clock.NewFake(Epoch())
	foundationOptions.Environ = append(environment(tb), o.env...)
	foundationOptions.LogWriter = io.Discard
	foundationOptions.Clock = fake

	fa, err := foundation.New(tb.Context(), foundationOptions)
	if err != nil {
		return nil, nil, err
	}

	tb.Cleanup(func() { require.NoError(tb, fa.Shutdown(context.WithoutCancel(tb.Context()))) })

	return application, fake, nil
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
	return testkit.Table(tc.T, tc.App.DB, table)
}

/*
environment returns the test's own database, Valkey prefix, data
directory and key, with sign-up open, cheap hashing and the fake mail
transport's log driver.
*/
func environment(tb testing.TB) []string {
	tb.Helper()

	return []string{
		"DIMMI_APP__ENV=testing",
		"DIMMI_APP__KEY=" + encryption.GenerateAppKey(),
		"DIMMI_APP__DATA_DIR=" + tb.TempDir(),
		"DIMMI_DATABASE__URL=" + MigratedDatabase(tb).URL,
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
