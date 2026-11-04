# 00 Accounts: API, CLI, MCP, mail

Style: caveman. Conventions from `00-foundation/api.md`; REST rules and route map from `00-foundation/rest.md` (binding). All JSON. Route name = OpenAPI `operationId`.

Auth letters (foundation `api.md` "Who calls what"): `S` = `auth:session` (browser cookie; `http.CrossOriginProtection` on unsafe), `U` = `auth:user` (session or first-party app token), `A` = `auth` (any: session, first-party app, PAT, third-party OAuth; abilities checked for tokens), `G` = guest (no credential needed; existing session allowed but ignored unless noted). Extra middleware: `W` = `workspace` + `access.policy` (`AccessPolicy`; was `two_factor.enforced`) + suspension check, `C` = `password.confirm` (recent auth), `V` = `verified`. PAT or third-party OAuth token on `U` route: 403 `first_party_only`. Ability column applies to token requests; session and app use role permissions (`roles.go`). Every `/api/v1` route also passes foundation `client.version` (426 `client_upgrade_required` when `Dimmi-Client-Version` below `apps.min_versions.<platform>`; header absent = allowed) and `Dimmi-Version` (echoed; unknown 400 `invalid_api_version`).

## Endpoints

### Auth, sessions, secret-token redemption (no workspace)
| Method | Path | Name | Auth | Purpose |
|---|---|---|---|---|
| GET | `/api/v1/auth/config` | auth.config.show | G | login options + capability discovery: providers and factors (arrays of objects), signup mode, magic link, captcha site key, setup required, mail configured, edition, `server_version`, `api_version`, `region`, `capabilities`, `min_client_versions` |
| POST | `/api/v1/users` | users.store | G | register: user + session (+ workspace or membership via `invitation_token`) |
| POST | `/api/v1/sessions` | sessions.store | G (`two_factor` grant: challenge cookie) | log in. Body `grant`: `password` `{email, password}`, `two_factor` `{method, code}` (`method` required: `totp`, `recovery_code` with `code`; `passkey` with `credential`, the assertion JSON), `magic_link` `{token}`, `email_code` `{email, code}`. Optional `reauth: true` (any primary grant): result must be session cookie's user (flows.md 24). 201 `MeResource` + session cookie, or 200 `two_factor_challenge`. SSO starts on web route `/auth/sso/{ssoc}/redirect` (21), not a grant here. Optional `sso_bypass: true` (21) |
| GET | `/api/v1/sessions` | sessions.index | U | own browser sessions and first-party app grants, `kind`, `current` flag |
| DELETE | `/api/v1/sessions/{ses}` | sessions.destroy | U | revoke one. `{ses}` = `ses_...` (browser), `ogr_...` (first-party app grant) or `current` (logout) |
| DELETE | `/api/v1/sessions` | sessions.destroy-many | U C | revoke every session and app grant except current |
| PATCH | `/api/v1/two-factor-resets/{ott}` | two-factor-resets.update | G | `{token, password?}`; finish admin-started reset ([settings-pages.md](settings-pages.md)) |
| POST | `/api/v1/magic-links` | magic-links.store | G | request magic link + code, always 202 |
| POST | `/api/v1/password-resets` | password-resets.store | G | request reset, always 202 |
| PATCH | `/api/v1/password-resets/{prs}` | password-resets.update | G | `{token, password}`; sets password, then session or `two_factor_challenge` |
| POST | `/api/v1/email-verifications` | email-verifications.store | U | resend verification, 202 |
| PATCH | `/api/v1/email-verifications/{ott}` | email-verifications.update | G | `{token}`; verify email (cross-device, no session needed) |
| POST | `/api/v1/email-changes` | email-changes.store | U C | `{email}`; creates `ott_` purpose `email_change` |
| PATCH | `/api/v1/email-changes/{ott}` | email-changes.update | G (session of same user allowed) | `{token}`; confirm new email |
| GET | `/api/v1/user/confirmation` | user.confirmation.show | U | sudo state `{confirmed_until, method}`; 404 when none or expired |
| POST | `/api/v1/user/confirmation` | user.confirmation.store | U | `{method, password \| code}`; `method` in `password`, `email_code`, or any active factor type whose driver has `Confirmation` (MVP `totp`; later `{method: "webauthn", credential}`) |
| POST | `/api/v1/user/confirmation-codes` | user.confirmation-codes.store | U | email 6-digit code (passwordless users), 202 |
| GET | `/api/v1/setup` | setup.show | G | first-run status (self-host); controller owned by 11 |
| POST | `/api/v1/setup` | setup.store | setup token | create first owner + workspace; controller owned by 11, calls `users.CreateFirstUser` |
| GET | `/auth/{provider}/redirect` | oauth.redirect | G (login, `intent=reauth` with session) / S C (`intent=link`) | start OAuth for any registered `IdentityProvider` key (MVP `github`, `google`; web route, browser only). Unknown or unconfigured key: 404 |
| GET | `/auth/{provider}/callback` | oauth.callback | OAuth state cookie | finish OAuth (web route, browser only) |

CSRF note: no CSRF endpoint or token cookie. Framework `CSRF` middleware (`net/http.CrossOriginProtection`) guards every unsafe request in guest and cookie-capable groups, login and register included (login CSRF): `Sec-Fetch-Site` cross-site/same-site from untrusted origin, or `Origin` host not matching `Host` / trusted origins, gets 403 `cross_origin_request`. Requests without either header (CLI, apps, server-to-server) pass.

First-party app (Dimmi mobile) never calls `POST /sessions`, `POST /users`, `/magic-links` or `/password-resets`: it logs in through system browser sheet + OAuth code flow (09 `oauth.md` "First-party apps"). App calls every `U` and `A` route with its `dmi_oat_` token.

### Current user (no workspace)
| Method | Path | Name | Auth | Purpose |
|---|---|---|---|---|
| GET | `/api/v1/me` | me.show | A (any ability) | actor, workspace (if header/token), abilities, instance info |
| GET | `/api/v1/user` | user.show | A | own profile |
| PATCH | `/api/v1/user` | user.update | U | name, locale, timezone, `preferences` (merge). Managed user: name, email 403 `managed_by_organization` |
| DELETE | `/api/v1/user` | user.destroy | U C | delete account |
| GET | `/api/v1/user/deletion-preview` | user.deletion-preview.show | U | blocking/affected workspaces |
| PATCH | `/api/v1/user/password` | user.password.update | U | change password |
| PUT | `/api/v1/user/avatar` | user.avatar.update | U | multipart upload (replaces whole image), 2 MB, png/jpeg/webp/gif, re-encoded 256x256 |
| DELETE | `/api/v1/user/avatar` | user.avatar.destroy | U | remove |
| GET | `/avatars/{file}` | avatars.show | public | stored picture, name is random uuid; not under `/api/v1` |
| GET | `/api/v1/user/auth-factors` | user.auth-factors.index | U | list own factors (no secrets): `id, object: auth_factor, type, name, status, preferred, confirmed_at, last_used_at, created_at`; recovery set adds `remaining`, `total`. Pending listed with `status: pending` |
| POST | `/api/v1/user/auth-factors` | user.auth-factors.store | U C | `{type: "totp", name?}`: 201 `auth_factor` status `pending` + one-time `setup` (`secret`, `otpauth_url`, `qr_svg`). `{type: "recovery_code"}`: regenerate set (2FA on only), 201 active `auth_factor` + one-time `codes` |
| PATCH | `/api/v1/user/auth-factors/{fac}` | user.auth-factors.update | U | `{code}`: pending to active (response adds one-time `recovery_codes` when this activation enabled 2FA). `{name}`, `{preferred: true}` (any status). `code` combined with other fields: 422 `prohibited` |
| DELETE | `/api/v1/user/auth-factors/{fac}` | user.auth-factors.destroy | U C | remove one. Last 2FA-enabling factor = 2FA disabled (409 `two_factor_required_by_workspace`; app grants revoked except caller's). Recovery set while 2FA on: 409 `recovery_codes_required` |
| DELETE | `/api/v1/user/auth-factors` | user.auth-factors.destroy-many | U C | disable 2FA in one call: every factor incl. pending (same rules). None: 204 no-op |
| GET | `/api/v1/user/identities` | user.identities.index | U | linked GitHub/Google |
| DELETE | `/api/v1/user/identities/{idn}` | user.identities.destroy | U C | unlink (link itself = browser `oauth.redirect?intent=link`, S) |
| GET | `/api/v1/user/memberships` | user.memberships.index | U | `membership` resources with `workspace` included, sorted by last access; `filter[trashed]=with\|only` (owners only see trashed) |
| POST | `/api/v1/user/memberships` | user.memberships.store | U | `{invitation_token}` accept invitation (domain join `{workspace_id}` later, `domain-join`) |
| DELETE | `/api/v1/user/memberships/{mem}` | user.memberships.destroy | U | leave workspace |
| GET | `/api/v1/user/invitations` | user.invitations.index | U | pending invitations for my verified email |
| GET | `/api/v1/user/security-events` | user.security-events.index | U | own security log, cursor paginated |
| POST | `/api/v1/workspaces` | workspaces.store | U | create workspace |
| PATCH | `/api/v1/workspaces/{ws}` | workspaces.update | U C | `{trashed: false}` restore during grace (owner). Only field accepted here (other fields: `PATCH /workspace`) |
| GET | `/api/v1/invitations/preview` | invitations.preview.show | G (`Dimmi-Invitation-Token` header) | public invite info |

### Workspace-scoped (`Dimmi-Workspace` header for session and app; token implies workspace)
| Method | Path | Name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | `/api/v1/workspace` | workspace.show | A W, any member; `settings:read` for full settings | current workspace |
| PATCH | `/api/v1/workspace` | workspace.update | A W `settings:write`; field `require_two_factor` (setting `security.require_two_factor`) U C only | update |
| DELETE | `/api/v1/workspace` | workspace.destroy | U W C owner | soft delete (trash) |
| GET | `/api/v1/members` | members.index | A W `members:read` | list; `filter[status]=active,deactivated`, `filter[role]`, `filter[team]`, `filter[two_factor]` (`members:write`), `q` (name/email), `sort=name\|-created_at\|-last_active_at`, `include=teams,user`; `Accept: text/csv` (`members:write`) streams CSV |
| GET | `/api/v1/members/{mem}` | members.show | A W `members:read` | `me` alias allowed |
| PATCH | `/api/v1/members/{mem}` | members.update | A W; per field below | `role`, `status` (+ `reassign_to_member_id`), `keep_owner`, `custom_role_id`, `conversation_access`, `conversation_access_unassigned`, `access_team_ids` |
| PATCH | `/api/v1/members` | members.update-many | A W `members:write` | rest.md rule 8; `changes`: `role`, `custom_role_id` |
| POST | `/api/v1/members/{mem}/two-factor-resets` | members.two-factor-resets.store | U W C `members:reset_two_factor` | mail teammate a reset link |
| DELETE | `/api/v1/members/{mem}/sign-in-lock` | members.sign-in-lock.destroy | U W `members:reset_two_factor` | clear soft lock |
| POST | `/api/v1/member-imports` | member-imports.store | U W C `members:import` | CSV, `dry_run`; synchronous |
| PATCH | `/api/v1/members/me` `{availability}` | members.update | A W self; `conversations:write` | online/away (self may change only `availability`; other fields need role per PATCH /members/{mem} table) |
| GET | `/api/v1/teams` | teams.index | A W `members:read` | list, `include=members` |
| POST | `/api/v1/teams` | teams.store | A W admin `members:write` | create |
| GET | `/api/v1/teams/{team}` | teams.show | A W `members:read` | `include=conversation_counts` (`members:write`) |
| PATCH | `/api/v1/teams/{team}` | teams.update | A W admin `members:write` | rename |
| DELETE | `/api/v1/teams/{team}` | teams.destroy | A W admin `members:write` | delete; optional `reassign_to_team_id` or `reassign_to_member_id` query |
| POST | `/api/v1/teams/{team}/members` | teams.members.store | A W admin `members:write` | add members (bulk) |
| DELETE | `/api/v1/teams/{team}/members/{mem}` | teams.members.destroy | A W admin `members:write` | remove |
| GET | `/api/v1/invitations` | invitations.index | A W admin `members:read` | `filter[status]` default pending |
| POST | `/api/v1/invitations` | invitations.store | A W V admin `members:write` | bulk invite |
| POST | `/api/v1/invitations/{inv}/deliveries` | invitations.deliveries.store | A W admin `members:write` | resend (rotates token), 201 `invitation_delivery`; `{channel: "link"}` rotates without mail, returns `accept_url` |
| DELETE | `/api/v1/invitations/{inv}` | invitations.destroy | A W admin `members:write` | revoke |
| GET | `/api/v1/tokens` | tokens.index | U W | own; admin `filter[member]` |
| POST | `/api/v1/tokens` | tokens.store | U W C V | create (returns secret once) |
| DELETE | `/api/v1/tokens/{tok}` | tokens.destroy | U W (own or admin) | revoke |
| GET | `/api/v1/tokens/abilities` | tokens.abilities.index | U W | ability groups (with member permissions) + `allowed` for my role |
| GET | `/api/v1/audit-logs` | audit-logs.index | A W `audit_logs:read` (token: `settings:read`) | list/filter; `sort=-created_at` (default) or `created_at` (ascending tail for SIEM pull); `Accept: text/csv` streams CSV |
| GET | `/api/v1/audit-logs/{aud}` | audit-logs.show | A W `audit_logs:read` (token: `settings:read`) | single |
| GET, POST | `/api/v1/roles` | roles.index, roles.store | [roles.md](roles.md) | |
| GET, PATCH, DELETE | `/api/v1/roles/{rol}` | roles.show, .update, .destroy | [roles.md](roles.md) | |
| GET | `/api/v1/permissions` | permissions.index | A W `members:read` | catalog with labels |
| GET, PATCH | `/api/v1/workspace/access-policy` | workspace.access-policy.show, .update | [21 access-policy.md](../21-enterprise-identity/access-policy.md) | |

SSO connections, organization domains, SCIM tokens, SSO lookups: [21 README](../21-enterprise-identity/README.md).

`PATCH /members/{mem}` field permissions (policy `UpdateField` calls `authz.Can` per field; any field denied: whole request rejected, nothing changes):
| Field | Values | Permission | Ability (tokens) |
|---|---|---|---|
| `role` | `member` <-> `admin` on non-owner | `members:write` | `members:write` |
| `role` | `owner` (transfer; caller becomes admin same tx unless `keep_owner: true`) | `members:manage_owners`, U C | n/a: token 403 `first_party_only` |
| `role` | from `owner` to `admin`/`member` (demote owner, incl. self) | `members:manage_owners`, U C | n/a: token 403 `first_party_only` |
| `status` | `deactivated` (+ `reassign_to_member_id?`), `active` | `members:write` for member/admin target; `members:manage_owners` for owner target | `members:write` |
| `custom_role_id` | `rol_` or null | `roles:manage` + holds every permission in role | `members:write` |
| `conversation_access`, `conversation_access_unassigned`, `access_team_ids` | per 03 `access.md` | `members:manage_access` | `members:write` |
Illegal transition: 422 `invalid_transition` with `from`, `to`. Same value as current: 200, no event. Each changed field fires own event (`member.role_changed`, `member.deactivated`, `member.reactivated`).

Tokens (PAT, third-party OAuth) cannot create tokens, change owner roles, delete or restore workspaces, or reach any `U` route (403 `first_party_only`). Reason: stolen token must not escalate or persist. First-party app token can (it is the user, same as session).

## Key examples

### POST /api/v1/users
```json
{ "name": "Ada Lovelace", "email": "ada@acme.com", "password": "correct horse battery", "workspace_name": "Acme" }
```
201:
```json
{
  "data": {
    "object": "me",
    "user": { "id": "usr_01jc...", "object": "user", "name": "Ada Lovelace", "email": "ada@acme.com", "email_verified": false, "two_factor_enabled": false, "avatar_url": null, "locale": "en", "timezone": null, "has_password": true, "created_at": "2026-11-01T10:00:00Z" },
    "workspace": { "id": "ws_01jc...", "object": "workspace", "name": "Acme", "slug": "acme", "timezone": "UTC", "locale": "en", "test_mode": false },
    "member": { "id": "mem_01jc...", "object": "member", "role": "owner", "status": "active", "availability": "online",
      "permissions": ["agent_access:manage", "ai:keys", "ai:read", "ai:write", "articles:read", "articles:write", "audit_logs:read", "billing:cap_unrestricted", "billing:read", "billing:write", "...", "workspace:delete"] },
    "auth": { "type": "session", "session_id": "ses_01jc...", "abilities": ["*"], "effective_abilities": ["ai:actions", "ai:keys", "ai:read", "ai:write", "articles:read", "articles:write", "billing:read", "billing:write", "changelog:read", "changelog:write", "contacts:read", "contacts:write", "conversations:read", "conversations:write", "engage:read", "engage:write", "exports:read", "exports:write", "feedback:read", "feedback:write", "members:read", "members:write", "notifications:read", "notifications:write", "settings:read", "settings:write", "webhooks:read", "webhooks:write"], "auth_methods": ["password"], "two_factor_used": false, "authenticated_at": "2026-11-01T10:00:00Z" },
    "workspaces": [ { "id": "ws_01jc...", "object": "workspace", "name": "Acme", "slug": "acme", "role": "owner" } ],
    "instance": { "edition": "cloud", "version": "1.0.0", "api_version": "2026-11-01", "region": "us", "mail_configured": true, "server_time": "2026-11-01T10:00:00Z" }
  }
}
```

### POST /api/v1/sessions
```json
{ "grant": "password", "email": "ada@acme.com", "password": "correct horse battery" }
```
201 `MeResource` (same shape as register) + `Set-Cookie: __Host-dimmi_session=...`.

2FA needed: 200, no session:
```json
{ "data": { "object": "two_factor_challenge", "expires_at": "2026-11-01T10:05:00Z",
  "methods": [
    { "method": "totp", "factor_id": "fac_01jc...", "name": "Authenticator app", "preferred": true, "options": null },
    { "method": "recovery_code", "factor_id": "fac_01jd...", "name": "Recovery codes", "preferred": false, "options": null } ] } }
```
+ `Set-Cookie: __Host-dimmi_2fa=...`. `methods` = active factors of enabled drivers with `SecondFactor` (preferred first; recovery set omitted when `remaining = 0`); `options` = driver challenge options (null for totp and recovery_code; WebAuthn request options for `passkey`, the same on each passkey; recovery set listed last). Then with cookie:
```json
{ "grant": "two_factor", "method": "totp", "code": "123456" }
```
or `{"grant": "two_factor", "method": "recovery_code", "code": "7k2m9-x4q8z"}`: 201 `MeResource` (`auth.auth_methods`: `["password", "totp"]` or `["password", "recovery_code"]`). Missing `method`: 422 `errors.method` `required`. `method` not offered by this challenge (not enrolled, disabled driver, recovery set exhausted): 422 `errors.method` `method_not_available` (no attempt counted). Wrong code: 422 `errors.code` `invalid_code`.

Same challenge response from `magic_link` / `email_code` grants and `PATCH /password-resets/{prs}` when user has 2FA.

422:
```json
{ "message": "These credentials do not match our records.", "code": "invalid_credentials", "errors": { "email": ["These credentials do not match our records."] } }
```
Unknown `grant`: 422 `errors.grant` (`in` rule). `magic_link` / `email_code` while `auth.magic_link.enabled=false`: 422 `errors.grant` `grant_not_allowed`.
429: `{ "message": "Too many login attempts. Try again in 30 seconds.", "code": "too_many_attempts" }` + `Retry-After: 30`.

### PATCH /api/v1/password-resets/{prs}
Link in mail: `https://{app}/reset-password#id=prs_01jc...&token=...`. SPA sends:
```json
{ "token": "q8Zr...", "password": "new correct horse" }
```
200 `MeResource` + session cookie (`auth_method=password_reset`), or 200 `two_factor_challenge`. Unknown `prs_` id, wrong or expired token: 422 `errors.token` `invalid_reset_token` (identical body and timing).

### PATCH /api/v1/email-verifications/{ott}
Link: `https://{app}/verify-email#id=ott_01jc...&token=...`. Body `{"token":"..."}`. 200:
```json
{ "data": { "object": "email_verification", "id": "ott_01jc...", "email": "ada@acme.com", "status": "verified", "verified_at": "2026-11-01T10:02:00Z", "user_id": "usr_01jc..." } }
```
`PATCH /email-changes/{ott}` same shape with `object: "email_change"`, `status: "confirmed"`.

### GET /api/v1/me (token)
```json
{
  "data": {
    "object": "me",
    "user": { "id": "usr_...", "object": "user", "name": "Ada Lovelace", "email": "ada@acme.com", "email_verified": true, "two_factor_enabled": true },
    "workspace": { "id": "ws_...", "object": "workspace", "name": "Acme", "slug": "acme", "timezone": "Europe/Berlin", "locale": "en", "test_mode": false },
    "member": { "id": "mem_...", "object": "member", "role": "admin", "status": "active", "availability": "away", "permissions": ["conversations:delete", "conversations:moderate", "conversations:read", "conversations:write", "saved_replies:manage_shared"] },
    "auth": {
      "type": "personal_access_token",
      "token": { "id": "tok_...", "object": "personal_access_token", "name": "CI", "prefix": "dmi_live_Ab12", "expires_at": null },
      "abilities": ["conversations:read", "conversations:write"],
      "effective_abilities": ["conversations:read", "conversations:write"]
    },
    "workspaces": null,
    "instance": { "edition": "self_host", "version": "1.0.0", "api_version": "2026-11-01", "region": "self", "mail_configured": false, "server_time": "2026-11-01T10:00:00Z" }
  }
}
```
`member.permissions` = effective permissions of caller in this workspace (session/app: role permissions; token: role permissions intersect expanded abilities). Clients gate UI on it, never on `role`. `auth.auth_methods` / `authenticated_at`: session from row, app from grant (09 `auth_methods`, `auth_time`); absent for tokens. `two_factor_used` derived (methods contain a second factor). Session or first-party app without `Dimmi-Workspace` header: `/me` is only workspace-aware endpoint that does not return 400 `workspace_required`; returns `workspace: null`, `member: null`, `workspaces: [...]`. `auth.type`: `session` (with `session_id`), `app` (first-party; `client` + `grant_id`), `personal_access_token` (`token`), `oauth` (09, third-party; `client`).

### POST /api/v1/tokens
```json
{ "name": "CI deploy", "abilities": ["conversations:read", "articles:write"], "expires_at": "2027-02-01T00:00:00Z" }
```
201:
```json
{
  "data": {
    "id": "tok_01jc...", "object": "personal_access_token", "name": "CI deploy",
    "prefix": "dmi_live_Ab12", "abilities": ["conversations:read", "articles:write"],
    "expires_at": "2027-02-01T00:00:00Z", "last_used_at": null, "last_used_ip": null, "revoked_at": null,
    "member": { "id": "mem_...", "object": "member", "name": "Ada Lovelace" },
    "created_at": "2026-11-01T10:00:00Z",
    "plain_text_token": "dmi_live_Ab12cD34eF56gH78iJ90kL12mN34oP56q7R8s9"
  }
}
```
`plain_text_token` present only in this response. Response header `Cache-Control: no-store`.

### POST /api/v1/invitations
```json
{ "invitations": [
  { "email": "grace@acme.com", "role": "admin", "team_ids": ["team_01jc..."] },
  { "email": "bob@acme.com", "role": "member" },
  { "email": "not-an-email", "role": "member" }
] }
```
200 with per-email result (partial success, never silent; 207 not used):
```json
{
  "data": [
    { "email": "grace@acme.com", "result": "created", "invitation": { "id": "inv_...", "object": "invitation", "email": "grace@acme.com", "role": "admin", "status": "pending", "expires_at": "2026-11-08T10:00:00Z", "accept_url": null } },
    { "email": "bob@acme.com", "result": "already_member", "invitation": null },
    { "email": "not-an-email", "result": "invalid", "error": { "code": "invalid_email", "message": "The email must be a valid email address." } }
  ]
}
```
`accept_url` returned to every caller with `members:write` (A-07). Token only hashed, so a later copy needs `deliveries {channel: "link"}`, which rotates it.

### POST /api/v1/invitations/{inv}/deliveries
No body. 201:
```json
{ "data": { "object": "invitation_delivery", "sent_at": "2026-11-02T09:00:00Z", "invitation": { "id": "inv_...", "object": "invitation", "status": "pending", "expires_at": "2026-11-09T09:00:00Z", "send_count": 2, "accept_url": null } } }
```
Computed output, no stored delivery row (invitation `last_sent_at`, `send_count` updated).

### GET /api/v1/invitations/preview (header `Dimmi-Invitation-Token`)
```json
{ "data": { "object": "invitation_preview", "workspace": { "name": "Acme", "avatar_url": null }, "inviter": { "name": "Ada Lovelace" }, "role": "member", "email_masked": "g***@acme.com", "status": "pending", "expires_at": "2026-11-08T10:00:00Z", "has_account": false } }
```

### POST /api/v1/user/memberships
```json
{ "invitation_token": "Zx9..." }
```
201:
```json
{ "data": { "id": "mem_01jd...", "object": "membership", "role": "member", "status": "active", "joined_via": "invitation", "last_accessed_at": null,
  "workspace": { "id": "ws_01jc...", "object": "workspace", "name": "Acme", "slug": "acme", "deleted_at": null, "purge_after": null } } }
```
Both or neither field: 422.

### PATCH /api/v1/members/{mem}
Deactivate:
```json
{ "status": "deactivated", "reassign_to_member_id": "mem_01jd..." }
```
200 member resource with `status: "deactivated"`. Reactivate `{"status": "active"}` (optional `role`). Ownership transfer `{"role": "owner"}` (owner, recent auth): 200 target member; caller's own member now `admin` (`keep_owner: true` keeps caller owner = plain promotion).

### PATCH /api/v1/workspaces/{ws}
```json
{ "trashed": false }
```
200 workspace resource (`deleted_at: null`, `purge_after: null`). Any other field: 422 `errors.<field>` `prohibited`.

### GET /api/v1/sessions
```json
{ "data": [
  { "id": "ses_...", "object": "session", "kind": "browser", "current": true, "auth_method": "github", "auth_methods": ["github", "totp"], "two_factor_used": true, "authenticated_at": "2026-10-20T08:00:00Z", "device_label": "Firefox on Linux", "client": null, "platform": null, "app_version": null, "ip": "203.0.113.4", "country": "DE", "last_activity_at": "2026-11-01T09:58:00Z", "created_at": "2026-10-20T08:00:00Z", "expires_at": "2026-11-19T08:00:00Z" },
  { "id": "ogr_...", "object": "session", "kind": "app", "current": false, "auth_method": null, "auth_methods": ["password", "totp"], "two_factor_used": true, "authenticated_at": "2026-09-02T18:00:00Z", "device_label": "Ada's iPhone", "client": { "id": "ocl_...", "name": "Dimmi" }, "platform": "ios", "app_version": "1.4.0", "ip": "198.51.100.7", "country": "DE", "last_activity_at": "2026-11-01T07:12:00Z", "created_at": "2026-09-02T18:00:00Z", "expires_at": null }
], "meta": { "next_cursor": null, "prev_cursor": null, "per_page": 25 } }
```
`kind: app` rows come from 09 `ListUserAppGrants(userID, currentGrantID)` (active user-wide grants of `first_party` system clients). Mapping: `id` = grant `ogr_`, `device_label` = `device_name`, `platform` = `device_platform`, `app_version`, `ip` = `last_used_ip`, `last_activity_at` = `last_used_at`, `created_at`, `current`; `auth_methods` = grant `auth_methods`, `authenticated_at` = grant `auth_time`; `client` = `{id, name}` from row `client_id`, `client_name`; `country` = GeoIP of `ip` computed here (null without GeoIP DB). `device_id` never exposed (internal). Third-party OAuth grants never listed here (09 connected apps). Sorted `last_activity_at desc`, cursor over merged `(last_activity_at, id)`. `current: true` on the caller's own session or grant.

### GET /api/v1/user/auth-factors
```json
{ "data": [
  { "id": "fac_01jc...", "object": "auth_factor", "type": "totp", "name": "Work phone", "status": "active", "preferred": true, "confirmed_at": "2026-10-01T09:00:00Z", "last_used_at": "2026-11-01T08:00:00Z", "created_at": "2026-10-01T08:59:00Z" },
  { "id": "fac_01jd...", "object": "auth_factor", "type": "recovery_code", "name": "Recovery codes", "status": "active", "preferred": false, "remaining": 8, "total": 10, "confirmed_at": "2026-10-01T09:00:00Z", "last_used_at": null, "created_at": "2026-10-01T09:00:00Z" }
], "meta": { "next_cursor": null, "prev_cursor": null, "per_page": 25 } }
```
Only drivers with `UserVisible`. Factors of disabled drivers still listed (deletable). Never secrets, never `lookup_hash`, `last_counter`.

### POST /api/v1/user/auth-factors
```json
{ "type": "totp", "name": "Work phone" }
```
201 (`Cache-Control: no-store`):
```json
{ "data": { "id": "fac_01jc...", "object": "auth_factor", "type": "totp", "name": "Work phone", "status": "pending", "preferred": false, "confirmed_at": null, "last_used_at": null, "created_at": "2026-11-01T10:00:00Z",
  "setup": { "secret": "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP", "otpauth_url": "otpauth://totp/Dimmi:ada%40acme.com?secret=JBSW...&issuer=Dimmi&algorithm=SHA1&digits=6&period=30", "qr_svg": "<svg ...>" } } }
```
`setup` only in this response. `type` not in enabled registry: 422 `errors.type` `in`. Active `totp` already present (driver `Multiple` false): 422 `errors.type` `factor_limit_reached`. Active factors at `auth.factors.max_per_user`: 422 `factor_limit_reached`. Earlier pending factor of same type: replaced (deleted) by this one.

`{ "type": "recovery_code" }` (regenerate; 2FA must be on, else 409 `two_factor_not_enabled`): old set deleted, new set inserted active, 201:
```json
{ "data": { "id": "fac_01je...", "object": "auth_factor", "type": "recovery_code", "name": "Recovery codes", "status": "active", "remaining": 10, "total": 10, "codes": ["7k2m9-x4q8z", "..."] } }
```

### PATCH /api/v1/user/auth-factors/{fac}
```json
{ "code": "123456" }
```
200 `auth_factor` with `status: "active"`. When this activation enabled 2FA (first 2FA-enabling factor), response adds one-time `recovery_codes` (10) and `Cache-Control: no-store`:
```json
{ "data": { "id": "fac_01jc...", "object": "auth_factor", "type": "totp", "name": "Work phone", "status": "active", "preferred": true, "confirmed_at": "2026-11-01T10:01:00Z",
  "recovery_codes": { "id": "fac_01jd...", "object": "auth_factor", "type": "recovery_code", "remaining": 10, "total": 10, "codes": ["7k2m9-x4q8z", "..."] } } }
```
Wrong code: 422 `errors.code` `invalid_code`. `{code}` on active factor: 422 `invalid_transition` (`from: active`, `to: active`). Pending factor past 1 h: 404 `not_found` (purged or treated gone). `{name: "Old phone"}` or `{preferred: true}`: 200 (preferred moves from other factor in same tx; recovery set cannot be preferred: 422 `errors.preferred`).

### DELETE /api/v1/user/auth-factors/{fac} and DELETE /api/v1/user/auth-factors
204. Removing last 2FA-enabling factor (or collection delete): 2FA disabled, recovery set deleted, `TwoFactorDisabledMail`, first-party app grants revoked except caller's. Member of active workspace with `security.require_two_factor`: 409 `two_factor_required_by_workspace` `{workspaces: [{id, name}]}`, nothing removed. Recovery set alone while 2FA on: 409 `recovery_codes_required`.

### POST /api/v1/user/confirmation
```json
{ "method": "totp", "code": "123456" }
```
`method`: `password` (+ `password`), `email_code` (+ `code` from `POST /user/confirmation-codes`, passwordless users), or factor type with `Confirmation` capability and an active factor (MVP `totp` + `code`). Other or not enrolled: 422 `errors.method` `method_not_available`. 201 `{ "data": { "object": "confirmation", "method": "totp", "confirmed_until": "2026-11-01T10:15:00Z" } }`. Browser: session `confirmed_at`, `confirmed_method`. `GET /user/confirmation` same shape, 404 when none or expired.

### GET /api/v1/audit-logs
Filters: `filter[event]=member.role_changed,token.created`, `filter[actor]=mem_...`, `filter[via]=tok_...|ocl_...|ogr_...` (personal token, OAuth client, first-party grant), `filter[subject]=mem_...`, `filter[created_after]`, `filter[created_before]`. Sort `-created_at` (default) or `created_at` (ascending, stable cursor on `(created_at, id)`; SIEM pull with `filter[created_after]`). CSV: same filters, `Accept: text/csv`, streamed, max 366-day range (422 `range_too_large`).
```json
{ "data": [
  { "id": "aud_...", "object": "audit_log", "event": "member.role_changed",
    "actor": { "type": "member", "id": "mem_...", "via": { "kind": "session", "id": "ses_..." }, "label": "Ada Lovelace <ada@acme.com>" },
    "subject": { "type": "member", "id": "mem_...", "label": "Grace Hopper" },
    "changes": { "role": { "from": "member", "to": "admin" } },
    "ip": "203.0.113.4", "user_agent": "Mozilla/5.0 ...", "request_id": "req_...", "created_at": "2026-11-01T10:00:00Z" }
], "meta": { "next_cursor": "...", "prev_cursor": null, "per_page": 25 } }
```

### GET /api/v1/auth/config
```json
{ "data": { "object": "auth_config", "edition": "self_host", "server_version": "1.0.0", "api_version": "2026-11-01", "region": "self",
  "setup_required": false, "signup_mode": "invite_only", "password_login": true, "magic_link": true, "captcha": null, "mail_configured": false, "password_min_length": 10,
  "providers": [ { "key": "github", "name": "GitHub", "protocol": "oauth2", "redirect_url": "/auth/github/redirect" } ],
  "factors": [ { "type": "totp", "name": "Authenticator app" }, { "type": "recovery_code", "name": "Recovery codes" } ],
  "capabilities": ["auth_factors", "magic_link", "email_code", "first_party_apps"],
  "min_client_versions": { "ios": null, "android": null, "cli": null } } }
```
Capability discovery (P7): apps read before login; newer app against older server hides missing features. `providers` = configured `IdentityProvider` drivers (`Display()`); `factors` = enabled factor drivers; `capabilities` = string tokens from registry (each spec registers its own, e.g. 06 `push_web`); `min_client_versions` from `apps.min_versions.*` (null = no gate). Arrays of objects (P6): new fields per entry additive.

## Error codes (stable)

| Status | Code | When |
|---|---|---|
| 400 | `workspace_required` | session or app request without `Dimmi-Workspace` on workspace route (user has more than one workspace) |
| 401 | `unauthenticated` | no/invalid session or app token |
| 401 | `invalid_token` / `token_expired` / `token_revoked` | bearer issues (unknown, bad checksum, expired, revoked) |
| 401 | `two_factor_challenge_expired` | challenge cookie missing/expired/burned |
| 403 | `first_party_only` | PAT or third-party OAuth token on `U` route or owner-only field |
| 403 | `missing_ability` | foundation, with `required_ability` |
| 403 | `forbidden` | role insufficient |
| 403 | `two_factor_required` | `AccessPolicy` failure (workspace `security.require_two_factor`, user without 2FA). Body `{code, message, requirement: "two_factor", action_url: "/settings/security/two-factor"}`. Every policy failure uses this shape; clients branch on `requirement` (`two_factor`, `reauthentication_required`, `sign_in_method`, `sso_required`, `ip_not_allowed`; 19 access-policy.md) |
| 403 | `email_unverified` | `verified` middleware |
| 403 | `membership_deactivated` | member of workspace but deactivated or removed |
| 403 | `signup_closed` | signup not allowed |
| 403 | `invitation_email_mismatch` | logged-in email differs from invitation |
| 403 | `workspace_suspended` | write on suspended workspace |
| 403 | `cross_origin_request` | unsafe browser request rejected by `http.CrossOriginProtection` (cross-site `Sec-Fetch-Site` or untrusted `Origin`) |
| 403 | `workspace_mismatch` | token used with `Dimmi-Workspace` of other workspace |
| 403 | `invitations_paused` | cloud: workspace invite complaints over threshold |
| 404 | `workspace_not_found` | unknown, other, or deleted workspace |
| 404 | `not_found` | any foreign id; `GET /user/confirmation` when none |
| 409 | `last_owner` | demote/deactivate/leave last owner |
| 409 | `sole_owner` | account deletion blocked |
| 409 | `already_member` | single-invite style conflicts; `POST /user/memberships` when already active |
| 409 | `invitation_already_accepted` | |
| 503 / 410 | `setup_required` / `setup_completed` | first-run state (11 owns) |
| 409 | `two_factor_not_enabled` / `two_factor_required_by_workspace` / `recovery_codes_required` | regenerate recovery codes or toggle workspace requirement without 2FA; removing last 2FA factor blocked (with `workspaces` list); deleting recovery set while 2FA on |
| 409 | `sso_required` | `LoginPolicy`: address under enforced verified domain, with `redirect_url` (21) |
| 403 | `sign_in_method_required` / `reauthentication_required` / `ip_not_allowed` | `AccessPolicy` failures (21) |
| 403 | `managed_by_organization` | managed account field or delete (21) |
| 409 | `managed_by_directory` | change to directory-managed member (21) |
| 409 | `two_factor_reset_blocked` | teammate works in another organization |
| 409 | `role_in_use` | delete custom role with members |
| 409 | `last_login_method` | unlink would lock user out |
| 409 | `token_limit_reached` / `workspace_limit_reached` | |
| 409 | `email_taken` | email change race |
| 409 | `target_unverified` / `target_two_factor_required` | ownership transfer target not eligible |
| 410 | `workspace_purging` | restore after purge started |
| 426 | `client_upgrade_required` | foundation `client.version`: `Dimmi-Client-Version` below `apps.min_versions.<platform>`; body `{code, message, min_version, upgrade_url}` |
| 400 | `invalid_api_version` | foundation: unknown `Dimmi-Version` |
| 410 | `magic_link_expired` / `invitation_expired` / `invitation_revoked` / `verification_link_stale` / `verification_link_expired` | |
| 422 | Laravel shape; codes inside: `invalid_credentials`, `invalid_code`, `method_not_available`, `factor_limit_reached`, `reauth_user_mismatch`, `setting_locked` (reserved: no org lock UI exists), `sign_in_method_excludes_caller`, `no_owner_can_sign_in`, `ip_allowlist_excludes_caller`, `permission_not_assignable`, `permission_above_base_role`, `permission_not_held`, `unknown_column`, `invalid_reset_token`, `invalid_verification_token`, `invalid_email_change_token`, `grant_not_allowed`, `invalid_transition` (with `from`, `to`), `disposable_email`, `ability_not_allowed`, `captcha_failed`, `password_compromised`, `password_too_common`, `slug_reserved`, `range_too_large`, `test_mode_immutable`, `invitation_email_mismatch`, `prohibited` | |
| 423 | `password_confirmation_required` | recent auth missing (Laravel `password.confirm` semantics). 423 shared: 12 billing returns 423 `spend_cap_reached` (and other billing block codes) on replies. Clients switch on `code`, never on status alone |
| 429 | `too_many_attempts` / `invitation_limit_reached` / `too_many_requests` | with `Retry-After`; last is API bucket ([rate-limits.md](rate-limits.md)) |
| 503 | `auth_busy` | hash semaphore wait over 2 s |
| 503 | `captcha_unavailable` | cloud register when captcha provider unreachable |

Revoked token note: return `token_revoked` (helps owner debug CI); token secret already proves possession so no new information leaks.

## Rate limits (in addition to foundation buckets)

API buckets per token, user, workspace: [rate-limits.md](rate-limits.md). Table below: per-action limits.

| Bucket | Limit | Key |
|---|---|---|
| `POST /sessions` grant `password` | 10/min + escalating lockout (flows.md section 12) | IP + email |
| `POST /users` | cloud 5/hour, self-host 20/hour | IP |
| `POST /magic-links` | 3 per 10 min per email; 20/hour per IP | silent drop |
| `POST /password-resets` | 1/min and 5/hour per email; 20/hour per IP | silent drop |
| `POST /sessions` grants `magic_link`, `email_code`, `two_factor`; `PATCH /password-resets/{prs}`; `POST /user/confirmation` | 10/min per IP; 5 per token | |
| `POST /user/auth-factors` | 10/hour | user |
| `PATCH /user/auth-factors/{fac}` with `code` | 10/min; 5 wrong codes per pending factor deletes it | user, factor |
| `POST /email-verifications` | 1/min, 6/hour | user |
| `PATCH /email-verifications/{ott}`, `PATCH /email-changes/{ott}` | 10/min per IP; 5 wrong tokens per id burns it | IP, id |
| `POST /user/confirmation-codes` | 1/min, 5/hour | user |
| setup | 10/min | IP |
| invitation preview, `POST /user/memberships` | 30/min | IP |
| invitation create | cloud 100/day per workspace; 50 per request | workspace |
| `POST /invitations/{inv}/deliveries` | 3 per invitation per 24 h | invitation |
| token create | 20/hour | user |
| `POST /email-changes` | 5/hour | user |
| `POST /members/{mem}/two-factor-resets` | 3/24 h | target member |
| `POST /member-imports` | 5/hour | workspace |
| audit log, member CSV | 10/hour | workspace |
Self-host: `throttle` config can raise or disable all except lockout (always on).

## MCP tools (`app/mcp/tools`)

| Tool | Ability | Purpose |
|---|---|---|
| (09 core `dimmi_get_workspace`) | any | replaces `dimmi_get_me`: me, abilities, teammates (max 100), teams. 00-accounts supplies `MeResource`, member/team queries |
| `dimmi_list_members` | `members:read` | toolset `accounts` (09 feature toolset; off by default): paginated members beyond 100, filter team/availability |
| `dimmi_list_teams` | `members:read` | toolset `accounts`: teams with member ids |
| `dimmi_update_availability` | `conversations:write` | toolset `accounts`: set own availability online/away (`PATCH /members/me {availability}`) |
Deliberately no MCP tools for invitations, roles, tokens, workspace delete (agent prompt injection must not escalate access).

## CLI commands (`dimmi ...`)

| Command | Behavior |
|---|---|
| `user:create --email --name [--password | --password-stdin] [--instance-admin] [--workspace "Acme" | --workspace-slug acme] [--role owner|admin|member] [--no-interaction]` | Prompts password (hidden, twice) unless `--password` or `DIMMI_USER_PASSWORD`; `--generate-password` prints one. User verified, `created_via=cli`. No workspace exists: creates one via `CreateWorkspace` (`--workspace` or "My workspace", runs workspace initializer hook) with user owner. Completes first-run setup (deletes 11 setup token). `--instance-admin` sets `users.is_instance_admin` (11). Existing email: adds membership if `--workspace-slug` given, else exit 1 |
| `user:list [--json]` | id, email, verified, 2FA (factor types), workspaces |
| `user:reset-password <email>` | prints reset link (`/reset-password#id=prs_...&token=...`, 60 min) to stdout; with `--password` sets directly and revokes sessions and first-party app grants |
| `user:disable-2fa <email>` | `RemoveAllAuthFactors` with `force` (skips `two_factor_required_by_workspace`; operator recovery for lost device): deletes every factor, clears `two_factor_confirmed_at`, 09 `RevokeUserAppGrants(two_factor_disabled)`, security events actor `system` via `cli`, `TwoFactorDisabledMail`. User then hits 403 `two_factor_required` in requiring workspaces until new factor added |
| `user:verify-email <email>` | sets verified |
| `user:logout <email>` | revokes all sessions and first-party app grants (09 `RevokeUserAppGrants(admin)`) |
| `workspace:list [--with-deleted] [--json]` | |
| `workspace:suspend <slug> --reason` / `workspace:unsuspend <slug>` | |
| `workspace:restore <slug>` | restore during grace |
| `workspace:purge <slug> --force` | purge now (only already soft-deleted workspaces) |
| `workspace:repair [<slug>]` | re-run workspace initializer hook (00-foundation/contracts.md); idempotent; all workspaces when no slug |
| `token:create --email --workspace-slug --name --abilities a,b [--expires 90d] [--json]` | prints token once; for headless bootstrap (CI, Terraform) |
All write commands log audit/security events with `Actor{Type: system, Via: {Kind: cli}}`. Commands work with app stopped (direct DB). CLI command names stay verbs (rest.md rule 15).

## Mail templates

Queue `mail`. Auth-critical (*) use River priority 1 and never batched. All templ, plain-text part, rendered in recipient locale: `a.Mail.To(addr).Locale(user.locale or workspace.locale).QueueTx(ctx, tx, m)` (unset falls back to `app.locale`, `en`; subject/body strings via `i18n.T` keys in `lang/<locale>/mail.json`). No tracking pixels. Links use `route()` and fragment tokens (`#id=<public id>&token=...` or `#token=...`).

| Mailable | To | Trigger |
|---|---|---|
| `VerifyEmailMail`* | user | register, `POST /email-verifications` (link `/verify-email#id=ott_...&token=...`, 24 h) |
| `MagicLinkMail`* | email | magic link request (link + 6-digit code, requester IP/device) |
| `ResetPasswordMail`* | user | `POST /password-resets` (link `/reset-password#id=prs_...&token=...`) |
| `ConfirmCodeMail`* | user | `POST /user/confirmation-codes` (sudo code for passwordless users) |
| `EmailChangeVerificationMail`* | new address | email change request (link `/email/confirm#id=ott_...&token=...`) |
| `EmailChangeRequestedMail` | old address | email change request |
| `EmailChangedMail` | old address | email change confirmed |
| `PasswordChangedMail` | user | change, reset, pre-hijack password removal |
| `NewSignInMail` | user | login from new device |
| `SuspiciousSignInAttemptsMail` | user | per-email soft lock (once per 24 h) |
| `TwoFactorEnabledMail` | user | 2FA enabled (first 2FA-enabling factor activated) |
| `TwoFactorDisabledMail` | user | 2FA disabled (last 2FA-enabling factor removed: self or CLI) |
| `AuthFactorAddedMail` | user | factor activated while 2FA already on (factor name, type, IP/device). Unreachable at MVP (one totp max); fires for later drivers |
| `RecoveryCodeUsedMail` | user | recovery code used (remaining count; 2 or fewer: "regenerate now"; 0: "no codes left") |
| `IdentityLinkedMail` | user | GitHub/Google linked (manual or auto) |
| `TokenCreatedMail` | user | PAT created |
| `TokenExpiringMail` | user | 7 days before expiry |
| `AccountDeletedMail` | former address | account deleted |
| `InvitationMail`* | invitee | invite, `POST /invitations/{inv}/deliveries` |
| `InvitationAcceptedMail` | inviter | accept |
| `OwnershipTransferredMail` | new and old owner | transfer |
| `MemberDeactivatedMail` | removed user | deactivation (not on self-leave) |
| `TwoFactorRequiredMail` | affected members | workspace enables require 2FA |
| `WorkspaceDeletionScheduledMail` | owners | delete |
| `WorkspaceDeletionReminderMail` | owners | 3 days before purge |
| `WorkspaceRestoredMail` | owners | restore |
| `WorkspaceSuspendedMail` | owners | suspend |
| `OwnerPromotedBySystemMail` | promoted member | `CheckWorkspaceOwners` repair |
| `TwoFactorResetMail`* | teammate | admin starts reset (A-05) |
| `TwoFactorResetCompletedMail` | teammate | reset finished |
| `InvitationPendingMail` | inviter | pending 3 days, once (A-07) |

SMTP not configured (self-host, `mail.default=log`): mails rendered to log at WARN with `mail_fallback=true`, subject, recipient, and every link; `auth/config` and `/me` expose `mail_configured=false`; invitation responses include `accept_url`.
