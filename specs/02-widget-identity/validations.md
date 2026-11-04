# 02 Widget identity: modes, validations, mistake detector, JWKS, dashboard

Style: caveman. Roadmap name identity-validations, now in scope. Milestone M5, track 09 widget-identity.

## Modes

Settings registry key `identity.mode`, workspace scope, values `off`, `optional` (default), `required`. Behavior table: [README](README.md) "Identity behavior". Applies to all widgets of workspace, same as secrets. Read cached with secrets (otter, PubSub bust).

`required` is about proof of identity, not who may chat: who may chat is 18 `messenger.start`. Same effect as 01 `allow_anonymous = false` on every widget, plus invalid tokens hide widget instead of falling to anonymous.

Change: `PATCH /api/v1/identity-verification/settings {mode}` (`identity:manage`, audit logged, event `identity_settings.updated`, realtime yes, webhook no). Busts 01 config (`features.allow_anonymous` reflects mode; config includes `identity_mode` so SDK skips anonymous path).

Rollout guard (only for switching to `required`): needs at least one active secret and, in last 7 days, at least 100 session starts with `verified / (verified + invalid + anonymous_started)` at least 95%. Else 409 `rollout_guard` `{verified, invalid, anonymous, required_percent}`. `{mode: "required", force: true}` overrides, needs `user/confirmation` recent (existing recent-auth rule). `off` needs recent confirmation too (it stops verified history showing).

## Stats

`identity_verification_stats`, no public id. Migration `20261201090002_create_identity_verification_stats_table.sql`.

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid not null | |
| day | date not null | UTC |
| outcome | text not null | `verified`, `anonymous_started`, or error code (`token_expired`, ...) |
| count | bigint not null | |

PK `(workspace_id, day, outcome)`. Counted in memory by `VerifyIdentityToken`, flushed every 60 s (upsert add), same pattern as 01 install tracker. Pruned at 90 days (`RetentionGuard`). `GET /api/v1/identity-verification/stats?days=7` returns totals by outcome, by day. No per-person data.

## Validations endpoint

`POST /api/v1/identity-verification/validations` (rest rule 11: computed result, `identity:read`, 30/min per principal).

```json
{ "token": "eyJ...", "app_id": "wgt_...", "surface": "widget" }
```

```json
{
  "data": {
    "object": "identity_validation",
    "valid": false,
    "code": "token_signature_invalid",
    "message": "Signature does not match.",
    "hint": "Your secret was base64-decoded before signing. Use the full dmi_idsec_ string as is.",
    "mistake": "secret_base64_decoded",
    "header": { "alg": "HS256", "kid": "isk_..." },
    "claims": { "sub": "user_42", "exp": 1793530800, "email": "ada@acme.com", "email_verified": false },
    "contact_preview": { "action": "create", "contact_id": null },
    "warnings": []
  }
}
```

Runs `VerifyIdentityToken` in dry mode: no nonce consumed, no `last_used_at`, no counters, no contact or session write, token never logged, request body excluded from access and idempotency stores. Signature part never returned. `contact_preview.action` in (`create`, `reuse`). Claims shown only to member who pasted token.

## Mistake detector

`DiagnoseIdentityToken` runs only after failure. Hint only: a variant match never makes token valid (RFC 8725 rules and R1 to R8 unchanged). Runs on validator always; on session 401 adds `mistake` code + specific hint, throttled 10/min per IP (then static hint).

| `mistake` | Detected how |
|---|---|
| `secret_base64_decoded` | HMAC with base64url or base64 decoded secret bytes matches |
| `secret_trailing_newline` | HMAC with secret + `\n` or `\r\n` matches |
| `secret_quoted` | HMAC with secret wrapped in `"` matches |
| `secret_prefix_dropped` | HMAC with secret minus `dmi_idsec_` matches |
| `other_workspace_key` | validator only: `kid` exists in other workspace via `identity_secret_exists(kid)` SECURITY DEFINER returning bool; says "different workspace", no name |
| `exp_milliseconds` | `exp` over 10^11 |
| `exp_iso_string` | `exp` is date string |
| `company_flat` | top-level `company_id` or `company_name`, no `company` |
| `user_id_alias` | `user_id`, `userId` or `id` present, no `sub` |
| `email_not_flagged` | warning: `email` present, no `email_verified` (no merge happens, R3) |
| `public_key_as_secret` | HS256 token, `kid` absent, signature matches a stored public key PEM bytes |
| `bearer_prefix` | leading `Bearer ` |
| `identity_endpoint_cached` | same token `jti`-less hash seen with over 20 distinct IPs in 10 min (Valkey HLL, keyed by SHA-256 of token, 10 min TTL): warning on validator and console |

Bounded: at most 5 secrets times 5 variants on failure path only; p95 under 1 ms. Text in `lang/en/identity.json`.

## JWKS URL keys (auth-provider tokens)

Accept tokens from Clerk, Supabase, Auth0 and similar directly, no host signing step. Migration `20261201090001_add_jwks_to_identity_secrets_table.sql`.

`identity_secrets` gains: `jwks_url text null` (https only), `jwks_issuer text null`, `jwks_cache jsonb null`, `jwks_fetched_at timestamptz`, `jwks_error text`. Algorithm `RS256`, `EdDSA` or `ES256` (same migration widens `algorithm` CHECK and `identity.algorithms`, step 2 allowlist, because providers use it; `framework/jwt` already on go-jose). CHECK: exactly one of `secret_encrypted`, `public_key_jwk`, `jwks_url`.

- Create: `POST /api/v1/identity-secrets {name, algorithm, jwks_url, issuer}`; fetched once through `a.HTTP` (safedial, 5 s, 512 KB), 422 `jwks_unreachable`, `jwks_invalid`.
- `kid` in token picks key in set. Token `kid` is provider's; so lookup: `kid` not an `isk_`: search active JWKS secrets of workspace (max 5).
- `iss` must equal `jwks_issuer` when set. `aud`: step 9 rule, plus optional per-secret `audience`. Provider `sub` becomes contact `external_id`.
- Refresh: job every 6 h; on unknown `kid` at most once per 5 min per secret. Cache across replicas via row. 24 h stale limit: then `token_jwks_unavailable`, fails closed.
- Key rules as uploaded keys: no `none`, no symmetric, RSA at least 2048 bits, `jku` and friends in token header still ignored; URL comes only from dashboard.

Provider claims map through same claim schema; custom claims only via `custom_attributes` if provider emits them. Next adapters (signing own token) stay available.

## Client-writable attributes

Registry key `identity.client_writable_attributes` (workspace, array of attribute keys, default empty, max 20). `PATCH /widget/v1/contact {custom_attributes}` by identified session: only these keys accepted, others 403 `identity_locked`. Written keys removed from `trusted_attribute_keys` (same rule as leads, 01 E143). Name, email, phone stay JWT or REST only. R8 reads: widget writes to user contacts rejected except this explicit list.

## Sign out visitor everywhere

`DELETE /api/v1/contacts/{contact}/widget-sessions` (`identity:manage`): `RevokeWidgetSession` reason `admin` for all, disconnects, also deletes contact's push devices (20). 200 `{sessions_revoked}`. Contact panel action in 03 UI mounts when panel lands (button `resources/js/features/inbox/customer-panel/`, needs track 04 customer panel).

## Dashboard page

`resources/js/features/settings/identity/` replaces static `IdentitySettings` (`pages/developers.tsx`). Route stays `identity` view. BoardUI, semantic tokens.

| Section | Controls | API |
|---|---|---|
| Status | configured, mode, verified share (7 days), top errors | stats, settings |
| Mode | Choice Off, Optional, Required; guard message with numbers | settings |
| Keys | list: name, algorithm, fingerprint, status, last used; Create (HS256, public key, JWKS), Rotate, Retire, Revoke with `revoke_sessions` | identity-secrets |
| New secret dialog | shown once, copy, env var names | |
| Token checker | paste token; diagnosis card with mistake hint, claims, contact preview | validations |
| Snippets | language Choice, code blocks | snippets |
| Advanced | client-writable attributes list | settings |

e2e `tests/e2e/identity-settings.spec.ts`, light and dark.
