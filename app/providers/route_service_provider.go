/*
|--------------------------------------------------------------------------
| Route Service Provider
|--------------------------------------------------------------------------
|
| Names the product's route middleware, links the browser app's scripts,
| loads the messenger widget's build, and declares the routes once the
| actions exist. A binary without a widget build still boots; a broken
| build stops it, since every customer's page would fail. Cross-origin protection
| trusts the application's own origin and the extra origins configured
| for split deployments, such as a dashboard on another host. Sensitive routes ask for a recent
| confirmation of who is calling, within the configured window, and
| steps that reach other people ask for a proven email where the
| install requires one. Workspace routes run inside the workspace the
| request names, resolved once the caller is known, and only for callers
| who meet what that workspace requires, such as a second factor.
|
| A request picks its language before anyone is known, from the browser.
| Once authentication, and then the workspace, tell who is calling, the
| language is picked again, so the person's own choice and then their
| workspace's win over the browser's on every authenticated route.
|
| The route files sit outside the application package, since routes
| depend on the application and never the other way around, so the
| function that declares them is handed in.
|
*/

package providers

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/foundation"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/telemetry"
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app"
	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
	"github.com/dimmihq/dimmi/app/support/helpsample"
	"github.com/dimmihq/dimmi/app/support/widgetassets"
)

/*
RouteServiceProvider declares the routes. Routes declares them on the
booted application.
*/
type RouteServiceProvider struct {
	App    *app.App
	Routes func(a *app.App)
}

/*
Register names the product's route middleware.
*/
func (p RouteServiceProvider) Register(fa *foundation.Application) error {
	trusted := append([]string{origin(fa.Config.App.URL)}, p.App.Config.HTTP.TrustedOrigins...)

	csrf, err := middleware.CSRF(trusted...)
	if err != nil {
		return fmt.Errorf("cross-origin protection: %w", err)
	}

	fa.Kernel.Alias("csrf", func(...string) (fhttp.Middleware, error) { return csrf, nil })
	fa.Kernel.Alias("password.confirm", func(...string) (fhttp.Middleware, error) {
		return fauth.RequireConfirmation(fa.Clock, p.App.Config.Auth.PasswordConfirmation.TTL, fa.Errors), nil
	})
	fa.Kernel.Alias("verified", func(...string) (fhttp.Middleware, error) {
		return appmiddleware.EnsureEmailVerified(p.App.Config.Auth.Verification, fa.Errors), nil
	})
	p.aliasCallers(fa)
	p.aliasLimits(fa)
	fa.Kernel.Alias("access.policy", func(...string) (fhttp.Middleware, error) {
		return appmiddleware.EnforceAccessPolicy(p.App.AccessPolicy, fa.Clock, fa.Errors), nil
	})

	p.App.Vite = vite.New(vite.Options{
		Build: p.App.Frontend, Base: "/build", HotFile: p.App.Config.Frontend.HotPath, React: true,
	})
	fa.Kernel.Alias("csp", func(params ...string) (fhttp.Middleware, error) {
		return appmiddleware.ContentSecurityPolicy(p.App.Vite, p.pageSources(params)), nil
	})

	widget, err := widgetassets.New(p.App.Widget, p.App.Config.Widget.AssetsBaseURL)
	if err != nil && !errors.Is(err, widgetassets.ErrNoBuild) {
		return fmt.Errorf("widget assets: %w", err)
	}

	p.App.WidgetAssets = widget

	sample, err := helpsample.Load()
	if err != nil {
		return fmt.Errorf("sample help center: %w", err)
	}

	p.App.HelpSample = sample

	return nil
}

/*
Boot declares the routes.
*/
func (p RouteServiceProvider) Boot(context.Context, *foundation.Application) error {
	p.Routes(p.App)

	return nil
}

/*
aliasCallers names the middleware that learn who is calling: auth, and
workspace for the workspace they work in. Each picks the request's
language again once it knows more.
*/
func (p RouteServiceProvider) aliasCallers(fa *foundation.Application) {
	locale := middleware.Locale(fa.Lang, appmiddleware.LocalePreference(fa.Lang))
	slow := appmiddleware.SlowBadCredentials(appmiddleware.BadCredentialsOptions{
		Limiter: fa.Limiter, PerMinute: p.App.Config.API.Limits.BadCredentialPerMinute,
		Cookie: fa.Session.CookieName(), Clock: fa.Clock, Errors: fa.Errors,
	})

	fa.Kernel.Alias("auth", func(params ...string) (fhttp.Middleware, error) {
		guard, err := fa.Guards.Middleware(params...)
		if err != nil {
			return nil, fmt.Errorf("auth middleware: %w", err)
		}

		return then(slow, then(guard, locale)), nil
	})
	fa.Kernel.Alias("workspace", func(...string) (fhttp.Middleware, error) {
		return then(appmiddleware.ResolveWorkspace(p.App.Actions.ResolveWorkspace, fa.Errors), locale), nil
	})
}

/*
aliasLimits names api.throttle, the public API's fairness limits, which
run after the caller and the workspace are known. Every refusal is
counted by scope.
*/
func (p RouteServiceProvider) aliasLimits(fa *foundation.Application) {
	throttled, err := fa.Telemetry.Counter("dimmi_api_throttled")

	fa.Kernel.Alias("api.throttle", func(...string) (fhttp.Middleware, error) {
		return appmiddleware.APIThrottle(appmiddleware.APIThrottleOptions{
			Limiter: fa.Limiter, Entitlements: p.App.Entitlements, Limits: p.App.Config.API.Limits,
			Clock: fa.Clock, Refused: func(ctx context.Context, scope string) {
				if err == nil {
					throttled.Add(ctx, 1, telemetry.String("scope", scope))
				}
			},
		}), nil
	})
}

/*
then runs the second middleware inside the first, once the first let
the request through.
*/
func then(first, second fhttp.Middleware) fhttp.Middleware {
	return func(next fhttp.Handler) fhttp.Handler {
		return fhttp.Chain(next, first, second)
	}
}

/*
origin returns the scheme and host of the application's URL, the form
browsers send in Origin. The framework refuses to boot with a URL that
does not parse, so the error is never set here.
*/
func origin(appURL string) string {
	u, _ := url.Parse(appURL)

	return u.Scheme + "://" + u.Host
}

/*
messengerSources returns the origins a help center on its own host loads
the messenger from: the app, which serves its loader and answers its
requests, and the host of its files when that is another. None when
help centers are served under the app's own address with the files.
*/
func messengerSources(a *app.App) []string {
	var out []string
	if a.Config.HelpCenters.BaseDomain != "" {
		out = append(out, origin(a.Application.Config.App.URL))
	}
	if a.Config.Widget.AssetsBaseURL != "" {
		out = append(out, origin(a.Config.Widget.AssetsBaseURL))
	}
	return out
}

/*
pageSources returns the extra sources of the pages a csp middleware
covers: every page may load the captcha provider when one is on and the
messenger from wherever it is served, and the dashboard also reaches
GIPHY when the instance has a GIF search key.
*/
func (p RouteServiceProvider) pageSources(params []string) appmiddleware.Sources {
	extra := captchaSources(p.App)
	messenger := messengerSources(p.App)
	extra.Scripts = append(extra.Scripts, messenger...)
	extra.Connect = append(extra.Connect, messenger...)
	extra.Images = append(extra.Images, messenger...)
	if !slices.Contains(params, "dashboard") || p.App.Config.Inbox.GIF.GiphyAPIKey == "" {
		return extra
	}

	extra.Connect = append(extra.Connect, "https://api.giphy.com")
	extra.Images = append(extra.Images, "https://*.giphy.com")

	return extra
}
