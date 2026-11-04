/*
|--------------------------------------------------------------------------
| Entitlements
|--------------------------------------------------------------------------
|
| One plan, every feature: an entitlement is a limit or a rollout, never a
| paywall. Every cap in the product is read through this one contract, so
| no call site ever branches on which edition it runs in.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
Entitlements answers what a workspace may have.
*/
type Entitlements interface {
	/*
		Has reports whether the feature is on for the workspace.
	*/
	Has(ctx context.Context, workspaceID ids.UUID, feature string) (bool, error)

	/*
		Limit returns the workspace's cap of the key, -1 meaning no cap.
	*/
	Limit(ctx context.Context, workspaceID ids.UUID, key string) (int64, error)
}
