/*
|--------------------------------------------------------------------------
| Notification Routes
|--------------------------------------------------------------------------
|
| A member's own bell and notification setup inside a workspace, and the
| browsers their user enabled push on, which belong to the user and work
| across workspaces. Tokens need notifications:read or :write; push
| devices and the focus heartbeat come from the person's own browser
| only, never a token. Slack destinations post where the team picked
| and to each teammate's own direct message.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/routing"

	"github.com/dimmihq/dimmi/app"
	v1 "github.com/dimmihq/dimmi/app/http/controllers/api/v1"
)

/*
notificationRoutes declares the notification routes.
*/
func notificationRoutes(api *routing.Router, a *app.App) {
	acts := a.Actions
	if acts.NotificationCenter == nil {
		return
	}
	c := &v1.NotificationController{Center: acts.NotificationCenter, Presence: acts.NotificationPresence, IDs: a.IDs}

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

		routing.Get(r, "/notifications", "notifications.index", c.Index).Ability("notifications:read")
		routing.Patch(r, "/notifications", "notifications.update-many", c.Update).Ability("notifications:write")
		routing.Get(r, "/notification-preferences", "notification-preferences.index", c.Preferences).
			Ability("notifications:read")
		routing.Patch(r, "/notification-preferences", "notification-preferences.update", c.UpdatePreferences).
			Ability("notifications:write")
		routing.Delete(r, "/notification-preferences", "notification-preferences.destroy", c.ResetPreferences).
			Ability("notifications:write")
		routing.Patch(r, "/notification-settings", "notification-settings.update", c.UpdateSettings).
			Ability("notifications:write")
		routing.Post(r, "/notification-tests", "notification-tests.store", c.Test).Ability("notifications:write").
			Status(fhttp.StatusNoContent)
	})

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth:user", "workspace", "access.policy", "api.throttle")

		routing.Post(r, "/notification-focus", "notification-focus.store", c.Focus).Status(fhttp.StatusNoContent)
	})

	pushRoutes(api, a)
	destinationRoutes(api, a)
}

/*
destinationRoutes declares the routes of Slack destinations: the
team's channels, which need notification_destinations:manage to change,
and each teammate's own direct message.
*/
func destinationRoutes(api *routing.Router, a *app.App) {
	if a.Actions.NotificationDestinations == nil {
		return
	}

	d := &v1.NotificationDestinationController{Destinations: a.Actions.NotificationDestinations, IDs: a.IDs}
	one := "/notification-destinations/{destination}"

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth", "workspace", "access.policy", "api.throttle")

		routing.Get(r, "/notification-destinations", "notification-destinations.index", d.Index).
			Ability("notifications:read")
		routing.Post(r, "/notification-destinations", "notification-destinations.store", d.Store).
			Ability("notifications:write").Status(fhttp.StatusCreated)
		routing.Get(r, one, "notification-destinations.show", d.Show).Ability("notifications:read")
		routing.Patch(r, one, "notification-destinations.update", d.Update).Ability("notifications:write")
		routing.Delete(r, one, "notification-destinations.destroy", d.Destroy).Ability("notifications:write").
			Status(fhttp.StatusNoContent)
		routing.Post(r, one+"/tests", "notification-destinations.tests.store", d.Test).
			Ability("notifications:write").Status(fhttp.StatusCreated)
		routing.Get(r, "/integrations/{integration}/slack-channels", "integrations.slack-channels.index",
			d.SlackChannels).Ability("settings:read")
	})
}

/*
pushRoutes declares the routes of a user's push devices.
*/
func pushRoutes(api *routing.Router, a *app.App) {
	p := &v1.PushSubscriptionController{Devices: a.Actions.PushDevices, IDs: a.IDs}

	api.Group("", func(r *routing.Router) {
		r.Middleware("csrf", "session", "auth:user", "api.throttle")

		routing.Get(r, "/push-config", "push-config.show", p.Config)
		routing.Get(r, "/push-subscriptions", "push-subscriptions.index", p.Index)
		routing.Post(r, "/push-subscriptions", "push-subscriptions.store", p.Store).Status(fhttp.StatusCreated)
		routing.Delete(r, "/push-subscriptions/{subscription}", "push-subscriptions.destroy", p.Destroy).
			Status(fhttp.StatusNoContent)
	})
}
