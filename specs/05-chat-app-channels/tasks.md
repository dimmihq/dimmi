# 05 Chat app channels: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Tests never hit network: recorded fixtures under `tests/fixtures/channels/<provider>/` via `httpclienttest.New(a.HTTP)` (SDK clients built on `a.HTTP.StdClient()`), fake gateway/socket servers in-process. Test rule (testing.md): actions, jobs, controllers, listeners = integration tests on real Postgres + Valkey + real App; unit tests only for pure parsing/format/mapping in `app/drivers/channel/*`; no mocks; tests named in plain words, every edge case covered. Libraries: slack-go/slack, disgoorg/disgo v0.19.x (`CGO_ENABLED=0`, depguard denies `disgo/voice`), go-telegram/bot; WhatsApp own thin client (providers.md Libraries). Prereqs: foundation done; 03 contract (`contracts.Channel`, `ChannelManager`, `conversations.ReceiveInboundMessage`, `UpdateMessageDelivery`, outbound jobs) merged or stubbed; 03 extensions X1, X3-X5 (`03-shared-inbox/channel-contract.md` section "Additions for 05 (X1, X3-X5)") merged.

## M1 Channel plumbing (shared by all four)

- [ ] **Migrations + queries + factories.** `20261101000502`..`000510` (indexes, 7 tables, RLS; no core CHECK migration). Register identity types (`slack_user`, `discord_user`, `telegram_user`, `whatsapp_wa_id`, `whatsapp_bsuid`) and channel types in foundation registries at boot. sqlc queries per table. Factories for connection types, rooms (Slack), states, templates, inbound events.
  - Accept: `dimmi migrate` clean on fresh + on DB with foundation rows; inserting `contact_identities` row with `whatsapp_bsuid` passes with no 05 migration touching core tables (test); unregistered type rejected by 03 action (edge case 150).
  - Tests: lint check (tenant queries filter `workspace_id`), RLS cross-tenant test for each tenant table.
- [ ] **Signature middleware x4 + raw body capture.** Slack via `slack.NewSecretsVerifier` (v0 HMAC + 300 s window), Discord via disgo `httpserver` Ed25519, Telegram secret header (own), Meta `sha256=` HMAC (own); size caps; constant-time compares.
  - Accept: invalid signature 401 before JSON parse; oversize 413.
  - Tests: known-answer vectors per provider (Slack docs example, Ed25519 RFC 8032 vector, Meta sample), timestamp skew, header case-insensitivity.
- [ ] **StoreInboundEvent + process job skeleton.** Valkey fast-path dedupe (`SET NX EX`, per-provider window `channels.inbound.dedupe_windows`), insert raw event (durable unique backstop), InsertTx `process_channel_inbound_event`, per-thread advisory lock, `held` for suspended workspace, status transitions, prune job (`RetentionGuard` per workspace batch and per linked conversation), `POST /channels/{chn}/inbound-events/{cie}/replays` + `channels:replay`.
  - Accept: duplicate POST yields one row, one job; webhook p99 under 150 ms in bench (local Postgres).
  - Tests: dedupe (Valkey hit: no DB write; Valkey flushed: unique index still dedupes, edge 170), held/release, failure isolation, prune batch, fake guard false keeps rows (edge case 168), replay only `failed`.
- [ ] **Markdown AST renderer framework + splitter.** Walk framework `markdown.Parse` neutral AST, citation footer, table-to-code, fence-aware splitter.
  - Accept: splitter never breaks fence/link; parts within limit for 4 providers.
  - Tests: property test (random Markdown, join(parts) semantically equals input), golden cases.
- [ ] **Provider rate limiter + backoff helpers.** Token buckets on `a.Limiter` (Valkey driver, shared across replicas), 429 parsing helpers, `RateLimitedError` mapping, Discord global limiter (per-route buckets inside disgo), invalid-request breaker.
  - Tests: bucket math, header parsing, breaker trip/reset with frozen clock; two App instances share bucket via Valkey.
- [ ] **Socket supervisor + lease table.** Acquire/renew/release, fair share, reconcile loop, close on renew failure, state persistence API.
  - Accept: two-replica test: kill holder, other takes over under 30 s.
  - Tests: lease contention, partition simulation (renew fails), reconciliation of deleted connections.
- [ ] **Generic channel API (read/update/delete).** `GET /channel-types`, `GET /channels` (compact/full by ability), `GET /channels/{id}`, `PATCH`, `DELETE` (soft delete + provider cleanup job), `POST /channels/{id}/reconnections`, `POST credentials`, `GET health` (cached), `GET setup`, rooms index/update (Slack), `POST /channels/{id}/room-syncs` (202), member links CRUD, `GET /conversations/{c}/channel-state`. Policies, resources, audit log, `channel.updated` event.
  - Accept: widget/email listed read-only (`DELETE` 409 `channel_managed_elsewhere`); secrets never in any resource.
  - Tests: feature tests per endpoint (happy, validation, member vs admin, token ability, cross-workspace 404, idempotency on POST); `route:lint` clean (no verb paths); policies use `authz.Can` with `channels:manage` / `channels:inspect` (registered in 00-accounts catalog), no role names (edge case 169).
- [ ] **Connection state actions + events.** `MarkConnectionErrored` (transition-only `channel.errored`), `MarkConnectionDisconnected`, `MarkConnectionRecovered`, `check_channel_health` periodic fan-out, error catalog (`errors_catalog.go`) with human text + fix URL.
  - Tests: no duplicate events on repeated failures; recovery emits `channel.connected` reason `recovered`; every catalog code has message (integration test enumerates catalog).
- [ ] **Shared classify + identity + attachments helpers.** Author classification (self/bot/teammate/contact), identity claim builders, attachment streaming with host allowlist + caps + placeholders, `RecordContactState`, profile refresh job.
  - Tests: allowlist refusal (SSRF payload), size cap placeholder, profile name overwrite rule, display name sanitizing (157), `public_room` flag set per place type (156).
- [ ] **Cross-tenant and abuse suite.** Shared endpoints (Slack, Discord, Meta) with two workspaces: routing never crosses (158, 159); unsigned flood 401 without DB write (160); every API route other-workspace `chn_`/`chr_`/`wtp_`/`cie_` returns 404.
  - Tests: feature tests per case; RLS on with app role.

## M2 Telegram (proves pipeline end to end)

- [ ] **Telegram connect.** go-telegram/bot client on `a.HTTP.StdClient()`; token rule, `getMe`, `setWebhook` with secret, `setMyCommands`, transport `auto`, conflicts (409 elsewhere/already), `register_telegram_webhook` job, credentials rotate (same bot id check).
  - Tests: fixtures for getMe ok/401, setWebhook ok/error, port/public URL detection.
- [ ] **Telegram inbound.** Private chats, `/start`, private-chat topics, `my_chat_member` (block/unblock), group messages ignored, media download (20 MB cap), entities to Markdown with UTF-16 offsets.
  - Tests: golden entity conversion (emoji, ZWJ, RTL), group message ignored (no room, no conversation), dedupe `update_id`.
- [ ] **Telegram outbound driver.** HTML renderer, split, private-chat `message_thread_id`, photo/document/media group, typing, error mapping (403 blocked, cannot initiate, 429 retry_after, 401).
  - Tests: 03 contract suite passes; format golden suite (250+ cases).
- [ ] **Telegram polling transport.** Lease `telegram_polling`, offset persistence, 409 conflict handling, switch webhook/polling.
  - Tests: fake Bot API server; offset never skips on crash between fetch and commit.

## M3 Slack

- [ ] **Slack shared install.** slack-go client; `POST /channels/slack/installations` (`S`: browser URL, callback needs session cookie) returns install URL with signed state (nonce single-use in Valkey), callback controller, `oauth.v2.access`, enterprise reject, scope check, reconnect reuse of disconnected row, room sync (`conversations.list` bot member), `channels:join` for chosen public channel.
  - Tests: state tampering, user mismatch, cancel, duplicate team across workspaces; bearer token (PAT or first-party app) on `installations` 403 `session_required`.
- [ ] **Slack custom app + manifest.** Manifest template, `create_app_url`, pending connection, url_verification for pending, credentials (`auth.test`, scopes header), Socket Mode token check.
  - Tests: manifest snapshot test; url_verification only path without secret.
- [ ] **Slack inbound.** Events endpoint (shared + per connection), retries/dedupe, message subtypes, rich_text + mrkdwn to Markdown, room rules (modes, group window, threads, DMs), teammate classification (ignored; auto-link by email for `community` rooms), edit/delete events dropped, `apps.event.authorizations.list` fan-out for multi-install shared channels, lifecycle events (uninstall, revoke, channel archive/rename/id change/shared).
  - Tests: Slack Connect external vs internal user classification, guest users, bot and teammate messages ignored, edit/delete events dropped (E103).
- [ ] **Slack outbound driver.** `markdown_text`, escaping, `username`/`icon_url`, thread_ts, file upload external flow (+ refs), close marker reactions, error mapping, 429.
  - Tests: contract suite; format golden suite; mention injection tests (`<!channel>` never pings).
- [ ] **Slack Socket Mode transport.** slack-go `socketmode` (`apps.connections.open`, envelope ack), reconnect on `disconnect`, lease, backfill after gap (`conversations.history`/`replies`, rate-aware).
  - Tests: fake socket server: ack within 3 s, refresh_requested reconnect, gap backfill dedupes.

## M4 Discord

- [ ] **Discord gateway on disgo.** disgo `gateway` + `sharding` ShardManager limited to lease-owned shard ids; our glue: identify budget guard, close-code policy (4004, 4010/4011 reshard), resume state persistence, lease hand-off. No own heartbeat/zstd code. CI `CGO_ENABLED=0` build + depguard `disgo/voice`.
  - Tests: fake gateway server scripts every op/close code; resume after takeover by second replica.
- [ ] **Discord shared install + custom bot.** `POST /channels/discord/installations` (`S`) returns OAuth bot invite URL (`scope=bot`, permissions 0), callback + code exchange, custom token validation (`/users/@me`, `/applications/@me`), guild picker, pending until `GUILD_CREATE`, custom bot cap via `Entitlements.Limit(ws, "channels.discord.custom_bots")` (catalog: self-host -1, cloud 3; 422 `entitlement_limit_reached`, edge cases 166, 167).
  - Tests: multiple guilds 422 with list, guild already bound elsewhere.
- [ ] **Discord inbound routing.** DMs only: shared-bot DM routing (select menu, sticky `dm_route`), guild messages ignored (intents exclude them), linked members flagged `IsTest`, REST backfill of active DMs.
  - Tests: DM routing 0/1/many guilds; sticky route expiry re-asks.
- [ ] **Discord interactions endpoint.** disgo `httpserver` Ed25519 verify, PING, component select for DM routing.
  - Tests: signature vectors; 3 s budget.
- [ ] **Discord outbound driver.** Markdown renderer + mention neutralizing, `allowed_mentions` none, split 2,000, multipart files (20 MiB, 25 MiB request batching, link fallback), author name prefix, typing (1 per 8 s), disgo REST limiter + Valkey global limiter + breaker, error mapping (50007, 10003).
  - Tests: contract suite; format golden suite; breaker.

## M5 WhatsApp

- [ ] **WhatsApp webhook endpoint.** GET verify (shared + per connection), POST HMAC, per-change events, routing by `phone_number_id` + WABA cross-check, Meta deauthorize + data deletion callbacks.
  - Tests: verify token mismatch 403; multi-change payload; signed_request parsing.
- [ ] **WhatsApp inbound.** All message types, BSUID/wa_id/phone identities (X1), thread keys + one-time rekey from wa_id to BSUID, system messages (number/user id change), context reply ids, window state update, statuses to `UpdateMessageDelivery` via refs, pricing metadata, reactions ignored, loop guard.
  - Tests: payload fixtures with and without `wa_id`, username present, `user_changed_user_id`, statuses order (read before delivered), failed after delivered.
- [ ] **WhatsApp outbound driver.** Own thin Graph client on `a.HTTP`; text + split, media upload by type with document/link fallback, captions, `to` vs `recipient` (BSUID), window check (X3) + 131047 mapping, templates send via `channel_options`, `MarkRead`, typing indicator, error catalog (131026, 131042, 131048, 131049, 131056, 130429, 132xxx, 133010, 190, 368, 80007).
  - Tests: contract suite; window boundary at 24 h with frozen clock; pair-rate snooze growth.
- [ ] **Embedded Signup (cloud) + custom connect.** `GET /channels/whatsapp/embedded-signup-config` + `POST /channels/whatsapp/embedded-signups` (both `S`: FB SDK popup in dashboard browser), code exchange, subscribed_apps, register with PIN (generated, encrypted), phone info, onboarding-limit error, custom connect with verify token + callback URL, `pending` to `active` on verification/subscription.
  - Tests: double submit idempotent; register failure 424 keeps nothing half-active; scopes missing on custom token.
- [ ] **Checklist + templates.** `refresh_whatsapp_checklist` (all steps, webhook triggers, 6 h periodic, `GET /channels/{chn}/whatsapp-checklist?refresh=true` throttled 1/min); `POST /channels/{chn}/whatsapp-template-syncs` (202), templates create (text header/body/footer/quick replies, examples)/delete, `message_template_status_update` handling, `whatsapp_template.status_changed` event, composer list endpoint with locale default.
  - Tests: checklist step mapping table-driven; template validation rules; status webhook for unknown template triggers sync.

## M6 Lifecycle, CLI, MCP, hardening

- [ ] **Close/reopen markers + read/forget listeners.** Slack reaction, WhatsApp mark read on `conversation.read` (30 s coalesce), contact merge state collapse, member deactivation unlink (`member.deactivated`), held release on `workspace.unsuspended`/`workspace.restored`, `system.app_url_changed` re-registration listener (11).
  - Tests: each listener.
- [ ] **Contact eraser `EraseChannelContactData`.** `app/actions/channels/erase_channel_contact_data.go`, registered on 03 eraser registry: deletes `channel_contact_states`, contact's `channel_message_refs`, inbound event payloads matching provider user ids.
  - Tests: erase removes all three for contact, other contact rows untouched, second run no-op, cross-workspace contact id no-op; 03 registry test lists it; edge case 146.
- [ ] **Ingest via `AfterPersist`.** `RecordContactState` + `channel_message_refs` writes run in 03 `InboundMessage.AfterPersist`.
  - Tests: forced hook error rolls back message and state; job retry ingests once (edge case 164).
- [ ] **CLI commands.** `channels:add` (telegram, slack shared/custom/socket, discord shared/custom, whatsapp cloud/custom), `channels:list`, `channels:manifest`, `channels:check` (exit codes), `channels:replay`; `dimmi doctor` hook.
  - Tests: CLI against fake API (09 harness), hidden token input, `--json` output schema.
- [ ] **MCP tools.** `dimmi_list_channels`, `dimmi_get_channel_health`, `dimmi_connect_telegram_bot`, `dimmi_list_whatsapp_templates` with hints and ability checks; registered in 09 toolset `channels`; connect via 09 `app/mcp/confirm` gate.
  - Tests: MCP schema snapshot; ability denial; toolset under 2,000 tokens; connect without confirmation creates nothing (163); token absent from logs.
- [ ] **Chaos + load.** Failover test (kill holder mid-burst of 1,000 Discord DMs: 0 lost, 0 dup), webhook burst 3,000/min per connection, outbound 429 storm, RSS per socket under 4 MB, inbound latency budget p95 under 2 s.
  - Accept: CI benchmark job records numbers; regressions over 10% fail.
- [ ] **Docs pages data.** Per-channel setup guides referenced by `docs_url`/`help_url` (content owned by docs site; spec ensures every error catalog code has a help URL anchor).

## Done when

- `go test ./...` green (no network). Contract suite passes for 4 drivers. Format golden suites pass.
- Demo script (cloud staging, then self-host on laptop):
  1. `dimmi channels:add telegram`, paste token. Open printed `t.me` link, send "hi" with photo. Inbox shows conversation under 2 s with image. Reply "**Hello** there" with PDF from inbox: Telegram shows bold text + file. Typing visible while composing.
  2. Dashboard "Add to Slack" (cloud) / `dimmi channels:add slack --socket` (laptop, no public URL). Invite bot to Slack Connect channel. External user posts question: conversation created; reply lands in thread with teammate name/avatar. Close conversation: checkmark reaction on root.
  3. Add Discord bot to server. Member DMs bot: DM conversation in inbox; reply lands in DM; typing shown.
  4. Embedded Signup WhatsApp (cloud test number). Checklist lists steps with statuses. Send WhatsApp message: conversation; reply shows delivered then read ticks; typing shown. Freeze clock +25 h: reply blocked with `channel_window_closed`; send approved template with variables: delivered.
  5. Kill replica holding Discord shard during 200-DM burst: all 200 in inbox once.
  6. Uninstall Slack app in Slack: channel red "Slack app was uninstalled", `channel.disconnected` webhook fires, admin gets notification (06). Reconnect: green, rooms restored.
  7. `dimmi channels:check` exits 0; revoke Telegram token in BotFather; `channels:check` exits 1 with "Telegram token revoked: create new token in @BotFather and rotate credentials".
