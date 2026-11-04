# 02 Widget identity: edge cases

Style: caveman. Each: case. decided behavior. Session and transport cases shared with [01 edge-cases](../01-messenger-widget/edge-cases.md).

## Token format and crypto

1. `alg: none` token. Rejected `token_alg_not_allowed` before key lookup. Fuzz test.
2. Token signed HS256 using RSA public key bytes as HMAC key (alg confusion). Key for `kid` is RS256: `token_alg_mismatch`. HS256 path never loads asymmetric keys.
3. Header `jku`, `x5u`, `jwk`, `x5c`. Ignored; never fetched; key only from workspace table.
4. Header `crit` present. `token_malformed` (we implement no extensions).
5. Duplicate JSON keys in payload (`"sub":"a","sub":"b"`). Parser rejects duplicates: `token_malformed` (prevents parser differential).
6. Payload JSON array or string instead of object. `token_malformed`.
7. Base64 with padding `=` or standard alphabet. Strict base64url without padding per RFC 7515: `token_malformed`; hint "use base64url without padding (any JWT library does this)".
8. Token with whitespace or `Bearer ` prefix. Trimmed of surrounding whitespace; `Bearer ` prefix rejected with hint.
9. Token 9 KB (huge custom attributes). `token_too_large`; hint to use REST contacts update (03).
10. HS256 secret pasted with trailing newline in `.env`, or base64-decoded by host. `token_signature_invalid` with static hint naming common causes; mistake detector names exact cause (validations.md); variant key never accepted.
11. RS256 public key under 2048 bits uploaded. 422 `public_key.weak`.
12. EdDSA key not Ed25519 (Ed448). 422 `public_key.unsupported_curve`.
13. PEM with private key uploaded by mistake. 422 `public_key.private_key_uploaded` with message "Upload public key only. Rotate this key: it was sent to Dimmi." Private material never stored or logged (request body excluded from logs).

## Time

14. `exp` in milliseconds. Far-future `exp`: `token_lifetime_too_long`; static hint says seconds.
15. `exp` as string `"1793530800"`. `token_malformed` field type; hint "exp must be a number".
16. Host server clock 2 minutes fast, `iat` in future. Beyond 60 s leeway: `token_not_yet_valid` with computed offset in message.
17. Dimmi server clock skew. `dimmi doctor` checks clock (foundation); NTP required; leeway covers small skew.
18. Token valid at session start, expires during session. Session lives its own TTL (24 h from verification); token expiry irrelevant after start.
19. Session TTL hits while visitor typing. Send gets 401 `session_expired`; SDK asks identity source, `PATCH /widget/v1/session` (admits expired, not revoked identified session), resends same `client_id` (no duplicate).
20. No identity source configured (custom code passed one token once) and session expired. SDK falls back anonymous with warn `identity_refresh_missing`; outbox entries of identified user discarded (never sent as anonymous).
21. `exp` 30 days ahead. `token_lifetime_too_long`.
22. No `iat`, `exp` 6 days ahead. Lifetime measured from now: allowed.

## Keys and rotation

23. Rotation: new secret active, old retiring 7 days. Tokens with either kid valid; tokens without kid try both.
24. Retire window ends. Job marks revoked (`retired`); existing sessions verified by it keep working until their TTL; new session starts with old kid fail `token_secret_revoked` with date.
25. Secret leaked: revoke with `revoke_sessions`. Secret unusable cluster-wide under 1 s (PubSub cache bust); sessions verified by it revoked and realtime streams disconnected; 14 portal sessions created with it revoked (14 listener); visitors on next request become anonymous (identified ones start new session via `POST /widget/v1/sessions` with new-secret JWT once host deploys).
26. Revoke only active secret (`PATCH {status: "revoked"}`). Allowed; warning in response `no_active_secret`; session starts with JWT fail `identity_not_configured` until new secret. Dashboard banner.
27. Sixth secret. 422 `secret_limit_reached` (max 5 non-revoked).
28. Two admins rotate same secret at once (`POST .../rotations`). Row lock on old secret; second gets 409 `secret_not_active` (already retiring). Idempotency-Key replay returns first result.
29. Create secret response lost (network). Idempotent replay within 24 h returns same secret (encrypted stored body). Without key: admin creates another; old unused one can be revoked.
30. Token signed with key from other workspace (wrong `.env` across projects). `kid` unknown in this workspace: `token_kid_unknown`, hint "Wrong workspace or region?". Without kid: `token_signature_invalid` with static hint listing wrong-workspace as likely cause. Never looks up other workspaces (no cross-tenant read).
31. Workspace on cloud, token signed with self-host secret (migration). Same as 30.

## Claims

32. `sub` missing, `user_id` present. `token_sub_missing`; hint "Set sub (not user_id)". No alias.
33. `sub` numeric. Converted to decimal string; warning. Large number beyond 2^53 already lost precision in host JSON: documented ("send string").
34. `sub` with leading/trailing spaces. Trimmed; `" 42"` and `"42"` same user. Internal spaces kept.
35. `sub` changes format (host migrates ids int to uuid). New contact created; old one stays. Merge via 03 manual merge or REST update `external_id`. Documented.
36. `email` invalid. Claim dropped, warning; session still verified.
37. `email` uppercase. Domain lowercased; local part preserved; citext comparisons case-insensitive.
38. Unicode name with emoji, RTL, combining marks. NFC stored; bidi controls stripped; max 120 graphemes measured in runes after NFC.
39. `custom_attributes` nested object value. Key dropped with warning `claim_custom_attributes_invalid` (scalar only).
40. More than 100 custom attribute keys after merge with existing. Keys beyond limit rejected (JWT keys first, existing kept), warning; never 500.
41. `company` without `id`. Company claim dropped, warning.
42. `company.id` changes for same user (user switches org). `contacts.company_id` updated to new company; old company kept.
43. Attribute removed from host payload. Untouched (absent = no change). Explicit `null` deletes.
44. Huge `avatar_url` or `javascript:` scheme. Dropped (https only), warning.

## Merge and identity

45. Two tabs start sessions for same new user simultaneously. Advisory lock serializes; one contact, two sessions.
46. Lead `L` logs in as existing user `C`. `L` merged into `C`; `L` conversations visible under `C`; `contact.merged` emitted; `L` widget channel told `contact.merged`, SDK fetches new realtime token for `C` channel.
47. Lead logs in as brand-new user. `L` upgraded in place (same contact id), no merge event.
48. Shared device: anonymous visitor A chats, then user B logs in same browser. A's lead merges into B (same device trust boundary; A's chat already visible on that device). Documented.
49. Identified user U1 page loads as U2 without `logout()`. SDK sends `POST /widget/v1/sessions` with U1 token as `previous_token`: U1 session revoked `identity_switch`; new U2 session; no merge between users (R7).
50. Attacker types victim email in widget (lead). Stored unverified on lead; no identity claim; no history (R2).
51. Host sends JWT email without `email_verified` equal to email of email-channel contact D. No merge (R3). `C.email` set; inbox shows "possible duplicate" only if D is verified user; otherwise nothing.
52. JWT with `email_verified: true` matches email-channel lead D with verified email (confirmed via reply). Auto-merge D into C (both verified, foundation rule).
53. JWT `email_verified: true` matches other user contact D (different `sub`). No merge (R7); duplicate flag.
54. Host app lets users sign up with unconfirmed email and sets `email_verified: true` wrongly. Host bug; docs warn loudly in snippets; Dimmi cannot detect. No unmerge action; merge audit logged with full from/to ids for manual repair.
55. Contact `C` erased (GDPR), same `sub` starts session again. New contact created (erased tombstone has no `external_id`).
56. Contact `C` merged into `C2` by teammate; session start with old `sub`. Lookup follows `merged_into_id`; `C2` gets `external_id` if it had none; if `C2` has different `external_id`, session uses `C2` and logs conflict (teammate decided same person).
57. Contact blocked. Verified session start succeeds (no tell); messages land as spam.
58. Workspace member logs into own host app (verified email equals member email). Session `test`; conversations not billed (D10).
59. Member email unverified in JWT. Not test (prevents customers spoofing member email to avoid billing).

## Invalid tokens and hints (fixed behavior)

60. Invalid JWT. Visitor proceeds anonymous; loud console error; 401 has no side effects.
61. `user.user_id` given. Stored `custom_attributes.claimed_user_id` on lead; inbox shows "Unverified user id"; no lookup (R5).

## API

62. Snippets requested before any secret. `{{.Kid}}` rendered as empty string with header comment "create secret first"; response `meta.configured: false`.
63. Nonce keys grow under replay attack. Insert rate bounded by widget rate limits; keys expire by Valkey TTL (`exp` + leeway); nonce hashed (SHA-256) so key size fixed; nonce max 128 chars.

## Secret status via PATCH

64. `PATCH {status: "retiring", retires_at}` on already retiring secret. Moves date within bounds (max +30 d from now); no new event; audit logged.
65. `PATCH {status: "revoked"}` on retiring secret. Allowed (cuts grace short); `revoke_sessions` default true; event `identity_secret.revoked`.
66. `PATCH {status: "active"}` on retiring, or any status on revoked. 422 `invalid_transition` with `from`, `to`. Revoked is terminal; un-retire not supported (rotate or create).
67. `retires_at` sent without `status: "retiring"`, or past/over 30 d. 422 field error; nothing changed.
68. `PATCH {name, status}` together. Both applied in one tx; `status` fires its event, rename audit only. `reason: "rotated"` or `"retired"` from client: 422 (system-only values).
69. Stored identified token, page load without JWT, forked SDK calls `GET /widget/v1/session`. 409 `identity_proof_required` (R6 server guard); session not resumed. Compliant SDK sends `DELETE /widget/v1/session`.

## Seams (`00-foundation/extensibility.md`)

70. Admin demoted to member between page load and secret rotation. `authz.Can(principal, "identity:manage")` evaluated per request from current role: 403. Admin token of demoted user same (role permissions intersect abilities).
71. JWT `company` claim while contact already linked to other company via REST. JWT company becomes `contacts.company_id` (primary). Overwrites primary only, never deletes company row.
72. Legal hold (later guard driver) on workspace. Nonce keys expire by TTL; contact erasure stays held by 03.

## Identity source and token lifetime (SDK redesign)

73. Host inlines token into HTML (custom code), page cached by CDN. Every visitor gets same `sub`: first visitor's identity. Server cannot detect from one request; docs and helpers never inline; 10-minute `exp` bounds replay; `token_expired` after 10 min surfaces bug fast.
74. Identity endpoint cached with `public` headers. Many users share token or get expired one. `token_expired` / `identity_mismatch` errors; SDK `identity_endpoint_invalid` name `Cache-Control: no-store, private`.
75. Custom signer uses 7-day tokens (old docs). Accepted up to `identity.max_token_lifetime`; helpers refuse over 1 hour.
76. Token minted at 9:59, request arrives at 10:10 (`exp` 10-minute window, slow page). `token_expired`; SDK asks source again once (identity source called on 401 `token_expired`), retries handshake.

## Portal surface (14)

77. Portal token (`aud: "dimmi-portal"`) sent to widget handshake. Widget surface does not accept `dimmi-portal`: `token_audience_mismatch` (portal tokens are 300 s, nonce, fragment-borne; never widget credentials).
78. Widget token (no `aud`, or `aud: dimmi`) sent to portal SSO. Portal surface requires `aud` containing `dimmi-portal`: `token_audience_mismatch`; portal shows plain error with docs link.
79. Portal SSO link opened twice (back button, link preview bot fetching fragment-less URL). Fragment never reaches server on bot fetch; second real use of same token: `token_replayed` (nonce), portal shows "Sign in again" and redirects to `sso_login_url`.
80. `IdentifyContact` for `sub` held by contact merged into other contact. Follows `merged_into_id` (same as step 2); portal session bound to survivor.
81. Secret revoked with `revoke_sessions` while portal session active. 14 listener revokes portal sessions by `identity_secret_id`; next portal request 401, user re-SSOs with new secret once host deploys.

## Claims conventions

82. `custom_attributes.signed_up_at` as `"last tuesday"`. Kept as string, warning `claim_signed_up_at_invalid`; 18 rules treat as missing date.
83. `signed_up_at` in milliseconds (`1767225600000`). Value over 10^11: warning `claim_signed_up_at_invalid` with hint "seconds, not milliseconds"; stored as given.
84. Claim change (plan upgrade) on next token. `ApplyIdentityClaims` writes changed keys; `contact.updated` `changes` lists changed `custom_attributes` keys (14 revenue, 18 dependency filter); SDK refetches engage items on `identity_changed`.

## Nonce cache (Valkey)

85. Valkey restarted mid-window. Nonce keys lost; token replayed within its `exp` accepted once more. Accepted risk (helper tokens 10 min, portal 300 s); no durable table.
86. Valkey unreachable. Tokens with `nonce` (all portal tokens) fail 503, retried; tokens without `nonce` verify normally (widget handshakes unaffected).

## Trusted attributes (07 agent actions)

87. JWT `custom_attributes.stripe_customer_id: "cus_1"`. Key added to `contacts.trusted_attribute_keys`; 07 actions may bind `{contact.custom_attributes.stripe_customer_id}` for verified widget messages.
88. Next token sends `stripe_customer_id: null`. Value deleted and key removed from `trusted_attribute_keys`; actions binding it become ineligible (07 eligibility).
89. Lead wrote `plan: "enterprise"` via widget, then logs in; token has no `plan`. Upgrade in place keeps value; key stays untrusted (absent from list) until a claim writes it. 07 never binds it.
90. Token omits key previously written by JWT (absent, not null). Value and trusted flag unchanged (absent keys untouched).
91. Claims hash unchanged (skip path). No writes; trusted list unchanged (written on first application).

## Modes, validations, JWKS

92. Switch to `required` with under 95% verified last 7 days or no traffic. 409 `rollout_guard` with numbers; retry with `force: true` + recent confirmation succeeds, audit logged.
93. Mode `required`, identified visitor's token fails (expired source). 401, widget hidden, console error; existing outbox kept, not sent.
94. Mode `required`, existing anonymous sessions. Next request 403 `identity_required`; SDK hides launcher; data kept. Switch back restores.
95. Mode `off`, JWT sent. Ignored, no verification, no error beyond one console warn. Stored identified session: `PATCH /session` answers 200 without extending; expires naturally, then anonymous.
96. Validator gets production token. Not stored, not logged, no nonce consumed, no session, no contact written.
97. Validator with token signed by other workspace's `kid`. Authenticated caller told "belongs to a different workspace" with no name; widget route keeps generic `token_kid_unknown`.
98. Session route token with newline-variant key. 401 `token_signature_invalid`, `mistake: secret_trailing_newline`. Never accepted. Detector runs at most 5 secrets times 5 variants, per-IP throttle 10/min then static hint.
99. Detector leak check: variant match reveals only that key was misused, to someone already holding it. Not an oracle for outsiders: needs valid HMAC.
100. JWKS URL private address or redirect to one. safedial refuses; `jwks_error`, secret stays pending; 422 at create with `jwks_unreachable`.
101. JWKS endpoint down. Cached keys used up to 24 h; after that `token_jwks_unavailable` 401 (fails closed), banner in page.
102. Unknown `kid` from JWKS secret. Refetch at most once per 5 min, then `token_kid_unknown`. Flood of random kids: no extra fetches.
103. JWKS key with `alg: none`, symmetric key or weak RSA. Key skipped, never used.
104. Provider token lacks our claims (`email_verified` shape differs). Same claim rules; unknown ignored. `iss` must equal `jwks_issuer` when set.
105. Stats flush lost on crash. Up to 60 s of counts missing; hint data, not billing.
106. Client-writable attribute on verified contact, key also claimed by JWT. Client write stores value, removes key from trusted list; next JWT write trusts it again. Default list empty: 403 `identity_locked` as before.
107. `DELETE /contacts/{ctc}/widget-sessions` while streaming. All revoked `admin`, disconnected within 5 s; next handshake with valid JWT starts new session.

