/*
|--------------------------------------------------------------------------
| Widget Provider
|--------------------------------------------------------------------------
|
| Wires the messenger. Its route middleware is named while the application
| registers; its actions are built once the services they use exist, and the
| directory of widgets starts listening for what other replicas change.
|
*/

package providers

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/foundation"
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/contracts"
	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
	"github.com/dimmihq/dimmi/app/support/entitlements"
	"github.com/dimmihq/dimmi/app/support/widgetinstall"
	"github.com/dimmihq/dimmi/app/support/widgetstrings"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
registerWidget names the messenger's route middleware and registers the
default widget's setup step, which runs after the default inbox's.
*/
func registerWidget(a *app.App, fa *foundation.Application) {
	a.WorkspaceInitializers.Register(&widget.EnsureDefaultWidget{Clock: fa.Clock, Events: fa.Events})

	fa.Kernel.Alias("widget.cors", func(...string) (fhttp.Middleware, error) {
		enabled := func() bool { return a.Config.Widget.Enabled }

		return appmiddleware.WidgetCORS(enabled, fa.Clock, fa.Errors), nil
	})
	fa.Kernel.Alias("widget.app", func(...string) (fhttp.Middleware, error) {
		return appmiddleware.ResolveWidgetApp(a.Actions.Widget.Directory, fa.IDs, fa.Errors), nil
	})
	fa.Kernel.Alias("widget.origin", func(...string) (fhttp.Middleware, error) {
		return appmiddleware.VerifyWidgetOrigin(origin(fa.Config.App.URL), a.Actions.Widget.Install, fa.Errors), nil
	})
}

/*
bootEntitlements answers limits from the catalog of the edition, unless a
test or a cloud driver already set another.
*/
func bootEntitlements(a *app.App) {
	if a.Entitlements != nil {
		return
	}

	catalog := entitlements.SelfHost(a.Config.Widget.MaxWidgetsPerWorkspace)
	if a.Config.App.Edition == appconfig.EditionCloud {
		catalog = entitlements.Cloud()
	}

	a.Entitlements = entitlements.Driver{Catalog: catalog}
}

/*
bootBilling answers that nothing was paid on an install without billing.
*/
func bootBilling(a *app.App) contracts.BillingStatus {
	if a.BillingStatus == nil {
		a.BillingStatus = widget.NeverPaid{}
	}

	return a.BillingStatus
}

/*
bootWidget builds the messenger's actions.
*/
func bootWidget(ctx context.Context, a *app.App) error {
	bootEntitlements(a)

	cfg := a.Config.Widget

	words, err := widgetstrings.Load()
	if err != nil {
		return err
	}

	disk, err := a.Storage.Default()
	if err != nil {
		return fmt.Errorf("widget disk: %w", err)
	}

	directory := widget.NewDirectory(a.DB, a.PubSub, cfg.ConfigCacheTTL)
	branding := &widget.ResolveBranding{Edition: a.Config.App.Edition, Billing: bootBilling(a)}
	install := widgetinstall.New(a.DB, a.Cache, a.Clock)

	a.Actions.Widget = &app.WidgetActions{
		Directory: directory, Strings: words, Branding: branding,
		ShowSettings: &widget.ShowWidgetSettings{DB: a.DB, Branding: branding},
		Create:       &widget.CreateWidget{DB: a.DB, Events: a.Events, Clock: a.Clock, Entitlements: a.Entitlements},
		Update: &widget.UpdateWidgetSettings{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Branding: branding, Bust: directory,
		},
		ShowConfig: &widget.ShowWidgetConfig{
			Directory: directory, Branding: branding, Strings: words, PollingInterval: cfg.PollingInterval.Milliseconds(),
		},
		Availability: &widget.ComputeTeamAvailability{DB: a.DB, Clock: a.Clock, Presence: a.Broadcast, Disk: disk},
		Install:      install,
		Delete: &widget.DeleteWidget{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Bust: directory, Streams: widget.BroadcastStreams{Server: a.Broadcast},
		},
	}

	unsubscribe, err := directory.Listen(context.WithoutCancel(ctx))
	if err != nil {
		return err
	}

	stop := install.Start(context.WithoutCancel(ctx), cfg.InstallFlushInterval)

	a.OnClose("widget directory", func(ctx context.Context) error {
		unsubscribe()

		return stop(ctx)
	})

	return nil
}
