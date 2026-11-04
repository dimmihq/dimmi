/*
|--------------------------------------------------------------------------
| Retention Guards
|--------------------------------------------------------------------------
|
| No legal holds or retention policies exist yet, so the guard every
| purge asks lets everything go once its own time is up. A hold later is
| a guard of its own, swapped in at boot.
|
*/

package retention

import (
	"context"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/contracts"
)

/*
NoHolds is the guard of an install without holds: every subject may be
destroyed.
*/
type NoHolds struct{}

var _ contracts.RetentionGuard = NoHolds{}

/*
MayDestroy always allows.
*/
func (NoHolds) MayDestroy(context.Context, ids.UUID, contracts.RetentionSubject) (bool, error) {
	return true, nil
}
