# 15 Engage: edge cases (decided)

Style: caveman. Format: "Case. Behavior." Numbered across groups.

## Eligibility and targeting

1. Workspace has no live items. Config `engage.active=false`, `anonymous=false`; SDK sends zero engage requests, loads zero engage chunks.
2. Config cached by CDN for 60 s says `active=false` right after publish. Item appears on next config refresh (at most about 60 s plus page load). Documented "publish takes up to 1 minute to reach visitors".
3. Item published while visitor has page open (SPA, no reload). Visible after next refetch: `refresh_interval_ms` (10 min), identity change, or `Dimmi('update')`. No realtime fan-out to every contact socket (cost).
4. Item paused or ended while visitor has it cached. Client keeps showing until next fetch at most 10 min; `engage-events` for that item return `item_not_available` and SDK removes it immediately on that response. Urgent removal: ending is reflected on next page load.
5. Contact attribute changes (JWT claim `plan` from `free` to `pro`) mid-session. ETag includes contact attribute hash; SDK refetches on `identity_changed`; next list reflects new plan.
6. Saved audience edited while items live. Applies at next evaluation (resolved at request time); `audience_references` finds items, engage version bumped so caches and ETags change.
7. Saved audience deleted while referenced by draft item. Allowed; builder shows `missing_reference`; publish blocked.
8. Saved audience delete attempted while used by live item. 409 `audience_in_use` lists items.
9. Custom attribute stored as string `"42"` compared with `gt 10`. Type mismatch: false (never coerced). Dashboard rule builder shows attribute type from sample values to avoid this.
10. Custom date attribute as unix seconds vs ISO string. Both accepted by date ops; any other type false.
11. Attribute missing on contact with `neq "free"`. True (missing counts as not equal). Documented; use `exists` + `neq` for strict.
12. Lead (anonymous with session) and audience `identified`. Not eligible. Lead becomes user via JWT mid-session: refetch makes eligible.
13. Contact blocked (`blocked_at`). Never eligible for any item; widget engage routes return empty list; CSAT never sent.
14. Contact merged (lead into user). InTx listener repoints receipts and responses to target; receipt conflict (both saw same item): keep earliest `first_seen_at`, sum impressions, keep any `dismissed_at`/`completed_at`, max `cycle`.
15. Contact merged where both completed same survey cycle. Both responses kept (target contact), results count both; rare, documented.
16. Two items with same priority eligible on same page. Order by `published_at` asc (older first), then id. Deterministic across devices.
17. Over 70 live items with complex rules. Evaluator budget under 1 ms total measured in benchmark; cloud live caps (entitlements 50 + 20) bound it; self-host unlimited by default (operator sets keys if needed).
18. List response would exceed 64 KB (many translations, long bodies). Lowest priority items dropped, `meta.truncated: true`; dashboard warns at publish when item body over 10 KB.
19. Locale requested `pt-BR`, translations have `pt`. Primary subtag fallback to `pt`; then default locale per field.
20. Audience leaf `contact.locale in [de]` and contact has no locale. Missing value: false, not eligible. Author wanting browser language uses `visitor.locale` (client leaf).
21. Workspace suspended. `live_summary.active=false`; lists empty; CSAT jobs skip; items keep status (resume when unsuspended).
22. Workspace soft-deleted. Widget already 404; engage jobs skip workspace.
23. `engage.enabled=false` on self-host. Config omits engage block (`active=false`); admin routes 404 `feature_disabled`; data untouched.
24. Test session (dashboard preview) without `preview` param. Sees live items like real contact; receipts and responses `is_test`; stats excluded by default.
25. `preview` param on non-test session. 403 `preview_requires_test_session`.
26. Preview of draft with deleted saved audience. Preview ignores audience entirely (shows item); publish still blocked.
27. Contact in two widgets (`wgt_A`, `wgt_B`) of same workspace. Items are workspace-wide; receipts per contact so seen-once applies across widgets. Per-widget targeting via `page.host` leaf.

## Triggers (client) and page rules

28. SPA route change. SDK re-evaluates residual on 01 route detection; pending delay timer for item whose residual no longer matches is cancelled.
29. `matches` glob with `**` and query string. Matched against leaf's field: `page.path` path, `page.query` query, `page.url` whole URL; fragment never. Shared JSON corpus (18) guarantees SDK and server (`app/support/urlmatch`) agree.
30. Leaf `page.url starts_with https://other.com/x`. Host compared because field holds whole URL; item never shows on other hosts.
31. Page leaf contains regex-like input `(a+)+`. Treated literally (glob only); no ReDoS possible.
32. `track_event` fired before engage list fetched. SDK buffers last 20 track calls for 30 s and replays after list arrives.
33. `track` called with unknown event or no matching item. No-op, no network (sdk-contract).
34. `track` called 1,000 times per second by host bug. Client-side only; bounded buffer; item frequency still applies.
35. `delay_seconds` 3600 and visitor leaves after 10 minutes. Not shown; no event sent.
36. Device `mobile` and desktop browser narrowed below 640 px. Treated mobile at evaluation time; decision fixed until next page view.
37. Messenger panel open when modal trigger fires and `quiet_while_messenger_open`. Deferred until panel closes (same page view); page change drops it.
38. Two tabs: modal shown in tab A, tab B fires same `once` item within seconds. SDK `storage` event marks seen across tabs; race where both show: server receipts merge; at most 1 duplicate impression accepted.
39. User typing in host page input when modal triggers. SDK waits until focus leaves editable element (max 30 s), then shows; avoids keystroke hijack (WCAG and usability).
40. Host page uses strict CSP without `img-src` for Dimmi host. Images fail; alt text shown; console warns `csp_blocked` once (01 pattern).
41. Host page sets `engage: false` boot option (e.g. checkout page). SDK skips engage entirely on that page.

## Frequency and throttle

42. `once` item and visitor never closes it (navigates away). Counted seen at render; never shown again.
43. `until_engaged` banner dismissed. Never shown again (Intercom-like permanent dismissal), even when `max_impressions` not reached.
44. `until_engaged` with `min_hours_between 24` and 5 page views today. Banner shows on each page view, impressions count increments at most once per 24 h toward `max_impressions`.
45. `recurring` NPS completed. Next cycle eligible after `recur_days` from `completed_at`; dismissed: after `recur_days` from `dismissed_at`.
46. Global throttle hit (2 interruptive today). Third item omitted until `next_slot_at`; banners and `bypass_throttle` items unaffected.
47. Throttle window crosses midnight. Rolling 24 h, not calendar day; timezone irrelevant.
48. Anonymous visitor (no session) throttle. Enforced from `localStorage` mirror; cleared storage resets (accepted, no identity).
49. `localStorage` unavailable. Memory-only: frequency holds for page lifetime; `once` items may reappear on next load; anonymous interruptive items suppressed entirely to avoid nagging (banners still show, dismissal per page).
50. Survey cooldown active and NPS recurring due same day. NPS waits until cooldown ends (cooldown wins) unless `bypass_throttle`.
51. Item edited with `reshow: true`. Cycle incremented for all receipts; `once` item shows again; audit records actor; contacts already dismissed see it again (explicit admin choice).
52. Visitor gets session after seeing anonymous items. SDK uploads local seen/dismissed (max 7 days old, max 50) once; server receipts created with original timestamps; no double show.
53. Clock skew on client (`at` far future/past). Server clamps to `[now - 7 d, now + 60 s]`; ordering uses server time.

## Content and rendering

54. Markdown body with raw HTML `<script>`. 08 renderer escapes raw HTML; sanitizer pass; stored `body_html` only safe tags.
55. Link `javascript:` in body or CTA URL. Rejected at save (422 `invalid_url` for CTA; body link rendered as text).
56. External `http://` image in body. Blocked at render (08 rule), warning `external_http_image`.
57. Image hosted elsewhere via `https`. Allowed in body; recommended assets; no proxy.
58. Variable `{{contact.first_name}}` in anonymous list. Rendered with fallback (or empty); never error; contact data never in CDN-cached response.
59. Variable referencing unknown key. Rendered blank (03 saved reply rule); publish warning `unknown_variable`.
60. Cover asset infected after publish (async scan). Asset 404; modal renders without cover; dashboard alert event; item keeps running.
61. Sender member deactivated (chat format). Shows workspace name + logo; publish of new chat items with that sender blocked.
62. CTA `open_article` to article later unpublished. CTA hidden in widget; warning `article_unpublished_cta` in dashboard via `article.unpublished` consumer.
63. CTA `open_changelog_entry` and widget changelog tab disabled. Falls back to entry public URL in new tab; unpublished entry: CTA hidden.
64. CTA `open_survey` to survey that ended. CTA hidden; announcement still shows.
65. CTA `sdk_event` and host registered no handler. SDK emits event, closes item, logs `console.info` once.
66. RTL locale (`ar`, `he`). Item resource carries `dir: "rtl"`; widget renders mirrored; CSAT rating page `dir` attribute.
67. Emoji, combining characters in title. NFC normalized; length limits count grapheme clusters for UI limits, bytes for storage limits.
68. Accent color with poor contrast. Publish warning `low_contrast`; never blocks. `banner.tone` palette fixed AA.
69. Translation for locale removed while contacts mid-survey in that locale. Next question falls back to default locale per field; answers keep `locale` of response start.

## Surveys: structure and branching

70. Branch rule pointing backwards. 422 `branch_backwards`; loops impossible by construction.
71. Branch target question archived. 422 `branch_target_archived` at publish; live survey: archived target treated as `end` (edge 72 prevents reaching it).
72. Question archived on live survey while respondent mid-way. Respondent skips it on next step (server computes next non-archived); answers already given kept.
73. Question type change requested after answers exist. 422 `question_type_locked`; create new question instead.
74. Option label edited after answers. Results show current label; CSV uses current labels; option id stable.
75. Option removed after answers. Archived, hidden for new respondents, kept in results with "(archived)".
76. Survey with only `statement` questions. Publish blocked `no_questions`.
77. Required question skipped on branch path. Required only applies to questions on actual path; complete succeeds.
78. `multiple_choice` with `max_select 2` and 3 sent. 422 `invalid_answer_value`.
79. `allow_other` with empty other text. Treated as choice `other` with null text; required other text not enforced at MVP.
80. Answers submitted out of order (step 3 before step 1) via API. Server validates against path computed from all answers; answer not on path: 422 `answer_not_on_path`.
81. Survey edited (revision bump) between list fetch and answer. Compatible answers accepted; incompatible: 409 `survey_changed` with fresh item.
82. 20 questions limit reached. 422 `question_limit_reached`.
83. NPS value 11 or -1. 422 `invalid_answer_value`.

## Surveys: responses

84. Same `client_id` retried after network drop. Returns existing response (idempotent), no duplicate.
85. Contact answers on phone, then opens laptop. Session list returns `state.response_id` + `partial_answers`; laptop resumes at next question.
86. Contact completes on phone while laptop shows same survey. Laptop answer gets 409 `response_completed`; SDK shows thanks.
87. Partial response never completed. Stays `partial`; counts in funnel `started`; included in results for answered questions (`answered` counts), flagged; excluded from headline NPS (completed only).
88. Text answer contains card number. Luhn match redacted before insert; `redacted: true`; metric.
89. Text answer contains prompt injection ("ignore previous instructions"). Stored as plain text; MCP output in untrusted envelope.
90. Text answer with 50 KB pasted. 422 `answer_too_long` (widget shows counter, never truncates silently).
91. Attribute write-back key collides with JWT-managed key `plan`. Written; next JWT boot overwrites (02 rule). Dashboard warns when key seen in recent JWT claims.
92. Attribute write-back would exceed 100 custom attribute keys. Write skipped, response completes, `meta.warnings` `attribute_limit` logged and dashboard event.
93. API response with `contact.external_id` unknown and no `create`. 422 `contact_not_found`.
94. Survey deleted (soft) with responses. Responses hidden with survey; restore brings back; purge after 30 days deletes responses (cascade); stats rows deleted.
95. Response delete by admin. Hard delete; receipt kept (contact not re-asked); results cache busted.
96. Results request at 1M responses with filters. Valkey cache 60 s per filter set; statement timeout 5 s; timeout returns 503 `results_timeout` with hint to narrow range (never partial numbers).
97. `filter[audience]` on results uses current attributes. Documented; snapshot of attributes at answer time not stored (only `context` page context).

## CSAT

98. Conversation closed then reopened within delay window. `conversation.reopened` cancels job when possible; worker re-check skips (`still_open`).
99. Conversation closed, reopened, closed again within 1 h. Each close schedules job unique by `closed_at`; cooldown (`cooldown_days`, per contact) stops second request when first already sent.
100. Closed by merge (source conversation). `conversation.closed` with reason merge: skipped (`merged`).
101. Conversation marked spam or contact blocked after request sent. Response still accepted if answered; excluded from results (`filter` default excludes spam conversations).
102. Conversation with only AI replies and `include_ai_only=false`. Skipped. `true`: sent, `ai_involved=true` for split results.
103. Conversation with no workspace reply (customer wrote, team closed silently). Skipped (`no_reply`).
104. Email channel, contact email suppressed (hard bounce). Skipped (`suppressed`).
105. Email client link scanner opens every rating link. GET never records; only POST from page submits.
106. Customer clicks "1" in email then changes mind on page to "4". Page preselects 1, customer selects 4, submits: 4 stored.
107. Customer opens CSAT link after 14 days. `expired`: page "This survey closed. Reply to the email to reach us."
108. Customer replies to CSAT email with text instead. 04 inbound: customer message, conversation reopens (normal rule); CSAT request remains open until expiry.
109. Widget CSAT card answered on second device. Card fetch returns `completed`; shows thanks.
110. Chat-app conversation (Slack, Discord, Telegram, WhatsApp) closed. Never sent: `csat.channels` accepts only `widget`, `email` (other values 422 `invalid_value`); worker skips (`channel_not_enabled`). Later: chat app csat.
111. Live CSAT survey paused. Pending jobs skip (`survey_not_live`); outstanding requests stay answerable until expiry (status paused still accepts answers to already-sent requests).
112. CSAT request message on conversation that later gets erased. Eraser removes response rows; message body removed by 03 eraser.
113. `is_test` conversation (preview). Never sent unless triggered from survey preview flow (explicit test, `is_test` response).
114. Negative CSAT on unassigned conversation. 06 notifies team members of conversation team; no team: workspace admins in-app only.

## CSAT email rating page

115. CSAT rating link tampered (`svr` or `expires` changed). Signature fails: 403 page. `score` unsigned (preselect only).
116. JS disabled. CSAT rating page works via plain form POST.

## Concurrency, jobs, limits, upgrades

117. Two admins publish 50th and 51st live announcement simultaneously. Cap check under `SELECT ... FOR UPDATE` on `engage_settings` row in publish tx; one gets 422 `entitlement_limit_reached` (`key: engage.max_live_announcements`, `limit: 50`).
118. Two members edit same announcement. Last write wins unless `If-Unmodified-Since` sent (409 `stale_resource`); both `announcement.updated` events recorded.
119. Stats flush fails (DB down). Counters kept in memory, retried next interval; replica crash loses at most 60 s counters; receipts (durable) unaffected.
120. `TransitionEngageSchedules` job delayed 5 minutes (queue backlog). Eligibility checks `ends_at` at request time anyway; job only updates status and summary; no item shown past `ends_at`.
121. DST change between schedule creation and start (`Europe/Berlin` 02:30 on spring-forward). Stored UTC from offset sent by client; dashboard converts; nonexistent local time rejected by dashboard, API accepts only absolute timestamps.
122. Item `ends_at` before `starts_at`. 422 `invalid_value`.
123. Renderer version bump (08). Boot enqueues re-render of `rendered` cache for live items; old HTML served until done (safe either way).
124. Receipt table growth at 1M contacts x 60 items. About 7 GB worst case; pruning 365 days after item end; `dimmi doctor` reports engage table sizes.
125. GDPR erase for contact with 10,000 receipts. Eraser deletes in batches of 1,000, idempotent on retry.
126. Contact export (03) for contact with responses. Export includes receipts and responses with question prompts.
127. Webhook consumer slow; burst of 5,000 `survey_response.completed` after email CSAT batch. Standard 10 fan-out, per-endpoint ordering by sequence; no special handling.
128. Migration on self-host with no widget installed. Tables created; engage idle; zero runtime cost.

## Cross-feature: CTA targets, SDK

129. CTA `open_article` clicked after article unpublished, before list refetch. SDK `show('help', {article})` gets 404 from `GET /widget/v1/help/articles/{art}`: Help home opens, `console.warn` `view_unavailable`; next fetch hides CTA (case 62).
130. CTA `open_article` on widget with Help tab off or no help center. Publish warning `help_tab_disabled_cta`; widget item resource omits CTA for that widget (per-widget check at list time).
131. CTA `open_changelog_entry` entry unpublished between fetch and click. 13 route `GET /widget/v1/changelog/entries/{cle}` 404: SDK opens "What's new" home; next fetch hides CTA (`changelog.FindPublishedEntry` false).

## Valkey (rebuildable state)

132. Valkey flushed mid-day. Receipts, throttle, show-once, responses untouched (Postgres): no item reappears. Rate limit windows reset (brief burst allowed). Results cache cold: next request recomputes. otter L1 keeps serving until `engage_settings.version` changes (version in key, so no stale item survives a publish).
133. Valkey unreachable. Widget lists still served (Postgres + otter L1); engage-events rate limiter fails open with metric `dimmi_engage_limiter_errors_total` (receipts still durable); config busts delayed: replicas pick up new `version` on `engage.items_cache_ttl` expiry (30 s); realtime CSAT rating push lost (inbox refetches on focus).

## Imported data (16)

134. 16 imports closed conversations (Intercom, Zendesk, Help Scout). No `conversation.closed` dispatched (16 suppression), no CSAT job; defense: worker skips `source.imported` conversations with no live message after import. Imported conversation later reopened by live reply and closed by teammate: normal CSAT eligibility (live close cycle).
135. Imported source CSAT ratings (Intercom `conversation_rating`). Stay in 16 `metadata.import.rating`; never `survey_responses`, never in 15 results or CSAT stats, never trigger 06 `csat_negative` (16 E45).
136. Imported contacts match audience (backdated `signed_up_at`, imported custom attributes). Nothing sent: announcements and surveys show only on live widget visit (client fetch after handshake); import creates no receipts, no engage events, no `survey_response.*`.

