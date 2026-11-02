# 08 Review queue and cold-start generator

Style: caveman. Bar: Productlane self-updating help center review (`research/competitors/productlane.md`): +/- counts, in-place highlights, accept or reject per edit, provenance link, version history, nothing auto-publishes. Scope trim 2026-11-02: 3-way rebase (B18) out; stale rule instead (Out (later) `review-rebase`).

## Principles

1. Every AI-written or source-imported (knowledge `import` mode) change lands as `article_suggestions` row. Only human action (dashboard session, REST token, user CLI with interactive confirm) turns it into article content. Exception: 16 import runs (migration of articles already human-written at old tool) write articles directly through 08 import helpers (README "Import helpers (16)"): draft by default; published only when human explicitly set run option `articles.publish: preserve`; audit logged.
2. No MCP tool accepts, publishes, or deletes, even behind 09 confirm gate (09 resolved, Contradiction 15: this rule wins). Enforced: accept/publish actions (suggestion or batch `PATCH {status: "accepted"}`, `POST /articles/{art}/publications`) call `RequireHumanReview(principal)` (`app/actions/articles/require_human_review.go`), built only on what 09 defines (no token claim, no `client_kind`):
   - request surface `mcp` (request reached via `/mcp` mount; 09 runtime sets surface in request context, same value as `messages.metadata.sent_via.surface`): refuse. Covers MCP clients using PAT or OAuth.
   - principal `Via.Kind = oauth_client` with client id other than `dimmi-cli` (third-party OAuth app or MCP client calling REST directly with its token): refuse.
   - allowed: session, `Via.Kind = app` (first-party Dimmi app, `ogr_` grant; never reaches `/mcp`), `personal_token`, `oauth_client` `dimmi-cli` (human at keyboard; CLI confirms interactively).
   Refusal: 403 `human_review_required`.
   Only carve-out: 16 `dimmi_run_import` with `articles.publish: preserve` (migrated content, not AI-written) allowed on MCP after 09 confirm gate (prompt shows "N articles will go live"); never in `direct` bypass. Rule 2 still blocks every MCP accept/publish of suggestions and articles.
3. Diffs always computed server side against current working copy, so reviewer sees what accept will really do.
4. Provenance always present: `source_kind` + `source_ref` + `source_url`.

## Suggestion kinds

| Kind | Required input | Accept does |
|---|---|---|
| create | title, proposed_markdown; optional slug, description, collection path, visibility | creates article (collection path resolved, missing collections created at accept, max depth 2), status draft or published per `publish` flag (default true) |
| edit | article_id, proposed_markdown (full new body) and/or proposed title/description; base_version_id strongly recommended | applies accepted hunks to working copy (refused when `stale`); publishes if article published and `publish` (default true) |
| archive | article_id, summary | archives article (optional redirect target) |

Proposals send full proposed Markdown, never patch text. Reason: LLM-written patches break; full text plus server diff is robust.

## Hunk model

Line-based diff (Myers, `go-udiff`) between `base_markdown` (working copy body at proposal or refresh) and `proposed`. Context 3 lines. Each hunk:

```json
{
  "id": "h3",                       // stable within this computation: "h" + index
  "old_start": 41, "old_lines": 4,
  "new_start": 41, "new_lines": 6,
  "lines": [
    {"op": "context", "text": "## Refunds"},
    {"op": "delete", "text": "Refunds take 5 days.", "segments": [
        {"op": "equal", "text": "Refunds take "}, {"op": "delete", "text": "5"}, {"op": "equal", "text": " days."}]},
    {"op": "insert", "text": "Refunds take 3 to 5 business days.", "segments": [
        {"op": "equal", "text": "Refunds take "}, {"op": "insert", "text": "3 to 5 business"}, {"op": "equal", "text": " days."}]},
    {"op": "context", "text": ""}
  ],
  "additions": 1, "deletions": 1,
  "section": "Billing > Refunds",    // nearest heading path, for review rail card label
  "decision": null                   // null | "accepted" | "rejected"
}
```
- Inline `segments`: word-level diff (`sergi/go-diff` word mode, semantic cleanup) only when paired delete/insert lines similarity over 40%; else none (whole-line highlight).
- Metadata hunks: title, description, slug, collection, visibility changes appear as pseudo-hunks `{"id":"meta:title","field":"title","old":"...","new":"..."}` accepted/rejected same way.
- `additions`/`deletions` on suggestion = sum of lines (Productlane style "+173 / -39"). Create suggestion: additions = line count, deletions 0.
- Max hunks 500; beyond -> suggestion flagged `too_large_for_hunks`, only whole accept/reject.
- Markdown-aware cleanup: hunks never split inside fenced code block or table; diff boundaries expanded to block edges (prevents half-table accept producing broken Markdown).

## Stale rule (no rebase at MVP)

Base = `base_markdown` (working copy body when proposal created or last refreshed). Current = working copy now. Proposed = suggestion.

- base == current: hunks = diff(base, proposed). Normal; accept allowed.
- base != current (article edited, or another suggestion accepted, after proposal): suggestion `stale: true` (computed at read and inside accept tx; no job, no stored column). Accept refused 409 `suggestion_stale`. Reviewer refreshes (`PATCH /article-suggestions/{sug} {proposed_markdown}`, same or edited text: base reset to current, hunks recomputed, decisions cleared) or rejects. Proposer may also re-propose (supersedes older).
- `base_version_id` given and its body differs from working copy at proposal: base = that version body, created already `stale` (proposer sees `stale: true`; base version of other article: 422 `base_version_mismatch`). No `base_version_id`: base = working copy at proposal, warning `base_assumed`.
- Archive kind on article already archived or deleted: status `superseded`.
- Docs sync suggestions (`anchored_edits` set): reviewer may `PATCH {base: "current"}` instead: stored anchored edits re-applied exactly onto current working copy, base reset, hunks recomputed, unmatched edits dropped with warning `edit_unmatched`, zero matched 409 `edits_unmatched` (docs-sync.md 7.4). Other suggestions: 422 `reapply_not_supported`. `base` never mixed with `status` or `proposed_*` in one body (422).
- 3-way rebase (diff3 replay onto current, conflict hunks): later (`review-rebase`).

## Accept / reject

Accept = `PATCH /article-suggestions/{sug}` with `status: "accepted"` (rest.md rule 3; side params ride along):
```json
{ "status": "accepted", "hunks": "all" | ["h1","h3","meta:title"], "publish": true, "collection_id": "col_...", "slug": "override-slug", "note": "thanks", "acknowledge_warnings": false }
```
`status` change never mixed with proposal edits (`proposed_*`) in one body: 422. Client sends `accepted` or `rejected`; server lands `accepted` or `partially_accepted` (step 7).
Transaction (action `AcceptSuggestion`):
1. Lock suggestion row `FOR UPDATE`, require `pending`. Lock article row.
2. Stale check (inside tx): working copy != `base_markdown` -> 409 `suggestion_stale`.
3. Selected hunks must exist. Unselected hunks count as rejected.
4. Apply selected hunks to working copy (== base; in order, offsets adjusted) -> new body. Metadata pseudo-hunks set fields.
5. Validate result (size, Markdown renders, slug unique). Slug collision on create: auto-suffix `-2`, `-3` and report in response `warnings`.
6. Update working copy, `lock_version++`. Create version kind `suggestion` (suggestion_id set). If `publish` and article status draft/published -> `PublishArticle` within same tx (version kind `published`). If article archived -> 409 `article_archived` (unarchive first: `PATCH /articles/{art} {status: "draft"}`).
7. Status: all hunks accepted -> `accepted`; some -> `partially_accepted`; reviewed_by, reviewed_at, result_version_id. Batch counters updated.
8. Other pending suggestions for same article become `stale` (computed, nothing queued).
9. Events: `article_suggestion.reviewed`, plus `article.created`/`article.published`/`article.updated` as applicable.

Reject = `PATCH /article-suggestions/{sug} {status: "rejected", note}` -> `rejected`. Rejection note stored; MCP `dimmi_list_article_suggestions` returns note so proposer agent learns.

Selecting zero hunks on accept: 422 `no_hunks_selected` (use reject).

Batch accept `PATCH /article-suggestion-batches/{sugb} {status: "accepted", publish: true}`: processes pending suggestions in batch in chunks of 20 per transaction (not one giant tx); per-suggestion result list returned (`accepted`, `skipped_stale`, `failed` with code). Two pending edits on same article in one batch: first accepted, second `skipped_stale`. Create suggestions with collection paths processed first (sorted by path) so collections exist before children. Response 200 with results; partial success never silent. Batch reject `PATCH {status: "rejected", note?}` rejects all pending. Batch `status` is derived from counters (`pending`, `accepted`, `rejected`, `mixed`; README "Review state"), never stored.

## Supersede and dedupe

- Same proposer submits new suggestion for same article (`kind=edit`) while older pending from same `proposed_by` + same `source_kind`: older -> `superseded`.
- `dedupe_key` (client provided or computed `sha256(kind, article_id or slug, proposed_markdown)`): identical pending proposal returns existing suggestion (200, `Idempotent-Replayed`-like `deduplicated: true`), no new row.
- Proposal identical to current content (no hunks): 422 `no_changes`.
- Import-mode sync: each changed document produces one suggestion; earlier pending suggestion for same document superseded.

## Provenance shapes (`source_ref`)

| source_kind | source_ref | source_url |
|---|---|---|
| agent | `{agent: "claude-code", client_id: "ocl_...", repo: "acme/app", commit_sha: "9f2c...", paths: ["src/billing/refund.ts"], prompt_id: "dimmi_draft_help_center"}` | `https://github.com/acme/app/commit/9f2c...` when repo is GitHub and sha given, else null |
| import | `{knowledge_source_id, knowledge_document_id, path or url, commit_sha?}` | file URL on GitHub at commit, or page URL |
| member | `{member_id}` | null |
| api | `{token_id}` | caller-provided `source_url` (https only) |
| commit (docs sync, [docs-sync.md](docs-sync.md) 6.7) | `{repo, repo_id, github_repository_id, pr_number, pr_url, merge_commit_sha, base_sha, docs_sync_source_id, docs_sync_run_id, paths[], screen_summary}` | PR URL (push: compare URL) |
| conversation (reserved; later `conversation-drafts` with knowledge-gap report) | `{conversation_id}` | dashboard conversation URL |

Caller-provided `source_url` validated: https, max 2,048, rendered as `rel=noopener noreferrer`.

## Cold-start generator

Goal: empty workspace to full draft help center in minutes. Dimmi server never reads user repo (unless Git source connected); user's local coding agent (Claude Code, Cursor, Codex) reads code and calls Dimmi.

### Flow

1. User connects Dimmi MCP (09) or runs CLI. Asks agent: "Draft our help center" (or `npx dimmi init` step "seed help center", which prints prompt for agent or runs `dimmi docs push` on existing `docs/`).
2. Agent fetches MCP prompt `dimmi_draft_help_center` (arguments: `audience` = end users default, `tone`, `max_articles` default 30, `help_center`).
3. Agent calls `dimmi_get_help_center_outline` (avoid duplicates, reuse collections).
4. Agent reads repo locally (README, routes, UI copy, settings pages, pricing config, error messages, OpenAPI) and writes articles.
5. Agent calls `dimmi_propose_help_center` with batch (or many `dimmi_propose_article`). Server validates, renders preview, computes stats, returns batch URL in dashboard.
6. Human reviews in queue; accept all or per article; publish.

### MCP prompt `dimmi_draft_help_center` (content summary, full text in `app/mcp/prompts/draft_help_center.md` embedded)

- Write for end customers of the product, not developers, unless `audience=developers`.
- Only describe behavior visible in code on default branch; never invent features, prices, limits. Unknown -> omit.
- Never include secrets, env var values, internal hostnames, admin-only screens, other customers' data. Mark internal policy (refund rules, escalation) as `visibility: agent_only`.
- One task per article; title as user question or task ("Reset your password"); 150 to 800 words; steps numbered; UI labels in bold exactly as in code.
- Collections: 3 to 8 top-level, max depth 2.
- Provide `source_ref.paths` per article (files used) and `commit_sha` (`git rev-parse HEAD`).
- Use `base_version_id` from `dimmi_get_article` when editing.
- Markdown dialect: CommonMark + GFM tables + `> [!NOTE]`/`[!TIP]`/`[!WARNING]` alerts; no raw HTML; images only as existing public https URLs (no local paths; upload not available via MCP at MVP).

### Batch request (REST and MCP same schema)

```json
{
  "help_center_id": "hc_01jc...",
  "title": "Initial help center from acme/app@9f2c1e",
  "source_kind": "agent",
  "source_ref": { "agent": "claude-code", "repo": "acme/app", "commit_sha": "9f2c1e7a..." },
  "items": [
    {
      "kind": "create",
      "title": "Reset your password",
      "slug": "reset-password",
      "description": "Get back into your account when you forget your password.",
      "collection_path": "Account/Sign in",
      "visibility": "public",
      "markdown": "Forgot your password? ...",
      "summary": "Covers /forgot-password flow",
      "source_ref": { "paths": ["app/routes/forgot-password.tsx"] },
      "dedupe_key": "acme-reset-password"
    },
    {
      "kind": "edit",
      "article_id": "art_01jc...",
      "base_version_id": "artv_01jc...",
      "markdown": "...full new body...",
      "summary": "Plan limits changed in pricing.ts"
    }
  ]
}
```
Validation per item; invalid items returned in `errors` with index (`items.3.markdown`), valid items still created (partial batch) unless `?atomic=true` (then 422 all-or-nothing). Max 200 items, 10 MB body, each Markdown 256 KB.

### Markdown folder push (no LLM)

`dimmi docs push ./docs` (09 CLI) reads files, front matter, uploads referenced local images first via `POST /help-centers/{hc}/assets` (CLI rewrites image paths to asset URLs), then sends batch with `source_kind=import`, `source_ref.paths`. Existing article matched by front matter `slug` or path mapping stored in `source_ref` of previous accepted suggestion -> edit suggestion instead of create. Same review flow.

### Git source in import mode

Sync detects changed files -> `CreateImportSuggestions`: new file -> create suggestion; changed file with linked article -> edit suggestion with base = current working copy, proposed = new file content (Dimmi-side edits show as hunks; reviewer rejects those hunks; 3-way from `imported_hash` later: `review-rebase`); deleted file -> archive suggestion. Provenance link to file at commit. Pure file diff, no LLM. LLM drafting from merged code changes: docs sync ([docs-sync.md](docs-sync.md)); import-owned paths excluded there.

## Review queue API views (for frontend)

- `GET /article-suggestions?filter[status]=pending&sort=-created_at` list rows carry: kind, title (proposed or article), article slug, additions, deletions, hunk count, `stale`, source_kind, source label (`"claude-code @ 9f2c1e"`), source_url, batch, created_at, proposed_by.
- `GET /help-centers/{hc}/collections` includes `pending_suggestions` count per collection (nav counters like Productlane).
- `GET /article-suggestions/{sug}` returns current article markdown + hunks with line numbers, so client renders article left with highlights and rail of hunk cards right (hover lights location via `new_start`/`old_start`).
- Realtime: `article_suggestion.created` and `.reviewed` on `workspace:{ws}` channel (centrifuge) keep queue live; reconnect recovers from channel history, else refetch list.
