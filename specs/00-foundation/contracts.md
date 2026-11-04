# Shared contracts (canonical)

Style: caveman. Scope trim 2026-11-02: reply_drafts eraser, `engage.link.public_daily_limit_unpaid` entitlement, X2 echo removed. Interfaces used by more than one spec. This file wins over any feature spec that disagrees.

Where code lives (`00-conventions/framework.md` 2.3):
- Cross-cutting contracts (any SaaS) = interfaces in their `framework/<pkg>` package. Listed below as pointers only; framework.md section 3.2 is the API.
- Domain contracts (Dimmi only) = `app/contracts/`, one interface per file, file name = snake_case of interface (`knowledge.go`, `entitlements.go`, `access_policy.go`, `retention_guard.go`, `audit_sink.go`, `import_source.go`). Fake per contract (`<pkg>.Fake()`), suite in `tests/contracts/<contract>_test.go`.
- Domain contract code uses framework types only: `ids.UUID` (`github.com/dimmihq/dimmi/framework/ids`), never `github.com/google/uuid`; `*database.Tx`; `mail.Message`.
- Fakes (framework, PO 2026-11-02): only at true process boundaries: `httpclienttest.New(a.HTTP)` (`framework/httpclient/httpclienttest`), `a.DNS.Fake()`, `a.Mail.Fake()` (mail transport), `a.Scanner.Fake()` (malware), `a.Social.Fake()` (socialite), `clock.NewFake(t)`. Every Fake panics unless app env is `testing` (`AllowFakes` set by foundation). No fakes for database, cache, filesystem, queue, events, broadcasting, pubsub, ratelimit, encryption, hashing, process, log, telemetry, notifications, mcp: tests use real services from root `compose.yaml` (testing.md).
- Framework helpers other specs rely on (framework.md 3.2 rows): `ids.NewV7At(t)` (UUIDv7 with given ms timestamp; 16 preserves source order), `(*database.Tx).CopyFrom(ctx, table, columns, rows) (int64, error)` (pgx COPY; COPY cannot load RLS tables for app role: 16 tenant tables use batched inserts or COPY into staging table), `db.Reader(ctx)` (read replica `Querier`, primary fallback), `db.Notify(ctx, channel, payload)` (outside tx only), `database.Vector`, `KeyManager.MAC(ctx, scope, data)` (Tink PRF/HMAC keyed per scope, never auto-rotated; 16 erasure tombstones), `mcp.Connect(ctx, url, mcp.ClientOptions{HTTP: a.HTTP, Headers})` client (07 MCP action connections), `customdomain` (08 TLS + verification), `image` (08, 13, 15), `support/diff` (08), `httpclient/robots` (08, 16), `view/feed` (13), `markdown.FrontMatter` (08).

## Framework contracts (pointers)

| Contract | Package | Used by | Note |
|---|---|---|---|
| `filesystem.Filesystem` | `framework/filesystem` | 01, 03, 04, 08, 11, 13, 15 | `Put(ctx, path, []byte, PutOptions{ContentType, CacheControl, ContentEncoding, Metadata map[string]string})`, `PutStream(ctx, path, io.Reader, PutOptions)`, `Get`, `Open`, `Exists`, `Stat` (`Attributes{Size, ContentType, CacheControl, ContentEncoding, LastModified, Metadata}`), `Delete` (missing = no error), `List(ctx, prefix)` (`iter.Seq2[Entry, error]`), `URL` (public base URL or `ErrNoPublicURL`), `TemporaryURL`, `TemporaryUploadURL(ctx, path, UploadURLOptions{TTL, ContentType})`. Drivers `local` (dev, tests, single node; sidecar attrs file, 15 `app` edge driver reads headers) and `s3` (staging, prod). No fake |
| `mail.Mailer` | `framework/mail` | 00-accounts, 04, 06, 10, 12 (via 04 for customer stream) | see "Mail" below |
| `mailbox.Source` | `framework/mailbox` | 04 | inbound sources |
| `*cache.Store` (concrete, no interface) | `framework/cache` | all | Valkey only, no fake. Every write needs TTL. `Add(ctx, key, value, ttl)` = `SET NX PX` for replay/dedupe (JWT `jti`/nonce, provider webhook replay, inbound dedupe windows): no TTL tables in Postgres. `Remember`/`Forget` (rueidisaside), `cache.For[T]`, `Lock`/`TryLock` (rueidislock, best effort) |
| `*ratelimit.Limiter` (concrete) | `framework/ratelimit` | all | Valkey (rueidislimiter): `Attempt`, `Hit`, `Remaining`, `AvailableIn`, `Clear`; `ratelimit.Local` in-process pacer (golang.org/x/time/rate). No fake |
| `*pubsub.Bus` (concrete) | `framework/pubsub` | cache bust, entitlements bust | internal bus on Valkey (`Publish`, `Subscribe`), not user-facing. No fake |
| `broadcasting.Broadcaster` (`a.Broadcast` = `*broadcasting.Server`) | `framework/broadcasting` | realtime listeners | centrifuge (events-and-realtime.md). No fake: tests subscribe a real client |
| `encryption.KeyManager`, `encryption.Encrypter`, `encryption.KEK`, `encryption.KeysetStore` | `framework/encryption` | all `*_encrypted` columns | "KeyManager" below = storage + scope rules. No fake |
| `*httpclient.Client` (concrete) | `framework/httpclient` | every outbound call | SSRF guard (security.md); tests `httpclienttest.New(a.HTTP).Stub(...).PreventStrayRequests()` |
| `mcp` client (`mcp.Connect`) | `framework/mcp` | 07 agent actions | remote MCP servers via `a.HTTP` (SSRF guard) |
| `customdomain.Verifier`, `customdomain.TLS` | `framework/customdomain` | 08 (13, 14 via 08 hosts) | storage `certmagic_storage` (framework migration set) |
| `clock.Clock`, `hashing.Hasher`, `log.Logger`, `validation.Rule`, `notifications.Channel`, `malware.Scanner`, `dns.Resolver`, `auth.Guard`, `session.Store`, `i18n.Detector`, `queue.Args` + `queue.Worker[A]` | framework | all | queue worker returns `queue.Postpone(d)`, `queue.Cancel(err)`, `queue.RetryAfter(d)`; no queue fake (assert job rows) |
| `validation` (`a.Validator.Struct(ctx, v)`, `validate:"..."` tags, `validation.Rule` via `Register`) | `framework/validation` | every request | own rule engine (go-playground removed); `*validation.Error{Violations}` → 422 `validation_failed`, field paths `emails.0`; Laravel-style English messages, translated per request locale |
| `i18n` (`i18n.T(ctx, key, Args)`, `a.Lang`, `FormatNumber`, `FormatMoney`, `FormatDate/Time`) | `framework/i18n` | every user-facing string | locale from request (preference hook > Accept-Language > `app.locale`); `lang/<locale>/*.json`; `lang:check` |
| `notifications.Notification` (`Type()`, channels), `Notifiable` (+ optional `Localized`) | `framework/notifications` | 06, all notifiers | `a.Notify.Send(ctx, n, to...)` / `SendTx(ctx, tx, n, to...)`; rendered in recipient locale; channels `mail`, `database`, `broadcast`, `webpush` |
| `events.Dispatcher` | `framework/events` | all | see "Event dispatch phases". No fake: tests assert real effects |

## Domain contracts index (`app/contracts/`)

| Interface | File | Owner | Consumers |
|---|---|---|---|
| `Channel` (+ errors `RateLimitedError`, `ErrPermanent`, `ErrRecipientUnreachable`) | `channel.go` | 03 | 01, 04, 05 |
| `Knowledge` | `knowledge.go` | 08 | 07, 13, 14 |
| `LLM`, `Embeddings` | `llm.go`, `embeddings.go` | 07 | 08, 13, 14, 15 |
| `Search` | `search.go` | 03 | 09 |
| `BillingStatusReader` | `billing_status_reader.go` | 12 | 01, 15 |
| `Actor`, `ActorVia` | `actor.go` | foundation | all |
| `ContactEraser` | `contact_eraser.go` | foundation (registry), 03 (runner) | every PII spec |
| `WorkspaceInitializer` | `workspace_initializer.go` | foundation | 01, 03, 04, 07, 08, 12, 13, 14 |
| `AccessPolicy` | `access_policy.go` | foundation (00-accounts rules) | `access.policy` middleware |
| `Entitlements` | `entitlements.go` | foundation (drivers 11, 12) | all capped specs |
| `RetentionGuard` | `retention_guard.go` | foundation | every purge/erase job |
| `AuditSink` | `audit_sink.go` | foundation | `ForwardAuditEntries` |
| `ImportSource` (+ normalized record types, `ImportCursor`, `RateBudget`, capability flags) | `import_source.go` | 16 | 16 drivers (registry enum `imports.source`) |
| `ChannelRehomer` (optional, `Rehome(ctx, tx, conv) (*models.ChannelConnection, error)`) | `channel_rehomer.go` | 03 | 16 `import` channel driver (replies on imported conversations move to inbox email connection) |
| `email.RegisterMailtoUnsubscribe(prefix, handler)` (boot registry, not interface; prefix `[a-z]{2,10}`, `unsub` reserved for 04) | `app/actions/email` (04 outbound.md 10) | 04 | 13 (prefix `cunsub` -> `UnsubscribeFromChangelog`, reason `mailto`) |

## Channel

Canonical definition: `specs/03-shared-inbox/channel-contract.md` (owner 03; Go package `github.com/dimmihq/dimmi/app/contracts`, ids as `ids.UUID`), including additions X1, X3-X5 requested by 05 (X2 echo later: `teammate-echo`). Inbound entry point for every channel: action `conversations.ReceiveInboundMessage`. Email (04), chat apps (05), widget (01) implement or call exactly that. No other names (`ReceiveChannelMessage` is wrong).

Outbound retry window per channel: 03 default about 3 hours. Driver may declare longer window in capabilities; email (04) uses about 24 hours (SMTP norms). `send_channel_message` job reads window from driver, not hardcoded.

## Knowledge (owner 08, consumer 07)

Implementation: `app/actions/knowledge` (08). Interface: `app/contracts/knowledge.go`. `*App.Knowledge` typed as `contracts.Knowledge`. Fake: `knowledge.Fake()`. Consumers: 07 (agent), 13 (changelog entries indexed as documents), 14 (`EmbedTexts` helper, 08 export).

```go
package contracts

type Knowledge interface {
    // Ranked chunks inside one workspace. No embeddings key or embed timeout: FTS fallback, never error.
    Search(ctx context.Context, workspaceID ids.UUID, query string, opts KnowledgeSearchOptions) (KnowledgeSearchResult, error)
    // Full normalized Markdown of one document, same visibility filter.
    Document(ctx context.Context, workspaceID, documentID ids.UUID, visibility []KnowledgeVisibility) (KnowledgeDocument, error)
    // Readiness for 07 (cached 10 s).
    Status(ctx context.Context, workspaceID ids.UUID) (KnowledgeStatus, error)
    // 07 refuses to delete provider key still used for embeddings.
    EmbeddingKeyInUse(ctx context.Context, workspaceID, providerKeyID ids.UUID) (bool, error)
    // Help center hosts for 07 outbound link allowlist.
    PublicHosts(ctx context.Context, workspaceID ids.UUID) ([]string, error)
}

type KnowledgeVisibility string // "public" | "agent_only"

type KnowledgeSearchOptions struct {
    Limit             int                   // default 8, max 50
    AllowedAudienceIDs []ids.UUID           // 18: filter effective_audience_ids <@ allowed; public surfaces and anonymous MCP pass empty
    Visibility        []KnowledgeVisibility // agent: [public, agent_only]; public surfaces: [public]
    Queries           []string              // up to 3 extra rewrites, fused with query by RRF (07)
    Locale            string                // preferred BCP 47, others ranked lower
    HelpCenterIDs     []ids.UUID
    SourceIDs         []ids.UUID
    ExcludeChunkIDs   []ids.UUID           // follow-up searches skip chunks already given
    ExcludeArticles   bool
    ExcludeSources    bool
    ExcludeChangelog  bool                  // 13: skip changelog documents
    Mode              string                // "auto" (default) | "keyword" | "hybrid"
    MaxPerDocument    int                   // default 3
    QueryEmbedTimeout time.Duration         // default knowledge.embed.query_timeout (1.5 s)
    Surface           string                // agent | help_center | widget | api | public_mcp | ask_ai | docs_sync (08: no logging, mode auto)
    LogSearch         bool                  // public surfaces true; agent false (07 logs in ai_run_steps)
}

type KnowledgeSearchResult struct {
    Chunks         []KnowledgeChunk
    Mode           string            // "hybrid" | "keyword" | "fts_fallback"
    Degraded       string            // "" | no_embeddings_key | embedding_timeout | embedding_error | vector_timeout | reembed_in_progress_old_key_missing | spend_cap
    EmbeddingModel string
    EmbeddingKeyID *ids.UUID        // ai_provider_keys row used for query embedding (cost attribution)
    EmbeddingUsage *LLMUsage         // nil when cached or FTS only; 07 adds to spend
    Timings        map[string]time.Duration
}

type KnowledgeChunk struct {
    ID           ids.UUID           // knowledge_chunks.id, public kc_
    DocumentID   ids.UUID
    ArticleID    *ids.UUID          // published article version only
    HelpCenterID *ids.UUID
    SourceID     *ids.UUID
    ChangelogEntryID *ids.UUID       // 13: published changelog entry
    SourceType   string              // article | website | upload | git | text | changelog (13)
    Title        string
    HeadingPath  []string
    URL          string              // canonical citation URL with anchor; "" when agent_only or not citable
    Text         string              // Markdown, max 2,000 chars, sanitized, untrusted data for prompts
    Similarity   *float64            // cosine 0..1 from vector arm; nil when FTS-only hit (07 threshold uses this)
    Score        float64             // fused RRF score, comparable within one result only
    Rank         int
    MatchedBy    []string            // fts | trigram | vector
    Visibility   KnowledgeVisibility
    Locale       string
    ContentHash  string              // sha256 of Text, replay drift detection
    UpdatedAt    time.Time
}

// KnowledgeDocument and KnowledgeStatus fields: defined in specs/08-help-center/knowledge.md section 1 (owner 08).
// KnowledgeDocument adds ChangelogEntryID *ids.UUID (13). Changelog chunks: title "Changelog: {title} ({date})", public visibility, kept by ExcludeSources.
```

Rules 07 relies on: only published/active content; `agent_only` chunks have empty URL; deterministic ordering (ties by chunk id); p95 under 100 ms excluding query embedding; query over 2,000 chars truncated; empty query returns empty slice; embedding uses workspace key via 07 `ai.ResolveProviderKey`; 08 records ingestion embedding spend through 07 action `RecordAiSpend` (kind `embedding`).

## Mail (owner foundation; `framework/mail`; used by 00-accounts, 04, 06, 10, 12, 13, 14, 15)

Sending model (product owner, binding): Dimmi sends mail only through one instance-level managed provider API or relay set in deploy config (`mail.*`). Never: outbound MTA, direct-to-MX, dedicated IPs / IP warming, own mail servers (incl. SMTP receiver), per-workspace provider credentials.

Two-stream rule (never mixed):
- **system** stream: mail to Dimmi users/members only (auth: magic link, verification, reset, invitations, security notices; 06 member notifications; 12 billing; system alerts). Sent as `app/mail` Mailables through default mailer `mail.mailers.system`.
- **customer** stream: every mail to a contact, visitor, subscriber, voter. Sent only by 04 actions (`Channel.Send` for conversations, `SendWidgetOfflineReply`, `SendContactMail` for 13/14/15/16; 04 outbound.md 15, 16) through `a.Mail.Mailer(channels.email.mailer)` (default name `customer`). Per-workspace sender identity (verified domain via instance provider), SES tenant per workspace (cloud), bounce/complaint auto-pause apply here only.
- Same provider account may back both; isolation via named mailer instances, SES configuration sets (`dimmi-system` / `dimmi-customer`), Postmark message streams, provider tags (`stream=`). Customer-stream pause never blocks system stream.
- Resolver: `mail.Manager.Mailer(name string) (Mailer, error)` returns named instance; customer name undefined in config resolves to default driver + credentials with stream overrides. Arch test: only `app/actions/email` resolves customer stream; Mailables route to member/user only.
- Config shape (one, both streams; framework.md "mail"): `mail.default` = default mailer **name** (`system`); `mail.mailers.<name>.driver` (`ses`, `smtp`, `postmark`, `resend`, `mailgun`, `sendgrid`, `log`) + that driver's keys flat under the mailer (`token`, `api_key`, `domain`, `region`, `configuration_set`, `message_stream`, `host`, `port`, `username`, `password`; `_FILE` for secrets). Never nested per driver (`mail.mailers.system.postmark.token` is wrong). 04 `channels.email.mailer` names customer mailer (default `customer`).
- System-stream suppression (06 `system_suppression_guard`): non-critical Mailables to a suppressed member address skipped + logged; critical user-initiated auth mail (magic link, email code, password reset, email verification) bypasses suppression, max 3 per address per 24 h (`notifications.system_mail.critical_bypass_per_day`; PO default keep, confirmed final pass). 06 owns system-stream bounce/complaint handling (04 routes provider events by `stream` tag).

Types live in `framework/mail` (framework.md 3.2), shown here because every spec depends on them:

```go
package mail

type Mailer interface {
    Send(ctx context.Context, msg Message) (SendResult, error)
}

type Message struct {
    From, ReplyTo      Address
    To, Cc, Bcc        []Address
    Subject            string
    Text, HTML         string
    Headers            map[string]string   // Message-ID, In-Reply-To, References, List-Unsubscribe, Auto-Submitted ...
    Attachments        []Attachment
    Raw                []byte              // optional pre-built RFC 5322 MIME; when set, other body fields ignored
    Tags               map[string]string   // provider tags (SES message tags) for event correlation
    ConfigurationSet   string              // SES only, ignored by others
    TenantName         string              // SES tenant (per-workspace reputation isolation, customer stream only), ignored by others
}

type SendResult struct {
    ProviderMessageID string
    MessageID         string // RFC Message-ID actually sent when provider rewrites ours (SES, others when known); "" = ours kept or unknown
}
```
Mailables: `mail.Mailable` (`Envelope()`, `Content(ctx)`), registered `mail.Register[M](a.Mail, name)`, sent with `a.Mail.To(addr).Locale(l).Send(ctx, m)` or `.Queue(ctx, m)` / `.QueueTx(ctx, tx, m)` on system stream only; rendered in recipient locale (`Locale` unset = `app.locale`). Tests: `a.Mail.Fake()` + `mail.AssertSent[M]`, `AssertMessage`, `AssertNothingSent` (fake transport; only process-boundary fake here). Contacts, visitors, subscribers, voters get mail only through 04 actions (`SendContactMail`, `SendWidgetOfflineReply`, `Channel.Send`).

Errors (`errors.Is`): `mail.ErrPermanent` (invalid address, suppressed, rejected content; no retry), `mail.ErrTemporary` (retry with backoff), `*mail.RateLimitedError{RetryAfter}`. Drivers (instance config only): `smtp` (relay/submission host only, never MX lookup; default self-host), `ses` (default AWS / cloud, shared IP pools), `postmark`, `resend`, `mailgun`, `sendgrid`, `log` (dev; may print auth links, see security.md exception; refused for customer stream in production). Drivers without raw-MIME endpoint (postmark, resend, sendgrid) decompose `Raw` into provider JSON. Open/click tracking always off.

Exceptions added 2026-11-04 (04): one system Mailable to a non-member, `EmailSetupInstructionsMail`, allowlisted in `email_streams_test.go`. Conversation mail on a connected Google or Microsoft mailbox goes through mailbox API, not customer stream.

## Event dispatch phases

```go
a.Events.Dispatch(ctx, tx, evt) // tx *database.Tx (framework/database)
events.Listen[E](a.Events, events.InTx|events.AfterCommit, fn)   // sync phases
events.ListenQueued[E](a.Events, func(E) queue.Args)            // River job inserted in same tx
```
Listener kinds (registered in `app/providers/event_service_provider.go`):
1. `InTx`: runs synchronously inside transaction, may return error to abort whole action (billing activation and cap gate in 12, audit log). Keep fast, no network.
2. `AfterCommit`: sync after commit (realtime broadcast, cache bust). Errors logged, never surfaced.
3. `Queued`: River job inserted in same tx (`InsertTx`).
No dispatcher fake: tests assert listener effects (rows, job rows, received broadcasts). `events.IsEphemeral(e)` marks events never recovered from history.
Plus `(*database.Tx).AfterRollback(func())` hook for cleanup of side effects prepared optimistically (temporary files).

## Actor

```go
type Actor struct {
    Type string       // member | contact | agent | system | integration
    ID   *ids.UUID
    Via  *ActorVia    // how request arrived: {Kind: session|app|personal_token|oauth_client|widget|channel|cli|mcp, ID}; app = first-party Dimmi app (ID = ogr_ grant)
}
```
`messages.author_type` keeps 5 values; `Via` stored in `messages.metadata.sent_via` and `audit_logs`. `Via.Kind` list is a registry (P1): `audit_logs.via_kind` and `audit_logs.actor_type` are registry enums and must accept every value here (incl. `app`; 00-accounts table). New kind = registry entry, no CHECK migration.

## Workspace initializer hook

`CreateWorkspace` (00-accounts) inserts `organizations` row, then workspace row, same tx, before any initializer (always new org at MVP). `app/providers` registers ordered initializers run inside `CreateWorkspace` transaction after `workspace.created` row insert: 03 default inbox, 01 default widget, 04 default forwarding address, 08 default help center (unpublished), 13 changelog for default help center (`EnsureChangelog`, after 08), 14 feedback defaults (settings, statuses, default board; after 08), 07 default agent (disabled), 12 billing account for workspace's organization if absent (cloud). 15 (engage) creates nothing at workspace creation (`engage_settings` row lazily). 06 creates notification settings lazily per member. Each initializer idempotent (safe re-run for repair command `dimmi workspace:repair`).

## Contact eraser hooks

GDPR erase (`DELETE /api/v1/contacts/{id}?erase=true`, 03 action `EraseContact`) calls every registered eraser inside erase job, in order, each idempotent:
- 03: messages bodies, attachments, identities, custom attributes, exports.
- 01: widget sessions, visitor metadata (`EraseWidgetContactData`).
- 04: inbound/outbound email rows, raw `.eml` files, suppression entries keyed by address, email thread refs.
- 05: channel message refs, channel contact states, provider user ids.
- 06: notification payload snippets mentioning contact.
- 07: ai_runs/ai_run_steps prompts and outputs for contact's conversations (keep token and cost numbers); `ai_action_calls` inputs/results for contact (same hook `EraseAiContactData`).
- 08: help_center_searches rows linked to visitor.
- 10: webhook delivery bodies containing contact (scrub stored payloads).
- 13: changelog subscribers (by `contact_id` and contact emails), reads, reactions (`EraseChangelogContactData`).
- 14: votes anonymized, posts/comments/requests scrubbed, subscriptions, notifications, portal sessions/sign-ins, contact state (`EraseFeedbackContactData`).
- 15: survey responses + answers, engage receipts, CSAT request rows (`EraseEngageContactData`).
- 16: `import_records` rows for contact and its conversations, `import_errors` mentioning its external id, tombstones (`encryption.MAC` of external ids + emails) in `import_erasures` so delta sync never re-imports (`EraseImportContactData`).
- 03 additions: drafts, reactions, translations, side conversations, `contact_tag`, notes. 01 hook also deletes `widget_devices` (20 registers none). 04 hook also deletes `email_mailbox_messages`, never touches customer's own mailbox. 18: `contact_tag` and `contact_event_summaries` by cascade. 21 none.
Message eraser (`contracts.MessageEraser`, `EraseMessage(ctx, workspaceID, messageID ids.UUID) error`): same registry shape, job retries until all nil. Registered by 04 (raw `.eml`, outbound rows), 07 (prompt copies), 10 (stored delivery bodies). Used by delete and redaction (03 safety.md).
Eraser interface (`contracts.ContactEraser`): `Erase(ctx, workspaceID, contactID ids.UUID) error`, registered in provider. Erase job fails loudly if any eraser errors; retried until all succeed. New feature storing contact PII must register eraser (review checklist).

## Auth interfaces owned by 00-accounts (pointers)

Defined in 00-accounts (`app/auth`), listed here so consumers use one name. Design: `00-foundation/extensibility.md` sections 1, 2, 4.
- `factors.Factor` (`app/auth/factors`): auth factor driver registry (`Register`, `Enabled`; capabilities `SecondFactor`, `Primary`, `Confirmation`, `PhishingResistant`, `Multiple`, `UserVisible`). MVP drivers `totp`, `recovery_code`. Rows in `user_auth_factors`.
- `providers.IdentityProvider` + `ProviderResolver` (`app/auth/providers`): login provider drivers (`github`, `google`; later SSO connections). Key = `oauth_identities.provider` registry value.
- `LoginPolicyResolver` (`ForEmail(ctx, email) (LoginPolicy, error)`): called by every primary auth method before secret verification. MVP driver allow-all. Later 409 `sso_required`.
- `authz.Can(principal, permission)`: permission check used by every policy. Catalog `app/auth/permissions.go` (strings `resource:action`, each in exactly one token ability group, api.md); role map `app/auth/roles.go` (only file naming roles for authorization). Principal for `authz.Can` = guard-resolved request principal (`Principal` below + workspace member + effective permissions), built once per request.

## AccessPolicy (owner foundation contract, 00-accounts implements rules)

Middleware `access.policy` (replaces `two_factor.enforced`, same position in `W` stack) evaluates workspace (later organization) policies against how caller authenticated (P3). Go: `app/contracts/access_policy.go`.

```go
type Principal struct {
    UserID          ids.UUID
    AuthMethods     []string   // every method passed: {password,totp}, {github}; later {passkey}, {sso}
    AuthenticatedAt time.Time  // sessions.authenticated_at or grant auth_time
    DeviceID        string     // sessions.device_id / oauth_grants.device_id
    IP              netip.Addr
    Via             ActorVia
    TwoFactorEnabled bool      // users.two_factor_confirmed_at set (cached with principal); MVP rule reads it, no extra query
    ConfirmedAt     *time.Time // last sudo: sessions.confirmed_at or oauth_grants.confirmed_at (09, app sudo via POST /user/confirmation)
    ConfirmedMethod string     // sessions.confirmed_method / oauth_grants.confirmed_method
}
type PolicyFailure struct {
    Code        string // two_factor_required | reauthentication_required | sso_required | ip_not_allowed
    Requirement string // two_factor | reauthentication | sso | ip
    ActionURL   string // setup or re-login URL
    Source      string // workspace | organization
}
type AccessPolicy interface {
    Evaluate(ctx context.Context, p Principal, workspaceID ids.UUID) (*PolicyFailure, error)
}
```
- MVP rule set: one rule, workspace setting `security.require_two_factor` against `Principal.TwoFactorEnabled`. Evaluated with cached principal (no extra query).
- `password.confirm` (sudo) reads `ConfirmedAt` (session or app grant) or `AuthenticatedAt` within `auth.password_confirmation.ttl` (api.md "Who calls what").
- Failure: 403 `{code, message, requirement, action_url}` (api.md "Errors"). Clients branch on `requirement`, not `code`.
- Applies to every caller on workspace routes (session, app, token: token principal = owning member's user; member without 2FA gets 403, same as today). Exempt routes unchanged (00-accounts flows "Require 2FA").
- Rules now in scope (21 access-policy.md): session max age, idle limit, dashboard and API IP allowlists, allowed sign-in methods, Google domains, SSO only. `Requirement` values: `two_factor`, `reauthentication_required`, `sign_in_method`, `sso_required`, `ip_not_allowed`; `action_url` may be null; extra fields `allowed_methods`, `allowed_domains`, `redirect_url`, `ip`, `reason`. `AccessRule.Check(p, settings, now)`; `Principal` adds `LastActivityAt`, `SSOConnectionID`, `AuthDomain`.
- `LoginPolicyResolver` driver: `OrganizationLoginPolicy`; `AllowAll` tests only. Effective permissions: `EffectivePermissions(catalog, role, customPermissions, abilities)` (00-accounts roles.md). `contracts.Actor.Via.Kind` adds `scim`.
- Later rules = new rule structs, no middleware change: phishing-resistant MFA, managed device.

## Entitlements (owner foundation contract; drivers 11 self-host, 12 cloud)

Pricing promise: one plan, every feature. Entitlements = limits + contract extras + rollouts, never paywalls. Go: `app/contracts/entitlements.go`; `*App.Entitlements`. Fake: `entitlements.Fake()`.

```go
type Entitlements interface {
    Has(ctx context.Context, workspaceID ids.UUID, feature string) (bool, error)
    Limit(ctx context.Context, workspaceID ids.UUID, key string) (int64, error) // -1 = unlimited
}
```
- Catalog `app/support/entitlements/catalog.go`: key, kind (`feature` | `limit`), default self-host, default cloud, owner spec. Unknown key panics at boot. Every cloud cap in every spec is a catalog key; call site reads `Entitlements.Limit`, never `if edition == cloud` branch or direct config. Rate limit defaults are keys too (`api.rate_limit_per_minute`, `api.user_rate_limit_per_minute`).
- Drivers: `self_host` (11: features all true, limits from config, no overrides); `cloud` (12: catalog cloud default, then `billing_accounts` program/contract, then `entitlement_overrides` by organization then workspace). Cached 60 s per workspace, busted on override change.
- Rollout flags: kind `feature`, cloud default false, enabled per org by override. No third-party flag service.
- API: `GET /api/v1/workspace/entitlements` (A W, any member; singleton, route name `workspace.entitlements.show`): `{object: "entitlements", features: {key: bool}, limits: {key: {limit, used}}}` (`used` where cheap). Controller in foundation over the contract; works on both editions. `used` counters registered by owning specs (`entitlements.RegisterUsed(key, fn)`).
- Over limit (one shared code, every key): count caps 422 `entitlement_limit_reached` `{code, message, key, limit, used}`; per-period caps (daily, hourly, monthly) 429 `entitlement_limit_reached` same body + `Retry-After` (period reset). Existing rows kept when limit drops; only creation refused. Feature-specific `*_limit_reached` codes only for fixed product limits that are not catalog keys (`tag_limit_reached`, `participant_limit_reached`, `factor_limit_reached`).

### Catalog (MVP keys)

Self-host column: value from owner config key (`0` = unlimited -> `-1`) unless stated. Owner spec defines enforcement point.

| Key | Kind | Self-host | Cloud default | Owner |
|---|---|---|---|---|
| `api.rate_limit_per_minute` | limit | 600 | 600 | foundation |
| `api.user_rate_limit_per_minute` | limit | 1200 | 1200 | foundation |
| `widget.max_per_workspace` | limit | config (10) | 10 | 01 |
| `email.verified_daily` | limit (daily) | -1 | 2000 | 04 |
| `email.unverified_daily` | limit (daily) | -1 | 200 | 04 |
| `email.new_workspace_hourly` | limit (hourly) | -1 | 50 | 04 |
| `email.unverified_relay_workspace_daily` | limit (daily) | -1 | 200 | 04 |
| `channels.discord.custom_bots` | limit | -1 | 3 | 05 |
| `notifications.email_daily_per_workspace` | limit (daily) | -1 | 5000 | 06 |
| `ai.workspace_concurrency` | limit | config (3) | 3 | 07 |
| `help_center.max_per_workspace` | limit | -1 | 3 | 08 |
| `help_center.max_articles` | limit (per help center) | -1 | 5000 | 08 |
| `knowledge.max_sources` | limit | -1 | 25 | 08 |
| `knowledge.max_chunks` | limit | -1 | 200000 | 08 |
| `knowledge.crawl.max_pages_cap` | limit | -1 | 5000 | 08 |
| `webhooks.http_endpoints` | limit | config `webhooks.max_endpoints` (20) | 20 | 10 |
| `billing.test_mode_cap` | limit (per account period) | -1 | 1000 | 12 |
| `billing.outbound_fair_use_min` | limit (per account period) | -1 | 1000 | 12 |
| `changelog.email.default_sender_max_recipients` | limit | -1 | 50 | 13 |
| `changelog.email.monthly_recipient_cap` | limit (monthly) | -1 | 10000 | 13 |
| `feedback.boards` | limit | -1 | 25 | 14 |
| `feedback.posts` | limit | -1 | 100000 | 14 |
| `feedback.statuses` | limit | 12 | 12 | 14 |
| `feedback.import_rows_per_month` | limit (monthly) | -1 | 100000 | 14 |
| `engage.max_live_announcements` | limit | config (50) | 50 | 15 |
| `engage.max_live_surveys` | limit | config (20) | 20 | 15 |
| `audiences.max_per_workspace` | limit | config (200) | 200 | 18 |
| `api.workspace_rate_limit_per_minute` | limit | 6000 | 6000 | foundation (00-accounts rate-limits.md) |
| `sso.connections_per_organization` | limit | 5 | 5 | 21 |
| `scim.tokens_per_organization` | limit | 5 | 5 | 21 |
| `roles.custom_per_workspace` | limit | 50 | 50 | 00-accounts |
| `email.connected_mailboxes` | limit | config (0 = -1) | 25 | 04 |
| `changelog.max_labels` | limit (per changelog) | 20 | 20 | 13 |
| `changelog.max_sources` | limit (per changelog) | 5 | 5 | 13 |
| `ai.actions.max_connections` | limit | config (0 = -1) | 20 | 07 |
| `ai.actions.max_published` | limit | config (0 = -1) | 50 | 07 |
| `ai.actions.calls_hourly` | limit (hourly) | config (0 = -1) | 2000 | 07 |
| `docs_sync.max_sources` | limit | config (0 = -1) | 10 | 08 |
| `docs_sync.runs_daily` | limit (daily) | -1 | 200 | 08 |
| `imports.concurrent_runs` | limit (active runs) | config (2) | 2 | 16 |
| `imports.records_monthly` | limit (monthly) | -1 | 1000000 | 16 |
| `imports.attachment_bytes_monthly` | limit (monthly) | -1 | 107374182400 (100 GB) | 16 |
| `imports.public_site_pages` | limit (per run) | config (5000) | 5000 | 16 |

Not entitlements (abuse protection, stay config): per-IP/per-visitor rate limits, PoW, `ai.contact_daily_runs`, per-recipient mail limits.

## Feedback shipping (13 -> 14)

- 13 emits `changelog_entry.published{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids[] (max 50), notify_voters}` and `changelog_entry.deleted{entry_id, purged}` (events-and-realtime.md).
- 14 listener `LinkFeedbackOnChangelogPublish` calls `feedback.LinkChangelogEntry(ctx, tx, workspaceID, entryID, postIDs)`; 14 sends voter emails when `notify_voters`. 13 sends to its subscribers minus `feedback.ShippedRecipientContactIDs(ctx, workspaceID, entryID)` (no double mail per contact).
- FKs `feedback_posts.changelog_entry_id`, `feedback_post_updates.changelog_entry_id` (`on delete set null`) declared inline by 14 migrations `001406` / `001414` (13 block runs first; no 13 migration on 14 tables).

## Shared GitHub layer (08 -> 08 knowledge, 08 docs sync, 13 changelog)

- 08 owns `github_connections` (fine-grained PAT, `kind` registry enum: `token` at MVP, `app` reserved for later `github-app`) and `github_repositories` (one webhook + secret per repo). Adapter `app/services/github` (go-github v93, 08 owner; 13 adds releases/compare/pulls files).
- One webhook route per repo `/webhooks/github/repositories/{ghr}` (rest.md exceptions); 08 dispatcher verifies `X-Hub-Signature-256`, dedupes by delivery id, fans out to registered `github.WebhookHandler`s (08 knowledge `push`, 08 docs sync `push`/`pull_request`, 13 changelog `release`). Polling fallback when no webhook.
- Consumers get clients only via `github.ClientFor(ctx, a, workspaceID, githubRepositoryID)`; never store token copies. Error codes `github_token_invalid`, `github_repo_not_found`, `github_permission_missing` raised by 08 layer.

## RetentionGuard (owner foundation; noop driver at MVP)

Legal hold and retention policies later = new driver, not edits in every purge job. Go: `app/contracts/retention.go`.

```go
type RetentionSubject struct { Kind string; ID ids.UUID } // workspace | contact | conversation | audit_log | inbound_email | attachment
type RetentionGuard interface {
    MayDestroy(ctx context.Context, workspaceID ids.UUID, s RetentionSubject) (bool, error)
}
```
- Every purge, erase, prune job calls `MayDestroy` before destroying. False = skip, record `retention_held` (log + metric; erase request stays `pending_hold`). MVP noop always true.
- Consumers: 00-accounts `PurgeWorkspace`, `PruneAuditLogs`; 03 `EraseContact`, spam purge; 04 raw `.eml` prune; 01 attachment cleanup (unattached uploads); 08 search logs prune; 07 `PruneAiTraces` (incl. `ai_action_calls` payloads); 16 import rollback (deletes imported rows) and import file purge; any data retention job (security.md "Privacy").
- Review rule: new destroy job must call guard. Feature test per job with fake guard returning false (nothing destroyed).
- Exempt (operational data, no held customer record; never call guard): `idempotency_keys` prune, River job cleanup, anything in Valkey (TTL expiry, never held), expired auth artifacts (sessions, one-time tokens, OAuth codes/tokens, portal sign-ins, widget realtime tokens), webhook delivery attempt logs past retention, 06 member notification prune (decided unguarded: member-facing copies; held content lives in conversations), stats rollups. Everything holding customer content or contact PII calls guard.

## AuditSink (owner foundation; noop driver at MVP)

Audit streaming to SIEM later = new driver. Go: `app/contracts/audit_sink.go`.

```go
type AuditSink interface {
    Write(ctx context.Context, entries []AuditEntry) error // AuditEntry = audit log resource fields (00-accounts), workspace id included
}
```
Called by queued listener `ForwardAuditEntries` after audit row insert (events-and-realtime.md). Errors retried by River, never block action. Later drivers: webhook (10 signing), S3 JSONL, Datadog, Splunk HEC; sinks configured per org. Pull alternative already at MVP: `GET /audit-logs?sort=created_at` ascending cursor + `filter[created_after]` (00-accounts).

## KeyManager (owner foundation)

Envelope encryption for every `*_encrypted bytea` column. Go: `framework/encryption` (Tink keysets; app uses `encryption.Seal/Open[T]` and column type `encryption.Sealed`). framework.md 3.2 = API. Storage: `encryption_keys` (data-model.md, foundation migration) behind app `encryption.KeysetStore`. Rules: security.md "Secrets".

```go
package encryption

type Scope struct {
    Kind string // system | organization | workspace
    ID   string // "" for system
}
type KeyManager interface {
    // Seals with scope's aead keyset (created on first use); Tink key-id prefix in ciphertext.
    Encrypt(ctx context.Context, scope Scope, plaintext, associatedData []byte) ([]byte, error)
    // Scope read from ciphertext; never guessed. Older keys in keyset still decrypt.
    Decrypt(ctx context.Context, ciphertext, associatedData []byte) ([]byte, error)
    MAC(ctx context.Context, scope Scope, data []byte) ([]byte, error) // deterministic HMAC-SHA256
    NewStreamWriter(ctx context.Context, scope Scope, w io.Writer) (io.WriteCloser, error)
    NewStreamReader(ctx context.Context, scope Scope, r io.Reader) (io.Reader, error)
}
type KEK interface { Wrap(ctx, plaintext, aad) ([]byte, error); Unwrap(ctx, wrapped, aad) ([]byte, error) }
type KeysetStore interface { Find; Create; Replace /* compare-and-swap */; All }

keys, _ := a.Crypt.Keys(store)          // *encryption.Keys: Rotate(ctx, scope), RewrapAll(ctx)
sealed, _ := encryption.Seal(ctx, keys, encryption.Slot{Scope: scope, AAD: aad}, value)
value, _ := encryption.Open[T](ctx, keys, sealed, aad)
```
- Keyset per (scope, purpose `aead` | `stream` | `mac`), one row each; not one row per data key. `Rotate(scope)` adds new primary to `aead` + `stream` keysets (old keys stay to decrypt); MAC keys never rotated automatically. `RewrapAll` re-wraps every keyset after KEK change (`dimmi key:rotate`), no data re-encryption.
- MVP scopes: `system` (user-level secrets: auth factors, OAuth identity tokens; users span workspaces), `workspace` (tenant secrets: channel credentials, provider keys, webhook secrets). `organization` reserved (CMK later: org-scoped KEK).
- KEK drivers (`encryption.kek` names instance in `encryption.keks`, `encryption.previous` still unwrap): `local` (derived from app key `app.key` / `<data_dir>/app.key`; optional own `key`), `awskms` (own thin driver on aws-sdk-go-v2 `kms`, Tink-compatible encryption context; tink-go-awskms needs SDK v1). Later `gcpkms`, `vault`.
- `aad` = table + column + row id (ciphertext cannot be moved between rows). Unwrapped keysets cached in memory (`CacheTTL` 5 min), never logged.
- App-key `Encrypter` (`a.Encrypter`): cookies, signed values; previous app keys decrypt only.

## Settings registry (owner foundation; keys declared by owner specs)

One declaration of setting keys with scopes and inheritance over existing storage. Go: `app/support/settings` (moved from 11).

```go
type Scope string // instance | organization | workspace | user
type Definition struct {
    Key         string   // "security.require_two_factor"
    Type        any      // Go type for decode
    Default     any
    Scopes      []Scope  // where it may be set
    Enforceable bool     // higher scope may lock it (org lock wins over workspace)
    Secret      bool
    Validate    func(any) error
}
func Get[T any](ctx context.Context, key string, at ScopeRef) (T, error) // resolves user -> workspace -> organization -> instance -> default
```
- Storage: instance = `system_settings` (11); organization = `organizations.settings`; workspace = `workspaces.settings`; user = existing columns/tables.
- Rule: scalar policy or preference = registry key in jsonb; anything with identity, lifecycle or many rows = own table.
- Keys added 2026-11-04, full list with defaults in owner specs: 03 `inbox.*`, `snooze.*`, `availability.*` (README Config), 02 `identity.mode`, `identity.client_writable_attributes`, 00-accounts `customer_facing_name`, 21 `security.*`.
- MVP keys: workspace keys from 00-accounts (`security.require_two_factor`, `previous_slugs`) and 09 (`agent_access`: client-kind allowlist only); instance keys from 11. No org keys used yet.
- Later: org lock on enforceable key; workspace `PATCH` of locked key returns 422 `setting_locked` (reserved code).

