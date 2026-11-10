/*
|--------------------------------------------------------------------------
| Prune Channel Inbound Events
|--------------------------------------------------------------------------
|
| A kept delivery from a chat app holds the provider's payload, with
| what the customer wrote and who they are, so it goes once it is no
| longer needed: processed and ignored deliveries after
| ProcessedRetentionDays, failed and held ones after FailedRetentionDays.
| A held delivery still waiting when that ends is lost on purpose. The
| round runs once a day, one workspace at a time, a batch per
| transaction, and skips a workspace whose records are held.
|
*/

package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/ids"
	"github.com/dimmihq/dimmi/framework/queue"
	"github.com/dimmihq/dimmi/framework/tenancy"

	"github.com/dimmihq/dimmi/app/contracts"
	"github.com/dimmihq/dimmi/app/enums"
	"github.com/dimmihq/dimmi/app/models"
)

/*
PruneChannelInboundEventsArgs names the job; it takes no arguments.
*/
type PruneChannelInboundEventsArgs struct{}

/*
Kind names the job.
*/
func (PruneChannelInboundEventsArgs) Kind() string { return "prune_channel_inbound_events" }

/*
Options run the job once a day at most, and retry it a few times.
*/
func (PruneChannelInboundEventsArgs) Options() queue.Options {
	return queue.Options{MaxAttempts: pruneAttempts, Unique: queue.UniqueOpts{ByPeriod: day}}
}

/*
PruneChannelInboundEvents deletes processed and ignored deliveries past
ProcessedRetentionDays and failed and held ones past
FailedRetentionDays; a period of 0 keeps them for ever. Retention
decides which workspaces' deliveries may go.
*/
type PruneChannelInboundEvents struct {
	DB                     *database.DB
	Clock                  clock.Clock
	Retention              contracts.RetentionGuard
	ProcessedRetentionDays int
	FailedRetentionDays    int
}

/*
Handle prunes every workspace's deliveries.
*/
func (w *PruneChannelInboundEvents) Handle(
	ctx context.Context, _ *queue.Job[PruneChannelInboundEventsArgs],
) error {
	workspaces, err := models.New(w.DB).ListWorkspaces(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces to prune: %w", err)
	}

	for _, workspace := range workspaces {
		if err := w.pruneWorkspace(ctx, workspace.ID); err != nil {
			return err
		}
	}

	return nil
}

/*
pruneWorkspace prunes one workspace's deliveries, unless the retention
guard holds them.
*/
func (w *PruneChannelInboundEvents) pruneWorkspace(ctx context.Context, workspaceID ids.UUID) error {
	if w.Retention != nil {
		subject := contracts.RetentionSubject{Kind: contracts.RetentionWorkspace, ID: workspaceID}
		ok, err := w.Retention.MayDestroy(ctx, workspaceID, subject)
		if err != nil {
			return fmt.Errorf("ask retention guard: %w", err)
		}
		if !ok {
			return nil
		}
	}

	scoped := tenancy.With(ctx, workspaceID.String())

	done := []string{string(enums.ChannelEventProcessed), string(enums.ChannelEventIgnored)}
	if err := w.pruneStatuses(scoped, workspaceID, done, w.ProcessedRetentionDays); err != nil {
		return err
	}

	stuck := []string{string(enums.ChannelEventFailed), string(enums.ChannelEventHeld)}

	return w.pruneStatuses(scoped, workspaceID, stuck, w.FailedRetentionDays)
}

/*
pruneStatuses deletes the workspace's deliveries in the statuses
received before the period, a batch per transaction. A period of 0 or
less deletes nothing.
*/
func (w *PruneChannelInboundEvents) pruneStatuses(
	ctx context.Context, workspaceID ids.UUID, statuses []string, days int,
) error {
	if days <= 0 {
		return nil
	}

	params := models.PruneChannelInboundEventsParams{
		WorkspaceID: workspaceID, Statuses: statuses,
		Cutoff: w.Clock.Now().Add(-time.Duration(days) * day), Batch: pruneBatch,
	}

	err := prune(ctx, func(ctx context.Context) (int64, error) {
		var n int64
		err := w.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
			var err error
			n, err = models.New(tx).PruneChannelInboundEvents(ctx, params)
			return err
		})
		return n, err
	})
	if err != nil {
		return fmt.Errorf("prune channel deliveries: %w", err)
	}

	return nil
}
