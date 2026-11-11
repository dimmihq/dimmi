/*
|--------------------------------------------------------------------------
| Fan-Out
|--------------------------------------------------------------------------
|
| Hearing an event costs one indexed lookup in its own transaction: a
| workspace with no enabled endpoint queues nothing. Otherwise a job
| matches the endpoints at the moment it runs, so one added a second
| after the event still receives it, and builds the snapshot once for
| all of them. Internal notes reach only endpoints that include notes,
| an inbox filter drops conversations of other inboxes, and system
| timeline entries never leave. Each object's snapshots are numbered
| under a lock on the object, so a higher number never carries an older
| snapshot. A fan-out that runs twice stores each delivery once.
|
*/

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/events"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/jobs"
	"github.com/dimmihq/dimmi/app/models"
)

/*
Listen makes the webhooks hear every event, in the transaction that
dispatched it.
*/
func (w *Webhooks) Listen(d *events.Dispatcher) {
	events.Listen(d, events.InTx, func(ctx context.Context, e events.Event) error {
		return w.queueFanOut(ctx, e)
	})
}

/*
queueFanOut pushes the fan-out of a subscribable event, in the event's
transaction, when the workspace has an endpoint that may want it.
*/
func (w *Webhooks) queueFanOut(ctx context.Context, e events.Event) error {
	if !w.Config.Enabled {
		return nil
	}

	args, ok := occurrenceOf(e, w.Clock.Now())
	if !ok {
		return nil
	}

	return w.inWorkspace(ctx, args.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		has, err := q.HasEnabledWebhookEndpoints(ctx, args.WorkspaceID)
		if err != nil || !has {
			return wrapf("look for webhook endpoints", err)
		}

		return wrapf("queue webhook fan-out", w.Queue.PushTx(ctx, tx, args))
	})
}

/*
FanOut stores one delivery per endpoint the event matches and queues
the send of each.
*/
func (w *Webhooks) FanOut(ctx context.Context, args jobs.FanOutWebhookEventArgs) error {
	return w.inWorkspace(ctx, args.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		workspace, err := q.FindWorkspace(ctx, args.WorkspaceID)
		if errors.Is(err, database.ErrNotFound) || workspace.DeletedAt != nil || workspace.SuspendedAt != nil {
			return nil
		}

		if err != nil {
			return fmt.Errorf("find workspace to fan out: %w", err)
		}

		return w.store(ctx, tx, args, workspace)
	})
}

/*
subscribed returns the enabled endpoints subscribed to the event.
*/
func (w *Webhooks) subscribed(
	ctx context.Context, q *models.Queries, args jobs.FanOutWebhookEventArgs,
) ([]models.WebhookEndpoint, error) {
	all, err := q.ListEnabledWebhookEndpoints(ctx, args.WorkspaceID)
	if err != nil {
		return nil, fmt.Errorf("list webhook endpoints: %w", err)
	}

	return slices.DeleteFunc(all, func(e models.WebhookEndpoint) bool {
		return !appevents.WebhookSubscribed(e.EventTypes, args.Type)
	}), nil
}

/*
store snapshots the object, numbers it and stores one delivery per
subscribed endpoint that may receive it.
*/
func (w *Webhooks) store(
	ctx context.Context, tx *database.Tx, args jobs.FanOutWebhookEventArgs, workspace models.Workspace,
) error {
	q := models.New(tx)

	endpoints, err := w.subscribed(ctx, q, args)
	if err != nil || len(endpoints) == 0 {
		return err
	}

	ref := reference{kind: args.Subject, id: args.SubjectID}
	if err := q.LockWebhookObject(ctx, ref.kind+":"+ref.id.String()); err != nil {
		return fmt.Errorf("lock webhook object: %w", err)
	}

	snap, err := w.snapshot(ctx, args, ref)
	if err != nil || snap.Skip {
		return err
	}

	targets := slices.DeleteFunc(endpoints, func(e models.WebhookEndpoint) bool { return !receives(e, snap) })
	if len(targets) == 0 {
		return nil
	}

	sequence, err := q.NextWebhookSequence(ctx)
	if err != nil {
		return fmt.Errorf("number webhook snapshot: %w", err)
	}

	payload, err := w.encode(envelopeFor(args, sequence, workspace.TestMode), args.Data, snap.Object, ref)
	if err != nil {
		return err
	}

	for _, e := range targets {
		if err := w.insertDelivery(ctx, tx, delivery{
			args: args, endpoint: e, snap: snap,
			payload: payload, sequence: sequence,
		}); err != nil {
			return err
		}
	}

	return nil
}

/*
snapshot renders the object of the event, a tombstone when it is gone.
*/
func (w *Webhooks) snapshot(ctx context.Context, args jobs.FanOutWebhookEventArgs, ref reference) (Snapshot, error) {
	if ref.kind == subjectContactEvent {
		id := args.SubjectID

		return Snapshot{Object: contactEventObject(args), ContactID: &id}, nil
	}

	snap, err := w.Snapshots.Snapshot(ctx, args.WorkspaceID, ref.kind, ref.id)
	if err != nil {
		return snap, fmt.Errorf("snapshot webhook object: %w", err)
	}

	if snap.Object == nil {
		snap.Object = tombstone(ref)
	}

	return snap, nil
}

/*
receives reports whether an endpoint may receive the snapshot: notes
only when it includes them, and conversations only of its inboxes.
*/
func receives(e models.WebhookEndpoint, snap Snapshot) bool {
	if snap.Note && !e.IncludeNotes {
		return false
	}

	if e.InboxIds == nil || snap.InboxID == nil {
		return true
	}

	return slices.Contains(e.InboxIds, *snap.InboxID)
}

/*
delivery is one delivery to store.
*/
type delivery struct {
	args     jobs.FanOutWebhookEventArgs
	endpoint models.WebhookEndpoint
	snap     Snapshot
	payload  encoded
	sequence int64
}

/*
insertDelivery stores one delivery, once, and queues its first send.
*/
func (w *Webhooks) insertDelivery(ctx context.Context, tx *database.Tx, d delivery) error {
	id := d.args.SubjectID

	row, err := models.New(tx).CreateWebhookDelivery(ctx, models.CreateWebhookDeliveryParams{
		WorkspaceID: d.endpoint.WorkspaceID, EndpointID: d.endpoint.ID, EventID: d.args.EventID,
		EventType: d.args.Type, Sequence: d.sequence, ObjectID: &id, ContactID: d.snap.ContactID,
		Payload: d.payload.body, PayloadBytes: clampInt32(int64(len(d.payload.body))), Thin: d.payload.thin,
		Note: d.snap.Note, Now: w.Clock.Now(),
	})
	if errors.Is(err, database.ErrNotFound) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("store webhook delivery: %w", err)
	}

	return wrapf("queue webhook delivery", w.Queue.PushTx(ctx, tx, jobs.DeliverWebhookArgs{
		WorkspaceID: row.WorkspaceID, DeliveryID: row.ID, Trigger: triggerAuto,
	}))
}
