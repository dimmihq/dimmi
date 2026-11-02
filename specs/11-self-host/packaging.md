# 11 Self-host: packaging

Style: caveman. Every artifact Dimmi ships and how built. Paths relative to repo root.

## 1. Release pipeline (goreleaser)

Trigger: git tag `v*` on `main`. GitHub Actions, `permissions: id-token: write, contents: write, packages: write, attestations: write`.

| Step | Detail |
|---|---|
| Frontend build | `pnpm build` (dashboard + widget) before Go build; output embedded via `go:embed` (`public/`) |
| Go build | `CGO_ENABLED=0`, `-trimpath`, `-ldflags "-s -w -X internal/version.Version={{.Version}} -X .Commit={{.Commit}} -X .Date={{.Date}}"`, `-tags timetzdata` (embed tz database: no OS tzdata dependency on bare hosts or slim image; workspace timezones must work). Never `-tags dev`: dev-only commands (`make:*`, `route:lint`, `lang:check`) stay out of release binaries. OTLP tracing: release registers `otlp.Tracing()` only if PO accepts gRPC size cost [verify]; framework follow-up (own OTLP HTTP exporter) removes the cost. Without it `telemetry.tracing=otlp` fails boot (unknown driver) |
| Targets | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64 |
| Archives | `dimmi_<v>_<os>_<arch>.tar.gz` (zip on windows), include LICENSE, README, `dimmi.example.yaml` |
| Checksums | `checksums.txt` (SHA-256) |
| SBOM | syft via goreleaser `sboms:` SPDX JSON per archive and per image |
| Signing | cosign keyless (Sigstore, GitHub OIDC): `checksums.txt.sig` + `.pem`; images signed by digest |
| Provenance | `actions/attest-build-provenance` for archives and images (SLSA v1) |
| Images | `docker buildx` multi-arch (linux/amd64, linux/arm64) pushed to `ghcr.io/dimmihq/dimmi` |
| Tags | `<v>`, `<major>.<minor>`, `<major>`, `latest` (stable only). One image, no variant suffixes. Prerelease tags (`-rc.N`) never move `latest` |
| Changelog | from conventional commits, sections Features, Fixes, Migrations (lists every migration + "expand" or "contract"), Breaking |
| Install script | `deploy/install/install.sh` uploaded as release asset and served at `https://trydimmi.com/install.sh` (same bytes, checksum in release) |
| Budget gate | job fails when compressed image over 80 MB per arch [verify after first build: Debian slim ~30 MB + `postgresql-client-18` ~15 MB + binary ~12 MB gzipped] or binary over 50 MB uncompressed (realistic app build 2026-11-02: 36.0 MB stripped, 12.3 MB gzipped, linux/amd64). `tests/arch` binary test fails CI when any `<ReflectMethod>` marker appears in the linker dependency dump (lost dead method elimination roughly doubles size) |

Windows binary: full server works (no systemd); main use CLI, `dimmi mcp` stdio.

Mirror: release assets also pushed to `https://get.trydimmi.com/<v>/` (platform risk lesson from PocketBase, `tech-stack.md`). Install script tries GitHub first, mirror second, verifies same checksum.

## 2. Image

One image: `ghcr.io/dimmihq/dimmi:<v>`. Used by compose, every one-click template, any container platform. No variants, no suffix tags (PO 2026-11-02): one Dockerfile, one tag set.

```dockerfile
# deploy/docker/Dockerfile
FROM --platform=$BUILDPLATFORM golang:1.26 AS build   # digest pinned
# ... build as above, output /out/dimmi
FROM debian:trixie-slim                                # digest pinned
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl gnupg \
 && install -d /usr/share/postgresql-common/pgdg \
 && curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc \
 && echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt trixie-pgdg main" > /etc/apt/sources.list.d/pgdg.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends postgresql-client-18 \
 && apt-get purge -y curl gnupg && apt-get autoremove -y && rm -rf /var/lib/apt/lists/* \
 && useradd --system --uid 65532 --user-group --no-create-home --shell /usr/sbin/nologin dimmi \
 && install -d -o 65532 -g 65532 -m 0700 /var/lib/dimmi
COPY --from=build /out/dimmi /usr/local/bin/dimmi
ENV DIMMI_APP__DATA_DIR=/var/lib/dimmi DIMMI_PLATFORM=docker DIMMI_APP__KEY_AUTO_GENERATE=true
VOLUME ["/var/lib/dimmi"]
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=20s CMD ["/usr/local/bin/dimmi","healthcheck"]
ENTRYPOINT ["/usr/local/bin/dimmi"]
CMD ["serve"]
```
- `pg_dump`, `pg_restore`, `psql` 18 from apt.postgresql.org (works against server 17 and 18; pg_dump must be >= server major). Backups work in every container deploy.
- CA certificates from Debian. tzdata: embedded in binary.
- Nonroot uid 65532. Read-only rootfs supported: writes only to `/var/lib/dimmi` volume (app key file, local disk uploads, local backups) and `/tmp` (tmpfs in compose).
- Size target under 80 MB compressed per arch [verify]. Base digest pinned, Renovate bumps; Grype scan gates release (operations.md section 12).

## 3. docker-compose.yml

`deploy/compose/docker-compose.yml`, also copied to repo root `docker-compose.yml`. Download: `curl -fsSLO https://trydimmi.com/compose.yml`.

```yaml
name: dimmi

x-dimmi-env: &dimmi-env
  DIMMI_DATABASE__URL: postgres://dimmi:${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD in .env}@postgres:5432/dimmi?sslmode=disable
  DIMMI_REDIS__URL: redis://:${VALKEY_PASSWORD:?set VALKEY_PASSWORD in .env}@valkey:6379/0
  DIMMI_APP__URL: ${DIMMI_APP__URL:-http://localhost:8080}
  DIMMI_PLATFORM: compose
  DIMMI_HTTP__TRUSTED_PROXIES: ${DIMMI_HTTP__TRUSTED_PROXIES:-private}

services:
  dimmi:
    image: ghcr.io/dimmihq/dimmi:${DIMMI_VERSION:-1}
    restart: unless-stopped
    environment: *dimmi-env
    env_file: [{ path: .env, required: false }]
    ports: ["${DIMMI_PORT:-8080}:8080"]
    volumes: [dimmi-data:/var/lib/dimmi]
    depends_on:
      postgres: { condition: service_healthy }
      valkey: { condition: service_healthy }
    read_only: true
    tmpfs: [/tmp]
    security_opt: ["no-new-privileges:true"]
    cap_drop: [ALL]
    stop_grace_period: 30s

  postgres:
    image: pgvector/pgvector:pg18                     # official postgres:18 lacks pgvector; release copy pins 0.8.x-pg18-trixie + digest
    restart: unless-stopped
    environment:
      POSTGRES_USER: dimmi
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set POSTGRES_PASSWORD in .env}
      POSTGRES_DB: dimmi
      POSTGRES_INITDB_ARGS: "--auth-host=scram-sha-256 --data-checksums"
    command: ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]
    volumes:
      - pg-data:/var/lib/postgresql                    # PG18 image: mount parent dir, PGDATA=/var/lib/postgresql/18/docker
      - ./postgresql.small.conf:/etc/postgresql/postgresql.conf:ro
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U dimmi -d dimmi"]
      interval: 5s
      timeout: 3s
      retries: 20
    shm_size: 128mb
    stop_grace_period: 60s
    # no ports: never exposed on host

  valkey:
    image: valkey/valkey:8-alpine                     # digest pinned in release copy; rebuildable state only, never backed up
    restart: unless-stopped
    command: ["valkey-server", "--requirepass", "${VALKEY_PASSWORD}", "--appendonly", "yes", "--appendfsync", "everysec", "--save", "", "--maxmemory", "64mb", "--maxmemory-policy", "noeviction", "--protected-mode", "yes"]
    volumes: [valkey-data:/data]                      # AOF keeps presence/history across Valkey restart only
    healthcheck:
      test: ["CMD-SHELL", "valkey-cli -a \"$${VALKEY_PASSWORD}\" --no-auth-warning ping | grep PONG"]
      interval: 5s
      timeout: 3s
      retries: 20
    environment: { VALKEY_PASSWORD: "${VALKEY_PASSWORD}" }
    read_only: true
    security_opt: ["no-new-privileges:true"]
    cap_drop: [ALL]
    # no ports: never exposed on host

  caddy:
    profiles: [tls]
    image: caddy:2-alpine                              # digest pinned
    restart: unless-stopped
    ports: ["80:80", "443:443", "443:443/udp"]
    environment:
      DIMMI_DOMAIN: ${DIMMI_DOMAIN:?set DIMMI_DOMAIN for tls profile}
      ACME_EMAIL: ${ACME_EMAIL:-}
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    depends_on: [dimmi]

volumes:
  dimmi-data:
  pg-data:
  valkey-data:
  caddy-data:
  caddy-config:
```

`.env.example`:
```
POSTGRES_PASSWORD=          # install helper fills: openssl rand -hex 24
VALKEY_PASSWORD=            # install helper fills: openssl rand -hex 24
DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=log   # postmark | resend | mailgun | sendgrid | ses | smtp (outbound relay); customer stream inherits; see operations.md mail keys
DIMMI_MAIL__MAILERS__SYSTEM__TOKEN=       # postmark token (resend/mailgun/sendgrid: ..._API_KEY); or _FILE
DIMMI_APP__URL=https://support.example.com
DIMMI_DOMAIN=support.example.com   # only with --profile tls
ACME_EMAIL=you@example.com
DIMMI_VERSION=1
```

With TLS profile: `docker compose --profile tls up -d`; dimmi port mapping should be removed or bound `127.0.0.1:8080:8080` (documented; `compose.tls.yml` override provided).

`Caddyfile`:
```
{$DIMMI_DOMAIN} {
  encode zstd gzip
  reverse_proxy dimmi:8080 {
    flush_interval -1
    transport http {
      read_timeout 0
      write_timeout 0
    }
  }
}
# help center custom domains: on-demand TLS gated by Dimmi ask endpoint (08 owns domain verification)
https:// {
  tls { on_demand }
  reverse_proxy dimmi:8080
}
```
Global option `on_demand_tls { ask http://dimmi:9090/internal/tls/ask }` (admin port, never public port 8080; endpoint owned by 08-help-center custom domains: `GET /internal/tls/ask?domain=` returns 200 only for ownership-verified `help_center_domains`, else 404). TLS profile sets `DIMMI_HTTP__ADMIN_ADDR=0.0.0.0:9090` on dimmi service with no `ports:` entry (reachable only on compose network); doctor `security.admin_port` treats compose-network-only bind as ok. Caddy streams WebSockets natively.

### 3.1 Postgres small-memory tuning

`deploy/docker/postgres/postgresql.small.conf` (compose; install.sh copies it as `conf.d/dimmi.conf` minus `listen_addresses`). Target: 1 GB VPS shared with dimmi + caddy, 256 MB idle total.

```
listen_addresses = '*'                 # compose internal network only (never published); install.sh omits (Debian default localhost)
max_connections = 40                   # dimmi pool 10 + River notifier + backups + admin headroom (realtime/pubsub on Valkey, no LISTEN broker conns)
superuser_reserved_connections = 3
shared_buffers = 64MB
effective_cache_size = 256MB
work_mem = 4MB
maintenance_work_mem = 32MB
autovacuum_work_mem = 16MB
wal_buffers = 4MB
min_wal_size = 64MB
max_wal_size = 512MB
checkpoint_completion_target = 0.9
random_page_cost = 1.1
effective_io_concurrency = 100
io_method = worker
io_workers = 2
max_worker_processes = 4
max_parallel_workers = 2
max_parallel_workers_per_gather = 1
max_parallel_maintenance_workers = 1
autovacuum_max_workers = 2
jit = off                              # saves memory, no benefit for OLTP
huge_pages = off
password_encryption = scram-sha-256
shared_preload_libraries = ''
log_min_duration_statement = 1000
log_checkpoints = on
log_lock_waits = on
idle_in_transaction_session_timeout = 60s
statement_timeout = 0                  # dimmi sets per-request timeouts
```
Doc table for bigger boxes: 2 GB (`shared_buffers=256MB, effective_cache_size=1GB, work_mem=8MB`), 4 GB+ (`shared_buffers=1GB, effective_cache_size=3GB, work_mem=16MB, max_connections=100`). Doctor warns when `shared_buffers` under 1/16 of detected memory over 2 GB.

## 4. One-click templates

Six templates: Railway, Render, Fly, DigitalOcean, Coolify, Dokploy. Same shape everywhere: image `ghcr.io/dimmihq/dimmi:<major>` + platform managed Postgres (pgvector) + platform managed Valkey/Redis. Coolify and Dokploy have no managed services: they run the 3-service compose (dimmi, `pgvector/pgvector:pg18`, `valkey/valkey:8-alpine`) from section 3.

All templates: set `DIMMI_PLATFORM`, wire `DATABASE_URL` and `REDIS_URL`/`DIMMI_REDIS__URL` from platform service references, honor platform `PORT` (aliases are a framework follow-up; until built, templates map them to `DIMMI_DATABASE__URL`, `DIMMI_REDIS__URL`, `DIMMI_HTTP__ADDR`), set `DIMMI_APP__URL` from platform domain variable where supported (else setup form asks `app_url`, stored runtime setting). App key: env secret generated by platform when supported (`DIMMI_APP__KEY`), else auto-generate to volume. Managed Redis must run `noeviction` (doctor `valkey.connection` warns otherwise).

| Platform | File | Shape | Storage | Notes |
|---|---|---|---|---|
| Railway | `deploy/railway/railway.json` + published template (dashboard-defined) | service `dimmi` (image `dimmi:1`) + Railway Postgres with pgvector (Railway pgvector template image) + Railway Redis/Valkey service | Railway volume at `/var/lib/dimmi` | `DATABASE_URL=${{Postgres.DATABASE_URL}}`, `REDIS_URL=${{Redis.REDIS_URL}}`, `DIMMI_APP__URL=https://${{RAILWAY_PUBLIC_DOMAIN}}`, `DIMMI_APP__KEY=${{secret(64)}}`. Healthcheck path `/ready`. Replicas 1 unless S3 env set |
| Render | `render.yaml` (Blueprint) at repo root | `services: - type: web, runtime: image, image.url: ghcr.io/dimmihq/dimmi:1, healthCheckPath: /ready, disk: {name: dimmi-data, mountPath: /var/lib/dimmi, sizeGB: 5}` + `databases: - name: dimmi-db` + Render Key Value (Valkey) `type: keyvalue, maxmemoryPolicy: noeviction`, `DIMMI_REDIS__URL` from `fromService.connectionString` | Render disk (paid instance types only) | Plan `starter` minimum: free web instances spin down and kill WebSockets. `DATABASE_URL` from `fromDatabase.connectionString`; `DIMMI_APP__KEY` `generateValue: true`. Render Postgres supports `vector` extension. Disk means single instance (Render rule) |
| Fly.io | `deploy/fly/fly.toml` + `deploy/fly/README.md` launch steps | `image = "ghcr.io/dimmihq/dimmi:1"`, `[[vm]] memory="512mb"`, `[http_service] internal_port=8080, force_https=true, auto_stop_machines="off", min_machines_running=1`, `[checks]` HTTP `/ready`. Postgres: Fly Managed Postgres (`fly mpg create`, attach sets `DATABASE_URL`) [verify: `vector` extension available on Fly MPG]. Valkey: Upstash Redis via Fly (`fly redis create`, URL set as `REDIS_URL` secret, eviction off) [verify: centrifuge Redis broker + presence work on Upstash: pub/sub, Lua `EVAL`/`EVALSHA`, streams if history uses them; `CONFIG GET maxmemory-policy` readable for doctor] | `[mounts] source="dimmi_data" destination="/var/lib/dimmi"` only when `filesystems.default=local`; Tigris/S3 env: no volume, more machines allowed | Auto-stop off (WebSockets, scheduler). Upstash unsupported command: doctor `broadcasting.broker` fails at boot check, README fallback = Valkey as own Fly app (`valkey/valkey:8-alpine` + volume, private `.flycast` address) |
| DigitalOcean App Platform | `.do/deploy.template.yaml` (Deploy to DO button) | service image `dimmi:1`, `databases: - engine: PG, version: "18"` (dev or managed cluster) + `- engine: VALKEY` (DO managed Valkey), health check `/ready` | **no persistent disk on App Platform**: template requires Spaces (S3) env vars `DIMMI_FILESYSTEMS__DEFAULT=s3`, bucket, key, secret, endpoint | Boot with `DIMMI_PLATFORM=digitalocean` and local disk: refuse uploads with 503 `storage_not_configured` + doctor fail. Backups: use DO managed DB backups; `backups.enabled=false` default for this platform |
| Coolify | `deploy/coolify/dimmi.yaml` (PR to `coollabsio/coolify` `templates/compose/`) | compose: dimmi `dimmi:1` + `pgvector/pgvector:pg18` + `valkey/valkey:8-alpine` | named volumes | `VALKEY_PASSWORD=${SERVICE_PASSWORD_VALKEY}`; header comments (`# documentation:`, `# slogan:`, `# tags:`, `# port: 8080`). `SERVICE_FQDN_DIMMI_8080` for domain + `DIMMI_APP__URL=${SERVICE_FQDN_DIMMI}`; `POSTGRES_PASSWORD=${SERVICE_PASSWORD_POSTGRES}`; `DIMMI_APP__KEY=${SERVICE_PASSWORD_64_APPKEY}` |
| Dokploy | `deploy/dokploy/blueprints/dimmi/` (PR to `Dokploy/templates`) | `docker-compose.yml` (dimmi, postgres, valkey; same 3 services; no host ports, no `container_name`) + `template.toml` | named volumes | `[variables] main_domain="${domain}", pg_password="${password:32}", valkey_password="${password:32}", app_key="${base64:64}"`; `[[config.domains]] serviceName="dimmi" port=8080 host="${main_domain}"`; env writes `DIMMI_APP__URL=https://${main_domain}` |

Template facts above verified against provider docs 2026-11-01 where public (Render blueprint, Coolify magic vars, Dokploy blueprint layout); Railway template is dashboard-defined (only `railway.json` config-as-code lives in repo); Fly managed-service rows marked `[verify]`. CI job `templates-lint` validates `render.yaml` against Render JSON schema, `fly.toml` via `flyctl config validate` (offline), DO spec via `doctl apps spec validate` (offline mode), compose files via `docker compose config`.

Each template README states: memory minimum 512 MB for app container (managed Postgres and Valkey sized separately), Valkey required (no template without it), single replica unless S3 configured, how to get setup link (platform log viewer).

## 5. Bare metal: install script + systemd

Two non-Docker paths: `install.sh` (default) or manual binary run.

### install.sh
Bash script. `curl -fsSL https://trydimmi.com/install.sh | sudo bash` installs Dimmi + Postgres 18 (pgvector) + Valkey 8 on one Debian/Ubuntu box. Box already runs them: `sudo bash -s -- --database-url postgres://... --redis-url redis://...` (either flag alone skips only that install). `--domain support.example.com` adds Caddy TLS.

| Step | Behavior |
|---|---|
| Detect | OS linux only (darwin prints manual instructions; windows refuses), arch amd64/arm64, systemd required (else print manual run instructions, section "Manual"). Default path needs apt: Debian 12/13, Ubuntu 24.04/26.04. Other distros: allowed only with both `--database-url` and `--redis-url` (binary + unit only); else exit 1 before any change, print manual steps |
| Preflight (no changes yet) | Without `--database-url`: existing Postgres detected (`postgresql` unit active, listener on 5432 via `ss -ltn`, or `/etc/postgresql/*/main`) refuses, exit 1 (edge 58). Without `--redis-url`: existing Valkey/Redis (`valkey-server`/`redis-server` unit active or listener on 6379) refuses, exit 1 (edge 59). Rerun on box this script set up: `/etc/dimmi/install.json` marker (`{installed_postgres, installed_valkey, version}`) + `/etc/dimmi/env` URLs reused, no refusal. `--database-url` given: downloaded binary runs `dimmi doctor --only postgres.connection,postgres.version,postgres.extensions --skip-network`; fail exits 1 with hint (edge 60). `--redis-url` given: `--only valkey.connection`; warn printed, install continues (edge 61) |
| Download | latest release (or `--version`) archive + `checksums.txt` from GitHub, mirror fallback |
| Verify | SHA-256 always; cosign signature when `cosign` on PATH (`--require-signature` makes mandatory) |
| Install | `/usr/local/bin/dimmi` 0755; user `dimmi` (system, no shell, home `/var/lib/dimmi`); `/etc/dimmi/dimmi.yaml` 0640 root:dimmi (only if absent, never overwrite); `/var/lib/dimmi` 0700 dimmi |
| pg_dump (always on apt hosts) | add apt.postgresql.org repo (signed key), install `postgresql-client-18` even with `--database-url` (backups need pg_dump >= server major) |
| Postgres (default; skipped with `--database-url`) | install `postgresql-18 postgresql-18-pgvector`, create role + db with random 24-byte password, `CREATE EXTENSION IF NOT EXISTS vector, pg_trgm, unaccent, citext` as superuser, write `DATABASE_URL` (localhost) to `/etc/dimmi/env` 0640 root:dimmi, apply small-memory conf (section 3.1) as `conf.d/dimmi.conf`, listen localhost only |
| Valkey (default; skipped with `--redis-url`) | install Valkey 8 (`valkey-server` distro package where 8.x, else upstream apt repo [verify package source per supported distro]), write `/etc/valkey/valkey.conf` (`bind 127.0.0.1 -::1`, `requirepass` random 24 bytes, `maxmemory 64mb`, `maxmemory-policy noeviction`, `appendonly yes`, `appendfsync everysec`, `save ""`), write `DIMMI_REDIS__URL` to `/etc/dimmi/env`. `--redis-url URL`: written as given |
| Caddy (`--domain`) | install Caddy from official apt repo, write site block reverse proxy to 127.0.0.1:8080 |
| App key | bare binary never auto-generates: script runs `dimmi key:generate --write /var/lib/dimmi/app.key` (0600, owner dimmi) when absent, never overwrites |
| Service | install `dimmi.service`, `systemctl enable --now dimmi`; waits for local `postgresql` and `valkey-server` units first when installed |
| Output | prints URL and setup link (reads `journalctl -u dimmi` for setup line, waits up to 30 s) |
| Idempotent | rerun upgrades binary (keeps config and env), adds Valkey when missing on pre-Valkey install (upgrade note, operations.md section 3), restarts service; `--uninstall` removes binary + unit, never data, never Postgres/Valkey packages (prints paths and commands to remove manually) |
| Non-interactive | no prompts; all via flags/env (`DIMMI_INSTALL_*`): `--version`, `--database-url`, `--redis-url`, `--domain`, `--require-signature`, `--uninstall` |

### Manual (no script, no Docker)
Download archive from release, verify `checksums.txt`, then: `DIMMI_DATABASE__URL=postgres://... DIMMI_REDIS__URL=redis://... DIMMI_APP__URL=https://... dimmi serve --generate-key`. Operator supplies Postgres 17+ with pgvector and Valkey 8 (`noeviction`). Docs page "Run the binary" covers it plus the unit below.

### dimmi.service
```ini
[Unit]
Description=Dimmi support platform
Documentation=https://trydimmi.com/docs/self-host
After=network-online.target postgresql.service valkey-server.service   # ordering only; absent units ignored
Wants=network-online.target

[Service]
Type=notify                       # dimmi sends sd_notify READY=1 after /ready would pass, WATCHDOG=1 every 10 s
User=dimmi
Group=dimmi
EnvironmentFile=-/etc/dimmi/env
Environment=DIMMI_CONFIG=/etc/dimmi/dimmi.yaml DIMMI_APP__DATA_DIR=/var/lib/dimmi DIMMI_PLATFORM=systemd
ExecStart=/usr/local/bin/dimmi serve
ExecReload=/bin/kill -HUP $MAINPID
Restart=on-failure
RestartSec=2
TimeoutStartSec=15min             # long migrations on upgrade
TimeoutStopSec=35s
WatchdogSec=30s
LimitNOFILE=65536                 # WebSockets
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallArchitectures=native
CapabilityBoundingSet=
AmbientCapabilities=
ReadWritePaths=/var/lib/dimmi
StateDirectory=dimmi
StateDirectoryMode=0700

[Install]
WantedBy=multi-user.target
```
Binding port under 1024 not supported in unit (use Caddy). sd_notify via `github.com/coreos/go-systemd/v22/daemon` (pure Go): `SdNotify(READY=1)` after `/ready` passes, `WATCHDOG=1` heartbeat; no-op when `$NOTIFY_SOCKET` unset.
