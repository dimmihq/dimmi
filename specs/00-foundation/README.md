# 00 Foundation

Style: caveman. Shared core every MVP feature builds on. Read with `specs/00-conventions/README.md`.

## Files

| File | What |
|---|---|
| [data-model.md](data-model.md) | core tables, ownership map, state machine, limits |
| [api.md](api.md) | REST/widget/MCP shared conventions, auth, pagination, errors, rate limits, endpoint ownership |
| [rest.md](rest.md) | REST rules + old-to-new route map; wins over feature specs |
| [events-and-realtime.md](events-and-realtime.md) | event catalog, listeners, realtime (embedded centrifuge: channels, auth callbacks, history/recovery, brokers) |
| [security.md](security.md) | tenancy, secrets, SSRF, untrusted content, GDPR, audit, abuse |
| [contracts.md](contracts.md) | canonical shared interfaces: framework contract pointers, domain contracts index, Channel pointer, Knowledge, mail streams, event dispatch phases, Actor, workspace initializer hook, contact erasers, auth pointers (Factor, IdentityProvider, LoginPolicy, `authz.Can`), AccessPolicy, Entitlements (+ catalog), RetentionGuard, AuditSink, KeyManager, settings registry, Feedback shipping (13 -> 14), shared GitHub layer (08 -> 13). Wins over feature specs |
| [extensibility.md](extensibility.md) | seams for post-launch growth: factors, orgs, permissions, policies, entitlements, settings, retention, keys. Binding for new specs |
| [tasks.md](tasks.md) | build checklist for foundation |
| [reconciliation-log.md](reconciliation-log.md) | history: conflicts between feature specs and how resolved |
| [spec-writer-brief.md](spec-writer-brief.md) | template and rules given to spec writers (reuse for new features) |

## Decisions

1. Go 1.26 monolith, Laravel structure (`00-conventions`).
2. PostgreSQL 18 is the only database of record. UUIDv7 keys, prefixed public IDs. Valkey required in every deployment for rebuildable state only (realtime, cache, rate limits, pubsub, dedupe windows; data-model.md "Valkey").
3. One event catalog drives realtime, webhooks, notifications, audit, metering, AI triggers.
4. Realtime = `github.com/centrifugal/centrifuge` embedded in `framework/broadcasting` (no Pusher protocol, no own protocol server). Dashboard and apps: centrifuge-js / official mobile SDKs over WebSocket. Widget: plain `EventSource` on uni-SSE with 60 s connect token. History + recovery replaces last-event-id resume. Broker, presence, history: centrifuge Redis broker on Valkey (required everywhere) ([events-and-realtime.md](events-and-realtime.md)).
5. Dashboard and first-party apps use public REST API only. No private endpoints. REST only: noun paths, state via `PATCH`, guards decide caller class ([rest.md](rest.md), [api.md](api.md)).
6. Every external service behind contract with local default and AWS driver.
7. Same codebase for self-host and cloud. Cloud-only modules (billing, RLS enforcement, abuse) behind `app.edition = cloud` config.
8. Multi-workspace always (users can belong to many workspaces). Self-host creates one on first run.
9. Org layer exists from day one (1:1 with workspace, hidden from API); enterprise features (SSO, SCIM, policies, billing owner, CMK, entitlement overrides) attach to org ([extensibility.md](extensibility.md)).
10. Reuse before build: `framework/` wraps maintained libraries; own code only for domain logic or glue (`00-conventions/framework.md` 2.5, `research/build-vs-reuse.md`).
11. Mail: one instance-level provider (deploy config), two streams `system` / `customer`, no SMTP receiver, no per-workspace providers (contracts.md "Mail").
12. Growth after launch is additive (new rows, drivers, registry enum values, nullable columns, endpoints). Driver-backed enums are registry enums; policies check permissions, never role names; cross-cutting hooks (entitlements, retention guard, login policy, access policy, audit sink) exist now as noop/MVP drivers.
13. i18n first class: request locale (preference hook > `Accept-Language` > `app.locale`), translated API error and validation messages with stable codes, mail and notifications in recipient locale, `app.locale`/`app.locales`, format helpers, `lang:check` (`00-conventions/README.md` "Localization (i18n)").
14. Tests use real backing services (root `compose.yaml`: Postgres pgvector pg18, Valkey 8, SeaweedFS S3, Mailpit). Fakes only at process boundaries (httpclient, dns, mail transport, malware, socialite, clock); every fake panics outside `testing` env.

## Performance budgets (whole system)

| Metric | Budget |
|---|---|
| Idle RSS, single binary, all roles | under 50 MB |
| Valkey idle RSS (self-host compose, empty workspace) | about 10 MB target [verify in CI], `maxmemory 64mb` default self-host |
| Whole self-host stack idle ($5 VPS: app + Postgres 18 + Valkey) | under 256 MB (CI measures all three containers) |
| RSS at 1,000 concurrent WebSockets | under 150 MB (centrifuge about 27-35 KB per idle socket) |
| API read p95 (workspace with 1M messages) | under 50 ms server time (full-text search endpoints: under 150 ms) |
| API write p95 | under 100 ms |
| Widget message sent to teammate screen | under 300 ms p95 same region |
| Inbound email to inbox | under 10 s after provider delivery |
| Cold boot to ready (migrations current) | under 1 s |
| Docker image | under 40 MB compressed |

Budgets checked in CI benchmark job where measurable; regressions over 10% fail.

## Observability

- `log/slog` JSON. Request id (`X-Request-Id` in and out), workspace id, actor in every log line.
- OpenTelemetry traces and metrics, off by default. OTLP tracing opt in: app registers `otlp.Tracing()` (`framework/telemetry/otlp`) in `foundation.Options.Telemetry`, then endpoint via config/env; default binary links no gRPC (own OTLP HTTP exporter later, tasks "Framework follow-ups").
- Prometheus `/metrics` on admin port: HTTP latency, River queue depth and age, centrifuge metrics (connections, subscriptions, broker publish latency), PubSub lag, outbound provider errors, LLM tokens and cost.
- `dimmi doctor` checks config, Postgres version and extensions, migrations, disk, S3 reachability, mail provider config per stream (`system` mailer reachable; `customer` stream configured, never `log` driver in production, SES configuration sets / Postmark streams present, bounce/complaint webhooks registered), mail DNS (SPF/DKIM), clock skew, outbound HTTP allowlist (`http_client.allow_hosts`), Valkey (reachable, `maxmemory-policy`, AOF warn-only).

## Cross-cutting edge cases

- Clock: all business time from `framework/clock` (`a.Clock`). Office hours, snooze, SLA use workspace timezone; DST transitions tested (snooze to 02:30 on spring-forward day resolves to 03:00).
- Unicode: names and bodies NFC-normalized; emoji and RTL safe; search uses `unaccent`.
- Huge workspaces: every list endpoint cursor-paginated and index-backed; no `COUNT(*)` on hot paths (counts cached or estimated).
- Deleted references: member deleted, history shows "Former teammate"; contact erased, "Deleted contact"; team deleted, conversations unassigned from team with event.
- Concurrent edits: two teammates assign at once, last write wins with both events recorded; two replies at once both stored in order of commit.
- Valkey down: `/ready` 503 (`/up` 200), auth and abuse guards fail closed (503 + `Retry-After`), outbound provider limiters defer, everything else fails open with log + metric, caches miss to Postgres, realtime down; recovers without data loss (data-model.md "Valkey unreachable" policy).
- Partial outages: LLM provider down does not block human flow; email provider down queues outbound with retries and shows `pending` delivery; Postgres down returns 503 with `Retry-After`.
- Maintenance mode: `dimmi down` / `dimmi up` (Laravel; state file `<data_dir>/framework/down` announced to every replica over pubsub; `/ready` reports it; production `down` asks confirmation or `--force`; paths in `Options.MaintenanceExcept`, e.g. inbound webhooks, keep working; hard mode, queue pause, late-joining replica sync later), widget shows offline form that stores to local queue in browser and retries.
