# 11 Self-host: packaging and operations

Style: caveman. Scope trim 2026-11-02: inbound mail = SES only (later: `imap-adapter`, `extra-mail-adapters`); `dimmi listen` proxy note dropped (later: `dimmi-listen`). Read with `specs/00-conventions/README.md`, `specs/00-foundation/*`. Extra files: [packaging.md](packaging.md) (image, release, compose, templates, install.sh + systemd), [operations.md](operations.md) (config, first run, upgrades, backups, doctor, maintenance, proxy, scale, budget, hardening). Final sweep 2026-11-02: Valkey required in every deployment (PO decision), framework reuse (automemlimit, realclientip-go, goose session locker, age backups, go-systemd), mail final model (04), realtime on centrifuge with Redis broker on Valkey, feedback (14) doctor checks, integration-only app tests. Deployment simplification 2026-11-02 (PO): one image, two self-host paths (compose, bare binary) + six one-click templates on managed services; no single-container bundle ever; no AWS guide for self-hosters (AWS = Dimmi cloud only, 12).

## Goal

Anyone runs full Dimmi on own box in one command: one static binary + Postgres 18 + Valkey 8, under 300 MB RAM idle (fits $5 VPS, 1 GB), first message in inbox under 5 minutes. Upgrades, backups, health checks, export built in. No license key, no enterprise edition, no telemetry.

Why: roadmap launch item 11 ("One-command self-host"), `research/roadmap.md`. Core promise vs Fin/Salesforce lock-in; no competitor in set self-hosts well (Chatwoot needs 4 GB+).

## World-class bar (measurable)

| Bar | Target | How verified |
|---|---|---|
| Fresh VPS to first widget message | under 5 minutes, 1 command (`curl ... \| sudo bash` or `docker compose up -d`) | timed demo script in CI (tasks.md "Done when") |
| Whole compose stack idle (dimmi + postgres + valkey + caddy) | under 300 MB (dimmi 50, postgres 180, valkey 20, caddy 25 + headroom), p95 over 5 min after 10 min warmup | CI `idle-budget` job, cgroup v2 method (operations.md section 9) |
| Dimmi binary idle RSS | under 50 MB | same job, per container |
| Dimmi binary size (linux/amd64, stripped) | about 36 MB (12.3 MB gzipped) measured on realistic app build 2026-11-02 (was 63.9 MB before removing libs that disable linker dead method elimination); OTLP/gRPC not linked by default | `tests/arch` binary test fails CI on any `<ReflectMethod>` marker in linker dump |
| Image `dimmi:<v>` (Debian slim + pg_dump 18) | under 80 MB compressed per arch [verify after first build] | CI fails over budget |
| Cold boot, no pending migrations | under 1 s to `/ready` 200 | CI benchmark |
| Upgrade | pull new image + restart; zero manual SQL; rolling deploy safe between adjacent minor releases | upgrade test matrix N-1 to N, N-2 to N |
| Backup | `dimmi backup` 1 GB database under 2 min on 2 vCPU; restore tested in CI every release | CI backup-restore round trip |
| `dimmi doctor` | full run under 5 s, every failed check prints fix command or doc link | feature tests (real Postgres + Valkey) |
| Release supply chain | every binary and image signed (cosign keyless), SBOM (SPDX), checksums, provenance attestation | `cosign verify` in release CI |
| Phone home | zero by default except update check to `api.github.com` (opt-out, one env var) | network test: boot with egress logged, only allowed host seen |
| Platforms | linux, darwin, windows x amd64, arm64 binaries; images linux/amd64 + linux/arm64 | goreleaser matrix |

## Scope

### In (MVP)

- Release pipeline: goreleaser, 6 binaries, 1 multi-arch image, cosign, SBOM, checksums, GitHub Releases + `ghcr.io`.
- `docker-compose.yml`, three containers: dimmi image + postgres 18 with pgvector + valkey 8 (`valkey/valkey:8-alpine`, AOF `everysec`, `maxmemory 64mb`, `noeviction`), tuned for small memory + optional Caddy profile (auto TLS).
- One-click templates: Railway, Render blueprint, Fly `fly.toml`, DigitalOcean App Platform, Coolify, Dokploy. Each uses platform managed Postgres + managed Valkey/Redis (Fly: Managed Postgres + Upstash Redis); Coolify and Dokploy run the same 3-service compose.
- Bare binary, no Docker: `install.sh` (bash) downloads binary, installs Postgres 18 + pgvector and Valkey 8 from apt (Debian/Ubuntu) by default, writes config, installs systemd unit, starts Dimmi; `--database-url` / `--redis-url` reuse services already on the box and skip that install. Manual `dimmi serve` with env documented.
- Config: `dimmi.yaml` + env `DIMMI_<SECTION>__<KEY>`, `PORT`, `DATABASE_URL` and `REDIS_URL` PaaS conventions (aliases: framework follow-up), `_FILE` secrets.
- First run: app key auto-generate to data dir (images; bare binary needs `--generate-key`), setup token, first user via 00-accounts action, `dimmi user:create`.
- Upgrades: auto-migrate (`migrations.auto`) under goose Postgres session advisory lock (framework `database/migrate`), expand/contract, version skew guard (`dimmi_releases`), `dimmi migrate:plan`, pre-migrate backup.
- Backups: `dimmi backup`, `dimmi restore`, scheduled backups, retention, optional age encryption (filippo.io/age), backups table + admin API. Postgres only: Valkey never backed up (rebuildable state).
- `dimmi doctor` (CLI, API, MCP), `dimmi down` / `dimmi up` maintenance mode (soft built in framework; hard mode + queue pause: framework follow-up).
- System settings table + admin API (mail test send per named mailer `system` / `customer`, storage test, URL base, registration policy, update check toggle). Registry itself is foundation `app/support/settings` (scoped: instance, organization, workspace, user; `00-foundation/extensibility.md` section 7); 11 registers instance-scope keys and owns `system_settings` storage.
- Entitlements `self_host` driver (extensibility.md section 6): every feature true, limits from config (no cloud caps on self-host).
- Instance identity exposure: `region` (`app.region`, self-host default `self`) and `server_version` in `GET /api/v1/auth/config` and `/me.instance` (capability discovery P7; 00-accounts renders, 11 supplies values).
- Update notifications (GitHub releases, opt-out).
- Data export: whole instance (instance admin) and single workspace (owner). JSONL + attachments, zip.
- Reverse proxy guide, app scale guide, backing-service connection contract (Postgres and Valkey topologies; operating them out of scope, decision 20), trusted proxies config, node heartbeat registry.
- Security hardening defaults (nonroot, read-only rootfs, admin port local, signup `invite_only` after first user).
- Resource budget measurement method + CI job + published numbers.

### Out (later)

- AWS deploy guidance for self-hosters: none. AWS infra is Dimmi cloud only (12). S3-compatible storage and SES mail drivers stay (drivers, not deploy guides).
- Helm chart (later; demand-driven). When built: Valkey as required dependency (subchart or external `redis.url`).
- Single container bundling app + Postgres + Valkey: never (PO 2026-11-02; not a later item).
- Elestio listing (vendor-run, after launch).
- Point-in-time recovery (WAL archiving). MVP: logical dumps; managed hosts use provider PITR.
- Importers: launch: importers, owned by `16-importers` (11 documents config keys + doctor check only; operations.md "Feature config notes").
- Telemetry of any kind: not planned. Decision below.
- License keys: never. No enterprise edition.
- `extra-mail-adapters` (audit B6) / `imap-adapter` (audit C1): self-host inbound via Cloudflare Email Routing worker, Postmark/Mailgun inbound webhooks, IMAP: later. MVP inbound = SES receiving; outbound drivers unchanged.

### Decisions

1. **Telemetry: none.** No anonymous usage stats, no install ping, no crash reporting by default. Optional `sentry-go` / OTLP only when operator sets DSN/endpoint (foundation). OTLP tracing is opt-in at build (`framework/telemetry/otlp`, `otlp.Tracing()` in `foundation.Options.Telemetry`): default build links no gRPC. Reason: audience distrusts phone-home; no business need (self-host free, no license).
2. **Only outbound call by default:** update check, `GET https://api.github.com/repos/dimmihq/dimmi/releases/latest` once per 24 h (jittered), `User-Agent: dimmi` (no version, no instance id). Opt-out `DIMMI_UPDATES__CHECK=false`. GitHub org name pending (`research/naming.md` suggests `dimmihq`); config key `updates.repository`.
3. **No license key, no edition gate** on self-host. `app.edition = self_host` only disables cloud modules (billing, RLS enforcement, abuse). Every feature available.
4. **Deployment paths (PO 2026-11-02).** Exactly two self-host paths: compose (3 containers: dimmi, `pgvector/pgvector:pg18`, Valkey 8; optional Caddy profile) and bare binary (`install.sh` or manual `dimmi serve`). Plus six one-click templates on platform managed Postgres + Valkey/Redis. Postgres and Valkey always separate processes; Dimmi never supervises them.
5. **Postgres image:** official `postgres:18` lacks pgvector. Compose uses `pgvector/pgvector:pg18` image (official pgvector image, Debian; release copy pins `<pgvector>-pg18-trixie` tag + digest, Renovate bump). PG18 data volume mounted at `/var/lib/postgresql` (PG18 layout, PGDATA `/var/lib/postgresql/18/docker`), not old `/var/lib/postgresql/data`.
6. **One image** `dimmi:<v>`: Debian slim + `postgresql-client-18` (apt.postgresql.org), nonroot uid 65532, read-only rootfs compatible. Used by compose, every template, any container platform. Old distroless + pg-client split dropped (PO 2026-11-02): backups work in every container deploy; size cost accepted.
7. **Backups need pg_dump.** Image ships pg_dump 18; `install.sh` always installs `postgresql-client-18`. Backup jobs run on dedicated River queue `backups`; only processes with compatible `pg_dump` on PATH subscribe to it (manual bare binary on host without client never claims). Doctor flags "backups enabled, no capable worker".
8. **Restore CLI only.** Restore needs hard maintenance and process restart; never exposed over HTTP.
9. **Registration after first user: `invite_only`** default (00-accounts invitations). Instance admin may set `open` or `closed`.
10. **Instance admin** = new flag `users.is_instance_admin`. First user gets it. Distinct from workspace owner. Cloud: Dimmi staff only, 2FA required.
11. **App key.** Missing key: compose and the container image auto-generate on first boot (images set `DIMMI_APP__KEY_AUTO_GENERATE=true`), persist to data volume `<data_dir>/app.key` (`/var/lib/dimmi/app.key` in images, `storage/app.key` for local/bare runs), mode 0600, reused every later boot. Bare binary (no image env) without key fails boot with clear message unless `dimmi serve --generate-key`.
12. **River UI** `/admin/jobs`: not included (deferred, framework follow-up). When added: instance admins only (`users.is_instance_admin`), not workspace owners (jobs span workspaces).
13. **Workspace export lives here** (data ownership pledge, roadmap 11 "Full JSON export"), available on cloud too.
14. **Settings registry moved to foundation** (`app/support/settings`, extensibility.md section 7). `Definition{Key, Type, Default, Scopes, Enforceable, Secret, Validate}`; `settings.Get[T](ctx, key, ScopeRef)` resolves user -> workspace -> organization -> instance -> default. 11 keeps instance storage (`system_settings`), precedence with config/env (operations.md section 1), admin API, `settings:get|set` CLI, and registers instance keys (`app/support/settings/instance_keys.go`). Instance value of a key with wider `Scopes` = instance default for lower scopes.
15. **Entitlements `self_host` driver** (`app/support/entitlements/self_host.go`, 11 owns; contract in foundation `contracts.md`, cloud driver in 12). `Has`: always true (one plan, every feature, never paywalled; no license). `Limit`: catalog entry's self-host config key when set (e.g. 10 `webhooks.max_endpoints`), else optional operator map `entitlements.limits.<key>` (dimmi.yaml / `DIMMI_ENTITLEMENTS__LIMITS__<KEY>`), else catalog self-host default (`-1` unlimited for every cloud cap: 05 custom Discord bots, 06 email daily cap, 08 help centers, 04 email caps). No `entitlement_overrides` table read on self-host. `GET /workspace/entitlements` works on self-host (shows unlimited).
16. **Region and version.** Self-host `app.region = self` (foundation key; cloud `us`/`eu` cells). Exposed with `server_version` (build version) and `api_version` in `/auth/config` and `/me.instance`, and `/.well-known/dimmi.json` (09). Lets newer mobile app / CLI hide features an older self-host lacks.
17. **Retention.** 11 jobs destroy only derived copies (backup archives, export zips; node registry lives in Valkey with TTL, no prune job); `RetentionGuard` (foundation) protects source data, which stays in DB and so in next backups. `PruneBackups`, `PruneDataExports` not guarded. Exception: `dimmi restore` overwrites whole DB, so it calls `RetentionGuard.MayDestroy(ws, {workspace, ws})` for every live workspace first; any false: refuse, list held workspaces, exit 1 (noop driver at MVP never refuses).
18. **Valkey required (PO decision 2026-11-02).** Every deployment = app + Postgres + Valkey (8.x; Redis 7.2+ protocol-compatible accepted, not tested). Config `redis.url` (`DIMMI_REDIS__URL`, PaaS `REDIS_URL`). Valkey holds only rebuildable state: cache (`cache` `redis` store), rate limits and lockout counters (`ratelimit` `redis`), cross-replica pubsub (`pubsub` `redis`: auth/token cache bust, maintenance flag, settings reload), centrifuge Redis broker + presence + history (realtime), replay/dedupe caches (`SET NX EX`), counters, node heartbeat registry. Postgres keeps River, sessions, tokens, audit, idempotency, billing, advisory locks, search, settings. Consequences: no Postgres LISTEN/NOTIFY broker for realtime or pubsub (River's job notifier still LISTENs; transaction poolers handled by `database.direct_url`, decision 20), no `realtime_*`/`cache`/`rate_limits` tables. Valkey loss = brief cache misses, realtime reconnect with refetch, counter reset; never data loss. Not backed up; restore docs say "start empty Valkey". AOF `everysec` only to keep presence/history across a Valkey restart. Boot waits up to `redis.connect_retry` (60 s) then fails; `/ready` 503 while Valkey unreachable.
19. **Valkey memory defaults: `maxmemory 64mb`, `maxmemory-policy noeviction`** (compose, install script, systemd conf). Why `noeviction` over `volatile-lru`: rate-limit and lockout counters, replay/dedupe keys, slot leases and presence carry TTLs, so `volatile-lru` would silently evict them under pressure (lockout bypass, duplicate webhook/LLM work, wrong concurrency). `noeviction` turns pressure into visible write errors: cache writes skipped (miss), limiter writes follow foundation Valkey-down policy (data-model.md: auth/abuse guards fail closed 503 + `Retry-After`, outbound provider limiters defer, rest fail open with warning), doctor `valkey.connection` warns over 80% memory and on any other policy. 64 MB holds typical small-team load (cache entries bounded by TTL, centrifuge history 100 to 300 msgs x 10 min per channel) with margin; raise via `--maxmemory` for big workspaces. Operator-supplied Valkey/Redis with `allkeys-lru` works degraded (edge 111).
20. **Backing-service topologies: connect, never operate (PO decision 2026-11-02).** Dimmi scales only its own process. Postgres and Valkey replication, failover, sharding and pooling belong to the operator or managed provider. Dimmi guarantees correct connection to: Postgres single instance, HA writer endpoint, libpq multi-host, session or transaction pooler (`database.pool_mode`, `database.direct_url`), optional read replicas (`database.read_urls`, `db.Reader`); Valkey standalone, managed HA endpoint, Sentinel, cluster (`redis.mode`). Contract and failover behavior: operations.md section 11. Proven by nightly topology suites (00-conventions/testing.md section 5). Not supported: multi-primary or sharded Postgres, Valkey replica reads, client-side Valkey sharding.

## User stories

- Solo dev: "I run `docker compose up -d` on $5 VPS, open URL, paste setup link from logs, create account, see widget snippet. Under 5 minutes."
- Solo dev on Railway/Render/Fly: "I click deploy button, wait, open URL, finish setup. No terminal."
- Operator: "I pull new image, restart. Migrations run once even with 3 replicas. Old replicas keep serving during rollout."
- Operator: "Nightly backup lands in my S3 bucket; I restore it on fresh box with one command and app key."
- Operator: "`dimmi doctor` tells me SPF missing, clock skew 4 s, S3 unreachable, each with fix."
- Instance admin: "I set mail provider in deploy config (Postmark, Resend, Mailgun, SendGrid, SES or SMTP relay), click test per stream in admin API, get email in 10 s or exact provider error."
- Workspace owner (self-host or cloud): "I export whole workspace as JSONL + files any time."
- Security reviewer: "I verify image signature and SBOM before deploy; container runs nonroot, read-only."
- AI agent (MCP, instance admin token): "`dimmi_get_system_health` tells me why widget messages stall (queue backlog, failing mail)."

## Data model

All tables below are instance-level (no `workspace_id`, not RLS'd), except `data_exports` (nullable `workspace_id`).

### system_settings
| Column | Type | Notes |
|---|---|---|
| key | text primary key | dotted config key; allowlist = settings registry definitions with scope `instance` (`app.url`, `backups.schedule`, `updates.check`, ...; mail provider keys are deploy config only, never rows here, per 04 final model), registered by 11 (`instance_keys.go`) and other specs |
| value | jsonb | non-secret value; null when secret |
| value_encrypted | bytea | secret value (`Encrypted[T]`, instance key ring) |
| updated_by_user_id | uuid null references users on delete set null | |
| created_at, updated_at | timestamptz | |
CHECK `(value IS NULL) <> (value_encrypted IS NULL)` except reserved internal keys (`setup.token_hash`, `maintenance.state`, `updates.latest`, `app.url_last_seen`).

### backups
| Column | Type | Notes |
|---|---|---|
| id | uuid | public `bak_` |
| kind | text CHECK in ('manual','scheduled','pre_migrate') | |
| status | text CHECK in ('pending','running','succeeded','failed','pruned') | |
| disk | text not null | filesystem disk name |
| path | text | object key / file path of archive |
| size_bytes | bigint | |
| sha256 | text | of archive |
| includes_files | boolean not null default false | local disk files copied |
| encrypted | boolean not null default false | |
| app_version | text not null | |
| schema_version | bigint not null | goose version at dump |
| pg_server_version | text not null | |
| pg_dump_version | text | |
| key_fingerprint | text not null | SHA-256 of app key id, never key material |
| error | text | human readable, redacted |
| triggered_by_user_id | uuid null | |
| started_at, finished_at | timestamptz | |
| created_at, updated_at | timestamptz | |
Indexes: `(status, created_at desc)`, `(kind, created_at desc)`. Partial unique `(status) WHERE status = 'running'` (one backup at a time).

### data_exports
| Column | Type | Notes |
|---|---|---|
| id | uuid | public `iex_` (not `exp_`: 03 contact exports own it) |
| scope | text CHECK in ('instance','workspace') | |
| workspace_id | uuid null references workspaces on delete cascade | required when scope workspace |
| status | text CHECK in ('pending','running','succeeded','failed','expired') | |
| disk, path | text | |
| size_bytes | bigint | |
| sha256 | text | |
| progress | jsonb not null default '{}' | `{"stage":"messages","done":12000,"total":48000}` |
| requested_by_user_id | uuid null | |
| expires_at | timestamptz | succeeded + 7 days |
| error | text | |
| started_at, finished_at, created_at, updated_at | timestamptz | |
Index `(workspace_id, created_at desc)`, `(scope, created_at desc)`. Partial unique `(coalesce(workspace_id, '00000000-0000-0000-0000-000000000000'::uuid)) WHERE status IN ('pending','running')` (one active export per scope).

### nodes (Valkey, not a table)
Rebuildable registry, moved from UNLOGGED table to Valkey (decision 18). Hash `dimmi:nodes:<id>`, `EX 60`, refreshed every 15 s by heartbeat goroutine, plus index sorted set `dimmi:nodes` (member = id, score = heartbeat unix ms). Listing = `ZRANGEBYSCORE` over last 60 s, then `HGETALL` per id; stale members removed with `ZREMRANGEBYSCORE` on read. No `SCAN` (cluster-safe, data-model.md key rules). Fields: `id` (process instance id, new each boot), `hostname`, `version`, `roles` (web, worker, scheduler), `queues` (River queues worked), `pg_dump_version` (empty = cannot run backups), `filesystem_default`, `platform`, `started_at`, `heartbeat_at`. Expiry replaces `PruneNodes`.

### dimmi_releases
| Column | Type | Notes |
|---|---|---|
| version | text primary key | semver of release that ran migrations |
| schema_version | bigint not null | goose head after migrate |
| min_app_version | text not null | oldest binary allowed to run on this schema (version skew guard) |
| migrated_at | timestamptz not null default now() | |

### Core table changes

| Table | Change | Migration |
|---|---|---|
| users | add `is_instance_admin boolean not null default false`; partial index `(id) WHERE is_instance_admin` | `20261101001101_add_is_instance_admin_to_users_table.sql` |

Other migrations (owned here):
- `20261101001102_create_system_settings_table.sql`
- `20261101001103_create_backups_table.sql`
- `20261101001104_create_data_exports_table.sql`
- `20261101001105_create_nodes_table.sql`
- `20261101001106_create_dimmi_releases_table.sql`

## Code map

```
cmd/dimmi/main.go                                   # registers commands below (foundation owns root)
app/console/commands/
  healthcheck_command.go                            # dimmi healthcheck (GET /up on local addr; image HEALTHCHECK)
  doctor_command.go                                 # dimmi doctor [--json] [--only=id,...]
  migrate_command.go                                # extends framework migrate/migrate:plan: skew guard, --no-backup, plan estimates + --json
  backup_command.go                                 # dimmi backup [--include-files] [--disk] [--output]
  backup_list_command.go                            # dimmi backup:list
  restore_command.go                                # dimmi restore <bak_id|path> --force
  down_command.go, up_command.go                    # maintenance mode
  export_command.go                                 # dimmi export [--workspace ws_] [--output]
  admin_promote_command.go                          # dimmi admin:promote <email> [--revoke]
  setup_link_command.go                             # dimmi setup:link (new setup token)
  settings_get_command.go, settings_set_command.go  # dimmi settings:get|set <key> [value]
  update_check_command.go                           # dimmi update:check
  storage_migrate_command.go                        # dimmi storage:migrate (uses app/actions/system/migrate_storage_disk.go)
  version_command.go                                # dimmi version [--json]
app/actions/system/
  complete_setup.go                                 # validates setup token, calls accounts.CreateFirstUser, sets instance admin
  issue_setup_token.go
  update_system_settings.go                         # emits system.app_url_changed when app.url changes
  send_test_mail.go                                 # per named mailer (`system`, `customer`); provider error text redacted
  test_storage_disk.go
  create_backup.go                                  # inserts row + RunBackup job
  run_backup.go                                     # pg_dump + files + manifest + upload (used by job and CLI)
  restore_backup.go                                 # CLI only; RetentionGuard check per live workspace before pg_restore (decision 17)
  prune_backups.go
  enable_maintenance_mode.go, disable_maintenance_mode.go
  check_for_updates.go
  request_data_export.go
  build_data_export.go
  set_instance_admin.go
app/support/doctor/                                 # product checks on `framework/health` registry (generic checks database, migrations, queue, storage, mail transport, clock, memory, disk live in framework): postgres.go, valkey.go, mail.go (both streams), dns.go, backups.go, proxy.go, security.go, feedback.go (14)
app/support/settings/instance_keys.go               # registers 11 instance-scope keys in foundation registry (`app/support/settings`, owned by foundation); instance store adapter over `system_settings` + config precedence
app/support/entitlements/self_host.go               # Entitlements self_host driver (decision 15); registered when app.edition = self_host
app/support/backupfmt/                              # archive layout + manifest only (stdlib archive/tar); encryption = filippo.io/age STREAM (no own AES code)
# migrations: framework/database/migrate (goose v3 Provider + session locker, plan, lint); skew guard + `dimmi_releases` stay 11
# GOMEMLIMIT: framework/foundation runtime (KimMachineGun/automemlimit), key runtime.memory_limit
# trusted proxies: framework/http/middleware TrustProxies (realclientip/realclientip-go, rightmost-trusted-range)
# sd_notify: later (not in framework yet; coreos/go-systemd/v22/daemon candidate, systemd platform only)
app/support/nodes/heartbeat.go                      # node registry writer (Valkey hash with TTL)
app/http/middleware/maintenance.go                  # 503 + Retry-After, bypass cookie
app/http/middleware/instance_admin.go               # requires users.is_instance_admin (+ 2FA on cloud)
app/http/middleware/setup_guard.go                  # redirects web to /setup when no users
app/http/controllers/api/v1/
  setup_controller.go                               # Show, Store
  system_info_controller.go                         # Show
  system_health_controller.go                       # Show
  system_node_controller.go                         # Index
  system_setting_controller.go                      # Show, Update
  system_mail_test_controller.go                    # Store
  system_storage_test_controller.go                 # Store
  system_backup_controller.go                       # Index, Store, Show, Destroy
  system_backup_download_link_controller.go         # Store
  system_maintenance_controller.go                  # Show, Store, Destroy
  system_update_controller.go                       # Show
  system_export_controller.go                       # Index, Store, Show
  system_export_download_link_controller.go         # Store
  system_user_controller.go                         # Index, Update
  system_workspace_controller.go                    # Index
  export_controller.go                              # workspace scope: Index, Store, Show
  export_download_link_controller.go                # Store
app/http/controllers/web/setup_page_controller.go   # SSR shell for /setup (UI later)
app/http/requests/
  complete_setup_request.go, update_system_settings_request.go, send_test_mail_request.go,
  store_backup_request.go, store_maintenance_request.go, store_export_request.go, update_system_user_request.go
app/http/resources/
  system_info_resource.go, health_check_resource.go, node_resource.go, system_settings_resource.go,
  backup_resource.go, maintenance_resource.go, update_resource.go, data_export_resource.go, system_user_resource.go
app/policies/system_policy.go                       # `authz.Can(p, "system:read|write")`; `roles.go` maps `users.is_instance_admin` to `system:*` (instance role), no flag checks in policy
app/policies/data_export_policy.go                  # `authz.Can(p, "exports:read|write")`; `roles.go` grants exports permissions to owner only (no role names in policy)
app/jobs/
  run_backup.go                                     # RunBackupArgs, queue backups
  schedule_backups.go                               # periodic
  prune_backups.go
  check_for_updates.go
  build_data_export.go
  prune_data_exports.go
app/events/
  backup_succeeded.go, backup_failed.go, update_available.go, maintenance_mode_changed.go, data_export_completed.go
app/listeners/
  notify_instance_admins_of_backup_failure.go       # mail
  notify_instance_admins_of_update.go               # mail, once per version
  broadcast_data_export_completed.go                # `member:{mem_}` channel (centrifuge)
app/mail/
  backup_failed_mail.go, update_available_mail.go, test_mail.go, data_export_ready_mail.go
app/mcp/tools/
  get_system_health.go                              # dimmi_get_system_health
  list_backups.go                                   # dimmi_list_backups
  create_backup.go                                  # dimmi_create_backup
config/
  app.go (edition, url, key, key_file, key_auto_generate, data_dir, roles, platform)
  http.go (addr, admin_addr, trusted_proxies, timeouts)       # new file
  database.go (url, max_conns; migrations.auto + migrations.lock_timeout are framework keys)
  redis.go (url, connect_retry, pool_size, key_prefix)        # Valkey connection shared by cache, ratelimit, pubsub, broadcasting
  backups.go, updates.go, maintenance.go                      # new files
routes/api.go                                       # system group (auth + instance_admin), setup group (no auth, throttle:setup)
routes/web.go                                       # /setup
routes/console.go                                   # ScheduleBackups (cron from settings), PruneBackups, CheckForUpdates, PruneDataExports
database/migrations/2026110100110{1..6}_*.sql
database/queries/system_settings.sql, backups.sql, data_exports.sql, dimmi_releases.sql
database/factories/backup_factory.go, data_export_factory.go
deploy/
  docker/Dockerfile                                 # the one image: Debian slim + postgresql-client-18
  docker/postgres/postgresql.small.conf
  compose/docker-compose.yml, compose/Caddyfile, compose/.env.example
  railway/railway.json + template notes
  render/render.yaml
  fly/fly.toml
  digitalocean/.do/deploy.template.yaml (copied to repo root .do/)
  coolify/dimmi.yaml
  dokploy/blueprints/dimmi/{docker-compose.yml,template.toml,logo.svg}
  systemd/dimmi.service
  install/install.sh                                # bash; default installs Postgres + Valkey (apt)
.goreleaser.yaml
tests/feature/system/ (setup, settings, mail test per stream, storage test, backups incl. age round trip, maintenance, exports, users, info, health, doctor checks, nodes registry; real Postgres + Valkey + S3 from root `compose.yaml` via testkit)
# no tests/unit for app logic (testing.md); migrator, memlimit, trustedproxy primitives tested in framework packages
tests/e2e/selfhost/ (compose boot, install.sh boot in systemd container, upgrade N-1 to N, backup-restore round trip, idle budget) # run in CI with Docker, not in `go test ./...`
```

## HTTP API

All `/api/v1/system/*` routes: middleware `auth` (`A`: session, first-party app or token), `instance_admin`, `throttle:api`. Token needs ability `system:read` (GET) or `system:write` (others); only instance admins can mint tokens with `system:*` abilities. `system` routes ignore `Dimmi-Workspace` header (instance scope).

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | /api/v1/setup | setup.show | none, `throttle:setup` (30/min/IP) | `{needs_setup, mail_configured, app_url, version}`; never leaks more |
| POST | /api/v1/setup | setup.store | setup token in body, `throttle:setup` | create first user + workspace + instance admin, returns session |
| GET | /api/v1/system/info | system.info.show | instance admin, `system:read` | version, edition, platform, region, api_version, schema, pg version, nodes count, update available |
| GET | /api/v1/system/health | system.health.show | instance admin, `system:read` | doctor checks (cached 30 s; `?refresh=1` reruns) |
| GET | /api/v1/system/nodes | system.nodes.index | instance admin, `system:read` | live processes from Valkey node registry |
| GET | /api/v1/system/settings | system.settings.show | instance admin, `system:read` | effective settings with `source` per key; secrets masked |
| PATCH | /api/v1/system/settings | system.settings.update | instance admin, `system:write` | update runtime-editable keys |
| POST | /api/v1/system/settings/mail-tests | system.settings.mail-tests.store | instance admin, `system:write` | send test email through named mailer (`system` or `customer`), sync result (timeout 15 s) |
| POST | /api/v1/system/settings/storage-tests | system.settings.storage-tests.store | instance admin, `system:write` | write/read/sign/delete probe on disk |
| GET | /api/v1/system/backups | system.backups.index | instance admin, `system:read` | list, cursor |
| POST | /api/v1/system/backups | system.backups.store | instance admin, `system:write`, `idempotent` | start manual backup (202) |
| GET | /api/v1/system/backups/{backup} | system.backups.show | instance admin, `system:read` | status |
| POST | /api/v1/system/backups/{backup}/download-links | system.backups.download-links.store | instance admin, `system:write` | signed URL, 15 min, audit logged |
| DELETE | /api/v1/system/backups/{backup} | system.backups.destroy | instance admin, `system:write` | delete archive, row `pruned` |
| GET | /api/v1/system/maintenance | system.maintenance.show | instance admin, `system:read` | state |
| POST | /api/v1/system/maintenance | system.maintenance.store | instance admin, `system:write` | enable (soft only via API; hard only CLI) |
| DELETE | /api/v1/system/maintenance | system.maintenance.destroy | instance admin, `system:write` | disable |
| GET | /api/v1/system/updates | system.updates.show | instance admin, `system:read` | latest release, notes URL, `update_available` |
| GET | /api/v1/system/exports | system.exports.index | instance admin, `system:read` | instance exports |
| POST | /api/v1/system/exports | system.exports.store | instance admin, `system:write`, `idempotent` | start instance export (202) |
| GET | /api/v1/system/exports/{export} | system.exports.show | instance admin, `system:read` | progress |
| POST | /api/v1/system/exports/{export}/download-links | system.exports.download-links.store | instance admin, `system:write` | signed URL 15 min |
| GET | /api/v1/system/users | system.users.index | instance admin, `system:read` | users with workspace counts, `filter[instance_admin]` |
| PATCH | /api/v1/system/users/{user} | system.users.update | instance admin, `system:write` | `is_instance_admin` toggle |
| GET | /api/v1/system/workspaces | system.workspaces.index | instance admin, `system:read` | all workspaces (name, slug, member count, created) |
| GET | /api/v1/exports | exports.index | permission `exports:read` (owner via `roles.go`) | workspace exports |
| POST | /api/v1/exports | exports.store | permission `exports:write` (owner), `idempotent` | start workspace export (202) |
| GET | /api/v1/exports/{export} | exports.show | permission `exports:read` (owner) | progress |
| POST | /api/v1/exports/{export}/download-links | exports.download-links.store | permission `exports:write` (owner) | signed URL 15 min |

Admin port routes (`http.admin_addr`, default `127.0.0.1:9090`, never on public listener): metrics, pprof, and 08-owned `GET /internal/tls/ask?domain=` (`tls.ask`) for Caddy on-demand TLS: 200 only for ownership-verified `help_center_domains`, else 404; no auth (network-isolated), throttled.

Health routes (foundation owns `/up`, `/ready`): 11 adds `/ready` body `{"status":"ok","maintenance":false,"migrations":"current","version":"1.4.2"}`; `/ready` stays 200 during soft and hard maintenance (load balancer keeps routing so users see maintenance response, not LB error); returns 503 only when Postgres unreachable or migrations pending beyond lock wait.

### Examples

`POST /api/v1/setup`
```json
{
  "token": "dmi_setup_3k9q...",
  "name": "Ada Lovelace",
  "email": "ada@example.com",
  "password": "correct horse battery staple",
  "workspace_name": "Acme",
  "app_url": "https://support.acme.dev"
}
```
201:
```json
{
  "data": {
    "object": "setup",
    "user": { "id": "usr_01jc...", "object": "user", "email": "ada@example.com", "is_instance_admin": true },
    "workspace": { "id": "ws_01jc...", "object": "workspace", "name": "Acme", "slug": "acme" },
    "next": { "widget_snippet_url": "https://support.acme.dev/api/v1/widget-settings", "dashboard_url": "https://support.acme.dev/" }
  }
}
```
Sets session cookie. Errors: 410 `setup_completed` (users exist), 403 `invalid_setup_token`, 422 validation, 429.

`GET /api/v1/system/health`
```json
{
  "data": {
    "object": "health_report",
    "status": "warn",
    "generated_at": "2026-11-01T10:00:00Z",
    "checks": [
      { "id": "postgres.version", "status": "pass", "summary": "PostgreSQL 18.1", "fix": null },
      { "id": "mail.spf", "status": "warn", "summary": "SPF for acme.dev does not include smtp.fastmail.com", "fix": "Add include:spf.messagingengine.com to TXT acme.dev", "docs_url": "https://trydimmi.com/docs/self-host/doctor#mail-spf" },
      { "id": "backups.worker", "status": "fail", "summary": "Backups enabled but no running process has pg_dump 18", "fix": "Install postgresql-client-18 on this host (install.sh does) or run image ghcr.io/dimmihq/dimmi:1.4.2; else set DIMMI_BACKUPS__ENABLED=false", "docs_url": "..." }
    ]
  }
}
```

`PATCH /api/v1/system/settings`
```json
{
  "backups.schedule": "0 2 * * *",
  "updates.check": false,
  "app.url": "https://support.acme.dev"
}
```
200 returns settings resource:
```json
{
  "data": {
    "object": "system_settings",
    "settings": {
      "app.url": { "value": "https://support.acme.dev", "source": "database", "editable": true },
      "mail.mailers.system.driver": { "value": "postmark", "source": "env", "editable": false },
      "mail.mailers.system.token": { "value": null, "is_set": true, "secret": true, "source": "env", "editable": false },
      "filesystems.default": { "value": "s3", "source": "env", "editable": false }
    }
  }
}
```
Errors: 422 `setting_locked_by_env` per key (in `errors`), 422 unknown key `unknown_setting` (incl. any `mail.*` / `channels.email.*` provider key: deploy config only), 403 `managed_by_cloud` (cloud edition).

`POST /api/v1/system/settings/mail-tests` `{ "to": "ada@example.com", "mailer": "customer" }` (`mailer` default `system`) 200:
```json
{ "data": { "object": "mail_test", "status": "sent", "mailer": "customer", "driver": "postmark", "duration_ms": 412, "message_id": "<...@acme.dev>", "provider_message_id": "b7bc2f4a-...", "error": null } }
```
Failure still 200 with `"status":"failed","error":{"stage":"auth","message":"401 Invalid API token"}` (test result, not API error; stages `config`, `connect`, `tls`, `auth`, `send`).

`POST /api/v1/system/backups` `{ "include_files": true }` 202:
```json
{ "data": { "id": "bak_01jc...", "object": "backup", "kind": "manual", "status": "pending", "disk": "local", "includes_files": true, "encrypted": false, "size_bytes": null, "created_at": "2026-11-01T10:00:00Z" } }
```
Errors: 409 `backup_in_progress`, 409 `no_backup_worker` (no node with compatible pg_dump), 503 `maintenance_hard`.

Error codes (feature): `setup_completed`, `invalid_setup_token`, `setting_locked_by_env`, `unknown_setting`, `managed_by_cloud`, `backup_in_progress`, `no_backup_worker`, `backup_not_ready`, `export_in_progress`, `export_expired`, `last_instance_admin`, `maintenance` (503), `maintenance_hard` (503), `not_instance_admin` (403), `feature_unavailable` (404 in other edition).

## MCP tools and CLI

MCP (instance admin token or OAuth with `system:*`, toolset `system`): `dimmi_get_system_health` (read), `dimmi_list_backups` (read), `dimmi_create_backup` (write, idempotency key required, 09 `app/mcp/confirm` gate: backups are data egress).

CLI (server-side, runs against DB directly; Laravel artisan style). Full flags in operations.md.

| Command | Purpose |
|---|---|
Framework-provided (release binary): `serve [--roles] [--no-migrate]`, `migrate [--force]`, `migrate:status`, `migrate:plan`, `migrate:rollback [--force --confirm-version]`, `down [--message] [--force]`, `up`, `config:show`, `route:list`. Production `migrate`, `down`, `migrate:rollback` warn (database host/name, migrations) and need confirmation or `--force`; non-interactive without `--force` refuses; production rollback also needs exact version. Dev-only (`dev` build tag, never in release binary): `make:*`, `route:lint`, `lang:check`. Rows below = 11 app commands or app extensions of framework commands.

| `dimmi serve [--generate-key]` | run server; `--generate-key` (app flag on top of framework `serve`) creates `<data_dir>/app.key` (0600) when no key configured (bare binary) |
| `dimmi healthcheck [--url]` | exit 0 when `/up` 200 (Docker HEALTHCHECK) |
| `dimmi doctor [--json] [--only ids] [--skip-network]` | run checks; exit 1 on any fail, 0 on pass/warn |
| `dimmi migrate [--force] [--no-backup]`, `dimmi migrate:plan [--json]` | apply migrations under lock (framework); `migrate:plan` (was `--dry-run`) prints plan + lint, 11 adds estimates, live node versions, `--json`, `--no-backup`. With positional source (`dimmi migrate intercom`) same command runs 16 import wizard instead (09 cli.md; never DB migrations) |
| `dimmi backup [--include-files] [--disk name] [--output path] [--encrypt]` | backup now (foreground) |
| `dimmi backup:list [--json]` | list backups |
| `dimmi restore <bak_id or path> --force [--skip-files] [--key-file]` | restore (hard maintenance) |
| `dimmi down [--message] [--force]` / `dimmi up` | maintenance mode (framework). Later (framework follow-ups): `--retry`, `--secret`, `--hard` |
| `dimmi export [--workspace ws_...] [--output path]` | instance or workspace export |
| `dimmi user:create --email --name [--password-stdin] [--instance-admin] [--workspace name]` | owned by 00-accounts; 11 adds `--instance-admin` flag behavior |
| `dimmi admin:promote <email> [--revoke]` | instance admin flag |
| `dimmi setup:link` | print fresh setup URL (invalidates old) |
| `dimmi settings:get <key>` / `settings:set <key> <value>` | system settings |
| `dimmi update:check` | check now, print result |
| `dimmi storage:migrate --from local --to s3 [--dry-run]` | copy objects between disks, update `attachments.disk` in batches, resumable |
| `dimmi version [--json]` | version, commit, build date, go version |

## Events

Emitted (new, add to catalog):

| Event | Emitted by | Realtime | Webhook | Notes |
|---|---|---|---|---|
| system.backup_succeeded | RunBackup | no | no | instance-level; metrics + audit |
| system.backup_failed | RunBackup | no | no | mails instance admins (max 1 per 6 h) |
| system.update_available | CheckForUpdates | no | no | mails instance admins once per version |
| system.maintenance_changed | Enable/DisableMaintenanceMode | no | no | invalidates maintenance cache on every replica via Valkey pubsub topic `system` |
| system.app_url_changed | UpdateSystemSettings (`app.url` write), boot when env `DIMMI_APP__URL` differs from last stored `app.url_last_seen` | no | no | `{from, to}`; instance-level, queued listeners: 08 `RebuildHelpCenterUrls` (canonical, path mode, sitemap), 01 widget loader/snippet URL cache bust, 05 `RebuildWebhookRegistrations` |
| export.completed | BuildDataExport | yes (`member:{mem_}` of requester) | yes (workspace scope only) | `{export_id, scope, status}` |

Webhook note: workspace webhooks never see instance events.

Consumed: none from catalog. Audit: `WriteAuditLog` covers settings changes, backups (create, download link, delete), restore (CLI writes audit row after restore), maintenance toggles, instance admin changes, exports (create, download).

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| RunBackup | backups | CreateBackup action, ScheduleBackups, pre-migrate (inline, not job) | 2 (backoff 5 min) | by backup id; global single running via partial unique index |
| ScheduleBackups | maintenance | periodic, cron from `backups.schedule` (default `0 3 * * *` UTC) | 0 | unique per scheduled minute |
| PruneBackups | maintenance | after each successful backup + daily 04:00 UTC | 3 | unique per day |
| CheckForUpdates | maintenance | every 24 h + random 0-6 h jitter; on boot after 10 min | 3 | unique per day |
| BuildDataExport | maintenance | RequestDataExport | 3, resumable from `progress` | by export id |
| PruneDataExports | maintenance | daily | 3 | per day |

Node heartbeat is goroutine, not job (must run even when queues stuck); writes Valkey hash with TTL (no prune job).

New queue `backups` (concurrency 1). Add to conventions queue list.

## Config keys

Precedence: defaults < `dimmi.yaml` < env `DIMMI_*` (+ `_FILE`; `__` between levels, framework `config`) < CLI flags. `system_settings` rows apply only to runtime-editable keys and only when env/yaml does not set key (env always wins, shown `source: env`, `editable: false`). Full table in operations.md section 1. Key highlights:

| Key | Env | Default | Notes |
|---|---|---|---|
| app.edition | DIMMI_APP__EDITION | self_host | `cloud` enables 12 |
| app.url | DIMMI_APP__URL | `http://localhost:8080` | runtime-editable; base for links, cookies Secure flag |
| app.key | DIMMI_APP__KEY / DIMMI_APP__KEY_FILE | none | empty = `<data_dir>/app.key` |
| app.key_auto_generate | DIMMI_APP__KEY_AUTO_GENERATE | true in container images (Dockerfile ENV), false for bare binary | writes `<data_dir>/app.key` 0600 when no key; `dimmi serve --generate-key` forces true for that run |
| app.data_dir | DIMMI_APP__DATA_DIR | `/var/lib/dimmi` (images), `./storage` (dev) | |
| app.platform | DIMMI_PLATFORM | detected or `unknown` | docker, compose, railway, render, fly, digitalocean, coolify, dokploy, systemd, kubernetes |
| app.roles | DIMMI_APP__ROLES | `web,worker,scheduler` | only `scheduler` nodes join River leader election |
| app.locale / app.locales | DIMMI_APP__LOCALE / DIMMI_APP__LOCALES | `en` / `[en]` | framework i18n |
| http.addr | DIMMI_HTTP__ADDR or `PORT` | `:8080` | `PORT` alias: framework follow-up (not built) |
| http.admin_addr | DIMMI_HTTP__ADMIN_ADDR | `127.0.0.1:9090` | metrics, pprof |
| http.trusted_proxies | DIMMI_HTTP__TRUSTED_PROXIES | empty | CIDRs or preset `private`, `cloudflare` |
| database.url | DIMMI_DATABASE__URL or `DATABASE_URL` | none (required) | `DATABASE_URL` alias: framework follow-up |
| database.max_conns | DIMMI_DATABASE__MAX_CONNS | 10 | |
| migrations.auto | DIMMI_MIGRATIONS__AUTO | true | `serve --no-migrate` overrides |
| migrations.lock_timeout | DIMMI_MIGRATIONS__LOCK_TIMEOUT | 10m | |
| redis.url | DIMMI_REDIS__URL or `REDIS_URL` (alias: follow-up) | none (required) | `redis://` / `rediss://` (TLS), optional `?db=`; Valkey 8 |
| redis.connect_retry | | 60s | boot retry window (later) |
| redis.key_prefix | DIMMI_REDIS__KEY_PREFIX | `dimmi:` | lets several instances share one Valkey |
| mail.mailers.system.driver | DIMMI_MAIL__MAILERS__SYSTEM__DRIVER | `log` | `ses`, `smtp`, `postmark`, `resend`, `mailgun`, `sendgrid`, `log`; driver keys flat beside it (`DIMMI_MAIL__MAILERS__SYSTEM__TOKEN`, `...__API_KEY`, ...); `mail.default` = mailer name `system` (full mail keys: operations.md section 1) |
| upgrades.backup_before_migrate | DIMMI_UPGRADES__BACKUP_BEFORE_MIGRATE | auto | auto = when pg_dump present and DB under 1 GB |
| backups.enabled | DIMMI_BACKUPS__ENABLED | true when pg_dump on PATH at boot, else false | |
| backups.schedule | DIMMI_BACKUPS__SCHEDULE | `0 3 * * *` | runtime-editable |
| backups.disk | DIMMI_BACKUPS__DISK | `local` (`<data_dir>/backups`) | any filesystem disk |
| backups.retention.daily/weekly/monthly | | 7 / 4 / 6 | runtime-editable |
| backups.encryption | DIMMI_BACKUPS__ENCRYPTION | `auto` | auto = `app_key` when disk is s3, `none` when local |
| backups.include_files | | true when `filesystems.default = local` | |
| updates.check | DIMMI_UPDATES__CHECK | true | runtime-editable |
| updates.repository | | `dimmihq/dimmi` | |
| auth.signup.mode | DIMMI_AUTH__SIGNUP__MODE | invite_only | key owned by 00-accounts; `open`, `invite_only`; runtime-editable here |
| app.region | DIMMI_APP__REGION | `self` | foundation key; self-host default `self`; exposed in `/auth/config`, `/me.instance`, discovery |
| entitlements.limits.<key> | DIMMI_ENTITLEMENTS__LIMITS__<KEY> | unset | optional operator cap per catalog limit key (self_host driver); unset = spec config key or `-1`; unknown key fails boot (catalog registry) |
| broadcasting.ping_interval | | 25s | centrifuge server ping; keeps sockets under 60 s LB idle defaults |
| runtime.memory_limit | DIMMI_RUNTIME__MEMORY_LIMIT / GOMEMLIMIT | auto (90% of cgroup memory.max) | automemlimit |

## Permissions matrix

Policies call `authz.Can` (extensibility.md P2). Columns derived from 00-accounts `roles.go`: workspace roles map to `exports:*` (owner only); instance admin flag maps to `system:*`. No `role ==` or `is_instance_admin` checks in 11 policies (middleware `instance_admin` keeps 2FA-on-cloud rule, reads permission).

| Action | Instance admin | Workspace owner | Admin | Member | Token ability |
|---|---|---|---|---|---|
| Setup (first run) | n/a (setup token) | | | | none |
| View system info/health/nodes | yes | no | no | no | `system:read` |
| Edit system settings, mail/storage test | yes | no | no | no | `system:write` |
| Backups list/create/download/delete | yes | no | no | no | `system:read` / `system:write` |
| Restore | CLI only (shell access) | | | | n/a |
| Maintenance soft | yes | no | no | no | `system:write` |
| Maintenance hard | CLI only | | | | n/a |
| Instance export | yes | no | no | no | `system:write` |
| Workspace export | yes (any workspace via instance export) | yes | no | no | `exports:write`, `exports:read` |
| Promote/demote instance admin | yes (cannot demote last) | no | no | no | `system:write` |
New abilities: `system:read`, `system:write`, `exports:read`, `exports:write` (add to foundation ability list). `*` tokens of non-instance-admins never gain `system:*`.

## Performance budgets

| Metric | Budget |
|---|---|
| Idle stack compose (dimmi, postgres, valkey, caddy) | under 300 MB |
| Valkey idle (`maxmemory 64mb`, AOF everysec) | under 20 MB RSS |
| Idle dimmi process | under 50 MB RSS |
| Postgres idle (tuned conf) | under 120 MB cgroup memory (excludes inactive file cache) |
| Cold boot no migrations | under 1 s |
| Advisory lock acquire + migration check, no pending | under 50 ms |
| `GET /api/v1/system/health` cached | under 20 ms; fresh run under 5 s |
| Backup throughput | at least 10 MB/s dump+compress on 2 vCPU |
| Instance export | 1M messages under 10 min, memory flat (streaming, under 30 MB extra) |
| Maintenance flag propagation to all replicas | under 2 s (Valkey pubsub + 1 s poll of `system_settings` fallback) |
| Heartbeat write | 1 Valkey `HSET`+`EXPIRE` per 15 s per process |

## Observability

Metrics (admin port): `dimmi_build_info{version,commit,edition,platform}`, `dimmi_migrations_pending`, `dimmi_schema_version`, `dimmi_backup_last_success_timestamp_seconds`, `dimmi_backup_duration_seconds`, `dimmi_backup_size_bytes`, `dimmi_backup_failures_total`, `dimmi_maintenance_mode{mode}`, `dimmi_update_available`, `dimmi_nodes`, `dimmi_doctor_check_status{id}` (0 pass, 1 warn, 2 fail), `dimmi_export_duration_seconds`, Go runtime + process collectors (RSS, GOMEMLIMIT).
Logs: boot banner (version, edition, platform, roles, data dir, config sources, never secrets), migration start/finish with durations, setup URL (once per boot while no users, level WARN so visible), backup lifecycle, update available (INFO once per version).

## Security and privacy notes

- Setup token: 32 random bytes, `dmi_setup_` prefix (secret, not public id; matches `dmi_` log redaction), stored as SHA-256 in `system_settings.setup.token_hash`, expires 24 h, single use, regenerated each boot while zero users. Prevents "first visitor owns instance" race on public IP.
- Backups contain all customer data. Archive mode 0600 on local disk. Download only via signed URL (15 min) + audit log. Encryption via filippo.io/age (STREAM, ChaCha20-Poly1305 chunks): `app_key` mode = X25519 identity derived from app key (HKDF info `dimmi-backup-age-v1`), `dimmi backup:identity` prints it as age identity file so operators decrypt with stock `age -d -i`; `age` mode = operator recipients `backups.age_recipients` (keys outside the instance). Losing app key loses `app_key`-mode backups and all encrypted columns anyway; docs: store key apart from backups.
- Restore refuses when backup `key_fingerprint` differs from current key unless `--key-file` provides matching key (encrypted columns unreadable otherwise).
- Hardening defaults in operations.md section 11: nonroot uid 65532, read-only rootfs, `no-new-privileges`, `cap_drop: [ALL]`, Postgres never published to host, admin port bound to 127.0.0.1, River UI (when added; deferred) instance admin only, `auth.signup.mode=invite_only`.
- Update check sends no identifiers. No telemetry.
- System settings secrets: `Encrypted[T]` with instance key ring (KeyManager scope `instance`); API returns `is_set` only.
- Export archives: same signed URL rules; workspace export never includes other workspaces. Exports never include secrets (BYOK API keys, channel credentials, webhook secrets, password hashes, 2FA secrets, token hashes). Exclusion list in operations.md section 8.

## Metering impact

None on self-host: `app.edition = self_host` registers no billing module, `metered` middleware no-op. On cloud, backups/restore here are not used (managed RDS PITR); exports and setup never create billable activity (no substantive replies). Cloud restore of a workspace never replays `usage_ledger` (ledger excluded from workspace export; instance export on cloud is disabled: 404 `feature_unavailable`).

## Dependencies on other specs

| Spec | Needs |
|---|---|
| 00-foundation / framework | settings registry (`app/support/settings` on `config.RuntimeStore`, scopes; 11 registers instance keys), `Entitlements` contract + catalog (11 owns `self_host` driver), `RetentionGuard` (restore), `app.region` / `api.version` keys, `framework/config`, `encryption.KeyManager` with `instance` scope, `key:generate`, `framework/database/migrate` (goose session locker), `framework/queue` (+ `backups` queue), `pubsub.PubSub` topic `system` (Valkey), `framework/broadcasting` (centrifuge Redis broker on Valkey), `filesystem` disks + signed URLs, `mail.Mailer` named mailers, `framework/health` (`/up` `/ready`, doctor engine), `framework/http/middleware` (`TrustProxies`, `Maintenance`, `idempotent`), `framework/foundation` runtime (automemlimit), `framework/testkit` (compose services + env vars), abilities list (+4 new) |
| 00-accounts | permission catalog + `roles.go` (instance admin -> `system:*`, owner -> `exports:*`), `/auth/config` + `/me.instance` rendering (`region`, `server_version` from 11), `CreateFirstUser` action (user + workspace + owner membership, session login), invitations for `invite_only`, audit log writer, users table (core change `is_instance_admin`), 2FA status for cloud staff gate |
| 01-messenger-widget | widget offline behavior on 503 `maintenance` (foundation edge case) |
| 03-shared-inbox | resources reused by export (conversation, message, contact, company, tag, saved reply, inbox) |
| 04-email-channel | mail final model: instance-level provider only, two streams (`mail.mailers.system`, `mail.mailers.customer` via `channels.email.mailer`), no SMTP receiver; DNS check helpers (SPF/DKIM/DMARC) and provider webhook secrets reused by doctor |
| 14-feedback-roadmap | doctor checks (operations.md section 5), seeders demo data |
| 05-chat-app-channels | channel health per connection for doctor (`channels.errored` count) |
| 07-dimmi-agent, 08-help-center, 10-webhooks | resources for export (agents config without keys, articles, webhook endpoints without secrets) |
| 07 (agent actions), 08 (docs sync), 16-importers | config keys + doctor checks documented in operations.md "Feature config notes" (`http_client.allow_hosts` for action connections, `ai.actions.*`, `docs_sync.*`, `github.*`, `imports.*`); doctor `github.app_url_public`, `github.repositories`, `ai.actions.private_hosts`, `imports.queue`; 09 shares `dimmi migrate` command name (positional source = import wizard) |
| 09-agent-native | MCP registration, OAuth scopes `system:*` |
| 12-cloud-billing | edition flag only; billing tables excluded from exports; cloud `Entitlements` driver (self-host never reads `entitlement_overrides`) |

## Open questions

1. GitHub org (`dimmihq` vs `dimmi-hq`): default `dimmihq`; one config constant.
