/*
|--------------------------------------------------------------------------
| Slack Webhook Routes
|--------------------------------------------------------------------------
|
| Where Slack delivers events and interactions: the shared addresses for
| the install's own app, and per connection for a workspace's own app,
| each signed with that app's secret. They carry no session and no
| cookie; the signature is checked before anything is read.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/webhooks"
)

/*
slackWebhookRoutes declares the Slack delivery routes.
*/
func slackWebhookRoutes(a *app.App) {
	c := &webhooks.SlackController{
		Slack: a.Actions.Slack.Channel, IDs: a.IDs, MaxBytes: a.Config.Channels.Slack.MaxPayloadBytes,
	}

	a.Router.Handle(fhttp.MethodPost, "/webhooks/slack/events", "webhooks.slack.events",
		fhttp.HandlerFunc(c.Events))
	a.Router.Handle(fhttp.MethodPost, "/webhooks/slack/events/{channel}", "webhooks.slack.connection-events",
		fhttp.HandlerFunc(c.ConnectionEvents))
	a.Router.Handle(fhttp.MethodPost, "/webhooks/slack/interactions", "webhooks.slack.interactions",
		fhttp.HandlerFunc(c.Interactions))
	a.Router.Handle(fhttp.MethodPost, "/webhooks/slack/interactions/{channel}",
		"webhooks.slack.connection-interactions", fhttp.HandlerFunc(c.ConnectionInteractions))
}
