/*
|--------------------------------------------------------------------------
| Integration Controller
|--------------------------------------------------------------------------
|
| The workspace's installations: listed, shown, and disconnected. Every
| id is a public one, and no token ever leaves.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IntegrationController serves the installations.
*/
type IntegrationController struct {
	List       *integrations.ListInstallations
	Disconnect *integrations.DisconnectInstallation
	IDs        *ids.Registry
}

/*
IntegrationsOutput is the installation list.
*/
type IntegrationsOutput struct {
	Body fhttp.Collection[resources.Integration]
}

/*
IntegrationOutput is one installation.
*/
type IntegrationOutput struct {
	Body fhttp.Resource[resources.Integration]
}

/*
Index lists the installations.
*/
func (c *IntegrationController) Index(ctx context.Context, in *requests.IntegrationsRequest,
) (*IntegrationsOutput, error) {
	views, err := c.List.List(ctx, membership(ctx), in.Provider)
	if err != nil {
		return nil, err
	}

	items := resources.NewIntegrations(views)

	return &IntegrationsOutput{Body: fhttp.NewCollection(pagination.Page[resources.Integration]{
		Items: items, Limit: len(items),
	})}, nil
}

/*
Show answers one installation.
*/
func (c *IntegrationController) Show(ctx context.Context, in *requests.IntegrationRequest) (*IntegrationOutput, error) {
	id, err := integrationID(c.IDs, in.Integration)
	if err != nil {
		return nil, err
	}

	view, err := c.List.Show(ctx, membership(ctx), id)
	if err != nil {
		return nil, err
	}

	return &IntegrationOutput{Body: fhttp.NewResource(resources.NewIntegration(view))}, nil
}

/*
Destroy disconnects an installation.
*/
func (c *IntegrationController) Destroy(ctx context.Context, in *requests.IntegrationRequest) (*EmptyOutput, error) {
	id, err := integrationID(c.IDs, in.Integration)
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, c.Disconnect.Handle(ctx, integrations.DisconnectInstallationInput{
		Membership: membership(ctx), InstallationID: id,
	})
}

/*
integrationID decodes an installation's public id; an unknown one is not
found.
*/
func integrationID(registry *ids.Registry, public string) (ids.UUID, error) {
	id, err := registry.Decode(string(models.PrefixIntegrationInstallation), public)
	if err != nil {
		return ids.UUID{}, exceptions.ErrNotFound
	}

	return id, nil
}
