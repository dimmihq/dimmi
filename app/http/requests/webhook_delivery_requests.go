/*
|--------------------------------------------------------------------------
| Webhook Delivery Requests
|--------------------------------------------------------------------------
|
| Inputs of the webhook secret, test and delivery endpoints.
|
*/

package requests

/*
RotateWebhookSecretRequest is the input of POST
/webhook-endpoints/{endpoint}/secret-rotations: how many seconds the
replaced secret keeps signing, and whether to replace it while an
earlier rotation still overlaps.
*/
type RotateWebhookSecretRequest struct {
	Endpoint string `path:"endpoint"`
	Body     struct {
		OverlapSeconds int64 `json:"overlap_seconds,omitempty" minimum:"0"`
		Force          bool  `json:"force,omitempty"`
	}
}

/*
TestWebhookEndpointRequest is the input of POST
/webhook-endpoints/{endpoint}/tests: the event type to send a sample
of, webhook.test when left out.
*/
type TestWebhookEndpointRequest struct {
	Endpoint string `path:"endpoint"`
	Body     struct {
		Type string `json:"type,omitempty" maxLength:"80"`
	}
}

/*
WebhookDeliveriesRequest filters and pages an endpoint's deliveries.
*/
type WebhookDeliveriesRequest struct {
	Endpoint  string `path:"endpoint"`
	Status    string `query:"status"     enum:"pending,retrying,succeeded,failed,canceled,"`
	EventType string `query:"event_type" maxLength:"80"`
	Cursor    string `query:"cursor"     maxLength:"80"`
	Limit     int32  `query:"limit"      maximum:"100"`
}

/*
WebhookDeliveryRequest names a delivery in the path.
*/
type WebhookDeliveryRequest struct {
	Delivery string `path:"delivery"`
}
