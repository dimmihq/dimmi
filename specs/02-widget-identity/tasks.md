# 02 Widget identity: tasks

Style: caveman. Order matters. Each task about 1 day with tests. Interleaves with 01: 02 M1 to M2 before 01 M3 JWT wiring task.

Tests: `framework/jwt` = unit + fuzz (framework layer). Everything in `app/` = integration only (real App, Postgres, Valkey; testing.md), driven through endpoints or actions with real App; no mocks. Test names carry edge-case ids (`E14 ...`).

## M1 Verifier core

- [ ] `framework/jwt` (`jwt.Verify`, framework.md 10.1): strict wrapper on go-jose v4 (`jose.ParseSigned` with alg allowlist per key, `jose.JSONWebKey` JWK/PEM import, `jwt.Claims.ValidateWithLeeway` on `a.Clock`) + strict base64url, duplicate-key-rejecting JSON parse, `crit`/`typ` rules, lifetime cap, `aud` matcher, typed errors. One JOSE lib in tree.
  - Tests (framework unit): table tests per step; RFC 8725 attack cases (none, alg confusion, `jku`, duplicate keys) (E1 to E7); `FuzzVerify` 5 minutes in CI nightly, 30 s per PR.
- [ ] `app/support/identityclaims`: claim schema (per-claim warnings, `sub` number conversion, `signed_up_at` date convention) + static hints per error code.
  - Tests (integration via 01 `POST /widget/v1/sessions`): bad-secret token gets `token_signature_invalid` + static hint, never valid (E10); ms `exp` gets `token_lifetime_too_long` (E14); every claim invalid variant yields warning, not failure (E36 to E44, E82, E83).
- [ ] Benchmark: HS256 verify under 0.2 ms p95 (`go test -bench`), recorded in CI benchmark job.

## M2 Secrets and settings

- [ ] Migrations `20261101000201`, `204` (`202`, `205` deferred: identity-validations; `203` unused: nonce cache in Valkey), sqlc queries, factories, RLS.
  - Tests: CHECK constraints (HS256 needs secret, asymmetric needs JWK); FK on widget_sessions set null.
- [ ] `CreateIdentitySecret` (HS256 generate `dmi_idsec_`, RS256/EdDSA import PEM SPKI or JWK, weak key and private key rejection, thumbprint), events, audit, redaction pattern `dmi_idsec_`.
  - Tests: secret shown once (show endpoint has no material); private PEM 422 (E13); weak key 422 (E11); 6th secret 422 (E27); log redaction test.
- [ ] `RotateIdentitySecret`, `RetireIdentitySecret`, `RevokeIdentitySecret` (+ revoke widget sessions via 01, chunk over 10k), `RenameIdentitySecret`; otter cache + Valkey PubSub bust listener.
  - Tests: concurrent rotate one wins 409 (E28); revoke invalidates on second replica under 1 s (two App instances, one Valkey) (E25); sessions revoked count in event; event consumed by 14 fake listener for portal sessions (E81).
- [ ] Register `identity:read`, `identity:manage` in 00-accounts permission catalog + roles.go; policies `authz.Can` only.
  - Tests: role matrix; member token with `settings:write` denied `identity:manage`; lint no role names.
- [ ] Controllers, requests, resources, policies for `/api/v1/identity-secrets*`: `POST .../rotations`, `PATCH` per-field dispatch (`name`, `status: retiring` + `retires_at`, `status: revoked` + `reason`, `revoke_sessions`) + idempotency with encrypted stored body.
  - Tests: member 403 on write; token ability; other workspace 404; replay returns same secret; transition table (retiring date move, retiring to revoked, revoked terminal 422 `invalid_transition`, `retires_at` without status 422, system-only reasons 422).

## M3 Verification and identify

- [ ] `VerifyIdentityToken{..., Surface}`: key selection (kid or try-all HS256 max 5), cached decrypted secrets, nonce `SET NX EX` in Valkey, `aud` rule per surface (`portal` requires `dimmi-portal`), `last_used_at` coalesce.
  - Tests: revoked kid message (E24); try-all with 2 active (E23); nonce replay `token_replayed`; Valkey key TTL = `exp` + leeway; Valkey down fails closed only for nonce tokens (E86); portal token on widget surface rejected (E77) and widget token on portal rejected (E78); zero DB writes on warm path (query counter assert).
- [ ] `ApplyIdentityClaims`: attributes, company upsert, identity rows (verified only per flags), claims hash skip, member-email test detection, `contacts.trusted_attribute_keys` maintenance. Exported for 03 REST upsert (`UpsertIdentifiedContact`).
  - Tests: absent key untouched, null deletes (E43); company switch (E42); unverified email no identity row (E51); test detection only with `email_verified` (E58, E59); `contact.updated.changes` lists changed `custom_attributes` keys (E84); claim keys added to `trusted_attribute_keys`, null removes, lead keys stay untrusted, absent untouched (E87-E91).
- [ ] `IdentifyVisitor`: advisory lock, merge table rows (create, upgrade lead, merge lead via 03 `MergeContacts`, reuse), verified email collision rules.
  - Tests (through 01 handshake endpoints): every row of merge table (E46, E47); two concurrent requests same `sub` produce one contact (E45); user-into-user never merges (E49, E53).
- [ ] `IdentifyContact` (14 portal SSO): steps 1, 2, 4, 5, 6 without lead upgrade/merge.
  - Tests (action entry with real App): create, reuse, merged chain followed (E80), verified email collision, no widget session created.
- [ ] Leak rules suite `tests/feature/identity/leak_rules_test.go` R1 to R8 (end-to-end through 01 widget endpoints).
  - Accept: suite required check in CI; any failure blocks merge.
- [ ] Wire into 01 handshake (`POST /widget/v1/sessions {user_jwt, previous_token}`, `PATCH /widget/v1/session`), logout (pair with 01 M3 task); fixed behavior (invalid = 401, plain hints unverified per R5); warnings array; translator strings `lang/en/identity.json`.
  - Tests: handshake rows with JWT in 01 README; 401 with no side effects (E60); snapshot per error code (hints reference identity source / helpers); token never in logs (log capture assert); `GET /widget/v1/session` identified 409 (R6, E69); error object shape `{message, code, hint, docs_url}`; hints never mention `user_jwt` (snapshot).

## M4 Developer experience

- [ ] Snippet templates `resources/snippets/identity/*.tmpl` (one per snippets.md block: helpers per stack, raw snippets, portal SSO helpers) + `BuildIdentitySnippets` + `/snippets` endpoint.
  - Tests: render with and without kid (E62); language filter; unknown language 422.
- [ ] CI job `snippets-ci`: Docker images (node 22, python 3.13, go 1.26, ruby 3.4, php 8.4) run each raw snippet and `@dimmi/nextjs`/`@dimmi/server` example token function with fixture secret, Go harness verifies via `VerifyIdentityToken`.
  - Accept: every raw snippet and helper example produces `valid: true` with `exp - iat = 600`; signed-out request returns `{"token": null}`; portal SSO helper produces `aud: dimmi-portal`, 300 s, nonce; job runs on changes to `resources/snippets/**` and `sdk/**`.
- [ ] MCP tool `dimmi_get_identity_status`.
  - Tests: ability enforcement; no secret material in any tool output.
- [ ] Job `ExpireRetiringIdentitySecrets`. No nonce prune (Valkey TTL).
  - Tests: `tc.Clock.Freeze` past `retires_at` revokes (E24); sessions untouched on natural retire.

## Done when

- `go test ./...` green including leak rules suite and fuzz smoke; `snippets-ci` green.
- Demo script:
  1. Fresh workspace: `dimmi_get_identity_status` shows `configured: false`.
  2. `POST /api/v1/identity-secrets` returns `dmi_idsec_...` once; put in Next.js example `.env.local`.
  3. `npx dimmi init` in Next.js example (Clerk adapter, `dimmi.config.ts`); sign in to example app; widget starts verified session via server action; inbox shows external id, plan, company.
  4. Anonymous chat first, then sign in: lead conversation appears under user (merge).
  5. Custom signer sends expired token: browser console "Token expired 3 minutes ago" + hint; widget stays anonymous.
  6. In another browser, type user's email as anonymous lead: no history shown.
  7. Rotate secret (`POST .../rotations`): old and new tokens both start sessions; `PATCH {status: "revoked", revoke_sessions: true}` on old: sessions on old secret drop within 1 s.
  8. Sign out in host app: next navigation widget anonymous (`DELETE /widget/v1/session`); `Dimmi.logout()`: storage empty, session revoked; reload: anonymous, no history, no session request.
  9. Portal SSO link from example app: portal signed in as same contact; replay same link: `token_replayed`.
