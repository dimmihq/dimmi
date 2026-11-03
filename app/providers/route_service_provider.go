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
| install requires one.
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

	fauth "github.com/dimmihq/dimmi/framework/auth"
	"github.com/dimmihq/dimmi/framework/foundation"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/middleware"
	"github.com/dimmihq/dimmi/framework/view/vite"

	"github.com/dimmihq/dimmi/app"
	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
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

	p.App.Vite = vite.New(vite.Options{
		Build: p.App.Frontend, Base: "/build", HotFile: p.App.Config.Frontend.HotPath, React: true,
	})
	fa.Kernel.Alias("csp", func(...string) (fhttp.Middleware, error) {
		return appmiddleware.ContentSecurityPolicy(p.App.Vite), nil
	})

	widget, err := widgetassets.New(p.App.Widget, p.App.Config.Widget.AssetsBaseURL)
	if err != nil && !errors.Is(err, widgetassets.ErrNoBuild) {
		return fmt.Errorf("widget assets: %w", err)
	}

	p.App.WidgetAssets = widget

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
origin returns the scheme and host of the application's URL, the form
browsers send in Origin. The framework refuses to boot with a URL that
does not parse, so the error is never set here.
*/
func origin(appURL string) string {
	u, _ := url.Parse(appURL)

	return u.Scheme + "://" + u.Host
}
