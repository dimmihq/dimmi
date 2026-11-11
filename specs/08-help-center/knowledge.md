# 08 Knowledge: contract, ingestion, retrieval

Style: caveman. Owner 08. Consumer 07-dimmi-agent (retrieval), 08 help center search, public MCP, widget. Facts verified 2026-11-01 (sources at bottom).

## 1. Knowledge contract (for 07)

Canonical interface and types: `contracts.Knowledge` in `app/contracts/knowledge.go`, defined in `specs/00-foundation/contracts.md` (wins over this file). Types `KnowledgeSearchOptions`, `KnowledgeSearchResult`, `KnowledgeChunk`, `KnowledgeDocument`, `KnowledgeStatus`, `KnowledgeVisibility`. Methods `Search`, `Document`, `Status`, `EmbeddingKeyInUse`, `PublicHosts`. Audience filter (18 M7): `KnowledgeSearchOptions.AllowedAudienceIDs`, chunk kept when `effective_audience_ids <@ $allowed`; public `help_center` runs and anonymous MCP pass empty set. Implementation `app/actions/knowledge` (import path `dimmi/app/actions/knowledge`); `*App.Knowledge` typed `contracts.Knowledge`. 07 depends only on interface. `knowledge.Fake()` for tests (scripted chunks).

08-only implementation notes:
- `Search` never errors for missing embeddings: falls back to FTS, `KnowledgeChunk.MatchedBy` lacks `vector`, `Similarity` nil. Errors only: ctx canceled/deadline, database failure, invalid options.
- `KnowledgeSearchResult.Mode`, `Degraded`, `Timings` (embed_query, fts, trigram, vector, fuse, total) feed debugger, logs, 07 run steps. No separate detailed method.
- `KnowledgeSearchOptions.Mode`: `auto` (default; hybrid when embeddings ready and query embedding obtained within `QueryEmbedTimeout`, else FTS), `keyword` (FTS + trigram only, no provider call; public search-as-you-type), `hybrid` (like auto, same fallback). Option `ExcludeChangelog bool` (default false; public scope `ExcludeSources` keeps changelog: entries are public). Defaults: `Visibility` [public, agent_only], `MaxPerDocument` 3 (adjacent chunks of same doc merged before cap), empty `HelpCenterIDs`/`SourceIDs` = all (articles always included unless `ExcludeArticles`). `LogSearch` writes `help_center_searches` row.
- `KnowledgeVisibility`: `public` (may be shown and linked to customers), `agent_only` (agent may use; never link; never expose URL).
- `KnowledgeChunk.SourceType`: article | changelog (13) | website | upload | git | text. `KnowledgeChunk.ChangelogEntryID *ids.UUID` set for changelog chunks. `HeadingPath` []string (e.g. ["Billing", "Refunds"]). `UpdatedAt` = document indexed_at (article: version created_at).
- `KnowledgeSearchResult.EmbeddingModel` = active model key (`<provider>:<model>@<dims>`, "" when none). `EmbeddingKeyID` + `EmbeddingUsage` set when provider called (cost attribution in 07).
- `KnowledgeDocument` (08 defines fields; contracts.md names type): `ID`; `ArticleID`, `SourceID`, `ChangelogEntryID` *`ids.UUID` (`framework/ids`); `Title`, `URL`, `Markdown` string; `Visibility`; `UpdatedAt`. Used by 07 "read more" and citation expansion; same visibility filter.
- `KnowledgeStatus` (08 defines fields; cached 10 s): `HasContent` bool (at least one active chunk visible to agent), `PublicArticles`, `AgentOnlyArticles`, `SourceDocuments`, `Chunks` int, `Embeddings` string (none | ready | building | degraded), `EmbeddingCoverage` float64 (0..1 share of chunks with active-model vector), `EmbeddingModel` string.

Rules 07 can rely on (contracts.md list plus):
1. Only published article versions and `active` source documents returned. Drafts, archived, trashed, excluded, removed never.
2. `agent_only` chunks have `URL == ""`. 07 must not invent links for them.
3. Embedding call bounded by `QueryEmbedTimeout`, then FTS fallback with `Degraded = "embedding_timeout"`.
4. Empty or stopword-only query: empty slice, nil error.
5. Thread-safe; no per-call allocation of drivers (embedder cached per key version).

## 2. Embeddings contract

`app/contracts/embeddings.go`. Owner 08. Built from 07's `ai_provider_keys` rows by `managers.EmbeddingsManager.ForKey(ctx, key)`: credentials via 07 `ai.ResolveProviderKey(ctx, workspaceID, keyID)` (never persisted by 08), LRU keyed `(key_id, updated_at)`, same `framework/httpclient` transport (`a.HTTP.StdClient()` for SDK drivers), same redaction rules.

```go
type Embeddings interface {
    Provider() string
    // Models known to support embeddings for this key (static table + list endpoint where available).
    Models(ctx context.Context) ([]EmbeddingModel, error)
    // Embed returns one vector per input, same order. Inputs already within model token limit (caller chunks).
    Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
}

type EmbedRequest struct {
    Model      string
    Inputs     []string   // 1..64
    Dimensions int        // requested output dims; 0 = model native. Driver errors if model fixed-dim and value differs
    InputType  EmbedInputType // "document" | "query" (Gemini task_type, Voyage input_type, Cohere input_type; ignored by OpenAI)
}

type EmbedResponse struct {
    Vectors [][]float32  // L2-normalized by driver (Matryoshka truncation requires renormalize)
    Dims    int
    Usage   EmbedUsage   // InputTokens, Reported bool
    Model   string
}

type EmbeddingModel struct { ID string; NativeDims int; AllowedDims []int; MaxInputTokens int; MaxBatch int }
```

Errors: reuse 07 `*LLMError` kinds (`auth`, `quota`, `rate_limited`, `server`, `timeout`, `invalid_request`, `model_not_found`, `network`, `blocked_host`). Retry layer `support/llmretry` reused.

### Drivers and default model per provider

| Key provider (07) | Driver | Endpoint | Default model @ dims | Notes |
|---|---|---|---|---|
| openai | `embeddings/openai` | `POST /v1/embeddings` | `text-embedding-3-small` @ 1024 | `dimensions` param shortens (default 1536, large 3072); max input 8192 tokens |
| openrouter | `embeddings/openaicompat` | `POST https://openrouter.ai/api/v1/embeddings` | `openai/text-embedding-3-small` @ 1024 | OpenAI-shaped; model list filtered by embeddings modality |
| gemini | `embeddings/gemini` | `:batchEmbedContents` (`x-goog-api-key` header) | `gemini-embedding-001` @ 768 | `outputDimensionality`; `taskType` RETRIEVAL_DOCUMENT / RETRIEVAL_QUERY; default 3072; per-text limit 2,048 tokens [3P] so chunk max 800 tokens safe |
| bedrock | `embeddings/bedrock` | `InvokeModel` | `amazon.titan-embed-text-v2:0` @ 1024 | dims 256, 512, 1024; Cohere embed models selectable |
| openai_compatible | `embeddings/openaicompat` | `POST {base}/embeddings` | none: user picks (Ollama `nomic-embed-text` 768, `mxbai-embed-large` 1024, `bge-m3` 1024) | dims detected by probe embed of "dimmi"; `dimensions` param not sent unless key config says supported |
| voyage (requested from 07) | `embeddings/voyage` | `POST https://ai.mongodb.com/v1/embeddings` (Atlas) or legacy Voyage base via config | `voyage-4-lite` @ 1024 | `input_type` query/document; dims 256, 512, 1024, 2048 |
| anthropic | none | Anthropic offers no embedding model | n/a | FTS-only unless another embeddings-capable key exists. Dashboard nudge: "add OpenAI, Gemini, Voyage, OpenRouter or Ollama key for semantic search" |

Dims policy: always request a dims value in `knowledge.embed.supported_dims` (384, 512, 768, 1024, 1536) so HNSW index exists. Fixed-dim model outside set (example 2048 or 4096): allowed, exact scan only, warning `dims_not_indexed` shown; refused if workspace chunk_count over `exact_max_chunks` (422 `embedding_model_invalid`, message explains).

Model key string: `<provider>:<model>@<dims>` e.g. `openai:text-embedding-3-small@1024`. Key id not part of model key: rotating key or swapping to another key of same provider+model keeps vectors (same vector space).

### System default embedder (owner decision 2026-11-10)

- Every workspace gets semantic search with no setup: the built-in embedder is DeepInfra `Qwen/Qwen3-Embedding-8B` (OpenAI-compatible `POST https://api.deepinfra.com/v1/openai/embeddings`), its own key in config `ai.managed.embeddings.{api_key,base_url,model,dims}` (env `DIMMI_AI__MANAGED__EMBEDDINGS__API_KEY`), separate from the managed LLM keys. Default dims 1024 (Matryoshka; renormalized), model key `dimmi:Qwen/Qwen3-Embedding-8B@1024`. Query inputs carry Qwen's instruction prefix (`Instruct: Given a customer support question, retrieve help articles that answer it\nQuery: `); documents none.
- Without the managed key (self-host that sets none) the built-in embedder is absent and the old rules apply: first embeddings-capable workspace key, else FTS-only.
- A workspace may pick another embedder (any embeddings-capable 07 key, or back to `dimmi`). The chosen provider + model + dims is bound in `knowledge_settings` (`active_model_key`, `pending_model_key`, `embedding_key_id` null for built-in): retrieval always embeds the query with the active model, never with whatever key is newest. Switching re-embeds in the background (section 6) and search stays on the old model until coverage is complete.
- Embedding usage is recorded per workspace (tokens, model key) like LLM usage; billing stays out of scope.

### Background embedding jobs

- `EmbedKnowledgeChunks` (batches of 64 hashes, per model key): on article publish/update, source sync, and re-embed. Idempotent by `(workspace, model_key, embed_hash)`.
- `ReembedWorkspaceKnowledge`: model switch or a missing-vector sweep.
- Nightly `SweepKnowledgeEmbeddings`: retries failed hashes, embeds chunks missing a vector for the active model, purges vectors of models unused for 7 days.
- Conversations (03 "Similar conversations"): a closed conversation's subject + first customer message + resolution summary is embedded with the workspace's active model (`EmbedConversation`, queued on `conversation.closed`, re-run when reopened and closed again); similar conversations and the agent's "solved before" lookup use hybrid search over them, members only, never shown to customers.

### Choosing model (KnowledgeSettings)

- Workspace has no `knowledge_settings.embedding_key_id`: on first 07 key create (08 queued listener `SuggestEmbedderOnKeyChange` on `ai.provider_key_changed`; 07 never imports 08 package) pick first embeddings-capable key by priority openai, gemini, voyage, openrouter, bedrock, openai_compatible; set pending model; start re-embed. Anthropic-only: stays `none`.
- Member changes model via `PATCH /knowledge/settings` -> `pending_model_key` set, re-embed starts.
- Key deleted (07): next embed/query call gets `ErrKeyNotFound` -> `embeddings_status = degraded`, `embeddings_error = "key_removed"`, search FTS-only. Existing vectors kept 7 days (key re-added with same provider+model resumes instantly), then purge job removes.

## 3. Ingestion pipeline

```
source sync start (`POST /knowledge/sources/{ks}/syncs`, scheduler every 5 min, create, selection change) -> run row `knowledge_sync_runs` (queued); cancel: `DELETE /knowledge/sources/{ks}/syncs/{ksr}`
  DiscoverKnowledgeSource (one job per listing page, cursor in args) -> driver.List -> documents seen this run (pending)
  FetchKnowledgeDocument xN (one per changed ref; refs carrying their Document inline skip the fetch) -> driver.Fetch
  sync mode: store markdown -> knowledge_document_chunks rows in same tx -> EmbedKnowledgeChunks (missing embed_hash only)
  FinishKnowledgeSync -> removals (docs not seen in a successful full run -> removed) -> stats -> event + failure mail
  import mode -> ImportKnowledgeSource (batches of 25) writes help center articles + collections + redirects
```

Built shape (2026-11-11):
- Driver contract `app/integrations/knowledge/sources`: `Driver{Describe, Check, List(cursor), Fetch(ref)}`; optional `Browser{Tree}` for services with structure. Registry built in provider, no `init`. Conformance suite `tests/sourcetest` (replays recorded fixtures from `tests/fixtures/sources` through `httpclienttest`; `Record` re-records against dummy accounts with `DIMMI_RECORD_SOURCES=1`). Contributor guide `docs/knowledge-sources.md`.
- Types shipped: `website`, `notion`, `intercom_help_center`, `zendesk_help_center`, `github`, `markdown_upload`.
- Settings form = driver JSON schema. Extensions: `x-secret` (sealed in `credentials_encrypted`, never returned), `x-integration` (connected account picker), `x-account` (per-customer host asked before consent, Zendesk subdomain), `x-choice` + `x-depends` (value picked from a list the service returns, `POST /knowledge/sources/options`; driver implements `Chooser`), `x-tree` (selection, not rendered as a field).
- Rule: no pasted keys or typed ids for a connected service. Account via OAuth, everything else picked from data pulled from it.
- Document chunks live in own table `knowledge_document_chunks` (article chunks table keeps `article_id NOT NULL`; dropping it is contract-phase). Keyword + vector search union both tables.
- Run status spelling `canceled` (lint dictionary), not `cancelled`.
- Last 50 runs kept per source. Failure mail to admins only when `last_error_code` changes.
- Pacing: framework `ratelimit` local, non-blocking; over budget -> job snoozed (`RateLimitError`). Interval `DIMMI_AI__KNOWLEDGE__CRAWL_INTERVAL_MS` (default 1000).

### Usage and cost (built 2026-11-11)

- Table `knowledge_usage_periods`: one row per (workspace, month, kind `embedding|crawl`, model or service key, source or null). Columns: input_tokens, estimated_tokens, browser_ms, requests, cost_micros, unpriced. No FK to the source, so a removed source keeps its history.
- Embedding: each `EmbedKnowledgeChunks` batch adds the provider's token count, split across the sources whose chunks it embedded by input length; articles and conversations go to source null. Provider silent on tokens: estimated from text (chars/4) and counted in `estimated_tokens`. Cost from `resources/ai/prices.json` (embedding entries added: dimmi Qwen3-Embedding-8B $0.01, openai text-embedding-3-small $0.02, -large $0.13 per Mtok). Model without a price: `unpriced`, never free.
- Crawl: Cloudflare reports `browserSecondsUsed` per crawl job; recorded once per job, priced by `DIMMI_AI__KNOWLEDGE__CLOUDFLARE_BROWSER_HOUR_MICROS` (default 90,000 = $0.09/h). Single-page `/markdown` reads are not timed yet.
- Separate from the agent's `ai_spend_periods` and its cap (owner decision 2026-11-11). Knowledge spend is never capped: syncs and embeddings never stop for cost; usage is only recorded and shown.
- `GET /knowledge/usage`: this month's lines, costliest first, with total; Settings → Knowledge shows it as "This month".

### Connected services (OAuth)

- Notion, Intercom, Zendesk, GitHub read through `integration_installations` (OAuth 2, feature `knowledge`, one installation per connected account, `Separate` so several can coexist). API tokens not accepted: Zendesk retires them 2027-04-30; Intercom and Notion tokens are per-workspace secrets the customer would have to paste.
- Engine injects `access_token` + `account` from the installation at read time (`Integrations.Tokens`); source config stores only the installation id.
- Notion: public connection, `owner=user`, identify via `/users/me`. Intercom: app OAuth, identify via `/me`. GitHub: OAuth app, scopes `repo read:user` (only scope reaching private repos; read only in practice), identify via `/user`. Zendesk: framework `oauthclient` driver `hosted` (`https://{account}.zendesk.com/oauth/...`, confidential client, PKCE, scope `read`); access tokens expire, refresh token stored as `<subdomain> <token>` so a refresh reaches the right host.
- GitHub source: repository (from `/user/repos`) and branch (default preselected) are `x-choice`; files ticked in a folder/file tree (`dir:<path>` ids). No access-token field, no public-address template (files cite their GitHub address, `agent_only`).
- App credentials: `DIMMI_SERVICES__{NOTION,INTERCOM,ZENDESK,GITHUB}__CLIENT_ID/_SECRET`. Redirect `{app_url}/integrations/{provider}/callback`. Add flow returns to `?add=<type>` with new account picked.
- Notion search is eventually consistent: pages just shared may show in the tree a minute later, at the top level until their parents are indexed.

### Choosing content (tree)

- Notion pages, Intercom collections + articles, Zendesk categories + sections + articles, GitHub folders + Markdown files shown as tree (`POST /knowledge/sources/trees` before create, `GET /knowledge/sources/{ks}/tree` after). Node `{id, title, kind, children}`; Zendesk ids prefixed `category:` / `section:`.
- Ticked ids stored in setting `selected`. Doc read when it or any ancestor ticked; nothing ticked = everything. Ticking container includes items added later.
- Saving different ticks starts a sync (trigger `manual`): new items pulled, unticked ones `removed`.

### Website crawler providers

- `website` driver wraps `Crawler{List, Fetch}`. Built-in crawler (robots, sitemaps, link following) default. Cloudflare Browser Run (`/crawl` async job, `/markdown`, `/links`) used when `DIMMI_AI__KNOWLEDGE__CLOUDFLARE_API_TOKEN` set (account id optional, resolved from `/accounts`). Check always uses built-in (fast).
- Cloudflare job waits by snoozing discovery (`wait:<job>:<started>` cursor); after `MaxWait` (default 15 min) job is deleted and partial results kept. Include/exclude patterns passed to Cloudflare so page budget is not spent on excluded paths. Slower than built-in (12-page cap took ~15 min live), accepted.

Articles path: `PublishArticle` action, in same transaction: upsert knowledge_document (article_id), rechunk, write tsvector; queue `EmbedKnowledgeChunks` for new embed hashes. Unpublish/archive/delete/agent_only<->public switch: update or delete document in same tx. FTS consistent at commit.

### Source config schemas

Website (`config`):
```json
{
  "start_url": "https://docs.example.com/",
  "sitemap_url": "https://docs.example.com/sitemap.xml",
  "include": ["/docs/**", "/guides/**"],
  "exclude": ["/blog/**", "**/*.pdf", "**?*"],
  "max_pages": 1000,
  "follow_links": true,
  "allowed_hosts": ["docs.example.com"]
}
```
- One of `start_url`, `sitemap_url` required. Default `allowed_hosts` = start host only. Extra hosts allowed only if same registrable domain (eTLD+1 via public suffix list) e.g. `www.` and `docs.`; never other domains.
- Discovery order: sitemap(s) (sitemap index followed, max 50 sitemaps, 50,000 URLs each per protocol; gz supported), `robots.txt` `Sitemap:` lines, then link following from start URL (BFS, depth cap) when `follow_links`.
- URL canonicalization: lowercase scheme/host, drop fragment, drop default port, sort query params, drop tracking params (`utm_*`, `ref`, `fbclid`, `gclid`), `rel=canonical` honored when same host (document keyed by canonical; duplicates `excluded_by=duplicate`).
- Glob semantics: path-only match, `*` within segment, `**` across segments, query excluded unless pattern contains `?`.

Git:
```json
{
  "github_repository_id": "ghr_01jc...",
  "ref": "main",
  "paths": ["docs/**/*.md", "docs/**/*.mdx", "README.md"],
  "exclude": ["docs/internal/**"],
  "public_url_template": "https://docs.acme.com/{path_without_ext}",
  "sync_on_push": true
}
```
- Built 2026-11-11: shared GitHub layer not built yet; `github` driver reads through the workspace's GitHub OAuth installation (repo + branch picked, files ticked). Docs sync should reuse that installation. Push webhook deferred. Target below stays.
- Repo + credentials: shared GitHub layer ([docs-sync.md](docs-sync.md) section 4). Source references `github_repositories` row (`knowledge_sources.github_repository_id`, required for type `git`); token lives once on `github_connections` (fine-grained PAT, Contents read + Metadata read for this consumer). Public repo may use repository row without connection (60 requests/hour unauthenticated limit per IP; cloud shares IP, so decided: unauthenticated repos limited to daily schedule). Create body may send `{full_name, connection_id?}` instead of `github_repository_id`: action calls `github.AddRepository` first (same validation).
- GitHub REST through shared adapter `app/services/github` (google/go-github/v93 v93.0.0, SDK retries off; client from `github.ClientFor`, 13 and docs sync reuse). Change check: `GET /repos/{owner}/{repo}/commits/{ref}` with `Accept: application/vnd.github.sha` (returns SHA only). Same as `last_commit_sha` -> run `succeeded` with `unchanged` stat, no download.
- Download: `GET /repos/{owner}/{repo}/tarball/{ref}` returns 302 to codeload; via `app/services/github` (go-github v93 on `a.HTTP.StdClient()`; allow redirect to `codeload.github.com`; private links expire after 5 minutes so download starts immediately). Stream gzip+tar, never write full tarball to disk; skip entries not matching paths; cap total, files, per-file size.
- Paths default `["**/*.md", "**/*.mdx"]` excluding `node_modules/**`, `vendor/**`, `.github/**`, `CHANGELOG.md`, `LICENSE*`.
- Webhook: one per repo, shared (`/webhooks/github/repositories/{ghr}`, auto-installed when token has Webhooks write, else manual URL + secret shown on repository add). Knowledge registers `github.WebhookHandler` `knowledge_git` (event `push`, wants repo when a non-paused git source with `sync_on_push` references it). Push to configured ref enqueues sync (debounced 60 s via River unique job with schedule; `X-GitHub-Delivery` dedupe in shared dispatcher).
- Hourly schedule allowed only for git with connection (token).
- Import-mode git source paths on a repo + ref are excluded from docs sync digests on same repo + branch (docs-sync.md D10).

Upload:
- Zip only. Accepted files `.md`, `.mdx`, `.markdown`, `.txt`; images `.png .jpg .jpeg .gif .webp` (import mode only, become assets). Others skipped (stat `skipped_type`), `.avif` included (framework `image` refuses AVIF).
- Re-upload to same source: full replace semantics (documents not in new zip -> removed).
- Single `.md` file upload accepted too (wrapped as one-entry archive).

### Normalization

- HTML (website): main content extraction `dom.Readable` (`framework/dom` on `codeberg.org/readeck/go-readability/v2` v2.1.3, MIT) then `dom.ToMarkdown` (`github.com/JohannesKaufmann/html-to-markdown/v2` v2.5.2, MIT). Strip nav, footer, cookie banners (readability), scripts, styles, forms. Title from `og:title` > `<title>` stripped of site suffix (` | Site`, ` - Site` when suffix repeats across pages) > first H1.
- Markdown/MDX: front matter parsed via `markdown.FrontMatter` (go.abhg.dev/goldmark/frontmatter, YAML + TOML, 16 KB cap) (`title`, `description`, `slug`, `collection`, `visibility`, `order`, `draft`, `hidden`); `draft: true` or `hidden: true` -> excluded (`excluded_by=pattern`). MDX: `import`/`export` lines removed, JSX component tags removed but inner text kept, `{expressions}` removed. Title: front matter > first H1 > file name humanized.
- Text: NFC, normalize newlines, collapse 3+ blank lines, strip zero-width chars except ZWJ in emoji sequences.
- JS-rendered detection: HTML response where extracted text under 200 chars AND (body has `<noscript>` mentioning JavaScript OR only root div `#root`, `#__next`, `#app` with scripts) -> `excluded_by=js_rendered`, run stat `skipped_js`. If over 50% of fetched pages hit this: source `last_error_code=site_requires_javascript`, message: "Site renders with JavaScript. Use sitemap of server-rendered pages, upload Markdown, or connect Git repo."
- Content hash unchanged vs stored -> skip chunk/embed (stat `unchanged`). Conditional GET (`If-None-Match`, `If-Modified-Since`) first; 304 -> unchanged.

### Crawl politeness

- robots.txt per RFC 9309: fetched once per run per host (cached max 24 h); user-agent token `DimmiBot`, else `*` group; longest match wins, allow wins ties; 4xx = allow all; 5xx/unreachable = disallow all for that run (run `partial`, error `robots_unreachable`); follow up to 5 redirects; parse cap 500 KiB. `Crawl-delay` (non-standard) honored when present, capped at 10 s.
- Rate: token bucket per host (default 1 req/s, concurrency 2) via framework `ratelimit` local token-bucket mode (app zone never imports `golang.org/x/time/rate`). 429 or 503 with `Retry-After`: page job snoozed for that duration (max 1 h), host bucket paused same duration. Three consecutive 5xx for host: back off host 5 min.
- Never send cookies or auth. Accept `text/html, text/markdown, text/plain;q=0.9`. Max page 5 MB (truncate read, `too_large`). Timeouts connect 5 s, total 20 s.
- `X-Robots-Tag: noindex` or `<meta name="robots" content="noindex">`: still indexed for agent (customer's own site, owner opted in), but `nofollow` honored for link discovery. Decision: owner intent (source creation) overrides noindex; robots.txt disallow still honored (polite crawler identity).

## 4. Chunking

Pure function `knowledge.Chunk(doc Markdown, title string, opts) []ChunkDraft` in `chunk.go`, golden tests.

1. Parse Markdown AST (goldmark). Sections split at H1-H3. Heading path = chain of ancestor headings (H1 omitted when equals title).
2. Inside section, pack blocks (paragraph, list, table, code block, blockquote) greedily to `target_tokens` (400); never split inside table row, list item, or code block unless block alone exceeds `max_tokens` (800), then split code by lines, tables by rows (header row repeated), paragraphs by sentence.
3. Section smaller than 60 tokens merged with next sibling section (keeps both headings in path `"A / B"`).
4. Token estimate: `ceil(runes / 3.5)` (no tokenizer dependency; conservative for English, safe for CJK where 1 rune about 1 token since chunk max well under 2,048).
5. Embed input = `title + "\n" + heading_path + "\n\n" + text`. `embed_hash = sha256(embed input)`.
6. Anchor = goldmark auto heading id of nearest heading (same algorithm as SSR so `#anchor` links work).
7. Max chunks per document 500 (huge docs truncated, flag `truncated`).

## 5. Retrieval SQL

Parameters: `$ws`, `$cfg` (knowledge_settings.text_search_config), `$q`, visibility array, filters.

FTS candidates (keyword retriever):
```sql
-- name: SearchKnowledgeChunksFTS :many
SELECT c.id, ts_rank_cd(c.search, query, 32) AS rank
FROM knowledge_chunks c,
     websearch_to_tsquery(sqlc.arg(cfg)::regconfig, f_unaccent(sqlc.arg(q))) AS query
WHERE c.workspace_id = sqlc.arg(ws)
  AND c.search @@ query
  AND c.visibility = ANY(sqlc.arg(visibility)::text[])
  AND (cardinality(sqlc.arg(help_center_ids)::uuid[]) = 0 OR c.help_center_id = ANY(sqlc.arg(help_center_ids)::uuid[]))
  AND (cardinality(sqlc.arg(source_ids)::uuid[]) = 0 OR c.source_id = ANY(sqlc.arg(source_ids)::uuid[]) OR c.source_id IS NULL)
ORDER BY rank DESC, c.id
LIMIT sqlc.arg(candidates);
```
Prefix support for search-as-you-type: keyword mode appends `:*` to last term (built as `to_tsquery` from sanitized tokens; never raw user syntax).

Trigram retriever (only when FTS returns under 3): `knowledge_documents.title_trgm % lower(f_unaccent($q))` with `similarity > 0.3`, top 10 documents, contributes their first chunk.

Vector retriever, exact (chunk_count <= exact_max_chunks):
```sql
-- name: SearchKnowledgeChunksVectorExact :many
WITH c AS MATERIALIZED (
  SELECT id, embed_hash FROM knowledge_chunks
  WHERE workspace_id = $ws AND visibility = ANY($vis) /* + filters */
)
SELECT c.id, e.embedding <=> $qvec::halfvec AS distance
FROM c JOIN knowledge_embeddings e
  ON e.workspace_id = $ws AND e.model_key = $model AND e.embed_hash = c.embed_hash
ORDER BY distance, c.id
LIMIT $candidates;
```
Vector retriever, ANN (above threshold), dims-specific query generated per supported dims (sqlc query per dims: `SearchKnowledgeEmbeddingsANN1024` etc.):
```sql
SET LOCAL hnsw.ef_search = 100;
SET LOCAL hnsw.iterative_scan = relaxed_order;
SELECT e.embed_hash, (e.embedding::halfvec(1024)) <=> $qvec::halfvec(1024) AS distance
FROM knowledge_embeddings e
WHERE e.dims = 1024 AND e.workspace_id = $ws AND e.model_key = $model
ORDER BY (e.embedding::halfvec(1024)) <=> $qvec::halfvec(1024)
LIMIT 200;
-- then join hashes -> chunks with visibility/filters in Go-side second query, keep top 50
```
Runs in read-only transaction with `SET LOCAL statement_timeout = '250ms'`; timeout -> vector retriever dropped, `Degraded="vector_timeout"`, FTS result returned.

Fusion (Go, `rrf.go`):
```
score(chunk) = Σ over retrievers r containing chunk: 1 / (rrf_k + rank_r(chunk))     rrf_k = 60
title boost: +0.5/(rrf_k+1) when all query terms appear in document title
merge adjacent chunks of same doc (ordinal diff 1) into one result (texts joined, best score)
cap MaxPerDocument, sort score desc then chunk id, cut Limit
```
Query embedding: normalized query (NFC, lowercase, collapse spaces) -> L1 LRU then L2 Valkey `cache.Store` (TTL 24 h, rebuildable) keyed `(workspace_id, model_key, query)` (never shared across tenants: `openai_compatible` model keys from different servers may name different models; cost and timing never leak) -> provider call with `InputType=query`, timeout `QueryEmbedTimeout`. Concurrent identical queries coalesced (singleflight).

Mode selection by surface:
| Surface | Default mode | Scope (server-enforced) |
|---|---|---|
| agent (07), dimmi_search_knowledge (authenticated) | auto (hybrid) | `[public, agent_only]`, all help centers + sources |
| ask_ai (07 `AnswerQuestion` kind help_center) | auto (hybrid) | `[public]`, `HelpCenterIDs [hc]`, `ExcludeSources` |
| help center `/search` submit, widget search submit | auto | `[public]`, `HelpCenterIDs [hc]`, `ExcludeSources` |
| `/api/search` as-you-type (`?instant=1`), widget instant | keyword | same as above |
| public MCP | auto, but `QueryEmbedTimeout` 800 ms | same as above |
| docs sync mapping (`Surface docs_sync`, docs-sync.md 6.5) | auto, `LogSearch false` | `[public, agent_only]`, `HelpCenterIDs [source hc]`, `ExcludeSources`, `ExcludeChangelog` (articles only) |

## 6. Re-embedding on model change

Blue/green, never search-down.

1. `PATCH /knowledge/settings {embedding_key_id, model, dims}` -> validate with 1 test embed (real call, tiny) -> set `pending_model_key`, `reembed_*` counters, `embeddings_status = building` (if no active model) or keep `ready` (active stays live).
2. `ReembedWorkspaceKnowledge` enumerates distinct `embed_hash` in chunks lacking row for pending model, enqueues `EmbedKnowledgeChunks` batches of 64. Progress counters updated per batch.
3. Search keeps using `active_model_key` during build. New content indexed during build embeds with BOTH models (active and pending) so switch never leaves holes.
4. Coverage reaches 100% (or 99.5% with remaining hashes failed permanently, e.g. provider rejects input) -> `SwitchEmbeddingModel` in tx: `active_model_key = pending`, `pending = null`, emit `knowledge.embeddings_switched`. Failed hashes retried nightly; their chunks still reachable via FTS.
5. Old model vectors deleted 7 days after switch (rollback window: `PATCH` back to old model within 7 days switches instantly if coverage still 100%).
6. Old key missing during build (user replaced provider): search runs FTS-only until switch (`Degraded = reembed_in_progress_old_key_missing`).
7. Second model change during build: pending replaced, in-flight batches for old pending discarded (worker checks pending still equals its model before writing).
8. `dimmi knowledge:reembed` forces full re-embed of same model (e.g. provider silently updated model); writes to temp model key suffix `#r<n>` then switches same way.
9. Text search config change (workspace locale change): `knowledge:reindex` job rewrites `search` tsvector in batches of 1,000 rows; queries use new config immediately; old rows still match partially until rewritten (acceptable, minutes).

## 7. Limits and costs shown before sync

- Before first website crawl and on every source create: dry discovery returns `estimated_pages` (sitemap count) and `estimated_tokens` (pages x 1,500 average) so dashboard shows "about N tokens to embed with <model>" with dollar estimate from 07 price table.
- Cloud caps: entitlement `knowledge.max_chunks` (`Entitlements.Limit`, cloud default 200,000, override per customer) hit -> new documents `error` with `chunk_limit_reached`; existing kept; source `partial`. Raised limit: next sync indexes skipped documents (no manual action).

## 8. Exported knowledge actions for other specs

| Action | Caller | Rule |
|---|---|---|
| `knowledge.IndexChangelogEntry(ctx, tx, entry)` | 13 publish / update of published entry | same tx as publish: upsert `knowledge_documents` (`changelog_entry_id`, visibility `public`, `help_center_id` of changelog, url = canonical entry URL), chunks + tsvector in tx, `EmbedKnowledgeChunks` queued (`ingest`). Title `Changelog: {title} ({date})`; chunk text carries publish date (07 answers "available since {date}") |
| `knowledge.RemoveChangelogEntry(ctx, tx, entryID)` | 13 unpublish, trash, purge | same tx; document + chunks removed; embeddings pruned by hash later |
| `knowledge.EmbedTexts(ctx, ws, texts) ([][]float32, modelKey string, usage, error)` | 14 duplicate detection | active embedding key/model, 07 `RecordAiSpend` kind `embedding`, spend cap respected; `ErrNoEmbeddings` when none configured or `degraded` |
| `github.ClientFor(ctx, a, ws, githubRepositoryID) (*github.Repo, error)` | 13 changelog source, 08 knowledge git, 08 docs sync | repository of same workspace only; token decrypted in memory per call, never copied or logged; deleted or other workspace: `ErrNotFound`. Replaces `knowledge.ResolveGitCredentials` (removed) |
| `github.RegisterWebhookHandler(h)` | 13 (`release`), 08 knowledge (`push`), 08 docs sync (`pull_request`, `push`) | boot-time registry; shared route verifies signature, repo id, dedupe, then dispatches (docs-sync.md 4.3) |

Help center delete/disable removes changelog documents with article documents (same purge path); restore re-indexes published entries via 13 `OnHelpCenterRestored` hook.

## Sources

- llms.txt format and `.md` convention: https://llmstxt.org/
- RFC 9309 Robots Exclusion Protocol: https://www.rfc-editor.org/rfc/rfc9309.html
- pgvector HNSW dims limits (vector 2,000, halfvec 4,000), iterative scans 0.8.0: https://pgxn.org/dist/vector/0.8.0/README.html , https://supabase.com/docs/guides/ai/vector-indexes/hnsw-indexes
- OpenAI embeddings (3-small 1536, 3-large 3072, `dimensions`, 8192 max input): https://developers.openai.com/api/docs/guides/embeddings
- Anthropic: no own embedding model, recommends Voyage (voyage-4 family, 1024 default, 256/512/2048, Atlas endpoint): https://platform.claude.com/docs/en/build-with-claude/embeddings
- OpenRouter embeddings endpoint: https://openrouter.ai/docs/api-reference/embeddings
- Gemini embeddings (outputDimensionality, taskType): https://ai.google.dev/api/embeddings [3P summaries for limits]
- GitHub tarball endpoint (302, private link 5 min expiry): https://docs.github.com/en/rest/repos/contents#download-a-repository-archive-tar
