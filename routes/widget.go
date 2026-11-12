/*
|--------------------------------------------------------------------------
| Widget Routes
|--------------------------------------------------------------------------
|
| The API a visitor's browser calls, under /widget/v1, apart from the
| dashboard's API: it has its own contract, because SDKs in customers' apps
| depend on it, and its own credential. No cookie is ever read or set here.
| Public routes name their app and pass the origin check; everything else
| also carries the visitor's session.
|
*/

package routes

import (
	"context"
	"net/url"
	"strings"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/http/useragent"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/widget"
	appmiddleware "github.com/dimmihq/dimmi/app/http/middleware"
)

/*
Widget declares the /widget/v1 routes.
*/
func Widget(a *app.App) {
	install := installOrigin(a.Application.Config.App.URL)

	a.Router.Handle(fhttp.MethodOptions, "/widget/v1/*", "widget.preflight",
		appmiddleware.WidgetPreflight(a.Actions.Widget.Directory, a.IDs, install))

	a.Router.API("/widget/v1", "Dimmi Messenger API", a.Config.API.Version, func(api *routing.Router) {
		api.Middleware("widget.cors")

		api.Group("", func(public *routing.Router) {
			public.Middleware("widget.app", "widget.origin")
			widgetAppRoutes(public, a)
		})
		api.Group("", func(visitor *routing.Router) {
			visitor.Middleware("widget.session", "widget.origin")
			widgetSessionRoutes(visitor, a)
			widgetAttachmentRoutes(visitor, a)
		})
		api.Group("", func(reviving *routing.Router) {
			reviving.Middleware("widget.session:reviving", "widget.origin")
			routing.Patch(reviving, "/session", "widget.session.update", sessionController(a).Update)
		})
		api.Group("", func(leaving *routing.Router) {
			leaving.Middleware("widget.session:ending", "widget.origin")
			routing.Delete(leaving, "/session", "widget.session.destroy", sessionController(a).Destroy).
				Status(fhttp.StatusNoContent)
		})
	})
}

/*
widgetAppRoutes declares the routes that name their app.
*/
func widgetAppRoutes(public *routing.Router, a *app.App) {
	acts := a.Actions.Widget
	base := baseURLOf(a)

	config := &widget.ShowWidgetConfigController{Show: acts.ShowConfig, BaseURL: base}
	availability := &widget.ShowWidgetAvailabilityController{
		Compute: acts.Availability, Strings: acts.Strings, Clock: a.Clock,
	}

	routing.Get(public, "/apps/{app}/config", "widget.apps.config.show", config.Invoke)
	routing.Get(public, "/apps/{app}/availability", "widget.apps.availability.show", availability.Invoke)
	routing.Get(public, "/apps/{app}/help", "widget.apps.help.show", helpController(a).Show)
	routing.Get(public, "/apps/{app}/changelog", "widget.apps.changelog.show", changelogController(a).Show)
	routing.Post(public, "/apps/{app}/article-views", "widget.apps.article-views.store", helpController(a).View).
		Status(fhttp.StatusNoContent)

	sessions := sessionController(a)
	routing.Post(public, "/sessions", "widget.sessions.store", sessions.Store)

	widgetAppTicketRoutes(public, a)
}

/*
widgetSessionRoutes declares the routes a visitor's session names.
*/
func widgetSessionRoutes(visitor *routing.Router, a *app.App) {
	sessions := sessionController(a)

	routing.Get(visitor, "/session", "widget.session.show", sessions.Show)

	contact := &widget.ContactController{DB: a.DB, Update: a.Actions.Widget.UpdateContact}
	routing.Get(visitor, "/contact", "widget.contact.show", contact.Show)
	routing.Patch(visitor, "/contact", "widget.contact.update", contact.Patch)

	proofs := &widget.EmailProofController{Proofs: a.Actions.Widget.EmailProofs}
	routing.Post(visitor, "/email-verifications", "widget.email-verifications.store", proofs.Store)
	routing.Post(visitor, "/email-verifications/checks", "widget.email-verifications.checks.store", proofs.Check).
		Status(fhttp.StatusOK)

	widgetHelpRoutes(visitor, a)

	tokens := &widget.RealtimeTokenController{Realtime: a.Actions.Widget.Realtime, BaseURL: baseURLOf(a)}
	routing.Post(visitor, "/realtime-tokens", "widget.realtime-tokens.store", tokens.Store)

	updates := &widget.UpdateController{
		List: a.Actions.Widget.Updates, Cursors: pagination.NewCursors(a.DerivedKey("widget.cursors")), Clock: a.Clock,
		Interval: a.Config.Widget.PollingInterval,
	}
	routing.Get(visitor, "/updates", "widget.updates.index", updates.Index)

	pings := &widget.PingController{Record: a.Actions.Widget.Ping}
	routing.Post(visitor, "/pings", "widget.pings.store", pings.Store)

	one := "/conversations/{conversation}"
	conv := widgetConversationRoutes(visitor, a, one)

	activity := &widget.ActivityController{Read: a.Actions.Widget.MarkRead, Tell: a.Actions.Widget.Typing, IDs: a.IDs}
	routing.Patch(visitor, one+"/read-state", "widget.conversations.read-state.update", activity.ReadState)
	routing.Post(visitor, one+"/typing-indicators", "widget.conversations.typing-indicators.store", activity.Typing)

	routing.Post(visitor, one+"/ai/helped", "widget.conversations.ai.helped", conv.Helped)
	routing.Post(visitor, one+"/ai/handoff", "widget.conversations.ai.handoff", conv.Handoff).Middleware("idempotent")

	widgetActionCallRoutes(visitor, a, one)
	widgetTicketRoutes(visitor, a)
	widgetEventRoutes(visitor, a)
	widgetRatingRoutes(visitor, a, one)
}

/*
widgetConversationRoutes declares the visitor's conversations and the
messages in them, under one, and returns their controller for the
routes that follow.
*/
func widgetConversationRoutes(visitor *routing.Router, a *app.App, one string) *widget.ConversationController {
	conv := &widget.ConversationController{
		Read: a.Actions.Widget.Conversations, Post: a.Actions.Widget.PostMessage,
		Help: a.Actions.Widget.ConfirmHelp, HandoffAction: a.Actions.Widget.RequestHandoff,
		IDs: a.IDs, Clock: a.Clock, Tickets: a.Actions.Tickets,
		Cursors: pagination.NewCursors(a.DerivedKey("widget.cursors")), Agents: useragent.New(),
	}

	routing.Get(visitor, "/conversations", "widget.conversations.index", conv.Index)
	routing.Post(visitor, "/conversations", "widget.conversations.store", conv.Store).Middleware("idempotent")
	routing.Get(visitor, one, "widget.conversations.show", conv.Show)
	routing.Get(visitor, one+"/messages", "widget.conversations.messages.index", conv.Messages)
	routing.Post(visitor, one+"/messages", "widget.conversations.messages.store", conv.Send).Middleware("idempotent")

	return conv
}

/*
widgetActionCallRoutes declares the visitor's confirmation cards: read
one, and confirm or cancel it.
*/
func widgetActionCallRoutes(visitor *routing.Router, a *app.App, one string) {
	if a.Actions.AI == nil || a.Actions.AI.VisitorCalls == nil {
		return
	}
	calls := &widget.ActionCallController{Calls: a.Actions.AI.VisitorCalls, IDs: a.IDs}
	routing.Get(visitor, one+"/action-calls/{call}", "widget.conversations.action-calls.show", calls.Show)
	routing.Patch(visitor, one+"/action-calls/{call}", "widget.conversations.action-calls.update", calls.Update)
}

/*
sessionController returns the controller of visitors' sessions.
*/
func sessionController(a *app.App) *widget.SessionController {
	acts := a.Actions.Widget

	return &widget.SessionController{
		Start: acts.StartSession, Describe: acts.ShowSession, Revoke: acts.RevokeSession, Surfaces: acts.Surfaces,
		Clock: a.Clock, BaseURL: baseURLOf(a),
	}
}

/*
widgetAttachmentRoutes declares the three steps of a visitor's upload. The
bytes route is a plain handler, so the body streams to the disk.
*/
func widgetAttachmentRoutes(visitor *routing.Router, a *app.App) {
	files := &widget.AttachmentController{
		Uploads: a.Actions.Widget.Uploads, IDs: a.IDs, Errors: a.Errors, BaseURL: baseURLOf(a),
	}
	one := "/attachments/{attachment}"

	routing.Post(visitor, "/attachments", "widget.attachments.store", files.Store).Status(fhttp.StatusCreated)
	routing.Patch(visitor, one, "widget.attachments.update", files.Update)

	content := fhttp.HandlerFunc(files.Content)
	visitor.Handle(fhttp.MethodPut, one+"/content", "widget.attachments.content.update", content)
}

/*
baseURLOf returns how to find the base address a request reached this install
on, which is the configured one until a custom domain says otherwise.
*/
func baseURLOf(a *app.App) func(ctx context.Context) string {
	base := strings.TrimRight(a.Application.Config.App.URL, "/")

	return func(context.Context) string { return base }
}

/*
installOrigin returns this install's own origin, the form browsers send.
*/
func installOrigin(appURL string) string {
	u, err := url.Parse(appURL)
	if err != nil {
		return ""
	}

	return u.Scheme + "://" + u.Host
}

/*
widgetHelpRoutes declares the Help tab as the visitor reads it, the
passes that open the help center signed in, the visitor's answers
about articles and the articles they open, counted.
*/
func widgetHelpRoutes(visitor *routing.Router, a *app.App) {
	help := helpController(a)
	routing.Get(visitor, "/help", "widget.help.show", help.Mine)
	routing.Post(visitor, "/help-passes", "widget.help-passes.store", help.Pass)
	routing.Post(visitor, "/article-ratings", "widget.article-ratings.store", help.Rate).
		Status(fhttp.StatusNoContent)
	routing.Post(visitor, "/article-views", "widget.article-views.store", help.View).
		Status(fhttp.StatusNoContent)
}

/*
helpController serves the messenger's Help tab.
*/
func helpController(a *app.App) *widget.HelpController {
	return &widget.HelpController{
		Public: a.Actions.HelpPublic, Readers: a.Actions.HelpReaders, Ratings: a.Actions.HelpRatings,
		Addresses: a.Actions.HelpCenterAddresses, Clock: a.Clock, Analytics: a.Actions.HelpAnalytics,
	}
}

/*
changelogController serves the messenger's "What's new" tab.
*/
func changelogController(a *app.App) *widget.ChangelogController {
	return &widget.ChangelogController{
		Public: a.Actions.HelpPublic, Changelog: a.Actions.ChangelogPublic, Addresses: a.Actions.HelpCenterAddresses,
	}
}
