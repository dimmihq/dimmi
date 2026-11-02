# 05 Chat app channels: Slack, Discord, Telegram, WhatsApp

Style: caveman. Date 2026-11-01. Read with `00-conventions/README.md`, `00-foundation/*`, `03-shared-inbox/README.md` + `03-shared-inbox/channel-contract.md` (owns `contracts.Channel`, `conversations.ReceiveInboundMessage`, outbound pipeline, `send_channel_message` / `send_channel_typing` jobs).

Files in this folder:

| File | What |
|---|---|
| README.md | this: goal, scope, data model, code map, API, events, jobs, config, permissions, budgets |
| [providers.md](providers.md) | per provider: verified 2026 API facts, connect flow, scopes, inbound, outbound, formatting, threading, identity, receipts, limits, lifecycle |
| [edge-cases.md](edge-cases.md) | decided behavior for 171 cases |
| [tasks.md](tasks.md) | build checklist, demo script |

## Goal

Customers write from Slack, Discord, Telegram, WhatsApp. Each lands in same Dimmi inbox as widget and email, same conversation model, same Dimmi Agent, same billing rule. Teammates reply from inbox with attachments; reply reaches customer in thread on their app. 05 owns four channel drivers, generic channel connection CRUD API (`/api/v1/channels`), per-type connect flows (cloud shared app/bot, self-host own app), inbound webhook/socket ingestion, WhatsApp verification checklist and minimal template management.

Why: roadmap launch item 5 (`research/roadmap.md` "Chat app channels"). Vibe-coder communities live in Discord and Telegram, early B2B customers in Slack, most of world on WhatsApp. Featurebase gates Slack/Discord to paid tier; Fin added Discord and Telegram in 2026. We ship all four free, self-hostable.

## World-class bar (measurable)

| Area | Bar |
|---|---|
| Time to connect | Telegram: paste BotFather token, live under 60 s. Slack cloud: OAuth + pick channel under 3 min. Discord cloud: invite to server under 3 min. Self-host Slack/Discord own app with generated manifest under 10 min. WhatsApp: as fast as Meta allows; checklist shows every pending Meta step with status and fix link |
| First message | `setup.test_link` on every connection (t.me link, wa.me link, Slack channel link, Discord bot DM link): connect to message-in-inbox under 5 min |
| Inbound latency | provider delivery to inbox realtime p95 under 2 s (webhook ack p99 under 150 ms) |
| Outbound latency | teammate reply to provider accepted p95 under 1.5 s when not rate limited (03 budget 2 s incl. queue) |
| Zero duplicates | provider retries (Slack 3 retries, Meta 7-day retries, Telegram re-delivery, Discord resume replay) never create second message: dedupe tests per provider |
| Zero loss across failover | Discord gateway / Slack Socket Mode / Telegram polling: lease failover under 30 s, resume or REST backfill closes gap; chaos test kills holder replica mid-burst, 0 lost messages |
| Formatting fidelity | Markdown converters (4 out, 4 in) pass golden suite of 250+ cases each; no raw escape characters visible to customer; zero unintended `@everyone` / `@here` / `<!channel>` pings (allowed_mentions none, Slack special mentions escaped) |
| Errors explain themselves | every provider error code we can receive mapped to `last_error_code` + human `last_error` + fix link; 0 "unknown error" strings in UI for codes in mapping table |
| Self-host without public URL | Discord (gateway), Slack (Socket Mode), Telegram (long polling) work on laptop / private LAN. Only WhatsApp needs public HTTPS (Meta requirement) |
| Memory | one Discord shard or Slack socket or Telegram poller under 4 MB RSS; idle binary budget (50 MB) unchanged with all four connected |
| Security | 100% inbound requests signature-verified before body parse beyond size cap; tokens envelope-encrypted, never logged (redaction patterns tested) |

## Scope

### In (MVP)
- Drivers implementing `contracts.Channel` (03) for `slack`, `discord`, `telegram`, `whatsapp`: `Type`, `Capabilities`, `Send`, optional `ChannelTyping.SendTyping`, `ChannelReadMarker.MarkRead` where provider supports.
- Inbound: Slack Events API (HTTP) + Socket Mode (self-host option); Discord Gateway WebSocket (lease-held, sharded; DM events only) + HTTP interactions endpoint (DM routing select menu); Telegram webhook with `secret_token` + long polling (self-host option); WhatsApp Cloud API webhooks with `X-Hub-Signature-256`.
- Raw inbound event store + async processing (`channel_inbound_events`), dedupe, suspended-workspace hold.
- Generic channel connection API: list, show, update, delete (disconnect), reconnect, credentials rotate, health, setup info. Lists all types (widget/email read-only metadata here; created by 01/04).
- Connect flows: Slack OAuth v2 (shared app) and custom app (manifest + bot token + signing secret, optional app token for Socket Mode); Discord OAuth bot invite (shared bot) and custom bot token; Telegram bot token; WhatsApp Embedded Signup v4 (cloud, Dimmi as Meta Tech Provider) and custom Meta app (system user token).
- Rooms: which Slack channels are watched, per-room mode and settings.
- Threading model per provider (Slack thread per conversation, Slack Connect, DMs; Discord DMs to bot; Telegram private chats; WhatsApp per user per business number).
- Contact identity mapping, profile enrichment, member links (Slack/Discord user to Dimmi member: teammate classification, test flag). Teammate native messages in provider threads ignored (not ingested).
- Formatting conversion Markdown to/from Slack `markdown_text` + mrkdwn, Discord markdown, Telegram HTML + entities, WhatsApp formatting. Message splitting within declared `MaxBodyChars`.
- Attachments both ways within provider limits; oversize outbound falls back to signed link.
- Delivery status mapping (WhatsApp sent/delivered/read/failed; others sent/failed), WhatsApp read receipts + typing indicator, Telegram/Discord typing.
- Rate limiting per provider (token buckets, Discord buckets, 429 Retry-After), invalid-request circuit breaker (Discord).
- Bot loop protection, uninstall / token revoked handling (status `error`/`disconnected` + `channel.errored`/`channel.disconnected` for 06 notifications).
- WhatsApp: 24 h customer service window state, template send via 03 `channel_options.template`, minimal template management (list, sync, create text template, delete), verification checklist API, BSUID (business-scoped user ID) support.
- Meta app compliance endpoints (deauthorize, data deletion callback) for cloud.
- CLI `dimmi channels:add <type>`, `channels:list`, `channels:manifest`, `channels:check`. MCP tools for listing channels, health, templates, Telegram connect.

### Out (later)
| Item | When |
|---|---|
| Team reply from Slack **notifications** (internal notification thread) | post-MVP: chat replies. 06 sends notifications only |
| SMS, Instagram, Facebook Messenger, Microsoft Teams | post-MVP: more channels |
| Inbound reactions mapped into timeline | post-MVP (needs 03 reaction model). MVP ignores inbound reactions (counted in metrics only) |
| AI streaming to chat apps (Telegram `sendMessageDraft`, Slack `chat.startStream`) | post-MVP; 07 sends whole message at MVP |
| WhatsApp Coexistence (WhatsApp Business app + API on same number), WhatsApp Groups API, WhatsApp media/auth templates, flows, catalogs, payments | post-MVP |
| Solution Partner credit line (Dimmi pays Meta) | not planned; Tech Provider model, Meta bills business directly, Dimmi passes through nothing |
| Slack Enterprise Grid org-wide install | post-MVP; MVP rejects org-wide install with clear error |
| Business-initiated first contact on Telegram/Discord/Slack | never possible (Telegram/Slack) or post-MVP (Discord DM to guild member) |
| Telegram deep-link identity handoff from widget (`?start=<signed>`) | post-MVP |
| Custom bot branding on cloud for Slack (own app) | supported (custom mode works on cloud); no extra UI polish at MVP |
| Backfill of history older than connect time | not planned. 16 importers (launch: importers) import helpdesk sources (Intercom, Zendesk, Help Scout), never chat-app history; imported conversations live on `import` connection, never on 05 connections |
| later: message-edit | edits and deletes both ways (Dimmi to provider `chat.update`/`editMessageText`/`PATCH`/`chat.delete`/`deleteMessage`, provider edit/delete events to Dimmi). MVP: delete local only; provider edit/delete events dropped |
| later: telegram-groups | groups via mention/reply chains, forum topics (`telegram_group`/`telegram_forum` rooms, `/support` in groups). MVP: private chats only; group messages ignored |
| later: discord-ticket-modes | forum posts, text-channel thread-on-ingest, private `/support` tickets, close tag/archive, MESSAGE_CONTENT intent + limited content mode, `/support switch`. MVP: DMs to bot only |
| later: teammate-echo | teammates replying natively inside Slack/Discord thread recorded as replies (03 X2 `InboundMessage.Echo`). MVP: teammate native messages ignored |
| later: chat-app-csat | CSAT over chat apps: 15 CSAT system messages sent to Slack/Discord/Telegram/WhatsApp. MVP: no system-message rendering rules in drivers |

## User stories

Workspace admin:
1. Connect Telegram by pasting BotFather token; see bot link; message bot from phone; see message in inbox.
2. Click "Add to Slack", approve, invite bot to `#acme-support` Slack Connect channel; each customer question becomes conversation; replies land in thread.
3. Click "Add to Discord", pick server; members DM support bot; each DM thread becomes conversation.
4. Connect WhatsApp via Embedded Signup; see checklist (business verification pending, display name approved, payment method missing) with links; status turns green when Meta approves.
5. See channel go red with "Slack app was uninstalled from Acme workspace" and one-click reconnect; get email/push (06).
6. Self-host: run `dimmi channels:add slack`, get manifest + create-app link, paste tokens, done. No public URL: pick Socket Mode.
7. Link my Slack/Discord account so my own messages count as teammate (ignored in `community` rooms, test-flagged in bot DMs).

Workspace member:
8. Reply to Slack/Discord/Telegram/WhatsApp conversation with attachments; see sent / delivered / read ticks where supported.
9. Try to reply to WhatsApp after 24 h: composer shows window closed, pick approved template, send with variables.

End customer:
10. Ask in Slack Connect channel, get answer in thread, keep conversing in thread.
11. DM support bot in Discord or Telegram, get answer in DM.
12. Message business WhatsApp number, see typing then reply, my message shows blue ticks when read.

Developer:
13. Use REST API to list channels, read health, rotate Telegram token, manage WhatsApp templates; receive `channel.errored` webhook (10) to page on-call.

AI agent:
14. Via MCP: `dimmi_list_channels`, `dimmi_get_channel_health`; Claude Code connects Telegram bot with `dimmi_connect_telegram_bot` (token passes through agent context; docs recommend CLI for secret entry).
15. Dimmi Agent (07) answers on every chat channel; adapter renders citations footer from `metadata.ai.citations`, shows typing while generating.

## Implements 03 `contracts.Channel`

03 owns interface in `03-shared-inbox/channel-contract.md` (canonical per `00-foundation/contracts.md`). 05 drivers implement it unchanged, exact names:
`Type() enums.ChannelType`, `Capabilities(conn *models.ChannelConnection) ChannelCapabilities`, `Send(ctx, conn *models.ChannelConnection, msg OutboundMessage) (SendResult, error)`, optional `ChannelTyping.SendTyping`, `ChannelReadMarker.MarkRead`. Errors: `contracts.ErrPermanent`, `ErrRecipientUnreachable`, `ErrWindowClosed`, `ErrConnectionBroken`, `ErrContentRejected`, `ErrUnsupported`, `*RateLimitedError`, `*SendError`. Inbound via 03 actions `conversations.ReceiveInboundMessage` (input `InboundMessage`, result `InboundResult`), `UpdateMessageDelivery`. Provider edit/delete events dropped before ingest (later).

Extensions X1, X3-X5 below defined canonically in `03-shared-inbox/channel-contract.md` section "Additions for 05 (X1, X3-X5)"; table here is summary, 03 wins on conflict (X2 teammate echo: later):

| # | Extension | Why |
|---|---|---|
| X1 | `enums.IdentityType` registry gains `whatsapp_bsuid` (registered by 05 at boot; `contact_identities.type` is registry enum, `00-foundation/extensibility.md` P1: no CHECK migration); `Recipient.IdentityType` may be `whatsapp_bsuid` | Meta business-scoped user IDs in all webhooks since early April 2026; phone (`wa_id`) omitted when user enabled username |
| X3 | optional interface `ChannelWindowResolver{ WindowState(ctx, conn, recipient Recipient) (open bool, closesAt time.Time, err error) }`; 03 request-time window check uses it when implemented, else `last_customer_message_at` | WhatsApp window is per (business number, user), any user message incl. ones in other conversations; conversation `last_customer_message_at` ignores non-substantive messages |
| X4 | `SendResult.Metadata["parts"]` convention: driver also writes extra provider ids to `channel_message_refs` (05 table) itself | split messages and Slack file shares produce several provider ids; part retries and receipts need all |
| X5 | `conversations.ReceiveInboundMessage` must tolerate `ExternalThreadKey` created by driver before call | already true per contract (key is input); no MVP user (Discord thread-at-ingest deferred) |

Driver package layout: `app/drivers/channel/{slack,discord,telegram,whatsapp}/` (adapter zone) each with `driver.go` (contract), `client.go` (SDK wrapper: slack-go, disgo, go-telegram/bot; own Graph client for WhatsApp; all on `a.HTTP.StdClient()`, providers.md Libraries), `inbound.go` (normalizer), `format.go` (Markdown converters), `errors.go` (provider error to `SendError` mapping).

### Capabilities declared (verified against provider docs 2026-11, details in providers.md)

| Field | slack | discord | telegram | whatsapp |
|---|---|---|---|---|
| Transport | push | push | push | push |
| StartConversation | never | never | never | with_template |
| Formatting | FormatSlackMrkdwn (sent as `markdown_text`) | FormatDiscordMarkdown | FormatTelegramHTML | FormatWhatsApp |
| MaxBodyChars | 12,000 (one `markdown_text` message) | 8,000 (split max 4 x 2,000) | 16,000 (split max 4 x 4,096 after entity parse) | 16,000 (split max 4 x 4,096) |
| Attachments / MaxAttachments | yes / 10 | yes / 10 | yes / 10 (media group max 10) | yes / 10 (one media per WhatsApp message) |
| MaxAttachmentBytes | 25 MB (global cap; Slack allows more) | 25 MB (over 20 MiB Discord default: link fallback) | 25 MB (over 50 MB never; photos over 10 MB sent as document) | 25 MB (images over 5 MB sent as document; video/audio over 16 MB link fallback) |
| AllowedContentTypes | nil | nil | nil | nil (driver maps unsupported types to document or link fallback) |
| Typing | no (bots cannot show typing outside Slack assistant threads) | yes (10 s) | yes (`sendChatAction` 5 s) | yes when room/connection setting `send_read_receipts` true (typing indicator also marks read; 25 s) |
| DeliveryReceipts / ReadReceipts | no / no | no / no | no / no | yes / yes |
| MessagingWindow | 0 | 0 | 0 | 24 h |

## Data model

### Core table changes

| Migration | Change | Why |
|---|---|---|
| `20261101000502_add_channel_lookup_indexes.sql` | `channel_connections (workspace_id, type, status) WHERE deleted_at IS NULL`; `channel_connections (type, external_id) WHERE deleted_at IS NULL` already unique (foundation), no change | list filters |

No CHECK migrations on core tables. `contact_identities.type` and `channel_connections.type` are registry enums (`text` + format CHECK `^[a-z][a-z0-9_]{1,39}$` + Go registry, extensibility.md P1). 05 registers in `app_service_provider.go` at boot: identity types `slack_user`, `discord_user`, `telegram_user`, `whatsapp_wa_id`, `whatsapp_bsuid`; channel types `slack`, `discord`, `telegram`, `whatsapp` (with driver). Next channel (SMS, Instagram, Teams) = new driver + registration, zero migration on the biggest tenant table. Migration slot `000501` unused (was CHECK swap).

`channel_connections` columns unchanged. Typed `config` shapes (`models.SlackConnectionConfig` etc.) and `credentials_encrypted` shapes in providers.md. `config` common keys owned by 05: `mode` (`shared`|`custom`), `external_name`, `bot_user_id`, `last_error_code`, `health` (last check result), `setup` (pending steps), `inbound_transport` (`webhook`|`socket`|`polling`|`gateway`).

`external_id` (routing key, unique per type among non-deleted):

| Type | external_id | Example |
|---|---|---|
| slack | `{api_app_id}:{team_id}` | `A07Q1:T024BE` (same Slack team can connect two different apps to two workspaces; shared app: one Dimmi workspace per Slack team) |
| discord | `{application_id}:{guild_id}` | `1290...:1102...` |
| telegram | bot user id (from `getMe`) | `7012345678` |
| whatsapp | `phone_number_id` | `106540352242922` |

Custom connections in `pending` before provider identity known keep `external_id` null (allowed by partial unique index).

### New tables

All tenant tables: `workspace_id uuid not null references workspaces on delete cascade`, `id uuid default uuidv7()`, `created_at`, `updated_at`, RLS in cloud, indexes lead with `workspace_id` unless stated.

#### channel_rooms (public prefix `chr_`)
Provider-side place watched by connection: Slack channel. DMs, Discord, Telegram, WhatsApp have no room rows at MVP.

| Column | Type | Notes |
|---|---|---|
| channel_connection_id | uuid not null references channel_connections on delete cascade | |
| kind | text not null CHECK in ('slack_channel') | Discord/Telegram room kinds later (Discord server threads, Telegram groups) |
| external_id | text not null | Slack channel id `C...`/`G...` |
| name | text not null | `#acme-support`; refreshed on rename events |
| status | text not null default 'active' CHECK in ('active','paused','removed','error') | `paused`: messages ignored; `removed`: bot left/channel deleted/archived |
| mode | text not null CHECK in ('shared','community') | `shared` (Slack Connect: other-org users are contacts, own-org users are teammates) or `community` (everyone is contact unless linked member) |
| settings | jsonb not null default '{}' | `models.ChannelRoomSettings` (providers.md): `group_window_seconds`, `close_marker`, `allow_bot_authors`, `inbox_id` override |
| is_shared_external | boolean not null default false | Slack Connect channel flag from `is_ext_shared` |
| last_error | text | e.g. "Invite @dimmi to #help" |
| last_seen_event_at | timestamptz | gap detection for backfill |
| removed_at | timestamptz | |
Unique `(channel_connection_id, external_id)`. Index `(workspace_id, channel_connection_id, status)`.

#### channel_contact_states
Per (connection, identity) reachability and window. One row per person per connection.

| Column | Type | Notes |
|---|---|---|
| channel_connection_id | uuid not null references channel_connections on delete cascade | |
| contact_identity_id | uuid not null references contact_identities on delete cascade | |
| last_inbound_at | timestamptz | any user-authored message incl. non-substantive (window source) |
| window_expires_at | timestamptz | WhatsApp: `last_inbound_at + 24h`; null others |
| reachable | boolean not null default true | false when blocked bot (Telegram 403), WhatsApp 131026 repeated, Discord DM closed (50007) |
| unreachable_reason | text | `blocked_bot`, `not_on_whatsapp`, `dm_closed`, `left_server` |
| unreachable_at | timestamptz | |
| dm_channel_id | text | Slack `D...`, Discord DM channel id; cache |
| profile | jsonb not null default '{}' | provider display name, username, avatar URL, language, Slack `team_id`, `is_restricted`; max 4 KB |
| profile_refreshed_at | timestamptz | users.info cache 24 h |
Unique `(channel_connection_id, contact_identity_id)`. Index `(workspace_id, contact_identity_id)`.

#### channel_message_refs
Every provider message id produced by or mapped to Dimmi message. Covers split parts, file shares, Telegram media group members.

| Column | Type | Notes |
|---|---|---|
| message_id | uuid not null references messages on delete cascade | |
| channel_connection_id | uuid not null references channel_connections on delete cascade | |
| external_message_id | text not null | `C123:1730000000.000100` (Slack channel:ts), Discord snowflake, Telegram `chat_id:message_id`, WhatsApp `wamid...` |
| part_index | smallint not null default 0 | 0 = primary (equals `messages.channel_message_id`) |
| kind | text not null CHECK in ('text','file','media_group','template') | |
| status | text not null default 'sent' CHECK in ('sent','delivered','read','failed') | per part; message status = min over parts (failed if any failed) |
Unique `(channel_connection_id, external_message_id)`. Index `(workspace_id, message_id)`.

#### channel_inbound_events
Raw verified inbound payloads. Persist then process (security.md: respond under 1 s).

| Column | Type | Notes |
|---|---|---|
| channel_connection_id | uuid not null references channel_connections on delete cascade | |
| provider | text not null CHECK in ('slack','discord','telegram','whatsapp') | |
| transport | text not null CHECK in ('webhook','socket','polling','gateway','interaction') | |
| dedupe_key | text not null | Slack `event_id`, Telegram `update_id`, Discord `{event}:{id}`, WhatsApp `sha256(change json)` per change |
| serial_key | text | thread key for per-thread serialization; null = no ordering need |
| kind | text not null | normalized: `message`, `status`, `lifecycle`, `room`, `interaction`, `template_status`, `account` (provider edit/delete events dropped before insert) |
| payload | jsonb not null | provider JSON (one event / one change); max 1 MB (larger rejected at controller) |
| status | text not null default 'pending' CHECK in ('pending','processed','ignored','failed','held') | `held` = workspace suspended |
| attempts | int not null default 0 | |
| error | text | |
| received_at | timestamptz not null default now() | |
| processed_at | timestamptz | |
Unique `(workspace_id, provider, dedupe_key)`. Index `(workspace_id, status, received_at)`; maintenance index `(received_at)` for prune (documented exception, no workspace lead). Retention: processed/ignored 7 days, failed/held 30 days (config). Payload holds PII: erased by contact eraser `EraseChannelContactData` (match by identity value; runs only after 03 `EraseContact` passed `RetentionGuard`) and by prune (calls `RetentionGuard`, Jobs).

#### channel_socket_sessions (infrastructure, no `workspace_id`, no RLS)
Lease + resume state for long-lived inbound connections (Discord gateway shards, Slack Socket Mode, Telegram polling).

| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| kind | text not null CHECK in ('discord_gateway','slack_socket','telegram_polling') | |
| key | text not null | Discord `{application_id}:{shard_id}/{shard_count}`; Slack `{connection_id}`; Telegram `{bot_id}` |
| owner | text | replica id (`hostname:pid:boot_ulid`) holding lease |
| lease_expires_at | timestamptz | renewed every 5 s, TTL 15 s |
| state | jsonb not null default '{}' | Discord `session_id`, `resume_gateway_url`, `seq`, `last_message_ids` per DM; Telegram `offset`; Slack none |
| connected_at, last_event_at | timestamptz | |
| last_error | text | |
| created_at, updated_at | timestamptz | |
Unique `(kind, key)`. Credentials never stored here (read from `channel_connections` / config).

#### channel_member_links (public prefix `cml_`)
Maps provider user to Dimmi member: teammate classification in Slack `community` rooms (their messages ignored) and `IsTest` for teammates testing own bot in DMs (D10). Native teammate replies as authored replies: later (teammate echo).

| Column | Type | Notes |
|---|---|---|
| channel_connection_id | uuid not null references channel_connections on delete cascade | |
| workspace_member_id | uuid not null references workspace_members on delete cascade | |
| external_user_id | text not null | Slack `T..:U..`, Discord user id |
| source | text not null CHECK in ('email_match','manual') | Slack auto-link when Slack profile email equals member user email (verified Dimmi email) |
Unique `(channel_connection_id, external_user_id)`, unique `(channel_connection_id, workspace_member_id)`.

#### whatsapp_templates (public prefix `wtp_`)
Cache of Meta message templates per WABA (minimal MVP management).

| Column | Type | Notes |
|---|---|---|
| channel_connection_id | uuid not null references channel_connections on delete cascade | |
| external_id | text | Meta template id; null while create request in flight |
| name | text not null | `^[a-z0-9_]{1,512}$` |
| language | text not null | Meta locale code `en_US`, `pt_BR` |
| category | text not null CHECK in ('marketing','utility','authentication') | authentication read-only (sync only) |
| status | text not null CHECK in ('pending','approved','rejected','paused','disabled','in_appeal','pending_deletion','deleted','limit_exceeded') | lowercase of Meta status |
| quality | text CHECK in ('green','yellow','red','unknown') | |
| components | jsonb not null | Meta components array as returned |
| parameter_count | int not null default 0 | body `{{n}}` count, header count in `metadata` |
| rejected_reason | text | |
| created_via | text not null CHECK in ('dimmi','meta') | synced-in templates `meta` |
| synced_at | timestamptz | |
Unique `(channel_connection_id, name, language)`. Index `(workspace_id, channel_connection_id, status)`.

### Public ID prefixes (new)
`chr_` channel_rooms, `cml_` channel_member_links, `wtp_` whatsapp_templates, `cie_` channel_inbound_events. `chn_` existing for channel_connections.

### Thread keys (`conversations.external_thread_key`)

| Provider / place | Key | Notes |
|---|---|---|
| Slack channel thread | `slack:r:{room_id}:{root_ts}` | room uuid not Slack channel id: channel ids change on Slack Connect share |
| Slack DM | `slack:d:{dm_channel_id}` | one open conversation at a time per DM |
| Discord DM | `discord:d:{dm_channel_id}` | |
| Telegram private chat | `telegram:p:{chat_id}` | |
| WhatsApp | `whatsapp:u:{bsuid}`; fallback `whatsapp:p:{wa_id}` when no BSUID | rekeyed on `user_changed_user_id` system webhook |

Unique scope per 03: `(channel_connection_id, external_thread_key)`.

### Dedupe keys (`InboundMessage.DedupeKey`)
`slack:{team_id}:{channel_id}:{ts}`, `discord:{message_id}`, `telegram:{bot_id}:{chat_id}:{message_id}`, `whatsapp:{wamid}`. Message identity, not event identity: same message via retry, Socket Mode + HTTP, gateway resume and backfill collapses.

## Code map

### Contracts / drivers / managers
- `app/drivers/channel/slack/{driver.go,client.go,inbound.go,format.go,errors.go,manifest.go,socket.go,files.go}`: `client.go` / `socket.go` wrap `slack-go/slack` (+ `socketmode`, `slackevents`); no own Web API or Socket Mode protocol code
- `app/drivers/channel/discord/{driver.go,client.go,inbound.go,format.go,errors.go,gateway.go,shards.go,interactions.go}`: `disgoorg/disgo` v0.19.x (REST + rate limiter, `gateway`, `sharding` with lease-owned shard ids, `httpserver` Ed25519); `gateway.go` / `shards.go` = close-code policy + lease glue only; no `buckets.go` (disgo limiter). Never `disgo/voice` (cgo)
- `app/drivers/channel/telegram/{driver.go,client.go,inbound.go,format.go,errors.go,poller.go,entities.go}`: `client.go` / `poller.go` wrap `go-telegram/bot` v1.27.x (`WithHTTPClient(a.HTTP.StdClient())`, `getUpdates`); poller keeps lease + offset persistence
- `app/drivers/channel/whatsapp/{driver.go,client.go,inbound.go,format.go,errors.go,templates.go,media.go,checklist.go,signup.go}`: own thin Graph client on `a.HTTP` (no mature Go SDK)
- `app/integrations/channels/`: shared plumbing: `markdown_ast.go` (walks framework `markdown.Parse` neutral AST; converters stay in 05), `splitter.go` (code-fence and paragraph aware split), `ratelimit.go` (provider token buckets on `a.Limiter`, Valkey driver: shared across replicas), `socket_supervisor.go` (lease manager for gateway/socket/polling), `lease.go`, `mentions_guard.go`, `attachments.go` (download via `a.HTTP` with caps, provider host allowlist), `identity.go` (identity claim builders), `classify.go` (contact vs teammate vs bot vs ignore), `backoff.go`, `errors_catalog.go` (code to human message + fix URL).
- `app/providers/app_service_provider.go`: register four drivers on `ChannelManager` (03); register channel types and identity types in foundation registries (`enums.ChannelType.Register`, `enums.IdentityType.Register`); start `SocketSupervisor` in `worker` role.

### Actions (`app/actions/channels/`)
- Connections: `list_channel_connections.go`, `update_channel_connection.go`, `disconnect_channel_connection.go` (provider-side revoke best effort + soft delete), `reconnect_channel_connection.go`, `rotate_channel_credentials.go`, `check_channel_health.go`, `mark_connection_errored.go` (called by 03 on `ErrConnectionBroken` and by 05 on auth failures; transition-only events), `mark_connection_disconnected.go`, `mark_connection_recovered.go`, `build_channel_setup.go` (manifest/URLs/steps).
- Connect: `start_slack_install.go`, `complete_slack_install.go`, `connect_custom_slack_app.go`, `start_discord_install.go`, `complete_discord_install.go`, `connect_custom_discord_bot.go`, `connect_telegram_bot.go`, `register_telegram_webhook.go`, `complete_whatsapp_embedded_signup.go`, `connect_custom_whatsapp_app.go`, `subscribe_whatsapp_webhooks.go`, `register_whatsapp_phone_number.go`.
- Rooms: `sync_channel_rooms.go`, `upsert_channel_room.go`, `update_channel_room.go`, `remove_channel_room.go`.
- Inbound: `store_inbound_event.go` (insert raw + job, InsertTx), `process_inbound_event.go` (dispatch to driver normalizer, then 03 actions; sets `InboundMessage.AfterPersist` so `record_contact_state.go` + `channel_message_refs` writes run inside 03 ingest tx), `route_discord_dm.go`, `record_contact_state.go`, `apply_channel_lifecycle_event.go` (uninstall, revoke, guild leave, channel archive, `my_chat_member`, Meta account updates).
- Members: `link_channel_member.go`, `unlink_channel_member.go`, `auto_link_slack_member.go`.
- WhatsApp: `refresh_whatsapp_checklist.go`, `sync_whatsapp_templates.go`, `create_whatsapp_template.go`, `delete_whatsapp_template.go`, `apply_whatsapp_template_status.go`, `rekey_whatsapp_user.go` (BSUID change), `handle_meta_deauthorize.go`, `handle_meta_data_deletion.go`.
- Lifecycle hooks: `apply_conversation_closed_marker.go` (Slack close reaction), `apply_conversation_reopened_marker.go`, `erase_channel_contact_data.go` (`EraseChannelContactData`, GDPR contact eraser per `00-foundation/contracts.md` "Contact eraser hooks"; registered on 03 eraser registry in `app_service_provider.go`).

### HTTP
Controllers `app/http/controllers/api/v1/`:
`channel_type_controller.go` (Index), `channel_controller.go` (Index, Show, Update, Destroy), `channel_reconnection_controller.go` (Store), `channel_credential_controller.go` (Store), `channel_health_controller.go` (Show), `channel_setup_controller.go` (Show), `channel_room_controller.go` (Index, Update), `channel_room_sync_controller.go` (Store), `channel_member_link_controller.go` (Index, Store, Destroy), `slack_channel_controller.go` (Store), `slack_installation_controller.go` (Store), `discord_channel_controller.go` (Store), `discord_installation_controller.go` (Store), `discord_guild_controller.go` (Index; custom bot guild picker), `telegram_channel_controller.go` (Store), `whatsapp_channel_controller.go` (Store), `whatsapp_embedded_signup_config_controller.go` (Show), `whatsapp_embedded_signup_controller.go` (Store), `whatsapp_checklist_controller.go` (Show; `?refresh=true` re-queries Meta, throttled), `whatsapp_template_controller.go` (Index, Store, Show, Destroy), `whatsapp_template_sync_controller.go` (Store), `conversation_channel_state_controller.go` (Show), `channel_inbound_event_controller.go` (Index), `channel_inbound_event_replay_controller.go` (Store). No action-named controllers (rest.md rule 14).

Controllers `app/http/controllers/web/`: `slack_oauth_callback_controller.go` (Invoke), `discord_oauth_callback_controller.go` (Invoke). Redirect to dashboard `/settings/channels/{chn}?connected=1` or `?error=<code>`.

Controllers `app/http/controllers/webhooks/`: `slack_event_controller.go` (shared + per-connection), `discord_interaction_controller.go` (shared + per-connection), `telegram_update_controller.go`, `whatsapp_webhook_controller.go` (Verify GET, Store POST; shared + per-connection), `meta_deauthorize_controller.go`, `meta_data_deletion_controller.go`.

Middleware (`app/http/middleware/`): `verify_slack_signature.go` (`slack.NewSecretsVerifier`), `verify_discord_signature.go` (disgo `httpserver` Ed25519), `verify_telegram_secret.go` (header compare, own 5 lines), `verify_meta_signature.go` (HMAC, own). All read raw body once (cap per provider), constant-time compare, stash raw bytes in context.

Requests (`app/http/requests/`): `update_channel_request.go`, `store_channel_credentials_request.go`, `update_channel_room_request.go`, `store_channel_member_link_request.go`, `store_slack_channel_request.go`, `store_slack_installation_request.go`, `store_discord_channel_request.go`, `store_discord_installation_request.go`, `store_telegram_channel_request.go`, `store_whatsapp_channel_request.go`, `store_whatsapp_embedded_signup_request.go`, `store_whatsapp_template_request.go`, `list_channels_request.go`.

Resources (`app/http/resources/`): `channel_resource.go` (+ compact), `channel_type_resource.go`, `channel_room_resource.go`, `channel_health_resource.go`, `channel_setup_resource.go`, `channel_member_link_resource.go`, `whatsapp_checklist_resource.go`, `whatsapp_template_resource.go`, `conversation_channel_state_resource.go`, `installation_url_resource.go`.

Policies: `app/policies/channel_connection_policy.go` (viewAny, view, viewSetup, create, update, delete, reconnect), `whatsapp_template_policy.go`.

Rules (`app/rules/`): `telegram_bot_token_rule.go` (`^\d{5,15}:[A-Za-z0-9_-]{30,64}$`), `slack_token_rule.go` (`xoxb-` / `xapp-` prefix), `whatsapp_template_name_rule.go`, `meta_locale_rule.go`.

### Jobs (`app/jobs/`)
`process_channel_inbound_event.go`, `download_channel_attachment.go` (only for deferred downloads; normal path streams inside processing), `sync_channel_rooms.go`, `check_channel_health.go`, `refresh_whatsapp_checklist.go`, `sync_whatsapp_templates.go`, `backfill_channel_room.go`, `prune_channel_inbound_events.go`, `refresh_channel_contact_profile.go`, `apply_channel_close_marker.go`, `register_telegram_webhook.go`, `release_held_inbound_events.go`. (`send_channel_message`, `send_channel_typing` are 03 jobs calling 05 drivers.)

### Listeners (`app/listeners/`)
`apply_channel_close_marker_listener.go` (conversation.closed, queued), `apply_channel_reopen_marker_listener.go` (conversation.reopened, queued), `mark_channel_messages_read.go` (conversation.read, queued; sole caller of 03 `ChannelReadMarker.MarkRead`: 03 defines interface, has no caller), `move_contact_channel_states.go` (contact.merged, queued; dedupe rows), `unlink_deactivated_member.go` (00-accounts `member.deactivated`, queued), `release_held_inbound_events_listener.go` (00-accounts `workspace.unsuspended` and `workspace.restored`), `rebuild_webhook_registrations.go` (11 `system.app_url_changed`, queued).

### Events (`app/events/`)
`channel_connected.go`, `channel_disconnected.go`, `channel_errored.go` (catalog), new: `channel_updated.go`, `whatsapp_template_status_changed.go`.

### MCP (`app/mcp/tools/`)
`list_channels_tool.go`, `get_channel_health_tool.go`, `connect_telegram_bot_tool.go`, `list_whatsapp_templates_tool.go`.

### Console (`app/console/commands/`)
`channels_add_command.go`, `channels_list_command.go`, `channels_manifest_command.go`, `channels_check_command.go`. Stubs: `resources/stubs/slack_manifest.yaml.tmpl`.

### Config
`config/channels.go` (behavior), `config/services.go` additions (provider app credentials).

### Database
- Migrations: `20261101000502_add_channel_lookup_indexes.sql`, `20261101000503_create_channel_rooms_table.sql`, `20261101000504_create_channel_contact_states_table.sql`, `20261101000505_create_channel_message_refs_table.sql`, `20261101000506_create_channel_inbound_events_table.sql`, `20261101000507_create_channel_socket_sessions_table.sql`, `20261101000508_create_channel_member_links_table.sql`, `20261101000509_create_whatsapp_templates_table.sql`, `20261101000510_add_rls_to_channel_tables.sql`.
- Queries: `database/queries/channel_connections.sql` (05 owns queries file; table core), `channel_rooms.sql`, `channel_contact_states.sql`, `channel_message_refs.sql`, `channel_inbound_events.sql`, `channel_socket_sessions.sql`, `channel_member_links.sql`, `whatsapp_templates.sql`.
- Factories: `channel_connection.go` additions `.Slack()`, `.Discord()`, `.Telegram()`, `.WhatsApp()`, `.Custom()`, `.Errored()`; `channel_room.go`, `channel_contact_state.go`, `whatsapp_template.go`, `channel_inbound_event.go`.

### Tests
- `tests/contracts/channel_test.go` (03 suite) run for 4 drivers with recorded HTTP fixtures (`tests/fixtures/channels/{provider}/*.json`) via `httpclienttest.New(a.HTTP)` (SDKs share the faked `StdClient`).
- `tests/unit/channels/` (domain driver zone, testing.md: pure parsing/mapping only, no mocks): `slack_format_test.go`, `discord_format_test.go`, `telegram_format_test.go`, `whatsapp_format_test.go` (golden files `tests/fixtures/channels/format/*.md`), `splitter_test.go`, `telegram_entities_test.go` (UTF-16 offsets, emoji, RTL), `signature_test.go` (Telegram + Meta own checks, known vectors), `classify_test.go`, `thread_key_test.go`, `errors_catalog_test.go`. Lease, buckets, dedupe = feature tests (real Postgres + Valkey).
- `tests/feature/channels/`: `channels_api_test.go`, `slack_install_test.go`, `slack_events_test.go`, `slack_custom_test.go`, `discord_install_test.go`, `discord_gateway_test.go` (fake gateway server), `discord_interactions_test.go`, `telegram_test.go`, `whatsapp_signup_test.go`, `whatsapp_webhook_test.go`, `whatsapp_window_test.go`, `whatsapp_templates_test.go`, `whatsapp_checklist_test.go`, `inbound_dedupe_test.go`, `socket_failover_test.go`, `lifecycle_test.go`, `gdpr_test.go`, `cross_workspace_test.go`.

## HTTP API

Base `/api/v1`. Auth `A` (`auth`: session, first-party app or token; abilities apply to tokens) + `workspace`, unless row says `S` (`auth:session`, browser only). Letters per api.md "Who calls what". Abilities from api.md (no new ability: channel management is settings). Members read compact channel info via `conversations:read` (needed for inbox filters and badges). "admin" in table = permission, not role (extensibility.md P2; policies call `authz.Can`): writes need `channels:manage` (ability group `settings:write`), full reads need `channels:inspect` (group `settings:read`); both held by owner, admin via 00-accounts `roles.go`.

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | /channel-types | channel-types.index | any member; `conversations:read` | types, available modes on this instance (shared app configured?), requirements |
| GET | /channels | channels.index | member `conversations:read` (compact) / admin `settings:read` (full) | list all connections incl. widget/email (read-only for those); `filter[type]` comma list; type `import` (16 importer connections) hidden unless `filter[type]=import` (read-only, managed by 16) |
| GET | /channels/{channel} | channels.show | same | one connection |
| PATCH | /channels/{channel} | channels.update | admin; `settings:write` | name, inbox, settings |
| DELETE | /channels/{channel} | channels.destroy | admin; `settings:write` | disconnect + soft delete (slack/discord/telegram/whatsapp only; widget/email 409 `channel_managed_elsewhere`) |
| POST | /channels/{channel}/reconnections | channels.reconnections.store | admin; `settings:write` | re-validate creds (custom, any caller); shared mode returns OAuth URL, so bearer caller gets 403 `session_required` (callback needs browser session) |
| POST | /channels/{channel}/credentials | channels.credentials.store | admin; `settings:write` | set/rotate custom creds (bot token, signing secret, app token, access token, app secret) |
| GET | /channels/{channel}/health | channels.health.show | admin; `settings:read` | live checks (cached 60 s in Valkey `cache.Store`; `?fresh=1` admin forces) |
| GET | /channels/{channel}/setup | channels.setup.show | admin; `settings:read` | manifest, webhook URL, verify token, invite URL, pending steps |
| GET | /channels/{channel}/rooms | channels.rooms.index | admin; `settings:read` | watched Slack rooms + available (`filter[status]`) |
| PATCH | /channels/{channel}/rooms/{room} | channels.rooms.update | admin; `settings:write` | status active/paused, mode, settings |
| POST | /channels/{channel}/room-syncs | channels.room-syncs.store | admin; `settings:write` | refresh room list from provider (async job, 202 `{object: "channel_room_sync", status}`) |
| GET | /channels/{channel}/member-links | channels.member-links.index | admin; `settings:read` | |
| POST | /channels/{channel}/member-links | channels.member-links.store | admin; `settings:write` | link provider user to member |
| DELETE | /channels/{channel}/member-links/{link} | channels.member-links.destroy | admin; `settings:write` | |
| POST | /channels/slack/installations | channels.slack.installations.store | admin; `S` (returns browser URL; callback needs session cookie) | returns Slack OAuth `authorize_url` (shared app) |
| POST | /channels/slack | channels.slack.store | admin; `settings:write` | custom app: create pending connection, or full connect when tokens given |
| POST | /channels/discord/installations | channels.discord.installations.store | admin; `S` (returns browser URL; callback needs session cookie) | returns Discord bot invite `authorize_url` |
| POST | /channels/discord | channels.discord.store | admin; `settings:write` | custom bot |
| GET | /channels/discord/guilds | channels.discord.guilds.index | admin; `settings:write` | custom bot guild picker for pending connection (`?channel=chn_...`); token never in query string |
| POST | /channels/telegram | channels.telegram.store | admin; `settings:write` | connect bot by token |
| GET | /channels/whatsapp/embedded-signup-config | channels.whatsapp.embedded-signup-config.show | admin; `S` (FB SDK popup runs in dashboard browser) | FB SDK config: `app_id`, `config_id`, `graph_version`, `extras` |
| POST | /channels/whatsapp/embedded-signups | channels.whatsapp.embedded-signups.store | admin; `S` (code comes from FB SDK popup in dashboard browser) | finish Embedded Signup (code, waba_id, phone_number_id) |
| POST | /channels/whatsapp | channels.whatsapp.store | admin; `settings:write` | custom Meta app |
| GET | /channels/{channel}/whatsapp-checklist | channels.whatsapp-checklist.show | admin; `settings:read` (`?refresh=true` needs `settings:write`) | verification checklist; `?refresh=true` re-queries Meta now (throttled 1/min per connection, 429 `checklist_refresh_throttled`) |
| GET | /channels/{channel}/whatsapp-templates | channels.whatsapp-templates.index | member `conversations:read` | approved templates for composer (`filter[status]`, `filter[language]`, `q`) |
| GET | /channels/{channel}/whatsapp-templates/{template} | channels.whatsapp-templates.show | member `conversations:read` | |
| POST | /channels/{channel}/whatsapp-templates | channels.whatsapp-templates.store | admin; `settings:write` | create text template, submit to Meta |
| DELETE | /channels/{channel}/whatsapp-templates/{template} | channels.whatsapp-templates.destroy | admin; `settings:write` | delete in Meta (all languages of name: 409 unless `?all_languages=1`) |
| POST | /channels/{channel}/whatsapp-template-syncs | channels.whatsapp-template-syncs.store | admin; `settings:write` | pull from Meta (202) |
| GET | /channels/{channel}/inbound-events | channels.inbound-events.index | admin; `settings:read` | debug list (`filter[status]=failed,held`), payload omitted unless `include=payload` |
| POST | /channels/{channel}/inbound-events/{event}/replays | channels.inbound-events.replays.store | admin; `settings:write` | re-run `failed` event (409 `event_not_replayable` otherwise) |
| GET | /conversations/{conversation}/channel-state | conversations.channel-state.show | member `conversations:read` | routed channel, capabilities, window open/closes_at, reachable, template required |

Static segments (`slack`, `discord`, `telegram`, `whatsapp`) registered before `{channel}`; `{channel}` only matches `chn_` ids (wrong prefix 404).

Web routes (`routes/web.go`): `GET /channels/slack/callback` (`channels.slack.callback`), `GET /channels/discord/callback` (`channels.discord.callback`). `auth:session` required (why install/signup routes are `S`: bearer callers have no cookie for callback); state must match session user + workspace.

Webhook routes (`routes/webhooks.go`, no session, no CSRF, `throttle:webhooks` 3,000/min per connection; shared paths also `throttle:webhooks_ip` 600/min per IP for failed signatures, edge 160):

| Method | Path | Name | Verify |
|---|---|---|---|
| POST | /webhooks/slack/events | webhooks.slack.events | shared signing secret (`services.slack.signing_secret`) |
| POST | /webhooks/slack/events/{channel} | webhooks.slack.connection-events | per-connection signing secret |
| POST | /webhooks/discord/interactions | webhooks.discord.interactions | Ed25519 shared public key |
| POST | /webhooks/discord/interactions/{channel} | webhooks.discord.connection-interactions | per-connection public key |
| POST | /webhooks/telegram/updates/{channel} | webhooks.telegram.updates | `X-Telegram-Bot-Api-Secret-Token` equals connection secret |
| GET | /webhooks/whatsapp/events | webhooks.whatsapp.verify | `hub.verify_token` equals `services.whatsapp.verify_token` |
| POST | /webhooks/whatsapp/events | webhooks.whatsapp.store | `X-Hub-Signature-256` HMAC-SHA256(app secret, raw body) |
| GET | /webhooks/whatsapp/events/{channel} | webhooks.whatsapp.connection-verify | per-connection verify token |
| POST | /webhooks/whatsapp/events/{channel} | webhooks.whatsapp.connection-store | per-connection app secret |
| POST | /webhooks/meta/deauthorize | webhooks.meta.deauthorize | `signed_request` HMAC (app secret) |
| POST | /webhooks/meta/data-deletion | webhooks.meta.data-deletion | `signed_request`; returns `{url, confirmation_code}` |

### Key request/response examples

`GET /api/v1/channel-types`
```json
{
  "data": [
    { "object": "channel_type", "type": "slack", "modes": ["shared", "custom"], "shared_available": true,
      "inbound_transports": ["webhook", "socket"], "public_url_required": false,
      "docs_url": "https://trydimmi.com/docs/channels/slack" },
    { "object": "channel_type", "type": "telegram", "modes": ["custom"], "shared_available": false,
      "inbound_transports": ["webhook", "polling"], "public_url_required": false },
    { "object": "channel_type", "type": "whatsapp", "modes": ["shared", "custom"], "shared_available": true,
      "inbound_transports": ["webhook"], "public_url_required": true }
  ]
}
```
`shared_available` true only when instance has provider app credentials configured (cloud always; self-host when admin set `services.*`).

`ChannelResource` (admin full form):
```json
{
  "data": {
    "id": "chn_01jbd2k...", "object": "channel", "type": "slack", "name": "Acme Slack",
    "status": "active", "mode": "shared", "inbound_transport": "webhook",
    "inbox": { "id": "inbox_01jb...", "object": "inbox", "name": "Support" },
    "external_id": "A07Q1:T024BE", "external_name": "Acme Corp",
    "bot": { "user_id": "U07BOT", "name": "dimmi" },
    "capabilities": { "transport": "push", "start_conversation": "never", "max_body_chars": 12000,
      "attachments": true, "max_attachments": 10, "max_attachment_bytes": 26214400,
      "typing": false, "delivery_receipts": false,
      "read_receipts": false, "messaging_window_seconds": 0 },
    "settings": { "send_read_receipts": false, "allow_bot_authors": false, "ignore_teammate_top_level": true },
    "rooms_count": 3,
    "setup": { "complete": true, "pending_steps": [], "test_link": "https://app.slack.com/client/T024BE/C07SUP" },
    "health": { "status": "ok", "checked_at": "2026-11-01T10:00:00Z" },
    "last_error": null, "last_error_code": null, "last_error_at": null,
    "last_activity_at": "2026-11-01T09:58:12Z",
    "created_at": "2026-10-30T12:00:00Z", "updated_at": "2026-11-01T10:00:00Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/channels/chn_01jbd2k...",
      "setup": "https://app.trydimmi.com/api/v1/channels/chn_01jbd2k.../setup" }
  }
}
```
Compact form (member): `id, object, type, name, status, inbox, capabilities, last_error` only if member can see (yes: errors visible to all so teammates know why replies fail; `last_error` never contains secrets).

`POST /api/v1/channels/telegram`
```json
{ "bot_token": "7012345678:AAH...", "inbox_id": "inbox_01jb...", "name": "Acme bot", "inbound_transport": "auto" }
```
201:
```json
{ "data": { "id": "chn_01jbe...", "object": "channel", "type": "telegram", "status": "active",
  "external_id": "7012345678", "external_name": "@acme_support_bot", "inbound_transport": "webhook",
  "setup": { "complete": true, "test_link": "https://t.me/acme_support_bot" } } }
```
Errors: 422 `bot_token` invalid format; 422 `telegram_token_rejected` (getMe 401); 409 `channel_already_connected` (same bot connected in this workspace, returns existing id) or 409 `channel_connected_elsewhere` (bot connected in other workspace: never reveal which); 424 `telegram_webhook_failed` with Telegram description (connection kept `error`, retry via reconnect).

`POST /api/v1/channels/slack/installations`
```json
{ "inbox_id": "inbox_01jb...", "redirect_to": "/settings/channels" }
```
200:
```json
{ "data": { "object": "installation_url",
  "authorize_url": "https://slack.com/oauth/v2/authorize?client_id=...&scope=...&redirect_uri=https%3A%2F%2Fapp.trydimmi.com%2Foauth%2Fslack%2Fcallback&state=...",
  "expires_at": "2026-11-01T10:10:00Z" } }
```
State: signed (app key HMAC), contains workspace id, member id, inbox id, nonce, expiry 10 min; nonce single-use (Valkey `SET NX EX 600` `slack:oauth_state:{nonce}`). Callback errors redirect with `?error=slack_access_denied|state_invalid|enterprise_install_unsupported|channel_connected_elsewhere`.

`POST /api/v1/channels/slack` (custom app, two-step)
Step 1 (no tokens): `{ "name": "Acme Slack", "inbox_id": "inbox_...", "inbound_transport": "webhook" }` returns 201 `status: pending` + `setup.manifest` (YAML), `setup.create_app_url` (`https://api.slack.com/apps?new_app=1&manifest_json=<urlencoded>`), `setup.request_url`.
Step 2: `POST /channels/{chn}/credentials` `{ "bot_token": "xoxb-...", "signing_secret": "...", "app_token": "xapp-..." }` validates via `auth.test` (+ `apps.connections.open` for socket), sets `external_id`, status `active`. One-step allowed: send tokens in first POST.

`POST /api/v1/channels/whatsapp/embedded-signups`
```json
{ "code": "AQB...", "waba_id": "1023...", "phone_number_id": "1065...", "inbox_id": "inbox_...", "pin": "482913" }
```
`pin` optional: 6 digits for two-step verification on register; generated and stored encrypted when absent.
201: connection `status: pending` until phone registered + webhooks subscribed (usually same request) then `active`; checklist included. Errors: 422 `whatsapp_code_invalid` (exchange failed, code single use, about 10 min life), 409 `channel_connected_elsewhere`, 424 `whatsapp_register_failed` (Meta error text + code), 429 `whatsapp_onboarding_limit` (Tech Provider onboarding cap 10 per rolling 7 days until Dimmi verified; cloud only).

`GET /api/v1/channels/{channel}/whatsapp-checklist`
```json
{
  "data": {
    "object": "whatsapp_checklist", "channel_id": "chn_01jbf...", "overall": "action_required",
    "checked_at": "2026-11-01T09:00:00Z",
    "steps": [
      { "key": "business_portfolio", "status": "complete", "detail": "Acme Inc (portfolio 3341...)", "help_url": "https://business.facebook.com/settings" },
      { "key": "business_verification", "status": "pending", "detail": "Meta is reviewing documents. Usually 1 to 5 business days.", "help_url": "https://business.facebook.com/settings/security" },
      { "key": "display_name", "status": "complete", "detail": "Acme Support (approved)" },
      { "key": "phone_registered", "status": "complete", "detail": "+49 30 1234567" },
      { "key": "webhooks_subscribed", "status": "complete" },
      { "key": "payment_method", "status": "unknown", "detail": "Add payment method in WhatsApp Manager before messaging limits apply to paid messages.", "help_url": "https://business.facebook.com/wa/manage/home/" },
      { "key": "messaging_limit", "status": "complete", "detail": "TIER_250: 250 new customers per 24 h outside open windows. Verify business to reach 2,000." },
      { "key": "quality_rating", "status": "complete", "detail": "GREEN" },
      { "key": "first_message_received", "status": "pending", "detail": "Send a WhatsApp message to +49 30 1234567 to test.", "help_url": "https://wa.me/49301234567" }
    ]
  }
}
```
Step status enum: `complete`, `pending` (waiting on Meta), `action_required` (user must act), `unknown` (Meta does not expose), `not_applicable`. `overall`: worst of required steps (`business_verification` is not required to message; required: phone_registered, webhooks_subscribed, display_name not declined).

`GET /api/v1/conversations/{conversation}/channel-state`
```json
{ "data": { "object": "conversation_channel_state", "channel": { "id": "chn_...", "type": "whatsapp", "status": "active" },
  "recipient": { "identity_type": "whatsapp_bsuid", "reachable": true },
  "window": { "applies": true, "open": false, "closed_at": "2026-10-31T08:12:00Z" },
  "public_room": false,
  "can_send_freeform": false, "requires_template": true,
  "templates_url": "https://app.trydimmi.com/api/v1/channels/chn_.../whatsapp-templates?filter[status]=approved",
  "capabilities": { "...": "same as channel" } } }
```
Template send uses 03 reply endpoint with `channel_options`:
```json
POST /api/v1/conversations/conv_.../replies
{ "body": "", "channel_options": { "template": { "id": "wtp_01jb...", "parameters": { "body": ["Bob", "#1042"], "header": [] } } } }
```
Driver validates template approved, language, parameter count; message `body` stored as rendered template text (03 stores what driver renders: driver exposes `whatsapp.RenderTemplatePreview` used by 03 via `Options` validation hook; if 03 cannot call, body stored as given and `metadata.whatsapp.template` keeps name/params). Errors: 422 `template_not_approved`, 422 `template_parameters_mismatch` (expected N), 404 template from other connection.

### Error codes (05)
`session_required` 403 (bearer on `S` install/signup routes; same code as 06, 12), `entitlement_limit_reached` 422 (`key`, `limit`; custom Discord bot cap, entitlement `channels.discord.custom_bots`), `channel_already_connected` 409, `channel_connected_elsewhere` 409, `channel_managed_elsewhere` 409, `channel_not_pending` 409, `shared_app_not_configured` 409, `state_invalid` 400, `enterprise_install_unsupported` 422, `telegram_token_rejected` 422, `telegram_webhook_failed` 424, `slack_token_rejected` 422, `slack_scopes_missing` 422 (lists missing scopes), `slack_socket_mode_disabled` 422, `discord_token_rejected` 422, `discord_guild_required` 422 (bot in several guilds; response `errors.guild_id` + `meta.guilds`), `whatsapp_code_invalid` 422, `whatsapp_register_failed` 424, `whatsapp_onboarding_limit` 429, `whatsapp_token_scopes_missing` 422, `template_not_approved` 422, `template_parameters_mismatch` 422, `template_name_taken` 409, `meta_rate_limited` 429 (`Retry-After`), `provider_unavailable` 503 (provider 5xx during connect; `Retry-After: 30`), `checklist_refresh_throttled` 429. 424 Failed Dependency used for provider-side failures after our validation passed (not in conventions list: documented addition).

## MCP tools

| Tool | Ability | Notes |
|---|---|---|
| `dimmi_list_channels` | conversations:read | compact list; `readOnlyHint` |
| `dimmi_get_channel_health` | settings:read | runs cached health; returns failing checks with fix text |
| `dimmi_connect_telegram_bot` | settings:write | input `bot_token`, `inbox_id?`; description warns token enters agent context, suggests `dimmi channels:add telegram` for secrecy; channel connect goes through 09 `app/mcp/confirm` gate (elicitation, then `confirm` fallback; no own confirm scheme); `destructiveHint: false`, `idempotentHint: true` |
| `dimmi_list_whatsapp_templates` | conversations:read | approved templates + parameter counts so agent can send via core `dimmi_send_reply` (09) with `channel_options` |

Tools registered in 09 toolset `channels` (`routes/mcp.go`), not in default core list; toolset under 2,000 tokens. Tool outputs carrying provider names/messages wrapped in 09 untrusted envelope.

## CLI commands (05 owns; framework, auth, output modes from 09)

All wrap REST (work against cloud and self-host). `--json`, `--workspace`, non-interactive with flags.

| Command | Does |
|---|---|
| `dimmi channels:add telegram [--token T] [--inbox I] [--polling]` | prompt token (hidden input), POST, print bot link |
| `dimmi channels:add slack [--shared] [--socket] [--inbox I]` | shared: open dashboard channel settings URL in browser (CLI token cannot call `S` route `installations`; browser session starts install), wait for connection via `GET /channels` (poll 2 s, 10 min). Custom (default when instance has no shared app): create pending connection, print manifest + create link, open browser, prompt bot token + signing secret (+ app token when `--socket`), validate, print rooms hint "invite @dimmi to a channel" |
| `dimmi channels:add discord [--shared] [--inbox I]` | shared: same browser hand-off as slack. Custom: print portal steps (create app, bot; no privileged intent), prompt token, list guilds or print invite URL, wait for guild join |
| `dimmi channels:add whatsapp [--inbox I]` | cloud: print dashboard URL (Embedded Signup needs browser + FB SDK). Custom: prompt WABA id, phone number id, system user token, app secret; print callback URL + verify token; verify subscription; print checklist |
| `dimmi channels:list` | table: id, type, name, status, last error |
| `dimmi channels:manifest slack [--socket]` | print manifest only (YAML) for pending connection or generic |
| `dimmi channels:check [chn_...]` | health checks, exit 1 on failure (CI friendly) |
| `dimmi channels:replay <event_id>` | admin: re-run failed inbound event (`failed` status only) |

`dimmi doctor` (11) calls `channels:check` logic for every connection.

## Events

### Emitted (catalog)
| Event | When | Payload extras |
|---|---|---|
| channel.connected | connection reaches `active` first time, and on recovery from `error` | `reason`: `connected` / `recovered` / `reconnected` |
| channel.errored | transition to `error` only (not repeated) | `error_code`, `error` (human), `since` |
| channel.disconnected | transition to `disconnected` (uninstall, revoke, guild removed, admin delete) | `reason`: `uninstalled`, `token_revoked`, `guild_removed`, `deleted_by_member`, `meta_deauthorized`, `account_deleted` |
| conversation.created, message.created, message.delivery_updated, contact.created, contact.updated, contact.merged | via 03 actions | |

### New events (add to catalog in same PR)
| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| channel.updated | yes (`workspace:{ws}`) | no | name, inbox, settings, rooms added/removed/paused, checklist change, health change. `changes` list |
| whatsapp_template.status_changed | yes | yes | `{template_id, name, language, from, to, reason}`; drives composer refresh and alert on rejection/pause |

### Consumed
| Event | Listener | Effect |
|---|---|---|
| conversation.closed | `ApplyChannelCloseMarkerListener` (queued `channels`) | Slack: add `close_marker` reaction (default `white_check_mark`) on thread root when room setting on; Discord/Telegram/WhatsApp: nothing |
| conversation.reopened | `ApplyChannelReopenMarkerListener` | remove Slack reaction |
| conversation.read | `MarkChannelMessagesRead` | WhatsApp: mark latest inbound message read (blue ticks) when connection `send_read_receipts` true; one call per conversation per 30 s |
| contact.merged | `MoveContactChannelStates` | identities moved by 03; states follow identity id (no-op) except duplicate (same connection) rows: keep newest `last_inbound_at` |
| member.deactivated (00-accounts) | `UnlinkDeactivatedMember` | delete member links; user's later messages in `community` rooms classified contact |
| workspace.unsuspended, workspace.restored (00-accounts) | `ReleaseHeldInboundEventsListener` | enqueue `release_held_inbound_events` |
| system.app_url_changed (11) | `RebuildWebhookRegistrations` | re-run Telegram `setWebhook`, print new Slack/Meta URLs in `channel.updated` setup pending steps (provider-side URLs for custom apps need manual update; status stays active, pending step added) |

06 consumes `channel.errored` and `channel.disconnected` to notify owners/admins (06 `system_alert` sources `channel.errored`, `channel.disconnected`; owners/admins, in-app + email, deduped once per connection per 24 h). `channel.connected` reason `recovered` resolves it (06 marks alert resolved, no new notification). 10 delivers webhooks.

Imported messages (16, `metadata.imported`) never reach 05 drivers (03 `SendChannelOutbound` skips); drivers return `ErrPermanent` `imported_message` if one does (defense, same as 04).

## Jobs

| Job (kind) | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| `process_channel_inbound_event` | `channels` | `StoreInboundEvent` InsertTx | 10, backoff 1 s to 10 min; final failure: event `failed`, metric + log, never blocks other events | `ByArgs` (event_id). Per-thread ordering: worker holds `pg_advisory_xact_lock(hashtext(connection_id || serial_key))` |
| `backfill_channel_room` | `channels` | gateway re-identify after gap (Discord: active DMs), Socket Mode reconnect after gap over 60 s, health job sees `last_seen_event_at` gap, webhook outage detection | 5 | `ByArgs` (room_id) + `ByPeriod` 5 min |
| `sync_channel_rooms` | `channels` | connect, `POST room-syncs`, daily | 5 | `ByArgs` (connection_id) + `ByPeriod` 1 min |
| `check_channel_health` | `maintenance` | periodic 15 min per active/error connection (fan-out batch job), on demand | 2 | `ByArgs` (connection_id) + `ByPeriod` 5 min |
| `refresh_whatsapp_checklist` | `channels` | connect, Meta `account_update` / `phone_number_name_update` / `phone_number_quality_update` / `account_review_update` / `business_capability_update` webhooks, periodic 6 h | 5 | `ByArgs` (connection_id) + `ByPeriod` 1 min |
| `sync_whatsapp_templates` | `channels` | connect, `POST whatsapp-template-syncs`, daily, `message_template_status_update` for unknown template | 5 | `ByArgs` (connection_id) + `ByPeriod` 1 min |
| `refresh_channel_contact_profile` | `channels` | first sight of identity, then lazily when older than 24 h on inbound | 3 | `ByArgs` (state_id) + `ByPeriod` 24 h |
| `apply_channel_close_marker` | `channels` | close/reopen listeners | 5 | `ByArgs` (conversation_id, op) |
| `register_telegram_webhook` | `channels` | connect, reconnect, `app.url` change, health detects `getWebhookInfo.url` mismatch | 8 | `ByArgs` (connection_id) |
| `release_held_inbound_events` | `channels` | unsuspend | 3 | `ByArgs` (workspace_id) |
| `prune_channel_inbound_events` | `maintenance` | daily 04:00 UTC | 3 | `ByPeriod` 24 h; deletes 5,000 per batch; per workspace batch calls `RetentionGuard.MayDestroy(ws, {workspace, ws})`, then per event with linked conversation `{conversation, id}` (false = row kept, `retention_held` metric; noop driver at MVP) |
| `download_channel_attachment` | `channels` | attachment fetch failed transiently inside processing (message already stored with placeholder) | 6 | `ByArgs` (message_id, provider_file_id) |

Not jobs: `SocketSupervisor` (long-running goroutine in `worker` role): every 5 s lists required sockets (Discord shards for shared bot + each custom bot application, DM events only; Slack socket-mode connections; Telegram polling bots), acquires free leases (`UPDATE channel_socket_sessions SET owner=$me, lease_expires_at=now()+15s WHERE kind=$1 AND key=$2 AND (owner IS NULL OR lease_expires_at < now()) RETURNING *`), runs connection, renews every 5 s, drops connection immediately when renew fails. Fair share: replica takes at most `ceil(total / live_replicas) + 1` leases (live replicas from node heartbeat, `app/support/nodes/heartbeat.go`). River leader election not used: River elector is single global leader, we need per-shard ownership spread across replicas.

## Config

`config/services.go` (secrets via env or `_FILE`):

| Key | Env | Default | Notes |
|---|---|---|---|
| `services.slack.client_id` / `client_secret` / `signing_secret` / `app_id` | `DIMMI_SLACK_CLIENT_ID` ... | empty | shared app; empty = shared mode unavailable |
| `services.discord.application_id` / `public_key` / `bot_token` / `client_secret` | `DIMMI_DISCORD_*` | empty | shared bot |
| `services.whatsapp.app_id` / `app_secret` / `config_id` / `verify_token` / `system_user_token` | `DIMMI_WHATSAPP_*` | empty | Meta Tech Provider app; `config_id` = Embedded Signup configuration (v4) |
| `services.whatsapp.graph_version` | `DIMMI_WHATSAPP_GRAPH_VERSION` | `v26.0` | pinned; v26.0 released 2026-07-29; bumped by release PR, contract fixtures re-recorded |
| `services.telegram.api_base_url` | `DIMMI_TELEGRAM_API_BASE_URL` | `https://api.telegram.org` | self-host may point to local Bot API server (`http_client.allow_hosts` entry needed) |

`config/channels.go`:

| Key | Env | Default | Notes |
|---|---|---|---|
| `channels.enabled` | `DIMMI_CHANNELS_ENABLED` | `slack,discord,telegram,whatsapp` | disable types per instance |
| `channels.inbound.max_payload_bytes` | | 1 MB (WhatsApp 3.5 MB: Meta payloads up to 3 MB) | 413 above |
| `channels.inbound.retention_days` | | 7 (failed/held 30) | raw event prune |
| `channels.inbound.dedupe_windows` | | `slack: 1h, discord: 1h, telegram: 24h, whatsapp: 168h` | Valkey `SET NX EX` fast-path dedupe before insert (providers.md pipeline); durable unique index stays backstop |
| `channels.socket.lease_ttl` | | 15s | |
| `channels.socket.renew_interval` | | 5s | |
| `channels.telegram.default_transport` | | `auto` | `auto` = webhook when `app.url` is public HTTPS on port 443/80/88/8443, else polling |
| `channels.slack.default_transport` | | `webhook` | custom app may pick `socket` |
| `channels.outbound.max_parts` | | 4 | split cap; feeds `MaxBodyChars` |
| `channels.outbound.oversize_attachments` | | `link` | `link` (signed URL 7 days) or `reject` (422 at request) |
| `channels.attachments.inbound_max_bytes` | | `filesystems.max_upload_mb` (25 MB) | larger inbound files: placeholder note in body `[file.pdf, 40 MB, too large to import]` |
| `channels.discord.intents` | | 4097 | GUILDS, DIRECT_MESSAGES (no privileged intent: bot DM content needs no MESSAGE_CONTENT) |
| `channels.discord.permissions` | | 0 | DMs need no guild permissions; bot joins server so members can DM it |
| `channels.discord.invalid_request_budget` | | 5,000 per 10 min | circuit breaker trips at half of Discord's 10,000 ban threshold |
| `channels.whatsapp.read_receipts_default` | | true | new connections `send_read_receipts` |
| `channels.slack.group_window_seconds_default` | | 120 | consecutive top-level messages join open conversation |
| `channels.health.interval` | | 15m | |

## Permissions matrix

Policies call `authz.Can(principal, permission)`; role columns derived from `roles.go`, never checked in 05 code. 05 registers `channels:manage` (group `settings:write`) and `channels:inspect` (group `settings:read`) in 00-accounts permission catalog; owner, admin.

| Action | Owner | Admin | Member | Permission (token ability group) |
|---|---|---|---|---|
| List/show channels (compact) | yes | yes | yes | `conversations:read` |
| Show full channel (settings, setup, health, rooms, member links, checklist) | yes | yes | no | `channels:inspect` (`settings:read`) |
| Connect any channel (OAuth start / custom POST / embedded signup) | yes | yes | no | `channels:manage` (`settings:write`) (OAuth start + Embedded Signup `S` only: browser flows, callback/popup need browser session) |
| Update, rotate credentials, reconnect, delete | yes | yes | no | `channels:manage` (`settings:write`) |
| Rooms sync/update, member links | yes | yes | no | `channels:manage` (`settings:write`) |
| Link own provider account (Slack auto by email; manual self-link) | yes | yes | yes (self only via `member_id` = self) | `conversations:write` for self |
| List/show WhatsApp templates | yes | yes | yes | `conversations:read` |
| Create/delete/sync templates | yes | yes | no | `channels:manage` (`settings:write`) |
| Send template message (via 03 reply) | yes | yes | yes | `conversations:write` |
| Conversation channel state | yes | yes | yes | `conversations:read` |

Cloud: connect endpoints require `verified` middleware (00-accounts: email verified before channels go live). All connect/disconnect/credential changes audit logged (WriteAuditLog, credentials redacted, only key names recorded).

## Performance budgets

| Metric | Budget |
|---|---|
| Webhook controller (verify + insert raw + enqueue) | p99 under 150 ms, p50 under 20 ms; never calls provider APIs |
| Inbound processing (job pickup to `message.created` commit), text message | p95 under 400 ms; with 5 MB attachment download under 3 s |
| Provider event to teammate screen | p95 under 2 s |
| Outbound driver `Send` (text) | p95 under 800 ms provider round trip excluded rate-limit waits |
| Gateway / socket failover | under 30 s (lease TTL 15 s + connect + resume) |
| Discord resume success after failover within 60 s | over 95% (else backfill) |
| RSS per active socket | under 4 MB; zstd-stream transport compression for Discord gateway |
| `GET /channels` (50 connections) | p95 under 30 ms (no provider calls; health from cache) |
| Health check | max 10 s total, 4 s per provider call |

## Observability

Metrics (Prometheus):
- `dimmi_channel_inbound_events_total{provider,transport,kind,result}` (stored, duplicate, rejected_signature, unroutable, held)
- `dimmi_channel_inbound_process_seconds{provider}` histogram; `dimmi_channel_inbound_backlog{provider}` (pending count, oldest age)
- `dimmi_channel_outbound_total{provider,result,code}` (03 has generic; 05 adds provider code label, cardinality capped to mapped codes + `other`)
- `dimmi_channel_rate_limited_total{provider,scope}` (scope: `route`, `global`, `shared`, `pair`, `chat`)
- `dimmi_channel_socket_connected{kind}` gauge, `dimmi_channel_socket_reconnects_total{kind,reason}`, `dimmi_channel_gateway_resume_total{result}`
- `dimmi_channel_signature_failures_total{provider}` (alert on spike)
- `dimmi_channel_connection_status{type,status}` gauge
- `dimmi_whatsapp_window_closed_total`, `dimmi_whatsapp_pricing_messages_total{category,billable}`
- `dimmi_channel_events_ignored_total{provider,kind}` (kind `reaction`, `edit`, `delete`, `teammate`, `group`)

Logs: every inbound event logs `provider`, `connection_id`, `dedupe_key`, `kind`, result; bodies never at info. Provider errors log code + request id (`x-slack-req-id`, Discord `X-RateLimit-Bucket`, Meta `fbtrace_id`). Tracing spans: `channels.inbound.<provider>`, `channels.send.<provider>`.

## Security and privacy

- Signature verification before parse, constant-time, raw body bytes (providers.md lists algorithm per provider). Slack timestamp window 300 s. Discord Ed25519 over `timestamp + body`. Telegram secret: 64 chars `[A-Za-z0-9_-]` random per connection, rotated on reconnect. Meta: `sha256=` + hex HMAC-SHA256(app secret, raw body).
- Failed signature: 401, no body echo, metric; never stored. Unroutable valid event (connection deleted): 200 (stop provider retries), dropped, debug log.
- Credentials: `channel_connections.credentials_encrypted` (envelope, workspace data key). Shared app secrets only in config. Redaction patterns added to log middleware: `xoxb-`, `xoxp-`, `xapp-`, `xoxe.`, Telegram `\d{5,15}:[A-Za-z0-9_-]{30,64}`, Meta `EAA[A-Za-z0-9]{20,}`, Discord bot token pattern `[MN][A-Za-z\d]{23,}\.[\w-]{6}\.[\w-]{27,}`.
- Outbound to provider APIs via `framework/httpclient` (`a.HTTP`; SDKs get `a.HTTP.StdClient()`); attachment download hosts allowlisted: `files.slack.com`, `slack-files.com`, `cdn.discordapp.com`, `media.discordapp.net`, `api.telegram.org` (or configured base), `lookaside.fbsbx.com`, `*.fbcdn.net`, `graph.facebook.com`. URL in payload pointing elsewhere: refused (payload-driven SSRF blocked).
- Mentions: outbound Discord `allowed_mentions: {"parse": []}`; Slack text `<!channel>`, `<!here>`, `<!everyone>`, `<@U...>` and `<#C...>` sequences escaped (`&lt;`) unless produced by our renderer for teammate-linked mention; Telegram HTML escaping; WhatsApp none needed.
- Inbound content untrusted: Markdown stored, rendered by 03/frontend safe renderer; Slack/Discord/Telegram links converted to Markdown links only for http/https/mailto/tel.
- Prompt injection: chat messages, provider display names, Slack/Discord channel names flow to 07 as untrusted user content (07 rules); never placed in system prompt.
- Public rooms: conversations in places other people can read (Slack channel incl. Slack Connect) carry `messages.metadata.<type>.public_room = true` on inbound and `conversations.source.public_room = true`. Contract for 07 (dependency): in public rooms AI never reveals contact custom attributes, email, phone, other conversations, or verified-user data; answers from knowledge only. DMs (Slack, Discord), Telegram private chats, WhatsApp: `public_room = false`. Teammates see "Public thread" badge in composer (frontend reads `channel-state.public_room`).
- Tenancy: routing only by verified `(type, external_id)` / path connection id + verified signature; foreign `chn_` in path with valid other-connection signature impossible (secret per connection).
- Privacy: erasing contact in Dimmi does not delete copies in Slack/Discord/Telegram/WhatsApp (UI copy states). Raw inbound payloads pruned 7 days. Profile cache max 4 KB.
- Contact eraser `EraseChannelContactData` (`app/actions/channels/erase_channel_contact_data.go`, `Erase(ctx, workspaceID, contactID) error`, called by 03 `EraseContact` job): deletes `channel_contact_states` (provider profile, window, DM route), `channel_message_refs` of contact's messages, `channel_inbound_events` payloads matching contact's provider user ids (JSON containment, batched), Discord DM route cache entries (`dm_route` lives in state profile); collects provider user ids before 03 deletes identities (eraser runs before identity wipe or reads tombstone list). Keeps `channel_member_links` (members, not contact). Idempotent.
- Meta compliance (cloud): deauthorize callback marks all WhatsApp connections of that business disconnected; data deletion callback deletes stored Meta user data (token, profile cache) and returns status URL + confirmation code (Meta platform requirement for apps using Facebook Login for Business).
- Discord: bot token reset by owner yields 401 on gateway identify (close code 4004 "authentication failed"): connection `error` `discord_token_invalid`; identify never retried in loop (protects 1,000/day identify budget).

## Metering impact (pricing.md section 2)

| Rule | 05 behavior |
|---|---|
| D1 activation | chat messages flow through 03 actions; teammate native replies not ingested (teammate echo: later), so only Dimmi-sent replies activate |
| D2 substantive | bot/automation messages from other bots ignored before ingest (never stored); Telegram `/start` with no text stored as `AutoGenerated` (non-substantive) |
| D5 channels | same rule for all four. **Meta fees**: MVP cloud uses Tech Provider model: customer adds own payment method in WhatsApp Manager, Meta bills customer directly, Dimmi passes through nothing and adds no margin. 05 records Meta `pricing` object from status webhooks in `messages.metadata.whatsapp.pricing` (`billable`, `category`, `type`) and metric, so UI can show "Meta: 1 paid service message" per conversation. Self-host: customer's own Meta app, same. Metadata informational only; 12 reads nothing from it, no Meta line items on Dimmi invoice |
| D7 outbound | WhatsApp template to new contact: not billable until contact replies and workspace answers |
| D9 spam/bots | other bots' messages never ingested; blocked contacts per 03 |
| D10 test | messages from linked members (teammate writes to own bot) flagged member-as-contact by 03 email rule only when email known; Telegram/Discord teammates testing their own bot: messages from users in `channel_member_links` create conversations with `IsTest` (adapter passes `IsTest = true`) |
| D13 failed delivery | final `failed` (window closed 131047, blocked bot 403, Discord 50007) emits `message.delivery_updated` final via 03 |
| Meta pricing changes 2026-10-01 | service messages: first 1,000 per number per month free then charged; utility templates inside open window now charged. Shown in checklist help text; no Dimmi-side billing effect at MVP |

## Dependencies on other specs

| Spec | Needs from it | Gives to it |
|---|---|---|
| 00-foundation + framework | channel_connections, contact_identities, messages, River, events, `httpclient` (`a.HTTP`, safedial, `websocket.Dial`), `encryption.KeyManager`, `ratelimit.Limiter` + `cache.Store` (Valkey), signed URLs (`routing`), `markdown.Parse` AST, live replica count (centrifuge node info on Valkey), registry enums (`contact_identities.type`, `channel_connections.type`), `RetentionGuard`, `Entitlements` (key `channels.discord.custom_bots`) | new tables, new events, registry entries (X1 `whatsapp_bsuid`, no migration) |
| 00-accounts | roles, permission catalog + `authz.Can` (05 registers `channels:manage`, `channels:inspect`), `verified` middleware, `member.deactivated`, `workspace.unsuspended` / `workspace.restored` events, audit log | |
| 03-shared-inbox | `contracts.Channel`, `ChannelManager`, `conversations.ReceiveInboundMessage`, `UpdateMessageDelivery`, `send_channel_message`/`send_channel_typing` jobs, `channel_options` passthrough, conversation.read/closed/reopened events. **Extensions X1, X3-X5** (`channel-contract.md` section "Additions for 05 (X1, X3-X5)") | 4 drivers, `MarkConnectionErrored` action, channel capabilities, `GET /conversations/{c}/channel-state` |
| 01-messenger-widget | widget connections (read-only in generic list) | |
| 04-email-channel | email connections (read-only in generic list), `channel.*` events shared semantics | |
| 06-notifications | consumes `channel.errored`, `channel.disconnected` (`system_alert` to owners/admins), `channel.connected` reason `recovered` (resolve) | events |
| 07-dimmi-agent | sends via 03 actions; reads capabilities (`MaxBodyChars`, `Formatting`) to size answers; typing while generating via `send_channel_typing`; NEW: honor `public_room` (no contact data in public replies) and skip rule for `metadata.<type>.bot_author` | citations footer rendered by each driver from `metadata.ai.citations` (`Sources: 1. [Title](url)` per platform format) |
| 09-agent-native | CLI framework, MCP transport | CLI commands + MCP tools above |
| 10-webhooks | delivers `channel.*`, `whatsapp_template.status_changed` | |
| 11-self-host | `system.app_url_changed` event, `dimmi doctor` hook | channel checks |
| 12-cloud-billing | none (Tech Provider: Meta bills business directly, Dimmi passes through nothing) | `metadata.whatsapp.pricing` informational only |

## Contradictions / notes for foundation owners
1. Resolved (extensibility.md section 10): `contact_identities.type` is registry enum; 05 registers `whatsapp_bsuid` in Go registry (no CHECK migration). Data-model says WhatsApp thread key is `wa_id`, but since April 2026 `wa_id` may be absent. 05 keys on BSUID (X1).
2. Roadmap: "Gateway connection held by one replica (River leader election)". 05 uses per-shard lease table instead (spreads shards, resume state persisted). Same intent. Leases stay in Postgres (`channel_socket_sessions`: resume state + offsets are durable); Valkey holds only rebuildable state (dedupe windows, limiter buckets, OAuth state nonces, health cache).
3. Roadmap CLI spelling `dimmi channels add <name>` outdated; 05 uses colon style `dimmi channels:add <type>` (server/admin commands) everywhere.
4. security.md lists "Discord Ed25519" under webhooks in: only interactions are HTTP; messages arrive over Gateway WebSocket.
5. api.md ability list has no channel ability; 05 uses `settings:*` (admin) and `conversations:read` (members).
6. Conventions status code list lacks 424; 05 uses 424 `Failed Dependency` for provider-side failures during connect (documented).
7. Resolved: Tech Provider model, Meta bills business directly, Dimmi passes through nothing. pricing.md wording "passed through" outdated (owner 12 to fix).
8. Resolved: 11 emits `system.app_url_changed`; 05 listener `RebuildWebhookRegistrations` consumes it.
9. Resolved: inbound adapters write own rows (`channel_contact_states`, `channel_message_refs` for media groups) via 03 `InboundMessage.AfterPersist` hook inside `ReceiveInboundMessage` tx (channel-contract.md section 3 step 9b); all idempotent (upserts keyed by connection + identity / external id).

## Open questions (recommended default in bold)
1. Treat Slack profile email as verified for auto-merge? **No**: store `email` identity `verified=false` (Slack org admins can set arbitrary emails via SCIM); duplicates surface in 03 suggestions.
2. Cloud: allow customer-supplied custom Discord bots (one gateway session each)? **Yes, cap 3 custom Discord bots per workspace on cloud**; self-host unlimited. Cap = entitlement catalog key `channels.discord.custom_bots` (kind limit; self-host -1, cloud 3), read via `Entitlements.Limit`; staff raises per customer with `entitlement_overrides`, no deploy. Over limit: 422 `entitlement_limit_reached` (`key`, `limit`) on `POST /channels/discord` custom.
3. Slack Marketplace listing for shared app before cloud launch? **Yes, submit at beta**; until approved, shared app avoids `conversations.history/replies` except gap backfill (non-Marketplace distributed apps limited to 1 request/min, 15 objects since 2025-05-29).
4. WhatsApp read receipts default on? **Yes** (customers expect blue ticks; admin can turn off).
