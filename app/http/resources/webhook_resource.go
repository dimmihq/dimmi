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
rawOrNull passes stored JSON through, null when nothing is stored.
*/
func rawOrNull(b []byte) json.RawMessage {
	if len(b) == 0 || !json.Valid(b) {
		return json.RawMessage(`null`)
	}

	return json.RawMessage(b)
}
