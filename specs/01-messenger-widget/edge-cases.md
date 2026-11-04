# 01 Messenger widget: edge cases

Style: caveman. Each: case. decided behavior. Identity-specific cases live in [02 edge-cases](../02-widget-identity/edge-cases.md); referenced where they touch widget.

## Boot and config

1. Wrong app id (typo, other region, deleted widget). Config 404 `app_not_found`; SDK one `console.error` with docs link; launcher never renders. No retry loop.
2. Malformed app id (not `wgt_`, bad base32). Same 404 `app_not_found` (never 400; no format oracle).
3. Widget disabled by admin while visitors have panel open. Next request (any) returns 403 `widget_disabled`; sessions stay valid (not revoked); SDK hides launcher on next config refresh (60 s max) and on any 403 `widget_disabled`. Messages already typed stay in outbox, not sent. Re-enable restores without data loss.
4. Snippet pasted twice on page. Loader guards `window.__dimmiLoaded`; second `boot` same app id = `update`, warn `boot_twice`.
5. Two different app ids on one page (marketing + app widgets). Second `boot` shuts first down (one widget per page). Warn.
6. `boot` called before DOM ready or in `<head>`. Queue stub safe; loader mounts on `DOMContentLoaded`.
7. Config cached at CDN then settings changed. Max 60 s staleness; `config_version` in session response (`GET`/`PATCH /widget/v1/session`, `POST /widget/v1/sessions`) newer than cached config: SDK refetches config with `?v=<version>` cache-buster key. Allowed origins change applies on server immediately (origin check never cached at CDN: CDN key varies by Origin and 403 responses `no-store`).
8. Allowlist set but site served from `www.` and apex. Two entries needed; install status shows `origins_blocked` with exact origin and one-click add. Wildcard `https://*.acme.com` does not match apex `https://acme.com` (documented, shown in UI validation hint).
9. Origin `null` (file://, sandboxed iframe, data URL). Treated as not allowed when allowlist non-empty; allowed when empty.
10. `localhost` development with production allowlist. Not auto-allowed. Install status lists `http://localhost:3000` under blocked with add button. Docs recommend test widget per environment.
11. Workspace suspended. Config 403 `workspace_suspended`; open realtime streams disconnected within 5 s; messages from existing sessions 403. Data kept.
12. Workspace soft-deleted. 404 `app_not_found`.
13. Host page CSP blocks loader or connect. SDK detects `securitypolicyviolation` event; console `csp_blocked` with exact directive to add.

## Sessions and storage

14. Visitor never interacts. Only request: config GET (CDN cacheable). No session, no contact, no DB rows. Install tracker coalesces in memory: at most one `last_boot_at` write per widget per minute per replica.
15. Visitor clears storage mid-conversation. New anonymous visitor next write; old conversation not visible (security over convenience). If email given earlier, replies still reach email and email reply continues thread.
16. Safari ITP removes storage after 7 days without visit. Same as 15. Identified users restore via JWT.
17. `localStorage` unavailable. Memory-only session; email asked on first message; warn `storage_unavailable`.
18. Third-party iframe embed (host app inside other site's iframe). Partitioned storage; separate visitor per top-level site; no cookie dependence so no breakage.
19. Token stolen from storage and replayed elsewhere. Works until expiry/revoke (bearer). Mitigations: origin check (browser), anonymous sessions see only own lead's threads, identified sessions expire 24 h without fresh JWT, `logout` revokes server side. Stolen 60 s realtime connect token (URL, proxy log): expires in 60 s, connect only, session revoke re-checked on connect. Documented threat model.
20. Session token for revoked session sent. 401 `session_revoked`; SDK clears, remounts anonymous.
21. Anonymous session expires (180 days idle). `GET /widget/v1/session` 401 `session_expired`; SDK drops token, continues without session; old lead kept in inbox, never merged into later session.
22. Contact erased (GDPR) while widget open. Sessions revoked by eraser `EraseWidgetContactData` in 03 erase job; realtime streams disconnected; 14 portal sessions (surface widget) revoked; widget remounts anonymous; no residual messages in SDK memory (SDK wipes on revoke).
23. Contact merged by teammate: lead into lead. Sessions repointed to surviving contact; old channel gets `contact.merged {into}`; SDK fetches new realtime token (server subscribes `widget_contact:{into}`); conversations of both visible.
24. Contact merged by teammate: lead into user contact. Unverified sessions revoked (`contact_merged`), not repointed: lead cannot gain verified history. Visitor sees fresh widget; replies continue by email if email known.
25. Contact blocked. Widget behaves normally (no tell); messages stored `spam = true`, no notifications, no AI, never billed.
26. Widget deleted by admin. Sessions revoked (`widget_deleted`), realtime streams disconnected, config 404. Conversations remain in inbox; channel connection marked `disconnected`.
27. Clock on visitor device wrong by hours. All expiry decisions server side; SDK uses `server_time` offset for display.

## Conversations and messages

28. Same message sent twice (retry after timeout, double click). `client_id` dedupe returns original, 200. No duplicate row, no duplicate event.
29. Same `client_id` reused with different body (buggy client). Returns original message (dedupe wins), logs warn. Not 422 (avoid losing typed text on flaky networks).
30. Conversation closed by teammate while visitor typing. Widget gets `conversation.closed`; composer stays enabled with hint "Sending reopens this conversation". Send reopens (03 rule, reason `customer_message`).
31. Closed beyond reopen window (default 30 days). Send creates new conversation linked via `metadata.previous_conversation_id`; response `meta.redirected_from`; SDK switches thread view.
32. Conversation deleted by teammate while open in widget. Messages 404; SDK removes thread, offers "Start new conversation" with draft preserved.
33. Conversation marked spam. Visitor sees nothing different; replies none. Unmark restores.
34. Visitor sends to conversation of another contact (guessed id). 404 `conversation_not_found` (same for nonexistent).
35. Very long message (over 64 KB). 422 `body` max error; SDK counts before send and warns at 90%.
36. Empty body with no attachments. 422. Whitespace-only = empty.
37. Message with only emoji, RTL text, ZWJ sequences. Stored NFC; no stripping except control chars; subject excerpt cut on grapheme boundary (never split surrogate or ZWJ sequence).
38. Visitor pastes HTML or script. Stored as text; rendered as Markdown subset with raw HTML disabled; teammates see escaped text.
39. Visitor sends 61 messages in a minute. 429 `rate_limited` with `Retry-After`; outbox holds rest; bot score +20.
40. Visitor creates 21st conversation in an hour. 429; SDK suggests continuing latest open thread.
41. `require_email_when_offline` and team goes online between check and send. Server decides at send time; if online, accepted without email.
42. Team offline, `collect_email = offline`, visitor declines email. Message stored; inbox shows "No email: reply only reaches visitor in widget". No block unless `require_email_when_offline`.
43. Two teammates reply simultaneously. Both stored in commit order; widget receives both; order by `created_at, id`.
44. Message deleted by teammate after visitor saw it. `message.deleted` to widget; deleted shows placeholder "Message deleted".
45. Internal note created. Never reaches widget channel, polling, or list (filter `type = 'message'` in every widget query plus resource guard; test asserts).
46. Teammate reply over 10 KB broadcast limit. Widget gets `{id, object, updated_at}`; SDK fetches message via REST.

## Attachments

47. Upload interrupted (tab closed). Row stays `uploaded_at null`; purged by 24 h unattached job (03); object deleted.
48. Presigned URL expired before upload (15 min). S3 returns 403; SDK requests new upload (new attachment id); old purged.
49. Declared size differs from uploaded. `PATCH {status: "uploaded"}` deletes object, 422 `attachment_size_mismatch`.
50. File renamed `.png` but is executable. Sniff on upload PATCH detects; deleted; 422 `attachment_type_blocked`.
51. Huge image (decompression bomb). Header decode only; over 40 megapixels rejected; never fully decoded server side.
52. S3 down during upload PATCH. 503 with `Retry-After`; SDK retries PATCH (idempotent: `status: uploaded` on uploaded row returns 200).
53. Message references attachment still pending. 409 `attachment_not_uploaded`; SDK awaits upload PATCH then retries same `client_id`.
54. Attachment uploaded by session A used by session B (same contact, other tab). Allowed when same contact; other contact 404.
55. Attachments disabled after upload, before send. Send rejects 403 `attachments_disabled`; text still sendable; attachment purged later.
56. Malware scan driver flags infected after message sent. 03 marks `scan_status infected`; widget resource returns attachment with `url: null`, `blocked: true`.
57. Local disk driver with multiple replicas. `dimmi doctor` fails check (foundation rule: S3 required for replicas); direct uploads to replica without file would 404 on serve; documented hard requirement.

## Realtime, tabs, network

58. SSE blocked or buffered (corporate proxy, first-party proxy that buffers). 2 failed connects in 30 s or no first frame in 10 s: polling fallback; delivery under 6 s p95; SSE retried every 5 min.
59. Ten tabs open. Ten SSE streams; eleventh connect disconnects oldest (code 4004); that tab falls back to polling silently.
60. Laptop sleeps 3 hours. Stream dead; on wake SDK fetches fresh connect token, reconnects with last `offset`/`epoch`; history (10 min) gone, server answers `recovered: false`; SDK refetches via REST + `/updates?since` (DB-based). Identified session may have expired: identity source refreshes.
61. Duplicate event (recovery replay overlaps REST refetch, or polling then stream). SDK dedupes by event id and message id.
62. Visitor offline sends 5 messages. Outbox persists in storage; flush in order on `online`; server timestamps = arrival time (not device time); order preserved by sequential send.
63. Tab open for 30 days. Anonymous token slides; identified refreshed by identity source on navigation / visibility (sdk-contract) at least every 24 h, else drops to anonymous with warn `identity_refresh_missing`; realtime reconnects handled (fresh connect token each time).
64. Server deploy (rolling restart). Centrifuge node shutdown disconnects streams with reconnect advice; SDK reconnects with jitter (0 to 5 s random delay), recovers from Valkey history (shared across replicas, `recovered: true`).
65. `dimmi down` maintenance. API 503 `maintenance` with `Retry-After`; SDK shows offline banner, keeps outbox (foundation rule).
66. Typing event spam. Server throttles 1 per 2 s; ignores extras; never stored.
67. Teammate typing while visitor stream closed. Dropped (ephemeral, published without history: recovery never replays typing). Not in polling.
68. Connect token expires between `POST /realtime-tokens` and `EventSource` open (slow network, over 60 s). Connect rejected (centrifuge disconnect `token expired`); SDK treats as connect failure, fetches new token, retries with backoff. Never reuses old URL.
69. EventSource built-in auto-retry after network blip reuses URL with expired token. SDK closes EventSource on first `error` before browser retry; token single purpose and 60 s, so stale retry fails harmlessly.
70. Connect token for session revoked after issue (logout in other tab within 60 s). `OnConnecting` re-checks session: rejected; SDK sees `session_revoked` on next REST call and remounts anonymous.
71. Forged `cf_connect.subs` naming other contact's channel. Ignored: server-side subscription comes only from token claims; client `subs` used only for recovery position of that channel.
72. `cf_connect` (token) captured in host access log or proxy log. Server access log redacts it; leaked value expires in 60 s, cannot call REST.
73. Valkey restart (history lost). Reconnects get `recovered: false`; SDK refetches via REST; no message loss (DB is source of truth).

## Offline email

74. Teammate replies, visitor gone, has email. Email after 120 s unless read meanwhile; replies in that window batch into one email (unique job per conversation).
75. Visitor reads in widget at 119 s. Worker skips (all read).
76. Visitor has unverified email typed by someone else. Email sent with "someone entered this address" footer and unsubscribe; reply from that inbox threads back via 04 and marks email verified per 04 rules (not merge).
77. Email bounces. 04 marks delivery failed and suppresses address; later offline replies return `skipped{suppressed}` from 04 `SendWidgetOfflineReply` (job done, no retry); inbox shows; no retry storm.
78. Visitor replies by email to widget conversation. 04 threads into same conversation; widget shows message with author contact (channel email), visible in widget thread.
79. Contact unsubscribed from conversation emails. No offline emails; inbox shows "Visitor unsubscribed from email".

## Abuse

80. Bot hits `/sessions` from one IP 500 times/hour. Above 20/hour PoW required; above 60/hour 429; bot score raised; metric emitted.
81. Distributed bots (many IPs) on one app. Per-app anomaly: over 10x 7-day hourly median sessions, PoW `maxnumber` raised (adaptive, up to `widget.pow.max_number` ceiling 8,000,000) for that app for 1 hour; owner notified (06).
82. Honeypot field filled. Session `bot_score 100`; conversations stored spam; 201 responses (no tell); never billed (D9).
83. Message sent 200 ms after session create. Bot score +40 (`min_send_ms`); alone not spam unless other signals.
84. Spam link floods (10 links in message). Score +20; foundation spam hook may classify; spam before reply = never billed.
85. Attacker embeds victim's widget on spam site to burn victim's quota. Allowlist blocks browsers (403). Without allowlist: dashboard shows origins seen with warning, spam conversations auto-flagged not billed. Docs recommend allowlist.
86. Header-spoofing script (non-browser) bypasses allowlist. Treated as untrusted traffic; bot checks and rate limits apply; conversations still need human reply to bill (D1); spam marking credits (D9).
87. Flood of email changes to spam victims via offline emails. Email changes 5/day/session, offline email only after real teammate reply, unsubscribe link and unverified-relay limits enforced by 04 `SendWidgetOfflineReply`.

## Settings changes mid-flight

88. Admin changes `inbox` of widget. Existing conversations stay in old inbox; new ones go to new inbox.
89. Admin turns `allow_anonymous` off. Anonymous sessions get 403 `anonymous_not_allowed` on next request; widget hidden for anonymous; data kept; identified unaffected.
90. Admin enables `require_email_when_offline` mid-conversation. Applies only to new conversations; existing threads never blocked.
91. Help center unlinked while Help tab open. Help endpoints 404 `help_center_not_linked`; SDK hides tab.
92. `max_attachment_mb` lowered after upload created. Upload PATCH re-checks against current value; too large: delete, 413.

## Test mode and billing

93. Member previews widget via test token. Session `test`; conversation `is_test = true` (03 column, via `InboundMessage.IsTest`); never billable; inbox shows Test badge; webhooks include `is_test: true`.
94. Test token reused. One-time: second `POST /widget/v1/sessions` with same token 401 `test_token_used`; preview page mints fresh.
95. Monthly test cap (entitlement `billing.test_mode_cap`, cloud 1,000 per billing account) hit. Mint returns 429 `test_quota_exceeded`; verified-JWT member-email sessions no longer flagged test; session response `test_quota_exceeded: true`.
96. Customer passes `test_token` forged. Unknown token: `POST /widget/v1/sessions` fails 401 `test_token_invalid` (no silent anonymous fallback) so dev sees error.

## AI agent

97. Agent reply arrives while visitor offline. Same as human reply: realtime, polling, offline email.
98. Agent run takes 20 s. Agent typing indicator (07 broadcasts `typing.started` author agent); auto-expire 6 s, 07 re-sends every 4 s while running.
99. Visitor clicks "Talk to human" twice. Idempotent; one `ai.handoff_requested`.
100. AI provider down. Human flow unaffected; agent typing never shown; handoff automatic per 07.

## Assets and CDN

101. New widget version deployed while visitors on old core. Loader cached max 5 min; old hashed chunks remain in binary for previous version (embed keeps current and previous release manifests) so lazy chunk fetch never 404s mid-session.
102. CDN in front strips `Vary: Accept-Encoding`. Assets served with `Content-Encoding` only when `Accept-Encoding` matches; docs require CDN to respect Vary; self-host Caddy config shipped.
103. Ad-blocker blocks `app.trydimmi.com`. Loader `onerror` -> console `assets_blocked` with proxy instructions. Script never loads, so detection lives in inline queue stub: stub sets 10 s timer, logs if loader never ran.
104. Self-host behind subpath (`https://example.com/support`). `app.url` includes path; loader and API URLs derive from it; snippet correct.

## Unicode and time

105. Name with bidi override to spoof "Kim (Staff)". Bidi controls stripped from names; widget-side names of teammates come from member profile only.
106. Page URL with unicode path. Stored as given (percent-encoded form preserved), max 2,048 chars, truncated at char boundary.
107. Visitor timezone invalid (`Mars/Base`). Ignored; contact timezone unchanged.
108. DST jump during session. All server times UTC; `office_hours_text` is free text; `within_hours` computed by 17 `officehours` in schedule timezone.

## Boot handshake

109. Host forgot `logout()`; next person on shared device loads page signed out (stored identified token, no JWT). SDK sends `DELETE /widget/v1/session` (revoked `logout`), wipes storage, continues without session. Buggy or forked client sending `GET /widget/v1/session` instead gets 409 `identity_proof_required`: identified session never resumes without JWT for same `sub` (02 R6).
110. Stored anonymous token + host JWT, `POST /widget/v1/sessions` fails (invalid JWT). No side effects: old anonymous session untouched, nothing revoked. SDK resumes via `GET /widget/v1/session` + `console.error` (mode `optional`; `required`: widget hidden, 02 validations.md).
111. `previous_token` invalid, expired, revoked, or of other widget. Ignored silently (treated as none); no merge from expired lead; no error (no token oracle).
112. Two tabs load at once with same stored anonymous token and new JWT. Both `POST /widget/v1/sessions` with same `previous_token`. 02 advisory lock serializes: first upgrades/merges lead, revokes old token; second finds `previous_token` revoked (ignored), `IdentifyVisitor` finds user contact: second identified session, same contact. Storage `storage` event makes tabs converge on last written token.
113. Config served from CDN cache. Origin server sees each origin at least once per cache window (`s-maxage` 60 s, key varies by Origin; revalidations and 304 count). `last_boot_at` about 2 minutes stale worst case; install status still turns "installed". Self-host without CDN: every load reaches tracker.
114. Identified session expired while tab open (24 h), identity source configured. Next request 401 `session_expired`; SDK asks identity source, `PATCH /widget/v1/session` revives same session (expired, not revoked, same `sub`), resends outbox with same `client_id`. Revoked session: 401 `session_revoked`, SDK starts over.

## Seams (`00-foundation/extensibility.md`)

115. Staff override raises `widget.max_per_workspace` to 20, later expires with 14 widgets. All 14 keep serving; create returns 422 `entitlement_limit_reached` (`key widget.max_per_workspace`) until under limit. Nothing deleted.
116. Two concurrent widget creates at limit - 1. `CreateWidget` counts under workspace row lock; one 201, one 422 `entitlement_limit_reached`.
117. Admin demoted to member mid-session. Next PATCH computes permissions from current role: 403 (`widgets:manage` gone). Token of demoted admin same.
118. Legal hold (later guard driver) on contact with expired widget sessions. `PruneWidgetSessions` keeps that contact's rows (current_page/recent_pages preserved), prunes others.

## Identity source (SDK redesign)

119. Identity token inlined into server HTML by host (old pattern, custom code). Page cache, bfcache or Turbo snapshot replays previous user's token after sign-out. Server cannot detect. Docs and helpers never inline (security.md, 02 snippets.md). Server side: 10-minute `exp` (helpers) limits replay; session switch rules (handshake table) still prevent cross-user history.
120. Identity endpoint response cached (CDN, service worker, `public` headers): many users get same or expired token. `token_expired` or `identity_mismatch` errors; SDK console `identity_endpoint_invalid` names missing `Cache-Control: no-store`. Handshake falls back to anonymous.
121. Next.js server action fails (deploy skew: action id changed, 404/500, network). Page load: treated as signed out for this load (fail closed), `identity_source_failed` warn; mid-session: keep state, retry on next trigger. Never boots previous user's session.
122. Server action returns `{token}` for user A while stored session belongs to B (account switch in other tab). `POST /widget/v1/sessions` with `previous_token`; B session revoked `identity_switch`; no user-to-user merge (02 R7).
123. React StrictMode double mount (dev). Unmount defers `shutdown()` one macrotask; remount cancels it: one handshake, one SSE stream, no `boot_twice` warn.
124. Identity source answers after 5 s (slow auth provider). Handshake proceeds as signed out for this load; late answer handled as mid-session check (may `POST /sessions` with `previous_token`).

## Feature tabs and engage (13 to 15)

125. `changelog_tab_enabled = true` without `help_center_id`. PATCH 422 `changelog_tab_enabled: "Link a help center first."`. Help center unlinked later: tab off in config (`features.changelog.enabled = false`), column kept.
126. Changelog entry published while visitors have cached config. 13 bumps `config_version`; next session response or 60 s refresh refetches config; spotlight appears.
127. `feedback_tab_enabled` without 14 portal settings row. PATCH 422. Public portal disabled: tab still works (independent of `portal_enabled`).
128. Visitor logs out (or identity switch, contact merge, erase) while feedback portal session (surface widget) alive. `RevokeWidgetSession` calls 14 `RevokePortalSessions` in same flow; next feedback request with old `Dimmi-Feedback-Session` 401; SDK wipes `dimmi:{app}:feedback`.
129. Feedback image uploaded via widget attachments, post saved 2 days later. Row linked by 14 to post; 03 orphan purge treats `feedback_post_id`/`feedback_comment_id` rows as attached; never purged. Never linked within 24 h: purged like any widget upload.
130. `POST /widget/v1/conversations` with `announcement_id` of item visitor never saw, or ended over 7 days ago. 422 `announcement_not_replyable`; no rows.
131. CSAT system message (15 `survey_request`) arrives while widget closed. Unread count +1 like team message; 01 offline email listener fires only for member/agent authors, so no 01 relay mail (15 owns any CSAT mail); widget shows card on open.
132. Preview page with `#preview=ann_...` for draft announcement. Test session only; draft shown; receipts `is_test`; real visitors never see draft.
133. Engage settings changed (segment edited). 15 bumps `engage_settings.version`; config cache key includes it; SDK refetches engage items after `refresh_interval_ms` or next config.

## Proof-of-work (ALTCHA)

134. Same ALTCHA solution replayed (bot shares solved payload). First use marks signature in Valkey (`SET NX EX` until expiry); second 422 `pow_invalid`.
135. Solution for challenge issued to other IP prefix. `ip` salt param mismatch: 422 `pow_invalid`.
136. Valkey unavailable during PoW verify. Fail closed for PoW-required creates (503 `maintenance`-style retry with `Retry-After`); identified and test session creates unaffected.

## Agent action card (07)

137. Visitor clicks Confirm twice fast (double click, two tabs). First PATCH 200 `confirmed`; second 409 `action_call_not_pending` (or 200 idempotent when same status already set); one `ExecuteAiActionCall` (07 unique by call id). Card shows final state from refetch.
138. Confirm click after identified session expired (24 h) with identity source. SDK `PATCH /session` refresh first, then confirm; refresh fails or `sub` differs: PATCH 409 `identity_changed`, card greyed, agent re-proposes on next message.
139. Anonymous lead sees confirmation card (should not happen: 07 offers writes only in verified context). Buttons hidden; PATCH by anonymous session 409 `identity_changed`; no call executed.
140. Card expires (15 min) while widget closed. 07 `ExpireAiActionCall` sets `expired`, 03 `inbox.SetMessageMetadataKey` updates card status, `ai.action_call_updated` published; widget opened later renders greyed card from message metadata (history recovery not needed).
141. Visitor types "yes" instead of clicking. Sent as normal message; 07 fast path confirms server side; card updates via `ai.action_call_updated`.
142. `ai.action_call_updated` missed (SSE recovery false, polling). SDK refetch of messages shows current status in `metadata.ai.confirmation`. Polling mode (no stream): while card pending or running, SDK polls `GET .../action-calls/{call}` each poll cycle; `/updates` itself carries no action call rows.

## Trusted attributes and identity stamp (07)

143. Lead sets `custom_attributes.stripe_customer_id` via `PATCH /widget/v1/contact`. Value stored; key removed from `contacts.trusted_attribute_keys` (never added). Later JWT claim sets same key: 02 adds key back (trusted again). Lead value surviving upgrade to user stays untrusted until JWT writes it.
144. Identified session expires mid-conversation, visitor keeps typing (SDK refresh failed, `identity_refresh_missing`). Messages sent after expiry (anonymous fallback session) stamped `metadata.widget.identity_verified = false`; 07 binds no identity variables for them.
145. Test session (dashboard preview) sends message. `identity_verified = false` (test sessions never verified context); agent actions with `requires_verified_contact` not offered in preview.
146. Client sends `metadata` or `widget` keys in message body. Ignored (unknown keys); stamp always server-set.

## Intercom shim and imports (16)

147. Page has both Intercom snippet and Dimmi with `intercom_shim = true`. Real Intercom loader defines `window.Intercom` first: shim does not overwrite, `console.warn` `intercom_shim_conflict` (remove Intercom snippet). Shim first, Intercom later overwrites: Intercom wins, same warn on next Dimmi boot.
148. `Intercom('boot', {user_id, email, user_hash, company, plan})`. Shim boots unverified: `user_id` -> `claimed_user_id`, email unverified hint, `company`/`plan` ignored (`intercom_attributes_ignored` info; 02 R8), `user_hash` ignored + `console.warn` `intercom_user_hash_ignored` (use Dimmi identity source). Identity source configured: source wins.
149. `Intercom('showArticle', 123)` for article not imported or unpublished. Resolver 404; shim opens Help home, `console.warn` `article_not_found`.
150. `intercom_shim = false` but SDK option `intercomShim: true`. Option wins for that page (opt-in per page); dashboard flag only sets default.
151. Identified user whose contact has imported Intercom conversations opens widget. Imported conversations listed read-only (history); reply composer on imported one starts linked new widget conversation (`meta.redirected_from`). Anonymous session with same email (unverified): imported history never shown.

## Several widgets and same person

152. Verified user chats on widget A, opens widget B. Same contact. Teammates see both threads on contact. Visitor on B sees only B threads; `GET /conversations/{A's conv}` 404.
153. Verified user has email-origin conversation; two widgets on different inboxes. Listed only on widget whose inbox equals conversation inbox. Neither matches: not listed in widget; email continues.
154. Visitor anonymous on A and B, same browser, different origins or same origin. Separate leads, separate tokens (`dimmi:{app}:*`). Typed same email: stays unverified, no merge.
155. Lead on A signs in, already has conversations on A and B as other leads. Only lead of `previous_token` merges (same widget). B's lead merges only when visitor signs in on B. Each conversation keeps own `source.widget_id`.
156. Admin moves widget to other inbox. New conversations new inbox (E88). Email-origin listing follows current inbox.
157. Widget deleted while verified user has threads there. Conversations stay in inbox; reply cannot reach widget B (stays on starting widget); offline email still sends when email known. Contact keeps other widgets.
158. Publication for other widget reaches stream on shared contact channel. Client drops by `widget_id`. Unread badge on B unaffected.
159. Eleventh widget. 422 `entitlement_limit_reached`, button disabled in page.
160. Last widget deleted. Page shows empty state + New widget; (`EnsureDefaultWidget` counts live rows, so explicit `dimmi workspace:repair` restores default).

## Settings

161. Locale `fr-CA` requested, rows `fr`. Uses `fr` row, then built-in `fr`. `locale` in response `fr`.
162. Attacker sends 10,000 random `?locale=`. Normalized before cache to bounded set; no cache growth; CDN key still varies by query, CDN origin hits answered from otter.
163. Customer writes English greeting, French visitor, no `fr` row. Shows English greeting (customer's words), French for untouched keys. Dashboard shows `fr` missing count when row exists.
164. `default_locale` changed to `de` with `texts` in English. Texts stay, label now wrong language; page warns "texts are read as German". `de` row, if any, deleted after confirm (row may not equal default).
165. RTL locale with launcher `position: right`. Launcher keeps physical side; panel content mirrors.
166. Translation row disabled. Same as missing. `locales` omits it unless shipped built-in.
167. Page rule hides launcher on `/checkout/*` while panel open mid-conversation, route changes. Stays open until visitor closes, then launcher gone.
168. Bad pattern `pricing` (no `/` or scheme). 422 `page_rules.patterns.0`.
169. Page rule hides everywhere (`only` with empty list). Allowed; launcher hidden everywhere; page warns; `Dimmi.open()` still works; install status still updates.
170. `open_to = conversation` with no open thread and `allow_anonymous = false`, anonymous. E89 wins, nothing shown.
171. Open conversation exists but closed by team. `open_to` opens new thread composer with welcome; closed ones in Messages tab.
172. `open_to = conversation`, all optional tabs off. Tab bar hidden; back goes to Messages only when visitor has more than one conversation.
173. `privacy_notice.enabled` with empty URL. Text only, no link. Empty text uses built-in.
174. `tab_order` missing `feedback`, or unknown value. Missing appended; unknown 422.
175. White header colour, white logo. `header_ink` dark handles text; logo unreadable is customer's choice; page shows contrast warning under 2:1 tile vs header.
176. Dark mode with custom colours. Brand surfaces keep customer colours; page surfaces follow theme. No auto-darkening of images.
177. Background image deleted while in use. 409 `asset_in_use`. Widget deleted: assets cascade after grace by prune.
178. Cached config refers to replaced logo. Old file still served until prune at 30 days.
179. Uploaded PNG is really HTML. `Sniff` rejects, 422 `asset_type_not_allowed`. Animated GIF logo: keeps frames, size cap applies.
180. Unsaved draft, other admin saves. Own save sends only changed keys; other keys keep other admin's values. Preview refreshes after save.
181. Sound on, visitor has never interacted with page. Browser blocks audio; no error; sound starts after first gesture.
182. Two edits within preview debounce. Latest wins; preview never saved to server.
183. Preview iframe is same origin as dashboard. Parent posts draft only to own origin; child checks origin; other frames ignored.

## Site import

184. URL resolves to private address or redirects to one. safedial refuses, `site_unreachable`; same code as dead host (no network oracle).
185. Response never ends (slowloris). 10 s timeout, 60 s total, `site_timeout`.
186. HTML 50 MB. `MaxBody` cut; `site_too_large`.
187. Script-only single-page site. Usually theme-color and icon only; else `nothing_found`; no JS run.
188. Logo URL returns SVG only. Skipped, `found` omits logo, page notes "SVG logos need a PNG upload".
189. Image decompression bomb from site. `framework/image` limits refuse; logo skipped; import still succeeds on colours.
190. Second import while first runs. Same row returned; 11th in hour 429.
191. Customer never clicks "Use these". Nothing changes; imported logo pruned as unreferenced after 30 days.
192. Stylesheet on other CDN host. Only first same-origin sheet read; cross-origin skipped by design. Theme-color and icon still work.

## Hosted page and apps

193. Request without `Origin`, allowlist set, `native_apps_enabled` false. 403 `origin_not_allowed`; SDK error names setting.
194. Hosted page opened with `#dimmi-transfer` twice (reload). Fragment removed after first read; replay 401 `transfer_token_invalid`; page falls to anonymous + "Open again from the app".
195. `#dimmi-identity` JWT expired. 401 02 code, anonymous with console; no refresh source.
196. Hosted page framed by site not on allowlist. `frame-ancestors` blocks. WebView top-level load unaffected.
197. Hosted page disabled. 404 page; existing script installs unaffected.
198. WebView clears storage. New anonymous lead. SDK-driven transfer restores session each open.
199. Hosted URL with `conversation=` of someone else. Own-only rule; 404 shows list + toast.

## Custom domain

200. Hostname already used by help center or other widget. 422 `hostname_claimed`.
201. Domain active, then DNS moved away. Check fails per 08 failure policy, status `failed` after threshold; old certificates expire; snippet warning in page; `app.trydimmi.com` keeps working for same widget id.
202. Request to `chat.acme.com/api/v1/...` or dashboard paths. 404. No cookies ever set there.
203. `chat.acme.com/widget/v1/apps/{other_app}/config`. 404 `app_not_found`.
204. Session token from default host used on custom domain, same widget. Works. Other widget: 401 `session_invalid`.
205. CDN `assets_base_url` plus custom domain. Custom domain request gets own-host chunk URLs.

## Live features

206. Delta for unknown run (joined mid-stream). Ignored; final `message.created` renders.
207. Stream drops mid reply. Bubble kept, deltas stop; on reconnect message arrives or 60 s removal then refetch.
208. `reply_discarded` after final message (ordering race). Final wins, discard ignored by run id already finished.
209. Status chip on conversation with only agent replies. `waiting_on_you`, unless `ai_state = active` then AI chip.
210. Dev context contains secret in URL query. Stripped both sides. Secret inside path segment: not detected, documented; customer can keep network capture off.
211. Host page overrides `console.error` after core wraps. Their wrapper runs ours; no recursion because ours guards re-entry.
212. Host fetch wrapper throws. Original exception passes through untouched.
213. Dev context turned off while session open. Next config stops sending; earlier stored data stays until erased with conversation.
214. Customer turned on capture with strict CSP. No extra network; only same POST bodies.

