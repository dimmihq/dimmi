/*
|--------------------------------------------------------------------------
| Webhook Requests
|--------------------------------------------------------------------------
|
| The inputs of the outbound webhooks API. An endpoint subscribes to
| catalog event types, area wildcards such as conversation.* or *. Its
| inbox filter is a list of inbox ids; an empty list removes it, so the
| endpoint hears every inbox. Deliveries page newest first with the id
| of the last one seen as the cursor.
|
*/

package requests

/*
WebhookEndpointBody is an endpoint to create, or the fields of one to
change: a field left out stays as it is.
*/
type WebhookEndpointBody struct {
	URL          *string            `json:"url,omitempty"           maxLength:"2048"`
	Description  *string            `json:"description,omitempty"   maxLength:"255"`
	EventTypes   *[]string          `json:"event_types,omitempty"   maxItems:"100"`
	IncludeNotes *bool              `json:"include_notes,omitempty"`
	InboxIDs     *[]string          `json:"inbox_ids,omitempty"     maxItems:"100"`
	APIVersion   *string            `json:"api_version,omitempty"   maxLength:"20"`
	Metadata     *map[string]string `json:"metadata,omitempty"`
	Enabled      *bool              `json:"enabled,omitempty"`
}

/*
StoreWebhookEndpointRequest is the input of POST /webhook-endpoints.
*/
type StoreWebhookEndpointRequest struct {
	Body WebhookEndpointBody
}

/*
WebhookEndpointRequest names an endpoint in the path.
*/
type WebhookEndpointRequest struct {
	Endpoint string `path:"endpoint"`
}

/*
UpdateWebhookEndpointRequest is the input of PATCH
/webhook-endpoints/{endpoint}.
*/
type UpdateWebhookEndpointRequest struct {
	Endpoint string `path:"endpoint"`
	Body     WebhookEndpointBody
}

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
