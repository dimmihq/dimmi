# Testing

Binding for every package. Product owner rule: the test database is always a real PostgreSQL, the test Valkey is always a real Valkey, and S3 tests run against a real S3-compatible server. Business logic is tested only through integration tests. The framework layer uses unit and integration tests together.

## 1. Test kinds by layer

| Layer | Test kinds | Coverage gate (statements) |
|---|---|---|
| `app/` (actions, controllers, requests, policies, jobs, listeners, resources, mail, notifications, MCP tools, console commands) | **Integration only.** No unit tests with mocks. | 95% minimum per package, 100% target. CI fails below 95%. |
| `framework/` | Unit tests for pure logic, plus integration tests against real backing services. Every driver passes its `drivertest` suite. | 90% minimum per package, 95% total. |
| `app/contracts` + domain drivers (`app/drivers/*`, e.g. Channel, LLM, Knowledge) | Contract suites in `tests/contracts` run against every driver, plus unit tests for pure parsing and mapping code. | 90% minimum per package. |
| Generated code (sqlc, templ, huma OpenAPI) | None directly; covered through callers. | Excluded from gates. |

"Integration" means the real `App` wired the same way as production. It uses real Postgres, real Valkey, real migrations, the real event dispatcher, real River jobs, real RLS, and real transactions with real commits. Only process boundaries are faked (section 4).

## 2. Backing services: always real

### 2.1 Where the services come from

Tests use the same services the application uses, started the same way in every environment. There are no
testcontainers: tests never start or stop containers themselves.

- **Locally**, `make services` (or `docker compose up -d`) starts the root `compose.yaml`. Host ports are offset so
  they never clash with other local projects.

  | Service | Image | Host port |
  |---|---|---|
  | PostgreSQL | `pgvector/pgvector:pg18`, started with `max_connections=300` (parallel tests each open a pool) | 15432 |
  | Valkey | `valkey/valkey:8` | 16379 |
  | S3 | `chrislusf/seaweedfs:3.97` (S3 gateway) | 18333 |
  | Mail | `axllent/mailpit:v1.27` | 11025 (SMTP), 18025 (API) |

- **In CI**, the same images run as service containers.
- **Tests find the services through the environment variables the application itself reads**, set per environment.
  There are no `TEST_*` or `DIMMI_TEST_*` variables.

  | Variable | Points at | What testkit does with it |
  |---|---|---|
  | `DATABASE_URL` | PostgreSQL, as a superuser | Creates a database and a non-superuser owner role per test |
  | `REDIS_URL` | Valkey | Gives each test a unique key prefix |
  | `AWS_ENDPOINT_URL_S3`, `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | The S3-compatible server | Creates a bucket per test |
  | `MAIL_SMTP_URL`, `MAILPIT_API_URL` | Mailpit's SMTP port and its HTTP API | The `smtp` mail driver sends to it; the test reads the message back through the API |

- Local values live in `.env.testing`. `make test` loads `ENV_FILE` (default `.env.testing`; CI passes its own file),
  then runs `go test -race -shuffle=on ./...`, and runs `./framework/foundation/...` a second time with `-tags dev`,
  because the dev-only commands (`make:*`, `route:lint`, `lang:check`) are compiled only with that tag.
  `make cover` runs the framework packages with `-coverpkg=./framework/...` and prints the total.
- **A missing variable fails the test loudly**, naming the variable and how to start the services. Tests never skip
  because a service is absent, so a green run always means the code ran against real services.
- MinIO is not used: its images are no longer pullable. SeaweedFS is the local S3. testcontainers-go is not used
  either: per-run container starts and a Docker API dependency, where compose plus environment variables already do
  the job.
- Measured: the full `make test` runs in about 9 seconds on a laptop with the services already running.

The testkit API for this layer:

| Function | Gives the test |
|---|---|
| `testkit.NewDatabase(t)` | `Database{URL, AdminURL}`: a fresh database, `URL` as its non-superuser owner, `AdminURL` as the superuser; both dropped at cleanup |
| `testkit.NewMigratedDatabase(t, version, migrate)` | The same, copied from a template database already migrated for `version` (section 2.2) |
| `testkit.Fingerprint(t, fsys...)` | A version for `NewMigratedDatabase` that changes whenever a migration file is added, removed, renamed or edited |
| `testkit.NewValkey(t)` | A `*redis.Client` on `REDIS_URL` with the test's own key prefix, closed at cleanup |
| `testkit.ValkeyConfig(t)` | The same configuration without connecting, for tests that open clients themselves |
| `testkit.NewBucket(t)` | The name of an empty bucket, emptied and removed at cleanup |
| `testkit.Env(t, name)` | The variable's value, or a test failure when it is not set |
| `testkit.UniqueName(prefix)` | The prefix plus ten random lowercase letters and digits, for names no other test uses |

The testkit also holds the assertion helpers tests share:

| API | Use |
|---|---|
| `testkit.NewClient(t, handler)` | In-process HTTP client. `ActingAs(principal)`, `ImpersonateWith(fn)`, `WithHeader`, `WithCookie` return copies; `GetJSON`, `PostJSON`, `PatchJSON`, `PutJSON`, `Delete` and `Send(method, path, body)` return a `*Response` |
| `*testkit.Response` | `AssertStatus`, `AssertOK`, `AssertCreated`, `AssertNoContent`, `AssertNotFound`, `AssertForbidden`, `AssertUnauthorized`, `AssertJSONPath`, `AssertJSONMissing`, `AssertHeader`, `AssertValidationError(fields...)`, `Decode(&v)`, `String(path)` |
| `testkit.Table(t, q, table)` | `AssertHas(row)`, `AssertMissing(row)`, `AssertCount(n, where...)`; the package funcs `AssertDatabaseHas`, `AssertDatabaseMissing`, `AssertDatabaseCount` are the one-shot forms |
| `factory.Define[T](func(seq int) T)` (`testkit/factory`) | `Count`, `State`, `Sequence`, `Persist`, then `Create`/`CreateOne` (stored through the persist function, real inserts) or `Make`/`MakeOne` (not stored) |
| `arch.Load(t, module)`, `arch.LoadDir(t, root)` (`testkit/arch`) | `Expect(patterns...)` with `NotToImport`, `ToOnlyImport`, `NotToContainWords`, `Except`, `IgnoringTests` (section 5) |

### 2.2 PostgreSQL

- The image is `pgvector/pgvector:pg18`, the same as compose and production. The required extensions (`vector`, `pg_trgm`, `unaccent`, `citext`, `btree_gin`) are installed by the real migrations.
- Banned:
  - `embedded-postgres`: its binaries ship without pgvector.
  - SQLite, `sqlmock`, `pgxmock`, in-memory fakes of queries, and any interface wrapper around sqlc used for mocking.
  - depguard denies these packages in tests too.
- **Database per test.** `testkit.NewDatabase(t)` creates a database owned by a fresh role without superuser rights. Tests run in parallel without seeing each other's rows, and row-level security applies exactly as in production, where the application never connects as a superuser.
- **Template database.** `testkit.NewMigratedDatabase(t, version, migrate)` runs the migrations once into a template database named after `version`, as a role without superuser rights, and hands each test a copy (`CREATE DATABASE … TEMPLATE …`), so no test pays for migrations. Pass `testkit.Fingerprint(t, frameworkMigrations, appMigrations)` as the version: a schema change builds a new template, an unchanged one reuses the template already on the server, even across runs. Concurrent test processes build it under an advisory lock; a half-built template is dropped and redone.
- **Default isolation for app tests is a fresh tenant per test**, with real commits.
  - The app's test case, `tc := tests.New(t)` in the root `tests` package (built on the testkit primitives above once `app/` exists), creates its own organization, workspace, owner and member. Random emails and UUIDv7 ids mean parallel tests never collide.
  - RLS keeps each test's rows invisible to the others, so tenant isolation is exercised on every test.
  - Rows are never rolled back. Commits must happen so that AfterCommit listeners, `NOTIFY`, River jobs, advisory locks and `SET LOCAL app.workspace_id` behave exactly as in production.
  - Tenant-scoped data is reachable only inside `db.Transaction`, where the tenant is set. Tests that read tenant tables outside a transaction see them empty, by design.
- **Instance-global tests** cover system settings, the setup wizard, migrations, backup/restore and instance admin. They take a database of their own with `testkit.NewDatabase(t)` or `testkit.NewMigratedDatabase(t, ...)`.
- **Migration tests.** Every migration is tested up, down, then up again on its own database. A seeded dataset checks the expand/contract steps.
- **Time.** Business time never comes from `now()` in SQL. The clock is passed in from `framework/clock`, and SQL takes `@now` parameters (already a convention).

### 2.3 Valkey

- One shared Valkey from `REDIS_URL`. Each test gets its own key prefix (`t<random>:`), so parallel tests never see each other's cache entries, rate-limit counters, locks, pubsub topics or realtime channels, and nothing needs flushing afterwards: every key the framework writes expires on its own.
- `miniredis` and other in-memory Redis fakes are banned, like `embedded-postgres`. There is no in-memory cache store either.
- Tests never run `FLUSHALL` or `FLUSHDB`: other tests share the server. A test that needs a cold cache flushes its own keyspace (`cache.Store.Flush`). A test that needs Valkey to be unreachable points a client at a closed port, or runs it through an in-test TCP proxy that it cuts and restores.

### 2.4 S3

- `testkit.NewBucket(t)` creates an empty bucket on the S3-compatible server the standard AWS variables point at. The `filesystem` `s3` driver and its `drivertest` suite run against it.
- Business tests use the `local` disk driver in `t.TempDir()`; production and staging use `s3`. Signed URLs on the local disk need `signed_url_base` and `signing_key`.

## 3. How business-logic integration tests look

Preferred entry points, in order:
1. **HTTP feature tests**, the default. Each endpoint goes through router, middleware, request, policy, action, resource and database.
2. **Action or job entry with the real App**, for logic with no HTTP surface: scheduled jobs, listeners, inbound channel pipelines, metering rules.
3. **CLI and MCP tests** call the real command or tool handler.

Every endpoint covers:
- the happy path
- validation (422 shape)
- authorization:
  - another workspace returns 404
  - a missing permission returns 403
  - a missing token ability returns 403 `missing_ability`
  - a first-party-only route called with a PAT returns 403
- idempotency replay where `idempotent` applies
- event dispatch
- database state
- realtime broadcast
- queued job effects, run with `tc.Queue.Drain(ctx)` (works every ready job, then returns; scheduled and postponed jobs stay put)

Example:

```go
func TestCloseConversation(t *testing.T) {
	t.Parallel()

	t.Run("closes and broadcasts", func(t *testing.T) {
		tc := tests.New(t)
		conv := factories.Conversation(tc).WithMessages(2).Create()
		sub := tc.Subscribe("workspace:" + tc.Workspace.PublicID) // real realtime client on the Valkey broker

		res := tc.ActingAs(tc.Member).
			PatchJSON("/api/v1/conversations/"+conv.PublicID, testkit.JSON{"status": "closed"})

		res.AssertOK().AssertJSONPath("data.status", "closed")
		tc.Table("conversations").AssertHas(testkit.Row{"id": conv.ID, "status": "closed"})
		tc.Table("audit_events").AssertHas(testkit.Row{"action": "conversation.closed"}) // the listener's effect
		sub.AssertReceived(t, "conversation.closed")
	})

	t.Run("rejects another workspace", func(t *testing.T) {
		tc := tests.New(t)
		other := factories.Conversation(tests.New(t)).Create()

		tc.ActingAs(tc.Member).
			PatchJSON("/api/v1/conversations/"+other.PublicID, testkit.JSON{"status": "closed"}).
			AssertNotFound()
	})
}
```

Rules:
- Assert observable behavior: HTTP response, database rows, what listeners and jobs did, broadcasts a real subscriber received, outbound mail/HTTP captured by boundary fakes. There is no event or queue fake: a dispatched event shows in its listeners' effects, a pushed job in its River row or, after `Drain`, in what it did. Never assert that an internal function was called.
- Factories create data through real inserts (`factories.*`), never through hand-written SQL in tests.
- Every edge case in a spec's `edge-cases.md` is covered by at least one test. Tests are named for humans, in plain words that describe the behavior (`t.Run("reopens a closed conversation when another is merged into it", ...)`), never by case ids (PO 2026-11-02). Review checks the spec's edge-case list against the tests.
- Use `t.Parallel()` everywhere except tests that take instance-global locks.
- Suite budget: the full `go test ./...` runs in under 5 minutes on CI (8 vCPU), and one feature package runs in under 20 s locally.

## 4. Fakes: process boundaries only

Fakes exist only where a test cannot use the real thing. A service with a real local option has no fake at all. There are no fakes for the database, cache, filesystem, queue, events, broadcasting, pubsub, rate limiter, encryption, hashing, process, log, telemetry, notifications or MCP: tests run them for real and assert on their effects.

Real in business tests: Postgres, Valkey, event dispatcher, River (drained with `Queue.Drain` or run by a worker), realtime broadcaster (real centrifuge node on the Valkey broker; tests subscribe a real client to see broadcasts), cache (real Valkey), rate limiter, internal pubsub, filesystem (`local` driver in a temp dir), encryption (`local` KEK on the app key), hashing (argon2id with test cost parameters), notifications (their channels write real rows, broadcasts and, through the mail fake, mail), validation, authz, the settings registry, entitlements (self-host driver, or cloud driver with seeded overrides).

Faked, because they cross the process boundary:

| Boundary | Fake | Asserts |
|---|---|---|
| Outgoing mail transport | `a.Mail.Fake()` replaces every mailer and records each message after the full build | `mail.AssertSent[M](t, fake, match)`, `fake.AssertMessage`, `fake.AssertNothingSent`, `fake.Sent()` |
| Outbound HTTP to third parties (Slack, Discord, Telegram, WhatsApp, Stripe, GitHub, AWS APIs, webhook endpoints, crawler targets) | `httpclienttest.New(a.HTTP)` (package `framework/httpclient/httpclienttest`) swaps the client's transport, so SDKs holding `StdClient()` are faked too; `.Stub(pattern, httpclient.Response)`, `.PreventStrayRequests()` | `fake.AssertSent(t, match)` on request shape, retries |
| DNS | `a.DNS.Fake()` with `.Set(name, dns.Records{...})` | lookups answered from the zone |
| Malware scanners | `a.Scanner.Fake()` (finds only EICAR unless `.Set(path, verdict)`) | `fake.AssertScanned(t, path)` |
| Social login providers | `a.Social.Fake()`; `fake.Approve(authURL, user)` returns the callback URL the provider would redirect to | the signed-in user |
| LLM providers | `llm.Fake()` scripted responses (07) | prompts, tool calls, spend |
| Push (web push, APNs, FCM) | the push service's HTTP endpoint through the HTTP fake | payload rule (06) |
| Cloud-only services (KMS, SES API, CloudFront) | local drivers in business tests; driver behavior proven in framework tests against the HTTP fake (section 5) | |
| Clock | `clock.NewFake(t0)` passed where the code takes a clock; `Freeze(t)`, `Travel(d)` | DST, postponing, SLA |
| Randomness for tokens | real `crypto/rand`; tests read tokens from captured mail or responses | |

Every service fake refuses to exist outside the test suite (the fake clock is a plain value a test builds and passes in, so it needs no switch): `Fake()` and `httpclienttest.New` panic with the package's `ErrFakesNotAllowed` unless the configuration allows fakes, and the foundation sets that switch (`AllowFakes`, `koanf:"-"`, never read from configuration) only when `app.env` is `testing`. A fake in production would drop mail, pass infected files or sign anyone in, so reaching one there is a programming error.

Never fake: the database, the cache, the filesystem, actions, policies, queries, events or jobs. A test that needs to fake one of these is a sign the code is badly shaped, so restructure the code instead.

## 5. Framework and interface tests

- **Unit tests** cover pure logic: cursor encoding, signing, TypeID parsing, MIME building, rule evaluators, rate-limit math, header parsing, config merging. They are table-driven, use no I/O, and keep each case under 10 ms.
- **Integration tests run against the real backing services** from section 2.1, found through the normal environment variables. A missing variable fails the test; nothing is skipped, locally or in CI.

| Package / driver | Real service |
|---|---|
| database, migrate, queue (River), advisory locks, idempotency store | `pgvector/pgvector:pg18` |
| cache (including `Remember` stampede protection and locks), ratelimit, pubsub, broadcasting (centrifuge Redis broker, presence, history/recovery across two nodes), `cache.Add` dedupe | `valkey/valkey:8` |
| filesystem `s3` (`drivertest`) | SeaweedFS S3 (`chrislusf/seaweedfs:3.97`) |
| filesystem `local` (`drivertest`) | temp directory |
| mail `smtp` | Mailpit (`MAIL_SMTP_URL`; asserts via `MAILPIT_API_URL`) |
| encryption `awskms`, mail `ses`, mailbox `ses`, malware `guardduty` | the AWS API answered by the HTTP fake (`httpclienttest`) or an unreachable endpoint for failure paths; real AWS only behind the `live` tag (section 6) |
| httpclient safedial (SSRF) | local listeners on loopback and private ranges |
| database topologies: transaction pooler, multi-host failover, read replica (lag + fallback) | PgBouncer (transaction mode) in front of pg18; primary + streaming replica pair (promote replica mid-test) |
| redis topologies: Sentinel failover, cluster (`MOVED`, resharding, `CROSSSLOT` guard), centrifuge broker in cluster mode | Valkey 1 primary + 1 replica + 3 Sentinels; Valkey 3-primary cluster |

- **Topology suites** run every `framework/database`, `framework/redis`, `cache`, `ratelimit`, `pubsub`, `broadcasting` drivertest suite against each topology above, plus failover scenarios (kill primary mid-load: no wrong-primary writes, 503s only during the window). These need extra services beyond the root `compose.yaml` (a separate compose profile), so they run in the nightly `topology` CI job and on PRs touching `framework/database` or `framework/redis`; the default PR job uses single instances.

- **Driver contract suites** (`framework/<pkg>/drivertest`, `tests/contracts` for domain contracts): one suite per interface runs unchanged against every driver, including the fake where one exists. A community driver is supported only if it passes the suite. Suites live in non-test files so driver packages can import them; depguard treats `drivertest` directories as test code.
- **Interface abstractions** (`app/contracts`) have contract suites too. The fake implementation must pass the same suite as the real one, so fakes cannot drift.
- **Architecture tests** (`tests/arch`) use `testkit/arch`: the framework never imports the application and contains no product words. `TestBinarySize` builds a small application (`tests/arch/testdata/sizeapp`) with `-ldflags=-dumpdep` and fails when any function in the dependency dump carries the `<ReflectMethod>` marker. Such a function (a method looked up by a name known only at run time, as `text/template`, `reflect.Value.MethodByName` and libraries built on them do) makes the linker keep every exported method of every type and roughly doubles the binary, so CI refuses it.

## 6. Live tests against real third parties

- Build tag `live`, never part of `go test ./...` or CI default jobs. Examples: real SES send, real Slack workspace, real LLM providers, real Stripe test mode.
- Running a live test that calls a paid API (LLM providers above all) needs explicit approval for that run, with its rough cost stated first. An earlier approval never carries over.
- Live tests only check that the fixtures used by fakes still match real provider behavior. They never replace integration tests.

## 7. CI

- Service containers `pgvector/pgvector:pg18` (`max_connections=300`), `valkey/valkey:8`, `chrislusf/seaweedfs:3.97` (S3) and `axllent/mailpit:v1.27`. CI passes its own env file to `make test` (`ENV_FILE=...`), setting the same variables as `.env.testing`: `DATABASE_URL`, `REDIS_URL`, `AWS_ENDPOINT_URL_S3`, `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `MAIL_SMTP_URL`, `MAILPIT_API_URL`.
- Steps: lint (code-style.md; `make lint` runs once without and once with the `dev` build tag), then `make test` (including the `tests/arch` binary-size test), then the coverage gates.
- Coverage gates:
  - `go test -coverpkg=./app/... ./...` gives the app number; fail if any `app` package is under 95%.
  - `-coverpkg=./framework/...` gives the framework number; fail if any package is under 90% or the total is under 95%.
  - Generated files are excluded by path (`*.sql.go`, `*_templ.go`, `openapi_gen.go`).
- Nightly `topology` job: topology suites (section 5) against pooler, replica, Sentinel and cluster setups.
- Flaky test policy: a flaky test is quarantined in under 24 hours with an issue link and fixed within a week. Never use `t.Skip` without an issue.
