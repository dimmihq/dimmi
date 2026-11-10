/*
|--------------------------------------------------------------------------
| Integration Installation Controller
|--------------------------------------------------------------------------
|
| A teammate starts installing a tool's app from their own browser: the
| answer is the provider URL to send the browser to, bound to the
| browser session that asked. When the workspace's installation already
| holds every scope, the URL leads straight back to the page instead.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/session"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
IntegrationInstallationController starts installs.
*/
type IntegrationInstallationController struct {
	Start *integrations.StartInstallation
}

/*
IntegrationAuthorizationOutput is where to send the browser.
*/
type IntegrationAuthorizationOutput struct {
	Body fhttp.Resource[resources.IntegrationAuthorization]
}

/*
Store starts an install.
*/
func (c *IntegrationInstallationController) Store(ctx context.Context,
	in *requests.StoreIntegrationInstallationRequest,
) (*IntegrationAuthorizationOutput, error) {
	authURL, err := c.Start.Handle(ctx, integrations.StartInstallationInput{
		Membership: membership(ctx), Provider: in.Provider, Features: in.Body.Features,
		ReturnTo: in.Body.ReturnTo, Binding: sessionBinding(ctx),
	})
	if err != nil {
		return nil, err
	}

	return authorization(authURL), nil
}

/*
authorization answers the URL to send the browser to.
*/
func authorization(authURL string) *IntegrationAuthorizationOutput {
	return &IntegrationAuthorizationOutput{Body: fhttp.NewResource(resources.IntegrationAuthorization{
		Object: "integration_authorization", AuthorizeURL: authURL,
	})}
}

/*
sessionBinding names the browser session of the request, empty for
none.
*/
func sessionBinding(ctx context.Context) string {
	s, ok := session.From(ctx)
	if !ok {
		return ""
	}

	return s.ID().String()
}
