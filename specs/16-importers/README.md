# 16 Importers

Style: caveman. Date 2026-11-02. Owner: 16-importers. Backend only. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*` (incl. `extensibility.md`, `contracts.md`), `specs/00-foundation/rest.md` (binding).

Files in folder:

| File | What |
|---|---|
| README.md | this: scope, source set, data model, code map, API table, MCP/CLI, events, jobs, config, permissions, budgets, required edits in other specs |
| [api.md](api.md) | request/response JSON, run lifecycle, preview shape, error codes, webhook payload |
| [sources.md](sources.md) | per-source facts (auth, limits, endpoints, verified 2026-11-02) and field mapping into Dimmi tables |
| [edge-cases.md](edge-cases.md) | decided behavior per case (140) |
| [tasks.md](tasks.md) | build checklist, demo script |

## Goal

Team switching from Intercom, Zendesk, Help Scout, Featurebase, Productlane, Canny or Ferndesk-style help center moves conversations, contacts, companies, tags, saved replies, help articles (old URLs keep working via 301), feedback posts with votes and changelog in under an hour. Safe (dry-run preview, nothing billed, nothing sent), resumable (checkpoints), idempotent (re-run never duplicates), reversible (rollback by import), incremental (delta sync during switchover window).

## Why

Roadmap launch item 17 (`research/roadmap.md` "17. Importers"), scope audit gap G2 (`research/scope-audit-4-competitors.md`). Switching cost is first objection. Salesforce/Fin churn window open now (`research/competitors/fin.md`). Featurebase imports from 10+ tools on free plan (`competitors/featurebase.md`), Productlane imports Intercom/Zendesk incl. collections, drafts, translations (`competitors/productlane.md`), Ferndesk imports 22 help center platforms by public URL with redirects, no API keys (`competitors/ferndesk.md`). Roadmap bar: `npx dimmi migrate intercom`, imports never billed, idempotent re-run, resumable job with progress, dry-run report first.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Typical switch (Intercom: 10k contacts, 10k conversations avg 8 parts, 300 articles, 2 GB attachments) end to end | under 30 min wall clock from token paste to `import.completed` (cloud and $5 VPS); preview under 60 s |
| Large Intercom (100k conversations, 50k contacts) | under 2 h at 30% of Intercom private-app budget (3,000 req/min) |
| Zendesk 50k tickets on Professional plan (400 req/min, incremental 10 req/min) | under 60 min via incremental ticket events (comments sideloaded) |
| Help Scout | bounded by account plan limit; preview shows ETA within 20% (CI fixture with simulated limit) |
| Public help center by URL (no keys), 500 articles | under 10 min incl. images rehosted, collections rebuilt, 301 per old URL |
| Writer throughput | at least 2,000 messages/s inserted (COPY into temp staging table path) on 2 vCPU Postgres; import never raises live inbox API p95 above 100 ms (import queue isolated, low priority, batch tx under 200 ms) |
| Duplicates after any number of re-runs, crashes, retries | 0 (property test: kill worker at random points 50 times, final row counts equal single clean run) |
| Billed conversations from import | 0 (12 replay fixture `import_never_billed`) |
| Outbound side effects from import | 0 emails, 0 channel sends, 0 webhooks except `import.completed`, 0 member notifications, 0 AI runs, 0 per-record realtime events (assertion helper `tc.AssertNoSideEffects`) |
| Old help center URL coverage | 100% of imported published articles and collections get 301 from old path (when old host is pointed at Dimmi) |
| Resume after crash or deploy | continues from last committed page, loses at most one page of work (re-fetched, deduped) |
| Rollback | removes every row the import created (not rows it matched), 100k conversations under 10 min, never touches records modified after import (kept and reported) |
| Delta sync during switchover | new source activity visible in Dimmi within `sync.interval_minutes` (default 10, min 5) |
| Agent-native | `npx dimmi migrate intercom` wizard end to end in one command; MCP tools for status, preview, run, rollback; credentials never pass through MCP |

## Scope

### In (MVP)

- Import framework: `imports` (one per source account), `import_runs` (`preview`, `full`, `delta`, `rollback`), checkpointed stages, mapping step, per-record errors, idempotency map `import_records`.
- Source drivers (registry enum `imports.source`, P1; contract `contracts.ImportSource`):

| Source | Credentials | Objects |
|---|---|---|
| `intercom` | access token (Developer Hub app), region `us`/`eu`/`au` | teammates, teams, tags, companies, contacts (users + leads), data attributes, conversations + tickets (parts, notes, attachments), help center collections + articles (all locales listed, one per help center) |
| `zendesk` | subdomain + agent email + API token | agents, groups, brands, organizations, end users, tags, user/org fields, tickets (comments public + private, CCs, attachments), macros (saved replies), Guide categories + sections + articles |
| `helpscout` | Mailbox app id + secret (OAuth2 client credentials); Docs API key optional | users, teams, mailboxes, customers, tags, conversations (threads, notes, line items, attachments), saved replies, Docs collections + categories + articles |
| `featurebase` | API key (Bearer) | boards, post statuses, posts, voters, comments, changelogs, help center collections + articles, contacts, companies, conversations |
| `productlane` | API key (Bearer, API v2) | members, tags, contacts, companies, threads (as conversations), changelogs, docs articles, portal projects/issues with upvotes (as feedback posts) |
| `canny` | API key | boards, categories, tags, statuses, posts, votes (voter identity), comments, users, companies, changelog entries |
| `public_site` | none: public help center URL | articles + collections from any public help center by sitemap (detectors: Intercom, Zendesk Guide, Help Scout Docs, Crisp, Featurebase, GitBook, Mintlify, Document360, Docusaurus, generic), images rehosted, 301 per old URL |
| `csv` | uploaded files | contacts CSV, companies CSV (column mapping) |

- Covered elsewhere, linked from `GET /import-sources` catalog (no 16 code): Markdown zip / Notion export / Git docs repo (GitBook, Mintlify, Docusaurus sources) = 08 knowledge source `import` mode (review queue); feedback CSV with Canny/Featurebase presets = 14 `/feedback-imports` (kept, decision below); changelog subscribers CSV = 13 `/changelog-subscriber-imports`.
- Dry-run preview: counts per object, date range, attachment size estimate, ETA from rate budget, discovered mappings with suggestions, sample of 20 transformed conversations and 5 articles rendered, conflicts (existing contacts matched), warnings. Writes nothing to business tables.
- Mapping step: teammates to members by email (unknown: "Former teammate" placeholder), teams, mailboxes/brands to inboxes, tags, custom attributes (rename, skip, type, `trusted` flag), statuses (feedback), boards, help center target + locale, changelog label.
- Attribute trust (07 agent actions): imported custom attributes never trusted by default (never added to `contacts.trusted_attribute_keys`); admin marks specific attribute mappings `trusted` (e.g. `stripe_customer_id`); writer then adds key for `kind user` contacts where import wrote the value. Leads never. Audited.
- Writers that preserve source timestamps (`created_at`, `first_seen_at`, `closed_at`, `published_at`), authorship (mapped member, placeholder, bot, contact), conversation status (`close_all` default or `preserve`), CCs, notes, attachments (re-uploaded via `Filesystem`), inline images, article HTML to Markdown, article cross-links rewritten, collections two levels.
- Redirects: 08 `help_center_redirects` rows `source = 'import'` from every old article/collection path (+ id-only variant).
- Side-effect suppression: import writers dispatch no per-record domain events; only `import.*` lifecycle events. Defense in depth: `messages.metadata.imported = true`, `conversations.source.imported` (12 D11, 07, 06, 10, 04 guards).
- Rate-limit-aware source clients: per source account token bucket in Valkey, header-driven backoff (`Retry-After`, `X-RateLimit-*`), share of source budget (default 50%) so customer's live tool keeps working.
- Resumable: River job per page, checkpoint (cursor) committed in same tx as page writes; pause, resume, cancel.
- Incremental: `delta` runs from watermark (`updated_since` per source), scheduled every `sync.interval_minutes` until `sync.until` (max 30 days).
- Rollback: `POST /imports/{imp}/runs {kind: "rollback"}` deletes rows with disposition `created` in reverse dependency order; keeps rows touched after import; `RetentionGuard` checked.
- GDPR: contact eraser removes import mapping rows and writes erasure tombstone so delta/re-run never re-imports erased person.
- Entitlements (cloud): concurrent runs, monthly records, monthly attachment bytes, public site page cap.
- Progress: REST + realtime (`import.progressed` coalesced 2 s) + system email to starter on finish.
- `import` channel driver (registry value of `channel_connections.type`): imported conversations rehome to inbox email address on first teammate reply.
- REST, MCP toolset `imports` (6 tools), CLI `dimmi migrate <source>` wizard + generated `dimmi imports ...`. Shares `dimmi migrate` with 11 DB migrations: positional source runs import wizard, no positional runs DB migrations, unknown source exits 2 (09 cli.md).

### Decision: 14 feedback CSV import

Kept in 14 (`feedback_imports`, `/feedback-imports`). Reasons: shipped design, file-based mapping UX differs, own cap `feedback.import_rows_per_month`. 16 feedback API sources (Featurebase, Canny, Productlane) write through 14 import write helpers (`feedback.UpsertImportedPost`, `UpsertImportedVote`, `UpsertImportedComment`, Required from 14) with `import_key = '<connection_key>:<external_id>'`, never through `feedback_imports`. `GET /import-sources` lists `feedback_csv` as kind `external` with link to 14 endpoint. Fold into 16 framework later (`feedback-csv-unify`).

### Out (later)

| Item | When |
|---|---|
| Crisp, Front, Freshdesk, Gorgias, HubSpot inbox, LiveChat/HelpDesk, Plain, Pylon, Kustomer conversations | later: `more-import-sources` (driver = new registry entry, zero migration) |
| Beamer, Headway, LaunchNotes changelog; Productboard, Frill, UserVoice feedback | later: `more-import-sources` |
| OAuth app install for Intercom / Zendesk / Help Scout (instead of pasted token) | later: `import-oauth` (credentials shape already per-source) |
| Intercom macros, Zendesk triggers/automations, SLAs, views, ticket fields as typed fields | later: macros after `more-import-sources`; rest with post-MVP: ticket fields, rules lite |
| Inviting unmapped teammates during import | later (MVP: placeholder or manual invite first) |
| Contact tags as entity (MVP: optional custom attribute string) | later (15 open question 1: no contact tags at MVP; custom attributes cover targeting) |
| Multi-locale help center (one source locale per target help center at MVP) | post-MVP: auto-translation (reserved `/{locale}/` prefix) |
| Two-way sync, writing back to source | never |
| `window.Intercom` shim | owned by 01 (Required from 01); 16 defines mapping table |
| Intercom `user_hash` (HMAC) identity verification compatibility | open question (default: not accepted) |
| Fold 14 feedback CSV into 16 | later: `feedback-csv-unify` |
| Spam folders, deleted tickets, side conversations, call recordings | never at MVP (skipped, counted) |

## User stories

- As owner switching from Intercom, I paste token, see "18,240 contacts, 9,811 conversations, 312 articles, 2.1 GB files, about 22 min", fix two teammate mappings, press Run, keep answering in Intercom; within 30 min Dimmi has full history; I point `help.acme.com` at Dimmi and old article links 301 to new ones.
- As owner during switchover week, delta sync every 10 min pulls new Intercom conversations so nothing gets lost while widget and email move over; when done I archive import, credentials destroyed.
- As admin, I run import twice by accident (or worker crashes): nothing duplicated.
- As admin unhappy with result, I roll back import: everything it created is gone, contacts that already chatted live in Dimmi stay.
- As member, I open contact Bob and see his 2023 Intercom conversations, written by "Sarah (former teammate)", with original dates and attachments.
- As developer, `npx dimmi migrate intercom` walks me through; `dimmi imports show imp_... --watch` streams progress.
- As AI agent (Claude Code via MCP), I check import status and propose rollback; I never see source tokens.
- As end customer, I receive no email, no survey, no notification because of import; my erase request also stops future re-import of my data.
- As finance on cloud, imported conversations never appear on invoice.

## Data model

Migration range `20261101001601-001699` (0016NN free since 15 moved to 0015NN, data-model.md). New prefixes `imp_` imports, `impr_` import_runs, `impm_` import_mappings, `impf_` import_files (checked against registry + reserved list 2026-11-02: no collision; `fbi_` is 14 feedback_imports). `import_run_stages`, `import_records`, `import_errors`, `import_erasures` have no public id (keyed by parent or internal). All tables carry `workspace_id`, RLS via `dimmi_visible_workspaces()`, cascade on workspace purge.

### imports (`imp_`)

One per source account per workspace (switch project). Long-lived: holds credentials, scopes, options, mapping, watermark.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `imp_` |
| workspace_id | uuid not null references workspaces on delete cascade | |
| source | text not null, registry enum | `intercom`, `zendesk`, `helpscout`, `featurebase`, `productlane`, `canny`, `public_site`, `csv`; format CHECK `^[a-z][a-z0-9_]{1,39}$`, Go registry validates |
| name | text not null | 1 to 80, default "Intercom import" |
| status | text not null default 'draft' CHECK in ('draft','ready','active','archived') | state machine below |
| connection_key | text not null | idempotency namespace `<source>:<account id>`: `intercom:<app_id>`, `zendesk:<subdomain>`, `helpscout:<company id>`, `public_site:<host>`, `csv:<imp id>` |
| source_account | jsonb not null default '{}' | `{id, name, region, url, plan_hint}` from `Connect` |
| credentials_encrypted | bytea null | `encryption.Sealed`, workspace scope, aad `imports.credentials_encrypted.<id>`; null after archive or purge |
| credentials_hint | text null | last 4 chars of primary secret |
| credentials_purge_after | timestamptz null | `last activity + imports.credentials.ttl_days` (30) |
| scopes | text[] not null default '{}' | subset of `contacts, companies, tags, conversations, saved_replies, articles, feedback, changelog`; validated against driver capabilities |
| options | jsonb not null default '{}' | typed `models.ImportOptions` (api.md "Options") |
| sync_enabled | boolean not null default false | delta schedule |
| sync_interval_minutes | int not null default 10 CHECK between 5 and 1440 | |
| sync_until | timestamptz null | max `now + 30 days` |
| next_sync_at | timestamptz null | scheduler |
| watermark | timestamptz null | source time up to which data complete (end of last successful full/delta minus 5 min overlap) |
| last_run_id | uuid null references import_runs on delete set null (deferrable) | |
| last_preview_run_id | uuid null references import_runs on delete set null (deferrable) | |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| archived_at, rolled_back_at | timestamptz null | |
| created_at, updated_at | timestamptz | |

Indexes: `(workspace_id, created_at desc)`; unique `(workspace_id, connection_key) where status <> 'archived'` (one live import per source account: 409 `import_exists`); `(next_sync_at) where sync_enabled and status = 'active'` (scheduler scan by system job, documented RLS-bypass exception like 08 domains).

State machine (strict CHECK):
```
draft  -> ready     (preview run completed)
ready  -> active    (first full run queued)
draft|ready|active -> archived (PATCH {status: "archived"}: credentials shredded, sync off; rollback still allowed)
ready  -> draft     (credentials or scopes changed: preview stale)
active -> active    (delta, rollback runs)
archived: final (rollback run allowed; DELETE allowed)
```

### import_runs (`impr_`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `impr_` |
| workspace_id | uuid not null | |
| import_id | uuid not null references imports on delete cascade | |
| kind | text not null CHECK in ('preview','full','delta','rollback') | |
| status | text not null default 'queued' CHECK in ('queued','running','paused','completed','completed_with_errors','failed','cancelled') | |
| pause_reason | text null CHECK in ('member','rate_limited','credentials_invalid','entitlement_limit_reached','source_unavailable') | |
| scope | text[] not null | scopes this run covers (rollback may be partial) |
| window_since, window_until | timestamptz null | delta window on source `updated_at` |
| preview | jsonb null | preview result (api.md "Preview") |
| totals | jsonb not null default '{}' | per object `{created, updated, matched, skipped, failed, kept}` rolled up at finalize |
| error_count | int not null default 0 | |
| last_error_code, last_error | text null | run-level failure (`source_credentials_invalid`, `source_unavailable`, `internal`) |
| resume_at | timestamptz null | auto-resume time for `rate_limited`, `source_unavailable`, `entitlement_limit_reached` |
| started_by_type | text not null CHECK in ('member','token','system') | system = scheduled delta |
| started_by_id | uuid null | |
| queued_at, started_at, finished_at, heartbeat_at | timestamptz | heartbeat every page commit; stale over 15 min = requeue |
| created_at, updated_at | timestamptz | |

Indexes: `(workspace_id, import_id, created_at desc)`; unique `(import_id) where status in ('queued','running','paused')` (one active run per import, 409 `import_run_active`); `(resume_at) where status = 'paused' and resume_at is not null` (scheduler).

State machine:
```
queued -> running (worker picks first stage)
running -> paused (member PATCH, rate limit wait over 2 min, credentials 401, source 5xx streak, entitlement)
paused -> running (member PATCH {status: "running"}, resume_at reached, credentials PATCHed)
queued|running|paused -> cancelled (member PATCH; committed pages stay)
running -> completed | completed_with_errors (record errors > 0) | failed (run-level error after retries)
terminal states final
```

### import_run_stages

Checkpoint per stage. PK `(run_id, stage)`. `workspace_id, run_id references import_runs on delete cascade, stage text CHECK in ('discover','tags','teams','companies','contacts','conversations','attachments','saved_replies','collections','articles','article_links','redirects','feedback','changelog','finalize','rollback'), position smallint, status text CHECK in ('pending','running','done','skipped','failed'), cursor jsonb null (opaque driver cursor, never secrets), total_estimate bigint, created bigint, updated bigint, matched bigint, skipped bigint, failed bigint, kept bigint, bytes bigint, started_at, finished_at, updated_at`. Row lock per stage only (parallel stages never contend).

### import_mappings (`impm_`)

Discovered source entities and decided targets.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `impm_` |
| workspace_id, import_id | uuid | cascade with import |
| kind | text not null CHECK in ('teammate','team','inbox','tag','attribute','status','board','category','help_center','locale','label') | |
| external_id | text not null | |
| external_name | text not null | |
| external_meta | jsonb not null default '{}' | email, object (contact/company), type, sample values (max 3, 64 chars), usage count |
| action | text not null CHECK in ('map','create','skip','placeholder') | `placeholder` only for teammate |
| target_type | text null | `workspace_member`, `team`, `inbox`, `tag`, `custom_attribute`, `feedback_status`, `feedback_board`, `feedback_category`, `help_center`, `changelog_label` |
| target_id | uuid null | for `map` |
| target_key | text null | attribute key / locale / name override for `create` |
| trusted | boolean not null default false | attribute kind only (CHECK `trusted = false or kind = 'attribute'`); true = imported values of this key join `contacts.trusted_attribute_keys` (07) for user contacts |
| suggested | boolean not null default true | false once member edits |
| conflict | text null | `attribute_key_taken_other_type`, `status_cap_reached` (14 `feedback.statuses`), `label_cap_reached` (13 `changelog.max_labels`), `member_inactive`, `ambiguous_email` (blocks full run) |
| created_at, updated_at | timestamptz | |

Unique `(import_id, kind, external_id)`. Re-discovery upserts names/meta, never overwrites `suggested = false` decisions.

### import_records

Idempotency and provenance map. PK `(workspace_id, connection_key, object_type, external_id)`.

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid not null | |
| connection_key | text not null | |
| object_type | text not null CHECK in ('team','tag','company','contact','conversation','saved_reply','collection','article','redirect','feedback_board','feedback_post','feedback_comment','changelog_entry','attachment_blob') | messages dedupe by `messages.dedupe_key`; votes by 14 unique `(post, contact)` |
| external_id | text not null | source id, max 255 |
| record_id | uuid not null | Dimmi row |
| import_id | uuid not null references imports on delete cascade | creating import |
| run_id | uuid null references import_runs on delete set null | first run |
| disposition | text not null CHECK in ('created','matched') | `matched` = existing Dimmi row linked (never deleted by rollback) |
| source_updated_at | timestamptz null | delta change detection |
| checksum | text null | sha256 of normalized source payload; equal = skip update |
| created_at, updated_at | timestamptz | |

Indexes: `(workspace_id, import_id, object_type, record_id)` (rollback), `(workspace_id, object_type, record_id)` (reverse lookup "imported from").

### import_errors

`id uuid pk, workspace_id, run_id references import_runs on delete cascade, stage text, object_type text, external_id text, code text not null (registry list in api.md "Record error codes"), message text (max 500, never body text or tokens), attempt smallint, created_at`. Index `(workspace_id, run_id, created_at)`. Cap `imports.errors.max_per_run` (10,000) rows; beyond: counters only, run `meta.errors_truncated`. Pruned 90 days after run finish.

### import_files (`impf_`)

CSV uploads for `csv` source. `id, workspace_id, import_id cascade, kind text CHECK in ('contacts_csv','companies_csv'), disk, path, filename, size_bytes, sha256, row_count int, header jsonb (column names), sample jsonb (first 5 rows, shown in mapping, deleted with file), created_by_member_id, created_at, updated_at`. Disk `imports` (private). Max `imports.csv.max_file_mb` (50). Files deleted on archive and on import delete.

### import_erasures

Tombstones so erased contacts never come back via delta/re-run. PK `(workspace_id, connection_key, key_hash)`. `workspace_id, connection_key text, key_kind text CHECK in ('external_id','email'), key_hash bytea (HMAC-SHA256 via `framework/encryption` MAC, workspace scope), created_at`. No plaintext. Written by eraser for every external id and email of erased contact that came from import (or matches mapping).

### Core table changes

No column changes. JSON keys and registry values only:

| Where | Change | Owner of base |
|---|---|---|
| `conversations.source.imported` | `{import_id, source, external_id, external_number, external_url, channel}` set once by writer; read by 12 (`conv.Source.Imported`), 03 resource (`imported` block), 07 | 03 typed `models.ConversationSource` |
| `messages.metadata.imported` | `true` on every imported message/note/event row (12 `IsActivity` excludes) | 03 |
| `messages.metadata.import` | `{source, external_id, author: {kind: "teammate"\|"former_teammate"\|"bot"\|"contact"\|"unknown", name, email?, external_id}, truncated_before?: n}` | 03 |
| `messages.dedupe_key` | `imp:<connection_key>:<part external id>` (unique, existing index) | foundation |
| `channel_connections.type` registry | value `import` (driver `app/drivers/channel/import`) | 03 registry |
| `help_center_redirects.source` | existing value `import` | 08 |
| `feedback_posts.source`, `feedback_votes.source/trust`, `changelog_entries.source_kind` | existing value `import` | 14, 13 |
| `attachments.uploaded_by_type` | value `system` (provenance via `import_records` `attachment_blob`) | foundation |

Migrations:
```
database/migrations/20261101001601_create_imports_table.sql
database/migrations/20261101001602_create_import_runs_table.sql          # + deferred FKs imports.last_run_id, last_preview_run_id
database/migrations/20261101001603_create_import_run_stages_table.sql
database/migrations/20261101001604_create_import_mappings_table.sql
database/migrations/20261101001605_create_import_records_table.sql
database/migrations/20261101001606_create_import_errors_table.sql
database/migrations/20261101001607_create_import_files_table.sql
database/migrations/20261101001608_create_import_erasures_table.sql
database/migrations/20261101001609_add_import_rls_policies.sql
```

## Pipeline

```
POST /imports (creds) --Connect (sync, 10 s)--> imports draft
POST /imports/{imp}/runs {kind: preview} --> PreviewImport job
     Discover (teammates, teams, inboxes, tags, attributes, statuses, boards, locales, counts)
     -> import_mappings upsert with suggestions; sample transform (no business writes) -> run.preview; import ready
PATCH /import-mappings/{impm} | PATCH /imports/{imp}/mappings (bulk) | PATCH /imports/{imp} {options, scopes}
POST /imports/{imp}/runs {kind: full} --> stages in order (each stage = chain of page jobs):
     tags -> teams -> companies -> contacts -> conversations (+ attachments lane in parallel)
     -> saved_replies -> collections -> articles (+ images) -> article_links -> redirects
     -> feedback (boards/statuses mapped, posts + votes + comments per page) -> changelog -> finalize
page job: fetch page from source OUTSIDE tx (rate budget) -> normalize -> short tx:
     SET LOCAL app.workspace_id; advisory xact lock (workspace, connection_key, stage);
     upsert rows via writer (`tx.CopyFrom` into temp staging table + `INSERT ... SELECT ... ON CONFLICT DO NOTHING`; COPY cannot load RLS tenant tables for the app role, so never COPY into tenant tables directly; small batches use batched multi-row INSERT); import_records upsert;
     stage counters + cursor; heartbeat; InsertTx next page job (or next stage) -> commit
finalize: recompute contact/company/conversation aggregates touched, run totals, watermark,
     dispatch import.completed (webhook) -> system mail to starter
delta (scheduled or manual): same stages with window_since = watermark - 5 min; append-only for touched records
rollback: reverse order via import_records disposition=created, batches of 1,000, RetentionGuard per subject; owner delete helpers where owner holds side tables (13 `changelog.DeleteImportedEntries`, 14 `feedback.DeleteImportedPosts`), no events
```

Suppression rule: writers in `app/actions/imports/writers` never call `a.Events.Dispatch` for record-level events and never call 03 actions that dispatch (`ReceiveInboundMessage`, `ReplyToConversation`, `CreateConversation`, `UpsertContact`). They call 03/08/13/14 import write helpers (Required from other specs) that insert rows with invariant checks but no events. Arch test: `app/actions/imports/writers` imports no `app/events` package and no dispatching action.

## Code map

```
app/contracts/import_source.go                 # ImportSource interface + normalized record types (ImportedContact, ImportedConversation{Parts}, ImportedArticle ...), ImportCursor, RateBudget, capability flags
app/drivers/importsource/
  intercom/        client.go (HTTP over a.HTTP, Intercom-Version header pinned), source.go, map_conversation.go, map_article.go
  zendesk/         client.go (basic auth email/token, incremental cursor + ticket_events comment_events), source.go, map_ticket.go, map_guide.go
  helpscout/       client.go (client-credentials token cache 2 days, Docs API basic auth), source.go, map_conversation.go, map_docs.go
  featurebase/     client.go (Featurebase-Version pinned, cursor), source.go, map_*.go
  productlane/     client.go (API v2, cursor/has_more), source.go, map_*.go
  canny/           client.go (POST + apiKey body, skip/limit and cursor), source.go, map_*.go
  publicsite/      detect.go (platform fingerprints), source.go (sitemap enumerate via 08 crawl), extract_<platform>.go (selectors: title, body, breadcrumb, updated)
  csv/             source.go (reads import_files via Filesystem, stdlib encoding/csv, delimiter + BOM sniff)
  fake/            source.go (deterministic synthetic tenant: N contacts, conversations, parts; fault injection: 429, 5xx, 401, truncated parts, expiring attachment URL)
app/drivers/channel/import/channel.go          # Channel driver type `import`: TransportNone, StartNever; implements ChannelRehomer (03) -> inbox email connection
app/support/importkit/
  ratebudget.go     # per (source, account) token bucket in Valkey (`dimmi:rl:import:`) + header parse (Retry-After, X-RateLimit-*, X-Rate-Limit-Remaining, ratelimit-reset)
  htmlmd.go         # HTML -> Markdown (framework/dom.ToMarkdown) + sanitize; inline image extraction; Intercom/Zendesk quirks (empty <p>, <br> runs)
  attrs.go          # custom attribute key normalize (`[a-z0-9_]`, 1-64), value coercion (date -> ISO 8601, bool, number, list -> comma string), 100-key cap
  paths.go          # old URL -> redirect from_path normalize (08 rules) + id-only variants
  links.go          # article cross-link rewrite (source URL/id -> new article URL)
  checksum.go       # stable sha256 of normalized record
app/actions/imports/
  create_import.go, update_import.go, archive_import.go, delete_import.go
  connect_import_source.go           # validates creds via driver Connect, fills source_account, connection_key
  start_import_run.go                # kind dispatch, preconditions, entitlements, enqueues first stage
  update_import_run_status.go        # pause, resume, cancel (field dispatch from PATCH)
  discover_import.go                 # mappings upsert + suggestions
  update_import_mapping.go, bulk_update_import_mappings.go
  store_import_file.go, delete_import_file.go
  preview_import.go                  # sample transform, estimates
  run_import_page.go                 # generic page loop: fetch, normalize, write, checkpoint, enqueue next
  finalize_import_run.go
  rollback_import_page.go
  schedule_import_deltas.go
  purge_import_credentials.go
  erase_import_contact_data.go       # ContactEraser (contracts.md)
  resolve_import_author.go           # teammate mapping -> author tuple
  writers/{tags,teams,companies,contacts,conversations,attachments,saved_replies,collections,articles,article_links,redirects,feedback,changelog}.go
app/http/controllers/api/v1/
  import_source_controller.go        # Index
  import_controller.go               # Index, Store, Show, Update, Destroy
  import_mapping_controller.go       # Index, UpdateMany (collection), Update (shallow /import-mappings/{impm})
  import_run_controller.go           # Index, Store (nested), Show, Update (shallow /import-runs/{impr})
  import_run_error_controller.go     # Index
  import_record_controller.go        # Index
  import_file_controller.go          # Index, Store (multipart), Destroy
app/http/requests/imports/           # store_import_request.go (source-discriminated credentials), update_import_request.go, store_import_run_request.go, update_import_run_request.go, update_import_mapping_request.go, bulk_update_import_mappings_request.go, store_import_file_request.go
app/http/resources/                  # import_resource.go, import_run_resource.go, import_mapping_resource.go, import_record_resource.go, import_error_resource.go, import_file_resource.go, import_source_resource.go
app/policies/import_policy.go        # imports:read, imports:manage, imports:rollback
app/events/                          # import_created.go, import_updated.go, import_deleted.go, import_started.go, import_progressed.go, import_paused.go, import_resumed.go, import_completed.go, import_failed.go, import_cancelled.go, import_rolled_back.go
app/listeners/                       # coalesce_import_progress.go (AfterCommit, 2 s window per run), send_import_finished_mail.go (Queued)
app/jobs/                            # preview_import.go, run_import_page.go, import_conversation_batch.go, download_import_attachment.go, finalize_import_run.go, rollback_import_page.go, schedule_import_deltas.go, resume_paused_import_runs.go, requeue_stale_import_runs.go, purge_import_credentials.go, prune_import_errors.go
app/mail/import_finished_mail.go     # system stream to starter (completed, completed_with_errors, failed); resources/views/mail/import_finished.templ
app/mcp/tools/imports.go             # toolset `imports`
app/console/commands/client/migrate_command.go   # `dimmi migrate <source>` wizard (09 client mode)
app/models/                          # sqlc output + ImportOptions, ImportSourceAccount typed jsonb
app/enums/import_status.go, import_run_kind.go, import_run_status.go, import_stage.go, import_scope.go, import_source.go (registry)
config/imports.go
database/queries/{imports,import_runs,import_run_stages,import_mappings,import_records,import_errors,import_files,import_erasures}.sql
database/factories/import_factory.go
tests/feature/imports/               # http_test.go, preview_test.go, mapping_test.go, run_test.go, resume_test.go, idempotency_test.go, delta_test.go, rollback_test.go, suppression_test.go, billing_test.go, erasure_test.go, redirects_test.go, entitlements_test.go, cross_tenant_test.go, mcp_test.go
tests/contracts/import_source_test.go  # drivertest suite every driver passes against recorded fixtures (tests/fixtures/imports/<source>/*.json served by httptest upstream)
tests/arch/imports_test.go           # writers dispatch no events; drivers import no app/actions
```

Reuse check:
- Intercom Go SDK `github.com/intercom/intercom-go` (license file, no releases, README "limited capacity", targets API 1.3): not used, API 2.x shapes needed. Thin client on `framework/httpclient` (each endpoint one GET/POST + JSON struct).
- Zendesk `github.com/nukosuke/go-zendesk` (MIT, v0.18.0, 2024-02-06, over 12 months stale, no ticket-events cursor, partial Guide): not used. Thin client.
- Help Scout, Featurebase, Productlane, Canny: no Go SDK found (pkg.go.dev search 2026-11-02). Thin clients; OAuth2 client credentials token via `framework/socialite` x/oauth2 `clientcredentials` (wrapped) instead of own token code.
- HTML to Markdown: `JohannesKaufmann/html-to-markdown/v2` v2.5.2 (MIT) via `framework/dom.ToMarkdown` (already decided, build-vs-reuse 3.5). Sanitize: bluemonday via `framework/sanitize`. Readability for `public_site` generic fallback: `codeberg.org/readeck/go-readability/v2` v2.1.3 via `framework/dom.Readable` (08). Sitemap, robots, politeness: 08 `app/support/crawl` reused, not copied.
- CSV: stdlib `encoding/csv` (+ BOM strip, delimiter sniff ~30 lines, same helper as 14 `support/csvsniff`; one shared file, 14 owns).
- Rate limiting: `golang.org/x/time/rate` semantics on Valkey counter via `framework/ratelimit`; header parsing is glue (< 100 lines).
- HMAC for erasure tombstones: `framework/encryption` MAC (Tink) never hand-written.
- Slugs: `gosimple/slug` via 08 `support/slug`.

## HTTP API

All under `/api/v1`, auth `A` + `W`. Abilities: `settings:read` (imports:read), `settings:write` (imports:manage, imports:rollback). JSON shapes in [api.md](api.md).

| Method | Path | Route name | Auth + permission | Purpose |
|---|---|---|---|---|
| GET | /import-sources | import-sources.index | A `imports:read` | catalog: key, name, kind (`api`, `public_url`, `file`, `external`), objects, credential fields, docs URL, enabled |
| GET | /imports | imports.index | A `imports:read` | filter `status`, `source`; sort `-created_at` |
| POST | /imports | imports.store | A `imports:manage` | create with source, credentials, scopes, options; validates credentials (sync, 10 s); 201 |
| GET | /imports/{imp} | imports.show | A `imports:read` | include `last_run`, `last_preview_run` |
| PATCH | /imports/{imp} | imports.update | A `imports:manage` | name, credentials (replace), scopes, options, `sync_enabled`, `sync_interval_minutes`, `sync_until`, `status: "archived"` |
| DELETE | /imports/{imp} | imports.destroy | A `imports:rollback` + `password.confirm` | only when no active run and (no `created` records or archived); removes import, mappings, records map, files; imported data stays (409 `import_has_records` otherwise with hint to archive) |
| GET | /imports/{imp}/mappings | imports.mappings.index | A `imports:read` | filter `kind`, `action`, `conflict` |
| PATCH | /imports/{imp}/mappings | imports.mappings.update-many | A `imports:manage` | bulk `{ids, changes: {action, target_id?, target_key?}}` max 100 (rest.md rule 8) |
| PATCH | /import-mappings/{impm} | import-mappings.update | A `imports:manage` | `action`, `target_id`, `target_key` |
| GET | /imports/{imp}/runs | imports.runs.index | A `imports:read` | |
| POST | /imports/{imp}/runs | imports.runs.store | A `imports:manage` (`rollback`: `imports:rollback` + `password.confirm`) | `{kind, scope?}` 202 job-like run |
| GET | /import-runs/{impr} | import-runs.show | A `imports:read` | stages, totals, ETA, preview |
| PATCH | /import-runs/{impr} | import-runs.update | A `imports:manage` | `{status: "paused" \| "running" \| "cancelled"}` |
| GET | /import-runs/{impr}/errors | import-runs.errors.index | A `imports:read` | filter `stage`, `code`, `object_type`; `?format=csv` |
| GET | /imports/{imp}/records | imports.records.index | A `imports:read` | lookup `filter[object_type]`, `filter[external_id]` or `filter[record_id]`; "where did ticket 4512 go" |
| GET | /imports/{imp}/files | imports.files.index | A `imports:read` | |
| POST | /imports/{imp}/files | imports.files.store | A `imports:manage` | multipart CSV, `kind`; 201 with header + sample |
| DELETE | /import-files/{impf} | import-files.destroy | A `imports:manage` | |

No verbs in paths; preview, run, delta, rollback are run kinds (rest.md rule 4); pause/resume/cancel are `status` field (rule 3). `Idempotency-Key` honored on `POST /imports` and `POST /imports/{imp}/runs`.

Lookups added to other resources (Required from 03/08): conversation resource `imported` block `{import_id, source, external_id, external_number, external_url}`; `GET /conversations?filter[imported]=true|false` and `filter[import_id]=imp_...`; contact resource `imported_from` (first import record) when `include=import`.

## MCP tools and CLI

MCP toolset `imports` (09 registry; enabled via `?toolsets=imports`; under 2,000 tokens). Credentials never accepted or returned by MCP: `dimmi_start_import` for credentialed sources returns `credentials_url` (dashboard deep link) and CLI hint; only `public_site` runs fully over MCP.

| Tool | Risk | REST | Notes |
|---|---|---|---|
| `dimmi_list_import_sources` | read | `GET /import-sources` | |
| `dimmi_start_import` | write | `POST /imports` (no credentials) + `POST /imports/{imp}/runs {kind: preview}` | `public_site` takes `url`; others return `credentials_url` |
| `dimmi_get_import` | read | `GET /imports/{imp}` + last run + preview + mapping conflicts | progress, ETA, errors summary |
| `dimmi_run_import` | write (destructive when `articles.publish = preserve`: publishes content) | `POST /imports/{imp}/runs {kind: full\|delta}` | confirm gate shows counts and "N articles will go live" |
| `dimmi_cancel_import` | write | `PATCH /import-runs/{impr} {status: cancelled}` | |
| `dimmi_rollback_import` | destructive | `POST /imports/{imp}/runs {kind: rollback, scope?}` | always confirmation with counts to delete; token needs `settings:write` and owner/admin role |

CLI (09 client mode, space style):

```
npx dimmi migrate intercom|zendesk|helpscout|featurebase|productlane|canny|site|csv
  [--token-env INTERCOM_TOKEN] [--region eu] [--subdomain acme] [--url https://help.acme.com]
  [--scopes contacts,conversations,articles] [--since 2024-01-01] [--inbox inbox_...]
  [--help-center hc_...] [--publish preserve|draft] [--map mapping.yaml] [--yes] [--sync 10m --sync-until 7d]
dimmi imports list | show <imp> [--watch] | runs <imp> | errors <impr> [--csv] | records <imp> --external-id 4512
dimmi imports cancel <impr> | pause <impr> | resume <impr> | archive <imp> | rollback <imp> [--scope articles] (asks typed confirmation)
```

Wizard flow (`migrate_command.go`): login check (`GET /me`, needs `settings:write`) -> prompt secret with hidden input (or env var; never flag value, never written to disk or shell history) -> `POST /imports` -> preview (spinner on realtime `import.progressed`) -> print counts, ETA, mapping conflicts table -> optional `--map` YAML apply or interactive fix of conflicts -> confirm (`--yes` skips) -> `POST runs {kind: full}` -> live progress bars per stage -> summary + next steps checklist (install widget / `dimmi init`, Intercom shim, email forwarding (04), point old help center host at Dimmi (08 custom domain), enable delta sync). Exit codes: 0 completed, 2 completed_with_errors, 1 failed/cancelled.

## Events

New events (Required from 00-foundation catalog). Payload `{import_id, run_id?, kind?, status, counts?}`; resource `import` or `import_run`.

| Event | Realtime | Webhook | Audit | Notes |
|---|---|---|---|---|
| import.created | yes (`workspace:`) | no | yes | |
| import.updated | yes | no | yes (credentials change: `credentials_changed: true`, never values) | incl. archive |
| import.deleted | yes | no | yes | |
| import.started | yes (`workspace:` + `member:` of starter) | no | yes | run queued -> running (kind) |
| import.progressed | yes, coalesced 2 s per run, history kept (not ephemeral) | no | no | stage, percent, counts |
| import.paused / import.resumed | yes | no | yes (member-initiated only) | `pause_reason` |
| import.completed | yes | **yes** (object `import_run`) | yes | full/delta/preview terminal success incl. `completed_with_errors` (`status` field tells) |
| import.failed | yes | no (later additive) | yes | |
| import.cancelled | yes | no | yes | |
| import.rolled_back | yes | no | yes | rollback run finished |

Per-record domain events (`conversation.created`, `message.created`, `contact.created`, `company.created`, `tag.created`, `article.*`, `feedback_*`, `changelog_entry.*`) never dispatched for imported rows. Dashboard and apps refetch lists on `import.progressed` / `import.completed` (list `updated_since` cursor). Consumed: `contact.deleted` (03 GDPR erase; eraser registered, no listener needed), `member.deactivated` (mapping suggestions skip inactive members), `workspace.deleted` (cascade).

## Jobs

Queue `imports` (new, River; `imports.queue.workers` default 4 self-host, 8 cloud per node; priority below `default`, `notifications`, `channels`, `ai`). Attachments lane = queue `imports_files` (workers 8) so text never waits for blobs.

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| PreviewImport (`preview_import`) | imports | run kind preview | 3 | `ByArgs` run_id |
| RunImportPage (`run_import_page`) | imports | stage start, previous page commit (InsertTx), resume | 10 (backoff; 429 uses `Retry-After`, not retry count) | `ByArgs` (run_id, stage, page_token) |
| ImportConversationBatch (`import_conversation_batch`) | imports | conversations page with detail fetch (Intercom, Help Scout): fan-out up to 25 ids per batch, max `imports.conversations.parallel_batches` (4) in flight per run | 10 | `ByArgs` (run_id, batch_key) |
| DownloadImportAttachment (`download_import_attachment`) | imports_files | message/article write with external file | 5, then placeholder | `ByArgs` (workspace_id, connection_key, external file id) |
| FinalizeImportRun (`finalize_import_run`) | imports | last stage done | 5 | `ByArgs` run_id |
| RollbackImportPage (`rollback_import_page`) | imports | rollback run, chain | 10 | `ByArgs` (run_id, object_type, page) |
| ScheduleImportDeltas (`schedule_import_deltas`) | imports | periodic 1 min | 1 | periodic |
| ResumePausedImportRuns (`resume_paused_import_runs`) | imports | periodic 1 min (`resume_at <= now`) | 1 | periodic |
| RequeueStaleImportRuns (`requeue_stale_import_runs`) | imports | periodic 5 min (`heartbeat_at` older 15 min, status running) | 1 | periodic |
| PurgeImportCredentials (`purge_import_credentials`) | maintenance | daily | 3 | periodic |
| PruneImportErrors (`prune_import_errors`) | maintenance | daily (90 days) | 3 | periodic |
| SendImportFinishedMail (listener job) | mail | import.completed, import.failed (not preview, not scheduled delta unless errors) | 5 | `ByArgs` run_id |

Every page job: sets tenant (`SET LOCAL app.workspace_id`), takes `pg_advisory_xact_lock(hash(workspace, connection_key, stage))`, checks run status (cancelled/paused: exit without enqueue), verifies credentials still present.

## Config keys

`config/imports.go`, env `DIMMI_IMPORTS__*`.

| Key | Default | Notes |
|---|---|---|
| imports.enabled | true | false hides catalog, 404 on routes |
| imports.sources.<key>.enabled | true | per driver kill switch |
| imports.queue.workers / imports.files_queue.workers | 4 / 8 | |
| imports.rate_share | 0.5 | fraction of source documented budget used; per import override `options.rate_share` 0.1 to 0.9 |
| imports.page_size.<source> | source max (Intercom 150, Zendesk 1000 incremental / 100 cursor, Help Scout 50, Featurebase 100, Productlane 200, Canny 100) | |
| imports.conversations.parallel_batches | 4 | per run |
| imports.attachments.max_bytes | `filesystems.max_upload_mb` (25 MB) | larger: placeholder line |
| imports.credentials.ttl_days | 30 | after last run finish; then shredded, import stays |
| imports.errors.max_per_run | 10000 | |
| imports.delta.min_interval_minutes | 5 | |
| imports.delta.max_window_days | 30 | `sync_until` cap |
| imports.delta.overlap | 5m | |
| imports.preview.sample_conversations / sample_articles | 20 / 5 | |
| imports.csv.max_file_mb / max_rows | 50 / 500000 | |
| imports.public_site.max_pages | 5000 | self-host config; cloud via entitlement |
| imports.public_site.rate_per_host | 2 req/s, concurrency 2 | 08 crawl politeness |
| imports.intercom.api_version | `2.17` (current at 2026-11-02 check) | pinned header `Intercom-Version`; fixtures recorded against it |
| imports.featurebase.api_version | `2026-08-19.orbit` | pinned header `Featurebase-Version` |
| imports.stale_heartbeat | 15m | |

Entitlement catalog keys (owner 16, `contracts.md` Entitlements):

| Key | Kind | Self-host | Cloud default |
|---|---|---|---|
| `imports.concurrent_runs` | limit (per workspace, active runs) | config (2) | 2 |
| `imports.records_monthly` | limit (monthly; top-level records created: contacts, companies, conversations, articles, feedback posts, changelog entries, saved replies) | -1 | 1,000,000 |
| `imports.attachment_bytes_monthly` | limit (monthly) | -1 | 107,374,182,400 (100 GB) |
| `imports.public_site_pages` | limit (per run) | config (5000) | 5,000 |

Over limit at run start: 422 `entitlement_limit_reached`. Hit mid-run: run `paused` (`pause_reason entitlement_limit_reached`, `resume_at` = period reset), never failed, never partial page.

## Permissions matrix

| Permission | Ability group | Owner | Admin | Member | Use |
|---|---|---|---|---|---|
| imports:read | settings:read | yes | yes | no | sources catalog, imports, runs, errors, records, files |
| imports:manage | settings:write | yes | yes | no | create, credentials, mappings, preview, full/delta runs, pause/cancel, archive, files |
| imports:rollback | settings:write | yes | yes | no | rollback runs, DELETE import; always `password.confirm` (423 `password_confirmation_required`) |

Tokens: role permissions ∩ abilities (`settings:read` / `settings:write`). Members see imported data through normal 03/08/14 permissions; `imported` block on conversation resource visible to anyone who can read conversation.

## Performance budgets

| Operation | Budget |
|---|---|
| `POST /imports` incl. credential check | p95 under 3 s (source round trip), hard timeout 10 s |
| Page write tx | under 200 ms p95 (batch ≤ 500 messages or ≤ 150 conversations; split otherwise) |
| Writer throughput | ≥ 2,000 messages/s, ≥ 1,000 contacts/s per worker on 2 vCPU Postgres |
| Live API impact during import | inbox list/reply p95 unchanged within 10% (bench with fake source 100k conversations running) |
| Memory per worker | under 64 MB (stream pages, never whole export in memory; attachments streamed to Filesystem) |
| Progress latency | realtime update within 3 s of page commit |
| Rollback | ≥ 500 conversations/s deleted incl. attachments metadata (files deleted async by 03 attachment purge) |
| Preview | under 60 s for 50k contacts / 100k conversations (counts via source count endpoints, sample only) |

## Observability

Metrics: `dimmi_import_records_total{source,object,outcome}` (created, matched, updated, skipped, failed), `dimmi_import_source_requests_total{source,status}`, `dimmi_import_source_rate_wait_seconds{source}`, `dimmi_import_page_seconds{source,stage}`, `dimmi_import_attachment_bytes_total{source}`, `dimmi_import_runs{status}` gauge, `dimmi_import_run_duration_seconds{source,kind}`. Logs: run start/finish summary (ids, counts, duration), source 4xx/5xx with endpoint template (no query strings, no tokens), pause reasons. Never log tokens, contact emails, message bodies, article bodies. Trace span per page with source request spans.

## Security and privacy

- Credentials: sealed with workspace DEK (`KeyManager`), aad bound to row; write-only in API (`credentials: {configured, hint, fields}`); never in logs, errors, MCP, CLI history. Shredded on archive, after `credentials_purge_after`, on workspace delete. Rotating source token: `PATCH /imports/{imp} {credentials}` validates first.
- Outbound only to fixed provider hosts per driver (`api.intercom.io`, `api.eu.intercom.io`, `api.au.intercom.io`, `<subdomain>.zendesk.com` with subdomain `^[a-z0-9][a-z0-9-]{0,62}$`, `api.helpscout.net`, `docsapi.helpscout.net`, `do.featurebase.app`, `productlane.com`, `canny.io`); attachment and image URLs follow redirects via `framework/httpclient` SSRF guard (every hop). `public_site` = any public host, SSRF guard + robots + politeness (08 crawl).
- Imported content untrusted: HTML sanitized (bluemonday), Markdown rendered by 03/08 pipelines (raw HTML off), `body_html` for email-origin parts sanitized like 04. Attachments: type sniff, executables blocked, malware scan (`scan_status pending`), same as uploads.
- Imported email identities stored `verified = false` (default) so import never grants widget/portal access to history; `email_trust: verified_for_users` is explicit admin attestation (audited) for sources that mark users as verified-by-app (Intercom `role=user` with `external_id`).
- Attribute trust: imported attribute values untrusted for 07 identity-bound agent actions unless admin marks mapping `trusted` (explicit attestation that source value was set by their backend, not by end user; audit `import.attribute_trusted`). Never for leads. Never via MCP without confirm gate (mapping PATCH with `trusted: true` is `write` risk; dashboard shows warning copy).
- Imported messages never sent anywhere (no channel outbound job exists for them: writer bypasses `SendChannelOutbound`).
- AI: imported content enters 07 conversation history and 08 knowledge (published articles) like any data; embeddings for published imported articles spend workspace BYOK key (preview shows estimate; spend cap applies). Prompt-injection rules unchanged (untrusted wrapping).
- Tenancy: every query `workspace_id`; RLS; cross-tenant tests; `import_records` lookups always scoped.
- Audit: import create/update/archive/delete, credentials change, run start/pause/cancel, rollback, mapping bulk change, `email_trust` attestation.
- GDPR: eraser removes `import_records` rows pointing at erased contact and its conversations, writes `import_erasures` tombstones (HMAC of external ids and emails), deletes `import_errors` mentioning external id of erased contact. Export (03) includes imported conversations (normal rows).

## Metering impact

None billable (12 D11). Imported messages carry `metadata.imported = true` (excluded from `IsActivity`); imported conversations carry `source.imported`; no `message.created` dispatched, so `RecordBillableActivity` never runs for import writes. Live activity after import is normal: contact writes live message then workspace replies = one charge (12 fixture 25). Rehome reply to imported conversation (teammate writes first after import, no live inbound) = D7 outbound rule: no charge until contact replies. Delta-imported contact messages are imported (not qualifying inbound); a live teammate reply after only delta-imported inbound is not billed (documented trade-off, edge case E52). Not activity in fair-use guard (no `conversation.created`). New 12 replay fixture `import_never_billed` (Required from 12).

## Dependencies on other specs

| Spec | Uses |
|---|---|
| 00-foundation | KeyManager, Filesystem, httpclient SSRF, River, ratelimit (Valkey), events/realtime, Entitlements, RetentionGuard, ContactEraser registry, AuditSink via audit log, ids (`ids.NewV7At(t)` for source-time UUIDv7) |
| 00-accounts | permissions catalog, roles, audit log, `password.confirm`, members/teams for mapping |
| 03 | conversations, messages, contacts, contact_identities, companies, tags, attachments, saved replies, inboxes; import write helpers; `ChannelRehomer`; resource `imported` block; unread rule |
| 04 | email connection used by rehome (`Channel.Send`); no contact mail from import |
| 06, 07, 10 | guards ignore `metadata.imported` (defense in depth); 07 `contacts.trusted_attribute_keys` (core column owned by 07 migration) for trusted attribute mappings |
| 08 | help centers, collections, articles, versions (publish path), assets, redirects, crawl support for `public_site` |
| 09 | MCP registry + confirm gate, CLI client mode (`migrate`) |
| 12 | D11, entitlement cloud driver catalog keys, replay fixture |
| 13 | changelog entries import helper (backdated, `announce false`) |
| 14 | feedback import helpers (posts, votes, comments), statuses/boards for mapping; feedback CSV stays in 14 |
| 01 | `window.Intercom` shim (Required from 01) |

## Required from other specs (exact edits; orchestrator applies or forwards)

### 00-foundation
1. `data-model.md` ownership map: row `imports, import_runs, import_run_stages, import_mappings, import_records, import_errors, import_files, import_erasures | 16-importers`.
2. `data-model.md` prefix registry: `imp_` imports (16), `impr_` import_runs (16), `impm_` import_mappings (16), `impf_` import_files (16); code `app/models/public_ids.go`.
3. `data-model.md` migration ranges: add `16 importers 001601-001699`. "Core table changes" jsonb row: `conversations.source.imported`, `messages.metadata.imported` (bool) + `metadata.import`, `channel_connections.type` registry value `import` | 16.
4. `events-and-realtime.md` "Events added by feature specs": row 16: `import.created, import.updated, import.deleted, import.started, import.progressed (realtime only, coalesced 2 s), import.paused, import.resumed, import.completed (only webhook=yes event of 16), import.failed, import.cancelled, import.rolled_back`. Add rule line: "Import writers (16) dispatch no per-record events; lifecycle events only." `member:` channel row "import completion" already present.
5. `api.md` endpoint ownership row: `import sources, imports, import runs, mappings, records, files | 16-importers`. Permission note: `imports:read` (settings:read group), `imports:manage`, `imports:rollback` (settings:write group), owner + admin.
6. `contracts.md`: domain contracts index row `ImportSource | import_source.go | 16 | 16 drivers`; contact eraser list `16: import_records rows, import_errors mentions, tombstones in import_erasures (EraseImportContactData)`; entitlement catalog rows (4 keys above); RetentionGuard consumers add `16 rollback`.
7. `framework.md` / framework: built: `ids.NewV7At(t time.Time)`, `(*database.Tx).CopyFrom(ctx, table, columns, rows)` (pgx CopyFrom; target = temp staging table, RLS blocks COPY into tenant tables for the app role), `KeyManager.MAC(ctx, scope, data)` (Tink HMAC-SHA256, MAC keys never rotated automatically).
8. `security.md` Outbound HTTP "Used by" list: add `importers (16): fixed provider hosts, public_site crawl`.

### 00-accounts
1. `app/auth/permissions.go` catalog: `imports:read` (group `settings:read`), `imports:manage`, `imports:rollback` (group `settings:write`); `roles.go`: owner, admin all three; member none.
2. Audit action names: `import.created`, `import.updated`, `import.credentials_changed`, `import.archived`, `import.deleted`, `import_run.started`, `import_run.paused`, `import_run.cancelled`, `import_run.rollback_started`, `import.email_trust_attested`, `import.attribute_trusted`.

### 03-shared-inbox
1. README Out row "Importers and `imported` messages | post-MVP" becomes "launch: 16-importers (in)". D11 row: import path sets `source.imported` + `metadata.imported`.
2. Import write helpers in `app/actions/conversations/importing/` (03 owns, 16 calls; no events, no outbound, no billing listeners): `ImportConversations(ctx, tx, []ImportedConversationRow)` (number block allocation via one `UPDATE workspaces ... RETURNING`, preserved timestamps, counters `message_count`, `first_customer_message_at`, `last_message_at`, `closed_at`, `waiting_since = null`, `ai_state = 'off'`, `is_test = false`), `ImportMessages` (`tx.CopyFrom` into temp staging table + `INSERT ... SELECT ... ON CONFLICT (workspace_id, dedupe_key) DO NOTHING`; no direct COPY into RLS tenant tables, author_id null allowed for `member`/`agent` when `metadata.imported`), `ImportContacts` (match order `external_id`, then verified email identity, then exactly one contact with same email; identities `verified = false` unless attested; `trusted_attribute_keys` appended only for keys passed in `TrustedKeys` (from mappings `trusted = true`) on `kind user` rows where helper wrote the value), `ImportCompanies` (match `external_id` then `domain`), `ImportTags`, `ImportSavedReplies`, `AttachImportedFile`.
3. Optional channel interface `contracts.ChannelRehomer` (`Rehome(ctx, tx, conv) (*models.ChannelConnection, error)`): `ReplyToConversation` calls it when conversation's connection type implements it; updates `channel_connection_id` in same tx (`conversation.updated`), then validates reply against new connection. 422 `import_channel_cannot_send` when no email connection or contact has no email.
4. Unread rule: imported messages (`metadata.imported`) never count as unread; conversation with only imported messages not unread for any member.
5. Resources: conversation `imported` block (from `source.imported`); message author for `metadata.import.author.kind = former_teammate|bot|unknown` rendered `{object: "former_teammate", name}` (no member link); filters `filter[imported]`, `filter[import_id]`; search operator `imported:<external id or number>` (search.md).
6. `channel_connections.type` registry adds `import` (16 driver); channel settings list hides type `import` unless `filter[type]=import`.
7. Listener defense: `SendChannelOutbound` and 03 auto-assign skip messages with `metadata.imported`.

### 04-email-channel
1. README Out row "Importing historical mailbox | launch: importers": state "16 imports helpdesk history (Zendesk, Help Scout, Intercom email conversations); raw mailbox (IMAP/mbox) import later (`imap-adapter`)".
2. Rehome target: `Channel.Send` for rehomed imported conversation uses inbox's active email address; first outbound has no `In-Reply-To` (old thread lives at old provider); subject `Re: <conversation subject>`.

### 06-notifications
1. `SendMemberNotifications` guard: skip events whose message has `metadata.imported` (defense; 16 never dispatches them).

### 07-dimmi-agent
1. `actions.md` rows (foundation `data-model.md` row and "16-importers" row) say imports write attributes via trusted path: change to "16 imports never trusted by default; only attribute mappings admin marks `trusted` add keys to `trusted_attribute_keys` (user contacts only)".
2. `TriggerDimmiAgent` guard: never run for `metadata.imported` messages; imported conversations start `ai_state = 'off'`.
3. Prompt history: imported messages included as history (author labels `former teammate`, `bot`); imported bot messages (author_type `agent`, author_id null) never counted in `ai_reply_count` or spend.

### 08-help-center
1. Out row "Importers from Intercom, Zendesk, Help Scout (keep URLs) | launch: importers" becomes in-launch via 16.
2. Import write helpers in `app/actions/helpcenters/importing/` (08 owns): `ImportCollection` (two levels; deeper flattened `A / B`), `ImportArticle` (working copy + optional publish through normal `PublishArticle` path in same tx when `options.articles.publish = preserve`, `first_published_at`/`published_at` preserved, `source_kind` provenance `import`, `last_edited_by_type system`), `ImportAsset` (dedupe sha256 via `app/support/images`), `ImportRedirect` (source `import`, refuses reserved provider paths with error code `redirect_path_reserved`, existing path same target = no-op).
3. Rule "Nothing AI-generated or imported ever auto-publishes": amend to "... except 16 import run where human explicitly set `articles.publish: preserve` (session/app, or MCP after confirm gate); audit logged".
4. Expose `app/support/crawl` sitemap/robots/politeness and `framework/dom` extract helpers for `public_site` driver (no duplicate crawler).
5. Fix stale reference: `help_center_assets` note "also 16 `engage_assets`" should read 15.

### 09-agent-native
1. Toolset registry: add `imports` (6 tools, risks above). Confirm gate: `dimmi_rollback_import` destructive; `dimmi_run_import` destructive when publish preserve.
2. README Out row "`npx dimmi migrate intercom` | launch: importers (owned there)": keep, point to 16 `migrate_command.go`; `cli.md` command list adds `migrate <source>` and `imports ...`; hidden-input secret prompt helper (`app/client/prompt`).
3. Edge 141 (Intercom script detected by `dimmi init`): summary suggests `npx dimmi migrate intercom` and shim.

### 10-webhooks
1. Catalog registry entry `import.completed` (object `import_run`, fixture). No other 16 events subscribable.
2. `QueueWebhookDeliveries` guard: skip `message.created`/`conversation.created` whose subject is imported (defense).

### 12-cloud-billing
1. D11 row: replace "Importer post-MVP (03 README Out)" with "Importer 16 (launch)". Replay fixture `import_never_billed`: full import of 1,000 conversations + delta + live inbound + reply = exactly one charge (live).
2. Entitlement cloud catalog: `imports.concurrent_runs` 2, `imports.records_monthly` 1,000,000, `imports.attachment_bytes_monthly` 100 GB, `imports.public_site_pages` 5,000.

### 13-changelog
1. Out row "Importers from Beamer, Headway, Featurebase, Canny": Featurebase, Canny, Productlane in 16 launch; Beamer, Headway later (`more-import-sources`).
2. Write helper `changelog.ImportEntry` (backdated `published_at`, `source_kind import`, `announce false`, `notify_subscribers false`, no mailing, no widget badge, labels mapped/created via `changelog.EnsureLabel`, cover via `ImportAsset`); no `changelog_entry.*` events. Rollback `changelog.DeleteImportedEntries`. Applied in 13 README "Import helper (for 16)"; label cap conflict `label_cap_reached`.

### 14-feedback-roadmap
1. Out row "Native API importers (Canny API ..., Featurebase API, Productboard) | post-MVP" becomes "Canny, Featurebase, Productlane API: 16 launch; Productboard later". Open question 7 note: CSV stays in 14 (decision in 16).
2. Write helpers `feedback.UpsertImportedPost` (`import_key` = `<connection_key>:<id>`, `import_id` null, `source import`, `imported_vote_count` for votes without identity), `UpsertImportedVote` (trust `imported`, source `import`, original time), `UpsertImportedComment`, `UpsertImportedRequest` (E112), rollback `DeleteImportedPosts`; no events, no notifications; `subscribe_voters` honored silently. Applied in 14 README "Import write helpers (for 16)". Status creation respects `feedback.statuses` cap (mapping conflict `status_cap_reached` otherwise). Shared `support/csvsniff` used by 16 csv driver.

### 01-messenger-widget
1. Out row "`window.Intercom` shim | post-MVP": becomes launch (roadmap 17 bar). Loader installs `window.Intercom` function when `widget_settings.intercom_shim` (new bool, default false; 01 migration) or SDK option `intercomShim: true`: maps Intercom calls to sdk-contract verbs (`boot` -> `boot({appId, userId, visitor})` with `user_id` -> unverified `claimed_user_id` (02 R5), `name`/`email` -> `visitor` hints; `company`, custom attributes ignored + one `console.info` (browser never writes attributes, 02 R8); `update` -> `update({})` / `identify()` on `user_id` change; `shutdown` -> `logout()`; `show`/`hide` -> `open()`/`close()`; `showMessages` -> `show('messages')`; `showNewMessage(text)` -> `startConversation(text)`; `showArticle(id)` -> `show('help', {article})` (via `imports.records` lookup endpoint on widget API: Intercom article id -> `art_`); `onShow`/`onHide`/`onUnreadCountChange` -> `on('open'|'close'|'unread_count_changed')`; `trackEvent` (no-op + console note); `getVisitorId` -> `visitorId`); unknown methods no-op. Verified identity only through Dimmi JWT identity source. Mapping table owned by 16 (sources.md "Intercom shim").
2. Widget API `GET /widget/v1/help/articles?filter[intercom_id]=...` (or 16-provided resolver) for `showArticle`.

### specs/README.md
1. Folder table row: `[16-importers](16-importers/README.md) | 17 | imports from Intercom, Zendesk, Help Scout, Featurebase, Productlane, Canny, public help center URL, CSV; preview, mapping, resumable, delta, rollback, redirects, never billed | 140 | 38`.
2. Build order: M4 parallel after 03, 08, 13, 14, 09 CLI core: "16 importers" demo "`npx dimmi migrate intercom` from fixture tenant: preview, run, redirects, rollback".
3. Key decisions: "Importers: per-record events suppressed, lifecycle events only; idempotent by `(connection_key, object, external_id)`; never billed."

## Open questions (recommended default in bold)

1. Accept Intercom `user_hash` (HMAC) through shim with imported Intercom identity secret? **No at launch**; shim boots unverified; docs show JWT identity source migration (02). Revisit if switchers complain.
2. Default `conversations.status_mode`: **`close_all`** (history only; open items finish in old tool until cutover; delta keeps syncing). `preserve` available.
3. Default `contacts.only_with_conversations` for Intercom leads: **false** but preview highlights lead count; Intercom workspaces often hold 100k+ anonymous leads: option `skip_anonymous_leads` default **true** (leads without email and without conversations skipped).
4. Redirect fuzzy match for stale slugs (`/en/articles/123-old-slug`): **exact + id-only variant rows at launch**; ask 08 for numeric-id-prefix fallback later.
5. Cloud `imports.records_monthly` 1,000,000: **yes**, staff override for bigger switches (free white-glove).
