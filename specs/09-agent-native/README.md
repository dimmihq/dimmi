# 09 Agent-native: OAuth, MCP, CLI

Style: caveman. Date 2026-11-01. Scope trim 2026-11-02: reply drafts (B9), agent send modes (B8), `dimmi listen` (C6), identity helper packages beyond Next.js, React, browser, `@dimmi/server` (C16) moved to Out (later); MCP core tools 22 to 19; every MCP send needs human confirmation. Backend + CLI only. Consent and "connected apps" UI come later; every API they need is here.

Files in this folder:

| File | What |
|---|---|
| README.md | this: goal, scope, data model, code map, HTTP API, events, jobs, config, permissions, budgets |
| [oauth.md](oauth.md) | authorization server protocol detail: metadata docs, flows, token rules, client registration (CIMD, DCR, pre-registered), consent rules |
| [mcp.md](mcp.md) | MCP server: transports, protocol versions, auth, tool catalog with schemas, confirmation (MRTR elicitation), output budget, resources, prompts, errors |
| [cli.md](cli.md) | `dimmi` CLI: distribution decision, auth, commands, `init` framework matrix, docs-as-code, install-by-prompt deliverables (Claude Code skill, Cursor rule, AGENTS.md, llms.txt) |
| [edge-cases.md](edge-cases.md) | decided behavior for 252 edge cases (incl. first-party apps, section O; extensibility seams, section P) |
| [tasks.md](tasks.md) | build checklist, demo script |

## Goal

Make Dimmi best support tool to drive from Claude Code, Cursor, any MCP client, and terminal. One OAuth 2.1 authorization server, one MCP server (`/mcp` + `dimmi mcp` stdio), one `dimmi` CLI. `npx dimmi init` takes repo from zero to widget + secure identity + seeded help center in under 2 minutes, on cloud or self-host.

Why: roadmap launch item 9 (`research/roadmap.md` "9. Agent-native: CLI, MCP, API"). Audience lives in editor. Fin CLI ships (US-only, ~14-day tokens, no browser OAuth). Fin MCP cannot reply to customers. Featurebase MCP has 168 tools (context bloat). We beat all three: any host, non-expiring scoped auth, read + write with human confirmation, 19 sharp core tools plus opt-in toolsets.

## World-class bar (measurable)

| # | Bar | Target | How measured |
|---|---|---|---|
| 1 | `npx dimmi init` on fresh Next.js App Router repo to SDK installed + `dimmi.config.ts` / `withDimmi` written (identity via identity source, no JWT endpoint file; `01-messenger-widget/sdk-contract.md`) | median under 90 s incl. device approval click; p95 under 2 min | e2e test with scripted approval, timed in CI; human benchmark video vs Fin CLI |
| 2 | `npx dimmi --version` cold (empty npm cache, 50 Mbps) | under 4 s | CI job on clean runner |
| 3 | CLI binary startup (`dimmi whoami` with warm token, excluding network) | under 30 ms | `hyperfine` in CI |
| 4 | Connect Claude Code: `claude mcp add --transport http dimmi https://app.trydimmi.com/mcp`, approve in browser, first tool call | under 30 s, zero manual client registration | manual QA per release, scripted with MCP conformance client |
| 5 | Works with self-host URL and any cloud region host | same commands, only `--host` differs | e2e matrix: cloud staging + `docker compose` self-host |
| 6 | Auth never silently expires for active user | refresh rotation, 90-day sliding idle window, no absolute expiry; PAT option with no expiry | integration tests (real Postgres + Valkey) + 100-day clock-forward test |
| 7 | MCP `tools/list` size | under 6,000 tokens total for all 19 core tools | CI estimator (chars/4) fails build over budget |
| 8 | Default tool response size | under 8,000 tokens, hard cap 20,000 | renderer enforces; CI fixture with 1M-message workspace |
| 9 | Read tool latency server-side | p95 under 80 ms (1M-message workspace) | benchmark job |
| 10 | Token check overhead | p95 under 1 ms (cache hit), under 5 ms (miss) | benchmark |
| 11 | No customer-visible send without explicit human approval in MCP | 100% of `dimmi_send_reply` paths gated (elicitation or `confirm` param; no bypass mode); no MCP tool publishes, accepts, or deletes articles (08 rule) | feature tests per path |
| 12 | MCP spec conformance | passes official MCP conformance suite (server, 2026-07-28) and inspector smoke on 2025-11-25 and 2025-06-18 | CI |
| 13 | OAuth correctness | PKCE S256 mandatory, exact redirect match, RFC 9207 `iss`, RFC 8707 audience, refresh rotation with reuse detection | feature tests per RFC clause (table in oauth.md) |
| 14 | Every CLI command supports `--json`; every write supports `--dry-run` | 100% | CLI test asserts flag present on every command |
| 15 | Errors actionable | every CLI/MCP error has stable `code`, human message, and next step | snapshot tests |

## Scope

### In (MVP)

- OAuth 2.1 authorization server: authorization code + PKCE (S256 only), device authorization grant (RFC 8628) for CLI, refresh tokens with rotation, token revocation (RFC 7009), authorization server metadata (RFC 8414), protected resource metadata (RFC 9728) for `/mcp` and `/api/v1`, resource indicators (RFC 8707), issuer in authorization response (RFC 9207).
- Client registration: pre-registered system clients (`dimmi-cli`, `dimmi-mobile`), Client ID Metadata Documents (CIMD, primary per MCP 2026-07-28), Dynamic Client Registration RFC 7591 (deprecated by MCP spec, kept for clients that still use it).
- First-party apps (Dimmi mobile, later desktop): `dimmi-mobile` system client, user-scoped role-based grants, `auth:app` guard resolution, claimed HTTPS redirect + association files (`/.well-known/apple-app-site-association`, `/.well-known/assetlinks.json`), `max_age`/`prompt=login` re-auth for sudo, revocation hooks for 00-accounts. Detail: oauth.md "First-party apps".
- Consent API (authorization requests, device requests), connected-apps API (grants list + revoke), workspace agent-access settings.
- `oauth_*` tables, `oauth_grants`, `oauth_authorization_requests`.
- Scoped tokens: OAuth scopes = abilities from `00-foundation/api.md` = permission groups (one vocabulary with PAT abilities and 00-accounts permission catalog `app/auth/permissions.go`; `00-foundation/extensibility.md` sections 4, 11). Effective permissions = role permissions (`roles.go`) intersect expanded granted scopes, evaluated each request. Marketplace apps later request same strings.
- Client version gate on `/oauth/token` (`client.version` middleware, foundation): too-old first-party app or CLI refused (OAuth error shape, below). CLI sends `Dimmi-Client-Version: cli/<ver>` and `Dimmi-Version` on every request.
- MCP server: streamable HTTP at `/mcp` (protocol 2026-07-28 stateless + legacy 2025-03-26..2025-11-25), stdio via `dimmi mcp` (bridge mode and in-process mode). 19 core tools (plus opt-in feature toolsets), 4 resources, 4 prompts. Confirmation via MRTR form elicitation, fallback `confirm: true`; required on every `dimmi_send_reply` (no workspace bypass).
- CLI client commands in same Go codebase, published to npm as `dimmi` (platform binaries via optionalDependencies), Homebrew, install script, Docker image (full flavor).
- CLI commands: `login`, `logout`, `whoami`, `profiles`, `init`, `conversations list|show|reply|close`, `articles pull|push`, `changelog`, `feedback`, `announcements`/`surveys`/`segments` (13-15, cli.md), `tokens create|list|revoke` (wraps 00-accounts PAT API), `mcp`, `agents install`, `api` (raw), `version`.
- Install-by-prompt deliverables: Claude Code skill, Claude Code plugin (skill + MCP config), Cursor rule, AGENTS.md snippet, copy-paste prompt for Lovable/Bolt/v0, `llms.txt` + `llms-full.txt` for Dimmi docs. Contents specified in `cli.md`.
- OpenAPI as single schema source: MCP tool input schemas from same Go request types that feed huma; CLI API client generated from `openapi.json`; drift tests.

### Out (later)

| Item | When |
|---|---|
| Workspace-registered confidential OAuth apps ("Sign in with Dimmi" for third parties), RFC 7592 client management | later |
| Token introspection endpoint (RFC 7662), JWT access tokens | later; resource server in-process |
| `private_key_jwt` client auth for CIMD clients | later |
| OpenID Connect (id_token, userinfo, `/.well-known/openid-configuration`) | later |
| `subscriptions/listen` change notifications on MCP | later (catalog static) |
| MCP tasks extension (long-running tool calls) | later |
| Public read-only help center MCP for end customers' agents | in MVP, owned by 08 (`rendering.md` "Public MCP", separate unauthenticated endpoint, own 3 tools) |
| MCP client inside Dimmi Agent (agent calls other MCP servers) | launch: agent actions (owned there, not 09) |
| Agent evals CLI `dimmi test` | post-MVP: agent evals |
| `npx dimmi migrate intercom` | launch: importers (16 owns wizard `app/console/commands/client/migrate_command.go`; 09 provides client mode, hidden-input prompt helper `app/client/prompt`; cli.md `dimmi migrate`, `dimmi imports`) |
| GitHub PR mode for docs-as-code (Dimmi opens PR into docs repo) | later (`docs-as-code-pr`, owner 08); launch: self-updating docs ships review-queue suggestions (08 docs-sync.md, 09 toolset `docs_sync`) |
| Bulk tools (bulk close/tag via MCP) | later; REST bulk `PATCH /conversations {ids, changes}` exists (03) |
| Image upload in `articles push` (relative image links) | later; MVP warns and leaves link |
| Priority, spam, merge tools on MCP | later; REST has them (`PATCH /conversations/{conv} {priority}` / `{spam: true}`, `POST /conversations/{target}/merges {from}`) |
| CLI telemetry | never by default |
| reply-drafts (audit B9): server-side `reply_drafts` (REST `/conversations/{conv}/drafts`, `/drafts`, MCP `dimmi_list_drafts`/`dimmi_draft_reply`/`dimmi_discard_draft`, CLI `drafts`, `draft_id` on send, `reply_draft.*` events) | later |
| agent-send-modes (audit B8): workspace `agent_access.send_mode` `confirm`/`drafts_only`/`direct`; MVP = always confirm every MCP send | later |
| dimmi-listen (audit C6): `dimmi listen --forward-to` local webhook forwarding (10 server side also later) | later |
| identity-helper-packages (audit C16): SDK/identity framework packages beyond Next.js, React, browser, `@dimmi/server` (SvelteKit, Nuxt, Astro, Vue, Svelte, Laravel, gem, pip, Go); MVP `init` writes 02 raw snippet route for those stacks | later |

## User stories

Workspace member (solo dev):
- Run `npx dimmi init` in repo, approve once in browser, get SDK + `dimmi.config.ts` with identity wired through my framework's identity source (raw snippet route on stacks without package) and draft help center, no dashboard visit needed.
- In Claude Code say "reply to #1042 with the refund steps"; approve the exact text in Claude Code dialog before it sends.
- Run `dimmi conversations list --assignee me` and `dimmi conversations reply 1042 --file reply.md` from terminal.
- Keep docs in `docs/help/*.md`, run `dimmi articles push` in CI; edits to published articles land in review queue.
- See every connected app (Claude Code, Cursor, CLI on laptop) and revoke one; get email when new app connects.
- Sign in to Dimmi mobile app through system browser sheet (existing web login, no password typed into app); see phone in session list and sign it out remotely.

Workspace admin:
- Block dynamically registered clients for workspace; see and revoke any member's grants.
- Create non-expiring scoped token for CI (`dimmi tokens create --abilities articles:write --no-expiry`).

Developer (integrator):
- Script against `--json` output with stable exit codes. Use `dimmi api GET /conversations` for anything not wrapped.
- Self-host: `dimmi login --host https://support.acme.dev` same flow.

AI agent (Claude Code, Cursor, Codex, ChatGPT connector):
- Discover auth via 401 + protected resource metadata, register via CIMD or DCR, get scoped token, call tools.
- Get concise, ID-rich text plus structured content; paginate with cursor; never blow context.
- Ask human before sending: elicitation dialog shows recipient, channel, body.

End customer: never sees difference. Replies sent from MCP/CLI arrive like any teammate reply, under teammate's name.

## Data model

All tables: `id uuid primary key default uuidv7()`, `created_at`, `updated_at` unless stated. Tenant tables lead indexes with `workspace_id` and get RLS in cloud. Global tables (not tenant-scoped) marked GLOBAL: RLS not applied; access only via OAuth actions; never exposed through tenant queries.

Secrets stored as SHA-256 hash (`*_hash bytea`), never plaintext, never encrypted-reversible (no need to read back). Token prefixes for secret scanning: see `oauth.md`.

Framework storage (2026-11-02, binding over tables below): `framework/oauth` migration (framework set) creates `oauth_clients` (text `id` = client_id), `oauth_authorization_codes`, `oauth_grants` (`tenant_id text`, `first_party`, `auth_time`, `confirmed_at`, `revoked_at`), `oauth_tokens` (`kind` access|refresh, `parent_id`, `used_at`). Mapping: `oauth_auth_codes` = `oauth_authorization_codes`; `oauth_access_tokens` + `oauth_refresh_tokens` = `oauth_tokens`. 09 app migrations `ALTER TABLE` these to add product columns (`workspace_member_id`, device fields, `revoked_reason`, `auth_methods`, `resources`/`audiences` once RFC 8707 lands, client `kind`/metadata/CIMD cache, RLS policy on `tenant_id`), never `CREATE` them. `oauth_authorization_requests` and `oauth_device_codes` stay 09 tables (device grant deferred in framework).

### oauth_clients (GLOBAL)

| Column | Type | Notes |
|---|---|---|
| id | uuid | public id `ocl_` |
| client_id | text not null unique | `dimmi-cli`, `dimmi-mobile` (system), `dcr_<26 base32>` (dynamic), full HTTPS URL (metadata document) |
| kind | text not null CHECK in ('system','dynamic','metadata_document') | |
| first_party | boolean not null default false | true only for seeded first-party system clients (`dimmi-mobile`); CHECK `NOT first_party OR kind = 'system'`; never set by DCR, CIMD or commands |
| name | text not null | 1..100 chars, NFC, control chars stripped |
| client_uri | text | https only |
| logo_uri | text | https only; proxied through image proxy on consent page, never hotlinked |
| redirect_uris | text[] not null default '{}' | max 10, each max 2,048 chars; system clients may use `{app_url}` placeholder (resolved to APP_URL at match time) |
| grant_types | text[] not null | subset of `authorization_code`, `refresh_token`, `urn:ietf:params:oauth:grant-type:device_code` |
| token_endpoint_auth_method | text not null CHECK in ('none','client_secret_basic','client_secret_post') | |
| client_secret_hash | bytea | null for `none` |
| application_type | text not null default 'native' CHECK in ('native','web') | |
| software_id, software_version | text | from DCR, informational |
| metadata | jsonb not null default '{}' | raw registration or fetched document, max 16 KB |
| metadata_fetched_at, metadata_expires_at | timestamptz | CIMD cache window |
| metadata_etag | text | conditional refetch |
| registration_ip | inet | DCR only, abuse tracing |
| last_used_at | timestamptz | updated at most once per hour |
| disabled_at | timestamptz | kill switch (abuse, admin command) |
| disabled_reason | text | |

Indexes: unique `client_id`; `(kind, last_used_at)` for prune.

### oauth_authorization_requests (GLOBAL until approved)

Pending browser authorization between `/oauth/authorize` and consent decision.

| Column | Type | Notes |
|---|---|---|
| id | uuid | public id `oar_`; appears in consent URL |
| oauth_client_id | uuid not null references oauth_clients on delete cascade | |
| user_id | uuid null references users on delete cascade | bound on first authenticated view; other user viewing gets 404 |
| redirect_uri | text not null | exact value presented |
| scopes | text[] not null | requested, normalized |
| resources | text[] not null | RFC 8707 targets, canonical |
| state | text | max 1,024 chars, echoed |
| code_challenge | text not null | 43..128 chars |
| code_challenge_method | text not null CHECK in ('S256') | |
| status | text not null default 'pending' CHECK in ('pending','approved','denied','expired') | |
| decided_workspace_id | uuid null references workspaces on delete cascade | |
| request_ip | inet, user_agent text | shown on consent screen |
| device_name | text | `device_name` authorize param (first-party), max 100 chars |
| device_id | text | `device_id` authorize param (first-party app install id: random UUID kept in keychain/keystore), max 64 chars `[A-Za-z0-9-_]`; null when absent; copied to grant |
| auth_methods | text[] not null default '{}' | session `auth_methods` snapshot taken with `auth_time` (e.g. `{password,totp}`); copied to grant |
| max_age | int | requested `max_age` (null when absent; `prompt=login` stored as 0) |
| auth_time | timestamptz | `sessions.authenticated_at` (00-accounts column: last full login incl. factors) when request created (after any forced re-login); copied to grant |
| expires_at | timestamptz not null | created + 10 minutes |

Index: `(status, expires_at)`.

### oauth_device_codes (GLOBAL until approved)

| Column | Type | Notes |
|---|---|---|
| id | uuid | public id `odr_` |
| oauth_client_id | uuid not null references oauth_clients on delete cascade | |
| device_code_hash | bytea not null unique | |
| user_code | text not null | normalized 8 chars `[BCDFGHJKLMNPQRSTVWXZ]`, displayed `XXXX-XXXX` |
| scopes, resources | text[] not null | |
| device_name | text | from CLI (`hostname`), max 100 chars, shown on consent |
| request_ip | inet, user_agent text | |
| status | text not null default 'pending' CHECK in ('pending','approved','denied','expired','consumed') | |
| user_id | uuid null references users on delete cascade | bound on first successful lookup; other users then get empty lookup / 404 |
| workspace_id | uuid null references workspaces on delete cascade | set on approve |
| oauth_grant_id | uuid null references oauth_grants on delete set null | set on approve |
| granted_scopes | text[] | |
| interval_seconds | int not null default 5 | grows on `slow_down` (persisted so restart keeps interval; per-poll rate check lives in Valkey, below) |
| last_polled_at | timestamptz | written at most once per 30 s; fast `slow_down` decision = Valkey `SET odr:{id}:poll NX PX <interval>` (rebuildable; Valkey loss = one extra poll accepted) |
| lookup_failures | int not null default 0 | |
| expires_at | timestamptz not null | created + 15 minutes |

Indexes: unique `user_code` WHERE status = 'pending'; `(status, expires_at)`.

### oauth_grants (tenant)

One row per (user, workspace, client) authorization, or per (user, device login) for first-party apps. Token family root. Connected-apps list (workspace grants) and session list `kind: app` (first-party grants).

| Column | Type | Notes |
|---|---|---|
| id | uuid | public id `ogr_` |
| workspace_id | uuid null | null only for first-party grants (CHECK below) |
| user_id | uuid not null references users on delete cascade | |
| workspace_member_id | uuid null references workspace_members on delete cascade | null only for first-party grants |
| oauth_client_id | uuid not null references oauth_clients on delete cascade | |
| first_party | boolean not null default false | copied from client at creation; drives `auth:app` resolution |
| scopes | text[] not null | granted (subset of requested, clamped to role at grant time); first-party: exactly `{first_party}` |
| resources | text[] not null | audiences allowed; first-party: `{APP_URL/api/v1}` |
| origin_grant_type | text not null CHECK in ('authorization_code','device_code') | |
| auth_time | timestamptz | user's last full login before this grant (`sessions.authenticated_at`); sudo freshness for first-party; never changed by refresh |
| confirmed_at | timestamptz | last sudo confirmation by app via `POST /user/confirmation` (00-accounts) through 09 `ConfirmGrant`; first-party only; never changed by refresh. Principal `ConfirmedAt`; `password.confirm` uses `max(auth_time, confirmed_at)` |
| confirmed_method | text | method of last confirmation (`password`, `email_code`, factor type e.g. `totp`); registry format CHECK; later "sudo needs phishing-resistant" policy reads it |
| auth_methods | text[] not null default '{}' | how user proved identity for this grant (P3): copied from approving session `auth_methods` at approval (authorization request snapshot; device flow: approving session at `PATCH /oauth/device-requests`). Never changed by refresh. Read later by `AccessPolicy` rules (phishing-resistant MFA, SSO only) |
| device_id | text | stable device identity (P3, extensibility.md section 5): first-party = authorize `device_id` param; third-party and device flow = null at MVP. Joins grant, session (`sessions.device_id`) and push rows (06 `push_subscriptions.device_id`) for "revoke this device" later |
| device_name | text | CLI host name, client name, or app `device_name` param |
| device_platform | text CHECK in ('ios','android','macos','windows','linux','unknown') | first-party; from token endpoint User-Agent |
| app_version | text | first-party; from User-Agent, updated on refresh, max 32 chars |
| user_agent | text | last token endpoint User-Agent, max 255 chars |
| last_used_at | timestamptz | buffered update, at most once per minute |
| last_used_ip | inet | |
| revoked_at | timestamptz | |
| revoked_reason | text CHECK (revoked_reason ~ '^[a-z][a-z0-9_]{1,39}$') | registry enum (P1; Go registry `enums.GrantRevokedReason`, validated in revoke actions). MVP values: `user`, `admin`, `user_deleted`, `member_deactivated`, `role_changed`, `refresh_reuse`, `client_disabled`, `code_reuse`, `workspace_deleted`, `idle`, `logout_others`, `password_changed`, `password_reset`, `two_factor_disabled`, `pre_hijack_defense`. `idle` = no refresh for 90 days (prune job); last five from 00-accounts hooks (first-party only; `pre_hijack_defense` = unverified account auto-linked by verified OAuth email, 00-accounts flows 5). Later (`device_revoked`, `policy_violation`, `sso_deprovisioned`) = registry addition, no migration |

CHECK `oauth_grants_first_party_shape`: `(first_party AND workspace_id IS NULL AND workspace_member_id IS NULL AND scopes = '{first_party}' AND origin_grant_type = 'authorization_code') OR (NOT first_party AND workspace_id IS NOT NULL AND workspace_member_id IS NOT NULL AND NOT 'first_party' = ANY(scopes))`.

Indexes: `(workspace_id, user_id, revoked_at)`, `(workspace_id, oauth_client_id)`, `(user_id, last_used_at desc) WHERE first_party AND revoked_at IS NULL` (session list, revoke-all). Multiple active grants per (user, workspace, client) allowed (two laptops); re-consent from same device does not merge.

RLS: tenant policy `workspace_id = current_setting('app.workspace_id')::uuid` hides first-party rows (null workspace) from every tenant query. First-party rows reached only through SECURITY DEFINER functions: `oauth_find_access_token`, `oauth_user_app_grants(user_id)`, `oauth_revoke_user_app_grants(user_id, reason, except_grant_id)`, `oauth_revoke_app_grant(user_id, grant_id, reason)`. Tenant lint exempts these four (annotated `-- lint:user-scoped`).

### oauth_auth_codes (tenant)

`id, workspace_id (null for first-party grant), oauth_grant_id not null references oauth_grants on delete cascade, code_hash bytea unique not null, redirect_uri text not null, code_challenge text not null, resources text[] not null, scopes text[] not null, expires_at timestamptz not null (60 s), used_at timestamptz, created_at`. No `updated_at` (immutable except `used_at`).

### oauth_access_tokens (tenant)

`id, workspace_id (null for first-party grant), oauth_grant_id not null references oauth_grants on delete cascade, token_hash bytea unique not null, scopes text[] not null, audiences text[] not null, expires_at timestamptz not null, revoked_at timestamptz, created_at`. Index `(workspace_id, oauth_grant_id)`, `(oauth_grant_id)`, `(expires_at)` for prune. `auth_time` not copied: lookup function returns grant `auth_time`.

### oauth_refresh_tokens (tenant)

`id, workspace_id (null for first-party grant), oauth_grant_id not null references oauth_grants on delete cascade, token_hash bytea unique not null, parent_id uuid null references oauth_refresh_tokens on delete set null, expires_at timestamptz not null (idle window), used_at timestamptz, revoked_at timestamptz, created_at`. Index `(workspace_id, oauth_grant_id)`, `(oauth_grant_id)`.

Token and code rows: `workspace_id` null iff grant `first_party`. Actions copy it from grant row (never from request); feature test asserts invariant for every issue path.

### Core table changes

| Table | Change | Migration |
|---|---|---|
| none (schema) | No new columns on core tables. | |
| workspaces.settings (jsonb) | New typed key `agent_access`: `{"allow_dynamic_clients":true,"allow_metadata_document_clients":true}`. Add to `models.WorkspaceSettings`; declared in app settings registry (`app/support/settings`, backed by `config.RuntimeStore`, key `agent_access`, scope workspace; later org may lock it). No migration (jsonb). | none |
| messages.metadata (jsonb) | Key `sent_via` = contracts.md `Actor.Via` plus 09 extras: `{"kind":"oauth_client\|personal_token\|app","id":"ocl_..\|tok_..\|ogr_..","surface":"mcp\|cli\|api\|app","client_name":"Claude Code","grant_id":"ogr_..","confirmation":"elicitation\|param\|cli_prompt\|none"}` on replies sent through tokens. `kind`/`id` from Via; rest snapshot for display/audit. | none |

### Migrations

| File | Creates |
|---|---|
| `database/migrations/20261101000901_alter_oauth_clients_table.sql` | ALTER framework oauth_clients (kind, metadata, CIMD cache columns, disabled_at) + seed system clients `dimmi-cli`, `dimmi-mobile` (or via `oauth.Options.Clients`) |
| `database/migrations/20261101000902_alter_oauth_grants_table.sql` | ALTER framework oauth_grants (nullable `workspace_id`/`workspace_member_id`, `first_party`, `auth_time`, `auth_methods`, `device_id`, device fields, `confirmed_at`, `confirmed_method`, `revoked_reason` registry-format CHECK, shape CHECK, user index) (+ RLS policy) + SECURITY DEFINER user-scoped functions |
| `database/migrations/20261101000903_create_oauth_authorization_requests_table.sql` | oauth_authorization_requests (incl. `device_name`, `device_id`, `max_age`, `auth_time`, `auth_methods`) |
| `database/migrations/20261101000904_alter_oauth_authorization_codes_table.sql` | ALTER framework oauth_authorization_codes (resources) (+ RLS on `tenant_id`) |
| `database/migrations/20261101000905_alter_oauth_tokens_table.sql` | ALTER framework oauth_tokens (audiences) (+ RLS on nullable tenant) |
| `database/migrations/20261101000907_create_oauth_device_codes_table.sql` | oauth_device_codes |

Token lookup path: auth middleware looks up `oauth_access_tokens` by `token_hash` before tenant is known. Lookup query runs as privileged role function `oauth_find_access_token(hash)` (SECURITY DEFINER, returns workspace_id (nullable) + grant id + user id + scopes + audiences + `first_party` (grant and client) + `auth_time` + `confirmed_at`) so RLS stays on for everything else. Same for refresh and auth code exchange.

No separate first-party migration: tables not shipped yet, so columns live in create migrations above.

## Code map

Directory `app/client/` (CLI client code, no server imports); conventions directory layout adds it (Contradictions 5 resolved). Command naming rule in cli.md (server/admin colon style, client resource commands space style). Depguard rule: `app/client/**` and `app/console/commands/client/**` must not import `app/models`, `app/actions`, `app/drivers`, `database/**`.

### OAuth server

| Path | What |
|---|---|
| `config/oauth.go` | typed config (keys below) |
| framework (no app file) | AS core built in `framework/oauth` (2026-11-02: storage + migration, auth code + PKCE, refresh rotation with family revoke on reuse, revocation, RFC 8414 metadata, DCR, `ConfirmGrant`, bearer resolver for guards `token`/`app`; deferred: RFC 8707 audiences, CIMD, device grant, consent-skip rules, last-use tracking) on reused libs. Original plan: token format `framework/auth/token` (prefix + base62 + CRC32, SHA-256, constant-time compare); `framework/oauth` (`PKCE.VerifyS256`, `Redirects.Match` exact/loopback/private-use, `Resource.Canonical` RFC 8707, `DeviceCode.New`, `ClientMetadataDocument.Fetch` CIMD on `a.HTTP`); AS metadata (RFC 8414), PRM (RFC 9728), DCR request/response/error types and `MatchesResource` from MCP go-sdk `oauthex` (v1.8.0); bearer middleware + PRM handler from go-sdk `auth` (`RequireBearerToken`, `ProtectedResourceMetadataHandler`, `TokenVerifier` = our resolver) |
| `app/auth/oauth/scopes.go` | scope parsing, defaults; scope = ability group string from 00-accounts `permissions.go` (never a copy); clamp = `authz` role permissions intersect expanded scopes |
| `app/auth/oauth/metadata.go` | fills `oauthex.AuthServerMeta` / `oauthex.ProtectedResourceMetadata` from config (issuer, endpoints, scopes from permission catalog); no own JSON shapes |
| `app/auth/oauth/token_cache.go` | in-process otter cache of token hash to principal, 30 s TTL; invalidation via Valkey pubsub (`a.PubSub` `redis` driver, topic `oauth.bust`) |
| `app/auth/oauth/usage_flusher.go` | buffered `last_used_at` writes, flush every 60 s and on shutdown |
| `app/actions/oauth/register_dynamic_client.go` | DCR |
| `app/actions/oauth/resolve_metadata_document_client.go` | CIMD fetch (`oauth.ClientMetadataDocument.Fetch` on `a.HTTP`), validate, cache (`a.Cache`, Valkey) |
| `app/actions/oauth/start_authorization_request.go` | validate `/oauth/authorize` params, create row |
| `app/actions/oauth/approve_authorization_request.go` | create grant + auth code, build redirect (first-party: no workspace, `{first_party}` scope, device fields incl. `device_id`); copies `auth_time`, `auth_methods` from request snapshot |
| `app/actions/oauth/deny_authorization_request.go` | approve/deny lock row `FOR UPDATE`, guard `pending` only (422 `invalid_transition`); controller picks action by `status` |
| `app/actions/oauth/exchange_authorization_code.go` | code + PKCE to tokens, code reuse revokes grant |
| `app/actions/oauth/refresh_access_token.go` | rotation, reuse detection |
| `app/actions/oauth/start_device_authorization.go` | RFC 8628 device code |
| `app/actions/oauth/lookup_device_request.go` | user code lookup with throttle, binds `user_id` |
| `app/actions/oauth/approve_device_request.go` | |
| `app/actions/oauth/deny_device_request.go` | |
| `app/actions/oauth/poll_device_token.go` | pending, slow_down (Valkey `SET NX PX interval` per `odr_`), expired, issue |
| `app/actions/oauth/revoke_token.go` | RFC 7009 |
| `app/actions/oauth/revoke_grant.go` | |
| `app/actions/oauth/revoke_member_grants.go` | on member removal, deactivation, role change |
| `app/actions/oauth/revoke_app_grant.go` | `RevokeAppGrant(userID, grantID, reason)`: one first-party grant (00-accounts `DELETE /sessions/{ogr}`, `DELETE /sessions/current` from app) |
| `app/actions/oauth/revoke_user_app_grants.go` | `RevokeUserAppGrants(userID, reason, exceptGrantID)`: all first-party grants of user (logout others, password change/reset, 2FA disable, account delete); runs in caller tx |
| `app/actions/oauth/list_user_app_grants.go` | `ListUserAppGrants(userID, currentGrantID)`: rows for 00-accounts `GET /sessions` (`kind: app`): `ogr_`, `client_id`, `client_name`, `device_name`, `device_platform`, `app_version`, `auth_methods`, `auth_time`, `confirmed_at`, `last_used_ip`, `last_used_at`, `created_at`, `current` |
| `app/actions/oauth/confirm_grant.go` | `ConfirmGrant(grantID, method)`: called by 00-accounts `ConfirmUser` when caller is first-party app; sets grant `confirmed_at = now()`, `confirmed_method`; busts token cache |
| `app/auth/oauth/first_party.go` | first-party rules: redirect kind (claimed vs scheme), consent skip, scope/resource rules, User-Agent parse (`Dimmi/<ver> (<platform> <os>; <model>)`, own format, kept custom; browser UAs via `framework/http/useragent`) |
| `app/actions/oauth/disable_client.go` | admin kill switch |
| `app/actions/agentaccess/update_agent_access_settings.go` | client kind allowlist |
| `app/http/controllers/oauth/authorization_server_metadata_controller.go` | `GET /.well-known/oauth-authorization-server` |
| `app/http/controllers/oauth/protected_resource_metadata_controller.go` | `GET /.well-known/oauth-protected-resource[/mcp\|/api/v1]` |
| `app/http/controllers/oauth/discovery_controller.go` | `GET /.well-known/dimmi.json` |
| `app/http/controllers/oauth/app_association_controller.go` | `GET /.well-known/apple-app-site-association`, `GET /.well-known/assetlinks.json` (from `config/apps.go`) |
| `app/http/controllers/web/app_callback_page_controller.go` + `resources/views/oauth/app_callback.templ` | `GET /oauth/callback/app` fallback page ("Open the Dimmi app") |
| `config/apps.go` | `apps.*` keys (first-party apps) |
| `app/http/controllers/oauth/authorize_controller.go` | `GET /oauth/authorize` |
| `app/http/controllers/oauth/token_controller.go` | `POST /oauth/token` (all grants); route carries foundation `client.version` middleware in OAuth error mode (426, below) |
| `app/http/controllers/oauth/device_authorization_controller.go` | `POST /oauth/device/code` |
| `app/http/controllers/oauth/client_registration_controller.go` | `POST /oauth/register` |
| `app/http/controllers/oauth/revocation_controller.go` | `POST /oauth/revoke` |
| `app/http/controllers/web/device_page_controller.go` | `GET /device` serves SPA shell (frontend later) |
| `app/http/controllers/api/v1/oauth_authorization_request_controller.go` | Show, Update (`status` approved/denied) |
| `app/http/controllers/api/v1/oauth_device_request_controller.go` | Index (`filter[user_code]` lookup), Update (`status` approved/denied) |
| `app/http/controllers/api/v1/oauth_grant_controller.go` | Index, Destroy |
| `app/http/controllers/api/v1/agent_access_settings_controller.go` | Show, Update |
| `app/http/requests/oauth/*.go` | `AuthorizeRequest`, `TokenRequest` (form-encoded, per grant), `DeviceAuthorizationRequest`, `ClientRegistrationRequest`, `RevocationRequest` |
| `app/http/requests/update_authorization_request_request.go`, `update_device_request_request.go`, `index_device_requests_request.go`, `update_agent_access_settings_request.go` | `status` in `approved\|denied`; `workspace_id`, `scopes` required when approved (prohibited for first-party); `filter[user_code]` required |
| `app/http/resources/oauth_authorization_request_resource.go` | consent payload |
| `app/http/resources/oauth_device_request_resource.go` | |
| `app/http/resources/oauth_client_resource.go` | public client display |
| `app/http/resources/oauth_grant_resource.go` | |
| `app/http/resources/agent_access_settings_resource.go` | |
| `app/http/middleware/authenticate_token.go` | (foundation file) extend: accept `dmi_oat_`, audience check, scope to abilities, `WWW-Authenticate` on 401/403 |
| `app/auth/oauth/resolver.go` | `ResolveAccessToken` (also the go-sdk `auth.TokenVerifier` for `/mcp`): one lookup, returns `auth:app` principal (first-party grant, null workspace) or `auth:token` principal; plugged into 00-accounts guards `auth:app` and `auth:token` |
| `app/http/middleware/log_redaction.go` | (foundation file) extend: redact `filter[user_code]` query value and `/oauth/callback/app` query |
| `app/http/middleware/oauth_cors.go` | CORS `*` without credentials on `.well-known/*`, `/oauth/token`, `/oauth/register`, `/oauth/revoke`, `/oauth/device/code` |
| `app/exceptions/oauth_errors.go` | RFC 6749 error type rendered `{"error":"...","error_description":"..."}` (not Laravel shape, OAuth endpoints only) |
| `app/policies/oauth_grant_policy.go` | view/revoke own; any in workspace needs `authz.Can(p, "oauth_grants:manage_any")` (no role names, P2) |
| `app/policies/agent_access_settings_policy.go` | update needs `authz.Can(p, "agent_access:manage")` |
| `app/events/oauth_grant_created.go`, `app/events/oauth_grant_revoked.go` | |
| `app/listeners/revoke_member_oauth_grants.go` | queued |
| `app/listeners/send_oauth_grant_created_mail.go` | queued (mail) |
| `app/listeners/invalidate_oauth_token_cache.go` | sync after commit |
| `app/mail/oauth_grant_created_mail.go` + `resources/views/mail/oauth_grant_created.templ` | "New app connected" security mail |
| `app/jobs/prune_oauth_records.go` | |
| `app/console/commands/oauth_client_command.go` | `dimmi oauth:client list\|disable\|enable` (self-host admin; refuses `first_party` clients, points to `apps.enabled`) |
| `app/console/commands/oauth_prune_command.go` | `dimmi oauth:prune` manual |
| `database/queries/oauth_clients.sql`, `oauth_grants.sql`, `oauth_authorization_requests.sql`, `oauth_auth_codes.sql`, `oauth_access_tokens.sql`, `oauth_refresh_tokens.sql`, `oauth_device_codes.sql` | sqlc |
| `database/factories/oauth_client_factory.go`, `oauth_grant_factory.go`, `oauth_access_token_factory.go` | `factories.OAuthGrant(t).For(member).WithScopes(...).Create(ctx)` returns grant + plaintext token |
| `routes/web.go` | `oauth` group (token/register/revoke/device: no session, cross-origin CSRF middleware off (form posts from any client, CORS `*`); `/oauth/authorize` uses session), `/.well-known/*`, `/oauth/callback/app` |
| `routes/api.go` | consent + grants + settings routes |

### MCP server

| Path | What |
|---|---|
| `config/mcp.go` | typed config |
| `app/mcp/server.go` | build server through `framework/mcp` (wraps modelcontextprotocol/go-sdk v1.8.0: 2026-07-28 stateless + legacy back to 2025-03-26, `server/discover`, MRTR), register tools/resources/prompts, server info, instructions |
| `app/mcp/http_handler.go` | thin: `s.HTTPHandler(Options{MaxBody, Auth})` from `framework/mcp` (`StreamableHTTPOptions.Stateless = true`) at `/mcp`: stateless mode, POST only, Origin check, header validation, legacy version shim, GET/DELETE 405 |
| `app/mcp/stdio.go` | stdio transport for in-process mode |
| `app/mcp/bridge.go` | stdio to remote HTTP bridge (client flavor), token refresh |
| `app/mcp/auth.go` | principal from bearer (OAuth or PAT) via go-sdk `auth.RequireBearerToken` with our `TokenVerifier`, or env token (stdio); `WWW-Authenticate` from go-sdk (no own builders) |
| `app/mcp/principal.go` | `Principal{Member, Workspace, Abilities, Grant, Client, TokenKind}`; `Actor()` returns `contracts.Actor{Type: member, ID, Via{Kind: oauth_client\|personal_token, ID}}` for actions |
| `app/mcp/registry.go` | tool registry: name, description, input type, output type, annotations, required abilities, operationId link |
| `app/mcp/schema.go` | JSON Schema from Go types via huma registry (same as OpenAPI) |
| `app/mcp/confirm/confirmation.go` | confirmation gate: elicitation (MRTR), `confirm` param (no policy bypass) |
| `app/mcp/confirm/request_state.go` | XChaCha20-Poly1305 sealed `requestState` (key HKDF from app key, info `mcp-request-state-v1`) |
| `app/mcp/render/*.go` | concise text renderers per resource, token estimator, truncation, untrusted-content envelope |
| `app/mcp/errors.go` | domain error to tool error mapping |
| `app/mcp/tools/get_workspace.go` | `dimmi_get_workspace` |
| `app/mcp/tools/list_conversations.go` | `dimmi_list_conversations` |
| `app/mcp/tools/search_conversations.go` | `dimmi_search_conversations` |
| `app/mcp/tools/get_conversation.go` | `dimmi_get_conversation` |
| `app/mcp/tools/send_reply.go` | `dimmi_send_reply` |
| `app/mcp/tools/add_note.go` | `dimmi_add_note` |
| `app/mcp/tools/tag_conversation.go` | `dimmi_tag_conversation` |
| `app/mcp/tools/assign_conversation.go` | `dimmi_assign_conversation` |
| `app/mcp/tools/close_conversation.go` | `dimmi_close_conversation` |
| `app/mcp/tools/reopen_conversation.go` | `dimmi_reopen_conversation` |
| `app/mcp/tools/snooze_conversation.go` | `dimmi_snooze_conversation` |
| `app/mcp/tools/search_contacts.go` | `dimmi_search_contacts` |
| `app/mcp/tools/get_contact.go` | `dimmi_get_contact` |
| `app/mcp/tools/update_contact.go` | `dimmi_update_contact` |
| `app/mcp/tools/search_knowledge.go` | `dimmi_search_knowledge` |
| `app/mcp/tools/list_articles.go` | `dimmi_list_articles` |
| `app/mcp/tools/get_article.go` | `dimmi_get_article` |
| `app/mcp/tools/propose_article.go` | `dimmi_propose_article` (08 owns behavior: suggestion kind create) |
| `app/mcp/tools/propose_article_edit.go` | `dimmi_propose_article_edit` (08: suggestion kind edit) |
| `app/mcp/resources/workspace.go`, `conversation.go`, `article.go`, `contact.go` | resource + templates |
| `app/mcp/prompts/triage_inbox.go`, `summarize_conversation.go`, `article_from_conversation.go`, `daily_digest.go` | prompts |
| `routes/mcp.go` | registration list (readable, ordered, deterministic) |
| `app/console/commands/mcp_command.go` | `dimmi mcp` (bridge default; `--in-process` in full flavor) |
| `tests/feature/mcp/*_test.go` | per tool: happy, ability, other workspace, budget, confirmation |
| `tests/feature/mcp/conformance_test.go` | runs conformance client against test server |

### CLI (client flavor + full flavor)

| Path | What |
|---|---|
| `cmd/dimmi/main.go` | root; registers client commands always, server commands only `//go:build !cli` |
| `cmd/dimmi/server_commands.go` | `//go:build !cli` |
| `app/console/commands/client/root.go` | global flags `--host --profile --json --dry-run --yes --no-input --quiet --debug` |
| `app/console/commands/client/login_command.go`, `logout_command.go`, `whoami_command.go`, `profiles_command.go` | |
| `app/console/commands/client/init_command.go` | |
| `app/console/commands/client/conversations_command.go` | list, show, reply, close |
| `app/console/commands/client/changelog_command.go` | 13: list, new, draft, publish, subscribers import/export |
| `app/console/commands/client/feedback_command.go` | 14: import helper, status shortcut; rest generated |
| `app/console/commands/client/engage_command.go` | 15-announcements-surveys: `announcements`, `surveys`, `segments` |
| `app/console/commands/client/articles_command.go` | pull, push |
| `app/console/commands/client/tokens_command.go` | create, list, revoke |
| `app/console/commands/client/agents_command.go` | `agents install` |
| `app/console/commands/client/api_command.go` | raw `dimmi api METHOD PATH` |
| `app/console/commands/client/version_command.go` | |
| `app/client/apiclient/` | generated from `docs/openapi/v1.json` (oapi-codegen, client only) + hand wrapper (retries, Idempotency-Key, errors) |
| `app/client/auth/device_flow.go` | RFC 8628 client on golang.org/x/oauth2 (`DeviceAuth`, `DeviceAccessToken`, PKCE helpers) |
| `app/client/auth/credentials.go` | keychain (zalando/go-keyring) + 0600 file fallback, file lock on refresh |
| `app/client/config/profiles.go` | `~/.config/dimmi/config.json` profiles; `dimmi.json` project file |
| `app/client/discovery/discovery.go` | `/.well-known/dimmi.json` fetch + version gate |
| `app/client/output/` | table, JSON, exit codes, error rendering, OSC 8 links, color (NO_COLOR) |
| `app/client/initwizard/detect/*.go` | one detector per SDK framework id (`nextjs-app`, `nextjs-pages`, `react`, `react-router`, `sveltekit`, `nuxt`, `vue`, `astro`, `rails`, `laravel`, `django`, `fastapi`, `go`, `express`, `hono`, `html`, `proxy`; same list as 01 `BuildInstallSnippets`) |
| `app/client/initwizard/patch/*.go` | anchored patchers, markers, diff preview, rollback |
| `app/client/initwizard/steps/*.go` | login, workspace, widget, SDK install + `dimmi.config.ts`/`withDimmi`, identity source hook (package stacks) or raw snippet route (other stacks), env, help center seed, agent files, 3-line summary |
| `app/client/docsync/` | articles pull/push, frontmatter, lock file |
| `app/client/watch/` | `list --watch` realtime via centrifuge-go client |
| `app/client/agentfiles/` | writes skill, Cursor rule, AGENTS.md block, MCP configs |
| `resources/cli/init/<framework>/*.tmpl` | SDK config + identity source hook templates for package stacks (no JWT endpoint templates); route file placement for raw-snippet stacks (body = 02 raw snippet from snippets API); mirror `02-widget-identity/snippets.md` |
| `resources/cli/agents/SKILL.md.tmpl`, `dimmi.mdc.tmpl`, `AGENTS.snippet.md.tmpl`, `prompt.md.tmpl` | install-by-prompt sources (embedded, also published) |
| `npm/dimmi/` | npm wrapper: `package.json`, `bin/dimmi.js`, `lib/resolve.js`, `checksums.json` |
| `npm/platforms/*/package.json` | `@dimmi/cli-<os>-<arch>` |
| `.goreleaser.yaml` | builds `dimmi` (full) and `dimmi-cli` flavor archives, npm publish step, Homebrew tap, cosign |
| `docs/llms.txt`, `docs/llms-full.txt` (generated) | Dimmi product docs index (docs site, see cli.md) |
| `tests/cli/*_test.go` | golden output tests, init fixtures per framework (`tests/cli/fixtures/<framework>/`) |
| `tests/e2e/init_test.go` | full init against test server with scripted device approval |

## HTTP API

Auth legend (api.md letters in brackets): `public` none (G); `session` = `auth:session` browser session + `http.CrossOriginProtection` (S); `user` = `auth:user`, session or first-party app (U); ability names alone = `auth` any guard, abilities checked for tokens only (A); `client` OAuth client auth at token endpoint (none/basic/post).

### OAuth and discovery endpoints (web routes, OAuth error shape)

| Method | Path | Route name | Auth | Purpose |
|---|---|---|---|---|
| GET | `/.well-known/oauth-authorization-server` | oauth.metadata | public | RFC 8414 AS metadata |
| GET | `/.well-known/oauth-protected-resource` | oauth.resource-metadata | public | RFC 9728, resource = APP_URL |
| GET | `/.well-known/oauth-protected-resource/mcp` | oauth.resource-metadata.mcp | public | resource = APP_URL/mcp |
| GET | `/.well-known/oauth-protected-resource/api/v1` | oauth.resource-metadata.api | public | resource = APP_URL/api/v1 |
| GET | `/.well-known/dimmi.json` | dimmi.discovery | public | CLI and app discovery: issuer, api_base, mcp_url, region, edition, version, cli versions, `apps` block |
| GET | `/.well-known/apple-app-site-association` | apps.association.apple | public | iOS associated domains (applinks + webcredentials); 404 when `apps.ios_app_ids` empty or apps disabled |
| GET | `/.well-known/assetlinks.json` | apps.association.android | public | Android App Links; 404 when `apps.android_package_fingerprints` empty or apps disabled |
| GET | `/oauth/authorize` | oauth.authorize | session (redirects to login) | validate, create authorization request, redirect to `/oauth/consent/{oar_id}` (SPA) or auto-approve; `max_age`, `prompt=login\|none\|consent`, `device_name`, `device_id` |
| POST | `/oauth/token` | oauth.token | client | grants: `authorization_code`, `refresh_token`, `urn:ietf:params:oauth:grant-type:device_code` |
| POST | `/oauth/device/code` | oauth.device.code | client (public) | RFC 8628 device authorization |
| POST | `/oauth/register` | oauth.register | public | RFC 7591 DCR |
| POST | `/oauth/revoke` | oauth.revoke | client | RFC 7009 |
| GET | `/device` | device.show | session (redirects to login) | SPA page for user code entry; `?user_code=` prefill |
| GET | `/oauth/consent/{oar}` | oauth.consent.show | session | SPA page (frontend later) |
| GET | `/oauth/callback/app` | oauth.app-callback.show | public | fallback page when claimed link not opened by OS: "Open the Dimmi app"; query never logged or rendered |

### REST API (`/api/v1`, Laravel shapes)

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | `/api/v1/oauth/authorization-requests/{oar}` | oauth.authorization-requests.show | session | consent screen payload |
| PATCH | `/api/v1/oauth/authorization-requests/{oar}` | oauth.authorization-requests.update | session | `{status: "approved", workspace_id, scopes}` or `{status: "denied"}`; returns resource + `redirect_to` (deny: `error=access_denied`); first-party: `{status}` only |
| GET | `/api/v1/oauth/device-requests?filter[user_code]=XXXX-XXXX` | oauth.device-requests.index | session | user code lookup; 0 or 1 pending rows; `user_code` redacted in logs; throttled |
| PATCH | `/api/v1/oauth/device-requests/{odr}` | oauth.device-requests.update | session | `{status: "approved", workspace_id, scopes}` or `{status: "denied"}` |
| GET | `/api/v1/oauth/grants` | oauth.grants.index | session, or token + `members:read` for `filter[user]=all` | connected apps (workspace grants only; first-party grants listed by 00-accounts `GET /sessions`); filters `filter[user]=me\|all\|usr_..`, `filter[client]` |
| DELETE | `/api/v1/oauth/grants/{ogr}` | oauth.grants.destroy | session (own, or `oauth_grants:manage_any` for any); token cannot revoke other grants, only own grant (`ogr` = current) | revoke grant + all tokens |
| GET | `/api/v1/settings/agent-access` | settings.agent-access.show | session or token `settings:read` | client kind policy |
| PATCH | `/api/v1/settings/agent-access` | settings.agent-access.update | `agent_access:manage` (ability group `settings:write`) | update |

Send: no 09 route. `POST /api/v1/conversations/{conversation}/replies {body, then?}` (03, `conversations.replies.store`, `conversations:write`, `metered`); 09 adds nothing to 03 reply tx. REST accepts no `reopen`/`close_after` (unknown field 422): those are MCP tool args and CLI flags; tool/CLI translate `reopen: true` to 03 `then: "open"`, `close_after: true` to `then: "close"`, neither to `then: "keep"` (both true: tool validation error, `then` takes one value).

Used but owned elsewhere: `GET /api/v1/me` (00-accounts), `GET/DELETE /api/v1/sessions` (00-accounts; first-party rows from `ListUserAppGrants`, revoke via `RevokeAppGrant`/`RevokeUserAppGrants`), PAT CRUD `/api/v1/tokens` (00-accounts), conversations/messages/contacts (03), articles/suggestions/cold-start/knowledge search (08), widget settings (01), identity secrets (02), webhook endpoints (10).

### Key payloads

`GET /.well-known/oauth-authorization-server` (cloud example):

```json
{
  "issuer": "https://app.trydimmi.com",
  "authorization_endpoint": "https://app.trydimmi.com/oauth/authorize",
  "token_endpoint": "https://app.trydimmi.com/oauth/token",
  "device_authorization_endpoint": "https://app.trydimmi.com/oauth/device/code",
  "registration_endpoint": "https://app.trydimmi.com/oauth/register",
  "revocation_endpoint": "https://app.trydimmi.com/oauth/revoke",
  "scopes_supported": ["conversations:read","conversations:write","contacts:read","contacts:write","articles:read","articles:write","settings:read","settings:write","webhooks:read","webhooks:write","members:read","members:write","notifications:read","notifications:write","ai:read","ai:write","ai:keys","billing:read","billing:write","exports:read","exports:write","offline_access"],
  "response_types_supported": ["code"],
  "response_modes_supported": ["query"],
  "grant_types_supported": ["authorization_code","refresh_token","urn:ietf:params:oauth:grant-type:device_code"],
  "token_endpoint_auth_methods_supported": ["none","client_secret_basic","client_secret_post"],
  "revocation_endpoint_auth_methods_supported": ["none","client_secret_basic","client_secret_post"],
  "code_challenge_methods_supported": ["S256"],
  "authorization_response_iss_parameter_supported": true,
  "client_id_metadata_document_supported": true,
  "service_documentation": "https://trydimmi.com/docs/api/oauth"
}
```

`registration_endpoint` omitted when DCR disabled in config. `client_id_metadata_document_supported` false when CIMD disabled.

`GET /.well-known/oauth-protected-resource/mcp`:

```json
{
  "resource": "https://app.trydimmi.com/mcp",
  "authorization_servers": ["https://app.trydimmi.com"],
  "scopes_supported": ["conversations:read","conversations:write","contacts:read","contacts:write","articles:read","articles:write","members:read"],
  "bearer_methods_supported": ["header"],
  "resource_name": "Dimmi",
  "resource_documentation": "https://trydimmi.com/docs/mcp"
}
```

`GET /.well-known/dimmi.json`:

```json
{
  "object": "dimmi_discovery",
  "name": "Dimmi",
  "issuer": "https://app.trydimmi.com",
  "api_base": "https://app.trydimmi.com/api/v1",
  "mcp_url": "https://app.trydimmi.com/mcp",
  "device_authorization_endpoint": "https://app.trydimmi.com/oauth/device/code",
  "token_endpoint": "https://app.trydimmi.com/oauth/token",
  "edition": "cloud",
  "region": "us",
  "regions": [{ "id": "us", "host": "https://app.trydimmi.com" }],
  "server_version": "1.0.0",
  "api_version": "2026-11-01",
  "cli": { "min_version": "1.0.0", "latest_version": "1.0.0" },
  "apps": { "enabled": true, "client_id": "dimmi-mobile", "claimed_redirect_uri": "https://app.trydimmi.com/oauth/callback/app", "fallback_redirect_uri": "com.trydimmi.app:/oauth/callback" }
}
```

Self-host: `edition: "self_hosted"`, `region: null`, `regions: []`. `apps.claimed_redirect_uri` null when both association lists empty; `apps.enabled: false` hides the rest. App picks claimed URI only when non-null and host is in its compiled associated domains, else fallback (consent shown).

`GET /api/v1/oauth/authorization-requests/oar_01jc...` (consent screen):

```json
{
  "data": {
    "id": "oar_01jc...",
    "object": "oauth_authorization_request",
    "status": "pending",
    "expires_at": "2026-11-01T10:10:00Z",
    "client": {
      "id": "ocl_01jc...",
      "object": "oauth_client",
      "kind": "metadata_document",
      "name": "Claude Code",
      "client_id_host": "claude.ai",
      "client_uri": "https://claude.ai",
      "logo_url": "https://app.trydimmi.com/img-proxy/...",
      "verified": false,
      "official": false,
      "first_party": false
    },
    "redirect": { "host": "localhost", "loopback": true, "warning": "loopback_redirect" },
    "requested_scopes": [
      { "scope": "conversations:read", "description": "Read conversations, messages, and notes", "grantable": true },
      { "scope": "conversations:write", "description": "Reply, note, tag, assign, close, snooze", "grantable": true },
      { "scope": "settings:write", "description": "Change workspace settings", "grantable": false, "reason": "role_member" }
    ],
    "resources": ["https://app.trydimmi.com/mcp"],
    "workspaces": [
      { "id": "ws_01ja...", "name": "Acme", "role": "owner", "last_used": true, "client_allowed": true }
    ],
    "existing_grant": null,
    "request": { "ip": "203.0.113.7", "user_agent": "claude-code/2.4.1", "created_at": "2026-11-01T10:00:00Z" }
  }
}
```

`PATCH /api/v1/oauth/authorization-requests/oar_01jc...`:

```json
{ "status": "approved", "workspace_id": "ws_01ja...", "scopes": ["conversations:read","conversations:write","contacts:read","articles:read"] }
```

Response 200:

```json
{ "data": { "id": "oar_01jc...", "object": "oauth_authorization_request", "status": "approved", "redirect_to": "http://localhost:53682/callback?code=dmi_oac_...&state=xyz&iss=https%3A%2F%2Fapp.trydimmi.com" } }
```

First-party (scheme fallback) consent payload differs: `client.first_party: true`, `workspaces: []`, `requested_scopes: []`, `grant_reach: "account"`, `warning: "first_party_scheme"`, `device_name`, `device_platform`. Approve body `{ "status": "approved" }`.

`GET /api/v1/oauth/device-requests?filter[user_code]=BDWP-HQKX`:

```json
{ "data": [ { "id": "odr_01jc...", "object": "oauth_device_request", "status": "pending", "user_code": "BDWP-HQKX", "device_name": "frantz-mbp", "client": { "id": "ocl_...", "object": "oauth_client", "name": "Dimmi CLI", "official": true }, "requested_scopes": [{ "scope": "conversations:read", "grantable": true }], "workspaces": [{ "id": "ws_01ja...", "name": "Acme", "last_used": true, "client_allowed": true }], "request": { "ip": "203.0.113.7", "location": "Berlin, DE", "created_at": "2026-11-01T10:00:00Z" }, "expires_at": "2026-11-01T10:15:00Z" } ], "meta": { "per_page": 1 } }
```

`POST /oauth/device/code` (form): `client_id=dimmi-cli&scope=conversations:read conversations:write ...&resource=https://app.trydimmi.com/api/v1&resource=https://app.trydimmi.com/mcp&device_name=frantz-mbp`

```json
{
  "device_code": "dmi_odc_6Fq...",
  "user_code": "BDWP-HQKX",
  "verification_uri": "https://app.trydimmi.com/device",
  "verification_uri_complete": "https://app.trydimmi.com/device?user_code=BDWP-HQKX",
  "expires_in": 900,
  "interval": 5
}
```

`POST /oauth/token` success (any grant):

```json
{
  "access_token": "dmi_oat_Xk2...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "refresh_token": "dmi_ort_9Pa...",
  "scope": "conversations:read conversations:write contacts:read articles:read",
  "workspace": { "id": "ws_01ja...", "name": "Acme" },
  "grant_id": "ogr_01jc..."
}
```

First-party (`dimmi-mobile`) token response: `expires_in: 900`, `scope: "first_party"`, `grant_id`, `auth_time` (unix seconds), no `workspace` (app sends `Dimmi-Workspace` per request).

Client too old (`Dimmi-Client-Version: <platform>/<semver>` below `apps.min_versions.<platform>`; any grant type incl. refresh): token endpoint keeps OAuth error shape (RFC 6749), status 426:

```json
{ "error": "invalid_request", "error_description": "client_upgrade_required", "min_version": "1.4.0", "upgrade_url": "https://apps.apple.com/app/dimmi/id..." }
```
Header absent (third-party MCP clients, browsers) = allowed. Config empty = noop. Refresh refused leaves grant and refresh token untouched (app retries after upgrade). Same gate on `/api/v1` uses Laravel shape 426 `client_upgrade_required` (foundation `api.md`).

`PATCH /api/v1/settings/agent-access`:

```json
{ "allow_dynamic_clients": false }
```

### Error codes (new)

| HTTP | code | When |
|---|---|---|
| 404 | `authorization_request_not_found` | unknown, expired over 24 h, or bound to other user |
| 410 | `authorization_request_expired` | past `expires_at` (authorization and device requests) |
| 422 | `invalid_transition` | `PATCH` status on request not `pending` (double click, approve after deny); `from`, `to` (rest.md rule 3) |
| 422 | `workspace_not_allowed` | user not active member, or workspace policy blocks client kind |
| 422 | `scope_not_grantable` | scope above role |
| 404 | `device_request_not_found` | `PATCH` on unknown `odr`, or bound to other user (lookup with bad code returns empty list, not 404) |
| 429 | `device_lookup_throttled` | over 10 lookups per minute per user |
| 422 | `invalid_transition` | MCP `dimmi_send_reply` / CLI `conversations reply` on closed conversation without `reopen: true` / `--reopen` (or `close_after` / `--close`): 09 tool-layer guard, body `from: "closed"`, `to: "open"` + hint; 03 REST itself sends on closed with `then: keep` (never `conversation_closed`, code retired) |
| 426 | `client_upgrade_required` | `/oauth/token` with `Dimmi-Client-Version` below `apps.min_versions.<platform>` (OAuth shape `{error: invalid_request, error_description: client_upgrade_required, min_version, upgrade_url}`) |
| 403 | `first_party_token_not_allowed` | first-party (`auth:app`) token at `/mcp` |
| 403 | `first_party_app_disabled` | first-party token while `apps.enabled=false` |
| 422 | `secret_detected` | MCP/CLI send body matches secret pattern (`sk-`, `dmi_`, `AKIA`, private key header, `xox[bp]-`); nothing sent |
| 403 | `missing_ability` | foundation |
| 423 | `spend_cap_reached` (and other 12 block codes) | 12 `InTx` gate inside 03 `ReplyToConversation` (authoritative); `metered` middleware only fast pre-check |

OAuth endpoint errors use RFC 6749 shape `{"error":"invalid_grant","error_description":"..."}` with status 400 (401 for `invalid_client` with `WWW-Authenticate: Basic` when Basic used). Full list in `oauth.md`.

## MCP tools (summary; full spec in mcp.md)

Transport: `POST /mcp` streamable HTTP, and `dimmi mcp` stdio. 19 core tools, deterministic order:

| Tool | Ability | Annotations | Confirmation | REST equivalent (`/api/v1`) |
|---|---|---|---|---|
| `dimmi_get_workspace` | any | readOnly | | `GET /me` + members, teams, inboxes, tags, collections reads (composite) |
| `dimmi_list_conversations` | conversations:read | readOnly | | `GET /conversations?filter[...]` |
| `dimmi_search_conversations` | conversations:read | readOnly | | `GET /conversations?q=` |
| `dimmi_get_conversation` | conversations:read | readOnly | | `GET /conversations/{conv}` + `GET /conversations/{conv}/messages` |
| `dimmi_send_reply` | conversations:write | destructive true, openWorld true | yes, always | `POST /conversations/{conv}/replies {body, then}` (`reopen`/`close_after` args map to `then`) |
| `dimmi_add_note` | conversations:write | destructive false | no | `POST /conversations/{conv}/notes` |
| `dimmi_tag_conversation` | conversations:write | idempotent | no | `POST /conversations/{conv}/tags`, `DELETE /conversations/{conv}/tags/{tag}` |
| `dimmi_assign_conversation` | conversations:write | idempotent | no | `PATCH /conversations/{conv} {assignee_id, team_id}` |
| `dimmi_close_conversation` | conversations:write | idempotent | no | `PATCH /conversations/{conv} {status: "closed"}` |
| `dimmi_reopen_conversation` | conversations:write | idempotent | no | `PATCH /conversations/{conv} {status: "open"}` |
| `dimmi_snooze_conversation` | conversations:write | idempotent | no | `PATCH /conversations/{conv} {status: "snoozed", snoozed_until}` |
| `dimmi_search_contacts` | contacts:read | readOnly | | `GET /contacts?q=` |
| `dimmi_get_contact` | contacts:read | readOnly | | `GET /contacts/{ctc}` |
| `dimmi_update_contact` | contacts:write | idempotent | no | `PATCH /contacts/{ctc}` |
| `dimmi_search_knowledge` | articles:read | readOnly | | `GET /knowledge/search` (08) |
| `dimmi_list_articles` | articles:read | readOnly | | `GET /articles` |
| `dimmi_get_article` | articles:read | readOnly | | `GET /articles/{art}` |
| `dimmi_propose_article` | articles:write | destructive false | no (review queue, never live) | `POST /article-suggestions {kind: "create"}` |
| `dimmi_propose_article_edit` | articles:write | destructive false | no (review queue, never live) | `POST /article-suggestions {kind: "edit"}` |

REST column per rest.md rule 15: tool names stay verbs, REST is the contract; tool declares matching `operationId`s (`conversations.update`, `conversations.replies.store`, ...), drift test in CI.

No MCP tool publishes, accepts, verifies, or deletes articles (08 world-class bar; 08 endpoints reject MCP with 403 `human_review_required`). Publishing stays in dashboard, REST, CLI (`articles push --publish`, `articles publish`). Only carve-out: 16 `dimmi_run_import` with `articles.publish: preserve` after confirm gate (migrated human-written content). Other 08 tools (`dimmi_get_help_center_outline`, `dimmi_propose_help_center`, `dimmi_list_article_suggestions`, `dimmi_list_knowledge_sources`, `dimmi_sync_knowledge_source`) live in `help_center` toolset.

Feature toolsets: tools defined by other specs (00-accounts `accounts`, 01/02 `widget`, 03 `inbox_extra`, 04, 05, 06, 07 `ai`, 07 `ai_actions`, 08 `help_center`, 08 `docs_sync`, 10, 11, 12, 13 `changelog`, 14 `feedback`, 15 `engage`, 16 `imports`; names and confirm-gate rules in mcp.md) register through same registry in named toolsets, off by default; `/mcp?toolsets=billing,channels` or `dimmi mcp --toolsets`. Rules in mcp.md "Feature toolsets". Core 19 above always on.

Resources: `dimmi://workspace`, `dimmi://conversations/{number}`, `dimmi://articles/{id_or_slug}`, `dimmi://contacts/{id}`.
Prompts: `triage_inbox`, `summarize_conversation`, `article_from_conversation`, `daily_digest`.

Send decision (summary, MVP safe default): every `dimmi_send_reply` needs human confirmation, any token kind, no workspace bypass: client elicitation capability (form elicitation via MRTR `InputRequiredResult`, sealed single-use `requestState`) else fallback `confirm: true` param required (first call without it returns preview + `status: "confirmation_required"`). Decline/cancel stores nothing. Send modes and server-side drafts: later: agent-send-modes, later: reply-drafts.

## CLI commands (summary; full spec in cli.md)

Distribution decision: same Go codebase, two build flavors (`dimmi` full = server + client; `dimmi` client flavor via `-tags cli`, under 15 MB). npm package `dimmi` = tiny JS launcher + platform binary via `optionalDependencies` (`@dimmi/cli-<os>-<arch>`), no postinstall. Pure TS CLI rejected: second implementation of API client, auth, output; drift from OpenAPI types; `init` code patching does not need JS AST (anchored patches + agent handoff). Full rationale in `cli.md`.

| Command | Purpose |
|---|---|
| `dimmi login [--host] [--token\|--with-token] [--scopes] [--profile]` | device flow, any cloud region or self-host URL |
| `dimmi logout [--all]` | revoke grant server-side, wipe local |
| `dimmi whoami` | user, workspace, host, abilities, token kind, expiry |
| `dimmi profiles list\|use\|remove` | multiple hosts/workspaces |
| `dimmi init` | detect framework (SDK framework ids), install SDK, write `dimmi.config.ts` / `withDimmi` + identity source hook (package stacks) or 02 raw snippet route (other stacks, later: identity-helper-packages), env, `dimmi.json` (`wgt_`), seed help center, agent files, 3-line summary |
| `dimmi conversations list\|show\|reply\|close` | inbox from terminal |
| `dimmi articles pull\|push\|publish` | docs-as-code minimal |
| `dimmi docs suggestions\|sync\|stale\|verify` | 08 review queue + docs sync (`docs push\|pull` alias `articles push\|pull`) |
| `dimmi github repos add\|list` | 08 shared GitHub layer |
| `dimmi actions list\|get\|test\|publish\|calls\|apply\|import` | 07 agent actions |
| `dimmi migrate <source>`, `dimmi imports ...` | 16 importers (wizard owned by 16) |
| `dimmi webhooks endpoints\|deliveries\|redeliver`, `dimmi trigger <event_type>` | 10 webhooks |
| `dimmi changelog list\|new\|draft\|publish\|subscribers import\|subscribers export` | 13 changelog |
| `dimmi feedback posts\|status\|link\|import\|suggestions` | 14 feedback (import = dry run then `--run`) |
| `dimmi announcements ...`, `dimmi surveys ...`, `dimmi segments ...` | 15 engage (`15-announcements-surveys`) |
| `dimmi tokens create\|list\|revoke` | scoped PATs, `--no-expiry` |
| `dimmi mcp [--in-process]` | stdio MCP |
| `dimmi agents install [--claude-code] [--cursor] [--agents-md] [--mcp]` | install-by-prompt files |
| `dimmi api METHOD PATH [-f k=v] [--input file]` | raw REST |
| `dimmi version` | client + server version, update hint |

Global: `--json`, `--dry-run` (every write), `--yes`, `--no-input` (auto when not TTY or `CI=true`), `--host`, `--profile`, `--quiet`, `--debug`. Exit codes: 0 ok, 1 error, 2 usage, 3 unauthenticated, 4 forbidden, 5 not found, 6 conflict, 7 validation, 8 rate limited, 9 network, 10 upgrade required, 11 partial success.

## Events

### Emitted (new)

| Event | Emitted by | Realtime | Webhook | Notes |
|---|---|---|---|---|
| oauth_grant.created | ApproveAuthorizationRequest, ApproveDeviceRequest | yes (`member:{mem_}` only; none for first-party: no member) | no | workspace audit log; first-party: 00-accounts user security event instead; security mail |
| oauth_grant.revoked | RevokeGrant, RevokeMemberGrants, RevokeAppGrant, RevokeUserAppGrants, refresh reuse | yes (`member:{mem_}`; none for first-party) | no | audit log / security event as above; payload `{grant_id, user_id, first_party, device_id, reason}`; reason registry incl. `pre_hijack_defense` |

Listed in `00-foundation/events-and-realtime.md` "Events added by feature specs".

### Emitted (existing, via called actions)

`message.created`, `conversation.tagged/untagged`, `conversation.assigned`, `conversation.closed`, `conversation.reopened`, `conversation.snoozed`, `contact.updated`, `article.*`, `article_suggestion.created`. Actor on all: member + `via` (see Contradictions).

### Consumed

| Event | Listener | Action |
|---|---|---|
| member.deactivated (00-accounts) | RevokeMemberOAuthGrants (queued) | revoke member's grants in workspace, reason `member_deactivated` |
| user.deleted (00-accounts) | RevokeMemberOAuthGrants (queued) | revoke all user's grants in every workspace, reason `user_deleted` (users soft-deleted; FK cascade only at hard purge) |
| member.role_changed (00-accounts) | RevokeMemberOAuthGrants | revoke grants only on downgrade where granted scopes exceed new role; else no-op (clamp happens per request anyway) |
| oauth_grant.created | SendOAuthGrantCreatedMail (queued `mail`) | mail to user unless client `official` and same device_name seen in last 30 days |
| oauth_grant.revoked | InvalidateOAuthTokenCache (sync, AfterCommit) | bust cache on all replicas via Valkey pubsub (`a.PubSub`, topic `oauth.bust`); also disconnects first-party app realtime connections of that grant (centrifuge, Redis broker on Valkey) |

`oauth_grant.revoked` also consumed by 06 (`DeletePushSubscriptionsForGrant`: push rows of revoked first-party grant). Payload carries `grant_id`, `user_id`, `first_party`, `device_id`, `reason` so consumers never re-query.

### Called synchronously by other specs

| Caller | 09 entry point | When |
|---|---|---|
| 00-accounts `GET /sessions` | `ListUserAppGrants(userID, currentGrantID)` | session list rows `kind: app` |
| 00-accounts `DELETE /sessions/{ogr}`, `DELETE /sessions/current` (app) | `RevokeAppGrant` | same tx |
| 00-accounts `DELETE /sessions`, password change, password reset, 2FA disable, account delete, pre-hijack defense | `RevokeUserAppGrants` | same tx as the account change (reasons in oauth.md "First-party apps"; pre-hijack = `pre_hijack_defense`) |
| 00-accounts `POST /user/confirmation` from first-party app | `ConfirmGrant(grantID, method)` | same tx; sets grant `confirmed_at`, `confirmed_method` |
| 00-accounts guards `auth:app`, `auth:token` | `oauth.ResolveAccessToken` | every `dmi_oat_` request |

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| PruneOAuthRecords | maintenance | periodic hourly (`routes/console.go`) | 3 | periodic unique by kind |
| RevokeMemberOAuthGrants | default | member events | 10, exp backoff | unique by `member_id` for 1 minute |
| SendOAuthGrantCreatedMail | mail | oauth_grant.created | 5 | unique by `grant_id` (first-party: "New sign-in to the Dimmi app on <device_name>"; skipped for re-auth when same device_name + platform had active grant in last 30 days) |

PruneOAuthRecords revokes grants with no refresh in 90 days (reason `idle`), then deletes: auth codes expired over 1 hour; authorization requests and device codes expired over 24 hours; access tokens expired over 7 days; refresh tokens used or expired over 30 days (keep chain parents of active tokens); dynamic and metadata-document clients with zero grants and `last_used_at` over 30 days. Batches of 1,000, stops after 60 s.

## Config keys

`config/oauth.go` (env `DIMMI_OAUTH_*`):

| Key | Default | Notes |
|---|---|---|
| `oauth.enabled` | true | false disables OAuth endpoints (PAT only) |
| `oauth.access_token_ttl` | `1h` | 5m..24h |
| `oauth.refresh_token_idle_ttl` | `2160h` (90 days) | sliding; 0 = never expires |
| `oauth.refresh_token_absolute_ttl` | `0` | 0 = none |
| `oauth.auth_code_ttl` | `60s` | |
| `oauth.authorization_request_ttl` | `10m` | |
| `oauth.device_code_ttl` | `15m` | |
| `oauth.device_poll_interval` | `5s` | |
| `oauth.dynamic_registration.enabled` | true | |
| `oauth.dynamic_registration.per_ip_per_hour` | 20 | |
| `oauth.metadata_documents.enabled` | true | |
| `oauth.metadata_documents.fetch_timeout` | `5s` | |
| `oauth.metadata_documents.max_bytes` | 16384 | |
| `oauth.metadata_documents.cache_min` | `5m` | floor on Cache-Control |
| `oauth.metadata_documents.cache_max` | `24h` | ceiling |
| `oauth.metadata_documents.allowed_hosts` | `[]` | empty = any public host |
| `oauth.metadata_documents.blocked_hosts` | `[]` | |
| `oauth.allow_insecure_http` | false | self-host LAN without TLS; `dimmi doctor` warns |
| `oauth.official_client_hosts` | `["claude.ai","anthropic.com","cursor.com","openai.com","github.com","code.visualstudio.com"]` | CIMD hosts shown with "known client" badge only (never auto-trust) |
| `oauth.first_party.access_token_ttl` | `15m` | 5m..1h; first-party (`auth:app`) access tokens |

`config/apps.go` (env `DIMMI_APPS_*`; first-party apps, security.md names):

| Key | Default | Notes |
|---|---|---|
| `apps.enabled` | true | false: `dimmi-mobile` authorize/token refused, API 403 `first_party_app_disabled`, association files 404 |
| `apps.ios_app_ids` | `["<DIMMI_TEAM_ID>.com.trydimmi.app"]` | `apple-app-site-association` `appIDs` + `webcredentials`; empty disables claimed redirect on iOS |
| `apps.android_package_fingerprints` | `["com.trydimmi.app=<release key SHA-256>"]` | `assetlinks.json`; `<package>=<AA:BB:...>` entries; empty disables claimed redirect on Android |
| `apps.min_versions.ios` / `.android` / `.cli` | empty (no gate) | semver; foundation `client.version` middleware on `/api/v1` and `/oauth/token` returns 426 when `Dimmi-Client-Version` lower. Exposed in `/auth/config` `min_client_versions` and discovery `cli.min_version`. Self-host default for `.cli`: bundled CLI major.0.0 |
| `apps.upgrade_urls.ios` / `.android` / `.cli` | store URLs; CLI docs URL | `upgrade_url` in 426 body |

Both lists empty: claimed redirect URI rejected, app uses private-use scheme (consent always).

`config/mcp.go` (env `DIMMI_MCP_*`):

| Key | Default | Notes |
|---|---|---|
| `mcp.enabled` | true | |
| `mcp.path` | `/mcp` | |
| `mcp.protocol_versions` | `["2026-07-28","2025-11-25","2025-06-18","2025-03-26"]` | first = preferred |
| `mcp.allowed_origins` | `[]` | requests with Origin not in list and not APP_URL origin get 403 |
| `mcp.response_token_budget` | 8000 | default per tool response |
| `mcp.response_token_cap` | 20000 | hard cap (`detail: "full"`) |
| `mcp.request_state_ttl` | `10m` | confirmation window |
| `mcp.send_rate_per_minute` | 30 | per grant/PAT, `dimmi_send_reply` |
| `mcp.write_rate_per_minute` | 120 | per grant/PAT, all write tools |
| `mcp.max_request_bytes` | 1048576 | |
| `mcp.default_page_size` | 20 | max 50 |

`config/cli.go` (server side): `cli.latest_version` (build-time). Minimum versions live in `config/apps.go` `apps.min_versions.*` (one gate for CLI and apps; discovery `cli.min_version` reads `apps.min_versions.cli`).

CLI env (client side): `DIMMI_HOST`, `DIMMI_TOKEN` (PAT or access token, skips keychain), `DIMMI_PROFILE`, `DIMMI_CONFIG_DIR` (default `$XDG_CONFIG_HOME/dimmi` or `~/.config/dimmi`; Windows `%AppData%\dimmi`), `DIMMI_NO_KEYRING=1`, `DIMMI_CA_FILE`, `NO_COLOR`, `CI`.

## Permissions matrix

Scopes = abilities (`00-foundation/api.md`) = permission groups: one vocabulary (extensibility.md sections 4, 11). Catalog canonical in 00-accounts `app/auth/permissions.go`, role mapping in `app/auth/roles.go`; consent clamp and per-request check call `authz` (never a copy). Grantable at consent = scope groups where caller's role holds at least one permission. Effective permissions per request = role permissions intersect expanded scopes (finer permission added later to a group keeps old tokens' meaning). 09 permissions in catalog: `oauth_grants:manage_any` (group `members:write`), `agent_access:manage` (group `settings:write`); both owner, admin. Policies call `authz.Can`, never role names. Table informative (derived from `roles.go`); on disagreement 00-accounts wins:

| Scope / action | owner | admin | member |
|---|---|---|---|
| conversations:read / write | yes | yes | yes |
| contacts:read / write | yes | yes | yes |
| articles:read / write | yes | yes | yes |
| members:read | yes | yes | yes |
| settings:read | yes | yes | yes |
| settings:write | yes | yes | no |
| webhooks:write | yes | yes | no |
| members:write | yes | yes | no |
| billing:read / billing:write | yes | yes | no |
| webhooks:read | yes | yes | no |
| notifications:read / write | yes | yes | yes (own) |
| ai:read | yes | yes | yes |
| ai:write / ai:keys | yes | yes | no |
| exports:read / write | yes | yes | no |
| changelog:read / write (13) | yes | yes | yes |
| feedback:read / write (14) | yes | yes | yes (write group also holds `feedback:delete`, admin+ only via role) |
| engage:read / write (15) | yes | yes | yes (`engage:manage` inside write group: admin+ only via role) |
| system:read / write | never via OAuth (instance admin PAT only, self-host) | | |
| Authorize OAuth client for self | yes | yes | yes |
| List own grants / revoke own | yes | yes | yes |
| List / revoke any member's grant | yes | yes | no |
| Change agent-access settings | yes | yes | no |
| Create PAT (00-accounts) | yes | yes | yes (abilities clamped to role) |
| Sign in to first-party app (`dimmi-mobile`) | yes | yes | yes (role-based, no scope list; same permissions as dashboard in each workspace) |

Effective permissions on each request = expanded token scopes AND current role permissions AND member active AND workspace not suspended AND `AccessPolicy` passes (00-accounts; reads grant `auth_methods`, `auth_time`, `device_id`). First-party (`auth:app`) tokens: no abilities, member role permissions + policies of `Dimmi-Workspace` workspace (api.md). Scope `*` never issued by OAuth (PAT only). `offline_access` accepted and ignored (refresh token always issued to `dimmi-cli`, CIMD, DCR clients with `refresh_token` grant type).

## Performance budgets

| Metric | Budget |
|---|---|
| Token auth (cache hit / miss) | p95 1 ms / 5 ms |
| `/oauth/token` | p95 30 ms (one tx) |
| Device poll | p95 10 ms; poll storm of 10k pending devices under 5% CPU |
| `/mcp` `tools/list` | p95 5 ms (precomputed bytes, ETag-able) |
| MCP read tools | p95 80 ms server time |
| MCP write tools | p95 150 ms |
| CIMD fetch on cold cache | 5 s timeout, p95 under 800 ms |
| CLI startup | under 30 ms; client flavor binary under 15 MB uncompressed, npm platform tarball under 7 MB |
| RSS impact of MCP + OAuth on idle server | under 3 MB |

## Observability

Metrics (Prometheus):
- `dimmi_oauth_tokens_issued_total{grant_type,client_type}`
- `dimmi_oauth_token_errors_total{endpoint,error}`
- `dimmi_oauth_refresh_reuse_total` (every reuse revokes the family)
- `dimmi_oauth_first_party_logins_total{redirect="claimed|scheme",platform}`
- `dimmi_oauth_first_party_active_grants` (gauge, sampled hourly)
- `dimmi_oauth_cimd_fetch_total{outcome}` + `dimmi_oauth_cimd_fetch_seconds`
- `dimmi_oauth_device_requests_total{status}`
- `dimmi_oauth_active_grants` (gauge, sampled hourly)
- `dimmi_mcp_requests_total{method,protocol_version,outcome}`
- `dimmi_mcp_tool_calls_total{tool,outcome,client_type}`
- `dimmi_mcp_tool_duration_seconds{tool}`
- `dimmi_mcp_response_tokens{tool}` histogram (estimated)
- `dimmi_mcp_truncations_total{tool}`
- `dimmi_mcp_confirmations_total{tool,path="elicitation|param",result="accept|decline|cancel|expired"}`

Logs: request id, workspace id, member id, grant id, client name, protocol version, tool name, outcome, duration. Never: tool arguments bodies, tokens, codes, user codes (log first 2 chars only; `filter[user_code]` query value redacted by access log middleware), `/oauth/callback/app` query, customer content. Debug level may log argument keys.

Traces: span per tool call (`mcp.tool/<name>`), child spans from actions; propagate `traceparent` from MCP `_meta` when present.

## Security and privacy notes (feature-specific)

1. Prompt injection into user's coding agent: tool outputs contain customer-written text; Claude Code/Cursor agents have shell access. Mitigations: customer content wrapped in `<untrusted source="customer" ...>` envelope with escaping; server `instructions` and tool descriptions state content is data, never instructions; sends and publishes require human confirmation; no tool executes code or fetches URLs. Documented in skill file and docs.
2. Exfiltration via reply: injected text could make agent paste secrets into customer reply. Confirmation preview shows full body (first 2,000 chars + length) and recipient; no bypass mode.
3. Token audience binding: OAuth tokens only accepted at resources in `audiences`; tokens never forwarded to other services (no passthrough).
4. Device code phishing: device grant only for `dimmi-cli` (system client); consent shows device name, IP, approximate location, time, and warning "Only approve if you just ran `dimmi login`". User code entry required to match terminal display.
5. CIMD SSRF: fetch via `a.HTTP` (`framework/httpclient`, code.dny.dev/ssrf dialer guard; private ranges blocked even on self-host unless `http_client.allow_private_networks`), 5 s timeout, 16 KB cap, no redirects followed (document URL must serve directly), `client_id` exact match.
6. Localhost redirect impersonation: consent shows redirect host and loopback warning; no auto-approve for loopback redirects.
7. DCR abuse: per-IP rate limit, name sanitization, "self-registered app (unverified)" label, workspace policy can block, prune unused.
8. Secrets in repo: `init` never writes identity secret to tracked file; checks `.gitignore` covers env file before writing; secret never printed (even `--debug`).
9. CLI credential storage: OS keychain first; file fallback 0600 with warning; refresh under file lock.
10. `requestState` sealed AEAD; binds principal (grant or PAT id), tool name, args digest, nonce, expiry 10 min; nonce single-use via `idempotency_keys` row (Postgres, durable). Valkey only for rebuildable caches (confirm-param dedupe, device poll throttle, token cache bust).
11. Audit log entries: grant created/revoked, refresh reuse, client disabled, agent-access settings change, PAT create/revoke (00-accounts). Actor for tool writes: member + via oauth client.
12. GDPR: 09 stores no customer content (no drafts at MVP). Grants deleted with user deletion (cascade).
13. Self-host without TLS: OAuth refuses non-HTTPS issuer except loopback hosts unless `oauth.allow_insecure_http`; CLI refuses http host unless `--allow-http` (loopback exempt).
14. First-party impersonation: any app can send `client_id=dimmi-mobile` and claim `com.trydimmi.app:` scheme. Consent skipped only for OS-verified claimed HTTPS redirect; scheme path always consent with device warning; PKCE binds code to starting app; callback page never forwards code to scheme.
15. First-party blast radius (user-wide grant): 15 min access tokens, refresh rotation + reuse detection, session list with device/IP/last used, revocation on password change/reset, 2FA disable, account delete, logout others; membership rechecked per request; tokens refused at `/mcp`.
16. Sudo from app: `auth_time` only from full browser login (`max_age=0`), never extended by refresh. In-app confirmation (`POST /user/confirmation`, password/email code/TOTP) sets grant `confirmed_at` via `ConfirmGrant`; `password.confirm` reads `max(auth_time, confirmed_at)`; also never extended by refresh.

## Metering impact (pricing.md section 2)

- API, MCP, CLI calls: never billable (roadmap "API unmetered").
- Reply sent via MCP/CLI: substantive teammate reply (D1, D2). Activates billing exactly like dashboard reply. Ledger (12) `actor_type=member` (confirmed by human); `sent_via` kept for D15 audit.
- Internal notes via `dimmi_add_note`: not replies (D8).
- `dimmi init` test conversation (if user sends from widget in test mode): D10 free.
- Spend cap enforced by 12 `InTx` listener inside `ReplyToConversation`: blocked send rolls back message; returns `spend_cap_reached` (MCP tool error / HTTP 423). `metered` middleware = fast pre-check only.

## Dependencies on other specs

| Spec | Needed |
|---|---|
| 00-foundation | ID prefixes `ocl_`, `ogr_`, `oar_`, `odr_`, `mem_` (registered); event catalog rows (listed); Actor `via` field; auth middleware hook for OAuth tokens; `framework/httpclient` (`a.HTTP`), `framework/http/rest` idempotency store, `ratelimit.Limiter` (Valkey), `pubsub.PubSub` (Valkey), `framework/broadcasting` (centrifuge), `framework/mcp` (go-sdk v1.8.0), `framework/oauth`, `framework/auth/token`, `framework/webhooks`, `encryption.KeyManager` app key for HKDF; `client.version` middleware (426) + `Dimmi-Version` header; registry enum lint (`oauth_grants.revoked_reason`) |
| 00-accounts | users, sessions (`sessions.authenticated_at` = last full login incl. factors, `sessions.auth_methods`, `sessions.device_id`, all defined by 00-accounts per extensibility.md section 5; login `reauth=1` mode forcing fresh login on existing session), `GET/DELETE /sessions` rendering `kind: app` rows from `ListUserAppGrants` and calling revoke actions, revoke hooks on password change/reset, 2FA disable, account delete, guards `auth:app`/`auth:token` delegating `dmi_oat_` to `ResolveAccessToken`, `password.confirm` honoring principal `AuthTime`, login redirect with `intended`, workspace_members roles + permission catalog (`permissions.go`, `roles.go`, `authz.Can`) incl. 09 permissions, `AccessPolicy` middleware, PAT API (`/api/v1/tokens` with abilities + nullable `expires_at`), `GET /api/v1/me`, audit_logs with `via_kind=oauth_client` + `via_id` (contracts.md Actor.Via; actor type stays `member`), lifecycle events `member.deactivated`, `member.role_changed`, `user.deleted`, `workspace.deleted` (all in catalog) |
| 01-messenger-widget | widget id (`wgt_`) API, SDK packages (Next.js, React, browser) + framework ids (`sdk-contract.md`, `BuildInstallSnippets` list) for `init` templates, `dimmi.config.ts`/`withDimmi` shape, `data-identity-url` script tag option, test-mode flag |
| 02-widget-identity | identity source helpers (`@dimmi/nextjs`, `@dimmi/server`) + raw snippets per stack via snippets API (`snippets.md`; tokens minted per request, never inlined in HTML, TTL 10 min), identity secret create API (returns secret once), claim names |
| 03-shared-inbox | `POST /conversations/{conv}/replies {body, then}` (no 09 hook in reply tx); actions `ReplyToConversation`, `AddNote`, `TagConversation`, `AssignConversation`, `CloseConversation`, `ReopenConversation`, `SnoozeConversation`, `UpdateContact`, search query builder, resources |
| 06-notifications | consumer of 09: `push_subscriptions.oauth_grant_id` references `oauth_grants` (on delete cascade) and 06 listens to `oauth_grant.revoked` to delete push rows of revoked first-party grants; `device_id` same value as grant `device_id` |
| 07-dimmi-agent | when MCP/CLI reply lands on conversation with `ai_state = active`, 07 rule decides (expected: human reply sets `handed_off`) |
| 08-help-center | article actions for REST/CLI (publish, archive; never from MCP), suggestions API (review queue, MCP proposals), knowledge search action, cold-start endpoint accepting uploaded Markdown files, article frontmatter fields for push/pull |
| 07-dimmi-agent (actions) | toolset `ai_actions` (8 tools, actions.md), CLI `dimmi actions ...` over `/ai/actions*`, YAML action format; scope group `ai:actions` |
| 08-help-center (docs sync) | toolset `docs_sync` (3 tools + prompt `dimmi_update_docs_for_change`), CLI `dimmi docs ...`, `dimmi github repos ...`; `verified` refused on MCP surface; no new scopes |
| 10-webhooks | CLI `dimmi webhooks ...`, `dimmi trigger` over endpoints/deliveries/tests REST; toolset `webhooks` |
| 16-importers | toolset `imports` (6 tools), `dimmi migrate` wizard (16 owns `migrate_command.go`), CLI `dimmi imports ...`; credentials never over MCP |
| 13-changelog, 14-feedback-roadmap, 15-announcements-surveys | MCP toolsets `changelog`, `feedback`, `engage` (tool files + REST in owner specs); CLI command groups; scopes `changelog:*`, `feedback:*`, `engage:*` (00-accounts groups) |
| 11-self-host | deploy button URLs (Railway, Render, Fly, DigitalOcean, Coolify, Dokploy) for `init`; `dimmi doctor` check: APP_URL https + reachable `/.well-known`; Valkey (`redis.url`) required in every deployment |
| 12-cloud-billing | `metered` check callable from MCP tool path |

## Contradictions and gaps in foundation docs

1. MCP spec moved: brief/roadmap assume DCR "required" and server-initiated elicitation. MCP 2026-07-28 deprecates DCR (CIMD primary) and replaces server-initiated requests with MRTR (`InputRequiredResult` + `requestState`), removes sessions and GET stream. Spec here follows 2026-07-28 with legacy compat.
2. `api.md` lists actor "OAuth client" but `messages.author_type` has no such value. Resolved by 00-foundation/contracts.md: `Actor{Type, ID, Via}`; author stays `member`; OAuth client recorded as `Via.Kind=oauth_client` (PAT: `personal_token`) in `messages.metadata.sent_via` and `audit_logs.via_kind`.
3. Resolved: catalog has `member.deactivated`, `member.role_changed`, `user.deleted` (00-accounts). No `member.removed` exists; removal = deactivation.
4. Resolved: ownership map lists all `oauth_*` tables.
5. Conventions directory layout had no home for CLI client code. Resolved: conventions adds `app/client/` (client-only packages, depguard-isolated from server packages) and `app/console/commands/client/`.
6. Conventions name commands colon style (`channels:add`); roadmap and brief use spaces (`dimmi conversations list`). Rule: server/admin commands colon style (`channels:add`, `key:generate`; dev-only `make:*` generators too); client resource commands space-separated (`dimmi conversations list`, gh/stripe convention). Stated in cli.md.
7. Resolved: `ocl_`, `ogr_`, `oar_`, `odr_`, `mem_` in data-model.md registry. `oauth_auth_codes`, `oauth_access_tokens`, `oauth_refresh_tokens` never exposed by id (no prefix needed).
8. `tech-stack.md` suggests `zitadel/oidc` for OAuth provider; conventions say own OAuth 2.1. Decision (confirmed by `research/build-vs-reuse.md` section 2): AS core custom (MCP subset, CIMD, DCR, RFC 8707, device flow, opaque `dmi_oat_` tokens, first-party rules). Rejected: ory/fosite (last release 2024-12, stalled; no DCR, CIMD, 8707), zitadel/oidc (OIDC-centric, `op.Storage` needs key sets + ID-token plumbing; no DCR, CIMD, 8707), Ory Hydra (separate server). Reused: MCP go-sdk v1.8.0 `oauthex` (metadata, DCR types, `MatchesResource`) + `auth` (`RequireBearerToken`, PRM handler, `TokenVerifier`), `framework/oauth` + `framework/auth/token`, golang.org/x/oauth2 on CLI side.
9. `api.md` rate-limit table has no OAuth endpoint buckets; added here (device 10/min/IP, DCR 20/hour/IP, lookups 10/min/user).
10. Drafts: retired 2026-11-02 (later: reply-drafts). Foundation `rest.md`/`api.md` rows for drafts and `draft_id` on `POST /replies` to drop (00-foundation owner).
11. `security.md` says self-host association files list official app ids by default. True, but official app cannot claim custom self-host domains (associated domains / App Link hosts compiled into app). Self-host with official app always uses scheme + consent. Files stay harmless; white-label builds set own ids.
12. `rest.md` exceptions list `/oauth/consent`; real web paths are `/oauth/consent/{oar}`, `/device`, and new `/oauth/callback/app` (claimed-link fallback page).
13. Resolved (extensibility.md section 5): 09 copied "session `authenticated_at`" into `auth_time` but 00-accounts had no such column. Now `sessions.authenticated_at`, `auth_methods`, `device_id` defined by 00-accounts; 09 reads them.
14. Resolved: 03 reply takes `then: keep|open|close|snooze` (no closed-conversation error code). Now `reopen`/`close_after` are MCP args / CLI flags translated to `then`; closed-without-reopen guard is 09 tool layer, 422 `invalid_transition`.
15. 08 README "MCP catalog reconciliation" still names 09 tools `dimmi_suggest_article`, `dimmi_create_article`, `dimmi_publish_article`, `dimmi_delete_article`. Resolved on 09 side: none exist; 09 catalog = `dimmi_propose_article` + `dimmi_propose_article_edit` (core) + 08 `help_center` toolset; no MCP article publish/accept/delete even behind confirm gate (08 rule 2 wins). 08 owner: replace that paragraph with "resolved, 09 mcp.md Feature toolsets".
16. Resolved: 08 human-review refusal uses no token field (08 updated). Metrics label `client_type` = `oauth_clients.kind` (system/dynamic/metadata_document). Rule for 08: surface `mcp`, else principal Via `oauth_client` and client id not `dimmi-cli`, else grant `first_party` (app) = human.

## Open questions (recommended default in bold)

1. Should replies sent from AI clients show "AI-assisted" label to customer? **No** (human approved; metadata only).
2. Refresh token idle window: **90 days sliding, no absolute** (vs never expiring).
3. Device grant for third-party clients (e.g. other CLIs via CIMD)? **No in MVP** (phishing surface).
4. Publish Dimmi CLI to Homebrew core vs own tap: **own tap at launch**, core after 75 stars threshold met.
