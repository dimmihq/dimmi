# API conventions

Style: caveman. Scope trim 2026-11-02: drafts, `/survey-forms/*` (now `/csat-ratings/{svr}`), changelog own domain removed. Applies to REST API (`/api/v1`), widget API (`/widget/v1`), MCP tools (same resources), CLI (wraps REST). Laravel JSON shapes.

## Surfaces

| Surface | Base | Auth | Audience |
|---|---|---|---|
| REST API | `/api/v1` | guards below | dashboard SPA, Dimmi mobile/desktop apps, developers, CLI, MCP, third-party apps |
| Widget API | `/widget/v1` | widget app id + visitor session token | Messenger widget, headless SDK, `window.Intercom` shim (16), customer mobile SDKs (post-MVP: mobile SDKs). Feature routes under it follow normal REST rules (07 `conversations/{conv}/action-calls/{aax}`, 13 changelog, 14 feedback, 15 engage) |
| Webhooks in | `/webhooks/{provider}/{purpose}` (e.g. `/webhooks/ses/inbound`, `/webhooks/slack/events`, `/webhooks/postmark/events`) | provider signature (security.md "Webhooks in") | mail delivery events (`/webhooks/{ses,postmark,resend,mailgun,sendgrid}/events`), SES inbound (04), Slack, Discord interactions, Telegram, WhatsApp, Stripe, GitHub (`/webhooks/github/repositories/{ghr}`: one per repo, 08 shared layer for 08 knowledge, 08 docs sync, 13 changelog) |
| Realtime | `GET /connection/websocket` (centrifuge protocol) and `GET /connection/uni_sse` (widget, receive-only) | connect: session cookie, bearer in connect `token`, or 60 s widget connect token (`POST /widget/v1/realtime-tokens`); subscribe: `routes/channels.go` authorizers | dashboard (centrifuge-js), apps, CLI, integrations, widget (`EventSource`). events-and-realtime.md |
| MCP | `/mcp` (streamable HTTP) and `dimmi mcp` (stdio) | OAuth 2.1 or token | AI agents |
| OAuth | `/oauth/*`, `/.well-known/*` | per RFC | apps, CLI, MCP clients |
| Web | `/`, `/login`, `/auth/{provider}/*`, `/device`, consent page, unsubscribe/confirmation pages, `/csat-ratings/{svr}` (15: signed CSAT rating page; GET renders, POST stores) | session or signed link | humans in a browser |
| Help center hosts | `{sub}.help...`, custom domains: articles, `/api/answers`, changelog (13: `/changelog`; own domain later: `changelog-custom-domain`), feedback portal (14: `/feedback`, `/roadmap`, `/api/feedback/*`) | none, portal session (14) | public |

One API. Dashboard SPA, Dimmi mobile app and third parties call the same `/api/v1` routes. No private endpoints, no `/internal`, no `/mobile` prefix. If an app needs it, API has it. Separate prefixes exist only where the caller is a different kind of principal (`/widget/v1`: end customers; `/webhooks`: providers; `/oauth`, `/mcp`, `/connection`: protocols).

REST rules and the full old-to-new route map: [rest.md](rest.md). Binding.

## Who calls what (guards)

Callers differ by guard, not by URL. Four caller classes:

| Class | Credential | Guard | Workspace from | Permission check | Can reach |
|---|---|---|---|---|---|
| Browser (dashboard SPA, same origin) | `__Host-dimmi_session` cookie + cross-origin protection (Go `http.CrossOriginProtection`: `Sec-Fetch-Site`, fallback `Origin` vs `Host`) | `auth:session` | `Dimmi-Workspace` header | role permissions | everything a user can do |
| First-party app (Dimmi mobile, later desktop) | OAuth token `dmi_oat_` from system client with `first_party = true`, user-scoped grant | `auth:app` | `Dimmi-Workspace` header | role permissions (same as session) | everything a session can, except browser-only routes |
| Personal access token | `dmi_live_` / `dmi_test_` | `auth:token` | token's workspace | role permissions ∩ ability groups | workspace routes, `GET /me`, `GET /user` |
| Third-party OAuth (CLI, MCP clients, integrations) | OAuth token `dmi_oat_` from workspace-scoped grant | `auth:token` | grant's workspace | role permissions ∩ scope groups | workspace routes, `GET /me`, `GET /user` |

Middleware groups (Laravel names; implemented in 00-accounts, OAuth resolver from 09):

| Group | Accepts | Used for |
|---|---|---|
| `auth:session` | browser session only | browser-only flows: OAuth provider link start, consent and device approval pages, provider install flows (Slack, Discord, WhatsApp embedded signup), billing redirect sessions (Stripe Checkout/Portal). No CSRF token endpoint, no `XSRF-TOKEN` cookie |
| `auth:user` | `auth:session` or `auth:app` | account routes: `/user/*`, `/sessions`, `/user/memberships`, `POST /workspaces`, `/email-changes`, `/email-verifications` (create) |
| `auth` (any) | `auth:session`, `auth:app`, `auth:token` | workspace routes (conversations, contacts, settings ...) |

Rules:
- Token kind is decided by lookup, never by client claim. A `dmi_oat_` token is `auth:app` only when its grant belongs to a system client flagged `first_party` and the grant has no workspace. Every other `dmi_oat_` is `auth:token`.
- PATs and third-party OAuth tokens never reach `auth:user` routes. Agents and integrations cannot change passwords, 2FA, sessions or memberships. 403 `first_party_only`.
- Bearer of any kind (PAT, third-party OAuth, first-party app) on `auth:session` routes: 403 `session_required` with `message` telling caller to open the dashboard URL. Used for flows whose provider callback relies on the browser cookie (Slack/Discord installations, WhatsApp embedded signup, Stripe Checkout/Portal sessions).
- Abilities apply to `auth:token` only. Session and first-party apps use role permissions and policies, same as dashboard.
- Workspace stack `W` = `workspace` + `access.policy` (renamed from `two_factor.enforced`, same position; contracts.md "AccessPolicy") + suspension check.
- Client version gate `client.version` runs on every `/api/v1` route and `/oauth/token` before auth (see "Headers"). OAuth mode on `/oauth/token`: 400 OAuth error `{"error": "invalid_request", "error_description": "client_upgrade_required", "min_version", "upgrade_url"}` instead of 426 JSON (OAuth clients parse RFC 6749 shape).
- Bearer wins when both cookie and bearer present; cross-origin protection applies only to cookie-authenticated unsafe requests (bearer requests are not browser-ambient).
- `password.confirm` (sudo) satisfied by: session confirmation (`POST /user/confirmation`), or first-party token whose `auth_time` is within `auth.password_confirmation.ttl` (app re-authenticates in browser sheet with `max_age=0`), or `POST /user/confirmation` from app with `method` `password`, `email_code` or any factor type with `Confirmation` capability (MVP `totp`). Method used stored as `sessions.confirmed_method`.
- CORS on `/api/v1`: any origin for bearer requests, `Access-Control-Allow-Credentials` never set. Cookies only work same-origin (SPA served by monolith).
- Rate limit bucket: session and first-party app share per-user bucket; tokens per token.
- Workspace header rules: session and first-party app requests to workspace routes need `Dimmi-Workspace: ws_...` (400 `workspace_required` when missing and user has more than one workspace; single workspace implied). Token requests ignore the header; mismatch with token workspace returns 403 `workspace_mismatch`.

Endpoint tables mark auth with letters: `S` = `auth:session`, `U` = `auth:user`, `A` = `auth` (any; abilities for tokens), `G` = guest, `W` = workspace resolved. Feature tables that list only an ability mean `A`.

## OpenAPI

- huma generates OpenAPI 3.1 at `/api/v1/openapi.json` and `/widget/v1/openapi.json` (built at runtime from Go types).
- `dimmi openapi:export [--out openapi/]` (server command, no DB, no network) writes `openapi/api-v1.json` and `openapi/widget-v1.json` from the same registry, sorted keys, stable output (byte-identical across runs). CI exports from `origin/main` and branch, runs `oasdiff breaking --fail-on ERR` per pair (code-style.md). Files also feed CLI codegen (09) and docs.
- Spec is source for: docs site, CLI commands, MCP tool schemas, TypeScript SDK (later, generated).
- Every operation has `operationId` = route name (`conversations.replies.store`), summary, tags, examples.
- Breaking change check in CI (oasdiff on exported files). Additive only inside v1.

## Resource shape

```json
{
  "data": {
    "id": "conv_01jbc9x...",
    "object": "conversation",
    "number": 1042,
    "status": "open",
    "created_at": "2026-11-01T10:00:00Z",
    "contact": { "id": "ctc_...", "object": "contact", "name": "Bob" },
    "links": { "self": "https://app.trydimmi.com/api/v1/conversations/conv_..." }
  }
}
```

- Single: `{ "data": {...} }`. Collection: `{ "data": [...], "meta": { "next_cursor": "...", "prev_cursor": null, "per_page": 25 } }`.
- `object` field on every resource (agents and SDKs dispatch on it).
- Timestamps ISO 8601 UTC with `Z`. Null when unset, key always present.
- IDs prefixed strings (`conv_`). Never raw UUID.
- Enums lowercase snake_case strings.
- Related resources: compact form by default (`id`, `object`, display fields). Full form via `include=contact,assignee,tags`.

## Query parameters (spatie/laravel-query-builder style)

| Param | Example | Notes |
|---|---|---|
| `filter[field]` | `filter[status]=open,snoozed&filter[assignee]=me` | allowlist per endpoint; unknown filter returns 400 `invalid_filter` |
| `sort` | `sort=-last_message_at` | allowlist; `-` desc |
| `include` | `include=contact,tags` | allowlist |
| `cursor`, `limit` | `limit=50` | cursor pagination only; default 25, max 100 |
| `q` | `q=refund` | full-text search where supported |

Cursor opaque, encodes sort key + id, signed (tamper returns 400). Stable under inserts.

## Writes

- `POST` create, `PATCH` partial update (only sent fields change), `DELETE` remove. No `PUT`.
- `Idempotency-Key` header accepted on every POST. Same key + same body within 24 hours returns stored response with header `Idempotent-Replayed: true`. Same key + different body returns 422 `idempotency_key_reused`.
- Optimistic concurrency: resources with `updated_at`; `If-Unmodified-Since` optional; conflict returns 409 `stale_resource`.
- State changes: `PATCH` the field (`{"status": "closed"}`), never verb paths. See [rest.md](rest.md).
- Bulk operations: `PATCH /api/v1/conversations` with `{ "ids": [...], "changes": { "status": "closed" } }`, max 100 IDs, returns per-id result. Partial success allowed, never silent. Rule 8 in [rest.md](rest.md).

## Auth and abilities

- Token header: `Authorization: Bearer dmi_live_<random>` (personal access token) or OAuth access token `dmi_oat_<random>`.
- Token prefix visible in UI, secret shown once, stored as SHA-256.
- Abilities (Sanctum style) = permission groups: `conversations:read`, `conversations:write`, `contacts:read`, `contacts:write`, `articles:read`, `articles:write`, `settings:read`, `settings:write` (also covers channels and audit log read via `settings:read`), `webhooks:read`, `webhooks:write`, `members:read`, `members:write`, `notifications:read`, `notifications:write`, `ai:read`, `ai:write`, `ai:keys`, `ai:actions` (07 agent actions: connections, secrets, publish; not implied by `ai:write` or `settings:write`), `billing:read`, `billing:write`, `exports:read`, `exports:write`, `system:read`, `system:write` (instance admin only, self-host), `changelog:read`, `changelog:write` (13), `feedback:read`, `feedback:write` (14), `engage:read`, `engage:write` (15). `*` = all. Missing ability returns 403 `missing_ability` with `required_ability`.
- Permissions (P2): catalog in `app/auth/permissions.go` (00-accounts), strings `resource:action`. Each permission belongs to exactly one ability group (`conversations:write` group holds later `conversations:assign`, `conversations:delete`); finer permissions later join a group, so existing tokens keep meaning. MVP catalog = ability list above + finer permissions inside groups: owner-only `workspace:delete`, `members:manage_owners`, `tokens:manage_any`, `billing:cap_unrestricted` (`billing:write` group); owner + admin `audit_logs:read` (`settings:read` group), `billing:read`, `billing:write`, `feedback:delete` (`feedback:write` group), `feedback:manage` (`settings:write` group), `ai_actions:manage` (`ai:actions` group), `github:manage`, `docs_sync:manage`, `imports:manage`, `imports:rollback` (`settings:write` group); owner + admin + member `ai_actions:read` (`ai:read`), `ai_action_calls:approve` (`ai:write`), `github:read` (`settings:read`); owner + admin `imports:read` (`settings:read`). Feature permission rows: owner spec Permissions matrix (07 actions.md, 08 docs-sync.md, 16 README). Role to permission set only in `roles.go`. Policies call `authz.Can(principal, permission)`, never compare role names (exceptions: owner transfer, last-owner checks).
- Effective permissions: session and app = role permissions. Token = role permissions ∩ expanded abilities (replaces "ability ceilings by role" table; same outcomes; demotion shrinks token live). `*` expands to role permissions at request time.
- One vocabulary: OAuth scopes (09), token abilities, permission groups = same strings. Marketplace apps later request same strings.
- `GET /me` returns `member.permissions` (effective list for caller in workspace). Dashboard and apps gate UI on `permissions`, never on `role` (custom roles later keep old app builds working).
- Actor: every write records `Actor{Type, ID, Via}` (`specs/00-foundation/contracts.md`). OAuth client, token, CLI, MCP recorded as `Via`, not as author type.
- PATs and third-party OAuth tokens are workspace-scoped. Session and first-party app requests choose workspace via `Dimmi-Workspace: ws_...` header (see "Who calls what").

## Rate limits

| Bucket | Default |
|---|---|
| API per token | 600 requests/minute, burst 100 (entitlement key `api.rate_limit_per_minute`) |
| API per user (session + first-party apps) | 1,200/minute (entitlement key `api.user_rate_limit_per_minute`) |
| Widget per visitor | 60 messages/minute, 600 requests/minute |
| Widget per IP | 1,200 requests/minute (NAT friendly) |
| Login per IP + email | 10/minute, lockout escalates |
| Webhooks in per provider connection | 3,000/minute |
Headers: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After` on 429. Self-host configurable, can disable. API defaults read from `Entitlements.Limit` (contracts.md), not direct config: per-contract limits = override row.

Added 2026-11-04 (build: `00-accounts/rate-limits.md`):
| Bucket | Default |
|---|---|
| API per workspace, token traffic only | 6,000/minute (entitlement key `api.workspace_rate_limit_per_minute`) |
| Token burst | limit/6 per 10 s (the "burst 100" above) |
| Failed credential per IP | 60/minute |
| SCIM per token | 600/minute (21) |
Also on every limited response `X-RateLimit-Scope` (`token`, `burst`, `user`, `workspace`); 429 adds `X-RateLimit-Reset`. Body `{message, code: "too_many_requests"}`. Sign-in throttles answer `too_many_attempts`. Fairness limits fail open when Valkey is down. Applied to every authenticated `/api/v1` route and `/mcp`. Widget buckets above owned by 01.

## Errors

See conventions. Always JSON, one envelope (`framework/http` `Errors`): `{message, code, errors?, ...meta}`. `code` stable (programs branch on it, never on `message`); fault details (`key`, `limit`, `from`, ...) sit top level beside `message`/`code` and never change with locale. Unknown route returns 404 JSON on API prefixes. Method not allowed 405 with `Allow` header. Request body over 1 MB (non-upload) returns 413 `payload_too_large`. Non-fault errors: bare 500 `internal_error` (logged + reported, nothing leaked).

Localized messages (i18n first class):
- Request locale picked by `Locale` middleware: app preference hook (`foundation.Options.LocalePreference`: user `locale`, else workspace `locale`) > `Accept-Language` (matched to `app.locales`) > `app.locale` (default `en`).
- `message` translated by code: key `errors.<code>` in `lang/<locale>/errors.json` with fault details as args; missing key keeps original English text. Framework English messages in `framework/http/lang/en/errors.json` (`validation_failed` "The given data was invalid.", `not_found` "Not Found.", ...), app adds own codes.
- Validation 422 `validation_failed`: from `*validation.Error`; `errors` = `{"<field path>": ["message", ...]}`, paths dotted with indexes (`emails.0`, `address.city`), field keys = JSON names, never translated. Messages Laravel-style English by default ("The email field must be a valid email address."), translated via `validation.<rule>` keys (plural forms by CLDR rules). `message` = first message + "(and N more errors)".

```json
{"message": "The emails.0 field must be a valid email address.", "code": "validation_failed",
 "errors": {"emails.0": ["The emails.0 field must be a valid email address."]}}
```

Cross-cutting codes:

| Status | Code | When | Extra body fields |
|---|---|---|---|
| 400 | `invalid_api_version` | `Dimmi-Version` request header not current version | `supported_versions` |
| 403 | any `AccessPolicy` failure (MVP `two_factor_required`; later `reauthentication_required`, `ip_not_allowed`) | workspace/org policy not met | `requirement` (`two_factor` \| `reauthentication` \| `sso` \| `ip`), `action_url`. Clients branch on `requirement` generically |
| 409 | `sso_required` | reserved (later): email domain enforces SSO; domain-level, no account enumeration | `redirect_url` |
| 422 | `setting_locked` | reserved (later): workspace `PATCH` of setting locked at organization scope | `setting` |
| 403 | `session_required` | bearer (PAT, OAuth, first-party app) on `auth:session` route | `message` points to dashboard URL |
| 403 | `first_party_only` | PAT or third-party OAuth on `auth:user` route | |
| 403 | `missing_ability` | token lacks ability group | `required_ability` |
| 400 | `workspace_required` / 403 `workspace_mismatch` | `Dimmi-Workspace` rules ("Who calls what") | |
| 422 | `entitlement_limit_reached` | count cap from `Entitlements.Limit` reached (contracts.md "Entitlements") | `key`, `limit`, `used` |
| 429 | `entitlement_limit_reached` | per-period cap (daily, hourly, monthly) reached | `key`, `limit`, `used`; `Retry-After` |
| 422 | `invalid_transition` | illegal state change via `PATCH` (rest.md rule 3) | `from`, `to` |
| 426 | `client_upgrade_required` | `Dimmi-Client-Version` below `apps.min_versions.<platform>` | `min_version`, `upgrade_url` |

Import seam codes (owners): `import_channel_cannot_send` (422, `reason`; reply to imported conversation with no email route; 03 raises, 16 documents), `merge_imported_target` (422; live conversation merged into imported target; 03), `imported_message` (not HTTP: `SendError` code `ErrPermanent` from 04/05 drivers, 07 gate skip reason; imported messages never sent).

## Headers

| Header | Direction | Rule |
|---|---|---|
| `Dimmi-Version` | response, every `/api/v1` response | current API version (`api.version` config, `2026-11-01`). Reserved for date versions later (Stripe model: response transformers keyed by version, no SDK change) |
| `Dimmi-Version` | request, optional | absent or equal to current = OK; unknown = 400 `invalid_api_version`. SDKs, CLI, MCP, apps send it from day one |
| `Dimmi-Client-Version` | request, first-party apps and CLI | `<platform>/<semver>`: `ios/1.4.0`, `android/1.4.0`, `cli/0.9.0`, later `desktop/...`. Middleware `client.version` on `/api/v1` and `/oauth/token`: when `apps.min_versions.<platform>` set and header lower, 426 `client_upgrade_required` `{min_version, upgrade_url}` (`upgrade_url` from `apps.upgrade_urls.<platform>`); `/oauth/token` OAuth mode: 400 `invalid_request` + `error_description: client_upgrade_required` (see "Who calls what", 09). Header missing (browsers, third parties) = allowed. Malformed = ignored. Config empty at MVP = noop |
| `Dimmi-Workspace` | request, session and app | see "Who calls what" |
| `Idempotency-Key`, `Idempotent-Replayed` | request / response | see "Writes" |

## Versioning

- `/api/v1` frozen for breaking changes. Additive fields and endpoints anytime.
- Deprecations: `Deprecation` and `Sunset` headers, changelog entry, 6 months minimum.
- Webhook payloads follow same resource versions; `webhook_endpoints.api_version` pins payload shape per endpoint (10).
- Version header: `Dimmi-Version` (see "Headers"). Clients discover server capabilities, servers refuse too-old clients (P7).
- Sync-safe resources (tombstone rule, for mobile delta sync later): resources apps sync (conversations, messages, contacts, notifications, tags) keep soft delete (`deleted_at`) or emit delete event; every change bumps `updated_at`. Hard delete only for GDPR erase and purge. Later `filter[updated_since]` + `sort=updated_at` + `filter[trashed]=with` additive.
- Extensible lists are arrays of objects, never arrays of strings (P6: `methods`, `providers`, `factors`), so new fields per entry stay additive under oasdiff.

## Endpoint ownership

| Area | Spec |
|---|---|
| auth, users, workspaces, members, teams, invitations, tokens, audit logs | 00-accounts |
| conversations, messages, notes, contacts, companies, tags, saved replies, search, inboxes, attachments | 03-shared-inbox |
| widget API, widget settings | 01-messenger-widget |
| identity verification secrets, JWT | 02-widget-identity |
| email domains, addresses | 04-email-channel |
| channel connections (all types) | 05-chat-app-channels (generic CRUD + per-type connect flows) |
| notification preferences, push subscriptions | 06-notifications |
| ai agents, provider keys, playground, runs, agent actions (`/ai/action-templates`, `/ai/action-connections*`, `/ai/actions*`, `/ai/action-imports`, `/ai/action-calls*`, widget `/widget/v1/conversations/{conv}/action-calls*`) | 07-dimmi-agent |
| help centers, collections, articles, suggestions, knowledge sources | 08-help-center |
| github connections, github repositories, docs sync sources/runs, article source refs, freshness checks, `/webhooks/github/repositories/{ghr}` | 08-help-center (docs-sync.md) |
| OAuth, device flow, MCP, CLI | 09-agent-native |
| webhook endpoints, deliveries | 10-webhooks |
| system settings, backup, health | 11-self-host |
| usage, billing, spend caps, `entitlement_overrides` (staff command) | 12-cloud-billing |
| `GET /workspace/entitlements` (controller over `Entitlements` contract, both editions) | 00-foundation |
| changelog, changelog entries, labels, subscribers, mailings, sources, generations, `/widget/v1/changelog*` | 13-changelog |
| feedback boards, posts, votes, comments, requests, suggestions, imports, portal routes, `/widget/v1/feedback*` | 14-feedback-roadmap |
| announcements, surveys, segments, engage settings, engage assets, survey templates, `/conversations/{conv}/survey-responses`, `/contacts/{ctc}/survey-responses`, `/contacts/{ctc}/engage-receipts`, widget engage routes, `/csat-ratings/{svr}` | 15-announcements-surveys |
| import sources, imports, import runs, mappings, records, files (`/import-sources`, `/imports*`, `/import-runs*`, `/import-mappings*`); Intercom shim article id lookup (01 widget route, 16 resolver) | 16-importers |
| `POST /widget/v1/realtime-tokens` (widget connect token), realtime endpoints `/connection/*` | 01 (token controller), foundation (`/connection/*`) |

## Health and meta

- `GET /up` returns 200 when process alive (Laravel 11 health route).
- `GET /ready` checks Postgres, migrations current, Valkey (`PING` on `redis.url`), required drivers reachable. Used by load balancers.
- `GET /api/v1/me` returns actor, workspace, abilities, `member.permissions`, `instance` (incl. `region`, `server_version`). CLI and MCP call first.
- `GET /api/v1/auth/config` (guest, 00-accounts) = capability discovery, read by apps before login: `server_version`, `api_version`, `region`, `capabilities` (string tokens from registry, e.g. `auth_factors`, `push_web`), `min_client_versions: {ios, android, cli}`, `providers` (objects `{key, name, protocol, redirect_url}`), `factors` (objects `{type, name}`). Newer app against older self-host hides missing features instead of crashing.
