/*
|--------------------------------------------------------------------------
| Webhook Controller
|--------------------------------------------------------------------------
|
| The outbound webhooks API: the catalog of events an endpoint can
| subscribe to; endpoints listed, added, read, changed and removed; an
| endpoint's numbers for the last day, its secret revealed or rotated
| and a sample sent to it; and its delivery log, each delivery read
| with its attempts and sent again. A new endpoint's secret comes back
| once, with it.
|
*/

package v1

import (
	"context"
	"time"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/webhooks"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Pages of the delivery log.
*/
const (
	defaultWebhookDeliveries = 25
	maxWebhookDeliveries     = 100
)

/*
WebhookController serves webhook endpoints and their deliveries.
*/
type WebhookController struct {
	Webhooks *webhooks.Webhooks
	IDs      *ids.Registry
}

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

/*
EventTypes lists the events an endpoint can subscribe to.
*/
func (c *WebhookController) EventTypes(_ context.Context, _ *requests.NoInput) (*WebhookEventTypesOutput, error) {
	items := resources.NewWebhookEventTypes(appevents.WebhookCatalog())
	page := pagination.Page[resources.WebhookEventType]{Items: items, Limit: len(items)}

	return &WebhookEventTypesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Index lists the workspace's endpoints.
*/
func (c *WebhookController) Index(ctx context.Context, _ *requests.NoInput) (*WebhookEndpointsOutput, error) {
	rows, err := c.Webhooks.List(ctx, membership(ctx))
	if err != nil {
		return nil, err
	}

	items := resources.NewWebhookEndpoints(rows)
	page := pagination.Page[resources.WebhookEndpoint]{Items: items, Limit: len(items)}

	return &WebhookEndpointsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Store adds an endpoint and answers it with its secret.
*/
func (c *WebhookController) Store(ctx context.Context, in *requests.StoreWebhookEndpointRequest,
) (*CreatedWebhookEndpointOutput, error) {
	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}

	row, secret, err := c.Webhooks.Create(ctx, input)
	if err != nil {
		return nil, err
	}

	out := &CreatedWebhookEndpointOutput{}
	out.Body.Data = resources.NewWebhookEndpoint(row)
	out.Body.Meta.Secret = secret

	return out, nil
}

/*
Show reads one endpoint.
*/
func (c *WebhookController) Show(ctx context.Context, in *requests.WebhookEndpointRequest,
) (*WebhookEndpointOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	return endpointOutput(c.Webhooks.Show(ctx, membership(ctx), id))
}

/*
Update changes an endpoint.
*/
func (c *WebhookController) Update(ctx context.Context, in *requests.UpdateWebhookEndpointRequest,
) (*WebhookEndpointOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	input, err := c.input(ctx, in.Body)
	if err != nil {
		return nil, err
	}

	return endpointOutput(c.Webhooks.Update(ctx, id, input))
}

/*
Destroy removes an endpoint with its log.
*/
func (c *WebhookController) Destroy(ctx context.Context, in *requests.WebhookEndpointRequest) (*EmptyOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Webhooks.Delete(ctx, membership(ctx), id)
}

/*
Stats answers an endpoint's numbers for the last day.
*/
func (c *WebhookController) Stats(ctx context.Context, in *requests.WebhookEndpointRequest,
) (*WebhookStatsOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	m := membership(ctx)
	if _, err := c.Webhooks.Show(ctx, m, id); err != nil {
		return nil, err
	}

	stats, err := c.Webhooks.Stats(ctx, m, id)
	if err != nil {
		return nil, err
	}

	return &WebhookStatsOutput{Body: fhttp.NewResource(resources.NewWebhookEndpointStats(stats))}, nil
}

/*
RevealSecret answers an endpoint's secrets and records who saw them.
*/
func (c *WebhookController) RevealSecret(ctx context.Context, in *requests.WebhookEndpointRequest,
) (*WebhookSecretOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	secrets, err := c.Webhooks.RevealSecret(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &WebhookSecretOutput{Body: fhttp.NewResource(resources.NewWebhookSecret(id, secrets))}, nil
}

/*
RotateSecret gives an endpoint a new secret and answers both.
*/
func (c *WebhookController) RotateSecret(ctx context.Context, in *requests.RotateWebhookSecretRequest,
) (*WebhookSecretOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	_, secrets, err := c.Webhooks.RotateSecret(ctx, webhooks.RotationInput{
		Membership: membership(ctx), EndpointID: id, Overlap: time.Duration(in.Body.OverlapSeconds) * time.Second,
		Force: in.Body.Force,
	})
	if err != nil {
		return nil, err
	}

	return &WebhookSecretOutput{Body: fhttp.NewResource(resources.NewWebhookSecret(id, secrets))}, nil
}

/*
Test sends a sample event to an endpoint and answers how it went.
*/
func (c *WebhookController) Test(ctx context.Context, in *requests.TestWebhookEndpointRequest,
) (*WebhookTestOutput, error) {
	id, err := c.endpoint(in.Endpoint)
	if err != nil {
		return nil, err
	}

	result, err := c.Webhooks.Test(ctx, webhooks.TestInput{
		Membership: membership(ctx), EndpointID: id, Type: in.Body.Type,
	})
	if err != nil {
		return nil, err
	}

	return &WebhookTestOutput{Body: fhttp.NewResource(resources.NewWebhookTestResult(result))}, nil
}

/*
input reads a body into the actions' input.
*/
func (c *WebhookController) input(ctx context.Context, b requests.WebhookEndpointBody) (webhooks.EndpointInput, error) {
	in := webhooks.EndpointInput{
		Membership: membership(ctx), URL: b.URL, Description: b.Description, EventTypes: b.EventTypes,
		IncludeNotes: b.IncludeNotes, APIVersion: b.APIVersion, Metadata: b.Metadata, Enabled: b.Enabled,
	}

	if b.InboxIDs == nil {
		return in, nil
	}

	if len(*b.InboxIDs) == 0 {
		in.ClearInboxes = true

		return in, nil
	}

	list := make([]ids.UUID, 0, len(*b.InboxIDs))
	for _, public := range *b.InboxIDs {
		id, err := decodeID(c.IDs, models.PrefixInbox, public)
		if err != nil {
			return in, err
		}

		list = append(list, id)
	}

	in.InboxIDs = &list

	return in, nil
}

/*
endpoint decodes an endpoint's public id.
*/
func (c *WebhookController) endpoint(public string) (ids.UUID, error) {
	return decodeID(c.IDs, models.PrefixWebhookEndpoint, public)
}

/*
endpointOutput shapes an endpoint or passes its error on.
*/
func endpointOutput(e models.WebhookEndpoint, err error) (*WebhookEndpointOutput, error) {
	if err != nil {
		return nil, err
	}

	return &WebhookEndpointOutput{Body: fhttp.NewResource(resources.NewWebhookEndpoint(e))}, nil
}
