# 01 Messenger widget (backend)

Style: caveman. Date 2026-11-01. Backend only. Widget UI (Preact, Shadow DOM) built later against this spec and [sdk-contract.md](sdk-contract.md). Full endpoint shapes in [api.md](api.md). Edge cases in [edge-cases.md](edge-cases.md). Build order in [tasks.md](tasks.md).

Sibling spec: [02-widget-identity](../02-widget-identity/README.md) owns JWT verification, identity secrets, merge-on-login. This spec calls 02 actions; never re-implements them.

## Goal

Every customer conversation starts here. Serve tiny widget from monolith, give visitors a fast, safe chat API (anonymous or verified), give dashboard full widget settings API. First message in inbox under 5 minutes from signup or `docker run`.

Why: roadmap launch item 1 (`research/roadmap.md` "1. Messenger widget"). Feeds item 2 (identity), 3 (inbox), 7 (agent replies), 8 (Help tab), 12 (metering, test mode, badge).

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Loader script (`loader.js`) | at most 3 KB gzipped, async, zero blocking work, `requestIdleCallback` before fetching core |
| Core bundle (launcher + panel + chat + realtime client + engage scheduler) | at most 35 KB gzipped. Realtime client = plain `EventSource` on centrifuge uni-SSE (about 1 KB gz, no centrifuge-js). Engage scheduler at most 1.5 KB inside core (15). Lazy chunks at most 20 KB gz each, named budgets: `help` (08), `changelog` 8 KB (13), `feedback` 15 KB (14), `engage-banner` 6 KB, `engage-modal` 10 KB, `engage-survey` 14 KB (15), `intercom-shim` 2 KB (16), `markdown`, `emoji`, `attachments`. CI size budget fails build on regression |
| Lighthouse impact on host page | TBT +0 ms, no layout shift (launcher fixed position, reserved), no third-party cookies |
| `GET /widget/v1/apps/{app}/config` | p95 under 5 ms server (in-memory cache), CDN cacheable 60 s |
| Boot handshake (`GET /widget/v1/session`, `PATCH /widget/v1/session`, `POST /widget/v1/sessions` identified) | p95 under 30 ms server. Anonymous page load without stored token: zero requests beyond config |
| Message send to teammate screen | p95 under 300 ms same region (foundation budget) |
| Teammate reply to visitor screen | p95 under 300 ms when SSE stream open; under 6 s in polling fallback |
| Time to first message | under 5 minutes: paste snippet, open site, send. Under 2 minutes with `npx dimmi init` (09) |
| Silent failures | zero. Wrong app id, wrong origin, bad token, blocked assets, disabled widget: each gives one `console.error` with code, plain words, docs link |
| Message loss | zero. Client outbox + `client_id` dedupe means retries never duplicate, offline sends never vanish |
| Branding | no badge on self-host by default. Cloud badge only while workspace never paid (pricing.md section 8) |

## Scope

### In (MVP)

- Widget public API `/widget/v1`: app config, boot handshake (session resume, identified/test session start, lazy anonymous session), session refresh, logout, contact read/update (unverified, leads only), conversations list/create/show, messages list/send, attachments (presigned S3 PUT or direct upload), typing indicators, read state, human handoff request, page-view/context ping, realtime connect tokens, polling fallback for realtime, team availability. Route group shared with feature tabs owned elsewhere: Help (08), What's new `/widget/v1/changelog*` (13), Feedback `/widget/v1/feedback/*` (14), engage `/widget/v1/engage-items`, `/engage-events`, `/announcements/*`, `/surveys/*`, `/engage-assets/*` (15). Config carries their blocks (`features.changelog`, `features.feedback`, `engage`).
- Default widget: created by workspace initializer hook (`EnsureDefaultWidget`, contracts.md) at workspace creation, never lazily; install snippet ready immediately.
- Dashboard API `/api/v1/widget-settings`: CRUD, install snippets per framework, install status (last boot, origins seen), test session minting.
- Realtime: centrifuge uni-SSE (`framework/broadcasting`, embedded centrifuge, built-in Redis broker + presence on Valkey; history in Valkey). Widget opens plain `EventSource` on `GET /connection/uni_sse?cf_connect=<json>` with 60 s single-purpose connect token from `POST /widget/v1/realtime-tokens` (visitor session token never in URL). Server-side subscription to `widget_contact:{ctc}` (uni transport cannot subscribe). History + recovery: client reconnects with last `offset`/`epoch`. Widget-safe payloads; teammate typing relayed to widget (03 REST relay); visitor typing stays REST (`typing-indicators`, stream is receive-only); connection cap per session; kick on revoke (`Disconnect`).
- Widget channel adapter `app/integrations/widget` (Channel contract: outbound = realtime + offline email fallback).
- Offline reply by email: unread teammate/agent reply to visitor with email gets email after delay, threaded via 04 reply address.
- Asset serving: loader, hashed chunks, manifest with SRI hashes, precompressed br/gzip, CDN headers, `assets_base_url` override.
- Test mode: test sessions (dashboard-minted token, member email match, sandbox workspace) mark `conversations.is_test = true` (column owned by 03, migration `20261101000303`; never billable, D10).
- Anti-abuse: origin allowlist, honeypot, timing heuristic, adaptive proof-of-work challenge (no CAPTCHA), per-session/IP/app rate limits, bot score, blocked contacts silent spam.
- JS SDK public contract (document only): [sdk-contract.md](sdk-contract.md).
- `window.Intercom` shim (launch: importers, 16): loader installs `window.Intercom` when `widget_settings.intercom_shim` or SDK option `intercomShim: true`; maps Intercom calls onto Dimmi SDK (sdk-contract "Intercom shim"; mapping table owned by 16 `sources.md`). Identity through shim always unverified (`user_id` -> `claimed_user_id`, 02 R5), attributes ignored (02 R8); verified identity only via Dimmi identity source.
- Agent action confirmation card (07 agent actions): widget renders `metadata.ai.confirmation` card, Confirm/Cancel via `PATCH /widget/v1/conversations/{conversation}/action-calls/{call}` (07 controller, 01 route group), live status via `ai.action_call_updated` (section "Agent action card").
- MCP tools for widget install and settings.

### Out (later)

| Item | When |
|---|---|
| Server-side `track` ingestion and event audiences | post-MVP: upmarket set (outbound). MVP `track()` is client-only (15 engage triggers, no network) |
| Office hours schedule (rules), auto-replies by schedule | post-MVP: rules lite. MVP has free-text office hours + reply time expectation only |
| Console error / network capture (developer context) | standout: developer context |
| AI token streaming into widget | later; MVP shows agent typing indicator, reply arrives complete |
| Custom widget API domain (CNAME + TLS) | later; MVP solves ad-blockers via first-party proxy path (`api_base`) documented in sdk-contract |
| React Native / mobile SDKs | post-MVP: mobile SDKs. Same `/widget/v1` API, headless |
| Headless hooks SDK (`useConversation`) | standout: headless widget SDK. API already sufficient |
| Translations of widget strings beyond `en` + workspace locale overrides | post-MVP: auto-translation. MVP ships `en`, dictionary keyed for later |
| Multi-brand widgets with separate help centers | later. MVP: up to 10 widgets per workspace, each may link one help center |
| Per-stack packages `@dimmi/sveltekit`, `@dimmi/nuxt`, `@dimmi/vue`, `@dimmi/astro`, Rails/Laravel/Python/Go libs, `@dimmi/server/fastify`, Supabase helpers | later: identity-helper-packages. MVP: script tag, `@dimmi/browser`, `@dimmi/react`, `@dimmi/nextjs`, `@dimmi/server`; other stacks get install text = tag + 02 raw snippet |
| Bidirectional WebSocket / centrifuge-js in widget | not planned. Widget stream is receive-only uni-SSE; writes are REST. Long-poll fallback only when proxy buffers streaming |

CSAT prompt on close ships in 15 (system message `survey_request`, widget card). What's new tab ships in 13. Feedback tab ships in 14. `window.Intercom` shim ships with 16 importers (launch: importers).

## User stories

- Workspace member (owner/admin): set colors, launcher, greeting, reply-time text, allowed origins, email capture rule; copy install snippet for my framework; see "installed, last seen on https://app.acme.com 2 minutes ago"; preview widget as test without paying.
- Workspace member (member): read install snippet, mint test session to try widget, see visitor's current page in inbox sidebar.
- End customer (anonymous): open widget, ask question, attach screenshot, see typing and replies live, leave email when team offline, get reply by email, reply to that email and land in same conversation.
- End customer (logged in host app): see my past conversations on any device; logout on shared laptop leaves nothing behind.
- Developer: run `npx dimmi init`, paste one script tag, or `npm i @dimmi/react` / `@dimmi/nextjs` (sdk-contract Packages); get loud console error when app id wrong or origin not allowed; SPA route changes tracked automatically; proxy widget through my domain to dodge ad-blockers.
- AI agent (Dimmi Agent, 07): replies land in widget via realtime same as human; typing indicator while run in progress; handoff button always works.

## Data model

New tables owned by 01. Conventions apply (UUIDv7 `id`, `workspace_id` first in every index, `created_at`/`updated_at`, text + CHECK).

### widget_settings

One row per widget install. Public id prefix `wgt_` = **app id** in snippet. Each row owns one `channel_connections` row of type `widget` (created in same tx).

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `wgt_...` |
| workspace_id | uuid not null references workspaces on delete cascade | |
| channel_connection_id | uuid not null unique references channel_connections on delete cascade | type `widget`, carries `inbox_id` |
| name | text not null | 1 to 60 chars, default "Website" |
| enabled | boolean not null default true | false: config returns 403 `widget_disabled` |
| appearance | jsonb not null default '{}' | typed `models.WidgetAppearance`: `primary_color` (#RRGGBB), `theme` (`light`,`dark`,`auto`), `position` (`right`,`left`), `padding_x`, `padding_y` (0 to 200 px), `launcher.style` (`bubble`,`tab`,`hidden`), `launcher.label` (0 to 24 chars), `launcher.icon` (`chat`,`help`,`wave`), `font` (`system`,`inherit`). Max 4 KB |
| texts | jsonb not null default '{}' | typed `models.WidgetTexts`: `greeting_title` (60), `greeting_body` (280), `office_hours_text` (140), `offline_message` (280), `email_prompt` (140), `composer_placeholder` (60), `feedback_tab_label` (24, 14). Plain text, NFC, no HTML |
| locale | text not null default 'auto' | `auto` = browser, else BCP 47 |
| reply_time | text not null default 'auto' CHECK in ('auto','minutes','hours','day') | `auto` from team availability |
| show_team_avatars | boolean not null default true | up to 3 most recently active members with avatars |
| show_availability | boolean not null default true | online dot + "Usually replies in ..." |
| allowed_origins | text[] not null default '{}' | max 50; each normalized `scheme://host[:port]`, lowercase, punycode; wildcard only as leftmost label `https://*.acme.com`. Empty = any origin |
| allow_anonymous | boolean not null default true | false: visitors without verified identity see nothing (launcher hidden, console info) |
| collect_email | text not null default 'offline' CHECK in ('never','offline','always') | when widget asks lead for email (after first message) |
| require_email_when_offline | boolean not null default false | server blocks lead message send with 422 `email_required` while team offline and lead has no email |
| attachments_enabled | boolean not null default true | |
| max_attachment_mb | int not null default 10 CHECK between 1 and 25 | also capped by `filesystems.max_upload_mb` |
| help_center_id | uuid null | references `help_centers` (08) on delete set null; FK added in 08's migration if table created later |
| help_tab_enabled | boolean not null default false | requires `help_center_id` |
| changelog_tab_enabled | boolean not null default false | 13 "What's new" tab; requires `help_center_id` (changelog lives on help center). Migration `20261101000107` |
| feedback_tab_enabled | boolean not null default false | 14 Feedback tab. Column added by 14 migration `001421` (documented here, 01 table). Requires 14 portal settings row; independent of `portal_enabled` |
| intercom_shim | boolean not null default false | 16 switchers: loader installs `window.Intercom` shim (sdk-contract "Intercom shim"). Config exposes `features.intercom_shim`. Migration `20261101000108` |
| show_branding | boolean null | null = edition default; see Branding rule |
| config_version | bigint not null default 1 | bumped on every update; used in ETag and cache key |
| last_boot_at | timestamptz | last SDK boot seen (any widget request incl. config GET); in-memory install tracker, flushed at most once per minute per replica (see Boot handshake "Install tracking") |
| last_boot_origin | text | origin of that request |
| created_at, updated_at, deleted_at | timestamptz | soft delete; deleted widget returns 404 `app_not_found` to config |

Indexes: `(workspace_id, deleted_at)`, unique `(channel_connection_id)`.

Origins seen and blocked (install helper): kept in Valkey cache (rebuildable; keys `widget:origins:{wgt}`, `widget:origins_blocked:{wgt}`, 50 most recent each, 7 days), not table. Written by install tracker flush.

### widget_sessions

Visitor credential. Bearer token random 32 bytes, base64url, prefixed `dmi_ws_`; only SHA-256 stored. Public id prefix `wss_` (also visitor id).

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `wss_` |
| workspace_id | uuid not null | |
| widget_settings_id | uuid not null references widget_settings on delete cascade | |
| contact_id | uuid not null references contacts on delete cascade | |
| token_hash | bytea not null | SHA-256 of token; unique |
| kind | text not null CHECK in ('anonymous','identified') | `identified` only from verified JWT (02) |
| test | boolean not null default false | test session (D10) |
| identity_secret_id | uuid null references identity_secrets on delete set null | secret that verified session JWT (02) |
| external_id | text null | snapshot of verified `sub`; resume (`PATCH /widget/v1/session`) requires same `sub` |
| verified_at | timestamptz | last successful JWT verification |
| expires_at | timestamptz not null | anonymous: sliding `widget.anonymous_session_ttl` (180 days) on activity. identified: `verified_at + identity.session_ttl` (02, default 24 h), extended only by fresh JWT |
| revoked_at | timestamptz | |
| revoke_reason | text CHECK in ('logout','identity_switch','contact_merged','contact_erased','secret_revoked','expired','widget_deleted','bot','admin') | |
| bot_score | smallint not null default 0 | 0 to 100; at least `widget.bot.block_score` (70) = conversations land as spam |
| pow_solved_at | timestamptz | proof-of-work solved at create |
| origin | text | normalized origin at create |
| user_agent | text | truncated 512 |
| ip_prefix | inet | /24 IPv4, /48 IPv6 (privacy); rate limits use full IP in memory only |
| country | text | ISO 3166-1 alpha-2 from optional GeoIP |
| locale, timezone | text | client-reported, validated |
| current_page | jsonb | `{url, title, referrer, at}`; URL max 2,048 chars, query stripped of keys matching `token|key|secret|password|code|session` |
| recent_pages | jsonb not null default '[]' | last 10 pages, same shape |
| context | jsonb not null default '{}' | page context from `Dimmi('update', {context})`, max 4 KB, 30 keys, scalar values |
| last_seen_at | timestamptz not null default now() | throttled 60 s |
| created_at, updated_at | timestamptz | |

Indexes: unique `(token_hash)` (global, lookup before tenant known, see Security), `(workspace_id, contact_id) WHERE revoked_at IS NULL`, `(workspace_id, identity_secret_id) WHERE revoked_at IS NULL`, `(workspace_id, expires_at)`.

Tenant resolution before workspace known: two `SECURITY DEFINER` SQL functions return only `(workspace_id, id)`: `widget_resolve_app(uuid)` and `widget_resolve_session(bytea)`. Then middleware puts workspace in context (`tenancy.With`); every transaction applies `SET LOCAL app.workspace_id` and normal RLS-scoped queries follow. No other cross-tenant read path.

Contact identity row: anonymous lead gets `contact_identities (type 'widget_visitor', value = wss_ id of first session, verified false)`.

Registry enums (`00-foundation/extensibility.md` P1, 03 data model): 01 registers channel type `widget` (`enums.ChannelType`) and identity type `widget_visitor` (`enums.IdentityType`) in Go registry at boot (`app_service_provider.go`, beside eraser registration). No CHECK value, no migration.

### Core table changes

| Table | Change | Migration |
|---|---|---|
| attachments | `uploaded_at timestamptz` (null until upload complete; messages accept only uploaded rows). Backfill `uploaded_at = created_at` | `20261101000106_add_uploaded_at_to_attachments_table.sql` |

Test flag: 01 adds no conversations column. Uses 03 `conversations.is_test` (`20261101000303_add_inbox_columns_to_conversations.sql`, partial index there if needed); 01 sets it only via `InboundMessage.IsTest`. Migration number `20261101000105` unused (removed).

`conversations.source` (existing jsonb) gets documented widget keys: `widget_id`, `session_id`, `page_url`, `page_title`, `referrer`, `recent_pages`, `context`, `user_agent`, `browser`, `os`, `country`, `locale`, `timezone`, `announcement_id` (15: chat-format announcement reply). `messages.metadata.client` (existing jsonb): `client_id`, `page_url`. `messages.metadata.widget` (server-stamped on every contact message from widget, never from client input): `{session_id, identity_verified}`; `identity_verified = true` only when session `kind = identified`, verified via 02 JWT and not expired at insert (07 agent actions bind identity only on this flag). Test sessions: `identity_verified = false`.

### Migrations

| File | What |
|---|---|
| `20261101000101_create_widget_settings_table.sql` | table + indexes + RLS |
| `20261101000102_create_widget_sessions_table.sql` | table + indexes + RLS. `identity_secret_id` column created without FK (goose runs by version; `identity_secrets` arrives in 02's `20261101000201`). FK added by 02 in `20261101000204_add_identity_secret_foreign_key_to_widget_sessions_table.sql` |
| `20261101000103_create_widget_resolver_functions.sql` | `widget_resolve_app`, `widget_resolve_session` SECURITY DEFINER, owner role, `search_path` pinned |
| `20261101000104_add_widget_visitor_identity_index.sql` | partial index on `contact_identities (workspace_id, value) WHERE type = 'widget_visitor'` |
| `20261101000106_add_uploaded_at_to_attachments_table.sql` | core change |
| `20261101000107_add_changelog_tab_enabled_to_widget_settings_table.sql` | `changelog_tab_enabled` (13) |
| `20261101000108_add_intercom_shim_to_widget_settings_table.sql` | `intercom_shim` (16 shim) |

## Code map

```
routes/widget.go                                   # /widget/v1 group, middleware: cors:widget, widget.origin, throttle:widget; mounts 08 help, 13 changelog, 14 feedback, 15 engage controllers in same group
routes/api.go                                      # + /api/v1/widget-settings group
routes/web.go                                      # + /widget/v1/loader.js, /widget/assets/*, /widget/v1/manifest.json
routes/channels.go                                 # + widget_contact:{contact} authorizer (server-side subscription only; widget connections never subscribe by name)
routes/console.go                                  # + prune_widget_sessions schedule

config/widget.go                                   # WidgetConfig (see Config keys)

app/http/middleware/
  authenticate_widget.go                           # auth:widget: bearer dmi_ws_, resolve session, workspace into context (SET LOCAL per tx), expiry checks; PATCH /session admits expired (not revoked) identified session
  widget_cors.go                                   # cors:widget (framework CORS middleware + app config): reflect allowed Origin, preflight Max-Age 7200, Vary: Origin; allowed request headers incl. `Dimmi-Feedback-Session` (14)
  verify_widget_origin.go                          # widget.origin: allowlist check, 403 origin_not_allowed; records (widget, origin, allowed) in install tracker on every request
  resolve_widget_app.go                            # widget.app: {app} path or body app_id to WidgetSettings

app/http/controllers/widget/
  show_widget_config_controller.go                 # Invoke  GET  /apps/{app}/config
  show_widget_availability_controller.go           # Invoke  GET  /apps/{app}/availability
  session_controller.go                            # Store POST /sessions, Show GET /session, Update PATCH /session, Destroy DELETE /session
  contact_controller.go                            # Show, Update
  conversation_controller.go                       # Index, Store, Show
  conversation_message_controller.go               # Index, Store
  conversation_typing_indicator_controller.go      # Store  POST  /conversations/{conversation}/typing-indicators
  conversation_read_state_controller.go            # Update PATCH /conversations/{conversation}/read-state
  # conversation_handoff_controller.go owned by 07 (Store POST /conversations/{conversation}/handoffs; route registered here)
  # conversation_action_call_controller.go owned by 07 (Show, Update /conversations/{conversation}/action-calls/{call}; route registered here)
  attachment_controller.go                         # Store (create upload), Update (PATCH {status: "uploaded"})
  attachment_content_controller.go                 # Update (direct PUT body)
  ping_controller.go                               # Store
  realtime_token_controller.go                     # Store  POST /realtime-tokens (60 s uni-SSE connect token)
  update_controller.go                             # Index (polling fallback, only when proxy blocks or buffers SSE)
  # /widget/v1/help/* controllers + resources owned by 08 (routes/widget.go group shared)
  asset_controller.go                              # Loader, Asset, Manifest (web group)

app/http/controllers/web/
  widget_preview_controller.go                     # Show GET /widget-preview/{widget} (resources/views/widget/preview.templ)

app/http/controllers/api/v1/
  widget_settings_controller.go                    # Index, Store, Show, Update, Destroy
  widget_install_controller.go                     # Show   GET  /widget-settings/{widget}/install
  widget_test_session_controller.go                # Store  POST /widget-settings/{widget}/test-sessions

app/http/requests/
  store_widget_session_request.go, update_widget_session_request.go, update_widget_contact_request.go,
  store_widget_conversation_request.go, store_widget_message_request.go, store_widget_attachment_request.go,
  update_widget_attachment_request.go, store_widget_ping_request.go, update_widget_read_state_request.go,
  store_widget_typing_indicator_request.go, index_widget_updates_request.go, store_widget_realtime_token_request.go,
  store_widget_settings_request.go, update_widget_settings_request.go, store_widget_test_session_request.go

app/http/resources/
  widget_config_resource.go                        # public config (no secrets, no member emails); embeds 13 `changelog.WidgetConfigFragment`, 14 feedback fragment, 15 engage block
  widget_session_resource.go                       # handshake payload: session, contact, identity, unread, realtime, config_version, server_time, changelog (13), feedback_unread_count (14)
  widget_realtime_token_resource.go                # {token, expires_at, url, channel}
  widget_read_state_resource.go
  widget_contact_resource.go                       # widget-safe contact
  widget_conversation_resource.go                  # widget-safe conversation (no assignee email, no tags, no notes)
  widget_message_resource.go                       # type message only; author {type, name, avatar_url}; system messages expose `survey_request {survey_id, response_id}` (15 CSAT card); agent messages expose `ai.confirmation {action_call_id, title, text, status, expires_at}` (07 card), never `ai.action_calls`
  widget_attachment_resource.go                    # signed URL 1 h
  widget_upload_resource.go                        # upload instructions
  widget_settings_resource.go                      # dashboard full resource
  widget_install_resource.go                       # snippets + status

app/actions/widget/
  create_widget.go                                 # CreateWidget: widget_settings + channel_connections(type widget) in one tx
  ensure_default_widget.go                         # EnsureDefaultWidget: workspace initializer (contracts.md hook, after 03 EnsureDefaultInbox, in CreateWorkspace tx); one "Website" widget on default inbox; idempotent (skips when any widget exists) for `dimmi workspace:repair`
  update_widget_settings.go                        # UpdateWidgetSettings: validate, bump config_version, event, cache bust
  bump_widget_config_version.go                    # BumpWidgetConfigVersion: called by 13 (entry published/unpublished) and 15 (engage settings version); same Valkey PubSub invalidation as settings update
  delete_widget.go                                 # DeleteWidget: soft delete, revoke sessions, disconnect channel
  start_widget_session.go                          # StartWidgetSession: POST /sessions. test_token -> test session; identity token (wire `user_jwt`) -> 02 VerifyIdentityToken + IdentifyVisitor, previous_token revoke/merge; else CreateAnonymousSession
  resume_widget_session.go                         # ResumeWidgetSession: GET /session (anonymous resume) and PATCH /session {user_jwt} (identified: 02 verify, same sub, extend, refresh claims)
  create_anonymous_session.go                      # CreateAnonymousSession: ALTCHA proof-of-work check, lead contact + identity + session
  revoke_widget_session.go                         # RevokeWidgetSession: set revoked, disconnect realtime (`Disconnect` by session user id), then 14 `RevokePortalSessions` (surface widget, same widget + contact) for reasons logout, identity_switch, contact_merged, contact_erased
  issue_widget_realtime_token.go                   # IssueWidgetRealtimeToken: connection JWT (60 s, single purpose, `sub` = wss_ id, server-side sub `widget_contact:{ctc}`), connection cap check
  update_widget_contact.go                         # UpdateWidgetContact: lead-only unverified fields; removes every written `custom_attributes` key from `contacts.trusted_attribute_keys` in same tx (widget-set values never trusted, 07)
  start_widget_conversation.go                     # StartWidgetConversation: wraps 03 ReceiveInboundMessage, sets source (incl. `announcement_id` 15 + event row `announcement.replied`), IsTest, spam
  send_widget_message.go                           # SendWidgetMessage: dedupe by client_id, wraps 03 action (reopen rules); stamps `metadata.widget {session_id, identity_verified}` (StartWidgetConversation same)
  mark_widget_conversation_read.go                 # MarkWidgetConversationRead: messages.read_at, delivery_updated
  record_widget_ping.go                            # RecordWidgetPing: current_page, recent_pages, context, last_seen throttle
  broadcast_visitor_typing.go                      # BroadcastVisitorTyping: throttled ephemeral typing.started on `conversation:{conv}` (no history)
  mint_widget_test_session.go                      # MintWidgetTestSession: one-time test token, 10 min
  score_widget_bot.go                              # ScoreWidgetBot: honeypot, timing, UA, PoW, rate signals
  compute_team_availability.go                     # ComputeTeamAvailability: online flag, avatars, reply time
  resolve_branding.go                              # ResolveBranding: edition + 12 billing HasPaid + show_branding
  build_install_snippets.go                        # BuildInstallSnippets: framework ids = 09 `dimmi init` ids: nextjs-app, nextjs-pages, react, react-router, sveltekit, nuxt, vue, astro, rails, laravel, django, fastapi, go, express, hono, html, proxy; text from sdk-contract "Install by stack"
  erase_widget_contact_data.go                     # EraseWidgetContactData: GDPR contact eraser (00-foundation/contracts.md "Contact eraser hooks"), registered on 03 eraser registry; revokes contact's widget_sessions (reason contact_erased, disconnects realtime), scrubs current_page/recent_pages/context; idempotent

app/actions/attachments/                           # shared with 03 (see Dependencies)
  create_pending_upload.go                         # CreatePendingUpload: row with uploaded_at null, presign or direct URL
  complete_upload.go                               # CompleteUpload: HEAD, size check, sniff type, set uploaded_at, queue scan

app/integrations/widget/
  adapter.go                                       # contracts.Channel impl (03): Type, Capabilities, Send (realtime delivery + offline email schedule); passes tests/contracts/channel_test.go
  capabilities.go                                  # Transport realtime; attachments yes, typing yes, read receipts yes; deletes via realtime (03 rule; edits later: message-edit); reactions no; window none
  payloads.go                                      # widget-safe broadcast payload builders (10 KB cap, fallback {id,object,updated_at})

app/broadcasting/widget_contact_channel.go         # authorizer for widget_contact:{ctc} (connection JWT `sub` session must own contact, session not revoked); used by OnConnecting server-side subscription
(teammate typing to widget: 03 `RecordTyping` relay publishes to widget_contact:{ctc}; no client-event hook)

app/listeners/
  broadcast_to_widget.go                           # AfterCommit: message.created/updated/deleted/delivery_updated, conversation.closed/reopened/created, conversation.seen_by_team (03), ai.handoff_requested, ai.action_call_updated (07, widget-safe `{id, title, status}`), contact.updated -> widget_contact:{ctc} (history on, ephemeral events without history)
  schedule_widget_offline_email.go                 # queued(default): message.created (member/agent, widget conversation) -> SendWidgetOfflineReplyEmail delayed
  repoint_widget_sessions.go                       # InTx: contact.merged -> repoint lead targets, revoke when target is user
  (contact erase: no listener; see eraser `EraseWidgetContactData` in actions)
  bust_widget_config_cache.go                      # AfterCommit: widget.settings_updated, 13 changelog_entry.published/unpublished, 15 engage_settings version change (`BumpWidgetConfigForEngage`) -> otter delete + Valkey PubSub invalidation (all replicas)

app/jobs/
  send_widget_offline_reply_email.go               # SendWidgetOfflineReplyEmail: collects unread replies, renders WidgetOfflineReplyMail, delivers only via 04 `email.SendWidgetOfflineReply` (never `a.Mail` directly); 04 `skipped{reason}` = done
  prune_widget_sessions.go                         # PruneWidgetSessions
  revoke_widget_sessions_for_secret.go             # RevokeWidgetSessionsForSecret (02 compromise path)

app/mail/widget_offline_reply_mail.go              # WidgetOfflineReplyMail (templ resources/views/mail/widget_offline_reply.templ): render only (HTML, text, subject); delivered only via 04 `SendWidgetOfflineReply` (sender, Reply-To token, List-Unsubscribe, suppression, limits, customer stream). Continue link = conversation `source.page_url` + `#dimmi-conversation=conv_...` (SDK opens thread if session owns it, else shows "reply to this email to continue"); email reply continues thread via 04

app/events/widget_settings_updated.go              # widget.settings_updated
app/events/widget_created.go                       # widget.created
app/events/widget_deleted.go                       # widget.deleted

app/policies/widget_settings_policy.go             # viewAny, view, create, update, delete, mintTestSession; each = authz.Can(principal, permission) (Permissions matrix), no role names
app/enums/widget_session_kind.go, widget_collect_email.go, widget_reply_time.go, widget_revoke_reason.go
app/exceptions/widget.go                           # ErrAppNotFound, ErrWidgetDisabled, ErrOriginNotAllowed, ErrSessionExpired, ErrSessionRevoked, ErrEmailRequired, ErrPowRequired, ErrIdentityLocked, ErrIdentityProofRequired, ErrAttachmentsDisabled ...

app/support/widgetassets/                          # embed.FS of public/widget + widget asset config only; manifest parse, SRI, precompressed variant pick = `framework/view/vite` + `framework/view/static`
app/support/pow/                                   # thin wrapper on altcha-org/altcha-lib-go (MIT): HMAC-signed SHA-256 challenge (key from `encryption` app key), expiry + IP prefix in salt params, adaptive `maxnumber`; single-use tracking (Valkey `SET NX EX` on challenge signature, TTL = expiry) stays ours. Widget ships ~30-line ALTCHA solver in Web Worker (no ALTCHA web component)
(origin normalize, wildcard match, punycode: `framework/http/origin`; wildcard leftmost-label rule passed as option)
app/support/widgetinstall/                         # install tracker: in-memory last seen + origins seen/blocked per widget, flush every widget.install_flush_interval (UpdateWidgetLastBoot + cache), flush on shutdown

app/mcp/tools/
  dimmi_get_widget_install.go
  dimmi_list_widgets.go
  dimmi_update_widget_settings.go

database/migrations/2026110100010{1..8}_*.sql
database/queries/widget_settings.sql               # FindWidgetSettings, ListWidgetSettings, CreateWidgetSettings, UpdateWidgetSettings, DeleteWidgetSettings, UpdateWidgetLastBoot, CountWidgetSettings
database/queries/widget_sessions.sql               # FindWidgetSessionByTokenHash, CreateWidgetSession, UpdateWidgetSessionActivity, ExtendWidgetSession, RevokeWidgetSession, RevokeWidgetSessionsForContact, RevokeWidgetSessionsForSecret, RepointWidgetSessions, ListActiveWidgetSessionsForContact, DeleteExpiredWidgetSessions
database/queries/widget_conversations.sql          # ListWidgetConversationsForContact, ListWidgetMessages, ListWidgetUpdatesSince, CountWidgetUnread, MarkWidgetMessagesRead
database/factories/widget_settings_factory.go, widget_session_factory.go

tests/feature/widget/                              # integration (real App, Postgres, centrifuge node in memory); one file per controller (config, sessions/handshake, contact, conversations, messages, attachments, typing indicators, read state, pings, realtime tokens, uni-SSE stream, updates, help, assets); action logic (session start/resume, PoW, dedupe/reopen, bot score, branding, availability, install tracker) covered through these endpoints and jobs. Tests named in plain words; every edge case covered
tests/feature/api/widget_settings_test.go, widget_install_test.go, widget_test_session_test.go
(origin normalize + asset manifest unit tests live with `framework/http/origin`, `framework/view/vite`)
tests/e2e/widget_protocol_test.go                  # Node script: every handshake row, send, receive via plain EventSource on /connection/uni_sse (reconnect with offset recovers), polling fallback
```

## Boot handshake

`boot()` is SDK API ([sdk-contract.md](sdk-contract.md)), not HTTP. No `boot` route (rest.md rule 1). On every page load SDK runs handshake below; server enforces every row that touches identity. Bodies in [api.md](api.md) "Boot handshake".

Identity token comes from page's identity source (sdk-contract "Identity sources": Next server action, same-origin identity URL `GET /api/dimmi/token`, identity function or source object). Helpers mint per request, recommended `exp` 10 minutes (02 snippets.md). Never inlined in HTML. Wire field on Dimmi routes stays `user_jwt` (JSON body only, never URL). Column "Identity token" below = what identity source answered.

Always first: `GET /widget/v1/apps/{app}/config` (CDN cacheable). Gives features, `config_version`. Anonymous visitor without stored token and no identity source stops here.

| Stored token (`dimmi:{app}:session`) | Identity token | Request | Result |
|---|---|---|---|
| none | none | none | no session, no contact, no DB row. First interaction: lazy `POST /widget/v1/sessions {app_id}` (PoW rules unchanged) |
| none | present | `POST /widget/v1/sessions {app_id, user_jwt}` | 201 identified session (02 `IdentifyVisitor`) |
| anonymous | none | `GET /widget/v1/session` (bearer) | 200 resume: session, contact summary, unread count, server_time; slides `expires_at` (write throttled) |
| anonymous | present | `POST /widget/v1/sessions {app_id, user_jwt, previous_token}` | 201 identified; lead upgraded in place or merged into user (02); old session revoked `identity_switch` |
| identified, same `sub` | present | `PATCH /widget/v1/session {user_jwt}` (bearer) | 200 resume: extend `expires_at`, refresh claims (02). Admits expired, not revoked identified session (JWT proves identity again) |
| identified, other `sub` | present | `POST /widget/v1/sessions {app_id, user_jwt, previous_token}` | 201 new identified session; old revoked `identity_switch`; no merge user into user (02 R7) |
| identified | none (source answered `{token: null}`) | `DELETE /widget/v1/session` | 204 revoked `logout`; SDK wipes storage, continues as row 1 (02 R6: sign-out detected, shared device; `Dimmi.logout()` rarely needed) |
| any | (dashboard preview) `test_token` | `POST /widget/v1/sessions {app_id, test_token}` | 201 test session (`test = true`), per-member test lead. Replaces former boot `test_token` path |

Rules:
- Route choice: SDK base64url-decodes JWT payload `sub` (no verification, routing hint only) and compares SHA-256 with stored `external_id_hash`. Server never trusts it: `PATCH /session` with other `sub` returns 409 `identity_mismatch`; SDK then sends `POST /sessions` with `previous_token`. Undecodable JWT: SDK sends `POST /sessions` and server returns 401 `token_malformed`.
- `GET /session` resumes anonymous sessions only. Identified session: 409 `identity_proof_required`, session untouched (identified session never resumes without JWT for same `sub`, 02 R6; guards buggy or forked clients). SDK never sends it for identified kind.
- 401 `session_expired` / `session_revoked` / `session_invalid` on `GET /session`: SDK drops token, continues with "stored token none" row. Identified + `session_expired` + JWT: `PATCH /session` (revives). Anonymous expired: old lead stays in inbox, no merge.
- `previous_token` (body only, never URL or query): honored only when valid (not expired, not revoked) and same widget. Else ignored silently, treated as none. Never returned or logged.
- `POST /sessions` with `previous_token` identified, same `sub`: no new session; 200 with existing session extended (`token` omitted). Same as `PATCH /session`.
- Failed `POST /sessions` (invalid JWT, any 4xx) has no side effects: previous session untouched, nothing revoked, no contact. Invalid JWT returns 401 with 02 code (no modes at MVP, 02 "Identity behavior"). SDK continues as if no JWT (stored anonymous: `GET /session`; none: lazy) plus `console.error`.
- Mid-session re-check (navigation, tab visible, `identify()`, source `subscribe` callback; rules in sdk-contract "Identity sources"): same table (stored anonymous: `POST /sessions` with `previous_token`; stored identified same `sub`, claims changed or near expiry: `PATCH /session`; same `sub`, nothing changed: no request).
- Identity source error (network, 5xx, server action rejected): page load = treated as no token (fail closed); mid-session = keep state, retry next trigger.
- `identity_mismatch` (409) kept only on `PATCH /session` (refresh or resume with other `sub`). Fix: `POST /sessions` with `previous_token`. Old "reboot" hint replaced.

### Install tracking

Install status (`last_boot_at`, `last_boot_origin`, `origins_seen`, `origins_blocked`, "installed" in widget settings) must work without extra request on anonymous page load. Source: every widget request, config GET included (always sent on load).
- `widget.origin` middleware records `(widget, normalized origin, allowed)` into in-memory `support/widgetinstall` tracker. Map write only; config p95 5 ms budget unchanged; zero DB on request path.
- Tracker flush every `widget.install_flush_interval` (60 s) per replica: `UpdateWidgetLastBoot` (keeps max `last_boot_at`) + origins seen/blocked cache sets. Flush on shutdown. Lost counters on crash: up to 60 s (install hint, not billing).
- CDN in front of config (`s-maxage=60`, cache key varies by Origin, already required by CORS): origin server still sees each origin at least once per cache window (miss, revalidation, ETag 304). `last_boot_at` freshness about 2 minutes worst case vs 1 minute before. Accepted. Self-host without CDN: every load hits server, tracker coalesces.
- 403 `origin_not_allowed` responses `no-store`: every blocked load reaches server, recorded in `origins_blocked`.

### Agent action card (07 agent actions)

UI contract for 07 write actions (`07-dimmi-agent/actions.md` "Write call", "Confirmation"). Server pieces: 07 controller, 01 route group, widget-safe resource, widget channel event.
- Source: agent message `metadata.ai.confirmation {action_call_id, title, text, status, expires_at}` (widget message resource exposes this subtree only). Body stays plain text (transcript, offline email); card = body + buttons.
- Buttons: Confirm / Cancel, labels from 07 lang keys `action_confirm`, `action_cancel` in conversation language (config ships strings). Enabled only while `status = pending_confirmation`, `expires_at` in future and session identified + verified; anonymous or expired identified session: buttons hidden, text "Sign in again to confirm" (`PATCH /session` refresh first when identity source exists).
- Click: `PATCH /widget/v1/conversations/{conversation}/action-calls/{call} {status: "confirmed" | "canceled"}`; optimistic spinner; 200 sets card state from response; 409 `action_call_not_pending` / `action_call_expired` / `identity_changed`: refetch `GET .../action-calls/{call}`, render final state, no retry. Double click: one request in flight per card.
- States rendered: `pending_confirmation` (buttons), `confirmed`/`running` ("Checking {title}…" indicator; same indicator for read calls on `ai.action_call_updated` status `running`), `pending_approval` ("Waiting for the team to approve"), `succeeded` / `failed` (card closed; follow-up agent message carries result), `canceled`, `declined`, `expired` (greyed, no buttons). Unknown status: greyed, no buttons.
- Live: `ai.action_call_updated` on `widget_contact:{ctc}` (widget-safe `{id, conversation_id, title, status}`) updates card and indicator; recovery or polling: refetch messages (status in `metadata.ai.confirmation`, kept current by 03 `SetMessageMetadataKey`, no `message.updated`).
- Typed "yes"/"no" in composer = normal message; 07 fast path handles it server side (card updates via event).
- Never: buttons for `ai.action_calls` (member-only), raw input/response, connection names.

## HTTP API

Full request/response bodies in [api.md](api.md). Widget responses use widget-safe resources only.

### Widget API (`/widget/v1`)

Auth legend: `app` = public, app id resolved and origin checked; `widget` = `auth:widget` bearer `dmi_ws_...` (plus origin check).

| Method | Path | Route name | Auth | Purpose |
|---|---|---|---|---|
| GET | `/widget/v1/apps/{app}/config` | widget.apps.config.show | app | appearance, texts, features, branding, `config_version`. CDN cacheable |
| GET | `/widget/v1/apps/{app}/availability` | widget.apps.availability.show | app | team online, avatars, reply time. Cache 30 s |
| POST | `/widget/v1/sessions` | widget.sessions.store | app (body `app_id`) | start session: anonymous (lazy, may require PoW), identified (`user_jwt`, optional `previous_token`), test (`test_token`). See Boot handshake |
| GET | `/widget/v1/session` | widget.session.show | widget | resume anonymous session: session, contact summary, unread count, server_time. Identified: 409 `identity_proof_required` |
| PATCH | `/widget/v1/session` | widget.session.update | widget (expired identified admitted) | `{user_jwt}`: resume or refresh identified session (02), same `sub` else 409 `identity_mismatch` |
| DELETE | `/widget/v1/session` | widget.session.destroy | widget | logout: revoke token, disconnect realtime streams |
| GET | `/widget/v1/contact` | widget.contact.show | widget | own widget-safe profile |
| PATCH | `/widget/v1/contact` | widget.contact.update | widget | lead only: name, email, phone, custom_attributes (unverified) |
| GET | `/widget/v1/conversations` | widget.conversations.index | widget | own conversations, newest activity first |
| POST | `/widget/v1/conversations` | widget.conversations.store | widget + `idempotent` | create conversation + first message |
| GET | `/widget/v1/conversations/{conversation}` | widget.conversations.show | widget | |
| GET | `/widget/v1/conversations/{conversation}/messages` | widget.conversations.messages.index | widget | cursor, oldest-first pages scrolling back |
| POST | `/widget/v1/conversations/{conversation}/messages` | widget.conversations.messages.store | widget + `idempotent` | send; dedupe by `client_id`. Also continue/reopen route for past conversations: closed within reopen window reopens (`meta.reopened`), past window creates linked new conversation (`meta.redirected_from`). No separate `continue` route |
| POST | `/widget/v1/conversations/{conversation}/typing-indicators` | widget.conversations.typing-indicators.store | widget | visitor typing, 202, throttled, ephemeral (not stored) |
| PATCH | `/widget/v1/conversations/{conversation}/read-state` | widget.conversations.read-state.update | widget | `{last_read_message_id}`: read receipt up to message |
| POST | `/widget/v1/conversations/{conversation}/handoffs` | widget.conversations.handoffs.store | widget | "Talk to a human"; controller + action `HandOffConversation` owned by 07 |
| GET | `/widget/v1/conversations/{conversation}/action-calls/{call}` | widget.conversations.action-calls.show | widget | 07 agent action call, widget-safe `{id, object: "action_call", title, status, confirmation_text, expires_at}`; controller owned by 07 |
| PATCH | `/widget/v1/conversations/{conversation}/action-calls/{call}` | widget.conversations.action-calls.update | widget (session identified + verified, identity matches call snapshot) | `{status: "confirmed" \| "canceled"}`; 200 (status `confirmed` or `pending_approval`); 409 `action_call_not_pending`, `action_call_expired`, `identity_changed`; other contact 404. Controller owned by 07 |
| POST | `/widget/v1/attachments` | widget.attachments.store | widget | create pending upload, get upload instructions |
| PUT | `/widget/v1/attachments/{attachment}/content` | widget.attachments.content.update | widget | direct upload (local disk driver, or S3 without presign) |
| PATCH | `/widget/v1/attachments/{attachment}` | widget.attachments.update | widget | `{status: "uploaded"}`: verify object, mark uploaded |
| POST | `/widget/v1/pings` | widget.pings.store | widget | page view + context |
| POST | `/widget/v1/realtime-tokens` | widget.realtime-tokens.store | widget | 201 connect token for uni-SSE: 60 s, single purpose (connect only, never REST), `sub` = `wss_` session, server-side subscription `widget_contact:{ctc}` |
| GET | `/widget/v1/updates` | widget.updates.index | widget | polling fallback when proxy blocks or buffers SSE: events since cursor |
| GET | `/widget/v1/help`, `/help/collections/{col}`, `/help/articles/{art}`, `/help/articles?filter[intercom_id]=`, `/help/search`, POST `/help/articles/{art}/feedback` | widget.help.* | app or widget | owned by 08 (`08-help-center/README.md` Widget routes); 01 only shares route group + middleware. Resolves widget's `help_center_id`. `filter[intercom_id]` resolves imported Intercom article id to `art_` via 16 import records (shim `showArticle`) |
| * | `/widget/v1/changelog*` | widget.changelog.* | app or widget | owned by 13; shares group + middleware, like Help |
| * | `/widget/v1/feedback/*` | widget.feedback.* | app (reads) or widget | owned by 14; CORS allows `Dimmi-Feedback-Session` request header |
| * | `/widget/v1/engage-items`, `/engage-events`, `/announcements/*`, `/surveys/*`, `/engage-assets/*` | widget.engage-items.*, ... | per 15 api.md | owned by 15; test sessions may pass `preview` |
| GET | `/widget/v1/openapi.json` | widget.openapi | none | |
| GET | `/connection/uni_sse?cf_connect=<json>` | broadcasting.uni-sse (framework) | connect token in `cf_connect` | own small uni-SSE handler in `framework/broadcasting` on centrifuge public API; not under `/widget/v1`, not REST (rest.md exceptions). `OnConnecting` verifies token + session, returns server-side subscription with recovery from client `offset`/`epoch`. `cf_connect` redacted in access logs |

### Assets (web group, no cookies, CORS `*`)

| Method | Path | Route name | Cache |
|---|---|---|---|
| GET | `/widget/v1/loader.js` | widget.loader | `public, max-age=300, stale-while-revalidate=86400`, ETag |
| GET | `/widget/assets/{file}` (hashed names) | widget.assets | `public, max-age=31536000, immutable` |
| GET | `/widget/v1/manifest.json` | widget.manifest | `max-age=60`; lists version, files, sizes, SRI `sha384` |
| GET | `/widget-preview/{widget}` | widget.preview | `no-store`; session auth (member). Bare SSR host page that boots widget with test token from URL fragment; also reads `#preview=ann_...` or `#preview=svy_...` and forwards to SDK (15 preview) |

Headers on all: `Cross-Origin-Resource-Policy: cross-origin`, `Timing-Allow-Origin: *`, `X-Content-Type-Options: nosniff`, `Content-Type: text/javascript; charset=utf-8`. Precompressed `.br` and `.gz` picked by `Accept-Encoding`, `Vary: Accept-Encoding`. `widget.assets_base_url` set (CDN): loader points chunk URLs there; monolith remains origin.

### Dashboard API (`/api/v1`)

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | `/api/v1/widget-settings` | widget-settings.index | `widgets:read` (settings:read) | list widgets |
| POST | `/api/v1/widget-settings` | widget-settings.store | `widgets:manage` (settings:write) | create widget (+ channel connection); entitlement `widget.max_per_workspace` (422 `entitlement_limit_reached`) |
| GET | `/api/v1/widget-settings/{widget}` | widget-settings.show | `widgets:read` (settings:read) | |
| PATCH | `/api/v1/widget-settings/{widget}` | widget-settings.update | `widgets:manage` (settings:write) | partial update (incl. `changelog_tab_enabled` 13, `feedback_tab_enabled` 14, `intercom_shim` 16) |
| DELETE | `/api/v1/widget-settings/{widget}` | widget-settings.destroy | `widgets:manage` (settings:write) | soft delete, revoke sessions |
| GET | `/api/v1/widget-settings/{widget}/install` | widget-settings.install.show | `widgets:read` (settings:read) | `?framework=<id>` (09 `dimmi init` ids, api.md): snippet + install status + origins seen |
| POST | `/api/v1/widget-settings/{widget}/test-sessions` | widget-settings.test-sessions.store | `widgets:read` (settings:read), session, app or token | one-time test token for preview |

`{widget}` accepts `wgt_` id. Other workspace id returns 404.

### Key examples

Start identified session (stored anonymous token becomes `previous_token`):

```http
POST /widget/v1/sessions
Origin: https://app.acme.com
Content-Type: application/json

{
  "app_id": "wgt_01jbd2k7m3q8r5t6v9w0x1y2z3",
  "user_jwt": "eyJhbGciOiJIUzI1NiIsImtpZCI6ImlkayJ9...",
  "previous_token": "dmi_ws_Q2x...stored-or-null",
  "page": { "url": "https://app.acme.com/billing", "title": "Billing", "referrer": "" },
  "locale": "en-GB",
  "timezone": "Europe/Berlin",
  "sdk": { "name": "dimmi-nextjs", "version": "1.0.0" }
}
```

201:

```json
{
  "data": {
    "id": "wss_01jbd3...",
    "object": "widget_session",
    "token": "dmi_ws_new-token-only-when-issued",
    "kind": "identified",
    "test": false,
    "expires_at": "2026-11-02T10:00:00Z",
    "contact": { "id": "ctc_01jb...", "object": "contact", "kind": "user", "name": "Ada", "email": "ada@acme.com", "email_verified": true },
    "identity": { "status": "verified", "error": null, "warnings": [] },
    "unread_count": 1,
    "open_conversation_ids": ["conv_01jb..."],
    "realtime": { "url": "https://app.trydimmi.com/connection/uni_sse", "channel": "widget_contact:ctc_01jb...", "token_url": "https://app.trydimmi.com/widget/v1/realtime-tokens" },
    "changelog": { "last_seen_at": "2026-10-30T08:00:00Z", "unread_count": 2 },
    "feedback_unread_count": 0,
    "config_version": 14,
    "test_quota_exceeded": false,
    "server_time": "2026-11-01T10:00:00Z"
  }
}
```

`GET /widget/v1/session` and `PATCH /widget/v1/session` return same `widget_session` shape, `token` omitted. Anonymous visitor without stored token sends nothing beyond config; `POST /widget/v1/sessions` only on first write (no contact row per page view). `changelog` present only when `changelog_tab_enabled` (13); `feedback_unread_count` (from 14 `feedback_notifications`, capped 99) only when `feedback_tab_enabled`.

Realtime connect (after handshake, and on every reconnect):

```http
POST /widget/v1/realtime-tokens
Authorization: Bearer dmi_ws_...
```

```json
{ "data": { "object": "realtime_token", "token": "eyJ...60s", "expires_at": "2026-11-01T10:01:00Z", "url": "https://app.trydimmi.com/connection/uni_sse", "channel": "widget_contact:ctc_01jb..." } }
```

SDK then opens `new EventSource(url + '?cf_connect=' + encodeURIComponent(JSON.stringify({token, subs: {[channel]: {recover: true, offset, epoch}}})))`. First connect sends no `offset`. Frames are centrifuge JSON pushes; SDK tracks `offset`/`epoch` per publication. On `error`: close EventSource (auto-retry would reuse expired URL), backoff 1, 2, 4, 8, 15, 30 s with jitter, new token, reopen with last `offset`/`epoch`. Server answers `recovered: true` (missed publications replayed from Valkey history, 10 min / 300 per channel) or `recovered: false` (SDK refetches conversations and messages via REST). Two failed connects in 30 s, or no first frame within 10 s (proxy buffers stream): poll `GET /widget/v1/updates`, retry SSE every 5 min.

Send message:

```http
POST /widget/v1/conversations/conv_01jb.../messages
Authorization: Bearer dmi_ws_...
Content-Type: application/json

{ "client_id": "2f1c6c1e-8a7e-4d0b-9a55-1c2f0b6f7a11", "body": "Still can't export CSV 😕", "attachment_ids": ["att_01jb..."], "website": "" }
```

```json
{
  "data": {
    "id": "msg_01jb...",
    "object": "message",
    "conversation_id": "conv_01jb...",
    "client_id": "2f1c6c1e-8a7e-4d0b-9a55-1c2f0b6f7a11",
    "author": { "type": "contact", "name": "Ada", "avatar_url": null },
    "body": "Still can't export CSV 😕",
    "attachments": [{ "id": "att_01jb...", "object": "attachment", "filename": "error.png", "content_type": "image/png", "size_bytes": 48211, "url": "https://.../signed", "width": 800, "height": 600 }],
    "created_at": "2026-11-01T10:00:01Z",
    "read_at": null
  },
  "meta": { "conversation_status": "open", "reopened": false, "redirected_from": null, "server_time": "2026-11-01T10:00:01Z" }
}
```

Update widget settings:

```http
PATCH /api/v1/widget-settings/wgt_01jbd2...
Authorization: Bearer dmi_live_...

{ "appearance": { "primary_color": "#5B5BD6", "position": "left" }, "allowed_origins": ["https://acme.com", "https://*.acme.com"], "require_email_when_offline": true }
```

Returns full `widget_settings` resource (see api.md) with `config_version` bumped.

### Error codes (widget)

Console text (code, sentence, stack-aware fix, docs link) canonical in [sdk-contract.md](sdk-contract.md) "Loud console errors". Column below = SDK behavior only.

| Status | Code | When | SDK behavior |
|---|---|---|---|
| 404 | `app_not_found` | unknown, malformed, or deleted app id | console `app_not_found`; launcher never renders |
| 403 | `widget_disabled` | `enabled = false` | console `widget_disabled`; launcher hidden |
| 403 | `origin_not_allowed` | Origin not in allowlist | console `origin_not_allowed`; launcher hidden |
| 403 | `workspace_suspended` | `workspaces.suspended_at` | launcher hidden |
| 403 | `anonymous_not_allowed` | `allow_anonymous=false`, no verified identity | info level, launcher hidden |
| 401 | `session_expired` | token past `expires_at` | identified: ask identity source, `PATCH /session`; anonymous or no source: drop token, continue without session (`identity_refresh_missing` warn when identified) |
| 401 | `session_revoked` | logout elsewhere, merge, erase | SDK clears storage, restarts |
| 401 | `session_invalid` | unknown token | same as revoked |
| 401 | `token_*`, `identity_not_configured` | invalid JWT on `POST /sessions` or `PATCH /session` (no side effects), see 02 error table | 02 message + stack-specific hint; SDK continues anonymous |
| 409 | `identity_mismatch` | `PATCH /session` JWT `sub` differs from session `external_id` | handled silently: SDK sends `POST /sessions` with `previous_token` |
| 409 | `identity_proof_required` | `GET /session` with identified token | handled silently: SDK sends `PATCH /session` (token) or `DELETE /session` (no token) |
| 401 | `test_token_invalid`, `test_token_used` | bad or reused dashboard test token on `POST /sessions` | preview shows "link expired, reopen from dashboard" |
| 428 | `pow_required` | anonymous session create needs proof-of-work | handled silently: ALTCHA solve in Web Worker, retry |
| 422 | `pow_invalid` | wrong, expired or reused solution | SDK retries once with fresh challenge |
| 422 | `email_required` | `require_email_when_offline`, team offline, lead has no email | SDK shows email field, keeps draft |
| 403 | `identity_locked` | widget tried to change profile of `user` contact | console info; field read-only |
| 403 | `attachments_disabled` | | |
| 413 | `attachment_too_large` | over `max_attachment_mb` | |
| 422 | `attachment_type_blocked` | executable, html, svg-as-inline etc. | |
| 409 | `attachment_not_uploaded` | message references pending upload | |
| 404 | `conversation_not_found` | other contact's conversation, any workspace | |
| 429 | `rate_limited` | with `Retry-After` | |
| 413 | `payload_too_large` | body over 1 MB | |
| 409 | `action_call_not_pending`, `action_call_expired`, `identity_changed` | agent action card PATCH (07) | refetch call, render final state |
| 503 | `maintenance` | `dimmi down` | SDK keeps outbox, retries with backoff |

## MCP tools

| Tool | Ability | Does |
|---|---|---|
| `dimmi_list_widgets` | settings:read | list widgets with app id, enabled, last boot, origins seen |
| `dimmi_get_widget_install` | settings:read | snippet for framework (install endpoint ids: `nextjs-app`, `react`, `rails`, ..., `html`, `proxy`), install status. Used by agents wiring widget into codebase |
| `dimmi_update_widget_settings` | settings:write | patch appearance, texts, allowed origins, email capture; returns diff. Toolset `widget`; goes through 09 `app/mcp/confirm` gate (widget settings rule), always confirms `allowed_origins`/`allow_anonymous` changes. `idempotentHint=true` |

No CLI commands owned. `npx dimmi init` (09) calls `GET /api/v1/widget-settings/{widget}/install`.

## Events

Emitted (catalog names):

| Event | By | Notes |
|---|---|---|
| conversation.created | StartWidgetConversation (via 03 action) | `source.widget_id`, `is_test` in resource, `initiated_by = contact` |
| message.created | SendWidgetMessage, StartWidgetConversation | author contact |
| message.delivery_updated | MarkWidgetConversationRead | status `read` for member/agent messages; also `sent` set at create by adapter |
| contact.created | CreateAnonymousSession, StartWidgetSession (identified, via 02) | |
| contact.updated | UpdateWidgetContact | changed fields only |
| conversation.reopened | via 03 when visitor writes to closed | reason `customer_message` |
| typing.started / typing.stopped | BroadcastVisitorTyping (to `conversation:{conv}`) | ephemeral (published without history) |

Consumed: `message.created` (offline email, widget broadcast), `message.deleted`, `message.delivery_updated`, `conversation.created`, `conversation.closed`, `conversation.reopened`, `conversation.seen_by_team` (03, widget-safe), `ai.handoff_requested`, `ai.action_call_updated` (07), `contact.updated`, `contact.merged`, `contact.deleted`, `member.availability_changed` (availability cache bust), `system.app_url_changed` (11; bust widget config cache + install snippets so loader/asset URLs use new `app.url`), `changelog_entry.published` / `changelog_entry.unpublished` (13; config bust: `features.changelog.spotlight`, `latest_announced_at`), `engage_settings.updated` (15; config bust, cache key includes `engage_settings.version`).

New events (add rows to catalog):

| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| widget.created | yes (workspace channel) | no | |
| widget.settings_updated | yes (workspace channel) | no | `changes` keys; drives config cache bust across replicas |
| widget.deleted | yes | no | |

"Seen" receipt: widget never receives `conversation.read` (catalog: member only). 03 emits widget-safe `conversation.seen_by_team` (03 README New events) when member read passes latest workspace message; payload `{conversation_id, seen_at}` only; `seen_by: {name, avatar_url}` added only when workspace setting `seen_receipt_shows_name = true` (03 key, default false). Never member id. Widget shows "Seen" under latest team message with `created_at <= seen_at`.

Widget channel event list (what `widget_contact:{ctc}` carries; history on for stored events, ephemeral events published without history so recovery never replays them): `message.created` (type message, not deleted), `message.deleted`, `message.delivery_updated`, `conversation.created`, `conversation.closed`, `conversation.reopened`, `conversation.seen_by_team` (`{conversation_id, seen_at, seen_by?}`), `typing.started`, `typing.stopped` (author `{type: member|agent, name, avatar_url}`), `ai.handoff_requested` (`{conversation_id}`), `ai.reply_started`, `ai.reply_delta` (ephemeral), `ai.reply_discarded`, `ai.state_changed` (state only; 07 broadcasts widget-safe payloads), `ai.action_call_updated` (07: `{id, conversation_id, title, status}`, never payloads), `contact.updated`, `contact.merged`, `message.created` with `survey_request` (15 CSAT card), `widget.session_revoked` (pseudo-event before disconnect, not in catalog, never stored).

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| SendWidgetOfflineReplyEmail | mail | listener on member/agent `message.created` in widget conversation, `ScheduledAt = now + widget.offline_email_delay` (120 s) | 5, backoff | unique by `conversation_id` while pending (later replies join same email) |
| RevokeWidgetSessionsForSecret | default | 02 `RevokeIdentitySecret` with `revoke_sessions` when over 10,000 sessions | 5 | unique by `identity_secret_id`; batches 1,000, disconnects realtime per batch |
| PruneWidgetSessions | maintenance | daily 03:10 UTC | 3 | periodic singleton; deletes rows revoked or expired over 30 days ago, batches 1,000 per workspace; `RetentionGuard.MayDestroy(ws, {workspace, ws})` per batch, then `{contact, contact_id}` per distinct contact (rows hold `current_page`/`recent_pages`/context); held: rows kept, `retention_held` logged, `dimmi_retention_held_total{kind}` |
| CompleteUpload scan | (03 `ScanAttachment`) | CompleteUpload | per 03 | per attachment |

Retention: widget uploads never attached are purged by 03 `purge_orphan_attachments` (guarded per attachment by 03; rows with `feedback_post_id`/`feedback_comment_id` (14) count as attached, so widget uploads for feedback posts and comments survive). `EraseWidgetContactData` runs inside 03 `EraseContact` (guard checked there once, not again); unchanged by 15 (engage data has own 15 eraser). Feature test: fake guard false keeps session rows.

SendWidgetOfflineReplyEmail worker: re-fetch conversation; skip when contact has no email, contact blocked, conversation spam/deleted, all referenced messages read (`read_at` set), visitor stream connected within last 60 s on contact channel AND messages delivered to it (centrifuge presence on `widget_contact:{ctc}`), or contact unsubscribed from conversation emails. Email body includes every unread member/agent message since last email (max 10). Renders `WidgetOfflineReplyMail`, then calls 04 `email.SendWidgetOfflineReply{ConversationID, ContactID, MessageIDs, HTML, Text, Subject}`: only delivery path (04 adds sender, Reply-To token address so reply threads back, List-Unsubscribe, suppression, limits). 04 `skipped{reason}` (suppressed, limited, disabled) = done, no retry. Marks `messages.metadata.offline_emailed_at` on `sent`. Unverified email recipient: footer "You got this because someone entered this address in chat on {site}. Not you? Ignore." (unsubscribe link and headers from 04).

## Config keys

`config/widget.go`, env prefix `DIMMI_WIDGET_`.

| Key | Default | Notes |
|---|---|---|
| widget.enabled | true | false: all `/widget/v1` return 503 `widget_unavailable` |
| widget.assets_base_url | "" | CDN origin for hashed chunks; empty = same host |
| widget.loader_max_age | 300 s | |
| widget.anonymous_session_ttl | 4320 h (180 d) | sliding |
| widget.max_widgets_per_workspace | 10 | self-host value of entitlement `widget.max_per_workspace` (0 = unlimited); cloud default in catalog (below) |
| widget.max_connections_per_session | 10 | uni-SSE streams per session (one per tab); 11th connect disconnects oldest (custom disconnect code 4004) |
| widget.realtime_token_ttl | 60 s | connect token lifetime; single purpose (uni-SSE connect only) |
| widget.offline_email_delay | 120 s | |
| widget.polling_interval | 5 s | returned to client; 15 s when tab hidden |
| widget.typing_throttle | 2 s | matches foundation |
| widget.bot.block_score | 70 | |
| widget.bot.min_send_ms | 1500 | first message faster than this after session create adds score |
| widget.pow.enabled | true | adaptive: challenge only above `widget.pow.sessions_per_ip_hour` or under attack. Same default self-host (Valkey always deployed; single-use keys there) |
| widget.pow.sessions_per_ip_hour | 20 | above: PoW required |
| widget.pow.max_number | 500000 | ALTCHA `maxnumber` (expected work about half); adaptive up to 8000000 under attack |
| widget.rate.sessions_per_ip_hour | 60 | hard limit, 429 |
| widget.rate.conversations_per_session_hour | 20 | |
| widget.rate.messages_per_session_minute | 60 | foundation |
| widget.rate.requests_per_session_minute | 600 | foundation |
| widget.rate.requests_per_ip_minute | 1200 | foundation |
| widget.rate.uploads_per_session_hour | 30 | |
| widget.rate.contact_updates_per_session_day | 20 | email changes max 5/day |
| widget.config_cache_ttl | 60 s | otter (per replica), plus Valkey PubSub bust |
| widget.install_flush_interval | 60 s | install tracker flush (`last_boot_at`, origins seen/blocked) |

Self-host: every rate limit configurable, `0` disables.

Config resource blocks owned by other specs (01 serves, cache key and ETag include their versions):

| Block | Shape | Owner |
|---|---|---|
| `features.changelog` | `{enabled, title, latest_announced_at, spotlight: {id, title, summary, cover_url, published_at} \| null}` from 13 `changelog.WidgetConfigFragment` | 13 |
| `features.feedback` | `{enabled, boards_count, roadmap}`; label in `texts.feedback_tab_label` | 14 |
| `engage` | `{active, anonymous, version, refresh_interval_ms}` | 15 |
| `features.intercom_shim` | boolean (from `widget_settings.intercom_shim`) | 01 (mapping 16) |

Entitlement keys (catalog `app/support/entitlements/catalog.go`, contracts.md `Entitlements`; read via `Entitlements.Limit`, -1 = unlimited; cloud overrides via 12):

| Key | Kind | Self-host | Cloud default | Enforced in |
|---|---|---|---|---|
| `widget.max_per_workspace` | limit | config `widget.max_widgets_per_workspace` (10) | 10 | `CreateWidget` (count non-deleted under workspace row lock): 422 `entitlement_limit_reached` `{key, limit, used}` (contracts.md shared code); never deletes when limit drops |

Test conversation cap (D10) reads 12 key `billing.test_mode_cap` (cloud 1,000 per billing account period; self-host -1 = no cap, no billing). Abuse rate limits (`widget.rate.*` on Valkey rate limiter, PoW, connections) stay config: protection, not contract.

## Branding rule

`ResolveBranding(workspace, widget)`:

| Edition | Paid ever (12 `BillingStatus.HasPaid`) | `show_branding` | Badge |
|---|---|---|---|
| self-host | n/a | null or false | hidden |
| self-host | n/a | true | shown (opt-in, links trydimmi.com) |
| cloud | no | any | shown, `locked: true` (PATCH `show_branding=false` returns 403 `branding_locked` with message "Badge hides once workspace pays any invoice.") |
| cloud | yes | null or false | hidden |
| cloud | yes | true | shown |

Badge link: `https://trydimmi.com/?utm_source=widget&utm_medium=badge&ref={workspace_slug}`. Config resource: `"branding": {"show": true, "locked": true, "url": "..."}`.

## Permissions matrix

Implemented as permissions (`00-foundation/extensibility.md` section 4, P2). `widget_settings_policy.go` calls `authz.Can(principal, permission)`; role columns derived from 00-accounts `roles.go`, never checked in 01 code. Token effective permissions = role permissions intersect expanded abilities (replaces "token owner must be owner/admin").

01 permissions (registered in 00-accounts catalog `app/auth/permissions.go`; each in exactly one ability group):

| Permission | Ability group | Roles (roles.go) | Gates |
|---|---|---|---|
| `widgets:read` | settings:read | owner, admin, member | list/view widget settings, install snippet, mint test session |
| `widgets:manage` | settings:write | owner, admin | create/update/delete widget, `allowed_origins`, `allow_anonymous`, MCP `dimmi_update_widget_settings` |

| Action | owner | admin | member | Permission (token ability group) |
|---|---|---|---|---|
| list / view widget settings, install snippet | yes | yes | yes | `widgets:read` (settings:read) |
| create / update / delete widget | yes | yes | no | `widgets:manage` (settings:write) |
| mint test session | yes | yes | yes | `widgets:read` (settings:read) |
| change `allowed_origins`, `allow_anonymous` | yes | yes | no | `widgets:manage` (settings:write); audit logged |
| MCP `dimmi_update_widget_settings` | yes | yes | no | `widgets:manage` (settings:write) |

Visitor (widget session) can only touch own contact and conversations where `conversations.contact_id = session.contact_id` and channel type in (`widget`,`email`,`api`), plus imported conversations (channel type `import`, 16) read-only for identified sessions: listed as history, posting to one creates linked new widget conversation (`meta.redirected_from`, same as past reopen window). Anonymous sessions never see imported conversations. Everything else 404.

## Performance budgets

| Path | Budget | How |
|---|---|---|
| config GET | p95 5 ms | otter by `wgt` + `config_version`; ETag 304; CDN `s-maxage=60` |
| handshake (`GET`/`PATCH /session`, identified `POST /sessions`) | p95 30 ms | one resolver call, one session lookup by hash, contact read, 02 identity step under 5 ms; writes throttled (last_seen 60 s); install tracking in memory |
| message send | p95 80 ms | single tx via 03 action; broadcast after commit |
| updates (poll) | p95 20 ms | index `(workspace_id, conversation_id, created_at)` + contact's conversations (max 100) |
| realtime token | p95 10 ms | session lookup (already resolved by auth:widget) + JWT sign; no DB write |
| SSE streams | 1 stream = 1 server-side subscription; 10k visitors under 150 MB RSS (foundation load test; centrifuge about 27 to 35 KB per idle connection) | |
| widget assets | served from embed.FS memory, zero disk IO | |

## Observability

Metrics (Prometheus): `dimmi_widget_handshakes_total{step=resume|identify|refresh|signout|anonymous|test,outcome}`, `dimmi_widget_install_flushes_total{result}`, `dimmi_widget_sessions_created_total{kind,test}`, `dimmi_widget_messages_total`, `dimmi_widget_origin_rejections_total`, `dimmi_widget_bot_blocks_total{reason}`, `dimmi_widget_pow_challenges_total{result}`, `dimmi_widget_realtime_tokens_total`, `dimmi_widget_realtime_recoveries_total{recovered}`, `dimmi_widget_poll_requests_total`, `dimmi_widget_offline_emails_total{result}`, `dimmi_widget_asset_bytes_total{file}`, `dimmi_widget_upload_failures_total{reason}`.

Logs: app id, session public id, contact public id, outcome, error code. Never token, never JWT, never message body at info. Access log redacts `cf_connect` query param on `/connection/uni_sse`.

## Security and privacy

- Tokens: `dmi_ws_` bearer in `Authorization` header (not cookie): no CSRF, no third-party cookie dependence. Stored in host-origin `localStorage` by SDK; host-page XSS can steal it (same as host session), documented.
- Realtime connect token in URL (EventSource sends no headers): separate JWT, 60 s, audience realtime connect only (rejected by REST guards), `sub` = `wss_` session, carries server-side subscription; session revoke check repeated in `OnConnecting`. Visitor session token never in URL. `cf_connect` redacted in logs.
- Identity tokens never inlined in host HTML; minted per request by identity source (02 snippets.md, security.md).
- Origin allowlist: browser defense against foreign sites embedding widget, not authentication (non-browsers can spoof). Identity security comes from JWT (02).
- CORS: reflect origin only if allowed; credentials not allowed (`Access-Control-Allow-Credentials` absent); preflight `Max-Age: 7200`.
- Widget-safe resources: no notes, no internal events, no tags, no assignee, no member email, no AI citations internals beyond public article links, no other participants' emails (names only).
- `current_page` URL query scrubbed of secret-like keys before storage.
- IP stored as prefix only; full IP used in memory for rate limits.
- Message bodies sanitized: Markdown subset from visitor rendered with safe renderer; HTML tags in visitor text treated as text.
- Attachments: SVG and HTML forced `Content-Disposition: attachment`; type sniffed at complete; executables blocked.
- Lead email unverified: never merges, never reveals history (02 rules).
- PoW challenge: ALTCHA (altcha-lib-go) HMAC-signed with key derived from app key, salt params carry IP prefix + expiry 2 minutes; single use (Valkey `SET NX EX`).
- Suspended or deleted workspace: config 403/404; no data returned.

## Metering impact (pricing.md section 2)

| Rule | Effect here |
|---|---|
| D1/D2 | widget welcome/greeting never stored as message. "We'll reply by email" notice stored as `type event` or `substantive=false`. Offline email is delivery of existing reply, not new reply |
| D9 | honeypot hit, `bot_score >= block_score`, rate-limited sessions: conversation `spam = true` before any reply, never billable. Blocked contact: stored spam |
| D10 | `widget_sessions.test` -> `InboundMessage.IsTest` -> 03 `conversations.is_test = true` -> never billable. Sources: dashboard test token, verified JWT email equal to active member email with `email_verified=true`, `workspaces.test_mode`. Cap: entitlement `billing.test_mode_cap` (cloud 1,000 test conversations per billing account per month, 12 counter; self-host none); above cap `is_test` not set and API returns `meta.test_quota_exceeded = true` (billing applies normally) |
| D12 | lead-to-user merge on login (02) is contact merge, not thread merge; no credit needed |
| D5 | widget same price as other channels |

## Dependencies

| Spec | Needs |
|---|---|
| 00-foundation | `Entitlements` contract + catalog (01 registers `widget.max_per_workspace`), `RetentionGuard`, registry enums (`channel_connections.type`, `contact_identities.type`), auth:widget middleware slot, `framework/broadcasting` (embedded centrifuge, Redis broker + presence on Valkey, uni-SSE handler `/connection/uni_sse`, connection JWT verify in `OnConnecting`, server-side subscriptions, history + recovery, `Disconnect` by user id across nodes), Valkey (always deployed: cache, rate limits, pubsub, single-use keys `SET NX EX`), `framework/http/origin`, `framework/view/vite` + `static`, rate limiter, signed URLs, Filesystem presign capability |
| 00-accounts | `authz.Can` + permission catalog (01 permissions `widgets:read`, `widgets:manage` registered there), member availability, member avatars, audit log writes, workspace suspended flag, workspace initializer hook (contracts.md): 01 registers `EnsureDefaultWidget` (after 03 `EnsureDefaultInbox`) |
| 02-widget-identity | `identity.VerifyIdentityToken`, `identity.IdentifyVisitor` (upsert user contact, merge lead), `identity_secrets` table, session TTL, error codes |
| 03-shared-inbox | `conversations.ReceiveInboundMessage` (contracts.md canonical name: create/append, reopen window, number allocation, `conversation.created`/`message.created`), `conversations.is_test` column, `conversation.seen_by_team` event, default inbox (initializer runs first), `contacts.MergeContacts`, attachments contract (`CreatePendingUpload`/`CompleteUpload` shared, `ScanAttachment` job), contact block, contacts REST identify (server-to-server) |
| 04-email-channel | `email.SendWidgetOfflineReply` (only delivery path for offline reply: sender, Reply-To token address, List-Unsubscribe, suppression, limits, customer stream), verifies email on inbound reply |
| 05-chat-app-channels | generic `channel_connections` CRUD lists widget type read-only (created/deleted only via 01) |
| 06-notifications | skip member notifications for `is_test` conversations except to minting member |
| 07-dimmi-agent | agent replies as `message.created` author agent; agent typing via `typing.started` on widget channel; `ai.reply_*` streaming + `ai.state_changed` + `ai.action_call_updated` on widget channel; widget handoff controller + `HandOffConversation` action and action-call controller (01 registers routes only); lang keys `action_confirm`, `action_cancel`. 01 gives: `metadata.widget.identity_verified` stamp, lead PATCH clears `trusted_attribute_keys` for written keys |
| 08-help-center | owns `/widget/v1/help/*` controllers, resources, search; `help_centers` FK. 08 must accept app-only auth (`app_id`, no session) so anonymous visitors see Help tab |
| 09-agent-native | `npx dimmi init` consumes install endpoint; MCP server hosts tools |
| 10-webhooks | webhook payloads include `conversation.is_test`, `source.widget_id` |
| 11-self-host | edition flag; asset embedding in binary; `dimmi doctor` checks widget assets present |
| 12-cloud-billing | `BillingStatus.HasPaid` (resolves workspace -> org -> billing account), ledger excludes `is_test` and spam conversations, test cap counter (per billing account) + entitlement key `billing.test_mode_cap` |
| 13-changelog | `/widget/v1/changelog*` controllers, `changelog.WidgetConfigFragment`, session `changelog {last_seen_at, unread_count}`, `changelog_entry.published/unpublished` for config bust |
| 14-feedback-roadmap | `/widget/v1/feedback/*` controllers, `feedback_tab_enabled` migration `001421`, config `features.feedback`, `feedback_unread_count`, `RevokePortalSessions` (01 calls on session revoke) |
| 15-announcements-surveys | engage widget routes, config `engage` block, `BumpWidgetConfigForEngage`, `announcement_id` validation on conversation create, `survey_request` system messages via 03 `SendSystemMessage` |
| 16-importers | Intercom shim mapping table (`16-importers/sources.md`), import records lookup for `filter[intercom_id]` (via 08 help route) |

## Contradictions noted with foundation and research

- `events-and-realtime.md` marks `conversation.read` realtime "member only". Resolved: `conversation.read` stays member only; widget "Seen" uses separate 03 event `conversation.seen_by_team` (timestamp only, name only with workspace setting). Catalog adds row.
- `research/tech-stack.md` says widget realtime "WebSocket with SSE/long-poll fallback". Resolved (research/build-vs-reuse.md section 1): widget uses centrifuge uni-SSE only (receive), REST for writes; long-poll (`GET /widget/v1/updates`) only when proxy blocks or buffers streaming. No WebSocket, no centrifuge-js in widget.
- `research/tech-stack.md` suggests iframe for "identity verification" surfaces. Not needed: JWT verified server side; widget never handles secrets.
- Size budget wording: brief says "loader under 35 KB gz"; tech-stack says loader under 3 KB and widget under 35 KB. This spec adopts both: loader at most 3 KB, core at most 35 KB.
- `data-model.md` ownership map lists only `widget_settings`, `widget_sessions` for 01 and `identity_secrets` for 02; 02 adds no other table (nonce replay cache in Valkey; settings/stats tables deferred with identity-validations).
- `security.md` "visitor session token random 32 bytes, stored hashed": kept; token transport is header bearer (not cookie), so no `SameSite`/CSRF concerns apply.

## Open questions (recommended default in bold)

1. Anonymous session lifetime: 180 days sliding vs 30 days for privacy. **180 days** (Safari wipes after 7 idle days anyway; leads value continuity).
2. Custom widget API domain (CNAME + TLS) at launch vs first-party proxy only. **Proxy only at MVP**; revisit when ad-block reports appear.
3. Offline reply email delay: 120 s vs 5 minutes. **120 s**, configurable.
4. Widget test conversation cap: shares D10 sandbox cap of 1,000/month. **Yes, shared counter** (12 confirms; entitlement `billing.test_mode_cap`, counter per billing account).
