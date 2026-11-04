# 08 Help center: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Test policy (`00-conventions/testing.md`): app logic integration only (real App, real Postgres `pgvector/pgvector:pg18`, real Valkey with per-test key prefix, real dispatcher + River drained via `tc.Queue.Drain(ctx)`); no unit tests with mocks, no sqlmock/pgxmock/embedded-postgres. Fakes only at process boundaries: `httpclienttest.New(tc.App.HTTP)` (crawl sites, GitHub, embeddings providers), `llm.Fake()` (07), `framework/dns` fake resolver, `tc.Clock`, fake ACME (pebble in CI container). Pure funcs with no I/O (chunker, RRF math, hunk builder) may keep table tests without mocks. Tests named in plain words; every edge case covered. Scope trim 2026-11-02: changelog-purpose domain (B12), provider MCP tools (B13), 3-way rebase (B18) tasks removed; old E196-E210 renumbered to E194-E208. Docs sync (launch item 16) adds M11 (E209-E262); M5 git task and M10 exports retargeted to shared GitHub layer. Tests never call real LLM or network; prompt evals with real model only on explicit approval (cost estimate first).

## M1. Data and articles core

- [ ] Migrations 0801 to 0807 + 0818 (help_centers, assets, collections, articles, versions, suggestion tables, f_unaccent, btree_gin), sqlc queries, enums, prefixes `hc_ col_ artv_ sug_ sugb_ ast_` in `app/models/public_ids.go` (`framework/ids`, TypeID).
  - AC: `migrate` up/down clean; ids round trip tests; lint rule passes (workspace_id predicates).
- [ ] Factories (help center, collection, article with versions/published, suggestion) + seeder demo help center.
  - AC: `dimmi db:seed` creates 3 collections, 12 articles, 2 suggestions.
- [ ] `CreateHelpCenter`, `UpdateHelpCenter`, `DeleteHelpCenter`, `CreateDefaultHelpCenter` registered as workspace initializer (contracts.md hook), subdomain rules (reserved, collision suffix, holds).
  - Tests: cap via real `Entitlements` driver with seeded overrides (limit 3, -1, override 5; over: 422 `entitlement_limit_reached`), concurrent create at cap - 1 (one wins), reserved, collision, default reassignment, other-workspace 404; initializer idempotent on `dimmi workspace:repair` re-run.
- [ ] Register 08 entitlement keys in catalog (`help_center.max_per_workspace`, `help_center.max_articles`, `knowledge.max_sources`, `knowledge.max_chunks`, `knowledge.crawl.max_pages_cap`) with self-host config mapping (0 -> -1) + `used` providers for `GET /workspace/entitlements`; no `edition == cloud` branch in 08 code.
  - Tests: self-host driver unlimited; cloud defaults; lint/grep: no `app.edition` check in 08 cap paths.
- [ ] Register 08 permissions in 00-accounts catalog `app/auth/permissions.go` (`help_centers:read_settings`, `help_centers:manage`, `help_centers:delete`, `knowledge:manage`, `knowledge:sync`) + roles.go rows; policies use `authz.Can` only.
  - Tests: matrix table test per role and per token ability; member token with `settings:write` denied `knowledge:manage`, allowed `knowledge:sync`; no role names in 08 policies (lint).
- [ ] Help center REST controllers + requests + resource + policy.
  - Tests: feature per endpoint (happy, 422, 403 permission, ability, 404 cross-tenant, idempotency on POST).
- [ ] Collections: actions, depth trigger, order via `PATCH /help-centers/{hc}/collections {order, parent_id}` and `PATCH /collections/{col}/articles {order}`, delete modes; REST.
  - Tests: depth 3 rejected, order stable, `order_mismatch` 422 on missing/extra id, delete with children 409.
- [ ] Help center dialect `app/support/helpmarkdown/options.go` on `framework/markdown` (GFM, alerts, heading ids, toc via goldmark-toc, link/image rewrite, `Highlight`, `framework/sanitize` policy, warnings), profiles `Help()` + `Engage()` (15-announcements-surveys); plain text via `markdown.PlainText`. Needs framework catalog: alerts extension, `FrontMatter`.
  - Tests: golden files through real render; XSS corpus (E39); anchors dedupe; warnings each code; `Engage()` strips H1/H2, footnotes, embeds.
- [ ] Articles: create/update with lock_version, slug via `str.Slug` (gosimple/slug, transliteration) + collision suffix, content_hash, word count, title trigram.
  - Tests: If-Match 409, session missing If-Match 400, slug collisions, 256 KB limit.
- [ ] Publish/unpublish/archive/unarchive/delete/restore/move actions + versions (kinds, snapshot throttle) + events. REST: `POST /articles/{art}/publications` (201 `article_version`); `PATCH /articles/{art}` per-field dispatch (README "Article state": `status` draft/archived, `trashed: false`, `collection_id`/`position`/`help_center_id`, `restore_version_id`).
  - Tests: state machine table test over PATCH (every row incl. `status: published` 422 `invalid_transition`, same-status no-op, trashed 409 `article_trashed`, If-Match only for content fields); events dispatched; nothing_to_publish; versions immutable.
- [ ] Version list/show/diff endpoints (`app/support/textdiff` hunks on framework `support/diff`: go-udiff line + sergi/go-diff word); restore via `PATCH {restore_version_id, publish?, restore_slug?}`.
  - Tests: diff golden; restore with/without slug; restore publish; restore + content fields 422.
- [ ] Redirects: automatic on slug change, manual CRUD, loop detection, shadow check.
  - Tests: chain collapse by id targeting, loop 422, new article takes redirected slug, provider-reserved prefix 422 `redirect_shadows_section` (E191).
- [ ] Assets: upload via shared `app/support/images` on `framework/image` (sniff, dims, bomb guard, metadata strip, `a.Scanner`, SVG reject, dedupe by sha), alt text, delete in-use 409 via `AssetReferencer` registry, `ProcessHelpCenterAsset`, `a.Storage.Disk("public")` paths.
  - Tests: local disk driver in temp dir (`a.Storage`); png ok, svg 422, 50k x 50k png rejected, same file twice one row, asset referenced by registered referencer 409 (E194).

## M2. Public site (SSR, SEO, caching)

- [ ] `resolve_help_center` middleware: custom domain, base_domain subdomain, path mode, canonical 301 rules, host map L1 cache + `pubsub.PubSub` (Valkey) bust.
  - Tests: each host kind; non-primary 301 except api/mcp/static; disabled 404; suspended 503; bust across two App instances sharing Valkey.
- [ ] templ views: layout, home, collection (paginated), article, search, 404, 410; theme CSS (brand vars, contrast), system fonts, hashed static assets.
  - AC: Lighthouse CI 100/100/100/100 on seeded article page (mobile).
- [ ] SEO head: title, description, canonical, OG, Twitter, JSON-LD, noindex rules; sitemap.xml (+ index over 50k); robots.txt variants.
  - Tests: snapshot of head per page type; sitemap excludes agent_only/draft; robots for non-primary.
- [ ] Page cache middleware: otter L1 LRU by bytes, cache_version keys, singleflight, ETag/304, Cache-Control; `BumpHelpCenterCache` + listener; `help-center:cache-clear` command.
  - Tests: publish busts; 304 path; stampede test; bench TTFB p95 under 50 ms cached.
- [ ] `.md` twins, `Accept: text/markdown` negotiation, `Link` headers, llms.txt (H2 sections, Optional), llms-full.txt (5 MB split).
  - Tests: llms.txt parses per llmstxt.org rules (H1, blockquote, H2 link lists); agent_only never present (leak test crawls every public route).
- [ ] Feedback endpoint (form + JSON), view recording, stats buffer flush ticker, `help_center_article_stats_daily`, bot filter.
  - Tests: dedupe per IP per day (Valkey `SET NX EX`); bot UA excluded; flush to Postgres on shutdown.
- [ ] `RerenderArticleVersions` job + `help-center:rerender` command; renderer_version check at boot.

## M3. Knowledge index and keyword search

- [ ] Migrations 0810 to 0815 + 0817; sqlc; prefixes `ks_ ksr_ kd_ kc_` in `app/models/public_ids.go`; `knowledge_sources.type` registry enum (format CHECK + Go registry `app/integrations/knowledge/sources.go`, E200).
- [ ] Chunker (`chunk.go`): heading sections, packing, tables/code safe, merge small, anchors, embed_hash.
  - Tests: golden chunk sets for 10 fixture docs through real `IndexDocument`; CJK; huge code block split.
- [ ] `IndexArticle` inside publish tx (document + chunks + tsvector with workspace config); unpublish/visibility updates in same tx.
  - Tests: publish then search finds in same tx commit; agent_only visibility; archived removed.
- [ ] `contracts.Knowledge` impl keyword path: FTS SQL, prefix for instant, trigram fallback, filters, MaxPerDocument + adjacent merge, deterministic ties; `KnowledgeSearchResult.Timings`; `Document`, `Status`, `EmbeddingKeyInUse`, `PublicHosts`; `knowledge.Fake()` passes same `tests/contracts` suite as real impl (for consumers' boundary use only).
  - Tests: ranking fixtures; visibility filter; empty/stopword queries; injection chars.
- [ ] Public search page + `/api/search` (items carry `kind`) + widget search + search log (`help_center_searches`) + query cache 30 s (L1).
  - Tests: public never returns agent_only; zero-result logged; bench p95 under 100 ms at 100k chunks/10M total.
- [ ] Retrieval debugger `GET /api/v1/knowledge/search` + `knowledge.settings.show`.

## M4. Embeddings and hybrid

- [ ] `contracts.Embeddings` + manager (`framework/manager`) from 07 `ai_provider_keys`; drivers (adapter zone) openai, openaicompat (OpenRouter, Ollama), gemini, bedrock, voyage; HTTP via `a.HTTP.StdClient()`; normalization; dims validation.
  - Tests: `tests/contracts/embeddings_test.go` suite per driver with `httpclienttest.New(tc.App.HTTP)` recorded fixtures; key redaction; error kind mapping.
- [ ] Migration 0815 HNSW partial indexes per dims; `EmbedKnowledgeChunks` job (batching, hash dedupe, retry, quota stop, spend cap check via 07).
  - Tests: unchanged text never re-embedded; quota sets degraded; 429 snooze.
- [ ] Vector retriever exact + ANN (per-dims queries, iterative scan, statement timeout) + RRF fusion + query embedding cache (L1 + Valkey L2) + singleflight + timeout fallback.
  - Tests: RRF table tests (pure); hybrid ranking through real index; timeout fallback marks degraded; exact vs ANN switch at threshold; query embedding cache never hit across workspaces (two workspaces, same query, two provider calls).
- [ ] Knowledge settings: auto-pick embedder on first key (`SuggestEmbedderOnKeyChange` listener on `ai.provider_key_changed`), PATCH model, validate test embed, blue/green re-embed, dual-write during build, switch, rollback window, purge old vectors; `knowledge:reembed`, `knowledge:reindex` commands; event `knowledge.embeddings_switched`.
  - Tests: search uses old model during build; switch at 100%; switch back within 7 days instant; key deleted -> FTS-only.

## M5. Knowledge sources

- [ ] Sources CRUD (website, git), credentials `encryption.Sealed`, over cap 422 `entitlement_limit_reached`, sync runs table, `POST /knowledge/sources/{ks}/syncs` (start), `DELETE .../syncs/{ksr}` (cancel), `GET .../syncs[/{ksr}]` (history), scheduler `ScheduleKnowledgeSyncs`, watchdog, `DeleteKnowledgeSourceData`.
  - Tests: schedule picks due; overlap skip; delete mid-run; start while running 409; cancel finished 409 `sync_not_running`, cancel twice 200.
- [ ] `app/support/crawl`: robots via framework `httpclient/robots` (temoto/robotstxt; RFC 9309 vectors as our tests; 5xx = disallow, E202), sitemap (stdlib xml/gzip, index, 50k cap), URL canonicalization, globs, per-host politeness limiter (framework `ratelimit` local token-bucket mode, no direct `x/time/rate`), extraction `dom.Readable` + `dom.ToMarkdown`, charset `dom.DecodeCharset`. Needs framework catalog: `httpclient/robots`.
- [ ] Website crawl jobs: discovery, `FetchKnowledgePage` (conditional GET, size cap, charset, extraction, JS-rendered and auth-wall detection), `FinishKnowledgeSync` removals.
  - Tests: `httpclienttest.New(tc.App.HTTP)` site with 60 pages incl. robots rules, 429, 5xx, redirects to login, SPA shell, duplicate canonical; stats exact.
- [ ] `app/services/github` adapter (google/go-github/v93, `a.HTTP.StdClient()`, retries off; depguard adapter row) + git sync on shared GitHub layer (M11 first two tasks land before this one): `knowledge_sources.github_repository_id` (migration 0823), `github.ClientFor`, SHA check, tarball streaming redirect to codeload, path filters, caps, symlink skip, `knowledge_git` webhook handler (`push`, ref filter, River unique debounce).
  - Tests: `httpclienttest.New(tc.App.HTTP)` GitHub fixtures; private 404 hint (E94); force push; shared route bad signature 401 (E99); other branch ignored (E101); redelivery after Valkey flush enqueues no second run (E205).
- [ ] Upload: `POST /knowledge/uploads`, safe unzip (slip, bomb, symlink, count), front matter (`markdown.FrontMatter`, YAML + TOML), MDX stripping, replace semantics.
  - Tests: malicious zip corpus; mdx fixtures; TOML front matter + invalid front matter (E201).
- [ ] Import mode: `CreateImportSuggestions` (create/edit vs current working copy, change detection by `imported_hash`/archive), asset re-hosting for images, internal link rewrite.
  - Tests: Dimmi edit preserved across upstream change; deleted file -> archive suggestion.
- [ ] Source events `knowledge_source.sync_completed/failed`, admin email on credential failure (system stream Mailable to members, 24 h throttle), estimates on create.

## M6. Review queue and cold start

- [ ] `app/support/textdiff`: Markdown-aware hunk boundaries, word segments (go-udiff + go-diff via framework `support/diff`). No diff3. Needs framework catalog: `support/diff`.
  - Tests: property tests on pure hunk builder (apply all hunks == proposed; apply none == current); table/code block boundary tests; stale rule through real accept.
- [ ] Suggestion actions: propose create/edit/archive, dedupe, supersede, secret-pattern warning, update (reviewer edits / refresh resets base), `stale` computed at read and in accept tx.
- [ ] Accept/reject via `PATCH /article-suggestions/{sug} {status}` (per hunk, meta hunks, publish flag, row locks, warnings ack) + events + batch counters; `RequireHumanReview(principal)` refusal `human_review_required` (surface `mcp`, or `Via.Kind = oauth_client` other than `dimmi-cli`; review-queue.md rule 2).
  - Tests: session, first-party app (`Via app`), PAT, `dimmi-cli` allowed; CIMD/DCR OAuth client 403; any `/mcp` surface 403 (PAT too).
  - Tests: concurrent accept 409; partial accept lands `partially_accepted`, content exact; stale suggestion 409 `suggestion_stale`, refresh then accept ok; status + proposal fields 422; MCP context 403.
- [ ] Batches: create (200 items, 10 MB route limit, partial/atomic), `PATCH /article-suggestion-batches/{sugb} {status}` accept-all chunked / reject-all, derived batch `status`; review list fields; collection pending counters.
- [ ] MCP tools (`dimmi_get_help_center_outline`, `dimmi_list_articles`, `dimmi_get_article`, `dimmi_search_knowledge`, `dimmi_propose_article`, `dimmi_propose_article_edit`, `dimmi_propose_help_center`, `dimmi_list_article_suggestions`, `dimmi_list_knowledge_sources`, `dimmi_sync_knowledge_source`) + prompt `dimmi_draft_help_center`.
  - Tests: tool schemas from request structs; abilities enforced; no publish/accept/delete path reachable from any MCP tool, also with elicitation-capable client (E208, 09 Contradiction 15).
- [ ] CLI contract doc for 09 (`dimmi docs push|pull|publish|suggestions`, docs sync commands in docs-sync.md 11) + example script in repo `examples/docs-push`.

## M7. Custom domains and TLS

- [ ] Migration 0808; domain add (normalize, IDNA, platform/apex rules via `framework/customdomain`; claims + reserved list `app/support/customdomains`), DNS records response, remove. `certmagic_storage` ships in framework migration set (not 08). Needs framework catalog: `customdomain`.
  - Tests: second domain on same help center 422 `domain_limit_reached`; host claimed elsewhere 409 `domain_taken`.
- [ ] `VerifyHelpCenterDomain` (TXT, CNAME/A, HTTP probe route `/.well-known/dimmi-domain-check`, CAA) + backoff + daily recheck + failure email + events; `POST /help-centers/{hc}/domains/{hcd}/checks` (202, 1/10 s).
  - Tests: `framework/dns` fake resolver; each error code; dangling claim stale after 7 days; checks endpoint 202 then 429 within 10 s.
- [ ] Wire `framework/customdomain` TLS drivers (`certmagic` with Postgres `certmagic_storage` + advisory-lock `Locker`, `caddyask`, `none`; built in framework) with app authorizer `AuthorizeTLSHostname` (`foundation.Options.Domains`); admin-port route on `caddyask.Driver.AskHandler()`. certmagic needs HTTPS listener (framework follow-up).
  - Tests: driver suite lives in `framework/customdomain/drivertest` (pebble in CI container); app tests: ask endpoint 200/404 per status; concurrent issuance one cert (E207).
- [ ] `help-center:domains:check` command, `dimmi doctor` checks.

## M8. AI surfaces and widget

- [ ] Ask AI: `POST /api/answers` SSE + `/answers` form, rate limits on Valkey (IP minute/day, help center day; Postgres fallback for day cap), real 07 `AnswerQuestion`, citations filtering, logging, "Talk to us" via SDK `startConversation`.
  - Tests: real 07 action with `llm.Fake()` scripted stream; rate limit 429; ai_unavailable (incl. 07 cap reached); no-JS path; retrieval scope asserted from stored `ai_runs` steps (`[public]`, `HelpCenterIDs`, `ExcludeSources`); Valkey stopped: day cap still enforced (E206).
- [ ] Widget Help API (`/widget/v1/help/*`) + resources + widget_views. App-only auth (`app_id`, session optional), scoped to widget `help_center_id` (01).
  - Tests: agent_only 404; deep open target for `show('help', {article})` missing article 404 (E199); popular fallback; `app_id` required; anonymous (no session) 200 with public cache header; widget linked to A cannot read B article (404); unlinked or `help_tab_enabled=false` 404 `help_center_not_linked`; edge cases 5, 168.
- [ ] Audience columns, read filter, SSO, knowledge filter: built in 18 M7 after this spec, not here. Here: `customdomains.Claimed` covers `widget_domains` once table exists (01 M14).
- [ ] Contact eraser `EraseHelpCenterContactData` + `help_center_searches.contact_id` (widget session only).
  - Tests: widget search with session stores contact_id, public search null; erase deletes contact rows only, second run no-op; 03 registry test lists it; edge case 169.
- [ ] `RebuildHelpCenterUrls` listener on 11 `system.app_url_changed`.
  - Tests: event bumps every help center cache_version; path-mode canonical uses new host; edge case 170.
- [ ] Public MCP endpoint (3 tools) on `framework/mcp` / 09 runtime, stateless, Valkey rate limit, disable toggle; `dimmi_search_docs` `kind`, `dimmi_get_doc` resolves changelog URLs via provider.
  - Tests: tools/list (exactly 3 tools), tools/call each, disabled 404, agent_only never returned, changelog URL resolve (E197, E198).

## M9. Hardening and budgets

- [ ] Purge job (trash, orphans, embeddings GC, old model vectors, runs, search log retention) with `RetentionGuard` per workspace batch + per contact for search rows (README Jobs, retention rule).
  - Tests: guard driver configured to refuse per destroy class (real `RetentionGuard` with test policy) (trash purge, search prune) keeps rows + metric; unguarded classes (embeddings GC, runs beyond 50) still run.
- [ ] Metrics, logs, traces per README; audit log entries.
- [ ] Benchmarks in CI: TTFB cached/cold, search keyword/hybrid (fake embedder), sitemap 10k, suggestion 50 hunks; fail on 10% regression.
- [ ] Leak test suite: crawl all public routes (HTML, md, llms, sitemap, search, widget, MCP, Ask AI with fake LLM echoing sources) with agent_only, draft, knowledge-source, and second-help-center fixtures containing marker strings; assert absent.
- [ ] Cross-tenant test sweep: every endpoint with other-workspace ids returns 404.
- [ ] Docs: help center + knowledge pages in Dimmi's own help center (dogfood), OpenAPI examples.

## M10. Section providers and cross-spec seams (13, 14, 15-announcements-surveys)

- [ ] `helpcenter.PublicSectionProvider` registry (NavLinks, HomeSections, Routes, ReservedPaths, SitemapURLs, LlmsTxtSection, RobotsRules, SearchResultKind, CSP, HcJsModule) iterated by layout, home, router, sitemap (child `/sitemaps/{section}-{n}.xml`), llms.txt, robots, search, CSP middleware, hc.js bundle (5 KB gz budget).
  - Tests: fake provider registered in test App: nav/home/sitemap/llms/robots/CSP output, provider route before redirect fallback, provider error isolated (E191, E192, E193).
- [ ] `AssetReferencer` and `LifecycleHook` registries; `DeleteHelpCenter`/restore call hooks in same tx.
  - Tests: real 13 registration once 13 lands (E194, E196).
- [ ] Knowledge exports: `IndexChangelogEntry`, `RemoveChangelogEntry` (same tx), `ExcludeChangelog`, `SourceType changelog`, `KnowledgeDocument.ChangelogEntryID`; `EmbedTexts` (+ `ErrNoEmbeddings`); GitHub layer exports for 13: `github.ClientFor`, `github.RegisterWebhookHandler` (`ResolveGitCredentials` removed). 13 migration 001314 adds `knowledge_documents.changelog_entry_id` + three-way CHECK.
  - Tests: index/remove in tx, rollback leaves nothing (E195); `EmbedTexts` no key and spend cap (E204); `ClientFor` other workspace `ErrNotFound` (E203); fake `release` handler registered in test App receives dispatched delivery.
- [ ] `help_center.updated` payload `changes` with `status` / `deleted`.
- [ ] Valkey resilience: L1 + pubsub bust, limiter fail-open policy, Ask AI day cap Postgres fallback, query embedding L2.
  - Tests: test's keyspace flushed mid-suite (E205); Valkey unreachable via in-test TCP proxy cut (E206).

## M11. Docs sync and freshness (launch item 16, docs-sync.md)

- [ ] GitHub connections: migration 0821, prefix `ghc_`, `kind` registry enum (`app/integrations/github/connection_kinds.go`), create/rotate/delete/test actions + REST, token `encryption.Sealed`, expiry header, `CheckGithubConnections` daily, event `github_connection.errored`; permissions `github:read`, `github:manage` registered (00-accounts catalog).
  - Tests: E209, E210, E257; token never in any response, log line or job arg (grep captured logs); expiring in 6 days emits event once per day.
- [ ] GitHub repositories + shared webhook: migration 0822, prefix `ghr_`, `AddRepository` (resolve, permission probe incl. `X-Accepted-GitHub-Permissions`, auto hook create / manual secret once / none), `SyncRepositoryWebhook` (events union PATCH), `CheckGithubRepositories`, route `/webhooks/github/repositories/{ghr}` (HMAC, repo id match, delivery dedupe Valkey + River unique, compact envelope, 202), `HandleGithubDelivery` + `WebhookHandler` registry; `accessible-repositories`; `checks`; `BackfillArticleSourceRefs`.
  - Tests: E211, E212, E213, E215, E216, E258, E259, E261; webhook ack bench under 1 s; envelope has no raw payload (assert River args).
- [ ] Docs sync sources: migration 0824, prefix `dss_`, CRUD + REST, defaults, branch check, import-path exclusion, entitlement `docs_sync.max_sources` (catalog + `used`), permission `docs_sync:manage`, pause/resume cursor, cost estimate in `meta`.
  - Tests: E223; over cap 422; member 403; import-owned paths listed in `meta.excluded_by_import`.
- [ ] Runs + triggers: migration 0825, prefix `dsr_`, change keys + partial unique index, PR handler, push handler with debounce window + PR coverage check (`ProcessDocsSyncPush`), `PollDocsSyncSources` (poll, reconcile, watchdog), manual/redraft/freshness `POST /docs-sync-runs`, cancel PATCH, entitlement `docs_sync.runs_daily` (async skip + REST 429).
  - Tests: E215, E218, E219, E220, E221, E222, E243, E262; webhook and poll for same PR create one run; PR merged before source created never processed (D11).
- [ ] Gate + fetch + digest (pure parts table-tested): author/label/opt-out/revert rules, path globs, file scorer, budgeter, `app/support/secretscan` redaction, PR body template stripping, rate-limit snooze.
  - Tests: E217, E224, E225, E242, E260; digest golden files from fixtures; marker leak test (code marker absent from Postgres dump incl. `river_job` + `ai_run_steps`, Valkey, logs, traces).
- [ ] Screen: prompt `resources/prompts/docs_sync/v1/screen.md` + JSON schema, `ai.RunTask` kind `docs_sync` (`PersistInput: ptr(false)`; 07 `RunTaskInput.PersistInput *bool`, done in 07), sensitivity rule (pure), skipped log.
  - Tests: `llm.Fake()` scripted screen JSON per fixture; strict/balanced/eager table; force label bypass; invalid JSON twice -> `failed invalid_output`; E238 (no key), E239 (cap at gate).
- [ ] Map: explicit `docs:` refs (ids, slugs, URLs on every host incl. redirects), source refs prefix match, hybrid search via `contracts.Knowledge.Search` (`Surface docs_sync`), RRF across queries, import-managed exclusion, caps; embedding spend recorded.
  - Tests: E214, E226, E227, E228, E235; FTS-only workspace still maps by refs and keywords.
- [ ] Draft + assemble: prompts `draft_edit.md`, `draft_article.md`; anchored edit applier (pure, exact-once, overlap drop); replace validation (render, `linkguard`, secretscan warning); suggestion batch per run, `source_kind commit`, `source_ref`, `anchored_edits` (0807 column), dedupe key, redraft supersede; hunk `notes` at read; `docs_sync_run.completed/failed` events + webhook catalog entries (10).
  - Tests: E229, E230, E231, E232, E233, E234, E239, E240, E241; 12 PR fixtures end to end (docs-sync.md 15); arch test: `app/actions/docssync` never imports accept/publish actions.
- [ ] Review additions: `PATCH /article-suggestions/{sug} {base: "current"}` reapply, `reapply_not_supported`, `edits_unmatched`; accept hook writes `article_source_refs` + freshness baseline; rejection notes fed to draft prompt (last 10).
  - Tests: E233, E236, E237; reapply on non-docs-sync suggestion 422.
- [ ] Source refs + freshness: migration 0826, prefix `asr_`, refs REST, `article_freshness`, help center `settings.freshness`, `ScheduleFreshnessScans`, `ScanArticleFreshness` (compare grouped by sha, truncated fallback, link check, feedback ratio, age), `VerifyArticleClaims` (prompt `verify_article.md`, weekly cap), `RecordArticleVerification`, `PATCH /articles/{art} {verified: true}` (`RequireHumanReview`), `GET /articles?filter[freshness]`, `POST /help-centers/{hc}/freshness-checks`, event `article.freshness_changed`.
  - Tests: E244 to E255; scan bench 1,000 articles under 30 s with fake GitHub.
- [ ] MCP toolset `docs_sync` (3 tools) + prompt `dimmi_update_docs_for_change`; toolset budget under 2,000 tokens; CLI contract rows for 09.
  - Tests: tool schemas from request structs; `dimmi_request_docs_update` creates run, cannot accept; `verified` via `/mcp` surface 403 (E252).
- [ ] RLS migration 0827, ownership/prefix rows requested in 00-foundation, metrics/logs/traces (docs-sync.md 14), `dimmi doctor` checks (poll-mode repos, failing hooks, expiring tokens, `app_url` public), `dimmi docs-sync:replay`, `dimmi github:check`, audit entries.
- [ ] Prompt eval set `tests/evals/docs_sync/` (30 labeled PR x article cases, rubric); runner prints cost estimate and needs explicit approval per run; never in CI.

## M12. Import helpers for 16-importers (launch: importers)

- [ ] `app/actions/helpcenters/importing/`: `ImportCollection` (two levels, flatten `A / B`), `ImportArticle` (draft default; `Publish` with preserved timestamps via shared publish internals, events off, knowledge index in tx), `ImportAsset` (`app/support/images`, sha256 dedupe), `ImportRedirect` (`source import`, `ErrRedirectPathReserved`, same-target no-op); crawl reuse surface for 16 `public_site` (exported `crawl` + `dom` helpers); arch test: package imports no `app/events`.
  - Tests: E263 to E270 (integration, real Postgres); MCP preserve path only via 09 confirm gate, other MCP publish 403.

## Done when

- `go test ./...` green against real Postgres + Valkey only; benchmarks within budgets; Lighthouse 100s.
- Demo script (fresh `docker compose up`, self-host):
  1. Sign up; default help center exists at `http://localhost:8080/hc/acme`.
  2. Connect Dimmi MCP in Claude Code (09); prompt "draft our help center from this repo" with `dimmi_draft_help_center`; 25 suggestions appear in review queue within 2 minutes; nothing public yet (curl `/hc/acme/llms.txt` lists nothing).
  3. Open one suggestion: +/- counts, hunks; accept batch with publish.
  4. Edit one article in dashboard (API), publish; page updates under 2 s; old slug renamed -> old URL 301.
  5. Propose edit via MCP on that article with stale base -> suggestion `stale: true`, accept 409 `suggestion_stale`; refresh, accept selected hunks.
  6. Version history shows 3 versions; restore v1; diff visible.
  7. Add website source pointing to local test docs site; crawl completes with stats; robots-disallowed pages skipped.
  8. Connect GitHub repo (fixture repo) in import mode; push change -> webhook -> edit suggestion with commit provenance link.
  9. With no embeddings key: `/api/v1/knowledge/search` mode `keyword`/`fts_fallback`. Add OpenAI key (fake driver in demo env, or real key only with user approval): re-embed progress to 100%, mode `hybrid`.
  10. Mark article agent_only: public 404, absent from sitemap, llms.txt, public MCP; 07 playground still answers from it without link.
  11. `curl -H 'Accept: text/markdown' /hc/acme/articles/x` returns Markdown; `/llms-full.txt` contains all public articles.
  12. Add custom domain `help.example.test` with fake resolver in demo mode: TXT + CNAME checks pass, TLS via caddyask; page served on custom host, path mode 301s to it.
  13. Ask AI on help center with fake 07 agent: streamed answer with public citations.
  14. Widget Help tab endpoints return collections and article HTML.
  15. Public MCP `tools/call dimmi_search_docs` returns results.
  16. Enable changelog (13) and feedback portal (14): nav links and sitemap child sitemaps appear, llms.txt `## Changelog` section appears; `/changelog` on help center host; manual redirect to `/feedback/x` refused.
  17. Docs sync (fake GitHub via `httpclienttest.New(tc.App.HTTP)` fixtures in demo mode, scripted `llm.Fake()`; real GitHub + real model only with user approval): add GitHub connection (token), add repo `acme/app` -> auto hook installed, `webhook_status active` after ping.
  18. Create docs sync source for help center (balanced). Deliver fixture `pull_request` closed+merged "Raise team seat limit": run row within 5 s; under 3 min batch "PR #128" in review queue with 1 edit suggestion (+1 / -1), hunk note cites `src/billing/plans.ts`, source link to PR; nothing public changed.
  19. Deliver dependency-bump PR and PR labeled `no-docs`: both runs `skipped` (`no_user_facing_change`, `label_ignored`) visible in skipped log with reasons; cost shown per run.
  20. Accept suggestion with publish: article page updates; `article_source_refs` holds `src/billing/plans.ts`; freshness `fresh`.
  21. Second fixture PR edits same article; first accepted meanwhile -> suggestion `stale`; `PATCH {base: "current"}` reapplies, accept.
  22. Fixture compare shows `src/billing/plans.ts` changed since baseline; `POST /help-centers/{hc}/freshness-checks` -> article `needs_review` (`referenced_code_changed`, then `claims_outdated` with quote after fake AI check); "Draft fix" creates run + suggestion; `PATCH {verified: true}` via MCP surface 403, via dashboard clears.
  23. Disable webhook (mode none): merged PR fixture picked up by poll within 15 min (clock advanced).
  24. Leak test passes: code marker from fixture patch absent from DB dump, Valkey, logs.
