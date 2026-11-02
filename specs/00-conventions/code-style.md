# Code style

Date: 2026-11-02. Binding for all Go, SQL, templ and Markdown in the repository.

This file defines how Dimmi code reads, how it is checked and how it is commented. [README.md](README.md) defines
where code lives and the request lifecycle. [framework.md](framework.md) defines the `framework/` layer and owns the
import and identifier rules (depguard, forbidigo, importas, arch tests, section 7). This file owns everything else in
the lint pipeline. Module path: `github.com/dimmihq/dimmi`. Tool facts were checked against upstream docs on
2026-11-02.

## 1. Principles

1. **Readable over clever.** Code is read far more often than it is written, by humans and by agents. If a reviewer
   has to stop and work out what a line does, rewrite the line.
2. **Business code reads like the domain.** An action body reads like the spec paragraph it implements:
   `if conversation.IsClosed() { return exceptions.ErrConversationClosed }`. Plumbing (drivers, retries, HTTP,
   serialization) lives in `framework/`, out of sight.
3. **One way to do things.** One formatter, one import order, one error style, one log call shape, one suppression
   mechanism. When two styles are both fine, this file picks one and a linter enforces it.
4. **Boring Go.** Standard library and framework first. No reflection magic, no generics unless they remove real
   duplication across two or more concrete types, no goroutines in `app/` (use a job). Prefer the obvious loop.
5. **Machines enforce, humans review intent.** Anything a linter can check is a linter rule, so review time goes to
   naming, design and correctness.

## 2. Lint and format pipeline

### 2.1 Commands

Lint tools are developer tools, not part of the shipped binary, so the entry point is `make`. The exceptions are
the dev-only commands of the application binary (`route:lint` needs the compiled route table, README "Routes";
`lang:check` reads the source and the message files), which `make gen` runs.

| Command | Does |
|---|---|
| `make services` | `docker compose up -d --wait`: PostgreSQL, Valkey, S3 (SeaweedFS) and Mailpit from root `compose.yaml` |
| `make tools` | Installs pinned tool versions into `./bin` |
| `make fmt` | `golangci-lint fmt` (gofumpt + gci) and, once templ files exist, `templ fmt .` |
| `make lint` | `golangci-lint config verify`, then `golangci-lint run ./...` twice: once as the release build sees the code and once with `--build-tags dev`, so dev-only files are checked too. Later also every other step of 2.2 except tests, collecting all failures |
| `make lint-fast` | Format check + `golangci-lint run --new-from-rev=origin/main` (inner loop; planned) |
| `make test` | Loads `ENV_FILE` (default `.env.testing`; CI passes its own), then `go test -race -shuffle=on ./...` against the running services, then `go test -race -tags dev ./framework/foundation/...` for the dev-only commands. No third-party network; see testing.md |
| `make cover` | Framework coverage: `go test -race -count=1 -coverpkg=./framework/... -coverprofile=coverage.out ./framework/...`, then prints the total |
| `make gen ARGS="make:action tags/CreateTag"` | Runs a dev-only command: `go run -tags dev ./cmd/$(APP) $(ARGS)`, `APP` defaulting to `dimmi` |

The application binary splits its commands by build tag. Commands that only help development live in files with
`//go:build dev`, so a release build (built without the tag) does not contain them:

| Build | Commands |
|---|---|
| Dev only (`-tags dev`, via `make gen`) | `make:action`, `make:command`, `make:event`, `make:job`, `make:listener`, `make:mail`, `make:migration`, `make:test`, `make:controller`, `make:request`, `make:policy`, `make:notification`; `route:lint`; `lang:check` |
| Every build | `serve`, `migrate`, `migrate:status`, `migrate:plan`, `migrate:rollback`, `down`, `up`, `config:show`, `route:list`, plus the application's own commands |

There is no `make:model`: sqlc generates the models from the queries. Every generator accepts `--force` to overwrite
an existing file; `make:listener` takes `--event` and `--queued`, `make:request` takes `--authorize` and `--validate`,
`make:policy` takes `--model` and `make:command` takes `--signature`. The application can override any default stub
from `framework/console/stubs` with its own file in the repository's `stubs/` directory.

In production (`app.env: production`) the commands that change the running system ask first. `migrate` shows the
database host and name and every pending migration, `down` warns that every user will be turned away, and
`migrate:rollback` shows the migration it will undo and the database. `--force` skips the question for automation;
without a terminal and without `--force` the command refuses. Rolling back production additionally needs the exact
version typed at the prompt or passed as `--confirm-version`, even with `--force`, so a pipeline never undoes the
wrong migration. `migrate:status` and `migrate:plan` only read and never ask.

Versions are Makefile variables (`GOLANGCI_LINT_VERSION := v2.15.0`, and so on). golangci-lint is installed from its
release binary, as its docs recommend; they advise against `go install` and `go tool` for it.

### 2.2 CI order

Cheap, deterministic checks run first, so a formatting slip fails in seconds.

| # | Step | Command | Fails when |
|---|---|---|---|
| 1 | Format | `golangci-lint fmt && templ fmt . && git diff --exit-code` | A file was not formatted |
| 2 | Generated code | `sqlc generate && templ generate && go mod tidy -diff && git diff --exit-code` | Generated code or `go.mod` is stale |
| 3 | Go lint | `golangci-lint config verify && golangci-lint run ./...` | Any issue (2.4) |
| 4 | Architecture | `go test ./tests/arch/...` (framework.md 7.2) | Import graph or business-word violation, or a `<ReflectMethod>` function in the linker's dependency dump (binary size, section 4.5) |
| 5 | Comment rules | `make lint-comments` (2.6) | Missing `doc.go` or file banner, `//` comment, comment inside a function body, bad `TODO`, banned suppression |
| 6 | Routes and messages | `make gen ARGS="route:lint"`, `make gen ARGS="lang:check"` | Verb in path, `PUT` outside allowlist, unlisted route; a locale missing a message or a message no code uses |
| 7 | Migrations | `make lint-migrations` (squawk, 2.6) | Destructive or locking change without contract note |
| 8 | SQL | `sqlc vet` against CI Postgres with migrations applied | Query fails to prepare or matches a vet rule |
| 9 | Vulnerabilities | `govulncheck ./...` | A known vulnerability is reachable from our code |
| 10 | API compatibility | `oasdiff breaking --fail-on ERR base.json head.json` (2.6) | Breaking change in `/api/v1` or `/widget/v1` |
| 11 | Docs | `markdownlint-cli2 "**/*.md" "#node_modules"` | Markdown structure errors |
| 12 | Tests | `make test` | Any failure |

govulncheck exits non-zero when it finds vulnerabilities only in its default text mode (JSON, SARIF and OpenVEX
output always exit 0), so CI uses text mode.

### 2.3 Import groups

Four groups in this order, separated by a blank line: standard library, third party, framework, the rest of the
module. gci enforces it and `make fmt` fixes it. Third-party imports appear only where framework.md allows them.

```go
import (
	"context"
	"fmt"

	"github.com/riverqueue/river"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"

	appevents "github.com/dimmihq/dimmi/app/events"
	"github.com/dimmihq/dimmi/app/models"
)
```

gci assigns an import to the longest matching prefix, so `.../framework/...` lands in the framework group and `app`,
`config`, `routes`, `database` and `tests` land in the last group. gopls `formatting.local` takes one prefix only, so
editors get close and `make fmt` is the authority. Aliases such as `appevents` come from importas (framework.md 7.1).

### 2.4 `.golangci.yml`

The committed file is this block merged with framework.md 7.1. `linters.default: none` makes the enable list the
complete, reviewed set; each linter carries its reason.

```yaml
version: "2"

run:
  relative-path-mode: gomod   # exclusion paths are relative to go.mod

linters:
  default: none
  enable:
    # Correctness
    - govet          # compiler-adjacent checks (printf, copylocks, unusedresult)
    - staticcheck    # bugs, simplifications, style (SA, S, ST, QF checks)
    - errcheck       # every error handled or discarded with an explained `_ =`
    - errorlint      # errors.Is/As and %w; never == or type switches on wrapped errors
    - nilerr         # returning nil after checking err != nil
    - nilnesserr     # returning a different, nil error after a failed check
    - ineffassign    # assignments never read
    - unused         # dead code
    - unparam        # parameters that always receive the same value
    - wastedassign   # values overwritten before use
    - unconvert      # conversions to the type a value already has
    - exhaustive     # every switch over an enum lists every value
    - bodyclose      # HTTP response bodies closed (framework/httpclient)
    - sqlclosecheck  # rows closed (database/sql and pgx v5)
    - noctx          # outbound requests carry a context
    - contextcheck   # functions pass the caller's ctx on
    - containedctx   # no context.Context stored in structs
    - gosec          # security footguns (weak crypto, file modes, string-built SQL)
    - recvcheck      # a type uses value or pointer receivers, not both
    - thelper        # test helpers call t.Helper()
    - usetesting     # t.Context(), t.TempDir(), t.Setenv() in tests
    # Architecture (settings in framework.md 7.1)
    - depguard
    - forbidigo
    - importas
    - gochecknoinits
    - gochecknoglobals # no package-level mutable state
    # Size and complexity (numbers in section 3)
    - funlen
    - gocognit
    - nestif
    - lll
    - interfacebloat
    # Readability
    - revive         # Go style, file length, argument and result limits
    - gocritic       # diagnostic, style, performance; includes commented-out code
    - errname        # sentinels start with Err, error types end with Error
    - inamedparam    # interface method parameters are named
    - modernize      # current stdlib idioms (min/max, slices, maps, range over int)
    - intrange       # `for i := range n`
    - usestdlibvars  # http.MethodPost, http.StatusOK instead of literals
    - tagliatelle    # JSON and YAML tags are snake_case (Laravel JSON shape)
    - sloglint       # one log call shape: context, static message, attributes
    - whitespace     # no blank lines at the start or end of blocks
    - misspell       # US English
    - dupword        # "the the"
    # Comments (section 5)
    - godot          # every comment is a sentence: capital letter, final period
    - godoclint      # doc line length, link hygiene, deprecation format
    - godox          # FIXME, XXX, HACK, BUG are not mergeable
    - nolintlint     # //nolint names the linter and gives a reason

  settings:
    # depguard, forbidigo, importas: framework.md 7.1, plus the forbid entries below.
    forbidigo:
      forbid:   # appended to framework.md's list
        - pattern: '^panic$'
          msg: '[panic] return an error; Must* helpers live in framework/ only'
        - pattern: '^(print|println)$'
          msg: '[print] use framework/log or console output'
    govet:
      enable-all: true
      disable:
        - fieldalignment   # field order is for readers, not for saving bytes
        - shadow           # `err` shadowing is idiomatic
    errcheck:
      check-type-assertions: true
    exhaustive:
      check: [switch, mapliteral]
      default-signifies-exhaustive: false   # a new enum value must surface every switch
    funlen:
      lines: 50
      statements: 30
      ignore-comments: true
    gocognit:
      min-complexity: 15
    nestif:
      min-complexity: 4
    lll:
      line-length: 120
      tab-width: 4
    interfacebloat:
      max: 5
    revive:
      severity: error
      enable-default-rules: true
      rules:
        - name: file-length-limit
          arguments: [{ max: 400, skip-comments: true, skip-blank-lines: true }]
        - name: argument-limit
          arguments: [4]
        - name: function-result-limit
          arguments: [3]
        - name: max-control-nesting
          arguments: [3]
        - name: max-public-structs
          arguments: [5]
        - name: exported            # doc on exported identifiers, starting with the name; no stutter
          arguments: ["check-private-receivers", "check-public-interface"]
        - name: package-comments
        - name: bare-return         # no naked returns
        - name: early-return
        - name: indent-error-flow   # no else after return
        - name: superfluous-else
        - name: flag-parameter      # no bool parameters that switch behaviour
        - name: context-as-argument # ctx first
        - name: deep-exit
        - name: confusing-naming
        - name: import-shadowing
        - name: redefines-builtin-id
        - name: unexported-return
        - name: use-any
        - name: use-errors-new
        - name: identical-branches
        - name: comment-spacings    # `// text`, never `//text` (directives excepted)
    gocritic:
      enabled-tags: [diagnostic, style, performance]   # diagnostic includes commentedOutCode
      disabled-checks:
        - hugeParam        # pointer vs value is a design decision
        - rangeValCopy     # same
        - whyNoLint        # nolintlint already requires reasons
        - unnamedResult    # named results only when they document
        - importShadow     # revive import-shadowing covers it
        - paramTypeCombine # gofumpt group-params rewrites it
    tagliatelle:
      case:
        rules:
          json: snake
          yaml: snake
    sloglint:
      no-global: all        # use the logger from framework/log
      context: all          # InfoContext, never Info (request and trace correlation)
      static-msg: true
      msg-style: lowercased
      attr-only: true       # log.String, log.Err (framework/log attrs)
      key-naming-case: snake
      forbidden-keys: [time, level, msg, source]
    misspell:
      locale: US
    godot:
      scope: all
      capital: true
      exclude:
        - '^\s*\|'   # file banners: the banner frame is not a sentence
    godoclint:
      default: none
      enable: [single-pkg-doc, deprecated, max-len, no-unused-link]
      options:
        max-len:
          length: 80
    godox:
      keywords: [FIXME, XXX, HACK, BUG]
    nolintlint:
      require-explanation: true
      require-specific: true
      allow-unused: false

  exclusions:
    generated: strict       # sqlc and templ write the standard "Code generated ... DO NOT EDIT." header
    warn-unused: true
    # No presets: the "comments" preset would silence the doc comment rules.
    paths:
      - _templ\.go$
      - ^app/models/.*\.sql\.go$
    rules:
      # framework.md 7.1 exclusion rules are merged here.
      - path: ^framework/
        linters: [forbidigo]
        text: '\[panic\]'
      - path: ^framework/
        linters: [interfacebloat]   # driver contracts mirror Laravel's (filesystem.Filesystem)
      - path: _test\.go$
        linters: [funlen]           # test tables are long by nature; gocognit still applies
      - linters: [lll]
        source: https?://

issues:
  max-issues-per-linter: 0
  max-same-issues: 0

formatters:
  enable: [gofumpt, gci]
  settings:
    gofumpt:
      module-path: github.com/dimmihq/dimmi
      extra:
        group-params: true     # func f(a, b string)
        clothe-returns: true   # rewrites naked returns
    gci:
      sections:
        - standard
        - default
        - prefix(github.com/dimmihq/dimmi/framework)
        - prefix(github.com/dimmihq/dimmi)
      custom-order: true
  exclusions:
    generated: strict
```

Considered and rejected:

| Linter | Why not |
|---|---|
| wrapcheck, err113 | Demand a wrapper at every boundary and produce noise wrappers; 4.3 plus review instead |
| varnamelen, mnd, exhaustruct | Fight idiomatic short names, named constants and zero values |
| wsl, nlreturn | Blank-line rules too rigid; `whitespace` covers the useful part |
| cyclop, gocyclo, maintidx | One complexity metric only; `gocognit` measures what readers feel |
| dupl, goconst | High false-positive rate in controllers and tests |
| prealloc, perfsprint | Micro-optimizations; use `PERF:` with a benchmark when it matters |
| paralleltest, testpackage | Database tests run in a transaction each; `tests/` already separates feature tests |
| nonamedreturns, nakedret | Named results are allowed for documentation; revive `bare-return` bans naked returns |

### 2.5 Generated code

sqlc output (`app/models/*.sql.go`, `models.go`, `db.go`) and templ output (`*_templ.go`) carry the standard
generated header, so lint and size limits skip them. They are never edited by hand and never committed stale (CI step
2). huma writes nothing to disk; it builds OpenAPI at runtime from Go types.

### 2.6 Non-Go checks

**sqlc vet.** Rules live in `sqlc.yaml`. CI sets `database.uri` to the Postgres service after migrations; sqlc
expands `${...}` environment variables in the URI.

```yaml
sql:
  - engine: postgresql
    # schema, queries and overrides: framework.md 7.4
    database:
      uri: ${DIMMI_SQLC_DATABASE_URL}
    rules: [sqlc/db-prepare, tenant-filter]
rules:
  - name: tenant-filter
    message: Queries on tenant tables must filter by workspace_id.
    rule: "!query.sql.contains('workspace_id')"
```

`tenant-filter` is coarse on purpose. Queries on global tables (`workspaces`, `users`, `oauth_*`) and pre-tenant
lookups through `SECURITY DEFINER` functions opt out per query with `-- @sqlc-vet-disable tenant-filter` and a comment
saying why, so every exception is visible in review.

**Migrations (squawk).** `make lint-migrations` runs squawk on the `-- +goose Up` section of each migration changed on
the branch (the Down section drops things by design and runs only locally). `.squawk.toml` sets
`pg_version = "18.0"` and `assume_in_transaction = true`; migrations marked `-- +goose NO TRANSACTION` are passed with
`-- squawk-disable-assume-in-transaction` prepended. Default rules stay on, including `ban-drop-column`,
`ban-drop-table`, `renaming-column`, `changing-column-type`, `adding-required-field`,
`require-concurrent-index-creation` and `constraint-missing-not-valid`. A destructive statement is allowed only as the
contract step of an expand/contract change (README "Database rules"), annotated per statement:

```sql
-- Contract step for 20261101000004_add_conversations_closed_at (released in v1.4).
-- squawk-ignore ban-drop-column
ALTER TABLE conversations DROP COLUMN closed;
```

`-- squawk-ignore-file` is banned.

**OpenAPI.** `dimmi openapi:export` (specified in 00-foundation `api.md` "OpenAPI": no DB, no network, byte-identical output) writes `openapi/api-v1.json` and
`openapi/widget-v1.json`. CI exports both from `origin/main` and from the branch, then runs
`oasdiff breaking --fail-on ERR` on each pair. v1 changes are additive only (00-foundation `api.md`).

**templ.** Formatted in step 1. Locally, `templ fmt -fail .` exits 1 when it had to change a file.

**Markdown.** `markdownlint-cli2` with `.markdownlint-cli2.yaml` containing `config: { MD013: false }`: prose lines
are not hard-wrapped and tables cannot be. All other default rules apply.

**Comment rules (`make lint-comments`).** Each command is one rule from section 5; any output fails the target, and
the target prints every problem before exiting.

```sh
# Every Go package has doc.go.
go list -f '{{.Dir}}' ./... | while read -r d; do [ -f "$d/doc.go" ] || echo "missing $d/doc.go"; done
# Every non-test, hand-written Go file opens with a banner (doc.go holds the package comment instead).
git ls-files '*.go' ':!:*_test.go' ':!:*/doc.go' ':!:*_templ.go' ':!:app/models/*.sql.go' \
  | while read -r f; do head -2 "$f" | tail -1 | grep -q '^|---' || echo "missing banner $f"; done
# Block comments only, never inside a function body (go/ast walk; //go: and //nolint: directives allowed).
go run ./scripts/commentlint ./...
# Comments carry intent, not pointers to documents.
grep -rnE --include='*.go' 'specs/|research/' .
# TODOs name an owner and an issue.
grep -rnwE --include='*.go' --include='*.sql' --include='*.templ' 'TODO' . \
  | grep -vE 'TODO\([a-z0-9-]+\): #[0-9]+ '
# One suppression mechanism: //nolint.
grep -rnE --include='*.go' --include='*.sql' 'revive:disable|#nosec|lint:ignore|squawk-ignore-file' .
# templ renders HTML comments to the client.
grep -rn --include='*.templ' '<!--' .
```

`scripts/commentlint` is a small go/ast program: it reports any `//` comment that is not a `//go:` or `//nolint:`
directive, any comment whose position falls inside a function body (function literals included), and any comment on
the same line as code. It reads files outside the `generated: strict` set only.

### 2.7 Pre-commit hook

`lefthook.yml` runs a fast subset on staged files; the full `make lint test` runs on pre-push and in CI.

```yaml
pre-commit:
  parallel: true
  jobs:
    - name: go format
      glob: "*.go"
      run: golangci-lint fmt {staged_files}
      stage_fixed: true
    - name: templ format
      glob: "*.templ"
      run: templ fmt {staged_files}
      stage_fixed: true
    - name: go lint (new issues only)
      glob: "*.go"
      run: golangci-lint run --new-from-rev=HEAD ./...
    - name: migrations
      glob: "database/migrations/*.sql"
      run: make lint-migrations FILES="{staged_files}"
    - name: markdown
      glob: "*.md"
      run: markdownlint-cli2 {staged_files}
pre-push:
  jobs:
    - run: make lint test
```

### 2.8 Zero warnings and `//nolint`

- There are no warnings, only errors. `main` is always clean; a PR that adds an issue does not merge.
- `//nolint` is the only suppression mechanism. It names the linter, gives a reason, and covers the narrowest scope
  (one line or one function, never a file):

  ```go
  //nolint:gocognit // Mirrors the RFC 5322 address grammar; splitting it hides the mapping.
  ```

- A bare `//nolint`, `//nolint:all`, a missing reason and an unused directive all fail `nolintlint`.
- CI posts the count per linter to the job summary
  (`grep -rhoE --include='*.go' '//nolint:[a-z0-9,]+' . | sort | uniq -c`). A PR that raises the count needs the
  reviewer to approve each new directive explicitly.

## 3. Size limits

Soft limits are where a reviewer asks "should this be split?". Hard limits fail `make lint`. Generated code is
excluded. Counts skip comments and blank lines where the tool allows it, so documentation never pushes code over a
limit.

| What | Soft (review) | Hard (lint) | Enforced by |
|---|---|---|---|
| File length | 250 code lines | 400 code lines | revive `file-length-limit` |
| Function length | 30 lines, 20 statements | 50 lines, 30 statements (tests: statements only) | funlen |
| Cognitive complexity | 10 | 15 | gocognit |
| `if` nesting complexity | 3 | reported at 4 | nestif |
| Control-flow nesting depth | 2 | 3 | revive `max-control-nesting` |
| Parameters (including `ctx`) | 3 | 4 | revive `argument-limit` |
| Results (including `error`) | 2 | 3 | revive `function-result-limit` |
| Exported structs per file | 3 | 5 | revive `max-public-structs` |
| Line length (tab = 4 columns) | 100 | 120 | lll |
| Doc comment line (text only) | 80 | 80 | godoclint `max-len` |
| Interface methods (`app/`) | 1-3 | 5 | interfacebloat |
| Package size | 20 non-test files | none | review |
| Controller method | 15 lines | 50 (function limit) | review |

When a limit is hit, change the shape of the code, not the limit:

- **File too long:** split by responsibility. A long model file splits by concern (`conversation.go`,
  `conversation_status.go`, `conversation_snooze.go`).
- **Function too long or complex:** extract steps whose names read like the spec (`ensureCanReply`,
  `notifyWatchers`). Replace flag-driven branches with a `switch` over an enum or a lookup table.
- **Nested too deep:** invert conditions into guard clauses (4.2).
- **Too many parameters:** pass an input struct (`ReplyInput`). Optional settings become an options struct whose zero
  value means "defaults"; no functional options in `app/`.
- **Too many results:** return a named struct (`type ImportResult struct { Created, Skipped int }`).
- **Interface too big:** split by role (`Reader`, `Writer`) and embed where a caller needs both. Define interfaces
  where they are consumed.
- **Package too big:** split by sub-domain (`actions/conversations/merging`).

Raising a hard limit is a change to this file, not a `//nolint`, unless the reason is specific to one function.

## 4. Readable code

### 4.1 Naming

- README "Naming" is the source for Laravel-shaped names (controllers, actions, events, jobs, routes, tables). Use
  the spec's words: `Conversation`, `Contact`, `Snooze`, never `Thread`, `User` or `Pause` for the same thing.
- `MixedCaps`; initialisms keep one case (`ConversationID`, `HTMLBody`, `apiURL`); no `Get` prefix on getters;
  boolean functions read as questions (`IsClosed`, `HasUnread`, `CanReply`).
- Package names are short, lowercase, without underscores. Do not start a name with its package's name:
  `enums.ConversationStatus`, not `enums.EnumConversationStatus` (revive flags the stutter).
- Receivers are one or two letters and the same on every method of a type (`c *ConversationController`).
- Scope sets length: `i` in a three-line loop, `conversation` across a function. Never `data`, `info`, `obj`, `tmp`,
  `mgr`, `util` or `helper`.

### 4.2 Control flow

Handle the unusual case first and return. The happy path runs down the left edge with no `else`.

Bad: the rule hides in an else branch and the reader tracks three levels.

```go
func (a *ReplyToConversation) Handle(ctx context.Context, in ReplyInput) (*models.Message, error) {
	if in.Conversation.Status != enums.ConversationStatusClosed {
		if len(in.Body) > 0 {
			return a.create(ctx, in)
		} else {
			return nil, exceptions.ErrEmptyReply
		}
	} else {
		return nil, exceptions.ErrConversationClosed
	}
}
```

Good: guards first, then the one thing the function exists to do.

```go
func (a *ReplyToConversation) Handle(ctx context.Context, in ReplyInput) (*models.Message, error) {
	if in.Conversation.IsClosed() {
		return nil, exceptions.ErrConversationClosed
	}
	if in.Body == "" {
		return nil, exceptions.ErrEmptyReply
	}

	return a.create(ctx, in)
}
```

### 4.3 Errors

- Domain errors are sentinels in `app/exceptions`, built with `fault.New(kind, code, msg)` (framework.md), and are
  returned unwrapped so the HTTP renderer maps them to status and `code`. Errors carrying data are types ending in
  `Error`.
- Every other error is wrapped once per layer with what was being attempted: lowercase, no "failed to", no final
  punctuation. `fmt.Errorf("close conversation %s: %w", id, err)`.
- Compare with `errors.Is` and `errors.As` only.
- Never log and return. The boundary that handles the error logs it once (HTTP renderer, job runner, console).
  Logging and returning reports one failure twice with different context.
- `_ = f()` is allowed only when the enclosing function's docblock says why the failure does not matter.

### 4.4 Functions and types

- `ctx context.Context` is the first parameter of anything that does I/O, and is never stored in a struct.
- No naked returns. Named results only when two results of one type need names to be understood.
- No boolean parameters that switch behaviour. Write two functions or use a named type:
  `Archive(ctx, id, enums.ArchiveModeSoft)`, not `Archive(ctx, id, true)`.
- Enums are typed strings with a `Valid()` method. The zero value `""` is invalid and rejected at the edge. Enum
  switches list every value (exhaustive) and carry no `default` that would hide a new value.

  ```go
  /*
  ConversationStatus is where a conversation sits in the inbox workflow.
  */
  type ConversationStatus string

  /*
  The conversation statuses, in workflow order. A snoozed conversation
  reopens on its own when the snooze ends.
  */
  const (
  	ConversationStatusOpen    ConversationStatus = "open"
  	ConversationStatusSnoozed ConversationStatus = "snoozed"
  	ConversationStatusClosed  ConversationStatus = "closed"
  )
  ```

- Make zero values useful where it is cheap (`var buf bytes.Buffer`, an options struct whose zero value means
  defaults).

### 4.5 State and wiring

- No package-level variables except `Err*` sentinels, compiled regexps and `//go:embed` values (the gochecknoglobals
  exceptions). No `init()` anywhere (framework.md 7.1).
- Dependencies are explicit struct fields set by the provider that builds the type
  (`&conversations.CloseConversation{DB: a.DB, Events: a.Events, Clock: a.Clock}`, framework.md 5). Actions never
  hold the whole `*App`: hidden dependencies make every test build the world.
- No goroutines in `app/`. Background work is a job; fan-out is several jobs.
- Keep the linker's dead method elimination. A function that looks up a method by a name known only at run time
  (`reflect.Value.MethodByName` or `Method` with a non-constant argument, executing `text/template` or
  `html/template`, and libraries built on them) makes the linker keep every exported method of every type, which
  roughly doubles the binary. Such code stays out of every path a running application reaches; dev-only code behind
  the `dev` build tag (the `make:*` stubs) may use `text/template`. `tests/arch` builds a small application with
  `-ldflags=-dumpdep` and fails on any `<ReflectMethod>` marker. Prefer a library's small module over the whole
  library, and never add a large dependency for one helper.

### 4.6 Layers

- Controllers stay thin (README "Request lifecycle"): parse, authorize, call one action, return a resource.
- An action is one exported type per file with one exported method, `Handle(ctx, input) (output, error)`. Helpers are
  unexported methods in the same file; shared helpers belong on the model or in `app/support`.
- SQL lives only in `database/queries`. Business rules live in actions and model methods, not in SQL triggers.

### 4.7 Tests

- One top-level test per unit (`TestCloseConversation`, `TestConversationReplyController_Store`) with subtests named
  as lowercase sentences, so output reads as a spec:
  `TestCloseConversation/rejects_an_already_closed_conversation`.
- Table-driven when three or more cases share one shape; plain subtests otherwise.
- Helpers call `t.Helper()`. Use `t.Context()` and a `clock.NewFake(...)` passed where the code takes a clock;
  never real time or a fresh context.
- Fakes only at process boundaries (testing.md section 4): mail transport, third-party HTTP (`httpclienttest`),
  DNS, malware scanners, social login providers, LLM providers and the clock. Everything else runs for real against
  the services in `compose.yaml`.
- Comments in tests follow 5.9: the subtest name is the only description of a test.

## 5. Comments

A comment states business intent: why the code exists, what must stay true, and which choices would look wrong
without the reason. Each comment is read hundreds of times, by humans and by agents, so each one must earn its place.

### 5.1 Where comments go

Go code carries comments in exactly three places, all written as block comments (`/* ... */`).

| Place | Shape | Holds |
|---|---|---|
| File banner | Laravel config-style banner (5.3), first thing in every non-test Go file, blank line before `package` | Why the file exists and what must stay true, in business words |
| Package doc | `doc.go`: a short plain `/* Package x ... */` attached to the `package` clause (5.4) | One or two sentences on what the package is for |
| Docblock | `/* ... */` directly above a declaration (5.5) | The behavior and intent of that identifier |

Nowhere else:

- No comments inside function bodies, ever. If a step needs explaining, the explanation goes in the function's
  docblock, or the step is renamed or extracted into a function whose name says it.
- No `//` comments at all. The only `//` lines are `//go:` directives (`//go:embed`, `//go:build`, `//go:generate`),
  `//nolint:` directives (2.8), and code examples quoted inside a docblock.
- No end-of-line comments, on fields, constants or anything else.

Test files follow the same rules without the banner requirement (5.9).

### 5.2 What a comment says

Say:

- **Why**, when the reason is not visible: a business rule, a past incident described as a rule ("a slow upload must
  never leave half a file"), a provider quirk.
- **Invariants** that callers and future editors must keep ("rows come back newest first").
- **Non-obvious constraints**: ordering, locking, idempotency, time zones, third-party limits.
- **Security reasoning** and **performance trade-offs**, tagged (5.6).
- **External standards** by name when behavior follows one: `[RFC 5322]`, a provider's documented limit.

Never say:

- What the next line does. Rename until the code says it.
- Where the rule is written down. Comments never reference spec or research files, section numbers, tickets or
  issue threads; the comment states the rule itself in plain words. The one exception is the issue number on a
  `TODO` line (5.6).
- History (`changed in v1.3`, `used to call X`). `git log` holds history.
- Authors, dates, apologies (`hacky but works`). Fix it, or file an issue and write a `TODO`.
- Commented-out code. gocritic flags it; git keeps it.
- A restatement of the name (`Close closes.`). When the name says everything, the docblock says what the name cannot:
  the contract, the failure modes, the reason.

### 5.3 File banner

Every non-test Go file opens with a banner. Generated files (sqlc, templ) are exempt, and so is `doc.go`, which
holds the package comment instead. The banner states the business intent of the file: why it exists, what must stay
true, and the decisions a reader would otherwise question. It is not a list of the file's functions.

The exact shape, from `framework/cache/store.go`:

```go
/*
|--------------------------------------------------------------------------
| Cache Store
|--------------------------------------------------------------------------
|
| The cache only ever saves work. Every value expires on its own, so the
| store can be flushed or lost at any moment and the application simply
| computes again. A store lives in its own keyspace under the client's
| prefix, which lets several stores share one Valkey without ever reading
| or flushing each other's keys.
|
| Every command touches a single key, so the store behaves the same on a
| single server and on a cluster.
|
*/

package cache
```

- `/*` alone on the first line, then the frame: `|` followed by 74 dashes. Then `| ` and the title in Title Case
  (`Cache Store`, `Transactions`, `Close Conversation`), then the frame again and a bare `|`.
- Prose lines start with `| ` and wrap so no line passes the frame (75 columns). Paragraphs are separated by a bare
  `|`. The last line before `*/` is a bare `|`.
- Full sentences. godot skips every line that starts with `|` (2.4), so the frame passes lint and review keeps the
  prose to sentence rules.
- A short configuration or usage example may sit in the banner, indented two spaces after `| ` (see
  `framework/filesystem/manager.go`).
- A blank line separates `*/` from the `package` clause. Without it the banner would become the package comment.
- One banner per file. A file that needs two banners is two files.

### 5.4 Package doc

Every package has a `doc.go` that contains only the package comment and the `package` clause. The comment is a short
plain block comment attached to the clause, with no banner and no headings:

```go
/*
Package cache stores short-lived values in Valkey, shared by every process.
*/
package cache
```

Longer reasoning about the package belongs in the banners of the files that implement it.

### 5.5 Docblocks

- Every exported identifier has a docblock, and so does every unexported one whose name and signature do not say
  enough (`incrementScript` in `framework/cache/store.go` explains why the expiry is set in the same round trip; the
  one-line `key` helper beside it needs nothing).
- The docblock sits directly above the declaration, starts with the identifier's name, and is written in full
  sentences ending with a period, at most 80 columns of text per line (godoclint `max-len`).
- It describes behavior and intent: what the caller can rely on, what fails and how, why it works this way. Boolean
  functions "report whether". It does not describe the implementation line by line.
- Grouped `const` and `var` declarations get one docblock above the group that explains the members together (the
  `Err*` sentinels in `framework/cache/store.go`). Members carry no comments of their own.
- Struct fields are explained in the type's docblock, in prose, never with end-of-line or per-field comments. Fields
  whose names say everything are not mentioned.
- Interface methods get their own docblocks, indented one tab with the method and their text indented one more:

  ```go
  type Filesystem interface {
  	/*
  		Delete removes the file at the path.
  	*/
  	Delete(ctx context.Context, path string) error
  }
  ```

- gofmt does not reformat block comments, so the layout is written by hand, using Go doc syntax:
  - Text flush with the comment markers. Paragraphs are separated by a blank line.
  - Lists are indented two spaces: `  - item` or `  1. item`. No nested lists.
  - Code examples are indented one tab. A `//` inside a code example is part of the example, not a comment.
  - Doc links: `[CloseConversation]`, `[appevents.ConversationClosed]`, `[io.EOF]`. Prefer them to backticks so a
    rename shows up in `go doc`.
  - URL links: `[RFC 5322]` in the text and `[RFC 5322]: https://www.rfc-editor.org/rfc/rfc5322` on its own line at
    the end of the docblock.
  - Deprecations: a paragraph starting with `Deprecated:`.

Templates fix the order of information in common docblocks. Leave out a section that would be empty; never write
"Events: none".

- **Action:** what it does in domain words, then `Rules:`, `Events:`, `Errors:` (full example in 5.7).
- **Job:** what it does, then `Queue:`, `Unique:`, `Retries:`, `Idempotency:` (what happens if it runs twice).

```go
/*
DeliverWebhookWorker posts one event to one subscriber endpoint.

Queue: webhooks. Unique: by delivery ID, so replaying an event while an
attempt is pending does not enqueue a second one.

Retries: up to MaxAttempts in [DeliverWebhookArgs.Options], then the
delivery is marked failed and counts toward disabling the endpoint.

Idempotency: a retry after a timeout can deliver twice, so receivers
dedupe on the event ID header.
*/
type DeliverWebhookWorker struct {
```

Non-Go files keep their own comment syntax with the same content rules (intent only, no document references):

**Migration header.** What and why, and the expand/contract step.

```sql
-- Adds conversations.closed_at so reports can measure time to close.
-- Expand step: nullable with no default, so old code ignores it and the
-- ALTER does not rewrite the table.

-- +goose Up
ALTER TABLE conversations ADD COLUMN closed_at timestamptz;

-- +goose Down
ALTER TABLE conversations DROP COLUMN closed_at;
```

**sqlc query.** The `-- name:` line, then one or two lines starting with the query name. sqlc places comment lines
that follow the annotation above the generated Go method, so they become its doc comment (generated code is exempt
from the `//` rule).

```sql
-- name: CloseConversation :one
-- CloseConversation closes an open or snoozed conversation and clears its
-- snooze. It returns no row when the conversation is already closed.
UPDATE conversations
SET status = 'closed', closed_at = @closed_at, snoozed_until = NULL, updated_at = now()
WHERE workspace_id = @workspace_id AND id = @id AND status <> 'closed'
RETURNING *;
```

**templ component.** A block comment above the `templ` declaration saying what it renders and for whom. Never put HTML
comments (`<!-- -->`) inside a component: templ renders them into the output, so they reach browsers and inboxes.

```templ
/*
ConversationReplyMail is the email a contact receives when an agent
replies. ConversationReplyText is its plain-text twin; change both.
*/
templ ConversationReplyMail(m ConversationReplyView) {
```

### 5.6 Tags

Tags start a paragraph inside a docblock or banner. This is the complete set. `NOTE:`, `IMPORTANT:`, `XXX`, `HACK`,
`BUG` and `FIXME` are not used; godox rejects the last four.

| Tag | Use | Example line |
|---|---|---|
| `TODO(handle): #issue` | Known gap with an owner and an issue, both required | `TODO(bahdcoder): #142 Replace polling with LISTEN/NOTIFY.` |
| `SECURITY:` | Deliberate code that looks wrong or slow; audits grep for it | `SECURITY: Compare in constant time; == leaks the match length.` |
| `PERF:` | Readability traded for speed, with the measurement | `PERF: One batch insert; per-row calls were 40x slower (BenchmarkImport).` |
| `Deprecated:` | Go's deprecation paragraph | `Deprecated: Use [AssignConversation] instead.` |

A tag explains the function it sits on. When it is about one step, the docblock names the step:

```go
/*
VerifySignature reports whether the signature matches the payload.

SECURITY: The comparison runs in constant time; == would leak how many
leading bytes matched.
*/
func VerifySignature(secret, payload, signature []byte) bool {
```

### 5.7 A complete example

One package, `app/actions/conversations`, shown as its `doc.go` and one action file. Imports are omitted; framework
APIs follow framework.md.

`doc.go`:

```go
/*
Package conversations holds the business actions of the shared inbox:
replying to, assigning, snoozing, closing and merging conversations.
*/
package conversations
```

`close_conversation.go`:

```go
/*
|--------------------------------------------------------------------------
| Close Conversation
|--------------------------------------------------------------------------
|
| Closing takes a conversation out of the open queue and stops its SLA
| timers. Two agents often close the same conversation at the same moment,
| and only one of them may produce the closed event, or listeners would
| act twice. So the status check lives in the UPDATE statement, not in
| Go, and the database decides who wins.
|
| A pending snooze is cleared by the same statement; otherwise its wake-up
| job would reopen the conversation later.
|
*/

package conversations

/*
CloseConversation marks a conversation as closed.

Rules:
  - Only open or snoozed conversations can be closed. Closing a closed
    conversation returns [exceptions.ErrConversationClosed], so a double
    click becomes a 409 instead of a second event.
  - A pending snooze is cleared.

Events: [appevents.ConversationClosed], delivered after commit.

Errors: [exceptions.ErrConversationClosed], or a wrapped database error.
*/
type CloseConversation struct {
	DB     *database.DB
	Events *events.Dispatcher
	Clock  *clock.Manager
}

/*
CloseInput is the input to [CloseConversation.Handle]. Actor is recorded
on the event as the member who closed the conversation.
*/
type CloseInput struct {
	Conversation *models.Conversation
	Actor        *models.Member
}

/*
Handle closes the conversation and returns the updated row.

When the UPDATE matches no row, another request closed the conversation
first, and the caller gets [exceptions.ErrConversationClosed] instead of
a second event.
*/
func (a *CloseConversation) Handle(ctx context.Context, in CloseInput) (*models.Conversation, error) {
	var closed models.Conversation

	err := a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
		row, err := models.New(tx).CloseConversation(ctx, models.CloseConversationParams{
			WorkspaceID: in.Conversation.WorkspaceID,
			ID:          in.Conversation.ID,
			ClosedAt:    a.Clock.Now(),
		})
		if errors.Is(err, database.ErrNotFound) {
			return exceptions.ErrConversationClosed
		}
		if err != nil {
			return fmt.Errorf("close conversation %s: %w", in.Conversation.ID, err)
		}

		closed = row

		return a.Events.Dispatch(ctx, tx, appevents.ConversationClosed{
			ConversationID: row.ID,
			ClosedBy:       in.Actor.ID,
		})
	})

	return &closed, err
}
```

What is not commented matters as much: the dependency fields, the obvious input field, the error wrapping, and every
line inside `Handle`. The race the code cannot show is explained once, in the banner, and the caller-visible
consequence is in the docblock.

### 5.8 Bad comments, rewritten

```go
// ==================== HELPERS ====================

// getStatus - gets the status
// Author: jdoe, 2026-03-02
// Updated to handle snoozed (see slack thread)
func getStatus(c models.Conversation) string {
	// check if snoozed
	if c.SnoozedUntil != nil {
		// if snooze is past, it's open
		if c.SnoozedUntil.Before(time.Now()) {
			return "open"
		} else {
			return "snoozed"
		}
	}
	// return c.Status
	return string(c.Status) // TODO fix this
}
```

```go
/*
effectiveStatus reports the status an agent should see. A snooze whose
time has passed counts as open before the wake-up job runs, because the
job can lag by up to a minute under load.
*/
func effectiveStatus(c *models.Conversation, now time.Time) enums.ConversationStatus {
	if c.SnoozedUntil == nil {
		return c.Status
	}
	if now.After(*c.SnoozedUntil) {
		return enums.ConversationStatusOpen
	}

	return enums.ConversationStatusSnoozed
}
```

The rewrite drops the divider, the restated name, the author and history lines, the pointer to a chat thread, the
comments narrating each branch, the commented-out code and the untracked TODO. It adds the one fact the code cannot
show, in the docblock: why a past snooze counts as open before the job runs. Taking `now` as a parameter and
returning the enum makes it testable and self-describing.

### 5.9 Comments in tests

- Test files need no banner, but every other rule applies: block comments only, nothing inside function bodies.
- The subtest name is the only description of a test. If a test needs a sentence to explain what it checks, that
  sentence is its name: `t.Run("removes every key in the store, past one scan batch", ...)`. A regression test is
  named for the behavior that broke, not for the issue.
- Test helpers, test constants and shared fixtures get `/* */` docblocks when their purpose is not obvious:

  ```go
  /*
  Test TTLs. Short ones leave room for two calls in a row even on a loaded
  CI machine running with -race.
  */
  const (
  	long  = time.Minute
  	short = 250 * time.Millisecond
  )
  ```

- Given/When/Then is optional: separate setup, action and assertions with blank lines. Setup that is not obvious
  moves into a named helper whose docblock says why ("both messages share a timestamp, so ordering falls back to
  ID").

### 5.10 Enforcement

| Rule | Check |
|---|---|
| Banner opens every non-test, hand-written Go file | `make lint-comments` |
| Package comment present, well formed, one per package, in `doc.go` | revive `package-comments`, godoclint `single-pkg-doc`, `make lint-comments` |
| Docblock on every exported identifier, starting with its name; no stutter | revive `exported` |
| Block comments only; nothing inside function bodies; no end-of-line comments | `make lint-comments` (`scripts/commentlint`) |
| Sentences with capital letter and final period (banner lines exempt) | godot (banner `exclude`, 2.4) |
| Doc lines at most 80 columns; no unused links; deprecation format | godoclint |
| No commented-out code | gocritic `commentedOutCode` |
| No references to `specs/` or `research/` | `make lint-comments` |
| `TODO(handle): #issue`; no FIXME, XXX, HACK, BUG | `make lint-comments`, godox |
| No HTML comments in templ | `make lint-comments` |
| Comments state business intent; no history, authors, tickets or narration | review checklist |

## 6. Review checklist

Copied into `.github/pull_request_template.md`. Linters cover the rest; reviewers spend their time here.

- [ ] Names use the spec's words; no `data`, `info`, `util`, `manager`.
- [ ] Controllers are thin; business rules live in one action with one `Handle`.
- [ ] Guard clauses first; no `else` after `return`; no boolean behaviour flags.
- [ ] Domain errors returned as sentinels; other errors wrapped with context; nothing logged and returned.
- [ ] Every new query filters by `workspace_id`, or its opt-out says why.
- [ ] Migrations are expand/contract safe; destructive statements name their expand step.
- [ ] Jobs are idempotent and their docblock says what happens when one runs twice.
- [ ] Each new file's banner states its business intent: why it exists and what must stay true.
- [ ] New packages have `doc.go`; exported identifiers have docblocks that describe behavior and intent.
- [ ] No comments inside function bodies; steps that need explaining are named or explained in the docblock.
- [ ] Comments never point at specs, research, section numbers, tickets or history.
- [ ] Each new `//nolint` is justified and explicitly approved.
- [ ] Tests cover happy path, validation, authorization (other workspace returns 404) and each `Rules:` item.
- [ ] Spec updated in the same PR when behaviour changed.

## 7. AI coding agents

- `AGENTS.md` at the repository root summarizes README.md, framework.md and this file and links to all three.
  `CLAUDE.md` is a symlink to it, so every agent reads the same rules.
- Where this file and an agent's defaults disagree, this file wins.
- A task is not done until `make lint test` passes with zero issues. Agents never add `//nolint`, raise a limit or
  edit `.golangci.yml` to get there; they change the code, or stop and ask.
- Agents follow section 5: a banner on every new non-test file, `doc.go` for every new package, `/* */` docblocks on
  declarations, and nothing else. No comments inside function bodies, no `//` comments, no comments narrating their
  own edits ("now uses X", "added for Y"), no history, no restating code, and no references to the spec, research
  file, section or task that prompted the change: the comment states the business rule itself.
- Agents use the generators (`make gen ARGS="make:action tags/CreateTag"` and the other `make:*` commands of 2.1),
  whose output already passes `make lint`, instead of hand-writing boilerplate. Dev-only commands go in files with
  `//go:build dev`; `make lint` and `make test` already cover both builds.
- Agents keep the binary small (4.5): no method lookups by run-time name, no `text/template` in code a running app
  reaches, the smallest module of a library that does the job.
- Agents test against the real services (`make services`, then `make test`) and add fakes only at process
  boundaries; a new fake refuses to run outside the testing environment and passes the driver's `drivertest` suite.
- Agents never run tests, evals or harnesses that call paid third-party APIs (LLM providers above all) without the
  user's approval for that run.
