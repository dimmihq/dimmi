# 10 Webhooks (outbound)

Style: caveman. Date 2026-11-01. Scope trim 2026-11-02: `dimmi listen` (C6) and bulk recover (C7) moved to Out (later); `conversation.restored` (B20) dropped from catalog; `message.updated` = note edits only (C2). Final sweep 2026-11-02: sign/verify on `framework/webhooks` (standard-webhooks Go library), catalog entries for 13, 14, 15-announcements-surveys, over-limit code `entitlement_limit_reached`, Valkey for cache bust/pubsub/rate limits, integration-only app tests. Read with `specs/00-conventions/README.md` and `specs/00-foundation/*`. Backend only.

Scope boundary: outbound only. Inbound provider event webhooks `/webhooks/{postmark,resend,mailgun,sendgrid}/events` (and SES via SNS) belong to 04 (provider signature checks there); they share the `/webhooks/` path prefix (rest.md exceptions row) but no code, tables or catalog with this spec. Outbound endpoint URLs may never point at this instance's own `/webhooks/*` (create-time 422 `invalid_webhook_url`, loop guard).

Files: [README.md](README.md) (this), [protocol.md](protocol.md) (headers, signature, envelope, payload examples, verification code, retry schedule: public developer contract), [edge-cases.md](edge-cases.md), [tasks.md](tasks.md).

## Goal

Developers wire Dimmi into own app: every catalog event with `webhook=yes` delivered to their HTTPS endpoints, signed, retried 24 hours, observable, replayable. Verify with any Standard Webhooks library in one line.

Why: roadmap launch item 10 "Webhooks done right" (`research/roadmap.md`). Fin webhooks: HMAC-SHA1, no timestamp, one retry, subscriptions not API-manageable (`research/competitors/fin.md`). HelpDesk: thin docs, no payload schema (`research/competitors/helpdesk.md`). Productlane bar: HMAC-SHA256, backoff, 200-delivery log, email on auto-disable.

## Decision: Standard Webhooks, not custom `Dimmi-Signature`

Adopt Standard Webhooks spec (standardwebhooks.com, Apache-2.0, 1.8k stars, active 2026-10) exactly:
- Headers `webhook-id`, `webhook-timestamp`, `webhook-signature`.
- Signature `v1,<base64 HMAC-SHA256 of "{webhook-id}.{webhook-timestamp}.{body}">`, secret `whsec_<base64 32 bytes>`, space-separated multiple signatures during rotation.

Why: official verify libraries in 10+ languages (JS, Python, Go, Ruby, PHP, Java, C#, Rust, Elixir, Kotlin) plus Svix libraries work unchanged; zero custom verification code for users; rotation and replay protection designed in; same scheme Supabase/Svix-powered vendors use, so AI coding agents already know it. Rejected: Stripe-style `Dimmi-Signature: t=...,v1=...` (needs custom verifier per language). No second legacy header (one way to verify, less confusion). Informational `dimmi-*` headers added (not signed-critical).

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Event commit to first attempt start | p95 under 2 s, p99 under 5 s |
| Signature verify | passes official Standard Webhooks libraries (JS, Python, Go) in CI test |
| Retries | 12 attempts over 24 h, exponential with ±20% jitter |
| Delivery log | every attempt: request headers + body, response status, headers, body (first 4 KB), duration, error class; 30-day retention |
| Redeliver | one API call / button per delivery |
| Test | `POST /webhook-endpoints/{id}/tests` returns real attempt result synchronously under 16 s |
| Auto-disable | after 72 h of continuous failure (or 410), owners emailed at 24 h and at disable |
| Payload size | under 256 KB always; larger falls back to thin payload (never dropped) |
| Docs | every event type with example payload, generated from same fixtures as tests |
| Zero secrets in logs | signatures/secrets never logged; secrets encrypted at rest |

## Scope

### In (MVP)

- Endpoints CRUD: url, description, event types (exact, `prefix.*`, `*`), enabled, include_notes, inbox filter, api_version, metadata.
- Secret per endpoint, reveal, rotate with overlap (default 24 h, max 7 days).
- Fan-out from event catalog, resource snapshot envelope, per-object `sequence`.
- Delivery with `a.HTTP` (`framework/httpclient`) SSRF rules, no redirects, timeouts, response size cap, payload cap with thin fallback.
- Retry schedule over 24 h, jitter, Retry-After honoring, per-endpoint rate limit.
- Auto-disable + owner/admin notification (via 06 `system_alert`, email + in-app).
- Delivery log + attempts API, single redeliver, test event send, event type catalog API.
- MCP tools, metrics.

### Out (later)

- `GET /api/v1/events` (event history API, Stripe-style): post-MVP; delivery log covers debugging.
- Asymmetric signatures (`v1a` ed25519): later if requested; header format already compatible. standard-webhooks Go library has v1 HMAC only, so `v1a` would be own code on `crypto/ed25519` inside `framework/webhooks` (no current consumer: status page subscriber webhooks removed with 15-status-page).
- Custom headers per endpoint, transformations, payload templating: later.
- mTLS / static egress IPs for allowlisting: cloud later (document egress IP range when stable).
- Workflow HTTP step: post-MVP: rules lite.
- Inbound webhooks from user app (error-triggered proactive support): standout #3.
- Endpoint-level ordering guarantees (FIFO): never in MVP; documented as unordered.
- `dimmi-listen` (audit C6): `dimmi listen --forward-to localhost` server side (cli endpoint kind, SSE stream, result reporting, lease, replay): later. MVP local dev: public tunnel + test endpoint.
- `bulk-recover` (audit C7): redeliver all failed since timestamp (`POST /webhook-endpoints/{id}/recoveries`): later. MVP: single redeliver per delivery.

## User stories

- Developer: creates endpoint for `conversation.*` and `message.created` with curl, gets secret, verifies with `standardwebhooks` npm package in 3 lines.
- Developer: endpoint down for 2 hours during deploy; all events arrive after it recovers, no manual action.
- Developer: deploy bug returned 500 for a few events; filters delivery log `status=failed`, redelivers each after fix.
- Owner: gets email "Webhook endpoint api.example.com failing for 24 hours" with link to delivery log; later "disabled".
- Admin: rotates secret with 24 h overlap; deploys new secret; old one expires automatically.
- AI agent (MCP): lists failed deliveries for endpoint, redelivers after user confirms fix.

## Event types

Subscribable = catalog rows with `Webhook=yes` in `events-and-realtime.md`, plus new events below where flagged. Registry in `app/events/webhook_catalog.go` (type, object kind, description, fixture). Snapshot object per type:

| Event type(s) | `data.object` | Extra in `data` |
|---|---|---|
| conversation.created, .assigned, .closed, .reopened, .snoozed, .unsnoozed, .priority_changed, .tagged, .untagged, .marked_spam, .unmarked_spam | conversation (include contact, assignee, team, tags, inbox) | `changes` (from/to), `reason` where event has it |
| conversation.deleted | `{id, object, deleted: true, number}` | |
| message.created, .updated (note edits only; reply edit later: `message-edit`), .deleted | message (include conversation compact, author, attachments metadata) | `changes` for updated |
| message.delivery_updated | message compact + `delivery_status`, `delivery_error` | `changes` |
| contact.created, .updated | contact (include company) | `changes` keys list |
| contact.merged | contact (surviving) | `merged_ids` |
| contact.deleted | `{id, object, deleted: true}` | |
| ai.handoff_requested | conversation | `handoff` `{summary, reason, agent}` |
| ai.run_completed | ai_run (07 resource) | |
| article.published, .updated, .unpublished | article (08) | |
| article_suggestion.created | article_suggestion (08) | |
| channel.connected, .disconnected, .errored | channel_connection (never credentials) | `error` |
| billing.conversation_activated, billing.threshold_reached | usage ledger line / threshold (12) | |
| webhook.test | fixture of requested type, envelope `test: true` | only via test API |
| changelog_entry.created, .updated, .published, .unpublished, .deleted (13) | changelog_entry (13 Resource; `body_markdown` omitted when over 10 KB, `links.self` always) | `changes` for updated; published carries `feedback_post_ids` |
| changelog_mailing.completed (13) | changelog_mailing | counts (sent, failed, skipped) |
| changelog_subscriber.subscribed, .unsubscribed (13) | changelog_subscriber (email included; `contacts:read` not required: endpoint holder is admin) | `source`, unsubscribe `reason` |
| feedback_post.created, .updated, .status_changed, .moderated, .merged, .unmerged, .deleted, .restored (14) | feedback_post (include board, status, category) | `changes`; status_changed `from`/`to`, `notify`; merged `source_id`, `target_id` |
| feedback_vote.created, .deleted (14) | feedback_vote (contact compact) | `trust`, `source` |
| feedback_comment.created, .updated, .deleted (14) | feedback_comment | `visibility = internal` comments delivered only to endpoints with `include_notes = true` (same rule as notes); catalog `notes_opt_in: true` |
| feedback_request.created, .deleted (14) | feedback_request (conversation, message, contact, company compact) | |
| feedback_post_update.sent (14) | feedback_post_update | `kind`, `post_id`, `recipients_count`, `emailed_count` |
| ai.action_call_completed (07 agent actions) | ai_action_call (action compact, status, outcome, latency; never request/response payloads) | `conversation_id` |
| ai.action_call_approval_requested (07) | ai_action_call (include conversation compact) | `approval` `{required_role, expires_at}` |
| ai.action_connection_failed (07) | ai_action_connection (never secrets) | `reason` |
| docs_sync_run.completed, .failed (08 docs sync) | docs_sync_run (repo, PR ref, status, skip reason, suggestion count, review URL; never code) | `error_code` on failed |
| article.freshness_changed (08) | article_freshness (article compact, status, reasons) | `changes` (from/to status) |
| import.completed (16; only subscribable 16 event) | import_run (kind, status, counts, import compact; never credentials) | `errors_count` |
| audience.created, .updated, .deleted (18) | audience (id, name, description, rule tree, never member lists) | `changes` on updated |
| attribute_definition.created, .updated, .deleted (18) | attribute_definition (contact or company) | `changes` on updated |
| contact_event.tracked (18; off unless named in subscription, `*` and `contact_event.*` do not include it: high volume) | contact_event (name, contact compact, occurred_at, metadata) | |
| survey_response.completed (15-announcements-surveys) | survey_response (answers, survey compact, contact compact) | text answers raw (consumer treats as untrusted) |
| feature-added events with `Webhook=yes` in owner spec (00-accounts `workspace.*`, `member.*`, `team.*`; 03 `conversation.updated/merged/participants_changed`, `company.*`, `tag.*`, `contact.blocked/unblocked`; 04 email events; 05 `whatsapp_template.status_changed`; 07 `ai.state_changed`; 08 article/knowledge events; 11 `export.completed` workspace scope; 12 `billing.*`; 07 actions, 08 docs sync, 13, 14, 15-announcements-surveys, 16 rows above) | owner spec Resource | each owner adds registry entry (type, object, fixture) in `webhook_catalog.go` in same PR; CI test fails when catalog row with `Webhook=yes` lacks registry entry |

Internal notes excluded by default: any `message.*` event (`message.created`, `message.updated`, `message.deleted`) whose message has `type='note'` delivered only to endpoints with `include_notes=true` (default false); no delivery row otherwise. Other payloads never embed note bodies. `type='event'` system messages: never sent as `message.created` (duplicates `conversation.*`). Workspace `test_mode=true`: delivered with envelope `test_mode: true`.

Wildcards: `*` all current and future types; `conversation.*` prefix match on first segment. Future types join wildcard subscriptions automatically (documented), except `contact_event.tracked` (18): named only, high volume. Unknown type in subscription: 422.

## Data model

New ID prefixes: `whd_` deliveries, `wha_` attempts (`whk_`, `evt_` exist). Event ids are `evt_` of domain event UUIDv7.

### webhook_endpoints

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `whk_` |
| workspace_id | uuid not null fk cascade | |
| url | text not null | max 2048, normalized |
| description | text | max 255 |
| event_types | text[] not null | 1..100 entries; exact, `prefix.*`, or `*` |
| include_notes | boolean not null default false | |
| inbox_ids | uuid[] null | filter conversation/message events by inbox; null = all |
| api_version | text not null default '2026-11-01' | same date vocabulary as REST `Dimmi-Version` header (foundation `api.version`, extensibility.md section 11). Validated against supported list `webhooks.api_versions` (= foundation supported API versions). Pinned at create (request may pass older supported value); payload snapshot built for this version, so later shape change never breaks existing endpoints |
| enabled | boolean not null default true | |
| disabled_reason | text CHECK in ('manual','auto_failure','gone') | null when enabled |
| disabled_at | timestamptz | |
| secret_encrypted | bytea not null | `whsec_...` plaintext inside envelope |
| previous_secret_encrypted | bytea | rotation overlap |
| previous_secret_expires_at | timestamptz | |
| health | text not null CHECK in ('healthy','failing') default 'healthy' | |
| failing_since | timestamptz | first failed final-or-retrying attempt after last success |
| failing_notified_at | timestamptz | 24 h warning sent |
| last_success_at, last_failure_at | timestamptz | |
| metadata | jsonb not null default '{}' | user key/values, max 20 keys, 500 chars values |
| created_by_type, created_by_id | text, uuid | actor |
| created_at, updated_at | timestamptz | |

Indexes: `(workspace_id, enabled)`; unique `(workspace_id, url)`. Limit endpoints per workspace = entitlement key `webhooks.http_endpoints` (self-host driver reads config `webhooks.max_endpoints`, default 20; cloud catalog default 20; per-customer `entitlement_overrides`), read via `Entitlements.Limit`.

### webhook_deliveries

One row per (event, endpoint). Payload snapshot stored once, identical bytes on every attempt.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `whd_` |
| workspace_id | uuid not null | |
| endpoint_id | uuid not null fk webhook_endpoints cascade | |
| event_id | uuid not null | `webhook-id` = `evt_` encoding |
| event_type | text not null | |
| sequence | bigint not null | from Postgres sequence `webhook_event_sequence`, see ordering |
| object_id | uuid | subject resource id |
| contact_id | uuid null | contact the payload is about (conversation/message/contact events); GDPR purge key, partial index `(workspace_id, contact_id) WHERE contact_id IS NOT NULL` |
| payload | bytea not null | exact JSON bytes signed (column `SET COMPRESSION lz4`) |
| payload_bytes | int not null | |
| thin | boolean not null default false | fell back to thin payload |
| test | boolean not null default false | from test API |
| status | text not null CHECK in ('pending','retrying','succeeded','failed','canceled') | |
| attempt_count | int not null default 0 | |
| next_attempt_at | timestamptz | |
| last_attempt_at, succeeded_at | timestamptz | |
| last_response_status | int | |
| last_error_code | text | see error classes in protocol.md |
| created_at, updated_at | timestamptz | |

Unique `(endpoint_id, event_id)` (fan-out idempotency; test deliveries use fresh event id). Indexes: `(workspace_id, endpoint_id, created_at desc, id desc)`, `(workspace_id, event_id)`.

### webhook_delivery_attempts

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `wha_` |
| workspace_id | uuid not null | |
| delivery_id | uuid not null fk cascade | |
| number | int not null | 1-based across auto and manual |
| trigger | text not null CHECK in ('auto','manual','test') | |
| requested_at | timestamptz not null | |
| duration_ms | int | |
| request_headers | jsonb not null | all sent headers (signature included; not secret) |
| response_status | int | null on network error |
| response_headers | jsonb | first 30 headers, values max 512 chars |
| response_body | text | first 4 KB, invalid UTF-8 replaced, NUL stripped |
| error_code | text | `timeout`, `connect_refused`, `dns_failed`, `tls_failed`, `ssrf_blocked`, `redirect_not_followed`, `response_too_large`, `http_error`, `signing_unavailable`, `workspace_suspended` |
| error_message | text | human, max 500 |
| remote_ip | inet | resolved IP actually connected |
| created_at | timestamptz | |

Unique `(delivery_id, number)`. Index `(workspace_id, delivery_id, number)`.

### Sequence

`CREATE SEQUENCE webhook_event_sequence AS bigint` (global, not per tenant, values leak nothing but volume order; acceptable, documented).

### Core table changes

None.

### Migrations

| File | Creates |
|---|---|
| `database/migrations/20261101001001_create_webhook_endpoints_table.sql` | webhook_endpoints |
| `database/migrations/20261101001002_create_webhook_deliveries_table.sql` | webhook_deliveries + `webhook_event_sequence` |
| `database/migrations/20261101001003_create_webhook_delivery_attempts_table.sql` | webhook_delivery_attempts |
| `database/migrations/20261101001004_add_webhook_rls_policies.sql` | RLS (cloud) |

## Ordering and idempotency (consumer contract)

- No ordering guarantee across events. Retries and parallel workers reorder.
- `sequence`: assigned at fan-out under `pg_advisory_xact_lock(hashtextextended(object_id::text, 0))` after snapshot read in same tx. Guarantee: for same `data.object.id`, higher `sequence` never carries older snapshot. Consumers: drop event if `sequence <= last_seen_sequence[object.id]`. Values not contiguous.
- `data.object.updated_at` also present for resources with it.
- `webhook-id` (= event id) stable across retries and redeliveries. Consumers dedupe on it (store 7 days; retries span 24 h, manual redeliver up to 30 days).
- Snapshot reflects state at fan-out (usually under 1 s after event), not exact event instant; `data.changes` carries exact from/to of event.
- At-least-once. Duplicates possible (timeout after consumer processed).

## Code map

```
app/
├── enums/
│   ├── webhook_delivery_status.go
│   ├── webhook_attempt_trigger.go
│   └── webhook_error_code.go
├── events/
│   ├── webhook_catalog.go                 # registry: type -> object kind, description, webhook flag, fixture fn
│   ├── webhook_endpoint_disabled.go
│   └── webhook_endpoint_failing.go
│   (no app/support/webhooksig: sign/verify/secret = framework/webhooks on standard-webhooks Go library;
│    `webhooks.NewSecret`, `ParseSecret`, `Sign` (multi-signature header = join per-secret v1 outputs with spaces), `Verify`)
├── actions/webhooks/
│   ├── create_webhook_endpoint.go
│   ├── update_webhook_endpoint.go
│   ├── delete_webhook_endpoint.go
│   ├── reveal_webhook_endpoint_secret.go
│   ├── rotate_webhook_endpoint_secret.go
│   ├── fan_out_webhook_event.go           # match endpoints, build snapshot once per distinct endpoint `api_version` (MVP: one version, one build), insert deliveries + jobs
│   ├── build_webhook_payload.go           # envelope, resource snapshot for given `api_version` (version transformers keyed by date, none at MVP), size cap, thin fallback
│   ├── attempt_webhook_delivery.go        # sign (`framework/webhooks`), send via `a.HTTP` webhook profile, classify, record attempt
│   ├── redeliver_webhook_delivery.go
│   ├── send_test_webhook.go
│   ├── disable_webhook_endpoint.go        # auto + gone + manual paths, emits webhook_endpoint.disabled
│   ├── evaluate_webhook_endpoint_health.go
│   ├── erase_webhook_contact_data.go      # EraseWebhookContactData: GDPR contact eraser (00-foundation/contracts.md "Contact eraser hooks"), registered on 03 eraser registry
├── http/
│   ├── controllers/api/v1/
│   │   ├── webhook_endpoint_controller.go               # Index, Store, Show, Update, Destroy
│   │   ├── webhook_endpoint_secret_controller.go        # Show
│   │   ├── webhook_endpoint_secret_rotation_controller.go # Store
│   │   ├── webhook_endpoint_test_controller.go          # Store (POST /webhook-endpoints/{id}/tests)
│   │   ├── webhook_endpoint_delivery_controller.go      # Index
│   │   ├── webhook_delivery_controller.go               # Show
│   │   ├── webhook_delivery_attempt_controller.go       # Index
│   │   ├── webhook_delivery_redelivery_controller.go    # Store
│   │   └── webhook_event_type_controller.go             # Index
│   ├── requests/
│   │   ├── store_webhook_endpoint_request.go
│   │   ├── update_webhook_endpoint_request.go
│   │   ├── store_webhook_secret_rotation_request.go
│   │   ├── store_webhook_test_request.go
│   │   └── index_webhook_deliveries_request.go
│   └── resources/
│       ├── webhook_endpoint_resource.go
│       ├── webhook_delivery_resource.go
│       ├── webhook_delivery_attempt_resource.go
│       ├── webhook_event_type_resource.go
│       └── webhook_event_resource.go      # envelope (used by payload builder and fixtures)
├── listeners/
│   ├── queue_webhook_deliveries.go        # foundation name; queued, InsertTx FanOutWebhookEvent
│   └── invalidate_webhook_endpoint_cache.go
├── jobs/
│   ├── fan_out_webhook_event.go
│   ├── deliver_webhook.go
│   ├── check_webhook_endpoint_health.go
│   ├── expire_webhook_endpoint_secrets.go
│   └── prune_webhook_deliveries.go
├── notifications/
│   ├── webhook_endpoint_failing_notification.go   # system_alert via 06 (mail + in_app), owners + admins
│   ├── webhook_endpoint_disabled_notification.go
│   └── webhook_endpoint_created_notification.go   # security notice to owners (mail)
├── policies/webhook_endpoint_policy.go
├── mcp/tools/
│   ├── list_webhook_endpoints.go          # dimmi_list_webhook_endpoints
│   ├── create_webhook_endpoint.go         # dimmi_create_webhook_endpoint
│   ├── update_webhook_endpoint.go         # dimmi_update_webhook_endpoint
│   ├── list_webhook_deliveries.go         # dimmi_list_webhook_deliveries
│   ├── redeliver_webhook_delivery.go      # dimmi_redeliver_webhook_delivery
│   └── send_test_webhook.go               # dimmi_send_test_webhook
app/console/commands/webhooks_fixtures_command.go   # dimmi webhooks:fixtures (dev): regenerate resources/fixtures/webhooks
config/webhooks.go
database/
├── migrations/20261101001001..004
├── queries/webhook_endpoints.sql, webhook_deliveries.sql, webhook_delivery_attempts.sql
└── factories/webhook_endpoint_factory.go, webhook_delivery_factory.go
resources/views/mail/webhooks/ (failing.templ, disabled.templ, created.templ)
resources/fixtures/webhooks/<event_type>.json   # generated fixtures, used by docs, test API, `dimmi trigger`
tests/
├── feature/webhooks/
│   ├── webhook_endpoints_api_test.go
│   ├── webhook_secret_rotation_test.go
│   ├── webhook_fan_out_test.go            # matching, wildcards, notes opt-in, inbox filter, test_mode
│   ├── webhook_delivery_test.go           # status classes, redirects, timeouts, size caps
│   ├── webhook_retry_schedule_test.go
│   ├── webhook_auto_disable_test.go
│   ├── webhook_redelivery_test.go
│   └── webhook_mcp_tools_test.go
│   ├── webhook_payload_test.go            # thin fallback, sequence, golden per catalog type (via real fan-out)
│   ├── webhook_event_type_matcher_test.go # wildcards through endpoint create + fan-out (integration, no unit mocks)
│   └── webhook_catalog_test.go            # every Webhook=yes type has fixture + Resource (13, 14, 15-announcements-surveys incl.)
(sign vectors from Standard Webhooks repo live in framework/webhooks unit tests)
└── contracts/standard_webhooks_compat_test.go  # node + python verify in CI job (outside go test, needs runtimes)
```

## HTTP API

Base `/api/v1`, middleware `auth` (`A`: session, first-party app or token; abilities apply to tokens), `workspace`, `throttle:api`, `idempotent` on POST. Policies call `authz.Can` with permissions `webhooks:read` (GET) and `webhooks:write` (writes) (extensibility.md P2); MVP `roles.go` grants both to owner, admin only (members 403 `forbidden`). Token abilities = same strings (one vocabulary).

| Method | Path | Route name | Ability | Purpose |
|---|---|---|---|---|
| GET | /webhook-endpoints | webhook-endpoints.index | webhooks:read | list endpoints; `filter[enabled]` |
| POST | /webhook-endpoints | webhook-endpoints.store | webhooks:write | create; response includes `secret` once |
| GET | /webhook-endpoints/{endpoint} | webhook-endpoints.show | webhooks:read | includes `stats` (24 h: attempts, success rate, p50/p95 duration) |
| PATCH | /webhook-endpoints/{endpoint} | webhook-endpoints.update | webhooks:write | url, description, event_types, include_notes, inbox_ids, enabled, api_version, metadata |
| DELETE | /webhook-endpoints/{endpoint} | webhook-endpoints.destroy | webhooks:write | hard delete, cascade deliveries, cancel pending jobs |
| GET | /webhook-endpoints/{endpoint}/secret | webhook-endpoints.secret.show | webhooks:write | reveal current (+ previous while overlapping); audited |
| POST | /webhook-endpoints/{endpoint}/secret-rotations | webhook-endpoints.secret-rotations.store | webhooks:write | `{expire_previous_in_seconds}` default 86400, 0..604800; returns new secret |
| POST | /webhook-endpoints/{endpoint}/tests | webhook-endpoints.tests.store | webhooks:write | `{type?}` default `webhook.test`; one attempt, sync result: stored `webhook_delivery` with `test: true` (works on disabled endpoint) |
| GET | /webhook-endpoints/{endpoint}/deliveries | webhook-endpoints.deliveries.index | webhooks:read | `filter[status]`, `filter[event_type]`, `filter[event_id]`, `filter[created_at][gte]`; cursor |
| GET | /webhook-deliveries/{delivery} | webhook-deliveries.show | webhooks:read | `include=attempts,payload` (payload decoded JSON) |
| GET | /webhook-deliveries/{delivery}/attempts | webhook-deliveries.attempts.index | webhooks:read | full attempt log |
| POST | /webhook-deliveries/{delivery}/redeliveries | webhook-deliveries.redeliveries.store | webhooks:write | immediate manual attempt; 202 with attempt id; `?wait=true` waits for result (max 16 s) |
| GET | /webhook-event-types | webhook-event-types.index | webhooks:read | catalog: type, object, description, example payload, `notes_opt_in` flag |

### Examples

`POST /api/v1/webhook-endpoints`

```json
{
  "url": "https://api.example.com/webhooks/dimmi",
  "description": "Sync conversations to CRM",
  "event_types": ["conversation.*", "message.created", "contact.updated"],
  "include_notes": false,
  "inbox_ids": null,
  "metadata": { "env": "production" }
}
```

201:

```json
{
  "data": {
    "id": "whk_01jbf2k8q",
    "object": "webhook_endpoint",
    "url": "https://api.example.com/webhooks/dimmi",
    "description": "Sync conversations to CRM",
    "event_types": ["conversation.*", "message.created", "contact.updated"],
    "include_notes": false,
    "inbox_ids": null,
    "api_version": "2026-11-01",
    "enabled": true,
    "disabled_reason": null,
    "health": "healthy",
    "failing_since": null,
    "last_success_at": null,
    "secret": "whsec_C2FVsBQIhrscChlQIMV+b5sSYspob7oD62BkjDVUOEk=",
    "secret_rotation": null,
    "metadata": { "env": "production" },
    "created_at": "2026-11-01T10:00:00Z",
    "updated_at": "2026-11-01T10:00:00Z",
    "links": { "deliveries": "https://app.trydimmi.com/api/v1/webhook-endpoints/whk_01jbf2k8q/deliveries" }
  }
}
```

`secret` has value only in create, rotate, and reveal responses. Elsewhere key present with `null` (foundation "key always present" rule). `secret_rotation` = `{previous_expires_at}` while overlap active, else null.

`POST /api/v1/webhook-endpoints/{id}/tests` with `{ "type": "conversation.created" }`:

```json
{
  "data": {
    "object": "webhook_delivery",
    "id": "whd_01jbf3a",
    "event_id": "evt_01jbf3a0",
    "event_type": "conversation.created",
    "test": true,
    "status": "succeeded",
    "attempt": {
      "id": "wha_01jbf3a1",
      "number": 1,
      "trigger": "test",
      "response_status": 200,
      "duration_ms": 182,
      "error_code": null,
      "response_body": "{\"ok\":true}"
    }
  }
}
```

`GET /api/v1/webhook-deliveries/{id}?include=attempts`:

```json
{
  "data": {
    "id": "whd_01jbf4",
    "object": "webhook_delivery",
    "endpoint": { "id": "whk_01jbf2k8q", "object": "webhook_endpoint", "url": "https://api.example.com/webhooks/dimmi" },
    "event_id": "evt_01jbf40",
    "event_type": "message.created",
    "sequence": 98231,
    "status": "retrying",
    "attempt_count": 3,
    "next_attempt_at": "2026-11-01T10:12:40Z",
    "thin": false,
    "test": false,
    "payload_bytes": 2310,
    "last_response_status": 503,
    "last_error_code": "http_error",
    "created_at": "2026-11-01T10:02:01Z",
    "attempts": [
      { "id": "wha_1", "number": 1, "trigger": "auto", "requested_at": "2026-11-01T10:02:01Z", "duration_ms": 41, "response_status": 503, "error_code": "http_error", "response_body": "upstream unavailable" },
      { "id": "wha_2", "number": 2, "trigger": "auto", "requested_at": "2026-11-01T10:02:33Z", "duration_ms": 15000, "response_status": null, "error_code": "timeout", "response_body": null }
    ]
  }
}
```

### Error codes

| Status | Code | When |
|---|---|---|
| 403 | `forbidden` | member role |
| 403 | `missing_ability` | token ability |
| 404 | `not_found` | other workspace |
| 422 | `entitlement_limit_reached` | endpoints at entitlement `webhooks.http_endpoints` limit (default 20); shared foundation over-limit code (was `endpoint_limit_reached`); body `key: "webhooks.http_endpoints"`, `limit` |
| 409 | `endpoint_url_taken` | same URL exists in workspace |
| 409 | `rotation_in_progress` | rotate while previous secret not expired, unless `force: true` (expires older immediately) |
| 409 | `delivery_in_progress` | redeliver while attempt running (lock held) |
| 422 | validation | Laravel shape |
| 422 | `invalid_webhook_url` | scheme, userinfo, fragment, length, host IP literal in blocked range, `localhost` (cloud) |
| 422 | `insecure_webhook_url` | `http://` when `webhooks.require_https` |
| 422 | `unknown_event_type` | not in catalog and not valid wildcard |
| 422 | `api_version_unsupported` | |
| 429 | `throttled` | test 10/minute/endpoint |

URL validation at create resolves DNS once: private IP result is 422 `invalid_webhook_url` (cloud, or self-host without `http_client.allow_private_networks`). Delivery re-validates every attempt (DNS rebinding).

## MCP tools

| Tool | Ability | Notes |
|---|---|---|
| `dimmi_list_webhook_endpoints` | webhooks:read | no secrets |
| `dimmi_create_webhook_endpoint` | webhooks:write | requires `confirm: true` arg; MCP annotation `openWorldHint: true`, `destructiveHint: false`; tool description tells client to ask human. Never returns secret (human reveals in dashboard/CLI). Triggers owner security email |
| `dimmi_update_webhook_endpoint` | webhooks:write | `confirm: true` required when `url` or `event_types` or `include_notes` changes |
| `dimmi_list_webhook_deliveries` | webhooks:read | filter status, type; returns attempts summary |
| `dimmi_redeliver_webhook_delivery` | webhooks:write | single delivery |
| `dimmi_send_test_webhook` | webhooks:write | sync result |

Why confirm gates: endpoint creation exfiltrates all workspace data to arbitrary URL; prompt injection in customer message could ask agent to add one.

## CLI (owned by 09, server contract here)

- `dimmi trigger <event_type> --endpoint whk_`: wraps `POST /webhook-endpoints/{id}/tests` (`--endpoint` required).
- `dimmi webhooks endpoints list|show`, `dimmi webhooks deliveries --endpoint whk_ | show whd_`, `dimmi webhooks redeliver whd_ [--wait]`: wrap REST (09 `cli.md` "`dimmi webhooks` and `dimmi trigger`"). No endpoint create/update command at MVP (dashboard or `dimmi api`).

## Events

Consumed: every catalog event with `Webhook=yes`, including feature-added rows (listener `QueueWebhookDeliveries`). Member removal: endpoints created by member stay (workspace-owned). `workspace.suspended`: fan-out skips, pending deliveries snooze until `workspace.unsuspended`. `workspace.deleted`: pending deliveries `canceled`, no new fan-out.

New events (add to catalog):

| Event | Emitted by | Realtime | Webhook | Notes |
|---|---|---|---|---|
| webhook_endpoint.disabled | 10 | yes `workspace:{ws_}` (admins UI) | no | endpoint id, reason; same tx inserts 06 `Notify(system_alert)` job (10 notification class); 06 does not listen |
| webhook_endpoint.failing | 10 | yes `workspace:{ws_}` | no | at 24 h failure; same `Notify` path |
| webhook.test | 10 (test API only) | no | test only | synthetic; never subscribed, never broadcast, not a domain event |

Audit log (WriteAuditLog): endpoint created, updated (diff without secret), deleted, secret revealed, secret rotated, auto-disabled.

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| FanOutWebhookEvent | `webhooks` | `QueueWebhookDeliveries` listener InsertTx per event | 10, backoff 1 s..5 m | unique by `event_id` |
| DeliverWebhook | `webhooks` | inserted by fan-out per delivery; manual redeliver inserts new job | max 12 attempts, custom `NextRetry` from schedule | unique by `(delivery_id, attempt_trigger_group)`: one auto chain + one manual at a time |
| CheckWebhookEndpointHealth | `maintenance` | periodic every 5 min: failing notice at 24 h, disable at 72 h | default | periodic (leader) |
| ExpireWebhookEndpointSecrets | `maintenance` | periodic every 10 min: clear expired previous secrets | default | periodic (leader) |
| PruneWebhookDeliveries | `maintenance` | periodic daily 03:30 UTC: delete deliveries + attempts older than retention, batches of 5,000; calls `RetentionGuard.MayDestroy` per workspace batch (`{workspace, ws}`) and per delivery with `contact_id` (`{contact, id}`); false = rows kept, metric `retention_held` (noop driver at MVP) | default | periodic (leader) |

Retry schedule (cumulative from first attempt, each ±20% jitter, never earlier than previous + 5 s): 0, 30 s, 2 m, 10 m, 30 m, 1 h, 2 h, 4 h, 8 h, 14 h, 20 h, 24 h. Twelve attempts. `Retry-After` header on 429/503 honored if later than schedule (capped at schedule end 24 h). After last failure: delivery `failed`.

Per-endpoint rate limit: token bucket `webhooks.rate_limit` (default 25/s, burst 50) in Valkey (`a.Limiter`, `*ratelimit.Limiter` on Valkey; rebuildable, loss = bucket refills; Valkey unreachable: fails open per data-model.md policy with `log.Warn` + `dimmi_valkey_fallback_total{subsystem="webhooks"}`, bounded by queue concurrency); no token: `river.JobSnooze(1s..5s)` without consuming attempt.

## Delivery rules

- HTTP `POST`, body = stored payload bytes, `Content-Type: application/json`, `User-Agent: Dimmi-Webhooks/1.0 (+https://trydimmi.com/docs/webhooks)`.
- `a.HTTP` (`framework/httpclient`) with webhook profile: connect timeout 5 s, total 15 s, no redirects (3xx = failure `redirect_not_followed`, Location shown in log), response read max 64 KB then close (`response_too_large` only when status not 2xx; 2xx with big body still success, body truncated), HTTP/1.1 and HTTP/2, TLS verification on (no insecure skip).
- 2xx: success. 410: disable endpoint immediately (`gone`), delivery `failed`. 3xx, 4xx, 5xx, network errors: retry. 429, 502, 503, 504: retry, honor `Retry-After`.
- Endpoint disabled while deliveries retrying: pending jobs exit, deliveries `canceled`. Re-enable does not auto-resend; manual redeliver per delivery does.
- Endpoint deleted: cascade; running attempt finishes, write ignored.
- Imported records (16): `QueueWebhookDeliveries` skips `message.created` / `conversation.created` whose subject carries `metadata.imported` / `source.imported` (defense; 16 writers dispatch no per-record events). Only `import.completed` reaches endpoints for an import.

## Config keys

`config/webhooks.go`, env `DIMMI_WEBHOOKS_*`.

| Key | Default | Notes |
|---|---|---|
| webhooks.enabled | true | false: no fan-out (self-host can disable) |
| webhooks.queue_concurrency | 50 | `webhooks` queue |
| webhooks.max_endpoints | 20 | per workspace; self-host source of entitlement `webhooks.http_endpoints` (cloud uses catalog default + overrides) |
| webhooks.require_https | true cloud, false self-host | |
| webhooks.connect_timeout | 5s | |
| webhooks.timeout | 15s | Standard Webhooks recommends 15 to 30 s |
| webhooks.max_payload_bytes | 262144 | above: thin payload |
| webhooks.max_response_bytes | 65536 | read cap |
| webhooks.stored_response_bytes | 4096 | |
| webhooks.retry_schedule | `[0,30s,2m,10m,30m,1h,2h,4h,8h,14h,20h,24h]` | |
| webhooks.retry_jitter | 0.2 | |
| webhooks.failing_notify_after | 24h | |
| webhooks.disable_after | 72h | continuous failure, no success |
| webhooks.rate_limit | `{per_second: 25, burst: 50}` | per endpoint |
| webhooks.delivery_retention_days | 30 | self-host configurable |
| webhooks.rotation_max_overlap | 168h | |
| webhooks.api_versions | `["2026-11-01"]` | first is current; must equal foundation supported `Dimmi-Version` list (boot check) |
| webhooks.tls_skip_verify_hosts | [] | self-host only, ignored on cloud; doctor warns |
| http_client.allow_private_networks | false | framework key (was `safehttp.allow_private_networks`); self-host intranet endpoints |

## Permissions matrix

Role columns derived from 00-accounts `roles.go`; 10 code checks permissions only. Notification recipients: failing/disabled to holders of 06 `notification_alerts:receive`; security email to owners resolved through owner-only permission set (`roles.go`), never `role ==` in 10.

| Action | owner | admin | member | token |
|---|---|---|---|---|
| List/show endpoints, deliveries, attempts, event types | yes | yes | no | `webhooks:read` |
| Create/update/delete endpoint | yes | yes | no | `webhooks:write` |
| Reveal/rotate secret | yes | yes | no | `webhooks:write` |
| Test, redeliver | yes | yes | no | `webhooks:write` |
| Receive failing/disabled notifications | yes | yes | no | n/a |
| Receive new-endpoint security email | yes | no | no | n/a |

## Performance budgets

| Metric | Budget |
|---|---|
| FanOutWebhookEvent with 0 matching endpoints | under 2 ms (endpoint set cached per workspace in otter, invalidated on change via Valkey pubsub) |
| FanOutWebhookEvent with 5 endpoints | p95 under 30 ms (one snapshot build per distinct `api_version`; one at MVP) |
| Commit to first attempt | p95 under 2 s |
| Worker memory per in-flight attempt | under 100 KB (payload + capped response) |
| Delivery log list p95 | under 30 ms |
| Storage | ~3 KB per delivery compressed; 100k deliveries/day x 30 d about 9 GB worst case; retention configurable |

## Observability

- `dimmi_webhook_deliveries_total{status}` (final outcome), `dimmi_webhook_attempts_total{result, status_class, error_code}`
- `dimmi_webhook_attempt_duration_seconds` histogram
- `dimmi_webhook_fanout_lag_seconds` (event occurred_at to delivery insert), `dimmi_webhook_first_attempt_lag_seconds`
- `dimmi_webhook_endpoints{health}` gauge, `dimmi_webhook_endpoints_disabled_total{reason}`
- `dimmi_webhook_payload_thin_total`, `dimmi_webhook_ssrf_blocked_total`
- Logs: one line per attempt: workspace, endpoint id, delivery id, event type, status, duration, error code, URL host only (never path/query: may hold tokens). Never log secrets, signatures, payload.
- Endpoint `stats` in API from attempts table (24 h window, cached 60 s).

## Security and privacy

- SSRF: `a.HTTP` (code.dny.dev/ssrf dialer guard: DNS resolve, block private/loopback/link-local/CGNAT/metadata, recheck at connect, no redirects). Create-time validation plus per-attempt. Self-host intranet requires `http_client.allow_private_networks`.
- URL userinfo (`https://user:pass@`) rejected (credentials in URL leak to logs). Query strings allowed (common for consumer tokens). URL stored plaintext (needed for uniqueness and display), returned only to owner/admin, logs and metrics show host only.
- Secrets: 32 random bytes, `whsec_` base64 (Standard Webhooks range 24..64 bytes), envelope-encrypted, revealed only to owner/admin, reveal audited.
- Rotation overlap: both signatures sent (`v1,new v1,old`); consumer libraries accept any match.
- Replay: consumers reject `webhook-timestamp` outside 5 minutes (documented; libraries default). Timestamp regenerated each attempt.
- Notes (internal) excluded unless `include_notes`; changing `include_notes` to true audited + owner email (same as creation).
- Payload contains contact PII (email, name, attributes). Documented; https required on cloud. Erased contacts (GDPR): future events show tombstone; past delivery payloads purged: 03 erase job calls contact eraser `EraseWebhookContactData` (10 owns, idempotent) deleting delivery rows by `contact_id` (attempts cascade). Pending retries for those deliveries canceled.
- New endpoint creation emails owners ("Webhook endpoint added: api.example.com by Jane via API token tok_...") so silent exfiltration is visible.
- Response bodies stored are third-party data from user's own server; shown only to admins; retention 30 days.

## Metering impact

None. Webhook deliveries never billable (D1 requires contact message + substantive reply). API and webhooks unmetered (roadmap 9, pricing section 8). Spend cap does not block deliveries. `billing.*` events delivered like others.

## Dependencies

| Spec | Need |
|---|---|
| 00-foundation / framework | event catalog + `a.Events` dispatcher, `framework/httpclient` webhook profile (no redirects, response cap), `framework/webhooks` (standard-webhooks lib), `ratelimit.Limiter` (Valkey), `encryption.KeyManager`, `framework/queue` (River), `pubsub.PubSub` (Valkey: cache invalidation); shared over-limit code `entitlement_limit_reached`; add abilities `webhooks:read`; prefixes `whd_`, `wha_`; `api.version` / supported versions (shared with `Dimmi-Version`), `Entitlements` (key `webhooks.http_endpoints`), `RetentionGuard`, `AuditSink` contract |
| 00-accounts | roles + permission catalog (`webhooks:read`, `webhooks:write` permissions; `authz.Can`), member removal event, audit log |
| 03-shared-inbox | conversation/message/contact Resources (snapshot), contact erase hook to purge payloads |
| 05, 07, 08, 12 | Resources for channel_connection, ai_run, article, article_suggestion, billing ledger snapshots; their events flagged webhook=yes |
| 13, 14, 15-announcements-surveys | Resources + fixtures for changelog_entry, changelog_mailing, changelog_subscriber, feedback_post, feedback_vote, feedback_comment, feedback_request, feedback_post_update, survey_response; registry rows in `webhook_catalog.go` in their PRs |
| 07 (agent actions), 08 (docs sync), 16-importers | Resources + fixtures for ai_action_call, ai_action_connection, docs_sync_run, article_freshness, import_run; registry rows in `webhook_catalog.go`; 16 writers dispatch no per-record events |
| 04-email-channel | owns inbound provider event webhooks `/webhooks/{ses,postmark,resend,mailgun,sendgrid}/events`; no shared code |
| 06-notifications | `system_alert` delivery (email + in-app) for failing/disabled/created notices |
| 09-agent-native | CLI `dimmi trigger`, `dimmi webhooks endpoints\|deliveries\|redeliver` (defined in 09 cli.md); MCP tool registration + confirmation UX |
| 11-self-host | `dimmi doctor` check: outbound reachability, `http_client.allow_private_networks` warning; Valkey required |

## Contradictions with foundation (reported, not edited)

1. Ability list has `webhooks:write` only; read-only tools need `webhooks:read` (added).
2. Ownership map lists `webhook_endpoints`, `webhook_deliveries`; spec adds `webhook_delivery_attempts`.
3. Foundation webhook description says retries send "identical body": kept; but headers (timestamp, signature) change per attempt per Standard Webhooks. Not a conflict, clarified.
4. data-model.md note on messages: "never in customer webhooks payload with `visibility` public" is unclear (no `visibility` field exists). Spec rule: notes only to endpoints with `include_notes=true`.
5. api.md "Null when unset, key always present": `secret` key present as `null` outside create/rotate/reveal.

## Open questions (with recommended default)

1. Auto-disable threshold: 72 h continuous failure (Svix uses 5 days, Intercom 7 days). Default 72 h: retries span 24 h, two more days covers weekend deploy outages; manual redeliver restores.
2. Global `webhook_event_sequence` leaks relative cloud volume between deliveries. Default accept; switch to per-workspace sequence table if concern raised.
3. Static egress IPs for cloud: default no for MVP, document "verify signatures, not IPs".

## Later (extensibility notes, not MVP)

- `AuditSink` webhook driver (extensibility.md section 8): foundation `AuditSink` contract (noop at MVP) gets driver posting audit entries (JSONL batches) to customer SIEM URL signed with this spec's Standard Webhooks scheme (`webhook-id`, `webhook-timestamp`, `webhook-signature`, `whsec_` secret, retry schedule, `a.HTTP`). Reuses `attempt_webhook_delivery` plumbing; separate config table per org then, no change to `webhook_endpoints`.
- Marketplace apps: `webhook_endpoints.app_installation_id uuid null` (additive) for app-owned endpoints.
- Breaking payload change: second date in `webhooks.api_versions`, transformer in `build_webhook_payload` keyed by version; existing endpoints stay on pinned version.
