# Dimmi Tech Stack Decision

Date: 2026-11-01. Scope: language/runtime, database, frontend, monolith strategy, "don't build" list. Style: caveman. Claims with single or weak source marked [unverified]. GitHub stars/licenses/last-push pulled read-only via `gh api` on 2026-11-01.

## TL;DR decision table

| Layer | Pick | Runner-up | Why |
|---|---|---|---|
| Backend language | Go 1.26+ | TypeScript on Bun | Low RSS, static binary, fast compile, best agent DX evidence for backend, full ecosystem for our list. Bun shares language with frontend + Tier-1 MCP SDK, but higher RSS and 2026 leak reports. |
| HTTP/API | `net/http` + chi + huma (OpenAPI 3.1 generated from Go types) | echo | OpenAPI spec feeds REST docs, CLI, MCP tool schemas from one source. |
| DB access | pgx v5 + sqlc | bun ORM | SQL-first, typed, LLM-friendly; no ORM magic. |
| Database | PostgreSQL 18 (support 17+; test 19 at GA 2026-11-19) | none (SQLite rejected for v1) | One DB does queue, pub/sub, FTS, vectors, rollups, RLS. Dual-DB = maintain two copies of every heavy feature. |
| Jobs + cron + leader | River (Postgres) | pg-boss-style own SKIP LOCKED | Transactional enqueue, periodic jobs, leader election, UI, MPL-2.0. |
| Realtime | embedded `centrifugal/centrifuge` library (MIT) with its Redis broker + presence on Valkey; dashboard centrifuge-js, widget plain `EventSource` uni-SSE | own Pusher-protocol hub on coder/websocket + Postgres LISTEN/NOTIFY (rejected 2026-11-02, `build-vs-reuse.md` section 1); Centrifugo sidecar (rejected: second process) | Updated 2026-11-02: PO made Valkey required in every deployment; history/recovery, presence, cross-node fan-out from library. |
| Search | Postgres FTS (tsvector) + pg_trgm + pgvector hybrid (RRF) | pg_textsearch BM25 (optional adapter) | Works on every managed Postgres. BM25 extensions need preload, missing on many hosts. |
| Dashboard | React 19 + Vite + TanStack Router + TanStack Query + shadcn/ui (Base UI) + Tailwind v4 | Svelte 5 + SvelteKit SPA | Largest ecosystem, best LLM output, shadcn now defaults to Base UI. |
| Inbox sync | TanStack Query + WebSocket patch events + optimistic mutations | TanStack DB (1.0 RC) later | Zero/Electric need extra service + logical replication; break "one binary". |
| Editor | Tiptap 3 (MIT core, ProseMirror) | Lexical (still 0.x) | Markdown round-trip, mature extensions, AI-diff friendly. |
| Widget | Preact + Shadow DOM, Vite lib build, target <35KB gz | Svelte 5 compiled custom element | Shares React mental model + shadcn-like code; 4KB runtime. |
| Help center | Go SSR (templ + goldmark) from monolith, cached HTML | Astro SSG | SEO, zero JS by default, no Node at runtime. |
| Embed frontend | `go:embed` of Vite `dist/` | separate static host | One binary. |
| Plugins | Compiled-in Go interfaces + outbound webhooks + MCP/REST | Extism WASM hooks (later) | Simple, fast, type-safe; WASM only for user scripts. |
| Packaging | Static Go binary, goreleaser, compose (app + Postgres + Valkey), `install.sh` bare binary + systemd | none (all-in-one image rejected 2026-11-02) | Container per service; binary runs without Docker. |

## A. Language / runtime

### Evidence table

| Runtime | Idle RSS (small API) | WebSocket memory per connection | Startup | Single binary | Compile/iterate | LLM/agent evidence |
|---|---|---|---|---|---|---|
| Go 1.26 | ~10-30MB typical; Listmonk peak 57MB sending 7M+ mails (vendor claim) [unverified independent] | goroutine stack 2-8KB + read/write buffers; naive ~20KB/conn, netpoll tricks lower (Mail.ru 2017) | ms | Yes, static, cross-compile trivial | Fast | Ronacher (Flask author) found agents "strongly favor" Go; simple test runner |
| Rust (axum) | lowest, ~5-15MB [unverified] | lowest in community tests, ~8-12KB [unverified, conflicting] | ms | Yes | Slow: 104k-line axum app 3-4 min full build, 20s `cargo check` [single source]; Rust 2026 goal names build time top pain | Agents trip on cargo invocations (Ronacher); Multi-SWE-bench lower for Rust/Go than Java [unverified] |
| Elixir/Phoenix | Phoenix prod ~80MB app, ~170MB VM total on 256MB Fly VM, OOM reports | 2M conns on 58-84GB in 2015 = ~29-40KB/conn; 2.3M on 64GB later | ~1s | Release tarball; single file via Burrito, not native | Medium | AutoCodeBench: Elixir top language for most frontier models (~97.5% union upper bound) |
| TypeScript/Node | ~190MB flat (Trigger.dev production chart) | ws ~25KB/conn [unverified] | ~100ms+ | `node --experimental-sea` awkward | Fast (no compile) | Most training data; React-adjacent |
| TypeScript/Bun | ~85MB fixed build (Trigger.dev); leaks in HTTP disconnect path fixed after 2026-03-30; Autumn reports native-layer retention Aug 2026 | lower than Node [unverified] | fast | `bun build --compile` (~60-100MB file) [unverified] | Fast | Same as TS; Bun owned by Anthropic since 2025-12-03 |
| .NET 10 Native AOT | ~30MB idle minimal API [vendor, unverified] | n/a | 30-80ms | Yes | Medium; AOT forbids much reflection | Good C# scores; smaller OSS self-host community |
| Kotlin + GraalVM native | 80-150MB native vs 350-550MB JVM [unverified]; Spring PetClinic 101MB native vs 417MB JDK | n/a | ~100ms native | Yes but native build 194s and ~7GB RAM | Slow | OK |
| Zig | tiny | tiny | ms | Yes | Fast | Weak: thin ecosystem, low training data |

### Ecosystem coverage for Dimmi needs

| Need | Go | Rust | Elixir | TypeScript |
|---|---|---|---|---|
| Postgres driver | pgx (14.4k stars, MIT) | sqlx (17.6k) | Postgrex | postgres.js, pg |
| Job queue on Postgres | River (5.8k, MPL-2.0, Postgres + SQLite drivers) | apalis, own | Oban (4k) best-in-class | pg-boss (4.1k), graphile-worker |
| WebSockets | coder/websocket (5.5k, ISC) | tokio-tungstenite | Phoenix Channels (gold standard) | ws, uWS |
| MIME parse | enmime (526, active), go-message | mail-parser (Stalwart, strong) | gen_smtp/mail parsers thinner | mailparser (1.7k) |
| DKIM/SPF/DMARC verify | go-msgauth (229 stars, last push 2025-04) | mail-auth (Stalwart, best) | thin | mailauth |
| OIDC / SAML | go-oidc, zitadel/oidc, crewjam/saml | openidconnect-rs, samael | ueberauth, samly | openid-client, better-auth, node-saml |
| AWS S3/SES/KMS/Bedrock | aws-sdk-go-v2 official | aws-sdk-rust official | ex_aws community | aws-sdk-js v3 official |
| MCP SDK | official go-sdk v1.8.0 (Google co-maintained) + mcp-go (9.2k) | official rust-sdk (4.1k) | community only | official TS SDK (13.7k), reference impl |
| Stripe | stripe-go official | community async-stripe | stripity_stripe community | official |
| Slack | slack-go (5k, community) | community | community | official Bolt |
| HTML sanitize | bluemonday (3.7k, last push 2025-04, stable) | ammonia | html_sanitize_ex | DOMPurify (needs DOM; jsdom), sanitize-html archived 2026 |
| Markdown | goldmark (5.1k, CommonMark+GFM) | comrak | Earmark/MDEx | markdown-it, remark |
| i18n | golang.org/x/text + own loader (go-i18n dropped 2026-11-02: binary size) | fluent-rs | gettext | i18next |
| LLM clients | openai-go, anthropic-sdk-go official; Bifrost core; eino | async-openai community | community (req_llm, LangChain-ex) | Vercel AI SDK, official SDKs (richest) |

### Analysis

- Go wins memory per dollar among languages with full ecosystem. Rust lower memory but compile time hurts contributors and agents; ecosystem thinner for Slack/Stripe (community crates). Rust win only on mail parsing/auth (Stalwart crates).
- Elixir strongest challenger. BEAM built for chat: per-process isolation, Phoenix Channels + PubSub across nodes via distributed Erlang (no Redis). Oban best queue anywhere. AutoCodeBench shows frontier models score highest on Elixir. But: idle VM 80-170MB, OOM on 256MB VMs, release not one static binary, distributed Erlang clustering needs node discovery (DNS/libcluster) on PaaS, contributor pool small (2.7% of SO 2025 respondents did extensive Elixir work), AWS SDK community-maintained, no official MCP SDK. Elixir loses on requirements 1, 4, 5 and hiring.
- TypeScript/Bun real contender for DX: one language front and back, official Tier-1-class MCP SDK, Vercel AI SDK, best Slack/Stripe SDKs. Bun owned by Anthropic now; long-term funding solid. Against: RSS 2-6x Go, two production leak writeups in 2026 (Trigger.dev, Autumn), npm supply-chain surface, `--compile` binaries large. Node ~190MB flat under load in same chart. Fails "extremely low memory".
- .NET AOT: memory good, TechEmpower Fortunes leader (ASP.NET ~610k req/s round 23, filtered view [unverified]). But AOT restricts reflection-heavy libs; self-host OSS community and vibe-coder audience lean Go/TS. Skip.
- Kotlin/GraalVM: native image builds need ~7GB RAM and minutes. Kills contributor DX. Skip.
- Zig: no Postgres/OIDC/AWS ecosystem. Skip.
- LLM coding quality: no rigorous benchmark ranks Go vs TS for full-stack agentic work. Evidence: AutoCodeBench (Tencent, 2025) favors Elixir/Kotlin/C#; Multi-SWE-bench reports lower resolve rates for Go/Rust/TS than Java [unverified secondary summary]; practitioner reports (Ronacher 2025, 2026) favor Go for agents due to simple tooling, explicit errors, `go test`, fast compile feedback loop. Go stable syntax for 10+ years means training data never stale (contrast Svelte 4 vs 5 rune confusion).
- Go weak spots to mitigate: email auth libs small-maintainer (go-msgauth 229 stars, last push 2025-04); gorilla/websocket stale (last push 2025-03) so use coder/websocket; SAML libs thin; Slack SDK community-run.

**Pick: Go.** Prior research confirmed, but for different main reason: not only Listmonk RSS; Elixir beats Go on realtime model and LLM benchmark, loses on memory floor, single binary, ecosystem breadth, contributor pool. TypeScript/Bun is runner-up, chosen only if team values one-language over RSS.

Go 1.26 notes: Green Tea GC on by default, 10-40% lower GC overhead. Set `GOMEMLIMIT` from container limit for $5 VPS. Container-aware GOMAXPROCS since 1.25.

## B. Database

### Postgres versions

- PostgreSQL 18 (GA 2025-09): async I/O (up to 3x read speedups), `uuidv7()`, virtual generated columns default, OAuth auth, skip scan on multicolumn B-tree, checksums on by default.
- PostgreSQL 19: RC1 2026-10-15, GA planned 2026-11-19. Adds `REPACK CONCURRENTLY`, parallel autovacuum, `WAIT FOR LSN` (read-your-writes on replicas), sequence logical replication, SQL/PGQ.
- Decision: target 18 as baseline image, support 17+ (most managed hosts), use `uuidv7()` only when 18 detected else app-generated UUIDv7 (Go `google/uuid` v7).

### Heavy-lifting matrix

| Job | Postgres approach | Notes / limits |
|---|---|---|
| Queue | River (SKIP LOCKED, transactional enqueue) | Jobs inserted in same transaction as message row. No dual-write bugs. pgmq skipped: extension not on RDS list checked [unverified], River richer. |
| Cron / scheduler | River periodic jobs + leader election | Skip pg_cron: needs superuser/preload on some hosts, logic outside app. |
| Pub/sub | Valkey PUBLISH/SUBSCRIBE (updated 2026-11-02) | Postgres LISTEN/NOTIFY no longer used for fan-out (commit-lock section kept as background). |
| FTS | `tsvector` + GIN + `websearch_to_tsquery` + `unaccent` + pg_trgm for typo/prefix | ts_rank not BM25; quality OK for help center <100k articles. |
| BM25 | pg_textsearch v1.0 (PostgreSQL license, Timescale) optional; ParadeDB pg_search AGPL | Both need `shared_preload_libraries`; missing on many managed hosts. Optional adapter only. |
| Vector | pgvector 0.8.x HNSW (23.6k stars) | On RDS (0.8.0 for PG18), Supabase, Neon, Railway images. pgvectorscale optional. |
| Analytics / metering | Append-only `usage_events` partitioned by month (native declarative partitioning, partitions created by River job) + hourly/daily rollup tables via River | Skip TimescaleDB (TSL license parts, rare on cheap hosts). pg_partman optional. |
| Multi-tenancy | `workspace_id` on every row, leading column of every index; RLS as defense-in-depth with `SET LOCAL app.workspace_id`, `FORCE ROW LEVEL SECURITY` | Without tenant-leading index RLS ~100x slower [single source]. `SET LOCAL` is transaction-scoped, so transaction poolers work. |
| Cache | Valkey shared cache (`rueidisaside` Remember with client-side caching, `rueidislock` locks) + optional in-process otter for hot keys (updated 2026-11-02) | Valkey required (PO); holds only rebuildable state. |
| Rate limit | Valkey counters (httprate sliding window on rueidis) on every deployment (updated 2026-11-02) | In-memory per replica and UNLOGGED counter table dropped: Valkey required (PO). |

### LISTEN/NOTIFY commit lock: verified status

- Problem: transaction calling NOTIFY takes global exclusive lock at commit, held through fsync; serializes commits, defeats group commit. Recall.ai outage 2025-03-19..22 with tens of thousands of writers. DBOS (2026-07-24) measured ~2.9K writes/s ceiling with per-write NOTIFY trigger; batching NOTIFYs reached ~60K writes/s.
- Fix status: Postgres commit `282b1cde` (2026-01-15, Joel Jacobson, reviewed by Tom Lane) adds shared channel map and direct queue advancement. Commit message: improves throughput when many listeners but few interested; "no improvement for simplest case where every listener reads every message". Ships in PG19. Global commit lock not removed (DBOS confirms). Recall.ai page update claiming bottleneck "eliminated" overstates it. Out-of-order NOTIFY GUC still proposal only.
- Superseded 2026-11-02: realtime and internal pubsub moved to Valkey (centrifuge Redis broker, `framework/pubsub`); Dimmi no longer uses LISTEN/NOTIFY for fan-out. River's own job notifier still LISTENs. Original design kept below as background.
- Original design: never NOTIFY inside hot write transaction. Write message, commit, then app-side publisher batches IDs every ~10-25ms into one `pg_notify` per channel per batch (separate autocommit). Subscribers re-read rows by ID (payload 8000-byte limit irrelevant). Replicas poll `events` cursor every 1-2s as fallback for lost notifications. Expected headroom: low thousands of messages/s, far beyond first users. Above that: swap PubSub adapter to NATS or Redis.

### SQLite/libSQL vs Postgres, and "support both?"

| Factor | SQLite (modernc/ncruces, Litestream) | Postgres |
|---|---|---|
| Ops for $5 VPS | zero extra process | one more container, ~30-60MB idle tuned [unverified] |
| Horizontal scale (req 6) | single writer; LiteFS needs FUSE + primary routing; LiteFS Cloud sunset 2024-10; Fly support limited | native |
| Concurrent writes | one writer; Turso MVCC `BEGIN CONCURRENT` beta, not production-ready | MVCC |
| FTS | FTS5 (BM25 built in, good) | tsvector / BM25 extensions |
| Vector | sqlite-vec | pgvector |
| Pub/sub | none (River emulates via polling table) | LISTEN/NOTIFY |
| RLS | none | yes |
| Managed hosting | Turso cloud only | everywhere |
| Backups | Litestream 0.5 (LTX, PITR, VFS read replicas) excellent | pg_dump / managed PITR |

- Verdict: supporting both is trap for Dimmi. Every heavy feature (FTS, vector, pub/sub, RLS, partitioning, JSONB ops, advisory locks) needs two implementations and two test matrices. sqlc generates per-engine code; queries diverge. Precedent: Umami v3 (2025-11) dropped MySQL to standardize on Postgres; Mattermost v11 deprecates MySQL; n8n says SQLite unsuitable for queue mode; Gitea ships SQLite only via build tag and has no cross-backend migration tool. PocketBase (SQLite-only) explicitly single-node. River now has SQLite driver, so queue alone no longer blocker, but rest still is.
- Make Postgres feel free instead: compose file bundles `postgres:18-alpine` with tuned small-memory configuration (`shared_buffers=64MB`, `max_connections=50`); `install.sh` sets up Postgres + Valkey natively for bare-binary installs (all-in-one omnibus image rejected 2026-11-02: PaaS templates use managed Postgres + Valkey); `fergusstrange/embedded-postgres` for dev/tests only.
- Revisit SQLite only if PocketBase-class demand appears, as separate "Dimmi Lite" edition with reduced features, never same code paths.

### Managed Postgres on cheap hosts

| Host | Postgres | Notes |
|---|---|---|
| Railway | metered, few dollars small DB; HA experimental 2026-03 [unverified] | $5 Hobby min; template deploy easy |
| Render | free 30-day 1GB trial, paid from ~$6-7/month | |
| Fly | Managed Postgres from ~$38/month; self-run Postgres unmanaged | expensive for tiny installs |
| DigitalOcean / Coolify / VPS | compose bundles Postgres + Valkey | cheapest |
| AWS | RDS PG18 (pgvector) + ElastiCache Valkey | Dimmi Cloud only (EC2 + CloudFormation, `specs/12-cloud-billing/infrastructure.md`); no self-host AWS guide |

Required extensions only: `pgvector`, `pg_trgm`, `unaccent` (all widely available). Everything else optional with feature detection.

**Pick: PostgreSQL 18, Postgres-only.**

## C. Frontend

### Dashboard / inbox

| Option | Ecosystem | LLM output quality | Notes |
|---|---|---|---|
| React 19 + Vite | largest; shadcn/ui 127k stars, TanStack Query 50k, Radix 19k, Base UI 11k | best; most training data | Pick |
| Svelte 5 | 88k stars, smaller lib set | LLMs mix Svelte 4/5 syntax; svelte-evals: explicit rules needed, 60% pass rate | Runner-up |
| Solid | 36k stars, small | weaker | Skip |
| Vue 3 | large | good | Chatwoot uses Vue; fine but smaller component ecosystem than React for shadcn-style |

- Component layer: shadcn/ui made Base UI default 2026-07-02; Radix still supported. Base UI 1.x stable since ~2025-12 [unverified exact version]. Pick shadcn/ui on Base UI, Tailwind v4.
- Routing: TanStack Router (type-safe, file routes, SPA). No Next.js: needs Node at runtime, breaks one binary.
- Virtualized lists: TanStack Virtual for inbox.

### Local-first sync evaluation

| Option | Status | Runtime needs | Fit |
|---|---|---|---|
| Zero (Rocicorp) | 1.0 shipped 2026 (symbolic over 0.26); repo latest tag 0.26.4 [conflict, unverified] | separate `zero-cache` Node service, `wal_level=logical`, direct non-pooled connection, persistent volume, 10min shutdown grace | Breaks one-binary rule. Instant UX great. |
| ElectricSQL 1.0 | stable since 2025-03 | separate Elixir sync service, logical replication; read-path only, writes via own API | Same problem. |
| TanStack DB | 1.0 RC (2026-10-31) | client-only; collections fed by any source incl. TanStack Query or custom WS | Good later layer. |
| Replicache | archived | | Dead. |
| TanStack Query + WS patches | mature | none | Pick |

- Pick: TanStack Query cache as client store. Go hub pushes typed events (`conversation.updated`, `message.created` with full row) over WebSocket; client patches query cache via `setQueryData`; optimistic mutations with rollback. Gives near-instant inbox without second server. Add TanStack DB once 1.0 stable for live joins/filters on client. Persist cache to IndexedDB for warm start.

### Editor

- Tiptap 3: MIT core on ProseMirror, 38.9k stars, Markdown extension, Yjs collaboration if needed, rich extension set. Paid extras (AI, comments) avoidable.
- Lexical: 24k stars, still 0.x (0.42 [unverified]), smaller/faster but less Markdown tooling.
- Pick Tiptap for help center editor + composer. Store canonical Markdown (help center) so AI drafts from PRs produce diffable Markdown; render with goldmark server-side.

### Widget (Messenger)

- Budget: Intercom loader ~3KB then ~170KB + ~91KB compressed stage-two assets; Lighthouse dropped >90 to 77 on one site [single source]. Dimmi target: loader <3KB, widget <35KB gz, lazy-load emoji/attachments/markdown.
- Sentry evaluated Preact vs Svelte for embedded widget: Svelte runtime ~2.7KB, Preact ~4.5KB, total widget size close.
- Lit: native Shadow DOM, ~5KB core; class-based, weaker LLM fluency.
- Pick: Preact + `preact/hooks`, rendered inside Shadow DOM root (manual `attachShadow`, styles injected as constructable stylesheet), Vite library mode, iframe only for sensitive surfaces (attachment upload, identity verification). Same TS types and API client as dashboard. Runner-up Svelte 5 custom element.
- Realtime in widget: plain `EventSource` on centrifuge uni-SSE (60 s connect token), REST for sends and typing; polling only when a proxy blocks streaming (updated 2026-11-02).

### Help center

- Go SSR with templ (10.6k stars, type-safe templates) + goldmark + bluemonday. Cache rendered HTML per article version in memory, invalidate via NOTIFY. Zero JS by default, tiny island for search box and widget. Custom domains via Host header + autocert (or Caddy in front).
- Astro SSG runner-up: needs Node build per publish; breaks "one binary" unless pre-rendered by worker invoking external toolchain. Skip.

### Embedding into binary

- Monorepo: `web/dashboard`, `web/widget`, `web/help` (CSS only), built by Vite, output into `internal/assets/dist`, embedded with `//go:embed all:dist`. Hashed filenames, immutable cache headers, `index.html` no-cache. Dev mode: Go proxies to Vite dev server. Same pattern as Listmonk, PocketBase admin, Gitea.

## D. Monolith strategy

### Process roles

- One binary, subcommands via cobra: `dimmi serve` (default: web + worker + scheduler all on), `dimmi worker`, `dimmi migrate`, `dimmi backup`, `dimmi admin create-user`, `dimmi mcp` (stdio MCP server).
- Roles toggled by env `DIMMI_ROLES=web,worker,scheduler`. River leader election picks single scheduler among replicas; safe to run all roles everywhere.
- Graceful shutdown: drain WebSockets with reconnect hint, River stop with job timeout.

### Configuration

- koanf: defaults < `dimmi.yaml` < env `DIMMI_*`. Only required value: `DATABASE_URL`. Secrets from env or file paths (`_FILE` suffix for Docker secrets). `dimmi config:show` shows effective configuration with secrets redacted (env nesting `DIMMI_<SECTION>__<KEY>`).

### Migrations

- goose with SQL files embedded (`embed.FS`), run automatically at boot under Postgres advisory lock (safe with multiple replicas); `--no-migrate` flag for controlled rollouts. Expand/contract rule: every release must run against previous schema version (zero-downtime rolling deploys). Atlas runner-up for schema diffing in CI.

### Background jobs

- River: transactional enqueue, retries with backoff, unique jobs, periodic jobs, queues per workload (`email`, `ai`, `webhooks`, `rollups`), River UI at `/admin/jobs` deferred (not in framework yet, updated 2026-11-02). AI jobs low concurrency; respect BYOK provider rate limits.

### Realtime across replicas (updated 2026-11-02)

- Embedded centrifuge node per replica; centrifuge Redis broker + presence manager on Valkey give cross-node fan-out, presence, history and recovery. No sticky sessions. Details: `specs/00-foundation/events-and-realtime.md`.
- Superseded design (kept for history): own hub + Postgres NOTIFY batching + `realtime_events` cursor table; Centrifugo Postgres mode sidecar.

### Cache and ephemeral state: Valkey (updated 2026-11-02)

- PO decision: Valkey required in every deployment (app + Postgres + Valkey, incl. $5 VPS). Holds only rebuildable state: realtime, cache, rate counters, internal pubsub, replay/dedupe windows (`SET NX EX`), typing/collision counters. Loss = reset, never data loss. One Go client: `redis/rueidis` (already linked by centrifuge).
- otter v2 in-process only where a spec asks for per-process hot cache.
- Superseded: "no Redis" stance and UNLOGGED cache/rate-limit tables.

### File storage

- `BlobStore` interface. Default: local disk (`./data/uploads`) for single node. Required: S3-compatible when replicas > 1 (aws-sdk-go-v2 S3 client works with R2, Backblaze B2, Garage, SeaweedFS, RustFS). Warning: MinIO GitHub repo archived ("no longer maintained", 2026); do not recommend MinIO in docs. Recommend SeaweedFS (Apache-2.0, 35.9k) or RustFS (Apache-2.0, 35.6k) or Garage for self-host S3.
- Signed URLs for downloads; virus scanning hook optional.

### Email

- Out (updated 2026-11-02, `specs/00-foundation/contracts.md` "Mail"): one instance-level provider from deploy config: SES, Postmark, Resend, Mailgun, SendGrid, or SMTP relay (wneessen/go-mail). Two named streams never mixed: `system` (members) and `customer` (contacts, sent only by 04 actions). No per-workspace provider credentials, no outbound MTA, no direct-to-MX.
- In: `mailbox.Source` drivers: SES receipt rule to S3 + SNS at MVP. Later: provider inbound webhooks (Postmark, Mailgun, SendGrid, Cloudflare Email Routing; `extra-mail-adapters`), IMAP IDLE via emersion/go-imap v2 (`imap-adapter`). No SMTP receiver (PO decision: mail only through instance provider).
- Parse with enmime; verify DKIM/SPF/DMARC with go-msgauth; strip quoted replies (port of GitHub email_reply_parser, small own code); thread via Message-ID/In-Reply-To/References plus reply+token address.

### Plugin/adapter architecture

| Approach | Pros | Cons | Use |
|---|---|---|---|
| Compiled-in Go interfaces (database/sql, Caddy modules pattern) | fast, typed, one binary, easy for LLMs | community adapter needs PR or custom build (`xcaddy`-style builder) | Default for storage, mail, LLM, KMS, PubSub |
| hashicorp/go-plugin (gRPC subprocess) | runtime loading | extra processes, versioning, memory | Skip |
| Extism / WASM | sandboxed user hooks, multi-language | runtime overhead, immature host APIs | Later: per-workspace custom routing/AI tool scripts |
| Outbound webhooks + REST + MCP | zero coupling | network hop | Integrations default; AI agents as operators |

### Observability

- `log/slog` JSON logs; OpenTelemetry Go SDK (traces, metrics) off by default; OTLP trace exporter opt-in at build (`framework/telemetry/otlp`, keeps gRPC out of default binary; own OTLP HTTP exporter on otlptrace + protowire planned), then enabled by config; Prometheus `/metrics`; optional sentry-go. pprof on admin port. Per-conversation usage metering (tokens, cost, tool calls) stored in Postgres for product use, separate from telemetry.

### Upgrades and backups

- Upgrades: replace image tag; migrations auto-run under lock; expand/contract keeps rolling deploys safe. `dimmi doctor` checks Postgres version, extensions, disk, S3 reachability, DNS for email.
- Backups: `dimmi backup` wraps `pg_dump` (custom format) + uploads to BlobStore; compose includes optional nightly backup sidecar. Managed hosts: rely on PITR.

### Multi-tenant cloud vs single-tenant self-host

- Same codebase, always multi-tenant schema (`workspace_id` everywhere). Self-host = one workspace by default, more allowed. Cloud adds billing module (Stripe), quotas, noisy-neighbor limits, behind build tag or config flag. Avoid "enterprise edition fork". RLS defense-in-depth on cloud.
- Secrets (BYOK LLM keys, integration tokens): envelope encryption via `KeyManager` interface; adapters: local key from env (AES-GCM, default), AWS KMS, later GCP KMS/Vault.

### Packaging

- Updated 2026-11-02 (PO): two self-host paths plus templates. (1) `docker-compose.yml`: dimmi + `pgvector/pgvector:pg18` + Valkey 8 (+ optional Caddy). (2) `install.sh`: bare binary + systemd, installs Postgres 18 + pgvector and Valkey from apt unless `--database-url` / `--redis-url` point at existing ones; no Docker. One-click templates (Railway, Render, Fly, DigitalOcean, Coolify, Dokploy) use platform-managed Postgres + Valkey/Redis. No all-in-one image, ever. Dimmi Cloud: same binary on EC2 (systemd, no containers), RDS + ElastiCache, CloudFormation.
- goreleaser: linux/darwin/windows amd64/arm64 binaries, SBOM, cosign signatures; one Docker image (Debian slim + `pg_dump` 18 client for backups). CGO off (pure-Go deps only; no SQLite cgo).
- Helm chart: later.

### Lessons from comparable projects

| Project | Stack | Got right | Got wrong / lesson |
|---|---|---|---|
| Listmonk | Go + Postgres + embedded Vue | single binary, 57MB peak RSS [vendor], only Postgres | AGPL limits some adopters |
| PocketBase | Go + SQLite | one file, embedded admin | single node only; moved off GitHub 2026-10 citing LLM spam, repo archived, releases now self-hosted |
| Gitea/Forgejo | Go, multi-DB via xorm | easy install | multi-DB testing burden; SQLite build-tag only; no cross-DB migration |
| Plausible CE | Elixir + Postgres + ClickHouse | great product | ClickHouse needs ~2GB RAM, SSE4.2 CPU; 3 services |
| Umami | Next.js + Postgres | simple | dropped MySQL in v3; Node memory |
| Chatwoot | Rails + Sidekiq + Redis + Postgres | features | 4GB min (10k convs/day); Sidekiq leaks to 1GB+ |
| Mattermost | Go + Postgres | 2GB RAM for 1k users | dropping MySQL in v11 |
| Supabase | ~12-15 containers | power | 4GB floor; self-host complexity |
| n8n | Node + Postgres (+Redis in queue mode) | templates | horizontal scaling needs Redis; SQLite not for queue mode |
| Ghost | Node + MySQL 8 | one-command install | single DB vendor lock [unverified "only"] |
| Coolify | PHP/Laravel + Postgres + Redis | self-host UX | Redis dependency |

Pattern: winners ship one process + one DB, embed UI, auto-migrate. Losers add Redis/ClickHouse/Kafka "for scale" and punish small installs. Dimmi update 2026-11-02: Valkey added as required companion (PO), constrained to rebuildable state and about 10 MB idle so $5 VPS stack stays under 256 MB.

## E. "Don't build" list

Stars and last push from GitHub API 2026-11-01.

| Subsystem | Library | License | Stars | Last push | Notes |
|---|---|---|---|---|---|
| HTTP router | go-chi/chi | MIT | 23.0k | 2026-10 | stdlib-compatible |
| OpenAPI from types | danielgtaylor/huma | MIT | 4.5k | 2026-10 | spec drives CLI + MCP tool schemas |
| Postgres driver | jackc/pgx v5 | MIT | 14.4k | 2026-10 | |
| Typed SQL | sqlc-dev/sqlc | MIT | 18.4k | 2026-10 | |
| Migrations | pressly/goose | MIT-style | 11.6k | 2026-10 | runner-up ariga/atlas (8.8k, Apache-2.0); uses database/sql via pgx stdlib adapter (size accepted, PO 2026-11-02) |
| Jobs/cron/leader | riverqueue/river | MPL-2.0 | 5.8k | 2026-11 | v0.51.0; River Pro paid extras optional |
| WebSocket | coder/websocket | ISC | 5.5k | 2026-10 | gorilla/websocket stale (2025-03) |
| Realtime | centrifugal/centrifuge (library) | MIT | 1.5k | 2026-11 | embedded; Redis broker on Valkey (updated 2026-11-02) |
| Valkey client | redis/rueidis | Apache-2.0 | | | one client, shared with centrifuge [verify version at build] |
| Tests | docker compose (root `compose.yaml`) | | | | real Postgres (`pgvector/pgvector:pg18`) + real Valkey + SeaweedFS S3 via the app's env vars; testcontainers-go not used, MinIO not used (images no longer pullable), embedded-postgres rejected (no pgvector) (updated 2026-11-02) |
| Cache | maypok86/otter | Apache-2.0 | 2.7k | 2026-10 | |
| Rate limit | go-chi/httprate | MIT | 479 | 2026-10 | ulule/limiter stale (2024-12) |
| Config | knadh/koanf | MIT | 4.2k | 2026-10 | viper heavier |
| CLI | spf13/cobra | Apache-2.0 | 44.7k | 2026-07 | |
| Templates (SSR) | a-h/templ | MIT | 10.6k | 2026-10 | |
| Markdown | yuin/goldmark | MIT | 5.1k | 2026-10 | |
| HTML sanitize | microcosm-cc/bluemonday | BSD-3 | 3.7k | 2025-04 | stable, low churn; watch |
| HTML to text | k3a/html2text | MIT | 192 | 2026-10 | |
| MIME parse | jhillyerd/enmime | MIT | 526 | 2026-10 | |
| DKIM/SPF/DMARC | emersion/go-msgauth | MIT | 229 | 2025-04 | thin bus factor; fallback port Stalwart logic |
| SMTP send | wneessen/go-mail | MIT | 1.5k | 2026-10 | |
| IMAP (later: `imap-adapter`) | emersion/go-imap v2 | MIT | 2.4k | 2026-11 | not at MVP |
| OIDC client | coreos/go-oidc + x/oauth2 | Apache-2.0 | 2.5k | 2026-10 | |
| OIDC provider (if Dimmi issues tokens for MCP OAuth) | zitadel/oidc | Apache-2.0 | 1.9k | 2026-10 | MCP auth spec uses OAuth 2.1 |
| SAML SP | crewjam/saml | BSD-2 | 1.1k | 2026-01 | or delegate to Ory Polis (ex BoxyHQ, 2.3k) |
| Passkeys | go-webauthn/webauthn | BSD-3 | 1.4k | 2026-10 | |
| Sessions | alexedwards/scs (Postgres store) | MIT | 2.6k | 2025-11 | |
| MCP server | modelcontextprotocol/go-sdk | Apache-2.0/MIT [unverified, NOASSERTION in API] | 5.3k | 2026-11 | v1.8.0; runner-up mark3labs/mcp-go (9.2k, MIT) |
| LLM clients | openai/openai-go, anthropics/anthropic-sdk-go, aws-sdk-go-v2 bedrockruntime (Converse) | Apache-2.0 | 3.5k / 1.2k / 3.7k | 2026-11 | thin own `LLM` interface; OpenAI-compatible base URL covers Ollama, OpenRouter, Groq, vLLM |
| LLM gateway alt | maximhq/bifrost (core usable as Go library) | Apache-2.0 | 9.3k | 2026-11 | consider if provider matrix grows; eino (13.5k) too framework-heavy |
| AWS | aws/aws-sdk-go-v2 | Apache-2.0 | 3.7k | 2026-10 | S3, SES, KMS, Bedrock |
| Stripe | stripe/stripe-go | MIT | 2.7k | 2026-10 | Billing Meters for cloud usage billing |
| Slack | slack-go/slack | BSD-2 | 5.0k | 2026-11 | |
| GitHub | google/go-github | BSD-3 | not fetched | | PR-to-article pipeline |
| i18n (server) | golang.org/x/text (language matching, CLDR plurals) + own JSON loader, `{{.name}}` placeholders | BSD-3 | | | nicksnyder/go-i18n removed 2026-11-02 (text/template disables linker dead method elimination) |
| Binary size guards | own (PO 2026-11-02) | | | | removed gocloud.dev (filesystem on aws-sdk-go-v2 s3 + stdlib `os.Root`), creasty/defaults (own `default` tag applier), go-playground/validator (own rule engine, same `validate:"..."` tags), go-i18n; gRPC out of default build. Realistic app 63.9 MB to 36.0 MB stripped (12.3 MB gzipped). Added: bep/imagemeta (EXIF orientation), sergeymakinen/go-ico (ICO), go.uber.org/zap (certmagic log bridge only), golang.org/x/time (local rate limiter). chroma kept (+4.4 MB accepted). `tests/arch` binary test fails on `<ReflectMethod>` |
| Telemetry | open-telemetry/opentelemetry-go | Apache-2.0 | 6.6k | 2026-11 | |
| Release | goreleaser/goreleaser | MIT | 16.1k | 2026-11 | ko (8.6k) alt |
| Base image | GoogleContainerTools/distroless | Apache-2.0 | 23.2k | 2026-10 | |
| Vector search | pgvector | PostgreSQL | 23.6k | 2026-10 | |
| BM25 (optional) | timescale/pg_textsearch | PostgreSQL | 4.1k | 2026-11 | v1.0 2026-04; avoid ParadeDB AGPL as hard dep |
| Frontend UI | shadcn/ui + Base UI | MIT | 127k / 11.3k | 2026-10 | |
| Server state | TanStack Query | MIT | 50.5k | 2026-11 | |
| Router | TanStack Router | MIT | 15.3k | 2026-11 | |
| Client DB (later) | TanStack DB | MIT | 4.0k | 2026-10 | 1.0 RC |
| Virtual lists | TanStack Virtual | MIT | 7.2k | 2026-11 | |
| Editor | ueberdosis/tiptap | MIT | 38.9k | 2026-10 | |
| Widget runtime | preactjs/preact | MIT | 39.0k | 2026-10 | |
| Positioning | floating-ui | MIT | 32.8k | 2026-10 | widget popovers |
| CSS | tailwindcss v4 | MIT | 98.3k | 2026-11 | widget: prefixed, inside Shadow DOM |
| Frontend i18n | i18next or Lingui | MIT | not fetched | | widget uses tiny own dictionary |
| Client sanitize | cure53/DOMPurify | Apache-2.0/MPL | 17.5k | 2026-11 | sanitize-html archived |

Metering/billing:

| Option | Deps | Verdict |
|---|---|---|
| Own `usage_events` + rollups in Postgres | none | Pick for product metering (per-conversation tokens, resolutions) |
| Stripe Billing Meters | Stripe | Pick for Dimmi Cloud invoicing; push rollups |
| Lago (AGPL, 10.7k) | Postgres + Redis, ClickHouse/Redpanda for analytics | Too heavy to bundle |
| OpenMeter (Apache-2.0, 2.4k) | Kafka + ClickHouse + Postgres | Too heavy to bundle; possible cloud-only adapter |

## Risks and mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| NOTIFY commit lock under write burst | inbox stalls | resolved 2026-11-02: realtime and pubsub on Valkey, no NOTIFY on hot path |
| Go email-auth libs small maintainers | DKIM bugs, spoofed tickets | pin + vendor, fuzz tests, fall back to "unverified sender" UX, contribute upstream |
| bluemonday low churn | XSS in help center/email render | defense in depth: CSP, sandboxed iframes for email HTML, DOMPurify client-side |
| Contributors prefer TypeScript | slower community growth | frontend is TS anyway; Go backend kept boring (sqlc, chi, River); CLAUDE.md/AGENTS.md with conventions |
| No SQLite mode | some vibe coders want zero-DB | compose or `install.sh` set up Postgres + Valkey; Railway/Render templates provision managed services automatically |
| BM25 quality gap with ts_rank | worse help-center search | hybrid FTS + pgvector RRF; optional pg_textsearch adapter where available |
| Postgres 19 behavior changes | upgrade friction | CI matrix 17/18/19 |
| Local-first expectations (Linear-like speed) | UX gap | optimistic updates + WS patches + IndexedDB persistence now; TanStack DB later |
| Widget bloat over time | merchant site slowdowns | CI bundle-size budget (size-limit), lazy chunks |
| MinIO archived | broken self-host docs | recommend SeaweedFS/RustFS/Garage/R2 |
| BYOK key leakage | security incident | envelope encryption via KeyManager, never log keys, per-workspace scoping |
| PocketBase-style platform risk (GitHub spam, policy) | contributor friction | mirror releases to own download host; keep build reproducible |

## Open questions / unverified

- WebSocket per-connection memory across Go/Rust/Elixir/Bun: no trustworthy 2025-2026 benchmark found; community numbers contradict. Run own test (10k idle + 1k active, same box, `docker stats`) before claims in marketing.
- Exact idle RSS for Dimmi binary: unmeasured; target <50MB idle, <150MB at 1k concurrent WS.
- Zero 1.0 version: InfoQ says 1.0 shipped mid-2026; GitHub latest release tag shows 0.26.4. Unresolved.
- MCP go-sdk tier in official SDK tiering (published 2026-02-23): tier assignment not confirmed.
- pgmq availability on RDS: not found in RDS extension list checked.
- Base UI exact stable version history: sources conflict.
- Lexical 1.0 status: still 0.x per aggregator [unverified].
- Railway HA Postgres status and pricing: third-party sources only.
- Centrifuge Go library support for Postgres broker: confirmed none (build-vs-reuse.md); moot after Valkey decision.
- Multi-SWE-bench per-language numbers for Go vs TypeScript: secondary summaries only.
- TechEmpower: project reportedly archived 2026-03 [unverified]; round 23 last data.
- Should cloud edition use Postgres schema-per-tenant for large customers? Defer.

## Sources

1. https://github.com/riverqueue/river (and CHANGELOG, riverdriver/riversqlite)
2. https://brandur.org/river
3. https://github.com/pgmq/pgmq
4. https://github.com/pgvector/pgvector
5. https://github.com/paradedb/paradedb
6. https://github.com/timescale/pg_textsearch
7. https://www.postgresql.org/about/news/pg_textsearch-v10-3264
8. https://postgresql.org/about/news/postgresql-18-released-3142/
9. https://www.postgresql.org/about/news/postgresql-19-rc-1-released-3409/
10. https://postgresql.org/about/news/postgresql-19-beta-1-released-3313/
11. https://github.com/postgres/postgres/commit/282b1cde9dedf456ecf02eb27caf086023a7bb71
12. https://dbos.dev/blog/postgres-listen-notify-scalability
13. https://recall.ai/blog/postgres-listen-notify-does-not-scale
14. https://postgrespro.com/list/id/CAK80=jiWjMAfjwJymPoG3=FNE=hHygn5dXaoax1BWQT1rJrCCw@mail.gmail.com
15. https://go.dev/doc/go1.26
16. https://lucumr.pocoo.org/2025/6/12/agentic-coding.md
17. https://lucumr.pocoo.org/2026/2/9/a-language-for-agents/
18. https://arxiv.org/html/2508.09101v1 (AutoCodeBench)
19. https://codemyspec.com/blog/autocodebench-explained
20. https://arxiv.org/pdf/2208.08227 (MultiPL-E)
21. https://benchmarklist.com/benchmarks/multi_swe_bench/
22. https://survey.stackoverflow.co/2025/technology/
23. https://elixirforum.com/t/stack-overflow-developer-survey-2025/71073
24. https://www.freecodecamp.org/news/million-websockets-and-go-cc58418460bb/
25. https://github.com/dsander/phoenix-connection-benchmark
26. https://elixirconf.com/legacy-content/2016/gary-rennie/
27. https://community.fly.io/t/oom-on-small-elixir-phoenix-application/25850
28. https://trigger.dev/blog/firebun
29. https://useautumn.com/blog/eliminating-memory-leaks
30. https://anthropic.com/news/anthropic-acquires-bun-as-claude-code-reaches-usd1b-milestone
31. https://stackharbor.com/en/knowledge-base/paas-runtime-dotnet-core-trim-aot-image-size/
32. https://www.slideshare.net/slideshow/spring-boot-native-written-by-software-developers/277142729
33. https://rust-lang.github.io/rust-project-goals/2026/roadmap-fast-builds.html
34. https://dev.to/tuananhpham/popular-backend-frameworks-performance-benchmark-1bkh
35. https://github.com/modelcontextprotocol/go-sdk
36. https://modelcontextprotocol.io/community/sdk-tiers
37. https://listmonk.app
38. https://developers.chatwoot.com/self-hosted/deployment/requirements
39. https://github.com/plausible/community-edition
40. https://docs.umami.is/docs/guides/migrate-mysql-postgresql
41. https://docs.mattermost.com/deployment-guide/software-hardware-requirements.html
42. https://docs.n8n.io/hosting/scaling/queue-mode/
43. https://findstack.com/resources/best-vps-for-supabase-self-hosted
44. https://github.com/pocketbase/pocketbase/discussions/7841
45. https://community.fly.io/t/sunsetting-litefs-cloud/20829
46. https://simonwillison.net/2025/Oct/3/litestream
47. https://turso.tech/blog/turso-0.5.0
48. https://zero.rocicorp.dev/docs/self-host
49. https://infoq.com/news/2026/06/zero-version-1
50. https://electric.ax/blog/2025/03/17/electricsql-1.0-released
51. https://github.com/TanStack/db/releases
52. https://blog.openreplay.com/shadcn-ui-radix-base-ui-switch/
53. https://shadcnstudio.com/blog/migrate-from-radix-ui-to-base-ui
54. https://devpick.pages.dev/lexical-vs-tiptap-core
55. https://blog.sentry.io/preact-or-svelte-an-embedded-widget-use-case/
56. https://www.intercom.com/blog/reducing-intercom-messenger-bundle-size/
57. https://www.greengeeks.com/blog/ai-chatbot-website-loads-slow-fixes/
58. https://sveltethemes.dev/nkwib/svelte-evals
59. https://centrifugal.dev/blog/2026/05/25/pg-controller-multi-node
60. https://openmeter.io/docs/open-source/architecture
61. https://doc.getlago.com/guide/lago-self-hosted/compatibility-matrix
62. https://docs.aws.amazon.com/AmazonRDS/latest/PostgreSQLReleaseNotes/postgresql-extensions.html
63. https://dev.to/ashwin_sridhar_koto7/does-postgres-rls-actually-ruin-performance-lets-look-at-the-data-24jf
64. https://instapods.com/compare/flyio-vs-railway/
65. https://www.morphllm.com/comparisons/flyio-vs-render
66. https://techsy.io/en/blog/railway-vs-render-vs-fly-io
67. https://github.com/minio/minio (README: no longer maintained)
68. https://github.com/maximhq/bifrost
69. GitHub API repo metadata (stars, license, last push) for ~150 repos listed in section E and tables above, fetched 2026-11-01 via `gh api repos/<owner>/<repo>`.
