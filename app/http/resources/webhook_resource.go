/*
|--------------------------------------------------------------------------
| Webhook Resources
|--------------------------------------------------------------------------
|
| A webhook endpoint as its managers see it: where it posts, what it
| subscribes to, whether it is on and how healthy it is. Its secret is
| never part of it; it is shown once on create and on a reveal or a
| rotation. A delivery is one event sent to one endpoint, with its
| attempts on the detail.
|
*/

package resources

import (
	"encoding/json"
	"time"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/actions/webhooks"
	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)

/*
WebhookEndpoint is one endpoint.
*/
type WebhookEndpoint struct {
	ID                      string            `json:"id"`
	Object                  string            `json:"object"`
	URL                     string            `json:"url"`
	Description             string            `json:"description"`
	EventTypes              []string          `json:"event_types"`
	IncludeNotes            bool              `json:"include_notes"`
	InboxIDs                []string          `json:"inbox_ids"`
	APIVersion              string            `json:"api_version"`
	Metadata                map[string]string `json:"metadata"`
	Enabled                 bool              `json:"enabled"`
	DisabledReason          *string           `json:"disabled_reason"`
	DisabledAt              *time.Time        `json:"disabled_at"`
	Health                  string            `json:"health"`
	FailingSince            *time.Time        `json:"failing_since"`
	LastSuccessAt           *time.Time        `json:"last_success_at"`
	LastFailureAt           *time.Time        `json:"last_failure_at"`
	PreviousSecretExpiresAt *time.Time        `json:"previous_secret_expires_at"`
	CreatedAt               time.Time         `json:"created_at"`
	UpdatedAt               time.Time         `json:"updated_at"`
}

/*
NewWebhookEndpoint shapes the endpoint. A nil inbox filter, every
inbox, is null.
*/
func NewWebhookEndpoint(e models.WebhookEndpoint) WebhookEndpoint {
	metadata := map[string]string{}
	_ = json.Unmarshal(e.Metadata, &metadata)

	out := WebhookEndpoint{
		ID: webhooks.EndpointPublicID(e.ID), Object: "webhook_endpoint", URL: e.Url, Description: e.Description,
		EventTypes: e.EventTypes, IncludeNotes: e.IncludeNotes, APIVersion: e.ApiVersion, Metadata: metadata,
		Enabled: e.Enabled, DisabledReason: e.DisabledReason, DisabledAt: e.DisabledAt, Health: e.Health,
		FailingSince: e.FailingSince, LastSuccessAt: e.LastSuccessAt, LastFailureAt: e.LastFailureAt,
		PreviousSecretExpiresAt: e.PreviousSecretExpiresAt, CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}

	if out.EventTypes == nil {
		out.EventTypes = []string{}
	}

	if e.InboxIds != nil {
		out.InboxIDs = make([]string, 0, len(e.InboxIds))
		for _, id := range e.InboxIds {
			out.InboxIDs = append(out.InboxIDs, models.PublicID(models.PrefixInbox, id))
		}
	}

	return out
}

/*
NewWebhookEndpoints shapes a list of endpoints, in the order given.
*/
func NewWebhookEndpoints(rows []models.WebhookEndpoint) []WebhookEndpoint {
	out := make([]WebhookEndpoint, 0, len(rows))
	for _, e := range rows {
		out = append(out, NewWebhookEndpoint(e))
	}

	return out
}

/*
WebhookSecret is an endpoint's signing secret and, while a rotation
overlaps, the previous one with when it stops signing.
*/
type WebhookSecret struct {
	Object          string     `json:"object"`
	EndpointID      string     `json:"endpoint_id"`
	Secret          string     `json:"secret"`
	PreviousSecret  *string    `json:"previous_secret"`
	PreviousExpires *time.Time `json:"previous_expires_at"`
}

/*
NewWebhookSecret shapes an endpoint's secrets.
*/
func NewWebhookSecret(endpoint ids.UUID, s webhooks.Secrets) WebhookSecret {
	out := WebhookSecret{
		Object: "webhook_secret", EndpointID: webhooks.EndpointPublicID(endpoint), Secret: s.Current,
		PreviousExpires: s.PreviousExpires,
	}

	if s.Previous != "" {
		out.PreviousSecret = &s.Previous
	}

	return out
}

/*
WebhookEndpointStats are an endpoint's attempts over the last day.
*/
type WebhookEndpointStats struct {
	Object    string `json:"object"`
	Attempts  int64  `json:"attempts"`
	Succeeded int64  `json:"succeeded"`
	P50Ms     int64  `json:"p50_ms"`
	P95Ms     int64  `json:"p95_ms"`
}

/*
NewWebhookEndpointStats shapes an endpoint's numbers.
*/
func NewWebhookEndpointStats(s webhooks.Stats) WebhookEndpointStats {
	return WebhookEndpointStats{
		Object: "webhook_endpoint_stats", Attempts: s.Attempts, Succeeded: s.Succeeded, P50Ms: s.P50Ms, P95Ms: s.P95Ms,
	}
}

/*
WebhookEventType is one event an endpoint can subscribe to. ObjectType
is the kind of object its payload carries.
*/
type WebhookEventType struct {
	Object      string `json:"object"`
	Type        string `json:"type"`
	ObjectType  string `json:"object_type"`
	Group       string `json:"group"`
	Description string `json:"description"`
	NotesOptIn  bool   `json:"notes_opt_in"`
	NamedOnly   bool   `json:"named_only"`
}

/*
NewWebhookEventTypes shapes the catalog.
*/
func NewWebhookEventTypes(rows []appevents.WebhookEventType) []WebhookEventType {
	out := make([]WebhookEventType, 0, len(rows))
	for _, t := range rows {
		out = append(out, WebhookEventType{
			Object: "webhook_event_type", Type: t.Type, ObjectType: t.Object, Group: t.Group,
			Description: t.Description, NotesOptIn: t.NotesOptIn, NamedOnly: t.NamedOnly,
		})
	}

	return out
}

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
WebhookDeliveryDetail is a delivery with the exact payload it sends and
its attempts, oldest first.
*/
type WebhookDeliveryDetail struct {
	WebhookDelivery

	Payload  json.RawMessage  `json:"payload"`
	Attempts []WebhookAttempt `json:"attempts"`
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
WebhookTestResult is a test send's delivery and how its one attempt
was answered.
*/
type WebhookTestResult struct {
	Object   string          `json:"object"`
	Delivery WebhookDelivery `json:"delivery"`
	Attempt  WebhookAttempt  `json:"attempt"`
}

/*
NewWebhookTestResult shapes a test send.
*/
func NewWebhookTestResult(r webhooks.TestResult) WebhookTestResult {
	return WebhookTestResult{
		Object: "webhook_test", Delivery: NewWebhookDelivery(r.Delivery), Attempt: NewWebhookAttempt(r.Attempt),
	}
}

/*
rawOrNull passes stored JSON through, null when nothing is stored.
*/
func rawOrNull(b []byte) json.RawMessage {
	if len(b) == 0 || !json.Valid(b) {
		return json.RawMessage(`null`)
	}

	return json.RawMessage(b)
}
