/*
|--------------------------------------------------------------------------
| User Identity Controller
|--------------------------------------------------------------------------
|
| GET /user/identities lists the provider accounts linked to the caller
| and DELETE /user/identities/{identity} unlinks one, which takes a
| recent confirmation. Linking happens in the browser, through the
| provider's redirect.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/auth"
	appauth "github.com/dimmihq/dimmi/app/auth"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
UserIdentitiesOutput is the list of the caller's linked accounts.
*/
type UserIdentitiesOutput struct {
	Body fhttp.Collection[resources.OAuthIdentity]
}

/*
UserIdentityController serves the caller's identity collection.
*/
type UserIdentityController struct {
	List    *auth.ListOAuthIdentities
	Unlink  *auth.UnlinkOAuthIdentity
	IDs     *ids.Registry
	Cookies appauth.Cookies
}

/*
Index lists the caller's linked accounts.
*/
func (c *UserIdentityController) Index(ctx context.Context, _ *requests.NoInput) (*UserIdentitiesOutput, error) {
	identities, err := c.List.Handle(ctx, caller(ctx).User.ID)
	if err != nil {
		return nil, err
	}

	items := resources.NewOAuthIdentities(identities)
	page := pagination.Page[resources.OAuthIdentity]{Items: items, Limit: len(items)}

	return &UserIdentitiesOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Destroy unlinks one of the caller's accounts.
*/
func (c *UserIdentityController) Destroy(
	ctx context.Context, in *requests.DestroyUserIdentityRequest,
) (*EmptyOutput, error) {
	id, err := c.IDs.Decode(string(models.PrefixOAuthIdentity), in.Identity)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	err = c.Unlink.Handle(ctx, auth.UnlinkOAuthIdentityInput{
		UserID: caller(ctx).User.ID, IdentityID: id, Client: clientOf(ctx, c.Cookies),
	})
	if err != nil {
		return nil, err
	}

	return &EmptyOutput{}, nil
}
