# 14 Feedback board and roadmap: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Tests per `00-conventions/testing.md`: integration only for app logic (real Postgres `pgvector/pgvector:pg18`, real App via `testkit.New(t)`, real dispatcher, real River drained with `tc.Queue.Drain(ctx)`, real RLS, real Valkey from root `compose.yaml` via `REDIS_URL` with per-test key prefix, never faked); no unit tests with mocks, no sqlmock/pgxmock/embedded-postgres. Fakes only at process boundaries: `tc.Mail` (captured MIME), `httpclienttest.New(tc.App.HTTP)`, 07 `llm.Fake()`, 08 fake embeddings driver, `tc.Clock.Freeze`/`Advance`. Never real LLM or network. Every edge case in edge-cases.md has a test, named in plain words (`t.Run("rejects a replayed sso nonce", ...)`). Prereqs: 00-foundation, 00-accounts, 03 core (contacts, conversations, tags, attachments), 08 host resolution + Markdown pipeline, 01 widget group, 02 verifier, 04 mailer path. AI tasks need 07 `ai.RunTask`.

## M1. Data core and configuration

- [ ] Migrations 001401 to 001419 + 001422 (RLS), sqlc queries, enums, prefixes `fbd_ fbct_ fbs_ fbp_ fbv_ fbc_ fbr_ fbm_ fbsb_ fbu_ fbn_ fbsg_ fbi_ pts_ psi_` added to `app/models/public_ids.go` (`framework/ids` registry).
  - AC: migrate up/down clean; id round-trip tests; query lint (workspace_id predicate) passes; every index leads with `workspace_id` (lint), documented scheduler index exception.
- [ ] Core table migrations 001420 (attachments feedback columns + one-target CHECK), 001421 (`widget_settings.feedback_tab_enabled`) and 001423 (`notifications.feedback_post_id` + aggregation index, 06 table).
  - Tests: CHECK rejects two targets; 03/01 orphan purge ignores feedback-linked rows.
- [ ] Permission catalog entries (`feedback:read|write|delete|manage`), abilities `feedback:read|write`, roles mapping, audit action names (00-accounts files, `app/auth/permissions.go`), entitlement catalog keys; over-limit 422 `entitlement_limit_reached` (`key`, `limit`).
  - Tests: member lacks `feedback:delete`/`feedback:manage`; token ability intersect; `/me` permissions list.
- [ ] `EnsureFeedbackDefaults` initializer (settings, 6 statuses, default board) + boot repair for existing workspaces (marker).
  - Tests: idempotent on `dimmi workspace:repair`; existing workspace backfill once; order after 08 initializer.
- [ ] Settings show/update (typed jsonb models, validation, warnings: sso unavailable, no sender, help center disabled, ai unavailable), audit.
  - Tests: each field validation; `feedback:manage` required; warnings matrix.
- [ ] Boards CRUD + order + trash/restore with posts + categories CRUD/order; statuses CRUD/order/delete with replacement (job), category rules.
  - Tests: reserved slugs, last board 409, max 12 statuses, `status_category_required`, replacement moves posts without events per post, cross-tenant 404.
- [ ] Factories + `feedback_seeder` (2 boards, 40 posts, 600 votes, comments, requests linked to seeded conversations, roadmap filled, 3 suggestions).
  - AC: `dimmi db:seed` shows populated board and roadmap.

## M2. Posts, votes, comments, requests, merges

- [ ] `CreatePost`: number allocator, body render (`framework/markdown` with 08 dialect options + `framework/sanitize`), moderation evaluation (approval modes, links, banned words, new contact), author vote + subscription, optional request, attachments link (03 `LinkAttachments` feedback targets), events.
  - Tests: concurrent creates unique numbers; each moderation path; `on_behalf_of` validation; 413 body; XSS corpus renders safe.
- [ ] `PATCH /feedback-posts/{fbp}` field dispatch: `UpdatePost`, `ChangePostStatus` (shipped_at, status_update comment), `ModeratePost` (+ ban), `MovePost`, tags, owner, `trashed: false`; `?preview=true` estimate; `If-Unmodified-Since`.
  - Tests: table test every field and permission; same-status no-op; transitions 422; preview writes nothing.
- [ ] Bulk `PATCH /feedback-posts` (ids, max 100, per-id results) and `DELETE` trash; purge job with `RetentionGuard`.
  - Tests: partial failure reporting; forbidden field per id; purge cascade incl. files.
- [ ] Votes: `CastVote` (ON CONFLICT, trending log-sum-exp SQL, counts, last_activity), retract, on-behalf with 03 upsert, list with company + revenue, bulk delete anonymous, `recompute_feedback_post_counts`.
  - Tests: double vote once; race vote/unvote; trending ordering equals brute-force decayed sum (property test, 1,000 random votes); 200 votes/s bench on one post p95 under 30 ms.
- [ ] Revenue: settings-driven parse, distinct companies, async recompute jobs, listeners on company/contact attribute change.
  - Tests: non-numeric, negative, minor/major units, two contacts same company, attribute change triggers only when key changed.
- [ ] Comments: create (public/internal, one-level threading, mentions table), edit windows, delete placeholder, moderation, lock, counts.
  - Tests: reply-to-reply flattening; internal never in public resource; mention of deactivated member silent.
- [ ] Requests: link (vote + subscription in tx, company snapshot, message check), update/move, unlink (`remove_vote` rule), global index with filters, listener on `conversation.merged`.
  - Tests: 409 `request_exists`; cross-workspace conversation 404; move dedupes votes; conversation merge repoints.
- [ ] Merge/unmerge: locks ordered by id, dedupe with recorded contact ids, moved comments/requests/subscriptions, cycle and limit checks, merge notice comment, 301 info.
  - Tests: merge then unmerge restores exact counts (property test); cycle 422; unmerge after target trashed 409.
- [ ] Search: FTS + trigram + RRF `SearchSimilarPosts`, list filters/sorts with keyset cursors, roadmap query (columns, manual order), stats endpoint.
  - Tests: each filter/sort; cursor stability under inserts; bench 100k posts / 2M votes p95 under 50 ms.
- [ ] Remaining REST controllers, requests, resources (member + public), policies, subscriptions endpoints, contact states (ban/unban/hide/restore content, revoke sessions).
  - Tests: feature test per endpoint (happy, 422, permission, ability, cross-tenant 404, idempotency on POST).

## M3. Portal identity

- [ ] `portal_sessions`, `portal_sign_ins`, SECURITY DEFINER resolvers, `portal_session` middleware (cookie names per host kind, sliding expiry throttled), CSRF via `framework/http/middleware` (`http.CrossOriginProtection`).
  - Tests: path mode cookie path; host/workspace mismatch invalid; cross-origin unsafe request 403 `csrf_mismatch`; same-origin form POST without token passes; SSO form grant exempt.
- [ ] Email sign-in: `StartSignIn` (rate limits, domain allowlist, suppression, constant response, timing pad), 04 `SendContactNotification{portal_sign_in}`, redeem by token or code, attempts lock (Valkey counter, `locked_at` in Postgres), `ResolvePortalEmailContact` (03 verified claim rule).
  - Tests: enumeration (same body/timing within 10 ms p50); scanner GET does not consume; 5 bad codes lock; unverified row moves to claimant; banned contact 403 at redeem.
- [ ] SSO grant: 02 `VerifyIdentityToken{portal}` + `IdentifyContact`, `sso_login_url` redirect flow, fragment reader page, form POST variant, error hints.
  - Tests: every 02 error code surfaces; nonce replay (E89); widget-audience token 401 `token_audience_mismatch`; token from SDK helper fixture (`aud dimmi-portal`, 300 s, nonce) signs in; identity switch revokes old session; no active identity secret 409 `sso_not_configured` (E90).
- [ ] Widget hand-off codes and grant; anonymous grant with PoW/honeypot; repoint anonymous votes on later sign-in.
  - Tests: hand-off single use, 60 s; anonymous off returns `sign_in_required`; anonymous votes move on sign-in with dedupe.
- [ ] Session revocation listeners (contact merged/erased/blocked, secret revoked, ban, access change) + `dimmi feedback:portal-sessions:revoke`.

## M4. Portal pages and host integration

- [ ] Route group inside 08 help center group (`resolve_portal`), access modes, 404 rules (private, pending, unknown), canonical slug 301, merged 301, path mode.
  - Tests: every route per host kind; signed_in access redirects; private board marker never served.
- [ ] templ pages: home, board, post, new (no-JS two-step similar check), roadmap, sign-in, landing, sso, account; `portal.css`; optional `fb.js` under 6 KB gz.
  - AC: Lighthouse CI 100/100/100/100 mobile on `/feedback`, post page, `/roadmap` (seeded).
- [ ] `/api/feedback/*` JSON + form PRG controllers (posts, votes, comments, subscription, roadmap, sign-ins, sessions, notifications, preferences, attachments).
  - Tests: same validation for form and JSON; 303 with relative `return_to` only; trust matrix (portal.md "Actions by trust") table test.
- [ ] Caching: anonymous page cache (in-process L1) keyed by both cache versions, `Vary: Cookie`, ETag; `BustPortalCache` listener + Valkey pubsub `feedback.cache` (two App instances in test share one Valkey).
  - Tests: signed-in never cached; status change busts; votes do not bust; TTFB bench cached p95 under 30 ms.
- [ ] SEO: head tags, JSON-LD, 08 sitemap/robots/nav hooks (08 edits), CSP `form-action`, `frame-ancestors` setting. No `.md` twins or llms.txt sections (later: portal-md-twins).
  - Tests: head snapshots; sitemap excludes private/pending.
- [ ] Public file route for post/comment images; image-only upload validation.
  - Tests: image of private or pending post 404; SVG rejected.
- [ ] Leak test: crawl every portal, widget, sitemap, email render for markers from private boards, internal comments, tags, requests, emails.

## M5. Widget tab

- [ ] `/widget/v1/feedback/*` controllers in 01 group, `feedback_actor` middleware (header session > identified > anonymous), CORS header, app-only cache headers.
  - Tests: actor matrix; tab disabled 404; header session for other widget invalid meta.
- [ ] Widget sign-in (email code, token returned once), hand-off endpoint, notifications feed + bulk read, unread count hook for 01 session payload.
  - Tests: identified vote zero extra calls; feed unread count capped 99.

## M6. Close the loop

- [ ] Subscriptions rules (reasons, unsubscribed sticky) wired into vote, post, comment, request, import, merge.
  - Tests: re-vote never resubscribes; explicit follow clears.
- [ ] `feedback_post_updates`: schedule from status change/merge/comments/replies, supersede and revert, cancel and send-sooner endpoints, release periodic job (leader, SKIP LOCKED).
  - Tests: flapping matrix (loop.md) with frozen clock; hidden post skip; leader crash simulation.
- [ ] Fan-out job (chunks 500, ON CONFLICT), email decision rules (opt-out, unverified, cooldown coalescing, daily cap in contact timezone), email job: `RenderFeedbackEmail` (templ views, not `app/mail` Mailables) then 04 `SendContactNotification` (wrapper over `SendContactMail`, customer stream) with idempotency key; templates (update, shipped, merged, comment, digest, sign-in).
  - Tests: 10,000 subscribers queued under 60 s (bench); one row per contact per update; coalesced digest content; 04 skipped reasons mapped; `tc.Mail` capture shows customer stream tags + `outbound_emails.kind = feedback_update`, `metadata.purpose` = update kind; complaint suppression scope `feedback` skips next update but sign-in mail still sent; arch test `tests/arch/email_streams_test.go` green (14 imports no `a.Mail`); `SyncFeedbackSuppression` on `email_suppression.created{outbound_kind: feedback_update}` marks row `suppressed` (E147).
- [ ] Signed unsubscribe (`/feedback-unsubscriptions` GET page, POST RFC 8058, undo) on `framework/routing` signed routes (`a.URL.SignedRoute`, middleware `signed`), preferences endpoint, `List-Unsubscribe` via `SendContactMail.Unsubscribe`.
  - Tests: tampered signature 403 page; one-click POST without CSRF; never subscribes; undo restores.
- [ ] 06 triggers (`feedback_post_submitted`, `feedback_post_pending`, `feedback_comment_created`, `feedback_mentioned`) as `notifications.trigger` registry entries, notification classes + source event wiring.
  - Tests: recipients and defaults; pending email skipped when moderated within 5 min.
- [ ] Changelog seam: `LinkChangelogEntry` action + listener for 13 `changelog_entry.published` (`feedback_post_ids`, `notify_voters`), `changelog_entry.deleted` listener, shipped dedupe rules. FK to `changelog_entries` inline in migrations 001406/001414.
  - Tests (real 13 `PublishChangelogEntry`, no fake events): status then changelog inside window = one email with link; after 24 h no second email; `notify_voters: false` links without email; entry deleted nulls link, no email; post id of other workspace ignored.
- [ ] Newsletter dedupe read action `feedback.ShippedRecipientContactIDs(ctx, entryID)` for 13 mailing snapshot.
  - Tests: voter emailed for entry's post excluded from 13 mailing recipients; opted-out voter not excluded (13 subscription is separate consent); other workspace contacts never returned.
- [ ] Member/contact lifecycle: `RepointFeedbackContact` on `contact.merged`, `EraseFeedbackContactData` eraser, export hook, member deactivation release.
  - Tests: erase leaves counts, removes PII (grep test on all 14 tables); merge dedupe; eraser idempotent re-run.

## M7. AI (BYOK)

- [ ] Prompts `resources/prompts/feedback/2026-11-01/*`, 07 `ai.RunTask` integration (final API, `Kind feedback`, `Purpose capture|duplicate`, customer text via `ai.WrapUntrusted`), JSON schema parsing with repair retry.
  - Tests: fake LLM fixtures valid/invalid JSON; spend cap skip; no-key degrade.
- [ ] Duplicate detection job + suggestions `merge_posts`; embeddings job via 08 `EmbedTexts` (optional vector arm).
  - Tests: no candidates means no LLM call; threshold; pending pair dedupe; model switch disables vector arm.
- [ ] Capture: triggers (close delay, 07 intent hook, manual `POST /feedback-captures`), pass A verbatim check, pass B match, suggestion creation, daily cap.
  - Tests: reopened conversation skipped; non-verbatim quote dropped; existing request skip; cap reached metric.
- [ ] Suggestion review endpoints (accept with overrides, dismiss, bulk), expiry job.
  - Tests: each kind accept path; merged target follow; dismissed never re-suggested 90 days.

## M8. Agent-native, webhooks, import

- [ ] MCP toolset `feedback` (8 tools), confirm gate mapping, identifier resolution (number, status name), untrusted envelopes, token budget under 2,000.
  - Tests: CI token budget; gate preview text with recipient count; cross-workspace number 404.
- [ ] CLI: `dimmi feedback import` helper and status shortcut (09 client), generated commands verified.
- [ ] Webhook catalog entries + fixtures; internal comments behind `include_notes`.
  - Tests: CI catalog completeness; internal comment no delivery row without opt-in.
- [ ] CSV import: upload, delimiter sniffing, presets (canny, featurebase), mapping validation (unmapped statuses), dry run report, chunked run, idempotent keys, cancel/resume, imported vote counts.
  - Tests: fixture CSVs from both tools' export headers; 20,001 rows 413; rerun skips existing.
- [ ] Import write helpers for 16 (`UpsertImportedPost/Vote/Comment/Request`, `DeleteImportedPosts`, shared `app/support/csvsniff`): no events, no notifications, no AI jobs, counts recomputed once per page, status/post caps.
  - Tests: E148, E149, E150, E151, E152; `tc.AssertNoSideEffects` after importing 100 posts with votes and comments.

## M9. Hardening and docs

- [ ] Rate limits on Valkey `ratelimit.Limiter` (portal per IP, per contact actions, sign-in, anonymous votes), anonymous vote anomaly metric, disposable domain check (cloud).
- [ ] Valkey failure behavior: fail closed on sign-in start/redeem and anonymous writes (503 + `Retry-After`), fail open on member and signed-in contact writes with bypass metric.
  - Tests (E145, E146; real Valkey, per-test key prefix): test's keyspace flushed mid sign-in keeps `locked_at` lock and allows at most 4 extra tries; flush leaves votes/counts/notifications intact; Valkey unreachable (in-test TCP proxy cut): anonymous vote 503, identified vote 201 + metric, portal page still served.
- [ ] Metrics, logs, `dimmi doctor` checks, server commands (`feedback:recount`, `feedback:reembed`, `feedback:rerender`).
- [ ] 12 replay fixture `feedback_activity_not_billable`: full feedback flow produces zero ledger lines.
- [ ] Docs: portal setup, SSO via SDK helpers (`createPortalLink`, `portalLink`, `/api/dimmi/sso` as `sso_login_url`) + raw snippet per stack (02 snippets), widget tab (`show('feedback')`, `show('roadmap')`), import guide, privacy defaults, MCP examples.
- [ ] Dogfood: Dimmi workspace portal on `feedback.trydimmi.com`, dashboard SSO, widget tab in dashboard.

## Done when (demo script, under 10 minutes)

1. Fresh workspace: `GET /api/v1/feedback/settings` shows defaults; enable portal (`PATCH {portal_enabled: true}`) on help center `help.acme.dev`.
2. Open `https://help.acme.dev/feedback`: empty board renders, Lighthouse 100, works with JS disabled.
3. Host app (Ada signed in) renders `createPortalLink({returnTo: '/feedback'})` (`@dimmi/server`) -> `/api/dimmi/sso?return_to=%2Ffeedback`; host route (`createPortalSsoHandler`) mints portal token (`aud dimmi-portal`, 300 s, nonce) and 302s to `/feedback/sso#token=...`: signed in as Ada; submit "Export conversations to CSV" (similar posts shown while typing); vote recorded.
4. Second visitor signs in with email code; votes; receives nothing yet.
5. In widget on demo app (identified user Bob): `Dimmi.show('feedback', {post: 12})` opens Feedback tab on post, one tap votes (no login); "Open in portal" hand-off signs Bob into portal in one click.
6. In inbox, conversation #1042 says "can we get a CSV of all chats every month?": member selects sentence, "Link to feedback" -> post #12. Request shows quote, Globex company, revenue $4,999; vote count 4; inbox side panel lists #12.
7. Enable AI capture with fake/real BYOK key; close conversation #1050 with similar ask; suggestion appears; accept: request linked.
8. Create duplicate "CSV export of chats"; duplicate suggestion appears; accept merge: votes move, duplicate URL 301s to #12; unmerge and merge again.
9. Roadmap: move #12 to Planned; `/roadmap` shows it. Then to Shipped with note; dashboard shows "Notifying 4 people in 10:00. Undo"; send now.
10. Mailpit shows one email per subscriber with note and one-click unsubscribe; widget feed shows "Shipped" for Bob; webhook endpoint receives `feedback_post.status_changed` and `feedback_post_update.sent`.
11. Publish 13 changelog entry linking #12 (`notify_subscribers` on): post shows changelog link, no second email to voters; 13 newsletter skips voters already emailed.
12. Click "Stop all product update emails" in email: confirm page, POST, opt-out set; next update skips email, feed still shows.
13. Claude Code via MCP: `dimmi_search_feedback "csv"`, `dimmi_link_feedback_request` on conversation #1060, `dimmi_update_feedback_status` asks confirmation showing recipient count.
14. Import Canny CSV with preset: dry run reports 0 errors, run imports 120 posts with scores.
15. Erase contact Ada: her vote stays counted anonymously, her post body removed, no PII left (grep test). Billing ledger unchanged.
