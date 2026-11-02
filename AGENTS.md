# Working in this repository

This file is for every coding agent and every new contributor. `CLAUDE.md` links here. The full rules live in
`specs/00-conventions/` (`framework.md`, `code-style.md`, `testing.md`); this page is the part you need on every
change. When this page and those files disagree, this page wins and the disagreement is reported.

## Layout

- `framework/` is the reusable foundation: anything a different SaaS product would need unchanged. It wraps
  third-party libraries behind small Laravel-shaped APIs. It never contains product words (`workspace`,
  `conversation`, `contact`, `inbox`, `widget`, `dimmi`, ...) in identifiers, strings, comments or file names.
  Generic words (`tenant`, `user`, `member`, `channel`, `message`) are fine.
- `app/`, `routes/`, `config/`, `bootstrap/`, `database/`, `cmd/` hold the product. Not built yet.
- `specs/` and `research/` describe what to build. Code never refers to them.
- The package catalog, what each package wraps and its public API: `specs/00-conventions/framework.md` section 3.2.

## Write only the code we need

- A mature open source library for the job wins over our own code. Allowed modules are listed in the depguard
  `framework` allowlist in `.golangci.yml`; licenses must be MIT, BSD, ISC, Apache-2.0 or MPL-2.0.
- Wrappers stay thin: translate config, map errors to package errors, hide library types. Do not re-implement
  what the library already does, and do not build features no spec asks for.
- Watch binary size: prefer the small, focused module of a library (a sub-package, an HTTP exporter instead of
  gRPC) and never pull a big dependency for one helper.
- No ORM. Queries are SQL compiled by sqlc; `framework/database` is only pool, transactions, tenant scoping and
  error mapping.

## Comments

Every non-test Go file opens with a Laravel-style banner that states the business intent of the file: why it
exists and what must stay true. A blank line separates it from the `package` clause.

```go
/*
|--------------------------------------------------------------------------
| Cache Store
|--------------------------------------------------------------------------
|
| The cache only ever saves work. Every value expires on its own, so the
| store can be flushed or lost at any moment and the application simply
| computes again.
|
*/

package cache
```

- Each package has `doc.go` holding only a short attached doc comment: `/*\nPackage cache stores ...\n*/`.
- Every exported identifier gets a `/* ... */` docblock that starts with its name, in full sentences, describing
  behavior and intent. Grouped constants or errors share one docblock above the group. Interface methods get
  indented `/* */` docblocks. Struct fields are explained in the type's docblock.
- Unexported helpers get a docblock only when the reason is not obvious from the name.
- No comments inside function bodies, ever. If a step needs explaining, explain it in the function's docblock or
  extract a well-named function.
- No `//` comments at all, except `//go:` directives and code examples inside a docblock.
- Never mention specs, research files, section numbers, tickets, history or authors. Say the real reason in plain
  words.
- Test files have no banner. Subtest names are lowercase sentences and are the only description a test needs;
  test helpers use `/* */` docblocks.

## Code

- Guard clauses first, the happy path down the left edge, no `else` after `return`.
- Errors: package sentinels (`ErrX`) or types ending in `Error`; wrap once per layer with what was being attempted
  (`fmt.Errorf("open disk %s: %w", name, err)`), lowercase, no "failed to". Never log and return.
- `ctx context.Context` first on anything doing I/O; never stored in a struct.
- No package-level mutable variables, no `init()`, no `panic` outside `framework/` `Must*` helpers.
- Time comes from `framework/clock`, never `time.Now()`; outbound HTTP from `framework/httpclient` (once it exists).
- Limits enforced by lint: 400 code lines per file, 50 lines and 30 statements per function, cognitive complexity
  15, 4 parameters, 3 results, 120-column lines, 80-column doc lines.
- Managers (`framework/manager`) for services with several drivers; register drivers with `Extend`, never `init`.

## Tests

- Business logic is tested only with integration tests against real services. Framework packages use unit tests
  for pure logic plus integration tests against real services.
- Real PostgreSQL, Valkey, S3 (SeaweedFS) and SMTP (Mailpit) come from `compose.yaml`. Start them with
  `make services`. Tests read the same environment variables the application reads (`DATABASE_URL`,
  `REDIS_URL`, `AWS_*`, `MAIL_*`), loaded from `.env.testing` by `make test`. A missing variable fails the test.
- Per-test isolation helpers live in `framework/testkit`: `NewDatabase`, `NewValkey`, `NewBucket`, `Env`,
  `UniqueName`.
- No fakes for things we can run for real (database, cache, filesystem via the `local` driver, queues, events).
  Fakes exist only at true process boundaries: outbound mail transport, third-party HTTP
  (`framework/httpclient/httpclienttest`), DNS, malware scanners, social login providers, LLM providers, and the
  clock. External programs run for real. A fake must pass the same `drivertest` contract suite as the real driver,
  and every `Fake()` refuses to run outside the testing environment.
- One top-level test per unit, subtests named as lowercase sentences, `t.Parallel()` everywhere, `t.Context()`
  for contexts, `require`/`assert` from testify.
- Coverage gate: 90% of statements per framework package, measured with `make cover`.
- Never run tests that call paid third-party APIs (LLMs above all) without explicit approval for that run.

## Commands

| Command | Does |
|---|---|
| `make services` | Starts PostgreSQL, Valkey, SeaweedFS and Mailpit |
| `make fmt` | Formats Go code |
| `make lint` | golangci-lint with zero issues allowed |
| `make test` | All tests with `-race` against the services |
| `make cover` | Coverage across framework packages |
| `make gen ARGS="make:action Name"` | Runs a dev-only command (generators, `route:lint`, `lang:check`) |

Development-only commands live in files with `//go:build dev` and are never compiled into release builds;
`make lint` and `make test` cover both builds.

The Go linker must keep removing unused methods: never call `reflect.Value.MethodByName` or `Method` with a
non-constant name, and never execute `text/template` or `html/template` in framework code paths a running app
reaches. `tests/arch` fails the build when this regresses, which doubles the binary size.

A change is done when `make lint test` passes. Never add `//nolint`, raise a limit or edit `.golangci.yml` to get
there: change the code, or stop and report why the rule does not fit.
