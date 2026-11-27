# Contributing to dimmi

This guide walks AI agents and human contributors from a fresh clone to a running app with seeded data, local backing services, and fast targeted checks.

Read [AGENTS.md](AGENTS.md) before changing code. It holds the binding rules for API and SDK compatibility, generated files, copywriting, commits, and pull requests.

## Prerequisites

Install these tools before running setup:

- **Go 1.26.0** (`go.mod` selects the toolchain when Go downloads toolchains on demand)
- **Node.js 22+** (CI runs Node 24) and **pnpm 11.22.0**
- **Docker** with Docker Compose (for local Postgres, Valkey, S3, and Mailpit)
- **PHP 8.2+, Composer, Python 3.10+, and uv** (only needed if you run `make sdk-check` or `make sdk-compat`)

## Quick setup

Run these five commands from the repository root:

```bash
make tools        # installs golangci-lint, sqlc, air into ./bin, pnpm deps, Playwright Chromium
make services     # starts Postgres (pgvector), Valkey, SeaweedFS S3, and Mailpit
make sdk-generate # generates gitignored SDK layers from tests/contracts/api_v1.openapi.json
make seed         # creates the dev DB, runs migrations, and seeds Stowline and Lumen
make dev          # starts Go (:8080), Vite (:5173), and the messenger watcher
```

Open `http://localhost:8080/d` once `make dev` is running. Pass `TAGS=cloud` to run the cloud edition locally (`make dev TAGS=cloud`).

## Local services and configuration

`compose.yaml` binds every backing service to offset localhost ports so they do not clash with other projects:

| Service | Address | Purpose |
| --- | --- | --- |
| Postgres (`pgvector:pg18`) | `127.0.0.1:15432` | Dev database `dimmi` (`postgres` / `postgres`) |
| Valkey 8 | `127.0.0.1:16379` | Cache, rate limits, pub/sub, realtime |
| SeaweedFS S3 | `127.0.0.1:18333` | Local object storage |
| Mailpit | `127.0.0.1:11025` (SMTP), `http://localhost:18025` (UI) | Captures all local outbound email |

`make dev` copies `dimmi.example.yaml` to `dimmi.yaml` (gitignored) on first run. That file already points at the local Docker Compose services, turns on automatic migrations, and generates `./storage/app.key` on first boot.

Override any setting from `dimmi.example.yaml` with a `DIMMI_` environment variable using `__` between nested keys (for example, `DIMMI_LOG__DRIVER=text`). Secret fields also accept a `_FILE` suffix pointing to a file on disk.

Tests read their service endpoints from `dimmi.test.yaml` (or the file named in `TEST_CONFIG`).

## Seed data

`make seed` ensures the `dimmi` database exists, runs pending migrations, and executes `go run -tags dev ./cmd/dimmi db:seed $(SEED_ARGS)`.

By default, `db:seed` creates two development workspaces. Both use the password `stowline-demo-2026` unless you pass `--password`:

- **Stowline (`/w/stowline`, `maya@stowline.test`):** a shipping software company with 440+ help center articles, 90 days of help center and changelog analytics, a full support team, companies, customers, 1 year of multi-channel conversations, AI agents and past runs, workflows, broadcasts, series, tickets, SLAs, trackers, and 365 days of backlog snapshots.
- **Lumen (`/w/lumen`, `maya@lumen.test`):** a curated workspace with 6 teammates, 12 conversations, billing actions, versioned refund procedures with saved simulations, and a triage workflow used for demos and screenshots.

### Choosing size and workspace

Use `--size` to control Stowline's volume:

| `--size` | Conversations | Contacts | Companies | Teammates | Tickets |
| --- | ---: | ---: | ---: | ---: | ---: |
| `small` (default) | 1,500 | 900 | 120 | 10 | 40 |
| `medium` | 12,000 | 5,000 | 600 | 22 | 160 |
| `large` | 60,000 | 25,000 | 2,500 | 40 | 400 |

Common runs:

```bash
make seed                                                       # Stowline (small) + Lumen
make seed SEED_ARGS="--only stowline --size medium --reset"     # replace Stowline only
make seed SEED_ARGS="--only lumen --reset"                      # replace Lumen only
make seed SEED_ARGS="--owner-email you@example.com --reset"     # own both workspaces with one login
```

`db:seed` refuses to run when `app.env` is `production`. It also refuses to overwrite an existing seeded workspace unless you pass `--reset`, and `--reset` purges only workspaces marked by the seeder.

### Vector search embeddings

By default, `db:seed` leaves the Stowline help center on keyword search so seeding never calls a paid API.

To build vector search for hybrid retrieval, set `ai.managed.embeddings.api_key` (`DIMMI_AI__MANAGED__EMBEDDINGS__API_KEY`) and run either:

```bash
make seed SEED_ARGS="--embed --reset"           # seed and queue embedding build
go run -tags dev ./cmd/dimmi db:embed           # embed an already-seeded Stowline workspace
go run -tags dev ./cmd/dimmi db:embed --now     # embed inline without waiting for a queue worker
```

Both `db:embed` and `db:seed --embed` check for built-in embedding credentials first and exit with an error if they are missing.

## Running checks as an AI agent

Run only the tests and linters related to your change. CI runs the full matrix.

### Go tests and lint

Load `dimmi.test.yaml` before running targeted Go packages:

```bash
eval "$(sh scripts/test-env.sh)"
go test -race -count=1 ./app/path/to/pkg/...
```

Match CI's cloud row-level security pass when touching queries, tenancy, or cloud code:

```bash
eval "$(sh scripts/test-env.sh)"
DIMMI_TEST_APPLICATION_ROLE=1 go test -race -count=1 -tags cloud ./tests/feature/yourfeature/...
```

Lint only the packages you changed:

```bash
./bin/golangci-lint run ./app/path/to/pkg/...
./bin/golangci-lint run --build-tags dev ./app/console/commands/...
./bin/golangci-lint run --build-tags cloud ./app/path/to/pkg/...
```

### Frontend and SDK checks

```bash
pnpm lint                       # Biome lint and format check
pnpm typecheck                  # TypeScript check across app, widget, SDKs, site, and docs
pnpm test path/to/file.test.ts  # targeted Vitest run
```

### Code generation and contracts

Run the matching generator whenever you edit its source files:

- **SQL queries (`database/queries/`):** `make sqlc`
- **Templ views (`resources/views/`):** `make views`
- **Config structs (`config/`):** `make config-example`
- **Routes, requests, resources, or webhook events:** run `make api-snapshot`, `make api-compat`, `make sdk-check`, and `make sdk-compat` in that order, then commit the updated snapshots in `tests/contracts/`. Never hand-edit generated SDK files.

## Rules to remember

- **Protect public contracts:** changes to `/api/v1`, `/widget/v1`, webhooks, and `sdk/` stay additive only. See [AGENTS.md](AGENTS.md).
- **Skip live LLM calls:** do not run commands that call paid AI or embedding APIs (`db:embed`, `ai:simulate` without `--dry-run`, or `make email-matrix`) without explicit owner approval for that run.
- **Leave publishing to CI:** never push tags, publish packages, or push to `dimmi-go`, `dimmi-php`, or `dimmi-laravel`.
- **Update docs with behavior changes:** change `docs/` in the same commit whenever you alter endpoints, fields, settings, defaults, or UI labels, following [WRITING.md](WRITING.md).
- **Format commits and PRs cleanly:** use lowercase Conventional Commits titles, short direct commit messages, light scannable PR descriptions, and zero AI or model attribution anywhere.
