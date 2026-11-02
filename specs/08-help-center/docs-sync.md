# 08 Docs sync: self-updating docs from merged PRs + article freshness

Style: caveman. Date 2026-11-02. Owner 08-help-center. Roadmap launch item 16 (`research/roadmap.md`, audit gap G3; freshness = launch-sized slice of G14). Tables, routes, jobs, config, permissions, entitlements canonical in [README.md](README.md); this file = design, pipeline, prompts, JSON, rules. Edge cases E209-E262 in [edge-cases.md](edge-cases.md) section R. Tasks M11 in [tasks.md](tasks.md).

## 1. Goal and bar

Merged PR on GitHub -> Dimmi finds help articles it makes wrong -> drafts precise edits with BYOK model (07 `ai.RunTask`) -> one suggestion batch per PR in 08 review queue -> human accepts per hunk -> publish. Never auto-publish. Plus weekly freshness scan: articles whose referenced code changed, whose links broke, or whose readers vote them down get `needs_review`.

Why: Productlane and Ferndesk core pitch (`research/competitors/ferndesk.md` "Automatic PR drafts", "Continuous verification"; `productlane.md` "Self-updating help center"). Vibe coders merge daily; docs rot daily; stale docs make 07 agent answer wrong. Our edge: open source, self-host, BYOK (zero AI markup, code only to customer's own key), prompts in repo, skipped-PR log with reasons, MCP path that costs zero server tokens.

| Bar | Target |
|---|---|
| Webhook ack | under 1 s (202), GitHub times out at 10 s |
| Merge to run row (`queued`) | under 5 s webhook mode; under 16 min poll mode |
| Merge to screened (skip or proceed) | p95 under 60 s |
| Merge to suggestions in review queue (up to 5 drafts) | p95 under 3 min (dominated by provider) |
| Every merged PR on watched branch accounted for | 100%: run row `completed`, `skipped` (with reason) or `failed`; reconcile poll catches lost deliveries within 6 h |
| Customer code at rest in Dimmi | zero: no patch text in Postgres, Valkey, River job args, logs, traces (leak test greps marker string) |
| Code leaves Dimmi only to | workspace's own BYOK key via 07 (07: no Dimmi-owned fallback key) |
| Edits precise | anchored find/replace, hunks only where model changed text; eval target median under 15 changed lines per suggestion (eval, not CI) |
| Nothing auto-publishes | enforced: docs sync code path never calls `AcceptSuggestion` or `PublishArticle` (arch test) |
| Cost per PR | shown per run (`stats.cost_micros`); typical under $0.10, worst case about $0.60 with defaults (section 9) |
| Freshness scan, 1,000-article help center | under 30 s deterministic part; GitHub calls: 1 compare per (repo, distinct verified sha), capped |

## 2. Scope

### In (MVP)

- Shared GitHub layer (08 owns, 13 reuses): `github_connections` (fine-grained PAT, `kind` registry enum), `github_repositories` (repo + one webhook + secret), one webhook route per repo `/webhooks/github/repositories/{ghr}` with handler registry (08 knowledge git push, 08 docs sync, 13 changelog release). Auto-install webhook when token has Webhooks write; manual fallback; polling fallback when no webhook.
- Docs sync sources: (help center, repo, branch) with triggers, sensitivity, path/label/author filters, guidance, caps.
- Triggers: `pull_request` closed+merged into branch; direct `push` to branch not from PR (debounced 10 min); poll + reconcile; manual run (REST/CLI/MCP); redraft one article; fix from freshness.
- Pipeline: gate -> fetch PR files -> path filter -> digest (redacted, in memory) -> screen (classifier) -> map to articles (explicit `docs:` refs, source refs, hybrid search) -> draft anchored edits per article (answer model) -> suggestions (`source_kind commit`) in one batch per run -> events.
- Optional create suggestion (new article) when screen says needed (max 1 per run; off in `strict`); archive suggestion when feature removed.
- Suggestion additions: `anchored_edits` stored; per-hunk rationale + evidence links; `PATCH {base: "current"}` reapplies anchored edits onto current working copy (cheap stale fix, no LLM, no diff3).
- Article source refs (`article_source_refs`): article -> repo paths, learned from accepted suggestions (cold start `source_ref.paths`, docs sync evidence), editable by members.
- Freshness (`article_freshness`): weekly scan, reasons `referenced_code_changed`, `referenced_file_deleted`, `claims_outdated` (AI check on code-flagged articles only, capped), `broken_link`, `negative_feedback`, `not_reviewed_recently`; `needs_review` filter; `PATCH /articles/{art} {verified: true}` (human only); "Draft fix" = run trigger `freshness`.
- Skipped-PR log with reasons, run history 180 days, cost per run.
- MCP toolset `docs_sync` (list runs, list stale articles, request update; prompt for local coding agent). No MCP accept, publish or verify.
- CLI wrappers (09 implements).

### Out (later)

| Name | Item | Why later |
|---|---|---|
| `github-app` | GitHub App connection kind (one-click install, org-owned, auto webhooks, short-lived tokens). `github_connections.kind = app` reserved | install-claim verification, private key ops, self-host app registration; lands with roadmap "GitHub loop" |
| `docs-as-code-pr` | Dimmi opens PR in user's docs repo instead of in-place suggestion (roadmap item 16 bar line 4) | needs Contents + Pull requests write, branch/commit via Git Data API, conflict story with import mode |
| `docs-pr-check` | Pre-merge GitHub check: "this PR makes 3 articles wrong", drafts attached | needs Checks write (GitHub App) |
| `conversation-drafts` | Drafts from resolved conversations (roadmap item 16 bar line 1, second half) | ships with post-MVP: knowledge-gap report; `source_kind conversation` stays reserved |
| `continuous-verification` | Claim-level AI verification of every article on schedule (Ferndesk tabs Needs review / Up to date / Can't verify) | MVP AI-checks only code-flagged articles |
| `docs-sync-gitlab` | GitLab, Bitbucket | provider registry ready (`kind`) |
| `docs-sync-changelog` | Run summary prefills 13 changelog draft | 13 has own PR range generation |
| `docs-sync-multi-repo-runs` | One change spanning repos (monorepo split) | one run per (source, change) at MVP |

Roadmap bar deviations (PO confirm): `conversation-drafts` and `docs-as-code-pr` named in roadmap item 16 bar, deferred here.

## 3. Decisions

| # | Decision | Rejected | Reason |
|---|---|---|---|
| D1 | GitHub auth = fine-grained PAT stored once per workspace in `github_connections` (kind `token`), shared by 08 knowledge, 08 docs sync, 13 changelog. Classic PAT accepted with warning | GitHub App now; OAuth app | PO bar: "token shared with 08 git source / 13 changelog source". OAuth app `repo` scope = write to every private repo (too broad). App = best UX but install-claim verification + self-host app registration; reserved as `kind app` (later `github-app`), consumers never see kind |
| D2 | One webhook per repo (`github_repositories`), handler registry dispatches by event | webhook per feature source (per knowledge source, per changelog source) | user configures one hook per repo, not three; one secret; auto-install updates event list as consumers added |
| D3 | Polling fallback + reconcile | webhook only | GitHub does not auto-redeliver failed deliveries; self-host behind NAT has no inbound URL; token without Webhooks write |
| D4 | Never auto-publish, never auto-accept | auto-publish trivial edits | landscape rule (Ferndesk, Mintlify, Promptless, Productlane all human-gated); review-queue.md rule 1 |
| D5 | Model returns anchored edits `{find, replace}`; server applies exact match, builds full proposed Markdown, server diff makes hunks | full rewrite of article; line-number patches | full rewrite drifts (touches unrelated lines, costs output tokens); line patches break. Anchors keep diffs minimal and enable cheap reapply on stale (section 7.4). Proposal still stored as full Markdown (review-queue.md principle holds for external proposers) |
| D6 | Diff text only in memory of one job; never persisted, logged or in job args; re-fetched from GitHub on retry | store digest encrypted between steps | strongest privacy claim, simplest to test; GitHub fetch cheap |
| D7 | One run per (source, change); one suggestion batch per run (even with 1 item) | one suggestion per PR spanning articles | review UI groups by PR; batch accept; provenance one click |
| D8 | Screen with classifier model before mapping/drafting | draft for every PR | most PRs are internal; screen costs about 1/10 of a draft |
| D9 | Freshness deterministic first; AI only on code-flagged articles, capped per week | AI verify every article weekly | cost bounded, low false-alarm rate; full claim verification later (`continuous-verification`) |
| D10 | Articles managed by import-mode git source excluded from drafting | edit them anyway | import pipeline would revert Dimmi edits next sync (E109); repo file is truth there (later `docs-as-code-pr`) |
| D11 | Run includes only PRs merged after source created | backfill history | enabling never floods queue or spends BYOK on old PRs; manual run covers "try on last PR" |
| D12 | Code drift signals only for repos with a docs sync source on that help center | any connected repo | "Draft fix" needs a source (branch, filters, guidance); keeps one config surface |

## 4. GitHub layer (`app/actions/github`, adapter `app/services/github`)

### 4.1 Connection

- Create: `POST /github-connections {token, name?}`. Token prefix `github_pat_` (fine-grained) or `ghp_` (classic: warning `classic_token_broad_scope`); other prefixes 422 `github_token_invalid`. Validate `GET /user` (login, id). Response header `GitHub-Authentication-Token-Expiration` -> `token_expires_at`. Store `encryption.Sealed`, `token_last4`.
- Dashboard deep link to GitHub fine-grained token page with prefilled name, description, expiry and permissions (URL params) [verify at build that GitHub accepts prefill params for every permission listed; fallback: numbered steps + screenshot].
- Permissions are per repo for fine-grained tokens and not introspectable. Probed per repository at add and on `POST /github-repositories/{ghr}/checks` (section 4.2). Missing permission -> 403 from GitHub carries `X-Accepted-GitHub-Permissions` header; adapter maps to `github_permission_missing` with list.
- Rotate: `PATCH /github-connections/{ghc} {token}`: any valid token (owner may change); repositories re-probed (job); repo new token cannot see -> repo `status error github_access_lost` (E256).
- Delete: 409 `github_connection_in_use` while any non-deleted repository references it (list returned). Member detaches or deletes repos first.
- Daily `CheckGithubConnections`: `GET /user`; 401 -> `status error github_token_invalid`; expiry within 7 days -> event `github_connection.errored` code `github_token_expiring` (status stays active; 06 system alert to owners/admins, once per day); expired -> `github_token_expired`.

Required token permissions (repository permissions, read unless stated):

| Consumer | Metadata | Contents | Pull requests | Webhooks (optional) |
|---|---|---|---|---|
| 08 knowledge git source | read | read | no | write: auto-install hook |
| 08 docs sync | read | read | read | write: auto-install hook |
| 13 changelog source | read | read | read | write: auto-install hook |

### 4.2 Repository

- Add: `POST /github-repositories {full_name, connection_id?}`. `connection_id` null allowed only for public repo (unauthenticated, 60 req/h per IP): usable by knowledge git source only (daily schedule max, existing rule); docs sync and 13 require connection (422 `github_connection_required`).
- Resolve `GET /repos/{owner}/{repo}` -> `repo_id`, `full_name`, `default_branch`, `private`. Probe permissions: Metadata (repo GET), Contents (`GET /repos/{o}/{r}/commits/{default_branch}` with `Accept: application/vnd.github.sha`), Pull requests (`GET /repos/{o}/{r}/pulls?state=closed&per_page=1`), Webhooks (`GET /repos/{o}/{r}/hooks?per_page=1`; 200 = read; write assumed with read for fine-grained tokens, confirmed by create). Stored `permissions` `{metadata, contents, pull_requests, webhooks}` each `none|read|write`.
- Webhook install (`webhook_mode`):
  - `auto` when Webhooks write: `POST /repos/{o}/{r}/hooks {name: "web", config: {url, content_type: "json", secret, insecure_ssl: "0"}, events, active: true}`; `github_hook_id` stored; GitHub sends `ping`. Consumer added or removed -> `PATCH /repos/{o}/{r}/hooks/{id} {events}` (job `SyncGithubRepositoryWebhook`, unique per repo). Repository delete -> `DELETE` hook (best effort; 404 ignored).
  - `manual` otherwise: response `meta.webhook {url, secret (shown once), events, content_type: "json"}` + steps. Secret rotate: `PATCH /github-repositories/{ghr} {rotate_webhook_secret: true}` (shown once again).
  - `none`: member chooses polling only (`webhook_mode: none`), or app URL not public (11 doctor `github.app_url_public` warns: dashboard defaults to `none` + polling).
- `webhook_events` = union of `Events()` of registered handlers whose `Wants(repo)` is true (knowledge git source on repo -> `push`; docs sync source -> `pull_request`, `push`; 13 changelog source -> `release`). Empty union with auto mode -> hook kept with `push` only (cheap) until repo deleted.
- `webhook_status`: `pending` (created, no valid delivery yet), `active` (valid delivery or ping within 7 days, or auto hook `last_response.code` 2xx), `failing` (auto: hook `last_response` non-2xx; manual: no delivery 7 days while repo `pushed_at` newer), `none` (mode none). Daily `CheckGithubRepositories` refreshes; transitions emit `github_repository.webhook_status_changed`.
- Rename/transfer: deliveries carry `repository.id`; `full_name`, `default_branch` refreshed from payload. API 404/301 on old name -> resolve `GET /repositories/{repo_id}` and update.
- Delete: 409 `github_repository_in_use` while referenced by knowledge source, docs sync source, or 13 changelog source (body `consumers: [{type: knowledge_source|docs_sync_source|changelog_source, id, name}]`; 13 FK `on delete restrict` backs it).

### 4.3 Webhook route and dispatcher

`POST /webhooks/github/repositories/{ghr}` (web, no auth middleware, `throttle:webhooks`, body limit 25 MB = GitHub max payload):

1. Load repo by public id (unknown or deleted -> 404; GitHub marks delivery failed, no retry storm).
2. Verify `X-Hub-Signature-256` = `sha256=` HMAC-SHA256(body, secret) via `hashing.HMAC`, `hashing.Equal` (constant time). Bad -> 401, log `github_webhook_signature_invalid` (no body logged), metric.
3. Payload `repository.id` must equal `repo_id` (secret leaked to other repo cannot inject) -> else 401.
4. `ping` -> `webhook_status active`, 200.
5. Dedupe `X-GitHub-Delivery`: Valkey `SET NX EX 24h` fast path; River unique job `(repo, delivery id)` durable.
6. Parse compact envelope per event (never raw payload into job args): `pull_request` `{action, number, merged, merged_at, merge_commit_sha, base_ref, base_sha, head_sha, title (300), body (8,000, stored only in job args, never logged), labels[], author_login, author_type, html_url}`; `push` `{ref, before, after, forced, created, deleted, head_commit_id, commit_count}`; `release` (13 shape, `13-changelog/api.md` "GitHub release envelope": `{action, release_id, tag_name, name, body (64 KB cap, `body_truncated`), draft, prerelease, html_url, published_at, author_login}`; forwarded to handler `changelog`, which keeps actions `published`, `released`, `edited`). Other events 204.
7. Enqueue `HandleGithubDelivery` (queue `default`), respond 202. Handler loop calls each registered `github.WebhookHandler` whose `Events()` contains event and `Wants(repo)`; each handler only enqueues its own job (fast, idempotent).

```go
// app/actions/github/webhooks.go
type WebhookHandler interface {
    Name() string                    // "knowledge_git" | "docs_sync" | "changelog"
    Events() []string                // GitHub event names
    Wants(ctx context.Context, a *app.App, repo models.GithubRepository) (bool, error)
    Handle(ctx context.Context, a *app.App, d Delivery) error // enqueue only
}
type Delivery struct {
    ID, Event, Action string
    Repo              models.GithubRepository
    Envelope          json.RawMessage // compact per-event struct, no code
    ReceivedAt        time.Time
}
func RegisterWebhookHandler(h WebhookHandler) // routes/github.go at boot; duplicate Name panics
func ClientFor(ctx context.Context, a *app.App, ws, repoID ids.UUID) (*github.Repo, error) // token decrypted in memory; ErrNotFound other workspace; never logs token

// app/actions/github/add_github_repository.go (also called by 13 `POST /changelog-sources {full_name, connection_id}`)
func AddRepository(ctx context.Context, a *app.App, in AddRepositoryInput) (models.GithubRepository, error)
type AddRepositoryInput struct {
    WorkspaceID  ids.UUID
    FullName     string    // owner/name
    ConnectionID *ids.UUID // required for docs sync and 13 (422 github_connection_required)
    Actor        contracts.Actor
}
// existing row for (workspace, repo_id) reused (idempotent); resolve + probe + hook install per 4.2
```

### 4.4 GitHub API budget

All via `app/services/github` (go-github/v93 on `a.HTTP.StdClient()`, retries off, host allowlist `api.github.com` + `codeload.github.com`, `X-GitHub-Api-Version` pinned). Every call checks `X-RateLimit-Remaining`; under 100 -> job snoozed until `X-RateLimit-Reset` (max 1 h), run stage kept. Secondary rate limit (403/429 with `Retry-After`) -> snooze `Retry-After`.

| Step | Endpoint | Calls per PR |
|---|---|---|
| PR files with patches | `GET /repos/{o}/{r}/pulls/{n}/files?per_page=100` | 1 to 3 (cap `docs_sync.max_pr_files` 300) |
| PR (manual/poll only; webhook carries it) | `GET /repos/{o}/{r}/pulls/{n}` | 0 or 1 |
| Push: PR coverage | `GET /repos/{o}/{r}/commits/{after}/pulls` | 1 |
| Push: range | `GET /repos/{o}/{r}/compare/{before}...{after}` | 1 (files capped 300 by GitHub) |
| Poll | `GET /repos/{o}/{r}/pulls?state=closed&base={branch}&sort=updated&direction=desc&per_page=50` | 1 to 3 per poll |
| Freshness | `GET /repos/{o}/{r}/compare/{verified_sha}...{branch}`; truncated: `GET /repos/{o}/{r}/commits?sha={branch}&path={p}&per_page=1` | 1 per distinct sha; fallback max 100 per scan |

## 5. Docs sync sources

`docs_sync_sources` (README data model). One per (help center, repo, branch). Create `POST /docs-sync-sources` (`docs_sync:manage`, entitlement `docs_sync.max_sources`).

```json
{
  "help_center_id": "hc_01jc...",
  "github_repository_id": "ghr_01jc...",
  "branch": "main",
  "triggers": ["pull_request", "push"],
  "sensitivity": "balanced",
  "include_paths": [],
  "exclude_paths": ["**/*_test.go", "**/*.test.*", "**/*.spec.*", "**/__tests__/**", "**/test/**", "**/tests/**", "**/fixtures/**", "**/*.snap", "**/*.lock", "**/package-lock.json", "**/pnpm-lock.yaml", "**/yarn.lock", "**/go.sum", "**/Cargo.lock", "**/poetry.lock", "**/composer.lock", "**/Gemfile.lock", ".github/**", "**/node_modules/**", "**/vendor/**", "**/dist/**", "**/build/**", "**/*.min.*", "**/*.map", "**/*.{png,jpg,jpeg,gif,webp,avif,ico,svg,woff,woff2,ttf,pdf,zip}"],
  "ignore_labels": ["no-docs", "skip-docs", "dependencies"],
  "force_labels": ["docs"],
  "ignore_authors": ["dependabot[bot]", "renovate[bot]"],
  "guidance": "Audience: non-technical store owners. Never mention API internals. Our product name is Acme Shop.",
  "allow_new_articles": true,
  "max_drafts_per_run": 5,
  "max_suggestions_per_day": 20
}
```
- `exclude_paths` default list shown (request may replace); globs same semantics as knowledge (path-only, `*` segment, `**` across, `{a,b}` alternation).
- `ignore_authors` supports suffix glob `*[bot]`. Bots ignored only by this list, never by `author_type` (teams merging via own bot accounts remove the entry).
- `guidance` max 4,000 chars (Ferndesk parity), passed to every prompt as owner instructions (trusted, but still below system rules).
- `branch` default repo `default_branch`; branch must exist (`GET /repos/{o}/{r}/branches/{branch}` 404 -> 422 `github_branch_not_found`).
- Paths owned by an import-mode knowledge git source on same repo + branch are always excluded (D10, E214); response `meta.excluded_by_import` lists patterns.
- Create also: `webhook_events` union updated (auto hook PATCH), `poll_cursor_at = now()`, `last_seen_head_sha` = branch head (D11).
- Pause: `PATCH {enabled: false}`; deliveries ignored, no run rows. Resume: cursor kept; poll reconcile window starts at resume time (no catch-up of paused period: D11 spirit; E223).

## 6. Pipeline

### 6.1 Triggers and change keys

| Trigger | Source event | `change_key` (unique per source for automatic triggers) | Range |
|---|---|---|---|
| `pull_request` | `pull_request` action `closed`, `merged = true`, `base_ref == branch` | `pr:{repo_id}:{number}` | PR files API (`merge_commit_sha` as head) |
| `poll` | poll/reconcile finds merged PR without run | `pr:{repo_id}:{number}` (same key: webhook and poll never both run) | same |
| `push` | `push` to `refs/heads/{branch}`, not covered by PR | `push:{repo_id}:{before}..{after}` | compare `before...after` |
| `manual` | `POST /docs-sync-runs {source_id, pr_number}` or `{source_id, base_sha, head_sha}` | `manual:{run uuid}` (never deduped) | PR files or compare |
| `redraft` | `POST /docs-sync-runs {replay_of, article_id}` | `redraft:{run uuid}` | same range as `replay_of` run, one article |
| `freshness` | `POST /docs-sync-runs {article_id, trigger: "freshness"}` | `freshness:{run uuid}` | compare `verified_sha...head` for article's refs, claims as guidance |

Push handling (direct commits, vibe coder pushes to `main`):
1. `forced = true` -> run `skipped force_push` (history rewritten; compare meaningless). `deleted = true` or `created = true` (branch create) -> ignored.
2. Debounce: upsert `docs_sync_sources.pending_push` `{before (earliest), after (latest), first_at, pushes}`; schedule `ProcessDocsSyncPush` at `first_at + docs_sync.push_debounce` (10 min, River unique per source). Burst of 8 pushes = one run.
3. At process: `GET /commits/{after}/pulls`; any PR merged into branch -> run `skipped pr_covered` (PR trigger owns it; merge-button pushes always land here). Else run over `before...after`.
4. Mixed push (direct commits plus PR merge in one push window): PR run covers PR; direct commits in same window not drafted separately (E219).

Poll/reconcile (`PollDocsSyncSources`, every 15 min for sources whose repo `webhook_status` not `active`; every 6 h for all): list closed PRs into branch updated since `poll_cursor_at - 1 h` (overlap), merged after source `created_at` (and after last resume), without run for key -> create run trigger `poll`. Poll mode push detection: branch head vs `last_seen_head_sha`; changed -> same as push step 3 with `before = last_seen_head_sha`.

### 6.2 Gate (in order; cheap first)

Ignored, no run row (user intent or not ours): source paused or deleted; help center disabled/deleted; branch mismatch; PR closed without merge; `push` of non-branch ref.

Run row created, then:

| # | Check | Outcome |
|---|---|---|
| 1 | PR author matches `ignore_authors` | `skipped author_ignored` |
| 2 | PR has label in `ignore_labels` | `skipped label_ignored` |
| 3 | PR body line `docs: none`, `[skip docs]` or `[no docs]` (case-insensitive) | `skipped opted_out` |
| 4 | Revert PR (title `Revert "...` or body `Reverts {owner}/{repo}#N`) | `skipped revert`; pending suggestions of reverted PR's runs -> `superseded` (note `reverted by #M`); accepted ones listed in run `stats.reverted_accepted` + dashboard banner "PR #N reverted; 2 accepted doc edits may need undo" (never auto-undone; E217) |
| 5 | Entitlement `docs_sync.runs_daily` (`Entitlements.Limit`, counted on runs reaching stage `fetch`, UTC day) | `skipped daily_cap_reached` (replayable) |
| 6 | Source `max_suggestions_per_day` already reached (suggestions created today by this source) | `skipped suggestion_cap_reached` |
| 7 | 07 readiness: `ai.ResolveProviderKey(ws, classifier)` and `ai.PreflightSpend` | `skipped ai_unavailable` / `skipped spend_cap_reached` |
| 8 | Fetch files; drop `exclude_paths`, import-owned paths, files outside `include_paths` (when set); nothing left | `skipped paths_excluded` (file list kept with `used: false`) |

Explicit refs (section 6.5 a) and `force_labels` never bypass gate checks above; they only bypass the screen threshold (6.4).

### 6.3 Digest (in memory only)

Built per attempt from GitHub, discarded at job end (D6):
- Header: repo, PR number, title, author login, labels, merged_at, base/head shas.
- PR body: first 4,000 chars after stripping HTML comments (`<!-- -->` template text), unchecked checklist lines, image tags.
- File table: path, status (added, modified, removed, renamed with old path), +/-.
- Patches: GitHub `patch` field per file (absent for binary or huge files: listed "patch unavailable"). Keep hunk headers, `+`/`-` lines, 1 context line; per file 3,000 chars; total budget `docs_sync.max_diff_tokens` (12,000 tokens at 3.5 chars/token). File order by score: path in article source refs of this help center (+3), user-facing hints `docs_sync.user_facing_path_hints` (`app/`, `pages/`, `routes/`, `components/`, `views/`, `templates/`, `ui/`, `locales/`, `i18n/`, `lang/`, `pricing`, `plans`, `billing`, `config/`, `settings`, `api/`, `handlers/`, `controllers/`, `cmd/`, `cli/`, `openapi`, `errors`) (+2), added or removed file (+1), larger change (+log(lines)). Overflow files listed without patch, `stats.truncated = true`.
- Secret redaction: every patch line and PR body line through `secretscan.RedactLines` (patterns: `dmi_`, `sk-`, `sk_live_`, `AKIA`, `ghp_`, `github_pat_`, `xox[abpr]-`, `-----BEGIN .*PRIVATE KEY-----`, `(password|secret|token|api_key)\s*[:=]\s*\S{8,}`, base64/hex runs of 32+ chars on assignment lines) -> line replaced `[redacted secret-like line]`; count in `stats.redacted_lines`.
- Wrapped with `ai.WrapUntrusted("pull_request", ...)` before prompt.

### 6.4 Screen (LLM call 1)

`ai.RunTask{Kind: docs_sync, Purpose: "screen_change", ModelRole: classifier, StepKind: classify, ResponseFormat: json, JSONSchema: screen.schema.json, MaxOutputTokens: 800, Redact: false}`. Prompt `resources/prompts/docs_sync/v1/screen.md`. Input: digest + help center outline (collection names + article titles, max 300 titles, 3,000 tokens) + `guidance`.

```json
{
  "user_facing": true,
  "confidence": 0.82,
  "summary": "Team plans now allow up to 10 seats; seat price shown on Billing page.",
  "changes": [
    {"kind": "changed_limit", "description": "Team plan seat limit 5 -> 10", "evidence_paths": ["src/billing/plans.ts"]},
    {"kind": "renamed_ui", "description": "Button 'Upgrade' renamed 'Change plan'", "evidence_paths": ["app/settings/billing/page.tsx"]}
  ],
  "search_queries": ["team plan seat limit", "upgrade plan button", "billing settings page"],
  "new_article": null
}
```
- `kind` enum: `new_feature`, `changed_behavior`, `removed_feature`, `renamed_ui`, `changed_limit`, `changed_pricing`, `changed_error`, `changed_setting`, `deprecation`, `bugfix`, `internal`.
- `new_article`: null or `{title, reason, collection_hint}`.
- `search_queries` max 5, each max 120 chars.

Sensitivity rule (code, not prompt):

| Sensitivity | Proceed when |
|---|---|
| strict | `user_facing` and `confidence >= 0.8` and at least one change kind not in (`bugfix`, `internal`); `new_article` ignored |
| balanced (default) | `user_facing` and `confidence >= 0.6` and at least one change kind not `internal` |
| eager | `user_facing` or `confidence >= 0.4`; `bugfix` counts |

Fail -> `skipped no_user_facing_change`, `screen` stored (summary visible in skipped log; Ferndesk "No documentation changes needed" with reason). Explicit refs or `force_labels` -> proceed regardless; screen still runs (summary + queries feed drafts). Invalid JSON after 07 retry -> run `failed invalid_output`.

### 6.5 Map to articles (no LLM)

Candidates (max `docs_sync.max_candidates` 8) from union, each with score and `matched_by`:

| Signal | How | Score |
|---|---|---|
| a. Explicit | PR body lines `docs: <ref>[, <ref>]` where ref = `art_...`, article slug, or URL on any host of this help center (subdomain, custom domain, path mode; resolved via redirects too). Unknown ref -> run warning `docs_ref_unresolved` | 1.0, always drafted |
| b. Source refs | `article_source_refs` path equals changed file or is directory prefix of it (renamed files: old and new path) | 0.9 |
| c. Hybrid search | `contracts.Knowledge.Search` per query (`search_queries` + PR title + `summary`): `HelpCenterIDs [hc]`, `ExcludeSources`, `ExcludeChangelog`, `Visibility [public, agent_only]`, `Mode auto`, `Limit 8`, `MaxPerDocument 2`, `Surface docs_sync`, `LogSearch false`; per-query ranks fused by RRF (k 60) across queries at article level | normalized fused score; kept when >= `docs_sync.min_search_score` (0.35) |

- Only published articles appear (knowledge index has published versions only). Draft-only articles never candidates (E226).
- Articles linked to import-mode knowledge document (`knowledge_documents.linked_article_id`, source mode `import`) -> outcome `import_managed`, not drafted (D10).
- Article with pending docs sync suggestion from same source for same change key -> skipped (idempotent retry).
- Query embedding spend: `KnowledgeSearchResult.EmbeddingUsage` recorded by 08 via 07 `RecordAiSpend` kind `embedding`. No embeddings key -> FTS (`Mode` falls back); explicit and source refs still work.
- Order: explicit, refs, search; ties by article `updated_at desc`. Cut to `max_drafts_per_run` (rest outcome `over_cap`, listed in run).
- No candidate and no `new_article` (or `allow_new_articles` false, or strict) -> `skipped no_matching_articles`.

### 6.6 Draft (LLM call per candidate)

`ai.RunTask{Kind: docs_sync, Purpose: "draft_edit", ModelRole: answer, StepKind: generate, ResponseFormat: json, JSONSchema: draft_edit.schema.json, MaxOutputTokens: 3000}`. Prompt `resources/prompts/docs_sync/v1/draft_edit.md`. Calls sequential within run (per-workspace AI concurrency respected by 07 `ai.workspace_concurrency`).

Input blocks (system cacheable first):
1. System rules (cacheable): customer-facing language; change only what the code change makes wrong or incomplete; keep author voice, headings, anchors; never invent behavior not in evidence; never include code, file paths, env var names, internal hostnames, secrets, PR numbers or author handles in article text; UI labels in bold exactly as in code; Markdown dialect = help dialect (no raw HTML); return anchored edits only.
2. Owner `guidance` + last 10 reviewer rejection notes on docs sync suggestions of this help center (max 1,500 chars; "reviewer preferences").
3. Change: `summary`, `changes[]`, patches of `evidence_paths` + paths matching this article's source refs (budget 4,000 tokens, untrusted envelope).
4. Article: title, description, full working copy Markdown with line numbers stripped (budget `docs_sync.max_article_tokens` 16,000). Over budget: sections (H2 blocks) whose heading path matched search chunks or contain query terms, max 16,000 tokens; edits outside sent sections impossible (anchors must match).
5. Pending suggestions on this article (other sources/PRs): summary + replace texts, max 2,000 chars ("do not duplicate").

Output:
```json
{
  "relevant": true,
  "reason": "Article states Team plan has 5 seats and names the Upgrade button.",
  "confidence": 0.9,
  "edits": [
    {
      "find": "The Team plan includes up to 5 seats.",
      "replace": "The Team plan includes up to 10 seats.",
      "why": "Seat limit raised to 10.",
      "evidence": [{"path": "src/billing/plans.ts", "line": 42}]
    },
    {
      "find": "click **Upgrade**",
      "replace": "click **Change plan**",
      "why": "Button renamed.",
      "evidence": [{"path": "app/settings/billing/page.tsx", "line": 88}]
    }
  ],
  "title": null,
  "description": null,
  "archive": false
}
```
- `relevant false` -> outcome `not_affected` (counts as review of this change for freshness: section 8.3).
- Max 12 edits; `find` 1 to 2,000 chars, must occur exactly once in working copy after normalization (NFC, CRLF->LF, trailing spaces trimmed per line). Zero or multiple matches -> edit dropped (`unmatched_edits` count). All dropped -> outcome `unmatched_edits`, no suggestion (E229). Overlapping finds -> later dropped.
- `replace` validated: renders with help dialect, no raw HTML, `linkguard.Filter` allowlist = help center hosts + URLs already in article + public repo URL (private repo URLs stripped to text), `secretscan` hit -> suggestion warning `possible_secret` (accept needs ack, existing rule).
- `archive true` allowed only when screen has `removed_feature` change; creates suggestion kind `archive` with `summary` = reason (E231).
- `title`/`description` non-null -> metadata pseudo-hunks (review-queue.md).
- Proposed Markdown = working copy with edits applied in document order. Equal to working copy -> outcome `not_affected`.

New article (`Purpose: "draft_article"`, max 1 per run, not in strict, `allow_new_articles`): input change + outline + `new_article` hint; output `{title, slug, description, collection_path, visibility, markdown}` validated like MCP create proposals (review-queue.md), `collection_path` must name existing collection or one new top-level (no depth 3). Creates suggestion kind `create`.

Invalid output after 07 JSON retry -> candidate outcome `failed invalid_output`, other candidates continue; run `completed` with `stats.failed_candidates`. Provider error -> job retry (3, backoff 1, 5, 15 min); candidates already done keep their suggestions (dedupe key).

### 6.7 Assemble suggestions

- Batch: created at first suggestion, one per run: `title` = `PR #128: Raise team seat limit` (push: `Push 9f2c1e..a71b03 (3 commits)`), `source_kind commit`, `source_ref` = run ref below, `created_by_type system`.
- Suggestion per drafted article: kind `edit` (or `create`, `archive`), `base_markdown` = working copy at draft time, `base_version_id` = published version, `proposed_markdown`, `summary` = `reason` + " (from PR #128)", `anchored_edits` = validated edits, `source_kind commit`, `dedupe_key` `docs_sync:{source_id}:{change_key}:{article_id|new}`, `proposed_by_type system`, `proposed_by_id` = source id.
- `source_ref`:
```json
{
  "repo": "acme/app", "repo_id": 81234567, "github_repository_id": "ghr_01jc...",
  "pr_number": 128, "pr_url": "https://github.com/acme/app/pull/128",
  "merge_commit_sha": "a71b03c...", "base_sha": "9f2c1e7...",
  "docs_sync_source_id": "dss_01jc...", "docs_sync_run_id": "dsr_01jc...",
  "paths": ["src/billing/plans.ts", "app/settings/billing/page.tsx"],
  "screen_summary": "Team plans now allow up to 10 seats..."
}
```
- `source_url` = PR URL; push: `https://github.com/{o}/{r}/compare/{base}...{head}`; manual range same.
- Supersede: redraft (same run, same article) supersedes older pending suggestion. Different PRs on same article: both stay pending (E233); second goes `stale` after first accepted; reviewer reapplies (7.4).
- Run finish: `status completed`, `suggestion_count`, `batch_id`, `stats` (tokens, cost from each `RunTaskResult`, `ai_run_ids`), event `docs_sync_run.completed`. Zero suggestions after drafting (all `not_affected`/`unmatched_edits`) -> `completed` with `suggestion_count 0` (shown as "checked 3 articles, no change needed").

### 6.8 Job

`RunDocsSync` (`run_docs_sync`), queue `ai`, unique per run id, timeout `docs_sync.run_timeout` (10 min), retries 3. Stages checkpointed on run row (`stage`, `screen`, `candidates[].outcome`); retry re-fetches GitHub, rebuilds digest, skips finished stages (screen stored -> no second screen call) and drafted candidates (dedupe key). Watchdog in `PollDocsSyncSources`: `running` over 30 min -> `failed stalled`. Cancel: `PATCH /docs-sync-runs/{dsr} {status: "canceled"}` sets flag; job checks between LLM calls; suggestions already created stay pending.

## 7. Review queue additions (review-queue.md applies unchanged otherwise)

### 7.1 Suggestion resource fields (docs sync)

- `source: {kind: "commit", label: "PR #128 acme/app", url}`; `docs_sync_run_id`.
- Hunks carry `notes`: `[{why, evidence: [{path, line, url}]}]` computed at read: each anchored edit's `replace` located in proposed Markdown -> line range -> attached to hunks overlapping it. Evidence URL `https://github.com/{o}/{r}/blob/{merge_commit_sha}/{path}#L{line}` (private repos: link works only for people with repo access; shown only in dashboard/API, never rendered in article).
- Reviewer edits proposal (`PATCH {proposed_markdown}`) -> `anchored_edits` cleared (notes vanish; reapply no longer possible: E236).

### 7.2 Accept side effects (inside `AcceptSuggestion` tx, docs sync suggestions only)

- `article_source_refs` upsert for `source_ref.paths` (origin `docs_sync`, cap 50 per article: oldest non-member evicted).
- Freshness: `verified_shas[repo_id] = merge_commit_sha` (or head) when newer than stored (by merged time), reasons of kind `referenced_code_changed` for that repo cleared; status recomputed (event if changed).
- Accept of any suggestion with `source_ref.repo` + `paths` (cold start `agent`, `import`) also writes refs when repo is connected (`github_repositories.full_name` match, case-insensitive). Repo connected later: `BackfillArticleSourceRefs` (on repository create) scans last 1,000 accepted suggestions with that `source_ref.repo`.

### 7.3 Reject

Note stored (existing). Last 10 notes per help center feed draft prompt (6.6 block 2). All suggestions of a run rejected -> run `stats.all_rejected = true` (accept-rate metric).

### 7.4 Reapply on stale (`PATCH /article-suggestions/{sug} {base: "current"}`)

Docs sync suggestions keep `anchored_edits`, so stale fix needs no LLM and no diff3:
1. Lock suggestion + article; require `pending` and `anchored_edits` not null (else 422 `reapply_not_supported`).
2. Apply each edit to current working copy with same exact-once rule. Matched edits build new proposed Markdown; `base_markdown` = current working copy; `base_version_id` = current published version; hunks recomputed; decisions cleared; `anchored_edits` = matched only.
3. Unmatched edits listed in `meta.warnings` `edit_unmatched` (`{find, why}`). Zero matched -> 409 `edits_unmatched` (reject, or redraft via `POST /docs-sync-runs {replay_of, article_id}`).
4. Not a 3-way merge: edits are re-anchored only where the text they replace is unchanged; nothing else carried. B18 `review-rebase` stays later. Same field on non-docs-sync suggestion: 422 `reapply_not_supported`.

## 8. Freshness (stale detection)

### 8.1 Model

`article_freshness` row per article (README). Created lazily on first scan or first verify. `status` `fresh` | `needs_review`; `needs_review` iff `reasons` non-empty. Article resource gains `freshness {status, reasons, verified_at, last_scanned_at}`; `GET /articles?filter[freshness]=needs_review|fresh` (rows without freshness count as `fresh`).

Help center settings (`help_centers.settings.freshness`, typed `models.HelpCenterSettings`, no migration): `{enabled: true, stale_after_days: 180 (0 = off, 30..730), ai_check: true, ai_checks_per_week: 20 (0..100), negative_feedback: true}`.

### 8.2 Reasons

| Code | Severity | Rule | Needs |
|---|---|---|---|
| `referenced_code_changed` | medium | ref path (or file under ref dir) changed between `verified_shas[repo]` and branch head of a docs sync source repo on this help center | refs + docs sync source |
| `referenced_file_deleted` | high | ref path removed (compare status `removed`); renamed -> ref path updated in place (no reason) | same |
| `claims_outdated` | high | AI check (8.4) verdict `outdated`; detail lists `claims [{quote, problem, evidence_path}]`, each `quote` verified to exist in article (else dropped) | AI key |
| `broken_link` | high | internal link to article/collection now missing, archived, trashed, or agent_only from public article (renderer link check over published version) | none |
| `negative_feedback` | medium | last 30 days `not_helpful >= 10` and `not_helpful / (helpful + not_helpful) >= 0.4` (`help_center_article_stats_daily`) | stats |
| `not_reviewed_recently` | low | `verified_at` (or `published_at`) older than `stale_after_days` | none |

Reason item: `{code, severity, detail, paths[], since_sha, until_sha, commits, claims[], detected_at}`. Same code re-detected keeps `detected_at`.

### 8.3 Verification baseline

`verified_at`, `verified_shas {repo_id: sha}`, `verified_by_type/id` set by:
- Member verify: `PATCH /articles/{art} {verified: true}` (articles:write + `RequireHumanReview`: MCP surface and third-party OAuth refused 403 `human_review_required`; rule 3 state field). Clears all reasons; `verified_shas` = current heads of docs sync repos on help center (queued `RecordArticleVerification` fetches heads; until done, scan uses `verified_at` time fallback 8.5).
- Publish (`PublishArticle`, any path incl. suggestion accept): `verified_at = now`, reasons cleared except `negative_feedback` (votes are about readers; cleared when 30-day window recomputes) ; heads recorded by queued `RecordArticleVerification`.
- Docs sync judgement: candidate outcome `drafted` (on accept, 7.2) or `not_affected` (immediately) -> that repo sha advanced to run head if newer.
- AI check verdict `ok` -> sha advanced to scanned head (`verified_by_type system`, `ai_check`).
- First scan of article with no baseline: baseline = current heads, no code reasons (no flood on enable; E244).

### 8.4 AI claim check (bounded)

Only for articles newly flagged `referenced_code_changed` (not checked for same `until_sha`), `settings.freshness.ai_check`, key ready, `PreflightSpend` ok, help center weekly count under `ai_checks_per_week` (rest stay flagged `referenced_code_changed`, checked next week, oldest flag first).

`ai.RunTask{Kind: docs_sync, Purpose: "verify_article", ModelRole: classifier, ResponseFormat: json, MaxOutputTokens: 1200}`, prompt `verify_article.md`; input article (16,000 tokens budget) + compare patches of ref paths (6,000 tokens, redacted, untrusted). Output `{verdict: "ok"|"outdated"|"unsure", claims: [{quote, problem, evidence_path}]}` (max 10 claims). `ok` -> reason cleared, baseline advanced. `outdated` -> `claims_outdated` replaces `referenced_code_changed`. `unsure` -> stays `referenced_code_changed`, `detail: "ai_unsure"`. Patches in memory only (D6).

"Draft fix" button: `POST /docs-sync-runs {trigger: "freshness", article_id}` -> run over `verified_sha...head` for article's refs, claims passed as extra guidance; one candidate (this article); suggestion as 6.7. Needs docs sync source for repo (D12; else 422 `docs_sync_source_required`).

### 8.5 Scan job

`ScheduleFreshnessScans` (maintenance, daily): help centers with `freshness.enabled`, last scan older than `docs_sync.freshness.scan_interval` (7 days), spread by `hash(help_center_id) % 7` weekday. Enqueue `ScanArticleFreshness` (queue `default`, unique per help center).

Scan steps:
1. Load published articles, refs, freshness rows (batches of 500).
2. Per docs sync repo: branch head (1 call); group articles by `verified_shas[repo]`; per distinct sha `compare sha...head` (files up to 300). Compare `status 404` (sha gone after force push) -> baseline reset to head, no reason (E247). Files truncated (300) -> per-ref fallback `commits?path=p&per_page=1` newest commit date vs `verified_at` (max 100 calls per scan; remainder next scan). Baseline only `verified_at` (heads not yet recorded) -> same fallback.
3. Link check, feedback ratio, age: pure SQL/Go over published versions + stats.
4. Upsert freshness rows; status change -> event `article.freshness_changed` (one per article, coalesced per scan into realtime batch).
5. Enqueue `VerifyArticleClaims` (queue `ai`, unique (article, until_sha)) per newly code-flagged article within weekly cap.

Manual scan: `POST /help-centers/{hc}/freshness-checks` (202, `articles:write`, 1 per hour per help center else 429 `rate_limited`).

## 9. Noise control and cost guard

| Knob | Where | Default |
|---|---|---|
| Sensitivity | source | balanced |
| Ignore labels / force labels / ignore authors / opt-out line | source / PR | `no-docs, skip-docs, dependencies` / `docs` / dependabot, renovate / `docs: none` |
| Path include/exclude | source | tests, lockfiles, CI, vendor, build, binaries excluded |
| Max drafts per run | source | 5 (cap 10) |
| Max suggestions per day per source | source | 20 (0 = unlimited) |
| Runs per day per workspace (fair use) | entitlement `docs_sync.runs_daily` | cloud 200, self-host unlimited |
| Sources per workspace | entitlement `docs_sync.max_sources` | cloud 10, self-host unlimited |
| AI freshness checks per week | help center setting | 20 |
| Spend | 07 `PreflightSpend` at gate + inside every `RunTask`; cap hit mid-run -> remaining candidates outcome `spend_cap_reached`, run `completed` partial | 07 cap |
| Accept-rate nudge | dashboard: source accept rate under 20% over last 20 suggestions -> banner "switch to strict?" (never auto-changed) | |

Token budgets per run (input / output): screen 14,000 / 800 (classifier), draft 22,000 / 3,000 each (answer), new article 10,000 / 4,000, verify 22,000 / 1,200 (classifier). Rough cost at 2026 list prices (Haiku-class classifier, Sonnet-class answer): skipped-at-screen PR about $0.02; typical PR (screen + 1-2 drafts of 2-4k-token articles) $0.05-0.12; worst case default caps (5 drafts, full budgets) about $0.60. Shown before enabling (dashboard estimate from 07 price table x defaults) and per run (`stats.cost_micros`).

Billing: not metered (12): no conversations; LLM spend on customer's key via 07 (`ai_runs.kind docs_sync`).

## 10. API (JSON)

`POST /api/v1/github-connections` (`github:manage`):
```json
{ "token": "github_pat_11AB...", "name": "acme bot token" }
```
201:
```json
{
  "data": {
    "id": "ghc_01jc...", "object": "github_connection", "kind": "token", "name": "acme bot token",
    "account_login": "acme-bot", "token_last4": "x9Qa", "token_expires_at": "2027-05-01T00:00:00Z",
    "status": "active", "last_error_code": null, "repository_count": 0,
    "created_at": "2026-11-02T10:00:00Z", "updated_at": "2026-11-02T10:00:00Z"
  },
  "meta": { "warnings": [] }
}
```
Errors: 422 `github_token_invalid` (bad prefix, 401 from GitHub), 422 `github_token_expired`, 502 `github_unavailable` (GitHub 5xx/timeout).

`GET /api/v1/github-connections/{ghc}/accessible-repositories?q=acme` (computed, not stored; `GET /user/repos` or `GET /installation/repositories` later): `[{full_name, repo_id, private, default_branch, connected: "ghr_..." | null}]`, max 100.

`POST /api/v1/github-connections/{ghc}/tests` (rule 5, synchronous 201 `github_connection_test`): `{ok, account_login, token_expires_at, error_code}`.

`POST /api/v1/github-repositories` (`github:manage`):
```json
{ "full_name": "acme/app", "connection_id": "ghc_01jc..." }
```
201:
```json
{
  "data": {
    "id": "ghr_01jc...", "object": "github_repository", "full_name": "acme/app", "repo_id": 81234567,
    "private": true, "default_branch": "main", "connection_id": "ghc_01jc...",
    "permissions": { "metadata": "read", "contents": "read", "pull_requests": "read", "webhooks": "write" },
    "webhook": { "mode": "auto", "status": "pending", "events": ["push"], "last_delivery_at": null },
    "used_by": [], "status": "active"
  },
  "meta": { "webhook": null, "warnings": [] }
}
```
Manual mode: `meta.webhook = {url: "https://app.trydimmi.com/webhooks/github/repositories/ghr_...", secret: "shown once", content_type: "json", events: ["push", "pull_request"]}` + warning `webhook_manual_setup`. Errors: 422 `github_repo_not_found`, 422 `github_permission_missing` (`missing: ["pull_requests"]`), 409 `github_repository_exists` (same repo id in workspace; returns existing id), 422 `github_connection_required` (private repo or docs sync without token).

`POST /api/v1/github-repositories/{ghr}/checks` -> 202 repository (re-probe permissions; auto mode `POST /repos/{o}/{r}/hooks/{id}/pings`); 1 per 10 s (429).

`POST /api/v1/docs-sync-sources` body section 5. 201 `docs_sync_source` + `meta.estimate {cost_per_pr_typical_micros, cost_per_pr_max_micros, model_answer, model_classifier}`. Errors: 422 `entitlement_limit_reached` (`docs_sync.max_sources`), 422 `github_branch_not_found`, 422 `github_permission_missing` (`pull_requests` needed), 409 `docs_sync_source_exists`. No usable AI key: source still created, `meta.warnings` `ai_unavailable` (runs skip `ai_unavailable` until key added).

`GET /api/v1/docs-sync-runs?filter[help_center]=hc_&filter[status]=skipped&sort=-created_at` item:
```json
{
  "id": "dsr_01jc...", "object": "docs_sync_run", "source_id": "dss_01jc...", "help_center_id": "hc_01jc...",
  "trigger": "pull_request", "status": "completed", "stage": "finish", "skip_reason": null,
  "pull_request": { "number": 128, "title": "Raise team seat limit", "url": "https://github.com/acme/app/pull/128", "author": "ada", "labels": ["billing"], "merged_at": "2026-11-02T09:58:11Z" },
  "range": { "base_sha": "9f2c1e7", "head_sha": "a71b03c" },
  "screen": { "user_facing": true, "confidence": 0.82, "summary": "Team plans now allow up to 10 seats...", "changes": [{"kind": "changed_limit", "description": "Team plan seat limit 5 -> 10"}] },
  "candidates": [
    { "article_id": "art_01jc...", "title": "Plans and seats", "score": 0.9, "matched_by": ["source_ref", "search"], "outcome": "drafted", "suggestion_id": "sug_01jc..." },
    { "article_id": "art_01jd...", "title": "Invite teammates", "score": 0.52, "matched_by": ["search"], "outcome": "not_affected" }
  ],
  "files": { "total": 14, "used": 6, "truncated": false },
  "batch_id": "sugb_01jc...", "suggestion_count": 1,
  "stats": { "input_tokens": 18420, "output_tokens": 1210, "cost_micros": 41000, "redacted_lines": 0 },
  "review_url": "https://app.trydimmi.com/help-centers/hc_.../review?batch=sugb_...",
  "created_at": "2026-11-02T09:58:13Z", "finished_at": "2026-11-02T09:59:40Z"
}
```

`POST /api/v1/docs-sync-runs` (`articles:write`) variants -> 202 `docs_sync_run` (status `queued`):
```json
{ "trigger": "manual", "source_id": "dss_01jc...", "pr_number": 128 }
{ "trigger": "manual", "source_id": "dss_01jc...", "base_sha": "9f2c1e7", "head_sha": "a71b03c" }
{ "trigger": "redraft", "replay_of": "dsr_01jc...", "article_id": "art_01jc..." }
{ "trigger": "freshness", "article_id": "art_01jc..." }
```
Errors: 429 `entitlement_limit_reached` (`docs_sync.runs_daily`, `Retry-After`), 409 `docs_sync_run_in_progress` (same source + PR/range queued or running), 422 `github_pr_not_merged` (manual PR open, closed unmerged, or merged into other branch: docs describe shipped code only), 422 `docs_sync_source_required` (freshness without source for article repo), 409 `ai_unavailable`, 423 `spend_cap_reached`.

`PATCH /api/v1/docs-sync-runs/{dsr} {status: "canceled"}` -> 200; finished run 409 `docs_sync_run_finished`.

`GET /api/v1/articles/{art}/source-refs` -> `[{id: "asr_...", repository: {id, full_name}, path, origin, created_at}]`; `POST {github_repository_id, path}` (path normalized, no leading `/`, max 512; directory = trailing `/`; 409 duplicate; 422 `source_ref_limit_reached` over 50 member refs); `DELETE /article-source-refs/{asr}`.

`PATCH /api/v1/articles/{art} {verified: true}` -> 200 article with `freshness`. `verified: false` 422 (`invalid_transition`; scans set needs_review, humans only verify).

`POST /api/v1/help-centers/{hc}/freshness-checks` -> 202 `{object: "freshness_check", help_center_id, status: "queued"}` (not stored; progress via `article.freshness_changed` + help center `freshness_summary` on `GET /help-centers/{hc}`: `{needs_review, fresh, last_scanned_at}`).

## 11. MCP and CLI

09 toolset `docs_sync` (enable `?toolsets=docs_sync`; under 2,000 tokens). Tools in `app/mcp/tools/docs_sync_tools.go`.

| Tool | Ability | Risk | REST | Does |
|---|---|---|---|---|
| dimmi_list_docs_sync_runs | articles:read | read | `GET /docs-sync-runs` | recent runs: PR, status, skip reason, screen summary, suggestion count, review URL; filters status, repo, pr |
| dimmi_list_stale_articles | articles:read | read | `GET /articles?filter[freshness]=needs_review` | articles with reasons, quoted claims, paths |
| dimmi_request_docs_update | articles:write | write | `POST /docs-sync-runs` | manual run for PR number / sha range / freshness fix; spends workspace BYOK; returns run + review URL. Never accepts |

Pending doc updates: existing `dimmi_list_article_suggestions` with `source_kind: "commit"` (08 help_center toolset). Accept, publish, reapply-and-accept, verify: no MCP tool (review-queue.md rule 2; `verified` PATCH from MCP surface 403 `human_review_required`).

Prompt `dimmi_update_docs_for_change` (args `range` default `HEAD~1..HEAD` or `pr`): zero-server-LLM path for user's coding agent: read diff locally (`git diff`), `dimmi_get_help_center_outline`, `dimmi_search_knowledge` for affected articles, `dimmi_get_article` (base_version_id), write minimal edits, `dimmi_propose_article_edit` with `source_kind agent`, `source_ref {repo, commit_sha, pr_number, paths}`, `summary`; tell user review URL; never claim published. Same review flow; refs learned on accept (7.2).

User CLI (09 implements; command group follows 08 `dimmi docs *`, 09 may map to its `articles` group):

| Command | REST |
|---|---|
| `dimmi docs sync runs [--status S] [--pr N] [--help-center hc_]` | `GET /docs-sync-runs` |
| `dimmi docs sync run --pr N [--source dss_]` / `--range A..B` | `POST /docs-sync-runs {trigger: manual}`; polls run, prints review URL |
| `dimmi docs suggestions --source commit [--accept-all sugb_]` | existing (`GET /article-suggestions?filter[source_kind]=commit`, batch PATCH; TTY confirm, `dimmi-cli` passes `RequireHumanReview`) |
| `dimmi docs stale [--help-center hc_]` | `GET /articles?filter[freshness]=needs_review` |
| `dimmi docs verify <slug\|art_>` | `PATCH /articles/{art} {verified: true}` (TTY confirm; non-TTY needs `--yes`) |
| `dimmi github repos add owner/name [--connection ghc_]` | `POST /github-repositories` (prints manual webhook steps when needed) |

Server command: `dimmi docs-sync:replay {run}` (operator), `dimmi github:check` (connections + repos now).

## 12. Events

| Event | Realtime | Webhook | Payload |
|---|---|---|---|
| docs_sync_run.completed | yes (`workspace:{ws}`) | yes | run resource (status `completed` or `skipped`, skip_reason, suggestion_count, batch_id, PR ref; no code) |
| docs_sync_run.failed | yes | yes | run resource + error_code |
| article.freshness_changed | yes | yes | `{article_id, from, to, reasons: [{code, severity}]}` |
| github_repository.webhook_status_changed | yes | no | `{repository_id, from, to, error_code}` |
| github_connection.errored | yes | no | `{connection_id, error_code}` (`github_token_invalid`, `github_token_expired`, `github_token_expiring`) |

Also existing `article_suggestion.created` per suggestion, `article_suggestion.reviewed` on review. 06 (requested): system alert for `github_connection.errored`, repo `failing`; destination trigger `docs_suggestions_ready` (Slack post "PR #128: 2 doc updates to review").

## 13. Security and privacy

- Tokens: `encryption.Sealed`, last 4 only, never logged or returned, never in job args; `ClientFor` decrypts per call. Host allowlist `api.github.com`, `codeload.github.com`. Classic PAT warning.
- Webhook: per-repo 32-byte secret, HMAC SHA-256 constant-time, repo id match, delivery dedupe, 25 MB cap, 202 fast; route never authenticates as member (actor `system`).
- Code: patch text exists only in `RunDocsSync` / `VerifyArticleClaims` memory; sent only via `ai.RunTask` with workspace key (07 has no Dimmi fallback key: BYOK boundary); never persisted (run stores file names, stats, LLM summary), never in logs (log fields allowlist), never in traces (span attributes: counts only), never in River args (envelope has PR title/body only: metadata, not code), never in 07 `ai_run_steps.request` (`RunTaskInput.PersistInput false`, Required from 07 item 3). Leak test: fixture patch with marker `DIMMI_CODE_MARKER_7f3a` -> grep Postgres dump (all tables incl. `river_job`, `ai_run_steps`), Valkey keys/values, captured logs, trace exporter: marker absent.
- Secrets in diff: `secretscan.RedactLines` before LLM. Secrets in output: `possible_secret` warning (accept needs ack).
- Prompt injection: PR title, body, diff = untrusted (`ai.WrapUntrusted`); output only proposals; `linkguard` on replace text; anchored edits cannot touch text model did not quote; nothing publishes without human. Malicious PR body `docs: art_x` only selects candidate (no privilege).
- Private repo evidence links only in dashboard/API resources to workspace members; stripped from proposed Markdown when repo private.
- Tenancy: all new tables RLS; `ClientFor` filters by workspace; webhook route resolves repo by public id then workspace from row.
- Audit log: connection create/rotate/delete, repository add/delete/secret rotate, docs sync source create/update/delete, article verify.

## 14. Observability

Metrics: `dimmi_github_webhook_deliveries_total{event,result}` (accepted, duplicate, bad_signature, repo_mismatch, ignored), `dimmi_github_api_requests_total{endpoint,status}`, `dimmi_github_rate_limit_snoozes_total`, `dimmi_docs_sync_runs_total{trigger,status,skip_reason}`, `dimmi_docs_sync_stage_seconds{stage}`, `dimmi_docs_sync_merge_to_queue_seconds` (merged_at to first suggestion), `dimmi_docs_sync_candidates_total{matched_by,outcome}`, `dimmi_docs_sync_edits_total{result}` (applied, unmatched, overlapping), `dimmi_docs_sync_tokens_total{purpose}`, `dimmi_docs_sync_suggestion_reviews_total{result}` (accept rate = main quality signal), `dimmi_article_freshness_total{status}`, `dimmi_freshness_reasons_total{code}`, `dimmi_freshness_ai_checks_total{verdict}`.
Logs: run summary (run id, PR number, status, skip_reason, candidates count, tokens, cost, durations). Never code, PR body, token, article bodies.
Traces: spans per stage (gate, fetch, digest, screen, map, draft_n, assemble).
`dimmi doctor`: repos in poll mode, failing webhooks, connections expiring within 14 days, `app_url` publicly reachable (for webhooks).

## 15. Testing

Policy `00-conventions/testing.md`: real App, Postgres, Valkey, River drained; fakes only at process boundary: `httpclienttest.New(tc.App.HTTP)` GitHub fixtures (PR files with patches, compare, commits/pulls, hooks, 403 with `X-Accepted-GitHub-Permissions`, rate-limit headers), `llm.Fake()` scripted JSON per purpose, `tc.Clock`. Edge ids in names (`t.Run("E221 ...")`).
- Golden fixtures: 12 PR fixtures (UI rename, limit change, pricing, removed feature, internal refactor, dependency bump, test-only, revert, explicit `docs:` ref, huge PR 400 files, secret in diff, direct push burst) through real pipeline with scripted LLM; assert run rows, outcomes, suggestions, hunks, notes.
- Pure (table tests allowed): anchored edit applier, glob filter, file scorer, digest budgeter, `secretscan`, sensitivity rule.
- Arch test: packages under `app/actions/docssync` never import accept/publish actions.
- Leak test (section 13).
- Prompt eval set `tests/evals/docs_sync/` (30 labeled PRs x articles; precision of screen, mapping recall, edit acceptance by rubric): runs only on demand with real model and explicit approval (cost estimate printed first); never in CI.

## 16. Required from other specs

Exact edits for owners (08 does not edit these folders).

### 00-foundation

1. `data-model.md` ownership row 08 append: `github_connections, github_repositories, docs_sync_sources, docs_sync_runs, article_source_refs, article_freshness`.
2. `data-model.md` prefix registry rows: `ghc_` github_connections, `ghr_` github_repositories, `dss_` docs_sync_sources, `dsr_` docs_sync_runs, `asr_` article_source_refs (`article_freshness` keyed by article, no prefix).
3. `data-model.md` migration ranges: 08 uses `20261101000821-000827` inside its block (listed for grep; no rule change).
4. `events-and-realtime.md` "Events added by feature specs" row 08 append: `docs_sync_run.completed`, `docs_sync_run.failed`, `article.freshness_changed` (realtime + webhook), `github_repository.webhook_status_changed`, `github_connection.errored` (realtime only).
5. `rest.md` exceptions row `/webhooks/{provider}/...`: replace `/webhooks/github/changelog/{cls}` (13) with `/webhooks/github/repositories/{ghr}` (08, shared by 08 knowledge, 08 docs sync, 13 changelog).
6. `contracts.md` entitlement catalog rows: `docs_sync.max_sources` (limit, self-host config 0 -> -1, cloud 10, 08); `docs_sync.runs_daily` (limit daily, -1, cloud 200, 08).
7. `contracts.md` Knowledge: `KnowledgeSearchOptions.Surface` adds value `docs_sync` (no logging, default mode auto).
8. `api.md` endpoint ownership row: `github connections, github repositories, docs sync sources/runs, article source refs, freshness checks | 08-help-center`. Abilities unchanged.
9. `security.md` "Privacy": add "Customer source code (08 docs sync): processed in memory only; sent only to workspace BYOK key; never stored, logged, traced or put in job args; secret-like lines redacted before LLM."
No framework catalog change: `app/services/github` adapter row unchanged (08 owner); `app/support/secretscan` is app code (pure, no deps).

### 00-accounts

1. Permission catalog `app/auth/permissions.go`: `github:read` (group `settings:read`; owner, admin, member), `github:manage` (group `settings:write`; owner, admin), `docs_sync:manage` (group `settings:write`; owner, admin). `roles.go` rows.

### 07-dimmi-agent

1. `ai_runs.kind` CHECK add `docs_sync`; `enums.AiRunKind` value `docs_sync`; runs list filter shows it. Purposes stored in `config_snapshot.purpose`: `screen_change`, `draft_edit`, `draft_article`, `verify_article`.
2. `RunTask` used as is (classifier role for screen/verify, answer for drafts; `PreflightSpend`; `WrapUntrusted`; `linkguard.Filter` with caller allowlist).
3. `ai_run_steps.request` holds rendered messages (would hold customer code). `RunTaskInput.PersistInput *bool` (done in 07; nil = true): false -> `request` stores params, prompt hashes and message sizes only, never message text. Docs sync passes false for every purpose. `response` (model output: doc text, evidence paths, no code) stored as usual with 30-day retention.
4. Prompt guidance: none (docs sync prompts owned by 08 under `resources/prompts/docs_sync/`).

### 09-agent-native

1. `mcp.md` feature toolsets: add `docs_sync` (08): `dimmi_list_docs_sync_runs`, `dimmi_list_stale_articles`, `dimmi_request_docs_update`, prompt `dimmi_update_docs_for_change`; budget test.
2. `mcp.md` articles rule line: add "no MCP tool marks articles verified (`PATCH /articles/{art} {verified}` refused on MCP surface)".
3. `cli.md`: commands in section 11 (map `dimmi docs *` to 09's `articles` group if 09 keeps it); command to REST map rows.
4. OAuth scope docs: no new scopes (`articles:*`, `settings:*` cover).

### 10-webhooks

1. `webhook_catalog.go` entries: `docs_sync_run.completed`, `docs_sync_run.failed` (object `docs_sync_run`), `article.freshness_changed` (object `article_freshness`), fixtures from 08 resources.

### 12-cloud-billing

1. `metering-rules.md` "Not activity" table: row `08 docs sync | GitHub webhooks, docs sync runs, suggestions, freshness scans, AI drafting (BYOK, 07 spend) | none`.
2. Cloud entitlement driver: keys `docs_sync.max_sources`, `docs_sync.runs_daily` (catalog defaults above; overrides allowed).

### 13-changelog

1. `changelog_sources`: replace `repo`, `repo_id`, `credentials_encrypted`, `knowledge_source_id`, `webhook_secret_encrypted` with `github_repository_id uuid not null references github_repositories on delete restrict` (08 table). Create body `{github_repository_id}` (or `full_name` + `connection_id`, 13 calls 08 `github.AddRepository`).
2. Webhook: drop `/webhooks/github/changelog/{cls}` route + controller; register `github.WebhookHandler` `changelog` (events `release`) at boot; `HandleGithubChangelogEvent` job unchanged behind it (dedupe by delivery id stays, now in 08 dispatcher).
3. GitHub client: `github.ClientFor(ctx, a, ws, githubRepositoryID)` replaces `knowledge.ResolveGitCredentials` (removed from 08 exports).
4. Error codes `github_token_invalid`, `github_repo_not_found`, `github_permission_missing` now raised by 08 GitHub layer (same names).
5. Edge cases citing per-source secret/token: point to 08 E209-E216.

### 06-notifications (requested, non-blocking)

1. `system_alert` sources: `github_connection.errored` (expiring, invalid, expired), repo webhook `failing`.
2. Workspace destination trigger `docs_suggestions_ready` (source `docs_sync_run.completed` with `suggestion_count > 0`; posts PR ref + count + review link to Slack).

### 11-self-host

1. Docs: webhooks need public `app.url`; else polling (15 min) automatically; `dimmi doctor` check `github.app_url_public` (HEAD from outside not possible: check `app.url` host resolves to non-private IP; warn otherwise).
2. Config keys `docs_sync.*`, `github.*` documented in config reference.

## 17. Open questions (recommended default)

1. Fine-grained PAT prefill URL params for all four permissions: verify at build. Default: deep link with what GitHub accepts + written steps.
2. Drafting article in user's other locales: later with translations.
3. Should `not_affected` judgments advance freshness baseline without human? Default yes (model saw change and article; claims check is the same model class). Revisit if false negatives show in eval.
4. `push` trigger default on? Default yes (vibe coders push to main); sources created via CLI with `--triggers pull_request` for PR-only teams.
5. Cloud `docs_sync.runs_daily` 200: protects GitHub API + queue, not money (BYOK). Default 200; raise per customer by override.
