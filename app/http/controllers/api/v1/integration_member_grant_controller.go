/*
|--------------------------------------------------------------------------
| Integration Member Grant Controller
|--------------------------------------------------------------------------
|
| A teammate connects their own account on an installation from their
| own browser, and later takes the grant back; whoever manages
| integrations may take back anyone's.
|
*/

package v1

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/integrations"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/models"
)

/*
IntegrationMemberGrantController serves teammates' grants.
*/
type IntegrationMemberGrantController struct {
	Start  *integrations.StartMemberGrant
	Revoke *integrations.RevokeMemberGrant
	IDs    *ids.Registry
}

/*
Store starts the caller's own grant.
*/
func (c *IntegrationMemberGrantController) Store(ctx context.Context,
	in *requests.StoreIntegrationMemberGrantRequest,
) (*IntegrationAuthorizationOutput, error) {
	id, err := integrationID(c.IDs, in.Integration)
	if err != nil {
		return nil, err
	}

	authURL, err := c.Start.Handle(ctx, integrations.StartMemberGrantInput{
		Membership: membership(ctx), InstallationID: id, ReturnTo: in.Body.ReturnTo, Binding: sessionBinding(ctx),
	})
	if err != nil {
		return nil, err
	}

	return authorization(authURL), nil
}

/*
Destroy revokes a grant.
*/
func (c *IntegrationMemberGrantController) Destroy(ctx context.Context,
	in *requests.DestroyIntegrationMemberGrantRequest,
) (*EmptyOutput, error) {
	id, err := integrationID(c.IDs, in.Integration)
	if err != nil {
		return nil, err
	}

	grant, err := c.IDs.Decode(string(models.PrefixIntegrationMemberGrant), in.Grant)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return &EmptyOutput{}, c.Revoke.Handle(ctx, integrations.RevokeMemberGrantInput{
		Membership: membership(ctx), InstallationID: id, GrantID: grant,
	})
}
