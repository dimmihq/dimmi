# 01 Messenger widget: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Depends on foundation tasks done (router, huma, auth:widget slot, River, `framework/broadcasting` (embedded centrifuge, Redis broker + presence on Valkey, uni-SSE handler), Valkey, Filesystem, rate limiter). 02 tasks M1 to M3 interleave where marked.

Tests: business logic = integration only (real App, Postgres, Valkey, centrifuge node; testing.md). No unit tests with mocks for app code. Every edge case in edge-cases.md maps to a test named with its id (`t.Run("E58 SSE buffered falls back to polling", ...)`).

## M1 Schema and settings API

- [ ] Migrations `20261101000101` to `20261101000104`, `20261101000106` to `20261101000108` (widget_settings, widget_sessions, resolver functions, visitor identity index, `attachments.uploaded_at`, `changelog_tab_enabled` (13), `intercom_shim` (16); no `105`: test flag is 03 `conversations.is_test`). sqlc queries `widget_settings.sql`, `widget_sessions.sql`. Factories.
  - Accept: `migrate` up/down clean; RLS policies on both tables; resolver functions return only `(workspace_id, id)`; lint check passes (`workspace_id` predicate everywhere except resolvers, allowlisted).
  - Tests: resolver from app role works under RLS; cross-tenant select returns nothing.
- [ ] Wire `framework/http/origin` (normalize, punycode, wildcard leftmost label, `null` origin) into settings validation and `widget.origin`.
  - Tests (integration, `PATCH /widget-settings` + config GET): ports, trailing slash rejected, path rejected, IDN, uppercase, `*.` apex non-match (E8, E9). Pure table tests live in framework package.
- [ ] Actions `CreateWidget`, `UpdateWidgetSettings`, `DeleteWidget` + events `widget.created`, `widget.settings_updated`, `widget.deleted` + audit log entries for origin/anonymous changes.
  - Accept: create makes channel_connection type `widget` in same tx; update bumps `config_version`; limit via `Entitlements.Limit(ws, "widget.max_per_workspace")` (fake driver in tests: 10, -1, override 20); `widget` channel type + `widget_visitor` identity type registered in Go registry (no CHECK).
- [ ] `EnsureDefaultWidget` registered on workspace initializer hook (contracts.md, after 03 `EnsureDefaultInbox`, inside `CreateWorkspace` tx).
  - Tests: new workspace has one widget on default inbox; re-run (`dimmi workspace:repair`) idempotent; initializer failure rolls back workspace creation.
  - Tests: integration per action through endpoints; tx rollback leaves nothing.
- [ ] Register `widgets:read`, `widgets:manage` in 00-accounts permission catalog + roles.go rows; entitlement key `widget.max_per_workspace` in catalog (self-host maps `widget.max_widgets_per_workspace`).
  - Tests: role matrix table test; member token with `settings:write` denied `widgets:manage`; no role names in 01 policy (lint).
- [ ] `WidgetSettingsController` (index/store/show/update/destroy), requests, `WidgetSettingsResource`, `WidgetSettingsPolicy` (`authz.Can` only).
  - Tests: feature per endpoint: happy, 422 messages, member forbidden on write, token ability, other workspace 404, idempotency on POST.
- [ ] `ResolveBranding` + `branding_locked` rule (12 `BillingStatus` contract; fake in tests; self-host default hidden).
  - Tests: matrix of 5 rows from README.

## M2 Assets

- [ ] `app/support/widgetassets`: embed `public/widget` + config on `framework/view/vite` (manifest, SRI) and `framework/view/static` (precompressed `.br`/`.gz` pick); keep previous release manifest files.
  - Tests (feature, asset routes): correct encoding by `Accept-Encoding`; missing variant falls back to identity; unknown file 404; previous release chunk served (E101).
- [ ] `AssetController` routes `widget.loader`, `widget.assets`, `widget.manifest` with cache headers, CORP, Timing-Allow-Origin, ETag/304, `assets_base_url` rewrite in loader.
  - Tests: headers snapshot; 304 on matching ETag; SRI values match file bytes.
- [ ] Placeholder loader + core stub (until frontend lands) and CI size budget job (`size-limit`: loader 3 KB gz, core 35 KB gz incl. EventSource realtime client and engage scheduler 1.5 KB; named chunks `help`, `changelog` 8 KB, `feedback` 15 KB, `engage-banner` 6 KB, `engage-modal` 10 KB, `engage-survey` 14 KB, others 20 KB gz).
  - Accept: CI fails when budget exceeded; budgets published in `/widget/v1/manifest.json`.

## M3 Config, boot handshake, sessions

- [ ] Middleware `widget.app`, `widget.origin`, `cors:widget` (preflight Max-Age 7200, Vary Origin, no credentials).
  - Tests: preflight allowed/blocked; 403 `origin_not_allowed` body; empty allowlist allows any.
- [ ] `ShowWidgetConfigController` + otter cache keyed by `wgt:config_version` (+ `engage_settings.version`) + Valkey PubSub bust listener `BustWidgetConfigCache` + `BumpWidgetConfigVersion` action. Config blocks from 13 (`features.changelog`), 14 (`features.feedback`, `texts.feedback_tab_label`), 15 (`engage`); fakes until owners land.
  - Tests: ETag 304; update on replica A invalidates B (two app instances, one Valkey); each block absent when its tab/flag off; 13 publish bumps version (E126).
- [ ] `ComputeTeamAvailability` + availability endpoint.
  - Tests: online only with realtime connection in last 60 s (centrifuge presence); avatars max 3; no emails in payload.
- [ ] `auth:widget` middleware: hash lookup via resolver, expiry, revoke, workspace into context (`SET LOCAL` per tx by `framework/database`), sliding anonymous expiry (write throttled).
  - Tests: expired 401 `session_expired`, revoked 401 `session_revoked`, unknown 401 `session_invalid`.
- [ ] `CreateAnonymousSession` + `StartWidgetSession` anonymous branch + `POST /sessions`: lead contact, `widget_visitor` identity, token issue, honeypot, `sdk.name` metric buckets.
  - Tests: page load (config only) creates no contact (E14); honeypot sets bot score silently (E82); unknown `sdk.name` bucketed `other`.
- [ ] `ResumeWidgetSession` + `GET /session` (anonymous resume, sliding expiry) + `DELETE /session`; handshake rows without JWT.
  - Tests: each handshake row without JWT; identified token on `GET /session` 409 `identity_proof_required`, session untouched; 401 paths; logout idempotent.
- [ ] Install tracker `support/widgetinstall` fed by `widget.origin` middleware (allowed + blocked), 60 s flush (`UpdateWidgetLastBoot`, origins cache), flush on shutdown.
  - Tests: config GET alone marks widget installed after flush (E113); 1,000 requests = 1 write per flush; blocked origin listed (Valkey cache); config p95 unchanged (bench); two replicas keep max `last_boot_at`.
- [ ] (with 02 M3) Wire `identity.VerifyIdentityToken` (`Surface: widget`) + `identity.IdentifyVisitor` into `StartWidgetSession` (`POST /sessions {user_jwt, previous_token}`, `user_jwt` = identity source token) and `PATCH /session`; `test_token` branch lives in M9.
  - Tests: all JWT rows of handshake table through real 02 actions (02 M3 lands with this task); `previous_token` anonymous merges, identified other `sub` revoked `identity_switch`, same `sub` 200 reuse; invalid/expired/foreign `previous_token` ignored; failed POST leaves previous session untouched; PATCH other `sub` 409 `identity_mismatch`; PATCH revives expired identified, not revoked.
- [ ] Contact endpoints + `UpdateWidgetContact` (lead only, email identity rules, rate limits).
  - Tests: user contact 403 `identity_locked`; duplicate email identity not stolen; NFC + bidi strip; written `custom_attributes` key removed from `contacts.trusted_attribute_keys` (E143).

## M4 Conversations and messages

- [ ] `StartWidgetConversation` + `SendWidgetMessage` wrapping 03 `ReceiveInboundMessage`; dedupe key `widget:{session}:{client_id}`; source jsonb; spam on bot score; `IsTest` -> `conversations.is_test`; server stamp `metadata.widget {session_id, identity_verified}`.
  - Tests: duplicate client_id returns same (E28); closed within window reopens (E30); past window redirects (E31); deleted 404 (E32); email_required rule (online vs offline, E41); blocked contact spam (E25); `announcement_id` valid lands in source + `announcement.replied` event row, invalid 422 `announcement_not_replyable` (E130); `identity_verified` true only for live verified identified session (E144, E145); client-sent metadata ignored (E146).
- [ ] Conversation and message list/show controllers + widget resources (no notes, deleted placeholders, signed attachment URLs).
  - Tests: note never returned (E45); other contact 404 (E34); email-origin conversation of same contact listed; Slack-origin not listed; imported (16) listed read-only for identified only, post creates linked conversation (E151); system message exposes `survey_request` (E131); agent message exposes `ai.confirmation` only, never `ai.action_calls`.
- [ ] Read state `PATCH /conversations/{conversation}/read-state` + `MarkWidgetConversationRead` + `message.delivery_updated`; unread count query. "Seen": broadcast 03 `conversation.seen_by_team` on widget channel (never `conversation.read`).
  - Tests: pointer never moves back; one event per batch; widget socket never receives `conversation.read`; `seen_by_team` payload has no member id; name present only with `seen_receipt_shows_name`.
- [ ] `POST /conversations/{conversation}/typing-indicators` + `BroadcastVisitorTyping` (throttle, 202, ephemeral publish on `conversation:{conv}` without history); register 07 `POST /conversations/{conversation}/handoffs` route in widget group (controller in 07).
  - Tests: 3 calls in 1 s produce one broadcast (E66); typing absent from recovery replay (E67); handoff idempotent (E99).
- [ ] Pings: `RecordWidgetPing` (current_page, recent_pages dedupe, context merge, secret query scrub).
  - Tests: `?token=abc&page=2` stored as `?page=2`; 4 KB context limit 422.

## M5 Attachments

- [ ] `CreatePendingUpload` (presign PUT with signed Content-Type and Content-Length on S3 driver; direct URL on local) + `POST /attachments`.
  - Tests: size over limit 413; blocked extension 422; presign on `local` disk and `s3` against SeaweedFS (no mocks).
- [ ] Direct upload `PUT /attachments/{id}/content` streaming with cap + sha256.
  - Tests: body over declared size aborted; wrong session 404.
- [ ] `CompleteUpload` + `PATCH /attachments/{attachment} {status: "uploaded"}` (stat, size match, sniff, image dims, megapixel cap, queue scan).
  - Tests: mismatch deletes object (E49); exe renamed png blocked (E50); PATCH twice idempotent (E52); other status value 422 `invalid_transition`; upload later linked to 14 feedback post survives orphan purge (E129).

## M6 Realtime (uni-SSE) and polling

- [ ] `IssueWidgetRealtimeToken` + `POST /widget/v1/realtime-tokens` (connection JWT 60 s, `sub` = `wss_`, aud realtime connect only, server-side sub `widget_contact:{ctc}`) + `widget_contact:{contact}` authorizer in `routes/channels.go` + `OnConnecting` hook for widget tokens on `/connection/uni_sse` (session revoke re-check, connection cap 10, oldest disconnected 4004) + `cf_connect` log redaction.
  - Tests (feature, real centrifuge node + Valkey, EventSource-style HTTP client): own channel streams events; token used as REST bearer 401; expired token rejected (E68); revoked session after issue rejected (E70); forged `subs` for other contact ignored (E71); 11th stream disconnects first (E59); access log has no `cf_connect` value (E72).
- [ ] `BroadcastToWidget` listener + `app/integrations/widget/payloads.go` (widget-safe, 10 KB fallback) + `seen_by_team` relay. Stored events with history (300 / 10 min), ephemeral without.
  - Tests: snapshot of every widget event payload; note event never published to widget channel (E45); reconnect with last offset gets `recovered: true` and missed events (E64); offset older than history gets `recovered: false` (E60, E73).
- [ ] Teammate typing reaches widget via 03 `RecordTyping` relay (no client events); agent typing passthrough (07).
  - Tests: payload has no member email or id.
- [ ] Session revoke disconnects streams (`widget.session_revoked` then 4009) for logout, merge, erase, widget delete, secret revoke (02); same flow calls 14 `RevokePortalSessions` (fake until 14 lands).
  - Tests: stream closed within 5 s in each case (E20, E22, E26); portal sessions revoked for logout, identity_switch, merge, erase (E128).
- [ ] `RepointWidgetSessions` listener + contact eraser `EraseWidgetContactData` (registered on 03 eraser registry).
  - Tests: lead into lead repoints (E23); lead into user revokes (E24); erase revokes sessions (`contact_erased`), scrubs `current_page`/`recent_pages`, second run no-op, other contact untouched (E22).
- [ ] `GET /updates` polling from DB with signed cursor.
  - Tests: gap after 3 h sleep filled (E60); 100 cap with `has_more`; note excluded.
- [ ] e2e protocol test (`tests/e2e/widget_protocol_test.go` + Node script using plain `EventSource` polyfill on `/connection/uni_sse`): every handshake row (config only, resume, identify, switch, sign-out, test), realtime token, receive teammate reply under 300 ms, kill stream and reconnect with offset (recovered), proxy that buffers SSE then polling (E58).

## M7 Offline email

- [ ] Listener `ScheduleWidgetOfflineEmail` + job `SendWidgetOfflineReplyEmail` (unique per conversation, delay, presence check on `widget_contact:{ctc}`) + `WidgetOfflineReplyMail` templ (render only). Delivery only via 04 `email.SendWidgetOfflineReply` (fake action until 04 lands, then real); `skipped{reason}` = done.
  - Tests: read before delay skips (E75); two replies one email (E74); unverified footer present (E76); unsubscribed skip (E79); suppressed address returns skipped, job completes (E77); captured MIME via `tc.Mail` once 04 wired; system-authored message never schedules mail (E131).

## M8 Anti-abuse

- [ ] `app/support/pow` on altcha-org/altcha-lib-go (HMAC key from app key, `expires` + `ip` salt params, adaptive `maxnumber`), single use via Valkey `SET NX EX` + 428 flow on `/sessions`. Widget ALTCHA solver (~30 lines, Web Worker) in core stub.
  - Tests (feature, `/sessions`): valid solve accepted once; replay rejected (E134); other IP prefix rejected (E135); expired rejected; `maxnumber` raised on anomaly (E81); Valkey down fails closed for PoW creates only (E136).
- [ ] `ScoreWidgetBot` signals (honeypot, min_send_ms, link count, UA headless markers, rate hits) + spam marking at conversation create.
  - Tests (feature, via `/sessions` + `/conversations`): each signal adds expected score (E83, E84); threshold makes `spam = true`; metric incremented.
- [ ] Rate limit buckets from config wired on widget routes.
  - Tests: 429 headers on Valkey limiter (E39, E40, E80); self-host `0` disables.

## M9 Test mode, install, preview

- [ ] `MintWidgetTestSession` + endpoint + `POST /widget/v1/sessions {test_token}` branch + per-member test lead + monthly cap (12 counter contract).
  - Tests: one-time use (E94); cap 429 (E95); conversation `is_test = true` (E93); forged token 401 (E96).
- [ ] Verified member-email test detection (with 02): JWT `email_verified=true` email equal active member email sets `test`.
  - Tests: unverified email does not set test; deactivated member does not.
- [ ] `BuildInstallSnippets` + install endpoint (status, origins seen/blocked); `framework` enum = 09 `dimmi init` ids (`nextjs-app`, `nextjs-pages`, `react`, `react-router`, `sveltekit`, `nuxt`, `vue`, `astro`, `rails`, `laravel`, `django`, `fastapi`, `go`, `express`, `hono`, `html`, `proxy`); templates `resources/widget/install/<id>.tmpl` = sdk-contract text.
  - Tests: blocks snapshot per framework id; unknown id 422; app id only in env lines; self-host base URL with subpath (E104).
- [ ] `/widget-preview/{widget}` SSR host page (member session); forwards `#preview=ann_...`/`svy_...` to SDK (15).
  - Tests: non-member 404; page includes loader and token only in fragment; draft announcement preview only for test session (E132).

## M10 Help tab and MCP

- [ ] Help tab: verify 08 `/widget/v1/help/*` routes sit in widget group (origin check, app-only auth, cache headers); widget config `help_tab` flag.
  - Tests: anonymous visitor (no session) reads help; agent-only article never returned; unlinked 404 `help_center_not_linked`.
- [ ] MCP tools `dimmi_list_widgets`, `dimmi_get_widget_install`, `dimmi_update_widget_settings`.
  - Tests: schema from huma types; ability enforcement; diff output.

## M10b Feature tabs (13 to 15 seams)

- [ ] Settings fields `changelog_tab_enabled` (needs `help_center_id`), `feedback_tab_enabled` (needs 14 portal settings row); session `changelog`, `feedback_unread_count`; CORS header `Dimmi-Feedback-Session`; route group mounts 13/14/15 controllers.
  - Tests: 422 rules (E125, E127); session fields only when tab on; preflight allows `Dimmi-Feedback-Session`; routes registered with widget middleware (route list test).

## M10d Agent action card and Intercom shim (07, 16 seams)

- [ ] Register 07 routes `widget.conversations.action-calls.show/update` in widget group (07 controller); widget channel carries `ai.action_call_updated` (widget-safe `{id, conversation_id, title, status}`); config strings `action_confirm`, `action_cancel`; SDK card per README "Agent action card".
  - Tests: route list test; double confirm one execution (E137); expired identified session 409 `identity_changed` (E138); anonymous PATCH 409 (E139); expiry updates card metadata (E140); typed "yes" path (E141); polling mode refetch (E142); payload never carries input/response.
- [ ] `intercom_shim` setting + config `features.intercom_shim`; SDK shim per sdk-contract "Intercom shim" (mapping 16 sources.md); `filter[intercom_id]` article resolution requirement on 08 help route.
  - Tests (Playwright, `sdk/examples/intercom-shim`): Intercom snippet calls (`boot`, `update`, `show`, `showNewMessage`, `showArticle`, `onUnreadCountChange`, `shutdown` = logout) drive Dimmi; `user_hash` ignored + warn (E148); conflict warn (E147); unknown article opens Help home (E149); option overrides setting (E150).

## M10c SDK packages and examples (frontend team, `sdk/` workspace)

- [ ] Package builds per sdk-contract Packages table: `@dimmi/browser` (loader), `@dimmi/react`, `@dimmi/nextjs` (server component, `'use server'` token action, `withDimmi`, adapters `/clerk` `/authjs` `/supabase` `/better-auth`, `/portal-sso`), `@dimmi/server` (+ `/express` `/hono`). Other stacks: install templates only (script tag + 02 raw snippet; packages later: identity-helper-packages). ESM + CJS, `sideEffects: false`, `.d.ts`, exports conditions; size deltas in CI.
  - Accept: each package sends `sdk.name` (`dimmi-browser`, `dimmi-react`, `dimmi-nextjs`); retired names (`@dimmi/messenger`, `DimmiMessenger`, `user_jwt` option) absent (CI grep).
- [ ] `sdk/examples/<id>` per install id (Next: Clerk, Auth.js, Supabase, Better Auth variants) + Playwright suite per example: anonymous boot, sign in (soft navigation), account switch, sign out (widget anonymous within one navigation), static page stays static (`next build` check), StrictMode single stream (E123), server action failure fails closed (E121), `track` triggers survey, `show` every view.
  - Accept: runs in `sdk-ci` job (separate from `go test`), against compose stack.

## M11 Hardening

- [ ] Load test: 10k uni-SSE streams + 200 msgs/s; RSS and p95 budgets recorded in CI benchmark.
- [ ] Prune job `PruneWidgetSessions` (`RetentionGuard` per workspace batch + per contact) + metrics + log redaction test (no `dmi_ws_` in logs).
  - Tests: fake guard false keeps rows, logs `retention_held` (E118); true deletes.
- [ ] Cross-tenant suite: every widget endpoint with other workspace's ids returns 404.
- [ ] OpenAPI `/widget/v1/openapi.json` complete; oasdiff in CI.

## Done when

- `go test ./...` green; e2e protocol test green; size budget job green.
- Demo script:
  1. `docker compose up`; create workspace; `GET /api/v1/widget-settings/{wgt}/install` returns snippet.
  2. Paste snippet in `examples/plain.html`, open: launcher renders; DB has no new contact.
  3. Send "hello": inbox shows conversation under 300 ms with page URL and browser.
  4. Teammate replies: visitor sees reply live; close tab, wait 2 minutes: email arrives; reply by email lands in same conversation.
  5. Set allowed origins to other domain: console shows `origin_not_allowed`, launcher gone.
  6. Next.js example signed in (identity source = server action): previous user history shown; sign out in host app: widget anonymous within one navigation; `Dimmi.logout()`: widget empty, storage keys gone, server session revoked.
  7. Block `/connection/uni_sse` in devtools: replies arrive via polling within 6 s; unblock: stream reconnects with offset, no duplicates.
  8. Dashboard preview with test token: conversation marked Test, not in usage ledger.
  9. Script 500 session creates from one IP: PoW required then 429; no billable conversations.
