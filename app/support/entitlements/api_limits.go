/*
|--------------------------------------------------------------------------
| API Rate Limits
|--------------------------------------------------------------------------
|
| How many requests a minute the public API takes from one token, one
| signed-in person and one workspace's tokens together. They are fairness
| limits, read through entitlements like every other cap, so a customer
| who needs more gets an override rather than a code change. A catalog
| that does not list them leaves the install's configured numbers in
| force. Zero means no limit, which a self-hosted operator may choose.
|
*/

package entitlements

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
The keys of the API rate limits.
*/
const (
	KeyAPIRateLimit          = "api.rate_limit_per_minute"
	KeyAPIUserRateLimit      = "api.user_rate_limit_per_minute"
	KeyAPIWorkspaceRateLimit = "api.workspace_rate_limit_per_minute"
)

/*
Limits answers a workspace's caps; contracts.Entitlements satisfies it.
*/
type Limits interface {
	/*
		Limit returns the workspace's cap of the key, -1 meaning no cap.
	*/
	Limit(ctx context.Context, workspaceID ids.UUID, key string) (int64, error)
}

/*
APILimitQuery asks for one API limit: the workspace, the key, and the
number the install configured, used when the catalog has none.
*/
type APILimitQuery struct {
	Workspace  ids.UUID
	Key        string
	Configured int64
}

/*
APILimit returns the workspace's requests a minute under the key, or the
configured number when the catalog does not list the key or cannot be
read. Zero or less means no limit.
*/
func APILimit(ctx context.Context, l Limits, q APILimitQuery) int64 {
	perMinute, err := l.Limit(ctx, q.Workspace, q.Key)
	if err != nil {
		return q.Configured
	}

	return perMinute
}
