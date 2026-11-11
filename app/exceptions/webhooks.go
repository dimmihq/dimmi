/*
|--------------------------------------------------------------------------
| Webhook Errors
|--------------------------------------------------------------------------
|
| What can go wrong while managing outbound webhooks. An endpoint must be
| a public https address the workspace has not added yet, subscribed to
| events the catalog lists. A secret is rotated once at a time, and a
| delivery being sent is not sent again until that attempt is over.
|
*/

package exceptions

import "github.com/dimmihq/dimmi/framework/fault"

/*
Webhook failures.

ErrInvalidWebhookURL refuses a URL that is malformed, carries
credentials, uses a refused port, or points at a private address, this
server's own inbound webhooks or localhost. ErrInsecureWebhookURL
refuses plain http where https is required. ErrWebhookURLTaken refuses
a URL the workspace already added. ErrUnknownWebhookEventType refuses
a subscription entry the catalog does not list. ErrWebhookEventTypesRequired
refuses an endpoint subscribed to nothing. ErrWebhookAPIVersionUnsupported
refuses an api_version this server does not speak. ErrWebhookInboxUnknown
refuses an inbox filter naming an inbox the workspace does not have.
ErrWebhookMetadataInvalid refuses metadata over 20 keys or 500
characters a value. ErrWebhookEndpointLimitReached refuses an endpoint
past the workspace's cap. ErrWebhookRotationInProgress refuses a
rotation while the previous secret still signs, unless forced.
ErrWebhookRotationOverlap refuses an overlap past the maximum.
ErrWebhookDeliveryInProgress refuses a redelivery while another attempt
of it runs. ErrWebhookTestThrottled refuses test sends past the
endpoint's per minute cap. ErrWebhookTestTypeUnknown refuses a test of
a type the catalog does not list.
*/
var (
	ErrInvalidWebhookURL = fault.New(fault.Invalid, "invalid_webhook_url",
		"Enter a public URL, such as https://api.example.com/webhooks. Use a public tunnel for local servers.").
		OnField("url")
	ErrInsecureWebhookURL = fault.New(fault.Invalid, "insecure_webhook_url",
		"Use an https:// URL.").OnField("url")
	ErrWebhookURLTaken = fault.New(fault.Conflict, "endpoint_url_taken",
		"An endpoint with this URL already exists.").OnField("url")
	ErrUnknownWebhookEventType = fault.New(fault.Invalid, "unknown_event_type",
		"Subscribe to event types from the catalog, an area such as conversation.*, or *.").OnField("event_types")
	ErrWebhookEventTypesRequired = fault.New(fault.Invalid, "event_types_required",
		"Pick at least one event.").OnField("event_types")
	ErrWebhookAPIVersionUnsupported = fault.New(fault.Invalid, "api_version_unsupported",
		"This API version is not supported.").OnField("api_version")
	ErrWebhookInboxUnknown = fault.New(fault.Invalid, "inbox_not_found",
		"One of the inboxes does not exist.").OnField("inbox_ids")
	ErrWebhookMetadataInvalid = fault.New(fault.Invalid, "metadata_invalid",
		"Use at most 20 metadata keys with values of up to 500 characters.").OnField("metadata")
	ErrWebhookEndpointLimitReached = fault.New(fault.Invalid, "entitlement_limit_reached",
		"The workspace has as many webhook endpoints as it may.")
	ErrWebhookRotationInProgress = fault.New(fault.Conflict, "rotation_in_progress",
		"The previous secret still signs requests. Wait for it to expire, or force the rotation.")
	ErrWebhookRotationOverlap = fault.New(fault.Invalid, "rotation_overlap_invalid",
		"The previous secret may keep signing for 0 to 7 days.").OnField("expire_previous_in_seconds")
	ErrWebhookDeliveryInProgress = fault.New(fault.Conflict, "delivery_in_progress",
		"This delivery is being sent right now. Try again in a moment.")
	ErrWebhookTestThrottled = fault.New(fault.RateLimited, "throttled",
		"Too many test sends to this endpoint. Try again in a minute.")
	ErrWebhookTestTypeUnknown = fault.New(fault.Invalid, "unknown_event_type",
		"Test with an event type from the catalog.").OnField("type")
)
