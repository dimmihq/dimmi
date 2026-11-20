/*
|--------------------------------------------------------------------------
| WhatsApp Webhook Routes
|--------------------------------------------------------------------------
|
| Where Meta checks the webhook address and delivers WhatsApp changes:
| the shared address for the install's own Meta app, and per connection
| for a workspace's own app, each signed with that app's secret. They
| carry no session and no cookie; the signature is checked before
| anything is read.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/webhooks"
)

/*
whatsappWebhookRoutes declares the WhatsApp delivery routes.
*/
func whatsappWebhookRoutes(a *app.App) {
	c := &webhooks.WhatsAppController{
		WhatsApp: a.Actions.WhatsApp, IDs: a.IDs, MaxBytes: a.Config.Channels.WhatsApp.MaxPayloadBytes,
	}

	a.Router.Handle(fhttp.MethodGet, "/webhooks/whatsapp", "webhooks.whatsapp.check", fhttp.HandlerFunc(c.Check))
	a.Router.Handle(fhttp.MethodPost, "/webhooks/whatsapp", "webhooks.whatsapp.events", fhttp.HandlerFunc(c.Events))
	a.Router.Handle(fhttp.MethodGet, "/webhooks/whatsapp/{channel}", "webhooks.whatsapp.connection-check",
		fhttp.HandlerFunc(c.ConnectionCheck))
	a.Router.Handle(fhttp.MethodPost, "/webhooks/whatsapp/{channel}", "webhooks.whatsapp.connection-events",
		fhttp.HandlerFunc(c.ConnectionEvents))
}
