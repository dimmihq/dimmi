# 16 Importers: edge cases

Style: caveman. Format: "Case. Behavior." Every behavior decided. Tests named in plain words; every case covered.

## A. Credentials and connect

1. Token valid but lacks conversation read (Intercom app permission missing). Connect probes each chosen scope with one cheap call; 422 `source_scope_insufficient` `{missing: ["conversations"]}`; nothing stored.
2. Intercom token for EU workspace sent with region `us`. Source returns 401; 422 `source_credentials_invalid` `{field: "region"}` with message "token not valid in us region; try eu" (Connect retries other regions once only to build hint, never stores).
3. Zendesk subdomain typed as full URL `https://acme.zendesk.com/agent`. Normalized to `acme`; invalid pattern 422 `{field: "subdomain"}`; never connects to non-`zendesk.com` host.
4. Source down during create. 502 `source_unavailable` `{retry_after}`; no import row created (idempotency key replay returns same 502 within 24 h only if stored; 5xx not stored per framework rule, client retries).
5. Token revoked at source mid-run. Next request 401: run `paused` `credentials_invalid`, `import.paused`, system email to starter; resume needs `PATCH /imports/{imp} {credentials}` (validated) then `PATCH run {status: running}` (409 `credentials_required` before).
6. Help Scout access token expires (2 days) mid-run. Driver refreshes via client credentials once on 401, retries request; second 401 = case 5.
7. Credentials PATCHed to token of different source account. Connect returns different account id: 422 `source_account_mismatch` (new account = new import; prevents mixing `connection_key`).
8. Credentials purge date reached while delta sync enabled. `PurgeImportCredentials` skips imports with `sync_enabled` and `sync_until` in future; extends `credentials_purge_after` to `sync_until + 1 day`; archive or `sync_until` passing shreds.

## B. Preview and mapping

9. Preview on 500k-contact Intercom workspace. Counts via search `total_count`; contacts stage of preview streams only ids needed for `will_match_existing` sampling (first 10k emails/external ids, extrapolated; `basis: "sample"` field); finishes under 60 s.
10. Preview run while full run active. 409 `import_run_active`.
11. Options or scopes changed after preview. Import back to `draft`, `last_preview_run_id` kept but `preview_stale: true` in resource; full run 409 `preview_required`.
12. Teammate email matches two Dimmi users (impossible: email unique) or member deactivated. Deactivated member suggested `map` (history keeps real person, link shows deactivated); conflict `member_inactive` informational, not blocking.
13. Teammate email matches no member. Suggested `placeholder` named "{name} (former teammate)"; member may map to anyone (e.g. renamed email).
14. Custom attribute key exists in Dimmi with other type (text vs number). Mapping conflict `attribute_key_taken_other_type` blocks full run until renamed (`target_key`) or skipped.
15. Source has 140 custom attributes, Dimmi allows 100 keys per contact. Discovery suggests `create` for 100 most used (by usage count), `skip` for rest; preview warning `attribute_limit`.
16. Feedback statuses at source exceed 14 cap (12). Unmapped extras suggested `map` to closest category status; `create` beyond cap gives conflict `status_cap_reached` (blocking).
17. Mapping edited during running full run. Applies only to records written after commit (`meta.applies_to: future_records`); already-written rows never re-mapped (idempotent skip on re-run). Dashboard copy: fix mappings before running.
18. Re-discovery (second preview) after source added teammates/tags. New mapping rows added with suggestions; edited rows (`suggested = false`) untouched; rows for entities gone at source kept with `external_meta.missing: true`.

## C. Run lifecycle, resume, concurrency

19. Worker crashes after page writes committed, before enqueue of next page. Impossible by design: next-page job inserted in same tx (`InsertTx`); crash before commit loses whole page tx, River retries page job (same cursor).
20. Worker crashes during source fetch (before tx). Job retried, refetch same cursor; nothing written twice.
21. Deploy restarts all nodes mid-run. River jobs resume on new nodes; stale-heartbeat requeue (15 min) covers lost leases; run continues from checkpoint.
22. Two page jobs of same stage run concurrently (retry overlap). Advisory xact lock `(workspace, connection_key, stage)`; second waits, then sees cursor advanced past its page (stage cursor compare) and exits without writes.
23. Cancel while page in flight. Page commits (cancel is checked at job start and before enqueue next); cancel takes effect at next page boundary (under 1 page delay); run `cancelled`; committed data stays (rollback separate).
24. Pause then resume after 3 days with expired source cursor (Intercom/Help Scout cursors expire). Driver detects invalid cursor (400) and restarts stage listing from `last committed record created_at` (stage `cursor.resume_from`), dedupe skips repeats.
25. Second full run while first completed. Allowed; acts as reconcile: unchanged checksum skip, changed records updated (contacts attributes, article working copy if untouched since import), new records created; totals report `updated`, `skipped`.
26. Two imports in same workspace for different sources (Intercom + Canny). Allowed, parallel up to `imports.concurrent_runs`; contacts match each other by email/external id (second import sees first's contacts as existing: disposition `matched`).
27. Same Intercom app imported into two Dimmi workspaces. Separate imports (tenant), shared rate budget key (account-level) so source limit respected.
28. Live inbox traffic during import. Import queues lower priority, page tx under 200 ms, no locks on hot rows except contact rows being matched (row lock held under 200 ms); live reply never waits more than one page tx.
29. Stage fails permanently (source returns invalid JSON for every page after 10 retries). Stage `failed`, run continues other independent stages (articles independent of conversations), run ends `failed` with `last_error_code source_invalid_payload`; resumable by new `full` run (skips done work).
30. Run-level dependency: contacts stage failed, conversations need contacts. Conversations stage creates missing contact on the fly from conversation author data (email/name/external id) with disposition `created`; never blocks on contacts stage.
31. Run stuck `paused` `rate_limited` forever because source plan limit tiny (Zendesk Essential 10/min). `resume_at` scheduler resumes; preview ETA showed hours; run never fails for slowness.
32. `imports.enabled=false` set by operator while runs active. `imports` queues stop; requeue job marks running runs `paused` (`pause_reason source_unavailable`, `last_error_code imports_disabled`); runs resume when operator re-enables (boot hook sets `resume_at = now`).
33. Import deleted (DELETE) while page job queued. Job finds no run (cascade), exits.
34. Workspace suspended mid-run. Page job checks `suspended_at`; run `paused` (`last_error_code workspace_suspended`); resumes automatically on unsuspend event `workspace.unsuspended` (listener sets `resume_at = now`).

## D. Idempotency and duplicates

35. Same conversation appears in two search pages (source list mutated while paging). `import_records` PK + message `dedupe_key` make second write a no-op (counted `skipped`, not error).
36. Source returns same part twice in one conversation. Second insert `ON CONFLICT DO NOTHING`; counted `duplicate_in_source`.
37. Message part without stable id (Help Scout chat lines). Dedupe key `imp:<connection_key>:<thread id>:<line index>`; stable because line order immutable at source.
38. Contact exists in Dimmi already (widget user with same `external_id`). Disposition `matched`; Dimmi fields win, empty Dimmi fields filled from source (name, phone, company, first_seen_at earlier wins), custom attributes merged (Dimmi value kept on conflict); `contact.updated` not dispatched (lifecycle only).
39. Contact exists with same email but unverified, two Dimmi contacts share that email. Match only when exactly one non-merged contact has it; else new contact created and both listed in 03 duplicates finder (no auto merge).
40. Source contact merged into another at source between full and delta. Delta sees source id gone or `merged_into`: Dimmi contacts not merged automatically; record `external_meta` notes merge; member merges via 03 if wanted.
41. Imported article edited in Dimmi, then re-run. Article untouched check (`content_hash` differs from import-time hash stored in `import_records.checksum` of Dimmi side): skip update, counted `kept`; delta never overwrites human edits.

## E. Side effects, billing, AI, notifications

42. Imported customer message (open conversation, status preserve). No `message.created` event: no AI run, no member notification, no webhook, no billing gate, no unread badge (03 unread rule), no SLA/waiting timer (`waiting_since` null).
43. Imported teammate reply. Never sent to customer (writer inserts `delivery_status = 'not_applicable'`, no `send_channel_message` job).
44. Imported conversation from email source. No outbound mail, no suppression list writes, no auto-reply detection run (04 untouched).
45. Imported CSAT rating parts (Intercom `conversation_rating`). Stored as event row `metadata.import.rating {score, remark}`; never 15 survey response (no survey exists), never triggers survey.
46. Imported feedback post status `complete`. 14 helper writes status without `feedback_post.status_changed`; no voter emails, no post update; `shipped_at` from source when known.
47. Imported changelog entry published 2024. `announce false`, no mailing, no widget badge, `published_at` backdated; not in "What's new" unread window.
48. Imported articles published (`publish: preserve`). Publish via 08 path minus events (`article.published` not dispatched): indexes knowledge, embeds with workspace BYOK key (spend recorded, cap enforced; at cap: embeddings deferred, FTS works, 08 degrade rule).
49. Webhooks endpoint subscribed to `*`. Receives only `import.completed` from import (plus any live events). Defense: 10 fan-out skips imported subjects if a stray event appears.
50. Billing: 1,000 imported conversations with teammate replies. Zero ledger lines (`metadata.imported` excluded from `IsActivity`); replay fixture `import_never_billed`.
51. Imported conversation, then live customer message via widget/email, then teammate reply. Live conversation flow normal: charge once (12 fixture 25).
52. Delta-imported inbound (customer wrote in old tool during switchover), teammate replies in Dimmi (rehome to email). Reply is first workspace message after only imported inbound: no qualifying inbound, no charge (D7 rule). Accepted trade-off: switchover window free; documented in import guide.
53. Rehomed reply to imported conversation, customer answers by email (new thread at Dimmi). 04 threading: reply `In-Reply-To` matches Dimmi outbound Message-ID, lands in same conversation; billed normally (D1 at next workspace reply).
54. Teammate replies to imported conversation whose contact has no email (Intercom anonymous lead). 422 `import_channel_cannot_send` `{reason: no_contact_email}`; note still allowed.

## F. Contacts and companies

55. Intercom workspace with 300k anonymous leads. `skip_anonymous_leads` default true: leads without email, phone, external id and conversations skipped (counted); leads with conversations always imported (created from conversation author).
56. Contact email invalid (`bob@`). Identity not created; raw value in custom attribute `import_invalid_email`; contact still imported if other identifier exists, else skipped `contact_unresolvable`.
57. Contact with 3 emails (Help Scout). Primary -> `contacts.email`, all -> `contact_identities` `email` unverified; identity already owned by other contact: skipped for that email (unique `(workspace_id, type, value)`), warning.
58. `email_trust: verified_for_users`. Only `kind user` contacts with external id get `verified = true` email identity; audit `import.email_trust_attested` with actor; leads never verified.
59. Verified widget login later for email held unverified by imported lead. 03 rule 239 applies: identity row moves to verified claimant, history stays with imported lead; duplicates finder surfaces pair; member merges.
60. JWT `external_id` later equals imported contact external id. Same contact (unique external_id): widget user sees imported history (intended continuity; same as host app identity).
61. Company domain missing, two companies same name. Matched by `external_id` only; name collision allowed (companies not unique by name).
62. Contact belongs to 4 companies (Intercom). First (or Intercom primary) -> `company_id`; others listed in custom attribute `import_companies` (names, comma, 1,024 cap); later `contact_companies` additive.
63. Custom attribute value is object/array. Arrays of scalars joined `, `; objects JSON-encoded string (truncated 1,024); counted `attribute_dropped` only when truncated.
64. Imported custom attributes and agent actions trust (07 `contacts.trusted_attribute_keys`). Default untrusted: import never adds keys to `trusted_attribute_keys`; 07 identity-bound actions (Stripe lookup by `stripe_customer_id`) not eligible on imported values until admin marks key trusted or JWT/token API asserts it.
65. Admin marks attribute mapping `trusted: true` (e.g. `stripe_customer_id`). Writer adds key to `trusted_attribute_keys` only for `kind user` contacts where import wrote the value (created, or empty field filled on matched contact); leads never (lead-set at source = untrusted, 07 edge 181 rule); matched contact whose Dimmi value won keeps existing trust state. Audit `import.attribute_trusted` (key, actor) once per mapping change.
66. `trusted` toggled on after full run finished. Applies to future writes; next `full` or `delta` run adds key to trust list for imported user contacts whose current value still equals imported value (checksum compare), never for values edited since. Toggled off: future writes stop adding; keys already trusted stay (removal = 03 contact API), response `meta.applies_to: future_records`.
67. Rollback with trusted keys. Created contacts deleted (trust gone with row); matched contacts keep filled values and trust entries (same rule as fills not reverted, E124); rollback totals report `trusted_keys_kept` count.
68. Blocked contact in Dimmi matched by import. Stays blocked; imported conversations for blocked contact imported (history) with `spam` unchanged.

## G. Conversations and messages

69. Conversation over 500 parts (Intercom). 500 newest imported, first row event "Older history not available from Intercom API (N parts)", `metadata.import.truncated_before`, warning error `parts_truncated`.
70. Message body over 64 KB. Split into consecutive messages 1 ms apart (`metadata.import.split`); attachments on first chunk.
71. Body empty after HTML to Markdown (image-only part). Body `""` allowed when attachments exist; else part skipped (`unsupported_part_type`).
72. Conversation with 80 participants (CC heavy email). First 50 (03 cap) as participants, rest in `metadata.import.extra_participants` count only.
73. Conversation with 70 tags. First 50 (03 cap) by source order; rest dropped with warning.
74. Conversation status snoozed with `snoozed_until` in past (preserve mode). Imported `open`.
75. Status preserve and assigned to placeholder teammate. Imported unassigned (placeholder cannot own); `metadata.import.assignee_name` kept.
76. Assignee team mapped `skip`. Unassigned team; inbox from inbox mapping.
77. Source conversation number (Zendesk ticket #4512). New Dimmi `number` allocated (block per page); `source.imported.external_number` 4512; search `imported:4512` and records lookup find it.
78. Message author contact differs from conversation contact (second customer replies). Contact resolved (created if needed) and added as participant `participant`.
79. Part authored by deleted source user (no data). Author `unknown` with name "Deleted user"; for customer side: conversation contact used as author.
80. Bot part (Intercom Operator/Fin, Zendesk AI). `author_type agent`, author_id null, `metadata.import.author.kind bot`; never counted as Dimmi AI reply.
81. Internal note mentions `@Sarah` (source mention markup). Converted to plain text `@Sarah`; no `message_mentions` rows (no notifications).
82. Source event parts (assignment, close) huge volume. Event rows imported (count capped at 200 per conversation, rest dropped silently with counter) to keep timeline readable; event rows never change counters (03 rule).
83. Conversation deleted at source between list and detail fetch. 404 -> record error `source_not_found`, skipped; delta never resurrects.

## H. Attachments and images

84. Attachment over 25 MB. Not downloaded; message gets line `[report.pdf, 40 MB, too large to import]` (05 style); error `attachment_too_large`.
85. Attachment URL expired (signed CDN URL). Driver refetches parent record for fresh URL once, retries; then placeholder line + `attachment_fetch_failed`.
86. Executable or HTML attachment. Blocked by 03 upload rules (`attachment_blocked_type`), placeholder line.
87. Same file attached to 300 messages (logo in signature). Stored once per workspace by sha256 (`import_records` `attachment_blob` keyed by source file id, plus disk dedupe by sha256 within import); each message gets own `attachments` row pointing to same path.
88. Attachment download redirects to private IP (malicious source data). SSRF guard blocks hop; `attachment_fetch_failed`.
89. Monthly attachment entitlement reached mid-run. Text continues; attachments stage paused (`entitlement_limit_reached`, `resume_at` period reset); placeholders not written yet (attachments resume later and attach in place).
90. Zendesk private attachments (auth required). Basic auth sent only when URL host is `{subdomain}.zendesk.com` or `*.zdusercontent.com` documented host list; never forwarded on cross-host redirect.

## I. Help center articles and redirects

91. Source has 3 locales, target help center single locale. `articles.locale` imported to target; other locales mapped (`locale` mapping) to other help centers or skipped (warning `locales_skipped`); cap `help_center.max_per_workspace` applies.
92. Article in 3 nesting levels (Intercom collection > section > subsection). Level 1 = collection, level 2 = sub-collection named "Section / Subsection"; warning `collections_flattened`.
93. Slug conflict with existing Dimmi article. New slug `-2`; redirect from old URL to new article still correct.
94. Old article path collides with reserved provider path (`/changelog/...`, `/feedback/...`). 08 refuses: error `redirect_path_reserved`; listed in errors with old URL.
95. Redirect `from_path` already exists (manual redirect or earlier import) to other target. Not overwritten; error `redirect_conflict`; same target = no-op.
96. Old host is vendor domain (`acme.intercom.help`) that cannot point to Dimmi. Redirect rows still created (harmless); import resource `warnings: old_host_not_on_dimmi` with guidance (vendor-side redirect or custom domain move).
97. Article cross-links to other imported articles. `article_links` stage rewrites links matching old article URLs/ids to new article URLs (relative `/slug`); links to non-imported pages unchanged.
98. Article images hosted on source CDN. Downloaded, rehosted as `help_center_assets` (dedupe sha256), Markdown rewritten; failure keeps original URL (warning) so page still renders while vendor CDN alive.
99. Article body has embedded iframe/video (YouTube, Loom). HTML iframes dropped by sanitizer; replaced by link line `[Video](url)`; count warning.
100. Article contains HTML tables, callouts. GFM tables kept; Intercom/Zendesk callouts -> GitHub alerts `> [!NOTE]` when recognizable, else blockquote.
101. Draft at source, `publish: preserve`. Imported draft; never published.
102. Published at source, `publish: draft`. Imported draft; redirects created anyway (resolve only when published; 08 resolves redirect to unpublished article as 404, so preview warns).
103. Help center article cap reached (`help_center.max_articles` 5,000 cloud). Excess skipped `entitlement_limit_reached`; run `completed_with_errors`.
104. Public site page served with `noindex` or behind cookie wall. Imported if content extractable; cookie wall/empty body -> skipped `site_requires_js` warning.

## J. Feedback and changelog

105. Canny post with score 120 and 80 identifiable voters. 80 votes (trust `imported`, original vote time when available) + `imported_vote_count` 40.
106. Voter email matches existing contact. Vote attaches to that contact (no duplicate vote: 14 unique `(post, contact)`).
107. Canny/Featurebase merged posts. Merged child skipped (`merged_at_source`); parent carries votes already.
108. Comment replies deeper than 14 supports. Flattened to parent thread with `> quoted parent` prefix.
109. `feedback.posts` cap reached. Remaining posts skipped, record error `entitlement_limit_reached` (key `feedback.posts`, same code as 14 create surfaces); run continues other stages.
110. Board mapped `skip`. Its posts skipped (counted); votes and comments of skipped posts skipped.
111. Changelog entry scheduled at source for future date. Imported draft (never auto-schedules mail).
112. Productlane portal issue linked to imported thread. 14 `feedback_request` link via helper when both imported (no events).

## K. Delta sync and switchover

113. Delta every 10 min for 7 days. `ScheduleImportDeltas` enqueues when previous delta finished and `next_sync_at` due; overlapping schedule never stacks (one active run rule); `sync_until` passed: sync disabled, `import.updated`.
114. New parts at source on conversation already imported and untouched in Dimmi. Appended (dedupe keys); status follows source (status_mode preserve) else stays closed.
115. New parts at source on conversation also replied in Dimmi (rehomed). Parts appended by time; Dimmi status untouched (Dimmi wins once touched by live message).
116. Contact updated at source and in Dimmi. Delta fills empty Dimmi fields only; never overwrites Dimmi values (same as matched rule).
117. Record deleted at source after import. Not deleted in Dimmi (no tombstone sync); record `external_meta.deleted_at_source` noted; rollback still possible.
118. Delta during Intercom outage. Source 5xx streak (5 consecutive): run `paused` `source_unavailable`, `resume_at` +10 min backoff (max 1 h); next scheduled delta skipped while paused.
119. Watermark correctness when delta fails halfway. Watermark advanced only by successful finalize; next delta restarts from old watermark (dedupe makes repeat cheap).
120. Cutover: member archives import while delta running. 409 `import_run_active`; cancel run first (CLI `archive` cancels then archives with prompt).

## L. Rollback

121. Rollback after full + 3 deltas. Deletes every `created` record of import across runs, in order: redirects, article links n/a, articles (hard delete incl. versions; were drafts or published), collections (when empty after), changelog entries, feedback comments, votes (via posts), posts, conversations (messages, attachments rows; files purged by 03 attachment purge), saved replies, contacts, companies, tags (only if unused by non-imported rows), teams created by import (only if empty).
122. Imported conversation received live message or teammate reply after import. Kept (`kept`, reason `modified_after_import`); its import record left; listed in rollback totals.
123. Imported contact has live conversations (widget) after import. Kept; imported conversations of that contact still deleted.
124. Matched records (disposition `matched`). Never deleted; fields filled by import not reverted (documented).
125. Article edited in Dimmi after import. Kept (`modified_after_import`).
126. Rollback with partial scope `["articles"]`. Only articles, collections, redirects, assets of import; other scopes untouched; import stays `active`.
127. RetentionGuard returns false (legal hold later). Subject skipped `retention_held`, counted; run `completed_with_errors`.
128. Rollback interrupted (crash). Resumable like any run; deletes idempotent (missing row = skip).

## M. GDPR, security, tenancy

129. Contact erased (03 `EraseContact`) after import. Eraser deletes `import_records` for contact and its conversations, scrubs `import_errors` with its external id, writes `import_erasures` HMAC tombstones (external id, each email); delta/re-run skip matching source contact (`erased_contact_skipped`) and conversations whose author resolves only to it.
130. Erased contact's conversation also involves other participants. Conversation shell handled by 03 erase rules; tombstone prevents re-import of that conversation (keyed by conversation external id added to tombstones when contact was primary).
131. Workspace deleted mid-run. Purge cascade removes import tables; jobs exit on missing run; credentials crypto-shredded with workspace DEK destroy.
132. Member with `imports:manage` removed mid-run. Run continues (actor recorded; system continues); new runs need permission.
133. Cross-tenant: `GET /imports/{other ws imp}`. 404 (never 403).
134. Token with `settings:write` ability from member role. `authz.Can` fails (member lacks `imports:manage`): 403.
135. MCP agent asks for credentials. Tool never accepts/returns secrets; returns `credentials_url`; confirm gate on rollback shows counts.
136. Imported HTML with script/XSS payloads. Sanitized at conversion; Markdown render raw HTML off; XSS corpus test on article and message paths.
137. Source data contains prompt-injection text. Stored as untrusted content; 07 wraps as untrusted when used; no import-time AI.

## N. Limits, editions, upgrades

138. `imports.records_monthly` hit at 90% of run. Run `paused` `entitlement_limit_reached`, `resume_at` first of next month UTC; staff override resumes immediately (entitlement bust triggers `resume_at = now`).
139. Self-host $5 VPS (1 vCPU, 1 GB). Defaults `imports.queue.workers` 4 but memory per worker under 64 MB; page sizes adapt to keep tx under 200 ms; import of 10k conversations finishes under 30 min (bench in 11 CI image).
140. Source API version deprecated (Intercom-Version sunset). Pinned version config; driver contract tests on recorded fixtures; operator bumps `imports.intercom.api_version` after fixture refresh; sunset warning header from source logged + `dimmi doctor` warning.
