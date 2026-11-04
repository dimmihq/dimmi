/*
|--------------------------------------------------------------------------
| Webhook Routes
|--------------------------------------------------------------------------
|
| Deliveries from providers, under /webhooks. They carry no session and
| no cookie: each controller checks the provider's own proof, such as a
| signature or a shared secret, before reading anything.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/http/controllers/webhooks"
)

/*
Webhooks declares the provider delivery routes.
*/
func Webhooks(a *app.App) {
	inbound := &webhooks.TestInboundController{
		Receive: a.Actions.ReceiveInboundEmail, Config: a.Config.Channels.Email, Clock: a.Clock, Errors: a.Errors,
	}

	a.Router.Handle(fhttp.MethodPost, "/webhooks/test/inbound", "webhooks.test.inbound",
		fhttp.HandlerFunc(inbound.Store))

	mailboxRoutes(a)
}
