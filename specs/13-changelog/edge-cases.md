# 13 Changelog edge cases

Style: caveman. Format: "Case. Behavior." Every behavior decided.

## A. Changelog and hosts

1. Workspace created. Initializer `EnsureChangelog` creates changelog for default help center in `CreateWorkspace` tx after 08 initializer, seeds labels New, Improved, Fixed. Re-run (`dimmi workspace:repair`) no-op.
2. Second help center created. No changelog row until first `GET/PATCH /help-centers/{hc}/changelog` or entry create (lazy `EnsureChangelog`, `INSERT ... ON CONFLICT (help_center_id) DO NOTHING`). Two concurrent lazy creates: one row.
3. Upgrade from version without 13. Migration `001313` backfills changelog for every non-deleted help center.
4. Changelog with zero public published entries. `/changelog`, feeds, `.md` 404; nav link and home block hidden; llms.txt section omitted; widget tab hidden (01 hides when `entries` empty).
5. `enabled = false`. Public routes 404, widget 404 `changelog_not_linked`, scheduled publishes still run (content state) but mailings created as `blocked` `changelog_disabled`; API editing works.
6. Help center `status = disabled`. Changelog public 404 like help center; widget changelog 404; knowledge documents removed by 08 with help center docs.
7. Help center soft-deleted. Changelog public 404, scheduled entry jobs skip (help center deleted), pending mailings canceled by 08 delete hook calling `changelog.OnHelpCenterDeleted`; restore within 30 days: entries indexed again, scheduled entries past due publish on restore? No: past-due scheduled entries move to `draft` with `meta.warnings missing_schedule` on restore (never surprise-publish stale news).
8. Help center purged. FK cascade removes changelog and all 13 rows; generated OG assets removed with 08 assets.
9. Help center subdomain change. Changelog URLs follow (08 30-day 301 hold covers `/changelog/*`). Feed item ids `urn:uuid` unchanged, readers do not duplicate.
10. Path mode only (self-host without base domain). URLs `{app_url}/hc/{sub}/changelog`; emails and feeds use path-mode absolute URLs.
11. `app.url` changes (11 `system.app_url_changed`). 08 cache rebuild covers; feeds absolute URLs regenerate; already-sent emails keep old links (redirect is operator responsibility).
12. Help center `indexing_enabled = false`. Changelog pages `noindex`, sitemap excludes, robots Disallow (08). Feeds still served (subscribers use them).
13. Reserved slug collides with label slug route (`labels`). Entry slugs reserved list prevents; label slug lives under `/labels/` so no collision with entries.

## B. Entries and states

14. Two members edit same entry. Second PATCH with old `If-Match` 409 `stale_resource` with editor name. Token without `If-Match`: last write wins, actor recorded.
15. Publish draft with empty body and empty title. 409 `nothing_to_publish`. Empty body with title allowed (title-only "Fixed: login loop" entries are common), warning `empty_body`.
16. Publish when `changelog.enabled = false`. 409 `changelog_disabled` (avoid silent invisible publish).
17. Publish with `published_at` in future. 422 `published_at_in_future`; use `scheduled`.
18. Backdated publish (`published_at` past). Allowed (import history). `announce` defaults false for backdated older than `unread_window_days` unless sent explicitly; `notify_subscribers` default false; no widget badge for old dates (window rule).
19. Schedule in past or now. 422 `scheduled_for_in_past` (tolerance 30 s: within 30 s treated as publish now).
20. Schedule over 365 days ahead. 422 `scheduled_for_too_far`.
21. Scheduled time reached while server down. Sweeper on boot (1-minute periodic) publishes overdue entries with `published_at = scheduled_for` if lateness under 24 h; over 24 h late: publishes with `published_at = now`, mailing still created, metric `schedule_lag`, 06 `system_alert` to owners.
22. Scheduled entry edited between scheduling and publish. Publish uses latest content; job re-reads.
23. Reschedule. Old River job stays but no-ops (unique key includes `scheduled_for`; worker checks entry `scheduled_for` equals job arg).
24. Scheduled job runs twice (River retry after crash post-commit). Second run sees `published`, no-op.
25. Scheduled entry trashed. Job no-ops; restore returns it as `draft` (schedule cleared).
26. Unpublish published entry. Status `draft`, `published_at` kept for history display in dashboard, public 404 (not 410), knowledge document removed in tx, cache bust, pending mailing canceled, widget badge recomputed (`latest_announced_at` = previous announced entry), event `changelog_entry.unpublished`.
27. Republish previously published entry. `published_at` kept original by default (avoid fake "new"); `published_at: "now"` param resets date (then counts as new for badge and allows mailing if never mailed).
28. Edit published entry content. Live on save (no versions); `edited_after_publish_at` set when `content_hash` changes; mailing already sent not resent; RSS item content refreshes (readers may re-show, acceptable); widget does not re-badge.
29. Edit published entry title. Slug unchanged unless slug sent; OG card regenerated; feed ids stable.
30. Slug change on published entry. Old slug pushed to `previous_slugs` (max 10, oldest dropped); old URL 301. New entry later takes freed old slug: new entry wins (exact slug match first), redirect ignored.
31. Slug equals another entry's previous slug. Allowed; that redirect stops (current slug wins).
32. Delete (trash) published entry. Public 404, knowledge removed, mailing canceled if not started; sending mailing continues? No: sending mailing stops remaining batches (`partially_sent`, reason canceled).
33. Restore trashed entry. Returns to `draft` (never auto-republish).
34. Purge after 30 days. Rows hard-deleted; reactions, stats rows cascade; mailing rows keep `entry_id` null with counts (history).
35. Entry body over 128 KB. 413 `markdown_too_large`; never truncated.
36. Body contains raw HTML or script. Escaped as text (08 dialect, raw HTML off).
37. Body references asset of other help center. Rendered as broken image text + warning `missing_asset`; cover from other help center 422 `asset_not_in_help_center`.
38. Cover asset deleted. 08 refuses delete while referenced (409 `asset_in_use`, 13 references registered).
39. Related article unpublished, archived, deleted, or agent_only. Omitted at render; dashboard shows grey "not public" chip; FK cascade removes pivot on hard delete.
40. Related article from other help center. 422 `article_not_in_help_center`.
41. Author member deactivated. Still shown with name (they wrote it). User deleted: "Former teammate", no avatar. `show_authors = false`: no names anywhere public.
42. Over 5 labels, authors, or articles. 422 validation error on `label_ids` / `author_ids` / `article_ids` (`max: 5`).
43. Label deleted. Pivot rows removed; entries keep other labels; label page and label feeds 404; cache bust.
44. Label renamed. Label slug unchanged unless sent; changed slug: old label URL 404 (no redirects for labels; low value), cache bust.
45. 21st label. 422 `entitlement_limit_reached` (`key: changelog.max_labels`, `limit: 20`).
46. Title with emoji, RTL, CJK. Stored NFC; slug transliterated (08 slug generator, fallback `entry-<6 chars>`); OG card falls back to no-title layout when glyphs missing.
47. Same `version_tag` on two entries. Allowed (hotfix entries); not unique.
48. Bulk change with mixed permissions or states. Per-id results (rest.md rule 8); never silent.
49. Entry in workspace in `test_mode`. Works fully; mailings in test mode send only to workspace members' emails that are subscribers (others `skipped` reason `test_mode`), avoids surprising real users from sandbox.
50. Workspace suspended. Public pages 503 (08 rule), scheduled publishes deferred (job snoozes 1 h), mailings paused (`blocked` after 24 h).
51. Workspace deleted. Mailings canceled, jobs no-op, purge with workspace.

## C. Public site, feeds, SEO

52. Request `/changelog/` trailing slash or uppercase slug. 301 normalized (08 rule).
53. `/changelog/page/1`. 301 to `/changelog`. Page past last: 404.
54. `Accept: text/markdown` on entry path. Markdown twin with `Vary: Accept`.
55. Feed requested 10,000 times/min by aggregators. ETag 304 + 08 page cache; per-IP `throttle:help_center` limits; feeds `max-age=300`.
56. Feed reader with `If-Modified-Since`. `Last-Modified` = newest of published/edited among items; 304 when unchanged.
57. Entry published, aggregator fetched, then unpublished. Item disappears from feed next fetch; readers keep cached copy (out of our control).
58. Entry body uses relative links `/articles/x`. Rewritten absolute in feeds, emails, `.md`.
59. Over 50 entries. Feeds carry newest 50; `.md` index and llms-full list all (llms-full capped 5 MB, then truncated note).
60. Sitemap over 50,000 URLs with changelog. 08 sitemap index splits; changelog URLs in own child sitemap `/sitemaps/changelog-{n}.xml`.
61. OG card generation fails (font, image decode). Entry uses changelog/help center OG; job retries 3 times, then logs; never blocks publish.
62. Brand color low contrast with white text on OG card. Text color auto black/white by WCAG ratio (08 helper).
63. Bot user agents viewing pages. Not counted in views (08 list); feed readers counted as `feed_fetches`.
64. Search engine indexes label page duplicates. Label pages canonical to themselves, entries canonical to entry page; index paginated pages have `rel=prev/next`.
65. Date display across timezones. Displayed date = `published_at` in workspace timezone (entry published 23:30 UTC shows next day in Tokyo workspace); `<time datetime>` UTC ISO.

## D. Reactions

66. Same visitor reacts twice same emoji. Idempotent (PK), count unchanged.
67. Visitor toggles off. `DELETE` with same visitor key removes row, count decremented (never below 0, CHECK); unknown key: 200 no-op.
68. No-JS reaction form. Identity `ip+ua+day` hash; cannot un-react; second click same day no-op; next day counts again (accepted approximation).
69. Visitor clears localStorage and reacts again. Counts twice (no login; accepted, documented as approximate).
70. Reaction flood from one IP. 30/min per IP (Valkey limiter) then 429; bot UA rejected 403 silently (200 without effect on form path).
71. Widget identified contact reacts on two devices. Same contact identity: one reaction.
72. Anonymous widget visitor then identifies. Earlier visitor-key reactions not merged into contact identity (counts already include them; no double count because identity differs only if they react again; accepted).
73. Reactions disabled after reactions exist. Counts hidden everywhere, API 404 `reactions_disabled`; rows kept; re-enable shows them again.
74. Reaction on draft or other help center entry. 404.
75. Contact erased. Reactions with `contact_id` deleted, counts decremented in same tx (eraser).

## E. Widget

76. Widget without `help_center_id` or `changelog_tab_enabled = false`. 404 `changelog_not_linked`; config `features.changelog.enabled = false`.
77. Anonymous visitor first open. No server cursor; client localStorage empty: unread = announced entries within 30 days, capped 9; after open, client stores now.
78. Identified contact new to workspace with many old entries. Unread counts only last 30 days (window) and max 9; never "47 unread".
79. Read-state PATCH with older timestamp (stale tab). Stored value kept (monotonic `GREATEST`).
80. Read-state PATCH with future timestamp. Clamped to server now.
81. Contact merged (03). `changelog_reads` of source contact: keep max `last_seen_at` on target (merge hook through eraser-like registry? no: 13 listens `contact.merged` queued and upserts GREATEST, deletes source row).
82. Spotlight on two entries in 14 days. Newest spotlight entry only; older ignored.
83. Spotlight entry unpublished. Config fragment recomputed (widget `config_version` bump), card disappears.
84. Config CDN cached 60 s after publish. Badge appears within 60 s worst case; `latest_announced_at` in ETag so revalidation catches change.
85. Widget subscribe by identified contact with unverified email. Pending + confirmation email (verified only when `contacts.email_verified`).
86. Widget subscribe by lead with no email and no `email` param. 422 `email_required`.

## F. Subscribers and consent

87. Subscribe with existing active email. Same "check your email" response, no email sent (no enumeration, no spam vector).
88. Subscribe pending address again. Confirmation resent if under 3 per 24 h; else silently not sent (same response).
89. Attacker subscribes thousands of victims (list bombing). Per-IP 10/hour, honeypot, timing token, 3 per address per day, pending expiry 7 days, minimal confirmation content. Cloud: 06 `system_alert` + ops metric when one changelog gets over 500 pending/hour.
90. Corporate link scanner opens confirmation link. GET renders page only; nothing confirmed.
91. Confirmation link after 7 days. Expired page with subscribe form.
92. Confirmation link used twice. Second shows "already subscribed".
93. Email case or Unicode variants (`Bob@Example.org`, IDN domain). Normalized: domain lowercase + IDNA A-label; local part case preserved but uniqueness on citext (case-insensitive) as practical choice.
94. Plus-addresses (`bob+news@x.com`). Distinct subscriber; no stripping.
95. API resubscribe of unsubscribed person. 409 `subscriber_unsubscribed`; import skips (`skipped_unsubscribed`).
96. Person resubscribes via page after unsubscribing. Pending + double opt-in, `token_nonce` rotated (old unsubscribe links invalid, new ones issued).
97. Import 5,000 rows. 413 `import_too_large` above 1,000 per request; CLI chunks.
98. Import duplicates inside file. First wins, rest `skipped_duplicate_in_file`.
99. Import without consent. 422 `consent_required`.
100. Subscriber email matches contact later identified with verified email. Link `contact_id` lazily on next widget session or subscribe action (no background matching job).
101. Subscriber hard-deleted by admin. Row gone; recipient rows keep counts (`subscriber_id` null); person can subscribe again (fresh consent).
102. Contact erased (GDPR). Eraser deletes subscriber rows by `contact_id` and by any email of contact (primary + identities), deletes reads, reactions; idempotent.
103. Subscriber export by member role. 403 (admin only); audit logged for admin.
104. Subscriber count drift. Maintained in same tx as status change; nightly `PurgeChangelogData` recomputes and corrects (metric when corrected).

## G. Mailings

105. Publish then unpublish within undo window. Mailing canceled, nothing sent.
106. Publish then edit title within undo window. Mailing renders at start, so edit included.
107. `email_delay_minutes = 0`. Start job queued immediately after commit.
108. Mailing start finds zero active subscribers. `sent` with zero recipients, `blocked_reason no_recipients` informational.
109. Over 50 recipients without verified domain (cloud). `blocked` `sending_domain_required`; dashboard banner with 04 domain link; owner verifies domain and creates new mailing (`POST /changelog-mailings`, previous blocked one does not count as mailed).
110. Monthly recipient cap reached. Whole mailing `blocked` `monthly_cap_reached` (mailing status, not HTTP error; no partial sends, fairness); counts reset at UTC month start.
111. 04 pauses kind `changelog` (or whole workspace) mid-mailing (`SendChangelogEmail` returns `email.ErrSendingPaused`). Remaining recipients `skipped` reason `paused`, mailing `partially_sent`; resend later only to skipped (`POST /changelog-mailings {entry_id, only_skipped_of: clm_}` creates follow-up mailing targeting skipped recipients).
112. Worker crash mid-batch. Retry resends only `pending` recipients; recipient row set `sent` in same tx as counter after Mailer success; Mailer success then crash before commit: retry calls `SendChangelogEmail` with same `IdempotencyKey changelog:{clm}:{csub}`; 04 returns stored result (`Status sent`, `OutboundEmailID`) without resending (04 outbound.md 16), 13 marks row `sent`. Duplicate only if crash lands between provider accept and 04 own commit (04 at-least-once window, documented there).
113. Subscriber unsubscribes during sending. Re-check per recipient: `skipped` reason `unsubscribed`.
114. Subscriber suppressed in 04 (hard bounce from support mail earlier). `SendChangelogEmail` returns `email.ErrSuppressed`: recipient `skipped` reason `suppressed`; subscriber marked `bounced` when suppression scope `all` reason hard bounce.
115. Complaint on changelog email. 04 suppression scope `changelog`; subscriber `complained`, counts; support conversations still email that person.
116. Out-of-office auto-reply to changelog email. 04 discards (matches changelog Message-ID + auto-reply classification); never conversation.
117. From address (`email_from_address_id`) deleted. FK set null; default sender address used; dashboard warning.
118. Entry email over 90 KB HTML. Summary version sent, warning `email_over_budget` in preview and publish response.
119. Mailing for entry with images on private/asset URL. Images use public immutable asset URLs (08 assets are public); no attachments embedded.
120. Mailing re-created after cancel. Allowed (`POST /changelog-mailings`), unique index excludes canceled.
121. `POST /changelog-mailings` for draft entry. 409 `entry_not_published`.
122. Self-host with `log` mail driver. Mailings "sent" to log (dev); `dimmi doctor` warns subscribers exist with log driver.

## H. GitHub source and generations

123. Delivery with invalid signature or foreign `repository.id` on 08 shared route `/webhooks/github/repositories/{ghr}`. 08 returns 401 before any handler (08 E99, E213); no 13 job; 13 source untouched; repo `webhook_status` per 08.
124. Same delivery redelivered (GitHub retry or manual redeliver). 08 dispatcher dedupe (Valkey `SET NX EX 24h` + River unique, 08 E215); job unique `(source, delivery id)`; `dedupe_key` per source + release prevents second entry even after both windows pass.
125. Release published as prerelease, ignored, then promoted (`released`). Draft created at promotion.
126. Release edited after draft created, no human edit yet. Draft body refreshed from release; after any member edit: ignored.
127. Release deleted or unpublished on GitHub. Ignored; draft or published entry untouched (never auto-remove public content).
128. Repo renamed or transferred. 08 refreshes `full_name`, `default_branch` from delivery or `GET /repositories/{repo_id}` (08 E216); 13 references repo row by id, nothing to update; old entries keep `source_ref.repo` snapshot.
129. Token expired or revoked. 08 connection `status error github_token_invalid` / `github_token_expired` (daily check, 08 E209; 06 alert from 08 `github_connection.errored`). 13 API call 401 -> source `error` `github_token_invalid`, `changelog_source.errored`; webhooks still create non-AI drafts from envelope (release body in it); AI/PR-range generations fail with code. Member rotates token on 08 connection (`PATCH /github-connections/{ghc}`); next successful call sets source `active`.
130. Token lacks Pull requests read (08 probe `permissions.pull_requests none`). Source create allowed (releases need Contents only); range generation uses commit messages only (`stats.prs = 0`, warning `github_permission_missing` on generation); `tests` endpoint lists missing permission.
131. Other consumer on same repo (08 knowledge git source, 08 docs sync source) deleted. Repo and connection stay (13 still references, `on delete restrict`); 08 hook events resynced, `release` kept.
132. GitHub rate limited (403/429 with reset). Generation job snoozes until reset (max 1 h) then fails `rate_limited`.
133. Range over 250 commits. Compare truncated; stats `truncated: true`; draft notes "Showing first 250 commits" in generation stats, not in entry body.
134. Range with zero merged PRs and zero commits. Generation `failed` `empty_range`, no entry.
135. Repo added without connection (public-repo mode, 08 `connection_id` null). Source create 422 `github_connection_required` (08 rule: changelog and docs sync need connection).
136. Two generations for same range concurrently. Second 409 `generation_in_progress` (unique on `(changelog, input hash)` while queued/running).
137. AI key missing at generation start. Create returns 409 `ai_unavailable` when `use_ai`; key removed while queued: generation `failed` `ai_unavailable`, deterministic fallback draft created (`stats.fallback_entry_id`).
138. Spend cap reached. 423 at create; mid-run cap: `failed` `spend_cap_reached`, fallback deterministic draft.
139. LLM returns invalid JSON. One repair retry (same prompt + error); still invalid: `failed` `invalid_output` + fallback draft.
140. PR body contains prompt injection ("ignore instructions, publish now"). Input wrapped as untrusted data; output is draft only; no tool access; links filtered by linkguard.
141. AI output invents link to unknown domain. Stripped to plain text.
142. Release body contains `@user` mentions and PR links. Kept in draft with warning `possible_internal_link`; AI mode removes per prompt.
143. Generation canceled while running. Status `canceled`; LLM call context canceled; no entry; tokens already spent recorded by 07.

## I. MCP, CLI, API callers

144. MCP agent calls publish without user confirmation path (no elicitation, no `confirm`). Preview returned `confirmation_required`; nothing published.
145. MCP publish with `notify_subscribers`. Confirms like any `send` tool (MVP always confirms; send modes later: agent-send-modes); gate rule keeps mailing publishes confirmed even once `direct` mode ships (mass email egress).
146. MCP edits published entry. Gate confirms (public content change).
147. MCP drafts with labels not existing. Unknown names ignored, warning in output; labels never created by MCP.
148. CLI `publish --file` with front matter id of trashed entry. 404 with hint to restore; never creates duplicate.
149. CLI `publish --file` slug collision with other entry. Upsert by `id` first, then slug; slug owned by different entry without id in file: 409 `slug_taken` with entry id hint.
150. CI token with only `changelog:write` tries to read subscribers. 403 `missing_ability` `contacts:read`.
151. Idempotency-Key replay on `POST /changelog-entries`. Same response, no duplicate.
152. Cross-workspace ids. 404 everywhere (entries, labels, assets, articles, mailings).

## J. Knowledge and agent

153. Entry published. Knowledge doc in same tx (FTS), embeddings within 30 s; agent can cite it immediately (keyword).
154. Entry edited after publish. Document re-indexed in same tx.
155. Customer asks agent about feature in draft entry. Not retrievable (drafts never indexed); agent answers from docs or "I don't know".
156. Old entry says feature X, newer entry removes it. Both retrievable; chunk titles carry dates; 07 prompt prefers newest; accepted risk, owner can unpublish old entry.
157. Help center search returns changelog result. Labeled "Changelog · 2026-10-30"; link to entry.

## K. Cross-feature (14 feedback, 15 announcements-surveys, SDK, Valkey)

158. Entry links feedback posts (`feedback_post_ids`) and `notify_voters = true`. 14 emails voters (`feedback_update`); 13 mailing snapshot inserts subscribers matching `feedback.ShippedRecipientContactIDs` (by `contact_id` or contact email) as `skipped` reason `feedback_notified`; anonymous subscribers without matching contact get newsletter. One email per person per ship.
159. Same entry with `notify_voters = false` or no linked posts. Empty dedupe set; newsletter goes to every active subscriber.
160. Entry trashed while linked to posts. `changelog_entry.deleted {purged: false}`; 14 keeps link (restore possible) and shows entry as unavailable; purge (30 days) deletes row, 14 FK `on delete set null` clears link, `changelog_entry.deleted {purged: true}`.
161. 15 announcement CTA `open_changelog_entry` points at entry later unpublished or trashed. `changelog.FindPublishedEntry` returns not found (15 hides CTA, warning in dashboard); widget `GET /widget/v1/changelog/entries/{cle}` 404 `changelog_entry_not_found`.
162. SDK `show('changelog', {entry: 'cle_...'})` with draft, other help center, or unknown entry. Widget route 404; SDK opens "What's new" list instead, `console.warn` `view_unavailable` (reason `changelog_entry_not_found`; sdk-contract code). Tab disabled (`changelog_tab_enabled = false`): 404 `changelog_not_linked`, SDK warns, nothing opens (15 CTA falls back to public entry URL).
163. Valkey flushed or restarted. Reaction, subscribe, email-test and mailing rate windows reset (rebuildable state; accepted). Reaction dedupe intact (Postgres PK `(entry_id, reactor_hash, emoji)`), confirmations per address intact (`confirmation_count`), mailing progress intact (recipient rows + 04 idempotency keys). No duplicate email, no double count.
164. Valkey unreachable during publish. Publish commits (Postgres only); cache bust pubsub after commit fails: logged, L1 page cache entries expire by 08 max L1 TTL, so public page stale at most that TTL; widget config bust retried by listener. Rate limiters on public routes fall back to per-replica in-process windows with same limits (13 policy); never block reads.

## L. Shared GitHub layer (08) and imports (16)

165. Same repo feeds 08 knowledge git source, 08 docs sync and 13 changelog. One GitHub hook, one delivery per event; 08 dispatcher fans out; `release` reaches only 13 handler; `push`/`pull_request` never reach 13.
166. Two changelogs (two help centers) with sources on same repo. Handler enqueues one job per source; each changelog gets own draft (`dedupe_key` includes source id).
167. 08 repository delete while changelog source references it. 409 `github_repository_in_use` lists `cls_`; delete source first.
168. Last changelog source on repo deleted. 08 `SyncGithubRepositoryWebhook` drops `release` from auto hook events; manual hook still sends `release`: dispatcher finds no handler that `Wants` repo, nothing enqueued.
169. Source paused, release published, source resumed. Handler `Wants` false while paused; no job; missed release not replayed automatically (member runs generation `github_release`).
170. Repo `webhook_status` `none`, `pending` or `failing` (self-host app URL not public, manual hook never installed). `PollGithubReleases` every 15 min for active sources on such repos: `GET /repos/{o}/{r}/releases?per_page=10`, releases published after `max(source created_at, last_event_at)` go through same path; `dedupe_key` stops duplicates when webhook later delivers same release.
171. 16 imports Featurebase changelog entry published 2024. `ImportEntry`: `source_kind import`, `announce false`, `notify_subscribers false`, backdated `published_at`, knowledge indexed; no mailing, no widget badge, no `changelog_entry.*` event, no webhook, no 06 post; outside unread window (16 E47).
172. Imported source entry scheduled for future or draft at source. Imported as `draft`; never scheduled, never mailed (16 E111).
173. Delta import after member edited imported entry. `ImportEntry` skips (`last_edited_by_type` not `system`); 16 reports `kept`.
174. Import rollback. `DeleteImportedEntries` hard deletes untouched imported entries (knowledge documents removed same tx; 14 FK `on delete set null`); edited entries kept by 16 rule; no events.
175. Imported label count exceeds `changelog.max_labels` (20). 16 mapping conflict `label_cap_reached` (blocks full run) at preview; member maps extras to existing labels or `skip`.
