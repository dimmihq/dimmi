/*
|--------------------------------------------------------------------------
| Workspace Entitlements Controller
|--------------------------------------------------------------------------
|
| What the workspace in play may have, so pages and scripts can say how
| fast the API takes requests before they run into the limit.
|
*/

package v1

import (
	"context"

	fhttp "github.com/dimmihq/dimmi/framework/http"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/http/requests"
	"github.com/dimmihq/dimmi/app/http/resources"
	"github.com/dimmihq/dimmi/app/support/entitlements"
	appconfig "github.com/dimmihq/dimmi/config"
)

/*
WorkspaceEntitlementsOutput is the workspace's entitlements.
*/
type WorkspaceEntitlementsOutput struct {
	Body fhttp.Resource[resources.WorkspaceEntitlements]
}

/*
WorkspaceEntitlementController serves the entitlements of the workspace
in play, with the install's configured API limits for keys the catalog
does not list.
*/
type WorkspaceEntitlementController struct {
	Entitlements contracts.Entitlements
	Limits       appconfig.APILimitsConfig
}

/*
Show answers with the workspace's API limits, requests a minute, zero
meaning none.
*/
func (c *WorkspaceEntitlementController) Show(
	ctx context.Context, _ *requests.NoInput,
) (*WorkspaceEntitlementsOutput, error) {
	ws := membership(ctx).Workspace.ID
	limit := func(key string, configured int64) int64 {
		return max(entitlements.APILimit(ctx, c.Entitlements, entitlements.APILimitQuery{
			Workspace: ws, Key: key, Configured: configured,
		}), 0)
	}

	return &WorkspaceEntitlementsOutput{Body: fhttp.NewResource(resources.WorkspaceEntitlements{
		Object: "entitlements",
		API: resources.APILimits{
			TokenPerMinute:     limit(entitlements.KeyAPIRateLimit, c.Limits.TokenPerMinute),
			UserPerMinute:      limit(entitlements.KeyAPIUserRateLimit, c.Limits.UserPerMinute),
			WorkspacePerMinute: limit(entitlements.KeyAPIWorkspaceRateLimit, c.Limits.WorkspacePerMinute),
		},
	})}, nil
}
