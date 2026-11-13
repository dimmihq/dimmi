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
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/events"
	"github.com/dimmihq/dimmi/framework/filesystem"
	"github.com/dimmihq/dimmi/framework/foundation"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/attachments"
	"github.com/dimmihq/dimmi/app/actions/conversations"
	"github.com/dimmihq/dimmi/app/actions/helpcenters"
	"github.com/dimmihq/dimmi/app/actions/identity"
	"github.com/dimmihq/dimmi/app/actions/widget"
	"github.com/dimmihq/dimmi/app/contracts"
	appevents "github.com/dimmihq/dimmi/app/events"
	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
	widgetchannel "github.com/dimmihq/dimmi/app/integrations/widget"
	"github.com/dimmihq/dimmi/app/support/entitlements"
	"github.com/dimmihq/dimmi/app/support/widgetinstall"
	"github.com/dimmihq/dimmi/app/support/widgetstrings"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
registerWidget names the messenger's route middleware and registers the
default widget's setup step, which runs after the default inbox's, then
the default help center's, which the widget then shows.
*/
func registerWidget(a *app.App, fa *foundation.Application) {
	a.WorkspaceInitializers.Register(&widget.EnsureDefaultWidget{Clock: fa.Clock, Events: fa.Events})
	a.WorkspaceInitializers.Register(&helpcenters.HelpCenters{Clock: fa.Clock})

	fa.Kernel.Alias("widget.cors", func(...string) (fhttp.Middleware, error) {
		enabled := func() bool { return a.Config.Widget.Enabled }

		return appmiddleware.WidgetCORS(enabled, fa.Clock, fa.Errors), nil
	})
	fa.Kernel.Alias("widget.app", func(...string) (fhttp.Middleware, error) {
		return appmiddleware.ResolveWidgetApp(a.Actions.Widget.Directory, fa.IDs, fa.Errors), nil
	})
	fa.Kernel.Alias("widget.session", func(args ...string) (fhttp.Middleware, error) {
		if slices.Contains(args, "reviving") {
			return appmiddleware.AuthenticateWidgetReviving(a.Actions.Widget.Authenticate, fa.Errors), nil
		}

		if slices.Contains(args, "ending") {
			return appmiddleware.AuthenticateWidgetEnding(a.Actions.Widget.Authenticate, fa.Errors), nil
		}

		return appmiddleware.AuthenticateWidget(a.Actions.Widget.Authenticate, fa.Errors), nil
	})
	fa.Kernel.Alias("widget.origin", func(...string) (fhttp.Middleware, error) {
		help := helpcenters.HelpOrigins{DB: a.DB, Addresses: a.Actions.HelpCenterAddresses}
		install := origin(fa.Config.App.URL)
		return appmiddleware.VerifyWidgetOrigin(install, a.Actions.Widget.Install, help, fa.Errors), nil
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
openers answers whether a workspace greets visitors as they open a chat,
asking the engine at the moment a messenger loads: an install that runs
no workflows, and one that builds its messenger before its engine, both
answer no rather than failing.
*/
func openers(a *app.App) widget.OpenerFunc {
	return func(ctx context.Context, workspace ids.UUID) (bool, error) {
		engine := a.Actions.WorkflowEngine
		if engine == nil {
			return false, nil
		}

		return engine.HasOpener(ctx, workspace)
	}
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
	show := &widget.ShowWidgetConfig{
		Directory: directory, Surfaces: &widget.Surfaces{DB: a.DB, Clock: a.Clock, Bust: directory}, Branding: branding,
		Strings: words, Openers: openers(a), PollingInterval: cfg.PollingInterval.Milliseconds(),
	}

	a.Actions.Widget = &app.WidgetActions{
		Directory: directory, Strings: words, Branding: branding,
		ShowSettings: &widget.ShowWidgetSettings{DB: a.DB, Branding: branding},
		Create:       &widget.CreateWidget{DB: a.DB, Events: a.Events, Clock: a.Clock, Entitlements: a.Entitlements},
		Update: &widget.UpdateWidgetSettings{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Branding: branding, Bust: directory,
		},
		ShowConfig:   show,
		Preview:      &widget.PreviewWidgetConfig{DB: a.DB, Show: show},
		Availability: &widget.ComputeTeamAvailability{DB: a.DB, Clock: a.Clock, Presence: a.Broadcast, Disk: disk},
		Install:      widgetinstall.New(a.DB, a.Cache, a.Clock),
		Delete: &widget.DeleteWidget{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Bust: directory,
			Streams: widget.BroadcastStreams{Server: a.Broadcast},
		},
	}

	if err := a.Channels.Register(widgetchannel.Channel{}); err != nil {
		return err
	}

	bootWidgetSessions(a)

	if err := bootWidgetRealtime(a); err != nil {
		return err
	}

	bootWidgetConversations(a, disk, a.Application.Config.Filesystems.Default)

	return startWidget(ctx, a)
}

/*
bootWidgetSessions builds the actions on visitors' sessions.
*/
func bootWidgetSessions(a *app.App) {
	forgetOnWorkspaceChange(a)
	acts, cfg := a.Actions.Widget, a.Config.Widget
	streams := widget.BroadcastStreams{Server: a.Broadcast}
	acts.Surfaces = acts.ShowConfig.Surfaces

	acts.Authenticate = &widget.AuthenticateWidgetSession{
		DB: a.DB, Clock: a.Clock, Directory: acts.Directory, AnonymousTTL: cfg.AnonymousSessionTTL,
	}
	verifier := bootIdentityVerifier(a)
	acts.StartSession = &widget.StartWidgetSession{
		DB: a.DB, Events: a.Events, Clock: a.Clock, AnonymousTTL: cfg.AnonymousSessionTTL,
		IdentifiedTTL: a.Config.Identity.SessionTTL, Identity: verifier, Streams: streams,
		Outcomes: a.Actions.IdentityStats,
	}
	a.Actions.IdentitySetup = &identity.Setup{
		DB: a.DB, Clock: a.Clock, Widgets: &widget.ForgetWorkspaceWidgets{DB: a.DB, Bust: acts.Directory},
	}
	acts.ShowSession = &widget.ShowWidgetSession{DB: a.DB}
	acts.RevokeSession = &widget.RevokeWidgetSession{DB: a.DB, Clock: a.Clock, Streams: streams}
	acts.UpdateContact = &widget.UpdateWidgetContact{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.EmailProofs = &widget.EmailProofs{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Cache: a.Cache, Limiter: a.Limiter, Mail: a.Mail,
	}
	acts.Contacts = &widget.ContactSessions{Clock: a.Clock, Streams: streams}
}

/*
bootWidgetRealtime builds the visitors' stream tokens and lets the realtime
server open their streams.
*/
func bootWidgetRealtime(a *app.App) error {
	key, err := widget.NewRealtimeKey(a.DerivedKey("widget.realtime"))
	if err != nil {
		return fmt.Errorf("widget realtime key: %w", err)
	}

	a.Actions.Widget.Realtime = &widget.Realtime{
		DB: a.DB, Clock: a.Clock, IDs: a.IDs, Key: key, TTL: a.Config.Widget.RealtimeTokenTTL,
		MaxStreams: a.Config.Widget.MaxConnectionsPerSession,
	}

	if a.Realtime != nil {
		a.Realtime.Widget = a.Actions.Widget.Realtime
	}

	return nil
}

/*
bootWidgetConversations builds the actions on visitors' conversations,
which enter the inbox through its inbound path on the widget's disk.
*/
func bootWidgetConversations(a *app.App, disk filesystem.Filesystem, diskName string) {
	acts := a.Actions.Widget
	limits := &widget.VisitorLimits{Limiter: a.Limiter}

	acts.Conversations = &widget.ListWidgetConversations{DB: a.DB, Disk: disk}
	acts.Uploads = &widget.Uploads{
		Create:   &attachments.CreatePendingUpload{DB: a.DB, Clock: a.Clock, Disk: disk, DiskName: diskName},
		Put:      &attachments.PutUploadContent{DB: a.DB, Disk: disk},
		Complete: &attachments.CompleteUpload{DB: a.DB, Clock: a.Clock, Disk: disk, Queue: a.Queue},
		Disk:     disk,
		Limits:   limits,
	}
	acts.MarkRead = &widget.MarkWidgetConversationRead{DB: a.DB, Events: a.Events, Clock: a.Clock}
	acts.ConfirmHelp = &widget.ConfirmAiHelp{DB: a.DB, Clock: a.Clock}
	acts.RequestHandoff = &widget.RequestAiHandoff{DB: a.DB, Events: a.Events, Clock: a.Clock, Limiter: a.Limiter}
	acts.Ping = &widget.RecordWidgetPing{DB: a.DB, Clock: a.Clock, Cache: a.Cache}
	acts.Updates = &widget.ListWidgetUpdates{DB: a.DB, Clock: a.Clock, Read: acts.Conversations}

	broadcaster := &widgetchannel.Broadcaster{
		Server: a.Broadcast, Payloads: widgetchannel.Payloads{DB: a.DB, Read: acts.Conversations},
	}
	broadcaster.Listen(a.Events)
	acts.Typing = &widget.BroadcastVisitorTyping{
		DB: a.DB, Events: a.Events, Clock: a.Clock, Cache: a.Cache, Throttle: a.Config.Widget.TypingThrottle,
	}
	acts.OpenChat = &widget.OpenWidgetConversation{
		DB: a.DB, Clock: a.Clock, Events: a.Events, Surfaces: acts.Surfaces, Limits: limits,
	}
	acts.PostMessage = &widget.PostWidgetMessage{
		DB: a.DB, Clock: a.Clock, Team: acts.Availability, Surfaces: acts.Surfaces, Limits: limits,
		Receiver: &conversations.ReceiveInboundMessage{
			DB: a.DB, Events: a.Events, Clock: a.Clock, Disk: disk, DiskName: diskName, Queue: a.Queue,
			ReopenWindow: a.Config.Channels.Email.Inbound.ReopenWindow,
		},
	}
}

/*
forgetOnWorkspaceChange drops a workspace's widgets from every directory
once a change to its name or logo commits, since messengers show both.
*/
func forgetOnWorkspaceChange(a *app.App) {
	forget := &widget.ForgetWorkspaceWidgets{DB: a.DB, Bust: a.Actions.Widget.Directory}
	events.Listen(a.Events, events.AfterCommit, func(ctx context.Context, e appevents.WorkspaceUpdated) error {
		return forget.Handle(ctx, e.WorkspaceID, e.Changes)
	})
}

/*
startWidget starts hearing other replicas' widget changes and flushing what
the install tracker and the sign-in counts saw, and stops them, flushing
once more, on shutdown.
*/
func startWidget(ctx context.Context, a *app.App) error {
	acts := a.Actions.Widget

	unsubscribe, err := acts.Directory.Listen(context.WithoutCancel(ctx))
	if err != nil {
		return err
	}

	stop := acts.Install.Start(context.WithoutCancel(ctx), a.Config.Widget.InstallFlushInterval)
	stopStats := a.Actions.IdentityStats.Start(context.WithoutCancel(ctx), a.Config.Widget.InstallFlushInterval)

	a.OnShutdown("widget directory", func(ctx context.Context) error {
		unsubscribe()

		return errors.Join(stop(ctx), stopStats(ctx))
	})

	return nil
}
