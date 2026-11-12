/*
|--------------------------------------------------------------------------
| Webhook Endpoint Routes
|--------------------------------------------------------------------------
|
| The API of outbound webhooks. Reading endpoints, their numbers and
| their log takes webhooks:read; adding, changing and removing them,
| revealing or rotating a secret, test sends and redeliveries take
| webhooks:write. Creating and rotating take no idempotency key, since
| a replay cache would keep the secret in plain text.
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
webhookEndpointRoutes declares the outbound webhooks API.
*/
func webhookEndpointRoutes(workspace *routing.Router, a *app.App) {
	if a.Actions.Webhooks == nil {
		return
	}

	c := &v1.WebhookController{Webhooks: a.Actions.Webhooks, IDs: a.IDs}
	one := "/webhook-endpoints/{endpoint}"
	delivery := "/webhook-deliveries/{delivery}"

	routing.Get(workspace, "/webhook-event-types", "webhook-event-types.index", c.EventTypes).
		Ability("webhooks:read")
	routing.Get(workspace, "/webhook-endpoints", "webhook-endpoints.index", c.Index).Ability("webhooks:read")
	routing.Post(workspace, "/webhook-endpoints", "webhook-endpoints.store", c.Store).
		Ability("webhooks:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, one, "webhook-endpoints.show", c.Show).Ability("webhooks:read")
	routing.Patch(workspace, one, "webhook-endpoints.update", c.Update).Ability("webhooks:write")
	routing.Delete(workspace, one, "webhook-endpoints.destroy", c.Destroy).
		Ability("webhooks:write").Status(fhttp.StatusNoContent)
	routing.Get(workspace, one+"/stats", "webhook-endpoints.stats.show", c.Stats).Ability("webhooks:read")
	routing.Post(workspace, one+"/secret-reveals", "webhook-endpoints.secret-reveals.store", c.RevealSecret).
		Ability("webhooks:write").Status(fhttp.StatusOK)
	routing.Post(workspace, one+"/secret-rotations", "webhook-endpoints.secret-rotations.store", c.RotateSecret).
		Ability("webhooks:write").Status(fhttp.StatusCreated)
	routing.Post(workspace, one+"/tests", "webhook-endpoints.tests.store", c.Test).
		Ability("webhooks:write").Status(fhttp.StatusCreated)
	routing.Get(workspace, one+"/deliveries", "webhook-endpoints.deliveries.index", c.Deliveries).
		Ability("webhooks:read")
	routing.Get(workspace, delivery, "webhook-deliveries.show", c.ShowDelivery).Ability("webhooks:read")
	routing.Post(workspace, delivery+"/redeliveries", "webhook-deliveries.redeliveries.store", c.Redeliver).
		Ability("webhooks:write").Status(fhttp.StatusOK)
}
