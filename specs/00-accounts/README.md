# 00 Accounts: auth, users, workspaces, members, teams, tokens

Style: caveman. Date: 2026-11-01. Scope trim 2026-11-02: domain join deferred (later: `domain-join`, audit B16); invitations only. Updated 2026-11-02: extensibility seams applied (`00-foundation/extensibility.md` sections 1-5, 7, 8, 12, 14: auth factors, identity providers, organizations, permissions, session auth context, access policy, capability discovery). Final sweep 2026-11-02: framework migration (`00-conventions/framework.md` section 10), CSRF via `http.CrossOriginProtection`, realtime kick via centrifuge, permissions + audit names for 13-15; consistency pass 2026-11-02: permissions + audit names for 07 agent actions (`ai:actions`), 08 GitHub/docs sync, 16 importers; testing policy (integration only for app logic). Backend only. Read with `specs/00-conventions/README.md` and `specs/00-foundation/*`.

## Files

| File | What |
|---|---|
| README.md | this: goal, scope, data model, code map, events, jobs, config, permissions, budgets |
| [flows.md](flows.md) | step-by-step behavior of every auth and lifecycle flow (signup, login, OAuth, magic link, 2FA, reset, email change, first run, invitations, workspace delete, account delete) |
| [api.md](api.md) | HTTP endpoint table, request/response examples, error codes, rate limits, MCP tools, CLI commands, mail templates |
| [edge-cases.md](edge-cases.md) | numbered decided behaviors |
| [tasks.md](tasks.md) | build checklist |

## Goal

Every person reaches inbox safely in under 60 seconds: sign up (password, magic link, GitHub, Google), create or join workspace, invite teammates, mint API token. Self-host gets same flows plus first-run wizard and `dimmi user:create`. Owns identity, membership, authorization and audit for every other spec.

## Why

Not roadmap item; prerequisite of all 12 launch must-haves (`research/roadmap.md`). Directly serves: #3 "Unlimited teammates, simple roles", #9 "Non-expiring scoped tokens", #11 one-command self-host (first run), #12 cloud ("SSO and audit log included", unlimited seats). Fin CLI tokens expire about 14 days (`roadmap.md` Why now); Dimmi tokens never expire unless user sets expiry.

## World-class bar

| Bar | Target |
|---|---|
| Signup to first workspace (password or OAuth) | 3 HTTP calls, under 60 s human time, zero email wait (verification async, non-blocking) |
| Login API p95 (password, argon2id) | under 250 ms server time |
| Auth middleware overhead (session or token, warm cache) | p95 under 1 ms |
| Revocation (session, token, member deactivation) effective on every replica | under 2 s; sockets kicked under 5 s |
| Auth mail (magic link, reset, verify, invitation) enqueued | under 50 ms after request; mail job priority 1 |
| Magic link email contains link AND 6-digit code | works cross-device, immune to link scanners (token in URL fragment, consumed only by POST) |
| Account enumeration | zero on login, magic link, password reset (same status, body, timing within 10 ms p50) |
| Lockout | real user never locked out by attacker on their known device; attacker capped at 10 guesses/min per IP+email, escalating |
| 2FA | factor driver registry; MVP TOTP RFC 6238 + 10 recovery codes; passkeys later = new driver, zero migration; workspace "require 2FA" enforced on session, app AND token through `AccessPolicy` |
| Auth context | every session and first-party grant records `auth_methods`, `authenticated_at`, `device_id` (step-up, org policies, SSO enforcement read it later; cannot be backfilled) |
| Tokens | Sanctum abilities, shown once, prefix visible, optional expiry, `last_used_at` within 60 s accuracy, revoke effective under 2 s |
| Self-host first run | `docker compose up` then browser wizard secured by setup token printed in logs, or `dimmi user:create`; no mail server needed |
| Audit | every member/role/token/settings/security change recorded with actor, IP, UA, redacted diff; CSV export |
| Test coverage | every endpoint feature-tested incl. cross-workspace 404, ability scope, rate limit, enumeration timing |

## Scope

### In (MVP)

- Signup: email + password, magic link (link + code), GitHub OAuth, Google OIDC. Email verification (async, non-blocking except gated actions).
- Login, logout, logout other sessions, password confirmation ("sudo mode", 15 min), password reset, password change. Reauth mode (`reauth`) forcing full login on existing session (09 `prompt=login` / `max_age=0` sudo for apps).
- Sessions list and revoke (`/sessions`): browser sessions and first-party app grants (Dimmi mobile, 09 `ListUserAppGrants`) in one list. New-device sign-in mail. Sessions record `auth_methods`, `authenticated_at`, `confirmed_method`, `device_id`.
- Auth factors: TOTP + recovery codes via factor driver registry (`user_auth_factors`, `app/auth/factors`; TOTP math in `framework/auth/twofactor` on pquerna/otp); REST collection `/user/auth-factors`; 2FA challenge `methods` as objects; workspace setting `security.require_two_factor` enforced by `AccessPolicy` middleware (`access.policy`).
- Identity providers: GitHub, Google as `IdentityProvider` drivers (registry, config map `auth.oauth.<key>.*`). `LoginPolicyResolver` hook called by every primary method (MVP driver allow-all).
- OAuth identity link/unlink in settings, automatic link on verified email match.
- Organizations: thin `organizations` row, 1:1 with workspace, created inside `CreateWorkspace`, soft-deleted by `PurgeWorkspace`. No org API, no `organization_id` in resources (extensibility.md section 3).
- Permissions: permission catalog + `roles.go` (only place mapping role to permissions); policies call `authz.Can(principal, perm)`; token abilities = permission groups; `/me` returns `member.permissions`.
- Capability discovery: `GET /auth/config` returns providers, factors (arrays of objects), `server_version`, `api_version`, `region`, `capabilities`, `min_client_versions`.
- Email change with re-verification (new address) and notice (old address).
- Brute-force protection: per IP+email limiter, escalating lockout, per-email soft lock with known-device bypass, per-IP spray detection.
- First run on self-host: setup token, `POST /api/v1/setup`, `dimmi user:create`; open signup off after first user.
- Workspaces: create, update, delete (30-day grace, restore), purge, leave, switch (header + last accessed), suspend (ops).
- Members: roles owner/admin/member, role change, deactivate (with reassignment hint), reactivate, transfer ownership, last-owner protection, availability online/away.
- Teams CRUD and membership.
- Invitations: bulk email invite, 7-day expiry, resend (rotates token), revoke, accept (existing user and new user), copyable link when mail not configured.
- Personal access tokens: abilities from `api.md`, prefix, shown once, optional expiry, `last_used_at`, revoke, admin revoke.
- `GET /api/v1/me`, `/api/v1/user/*` account endpoints (`auth:user`: session or first-party app), account deletion.
- REST shape per `00-foundation/rest.md`: sessions created by `POST /sessions` with `grant`, secret links redeemed by `PATCH /{collection}/{id} {token}`, member state via `PATCH /members/{mem}`.
- Audit log table, writer, API (`GET /api/v1/audit-logs`, CSV). User security events log.
- Cloud: disposable email block, signup velocity limits, optional Turnstile captcha, breached password check, workspace suspension.
- Mail templates (list in [api.md](api.md#mail-templates)).
- CLI: `user:*`, `workspace:*`, `token:create` (`setup:link` owned by 11).

### Out (later)

| Item | When |
|---|---|
| SAML SSO, enterprise OIDC, SCIM provisioning, verified org domains, JIT | post-MVP: SSO + SCIM (upmarket set); cloud pricing promises free when shipped. Attach to `organizations` (`sso_connections`, `organization_domains`, `scim_tokens`), new `IdentityProvider` drivers per connection, `LoginPolicyResolver` driver enforcing SSO (409 `sso_required`). extensibility.md section 2 |
| Passkeys / WebAuthn (go-webauthn) | post-MVP. `webauthn` factor driver (second factor first, then `Primary` passkey login: `POST /passkey-challenges`, grant `passkey`, user resolved by `user_auth_factors.lookup_hash`). No migration |
| Email OTP as second factor | later: `email_otp` factor driver, `POST /two-factor-challenge-codes` (guest, challenge cookie) |
| "Remember this device" for 2FA | later: `user_devices` backfilled from `sessions.device_id` / grant `device_id`; sessions last 30 days so 2FA once per session |
| Organization API, org members, org settings, second workspace in same org | later, additive (`organization_members`, `POST /workspaces {organization_id}`) |
| Service accounts (tokens not tied to member) | later; see Open questions |
| Custom roles / granular permissions | post-MVP: custom roles (upmarket set): `roles` table + `workspace_members.custom_role_id` additive; permission layer already in place |
| Microsoft, GitLab, Apple login | later: new `IdentityProvider` driver + config block, same `oauth_identities` table (provider is registry enum) |
| SMS 2FA | never (SIM swap); no `sms` driver |
| Custom invitation message | later (spam vector on cloud) |
| OAuth server, device flow, OAuth tokens `dmi_oat_`, first-party app login (browser sheet + PKCE) and grants | 09-agent-native (this spec provides session login used by consent screen, `auth:user` guard group, and lists/revokes first-party grants in `/sessions`) |
| Notification preferences | 06-notifications |
| Billing role extras, invoices | 12-cloud-billing |
| SMTP configuration UI, system settings | 11-self-host |
| `domain-join` (audit B16): workspace email domains, joinable-workspaces list, self-join by domain, signup exception for domain match | later; invitations only at MVP |

## User stories

Workspace member
- As solo dev, I sign up with GitHub and land in new workspace in one click after consent.
- As teammate, I click invite email, set name and password (email pre-verified by invite), land in inbox.
- As member, I see my sessions (browsers and Dimmi app installs) and revoke lost laptop or phone. I add an authenticator app as a factor, name it, and save recovery codes.
- As member, I set myself away so widget shows team offline when I am last one online.
- As member in several workspaces, I switch between them; dashboard remembers last used.

Owner / admin
- As owner, I invite 10 people at once, assign roles and teams, resend expired invites, revoke wrong ones.
- As admin, I deactivate leaver; their sockets close, tokens stop, open conversations unassigned.
- As owner, I transfer ownership before leaving company. I cannot leave if I am last owner.
- As owner, I require 2FA for workspace; members without 2FA must set up on next request.
- As admin, I read audit log filtered by actor and export CSV for compliance.
- As owner, I delete workspace by mistake and restore within 30 days.

Developer / operator
- As developer, I create token with `conversations:read` only, see it once, see last used time, set 90-day expiry.
- As self-host operator, I run `docker compose up`, open URL printed in logs, create admin account, done. Or `dimmi user:create` in CI.
- As self-host operator without SMTP, I still log in (password) and invite people (copy invite link); magic links and reset links appear in server log.
- As self-host operator locked out of 2FA, I run `dimmi user:disable-2fa`.

AI agent (via token or OAuth from 09)
- As agent, I call `GET /api/v1/me` to learn workspace, my abilities, effective permissions, and timezone before acting.

Dimmi mobile app (first-party, 09)
- As app, I read `GET /auth/config` before login to learn providers, factors, capabilities and minimum client version; I hide features the server lacks.
- As app, I gate UI on `member.permissions`, never on role names (old builds keep working when custom roles ship).
- As agent, I list members and teams to assign conversations (`dimmi_list_members`, `dimmi_list_teams`).

## Data model

Core tables `workspaces`, `users`, `workspace_members`, `teams`, `team_members` keep schema from `00-foundation/data-model.md`; `organizations` schema from `00-foundation/extensibility.md` section 3 (data-model.md once foundation applies change list). This spec writes their migrations (owner per ownership map) exactly as there, then adds columns under "Core table changes". New public ID prefixes listed at end.

Registry enums (P1, `00-conventions` database rules): column `text` + format CHECK `^[a-z][a-z0-9_]{1,39}$` (dotted names: `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`) + validation against Go registry in request rule and action. New value = new Go driver/registration, zero migration. Owned here: `user_auth_factors.type`, `sessions.auth_method`, `users.created_via`, `oauth_identities.provider`, `security_events.type` (dotted), `workspace_members.joined_via`, `audit_logs.actor_type`, `audit_logs.via_kind`. State machines and statuses keep strict CHECK (`role`, member `status`, invitation `status`, factor `status`, `one_time_tokens.purpose`).

### Migrations (order; range 20261101000001-20261101000029 reserved for 00-accounts (widened 2026-11-02; foundation core 000030-000099), all before foundation core migrations since every tenant table references `workspaces`; 000020-000029 free)

| File | Content |
|---|---|
| `20261101000001_create_users_table.sql` | users per data-model (no 2FA secret columns; `two_factor_confirmed_at` kept) |
| `20261101000002_create_organizations_table.sql` | organizations (global, not tenant, no RLS) |
| `20261101000003_create_workspaces_table.sql` | workspaces per data-model incl. `organization_id uuid not null references organizations`, index `(organization_id)` |
| `20261101000004_create_workspace_members_table.sql` | per data-model |
| `20261101000005_create_teams_table.sql` | teams + team_members per data-model, plus 00-accounts changes: `teams.deleted_at`, unique `(workspace_id, lower(name)) WHERE deleted_at IS NULL`, `team_members.workspace_id uuid not null references workspaces on delete cascade` (P8; PK stays `(team_id, workspace_member_id)`) + RLS |
| `20261101000006_add_account_columns_to_sessions_table.sql` | ALTER framework `sessions` (framework set creates it, `framework/session`): product columns below |
| `20261101000007_create_user_auth_factors_table.sql` | new |
| `20261101000008_create_oauth_identities_table.sql` | new |
| `20261101000009_create_one_time_tokens_table.sql` | new |
| `20261101000010_create_password_reset_tokens_table.sql` | new |
| `20261101000011_create_invitations_table.sql` | new |
| `20261101000012_add_account_columns_to_personal_access_tokens_table.sql` | ALTER framework `personal_access_tokens` (framework set creates it, `framework/auth/token`): product columns + RLS below |
| `20261101000013_create_audit_logs_table.sql` | new |
| `20261101000014_create_security_events_table.sql` | new |
| `20261101000016_add_account_columns_to_users_table.sql` | core change (000015 freed: `workspace_domains` deferred, `domain-join`) |
| `20261101000017_add_lifecycle_columns_to_workspaces_table.sql` | core change |
| `20261101000018_add_access_columns_to_workspace_members_table.sql` | core change |
| `20261101000019_create_auth_lookup_functions.sql` | SECURITY DEFINER lookup functions (see RLS note) |

Teams `deleted_at` folded into `create_teams_table` (was separate `add_deleted_at_to_teams_table`): pre-launch, nothing deployed, one file per table is clearer.

### New tables

#### organizations (global; not tenant, no RLS)
Enterprise anchor (extensibility.md section 3, P4). One per workspace at MVP, invisible in API. Billing account (12), `encryption_keys` org scope, entitlement overrides (12), org-scope settings attach here.
| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `org_` (reserved; not exposed in MVP) |
| name | text not null | copy of first workspace name; renamed with workspace while 1:1 |
| created_by_user_id | uuid null references users on delete set null | |
| settings | jsonb not null default '{}' | org-scope settings (foundation settings registry). Empty at MVP |
| deleted_at | timestamptz | set by `PurgeWorkspace` when org has no live workspace; row kept while billing retention needs it |
| created_at, updated_at | timestamptz | |
Reached only through `workspace -> organization` join in actions or SECURITY DEFINER functions. Lint annotation `-- lint:global`.

#### sessions (not tenant table; user-scoped)
Table created by framework migration set (`framework/session`; columns `id`, `token_hash`, `user_id`, `data`, `flash`, `ip` text, `user_agent`, `authenticated_at`, `confirmed_at`, `last_activity_at`, `expires_at`, `created_at`; indexes `(user_id, last_activity_at desc)`, `(expires_at)`). 00-accounts migration ALTERs it, never CREATE: adds `auth_method`, `auth_methods`, `device_label`, `device_id`, `country`, `confirmed_method`, `updated_at`, FK `user_id references users on delete cascade` + `NOT NULL` (framework allows null for guests; Dimmi stores only logged-in sessions), index `(user_id, device_id)`. Full shape after ALTER:
| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `ses_` |
| token_hash | bytea unique not null | SHA-256 of cookie value (32 random bytes, base64url). Raw value never stored |
| user_id | uuid not null references users on delete cascade | |
| auth_method | text not null CHECK (format) | registry enum (P1). Primary method only. MVP registry: `password`, `magic_link`, `email_code`, `password_reset`, `setup`, `invitation`, plus every `IdentityProvider` key (`github`, `google`). Later `passkey`, `sso` |
| auth_methods | text[] not null default '{}' | every method passed at last full login: `{password,totp}`, `{github}`, `{password,recovery_code}`. Replaces `two_factor_used`. Resource keeps `two_factor_used` (derived: contains any second-factor method) |
| authenticated_at | timestamptz not null default now() | last full login incl. factors. Set by `StartSession` and reauth; never by confirmation, rotation or activity. 09 copies into `oauth_authorization_requests.auth_time` / grant `auth_time`; `max_age` compares it |
| ip | text (framework) | last seen IP |
| user_agent | text | truncated 512 chars |
| device_label | text | parsed, e.g. "Chrome on macOS" |
| device_id | text | SHA-256 hex of `__Host-dimmi_device` cookie value (set by `StartSession`). Internal: never in resources. Joins session, push rows (06) and later `user_devices` |
| country | text | ISO 3166-1 alpha-2 from optional GeoIP, null otherwise |
| confirmed_at | timestamptz | last sudo confirmation (password, factor, email code) or full login |
| confirmed_method | text | method of last confirmation (`password`, `email_code`, factor type e.g. `totp`; full login: last method of `auth_methods`). Later policy "sudo needs phishing-resistant" reads it |
| last_activity_at | timestamptz not null default now() | written at most once per 5 min per session |
| expires_at | timestamptz not null | created_at + `auth.sessions.absolute_lifetime` (30 d) |
| created_at, updated_at | timestamptz | |
Indexes: `(user_id, last_activity_at desc)`, `(expires_at)`, `(user_id, device_id)`. Idle expiry: `last_activity_at < now() - 14 days` treated expired.

#### user_auth_factors (user-scoped; `-- lint:user-scoped`)
Replaces `users.two_factor_secret_encrypted`, `two_factor_recovery_codes_encrypted`, `two_factor_last_used_step`. Behavior per driver (Support: factor drivers below).
| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `fac_` |
| user_id | uuid not null references users on delete cascade | |
| type | text not null CHECK (type ~ '^[a-z][a-z0-9_]{1,39}$') | registry enum. MVP: `totp`, `recovery_code`. Later `webauthn`, `email_otp`. `sms` never |
| name | text not null | user label 1..60 chars, NFC, control chars stripped. Defaults: totp "Authenticator app", recovery_code "Recovery codes" |
| status | text not null default 'pending' CHECK in ('pending','active') | pending = enrollment unfinished; pending older than 1 h deleted by `PurgeExpiredAuthRecords` |
| secret_encrypted | bytea (`encryption.Sealed`) | totp: 20-byte seed; recovery_code: list of SHA-256 code hashes with used flags. `encryption.Seal` with `encryption.KeyManager` system scope (`encryption.Scope{Kind: "system"}`), AAD = factor id. Null for public-key factors (later) |
| credential | jsonb not null default '{}' | non-secret driver data, max 8 KB. totp `{algorithm: "SHA1", digits: 6, period: 30}`; recovery_code `{total: 10, remaining: 10}`; later webauthn `{public_key_cose, aaguid, transports, backup_eligible, backup_state, attestation_fmt}` |
| lookup_hash | bytea | SHA-256 of external credential id (WebAuthn). Null at MVP. Passkey login resolves user by it |
| last_counter | bigint | totp: last accepted step (RFC 6238 5.2 replay); later WebAuthn sign count |
| is_preferred | boolean not null default false | listed first in challenge |
| confirmed_at | timestamptz | activation time |
| last_used_at | timestamptz | last successful challenge or confirmation |
| created_at, updated_at | timestamptz | |
Indexes: `(user_id, status)`; partial unique `(user_id) WHERE is_preferred`; unique `(user_id) WHERE type = 'recovery_code' AND status = 'active'`; unique `(lookup_hash) WHERE lookup_hash IS NOT NULL`. Max active per user `auth.factors.max_per_user` (10). Per-type multiplicity from driver `Multiple` capability (MVP: one `totp`, one `recovery_code` set).

Rules (maintained in same tx by factor actions):
- `users.two_factor_confirmed_at` set while user has at least one active factor whose driver `EnablesTwoFactor` (= `SecondFactor` and not `Backup`; recovery codes never enable 2FA alone). Cleared when last one removed. Cheap check for guards, `AccessPolicy` and `CompletePrimaryAuth`. Resource field `two_factor_enabled` = `two_factor_confirmed_at IS NOT NULL`.
- `recovery_code` row auto-created (10 codes) when first 2FA-enabling factor activates; deleted when last one removed.

#### oauth_identities (user-scoped)
| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `idn_` |
| user_id | uuid not null references users on delete cascade | |
| provider | text not null CHECK (format) | registry enum = `IdentityProvider.Key()` (`github`, `google`; later `microsoft`, `apple`, SSO) |
| provider_user_id | text not null | GitHub numeric id as string; Google `sub` |
| email | citext | provider email at last login (display only, never identity key) |
| email_verified | boolean not null default false | |
| username | text | GitHub login |
| avatar_url | text | provider URL, copied to own storage on signup |
| hosted_domain | text | Google `hd` claim |
| last_login_at | timestamptz | |
| created_at, updated_at | timestamptz | |
Unique `(provider, provider_user_id)`, unique `(user_id, provider)` (one account per provider per user). No provider access/refresh tokens stored (not needed after login; data minimization).

#### one_time_tokens (user/email-scoped)
Short-lived single-use secrets: magic links, 2FA login challenge, email verification, email change, confirmation codes.
| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `ott_` (exposed only for `email_verification`, `email_change` redemption: `PATCH /email-verifications/{ott}`, `PATCH /email-changes/{ott}`) |
| purpose | text not null CHECK in ('magic_link','two_factor_challenge','email_verification','email_change','confirm_code') | |
| user_id | uuid null references users on delete cascade | null for magic-link signup of unknown email |
| email | citext not null | target address (`email_verification`: user email at send time; mismatch with current email = stale) |
| token_hash | bytea unique not null | SHA-256 of 32-byte random token |
| code_hash | bytea | HMAC-SHA256(app key, token id + 6-digit code); null when purpose has no code |
| attempts | smallint not null default 0 | code/2FA/wrong-token attempts; burned at 5 |
| data | jsonb not null default '{}' | `new_email`, `invitation_id`, `redirect`, `session_intent`, `name`. `two_factor_challenge`: `primary_method`, `methods` (types offered), `factor_ids`, `driver_state` (per-driver challenge state; later WebAuthn nonce), `reauth_session_id` (reauth mode), `device_id` |
| ip | inet, user_agent text | requester |
| expires_at | timestamptz not null | magic_link 15 min, two_factor_challenge 5 min, email_verification 24 h, email_change 24 h, confirm_code 10 min |
| consumed_at | timestamptz | set once; consume = `UPDATE ... WHERE consumed_at IS NULL AND expires_at > now() RETURNING` |
| created_at | timestamptz | |
Indexes: `(email, purpose, created_at desc)`, `(user_id, purpose)`, `(expires_at)`.

#### password_reset_tokens (user-scoped, one active per user)
| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `prs_`; link carries id + token; new id on every request |
| user_id | uuid unique not null references users on delete cascade | one row per user: new request upserts (old link and id die) |
| token_hash | bytea unique not null | |
| requested_ip | inet | |
| created_at | timestamptz not null | |
| expires_at | timestamptz not null | created_at + 60 min |
| attempts | smallint not null default 0 | wrong-token tries; row deleted at 5 |
Row deleted on successful reset. Redeem: `PATCH /password-resets/{prs} {token, password}`; lookup by id, constant-time hash compare; unknown id compares against dummy hash (same timing).

#### invitations (tenant)
| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `inv_` |
| workspace_id | uuid not null references workspaces on delete cascade | |
| email | citext not null | NFC + lowercase domain, validated |
| role | text not null CHECK in ('admin','member') | owner never invited directly |
| team_ids | uuid[] not null default '{}' | applied on accept; missing teams skipped |
| invited_by_member_id | uuid null references workspace_members on delete set null | |
| token_hash | bytea unique not null | rotated on resend |
| status | text not null CHECK in ('pending','accepted','revoked','expired') default 'pending' | `expired` set lazily on read or by prune job |
| expires_at | timestamptz not null | 7 days from last send |
| last_sent_at | timestamptz not null | |
| send_count | smallint not null default 1 | |
| accepted_at | timestamptz, accepted_by_user_id uuid null references users on delete set null | |
| revoked_at | timestamptz, revoked_by_member_id uuid null | |
| created_at, updated_at | timestamptz | |
Unique `(workspace_id, email) WHERE status = 'pending'`. Indexes `(workspace_id, status, created_at desc)`, `(email) WHERE status = 'pending'` (show pending invites to logged-in user).

#### personal_access_tokens (tenant)
Table created by framework migration set (`framework/auth/token`: `id`, `owner_id`, `tenant_id` text, `name`, `hint`, `token_hash`, `abilities`, `expires_at`, `last_used_at`, `last_used_ip` text, `revoked_at`, `created_at`; index `(owner_id, created_at desc)`). Framework fields map: `owner_id` = workspace member id, `tenant_id` = workspace id (text), `hint` = token prefix. 00-accounts migration ALTERs it, never CREATE: adds `workspace_id`, FKs, `expiry_notified_at`, `revoked_by_member_id`, `updated_at`, tenant indexes, RLS. Full shape after ALTER:
| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `tok_` |
| workspace_id | uuid not null references workspaces on delete cascade | |
| owner_id | uuid not null references workspace_members on delete cascade (FK added by ALTER) | framework column = workspace member owning token; token acts as this member |
| name | text not null | 1 to 80 chars |
| hint | text not null | framework column: prefix + first chars, e.g. `dmi_live_Ab12` |
| token_hash | bytea unique not null | SHA-256 of full token |
| abilities | text[] not null | subset of ability catalog, or `{*}` |
| expires_at | timestamptz | null = never |
| last_used_at | timestamptz, last_used_ip text | coalesced writes (`token.Options.TouchInterval`, 60 s) |
| expiry_notified_at | timestamptz | 7-day warning sent |
| revoked_at | timestamptz, revoked_by_member_id uuid null | revoked rows kept 90 days for audit display, then deleted |
| created_at, updated_at | timestamptz | |
Indexes: `(workspace_id, owner_id, created_at desc)`, `(workspace_id, created_at desc) WHERE revoked_at IS NULL`, `(expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL`.

Token format: `dmi_live_` (or `dmi_test_` for `test_mode` workspaces) + 32 chars base62 random (190 bits) + 6 chars base62 CRC32 of preceding part. Total 47 chars. Checksum lets guard reject typos without DB hit and enables future secret-scanning partner registration.

#### audit_logs (tenant, append-only)
| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `aud_` |
| workspace_id | uuid not null references workspaces on delete cascade | |
| event | text not null | dotted, e.g. `member.role_changed`, `token.created`, `channel.connected` |
| actor_type | text not null CHECK (format) | registry enum = `contracts.Actor.Type` (00-foundation/contracts.md): `member`, `contact`, `agent`, `system`, `integration`. Token and OAuth requests act as owning member, never own type |
| actor_id | uuid | member id or ai_agent id; null for system |
| via_kind | text CHECK (format) | registry enum = `contracts.Actor.Via.Kind`: `session`, `app` (first-party grant, 09), `personal_token`, `oauth_client`, `widget`, `channel`, `cli`, `mcp`. Every value contracts defines is accepted (old strict CHECK lacked `app`) |
| via_id | uuid | session id, first-party grant id, personal token id, oauth client id; null for cli |
| actor_label | text not null | snapshot "Ada Lovelace <ada@acme.com>" or "Ada Lovelace via Token CI (dmi_live_Ab12)" |
| subject_type | text | `member`, `team`, `invitation`, `token`, `workspace`, `channel_connection`, ... |
| subject_id | uuid | |
| subject_label | text | snapshot |
| changes | jsonb not null default '{}' | `{"role":{"from":"member","to":"admin"}}`; secrets redacted to `"[redacted]"` |
| ip | inet | |
| user_agent | text | 512 chars |
| request_id | text | |
| created_at | timestamptz not null default now() | no `updated_at` (append-only, conventions exception) |
Indexes: `(workspace_id, created_at desc, id desc)`, `(workspace_id, event, created_at desc)`, `(workspace_id, actor_id, created_at desc)`, `(workspace_id, via_kind, via_id, created_at desc)`, `(workspace_id, subject_type, subject_id, created_at desc)`.
Migration revokes `UPDATE` on table from app role. `DELETE` only by `PruneAuditLogs` job (cloud retention 365 days; self-host unlimited by default), which asks `RetentionGuard.MayDestroy` (subject `audit_log`) per batch; held: skip, record `retention_held`. After insert, queued listener forwards entries to `AuditSink` (noop at MVP; SIEM drivers later).

#### security_events (user-scoped)
Account-level security log visible to user only (login has no workspace).
| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `sev_` |
| user_id | uuid not null references users on delete cascade | |
| type | text not null CHECK (dotted format) | registry enum. MVP: `login.succeeded`, `login.failed`, `login.locked`, `logout`, `password.changed`, `password.reset`, `two_factor.enabled`, `two_factor.disabled` (account state), `auth_factor.added`, `auth_factor.removed`, `auth_factor.renamed`, `auth_factor.recovery_code_used`, `auth_factor.recovery_codes_regenerated`, `email.change_requested`, `email.changed`, `identity.linked`, `identity.unlinked`, `session.revoked`, `sessions.revoked_others`, `magic_link.used`, `token.created`, `account.deletion_requested` |
| ip | inet, user_agent text, device_label text, country text | |
| metadata | jsonb not null default '{}' | method(s), session id, provider, failure reason, `reauth`; factor events `{factor_id, type}` (no secrets) |
| created_at | timestamptz | |
Index `(user_id, created_at desc)`. Pruned after 365 days (both editions). `login.failed` written only for existing users, max 1 row per user+IP per minute.

### Core table changes

`users` (000001): data-model columns minus `two_factor_secret_encrypted`, `two_factor_recovery_codes_encrypted` (moved to `user_auth_factors`). `two_factor_confirmed_at` kept, redefined (see `user_auth_factors` rules).

`20261101000016_add_account_columns_to_users_table.sql`
| Column | Type | Why |
|---|---|---|
| deleted_at | timestamptz | account deletion anonymizes row (FKs from members, messages stay valid) |
| password_changed_at | timestamptz | display, session invalidation reasoning, reset mails |
| risk_flags | text[] not null default '{}' | cloud: `disposable_email`, `signup_velocity`, `breached_password_reuse` |
| created_via | text CHECK (format) | registry enum: `register`, `magic_link`, `invitation`, `setup`, `cli`, plus every `IdentityProvider` key (`github`, `google`). Later `sso`, `scim` |
Index `(email) WHERE deleted_at IS NULL` not needed (email unique already); anonymized email `deleted+<uuid>@users.invalid` keeps unique constraint. (`two_factor_last_used_step` dropped: TOTP step lives in factor `last_counter`.)

`workspaces` (000003): `organization_id uuid not null references organizations` (no cascade; org outlives purge for billing retention), index `(organization_id)`. Not in workspace resource (additive later).

`20261101000017_add_lifecycle_columns_to_workspaces_table.sql`
| Column | Type | Why |
|---|---|---|
| created_by_user_id | uuid null references users on delete set null | limits, support |
| deleted_by_user_id | uuid null references users on delete set null | restore authorization, reminder mail |
| purge_after | timestamptz | `deleted_at + 30 days`; index `(purge_after) WHERE deleted_at IS NOT NULL` |
| suspended_reason | text | ops note, shown to owners |
`settings` keys owned here, declared in foundation settings registry (`app/support/settings`, scope `workspace`; typed in `models.WorkspaceSettings`): `security.require_two_factor bool` (default false; `Enforceable`: org may lock later, 422 `setting_locked`), `previous_slugs [{slug, until}]`. Stored nested in jsonb (`{"security": {"require_two_factor": true}}`). API field on workspace resource stays `require_two_factor` (maps to key).

`20261101000018_add_access_columns_to_workspace_members_table.sql`
| Column | Type | Why |
|---|---|---|
| last_accessed_at | timestamptz | workspace switcher order and default workspace; written once per 5 min |
| deactivated_at | timestamptz | |
| deactivated_by_member_id | uuid null references workspace_members on delete set null | |
| joined_via | text CHECK (format) | registry enum: `created`, `invitation`, `setup`, `cli`. Later `domain`, `sso`, `scim` |
Index `(user_id, last_accessed_at desc)` (no `workspace_id` lead: queried only through SECURITY DEFINER function by user; documented exception to tenant index rule). `role` stays strict CHECK owner/admin/member: role key only; permissions come from `roles.go`.

`teams` / `team_members` (000005): `teams.deleted_at` (team delete is two-phase: hide, unassign conversations in chunks (03), then purge); unique `(workspace_id, lower(name)) WHERE deleted_at IS NULL` (name reusable right after delete); `team_members.workspace_id` (tenant rule P8, RLS, lint FK rule). `AddTeamMember` sets it from team.

### RLS and cross-tenant lookups

Tenant tables here (`invitations`, `personal_access_tokens`, `audit_logs`, `workspace_members`, `teams`, `team_members`) use foundation RLS form `USING (workspace_id = ANY (dimmi_visible_workspaces()))`. User-scoped (`sessions`, `user_auth_factors`, `oauth_identities`, `one_time_tokens`, `password_reset_tokens`, `security_events`) and global (`organizations`) tables annotated for lint.

Some lookups happen before tenant known. They run through `SECURITY DEFINER` functions (migration 000019), owned by migration role, `search_path` pinned, returning only minimal columns:
- `auth_find_token(hash bytea)` returns token id, workspace_id, member id, abilities, expires_at, revoked_at.
- `auth_find_invitation(hash bytea)` returns invitation id, workspace_id, email, status, expires_at.
- `auth_user_memberships(user_id uuid)` returns member ids, workspace ids, names, slugs, roles, statuses, last_accessed_at, deleted_at, purge_after for `GET /user/memberships`.
- `auth_pending_invitations_for_email(email citext)`.
- `auth_organization_for_workspace(workspace_id uuid)` returns org id, name, deleted_at (billing resolve 12, settings inheritance).
After lookup, request sets `app.workspace_id` and uses normal tenant queries.

### Public ID prefixes (new)

`mem_` workspace_members, `inv_` invitations, `ses_` sessions, `aud_` audit_logs, `idn_` oauth_identities, `sev_` security_events, `ott_` one_time_tokens (email verification, email change redemption), `prs_` password_reset_tokens, `fac_` user_auth_factors, `org_` organizations (registered, not exposed in MVP). Existing used: `usr_`, `ws_`, `team_`, `tok_`. Read from 09: `ogr_` (first-party app grants in `/sessions`), `ocl_`. Reserved (do not reuse): `ssoc_`, `odom_`, `rol_`, `dev_`, `dom_` (workspace_domains, later). Registered in `app/models/public_ids.go` (`framework/ids` registry, TypeID encoding).

## Code map

### Routes
- `routes/api.go` groups (letters from `00-foundation/api.md`):
  - `guest` (G; framework `CSRF` middleware = `http.CrossOriginProtection` on unsafe, throttled): `GET /auth/config`, `POST /users`, `POST /sessions`, `POST /magic-links`, `POST /password-resets`, `PATCH /password-resets/{prs}`, `PATCH /email-verifications/{ott}`, `PATCH /email-changes/{ott}`, `GET /invitations/preview`.
  - `user` (U = `auth:user`: session or first-party app; no workspace): `/sessions` index/destroy, `/user`, `/user/*`, `/user/memberships`, `POST /workspaces`, `PATCH /workspaces/{ws}`, `POST /email-verifications`, `POST /email-changes`. `GET /me`, `GET /user` use `auth` (A) instead.
  - `workspace` (A = `auth` any + `workspace` + `access.policy` + suspension): members, teams, invitations, workspace, audit logs. `access.policy` (was `two_factor.enforced`, same position) evaluates `AccessPolicy` for principal + workspace. Foundation `client.version` (426) and `Dimmi-Version` middleware wrap every `/api/v1` group. Routes that tokens must not reach (tokens, `DELETE /workspace`) add `auth:user` per route; owner `role` field on `PATCH /members/{mem}` checked by field policy.
  - `setup` (11).
- `routes/web.go` (browser only, `auth:session` where authenticated): `/auth/{provider}/redirect`, `/auth/{provider}/callback`; SPA shell for `/login`, `/register`, `/setup`, `/invitation`, `/login/magic`, `/reset-password`, `/verify-email`, `/email/confirm` (frontend later; server sets `Referrer-Policy: no-referrer`, `Cache-Control: no-store` on these).
- `routes/console.go`: periodic jobs below.
- `routes/channels.go`: none new (foundation channels `workspace:{ws_}`, `member:{mem_}`). Kick = listeners below calling `a.Broadcast` disconnect (centrifuge `node.Disconnect`).
- `routes/mcp.go`: register tools below.

### Controllers `app/http/controllers/api/v1/`
One controller per resource or sub-resource (rest.md rule 14). Actions keep business names.
| File | Methods |
|---|---|
| `auth/auth_config_controller.go` | Show (public login options + capability discovery) |
| `auth/setup_controller.go` | Show, Store (owned by 11) |
| `user_controller.go` | Store (`POST /users`, register), Show, Update, Destroy (`/user` singleton) |
| `session_controller.go` | Index (browser sessions + 09 `ListUserAppGrants`), Store (dispatch on `grant`: `AttemptLogin`, `CompleteTwoFactorChallenge`, `ConsumeMagicLink`; optional `reauth`), Destroy (`ses_`, `ogr_`, `current`), DestroyMany (collection `DELETE /sessions`, route `sessions.destroy-many`) |
| `magic_link_controller.go` | Store (request) |
| `password_reset_controller.go` | Store (request link), Update (redeem `{token, password}`) |
| `email_verification_controller.go` | Store (resend), Update (redeem `{token}`) |
| `email_change_controller.go` | Store (request), Update (redeem `{token}`) |
| `user_confirmation_controller.go` | Show (sudo state), Store (`method` password, email_code, or factor type with `Confirmation` capability: MVP `totp`) |
| `user_confirmation_code_controller.go` | Store (send code for passwordless users) |
| `me_controller.go` | Show |
| `user_deletion_preview_controller.go` | Show |
| `user_password_controller.go` | Update |
| `user_avatar_controller.go` | Update (`PUT`), Destroy |
| `auth_factor_controller.go` | Index, Store (`{type}`: begin enrollment or regenerate recovery codes), Update (`{code}` activate, `{name}`, `{preferred}`), Destroy (one), DestroyMany (collection `DELETE /user/auth-factors`, route `user.auth-factors.destroy-many` per rest.md rule 14; extensibility.md table says `destroy-all`, rest.md wins) |
| `user_identity_controller.go` | Index, Destroy |
| `membership_controller.go` | Index, Store (`invitation_token` accept), Destroy (leave) |
| `user_invitation_controller.go` | Index (pending invites for my email) |
| `security_event_controller.go` | Index |
| `workspace_controller.go` | Store (`POST /workspaces`), Update (`PATCH /workspaces/{ws}`: `trashed: false` only) |
| `current_workspace_controller.go` | Show, Update, Destroy (`/workspace` singleton) |
| `member_controller.go` | Index, Show, Update (`role`, `status`, `keep_owner`; per-field policy; routes to `UpdateMemberRole`, `DeactivateMember`, `ReactivateMember`, `TransferOwnership`) |
| `member_availability_controller.go` | Update (self) |
| `team_controller.go` | Index, Store, Show, Update, Destroy |
| `team_member_controller.go` | Store, Destroy |
| `invitation_controller.go` | Index, Store, Destroy |
| `invitation_delivery_controller.go` | Store (resend) |
| `invitation_preview_controller.go` | Show (preview by header token) |
| `personal_access_token_controller.go` | Index, Store, Destroy |
| `token_ability_controller.go` | Index |
| `audit_log_controller.go` | Index, Show |

`app/http/controllers/web/`: `oauth_redirect_controller.go` (Invoke), `oauth_callback_controller.go` (Invoke).

### Requests `app/http/requests/`
`store_user_request.go` (register), `store_session_request.go` (grant-discriminated: `password`, `two_factor`, `magic_link`, `email_code` rule sets), `store_magic_link_request.go`, `store_password_reset_request.go`, `update_password_reset_request.go`, `update_email_verification_request.go`, `store_email_change_request.go`, `update_email_change_request.go`, `store_user_confirmation_request.go`, `setup_request.go`, `update_user_request.go`, `update_password_request.go`, `update_avatar_request.go`, `store_auth_factor_request.go` (`type` in enabled registry, `name?`), `update_auth_factor_request.go` (`code` alone, or any of `name`, `preferred`), `destroy_user_request.go`, `store_membership_request.go` (`invitation_token` required), `store_workspace_request.go`, `update_workspace_request.go` (`/workspaces/{ws}`: `trashed` only), `update_current_workspace_request.go`, `destroy_current_workspace_request.go`, `update_member_request.go` (`role`, `status`, `reassign_to_member_id`, `keep_owner`), `update_availability_request.go`, `store_team_request.go`, `update_team_request.go`, `store_team_member_request.go`, `store_invitations_request.go`, `store_personal_access_token_request.go`, `list_audit_logs_request.go`.
`store_session_request.go` `two_factor` rule set: `method` required (registry), `code` (later `credential`); every grant accepts optional `reauth` boolean.
Custom rules `app/rules/`: `password_rule.go` (length, common list, breached), `slug_rule.go` (format + reserved), `timezone_rule.go`, `email_deliverable_rule.go` (syntax, IDNA, disposable on cloud), `abilities_rule.go`, `registry_rule.go` (value registered in named Go registry: factor types, auth methods, providers).

### Resources `app/http/resources/`
`me_resource.go`, `user_resource.go` (self, full), `user_summary_resource.go` (others: name, avatar), `workspace_resource.go`, `workspace_summary_resource.go`, `member_resource.go`, `membership_resource.go` (`object: membership`, own membership + `workspace`), `team_resource.go`, `invitation_resource.go`, `invitation_delivery_resource.go`, `invitation_preview_resource.go` (public: workspace name, inviter name, email masked), `personal_access_token_resource.go`, `new_personal_access_token_resource.go` (includes `plain_text_token` once), `session_resource.go` (`kind` browser or app; app rows built from 09 `ListUserAppGrants` row), `two_factor_challenge_resource.go` (`methods` array of objects), `email_verification_resource.go`, `email_change_resource.go`, `confirmation_resource.go`, `oauth_identity_resource.go`, `audit_log_resource.go`, `security_event_resource.go`, `auth_config_resource.go`, `auth_factor_resource.go` (`object: auth_factor`, no secrets), `new_auth_factor_resource.go` (adds one-time `setup` / `codes` / `recovery_codes`; `Cache-Control: no-store`), `workspace_domain_resource.go`, `token_ability_resource.go` (ability groups with member permissions).

### Actions `app/actions/`
- `auth/`: `register_user.go`, `attempt_login.go`, `complete_primary_auth.go` (`CompletePrimaryAuth(user, method, opts{reauth})`: 2FA decision, challenge creation via factor drivers, or `StartSession`), `start_session.go`, `complete_two_factor_challenge.go` (dispatches to factor driver by `method`), `logout.go`, `send_magic_link.go`, `consume_magic_link.go`, `send_password_reset_link.go`, `reset_password.go`, `send_email_verification.go` (creates `ott_` purpose `email_verification`), `verify_email.go`, `confirm_password.go` (`ConfirmUser`: password, email code or factor driver; writes `confirmed_at`, `confirmed_method`), `send_confirm_code.go`, `handle_oauth_callback.go` (resolves provider via `ProviderResolver`), `link_oauth_identity.go`, `unlink_oauth_identity.go`, `list_sessions.go` (merges `sessions` rows and 09 `ListUserAppGrants`), `revoke_session.go` (`ses_`: delete row; `ogr_`/`current` from app: 09 `RevokeAppGrant`), `revoke_other_sessions.go` (browser sessions + 09 `RevokeUserAppGrants(logout_others, except current grant)`) (setup: 11 `system.CompleteSetup` owns endpoint and token).
- `users/`: `create_user.go` (CLI/setup/invite shared), `create_first_user.go` (`CreateFirstUser`: user + `CreateWorkspace` + owner; called by 11 `CompleteSetup` and `user:create`), `update_profile.go`, `update_password.go`, `request_email_change.go`, `confirm_email_change.go`, `update_avatar.go`, `delete_avatar.go`, `delete_account.go`, `fetch_user_avatar.go`.
- `factors/` (auth factors): `add_auth_factor.go` (`AddAuthFactor`: driver `BeginEnrollment`, pending row; `recovery_code` type delegates to regenerate), `activate_auth_factor.go` (`ActivateAuthFactor`: driver `CompleteEnrollment`; first 2FA-enabling factor sets `two_factor_confirmed_at`, creates recovery set, mails, rotates browser session), `update_auth_factor.go` (rename, preferred), `remove_auth_factor.go` (`RemoveAuthFactor`: last 2FA-enabling factor = disable path), `remove_all_auth_factors.go` (`RemoveAllAuthFactors`: disable 2FA; CLI `user:disable-2fa` calls with `force` skipping workspace requirement), `regenerate_recovery_codes.go` (uses `recovery_code` driver), `disable_two_factor.go` (shared tail: clear `two_factor_confirmed_at`, delete recovery set, 09 `RevokeUserAppGrants(two_factor_disabled, except caller grant)`, mail, events).
- `workspaces/`: `create_workspace.go` (inserts `organizations` row then workspace, same tx), `update_workspace.go` (renames org while 1:1), `delete_workspace.go`, `restore_workspace.go`, `purge_workspace.go` (calls `RetentionGuard`; soft-deletes org when no live workspace), `leave_workspace.go`, `join_workspace_by_domain.go`, `add_workspace_domain.go`, `remove_workspace_domain.go`, `suspend_workspace.go`, `unsuspend_workspace.go`, `touch_workspace_access.go`.
- `members/`: `add_member.go`, `update_member.go` (applies `PATCH /members/{mem}` fields in one tx, calls the actions below), `update_member_role.go`, `deactivate_member.go`, `reactivate_member.go`, `transfer_ownership.go`, `update_availability.go`, `ensure_workspace_has_owner.go`.
- `teams/`: `create_team.go`, `update_team.go`, `delete_team.go`, `purge_team.go`, `add_team_member.go`, `remove_team_member.go`.
- `invitations/`: `create_invitations.go`, `resend_invitation.go`, `revoke_invitation.go`, `accept_invitation.go`.
- `tokens/`: `create_personal_access_token.go`, `revoke_personal_access_token.go`, `record_token_usage.go`.
- `audit/`: `record_audit_log.go`, `export_audit_logs.go`.

### Support, services, middleware
- Framework (no app code): password hashing = `framework/hashing` (`a.Hash`; argon2id PHC via alexedwards/argon2id, `NeedsRehash`, semaphore, `auth_busy`); token format = `framework/auth/token` (random base62, CRC32 checksum; `dmi_live_`/`dmi_test_` prefixes stay app constants in `app/auth/tokens.go`); TOTP + recovery code primitives = `framework/auth/twofactor` (pquerna/otp); validation = `framework/validation` tag engine (`validate:"required,min=12,password"`, Laravel-style English messages, translated per request locale); `password` rule (common list + breached check) = app `validation.Rule` registered on `a.Validator` in `AppServiceProvider` (framework has no password rule; HIBP range client one GET with `Add-Padding: true`, via `a.HTTP`); UA parse = `framework/http/useragent` (medama-io/go-useragent per `research/build-vs-reuse.md`); OAuth/OIDC login = `framework/socialite` drivers `github`, `google` (golang.org/x/oauth2 PKCE, coreos/go-oidc v3, go-jose v4 underneath); session mechanics (id, rotation, expiry) = `framework/session` (`a.Session`, Postgres store on framework `sessions` table, product columns ALTERed by 00-accounts); CSRF = `framework/http/middleware` `CSRF` (`http.CrossOriginProtection`).
- `app/auth/` (was `app/support/auth/`): `lockout.go` (escalating limiter on `ratelimit.Limiter` / `a.Limiter`), `device.go` (UA label via `framework/http/useragent`, `__Host-dimmi_device` cookie, `DeviceID()` = SHA-256 hex of cookie value), `disposable_domains.go` (embedded CC0 list `resources/data/disposable_email_blocklist.conf`), `guards.go` (session, app and token guards implementing `auth.Guard`, cached lookup; caller class decided by lookup, never client claim; `dmi_oat_` handed to 09 `oauth.ResolveAccessToken`), `principal.go` (builds `contracts.Principal` from guard result: `UserID`, `AuthMethods`, `AuthenticatedAt`, `ConfirmedAt`, `DeviceID`, `IP`, `Via`, `TwoFactorEnabled`; session: from row; app: from 09 principal `AuthTime` + `ConfirmedAt` + grant `auth_methods`, `device_id`; token: empty methods), `usage_recorder.go` (coalesced `last_used_at` / `last_activity_at`, flushed in provider `Shutdown`), `login_policy.go` (`LoginPolicyResolver`, MVP driver `allowAll`), `access_policy.go` (MVP `AccessPolicy` driver, rule list), `registries.go` (registry enums: auth methods, created_via, joined_via, security event types, revoke reasons passed to 09: `user`, `logout_others`, `password_changed`, `password_reset`, `two_factor_disabled`, `user_deleted`, `admin`, `pre_hijack_defense`).
- `app/auth/factors/` (driver registry; contract pointer in `00-foundation/contracts.md`): `registry.go` (`Register`, `Enabled()` filtered by `auth.factors.enabled`, `Find(type)`), `totp.go` (product rules on `framework/auth/twofactor`: RFC 6238 SHA1, 6 digits, 30 s, window ±1, replay via `last_counter`, otpauth URL; QR SVG rendered from `otpauth_url`), `recovery_code.go` (codes from `twofactor.RecoveryCodes`: 10 codes `xxxxx-xxxxx` Crockford base32 lowercase, SHA-256 list, one use each, `remaining` in `credential`). Interface:

```go
package factors

type Capabilities struct {
    SecondFactor      bool // satisfies 2FA challenge (totp, recovery_code)
    Backup            bool // recovery-only: never enables 2FA alone (recovery_code). Added here (00-accounts owns interface)
    Primary           bool // logs in alone (passkey); false for all at MVP
    Confirmation      bool // satisfies password.confirm (totp true, recovery_code false)
    PhishingResistant bool // webauthn later
    Multiple          bool // user may hold several (MVP: false for both)
    UserVisible       bool // listed in GET /user/auth-factors (both true; recovery set shown with remaining)
}

type Factor interface {
    Type() string
    DefaultName() string
    Capabilities() Capabilities
    BeginEnrollment(ctx context.Context, u *models.User, name string) (Enrollment, error)            // secret to store + public setup payload
    CompleteEnrollment(ctx context.Context, f *models.UserAuthFactor, input json.RawMessage) (*models.UserAuthFactor, error)
    BeginChallenge(ctx context.Context, u *models.User, fs []models.UserAuthFactor) (ChallengeOptions, error) // nil options for totp/recovery_code
    Verify(ctx context.Context, f *models.UserAuthFactor, state ChallengeState, input json.RawMessage) (VerifyResult, error) // constant time; ErrInvalidCode, ErrReplay
}

func EnablesTwoFactor(f Factor) bool { c := f.Capabilities(); return c.SecondFactor && !c.Backup }
```
Registered in `auth_service_provider.go`. Unknown type in `auth.factors.enabled`: boot panic. Disabled driver: its rows ignored by challenge and confirmation (user may still list and delete them).
- `app/auth/providers/` (`IdentityProvider` registry, extensibility.md section 2): `registry.go` (`ProviderResolver` MVP = static registry from config map `auth.oauth.<key>.*`; provider listed only when configured), `github.go` (protocol `oauth2`, wraps `a.Social.Driver("github")`), `google.go` (protocol `oidc`, wraps `a.Social.Driver("google")`). Interface `IdentityProvider{Key, Protocol, Display, AuthorizeURL, Callback}` returning `ExternalIdentity{Provider, Subject, Email, EmailVerified, Name, Username, AvatarURL, HostedDomain, ConnectionID (nil), Groups (nil)}`. Callback code never names a provider (no `if provider == "github"`).
- Authorization (foundation authz principal = AccessPolicy `Principal` + member + effective permissions): `app/auth/permissions.go` (catalog: ability groups (`app/auth/abilities.go`), 00-accounts permissions, `permissions.Register` for feature specs, boot checks), `app/auth/roles.go` (system role -> permission set; ONLY file naming roles for authorization), `app/auth/authz/can.go` (package `authz`, used by gates/policies registered on `framework/auth` `Gate`: `authz.Can(p authz.Principal, perm string) bool`; `authz.Principal{contracts.Principal, Member, Permissions}`; `EffectivePermissions(role, abilities)`). Paths moved from `app/support/auth/` per framework.md 10.1 (feature specs and contracts.md still saying `app/support/auth/permissions.go` mean `app/auth/permissions.go`); `Can` in own package so call sites read `authz.Can`. Lint/review: no `Role ==` outside `roles.go` and role-change actions (owner transfer, last-owner checks, owner-field policy stay role-based by nature).
- `app/services/turnstile/client.go` (siteverify via `a.HTTP`; one POST, kept custom). GitHub/Google clients and HIBP: framework (above), no `app/services/{github,google,hibp}`.
- `app/http/middleware/`: `authenticate_session.go` (`auth:session`), `authenticate_app.go` (`auth:app`: `dmi_oat_` resolved by 09 `oauth.ResolveAccessToken` to first-party grant without workspace), `authenticate_user.go` (`auth:user` = session or app; PAT/third-party OAuth 403 `first_party_only`), `authenticate_token.go` (`auth:token`: PAT here; `dmi_oat_` via `ResolveAccessToken`), `authenticate.go` (`auth` = any of the above) (implement foundation names), `redirect_if_authenticated.go` (`guest`; never blocks API guest routes, `reauth` relies on it), `require_password_confirmation.go` (`password.confirm`: session `confirmed_at`; app `principal.AuthTime` (+ grant confirmation, see flows 8)), `enforce_access_policy.go` (`access.policy`, replaces `two_factor.enforced`), `ensure_setup_complete.go`, `ensure_email_verified.go` (`verified`), `require_permission.go` (`can:<permission>`, replaces `role:admin`/`role:owner` route middleware), `touch_session.go`. No app CSRF middleware (old `verify_csrf_token.go` dropped): framework `CSRF` alias (`http.CrossOriginProtection`: `Sec-Fetch-Site`, fallback `Origin` vs `Host`; trusted origins `app.url` + `http.trusted_origins` via `AddTrustedOrigin`; deny handler renders JSON 403 `cross_origin_request`; requests with neither header pass as non-browser) applied to every cookie-capable group. No token cookie, no `/auth/csrf`.
- `app/enums/` (strict CHECK, state machines): `role.go`, `member_status.go`, `availability.go`, `invitation_status.go`, `auth_factor_status.go`, `signup_mode.go`. Registry enums live with their registries (`app/auth/registries.go`, `factors`, `providers`); actor types and via kinds come from `app/contracts` Actor.
- `app/policies/`: `workspace_policy.go`, `member_policy.go`, `team_policy.go`, `invitation_policy.go`, `personal_access_token_policy.go`, `audit_log_policy.go`. Every check `authz.Can(principal, "<perm>")`.
- `app/exceptions/accounts.go`: domain errors (codes in api.md).
- `app/providers/auth_service_provider.go`: guards, gates, factor drivers, identity providers, `LoginPolicyResolver`, `AccessPolicy` driver, permission catalog, settings keys (`security.require_two_factor`, `previous_slugs`), `/auth/config` capability tokens (`auth_factors`, `magic_link`, `email_code`, `first_party_apps`), `WorkspaceInitializer` registry (other specs register default inbox, widget settings, help center).

### Events `app/events/`
`workspace_created.go`, `workspace_updated.go`, `workspace_deleted.go`, `workspace_restored.go`, `workspace_suspended.go`, `member_joined.go`, `member_role_changed.go`, `member_deactivated.go`, `member_reactivated.go`, `member_availability_changed.go`, `team_created.go`, `team_updated.go`, `team_deleted.go`, `team_member_added.go`, `team_member_removed.go`, `invitation_created.go`, `invitation_accepted.go`, `invitation_revoked.go`, `token_created.go`, `token_revoked.go`, `user_registered.go`, `user_email_verified.go`, `user_updated.go`, `user_deleted.go`, `session_revoked.go`.

### Listeners `app/listeners/`
`write_audit_log.go` (foundation listener, implemented here), `disconnect_member_sockets.go` (AfterCommit: `a.Broadcast` disconnect = centrifuge `node.Disconnect(userID)`, cross-node via centrifuge Redis broker on Valkey; dashboard reconnect re-auths per workspace and `OnSubscribe` refuses `workspace:`/`conversation:` channels of the removed workspace), `disconnect_session_sockets.go` (AfterCommit: disconnects centrifuge clients whose connect credential was that `ses_`; connection meta carries session id), `bust_auth_cache.go` (AfterCommit, `pubsub.PubSub` control message on Valkey), `send_new_sign_in_notification.go` (queued mail), `send_invitation_accepted_notification.go`, `send_two_factor_required_notifications.go`, `send_member_deactivated_mail.go`, `queue_workspace_purge_reminder.go`, `forward_audit_logs.go` (queued, AfterCommit: `AuditSink.Write`; noop driver at MVP).

### Jobs `app/jobs/`
`purge_expired_auth_records.go`, `schedule_workspace_purges.go`, `purge_workspace.go`, `send_workspace_deletion_reminders.go`, `check_workspace_owners.go`, `prune_audit_logs.go`, `prune_security_events.go`, `notify_expiring_tokens.go`, `fetch_user_avatar.go`, `purge_deleted_teams.go`.

### Mail `app/mail/` + templates `resources/views/mail/accounts/*.templ`
See [api.md](api.md#mail-templates) (28 mailables).

### MCP `app/mcp/tools/`
`list_members.go`, `list_teams.go`, `update_availability.go` (toolset `accounts`; `dimmi_get_workspace` core tool owned by 09).

### Commands `app/console/commands/`
`user_create_command.go`, `user_list_command.go`, `user_reset_password_command.go`, `user_disable_two_factor_command.go` (`RemoveAllAuthFactors` force, actor system via cli), `user_verify_email_command.go`, `user_logout_command.go`, `setup_token_command.go`, `workspace_list_command.go`, `workspace_suspend_command.go`, `workspace_unsuspend_command.go`, `workspace_restore_command.go`, `workspace_purge_command.go`, `workspace_repair_command.go` (`workspace:repair`, re-runs initializer hook, ensures live org row and org name sync), `token_create_command.go`, `auth_lists_update_command.go` (refresh embedded disposable list at build time, dev only).

### Database
- Queries: `database/queries/users.sql`, `organizations.sql`, `user_auth_factors.sql`, `workspaces.sql`, `workspace_members.sql`, `teams.sql`, `team_members.sql`, `sessions.sql`, `oauth_identities.sql`, `one_time_tokens.sql`, `password_reset_tokens.sql`, `invitations.sql`, `personal_access_tokens.sql`, `audit_logs.sql`, `security_events.sql`, `XX`.
- Factories: `database/factories/user_factory.go` (`.WithPassword()`, `.WithTwoFactor()` = active totp factor + recovery set + `two_factor_confirmed_at`, `.Unverified()`), `auth_factor_factory.go` (`.Totp()`, `.RecoveryCodes(n)`, `.Pending()`), `workspace_factory.go` (`.WithOwner(user)`; creates org), `member_factory.go` (`.Admin()`, `.Deactivated()`), `team_factory.go`, `invitation_factory.go` (`.Expired()`), `personal_access_token_factory.go` (`.WithAbilities(...)`, returns plain token), `session_factory.go`, `audit_log_factory.go`.
- Seeders: `database/seeders/accounts_seeder.go` (demo owner `demo@dimmi.test`, admin, member, two teams, one token printed).

### Tests
- `tests/feature/auth/` : `users_store_test.go` (register), `sessions_password_grant_test.go`, `lockout_test.go`, `sessions_two_factor_grant_test.go`, `magic_links_test.go` (request + `magic_link`/`email_code` grants), `password_resets_test.go`, `email_verifications_test.go`, `oauth_github_test.go`, `oauth_google_test.go`, `user_confirmation_test.go`, `setup_test.go`, `cross_origin_test.go` (cross-site 403 `cross_origin_request`, same-origin and trusted dashboard origin pass, no-header CLI pass, bearer pass), `enumeration_timing_test.go`, `lockout_test.go` (escalation with `clock.NewFake(t)` `Travel`), `login_policy_test.go` (fake deny driver registered in test App), `password_rule_test.go` (common list, HIBP via `httpclienttest.New(tc.HTTP)` stub, fail open).
- `tests/feature/user/`: `profile_test.go`, `password_test.go`, `email_changes_test.go`, `sessions_test.go` (list/revoke browser + app rows, `current`, `DELETE /sessions`), `auth_factors_test.go` (index, store totp/recovery_code, activate, rename, preferred, destroy one/all, limits, workspace requirement), `reauth_test.go`, `identities_test.go`, `memberships_test.go` (list, invitation accept, leave; `workspace_id` body 422 E178), `account_deletion_test.go`, `security_events_test.go`, `first_party_app_test.go` (app token on U and A routes; PAT and third-party OAuth 403 `first_party_only`).
- `tests/feature/workspaces/`: `workspace_test.go`, `workspace_trash_test.go` (delete + `PATCH /workspaces/{ws} {trashed:false}`), `members_test.go` (role and status fields, combined body), `ownership_test.go` (`role: owner`), `availability_test.go`, `teams_test.go`, `invitations_test.go` (incl. deliveries), `tokens_test.go`, `me_test.go` (`member.permissions`), `audit_logs_test.go` (ascending cursor), `access_policy_test.go` (2FA rule on session, app, token; 403 body `requirement`, `action_url`), `organizations_test.go` (1:1 create, rename sync, purge soft-delete), `suspension_test.go`.
- No `tests/unit/` for app logic (testing.md: integration only). Primitives tested in framework: `framework/hashing`, `framework/auth/token` (checksum), `framework/auth/twofactor` (RFC 6238 Appendix B vectors). App factor drivers through `tests/feature/user/auth_factors_test.go` (replay via `last_counter`, recovery reuse) and boot tests below.
- `tests/feature/authz/` (real App boot): `roles_test.go` (role -> permission snapshot equals permissions matrix, incl. 07 agent actions, 08 GitHub/docs sync, 13-16 rows), `can_test.go` (token = role perms intersect expanded ability groups, via HTTP with PATs), `registry_boot_test.go` (unknown enabled factor type panics at boot, unconfigured provider absent, permission without group fails boot). `tests/arch/no_role_compare_test.go` (AST rule: no `Role ==` outside `roles.go` and role-change actions).
- `tests/feature/console/`: `user_create_command_test.go`, `token_create_command_test.go`, `workspace_purge_command_test.go`.
- OAuth providers: `tc.Social.Fake()` (`Approve(authURL, user)`) for flow tests; driver tests against `framework/socialite/drivertest` local identity provider or `httpclienttest.New(tc.HTTP)` stubs (GitHub API, Google discovery + JWKS with test key) under real `framework/socialite` drivers. Fakes panic outside `testing` env. Tests named in plain words; every edge case covered.

## HTTP API

Full table, examples, errors and rate limits in [api.md](api.md).

## Events

### Emitted (all new; add rows to `00-foundation/events-and-realtime.md` catalog)

| Event | Realtime | Webhook | Channel / notes |
|---|---|---|---|
| workspace.created | no | no | internal; initializers run in tx, not via event |
| workspace.updated | yes | yes | `workspace:{ws_}`; changes keys only |
| workspace.deleted | yes | no | `workspace:{ws_}`; clients log out of workspace; webhooks stop at deletion |
| workspace.restored | no | yes | |
| workspace.suspended / workspace.unsuspended | yes | no | ops action |
| member.joined | yes | yes | via invitation, setup, cli, created |
| member.role_changed | yes | yes | from/to |
| member.deactivated | yes | yes | consumers: kick sockets (here, centrifuge disconnect), unassign open conversations (03), stop notifications (06) |
| member.reactivated | yes | yes | |
| member.availability_changed | yes (presence) | no | already in catalog; owned here |
| team.created / team.updated / team.deleted | yes | yes | team.deleted consumed by 03 to unassign |
| team.member_added / team.member_removed | yes | yes | |
| invitation.created / invitation.accepted / invitation.revoked | yes (admins on `workspace:{ws_}`) | no | |
| token.created / token.revoked | no | no | audit only |
| user.registered | no | no | internal: 12 cloud onboarding, metrics |
| user.email_verified | no | no | internal: unlock gated features |
| user.updated | yes | no | broadcast as `user.updated` (member resource: name, avatar) on `workspace:{ws_}` of each active workspace of user |
| user.deleted | no | no | internal: 06 drops push subscriptions, 09 revokes OAuth grants |
| session.revoked | no | no | internal: disconnect centrifuge connections bound to session, bust guard cache. App grant revocation from `/sessions` fires 09 `oauth_grant.revoked` instead |

Security events (login, 2FA, auth factors) are rows in `security_events`, not domain events (no fan-out needed). Factor changes never fire domain events (user-scoped, no workspace).

### Consumed

| Event | Listener | Why |
|---|---|---|
| all audit-worthy events from every spec (catalog `WriteAuditLog` set) | `WriteAuditLog` | single audit writer |
| user.deleted / member.deactivated | `DisconnectMemberSockets` | foundation edge case: kick within 5 s (centrifuge `node.Disconnect`, all nodes) |
| session.revoked | `DisconnectSessionSockets` | |

### Required from other specs (listeners they own)
- 03: listener `UnassignDeactivatedMember` on member.deactivated calls 03 `UnassignMemberConversations` (honors `reassign_to_member_id`); listener `UnassignDeletedTeam` on team.deleted calls 03 `UnassignTeamConversations` then `teams.PurgeTeam`.
- 06: stop notifications on member.deactivated; delete push subscriptions on user.deleted.
- 09: revoke OAuth grants on user.deleted and on member.deactivated (workspace-scoped grants only; first-party user-wide grants stay, workspace access re-checked per request). Called synchronously by 00-accounts in caller tx (09 `oauth.md` "First-party apps" Revocation):
  - `ListUserAppGrants(userID, currentGrantID)`: `GET /sessions` `kind: app` rows (`ogr_`, `client_name`, `device_name`, `device_platform`, `app_version`, `auth_methods`, `auth_time`, `last_used_ip`, `last_used_at`, `created_at`, `current`; 09 README confirms all fields).
  - `RevokeAppGrant(userID, grantID, reason)`: `DELETE /sessions/{ogr}`, `DELETE /sessions/current` from app (reason `user`).
  - `RevokeUserAppGrants(userID, reason, exceptGrantID)`: `DELETE /sessions` (`logout_others`), password change (`password_changed`), password reset (`password_reset`), 2FA disabled (`two_factor_disabled`), account delete (`user_deleted`), CLI `user:logout` (`admin`), pre-hijack defense (`pre_hijack_defense`). Reasons land in `oauth_grant.revoked` payload `{grant_id, user_id, first_party, device_id, reason}`.
  - `oauth.ResolveAccessToken`: guards `auth:app` and `auth:token` hand every `dmi_oat_` to it; principal carries `AuthTime`, `ConfirmedAt` (grant `confirmed_at`, set by app `POST /user/confirmation`; `password.confirm` reads `max(AuthTime, ConfirmedAt)`), grant `auth_methods`, `device_id`.
  - Grant `auth_methods` + `device_id` copied from session at approval; `oauth_authorization_requests.auth_time` from session `authenticated_at`.

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| `PurgeExpiredAuthRecords` | maintenance | hourly | 3 | unique per period |
| `ScheduleWorkspacePurges` | maintenance | daily 03:00 UTC | 3 | unique per period |
| `PurgeWorkspace` | maintenance | enqueued per workspace past `purge_after` | 10, backoff up to 1 h; resumable (chunked deletes 5,000 rows per step, re-enqueues self); asks `RetentionGuard.MayDestroy` (subject `workspace`) first, held: skip + `retention_held`; soft-deletes org when no live workspace left | unique by workspace_id |
| `SendWorkspaceDeletionReminders` | mail | daily | 3 | unique per period; reminder 3 days before purge, once (`settings` flag) |
| `CheckWorkspaceOwners` | maintenance | daily | 3 | unique per period |
| `PruneAuditLogs` | maintenance | daily (cloud, or self-host when retention set) | 3 | unique per period; deletes in 10,000-row batches; `RetentionGuard` (subject `audit_log`) per batch |
| `PruneSecurityEvents` | maintenance | daily | 3 | unique per period |
| `NotifyExpiringTokens` | mail | daily | 3 | unique per period; sets `expiry_notified_at` |
| `FetchUserAvatar` | default | OAuth signup with avatar URL | 3 | unique by user_id |
| `PurgeDeletedTeams` | maintenance | hourly safety net; purges teams deleted more than 1 h ago with no remaining assignments | 3 | unique per period |
| Mailables (`SendMail` from foundation) | mail | actions | 8 over 6 h | unique by mailable + recipient + token id for auth mails |

`PurgeExpiredAuthRecords` deletes: expired/consumed `one_time_tokens` older than 1 day, expired `password_reset_tokens`, expired sessions (absolute or idle), `user_auth_factors` with status `pending` older than 1 h, invitations expired or revoked more than 90 days ago (status `expired` set first), revoked tokens older than 90 days.

Token `last_used_at` and session `last_activity_at` use in-process coalescer (`app/auth/usage_recorder.go`, flushed in auth provider `Shutdown`): batched `UPDATE ... FROM unnest()` every 60 s per replica and on shutdown. Not River jobs.

## Config keys

`config/auth.go` (env `DIMMI_AUTH_*`):
| Key | Default self-host | Default cloud | Notes |
|---|---|---|---|
| `auth.signup.mode` | `invite_only` (after first user; before first user only setup works) | `open` | `open`, `invite_only`. 11-self-host may override via `system_settings` |
| `auth.verification.required` | false | true | gate `verified` middleware |
| `auth.passwords.min_length` | 10 | 10 | max fixed 256 |
| `auth.passwords.breach_check` | false | true | HIBP range API, 2 s timeout, fail open |
| `auth.passwords.argon2.memory_kib` / `iterations` / `parallelism` | 12288 / 3 / 1 | same | OWASP config 3 |
| `auth.passwords.argon2.max_concurrent` | 2 | 8 | semaphore; waits up to 2 s then 503 `auth_busy` |
| `auth.magic_link.enabled` | true | true | |
| `auth.magic_link.ttl` | 15m | 15m | |
| `auth.sessions.idle_lifetime` / `absolute_lifetime` | 336h / 720h | same | security.md 14 d / 30 d |
| `auth.sessions.cookie` | `__Host-dimmi_session` (`dimmi_session` when `app.url` is http) | `__Host-dimmi_session` | |
| `auth.password_confirmation.ttl` | 15m | 15m | |
| `auth.oauth.<key>.*` | map; `github`, `google` at MVP | | adding provider = driver + config block. `client_id` empty = provider hidden. Secrets via `_FILE` |
| `auth.oauth.github.client_id` / `client_secret` | empty (button hidden) | set | |
| `auth.oauth.google.client_id` / `client_secret` | empty | set | |
| `auth.factors.enabled` | `[totp, recovery_code]` | same | enabled factor drivers; unknown type: boot panic |
| `auth.factors.max_per_user` | 10 | 10 | active factors per user (recovery set counts) |
| `auth.factors.totp.issuer` | `app.name` | `Dimmi` | otpauth issuer (was `auth.two_factor.issuer`) |
| `auth.factors.pending_ttl` | 1h | 1h | unfinished enrollment purge |
| `auth.lockout.per_ip_email` | 10/min | 10/min | api.md |
| `auth.lockout.per_email_soft_lock` | 30 failures/h | same | |
| `auth.lockout.per_ip_spray` | 100 failures/h | same | |
| `auth.disposable_email` | `off` | `block` | `off`, `flag`, `block` |
| `auth.captcha.driver` | `none` | `turnstile` | `none`, `turnstile` |
| `auth.captcha.site_key` / `secret_key` | | | |
| `http.trusted_origins` | `[]` | `[]` | extra origins allowed by CSRF (`http.CrossOriginProtection.AddTrustedOrigin`); `app.url` origin always trusted. Split dashboard deploys, local Vite dev server. Framework key, listed here because 00-accounts relies on it |
| `auth.geoip.database_path` | empty | set | optional MaxMind/DB-IP mmdb for session country; read with oschwald/maxminddb-golang v2 (mmap) |
| `workspaces.max_owned_per_user` | 0 (unlimited) | 10 | |
| `workspaces.max_test_mode_per_user` | 0 | 3 | |
| `workspaces.deletion_grace_days` | 30 | 30 | |
| `invitations.ttl` | 168h | 168h | |
| `invitations.max_per_day` | 0 (unlimited) | 100 per workspace | |
| `tokens.max_per_member` | 50 | 50 | |
| `audit.retention_days` | 0 (unlimited) | 365 | security.md |
| `security_events.retention_days` | 365 | 365 | |

Read, owned elsewhere: `apps.min_versions.ios` / `.android` / `.cli` (09; exposed in `/auth/config`), `apps.enabled` (09), `app.region`, `api.version` (foundation; exposed in `/auth/config`, `/me.instance`).

## Permissions matrix

Implemented as permission catalog (`app/auth/permissions.go`) + role map (`app/auth/roles.go`) + `authz.Can` (`app/auth/authz/can.go`); tables below are derived from `roles.go` (snapshot test keeps them equal). Policies call `authz.Can(principal, perm)`, never compare role names (P2).

Rules:
- Permission string format `resource:action`. Ability (token scope, OAuth scope, `api.md` catalog) = permission group: each permission belongs to exactly one group. Finer permissions added later join an existing group, so existing tokens keep meaning.
- Session and first-party app: effective permissions = role permissions.
- Token (PAT, third-party OAuth): effective = role permissions ∩ expanded ability groups (`*` = all groups). Replaces old "ability ceilings by role" table; same outcomes. Recomputed per request, so demotion shrinks token power immediately.
- `U`-only routes still block tokens regardless of permission (403 `first_party_only`).
- `/me` returns `member.permissions` (effective list, sorted). Clients gate UI on it, never on `role`.
- Owner-only structural checks stay role-based by nature (last-owner protection, ownership transfer, owner-field policy) inside role-change actions only.
- `system:read` / `system:write` are not role permissions: instance admin flag (11, `users.is_instance_admin`) grants them.

Catalog registration: 00-accounts declares ability groups (= `api.md` ability catalog) and its own permissions. Each feature spec registers its permissions from its service provider: `permissions.Register(key, group, ownerSpec)`. Role defaults for every permission live in `roles.go` (central; owner spec edits only its rows). Boot checks: group is a known ability; every registered permission has a `roles.go` entry (possibly no role); every `roles.go` permission is registered; key format `resource:action`. Unknown permission in `authz.Can`: panic in tests, deny + ERROR log in prod.

Permission groups (each permission in exactly one group):
| Group (ability) | Permissions in group | Owner spec |
|---|---|---|
| `conversations:read` | `conversations:read` | 03 |
| `conversations:write` | `conversations:write`, `conversations:delete`, `conversations:moderate`, `saved_replies:manage_shared` | 03 |
| `contacts:read` | `contacts:read` | 03 |
| `contacts:write` | `contacts:write`, `contacts:erase`, `contacts:manage_external_id`, `companies:delete` (03); `email_suppressions:manage`, `email_suppressions:lift_complaint` (04) | 03, 04 |
| `articles:read` / `articles:write` | the ability itself | 08 |
| `members:read` | `members:read` | 00-accounts |
| `members:write` | `members:write`, `members:manage_owners` (U-only field), `tokens:manage_any` (U-only route) | 00-accounts |
| `settings:read` | `settings:read`, `audit_logs:read` (00-accounts); `widgets:read` (01); `identity:read` (02); `email:read`, `email:read_raw` (04); `channels:inspect` (05); `help_centers:read_settings`, `github:read` (08); `imports:read` (16) | many |
| `settings:write` | `settings:write`, `workspace:delete` (U-only route) (00-accounts); `widgets:manage` (01); `identity:manage` (02); `inboxes:manage`, `tags:manage` (03); `email:manage`, `email:rotate_dkim`, `email:resume_sending`, `email_signatures:manage_own`, `email_signatures:manage_any` (04); `channels:manage` (05); `notification_destinations:manage` (06); `help_centers:manage`, `help_centers:delete`, `knowledge:manage`, `knowledge:sync`, `github:manage`, `docs_sync:manage` (08); `oauth_grants:manage_any`, `agent_access:manage` (09); `feedback:manage` (14); `imports:manage`, `imports:rollback` (16) | many |
| `webhooks:read` / `webhooks:write` | the ability itself (permissions of same name) | 10 |
| `notifications:read` | `notifications:read`, `notification_alerts:receive` | 06 |
| `notifications:write` | `notifications:write` | 06 |
| `ai:read` | `ai:read`, `ai_runs:read_payloads`, `ai_actions:read` | 07 |
| `ai:write` | `ai:write`, `ai_agent:manage`, `ai_action_calls:approve` | 07 |
| `ai:keys` | `ai:keys` | 07 |
| `ai:actions` | `ai_actions:manage` (agent actions: connections, secrets, actions, publish, live tests, `approval: admin` calls) | 07 |
| `exports:read` / `exports:write` | the ability itself | 11 |
| `changelog:read` | `changelog:read` | 13 |
| `changelog:write` | `changelog:write` | 13 |
| `feedback:read` | `feedback:read` | 14 |
| `feedback:write` | `feedback:write`, `feedback:delete` | 14 |
| `engage:read` | `engage:read` | 15 |
| `engage:write` | `engage:write`, `engage:manage` | 15 |
| `billing:read` | `billing:read` | 12 |
| `billing:write` | `billing:write`, `billing:cap_unrestricted` | 12 |
| `system:read` / `system:write` | the ability itself; never from a role: granted only by instance admin flag (11, `users.is_instance_admin`) | 11 |

Role map (`roles.go`; nested: owner ⊇ admin ⊇ member). Rows for other specs mirror their permission tables:
| Permission | Member | Admin | Owner |
|---|---|---|---|
| conversations:read/write, contacts:read/write, articles:read/write, members:read, notifications:read/write | yes | yes | yes |
| widgets:read (01), identity:read (02), email:read, email_signatures:manage_own (04), help_centers:read_settings, knowledge:sync, github:read (08), ai:read, ai:write, ai_actions:read, ai_action_calls:approve (07) | yes | yes | yes |
| settings:read, settings:write, audit_logs:read, members:write, tokens:manage_any | no | yes | yes |
| widgets:manage (01), identity:manage (02) | no | yes | yes |
| conversations:delete, conversations:moderate, saved_replies:manage_shared, contacts:erase, contacts:manage_external_id, companies:delete, inboxes:manage, tags:manage (03) | no | yes | yes |
| email:read_raw, email:manage, email_suppressions:manage, email_signatures:manage_any (04) | no | yes | yes |
| channels:manage, channels:inspect (05) | no | yes | yes |
| notification_destinations:manage, notification_alerts:receive (06) | no | yes | yes |
| ai_runs:read_payloads, ai_agent:manage, ai:keys, ai_actions:manage (07) | no | yes | yes |
| help_centers:manage, knowledge:manage, github:manage, docs_sync:manage (08) | no | yes | yes |
| oauth_grants:manage_any, agent_access:manage (09) | no | yes | yes |
| changelog:read, changelog:write (13) | yes | yes | yes |
| feedback:read, feedback:write (14) | yes | yes | yes |
| feedback:delete, feedback:manage (14) | no | yes | yes |
| engage:read, engage:write (15) | yes | yes | yes |
| engage:manage (15: publish/resume/edit live item, `bypass_throttle`, engage settings, deletes, CSAT surveys) | no | yes | yes |
| imports:read, imports:manage, imports:rollback (16) | no | yes | yes |
| webhooks:read, webhooks:write (10) | no | yes | yes |
| billing:read, billing:write (12) | no | yes | yes |
| workspace:delete, members:manage_owners (00-accounts) | no | no | yes |
| email:rotate_dkim, email:resume_sending, email_suppressions:lift_complaint (04) | no | no | yes |
| help_centers:delete (08) | no | no | yes |
| billing:cap_unrestricted (12) | no | no | yes |
| exports:read, exports:write (11) | no | no | yes |
| system:read, system:write (11) | no | no | no (instance admin flag only) |
Rows for 03, 05, 06, 09 are 00-accounts proposals pending those specs' own tables; 07 agent actions (`07-dimmi-agent/actions.md` Permissions), 08 GitHub/docs sync (`08-help-center/docs-sync.md` section 16), 13-14 and 16 rows copied from their permissions matrices; `engage:manage` is 00-accounts naming for `15-announcements-surveys` "admin+" rows (its policies call `authz.Can(p, "engage:manage")`, never role compare; owner adopts on next edit); owner spec moves its row in `roles.go` only (no policy edit). Tokens reach a permission only when holding its group ability. Owner-only extras: `workspace:delete`, `members:manage_owners`, plus owner-only feature permissions above (billing read/write are owner + admin).

Action table (permission checked by policy):
| Action | Permission | Owner | Admin | Member | Session or app only (U) |
|---|---|---|---|---|---|
| Read own profile, `/me` | none (any valid credential) | yes | yes | yes | no |
| Update own profile, password, email, auth factors, sessions, identities, delete account | none (self) | yes | yes | yes | yes |
| List members, teams | `members:read` | yes | yes | yes | |
| Set own availability | `members:write` or `conversations:write` (self) | yes | yes | yes | |
| Invite, resend, revoke invitations (admin/member roles) | `members:write` | yes | yes | no | |
| Change role member<->admin, deactivate/reactivate member or admin | `members:write` | yes | yes | no | |
| Promote to owner, demote owner, deactivate owner, transfer ownership | `members:manage_owners` | yes | no | no | yes + confirm |
| Teams CRUD and membership | `members:write` | yes | yes | no | |
| View workspace settings | `settings:read` (members see name, slug, timezone, locale without it) | yes | yes | partial | |
| Update workspace (name, slug, timezone, locale) | `settings:write` | yes | yes | no | |
| Update `require_two_factor` (`security.require_two_factor`) | `settings:write` | yes | yes | no | yes + confirm |
| Delete / restore workspace | `workspace:delete` | yes | no | no | yes + confirm |
| Leave workspace (`DELETE /user/memberships/{mem}`) | none (self; not last owner) | yes | yes | yes | yes |
| Create own token | none (self) | yes | yes | yes | yes + confirm |
| List/revoke own tokens | none (self) | yes | yes | yes | yes |
| List/revoke any member token | `tokens:manage_any` | yes | yes | no | yes |
| Audit logs read/export | `audit_logs:read` (token: `settings:read` group) | yes | yes | no | |
| Billing (12) | `billing:read` / `billing:write` (`billing:cap_unrestricted` owner only) | yes | yes | no | |

Audit event names registered by feature specs (written by `WriteAuditLog`; `audit_logs.event` dotted, no CHECK; listed so `filter[event]` docs and CSV consumers know them):
| Spec | Events |
|---|---|
| 07-dimmi-agent (agent actions) | `ai_action_connection.created`, `ai_action_connection.updated`, `ai_action_connection.secret_rotated`, `ai_action_connection.signing_secret_rotated`, `ai_action_connection.disabled`, `ai_action_connection.deleted`, `ai_action.created`, `ai_action.published`, `ai_action.disabled`, `ai_action.trashed`, `ai_action.restored`, `ai_action.version_restored`, `ai_action.live_write_tested`, `ai_action_call.approved`, `ai_action_call.declined`, `ai_agent.actions_enabled_changed` |
| 08-help-center (GitHub, docs sync) | `github_connection.created`, `github_connection.token_rotated`, `github_connection.deleted`, `github_repository.added`, `github_repository.deleted`, `github_repository.secret_rotated`, `docs_sync_source.created`, `docs_sync_source.updated`, `docs_sync_source.deleted`, `article.verified` |
| 13-changelog | `changelog.settings_updated`, `changelog_source.created`, `changelog_source.deleted`, `changelog_subscribers.imported`, `changelog_subscribers.exported`, `changelog_subscriber.deleted`, `changelog_mailing.cancelled`, `changelog_entry.published`, `changelog_entry.unpublished` (last two only when actor via token or MCP) |
| 14-feedback-roadmap | `feedback.settings_updated`, `feedback.board_created`, `feedback.board_deleted`, `feedback.status_created`, `feedback.status_deleted`, `feedback.contact_banned`, `feedback.contact_unbanned`, `feedback.posts_merged`, `feedback.post_unmerged`, `feedback.import_started`, `feedback.post_trashed`, `feedback.post_restored`, `feedback.portal_access_changed`, `feedback.bulk_status_changed` |
| 15-announcements-surveys | `announcement.status_changed`, `survey.status_changed`, `announcement.bypass_throttle_changed`, `engage_settings.updated`, `announcement.deleted`, `survey.deleted`, `survey_responses.deleted`, `survey_responses.exported`, `segment.deleted` |
| 16-importers | `import.created`, `import.updated`, `import.credentials_changed`, `import.archived`, `import.deleted`, `import_run.started`, `import_run.paused`, `import_run.cancelled`, `import_run.rollback_started`, `import.email_trust_attested`, `import.attribute_trusted` |

## Performance budgets

| Metric | Budget |
|---|---|
| `POST /sessions` (`grant: password`) p95 | under 250 ms (argon2id 12 MiB t=3 about 40-70 ms on 1 vCPU) |
| Guard lookup (cached) | under 0.2 ms; in-process otter cache 30 s TTL (layered over Valkey), busted by Valkey pubsub (`a.PubSub` `*pubsub.Bus` on Valkey) on revoke/deactivate/role change |
| Guard lookup (cold) | under 2 ms (one indexed query via SECURITY DEFINER function) |
| `GET /api/v1/me` p95 | under 20 ms |
| Members list (1,000 members) p95 | under 30 ms |
| Audit log list p95 (1M rows workspace) | under 50 ms (index `(workspace_id, created_at desc, id desc)`) |
| Peak extra RSS from hashing | `max_concurrent x 12 MiB` (24 MiB self-host default) |
| Writes per authenticated request | zero on hot path (coalesced activity) |

## Observability

Metrics (Prometheus, admin port):
- `dimmi_auth_logins_total{method,result}` (result: success, invalid, locked, two_factor_required, two_factor_failed)
- `dimmi_auth_lockouts_total{scope}` (ip_email, email, ip)
- `dimmi_auth_password_hash_seconds` histogram, `dimmi_auth_password_hash_wait_seconds` histogram
- `dimmi_auth_signups_total{method,edition}`, `dimmi_auth_signups_blocked_total{reason}`
- `dimmi_auth_guard_cache_hits_total{guard}`, `dimmi_auth_guard_cache_misses_total{guard}`
- `dimmi_invitations_total{event}` (created, accepted, revoked, expired)
- `dimmi_sessions_active` (gauge, refreshed every 5 min)
- `dimmi_tokens_requests_total{prefix_kind}` (live, test, oat)
Logs: event name, user id (`usr_`), workspace id, session id (`ses_`), token id (`tok_`), never secrets. Mail `log` driver fallback is only path printing auth links (see Security). Traces: span per action.

## Security and privacy notes (feature-specific)

- Every secret (session cookie, tokens, magic link, reset, email verification, invitation, email change) is 32 random bytes from `crypto/rand`, stored as SHA-256 only. Codes (6 digits) stored as HMAC with app key, max 5 attempts.
- Tokens in links travel in URL fragment (`#token=` or `#id=<ott_|prs_>&token=`), never query string or path: never reach server logs, proxies, or Referer. SPA sends token in JSON body (`POST /sessions {grant: magic_link, token}`, `PATCH /{collection}/{id} {token}`; rest.md rule 12). Public id alone grants nothing. Email link scanners cannot consume (no JS, no POST/PATCH).
- Session cookie per security.md. `__Host-` prefix needs HTTPS; plain-http self-host (localhost or LAN) falls back to `dimmi_session` without `Secure`, boot WARN, `dimmi doctor` warns.
- CSRF: Go `net/http.CrossOriginProtection` (framework `CSRF` middleware) on every unsafe request in cookie-capable groups (guest, `auth:session`, `auth:user`, `auth`), including login and register (login CSRF). Rejects when `Sec-Fetch-Site` is `cross-site`/`same-site` from an untrusted origin, or (header absent) `Origin` host differs from `Host` and is not a trusted origin (`app.url` + `http.trusted_origins`). Requests with neither header pass (non-browser clients). 403 `cross_origin_request`. No `XSRF-TOKEN` cookie, no `/auth/csrf` call. Bearer wins when both present; check is harmless for bearer (CLI, apps send neither header).
- First-party app tokens (09) never authenticate guest login routes; app login is browser sheet + OAuth. App tokens reach `U` routes like a session; PATs and third-party OAuth never do (403 `first_party_only`).
- Session id rotated on login, reauth, 2FA completion, first 2FA-enabling factor activation, password change, email change; role change of user in any workspace not needed (role checked live).
- Password change revokes all other sessions and first-party app grants (keeps caller's own session or grant). Password reset revokes all sessions and all first-party app grants. 2FA disable (last 2FA-enabling factor removed, any path) revokes first-party app grants except caller's (09 `RevokeUserAppGrants`). Adding, renaming or removing a factor that leaves 2FA on revokes nothing. PATs and third-party OAuth grants unaffected (documented; user revokes explicitly).
- 2FA enforced after every primary method (password, magic link, email code, OAuth, password reset), including reauth. Setup token login and CLI bypass by design (operator).
- `LoginPolicyResolver.ForEmail` runs before any secret is verified on every primary method (MVP allow-all). Later SSO-enforced domain answers 409 `sso_required` at domain level (no account enumeration).
- Every session and first-party grant records `auth_methods`, `authenticated_at`, `device_id` (P3); `device_id` never leaves server (not in resources, logs only hashed already).
- Account linking by verified email only; unverified local account matched by verified OAuth email gets pre-hijack defense (password cleared, sessions killed, first-party grants revoked with reason `pre_hijack_defense`).
- Factor secrets (`user_auth_factors.secret_encrypted`: TOTP seed, recovery code hashes) use `encryption.Seal(ctx, keys, encryption.Slot{Scope: encryption.Scope{Kind: "system"}, AAD: factorID}, value)` (keys = `a.Crypt.Keys(store)`) (`encryption.KeyManager`) (`encryption_keys` system row; not workspace key, users span workspaces). Setup secret and codes returned once, `Cache-Control: no-store`, never logged.
- Mail `log` driver (self-host without SMTP): prints auth links at WARN with `mail_fallback=true`. Exception to "never log tokens" rule, allowed only when `mail.default=log` and `app.edition=self_host`. Boot banner and `/api/v1/auth/config` expose `mail_configured=false`.
- Enumeration: login, magic link, reset return same response and pad to constant work (dummy argon2 verify on unknown user). Register reveals taken email (422) by design, throttled 5/hour per IP on cloud.
- Audit `changes` pass through redactor: keys matching `password|secret|token|key|code` replaced with `[redacted]`.
- `access.policy` evaluates per workspace request with cached principal (no extra query); failures answer 403 `{code, message, requirement, action_url}`; applies to session, app and token alike.
- Privacy: account deletion anonymizes user row and deletes identities, sessions, tokens, security events, avatar file. Workspace audit logs keep actor label snapshot for retention period (legitimate interest; documented in privacy policy).

## Metering impact

No billable activity created here. Inputs to metering:
- D8 / D10: 03 and 12 call `members.FindActiveMemberByVerifiedEmail(workspace, email)` (query in `workspace_members.sql` joined to users) to mark conversations started by workspace members as internal/test, never billable.
- D10: `workspaces.test_mode = true` workspaces never billable (cap 1,000/month enforced by 12); creation limited by `workspaces.max_test_mode_per_user`. Tokens there use `dmi_test_` prefix.
- Workspace deletion: `workspace.deleted` consumed by 12 to finalize usage and cancel subscription. Purge must not cascade billing ledger (12 owns retention; see dependencies).
- Suspension (`suspended_at`) set here; 12 sets it for billing reasons through `SuspendWorkspace` action.

## Dependencies on other specs

| Spec | Needs from it | Gives to it |
|---|---|---|
| 00-foundation / framework | `framework/http` middleware (incl. `CSRF` = `http.CrossOriginProtection`), `ratelimit.Limiter`, `mail.Mailer` + `log` driver, `encryption.KeyManager` (`encryption.Scope{Kind: "system"}`), `pubsub.Bus` control topic (Valkey), `ratelimit` `redis` driver (Valkey; PO decision: Valkey required in every deployment), `framework/broadcasting` (centrifuge disconnect), `framework/queue` (River), `framework/hashing`, `framework/auth` (+ `token`, `twofactor`), `framework/session`, `framework/socialite`, `framework/validation` password rule, `framework/http/useragent`, new prefixes in `app/models/public_ids.go` (`ott_`, `prs_`, `fac_`, `org_`), contracts `AccessPolicy` + `Principal` + `PolicyFailure`, `RetentionGuard`, `AuditSink`, settings registry (`app/support/settings`), `client.version` + `Dimmi-Version` middleware, RLS function `dimmi_visible_workspaces()`, `GET /workspace/entitlements` (foundation-owned, not here) | migrations for core account tables + `organizations`, `auth:session`/`auth:app`/`auth:user`/`auth:token`/`auth` guard implementations, `access.policy` middleware + MVP `AccessPolicy` driver, `authz` package (permission catalog, `roles.go`, `Can`), factor and identity provider registries, `WriteAuditLog` |
| 01-messenger-widget | registers `WorkspaceInitializer` for widget settings; reads member availability | presence inputs |
| 03-shared-inbox | `WorkspaceInitializer` default inbox "Support"; listeners on member.deactivated, team.deleted | member/team lookups, `FindActiveMemberByVerifiedEmail` |
| 04 / 05 channels | apply `verified` middleware on connect endpoints (cloud) | |
| 06-notifications | stop notifications on deactivation; preferences per member; `push_subscriptions.device_id` (browser: same hashed device cookie as `sessions.device_id`) | member events, `sessions.device_id` |
| 08-help-center | redirect old slug (`settings.previous_slugs`) | |
| 09-agent-native | `oauth.ResolveAccessToken` (first-party app vs third-party) plugged into guards, principal `AuthTime`; consent screen uses session login; revoke grants on user.deleted; `ListUserAppGrants`, `RevokeAppGrant`, `RevokeUserAppGrants`; grant `auth_methods`, `device_id`, `auth_time`; `apps.min_versions.*` config | `/me` (`member.permissions`), permission catalog (scopes = ability groups, one vocabulary), session login + `reauth` mode, session `authenticated_at` / `auth_methods` / `device_id`, `auth:user` group, `/sessions` UI for app grants |
| 10-webhooks | delivers member.*, team.*, workspace.updated/restored | |
| 07-dimmi-agent (agent actions), 08-help-center (GitHub, docs sync), 13-changelog, 14-feedback-roadmap, 15-announcements-surveys, 16-importers | register permissions (`permissions.Register`) and audit event names above | permission groups `ai:actions`, `changelog:*`, `feedback:*`, `engage:*`; permissions `ai_actions:*`, `ai_action_calls:approve`, `github:*`, `docs_sync:manage`, `imports:*` in existing groups; `roles.go` rows |
| 11-self-host | `system_settings` override of `auth.signup.mode`; SMTP setup; `dimmi doctor` checks (cookie mode, mail); `region` (self-host default `self`), `server_version` | setup wizard API, CLI user commands |
| 12-cloud-billing | owner-only billing; keeps ledger/invoices outside workspace cascade; may call `SuspendWorkspace`; initializer creates billing account per org if absent | workspace lifecycle events, test_mode flag, `organizations` row (billing account owner), `auth_organization_for_workspace` |

## Open questions (with recommended default)

1. Service accounts / workspace bot tokens (survive member leaving). Default: not in MVP; tokens tied to member; docs recommend dedicated "CI" member. Later path: nullable `personal_access_tokens.owner_id` + `service_account_id` (extensibility.md section 10).
2. Password minimum length. NIST SP 800-63B-4 (2025): 15 chars when password single factor, 8 when part of MFA. Default: 10 + common-password list + breach check on cloud; revisit 15 once telemetry shows signup impact.
3. Audit log ability. Resolved: permission `audit_logs:read` exists in catalog, grouped under token ability `settings:read` (no new ability).
4. Self-host open signup toggle in UI. Default: config/env only in this spec; 11 may expose via `system_settings`.
5. Second workspace created by same user: new org or same org? Default (extensibility.md OQ1): new org at MVP; merge decided with org UI launch.
6. Expose `organization_id` in workspace resource? Default: no (additive later).
