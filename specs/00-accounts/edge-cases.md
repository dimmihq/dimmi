# 00 Accounts: edge cases

Style: caveman. Each: case. Decided behavior. Renumbered 2026-11-02 (extensibility seams: factors, providers, login/access policy, reauth, organizations, permissions). Scope trim 2026-11-02: domain join cases (old 179, 225; later: `domain-join`) removed, renumbered. Final sweep 2026-11-02: 28-30 rewritten for `http.CrossOriginProtection`; 261-270 added (CSRF, centrifuge kick, app confirmation, 13-15 permissions); 271-273 added (07 agent actions, 16 imports, 08 GitHub/docs sync permissions). Every case has a test, named in plain words (testing.md).

## Signup and identity

1. Same email registered twice concurrently (`POST /users`). Unique `users.email` (citext) decides; loser gets 422 `email.unique`. No partial workspace or org (one tx).
2. Email with uppercase or Unicode domain (`Ada@Bücher.de`). Local part kept as typed (citext compares case-insensitive), domain converted to IDNA A-label for storage and sending; display shows Unicode. `ada@bücher.de` and `ada@xn--bcher-kva.de` same user.
3. Email with plus tag (`ada+dimmi@acme.com`). Distinct address; no normalization of plus or dots (Gmail dots). Cloud abuse handled by velocity limits, not normalization.
4. Name with emoji, RTL, zero-width chars. NFC normalize, strip control and zero-width chars (except ZWJ inside emoji sequences), trim, 1 to 80 chars after cleanup; empty after cleanup: 422.
5. Password with Unicode. NFKC normalize before hashing (NIST); length counted in code points; max 256.
6. Password equals email or name. 422 `password_too_common`.
7. HIBP down or slow. Fail open after 2 s, log WARN, metric; signup continues.
8. Disposable domain in flag mode. Signup allowed, `risk_flags` += `disposable_email`; 12 may limit free tier; invitations still allowed.
9. Disposable list update. List embedded at build (CC0 `disposable_email_blocklist.conf`); existing users never retro-blocked.
10. Captcha provider down (cloud). Turnstile siteverify timeout 3 s: fail closed for `POST /users` (503 `captcha_unavailable`), fail open for `POST /sessions` (login never captcha-gated except after spray detection).
11. Register while logged in as other user. Allowed; new session replaces old cookie (old session row deleted).
12. Register with invitation token for different email. 422 `errors.email` `invitation_email_mismatch`.
13. Register in `invite_only` mode with valid invitation for same email. Allowed regardless of mode.
14. Workspace name given at register but invitation token too. Invitation wins; workspace_name ignored (no extra workspace, no org).
15. User with zero workspaces after signup (no invitation, no workspace name). `/me` workspace null, `member` null (no `permissions`); SPA shows create/join; workspace routes 400 `workspace_required`.
16. `LoginPolicyResolver` driver returns error (later SSO driver, DB down). Primary method fails closed: 503 `auth_busy` + `Retry-After`, nothing verified, no lockout count. MVP allow-all driver never errors.

## Login, sessions, CSRF (cross-origin protection)

17. Login unknown email (`grant: password`). Dummy argon2 verify, same 422 and timing (enumeration test asserts p50 delta under 10 ms).
18. Login OAuth-only user (no password) with password. Same 422 `invalid_credentials`; no hint.
19. Login deleted (anonymized) user's old email. Unknown email path.
20. Argon2 params raised in config. Rehash on next successful login; old hashes still verify (PHC string carries params).
21. Hash semaphore saturated (credential stuffing). Requests wait max 2 s then 503 `auth_busy` with `Retry-After: 1`; IP spray limiter normally trips earlier.
22. Session cookie stolen and used from new IP. Allowed (no IP binding; mobile networks change IP); session list shows last IP and device; user revokes. New-device mail only on login, not on reuse.
23. Session idle exactly 14 days. Expired (inclusive); 401; row pruned by hourly job.
24. Absolute lifetime reached mid-request. Request finishes; next request 401.
25. Two tabs log in simultaneously. Two sessions, same `device_id`; both valid; rotation deletes only session cookie sent with request.
26. Logout (`DELETE /sessions/current`) from one tab. Session row deleted; other tabs get 401 on next call, SPA redirects to login; sockets of that session closed (session.revoked).
27. Device cookie cleared or blocked (private window). `StartSession` sets new one: new `device_id`, new-device mail rules apply, known-device lockout bypass lost for that browser. Sessions without cookie support never get one (`device_id` null allowed).
28. SPA `POST /sessions` with no prior request. No CSRF token or preflight exists: same-origin fetch sends `Sec-Fetch-Site: same-origin`, `http.CrossOriginProtection` passes. 419 never used.
29. Cross-site POST with valid cookie (SameSite=Lax blocks most; old browsers). `http.CrossOriginProtection` rejects `Sec-Fetch-Site: cross-site` (or `same-site` from untrusted origin), or, header absent, `Origin` host differing from `Host` and not trusted: 403 `cross_origin_request` (JSON via deny handler).
30. Request carries both session cookie and bearer token. Bearer wins; session ignored. Cross-origin check still runs; CLI and apps send neither `Sec-Fetch-Site` nor `Origin`, so pass.
31. Self-host on plain http (`http://192.168.1.10:8080`). Cookie `dimmi_session` without `Secure`; WARN at boot; `__Host-` not used (browsers reject it on http). Device cookie same fallback (`dimmi_device`).
32. Self-host behind reverse proxy not forwarding proto. `app.url` drives cookie mode and links, not request scheme. `trusted_proxies` config (foundation) governs client IP for limits.
33. User has 100 sessions, logs in again. Oldest by `last_activity_at` deleted.
34. Clock skew between replicas. Expiry checks use DB `now()` in queries, not replica clock.
35. `POST /sessions` with unknown `grant` or missing grant fields. 422 (`errors.grant` or field errors); no lockout count.
36. `POST /sessions {grant: "two_factor"}` without challenge cookie (or cookie from expired challenge). 401 `two_factor_challenge_expired`.
37. Password login answered with `two_factor_challenge`, user then sends `grant: password` again. New challenge replaces cookie; old challenge row stays until expiry but cookie gone.
38. `DELETE /sessions/{ses}` with other user's `ses_` or `ogr_`. 404 `not_found` (no existence leak).
39. `DELETE /sessions/{ogr}` for third-party grant (CLI, MCP client). 404 (09 `RevokeAppGrant` refuses non-first-party); managed in 09 connected apps, never in `/sessions`.
40. PAT or third-party OAuth token calls `GET /sessions`, `DELETE /sessions/current` or any `/user/*` write (incl. `/user/auth-factors`). 403 `first_party_only`.
41. Password change from browser. Session id rotated; `auth_methods`, `authenticated_at` carried over (not a fresh login, so 09 `max_age` still sees original time).

## Reauth mode

42. `reauth: true` without session cookie. Ignored; plain login.
43. Reauth primary auth resolves to different user (typed other email, OAuth account of someone else). 422 `errors.email` `reauth_user_mismatch` / `/login?error=reauth_user_mismatch`; session untouched; counts as failure for lockout (password grant).
44. Reauth for user with 2FA. Challenge always issued (reauth never trusts earlier factor); success rotates session with fresh `authenticated_at` and `auth_methods`.
45. Reauth abandoned (user closes sheet). Old session stays valid with old `authenticated_at`; 09 keeps request unapproved; app stays on old grant.
46. `max_age=0` from app while browser session is minutes old. Still forced (age 0 never satisfied); matches 09 sudo re-auth edge case.
47. Reauth with wrong password 5 times. Normal lockout; existing session unaffected (still logged in).
48. OAuth-only user reauths. `intent=reauth` provider redirect; resolved identity must belong to session user. Passwordless magic-link user: `magic_link` grant with `reauth: true` from same browser.

## First-party app sessions

49. App revokes its own session (`DELETE /sessions/current` with app token, user taps "Log out"). 09 `RevokeAppGrant(userID, currentGrant, user)`: access and refresh tokens dead at once; next call 401 `token_revoked`, refresh `invalid_grant`; app clears keychain, shows sign-in. Browser sessions and other app installs untouched. 204.
50. App lists sessions (`GET /sessions` with app token). Rows from 09 `ListUserAppGrants(userID, currentGrantID)`: own grant `current: true`; every browser row `current: false`. `DELETE /sessions` from app keeps that grant (`RevokeUserAppGrants(logout_others, except current)`), revokes all browser sessions and other app grants.
51. App revokes its own grant by id (`DELETE /sessions/ogr_<own>`). Same as `current`.
52. User with only app sessions (browser sheet session expired or logged out; app is only credential). List shows app rows only. Revoking last grant leaves no credential; user logs in again through app sheet or web. No lock-out (login never needs existing session).
53. Two phones, same user. Two `ogr_` rows with different grant `device_id` (09 authorize param); each revocable alone.
54. Password reset while app connected. Reset (guest, `PATCH /password-resets/{prs}`) revokes all browser sessions and all first-party grants (`RevokeUserAppGrants(password_reset)`, no exception), including device that asked for reset. App gets 401 on next call, shows sign-in sheet.
55. Password change from app (`PATCH /user/password` with app token). Caller's grant kept; all browser sessions and other app grants revoked (`password_changed`).
56. 2FA disabled from browser while app connected. All first-party grants revoked (`two_factor_disabled`); browser session kept. Disabled from app: that app's grant kept, others revoked.
57. Member deactivated or leaves a workspace while app connected. Grant stays (user-wide; user may have other workspaces). App requests with that `Dimmi-Workspace` header: 403 `membership_deactivated`. `member.deactivated` kicks that member's realtime connections under 5 s (`DisconnectMemberSockets`: centrifuge `node.Disconnect(userID)`; app reconnects, `OnSubscribe` refuses the deactivated workspace's channels, other workspaces resubscribe); app switches workspace from `GET /user/memberships`. Other workspaces keep working.
58. Account deleted while app connected. `RevokeUserAppGrants(user_deleted)` in delete tx; app 401 after cache bust.
59. Instance admin disables first-party apps (`apps.enabled=false`). New grants refused by 09; existing grants still listed in `/sessions` so user can revoke; requests with existing app tokens refused by 09 resolver (403 `first_party_app_disabled`).
60. App needs sudo (`DELETE /sessions`, `POST /email-changes`, `POST /user/auth-factors`) and principal `AuthTime` older than 15 min. 423 `password_confirmation_required`; app re-auths in browser sheet with `max_age=0` (flow 24) or calls `POST /user/confirmation` (`totp`, `email_code`, `password`; persistence needs 09 grant confirmation columns).
61. App calls workspace route without `Dimmi-Workspace` header, user in two workspaces. 400 `workspace_required` (same as session).
62. App calls `POST /sessions` (should never). Treated as any guest client: browser-style login rules apply (no browser headers, so cross-origin check passes; lockout and captcha apply); app gets no token from it. Documented: app login only via 09 OAuth.
63. App build below `apps.min_versions.ios`. Foundation `client.version`: 426 `client_upgrade_required` with `min_version`, `upgrade_url` on every `/api/v1` call; app shows store link. Header missing (browser, third party): allowed.
64. Newer app against older self-host lacking a capability. `/auth/config.capabilities` lacks token; app hides feature instead of calling missing endpoint.

## Lockout and abuse

65. Attacker hammers victim email from many IPs. Per-email soft lock after 30 failures/hour; victim on known device (`__Host-dimmi_device` with prior success) still logs in; magic link and OAuth unaffected; victim mailed once per 24 h.
66. Attacker sprays one password across many emails from one IP. Per-IP limit 100 failures/hour blocks IP for 1 h on all guest auth endpoints.
67. Users behind shared NAT (office). IP+email keying means coworkers unaffected by one person's typos; spray limit 100/hour high enough for normal office.
68. Lockout counter storage lost (Valkey restart or flush; counters are rebuildable state, not backed up). Counters reset; acceptable; documented. Valkey unreachable: limiter fails closed for login (503 `auth_busy`, `Retry-After: 5`), fails open for read routes; `/ready` reports Valkey down.
69. Successful login after failures. IP+email consecutive counter reset; email-wide and IP-wide hourly counters kept.
70. Correct password while locked. Still 429 (no oracle); counter not incremented.
71. 2FA brute force. 5 attempts per challenge, then new password login needed; failures count toward IP+email lockout. 1M code space x 3 valid steps: max 50 guesses/hour per IP+email.
72. Magic link code brute force across many requested tokens. Max 3 active tokens per email, 5 attempts each, request limit 3 per 10 min: at most 15 guesses per 10 min against 1M space.
73. Magic link request flood to victim inbox. 3 per 10 min per email cap; silent drop; per-IP 20/hour.
74. Invitations used to spam outsiders (cloud). Require verified inviter, 100/day per workspace, no custom message, footer "ignore if unexpected"; complaints tracked by 04 mail reputation; workspace with complaint rate over 0.1% suspended from inviting (403 `invitations_paused`).
75. Bot signups (cloud). Turnstile on `POST /users` and `POST /magic-links`; 5 signups/hour per IP; `risk_flags` `signup_velocity` when /24 subnet exceeds 20/day.

## Magic link, reset, verification

76. Email security scanner prefetches magic link. Token in fragment, consumption only by `POST /sessions` from SPA JS: scanner never consumes.
77. Magic link opened on different device than requester. Works; session created on opening device (its own `device_id`). Mail shows requester IP/device so user can judge. Code alternative (`grant: email_code`) for original device.
78. Magic link clicked twice. Second: 410 `magic_link_expired`; SPA shows "link already used, request new one"; if same browser already has session, SPA just continues.
79. Magic link for unknown email, signup open, user clicks after someone registered same email with password. Consume finds existing user by email (lookup at consume time, not at request), signs in, pre-hijack defense if unverified (first-party grants revoked with reason `pre_hijack_defense`).
80. Password reset requested twice. Only latest link valid (upsert: new `prs_` id, old id gone: 422 `invalid_reset_token`).
81. Reset link used after password already changed by other means. Token row still valid until used or expired: allowed (proves inbox). Old sessions and app grants revoked anyway.
82. Reset for user with 2FA. Password set, then 200 `two_factor_challenge` before session. Reset never removes factors.
83. `PATCH /password-resets/{prs}` with wrong token. 422 `errors.token` `invalid_reset_token`, identical body and timing to unknown `prs_` id (dummy hash compare); 5 wrong tries delete row.
84. `PATCH /password-resets/{prs}` with malformed id (not `prs_`). Same 422 (never 404; no id oracle).
85. Verification link opened after email changed. 410 `verification_link_stale` (token `email` differs from current email).
86. Verification link opened after 24 h, user still unverified. 410 `verification_link_expired`; SPA asks user to log in and resend.
87. `PATCH /email-verifications/{ott}` on already verified user (second click, or verified via magic link/OAuth meanwhile). 200 `status: verified`, no event, no error (idempotent, even when token consumed or expired).
88. `PATCH /email-verifications/{ott}` with `ott_` id of other purpose (email change, magic link, 2FA challenge) or wrong token. 422 `invalid_verification_token`.
89. Verification link for deleted user. Tokens deleted with account: 422 `invalid_verification_token`.
90. `PATCH /email-changes/{ott}` second time. 422 `invalid_email_change_token` (single use; new address already active).
91. Token sent in query string or path (`?token=`). Ignored; body only (rest.md rule 12). Missing body token: 422.
92. Mail driver fails (SMTP down). Mail job retries 8 times over 6 h; API already returned 202; user can resend after 1 min. Auth flows never block on mail send.
93. Email change while TOTP active. `otpauth` label keeps old email in authenticator app (cosmetic; seed unchanged); factor keeps working. Documented.

## OAuth and identity providers

94. OAuth state cookie missing (third-party cookie settings, different browser). Redirect `/login?error=oauth_state`; nothing created.
95. GitHub user with no verified email or private emails only. `/user/emails` (scope `user:email`) still returns them; no primary verified: `oauth_email_unverified`.
96. GitHub primary email changed later. Identity keyed by GitHub id; Dimmi email unchanged; identity row `email` updated for display.
97. Google account email different from Dimmi email, identity already linked. Login by `sub` works; no email update.
98. OAuth email matches existing verified user who has 2FA. Auto-link, then 2FA challenge; mail `IdentityLinkedMail`.
99. Attacker controls GitHub account with victim's stale verified email (expired domain repurchased). Auto-link possible: mitigation is 2FA, link notification mail, and identity listed in settings. Documented risk; Google `hd` and GitHub verified flags only signals available.
100. Link intent while identity belongs to another Dimmi user. Error `identity_taken`; no transfer.
101. Link intent started from app. Not possible with bearer (web route needs browser session); app opens settings page in browser.
102. Provider outage or timeout during token exchange. Redirect `/login?error=oauth_unavailable`; password and magic link still work.
103. Open redirect via `redirect` param. Only relative paths starting with `/` and not `//` or `/\`; else `/`.
104. OAuth signup when `invite_only` and pending invitation exists for provider email but no invitation token in flow. Signup allowed (pending invitation for email counts). Invitation not auto-accepted: user sees pending list (`/user/invitations`) and accepts explicitly.
105. OAuth provider not configured on self-host. Absent from `/auth/config.providers`; redirect endpoint 404.
106. `/auth/{provider}/redirect` with key not in registry (`/auth/microsoft/redirect` before driver ships). 404 page; no code path branches on provider names.
107. Provider removed from config while users have identities with it. Rows kept; login with it 404; unlink still works; `last_login_method` counts only configured providers plus password and magic link.
108. Provider value not in registry reaches DB (bad import). Format CHECK passes, Go registry rejects in action; never inserted by app code. `provider` / `auth_method` / `created_via` new values need driver registration, never migration.

## Auth factors and 2FA

109. TOTP code reused within same 30 s window. Rejected (factor `last_counter`), 422 `invalid_code`.
110. Phone clock 60 s off. Window ±1 step accepts up to about 30-59 s drift; beyond: fails; error message suggests time sync.
111. User starts TOTP enrollment, never confirms. Pending factor invisible to login, challenge and confirmation; replaced by next `POST {type: totp}`; purged after 1 h by `PurgeExpiredAuthRecords`.
112. `PATCH /user/auth-factors/{fac} {code}` on pending factor older than 1 h but not yet purged. 404 `not_found` (treated gone); user starts again.
113. `PATCH /user/auth-factors/{fac} {code}` on active factor. 422 `invalid_transition` (`from: active`, `to: active`).
114. 5 wrong codes on pending factor activation. Pending factor deleted; next `PATCH` 404; user restarts enrollment.
115. Second TOTP while one active (`POST {type: totp}`). 422 `factor_limit_reached` at MVP (driver `Multiple` false). Replace phone: add after removing old, or later when `Multiple` true. Factor list never holds two active TOTPs at MVP.
116. Two pending TOTP enrollments (two tabs). Second `POST` deletes first pending; first tab's `PATCH` 404.
117. User at `auth.factors.max_per_user` (10) active factors. 422 `factor_limit_reached`. Unreachable at MVP (max 2: totp + recovery set).
118. Remove last 2FA-enabling factor while a workspace requires 2FA. 409 `two_factor_required_by_workspace` with workspace list; nothing removed. User leaves workspace or admin turns requirement off first. CLI `user:disable-2fa` overrides.
119. Remove last factor while member of two workspaces, one requires 2FA. 409 naming that workspace; other workspace irrelevant.
120. Remove recovery set while 2FA on (`DELETE /user/auth-factors/{fac}` on `recovery_code`). 409 `recovery_codes_required`; regenerate instead.
121. `DELETE /user/auth-factors` (all) with only pending factor. Pending deleted, 2FA was off: no disable mail, no grant revocation. 204.
122. `DELETE /user/auth-factors` with no factors. 204 no-op.
123. Regenerate recovery codes with 2FA off. 409 `two_factor_not_enabled`.
124. Regenerate while old code in flight on login (race). Factor row `FOR UPDATE` in both paths; whichever commits first wins; other: login gets 422 `invalid_code` (old set gone) or regenerate sees one fewer remaining before replace. No double use.
125. Recovery codes exhausted (last one used). Login succeeds; mail says 0 left; set row kept with `remaining: 0` (list shows it); next challenge `methods` lists only `totp`. Lost TOTP then = locked out: `dimmi user:disable-2fa` / support.
126. Recovery code used: mail with remaining count. 2 or fewer left: mail and settings prompt regenerate.
127. Same recovery code submitted twice concurrently (two tabs). Row lock; second 422 `invalid_code`.
128. Recovery code typed with spaces, uppercase or missing dash. Normalized before compare; accepted.
129. Challenge `method` not enrolled (`{method: "totp"}` for user with only recovery set left, or `{method: "webauthn"}`). 422 `errors.method` `method_not_available`; no attempt counted.
130. Challenge `method` missing. 422 `errors.method` `required` (pre-launch, no default).
131. TOTP driver disabled in config (`auth.factors.enabled` drops `totp`) while users have TOTP. 2FA stays on (`two_factor_confirmed_at` unchanged; never silently drops 2FA). Challenge offers remaining methods (recovery codes); none left: `methods: []`, SPA shows recovery path. Operator warned at boot when active factors exist for disabled driver.
132. `auth.factors.enabled` contains unknown type. Boot panic (registry rule).
133. Factor added from app (app token, `POST /user/auth-factors`). Allowed (U). Needs recent auth: principal `AuthTime` within 15 min (sheet re-auth `max_age=0`) else 423. Activation from app enables 2FA: no browser session to rotate; caller grant kept; other app grants untouched (enabling never revokes).
134. 2FA enabled from browser while app connected. Existing grants stay valid (enabling only adds protection); grant `auth_methods` lacks `totp` until next app login. Later policy "session must have used 2FA" would read it.
135. Rename factor to empty or 61+ chars. 422 `errors.name`. Name with control chars: stripped (NFC).
136. `PATCH {preferred: true}` on recovery set. 422 `errors.preferred`. On pending factor: allowed, takes effect on activation.
137. `PATCH {code, name}` together. 422 `errors.name` `prohibited`.
138. Removing preferred factor (later, several factors). Next active factor by `created_at` becomes preferred in same tx.
139. Factor secret decryption fails (system key lost or `encryption_keys` row destroyed). Factor unusable: 500 logged, challenge omits it; operator `dimmi user:disable-2fa`. Key rotation re-wraps DEK only; secrets still decrypt (ciphertext header carries key id).
140. Workspace requires 2FA, user is member of two workspaces, one requires. Requiring workspace 403 `two_factor_required` until factor added; other workspace unaffected.
141. Admin enables `security.require_two_factor` while member mid-session. Cache bust; member's next workspace request 403 `two_factor_required` (`requirement: two_factor`, `action_url`); realtime socket stays (presence only) but REST blocked; after activation works immediately (cache bust).
142. Tokens of member without 2FA under requirement. `AccessPolicy` applies to tokens: 403 `two_factor_required`; CI breaks loudly with message naming member; documented.
143. Admin without 2FA toggles requirement on. 409 `two_factor_not_enabled`.
144. CLI `user:disable-2fa` on user in requiring workspace. Allowed (operator override); user then 403 `two_factor_required` there until new factor added.
145. Same user with 2FA enabled but session created before (no `totp` in `auth_methods`). Passes `RequireTwoFactor` (user-level rule at MVP).

## Access policy and permissions

146. Policy failure body. Always 403 `{code, message, requirement, action_url}`; client branches on `requirement`, not code list. Unknown future `requirement`: client shows `message` + opens `action_url`.
147. `access.policy` on exempt routes (`/me`, `/user/*`, `/sessions`). Not applied; user can always reach factor setup and leave workspace.
148. Policy evaluation for token request. Same rule set as session; principal `AuthMethods` empty (tokens never carry login methods).
149. Old app build gating UI on `role`. Still works at MVP; new builds gate on `member.permissions`. Role-to-permission changes need only `roles.go` edit.
150. Token with `settings:read` on audit logs. Allowed: `audit_logs:read` belongs to `settings:read` group. Member role token with `settings:read` ability: denied (role lacks permission; 403 `missing_ability` with role-ceiling message).
151. `GET /tokens/abilities` for member. Groups listed with `allowed: false` where role holds none of group's permissions.
152. Policy code compares role names outside `roles.go`. CI AST lint fails.

## Workspaces and organizations

153. Slug collision on create with auto slug. Append `-2`, `-3` ... up to `-99`, then random 4-char suffix.
154. Slug in reserved list or previously used within 30 days. 422 `slug_reserved`.
155. Invalid timezone (`America/Nowhere`). 422 via embedded tzdata validation.
156. Cloud user hits 10 owned workspaces. 409 `workspace_limit_reached`; deleted-in-grace workspaces count until purge.
157. Workspace deleted while members online. Event broadcast on `workspace:{ws_}`; clients switch away; server unsubscribes all `workspace:`/`conversation:` channels of that workspace (centrifuge server-side unsubscribe); API 404.
158. Restore after purge started. `PurgeWorkspace` first sets `purge_started_at` in settings; restore checks, returns 410 `workspace_purging`.
159. Purge job crashes midway. Unique by workspace; re-enqueued with backoff; chunked deletes idempotent.
160. Purge blocked by `RetentionGuard` (later legal hold). Skip, `retention_held` recorded, workspace stays trashed; retried daily. MVP noop never blocks.
161. Two owners delete and restore concurrently. Row lock on workspace; last committed wins; both audited.
162. Inbound email/widget traffic during grace. Treated as unknown workspace by channel specs (dropped, provider gets 200). Documented in delete confirmation text.
163. Workspace suspended mid-request. Check at middleware start; in-flight write completes.
164. `test_mode` toggle requested after create. 422 `test_mode_immutable`.
165. `Dimmi-Workspace` header for workspace user left. 403 `membership_deactivated`.
166. `Dimmi-Workspace` with slug instead of id, or malformed id. 404 `workspace_not_found` (only `ws_` ids accepted).
167. `PATCH /workspaces/{ws} {trashed: false}` on workspace not trashed. 200 no-op, no event.
168. `PATCH /workspaces/{ws}` with any field besides `trashed` (`name`, `trashed: true`). 422 `errors.<field>` `prohibited`; rename via `PATCH /workspace`, delete via `DELETE /workspace`.
169. `PATCH /workspaces/{ws} {trashed: false}` by admin or member of trashed workspace. 404 `workspace_not_found` (trashed workspace visible to owners only).
170. Trashed workspace in `GET /user/memberships` without `filter[trashed]`. Excluded (rest.md rule 7); owners pass `filter[trashed]=with|only`.
171. Every workspace has org row. `CreateWorkspace` inserts org first in same tx; FK `not null` makes orphan impossible. Initializer failure rolls back both.
172. Org row soft-deleted while workspace live (manual SQL, bad repair). `workspace:repair` clears org `deleted_at` and resyncs name; billing resolve (12) treats deleted org as error and alerts.
173. Same user creates second workspace. New org (1:1 at MVP); separate billing account (12).
174. Workspace renamed. Org renamed in same tx while org has one workspace.
175. Workspace purged. Org `deleted_at` set when no live workspace remains; row kept (billing retention); `organization_id` never reused.
176. Client asks for `organization_id`. Not in any MVP resource; additive later.
177. Workspace PATCH of org-locked setting (later). 422 `setting_locked`. Never at MVP (no org settings).

## Memberships (own)

178. `POST /user/memberships` without `invitation_token`, or with `workspace_id` (domain join later: `domain-join`). 422.
179. `DELETE /user/memberships/{mem}` with someone else's `mem_`. 404 `not_found`.
180. `DELETE /user/memberships/{mem}` on own already deactivated membership. 204 no-op.
181. Leave as last owner. 409 `last_owner`; transfer first (`PATCH /members/{mem} {role: owner}`).
182. Leave workspace that blocks 2FA removal. Allowed (route exempt from `access.policy`); afterwards factor removal no longer blocked by it.

## Members and ownership

183. Two owners demote each other simultaneously. Workspace row `FOR UPDATE` serializes; second gets 409 `last_owner`.
184. Last owner deletes account. Blocked (409 `sole_owner`) when other active members; else workspace deleted with account.
185. Owner deactivated by another owner while holding tokens. Tokens stop at once (member inactive); realtime connections disconnected (centrifuge).
186. Admin tries to deactivate owner (`PATCH /members/{mem} {status: deactivated}`). 403 `forbidden` (lacks `members:manage_owners`).
187. Member deactivated then re-invited. Accept reactivates same `workspace_members` row (history, assignments, `mem_` id preserved); role from invitation.
188. Deactivated member has assigned open conversations. 03 unassigns (or reassigns to `reassign_to_member_id` if active) with events; closed conversations keep assignee for history.
189. `reassign_to_member_id` is deactivated or other workspace. 422 `errors.reassign_to_member_id`.
190. `reassign_to_member_id` sent without `status: deactivated`, or `keep_owner` without `role: owner`. 422 `prohibited`.
191. Member is in team and deactivated. Removed from team_members (team.member_removed events); not restored on reactivate.
192. Ownership transfer to member without verified email. 409 `target_unverified`.
193. Ownership transfer when workspace has `security.require_two_factor` and target lacks 2FA. 409 `target_two_factor_required`.
194. `PATCH /members/{mem} {role: owner}` by admin. 403 `forbidden`; by PAT or third-party OAuth token (even owner's): 403 `first_party_only`.
195. `PATCH /members/{mem} {role: owner}` without recent auth. 423 `password_confirmation_required`; nothing changes.
196. `PATCH /members/{mem}` with `role` and `status` in one body (`{status: active, role: admin}` reactivate as admin; `{role: member, status: deactivated}`). Both transitions validated against current state first; applied in one tx; events `member.reactivated` then `member.role_changed` then `member.deactivated` (only changed fields); any invalid field: nothing applied.
197. `PATCH /members/{mem} {role: owner, status: ...}`. 422 `invalid_transition` (transfer only to active member, alone).
198. `PATCH /members/{mem}` with current values (`status: active` on active). 200, no event, no audit.
199. `PATCH /members/{mem} {status: deactivated}` on self. 422 `invalid_transition`; leave via `DELETE /user/memberships/{mem}`.
200. `PATCH /members/{mem} {status: "suspended"}` (unknown value). 422 `errors.status` (`in` rule).
201. Role change of self from owner to admin when other owner exists. Allowed with recent auth.
202. Role change affects open tokens. Token power shrinks immediately (role permissions ∩ groups recomputed per request); token not revoked.
203. Role change while member's app open. `member.permissions` in next `/me` reflects it; cache bust on `member.role_changed`; app re-reads `/me` on realtime event.
204. Availability set by member with no sockets. Stored `online` but effective presence offline (foundation rule).
205. User account deleted. Member rows deactivated, `users.name` "Former teammate"; message author displays "Former teammate"; audit logs keep snapshot label.

## Teams

206. Team name differing only by case or Unicode form. NFC + case-insensitive uniqueness; 422.
207. Team deleted while conversation assignment to it in flight. Team hidden first (`deleted_at`); assign (03 `PATCH /conversations/{conv} {team_id}`) rejects deleted team 422; unassign job sweeps.
208. Re-create team with same name right after delete. Allowed (partial unique index).
209. Add deactivated member to team. Skipped with per-id result `inactive`.
210. Team deleted that is inbox default team. 03 sets `inboxes.default_team_id` null during sweep.
211. `team_members.workspace_id` differs from team's workspace (bug). Insert path sets it from team row; RLS hides cross-tenant rows; constraint test asserts equality.

## Invitations

212. Invite email of existing active member. Result `already_member`, no mail.
213. Invite same email twice. Second rotates token and resends (`resent`); one pending row.
214. Invitation expired then clicked. Preview shows `expired`; accept 410; admin `POST /invitations/{inv}/deliveries` (rotates).
215. `POST /invitations/{inv}/deliveries` on accepted or revoked invitation. 422 `invalid_transition`; 4th delivery in 24 h: 429.
216. Invitation accepted by user whose email was changed after invite. Match on current verified email at accept time; mismatch: 403.
217. Invitation accepted from app (`POST /user/memberships {invitation_token}` with app token). Allowed (U); same rules as browser.
218. Invitation to workspace requiring 2FA accepted by user without 2FA. Accepted; then 403 `two_factor_required` on workspace routes until factor added.
219. Inviter deactivated before accept. Invitation still valid (workspace invited, not person); `InvitationAcceptedMail` skipped.
220. Team in `team_ids` deleted before accept. Skipped silently; audit notes skipped ids.
221. Workspace deleted with pending invitations. Revoked at delete; restore does not revive.
222. Accept race (double click). Row lock; second request 409 `invitation_already_accepted` but SPA treats as success when user already member.
223. Invitation for admin role created by admin then admin demoted before accept. Invitation keeps role; acceptance valid (invitation is workspace decision). Owners can revoke.
224. Mail not configured on self-host. `accept_url` returned to admin; log fallback prints too.

## Tokens

225. Token typo or truncated. CRC32 checksum fails: 401 `invalid_token` without DB query.
226. Token from `test_mode` workspace used against live workspace. Token is workspace-bound; workspace implied by token; header `Dimmi-Workspace` with other id: 403 `workspace_mismatch`.
227. Token with `*` held by member later demoted. `*` expands to all groups, intersected with current role permissions.
228. Token expiring exactly now. `expires_at <= now()` rejected.
229. Hundreds of requests per second with one token. Lookup cached 30 s; `last_used_at` coalesced (one UPDATE per 60 s per replica).
230. Revoke on replica A, request on replica B within cache TTL. Valkey pubsub bust event; missed message (Valkey reconnect) bounded by local cache TTL capped at 2 s while pubsub is disconnected; worst case under 2 s.
231. Token in URL query (`?token=`). Not accepted; only `Authorization` header.
232. Token leaked in public GitHub repo. Checksummed format allows future secret-scanning partner program; manual revoke now; open question in README.
233. Member creates 51st token. 409 `token_limit_reached`; revoked tokens not counted.
234. PAT tries `POST /tokens` or `GET /tokens`. 403 `first_party_only` (token cannot mint or list tokens).
235. Member asks for `billing:read` ability. 422 `ability_not_allowed` (role holds no permission in group).
236. `dmi_oat_` token on any route. Guard hands to 09 `ResolveAccessToken`; never parsed here.

## Audit and security events

237. Audit write fails inside tx. Whole action rolls back (audit is sync in tx; never lose audit for committed change).
238. Audit `changes` contains secret keys (`client_secret`). Redactor replaces values with `[redacted]`.
239. Audit list for workspace with millions of rows. Cursor pagination on `(created_at, id)`; filters index-backed; no counts.
240. Ascending tail (`sort=created_at&filter[created_after]=...`) while new rows insert. Cursor on `(created_at, id)` stable; rows committed later with earlier `created_at` (long tx) may be missed: SIEM client re-reads last 5 min window (documented).
241. CSV export huge. Streamed, max 366-day range per request, `Content-Disposition: attachment`, rate 5/hour per member.
242. Actor token later revoked or member deleted. Label snapshot keeps display.
243. Audit row from first-party app request. `via_kind = app`, `via_id` = grant id (registry enum accepts every contracts value).
244. `AuditSink` driver fails (later SIEM). Queued listener retries; audit row already committed; never blocks action.
245. Security events for login failures flooding. One row per user+IP per minute max.
246. Old security event types (`two_factor.recovery_code_used`). Never written; renamed `auth_factor.*` pre-launch.

## Account changes and deletion

247. Email change to address that has pending invitation. Allowed; invitations matched on email at accept time.
248. Email change confirm while another user registered that email meanwhile. 409 `email_taken`; pending change discarded.
249. Email change confirm link opened by attacker who controls new address only. Requires token sent to new address created after recent auth by real user: attacker cannot start flow without account access.
250. Account deletion with pending outgoing invitations as inviter. Invitations stay valid (`invited_by_member_id` kept; member deactivated).
251. Account deletion user owns tokens in many workspaces. All revoked in same tx.
252. Account deletion with factors. Factor rows deleted (cascade + explicit in tx); `two_factor_confirmed_at` nulled.
253. Re-signup with same email after deletion. New user, no link to old history; old member rows remain "Former teammate".

## Self-host first run and CLI

254. Two people hit setup simultaneously. Advisory lock + users count check (11 `CompleteSetup`); second 410 `setup_completed`.
255. Setup token leaked in logs to log aggregator. 11 rules: single use, 24 h, `dmi_setup_` prefix caught by log redaction, rotate with `dimmi setup:link`.
256. Multiple replicas boot fresh. 11 `IssueSetupToken` under advisory lock; others print "setup pending: see first replica log or run `dimmi setup:link`".
257. `dimmi user:create` while web setup pending. Same advisory lock; creates user, deletes setup token; web setup then 410.
258. Operator runs `user:create` with existing email and no workspace flag. Exit 1 "user exists; use --workspace-slug to add membership".
259. Upgrade from version without these columns. Expand-only migrations; new columns nullable or defaulted; no backfill blocking boot.
260. Database restored from backup with old sessions. Sessions still valid if not expired; operator can `user:logout` all (also revokes first-party app grants); documented in backup docs (11).

## Final sweep additions (2026-11-02)
261. Old browser without `Sec-Fetch-Site` posts from dashboard origin. `Origin` equals `Host` (or trusted dashboard origin): pass. Different `Origin`: 403 `cross_origin_request`.
262. Dashboard served from separate origin (self-host split deploy, local Vite dev server). Origin must be in `http.trusted_origins` (registered via `AddTrustedOrigin` at boot); unlisted origin: 403 `cross_origin_request`; boot logs trusted list.
263. Unsafe request with session cookie but neither `Sec-Fetch-Site` nor `Origin` (curl with copied cookie). Passes cross-origin check by design (not a browser, so not CSRF); session rules still apply. Documented; no token fallback.
264. Member kick while centrifuge control message lost (Valkey reconnecting). Dashboard connections carry `ExpireAt` 5 min; `OnRefresh` re-checks session and membership and refuses: worst-case kick lag 5 min; REST already 401/403 within 2 s. Metric `dimmi_realtime_disconnect_failures_total`.
265. Session revoked on node A while user's socket on node B. `session.revoked` AfterCommit disconnect goes through centrifuge Redis broker/controller on Valkey (all nodes).
266. App confirms via `POST /user/confirmation` (`method: totp`). 09 stores grant `confirmed_at`, `confirmed_method`; other installs of same user not confirmed (per grant). `GET /user/confirmation` from app returns grant `confirmed_until`.
267. Pre-hijack defense while victim's app is signed in. App grants revoked with reason `pre_hijack_defense`; app shows "signed out for security" from `oauth_grant.revoked` reason; security event `sessions.revoked_others` metadata `reason: pre_hijack_defense`.
268. Member token with `engage:write` tries `PATCH /announcements/{ann} {status: live}`. Role member lacks `engage:manage`: 403 `forbidden`. Admin token same body: allowed (role has `engage:manage`, group `engage:write`).
269. Token with `feedback:write` only (member role) deletes post. `feedback:delete` not in member role: 403. Admin token with `feedback:write`: allowed (`feedback:delete` in group `feedback:write`). `feedback:manage` lives in group `settings:write`: admin token without `settings:write` gets 403 `missing_ability` on board create.
270. Pre-13-15 token minted with `*`. Expands to new groups `changelog:*`, `feedback:*`, `engage:*`, `ai:actions` automatically (abilities recomputed per request); documented in token docs.
271. Member approves `approval: teammate` agent action call (07). Allowed (`ai_action_calls:approve` in member role, group `ai:write`, `U` guard). Same member edits action or connection: 403 (`ai_actions:manage` admin/owner). Admin PAT with `ai:write` only (no `ai:actions`): publish 403 `missing_ability` `required_ability: ai:actions`.
272. Admin token with `settings:read` only lists imports (`imports:read`) but `POST /imports/{imp}/runs {kind: rollback}`: 403 `missing_ability` (`imports:rollback` in `settings:write`). Member session any import route: 403 (no `imports:*` in member role).
273. Member adds GitHub repository or docs sync source (08). 403 (`github:manage`, `docs_sync:manage` admin/owner). Member lists repositories and sync runs: allowed (`github:read`).
