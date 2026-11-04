/*
|--------------------------------------------------------------------------
| Billing Status
|--------------------------------------------------------------------------
|
| Parts of the product treat a workspace differently once it ever paid,
| such as the badge on the messenger. They ask here and never read billing
| tables, so an install without billing simply answers no.
|
*/

package contracts

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"
)

/*
BillingStatus answers what billing knows about a workspace.
*/
type BillingStatus interface {
	/*
		HasPaid reports whether the workspace ever paid an invoice.
	*/
	HasPaid(ctx context.Context, workspaceID ids.UUID) (bool, error)
}
