/*
|--------------------------------------------------------------------------
| Attempt Outcomes
|--------------------------------------------------------------------------
|
| What one attempt leaves behind. Its row in the log is numbered after
| every earlier attempt, scheduled or asked for, under a lock on the
| delivery so two attempts never take the same number. A 2xx delivers
| the delivery and makes the endpoint healthy. A scheduled attempt that
| failed waits for the next time on the schedule, or fails the delivery
| once the schedule is done. A redelivery that failed leaves a running
| schedule as it was. A test never changes the endpoint's health, and
| an endpoint deleted mid-attempt simply has nothing to record.
|
*/

package webhooks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/dimmihq/dimmi/framework/database"

	"github.com/dimmihq/dimmi/app/models"
)

/*
Delivery statuses.
*/
const (
	statusPending   = "pending"
	statusRetrying  = "retrying"
	statusSucceeded = "succeeded"
	statusFailed    = "failed"
)

/*
statusGone is the answer that turns an endpoint off.
*/
const statusGone = 410

/*
recordOutcome stores the attempt and what it means for the delivery
and its endpoint.
*/
func (w *Webhooks) recordOutcome(
	ctx context.Context, t target, r sent,
) (models.WebhookDelivery, models.WebhookDeliveryAttempt, error) {
	d, a := t.delivery, models.WebhookDeliveryAttempt{}

	err := w.inWorkspace(ctx, d.WorkspaceID, func(ctx context.Context, tx *database.Tx, q *models.Queries) error {
		current, err := q.LockWebhookDelivery(ctx, models.LockWebhookDeliveryParams{
			WorkspaceID: d.WorkspaceID, ID: d.ID,
		})
		if err != nil {
			return err
		}

		var auto int
		if a, auto, err = logAttempt(ctx, q, current, r); err != nil {
			return err
		}

		if d, err = q.SaveWebhookDeliveryOutcome(ctx, decide(current, r, auto)); err != nil {
			return fmt.Errorf("save webhook delivery outcome: %w", err)
		}

		if r.trigger == triggerTest {
			return nil
		}

		return w.updateHealth(ctx, tx, t.endpoint, r)
	})
	if errors.Is(err, database.ErrNotFound) {
		return d, a, nil
	}

	return d, a, err
}

/*
logAttempt writes the attempt's row and returns it with how many
scheduled attempts the delivery has had, this one included.
*/
func logAttempt(
	ctx context.Context, q *models.Queries, d models.WebhookDelivery, r sent,
) (models.WebhookDeliveryAttempt, int, error) {
	earlier, err := q.ListWebhookDeliveryAttempts(ctx, models.ListWebhookDeliveryAttemptsParams{
		WorkspaceID: d.WorkspaceID, DeliveryID: d.ID,
	})
	if err != nil {
		return models.WebhookDeliveryAttempt{}, 0, fmt.Errorf("list webhook attempts: %w", err)
	}

	auto := 0
	for _, a := range earlier {
		if a.Trigger == triggerAuto {
			auto++
		}
	}

	if r.trigger == triggerAuto {
		auto++
	}

	requestHeaders, _ := json.Marshal(r.headers)
	duration := clampInt32(r.duration.Milliseconds())
	params := models.CreateWebhookDeliveryAttemptParams{
		WorkspaceID: d.WorkspaceID, DeliveryID: d.ID, Trigger: r.trigger,
		Number:      clampInt32(int64(len(earlier)) + 1),
		RequestedAt: r.at, DurationMs: &duration, RequestHeaders: requestHeaders,
		ErrorCode: optional(r.errorCode), ErrorMessage: optional(r.errorText),
	}

	if r.status != 0 {
		status := clampInt32(int64(r.status))
		params.ResponseStatus, params.ResponseBody = &status, &r.body
		params.ResponseHeaders, _ = json.Marshal(r.respHeader)
	}

	attempt, err := q.CreateWebhookDeliveryAttempt(ctx, params)

	return attempt, auto, wrapf("record webhook attempt", err)
}

/*
decide is what the attempt leaves the delivery as.
*/
func decide(d models.WebhookDelivery, r sent, auto int) models.SaveWebhookDeliveryOutcomeParams {
	out := models.SaveWebhookDeliveryOutcomeParams{
		WorkspaceID: d.WorkspaceID, ID: d.ID, Status: statusFailed, AttemptCount: d.AttemptCount + 1,
		AttemptedAt: r.at, LastErrorCode: optional(r.errorCode),
	}

	if r.status != 0 {
		status := clampInt32(int64(r.status))
		out.LastResponseStatus = &status
	}

	switch {
	case success(r.status):
		out.Status, out.SucceededAt = statusSucceeded, &r.at
	case r.trigger == triggerTest || r.status == statusGone:
	case r.trigger == triggerManual:
		if d.Status == statusRetrying {
			out.Status, out.NextAttemptAt = statusRetrying, d.NextAttemptAt
		}
	default:
		first := r.at
		if d.FirstAttemptAt != nil {
			first = *d.FirstAttemptAt
		}

		if next, ok := nextAttempt(d.ID, first, auto, r); ok {
			out.Status, out.NextAttemptAt = statusRetrying, &next
		}
	}

	return out
}

/*
updateHealth marks the endpoint healthy after a delivered request,
failing after a failed one, and turns it off when it answered gone.
*/
func (w *Webhooks) updateHealth(
	ctx context.Context, tx *database.Tx, e models.WebhookEndpoint, r sent,
) error {
	q := models.New(tx)
	if success(r.status) {
		return wrapf("record webhook success", q.RecordWebhookEndpointSuccess(ctx,
			models.RecordWebhookEndpointSuccessParams{WorkspaceID: e.WorkspaceID, ID: e.ID, Now: new(r.at)}))
	}

	if err := q.RecordWebhookEndpointFailure(ctx, models.RecordWebhookEndpointFailureParams{
		WorkspaceID: e.WorkspaceID, ID: e.ID, Now: new(r.at),
	}); err != nil {
		return fmt.Errorf("record webhook failure: %w", err)
	}

	if r.status == statusGone {
		return w.disable(ctx, tx, e, disabledGone)
	}

	return nil
}

/*
optional writes an empty string as nil.
*/
func optional(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

/*
clampInt32 narrows a count or duration to the column's range, holding
it at the largest or smallest value it can store.
*/
func clampInt32(n int64) int32 {
	if n > math.MaxInt32 {
		return math.MaxInt32
	}

	if n < math.MinInt32 {
		return math.MinInt32
	}

	return int32(n)
}
