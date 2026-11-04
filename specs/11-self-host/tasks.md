# 11 Self-host: tasks

Style: caveman. Ordered. Each task about 1 day, merged with tests. Depends on foundation/framework skeleton (config, DB, River, Filesystem, `mail` named mailers, KeyManager, `framework/health`, `framework/testkit`). Final sweep 2026-11-02: Valkey required (decision 18), framework reuse (automemlimit, realclientip-go, goose session locker, age, go-systemd), mail final model, feedback doctor checks. App tests are integration tests on real Postgres + Valkey (services from root `compose.yaml`: `pgvector/pgvector:pg18`, `valkey/valkey:8`, SeaweedFS S3; found via `DATABASE_URL`, `REDIS_URL`, `AWS_*`); tests named in plain words, every edge case covered.

## M1 Boot, config, first run

- [ ] Config keys for app/http/database/redis/backups/updates/maintenance/registration/runtime (`config/*.go`), `PORT` + `DATABASE_URL` + `REDIS_URL` aliases and unknown-key WARN with suggestion (both: framework follow-ups in foundation), mail keys read-only in settings API. Env names `DIMMI_<SECTION>__<KEY>`; `_FILE` beats plain value (framework, no conflict error).
  - Accept: table in operations.md section 1 loads with sources. Tests: precedence (default < yaml < env < flag), `_FILE` wins over plain, `PORT` alias, typo suggestion.
- [ ] Platform detection (`app.platform`) + boot banner (version, edition, platform, roles, data dir, config sources).
  - Tests: env fixtures per platform produce expected value.
- [ ] App key resolution: env/file/`<data_dir>/app.key`/auto-generate with `O_EXCL` 0600 (default on in images via ENV, off for bare binary; `serve --generate-key`), fingerprint stored + verified against DB (`app.key_fingerprint`).
  - Tests: concurrent generate (two goroutines) yields one key; bare binary without key and without flag fails with message naming `--generate-key`; mismatch fails boot with both fingerprints; read-only data dir error message.
- [ ] `runtime.memory_limit` wired to `framework/foundation` runtime (KimMachineGun/automemlimit; cgroup v1/v2). No app code.
  - Tests: framework package owns cgroup fixtures; 11 asserts boot banner reports limit.
- [ ] Valkey connection (`redis.url`, retry window, key prefix) shared by cache, ratelimit, pubsub, broadcasting; boot fails after `redis.connect_retry`; `/ready` 503 while unreachable.
  - Tests (real Valkey; outage via in-test TCP proxy cut and restored): boot waits then fails with message naming upgrade note; Valkey restart mid-run: `/ready` flips and recovers, caches refill (E107-E109).
- [ ] Postgres version + extensions check at boot with host-specific hints.
  - Tests: compose `pgvector/pgvector:pg18` passes; simulated missing extension message (isolated database without extension).
- [ ] Migration `users.is_instance_admin` + `system_settings` table + sqlc queries + instance-scope key registration (`app/support/settings/instance_keys.go`) on foundation settings registry (`support/settings`, owned by foundation task) + `system_settings` instance store adapter.
  - Tests: registry rejects unknown key; workspace-scope key via system settings API 422 `unknown_setting`; secret keys stored encrypted (instance key ring); CHECK constraint; key registered by other package shows in `GET /system/settings` (edge cases 101, 102).
- [ ] Entitlements `self_host` driver (`app/support/entitlements/self_host.go`): `Has` true, `Limit` from `entitlements.limits.<key>` map, then catalog self-host config key, then catalog default (`-1`); unknown map key fails boot; driver bound when `app.edition = self_host`.
  - Tests: every catalog key resolves on self-host; operator map overrides; spec config key honored (`webhooks.max_endpoints`); `GET /workspace/entitlements` shows unlimited (edge cases 98 to 100).
- [ ] `app.region` (self-host default `self`) + `server_version` + `api_version` supplied to 00-accounts `/auth/config` and `/me.instance`, system info, discovery (09).
  - Tests: snapshot of `/auth/config` instance fields on self-host (edge case 103).
- [ ] `IssueSetupToken`, `setup_guard` middleware, `GET/POST /api/v1/setup` (`CompleteSetup` calls 00-accounts `CreateFirstUser`), WARN log line, 503 `setup_required` on other routes.
  - Tests: race (two parallel POST, one 201 one 410), bad token 403, expired token, throttle 429, audit row, instance admin flag set, session cookie set.
- [ ] `--instance-admin` flag on 00-accounts `dimmi user:create`, `dimmi admin:promote`, `dimmi setup:link`.
  - Tests: password from stdin, `--instance-admin`, demote last admin refused.
- [ ] `instance_admin` middleware + `SystemPolicy` + `DataExportPolicy` via `authz.Can` (`roles.go`: instance admin flag -> `system:*`, owner -> `exports:*`; no role names or flag checks in policies) + new abilities `system:read|write`, `exports:read|write` (token minting restricted to instance admins).
  - Tests: non-admin 403 `not_instance_admin`; workspace admin token with `*` lacks system access; cloud requires 2FA; demoted owner export link 403 (edge case 106).

## M2 Migrations and upgrades

- [ ] Migrations on `framework/database/migrate` (built: goose v3 Provider + session locker, wait logging, `lock_timeout` retry, `migrations.auto`/`migrations.lock_timeout`, production confirm on `migrate`/`migrate:rollback`): app adds NO TRANSACTION handling (drop INVALID index first), `dimmi_releases` table + row per release (11).
  - Tests: 3 goroutine "replicas" migrate once; killed connection mid-migration releases lock; blocked migration retries then fails naming blocker.
- [ ] Migration header lint (`phase`, `min_app_version`, `requires_backfill`) in CI + skew guard at boot.
  - Tests: binary older than min_app_version refuses; schema ahead (expand) boots; checkpoint jump refused.
- [ ] Extend framework `migrate:plan` (lint findings built) with row estimates + live node versions + `--json`; `migrate --force` also skips skew-vs-live-nodes check.
  - Tests: golden output for sample migrations; exit 2 on contract vs old nodes.
- [ ] Pre-migrate backup hook (`upgrades.backup_before_migrate` auto/always/never).
  - Tests: auto skips over 1 GB (stub size), failure aborts boot.
- [ ] Backfill job framework (`maintenance` queue, chunked, resumable, completion flag in `system_settings.backfills.*`).
  - Tests: resume after crash mid-chunk; completion flag; contract migration with `requires_backfill` aborts when incomplete.
- [ ] Upgrade e2e: boot N-1 image with seeded data, rolling to N with 2 replicas, traffic generator asserts zero 5xx except 503 during own restart.
  - Accept: CI job `upgrade-matrix` (N-1 to N, N-2 to N).

## M3 Packaging and release

- [ ] Single `deploy/docker/Dockerfile` (Debian slim + `postgresql-client-18` from apt.postgresql.org, uid 65532), `dimmi healthcheck` command, tzdata embed, read-only rootfs works.
  - Tests: container boots with `--read-only` + tmpfs `/tmp`; healthcheck exit codes; `pg_dump --version` reports 18 inside image; compose e2e `docker compose exec dimmi dimmi backup` succeeds; size gate under 80 MB compressed per arch [verify after first build].
- [ ] `.goreleaser.yaml`: 6 binaries, archives, checksums, SBOM (SPDX), cosign keyless, provenance attestation, multi-arch images + tags policy, changelog migration section.
  - Accept: snapshot release in CI; `cosign verify` and `gh attestation verify` pass in follow-up job.
- [ ] `docker-compose.yml` (dimmi, postgres, valkey 8 with AOF everysec + `noeviction` + `requirepass`), `postgresql.small.conf`, `valkey.small.conf`, Caddyfile, `.env.example` (`VALKEY_PASSWORD`, `DIMMI_MAIL__MAILERS__SYSTEM__DRIVER`), `compose.tls.yml`; compose e2e boot to `/ready`, widget message round trip over `/connection/uni_sse`.
  - Tests: `docker compose config` lint; PG18 volume path; Postgres and Valkey not published.
- [ ] `install.sh` bash (detect, download, verify checksum + optional cosign, user/dirs/config, default installs Postgres 18 + pgvector and Valkey 8 from apt on Debian/Ubuntu, `postgresql-client-18` always, `--database-url` / `--redis-url` skip that install, `--domain` Caddy, systemd enable, setup link print, `/etc/dimmi/install.json` marker, `--uninstall`); `dimmi.service` with sd_notify + watchdog via coreos/go-systemd `daemon`.
  - Tests: shellcheck; e2e in Debian 13 and Ubuntu 24.04 VMs (CI containers with systemd) reaching `/ready` for: no flags, `--database-url` only, `--redis-url` only, both flags; rerun idempotent (marker reused, no refusal).
- [ ] `install.sh` preflight (packaging.md section 5): detect existing Postgres / Valkey / Redis (units, 5432/6379 listeners, `/etc/postgresql`) and refuse without matching flag; external URL checks via downloaded `dimmi doctor --only ...`; all-or-nothing before any change; non-apt distro requires both flags.
  - Tests: edge cases 58 to 61 in systemd containers (preinstalled Postgres, preinstalled Redis, external Postgres without pgvector, external Redis `allkeys-lru`); box state unchanged after each refusal (package list + `/etc` diff).
- [ ] One-click templates on image `dimmi:<major>`: Railway (`railway.json` + template definition doc), Render `render.yaml`, Fly `fly.toml` + README (Fly Managed Postgres + Upstash Redis via `fly redis create`, volume only for local storage; [verify] pgvector on Fly MPG and centrifuge broker commands on Upstash, fallback Valkey Fly app), DO `.do/deploy.template.yaml`, Coolify compose, Dokploy blueprint (both 3-service compose); each with managed Postgres + Valkey/Redis + `REDIS_URL`; `templates-lint` CI job fails when a template lacks Valkey or uses a suffixed image tag.
  - Accept: manual deploy on each platform recorded in release checklist (screenshot + time to `/ready`); lint job green.

## M4 Operations: backups, doctor, maintenance, settings

- [ ] `backups` table, `app/support/backupfmt` (tar stream, manifest, sidecar manifest; encryption = filippo.io/age STREAM, `app_key` X25519 identity via HKDF, `age` recipients mode, `dimmi backup:identity`), `RunBackup` action + job on `backups` queue, node `pg_dump_version` registration (Valkey registry) and queue subscription rule. Valkey never in archive.
  - Tests: round trip archive; tamper detection (age auth failure); stock `age -d -i` decrypts exported identity (CI job with age binary); node without pg_dump on PATH never claims job.
- [ ] `dimmi backup`, `backup:list`, `ScheduleBackups`, `PruneBackups` retention buckets.
  - Tests: retention table-driven (daily/weekly/monthly, manual 30 d, never delete last); pushed only by River leader on scheduler-role node.
- [ ] `dimmi restore` with checks (schema, fingerprint, nodes, `RetentionGuard` per live workspace), hard maintenance, single-transaction pg_restore, files, migrations after.
  - Tests: e2e backup on version N-1, restore into N; failure mid-restore leaves DB untouched; wrong key refused; fake guard false refuses before any write (edge case 104).
- [ ] System backup API (index, store, show, destroy, download-links) + MCP `dimmi_list_backups`, `dimmi_create_backup` + mails on failure.
  - Tests: feature per endpoint (auth, ability, 409s, idempotency), signed URL expiry, audit rows.
- [ ] `app/support/doctor` product checks on `framework/health` registry + group 1 (config, postgres, valkey (`noeviction`, AOF, memory), broadcasting broker probe, migrations, queue, scheduler, nodes, storage, memory, security).
  - Tests: each check pass/warn/fail fixture; parallel run under 5 s with slow fake.
- [ ] Doctor checks group 2 (mail per stream `system`/`customer` with provider auth probes, SES two configuration sets + SNS destinations, provider webhook secrets, SPF/DKIM/DMARC via 04 helpers, inbound driver `ses` only (no SMTP receiver; IMAP and provider inbound later), dns, tls, clock, proxy, websocket + SSE buffering, backups, updates, channels, 13 changelog schedule, 14 feedback portal + customer-stream mail, pg_trgm, 08 `github.app_url_public` + `github.repositories`, 07 `ai.actions.private_hosts`, 16 `imports.queue`); `dimmi doctor`, `GET /system/health`, MCP `dimmi_get_system_health`.
  - Tests: DNS fakes; JSON output schema; exit codes; E119 to E122.
- [ ] Node heartbeat (Valkey hash `dimmi:nodes:<id>` EX 60 + sorted-set index `dimmi:nodes`, no SCAN), `GET /system/nodes`, `GET /system/info`. No table, no prune job.
  - Tests: stale node expires from listing after TTL (frozen clock + Valkey TTL); info resource fields.
- [ ] Maintenance mode: soft mode built in framework (`<data_dir>/framework/down` + Valkey pubsub propagation, `Maintenance` middleware with `MaintenanceExcept`, `dimmi down --message`/`up` with production confirm). App adds: `system_settings` state row, API soft toggle, instance admin bypass. Needs framework follow-ups first: hard mode, River queue pause/resume, late-joining replica sync, bypass secret cookie, centrifuge disconnect on hard mode.
  - Tests: matrix per surface (dashboard, API, widget, webhooks, realtime, workers); propagation under 2 s across 2 in-process nodes.
- [ ] System settings API (show, update) with env lock, secret masking, all-or-nothing PATCH, Valkey pubsub reload; `mail.*`/`channels.email.*` keys rejected (deploy config only); mail test per named mailer (`system`, `customer`) + storage test endpoints.
- [ ] `system.app_url_changed` event (`{from, to}`) from `UpdateSystemSettings` and boot env-change detection (`app.url_last_seen`); add to catalog. Tests: PATCH `app.url` emits once; same value no event; env change at boot emits once; fake listeners (08, 01, 05) receive it.
- [ ] Admin port mount of 08 `GET /internal/tls/ask?domain=` + Caddy TLS profile (`ask http://dimmi:9090/internal/tls/ask`, admin addr compose-network only). Tests: route absent on public listener (404); verified domain 200, unverified/unknown 404; compose e2e with Caddy issues cert only for verified domain (pebble).
  - Tests: locked key 422; mail key 422 `unknown_setting`; secret never returned; mail test failure stages (config, connect, tls, auth, send) via `tc.Mail` boundary fake and Mailpit (smtp relay driver); storage test via SeaweedFS S3 from root `compose.yaml` (`testkit.NewBucket`).
- [ ] Trusted proxies = framework `TrustProxies` (realclientip-go rightmost-trusted-range; 11 supplies presets private, cloudflare), cookie prefix fallback on http, `broadcasting.ping_interval` 25 s.
  - Tests: XFF spoof ignored from untrusted peer; rightmost-untrusted selection; IPv6.
- [ ] `dimmi storage:migrate` local to s3 (batches, resumable, dry-run).

- [ ] Backing-service topology config + checks (decision 20): keys `database.pool_mode`, `database.direct_url`, `database.read_urls`, `database.read_max_lag`, `redis.mode`, `redis.addrs`, `redis.sentinel.master`; boot standby refusal; auto-migrate refusal on pooler without `direct_url`; restore Valkey delete by prefix; doctor `postgres.pooler`, `postgres.replicas`, `postgres.topology`, `valkey.topology`, pool math across pooler + direct. Tests: edge cases 123-140.

- [ ] Nightly `topology` CI job (00-conventions/testing.md section 5): PgBouncer transaction mode, primary + replica promote, Valkey Sentinel failover, 3-primary Valkey cluster with resharding; boot to `/ready` and widget message round trip on each.
  - Tests: fake disks, interrupted run resumes, rows updated.

## M5 Updates, export, budget, docs

- [ ] `CheckForUpdates` (ETag, prerelease filter, security flag), `GET /system/updates`, mail once per version, `dimmi update:check`, egress e2e test (only api.github.com contacted; none when disabled).
  - Tests: fake GitHub responses (200, 304, 403 rate limit, deleted release).
- [ ] Data export: `data_exports` table, `BuildDataExport` streaming zip writer per resource, exclusion list, resumable progress, expiry prune, workspace + instance endpoints, `dimmi export`, `export.completed` event.
  - Tests: export of factory workspace matches API resources; secrets absent (grep test over archive); resume after crash; cloud instance export 404; two-workspace fixture: archive contains zero rows/files of other workspace; owner of B gets 404 on A's `iex_` and download link; non-owner admin 403.
- [ ] Idle budget CI job (method in operations.md section 9) + publish results page.
  - Accept: job fails over budget; results JSON artifact.
- [ ] Docs: self-host quickstart, compose, bare binary (install.sh flags + manual `dimmi serve`), each platform, systemd, upgrade policy, backups/restore runbook, doctor reference, reverse proxy (nginx, Caddy, Traefik, Cloudflare), app scale guide, Postgres and Valkey topology guide (connect only), hardening, telemetry statement ("none").

## Done when

- All M1-M5 tasks checked; CI green incl. upgrade matrix, backup-restore round trip, idle budget, image size gate, signature verification.
- Demo script (recorded for launch):
  1. Fresh Hetzner CX22 (Debian 13). `curl -fsSL https://trydimmi.com/install.sh | sudo bash -s -- --domain demo.trydimmi.dev` (installs Postgres + Valkey by default). Timer start.
  2. Script prints setup URL. Open, create account + workspace. Copy widget snippet into test HTML page, send message. Message in inbox. Timer under 5 minutes.
  3. `dimmi doctor`: all pass except expected warns (backups offsite).
  4. Second box: `docker compose --profile tls up -d` with `.env`. `docker stats` after 10 min: stack (dimmi, postgres, valkey, caddy) under 300 MB.
  5. Deploy buttons: Railway, Render, Fly each reach setup page.
  6. `dimmi backup --include-files`; destroy compose volumes; `dimmi restore <file> --force`; `dimmi up`; conversation still there.
  7. Bump image tag N-1 to N on 2-replica compose (`--scale dimmi=2` with S3 via Garage): migrations once, no failed requests in load script.
  8. `dimmi down --hard` (needs framework follow-up: hard mode), widget shows offline form, `dimmi up`, queued widget message delivered.
  9. `cosign verify ghcr.io/dimmihq/dimmi:<v>` passes; SBOM downloaded from release.

## Email inbound operator surface (owned by 04)

Built in 04 tasks M17 (track 06 email-inbound): operations.md section 15 config docs and doctor rows. No 11 milestone.
