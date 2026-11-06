/*
|--------------------------------------------------------------------------
| Access Policy Controller
|--------------------------------------------------------------------------
|
| GET /workspace/access-policy shows a workspace's rules to anyone who may
| read its settings, and PATCH changes them. A request names only the keys
| it changes and a limit sent as null is cleared.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/actions/accesspolicy"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
)

/*
AccessPolicyOutput is a workspace's rules.
*/
type AccessPolicyOutput struct {
	Body fhttp.Resource[resources.AccessPolicy]
}

/*
AccessPolicyController serves the rules of the workspace in play.
*/
type AccessPolicyController struct {
	Show   *accesspolicy.ShowAccessPolicy
	Update *accesspolicy.UpdateAccessPolicy
}

/*
ShowPolicy answers with the rules.
*/
func (c *AccessPolicyController) ShowPolicy(ctx context.Context, _ *requests.NoInput) (*AccessPolicyOutput, error) {
	policy, err := c.Show.Handle(ctx, caller(ctx), membership(ctx))
	if err != nil {
		return nil, err
	}

	return &AccessPolicyOutput{Body: fhttp.NewResource(resources.NewAccessPolicy(policy))}, nil
}

/*
UpdatePolicy changes the rules and answers with them as they now are.
*/
func (c *AccessPolicyController) UpdatePolicy(
	ctx context.Context, in *requests.UpdateAccessPolicyRequest,
) (*AccessPolicyOutput, error) {
	nulls := requests.Nulls(in.RawBody)

	policy, err := c.Update.Handle(ctx, accesspolicy.UpdateAccessPolicyInput{
		Principal: caller(ctx), Membership: membership(ctx), Change: accesspolicy.Change{
			AllowedSignInMethods:    in.Body.AllowedSignInMethods,
			GoogleAllowedDomains:    in.Body.GoogleAllowedDomains,
			SessionMaxAge:           in.Body.MaxAge,
			ClearSessionMaxAge:      nulls["session_max_age_minutes"],
			SessionIdleTimeout:      in.Body.IdleTimeout,
			ClearSessionIdleTimeout: nulls["session_idle_timeout_minutes"],
			DashboardAllowlist:      requests.AllowedAddresses(in.Body.DashboardAllowlist),
			APIAllowlist:            requests.AllowedAddresses(in.Body.APIAllowlist),
		},
	})
	if err != nil {
		return nil, err
	}

	return &AccessPolicyOutput{Body: fhttp.NewResource(resources.NewAccessPolicy(policy))}, nil
}
