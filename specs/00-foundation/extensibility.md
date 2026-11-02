# Extensibility seams (grow after launch without reshaping core)

Style: caveman. Date: 2026-11-02. Author: architect review. Status: accepted 2026-11-02 (product owner asked for these seams; open questions take recommended defaults). Change list applied to owner specs; this file explains why seams exist and is binding for new specs.

Goal (product owner): mobile app, multi-device login, B2B, enterprise ship after launch as additive work: new rows, new drivers, new enum values, new nullable columns, new endpoints. Never migrations that reshape core tables, never rewrites of auth or billing.

Rule for this file: no post-MVP feature enters MVP scope. Only seams: small schema, interface, API-shape costs paid now. Each seam marked cost `low` (hours, mostly spec + one table/column) or `med` (a day or two across specs).

## Principles (binding)

| # | Rule | Why |
|---|---|---|
| P1 | Driver-backed enums ("registry enums") = `text` + format CHECK + Go registry validation. State machines keep strict CHECK. | New provider, factor, channel = new Go driver, zero migration. CHECK swap on big table (`contact_identities`) needs full validation scan |
| P2 | Policies check permissions, never role names. Only `roles.go` maps role to permissions. | Custom roles later = new rows, no policy rewrite. Old mobile app builds keep working (gate UI on `permissions`, not `role`) |
| P3 | Every auth event records how user proved identity: methods used, when, which device. Sessions and first-party grants alike. | Step-up, org session policies, passkeys, SSO enforcement read this. Missing data cannot be backfilled |
| P4 | Enterprise things hang off `organizations`, never off a workspace. Org exists from day one, 1:1, invisible. | SSO, SCIM, consolidated billing, policies, CMK, residency attach later without moving data |
| P5 | Cross-cutting hooks exist now as noop drivers (entitlements, retention guard, login policy, access policy, audit sink). | Enterprise behavior later = new driver, not edits in 20 call sites |
| P6 | Extensible API lists are arrays of objects, not arrays of strings (`methods`, `providers`, `factors`). | New fields per entry stay additive under oasdiff |
| P7 | Clients discover server capabilities; servers can refuse too-old clients. | Self-host servers lag; app store builds lag. Both directions skew |
| P8 | Every tenant-owned row carries `workspace_id` (incl. join tables). User-scoped rows carry `user_id`. Nothing scoped only through a parent join. | RLS, workspace export/move between regions, org-level reads |

## Summary

| # | Area | Decision | Cost | Owner |
|---|---|---|---|---|
| 1 | Auth factors | `user_auth_factors` table + `Factor` driver registry; `/user/auth-factors` collection; challenge `methods` as objects | low-med | 00-accounts, rest.md |
| 2 | Login methods, IdP | `IdentityProvider` registry, `LoginPolicy` hook, provider registry enum; SSO tables later on org | low | 00-accounts |
| 3 | Organizations | YES: thin `organizations`, `workspaces.organization_id not null`, auto 1:1, no API in MVP | low | foundation, 00-accounts |
| 4 | Roles, permissions | Keep role enum; permission catalog in code; `/me` returns `permissions`; conversation visibility via one scope builder. No roles table | low | 00-accounts, api.md, 03 |
| 5 | Sessions, devices | `sessions.authenticated_at`, `auth_methods`, `confirmed_method`, `device_id`; same on `oauth_grants`; `AccessPolicy` middleware replaces `two_factor.enforced` | low | 00-accounts, 09, 06 |
| 6 | Entitlements | `Entitlements` contract, limit/feature catalog, `entitlement_overrides` (cloud), `GET /workspace/entitlements` | low | contracts, 12 |
| 7 | Settings | Scoped typed settings registry (instance, organization, workspace, user) over existing jsonb; no EAV table | low | foundation, 11 |
| 8 | Audit, compliance | `RetentionGuard` hook in every purge/erase; ascending audit cursor; `encryption_keys` keysets per scope + purpose; region via cells | low | foundation, 00-accounts |
| 9 | Multi-brand | Inbox not brand, but no seam now: `brands` + nullable `brand_id` on channel_connections later is additive | none | rejected |
| 10 | Users vs contacts | Contacts never in `users`; portal later on contact sessions; `contacts.company_id` documented as primary company | none-low | 03, 01 |
| 11 | API, integrations | `Dimmi-Version` header reserved and echoed; `webhook_endpoints.api_version`; scopes = permission strings; marketplace later additive | low | api.md, 10 |
| 12 | Mobile | `Dimmi-Client-Version` + 426 gate; capability discovery in `/auth/config`; tombstone rule for synced resources; push `device_id` | low | api.md, 00-accounts, 06 |
| 13 | Locale, currency | `billing_accounts.currency`; amounts = minor units. Nothing else | low | 12 |
| 14 | IDs, tenancy | RLS policies via `dimmi_visible_workspaces()`; `team_members.workspace_id`; keep `id` PKs | low | foundation, 00-accounts |

## 1. Authentication factors

Future: passkeys (WebAuthn) as 2FA and as passwordless login, several TOTP apps, security keys, email OTP as second factor, step-up for sensitive actions, org policy "phishing-resistant MFA only", "remember this device".

Breaks if MVP stays: `users.two_factor_secret_encrypted`, `two_factor_recovery_codes_encrypted`, `two_factor_last_used_step` hold exactly one TOTP. Passkeys need N credentials per user, each with public key, sign count, credential-id lookup. Adding means new table + data migration + rewrite of challenge, confirmation, recovery, `require_two_factor`, CLI `user:disable-2fa`. API `/user/two-factor` singleton cannot hold two authenticators. Challenge response `methods: ["totp","recovery_code"]` (strings) cannot carry WebAuthn request options. `sessions.two_factor_used boolean` cannot say which factor (policy "phishing-resistant only" impossible).

Seam now:

`user_auth_factors` (user-scoped, 00-accounts; replaces 3 user columns)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `fac_` |
| user_id | uuid not null references users on delete cascade | |
| type | text not null CHECK (type ~ '^[a-z][a-z0-9_]{1,39}$') | registry enum (P1). MVP registry: `totp`, `recovery_code`. Later: `webauthn`, `email_otp`. `sms`: never (00-accounts) |
| name | text not null | user label 1..60, NFC. Defaults "Authenticator app", "Recovery codes" |
| status | text not null default 'pending' CHECK in ('pending','active') | pending = enrollment unfinished; pending older than 1 h deleted by `PurgeExpiredAuthRecords` |
| secret_encrypted | bytea | TOTP seed; recovery code SHA-256 list. System data key (security.md). Null for public-key factors |
| credential | jsonb not null default '{}' | non-secret driver data, max 8 KB. totp `{algorithm,digits,period}`; recovery_code `{total,remaining}`; later webauthn `{public_key_cose,aaguid,transports,backup_eligible,backup_state,attestation_fmt}` |
| lookup_hash | bytea | SHA-256 of external credential id (WebAuthn credential id). Null at MVP. Unique where not null. Passkey login resolves user by it |
| last_counter | bigint | TOTP last accepted step (RFC 6238 5.2 replay); later WebAuthn sign count |
| is_preferred | boolean not null default false | partial unique `(user_id) WHERE is_preferred`; listed first in challenge |
| confirmed_at, last_used_at | timestamptz | |
| created_at, updated_at | timestamptz | |

Indexes: `(user_id, status)`; unique `(user_id) WHERE type = 'recovery_code' AND status = 'active'`; unique `(lookup_hash) WHERE lookup_hash IS NOT NULL`. Max active per user `auth.factors.max_per_user` (10). Per-type multiplicity from driver capabilities (MVP: one `totp`).

`users`: drop `two_factor_secret_encrypted`, `two_factor_recovery_codes_encrypted`, `two_factor_last_used_step`. Keep `two_factor_confirmed_at`, redefined: set while user has at least one active factor whose driver has `SecondFactor` capability (recovery codes excluded), cleared when last one removed. Maintained in same tx by factor actions. Cheap check for guards and `CompletePrimaryAuth`. Resource field `two_factor_enabled` unchanged.

Recovery codes: `recovery_code` factor row auto-created (10 codes) when first second factor activates; deleted when last removed. Never makes 2FA "enabled" alone.

Driver interface (`app/auth/factors`, contract in `contracts.md`):

```go
package factors

type Capabilities struct {
    SecondFactor      bool // satisfies 2FA challenge
    Primary           bool // logs in alone (passkey); false at MVP for all
    Confirmation      bool // satisfies password.confirm (sudo)
    PhishingResistant bool // webauthn true
    Multiple          bool // user may hold several
    UserVisible       bool // listed in GET /user/auth-factors (recovery_code true, shown as set)
}

type Factor interface {
    Type() string
    Capabilities() Capabilities
    // Enrollment: returns secret to store and public setup payload (otpauth URL, QR; later WebAuthn creation options).
    BeginEnrollment(ctx context.Context, u *models.User, name string) (Enrollment, error)
    CompleteEnrollment(ctx context.Context, f *models.UserAuthFactor, input json.RawMessage) (*models.UserAuthFactor, error)
    // Challenge: options sent to client per factor (nil for totp; WebAuthn request options; email_otp sends code).
    BeginChallenge(ctx context.Context, u *models.User, fs []models.UserAuthFactor) (ChallengeOptions, error)
    // Verify: constant time; returns updated counter/credential; ErrInvalidCode, ErrReplay.
    Verify(ctx context.Context, f *models.UserAuthFactor, state ChallengeState, input json.RawMessage) (VerifyResult, error)
}

func Register(f Factor)                 // in auth_service_provider.go
func Enabled() []Factor                 // filtered by config auth.factors.enabled (default [totp, recovery_code])
```

Challenge model: keep `one_time_tokens` purpose `two_factor_challenge`. `data` adds `methods` (types offered), `factor_ids`, `driver_state` (per-driver challenge state, e.g. WebAuthn nonce later). Challenge resource lists methods as objects (P6):

```json
{ "data": { "object": "two_factor_challenge", "expires_at": "2026-11-01T10:05:00Z",
  "methods": [
    { "method": "totp", "factor_id": "fac_...", "name": "Authenticator app", "preferred": true, "options": null },
    { "method": "recovery_code", "factor_id": "fac_...", "name": "Recovery codes", "preferred": false, "options": null } ] } }
```

Redeem: `POST /sessions {grant: "two_factor", method: "totp", code}` or `{grant: "two_factor", method: "recovery_code", code}`. `method` required (pre-launch, no compat). Later: `{method: "webauthn", credential: {...}}`, `options` carries WebAuthn request options. Email OTP later sends via `POST /two-factor-challenge-codes` (guest, challenge cookie).

Step-up: `POST /user/confirmation {method, code | password | credential}`; `method` in `password`, `email_code`, plus every factor type with `Confirmation` capability. Session stores `confirmed_method` (section 5). Later policy "sudo needs phishing-resistant" reads it.

API (replaces singleton; rest.md rule 6 stays valid for real singletons):

| Method | Path | Name | Auth | Purpose |
|---|---|---|---|---|
| GET | `/api/v1/user/auth-factors` | user.auth-factors.index | U | list (no secrets): `id, object: auth_factor, type, name, status, preferred, confirmed_at, last_used_at`, recovery set shows `remaining` |
| POST | `/api/v1/user/auth-factors` | user.auth-factors.store | U C | `{type: "totp", name?}` 201 `auth_factor` status `pending` + one-time `setup` (`secret`, `otpauth_url`, `qr_svg`). `{type: "recovery_code"}` regenerates set, 201 with one-time `codes` (like `plain_text_token`) |
| PATCH | `/api/v1/user/auth-factors/{fac}` | user.auth-factors.update | U | `{code}` pending to active (response carries one-time `recovery_codes` when this activation enabled 2FA); `{name}`; `{preferred: true}` |
| DELETE | `/api/v1/user/auth-factors/{fac}` | user.auth-factors.destroy | U C | remove; removing last second factor = 2FA disabled (409 `two_factor_required_by_workspace` rule kept; app grant revocation rule kept) |
| DELETE | `/api/v1/user/auth-factors` | user.auth-factors.destroy-many | U C | disable 2FA in one call (same rules) |

`GET /auth/config` adds `factors: [{type: "totp", name: "Authenticator app"}]` (enabled drivers).

Security events: `two_factor.enabled` / `two_factor.disabled` stay (account state). Add `auth_factor.added`, `auth_factor.removed`, `auth_factor.renamed`; rename `two_factor.recovery_code_used` to `auth_factor.recovery_code_used`, `two_factor.recovery_codes_regenerated` to `auth_factor.recovery_codes_regenerated`. Metadata `{factor_id, type}`.

MVP: TOTP (one) + recovery codes, same UX as today: begin, confirm with code, codes shown once. Same lockout, attempts, replay rules (step stored in `last_counter`).

Later: `webauthn` driver (go-webauthn) as second factor (adds row type, `options`), then passkey login (`Primary`): `POST /passkey-challenges` (guest) returns options, `POST /sessions {grant: "passkey", credential}` resolves user by `lookup_hash`. Email OTP factor. "Remember device" via `device_id` (section 5).

Cost: low-med (one table, 3 dropped columns, endpoint rename, 2 resources). Owner: 00-accounts; rest.md route map row; data-model.md users table.

## 2. Login methods and identity providers

Future: Microsoft, GitLab, Apple login; enterprise SSO (SAML 2.0, OIDC) per organization with verified domains; JIT provisioning; SCIM; "SSO required for @acme.com".

Breaks if MVP stays: `oauth_identities.provider CHECK in ('github','google')`, `sessions.auth_method CHECK` list, `users.created_via CHECK` list: every provider = migration on three tables. Login code assumes two hardcoded providers. No place for "this email must use SSO" before password check; adding later means touching every primary-auth path (password, magic link, email code, OAuth, reset).

Seam now:
- Registry enums (P1) for `oauth_identities.provider`, `sessions.auth_method`, `users.created_via`, `workspace_members.joined_via` (later `sso`, `scim`).
- `IdentityProvider` driver registry (`app/auth/providers`), GitHub and Google become drivers:

```go
type IdentityProvider interface {
    Key() string                       // "github", "google"; later "microsoft", "apple", per-connection SSO
    Protocol() string                  // "oauth2" | "oidc" | "saml"
    Display() ProviderDisplay          // name, icon key (for /auth/config)
    AuthorizeURL(ctx context.Context, req AuthRequest) (string, error) // caller stores state, PKCE, nonce
    Callback(ctx context.Context, r *http.Request, req AuthRequest) (ExternalIdentity, error)
}
type ExternalIdentity struct {
    Provider, Subject, Email, Name, Username, AvatarURL, HostedDomain string
    EmailVerified bool
    ConnectionID  *uuid.UUID          // SSO later; nil for social
    Groups        []string            // SSO later (role mapping)
}
type ProviderResolver interface {      // MVP: static registry from config; later adds SSO connections by id
    Find(ctx context.Context, key string) (IdentityProvider, error)
}
```
- `LoginPolicy` hook, called by every primary method before verifying secrets:

```go
type LoginPolicyResolver interface {
    ForEmail(ctx context.Context, email string) (LoginPolicy, error) // MVP driver: allow all
}
type LoginPolicy struct {
    AllowedMethods []string   // empty = all
    SSORedirectURL string     // set when domain enforces SSO
}
```
Later enforced domain: 409 `sso_required` with `redirect_url` (domain-level, so no account enumeration).
- `GET /auth/config` `providers` becomes objects: `[{key: "github", name: "GitHub", protocol: "oauth2", redirect_url: "/auth/github/redirect"}]` (P6).
- Config: `auth.oauth.<key>.*` treated as map (adding provider = driver + config block; no new code path).

MVP: GitHub, Google behind drivers. Behavior unchanged.

Later (additive):
- `sso_connections` (org-owned): `id (ssoc_), organization_id, protocol CHECK ('saml','oidc'), name, status, config jsonb (entity id, ACS, metadata URL, issuer, client id), secret_encrypted, jit_enabled, default_role, group_role_map jsonb, enforced boolean`.
- `organization_domains`: DNS TXT verified domains (`verification_token`, `verified_at`), claim users and enforce SSO. Separate from `workspace_domains` (join convenience, weak email-match proof; later: `domain-join`).
- `oauth_identities` gains `sso_connection_id uuid null`; uniques become `(provider, sso_connection_id, provider_user_id) NULLS NOT DISTINCT`, `(user_id, provider, sso_connection_id) NULLS NOT DISTINCT`. Small user-scoped table: cheap index swap.
- Web routes `/auth/sso/{ssoc}/redirect`, `/auth/sso/{ssoc}/callback`, SAML ACS `/auth/saml/{ssoc}/acs`, metadata `/auth/saml/{ssoc}/metadata` (rows added to rest.md exceptions then).
- SCIM 2.0 `/scim/v2/*` (protocol exception), `scim_tokens` per org, `users.managed_by_organization_id` (nullable column) for org-managed accounts.
- JIT: SSO login creates user + workspace memberships per connection `default_role`, `joined_via = sso`.

Cost: low. Owner: 00-accounts.

## 3. Organizations (B2B, enterprise)

Decision: YES. Add thin `organizations` now, one per workspace, created inside `CreateWorkspace`, invisible in MVP API.

Why:
- Enterprise features are company-level, not workspace-level: SSO connections, verified domains, SCIM, session policies, consolidated billing, audit export across workspaces, CMK, contract and entitlements. Company with prod + test workspaces, or 2 products, needs one place.
- Users already global (span workspaces). That was the expensive Slack-Grid mistake; avoided. Remaining retrofit cost is owner references: billing account, policies, keys. Point them at org from day one and later work is "let org hold 2 workspaces".
- Cost tiny: one table, one FK column, one line in `CreateWorkspace`, one join in billing resolve.

Why not more (rejected now): `organization_members`, org roles, org API, org switcher, moving workspaces between orgs. MVP org admin = owners of its workspace(s) (derived). Later `organization_members(organization_id, user_id, role CHECK ('owner','admin','billing','member'))` additive.

Seam now:

`organizations` (global, not tenant; foundation schema, 00-accounts owner)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `org_` |
| name | text not null | copy of first workspace name; renamed with workspace while 1:1 |
| created_by_user_id | uuid null references users on delete set null | |
| settings | jsonb not null default '{}' | org-scope settings (section 7). Empty at MVP |
| deleted_at | timestamptz | set when last workspace purged; row kept while billing retention needs it |
| created_at, updated_at | timestamptz | |

`workspaces.organization_id uuid not null references organizations` (no cascade), index `(organization_id)`. `CreateWorkspace`: insert org, then workspace, same tx; always new org at MVP (later `POST /workspaces {organization_id}` for org admins). `PurgeWorkspace`: soft-delete org when it has no live workspace.

Attach to org now (all 1:1 so MVP behavior identical):
- Billing account owner (section 6, 12).
- `encryption_keys` scope (section 8).
- Entitlement overrides (section 6).
- Settings inheritance level (section 7).

Stay on workspace: members, roles, inboxes, all content, `require_two_factor` setting (later org can enforce same key, section 7), audit_logs. (`workspace_domains` join later: `domain-join`.)

API: none in MVP (no `organization_id` in resources; additive later). Not tenant table: no RLS; reached only by `workspace -> organization` join in actions.

Later: org settings page, org members, add workspace to org, org-level audit (union of workspace audit + member `security_events`), `sso_connections`, `organization_domains`, `scim_tokens`, org policies, consolidated invoice, data residency choice at org creation.

Cost: low. Owner: data-model.md (schema), 00-accounts (create/purge), 12 (billing owner).

## 4. Roles and permissions

Future: custom roles (post-MVP: upmarket set era), granular permissions (billing manager, read-only analyst, agent without delete), team-scoped or inbox-scoped access ("only see Billing inbox").

Breaks if MVP stays: policies written as `role == admin` across 13 specs; mobile app gates UI on role names; token ceilings table keyed by role. Custom roles later = audit every policy + every client.

Decision: keep `workspace_members.role` CHECK owner/admin/member (and `invitations.role`). No `roles` table now (rejected below). Add permission layer in code:
- `app/auth/permissions.go`: permission catalog. Strings in ability format `resource:action`. MVP catalog = ability catalog (`api.md`) + extras: owner-only `workspace:delete`, `members:manage_owners`, `tokens:manage_any`, `billing:cap_unrestricted`; owner + admin `billing:read`, `billing:write`, `audit_logs:read` (grouped under `settings:read` ability for tokens). Each permission belongs to exactly one token ability group (`conversations:write` group contains future `conversations:assign`, `conversations:delete`...). Finer permissions later join a group, so existing tokens keep meaning.
- `app/auth/roles.go`: system role to permission set. Only file allowed to name roles for authorization.
- Policies call `authz.Can(principal, perm)`. Lint/review: no `Role ==` outside `roles.go` and role-change actions (owner transfer, last-owner checks stay role-based by nature).
- Token effective permissions = role permissions intersect expanded abilities (replaces "ability ceilings by role" table; same results).
- `/me` adds `member.permissions` (effective list for caller in workspace). Dashboard and apps gate UI on it (P2).
- OAuth scopes (09) are ability strings already: keep one vocabulary.
- Conversation visibility: every conversation list, search, count, realtime channel authorizer and export goes through one builder `conversations.VisibleScope(principal)` (03). MVP returns "all in workspace". Later inbox or team restriction = change one function + `inbox_members` table.

Later: `roles` table `(id rol_, workspace_id or organization_id, key, name, permissions text[], system boolean)`, `workspace_members.custom_role_id uuid null`; `role` stays base role (ceiling, owner semantics). API member `role` stays, adds `custom_role {id, name}`. Additive.

Cost: low. Owner: 00-accounts (catalog, roles.go, `/me`), api.md (vocabulary), 03 (`VisibleScope`).

## 5. Sessions, devices, multi-device

Future: native apps on many devices, desktop app, per-device revoke (session + grant + push in one click), "remember this device", org session policies (max session age, idle timeout, IP allowlist, require 2FA, allowed login methods, SSO only), step-up, DPoP.

Already good: browser sessions and first-party grants listed together in `GET /sessions`; multi-device login works (many sessions, many grants); push rows die with session or grant (06 already has `kind` web/apns/fcm + `oauth_grant_id`).

Breaks if MVP stays:
- `sessions.two_factor_used boolean` loses which methods (policy "passkey only" impossible). No `authenticated_at` on sessions, yet 09 copies "session `authenticated_at`" into `oauth_authorization_requests.auth_time` (gap: column missing in 00-accounts).
- No stable device identity across session, grant, push: "revoke this phone" cannot find its push rows or old grants; "remember device" needs it.
- `two_factor.enforced` middleware hardcodes one policy; IP allowlist, session age, SSO-only later = new middleware per rule, each with own error shape.

Seam now:

`sessions` changes:
| Column | Change |
|---|---|
| auth_method | registry enum (P1); primary method only |
| two_factor_used | replace with `auth_methods text[] not null default '{}'` (every method passed: `{password,totp}`, `{github}`, later `{passkey}`). Resource keeps `two_factor_used` (derived) and adds `auth_methods` |
| authenticated_at | new, `timestamptz not null default now()`: last full login incl. factors; reset by re-login (`prompt=login`, `max_age`). 09 reads it |
| confirmed_method | new, `text`: method used for last sudo confirmation |
| device_id | new, `text`: SHA-256 hex of `__Host-dimmi_device` cookie value (already set by `StartSession`) |

`oauth_grants` (09) changes: add `auth_methods text[] not null default '{}'` (copied from session at approval) and `device_id text` (new optional authorize param `device_id`: app install id, random UUID kept in keychain/keystore; max 64 chars). `auth_time` already there.

`push_subscriptions` (06): add `device_id text` (app sends install id; browser: same hashed device cookie). Rest already applied by 06.

No `user_devices` table now (rejected). Later `user_devices (user_id, device_id, name, platform, trusted_until, last_seen_at)` is backfillable from these columns.

Access policy (replaces `two_factor.enforced`, same position in `W` stack):

```go
type Principal struct {
    UserID          uuid.UUID
    AuthMethods     []string
    AuthenticatedAt time.Time
    DeviceID        string
    IP              netip.Addr
    Via             contracts.ActorVia
}
type PolicyFailure struct {
    Code        string // two_factor_required | reauthentication_required | sso_required | ip_not_allowed
    Requirement string // two_factor | reauthentication | sso | ip
    ActionURL   string // setup or re-login URL
    Source      string // workspace | organization
}
type AccessPolicy interface {
    Evaluate(ctx context.Context, p Principal, workspaceID uuid.UUID) (*PolicyFailure, error)
}
```
MVP rule set: one rule, `require_two_factor` (workspace setting). Error body for any failure: 403 `{code, message, requirement, action_url}`. Existing code `two_factor_required` kept. Clients branch on `requirement` generically. Evaluated with cached principal (no extra query).

Later rules (new rule structs, no middleware change): max session age (`authenticated_at` older than org limit: `reauthentication_required`, app re-auths with `max_age`), idle limit, IP allowlist (CIDR list on org), allowed methods / SSO only (`auth_methods` must contain `sso`), phishing-resistant MFA (factor capability lookup), managed device.

Cost: low. Owner: 00-accounts (sessions, middleware), 09 (grants, authorize param), 06 (`device_id`).

## 6. Entitlements, plans, feature flags

Future: enterprise contract features (custom retention, audit streaming, SLA, data residency), staff raising caps for one customer, staged rollouts, rate limits by contract. Pricing promise: one plan, every feature, never paywalled. So entitlements = limits + contract extras + rollouts, not feature paywalls.

Breaks if MVP stays: cloud caps scattered as `if edition == cloud` branches and config keys across specs (help centers 3 in 08, custom Discord bots 3 in 05, email daily caps in 04, invitations per day, tokens per member, workspaces per user, API rate limits). Raising one cap for one customer = code deploy or global config change.

Seam now:

```go
// app/contracts/entitlements.go; *App.Entitlements
type Entitlements interface {
    Has(ctx context.Context, workspaceID uuid.UUID, feature string) (bool, error)
    Limit(ctx context.Context, workspaceID uuid.UUID, key string) (int64, error) // -1 = unlimited
}
```
- Catalog `app/support/entitlements/catalog.go`: key, kind (feature|limit), default self-host, default cloud, owner spec. Unknown key panics at boot (registry). Every cloud cap in every spec moves to catalog key; call site reads `Entitlements.Limit`.
- Drivers: `self_host` (features all true, limits from config, overrides none); `cloud` (catalog cloud default, then `billing_accounts.program/contract`, then `entitlement_overrides`). Cached 60 s per workspace, busted on override change.
- Table `entitlement_overrides` (12, cloud):

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `ent_` |
| organization_id | uuid not null references organizations on delete cascade | |
| workspace_id | uuid null references workspaces on delete cascade | narrower than org when set |
| key | text not null | catalog key (validated in app) |
| value | jsonb not null | bool or integer |
| reason | text not null | staff note |
| granted_by_user_id | uuid null references users on delete set null | staff |
| expires_at | timestamptz | null = permanent |
| created_at, updated_at | timestamptz | |
Unique `(organization_id, workspace_id, key) NULLS NOT DISTINCT`. Staff-only via `dimmi entitlement:set` command (cloud). Audit logged.
- API: `GET /api/v1/workspace/entitlements` (A W any member; singleton): `{object: "entitlements", features: {key: bool}, limits: {key: {limit, used}}}` (`used` where cheap). Dashboard shows "2 of 3 help centers" without hardcoding.
- Feature rollout flags: same catalog, kind `feature`, cloud default false, enabled per org by override. No third-party flag service.
- Full MVP key list + shared over-limit error `entitlement_limit_reached`: contracts.md "Entitlements" (incl. `ai.actions.*` (07), `docs_sync.*` (08), `changelog.*` (13), `feedback.*` (14), `engage.*` (15), `imports.*` (16)).

MVP: catalog with existing caps, two drivers, overrides table, endpoint.

Later: contract entitlements set from `billing_accounts.contract`, rate limits per contract (`api.rate_limit_per_minute`), enterprise toggles (SSO enforcement, audit streaming, retention policies).

Cost: low. Owner: contracts.md (interface), 12 (table, cloud driver), each spec (moves its caps to keys).

## 7. Settings

Future: org-level settings that apply to all workspaces and may lock them (require 2FA everywhere, retention), per-user preferences synced across devices, instance defaults for self-host.

Today: `workspaces.settings` jsonb typed `models.WorkspaceSettings` (keys from 00-accounts, 09), `inboxes.settings`, per-feature tables (widget_settings, knowledge_settings, notification_settings), `system_settings` with key registry (11), user prefs as columns. Fine storage; missing: one declaration of keys with scopes and inheritance.

Seam now: generalize 11 registry into foundation `app/support/settings`:

```go
type Scope string // instance | organization | workspace | user
type Definition struct {
    Key         string   // "security.require_two_factor"
    Type        any      // Go type for decode
    Default     any
    Scopes      []Scope  // where it may be set
    Enforceable bool     // higher scope may lock it (org lock wins over workspace)
    Secret      bool
    Validate    func(any) error
}
func Get[T any](ctx context.Context, key string, at ScopeRef) (T, error) // resolves user -> workspace -> organization -> instance -> default
```
Storage stays: instance = `system_settings`; organization = `organizations.settings`; workspace = `workspaces.settings`; user = existing columns/tables. Rule: scalar policy and preference = registry key in jsonb; anything with identity, lifecycle or many rows = own table (widget installs, help centers, destinations).

MVP: registry declares existing workspace keys (`require_two_factor`, `previous_slugs`, `agent_access`) and 11 instance keys. No org keys used yet.

Later: org admin sets `security.require_two_factor` with lock; workspace PATCH of locked key returns 422 `setting_locked`. `users.preferences jsonb` added when needed.

Cost: low. Owner: foundation (registry), 11 (moves registry), 00-accounts (declares keys).

## 8. Audit and compliance

Future: audit streaming to SIEM (Splunk, Datadog, S3), org-wide audit, retention policies per org, legal hold, data residency (EU), customer-managed keys (CMK, BYOK KMS).

Breaks if MVP stays:
- Purge and erase code is spread across specs (workspace purge, contact erase with 9 erasers, spam purge 30 d, raw `.eml` 30 d, audit prune, data retention auto-erase, attachment cleanup). Legal hold later = edit every job; a missed one destroys held data.
- KeyManager "data key per workspace" has no storage spec (gap) and no scope beyond workspace/system: CMK per org needs KEK reference per scope.
- Audit API cursor only descending; SIEM pull needs stable ascending tail.
- `audit_logs.via_kind CHECK` lacks `app` though `contracts.Actor.Via.Kind` has it (gap; registry enum fixes class of bug).

Seam now:
- `RetentionGuard` contract, noop driver at MVP (always allows):

```go
type RetentionSubject struct { Kind string; ID uuid.UUID } // workspace | contact | conversation | audit_log | inbound_email | attachment
type RetentionGuard interface {
    MayDestroy(ctx context.Context, workspaceID uuid.UUID, s RetentionSubject) (bool, error)
}
```
Every purge/erase/prune job calls it before destroying; false = skip and record `retention_held` (erase request stays `pending_hold`). Feature test per job with fake guard returning false.
- `encryption_keys` (foundation, global): one row per (scope, purpose) holding a Tink keyset wrapped by the KEK (`framework/encryption` `KeysetStore`), not one row per data key. Columns, keys and rules: data-model.md "encryption_keys". Ciphertext carries Tink key-id prefix, so decrypt never guesses scope; rotation (new primary inside keyset) and KEK switch (`RewrapAll`) transparent; MAC keys never rotated automatically.
`KeyManager.Encrypt(ctx, encryption.Scope{Kind, ID}, plaintext, aad)`. MVP scopes: system, workspace. KEK drivers `local`, `awskms` (config `encryption.keks.<name>.driver`; later `gcpkms`, `vault`). Later CMK: org-scoped customer KEK, keysets re-wrapped under it (no data re-encryption).
- `audit_logs`: `via_kind` and `actor_type` registry enums; allow `sort=created_at` (ascending) with cursor and `filter[created_after]` on `GET /audit-logs` so pull-based SIEM export works now.
- `AuditSink` contract (noop): `Write(ctx, []AuditEntry) error`, called by queued listener after audit insert. Later drivers: webhook (10 signing), S3 JSONL, Datadog, Splunk HEC.
- Data residency: cell model. Each region = separate deployment + database (`app.region` config, e.g. `us`, `eu`). No region column. Rules now: no hardcoded hostnames in clients (CLI, MCP, mobile, snippets take base URL; 09 already does), `/auth/config` and `/me.instance` expose `region`, UUIDv7 ids globally unique so workspace can move between cells by export/import later.

MVP: noop guard and sink, `encryption_keys` with workspace/system scopes, ascending audit cursor, `app.region` exposed.

Later: `legal_holds (id, organization_id, workspace_id, subject, reason, created_by, released_at)`, `retention_policies` per org (settings keys), audit sinks per org, org audit view (adds nullable `organization_id` to `audit_logs`; drop NOT NULL on `workspace_id` is metadata-only), EU cell + region picker at signup + global directory for login routing.

Cost: low. Owner: foundation (contracts, `encryption_keys`, security.md), 00-accounts (audit API), every purge-owning spec (calls guard).

## 9. Multi-brand, multi-product

Future: one company, several products: each with own help center, widget look, sender addresses, agent persona, CSAT branding.

Assessment: inbox = routing (who handles), not presentation. Workspace already allows many widgets, many help centers (cloud 3), many email addresses, per-address display name. Missing only a grouping entity. Adding it later: `brands (id brd_, workspace_id, name, logo_asset, colors, default_locale)` + nullable `brand_id` on `channel_connections`, `help_centers`, `ai_agents`; conversation brand derives from `channel_connection_id`. All additive, nullable, no data move.

Decision: no seam now. Rejected (YAGNI). Rule only: presentation fields (sender display name, colors) stay on channel/help center rows, never on `workspaces` beyond defaults.

## 10. Users vs members vs contacts

Future: customer portal (contacts log in, see own tickets, company tickets for B2B), contacts who are also members, contacts in many companies (B2B), service accounts.

Assessment:
- Contacts must never become `users` (different tenancy: contacts are per workspace, users global; different threat model). Contact portal sessions implemented by 14 (`portal_sessions` with `scopes`, `portal_sign_ins`; email link/code, widget hand-off, SSO via identity token) on help center host. Later support portal reuses them (new scope), no new auth model.
- Contact also member: `members.FindActiveMemberByVerifiedEmail` already used for internal detection. Fine.
- Contacts in many companies: Intercom supports it. Later `contact_companies (workspace_id, contact_id, company_id, primary)` + resource `companies[]` additive. Seam now (doc-only): `contacts.company_id` documented as "primary company"; JWT `company` claim maps to primary; never assume one company in metering or webhooks.
- `contact_identities.type` registry enum (P1): biggest tenant table touched by every new channel (05 already had to add `whatsapp_bsuid`; SMS, Instagram, Teams, portal next).
- Service accounts / app bot identity: tokens tied to member (00-accounts open question 1). Later `personal_access_tokens.owner_id` nullable + `service_account_id` (small table, metadata-only change). `Actor.Type = integration` already reserved for installed apps. No seam now.

Cost: none-low. Owner: data-model.md (contact_identities), 03 (doc), 05 (registers type in registry instead of CHECK migration).

## 11. API and integrations

Future: OAuth app marketplace (third-party apps installed per workspace with bot identity, own webhooks), date-based API versions, per-contract rate limits.

Assessment: `oauth_clients` global with kinds `system`, `dynamic`, `metadata_document`; grants per user per workspace. Marketplace later adds `kind = 'registered'` (small table CHECK change), `oauth_clients.owner_organization_id`, `app_installations (workspace_id, oauth_client_id, installed_by_member_id, scopes, status)` with `Actor{Type: integration, ID: installation}`, `webhook_endpoints.app_installation_id null`. All additive.

Seam now:
- One vocabulary: OAuth scopes, token abilities, permission groups = same strings (section 4). Marketplace apps request the same strings.
- API version header reserved: every `/api/v1` response sends `Dimmi-Version: 2026-11-01`. Request may send `Dimmi-Version`; equal to current or absent = OK; unknown = 400 `invalid_api_version`. SDKs and CLI send it from day one. Later date versions add response transformers keyed by version (Stripe model) without SDK change.
- `webhook_endpoints.api_version text not null default '2026-11-01'` (10). Payload snapshot built for endpoint's version. Later shape change never breaks existing endpoints.
- Rate limits read from `Entitlements.Limit` keys (`api.rate_limit_per_minute`, `api.user_rate_limit_per_minute`) instead of direct config.

Cost: low. Owner: api.md, 10, 09 (scopes doc).

## 12. Mobile

Already specced: guards `auth:app`, system client `dimmi-mobile`, claimed links, PKCE, grants in session list, push kinds reserved (06).

Seam now:
- Client version gate: first-party apps and CLI send `Dimmi-Client-Version: <platform>/<semver>` (`ios/1.4.0`, `android/1.4.0`, `cli/0.9.0`, later `desktop/...`). Middleware `client.version` on `/api/v1` and `/oauth/token`: when `apps.min_versions.<platform>` set and header lower: 426 `client_upgrade_required` `{min_version, upgrade_url}`. Header missing (browsers, third parties) = allowed. Config empty at MVP = noop. Cost: one middleware.
- Capability discovery (P7): `GET /api/v1/auth/config` adds `server_version`, `api_version`, `region`, `capabilities: ["auth_factors", "push_web", ...]` (string tokens from registry), `min_client_versions: {ios, android, cli}`. Newer app against older self-host hides missing features instead of crashing. App reads before login.
- Tombstone rule (zero cost): resources apps will sync (conversations, messages, contacts, notifications, tags) keep soft delete (`deleted_at`) or emit delete event; every change bumps `updated_at`. Hard delete only for GDPR erase and purge. Keeps later delta sync additive.
- Push payload rule (06): payload carries ids (`ntf_`, `conv_`, `ws_`) + optional title/body; app fetches details; never more than APNs 4 KB; content level becomes user preference later.
- `device_id` on grants and push rows (section 5).

Later (additive): `filter[updated_since]` + `sort=updated_at` + `filter[trashed]=with` on synced lists (concurrent index adds), background refresh, notification content preference, DPoP-bound tokens, desktop system client.

Cost: low. Owner: api.md, 00-accounts (`/auth/config`), 09 (token endpoint gate), 06 (payload rule).

## 13. Localization, timezones, multi-currency

Assessment: users, workspaces, contacts, help centers already carry locale and timezone. Errors return stable `code` (translatable client-side). Translations of articles (post-MVP: auto-translation) belong to 08 later (`article_translations` or locale on article): no core seam.

Seam now (12 only): `billing_accounts.currency text not null default 'usd'` (Stripe fixes customer currency after first invoice; must be explicit per account). All `*_cents` columns documented as minor units of account currency. Price book versions keyed by `(version, currency)`; MVP only `usd`.

Cost: low. Owner: 12.

## 14. IDs and tenancy

Assessment: UUIDv7 + prefixed public ids, global uniqueness, `workspace_id` leading indexes: right for cells, export/move, org reads.

Seam now:
- RLS policies written once through function: `USING (workspace_id = ANY (dimmi_visible_workspaces()))`, where `dimmi_visible_workspaces()` (SQL, STABLE) returns `ARRAY[current_setting('app.workspace_id')::uuid]`. Later org-level reads (org audit, consolidated billing, reports) set `app.workspace_ids` and function returns that list: one function change instead of rewriting every policy. Btree index handles `= ANY(array)`. Perf test: RLS overhead under 5% on list endpoints.
- `team_members` lacks `workspace_id` (no RLS, breaks P8): add `workspace_id uuid not null`, PK stays `(team_id, workspace_member_id)`.
- Lint (security.md tenant check) extended: table with a FK to a tenant table must have `workspace_id` unless annotated `-- lint:user-scoped`, `-- lint:org-scoped` (12 account tables; RLS via `dimmi_visible_billing_accounts()`) or `-- lint:global`.
- Organization table global (not tenant); org reads via joins in actions or SECURITY DEFINER functions, same pattern as 00-accounts lookups.

Cost: low. Owner: foundation (security.md, tasks), 00-accounts (`team_members`).

## Change list per spec

Applied in final sweep 2026-11-02 (foundation, conventions); feature folders applied by owners. Kept as record. Pre-launch: no compat shims.

### 00-conventions/README.md
- Database rules: add "Registry enums: driver-backed enums use `text` + format CHECK (`^[a-z][a-z0-9_]{1,39}$`; dotted event names `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`) + validation against Go registry in request rule and action. State machines, statuses, visibility keep strict CHECK. Column list: `00-foundation/extensibility.md` change list, 00-conventions section."
- Authorization rules: "Policies call `authz.Can(principal, permission)`. Role names appear only in `app/auth/roles.go` and role-change actions."
- Lint: tenant FK rule (section 14).

Registry enum columns (initial list):
| Table.column | Owner |
|---|---|
| user_auth_factors.type | 00-accounts |
| sessions.auth_method | 00-accounts |
| users.created_via | 00-accounts |
| oauth_identities.provider | 00-accounts |
| security_events.type (dotted) | 00-accounts |
| workspace_members.joined_via | 00-accounts |
| audit_logs.actor_type, audit_logs.via_kind | 00-accounts |
| contact_identities.type | foundation, 05 registers types |
| channel_connections.type | foundation, channel specs register |
| oauth_grants.revoked_reason | 09 |
| notifications.trigger | 06 |
| ai_provider_keys.provider | 07 |
| inbound_emails.source | 04 |
| knowledge_sources.type | 08 |
| contact_identities.type adds `portal_visitor` | 14 (registers) |
| ai_action_connections.auth_type | 07 |
| github_connections.kind | 08 |
| portal_sessions.grant | 14 |
| imports.source | 16 |
(Canonical list: `00-conventions/README.md` "Database rules".)

### 00-foundation/data-model.md
- Ownership map: add `organizations`, `user_auth_factors` (00-accounts); `encryption_keys` (foundation infra); `entitlement_overrides` (12).
- Accounts: new `organizations` table (section 3). `workspaces` add `organization_id uuid not null references organizations`, index.
- `users`: remove `two_factor_secret_encrypted`, `two_factor_recovery_codes_encrypted`; `two_factor_confirmed_at` note "set while at least one active second factor (`user_auth_factors`)".
- `workspace_members`: note "role key; permissions from `roles.go`".
- `team_members`: add `workspace_id`.
- `contact_identities.type`, `channel_connections.type`: registry enums. Remove "05 adds CHECK value" row from core changes table; 05 registers `whatsapp_bsuid` in Go registry.
- `contacts.company_id`: note "primary company".
- Core table changes table: users row drops `two_factor_last_used_step`.
- Infrastructure tables: add `encryption_keys` (section 8).
- Prefix registry: add `org_` organizations, `fac_` user_auth_factors, `ent_` entitlement_overrides.

### 00-foundation/contracts.md
Add sections (Go code `app/contracts/`):
- `Entitlements` (section 6).
- `RetentionGuard` (section 8), consumers list: 00-accounts `PurgeWorkspace`, `PruneAuditLogs`; 03 `EraseContact`, spam purge; 04 raw `.eml` prune; 01 attachment cleanup; any data retention job.
- `AuditSink` (section 8).
- `AccessPolicy` + `Principal` + `PolicyFailure` (section 5).
- `KeyManager` with `encryption.Scope` (section 8).
- Pointers (interfaces owned by 00-accounts in `app/auth`): `factors.Factor`, `providers.IdentityProvider`, `LoginPolicyResolver`, `authz.Can`.
- Workspace initializer hook: note org row created by `CreateWorkspace` before initializers; 12 initializer creates billing account per org if absent.
- Actor: `Via.Kind` list is a registry; audit table must accept every value (`app` missing today).

### 00-foundation/security.md
- Tenancy: RLS via `dimmi_visible_workspaces()` (section 14); lint FK rule.
- Secrets: `encryption_keys` keyset storage, `encryption.Scope`, Tink key-id prefix in ciphertext; scopes system/workspace now, organization later.
- Passwords and sessions: replace "TOTP 2FA + recovery codes" with "auth factors (`user_auth_factors`, driver registry; MVP totp + recovery codes)"; sessions record `auth_methods`, `authenticated_at`, `confirmed_method`, `device_id`; workspace policies through `AccessPolicy`.
- First-party apps: authorize param `device_id`; `Dimmi-Client-Version` + 426 gate.
- Privacy: every purge/erase calls `RetentionGuard`.
- Audit log: ascending cursor; `AuditSink` noop.

### 00-foundation/api.md
- Who calls what: note `two_factor.enforced` renamed `access.policy` (same position).
- Auth and abilities: "ability = permission group; permission catalog in `app/auth/permissions.go`; OAuth scopes use same strings". `/me` returns `member.permissions`.
- Headers: `Dimmi-Version` (echo + validate), `Dimmi-Client-Version` (426 `client_upgrade_required`).
- Errors: 403 policy body `{code, message, requirement, action_url}`; 426 `client_upgrade_required`; 400 `invalid_api_version`; 409 `sso_required` (reserved, later); 422 `setting_locked` (reserved, later).
- Rate limits: defaults are entitlement keys.
- Health and meta: `/auth/config` capability discovery fields.

### 00-foundation/rest.md
- Rule 6 singleton examples: remove `/user/two-factor` (now collection `/user/auth-factors`); add `/workspace/entitlements`.
- Rule 13: `two_factor` grant body `{method, code}`; later grants `passkey`, `sso` (registry).
- Route map 00-accounts: remove row `POST /user/two-factor/confirm -> PATCH /user/two-factor`; add rows:
  | Old | New |
  |---|---|
  | POST /user/two-factor | POST /user/auth-factors `{type: "totp"}` |
  | POST /user/two-factor/confirm | PATCH /user/auth-factors/{fac} `{code}` |
  | POST /user/two-factor/recovery-codes | POST /user/auth-factors `{type: "recovery_code"}` |
  | DELETE /user/two-factor | DELETE /user/auth-factors (all) or DELETE /user/auth-factors/{fac} |
  | POST /auth/two-factor-challenge `{code \| recovery_code}` | POST /sessions `{grant: "two_factor", method, code}` |

### 00-foundation/README.md
- Files table: add `extensibility.md` row ("seams for post-launch growth: factors, orgs, permissions, policies, entitlements, settings, retention, keys").
- Decisions: add "9. Org layer exists from day one (1:1, hidden); enterprise features attach to org."

### 00-foundation/tasks.md
Add: `encryption_keys` + `encryption.Scope`; RLS function `dimmi_visible_workspaces()`; contracts `Entitlements`, `RetentionGuard`, `AuditSink`, `AccessPolicy` with noop/MVP drivers; settings registry with scopes; `Dimmi-Version` and `client.version` middleware; registry enum lint.

### 00-accounts/README.md
- Scope In: "Auth factors: TOTP + recovery codes via factor driver registry". Out: passkeys row points to `webauthn` driver + `lookup_hash`; SAML/SCIM row points to org seams (extensibility.md section 2).
- Migrations: add `create_organizations_table` (before workspaces, renumber in range), `create_user_auth_factors_table`; `add_account_columns_to_users_table` drops `two_factor_last_used_step`; `create_workspaces_table` includes `organization_id`; `create_teams_table` adds `team_members.workspace_id`.
- Tables: `user_auth_factors` (section 1); `sessions`: `auth_method` registry enum, replace `two_factor_used` with `auth_methods text[]`, add `authenticated_at`, `confirmed_method`, `device_id`; `oauth_identities.provider` registry enum; `security_events.type` registry enum + new types (section 1); `audit_logs.via_kind`/`actor_type` registry enums (fixes missing `app`); `users.created_via`, `workspace_members.joined_via` registry enums.
- Code map: controllers `two_factor_controller.go`, `two_factor_recovery_code_controller.go` replaced by `auth_factor_controller.go` (Index, Store, Update, Destroy, DestroyMany); resources `two_factor_setup_resource.go`, `recovery_codes_resource.go` replaced by `auth_factor_resource.go`, `new_auth_factor_resource.go` (one-time `setup` / `codes` / `recovery_codes`); actions `begin_two_factor_setup`, `confirm_two_factor`, `disable_two_factor`, `regenerate_recovery_codes` replaced by `add_auth_factor.go`, `activate_auth_factor.go`, `remove_auth_factor.go`, `remove_all_auth_factors.go`, `regenerate_recovery_codes.go` (uses factor driver); `complete_two_factor_challenge.go` dispatches to driver by `method`. Support: `app/auth/factors/` (registry, `totp.go`, `recovery_code.go`), `app/auth/providers/` (registry, `github.go`, `google.go`), `permissions.go`, `roles.go`, `login_policy.go` (allow-all driver). Middleware `enforce_two_factor.go` replaced by `enforce_access_policy.go` (`access.policy`). Actions `CreateWorkspace` creates org; `PurgeWorkspace` soft-deletes org, calls `RetentionGuard`.
- Events/listeners: unchanged; security event types renamed.
- Config: `auth.factors.enabled` (`[totp, recovery_code]`), `auth.factors.max_per_user` (10); `auth.oauth.<key>.*` documented as map.
- Permissions matrix: column header "Token ability" means ability group; add note "implemented as permission catalog; table derived from `roles.go`". Ability ceilings table replaced by "role permissions intersect ability groups" (same outcomes).
- Open question 1 (service accounts): add pointer to extensibility.md section 10.

### 00-accounts/api.md
- Endpoint table: replace 4 `/user/two-factor*` rows with 5 `/user/auth-factors` rows (section 1).
- `POST /sessions` row: `two_factor` grant body `{method, code}`.
- Challenge example: `methods` array of objects (section 1).
- `POST /user/confirmation`: `method` accepts any factor type with `Confirmation` capability.
- `/me` example: add `member.permissions`; `auth` object: replace `two_factor_used` with `auth_methods` (keep `two_factor_used` derived).
- `/auth/config` example: `providers` objects, `factors`, `server_version`, `api_version`, `region`, `capabilities`, `min_client_versions`.
- Sessions list example: add `auth_methods`, `device_id` not exposed (internal), keep `two_factor_used`.
- Errors: 403 policy body fields `requirement`, `action_url`; add 426 `client_upgrade_required`.
- CLI `user:disable-2fa` = `RemoveAllAuthFactors` (actor system via cli).
- Mail templates: `TwoFactorEnabledMail`, `TwoFactorDisabledMail` unchanged; add `AuthFactorAddedMail` (second factor added while 2FA already on).

### 00-accounts/flows.md
- 0 "Primary auth then 2FA": challenge `data.methods` from active factors via drivers; `LoginPolicyResolver.ForEmail` called first in every primary method (MVP allow-all).
- 4 2FA challenge: dispatch by `method`; TOTP step stored in factor `last_counter`; success sets session `auth_methods = {primary, method}`, `authenticated_at = now()`.
- 8 Sudo: records `confirmed_method`.
- 10 TOTP setup: rewrite on `/user/auth-factors` (begin = POST, confirm = PATCH, regenerate = POST type recovery_code, disable = DELETE).
- 11 Require 2FA: enforced by `AccessPolicy` rule `two_factor`.
- 19 Domain join: deferred (later: `domain-join`); DNS-verified `organization_domains` later for claim/SSO (separate).
- StartSession: store `device_id` (hash of device cookie), `auth_methods`, `authenticated_at`.

### 00-accounts/edge-cases.md, tasks.md
- Edge cases touching 2FA columns or `/user/two-factor` paths: rewrite against factors (same behavior). Add: second TOTP while one active (422 `factor_limit_reached` at MVP), remove recovery_code factor directly (409 `recovery_codes_required` while 2FA on), pending factor expiry, org row exists for every workspace (repair command recreates).
- Tasks: factor registry, providers registry, permissions catalog, access policy middleware, organizations migration.

### 03-shared-inbox
- `conversations.VisibleScope(principal)` single builder used by list, search, counts, exports, realtime `conversation:` channel authorizer.
- `EraseContact`, spam purge call `RetentionGuard`.
- `contacts.company_id` = primary company (doc).

### 05-chat-app-channels
- `contact_identities.type` new values registered in Go registry (no CHECK migration); `channel_connections.type` same.

### 06-notifications
- `push_subscriptions`: add `device_id text`. (`kind`, `oauth_grant_id`, nullable web columns already applied.)
- Push payload rule: ids + optional title/body, under 4 KB.
- `notifications.trigger` registry enum.

### 09-agent-native
- `oauth_grants`: add `auth_methods text[] not null default '{}'`, `device_id text`; `revoked_reason` registry enum.
- `/oauth/authorize` first-party extras: optional `device_id` param (max 64 chars).
- Fix gap: session field is `sessions.authenticated_at` (now defined by 00-accounts).
- Scopes = ability strings = permission groups (one vocabulary note).
- `/oauth/token`: `client.version` gate (426 as OAuth error `{"error":"invalid_request","error_description":"client_upgrade_required"}` because token endpoint uses OAuth error shape).
- CLI sends `Dimmi-Client-Version: cli/<ver>` and `Dimmi-Version`.

### 10-webhooks
- `webhook_endpoints.api_version text not null default '2026-11-01'`; snapshot built per endpoint version.
- `AuditSink` webhook driver later (note only).

### 11-self-host
- Settings registry moves to foundation `app/support/settings` with scopes; 11 registers instance keys.
- `/auth/config` and `/me.instance` expose `region` (self-host default `self`), `server_version`.
- Entitlements `self_host` driver: all features true, limits from config.

### 12-cloud-billing
- `billing_accounts`: replace `workspace_id uuid unique not null` with `organization_id uuid not null` (no FK, retention; unique index at MVP, droppable later for cost centers); add `currency text not null default 'usd'`.
- `billing_periods`, `spend_caps`, `credit_grants`, `invoices_cache`, `billing_adjustments`: key by `billing_account_id` instead of `workspace_id` (unique `(billing_account_id, period)`); free allowance, tiers, cap computed per billing account.
- `usage_ledger`, `billing_windows`, `billing_disputes`: keep `workspace_id` (attribution), add `billing_account_id`.
- Resolve path: workspace -> organization -> billing account (cached 60 s). API paths unchanged (`/billing/*` workspace-scoped, resolves account).
- Initializer: create billing account for org if absent.
- New table `entitlement_overrides` + cloud `Entitlements` driver + `dimmi entitlement:set` command.
- `_cents` columns documented as minor units of account currency.
- Remove "Consolidated billing across workspaces" from Out list wording to "later: second workspace in same org shares account (no schema change)".

### Other feature specs (01, 04, 07, 08)
- Move cloud caps to entitlement catalog keys (08 help centers 3, 04 email daily caps, 05 custom bots 3, 07 anonymous AI budget percent if per-contract).
- Purge/prune jobs call `RetentionGuard` (04 raw `.eml`, 01 attachment cleanup, 08 search logs prune).

### New public id prefixes
`org_` organizations, `fac_` user_auth_factors, `ent_` entitlement_overrides. Reserved for later (do not reuse): `ssoc_` sso_connections, `odom_` organization_domains, `rol_` roles, `brd_` brands, `lgh_` legal_holds, `ains_` app_installations, `dev_` user_devices.

### New config keys
| Key | Default | Owner |
|---|---|---|
| `auth.factors.enabled` | `[totp, recovery_code]` | 00-accounts |
| `auth.factors.max_per_user` | 10 | 00-accounts |
| `apps.min_versions.ios` / `.android` / `.cli` | empty (no gate) | 09 |
| `apps.upgrade_urls.ios` / `.android` / `.cli` | App Store / Play Store / npm URLs (empty = none in body) | 09 |
| `app.region` | `self` (self-host), `us` (cloud) | foundation |
| `api.version` | `2026-11-01` | foundation |

## Rejected seams (YAGNI, with reason)

| Considered | Rejected because | Later path |
|---|---|---|
| `roles` table + `member.role_id` now | 3 roles cover MVP; permission layer (section 4) gives same extensibility without seeding, joins, or role CRUD | additive `roles` + `custom_role_id` |
| `organization_members`, org roles, org API now | no MVP user needs it; derived owners suffice 1:1 | additive table |
| `organization_id` on every tenant table | denormalization tax on every insert; joins via workspace cheap; RLS function covers reads | none needed |
| Composite PKs `(workspace_id, id)` for Citus | single Postgres fine for years; cells scale out first | revisit if one cell outgrows |
| Postgres enum types or lookup tables for enums | enum types painful (conventions); lookup tables add joins and seeds while behavior lives in Go drivers | registry enums (P1) |
| Generic EAV `settings` table | jsonb per scope row already typed; EAV loses types and indexes | registry over jsonb |
| `user_devices` table now | `device_id` columns enough to backfill later | additive table |
| `brands` table now | inbox, widgets, help centers, addresses already plural; brand = grouping only | additive table + nullable FKs |
| `sso_connection_id` on `oauth_identities` now | FK target does not exist; small table, cheap index swap later | section 2 |
| `region` column on org/workspace | regions are separate cells; column would lie inside a single-region DB | `app.region` + directory later |
| `organization_domains` with DNS now | domain join (later: `domain-join`) uses email proof; DNS needed only for SSO/claim | additive table |
| Many-companies-per-contact join table now | primary company covers launch audience; additive later | `contact_companies` |
| Contact portal accounts in `users` | wrong tenancy and threat model | contact sessions (01 pattern) |
| Service accounts now | open question in 00-accounts; dedicated "CI" member workaround | nullable member on tokens |
| Change-feed table for mobile delta sync | `updated_at` + soft delete + events suffice; feed table costs writes on every change | `filter[updated_since]` |
| Stripe-style version transformers now | nothing to transform before first breaking change | header already reserved |
| Multi-account browser sessions (account switcher) | rare for audience; app holds several grants already | cookie per account later |
| SMS factor | SIM swap (00-accounts decision) | never |
| DPoP, device-bound tokens | needs app support; grants already short-lived | 09 post-MVP |
| Per-org KMS now | KEK chosen per keyset wrap; org-scoped KEK = config + `RewrapAll` later | CMK driver |
| Third-party feature flag service | entitlement catalog + overrides covers rollouts | none |
| Data model for article translations | 08 owns; not core | 08 later |

## Open questions (recommended default in bold)

1. Second workspace created by same user: new org (**yes at MVP**) or same org? Same org would merge billing (one free allowance, combined tiers): product call, defer to org UI launch.
2. Show `organization_id` in workspace resource now? **No** (additive later; avoids clients guessing semantics).
3. `GET /workspace/entitlements` visible to members, or admins only? **Members** (limits not sensitive; money fields stay in billing endpoints).
4. Move existing `require_two_factor` key name to `security.require_two_factor` now for registry naming? **Yes** (pre-launch rename is free).
