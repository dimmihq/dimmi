# 00 Accounts: public API rate limits (P-01)

Style: caveman. Track 07. Foundation `api.md` "Rate limits" has defaults; this file is the build. Widget API has own limits: owner 01, never here.

Today `framework/http/middleware/throttle.go` exists, no route names it; sign-in routes limited in actions (`app/auth/throttle.go`), unchanged.

## Numbers

Intercom's published limits could not be verified: gap report section 6 notes one page says 10,000 per minute per app and 25,000 per workspace, older says 1,000, both from search summaries. Not used as target. dimmi keeps foundation defaults; per-workspace default is ten tokens at full rate. Changeable per customer through overrides (12) and config on self-host.

| Bucket | Applies to | Key | Default | Entitlement key |
|---|---|---|---|---|
| Per token | PAT, third-party OAuth (`/api/v1`, `/mcp`, CLI) | `tok:<id>` or `ogr:<id>` | 600/min | `api.rate_limit_per_minute` (existing) |
| Token burst | same | same + `:b` | limit/6 per 10 s (100 at default) | derived |
| Per user | session, first-party app | `usr:<id>` | 1,200/min | `api.user_rate_limit_per_minute` (existing) |
| Per workspace | all token traffic of workspace | `ws:<id>` | 6,000/min | `api.workspace_rate_limit_per_minute` (new, delta) |
| Bad credential | failed bearer or session lookup | client IP | 60/min | none, config `api.bad_credential_per_minute` |

Decisions:
- Dashboard not counted in workspace bucket. Runaway script must never block teammates' inbox.
- Foundation "burst 100" expressed as second fixed window, since `framework/ratelimit` is fixed window (rueidislimiter). 10 s window also spreads load, same idea as Intercom's documented spreading.
- Fail open: fairness limit (throttle.go banner). Valkey down: WARN log, request proceeds.
- Bad credential bucket fails open too; checksum rejects most bad tokens before DB.
- Limit 0: no limit (self-host, entitlement `self_host` driver reads config `api.limits.*`).
- Guest routes: limits in actions stay; `/auth/config` and `/invitations/preview` unchanged.
- Protocol routes: SCIM own (21), OAuth framework limits (09), webhooks in per provider (foundation), realtime none.

## Framework change

`framework/http/middleware/throttle.go`, additive, no product words:

```go
type ThrottleOptions struct {
    // existing fields
    Limit func(r *http.Request) int  // when set, wins over MaxAttempts; 0 or less skips the limit
    Skip  func(r *http.Request) bool // when true, request passes uncounted
    Scope string                     // sets X-RateLimit-Scope
}
```
`Key` returning `""`: skip. Unit tests in framework (`throttle_test.go`).

## App middleware

`app/http/middleware/api_throttle.go` builds chain, alias `api.throttle`. Runs after auth + `workspace` + `access.policy` (refused calls do not eat budget; refusal is cheap).

Chain order: workspace, burst, credential. Each `Throttle`; last writes success headers, so client sees its credential's window. Refusal stops at bucket that refused and writes its headers.

Reads limit from `Entitlements.Limit(ctx, workspaceID, key)` (cached 60 s). Principal without workspace (U routes): catalog default of the key.

Entitlements not built yet (`app/` has none today): M18 first task builds `app/support/entitlements` catalog + `self_host` + cloud default drivers per contracts.md "Entitlements"; 12 adds `entitlement_overrides` driver later. Stated so builder does not wait.

Mount on every authenticated `/api/v1` group in `routes/api.go`, `routes/inbox.go`, new route files via one shared group helper `routes.authed(...)`; `/mcp` HTTP in `routes/mcp`. Route-table test `tests/arch/api_throttle_test.go`: every route with auth middleware under `/api/v1` passes `api.throttle`. Exempt list empty.

## Response

Every limited response:
- `X-RateLimit-Limit`, `X-RateLimit-Remaining` (as `Result.WriteHeaders`), `X-RateLimit-Scope: token|burst|user|workspace`.
429 adds `Retry-After` (seconds), `X-RateLimit-Reset` (Unix).

```json
{ "message": "Too many requests.", "code": "too_many_requests" }
```
Distinct from `too_many_attempts` (sign-in) and `entitlement_limit_reached` (period caps). OpenAPI: 429 on every operation via huma default.

- MCP: HTTP 429 + `Retry-After`, JSON-RPC error `-32000` `rate_limited` (09 `mcp.md`, unchanged; shares token bucket). Per-tool send limits stay 09.
- CLI: wraps REST; waits `Retry-After` and retries up to 3 times on GET and on writes carrying idempotency key; else prints message with wait. CLI code owner 09.
- Browser app: `lib/api/client.ts` on 429 retries GET once after `Retry-After`, under 10 s; writes surface error text.

## Observability and docs

`dimmi_api_throttled_total{scope}`. No token id labels. Docs page: limits table, headers, backoff advice. Tokens page in settings shows line: "600 requests a minute per token" from `GET /workspace/entitlements`. Developers page same.

## Edge cases

Numbered in [edge-cases.md](edge-cases.md) "API rate limits".

## Tests

`tests/feature/api/rate_limit_test.go`, real Valkey: "a token over its limit gets 429 with retry after", "limit headers come on every response", "two tokens of one workspace share the workspace limit", "a script hitting the workspace limit does not slow the dashboard", "a browser session has its own larger limit", "a burst over ten seconds is refused while the minute limit remains", "an override raises the limit" (cloud build tag), "zero means no limit", "mcp over http shares the token budget", "refused by access policy costs no budget", "repeated bad tokens from one address are slowed", "valkey down lets requests through and logs". `tc.Clock` where window matters; fixed window expiry by real Valkey TTL through short test decay.
