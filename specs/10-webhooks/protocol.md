# 10 Webhooks: protocol (public developer contract)

Style: caveman. This file becomes docs page `/docs/webhooks`. Stable within `api_version` `2026-11-01`. Standard Webhooks spec compliant (https://www.standardwebhooks.com, spec in github.com/standard-webhooks/standard-webhooks, verified 2026-11-01).

## Request

```
POST /webhooks/dimmi HTTP/1.1
Host: api.example.com
Content-Type: application/json
User-Agent: Dimmi-Webhooks/1.0 (+https://trydimmi.com/docs/webhooks)
webhook-id: evt_01jbf40
webhook-timestamp: 1793527200
webhook-signature: v1,ljGnIJqd/7KhrjgkZCDjChqYAfJI9ByHxCm9OHKXtwo=
dimmi-event-type: webhook.test
dimmi-delivery-id: whd_01jbf4
dimmi-attempt: 1
dimmi-api-version: 2026-11-01
dimmi-endpoint-id: whk_01jbf2k8q

{"id":"evt_01jbf40","object":"event","type":"webhook.test"}
```

(Body shortened; signature above is real for that exact body and secret `whsec_C2FVsBQIhrscChlQIMV+b5sSYspob7oD62BkjDVUOEk=`. Docs test asserts it.)

| Header | Signed | Meaning |
|---|---|---|
| `webhook-id` | yes | event id (`evt_...`). Same on every retry and redelivery. Idempotency key |
| `webhook-timestamp` | yes | Unix seconds at this attempt. New per attempt |
| `webhook-signature` | n/a | space-separated `v1,<base64>` list. Two entries during secret rotation |
| `dimmi-event-type` | no | convenience routing without parsing body |
| `dimmi-delivery-id` | no | `whd_...`, for support and log lookup |
| `dimmi-attempt` | no | 1-based attempt number |
| `dimmi-api-version` | no | envelope version |
| `dimmi-endpoint-id` | no | which endpoint config |

Header names lowercase (HTTP/2 lowercase anyway). Unsigned headers informational only; never trust for authorization.

## Signature

```
signed_content = webhook-id + "." + webhook-timestamp + "." + raw_body_bytes
key            = base64_decode(secret without "whsec_" prefix)   # 32 bytes
signature      = base64(HMAC-SHA256(key, signed_content))
header         = "v1," + signature
```

- Verify against raw body bytes. Never re-serialize JSON before verifying.
- Constant-time compare. Accept if any `v1,` entry matches.
- Reject timestamp older or newer than 5 minutes (library default).
- Secret format: `whsec_` + standard base64 (with padding) of 32 random bytes.

### Verify with official libraries

Node (`npm i standardwebhooks`):

```js
import { Webhook } from "standardwebhooks";
const wh = new Webhook(process.env.DIMMI_WEBHOOK_SECRET);
// Express: use express.raw({ type: "application/json" }) on this route
app.post("/webhooks/dimmi", (req, res) => {
  const event = wh.verify(req.body, req.headers); // throws on bad signature
  res.sendStatus(200);
  handle(event); // async after ack
});
```

Python (`pip install standardwebhooks`):

```python
from standardwebhooks.webhooks import Webhook
wh = Webhook(os.environ["DIMMI_WEBHOOK_SECRET"])
event = wh.verify(request.get_data(), dict(request.headers))
```

Go (`go get github.com/standard-webhooks/standard-webhooks/libraries/go`):

```go
wh, _ := standardwebhooks.NewWebhook(os.Getenv("DIMMI_WEBHOOK_SECRET"))
body, _ := io.ReadAll(r.Body)
if err := wh.Verify(body, r.Header); err != nil {
    http.Error(w, "bad signature", http.StatusUnauthorized)
    return
}
```

Also works: Svix libraries (`svix` npm/pypi/go `Webhook` class), same scheme. Ruby, PHP, Java, C#, Rust, Elixir, Kotlin libraries in Standard Webhooks repo.

## Envelope

```json
{
  "id": "evt_01jbf40",
  "object": "event",
  "type": "conversation.assigned",
  "api_version": "2026-11-01",
  "created_at": "2026-11-01T10:00:00.123456Z",
  "workspace_id": "ws_01ja",
  "sequence": 98231,
  "test": false,
  "test_mode": false,
  "truncated": false,
  "actor": { "type": "member", "id": "mem_01jb2", "name": "Jane Doe", "via": { "kind": "session", "id": null } },
  "data": {
    "object": { "...": "resource snapshot, same shape as REST GET" },
    "changes": { "assignee": { "from": null, "to": { "id": "mem_01jb2", "object": "member", "name": "Jane Doe" } } }
  }
}
```

| Field | Meaning |
|---|---|
| `id` | event id, equals `webhook-id` header |
| `type` | event type |
| `api_version` | envelope + resource version of endpoint |
| `created_at` | when event occurred (microseconds) |
| `sequence` | ordering hint per object (higher = newer snapshot for same `data.object.id`); not contiguous |
| `test` | true only for test API sends (fixture data, not real) |
| `test_mode` | workspace is sandbox |
| `truncated` | true when thin payload fallback used |
| `actor` | who caused it, 00-foundation/contracts.md `Actor`: `type` in `member`, `contact`, `agent`, `system`, `integration`; `id` null for system; `via` `{kind, id}` how request arrived (`session`, `personal_token`, `oauth_client`, `widget`, `channel`, `cli`, `mcp`). Token/OAuth calls show owning member + `via`, never type `token` |
| `data.object` | resource snapshot taken at fan-out (seconds after event) |
| `data.changes` | exact from/to of this event (state events), or changed keys (`contact.updated`) |

Extra `data` keys per type: `reason` (reopened, unsnoozed), `merged_ids` (contact.merged), `handoff` (ai.handoff_requested), `error` (channel.errored).

### Thin payload fallback

Serialized envelope over 256 KB (rare: huge custom attributes, long email bodies): `data.object` replaced with `{id, object, updated_at, links: {self}}`, `truncated: true`. Fetch full resource via REST (`links.self`). `data.changes` kept when under 16 KB else dropped.

### Deleted resources

`conversation.deleted`, `contact.deleted`, `message.deleted`: `data.object` = `{ "id": "...", "object": "...", "deleted": true }` (+ `number` for conversations). Erased contact (GDPR) later events show `"name": "Deleted contact"`, no email.

## Example payloads

Generated from `resources/fixtures/webhooks/*.json` (single source for docs, test API, `dimmi trigger`, tests). Fixture files exist for every catalog type, incl. changelog (`changelog_entry.created|updated|published|unpublished|deleted`, `changelog_mailing.completed`, `changelog_subscriber.subscribed|unsubscribed`), feedback (`feedback_post.*`, `feedback_vote.created|deleted`, `feedback_comment.created|updated|deleted`, `feedback_request.created|deleted`, `feedback_post_update.sent`) surveys (`survey_response.completed`), agent actions (`ai.action_call_completed`, `ai.action_call_approval_requested`, `ai.action_connection_failed`), docs sync (`docs_sync_run.completed|failed`, `article.freshness_changed`) and imports (`import.completed`). Objects shaped by owner spec Resources (07, 08, 13, 14, 15-announcements-surveys, 16). Signing uses the standard-webhooks Go library (`framework/webhooks`), so any official verifier works unchanged.

### conversation.created

```json
{
  "id": "evt_01jbc9x8",
  "object": "event",
  "type": "conversation.created",
  "api_version": "2026-11-01",
  "created_at": "2026-11-01T10:00:00.000000Z",
  "workspace_id": "ws_01ja",
  "sequence": 98001,
  "test": false,
  "test_mode": false,
  "truncated": false,
  "actor": { "type": "contact", "id": "ctc_01jbc9", "name": "Bob Smith" },
  "data": {
    "object": {
      "id": "conv_01jbc9x7",
      "object": "conversation",
      "number": 1042,
      "status": "open",
      "spam": false,
      "priority": false,
      "subject": "Checkout fails on Safari",
      "ai_state": "active",
      "inbox": { "id": "inbox_01ja", "object": "inbox", "name": "Support" },
      "channel": { "id": "chn_01ja", "object": "channel_connection", "type": "widget", "name": "Website" },
      "contact": { "id": "ctc_01jbc9", "object": "contact", "kind": "user", "name": "Bob Smith", "email": "bob@example.com", "external_id": "user_829" },
      "assignee": null,
      "team": null,
      "tags": [],
      "message_count": 1,
      "source": { "page_url": "https://example.com/checkout", "browser": "Safari 19", "os": "macOS" },
      "snoozed_until": null,
      "closed_at": null,
      "last_message_at": "2026-11-01T10:00:00Z",
      "created_at": "2026-11-01T10:00:00Z",
      "updated_at": "2026-11-01T10:00:00Z",
      "links": { "self": "https://app.trydimmi.com/api/v1/conversations/conv_01jbc9x7", "dashboard": "https://app.trydimmi.com/w/ws_01ja/conversations/conv_01jbc9x7" }
    }
  }
}
```

### message.created (customer message)

```json
{
  "id": "evt_01jbdq3w",
  "object": "event",
  "type": "message.created",
  "api_version": "2026-11-01",
  "created_at": "2026-11-01T10:02:11.000000Z",
  "workspace_id": "ws_01ja",
  "sequence": 98120,
  "test": false,
  "test_mode": false,
  "truncated": false,
  "actor": { "type": "contact", "id": "ctc_01jbc9", "name": "Bob Smith" },
  "data": {
    "object": {
      "id": "msg_01jbdq3w",
      "object": "message",
      "type": "message",
      "conversation": { "id": "conv_01jbc9x7", "object": "conversation", "number": 1042 },
      "author": { "type": "contact", "id": "ctc_01jbc9", "name": "Bob Smith" },
      "body": "Still broken after clearing cache. Screenshot attached.",
      "attachments": [ { "id": "att_01jbdq", "object": "attachment", "filename": "screenshot.png", "content_type": "image/png", "size_bytes": 184223, "url": null } ],
      "channel_message_id": null,
      "delivery_status": "not_applicable",
      "created_at": "2026-11-01T10:02:11Z",
      "updated_at": "2026-11-01T10:02:11Z"
    }
  }
}
```

Attachment `url` null in webhooks (signed URLs expire); fetch via `GET /api/v1/attachments/{id}` for fresh signed URL.

### message.created (internal note, only with include_notes)

`data.object.type = "note"`, `author.type = "member"`, `body` contains mentions as `@[Jane Doe](mem_01jb2)`.

### conversation.assigned

`data.changes`: `{"assignee": {"from": {...} | null, "to": {...} | null}, "team": {"from": ..., "to": ...}}` (only keys that changed).

### ai.handoff_requested

`data.object` conversation; `data.handoff`: `{"reason": "customer_requested" | "low_confidence" | "policy" | "error", "summary": "Customer cannot apply coupon on Safari; tried clearing cache.", "agent": {"id": "...", "object": "ai_agent", "name": "Dimmi"}}`. Reason values and agent id prefix owned by 07; shown illustrative.

### changelog_entry.published (13)

```json
{ "id": "evt_01jbg10", "object": "event", "type": "changelog_entry.published", "api_version": "2026-11-01", "created_at": "2026-11-01T10:00:00Z", "workspace_id": "ws_01ja", "sequence": 3, "test": false, "test_mode": false, "actor": { "type": "member", "id": "mem_01jb", "via": { "kind": "session", "id": null } }, "data": { "object": { "id": "cle_01jbg0", "object": "changelog_entry", "title": "Dark mode", "status": "published", "published_at": "2026-11-01T10:00:00Z", "labels": ["new"], "url": "https://help.acme.com/changelog/dark-mode", "links": { "self": "https://app.trydimmi.com/api/v1/changelog-entries/cle_01jbg0" } }, "feedback_post_ids": ["fbp_01jb7"] } }
```

### webhook.test

```json
{ "id": "evt_01jbf40", "object": "event", "type": "webhook.test", "api_version": "2026-11-01", "created_at": "2026-11-01T10:00:00Z", "workspace_id": "ws_01ja", "sequence": 0, "test": true, "test_mode": false, "truncated": false, "actor": { "type": "member", "id": "mem_01jb2", "name": "Jane Doe" }, "data": { "object": { "object": "webhook_test", "message": "Hello from Dimmi" } } }
```

Test with `type` set: fixture of that type, `test: true`, fresh `evt_` id. Fixture object ids are freshly generated valid prefixed ids that do not exist in workspace (REST lookup returns 404). Consumers should branch on `test: true`.

## Responses we expect

| Your response | We do |
|---|---|
| 2xx within 15 s | success; body ignored (first 4 KB logged) |
| 410 Gone | disable endpoint (`gone`), email owners/admins |
| 429, 502, 503, 504 | retry; honor `Retry-After` (seconds or HTTP date) if later than schedule |
| 3xx | failure (redirects not followed; update URL) |
| other 4xx, 5xx | retry |
| timeout (connect 5 s, total 15 s), DNS, TLS, connection errors | retry |

Best practice (documented): verify, enqueue, return 200 fast; process async; dedupe by `webhook-id`.

## Retry schedule

Attempt times after first attempt, each ±20% jitter:

| Attempt | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12 |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| At | 0 | 30 s | 2 m | 10 m | 30 m | 1 h | 2 h | 4 h | 8 h | 14 h | 20 h | 24 h |

After attempt 12 fails: delivery `failed`. Manual redeliver any time within retention (30 days).

## Endpoint lifecycle

- Healthy: any success resets `failing_since`.
- Failing: no success since first failure. At 24 h: email + in-app to owners/admins with failing count, last error, log link.
- Disabled: failing 72 h, or 410 response, or manual. Pending retries canceled. Email + in-app. Re-enable via PATCH `enabled: true`; then redeliver missed deliveries from log (bulk recover later).

## Secret rotation

1. `POST /webhook-endpoints/{id}/secret-rotations {"expire_previous_in_seconds": 86400}` returns new secret.
2. During overlap every request carries `webhook-signature: v1,<new> v1,<old>`. Libraries accept either.
3. Deploy new secret to consumer.
4. Old secret expires automatically (or rotate with `0` for immediate cut, e.g. leaked secret).

## Ordering and duplicates

- Unordered. Parallel delivery and retries reorder.
- Per object: keep highest `sequence` seen per `data.object.id`, ignore lower.
- Duplicates possible: dedupe on `webhook-id` (keep 7 days).
- Redelivery sends original snapshot (same body). For current state, fetch via REST.

## Versioning

- `api_version` pinned per endpoint at creation (default current; same dates as REST `Dimmi-Version` header, one versioning scheme). Payload snapshot built for endpoint's version: endpoints on different versions receive different shapes for same event. Additive changes (new fields, new event types) ship without version bump; consumers must ignore unknown fields.
- Breaking change: new `api_version` date; endpoint upgrades via PATCH; old version supported 6 months minimum (api.md deprecation rule).
- Wildcard subscriptions (`*`, `conversation.*`) receive new event types automatically.

## Local development

- Expose local server through any public tunnel, create test endpoint pointing at it, use test-mode workspace.
- `dimmi trigger conversation.created --endpoint whk_...` sends fixture via test API (`POST /webhook-endpoints/{id}/tests`).
- `dimmi listen` local forwarding (no tunnel): later (`dimmi-listen`, README Out).
