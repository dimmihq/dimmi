# 19 Enterprise identity: access policy rules

Style: caveman. Rules set per workspace, evaluated by `access.policy` on every `W` route, no query. Extends `app/auth/access_policy.go`. Stored in `workspaces.settings.security`, read and written through typed methods in `app/models/workspace_security.go` (same pattern as `RequiresTwoFactor`; moved to `app/support/settings` registry when foundation registry lands, keys already `Enforceable`).

## Settings keys

| Key | Type | Default | Validation |
|---|---|---|---|
| `security.allowed_sign_in_methods` | string[] | `[]` = all | values in `password`, `magic_link`, `sso`, plus configured provider keys. Passkey is second factor here, never listed |
| `security.google_allowed_domains` | string[] | `[]` = any | max 20, lowercase domains |
| `security.session_max_age_minutes` | int or null | null | 60..43200; capped by `auth.sessions.absolute_lifetime` |
| `security.session_idle_timeout_minutes` | int or null | null | 15..20160; capped by `auth.sessions.idle_lifetime` |
| `security.dashboard_ip_allowlist` | `[{cidr, label}]` | `[]` = any | max 100, IPv4 + IPv6, parsed with `net/netip` |
| `security.api_ip_allowlist` | `[{cidr, label}]` | `[]` = any | same |
| `security.sso_connection_ids` | uuid[] | `[]` | internal mirror of org's active connections, written by connection actions only, not in API |
| `security.sso_enforced_connection_ids` | uuid[] | `[]` | internal mirror of enforced subset, same writer, not in API |

## Principal changes

`Principal` gains `IP netip.Addr` (from `fhttp.ClientIP`, set by guards), `LastActivityAt`, `SSOConnectionID`, `AuthDomain`. `AccessRule.Check` takes decoded settings: `Check(p *Principal, s models.SecuritySettings, now time.Time) error`. `AccessRules.Evaluate` decodes settings once per request. `RequireTwoFactor` updated (below). Rule needs clock: `AccessRules` struct holds `framework/clock`.

## Rule order and refusals

Every refusal: 403 `{code, message, requirement, action_url}` plus fields below. Order: first failing wins.

| # | Rule | Applies to | code | requirement | action_url | Extra |
|---|---|---|---|---|---|---|
| 1 | Dashboard allowlist | session, app | `ip_not_allowed` | `ip_not_allowed` | none (null) | `ip` |
| 1 | API allowlist | token | `ip_not_allowed` | `ip_not_allowed` | null | `ip` |
| 2 | SSO enforced | session, app | `sso_required` | `sso_required` | `/auth/sso/<ssoc>/redirect?intent=link` | `redirect_url` |
| 3 | Sign-in method | session, app | `sign_in_method_required` | `sign_in_method` | `/login?reauth=1` | `allowed_methods` |
| 4 | Google domain | session, app using `google` | `sign_in_method_required` | `sign_in_method` | `/login?reauth=1` | `allowed_domains` |
| 5 | Session length | session, app | `reauthentication_required` | `reauthentication_required` | `/login?reauth=1` | `reason: max_age\|idle` |
| 6 | Require 2FA | all | `two_factor_required` | `two_factor` | security page | |

Status 403 always, token refusals too. App (`KindApp`) treats `action_url` as: open browser sheet, `prompt=login` (09).

Realtime: `app/broadcasting/authorizers.go` calls same policy on `workspace:` and `conversation:` subscribe, and on connection refresh (5 min `ExpireAt`, builds on F12 fix: sockets end with session). Refused: subscription denied, client shows access required.

## Sign-in methods (S-07)

- Method of session = `SignIn.Method` mapped: `email_code` to `magic_link`; `password_reset` to `password`; `invitation`, `setup` count only when `password` or `magic_link` allowed; provider key itself; `sso`.
- Second factors (totp, passkey, recovery code) never count: password + passkey is `password`.
- Exempt: `auth_methods` contains `sso_bypass` and caller is owner.
- Tokens exempt (README D11).
- Enforced at workspace boundary, not at login: user signs in globally, may belong to other workspaces without this rule. Global block is SSO domain enforcement only (`LoginPolicy`).

`PATCH` guards:
- List must include caller's own current method: else 422 `sign_in_method_excludes_caller`.
- At least one active owner must hold an allowed method (password set, linked provider, verified email for `magic_link`, or SSO identity): else 422 `no_owner_can_sign_in`.
- Value must be available on install: `google` needs configured provider; `sso` needs active connection. Else 422 `in`.

## Google domains (S-07)

Session's `auth_domain` comes from Google `hd` claim at callback (`oauth_identities.hosted_domain` already recorded). List non-empty: session using `google` must have `auth_domain` in list. Personal Gmail has no `hd`: refused. Caller's own domain must be in list when caller signed in with Google: else 422 `sign_in_method_excludes_caller`.

## Session length (S-06)

- `session_max_age_minutes`: `now - AuthenticatedAt` over value: refused.
- `session_idle_timeout_minutes`: `now - LastActivityAt` over value: refused. `last_activity_at` written at most every 5 min, so true idle is value + up to 5 min. Stated in UI. Minimum 15 chosen for this reason.
- Evaluated per workspace, so strictest workspace effectively wins for that workspace alone; other workspaces of same user unaffected. Better than global shortest: unrelated workspaces do not punish each other.
- Reauth uses existing `reauth` mode (00-accounts flows 24): same user, session row updated, `authenticated_at` reset. Unsaved composer draft survives: 03 drafts persist; access required screen keeps route.
- App: `AuthTime` for max age; grant `last_used_at` for idle. Coalesced 60 s, same note.
- Install config (`config/sessions.go`) still ends session absolute; rule only shortens. Value above cap: 422 `max`.
- Tokens exempt (README D11).
- Changing value applies next request, caller included. No lockout risk: caller just signs in again. Response has no special case.

## Dashboard allowlist (S-10)

- Session and first-party app requests on `W` routes. Non-`W` account routes (`/user/*`, `/sessions`, `/me`) stay reachable so person can see their account and leave workspace.
- Empty = off. Client IP from `fhttp.ClientIP` (framework trust config). `dimmi doctor` warns when allowlist set and no trusted proxy configured behind a proxy.
- `PATCH` must keep caller inside: else 422 `ip_allowlist_excludes_caller` with `ip`. IPv4-mapped IPv6 normalized by `netip.Addr.Unmap`.
- Refusal screen shows "Your address: x. Ask an owner to add it."
- Widget, webhooks in, SCIM, help center, portal: never subject.
- Lockout recovery: `dimmi workspace:clear-access-policy <slug> --only ip_allowlist` (self-host operator; cloud staff on owner request after verifying). Prints audit line.

## API allowlist (P-02)

- PAT + third-party OAuth, on `W` routes, MCP over HTTP, CLI (uses PAT). 403 `ip_not_allowed` with message naming address and "allowlist", so CI failure explains itself.
- Separate list from dashboard: office VPN vs CI runners. Empty = off.
- `/oauth/token` not subject (no workspace known); resource calls are.
- Changes write audit `workspace.access_policy_changed` with redacted-free CIDR diff (not secret).
- No caller check on `PATCH`: caller is a browser, list does not bind them. `PATCH` response includes `tokens_outside: n`, live tokens whose `last_used_ip` falls outside new list; UI shows it as warning after save.
- SCIM tokens not subject: IdP egress ranges are large and change; `dmi_scim_` is high-entropy, org-level, revocable. Reason stated in UI help.

## Require 2FA rule update

`RequireTwoFactor.Check` passes when: user has 2FA; or session method is `sso` and `SSOConnectionID` is in `security.sso_connection_ids` of this workspace (README D5).

## API shape

`GET /api/v1/workspace/access-policy` (A W `settings:read`), `PATCH` (U W C `security_policy:manage`). `require_two_factor` stays on `PATCH /workspace`; mirrored read-only here.

```json
{ "data": {
  "object": "access_policy",
  "allowed_sign_in_methods": ["sso", "google"],
  "google_allowed_domains": [],
  "session_max_age_minutes": 720,
  "session_idle_timeout_minutes": 60,
  "dashboard_ip_allowlist": [{ "cidr": "203.0.113.0/24", "label": "Office" }],
  "api_ip_allowlist": [],
  "require_two_factor": true,
  "sso_enforced": false,
  "available_sign_in_methods": ["password", "magic_link", "google", "github", "sso"],
  "your_ip": "203.0.113.4",
  "tokens_outside": 0
} }
```

Partial `PATCH`; whole list replaced per key. Keys locked by an org: 422 `setting_locked` (reserved code; no org lock UI exists).

## Events, audit, mail

- Change: event `workspace.access_policy_changed`, audit with `changes`, `AccessPolicyChangedMail` to owners (not to admin who made it). Listener disconnects dashboard sockets of workspace members; reconnect re-checks, so tightening bites in under 5 s.
- Cache: workspace loaded per request; rule reads fresh on next request. No separate bust.

## UI

`/d/settings/workspace-security`, gated on `security_policy:manage`, read-only with `settings:read`.

| Section | Controls |
|---|---|
| Sign-in methods | checkbox per `available_sign_in_methods`; all checked = no rule. Google row expands to domain list `Input` chips |
| Session length | two BoardUI `Select`: "Sign in again after" (no limit, 1 h, 8 h, 12 h, 24 h, 7 d, 30 d) and "Sign out when idle for" (no limit, 15 min, 1 h, 8 h, 24 h, 7 d). API takes any minute value; UI presets only |
| Dashboard IP allowlist | `Item` list, add dialog, your address shown |
| API IP allowlist | same; `tokens_outside` warning |
| Authentication | existing `RequireTwoFactorSection` |

Confirmation via existing `recent-auth.tsx`. 422s render inline in `Failure`.

## Tests (feature, plain names)

`tests/feature/identity/access_policy_test.go`: "a session older than the workspace limit must sign in again", "a session within the limit keeps working", "another workspace of the same person is not affected", "an idle session must sign in again", "a token ignores session length", "only allowed sign-in methods reach the workspace", "a google session from another domain is refused", "a personal gmail session is refused when domains are set", "a caller cannot remove their own sign-in method", "a list with no owner able to sign in is refused", "an address outside the dashboard list is refused", "an address inside the list works", "ipv6 ranges match", "a caller cannot exclude their own address", "a token from outside the api list is refused with its address", "mcp over http obeys the api list", "the dashboard list does not bind tokens", "a refused member can still reach their own account", "realtime subscribe is refused outside the list", "changing the policy closes workspace sockets", "changes land in the audit log", "the operator command clears the allowlist". Real Postgres, Valkey; `tc.Clock`.
