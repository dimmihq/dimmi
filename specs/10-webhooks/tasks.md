# 10 Webhooks: tasks

Style: caveman. Ordered. Each about 1 day, merged with tests. Requires foundation/framework (`a.Events`, River, `framework/httpclient`, `framework/webhooks`, KeyManager, `ratelimit` + `pubsub` on Valkey) and Resources from 03 for conversation/message/contact. Final sweep 2026-11-02: app logic integration-tested only (testing.md), tests named in plain words, every edge case covered; catalog rows for 13, 14, 15-announcements-surveys. Scope trim 2026-11-02: M6 local dev (`dimmi listen`, C6) and bulk recover (C7) removed; edge case refs renumbered.

## M1: Signing and schema

- [ ] Wire `framework/webhooks` (standard-webhooks Go library `Sign`/`Verify`; multi-signature header and rotation loop thin wrapper there): secret generate/parse (`whsec_`, 32 bytes, std base64). No `app/support/webhooksig`.
  - Accept: framework unit vectors from Standard Webhooks repo pass (framework package); integration test asserts protocol.md example signature (`ljGnIJ...`) on a real test delivery.
- [ ] Migrations 20261101001001..004, `webhook_event_sequence`, sqlc queries, factories, prefixes `whd_` `wha_`, ability `webhooks:read`.
  - Accept: migrate up/down; tenant lint; lz4 compression on `payload` column.
- [ ] `app/events/webhook_catalog.go` registry + event type matcher (`*`, `prefix.*`, exact) + `GET /webhook-event-types`.
  - Accept: matcher integration tests through endpoint create + fan-out (edge cases 11-13); CI test fails when catalog `webhook=yes` type lacks fixture or resource.
- [ ] Catalog rows + fixtures for 18 (`audience.*`, `attribute_definition.*`, `contact_event.tracked`, built by 18 M3, M6), 13 (`changelog_entry.*`, `changelog_mailing.completed`, `changelog_subscriber.subscribed|unsubscribed`), 14 (`feedback_post.*`, `feedback_vote.*`, `feedback_comment.*` with `notes_opt_in`, `feedback_request.*`, `feedback_post_update.sent`), 15-announcements-surveys (`survey_response.completed`), 07 (`ai.action_call_completed`, `ai.action_call_approval_requested`, `ai.action_connection_failed`), 08 (`docs_sync_run.completed|failed`, `article.freshness_changed`), 16 (`import.completed`); imported-subject skip in `QueueWebhookDeliveries` (E29, E86 to E91); fixtures in `resources/fixtures/webhooks/<type>.json` via `dimmi webhooks:fixtures`.
  - Accept: catalog test green; internal `feedback_comment.*` skipped for endpoints without `include_notes` (E81); `GET /webhook-event-types` lists `notes_opt_in: true` for them.

## M2: Endpoint management

- [ ] Endpoint CRUD API + requests + resource + policy (`authz.Can` with `webhooks:read` / `webhooks:write` permissions, no role names) + URL validation (scheme, userinfo, ports, IDN, DNS private check, https rule) + limit via `Entitlements.Limit(ws, "webhooks.http_endpoints")` (catalog entry; self-host from `webhooks.max_endpoints`) + `api_version` (`text not null default '2026-11-01'`, validated against supported list shared with `Dimmi-Version`) + audit log.
  - Accept: feature tests: happy path, 422 per URL rule, 409 duplicate URL, 422 `entitlement_limit_reached` at limit, member 403, other workspace 404, token abilities read/write, idempotency key on POST; edge cases 77, 78, 80.
- [ ] Secret reveal + rotation (overlap, force, expire job).
  - Accept: during overlap header carries 2 signatures, both verify with official Go lib; after expiry only 1; reveal audited.
- [ ] New-endpoint and `include_notes` enable security email via 06 `system_alert` (fallback direct mail if 06 absent).
  - Accept: `tc.Mail` (`mail.AssertSent`) asserts owners receive host + actor.
- [ ] Endpoint cache per workspace (otter) + invalidation listener via Valkey pubsub (`a.PubSub`).
  - Accept: create endpoint on replica A visible to fan-out on replica B within 1 s (two-app test harness).

## M3: Fan-out and payload

- [ ] `QueueWebhookDeliveries` listener + `FanOutWebhookEvent` job: match endpoints, notes opt-in, inbox filter, test_mode flag, suspended skip, single tx insert, unique conflict skip.
  - Accept: `webhook_fan_out_test.go` covers edge cases 21-24, 30-34; zero endpoints path allocates no snapshot (benchmark).
- [ ] `BuildWebhookPayload`: envelope, actor, `changes`, per-type extras, resource snapshot via Resources built for endpoint `api_version` (fan-out builds once per distinct version; transformer hook keyed by version, none at MVP), tombstones, `sequence` under per-object advisory lock, 256 KB thin fallback, ` ` escaping, exact bytes stored.
  - Accept: golden tests per event type against `resources/fixtures/webhooks/*.json`; sequence monotonic per object under concurrent fan-out test (100 goroutines); fake second version with test transformer: two endpoints on two versions get two shapes, one build each (edge case 76).
- [ ] Fixtures generator `dimmi webhooks:fixtures` (dev command) writes fixtures from factories; docs consume.
  - Accept: CI diff check fixtures up to date.

## M4: Delivery

- [ ] `framework/httpclient` webhook profile (`a.HTTP`): no redirects, 5 s connect, 15 s total, 64 KB read cap, header/body capture, remote IP capture, proxy support.
  - Accept: tests for redirect, slow-loris, huge body, private IP, rebinding (fake resolver), TLS failure.
- [ ] `DeliverWebhook` worker: sign per attempt, headers, classification table, attempt row, delivery status, 410 disable, Retry-After, custom `NextRetry` schedule with jitter, per-endpoint rate limit snooze, per-delivery lock, KeyManager outage snooze.
  - Accept: `webhook_delivery_test.go` per response class; `webhook_retry_schedule_test.go` with frozen clock asserts 12 attempt times within jitter bounds and ending at 24 h.
- [ ] Health evaluation: `CheckWebhookEndpointHealth` (24 h failing notice, 72 h disable with activity rule), `webhook_endpoint.failing/disabled` events, cancel pending on disable.
  - Accept: `webhook_auto_disable_test.go` edge cases 53-58.

## M5: Log, redeliver, test

- [ ] Deliveries index/show (+payload include, attempts include), attempts index, header redaction (`Set-Cookie`, `Authorization`, `Cookie`), endpoint `stats`.
  - Accept: cursor pagination, filters, other workspace 404, p95 under 30 ms on 1M deliveries seed (benchmark).
- [ ] Redeliver API (`?wait=true`), test throttle.
  - Accept: edge cases 49-51.
- [ ] Test API `POST /webhook-endpoints/{id}/tests` (`WebhookEndpointTestController.Store`, route `webhook-endpoints.tests.store`; default `webhook.test`, typed fixtures, sync result, never changes health, 401/403 verify hint).
  - Accept: tests with fake endpoint returning 200, 401, 410 (no disable).
- [ ] Prune job + retention config + `RetentionGuard` calls (per workspace batch, per `contact_id`); GDPR contact eraser `EraseWebhookContactData` (registered on 03 eraser registry; runs only after 03 erase passed guard).
  - Accept: frozen clock prune; fake guard false keeps held rows (edge case 79); erase contact removes deliveries by `contact_id`, cancels pending retries, other contacts untouched, second run no-op; 03 registry test lists it.

## M6: MCP, metrics, docs

- [ ] MCP tools (6) with `confirm` gates and annotations.
  - Accept: `webhook_mcp_tools_test.go`: create without `confirm` returns tool error explaining confirmation; secret never in output.
- [ ] Metrics + logs (host-only URLs) + `dimmi doctor` checks (outbound HTTPS reachability check (URL from 11 doctor config), private network warning).
  - Accept: metrics registered; log redaction test.
- [ ] Compatibility CI job: Node `standardwebhooks`, Python `standardwebhooks`, Go lib, Node `svix` verify real deliveries from test server.
  - Accept: job green; runs on PR touching `framework/webhooks` or payload builder.
- [ ] Docs page from protocol.md + event catalog with fixtures; recipes (Express, Next.js route handler, FastAPI, Go net/http, Laravel).

## Done when

- `go test ./...` green; compatibility job green.
- Demo script:
  1. `curl -X POST /api/v1/webhook-endpoints` with `event_types: ["conversation.*","message.created"]` pointing at local test receiver (Node script using `standardwebhooks`). Secret returned.
  2. Send widget message. Receiver logs verified `conversation.created` and `message.created` under 2 s.
  3. Stop receiver. Send message. Delivery log shows attempts 1-3 failing with `connect_refused`, `next_attempt_at` set. Start receiver: next scheduled attempt succeeds.
  4. Receiver returns 500 for 1 event; after fix, `POST /webhook-deliveries/{id}/redeliveries` resends it; receiver dedupes duplicate by `webhook-id`.
  5. Rotate secret with 1 h overlap; receiver still on old secret verifies; switch to new secret; both work until expiry.
  6. `POST /webhook-endpoints/{id}/tests {type: "ai.handoff_requested"}` returns sync 200 result with fixture.
  7. Point endpoint at server returning 410: endpoint disabled, owner email + in-app alert.
  8. Create endpoint `http://169.254.169.254/` rejected 422; endpoint whose DNS later points to 127.0.0.1 shows `ssrf_blocked` attempt.
