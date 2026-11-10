/*
|--------------------------------------------------------------------------
| Connected App Controller
|--------------------------------------------------------------------------
|
| The consent screen's API and the person's list of connected apps. A
| request the OAuth server refuses is answered with what the screen must
| say, and with where to send the person when the app may hear of it.
|
*/

package v1

import (
	"context"
	"net/url"

	"github.com/dimmihq/dimmi/framework/fault"
	fhttp "github.com/dimmihq/dimmi/framework/http"
	"github.com/dimmihq/dimmi/framework/http/pagination"
	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/oauthapps"
	"github.com/dimmihq/dimmi/app/exceptions"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/models"
)

/*
OAuthAuthorizationOutput is an app's request for review.
*/
type OAuthAuthorizationOutput struct {
	Body fhttp.Resource[resources.OAuthAuthorization]
}

/*
OAuthDecisionOutput is where to send the person after deciding.
*/
type OAuthDecisionOutput struct {
	Body fhttp.Resource[resources.OAuthDecision]
}

/*
ConnectedAppsOutput is the person's connected apps.
*/
type ConnectedAppsOutput struct {
	Body fhttp.Collection[resources.ConnectedApp]
}

/*
OAuthAppController serves the consent screen and connected apps.
*/
type OAuthAppController struct {
	Authorize *oauthapps.Authorize
	Connected *oauthapps.Connected
	IDs       *ids.Registry
}

/*
Show reviews an app's request for the signed-in person.
*/
func (c *OAuthAppController) Show(ctx context.Context, in *requests.OAuthAuthorizationRequest,
) (*OAuthAuthorizationOutput, error) {
	query, err := url.ParseQuery(in.Query)
	if err != nil {
		return nil, errMalformedRequest
	}

	r, err := c.Authorize.Review(ctx, caller(ctx).User.ID, query)
	if err != nil {
		return nil, refused(err)
	}

	return &OAuthAuthorizationOutput{Body: fhttp.NewResource(resources.NewOAuthAuthorization(r))}, nil
}

/*
Store records the person's decision on an app's request.
*/
func (c *OAuthAppController) Store(ctx context.Context, in *requests.DecideOAuthAuthorizationRequest,
) (*OAuthDecisionOutput, error) {
	query, err := url.ParseQuery(in.Body.Query)
	if err != nil {
		return nil, errMalformedRequest
	}

	p := caller(ctx)
	d := oauthapps.Decision{
		UserID: p.User.ID, Query: query, Approve: in.Body.Decision == "approve", Scopes: in.Body.Scopes,
		AuthTime: p.AuthenticatedAt,
	}

	if d.Approve {
		if d.Workspace, err = c.IDs.Decode(string(models.PrefixWorkspace), in.Body.WorkspaceID); err != nil {
			return nil, oauthapps.ErrNoWorkspace
		}
	}

	to, err := c.Authorize.Decide(ctx, d)
	if err != nil {
		return nil, refused(err)
	}

	decision := resources.OAuthDecision{Object: "oauth_decision", RedirectTo: to}

	return &OAuthDecisionOutput{Body: fhttp.NewResource(decision)}, nil
}

/*
Index lists the person's connected apps.
*/
func (c *OAuthAppController) Index(ctx context.Context, _ *requests.NoInput) (*ConnectedAppsOutput, error) {
	apps, err := c.Connected.List(ctx, caller(ctx).User.ID)
	if err != nil {
		return nil, err
	}

	page := pagination.Page[resources.ConnectedApp]{Items: resources.NewConnectedApps(apps), Limit: len(apps)}

	return &ConnectedAppsOutput{Body: fhttp.NewCollection(page)}, nil
}

/*
Destroy disconnects one of the person's apps.
*/
func (c *OAuthAppController) Destroy(ctx context.Context, in *requests.OAuthAppRequest) (*struct{}, error) {
	id, err := c.IDs.Decode(string(models.PrefixOAuthGrant), in.App)
	if err != nil {
		return nil, exceptions.ErrNotFound
	}

	return nil, c.Connected.Disconnect(ctx, caller(ctx).User.ID, id)
}

/*
errMalformedRequest refuses a request query that cannot be read.
*/
var errMalformedRequest = fault.New(fault.Invalid, "oauth_request_malformed",
	"This sign-in link is broken. Start connecting the app again from the app.")

/*
refused turns the OAuth server's refusal into the screen's answer: what
went wrong and, when the app may hear of it, where to send the person.
*/
func refused(err error) error {
	refusal, ok := oauthapps.IsRefusal(err)
	if !ok {
		return err
	}

	f := fault.New(fault.Invalid, "oauth_"+refusal.Code, refusal.Description)
	if to, ok := refusal.RedirectURL(); ok {
		f = f.With("redirect_to", to)
	}

	return f
}
