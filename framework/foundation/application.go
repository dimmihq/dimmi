/*
|--------------------------------------------------------------------------
| Application
|--------------------------------------------------------------------------
|
| The application is every framework service, built once at boot in an
| order where each service finds what it needs already running, and held
| in typed fields. There is no container and no lookup by name: code
| receives the services it uses through its constructor. Whatever boot
| opens is closed in reverse on shutdown, even when boot stops halfway.
|
*/

package foundation

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/cache"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/database/migrate"
	"github.com/dimmihq/dimmi/framework/dns"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/hashing"
	"github.com/dimmihq/dimmi/framework/health"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/idempotency"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/log"
	"github.com/dimmihq/dimmi/framework/mail"
	"github.com/dimmihq/dimmi/framework/mailbox"
	"github.com/dimmihq/dimmi/framework/malware"
	"github.com/dimmihq/dimmi/framework/notifications"
	"github.com/dimmihq/dimmi/framework/process"
	"github.com/dimmihq/dimmi/framework/pubsub"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/ratelimit"
	"github.com/dimmihq/dimmi/framework/redis"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/schedule"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/telemetry"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
Application holds every framework service of one process.

Name, Env and Roles say what is running. Config is the loaded framework
configuration. Errors renders failures for every HTTP layer, and
Encrypter encrypts with the app key. Maintenance holds the maintenance
state shared by every replica. The other fields are the services, each
documented in its own package.
*/
type Application struct {
	Name        string
	Env         Environment
	Roles       Roles
	Config      *Config
	Log         log.Logger
	Clock       clock.Clock
	Telemetry   *telemetry.Telemetry
	DB          *database.DB
	Migrator    migrate.Runner
	Redis       *redis.Client
	Encrypter   *encryption.AppEncrypter
	Crypt       *encryption.Manager
	Hash        *hashing.Manager
	Cache       *cache.Store
	Limiter     *ratelimit.Limiter
	PubSub      *pubsub.Bus
	Queue       *queue.Queue
	Schedule    *schedule.Schedule
	Events      *events.Dispatcher
	Broadcast   *broadcasting.Server
	HTTP        *httpclient.Client
	DNS         *dns.Client
	Process     *process.Process
	Storage     *filesystem.Manager
	Scanner     *malware.Manager
	Mail        *mail.Manager
	Mailbox     *mailbox.Manager
	Notify      *notifications.Manager
	IDs         *ids.Registry
	Validator   *validation.Factory
	Lang        *i18n.Translator
	Languages   *i18n.DetectorManager
	Errors      fhttp.Errors
	Guards      *auth.Guards
	Gate        *auth.Gate
	Session     *session.Manager
	Social      *socialite.Manager
	Domains     *customdomain.Manager
	Idempotency *idempotency.DatabaseStore
	Maintenance Maintenance
	Router      *routing.Router
	URL         *routing.URL
	Kernel      *fhttp.Kernel
	Health      *health.Registry

	opts     Options
	appKey   []byte
	closers  []closer
	web      atomic.Pointer[webServers]
	shutdown sync.Once
	stopErr  error
}

/*
closer releases one thing boot opened.
*/
type closer struct {
	name  string
	close func(ctx context.Context) error
}

/*
New loads the configuration and boots every service, then registers and
boots the application's providers. Pending migrations are applied when
migrations.auto is on. Nothing is started: Start or Serve runs the
roles. Shut the application down when done.
*/
func New(ctx context.Context, opts Options) (*Application, error) {
	cfg, err := LoadConfig(ctx, opts)
	if err != nil {
		return nil, err
	}

	return boot(ctx, opts, cfg)
}

/*
boot builds the application from a loaded configuration, closing
whatever it opened when a step fails.
*/
func boot(ctx context.Context, opts Options, cfg *Config) (*Application, error) {
	a := &Application{Name: opts.Name, Env: cfg.App.Env, Roles: cfg.App.Roles, Config: cfg, opts: opts}

	steps := []func(context.Context) error{
		a.bootObservability,
		a.bootData,
		a.bootSecurity,
		a.bootValkeyServices,
		a.bootIO,
		a.bootWeb,
		a.bootProviders,
	}

	for _, step := range steps {
		if err := step(ctx); err != nil {
			return nil, errors.Join(err, a.Shutdown(context.WithoutCancel(ctx)))
		}
	}

	return a, nil
}

/*
onClose registers what to release on shutdown, released in reverse
order of registration.
*/
func (a *Application) onClose(name string, fn func(ctx context.Context) error) {
	a.closers = append(a.closers, closer{name: name, close: fn})
}

/*
DerivedKey returns the 32-byte key for one purpose, derived from the app
key, such as the key that seals short codes sent by mail. Different
purposes give unrelated keys, and the same purpose gives the same key
on every replica sharing the app key. Boot refuses an app key of the
wrong size, so deriving cannot fail once the application exists.
*/
func (a *Application) DerivedKey(purpose string) []byte {
	key, _ := encryption.DeriveKey(a.appKey, purpose)

	return key
}

/*
Shutdown stops the roles and releases everything boot opened, in reverse
order, giving up on what is left when the context ends: the listeners
stop accepting, realtime clients are told to reconnect elsewhere, the
workers finish their jobs, providers flush, and the pools close. Only
the first call does anything; later calls return its result.
*/
func (a *Application) Shutdown(ctx context.Context) error {
	a.shutdown.Do(func() {
		var errs []error

		for _, c := range slices.Backward(a.closers) {
			if err := c.close(ctx); err != nil {
				errs = append(errs, err)
			}
		}

		a.stopErr = errors.Join(errs...)
	})

	return a.stopErr
}

/*
closeFunc adapts a release that cannot fail.
*/
func closeFunc(fn func()) func(context.Context) error {
	return func(context.Context) error {
		fn()

		return nil
	}
}
