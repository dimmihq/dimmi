/*
|--------------------------------------------------------------------------
| Webhook Secret Outputs
|--------------------------------------------------------------------------
|
| What the secret, stats and test endpoints answer with.
|
*/

package v1

import (
	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
WebhookSecretOutput is an endpoint's secrets.
*/
type WebhookSecretOutput struct {
	Body fhttp.Resource[resources.WebhookSecret]
}

/*
WebhookStatsOutput is an endpoint's numbers for the last day.
*/
type WebhookStatsOutput struct {
	Body fhttp.Resource[resources.WebhookEndpointStats]
}

/*
WebhookTestOutput is a test send and how it was answered.
*/
type WebhookTestOutput struct {
	Body fhttp.Resource[resources.WebhookTestResult]
}
