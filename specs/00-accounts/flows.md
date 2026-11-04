# 00 Accounts: flows

Style: caveman. Exact behavior per flow. Endpoint paths from [api.md](api.md) (REST per `00-foundation/rest.md`). Updated 2026-11-02 for `00-foundation/extensibility.md` (factors, providers, login policy, access policy, session auth context, organizations, permissions).

"Recent auth" (`password.confirm`) = browser: `sessions.confirmed_at` within `auth.password_confirmation.ttl` (15 min); first-party app: `now - principal.AuthTime <= ttl` (09 `ResolveAccessToken` principal; app re-auths in browser sheet with `max_age=0`, flow 24) or app confirmation via `POST /user/confirmation` (stored on 09 grant `confirmed_at` / `confirmed_method`, principal `ConfirmedAt`; see section 8). Missing returns 423 `password_confirmation_required`.

## 0. Common building blocks

### StartSession (action `auth/start_session.go`)
`StartSession(user, primaryMethod, factorMethod?, opts{reauth})`:
1. Set long-lived signed `__Host-dimmi_device` cookie (random device id, 1 year) if absent. `device_id` = SHA-256 hex of cookie value. Used for lockout bypass, new-device detection, session/grant/push join.
2. Generate 32-byte cookie value, insert `sessions` row: hash, user, `auth_method` = primary method, `auth_methods` = `{primary}` or `{primary, factor}` (e.g. `{password,totp}`), `authenticated_at = now()`, `confirmed_at = now()`, `confirmed_method` = last method of `auth_methods`, `device_id`, IP, UA, device label, country, `expires_at = now() + 30 d`.
3. Delete previous session row of this browser if request carried valid session cookie (rotation, fixation defense; reauth always lands here).
4. Set cookie `__Host-dimmi_session` (`Path=/; Secure; HttpOnly; SameSite=Lax`, `Max-Age` = absolute lifetime). No CSRF token cookie (CSRF = `http.CrossOriginProtection`, header based).
5. Write `security_events` `login.succeeded` (metadata `auth_methods`, `ses_` id, `reauth`).
6. New-device check: `device_id` never seen for this user in `security_events` last 90 days AND user has at least one earlier login: queue `NewSignInMail`. First login after signup never mails. Reauth never mails.
7. Over 100 active sessions for user: delete oldest by `last_activity_at`.

### LoginPolicy (every primary method)
`LoginPolicyResolver.ForEmail(email)` runs first in every primary method, before any secret check: password grant, magic link request and consume, email code, OAuth callback (after provider identity resolved, before link/session), password reset redeem, register. Driver `OrganizationLoginPolicy` (`21-enterprise-identity`): email under verified domain of enforced connection: 409 `sso_required` with `redirect_url` (domain-level answer, no account enumeration); otherwise all methods allowed. Owner `sso_bypass` per 19 README. Contract in README Support.

### Primary auth then 2FA (`CompletePrimaryAuth(user, method, opts{reauth})`)
Any primary method (password, magic link, email code, OAuth, password reset) ends here:
- User has no 2FA (`two_factor_confirmed_at` null): `StartSession(user, method)`; response 201 `MeResource` (`PATCH /password-resets/{prs}`: 200).
- Else: load active factors of enabled drivers with `SecondFactor` (recovery set only when `remaining > 0`); each driver `BeginChallenge` (nil options for totp/recovery_code). Create `one_time_tokens` purpose `two_factor_challenge` (5 min, data `{primary_method, methods, factor_ids, driver_state, invitation_id?, redirect?, reauth_session_id?, device_id}`), set cookie `__Host-dimmi_2fa` (token, HttpOnly, 5 min), respond 200 `two_factor_challenge` with `methods` array of objects (`method`, `factor_id`, `name`, `preferred`, `options`; preferred first). OAuth callback redirects to `/login/two-factor`. No session yet.
- 2FA on but no usable factor (drivers disabled by config, recovery exhausted, only disabled-type factors): challenge still issued with remaining methods; empty list: 200 challenge with `methods: []` and SPA shows "contact admin / use `dimmi user:disable-2fa`". Never silently skips 2FA.

### Guards
Caller class decided by lookup, never by client claim (foundation `api.md` "Who calls what").
- Session guard (`auth:session`): cookie -> SHA-256 -> otter cache (30 s) -> `sessions` row; reject expired (absolute or idle). User `deleted_at` set: reject.
- Bearer `dmi_live_`/`dmi_test_` (`auth:token`): verify CRC32 locally (bad checksum = 401 without DB hit), hash, `auth_find_token`, reject revoked/expired, load member, member must be `active`, user not deleted.
- Bearer `dmi_oat_`: hand to 09 `oauth.ResolveAccessToken`. Returns `auth:app` principal (first-party grant, null workspace: acts as user, workspace from `Dimmi-Workspace`, role permissions like session; principal `AuthTime`, grant `auth_methods`, `device_id`) or `auth:token` principal (workspace grant: scopes + role permissions).
- Other bearer: 401 `invalid_token`.
- `auth:user` accepts session or app; PAT / third-party OAuth: 403 `first_party_only`. `auth` accepts all.
- Guard (`app/auth/guards.go`, implements `auth.Guard`) builds `contracts.Principal` (`UserID`, `AuthMethods`, `AuthenticatedAt`, `ConfirmedAt`, `DeviceID`, `IP`, `Via`, `TwoFactorEnabled`) and, after `workspace` middleware, `authz.Principal` (+ member + effective permissions). Cached with credential (30 s).
- Cache busted by Valkey pubsub control message `auth.bust {kind, id}` (`a.PubSub`, `*pubsub.Bus` on Valkey) on revoke, deactivation, role change, user deletion, factor change that flips 2FA, `security.require_two_factor` change. App grant revocation busts via 09 `oauth_grant.revoked` listener.

### Authorization (`authz`)
- Policies call `authz.Can(principal, "<permission>")`. Effective permissions: session/app = `roles.go[role]`; token = `roles.go[role]` ∩ expand(abilities) (`*` = all groups). Computed once per request after `workspace` middleware.
- `require_permission.go` (`can:<perm>`) for route-level checks; field-level via policy `UpdateField`.
- Role names only in `roles.go` and role-change actions (`UpdateMemberRole`, `TransferOwnership`, `EnsureWorkspaceHasOwner`, last-owner checks).

### Access policy (`access.policy`, replaces `two_factor.enforced`, same position in `W` stack)
- After `workspace` middleware: `AccessPolicy.Evaluate(ctx, principal, workspaceID)`. MVP driver: one rule `RequireTwoFactor` (workspace setting `security.require_two_factor` read through settings registry; user `two_factor_confirmed_at` from cached guard user). Applies to session, app and token (tokens of member without 2FA stop until member sets up).
- Failure: 403 `{code: "two_factor_required", message, requirement: "two_factor", action_url: "/settings/security/two-factor"}`. Generic shape for every later rule (`reauthentication_required` from `authenticated_at` age, `sso_required`, `ip_not_allowed`): new rule struct, no middleware change.
- No extra query (principal + workspace settings already cached). Exempt routes (not in `W` group): `/me`, `/user`, `/user/*` (incl. `/user/auth-factors`, `DELETE /user/memberships/{mem}` leave), `/sessions`, `/email-*`, guest routes.

### Organizations (inside workspace lifecycle)
- `CreateWorkspace`: insert `organizations` (name = workspace name, `created_by_user_id`) then workspace with `organization_id`, same tx, before initializers (12 initializer creates billing account for org if absent). Always new org at MVP.
- `UpdateWorkspace` rename: org name updated same tx while org has one workspace.
- `PurgeWorkspace`: after workspace row gone, org with no live workspace gets `deleted_at = now()` (row kept for billing retention).
- `workspace:repair`: ensures workspace's org row live (clears `deleted_at` if workspace live) and name synced.
- Never in API resources (no `organization_id`), no org endpoints.

### First-party app login (09 owns)
Dimmi mobile never calls `POST /sessions`, `POST /users`, `POST /magic-links` or `/password-resets`. It opens system browser sheet on `/oauth/authorize` (client `dimmi-mobile`, PKCE S256, optional `device_id` = app install id); sheet shows normal web `/login`, which uses flows 3 to 7 below and creates a browser session in the sheet; 09 then issues code and user-wide first-party grant (`ogr_`; `auth_time` = session `authenticated_at`, `auth_methods` copied from session). From then on the app calls `U` and `A` routes with `dmi_oat_` bearer. Logout from app: `DELETE /sessions/current` (section 23). Sudo re-auth: `max_age=0` (section 24).

## 1. Register with email + password

1. `POST /api/v1/users {name, email, password, workspace_name?, invitation_token?, captcha_token?}` (browser sends `Sec-Fetch-Site: same-origin`; CrossOriginProtection passes; no CSRF preflight call).
2. Checks in order: setup completed (else 503 `setup_required`, returned by 11 `setup_guard`); `LoginPolicyResolver.ForEmail` (MVP allow); captcha when driver set (422 `captcha_failed`); signup allowed: mode `open`, or valid invitation token whose email equals request email (else 403 `signup_closed`); email syntax + IDNA; disposable list (cloud block: 422 `errors.email` code `disposable_email`; flag mode: allow, add `risk_flags`); email not taken by non-deleted user (422 `email.unique`); password rule (min length, max 256, not in common list, not equal email, breach check on cloud); IP velocity cloud 5 signups/hour (429).
3. Tx: `CreateUser` (argon2id hash, `created_via=register`, NFC name). If invitation token valid: mark email verified (token proves inbox), `AcceptInvitation`. Else if `workspace_name`: `CreateWorkspace` (org + workspace, runs workspace initializer hook). Dispatch `user.registered`.
4. If email unverified: `SendEmailVerification` (section 2).
5. `StartSession` (method `password`; or `invitation`). Response 201 `MeResource` (workspace null unless created/joined) with `workspaces` list.

## 2. Email verification

- `SendEmailVerification`: create `one_time_tokens` purpose `email_verification` (24 h, `email` = current user email, no code). Mail `VerifyEmailMail` link `https://{app}/verify-email#id=ott_...&token=...`. Earlier unconsumed verification tokens stay valid until expiry, max 3 active (oldest invalidated).
- SPA sends `PATCH /api/v1/email-verifications/{ott} {token}`. No session required (cross-device works); if session for other user present, still verifies target user, response carries `user_id`.
- Lookup by id + purpose, constant-time hash compare. Unknown id, wrong purpose or wrong token: 422 `errors.token` `invalid_verification_token` (same timing; 5 wrong tries burn the token).
- Token `email` differs from user's current email: 410 `verification_link_stale`.
- User already verified (any way: earlier click, magic link, OAuth, CLI): 200 `email_verification` resource, `status: verified`, no event (idempotent, also when token consumed or expired).
- Not verified, token expired: 410 `verification_link_expired`. Valid: set `consumed_at`, `email_verified_at = now()`, dispatch `user.email_verified`, 200.
- Resend: `POST /api/v1/email-verifications` (U), 1/min, 6/hour, 202. Already verified: 200 no-op, no mail.
- Gated while unverified (`verified` middleware): create invitations, create tokens, channel connect endpoints (04/05). Cloud only unless `auth.verification.required`. Everything else works (inbox, widget test).

## 3. Login with password

1. `POST /api/v1/sessions {grant: "password", email, password, captcha_token?}`.
2. `LoginPolicyResolver.ForEmail(email)` (MVP allow). Lockout check before hashing (see section 12). Locked: 429 `too_many_attempts` + `Retry-After`.
3. Find user by email (not deleted). Unknown or no `password_hash`: run dummy argon2 verify (same params), count failure, 422 `invalid_credentials`. Same for wrong password.
4. Correct: clear IP+email counter; `NeedsRehash` then rehash in same request. `CompletePrimaryAuth(user, password, {reauth})` (reauth rules section 24).
5. Response 201 `MeResource` + session cookie, or 200 `two_factor_challenge` + challenge cookie.

## 4. 2FA challenge

`POST /api/v1/sessions {grant: "two_factor", method, code}` with `__Host-dimmi_2fa` cookie. `method` required (no default; pre-launch, no compat). Later `{method: "webauthn", credential}`.
- Cookie missing/expired/consumed: 401 `two_factor_challenge_expired`; client restarts login.
- `method` missing: 422 `errors.method` `required`. `method` not in challenge `data.methods` (not enrolled, driver disabled after challenge, recovery set exhausted): 422 `errors.method` `method_not_available`; no attempt counted (not a guess).
- Dispatch: `factors.Find(method).Verify(factor, driver_state, input)` against offered factors of that type (several factors of one type later: try each, constant time). Factor row locked `FOR UPDATE` (replay and recovery-code races).
- `totp`: SHA1, 6 digits, 30 s step, accept steps t-1, t, t+1. Step must be greater than factor `last_counter` (replay: 422 `invalid_code`). Store step in `last_counter`.
- `recovery_code`: normalize (lowercase, strip spaces/dashes), compare SHA-256 against decrypted list, mark used, `credential.remaining--`, write `auth_factor.recovery_code_used` (`{factor_id, type}`), mail `RecoveryCodeUsedMail` with remaining count (2 or fewer: "regenerate"; 0: "no codes left, regenerate now"). Row kept at `remaining = 0` (shown in list, omitted from later challenges).
- Wrong: `attempts++`; at 5 challenge burned (401 `two_factor_challenge_expired`), counts toward IP+email lockout.
- Success: consume challenge, factor `last_used_at = now()`, `StartSession(user, primary_method, method)`: session `auth_methods = {primary, method}`, `authenticated_at = now()`, `confirmed_method = method`. Continue stored intent (invitation accept, OAuth link, reauth). 201 `MeResource`.

## 5. Magic link (login and signup)

1. `POST /api/v1/magic-links {email, invitation_token?, captcha_token?}`. `LoginPolicyResolver.ForEmail` first (MVP allow). Always 202 `{"message":"If this email can sign in, a link is on its way."}`.
2. Behind response: rate limit 3 per 10 min per email, 20/hour per IP (silently drop over limit; still 202). Existing user: create `one_time_tokens` magic_link (15 min, token + 6-digit code). Unknown email: signup allowed (same rules as register) then token with `user_id null`; else send nothing.
3. Mail `MagicLinkMail`: button `https://{app}/login/magic#token=...` + code `123-456` + requester IP/device + "ignore if not you". Previous unconsumed magic links for email stay valid until expiry (multiple tabs), max 3 active (oldest invalidated).
4. Consume: `POST /api/v1/sessions {grant: "magic_link", token, reauth?}` or `{grant: "email_code", email, code, reauth?}`. `LoginPolicyResolver.ForEmail` again (policy may change between request and consume). Atomic consume. Code path: wrong code `attempts++`, 5 attempts burns token (422 `invalid_code`). Expired/consumed: 410 `magic_link_expired`.
5. Unknown-email token: create user (`created_via=magic_link`, name from request if provided else local part title-cased, email verified, no password). Existing user: set `email_verified_at` if null (link proves inbox). Pre-hijack defense: when account was unverified and had password, password may belong to attacker who registered victim email first: clear password, revoke all other sessions, first-party app grants (09 `RevokeUserAppGrants(pre_hijack_defense)`) and tokens, mail `PasswordChangedMail` ("password removed, set new one in settings").
6. `CompletePrimaryAuth(user, magic_link | email_code)` (`sessions.auth_method` records which). Disabled when `auth.magic_link.enabled=false`: `POST /magic-links` 404, grants `magic_link`/`email_code` 422 `grant_not_allowed`.

## 6. OAuth login (identity providers: GitHub, Google at MVP)

Provider code path generic: `ProviderResolver.Find(key)` returns `IdentityProvider` driver; controllers and `HandleOAuthCallback` never branch on provider name. New provider = driver + `auth.oauth.<key>.*` config block.

Redirect: `GET /auth/{provider}/redirect?intent=login|link|reauth&invitation=...&redirect=/path`. Browser-only web route.
- Provider key unknown or not configured: 404 page. `intent=link` requires browser session (`auth:session`) + recent auth. App users link from settings page opened in browser.
- `intent=reauth` requires browser session (section 24). Generate `state` (32 bytes) and PKCE verifier (S256), nonce for OIDC providers (`Protocol() == "oidc"`). Store `{state, verifier, nonce, intent, invitation_token, redirect, user_id?}` in encrypted cookie `__Host-dimmi_oauth` (10 min, `SameSite=Lax` so it returns on top-level redirect).
- `driver.AuthorizeURL(req)`. GitHub driver (`oauth2`): `https://github.com/login/oauth/authorize` scope `read:user user:email`, `code_challenge`, `code_challenge_method=S256` (GitHub supports PKCE for OAuth apps since 2025-07-14). Google driver (`oidc`): discovery doc, scope `openid email profile`, `prompt=select_account`, nonce, PKCE S256.

Callback `GET /auth/{provider}/callback?code&state`:
1. State mismatch/missing cookie: redirect `/login?error=oauth_state`. Provider `error=access_denied`: `/login?error=oauth_denied`.
2. `driver.Callback(r, req)` returns `ExternalIdentity` (`framework/socialite` driver exchange via `a.HTTP`, 10 s timeout). GitHub: `GET /user` (id, login, name, avatar_url) + `GET /user/emails`, pick `primary && verified`; none: `/login?error=oauth_email_unverified`. Google: verify id_token (iss, aud, exp, nonce) via `framework/socialite` `google` driver (coreos/go-oidc v3); identity key `sub`; email + `email_verified` + `hd`.
3. `LoginPolicyResolver.ForEmail(identity.Email)` (MVP allow). Resolve by `(provider = identity.Provider, provider_user_id = identity.Subject)`:
   - Identity `(provider, provider_user_id)` exists: that user. Update email/username/avatar url/last_login_at.
   - `intent=reauth`: resolved user must equal session user, else `/login?error=reauth_user_mismatch` (nothing changes). Else continue to step 5 with reauth.
   - `intent=link`: identity owned by another user: `/settings/security?error=identity_taken`. Else attach to session user, mail `IdentityLinkedMail`, security event. Done (no new session).
   - No identity, provider email verified, Dimmi user with same email exists: auto-link. If Dimmi user unverified: pre-hijack defense (clear password, revoke all sessions, first-party app grants (`RevokeUserAppGrants(pre_hijack_defense)`) and tokens, mark verified). Mail `IdentityLinkedMail` either way.
   - No user: signup allowed? (open, invitation for same email) create user (`created_via` = provider key, verified, no password, name from provider, `FetchUserAvatar` job). Else `/login?error=signup_closed`.
4. Pending invitation in cookie: accept if emails match (else `/invitation?error=email_mismatch#token=...`).
5. `CompletePrimaryAuth(user, provider key, {reauth})`. Redirect to `redirect` (only same-origin relative paths starting with `/` and not `//`; else `/`), or `/login/two-factor`.

Unlink: `DELETE /api/v1/user/identities/{idn}` recent auth. Allowed when user keeps another method: password set, other identity, or magic link enabled and email verified. Else 409 `last_login_method`.

## 7. Password reset

1. `POST /api/v1/password-resets {email}`. Always 202. Existing non-deleted user: upsert `password_reset_tokens` (new `prs_` id + token, 60 min), mail `ResetPasswordMail` link `/reset-password#id=prs_...&token=...`. Throttle 1/min and 5/hour per email, 20/hour per IP (silent). OAuth-only user also gets link (lets them add password).
2. `PATCH /api/v1/password-resets/{prs} {token, password}`: lookup by id, constant-time hash compare (unknown id: dummy compare), not expired, else 422 `errors.token` `invalid_reset_token` (identical body and timing for unknown id, wrong token, expired; 5 wrong tries delete row). `LoginPolicyResolver.ForEmail` (MVP allow). Password rule. Tx: set hash, `password_changed_at`, `email_verified_at` if null, delete reset row, revoke all sessions, revoke all first-party app grants (09 `RevokeUserAppGrants(userID, password_reset)`, no exception), security event `password.reset`, mail `PasswordChangedMail`.
3. `CompletePrimaryAuth(user, password_reset)` (2FA still required): 200 `MeResource` + session cookie, or 200 `two_factor_challenge`.

## 8. Password confirmation (sudo mode)

- `GET /api/v1/user/confirmation` returns `{object: "confirmation", method, confirmed_until}`; 404 when none or expired. Browser: `sessions.confirmed_at` + ttl, `method` = `confirmed_method`. App: `confirmed_until` = max(principal `AuthTime`, grant confirmation) + ttl; `method` = `reauth` when from `AuthTime`.
- `POST /api/v1/user/confirmation {method, password | code}`. `method` in `password` (users with password), `email_code` (users without password), or any factor type whose driver has `Confirmation` and user has active factor of that type (MVP `totp`; recovery codes not accepted). Other value: 422 `errors.method` `method_not_available`. Factor path: `factors.Find(method).Verify` (same replay rules, updates `last_counter`, `last_used_at`). Wrong: 422 `errors.password` / `errors.code` `invalid_code`; counts toward lockout.
- `POST /api/v1/user/confirmation-codes` sends 6-digit code (`one_time_tokens` confirm_code, 10 min, 5 attempts) to user email, 202. Only for users without password (others 422 `errors.method`).
- Success: browser `sessions.confirmed_at = now()`, `confirmed_method = method` (`authenticated_at` unchanged: confirmation is not a full login). App: 09 `oauth.ConfirmGrant(grantID, method)` sets grant `confirmed_at`, `confirmed_method` (09 README `oauth_grants`); principal `ConfirmedAt` carries it. Browser-sheet re-auth (`max_age=0`) still works (fresh `AuthTime`). Required by: token create, factor add/remove (`POST` / `DELETE /user/auth-factors*`), email change, identity link/unlink, account delete, workspace delete/restore, ownership transfer, owner promotion/demotion, `security.require_two_factor` toggle, `DELETE /sessions`.
- `password.confirm` middleware: browser `now - sessions.confirmed_at <= ttl`; app `now - max(principal.AuthTime, principal.ConfirmedAt) <= ttl` (09 resolver). Later policy "sudo needs phishing-resistant" reads `confirmed_method`.

## 9. Change password, change email

Password: `PATCH /api/v1/user/password {current_password, password}`. User without password: `current_password` omitted but recent auth required. Success: rotate current session id (browser; `auth_methods`, `authenticated_at` carried over: not a new login), revoke other sessions and first-party app grants except caller's own (09 `RevokeUserAppGrants(userID, password_changed, except caller grant when app)`), mail `PasswordChangedMail`, security event.

Email change:
1. `POST /api/v1/email-changes {email}` (U, recent auth). New email not taken (422), deliverable rule. Create `one_time_tokens` email_change (24 h, data `new_email`), invalidate earlier pending change. 201 `email_change` resource (`status: pending`, masked new email). Mail `EmailChangeVerificationMail` to new address (`/email/confirm#id=ott_...&token=...`), mail `EmailChangeRequestedMail` to old address (masked new address, "secure account" link to `/settings/security`).
2. `PATCH /api/v1/email-changes/{ott} {token}` (session of same user OR no session; token is proof). Unknown id, wrong token, expired or consumed: 422 `errors.token` `invalid_email_change_token` (same timing; single redemption, no idempotent replay; 5 wrong tries burn it). Re-check uniqueness in tx (409 `email_taken` if raced). Set email, `email_verified_at = now()`. Mail `EmailChangedMail` to old address. Rotate browser session if present. OAuth identities and app grants untouched. Pending invitations to old email not transferred. Unconsumed `email_verification` tokens for old email become stale (410).

## 10. Auth factors: add, activate, manage, remove

All on `/api/v1/user/auth-factors` (U; `auth:user`, session or first-party app). Actions in `app/actions/factors/`. Drivers from `factors.Enabled()`.

1. Add `POST {type: "totp", name?}` (recent auth). Type not enabled: 422 `errors.type`. Active factor of type whose driver `Multiple` false (MVP: second totp): 422 `factor_limit_reached`. Active factors at `auth.factors.max_per_user` (10): 422 `factor_limit_reached`. Earlier pending factor of same type deleted (replaced). Driver `BeginEnrollment`: totp generates 20-byte seed, stores encrypted (`secret_encrypted`, system key, AAD factor id), `credential {algorithm, digits, period}`, status `pending`. 201 `auth_factor` + one-time `setup` (`secret` base32, `otpauth_url` `otpauth://totp/{issuer}:{email}?secret=...&issuer=...&algorithm=SHA1&digits=6&period=30`, `qr_svg`). Pending invisible to login and confirmation; purged after 1 h.
2. Activate `PATCH /{fac} {code}` (no recent auth needed: code proves possession; begin already needed it). Pending only (active: 422 `invalid_transition`). Driver `CompleteEnrollment` verifies code (window ±1, stores step in `last_counter`). Wrong: 422 `errors.code` `invalid_code`; 5 wrong codes delete pending factor. Tx: status `active`, `confirmed_at`, `is_preferred = true` if user has no preferred factor; security event `auth_factor.added`. If this is first factor where `EnablesTwoFactor`: set `users.two_factor_confirmed_at`, create `recovery_code` factor (10 codes `xxxxx-xxxxx` Crockford base32 lowercase, SHA-256 list encrypted, `credential {total: 10, remaining: 10}`), security event `two_factor.enabled`, mail `TwoFactorEnabledMail`, rotate browser session (auth context kept), bust guard cache; response adds one-time `recovery_codes`. Else (2FA already on; later drivers): mail `AuthFactorAddedMail`. 200.
3. Rename / prefer `PATCH /{fac} {name}` or `{preferred: true}` (no recent auth). Name 1..60, NFC. Preferred: unset on other factor in same tx (partial unique). Recovery set: `preferred` 422. Security event `auth_factor.renamed` for name. `code` with other fields: 422 `prohibited`.
4. Regenerate recovery codes `POST {type: "recovery_code"}` (recent auth). 2FA off: 409 `two_factor_not_enabled`. Tx: delete old set, insert new active set (new `fac_` id), security event `auth_factor.recovery_codes_regenerated`. 201 with one-time `codes`; old codes invalid at once.
5. Remove one `DELETE /{fac}` (recent auth). Pending: delete, 204. Recovery set while 2FA on: 409 `recovery_codes_required`. Active factor that is not the last 2FA-enabling one (later drivers): delete, `auth_factor.removed`, 204. Last 2FA-enabling factor: disable path (step 7).
6. Remove all `DELETE /api/v1/user/auth-factors` (recent auth): every factor incl. pending and recovery set; 2FA on: disable path. No factors: 204 no-op.
7. Disable path (`DisableTwoFactor`, shared by 5, 6 and CLI): blocked when user is active member of non-deleted workspace with `security.require_two_factor`: 409 `two_factor_required_by_workspace` with `workspaces` list, nothing removed (CLI `force` skips). Tx: delete factor(s) + recovery set, clear `two_factor_confirmed_at`, security events `auth_factor.removed` + `two_factor.disabled`, 09 `RevokeUserAppGrants(userID, two_factor_disabled, except caller's grant)`, bust guard cache. Browser sessions kept (caller proved recent auth). Mail `TwoFactorDisabledMail`. 204.
8. Lost device + no codes: self-host `dimmi user:disable-2fa` (`RemoveAllAuthFactors` force); cloud: support process (out of scope for code; documented).
9. Factor added from app: app holds `auth:user`, so `POST` / `PATCH` work with app token; `password.confirm` satisfied by fresh `AuthTime` (sheet re-auth `max_age=0`, section 24) or app confirmation. Activation from app: no browser session to rotate; caller's grant untouched.

## 11. Workspace "require 2FA" (`security.require_two_factor`)

- Toggle `PATCH /api/v1/workspace {require_two_factor: true}` (API field; settings registry key `security.require_two_factor`, scope workspace, `Enforceable` for later org lock): session or app only (U), `settings:write`, recent auth, actor must have 2FA enabled (else 409 `two_factor_not_enabled`). Org lock later: 422 `setting_locked`.
- Effect via `AccessPolicy` rule `RequireTwoFactor` (`access.policy` middleware, section 0) on every workspace-scoped request: member user without 2FA gets 403 `two_factor_required` with `requirement: "two_factor"`, `action_url: "/settings/security/two-factor"`. Applies to session, app and token requests (tokens of that member stop until setup). Exempt: routes outside `W` (`/me`, `/user/*` incl. `/user/auth-factors`, `/sessions`, `/email-*`, guest).
- Rule checks user-level 2FA (`two_factor_confirmed_at`), not session `auth_methods` (same behavior as before; "session must have used 2FA" later = new rule).
- On enable: bust caches, queue `TwoFactorRequiredMail` to affected active members. Invitations accepted later: new member lands on setup screen.

## 12. Brute-force protection and lockout

Counters on `ratelimit.Limiter` (`a.Limiter`, `*ratelimit.Limiter` on Valkey via rueidislimiter, shared by all replicas). Valkey holds rebuildable state only; loss resets counters.
| Scope | Rule |
|---|---|
| IP + email | 10 attempts/min. Consecutive failures: 5 -> locked 30 s, 10 -> 5 min, 15 -> 30 min, 20+ -> 1 h each further failure. Reset on success |
| Email (any IP) | 30 failures/hour -> soft lock 15 min: `password` grant rejected except from device cookie that previously logged in successfully as this user. Magic link, OAuth, reset still work. Mail `SuspiciousSignInAttemptsMail` max once per 24 h |
| IP (any email) | 100 failures/hour (credential spray) -> IP blocked from `POST /sessions`, `POST /users`, `POST /magic-links`, `/password-resets` for 1 h; cloud also requires captcha after 20 failures/hour |
| 2FA | 5 per challenge; failures count toward IP + email |
All locked responses identical for existing and unknown emails: 429 `too_many_attempts`, `Retry-After`. Security event `login.locked` for existing users.

## 13. Self-host first run

Owner split: 11-self-host owns setup token (`dmi_setup_`, hash in `system_settings`, 24 h, `IssueSetupToken`, `dimmi setup:link`), `setup_guard`, `GET/POST /api/v1/setup` controller and `system.CompleteSetup` (`11-self-host/operations.md` section 2 steps 7-9). 00-accounts owns user/workspace creation inside it.
1. Boot banner, token lifecycle, multi-replica: 11.
2. `GET /api/v1/setup` (11) returns `{needs_setup, mail_configured, app_url, version}` (no token needed).
3. `CompleteSetup` (11) calls `users.CreateFirstUser {name, email, password, workspace_name, timezone}` inside its tx: user verified, `created_via=setup`, workspace via `CreateWorkspace` (`joined_via=setup`, owner; runs workspace initializer hook). 11 sets `is_instance_admin`, deletes token, then `StartSession(setup)`. Response 201 `MeResource`. Users exist: 410 `setup_completed`.
4. After setup: `/setup` returns `needs_setup: false`, POST 410. `POST /users` follows `auth.signup.mode` (default `invite_only`).
5. `dimmi user:create` works any time (see api.md CLI); when it creates first user it also completes setup (deletes token).
6. Cloud edition: setup endpoints 404.

## 14. Workspaces: create, switch, update

- Create `POST /api/v1/workspaces {name, slug?, timezone?, locale?, test_mode?}`: session or app (U). Slug auto from name (ASCII fold, kebab, trim to 40, append `-2`.. on conflict); explicit slug conflict 422. Reserved slugs list (`api, app, www, admin, help, docs, status, mail, support, dimmi, auth, login, setup, static, assets, cdn, widget, mcp, oauth, billing`). Limits cloud: owned 10, test_mode 3 (409 `workspace_limit_reached`). Tx (`CreateWorkspace` action): insert `organizations` row (1:1, section 0), insert workspace (`organization_id`), owner member (`joined_via=created`), run workspace initializer hook after workspace row insert (00-foundation/contracts.md, ordered, idempotent, same tx): 03 default inbox "Support", 01 default widget, 04 default forwarding address, 08 default help center (unpublished), 07 default agent (disabled), 06 default notification settings, 12 billing account for org if absent (cloud). Any initializer error aborts whole creation. Dispatch `workspace.created`, audit. Repair: `dimmi workspace:repair` re-runs initializers and ensures live org row. Response 201 workspace (no `organization_id`).
- Switch: client (SPA or app) sends `Dimmi-Workspace: ws_...`. `workspace` middleware resolves membership (active required) and touches `last_accessed_at` (coalesced 5 min). `GET /api/v1/user/memberships` returns `membership` resources (with `workspace`) sorted by `last_accessed_at desc`; first entry = default. No server "current workspace" state (stateless, multi-tab and multi-device safe).
- Update `PATCH /api/v1/workspace {name, slug, timezone, locale, require_two_factor}` (`require_two_factor` maps to setting `security.require_two_factor`, section 11). Rename also renames org while 1:1. Slug change: old slug stored in `settings.previous_slugs` with `until = now()+30 d`, unclaimable until then. `test_mode` immutable after create (422).

## 15. Workspace delete, restore, purge

- `DELETE /api/v1/workspace {confirm_slug}` owner, U, recent auth. `confirm_slug` must equal slug (422). Tx: `deleted_at = now()`, `purge_after = now()+30 d`, `deleted_by_user_id`, revoke all tokens in workspace (set `revoked_at`), pending invitations revoked, dispatch `workspace.deleted` (09 revokes workspace-scoped OAuth grants), audit. Mail `WorkspaceDeletionScheduledMail` to all active owners.
- During grace: workspace hidden from `GET /user/memberships` for non-owners; owners see it only with `filter[trashed]=with|only` (`workspace.deleted_at`, `purge_after` set). All workspace-scoped API returns 404 `workspace_not_found`. Inbound channel webhooks, widget, help center treat as not found (owned by those specs via `workspace.deleted_at` check). Webhooks stop.
- Restore `PATCH /api/v1/workspaces/{ws} {trashed: false}` any active owner, U, recent auth, before `purge_after`. Non-owner: 404 `workspace_not_found` (no existence leak). Not trashed: 200 no-op. Other fields in body: 422 `prohibited`. Clears deleted fields. Tokens stay revoked, invitations stay revoked (documented; prevents zombie access). Mail `WorkspaceRestoredMail`, event `workspace.restored`.
- Reminder: `SendWorkspaceDeletionReminders` 3 days before purge.
- Purge: `PurgeWorkspace` first asks `RetentionGuard.MayDestroy(workspace, {Kind: workspace})` (MVP noop allows); false: skip, record `retention_held`, retry daily. Then chunked: storage files under workspace prefix (via Filesystem contract, batched), then tenant tables in dependency order 5,000 rows per statement (messages, conversations, contacts, ...), then workspace row (cascade remainder), then org `deleted_at = now()` when org has no live workspace (row kept for billing retention). Resumable, idempotent. Slug freed at purge. Billing tables excluded (12).

## 16. Members

All member state changes go through `PATCH /api/v1/members/{mem}` (rest.md rule 3). Policy checks `authz.Can`: `members:write` for member/admin targets, `members:manage_owners` for any owner change (role-change actions keep role logic: last owner, transfer). Body fields `role`, `status`, `reassign_to_member_id` (with `status: deactivated`), `keep_owner` (with `role: owner`). Policy `UpdateField` per field (api.md table). Fields validated together against current state, applied in one tx (`UpdateMember`); any failure rolls back all. Events per changed field in fixed order: `member.reactivated`, `member.role_changed`, `member.deactivated`.
- Role `{role: "member"|"admin"}`: `members:write` sets member<->admin for non-owners. Demoting an owner (incl. self): `members:manage_owners` + U + recent auth. Last-owner protection: lock workspace row `FOR UPDATE`, count active owners; demoting last active owner: 409 `last_owner`. Self-demotion allowed if another owner exists.
- Deactivate `{status: "deactivated", reassign_to_member_id?}`: cannot deactivate self (422 `invalid_transition`; use `DELETE /user/memberships/{mem}`), owner target needs `members:manage_owners` (admin: 403 `forbidden`), last owner rule. Tx: status `deactivated`, `deactivated_at/by`, remove from teams (keep history in audit), revoke member tokens, dispatch `member.deactivated` (with reassign hint), audit. Mail `MemberDeactivatedMail` to user. Sockets kicked under 5 s: AfterCommit listener calls centrifuge `node.Disconnect(userID)` (all nodes via broker Controller; disconnect code `member_removed`, client reconnects and `OnSubscribe` refuses this workspace's channels). First-party app grant of that user stays (user-wide); app requests with this workspace header: 403 `membership_deactivated`.
- Reactivate `{status: "active", role?}`: status active, role kept (owner reactivated as admin unless owner actor sends `role`), no teams restored. `member.reactivated`.
- Ownership transfer `{role: "owner", keep_owner?}`: `members:manage_owners`, U, recent auth (token: 403 `first_party_only`; non-owner: 403 `forbidden`); target active, email verified (409 `target_unverified`), has 2FA when workspace has `security.require_two_factor` (409 `target_two_factor_required`). Tx: target role owner, actor role admin (`keep_owner: true` keeps actor owner = plain promotion). `role: owner` combined with `status`: 422 `invalid_transition`. Mail `OwnershipTransferredMail` to target and actor. Two `member.role_changed` events (one when `keep_owner`) + audit `ownership.transferred`.
- Leave `DELETE /api/v1/user/memberships/{mem}` (U, own membership only, else 404): same as deactivate by self, no mail; last owner: 409 `last_owner`. 204.
- `CheckWorkspaceOwners` daily: workspace (not deleted) with zero active owners (race, account deletion): promote oldest active admin, else oldest active member; audit actor system; mail. Workspace with zero active members: soft-delete (grace applies).
- Availability `PATCH /api/v1/members/me {availability: online|away}`: dispatch `member.availability_changed` (presence). Effective online per foundation (setting AND centrifuge presence on `workspace:{ws_}` within TTL).

## 17. Teams

- Create/update `{name}` 1 to 50 chars, NFC, unique case-insensitive among non-deleted (422). Max 500 teams per workspace.
- Delete (`?reassign_to_team_id=` or `?reassign_to_member_id=`, [settings-pages.md](settings-pages.md)): set `deleted_at`, dispatch `team.deleted` with target; 03 listener reassigns or unassigns conversations (chunked, events), sets `inboxes.default_team_id` null, then calls `PurgeTeam` (deletes team_members, team row). `PurgeDeletedTeams` hourly safety net.
- Membership `POST /api/v1/teams/{team}/members {member_ids: [...]}` (max 100 per call, active members only; inactive skipped with per-id result), `DELETE /api/v1/teams/{team}/members/{mem}`. Idempotent (adding existing = no-op, no event).

## 18. Invitations

- Create `POST /api/v1/invitations {invitations: [{email, role, team_ids}]}` max 50 per request. Requires `verified`. Per email result: `created`, `resent` (pending exists: rotate + resend), `already_member` (active), `reactivation` (deactivated member: invite created, accept reactivates row), `invalid`. Cloud daily cap 100 per workspace (429 `invitation_limit_reached`). Mail `InvitationMail` (`/invitation#token=`), inviter name, workspace name, expiry date. Mail not configured: response includes `accept_url` per invitation (admin copies link).
- Resend `POST /api/v1/invitations/{inv}/deliveries`: pending or expired; rotate token, `expires_at = now()+7 d`, `send_count++`, `last_sent_at`, max 3 per invitation per 24 h (429). Accepted or revoked: 422 `invalid_transition`. 201 `invitation_delivery`.
- Revoke `DELETE /api/v1/invitations/{inv}`: pending only, status `revoked`. Old link shows "invitation revoked".
- Preview `GET /api/v1/invitations/preview` with header `Dimmi-Invitation-Token` (not URL): workspace name, avatar, inviter name, role, masked email `a***@acme.com`, status, `has_account` (true when email has user; acceptable: holder of token already knows email). Expired/revoked/accepted shown with status, 200.
- Accept `POST /api/v1/user/memberships {invitation_token}` (U):
  - Caller email equals invitation email: invitation token proves inbox: set `email_verified_at` if null. 2FA required by workspace and user lacks it: accept anyway, then 403 on workspace routes until setup.
  - Caller email differs: 403 `invitation_email_mismatch` (UI: sign out / sign in as invited email).
  - Not logged in: 401 `unauthenticated` from `auth:user`. SPA uses preview `has_account` to choose: account exists: log in (any method), then retry (token kept client-side); no account: `POST /users` with `invitation_token` (section 1) or magic link / OAuth with invitation (sections 5, 6). Signup mode ignored for invited email.
  - Tx: lock invitation row, status pending and not expired (else 410 `invitation_expired` / `invitation_revoked` / 409 `invitation_already_accepted`). Insert member or reactivate deactivated row with invitation role; add to existing non-deleted teams in `team_ids`; status accepted; `member.joined` (`joined_via=invitation`), audit, `InvitationAcceptedMail` to inviter (if still active). 201 `membership`.

## 19. Domain join: Out (later)

Deferred (`domain-join`, audit B16). MVP joins workspace by invitation only. Section number kept so later refs stay stable.

## 20. Personal access tokens

- Create `POST /api/v1/tokens {name, abilities, expires_at?}`: session or app only (U), `verified`, recent auth. Abilities subset of catalog, and every ability group must intersect caller's role permissions (no group the role cannot use at all; 422 `errors.abilities` `ability_not_allowed` listing offenders). Effective power re-computed per request (role permissions ∩ groups). `expires_at` future and at most 5 years (null allowed). Max 50 active per member (409 `token_limit_reached`). Response 201 with `plain_text_token` once. Mail `TokenCreatedMail` (name, abilities, IP). Audit + security event.
- List `GET /api/v1/tokens` own; `?filter[member]=all|mem_...` needs `tokens:manage_any`. Never returns hash or secret.
- Revoke `DELETE /api/v1/tokens/{tok}`: own, or any with `tokens:manage_any`. Sets `revoked_at`, busts cache, `token.revoked`, audit. Revoked tokens visible 90 days with `revoked_at`.
- Usage: guard records `last_used_at`, `last_used_ip` via coalescer (60 s).
- Expiry: 7 days before `expires_at` `NotifyExpiringTokens` mails member once. Expired: 401 `token_expired`.

## 21. Account deletion

1. `GET /api/v1/user/deletion-preview`: lists workspaces where user is sole active owner with other active members (`blocking`), workspaces where user is only active member (`will_be_deleted`).
2. `DELETE /api/v1/user {confirm_email}` recent auth. Any `blocking`: 409 `sole_owner` with list. Tx: for `will_be_deleted` run `DeleteWorkspace` (grace applies, owner restore impossible after account gone: purge proceeds); deactivate all memberships (with events, conversations unassigned via 03); revoke tokens; 09 `RevokeUserAppGrants(userID, user_deleted)`; delete sessions, auth factors, identities, one-time tokens, reset tokens, security events, avatar file (after commit); anonymize user: `name = 'Former teammate'`, `email = 'deleted+<uuid>@users.invalid'`, null password/`two_factor_confirmed_at`/avatar/locale/timezone, `deleted_at = now()`. Dispatch `user.deleted` (09 listener revokes workspace grants). Mail `AccountDeletedMail` to original address (address captured before anonymization). Clear session cookie when browser.
3. Immediate and irreversible. Same email may sign up again as new user.

## 22. Suspension (cloud ops, also CLI on self-host)

`SuspendWorkspace(reason)`: `suspended_at`, `suspended_reason`, mail owners `WorkspaceSuspendedMail`, event `workspace.suspended`, audit `Actor{Type: system}` (Via `cli` when from console command). Effect (enforced in `workspace` middleware): GET allowed, export allowed, billing endpoints allowed, other writes 403 `workspace_suspended`. Inbound handling per data-model (channels store only). Unsuspend reverses.

## 23. Sessions list and revoke (browser + first-party apps)

- `GET /api/v1/sessions` (U): merges own `sessions` rows (`kind: browser`, `ses_` id) and rows from 09 `ListUserAppGrants(userID, currentGrantID)` (`kind: app`, `ogr_` id: device name, platform, app version, last IP, last used, `auth_methods`, `auth_time`, current). Both carry `auth_methods`, `authenticated_at`; `device_id` never exposed. Third-party OAuth grants and PATs never listed (09 connected apps, `/tokens`). `current: true` on caller's own row: browser caller = its session; app caller = its grant. Shape in api.md.
- `DELETE /api/v1/sessions/{ses}` (U):
  - `ses_...` own: delete row, dispatch `session.revoked` (centrifuge connections of that session disconnected, cache bust), security event `session.revoked`. 204.
  - `ogr_...` own first-party grant: 09 `RevokeAppGrant(userID, grantID, user)` (404 when not user's own first-party grant) (access + refresh tokens dead, `oauth_grant.revoked`, app sockets closed). Security event `session.revoked` (metadata `kind: app`). 204.
  - `current`: logout of caller. Browser: delete session row, clear cookie, `logout` security event. App: 09 `RevokeAppGrant(userID, currentGrant, user)` (app then clears keychain). 204.
  - Foreign id, third-party grant id, already revoked: 404 `not_found`.
- `DELETE /api/v1/sessions` (U, recent auth): revoke every own browser session and first-party grant except caller's current one (`RevokeOtherSessions` + 09 `RevokeUserAppGrants(userID, logout_others, except: current grant if caller is app)`), same tx. Security event `sessions.revoked_others`. 204. Also run by password change (section 9); password reset revokes all (section 7); 2FA disable revokes other app grants (section 10).
- App calls (`auth:app`) count against per-user rate bucket shared with sessions.

## 24. Reauth mode (fresh full login on existing session)

Used by 09 `/oauth/authorize` freshness (`prompt=login`, or `max_age` with session `authenticated_at` older than `max_age`; first-party sudo uses `max_age=0`): 302 `/login?intended=<authorize url>&reauth=1`. Also usable by SPA for its own step-up.
1. SPA `/login?reauth=1` shows login form even with valid session (API `guest` routes never bounce authenticated callers), email prefilled and locked to session user.
2. Client sends any primary grant with `reauth: true` (`POST /sessions {grant: "password", email, password, reauth: true}`, `magic_link`/`email_code` with `reauth`, OAuth `intent=reauth`). Without valid session cookie: `reauth` ignored (plain login).
3. Primary auth must resolve to session cookie's user: else 422 `errors.email` `reauth_user_mismatch` (OAuth: `/login?error=reauth_user_mismatch`), session unchanged, counts as failed login for lockout.
4. `CompletePrimaryAuth` with reauth: 2FA challenge whenever user has 2FA (never skipped because session already passed it); challenge `data.reauth_session_id`.
5. Success: `StartSession` rotates: new `ses_` row, `authenticated_at = now()`, `auth_methods` fresh, `confirmed_at = now()`, old row deleted. Security event `login.succeeded` with `reauth: true`, no new-device mail. Redirect to `intended` (same-origin relative only). 09 then reads new `authenticated_at` into `auth_time`.
6. `max_age=0` always forces this path (age 0 never satisfied). `prompt=none` with stale session: 09 answers `login_required`, nothing here.
