# 08 API examples

Style: caveman. Prefixes `aik_` (AI provider key) and `air_` (AI run) are 07-owned prefixes. Shapes follow `specs/00-foundation/api.md` (`data`, `meta`, `object`, prefixed IDs, ISO UTC, null keys present). Route table in README.

## Help center

`POST /api/v1/help-centers`
```json
{ "name": "Acme Help", "subdomain": "acme", "locale": "en" }
```
201:
```json
{
  "data": {
    "id": "hc_01jcf2k8x0...",
    "object": "help_center",
    "name": "Acme Help",
    "subdomain": "acme",
    "locale": "en",
    "is_default": false,
    "status": "active",
    "url": "https://acme.dimmi.help",
    "urls": {
      "subdomain": "https://acme.dimmi.help",
      "path": "https://app.trydimmi.com/hc/acme",
      "custom_domain": null,
      "llms_txt": "https://acme.dimmi.help/llms.txt",
      "mcp": "https://acme.dimmi.help/mcp"
    },
    "settings": {
      "theme": { "primary_color": "#4f46e5", "appearance": "auto" },
      "header_links": [], "footer_text": null, "social": {}, "home_title": "How can we help?",
      "home_subtitle": null, "show_widget": false, "contact_url": null
    },
    "logo": null, "favicon": null, "og_image": null,
    "indexing_enabled": true,
    "ask_ai_enabled": true,
    "ask_ai_available": false,
    "public_mcp_enabled": true,
    "counts": { "published_articles": 0, "draft_articles": 0, "agent_only_articles": 0, "pending_suggestions": 0 },
    "created_at": "2026-11-01T10:00:00Z",
    "updated_at": "2026-11-01T10:00:00Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/help-centers/hc_01jcf2k8x0..." }
  }
}
```
`ask_ai_available` = enabled AND 07 agent ready. Counts cached (estimate), refreshed on change.

## Articles

`POST /api/v1/articles`
```json
{
  "help_center_id": "hc_01jcf2...",
  "collection_id": "col_01jcf3...",
  "title": "Request a refund",
  "slug": null,
  "description": "How refunds work and how long they take.",
  "body_markdown": "Refunds go back to the original card.\n\n## How long\n\n3 to 5 business days.\n\n> [!NOTE]\n> Annual plans refund pro rata.",
  "visibility": "public",
  "publish": false
}
```
201:
```json
{
  "data": {
    "id": "art_01jcf4...",
    "object": "article",
    "help_center_id": "hc_01jcf2...",
    "collection": { "id": "col_01jcf3...", "object": "collection", "name": "Billing", "slug": "billing" },
    "title": "Request a refund",
    "slug": "request-a-refund",
    "description": "How refunds work and how long they take.",
    "body_markdown": "Refunds go back ...",
    "status": "draft",
    "visibility": "public",
    "has_unpublished_changes": true,
    "published_version": null,
    "version_count": 0,
    "lock_version": 1,
    "word_count": 21,
    "reading_minutes": 1,
    "position": 1024,
    "seo": { "title": null, "noindex": false, "og_image_asset_id": null },
    "url": null,
    "markdown_url": null,
    "author": { "id": "mem_...", "object": "member", "name": "Ada" },
    "last_edited_by": { "type": "member", "id": "mem_...", "name": "Ada" },
    "pending_suggestions": 0,
    "warnings": [],
    "published_at": null, "first_published_at": null, "archived_at": null, "deleted_at": null,
    "created_at": "2026-11-01T10:05:00Z", "updated_at": "2026-11-01T10:05:00Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/articles/art_01jcf4..." }
  }
}
```
`url` set only when published and public. `slug: null` -> generated from title (ASCII fold; non-Latin titles transliterated; fallback `article-<6 chars>`).

`PATCH /api/v1/articles/{art}` header `If-Match: "3"` (lock_version). Mismatch 409:
```json
{ "message": "Article changed since you loaded it.", "code": "stale_resource", "current_lock_version": 4, "updated_by": { "type": "member", "name": "Grace" } }
```

`POST /api/v1/articles/{art}/publications` `{ "note": "Fix refund window" }` 201 (created resource = published version):
```json
{
  "data": { "id": "artv_...", "object": "article_version", "article_id": "art_...", "number": 3, "kind": "published", "created_at": "2026-11-01T10:20:00Z", "...": "..." },
  "meta": {
    "article": { "id": "art_...", "object": "article", "status": "published", "has_unpublished_changes": false, "url": "https://help.acme.com/articles/request-a-refund" },
    "warnings": [ { "code": "missing_alt", "message": "Image on line 12 has no alt text.", "line": 12 } ]
  }
}
```
409 `nothing_to_publish` when working copy equals published version. 403 `human_review_required` from MCP surface or third-party OAuth client (review-queue.md rule 2).

State changes on `PATCH /api/v1/articles/{art}` (table: README "Article state"):
```json
{ "status": "draft" }
{ "status": "archived", "redirect_to": "art_01jcf9..." }
{ "trashed": false }
{ "collection_id": "col_01jcf3...", "position": 2048 }
{ "help_center_id": "hc_01jcg0...", "collection_id": null }
{ "restore_version_id": "artv_01jcf5...", "publish": false, "restore_slug": false }
```
Each returns 200 `article`. `{ "status": "published" }`: 422 `{ "message": "Publish via POST /articles/{art}/publications.", "code": "invalid_transition", "from": "draft", "to": "published" }`.

`GET /api/v1/articles/{art}/versions/{artv}/diff?against=working`:
```json
{
  "data": {
    "object": "article_diff",
    "from": { "id": "artv_...", "number": 3 },
    "to": { "id": null, "label": "working" },
    "additions": 4, "deletions": 1,
    "meta_changes": [ { "field": "title", "old": "Refunds", "new": "Request a refund" } ],
    "hunks": [ { "id": "h0", "old_start": 3, "old_lines": 2, "new_start": 3, "new_lines": 5, "lines": [ "...see review-queue.md hunk model..." ] } ]
  }
}
```

`PATCH /api/v1/articles/{art}` `{ "restore_version_id": "artv_...", "publish": false }` -> working copy replaced by version content, new version kind `restored` (when publish true also `published`). Returns article. Content fields in same body: 422.

Ordering: `PATCH /api/v1/collections/{col}/articles` `{ "order": ["art_a", "art_b", "art_c"] }` and `PATCH /api/v1/help-centers/{hc}/collections` `{ "parent_id": null, "order": ["col_a", "col_b"] }` -> 200 list in new order. List must equal current siblings exactly, else 422 `order_mismatch` with `current` ids.

## Suggestions

`POST /api/v1/article-suggestions` (edit):
```json
{
  "kind": "edit",
  "article_id": "art_01jcf4...",
  "base_version_id": "artv_01jcf5...",
  "proposed_markdown": "Refunds go back to the original card.\n\n## How long\n\n5 to 10 business days.\n",
  "summary": "Stripe payout window changed in billing/refund.ts",
  "source_kind": "agent",
  "source_ref": { "agent": "claude-code", "repo": "acme/app", "commit_sha": "9f2c1e7a", "paths": ["src/billing/refund.ts"] },
  "dedupe_key": "refund-window-9f2c1e7a"
}
```
201 `article_suggestion`:
```json
{
  "data": {
    "id": "sug_01jcf6...",
    "object": "article_suggestion",
    "kind": "edit",
    "status": "pending",
    "help_center_id": "hc_...",
    "article": { "id": "art_01jcf4...", "object": "article", "title": "Request a refund", "slug": "request-a-refund", "status": "published" },
    "batch_id": null,
    "title": "Request a refund",
    "summary": "Stripe payout window changed in billing/refund.ts",
    "additions": 1, "deletions": 1, "hunk_count": 1,
    "base": { "version_id": "artv_01jcf5...", "assumed": false }, "stale": false,
    "current_markdown": "Refunds go back to the original card.\n\n## How long\n\n3 to 5 business days.\n...",
    "proposed_markdown": "...",
    "meta_hunks": [],
    "hunks": [
      {
        "id": "h0", "section": "How long",
        "old_start": 3, "old_lines": 3, "new_start": 3, "new_lines": 3,
        "additions": 1, "deletions": 1, "decision": null,
        "lines": [
          { "op": "context", "text": "## How long" },
          { "op": "context", "text": "" },
          { "op": "delete", "text": "3 to 5 business days.", "segments": [ { "op": "delete", "text": "3 to 5" }, { "op": "equal", "text": " business days." } ] },
          { "op": "insert", "text": "5 to 10 business days.", "segments": [ { "op": "insert", "text": "5 to 10" }, { "op": "equal", "text": " business days." } ] }
        ]
      }
    ],
    "source": {
      "kind": "agent", "label": "claude-code @ 9f2c1e7",
      "url": "https://github.com/acme/app/commit/9f2c1e7a",
      "ref": { "agent": "claude-code", "repo": "acme/app", "commit_sha": "9f2c1e7a", "paths": ["src/billing/refund.ts"] }
    },
    "proposed_by": { "type": "oauth_client", "id": "ocl_...", "name": "Claude Code" },
    "reviewed_by": null, "reviewed_at": null, "review_note": null, "result_version_id": null,
    "deduplicated": false,
    "created_at": "2026-11-01T11:00:00Z", "updated_at": "2026-11-01T11:00:00Z"
  }
}
```

`PATCH /api/v1/article-suggestions/{sug}` (accept)
```json
{ "status": "accepted", "hunks": ["h0"], "publish": true, "note": "Confirmed with finance" }
```
Reject: `{ "status": "rejected", "note": "Window is still 3 to 5 days" }` -> 200, status `rejected`. Proposal fields (`proposed_markdown` ...) with `status` in same body: 422.
200:
```json
{
  "data": { "id": "sug_...", "object": "article_suggestion", "status": "accepted", "result_version_id": "artv_...", "reviewed_by": { "id": "mem_...", "name": "Ada" }, "...": "..." },
  "meta": { "article": { "id": "art_...", "status": "published", "published_version_number": 4 }, "warnings": [] }
}
```
Errors: 409 `suggestion_not_pending`, 422 `hunk_not_found` (`errors.hunks`), 409 `suggestion_stale` (working copy changed since proposal: refresh via `PATCH {proposed_markdown}` or reject), 422 `no_hunks_selected`, 409 `article_archived`, 403 `human_review_required` (MCP surface or third-party OAuth client, review-queue.md rule 2).

`POST /api/v1/article-suggestion-batches` (schema in review-queue.md) 201:
```json
{
  "data": {
    "id": "sugb_01jcf7...", "object": "article_suggestion_batch", "status": "pending",
    "title": "Initial help center from acme/app@9f2c1e",
    "total": 28, "pending": 28, "accepted": 0, "rejected": 0,
    "additions": 3120, "deletions": 0,
    "review_url": "https://app.trydimmi.com/w/acme/help/suggestions?batch=sugb_01jcf7...",
    "source": { "kind": "agent", "label": "claude-code @ 9f2c1e7" }
  },
  "meta": {
    "created": 28,
    "deduplicated": 0,
    "errors": [ { "index": 29, "field": "items.29.markdown", "code": "markdown_too_large", "message": "Markdown over 256 KB." } ]
  }
}
```

`PATCH /api/v1/article-suggestion-batches/{sugb}` `{ "status": "accepted", "publish": true }` 200 (batch `status` derived, README "Review state"; one stale suggestion left pending so stays `pending`):
```json
{
  "data": { "id": "sugb_...", "object": "article_suggestion_batch", "status": "pending", "pending": 1, "accepted": 27 },
  "meta": { "results": [
    { "suggestion_id": "sug_a", "status": "accepted", "article_id": "art_x" },
    { "suggestion_id": "sug_b", "status": "skipped_stale", "code": "suggestion_stale" }
  ] }
}
```

## Knowledge sources

`POST /api/v1/knowledge/sources` (website):
```json
{
  "type": "website",
  "mode": "knowledge",
  "name": "Docs site",
  "config": { "start_url": "https://docs.acme.com/", "include": ["/docs/**"], "exclude": ["/blog/**"], "max_pages": 500 },
  "schedule": "weekly"
}
```
201:
```json
{
  "data": {
    "id": "ks_01jcf8...", "object": "knowledge_source", "type": "website", "mode": "knowledge",
    "name": "Docs site", "status": "queued", "schedule": "weekly", "citable": true,
    "config": { "start_url": "https://docs.acme.com/", "sitemap_url": null, "include": ["/docs/**"], "exclude": ["/blog/**"], "max_pages": 500, "follow_links": true, "allowed_hosts": ["docs.acme.com"] },
    "has_credentials": false,
    "document_count": 0, "chunk_count": 0,
    "estimate": { "pages": 214, "tokens": 321000, "embedding_model": "openai:text-embedding-3-small@1024" },
    "last_run": { "id": "ksr_...", "status": "queued", "trigger": "create" },
    "last_error_code": null, "last_error": null,
    "next_sync_at": "2026-11-08T10:00:00Z", "last_synced_at": null,
    "created_at": "2026-11-01T12:00:00Z", "updated_at": "2026-11-01T12:00:00Z"
  }
}
```
Errors: 422 `url_not_allowed` (`framework/httpclient` SSRF guard: private IP, non-http), 422 `entitlement_limit_reached` (`key: knowledge.max_sources`, `limit`). `last_run` = latest `knowledge_sync_run` (`GET .../syncs/{ksr}`).

Git source:
```json
{
  "type": "git", "mode": "import", "name": "acme/app docs",
  "help_center_id": "hc_...", "target_collection_id": null,
  "config": { "github_repository_id": "ghr_01jc...", "ref": "main", "paths": ["docs/**/*.md"], "sync_on_push": true },
  "schedule": "daily"
}
```
Repo and token come from shared GitHub layer (`POST /github-connections`, `POST /github-repositories`; [docs-sync.md](docs-sync.md) section 10). Shortcut: `config.full_name` + `config.connection_id` instead of `github_repository_id` adds repository first. Webhook is per repository (`/webhooks/github/repositories/{ghr}`), not per source: auto-installed or shown once on repository add; adding this source adds `push` to repository `webhook_events`. Errors: 422 `github_repo_not_found` (hint: connect token for private repo), 422 `git_credentials_invalid` (connection token rejected), 422 `github_connection_required` (hourly schedule without connection).

`POST /api/v1/knowledge/uploads` multipart: `file` (zip or .md), `source_id` optional (replace contents), `mode`, `help_center_id` (import), `name`. 202 with source + run.

`POST /api/v1/knowledge/sources/{ks}/syncs` (no body) -> 202 `knowledge_sync_run` (`trigger: manual`, `status: queued`); 409 `sync_in_progress`. `DELETE /api/v1/knowledge/sources/{ks}/syncs/{ksr}` cancels: 200 run with `status: cancelled` (row kept); finished run 409 `sync_not_running`.

`GET /api/v1/knowledge/sources/{ks}/syncs` item:
```json
{
  "id": "ksr_...", "object": "knowledge_sync_run", "trigger": "schedule", "status": "partial",
  "commit_sha": null,
  "stats": { "discovered": 220, "fetched": 214, "unchanged": 190, "created": 4, "updated": 20, "removed": 2, "failed": 3, "skipped_robots": 3, "skipped_js": 0, "bytes": 4100223, "tokens_estimated": 36000 },
  "error_code": null, "error": null,
  "started_at": "2026-11-08T10:00:02Z", "finished_at": "2026-11-08T10:04:51Z"
}
```

## Knowledge search (debugger and API)

`GET /api/v1/knowledge/search?q=how+long+do+refunds+take&limit=5&visibility=public,agent_only&mode=auto`
```json
{
  "data": [
    {
      "id": "kc_...", "object": "knowledge_chunk", "rank": 1, "score": 0.03252,
      "matched_by": ["fts", "vector"],
      "title": "Request a refund", "heading_path": ["How long"],
      "text": "## How long\n\n3 to 5 business days.",
      "visibility": "public",
      "source_type": "article", "article_id": "art_...", "source_id": null,
      "url": "https://help.acme.com/articles/request-a-refund#how-long",
      "document_id": "kd_...", "updated_at": "2026-11-01T10:30:00Z"
    }
  ],
  "meta": {
    "mode": "hybrid", "degraded": null, "embedding_model": "openai:text-embedding-3-small@1024",
    "timings_ms": { "embed_query": 182, "fts": 3, "trigram": 0, "vector": 6, "fuse": 0, "total": 192 }
  }
}
```

## Knowledge settings

`GET /api/v1/knowledge/settings`:
```json
{
  "data": {
    "object": "knowledge_settings",
    "embeddings_status": "building",
    "active_model_key": "openai:text-embedding-3-small@1024",
    "pending_model_key": "gemini:gemini-embedding-001@768",
    "reembed": { "total": 8200, "done": 5100, "failed": 0, "started_at": "2026-11-01T13:00:00Z" },
    "embedding_key": { "id": "aik_...", "provider": "gemini", "name": "Gemini prod" },
    "available_models": [
      { "key_id": "aik_...", "provider": "openai", "model": "text-embedding-3-small", "dims": [512, 768, 1024, 1536], "default_dims": 1024 },
      { "key_id": "aik_...", "provider": "gemini", "model": "gemini-embedding-001", "dims": [768, 1536], "default_dims": 768 }
    ],
    "text_search_config": "english",
    "chunk_count": 8200,
    "coverage": 1.0,
    "embeddings_error": null
  }
}
```
`PATCH` `{ "embedding_key_id": "aik_...", "model": "gemini-embedding-001", "dims": 768 }` -> 202 with settings. `{ "embedding_key_id": null }` -> embeddings off, FTS only, vectors kept 7 days. 422 `embedding_key_unsupported` (Anthropic key). 409 `reembed_in_progress` only when same target already building (idempotent no-op returns 202).

## Domains

`POST /api/v1/help-centers/{hc}/domains` `{ "hostname": "help.acme.com" }` 201:
```json
{
  "data": {
    "id": "hcd_...", "object": "help_center_domain", "hostname": "help.acme.com",
    "status": "pending", "tls_status": "none", "is_primary": false,
    "records": [
      { "type": "TXT", "name": "_dimmi-verify.help.acme.com", "value": "dimmi-verify=k3j9q...", "purpose": "ownership", "ok": false },
      { "type": "CNAME", "name": "help.acme.com", "value": "domains.dimmi.help", "purpose": "routing", "ok": false }
    ],
    "checks": { "ownership": false, "routing": false, "caa": true, "tls": false },
    "last_error_code": "txt_missing",
    "last_error": "TXT record _dimmi-verify.help.acme.com not found yet. DNS can take up to an hour.",
    "last_checked_at": "2026-11-01T14:00:05Z", "next_check_at": "2026-11-01T14:01:05Z"
  }
}
```

`POST /api/v1/help-centers/{hc}/domains/{hcd}/checks` (no body) -> 202 same `help_center_domain` resource (`next_check_at` = now); result via `help_center.domain_status_changed` event or `GET`. Within 10 s of previous check: 429 `rate_limited` with `Retry-After`.

## Public help center JSON

`GET https://help.acme.com/api/search?q=refu&instant=1`:
```json
{
  "query": "refu",
  "mode": "keyword",
  "ask_ai": true,
  "results": [
    { "title": "Request a refund", "url": "/articles/request-a-refund", "section": "How long", "snippet_html": "<mark>Refunds</mark> go back to the original card." }
  ]
}
```
No `data` envelope (public page API, not `/api/v1`). Never includes agent_only.

`POST https://help.acme.com/api/answers` `{"question":"How long do refunds take?"}` -> `text/event-stream` (computed answer, nothing stored as resource; 07 records `ai_runs`):
```
event: delta
data: {"text":"Refunds take 3 to 5 business days"}

event: citations
data: {"citations":[{"title":"Request a refund","url":"https://help.acme.com/articles/request-a-refund#how-long"}]}

event: done
data: {"answered":true,"html":"<p>Refunds take 3 to 5 business days ...</p>","run_id":"air_..."}
```
Errors before stream: JSON 409 `ai_unavailable`, 429 `ask_rate_limited` (`Retry-After`), 422 validation. Mid-stream error: `event: error` `{"code":"ai_unavailable"}` then close.

## Widget Help

Auth: `app_id` (`wgt_...`) query or header, visitor session optional (public help). Scope: widget's `widget_settings.help_center_id` (01); null or `help_tab_enabled = false`: 404 `help_center_not_linked`. Ids from other help centers 404.

`GET /widget/v1/help/articles/{art}`:
```json
{
  "data": {
    "id": "art_...", "object": "help_article",
    "title": "Request a refund", "description": "How refunds work...",
    "html": "<p>Refunds go back...</p><h2 id=\"how-long\">How long</h2>...",
    "toc": [ { "level": 2, "text": "How long", "anchor": "how-long" } ],
    "url": "https://help.acme.com/articles/request-a-refund",
    "collection": { "id": "col_...", "name": "Billing" },
    "updated_at": "2026-11-01T10:30:00Z"
  }
}
```
agent_only, draft, archived -> 404.

## Public MCP

`POST https://help.acme.com/mcp` (MCP 2026-07-28 streamable HTTP, stateless; `Mcp-Method` header per spec):
```json
{ "jsonrpc": "2.0", "id": 1, "method": "tools/call",
  "params": { "name": "dimmi_search_docs", "arguments": { "query": "refund time", "limit": 3 } } }
```
Result:
```json
{ "jsonrpc": "2.0", "id": 1, "result": {
  "content": [ { "type": "text", "text": "1. Request a refund (https://help.acme.com/articles/request-a-refund#how-long)\n3 to 5 business days..." } ],
  "structuredContent": { "results": [ { "title": "Request a refund", "url": "https://help.acme.com/articles/request-a-refund#how-long", "markdown_url": "https://help.acme.com/articles/request-a-refund.md", "section": "How long", "snippet": "3 to 5 business days." } ] }
} }
```

## Webhook payloads (10 delivers)

`article_suggestion.reviewed`:
```json
{
  "id": "evt_...", "type": "article_suggestion.reviewed", "created_at": "2026-11-01T15:00:00Z",
  "workspace_id": "ws_...",
  "data": {
    "object": "article_suggestion", "id": "sug_...", "kind": "edit", "status": "partially_accepted",
    "article_id": "art_...", "accepted_hunks": 2, "rejected_hunks": 1,
    "reviewed_by": { "type": "member", "id": "mem_..." },
    "source": { "kind": "agent", "url": "https://github.com/acme/app/commit/9f2c1e7a" }
  }
}
```
Body not included in webhook payload (size); `links.self` to fetch.

`knowledge_source.sync_completed`: `{ object: "knowledge_sync_run", id, source_id, status, stats }`.

## Error shape examples

```json
{ "message": "Subdomain acme is taken.", "code": "subdomain_taken", "errors": { "subdomain": ["Subdomain acme is taken."] } }
{ "message": "Collections can nest only one level.", "code": "collection_depth_exceeded" }
{ "message": "This action needs review by a person in the dashboard.", "code": "human_review_required" }
```
