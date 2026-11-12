/*
|--------------------------------------------------------------------------
| Webhook Delivery Resources
|--------------------------------------------------------------------------
|
| A delivery of an event to an endpoint, its attempts and the result\nof a test send.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/app/actions/webhooks"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WebhookDelivery is one event sent, or to be sent, to one endpoint.
*/
type WebhookDelivery struct {
	ID                 string     `json:"id"`
	Object             string     `json:"object"`
	EndpointID         string     `json:"endpoint_id"`
	EventID            string     `json:"event_id"`
	EventType          string     `json:"event_type"`
	Sequence           int64      `json:"sequence"`
	PayloadBytes       int32      `json:"payload_bytes"`
	Thin               bool       `json:"thin"`
	Test               bool       `json:"test"`
	Status             string     `json:"status"`
	AttemptCount       int32      `json:"attempt_count"`
	NextAttemptAt      *time.Time `json:"next_attempt_at"`
	LastAttemptAt      *time.Time `json:"last_attempt_at"`
	SucceededAt        *time.Time `json:"succeeded_at"`
	LastResponseStatus *int32     `json:"last_response_status"`
	LastErrorCode      *string    `json:"last_error_code"`
	CreatedAt          time.Time  `json:"created_at"`
}

/*
WebhookAttempt is one request of a delivery and how it was answered.
*/
type WebhookAttempt struct {
	ID              string          `json:"id"`
	Object          string          `json:"object"`
	Number          int32           `json:"number"`
	Trigger         string          `json:"trigger"`
	RequestedAt     time.Time       `json:"requested_at"`
	DurationMs      *int32          `json:"duration_ms"`
	RequestHeaders  json.RawMessage `json:"request_headers"`
	ResponseStatus  *int32          `json:"response_status"`
	ResponseHeaders json.RawMessage `json:"response_headers"`
	ResponseBody    *string         `json:"response_body"`
	ErrorCode       *string         `json:"error_code"`
	ErrorMessage    *string         `json:"error_message"`
	Hint            string          `json:"hint,omitempty"`
}

/*
WebhookDeliveryDetail is a delivery with the exact payload it sends and
its attempts, oldest first.
*/
type WebhookDeliveryDetail struct {
	WebhookDelivery

	Payload  json.RawMessage  `json:"payload"`
	Attempts []WebhookAttempt `json:"attempts"`
}

/*
WebhookTestResult is a test send's delivery and how its one attempt
was answered.
*/
type WebhookTestResult struct {
	Object   string          `json:"object"`
	Delivery WebhookDelivery `json:"delivery"`
	Attempt  WebhookAttempt  `json:"attempt"`
}

/*
NewWebhookDeliveryRow shapes a delivery of an endpoint's log.
*/
func NewWebhookDeliveryRow(d models.ListWebhookDeliveriesRow) WebhookDelivery {
	return WebhookDelivery{
		ID: models.PublicID(models.PrefixWebhookDelivery, d.ID), Object: "webhook_delivery",
		EndpointID: webhooks.EndpointPublicID(d.EndpointID),
		EventID:    models.PublicID(models.PrefixWebhookEvent, d.EventID), EventType: d.EventType,
		Sequence: d.Sequence, PayloadBytes: d.PayloadBytes, Thin: d.Thin, Test: d.Test, Status: d.Status,
		AttemptCount: d.AttemptCount, NextAttemptAt: d.NextAttemptAt, LastAttemptAt: d.LastAttemptAt,
		SucceededAt: d.SucceededAt, LastResponseStatus: d.LastResponseStatus, LastErrorCode: d.LastErrorCode,
		CreatedAt: d.CreatedAt,
	}
}

/*
NewWebhookDelivery shapes a stored delivery.
*/
func NewWebhookDelivery(d models.WebhookDelivery) WebhookDelivery {
	return NewWebhookDeliveryRow(models.ListWebhookDeliveriesRow{
		ID: d.ID, WorkspaceID: d.WorkspaceID, EndpointID: d.EndpointID, EventID: d.EventID, EventType: d.EventType,
		Sequence: d.Sequence, PayloadBytes: d.PayloadBytes, Thin: d.Thin, Test: d.Test, Status: d.Status,
		AttemptCount: d.AttemptCount, NextAttemptAt: d.NextAttemptAt, LastAttemptAt: d.LastAttemptAt,
		SucceededAt: d.SucceededAt, LastResponseStatus: d.LastResponseStatus, LastErrorCode: d.LastErrorCode,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt,
	})
}

/*
NewWebhookAttempt shapes one attempt.
*/
func NewWebhookAttempt(a models.WebhookDeliveryAttempt) WebhookAttempt {
	return WebhookAttempt{
		ID: models.PublicID(models.PrefixWebhookAttempt, a.ID), Object: "webhook_delivery_attempt",
		Number: a.Number, Trigger: a.Trigger, RequestedAt: a.RequestedAt, DurationMs: a.DurationMs,
		RequestHeaders: rawOrNull(a.RequestHeaders), ResponseStatus: a.ResponseStatus,
		ResponseHeaders: rawOrNull(a.ResponseHeaders), ResponseBody: a.ResponseBody, ErrorCode: a.ErrorCode,
		ErrorMessage: a.ErrorMessage, Hint: webhooks.VerifyHint(a.ResponseStatus),
	}
}

/*
NewWebhookDeliveryDetail shapes a delivery with its attempts.
*/
func NewWebhookDeliveryDetail(d webhooks.DeliveryDetail) WebhookDeliveryDetail {
	out := WebhookDeliveryDetail{
		WebhookDelivery: NewWebhookDelivery(d.Delivery), Payload: rawOrNull(d.Delivery.Payload),
		Attempts: make([]WebhookAttempt, 0, len(d.Attempts)),
	}

	for _, a := range d.Attempts {
		out.Attempts = append(out.Attempts, NewWebhookAttempt(a))
	}

	return out
}

/*
NewWebhookTestResult shapes a test send.
*/
func NewWebhookTestResult(r webhooks.TestResult) WebhookTestResult {
	return WebhookTestResult{
		Object: "webhook_test", Delivery: NewWebhookDelivery(r.Delivery), Attempt: NewWebhookAttempt(r.Attempt),
	}
}
