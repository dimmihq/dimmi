# 11 Self-host: edge cases

Style: caveman. Format: Case. Behavior. Final sweep 2026-11-02: Valkey required (decision 18), centrifuge realtime, mail final model; 107-118 added. Deployment simplification 2026-11-02: single-container cases cut, 58-61 (install.sh defaults) added, renumbered without gaps. Tests named in plain words; every case covered.

## First run and setup

1. Two people open setup URL at same time. Both POST: `pg_advisory_xact_lock('dimmi:setup')` serializes; second sees users exist, gets 410 `setup_completed`, token already deleted.
2. Attacker finds fresh instance on public IP before owner. No setup token: cannot create user (403 `invalid_setup_token`). `GET /api/v1/setup` reveals only `needs_setup: true` and version.
3. Setup token leaked in shared logs (Railway log viewer shared with teammate). Token single use, 24 h expiry; operator can rotate with `dimmi setup:link` (old token invalid).
4. Setup token expired, process not restarted. `GET /setup` page shows "Token expired. Run `dimmi setup:link` or restart Dimmi" ; PaaS users without shell: restart issues new token (logged).
5. Operator never sees logs (one-click PaaS, logs hidden). Template README and deploy success page link to platform log viewer; fallback `dimmi user:create` via platform shell (Railway, Render, Fly all provide shell).
6. Setup submitted with `app_url` differing from env-locked `DIMMI_APP__URL`. Env wins; response includes `warnings: ["app_url_locked_by_env"]`; no error.
7. Setup email on disposable-domain list (00-accounts check). Self-host edition skips disposable check for setup and `user:create` (operator owns box).
8. `user:create` run while web setup in progress. Same advisory lock; CLI user created first wins, web gets 410.
9. All instance admins deleted or demoted by SQL. `dimmi admin:promote <email>` restores; doctor fails `security.no_instance_admin`.
10. Demote last instance admin via API. 422 `last_instance_admin`.
11. Instance admin also workspace member: system routes ignore workspace header; workspace roles unaffected.

## Config and app key

12. `DIMMI_APP__KEY` and `DIMMI_APP__KEY_FILE` both set. File value wins (framework `config`: `_FILE` beats plain value, no conflict error); unreadable file fails boot naming the variable.
13. Two replicas share volume, both auto-generate key at once. `O_EXCL` create: one wins, other reads file. Replicas without shared volume and no env key: second replica generates different key, then key fingerprint check (stored in DB by first) fails boot with message "set DIMMI_APP__KEY explicitly for multiple replicas".
14. Operator restores volume without `app.key` but database has fingerprint. Auto-generate would create new key; fingerprint mismatch blocks boot: "App key does not match database. Restore app.key or set DIMMI_APP__KEY." Never writes data with wrong key.
15. Data dir read-only (misconfigured volume, container with read-only root and no volume). Key auto-generate fails: boot error names path and fix (mount volume at `/var/lib/dimmi` or set `DIMMI_APP__KEY`).
16. `PORT` set by PaaS and `DIMMI_HTTP__ADDR` set. Explicit `DIMMI_HTTP__ADDR` wins, WARN logged (PaaS health check may fail). `PORT` alias not built yet (framework follow-up).
17. `DATABASE_URL` uses `postgresql://` scheme or contains `sslmode=require` with self-signed cert. Accepted; `sslmode=verify-full` needs `sslrootcert`, error message explains.
18. Unknown env key typo `DIMMI_MAIL__MAILERS__SYSTEM__DRVIER`. Boot continues, WARN with suggestion; doctor `config.unknown_keys` warn. Not built yet: framework follow-up (today key silently ignored).
19. `app.url` with path (`https://example.com/support`). Boot fails: subpath hosting unsupported.
20. `app.url` changed after emails sent. Old links in sent emails still resolve if old host still routes to instance; signed URLs validated by path + signature, not host. Doctor warns `app.url_mismatch` if traffic arrives on old host.
21. Unicode/IDN domain in `app.url`. Stored as punycode for cookies and links, displayed as Unicode.

## Database and migrations

22. Postgres 16 or older. Boot fails with version and upgrade doc link.
23. Managed Postgres without pgvector available (some hosts). Boot fails listing missing extension and host-specific hint; never starts half-working.
24. App role lacks `CREATE EXTENSION` privilege but extensions already installed by provider. Migrations use `CREATE EXTENSION IF NOT EXISTS`; when present, no privilege needed; when absent and no privilege, error names exact SQL for operator to run as superuser.
25. Three replicas boot simultaneously with pending migrations. One holds advisory lock and migrates; others wait (log holder), then see no pending, start. None serves before migrations done.
26. Migrating replica crashes mid-way. Session advisory lock released by Postgres on disconnect; transactional migration rolled back; next replica acquires lock and re-runs. Non-transactional concurrent index left INVALID: dropped and rebuilt (authoring rule).
27. Migration blocked by long transaction holding lock (analytics query). `lock_timeout 5s`, retry 5 times with backoff, then boot fails naming blocking pid from `pg_locks`/`pg_stat_activity`.
28. Migration takes longer than PaaS health check start period (Render, Railway kill container). `/up` serves 200 during migration (process alive), `/ready` 503; templates use `/ready` with generous start period (300 s); systemd `TimeoutStartSec=15min`; docs: run `dimmi migrate` as release command for big upgrades.
29. Operator rolls back image after contract migration ran. Old binary refuses boot (skew guard) with version needed; no silent corruption.
30. Operator rolls back after expand-only release. Old binary boots fine; new nullable columns ignored.
31. Version jump across checkpoint release whose backfill unfinished. Binary refuses, names checkpoint version to run first.
32. Backfill job fails repeatedly (bad row). Job retries, then discarded after max attempts; doctor `migrations.backfills` warns with job id. Completion tracked in `system_settings.backfills.<name> = done`. Contract migrations declare `requires_backfill`; migrator aborts that migration with message while backfill incomplete; currently running binary keeps serving.
33. Pre-migrate backup fails (disk full). Boot aborts before touching schema (`auto`/`always`); operator frees disk or sets `DIMMI_UPGRADES__BACKUP_BEFORE_MIGRATE=never` for one boot.
34. Database larger than 1 GB, `auto` mode. Pre-migrate backup skipped with WARN recommending manual backup/PITR; migration proceeds.
35. `migrate:plan` against DB with schema ahead of binary. Prints "schema ahead by N migrations, nothing to apply", exit 0.
36. Postgres major upgrade 17 to 18 on compose. Not automatic; doc: `dimmi backup`, new PG18 volume, `dimmi restore`. PG18 image mount path changed to `/var/lib/postgresql`; compose file already uses PG18 layout; docs warn users migrating own compose from 17 (old `/var/lib/postgresql/data` mount silently unused).

37. Operator runs `dimmi migrate` while app replicas serve. Allowed: same advisory lock; expand migrations safe with live N-1 code; contract migration with live older nodes: refused unless `--force` (lists nodes).

## Backups and restore

38. Backup requested while one running. 409 `backup_in_progress` (API) / CLI waits up to 10 min with `--wait`, else exits 75.
39. Scheduled backup fires on two replicas. River periodic job pushed only by leader (leader election among scheduler-role nodes only); unique job per minute; partial unique index final guard.
40. No node has pg_dump (bare binary run by hand on host without `postgresql-client-18`; image and install.sh always ship it). `POST /system/backups` 409 `no_backup_worker`; scheduled jobs not enqueued (scheduler checks Valkey node registry), doctor fail if `backups.enabled=true` explicitly.
41. pg_dump 17 on node, server 18. Node registers `pg_dump_version`; incompatible nodes do not subscribe `backups` queue; error message if forced via CLI.
42. Disk fills during backup. Stream write error: partial file deleted, row `failed` with "no space left on device", mail to instance admins (max 1 per 6 h).
43. S3 upload fails mid multipart. Abort multipart (no orphan parts; bucket lifecycle rule for incomplete uploads recommended in docs), row failed, retry once after 5 min.
44. Backup during heavy writes. pg_dump uses consistent snapshot (repeatable read); app keeps running; files copied after dump may include attachments newer than dump (harmless: unreferenced files; restore leaves them, purge job removes unattached after 24 h).
45. File deleted between listing and copy during backup. Skipped, counted in manifest `files.skipped`, warning not failure.
46. Backup archive tampered/corrupt. Restore checks SHA-256 per entry before applying; mismatch aborts before any DB change.
47. Restore backup from newer Dimmi into older binary. Refused (`schema_version` ahead).
48. Restore backup from older Dimmi. Restored, then pending migrations applied automatically, then remains in hard maintenance until `dimmi up`.
49. Restore with other replicas still running. Refused after 30 s unless `--ignore-nodes`; hard maintenance already paused their queues and blocked HTTP.
50. Restore fails mid pg_restore. `--single-transaction`: database unchanged; maintenance stays on; message says DB untouched.
51. Restore to fresh empty DB with no app key set and encrypted backup. Refused: needs `--key-file` or `DIMMI_APP__KEY` matching manifest fingerprint.
52. Encrypted backup, app key rotated since (`key:rotate` re-wraps data keys, key id changes). Fingerprint is of key material at backup time; restore requires old key (`--key-file`); after restore, run `key:rotate` again. Docs: keep old keys until all backups encrypted with them expire.
53. Retention prunes while restore reads same archive. Prune skips archives touched in last 1 h (restore marks `system_settings.backups.in_use`).
54. Clock jumps backward (VM restore). Retention buckets use backup `created_at`; duplicates within day tolerated; never deletes last succeeded backup.
55. Backup of huge files store (100 GB local attachments). `include_files` auto true for local disk; doctor suggests S3 + bucket versioning when files over 10 GB; backup still works (streamed), duration logged.

## Containers

56. dimmi container memory limit 256 MB, load spike. GOMEMLIMIT from cgroup keeps Go heap below; OOM still possible: platform restarts container, boot log hint "raise container memory to 512 MB" (templates default 512 MB).
57. SIGKILL from platform without SIGTERM (hard stop, `docker kill`, host reboot). Postgres crash recovery on next boot (WAL); no data loss for committed tx; River jobs in progress re-run (idempotent jobs); `/up` 200 and `/ready` 503 until Postgres accepts connections.

## Bare binary install (install.sh)

58. Box already runs Postgres (unit active, 5432 listener, or `/etc/postgresql/*/main`), no `--database-url`. Preflight refuses before any change, exit 1: "Postgres already on this box. Rerun with --database-url postgres://USER:PASS@127.0.0.1:5432/DB (role owns DB; pgvector installed)". Never installs a second cluster, never edits existing one. Rerun on box the script set up (`/etc/dimmi/install.json` marker): no refusal, reuses `/etc/dimmi/env`.
59. Box already runs Valkey or Redis (unit active or 6379 listener), no `--redis-url`. Preflight refuses, exit 1, hint `--redis-url redis://:PASS@127.0.0.1:6379/0` (dedicated db index or distinct `redis.key_prefix` when shared). Postgres install not started either (all-or-nothing preflight).
60. `--database-url` given, server lacks pgvector (or Postgres under 17). Preflight `dimmi doctor --only postgres.connection,postgres.version,postgres.extensions` fails; exit 1 before installing Valkey or unit; hint per host (Debian: `apt install postgresql-<major>-pgvector`, then `CREATE EXTENSION vector` as superuser; managed: provider extension docs). Unreachable URL: same exit, connect error shown.
61. `--redis-url` points at existing Redis with `allkeys-lru` (or any eviction policy). Preflight prints doctor `valkey.connection` warn (evicted limiter counters, presence, pubsub state), install continues; doctor keeps warning after boot (case 111). Docs: `CONFIG SET maxmemory-policy noeviction` or dedicated Valkey.

## Maintenance mode

62. `dimmi down` on one node, others not yet notified. Valkey pubsub message + 1 s poll of `system_settings`: all nodes enforce within 2 s (poll covers Valkey outage).
63. Maintenance enabled, then database restored from backup taken before maintenance. Restore command re-writes maintenance state after restore (state not lost).
64. Operator forgets `dimmi up`. Hard maintenance persists across restarts (DB + file); boot banner logs "MAINTENANCE MODE ACTIVE since ..."; doctor warn after 1 h.
65. Widget messages sent during maintenance. Widget stores in browser queue and retries (01); soft mode inbound provider webhooks still accepted, nothing lost.
66. Provider webhooks during hard maintenance longer than provider retry window (Slack retries 3 times over ~1 hour). Docs: keep hard maintenance short; Telegram/WhatsApp/SES retry longer. Message loss window documented per provider; SES receipt retries via SNS.
67. Bypass secret guessed. Secret stored hashed, min 16 chars generated by default (`--secret` without value generates); bypass cookie signed, expires 12 h.

## Settings and admin API

68. PATCH sets key also set by env. 422 `setting_locked_by_env` for that key, nothing applied.
69. Mail provider credential rotated (deploy config, restart). In-flight mail jobs on old process finish or retry; new process uses new credentials; auth failures retried by mail job, doctor `mail.system`/`mail.customer` fail until fixed. No runtime edit path (04 final model).
70. Mail test against unsaved settings with wrong port (firewall drops). Connect timeout 10 s, result `failed` stage `connect` with hint "port 25 blocked on many VPS; use 587 or 465".
71. Storage test on S3 with clock skew. Error `RequestTimeTooSkewed` surfaced with clock doctor hint.
72. Switch `filesystems.default` local to s3 with existing local attachments. Old attachments keep `disk=local` and still served (node must keep local data); doc + command `dimmi storage:migrate --from local --to s3` (copies, updates rows in batches) listed as task. Multi-replica with old local files: doctor warns until migrated.
73. Instance admin disables `updates.check` via API but env sets true. Env wins (locked).
74. Very long setting value or binary garbage. Validators per key (length, format); 422.

## Updates

75. GitHub API rate limited (shared NAT IP). 403 with rate headers: skip, retry next day, no alert.
76. Release tagged then deleted (bad release). Next check sees older latest; `update_available` cleared if running version >= latest.
77. Running version newer than latest stable (prerelease/dev build). No notification; `update_available=false`.
78. Release notes contain malicious Markdown/HTML. Rendered with safe goldmark (no raw HTML), stored truncated 2,000 chars.

## Exports

79. Workspace export of 5M messages. Streamed, constant memory; progress visible; resumable on job retry.
80. Contact erased during export. Rows read after erase show tombstone; consistent per row, not snapshot across files (documented); instance export optionally `--snapshot` uses single repeatable-read tx (CLI only, holds snapshot: warns about vacuum delay).
81. Attachment object missing on disk. Skipped, listed in `manifest.json` `missing_attachments`.
82. Owner demoted while export running. Export finishes; download link creation re-checks policy (403 for non-owner).
83. Export download link shared publicly. Signed URL 15 min; each link creation audit logged; archive deleted after 7 days.
84. Two exports requested at once for same workspace. Second returns existing pending/running export (200 with existing resource), not duplicate.

## Proxy, scale, network

85. Proxy strips `Upgrade` header. Dashboard WebSocket fails (centrifuge-js falls back to HTTP-streaming/SSE transport); widget unaffected (uni_sse, plain HTTP); doctor `websocket.timeout`/handshake failure counter warns with nginx snippet.
86. LB idle timeout 60 s. centrifuge server ping every 25 s (`broadcasting.ping_interval`) keeps connection alive.
87. Spoofed `X-Forwarded-For` from direct client (no proxy). Peer not trusted: header ignored; rate limits use real peer IP.
88. Behind Cloudflare without trusted preset. All clients appear as Cloudflare IPs; rate limits per IP too coarse; doctor `proxy.trusted` warns.
89. Second replica added with local disk. Doctor fail; uploads still work per node but cross-node 404: admin API `GET /system/info` shows `warnings: ["local_storage_multi_node"]`.
90. Clock skew between replicas. Business time uses DB `now()` where ordering matters (foundation); doctor reports per-node skew via heartbeat timestamps vs DB time.

## Cloud vs self-host differences

91. `app.edition=cloud`. `/api/v1/system/settings` PATCH 403 `managed_by_cloud`; backups API 404 `feature_unavailable` (RDS PITR); instance export 404; workspace export available; setup flow disabled (signup via 00-accounts); update check off.
92. Self-host operator sets `app.edition=cloud` by mistake. Boot requires cloud-only config (Stripe keys, KMS); missing: boot fails listing required keys. Prevents accidental billing gates on self-host.
93. Self-host never shows billing UI/API: `/api/v1/billing/*` returns 404 `feature_unavailable`.

## Abuse and security

94. Setup endpoint brute force. `throttle:setup` 30/min/IP; token 256-bit, infeasible.
95. Instance admin token stolen. `system:*` abilities only on tokens minted by instance admins; token list shows them flagged; revoke via 00-accounts.
96. Backup download URL logged by proxy access log. URL expires 15 min; docs recommend excluding query strings from proxy logs.
97. Release artifact tampered on mirror. Install script compares checksum with GitHub `checksums.txt` (signed); mismatch aborts.

## Extensibility seams (`00-foundation/extensibility.md`)

98. `GET /api/v1/workspace/entitlements` on self-host. All features true; every cloud cap limit `-1` (custom Discord bots, email daily cap, help centers); dashboard shows unlimited.
99. Operator sets `DIMMI_ENTITLEMENTS__LIMITS__CHANNELS__DISCORD__CUSTOM_BOTS=2`. Limit 2 for every workspace on instance; 3rd custom bot 422 `entitlement_limit_reached`. Typo key (`..._BOTZ`): boot fails with closest-match hint.
100. `webhooks.max_endpoints=50` in dimmi.yaml, no `entitlements.limits` entry. self_host driver returns 50 (catalog maps key to spec config key).
101. `PATCH /api/v1/system/settings` with workspace-scope key (`security.require_two_factor`). 422 `unknown_setting` (not instance scope; set per workspace via workspace settings API).
102. New spec registers instance key in foundation settings registry. Appears in `GET /system/settings` with `source`; no 11 code change.
103. `GET /api/v1/auth/config` on self-host. `region: "self"`, `server_version: "1.4.2"`, `api_version: "2026-11-01"`. Newer mobile app sees missing `capabilities` token and hides feature instead of failing.
104. `dimmi restore` while `RetentionGuard` (later driver) holds a workspace in current DB. Refuses before `pg_restore`, lists held workspaces, exit 1, DB untouched. MVP noop guard: never refuses.
105. `PruneBackups` and `PruneDataExports` with held workspace (later driver). Not guarded: archives are derived copies; held source rows stay in DB and in newer backups. Documented in decision 17.
106. Workspace owner exports, then demoted to admin. Next `POST /exports/{id}/download-links` 403: `exports:write` permission (owner only in `roles.go`), checked via `authz.Can`, never role name.

## Valkey, realtime, mail (final sweep 2026-11-02)
107. Valkey unreachable at boot. Retry `redis.connect_retry` (60 s), then exit 1 with message linking upgrade note (operations.md section 3). Compose `depends_on: service_healthy` avoids race.
108. Valkey restarts while running (OOM, upgrade). `/ready` 503 until reconnect; API keeps serving reads from Postgres; Valkey-down policy (data-model.md): auth and abuse guards fail closed (503 + `Retry-After: 5`: login/lockout, magic link/email code, portal sign-in, anonymous votes, widget PoW, JWT nonce, provider webhook replay), outbound provider limiters defer (job snooze), everything else fails open with warning + `dimmi_valkey_fallback_total`; realtime clients reconnect, `recovered: false` (history lost unless AOF replayed) and refetch; caches refill. No data loss.
109. Valkey AOF file corrupt after crash. Valkey refuses start; doctor/docs: delete `valkey-data` volume (rebuildable state), restart. install.sh box: doctor fix text names `/var/lib/valkey` AOF dir to delete.
110. Operator upgrades to Valkey-required release without adding Valkey. Boot fails (107) with upgrade note; previous version still runs if rolled back (no schema dependency on Valkey).
111. Operator points `redis.url` at shared Redis with `allkeys-lru`. Doctor `valkey.connection` warns (evicted rate-limit counters, presence and pubsub state); works but degraded; docs require `noeviction`.
112. Two Dimmi instances share one Valkey db. Distinct `redis.key_prefix` required; same prefix: node registries and pubsub topics collide; doctor `valkey.shared` warns, boot logs other instance ids seen.
113. Restore backup onto box with old Valkey data. Docs: flush Valkey db before `dimmi up`; stale caches otherwise expire within TTLs (max 30 s guard cache, 24 h unrelated caches keyed by schema version are bumped on restore). `dimmi restore` deletes keys under `redis.key_prefix` (SCAN + `UNLINK` on every primary; never `FLUSHDB`) unless `--no-flush-valkey`.
114. $5 VPS with 512 MB RAM. Stack idle about 300 MB; doctor `memory.limit` warns under 768 MB; Valkey `maxmemory 64mb` with `noeviction` returns OOM errors to writes when full: cache writes skipped, limiter writes follow Valkey-down policy (auth/abuse guards 503, provider limiters defer, rest fail open), metric + doctor fail.
115. Customer-stream mailer left on `log` in production. Boot fails (04 rule); self-host dev (`app.env=local`) allowed with WARN.
116. SES driver with only one configuration set created. Doctor `mail.configuration_sets` fail naming missing `dimmi-system` or `dimmi-customer`; sends using missing set fail permanent; doctor fix text gives `aws sesv2 create-configuration-set` command per missing set.
117. Operator expects SMTP inbound on port 25. Not supported (04 Never list); doctor `mail.inbound` explains MVP option (SES receipt on own AWS); IMAP and other inbound adapters later (`imap-adapter`, `extra-mail-adapters`).
118. Fly template on Upstash Redis (`fly redis create`) rejects a command centrifuge broker or presence uses (Lua script, streams) [verify]. Boot broker probe fails, `/ready` 503, doctor `broadcasting.broker` fail names command and links Fly README fallback (Valkey as own Fly app on private network). Upstash eviction enabled: doctor `valkey.connection` warns like case 111.

## Launch extensions: agent actions (07), docs sync (08), importers (16)

119. Box behind NAT (`app.url` resolves to 192.168.x) with docs sync source. Doctor `github.app_url_public` warns; repos use polling (`docs_sync.poll_interval` 15 min) + 6 h reconcile automatically; no failing-webhook alerts for poll-mode repos.
120. Agent action connection `base_url` = `http://billing.internal:8080`. Blocked by SSRF guard until host in `http_client.allow_hosts` (runtime setting, audited); `http://` allowed only on self-host allowlisted hosts; doctor `ai.actions.private_hosts` warns per connection until then.
121. Operator sets `ai.actions.enabled=false` with published actions. Actions not offered to agent (eligibility), dashboard shows instance-disabled banner; no data deleted.
122. Large Intercom import (40 GB attachments) on local disk VPS. Import runs (files via `filesystems.default`); doctor `postgres.disk` / storage warnings apply; docs recommend S3 disk + `dimmi backup` first; imports queue needs a `worker` role node (doctor `imports.queue`).

## Backing-service topologies (decision 20)

123. `database.url` multi-host `h1,h2,h3?target_session_attrs=read-write`; h1 fails, h2 promoted. pgx reaches h2 on next acquire; in-flight requests 503 `service_unavailable` + `Retry-After: 1`; River attempts retry; no request served by a standby.
124. Pooled connection survives on demoted old primary (now read-only). Write gets SQLSTATE `25006`; pool reset, request 503 retryable; next acquire through writer endpoint reaches new primary.
125. Connection drops after COMMIT sent, before reply. Outcome unknown: no automatic retry; request 503; client retry with same `Idempotency-Key` returns stored result if commit landed, else runs once.
126. `database.url` points at a hot standby or reader endpoint. Boot `pg_is_in_recovery()` true: exit 1 "database.url points at a standby; use the writer endpoint".
127. `database.url` behind PgBouncer transaction mode, `database.pool_mode` left `session`. Named prepared statement errors (`prepared statement ... does not exist`) mapped to ERROR log with hint "set database.pool_mode=transaction"; doctor `postgres.pooler` fail.
128. Transaction pooler without `database.direct_url`, pending migrations at boot. Auto-migrate refused (session advisory lock needs a session): exit 1 naming `database.direct_url` and `dimmi migrate --database-url`; River poll-only (doctor warns: job pickup about 1 s slower); backups refused.
129. Transaction pooler with `direct_url`. Migrations, River notifier and pg_dump use direct; app queries use pooler; pool math doctor check sums both.
130. Read replica lag 45 s, `read_max_lag` 10 s. `db.Reader` uses primary; metric `dimmi_db_reader_fallback_total{reason="lag"}`; no read staler than limit.
131. All read replicas down. `db.Reader` falls back to primary; `/ready` stays 200 (replicas optional); doctor `postgres.replicas` warns.
132. Request writes then calls `db.Reader`. Write marker on request ctx routes read to primary (read-your-writes within request).
133. Operator points Dimmi at Citus or multi-primary Postgres. Unsupported (decision 20); doctor `postgres.topology` fails when `citus` extension present; docs state single writer.
134. Sentinel failover under load. rueidis follows new master within Sentinel detection window; writes in window follow Valkey-down policy (data-model.md); last async writes lost (counters, once-keys, presence) like a restart; durable dedupe in Postgres covers message and money paths.
135. ElastiCache cluster mode configuration endpoint with TLS + auth token. `rediss://` + `redis.mode=auto` detects cluster, discovers nodes; centrifuge broker built in cluster mode from same config; doctor `valkey.topology` reports `cluster`, primaries count.
136. Cluster mode with `redis.url` db index `/2`. Boot fails "logical databases unsupported in cluster mode; remove /2".
137. Cluster resharding during load. Client follows `MOVED`/`ASK`; multi-key commands share one hash tag, so no `CROSSSLOT`; `TRYAGAIN` retried by client [verify rueidis], else Valkey-down policy for that call.
138. New code adds multi-key command without shared hash tag. Nightly topology suite on 3-primary cluster fails with `CROSSSLOT`; keys built only via `framework/redis` `Key`, so the tag is never forgotten by hand.
139. Node listing on cluster. Uses `dimmi:nodes` sorted-set index, never `SCAN` (would cover one node only).
140. `dimmi restore` with Valkey shared by another Dimmi instance (different `redis.key_prefix`). Deletes only own prefix on every primary; other instance untouched.

