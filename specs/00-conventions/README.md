# Code structure and conventions

Date: 2026-11-01. Style: caveman. Scope trim 2026-11-02: inbound mail SES only (IMAP, provider inbound drivers later), Discord notifications later. Binding for all code. Language: Go 1.26 (see `research/tech-stack.md`). Structure and style: Laravel.

Rule: if Laravel has convention for it, copy it. Deviate only where Go forces it. Every deviation listed in "Deviations" below.

Framework/app split (Illuminate vs app): [framework.md](framework.md). App code never imports third-party libs; `framework/` wraps each one (reuse before build).

## Why Laravel structure in Go

- Every contributor and every coding agent knows Laravel layout. Zero guessing where code lives.
- Laravel conventions proven for exactly this kind of product (CRUD + jobs + events + mail + notifications + broadcasting).
- Go keeps memory low and ships one binary. Laravel keeps codebase boring and predictable.

## Stack (fixed)

| Concern | Laravel piece | Dimmi Go piece | Framework package (wraps) |
|---|---|---|---|
| Entry point | `artisan` | `cmd/dimmi/main.go`. `dimmi <command>` | `framework/console` (cobra), `framework/foundation` |
| HTTP routing | `routes/*.php`, Router | named routes, `routes/*.go` | `framework/routing`, `framework/http` (chi) |
| OpenAPI / validation | FormRequest | OpenAPI 3.1 from Go types + request structs in `app/http/requests` | `framework/http` (huma v2), `framework/validation` (own rule engine, `validate:"..."` tags; go-playground/validator removed: binary size, PO 2026-11-02) |
| ORM | Eloquent | sqlc. Generated structs and queries in `app/models`. Hand-written behavior in same package | `framework/database` (pgx v5) |
| Migrations | `database/migrations` | plain SQL, embedded | `framework/database/migrate` (goose; database/sql via pgx stdlib adapter, accepted) |
| Queue | Laravel Queue (database driver) | River (Postgres) | `framework/queue` (River) |
| Scheduler | `routes/console.php` schedule | declared in `routes/console.go` | `framework/schedule` (River periodic) |
| Events | Event, Listener, `ShouldQueue` | in-process dispatcher, queued listeners become River jobs (transactional) | `framework/events` |
| Broadcasting | Broadcasting + Reverb | realtime embedded in binary (centrifuge): namespaced channels, connect/subscribe auth callbacks, presence, history/recovery, WebSocket + uni-SSE (widget). Broker, presence, history in Valkey (centrifuge Redis broker). Clients: centrifuge-js (dashboard), official centrifuge mobile SDKs, plain `EventSource` (widget). `00-foundation/events-and-realtime.md` | `framework/broadcasting` (centrifugal/centrifuge) |
| Auth (SPA) | Sanctum session | session cookie + cross-origin protection (Go `http.CrossOriginProtection`, no CSRF token), `sessions` table | `framework/auth`, `framework/session` |
| Auth (API) | Sanctum personal access tokens with abilities | `personal_access_tokens`, hashed, abilities | `framework/auth`, `framework/auth/token` |
| OAuth server | Passport | own OAuth 2.1 (authorization code + PKCE, device flow) for MCP and CLI, `oauth_*` tables | `framework/oauth` (protocol), storage in app |
| Social login | Socialite | GitHub, Google, enterprise OIDC | `framework/socialite` (x/oauth2, go-oidc) |
| Mail | Mailables | `app/mail` (system stream: users/members only), templ templates. One instance-level provider from deploy config, named mailers `system` / `customer`; contact mail only via 04 actions | `framework/mail` (go-mail; drivers smtp relay, ses, postmark, resend, mailgun, sendgrid, log) |
| Inbound mail | (laravel-mailbox) | email channel (04): SES receipt only at MVP (IMAP later: `imap-adapter`; provider inbound webhooks later: `extra-mail-adapters`). No SMTP receiver | `framework/mailbox` (enmime, go-msgauth) |
| Notifications | Notifications with channels | `app/notifications`, channels: mail, push, slack, database (discord later: `discord-notifications`) | `framework/notifications` |
| Storage | Filesystem disks (Flysystem) | named disks, `app.Storage.Disk("attachments")` | `framework/filesystem` (aws-sdk-go-v2 s3 + stdlib `os.Root`; gocloud.dev removed: binary size) |
| Cache | Cache store (redis driver) | Valkey only (no driver manager, no fake): shared cache, every write with TTL, `Add` (`SET NX PX`) for replay/dedupe, `Remember` with cross-process stampede protection, auto-extending locks (non-critical; correctness locks = Postgres advisory locks) | `framework/cache` (rueidis, rueidisaside, rueidislock) |
| Rate limiting | `RateLimiter` (redis) | Valkey counters, Laravel headers | `framework/ratelimit` (httprate + rueidis counter) |
| Valkey (required) | Redis | every deployment runs Valkey (self-host compose, cloud ElastiCache for Valkey). Holds only rebuildable state: realtime, cache, limits, internal pubsub, dedupe windows, counters. Config `redis.url` (`DIMMI_REDIS_URL`) | one client: `github.com/redis/rueidis` |
| Config | `config/*.php` + `.env` | `config/*.go` typed structs: defaults, `dimmi.yaml`, env `DIMMI_*` | `framework/config` (koanf + own `default` tag applier; creasty/defaults removed) |
| Views | Blade, Vite | templ (`resources/views`) | `framework/view` (templ) |
| Hashing | `Hash` | argon2id | `framework/hashing` |
| Encryption | `Crypt`, `encrypted` cast | envelope encryption, `encryption.Sealed` column type | `framework/encryption` (Tink, KMS) |
| Outbound HTTP | `Http` client | SSRF-guarded client | `framework/httpclient` (resty, code.dny.dev/ssrf) |
| Logging | Monolog | JSON logs | `framework/log` (slog) |
| Telemetry | (Pulse/Telescope adjacent) | Prometheus metrics on admin port, optional Sentry; OTLP tracing opt in: `foundation.Options{Telemetry: []telemetry.Option{otlp.Tracing()}}` (links gRPC, so outside default build) | `framework/telemetry`, `framework/telemetry/otlp` (OpenTelemetry) |
| Localization | `__()`, `lang/` | `lang/<locale>/*.json`, locale middleware, formatting helpers (section "Localization") | `framework/i18n` (golang.org/x/text + own loader; go-i18n removed) |
| Tests | PHPUnit/Pest, feature + unit, factories, RefreshDatabase, `arch()` | `go test`, `tests/feature` (integration, all business logic), `tests/arch`, `tests/contracts`, `database/factories`, real Postgres + real Valkey + S3 (SeaweedFS) + Mailpit from root `compose.yaml`, found through the app's normal env vars (testing.md). Unit tests only for pure framework/driver logic, in package `_test.go` files | `framework/testkit` |

## Directory layout

Lowercase directory names (Go package rule; only deviation from Laravel layout). Everything else mirrors Laravel 12. `framework/` = Illuminate (details, rules, package catalog: [framework.md](framework.md)).

```
dimmi/                             # module github.com/dimmihq/dimmi
├── cmd/dimmi/main.go              # "artisan". Calls bootstrap, nothing else
├── framework/                     # Illuminate. Wraps every third-party lib. Never imports app/, routes/, database/, config/, bootstrap/
│   ├── foundation/                #   Application kernel, service provider lifecycle, roles, maintenance
│   ├── config/  manager/  fault/  tenancy/
│   ├── http/                      #   kernel, middleware, huma API, errors, pagination/, rest/, idempotency/, origin/, useragent/, lang/
│   ├── routing/                   #   named routes, URL + signed URLs, route:list, route:lint (dev-only command)
│   ├── auth/  session/  socialite/  oauth/  jwt/  hashing/  encryption/
│   ├── database/                  #   pool, Tx hooks, tenant SET LOCAL, readers, CopyFrom, Notify, Vector, migrate/ (goose runner; framework tables migrate from their own packages)
│   ├── events/  queue/  schedule/  pubsub/  broadcasting/  websocket/
│   ├── cache/  ratelimit/  filesystem/  malware/  mail/  mailbox/  notifications/  customdomain/  redis/  resilience/
│   ├── log/  telemetry/ (+ otlp/ opt in)  health/  httpclient/ (+ httpclienttest/)  dns/  aws/  process/  clock/  ids/  image/
│   ├── validation/  sanitize/  markdown/  dom/  i18n/  view/  console/  mcp/  webhooks/
│   ├── support/str/  support/diff/
│   └── testkit/                   #   DB/Valkey/S3 per test, template DB, HTTP client + assertions, table assertions, factory/, arch/
├── app/
│   ├── app.go                     # type App struct { *foundation.Application; Actions; domain services }
│   ├── actions/                   # business operations, one per file, grouped by domain
│   │   ├── conversations/         #   reply_to_conversation.go, assign_conversation.go ...
│   │   ├── contacts/
│   │   └── ...
│   ├── auth/                      # app guards (session, app, token, widget), abilities catalog, OAuth grant rules (implements framework/auth)
│   ├── broadcasting/              # channel authorization callbacks (Laravel routes/channels.php logic)
│   ├── client/                    # CLI client mode (`-tags cli` build): API client, login device flow, `dimmi init`
│   ├── console/commands/          # app commands (framework/console)
│   ├── contracts/                 # DOMAIN contracts only: Channel, Knowledge, LLM, Embeddings, Search, BillingStatusReader, Actor, ContactEraser, WorkspaceInitializer
│   ├── drivers/                   # adapter zone: domain contract drivers. One vendor SDK per dir max
│   │   ├── channel/{api,fake,slack,discord,telegram,whatsapp}/   # slack-go, disgo, go-telegram/bot (one SDK per dir)
│   │   └── llm/{openai,anthropic,bedrock,gemini,openrouter,openaicompat,fake}/   # official LLM SDKs (framework.md 7.3)
│   ├── enums/                     # typed string enums (ConversationStatus, Role ...)
│   ├── events/                    # domain event structs (past tense); import alias appevents
│   ├── exceptions/                # domain errors built with framework/fault
│   ├── http/
│   │   ├── controllers/
│   │   │   ├── api/v1/            # public REST API (token or session)
│   │   │   ├── web/               # dashboard shell, auth pages, help center SSR
│   │   │   ├── widget/            # Messenger widget public API
│   │   │   └── webhooks/          # inbound provider webhooks (ses, slack, discord, telegram, whatsapp, stripe)
│   │   ├── middleware/            # app middleware: workspace, verified, metered, role (alias appmiddleware)
│   │   ├── requests/              # request structs + Validate() + Authorize()
│   │   └── resources/             # JSON transformers (API Resources)
│   ├── integrations/              # conversation channel adapters: email, slack, discord, telegram, whatsapp, widget
│   ├── jobs/                      # job args + workers (framework/queue)
│   ├── listeners/                 # event listeners (InTx, AfterCommit, queued)
│   ├── mail/                      # mailables (alias appmail)
│   ├── managers/                  # domain managers only: ChannelManager, LLMManager, EmbeddingsManager (framework/manager)
│   ├── mcp/                       # MCP tools, resources, prompts (framework/mcp)
│   ├── models/                    # sqlc output (*.sql.go, models.go, db.go) + hand-written behavior + public_ids.go (prefix catalog)
│   ├── notifications/             # multi-channel notifications to workspace members (alias appnotifications)
│   ├── observers/                 # model observers (optional, prefer explicit events)
│   ├── policies/                  # authorization per model (framework/auth gate)
│   ├── providers/                 # service providers: app, auth, event, broadcast, route, console
│   ├── rules/                     # custom validation rules (framework/validation)
│   ├── services/                  # adapter zone: vendor API clients (stripe, ses identities, github, altcha, turnstile); one SDK per dir
│   └── support/                   # product helpers, pure Go, no third-party: searchquery, querybuilder, templating, excerpt,
│                                  # mentions, spam, officehours, metering, llmretry, pii, prompts, crawl, textdiff ...
├── bootstrap/app.go               # load config, build *App, register providers
├── config/                        # app.go, auth.go, broadcasting.go, cache.go, database.go, filesystems.go, logging.go,
│                                  # mail.go, queue.go, services.go, ai.go, channels.go, billing.go (embed framework <pkg>.Config)
├── database/
│   ├── migrations/                # app migrations: 20261101000001_create_workspaces_table.sql (framework tables in framework/database/migrate)
│   ├── queries/                   # sqlc query files, one per table: conversations.sql
│   ├── factories/                 # test factories (framework/testkit/factory)
│   └── seeders/                   # demo data, `dimmi db:seed`
├── lang/en/                       # translation files (JSON)
├── public/                        # built SPA + widget assets, embedded via go:embed (frontend later)
├── resources/
│   ├── views/                     # templ: emails, help center, auth pages (generated *_templ.go)
│   ├── data/                      # embedded data files (disposable email list ...)
│   ├── js/                        # React dashboard source (later)
│   └── widget/                    # Preact widget source (later)
├── routes/
│   ├── api.go                     # /api/v1
│   ├── web.go                     # dashboard, auth, help center
│   ├── widget.go                  # /widget/v1
│   ├── webhooks.go                # /webhooks/{provider}
│   ├── channels.go                # broadcast channel authorization
│   ├── console.go                 # scheduled jobs
│   └── mcp.go                     # MCP tool registration
├── storage/                       # local disk: app/, logs/ (gitignored)
├── stubs/                         # optional overrides of framework make:* stubs
├── tests/
│   ├── feature/                   # integration tests: every business behavior (testing.md)
│   ├── contracts/                 # domain contract suites (channel, knowledge, llm); framework suites live in framework/<pkg>/drivertest
│   ├── arch/                      # import + business-word architecture tests (framework/testkit/arch)
│   └── testcase.go                # tests.TestCase embeds *testkit.TestCase + *app.App
├── .golangci.yml                  # depguard/forbidigo/importas rules: framework.md section 7
├── sqlc.yaml                      # type overrides keep app/models on framework types
├── dimmi.example.yaml
├── .env.example
├── Dockerfile
├── docker-compose.yml
└── AGENTS.md                      # conventions summary for coding agents (copy of this file, short)
```

## Request lifecycle (always same shape)

```
route -> middleware -> controller -> request (validate) -> policy (authorize) -> action (business) -> resource (JSON)
```

- Controller: thin. Parse request, authorize, call one action, return resource. No SQL, no business rules.
- Request: struct with validation tags plus `Rules()` for complex rules. Failure returns Laravel 422 shape.
- Policy: `ConversationPolicy.Update(ctx, member, conversation) bool`. Controller calls `authorize(ctx, "update", conversation)`.
- Action: one exported type per file, method `Handle(ctx, input) (output, error)`. Owns transaction. Dispatches events. Reusable from controller, job, MCP tool, CLI command. Never imports `net/http`.
- Resource: `ConversationResource(model) map/struct`. Only way models leave app. Same resource used by REST, webhooks, realtime broadcast, MCP.

Example:

```go
// app/http/controllers/api/v1/conversation_reply_controller.go
func (c *ConversationReplyController) Store(ctx context.Context, req *requests.StoreReplyRequest) (*resources.MessageResponse, error) {
    conversation, err := c.app.Models.FindConversation(ctx, req.Workspace(), req.ConversationID)
    if err != nil {
        return nil, err
    }
    if err := c.app.Gate.Authorize(ctx, "reply", conversation); err != nil {
        return nil, err
    }
    message, err := c.app.Actions.ReplyToConversation.Handle(ctx, conversations.ReplyInput{
        Conversation: conversation,
        Author:       req.Actor(),
        Body:         req.Body.Body,
        Attachments:  req.Body.AttachmentIDs,
    })
    if err != nil {
        return nil, err
    }
    return resources.Message(message), nil
}
```

## Naming

| Thing | Name | File |
|---|---|---|
| Resource controller | `ConversationController` (Index, Store, Show, Update, Destroy) | `conversation_controller.go` |
| Non-CRUD endpoint | dedicated controller per sub-resource: `ConversationReplyController.Store`, `ConversationClosureController.Store` | `conversation_reply_controller.go` |
| Invokable controller | `ShowWidgetConfigController.Invoke` | `show_widget_config_controller.go` |
| Request | `StoreConversationRequest`, `UpdateContactRequest` | `store_conversation_request.go` |
| Resource | `ConversationResource` | `conversation_resource.go` |
| Action | verb + noun: `ReplyToConversation`, `AssignConversation`, `MergeContacts` | `reply_to_conversation.go` |
| Event | past tense: `ConversationAssigned`, `MessageCreated` | `conversation_assigned.go` |
| Listener | what it does: `BroadcastMessageCreated`, `QueueWebhookDeliveries`, `RecordBillableConversation` | |
| Job | imperative: `DeliverWebhook`, `ProcessInboundEmail`, `CrawlKnowledgeSource` | `deliver_webhook.go` |
| Policy | `ConversationPolicy` | `conversation_policy.go` |
| Mailable | `ConversationReplyMail`, `InvitationMail` | |
| Notification | `ConversationAssignedNotification` | |
| Server/admin command | `dimmi migrate:status`, `dimmi channels:add`, `dimmi key:generate` (Laravel colon style); dev-only `make:*` via `make gen ARGS="make:migration ..."` | `channels_add_command.go` |
| Client resource command (CLI mode, 09) | `dimmi conversations list`, `dimmi articles push` (space style, like `gh`) | `app/client/...` |
| Enum | `enums.ConversationStatus` values `ConversationStatusOpen` | |
| Route name | `conversations.index`, `conversations.replies.store` | |
| Table | plural snake_case: `conversations`, `contact_identities` | |
| Pivot | singular alphabetical: `conversation_tag` | |
| Foreign key | `conversation_id` | |
| Migration | `20261101000001_create_conversations_table.sql` (goose needs numeric version; Laravel words after) | |
| sqlc query | `-- name: FindConversation :one`, verbs Find, List, Create, Update, Delete, Count | `database/queries/conversations.sql` |
| JSON field | snake_case (Laravel default) | |
| Config key | `mail.default`, `filesystems.disks.s3.bucket`; env `DIMMI_MAIL_DEFAULT` | |
| Go | standard: exported PascalCase, `ctx context.Context` first, errors last | |

## Routes

- RESTful, plural nouns, kebab-case paths: `/api/v1/conversations/{conversation}/replies`.
- REST only. Paths are nouns; state changes are `PATCH` on fields (`PATCH /conversations/{id}` `{"status": "closed"}` maps to `ConversationController.Update`, which calls action `CloseConversation`); things with identity are child collections (`POST /conversations/{id}/merges` maps to `ConversationMergeController.Store`). Full rules and route map: `specs/00-foundation/rest.md`. One controller per resource or sub-resource (Laravel "cruddy by design"), never action-named controllers.
- Field-dispatch in `Update`: request validates allowed fields, controller hands each changed state field to its action (`status` to `CloseConversation`/`ReopenConversation`/`SnoozeConversation`, `assignee_id` to `AssignConversation`). Plain attributes go to one `UpdateConversation` action. All in one transaction; one response.
- Every route named. `route("conversations.show", id)` builds URLs (emails, webhooks, resources `links`).
- Route name style (Laravel resource style): URL resource segments in kebab-case joined by dots, then action: `saved-replies.index`, `conversations.replies.store`, `email-domains.checks.store`. Same name is OpenAPI `operationId`, so CLI, MCP and SDK names derive from it. Never snake_case in route names.
- Route files group by audience: `api.go`, `web.go`, `widget.go`, `webhooks.go`. Middleware assigned per group.
- `dimmi route:list` prints method, path, name, middleware, controller.
- CI route lint (`route:lint`, dev-only command: `make gen ARGS="route:lint"`; rules passed in by app via `foundation.Options.RouteRules`): fails on `PUT` outside the binary-singleton allowlist, on any path segment in the verb denylist (`close`, `open`, `reopen`, `assign`, `approve`, `deny`, `accept`, `reject`, `send`, `resend`, `sync`, `test`, `verify`, `validate`, `publish`, `unpublish`, `archive`, `restore`, `login`, `logout`, `register`, `refresh`, `retry`, `reset`, `confirm`, `join`, `leave`, `boot`, `bulk`, `lookup`, `consume`, `pause`, `resume`, `complete`, `probe`, `move`, `reorder`, `block`, `merge`, `export`, `identify`, `check`, `rotate`, `retire`, `revoke`, `replay`, `reconnect`, `install`, `unsubscribe`, `ask`), for API prefixes (`/api/v1`, `/widget/v1`, help center `/api`). Routes outside those prefixes must be declared in `routes/web.go`, `routes/webhooks.go` or `routes/admin.go` and match a row of the exceptions table in `specs/00-foundation/rest.md`.

## Middleware (Laravel names)

Framework ships `throttle`, `signed`, `idempotent`, CSRF, CORS, trusted proxies, request id, maintenance (`framework/http/middleware`) and the guard/ability machinery (`framework/auth`). App supplies guard implementations (`app/auth`) and app middleware (`workspace`, `verified`, `metered`, `role`), registered as aliases in `app/providers/route_service_provider.go`.

| Name | Does |
|---|---|
| `auth:session` | browser session cookie only, cross-origin protection on unsafe methods (`http.CrossOriginProtection`). Browser-only routes |
| `auth:app` | first-party app bearer (`dmi_oat_` from `first_party` system client, user-scoped grant). Never used alone on routes; part of `auth:user` and `auth` |
| `auth:user` | `auth:session` or `auth:app`. Account routes (`/user/*`, `/sessions`, memberships) |
| `auth:token` | PAT `dmi_live_` or third-party OAuth `dmi_oat_` (workspace-scoped). Abilities apply |
| `auth` | any of the above. Workspace routes. Guard matrix: `specs/00-foundation/api.md` "Who calls what" |
| `auth:widget` | widget visitor session token |
| `workspace` | resolve current workspace (token's workspace, or `Dimmi-Workspace` header for session and first-party app), set tenant in context; `framework/database` applies `SET LOCAL app.workspace_id` at start of every transaction for RLS (tenant data only inside `db.Transaction`) |
| `ability:conversations:write` | token ability check (Sanctum `abilities`); no-op for session and first-party app (role policies apply) |
| `throttle:api` | rate limit by token/user/IP (Valkey), Laravel headers |
| `throttle:public` | anonymous public surfaces (help center, Ask AI, public MCP, widget config and session start): per IP + per visitor |
| `verified` | email verified |
| `signed` | signed URL check (attachments, unsubscribe, magic links) |
| `idempotent` | `Idempotency-Key` replay on POST |
| `metered` | blocks new billable activity when spend cap hit (cloud only) |

## Authorization

- Policies call `authz.Can(principal, permission)` with permission strings `resource:action` (catalog `app/auth/permissions.go`). Check permissions, never role names. Role names appear only in `app/auth/roles.go` (role -> permission set) and role-change actions (owner transfer, last-owner checks).
- Each permission belongs to exactly one token ability group (`00-foundation/api.md`); OAuth scopes = abilities = permission groups (one vocabulary). Clients gate UI on `GET /me` `member.permissions`, never on `role`.
- Conversation visibility through one builder `conversations.VisibleScope(membership)` (lists, search, counts, exports, realtime authorizers).

## Database rules

- PostgreSQL 18. Required extensions: `vector` (pgvector), `pg_trgm`, `unaccent`, `citext`, `btree_gin`. Compose image `pgvector/pgvector:pg18` (official `postgres:18` lacks pgvector).
- Primary key: `id uuid primary key default uuidv7()` (Postgres 18 native). Time-ordered, safe to expose.
- Public IDs in API: type-prefixed. Full registry: `specs/00-foundation/data-model.md` "Public ID prefixes". New table, new prefix row, no collisions. Encoding: prefix + base32 (Crockford, lowercase) of UUID bytes. `framework/ids` registry encodes/decodes (TypeID format); prefix catalog in `app/models/public_ids.go`. Reject wrong prefix with 404.
- Pre-tenant lookups (login by email, inbound routing by `channel_connections.external_id`, widget app id) run through `SECURITY DEFINER` functions so RLS stays on for app role.
- Every tenant table has `workspace_id uuid not null references workspaces(id) on delete cascade`. Every index on tenant table leads with `workspace_id`. Row level security policy on every tenant table in cloud (`USING (workspace_id = ANY (dimmi_visible_workspaces()))`; org-scoped billing tables `dimmi_visible_billing_accounts()`), app role not superuser.
- Tenant FK lint: table with FK to tenant table must carry `workspace_id` unless migration annotated `-- lint:user-scoped`, `-- lint:org-scoped` or `-- lint:global` (`00-foundation/security.md`).
- Timestamps: `created_at timestamptz not null default now()`, `updated_at timestamptz not null default now()` (sqlc queries set `updated_at = now()`). Soft deletes (`deleted_at`) only where spec says.
- Exception: append-only billing tables (`usage_ledger` and billing records, 12) have no `updated_at`, no UPDATE/DELETE grants for app role, and no cascade from workspace purge (tax records outlive workspace). `audit_logs` append-only too (no `updated_at`), purge rules in 00-accounts.
- Enums as `text` + `CHECK` constraint. Never Postgres enum types (painful migrations). Two kinds (`00-foundation/extensibility.md` P1):
  - State machines, statuses, visibility: strict `CHECK in (...)`.
  - Registry enums (driver-backed: new provider, factor, channel, source type = new driver, zero migration): `text` + format CHECK `^[a-z][a-z0-9_]{1,39}$` (dotted names `^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`) + validation against Go registry in request rule and action. CI lint enforces.

Registry enum columns (canonical list; owner registers values):

| Table.column | Owner |
|---|---|
| user_auth_factors.type, sessions.auth_method, users.created_via, oauth_identities.provider, security_events.type (dotted), workspace_members.joined_via, audit_logs.actor_type, audit_logs.via_kind | 00-accounts |
| contact_identities.type (05 registers `whatsapp_bsuid`, 14 `portal_visitor`), channel_connections.type, encryption_keys.kek_provider | foundation |
| inbound_emails.source | 04 |
| notifications.trigger | 06 |
| ai_provider_keys.provider, ai_action_connections.auth_type | 07 |
| knowledge_sources.type, github_connections.kind | 08 |
| oauth_grants.revoked_reason, oauth_grants.confirmed_method | 09 |
| portal_sessions.grant | 14 |
| imports.source | 16 |
- Flexible data: `jsonb` named `metadata` or `custom_attributes`. Size limits enforced in app.
- Money: integer minor units (`amount_cents bigint`) + `currency text`.
- Secrets: `bytea` columns typed `encryption.Sealed` (envelope encryption, `encryption.Seal/Open[T]`). Column name suffix `_encrypted` (sqlc override maps type).
- Migrations: forward-only in production. Every release must run against previous schema (expand, migrate, contract). Never edit merged migration. Each migration has `-- +goose Down` for local dev.
- Queries: only in `database/queries/*.sql`. No string-built SQL except search filter builder in `app/support/querybuilder` (parameterized only).
- Transactions: `a.DB.Transaction(ctx, func(tx *database.Tx) error {...})` (`framework/database`; `models.New(tx)` for queries). Actions own transactions. Events dispatched inside tx are delivered after commit (Laravel `afterCommit`). `tx.AfterCommit(fn)`, `tx.AfterRollback(fn)` hooks.

## Events and listeners

- Action dispatches event inside transaction: `a.Events.Dispatch(ctx, tx, appevents.MessageCreated{...})` (`framework/events`; phases InTx, AfterCommit, Queued).
- Sync listeners run after commit (realtime broadcast, cache invalidation).
- Queued listeners (`ShouldQueue`) inserted as River jobs in same transaction (`InsertTx`). Never lost, never sent for rolled-back data.
- One event catalog (`specs/00-foundation/events-and-realtime.md`) feeds: realtime broadcasts, outbound webhooks, notifications, audit log, metering, AI agent triggers.
- Listener registration in `app/providers/event_service_provider.go`. Readable list, like Laravel `$listen`.

## Jobs

- River job: `type DeliverWebhookArgs struct{...}` with `Kind() string` = `"deliver_webhook"` and worker `DeliverWebhookWorker`.
- Queues: `default`, `notifications`, `mail`, `webhooks`, `ai`, `channels`, `ingest`, `billing`, `backups`, `maintenance`. Concurrency per queue in `config/queue.go`. `notifications` separate to hold 10-second delivery budget.
- Every job idempotent. Retries with backoff. Unique jobs where duplicates harmful (`UniqueOpts`).
- Job args carry IDs, never full models. Re-fetch inside worker. Check model still exists and state still valid.
- Long work (crawls, imports) chunked into many small jobs with progress row.

## Contracts and drivers (Laravel Manager pattern)

- Cross-cutting contracts (Filesystem, Mailer, Queue, PubSub, KeyManager, RateLimiter, Clock, Hasher, Encrypter, HTTP client, Logger, Tracer, Validator, Broadcaster, malware Scanner) live in their `framework/<pkg>` with `Manager`, drivers in `framework/<pkg>/drivers/<name>`, contract suite `framework/<pkg>/drivertest`, `Fake()` only at process boundaries (mail, httpclient via `httpclienttest`, dns, malware, socialite, clock). Cache is no contract: one concrete `cache.Store` on Valkey, real in tests. Driver matrix (dev / self-host / cloud defaults): [framework.md](framework.md) section 4.
- Domain contracts (Channel, ChannelRehomer, Knowledge, LLM, Embeddings, Search, BillingStatusReader, ContactEraser, WorkspaceInitializer, ImportSource; full index `00-foundation/contracts.md`) in `app/contracts`, drivers in `app/drivers/<contract>/<driver>`, managers in `app/managers` built on `framework/manager`, suites in `tests/contracts/<contract>_test.go`.
- Config picks named instance and driver: `mail.default = "system"` (`mail.mailers.system.driver = "ses"`), `filesystems.disks.attachments.driver = "s3"`.
- Built-in drivers registered by framework. Extra drivers: `a.<Service>.Extend("name", factory)` in `app/providers/app_service_provider.go`. No `init()` registration. Community drivers: PR into main repo (avoid adapter rot), must pass the contract suite.
- Local drivers default for self-host (local disk, SMTP relay or any provider API, local key). AWS drivers default for Dimmi cloud (s3, ses, kms); self-host may pick them too (S3-compatible disk, SES mail), no AWS deploy guide. Valkey on both (cache, limits, pubsub, realtime).

## Validation and errors (Laravel JSON shapes)

422:
```json
{
  "message": "The email field must be a valid email address. (and 1 more error)",
  "errors": {
    "email": ["The email field must be a valid email address."],
    "name": ["The name field is required."]
  }
}
```

Other errors:
```json
{ "message": "Cannot change status from closed to snoozed.", "code": "invalid_transition", "from": "closed", "to": "snoozed" }
```

- Status codes: 400 malformed, 401 unauthenticated, 403 forbidden, 404 not found (also for other workspace's IDs, never 403), 409 conflict, 410 gone, 413 too large, 422 validation, 423 locked (codes `spend_cap_reached`, `password_confirmation_required`, `email_sending_paused`; clients switch on `code`), 424 failed dependency (provider rejected during channel connect), 429 throttled, 500, 503 maintenance.
- `code` always snake_case, stable, documented. `message` human, translatable.
- Domain errors live in `app/exceptions` (`ErrInvalidTransition`, built with `fault.New(fault.Invalid, "invalid_transition", ...)` + `.With("from", ...)`). Handler maps to status + code. Never leak SQL or stack traces.

## Localization (i18n)

PO 2026-11-02: every language is first class; adding one must never touch code.

- Backend strings (emails, notifications, validation and API error `message` text, server-rendered pages) live in `lang/<locale>/<group>.json` (`{{.name}}` placeholders, CLDR plural forms via `one`/`other`... keys). Loader own on golang.org/x/text (language matching, plural rules); no go-i18n, no `text/template`. Framework ships English messages (validation, API errors, formats); app files (`foundation.Options.Language`) load over them. English is the source and fallback. Adding a language = adding a `lang/<locale>/` folder plus the locale to `app.locales`.
- Config: `app.locale` (default `en`, fallback), `app.locales` (served list, default `["en"]`).
- Locale per request (`middleware.Locale`): user or tenant preference (app hook `foundation.Options.LocalePreference`) > `Accept-Language` > `app.locale`. Sets the context locale, `Content-Language`, `Vary: Accept-Language`.
- Validation messages: Laravel-style English by default ("The email field is required."), translated per locale under `validation.<rule>`. API errors carry stable `code` + translated `message`.
- API contract: clients branch on stable `code` values and `params`, never on `message`. `message` is localized for display only.
- Queued mail and notifications carry the recipient's locale and render in it, never in the sender's.
- Dates, numbers and money formatted per locale: `i18n.FormatNumber(locale, v)`, `i18n.FormatMoney(locale, minor, code)` (golang.org/x/text), `Lang.FormatDate/FormatTime(locale, t, loc)` (layouts from `lang/<locale>/formats.json`). Times stay UTC in the API. Known limits (accepted): month names English only; money symbol always first (`$ 1,234.50`, also where locale puts it after).
- CI fails on keys missing from any shipped locale and on keys unused in code: `lang:check` (dev-only command, `make gen ARGS="lang:check"`).
- Frontend (dashboard SPA, widget, help center client code) keeps its own UI string bundles in ICU MessageFormat (`{name}`, the JS ecosystem standard), shares the same supported-locale list and negotiation rules, reads the user's `locale` from `/me` (widget: visitor's browser language), and renders API errors from `code` + `params`. Server and browser bundles stay separate on purpose: their placeholder syntaxes differ, and most strings belong to only one side.

## Testing

Full rules: [testing.md](testing.md). Summary:
- Tests run against real services from root `compose.yaml` (`make services`): PostgreSQL (`pgvector/pgvector:pg18`, port 15432, `max_connections=300`), Valkey (`valkey/valkey:8`, 16379), S3 (SeaweedFS, 18333), Mailpit (SMTP 11025, API 18025). Found through the app's normal env vars (`DATABASE_URL`, `REDIS_URL`, `AWS_*`, `MAIL_SMTP_URL`, `MAILPIT_API_URL`) from `.env.testing`; missing var fails loudly. No testcontainers, embedded-postgres, SQLite, sqlmock, miniredis, MinIO.
- Business logic (`app/`): 100% integration tests (real App, real Postgres, real Valkey, real events, real River, real commits), 95% coverage gate per package, 100% target. No unit tests with mocks; never "unit test per action". Tests named in plain words describing the behavior, never by edge-case ids; every edge case still gets a test.
- Framework layer and interfaces: unit tests for pure logic plus integration tests against real services (Postgres, Valkey, SeaweedFS S3, Mailpit); driver contract suites; 90% per package, 95% total.
- Database per test (`testkit.NewDatabase(t)`, own non-superuser owner role) or copy of template DB migrated once per schema version (`testkit.NewMigratedDatabase(t, testkit.Fingerprint(...), migrate)`); app tests: fresh tenant per test (RLS isolates).
- Fakes only at process boundaries: mail transport (`a.Mail.Fake()`), outbound HTTP (`httpclienttest.New(a.HTTP)`), DNS (`a.DNS.Fake()`), malware scanners (`a.Scanner.Fake()`), socialite providers (`a.Social.Fake()`), clock (`clock.NewFake`), LLM (domain). Every fake panics unless app runs in `testing` env (`AllowFakes`, set by foundation). No fakes for DB, cache, filesystem (tests use `local` disk), queue, events, broadcasting, pubsub, ratelimit, encryption, hashing, process, log, telemetry, notifications, mcp: assert real effects.
- `tests/arch` binary test fails CI when any `<ReflectMethod>` marker shows in the linker dump (dead method elimination kept).
- `live` build tag for real third-party calls; never in CI; paid APIs need per-run approval.

## Code style

Full rules: [code-style.md](code-style.md) (lint pipeline and CI order, size limits, readable-code rules, comments, review checklist, AI agents).

- `make lint test` before done (`make lint` also runs with `--build-tags dev`; `make test` also runs foundation with `-tags dev`). golangci-lint v2, gofumpt + gci, zero warnings. `//nolint` only with linter name + reason.
- Hard limits: file 400 code lines, function 50 lines / 30 statements, cognitive complexity 15, 4 params, 120-col lines.
- Comments state business intent, never what. Every non-test file opens with a Laravel-style banner block; every package has `doc.go`; every exported identifier has a `/* */` docblock in full sentences. No `//` comments (directives aside), nothing inside function bodies, no references to specs, tickets or history.
- Import and identifier rules (allowed imports, clock, HTTP client, no `init()`): [framework.md](framework.md) section 7.

## Generators (artisan make)

Dev-only: compiled only with `dev` build tag, never in production binary. Run `make gen ARGS="make:action tags/CreateTag"` (= `go run -tags dev ./cmd/dimmi ...`).

`make:migration create_tags_table`, `make:controller api/v1/TagController`, `make:request StoreTagRequest --authorize --validate`, `make:action tags/CreateTag`, `make:event TagCreated`, `make:listener SendWelcomeMail --event UserRegistered --queued`, `make:job X`, `make:policy TagPolicy --model Tag`, `make:mail X`, `make:notification X`, `make:command PruneTags --signature tags:prune`, `make:test tags/TagCreation`. Every generator takes `--force` (overwrite). No `make:model` (sqlc generates models), no `make:resource`. Generator engine `framework/console/make` + default stubs `framework/console/stubs/`; app overrides in repo-root `stubs/` (`foundation.Options.Stubs`). Generated code passes lint. Other dev-only commands: `route:lint`, `lang:check`.

Production commands: `serve`, `migrate`, `migrate:status`, `migrate:plan`, `migrate:rollback`, `down`, `up`, `config:show`, `route:list` (+ app commands). In production `migrate` (shows database host/db + pending migrations), `down` (warns every user is turned away) and `migrate:rollback` (shows migration + database) warn and ask; non-interactive without `--force` refuses; rollback also needs exact version (`--confirm-version`). Details: code-style.md 2.1.

## Deviations from Laravel (complete list)

1. Lowercase directories (Go package names).
2. Migration filename uses goose numeric version (`20261101000001_`), not `2026_11_01_000001_`.
3. No Eloquent. sqlc queries + model behavior methods. No lazy loading; resources receive preloaded data.
4. No facades, no service container reflection. Explicit `*App` embedding `*foundation.Application`; process-boundary fakes swap in place on Managers (`a.Mail.Fake()`) or on the client transport (`httpclienttest.New(a.HTTP)`).
5. `app/mcp`, `app/integrations`, `app/drivers`, `app/managers`, `app/client`, `app/auth` extra directories.
6. UUIDv7 primary keys + prefixed public IDs (Laravel default is bigint).
7. Illuminate lives in same module as `framework/` (no Composer vendor dir), splittable later. Laravel app may import any package; Dimmi app zone imports only stdlib subset + `framework/*` (enforced by depguard + arch tests).
8. Adapter zone (`app/drivers/*`, `app/services/*`): may import exactly one named vendor SDK per directory (official LLM SDKs, slack-go, disgo, go-telegram/bot, stripe-go, SESv2, go-github, altcha-lib-go). Everything else in app is SDK-free. Narrow app exceptions, same one-module-per-dir rule (framework.md 7.1 `support-*` rules): `app/support/openapiimport` (pb33f/libopenapi), `app/support/actionschema` (google/jsonschema-go), `app/support/actiontemplate` (yosida95/uritemplate/v3), `app/support/actionresult` (tidwall/gjson) for 07 agent actions.
9. Colliding package names (`events`, `mail`, `notifications`, `broadcasting`, `http/middleware`) imported with `app` prefix alias (`appevents`), enforced by importas.
