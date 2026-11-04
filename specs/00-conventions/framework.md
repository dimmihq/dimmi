# Framework / application split

Date: 2026-11-02. Style: caveman. Scope trim 2026-11-02: inbound mail SES only (IMAP, provider inbound drivers later), Discord notifications later. Binding for all Go code. Read with `README.md` (this folder). Laravel parallel: `framework/` = Illuminate, everything else = Laravel app skeleton. Synced with built `framework/` code 2026-11-02: code (package `doc.go` + exported API) wins on conflict.

## 1. Why

- Laravel app code never touches Symfony, Monolog, Flysystem, PHPMailer directly. It calls `Hash::make`, `Storage::disk`, `Mail::to`. Dimmi same: app code calls `app.Hash.Make`, `app.Storage.Disk`, `app.Mail.To`.
- Reuse before build (2.5): framework never re-implements what a maintained library does; it wraps it.
- One place decides each library. Swap argon2 lib, S3 SDK, River version: one package changes, app untouched.
- Fakes only at true process boundaries, same shape everywhere (`a.Mail.Fake()`); everything with a real local option runs real in tests (Postgres, Valkey, SeaweedFS S3, Mailpit from root `compose.yaml`). Every driver passes same contract suite. Coding agents learn one pattern.
- Binary size is a rule (PO 2026-11-02): no library that disables the Go linker's dead method elimination (method lookup by runtime name, e.g. `text/template`, go-playground/validator, creasty/defaults, gocloud.dev, go-i18n). Removing them took a realistic app binary from 63.9 MB to 36.0 MB stripped (12.3 MB gzipped). `tests/arch` binary test guards it (7.2).
- `framework/` could become own module (`github.com/dimmihq/framework`) later with zero app edits beyond import path rewrite.

## 2. Rules

### 2.1 Dependency direction

```
cmd/dimmi ──> bootstrap ──> routes ──> app ──> config ──> framework ──> vetted third-party libs
                 │             │        │                    ▲               + Go stdlib
                 │             │        ├─ generated code ───┘ (sqlc in app/models, templ in resources/views)
                 │             │        └─ adapter zone (app/drivers, app/services) ──> framework + ONE vendor SDK per dir
                 └─────────────┴──> database/{factories,seeders}, tests ──> app + framework/testkit
framework ──X──> app, routes, database, config, bootstrap, cmd, tests, resources   (never)
```

- Module path `github.com/dimmihq/dimmi`. Framework import path `github.com/dimmihq/dimmi/framework/<pkg>`.
- Framework imports only stdlib, other framework packages, vetted libs (list in 7.1). No cycles between framework packages (`go vet` catches; layering in 3.1).
- App zone (`app/` minus adapter zone, `routes/`, `config/`, `bootstrap/`, `database/`, `tests/`) imports: stdlib minus denylist (7.1), `framework/*`, own module packages, generated code. Third-party allowlist for app zone: **empty**.
- Adapter zone (`app/drivers/<contract>/<driver>/`, `app/services/<vendor>/`): app zone rules + exactly the vendor SDK named for that directory in 7.1. Must take `*http.Client` from `app.HTTP.StdClient()` (SSRF-safe transport), SDK retries off. No vendor type crosses adapter package boundary (exported API uses app/framework types only).
- Generated code exempt from import rules (golangci-lint skips files with `Code generated ... DO NOT EDIT.`). Hand-written code in same package (`app/models`) still bound: sqlc overrides (7.4) keep generated structs on stdlib/framework types so hand-written code never names `pgtype`.
- `cmd/dimmi/main.go`: imports `bootstrap` + `os` only. Five lines.

### 2.2 Framework or app? One test

"Would a different SaaS product (CRM, project tracker, help desk) need this code **unchanged**?"
- Yes → `framework/`. Examples: password hashing, S3 disk, realtime server on centrifuge, Standard Webhooks signing, MIME parse, OAuth PKCE, TOTP, Vite manifest, MCP server plumbing.
- No, uses product words or product tables → `app/`. Examples: conversation threading, spam heuristics, saved-reply templating, metering rules, channel drivers, prompt files.
- Also framework, even if small: anything wrapping a third-party lib or a denied stdlib primitive (`crypto/*`, `os/exec`, `net`, `database/sql`). App zone cannot import those, so wrapper must live in framework (or adapter zone for vendor SDKs of domain contracts).
- Not framework: pure-Go product helpers with no third-party deps (`app/support/searchquery`, `officehours`). Stay in `app/support`. Product logic over a wrapped library also stays app (`app/support/textdiff`: hunk model + review rules; diff engines come from `framework/support/diff`).

### 2.3 Contracts split

| Kind | Where | Members |
|---|---|---|
| Cross-cutting, concrete (one backend) | type in its framework package, no interface | `cache.Store` (Valkey only), `database.DB` (Postgres only), `redis.Client`, `pubsub.Bus`, `ratelimit.Limiter`, `queue.Queue` (River only, thin), `httpclient.Client`, `dns.Client`, `telemetry.Telemetry`, `log.Logger` (= `*slog.Logger`), `oauth.Server` |
| Cross-cutting (any SaaS) | interface in its framework package | `filesystem.Filesystem`, `mail.Mailer`, `mailbox.Source`/`Receiver`, `encryption.Encrypter` + `KEK` + `KeyManager` + `KeysetStore`, `clock.Clock`, `hashing.Hasher`, `validation.Rule`, `broadcasting.Broadcaster`, `notifications.Channel`, `malware.Scanner`, `dns.Resolver`, `auth.Guard` + `TokenResolver` + `PermissionResolver`, `session.Store`, `auth/token.Store`, `idempotency.Store`, `socialite.Provider`, `customdomain.TLS`, `i18n.Detector` |
| Domain (Dimmi only) | `app/contracts/` | `Channel` (03), `Knowledge` (08), `LLM` + `Embeddings` (07/08), `BillingStatusReader` (12), `Search`, `Actor`, `ContactEraser`, `WorkspaceInitializer` |
| Domain managers | `app/managers/` on `framework/manager` | `ChannelManager`, `LLMManager`, `EmbeddingsManager` |

### 2.4 No business words in framework

Banned stems in framework identifiers, string literals, comments, file names (case-insensitive, camelCase split): `workspace`, `conversation`, `contact`, `inbox`, `teammate`, `widget`, `helpcenter`/`help_center`, `article`, `ticket`, `assignee`, `snooze`, `knowledge`, `messenger`, `intercom`, `dimmi`. Exception: import paths (`github.com/dimmihq/dimmi/framework/...`). Generic SaaS words allowed: `tenant`, `user`, `member`, `presence`, `channel` (Go + broadcasting), `message` (mail), `account`, `edition`.
- Product names, env prefix, default ports, cookie names: passed in by app (`foundation.Options{Name: "dimmi", EnvPrefix: "DIMMI_"}`).
- Tenant concept: `framework/tenancy` (context key, RLS setting name from config). App maps workspace → tenant in `workspace` middleware.
- Enforcement: `tests/arch/framework_words_test.go` (go/ast walk via `framework/testkit/arch`, zero allowlist; file names included) + planned `scripts/framework-words.sh` (`git grep -nIiwE` on non-Go files: stubs, SQL, testdata) in CI. forbidigo cannot check declarations, so not used for this.

### 2.5 Reuse before build

Product owner rule (binding): never write code a mature open source library already provides. Framework = thin wrappers over libraries + glue that makes them Laravel-shaped, swappable, fakeable.

- Every catalog row (3.2) names wrapped library in column "Wraps". Value `own:` allowed only with one-line justification of one kind: (a) no maintained Go library, (b) library too heavy (binary size, deps, storage coupling), (c) library cannot meet a written spec requirement (cite spec), (d) glue under ~150 lines with no algorithm (manager, context keys, error kinds).
- New framework package PR: catalog row first. Reviewer checks pkg.go.dev + GitHub for a maintained alternative (release in last 12 months, license MIT/BSD/Apache/MPL/ISC). Found one: use it.
- License (PO): Dimmi ships under MIT. Every dependency must be MIT-compatible: MIT, BSD, ISC, 0BSD, Apache-2.0, MPL-2.0 (unmodified files). No GPL, LGPL, AGPL dependencies. CI `go-licenses` check blocks others.
- Port rule: never copy code from another project's server (e.g. Centrifugo `internal/`) without PO approval; prefer library public API + small own glue.
- Maintained = release or meaningful commit in last 12 months, issues answered. Library going stale: open `framework/rfcs/` note, pick replacement or fork-and-vendor decision.
- Wrapper stays thin: translate config, map errors to framework errors, hide library types, add `Fake()` only at process boundaries (3). No re-implementation of library features. Exception: own code allowed (reason (b)) where the library breaks dead method elimination (config `default` tags, `validate` tags, i18n loader).
- `[verify]` marks a library choice whose fit (API, maintenance, license) the implementer must confirm before first commit; on failure fall back to named alternative or `own:` with justification in PR.

## 3. Package catalog

Conventions for every service package:
- `Manager` (pointer on `*App`) built on `framework/manager`. `Disk(name)` / `Mailer(name)` (on `manager.Instance(name)`) resolves named instance from config, caches it. Instance name ≠ driver name (disk `attachments` uses driver `s3`).
- `m.Extend("driver", factory)` registers driver (built-ins registered by framework provider; community drivers in `app/providers/app_service_provider.go`). No package-level registry, no `init()`.
- Fakes only at true process boundaries (PO 2026-11-02): outbound HTTP (`httpclienttest.New(a.HTTP)`, subpackage keeps httpmock out of production binaries), DNS (`a.DNS.Fake()`), mail transport (`a.Mail.Fake()`), malware scanners (`a.Scanner.Fake()`), socialite providers (`a.Social.Fake()`), clock (`clock.NewFake(t)` with `Freeze`/`Travel`, handed to the service under test). Each `Fake()` swaps in place and returns the fake; services captured at construction hold the same pointer, so the fake reaches every action. Every fake panics with `ErrFakesNotAllowed` unless config `AllowFakes` is set, which foundation sets only when `app.env` is `testing`. Fakes live in `<pkg>/fake.go`, assertions take `testing.TB`. App-level LLM/channel fakes are domain drivers (`fake`), not framework.
- No fakes for database, cache, filesystem, queue, events, broadcasting, pubsub, ratelimit, encryption, hashing, process, log, telemetry, notifications, mcp. Tests use the real services from root `compose.yaml` (Postgres pgvector pg18 on 15432, `max_connections` 300; Valkey 8 on 16379; SeaweedFS S3 on 18333; Mailpit SMTP 11025 / API 18025) with env from `.env.testing`, and assert on real effects (rows, job rows, Valkey state, received broadcasts, Mailpit messages). testing.md.
- Single-backend services (cache on Valkey, database on Postgres) skip the manager: one concrete type, no driver interface.
- Drivers in `framework/<pkg>/drivers/<name>/`. App never imports driver packages (arch test).
- Contract suite `framework/<pkg>/drivertest` (filesystem, mail, mailbox, malware, hashing, encryption, dns, httpclient, socialite, customdomain, i18n detectors). Every built-in and community driver must pass; where a fake exists it passes too (keeps fakes honest). Suites are test code in non-test files: depguard `framework-test-only` excludes `**/framework/**/drivertest/**` (7.1).
- Config struct `<pkg>.Config` with koanf tags + defaults; app `config/<file>.go` embeds it.

### 3.1 Layers (framework internal import order, lower never imports higher)

Indicative; `go list` is truth (no cycles, `go vet`).
1. `support/str`, `support/diff`, `clock`, `ids`, `fault`, `tenancy`, `manager`, `config`
2. `log`, `telemetry` (+ `telemetry/otlp` opt-in), `hashing`, `encryption`, `httpclient/safedial`, `httpclient` (+ `httpclienttest`), `dns`, `aws` (+ `aws/sns`), `process`, `redis`, `resilience`, `i18n`
3. `database`, `database/migrate`, `cache`, `ratelimit`, `pubsub`, `filesystem`, `malware`, `validation`, `sanitize`, `markdown`, `dom`, `jwt`, `webhooks`, `image`, `httpclient/robots`, `websocket`
4. `http` (+ `http/{pagination,origin,useragent,idempotency,rest}`), `events`, `queue`, `schedule`, `mail`, `mailbox`, `broadcasting` (centrifuge on `redis`), `view` (+ `view/{vite,static,feed}`), `customdomain` (+ `customdomain/storage`)
5. `http/middleware`, `notifications`, `session`, `auth` (+ `auth/{token,twofactor}`), `socialite`, `oauth`, `routing`, `health`, `mcp`, `console` (+ `console/{make,stubs}`)
6. `foundation` (wires all), `testkit` (+ `testkit/{factory,arch}`, tests only)

### 3.2 Catalog

Libraries pinned in `go.mod`; versions current at 2026-11 per `research/tech-stack.md` unless marked `[verify]`. Rows reflect built code (2026-11-02); `a.X` = field on `*foundation.Application` (5).

| Package | Purpose | Public API sketch | Default driver(s) | Wraps | Laravel |
|---|---|---|---|---|---|
| `foundation` | Application kernel: config load, boot order, provider lifecycle, roles, env, graceful shutdown, maintenance mode (shared by every replica), runtime tuning (GOMEMLIMIT from cgroup), command line, framework migration set | `New(ctx, Options) (*Application, error)` (boots, applies migrations when `migrations.auto`); `Run(ctx, Options, args, console.IO) int` (command line, exit code; commands `serve`, `migrate`, `migrate:status`, `migrate:plan`, `migrate:rollback`, `down`, `up`, `config:show`, `route:list` + app `Commands`; built with `-tags dev` only: `make:*`, `route:lint`, `lang:check`, run as `make gen ARGS="make:action Name"`; in production `migrate`, `down`, `migrate:rollback` warn (database host/db, migrations) and need a prompt yes or `--force`, non-interactive without `--force` refuses, rollback also needs the exact version via prompt or `--confirm-version`); `Options{Name, Version, EnvPrefix, ConfigFiles, Environ, Migrations, Providers, Commands, Language, LocalePreference, Broadcast, Permissions, Domains, WebPushGone, MaintenanceExcept, RouteRules, Module, Stubs, Root, LogWriter, Telemetry}`; `Provider{Register(*Application) error; Boot(ctx, *Application) error}`, optional `Shutdowner{Shutdown(ctx) error}`; `Command` iface (`Handle(ctx, *Application, *console.Input, *console.Output)`, app booted on first use); `a.Start(ctx)`, `a.Serve(ctx)`, `a.Shutdown(ctx)`, `a.Addr()`, `a.AdminAddr()`, `a.JWKS(url, opts)`; `a.Env` (`local`/`testing`/`production`), `a.Roles.Has(foundation.RoleWorker)` (`web`, `worker`, `scheduler`); `a.Maintenance.Down(ctx, msg)/Up/State/Active` (file `<data_dir>/framework/down`, change announced on `pubsub`); `LoadConfig(ctx, Options)` (`*ConfigError{Err, Redacted}`); `FrameworkMigrations()`; `ErrNotConfirmed`, `ErrNoAppKey`; health paths `/up`, `/ready`, `/metrics` | n/a | KimMachineGun/automemlimit (cgroup memory limit), exaring/otelpgx + otel `contrib` otelhttp (wiring), go.uber.org/zap (only to bridge certmagic logs into `log`); lifecycle own: (d) glue | `Illuminate\Foundation`, Artisan kernel |
| `config` | Typed config load: defaults < YAML files (`<name>.yaml`) < env `<PREFIX>_*` (`__` = nesting) < `_FILE` secrets; runtime store iface (DB settings, app) | `Load[T](ctx, LoadOptions) (*T, error)`; tags `koanf`, `default`, `secret`; `Section` (`map[string]any`, raw subtree for driver config) `.Decode(&v)`, `.String(key)`; `Redacted(v) map[string]any`; `Validator` iface (struct checks itself after load); `RuntimeStore` iface | file + env | knadh/koanf v2 (+ `providers/{file,env/v2,confmap}`, `parsers/yaml`); own `default` tag applier (constant-name reflection only, keeps linker dead method elimination; creasty/defaults removed) | `Illuminate\Config` |
| `manager` | Generic driver manager | `New[T](kind string, sections map[string]config.Section, defaultName string) *Manager[T]`; `Factory[T] func(name, config.Section) (T, error)`; `Extend(driver, Factory[T])`; `Instance(name) (T, error)`; `Default() (T, error)`; `DefaultName()`; `Names()`; `Swap(name, T)`; `ErrUnknownInstance`, `ErrUnknownDriver` | n/a | own: (d) ~80 lines generic map + sync | `Illuminate\Support\Manager` |
| `fault` | Transport-agnostic domain error | `fault.New(kind, code, msg) *Error`; kinds `NotFound`, `Conflict`, `Forbidden`, `Unauthenticated`, `Invalid`, `Gone`, `Locked`, `TooLarge`, `RateLimited`, `FailedDependency`, `Unavailable`; `fault.KindOf(err)`, `fault.CodeOf(err)`; `e.With(key, value)` meta, `e.Meta()` | n/a | own: (d) error type + kinds, stdlib `errors` | `HttpException` + exception `render()` |
| `http` | Server kernel: middleware pipeline + aliases (`"throttle:api"` string params), groups, net/http type aliases (app never imports `net/http`), fault → status + JSON error envelope (messages translated by error code), resource shapes, request context helpers | aliases `Request`, `ResponseWriter`, `Handler`, `HandlerFunc`, `Header`, `Cookie`, `Status*`; `Middleware = func(Handler) Handler`; `NewKernel()`, `k.Use(mw...)`, `k.Alias(name, MiddlewareFactory)`, `k.Group(name, entries...)`, `k.Resolve(entries...)`; input hooks `Validatable{Validate(ctx, *validation.Factory) error}`, `Authorizer{Authorize(ctx) error}`; `Errors{Report}.Render(w, r, err)`, `.Resolve(ctx, err) *ResponseError` (stable `code`, message from `errors.<code>` in request locale; English in `Messages()`); `Resource[T]{Data}`, `Collection[T]{Data, Meta{NextCursor, PrevCursor, PerPage}, Links}` (`NewCollection(page)`); ctx `Principal[T](ctx)`, `RequestID(ctx)`, `ClientIP(r)`; `WriteJSON(w, status, v)` | n/a | stdlib `net/http` (aliases); kernel own: (d) glue; huma integration lives in `routing` | `Illuminate\Http`, `Foundation\Http\Kernel`, FormRequest, API Resources |
| `http/middleware` | Built-ins | `RequestID(header)`, `Recover(errs)`, `TrustProxies(cidrs, header)`, `CORS(CORSOptions)`, `CSRF(trustedOrigins...)`, `Throttle(Limiter, ThrottleOptions)`, `Signed(verifier)`, `Idempotent(store, header, scope)`, `Maintenance(mode, retryAfter, except...)`, `BodyLimit(n)`, `Timeout(d)`, `SecurityHeaders(opts)`, `APIVersion(header, current, supported)`, `ClientVersion(header, minimums, upgradeURLs)`, `Locale(translator, LocalePreference)` (preference hook > `Accept-Language` > `app.locale`); foundation kernel runs RequestID, Locale, Recover, otelhttp, TrustProxies (when `http.trusted_proxies`), Maintenance on every request and aliases `session`, `auth`, `can`, `signed`, `csrf`, `idempotent`, `locale` | n/a | rs/cors, stdlib `http.CrossOriginProtection` (CSRF), realclientip/realclientip-go (proxies); throttle on `ratelimit`; idempotency + maintenance own: (a) no Go lib with Postgres replay store | `TrustProxies`, `HandleCors`, `VerifyCsrfToken`, `ThrottleRequests`, `ValidateSignature`, `SetLocale` |
| `http/idempotency` | Idempotency-key replay store | `Store` iface `Reserve(ctx, scope, key, fingerprint) (*Response, error)`, `Complete`, `Release`; `NewDatabaseStore(db, Config, clock)` (`a.Idempotency`, table `idempotency_keys`, `Prune(ctx)`); `ErrInProgress`, `ErrKeyReused` | database | own: (a)+(c) rules from `00-foundation/rest.md` | none |
| `http/pagination` | Signed cursors, `filter[]`, `sort`, `include`, `limit`, search parsing with allowlists | `NewCursors(key)` `.Encode(v)`, `.Decode(s, &v)`; `Parse(url.Values, Allow{Filters, Sorts, Includes, DefaultLimit, MaxLimit}) (Query, error)`; `Page[T]{Items, NextCursor, PrevCursor, Limit}`; faults `ErrInvalidCursor`, `ErrInvalidFilter`, `ErrInvalidSort`, `ErrInvalidInclude`, `ErrInvalidLimit` | n/a | own: (c) JSON:API-ish query grammar of `00-foundation/api.md`; no lib matches | `cursorPaginate` |
| `http/rest` | REST helpers: bulk PATCH (`{ids\|filter, changes}`), field-dispatch Update (state field → own handler, one tx) | `Bulk[C](...)` over `BulkRequest[C]` → `BulkResult`; `Dispatcher[R]{...}.Update(ctx, resource, Changes)`; `Transactor` iface | n/a | own: (c) rules from `00-foundation/rest.md` 7-8 | none |
| `http/origin` | Origin normalize, punycode, wildcard leftmost label, `null` origin | `Parse(s) (Origin, error)`; `Match(pattern, origin) bool`; `o.IsNull()` | n/a | golang.org/x/net (idna, publicsuffix) | none |
| `http/useragent` | UA parse (browser, OS, device) for session/device labels | `New() *Parser`; `p.Parse(ua) Agent{Browser, OS, Device}` | n/a | medama-io/go-useragent (trie, zero alloc; uap-go rejected: regex set heavier at boot, build-vs-reuse 3.2) | jenssegers/agent |
| `routing` | Named routes, typed huma operations (OpenAPI 3.1, binding, 422 shape), URL generation, signed URLs, `route:list`, `route:lint` engine (rules from app) | `New(Options{Middleware, Errors, Validator}) *Router`; `r.Group(prefix, fn)`, `r.API(prefix, title, version, fn)` (typed group), `r.Handle(method, path, name, h)`, `r.Middleware(names...)`, `r.Use(mw...)`; generic `routing.Get/Post/Patch/Put/Delete[I, O](g, path, name, endpoint) *Route`; `route.Middleware(...)`, `.Ability("x:write")`, `.Status`, `.Summary`, `.Tags`; `r.OpenAPI(prefix)`, `r.Routes()`, `CurrentRoute(ctx)`; `NewURL(router, baseURL, key, clock)` (`a.URL`): `Route(name, params) (string, error)`, `MustRoute`, `SignedRoute(name, params, ttl)`, `HasValidSignature(r)`; `Lint(routes, Rules) []string` | n/a | go-chi/chi v5 + danielgtaylor/huma v2 (`humachi`; operation registry = route table); name index + signing own: (a) chi has no named routes | `Illuminate\Routing`, `UrlGenerator` |
| `auth` | Guards, principal in ctx, gate + policies, token abilities, recent-auth confirmation | `Guard` iface `Authenticate(r) (Principal, error)`; `SessionGuard(name)`, `BearerGuard(resolvers...)`, `Any(guards...)`, `GuardFunc`; `a.Guards.Register(name, Guard)`, `.Define(name, refusal, kinds...)`, `.Middleware` (alias `auth:...`); `Principal` iface `ID() ids.UUID`, `Guard()`, `Tenant()`, `Abilities()`, `ConfirmedAt()`; `Identity` (default impl); `auth.From(ctx)`; `HasAbility(p, ability)`; `a.Gate.Define(permission, Rule)`, `auth.RegisterPolicy[T](gate, permission, Policy[T])`, `Gate.Authorize(ctx, permission, resource) error` (fault Forbidden/NotFound), `Gate.Can(ctx, p, permission, resource)`, `.Middleware` (alias `can:...`); `PermissionResolver` (`foundation.Options.Permissions`); `RequireConfirmation(clock, window, errs)`, `Confirmed(p, now, window)` | app supplies resolvers | own: (b) casbin/ory keto are policy engines with own model language and storage; Laravel gate = typed funcs, ~200 lines | `Illuminate\Auth`, Gate, Sanctum abilities, `password.confirm` |
| `auth/token` | Personal access tokens: prefix + base62 random + CRC32 checksum, SHA-256 digest stored, constant-time compare | `New(Store, clock, Options) (*Tokens, error)`; `t.Create(ctx, Token) (Token, plain, error)`, `Find(ctx, plain)`, `ForOwner(ctx, ownerID)`, `Use(ctx, tok, ip)`, `Revoke(ctx, id)`, `Resolver(via) auth.TokenResolver` (for `auth.BearerGuard`); `Store` iface + `NewPostgresStore(db, table)` (framework table `personal_access_tokens`); `Migrations()` | postgres store | own: (a) GitHub-style token format, no lib; stdlib `crypto/rand`, `hash/crc32`, `crypto/sha256`; pgx | Sanctum `NewAccessToken` |
| `auth/twofactor` | Second factors: TOTP (RFC 6238, otpauth URI, replay block), recovery codes, factor registry, pending password → second-factor challenge | `NewTOTP(issuer, enc, cache, clock)` `.NewSecret()`, `.Setup(sealed, account) (secret, url, err)`, `.Verify(ctx, cred, input)`; `NewRecoveryCodes(enc)` `.Generate(n) (codes, sealed, err)`, `.Remaining`, `.Verify`; `Factor` iface + `NewRegistry(factors...)`; `NewChallenges(cache, clock, ttl, attempts)` `.Begin/Find/Fail/Complete` | n/a | pquerna/otp | Fortify 2FA |
| `session` | Cookie session mechanics: id gen, rotation, flash, idle/absolute expiry, per-user device list | `NewManager(Store, clock, Config)` (`a.Session`); `m.Middleware` (alias `session`), `m.Start(r)`, `session.From(ctx)`; `sess.Get/Put/Forget/Flash/Flashed/Login(userID)/Regenerate/Confirm/Destroy`; `m.Sessions(ctx, userID)`, `m.Revoke`, `m.RevokeOthers`; `Store` iface + `NewPostgresStore(db, table)` (SHA-256 of cookie token, per-user index, `Prune`); `Migrations()` | postgres store on framework table `sessions` (app migrations ALTER it for product columns) | own: (c) 00-accounts stores SHA-256 of cookie token and lists/revokes per user; alexedwards/scs stores raw token as key, no per-user index | `Illuminate\Session` |
| `socialite` | OAuth/OIDC login client providers | `a.Social.Redirect(ctx, provider, redirectURL) (authURL, state, error)`; `a.Social.Callback(ctx, provider, state, query) (User, error)` (state + PKCE verifier kept in `cache`, `socialite.state_ttl`); `Driver(name) (Provider, error)`; `Extend`; `Fake()` (testing env only; `f.Approve(authURL, user)` returns the callback) | `github`, `google` (OIDC id_token verify), `oidc` (generic, discovery); `saml` later | golang.org/x/oauth2, coreos/go-oidc v3 (oidc driver); crewjam/saml later. markbates/goth rejected: `gothic` couples gorilla sessions, PKCE uneven across providers | Laravel Socialite |
| `oauth` | OAuth 2.1 authorization server with storage: registered + dynamic clients, authorization code + PKCE S256, rotating refresh tokens (reuse revokes the whole grant family, no grace window), revocation, metadata, bearer resolver | `New(db, clock, limiter, Options{Issuer, Prefix, Scopes, AuthorizationPath, CodeTTL, AccessTokenTTL, FirstPartyAccessTokenTTL, RefreshTokenTTL}) (*Server, error)`; `s.Routes(router)` (metadata RFC 8414, token, revoke RFC 7009, register RFC 7591 DCR); `s.ValidateAuthorization(ctx, query) (*AuthorizationRequest, error)` → app consent screen → `s.Approve(ctx, req, Approval)` / `s.Deny(req)`; `s.CreateClient`, `s.ConfirmGrant`, `s.RevokeGrant`; `s.Resolver()` (for `auth.BearerGuard`); `s.Metadata()`, `s.MetadataURL()`; `Migrations()` (tables `oauth_clients`, `oauth_authorization_codes`, `oauth_grants`, `oauth_tokens`). Deferred: RFC 8707 resource indicators/audiences (needed by MCP), client ID metadata documents (CIMD), device grant, consent-skip rules, last-use tracking | postgres (own tables) | own: (b)+(c) build-vs-reuse section 2 (fosite stalled, zitadel OIDC-centric, neither has CIMD/DCR/8707); pgx. RFC 9728 protected resource metadata served by `mcp.ProtectedResourceHandler` | Laravel Passport |
| `jwt` | Strict JWS verify + sign: base64url strict, duplicate-key-rejecting JSON, `alg` pin, `crit`/`typ` rules, leeway, lifetime cap, size cap, JWKS cache | `Verify(token, keys []Key, Options{Algorithms, Audience, Issuer, Leeway, MaxLifetime, MaxSize}) (Claims, error)`; `Sign(claims, key)`; `NewKey(id, alg, material)`, `ParseJWKS`, `ParsePublicKeyPEM`; `NewJWKS(url, fetch, JWKSOptions)` `.Keys(ctx)`, `.Verify(ctx, token, opts)` (`a.JWKS(url, opts)` fetches through `a.HTTP`, timed by `a.Clock`); algs HS256, RS256, ES256, EdDSA | n/a | go-jose/go-jose v4 (signature verify, JWK/JWKS); strict pre-parse own: (c) 02 requires duplicate-key rejection + strict base64url, not offered by go-jose or golang-jwt v5 | none |
| `hashing` | Password hashing + digests | `Hasher` iface; `a.Hash.Make(ctx, pw)`, `Check(ctx, pw, hash)`, `NeedsRehash(hash)`, `Hasher(name)`, `Extend("scrypt", f)`; config `hashing.default`, `concurrency`, `queue`, `hashers.<name>`; semaphore (`ErrBusy`), `ErrPasswordTooLong`; NFKC normalize; digests `SHA256(b)`, `SHA256Hex(b)`, `HMAC(key, msg)`, `Equal(a, b)` | `argon2id` (PHC string); `bcrypt` built-in alternate | alexedwards/argon2id, golang.org/x/crypto/bcrypt, golang.org/x/text/unicode/norm, golang.org/x/sync/semaphore | `Illuminate\Hashing` |
| `encryption` | App-key encrypter, KEK manager, per-scope data keys (envelope), sealed column type, streaming AEAD (backups), deterministic MAC, rotation | app key: `a.Encrypter` (`Encrypter` iface `Encrypt/Decrypt`, `EncryptString/DecryptString`; `NewEncrypter(appKey, previous...)`), `DeriveKey(appKey, purpose)` (HKDF-SHA256), `GenerateAppKey`, `ParseAppKey`; KEKs: `a.Crypt` (`KEK` iface `Wrap/Unwrap`; config `encryption.kek`, `previous`, `keks.<name>`); data keys: `a.Crypt.Keys(store KeysetStore) (*Keys, error)` = `KeyManager` iface `Encrypt(ctx, scope, plaintext, aad)`, `Decrypt(ctx, ciphertext, aad)`, `MAC(ctx, scope, data)`, `NewStreamWriter/NewStreamReader(ctx, scope, w/r)`; `Seal[T](ctx, keys, Slot{Scope, AAD}, v) (Sealed, error)`, `Open[T](ctx, keys, sealed, aad)`; `Scope{Kind, ID}`; one Tink keyset per (scope, purpose `aead`/`stream`/`mac`), created on first use, Tink key-id prefix in ciphertext; `KeysetStore` iface `Find/Create/Replace/All` (atomic create/replace; app table `encryption_keys`); `k.Rotate(ctx, scope)` (value + stream keys only: MAC keys never rotated, a new key would change every stored lookup hash), `k.RewrapAll(ctx)` (after KEK change); contract suite `encryption/drivertest` | KEK `local` (derived from app key) | tink-crypto/tink-go v2 (AEAD AES-256-GCM, streaming AEAD AES-GCM-HKDF, PRF HMAC-SHA256 for `MAC`, keysets + rotation); `awskms` KEK own thin code on aws-sdk-go-v2 `kms` (tink-go-awskms v2 needs SDK v1); vault, gcpkms later via `Extend` | `Illuminate\Encryption`, `encrypted` cast |
| `database` | Thin pgx layer under sqlc: pool, transaction in context with hooks, tenant `SET LOCAL` per transaction, advisory xact locks, read replicas, direct URL, COPY, NOTIFY, vectors, error mapping | `Open(ctx, Config{URL, DirectURL, ReadURLs, ReadMaxLag, MaxConns, PoolMode, TenantSetting}) (*DB, error)` (refuses a standby; `pool_mode=transaction` runs `QueryExecModeExec`); `*DB` and `*Tx` satisfy `Querier` = sqlc `DBTX` (`Exec`, `Query`, `QueryRow`); `db.Transaction(ctx, func(ctx, tx *Tx) error) error`: ctx passed to fn carries the tx, `*DB` methods called with it join that tx; nested `Transaction` = savepoint; rollback on error or panic; `tx.AfterCommit(fn)`, `tx.AfterRollback(fn)` (outermost tx; dropped with a failed savepoint); tenant from `tenancy.From(ctx)` written to `database.tenant_setting` (default `app.tenant_id`) with `SET LOCAL` semantics at the start of every transaction only: tenant data reachable only inside `db.Transaction`, outside one tenant tables look empty (fail closed); `tx.AdvisoryLock(ctx, key)`, `tx.TryAdvisoryLock(ctx, key)` (xact-scoped); `db.Reader(ctx) Querier` (healthy replica from `read_urls` within `read_max_lag`; primary when none healthy or the ctx already wrote, `TrackWrites(ctx)`; inside a tx = that tx); `cfg.SessionURL()` (`direct_url` behind a transaction pooler, `ErrNoSessionURL`; used by migrations and queue); `tx.CopyFrom(ctx, table, columns, rows)` (COPY; cannot load tables with row-level security as the app role: tenant tables use batched inserts or COPY into a staging table, 16); `db.Notify(ctx, channel, payload)` (autocommit only, `ErrNotifyInTransaction`: in a tx use `AfterCommit`); `Vector` (`NewVector`); `ErrNotFound`, `ErrStandby`, `ErrReadOnly` (SQLSTATE `25006`, resets pool), `IsUniqueViolation(err, constraint)`, `IsSerialization(err)`; topologies per 11 operations.md section 11.2 | PostgreSQL only | jackc/pgx v5 (pgxpool, pgconn errors), pgvector/pgvector-go; tx hooks own: (d) glue | `Illuminate\Database` (`DB::transaction`, `afterCommit`) |
| `database/migrate` | Migration runner: embedded goose SQL, library steps (River, notification tables) then framework set then app set (version tables `framework_migrations`, `app_migrations`), cluster-wide advisory lock with wait logging, `lock_timeout`/statement timeout retry, dry-run plan, migration lint | `Runner{URL, Framework, App fs.FS, Extra []Set, Steps []Step, LockWait, LockTimeout, StatementTimeout, LockRetries, AllowRollback, OnWait}`: `Up(ctx) ([]Result, error)`, `Status`, `Plan` (`Pending{Set, Version, Name, Phase, Transactional, Findings}`), `Rollback` (`ErrRollbackDisabled` unless allowed; production only via guarded `migrate:rollback`); `Step{Name, Up(ctx, pool)}` (forward only); `Lint(fs) []Finding`; `ErrLockTimeout`; config `migrations.auto` (default on), `migrations.lock_timeout` (10m) | n/a | pressly/goose v3 (Provider API + lock; runs on database/sql via pgx `stdlib` adapter: PO accepted), riverqueue/river `rivermigrate` (via `queue.Migration()` step); lint rules own: (a) squawk is Rust binary, (b) | `Illuminate\Database\Migrations` |
| `tenancy` | Tenant id in context | `With(ctx, id) ctx`; `From(ctx) (string, bool)` (empty id = none); `Required(ctx) (string, error)` (`ErrMissing`) | n/a | own: (d) one context key | stancl/tenancy (concept) |
| `events` | Dispatcher with phases | `events.New(q)` (`a.Events`); `d.Dispatch(ctx, tx *database.Tx, e Event) error`; `Listen[E](d, events.InTx\|events.AfterCommit, fn func(ctx, E) error)`; `ListenQueued[E](d, func(E) queue.Args)` (job pushed in same tx); `Ephemeral` marker, `IsEphemeral(e)`. No fake: tests assert real effects (rows, job rows) | n/a | own: (b)+(c) watermill/asaskevich EventBus lack tx phases tied to pgx tx + River `InsertTx`; ~200 lines | `Illuminate\Events`, `ShouldQueue`, `afterCommit` |
| `queue` | Jobs on River; thin by design (one driver) | `Args` iface `Kind() string`, optional `Options() Options`; `Worker[A]` iface `Handle(ctx, *Job[A]) error`; `Register[A](q, w)`; `q.Push(ctx, args, ...Opt)`, `q.PushTx(ctx, tx, args, ...Opt)` (unique duplicate skipped without error); opts `At(t)`, `MaxAttempts(n)`, `OnQueue(name)`, `Priority(p)`, `Unique(UniqueOpts{ByArgs, ByPeriod, ByState})`; worker returns `Postpone(d)` (run later, attempt not spent), `Cancel(err)`, `RetryAfter(d)`; `q.AddPeriodic(Periodic{Next, Args, Opts, RunOnStart})` (used by `schedule`); `q.Start/Stop/Close`; `q.Drain(ctx)` (tests: works every ready job, tolerates client/database clock skew); `Migration()` (River schema as migrate step); `Config{Queues, MaxConns, JobTimeout, PollInterval, NoLeader}`; `ErrUnknownKind`. No fake: tests push for real and `Drain` | River (Postgres) | riverqueue/river (+ `riverpgxv5`, `rivermigrate`, `rivertype`). River UI not included (deferred) | `Illuminate\Queue`, `Bus` |
| `schedule` | Scheduler on River periodic jobs, leader-elected | `schedule.New(q)` (`a.Schedule`); `s.Job(args, opts...)` `.EveryMinute()`, `.Hourly()`, `.Daily()`, `.DailyAt("03:00")`, `.Cron("0 3 * * *")`, `.Timezone(loc)`, `.RunOnStart()`, `.Next(after)`; `s.Apply()`; `ErrInvalidTime`. Only nodes with role `scheduler` join River leader election and push periodic jobs; worker-only nodes run with leader election off (`queue.Config.NoLeader`); scheduler-only nodes work no jobs | River | riverqueue/river (PeriodicJob, leader election), robfig/cron v3 (parser) | `Console\Scheduling\Schedule` |
| `pubsub` | Internal cross-replica bus (cache invalidation, auth cache bust, entitlement bust, maintenance state, control topics); not user-facing | `pubsub.New(client, Options)` (`a.PubSub` = `*Bus`); `b.Publish(ctx, topic, payload []byte)`; `b.Subscribe(ctx, topic, fn func(payload)) (Unsubscribe, error)`. No fake: tests use real Valkey | Valkey PUBLISH/SUBSCRIBE (keyspace under `redis.key_prefix`) | `framework/redis` client (rueidis) | Redis pub/sub |
| `broadcasting` | Realtime to browsers/apps: namespaced channels, connect/subscribe auth callbacks, presence, history + recovery, WebSocket + uni-SSE transports | `Open(ctx, client, Config{Keyspace, PingInterval, MaxPayload, Resolve, Clock}) (*Server, error)` (`a.Broadcast`); `Broadcaster` iface; `s.Broadcast(ctx, channel, event, payload)`, `BroadcastMany`, `BroadcastEphemeral`; `s.Channels().Authorize(pattern, ChannelOptions{Presence, JoinLeave, HistorySize, HistoryTTL}, Authorizer)` (registered in `routes/channels.go`); `s.Handlers()` (`/connection/websocket`, `/connection/uni_sse`); connect auth via `Resolver` (`foundation.Options.Broadcast`: session/bearer or widget connect token; nil refuses all); `s.Disconnect(ctx, userID)`; `s.Presence(ctx, channel)`; `s.Shutdown(ctx)`; `ErrUnknownChannel`, `ErrPayloadTooLarge`. No fake: tests subscribe a real client | centrifuge Redis broker + Redis presence manager on Valkey (always) | centrifugal/centrifuge (MIT, embedded library: Node, transports, presence, history/recovery, `OnConnecting`/`OnSubscribe` (client publish disabled), Redis broker + presence on rueidis = our Valkey client). No Centrifugo server code, no Centrifugo PRO features required. uni-SSE handler own: (a) library ships only an `_examples` handler; ~200 lines on public API. Clients: centrifuge-js (dashboard), centrifuge-swift/java/dart (mobile), centrifuge-go (CLI), plain `EventSource` (widget) | `Illuminate\Broadcasting` + Reverb |
| `websocket` | Thin client dial through safedial for any websocket client not covered by an adapter SDK (disgo and slack-go dial themselves) | `Dial(ctx, url, Options) (*Conn, error)`; `conn.Read(ctx)`, `Write(ctx, typ, data)`, `Close()`, `Subprotocol()` | n/a | coder/websocket | none |
| `cache` | Shared cache on Valkey: values with TTL, once-keys, counters, stampede-safe Remember, best-effort locks. No driver manager, no `Store` interface, no fake: Valkey only, real in tests | `cache.Open(client *redis.Client, cache.Options{Keyspace}) (*cache.Store, error)`, `s.Close()`; `Get(ctx, key) ([]byte, error)` (`ErrMiss`), `Many(ctx, keys)`, `Set(ctx, key, value, ttl)`, `Add(ctx, key, value, ttl) (bool, error)` (`SET NX PX`: replay/dedupe windows), `Increment(ctx, key, by, ttl) (int64, error)` (expiry set once, on the first increment: fixed windows; `ErrNotInteger`), `Delete(ctx, key)`, `Flush(ctx)` (SCAN + UNLINK of own keyspace on every node; never FLUSHDB); every write requires a TTL (`ErrTTLRequired`); `Remember(ctx, key, ttl, load) ([]byte, error)` + `Forget(ctx, key)`: one loader across all processes, others wait, hot keys served from server-assisted client-side cache; typed `cache.For[T](store, ttl).Remember(ctx, key, load)` (JSON); `Lock(ctx, name)` waits, `TryLock(ctx, name)` returns `ErrLocked` at once, both `(heldCtx, release, error)`: lock auto-extends while `heldCtx` lives, protected work runs with `heldCtx`; best effort (correctness-critical locks = `tx.AdvisoryLock`) | Valkey (`cache.keyspace`, default `cache`, under `redis.key_prefix`) | `framework/redis` (rueidis), `github.com/redis/rueidis/rueidisaside` (cache-aside: cross-process stampede protection + client-side caching), `github.com/redis/rueidis/rueidislock` (auto-extending locks); per-process L1 caches that specs ask for stay owner-built on maypok86/otter v2 | `Illuminate\Cache`, `Cache::remember`, `Cache::lock`, `Cache::add` |
| `ratelimit` | Rate limiter: shared on Valkey (`a.Limiter`); `Local` in-process token bucket per key (rate + burst) for per-job pacing like 08 crawl politeness | `ratelimit.New(client, Options) (*Limiter, error)`; `l.Attempt(ctx, key, max, decay) (Result, error)`, `Hit`, `Remaining`, `AvailableIn`, `Clear`; `Result{Allowed, Remaining, RetryAfter}`, `r.WriteHeaders(h, now)` (Laravel headers); `ErrUnavailable`; `NewLocal(interval, burst, clock)` `.Allow(key)`, `.Wait(ctx, key)`. No fake: tests use real Valkey | Valkey | `github.com/redis/rueidis/rueidislimiter` (Valkey counters), golang.org/x/time/rate (`Local`) | `Illuminate\Cache\RateLimiter` |
| `filesystem` | Disks | `Filesystem` iface: `Put(ctx, path, []byte, PutOptions)`, `PutStream(ctx, path, io.Reader, PutOptions)` (failed stream leaves nothing), `Get`, `Open` (`io.ReadCloser`), `Exists`, `Stat(ctx, path) (Attributes, error)`, `Delete` (missing file = no error), `List(ctx, prefix) iter.Seq2[Entry, error]` (path order), `URL(path)` (public base URL or `ErrNoPublicURL`), `TemporaryURL(ctx, path, ttl)`, `TemporaryUploadURL(ctx, path, UploadURLOptions{TTL, ContentType})`; `PutOptions{ContentType, CacheControl, ContentEncoding, Metadata}` (zero value valid: content type sniffed; metadata keys lowercased); `Attributes{Size, ContentType, CacheControl, ContentEncoding, LastModified, Metadata}`; `Entry{Path, Size, LastModified}`; `ValidatePath` on stdlib `io/fs.ValidPath` (empty, absolute, dotted paths refused before any storage call), `EscapePath`; `ErrNotFound`, `ErrInvalidPath`, `ErrNoPublicURL`, `ErrUnsupported`; `NewManager(Config)`, `a.Storage.Disk(name)`, `Default()`, `Extend`; drivers implement small `Store` iface, `NewDisk(store, DiskOptions)` adds path checks, URLs, signing; local driver signs temporary URLs only with `signed_url_base` + `signing_key` (sidecar attrs file per object); s3 driver signs the upload content type into the presigned PUT; contract suite `filesystem/drivertest` (local, s3). No fake | `local` (dev, tests, single node; default disk `local` at `<data_dir>/app`), `s3` (staging, prod) | aws-sdk-go-v2 `s3` (+ `feature/s3/transfermanager` for streams), stdlib `os.Root` (local); gocloud.dev removed (dead method elimination) | `Illuminate\Filesystem` (Flysystem) |
| `malware` | File scanning | `Scanner` iface `Scan(ctx, File) (Verdict, error)`; `a.Scanner.Scan(ctx, file)`, `Scanner(name)`, `Extend`; `Verdict{Status, Signature}`; `ErrScanFailed`, `ErrPending`; `EICAR()`; `Fake()` (testing env only; `f.Set(path, verdict)`, `f.AssertScanned(t, path)`) | `none` | `clamav`: own clamd INSTREAM client: (a) dutchcoders/go-clamd unmaintained, protocol ~80 lines; `guardduty`: aws-sdk-go-v2 `s3` (object tag read) | none |
| `mail` | Outbound mail: named mailers (streams), mailables, queued sends, MIME builder, DKIM signing, rendering in recipient locale | `Mailer` iface `Send(ctx, Message) (SendResult, error)`; `a.Mail.Mailer(name) (Mailer, error)` (`system` default, `customer` used only by 04 actions, arch test), `Default()`, `Extend`; `Message{From, ReplyTo, To, Cc, Bcc, Subject, Text, HTML, Headers, Attachments, Raw, Tags, ConfigurationSet, TenantName}`; `SendResult{ProviderMessageID, MessageID}`; `Mailable` iface `Envelope() Envelope`, `Content(ctx) (Content, error)` (`mail.View(component)`: templ HTML + text); `mail.Register[M](a.Mail, "name")` (needed to queue); `a.Mail.To(addrs...).Cc/Bcc/Mailer(name)/Locale(l)` then `.Send(ctx, m)`, `.Queue(ctx, m)`, `.QueueTx(ctx, tx, m)`; errors `ErrPermanent`, `ErrTemporary`, `*RateLimitedError{RetryAfter}`; `mime` builder/parser; `dkim` signer; `ParseAddress`; `Fake()` (testing env only; `mail.AssertSent[M](t, f, pred)`, `f.AssertMessage`, `f.AssertNothingSent`, `f.Sent()`) | built-in: `smtp` (relay/submission only, never MX), `ses`, `postmark`, `resend`, `mailgun`, `sendgrid`, `log` (default when none configured). JSON-only providers (postmark, resend, sendgrid) decompose `Raw` MIME into provider JSON | wneessen/go-mail (SMTP + MIME writer), jhillyerd/enmime v2 (MIME read), emersion/go-msgauth `dkim`, aws-sdk-go-v2 `sesv2`, k3a/html2text (text part); postmark/resend/mailgun/sendgrid own: (d) one JSON POST each on `httpclient` | `Illuminate\Mail` |
| `mailbox` | Inbound mail: sources, MIME parse, auth checks, bounce/complaint reports | `Source` iface `Name()`; `Receiver` iface `Receive(ctx, mailbox.Delivery) (*Envelope, error)`, `MaxWebhookBytes()`, optional `StatusMapper` (webhook-fed sources; 04 sources.md 1); `a.Mailbox.Source(name)`, `Receiver(name)`, `Extend`; `Envelope{Source, ProviderMessageID, EnvelopeFrom, Recipients, SizeBytes, ReceivedAt, Verdicts, OpenRaw, RawRef}`; `parse.Message(raw, Limits) (*Parsed, error)`; `authcheck.New(resolver).Verify(ctx, Input) Results`; `dsn.Parse`, `arf.Parse`; `ErrRejected`, `ErrTooLarge`, `ErrUnsubscribed`, `ErrNotReceiver`; contract suite `mailbox/drivertest` | `ses` (receipt rule -> S3 + SNS), `postmark`, `mailgun`, `sendgrid`, `resend`, `cloudflare`; optional `Backfiller`; account drivers `gmail`, `graph` (`mailbox.Account`); `mailbox/imap` client + `imap/imaptest` in-memory server. No SMTP receiver (PO: mail only through instance provider) | jhillyerd/enmime v2, emersion/go-imap/v2 (imap client), emersion/go-message (multipart/report walk), emersion/go-msgauth (dkim, dmarc), blitiri.com.ar/go/spf, golang.org/x/net (charset, publicsuffix); raw fetch via `filesystem` s3 disk; SNS verify via `aws/sns` | beyondcode/laravel-mailbox |
| `push` | Mobile push | `Sender` iface; drivers `apns`, `fcm`, `Fake()`, drivertest (20) |
| `notifications` | Multi-channel notifications, queued per recipient and channel, rendered in recipient locale | `Notification` iface `Type() string`, `Via(to Notifiable) []string`; optional `Queued{QueueOptions(Delivery)}`, `Conditional{ShouldSend(ctx, Delivery)}`; `Notifiable` iface `NotifiableType()`, `NotifiableID()`, `RouteFor(channel) any`, optional `Localized{Locale()}`; `Channel` iface `Send(ctx, Delivery, Notification) error`; `notifications.Register[N](a.Notify)`; `a.Notify.Resolve(type, Resolver)`; `a.Notify.Send(ctx, n, to...)`, `a.Notify.SendTx(ctx, tx, n, to...)`; `Extend`; errors `ErrPermanent`, `ErrNotifiableGone`, ... No fake: tests assert rows, jobs and Mailpit/mail fake | channels `mail`, `database` (framework table `notifications`, one migrate step per configured table), `broadcast`, `webpush` | webpush: SherClockHolmes/webpush-go v1.4 (RFC 8291 encryption + VAPID, `HTTPClient` = safe client; gone subscriptions reported to `foundation.Options.WebPushGone`); routing glue own: (d) | `Illuminate\Notifications` |
| `log` | Structured JSON logging, ctx fields (request id, tenant, actor), secret redaction | `Logger` (= `*slog.Logger` alias, app never imports slog); `log.New(Options{Driver, Level, Redact, Writer})`; `log.From(ctx)`, `log.NewContext(ctx, l)`, `log.With(ctx, attrs...)`; attrs `log.String`, `log.Int`, `log.Err`, `log.Any`, ... No fake: tests pass `foundation.Options.LogWriter` | `json` stdout; `text` (local) | stdlib log/slog, m-mizutani/masq (redaction ReplaceAttr) | `Illuminate\Log` |
| `telemetry` | Traces, metrics, Prometheus endpoint, optional error reporting | `telemetry.New(ctx, Config{ServiceName, Environment, Tracing, OTLPEndpoint, OTLPHeaders, Metrics, SentryDSN}, opts...)` (`a.Telemetry`); `t.Start(ctx, name, attrs...) (ctx, Span)` (`span.Set/Fail/End`); `t.Counter(name)`, `Histogram(name, buckets...)`, `Gauge(name)`; `t.MetricsHandler()`; `t.Report(ctx, err)`; `WithExporter(name, Exporter)` | tracing `none`; metrics `none` or `prometheus` (admin port); sentry when DSN set | go.opentelemetry.io/otel (sdk trace + metric, `exporters/prometheus`), prometheus/client_golang, getsentry/sentry-go (opt-in); `otelhttp` + exaring/otelpgx wired by foundation | none (Pulse/Telescope adjacent) |
| `telemetry/otlp` | Opt-in OTLP trace exporter (kept out of the default build: links gRPC) | `otlp.Tracing() telemetry.Option`, registered via `foundation.Options{Telemetry: []telemetry.Option{otlp.Tracing()}}`; config `telemetry.tracing: otlp`, `otlp_endpoint`, `otlp_headers` | n/a | go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp (follow-up: own HTTP exporter so enabling OTLP does not re-link gRPC) | none |
| `health` | Check registry, `/up`, `/ready` | `Check` iface `Name()`, `Run(ctx) Result{Status, Detail, Fix}` (`Pass`, `Warn`, `Fail`); `NewRegistry(timeout, clock)` (`a.Health`), `r.Add(checks...)`, `r.Run(ctx) Report`; built-in checks `Database(db)`, `Valkey(client)`, `Func(name, fn)` (foundation adds maintenance); handlers `Up()`, `Ready(registry)`. Doctor product checks stay app | n/a | own: (b) alexliesenfeld/health HTTP-only, no CLI doctor with fix hints; registry ~100 lines | `/up` route, spatie/laravel-health |
| `httpclient` | Outbound HTTP: SSRF guard, fluent client, SSE reader | `a.HTTP.New().WithToken(t).WithHeader(k, v).Accept(ct).Timeout(d).MaxBody(n).WithoutRedirects().Stream().Get/Delete(ctx, url)`, `.Post/Put/Patch(ctx, url, body)` → `(*Response, error)`; `resp.JSON(&v)`, `resp.Reader()`, `resp.Close()`, `resp.Status`; `ErrBlockedHost`, `ErrTooLarge`, `ErrTooManyRedirects`; `c.StdClient() *http.Client` (adapter SDKs), `c.CheckAddress(addr)`; `SSE(resp) *SSEReader` `.Next()` (`MaxEventSize` 1 MiB); `RetryAfter(header, now)`, `Permanent(status)`; config `http_client.allow_private_networks`, `allow_hosts`, `timeout`, `max_body`, `max_redirects`. Tests only: `httpclienttest.New(a.HTTP)` (swaps transport via `ReplaceTransport`, testing env only) `.Stub(pattern, httpclient.Response)`, `.PreventStrayRequests()`, `.AssertSent(t, pred)` (subpackage keeps httpmock out of production binaries) | safe transport | resty.dev/v3 (fluent client, redirects, SSE), code.dny.dev/ssrf (via `safedial`: private, loopback, link-local, CGNAT, metadata, ULA; checked at connect = rebinding safe), jarcoal/httpmock (`httpclienttest` only) | `Illuminate\Http\Client` |
| `httpclient/robots` | robots.txt fetch + RFC 9309 match | `robots.Fetch(ctx, client, origin) (*Rules, error)`; `rules.Allowed(userAgent, path) bool`; `rules.CrawlDelay(ua)`; 4xx = allow all, 5xx/unreachable = disallow all (08 rule, E202) | n/a | temoto/robotstxt v1.1.2 (MIT); RFC 9309 vectors kept as tests | none |
| `httpclient/safedial` | SSRF-guarded TCP dialer for non-HTTP (SMTP, websocket client) and the HTTP transport | `safedial.New(Options) (*Dialer, error)`; `d.DialContext(ctx, network, addr)`, `d.Check(addr)`; `ErrBlocked`, `ErrInvalidHost` | n/a | code.dny.dev/ssrf + stdlib `net.Dialer` | none |
| `dns` | Resolver with timeouts + fake (no network in tests) | `Resolver` iface `TXT/MX/CNAME/Addrs(ctx, name)`; `dns.New(Config)` (`a.DNS` = `*Client`); `ErrNotFound`; `a.DNS.Fake()` (testing env only) `.Set(name, Records)`; contract suite `dns/drivertest` (system + fake) | system resolver (`dns.nameserver` optional, `dns.timeout` 5s) | stdlib `net.Resolver` | none |
| `aws` | Build `aws.Config` from framework config with safe HTTP client; SNS message signature verify | `aws.Config(ctx, section, httpClient) (aws.Config, error)` (config `aws`); `sns.Parse(body)`, `sns.NewVerifier(client).Verify(ctx, msg, Allow)` (`ErrTopicNotAllowed`, `ErrCertificate`, `ErrSignature`) | n/a | aws-sdk-go-v2 (`config`, `credentials`); SNS verify own: (a) AWS ships no Go SNS message validator, ~80 lines | none |
| `process` | Run external programs (pg_dump, pg_restore) | `a.Process.Run(ctx, Cmd{Name, Args, Env, Dir, Stdin}) (Result, error)`; `a.Process.Start(ctx, Cmd) (*Handle, error)`, `h.Wait()`; `ErrNotFound`, `*ExitError`. No fake: tests run real programs | `os/exec` | stdlib `os/exec` | `Illuminate\Process` |
| `clock` | Fakeable time | `Clock` (= clockwork.Clock: `Now/Since/Until/Sleep/After/NewTimer/NewTicker`); `clock.System()` (`a.Clock`); tests: `clock.NewFake(t)` `.Freeze(t)`, `.Travel(d)`, passed to the service under test | system | jonboulle/clockwork | `Carbon::setTestNow`, `travel()` |
| `ids` | UUIDv7 + prefixed public ids | `UUID` (alias of google/uuid UUID), `New() UUID` (v7), `NewV7At(t time.Time) UUID` (v7 with given ms timestamp, random rest; 16 keeps source order for imported rows), `Parse`, `Nil`; `NewRegistry()` (`a.IDs`), `reg.Register(prefix, name) error` (`ErrPrefixTaken`, `ErrInvalidPrefix`); `reg.Encode(prefix, id) string`; `reg.Decode(prefix, s) (UUID, error)` (`ErrWrongPrefix` → 404, `ErrUnknownPrefix`, `ErrInvalid`) | n/a | google/uuid, go.jetify.com/typeid (TypeID = prefix + `_` + 26-char Crockford base32 lowercase of UUID: matches `data-model.md` encoding) | `Str::uuid7`, `HasUuids` |
| `validation` | Struct validation from `validate:"..."` tags, error bag by field path, Laravel 422 shape, messages via i18n | `validation.New(WithTranslator(t), WithClock(c)) (*Factory, error)` (`a.Validator`); `a.Validator.Struct(ctx, v) error` → `*validation.Error{Violations []Violation{Field, Rule, Param, Message}}`, `.Messages() map[string][]string`, `.Message()`; tags `required`, `omitempty`, `dive`, `keys`/`endkeys`, `len`, `min`, `max`, `gt`, `gte`, `lt`, `lte`, `eq`, `ne`, `oneof`, `email`, `url`, `uuid`, `alpha`, `alphanum`; custom `Rule` iface `Name/Message/Passes(ctx, value, param)` via `f.Register(rule)` (e.g. `public_id=usr` in app); `Validatable` input hook (`http`); `ErrNotStruct`, `ErrInvalidRule`; English Laravel-style messages (`validation.<rule>`, plural by `count`) in `Messages()`, translated per request locale. Not built: password rule (`NotCommon`, HIBP `Uncompromised`): app rule on `Register` until then | n/a | own rule engine over constant-name `reflect` (go-playground/validator removed: disables dead method elimination); `email` via stdlib `net/mail`; huma still validates JSON schema at bind | `Illuminate\Validation` |
| `sanitize` | HTML sanitizer policies | `UGC()`, `Strict()`, `NewPolicy().AllowElements(...).AllowAttrs(...).OnElements(...)/.Globally()/.Matching(re)`, `.AllowStyles(...)`, `.AllowURLSchemes(...)`, `.AllowDataURIImages()`; `p.Sanitize(html) string` | n/a | microcosm-cc/bluemonday | mews/purifier |
| `markdown` | Render (GFM, footnotes, alerts, heading ids, toc, link/image rewrite hooks, highlight with classes), plain text, neutral AST for converters (Slack mrkdwn, Discord, Telegram), front matter, warnings | `New(Options{GFM, Footnotes, Alerts, HeadingIDs, Highlight, RewriteLink func(Link) Link, Sanitize *sanitize.Policy}) *Renderer`; `r.HTML(src) (Result{HTML, TOC, Warnings}, error)`; `PlainText(src)`; `Parse(src) *Node`; `FrontMatter(src, maxBytes) (meta map[string]any, body []byte, err error)` (YAML + TOML, default cap 16 KB; 08 git/upload sources); `Policy()` (default sanitize policy); `StyleSheet(style)` (highlight CSS) | n/a | yuin/goldmark (+ GFM, footnotes), alecthomas/chroma v2 (highlight; +4.4 MB, PO accepted for now), go.abhg.dev/goldmark/toc, go.abhg.dev/goldmark/frontmatter v0.3.0 (BSD-3); alerts + heading ids + neutral AST own: (c) app zone cannot import goldmark AST, mapping ~150 lines | `Str::markdown` |
| `dom` | HTML DOM: parse, CSS select, mutate, render, text extract, HTML → Markdown, readability, charset decode | `Parse(r) (*Document, error)`; `doc.Find(sel) Selection`; `sel.Find/Each/Len/NextAll/Remove/Attr/SetAttr/Text/HTML`; `doc.HTML()`; `ToText(html)`; `ToMarkdown(html)`; `Readable(html, pageURL) (Page, error)` (`ErrNoContent`); `DecodeCharset(b, declared) (string, error)` | n/a | PuerkitoBio/goquery (on x/net/html + cascadia), golang.org/x/net/html/charset, k3a/html2text, JohannesKaufmann/html-to-markdown v2, codeberg.org/readeck/go-readability/v2 (MIT; successor of archived go-shiori) | symfony/dom-crawler |
| `i18n` | Translator (`lang/<locale>/<group>.json`), plurals, locale negotiation, formatting, key check, optional language detection | `i18n.New(Config{Locale, Locales}, files...)` (`a.Lang`; framework English messages loaded first, app `Options.Language` over them); `i18n.T(ctx, key, Args)`, `t.T(ctx, key, Args)`, `t.Lookup`; `Args{"name": v}` → `{{.name}}`, `count` picks plural form (CLDR); `WithLocale(ctx, l)`, `Locale(ctx)`; `t.Negotiate(acceptLanguage)`, `t.Match(locale)`, `t.Locales()`; `FormatNumber(locale, v)`, `FormatMoney(locale, minor, code)`, `t.FormatDate/FormatTime(locale, at, loc)` (known limits: month names English only, money symbol always first); `Keys(files...)`, `ScanKeys(code)`, `Check(used, files...) Report` (`lang:check`); config `app.locale` (default `en`), `app.locales`; `Detector` iface `Detect(text) (lang, confidence)` via `a.Languages` (config `language_detection`) | detector `none` (07 uses classifier `language` output); optional `whatlang` | golang.org/x/text (`language` matching, `feature/plural`, `message`, `number`, `currency`); own JSON loader and `{{.name}}` placeholder substitution (nicksnyder/go-i18n removed; no text/template: disables dead method elimination); detector optional abadojack/whatlanggo (tiny trigram tables). lingua-go rejected (110-1,800 MB, breaks 50 MB idle budget) | `Illuminate\Translation`, `__()` |
| `view/feed` | RSS 2.0 feed writer (Atom / JSON Feed later: `atom-json-feeds`) | `feed.RSS(w, Feed{Title, Link, Description, Updated, Items []Item{ID, Title, Link, Description, Content, Categories, Published, Updated}}) error` | n/a | gorilla/feeds v1.2.0 (BSD-3) | spatie/laravel-feed |
| `view` | templ rendering + Vite assets + embedded static serving | `Component` (alias of templ.Component); `HTML(ctx, c) (string, error)`; `Text(ctx, c)`; `Respond(w, r, status, c) error`; `WithNonce/Nonce(ctx)` (CSP); `vite.Manifest(fs, base) (*Assets, error)`, `assets.Tags(entry) Component`; `static.Handler(fs, Options)` | n/a | a-h/templ, olivere/vite, vearutop/statigz (precompressed br/gz from embed.FS), k3a/html2text (text alternative) | `Illuminate\View`, `Vite` |
| `console` | Command framework, output, prompts, `make:*` generator engine + default stubs | `Command` iface `Name()`, `Description()`, `Flags(*Flags)`, `Handle(ctx, *Input, *Output) error`; `console.New(name, version)`, `Register`, `Run(ctx, args, IO) int`; `flags.Argument/OptionalArgument/Bool/String/Int`; `in.Interactive()`; `out.Info/Warn/Error/Line/Table/Confirm/Progress`; `make.Generator{Stubs, Overrides, Module, Root, Clock}.Commands()` (dev build only; `make:action`, `command`, `event`, `job`, `listener`, `mail`, `migration`, `test`, `controller`, `request`, `policy`, `notification`; no `make:model`: sqlc generates models); stubs `console/stubs` | n/a | spf13/cobra (+ pflag), charmbracelet/huh (prompts), charmbracelet/lipgloss (tables), iancoleman/strcase (names); stubs on stdlib `text/template` (dev build only) | `Illuminate\Console`, Artisan, Prompts |
| `mcp` | MCP server plumbing (typed tools/resources/prompts, JSON schema from Go types, toolsets, confirmation hook, stateless streamable HTTP handler, stdio, auth hook, Origin check, protected resource metadata) + MCP client for remote servers (07 agent actions) | server: `mcp.New(ServerOptions{Name, Title, Version, Instructions, Errors, Confirm})`; `mcp.Tool[I, O](s, ToolDef, fn func(ctx, I) (O, error)) error`; `mcp.Resource(...)`, `mcp.Prompt(s, PromptDef, fn)`; `s.WithToolsets(ctx, names...)`; `s.HTTPHandler(Options{MaxBody, Auth, ResourceMetadata, AllowedOrigins})`; `s.ServeStdio(ctx)`; `ProtectedResourceHandler(ResourceMetadata)` (RFC 9728). client: `mcp.Connect(ctx, url, ClientOptions{Name, Version, HTTP: a.HTTP, Headers, Timeout}) (*Client, error)`; `c.ListTools(ctx, cursor)`, `c.CallTool(ctx, name, args) (ToolResult, error)`, `c.Close()`. No fake: tests run a real server in process | n/a | modelcontextprotocol/go-sdk v1.8.0 (`mcp`, `jsonrpc`, `auth`, `oauthex`; stateless; client over SSRF-guarded client), google/jsonschema-go + yosida95/uritemplate/v3 (go-sdk deps, used directly for schema types) | laravel/mcp |
| `webhooks` | Standard Webhooks v1 sign/verify, `whsec_` secrets, multi-signature header, secret rotation | `NewSecret() Secret`; `ParseSecret(s)`, `s.Encode()`; `Sign(secret, id, ts, body) string`, `SignAll(secrets, id, ts, body)`; `Headers(secrets, id, ts, body) http.Header`; `Verify(secrets []Secret, headers, body, Options) error` | n/a | standard-webhooks Go library (`github.com/standard-webhooks/standard-webhooks/libraries`, Apache-2.0); multi-secret loop own: (d). Only signer in codebase (10 outbound; any other spec signing webhooks uses it, no app-side signer package). `v1a` Ed25519 not at MVP | spatie/laravel-webhook-server/client |
| `image` | Upload-safe image handling + simple drawing | `Sniff(r) (Format, error)` (`f.ContentType()`); `DecodeConfig(r, Limits{MaxPixels, MaxBytes}) (Config, error)` (decompression bomb guard); `Decode(r, Limits) (image.Image, Config, error)`; `Reencode(r, Options{Format, Limits}) ([]byte, Config, error)` (EXIF orientation applied, then all metadata stripped; WebP and ICO written as PNG); ICO decoded (largest icon); AVIF refused with `ErrUnsupportedFormat` (pure-Go decoders cost +4 MB; browser-side conversion later if needed); `ErrTooLarge`, `ErrTooManyPixels`; `NewCanvas(w, h)` + `Fill`, `DrawText(face, ...)`, `DrawImage`, `EncodePNG`; `LoadFont(ttf, size) (*Face, error)` (`Measure`, `LineHeight`, `HasGlyph`) | n/a | stdlib `image/*` (png, jpeg, gif), golang.org/x/image (webp decode, draw, font/opentype), bep/imagemeta (EXIF orientation), sergeymakinen/go-ico (ICO); no cgo (bimg/govips rejected; disintegration/imaging stale) | intervention/image |
| `customdomain` | Customer hostnames: normalize (IDNA, apex/platform rules), ownership + routing verification (TXT, CNAME/A, HTTP probe handler), backoff schedule, on-demand TLS | `Normalize(host) (string, error)`, `IsApex(host)`; `NewVerifier(resolver, httpClient, Probe).Check(ctx, host, Expect{TXTName, TXTValue, CNAME, Addrs, ProbeToken}) (Result, error)`; `Probe.Handler()`, `Probe.Run(...)`; `DefaultSchedule().Next(failures, elapsed)`; `TLS` driver iface (`Ensure(ctx, host)`, `Remove(ctx, host)`, `TLSConfig()`, HTTP challenge handler) via `a.Domains` (config `custom_domains.default`, `custom_domains.tls.<name>`); app authorizer `foundation.Options.Domains` (`func(ctx, host) error`; nil refuses all); `ErrOwnershipMissing`, `ErrOwnershipMismatch`, `ErrRoutingMismatch`; `drivertest` (backend preloaded with a certificate, never reaches a CA); `storage.New(db, clock)` certmagic `Storage` on framework table `certmagic_storage` (`Migrations()`), locks = Postgres advisory locks (no `lock_expires_at`) | TLS drivers `none` (default), `certmagic` (terminates TLS in process), `caddyask` (external Caddy asks admin port `/internal/tls/ask`) | caddyserver/certmagic v0.25.6 (Apache-2.0, embedded; logs bridged through go.uber.org/zap), golang.org/x/net (idna, publicsuffix), `dns` package; Postgres storage own: (a) no licensed maintained impl (~150 lines). HTTPS listener for certmagic not wired yet (follow-up) | none |
| `support/diff` | Line and word diff engines | `diff.Lines(a, b) []Hunk` (unified); `diff.Words(a, b) []Segment` (semantic cleanup) | n/a | aymanbagabas/go-udiff v0.4.1 (BSD-3), sergi/go-diff v1.4.0 (MIT). diff3 not at MVP (later: `review-rebase`) | none |
| `support/str` | String helpers only (no collections: stdlib `slices`, `maps`) | `Random(n)` (crypto rand, base62), `Slug`, `Snake`, `Kebab`, `Studly`, `Plural`, `Singular` (uncached, ~140 µs per call: cache is a follow-up), `Limit`, `Mask(s, char, index, length)` | n/a | gosimple/slug, gertd/go-pluralize, iancoleman/strcase | `Str` |
| `testkit` | Gives each test its own slice of the real backing services, HTTP test client, database assertions, factories, arch checks | services: `testkit.Env(t, name)` (reads the app's normal env var, fails loudly when missing, never skips); `testkit.NewDatabase(t) Database{URL, AdminURL}` (database + non-superuser owner role per test from `DATABASE_URL`, dropped at cleanup); `testkit.NewMigratedDatabase(t, version, migrate)` (copy of a template database migrated once per version, `CREATE DATABASE ... TEMPLATE`; `testkit.Fingerprint(t, fs...)` derives the version from migration files); `testkit.NewValkey(t) *redis.Client`, `testkit.ValkeyConfig(t)` (shared `REDIS_URL`, unique key prefix per test); `testkit.NewBucket(t) string` (bucket per test on `AWS_ENDPOINT_URL_S3` with standard AWS vars, emptied and removed at cleanup); `testkit.UniqueName(prefix)`. HTTP: `testkit.NewClient(t, handler)` `.ActingAs(p)`, `.ImpersonateWith(fn)`, `.WithHeader`, `.WithCookie`, `.GetJSON/PostJSON/PatchJSON/PutJSON/Delete/Send` → `*Response` `.AssertStatus/AssertOK/AssertCreated/AssertNoContent/AssertUnauthorized/AssertForbidden/AssertNotFound/AssertJSONPath/AssertJSONMissing/AssertHeader/AssertValidationError(fields...)`, `.Decode(&v)`, `.String(path)`. DB: `testkit.Table(t, q, table).AssertHas(Row)/AssertMissing/AssertCount(n, where...)`, `AssertDatabaseHas/Missing/Count`. `factory.Define[T](func(seq) T)` `.Count(n)`, `.State(fn)`, `.Sequence(...)`, `.Persist(fn)`, `.Make/MakeOne/Create/CreateOne(ctx)`. `arch.Load(t, module)` / `LoadDir` → `m.Expect(globs...).Except(...).IgnoringTests().NotToImport(...)`, `.ToOnlyImport(...)`, `.NotToContainWords(...)`. Planned: app-level `TestCase` (booted app per test, fresh tenant, real commits; rules in testing.md) | services from root `compose.yaml` locally (`make services`), CI service containers; env per environment (`.env.testing`) | pgx (database/role setup), aws-sdk-go-v2 `s3` (buckets), stretchr/testify (assert core), tidwall/gjson (JSON paths); testcontainers-go rejected (per-run container start, Docker API coupling; compose + env vars instead), embedded-postgres rejected (no pgvector), miniredis rejected (not real Valkey); arch own: (a) no Go lib with Pest-style import + word rules (fdaines/arch-go is CLI+YAML, no word checks), ~150 lines on `go/build`, `go/ast` | `Illuminate\Foundation\Testing`, Pest `arch()` |
| `redis` | One Valkey client for all framework users (broadcasting, cache, ratelimit, pubsub); key building; topology (standalone, Sentinel, cluster per 11 operations.md section 11.3; same config builds centrifuge broker). Config `redis.url` (required; env `<PREFIX>_REDIS__URL`, `_FILE`; PaaS `REDIS_URL` alias is a follow-up), `redis.mode` (`auto`), `redis.addrs`, `redis.sentinel.master`, `redis.key_prefix` (default app name + `:`); later `redis.connect_retry` (60s), `redis.pool_size` (20). Unreachable at runtime: data-model.md "Valkey unreachable" policy | `Open(ctx, Config{URL, Mode, Addrs, Sentinel{Master}, KeyPrefix}) (*Client, error)` (pings; `ModeAuto` detects a cluster, `ModeStandalone`, `ModeSentinel`, `ModeCluster` fails with `ErrNotCluster` against a single server; `ErrInvalidMode`, `ErrSentinelMaster`, `ErrClusterDatabase`); `c.Ping(ctx)`; `c.Close()`; `c.Key(keyspace, tag, parts...)` (only key builder: wraps `tag` in `{}` so keys sharing a tag share one cluster slot); `c.Prefix(keyspace)`; `c.Rueidis()` (underlying client, framework packages only); `c.Option()` (client options, for framework services that build a dedicated client: cache-aside, locks) | Valkey 8 server (RESP; local + tests `valkey/valkey:8` in root `compose.yaml`, self-host compose `valkey/valkey:8-alpine` [verify tag], cloud ElastiCache for Valkey) | github.com/redis/rueidis (Apache-2.0; also linked by centrifuge Redis broker, so one client; valkey-io/valkey-go is a rueidis fork, not added: would be second client) | Redis facade |
| `resilience` | Circuit breaker + retry backoff for outbound providers | `NewBreaker(name, Settings) *Breaker` (`Execute(fn)`), `Backoff(Policy) iter.Seq[time.Duration]` | n/a | sony/gobreaker/v2, cenkalti/backoff/v5; retry budgets/fallback policy stay app (`app/support/llmretry`) | none |

Not in framework (decided): presence (inside `broadcasting`, centrifuge Redis presence on Valkey), locks (inside `cache`; correctness locks = `database` advisory locks), collections helpers (stdlib), Stripe (adapter zone `app/services/stripe`; Cashier is not Illuminate either), LLM/embeddings (domain contract, PO decision; official SDKs in adapter zone `app/drivers/llm/*`; may graduate to `framework/ai` later), MCP tool definitions (`app/mcp`), OAuth scope catalog + consent screen (app; storage is framework `oauth`), chat SDKs (adapter zone: slack-go, disgo, go-telegram/bot in `app/drivers/channel/*`), captcha (`app/services/turnstile` on httpclient), widget proof-of-work (adapter zone `app/services/altcha` wrapping altcha-org/altcha-lib-go; single-use tracking in app via `cache.Add`).

Realtime decision (PO 2026-11-02, Valkey required everywhere):
- Engine: centrifuge library embedded; broker = `centrifuge.NewRedisBroker`, presence = `centrifuge.NewRedisPresenceManager`, both on Valkey. History + recovery in Valkey streams. No custom broker, no Postgres realtime tables, no Centrifugo server code, no Centrifugo PRO features required.
- No memory broker, no broadcasting fake: every test uses real Valkey.
- Valkey holds only rebuildable state (data-model.md "Valkey"): loss = reconnect + `recovered: false` + REST refetch.

## 4. Driver matrix

Every swappable service: `framework/<svc>/drivertest` suite every driver passes (built-in, community, and the fake where one exists). `Fake()` only at process boundaries (3), testing env only; everything else is tested against the real service. Community drivers register via `Manager.Extend` in `app/providers/app_service_provider.go` (or PR into framework when broadly useful).

| Service | Local dev / test | Self-host default | Cloud (AWS) default | Built-in alternates | Community / later |
|---|---|---|---|---|---|
| Filesystem (`filesystem`, aws-sdk-go-v2 s3 + stdlib `os.Root`) | `local` (`<data_dir>/app`); tests: `local` disk in `t.TempDir()`, no fake; `s3` driver + `drivertest` against SeaweedFS S3 from root `compose.yaml` (bucket per test, `testkit.NewBucket`) | `local` (single node); `s3` required when replicas > 1 | `s3` | `s3` covers every S3-compatible store by config only (`endpoint`, `region`, `force_path_style`): Cloudflare R2, Backblaze B2, Wasabi, DigitalOcean Spaces, SeaweedFS, Garage, RustFS, GCS XML interop | `gcs` native, `azure` blob (non-S3 APIs only): own drivers via `Extend` on the vendor SDK, kept out of default binary for size (no gocloud) |
| Mail out (`mail`, named mailers `system` / `customer`) | `log` (default when none configured), or `smtp` to Mailpit (compose SMTP 11025); tests `a.Mail.Fake()`, `smtp` driver tests read Mailpit API (18025) | one instance provider from deploy config: `smtp` relay or any provider API | `ses` (configuration sets `dimmi-system` / `dimmi-customer`, tenant per workspace) | `postmark`, `resend`, `mailgun`, `sendgrid`, `log` | more via `Extend` |
| Mail in (`mailbox`) | SES SNS fixtures; no fake: tests feed fixture bodies to the real `ses` receiver | `ses` (own AWS) | `ses` (receipt rule → S3 + SNS) | `postmark`, `mailgun`, `sendgrid`, `resend`, `cloudflare`; account drivers `gmail`, `graph`; `imap` client (04 sources.md, mailbox-connections.md) | none; no SMTP receiver |
| Cache (`cache`) | Valkey from root `compose.yaml`; tests real Valkey (key prefix per test), no fake | Valkey | ElastiCache for Valkey | none (not swappable: Valkey only) | none |
| Rate limit (`ratelimit`) | Valkey; tests real Valkey, no fake | Valkey | ElastiCache for Valkey | `Local` in-process token bucket (per-job pacing) | none |
| PubSub (`pubsub`, internal bus) | Valkey; tests real Valkey, no fake | Valkey | ElastiCache for Valkey | none | `nats` |
| Broadcasting broker (`broadcasting`, centrifuge) | Redis broker on Valkey; tests real Valkey + real client, no fake | Redis broker on Valkey | Redis broker on ElastiCache for Valkey | none | none |
| KEK (`encryption`; data keys = Tink keysets) | `local` (derived from app key); tests real `local`, no fake | `local` | `awskms` (own thin driver on aws-sdk-go-v2 `kms`) | | `vault`, `gcpkms` via `Extend` |
| Malware scan (`malware`) | `none`; tests `a.Scanner.Fake()` (scripted verdicts, EICAR) | `none` | `guardduty` (S3 malware protection result tags) | `clamav` (clamd TCP) | `virustotal` |
| HTTP client (`httpclient`) | safe transport; tests `httpclienttest.New(a.HTTP).PreventStrayRequests()` | safe | safe | | |
| DNS (`dns`) | system resolver; tests `a.DNS.Fake()` | system | system | `dns.nameserver` override | DoH resolver |
| Clock (`clock`) | `clock.System()`; tests `clock.NewFake(t)` `Freeze`/`Travel` | system | system | | `postgres` offset sync (not built) |
| Password hashing (`hashing`) | `argon2id` (low cost in tests via config) | `argon2id` | `argon2id` | `bcrypt` (imports) | `scrypt` |
| Telemetry (`telemetry`) | tracing `none`, or `otlp` (opt-in package `telemetry/otlp`) to local Jaeger/Grafana | tracing `none` + metrics `prometheus` on admin port | `otlp` (ADOT collector; app links `otlp.Tracing()`) + `prometheus` | sentry errors (opt-in `sentry_dsn`) | Honeycomb/Datadog via OTLP (no code) |
| Logging (`log`) | `text` | `json` stdout | `json` stdout | | |
| Notifications channels (`notifications`) | all; no fake: tests assert rows, jobs, and the mail fake / Mailpit | `mail`, `database`, `broadcast`, `webpush` | same | | app registers `slack` (06; `discord` later: `discord-notifications`) |
| Social login (`socialite`) | `github`, `google` against test apps; tests `a.Social.Fake()` or `socialite/drivertest` local identity provider | configured providers | same | `oidc` (enterprise) | `saml` (crewjam), `microsoft`, `gitlab` |
| Custom domain TLS (`customdomain`) | `none`; every driver passes `drivertest` (no CA reached) | `none` (own proxy) or `caddyask` / `certmagic` | `none` (ALB/CloudFront) | `certmagic`, `caddyask` | |
| Language detect (`i18n`) | `none`; tests real detector, no fake | `none` (07 uses classifier output) | `none` | `whatlang` (optional, tiny) | |
| LLM (domain, 07, `app/managers` on `framework/manager`) | `fake` scripted; `openaicompat` to Ollama | per workspace BYOK | `bedrock` + BYOK | `openai`, `anthropic`, `gemini`, `openrouter`, `openaicompat` | more via `Extend` |
| Channels (domain, 03/05) | `fake` | all built-in | all built-in | widget, email, api, slack, discord, telegram, whatsapp | via `Extend` + `tests/contracts/channel_test.go` |

### 4.1 Not swappable (explicit)

| Service | Fixed choice | Why | Still wrapped because |
|---|---|---|---|
| Database (`database`) | PostgreSQL 18 (support 17+), pgx | queue, pubsub, FTS, vectors, RLS, advisory locks all in Postgres; dual-DB doubles every heavy feature (`research/tech-stack.md` B) | tx hooks (AfterCommit/AfterRollback), tenant `SET LOCAL`, error mapping, test harness, app never names pgx |
| Queue (`queue`) | River on Postgres | transactional enqueue (`PushTx`) is the reason; no second driver at MVP | API kept thin (`Args`, `Worker[A]`, `Push/PushTx`, `Postpone/Cancel/RetryAfter`, `Drain`) so a second driver stays possible, River types never leak |
| Scheduler (`schedule`) | River periodic + leader election (scheduler-role nodes only) | same as queue | Laravel-style fluent API |
| Full-text + vector search | Postgres FTS + pg_trgm + pgvector (inside domain `Search`/`Knowledge`) | works on every managed Postgres; BM25 extension optional adapter later | n/a (domain code over `database`) |
| Migrations (`database/migrate`) | goose, plain SQL | | lock, lock/statement timeouts + retry, plan, lint, library steps, two version tables, guarded production commands |
| HTTP router/OpenAPI (`http`, `routing`) | chi + huma | OpenAPI drives CLI, MCP, SDKs | app never imports chi/huma |
| Realtime engine (`broadcasting`) | centrifuge library + its Redis broker/presence on Valkey | presence, history, recovery, transports come from one lib | app sees `Broadcast` + channel authorizers only |
| Valkey (`redis`) | Valkey 8, required in every deployment | realtime, cache, rate limits, pubsub, dedupe windows need shared fast state across replicas; PO decision | holds only rebuildable state; `maxmemory-policy noeviction` (11 decision 19), every key has TTL; AOF `everysec` recommended, never relied on |

### 4.2 Config shape (filesystem example, Laravel `filesystems.disks` style)

```yaml
# dimmi.yaml
filesystems:
  default: attachments
  disks:
    attachments:
      driver: s3
      bucket: dimmi-attachments
      region: auto
      endpoint: https://<account>.r2.cloudflarestorage.com   # empty = AWS
      force_path_style: false                                # true for SeaweedFS, Garage
      access_key_id: ""          # empty = default AWS credential chain
      secret_access_key_file: /run/secrets/r2_secret
      url: ""                    # public base URL (CDN); empty = private, URL() returns ErrNoPublicURL
    backups:
      driver: s3
      bucket: dimmi-backups
      region: eu-central-1
    public_assets:
      driver: local
      root: storage/app/public
      url: https://help.example.com/assets
    exports:
      driver: local
      root: storage/app/exports
      signed_url_base: https://dimmi.example.com/files   # temporary URLs need both keys;
      signing_key_file: /run/secrets/files_signing_key   # without them the disk refuses to sign
    ses_inbound:
      driver: s3
      bucket: dimmi-ses-inbound
      region: eu-west-1
```

Env override (koanf, `__` = nesting, `_FILE` = read secret from file):
```
DIMMI_FILESYSTEMS__DEFAULT=attachments
DIMMI_FILESYSTEMS__DISKS__ATTACHMENTS__DRIVER=s3
DIMMI_FILESYSTEMS__DISKS__ATTACHMENTS__ENDPOINT=http://seaweedfs:8333
DIMMI_FILESYSTEMS__DISKS__ATTACHMENTS__FORCE_PATH_STYLE=true
DIMMI_FILESYSTEMS__DISKS__ATTACHMENTS__SECRET_ACCESS_KEY_FILE=/run/secrets/s3_secret
```

Go side:
```go
// config/filesystems.go (app)
type Filesystems = filesystem.Config // {Default string; Disks map[string]config.Section}

// usage
disk, err := a.Storage.Disk("attachments")
err = disk.PutStream(ctx, path, r, filesystem.PutOptions{ContentType: ct})
attrs, err := disk.Stat(ctx, path)
url, err := disk.TemporaryURL(ctx, path, 5*time.Minute)
upload, err := disk.TemporaryUploadURL(ctx, path, filesystem.UploadURLOptions{TTL: 10 * time.Minute, ContentType: ct})
for entry, err := range disk.List(ctx, "exports/") {
    if err != nil {
        return err
    }
    paths = append(paths, entry.Path)
}
```
Same shape for every manager: `mail.mailers.<name>.driver`, `encryption.keks.<name>.driver`, `malware.scanners.<name>.driver`, `mailbox.sources.<name>.driver`, `socialite.providers.<name>.driver`, `custom_domains.tls.<name>.driver`, `hashing.hashers.<name>.driver`, `language_detection.detectors.<name>.driver`. Cache has no manager: one store on Valkey, `cache.keyspace` only. Driver options sit flat beside `driver` in the instance (`mail.mailers.system.driver: postmark`, `mail.mailers.system.token_file: ...`), never nested under the driver name; `mail.default` = default mailer instance name (`system`), not a driver (contracts.md "Mail"). Valkey: one `redis.url` (env `DIMMI_REDIS__URL`, `_FILE` supported; PaaS aliases `REDIS_URL`/`DATABASE_URL`/`PORT` are a follow-up) shared by cache, ratelimit, pubsub, broadcasting with keyspace prefixes.

## 5. Container-free DI and boot

- No container, no reflection, no facades. Services are typed fields on `*foundation.Application`; app `*App` embeds it.

```go
// framework/foundation (application.go)
type Application struct {
    Name        string
    Env         Environment               // local, testing, production
    Roles       Roles                     // web, worker, scheduler
    Config      *Config                   // framework config, one section per service
    Log         log.Logger
    Clock       clock.Clock
    Telemetry   *telemetry.Telemetry
    DB          *database.DB
    Migrator    migrate.Runner
    Redis       *redis.Client
    Encrypter   *encryption.AppEncrypter  // app key
    Crypt       *encryption.Manager       // KEKs; a.Crypt.Keys(store) builds the KeyManager
    Hash        *hashing.Manager
    Cache       *cache.Store
    Limiter     *ratelimit.Limiter
    PubSub      *pubsub.Bus
    Queue       *queue.Queue
    Schedule    *schedule.Schedule
    Events      *events.Dispatcher
    Broadcast   *broadcasting.Server
    HTTP        *httpclient.Client
    DNS         *dns.Client
    Process     *process.Process
    Storage     *filesystem.Manager
    Scanner     *malware.Manager
    Mail        *mail.Manager
    Mailbox     *mailbox.Manager
    Notify      *notifications.Manager
    IDs         *ids.Registry
    Validator   *validation.Factory
    Lang        *i18n.Translator
    Languages   *i18n.DetectorManager
    Errors      fhttp.Errors              // renders failures for every HTTP layer
    Guards      *auth.Guards
    Gate        *auth.Gate
    Session     *session.Manager
    Social      *socialite.Manager
    Domains     *customdomain.Manager
    Idempotency *idempotency.DatabaseStore
    Maintenance Maintenance               // shared by every replica
    Router      *routing.Router
    URL         *routing.URL
    Kernel      *fhttp.Kernel
    Health      *health.Registry
}

// app/app.go
type App struct {
    *foundation.Application
    Config    *config.Config            // app typed config (root config/ package)
    Actions   *Actions                  // all actions, constructed once
    OAuth     *oauth.Server             // oauth.New(a.DB, a.Clock, a.Limiter, opts)
    Knowledge contracts.Knowledge
    Channels  *managers.ChannelManager
    LLM       *managers.LLMManager
    Billing   contracts.BillingStatusReader
}
```

- Pure helpers stay package funcs (`markdown.PlainText`, `ids.New`, `hashing.SHA256`); anything with config, state, or a fake hangs off `Application`. No console field: the command line is `foundation.Run`.
- Services the app builds from framework parts (`oauth.Server`, `token.Tokens`, `encryption.Keys` with the app keyset store, `twofactor.TOTP`) live on `App`.
- Actions get deps by constructor struct (`&conversations.ReplyToConversation{DB: a.DB, Events: a.Events, Clock: a.Clock}`), never `*App` globally.

Boot order (`foundation.New`; `foundation.Run` boots on the first command that needs the app):
1. Config load + validate (`LoadConfig`: defaults, YAML, env, `_FILE`; fail fast with `*ConfigError` holding the redacted config). Fakes allowed only when `app.env: testing`.
2. Observability: log, clock (`clock.System()`), telemetry (+ `Options.Telemetry` exporters), GOMEMLIMIT (`runtime.memory_limit: auto` = 90 % of the cgroup limit).
3. Data: database pool (otelpgx traced; refuses a standby); migrator built (library steps + framework set + app set); pending migrations applied when `migrations.auto` (default on; cluster-wide advisory lock, `migrations.lock_timeout` 10m); Valkey client (`PING`, fail fast).
4. Security: app key (`app.key`, else `<data_dir>/app.key`, generated only with `app.key_auto_generate`; else `ErrNoAppKey`), app encrypter, KEK manager, hashers.
5. Valkey services: cache, rate limiter, pubsub; queue (role-aware: scheduler-only node listens on leader queue only, non-scheduler node `NoLeader`), schedule, events; broadcasting server (centrifuge + Redis broker on Valkey; connections refused without `Options.Broadcast`); maintenance store (`<data_dir>/framework/down`, changes on pubsub).
6. IO: httpclient, dns, process, filesystem, malware, translator (framework English + `Options.Language`), mail, mailbox, notifications; built-in drivers registered (`Extend`); mail can queue.
7. Web: ids registry, validator (translated), language detectors, error renderer, health (database, Valkey, maintenance); guards, gate, session (Postgres store), socialite, custom domains, idempotency store; kernel (RequestID, Locale, Recover, otelhttp, TrustProxies, Maintenance) + aliases, router, URL generator (signing key derived from app key).
8. App providers `Register` (all, in listed order), then `Boot` (same order). Register only binds; Boot may use other services. Providers implementing `Shutdowner` get a shutdown hook.
9. `Start`/`Serve`: roles `worker`/`scheduler` start River (workers; leader election only on scheduler nodes), `web` starts public (`http.addr` :8080) + admin (`http.admin_addr` 127.0.0.1:9090: `/up`, `/ready`, `/metrics`) listeners.
10. Shutdown (SIGTERM, reverse of start/boot): stop listeners, broadcasting connections (clients reconnect elsewhere, recover from history), queue workers (job timeout), provider `Shutdown` (flush buffered writers: usage recorders, install trackers), maintenance, queue, cache, Valkey, pool, telemetry. Budget `http.shutdown_timeout` 25 s. Boot failing halfway closes what it opened.

## 6. How app code uses it (before / after)

### 6.1 Hash password in action

Before (`app/support/auth/password_hasher.go`, app imports `golang.org/x/crypto/argon2`, builds PHC string by hand):
```go
salt := make([]byte, 16)
_, _ = rand.Read(salt)
key := argon2.IDKey([]byte(norm.NFKC.String(pw)), salt, 3, 64*1024, 2, 32)
encoded := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", 64*1024, 3, 2, b64(salt), b64(key))
```
After:
```go
type RegisterUser struct {
    DB     *database.DB
    Hash   *hashing.Manager
    Events *events.Dispatcher
    Clock  clock.Clock
}

func (a *RegisterUser) Handle(ctx context.Context, in RegisterInput) (*models.User, error) {
    hash, err := a.Hash.Make(ctx, in.Password) // argon2id, PHC, NFKC, semaphore (hashing.ErrBusy -> 503 auth_busy)
    if err != nil {
        return nil, err
    }
    var user models.User
    err = a.DB.Transaction(ctx, func(ctx context.Context, tx *database.Tx) error {
        user, err = models.New(tx).CreateUser(ctx, models.CreateUserParams{
            ID: ids.New(), Email: in.Email, PasswordHash: hash,
        })
        if database.IsUniqueViolation(err, "users_email_key") {
            return exceptions.ErrEmailTaken // fault.Conflict("email_taken", ...)
        }
        if err != nil {
            return err
        }
        return a.Events.Dispatch(ctx, tx, appevents.UserRegistered{UserID: user.ID})
    })
    return &user, err
}

// login
ok, err := a.Hash.Check(ctx, in.Password, user.PasswordHash)
if ok && a.Hash.NeedsRehash(user.PasswordHash) { /* rehash + save */ }
```

### 6.2 Dispatch event, listeners by phase

```go
// action
return a.Events.Dispatch(ctx, tx, appevents.MessageCreated{MessageID: msg.ID, ConversationID: conv.ID})

// app/providers/event_service_provider.go
func (EventServiceProvider) Boot(ctx context.Context, a *app.App) error {
    d := a.Events
    events.Listen(d, events.InTx, listeners.RecordBillableConversation(a))   // may abort action
    events.Listen(d, events.InTx, listeners.WriteAuditLog(a))
    events.Listen(d, events.AfterCommit, listeners.BroadcastMessageCreated(a)) // errors logged only
    events.ListenQueued(d, func(e appevents.MessageCreated) queue.Args {      // River insert in same tx
        return jobs.QueueWebhookDeliveriesArgs{EventType: "message.created", ObjectID: e.MessageID}
    })
    return nil
}
```

### 6.3 Queue job

```go
// app/jobs/deliver_webhook.go
type DeliverWebhookArgs struct {
    DeliveryID ids.UUID `json:"delivery_id"`
}

func (DeliverWebhookArgs) Kind() string { return "deliver_webhook" }
func (DeliverWebhookArgs) Options() queue.Options {
    return queue.Options{Queue: "webhooks", MaxAttempts: 12, Unique: queue.UniqueOpts{ByArgs: true}}
}

type DeliverWebhookWorker struct{ Attempt *webhooks.AttemptWebhookDelivery }

func (w *DeliverWebhookWorker) Handle(ctx context.Context, job *queue.Job[DeliverWebhookArgs]) error {
    res, err := w.Attempt.Handle(ctx, job.Args.DeliveryID)
    if errors.Is(err, database.ErrNotFound) {
        return queue.Cancel(err) // row gone: stop retrying
    }
    if res.RetryAfter > 0 {
        return queue.Postpone(res.RetryAfter) // run later, attempt not spent
    }
    return err
}

// app/providers/app_service_provider.go
queue.Register(a.Queue, &jobs.DeliverWebhookWorker{Attempt: a.Actions.AttemptWebhookDelivery})

// push from action, same tx as rows
err := a.Queue.PushTx(ctx, tx, jobs.DeliverWebhookArgs{DeliveryID: d.ID})
```

### 6.4 Send mail

```go
// app/mail/invitation_mail.go
type InvitationMail struct {
    InvitationID ids.UUID `json:"invitation_id"`
    Inviter      string   `json:"inviter"`
    Workspace    string   `json:"workspace"`
    AcceptURL    string   `json:"accept_url"`
}

func (m InvitationMail) Envelope() mail.Envelope {
    return mail.Envelope{Subject: fmt.Sprintf("%s invited you to %s", m.Inviter, m.Workspace)}
}

func (m InvitationMail) Content(ctx context.Context) (mail.Content, error) {
    return mail.View(views.InvitationMail(m.Inviter, m.Workspace, m.AcceptURL)), nil // templ html + text; i18n.T(ctx, ...) inside
}

// app/providers/app_service_provider.go (queued mailables must be registered)
mail.Register[appmail.InvitationMail](a.Mail, "invitation")

// action: sync `.Send(ctx, m)`, or queued in same tx; renders in the recipient's locale
url, err := a.URL.SignedRoute("invitations.show", map[string]string{"invitation": pub}, 7*24*time.Hour)
if err != nil {
    return err
}
err = a.Mail.To(mail.Address{Email: inv.Email}).
    Locale(inv.Locale). // recipient locale; empty = locale of ctx
    QueueTx(ctx, tx, appmail.InvitationMail{
        InvitationID: inv.ID, Inviter: inviter.Name, Workspace: ws.Name, AcceptURL: url,
    })
```

### 6.5 HTTP call via safe client

Before (09 CIMD fetch):
```go
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, clientID, nil)
req.Header.Set("Accept", "application/json")
resp, err := safehttp.Client.Do(req)
// manual redirect block, size cap, decode
```
After:
```go
resp, err := a.HTTP.New().
    Accept("application/json").
    Timeout(5 * time.Second).
    MaxBody(16 << 10).
    WithoutRedirects().
    Get(ctx, clientID)
if errors.Is(err, httpclient.ErrBlockedHost) {
    return fault.New(fault.Invalid, "invalid_client_metadata", "Client metadata host not allowed.")
}
if err != nil {
    return err
}
var doc ClientMetadata // app type: CIMD is deferred in framework/oauth
if err := resp.JSON(&doc); err != nil {
    return err
}
```
Adapter zone SDK (07 anthropic driver):
```go
client := anthropic.NewClient(option.WithHTTPClient(a.HTTP.StdClient()), option.WithMaxRetries(0))
```

### 6.6 Test against real services, fakes at process boundaries

```go
func TestInvitingMemberQueuesMail(t *testing.T) {
    tc := tests.New(t) // app TestCase (planned): app booted with app.env testing on testkit.NewMigratedDatabase + NewValkey + NewBucket
    mails := tc.App.Mail.Fake()                             // mail transport: process boundary
    httpclienttest.New(tc.App.HTTP).PreventStrayRequests() // outbound HTTP: process boundary

    owner := factories.Member(tc).Owner().CreateOne(t.Context())

    tc.Client().ActingAs(owner).
        PostJSON("/api/v1/invitations", testkit.JSON{"emails": []string{"ana@example.com"}}).
        AssertCreated().
        AssertJSONPath("data.0.email", "ana@example.com")

    testkit.Table(t, tc.Admin, "invitations").AssertHas(testkit.Row{"email": "ana@example.com"}) // real Postgres (admin querier: RLS)
    require.NoError(t, tc.App.Queue.Drain(t.Context()))                                           // real River jobs run
    mail.AssertSent(t, mails, func(m appmail.InvitationMail) bool { return m.Inviter == owner.Name })
}

func TestInvitationValidation(t *testing.T) {
    tc := tests.New(t)
    owner := factories.Member(tc).Owner().CreateOne(t.Context())
    tc.Client().ActingAs(owner).
        PostJSON("/api/v1/invitations", testkit.JSON{"emails": []string{"nope"}}).
        AssertStatus(422).
        AssertValidationError("emails.0")
}

func TestInvitationExpires(t *testing.T) {
    tc := tests.New(t)
    clk := clock.NewFake(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)) // clock: handed to the action under test
    accept := &invitations.AcceptInvitation{DB: tc.App.DB, Clock: clk}
    inv := factories.Invitation(tc).CreateOne(t.Context())
    clk.Travel(8 * 24 * time.Hour)
    _, err := accept.Handle(t.Context(), inv.Token)
    assert.Equal(t, fault.Gone, fault.KindOf(err))
}
```
Generic assertions are package funcs (`mail.AssertSent[M]`): Go methods cannot take type params. No `queue.AssertPushed`, `events.AssertDispatched`, `AssertBroadcastOn`: run jobs with `a.Queue.Drain(ctx)` and assert effects (rows, mail fake, Mailpit, a real realtime client). Every `Fake()` panics outside `app.env: testing`.

## 7. Enforcement

### 7.1 `.golangci.yml` (import + identifier rules)

Notes:
- depguard v2: strict mode allows only `allow`; when allow and deny both match, longer match wins. `$gostd` expands to every std package path, so each stdlib deny uses exact form `pkg$` (one char longer, wins). Prefix deny on stdlib does not work against `$gostd`.
- `files` globs match absolute paths. Build and CI never check out under a path with `/app/` or `/framework/` segment (Dockerfile build stage uses `/src`). Wrong checkout path fails loudly (framework files hit app rules), never silently.
- Authoritative backstop: `tests/arch/*_test.go` (`framework/testkit/arch`, `go/build` import graph, module-relative paths). Lint is fast feedback; arch test is truth. Planned: lint self-test `tests/arch/testdata/lintviolations/` must produce known findings in CI.
- `.golangci.yml` today carries framework rules only (app zone rules below land with `app/`); its full linter set is in code-style.md. `make lint` runs twice: without and with `--build-tags dev`.
- Generated files skipped (`exclusions.generated: strict`: only standard `Code generated ... DO NOT EDIT.` header).

```yaml
version: "2"

run:
  relative-path-mode: gomod

linters:
  enable:
    - depguard
    - forbidigo
    - importas
    - gochecknoinits
  exclusions:
    generated: strict
    rules:
      # tag-scoped forbidigo exemptions (tag in msg, see forbid list)
      - path: '^framework/clock/'
        linters: [forbidigo]
        text: '\[clock\]'
      - path: '^framework/httpclient/'
        linters: [forbidigo]
        text: '\[httpclient\]'
      - path: '^framework/foundation/'
        linters: [forbidigo]
        text: 'servers started by framework/foundation only'
      - path: '^(cmd/|framework/foundation/|framework/testkit/|framework/console/)'
        linters: [forbidigo]
        text: '\[ctx\]|\[print\]'
      - path: '_test\.go$'
        linters: [forbidigo]
        text: '\[ctx\]'
      - path: ^framework/
        linters: [forbidigo]
        text: '\[panic\]'      # framework panics only for programming errors (Must*, fakes outside tests)
  settings:
    depguard:
      rules:
        # ---------------- framework ----------------
        framework:
          list-mode: strict
          files:
            - "**/framework/**/*.go"
          allow:
            - $gostd
            - go/ast              # testkit/arch, i18n ScanKeys (listed explicitly in .golangci.yml)
            - go/build
            - go/parser
            - go/token
            - github.com/dimmihq/dimmi/framework
            # http, routing
            - github.com/go-chi/chi/v5
            - github.com/go-chi/httprate
            - github.com/danielgtaylor/huma/v2
            - github.com/rs/cors
            - github.com/realclientip/realclientip-go
            - github.com/medama-io/go-useragent
            # database, queue, schedule
            - github.com/jackc/pgx/v5
            - github.com/jackc/puddle/v2
            - github.com/pgvector/pgvector-go
            - github.com/pressly/goose/v3
            - github.com/riverqueue/river
            - riverqueue.com/riverui         # still allowed in .golangci.yml; unused: River UI deferred
            - github.com/robfig/cron/v3
            # config, foundation, console
            - github.com/knadh/koanf
            - github.com/KimMachineGun/automemlimit
            - github.com/spf13/cobra
            - github.com/spf13/pflag
            - github.com/charmbracelet/huh
            - github.com/charmbracelet/lipgloss
            # cache, realtime, valkey, resilience
            - github.com/maypok86/otter/v2
            - github.com/centrifugal/centrifuge
            - github.com/redis/rueidis
            - github.com/coder/websocket
            - github.com/sony/gobreaker/v2
            - github.com/cenkalti/backoff/v5
            # security
            - golang.org/x/crypto
            - golang.org/x/oauth2
            - golang.org/x/sync
            - golang.org/x/time
            - github.com/alexedwards/argon2id
            - github.com/tink-crypto/tink-go/v2
            - github.com/tink-crypto/tink-go-awskms/v2   # still allowed in .golangci.yml; unused: awskms KEK is own code on aws-sdk-go-v2 kms
            - github.com/pquerna/otp
            - github.com/go-jose/go-jose/v4
            - github.com/coreos/go-oidc/v3
            - github.com/standard-webhooks/standard-webhooks/libraries
            - github.com/SherClockHolmes/webpush-go
            # ids, time, strings
            - github.com/google/uuid
            - go.jetify.com/typeid
            - github.com/jonboulle/clockwork
            - github.com/gosimple/slug
            - github.com/gertd/go-pluralize
            - github.com/iancoleman/strcase
            # content
            - github.com/a-h/templ
            - github.com/olivere/vite
            - github.com/vearutop/statigz
            - github.com/yuin/goldmark
            - go.abhg.dev/goldmark/toc
            - github.com/alecthomas/chroma/v2
            - github.com/microcosm-cc/bluemonday
            - go.abhg.dev/goldmark/frontmatter
            - github.com/gorilla/feeds
            - github.com/aymanbagabas/go-udiff
            - github.com/sergi/go-diff
            - golang.org/x/image
            - github.com/bep/imagemeta
            - github.com/sergeymakinen/go-ico
            - github.com/PuerkitoBio/goquery
            - github.com/andybalholm/cascadia
            - github.com/k3a/html2text
            - github.com/JohannesKaufmann/html-to-markdown/v2
            - codeberg.org/readeck/go-readability/v2
            - golang.org/x/net
            - golang.org/x/text
            - github.com/abadojack/whatlanggo
            # storage, mail, cloud
            - github.com/aws/aws-sdk-go-v2
            - github.com/aws/smithy-go
            - github.com/jhillyerd/enmime
            - github.com/emersion/go-message
            - github.com/emersion/go-msgauth
            - blitiri.com.ar/go/spf
            - github.com/wneessen/go-mail
            - github.com/emersion/go-sasl
            # http client
            - resty.dev/v3
            - code.dny.dev/ssrf
            - github.com/jarcoal/httpmock    # httpclient/httpclienttest only
            - github.com/temoto/robotstxt
            # custom domains, TLS
            - github.com/caddyserver/certmagic
            - go.uber.org/zap                # only to bridge certmagic logs
            # observability
            - github.com/m-mizutani/masq
            - go.opentelemetry.io/otel
            - go.opentelemetry.io/contrib
            - github.com/exaring/otelpgx
            - github.com/prometheus/client_golang
            - github.com/getsentry/sentry-go
            # mcp (server + client)
            - github.com/modelcontextprotocol/go-sdk
            - github.com/google/jsonschema-go
            - github.com/yosida95/uritemplate/v3
            # testkit only (framework-test-only rule)
            - github.com/stretchr/testify
            - github.com/google/go-cmp
            - github.com/tidwall/gjson
          deny:
            - pkg: "database/sql$"
              desc: "framework/database uses pgx; database/sql/driver allowed for Valuer/Scanner"
            - pkg: "math/rand$"
              desc: "use crypto/rand or math/rand/v2"
            - pkg: "log$"
              desc: "use log/slog via framework/log"
        framework-exec:
          list-mode: lax
          files:
            - "**/framework/**/*.go"
            - "!**/framework/process/**"
          deny:
            - pkg: "os/exec"
              desc: "only framework/process runs programs"
        framework-test-only:
          list-mode: lax
          files:
            - "**/framework/**/*.go"
            - "!**/framework/testkit/**"
            - "!$test"
            - "!**/framework/**/drivertest/**"   # contract suites are test code in non-test files
          deny:
            - pkg: "github.com/dimmihq/dimmi/framework/testkit"
              desc: "testkit only in tests"
            - pkg: "github.com/stretchr/testify"
              desc: "testkit only"
        framework-banned:
          list-mode: lax
          files: ["**/*.go"]
          deny:
            - { pkg: "github.com/fergusstrange/embedded-postgres", desc: "no pgvector; real Postgres only" }
            - { pkg: "github.com/alicebob/miniredis", desc: "real Valkey only" }
            - { pkg: "github.com/redis/go-redis", desc: "one Valkey client: rueidis" }
            - { pkg: "github.com/valkey-io/valkey-go", desc: "one Valkey client: rueidis" }
            - { pkg: "github.com/pemistahl/lingua-go", desc: "memory budget" }
            - { pkg: "github.com/disgoorg/disgo/voice", desc: "cgo" }
            - { pkg: "github.com/emersion/go-smtp", desc: "no SMTP receiver" }

        # ---------------- app zone ----------------
        app:
          list-mode: strict
          files:
            - "**/app/**/*.go"
            - "**/routes/**/*.go"
            - "**/config/**/*.go"
            - "**/bootstrap/**/*.go"
            - "**/database/**/*.go"
            - "**/tests/**/*.go"
            - "!**/app/drivers/**"
            - "!**/app/services/**"
            - "!**/app/support/openapiimport/**"
            - "!**/app/support/actionschema/**"
            - "!**/app/support/actiontemplate/**"
            - "!**/app/support/actionresult/**"
            - "!**/tests/arch/**"
          allow:
            - $gostd
            - github.com/dimmihq/dimmi
          deny: &app_std_deny
            - { pkg: "database/sql$", desc: "framework/database" }
            - { pkg: "database/sql/driver$", desc: "framework/database" }
            - { pkg: "os$", desc: "framework/filesystem, framework/config" }
            - { pkg: "os/exec$", desc: "framework/process" }
            - { pkg: "os/signal$", desc: "framework/foundation" }
            - { pkg: "os/user$", desc: "not needed" }
            - { pkg: "syscall$", desc: "not needed" }
            - { pkg: "unsafe$", desc: "not needed" }
            - { pkg: "plugin$", desc: "not needed" }
            - { pkg: "reflect$", desc: "no reflection; explicit code" }
            - { pkg: "runtime$", desc: "framework/foundation" }
            - { pkg: "runtime/debug$", desc: "framework/foundation" }
            - { pkg: "runtime/pprof$", desc: "framework/telemetry" }
            - { pkg: "expvar$", desc: "framework/telemetry" }
            - { pkg: "flag$", desc: "framework/console" }
            - { pkg: "log$", desc: "framework/log" }
            - { pkg: "log/slog$", desc: "framework/log" }
            - { pkg: "log/syslog$", desc: "framework/log" }
            - { pkg: "net$", desc: "framework/dns, framework/httpclient/safedial" }
            - { pkg: "net/http$", desc: "framework/http (server aliases), framework/httpclient" }
            - { pkg: "net/http/httptest$", desc: "framework/testkit" }
            - { pkg: "net/http/httputil$", desc: "framework/http" }
            - { pkg: "net/http/httptrace$", desc: "framework/telemetry" }
            - { pkg: "net/http/cookiejar$", desc: "framework/httpclient" }
            - { pkg: "net/http/pprof$", desc: "framework/telemetry" }
            - { pkg: "net/http/cgi$", desc: "not needed" }
            - { pkg: "net/http/fcgi$", desc: "not needed" }
            - { pkg: "net/smtp$", desc: "framework/mail" }
            - { pkg: "net/mail$", desc: "framework/mail (ParseAddress)" }
            - { pkg: "net/textproto$", desc: "framework/mail, framework/mailbox" }
            - { pkg: "net/rpc$", desc: "not needed" }
            - { pkg: "net/rpc/jsonrpc$", desc: "not needed" }
            - { pkg: "mime$", desc: "framework/filesystem (content type), framework/mail" }
            - { pkg: "mime/multipart$", desc: "framework/mail, framework/http" }
            - { pkg: "mime/quotedprintable$", desc: "framework/mail" }
            - { pkg: "html/template$", desc: "framework/view (templ)" }
            - { pkg: "text/template$", desc: "framework/view; saved replies use app/support/templating" }
            - { pkg: "path/filepath$", desc: "framework/filesystem" }
            - { pkg: "math/rand$", desc: "framework/support/str.Random" }
            - { pkg: "math/rand/v2$", desc: "framework/support/str.Random" }
            - { pkg: "crypto$", desc: "framework/hashing, framework/encryption" }
            - { pkg: "crypto/aes$", desc: "framework/encryption" }
            - { pkg: "crypto/cipher$", desc: "framework/encryption" }
            - { pkg: "crypto/des$", desc: "never" }
            - { pkg: "crypto/dsa$", desc: "never" }
            - { pkg: "crypto/ecdh$", desc: "framework/notifications/webpush" }
            - { pkg: "crypto/ecdsa$", desc: "framework/jwt" }
            - { pkg: "crypto/ed25519$", desc: "framework/jwt" }
            - { pkg: "crypto/elliptic$", desc: "framework/jwt" }
            - { pkg: "crypto/hkdf$", desc: "framework/encryption" }
            - { pkg: "crypto/hmac$", desc: "framework/hashing.HMAC" }
            - { pkg: "crypto/md5$", desc: "framework/hashing" }
            - { pkg: "crypto/mlkem$", desc: "never" }
            - { pkg: "crypto/pbkdf2$", desc: "framework/hashing" }
            - { pkg: "crypto/rand$", desc: "framework/support/str.Random, framework/auth/token" }
            - { pkg: "crypto/rc4$", desc: "never" }
            - { pkg: "crypto/rsa$", desc: "framework/jwt" }
            - { pkg: "crypto/sha1$", desc: "framework/hashing" }
            - { pkg: "crypto/sha256$", desc: "framework/hashing.SHA256" }
            - { pkg: "crypto/sha3$", desc: "framework/hashing" }
            - { pkg: "crypto/sha512$", desc: "framework/hashing" }
            - { pkg: "crypto/subtle$", desc: "framework/hashing.Equal" }
            - { pkg: "crypto/tls$", desc: "framework/httpclient, framework/mail" }
            - { pkg: "crypto/x509$", desc: "framework/jwt, framework/aws" }
            - { pkg: "crypto/x509/pkix$", desc: "framework" }
        app-runtime:
          list-mode: lax
          files:
            - "**/app/**/*.go"
            - "**/routes/**/*.go"
            - "**/config/**/*.go"
            - "**/bootstrap/**/*.go"
            - "!$test"
          deny:
            - { pkg: "github.com/dimmihq/dimmi/framework/testkit", desc: "tests only" }
            - { pkg: "github.com/dimmihq/dimmi/tests", desc: "tests only" }
            - { pkg: "github.com/dimmihq/dimmi/database/factories", desc: "tests and seeders only" }
        app-direction:
          list-mode: lax
          files:
            - "**/app/**/*.go"
          deny:
            - { pkg: "github.com/dimmihq/dimmi/routes", desc: "routes import app, never reverse" }
            - { pkg: "github.com/dimmihq/dimmi/bootstrap", desc: "bootstrap wires app" }
            - { pkg: "github.com/dimmihq/dimmi/cmd", desc: "entry point only" }
            - { pkg: "github.com/dimmihq/dimmi/database/seeders", desc: "seeders call app, never reverse" }
        config-direction:
          list-mode: lax
          files:
            - "**/config/**/*.go"
          deny:
            - { pkg: "github.com/dimmihq/dimmi/app", desc: "config is data; framework types only" }
            - { pkg: "github.com/dimmihq/dimmi/routes", desc: "" }
        cmd:
          list-mode: strict
          files:
            - "**/cmd/**/*.go"
          allow:
            - "os$"
            - github.com/dimmihq/dimmi/bootstrap

        # ---------------- adapter zone: one SDK per dir ----------------
        adapter-llm-openai:
          list-mode: strict
          files:
            - "**/app/drivers/llm/openai/**"
            - "**/app/drivers/llm/openaicompat/**"
            - "**/app/drivers/llm/openrouter/**"
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/openai/openai-go]
          deny: *app_std_deny
        adapter-llm-anthropic:
          list-mode: strict
          files: ["**/app/drivers/llm/anthropic/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/anthropics/anthropic-sdk-go]
          deny: *app_std_deny
        adapter-llm-bedrock:
          list-mode: strict
          files: ["**/app/drivers/llm/bedrock/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/aws/aws-sdk-go-v2, github.com/aws/smithy-go]
          deny: *app_std_deny
        adapter-llm-gemini:
          list-mode: strict
          files: ["**/app/drivers/llm/gemini/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, google.golang.org/genai]
          deny: *app_std_deny
        adapter-channel-slack:
          list-mode: strict
          files: ["**/app/drivers/channel/slack/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/slack-go/slack]
          deny: *app_std_deny
        adapter-channel-discord:
          list-mode: strict
          files: ["**/app/drivers/channel/discord/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/disgoorg/disgo]
          deny: *app_std_deny
        adapter-channel-telegram:
          list-mode: strict
          files: ["**/app/drivers/channel/telegram/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/go-telegram/bot]
          deny: *app_std_deny
        adapter-github:
          list-mode: strict
          files: ["**/app/services/github/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/google/go-github/v93]   # 08 owner (shared GitHub layer), 13 adds files
          deny: *app_std_deny
        adapter-altcha:
          list-mode: strict
          files: ["**/app/services/altcha/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/altcha-org/altcha-lib-go]
          deny: *app_std_deny
        adapter-stripe:
          list-mode: strict
          files: ["**/app/services/stripe/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/stripe/stripe-go/v87]
          deny: *app_std_deny
        adapter-ses:
          list-mode: strict
          files: ["**/app/services/ses/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/aws/aws-sdk-go-v2, github.com/aws/smithy-go]
          deny: *app_std_deny
        adapter-default:   # every other driver/service dir: no SDK (whatsapp, api, fake channels; turnstile, hibp-free; use framework/httpclient)
          list-mode: strict
          files:
            - "**/app/drivers/**"
            - "**/app/services/**"
            - "!**/app/drivers/llm/openai/**"
            - "!**/app/drivers/llm/openaicompat/**"
            - "!**/app/drivers/llm/openrouter/**"
            - "!**/app/drivers/llm/anthropic/**"
            - "!**/app/drivers/llm/bedrock/**"
            - "!**/app/drivers/llm/gemini/**"
            - "!**/app/drivers/channel/slack/**"
            - "!**/app/drivers/channel/discord/**"
            - "!**/app/drivers/channel/telegram/**"
            - "!**/app/services/github/**"
            - "!**/app/services/altcha/**"
            - "!**/app/services/stripe/**"
            - "!**/app/services/ses/**"
          allow: [$gostd, github.com/dimmihq/dimmi]
          deny: *app_std_deny
        # ---------------- narrow app/support exceptions (07 agent actions): one module per dir ----------------
        support-openapiimport:
          list-mode: strict
          files: ["**/app/support/openapiimport/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/pb33f/libopenapi]
          deny: *app_std_deny
        support-actionschema:
          list-mode: strict
          files: ["**/app/support/actionschema/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/google/jsonschema-go]
          deny: *app_std_deny
        support-actiontemplate:
          list-mode: strict
          files: ["**/app/support/actiontemplate/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/yosida95/uritemplate/v3]
          deny: *app_std_deny
        support-actionresult:
          list-mode: strict
          files: ["**/app/support/actionresult/**"]
          allow: [$gostd, github.com/dimmihq/dimmi, github.com/tidwall/gjson]
          deny: *app_std_deny
        arch-tests:
          list-mode: strict
          files: ["**/tests/arch/**"]
          allow: [$gostd, github.com/dimmihq/dimmi]   # go/build, go/parser via $gostd

    forbidigo:
      analyze-types: true
      forbid:
        - pattern: '^time\.(Now|Since|Until|Sleep|After|AfterFunc|Tick|NewTimer|NewTicker)$'
          pkg: '^time$'
          msg: '[clock] use framework/clock (fakeable)'
        - pattern: '^http\.(DefaultClient|DefaultTransport|Get|Head|Post|PostForm|Client|Transport|NewRequest|NewRequestWithContext)$'
          pkg: '^net/http$'
          msg: '[httpclient] use framework/httpclient (SSRF guard, fakes)'
        - pattern: '^http\.(ListenAndServe|ListenAndServeTLS|Serve|ServeTLS|Server)$'
          pkg: '^net/http$'
          msg: '[httpclient] servers started by framework/foundation only'
        - pattern: '^context\.(Background|TODO)$'
          pkg: '^context$'
          msg: '[ctx] pass ctx from caller; tests use t.Context()'
        - pattern: '^fmt\.Print(f|ln)?$'
          pkg: '^fmt$'
          msg: '[print] use framework/log or console output'
        - pattern: '^os\.(Getenv|LookupEnv|Environ|Setenv|Exit)$'
          pkg: '^os$'
          msg: '[ctx] config via framework/config; exit via foundation'
        - pattern: '^panic$'
          msg: '[panic] return an error; Must* helpers live in framework/ only'
        - pattern: '^(print|println)$'
          msg: '[print] use framework/log or console output'

    importas:
      no-unaliased: true
      alias:
        - pkg: github.com/dimmihq/dimmi/app/events
          alias: appevents
        - pkg: github.com/dimmihq/dimmi/app/mail
          alias: appmail
        - pkg: github.com/dimmihq/dimmi/app/notifications
          alias: appnotifications
        - pkg: github.com/dimmihq/dimmi/app/broadcasting
          alias: appbroadcasting
        - pkg: github.com/dimmihq/dimmi/app/http/middleware
          alias: appmiddleware
        - pkg: github.com/dimmihq/dimmi/app/console/commands
          alias: commands
```

- `gochecknoinits`: zero `init()` anywhere (drivers registered via `Manager.Extend`, not `init`). Supersedes README "no init() except driver registration".
- Adding a vetted lib: PR edits `go.mod` + framework allowlist + this file's catalog row. Adding an adapter SDK: new `adapter-*` rule naming one dir and one module.

### 7.2 Arch tests (`tests/arch/`, Pest-style)

Built (framework rules; app rules join when `app/` exists):
```go
// tests/arch/architecture_test.go
m := arch.Load(t, "github.com/dimmihq/dimmi")
m.Expect("framework/...").NotToImport("app/...", "routes/...", "config/...", "bootstrap/...", "database/...", "cmd/...", "tests/...")
m.Expect("framework/...").Except("framework/testkit/...", "framework/.../drivertest/...").IgnoringTests().NotToImport("framework/testkit/...")

// tests/arch/framework_words_test.go: identifiers, strings, comments, file names
arch.Load(t, module).Expect("framework/...").NotToContainWords(businessWords()...) // stems of 2.4
```
Planned with `app/`:
```go
m.Expect("app/...").Except("app/drivers/...", "app/services/...", supportExceptions...).ToOnlyImport(stdAllowed..., "framework/...", "app/...", "config")
m.Expect("app/...", "routes/...").NotToImport("framework/*/drivers/...")          // drivers via Manager only
m.Expect("app/actions/...").NotToImport("framework/http/...", "framework/routing") // actions transport-free
// adapter dirs: one third-party module each (7.1), e.g. app/drivers/llm/anthropic -> anthropic-sdk-go
```

Binary size test (`tests/arch/binary_test.go`, PO 2026-11-02): builds `tests/arch/testdata/sizeapp` (everything foundation boots, run through `foundation.Run`) for linux/amd64 with `-ldflags=-dumpdep` and fails when any dependency line carries `<ReflectMethod>`: one function reachable from main that looks methods up by runtime name makes the linker keep every exported method of every type (app binary 63.9 MB → 36.0 MB stripped, 12.3 MB gzipped once removed). New dependency or code tripping it: remove it, or keep it out of the default build behind an opt-in package (as `telemetry/otlp`) or the `dev` build tag (as `make:*` stubs on `text/template`).

### 7.3 Adapter zone rules

- One vendor SDK per directory, named in 7.1. Vendor types never in exported signatures.
- HTTP through `a.HTTP.StdClient()`; SDK retries off (retry policy lives in app, e.g. `app/support/llmretry`).
- Current adapter dirs (build-vs-reuse.md): `app/drivers/llm/{openai,openaicompat,openrouter}` (openai-go v3), `anthropic` (anthropic-sdk-go), `bedrock` (aws-sdk-go-v2 bedrockruntime), `gemini` (google.golang.org/genai); `app/drivers/channel/slack` (slack-go incl. socketmode), `discord` (disgo: gateway, sharding, REST limits, Ed25519; never `disgo/voice`), `telegram` (go-telegram/bot), `whatsapp` (none: own Graph client on `httpclient`); `app/services/stripe` (stripe-go v87), `app/services/ses` (sesv2 identities/tenants for 04), `app/services/github` (go-github v93; 08 owns shared GitHub layer, 13 adds releases/compare/pulls files), `app/services/altcha` (altcha-lib-go, 01 PoW), `app/services/turnstile` (none), `app/services/hibp` removed (now `validation` rule).
- Narrow `app/support` exceptions (07 agent actions; 7.1 `support-*` rules): `openapiimport` (pb33f/libopenapi [verify version]), `actionschema` (google/jsonschema-go), `actiontemplate` (yosida95/uritemplate/v3), `actionresult` (tidwall/gjson). Pure parsing/validation libs with no I/O; vendor types never leave the package. New exception = PR naming dir + module + why no framework wrapper fits.
- Adapter SDKs get HTTP via `a.HTTP.StdClient()` (SSRF guard) and websocket dials via `httpclient/safedial` where SDK allows custom dialer (disgo, slack-go socketmode) [verify per SDK].

### 7.4 sqlc overrides (keep `app/models` on framework types)

```yaml
# sqlc.yaml (excerpt)
version: "2"
sql:
  - engine: postgresql
    queries: database/queries
    schema: [framework/session, framework/auth/token, framework/oauth, framework/customdomain/storage, framework/foundation, database/migrations]   # framework *.sql (FrameworkMigrations) + app set; River + notification tables come from migrate steps
    gen:
      go:
        package: models
        out: app/models
        sql_package: pgx/v5
        emit_pointers_for_null_types: true
        emit_json_tags: true
        overrides:
          - db_type: uuid
            go_type: { import: github.com/dimmihq/dimmi/framework/ids, type: UUID }
          - db_type: uuid
            nullable: true
            go_type: { import: github.com/dimmihq/dimmi/framework/ids, type: UUID, pointer: true }
          - db_type: timestamptz
            go_type: time.Time
          - db_type: timestamptz
            nullable: true
            go_type: { type: time.Time, pointer: true }
          - db_type: jsonb
            go_type: { import: encoding/json, type: RawMessage }
          - db_type: vector
            go_type: { import: github.com/dimmihq/dimmi/framework/database, type: Vector }
          - db_type: inet
            go_type: { import: net/netip, type: Addr }
          - column: "*.*_encrypted"
            go_type: { import: github.com/dimmihq/dimmi/framework/encryption, type: Sealed }
```
CI: grep generated `app/models/*.go` struct fields for `pgtype.` → fail (override missing).

## 8. Stability rules

- Framework exported API = contract with app and future external users. CI runs `apidiff` (golang.org/x/exp/cmd/apidiff) per framework package, merge-base vs HEAD.
  - Incompatible change: PR must add `framework/rfcs/NNNN-<slug>.md` (problem, API before/after, migration steps, affected app call sites) and label `framework-breaking`; same PR updates all app call sites.
  - Compatible addition: one line in `framework/CHANGELOG.md`.
- Coverage: each framework package ≥ 90% statements, total ≥ 95% target; CI gate per package (`make cover`: `-coverpkg=./framework/...` with `.env.testing`, + script). `s3` driver covered against SeaweedFS and `smtp` against Mailpit from root `compose.yaml`; other cloud drivers (ses, kms, guardduty, provider mail APIs) via `httpclienttest` (AWS SDK clients use `a.HTTP`); no third-party network.
- Every driver passes its `drivertest` suite; fakes too, where a fake exists.
- Framework tests never import app; framework fixtures use neutral names (`posts`, `widgets` banned → use `items`, `notes`).
- No business words (2.4). No product defaults (ports, names, cookie names, env prefix) in framework.
- Deprecation: `Deprecated:` paragraph in the docblock + keep one minor release cycle before removal (inside monorepo: removal only after zero app call sites).
- Split readiness: `framework/` has no `go:embed` reaching outside its tree, no relative paths to repo root, own `README.md`, own test fixtures. Split = `git subtree split` + module path rewrite.

## 9. Directory layout (framework tree; full repo tree in README "Directory layout")

```
framework/               (built 2026-11-02; README.md, CHANGELOG.md, rfcs/ planned)
├── foundation/          application.go boot.go boot_services.go boot_web.go drivers.go config.go load.go options.go
│                        environment.go runtime.go appkey.go maintenance.go serve.go console.go commands.go
│                        commands_migrate.go dev.go (//go:build dev) nodev.go migrations.go jwks.go zaplog.go
│                        20261102150000_create_idempotency_keys_table.sql
├── config/              load.go env.go defaults.go (own `default` tags) lists.go section.go redact.go
├── manager/  fault/  tenancy/  clock/  ids/  process/  redis/  pubsub/  websocket/  resilience/
├── http/                kernel.go aliases.go context.go errors.go request.go resource.go paths.go lang/en/
│   ├── middleware/      requestid recover trustproxies cors csrf throttle signed idempotent maintenance
│   │                    bodylimit timeout securityheaders version locale respond
│   ├── idempotency/  pagination/  rest/  origin/  useragent/
├── routing/             router.go route.go api.go url.go signed.go lint.go
├── auth/                guard.go bearer.go gate.go policy.go principal.go abilities.go confirm.go
│   ├── token/           (+ personal_access_tokens migration, Postgres store)
│   └── twofactor/       totp.go recovery.go factor.go challenge.go
├── session/             manager.go middleware.go session.go store.go postgres.go (+ sessions migration)
├── socialite/           manager.go fake.go internal/oauth/ drivers/{github,google,oidc}/ drivertest/
├── oauth/               authorize code token refresh revoke register metadata resolver store ... (+ oauth_* migration)
├── jwt/  webhooks/
├── hashing/             hasher.go manager.go digest.go drivers/{argon2id,bcrypt}/ drivertest/
├── encryption/          appkey.go encrypter.go kek.go keys.go ops.go header.go sealed.go manager.go
│                        drivers/awskms/ drivertest/ (local KEK built in)
├── database/            db.go config.go tx.go errors.go lock.go reader.go replica.go copy.go notify.go vector.go
│   └── migrate/         runner.go session.go status.go sqlfile.go lint.go
├── events/  queue/ (queue.go worker.go tx.go options.go periodic.go drain.go migration.go)  schedule/
├── broadcasting/        server.go connect.go channels.go broadcast.go unisse.go (no fake, no drivertest: real Valkey)
├── cache/               store.go remember.go lock.go flush.go (Valkey only: no drivers, no fake)
├── ratelimit/           limiter.go local.go headers.go (no drivers, no fake)
├── filesystem/          filesystem.go disk.go manager.go drivers/{local,s3}/ drivertest/ (no fake)
├── malware/             malware.go manager.go fake.go drivers/{none,clamav,guardduty}/ drivertest/
├── mail/                mail.go mailable.go manager.go queued.go validate.go fake.go mime/ dkim/
│                        drivers/{smtp,ses,postmark,resend,mailgun,sendgrid,log}/ drivertest/ internal/provider/
├── mailbox/             mailbox.go manager.go account.go parse/ authcheck/ dsn/ arf/ accountauth/ imap/ drivers/{ses,postmark,mailgun,sendgrid,resend,cloudflare,gmail,graph}/ drivertest/ internal/{report,webhook}/
├── notifications/       notifications.go manager.go queued.go channels/{mail,database,broadcast,webpush}/
├── log/  health/
├── telemetry/           telemetry.go trace.go metrics.go report.go otlp/ (opt-in exporter)
├── httpclient/          client.go request.go response.go retry.go sse.go httpclienttest/ safedial/ robots/ drivertest/
├── dns/                 dns.go system.go fake.go drivertest/
├── aws/                 config.go sns/
├── validation/          validation.go walk.go rules.go errors.go lang/en/ (own `validate` tag engine)
├── i18n/                translator.go messages.go plural.go context.go format.go keys.go detector.go lang/en/
│                        drivers/{none,whatlang}/ drivertest/
├── sanitize/  markdown/  dom/
├── view/                view.go vite/ static/ feed/
├── image/               image.go reencode.go orient.go canvas.go
├── customdomain/        normalize.go verify.go probe.go backoff.go tls.go drivers/{certmagic,caddyask,none}/
│                        storage/ (certmagic Storage on Postgres + advisory-lock Locker, migration) drivertest/
├── console/             console.go command.go output.go make/ stubs/ (make + stubs linked only with -tags dev)
├── mcp/                 server.go tool.go resource.go confirm.go http.go stdio.go client.go
├── support/str/  support/diff/
└── testkit/             env.go postgres.go template.go valkey.go s3.go client.go response.go table.go factory/ arch/
```

Framework-owned tables (PO 2026-11-02), all migrated by foundation's runner from one framework set with one version table `framework_migrations` (`FrameworkMigrations()`), after the library steps and before the app set (`app_migrations`):
- `sessions` (`framework/session`), `personal_access_tokens` (`framework/auth/token`): generic columns only. App migrations that need product columns ALTER these tables (00-accounts), never CREATE them.
- `oauth_clients`, `oauth_authorization_codes`, `oauth_grants`, `oauth_tokens` (`framework/oauth`).
- `idempotency_keys` (`foundation`, `tenant_id`), `certmagic_storage` (`customdomain/storage`, non-tenant; key, value, modified; no `lock_expires_at`: locks are Postgres advisory locks).
- Steps (forward only, re-run safe): River schema (`queue.Migration()`, rivermigrate, own version table), one `notifications` table step per configured database notification channel (default table `notifications`, `CREATE TABLE IF NOT EXISTS`).
Columns generic (`tenant_id`, not `workspace_id`); RLS policy uses configured `database.tenant_setting`. No cache, rate-limit, pubsub or realtime tables: Valkey (data-model.md "Valkey"). Encryption keysets live in the app's `KeysetStore` table (`encryption_keys`, data-model.md).

## 10. Migration of spec references

Record of reference migration. 00-foundation and 00-conventions applied (2026-11-02 sweep); feature folders applied by owners. "Stays" = no path change, only listed refinement.

### 10.1 Paths and packages

| Current reference (where) | New | Note |
|---|---|---|
| `app/contracts/` holds cross-cutting interfaces (00-conventions, 00-foundation/contracts.md line 3) | cross-cutting → framework package; `app/contracts/` domain only | see 2.3 |
| `contracts.Filesystem` (00-conventions stack table) | `filesystem.Filesystem` | |
| `contracts.Mailer`, `MailMessage`, `MailSendResult`, `Address`, `MailAttachment` (contracts.md, 04 README 36/315, 00-accounts, 06, 10, 12) | `mail.Mailer`, `mail.Message`, `mail.SendResult`, `mail.Address`, `mail.Attachment` | errors keep names: `mail.ErrPermanent`, `mail.ErrTemporary`, `*mail.RateLimitedError` |
| `contracts.Cache` (00-conventions) | `cache.Store` | |
| `contracts.RateLimiter` (00-accounts README 408, flows.md 125, 05 tasks 19, 05 README 303) | `ratelimit.Limiter` (`a.Limiter`) | |
| `contracts.PubSub` (events-and-realtime.md 118) | `pubsub.Bus` (`a.PubSub`, Valkey) | |
| Pusher-protocol server, laravel-echo + pusher-js (00-conventions stack table, `README.md` root, 00-foundation README decision 4, events-and-realtime.md, api.md, rest.md, tasks.md Realtime, spec-writer-brief.md, 01 README/api/sdk-contract/tasks, 03 README/tasks, 07 edge-cases, 09 cli.md, 11 README/edge-cases/operations) | `framework/broadcasting` on centrifugal/centrifuge (Redis broker on Valkey); clients centrifuge-js (dashboard, CLI `dimmi listen` via centrifuge-go), centrifuge-swift/java/dart (mobile), plain `EventSource` (widget) | PO directive: wrap library, no own protocol server. Foundation applied |
| `/broadcasting/auth` endpoint (api.md, rest.md, events-and-realtime.md, tasks.md, 01 README/api/sdk-contract/tasks) | centrifuge connect auth (`OnConnecting`: session/bearer guard or 60 s widget connect token from `POST /widget/v1/realtime-tokens`) + subscribe auth (`OnSubscribe` → `routes/channels.go`) | foundation applied; 01 owns token endpoint |
| "Resume by last event id" (events-and-realtime.md, tasks.md) | centrifuge history + recovery in Valkey (`recover: true`, offset + epoch) | `realtime_events` table dropped |
| Presence gossip with TTL (tasks.md, 03 presence channel) | centrifuge Redis presence manager on Valkey | |
| Pusher client events (typing, 03 README/api/tasks, events-and-realtime.md) | REST `typing-indicators` for all clients; client publish off on every namespace | |
| Pusher channel prefixes `private-` / `presence-` (03 `presence-conversation.{id}`, 06 `private-member.{id}`, 13-15 `private-workspace.{ws}`, events-and-realtime.md) | namespaces `workspace:ws_...`, `conversation:conv_...`, `member:mem_...`, `widget_contact:ctc_...` (presence option on workspace + conversation) | naming rule in events-and-realtime.md "Channels" |
| KeyManager contract (00-conventions, foundation tasks) | `encryption.KeyManager` | |
| `contracts.InboundMail`, `InboundMailDriver`, `InboundEnvelope` (04 pipeline.md 32, 04 tasks 12, 04 README 316/321) | `mailbox.Source`, `mailbox.Envelope` | contract suite → `framework/mailbox/drivertest` |
| `contracts.RateLimitedError`, `contracts.ErrPermanent`, `ErrRecipientUnreachable` (04 outbound, 04 README) | stays `app/contracts` (Channel send errors) | domain |
| `contracts.Channel`, `Knowledge`, `LLM`, `Embeddings`, `Actor`, `BillingStatusReader` | stays `app/contracts` | domain |
| `uuid.UUID`, `"github.com/google/uuid"` in domain contract code (03 channel-contract.md 20, contracts.md Knowledge/Actor) | `ids.UUID`, import `github.com/dimmihq/dimmi/framework/ids` | module path `dimmi/app/...` in 03 snippet → `github.com/dimmihq/dimmi/app/...` |
| `app/managers` generic (00-conventions 76, 252) | framework managers on `*App`; `app/managers` keeps `channel_manager.go`, `llm_manager.go`, `embeddings_manager.go` on `framework/manager` | |
| `app/drivers/filesystem/{local,s3}`, `app/drivers/mail/{smtp,ses,log}` (00-conventions layout) | `framework/filesystem/drivers/*`, `framework/mail/drivers/*` | |
| `app/drivers/inboundmail/*` (04 README code map) | `framework/mailbox/drivers/{ses,postmark,mailgun,sendgrid,resend,cloudflare,gmail,graph}` + `framework/mailbox/imap`; `testdriver` dropped: tests feed SNS fixture bodies to the real `ses` receiver (no mailbox fake); no SMTP receiver | |
| `app/drivers/llm/*`, `app/drivers/channel/*` (07, 05, 03) | stays (adapter zone) | 7.3 |
| `app/drivers/llm/internal/sse` (07 README, llm-contract) | removed: official provider SDKs (openai-go/v3, anthropic-sdk-go, genai, Bedrock) stream natively in `app/drivers/llm/*` adapter zone | n/a |
| `support/ids` (00-conventions 221, foundation tasks 18, 03/04/07/08/00-accounts tasks, 07 README 89, 04 README 102, 03 README 616, 00-accounts README 638) | `framework/ids` registry; prefix catalog `app/models/public_ids.go` | "register in `support/ids`" → "add to `app/models/public_ids.go`" |
| `support/clock` (00-conventions 296, foundation README 54, 02 README 87, 03 edge-cases 230) | `framework/clock` (`a.Clock`; tests `clock.NewFake`) | "Postgres-synced" `postgres` source not built yet |
| `support/safehttp` (00-conventions 297, security.md 43, 05 README 704, 07 llm-contract 178, 09 oauth 183, 09 README 927/950) | `framework/httpclient` (`a.HTTP`); config key `safehttp.allow_private_networks` → `http_client.allow_private_networks` | |
| `support/safedial` | `framework/httpclient/safedial` | |
| `support/signedurl` (foundation tasks 39, 00-accounts flows 36) | `framework/routing` (`a.URL.SignedRoute`, middleware `signed`) | |
| `app/support/crypt`, `Encrypted[T]` column type (00-conventions 35, 229) | `framework/encryption`; column type `encryption.Sealed` + `encryption.Seal/Open[T]` | sqlc override on `*_encrypted` |
| `support/sanitize` (04 pipeline 285, 04 README 350) | `framework/sanitize`; email policy built in app (`app/integrations/email/htmlbody.go`) with `sanitize.NewPolicy()` | |
| `support/markdown` (06 delivery 18, 08 tasks 17) | `framework/markdown`; help center dialect = `markdown.Options` set in `app/support/helpmarkdown/options.go` | plain-text extractor → `markdown.PlainText` |
| goldmark AST walk `markdown_ast.go` (05 README 303) | `markdown.Parse` neutral AST | converters stay in 05 |
| chroma (08 rendering 13, tasks 17) | `markdown.Options.Highlight` | |
| bluemonday (08 rendering 23, foundation tasks) | `framework/sanitize` | |
| `app/support/auth/password_hasher.go` (00-accounts README 408, tasks 13) | `framework/hashing` (`a.Hash`) | semaphore + `auth_busy` in hashing config |
| `app/support/auth/tokens.go` (00-accounts README 408, tasks 14) | `framework/auth/token` | `dmi_live_`/`dmi_test_` prefixes stay app constants |
| `app/support/auth/totp.go`, `recovery_codes.go` (00-accounts, tasks 15) | `framework/auth/twofactor` | |
| `app/support/auth/guard.go` | `app/auth/guards.go` implementing `auth.Guard` | caller-class logic stays app |
| `app/support/auth/abilities.go` (00-accounts README 408, 09 README 857) | `app/auth/abilities.go` | catalog = product data |
| `app/support/auth/{lockout,device,disposable_domains,free_email_domains}.go` | `app/auth/` (lockout on `ratelimit`, device on `http/useragent`) | |
| `app/support/auth/common_passwords.go`, `app/services/hibp` (00-accounts) | planned `validation` password rule (not built: today an app `validation.Rule` via `a.Validator.Register`) | embedded list moves to framework when built |
| `support/auth.UsageRecorder` (00-accounts README 510) | `app/auth/usage_recorder.go` (flushed in provider `Shutdown`) | |
| `app/services/github/oauth_client.go`, `app/services/google/oidc_client.go` (00-accounts README 409) | `framework/socialite` drivers `github`, `google` | |
| go-oidc (00-accounts) | `framework/socialite` | |
| `app/support/oauth/{tokens,pkce,redirects,resources,metadata}.go` (09 README 288-293, tasks 9) | `framework/auth/token` + `framework/oauth` (full AS incl. storage + migration) | `resources` (RFC 8707 audiences) deferred |
| `app/support/oauth/{scopes,token_cache,usage_flusher,first_party,resolver}.go` (09 README 289/294/295/314/341, tasks 55) | scopes, first-party rules, consent screen: `app/auth/oauth/`; resolver: `oauth.Server.Resolver()`; usage flusher: deferred (last-use tracking) | product rules |
| CIMD fetch (09 oauth 183, README 927) | deferred in `framework/oauth`; when built: fetch on `httpclient` (6.5) | |
| `app/mcp/server.go` go-sdk (09 README 365, mcp.md 3, tasks 79) | `framework/mcp`; tools/resources/prompts stay `app/mcp` | |
| `support/jwtverify` (02 tasks 7, README 240/303) | `framework/jwt` (`jwt.Verify`); fuzz tests move with it | |
| golang-jwt in 02 snippets.md 203-218 | stays | customer-side example, not Dimmi code |
| `app/support/webhooksig` (10 tasks 7/67, README 221) | `framework/webhooks` | |
| `support/useragent` (03 README 127, tasks 55) | `framework/http/useragent` | |
| `support/origins` (01 tasks 10, README 288) | `framework/http/origin` | |
| `support/widgetassets` (01 tasks 24, README 286) | `framework/view/vite` + `framework/view/static`; `app/support/widgetassets/` keeps only `embed.FS` + widget config | |
| `support/pow` (01 tasks 95, README 287) | adapter zone `app/services/altcha` (altcha-lib-go server side); single-use tracking via `cache.Add` | build-vs-reuse 3.2 |
| `tests/unit/support/pow_test.go, origins_test.go, widgetassets_test.go` (01 README 305) | no `tests/unit`: pow behavior via feature tests; origins/widgetassets tests move into framework packages | testing.md |
| `support/langdetect`, lingua-go (07 README 280, pipeline 102, tasks 58) | dropped: 07 uses classifier `language`; optional `framework/i18n` detector `whatlang` | lingua breaks memory budget |
| `support/officehours`, `llmretry`, `aiprices`, `pii`, `prompts`, `linkguard` (07) | stays `app/support/*` | linkguard walks `markdown.Parse` AST |
| `support/metering` (12) | stays | |
| `app/services/stripe` (12 stripe.md 3, README 306) | stays (adapter zone, stripe-go) | HTTP via `a.HTTP.StdClient()` |
| `app/services/webpush` (06 tasks 54, delivery 33) | `framework/notifications/channels/webpush` (wraps SherClockHolmes/webpush-go v1.4: RFC 8291 + VAPID) | 06 own impl superseded |
| `app/services/slack` (06; discord later: `discord-notifications`), 05 own Slack/Discord/Telegram clients | 05 drivers in adapter zone with slack-go, disgo, go-telegram/bot; 06 incoming-webhook posts stay plain `httpclient` (one POST) | |
| `support/searchquery`, `querybuilder`, `templating`, `excerpt`, `mentions`, `spam` (03) | stays `app/support/*` | excerpt uses `markdown.PlainText` |
| `support/textdiff`, `support/crawl`, `support/review` (08) | stays `app/support/*` | crawl uses `framework/dom` + `httpclient` |
| readability + html-to-markdown (08 knowledge 139, README 493) | `framework/dom` (`dom.Readable`, `dom.ToMarkdown`) | |
| `golang.org/x/net/html`, `html/charset` (04 pipeline 168/286, 08 edge-cases 112) | `framework/dom` (`dom.Parse`, `dom.DecodeCharset`) | |
| enmime `parser.go` (04 README) | `framework/mailbox/parse` | size/memory limits in `parse.Limits` |
| go-mail `composer.go` (04 README) | `framework/mail/mime` builder | header policy stays 04 |
| `dkim/signer.go` (04 README) | `framework/mail/dkim` | |
| `auth.go` SPF/DKIM/DMARC (04 README) | `framework/mailbox/authcheck` | alignment policy decisions stay 04 |
| `dsn.go`, `arf.go` (04 README) | `framework/mailbox/dsn`, `framework/mailbox/arf` | |
| `sns/verify.go` (04 README) | `framework/aws/sns` | |
| `ses/identities.go` (04 README) | `app/services/ses/identities.go` (adapter, sesv2) | |
| `dns/checker.go` (04 README), mail DNS checks in doctor | uses `framework/dns` (`a.DNS`) | |
| Discord gateway / Slack Socket Mode websockets (05) | disgo gateway + sharding, slack-go `socketmode` (adapter zone) | `framework/websocket` only if SDK lacks it |
| `support/migrator` (11 README 217, tasks 28) | `framework/database/migrate` | `dimmi_releases` table stays 11 (app) |
| `support/memlimit` (11 README 218, tasks 13) | `framework/foundation` runtime (`runtime.memory_limit`) | |
| `support/trustedproxy` (11 README 219) | `framework/http/middleware` `TrustProxies` | |
| `support/doctor` (11 README 213, tasks 68) | `framework/health` registry + generic checks; product checks stay `app/support/doctor/` | |
| `support/settings` registry (11 README 97/214, tasks 17) | stays app; backed by `config.RuntimeStore` | |
| `support/backupfmt` (11 README 216, tasks 60) | stays app; stream crypto via `encryption.NewStreamWriter` | pg_dump via `a.Process` |
| `<data_dir>/framework/down` (11 operations 233) | stays (file name), owned by `foundation` maintenance | |
| `support/nodes/heartbeat.go`, `support/identitystats`, `support/widgetinstall` | stays `app/support/*` | |
| River UI `/admin/jobs` (foundation tasks 42) | not included (deferred) | riverui would add a large dependency |
| scs (tech-stack) | dropped; `framework/session` + app store | |
| zitadel/oidc (09 README 970) | unchanged decision (own OAuth); protocol pieces in `framework/oauth` | |
| sentry-go (11 README 64) | `framework/telemetry` `sentry` (opt-in) | |
| `models.Tx` (00-conventions 232) | `*database.Tx` | |
| `events.Dispatch(ctx, tx, evt)` (00-conventions 236, contracts.md) | `a.Events.Dispatch(ctx, tx, evt)` | |
| `tx.AfterRollback(func())` (contracts.md) | `(*database.Tx).AfterRollback` | |
| `mail.Fake()`, `events.Fake()`, `queue.Fake()`, storage fake, `http.Fake()`, `clock.Freeze()` (00-conventions 285) | `a.Mail.Fake()`, `httpclienttest.New(a.HTTP)`, `a.DNS.Fake()`, `a.Scanner.Fake()`, `a.Social.Fake()`, `clock.NewFake(t)`; events, queue, storage, broadcast fakes dropped: real services + `a.Queue.Drain(ctx)` | PO 2026-11-02: fakes only at process boundaries, testing env only |
| `storage.Disk("s3")` (00-conventions 31) | `a.Storage.Disk("attachments")` | named disks |
| `tests/contracts/<contract>_test.go` for framework contracts | `framework/<pkg>/drivertest`; domain suites stay `tests/contracts/` | |
| `tests/testcase.go` helpers (00-conventions) | `tests.TestCase` embeds `*testkit.TestCase` | |
| `app/console/commands/stubs/` (00-conventions 301) | `framework/console/stubs/` + app overrides in `stubs/` | |
| `app/exceptions` domain errors | stays; built with `fault.New(kind, code, msg)` | renderer in `framework/http` |
| infra tables `idempotency_keys`, `realtime_events`, `cache`, `rate_limits` (data-model ownership map 14, foundation tasks) | `idempotency_keys` owner `framework` (`tenant_id`); `realtime_events`, `cache`, `rate_limits` dropped (Valkey); `sessions`, `personal_access_tokens`, `oauth_*`, `notifications` now framework-owned (9) | data-model.md applied |
| `app.workspace_id` RLS GUC (00-conventions 208/223) | value unchanged; set by `framework/database` from `tenancy` with `database.tenant_setting: app.workspace_id` | |

### 10.2 00-foundation/tasks.md items to retarget (applied 2026-11-02 sweep)

- Skeleton: add `framework/` packages + `.golangci.yml` from 7.1 + `tests/arch`.
- Database: `support/ids` → `framework/ids`; test harness → `framework/testkit`.
- HTTP: signed URLs → `framework/routing`; pagination/idempotency/bulk/field-dispatch → `framework/http/{pagination,rest}`; CORS/CSRF → `framework/http/middleware`; route lint engine → `framework/routing` (rules config in app).
- Async: dispatcher → `framework/events`; River → `framework/queue`; scheduler → `framework/schedule`; workspace initializer registry stays app.
- Realtime: Pusher protocol server tasks replaced by `framework/broadcasting` on centrifuge (node, transports, auth callbacks, uni-SSE handler, Redis broker + presence on Valkey); compatibility tests centrifuge-js Node script + `EventSource` script; load test unchanged (10k idle sockets, RSS).
- Contracts and drivers: all items → framework packages with `drivertest`; add `malware` (none/clamav/guardduty), `dns`, `process`.
