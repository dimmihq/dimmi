# 11 Self-host: operations

Style: caveman. Behavior of running instance: config, first run, upgrades, backups, doctor, maintenance, updates, export, budget, proxy, scale, hardening, settings registry (instance keys), entitlements self_host driver.

## 1. Configuration

### Sources and precedence
1. Built-in defaults (`default` struct tags in framework + `config/*.go`).
2. `dimmi.yaml`: path from `--config`, `DIMMI_CONFIG`, else `./dimmi.yaml`, else `/etc/dimmi/dimmi.yaml`. Missing file fine.
3. Env `DIMMI_<SECTION>__<KEY>` (framework `config`: uppercase, `__` between levels, single `_` stays inside key names: `app.key_auto_generate` = `DIMMI_APP__KEY_AUTO_GENERATE`, `mail.mailers.system.driver` = `DIMMI_MAIL__MAILERS__SYSTEM__DRIVER`). `_FILE` suffix reads value from file (Docker/K8s secrets), surrounding whitespace trimmed; unreadable file fails boot. Both set: file value wins (framework behavior; no conflict error).
4. Aliases for PaaS: `DATABASE_URL` (if `DIMMI_DATABASE__URL` unset), `PORT` (sets `http.addr=:$PORT` if `DIMMI_HTTP__ADDR` unset). Not built yet: framework follow-up (00-foundation tasks "Framework follow-ups"); until then templates set the `DIMMI_*` names.
5. CLI flags on `serve` (`--no-migrate`, `--roles web,worker`; built in framework).
6. `system_settings` rows: only runtime-editable keys registered with scope `instance` in foundation settings registry (section 13), only when steps 2-5 left key unset.

Unknown keys in yaml or `DIMMI_*` env: WARN at boot with closest match ("did you mean DIMMI_MAIL__MAILERS__SYSTEM__DRIVER?"); not fatal (forward compat). Not built yet: framework follow-up (unknown keys are ignored today). `dimmi config:show` (framework) prints effective config, secrets masked; source per key later.

### Reference (11-owned keys; other sections owned by their specs)

| Key | Type | Default | Validation |
|---|---|---|---|
| app.edition | enum | self_host | self_host, cloud |
| app.url | URL | http://localhost:8080 | absolute http(s), no path except `/`, no query; trailing slash stripped |
| app.env | enum | production | local, testing, production (framework; `testing` allows fakes) |
| app.key | secret | none | base64 32 bytes (`base64:` prefix like Laravel); empty = read `<data_dir>/app.key` |
| app.key via `DIMMI_APP__KEY_FILE` | path | none | readable, mode warn if group/world readable (framework `_FILE`; no separate `app.key_file` key) |
| app.key_auto_generate | bool | true in container images (ENV), false for bare binary | `dimmi serve --generate-key` sets true for that run |
| app.data_dir | path | `/var/lib/dimmi` in images, `./storage` otherwise | must be writable dir |
| app.platform | enum | `DIMMI_PLATFORM` when set (image `docker`, compose `compose`, templates their platform), else detect: `kubernetes` (`KUBERNETES_SERVICE_HOST`), `railway` (`RAILWAY_ENVIRONMENT`), `render` (`RENDER`), `fly` (`FLY_APP_NAME`), `digitalocean` (template sets), `systemd` (`INVOCATION_ID`), `docker` (`/.dockerenv`), else `unknown` | |
| app.roles | list | web,worker,scheduler | subset; only nodes with `scheduler` take part in River leader election and push periodic jobs; worker-only nodes run River with leader election disabled |
| app.locale | string | en | default locale (framework i18n) |
| app.locales | list | [en] | served locales; each needs a `lang/<locale>/` dir |
| http.addr | host:port | :8080 | |
| http.admin_addr | host:port | 127.0.0.1:9090 | empty disables |
| http.trusted_proxies | list | empty | CIDR, IP, or presets `private` (10/8, 172.16/12, 192.168/16, fc00::/7, 127/8, ::1), `cloudflare` (embedded list, refreshed per release) |
| http.read_header_timeout | duration | 10s | |
| http.idle_timeout | duration | 120s | |
| http.shutdown_timeout | duration | 25s | |
| database.url | secret URL | required | postgres:// or postgresql://; libpq multi-host + `target_session_attrs` accepted; must reach a primary (section 11.2) |
| database.max_conns | int | 10 | 2-200 |
| migrations.auto | bool | true | framework key (was `database.auto_migrate`) |
| migrations.lock_timeout | duration | 10m | wait for another node's migration lock (was `database.migrate_lock_timeout`) |
| database.connect_retry | duration | 60s | total retry window at boot (later: not in framework `database.Config` yet) |
| database.pool_mode | enum | session | `session` (direct or session pooler), `transaction` (PgBouncer, PgCat, Supavisor, RDS Proxy in transaction mode) |
| database.direct_url | secret URL | empty (= `database.url`) | direct primary connection for session-scoped work (migration lock, River LISTEN, pg_dump); set when `database.url` is a transaction pooler |
| database.read_urls | list of secret URLs | empty | optional read replicas for `db.Reader` call sites |
| database.read_max_lag | duration | 10s | replica over this lag skipped (primary used) |
| redis.url | secret URL | required (`DIMMI_REDIS__URL`, PaaS `REDIS_URL`) | `redis://` or `rediss://`; Valkey 8 (decision 18); standalone, Sentinel or cluster (section 11.3) |
| redis.connect_retry | duration | 60s | boot retry window (later) |
| redis.pool_size | int | 20 | per process (later) |
| redis.key_prefix | string | `dimmi:` | |
| redis.mode | enum | auto | `auto` (detect cluster at connect), `standalone`, `sentinel`, `cluster` |
| redis.addrs | list host:port | empty | extra cluster seed nodes, or Sentinel addresses when `sentinel` |
| redis.sentinel.master | string | empty | master set name; required when `redis.mode=sentinel` |
| upgrades.backup_before_migrate | enum | auto | auto, always, never |
| backups.enabled | bool | auto | |
| backups.schedule | cron (5 field, UTC) | `0 3 * * *` | empty = no schedule |
| backups.disk | disk name | local | must exist in filesystems.disks; `local` resolves `<data_dir>/backups` |
| backups.retention.daily / weekly / monthly | int | 7 / 4 / 6 | 0-365 |
| backups.encryption | enum | auto | auto, none, app_key, age (filippo.io/age; auto = app_key when disk s3) |
| backups.age_recipients | list | empty | age X25519 / SSH public keys; required when `age` |
| backups.include_files | bool | auto | auto = filesystems.default local |
| backups.pg_dump_path | path | `pg_dump` on PATH | |
| updates.check | bool | true | |
| updates.repository | owner/repo | dimmihq/dimmi | |
| updates.include_prereleases | bool | false | |
| auth.signup.mode | enum | invite_only | open, invite_only (00-accounts key) |
| maintenance.retry_after | duration | 60s | `Retry-After` sent while down (framework) |
| broadcasting.ping_interval | duration | 25s | 10-55 s; centrifuge server ping (keeps sockets alive behind 60 s LB idle) |
| broadcasting.max_payload | bytes | 10240 | |
| (no key) broadcasting broker | | | always centrifuge Redis broker on Valkey; not configurable |
| runtime.memory_limit | bytes or `auto` or `off` | auto | auto = 90% of cgroup memory.max when set |
| log.level / log.driver | enum | info / json | `driver` json or text |
| telemetry.tracing | enum | none | `none`, `otlp`; `otlp` only when binary registers `otlp.Tracing()` (`framework/telemetry/otlp`, opt-in, keeps gRPC out of default build); release binary: decision per packaging.md |
| telemetry.otlp_endpoint / otlp_headers | URL / map | empty | OTLP HTTP collector |
| telemetry.metrics | enum | none | `none`, `prometheus` (`/metrics` on admin port) |
| telemetry.sentry_dsn | secret | empty | opt-in error reports |
| encryption.kek / encryption.keks.<name>.driver / encryption.previous | name / enum / list | local / local / [] | KEK wrapping data keysets: `local` (derived from app key; `key` option = older app key) or `awskms` (`key_arn`; own thin driver on aws-sdk-go-v2); `previous` KEKs only unwrap (rotate KEK: set new, keep old in `previous`, app command calls `Keys.RewrapAll`). Keysets one per (scope, purpose); MAC keys never rotated automatically |

Mail keys (owned by foundation `mail` + 04; 04 final model: instance-level provider in deploy config only, never `system_settings`, never per workspace; no SMTP receiver):

| Key | Type | Default | Notes |
|---|---|---|---|
| mail.default | mailer name | system | default mailer **name** (contracts.md "Mail"), not a driver; system Mailables use it |
| mail.mailers.system.driver | enum | log | env `DIMMI_MAIL__MAILERS__SYSTEM__DRIVER`. `ses`, `smtp` (outbound relay only), `postmark`, `resend`, `mailgun`, `sendgrid`, `log`. System stream: auth mails, notifications, digests (06 From) |
| mail.mailers.system.<driver key> | per driver | "" | flat beside `driver`, never nested per driver; secrets accept `_FILE`. Keys: postmark `token`, `message_stream` (default `outbound`); resend `api_key`; mailgun `api_key`, `domain`, `region` (`us`/`eu`); sendgrid `api_key`; ses `region`, `configuration_set` (default `dimmi-system`); smtp `host`, `port`, `username`, `password` |
| mail.mailers.customer.* | map | unset | customer stream (replies, widget offline, changelog, feedback updates). Unset: resolves to `mail.default` mailer with stream overrides (`channels.email.ses.configuration_set`, `channels.email.postmark.message_stream`, tag `stream=customer`; 04 outbound.md 15). Same key shape as system. Production boot fails when it resolves to `log` (04) |
| channels.email.mailer | name | customer | which named mailer 04 uses for customer stream |
| channels.email.provider_verified_domains | list | [] | domains operator verified at Postmark/Resend/Mailgun/SendGrid |
| channels.email.inbound_domain / inbound.drivers | string / list | "" / [] | inbound: `ses` receiving (own AWS). IMAP (later: `imap-adapter`) and Postmark/Mailgun/Cloudflare inbound (later: `extra-mail-adapters`) not at MVP. No SMTP receiver, no port 25 |
| channels.email.postmark.events_webhook_password, resend.webhook_secret, mailgun.signing_key, sendgrid.events_public_key | secret | "" | provider event webhook secrets for `/webhooks/{postmark,resend,mailgun,sendgrid}/events` (04 verifies) |
| channels.email.ses.configuration_set | string | dimmi-customer | customer set; system set via `mail.mailers.system.configuration_set` (default `dimmi-system`) |
| channels.email.postmark.message_stream | string | outbound-customer | Postmark stream for customer mail (system keeps `outbound`) |

### Feature config notes (owned by feature specs; listed for operators)

- Outbound allowlist: `http_client.allow_hosts` (runtime-editable, section 13) also covers 07 agent action connections to intranet APIs and self-hosted MCP servers (`base_url` on private host); `http_client.allow_private_networks` stays off. Cloud never allows private hosts. Doctor `ai.actions.private_hosts`.
- Agent actions (07 `actions.md`): `ai.actions.enabled` (instance kill switch), `ai.actions.max_calls_per_run` 4, `ai.actions.max_rounds` 4, `ai.actions.max_tools_per_run` 20, `ai.actions.run_budget_ms` 20000, `ai.actions.confirmation_ttl` 15m, `ai.actions.approval_ttl` 24h, `ai.actions.max_drafts` 500. Entitlement keys `ai.actions.max_connections`, `ai.actions.max_published`, `ai.actions.calls_hourly`: self-host unlimited (`-1`) unless set in `entitlements.limits.<key>` (section 14).
- Docs sync + GitHub (08 README "Config keys"): `docs_sync.*` (`enabled`, `poll_interval` 15m, `reconcile_interval` 6h, `push_debounce` 10m, budgets), `github.*` (`webhook_body_max_mb`, `delivery_dedupe_ttl`, `rate_limit_floor`, `api_version`). GitHub webhooks need public `app.url`; box behind NAT/VPN: repos fall back to polling automatically (15 min, plus 6 h reconcile), no operator action. Entitlements `docs_sync.max_sources`, `docs_sync.runs_daily` unlimited on self-host.
- Importers (16 README "Config keys"): `imports.enabled`, `imports.sources.<key>.enabled`, `imports.queue.workers` 4 / `imports.files_queue.workers` 8 (queues `imports`, `imports_files`; role `worker`), `imports.public_site.max_pages` 5000, `imports.credentials.ttl_days` 30. Large imports: attachments go through `filesystems.default` (S3 recommended over 10 GB); backups before big import recommended (`dimmi backup`). Imports never billed (cloud) and need no extra service.
- Data export (section 8) is not an importer source at launch (16 sources: Intercom, Zendesk, Help Scout, Featurebase, Productlane, Canny, public site, CSV).

## 2. First run

Sequence inside `dimmi serve`:

1. Load config. Print banner.
2. **App key.** Resolve `app.key` / `app.key_file`. Missing: if `<data_dir>/app.key` exists (`/var/lib/dimmi/app.key` in images, `storage/app.key` for local/bare), use it. Still missing:
   - `app.key_auto_generate=true` (default in compose and the container image) or `--generate-key` flag, and `<data_dir>` writable: generate 32 random bytes, write `base64:...` with `O_EXCL` 0600 (race-safe across replicas sharing volume: loser reads winner's file). Log WARN: "Generated app key at /var/lib/dimmi/app.key. Back it up separately from database backups. Lose it, lose encrypted secrets."
   - else (bare binary default) boot fails: "No app key. Run `dimmi serve --generate-key` (writes <data_dir>/app.key, 0600) or `dimmi key:generate` and set DIMMI_APP__KEY." (framework error `ErrNoAppKey`; `--generate-key` and `key:generate` are app commands/flags, framework `serve` has `--roles`, `--no-migrate` only)
   - Key present in env AND file exists with different key: boot fails (prevents silent switch that breaks decrypt) with both fingerprints shown.
3. **Database.** Connect with retry (exponential, cap 5 s, total `database.connect_retry`). Check server version: under 17 fail; 17 warn ("18 recommended"); 18+ ok. Check extensions available (`pg_available_extensions`): `vector`, `pg_trgm`, `unaccent`, `citext`. Missing: fail with provider-specific hint (RDS: parameter not needed; Debian: `apt install postgresql-18-pgvector`).
4. **Migrate** (section 3). Fresh database: all migrations, then `dimmi_releases` row.
5. **Key fingerprint check.** `system_settings.app.key_fingerprint` absent: store. Present and differs: boot fails "App key does not match key used with this database" (prevents writing new secrets with wrong key). Override `--accept-new-key` only with `key:rotate` flow (foundation).
6. Start roles. Start node heartbeat (Valkey `dimmi:nodes:<id>` hash + `dimmi:nodes` index, README "nodes").
7. **Setup.** Zero rows in `users`: issue setup token (`IssueSetupToken`), log WARN line once per boot:
   `Dimmi is not set up yet. Open https://support.example.com/setup?token=dmi_setup_... (valid 24h). Or run: dimmi user:create`.
   Web middleware `setup_guard` redirects all dashboard routes to `/setup` while zero users. API returns 503 `setup_required` on everything except `/api/v1/setup`, `/up`, `/ready`. Widget API returns 503 `setup_required`.
8. `POST /api/v1/setup` (`CompleteSetup`): in one transaction take `pg_advisory_xact_lock('dimmi:setup')`, re-check zero users, verify token hash + expiry, call 00-accounts `CreateFirstUser{name, email, password, workspace_name}` (creates user email-verified (instance creator owns box), workspace, owner membership, default inbox via 00/03 hooks), set `users.is_instance_admin=true`, delete token, store `app.url` from body if different from config and not env-locked, write audit log. Then session login.
9. Headless alternative: `dimmi user:create --email a@b.c --name Ada --instance-admin --workspace Acme --password-stdin` (00-accounts command; `--instance-admin` added here). Same action. Works any time (not only first run); `--instance-admin` on later users requires flag explicitly.

`app.url` mismatch detection: first authenticated request whose `Host` differs from `app.url` host (after trusted proxy resolution) logs WARN once per hour and doctor reports `app.url_mismatch` (emails, widget snippet, OAuth redirect URIs depend on it).

## 3. Upgrades and migrations

### Normal upgrade
Upgrade note (release that introduces Valkey, decision 18): add Valkey service before pulling (compose file, templates and `install.sh --upgrade` add it; PaaS: attach Redis/Valkey add-on) and set `DIMMI_REDIS__URL`; boot without `redis.url` fails with message pointing here. Nothing to migrate into Valkey (starts empty).

Pull new image / binary, restart. On boot, `migrations.auto=true` (framework `database/migrate` runner over `database.direct_url`, else `database.url`):
1. Dedicated connection, goose v3 session locker (Postgres session advisory lock, runner key `migrate`) until `migrations.lock_timeout` (`migrate.ErrLockTimeout`). Other replicas wait (log "waiting for the migration lock" with holder pid + address from `pg_stat_activity`, every 10 s). Order: steps (River schema, notification tables), framework set (`framework_migrations`), app set (own version table).
2. After lock: read goose version. No pending: release lock, continue (under 50 ms).
3. Pending:
   - **Skew guard** first (below).
   - Pre-migrate backup when `upgrades.backup_before_migrate` = always, or auto and pg_dump present and `pg_database_size` under 1 GB. Runs inline, kind `pre_migrate`. Failure: abort boot (exit 1) unless `never`; message suggests `--no-backup`.
   - Apply migrations in order. Each statement runs with `lock_timeout = '5s'`, `statement_timeout = '15min'` (runner defaults). Lock timeout hit: retry migration up to 5 times with backoff from 1 s doubling (busy table under live traffic), then fail boot. `-- +goose NO TRANSACTION` migrations (e.g. `CREATE INDEX CONCURRENTLY`) run outside tx; failure leaves INVALID index: next run drops `INVALID` index of same name first (migration authoring rule: concurrent index migrations always `DROP INDEX CONCURRENTLY IF EXISTS` + `CREATE INDEX CONCURRENTLY IF NOT EXISTS`).
   - Insert `dimmi_releases(version, schema_version, min_app_version)`.
4. Release lock. Start serving.

Data backfills never in migrations: shipped as River jobs (`maintenance` queue, chunked 1,000 rows, idempotent, resumable), enqueued by migration-adjacent code on first boot of new version.

### Expand / contract rule
- Release N may only **expand**: add tables, nullable columns or columns with constant default, new indexes (concurrently), new constraints `NOT VALID` then validate in later release.
- Code in N must work with schema N-1 + expansions (N's own) and must keep writing columns N-1 reads.
- **Contract** (drop column/table, rename, tighten constraint) only in release N+1 or later, after N stopped using it. Rename = add new + dual-write in N + backfill job + read new in N+1 + drop old in N+2.
- Each migration file header: `-- migrate: phase=expand|contract` (framework `migrate.Lint` rule `header`), app adds `min_app_version=1.3.0` (11 skew guard, app code). CI lint fails without header. `min_app_version` of release = max over its migrations (contract migrations set it to version that stopped using dropped object).

### Version skew rules
| Situation | Behavior |
|---|---|
| Binary version >= latest `dimmi_releases.min_app_version` | boot ok |
| Binary older than `min_app_version` (rollback past contract) | refuse: "Database schema requires Dimmi >= 1.5.0 (contract migrations applied). Running 1.3.2." exit 1 |
| Binary knows fewer migrations than DB has (rolling deploy: N-1 restarts after N migrated) | allowed when skew guard passes; log INFO "schema ahead by 3 migrations (expand only)" |
| Binary has pending migrations but `--no-migrate` | never serve with pending migrations: wait (poll every 5 s) up to `migrations.lock_timeout` for another process to apply them, then exit 1 "pending migrations, run dimmi migrate" |
| Skipping versions (1.1 to 1.6) | allowed: all migrations embedded, applied in order; backfill jobs of skipped versions enqueued in order. Exception: release notes may mark "checkpoint" release (must run once to complete backfills before next contract); binary refuses jump past unfinished checkpoint, names version to run first |
| Downgrade without contract between | allowed (old binary runs on expanded schema) |
| Mixed replicas N and N-1 | supported for adjacent minor versions only; docs say finish rollout before next upgrade |

### `dimmi migrate:plan` (was `migrate --dry-run`)
Framework `migrate:plan` prints set, version, name, phase, transaction flag and lint findings per pending migration, never executes. Target output below (row estimates, live node versions, backup line, `--json`: 11 app additions on `migrate.Runner.Plan`):
```
Current schema: 20261101001106 (Dimmi 1.4.2)
Pending: 3 migrations (target Dimmi 1.5.0, min_app_version 1.4.0)

 20261215000101_add_priority_reason_to_conversations  expand  tx
   ALTER TABLE conversations ADD COLUMN priority_reason text;
   lint: ok (metadata-only)
 20261215000102_index_messages_on_author  expand  no-tx
   CREATE INDEX CONCURRENTLY IF NOT EXISTS ... ON messages (workspace_id, author_id);
   est. rows: 4.1M (pg_class.reltuples), est. time 20-60 s, no write lock
 20261215000103_drop_conversations_legacy_source  contract  tx
   ALTER TABLE conversations DROP COLUMN legacy_source;
   lint: requires all nodes >= 1.4.0. Live nodes: 2 at 1.4.2 (ok)

Pre-migrate backup: will run (db size 412 MB, pg_dump 18.1 found)
```
Lint rules (also CI on migration PRs): non-concurrent `CREATE INDEX` on table over 10k estimated rows (warn in dry-run, fail in CI), `ADD COLUMN ... DEFAULT <volatile>`, `ALTER COLUMN TYPE`, `SET NOT NULL` without prior `NOT VALID` check constraint, missing phase header, `DROP` in expand migration. `--json` output for automation. Exit 0 always (informational), 2 when contract migration conflicts with live nodes' versions.

`dimmi migrate` in production (framework): prints database host/name + every pending migration, asks to confirm; `--force` skips the prompt; non-interactive without `--force` refuses (`ErrNotConfirmed`). 11 extends `--force` meaning: run even when live nodes older than `min_app_version` (operator accepts breaking them).

`dimmi migrate:rollback` (framework): undoes newest applied migration. Outside production free; in production warns (migration + database) and wants the exact version typed back, or `--force --confirm-version <version>`; non-interactive without both refuses. `migrate:status` lists every migration (set, version, name, ran/pending).

## 4. Backups and restore

### Archive format
Single file `dimmi-backup-<UTC yyyymmddThhmmssZ>-<bak_id>.tar` (`.tar.enc` when encrypted). Streamed: no temp copy of whole DB on disk (pg_dump stdout piped into tar writer into Filesystem disk multipart upload or local file).

Tar entry order (manifest last, written after hashes known):
```
database.dump      # pg_dump --format=custom --compress=zstd:3 --no-owner --no-privileges, excluded table data below
files/<path>       # local disk objects (only when includes_files)
manifest.json
```
`manifest.json`:
```json
{
  "format": "dimmi-backup/1",
  "id": "bak_01jc...",
  "created_at": "2026-11-01T03:00:04Z",
  "app_version": "1.4.2",
  "schema_version": 20261101001106,
  "pg_server_version": "18.1",
  "pg_dump_version": "18.1",
  "edition": "self_host",
  "key_fingerprint": "sha256:9f2c...",
  "encrypted": false,
  "database": { "entry": "database.dump", "bytes": 81234567, "sha256": "..." },
  "files": { "count": 1832, "bytes": 412345678, "disk": "local" }
}
```
Excluded table data (schema kept): none needed now (cache, rate limits, realtime history, node registry live in Valkey, never in Postgres; decision 18). `river_job` rows in `running` state are kept (River recovers). Sessions kept (users stay logged in after restore). Valkey is never backed up: restore docs say start with empty Valkey (or let `dimmi restore` delete keys under `redis.key_prefix`); caches refill, realtime clients reconnect with `recovered: false` and refetch.

Encryption: whole tar stream encrypted with filippo.io/age (STREAM, ChaCha20-Poly1305 64 KB chunks; file `.tar.age`). `app_key` mode: recipient = X25519 key derived from app key (HKDF-SHA256 info `dimmi-backup-age-v1`); `dimmi backup:identity > dimmi.agekey` exports the identity so `age -d -i dimmi.agekey file.tar.age | tar x` works without Dimmi. `age` mode: `backups.age_recipients` (identity kept off box). Manifest also copied unencrypted as sidecar `<name>.manifest.json` (no secrets) so list/inspect works without key. No own crypto code.

### Create
`RunBackup` (job or CLI):
1. Mark row `running` (partial unique index enforces one).
2. Check pg_dump major >= server major. Else fail `pg_dump 17 cannot dump server 18`.
3. Stream as above. Upload to `backups.disk` at `backups/<name>`. Compute SHA-256 while streaming.
4. Verify: re-read archive from disk, check tar structure + manifest SHA-256s, run `pg_restore --list` on `database.dump` entry (TOC readable). Local disk: always. S3: when archive under 5 GB (else skipped, logged).
5. Mark `succeeded`, emit `system.backup_succeeded`, enqueue `PruneBackups`.
Failure: delete partial object, row `failed` with error, emit `system.backup_failed`.

### Retention (PruneBackups)
Keep newest `daily` succeeded backups one per UTC day, newest `weekly` per ISO week (Sunday's or latest in week), newest `monthly` per month. `manual` and `pre_migrate` backups: kept 30 days, never counted in buckets. Never delete last succeeded backup. Pruned rows kept (status `pruned`, path null) for history.

### Restore (`dimmi restore <bak_id|path> --force`)
1. Requires `--force` and interactive confirmation typed workspace count (skip with `--yes` for automation).
2. Read manifest. Checks: format known; `schema_version` <= binary's latest migration (restore into older binary refused); key fingerprint matches current key or `--key-file` given (else refuse: encrypted columns unreadable); pg_restore major >= dump's pg_dump major.
3. Enable hard maintenance (`maintenance.state` with `hard=true`); wait 10 s for other replicas to observe and stop workers (they poll); check Valkey node registry: other live nodes with web/worker roles still heartbeating after 30 s: refuse unless `--ignore-nodes` ("stop other replicas first").
4. `pg_restore --clean --if-exists --no-owner --no-privileges --single-transaction --exit-on-error -d <db>` (single transaction: failure leaves DB as before).
5. Restore files (unless `--skip-files`): into `filesystems.default` disk; existing objects with same path overwritten; extra objects left (reported count).
6. Run pending migrations (backup from older version). Delete Valkey keys under `redis.key_prefix` (SCAN + UNLINK on every primary node; never `FLUSHDB`, Valkey may be shared or clustered; `--no-flush-valkey` skips): stale caches and node registry cleared.
7. Re-set maintenance state (restored DB has backup-time state), keep hard maintenance on; print "Restore complete. Verify, then run `dimmi up`."
8. Audit log row written into restored DB.

Restore to fresh box: install, set same app key, `dimmi restore /path/to/file.tar --force` before first boot (works with zero schema: restore creates schema).

## 5. `dimmi doctor`

Checks run in parallel, each 3 s timeout (network), total under 5 s. Output table (TTY) or JSON. Statuses: pass, warn, fail, skip. Exit 1 if any fail. Same registry powers `GET /api/v1/system/health` and MCP tool.

| Id | Checks | Fail / warn |
|---|---|---|
| config.app_key | key present, file mode 0600 if file | fail missing; warn mode |
| config.app_url | absolute URL; https when not localhost | warn http on public host; warn mismatch with observed Host |
| config.unknown_keys | unknown `DIMMI_*` env / yaml keys | warn |
| postgres.connection | connect, latency | fail unreachable; warn latency over 20 ms |
| postgres.version | server version | fail under 17; warn 17 |
| postgres.extensions | vector, pg_trgm, unaccent, citext installed | fail missing |
| postgres.settings | max_connections vs pool sizes x nodes, shared_buffers vs memory, `jit` | warn |
| postgres.disk | `pg_database_size`, data dir free space (compose/install.sh box via statfs when local) | warn under 20% free, fail under 5% |
| valkey.connection | `PING`, latency, version (8.x), `maxmemory-policy` = `noeviction`, AOF on, used memory vs `maxmemory` | fail unreachable; warn latency over 5 ms, eviction policy other than `noeviction`, memory over 80% |
| valkey.shared | other key prefixes present in same db (multi-tenant Valkey) | warn when `redis.key_prefix` is default |
| broadcasting.broker | centrifuge Redis broker + presence on Valkey healthy (publish/subscribe probe on `dimmi:doctor` channel) | fail |
| migrations.status | pending, schema ahead, invalid indexes (`pg_index.indisvalid=false`) | fail pending; warn invalid index |
| queue.health | River queue depth and oldest available job age per queue | warn age over 5 min, fail over 30 min |
| queue.workers | each queue has at least one live node working it | fail missing (e.g. no worker role anywhere) |
| scheduler.leader | River leader elected in last 2 min | fail none |
| nodes.versions | live nodes versions spread | warn mixed versions over 1 h |
| storage.write | write/read/delete probe on default disk | fail |
| storage.scale | more than one live web node and `filesystems.default=local` | fail ("S3 required for >1 replica") |
| storage.ephemeral | platform digitalocean (or other known ephemeral) with local disk | fail |
| mail.system / mail.customer | each named mailer resolves to a driver; provider API auth probe (Postmark server token, Resend key, Mailgun domain, SendGrid key, SES identity) or SMTP relay connect + AUTH (no send) | fail auth/connect; customer stream on `log`: fail in production, warn otherwise; system on `log`: warn |
| mail.configuration_sets | `ses`: both configuration sets (`dimmi-system`, `dimmi-customer`) exist with SNS event destination to `/webhooks/ses/events` | fail missing |
| mail.provider_webhooks | provider event webhook secret set for each non-SES driver in use (bounces/complaints) | warn missing |
| mail.provider_domains | manual drivers: sending domains present in `channels.email.provider_verified_domains` | warn |
| mail.spf / mail.dkim / mail.dmarc | DNS for each sending domain (`channels.email.default_sender_domain`, `channels.email.provider_verified_domains`; reuses 04 helpers) | warn |
| mail.inbound | inbound driver `ses` configured (topic ARN, receipt rule). No SMTP receiver check (none exists) | warn |
| dns.app_url | `app.url` host resolves; points to this box (best-effort public IP compare) | warn |
| tls.certificate | when https: cert valid, expires over 14 days | warn under 14, fail expired |
| clock.skew | compare with Postgres `now()` and HTTP `Date` from api.github.com (skipped when updates disabled and `--skip-network`) | warn over 2 s, fail over 30 s (TOTP, JWT, webhooks signatures break) |
| http_client.private | `http_client.allow_private_networks` on | warn |
| proxy.trusted | requests arrive with X-Forwarded-For from untrusted peer (counter since boot) | warn ("set http.trusted_proxies") |
| websocket.timeout | observed centrifuge WS disconnects with code 1006 near 60 s intervals | warn (proxy idle timeout too low) |
| sse.buffering | `/connection/uni_sse` probe receives first ping within 30 s through `app.url` | warn (proxy buffers SSE: set `proxy_buffering off`) |
| backups.enabled | backups on, last success under 26 h (scheduled) | warn none in 26 h, fail none in 72 h |
| backups.worker | backups enabled and a live node has compatible pg_dump | fail |
| backups.offsite | backups disk local only | warn ("copy off box: set backups.disk to s3") |
| backups.encryption | s3 disk with encryption none | warn |
| security.registration | `auth.signup.mode=open` | warn |
| security.admin_port | admin addr bound non-loopback | warn |
| security.default_password | Postgres password equals known example values | fail |
| security.instance_admin_2fa | instance admins without 2FA | warn |
| updates.available | newer stable release | warn with changelog link |
| channels.errors | channel connections in `error` state (05) | warn per connection |
| changelog.schedule | overdue scheduled entries, mailings stuck `sending` over 30 min (13) | warn |
| feedback.portal | feedback portal reachable on help center host (14) | warn |
| feedback.mail | customer-stream mailer not `log` when feedback portal enabled (sign-in and updates need it) (14) | warn |
| github.app_url_public | `app.url` host resolves to non-private IP when docs sync, 08 git knowledge or 13 GitHub sources exist (GitHub cannot reach private hosts; HEAD from outside not possible) | warn: "GitHub webhooks cannot reach this box; repos use polling (`docs_sync.poll_interval` 15 min)" |
| github.repositories | repos in poll mode, webhook status `failing`, connection tokens expiring under 14 days or invalid (08) | warn |
| ai.actions.private_hosts | published action connections whose `base_url` host is private and not in `http_client.allow_hosts` (calls would be blocked by SSRF guard) (07) | warn per connection |
| imports.queue | queue `imports` / `imports_files` has live worker when import runs exist (16) | warn |
| postgres.pg_trgm | `pg_trgm` installed (14 similar-posts, 03 search) | fail |
| memory.limit | cgroup limit under 384 MB | warn |

## 6. Maintenance mode (`dimmi down` / `dimmi up`)

Built in framework (`foundation` `MaintenanceStore`): state `{enabled, message, since}` in `<data_dir>/framework/down`, changes announced on Valkey pubsub so every replica flips within a second; `down` / `up` work without booting the app (config + Valkey only). `dimmi down --message "..."`; production asks to confirm (`--force` skips; non-interactive without `--force` refuses). `maintenance.retry_after` (60s) sets `Retry-After`. `/up`, `/ready`, `/metrics` and `foundation.Options.MaintenanceExcept` paths (inbound `/webhooks/*`) keep working.

Framework follow-ups (00-foundation tasks "Framework follow-ups"): hard mode, queue pause, late-joining replica sync (replica that boots after `down` on another node without shared data dir does not see it), secret bypass cookie, `system_settings.maintenance.state` mirror. Target state shape and surfaces below stay the 11 design:

```json
{ "enabled": true, "hard": false, "message": "Upgrading database, back in 10 minutes", "retry_after": 60, "secret_hash": "sha256:...", "started_at": "...", "started_by": "cli" }
```

| Surface | Soft (`dimmi down`) | Hard (`dimmi down --hard`) |
|---|---|---|
| Dashboard web | 503 HTML page with message | same |
| REST API `/api/v1` | 503 `{"message":...,"code":"maintenance"}` + `Retry-After` | 503 `maintenance_hard` |
| Bypass | cookie from `/<secret>` URL (Laravel), instance admins with valid session also bypass soft mode | secret cookie only; gives full access (operator verifying restore) |
| Widget API | 503 `maintenance`; widget shows offline form, retries (foundation) | same |
| Realtime | existing connections kept; new connections allowed (read-only events) | centrifuge disconnect all with reconnect advice (code 3001 `shutdown`, clients back off and reconnect after `dimmi up`) |
| Inbound provider webhooks `/webhooks/*` | accepted, persisted, queued (never lose messages) | 503 + `Retry-After` (providers retry: Slack, Telegram, SES/SNS, WhatsApp, Discord interactions retry with backoff) |
| Workers (River) | run normally | all queues paused (River `QueuePause`), resume on `up` |
| Scheduler | runs | paused |
| `/up` | 200 | 200 |
| `/ready` | 200 `maintenance:true` | 200 `maintenance:true` |
| `/api/v1/system/*` | allowed for instance admins | allowed for instance admins (read), writes 503 except maintenance destroy |

API can enable soft only. Hard only via CLI (operator with shell). `dimmi up` clears both, resumes queues.

## 7. Update notifications

`CheckForUpdates` (periodic job, pushed by River leader on a scheduler-role node): `GET https://api.github.com/repos/<updates.repository>/releases/latest` via `a.HTTP` (framework `httpclient`) with explicit allow (public host), headers `User-Agent: dimmi`, `Accept: application/vnd.github+json`, `If-None-Match` with stored ETag. One request per 24 h stays far below GitHub unauthenticated limit (60 requests/hour per IP), even behind shared NAT. Parse `tag_name` semver, `html_url`, `published_at`, body first 2,000 chars (stored, rendered as untrusted Markdown). Prereleases skipped unless `updates.include_prereleases`.

Store `system_settings.updates.latest`. Newer than running: `system.update_available` once per version: mail to instance admins, `GET /api/v1/system/updates` shows, dashboard banner for instance admins (frontend later). Security releases: release body containing line `Security: true` (release template) marks `security: true`; mail subject prefixed "[Security]".

Failures (offline box, GitHub down, 403 rate limit): log DEBUG, retry next day; never user-visible error. `updates.check=false`: no request ever (verified by e2e egress test).

## 8. Data export

### Format
Zip (`ZIP64`, streamed) `dimmi-export-<scope>-<UTC ts>-<iex_id>.zip`:
```
manifest.json                       # format "dimmi-export/1", scope, app_version, created_at, counts per file, workspaces
workspaces/<ws_id>/workspace.json   # WorkspaceResource + settings
workspaces/<ws_id>/members.jsonl
workspaces/<ws_id>/teams.jsonl
workspaces/<ws_id>/inboxes.jsonl
workspaces/<ws_id>/channel_connections.jsonl   # config only, credentials excluded
workspaces/<ws_id>/contacts.jsonl
workspaces/<ws_id>/companies.jsonl
workspaces/<ws_id>/conversations.jsonl         # ConversationResource full form (include=tags,participants)
workspaces/<ws_id>/messages.jsonl              # MessageResource incl. notes (internal export), event messages
workspaces/<ws_id>/tags.jsonl
workspaces/<ws_id>/saved_replies.jsonl
workspaces/<ws_id>/help_centers.jsonl, collections.jsonl, articles.jsonl, article_versions.jsonl
workspaces/<ws_id>/ai_agents.jsonl              # config, prompts, no provider keys
workspaces/<ws_id>/knowledge_sources.jsonl      # source definitions, no crawled chunks/embeddings
workspaces/<ws_id>/webhook_endpoints.jsonl      # URLs, events, no secrets
workspaces/<ws_id>/audit_logs.jsonl
workspaces/<ws_id>/attachments/<att_id>/<filename>
users.jsonl                         # instance scope only: id, name, email, created_at
```
- One JSON object per line, same Resource JSON as REST API (stable, documented, versioned with API v1). Re-importable by future 16 importer source (later: `more-import-sources`).
- Excluded always: password hashes, 2FA secrets, recovery codes, session rows, token hashes, OAuth secrets, BYOK keys, channel credentials, webhook signing secrets, identity verification secrets, embeddings, cache tables, billing ledger (cloud; usage CSV via 12 instead).
- Erased contacts appear as tombstones (`"name":"Deleted contact"`), same as API.
- Streaming: keyset pagination 1,000 rows per query, constant memory. Progress row updated every 5,000 rows.
- Resumable: job retry continues from `progress` checkpoint (per file cursor); zip written as new object on retry (partial object deleted).
- Expires 7 days after success (object deleted, row `expired`).
- Cloud: workspace export allowed (owner), instance export 404 `feature_unavailable`.

## 9. Resource budget: measurement method

Published numbers come only from this method. Job `idle-budget` in CI (GitHub-hosted `ubuntu-latest`, 4 vCPU, plus monthly run on real $5-class VPS: Hetzner CX22 / DO 1 GB, results posted to docs).

1. Fresh VM, cgroup v2, Docker. `docker compose --profile tls up -d` with `DIMMI_DOMAIN=localhost` and internal CA (no ACME).
2. Seed: `dimmi db:seed --demo` (1 workspace, 3 members, 500 conversations, 5,000 messages, 20 articles).
3. Warm: open 20 WebSocket connections (centrifuge-js Node script) subscribed to `workspace:` and `conversation:` channels (presence on) plus 5 widget `uni_sse` streams; send 50 widget messages over 2 minutes; then idle.
4. Wait 10 minutes (Go GC + Postgres checkpoints settle).
5. Sample every 10 s for 5 minutes per container (dimmi, postgres, valkey, caddy): `memory.current - memory.stat:inactive_file` from `/sys/fs/cgroup/.../memory.*` (what `docker stats` reports on cgroup v2). Also per-process RSS + PSS via `/proc/<pid>/smaps_rollup` inside containers for breakdown.
6. Report p50, p95, max per container and stack sum. Budget uses stack p95.
7. Gates: stack p95 under 300 MB, dimmi under 50 MB, valkey under 20 MB. Regression over 10% vs last release fails (foundation rule).
8. Artifacts: JSON results + markdown table committed to docs site `self-host/resource-usage.md` with date, version, VM type.

Dimmi runtime settings for budget: `GOMEMLIMIT` auto from cgroup (90%), `GOGC` default 100, otter caches bounded (config), pgx pool idle conns min 1 max idle time 5 min, River fetch poll idle backoff. Postgres: small conf (packaging.md 3.1). Valkey: `maxmemory 64mb`, `maxmemory-policy noeviction`, `appendonly yes`, `appendfsync everysec`, `save ""` (no RDB), `io-threads 1`.

$5 VPS (1 GB RAM) plan: dimmi ~50 MB + Postgres ~180 MB + Valkey ~20 MB + Caddy ~25 MB idle; leaves room for page cache and bursts. Under 768 MB total RAM: doctor `memory.limit` warns.

## 10. Reverse proxy notes

| Concern | Rule |
|---|---|
| Trusted proxies | Dimmi reads `X-Forwarded-For`, `X-Forwarded-Proto`, `X-Forwarded-Host`, `Forwarded` (RFC 7239) only when TCP peer in `http.trusted_proxies`. Client IP = rightmost XFF entry not in trusted set. Untrusted peer sending XFF: headers ignored, counter for doctor |
| Scheme | `app.url` scheme is truth for generated URLs and cookie `Secure`; `X-Forwarded-Proto` only used to redirect http to https when app.url is https |
| Cookie prefix | `__Host-` cookies require https; app.url http (localhost dev) falls back to non-prefixed cookie name, doctor warns on public host |
| WebSockets | path `/connection/websocket` must pass `Upgrade`/`Connection` headers. Proxy idle/read timeout at least 60 s; centrifuge server pings every `broadcasting.ping_interval` (25 s), keeping connection under common 60 s defaults (AWS ALB 60 s, nginx `proxy_read_timeout` 60 s, Cloudflare 100 s). Recommend 300 s anyway. Widget uses SSE on `/connection/uni_sse` (needs `proxy_buffering off`) |
| nginx | snippet in docs: `proxy_http_version 1.1; proxy_set_header Upgrade $http_upgrade; proxy_set_header Connection $connection_upgrade; proxy_read_timeout 300s; proxy_buffering off;` for `/connection/`, `client_max_body_size 30m` (25 MB uploads + overhead) |
| Traefik / Coolify / Dokploy | WebSocket works by default; set `respondingTimeouts.readTimeout` 0 or 300 s |
| Cloudflare | WebSockets on (default); 100 MB upload limit fine; use `cloudflare` trusted proxies preset; do not cache `/api/*`, `/widget/*` (Dimmi sends `Cache-Control: no-store`) |
| Body size | proxy limit at least 30 MB for uploads route `/api/v1/attachments` and `/widget/v1/attachments` |
| SSE / streaming | widget realtime `/connection/uni_sse`, AI streaming endpoints (07) need `proxy_buffering off` / `X-Accel-Buffering: no` header (Dimmi sends it) |
| Help center custom domains | proxy must forward original `Host` (08 routes by host) |
| Subpath hosting | not supported (Dimmi must own host root); `app.url` with path rejected at boot |

## 11. Scaling and backing-service topologies

Scope rule (PO 2026-11-02): Dimmi scales only its own process. Running, scaling, replicating and failing over Postgres and Valkey belongs to the operator or managed provider, never to Dimmi (no Dimmi-run replication, failover, sharding or proxy). Dimmi's job: connect correctly to whatever topology the operator hands it, single instance or scaled, and behave predictably during their failovers.

### 11.1 App replicas

| Topic | Rule |
|---|---|
| When | single node handles first thousands of conversations/day; scale web for WebSocket count (target 10k sockets per 1 GB node) or HA |
| Required for >1 replica | `filesystems.default=s3` (any S3-compatible: AWS S3, R2, B2, Garage, SeaweedFS, RustFS; never recommend MinIO, archived). Doctor `storage.scale` fails otherwise; uploads land on one node and 404 on others. `DIMMI_APP__KEY` explicit unless replicas share volume |
| Roles | all roles on every node safe (River leader election among scheduler-role nodes picks one; worker-only nodes run with leader election disabled and never push periodic jobs). Optional split: `DIMMI_APP__ROLES=web` (or `serve --roles web`) nodes behind LB + `worker,scheduler` nodes. At least one node needs `scheduler` or periodic jobs never run (doctor `scheduler.leader`) |
| Realtime fan-out | centrifuge built-in Redis broker + presence manager on Valkey (history + recovery per channel). No sticky sessions needed. Member kick and session revoke reach all nodes via broker control channel |
| Caches | framework `cache.Store` on Valkey (rueidisaside: server-assisted client-side cache for hot keys); per-process otter L1 where owner specs ask; settings/maintenance changes via pubsub |
| Rate limits | Valkey (`ratelimit` `redis` driver) on every deployment, single node included (one code path) |
| Backups | one backup at a time (DB-enforced); runs on any node with pg_dump, over `database.direct_url` |
| Discord gateway | single holder via River leader election (05) |
| Deploys | rolling, one node at a time; migrations by first booting node under lock; others wait |
| Load balancer | health `/ready`; no sticky sessions; WebSocket support; idle timeout >= 60 s (300 s recommended) |

### 11.2 Postgres topologies (connection contract)

Single writer always: every topology below has exactly one primary. Sharded or multi-primary Postgres (Citus, BDR, pgEdge) not supported (advisory locks, River, sequences assume one writer).

| Topology | Config | App behavior |
|---|---|---|
| Single instance (default; compose, VPS) | `database.url` | all reads and writes on it |
| HA primary behind one writer endpoint (RDS/Aurora writer, Cloud SQL, Neon, Supabase, Crunchy, CloudNativePG `-rw` service, Patroni + HAProxy) | `database.url` = writer endpoint | failover = connection errors, then pgx reconnects through endpoint to new primary |
| HA via libpq multi-host | `database.url` = `postgres://h1,h2,h3/dimmi?target_session_attrs=read-write` (pgx native) | pgx tries hosts in order, keeps only the read-write one |
| Transaction-mode pooler (PgBouncer, PgCat, Supavisor, RDS Proxy) | `database.url` = pooler, `database.pool_mode=transaction`, `database.direct_url` = primary (recommended) | pgx `QueryExecModeExec` (unnamed statements; no named prepared statement cache). Tenant `SET LOCAL` already transaction-scoped. Session-scoped work goes to `direct_url`: goose migration session lock, River LISTEN notifier, pg_dump. No `direct_url` with `pool_mode=transaction`: boot fails (`database: direct_url is required behind a transaction pooler`; framework `Config.SessionURL()`), `migrate` commands too. Poll-only River fallback not built |
| Session-mode pooler | `database.url` = pooler, `pool_mode=session` (default) | same as single instance |
| Read replicas (optional) | `database.read_urls` list (replica hosts, Aurora reader endpoint, CloudNativePG `-ro`) | only call sites using `db.Reader(ctx)` read there (stale-tolerant reads: reports, exports, public help center pages; owner specs opt in, default primary). Built (framework `db.Reader(ctx)`). Reader opens `BEGIN READ ONLY` so tenant `SET LOCAL` + RLS apply. Falls back to primary when no replica configured, replica unhealthy, lag over `database.read_max_lag` (10 s; lag 0 when receive LSN = replay LSN, else `now() - pg_last_xact_replay_timestamp()`, sampled every 5 s), or request already wrote (ctx write marker `database.TrackWrites(ctx)`: read-your-writes within a request) |

Failover rules (all topologies):
- `database.url` must reach a primary: boot checks `pg_is_in_recovery()`; true = boot fails "database.url points at a standby".
- Pooled connection lands on demoted primary (SQLSTATE `25006`): framework resets the pool, request returns 503 `service_unavailable` + `Retry-After: 1`.
- Connection lost mid-transaction: never auto-retry once COMMIT may have been sent (outcome unknown). Request 503 retryable; clients retry safely with `Idempotency-Key`. Retry automatically only when acquire or the first statement failed.
- River jobs: normal attempt retry; jobs already idempotent (at-least-once).
- Pool math: `nodes x (database.max_conns + 2) + 5 <= max_connections` without pooler (the 2 = River notifier + migrate/backup on direct connection). With a pooler, the same sum applies to pooler client limits; pooler server pool sized by operator.

### 11.3 Valkey topologies (connection contract)

| Topology | Config | App behavior |
|---|---|---|
| Standalone (default; compose, VPS) | `redis.url` (`redis://`, `rediss://`) | all commands on it |
| Managed HA with one primary endpoint (ElastiCache non-cluster, Memorystore, Upstash, Railway/Render add-ons) | `redis.url` = primary endpoint | failover = reconnect through endpoint |
| Sentinel | `redis.mode=sentinel`, `redis.addrs` = sentinels, `redis.sentinel.master` = master set; credentials/TLS from `redis.url` | rueidis follows master switch; centrifuge broker built with same Sentinel config |
| Cluster mode (ElastiCache cluster mode, self-run Valkey cluster) | `redis.url` = configuration endpoint or any node, `redis.mode=auto` (detects cluster) or `cluster`, extra seeds in `redis.addrs` | rueidis follows `MOVED`/`ASK`; centrifuge broker in cluster mode; `framework/pubsub` uses sharded `SPUBLISH`/`SSUBSCRIBE`; key rules in data-model.md "Valkey" (hash tags, no logical db index, no SCAN on request paths) |

- Reads never go to Valkey replicas: all commands on primaries (small state, simpler consistency).
- One logical Valkey only: client-side sharding across independent instances not supported.
- During any failover window, writes fail briefly and the Valkey-down policy applies (data-model.md). Async replication may drop the last writes (counters, once-keys, presence): same outcome as a Valkey restart; durable dedupe stays in Postgres.
- `redis.url` with a db index (`/2`) in cluster mode: boot fails ("logical databases unsupported in cluster mode").

## 12. Security hardening defaults

| Default | Detail |
|---|---|
| Nonroot | uid 65532 in the image; system user `dimmi` on install.sh hosts |
| Read-only rootfs | compose `read_only: true`, tmpfs `/tmp`; writes only to data dir volume |
| Capabilities | `cap_drop: [ALL]`, `no-new-privileges` |
| Network | Postgres and Valkey not published; Valkey `requirepass` random 24 bytes (installer/templates), `protected-mode yes`; admin port 127.0.0.1; app port 8080 (Caddy 80/443 in TLS profile) |
| DB auth | scram-sha-256, random 24-byte password generated by installer/templates; doctor fails on known example passwords |
| Registration | `invite_only` after first user |
| Setup | token-gated (section 2) |
| Instance admin | separate flag; cloud requires 2FA; self-host doctor warns without 2FA |
| Secrets | `_FILE` support, never logged, `config:show` redacts |
| Headers | HSTS (1 year, includeSubDomains off by default) when app.url https; CSP per foundation; `X-Content-Type-Options`, `Referrer-Policy: strict-origin-when-cross-origin`, `Permissions-Policy` minimal |
| River UI `/admin/jobs` | not included (deferred; framework follow-up). When added: instance admins only (`users.is_instance_admin`, `instance_admin` middleware; jobs span workspaces) |
| pprof | admin port only |
| Outbound HTTP (`framework/httpclient`) | private networks blocked by default (`http_client.allow_private_networks`) |
| Dev-only commands | `make:*` generators, `route:lint`, `lang:check` compiled only with `dev` build tag; release binary has none (production commands: `serve`, `migrate`, `migrate:status`, `migrate:plan`, `migrate:rollback`, `down`, `up`, `config:show`, `route:list` + app commands) |
| Supply chain | signed images, SBOM, pinned base digests, Renovate updates, `govulncheck` in CI, release blocked on critical CVE in image scan (Grype) |
| Systemd | sandboxing directives (packaging.md section 5) |

## 13. System settings registry (runtime-editable keys)

Registry lives in foundation `app/support/settings` (scopes instance, organization, workspace, user; `00-foundation/extensibility.md` section 7). 11 registers keys below as `Definition{Scopes: [instance], Secret, Validate}` in `app/support/settings/instance_keys.go` and owns their storage (`system_settings`). Instance is top scope: no lock semantics here; a key also declared for workspace/organization scope (later) reads instance value as default. Other specs may register instance keys the same way (appear in `GET /system/settings`).

Only these keys writable via API/`settings:set`. Each: type, validator, secret flag, apply mode.

| Key | Type | Secret | Apply |
|---|---|---|---|
| app.url | URL | no | immediate (cache bust); emits `system.app_url_changed` `{from, to}` (08 rebuilds canonical URLs, 01 loader URLs, 05 webhook registrations); warns links in sent emails unchanged |
| app.name_override | string 1-60 | no | branding of instance-level emails ("Dimmi" default) |
| auth.signup.mode | enum | no | immediate |
| filesystems.default | enum local, s3 | no | new uploads only; existing attachments keep `attachments.disk` |
| filesystems.disks.s3.bucket / region / endpoint / use_path_style / public_url | | no | immediate |
| filesystems.disks.s3.key / secret | string | yes | immediate |
| backups.schedule, backups.retention.*, backups.disk, backups.encryption, backups.include_files | | no | next run |
| updates.check, updates.include_prereleases | bool | no | next run |
| http_client.allow_private_networks / allow_hosts | bool / list | no | immediate; audit logged; doctor warns |

Not runtime-editable (deploy config only, 04 final model): every `mail.*` and `channels.email.*` provider key. Changing provider = edit env/yaml and restart.

PATCH semantics: all keys validated first, then applied in one transaction; any invalid key: 422, nothing applied. Secret key set to empty string clears it. Change publishes `system` message on Valkey pubsub; every node reloads settings within 2 s (1 s poll fallback while Valkey disconnected). Key registered with other scope only (workspace, user): 422 `unknown_setting` (not instance-editable).

## 14. Entitlements `self_host` driver

Contract `Entitlements{Has, Limit}` (foundation `contracts.md`, extensibility.md section 6). Driver chosen by `app.edition`: `self_host` (11) or `cloud` (12).

| Call | self_host result |
|---|---|
| `Has(ws, feature)` | true for every catalog feature (incl. rollout flags: self-host ships features on, operator disables via owning spec config if needed) |
| `Limit(ws, key)` | 1. `entitlements.limits.<key>` operator map if set; 2. catalog entry's self-host config key (e.g. `webhooks.max_endpoints`); 3. catalog self-host default (`-1` = unlimited for every cloud cap) |

- Unknown key in `entitlements.limits` fails boot (catalog registry, closest-match hint).
- No DB reads, no cache needed (config in memory; reload on SIGHUP config reload).
- `GET /api/v1/workspace/entitlements` on self-host: features all true, limits `-1` unless operator set them; dashboard shows "unlimited".
- `dimmi entitlement:set` (12, cloud staff) not registered on self-host.
