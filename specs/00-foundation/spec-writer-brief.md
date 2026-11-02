# Spec writer brief (shared)

You are writing one MVP feature spec folder for Dimmi (trydimmi.com): open-source Intercom/Fin alternative. Go 1.26 monolith, Laravel code structure and conventions, PostgreSQL 18 only, BYOK AI, per-conversation pricing, self-host in one command. First users: vibe coders, solo devs, early adopters. Goal: world-class, polished MVP. Today 2026-11-01.

READ FIRST (mandatory, all of them):
- /Users/bahdcoder/code/trydimmi.com/specs/00-conventions/README.md  (code layout, naming, request lifecycle, Laravel rules), framework.md (framework/app split, package catalog, import rules), testing.md (integration-only business tests), code-style.md
- /Users/bahdcoder/code/trydimmi.com/specs/00-foundation/README.md, data-model.md, api.md, rest.md (binding), contracts.md (binding), extensibility.md (binding seams), events-and-realtime.md, security.md
- /Users/bahdcoder/code/trydimmi.com/research/build-vs-reuse.md (reuse rule, decided libraries)
- /Users/bahdcoder/code/trydimmi.com/research/roadmap.md (your launch item(s), plus post-MVP list so you know what is OUT of scope; cite post-MVP items by name, e.g. "post-MVP: mobile SDKs", never by number)
- Relevant research files under /Users/bahdcoder/code/trydimmi.com/research/ (competitors/*.md, tech-stack.md, pricing.md) as your feature needs.

SCOPE: BACKEND ONLY. Frontend (React dashboard, Preact widget UI) comes later. Specify every API the frontend will need. Monolith still serves the web app; you may note server-side responsibilities (SSR pages, asset serving) but no UI design. Expose REST API (huma, /api/v1), MCP tools where useful (tools live in app/mcp; name them `dimmi_<verb>_<noun>`), CLI only if your feature owns it.

HARD RULES:
- Reuse before build: before listing any file under `app/support`, `app/services`, `app/drivers/*/internal` or any protocol implementation, add a "Reuse check" line: library searched (Go module path, license, latest version, last push) and why used or not. Never hand-write crypto, protocol servers, provider SDKs, parsers for public standards, or security filters a vetted library provides. App code never imports third-party libs: wrapped by `framework/` or one SDK per adapter-zone dir.
- Tests: business logic = integration tests only (real Postgres, real Valkey, real App); no unit tests with mocks for app code; tests named in plain words describing behavior, never edge-case ids; every edge case still gets a test. Never plan "unit test per action".
- Driver-backed enums are registry enums (`text` + format CHECK + Go registry), state machines strict CHECK. Policies check permissions (`authz.Can`), never role names. Cloud caps are entitlement catalog keys (over limit: `entitlement_limit_reached`).
- Never redefine core tables from data-model.md. If you need columns on a core table, list them under "Core table changes" with migration name.
- Use event names from events-and-realtime.md catalog. New events go under "New events" with realtime/webhook flags.
- Use code paths from conventions (app/actions/<domain>/..., app/http/controllers/api/v1/..., app/jobs/..., database/migrations/2026110100FFNN_... with FF = folder number, database/queries/<table>.sql, etc.). New public id prefixes checked against data-model.md registry + reserved list.
- Every edge case gets a DECIDED behavior, not a question. Put true open questions in a short "Open questions" section with your recommended default.
- Verify third-party API facts (Slack, Discord, Telegram, WhatsApp Cloud API, SES, MCP spec, OAuth RFCs, centrifuge, Stripe, LLM provider APIs) with WebSearch/WebFetch when your spec depends on them. Read-only web; no sign-ups, no form submissions. Never call LLM APIs.
- Only create/modify files inside your own folder(s) under /Users/bahdcoder/code/trydimmi.com/specs/. Do not edit foundation or other features.
- Writing style: caveman (full): drop articles (a/an/the), filler, pleasantries, hedging; fragments OK; keep technical terms, names, numbers exact; never drop not/never/no/only; no invented abbreviations; do not use the → character (ASCII "->" inside code blocks is fine). Tables, code blocks, headings fine.

OUTPUT FILES in your folder:
1. README.md with sections:
   - Goal (2-3 lines) and Why (link roadmap item)
   - World-class bar (concrete, measurable)
   - Scope: In (MVP) / Out (later, with roadmap post-MVP name if any)
   - User stories (workspace member, end customer, developer, AI agent as relevant)
   - Data model: new tables (columns, types, constraints, indexes), core table changes
   - Code map: every file to create, Laravel-style paths (controllers, requests, resources, actions, policies, jobs, events, listeners, mail, notifications, integrations/drivers, mcp tools, commands, migrations, queries, factories, tests)
   - HTTP API: table (method, path, route name, auth + ability, purpose) then request/response JSON examples for key endpoints, error codes. Paths follow `00-foundation/rest.md` (nouns only, state via `PATCH` field, child collections for actions, bulk via `PATCH` on collection, route name = path, one controller per resource). Auth column uses api.md letters (`S`, `U`, `A`, `G`); `S` only with reason (browser redirect/cookie flow)
   - MCP tools (if any) and CLI commands (if any)
   - Events: emitted, consumed, new events
   - Jobs: name, queue, trigger, retries, uniqueness
   - Config keys (config/*.go + env)
   - Permissions matrix (owner/admin/member/token abilities)
   - Performance budgets, observability (metrics/logs), security and privacy notes specific to feature
   - Metering impact (per research/pricing.md section 2 rules D1-D15) if any
   - Dependencies on other specs
2. edge-cases.md: grouped, numbered, each "Case. Behavior." Aim 40+ for big features, 25+ for small. Think hard: concurrency, retries, duplicates, partial failures, provider outages, limits, unicode, timezones, deleted references, permission changes mid-flight, abuse, migrations/upgrades, self-host vs cloud differences.
3. tasks.md: ordered build checklist `- [ ]`, small tasks (each about 1 day), grouped by milestone, each with acceptance criteria and tests; "Done when" section with demo script.
Optional extra files if feature large (e.g. api.md, protocol.md).

FINAL REPLY to orchestrator (max 250 words): files written; new events; core table changes; dependencies on other specs; contradictions you found in foundation docs; open questions.
