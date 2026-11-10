/*
|--------------------------------------------------------------------------
| Integration Provider Controller
|--------------------------------------------------------------------------
|
| The tools this install offers, whether each can be installed through
| its shared app, and the scopes each of the product's features asks
| for.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
IntegrationProviderController serves the providers.
*/
type IntegrationProviderController struct {
	List *integrations.ListInstallations
}

/*
IntegrationProvidersOutput is the provider list.
*/
type IntegrationProvidersOutput struct {
	Body fhttp.Collection[resources.IntegrationProvider]
}

/*
Index lists the providers.
*/
func (c *IntegrationProviderController) Index(_ context.Context, _ *requests.IntegrationProvidersRequest,
) (*IntegrationProvidersOutput, error) {
	items := resources.NewIntegrationProviders(c.List.Providers())

	return &IntegrationProvidersOutput{Body: fhttp.NewCollection(pagination.Page[resources.IntegrationProvider]{
		Items: items, Limit: len(items),
	})}, nil
}
