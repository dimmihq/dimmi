# 09 OAuth 2.1 authorization server

Style: caveman. Verified against MCP authorization spec revision 2026-07-28 (modelcontextprotocol.io/specification/2026-07-28/basic/authorization, fetched 2026-11-01) and RFCs 6749/OAuth 2.1 draft 13, 7009, 7591, 8252, 8414, 8628, 8707, 9207, 9728, draft-ietf-oauth-client-id-metadata-document-00.

## Spec facts that drive design (verified)

Implementation reuse (`research/build-vs-reuse.md` section 2): AS core custom (no maintained Go lib covers MCP profile: CIMD, DCR, RFC 8707, opaque tokens, first-party rules). Metadata JSON (RFC 8414 `oauthex.AuthServerMeta`, RFC 9728 `oauthex.ProtectedResourceMetadata`), DCR request/response/error types, `MatchesResource` from MCP go-sdk v1.8.0 `oauthex`; bearer check + PRM handler + `WWW-Authenticate` from go-sdk `auth` (`RequireBearerToken`, `ProtectedResourceMetadataHandler`, our resolver as `TokenVerifier`); PKCE, redirect matching, RFC 8707 canonicalization, device codes, CIMD fetch from `framework/oauth`; token format `framework/auth/token`. CLI client uses golang.org/x/oauth2.

Framework status (2026-11-02): `framework/oauth` is a full OAuth 2.1 AS with storage + migration (framework set: `oauth_clients`, `oauth_authorization_codes`, `oauth_grants`, `oauth_tokens` with `kind` access|refresh): clients (`CreateClient`, config clients), authorization code + PKCE S256 (`Approve`/`Deny`), rotating refresh tokens with reuse detection (family revoke), revocation (RFC 7009), metadata (RFC 8414), DCR (rate limited via `a.Limiter`), `ConfirmGrant`, bearer resolver for guards `token` (third party) and `app` (first party). Deferred in framework (09 builds on top or waits): RFC 8707 resource indicators/audiences (needed by MCP), CIMD client ID metadata documents, device grant (RFC 8628), consent-skip rules, last-use tracking. Product columns (device fields, `revoked_reason`, `auth_methods`, client kind/metadata) added by 09 app migrations with `ALTER TABLE` on framework tables.

| Fact (MCP 2026-07-28) | Our decision |
|---|---|
| MCP servers MUST implement RFC 9728 PRM; PRM MUST list `authorization_servers` | PRM at `/.well-known/oauth-protected-resource/mcp` + root; 401 `WWW-Authenticate` carries `resource_metadata` |
| AS MUST provide RFC 8414 or OIDC discovery; clients try `/.well-known/oauth-authorization-server` first for path-less issuer | RFC 8414 only; issuer = `APP_URL` without path; `openid-configuration` returns 404 |
| Clients MUST refuse AS without `code_challenge_methods_supported` | `["S256"]` always present |
| CIMD SHOULD be supported; DCR MAY, and DCR now deprecated | CIMD primary (`client_id_metadata_document_supported: true`); DCR kept for older clients |
| Clients MUST send `resource` (RFC 8707) in authorize and token requests; servers MUST validate audience | token `audiences` column; `/mcp` and `/api/v1` check audience |
| AS SHOULD send `iss` in authorization response (RFC 9207), advertise `authorization_response_iss_parameter_supported` | always send `iss`, including error redirects |
| Public clients: AS MUST rotate refresh tokens | rotation on every refresh, reuse detection |
| AS endpoints MUST be HTTPS; redirect URIs MUST be localhost or HTTPS | enforced; self-host LAN escape hatch `oauth.allow_insecure_http` (logged warning, `dimmi doctor` red) |
| AS MUST validate exact redirect URIs | exact string match; only exception: loopback port (RFC 8252 7.3) |
| AS MUST display redirect URI hostname on consent for CIMD; SHOULD warn on localhost-only | consent payload `redirect.host`, `redirect.warning` |
| MCP servers SHOULD NOT put `offline_access` in PRM `scopes_supported` | PRM omits; AS metadata lists it |
| Runtime insufficient scope: 403 + `WWW-Authenticate: Bearer error="insufficient_scope", scope=..., resource_metadata=...` | implemented on `/mcp` and `/api/v1` for OAuth tokens |
| stdio servers SHOULD NOT use OAuth spec, get credentials from environment | `dimmi mcp --in-process` reads `DIMMI_TOKEN`; bridge mode uses CLI profile |

## Identifiers and token formats

| Thing | Format | Storage |
|---|---|---|
| Issuer | `APP_URL` (e.g. `https://app.trydimmi.com`), no trailing slash | config |
| Resources (canonical) | `{APP_URL}/mcp`, `{APP_URL}/api/v1`, `{APP_URL}` (umbrella) | |
| System client ids | `dimmi-cli` (CLI), `dimmi-mobile` (first-party app) | seeded rows |
| DCR client id | `dcr_` + 26 Crockford base32 chars (128 bits) | |
| DCR client secret | `dmi_ocs_` + 43 base62 chars (256 bits) | SHA-256 |
| Authorization code | `dmi_oac_` + 43 base62 | SHA-256 |
| Access token | `dmi_oat_` + 43 base62 | SHA-256 |
| Refresh token | `dmi_ort_` + 43 base62 | SHA-256 |
| Device code | `dmi_odc_` + 43 base62 | SHA-256 |
| User code | 8 chars from `BCDFGHJKLMNPQRSTVWXZ` (20 letters, no vowels, RFC 8628 6.1), shown `XXXX-XXXX` | plaintext while pending (needed for lookup), unique among pending |

Prefixes registered with GitHub secret scanning partner program (cloud) and redaction middleware patterns (`dmi_`). Tokens opaque (not JWT): instant revocation, no key rotation burden, cache handles performance.

## Scopes

- Scope strings = abilities from `00-foundation/api.md` = permission groups of 00-accounts catalog `app/auth/permissions.go` (one vocabulary for OAuth scopes, PAT abilities, permission groups; extensibility.md sections 4, 11; never copied): `conversations:*`, `contacts:*`, `articles:*`, `settings:*`, `webhooks:read|write`, `members:*`, `notifications:read|write`, `ai:read|write|keys`, `billing:read|write`, `exports:read|write`. Never grantable via OAuth: `system:*`, `*`. Plus `offline_access` (accepted, no effect). New ability added to api.md becomes grantable automatically (role ceiling clamps).
- `*` never grantable via OAuth.
- Hierarchy: `X:write` implies `X:read` (MCP spec: servers MUST account for scope hierarchies). Stored scopes normalized: when `write` present, `read` stored too.
- Unknown scope in request: `invalid_scope` error (authorize: redirect with error; device/token: 400).
- Empty scope: default by resource. `/mcp` only: PRM default set (7 scopes in README). `/api/v1` or `dimmi-cli`: all OAuth-grantable abilities, clamped at consent.
- Consent clamp: granted = requested AND user selection AND groups where role (`roles.go`) holds at least one permission. Clamped-away scopes listed in token response `scope` (client sees actual set).
- Per-request effective permissions = expanded token scopes intersect current role permissions via `authz` (role downgrade takes effect next request without token change). Finer permissions added later join an existing group, so issued tokens keep meaning; marketplace apps later request same strings.
- `first_party`: internal marker scope, only on grants of `first_party` system clients (section "First-party apps"). Means "role-based, no ability list". Never in metadata `scopes_supported`, never on consent list, never grantable to other clients (requested by any other client: `invalid_scope`).

## Resource indicator rules (RFC 8707)

- Accept multiple `resource` params. Each canonicalized: lowercase scheme + host, no default port, no trailing slash, no fragment, no query.
- Allowed values: `{APP_URL}`, `{APP_URL}/mcp`, `{APP_URL}/api/v1`. Anything else: `invalid_target`.
- Missing `resource`: audience `[{APP_URL}]` (umbrella). Metric `dimmi_oauth_missing_resource_total{client_type}` tracks non-compliant clients.
- Audience check: `/mcp` accepts token whose audiences contain `{APP_URL}/mcp` or `{APP_URL}`; `/api/v1` accepts `{APP_URL}/api/v1` or `{APP_URL}`. Fail: 401 `invalid_token` (`error_description="token audience mismatch"`).
- Token request `resource` must be subset of grant resources; else `invalid_target`.
- PAT (`dmi_live_`): no audience concept; accepted at `/api/v1` and `/mcp` (issued by this server for its own resources, workspace scoped).

## Authorization code flow

```
client -> GET /mcp (no token)                       -> 401 WWW-Authenticate: Bearer resource_metadata="{APP_URL}/.well-known/oauth-protected-resource/mcp", scope="conversations:read ..."
client -> GET PRM -> GET /.well-known/oauth-authorization-server
client -> (CIMD: no call) | (DCR: POST /oauth/register)
browser -> GET /oauth/authorize?response_type=code&client_id=..&redirect_uri=..&scope=..&state=..&code_challenge=..&code_challenge_method=S256&resource=..
   no session -> 302 /login?intended=/oauth/authorize?...   (login, 2FA)
   session    -> StartAuthorizationRequest -> 302 /oauth/consent/oar_...  (or auto-approve, see below)
SPA -> GET /api/v1/oauth/authorization-requests/oar_.. -> user picks workspace + scopes
SPA -> PATCH /api/v1/oauth/authorization-requests/oar_.. {status: "approved", workspace_id, scopes}   (or {status: "denied"})
SPA -> window.location = data.redirect_to  (client redirect_uri?code=..&state=..&iss=..)
client -> POST /oauth/token grant_type=authorization_code&code=..&redirect_uri=..&client_id=..&code_verifier=..&resource=..
```

### `/oauth/authorize` validation order

Errors before redirect URI is trusted render HTML error page (never redirect). After, redirect with `error`, `error_description`, `state`, `iss`.

1. `client_id` missing or unknown (DCR id not found, CIMD fetch fails, client disabled): error page `invalid_client`.
2. CIMD: client_id is HTTPS URL with path; resolve (cache or fetch). Failure reasons shown on page: `metadata_fetch_failed`, `metadata_invalid`, `client_id_mismatch`, `host_blocked`, `workspace_policy` is checked later at consent.
3. `redirect_uri` missing: allowed only when client has exactly 1 registered URI; else error page. Not matching registered (see redirect rules): error page `invalid_redirect_uri`.
4. `response_type` not `code`: redirect `unsupported_response_type`.
5. `code_challenge` missing or method not `S256`: redirect `invalid_request` ("PKCE S256 required").
6. `code_challenge` not 43..128 chars of `[A-Za-z0-9-._~]`: redirect `invalid_request`.
7. Scopes invalid: redirect `invalid_scope`. First-party client: only absent, `offline_access` or `first_party` allowed.
8. Resources invalid: redirect `invalid_target`. First-party client: `/mcp` resource invalid.
9. `state` over 1,024 chars: redirect `invalid_request`.
10. `max_age` not non-negative integer, or `prompt` not in `none|login|consent`, or `device_id` over 64 chars or outside `[A-Za-z0-9-_]`: redirect `invalid_request`. First-party client and `apps.enabled=false`: redirect `access_denied` + `error_description=first_party_app_disabled`.
11. Freshness: `prompt=login`, or `max_age` set and session `authenticated_at` older than `max_age` s: 302 `/login?intended=<authorize url without prompt=login>&reauth=1` (00-accounts forces full login on current session, incl. 2FA, rotates session); with `prompt=none`: redirect `login_required`.
12. `prompt=none`: if auto-approve conditions met, issue code; else redirect `consent_required` or `login_required`.
13. Create `oauth_authorization_requests` row (`auth_time` = `sessions.authenticated_at`, `auth_methods` = `sessions.auth_methods` snapshot, `sso_connection_id`, `auth_domain` from session (21 saml.md), `device_name` param, `device_id` param), 302 to SPA consent route, or auto-approve.

`max_age` and `prompt=login` accepted for every client (OIDC semantics, no id_token). Main user: first-party sudo re-auth (`max_age=0`).

### Redirect URI rules

- Registered list max 10. Match exact string (case-sensitive path, query must match exactly, fragment forbidden).
- Loopback exception (RFC 8252 7.3): registered `http://127.0.0.1/cb`, `http://[::1]/cb`, or `http://localhost/cb` with or without port matches presented URI with any port, same scheme/host/path/query.
- `http` allowed only for loopback hosts. `https` any public host.
- Private-use schemes (`cursor://`, `vscode://`, `com.example.app:/cb`) allowed for `application_type: native`. Blocked schemes: `javascript`, `data`, `file`, `vbscript`, `about`, `blob`, `http` non-loopback, `ws`, `wss`.
- No wildcards ever.

### Consent rules

- Auth requires full session (2FA completed if enabled/required).
- Request bound to first viewing user; other user gets 404.
- Workspaces offered: user's active memberships, not suspended, not deleted. Each flagged `client_allowed` per workspace `agent_access` policy (`allow_dynamic_clients`, `allow_metadata_document_clients`). System client always allowed.
- Badges: `official` = system client. `verified` = false for MVP (no verification program). `known_client` = CIMD host in `oauth.official_client_hosts` (display hint only, wording "Claims to be from claude.ai, document hosted at claude.ai").
- DCR clients display "Self-registered app. Dimmi cannot verify who made it." plus name in quotes.
- Loopback redirect warning: "This app runs on your computer. Only continue if you started this connection yourself."
- Auto-approve (skip consent screen, still needs session): all true: client name and redirect host unchanged since grant (CIMD refetch changing name forces consent once); client kind `system` or `metadata_document`; active non-revoked grant exists for same user + client + workspace (last used workspace) with scopes covering request; redirect URI non-loopback HTTPS; request not `prompt=consent`. Loopback and DCR always show consent. First-party clients: own rule (section "First-party apps"): skip only on claimed HTTPS redirect, never on private-use scheme.
- Approve creates new grant every time (no merge), so each device revocable separately. Exception: auto-approve of third-party client reuses existing grant (new code under same grant). First-party auto-approve still creates new grant (one grant per app login).
- Decision API: `PATCH /api/v1/oauth/authorization-requests/{oar}` body `{status: "approved", workspace_id, scopes}` or `{status: "denied"}`. Response 200 authorization request resource with `status` and `redirect_to` (`redirect_to` only in this response, never in later `GET`: code not stored in plaintext). Controller dispatches `ApproveAuthorizationRequest` or `DenyAuthorizationRequest` by `status`.
- Transitions: only `pending` to `approved|denied`. Any other (incl. repeat of same decision, double click): 422 `invalid_transition` with `from`, `to`; SPA disables button on first click and follows first response. Past `expires_at`: 410 `authorization_request_expired` (checked before transition).
- Approve: validate workspace membership again (race), scopes subset of grantable, policy allows client kind. Create grant + auth code in one tx (grant copies `auth_time`, `auth_methods`, `sso_connection_id`, `auth_domain`, `device_id` from request row); emit `oauth_grant.created` (only for new grant). First-party request: body must not carry `workspace_id` or `scopes` (422 `errors.workspace_id`/`errors.scopes` `prohibited`).
- Deny: status `denied`, `redirect_to` carries `error=access_denied`.

### Code exchange

- Client auth per `token_endpoint_auth_method`. Public: `client_id` in body, no secret. Confidential: Basic or post; wrong secret 401 `invalid_client`.
- Code lookup by hash; must be unexpired, unused, belong to client, `redirect_uri` identical to authorize value (when sent at authorize), PKCE verify `BASE64URL(SHA256(code_verifier)) == code_challenge`, verifier 43..128 chars.
- Code reuse (already `used_at`): `invalid_grant`, revoke grant (all tokens from that code), reason `code_reuse`, audit.
- Success: mark `used_at`, issue access + refresh (if client grant_types include `refresh_token`; all our known client kinds do by default), audiences = request `resource` (subset of grant) or grant resources.
- Response headers: `Cache-Control: no-store`, `Pragma: no-cache`.

### Client version gate (`/oauth/token`, all grant types)

- Foundation middleware `client.version` (extensibility.md section 12, P7) runs before grant handling. Request header `Dimmi-Client-Version: <platform>/<semver>` (`ios/1.4.0`, `android/1.4.0`, `cli/0.9.0`). Platform with `apps.min_versions.<platform>` set and version lower: 426, OAuth error shape (token endpoint never uses Laravel shape): `{"error":"invalid_request","error_description":"client_upgrade_required","min_version":"1.4.0","upgrade_url":"..."}` + `Cache-Control: no-store`. Extra members allowed by RFC 6749 5.2.
- Header absent (third-party clients, MCP clients) or unknown platform: allowed. Malformed header: ignored (allowed), metric `dimmi_client_version_malformed_total`. Config empty = noop.
- Nothing consumed on refusal: auth code stays unused (expires 60 s), refresh token unused, device code still pollable. App shows upgrade screen, keeps credentials; after upgrade refresh succeeds.
- Same header on `/api/v1` gated by same middleware with Laravel 426 `client_upgrade_required` (api.md). CLI sends header on every request; app on every request.

## Refresh

- `grant_type=refresh_token&refresh_token=..&client_id=..[&scope=..][&resource=..]`.
- Lookup by hash. Checks: belongs to client, not revoked, grant not revoked, member active, `expires_at` (idle) not passed.
- Rotation: mark `used_at`, create child refresh token (`parent_id`), new `expires_at = now + idle_ttl` (if absolute TTL set, min with grant created + absolute), new access token. Old access tokens stay valid until expiry (1 hour max; 15 min first-party).
- Reuse (token has `used_at`): always revoke grant + whole token family (`refresh_reuse`), audit, mail user "We revoked access for <client> because its credentials were used twice", 400 `invalid_grant`. No grace window, no re-rotation (PO 2026-11-02; framework `oauth` behavior). CLI concurrent refresh relies on its file lock; mobile app contract: single in-flight refresh; lost refresh response = sign in again.
- Scope downscoping: requested `scope` must be subset of grant scopes; result token narrower; grant unchanged.

## Device authorization grant (RFC 8628)

- Allowed only for clients with device grant type: `dimmi-cli` system client. DCR requests asking for device grant: rejected `invalid_client_metadata`. CIMD documents listing it: grant type ignored (not allowed).
- `POST /oauth/device/code` params: `client_id`, `scope`, `resource` (repeatable), `device_name` (non-standard, optional, max 100 chars, sanitized).
- Response fields: `device_code`, `user_code`, `verification_uri` (`{APP_URL}/device`), `verification_uri_complete` (`?user_code=XXXX-XXXX`), `expires_in` 900, `interval` 5.
- Rate limit: 10 device codes per minute per IP (`a.Limiter`, Valkey), 100 pending per IP (Postgres count). Valkey unreachable: per-minute limit fails closed (503 + `Retry-After: 5`, auth guard per data-model.md); pending cap still Postgres.
- Token polling: `grant_type=urn:ietf:params:oauth:grant-type:device_code&device_code=..&client_id=dimmi-cli`.
  - pending: 400 `authorization_pending`.
  - polled faster than interval: 400 `slow_down`, interval += 5 s (stored on row), max 60 s. Hot check without row write: Valkey `SET oauth:odr:{id}:poll 1 NX PX <interval_ms>`; key present = `slow_down`. Valkey down: fall back to `last_polled_at` compare (row written at most every 30 s).
  - denied: 400 `access_denied`, row status stays `denied`.
  - expired: 400 `expired_token`.
  - approved: issue tokens, status `consumed`. Further polls: 400 `invalid_grant`.
- User code lookup: `GET /api/v1/oauth/device-requests?filter[user_code]=BDWP-HQKX` (session). Filter required (missing: 400 `invalid_filter`). Normalized (uppercase, strip spaces and dashes, map ambiguous none since alphabet has no 0/O/1/I). Returns collection with 0 or 1 pending row (`meta` without cursors). No match: 200 `data: []` (counts as failed lookup). Per-user 10 lookups/minute, per-IP 100/minute (`a.Limiter`, Valkey): 429 `device_lookup_throttled`. Valkey unreachable: abuse guard fails closed (data-model.md policy): 503 + `Retry-After: 5` (`service_unavailable`). Brute force math: 20^8 = 2.56e10 codes, 100 pending max per IP, rate limits make guessing infeasible inside 15 minutes.
- `user_code` in query string: access log and trace redaction replaces `filter[user_code]` value with first 2 chars + `******` (same rule as app logs). Response `Cache-Control: no-store`, `Referrer-Policy: no-referrer`.
- Decision: `PATCH /api/v1/oauth/device-requests/{odr}` `{status: "approved", workspace_id, scopes}` or `{status: "denied"}`. Approve copies approving session `authenticated_at` to grant `auth_time`, plus `auth_methods`, `sso_connection_id`, `auth_domain`; grant `device_id` null at MVP (CLI device identity later, additive). Same transition rules as authorization requests (`pending` only; else 422 `invalid_transition`; expired 410). Requires same user that looked it up (lookup binds `user_id`); other user 404 `device_request_not_found`.
- Consent payload adds `device_name`, `request.ip`, `request.location` (country + city via optional GeoIP), `user_code` (user confirms matches terminal).
- `verification_uri_complete` prefills code but page still shows code and requires click "Approve" (never auto-approve device).

## Token revocation (RFC 7009)

- `POST /oauth/revoke` `token=..&token_type_hint=access_token|refresh_token&client_id=..` (+ secret for confidential).
- Refresh token: revoke whole grant (all access and refresh tokens). Rationale: CLI logout and app disconnect expect full revoke.
- Access token: revoke that token only.
- Unknown, already revoked, or other client's token: 200 (no oracle). Other client's token never revoked.

## Client registration

### Pre-registered system client

Seeded in migration:

```
client_id: dimmi-cli
kind: system
name: Dimmi CLI
token_endpoint_auth_method: none
grant_types: [authorization_code, refresh_token, urn:ietf:params:oauth:grant-type:device_code]
redirect_uris: [http://127.0.0.1/callback]   # loopback any port; used by `dimmi login --web` (later), harmless now
application_type: native
```

Same id on cloud and every self-host. Cannot be disabled by workspace policy; can be disabled globally by `dimmi oauth:client disable dimmi-cli` (self-host admin).

Second system client `dimmi-mobile` (`first_party = true`) seeded in same migration; spec in section "First-party apps". `first_party` column writable only by seed migration: DCR, CIMD and `oauth:client` command never set it.

### Client ID Metadata Documents (primary for MCP clients)

- Trigger: `client_id` parses as `https` URL with non-empty path, no fragment, no userinfo, length max 2,048.
- Fetch via `oauth.ClientMetadataDocument.Fetch` on `a.HTTP` (`framework/httpclient`, code.dny.dev/ssrf dialer guard): GET, `Accept: application/json`, 5 s timeout, 16 KB cap, no redirects (3xx = failure), TLS valid, private networks blocked unless self-host allows.
- Validate: JSON object; `client_id` equals URL exactly (byte compare); `client_name` present (1..100 chars); `redirect_uris` non-empty array of valid URIs per redirect rules; `token_endpoint_auth_method` absent or `none` (others rejected MVP); `grant_types` subset of `authorization_code`, `refresh_token` (others ignored); `logo_uri`, `client_uri` https only else dropped.
- Cache: upsert `oauth_clients` row kind `metadata_document`; expiry from `Cache-Control: max-age` clamped to [5 min, 24 h]; ETag revalidation. On fetch failure with cached copy younger than 7 days: use cached, log warning. Older: fail.
- Abuse limits: 30 fetches per minute per document host; 1,000 distinct CIMD clients per day per host before throttling host.
- Allow/block host lists from config; workspace policy can block CIMD clients per workspace.

### Dynamic Client Registration (RFC 7591, compatibility)

- `POST /oauth/register` JSON. Accepted fields: `client_name` (required here), `redirect_uris` (required, 1..10), `grant_types` (default `["authorization_code","refresh_token"]`; device grant rejected), `response_types` (only `["code"]`), `token_endpoint_auth_method` (`none` default, `client_secret_basic`, `client_secret_post`), `application_type` (`native` default per MCP guidance; `web` requires all https non-loopback redirects), `client_uri`, `logo_uri`, `software_id`, `software_version`, `scope` (stored as hint only). Unknown fields ignored, stored in metadata (max 16 KB total).
- Response 201: RFC 7591 fields + `client_id`, `client_id_issued_at`, `client_secret` (once, only confidential), `client_secret_expires_at: 0`. No `registration_access_token` (RFC 7592 out of scope).
- Errors 400: `invalid_redirect_uri`, `invalid_client_metadata`.
- Rate limit 20 per hour per IP (config). Same `client_name` + same redirect set from same IP within 24 h returns existing client (idempotent re-register, clients often re-register each launch). Confidential re-register always new client (secret cannot be re-shown).
- Disabled by config: endpoint returns 404 and metadata omits `registration_endpoint`.
- Reserved names: `client_name` containing "dimmi" (case-insensitive, after homoglyph folding) rejected `invalid_client_metadata` for DCR; for CIMD allowed only when document host is `trydimmi.com` or subdomain. Bidi control characters stripped from names; mixed-script names flagged on consent.

## First-party apps

Dimmi mobile app (later desktop) uses same `/api/v1` as dashboard, via OAuth instead of cookie. Binding summary: `00-foundation/security.md` "First-party apps", `00-foundation/api.md` "Who calls what". Detail here.

### System client `dimmi-mobile`

```
client_id: dimmi-mobile
kind: system
first_party: true
name: Dimmi
token_endpoint_auth_method: none            # public client, PKCE S256 only
grant_types: [authorization_code, refresh_token]
redirect_uris: ["{app_url}/oauth/callback/app", "com.trydimmi.app:/oauth/callback"]
application_type: native
```

- Same id on cloud and every self-host. `{app_url}` placeholder resolved to current `APP_URL` at match time (APP_URL change needs no migration).
- No device grant. Workspace `agent_access` policy never applies (user-level client). Global switch `apps.enabled` (not `oauth:client disable`, which refuses `first_party` rows with message pointing to config).
- Later desktop app: own system client (`dimmi-desktop`, loopback redirect, consent always); same grant shape. Not MVP.

### Redirects and consent

| Redirect | App uses it when | Consent |
|---|---|---|
| `{APP_URL}/oauth/callback/app` (claimed HTTPS: iOS Universal Link / associated domain, Android App Link) | host is in app's compiled associated domains (cloud hosts) | skipped (login still required) |
| `com.trydimmi.app:/oauth/callback` (private-use scheme, RFC 8252 7.1) | any other host (self-host custom domain) or claimed link unavailable | always shown, never auto-approved |

- Claimed redirect accepted at `/oauth/authorize` only when `apps.ios_app_ids` or `apps.android_package_fingerprints` non-empty; else error page `invalid_redirect_uri` ("use the app's fallback login").
- Why: anyone can send `client_id=dimmi-mobile` and register the private-use scheme. PKCE stops theft of codes from flows the real app started; it cannot stop a malicious app starting its own flow. Claimed HTTPS link reaches only apps the OS verified against our association files, so consent skip is safe there only.
- Scheme consent payload (`GET /api/v1/oauth/authorization-requests/{oar}`): `client.first_party: true`, `device_name`, `device_platform` (from browser UA, display only), `request.ip`, `request.location`, `request.created_at`, `warning: "first_party_scheme"` (SPA text "Only continue if you are signing in to the Dimmi app on your phone"), `grant_reach: "account"` (SPA text "Full access to your account in all your workspaces, limited by your role in each"). `workspaces: []`, `requested_scopes: []` (no picker, no scope list).
- iOS: `ASWebAuthenticationSession` with `.https(host:path:)` callback (iOS 17.4+, needs `webcredentials` entry) else universal link; `prefersEphemeralWebBrowserSession = false` so web login cookie reused. Android: Custom Tabs + App Link intent filter `autoVerify="true"`. Never embedded webview (Google sign-in blocks it; password would pass through app).

### Association files (`/.well-known`, 09 owns)

| Path | Body | Config |
|---|---|---|
| `GET /.well-known/apple-app-site-association` | `{"applinks":{"details":[{"appIDs":[...],"components":[{"/":"/oauth/callback/app"}]}]},"webcredentials":{"apps":[...]}}` | `apps.ios_app_ids` (`<TEAM_ID>.<bundle id>`) |
| `GET /.well-known/assetlinks.json` | `[{"relation":["delegate_permission/common.handle_all_urls"],"target":{"namespace":"android_app","package_name":"...","sha256_cert_fingerprints":[...]}}]` | `apps.android_package_fingerprints` (`<package>=<SHA-256 cert fingerprint>`, grouped by package) |

- `Content-Type: application/json`, 200 direct (no redirect, Apple CDN rejects redirects), `Cache-Control: public, max-age=3600`, no auth, no cookies. Empty list or `apps.enabled=false`: 404.
- Defaults: official ids (`<DIMMI_TEAM_ID>.com.trydimmi.app`, `com.trydimmi.app=<release key fingerprint>`) on cloud and self-host.
- Self-host on custom domain: official app cannot claim it (associated domains and App Link hosts compiled into app). Files harmless; app uses scheme + consent. White-label builds set own ids (and own entitlements).
- Only `/oauth/callback/app` claimed. Other paths open in browser as usual.

### `/oauth/callback/app` page (web route)

Reached only when claimed link not handled by OS (app not installed, link opened in other browser, Android verification failed).
- Static templ page, `Cache-Control: no-store`, `Referrer-Policy: no-referrer`, CSP `default-src 'none'; style-src 'nonce-...'`, no scripts, no third-party assets. Query (`code`, `state`, `iss`) never logged (access log redaction) and never rendered.
- Text "Open the Dimmi app to finish signing in". Buttons: "Open the Dimmi app" (same claimed URL; user tap triggers universal link on iOS where redirect did not), "Get the app" (store links), "Sign in another way" (`{APP_URL}/login`).
- Never forwards code to private-use scheme (any app may own it). Code expires in 60 s anyway.

### Authorize extras

- `device_name` (non-standard, optional, max 100 chars, NFC, control chars stripped; default "<platform> device"). Stored on request, copied to grant.
- `device_id` (non-standard, optional, max 64 chars `[A-Za-z0-9-_]`): app install id, random UUID generated on first launch, kept in keychain (iOS) / keystore-backed storage (Android), same value sent as 06 `push_subscriptions.device_id`. Stored on request, copied to grant. Never a hardware id or advertising id. Any client may send it; only first-party grants use it at MVP (third-party value stored, not trusted). Later "revoke this device" (grant + session + push rows) and "remember device" read it.
- `scope`: absent, `offline_access` or `first_party` only; else `invalid_scope`. Stored grant scopes `{first_party}`.
- `resource`: absent, `{APP_URL}` or `{APP_URL}/api/v1`; audience always `[{APP_URL}/api/v1]`. `{APP_URL}/mcp`: `invalid_target`.
- `max_age` / `prompt=login` (validation steps 10, 11): sudo re-auth. Fresh login sets session `authenticated_at = now`, copied to grant `auth_time`.
- Platform and app version: from token endpoint `User-Agent` (app sends `Dimmi/<app version> (<ios|android> <os version>; <model>)`), parsed at code exchange and every refresh, written to grant `device_platform`, `app_version`, `user_agent`. Unparseable: `device_platform = 'unknown'`. Browser UA at authorize: consent display only.

### Access policy refusals

21 access policy 403 carries `{code, message, requirement, action_url}`. First-party app handles every `requirement` same way: open browser sheet on `/oauth/authorize` with `prompt=login` (session `sso_required` and method rules run in sheet), new grant replaces old. Never loops: refused again after sheet, app shows `message`.

### Grant and tokens

- Grant: `first_party = true`, `workspace_id` null, `workspace_member_id` null, `scopes = {first_party}`, `resources = {APP_URL/api/v1}`, `auth_time`, `auth_methods`, `sso_connection_id`, `auth_domain`, `device_id`, device fields, `confirmed_at` / `confirmed_method` (null until in-app confirmation). CHECK in README data model. Auth code and token rows `workspace_id` null.
- New grant per authorization (incl. re-auth, incl. auto-approve). After successful re-auth app revokes previous refresh token (`POST /oauth/revoke`, kills old grant). Missed revoke: old grant idles out (90 days) or user revokes it in session list.
- Token response: `access_token`, `token_type`, `expires_in: 900`, `refresh_token`, `scope: "first_party"`, `grant_id`, `auth_time` (unix seconds). No `workspace` field.
- Access TTL `oauth.first_party.access_token_ttl` (15 min). Refresh: same 90-day sliding idle window, rotation, reuse detection; reuse always revokes the family (section Refresh). Refresh never changes `auth_time`.
- `apps.enabled=false`: authorize redirects `access_denied` (`error_description=first_party_app_disabled`); token endpoint (code exchange, refresh) 400 `{"error":"unauthorized_client","error_description":"first_party_app_disabled"}`; API 403 `first_party_app_disabled`. Grants not revoked (switch reversible).
- Listed in 00-accounts `GET /sessions` as `kind: app` through 09 query `ListUserAppGrants(userID, currentGrantID)`: `id ogr_`, `client_id`, `client_name`, `device_name`, `device_platform`, `app_version`, `auth_methods`, `auth_time`, `confirmed_at`, `last_used_ip`, `last_used_at`, `created_at`, `current` (caller's own grant). Not in `GET /api/v1/oauth/grants` (connected apps = workspace grants only).

### Token resolver (`auth:app` vs `auth:token`)

```
dmi_oat_ -> hash -> oauth_find_access_token(hash)
         -> {grant_id, user_id, workspace_id, first_party, client_first_party, auth_time, confirmed_at, audiences, expires_at, revoked}
grant.first_party AND grant.workspace_id IS NULL AND client.first_party:
    apps.enabled = false        -> 403 first_party_app_disabled (tokens kept; re-enable restores)
    path /mcp                   -> 403 first_party_token_not_allowed
    audience lacks /api/v1      -> 401 invalid_token
    -> guard auth:app: principal {User, AuthTime, ConfirmedAt, AuthMethods, DeviceID, Grant}; Actor Via{Kind: app, ID: ogr_}
       workspace routes: Dimmi-Workspace header via 00-accounts workspace middleware; active membership rechecked each request (left: 403 membership_deactivated)
       permissions: member role + policies (no abilities); rate limit: per-user bucket shared with session
else -> guard auth:token (rules above, unchanged)
```

- Kind decided by row lookup, never by client claim. Grant `first_party` flag copied from client at grant creation; CHECK makes mixed rows impossible.
- `password.confirm` (00-accounts): satisfied when `now - max(principal.AuthTime, principal.ConfirmedAt) <= auth.password_confirmation.ttl`.
- App sudo without browser sheet: app calls `POST /api/v1/user/confirmation {method: password|email_code|totp, ...}` (00-accounts `ConfirmUser`), which calls 09 `ConfirmGrant(grantID, method)` in same tx: grant `confirmed_at = now()`, `confirmed_method = method`, cache bust. Per grant (other installs unaffected). Refresh never changes it. `GET /user/confirmation` from app returns `confirmed_until` from grant.
- Principal cached with token (30 s); grant revocation and confirmation bust cache via Valkey pubsub (`a.PubSub`, topic `oauth.bust`) like any grant.

### Revocation (called by 00-accounts, sync, same tx)

| 00-accounts trigger | 09 action | Reason |
|---|---|---|
| `DELETE /sessions/{ogr}` | `RevokeAppGrant(userID, grantID, "user")` (404 when not user's own first-party grant) | `user` |
| `DELETE /sessions` (log out other sessions) | `RevokeUserAppGrants(userID, "logout_others", except: current grant if caller is app)` | `logout_others` |
| `DELETE /sessions/current` from app | `RevokeAppGrant(userID, currentGrant, "user")` | `user` |
| password change | `RevokeUserAppGrants(userID, "password_changed", except: caller's grant when change made from app)` | `password_changed` |
| password reset (`PATCH /password-resets/{prs}`) | `RevokeUserAppGrants(userID, "password_reset")` (no exception) | `password_reset` |
| 2FA disable | `RevokeUserAppGrants(userID, "two_factor_disabled", except: caller's grant)` | `two_factor_disabled` |
| account delete | `RevokeUserAppGrants(userID, "user_deleted")` in tx; `user.deleted` listener also revokes workspace grants (existing) | `user_deleted` |
| CLI `user:logout`, `user:reset-password --password` | `RevokeUserAppGrants(userID, "admin" / "password_reset")` | `admin`, `password_reset` |
| pre-hijack defense (unverified local account auto-linked by verified OAuth email or magic link; 00-accounts flows 5) | `RevokeUserAppGrants(userID, "pre_hijack_defense")` (no exception) | `pre_hijack_defense` |

- Actions revoke grant + all tokens in caller's tx; cache bust + `oauth_grant.revoked` (`{grant_id, user_id, first_party, device_id, reason}`) after commit; AfterCommit listener disconnects that grant's realtime connections (centrifuge, Redis broker on Valkey). Idempotent (already revoked: no-op).
- Workspace removal never revokes first-party grant (user may have other workspaces); membership checked per request instead.
- Third-party workspace grants untouched by these triggers (existing lifecycle rules apply).

## Protected resource behavior (`/mcp` and `/api/v1`)

| Situation | Response |
|---|---|
| No `Authorization` header | 401, `WWW-Authenticate: Bearer resource_metadata="<PRM url>", scope="<default scopes>"` |
| Token in query string | 400 `invalid_request` (never accepted) |
| Unknown, expired, revoked token | 401, `WWW-Authenticate: Bearer error="invalid_token", error_description="...", resource_metadata="..."` |
| Audience mismatch | 401 `invalid_token` |
| Member deactivated / removed / workspace suspended | 401 `invalid_token` (`error_description="access revoked"`) |
| OAuth token lacks scope for operation | 403, `WWW-Authenticate: Bearer error="insufficient_scope", scope="<all scopes needed for this call>", resource_metadata="..."`; body (REST) `{"message":"...","code":"missing_ability","required_ability":"..."}`; MCP: HTTP 403 with JSON-RPC error body |
| PAT lacks ability | REST 403 `missing_ability`; MCP tool result `isError: true` (`code: missing_ability`, hint to create new token) since PAT cannot step up |
| Role ceiling blocks scope (token has it, role lost it) | same as missing scope but `WWW-Authenticate` omits `scope` hint (step-up cannot help); message "your role no longer allows ..." |
| First-party token (`auth:app`) at `/mcp` | 403 `first_party_token_not_allowed` (JSON body; MCP always uses workspace-scoped grants with scope list, so confirm gate and per-grant limits stay meaningful) |
| First-party token, `apps.enabled=false` | 403 `first_party_app_disabled` |
| First-party token, `Dimmi-Workspace` of workspace user left or was removed from | 403 `membership_deactivated` (00-accounts code); grant stays valid for other workspaces |
| First-party token, no `Dimmi-Workspace` on workspace route, user in 2+ workspaces | 400 `workspace_required` (00-accounts) |
| PAT or third-party OAuth token on `auth:user` route | 403 `first_party_only` (api.md) |

## Lifetimes summary

| Item | TTL |
|---|---|
| Authorization request | 10 min |
| Authorization code | 60 s, single use |
| Device code | 15 min |
| Access token | 1 h (first-party: 15 min) |
| Refresh token | 90 days idle, sliding, no absolute (first-party same) |
| Refresh reuse grace | none: any reuse revokes the family (PO 2026-11-02) |
| `auth_time` freshness for sudo | `auth.password_confirmation.ttl` (00-accounts) |
| Grant | until revoked |
| CIMD cache | 5 min..24 h by Cache-Control, stale-if-error 7 days |
| PAT (00-accounts) | user-chosen, including never |

## RFC clause test table (feature tests)

| Clause | Test |
|---|---|
| RFC 8414 3 | metadata fields present, issuer equals APP_URL exactly |
| RFC 9728 3/5.1 | PRM per resource path, 401 header format |
| OAuth 2.1 4.1.1 / 7.5.2 | missing PKCE rejected, `plain` rejected, wrong verifier `invalid_grant` |
| OAuth 2.1 4.1.3 | code reuse revokes grant |
| OAuth 2.1 4.3.1 | refresh rotation, reuse detection (family revoke, no grace) |
| OAuth 2.1 7.12 | open redirect: unregistered redirect shows error page, no redirect |
| RFC 8252 7.3 | loopback any port |
| RFC 8252 7.1 / 7.2 | `dimmi-mobile` private-use scheme always consent; claimed HTTPS skips consent only when association config non-empty |
| RFC 8252 8.12 | no embedded user agent: consent + login pages send `X-Frame-Options: DENY`, CSP `frame-ancestors 'none'` |
| OIDC Core 3.1.2.1 (`max_age`, `prompt=login`) | stale session forces login; `prompt=none` + stale: `login_required`; grant `auth_time` = fresh login time |
| RFC 8628 3.2/3.5 | device response fields, pending, slow_down increments, expired, denied |
| RFC 8707 2 | invalid_target, audience enforcement on both resources |
| RFC 9207 2 | `iss` on success and error redirects |
| RFC 7009 2 | 200 for unknown token, refresh revokes grant |
| RFC 7591 3 | registration response, invalid metadata, device grant refused |
| CIMD draft 00 | client_id mismatch rejected, redirects validated, SSRF private IP blocked, size cap, no redirect follow |
| RFC 6750 3.1 | insufficient_scope 403 header |
| RFC 6749 5.2 (extension members) | `/oauth/token` with `Dimmi-Client-Version: ios/1.0.0` below min: 426 `invalid_request` + `error_description=client_upgrade_required`, `min_version`; code still exchangeable after retry with newer version header |
