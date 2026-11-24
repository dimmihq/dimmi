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
| SES also reports what became of sent mail, bounces and complaints,
| on a route of its own.
|
*/

package routes

import (
	"slices"

	"github.com/dimmihq/dimmi/framework/aws/sesevents"
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
		Budget: a.Actions.InboundBudget, Metrics: a.Actions.InboundMetrics, Config: a.Config.Channels.Email,
		Log: a.Log, Errors: a.Errors,
	}

	routes := []struct{ path, source string }{
		{"/webhooks/ses/inbound", appconfig.InboundSourceSES},
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

	sesEventRoutes(a)
}

/*
sesEventRoutes declares where SES reports bounces and complaints. The
customer stream's topic and the system stream's are both accepted; an
install that named neither answers not found.
*/
func sesEventRoutes(a *app.App) {
	sending := a.Config.Channels.Email.Sending.SES
	topics := slices.DeleteFunc([]string{sending.EventsTopicARN, sending.SystemTopicARN}, func(arn string) bool {
		return arn == ""
	})

	controller := &webhooks.SESEventsController{
		Events: sesevents.New(a.HTTP, topics), Record: a.Actions.RecordDeliveryEvent, Enabled: len(topics) > 0,
		Log: a.Log, Errors: a.Errors,
	}

	a.Router.Handle(fhttp.MethodPost, "/webhooks/ses/events", "webhooks.ses.events",
		fhttp.HandlerFunc(controller.Store))
}
