# Dimmi MVP specs (backend)

Date: 2026-11-04. Style: caveman. Build plan for parallel agents: [BUILD-TRACKS.md](BUILD-TRACKS.md). 00-accounts, 01, 02, 03, 04, 06, 18, 20, 21 specify screens too (BoardUI); other folders stay backend first. Monolith serves web app.

Stack: Go 1.26 single binary, Laravel code structure and conventions, PostgreSQL 18 (only database of record), Valkey (required, rebuildable state only). License MIT. See `research/tech-stack.md`, `research/roadmap.md`, `research/pricing.md`, `research/build-vs-reuse.md`.

## Read order

1. [00-conventions](00-conventions/README.md): layout, naming, request lifecycle, Laravel rules, deviations, [framework.md](00-conventions/framework.md) (framework/app split, package catalog, import rules), [testing.md](00-conventions/testing.md), [code-style.md](00-conventions/code-style.md). Binding.
2. [00-foundation](00-foundation/README.md): core data model, ID prefix registry, API conventions, REST rules + old-to-new route map ([rest.md](00-foundation/rest.md)), event catalog, realtime (embedded centrifuge), security, canonical shared contracts ([contracts.md](00-foundation/contracts.md)), extensibility seams ([extensibility.md](00-foundation/extensibility.md)).
3. Feature folders below. Each has `README.md` (scope, data model, code map, API, events, jobs, config, permissions, budgets), `edge-cases.md` (decided behaviors), `tasks.md` (ordered checklist, demo script), plus extra files when large.

Conflict rule: `00-foundation/contracts.md` wins, then `00-foundation/rest.md`, then other `00-foundation/*`, then `00-conventions`, then owner feature spec.

## Folders

Counts: edge cases = numbered items in `edge-cases.md` (lettered items like `164a` count); tasks = `- [ ]` lines in `tasks.md`. Recount: `grep -cE '^[0-9]+[a-z]?\. ' edge-cases.md`, `grep -cE '^\s*- \[ \]' tasks.md`. Snapshot: 2026-11-02 last cleanup + verification pass; counts below are stale for folders changed 2026-11-04 (recount before relying).

| Folder | Roadmap item | What | Edge cases | Tasks |
|---|---|---|---|---|
| [00-accounts](00-accounts/README.md) | prerequisite | signup, login, OAuth (GitHub, Google), magic link, auth factors, sessions, workspaces, org layer, members, teams, invitations, tokens, permissions, audit log | 273 | 79 |
| [01-messenger-widget](01-messenger-widget/README.md) | 1 | widget API, settings, visitor sessions, loader serving, SDK contract (`@dimmi/nextjs`, `@dimmi/react`, browser, `@dimmi/server` + raw snippets; `npx dimmi init` golden path), realtime connect tokens, agent action confirmation card, `window.Intercom` shim | 151 | 53 |
| [02-widget-identity](02-widget-identity/README.md) | 2 | JWT identity verification, secrets rotation, merge rules, identity source helpers, trusted attribute keys, server snippets | 91 | 18 |
| [03-shared-inbox](03-shared-inbox/README.md) | 3 | conversations, messages, notes, contacts, companies, tags, saved replies, search, Channel contract, import write helpers | 262 | 65 |
| [04-email-channel](04-email-channel/README.md) | 4 | inbound via SES receiving (IMAP and provider inbound later), outbound via one instance provider (system/customer streams), domains, threading, auto-reply detection, bounces, contact mail for 13-15 | 198 | 62 |
| [05-chat-app-channels](05-chat-app-channels/README.md) | 5 | Slack, Discord, Telegram (private chats), WhatsApp | 172 | 36 |
| [06-notifications](06-notifications/README.md) | 6 | member notifications: email, web push, Slack, in-app (Discord later); system-stream suppression | 136 | 38 |
| [07-dimmi-agent](07-dimmi-agent/README.md) | 7, 18 | BYOK AI agent: providers (official SDKs), pipeline, citations, handoff, spend cap, playground; agent actions ([actions.md](07-dimmi-agent/actions.md): HTTP + MCP tools, OpenAPI import, confirmation, approval) | 233 | 59 |
| [08-help-center](08-help-center/README.md) | 8, 16 | help centers, articles, SSR, custom domains, knowledge ingestion + retrieval, cold start, review queue, llms.txt, public MCP; shared GitHub layer + self-updating docs ([docs-sync.md](08-help-center/docs-sync.md): PR-driven suggestions, freshness) | 270 | 77 |
| [09-agent-native](09-agent-native/README.md) | 9 | OAuth 2.1 server, MCP server (always-confirm gate), CLI, `dimmi init`, `dimmi migrate`, install by prompt | 265 | 75 |
| [10-webhooks](10-webhooks/README.md) | 10 | outbound webhooks, Standard Webhooks signatures, retries, delivery log | 91 | 22 |
| [11-self-host](11-self-host/README.md) | 11 | packaging (app + Postgres + Valkey): one image, compose, bare binary `install.sh`, one-click deploys, upgrades, backups, doctor | 140 | 43 |
| [12-cloud-billing](12-cloud-billing/README.md) | 12 | metering rules D1-D15 as code, ledger, spend caps, Stripe invoicing, entitlements cloud driver | 126 | 45 |
| [13-changelog](13-changelog/README.md) | 13 | changelog on help center host (`/changelog`), RSS, widget What's new, reactions, double opt-in email subscribers, GitHub/AI/MCP drafts, knowledge ingestion | 175 | 49 |
| [14-feedback-roadmap](14-feedback-roadmap/README.md) | 14 | feedback boards, votes, requests from conversations, public roadmap portal, AI capture + duplicates, notify-on-ship loop with 13, CSV import | 152 | 54 |
| [15-announcements-surveys](15-announcements-surveys/README.md) | 15 | in-app announcements (banner, modal, chat), surveys (NPS, CSAT, CES), audiences via 18, CSAT on close (widget + email rating page) ("engage") | 136 | 36 |
| [16-importers](16-importers/README.md) | 17 | imports from Intercom, Zendesk, Help Scout, Featurebase, Productlane, Canny, public help center URL, CSV; preview, mapping, resumable, delta, rollback, redirects, never billed | 140 | 38 |
| [17-workflows](17-workflows/README.md) | rules lite + workflows | one engine for automations; consumes 18 audiences and 03 assignment picker | see folder | see folder |
| [18-audiences](18-audiences/README.md) | launch addition | one rule language, saved audiences, attribute registry, contact and company tags, events, builder (owner approved) | in README | in README |
| [19-cloud-staging](19-cloud-staging/README.md) | launch addition | real AWS staging (draft, awaiting owner) | none | see folder |
| [20-native-sdks](20-native-sdks/README.md) | launch addition | React Native + Expo, iOS, Android, headless hooks on `/widget/v1` | see folder | see folder |
| [21-enterprise-identity](21-enterprise-identity/README.md) | launch addition | sign-in rules, session length, SAML, OIDC, SCIM, IP allowlists | see folder | see folder |
| [22-operator](22-operator/README.md) | launch addition | team's own agent: briefs, triage proposals, workspace questions, changesets; in app and Slack | none | none |
| [23-integrations](23-integrations/README.md) | launch addition | OAuth client connections to other tools (Slack first; GitHub, Linear next): one install per tool account shared by every feature, member grants, scope registry | in README | in README |
| [24-broadcasts](24-broadcasts/README.md) | launch addition | one message to many people by email from a connected address, in the Messenger (chat, post) or in each customer's Slack channel; topics, one-click unsubscribe, frequency cap, auto-pause on bounces, replies become conversations, live reports per account (owner approved 2026-11-10) | in README | in README |
| | | **Total features** | **3,011** | **849** (+53 foundation) |

No `15-status-page`: status page removed from product (PO); announcements/surveys folder renamed `16-` -> `15-announcements-surveys` (migration block `0015NN`).

## Build order

For work in flight use [BUILD-TRACKS.md](BUILD-TRACKS.md): it overrides this table for 00-accounts, 01, 02, 03, 04, 06, 18, 20, 21. Dependencies decide order. After milestone 3, tracks run in parallel.

| Milestone | Specs | Unblocks | Demo at end |
|---|---|---|---|
| M0 | 00-foundation (framework layer, Postgres, Valkey, centrifuge) | everything | `docker compose up` (app + Postgres + Valkey), `/ready` green, centrifuge-js client and widget `EventSource` receive broadcast, app under 50 MB idle |
| M1 | 00-accounts | every authenticated API | sign up, verify, create workspace, invite teammate, create token, `GET /api/v1/me` |
| M2 | 03-shared-inbox | all channels, agent, notifications, webhooks, importers | create conversation via API, reply, note, assign, close, search |
| M3 | 01 + 02 | first real channel | widget conversation with verified identity, live in inbox in under 300 ms |
| M4 (parallel) | 04 email | 13, 14, 15 contact mail | email in, reply out, threading, bounce |
| M4 (parallel) | 06 + 10 | | push/Slack notification under 10 s; signed webhook with retry |
| M4 (parallel) | 09 agent-native | 16 CLI wizard | `npx dimmi login`, `dimmi init` in Next.js app, Claude Code replies via MCP |
| M4 (parallel) | 08 help center (incl. shared GitHub layer) | 07, 13, 14, 16 | help center live on custom domain, cold-start drafts reviewed in queue, GitHub repo connected once (one webhook) |
| M4 (parallel, after 08 public site + 04 outbound core; AI drafts after 07) | 13 changelog | 14 notify-on-ship | entry live on help center `/changelog`, RSS validates, widget badge, double opt-in + mailing with undo, GitHub release draft via shared repo webhook, agent cites entry |
| M4 (parallel, after 08 + 04 + 01; AI capture after 07; ship link after 13) | 14 feedback + roadmap | | post from widget, vote, link conversation to post, roadmap page live, status change notifies voters |
| M4 (parallel, after 01 + 03 + 04 + 18 M4) | 15 announcements + surveys | | banner shown to audience, NPS survey answered in widget, CSAT after close by widget and email |
| M4 (parallel, after 03, 08, 13, 14, 09 CLI core) | 16 importers | switch campaign | `npx dimmi migrate intercom` from fixture tenant: preview, run, redirects, rollback; zero side effects (`AssertNoSideEffects`) |
| M5 | 07 Dimmi Agent | | BYOK key, agent answers widget with citations, handoff, cost shown |
| M5 (after 07 core + 01 card + 06) | 07 agent actions | | Stripe template: read subscription inline, cancel via confirmation card, approval by teammate |
| M5 (after 07 `RunTask` + 08 GitHub layer) | 08 docs sync | | merged PR -> suggested article edits in review queue; stale article flagged |
| M5 (parallel) | 05 chat channels | | Telegram in 1 minute, Discord, Slack, WhatsApp |
| M6 | 11 self-host | launch | one command on $5 VPS (app + Postgres + Valkey) under 256 MB idle, backup + restore, upgrade |
| M7 | 12 cloud billing | cloud launch | ledger matches D1-D15 fixtures (incl. `import_never_billed`), cap blocks reply with 423, Stripe invoice |

11 packaging work starts at M0 (CI image, compose) and finishes at M6.

## Key decisions

- Scope: parity with Intercom, Productlane, Featurebase, Ferndesk on what our users switch for, plus PO-requested extras (changelog, feedback + roadmap, announcements + surveys, self-updating docs, importers, agent actions). Audit: `research/scope-audit-4-competitors.md`. Deferred items: `research/roadmap.md` "Later" (specs cite `later: <name>`); post-MVP items cited by name (`post-MVP: <name>`), never number.
- License: MIT (PO). Dependencies MIT-compatible only (MIT, BSD, ISC, Apache-2.0, MPL-2.0 unmodified); no GPL/AGPL.
- Framework layer: `framework/` = Illuminate (wraps every third-party lib, no business words, own migrations, splittable); app zone imports stdlib subset + `framework/*` only; adapter zone (`app/drivers/*`, `app/services/*`) one vendor SDK per dir; four narrow `app/support` exceptions for 07 (`00-conventions/framework.md`).
- Reuse before build: never hand-write what a maintained MIT-compatible library does; port nothing without PO approval; "Reuse check" line in every new spec (`research/build-vs-reuse.md`).
- IDs: UUIDv7 keys, TypeID public IDs (prefix registry in `00-foundation/data-model.md`).
- API: REST only, noun paths, state changes via `PATCH` on fields, bulk via `PATCH` on collection. One API for all callers; guards (`auth:session`, `auth:app`, `auth:user`, `auth`) decide caller class incl. first-party Dimmi mobile app (`00-foundation/rest.md`, `api.md`). CSRF via Go `http.CrossOriginProtection`.
- Extensibility seams: org layer from day one (1:1, hidden), registry enums for driver-backed values, permissions not role names, entitlements catalog (one over-limit code `entitlement_limit_reached`), retention guard, access policy, audit sink, KeyManager scopes (`00-foundation/extensibility.md`).
- Conversation access: team-based, one `VisibleScope` for every list, search, count, channel (03 access.md). Audiences: one rule language owned by 18; 15, 17, 01, 07, 08 consume.
- Realtime: `centrifuge` library embedded (no Pusher protocol, no own protocol server). Dashboard centrifuge-js over WebSocket; widget plain `EventSource` on uni-SSE with 60 s connect token; channels `workspace:`, `conversation:`, `member:`, `widget_contact:`; history + recovery replaces last-event-id resume; ephemeral events never recovered.
- Valkey required in every deployment (PO): realtime broker/presence/history (centrifuge Redis broker), cache, rate limits, internal pubsub, replay/dedupe windows, counters, node registry. Holds only rebuildable state. Unreachable: auth/abuse guards fail closed, rest fail open with metric, `/ready` 503. Postgres keeps everything durable (River queue, sessions, tokens, audit, idempotency keys, ledger, FTS + pgvector).
- Backing services: Dimmi scales only its own process; operating, scaling and failing over Postgres and Valkey is the operator's or provider's job. Dimmi connects correctly to single instances and scaled setups: Postgres writer endpoint, multi-host, session or transaction pooler, read replicas; Valkey standalone, Sentinel, cluster (11 decision 20, operations.md section 11).
- Events: three dispatch phases (InTx abortable, Queued transactional, AfterCommit). Billing gate is InTx.
- Email: one instance-level provider from deploy config (SES, Postmark, Resend, Mailgun, SendGrid, or SMTP relay), two never-mixed streams `system` (members) and `customer` (contacts), inbound via SES receiving, provider webhooks, IMAP, OAuth Google and Microsoft mailboxes; no SMTP receiver, no BYO SMTP per workspace; all contact mail through 04 actions.
- Testing: business logic integration tests only (real Postgres, real Valkey, real App, fresh tenant per test), no mocks of DB/actions/events; framework unit + integration; tests named in plain words (no edge-case ids), every edge case covered; 95% app coverage gate (`00-conventions/testing.md`).
- SDK golden path: `npx dimmi init` writes framework package setup (`@dimmi/nextjs`, `@dimmi/react`, browser package, `@dimmi/server`) + identity source; other stacks get raw snippets (script tag + `data-identity-url`); no JWT code in customer app; identity tokens minted per request, never inlined in HTML (`01-messenger-widget/sdk-contract.md`). More framework packages later (`identity-helper-packages`).
- Widget identity: JWT, default mode `required` for new workspaces (anonymous chat still works), HS256 + RS256/EdDSA, `email_verified` only from explicit claim. Only verified sources mark attributes trusted (`contacts.trusted_attribute_keys`).
- Widget size: loader 3 KB or less, core 35 KB or less gzipped.
- Webhooks: Standard Webhooks headers (`webhook-id`, `webhook-timestamp`, `webhook-signature`), one signer in `framework/webhooks`.
- MCP: follows 2026-07-28 MCP spec (Client ID Metadata Documents, no sessions, input-required results), backward compatible, go-sdk v1.8.0. Central confirm gate: `send`/`destructive` tools always confirm at MVP (send modes later: `agent-send-modes`).
- CLI: same Go codebase built with `-tags cli`, npm `dimmi` ships per-platform binaries.
- Help center: several per workspace (cloud cap 3), Markdown storage, read-only public MCP at launch. Changelog (13) and feedback portal (14) live on help center hosts.
- GitHub: one fine-grained token per workspace (`github_connections`), one webhook per repo (`/webhooks/github/repositories/{ghr}`), shared by 08 knowledge, 08 docs sync, 13 changelog; GitHub App later (`github-app`).
- Self-updating docs: merged PRs drive suggestions in 08 review queue; nothing auto-publishes; customer code never stored or logged; BYOK only.
- Agent actions: identity from verified context only; writes need customer click on Dimmi-rendered card; reads inline; never metered; MCP never executes writes.
- Importers: per-record events suppressed, lifecycle events only; idempotent by `(connection_key, object, external_id)`; resumable, delta, rollback; never billed.
- Knowledge: hybrid FTS + pgvector, keyword fallback when no embeddings key.
- Billing: own ledger, Stripe one-off invoice items (not Stripe meters). Changelog, feedback, announcements, surveys, agent actions, docs sync, imports never metered.
- WhatsApp: Meta bills business directly (Tech Provider model).
- Self-host: `pgvector/pgvector:pg18` + Valkey 8 images, app key auto-generated in images.

## Top risks (from independent review)

| Risk | Mitigation in specs |
|---|---|
| MCP write tools across many specs bypass confirmation | central confirm gate in `app/mcp` middleware, tools declare `risk` (security.md "Agent safety rules") |
| Customer text steers external agents (prompt injection) | MCP `send`/`destructive` tools always confirm at MVP (send modes later: `agent-send-modes`); customer content wrapped untrusted |
| Agent action acts on wrong customer account | identity bound only from verified context + trusted attribute keys; customer click on Dimmi card for writes; DB CHECK blocks unconfirmed live write; approvals for risky actions (07 actions.md) |
| Import floods notifications, webhooks, AI runs, bills | writers dispatch no per-record events; listeners skip `metadata.imported`; `AssertNoSideEffects` test; replay fixture `import_never_billed` (16, 12) |
| Customer source code leaks via docs sync | in-memory only, BYOK key only, never stored/logged/in job args, secret lines redacted (08 docs-sync.md) |
| Anonymous Ask AI / public MCP drains workspace BYOK key | `throttle:public`, anonymous daily AI budget, bot checks, spend cap |
| Agent-only docs containing secrets quoted to customers | agent-only means unlinked, not secret; secret scanner blocks indexing |
| GDPR erase misses PII in feature tables | contact eraser hook registry (contracts.md), every PII-storing spec registers eraser (13, 14, 15 included) |
| Billing gate inside reply transaction slows replies | gate is single indexed upsert, no network; write p95 under 100 ms tested in M7 |
| Driver rows written outside ingest transaction | `AfterPersist` hook inside `ReceiveInboundMessage` transaction |
| Self-host setup race (`CompleteSetup` vs `user:create`) | shared advisory lock + test (11) |
| Bulk contact mail (13, 14, 15) hurts shared provider reputation | customer stream only, double opt-in (13), RFC 8058, per-workspace caps (entitlements), auto-pause per substream |
| Pre-1.0 libraries (centrifuge, River, disgo, certmagic) | pinned, wrapped behind framework contracts, upgrades in dedicated PRs |
| Valkey becomes accidental database of record | rule "Valkey holds only rebuildable state", every key has TTL, flush test in CI |

## Open questions (defaults chosen, confirm)

Answered questions dropped. Folders changed 2026-11-04 list theirs under "Decisions", decided; values below stand where not superseded. Other folders keep "Open questions".

| Question | Default | Spec |
|---|---|---|
| Password minimum length | 10 chars + common list + breach check (cloud); revisit 15 | 00-accounts |
| Service accounts / bot tokens | not at MVP; dedicated "CI" member | 00-accounts |
| Anonymous visitor session | 180 days sliding | 01 |
| Ad-blocker strategy | first-party proxy only | 01 |
| Identified widget session lifetime | 24 h, renewed per page-load handshake (`PATCH /widget/v1/session`) | 02 |
| Spam auto-purge | see 03 safety.md | 03 |
| Raw inbound `.eml` retention | 30 days | 04 |
| Cloud email cap | 200/day unverified domain, 2,000/day verified (entitlement keys) | 04 |
| Reply-To token address on replies | on; per-address toggle | 04 |
| Custom Discord bots per cloud workspace | 3 | 05 |
| `new_conversation` member email | on (solo first users) | 06 |
| Critical auth mail bypass of system suppression | keep, max 3 per address per 24 h | 06 |
| Default AI monthly cap | $25 | 07 |
| Write actions on email ("reply YES") | later (`email-action-confirmations`); email runs verified reads only | 07 actions |
| `requires_verified_contact` default for new read actions | true | 07 actions |
| Teammate approval | any member with `ai_action_calls:approve`; `admin` option per action | 07 actions |
| Connection edits apply to published actions live | yes, `confirm_affects_published` guard + audit | 07 actions |
| Follow-up after write action | LLM follow-up run (template option later) | 07 actions |
| Whole-response exposure when no `response_fields` | allowed with warning | 07 actions |
| Cloud help center base domain | separate registrable domain (cookie isolation) | 08 |
| SVG images in articles | rejected at MVP | 08 |
| Default `public_mcp_enabled` | true | 08 |
| Fine-grained PAT prefill URL | deep link with what GitHub accepts + written steps (verify at build) | 08 docs sync |
| `not_affected` judgment advances freshness baseline | yes | 08 docs sync |
| `push` trigger default | on (PR-only via `--triggers pull_request`) | 08 docs sync |
| Cloud docs sync runs per day | 200 (`docs_sync.runs_daily`), override per customer | 08 docs sync |
| Refresh token expiry | 90-day sliding, no absolute | 09 |
| "AI-assisted" label on replies from AI clients | no (metadata only) | 09 |
| Webhook auto-disable | after 72 h failing | 10 |
| GitHub org | `dimmihq` | 11 |
| Self-host deploy paths | compose (3 containers) + bare binary `install.sh` (installs Postgres + Valkey unless URLs given) + 6 one-click templates; one image; no single-container bundle; no AWS self-host guide | 11 |
| Valkey self-host memory | `maxmemory 64mb`, `noeviction`, AOF `everysec` (nothing durable) | 11 |
| Price bands 2 and 3 | ship price book `2026-11-01`; new version if cost measurement raises | 12 |
| Changelog cloud email recipients | 10,000/month free (`changelog.email.monthly_recipient_cap`), raise on request | 13 |
| Auto-subscribe identified widget users to changelog | no (explicit consent only) | 13 |
| Comments on changelog entries | no at MVP | 13 |
| Changelog results in help center search | yes, labeled; not toggleable | 13 |
| Changelog reaction emoji set | fixed 5, not configurable | 13 |
| Anonymous voting on feedback board | off | 14 |
| Voter names on public portal | hidden | 14 |
| Portal host with several help centers | one portal per workspace on chosen help center | 14 |
| Feedback AI capture | off (BYOK spend) | 14 |
| Feedback status-change undo window | 10 minutes (0-60 per workspace) | 14 |
| Revenue weighting source | company attribute (`mrr`), contact fallback | 14 |
| Max interruptive in-app messages per contact per day | 2 | 15 |
| CSAT channels | widget + email only (chat apps later: `chat-app-csat`) | 15 |
| Contact tags for targeting | yes (18 decision 8) | 18 |
| Intercom `user_hash` through shim | no; shim boots unverified, docs show JWT migration | 16 |
| Default `conversations.status_mode` | `close_all` (`preserve` available) | 16 |
| Intercom anonymous leads | `skip_anonymous_leads` true (no email, no conversations) | 16 |
| Redirect match for stale slugs | exact + id-only variant rows at launch | 16 |
| Cloud `imports.records_monthly` | 1,000,000; staff override for bigger switches | 16 |
