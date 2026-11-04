# 01 Messenger widget: tasks

Style: caveman. **Build order binding, owner decision: milestones run in this order.** M1 backend API and default widget, M2 visitor sessions and messaging with SSE, M3 connect static widget, M4 connect settings page with live preview, M5 install snippets and installed status, M6 anti-abuse and offline email fallback, M7 signed-in identity (spec 02). Then settings expansions, translations and the rest as listed. Each task about 1 day, merged with tests.

Tracks: every milestone tagged. Track 01 widget-platform is wave 1. M7 and [02 tasks](../02-widget-identity/tasks.md) are track 09 widget-identity, wave 2, after track 01 M1 to M6. M8 onward stay track 01 and need nothing from M7 (identified paths in them use session factories). [18 tasks](../20-native-sdks/tasks.md) are track 10 native-sdks. New migrations take `20261201 01 NNNN`; older `2026110100 01xx` numbers listed here get renumbered into same range by builders.

Depends on what is built already: router, huma, `framework/broadcasting` uni-SSE, Valkey, Filesystem, rate limiter, 03 conversations actions, asset routes (`AssetController`, `widgetassets`), permissions `widgets:read`, `widgets:manage` in `app/auth/permissions.go`, `widget_contact` channel and authorizer.

Tests: business logic = integration only (real App, Postgres, Valkey, centrifuge node; testing.md). No mocks for app code. Every edge case in edge-cases.md has a test named in plain words (`t.Run("falls back to polling when SSE is buffered", ...)`). Every new page or flow: Playwright in `tests/e2e`.

## Freeze: what track 10 waits for

Track 10 starts only after M3 merged and this contract frozen. Freeze = `openapi.json` snapshot committed at `tests/contract/widget_v1.openapi.json`, oasdiff breaking check required in CI, and `tests/contract/widget_v1_fixtures/` golden responses.

| Frozen | Contents |
|---|---|
| Boot handshake | `POST /sessions`, `GET`/`PATCH`/`DELETE /session`, decision table, `widget_session` resource, all listed status codes |
| App config | keys clients read: `company`, `appearance` colours and background, `texts`, `locale`, `dir`, `locales`, `tabs`, `open_to`, `features`, `realtime`, `branding`. Additive keys free |
| Conversations + messages | list, create, show, messages list and send, `client_id` dedupe, `meta.reopened`, `meta.redirected_from`, `widget_message`, `customer_status`, read-state, typing-indicators, handoffs route |
| Attachments | three-step upload, limits, codes |
| Realtime | `POST /realtime-tokens`, `cf_connect` shape, connect reply `recovered`, `offset`/`epoch`, publication envelope incl. `widget_id`, event name list, disconnect codes 4004, 4009 |
| Polling | `GET /updates` cursor and event shape |
| No-Origin rule | `native_apps_enabled` table |
| Errors | error object shape, every code in README table; PoW `428` flow and ALTCHA payload |

After freeze: additive only. Removing or changing frozen item needs a `Dimmi-Version` entry (foundation versioning). `/widget/v1/devices*`, push payload, session transfers are added by later milestones and frozen by their own PR.

## M1 Backend API and default widget (track 01 widget-platform)

- [ ] Migrations `20261101000101` to `20261101000104`, `20261101000106` to `20261101000108` (widget_settings, widget_sessions, resolver functions, visitor identity index, `attachments.uploaded_at`, `changelog_tab_enabled` (13), `intercom_shim` (16); no `105`). sqlc queries `widget_settings.sql`, `widget_sessions.sql`. Factories.
  - Accept: `migrate` up/down clean; RLS policies on both tables; resolver functions return only `(workspace_id, id)`; lint check passes.
  - Tests: resolver from app role works under RLS; cross-tenant select returns nothing.
- [ ] Wire `framework/http/origin` (normalize, punycode, wildcard leftmost label, `null` origin) into settings validation and `widget.origin`.
  - Tests (integration, `PATCH /widget-settings` + config GET): ports, trailing slash rejected, path rejected, IDN, uppercase, `*.` apex non-match (E8, E9).
- [ ] Actions `CreateWidget`, `UpdateWidgetSettings`, `DeleteWidget` + events `widget.created`, `widget.settings_updated`, `widget.deleted` + audit entries for origin/anonymous changes.
  - Accept: create makes channel_connection type `widget` in same tx; update bumps `config_version`; limit via `Entitlements.Limit(ws, "widget.max_per_workspace")` (10, -1, override 20); `widget` channel type + `widget_visitor` identity type registered.
  - Tests: 10 widgets, each own snippet and inbox; eleventh 422 `entitlement_limit_reached` (E159, E116); tx rollback leaves nothing.
- [ ] `EnsureDefaultWidget` on workspace initializer hook (after 03 `EnsureDefaultInbox`, in `CreateWorkspace` tx).
  - Tests: new workspace has one widget on default inbox; re-run idempotent; initializer failure rolls back.
- [ ] Entitlement key `widget.max_per_workspace` in catalog. Permissions present; add missing role tests.
  - Tests: role matrix; member token with `settings:write` denied `widgets:manage`; no role names in 01 policy (lint).
- [ ] `WidgetSettingsController` (index/store/show/update/destroy), requests, `WidgetSettingsResource`, `WidgetSettingsPolicy`. Fields of README table only; settings.md additions land M8.
  - Tests: happy, 422 messages, member forbidden, token ability, other workspace 404, idempotent POST.
- [ ] `ResolveBranding` + `branding_locked` (12 `BillingStatus`; fake in tests; self-host default hidden).
  - Tests: 5 rows from README.
- [ ] Middleware `widget.app`, `widget.origin` with no-Origin table incl. `native_apps_enabled` (migration `20261201010007`), `cors:widget`.
  - Tests: preflight allowed and blocked; 403 body; empty allowlist allows any; no Origin rows of table (E193).
- [ ] `ShowWidgetConfigController` + otter cache keyed `wgt:config_version:locale` + PubSub bust `BustWidgetConfigCache` + `BumpWidgetConfigVersion`. Blocks from 13, 14, 15 absent until owners land.
  - Tests: ETag 304; update on replica A invalidates B; block absent when flag off.
- [ ] `ComputeTeamAvailability` + availability endpoint (`within_hours` always true until 17).
  - Tests: online only with realtime connection in last 60 s; avatars max 3; no emails.

## M2 Visitor sessions and messaging with SSE (track 01 widget-platform)

- [ ] `auth:widget`: hash lookup via resolver, expiry, revoke, workspace into context, sliding anonymous expiry.
  - Tests: 401 `session_expired`, `session_revoked`, `session_invalid`.
- [ ] `CreateAnonymousSession` + `StartWidgetSession` anonymous branch + `POST /sessions`: lead, `widget_visitor` identity, token, honeypot, `sdk.name` buckets.
  - Tests: config only creates no contact (E14); honeypot silent (E82); unknown name bucketed.
- [ ] `ResumeWidgetSession` + `GET /session` + `DELETE /session`; handshake rows without JWT.
  - Tests: each row; identified token on `GET /session` 409 `identity_proof_required` (factory row); logout idempotent.
- [ ] Contact endpoints + `UpdateWidgetContact`.
  - Tests: user contact 403 `identity_locked`; duplicate email identity not stolen; NFC + bidi strip; written key removed from `trusted_attribute_keys` (E143).
- [ ] `StartWidgetConversation` + `SendWidgetMessage` wrapping 03 `ReceiveInboundMessage`; dedupe; source incl. `widget_id`; spam on bot score; `IsTest`; stamp `metadata.widget`.
  - Tests: E28, E30, E31, E32, E41, E25, E130, E144, E145, E146.
- [ ] Conversation and message list and show + widget resources, `customer_status` computed, same-person scope.
  - Tests: note never returned (E45); other contact 404 (E34); other widget 404 and not listed (E152); email-origin listed only in matching inbox (E153); imported rule (E151); `survey_request`, `ai.confirmation` exposure rules; status table values.
- [ ] Read state + `MarkWidgetConversationRead` + `message.delivery_updated`; unread count per widget; `seen_by_team` broadcast.
  - Tests: pointer never moves back; one event per batch; never `conversation.read`; no member id.
- [ ] Typing indicators + `BroadcastVisitorTyping`; register 07 handoffs route.
  - Tests: E66, E67, E99.
- [ ] Pings: `RecordWidgetPing`.
  - Tests: `?token=abc&page=2` stored `?page=2`; 4 KB limit.
- [ ] Attachments: `CreatePendingUpload`, direct PUT, `CompleteUpload`. Policy and scan: 03 owner (S-01, S-02, X7), builds on F14 download route: assume member and signed widget URLs go through that route, `attachments.scan_status` respected.
  - Tests: size 413; blocked extension; presign on `local` and SeaweedFS; body over size aborted; E49, E50, E52; wrong session 404.
- [ ] `IssueWidgetRealtimeToken` + `POST /realtime-tokens` + `OnConnecting` wiring on existing uni-SSE handler (revoke re-check, cap, log redaction).
  - Tests: own channel streams; token as REST bearer 401; E68, E70, E71, E59, E72.
- [ ] `BroadcastToWidget` + `payloads.go` (widget-safe, `widget_id`, 10 KB fallback); history for stored, none for ephemeral. `RecordTyping` relay moved onto `payloads.go`.
  - Tests: snapshot every payload; note never published (E45); recovery true and false (E64, E60, E73); `widget_id` present (E158).
- [ ] Session revoke disconnects streams, calls 14 `RevokePortalSessions` when present.
  - Tests: closed within 5 s (E20, E22, E26).
- [ ] `RepointWidgetSessions` + eraser `EraseWidgetContactData`.
  - Tests: E23, E24, E22, second run no-op.
- [ ] `GET /updates` from DB with signed cursor.
  - Tests: E60, 100 cap `has_more`, note excluded.
- [ ] Protocol e2e (Node, plain EventSource polyfill): handshake rows without JWT, realtime token, teammate reply under 300 ms, kill and recover, buffering proxy then polling (E58).

## M3 Connect static widget (track 01 widget-platform)

Per [client.md](client.md) "File-by-file".

- [ ] `resources/widget/client/` protocol layer + unit tests on injected fakes of browser APIs (fetch, storage, event source are process boundary for this code; server behavior covered by protocol e2e).
- [ ] `core.ts` boots from config; defaults only for first paint; errors per Loud console errors. Sample and `simulate` moved to chunk `preview`; `demo`, `latency` removed.
  - Accept: production core has no sample strings (build check); core gz under budget.
- [ ] Views and controllers read store; composer sends through outbox; attachments; voice note upload (chunk `voice`); read-state; typing; email capture.
- [ ] Launcher: `tab` style, label. Copy fix "Powered by dimmi", link `branding.url`. CSS logical properties.
- [ ] `scripts/build-widget.ts`: gzip budgets per named chunk, published in manifest (loader 3 KB, core 35 KB, chunks per [settings.md](settings.md)).
  - Accept: CI fails over budget.
- [ ] Playwright `tests/e2e/widget.spec.ts`, light and dark: launcher on plain page, send, teammate reply via API arrives live, reopen after reload shows history, attachment, polling when `/connection/uni_sse` blocked, wrong app id console error and no launcher, disallowed origin, strict CSP page, phone width, SPA route change sends ping.
- [ ] Freeze: snapshot + fixtures committed; oasdiff required.

## M4 Connect settings page with live preview (track 01 widget-platform)

Per [settings-page.md](settings-page.md).

- [ ] Move `MessengerSettings` to `features/settings/messenger/`; `controls.tsx` controlled values; `api.ts`; widget list + new widget dialog + URL selection.
- [ ] Draft + save bar; every static control bound per table.
- [ ] `/widget-preview/{widget}` SSR page (session member), `?embed=1`, `preview` chunk, `postMessage`, `preview.tsx`, toolbar.
  - Tests: non-member 404; token only in fragment; frame-ancestors self; origin check in host script; draft never sent to server.
- [ ] `MintWidgetTestSession` + endpoint + `test_token` branch + per-member test lead + cap; "Try it for real".
  - Tests: E94, E95, E93, E96.
- [ ] Playwright `tests/e2e/messenger-settings.spec.ts` for this milestone's controls.

## M5 Install snippets and installed status (track 01 widget-platform)

- [ ] Install tracker `support/widgetinstall` fed by `widget.origin`, 60 s flush, shutdown flush.
  - Tests: config GET alone marks installed after flush (E113); 1,000 requests 1 write; blocked listed; two replicas keep max; `app` origin for no-Origin requests.
- [ ] `BuildInstallSnippets` + install endpoint; templates `resources/widget/install/<id>.tmpl` = sdk-contract text. Native ids return "available soon" block until 18 publishes; `webview` ready after M11.
  - Tests: blocks snapshot per id; unknown id 422; app id only in env lines; self-host subpath (E104).
- [ ] Install section UI: framework choice, blocks, copy, status states, origins seen and blocked with add.
  - Tests: Playwright: status flips after loader visit from second origin; blocked add updates allowed sites.
- [ ] MCP `dimmi_list_widgets`, `dimmi_get_widget_install`, `dimmi_update_widget_settings`.
  - Tests: schema from huma; ability; diff output; confirm gate on origins.

## M6 Anti-abuse and offline email fallback (track 01 widget-platform)

- [ ] `app/support/pow` on altcha-lib-go, single use, 428 flow; solver worker in `client/session.ts`.
  - Tests: E134, E135, expired, E81, E136.
- [ ] `ScoreWidgetBot` + spam at conversation create.
  - Tests: E83, E84; threshold spam; metric.
- [ ] Rate limit buckets wired.
  - Tests: E39, E40, E80; self-host `0` disables.
- [ ] `ScheduleWidgetOfflineEmail` + `SendWidgetOfflineReplyEmail` + `WidgetOfflineReplyMail`. Delivery only via 04 `email.SendWidgetOfflineReply`; temporary local fake action until track 05 provides it, then real.
  - Tests: E75, E74, E76, E79, E77, E131; link has `#dimmi-conversation`.
- [ ] Playwright: leave email, close tab, reply in inbox, Mailpit gets message once, opening link shows thread.

## M7 Signed-in identity (track 09 widget-identity)

02 M1 to M4 land first or alongside, then:

- [ ] Wire `identity.VerifyIdentityToken` (`Surface: widget`) + `identity.IdentifyVisitor` into `StartWidgetSession` and `PATCH /session`; `client/identity.ts` identity sources on.
  - Tests: all JWT rows; `previous_token` merges; other `sub` revoked `identity_switch`; same `sub` 200 reuse; invalid or foreign `previous_token` ignored; failed POST no effect; PATCH 409; PATCH revives expired.
- [ ] Same-person tests through real tokens: E152, E155, one contact across two widgets, inbox shows both threads.
- [ ] Verified member-email test detection.
  - Tests: unverified does not set test; deactivated member does not.
- [ ] Playwright: example page with identity URL; sign in soft navigation merges lead; sign out goes anonymous within one navigation; account switch.

## M8 Settings expansions (track 01 widget-platform)

- [ ] Migrations `20261201010001`, `20261201010003`; models, requests, resource fields; config output per api.md, `ResolveWidgetTexts` for `default_locale` only.
  - Tests: every field 422 and happy; config shape snapshot; config gz under 4 KB; web-only fields present for all clients.
- [ ] Assets: `StoreWidgetAsset`, public serve route, prune job.
  - Tests: reencode strips EXIF; HTML renamed png refused (E179); limits; immutable headers; in use 409 (E177); guard false keeps rows.
- [ ] Widget: logo, company name chain, header and button colours, inks, gradient, image background, launcher image, shape, sound flag, tab order, `open_to`, privacy notice, team intro, welcome, delay notice, home links.
  - Tests: `widget.test.ts` per setting through `Dimmi("update")`; E165, E170 to E176, E181; core budget.
- [ ] Page rules matcher + route change handling; shared matcher module also used by dashboard tester.
  - Tests: table of patterns; E167 to E169; `open()` works when hidden; server has no rule logic (no config vary).
- [ ] Page controls per "New controls"; `sortable-tabs.tsx`; image field.
  - Tests: Playwright per group, keyboard reorder, preview reflects, E180.

## M9 Translations (track 01 widget-platform)

- [ ] `resources/widget/lang/` 28 files + `app/support/widgetstrings` + key check test.
  - Tests: missing key fails; RTL `dir`; negotiation table (E161, E162).
- [ ] Migration `20261201010002`, translation API, `ResolveWidgetTexts` full chain, config `?locale=`; offline email and availability text in contact locale.
  - Tests: chain rows incl. E163, E166, E164; PATCH bumps version and busts cache; 61st 422; mail in French.
- [ ] SDK `locale` option and `update({locale})`; RTL layout pass.
  - Tests: Arabic page Playwright, light and dark, no physical margin regressions.
- [ ] Languages section + dialog.
  - Tests: Playwright add French, preview language switch, missing count.

## M10 Site import (track 01 widget-platform)

- [ ] Migration `20261201010004`, `StartWidgetSiteImport`, job, `app/support/brandimport`, event.
  - Tests with `httpclienttest`: every signal row; every failure code; E187 to E192. With real safedial against local listener: E184. Limits E185, E186.
- [ ] Dialog: URL, progress, proposal, apply to draft.
  - Tests: Playwright with seeded result; failure text shown; nothing changes until Save.

## M11 Hosted page (track 01 widget-platform)

- [ ] `data-mode="page"` in core; `/messenger/{app}` route, templ, headers, params; `sdk` name `dimmi-hosted`; event bridge.
  - Tests: E196, E197, E199; noindex header; no launcher; Playwright phone width light and dark.
- [ ] Migration `20261201010008`, `POST /session-transfers`, `transfer_token` branch, parent revoke revokes child; `#dimmi-transfer`, `#dimmi-identity`, `#dimmi-logout` fragment handling.
  - Tests: E194, E195; single use; same contact and expiry; other widget refused; token absent from access log.
- [ ] Apps section controls + `webview` install block.

## M12 Live conversation features (track 01 widget-platform)

- [ ] Client: status chip, seen line (03 toggle), typing line, cards unchanged.
  - Tests: Playwright through API: teammate types, sees name; reads, Seen appears; close sets chip; toggle off hides; E209.
- [ ] AI streaming in client per [client.md](client.md); widget-safe `metadata.ai.run_id`; 07 provider fake only.
  - Tests: deltas in order, gap, final replaces once; E206 to E208; polling shows typing then final; verify mode indicator only. No real LLM ever.
- [ ] Availability `within_hours` from `schedule_id` when 17 present (skip silently until `app/support/officehours` exists).

## M13 Developer context (track 01 widget-platform)

- [ ] Loader ring; chunk `dev-context`; request field; `RedactDevContext`; `metadata.dev_context`.
  - Tests: off means no chunk request and field ignored; limits; redaction table; E210 to E214; widget routes never return it.
- [ ] Inbox section `customer-panel/developer-context.tsx` (needs track 04 customer panel milestone; popover until then).
  - Tests: Playwright: page throws, visitor sends, teammate sees error.
- [ ] Settings toggles with warning line.

## M14 Custom widget API domain (track 01 widget-platform)

- [ ] Migrations `20261201010005`, `20261201010006`; `AddWidgetDomain`, check job on `framework/customdomain`, claim check + TLS authorizer extended, API, event.
  - Tests with `a.DNS.Fake()`: status machine, E200, E201.
- [ ] `widget.host` middleware; config, loader, install use host.
  - Tests: E202 to E205; stream works on host; no cookies.
- [ ] Domain section UI.
  - Tests: Playwright add domain, DNS rows, check result.

## M15 Tabs and seams: 07, 08, 13 to 16 (track 01 widget-platform)

- [ ] Help tab: 08 routes in widget group, per-widget help center Select; tests anonymous reads help; agent-only article never; unlinked 404.
- [ ] `changelog_tab_enabled`, `feedback_tab_enabled` rules; session fields; `Dimmi-Feedback-Session` CORS; mounts.
  - Tests: E125, E127; fields only when on; preflight; route list.
- [ ] 07 action-call routes; `ai.action_call_updated`; card.
  - Tests: E137 to E142; payload never carries input or response.
- [ ] `intercom_shim`; shim chunk; `filter[intercom_id]` requirement.
  - Tests (Playwright, `sdk/examples/intercom-shim`): mapped calls; E147 to E150.

## M16 SDK packages and examples (track 01 widget-platform)

- [ ] `@dimmi/browser`, `@dimmi/react`, `@dimmi/nextjs` (+ adapters, `/portal-sso`), `@dimmi/server` (+ `/express`, `/hono`). ESM + CJS, `sideEffects: false`, `.d.ts`, exports conditions; size deltas in CI.
  - Accept: each sends `sdk.name`; retired names absent (CI grep). Identity parts exercised after M7.
- [ ] `sdk/examples/<id>` + Playwright per example: anonymous boot, sign in, account switch, sign out, static stays static, E123, E121, `track` triggers survey, `show` every view. Runs in `sdk-ci` job.

## M17 Hardening (track 01 widget-platform)

- [ ] Load test: 10k uni-SSE streams + 200 msgs/s; RSS and p95 recorded.
- [ ] `PruneWidgetSessions` + metrics + log redaction (`dmi_ws_`, `dmi_wst_`, `dmi_wtt_` absent).
  - Tests: guard false keeps rows (E118); true deletes.
- [ ] Cross-tenant suite: every widget and child-collection endpoint with other workspace's ids 404.
- [ ] Manual e2e flow table extended in `docs/testing/manual-e2e.md`.

## Done when

- `make lint test e2e` green; protocol e2e green; budgets green.
- Demo script:
  1. `docker compose up`; create workspace; messenger page shows default widget and snippet.
  2. Change colours, logo, French text: preview updates live; Save; paste snippet in `examples/plain.html`, open: launcher matches; DB has no new contact.
  3. Send "hello": inbox shows conversation under 300 ms with page URL and browser.
  4. Teammate replies: visitor sees typing, reply, Seen, status chip live; close tab, wait 2 minutes: email arrives; reply by email lands in same conversation.
  5. Set allowed sites to other domain: console `origin_not_allowed`, launcher gone, page shows blocked origin with Add.
  6. Second widget on other inbox: own snippet, own threads; signed-in user is one contact across both, thread stays where it started.
  7. Next.js example signed in: previous user history shown; sign out: anonymous within one navigation.
  8. Block `/connection/uni_sse` in devtools: replies via polling within 6 s; unblock: reconnects, no duplicates.
  9. Dashboard "Try it for real": conversation marked Test, not in usage ledger.
  10. Script 500 session creates from one IP: PoW then 429; no billable conversations.
  11. Open `/messenger/{app}` on phone width; custom domain serves same widget.
