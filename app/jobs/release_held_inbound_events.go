/*
|--------------------------------------------------------------------------
| Release Held Inbound Events
|--------------------------------------------------------------------------
|
| While a workspace is suspended its chat apps' deliveries are kept but
| held. Once it is back, this job sets them pending again and queues
| their processing in the order they were received, a batch per
| transaction, so nothing a customer wrote meanwhile is lost. A
| workspace suspended again before the job runs keeps its deliveries
| held for the next release.
|
*/

package jobs

import (
	"context"
	"fmt"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/actions/workspaces"
	"github.com/dimmihq/dimmi/app/models"
)

/*
heldBatch is how many held deliveries one transaction releases.
*/
const heldBatch = 500

/*
ReleaseHeldInboundEvents sets a workspace's held deliveries pending and
queues their processing.
*/
type ReleaseHeldInboundEvents struct {
	DB    *database.DB
	Clock clock.Clock
	Queue *queue.Queue
}

/*
Handle releases the workspace's held deliveries, batch after batch,
until a batch comes back short.
*/
func (w *ReleaseHeldInboundEvents) Handle(
	ctx context.Context, job *queue.Job[workspaces.ReleaseHeldInboundEventsArgs],
) error {
	workspaceID := job.Args.WorkspaceID
	ctx = tenancy.With(ctx, workspaceID.String())

	for {
		released, err := w.releaseBatch(ctx, workspaceID)
		if err != nil {
			return fmt.Errorf("release held deliveries: %w", err)
		}

		if released < heldBatch {
			return nil
		}
	}
}

/*
releaseBatch sets one batch pending and queues each delivery's
processing in the same transaction, returning how many it released.
*/
func (w *ReleaseHeldInboundEvents) releaseBatch(ctx context.Context, workspaceID ids.UUID) (int, error) {
	var released int

	params := models.ReleaseHeldChannelInboundEventsParams{
		Now: w.Clock.Now(), WorkspaceID: workspaceID, Batch: heldBatch,
	}

	err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		eventIDs, err := models.New(tx).ReleaseHeldChannelInboundEvents(ctx, params)
		if err != nil {
			return err
		}

		for _, eventID := range eventIDs {
			args := ProcessChannelInboundEventArgs{WorkspaceID: workspaceID, EventID: eventID}
			if err := w.Queue.PushTx(ctx, tx, args); err != nil {
				return err
			}
		}

		released = len(eventIDs)

		return nil
	})

	return released, err
}
