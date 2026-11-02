# 13 Changelog: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Tests per `00-conventions/testing.md`: business logic integration only (real App, real Postgres `pgvector/pgvector:pg18`, real Valkey, real River drained via `tc.Queue.Drain`, real event dispatcher); no unit tests with mocks, no sqlmock/embedded Postgres. Fakes only at process boundaries: `tc.Mail` (captured MIME after 04 build), `httpclienttest.New(tc.App.HTTP)` (GitHub fixtures), `llm.Fake()`, `tc.Clock`. Never real LLM, GitHub, or mail provider. Tests named in plain words; every edge case covered. Starts after 08 M2 (public site) and 04 outbound core; runs parallel to 08 M3+ in build order M4.

## M1. Data and entries core

- [ ] Migrations `001301` to `001314` (001314 = 08-owned `knowledge_documents.changelog_entry_id`), sqlc queries, enums, prefixes `clg_ cle_ cll_ csub_ clm_ cls_ cgn_` in `app/models/public_ids.go` (`framework/ids` registry), RLS policies.
  - AC: migrate up/down clean; id round trips; query lint (workspace_id predicate) passes; prefixes added to registry doc (via foundation owner).
- [ ] Factories (changelog, entry `Published()/Scheduled(at)/WithLabels`, subscriber `Active()/Pending()`, mailing, source) + seeder (8 entries over 3 months, labels, 20 subscribers, reactions).
  - AC: `dimmi db:seed` renders populated changelog.
- [ ] `EnsureChangelog` (initializer after 08 + lazy + backfill migration), default labels.
  - Tests: initializer idempotent on `workspace:repair`; concurrent lazy create one row; backfill covers existing help centers.
- [ ] Changelog singleton REST (`GET/PATCH /help-centers/{hc}/changelog`), request, resource (URLs, counts), policy.
  - Tests: member PATCH 403; token without `settings:write` 403 `missing_ability`; other workspace 404.
- [ ] Labels: create/update/delete/reorder REST, limits, slug rules.
  - Tests: 21st label 422; reorder stable; delete removes pivots, keeps entries.
- [ ] Entry create/update with lock_version, slug generator + reserved list, `previous_slugs`, render via `framework/markdown` + 08 `helpmarkdown` options + `framework/sanitize` (body_html, word count, warnings), content_hash, pivots (labels, authors, articles) with limits and same-help-center checks.
  - Tests: If-Match 409/400; reserved slug 422; 128 KB 413; XSS corpus reused from 08; article from other help center 422.
- [ ] State machine: publish, schedule, reschedule, unschedule, unpublish, republish (`published_at: "now"`), backdate, trash, restore; events; `If-Match`.
  - Tests: table test every transition incl. illegal 422 `invalid_transition` with from/to; events dispatched once; restore returns draft.
- [ ] Scheduled publish: River job with `ScheduledAt`, sweeper periodic, lateness rules (under/over 24 h), workspace suspended snooze.
  - Tests: `tc.Clock.Freeze` at scheduled time; job twice no-op; reschedule old job no-op; server-down catch-up; lag metric.
- [ ] Entries index/show/bulk REST with filters, `q` trigram, sort, include; trash filter.
  - Tests: filters each; bulk mixed results; cursor stable under inserts.

## M2. Public site and feeds

- [ ] `resolve_changelog` middleware on 08 group: base path `/changelog`, enabled checks, empty-changelog 404, canonical = help center primary host.
  - Tests: each host kind (subdomain, path mode, custom help domain).
- [ ] templ pages: index (first 3 full, rest summary), page N, label pages, entry (related articles, prev/next, reactions forms, subscribe box, "Talk to us"), 404; SEO head (title, description, canonical, OG, Twitter, JSON-LD `BlogPosting`/`Blog`, alternates).
  - AC: Lighthouse CI 100/100/100/100 mobile on seeded index and entry page. Tests: head snapshots; drafts/scheduled 404.
- [ ] Cache integration: `BumpHelpCenterCache` after commit on every public change (publish, edit, unpublish, labels, settings); page cache keys include base path.
  - Tests: publish visible under 2 s on second App instance (same Postgres + Valkey, real Valkey pubsub bust); 304 path.
- [ ] Feeds: `support/feeds` item mapping on framework `view/feed` (gorilla/feeds v1.2.0; catalog row with framework owner) for RSS 2.0 (+ label feeds), `urn:uuid` ids, absolute URLs, ETag/Last-Modified.
  - Tests: golden files; RSS validator in feature test over real endpoint; ids stable after slug and domain change.
- [ ] `.md` twins, `/changelog.md`, `llms-full.txt`, `Accept: text/markdown`.
  - Tests: content shape; 5 MB cap truncation note.
- [ ] 08 provider hooks: nav link, home "Latest updates", sitemap URLs (child sitemap when split), llms.txt `## Changelog` section (needs 08 registry task).
  - Tests: section present only with public entries; sitemap excludes drafts.
- [ ] OG card renderer (`support/ogimage` layout on `framework/image`, embedded Inter subset), `RenderChangelogOgImage` job, asset storage, `/changelog/og/{cle}.png` redirect, missing-glyph fallback.
  - Tests: golden PNG hashes per layout; CJK title fallback; contrast color pick.
- [ ] Reactions: tables, counts upsert, page JSON API + no-JS form, `reactor_hash` derivation (`hashing.HMAC`), rate limits on Valkey (`a.Limiter`), bot UA reject, disabled toggle.
  - Tests: idempotent same emoji; toggle off; no-JS ip-day identity; burst 200/s single entry without deadlock (bench); counts never negative.
- [ ] Stats: view/widget/markdown/feed counters in 08 buffered flusher; stats endpoint.
  - Tests: bot UA ignored; flush aggregation; date range max 90 days 422.

## M3. Widget

- [ ] Widget routes (`/widget/v1/changelog`, entry, read-state, reactions, subscriptions) in 01 group; scope by widget `help_center_id` + `changelog_tab_enabled`; anonymous cache headers.
  - Tests: not linked 404; other help center entry 404; anonymous `max-age=60`; p95 under 30 ms bench.
- [ ] Unread logic: `changelog_reads` monotonic upsert, window 30 days, cap 9; session resume fragment and config fragment (`latest_announced_at`, spotlight) for 01; widget config_version bump listener.
  - Tests: new contact sees max 9; stale/future timestamps; spotlight newest within 14 days; unpublish removes spotlight.
- [ ] `contact.merged` listener `MergeChangelogReads` (reads, subscriber contact link, reactions contact id).
  - Tests: GREATEST kept; source rows gone.

## M4. Subscribers and email

- [ ] Subscriber model + REST (index, show, store with consent, update, destroy, CSV export) + imports endpoint (1,000 per request, result counts) + policies (admin for import/export/delete).
  - Tests: consent required; unsubscribed resubscribe 409; import skips; export admin-only + audit entry; cross-tenant 404.
- [ ] Public subscribe (form + JSON), honeypot, timing token, per-IP limit (Valkey) and per-address limit (durable `confirmation_count`), no-enumeration responses; widget subscribe (verified identity single click).
  - Tests: identical responses for new/existing/active; 4th confirmation same day not sent; bot timing silently dropped.
- [ ] Tokens (`hashing.HMAC`, HKDF key from `framework/encryption`, purposes, nonce rotation) + confirmation pages (GET page, POST confirm) + unsubscribe pages (GET page, POST one-click RFC 8058) + mailto unsubscribe hook (register prefix `cunsub` via 04 `email.RegisterMailtoUnsubscribe` at boot; `MailtoToken cunsub-<token>`). Confirmation mail = templ view + 04 `SendContactMail{Kind: changelog, Purpose: confirmation}` (no Mailable).
  - Tests: GET never mutates (scanner test); one-click body handling; expired confirm; nonce rotation invalidates old links; route log redaction.
- [ ] `support/emailhtml` (inline styles, table layout, alerts, code, images, dark-mode safe) + templ mail views (entry, confirmation; rendered by 13 actions, never `app/mail` Mailables) + text twins + size budget fallback to summary.
  - Tests: golden HTML; under 90 KB budget check; Litmus-free structural checks (no external CSS, no scripts, absolute URLs).
- [ ] Email preview + test endpoints (member only, 10/hour).
  - Tests: token caller 403 `member_required`; rate limit 429.
- [ ] Mailing creation on publish (undo window), cancel on unpublish/trash/notify off, manual mailing and follow-up to skipped, guards (paused, sending domain, monthly cap, no recipients, disabled, test_mode).
  - Tests: each guard (monthly cap + default-sender cap via entitlement overrides); unique per entry; undo within window sends nothing (`tc.Mail` empty).
- [ ] Start/batch/finish jobs: snapshot recipients (+ 14 dedupe `feedback.ShippedRecipientContactIDs` -> `skipped feedback_notified`), per-recipient `Unsubscribe{URL, Post, MailtoToken}` + `ListID` + `Tags {entry, mailing}`, 04 `SendChangelogEmail` (`Kind: changelog`, `Purpose: entry`; customer stream; no `ReplyTo`; `outbound_emails.kind changelog`) with idempotency key, Valkey rate limiter, result/error mapping (`email.ErrSuppressed`, `email.ErrSendingPaused`, skip `limited`/`no_sender`/`disabled`, `mail.ErrPermanent`, `ErrTemporary`, `*mail.RateLimitedError`), counters, coalesced progress events, watchdog.
  - Tests: 10,000 recipients captured by `tc.Mail` under 60 s; crash mid-batch resumes without duplicates (same idempotency key returns stored result); 04 pause of kind `changelog` mid-send -> partially_sent; unsubscribe during send skipped; E158/E159 dedupe; arch test passes (no Mailable, no `a.Mail` in 13).
- [ ] Suppression sync listener (04 `email_suppression.created {outbound_email_id, outbound_kind}`): `outbound_kind = changelog` + hard bounce scope `all` by email; subscriber bounced/complained, recipient + mailing counters, subscriber_count.
  - Tests: hard bounce, complaint scope changelog, unrelated suppression ignored.

## M5. GitHub and generations

- [ ] Releases/compare/pulls files in 08 `app/services/github` adapter (08 owns adapter, `github.ClientFor`; 13 adds files only) + source REST (create by `github_repository_id` or `full_name` + `connection_id` via 08 `github.AddRepository`, update, delete, unique per changelog + repo) + `tests` endpoint (08 connection, repo, permission probe, webhook status). Needs 08 GitHub layer task (08 M11) first.
  - Tests: `httpclienttest.New(tc.App.HTTP)` GitHub fixtures; repo without connection 422 `github_connection_required` (E135); 08 repo delete 409 `github_repository_in_use` (E167); missing PR permission reported (E130); no token or secret column or field anywhere in 13 responses/logs.
- [ ] `github.WebhookHandler` `changelog` (Events `release`, `Wants` active source on repo) registered at boot via `github.RegisterWebhookHandler` + `HandleGithubChangelogEvent`: actions published/released/edited, prerelease rules, paused/deleted source no-op, `dedupe_key` per source + release; `PollGithubReleases` for repos without active webhook. Release envelope (13 shape) agreed with 08 dispatcher.
  - Tests: signed delivery through real 08 shared route `/webhooks/github/repositories/{ghr}`; bad signature never reaches handler (E123); redelivery no duplicate (E124); prerelease promoted (E125); edited after human edit ignored (E126); same repo shared with 08 docs sync source: `pull_request` never reaches 13 (E165); two changelogs one repo two drafts (E166); last source deleted drops `release` from hook (E168); paused (E169); poll then webhook same release one entry (E170).
- [ ] Generation resource REST + job: range (compare + PR filter), release, text inputs; deterministic assembler (label mapping, conventional commits); cancel.
  - Tests: golden Markdown from fixture PR set; empty range fails; over 250 commits truncated flag; concurrent same input 409.
- [ ] AI mode via 07 `ai.RunTask` (kind `changelog`, purpose `generate_entry`, fake LLM): prompt files, JSON schema validation, repair retry, linkguard, fallback draft, cost stats, spend cap.
  - Tests: invalid JSON twice -> failed + fallback entry; injected PR body cannot change status (always draft); unknown-domain link stripped; `ai_runs.kind changelog` recorded.

## M6. Knowledge, MCP, CLI, webhooks, notifications

- [ ] Knowledge integration: call 08 `IndexChangelogEntry`/`RemoveChangelogEntry` in publish/edit/unpublish/delete/restore tx; migration `001314` column on `knowledge_documents`.
  - Tests: agent (07 fake) cites entry URL; draft never retrievable; help center search shows changelog kind; Ask AI public-only still includes changelog.
- [ ] Public MCP: `dimmi_get_doc` changelog URL support (with 08); no changelog tool (later: public-mcp-changelog).
  - Tests: `dimmi_get_doc` resolves `changelog/{slug}`; `dimmi_search_docs` items carry `kind: changelog`.
- [ ] Authenticated MCP toolset `changelog` (5 tools) + prompt `dimmi_write_changelog_entry`; risk declarations; confirm gate cases (elicitation, param fallback, direct mode with notify still confirms).
  - Tests: publish without confirmation returns preview; direct + notify confirms; direct without notify executes; toolset under 2,000 tokens.
- [ ] CLI (with 09): `changelog list|new|draft|publish|subscribers import|export`, front matter parser, cover upload, `--dry-run` REST print.
  - Tests: `publish --file` upsert by id then slug; `draft --since` builds text input from fixture git repo; non-TTY without `--yes` exits 2.
- [ ] Webhook catalog entries + fixtures (10); 06 destination trigger `changelog_entry_published` wiring and `system_alert` sources.
  - Tests: catalog CI check passes; Slack destination receives entry post (06 fakes).
- [ ] GDPR eraser `EraseChangelogContactData` registered; subscriber hard delete path.
  - Tests: erase by contact id and by email identities; counts decremented; idempotent re-run.

- [ ] Feedback seam (14): entry editor `feedback_post_ids` -> `feedback.LinkChangelogEntry`; `changelog_entry.published` payload `{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids, notify_voters}`; `changelog_entry.deleted {purged}`.
  - Tests (real 14 tables, no fake events): E158, E159, E160; 14 voter mail and 13 newsletter never both reach same contact.
- [ ] Announcements seam (15-announcements-surveys): `changelog.FindPublishedEntry`; widget entry route 404 `changelog_entry_not_found` for unpublished/other help center; SDK `show('changelog', {entry})` contract fixture shared with 01.
  - Tests: E161, E162.
- [ ] Import helpers for 16: `changelog.ImportEntry`, `EnsureLabel`, `DeleteImportedEntries` (backdated, `source_kind import`, `announce false`, no events/mailing/badge, knowledge indexed in tx, delta update only when untouched).
  - Tests: E171, E172, E173, E174, E175; `tc.AssertNoSideEffects` (no mail, no events, no webhooks) after importing 50 entries.

## M7. Hardening

- [ ] Valkey resilience: flush mid-mailing and mid-reaction burst; Valkey unreachable during publish (commit succeeds, bust logged, limiter per-replica fallback).
  - Tests: E163, E164 against real Valkey (outage: client through in-test TCP proxy, proxy cut; flush: test's own keyspace).

- [ ] Leak test extension: drafts, scheduled, trashed, other-help-center entries with marker strings absent from HTML, `.md`, feeds, sitemap, llms.txt, widget, public MCP, knowledge public surfaces, OG endpoint.
- [ ] Purge job (trash 30 days, pending 7 days, recipients 400 days, generations 90 days, stats 400 days) + subscriber_count reconciliation.
- [ ] Metrics, logs (email redaction), audit entries, `dimmi doctor` checks (overdue scheduled, stuck mailings, sending domain).
- [ ] Benchmarks in CI: index TTFB cached/cold (2,000 entries), widget list, reaction burst, 10k mailing; fail on 10% regression.
- [ ] Cross-tenant sweep: every endpoint with other-workspace ids returns 404.
- [ ] Dogfood: Dimmi's own changelog content (launch entries) seeded into cloud workspace; served at Dimmi help center host `/changelog`.

## Done when

- `go test ./...` green against real Postgres + real Valkey only; RSS validates; Lighthouse 100s; budgets met.
- Demo script (fresh `docker compose up`, self-host, instance mail provider `smtp` relay to Mailpit container (customer stream = default mailer), fake LLM driver; real LLM only with explicit user approval):
  1. Sign up; default help center + changelog exist; `/hc/acme/changelog` returns 404 (no entries yet).
  2. `POST /changelog-entries` with cover and label New, publish with `notify_subscribers`; page live under 2 s at `/hc/acme/changelog/...`; help center header shows "Changelog"; home shows "Latest updates".
  3. Subscribe `bob@example.org` on page without JS; Mailpit shows confirmation; open link (GET) -> still pending; press button -> active.
  4. Publish second entry; within 5 minutes unpublish -> Mailpit receives nothing. Publish third entry; after delay Mailpit shows email with List-Unsubscribe headers; one-click POST unsubscribes.
  5. RSS validates; `curl -H 'Accept: text/markdown'` returns twin; `/llms.txt` lists changelog section.
  6. Widget (test session, `changelog_tab_enabled`): unread_count 2, open entry, read-state 0, react tada, count visible on public page.
  7. Add 08 GitHub connection + repository against fake GitHub, then changelog source on that repo; send signed `release published` delivery to 08 shared route `/webhooks/github/repositories/{ghr}` -> draft "v1.4.0" with `needs_review`; generation for range `v1.3.0...v1.4.0` with fake LLM -> AI draft; publish via REST.
  8. Claude Code with Dimmi MCP (`?toolsets=changelog`): prompt `dimmi_write_changelog_entry` -> draft created; ask agent to publish -> elicitation confirm -> live.
  9. Dimmi Agent playground (fake LLM echoing retrieval): "what's new in v1.4?" cites changelog entry URL.
  10. Erase contact linked to subscriber: subscriber, reads, reactions gone; counts adjusted.
  11. Webhook endpoint (10) subscribed to `changelog_entry.published` receives signed payload; Slack destination (06 fake) gets post.
  12. Feedback post (14) with voter `bob@example.org` linked to new entry, publish with email: Mailpit shows one voter mail (14) to Bob, newsletter skips Bob (`feedback_notified`).
  13. 15-announcements-surveys banner CTA `open_changelog_entry` in test widget opens entry via `show('changelog', {entry})`.
