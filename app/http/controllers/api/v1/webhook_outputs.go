/*
|--------------------------------------------------------------------------
| Webhook Outputs
|--------------------------------------------------------------------------
|
| What the webhook endpoints answer with.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
WebhookEventTypesOutput is the catalog of subscribable events.
*/
type WebhookEventTypesOutput struct {
	Body fhttp.Collection[resources.WebhookEventType]
}

/*
WebhookEndpointsOutput is the workspace's endpoints.
*/
type WebhookEndpointsOutput struct {
	Body fhttp.Collection[resources.WebhookEndpoint]
}

/*
WebhookEndpointOutput is one endpoint.
*/
type WebhookEndpointOutput struct {
	Body fhttp.Resource[resources.WebhookEndpoint]
}

/*
CreatedWebhookEndpointOutput is a new endpoint with its secret, shown
this once.
*/
type CreatedWebhookEndpointOutput struct {
	Body struct {
		Data resources.WebhookEndpoint `json:"data"`
		Meta struct {
			Secret string `json:"secret"`
		} `json:"meta"`
	}
}
