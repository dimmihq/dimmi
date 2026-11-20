/*
|--------------------------------------------------------------------------
| Messenger And Instagram Webhook Routes
|--------------------------------------------------------------------------
|
| Where Meta delivers a Page's and an Instagram account's events. Each
| platform has its own address, set on the Meta app for the page and the
| instagram object. Both are public: Meta's signature is the only proof
| a delivery is real.
|
*/

package routes

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app"
	"github.com/dimmihq/dimmi/app/actions/channels"
	"github.com/dimmihq/dimmi/app/http/controllers/webhooks"
)

/*
socialWebhookRoutes declares the Messenger and Instagram delivery
routes.
*/
func socialWebhookRoutes(a *app.App) {
	limit := a.Config.Channels.Social.MaxPayloadBytes

	platforms := []struct {
		path    string
		channel *channels.Social
	}{{"facebook", a.Actions.Facebook}, {"instagram", a.Actions.Instagram}}

	for _, p := range platforms {
		path, channel := p.path, p.channel

		c := &webhooks.SocialController{Channel: channel, MaxBytes: limit}

		a.Router.Handle(fhttp.MethodGet, "/webhooks/"+path, "webhooks."+path+".check", fhttp.HandlerFunc(c.Check))
		a.Router.Handle(fhttp.MethodPost, "/webhooks/"+path, "webhooks."+path+".events", fhttp.HandlerFunc(c.Events))
	}
}
