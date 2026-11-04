# 00 Accounts: tasks

Style: caveman. Scope trim 2026-11-02: domain join removed (later: `domain-join`); E refs renumbered. Order matters. Each task about 1 day, merged with tests. Depends on foundation skeleton, database, HTTP, async, mail, KeyManager (`encryption_keys` keysets + `encryption.Scope`), settings registry, contracts (`AccessPolicy`, `RetentionGuard`, `AuditSink`), RLS function `dimmi_visible_workspaces()`, `client.version` / `Dimmi-Version` middleware tasks. Updated 2026-11-02 for extensibility seams. Final sweep 2026-11-02: framework packages (hashing, auth/token, auth/twofactor, socialite, session, validation, ratelimit on Valkey) replace app primitives; CSRF = `http.CrossOriginProtection`; kick via centrifuge; 13-15 permissions. All app tests are integration (real Postgres + Valkey, testing.md); tests named in plain words, every edge case covered.

## M1. Schema and primitives

- [ ] Migrations 000001-000005: users (no 2FA secret columns), organizations, workspaces (`organization_id not null`), workspace_members, teams/team_members (`deleted_at`, partial unique name, `team_members.workspace_id` + RLS) per `data-model.md` / extensibility.md. Accept: `dimmi migrate` on empty DB; goose down works locally. Test: schema snapshot test.
- [ ] Migrations 000006-000014: sessions (`auth_methods`, `authenticated_at`, `confirmed_method`, `device_id`), user_auth_factors, oauth_identities, one_time_tokens, password_reset_tokens, invitations, personal_access_tokens, audit_logs (UPDATE revoked), security_events. Accept: constraints and indexes per README (factor partial uniques: one preferred, one active recovery set, unique `lookup_hash`; `one_time_tokens` purpose incl. `email_verification`; `password_reset_tokens` `id` pk + unique `user_id`). Test: constraint tests (unique pending invitation, unique provider id, one reset row per user, one active recovery set, strict CHECK values, registry format CHECK rejects `Bad-Value`).
- [ ] Migrations 000016-000018 core changes + 000019 SECURITY DEFINER lookup functions (incl. `auth_organization_for_workspace`). RLS via `dimmi_visible_workspaces()`. Accept: functions return only listed columns; app role cannot UPDATE audit_logs; lint annotations `user-scoped` / `global` present. Test: RLS on (cloud mode) still allows lookups via functions only.
- [ ] sqlc query files for all 16 tables; tenant-filter lint passes (documented exceptions for user-scoped/global tables and functions).
- [ ] Public id prefixes in `app/models/public_ids.go` (`framework/ids` registry): `mem_`, `inv_`, `ses_`, `aud_`, `idn_`, `dom_`, `sev_`, `ott_`, `prs_`, `fac_`, `org_`. Test: round trip, wrong prefix 404 (redemption routes: 422, no id oracle).
- [ ] Strict enums (role, member status, availability, invitation status, factor status, signup mode) + registry enums (`app/auth/registries.go`: auth methods, created_via, joined_via, security event types; provider and factor type from their registries; actor type, via kind from contracts) + `registry_rule.go`. Test: strict CHECK lists match enums (generated test); every registry value matches format CHECK; unregistered value rejected by rule and action.
- [ ] Wire `a.Hash` (`framework/hashing` argon2id, config `auth.passwords.argon2.*` -> hashing config, semaphore 2 s wait -> 503 `auth_busy`). Integration test: login under concurrency cap returns `auth_busy`; old bcrypt/param hash rehashed on login.
- [ ] `app/auth/tokens.go`: `dmi_live_`/`dmi_test_` prefixes on `framework/auth/token`. Integration test: created token authenticates; 1-char typo rejected without DB query (query counter).
- [ ] Password rule = app `validation.Rule` `password` (common list + HIBP range via `a.HTTP`) registered on `a.Validator`, used as `validate:"required,min=12,password"`, + app email/name check. Integration test via `POST /users`: common password 422, HIBP hit 422 (`httpclienttest.New(tc.HTTP)` range stub), HIBP timeout fails open.
- [ ] Authorization: `app/auth/permissions.go` (ability groups, 00-accounts permissions, `permissions.Register` for feature specs, boot checks), `app/auth/roles.go` (all role rows incl. feature specs' defaults and 07/08/13-16 rows: `ai_actions:read|manage`, `ai_action_calls:approve`, `github:read|manage`, `docs_sync:manage`, `changelog:*`, `feedback:read|write|delete|manage`, `engage:read|write|manage`, `imports:read|manage|rollback`; new group `ai:actions`), `app/auth/authz/can.go` (`Can`, `EffectivePermissions`), `require_permission.go` (`can:<perm>`). Integration test (`tests/feature/authz`, real App boot): role snapshot equals README matrix; token = role ∩ groups via HTTP; `*` expansion incl. new groups (E270); boot fails on permission without group, unregistered `roles.go` entry, or registered permission without `roles.go` entry. Arch test (`tests/arch`): no `Role ==` outside `roles.go` and role-change actions.
- [ ] Audit event name list for 07 agent actions, 08 GitHub/docs sync, 13-16 (README) in audit docs + `filter[event]` OpenAPI examples. Test: E268, E269, E271-E273 permission cases.
- [ ] Factories (user `.WithTwoFactor()` via factor rows, auth factor, workspace (creates org), member, team, invitation, token, session, audit log) + accounts seeder.

## M2. Sessions, CSRF, password login

- [ ] Session guard + `StartSession` (`auth_methods`, `authenticated_at`, `confirmed_method`, `device_id` from hashed device cookie) + cookie modes (`__Host-` vs http fallback) + device cookie + otter cache + Valkey pubsub bust (`a.PubSub`). Session mechanics on `framework/session` (Postgres store on framework `sessions` table + ALTERed product columns). Test: idle/absolute expiry, rotation keeps/creates auth context correctly, `device_id` = SHA-256 of cookie, http mode cookie name.
- [ ] Guard groups `auth:session`, `auth:app` (`dmi_oat_` to 09 `oauth.ResolveAccessToken`: first-party grant without workspace), `auth:user`, `auth:token`, `auth`; `contracts.Principal` builder (`principal.go`, incl. `ConfirmedAt`, `TwoFactorEnabled`); guards implement `auth.Guard` in `app/auth/guards.go`; route groups in `routes/api.go` per README. Test: PAT and third-party OAuth on `U` route 403 `first_party_only`; fake first-party app token reaches `U` and `A` routes with `Dimmi-Workspace`; client cannot claim app class; principal fields per caller class.
- [ ] `LoginPolicyResolver` (allow-all driver) wired as first step of every primary method. Test: fake deny driver blocks password, magic link request/consume, email code, OAuth callback, reset redeem, register before any secret check (hasher call counter 0).
- [ ] CSRF: enable framework `CSRF` alias (`http.CrossOriginProtection`) on guest + cookie-capable groups; trusted origins `app.url` + `http.trusted_origins`; JSON deny handler 403 `cross_origin_request`. No token cookie, no `/auth/csrf`. Test (E28, E29, E30, E261-E263): cross-site `Sec-Fetch-Site` 403, untrusted `Origin` 403, trusted dashboard origin passes, no headers passes, bearer passes.
- [ ] `auth/config` endpoint with capability discovery: `providers` and `factors` as objects, `server_version`, `api_version`, `region`, `capabilities` registry, `min_client_versions`. Test: reflects config per edition; unconfigured provider absent; disabled factor absent; OpenAPI schema arrays of objects.
- [ ] `POST /users` register (no invitation, optional workspace) with velocity limits. Test: happy, duplicate, weak password, signup_closed, disposable block (cloud config).
- [ ] `POST /sessions` grant dispatch (`store_session_request.go` per-grant rules) + `grant: password` + lockout (IP+email escalation, per-email soft lock with device bypass, per-IP spray). Counters on `a.Limiter` (Valkey). Test: escalation timings with `tc.Clock.Freeze`, known device bypass, identical responses, unknown grant 422, 201 vs 200 `two_factor_challenge`.
- [ ] Enumeration timing test (unknown vs wrong password p50 delta under 10 ms over 50 runs).
- [ ] `DELETE /sessions/current` (logout), `GET /sessions` (`auth_methods`, `authenticated_at`, derived `two_factor_used`; no `device_id`), `DELETE /sessions/{ses}`, `DELETE /sessions` (others, `password.confirm`) for browser rows (centrifuge connections of that session disconnected via `session.revoked`, E265). Test: revoked session 401 on other replica simulation (cache bust), foreign id 404.
- [ ] Reauth mode: `reauth` on every primary grant + OAuth `intent=reauth`; user match, forced 2FA, rotation with fresh `authenticated_at`. Test: mismatch 422 `reauth_user_mismatch` session kept; 2FA user challenged; `authenticated_at` advances; no new-device mail; without cookie = plain login.
- [ ] Usage coalescer for `sessions.last_activity_at` and token `last_used_at`. Test: one UPDATE per interval; flush on shutdown.

## M3. Verification, reset, magic link, sudo mode

- [ ] Email verification via `one_time_tokens` purpose `email_verification` (24 h): `POST /email-verifications` (resend), `PATCH /email-verifications/{ott} {token}`, `verified` middleware. Test: stale email 410, expired 410, already verified 200 idempotent, wrong token 422, cross-device verify.
- [ ] `POST /password-resets` + `PATCH /password-resets/{prs} {token, password}`. Test: always 202, upsert invalidates old id, wrong token and unknown id same 422 and timing, sessions revoked, 2FA still required (200 `two_factor_challenge`).
- [ ] `POST /magic-links` + `POST /sessions` grants `magic_link` (token) and `email_code` (email+code), signup via magic link, pre-hijack defense. Test: scanner-safe (GET never consumes), attempts burn, max 3 active, signup closed sends nothing, disabled = `grant_not_allowed`.
- [ ] `GET/POST /user/confirmation` (`method` password, email_code, or factor type with `Confirmation`; writes `confirmed_at`, `confirmed_method`) + `POST /user/confirmation-codes` + `password.confirm` middleware 423 (session `confirmed_at`; app `principal.AuthTime`, grant confirmation once 09 adds it). Test: TTL expiry with frozen clock, 404 when none, app path via `AuthTime` and via grant `ConfirmedAt` after app `POST /user/confirmation` (09 `ConfirmGrant`, E266), recovery code rejected as confirmation method, `authenticated_at` unchanged.
- [ ] Mailables for M2-M3 (Verify, MagicLink, Reset, ConfirmCode, PasswordChanged, NewSignIn, SuspiciousSignInAttempts) templ + text. Test: `tc.Mail.Fake()` + `mail.AssertSent[M]` asserts recipients and locale (`.Locale(...)` from user, fallback workspace), links use fragment tokens, no token in subject.
- [ ] Mail `log` fallback behavior (WARN line with `mail_fallback=true`, only self-host). Test: log capture.

## M4. OAuth

- [ ] `IdentityProvider` registry + `ProviderResolver` (static from `auth.oauth.<key>.*` map). Test: unconfigured key 404, unknown key 404, fake third driver works end-to-end with zero controller change.
- [ ] GitHub `IdentityProvider` (`oauth2`) on `a.Social.Driver("github")` (x/oauth2 PKCE S256, `/user`, `/user/emails`). Integration test through callback route: `httpclienttest.New(tc.HTTP)` stubs (or socialite `drivertest` local provider) happy path, no verified email.
- [ ] Google `IdentityProvider` (`oidc`) on `a.Social.Driver("google")` (go-oidc v3 discovery + JWKS, nonce, PKCE). Integration test: fake discovery + JWKS, bad nonce, `email_verified=false`.
- [ ] Redirect/callback web controllers (browser only; `intent=link` needs `auth:session` + confirm; `intent=reauth` needs session), encrypted state cookie, intents login/link/reauth, invitation carry-over, safe redirect. Test: state mismatch, open redirect rejected, link with bearer only 401, reauth user mismatch.
- [ ] Account resolution: existing identity, auto-link verified email, pre-hijack defense (grants revoked with reason `pre_hijack_defense`, E267), signup rules, `FetchUserAvatar` job (`a.HTTP`). Test matrix for each branch.
- [ ] Identities list/unlink with `last_login_method` rule. Test.

## M5. Auth factors and access policy

- [ ] Factor registry (`app/auth/factors`): interface, `Capabilities` (incl. `Backup`), `Enabled()` from `auth.factors.enabled`, boot panic on unknown type, `EnablesTwoFactor`. Integration test (boot + `GET /auth/config`): registry filtering, unknown type boot panic, capability table.
- [ ] `totp` driver on `framework/auth/twofactor` (pquerna/otp): enrollment (seed, otpauth URL, QR SVG), verify window ±1, replay via `last_counter`, secret `encryption.Seal` under `encryption.Scope{Kind: "system"}` with AAD factor id. RFC 6238 vectors live in framework unit tests. Integration test via `/user/auth-factors` + `POST /sessions`: replay rejected, wrong AAD fails decrypt.
- [ ] `recovery_code` driver: 10 codes, SHA-256 list, normalization, one use each, `remaining`. Test: reuse rejected, concurrent use (row lock), exhaustion leaves row `remaining: 0`.
- [ ] `/user/auth-factors` index, store (`totp`, `recovery_code` regenerate), update (`code` activate, `name`, `preferred`), destroy, destroy-many + actions (`AddAuthFactor`, `ActivateAuthFactor`, `UpdateAuthFactor`, `RemoveAuthFactor`, `RemoveAllAuthFactors`, `RegenerateRecoveryCodes`, `DisableTwoFactor`). Test: setup/codes shown once (`no-store`); second totp 422 `factor_limit_reached`; pending replaced; 5 wrong codes delete pending; first activation sets `two_factor_confirmed_at`, creates recovery set, mails, rotates session; remove last factor blocked by workspace requirement 409 with list; recovery set delete 409 `recovery_codes_required`; disable revokes other first-party grants (`RevokeUserAppGrants(two_factor_disabled)`) keeps caller's; security events renamed `auth_factor.*`; pending purge after 1 h.
- [ ] 2FA challenge after every primary method (password, magic link, email code, OAuth, reset, reauth): 200 `two_factor_challenge` with `methods` objects + cookie, then `POST /sessions {grant: two_factor, method, code}`. Test: each method path, `method` missing 422, not offered 422 `method_not_available` (no attempt), attempts burn, recovery code mail with remaining, session `auth_methods = {primary, method}`, preferred first, exhausted set omitted, disabled driver omitted.
- [ ] `AccessPolicy` MVP driver + `access.policy` middleware (replaces `two_factor.enforced`): rule `RequireTwoFactor` on setting `security.require_two_factor` (settings registry key), session + app + token, exempt routes, cache bust on toggle/factor change; `TwoFactorRequiredMail`. Test: 403 body `{code, message, requirement, action_url}` for each caller class; token of member without 2FA 403; works right after activation; toggle requires actor 2FA.
- [ ] CLI `user:disable-2fa` = `RemoveAllAuthFactors` force. Test: overrides workspace requirement, actor system via cli, grants revoked.

## M6. Workspaces and members

- [ ] `CreateWorkspace` (org row first, same tx) + `WorkspaceInitializer` registry + slug generator + reserved slugs + limits. Initializer list and order per 00-foundation/contracts.md. Test: org created 1:1, initializers run in tx (fake initializer failure rolls back org too), re-run idempotent, slug suffixing, no `organization_id` in resource.
- [ ] `dimmi workspace:repair [--workspace slug]` re-runs initializers and ensures live org (clears org `deleted_at`, syncs name). Test: deleted default inbox recreated, soft-deleted org restored, second run no-op.
- [ ] `workspace` middleware integration (session, app, token): membership check, deactivated 403, suspended writes 403, deleted 404, `last_accessed_at` touch. `GET /user/memberships` (`membership` resource, `filter[trashed]`). Test: cross-workspace 404, app token with header behaves like session.
- [ ] Workspace show/update (slug history, timezone/locale validation, test_mode immutable, rename syncs org, `require_two_factor` field stored as `security.require_two_factor` via settings registry). Events `workspace.updated`. Test.
- [ ] Members index/show + `PATCH /members/{mem}` field policies (`UpdateField` via `authz.Can`: `members:write`, `members:manage_owners`) and `UpdateMember` tx: `role` with last-owner lock, owner changes need U + confirm. Test: concurrent demotion race (two goroutines), role + status in one body (events order, all-or-nothing), same value no event, token on owner field 403 `first_party_only`.
- [ ] `PATCH /members/{mem} {status}` deactivate/reactivate + `DELETE /user/memberships/{mem}` leave + token revoke + socket kick + `member.deactivated` payload with reassign hint. Test: tokens 401 after deactivation, events asserted, self-deactivate 422, first-party app grant survives with 403 `membership_deactivated` for that workspace.
- [ ] Ownership transfer via `PATCH /members/{mem} {role: owner, keep_owner?}` (caller to admin same tx) + `CheckWorkspaceOwners` job. Test: eligibility errors, non-owner 403, `keep_owner`, repair promotes oldest admin.
- [ ] Availability endpoint + `member.availability_changed`. Test.
- [ ] Workspace delete (`DELETE /workspace`) / restore (`PATCH /workspaces/{ws} {trashed: false}`) / reminder / purge jobs (chunked, resumable, `RetentionGuard` first, org soft-delete after). Test: purge removes tenant rows and files (real `local` disk in `t.TempDir()`), fake guard returning false skips purge and records `retention_held`, org `deleted_at` set, restore after purge start 410, other fields 422 `prohibited`, non-owner 404.
- [ ] Suspension action + CLI. Test: GET allowed, POST 403.

## M7. Teams, invitations

- [ ] Teams CRUD, two-phase delete, `PurgeDeletedTeams`, membership bulk add/remove (`team_members.workspace_id` from team). Test: name uniqueness after delete, inactive member skipped, RLS hides other workspace team_members.
- [ ] Invitations bulk create (results per email), `POST /invitations/{inv}/deliveries` (resend rotation), revoke, preview by header token, daily caps. Test: all result kinds, `accept_url` only when mail unconfigured, delivery on accepted 422.
- [ ] Accept via `POST /user/memberships {invitation_token}`: logged-in match, mismatch 403, unauthenticated 401, register/magic link/OAuth with invitation, reactivation of deactivated member, team assignment, app token accept. Test each.
- [ ] Mailables: Invitation, InvitationAccepted, OwnershipTransferred, MemberDeactivated, TwoFactorRequired, WorkspaceDeletion*, WorkspaceRestored, WorkspaceSuspended, OwnerPromotedBySystem.

## M8. Tokens and /me

- [ ] Token guard (checksum, function lookup, cache, member active, effective permissions = role ∩ groups, `dmi_oat_` to 09 `ResolveAccessToken`). Test: bad checksum no DB query (query counter), expired, revoked, demotion shrinks power; token of workspace A with `Dimmi-Workspace: ws_B` header still scoped to A (header ignored for tokens), B ids 404; deactivated member's tokens 401 within cache TTL.
- [ ] Tokens create/list/revoke/abilities endpoints (abilities = permission groups with `allowed`), recent auth, limits, `TokenCreatedMail`, `NotifyExpiringTokens`. Test: plain token only in create response; `tokens:manage_any` revoke others; group role cannot use 422.
- [ ] `GET /api/v1/me` (`member.permissions`, `auth.auth_methods`, `authenticated_at`, derived `two_factor_used`, `instance.api_version`, `region`) + `/api/v1/user` (show/update) + `PUT /user/avatar` (sniff, re-encode, size). Test: session or app without header returns workspaces list; token returns token block and narrowed permissions; app returns `auth.type: app` with grant auth context; permissions change after role change.
- [ ] `accounts` MCP toolset (09 rules): `dimmi_list_members`, `dimmi_list_teams`, `dimmi_update_availability`. MCP integration test: ability enforcement, schema from OpenAPI.

## M9. Audit and security events

- [ ] `RecordAuditLog` + `WriteAuditLog` listener + redactor; wire events from this spec. Test: rollback removes audit row with action; redaction.
- [ ] Audit API list/show/filters (`filter[via]` incl. `ogr_`) + ascending `sort=created_at` cursor + CSV streaming + range cap. `PruneAuditLogs` calls `RetentionGuard`; `forward_audit_logs` listener calls `AuditSink` (noop). Test: 1M-row seeded benchmark p95 under 50 ms (CI benchmark job); ascending cursor stable; `via_kind = app` row accepted; fake guard false skips prune.
- [ ] Security events writer (registry types incl. `auth_factor.*`, rate-limited failure rows) + `/user/security-events`. Prune jobs. Test.

## M10. Email change, account deletion (track 07 accounts-settings)

- [x] `POST /email-changes` + `PATCH /email-changes/{ott} {token}` with both mails, race 409. Test: second redemption 422, wrong token 422.
- [x] Deletion preview + delete account (anonymize, delete factors, cascade, workspace deletion, `RevokeUserAppGrants(user_deleted)`, events). Test: sole_owner 409, re-signup same email works, factors gone.

## M11. Self-host first run and CLI

- [ ] `CreateFirstUser` action for 11 `CompleteSetup` and `user:create` (token, endpoints, banner owned by 11). Test: concurrent setup + `user:create` one wins under shared advisory lock; initializers run once.
- [ ] CLI `user:create`, `user:list`, `user:reset-password`, `user:disable-2fa`, `user:verify-email`, `user:logout`. Test via command harness.
- [ ] CLI `workspace:list/suspend/unsuspend/restore/purge`, `token:create`. Test.
- [ ] Signup mode `invite_only` default after first user. Test.

## M11b. First-party app sessions (with 09)

- [ ] `GET /sessions` merges 09 `ListUserAppGrants(userID, currentGrantID)` (`kind: app`, `ogr_`, device name, platform, app version, last IP, last used, `auth_methods`, `authenticated_at` from `auth_time`; `current` for caller grant; country via local GeoIP). Test: third-party grants never listed; app caller sees own grant current.
- [ ] `DELETE /sessions/{ogr}` and `DELETE /sessions/current` from app via 09 `RevokeAppGrant(userID, grantID, user)`; `DELETE /sessions` via `RevokeUserAppGrants(logout_others, except current)`. Test: app token 401 after self-revoke, other installs untouched, foreign/third-party `ogr_` 404.
- [ ] `RevokeUserAppGrants` hooks in caller tx: password change (`password_changed`, keep caller grant), password reset (`password_reset`, all), 2FA disable (`two_factor_disabled`, keep caller grant), account delete (`user_deleted`), `user:logout` (`admin`), `user:reset-password --password` (`password_reset`), pre-hijack defense (`password_changed`). Test each with fake grants; rollback of caller tx keeps grants.

## M12. Cloud abuse and hardening

- [ ] Disposable list embed + modes (off/flag/block), velocity flags. Test.
- [ ] Turnstile driver (siteverify via `a.HTTP`) on `POST /users` + `POST /magic-links` + post-spray `POST /sessions`. Test fail-closed/fail-open rules.
- [ ] Member kick + session revoke via centrifuge: `DisconnectMemberSockets` (`node.Disconnect(userID)` over Redis broker on Valkey), `DisconnectSessionSockets` (connection meta `ses_`), dashboard connection `ExpireAt` 5 min + `OnRefresh` re-check. Test with testkit broadcaster (real in-memory centrifuge node): E57, E157, E185, E264, E265.
- [ ] Prometheus metrics and log redaction review (grep tests: no `dmi_live_` or cookie values in captured logs).
- [ ] Security review checklist: OWASP ASVS L2 auth/session sections walkthrough, recorded in PR.
- [ ] Event catalog rows added to `00-foundation/events-and-realtime.md` (via foundation owner PR).
- [ ] Route lint: no path from rest.md old column in routes or OpenAPI (incl. every `/user/two-factor*` path); every route name equals noun path (`sessions.store`, `user.memberships.destroy`, `password-resets.update`, `user.auth-factors.destroy-many`). Test: OpenAPI snapshot.

## M13. Own profile, password, time zone (track 07 accounts-settings)

Migrations range `20261201070001+`. Spec: [settings-pages.md](settings-pages.md). Removes every dead control on `pages/account.tsx` first (delete, then re-add wired).

- [x] `routes/account.go`: `GET|PATCH /user`, `PATCH /user/password`, `PUT|DELETE /user/avatar`; actions `update_profile`, `update_password`, `update_avatar`, `delete_avatar`. Accept: `user.updated` realtime; `tokens_remaining`. Test list in settings-pages.md.
- [x] Profile page wired; `lib/api/account.ts`; email change dialog + `routes/_auth/email.confirm.tsx`. Needs M10 routes. E2e `profile.spec.ts`: "a person changes their name and picture", "a person changes their email and confirms from the new mailbox" (Mailpit).
- [x] Password dialog (change and set). E2e `password.spec.ts`: "changing the password signs out the other browser".
- [x] Time zone picker, `lib/location.ts` uses `/me.user.timezone`. Open: test "the snooze preset uses the person's zone" lands with track 03 W5.

## M14. Sign-in methods page, 2FA help (track 07 accounts-settings)

- [x] `sign-in-methods.tsx` wired to `/auth/config` + `/user/identities`, link + unlink; link callback lands on `?linked=<provider>`. E2e `sign-in-methods.spec.ts` with a local OpenID Connect provider (`tests/e2e/support/identity-provider.ts`, config only).
- [x] Migration `20261201070003_add_two_factor_reset_purpose` (phase=contract: drops and re-adds the CHECK). Permission `members:reset_two_factor`. `POST /members/{mem}/two-factor-resets`, `PATCH /two-factor-resets/{ott}`, cross-organization block, mails.
- [x] `Lockout.Clear`, `DELETE /members/{mem}/sign-in-lock`, `sign_in_locked` on member resource.
- [x] Dialog + `routes/_auth/two-factor-reset.tsx`. E2e `two-factor-help.spec.ts`.

## M15. Preferences, customer-facing name (track 07 accounts-settings)

- [ ] Migration `20261201070001_add_preferences_to_users_table`; `app/auth/preferences.go`; `PATCH /user {preferences}`; `/me` returns.
- [ ] Preferences page: each row per table in settings-pages.md; Available, Send with Enter, signature rows appear only with owner track merged. Hide Notifications nav until track 11. E2e `preferences.spec.ts`: "every control on the preferences page changes something".
- [ ] `customer_facing_name` key, `Workspace.CustomerFacingName()`, workspace page row. Accept: 01 and 04 read through method (their tracks).

## M16. Audit export and scope (track 07 accounts-settings)

Spec: [audit.md](audit.md). Builds on F71 fix.

- [ ] `export_audit_logs.go`, CSV stream, `app/support/csvsafe`, `audit_logs.exported`. Tests in audit.md.
- [ ] `MirrorSecurityEventToAudit` queued listener + label + filter groups in `log-labels.ts`.
- [ ] UI date range + export. Extend `audit-log.spec.ts`.
- [ ] Event names list for other specs in delta verified against what is registered at milestone end; labels added.

## M17. Members, invitations, teams (track 07 accounts-settings)

- [ ] Member resource fields, filters, permission-gated. `features/settings/members/*`. Extend `workspace-members.spec.ts`: "an admin sees who has no second factor".
- [ ] Member CSV export. Test formula guard.
- [ ] `POST /member-imports`, dialog with dry run. Permission `members:import`. E2e `member-import.spec.ts`.
- [ ] Migration `20261201070002_add_reminded_at_to_invitations`, `RemindInviters`, `InvitationPendingMail`, always-return `accept_url`, `{channel: "link"}`, team picker in dialog. Extend `invitations.spec.ts`.
- [ ] `DELETE /teams/{team}` params, dialog, `include=conversation_counts`. 03 `ReassignTeamConversations` built here if track 02 has not; agreed shape in delta. Extend `teams.spec.ts`.
- [ ] `team.member_added` payload `added_by_member_id`. Trigger delivery track 11.
- [ ] X1: `PATCH /members/{mem}` access fields + `member-access-dialog.tsx`, permission `members:manage_access`. **Needs track 02 access milestone** (columns + `VisibleScope`). Land last in M17.

## M18. Public API rate limits (track 07 accounts-settings)

Spec: [rate-limits.md](rate-limits.md).

- [ ] Entitlements catalog + `self_host`, cloud default drivers if still absent (contracts.md). Keys incl. `api.workspace_rate_limit_per_minute`.
- [ ] `ThrottleOptions` `Limit`, `Skip`, `Scope` + framework tests (90% coverage kept).
- [ ] `api_throttle.go`; mount on every authenticated group + `/mcp`; bad-credential bucket in guards. `tests/arch/api_throttle_test.go`.
- [ ] `lib/api/client.ts` 429 handling; tokens page line; docs; `dimmi_api_throttled_total`. Tests in rate-limits.md. `docs/testing/manual-e2e.md` row.

## M19. Custom roles (track 08 enterprise-identity)

Spec: [roles.md](roles.md). Migration `20261201080009_create_roles_table`.

- [ ] Migration, prefix `rol_`, queries, factory `.WithCustomRole`.
- [ ] `EffectivePermissions` with custom set; guard lookup joins; cache bust on `role.updated`. `tests/feature/authz` snapshot.
- [ ] `permissions.Assignable`, labels `lang/en/permissions.json`, `GET /permissions`. `lang:check` covers.
- [ ] CRUD, assign, bulk `PATCH /members`, escalation rules. Tests in roles.md.
- [ ] Finer permissions registered, `roles.go` rows; policy swaps where routes exist. Needs tracks 03, 04 for call sites; missing routes use them when built.
- [ ] Page + editor + presets + member picker. E2e `custom-roles.spec.ts`.

## M20. Interface language (track 07 accounts-settings)

Spec: [i18n.md](i18n.md). Last milestone of track, after wave-1 screens merge (L5).

- [ ] `lib/i18n`, i18next init, `DirectionProvider` wiring, `Accept-Language` from client, `/auth/config.locales`, `PATCH /user {locale}` rule.
- [ ] `lib/i18n/format.ts`; replace ad hoc date and number formatting.
- [ ] Extract strings per namespace (one PR per feature folder), `i18next-cli` config, pseudo locale.
- [ ] Gates in `pnpm lint`; Go test locale dirs. E2e `language.spec.ts`.
- [ ] Human: machine first pass + reviewer per language, each listing in `app.locales` is its own change. Owner approves any LLM run.

## Done when

- `go test ./...` green; every endpoint has feature tests including cross-workspace 404 and ability scope.
- Budgets met: login p95 under 250 ms, guard p95 under 1 ms, `/me` under 20 ms, audit list under 50 ms at 1M rows.
- Demo script (self-host, no SMTP):
  1. `docker compose up`; log shows 11 setup line `Open http://localhost:8080/setup?token=dmi_setup_...`.
  2. `curl /api/v1/setup` shows `needs_setup: true`. POST setup with token: owner Ada + workspace Acme created, session cookie set.
  3. `POST /api/v1/users` as outsider: 403 `signup_closed`.
  4. Ada invites `grace@acme.com` as admin: response has `accept_url` (mail unconfigured); log shows InvitationMail WARN line.
  5. Grace registers with invitation token: email verified, member admin, lands in Acme.
  6. Ada adds TOTP (`POST /user/auth-factors {type: totp}` then `PATCH /user/auth-factors/{fac} {code}`; recovery codes shown once), `DELETE /sessions/current`, `POST /sessions {grant: password}`: 200 `two_factor_challenge` with `methods` objects, completes with `{grant: two_factor, method: totp, code}`; `GET /sessions` shows `auth_methods: [password, totp]`.
  7. Ada sets `security.require_two_factor` (`PATCH /workspace {require_two_factor: true}`); Grace's next request 403 `two_factor_required` (`requirement: two_factor`, `action_url`); Grace adds factor, works. Ada tries `DELETE /user/auth-factors`: 409 `two_factor_required_by_workspace`.
  8. Grace creates token `conversations:read`; `curl -H "Authorization: Bearer dmi_live_..." /api/v1/me` shows abilities and `member.permissions: [conversations:read]`; `members:write` call 403 `missing_ability`.
  9. Ada `PATCH /members/{grace} {status: deactivated}`: Grace's token 401 within 2 s, socket closed within 5 s, audit log shows `member.deactivated` with IP.
  10. Ada `DELETE /workspace`, then `PATCH /workspaces/{ws} {trashed: false}`; `GET /api/v1/audit-logs` shows `workspace.deleted`, `workspace.restored`.
  11. `dimmi user:disable-2fa ada@acme.com` works with app running (overrides workspace requirement; Ada then 403 `two_factor_required` until new factor).
- Cloud demo: GitHub OAuth signup to workspace in 3 requests; disposable email blocked; 11th wrong password 429 with `Retry-After`.
