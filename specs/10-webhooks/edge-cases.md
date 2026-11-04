# 10 Webhooks: edge cases

Style: caveman. Each: case. Decided behavior. Scope trim 2026-11-02: listen-session cases (C6), bulk recover cases (C7) removed; renumbered. Final sweep 2026-11-02: 16 code renamed `entitlement_limit_reached`.

## Endpoint configuration

1. URL with userinfo `https://u:p@host`. 422 `invalid_webhook_url`.
2. URL with fragment `#x`. Fragment stripped on normalize (never sent by HTTP anyway); stored without.
3. URL host is IP literal in private range (`10.0.0.5`, `[::1]`, `169.254.169.254`). 422 unless self-host `http_client.allow_private_networks`; metadata IPs always 422.
4. URL host resolves to private IP at create (DNS). 422 same rule. Resolves public at create, private later (rebinding). Attempt blocked at connect, `ssrf_blocked`, counts as failure, retried (DNS may recover), log warns.
5. `http://` URL on cloud. 422 `insecure_webhook_url`. Self-host allowed, response warns `insecure: true`.
6. `localhost` URL on cloud. 422 `invalid_webhook_url` with hint "use a public tunnel".
7. Non-default port (`https://host:8443/`). Allowed. Ports 0, 22, 25 rejected (port allowlist: 80, 443, 1024..65535).
8. IDN host (`https://bücher.example/`). Normalized to punycode; stored punycode; displayed unicode.
9. Same URL twice in workspace. 409 `endpoint_url_taken` (avoids duplicate `webhook-id` deliveries looking like dupes).
10. Same URL in two workspaces. Allowed; different secrets.
11. `event_types` empty. 422 (at least one). Duplicate entries deduped. `*` plus explicit types: normalized to `["*"]`.
12. Event type with `webhook=no` (e.g. `typing.started`, `conversation.read`). 422 `unknown_event_type`.
13. Wildcard `conversation.assigned.*` or `*.created`. 422: only `*` and `<first segment>.*` supported.
14. `inbox_ids` contains deleted or other-workspace inbox. 422 (other workspace: treated as not found, same 422 message). Inbox deleted later: filter keeps stale id, matches nothing for it; if all ids stale, endpoint receives no conversation events (show warning in resource `warnings: ["inbox_filter_matches_nothing"]`).
15. `inbox_ids` on non-conversation events (contact.*, article.*, billing.*). Filter ignored for types without inbox; those still delivered.
16. 21st endpoint. 422 `entitlement_limit_reached` (`key: webhooks.http_endpoints`, `limit: 20`). Self-host raises via config.
17. PATCH url on failing endpoint. Pending retries continue to new URL (URL read at attempt time). Health reset to healthy, `failing_since` null.
18. PATCH `event_types` removes type while deliveries of that type retrying. Already-created deliveries continue (snapshot committed); new events filtered.
19. PATCH `include_notes` false while note deliveries retrying. Pending note deliveries canceled (privacy wins), status `canceled`.
20. Endpoint deleted while attempt in flight. Attempt completes, attempt insert fails FK, worker logs debug, exits.

## Fan-out

21. Event with zero matching endpoints. No snapshot built, no rows. Cached endpoint set makes this sub-millisecond.
22. Endpoint created 1 s after event committed but before fan-out ran. Endpoint receives event (matching happens at fan-out time). Documented: endpoint may receive events that occurred up to a few seconds before its creation.
23. Fan-out job retried after partial failure. Single tx inserts all deliveries + jobs; unique `(endpoint_id, event_id)` with `ON CONFLICT DO NOTHING`.
24. Subject deleted before fan-out (conversation created then hard-deleted within ms). Snapshot uses tombstone `{id, object, deleted: true}`; event still delivered (consumers learn it existed).
25. Subject resource from other spec missing Resource (spec not built). Catalog row has `webhook=yes` only once Resource registered; registry test fails CI if a webhook type lacks fixture + resource.
26. Snapshot over 256 KB. Thin payload, `truncated: true`, metric.
27. Contact `custom_attributes` contain unicode, emoji, RTL. JSON UTF-8, NFC normalized (foundation), no escaping beyond JSON spec; ` `/` ` escaped for JS consumers.
28. Message body includes HTML from email. Webhook carries canonical Markdown `body` only; `body_html` never included.
29. Burst: 10,000 events in a minute (import or spam wave). Fan-out jobs queue; per-endpoint rate limit 25/s spreads delivery; queue lag metric alerts. Imports (launch: importers, 16) dispatch no per-record events; listener also skips imported subjects (defense); one `import.completed` per run.
30. Event in `test_mode` workspace. Delivered with `test_mode: true`.
31. Workspace suspended. Fan-out skipped (no deliveries created); retrying deliveries snoozed until unsuspended or 24 h window ends then `failed` reason `workspace_suspended`.
32. Workspace soft-deleted. Fan-out skipped; pending deliveries canceled; purge deletes all.
33. Note `message.created` and endpoint `include_notes=false`. No delivery row at all.
34. `message.updated` on note (note edit). Same notes rule.
35. Contact merged: `contact.merged` delivered with surviving contact; earlier deliveries keep old contact id (consumers map via `merged_ids`).

## Delivery

36. Endpoint returns 200 after 14.9 s. Success. 15.1 s: `timeout`, retry; consumer probably processed: duplicate later, consumer dedupes by `webhook-id`.
37. Endpoint returns 301/302/307/308. Failure `redirect_not_followed`, Location header captured in attempt log, hint "update URL".
38. Endpoint returns 2xx with 10 MB body. Read stops at 64 KB, connection closed, success, stored first 4 KB.
39. Endpoint returns 500 with 10 MB body. Same cap, `http_error`, retry.
40. Endpoint returns 410. Endpoint disabled immediately (`gone`), all pending deliveries canceled, owners/admins notified. Test API returning 410: endpoint NOT disabled (test sends never change health).
41. Endpoint returns 429 with `Retry-After: 3600`. Next attempt at max(schedule, now + 3600 s), capped at 24 h window end.
42. `Retry-After` HTTP-date in past or garbage. Ignored, schedule used.
43. TLS cert expired or self-signed. `tls_failed`, retry. No insecure option on cloud. Self-host: `webhooks.tls_skip_verify_hosts` list (default empty), doctor warns.
44. Endpoint slow-loris (drips bytes). Total timeout 15 s covers headers + body read.
45. Endpoint responds before reading body (closes early). If status received: use status. Connection reset before status: `connect_refused` class `network`, retry.
46. HTTP/2 GOAWAY mid-request. Network error, retry.
47. Consumer verifies with re-serialized JSON and fails. Docs warn; test API response includes hint when consumer returns 401/403 on test: `"hint": "Verify the raw body bytes"`.
48. Clock skew on consumer over 5 minutes. Their library rejects. Delivery log shows 401; docs list skew as cause. Our timestamps from NTP-synced server (`dimmi doctor` clock skew check).
49. Two workers pick same delivery (manual redeliver + auto retry at same instant). Per-delivery advisory lock `pg_try_advisory_xact_lock(delivery)`: second waits via snooze 2 s; attempt numbers unique `(delivery_id, number)`.
50. Manual redeliver of succeeded delivery. Allowed (consumer asked). New attempt `trigger=manual`, same body, new timestamp. Delivery status reflects latest attempt: 2xx `succeeded`, else `failed`. Auto retry chain not restarted.
51. Manual redeliver while auto retry scheduled. Manual attempt runs now; if 2xx, delivery `succeeded`, scheduled auto job exits no-op. If fails, auto schedule continues unchanged.
52. Attempt log response body contains secrets of user's app. Stored as returned (their server sent it), visible to admins only, 30-day retention. Docs advise not echoing secrets.

## Health and auto-disable

53. Endpoint fails 23 h then succeeds once. `failing_since` reset, no notification.
54. Endpoint fails continuously 24 h. One `webhook_endpoint.failing` event + notification (`failing_notified_at` prevents repeats until healthy again).
55. Endpoint failing 72 h with zero traffic in last 48 h (no events). Disable rule requires at least 1 failed attempt in last 24 h; idle endpoints not disabled by time alone.
56. Owner removed, only admins left. Notifications go to remaining owners + admins. Zero active owners/admins impossible (foundation requires owner).
57. Notification email to owner bounces. 06 handles (`email_suppressed`); in-app still shown.
58. Endpoint re-enabled after auto-disable. Health healthy, counters reset; missed events not sent automatically; failed/canceled deliveries stay in log for manual redeliver.

## Secrets

59. Rotate twice quickly. 409 `rotation_in_progress` unless `force: true`, which drops oldest immediately (only current + new remain).
60. Rotate with `expire_previous_in_seconds: 0`. Old secret invalid immediately (leak response).
61. Reveal secret via token without `webhooks:write`. 403. Every reveal audited with actor, IP.
62. KeyManager unavailable (KMS outage). Signing impossible: attempt not sent, job snoozed 30 s (no attempt consumed), metric + log error. After 1 h still failing: attempts consumed as `signing_unavailable` failures.
63. App key rotated (`dimmi key:rotate`). Data keys re-wrapped; secrets unchanged; no consumer impact.

## Migrations, upgrades, self-host

64. New event type added in release. Appears in `/webhook-event-types`; wildcard endpoints start receiving; exact-list endpoints unaffected.
65. Resource gets new field. Additive in same `api_version`.
66. Breaking resource change. New `api_version`; payload builder keeps transformer for old version 6 months.
67. Self-host with egress proxy. `a.HTTP` honors `HTTPS_PROXY` env; SSRF checks apply to target host before proxying (CONNECT target validated).
68. Self-host webhooks disabled (`webhooks.enabled=false`). API still manages endpoints; response header `Dimmi-Warning: webhooks disabled by server config`; fan-out no-op.
69. Delivery table grows large on self-host. Retention job; `webhooks.delivery_retention_days` lower; doctor shows table size.
70. Restore from backup. Retrying deliveries resume per River state; may cause duplicate sends; `webhook-id` dedupe covers.

## Abuse and security

71. Attacker with stolen admin token creates endpoint to exfiltrate. Owners emailed immediately with host + actor; audit logged; endpoint visible in dashboard.
72. Prompt injection asks MCP agent to create webhook. MCP tool requires `confirm: true` and client human confirmation; owner email still sent.
73. Endpoint used to DoS third party (point URL at victim). Rate limit 25/s per endpoint, 20 endpoints per workspace, retries on failure spaced; victim returning 410 disables. Cloud abuse monitor: workspaces with all endpoints failing to same host across many workspaces flagged.
74. Consumer replays captured request later. Timestamp tolerance 5 min + `webhook-id` dedupe defeat replay.
75. Response headers include `Set-Cookie`. Stored redacted (`Set-Cookie`, `Authorization`, `Cookie` values replaced with `[redacted]`).

## Extensibility seams (`00-foundation/extensibility.md`)

76. Two endpoints on different `api_version` (later, after second version ships) subscribed to same event. Snapshot built once per version, each endpoint gets its version's shape; `sequence` shared (same object). MVP: one version, one build.
77. Create endpoint with `api_version: "2025-01-01"` (unknown). 422 `api_version_unsupported`. Omitted: current (`2026-11-01`, equals REST `Dimmi-Version`).
78. Staff raises cloud workspace endpoint cap via `entitlement_overrides` (`webhooks.http_endpoints` = 50). 21st create succeeds within 60 s, no deploy. Self-host: `webhooks.max_endpoints` config is the limit.
79. `RetentionGuard` (fake) holds contact during `PruneWebhookDeliveries`. Deliveries with that `contact_id` kept past retention; rest pruned; metric `retention_held`.
80. Member (role lacks `webhooks:write` permission) with PAT asking `webhooks:write`. PAT clamp drops ability at create (role permissions intersect abilities); request 403 `missing_ability`. Policy never names roles.
81. `feedback_comment.created` with `visibility = internal`, endpoint subscribed `feedback_comment.*` without `include_notes`. No delivery row. Same endpoint with `include_notes = true`: delivered. Public comments: always.
82. `changelog_entry.updated` with 200 KB body. `body_markdown` omitted (over 10 KB rule), `links.self` present; payload under cap, not thin fallback.
83. Endpoint URL points at own instance inbound path (`https://<app_url>/webhooks/postmark/events`). 422 `invalid_webhook_url` (loop guard; inbound provider webhooks are 04's).
84. Valkey restart mid-burst. Per-endpoint rate limit buckets reset (brief burst above 25/s possible, still bounded by queue concurrency); endpoint cache falls back to 2 s local TTL until pubsub reconnects.
85. `survey_response.completed` for contact later erased. Pending deliveries canceled and stored payloads purged by `EraseWebhookContactData` (same as other contact payloads).

## Launch extensions: agent actions (07), docs sync (08), importers (16)

86. 16 import writes 50,000 conversations, endpoint subscribed `*`. Zero `conversation.created` / `message.created` deliveries (no dispatch; listener skips imported subjects); one `import.completed` per run.
87. Delta import run every 10 min for 7 days. One `import.completed` per run (about 1,000 deliveries); consumers filter by `data.object.kind`; per-endpoint rate limit unaffected.
88. `ai.action_call_completed` for write call (refund). Payload has action name, status, outcome, latency; never request body, response body, or identity values (customer API data stays in 07 trace).
89. `ai.action_connection_failed` repeats (customer API down 6 h). 07 emits at most once per 24 h per connection (07 rule); 10 delivers each emitted event, no own dedupe.
90. `docs_sync_run.completed` for private repo. Payload carries repo full name, PR number, counts, review URL; never diff text or file contents.
91. Contact erased while `ai.action_call_approval_requested` delivery pending. `EraseWebhookContactData` cancels pending deliveries and purges stored payloads (same rule as E85).