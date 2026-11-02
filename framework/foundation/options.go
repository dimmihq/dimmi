/*
|--------------------------------------------------------------------------
| Application Options
|--------------------------------------------------------------------------
|
| The application tells the foundation who it is and what it brings: its
| name, where its configuration lives, its own migrations, providers and
| commands. Everything product-specific enters through these options, so
| the foundation itself stays free of any one product.
|
*/

package foundation

import (
	"context"
	"io"
	"io/fs"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/broadcasting"
	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/console"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/notifications/channels/webpush"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/telemetry"
)

/*
Options describe the application to the foundation.

Name names the binary, the default configuration file Name.yaml, the
telemetry service and the default key prefix on Valkey. Version is
printed by the command line. EnvPrefix selects the environment variables
that override configuration, such as APP for APP_DATABASE__URL;
ConfigFiles lists YAML files loaded in order instead of Name.yaml.
Environ replaces the process environment when not nil, which tests use
to boot without touching the process. Config, when set, points at the
application's own configuration struct, which is loaded from the same
files and environment as the framework's and checked with it, so a bad
application value stops the boot like a bad framework one.

Migrations holds the application's own SQL migrations. Providers
register and boot the application's services, and Commands add its
console commands. Language holds the translation files, one directory
per locale, loaded over the framework's own English messages.
LocalePreference returns the locale the user or tenant behind a request
picked; it may be nil, and requests then follow Accept-Language.

Broadcast authenticates realtime connections and Permissions lists what
a principal may do; without them every connection is refused and no
permission is held. Domains authorizes hostnames for certificates and
refuses all when nil. WebPushGone hears about push subscriptions the
push service dropped. MaintenanceExcept lists paths that keep working
during maintenance, such as inbound webhooks. RouteRules are the rules
route:lint checks. Module is the application's module path, Stubs its
own stubs and Root the project directory, "." when empty, all for the
make generators. LogWriter receives log
lines instead of standard output. Telemetry adds what telemetry can do,
such as otlp.Tracing() from telemetry/otlp to send spans over OTLP;
tracing backends are opt in so a binary links only the ones it uses.
Clock replaces the system clock, so tests can freeze and move time for
the whole application; production leaves it nil.
*/
type Options struct {
	Name              string
	Version           string
	EnvPrefix         string
	ConfigFiles       []string
	Environ           []string
	Config            any
	Migrations        fs.FS
	Providers         []Provider
	Commands          []Command
	Language          fs.FS
	LocalePreference  middleware.LocalePreference
	Broadcast         broadcasting.Resolver
	Permissions       auth.PermissionResolver
	Domains           customdomain.Authorizer
	WebPushGone       webpush.GoneFunc
	MaintenanceExcept []string
	RouteRules        routing.Rules
	Module            string
	Stubs             fs.FS
	Root              string
	LogWriter         io.Writer
	Clock             clock.Clock
	Telemetry         []telemetry.Option
}

/*
Provider registers and boots one part of the application. Register only
binds, such as declaring routes or listeners; Boot may use any service,
since every provider has registered by then.
*/
type Provider interface {
	/*
		Register binds the provider's parts to the application.
	*/
	Register(a *Application) error

	/*
		Boot starts using the application once every provider registered.
	*/
	Boot(ctx context.Context, a *Application) error
}

/*
Shutdowner is a provider with work to finish before the process exits,
such as flushing buffered writes. Shutdown runs after the workers stop
and before the pools close.
*/
type Shutdowner interface {
	/*
		Shutdown finishes the provider's work, giving up when the context
		ends.
	*/
	Shutdown(ctx context.Context) error
}

/*
Command is an application console command that needs the booted
application. The foundation boots the application the first time such a
command runs and shuts it down afterwards.
*/
type Command interface {
	/*
		Name is what the command is called with, such as "users:create".
	*/
	Name() string

	/*
		Description is the one line shown in the command list.
	*/
	Description() string

	/*
		Flags declares the command's arguments and flags.
	*/
	Flags(flags *console.Flags)

	/*
		Handle runs the command against the application.
	*/
	Handle(ctx context.Context, a *Application, in *console.Input, out *console.Output) error
}
