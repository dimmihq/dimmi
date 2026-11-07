# 08 Help center and knowledge

Style: caveman. Date 2026-11-01. Owner: 08-help-center. Docs sync added 2026-11-02 (launch item 16: shared GitHub layer, merged-PR drafts, freshness; [docs-sync.md](docs-sync.md)). Scope trim 2026-11-02: changelog custom domain (B12), public MCP changelog tool (B13), review 3-way rebase (B18), portal `.md` llms.txt sections (C10) moved to Out (later). Read with `specs/00-conventions/README.md` and `specs/00-foundation/*`.

Files in folder:

| File | What |
|---|---|
| README.md | this: scope, data model, code map, API table, events, jobs, config, permissions, budgets |
| [knowledge.md](knowledge.md) | `knowledge` package contract (used by 07), Embeddings contract, ingestion pipeline, chunking, hybrid search SQL, re-embedding |
| [review-queue.md](review-queue.md) | suggestions, hunks, stale rule, accept/reject, cold-start generator protocol |
| [docs-sync.md](docs-sync.md) | self-updating docs: shared GitHub layer (connections, repositories, one webhook per repo), merged PR -> screen -> map -> anchored-edit drafts -> review queue; article freshness scan; prompts, JSON, MCP/CLI, cross-spec asks |
| [api.md](api.md) | request/response JSON for key endpoints, public help center routes, widget Help API, MCP tools, error codes |
| [rendering.md](rendering.md) | SSR, Markdown dialect, caching, SEO, llms.txt, `.md` pages, custom domains + TLS, public MCP |
| [edge-cases.md](edge-cases.md) | decided behavior per case |
| [tasks.md](tasks.md) | build checklist |

## Goal

Public help center that is fast, SEO-clean, AI-readable, and fills itself: cold-start drafts from user's coding agent, Git/Markdown/URL import, all through review queue with inline diffs. Same content plus external sources feed one knowledge index (Postgres FTS + pgvector, RRF) that Dimmi Agent (07) retrieves from. Docs keep themselves current: merged GitHub PRs become reviewed article edits (BYOK), weekly freshness scan flags articles gone stale ([docs-sync.md](docs-sync.md)).

## Why

Roadmap launch item 8 (`research/roadmap.md`): "agent useless without docs. Public docs cut tickets, bring SEO. Empty docs is top blocker for new users." Productlane review UI is bar (`research/competitors/productlane.md`, self-updating help center deep dive). Ferndesk/Mintlify/Promptless never auto-publish (`research/landscape-2026-11.md`); same rule here. Roadmap launch item 16 (self-updating docs, audit G3; PO 2026-11-02) extends this spec: Productlane and Ferndesk core pitch.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Article page TTFB, cached, origin | p95 under 50 ms |
| Article page TTFB, cold (cache miss) | p95 under 150 ms (body HTML precomputed at publish) |
| Search (keyword mode) server time | p95 under 100 ms at 100k chunks per workspace and 10M chunks total |
| Search (hybrid) server time excluding provider embed call | p95 under 100 ms; query embedding cached (repeat query adds 0 ms) |
| Publish to searchable (FTS) | same transaction (0 s) |
| Publish to public page | under 2 s on every replica |
| Publish to vector-searchable | p95 under 30 s |
| JavaScript on article page | 0 bytes required. Optional `hc.js` under 5 KB gz for instant search + streamed Ask AI |
| Lighthouse (mobile) article page | 100 performance, 100 SEO, 100 accessibility, 100 best practices (CI check on seeded page) |
| Cold start: repo to drafts in review queue | under 2 minutes for 30 articles via MCP (dominated by user's agent) |
| Review: open suggestion with 50 hunks | API under 100 ms; per-hunk accept under 100 ms |
| Website crawl | 500 pages in under 10 minutes at default 1 req/s/host with concurrency 2 |
| Git sync no-op (SHA unchanged) | under 2 s, zero downloads |
| Custom domain to HTTPS live | under 2 minutes after DNS propagates |
| Every page has `.md` twin, listed in `llms.txt` | 100% of public published articles |
| Nothing AI-generated or imported ever auto-publishes, except 16 import run where human explicitly set `articles.publish: preserve` (session/app, or MCP after 09 confirm gate; audit logged) | enforced in code: no MCP/agent path calls `PublishArticle`; only `ImportArticle` with run option `preserve` (arch test) |
| Merged PR to doc suggestions in review queue | p95 under 3 min (up to 5 drafts; provider-dominated); screened (skip or proceed) p95 under 60 s; webhook ack under 1 s |
| Merged PRs accounted for | 100% get run row (`completed`, `skipped` with reason, `failed`); reconcile poll catches lost deliveries within 6 h |
| Customer code at rest | zero: diff only in job memory, sent only to workspace BYOK key (leak test) |
| Freshness scan | 1,000 articles under 30 s deterministic part; 1 GitHub compare per (repo, distinct verified sha) |

## Scope

### In (MVP)

- Help centers: many per workspace allowed by schema and API; cap = entitlement `help_center.max_per_workspace` (`Entitlements.Limit`, contracts.md; cloud default 3, self-host config 0 = unlimited; staff override per customer, 12). One marked `is_default` (agent default; prefill for new widgets' `help_center_id`; widget Help tab uses widget's own `widget_settings.help_center_id`). Auto-created on workspace create via workspace initializer hook (contracts.md, runs inside `CreateWorkspace` tx, idempotent).
- Collections: two levels (collection, sub-collection). Articles may be uncollected.
- Articles: Markdown canonical storage (CommonMark + GFM + alerts). Statuses `draft`, `published`, `archived`. Soft delete with 30-day trash. Visibility `public`, `agent_only`, plus optional saved audience (18 M7, one audience for humans and dimmi). `agent_only` = hidden from public surfaces, but conversation agent (07) may quote it to customers; never store secrets there (dashboard copy says so; `possible_secret` warning applies).
- Working copy + immutable versions. Publish pointer. Version history, diff any two versions, restore.
- Slugs, automatic 301 redirects on slug/collection change, manual redirects (migration-ready).
- SSR via templ + goldmark from monolith. Built-in theme with brand color, logo, light/dark (CSS only). Pages: home, collection, article, search, ask, 404/410.
- SEO: title, meta description, canonical, sitemap.xml, robots.txt, OpenGraph, Twitter card, JSON-LD (Article, BreadcrumbList, WebSite+SearchAction), `noindex` toggle.
- Hosting: default subdomain `{subdomain}.{help_center.base_domain}`, path mode `{app_url}/hc/{subdomain}`, one custom domain per help center (13 changelog served on same hosts under `/changelog`); TXT ownership + CNAME/A routing, automatic TLS via `framework/customdomain` (certmagic in-process or Caddy on-demand ask endpoint).
- Caching: precomputed body HTML per version, in-process page cache (L1, otter) keyed by `cache_version`, cross-replica bust via `pubsub.PubSub` on Valkey, ETag/304.
- Public search (keyword, hybrid on submit), Ask AI (delegates answer to 07, SSE stream, works without JS).
- Widget Help tab API (`/widget/v1/help/*`) used by 01. SDK opens article by id: `show('help', {article: 'art_...'})` (01 sdk-contract.md; also 15-announcements-surveys CTA `open_article`).
- `llms.txt`, `llms-full.txt`, `.md` for every public page, `Accept: text/markdown` negotiation, `Link rel=alternate`.
- Read-only public MCP endpoint per help center (3 tools; no auth, stateless). Decision and reasoning in rendering.md section "Public MCP".
- Public section providers (`helpcenter.PublicSectionProvider`): other features (13 changelog, 14 feedback portal) plug nav links, home sections, routes, sitemap URLs, llms.txt sections (13 only at MVP), robots rules, search result kinds into help center host without 08 importing their types.
- Knowledge sources (`type` registry enum, P1): website (URL or sitemap crawl), upload (zip of Markdown), git (GitHub repo from shared GitHub layer, public or private via connection token, path filters, push via shared repo webhook). Each source in `knowledge` mode (agent reference, never rendered) or `import` mode (produces article suggestions). Pasted text = agent-only article (no separate snippet entity).
- Sync schedules: manual, hourly (git only), daily, weekly.
- Knowledge index: chunks with FTS + trigram + pgvector embeddings via workspace BYOK key. FTS-only fallback without embeddings key. Re-embed on model change (blue/green).
- `knowledge` package contract for 07 (`knowledge.md`).
- Review queue: suggestions (create, edit, archive), hunks with per-hunk accept/reject, inline word segments, +/- counts, provenance, stale rule (article changed after proposal: refresh or reject, no auto-merge), batches with accept-all.
- Cold-start generator server side: suggestion batch API + MCP tools + MCP prompt for user's local coding agent. Dimmi never reads user repo unless Git source connected.
- Assets: images for articles, logo, favicon, OG image. Public immutable URLs, alt text.
- Analytics data collection only: daily article views, helpful/not helpful, search log (query, result count, surface) for gap report later.
- Shared GitHub layer (owner 08; used by 08 knowledge git, 08 docs sync, 13 changelog): `github_connections` (fine-grained PAT, `kind` registry enum: `token` MVP, `app` reserved), `github_repositories` (one webhook + secret per repo, auto-installed when token has Webhooks write, manual otherwise, polling fallback), route `/webhooks/github/repositories/{ghr}` with handler registry. [docs-sync.md](docs-sync.md) section 4.
- Docs sync (self-updating docs, launch item 16): per (help center, repo, branch) source; triggers merged PR, direct push (debounced), poll/reconcile, manual, redraft, freshness fix; gate (labels, authors, opt-out, revert, caps), redacted in-memory digest, screen (07 `ai.RunTask` classifier, sensitivity strict/balanced/eager), map (explicit `docs:` refs, article source refs, hybrid search), anchored-edit drafts (answer model), one suggestion batch per run (`source_kind commit`), per-hunk rationale + evidence links, skipped-PR log with reasons, cost per run. Never accepts or publishes. [docs-sync.md](docs-sync.md) sections 5-9.
- Article source refs (article -> repo paths, learned on accept, member-editable) and freshness (weekly scan: referenced code changed or deleted, AI claim check on code-flagged articles only (capped), broken links, negative feedback, age; `needs_review` filter; human-only `verified`). [docs-sync.md](docs-sync.md) section 8.
- Import write helpers for 16-importers (launch: importers; Intercom, Zendesk, Help Scout, public help center URL; keep URLs via redirects): `ImportCollection`, `ImportArticle`, `ImportAsset`, `ImportRedirect` in `app/actions/helpcenters/importing/` (section "Import helpers (16)"). `public_site` driver reuses `app/support/crawl` (sitemap, robots, politeness) and `framework/dom` extract (no second crawler).

### Out (later)

| Item | When |
|---|---|
| Drafts from resolved conversations (`conversation-drafts`; `source_kind conversation` reserved) | later, with post-MVP: knowledge-gap report. Merged-PR drafts are In (docs sync) |
| `docs-as-code-pr`: Dimmi opens PR into user's docs repo instead of in-place suggestion | later (needs Contents + Pull requests write); docs-sync.md Out |
| `docs-pr-check`: pre-merge GitHub check listing articles a PR breaks | later (needs Checks write, GitHub App) |
| Knowledge-gap report UI | post-MVP: knowledge-gap report (search log + zero-result data collected now) |
| Translations, multi-locale URLs | post-MVP: auto-translation (URL scheme reserves `/{locale}/` prefix) |
| Private help center (login required on whole site), `private` visibility | later (expand migration adds CHECK value). Per-article and per-collection audience ships in 18 M7 |
| Internal (teammate-only) articles excluded from customer agent | with post-MVP: agent assist |
| PDF, DOCX ingestion; Notion, Confluence, Google Docs sync | later |
| JS rendering for SPA crawl (headless browser) | later; MVP detects and explains |
| GitLab, Bitbucket, GitHub App install flow (`github-app`: `github_connections.kind = app`) | later (`github-app`, with post-MVP: GitHub loop); GitLab, Bitbucket later |
| Custom CSS, custom templates, custom head HTML, analytics scripts | later (CSP) |
| OpenAPI reference pages | later |
| Self-healing screenshots | standout #12 |
| `continuous-verification`: claim-level AI verification of every article on schedule (Up to date / Can't verify tabs) | later (standout #5); MVP freshness AI-checks code-flagged articles only |
| Reranker model step | later (contract leaves `Rerank` hook) |
| CloudFront SaaS Manager domain driver for cloud edge caching | later (contract ready) |
| Real-time collaborative editing (Yjs) | later; MVP uses optimistic lock |
| Article feedback comments (free text) | later; MVP thumbs only |
| SVG uploads | open question (security.md blocks inline SVG) |
| `changelog-custom-domain` (audit B12): `help_center_domains.purpose = changelog`, `changelog_host` flag, changelog served at root of own host | later; MVP changelog lives at `/changelog` on help center hosts (`purpose` column added then) |
| `public-mcp-changelog` (audit B13): provider MCP tools on public MCP (13 `dimmi_list_updates`, `PublicSectionProvider.PublicMCPTools`) | later; MVP `dimmi_search_docs` / `dimmi_get_doc` still cover changelog entries |
| `review-rebase` (audit B18): 3-way rebase (diff3) of pending suggestions onto edited article, conflict hunks, decision carry by fingerprint, import-mode 3-way | later; MVP marks suggestion `stale`, reviewer refreshes or rejects |
| `portal-md-twins` (audit C10): llms.txt `## Roadmap` / `## Feedback` sections (14 `.md` twins) | later; MVP llms.txt providers: 13 `## Changelog` only |

## User stories

Workspace member:
- Write article in Markdown, save draft, preview, publish; see history; restore older version.
- Organize collections and sub-collections, drag order; move article and old URL still works.
- Connect docs site URL, GitHub docs folder, or upload zip; see progress, errors per page, last sync.
- Mark article agent-only so AI knows internal policy but public never sees it.
- Open review queue: see suggestion with +12 / -3, hunks highlighted, accept two hunks, reject one, publish; see who proposed and from which commit.
- Accept 30-article cold-start batch in one click after skimming.
- Point `docs.myapp.com` at Dimmi, see DNS instructions, get HTTPS automatically.
- Pick embedding model; see re-embed progress; search still works during switch.
- Paste GitHub token once, add repo (webhook installs itself), enable docs sync for help center; merge PR "Raise team seat limit" -> 2 minutes later batch "PR #128" in review queue: "Plans and seats" +1 / -1 with note "Seat limit raised to 10 (src/billing/plans.ts)"; accept, publish.
- See skipped PRs with reasons ("internal refactor", "label no-docs") and cost per run.
- Open "Needs review" list: article flagged because `src/billing/plans.ts` changed and AI check quotes outdated sentence; click "Draft fix" or "Mark verified".

End customer:
- Land from Google on fast article page, read without JS, search, ask AI, get answer with links, talk to human from widget.
- Browse Help tab inside widget without leaving app.

Developer (workspace owner, solo dev):
- `claude` with Dimmi MCP: "write our help center from this repo" -> drafts land in review queue, nothing live until accepted.
- CI script pushes `docs/` via REST, publishes via `articles:write` token.
- After merging, asks Claude Code with Dimmi MCP prompt `dimmi_update_docs_for_change`: local agent proposes article edits from local diff (zero server LLM cost), same review queue.
- Customers' AI agents (Cursor, Claude) read docs through `llms.txt` or public MCP.

AI agent (Dimmi Agent, 07):
- `contracts.Knowledge.Search` returns ranked chunks with citation URLs and visibility; works with or without embeddings key.

## Data model

All tables: `id uuid primary key default uuidv7()` unless stated, `workspace_id uuid not null references workspaces(id) on delete cascade`, `created_at`, `updated_at`. Text + CHECK enums. RLS on all in cloud. All tables below (incl. NEW ones and `certmagic_storage`) listed in `data-model.md` ownership map.

New public ID prefixes (catalog `app/models/public_ids.go`, `framework/ids` TypeID encoding): `hc_` help center, `col_` collection, `art_` article (exists), `artv_` version, `sug_` suggestion, `sugb_` suggestion batch, `ks_` knowledge source, `ksr_` sync run, `kd_` knowledge document, `kc_` chunk, `hcd_` help center domain, `rdr_` redirect, `ast_` asset, `ghc_` GitHub connection, `ghr_` GitHub repository, `dss_` docs sync source, `dsr_` docs sync run, `asr_` article source ref (`article_freshness` keyed by article, no prefix).

### help_centers
| Column | Type | Notes |
|---|---|---|
| name | text not null | 1 to 80 |
| subdomain | citext not null | 3 to 40, `^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`, unique globally where deleted_at null; reserved list (`app/support/customdomains/reserved_subdomains.go`: `www`, `app`, `api`, `admin`, `mail`, `status`, `docs`, `help`, `changelog`, `feedback`, `dimmi`...) rejected; default workspace slug |
| locale | text not null default 'en' | BCP 47; drives `<html lang>`, FTS config |
| is_default | boolean not null default false | unique `(workspace_id) where is_default and deleted_at is null` |
| status | text CHECK in ('active','disabled') default 'active' | disabled: public routes 404, API still works |
| settings | jsonb not null default '{}' | typed `models.HelpCenterSettings`: theme (primary_color hex, appearance auto/light/dark), header_links (max 6 {label,url}), footer_text (max 500), social (twitter, github, discord urls), home_title, home_subtitle, show_widget (bool), contact_url, freshness `{enabled true, stale_after_days 180 (0 off, 30..730), ai_check true, ai_checks_per_week 20 (0..100), negative_feedback true}` (docs-sync.md 8.1) |
| logo_asset_id, favicon_asset_id, og_image_asset_id | uuid null references help_center_assets on delete set null | |
| primary_domain_id | uuid null references help_center_domains on delete set null | active custom domain used as canonical |
| indexing_enabled | boolean not null default true | false: robots Disallow all + `X-Robots-Tag: noindex` |
| ask_ai_enabled | boolean not null default true | effective only when 07 agent ready |
| public_mcp_enabled | boolean not null default true | |
| cache_version | bigint not null default 1 | bumped by every public-visible change |
| login_url | text null | https; signed-in reading (18 M7). Added by 18 migration |
| deleted_at | timestamptz | soft delete, purge after 30 days |
Indexes: `(workspace_id, created_at)`, unique `(subdomain) where deleted_at is null`.

### collections
| Column | Type | Notes |
|---|---|---|
| help_center_id | uuid not null references help_centers on delete cascade | |
| parent_id | uuid null references collections on delete restrict | depth max 2: parent must have `parent_id is null` (action check + trigger `collections_depth_check`) |
| name | text not null | 1 to 80 |
| slug | citext not null | `[a-z0-9-]` 1 to 80; unique `(help_center_id, slug) where deleted_at is null` |
| description | text | max 300 |
| icon | text | emoji (1 grapheme) or icon name from allowlist |
| position | integer not null | gaps of 1024; reorder renumbers |
| audience_id | uuid null references audiences on delete restrict | saved only; added by 18 M7 |
| deleted_at | timestamptz | |
Indexes: `(workspace_id, help_center_id, parent_id, position)`.

### articles (working copy)
| Column | Type | Notes |
|---|---|---|
| help_center_id | uuid not null references help_centers on delete cascade | |
| collection_id | uuid null references collections on delete set null | |
| title | text not null | 1 to 150, NFC |
| slug | citext not null | 1 to 120 `[a-z0-9-]`; unique `(help_center_id, slug) where deleted_at is null` |
| description | text | max 300, meta description + excerpt |
| body_markdown | text not null default '' | max 256 KB |
| status | text CHECK in ('draft','published','archived') | `published` = has live version |
| visibility | text CHECK in ('public','agent_only') default 'public' | |
| audience_id | uuid null references audiences on delete restrict | saved only (18 M7); collections get same column. Effective = article AND every collection above. Stays out of `content_hash`: change needs no review or re-embed |
| published_version_id | uuid null references article_versions(id) deferrable initially deferred | live content; null for never-published |
| has_unpublished_changes | boolean not null default false | working copy hash differs from published version hash |
| content_hash | text not null | sha256 of canonical (title, slug, description, body, visibility, collection_id) |
| position | integer not null | inside collection |
| seo | jsonb not null default '{}' | `{title, noindex, og_image_asset_id}` |
| author_member_id | uuid null references workspace_members on delete set null | creator |
| last_edited_by_type, last_edited_by_id | text, uuid | member, token, agent, system |
| lock_version | integer not null default 1 | optimistic lock (`If-Match`) |
| word_count | integer not null default 0 | reading time |
| published_at, first_published_at, archived_at | timestamptz | |
| deleted_at | timestamptz | trash, purge after 30 days |
| title_trgm | generated: `lower(f_unaccent(title))` stored | GIN trgm for admin search |
Indexes: `(workspace_id, help_center_id, collection_id, position)`, `(workspace_id, status, updated_at desc)`, GIN trgm `(title_trgm)`.

### article_versions (immutable)
| Column | Type | Notes |
|---|---|---|
| article_id | uuid not null references articles on delete cascade | |
| number | integer not null | 1..n per article, unique `(article_id, number)` |
| kind | text CHECK in ('published','snapshot','restored','suggestion') | |
| title, slug, description, body_markdown | text | copy |
| body_html | text not null | goldmark + sanitizer output, precomputed |
| toc | jsonb not null default '[]' | `[{level, text, anchor}]` H2/H3 |
| visibility, collection_id | | copy |
| content_hash | text not null | |
| created_by_type, created_by_id | text, uuid | |
| suggestion_id | uuid null references article_suggestions on delete set null | provenance |
| renderer_version | smallint not null | rerender job when renderer changes |
Indexes: `(workspace_id, article_id, number desc)`.
Rows never updated except `body_html`/`toc`/`renderer_version` by `RerenderArticleVersions` job.

### article_suggestions
See review-queue.md for semantics.
| Column | Type | Notes |
|---|---|---|
| help_center_id | uuid not null | |
| batch_id | uuid null references article_suggestion_batches on delete set null | |
| kind | text CHECK in ('create','edit','archive') | |
| article_id | uuid null references articles on delete cascade | required for edit/archive; set on create after accept |
| base_version_id | uuid null references article_versions on delete set null | version proposer read (edit) |
| base_markdown | text | snapshot of base body (survives version prune) |
| proposed_title, proposed_slug, proposed_description | text | null = unchanged (edit) |
| proposed_markdown | text | max 256 KB; null for archive |
| proposed_collection_path | text | create: `"Billing/Refunds"`; resolved or created on accept |
| proposed_visibility | text CHECK in ('public','agent_only') | |
| summary | text | max 500, why change (from proposer) |
| hunks | jsonb not null default '[]' | diff of `base_markdown` (working copy at proposal or refresh) vs proposed, see review-queue.md |
| additions, deletions | integer not null default 0 | line counts across hunks |
| status | text CHECK in ('pending','accepted','partially_accepted','rejected','superseded') | |
| source_kind | text CHECK in ('agent','import','member','api','commit','conversation') | MVP producers: agent, import, member, api, commit (docs sync); `conversation` reserved (later `conversation-drafts`) |
| anchored_edits | jsonb null | docs sync only: `[{find, replace, why, evidence: [{path, line}]}]` max 12; enables `PATCH {base: "current"}` reapply; cleared when reviewer edits proposal (docs-sync.md 7) |
| source_ref | jsonb not null default '{}' | `{agent:"claude-code", client_id, repo, commit_sha, paths[], knowledge_source_id, knowledge_document_id, conversation_id, url}` |
| source_url | text | provenance link shown in UI |
| dedupe_key | text | unique `(workspace_id, dedupe_key) where status='pending'`; same proposal resubmitted replaces pending row |
| proposed_by_type, proposed_by_id | text, uuid | member, token, oauth_client, system |
| reviewed_by_member_id | uuid null references workspace_members on delete set null | |
| reviewed_at | timestamptz | |
| review_note | text | max 1000 |
| result_version_id | uuid null references article_versions | version created on accept |
Indexes: `(workspace_id, status, created_at desc)`, `(workspace_id, article_id, status)`, `(workspace_id, batch_id)`.

### article_suggestion_batches (NEW)
`id, workspace_id, help_center_id, title text (max 150), source_kind, source_ref jsonb, total int, pending int, accepted int, rejected int, created_by_type, created_by_id, created_at, updated_at`. Counters maintained in same tx as suggestion status changes.

### help_center_domains (NEW)
| Column | Type | Notes |
|---|---|---|
| help_center_id | uuid not null references help_centers on delete cascade | |
| hostname | citext not null | lowercase IDNA A-label (punycode), max 253; unique `(hostname) where status <> 'removed'`; claim check `customdomains.Claimed` also covers `widget_domains` (01 hosted-and-domains.md) and rejects platform hosts |
| status | text CHECK in ('pending','active','failed','removed') | |
| verification_token | text not null | 32 random bytes base32; TXT value `dimmi-verify=<token>` at `_dimmi-verify.<hostname>` |
| ownership_verified_at, routing_verified_at, tls_issued_at | timestamptz | |
| tls_status | text CHECK in ('none','pending','issued','failed') default 'none' | |
| last_checked_at, next_check_at | timestamptz | |
| failure_count | integer not null default 0 | consecutive check failures |
| last_error | text | human readable, from enum `domain_error_code` in `last_error_code` |
| last_error_code | text | `txt_missing`, `txt_mismatch`, `cname_missing`, `cname_wrong_target`, `a_wrong_ip`, `caa_blocks`, `acme_failed`, `rate_limited` |
Indexes: `(workspace_id, help_center_id)`, `(next_check_at) where status in ('pending','active')` (scheduler, non-tenant scan by system job with RLS bypass role; documented exception). Verification mechanics (normalize, TXT/CNAME/A/CAA checks, HTTP probe, backoff) in `framework/customdomain`; rows, claims and TLS authorization in app (`app/support/customdomains`).

### help_center_redirects (NEW)
`id, workspace_id, help_center_id, from_path text not null (normalized: leading '/', no trailing '/', percent-decoded then re-encoded, max 512), target_type text CHECK in ('article','collection','url'), target_article_id uuid null references articles on delete cascade, target_collection_id uuid null references collections on delete cascade, target_url text (https only), source text CHECK in ('slug_change','manual','import'), hits bigint default 0, last_hit_at`. Unique `(help_center_id, from_path)`. CHECK exactly one target.

### help_center_assets (NEW)
`id, workspace_id, help_center_id, disk text, path text, filename text, content_type text CHECK in ('image/png','image/jpeg','image/gif','image/webp','image/x-icon') (AVIF refused: `framework/image` `ErrUnsupportedFormat`, pure-Go decoders +4 MB; browser-side conversion later if needed; WebP and ICO stored as PNG after re-encode, ICO = largest icon), size_bytes bigint, sha256 text, width int, height int, alt_text text (max 300), uploaded_by_type (member, token, system; `system` = generated, e.g. 13 OG PNGs), uploaded_by_id, referenced_at timestamptz (first time a version, settings, or a registered referencer (13 changelog entries) referenced it), scan_status text CHECK in ('pending','clean','infected','skipped'), created_at, updated_at`. Unique `(help_center_id, sha256)` (dedupe). Processing (sniff, dims, bomb guard, EXIF orientation applied then metadata strip, `a.Scanner` malware scan) in shared `app/support/images` (also 15 `engage_assets`) on `framework/image`. In-use check (`asset_in_use` 409) asks every registered `helpcenter.AssetReferencer` (08 article versions + settings; 13 `changelog_entries.cover_asset_id`, `og_image_asset_id`, body references). Not stored in core `attachments` (that table purges unattached rows after 24 hours and serves only signed URLs; help center images must be public and long-lived).

### knowledge_settings (NEW, one row per workspace)
| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid primary key | |
| text_search_config | regconfig not null default 'english' | from workspace locale: en english, de german, fr french, es spanish, pt portuguese, it italian, nl dutch, sv swedish, da danish, fi finnish, nb norwegian, ru russian, tr turkish; else simple |
| embedding_key_id | uuid null | references 07 `ai_provider_keys(id) on delete set null` |
| embedding_model | text | provider model id |
| embedding_dims | smallint | |
| active_model_key | text | `<provider>:<model>@<dims>` used for query + search; null = FTS only |
| pending_model_key | text | re-embed target; null when none |
| reembed_total, reembed_done, reembed_failed | integer default 0 | progress |
| reembed_started_at | timestamptz | |
| chunk_count | integer not null default 0 | maintained by indexer; chooses exact vs ANN |
| embeddings_status | text CHECK in ('none','ready','building','degraded') | degraded = key missing/invalid, FTS only |
| embeddings_error | text | |

### knowledge_sources
| Column | Type | Notes |
|---|---|---|
| type | text not null CHECK (type ~ '^[a-z][a-z0-9_]{1,39}$') | registry enum (P1, extensibility.md): Go registry `app/integrations/knowledge/sources.go`, MVP values `website`, `upload`, `git`; request rule + action validate against registry |
| mode | text CHECK in ('knowledge','import') | import needs `help_center_id` |
| help_center_id | uuid null references help_centers on delete cascade | import target; knowledge mode: optional scoping tag |
| target_collection_id | uuid null references collections on delete set null | import default collection |
| name | text not null | 1 to 80 |
| config | jsonb not null | per type, see knowledge.md |
| github_repository_id | uuid null references github_repositories on delete restrict | required for type `git` (CHECK `type <> 'git' or github_repository_id is not null`); token + webhook live on shared GitHub layer. Column added by migration 0823 (table after 0811) |
| credentials_encrypted | bytea | reserved for future source types needing own secrets (none at MVP; git uses GitHub layer) |
| citable | boolean not null | website default true; upload/git default false unless `config.public_url_template` |
| status | text CHECK in ('idle','queued','syncing','error','paused') | |
| schedule | text CHECK in ('manual','hourly','daily','weekly') default 'daily' | hourly only git |
| next_sync_at, last_synced_at, last_success_at | timestamptz | |
| last_error_code, last_error | text | |
| last_commit_sha | text | git |
| document_count, chunk_count | integer not null default 0 | |
| deleted_at | timestamptz | delete removes documents/chunks via job |
Indexes: `(workspace_id, created_at)`, `(next_sync_at) where status in ('idle','error') and schedule <> 'manual' and deleted_at is null` (scheduler, system scan).

### knowledge_sync_runs (NEW)
`id, workspace_id, source_id references knowledge_sources on delete cascade, trigger text CHECK in ('create','manual','schedule','webhook','retry'), status text CHECK in ('queued','running','succeeded','partial','failed','cancelled'), commit_sha text, stats jsonb ({discovered, fetched, unchanged, created, updated, removed, failed, skipped_robots, skipped_js, bytes, tokens_estimated}), error_code text, error text, started_at, finished_at, created_at, updated_at`. Keep last 50 per source (prune job).

### knowledge_documents
| Column | Type | Notes |
|---|---|---|
| source_id | uuid null references knowledge_sources on delete cascade | exactly one of `source_id`, `article_id`, `changelog_entry_id` (CHECK) |
| article_id | uuid null references articles on delete cascade | unique where not null |
| changelog_entry_id | uuid null references changelog_entries on delete cascade | unique where not null. Added by 13 migration `20261101001314_add_changelog_entry_id_to_knowledge_documents_table.sql` (FK needs 13 table; same migration swaps the one-of CHECK to three columns). Chunk `SourceType changelog`, title `Changelog: {title} ({date})` |
| help_center_id | uuid null | scoping filter for search |
| external_key | text | canonical URL (website) or repo path (git/upload); unique `(source_id, external_key)` |
| url | text | citation URL; null when not citable |
| title | text not null | |
| content_markdown | text not null | normalized Markdown, max 512 KB (truncated with flag) |
| content_hash | text not null | sha256(content_markdown + title) |
| visibility | text CHECK in ('public','agent_only') | article: mirror; changelog entry: `public` (only published entries indexed); source: `public` if citable else `agent_only` |
| status | text CHECK in ('pending','active','excluded','error','removed') | pending = discovered, not fetched |
| excluded_by | text CHECK in ('member','robots','pattern','js_rendered','duplicate','too_large','unsupported_type') | |
| http_etag, http_last_modified | text | conditional refetch |
| error_code, error | text | |
| fetched_at, indexed_at | timestamptz | |
| linked_article_id | uuid null references articles on delete set null | import mode: article created from this doc |
| imported_hash | text | import mode: content hash last turned into accepted article |
| last_seen_run_id | uuid | removal detection |
| title_trgm | generated `lower(f_unaccent(title))` | trigram fallback |
Indexes: `(workspace_id, source_id, status)`, GIN trgm `(title_trgm)`.

### knowledge_chunks
| Column | Type | Notes |
|---|---|---|
| document_id | uuid not null references knowledge_documents on delete cascade | |
| ordinal | integer not null | unique `(document_id, ordinal)` |
| heading_path | text | `"Billing > Refunds"` (stored joined; `KnowledgeChunk.HeadingPath` []string split on ` > `) |
| anchor | text | heading anchor for deep link `#refunds` |
| text | text not null | Markdown, max about 3,200 chars |
| token_estimate | integer not null | |
| embed_hash | text not null | sha256(model-independent embed input: title + heading path + text) |
| visibility | text | denormalized from document for filter without join |
| effective_audience_ids | uuid[] | denormalized from document (18 M7), same on `knowledge_documents`; recomputed when article or collection chain changes |
| help_center_id, source_id | uuid null | denormalized filters |
| search | tsvector not null | written by indexer: `setweight(to_tsvector(cfg, f_unaccent(title || ' ' || heading_path)),'A') || setweight(to_tsvector(cfg, f_unaccent(text)),'B')` (not generated column: cfg is per workspace setting) |
Indexes: GIN `(workspace_id, search)` (needs `btree_gin`), `(workspace_id, embed_hash)`, `(workspace_id, document_id, ordinal)`.

### knowledge_embeddings (NEW)
No `id`. Primary key `(workspace_id, model_key, embed_hash)`. Columns: `dims smallint not null`, `embedding halfvec not null` (dimension unconstrained column; per-dims expression index), `created_at`.
Indexes: partial HNSW per supported dims: `CREATE INDEX ... USING hnsw ((embedding::halfvec(1024)) halfvec_cosine_ops) WITH (m = 16, ef_construction = 64) WHERE dims = 1024` for dims 384, 512, 768, 1024, 1536. Keyed by content hash so unchanged chunk text never re-embeds and identical text across documents embeds once.

### help_center_article_stats_daily (NEW)
`workspace_id, help_center_id, article_id, day date, views int, widget_views int, helpful int, not_helpful int, ask_ai_citations int`. Primary key `(workspace_id, article_id, day)`. Upserts batched in memory, flushed every 30 s per replica.

### help_center_searches (NEW)
`id, workspace_id, help_center_id, surface text CHECK in ('help_center','widget','api','public_mcp','ask_ai'), query text (normalized, max 200), result_count int, top_document_ids uuid[] (max 5), mode text CHECK in ('keyword','hybrid','fts_fallback'), contact_id uuid null, created_at`. `contact_id` set only on `widget` surface when request carries widget session (visitor known); null on public surfaces and anonymous widget. No IP, no user agent. Index `(workspace_id, created_at desc)`, `(workspace_id, result_count) where result_count = 0`, `(workspace_id, contact_id) where contact_id is not null` (eraser). Pruned after `help_center.search_log_retention_days` (90).

### github_connections (NEW, docs-sync.md 4.1)
Shared GitHub credential (08 knowledge git, 08 docs sync, 13 changelog).
| Column | Type | Notes |
|---|---|---|
| kind | text not null CHECK (kind ~ '^[a-z][a-z0-9_]{1,39}$') | registry enum (P1) `app/integrations/github/connection_kinds.go`: MVP `token` (fine-grained PAT; classic accepted with warning); `app` reserved (later `github-app`) |
| name | text not null | 1 to 80, default `account_login` |
| account_login | text not null | token owner (`GET /user`) |
| account_id | bigint not null | |
| token_encrypted | bytea not null | `encryption.Sealed`; never returned |
| token_last4 | text not null | |
| token_expires_at | timestamptz | `GitHub-Authentication-Token-Expiration` header; null = no expiry |
| status | text not null CHECK in ('active','error') default 'active' | |
| last_error_code, last_error | text | `github_token_invalid`, `github_token_expired`, `github_token_expiring` (warning, status stays active), `rate_limited` |
| last_checked_at | timestamptz | daily `CheckGithubConnections` |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| deleted_at | timestamptz | delete refused while repositories reference it (409 `github_connection_in_use`) |
Indexes: `(workspace_id, created_at)`.

### github_repositories (NEW, docs-sync.md 4.2)
| Column | Type | Notes |
|---|---|---|
| connection_id | uuid null references github_connections on delete restrict | null = public repo unauthenticated (knowledge git only, daily max) |
| repo_id | bigint not null | GitHub numeric id; unique `(workspace_id, repo_id) where deleted_at is null` |
| full_name | text not null | `owner/name`; refreshed from deliveries (rename/transfer) |
| default_branch | text not null | refreshed |
| private | boolean not null | |
| permissions | jsonb not null default '{}' | probed `{metadata, contents, pull_requests, webhooks}` each `none`/`read`/`write` |
| webhook_mode | text not null CHECK in ('auto','manual','none') | auto = Dimmi-created hook (Webhooks write); none = polling only |
| github_hook_id | bigint | auto mode |
| webhook_secret_encrypted | bytea not null | 32 random bytes, `encryption.Sealed`; shown once (manual mode) |
| webhook_events | text[] not null default '{}' | union of registered handlers' events for consumers on repo (`push`, `pull_request`, `release`); auto hook PATCHed on change |
| webhook_status | text not null CHECK in ('pending','active','failing','none') | |
| last_delivery_at | timestamptz | |
| status | text not null CHECK in ('active','error') default 'active' | |
| last_error_code, last_error | text | `github_repo_not_found`, `github_permission_missing`, `github_token_invalid`, `webhook_install_failed`, `github_access_lost` |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| deleted_at | timestamptz | delete refused while knowledge, docs sync or 13 changelog source references it (409 `github_repository_in_use`) |
Indexes: `(workspace_id, full_name)`, `(webhook_status) where deleted_at is null` (daily check, system scan).

### docs_sync_sources (NEW, docs-sync.md 5)
| Column | Type | Notes |
|---|---|---|
| help_center_id | uuid not null references help_centers on delete cascade | |
| github_repository_id | uuid not null references github_repositories on delete restrict | repo must have connection |
| branch | text not null | default repo default branch |
| enabled | boolean not null default true | false = paused (deliveries ignored, no rows) |
| triggers | text[] not null default '{pull_request,push}' | subset of `pull_request`, `push` |
| sensitivity | text not null CHECK in ('strict','balanced','eager') default 'balanced' | |
| include_paths, exclude_paths | text[] not null | globs; exclude default list in docs-sync.md 5 |
| ignore_labels | text[] not null default '{no-docs,skip-docs,dependencies}' | |
| force_labels | text[] not null default '{docs}' | bypass screen threshold |
| ignore_authors | text[] not null default '{dependabot[bot],renovate[bot]}' | `*[bot]` glob |
| guidance | text | max 4,000 |
| allow_new_articles | boolean not null default true | create suggestions (never in strict) |
| max_drafts_per_run | smallint not null default 5 | 1 to `docs_sync.max_drafts_per_run_cap` (10) |
| max_suggestions_per_day | smallint not null default 20 | 0 = unlimited (entitlement still applies) |
| poll_cursor_at | timestamptz not null | set at create and resume (no backfill) |
| last_seen_head_sha | text | poll-mode push detection |
| pending_push | jsonb | `{before, after, first_at, pushes}` debounce window |
| last_run_at | timestamptz | |
| status | text not null CHECK in ('active','error') default 'active' | |
| last_error_code, last_error | text | `github_access_lost`, `github_branch_not_found` |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| deleted_at | timestamptz | |
Unique `(help_center_id, github_repository_id, branch) where deleted_at is null`. Index `(workspace_id, github_repository_id)`. Entitlement `docs_sync.max_sources`.

### docs_sync_runs (NEW, docs-sync.md 6)
| Column | Type | Notes |
|---|---|---|
| source_id | uuid not null references docs_sync_sources on delete cascade | |
| help_center_id | uuid not null | denormalized filter |
| trigger | text not null CHECK in ('pull_request','push','poll','manual','redraft','freshness') | |
| change_key | text not null | `pr:{repo_id}:{n}`, `push:{repo_id}:{before}..{after}`, `{trigger}:{uuid}` for manual/redraft/freshness; partial unique `(source_id, change_key) where trigger in ('pull_request','push','poll')` |
| replay_of_run_id | uuid null references docs_sync_runs on delete set null | |
| article_id | uuid null references articles on delete set null | redraft / freshness target |
| pr_number | integer | |
| pr_title | text | max 300 |
| pr_url | text | |
| pr_author_login | text | |
| pr_labels | text[] not null default '{}' | |
| pr_merged_at | timestamptz | |
| base_sha, head_sha | text | range (`merge_commit_sha` = head) |
| status | text not null CHECK in ('queued','running','completed','skipped','failed','canceled') | |
| stage | text CHECK in ('gate','fetch','screen','map','draft','finish') | checkpoint for retry |
| skip_reason | text CHECK in ('author_ignored','label_ignored','opted_out','revert','daily_cap_reached','suggestion_cap_reached','ai_unavailable','spend_cap_reached','paths_excluded','no_user_facing_change','no_matching_articles','force_push','pr_covered') | |
| screen | jsonb | classifier output (summary, changes, confidence, user_facing); never code |
| candidates | jsonb not null default '[]' | `[{article_id, score, matched_by[], outcome, suggestion_id, error_code}]`; outcome `drafted`, `not_affected`, `import_managed`, `over_cap`, `unmatched_edits`, `spend_cap_reached`, `failed` |
| files | jsonb not null default '[]' | `[{path, previous_path, status, additions, deletions, used}]` max 300; names only |
| batch_id | uuid null references article_suggestion_batches on delete set null | |
| suggestion_count | integer not null default 0 | |
| stats | jsonb not null default '{}' | `{files_total, files_used, truncated, redacted_lines, input_tokens, output_tokens, embedding_tokens, cost_micros, ai_run_ids[], failed_candidates, reverted_accepted[], all_rejected, warnings[]}` |
| cancel_requested | boolean not null default false | |
| error_code, error | text | `github_files_unavailable`, `github_access_lost`, `rate_limited`, `invalid_output`, `stalled`, `provider_error` |
| created_by_type, created_by_id | text, uuid | `system` (webhook/poll), member, token |
| started_at, finished_at | timestamptz | |
Indexes: `(workspace_id, help_center_id, created_at desc)`, `(workspace_id, source_id, status)`, `(workspace_id, source_id, pr_number)`. Retention `docs_sync.run_retention_days` (180), prune in `PurgeHelpCenterData` (operational data, not guarded).

### article_source_refs (NEW, docs-sync.md 7.2, 8)
`id, workspace_id, article_id uuid not null references articles on delete cascade, github_repository_id uuid not null references github_repositories on delete cascade, path text not null (normalized, no leading '/', directory = trailing '/', max 512), origin text not null CHECK in ('suggestion','docs_sync','member'), suggestion_id uuid null references article_suggestions on delete set null, created_by_type, created_by_id, created_at, updated_at`. Unique `(article_id, github_repository_id, path)`. Index `(workspace_id, github_repository_id, path)`. Max 50 per article (non-member oldest evicted; member refs max 50: 422 `source_ref_limit_reached`).

### article_freshness (NEW, docs-sync.md 8; one row per article)
| Column | Type | Notes |
|---|---|---|
| article_id | uuid primary key references articles on delete cascade | no `id` (keyed by parent) |
| workspace_id, help_center_id | uuid not null | |
| status | text not null CHECK in ('fresh','needs_review') default 'fresh' | `needs_review` iff reasons non-empty |
| reasons | jsonb not null default '[]' | `[{code, severity, detail, paths[], since_sha, until_sha, commits, claims[{quote, problem, evidence_path}], detected_at}]`; codes `referenced_code_changed`, `referenced_file_deleted`, `claims_outdated`, `broken_link`, `negative_feedback`, `not_reviewed_recently` |
| verified_at | timestamptz | member verify, publish, docs sync judgement, AI ok |
| verified_shas | jsonb not null default '{}' | `{"<repo_id>": "<sha>"}` baseline per docs sync repo |
| verified_by_type, verified_by_id | text, uuid | member, token, system (`publish`, `docs_sync`, `ai_check`, `baseline`) |
| last_scanned_at, ai_checked_at | timestamptz | |
| ai_checked_sha | text | skip re-check of same `until_sha` |
Index `(workspace_id, help_center_id, status)`.

### Support SQL
- `CREATE EXTENSION IF NOT EXISTS btree_gin;` (contrib; on RDS, Supabase, Neon). In foundation required extension list.
- `CREATE FUNCTION f_unaccent(text) RETURNS text LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;` (immutable wrapper for generated columns and index expressions). If 03 defines same helper, one migration wins; both use `CREATE OR REPLACE`.

### Core table changes

None owned by 08. 13 adds `knowledge_documents.changelog_entry_id` (13 migration 001314, documented above). Widget Help tab uses 01 `widget_settings.help_center_id` + `help_tab_enabled` (01 table; FK to `help_centers` on delete set null added by 08 migration `..._create_help_centers_table.sql` when 01 table exists first).

### Migrations

```
database/migrations/20261101000801_create_help_centers_table.sql
database/migrations/20261101000802_create_help_center_assets_table.sql
database/migrations/20261101000803_create_collections_table.sql
database/migrations/20261101000804_create_articles_table.sql
database/migrations/20261101000805_create_article_versions_table.sql
database/migrations/20261101000806_create_article_suggestion_batches_table.sql
database/migrations/20261101000807_create_article_suggestions_table.sql
database/migrations/20261101000808_create_help_center_domains_table.sql
database/migrations/20261101000809_create_help_center_redirects_table.sql
database/migrations/20261101000810_create_knowledge_settings_table.sql
database/migrations/20261101000811_create_knowledge_sources_table.sql
database/migrations/20261101000812_create_knowledge_sync_runs_table.sql
database/migrations/20261101000813_create_knowledge_documents_table.sql
database/migrations/20261101000814_create_knowledge_chunks_table.sql
database/migrations/20261101000815_create_knowledge_embeddings_table.sql
database/migrations/20261101000816_create_help_center_article_stats_daily_table.sql
database/migrations/20261101000817_create_help_center_searches_table.sql
database/migrations/20261101000818_create_f_unaccent_function_and_btree_gin.sql
database/migrations/20261101000819_add_help_center_rls_policies.sql
database/migrations/20261101000821_create_github_connections_table.sql
database/migrations/20261101000822_create_github_repositories_table.sql
database/migrations/20261101000823_add_github_repository_id_to_knowledge_sources_table.sql
database/migrations/20261101000824_create_docs_sync_sources_table.sql
database/migrations/20261101000825_create_docs_sync_runs_table.sql
database/migrations/20261101000826_create_article_source_refs_and_article_freshness_tables.sql
database/migrations/20261101000827_add_docs_sync_rls_policies.sql
```
`article_suggestions.anchored_edits` ships in 0807 (pre-release, no separate migration). 13 `changelog_sources.github_repository_id` FK needs 0822 (13 block runs later; fine).

`certmagic_storage` (non-tenant: key text pk, value bytea, modified timestamptz; no lock column: locks are Postgres advisory locks) moved to framework migration set (`framework/customdomain/storage`, data-model.md framework-owned tables); 08 no longer owns migration 0820 (number left unused). `help_center_domains.purpose` ships in 0808 (pre-release, no separate migration).

## Code map

```
app/
├── actions/
│   ├── helpcenters/
│   │   create_help_center.go, update_help_center.go, delete_help_center.go, purge_help_center.go
│   │   # delete/restore call registered hooks `helpcenter.LifecycleHook.OnHelpCenterDeleted/OnHelpCenterRestored(ctx, tx, hc)` in same tx (13 registers changelog; 08 never imports 13 types)
│   │   create_default_help_center.go            # workspace initializer (contracts.md hook), idempotent
│   │   bump_help_center_cache.go                # cache_version++ + `pubsub.PubSub` bust (Valkey, topic `help_center.cache`)
│   │   public_sections.go                       # `PublicSectionProvider` registry: NavLinks, HomeSections, Routes, ReservedPaths, SitemapURLs, LlmsTxtSection, RobotsRules, SearchResultKind, CSP, HcJsModule (13, 14 register)
│   │   asset_referencers.go                     # `AssetReferencer` registry for in-use check (13 registers)
│   │   add_help_center_domain.go, verify_help_center_domain.go, remove_help_center_domain.go
│   │   authorize_tls_hostname.go                # app `customdomain.Authorizer`: ask endpoint + certmagic DecisionFunc; ownership-verified `help_center_domains`
│   │   upload_help_center_asset.go, update_help_center_asset.go, delete_help_center_asset.go   # processing via `app/support/images`
│   │   create_redirect.go, delete_redirect.go, resolve_redirect.go   # create refuses paths reserved by providers (`/changelog*` 13, `/feedback*`, `/roadmap*` 14)
│   │   record_article_view.go, record_article_feedback.go, record_search.go   # buffered
│   │   erase_help_center_contact_data.go        # EraseHelpCenterContactData: GDPR contact eraser (00-foundation/contracts.md "Contact eraser hooks"), registered on 03 eraser registry; deletes help_center_searches rows with contact_id; idempotent
│   │   rebuild_help_center_urls.go              # on 11 system.app_url_changed: bump cache_version all help centers (path mode, canonical, sitemap, llms.txt rebuilt lazily), refresh host map
│   ├── helpcenters/importing/                   # 16 import helpers: import_collection.go, import_article.go, import_asset.go, import_redirect.go (no events; section "Import helpers (16)")
│   ├── collections/
│   │   create_collection.go, update_collection.go, delete_collection.go, reorder_collections.go
│   ├── articles/
│   │   create_article.go, update_article.go, publish_article.go, unpublish_article.go
│   │   archive_article.go, unarchive_article.go, delete_article.go, restore_article.go, purge_article.go
│   │   move_article.go, reorder_articles.go, restore_article_version.go, diff_article_versions.go
│   │   render_markdown_preview.go
│   │   require_human_review.go   # RequireHumanReview(principal): review-queue.md rule 2 (surface mcp or Via oauth_client other than dimmi-cli -> human_review_required)
│   ├── suggestions/
│   │   propose_article.go, propose_article_edit.go, propose_article_archive.go
│   │   create_suggestion_batch.go, update_suggestion.go   # update with `proposed_markdown` = refresh (base reset, hunks recomputed)
│   │   accept_suggestion.go, reject_suggestion.go, accept_suggestion_batch.go, reject_suggestion_batch.go
│   └── knowledge/
│       knowledge.go                             # implements contracts.Knowledge: Search, Document, Status, EmbeddingKeyInUse, PublicHosts
│       index_changelog_entry.go                 # IndexChangelogEntry(ctx, tx, entry), RemoveChangelogEntry(ctx, tx, entryID): called by 13 in publish/unpublish tx; embeddings queued
│       embed_texts.go                           # EmbedTexts(ctx, ws, texts) ([][]float32, modelKey, usage, error); ErrNoEmbeddings; 07 spend recorded (14 duplicates)
│       search_knowledge.go                      # Search internals (retrievers, RRF; knowledge.md)
│       chunk.go                                 # Markdown-aware chunker (pure)
│       index_document.go                        # upsert document + chunks + tsvector in tx
│       index_article.go                         # article version -> document
│       embed_chunks.go, switch_embedding_model.go, start_reembed.go
│       create_knowledge_source.go, update_knowledge_source.go, delete_knowledge_source.go
│       start_knowledge_sync.go, cancel_knowledge_sync.go, finish_knowledge_sync.go
│       exclude_knowledge_document.go, include_knowledge_document.go
│       ingest_upload.go, ingest_git_tree.go, ingest_web_page.go
│       update_knowledge_settings.go, resolve_embedder.go
│   ├── github/                                  # shared GitHub layer (docs-sync.md 4); 13 + 08 consumers
│   │   add_github_connection.go, update_github_connection.go, delete_github_connection.go, test_github_connection.go
│   │   add_github_repository.go (AddRepository{WorkspaceID, FullName, ConnectionID, Actor}: resolve, probe permissions, install hook; idempotent per repo; 13 calls for `{full_name, connection_id}` sources), update_github_repository.go, delete_github_repository.go
│   │   probe_repository_permissions.go, sync_repository_webhook.go (events union -> PATCH hook), check_github_repositories.go, check_github_connections.go
│   │   client_for.go                            # ClientFor(ctx, a, ws, ghrID): token decrypted per call; replaces knowledge.ResolveGitCredentials
│   │   webhooks.go                              # WebhookHandler registry, VerifyDelivery (HMAC, repo id), envelopes, Dispatch
│   │   list_accessible_repositories.go
│   ├── docssync/                                # docs-sync.md 5-7; never imports accept/publish actions (arch test)
│   │   create_docs_sync_source.go, update_docs_sync_source.go, delete_docs_sync_source.go
│   │   handle_pull_request_delivery.go, handle_push_delivery.go, webhook_handler.go (registers `docs_sync`)
│   │   start_docs_sync_run.go (manual, redraft, freshness), cancel_docs_sync_run.go
│   │   gate.go, fetch_change.go, digest.go (budgeter, file scorer, redaction), screen_change.go, map_articles.go, draft_edit.go, draft_article.go
│   │   apply_anchored_edits.go                  # pure: exact-once matcher, overlap drop; shared with reapply
│   │   assemble_suggestions.go, finish_docs_sync_run.go, poll_docs_sync_sources.go
│   │   reapply_suggestion_edits.go              # PATCH {base: "current"} (called by suggestions controller)
│   │   record_source_refs.go                    # accept hook: refs + freshness baseline
│   └── freshness/                               # docs-sync.md 8
│       scan_article_freshness.go, verify_article_claims.go, record_article_verification.go, mark_article_verified.go
│       backfill_article_source_refs.go, freshness_reasons.go (pure rules)
├── contracts/
│   embeddings.go                                # Embeddings contract (knowledge.md; domain, `ids.UUID`)
├── drivers/
│   └── embeddings/{openai,gemini,bedrock,openaicompat,voyage}/   # adapter zone; openrouter + ollama use openaicompat
├── managers/embeddings_manager.go               # on `framework/manager`: ForKey(ctx, ai_provider_key) -> contracts.Embeddings
├── integrations/knowledge/sources.go            # registry enum `knowledge_sources.type` (website, upload, git)
├── integrations/github/connection_kinds.go      # registry enum `github_connections.kind` (token; app later)
├── services/github/                             # adapter zone: google/go-github/v93 v93.0.0 (`a.HTTP.StdClient()`, retries off, host allowlist api.github.com + codeload.github.com, rate-limit header snooze); git sync tree/tarball, users, repos, hooks, pulls + PR files, compare, commits/pulls (08 docs sync); 13 adds releases
├── enums/
│   article_status.go, article_visibility.go, suggestion_kind.go, suggestion_status.go, suggestion_source_kind.go
│   knowledge_source_type.go, knowledge_source_mode.go, knowledge_source_status.go, sync_schedule.go
│   github_webhook_mode.go, github_webhook_status.go, docs_sync_trigger.go, docs_sync_run_status.go, docs_sync_skip_reason.go, docs_sync_sensitivity.go, freshness_status.go, freshness_reason.go
│   document_status.go, domain_status.go, search_surface.go, search_mode.go
├── events/
│   article_created.go, article_published.go, article_updated.go, article_unpublished.go, article_deleted.go
│   article_suggestion_created.go, article_suggestion_reviewed.go
│   knowledge_source_sync_completed.go, knowledge_source_sync_failed.go
│   help_center_domain_status_changed.go, knowledge_embeddings_switched.go, help_center_updated.go
│   docs_sync_run_completed.go, docs_sync_run_failed.go, article_freshness_changed.go, github_repository_webhook_status_changed.go, github_connection_errored.go
├── exceptions/help_center_errors.go             # codes in api.md
├── http/
│   ├── controllers/api/v1/
│   │   help_center_controller.go, help_center_domain_controller.go, help_center_domain_check_controller.go (Store)
│   │   help_center_cache_controller.go, help_center_asset_controller.go, help_center_redirect_controller.go
│   │   help_center_stats_controller.go, help_center_preview_controller.go
│   │   collection_controller.go, help_center_collection_controller.go (Index, Store, UpdateMany = order)
│   │   collection_article_controller.go (UpdateMany = order)
│   │   article_controller.go                    # Update dispatches per field (rest.md rule 3): verified true -> MarkArticleVerified (RequireHumanReview); content -> UpdateArticle; status draft -> UnpublishArticle | UnarchiveArticle; status archived -> ArchiveArticle; trashed false -> RestoreArticle; collection_id/position/help_center_id -> MoveArticle; restore_version_id -> RestoreArticleVersion
│   │   article_publication_controller.go        # Store POST /articles/{art}/publications -> PublishArticle
│   │   article_version_controller.go, article_version_diff_controller.go
│   │   article_suggestion_controller.go         # Update: proposal edits -> UpdateSuggestion; base current -> ReapplySuggestionEdits; status accepted|rejected -> AcceptSuggestion | RejectSuggestion
│   │   article_suggestion_batch_controller.go   # Store, Show, Update (status accepted|rejected -> AcceptSuggestionBatch | RejectSuggestionBatch)
│   │   knowledge_source_controller.go, knowledge_source_sync_controller.go (Index, Show, Store, Destroy = cancel)
│   │   knowledge_document_controller.go, knowledge_upload_controller.go, knowledge_search_controller.go
│   │   knowledge_settings_controller.go
│   │   github_connection_controller.go, github_connection_test_controller.go (Store), github_connection_accessible_repository_controller.go (Index)
│   │   github_repository_controller.go, github_repository_check_controller.go (Store)
│   │   docs_sync_source_controller.go, docs_sync_run_controller.go (Index, Show, Store, Update = cancel)
│   │   article_source_ref_controller.go (Index, Store, Destroy), help_center_freshness_check_controller.go (Store)
│   ├── controllers/web/helpcenter/
│   │   home_controller.go, collection_page_controller.go, article_page_controller.go, article_markdown_controller.go
│   │   search_page_controller.go, ask_page_controller.go, sitemap_controller.go, robots_controller.go
│   │   llms_txt_controller.go, llms_full_txt_controller.go, asset_controller.go, static_controller.go
│   │   search_api_controller.go, answer_api_controller.go, article_feedback_controller.go, public_mcp_controller.go
│   │   not_found_controller.go
│   ├── controllers/web/tls_ask_controller.go    # admin port only; thin: `customdomain.AskHandler(authorizer)`
│   ├── controllers/widget/
│   │   help_home_controller.go, help_collection_controller.go, help_article_controller.go
│   │   help_search_controller.go, help_article_feedback_controller.go
│   ├── controllers/webhooks/github_repository_webhook_controller.go   # shared: verify, dedupe, envelope, enqueue HandleGithubDelivery (202)
│   ├── middleware/resolve_help_center.go        # Host or /hc/{subdomain} -> help center, canonical redirect
│   ├── middleware/help_center_cache.go          # L1 page cache + ETag/304
│   ├── middleware/help_center_csp.go            # base CSP + provider overrides (14 portal `frame-ancestors`)
│   ├── requests/ (one per write endpoint, names mirror controllers: store_article_request.go, update_article_request.go,
│   │   store_article_publication_request.go, store_article_suggestion_request.go, update_article_suggestion_request.go,
│   │   store_article_suggestion_batch_request.go, update_article_suggestion_batch_request.go,
│   │   store_knowledge_source_request.go, update_knowledge_settings_request.go,
│   │   store_help_center_domain_request.go, store_help_center_request.go, update_help_center_request.go,
│   │   store_collection_request.go, update_order_request.go, store_redirect_request.go, search_knowledge_request.go ...)
│   └── resources/
│       help_center_resource.go, help_center_domain_resource.go, collection_resource.go, article_resource.go
│       article_version_resource.go, article_diff_resource.go, article_suggestion_resource.go, suggestion_batch_resource.go
│       knowledge_source_resource.go, knowledge_sync_run_resource.go, knowledge_document_resource.go
│       knowledge_chunk_resource.go, knowledge_settings_resource.go, help_center_asset_resource.go, redirect_resource.go
│       widget_help_article_resource.go, widget_help_collection_resource.go, public_search_result_resource.go
├── jobs/  (see Jobs table)
├── listeners/
│   suggest_embedder_on_key_change.go            # queued ingest: ai.provider_key_changed (created/rotated) -> pick embedder or resume degraded embeds
│   bump_help_center_cache_on_change.go          # sync after commit
│   index_article_knowledge.go                   # sync in tx (FTS) + queues EmbedKnowledgeChunks
│   supersede_suggestions_on_delete.go
│   record_article_verification_on_publish.go    # queued default: article.published/updated -> RecordArticleVerification (heads per docs sync repo)
│   sync_repository_webhook_events.go            # queued default: knowledge/docs sync/13 source created or deleted -> SyncRepositoryWebhook
│   rebuild_help_center_urls_listener.go         # queued default: 11 system.app_url_changed -> RebuildHelpCenterUrls
├── mcp/
│   tools/help_center_tools.go                   # dimmi_get_help_center_outline, dimmi_list_articles, dimmi_get_article
│   tools/suggestion_tools.go                    # dimmi_propose_article, dimmi_propose_article_edit, dimmi_propose_help_center, dimmi_list_article_suggestions
│   tools/knowledge_tools.go                     # dimmi_search_knowledge, dimmi_list_knowledge_sources, dimmi_sync_knowledge_source
│   tools/docs_sync_tools.go                     # toolset docs_sync: dimmi_list_docs_sync_runs, dimmi_list_stale_articles, dimmi_request_docs_update
│   prompts/update_docs_for_change.go            # MCP prompt dimmi_update_docs_for_change (local agent, zero server LLM)
│   prompts/draft_help_center.go                 # MCP prompt dimmi_draft_help_center
│   public/help_center_public_server.go          # public read-only MCP on `framework/mcp`: dimmi_search_docs (items carry `kind`: article | changelog), dimmi_get_doc (resolves article and changelog entry URLs/slugs), dimmi_list_docs
├── policies/
│   help_center_policy.go, article_policy.go, collection_policy.go, article_suggestion_policy.go, knowledge_source_policy.go
├── console/commands/
│   knowledge_reembed_command.go                 # dimmi knowledge:reembed {workspace?} [--model]
│   knowledge_sync_command.go                    # dimmi knowledge:sync {source}
│   knowledge_reindex_command.go                 # dimmi knowledge:reindex {workspace?} (rebuild tsvector after config change)
│   help_center_cache_clear_command.go           # dimmi help-center:cache-clear
│   help_center_rerender_command.go              # dimmi help-center:rerender (renderer upgrade)
│   help_center_domains_check_command.go         # dimmi help-center:domains:check {hostname}
│   docs_sync_replay_command.go                  # dimmi docs-sync:replay {run}
│   github_check_command.go                      # dimmi github:check (connections + repositories now)
├── support/
│   helpmarkdown/ options.go (help center dialect as `markdown.Options`: GFM, GitHub alerts (own ~100-line goldmark extension inside `framework/markdown`), heading ids, toc (go.abhg.dev/goldmark/toc), link/image rewrite, `Highlight` (chroma classes), `sanitize` policy), profiles: `Help()`, `Engage()` (15-announcements-surveys: no headings above H3, no footnotes, no embeds-as-cards, images `eas_` or https only); plain text via `markdown.PlainText`; front matter via `markdown.FrontMatter` (go.abhg.dev/goldmark/frontmatter v0.3.0, 16 KB cap)
│   textdiff/ hunks.go, words.go: hunk model + review rules (app); diff engines from framework `support/diff` (aymanbagabas/go-udiff v0.4.1, sergi/go-diff v1.4.0). No diff3 at MVP (`review-rebase` later)
│   crawl/ robots.go (framework `httpclient/robots` on temoto/robotstxt v1.1.2; RFC 9309 vectors kept as our test; 5xx = disallow is ours), sitemap.go (stdlib `encoding/xml`, `compress/gzip`), extract.go (`dom.Readable` on codeberg.org/readeck/go-readability/v2 v2.1.3 + `dom.ToMarkdown` on html-to-markdown/v2), politeness.go (per-host token bucket via framework `ratelimit.Limiter` local token-bucket mode (in-process, per crawl job; no `golang.org/x/time/rate` import in app zone; framework.md owner adds mode if absent) + concurrency semaphore); fetch via `a.HTTP` (SSRF guard)
│   customdomains/ claims.go (`Claimed(hostname)`: `help_center_domains` + `widget_domains` + platform hosts), reserved_subdomains.go (shared reserved list), authorizer.go (wires `AuthorizeTLSHostname` into `framework/customdomain`)
│   images/ images.go (allowed types, pixel/size caps, metadata strip, `a.Scanner` scan; on `framework/image`; shared with 15-announcements-surveys `engage_assets`)
│   secretscan/ scan.go, redact.go (pure: secret patterns for `possible_secret` warning and docs sync diff redaction)
│   ziputil/ safe_unzip.go (stdlib `archive/zip` + `filepath.IsLocal` via `framework/filesystem` path helpers; zip-slip, bomb guard)
│   # slug: `str.Slug` (`framework/support/str`, gosimple/slug v1.15.0) + collision suffix in actions; no own package
resources/views/helpcenter/
   layout.templ, home.templ, collection.templ, article.templ, search.templ, ask.templ, error.templ, partials/*.templ
   assets/hc.css, assets/hc.js (embedded, hashed names)
resources/prompts/docs_sync/v1/
   screen.md, draft_edit.md, draft_article.md, verify_article.md, *.schema.json (JSON Schema per purpose; prompts versioned, golden-tested)
routes/web.go       # help center host group + /hc/{subdomain} group
routes/api.go       # /api/v1 help center + knowledge routes
routes/widget.go    # /widget/v1/help/*
routes/webhooks.go  # /webhooks/github/repositories/{ghr}; GitHub handler registration (knowledge_git, docs_sync; 13 registers changelog)
routes/mcp.go       # tool registration + public MCP mount
routes/console.go   # periodic jobs
config/helpcenter.go, config/knowledge.go, config/docssync.go, config/github.go
database/queries/
   help_centers.sql, collections.sql, articles.sql, article_versions.sql, article_suggestions.sql,
   article_suggestion_batches.sql, help_center_domains.sql, help_center_redirects.sql, help_center_assets.sql,
   knowledge_settings.sql, knowledge_sources.sql, knowledge_sync_runs.sql, knowledge_documents.sql,
   knowledge_chunks.sql, knowledge_embeddings.sql, help_center_article_stats_daily.sql, help_center_searches.sql,
   github_connections.sql, github_repositories.sql, docs_sync_sources.sql, docs_sync_runs.sql, article_source_refs.sql, article_freshness.sql,
   certmagic_storage.sql
database/factories/
   help_center.go, collection.go, article.go (WithVersions(n), Published()), article_suggestion.go,
   knowledge_source.go, knowledge_document.go (WithChunks(n), WithEmbeddings(model)), help_center_domain.go,
   github_connection.go, github_repository.go (WithWebhook(mode)), docs_sync_source.go, docs_sync_run.go (Skipped(reason), WithSuggestions(n)), article_source_ref.go
database/seeders/help_center_seeder.go          # demo help center: 3 collections, 12 articles, 2 suggestions
tests/  (testing.md: app logic integration only on real Postgres `pgvector/pgvector:pg18` + real Valkey; fakes only at process boundaries: `httpclienttest.New(tc.App.HTTP)` (crawl targets, GitHub, embeddings providers), `llm.Fake()`, `tc.Clock`; tests named in plain words, every edge case covered)
   feature/helpcenter/*_test.go (per endpoint), feature/helpcenter/ssr_test.go, seo_test.go, llms_txt_test.go, public_sections_test.go (fake provider registered),
   feature/knowledge/search_test.go (RRF + chunker through real index), crawl_test.go (`httpclienttest.New(tc.App.HTTP)` site, RFC 9309 vectors), git_sync_test.go, upload_test.go,
   feature/suggestions/*_test.go (textdiff hunks + stale rule through real accept), feature/mcp/help_center_tools_test.go, public_mcp_test.go,
   feature/helpcenter/render_test.go (XSS corpus through real render + sanitize), feature/helpcenter/domains_test.go (`framework/dns` fake resolver, ask endpoint),
   feature/github/*_test.go (connections, repositories, webhook route: signature, repo id, dedupe, dispatch), feature/docssync/*_test.go (12 PR fixtures through real pipeline, `llm.Fake()` per purpose, `httpclienttest.New(tc.App.HTTP)` GitHub), feature/docssync/leak_test.go (code marker absent from DB, Valkey, logs, traces), feature/freshness/*_test.go,
   arch/docssync_test.go (no import of accept/publish actions),
   contracts/embeddings_test.go (domain contract suite; every driver + fake),
   bench/help_center_bench_test.go (TTFB, search p95)
   # chunker, RRF math, hunk builder: pure funcs with no I/O may keep table tests (no mocks); TLS drivers tested in `framework/customdomain/drivertest`
```

## HTTP API

Full JSON examples in [api.md](api.md). All `/api/v1` routes: `auth:session|auth:token`, `workspace`, `throttle:api`. Abilities: `articles:read` / `articles:write` for content, suggestions, search; `settings:read` / `settings:write` for help center config, domains, sources, knowledge settings. Policies call `authz.Can(principal, permission)` (Permissions matrix); column below names permission (ability group in matrix).

### REST `/api/v1`

| Method | Path | Route name | Permission | Purpose |
|---|---|---|---|---|
| GET | /help-centers | help-centers.index | articles:read | list |
| POST | /help-centers | help-centers.store | help_centers:manage | create (entitlement `help_center.max_per_workspace`; over: 422 `entitlement_limit_reached` with `key`, `limit`) |
| GET | /help-centers/{hc} | help-centers.show | articles:read | include=domains,collections |
| PATCH | /help-centers/{hc} | help-centers.update | help_centers:manage | name, subdomain, settings, toggles |
| DELETE | /help-centers/{hc} | help-centers.destroy | help_centers:delete | soft delete (not last default unless only one) |
| DELETE | /help-centers/{hc}/cache | help-centers.cache.destroy | help_centers:manage | bump cache_version |
| GET | /help-centers/{hc}/domains | help-centers.domains.index | help_centers:read_settings | |
| POST | /help-centers/{hc}/domains | help-centers.domains.store | help_centers:manage | add hostname, returns DNS records |
| DELETE | /help-centers/{hc}/domains/{hcd} | help-centers.domains.destroy | help_centers:manage | remove |
| POST | /help-centers/{hc}/domains/{hcd}/checks | help-centers.domains.checks.store | help_centers:manage | re-verify now: 202 + domain resource, queues `VerifyHelpCenterDomain` (unique per domain; rate limited 1/10 s, 429) |
| GET | /help-centers/{hc}/assets | help-centers.assets.index | articles:read | |
| POST | /help-centers/{hc}/assets | help-centers.assets.store | articles:write | multipart upload, max 10 MB |
| PATCH | /help-center-assets/{ast} | help-center-assets.update | articles:write | alt text |
| DELETE | /help-center-assets/{ast} | help-center-assets.destroy | articles:write | refused if referenced by published version, settings or any registered referencer (13 changelog entries) (409 `asset_in_use`) |
| GET | /help-centers/{hc}/redirects | help-centers.redirects.index | articles:read | |
| POST | /help-centers/{hc}/redirects | help-centers.redirects.store | articles:write | manual redirect |
| DELETE | /help-center-redirects/{rdr} | help-center-redirects.destroy | articles:write | |
| GET | /help-centers/{hc}/stats | help-centers.stats.show | articles:read | views, top articles, searches, zero-result queries, date range max 90 days |
| POST | /help-centers/{hc}/previews | help-centers.previews.store | articles:read | Markdown in, sanitized HTML + toc + warnings out (editor preview) |
| GET | /help-centers/{hc}/collections | help-centers.collections.index | articles:read | tree |
| POST | /help-centers/{hc}/collections | help-centers.collections.store | articles:write | |
| PATCH | /help-centers/{hc}/collections | help-centers.collections.update-many | articles:write | `{order: [col_...], parent_id?}`: exact sibling list under parent (null = top level) |
| GET | /collections/{col} | collections.show | articles:read | |
| PATCH | /collections/{col} | collections.update | articles:write | name, slug (adds redirect), parent, icon |
| DELETE | /collections/{col} | collections.destroy | articles:write | `?articles=move_to_parent|uncollect` (default uncollect); 409 if has children |
| PATCH | /collections/{col}/articles | collections.articles.update-many | articles:write | `{order: [art_...]}`: exact list of collection's non-trashed articles |
| GET | /articles | articles.index | articles:read | filter[help_center,collection,status,visibility,has_unpublished_changes,freshness], `filter[trashed]=with|only` (trash view, rest.md rule 7), q (title trigram), sort=position,-updated_at,title |
| POST | /articles | articles.store | articles:write | create draft (optional `publish: true`) |
| GET | /articles/{art} | articles.show | articles:read | working copy + published summary; `include=published_version,collection` |
| PATCH | /articles/{art} | articles.update | articles:write | content fields; `status` (`draft` \| `archived`, + `redirect_to`); `trashed: false`; `collection_id`, `position`, `help_center_id` (move); `restore_version_id` (+ `publish`, `restore_slug`); `verified: true` (freshness, `RequireHumanReview`). See Article state. `If-Match: <lock_version>` required from session for content fields; optional for token |
| DELETE | /articles/{art} | articles.destroy | articles:write | to trash |
| POST | /articles/{art}/publications | articles.publications.store | articles:write | `{note}`: publish working copy as new version, 201 `article_version`. `RequireHumanReview`: never via MCP, never third-party OAuth client (review-queue.md rule 2) |
| GET | /articles/{art}/versions | articles.versions.index | articles:read | |
| GET | /articles/{art}/versions/{artv} | articles.versions.show | articles:read | |
| GET | /articles/{art}/versions/{artv}/diff | articles.versions.diff | articles:read | `?against=working|published|artv_...` |
| GET | /article-suggestions | article-suggestions.index | articles:read | filter[status,help_center,article,batch,source_kind,kind] |
| POST | /article-suggestions | article-suggestions.store | articles:write | propose create/edit/archive |
| GET | /article-suggestions/{sug} | article-suggestions.show | articles:read | `stale` computed at read (working copy differs from `base_markdown`) |
| PATCH | /article-suggestions/{sug} | article-suggestions.update | articles:write; `status` change session or token, never MCP | reviewer edits or refreshes proposal (`proposed_markdown`; base reset to current working copy, hunks recomputed), or `{status: "accepted", hunks?, publish?, collection_id?, slug?, note?, acknowledge_warnings?}` (lands `accepted` or `partially_accepted`), or `{status: "rejected", note?}`, or `{base: "current"}` (docs sync suggestions: reapply anchored edits, docs-sync.md 7.4; else 422 `reapply_not_supported`). Never two of these in one body (422) |
| POST | /article-suggestion-batches | article-suggestion-batches.store | articles:write | cold start: up to 200 items, body limit 10 MB |
| GET | /article-suggestion-batches/{sugb} | article-suggestion-batches.show | articles:read | |
| PATCH | /article-suggestion-batches/{sugb} | article-suggestion-batches.update | articles:write (status never MCP) | `{status: "accepted", publish?}` accepts all pending; `{status: "rejected", note?}` rejects all pending. Per-suggestion `meta.results`. Batch `status` derived (see Review state) |
| GET | /knowledge/sources | knowledge.sources.index | help_centers:read_settings | |
| POST | /knowledge/sources | knowledge.sources.store | knowledge:manage | website or git (upload via /knowledge/uploads); entitlement `knowledge.max_sources` (422 `entitlement_limit_reached`) |
| GET | /knowledge/sources/{ks} | knowledge.sources.show | help_centers:read_settings | |
| PATCH | /knowledge/sources/{ks} | knowledge.sources.update | knowledge:manage | config, schedule, pause |
| DELETE | /knowledge/sources/{ks} | knowledge.sources.destroy | knowledge:manage | removes docs/chunks async |
| POST | /knowledge/sources/{ks}/syncs | knowledge.sources.syncs.store | knowledge:sync | run now: 202 `knowledge_sync_run` (trigger `manual`); 409 `sync_in_progress` |
| GET | /knowledge/sources/{ks}/syncs | knowledge.sources.syncs.index | help_centers:read_settings | run history (last 50) |
| GET | /knowledge/sources/{ks}/syncs/{ksr} | knowledge.sources.syncs.show | help_centers:read_settings | |
| DELETE | /knowledge/sources/{ks}/syncs/{ksr} | knowledge.sources.syncs.destroy | knowledge:sync | cancel run: row kept, `status: cancelled`, 200 run; finished run 409 `sync_not_running`; already cancelled 200 |
| GET | /knowledge/sources/{ks}/documents | knowledge.sources.documents.index | help_centers:read_settings | filter[status], q |
| GET | /knowledge/documents/{kd} | knowledge.documents.show | help_centers:read_settings | content + chunks |
| PATCH | /knowledge/documents/{kd} | knowledge.documents.update | knowledge:sync | `excluded: true|false` |
| POST | /knowledge/uploads | knowledge.uploads.store | knowledge:manage | multipart zip, max 50 MB; new or existing upload source |
| GET | /knowledge/search | knowledge.search | articles:read | retrieval debugger: chunks, scores, mode, timings |
| GET | /knowledge/settings | knowledge.settings.show | help_centers:read_settings | embedding status, progress |
| PATCH | /knowledge/settings | knowledge.settings.update | knowledge:manage | pick embedding key/model (starts re-embed) |
| GET | /github-connections | github-connections.index | github:read | shared GitHub layer (docs-sync.md 4, 10) |
| POST | /github-connections | github-connections.store | github:manage | `{token, name?}`; validates `GET /user`; token never returned |
| GET | /github-connections/{ghc} | github-connections.show | github:read | |
| PATCH | /github-connections/{ghc} | github-connections.update | github:manage | `name`, `token` (rotate; repos re-probed) |
| DELETE | /github-connections/{ghc} | github-connections.destroy | github:manage | 409 `github_connection_in_use` while repos reference it |
| POST | /github-connections/{ghc}/tests | github-connections.tests.store | github:manage | synchronous re-validate, 201 `github_connection_test` (rule 5) |
| GET | /github-connections/{ghc}/accessible-repositories | github-connections.accessible-repositories.index | github:read | computed from GitHub, not stored; `q`, max 100 |
| GET | /github-repositories | github-repositories.index | github:read | `used_by` consumers, webhook status |
| POST | /github-repositories | github-repositories.store | github:manage | `{full_name, connection_id?}`; auto hook or `meta.webhook` (secret shown once) |
| GET | /github-repositories/{ghr} | github-repositories.show | github:read | |
| PATCH | /github-repositories/{ghr} | github-repositories.update | github:manage | `connection_id`, `webhook_mode`, `rotate_webhook_secret: true` |
| DELETE | /github-repositories/{ghr} | github-repositories.destroy | github:manage | 409 `github_repository_in_use` with `consumers` (knowledge, docs sync, 13 changelog sources); deletes auto hook |
| POST | /github-repositories/{ghr}/checks | github-repositories.checks.store | github:manage | 202: re-probe permissions, ping hook; 1/10 s (429) |
| GET | /docs-sync-sources | docs-sync-sources.index | help_centers:read_settings | filter[help_center] |
| POST | /docs-sync-sources | docs-sync-sources.store | docs_sync:manage | entitlement `docs_sync.max_sources` (422 `entitlement_limit_reached`); `meta.estimate` cost per PR |
| GET | /docs-sync-sources/{dss} | docs-sync-sources.show | help_centers:read_settings | accept rate, last runs summary |
| PATCH | /docs-sync-sources/{dss} | docs-sync-sources.update | docs_sync:manage | settings, `enabled` (pause/resume) |
| DELETE | /docs-sync-sources/{dss} | docs-sync-sources.destroy | docs_sync:manage | soft delete; pending suggestions kept; webhook events union shrinks |
| GET | /docs-sync-runs | docs-sync-runs.index | articles:read | filter[help_center,source,status,skip_reason,trigger,pr_number]; skipped-PR log |
| GET | /docs-sync-runs/{dsr} | docs-sync-runs.show | articles:read | screen, candidates, files, stats, review URL |
| POST | /docs-sync-runs | docs-sync-runs.store | articles:write | 202; `trigger` `manual` (`source_id` + `pr_number` or `base_sha`/`head_sha`), `redraft` (`replay_of`, `article_id`), `freshness` (`article_id`); 429 `entitlement_limit_reached` (`docs_sync.runs_daily`) |
| PATCH | /docs-sync-runs/{dsr} | docs-sync-runs.update | articles:write | `{status: "canceled"}`; finished 409 `docs_sync_run_finished` |
| GET | /articles/{art}/source-refs | articles.source-refs.index | articles:read | |
| POST | /articles/{art}/source-refs | articles.source-refs.store | articles:write | `{github_repository_id, path}` |
| DELETE | /article-source-refs/{asr} | article-source-refs.destroy | articles:write | |
| POST | /help-centers/{hc}/freshness-checks | help-centers.freshness-checks.store | articles:write | 202 run scan now; 1/hour per help center (429) |

### Article state (`PATCH /articles/{art}`)

Status enum unchanged: `draft`, `published`, `archived` (`published` = has live version). Publishing creates a version (identity, history), so it is child collection `publications`; `status` on PATCH only moves away from live. Actions keep business names.

| From | Body | To | Action, event |
|---|---|---|---|
| draft, published (has unpublished changes or not) | `POST /articles/{art}/publications {note?}` | published | `PublishArticle`, new version kind `published`, `article.published` / `article.updated`. 409 `nothing_to_publish` when working copy equals live version. 201 `article_version` + `meta.article` (status, url, `has_unpublished_changes`) + `meta.warnings` |
| published | `{status: "draft"}` | draft | `UnpublishArticle` (unpublish), `article.unpublished` reason `reverted_to_draft` |
| archived | `{status: "draft"}` | draft | `UnarchiveArticle` (unarchive, never auto-publishes) |
| draft, published | `{status: "archived", redirect_to?}` | archived | `ArchiveArticle`; was published: `article.unpublished` reason `archived` |
| draft, archived | `status` equal to current | same | no-op 200, no event |
| any | `{status: "published"}` | 422 `invalid_transition` (`from`, `to`) | message: publish via `POST /articles/{art}/publications` |
| trashed | `{trashed: false}` | draft | `RestoreArticle` (rest.md rule 7). Other fields on trashed article: 409 `article_trashed` (restore first) |
| any not trashed | `{collection_id?, position?, help_center_id?}` | unchanged status | `MoveArticle` (old move route). `help_center_id` change: slug auto-suffix on conflict + redirect (edge-cases 28). `position` inside target collection |
| any not trashed | `{restore_version_id, publish?, restore_slug?}` | draft, or published when `publish: true` | `RestoreArticleVersion`: working copy replaced, version kind `restored` (+ `published`). Content fields in same body: 422 |
| published | `{verified: true}` | unchanged status | `MarkArticleVerified`: freshness reasons cleared, baseline reset, `article.freshness_changed` when status changes. `RequireHumanReview` (MCP surface, third-party OAuth: 403 `human_review_required`). `verified: false` 422 `invalid_transition`. Draft/archived: 409 `article_not_published` |

`If-Match` needed from session only when body has content fields (`title`, `slug`, `description`, `body_markdown`, `visibility`, `seo`); state, move and restore fields do not need it. Publish via MCP surface: always 403 `human_review_required` (review-queue.md rule 2).

### Review state (suggestions and batches)

- Suggestion `status` enum unchanged (`pending`, `accepted`, `partially_accepted`, `rejected`, `superseded`). Client sends `accepted` or `rejected` only; server lands `partially_accepted` when `hunks` is a subset. Only from `pending` (else 409 `suggestion_not_pending`). `superseded` system-only.
- Batch has no stored status column. Resource field `status` derived from counters: `pending` while `pending > 0`; else `accepted` when `rejected = 0`; `rejected` when `accepted = 0`; else `mixed`. `PATCH {status: "accepted"}` = accept all pending (old batch accept); stale items stay pending, so batch may stay `pending`; `meta.results` lists each (`accepted`, `skipped_stale`, `failed` with code). `PATCH {status: "rejected"}` = reject all pending.

### Help center hosts (web, no auth)

Group middleware: `resolve_help_center`, `help_center_cache`, `throttle:help_center`. Same routes under `{app_url}/hc/{subdomain}` (path mode).

| Method | Path | Route name | Purpose |
|---|---|---|---|
| GET | / | hc.home | collections + popular |
| GET | /index.md | hc.home.markdown | |
| GET | /collections/{slug} | hc.collections.show | |
| GET | /collections/{slug}.md | hc.collections.markdown | |
| GET | /articles/{slug} | hc.articles.show | `Accept: text/markdown` returns Markdown |
| GET | /articles/{slug}.md | hc.articles.markdown | |
| GET | /search | hc.search | `?q=` SSR results |
| GET, POST | /answers | hc.answers.index, hc.answers.store | no-JS Ask AI (POST form, renders answer) |
| GET | /sitemap.xml | hc.sitemap | |
| GET | /robots.txt | hc.robots | |
| GET | /llms.txt | hc.llms | |
| GET | /llms-full.txt | hc.llms-full | |
| GET | /assets/{ast}/{filename} | hc.assets.show | immutable |
| GET | /_static/{file} | hc.static | hashed CSS/JS/fonts |
| GET | /api/search | hc.api.search | JSON for hc.js; items carry `kind` (`article` \| `changelog`), changelog results labeled with date |
| POST | /api/answers | hc.api.answers.store | Ask AI: SSE stream (computed answer, not stored as resource), `Content-Type: application/json` required |
| POST | /api/articles/{art}/feedback | hc.api.feedback | thumbs |
| POST | /mcp | hc.mcp | public read-only MCP (streamable HTTP, stateless) |
| GET | /favicon.ico | hc.favicon | |
| GET | /.well-known/dimmi-domain-check/{token} | hc.domain-check | HTTP probe for domain routing verification (any host, before help center resolution; `framework/customdomain` probe handler) |
| * | provider routes | per provider | `PublicSectionProvider.Routes` mounted in this group before redirect/404 fallback: 13 `/changelog*`, 14 `/feedback*`, `/roadmap*`, `/api/feedback/*`. Same host resolution, path mode, canonical 301 (pages redirect to primary host; `/api/*` not redirected). Paths reserved: manual redirect onto them 422 `redirect_shadows_section` |

Provider-fed outputs (iterated by 08 templates/builders, providers registered in `routes/web.go`): header nav links ("Changelog", "Feedback", "Roadmap" when enabled and public), home sections ("Latest updates"), sitemap URLs (child sitemaps `/sitemaps/{section}-{n}.xml` when split), llms.txt sections (`## Changelog`; 14 sections later: `portal-md-twins`), robots.txt `Disallow` lines (14: `/feedback/sign-in`, `/feedback/sso`, `/feedback/account`, `/feedback/new`), search result kinds, CSP overrides on own paths only (14 portal `frame-ancestors` from `feedback_settings.frame_ancestors`, `form-action 'self'`). `hc.js` bundles provider modules (13 reactions + subscribe); budget 5 KB gz total holds.

Admin port only (never public listener): `GET /internal/tls/ask?domain=` (`tls.ask`), Caddy on-demand ask (11 Caddyfile). 200 only when hostname is ownership-verified `help_center_domains` row (`AuthorizeTLSHostname`), else 404.

### Widget `/widget/v1` (auth:widget, app-only allowed)

Auth: `app_id` (`wgt_...`) required; visitor session optional (public help, anonymous visitors see Help tab). Help center = widget's `widget_settings.help_center_id` (01); null or `help_tab_enabled = false`: 404 `help_center_not_linked`. Every query scoped to that help center (`HelpCenterIDs [widget help_center_id]`, visibility public, published only); article/collection of other help center 404. Origin check same as 01 widget group. No session: `Cache-Control: public, max-age=60`. Session present: only used to set `help_center_searches.contact_id`. `CreateDefaultHelpCenter` initializer (runs after 01 `EnsureDefaultWidget`) sets `help_center_id` on workspace widgets where null (Help tab stays off until `help_tab_enabled`).

| Method | Path | Route name | Purpose |
|---|---|---|---|
| GET | /help | widget.help.home | widget's linked help center: collections, top articles |
| GET | /help/collections/{col} | widget.help.collections.show | |
| GET | /help/articles/{art} | widget.help.articles.show | sanitized HTML + URL; deep-open target for SDK `show('help', {article})` and 15-announcements-surveys CTA `open_article`; unpublished/other help center: 404 `article_not_found` (widget shows Help home + toast) |
| GET | /help/search | widget.help.search | keyword search, public only |
| POST | /help/articles/{art}/feedback | widget.help.articles.feedback | |

### Webhooks in

| Method | Path | Route name | Auth |
|---|---|---|---|
| POST | /webhooks/github/repositories/{ghr} | webhooks.github.repositories | Shared per repo (08 knowledge git `push`, 08 docs sync `pull_request` + `push`, 13 changelog `release`). `X-Hub-Signature-256` HMAC (`hashing.HMAC`, constant-time `hashing.Equal`) with per-repo secret; payload `repository.id` must equal `repo_id`; `X-GitHub-Delivery` dedupe 24 h (Valkey `SET NX EX` + River unique job); compact envelope (no raw payload) -> `HandleGithubDelivery` -> registered handlers; 202 under 1 s; `ping` 200 + `webhook_status active`; unknown repo 404; bad signature 401. docs-sync.md 4.3 |

### Error codes (feature)

`entitlement_limit_reached` (422 count caps: help centers, articles, knowledge sources, docs sync sources; 429 + `Retry-After` per-period caps: `docs_sync.runs_daily`; body `key`, `limit`), `subdomain_taken` (422), `subdomain_reserved` (422), `slug_taken` (422), `collection_depth_exceeded` (422), `collection_not_empty` (409), `stale_resource` (409), `article_not_published` (409), `article_archived` (409), `article_trashed` (409, non-restore PATCH on trashed article), `invalid_transition` (422, `from`/`to`; e.g. PATCH `status: published`, batch or suggestion bad status), `order_mismatch` (422, `order` not exact sibling list), `nothing_to_publish` (409), `suggestion_not_pending` (409), `suggestion_stale` (409, article working copy changed since proposal; refresh or reject), `hunk_not_found` (422), `batch_too_large` (413), `markdown_too_large` (413), `domain_taken` (409), `domain_invalid` (422), `domain_is_platform` (422), `domain_limit_reached` (422), `asset_too_large` (413), `asset_type_not_allowed` (422), `svg_not_supported` (422), `asset_in_use` (409), `sync_in_progress` (409), `sync_not_running` (409, cancel finished run), `git_repo_not_found` (422), `git_credentials_invalid` (422), `url_not_allowed` (422, `framework/httpclient` SSRF guard), `zip_invalid` (422), `zip_too_large` (413), `embedding_key_unsupported` (422), `embedding_model_invalid` (422), `reembed_in_progress` (409), `ai_unavailable` (409, Ask AI `/api/answers`), `ask_rate_limited` (429), `help_center_disabled` (404 on public), `help_center_not_linked` (404, widget Help routes when widget has no help center or Help tab off), `human_review_required` (403, accept/publish reached through MCP surface or third-party OAuth client other than `dimmi-cli`), `no_hunks_selected` (422), `no_changes` (422), `apex_not_supported` (422), `precondition_required` (400, session PATCH without If-Match), `redirect_shadows_article` (422), `redirect_shadows_section` (422, path reserved by a public section provider: `/changelog*`, `/feedback*`, `/roadmap*`), `redirect_loop` (422), `article_not_found` (404, widget deep open), `base_version_mismatch` (422), `github_token_invalid` (422), `github_token_expired` (422), `github_repo_not_found` (422), `github_permission_missing` (422, `missing[]`), `github_branch_not_found` (422), `github_connection_required` (422), `github_connection_in_use` (409), `github_repository_in_use` (409), `github_repository_exists` (409), `github_unavailable` (502), `github_pr_not_merged` (422), `docs_sync_source_exists` (409), `docs_sync_source_required` (422, freshness fix without source for repo), `docs_sync_run_in_progress` (409), `docs_sync_run_finished` (409), `reapply_not_supported` (422), `edits_unmatched` (409, reapply matched zero edits), `source_ref_limit_reached` (422), `spend_cap_reached` (423, 07, manual docs sync run), `docs_sync_disabled` (404, instance kill switch `docs_sync.enabled false`), `site_requires_javascript` / `site_requires_login` / `robots_disallowed` / `robots_unreachable` (source errors, not HTTP). Warnings (non-blocking, in `meta.warnings`): `missing_alt`, `broken_link`, `heading_skip`, `empty_body`, `long_title`, `missing_description`, `external_http_image`, `missing_asset`, `collection_missing`, `collection_depth_trimmed`, `possible_secret` (accept needs `acknowledge_warnings: true`), `dims_not_indexed`, `low_contrast`, `classic_token_broad_scope`, `webhook_manual_setup`, `edit_unmatched` (reapply), `docs_ref_unresolved` (run warning).

## MCP tools and CLI

Authenticated MCP (09 server, OAuth or token; tools in `app/mcp/tools`). Schemas from same request structs.

| Tool | Ability | Does |
|---|---|---|
| dimmi_get_help_center_outline | articles:read | help centers, collection tree, article titles/slugs/status/visibility, word counts. Agent reads first to avoid duplicates |
| dimmi_list_articles | articles:read | filtered list |
| dimmi_get_article | articles:read | Markdown working copy + published version id (`base_version_id` for edits) |
| dimmi_search_knowledge | articles:read | `contracts.Knowledge.Search` over everything incl. agent_only |
| dimmi_propose_article | articles:write | suggestion kind create |
| dimmi_propose_article_edit | articles:write | suggestion kind edit with full proposed Markdown + base_version_id |
| dimmi_propose_help_center | articles:write | batch (cold start), up to 200 items |
| dimmi_list_article_suggestions | articles:read | review queue status |
| dimmi_list_knowledge_sources | settings:read | |
| dimmi_sync_knowledge_source | settings:write | trigger sync (`POST /knowledge/sources/{ks}/syncs`; tool keeps task name, rest.md rule 15) |
| dimmi_list_docs_sync_runs | articles:read | toolset `docs_sync`: runs with PR, status, skip reason, summary, review URL (`GET /docs-sync-runs`) |
| dimmi_list_stale_articles | articles:read | toolset `docs_sync`: `GET /articles?filter[freshness]=needs_review` with reasons |
| dimmi_request_docs_update | articles:write | toolset `docs_sync`: `POST /docs-sync-runs` (manual PR/range, redraft, freshness fix); spends workspace BYOK; never accepts |

No MCP tool publishes, accepts, or deletes articles, even behind 09 confirm gate (09 resolved, Contradiction 15: 08 rule wins). Only carve-out: 16 `dimmi_run_import` with `articles.publish: preserve` (migrated human-written content) after 09 confirm gate (review-queue.md rule 2). Human gate stays in dashboard, REST (session, PAT, first-party app) and CLI (`dimmi-cli`, interactive confirm). MCP may propose and create drafts only (nothing goes live).

MCP catalog (09 owns MCP runtime and canonical tool catalog, `09-agent-native/mcp.md`): 09 `dimmi_suggest_article` (= 08 `dimmi_propose_article` + `dimmi_propose_article_edit`), `dimmi_list_articles`, `dimmi_get_article`, `dimmi_search_knowledge` call 08 actions above. 08-only tools (`dimmi_get_help_center_outline`, `dimmi_propose_help_center`, `dimmi_list_article_suggestions`, `dimmi_list_knowledge_sources`, `dimmi_sync_knowledge_source`) and prompt `dimmi_draft_help_center` live in 09 toolset `help_center`. Resolved with 09: no `dimmi_publish_article`, `dimmi_delete_article` or accept tool; publish stays dashboard/REST/CLI. MCP prompt `dimmi_draft_help_center` (review-queue.md) gives user's agent full instructions. Toolset `docs_sync` (09 registry; docs-sync.md 11): 3 tools above + prompt `dimmi_update_docs_for_change` (local coding agent proposes edits from local diff, zero server LLM). Pending doc updates = `dimmi_list_article_suggestions` with `source_kind: commit`. No MCP tool accepts, reapplies-and-accepts, publishes, or marks articles verified.

Public MCP per help center (`/mcp` on help center host): `dimmi_search_docs`, `dimmi_get_doc`, `dimmi_list_docs`. See rendering.md.

Server commands (artisan style, this spec owns): `dimmi knowledge:reembed`, `dimmi knowledge:sync`, `dimmi knowledge:reindex`, `dimmi help-center:cache-clear`, `dimmi help-center:rerender`, `dimmi help-center:domains:check`, `dimmi docs-sync:replay`, `dimmi github:check`.

User CLI (`npx dimmi`, implemented by 09, wraps REST defined here): `dimmi docs push <dir> [--help-center hc_] [--batch-title]` (Markdown folder to suggestion batch, no LLM), `dimmi docs pull <dir>` (export published Markdown with front matter), `dimmi docs publish <slug>` (`POST /articles/{art}/publications`; `dimmi-cli` passes `RequireHumanReview`, interactive confirm), `dimmi docs suggestions [--source commit] [--accept-all sugb_]` (interactive confirm), `dimmi docs sync runs|run --pr N`, `dimmi docs stale`, `dimmi docs verify <slug>` (interactive confirm), `dimmi github repos add owner/name` (docs-sync.md 11).

## Events

Emitted (catalog rows exist): `article.published`, `article.updated` (published content changed: new published version), `article.unpublished` (reason: `reverted_to_draft`, `archived`, `deleted`, `made_agent_only`, `help_center_deleted`), `article_suggestion.created`.

New events (add to catalog):

| Event | Emitted by | Realtime | Webhook | Notes |
|---|---|---|---|---|
| article.created | 08 | yes (workspace) | yes | draft created (any source) |
| article.deleted | 08 | yes | yes | trash; `purged: true` on hard purge |
| article_suggestion.reviewed | 08 | yes | yes | status accepted, partially_accepted, rejected; hunk counts; reviewer |
| knowledge_source.sync_completed | 08 | yes | yes | stats; status succeeded or partial |
| knowledge_source.sync_failed | 08 | yes | yes | error code |
| help_center.updated | 08 | yes | no | settings, domain primary change (dashboard refresh, cache bust); payload `changes` includes `status` and `deleted` (14 listener) |
| help_center.domain_status_changed | 08 | yes | yes | from/to status, error code (devs automate DNS) |
| knowledge.embeddings_switched | 08 | yes | no | new active model, coverage |
| docs_sync_run.completed | 08 | yes | yes | run resource: status `completed` or `skipped` (+ `skip_reason`), suggestion_count, batch_id, PR ref; no code |
| docs_sync_run.failed | 08 | yes | yes | run resource + error_code |
| article.freshness_changed | 08 | yes | yes | `{article_id, from, to, reasons: [{code, severity}]}` |
| github_repository.webhook_status_changed | 08 | yes | no | from/to, error code |
| github_connection.errored | 08 | yes | no | `github_token_invalid`, `github_token_expired`, `github_token_expiring` (06 system alert, requested) |

Realtime channel: `workspace:{ws}` (centrifuge namespace, `framework/broadcasting`; Redis broker on Valkey; reconnect recovers via channel history) for all above. Payload Resource JSON (suggestion resource without hunks if over 10 KB, per foundation rule).

Consumed:

| Event / hook | Listener | Queue |
|---|---|---|
| workspace initializer hook (contracts.md) | CreateDefaultHelpCenter | inside `CreateWorkspace` tx |
| ai.provider_key_changed (07; created, rotated, deleted) | SuggestEmbedderOnKeyChange | `ingest` (queued) |
| article.published, article.updated, article.unpublished, article.deleted | BumpHelpCenterCacheOnChange | sync after commit |
| same | IndexArticleKnowledge | FTS in publish tx (called by action directly), embeddings queued `ingest` |
| article.deleted | SupersedeSuggestionsOnDelete | sync in tx |
| article.published, article.updated | RecordArticleVerificationOnPublish (freshness baseline heads) | `default` (queued) |
| knowledge git / docs sync / 13 changelog source created or deleted (hook, not event) | SyncRepositoryWebhookEvents | `default` (queued) |
| GitHub deliveries (shared route, registry not event) | `knowledge_git` (push), `docs_sync` (pull_request, push); 13 `changelog` (release) | `default` (`HandleGithubDelivery`) |
| `AcceptSuggestion` (docs sync or refs-bearing suggestion; call, not event) | RecordSourceRefs + freshness baseline | same tx |
| system.app_url_changed (11) | RebuildHelpCenterUrls | `default` (queued) |
| 03 contact eraser registry (not event) | EraseHelpCenterContactData | inside 03 `erase_contact` job |
| help center delete/restore (hook, not event) | registered `LifecycleHook`s (13 `changelog.OnHelpCenterDeleted` / `OnHelpCenterRestored`) | same tx as `DeleteHelpCenter` / restore |

07 consumes nothing from here by event; it calls `contracts.Knowledge.Search` at answer time (always fresh).

## Jobs

| Job (kind) | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| CrawlKnowledgeSource (`crawl_knowledge_source`) | ingest | sync start (website) | 3 | unique per source while queued/running |
| FetchKnowledgePage (`fetch_knowledge_page`) | ingest | crawl frontier | 3, backoff 30 s, 2 min, 10 min; honors 429 `Retry-After` up to 1 h | unique (source, url, run) |
| FinishKnowledgeSync (`finish_knowledge_sync`) | ingest | last page done (counter) or watchdog | 5 | unique per run |
| SyncGitKnowledgeSource (`sync_git_knowledge_source`) | ingest | sync start (git), webhook push | 3 | unique per source; webhook during run sets `rerun_requested` |
| IngestKnowledgeUpload (`ingest_knowledge_upload`) | ingest | upload | 1 (deterministic input) | unique per upload object |
| IndexKnowledgeDocument (`index_knowledge_document`) | ingest | fetched/changed doc | 5 | unique (document, content_hash) |
| EmbedKnowledgeChunks (`embed_knowledge_chunks`) | ingest | after index, reembed | 8, honors provider Retry-After; `auth`/`quota` errors stop and set embeddings_status degraded | unique (workspace, model_key, batch hash) |
| ReembedWorkspaceKnowledge (`reembed_workspace_knowledge`) | ingest | settings change, `knowledge:reembed` | 3 | unique per workspace |
| CreateImportSuggestions (`create_import_suggestions`) | ingest | import-mode run finished | 3 | unique per run |
| DeleteKnowledgeSourceData (`delete_knowledge_source_data`) | maintenance | source deleted | 5 | unique per source |
| ScheduleKnowledgeSyncs (`schedule_knowledge_syncs`) | maintenance | periodic every 5 min | 0 | periodic |
| VerifyHelpCenterDomain (`verify_help_center_domain`) | default | add, manual check (`POST .../domains/{hcd}/checks`), schedule | 0 (schedule handles retry with `framework/customdomain` backoff 1, 2, 5, 10, 30 min then hourly 72 h) | River unique per domain (no Valkey lock) |
| RecheckHelpCenterDomains (`recheck_help_center_domains`) | maintenance | periodic daily | 0 | periodic |
| ProcessHelpCenterAsset (`process_help_center_asset`) | default | upload | 3 | unique per asset |
| RerenderArticleVersions (`rerender_article_versions`) | maintenance | renderer_version bump on boot, command | 3 | unique per workspace |
| PurgeHelpCenterData (`purge_help_center_data`) | maintenance | daily: trash older 30 days (articles, help centers), orphan assets (unreferenced after 24 h), embeddings not referenced by any chunk for 7 days, old model embeddings 7 days after switch, sync runs beyond 50, searches beyond retention, docs sync runs beyond 180 days. `RetentionGuard` (below) before each destroy class | 3 | periodic |
| HandleGithubDelivery (`handle_github_delivery`) | default | shared webhook route | 5 | unique (repo, delivery id) |
| SyncRepositoryWebhook (`sync_repository_webhook`) | default | consumer added/removed, repo create | 3 | unique per repo |
| CheckGithubConnections (`check_github_connections`) | maintenance | periodic daily | 0 | periodic |
| CheckGithubRepositories (`check_github_repositories`) | maintenance | periodic daily, `POST .../checks` | 0 | periodic; unique per repo for manual |
| RunDocsSync (`run_docs_sync`) | ai | run created (webhook, poll, push window, manual) | 3, backoff 1, 5, 15 min; rate-limit snooze until reset (max 1 h); timeout 10 min | unique per run; stages checkpointed, diff re-fetched (never stored) |
| ProcessDocsSyncPush (`process_docs_sync_push`) | default | push debounce end (`docs_sync.push_debounce` 10 min) | 3 | unique per source |
| PollDocsSyncSources (`poll_docs_sync_sources`) | maintenance | periodic 15 min (no-webhook repos) + 6 h reconcile (all); also run watchdog (`running` 30 min -> `failed stalled`) | 0 | periodic |
| ScheduleFreshnessScans (`schedule_freshness_scans`) | maintenance | periodic daily (weekday spread per help center) | 0 | periodic |
| ScanArticleFreshness (`scan_article_freshness`) | default | schedule, `POST /help-centers/{hc}/freshness-checks` | 3 | unique per help center |
| VerifyArticleClaims (`verify_article_claims`) | ai | scan, newly code-flagged article within weekly cap | 3 | unique (article, until_sha) |
| RecordArticleVerification (`record_article_verification`) | default | publish, member verify | 3 | unique (article) |
| BackfillArticleSourceRefs (`backfill_article_source_refs`) | default | repository created | 3 | unique per repo |
| FlushHelpCenterStats | in-process ticker 30 s, not River; flushes to Postgres (durable stats; never Valkey) | | on failure keep buffer, cap 10k keys then drop oldest + metric | |

Retention rule (`00-foundation/contracts.md` `RetentionGuard`, extensibility.md section 8): `PurgeHelpCenterData` and `PurgeHelpCenter`/`PurgeArticle` call `RetentionGuard.MayDestroy` before destroying customer data, per workspace batch: `{workspace, ws}` first (workspace hold skips whole batch); search log prune additionally `{contact, contact_id}` for rows with `contact_id` (held contact: rows kept). MVP driver noop (always true). Held: skip, `dimmi_retention_held_total{kind}`, log `retention_held`. Not guarded (derived or operational data, rebuildable): orphan embeddings, old-model embeddings, sync run history beyond 50, docs sync runs beyond 180 days, unreferenced assets never published. Feature test per destroy class with fake guard returning false. `DeleteKnowledgeSourceData` (member deleted source: user intent, not retention) and contact eraser (03 `EraseContact` already guarded) not re-checked.

Long crawls: one job per page, progress via `knowledge_sync_runs.stats` counters (atomic `jsonb_set` increments in small tx). Watchdog: run with no progress for 30 min marked `failed` (`stalled`).

## Import helpers (16)

Launch: importers. 08 owns, 16 writers call inside 16 transaction. Code `app/actions/helpcenters/importing/` (16 README says `helpcenters/importing`: 08 package name wins). No events, no notifications, no billing (16 suppression rule; arch test: package imports no `app/events`). Idempotency by 16 `import_records`; helpers trust caller lookup.

- `ImportCollection(ctx, tx, ImportCollectionInput{HelpCenterID, ParentID, Name, Description, Position})`: two levels; deeper source trees flattened into sub-collection named `A / B`.
- `ImportArticle(ctx, tx, ImportArticleInput{HelpCenterID, CollectionID, Title, Slug, Description, BodyMarkdown, Visibility, CreatedAt, UpdatedAt, PublishedAt, FirstPublishedAt, Publish})`: working copy (`last_edited_by_type system`), version `kind snapshot` `created_by_type import`. Body already converted by 16 (`importkit/htmlmd`) to 08 dialect, validated with `helpmarkdown.Help()`; invalid -> error, 16 records import error. Slug collision: collision suffix (`-2`); old source URL covered by `ImportRedirect`. `Publish` true only when run option `articles.publish = preserve` and source article published: same publish internals as `PublishArticle` (render, sanitize, version `published`, knowledge index in tx, embeddings queued) with events off and `published_at` / `first_published_at` preserved. Else draft (default `articles.publish = draft`).
- `ImportAsset(ctx, tx, ImportAssetInput{HelpCenterID, Filename, ContentType, Body})`: `app/support/images` processing (sniff, caps, metadata strip, `a.Scanner`); dedupe `(help_center_id, sha256)`; `uploaded_by_type system`.
- `ImportRedirect(ctx, tx, ImportRedirectInput{HelpCenterID, FromPath, Target})`: `source import`; provider-reserved prefix (`/changelog*`, `/feedback*`, `/roadmap*`) -> `ErrRedirectPathReserved` (16 error code `redirect_path_reserved`; manual REST create keeps 422 `redirect_shadows_section`); same path same target = no-op; same path other target = error (existing row wins).
- Crawl reuse: 16 `public_site` driver calls `app/support/crawl` (sitemap, robots, politeness, canonicalization) and `framework/dom` (`Readable`, `ToMarkdown`) through `a.HTTP`; no second crawler. Pages cap entitlement `imports.public_site_pages` (16), separate from `knowledge.crawl.max_pages_cap`.
- Publish rule (World-class bar row): only path that publishes without review queue; requires human-set `articles.publish: preserve` (session/app, or MCP `dimmi_run_import` after 09 confirm gate); audit `import_run.started` records option.

## Config keys

`config/helpcenter.go` (env `DIMMI_HELP_CENTER_*`):

| Key | Default | Notes |
|---|---|---|
| help_center.base_domain | cloud `dimmi.help` [domain to register; open question], self-host empty | empty = path mode only |
| help_center.path_mode | true | `{app_url}/hc/{subdomain}` |
| help_center.max_per_workspace | 0 | self-host value for entitlement key (0 = unlimited); cloud default in catalog (below) |
| help_center.max_articles | 0 | self-host value for entitlement key, per help center; cloud default in catalog |
| help_center.max_collections | 200 | |
| help_center.article_max_kb | 256 | |
| help_center.cache.max_mb | 32 | in-process page cache (L1 otter) |
| help_center.cache.ttl | 10m | safety TTL; bust on change via Valkey pubsub. Max staleness when a bust is lost (Valkey down) = this TTL (13 E178 relies on it) |
| help_center.domains.enabled | true | |
| help_center.domains.cname_target | cloud `domains.dimmi.help`, self-host `app.url` host | |
| help_center.domains.a_records | [] | apex support: IPs users point A/AAAA to |
| help_center.domains.max_per_help_center | 1 | |
| custom_domains.default + custom_domains.tls.<name>.driver | framework default `none`; self-host `caddyask` when compose Caddy profile, else `certmagic` if binding 443 (HTTPS listener: framework follow-up), else `none`; cloud `certmagic` | `framework/customdomain` TLS driver (`certmagic` on caddyserver/certmagic v0.25.6 embedded, `caddyask`, `none`); framework keys, not `help_center.tls.*` |
| custom_domains.tls.certmagic.email | from `mail.from` | ACME account email |
| custom_domains.tls.certmagic.ca | Let's Encrypt production | staging for tests |
| (no key) TLS storage | `postgres` | certmagic storage in `certmagic_storage` table (framework migration set; shared across replicas; certs durable, never Valkey: ACME rate limits) |
| help_center.ask.per_ip_per_minute | 10 | `ratelimit.Limiter` on Valkey (all limits below too) |
| help_center.ask.per_ip_per_day | 60 | Valkey counter; flush resets window (07 spend cap in Postgres stays hard guard) |
| help_center.ask.per_help_center_per_day | 1000 | protects BYOK spend |
| help_center.search.per_ip_per_minute | 120 | |
| help_center.public_mcp.per_ip_per_minute | 60 | |
| help_center.search_log_retention_days | 90 | |
| help_center.assets.max_mb | 10 | |
| help_center.assets.max_pixels | 40000000 | decompression bomb guard |
| help_center.suggestions.batch_max_items | 200 | |
| help_center.suggestions.batch_max_mb | 10 | route-specific body limit |

`config/knowledge.go` (env `DIMMI_KNOWLEDGE_*`):

| Key | Default | Notes |
|---|---|---|
| knowledge.max_sources | 0 | self-host value for entitlement key (0 = unlimited) |
| knowledge.max_chunks | 0 | self-host value for entitlement key, per workspace |
| knowledge.crawl.max_pages | 1000 | per source default, user may lower; ceiling = entitlement `knowledge.crawl.max_pages_cap` |
| knowledge.crawl.max_depth | 6 | |
| knowledge.crawl.rate_per_host | 1/s | Crawl-delay honored up to 10 s |
| knowledge.crawl.concurrency_per_host | 2 | |
| knowledge.crawl.max_page_mb | 5 | |
| knowledge.crawl.user_agent | `DimmiBot/1.0 (+https://trydimmi.com/bot)` | robots token `DimmiBot` |
| knowledge.git.max_tarball_mb | 200 | |
| knowledge.git.max_files | 5000 | matching path filter |
| knowledge.git.max_file_kb | 1024 | |
| knowledge.upload.max_zip_mb | 50 | |
| knowledge.upload.max_uncompressed_mb | 200 | |
| knowledge.upload.max_files | 2000 | |
| knowledge.chunk.target_tokens | 400 | |
| knowledge.chunk.max_tokens | 800 | |
| knowledge.embed.batch_size | 64 | inputs per provider call |
| knowledge.embed.concurrency_per_workspace | 2 | |
| knowledge.embed.query_timeout | 1500ms | agent; FTS-only after |
| knowledge.embed.query_cache_size | 10000 | L1 LRU (workspace, model_key, normalized query); L2 Valkey `cache.Store` TTL 24 h (shared across replicas; rebuildable) |
| knowledge.embed.supported_dims | [384, 512, 768, 1024, 1536] | indexed dims; others exact scan only |
| knowledge.search.exact_max_chunks | 20000 | below: exact KNN; above: HNSW |
| knowledge.search.rrf_k | 60 | |
| knowledge.search.candidates | 50 | per retriever |
| knowledge.search.hnsw_ef_search | 100 | |

`config/docssync.go` (env `DIMMI_DOCS_SYNC_*`) and `config/github.go` (env `DIMMI_GITHUB_*`):

| Key | Default | Notes |
|---|---|---|
| docs_sync.enabled | true | instance kill switch (routes 404 `docs_sync_disabled`, handlers ignore deliveries) |
| docs_sync.max_sources | 0 | self-host value for entitlement key (0 = unlimited) |
| docs_sync.runs_daily | 0 | self-host value for entitlement key (0 = unlimited) |
| docs_sync.max_pr_files | 300 | files fetched per PR (3 pages) |
| docs_sync.max_diff_tokens | 12000 | screen digest budget (3.5 chars/token) |
| docs_sync.max_article_tokens | 16000 | above: matched sections only |
| docs_sync.max_candidates | 8 | |
| docs_sync.min_search_score | 0.35 | fused normalized score floor for search candidates |
| docs_sync.max_drafts_per_run_cap | 10 | ceiling for source `max_drafts_per_run` |
| docs_sync.user_facing_path_hints | list in docs-sync.md 6.3 | digest file scoring |
| docs_sync.push_debounce | 10m | |
| docs_sync.poll_interval | 15m | repos without active webhook |
| docs_sync.reconcile_interval | 6h | all docs sync repos (lost deliveries) |
| docs_sync.run_timeout | 10m | |
| docs_sync.run_retention_days | 180 | |
| docs_sync.freshness.scan_interval | 168h | per help center |
| docs_sync.freshness.max_fallback_calls | 100 | per scan, truncated compare fallback |
| github.webhook_body_max_mb | 25 | GitHub max payload |
| github.delivery_dedupe_ttl | 24h | Valkey fast path |
| github.rate_limit_floor | 100 | remaining below: snooze until reset |
| github.api_version | `2022-11-28` | `X-GitHub-Api-Version` pinned (13 shares) |

### Entitlement keys (catalog `app/support/entitlements/catalog.go`, contracts.md `Entitlements`)

Cloud caps live here, not in config branches. 12 cloud driver catalog references these keys. Call sites read `Entitlements.Limit(ctx, ws, key)` (-1 = unlimited). Self-host driver (11) maps config value (0 = unlimited -> -1). Cloud driver (12): catalog default, then contract, then `entitlement_overrides`. Over limit: existing rows kept, only creation refused.

| Key | Kind | Self-host | Cloud default | Enforced in | Error |
|---|---|---|---|---|---|
| `help_center.max_per_workspace` | limit | config (0) | 3 | `CreateHelpCenter` (count non-deleted, `FOR UPDATE` on workspace row) | 422 `entitlement_limit_reached` |
| `help_center.max_articles` | limit | config (0) | 5000 | `CreateArticle`, batch accept of create suggestions (per help center) | 422 `entitlement_limit_reached` |
| `knowledge.max_sources` | limit | config (0) | 25 | `CreateKnowledgeSource`, upload creating new source | 422 `entitlement_limit_reached` |
| `knowledge.max_chunks` | limit | config (0) | 200000 | ingestion: chunks beyond limit not embedded, document `error` `chunk_limit_reached`, `knowledge.sync_completed` warns | none (async) |
| `knowledge.crawl.max_pages_cap` | limit | -1 | 5000 | source config validation (`max_pages` <= cap) | 422 field error |
| `docs_sync.max_sources` | limit | config (0) | 10 | `CreateDocsSyncSource` (count non-deleted, `FOR UPDATE` on workspace row) | 422 `entitlement_limit_reached` |
| `docs_sync.runs_daily` | limit (daily, UTC) | config (0) | 200 | gate step 5 (runs reaching stage `fetch`); `POST /docs-sync-runs` | async: run `skipped daily_cap_reached`; REST 429 `entitlement_limit_reached` + `Retry-After` |

`GET /workspace/entitlements` `used` provided by 08 for `help_center.max_per_workspace` (count), `knowledge.max_sources` (count), `docs_sync.max_sources` (count), `docs_sync.runs_daily` (today count); chunk count from `knowledge_settings` cached counter.

## Permissions matrix

Implemented as permissions (`00-foundation/extensibility.md` section 4, P2). Policies (`help_center_policy.go`, `article_policy.go`, `collection_policy.go`, `article_suggestion_policy.go`, `knowledge_source_policy.go`) call `authz.Can(principal, permission)`; role columns derived from 00-accounts `roles.go`, never checked in 08 code. Token effective permissions = role permissions intersect expanded abilities (member token holding `settings:write` still lacks `help_centers:manage`).

08 permissions (registered in 00-accounts catalog `app/auth/permissions.go`; each in exactly one ability group):

| Permission | Ability group | Roles (roles.go) | Gates |
|---|---|---|---|
| `articles:read`, `articles:write` | same-named | owner, admin, member | content, collections, redirects, assets, suggestions, previews, search |
| `help_centers:read_settings` | settings:read | owner, admin, member | domains list, knowledge sources/syncs/documents/settings read |
| `help_centers:manage` | settings:write | owner, admin | help center create/update/settings/toggles, cache bust, domains add/remove/check |
| `help_centers:delete` | settings:write | owner | help center delete |
| `knowledge:manage` | settings:write | owner, admin | sources create/update/delete, uploads, embedding model |
| `knowledge:sync` | settings:write | owner, admin, member | trigger/cancel sync, exclude/include document |
| `github:read` | settings:read | owner, admin, member | GitHub connections, repositories, accessible repos list |
| `github:manage` | settings:write | owner, admin | connection create/rotate/delete/test, repository add/update/delete/check, webhook secret rotate |
| `docs_sync:manage` | settings:write | owner, admin | docs sync sources create/update/pause/delete |

| Action | Owner | Admin | Member | Permission (token ability group) |
|---|---|---|---|---|
| View help centers, articles, versions, suggestions, stats | yes | yes | yes | articles:read |
| Create/edit/publish/unpublish/archive/delete articles, collections, redirects, assets | yes | yes | yes | articles:write |
| Propose suggestion | yes | yes | yes | articles:write (also MCP) |
| Accept/reject suggestion or batch, publish | yes | yes | yes | articles:write; refused through MCP surface and third-party OAuth clients (`human_review_required`, review-queue.md rule 2) |
| Create/update help center, settings, toggles | yes | yes | no | `help_centers:manage` (settings:write) |
| Delete help center | yes | no | no | `help_centers:delete` (settings:write) |
| Domains add/remove/check | yes | yes | no | `help_centers:manage` (settings:write) |
| View domains, knowledge sources, runs, documents, knowledge settings | yes | yes | yes | `help_centers:read_settings` (settings:read) |
| Knowledge sources create/update/delete, uploads | yes | yes | no | `knowledge:manage` (settings:write) |
| Trigger/cancel sync, exclude document | yes | yes | yes | `knowledge:sync` (settings:write) |
| Knowledge settings (embedding model) | yes | yes | no | `knowledge:manage` (settings:write) |
| Retrieval debugger `/knowledge/search` | yes | yes | yes | articles:read |
| View GitHub connections, repositories | yes | yes | yes | `github:read` (settings:read) |
| Add/rotate/delete GitHub connection or repository | yes | yes | no | `github:manage` (settings:write) |
| Docs sync sources create/update/pause/delete | yes | yes | no | `docs_sync:manage` (settings:write) |
| View docs sync runs, skipped log, freshness | yes | yes | yes | articles:read |
| Manual docs sync run, redraft, draft fix, freshness check, source refs | yes | yes | yes | articles:write |
| Mark article verified, reapply docs sync suggestion | yes | yes | yes | articles:write; verify refused through MCP surface and third-party OAuth (`human_review_required`) |

Deactivated member: none.

## Performance budgets

See World-class bar. Mechanics:
- Body HTML precomputed per version; page render = templ execution with cached nav tree (per help center, per cache_version).
- Page cache: otter L1, key `(help_center_id, cache_version, path, variant)`, variants html/markdown. Hit path does no DB query except host map lookup (cached 60 s, busted via Valkey pubsub). Valkey holds only rebuildable state (busts, limiter windows, query embedding L2, webhook delivery dedupe); content, stats, certs, domains in Postgres. Valkey down: pages served from L1 until `help_center.cache.ttl`, limiters fail open (metric `dimmi_hc_ratelimit_bypass_total`) except Ask AI per-help-center day cap which falls back to Postgres `ai_runs` count (BYOK spend guard).
- Search: GIN `(workspace_id, search)` with btree_gin; trigram only when FTS returns under 3 results; vector: exact KNN under 20k chunks, HNSW iterative scan above.
- Docs sync: webhook route does signature + dedupe + enqueue only (no GitHub call, no LLM) for under 1 s ack. One `RunDocsSync` per change; GitHub calls per PR 2 to 5 (docs-sync.md 4.4); LLM calls 1 screen + up to `max_drafts_per_run` drafts, sequential, 07 workspace concurrency respected; rate-limit snooze instead of failure. Freshness scan: set-based SQL over published versions + stats, GitHub compare grouped by distinct verified sha.
- Benchmarks in CI (`tests/bench`): 10k articles help center, 100k chunks workspace among 10M chunks seeded synthetic; fail on regression over 10%.

## Observability

Metrics (Prometheus): `dimmi_hc_requests_total{route,status,cache}`, `dimmi_hc_ttfb_seconds{route,cache}`, `dimmi_hc_cache_bytes`, `dimmi_knowledge_search_seconds{mode,surface}`, `dimmi_knowledge_search_mode_total{mode}` (fallback rate visible), `dimmi_knowledge_embed_tokens_total{provider}`, `dimmi_knowledge_embed_errors_total{kind}`, `dimmi_knowledge_crawl_pages_total{result}`, `dimmi_knowledge_sync_duration_seconds{type}`, `dimmi_hc_domain_checks_total{result}`, `dimmi_hc_tls_issuance_total{result}`, `dimmi_suggestions_total{kind,source_kind,status}`, `dimmi_hc_ask_total{result}`. Docs sync, GitHub, freshness metrics: docs-sync.md section 14.
Logs: sync run summary (source id, counts, duration), domain transitions, TLS issuance, embedding model switch. Never log article bodies at info, queries at debug only, tokens never.
Traces: search spans per retriever (fts, trigram, vector, embed_query, fuse).
`dimmi doctor` adds: btree_gin + pgvector versions, HNSW indexes present, embeddings status per workspace, TLS mode reachable (ports 80/443 or Caddy ask), base_domain wildcard DNS check (Valkey reachability is foundation check).

## Security and privacy

- Rendering: `framework/markdown` (goldmark) raw HTML off; `framework/sanitize` policy (UGC + heading ids + alert classes, built in `app/support/helpmarkdown`); links `rel="nofollow noopener ugc"` only for external links on help center (own-domain links followed); schemes http, https, mailto, tel. Same renderer for widget and API previews. XSS corpus test.
- CSP on help center: `default-src 'self'; img-src 'self' https: data:; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'`. If `show_widget`, add widget origin to script-src/connect-src/frame-src. `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin`.
- Help center served on separate registrable domain (`base_domain`) on cloud, never under app domain: user content never shares cookie scope with dashboard.
- Provider CSP overrides apply only to provider-owned paths; base CSP never loosened site-wide (14 `frame-ancestors` on portal pages only).
- Agent-only content: never in public HTML, `.md`, sitemap, llms.txt, search API, widget API, public MCP, Ask AI answers, OG. 404 (not 403) on direct URL.
- Public surface scoping (help center search, `/api/search`, widget search, public MCP, Ask AI): `Visibility [public]`, `HelpCenterIDs [this help center]`, `ExcludeSources true`, hard-coded server side. Prevents cross-brand leak between help centers of same workspace and exposure of git/upload source text marked citable.
- Query embedding cache keyed by workspace (see knowledge.md section 5): no cross-tenant sharing. Tested with leak test that crawls every public route and greps for agent-only marker text.
- Drafts and unpublished changes never public. Preview only via authenticated API.
- Custom domains: TXT ownership required (prevents subdomain takeover of dangling CNAMEs pointed at shared target); TLS only for hostnames `active` or `pending` with ownership verified; ask endpoint on admin port/loopback only.
- Crawler: `framework/httpclient` (SSRF guard, `http_client.allow_private_networks` off), robots.txt RFC 9309 (temoto/robotstxt via framework), same-site scope, size caps, no cookies, no auth headers sent; user-supplied extra headers not allowed at MVP.
- GitHub tokens: one place (`github_connections.token_encrypted`, `encryption.Sealed` via `encryption.KeyManager`), never returned (last 4 shown), used only for `api.github.com` and `codeload.github.com`. All consumers (08 knowledge, 08 docs sync, 13) call `github.ClientFor` (never copy). Webhook secret per repository, constant-time compare, payload repo id must match row.
- Customer source code (docs sync): diff only in `RunDocsSync` / `VerifyArticleClaims` memory, secret-like lines redacted, sent only to workspace BYOK key through 07 `RunTask` (`PersistInput false`); never in Postgres, Valkey, River args, logs or traces (leak test). Private repo evidence links only in dashboard/API. PR text and diff untrusted (`ai.WrapUntrusted`, `linkguard`); output only proposals.
- Zip uploads: zip-slip, symlink entries rejected, ratio guard 100:1, file count cap, nested zip ignored.
- Prompt injection: ingested web pages and repo files are untrusted; 07 treats chunk text as data. Chunk text stored as Markdown; HTML stripped.
- Public endpoints (search, ask, MCP, feedback): rate limited per IP; Ask AI daily cap per help center; feedback dedupe per IP hash per article per day (Valkey `SET NX EX` 24 h, IP not stored; flush allows at most one extra vote per IP: stats only).
- Analytics: no cookies, no IP, no PII in search log. Bot user agents excluded from views.
- GDPR: only contact link is `help_center_searches.contact_id` (widget searches with session). Contact eraser `EraseHelpCenterContactData` deletes those rows (runs inside 03 `EraseContact`, which calls `RetentionGuard` first). Search queries may contain PII typed by visitors: retention 90 days (prune guarded, Jobs), purged with workspace.
- Audit log: help center create/delete, domain add/remove/check, knowledge source create/delete/credential change, embedding model change, batch accept, GitHub connection create/rotate/delete, repository add/delete/secret rotate, docs sync source create/update/delete, article verify.

## Metering impact

None billable (`research/pricing.md` section 2):
- Help center views, search, Ask AI, public MCP, widget Help tab: no conversation thread, so D1 never activates.
- Ask AI "Talk to human" opens normal widget conversation (01); billing per D1 by 03/12 as usual.
- Suggestion imports, crawls: no conversations; D11 spirit (imports free).
- 16 imports (articles, collections, assets, redirects via import helpers): never billable (12 D11); no events, no conversations.
- Embedding tokens and Ask AI tokens billed by user's own provider (BYOK, D6 spirit); ingestion embeddings recorded via 07 `RecordAiSpend` (kind `embedding`), Ask AI recorded by 07 `AnswerQuestion` as `ai_runs` kind `help_center`; both count toward 07 monthly AI spend cap. Embedding calls stop (FTS-only, `degraded`) when 07 spend cap hit; never spend Dimmi money.
- Cloud fair-use caps (entitlement keys `knowledge.max_sources`, `knowledge.max_chunks`, `knowledge.crawl.max_pages_cap`, `help_center.*`, `docs_sync.*`) protect Dimmi storage, queue and GitHub API budget; no charge; raised per customer by override (12), never paywalled.
- Docs sync (runs, suggestions, freshness, GitHub webhooks): no conversations, never D1. Screen/draft/verify tokens on workspace BYOK key via 07 `RunTask` (`ai_runs.kind docs_sync`, spend cap applies); query embeddings via `RecordAiSpend` kind `embedding`.

## Dependencies on other specs

| Spec | Need |
|---|---|
| 00-foundation / framework | `framework/httpclient` (SSRF), `filesystem.Filesystem` (`a.Storage.Disk`), `cache.Store` + `pubsub.PubSub` on Valkey (cache bust topic `help_center.cache`), `ratelimit.Limiter` on Valkey, `framework/markdown` + `framework/sanitize` + `framework/dom`, `framework/customdomain` + `framework/image` + framework `support/diff`, `httpclient/robots`, `markdown.FrontMatter` (catalog additions requested), `framework/routing` signed URLs, `framework/ids`, `framework/clock`, `framework/broadcasting` (centrifuge), RLS, idempotency; workspace initializer hook; events, ownership rows, `btree_gin` (all present); `Entitlements` contract + catalog (08 registers 5 keys), `RetentionGuard` |
| 00-accounts | workspace created hook for default help center; `authz.Can` + permission catalog (08 permissions registered there), roles.go; tokens; audit log |
| 01-messenger-widget | consumes `/widget/v1/help/*`; Help tab UI; "Talk to human" from Ask AI deep link (SDK `startConversation(text)`); deep open `show('help', {article})` (sdk-contract.md Views); `widget_settings.help_center_id` + `help_tab_enabled` (present in 01 data model) scope widget Help routes |
| 07-dimmi-agent | owns `ai_provider_keys`, key decryption (`ai.ResolveProviderKey`), AI spend cap, usage recording (`RecordAiSpend`), readiness (`CheckAgentReadiness`); provides `ai.AnswerQuestion` streaming for Ask AI (kind `help_center`, public-only retrieval enforced by 07); emits `ai.provider_key_changed`; consumes `contracts.Knowledge`. 07 provides `voyage` provider type (embeddings-only, never answer model) so Anthropic-only users get vectors |
| 09-agent-native | MCP server runtime (auth + public stateless mount, protocol versions), CLI `dimmi docs *` wrappers, OAuth scopes mapping to abilities. Caller identity for accept/publish refusal (`human_review_required`, review-queue.md rule 2) comes from what 09 defines, no token claim: principal `Actor.Via` (contracts.md; `app` = first-party Dimmi app `ogr_`, `oauth_client` + client id = OAuth client, `personal_token`, session) and request surface (`/mcp` mount sets surface `mcp`, recorded as `messages.metadata.sent_via.surface`; first-party tokens never reach `/mcp`, 403 `first_party_token_not_allowed`). `dimmi-cli` treated as human at keyboard. 09 confirm gate does not apply: no MCP tool publishes, accepts or deletes articles (09 Contradiction 15, 08 rule wins) |
| 18-audiences | `audiences`, `audience.Partial`, `access` mode, surfaces `help.article`, `help.collection`, `dimmi.knowledge`; 02 surface `help_center`; M7 adds columns and filters here |
| 10-webhooks | delivers new webhook events (incl. `docs_sync_run.*`, `article.freshness_changed`) |
| 11-self-host | Caddy compose profile with `on_demand_tls { ask http://dimmi:<admin_port>/internal/tls/ask }` (admin port, never public; 200 only for verified `help_center_domains`, else 404); event `system.app_url_changed` (08 rebuilds canonical/path-mode URLs); port 443 binding option; `dimmi doctor` hooks |
| 12-cloud-billing | none billable; cloud `Entitlements` driver + `entitlement_overrides` back the 08 cap keys (incl. `docs_sync.*`); metering-rules "Not activity" row for docs sync |
| 07 (docs sync) | `ai.RunTask` kind `docs_sync` (purposes `screen_change`, `draft_edit`, `draft_article`, `verify_article`), `ai_runs.kind` CHECK value, `RunTaskInput.PersistInput *bool` = false, `PreflightSpend`, `WrapUntrusted`, `linkguard.Filter` (docs-sync.md 16) |
| 09 (docs sync) | toolset `docs_sync` (3 tools + prompt), CLI `dimmi docs sync|stale|verify`, `dimmi github repos add`; MCP never verifies articles |
| 06 (requested) | system alert on `github_connection.errored`, repo webhook failing; destination trigger `docs_suggestions_ready` |

Provided to other specs (applied in this sweep):

| Spec | 08 provides |
|---|---|
| 13-changelog | `PublicSectionProvider` (nav, home "Latest updates", routes, sitemap child `/sitemaps/changelog-{n}.xml`, llms.txt `## Changelog`, search kind `changelog`); `AssetReferencer` (cover, OG, body refs; OG PNGs `uploaded_by_type system`); `knowledge.IndexChangelogEntry` / `RemoveChangelogEntry` (same tx as publish), `knowledge_documents.changelog_entry_id` (13 migration 001314), search option `ExcludeChangelog`, public scope keeps changelog; `dimmi_get_doc` resolves `changelog/{slug}` URLs, `dimmi_search_docs` item `kind`; `hc.js` bundles 13 module; shared GitHub layer: `github_repositories` (13 `changelog_sources.github_repository_id` replaces own repo/token/secret columns), `github.ClientFor` (replaces `knowledge.ResolveGitCredentials`), `github.RegisterWebhookHandler` (13 registers `release`; own `/webhooks/github/changelog/{cls}` route dropped; docs-sync.md 16); `LifecycleHook` for `OnHelpCenterDeleted` / `OnHelpCenterRestored`; shared `app/services/github` adapter. Help center subdomain change, disable, delete apply to changelog routes identically |
| 14-feedback-roadmap | `PublicSectionProvider` for portal routes (`/feedback*`, `/roadmap*`, `/api/feedback/*`), nav links, sitemap (`lastmod` = `last_activity_at`), robots `Disallow`, CSP overrides (portal pages only); reserved redirect paths (`redirect_shadows_section`); `knowledge.EmbedTexts`; `help_center.updated` payload `changes.status` / `deleted`; shared theme CSS + brand vars |
| 15-announcements-surveys | `helpmarkdown.Engage()` profile; shared `app/support/images`; widget Help deep open by article id (`show('help', {article})`, CTA `open_article`) |
| 16-importers | import write helpers (`ImportCollection`, `ImportArticle`, `ImportAsset`, `ImportRedirect`; section "Import helpers (16)"); `app/support/crawl` + `framework/dom` extract for `public_site` driver; redirect `source import`; `source_kind import` provenance; publish exception for `articles.publish: preserve` |

## Open questions (recommended default)

1. Cloud help center base domain: register separate domain (`dimmi.help` or similar) to isolate cookies. Default: yes, separate registrable domain; until registered, use `help.trydimmi.com` subdomains only if dashboard cookies are all `__Host-` (they are; CSRF via `http.CrossOriginProtection`, no XSRF cookie).
2. SVG images in articles: security.md says SVG attachment-only. Default: reject SVG at MVP; revisit with strict SVG sanitizer + served via `<img>` only.
3. Should `agent_only` chunks be quotable verbatim by agent? Default: yes, usable for answer, cited by title without link.
4. Voyage provider for Anthropic-only BYOK users: needs 07 key type. Default: add in 07 MVP (small driver).
5. Default `public_mcp_enabled` true? Default: true (content already public; rate limited). Owner can disable.
6. Docs sync open questions (PAT prefill URL, `not_affected` advancing freshness baseline, `push` trigger default, cloud runs/day): docs-sync.md section 17. Roadmap item 16 bar lines deferred (`conversation-drafts`, `docs-as-code-pr`): PO confirm.

## Decisions 2026-11-07

Built in this round (MVP slice), with these changes from the plan above:

- Hosted address: `{subdomain}.dimmi.help` on cloud (`help_centers.base_domain` config, settled per edition). Competitors use a separate domain for user content (`intercom.help`, `crisp.help`, `helpscoutdocs.com`), which keeps help pages off the app's cookies; `dimmi.site` is registered by someone else, `dimmi.help` was free on 2026-11-07. Self-host without a base domain serves `{app_url}/hc/{subdomain}`; with one, a kernel middleware answers `<sub>.<base>` as the `/hc/<sub>` pages with root-relative links. The owner must register `dimmi.help` and point a wildcard at the load balancer.
- Migrations renumbered to `20261209010001`..`05` (help_centers, collections, articles, article_versions, help_center_redirects) so fresh databases create them after the search configuration they use. Columns that reference unbuilt tables (assets, domains, audiences) are left out and come with those tables.
- Default help center: created by a workspace initializer (after the default widget, which it links) and lazily on the first `GET /help-centers` for workspaces made before. Subdomain = workspace slug, with a 4-letter suffix when taken or reserved.
- Public pages reuse the existing templ views: a per-request library reads a snapshot (collections + published public articles) cached in memory per `cache_version`. The sample help center answers only names no real help center holds (the `dimmi` demo, a reserved subdomain).
- Search: Postgres FTS on `article_versions.search_document` (generated, `dimmi_search` config) with `ts_headline` excerpts. No embeddings yet.
- Agent knowledge: `helpcenters.Knowledge` implements `contracts.Knowledge` over published articles (public and agent-only, never drafts): FTS picks up to 12 candidate articles matching any query word, then the existing BM25 section ranker picks passages with the article's public URL. Workspaces without published articles fall back to the seed index.
- Messenger Help tab: `GET /widget/v1/apps/{app}/help` returns collections and up to 200 articles with bodies as plain blocks (no HTML reaches the host page).
- Redirects: published slug changes (article or collection) answer 301 from the old path.
- Later: custom domains + TLS, assets/logo upload, Ask AI page, llms.txt / sitemap / `.md` negotiation beyond the article twin, analytics, review queue, knowledge sources, embeddings, docs sync, version diff/restore UI, multiple help centers in the dashboard.
