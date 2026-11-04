/*
|--------------------------------------------------------------------------
| Mailbox Webhook Routes
|--------------------------------------------------------------------------
|
| One route per receiving provider. Every route exists on every install,
| and answers not found unless the install enabled that source. Mailgun's
| extra path segment is its own rule: it posts the raw message only to a
| URL ending in "mime".
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/webhooks"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
mailboxRoutes declares a route per receiving provider.
*/
func mailboxRoutes(a *app.App) {
	controller := &webhooks.MailboxController{
		Sources: a.Mailbox, Receive: a.Actions.ReceiveInboundEmail, Health: a.Actions.InboundHealth,
		Budget: a.Actions.InboundBudget, Config: a.Config.Channels.Email, Log: a.Log, Errors: a.Errors,
	}

	routes := []struct{ path, source string }{
		{"/webhooks/postmark/inbound", appconfig.InboundSourcePostmark},
		{"/webhooks/mailgun/inbound/mime", appconfig.InboundSourceMailgun},
		{"/webhooks/sendgrid/inbound", appconfig.InboundSourceSendGrid},
		{"/webhooks/resend/inbound", appconfig.InboundSourceResend},
		{"/webhooks/cloudflare/inbound", appconfig.InboundSourceCloudflare},
	}

	for _, route := range routes {
		a.Router.Handle(fhttp.MethodPost, route.path, "webhooks."+route.source+".inbound",
			controller.Store(route.source))
	}
}
