/*
|--------------------------------------------------------------------------
| Boot: Identity, HTTP and Providers
|--------------------------------------------------------------------------
|
| The last services are the ones requests go through: who is calling,
| what they may do, their session, and the router and kernel that carry
| a request to its handler. Every request is traced, so its database
| queries and outbound calls appear under one trace. Then the
| application's providers register and boot, with every service ready.
|
*/

package foundation

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"

	"github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/customdomain"
	"github.com/dimmihq/dimmi/framework/encryption"
	"github.com/dimmihq/dimmi/framework/health"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/idempotency"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/i18n"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/routing"
	"github.com/dimmihq/dimmi/framework/session"
	"github.com/dimmihq/dimmi/framework/socialite"
	"github.com/dimmihq/dimmi/framework/validation"
)

/*
urlSigningPurpose derives the URL signing key from the app key.
*/
const urlSigningPurpose = "url-signing"

/*
bootWeb builds identity, validation, the HTTP kernel, the router and
the health checks.
*/
func (a *Application) bootWeb(context.Context) error {
	var err error

	a.IDs = ids.NewRegistry()

	a.Validator, err = validation.New(validation.WithTranslator(a.Lang))
	if err != nil {
		return fmt.Errorf("build validator: %w", err)
	}

	a.Languages = i18n.NewDetectorManager(a.Config.Languages)
	a.Errors = fhttp.Errors{Report: a.Telemetry.Report}
	a.bootIdentity()
	a.registerWebDrivers()

	if err := a.bootKernel(); err != nil {
		return err
	}

	a.Health = health.NewRegistry(health.DefaultTimeout, a.Clock)
	a.Health.Add(health.Database(a.DB), health.Valkey(a.Redis), health.Func("maintenance", a.maintenanceCheck))

	return nil
}

/*
bootIdentity builds the guards, the gate, sessions, social sign-in,
custom domains and the idempotency store.
*/
func (a *Application) bootIdentity() {
	permissions := a.opts.Permissions
	if permissions == nil {
		permissions = auth.PermissionsFunc(func(context.Context, auth.Principal) ([]string, error) {
			return nil, nil
		})
	}

	a.Guards = auth.NewGuards(a.Errors)
	a.Gate = auth.NewGate(permissions, a.Errors)

	sessionConfig := a.Config.Session
	sessionConfig.Errors = a.Errors
	a.Session = session.NewManager(session.NewPostgresStore(a.DB, session.DefaultTable), a.Clock, sessionConfig)

	a.Social = socialite.NewManager(a.Config.Socialite, a.Cache)
	a.Domains = customdomain.NewManager(a.Config.CustomDomains)
	a.Idempotency = idempotency.NewDatabaseStore(a.DB, a.Config.Idempotency, a.Clock)
}

/*
bootKernel builds the HTTP kernel with its global middleware and
aliases, the router and the URL generator. Every request gets an id and
a locale, recovers from panics, carries the trace its caller started
and is refused during maintenance.
*/
func (a *Application) bootKernel() error {
	a.Kernel = fhttp.NewKernel()
	a.Kernel.Use(
		middleware.RequestID(""),
		middleware.Locale(a.Lang, a.opts.LocalePreference),
		middleware.Recover(a.Errors),
		otelhttp.NewMiddleware(a.Name,
			otelhttp.WithTracerProvider(a.Telemetry.TracerProvider()),
			otelhttp.WithMeterProvider(a.Telemetry.MeterProvider()),
			otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{}, propagation.Baggage{},
			)),
		),
	)

	if len(a.Config.HTTP.TrustedProxies) > 0 {
		trust, err := middleware.TrustProxies(a.Config.HTTP.TrustedProxies, "")
		if err != nil {
			return fmt.Errorf("trust proxies: %w", err)
		}

		a.Kernel.Use(trust)
	}

	a.Kernel.Use(middleware.Maintenance(a.Maintenance, a.Config.Maintenance.RetryAfter, a.opts.MaintenanceExcept...))

	a.Router = routing.New(routing.Options{Middleware: a.Kernel, Errors: a.Errors, Validator: a.Validator})

	signing, err := encryption.DeriveKey(a.appKey, urlSigningPurpose)
	if err != nil {
		return fmt.Errorf("derive url signing key: %w", err)
	}

	a.URL = routing.NewURL(a.Router, a.Config.App.URL, signing, a.Clock)
	a.aliasMiddleware()

	return nil
}

/*
aliasMiddleware names the built-in middleware routes ask for: session,
auth, can, signed, csrf, idempotent and locale. Every request already
has a locale; "locale" after "auth" picks it again once the user, and
so their preference, is known.
*/
func (a *Application) aliasMiddleware() {
	a.Kernel.Alias("locale", func(...string) (fhttp.Middleware, error) {
		return middleware.Locale(a.Lang, a.opts.LocalePreference), nil
	})
	a.Kernel.Alias("session", func(...string) (fhttp.Middleware, error) { return a.Session.Middleware, nil })
	a.Kernel.Alias("auth", a.Guards.Middleware)
	a.Kernel.Alias("can", a.Gate.Middleware)
	a.Kernel.Alias("signed", func(...string) (fhttp.Middleware, error) { return middleware.Signed(a.URL), nil })
	a.Kernel.Alias("csrf", middleware.CSRF)
	a.Kernel.Alias("idempotent", func(...string) (fhttp.Middleware, error) {
		return middleware.Idempotent(a.Idempotency, middleware.DefaultIdempotencyHeader, nil), nil
	})
}

/*
maintenanceCheck warns while the application is down, which keeps the
node ready: load balancers keep sending traffic, and it is answered
with the maintenance response.
*/
func (a *Application) maintenanceCheck(ctx context.Context) health.Result {
	state, err := a.Maintenance.State(ctx)
	if err != nil {
		return health.Result{Status: health.Warn, Detail: err.Error()}
	}

	if state.Enabled {
		return health.Result{Status: health.Warn, Detail: "down for maintenance"}
	}

	return health.Result{Status: health.Pass}
}

/*
bootProviders registers every provider, then boots every provider, in
the order the application listed them. Providers with work to finish
are shut down before the pools close.
*/
func (a *Application) bootProviders(ctx context.Context) error {
	for _, provider := range a.opts.Providers {
		if err := provider.Register(a); err != nil {
			return fmt.Errorf("register provider %T: %w", provider, err)
		}
	}

	for _, provider := range a.opts.Providers {
		if err := provider.Boot(ctx, a); err != nil {
			return fmt.Errorf("boot provider %T: %w", provider, err)
		}

		if shutdowner, ok := provider.(Shutdowner); ok {
			a.onClose(fmt.Sprintf("provider %T", provider), shutdowner.Shutdown)
		}
	}

	return nil
}

/*
domainAuthorizer returns the hostname authorizer. The host of app.url
always gets a certificate, since the application serves it; any other
hostname is up to the application's authorizer, and refused without one.
*/
func (a *Application) domainAuthorizer() customdomain.Authorizer {
	own := appHost(a.Config.App.URL)

	return func(ctx context.Context, host string) error {
		if own != "" && host == own {
			return nil
		}

		if a.opts.Domains != nil {
			return a.opts.Domains(ctx, host)
		}

		return customdomain.ErrNotAuthorized
	}
}

/*
appHost returns the normalized host of the application's URL, or "" when
it is not served over HTTPS and so needs no certificate.
*/
func appHost(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme != "https" {
		return ""
	}

	host, err := customdomain.Normalize(parsed.Hostname())
	if err != nil {
		return ""
	}

	return host
}

/*
frameworkLanguage returns the framework's own English messages: date
layouts, validation messages and error messages.
*/
func frameworkLanguage() []fs.FS {
	return []fs.FS{i18n.Messages(), validation.Messages(), fhttp.Messages()}
}

/*
languageFiles returns the translation files, or an empty directory.
*/
func languageFiles(files fs.FS) fs.FS {
	if files != nil {
		return files
	}

	return emptyFS{}
}

/*
emptyFS is a directory with nothing in it.
*/
type emptyFS struct{}

/*
Open finds nothing.
*/
func (emptyFS) Open(name string) (fs.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}
