# Working in this repository

This file is for every coding agent and every new contributor. `CLAUDE.md` links here. The full rules live in
`specs/00-conventions/` (`framework.md`, `code-style.md`, `testing.md`); this page is the part you need on every
change. When this page and those files disagree, this page wins and the disagreement is reported.

## Layout

- `framework/` is the reusable foundation: anything a different SaaS product would need unchanged. It wraps
  third-party libraries behind small Laravel-shaped APIs. It never contains product words (`workspace`,
  `conversation`, `contact`, `inbox`, `widget`, `dimmi`, ...) in identifiers, strings, comments or file names.
  Generic words (`tenant`, `user`, `member`, `channel`, `message`) are fine.
- `app/`, `routes/`, `config/`, `bootstrap/`, `database/`, `cmd/`, `lang/`, `resources/` hold the product.
  `cmd/dimmi` only calls `bootstrap.Main`; `bootstrap` hands the framework the app's options and providers.
- `resources/js/` is the browser app (React, Vite, TanStack Router and Query, BoardUI on Tailwind v4). Go serves
  its page; `public/build` holds the production build embedded in the binary.
- Two editions, chosen at build time: the default build is the self-hosted binary (`bin/dimmi`); `-tags cloud`
  adds the cloud-only code (billing, marketing site) for `bin/dimmi-cloud`. Cloud-only code lives in `app/cloud/...`
  or in files starting with `//go:build cloud`, and only such files import `app/cloud/...` (`tests/arch` checks).
  The edition is never configurable: `config.BuiltEdition` says which binary runs.
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
  error mapping. Queries live in `database/queries/*.sql`; run `make sqlc` after editing them or a migration and
  commit the generated `app/models/*.sql.go`. Never edit generated files.

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

## Frontend

- Biome is the only linter and formatter of the browser app. `pnpm lint` (`biome ci --error-on-warnings`) must
  finish with zero errors and zero warnings, every time. A warning is a failure.
- Never add `biome-ignore` comments, never lower a rule or add an override in `biome.json`, and never exclude a
  file to get there: change the code, or stop and report why the rule does not fit.
- Run `pnpm format` (`biome check --write`) before finishing.
- BoardUI's files (`resources/js/components`, `resources/js/styles`, `resources/js/utils`) stay exactly as
  `boardui add` and `boardui init` write them: never edit them, and Biome skips them. Our own code lives elsewhere
  under `resources/js` and builds on them.
- `pnpm typecheck` and `pnpm test` pass too. `make lint test` runs all of it with the Go checks.

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
- App tests live in `tests/feature/<area>` and start with `tc := tests.New(t)`: the real application on its own
  migrated database and Valkey prefix, a fake clock (`tc.Clock`), a workspace and its owner. Act through the real
  guards with `tc.ActingAs(user)` (a real sign-in) or `tc.ActingAsToken(member, abilities...)`; create data with
  `database/factories`.
- Coverage gates: 90% of statements per framework package (`make cover`), 95% per app package (`make cover-app`).
- Never run tests that call paid third-party APIs (LLMs above all) without explicit approval for that run.
- After a milestone that adds or changes endpoints, run the release binary end to end as described in
  `docs/testing/manual-e2e.md`, and extend its flow table in the same change.

## Commands

| Command | Does |
|---|---|
| `make services` | Starts PostgreSQL, Valkey, SeaweedFS and Mailpit |
| `make fmt` | Formats Go code and the browser app (Biome) |
| `make lint` | golangci-lint and Biome with zero issues and zero warnings allowed, plus TypeScript |
| `make test` | All Go tests with `-race` against the services, then the browser app's tests |
| `make cover` | Coverage across framework packages |
| `make cover-app` | Coverage of app packages by the integration tests |
| `make sqlc` | Compiles `database/queries` into `app/models` |
| `make gen ARGS="make:action Name"` | Runs a dev-only command (generators, `route:lint`, `lang:check`) |
| `make build` | Builds the browser app and both binaries: `bin/dimmi` (self-hosted) and `bin/dimmi-cloud` |
| `make dev` | Go server on :8080 (rebuilt by air) plus the Vite dev server with hot updates; open `localhost:8080/d`. `TAGS=cloud` runs the cloud edition |
| `make web-build` | Builds the browser app into `public/build`, which the next `go build` embeds |
| `make web-test` | Type-checks and tests the browser app |

Development-only commands live in files with `//go:build dev` and are never compiled into release builds;
`make lint` and `make test` cover the dev build and both editions; edition-specific tests live in files tagged
`cloud` or `!cloud`.

The Go linker must keep removing unused methods: never call `reflect.Value.MethodByName` or `Method` with a
non-constant name, and never execute `text/template` or `html/template` in framework code paths a running app
reaches. `tests/arch` fails the build when this regresses, which doubles the binary size.

A change is done when `make lint test` passes, Biome included, with no warnings. Never add `//nolint`, raise a limit or edit `.golangci.yml` to get
there: change the code, or stop and report why the rule does not fit.

<!-- boardui:rules:start -->
# BoardUI design rules

This project uses BoardUI (React + Tailwind CSS v4, source-owned components under `components/`). These rules always apply when writing UI code. MCP tools are on demand; these rules are not optional context.

## Components first

- Before hand-building any UI element, check for an installed BoardUI component under `components/base/` and `components/application/`, and prefer it.
- Missing a component? Install it (BoardUI MCP `install_components`, or `npx boardui@latest add <name>`) instead of writing a lookalike.
- Import through the aliases configured in `boardui.json`, e.g. `import { Button } from "@/components/base/buttons/button"`.

## Color: semantic tokens only

- Never use raw palette classes (`text-gray-500`, `bg-white`, `border-neutral-200`) or hex/oklch literals. Every color rides a BoardUI semantic token, which also makes dark mode automatic.
- Text: `text-text-primary`, `text-text-secondary`, `text-text-tertiary`, `text-text-placeholder`, errors `text-text-error-primary`.
- Surfaces: `bg-background-primary-default`, `bg-background-secondary-default` / `-hover`, `bg-background-tertiary-default`, page ground `bg-background-full`.
- Borders: `border-border-button-default` / `-hover`, hairlines `border-separator-border`, tables `border-border-table`, errors `border-border-error-default`.
- Icons: `text-foreground-icon-primary` through `text-foreground-icon-quaternary`.
- Charts: the `chart-1` … `chart-5` tokens (plus `-active` variants). CTAs and selection states: the `accent-50` … `accent-950` ramp.
- Dark mode flips tokens via the `.dark` class on `<html>`. Do not write `dark:` overrides with raw colors; if a token pair looks wrong in dark mode, pick a different token, not a literal.

## Typography: composite utilities only

- Use BoardUI's composite type utilities: `text-title-1-medium`, `text-title-2-medium`, `text-title-3-semibold`, `text-headline-medium`, `text-body-medium`, `text-body-regular`, `text-body-2-*`, `text-caption-1-semibold`, and friends. Each sets size, weight, line-height, and letter-spacing together.
- Never rebuild type by stacking `text-sm font-medium leading-5`; if a style seems missing, look in `styles/typography.css` before inventing one.

## Spacing and shape

- Stay on Tailwind's spacing scale (`gap-2`, `p-4`, `mt-6`); prefer flex/grid `gap` over per-child margins. Arbitrary values (`p-[13px]`) only when matching an existing BoardUI component exactly.
- Cards and panels: `rounded-3xl` with `border-border-button-default`. Inputs and menu rows: `rounded-md` to `rounded-xl`. Pills: `rounded-full`.

## Mechanics

- Direction: wrap the app (or a mixed-language region) in `DirectionProvider` from `@/components/foundations/direction/direction`, with an explicit BCP 47 `locale` (`ar`, `he`, `en-US`, `zh-CN`). Set matching `lang` and `dir` on `<html>` for full-app RTL. CSS `dir` alone does not configure React Aria keyboard navigation or portal placement. Use `useDirection()` for custom interactions and put its value on custom portal roots.
- Use logical styles: `ms`/`me`, `ps`/`pe`, `start`/`end`, `text-start`/`text-end`, `border-s`/`border-e`, `rounded-s`/`rounded-e`, and inline logical CSS properties. Mirror navigation/disclosure glyphs and horizontal drawer motion in RTL. Preserve physical coordinates for measured geometry, charts, canvas, and decorative art. Keep code, OTP digits, email addresses and URLs in explicit LTR regions; isolate mixed text with `<bdi>` or `dir="auto"`. Translating labels and formatting application data is separate from layout direction. See https://www.boardui.com/rtl.

- Internal vertical scroll regions must soften the top edge with a progressive blur and surface fade once scrolled, rather than sharply clipping content. Keep headers outside the scroll region. Use a stationary, `pointer-events-none`, `aria-hidden` overlay in a relative wrapper: a 24px masked 1px backdrop blur, a 16px masked 4px backdrop blur, and a gradient from the enclosing semantic surface token to transparent. Ramp each layer's opacity from 0 to 1 over the first 24px of scrolling; keep it invisible at scrollTop 0. Apply opacity to the individual layers, never the overlay parent (a translucent parent creates a backdrop root and can break blur at normal browser zoom). Verify top/reset, overflow, light/dark mode, and 100% zoom; preserve keyboard scrolling and do not blur the whole content area. The `scroll-fade` registry item ships this recipe as `ScrollFade` + `useScrollFade` (`@/components/foundations/scroll-fade/scroll-fade`); reuse it rather than rebuilding the layers.

- Merge classes with `cx()` from `@/utils/cx` (tailwind-merge aware of BoardUI's composite text styles). No string concatenation, no plain `clsx`.
- Icons come from `@remixicon/react`, passed as component references (`leadingIcon={RiAddLine}`), not rendered elements.
- Form components build on `react-aria-components`; extend the installed BoardUI form components rather than raw `<input>`/`<select>`.
- Focus states: `outline-none focus-visible:ring-2 focus-visible:ring-border-focus-ring`.

When unsure about a token, a component's API, or working example code, ask the BoardUI MCP server: `get_theme`, `get_component`, `get_usage_examples`.
<!-- boardui:rules:end -->
