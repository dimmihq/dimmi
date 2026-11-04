# 03 Shared inbox: edge cases

Style: caveman. Each: case. Behavior. All decided.

## A. State machine, reopen, snooze

1. Customer writes on closed conversation closed 10 days ago (window 30). Same conversation reopens (`conversation.reopened` reason `customer_message`), assignee kept, unread for assignee.
2. Customer writes on conversation closed 31 days ago. New conversation, `previous_conversation_id` = old, same contact, same thread key (old is closed so unique index allows), team = inbox default team, assignee null. Timeline of new conversation starts with event row linking old.
3. Inbox `reopen_window_days = 0`. Every customer message on closed conversation starts new conversation.
4. Inbox reopen window changed from 30 to 7. Applies to next inbound only; no retroactive change.
5. Customer message exactly at window boundary. Compare `closed_at < now - window` strict; equal counts as within window (reopens).
6. Auto-generated inbound (OOO, bounce notice) on closed conversation. Stored on found conversation even when outside reopen window (never creates new conversation), no reopen, no unread bump for assignee, `substantive = false`.
7. Customer writes while snoozed. Unsnooze to open (`conversation.unsnoozed` reason `customer_message`), `snoozed_until` null, pending `unsnooze_conversation` job later no-ops.
8. Snooze job fires after member manually unsnoozed. Worker sees status not snoozed or `snoozed_until` different from args: no-op.
9. Re-snooze to new time. New job inserted with new `(conversation_id, snoozed_until)` unique args; old job no-ops when it fires.
10. Snooze job lost (River table restored from backup, worker crash past retries). `reconcile_snoozes` every 10 min unsnoozes rows `snoozed_until < now - 2 min`; metric `dimmi_unsnooze_lag_seconds` records lag.
11. Snooze preset `tomorrow` at 23:59 member local. Next calendar day 09:00 (1 min later day boundary handled by calendar date, so tomorrow = date + 1 at 09:00, about 9 h away).
12. Snooze `later_today` at 22:00. Still now + 3 h rounded (01:00 next day). Name kept for familiarity; documented.
13. Snooze `next_week` on Sunday. Next Monday 09:00 (1 day). On Monday 08:00: following Monday (7 days), not same day.
14. Snooze to 02:30 on spring-forward day in member timezone. Resolves to 03:00 (foundation rule). Fall-back ambiguous 01:30: first occurrence.
15. Member has no timezone. Falls back to workspace timezone; token without member uses workspace timezone.
16. Snooze `until` 30 s in future. 422 `snooze_in_past` (min 1 min) to avoid instant flip.
17. PATCH `status: snoozed` on closed conversation. 422 `invalid_transition` `{from: "closed", to: "snoozed"}`; client sends `status: open` first (or reply `then: "snooze"` after `open`).
18. Close snoozed conversation. Allowed (snoozed to closed); `snoozed_until` cleared; pending snooze job no-ops.
19. PATCH `status: closed` on closed conversation. 200 no-op, `meta.changed: []`, no event, no timeline row.
20. PATCH `status: open` on open conversation. 200 no-op.
21. Manual reopen of closed A while newer B holds same thread key (open). 409 `thread_conflict` with B; member can merge A into B instead.
22. Two members close and reopen at same moment. Row lock `SELECT … FOR UPDATE` in each action; serialized; final state = last committed; both events recorded with timeline rows (foundation rule last write wins).
23. Spam conversation receives customer message. Message stored, spam stays true, no reopen side effects shown in default views, no notification, no AI trigger (event carries `ConversationSpam`).
24. Unmark spam on closed conversation. Status stays closed (spam separate from status, data-model).
25. Conversation with `ai_state = active` closed by member. 03 closes; 07 listens to `conversation.closed` and stops agent. 03 never mutates `ai_state`.
26. Reply with `then: "snooze"` but snooze params invalid. Whole request 422; message not stored (single tx).
27. Reply `then: "close"` on conversation with pending outbound message. Close proceeds; outbound job still sends (closing never cancels sends).

## B. Assignment

28. Reply to unassigned conversation, inbox `assign_on_reply = true`. Assign to replier in same tx, `conversation.assigned` emitted with actor = replier, team untouched.
29. Note on unassigned conversation. No auto-assign.
30. Dimmi Agent reply on unassigned conversation. No auto-assign (agent is not member).
31. Token reply where token owner is member. Auto-assign to owner. OAuth client token acting without member (09 client credentials if any): no auto-assign.
32. Two members reply to unassigned conversation simultaneously. Both replies stored; auto-assign under row lock: first committer assigns, second sees assignee set and does not reassign.
33. Assign to deactivated member. 422 `member_inactive`.
34. Member deactivated with 2,000 assigned conversations. `UnassignMemberConversations` in batches of 500 per tx, team kept, one `conversation.assigned` per conversation (workspace realtime coalesced), timeline rows with actor system. Notifications to assignee suppressed (deactivated).
35. Team deleted. Conversations `assignee_team_id` set null (fk set null) plus action emits `conversation.assigned` per affected conversation (foundation rule "unassigned from team with event").
36. Assign member not in team, with team. Allowed; no membership check (Intercom parity).
37. Assign same member again. 200 no-op, no event.
38. Inbox default team deleted. `default_team_id` null (fk); new conversations get no team.

## C. Replies and outbound

39. Client retries reply after timeout with same `client_message_id`. Existing message returned 200 `Idempotent-Replayed: true`; no second send.
40. Two tabs send same text without `client_message_id`. Two messages (intentional; no content dedupe).
41. Reply to conversation where customer last wrote via email but conversation started on widget. Routed to email connection (reply follows customer); message `channel_connection_id` = email.
42. Routed connection disconnected (Slack app uninstalled). Fallback to origin if active else 409 `channel_disconnected`; nothing stored.
43. Connection disconnected after message stored but before job runs. Job marks `failed` `channel_disconnected` final; member sees error and retry button (retry re-validates, fails fast while disconnected).
44. WhatsApp window closed (last customer message 25 h ago). 409 `channel_window_closed` unless `channel_options.template` given (05 validates template).
45. Reply body longer than channel limit (Telegram, Discord). 422 with channel name and limit; never auto-split (splitting changes meaning of receipts).
46. Reply with 12 attachments. 422 (global 10). Channel lower limit: 422 naming channel.
47. Attachment of 30 MB to email (limit 25 MB total). 422 before insert.
48. Two replies sent 100 ms apart. Second job snoozes until first leaves `pending` (ordering guard), so customer sees order. First stuck failing: after 2 min second sends anyway.
49. Provider rate limit (429 with Retry-After). Job snoozed for Retry-After, attempts not consumed; message shows `pending`.
50. Provider timeout but message actually delivered. Retry sends with same `IdempotencyKey`; drivers pass to provider where supported (email: same Message-ID so client threads dedupe visually; Telegram/Discord no idempotency: possible duplicate, accepted, documented per driver).
51. Driver returns success without `ChannelMessageID` on push channel. Treated as transient error (contract violation logged at error); contract test forbids.
52. Message deleted by author while job pending. Job sees deleted: `failed` with `cancelled`, final, not billed (substantive reply never delivered). Note: billing activation may already have happened at `message.created`; 12 credits on final failure (D13).
53. `ErrConnectionBroken` from driver. Message failed, connection status `error` with `last_error`, `channel.errored` emitted; other pending messages on that connection fail through guard 3 after 10 min unless connection fixed.
54. Missing driver for channel type (community build without adapter). `failed` `channel_driver_missing`; `dimmi doctor` lists connections with missing drivers.
55. Reply to contact that has no identity on routed channel (email conversation contact lost email via erase). 422 `contact_unreachable_on_channel`.
56. Reply to blocked contact. Allowed (member deliberately writes); not auto-unblocked.
57. Reply on spam conversation. 409 `conversation_spam` (prevents emailing spammers, protects sender reputation).
58. Reply to merged source conversation. 409 `conversation_merged` with target id.
59. Reply on conversation whose workspace hit spend cap (cloud). 12 InTx listener `MeterMessageActivity` (on `message.created`, inside reply tx) returns `ErrActivationBlocked`; tx rolls back (no message, no auto-assign, no `then` close); 423 `spend_cap_reached` with `billing` block (12 rules: 20% buffer before block). Only replies that would open new billing window blocked; follow-up inside open window allowed. Notes still allowed (not billable). Inbound customer messages always captured, conversation stays waiting. Same 423 for member-authored conversation create. Dashboard keeps draft, shows banner with `billing.manage_url`.
60. Collision: Ada sends reply while Bob composing with `expected_last_message_id` old. Bob gets 409 `new_messages_since` with Ada's message; resend without guard to force.
61. Collision guard id references deleted message. Compare against latest non-deleted `type = message` id; deleted id never matches, so 409 shows current latest.
62. Reply with `body` empty and no attachments. 422 `body` required.
63. Reply containing raw HTML. Stored as Markdown text; rendering escapes (goldmark raw HTML disabled).
64. Outbound for `api` channel conversation. No job; `delivery_status = not_applicable`; developer gets `message.created` webhook.
65. Widget conversation reply. No job; `delivery_status = sent`; 01 updates `read` via widget read receipts; offline email fallback owned by 01. Member reading widget conversation emits `conversation.seen_by_team` `{conversation_id, seen_at}` to widget channel (no member id; name/avatar only with workspace `seen_receipt_shows_name`); `conversation.read` never reaches widget.

## D. Inbound

66. Provider redelivers same webhook. Dedupe by `dedupe_key`; `Duplicate = true`; adapter returns 200.
67. Two replicas process same inbound concurrently. Unique violation on `(workspace_id, dedupe_key)` caught; loser re-selects and returns duplicate.
68. Inbound with unknown thread key but `ReplyToChannelMessageID` matching our outbound message. Conversation of that message used (adapters set `ConversationID` via lookup helper `FindConversationByChannelMessage`).
69. Inbound on deleted (trash) conversation thread. New conversation with `previous_conversation_id`; trash untouched.
70. Inbound on merged source thread. Follows `merged_into_id` chain (max 5) to target.
71. Inbound for connection soft-deleted. Ignored with log; adapter returns 200 to stop provider retries.
72. Inbound body over 64 KB. Truncated at 64 KB rune boundary with marker "[message truncated, full text attached]", full text attached as `message.txt`.
73. Inbound with 0-byte body and attachments only. Allowed; subject excerpt = first attachment filename.
74. Inbound from sender with no identities and no ContactID. Adapter bug: action returns error `ErrSenderRequired`; adapter logs and acks (no infinite retries).
75. Inbound identity claim verified email already on other verified contact with different external_id. No merge, identity not attached to new contact, message goes to contact resolved by first matching identity (provider user id), duplicates surfaced.
76. Inbound attachment storage fails (S3 down). Action not called; adapter returns 5xx to provider (provider retries) or retries job (gateway/poller); no partial message.
77. Inbound burst 500 messages/s to one workspace. Number allocation row lock serializes conversation creation only (not messages on existing conversations); budget 200 new conversations/s/workspace; excess queues in adapter jobs, none lost.
78. Contact messages arrive out of order (provider timestamps reversed). Stored in arrival order (`created_at = clock.Now`), provider `SentAt` in metadata; timeline order = arrival (consistent with what members saw live).

## E. Note edit and delete

79. Edit sent reply. 422 `message_not_editable` (reply edit: later); member deletes and resends instead.
80. Delete sent Telegram reply. Local delete only (`message.deleted`); customer keeps copy in Telegram; no provider call (channel delete sync: later).
81. Member deleted from workspace; their notes. Notes stay, author renders "Former teammate"; only admins can delete them.
82. Admin redacts customer message containing password. Body cleared, attachments files deleted, `metadata.redacted_by`; search vector clears (generated column); channel copy untouched.
83. Delete last message of conversation. `last_message_id`, `last_message_at` recomputed to previous message; `message_count` decremented; list order may change (accepted).
84. Edit note adding new mention. New mentions get `message_mentions` rows, `message.updated.AddedMentionedMemberIDs` set, 06 notifies only added members; removed mentions rows deleted, no "unmention" notification.

## F. Attachments

85. Upload never linked. Purged after 24 h by `purge_orphan_attachments` (row + file).
86. Link attachment uploaded by other member. 422 unless actor admin.
87. Same file uploaded twice. Two rows (sha256 kept for future dedupe); no conflict.
88. File claims `image/png` but is HTML. Sniffed type stored; served as attachment download, never inline.
89. Image dimensions over decompression-bomb limit (foundation guard). Stored as file, `width/height` null, no thumbnail, `inline` forced false.
90. Signed URL expired in open dashboard tab. 403 from `signed`; client refetches message (fresh URLs every resource render).

## G. Read state and counts

91. Member authors reply or note. Author's read pointer moves to that message in same tx (own messages never unread).
92. Mark read with older message id. No-op (forward only).
93. Mark unread on conversation with only member's own messages. No-op; returns read state unchanged.
94. Counts cache stale after write on other replica. Counts cache lives in Valkey (shared by replicas); key includes per-workspace generation counter (Valkey `INCR`) bumped by `BustConversationCountsCache` after commit; worst case 5 s TTL. Valkey flushed: counts recomputed from Postgres on next read.
95. View count over 999. Returns 999 + `capped: true`; dashboard shows "999+".
96. Member with 10k unread conversations in "all". Unread computed only for mine/unassigned/mentions; "all" shows open count only.
97. Conversation moved to other inbox. Read state kept (per conversation, not per inbox).
98. Mention of member in note on conversation they never opened. Appears in Mentions view, unread until opened.

## H. Collision and realtime

99. Member subscribes `conversation:{conv}` of other workspace. Subscribe authorizer denies (centrifuge permission denied), generic error never leaks existence.
100. Member typing indicator stuck (tab closed mid-typing). Widget relay auto-expires 6 s (no further `typing-indicators` calls); channel typing expires on provider side.
101. Typing in note composer (`composer: note`). 202 no-op; never relayed to widget or channel, never shown to teammates.
102. Realtime event over 10 KB (long message). Broadcast `{id, object, updated_at}`; client fetches via REST (foundation).
103. Client receives own optimistic echo. Drops by `meta.client_mutation_id` (and by `client_message_id` for messages).
104. Reconnect after 15 min (history window 10 min exceeded). Centrifuge answers `recovered: false`; client refetches list and `messages?filter[updated_since]=` for open conversation. Within window: `recovered: true`, missed publications replayed from Valkey history.

## I. Contacts and merge

105. Identify with external_id already used by other contact (merged tombstone). Lookup follows `merged_into_id` to survivor and updates survivor.
106. Identify changes email of existing user. Email field updated, new email identity added verified, old email identity kept verified (was verified when used) so old email threads still route to same contact.
107. Identify with email of verified contact that has different external_id. New contact created with given external_id; same email on two contacts allowed (data-model: email not unique); email identity (unique) stays with first; new contact gets no email identity; `meta.possible_duplicates`.
108. Manual merge lead (unverified, widget visitor) into user. Conversations, participants, email/phone identities move; lead's `widget_visitor` identities deleted and its widget sessions revoked via `contact.merged` payload `revoke_widget_sessions: true` (01 listener) so browser that typed someone's email cannot read user's history.
109. Manual merge user into user (both with external_id). 409 `merge_conflict_external_id`; member must clear one external_id first via PATCH (allowed: PATCH `external_id: null` on contact only by admin).
110. Merge field precedence. Target values win; target nulls filled from source (name, phone, avatar, company, locale, timezone, location); custom attributes union, target wins per key, capped 100 keys (source extra keys dropped, listed in `contact_merges.moved.dropped_attributes`); `first_seen_at` min, `last_seen_at` max; `blocked_at` set if either blocked; `email_verified` OR.
111. Merge chain A into B, later B into C. Lookups of A resolve to C (chain depth max 5; deeper: 404 and log error; merge action flattens: when merging B into C also updates rows with `merged_into_id = B` to C).
112. Merge with 20k conversations. Single `UPDATE conversations SET contact_id` statement in tx (index-backed), plus participants dedupe; under 2 s; request timeout 30 s; above 50k conversations action runs as job and returns 202.
113. Concurrent merges A into B and B into A. Rows locked in UUID order; second sees its source already merged and target merged; returns 409 `contact_merged` naming survivor.
114. Merge conversations of contact where both open with same thread key? Impossible (unique among non-closed). Different thread keys: allowed.
115. Contact erased while conversation open in teammate dashboard. Contact renders "Deleted contact"; replies to that conversation 422 `contact_unreachable_on_channel` (identities wiped).
116. Erase contact who is participant in other contacts' conversations. Participant rows removed; messages they authored in those threads: bodies wiped too.
117. Custom attribute key `Plan` (uppercase). 422 (`[a-z0-9_]`); identify via API returns field error listing key. Value over 1024 chars: 422. Nested objects: 422.
118. Custom attributes with 101 keys after PATCH. 422 `custom_attributes` limit; no partial write.
119. Phone not E.164 (`(415) 555-0100`). API normalizes using workspace country default when parseable (libphonenumber port), else 422.
120. Email with uppercase and unicode domain. Stored lowercase (citext) and IDNA ASCII form for identity value; display keeps original.
121. Block contact with open conversations. Existing conversations untouched; new inbound to new conversations marked spam; existing conversations get messages flagged `spam_suspected`, no notifications (06 checks `ContactBlocked`).

## J. Conversation merge

122. Merge source with pending outbound message. Message moves to target; job still sends via message's routed connection (stored on message), no re-route.
123. Merge where target closed. Target stays closed; source messages appended; if source had unread customer message, target becomes unread (activity time recomputed).
124. Merge then customer replies on source email thread. Inbound follows `merged_into_id` to target; reply goes out on routed connection with target's thread key (customer may see new subject thread; documented).
125. Merge across inboxes. Target inbox wins.
126. Source has tags 45, target 10. Union capped at 50; overflow tags dropped from target set, listed in event payload.

## K. Tags and saved replies

127. Tag name collision differing in case (`Billing` vs `billing`). citext unique: returns existing tag (auto-create path) or 422 on explicit create.
128. Delete tag used on 80k conversations. Cascade delete of `conversation_tag` rows in one statement; single `tag.deleted` event (no per-conversation events); clients refetch lists.
129. Rename tag. `tag.updated`; conversations reference by id, no rewrite.
130. Tag name with emoji `🐛 bug`. Allowed, NFC normalized, 1 to 50 chars.
131. Shared and personal saved reply both `/refund`. Personal wins in palette list (`scope` sorted personal first); both valid.
132. Saved reply variable for missing attribute without fallback. Blank, listed in `missing_variables`.
133. Saved reply body with `{{` unbalanced. Left as literal text.
134. Owner of personal saved replies deactivated. Replies kept (invisible); deleted when member removed by cascade (fk on workspace_members delete cascade).

## L. Search

135. Query `"` alone or only operators. No text predicate; filtered list by recency.
136. Query in German with umlauts `Rückerstattung` vs `Ruckerstattung`. Match (unaccent); English stemmer on German words harmless (both sides same).
137. Query matches 100k messages. Candidate cap 2000 newest; `meta.search.capped = true`.
138. Search returns note hits for token with conversations:read. Notes searchable for members/tokens; widget never uses this endpoint.
139. Search hits on deleted message. Excluded (`deleted_at IS NULL`; body cleared anyway).
140. Search for conversation in trash. Excluded except `filter[view]=deleted` (admin).
141. Search statement times out. 503 `search_timeout` with `Retry-After: 1`.
142. Query contains SQL/tsquery syntax (`a & b | !c :*`). Parser treats as text tokens; lexemes bound as parameters; never syntax error 500.
143. CJK text. Default parser splits poorly; trigram fallback on subject/contact helps; builds on F57 fix for messages; no pg_bigm or ngram this round (extension not on every managed Postgres).

## M. Bulk, delete, purge

144. Bulk 100 ids with 3 from other workspace. Those 3 return `not_found` per id; others proceed.
145. Bulk `changes: {trashed: true}` by member. Whole request 403 (per-field permission checked once before per-id work).
146. Bulk request interrupted (client disconnect). Each id own tx; completed ones stay; client re-runs: already-applied ids return `skipped` (every field no-op).
147. Delete conversation with pending outbound. Soft delete keeps job; job sends (deleting conversation is not unsend). Purge after 30 days removes everything.
148. Purge job deletes conversation referenced as `previous_conversation_id`. fk set null.
149. Purge with attachments on disk. Files deleted after DB commit; file delete failure logged, retried by orphan file sweep in 11 (`dimmi doctor` reports orphans).

## N. Inboxes

150. Delete default inbox. 409 `cannot_delete_default_inbox`; change default first (`PATCH /inboxes/{id}` `default: true`).
151. Delete inbox with conversations and connections without `move_to`. 409 `inbox_not_empty`. With `move_to`: conversations and channel connections moved in batches (job when over 5,000), inbox soft-deleted, `inbox.deleted`.
152. Inbox name duplicate. Allowed (no unique); dashboard disambiguates by color.

## O. Permissions mid-flight

153. Member demoted from admin to member during bulk delete. Role checked at request start; in-flight request finishes (sub-second); next request denied.
154. Token ability `conversations:read` tries reply. 403 `missing_ability` `required_ability: conversations:write`.
155. Token owner deactivated. Token rejected 401 by auth (00-accounts); queued jobs created earlier still run (actor recorded).
156. Member removed while subscribed to `conversation:{conv}`. Disconnected within 5 s (`node.Disconnect(member)`, foundation); presence entry removed.

## P. Metering

157. Member replies on own test conversation (contact = member's verified email). `is_test = true`; never billable.
158. Developer creates contact-authored conversation via API then replies via API. Billable like D1 (API same rule, D11).
159. Member-started outbound email, customer never answers. Never billable (D7).
160. Dimmi Agent acknowledgement-only message ("Got it, a human will follow up"). 07 passes `Substantive = false`; no activation.
161. `initiated_by` on `conversation.created`: inbound/contact API `contact`; dashboard or member token outbound `member`; agent proactive `agent`; integration token without member `api`. 12 fair-use guard (InTx) counts `member|agent|api` only; 429 `outbound_fair_use_exceeded` rolls back create.
162. Billing listener error other than block (DB timeout in 12 tx work). Reply tx aborts, 500 `internal_error`, client retries with same `client_message_id`; inbound path unaffected (12 never gates contact-authored).
163. Mark spam 20 days after activation. Event emitted; 12 decides no credit (outside 14 days).
164. Unmark spam. `conversation.unmarked_spam`; 12 re-bills (D9 "stay billable if un-marked").
165. Merge two conversations of same contact 3 days apart. `conversation.merged` carries both ids; 12 credits one (D12).
166. Reply fails delivery permanently, no other reply delivered. `message.delivery_updated` `final = true`; 12 credits (D13).

## Q. Unicode, timezones, numbering

167. Body with RTL and emoji, ZWJ sequences. Stored NFC; excerpt cuts on grapheme boundary (never splits emoji).
168. Subject excerpt of message starting with Markdown image only. Excerpt uses alt text, else "Attachment".
169. Number allocation tx rolls back after allocation. Number rollback too (same tx); no gaps from failures.
170. Conversation number past 2^31. bigint; JSON number safe below 2^53.
171. Workspace timezone change. Future snoozes keep absolute `snoozed_until`; only new preset calculations change.
172. Clock skew between replicas. All business time from `framework/clock` (`a.Clock`, `postgres` source); ordering uses `(created_at, id)`.

## R. Migrations, self-host vs cloud, abuse

173. Upgrade adding indexes on large self-host DB. Indexes created `CONCURRENTLY` outside tx (goose NO TRANSACTION); boot not blocked beyond advisory lock holder; `dimmi migrate:status` shows progress.
174. `btree_gin` extension unavailable (exotic managed Postgres). `dimmi doctor` fails with clear message; boot fails at migration with "required extension btree_gin missing" (contrib extension available on RDS, Cloud SQL, Azure, Supabase, Neon).
175. Redefining `messages.search` generated column on existing data (if foundation shipped first with data). Migration adds new column `search_v2` generated, builds index concurrently, swaps in later release (expand, migrate, contract).
176. Self-host single node. Valkey still required (counts cache, presence, realtime broker, pubsub); same behavior as multi-replica.
177. Cloud RLS. Every query in jobs (`unsnooze_conversation`, purge) sets `app.workspace_id` per workspace batch before touching tenant rows; cross-tenant maintenance selects (orphan attachments, snooze reconcile list of workspaces) run as maintenance role documented in 11/foundation.
178. Spam wave: 10k new widget conversations from bots. Spam heuristics flag; spam conversations hidden from default views, no notifications, no AI; member bulk-marks remainder spam by filter (`filter: {channel: "widget", created_after}`, `changes: {spam: true}`, `confirm_count`, up to 10,000 as job, workbench.md).
179. Abuse: API client loops creating conversations. `throttle:api` 600/min/token; outbound fair-use (12) for member-started outbound.
180. Abuse: huge `filter[tag]` list. Max 5 tag ids (400 `invalid_filter`).
181. Abuse: deep cursor pagination of all conversations to scrape. Allowed for authorized token (own data); rate limits apply.

## S. PATCH field dispatch, bulk, new endpoints

182. PATCH `{assignee_id: "mem_bob", status: "closed"}` on open conversation assigned to Ada. Assign applied first, then close (fixed order, api.md "Order of application"); events `conversation.assigned` then `conversation.closed`, two timeline rows; `meta.changed: ["assignee_id", "status"]`.
183. Same PATCH with JSON keys in other order (`status` before `assignee_id`). Same result; JSON key order never matters.
184. PATCH `{status: "closed", assignee_id: "mem_inactive"}`. Assignment fails 422 `member_inactive`; whole tx rolled back, conversation stays open, no events.
185. PATCH `{assignee_id: "mem_ada"}` where Ada already assignee plus `{priority: "high"}` change. Assignment no-op (no event); only `conversation.priority_changed`; `meta.changed: ["priority"]`.
186. PATCH `{snoozed_until: "…"}` without `status`. 422 field error `snoozed_until` (side param needs `status: snoozed`). Both `snoozed_until` and `snooze_preset`: 422.
187. PATCH `{status: "snoozed", snoozed_until: X}` on conversation already snoozed until X. No-op. Different time: re-snooze, new `conversation.snoozed`, old job no-ops (case 9).
188. PATCH `{trashed: true, status: "closed"}`. 422 field error `trashed` ("must be sent alone"); nothing changes. `{trashed: false, status: "open"}`: same 422. `{trashed: false}` alone restores.
189. Member PATCH any field on trashed conversation. 404 (members never see trash). Admin PATCH `{priority: "high"}` on trashed: 409 `conversation_trashed`.
190. Member PATCH `{priority: "high", trashed: true}` (mixed allowed + forbidden). 403 for whole request; per-field policy runs before any work.
191. Token with `conversations:write` but no `contacts:write` sends `{spam: true, block_contact: true}`. 403 `missing_ability` `required_ability: contacts:write`; spam not set either.
192. PATCH `{status: "open", ai_state: "active"}` on closed conversation with `ai_state = off` after close. Reopen first, then `ai_state` step: `off -> active` 422 `invalid_transition` (AI activates only via gate); whole PATCH rolled back. `{status: "open", ai_state: "handed_off"}`: reopen + pause (reason `member_paused`), both applied.
193. PATCH `{ai_state: "active"}` on spam conversation in `handed_off`. 409 `ai_not_active` (07 rule).
194. PATCH `{ai_state: "off"}`. 422 validation (not settable; `off` only via close listener).
195. PATCH any field on merged source. 409 `conversation_merged` with `data.merged_into`.
196. PATCH `{add_tags: ["billing"], remove_tags: ["tag_billing"]}` where `billing` resolves to `tag_billing`. Add applied, then remove: tag absent at end; events `conversation.tagged` then `conversation.untagged`. Documented, not rejected.
197. PATCH `{unread: true}` by token with only `conversations:read`. 403 `missing_ability` (PATCH route needs write); use `PATCH …/read-state {unread: true}` (read ability).
198. Bulk ids with duplicates. Deduped; one result per unique id.
199. Bulk `changes: {}` or `changes` with unknown field. 422 before any conversation touched.
200. Bulk filter without `confirm_count`. 422 `confirmation_required`, `meta.matched: 412`; nothing changes. Resend with 412 while 3 new matching conversations arrived: 409 `bulk_count_mismatch` `meta.matched: 415`; nothing changes; client confirms again.
201. Bulk filter matching 15,000. 422 `bulk_too_many` `meta.matched: 15000`. Client narrows filter (e.g. `created_before`) or runs slices. Matching 1,500: confirmation, then 202 job.
202. Bulk filter containing `q`. 400 `invalid_filter` (ranking not stable enough to confirm count).
203. Bulk filter snapshot: conversation in snapshot closed by teammate mid-run while changes `{status: "closed"}`. That id `skipped`. Conversation trashed mid-run: `failed` `not_found` (listed in `meta.errors`).
204. Merge `POST /conversations/T/merges {from: [A, B]}` where B already merged into C. 409 `conversation_merged`; A not merged either (all-or-nothing).
205. Merge `from` contains target T. 422 `merge_self`. `from` contains A already merged into T (retry after timeout): A in `skipped_ids`, 201.
206. Two members merge A into T and T into A concurrently. Rows locked in UUID order; second sees target or source merged: 409 `conversation_merged`.
207. Read-state `PATCH {last_read_message_id: "msg_x", unread: true}`. 422 (exactly one). `msg_x` from other conversation: 422 `message_not_in_conversation`. Deleted message id: pointer moves to it (forward only), counts recomputed.
208. Typing indicator from token on email conversation (no `Typing` capability, not widget). 202 no-op; no relay, no channel job.
209. Reply body carries `draft_id` or `allow_stale` (old client, 09 drafts later: reply-drafts). 422 `unknown_field`; nothing sent.
210. Reply on imported conversation (16, connection type `import`), inbox has active email connection, contact has email. `ChannelRehomer.Rehome` in reply tx moves conversation to email connection (`conversation.updated`), reply sent per 04 rehome rules (no `In-Reply-To`, subject `Re: <subject>`); later routing follows new connection.
211. Same as 210 but inbox has no email connection, or contact has no email. 422 `import_channel_cannot_send`; nothing stored, conversation keeps `import` connection. Note still allowed.
212. Imported conversation with 300 imported messages appears in member lists. Not unread for anyone (imported messages excluded); no notification, no `message.created`, no outbound job, no auto-assign, `waiting_since` null; `filter[imported]=true` lists it.
213. `POST /contacts?upsert=true` with neither `external_id` nor `email`. 422 `upsert_key_required`. `POST /contacts` (no upsert) with existing `external_id`: 409 `contact_exists`.
214. `POST /companies?upsert=true` with `domain: "WWW.Acme.dev"`. Normalized `acme.dev`, matches existing company; 200 `meta.upserted: "updated"`.
215. `PATCH /contacts/{id} {blocked: true, name: "Bob"}`. `contact.updated` then `contact.blocked`, one tx. Already blocked: only `contact.updated`.
216. `POST /contacts/{id}/exports` twice. Second 409 `export_in_progress` while first pending/running.

## T. Extensibility seams (`00-foundation/extensibility.md`)

217. `RetentionGuard` (fake) returns false for conversation in `purge_deleted_conversations`. Row skipped, stays in trash, `dimmi_retention_held_total{kind="conversation"}` +1, log `retention_held`; rest of batch purged. Next daily run asks again.
218. Guard false for spam conversation in `purge_spam_conversations`. Not trashed; stays in spam view. Guard false for orphan attachment: file and row kept.
219. Guard false for contact in `erase_contact`. Nothing wiped, no eraser hook runs, contact stays hidden (`deleted_at` set in request tx), audit `contact_erasure` status `pending_hold`. `GET /contacts/{id}` 404 as for erased. Repeated `DELETE ?erase=true`: 202 same `pending_hold` body, no second job while held.
220. Guard returns error (later driver down). Job retries (River backoff); never treats error as allow.
221. `expire_contact_exports` with guard false for contact. Zip still expires (derived copy, not guarded).
222. Member without `conversations:delete` (later custom role) lists `filter[view]=deleted`. 403 `forbidden`. Same caller fetching trashed conversation by id: 404.
223. Policy code compares role name. Review/lint fails (`Role ==` outside `roles.go`); 03 policies call `authz.Can` only.
224. Token owned by admin with ability `conversations:write` deletes conversation. Allowed: effective permissions = role permissions (has `conversations:delete`) intersect ability group `conversations:write` (contains `conversations:delete`). Same token owned by member: 403 `forbidden` (role lacks permission).
225. Restricted `VisibleScope`: member searches word present only in hidden conversation. Phase 2 drops candidate; result empty, `meta.search.capped` unaffected; `#1042` jump to hidden conversation returns no jump. Test asserts every list/search/count/neighbor/realtime authorizer calls `VisibleScope` (restricted member hides it everywhere).
226. Realtime: member subscribes `conversation:{conv}` outside `VisibleScope`. Subscribe denied; no presence entry.
227. Inbound identity with type not in registry (05 driver unregistered after upgrade, old rows exist). Existing rows render (`type` shown raw); new writes of that type rejected by action (`ErrUnknownIdentityType`, logged), inbound message stored without new identity row.
228. Contact with company via JWT `company` claim, then `POST /companies?upsert=true` + contact `company` field with different company. `company_id` (primary) replaced; old company keeps no link (single primary at MVP). Webhook `contact.updated` `changes.company_id` shows swap.
229. `POST /conversations/{id}/replies {body, reopen: true}` (MCP arg leaked to REST). 422 `unknown_field` `reopen`; nothing sent. Correct REST field `then: "open"`.

## U. Realtime on centrifuge + Valkey

230. Two tabs of same member open same conversation. Presence keyed by client; member shown once (dedupe by `user` in dashboard); one tab closes, member still listed.
231. Valkey restart. Presence and history lost; clients reconnect, `recovered: false`, refetch via REST; presence rebuilt by reconnecting clients within seconds; typing throttle keys gone (one extra typing event at most). Postgres data untouched.
232. Typing REST spam from one member (10 calls/s). Valkey `SET NX EX` 2 s throttle: one relay per 2 s; extra calls 202 no-op.
233. Typing from token on conversation outside `VisibleScope`. 404 (same as any conversation read).

## V. Cross-spec seams (13, 14, 15)

234. `SendSystemMessage` (15 CSAT) on closed conversation. Message inserted, outbound runs; conversation stays closed (never reopens), `waiting_since`, `last_message_at`, `last_activity_at`, unread counts, assignment unchanged; 07 not triggered; `message.created` emitted with `AuthorType = system`, `Substantive = false` (never billable).
235. `SendSystemMessage` metadata with key outside allowlist (`survey_request`). Key dropped, logged warn; message still sent.
236. `SendSystemMessage` on spam or trashed conversation. Returns `ErrConversationUnavailable`; caller (15) marks request skipped.
237. Contact replies in widget to CSAT system message. Normal contact message rules (may reopen within window); 15 reads response via its own routes, not message body.
238. Conversation started from announcement (`source.announcement_id`, 15). First row is event `announcement.replied` (does not change counters), then contact message; inbox shows announcement link.
239. Verified email claim (14 portal sign-in, 04 DMARC-aligned mail) for address held as unverified identity by widget lead L. Row moves to claimant C, verified; L keeps `contacts.email` text; `contact.updated` on both; L's conversations stay with L (no merge).
240. Same as 239 but row held verified by other contact D. Never takeover; auto-merge rules apply (both verified; external_id conflict blocks).
241. Unverified claim for address held unverified by L. No takeover, no identity row for new contact (existing rule); duplicates finder surfaces pair.
242. Widget upload linked to 14 feedback post after 23 h. Orphan purge (24 h) skips row (feedback target set); never linked in 24 h: purged.
243. `LinkAttachments` with attachment already linked to feedback comment. 409 `attachment_already_attached` (one-target CHECK).
244. `GET /contacts?filter[audience]=adn_...` with heavy audience. Statement timeout 5 s: 422 `audience_too_slow`; other filters combine with AND; audience of other workspace 400 `invalid_filter`.
245. `include=csat` on conversation with two completed CSAT responses. Latest by `completed_at`; none: `csat: null`.
246. `include=feedback_requests` when 14 tables empty or feature off. `[]`, one cheap query.
247. Contact JWT changes `custom_attributes.plan` only. `contact.updated` `changes.custom_attributes` + `changed_attribute_keys: ["plan"]`; unchanged keys absent; nothing changed: no event.
248. Contact erase with 13/14/15 data. Hooks `EraseChangelogContactData`, `EraseFeedbackContactData`, `EraseEngageContactData` run after core wipe; one failing hook retries job, erasure not `completed`; CI registry test fails if any of these specs lacks hook.
249. Contact export with feedback + engage data. Zip has `feedback` and `engage` sections; 13 hook optional (`changelog` section when registered); hook error marks export `failed`.
250. Tag deleted that is used on feedback posts (14). Pivot rows cascade; `tag.deleted` carries conversation count and optional `feedback_post_count`.

## W. Agent actions timeline (07) and importers (16)

251. 07 calls `inbox.RecordTimelineEvent` with name not in `messages.event` list. `ErrUnknownTimelineEvent`, tx rolls back (caller bug, test catches); never stored.
252. `inbox.SetMessageMetadataKey(msg, "ai", ...)` on deleted message. Writes anyway (card history), no `message.updated`, no realtime from 03 (07 publishes `ai.action_call_updated`). Key outside allowlist (`email`): `ErrMetadataKeyNotOwned`.
253. Timeline row `ai.action_call_approval_requested` on closed conversation. Inserted; counters, `last_message_at`, unread untouched (event row rule); conversation stays closed.
254. Token `POST /contacts?upsert=true` with `custom_attributes.stripe_customer_id`. Value set, key in `trusted_attribute_keys`. Later widget lead write cannot happen (contact `kind = user`: 403 `identity_locked`).
255. Merge lead L (attribute `plan` untrusted) into user U lacking `plan`. U gets `plan`, stays untrusted (source trust kept). U already has trusted `plan`: U value and trust kept.
256. `ImportMessages` re-run with same rows (resume after crash). `ON CONFLICT (workspace_id, dedupe_key) DO NOTHING`: no duplicates, counters recomputed once at end of batch.
257. `ImportContacts` row matches two contacts by unverified email. No match (needs exactly one); new contact created, duplicates finder surfaces pair.
258. Imported conversation number block: two imports in parallel in same workspace. Each `UPDATE workspaces ... RETURNING` allocates contiguous block under row lock; live conversations keep allocating after; no gaps reused.
259. Imported message with `author_type = member`, `author_id` null (former teammate). Allowed only with `metadata.imported`; renders `former_teammate` author. Same insert without `metadata.imported`: CHECK/validation error.
260. Search `imported:4512`. Exact match on `source.imported.external_number`; unknown: empty result (no error).
261. `SendChannelOutbound` receives `message.created` for message with `metadata.imported` (defense; 16 never dispatches). No job inserted, warn log.
262. Merge live conversation (source) into imported target. 422 `merge_imported_target`; merge imported into live target allowed (imported messages move, keep `metadata.imported`).

## X. Conversation access (access.md)

263. Member `teams` mode, conversation assigned to them on hidden team. Visible (own assignment). Reassigned away: `conversation.hidden`, subscription dropped, open tab shows no-access page.
264. Member `assigned` mode mentioned in note on conversation they cannot see. Row written, no notification, not in Mentions. Later assigned: appears unread.
265. Restricted member opens `#1042` by number. 404. Search `#1042`: no jump.
266. Restricted member uses `filter[contact]` for customer with conversations on hidden team. Only visible rows; `total_estimate` counts visible only.
267. Access changed while tab open with composer draft. Socket disconnect, refetch; hidden conversation page. Draft kept on server, reachable if access returns.
268. Team in member's list deleted. Rows cascade. `teams` mode narrows, `all_except_teams` widens. No notice beyond audit of team delete.
269. Restricted member holds `conversations:delete`, opens trash. Only visible rows.
270. Token of restricted member lists conversations. Same scope; integration token with no member sees all.
271. Bulk by filter from restricted admin, access reduced while job runs. Rechecked per id: `failed` `not_found`.
272. Admin tries changing own access. 403. Promoting restricted member to owner: access reset `all`, audit row.
273. Restricted member on `workspace_conversations` channel. Authorizer denies, client uses `member:` copies. Client subscribes by capability from `/me` `conversation_access`.
274. Restricted member count in workspace large (200). Fan-out per event loops cached list; budget 5 ms for 200; metric. Above that unchanged (linear, fine at this scale).

## Y. Assignment and availability (assignment.md)

275. Round robin arrival, all pool members away, `include_away` false. Waits with `no_available_member`; team set, member null, explanation in panel. Someone back online: drain assigns.
276. Pool member deactivated between pick and commit. Cursor lock + active check inside tx; none lost, next eligible picked.
277. Two arrivals same millisecond. Cursor row lock serializes; different members.
278. Member at capacity of 10, closes one. Close triggers drain; oldest waiting assigned.
279. Team-only assign by member on inbox with `manual`. No pick; stays team.
280. Assign to team on `balanced` inbox when conversation `ai_state = active`. Skipped until `ai.handoff_requested`.
281. Conversation assigned manually while `assign_on_arrival` queued. Re-check member null under lock: no-op.
282. Assignee `away_reassign` true, customer replies. Member cleared, team kept, event; mode manual: visible in team or Unassigned. Second reply no further change.
283. Away member set back online. Reassigned conversations do not return.
284. Member sets away with reason required and no reason. 422 `away_reason_required`. Auto-away: allowed without.
285. Auto-away member keeps tab idle, returns, moves mouse. Activity call flips online since source `auto`. Manual away: stays away.
286. Valkey flush, auto-away enabled. Missing key: skip, nobody marked away.
287. `assign_on_reply: always` member reopens unassigned conversation. Assigned. `never` member replies with inbox rule on: stays unassigned.
288. Inbox `overflow_team_id` team has nobody eligible too. Waits with reason of primary.
289. Pool empty (team has no members). `no_available_member`; warning chip on inbox settings.
290. Inbox deleted with waiting conversations. `move_to` inbox rules apply next drain.
291. Workflow claim present on conversation (17). `AssignOnArrival` no-op; 17 run end calls it when member still null.

## Z. Views, sort, priority (views.md)

292. Custom view uses deleted tag. Condition matches nothing, warning on row, editor flags. View still lists by other groups.
293. Personal view of removed member. Cascades.
294. Definition with text condition plus sort `waiting_since`. Allowed, candidate cap applies; `meta.search.capped`.
295. Edit view while another tab paginates. Cursor hash differs: 400 `cursor_mismatch`, client restarts page.
296. Sort `waiting_since` with rows null. Nulls last; keyset handles null branch separately. Filter `waiting` hides them.
297. Sort `snoozed_until` on open status. 400 `invalid_sort`.
298. PATCH `priority: true` from old client. 422 validation listing levels. `filter[priority]=true`: 400 `invalid_filter`.
299. Status filter Closed + search for number. Number jump ignores status.
300. Presence lookup slow. `viewers` omitted; list returns.
301. Setting `inbox.show_viewers_on_list` false. No `viewers` include honored, no `conversation.viewers_changed`.
302. Preferences blob over 64 KB. 422; client trims columns.
303. Folder references team now hidden by access. Skipped at render, not pruned.

## AA. Workbench (workbench.md)

304. `J` at end of list. No move, soft bump. List empty: no-op.
305. Shortcut typed in composer. Plain letters go to composer; `allowInInput` ones act.
306. Cmd+K while modal open. Ignored; closes modal first on second press.
307. Snooze option key deleted between open and click. 422 `snooze_option_unknown`, picker reloads.
308. Team list present but conversation has no team. Workspace list, else built-ins.
309. Note by assignee on snoozed. Never wakes. Note by other: wakes only with `snooze.wake_on_others_note`.
310. Draft saved on phone, older tab saves. Older `client_updated_at` ignored, 200 with newer; tab updates when clean, else shows "Newer draft on another device" with Use and Keep.
311. Local storage blocked. Server path only; nothing throws.
312. Draft attachment past 24 h. Kept (orphan purge skips draft ids). Draft purged after 30 days: then purge removes.
313. Reply sent; draft save in flight arrives after. Client aborts pending save with `AbortController` on send. Drafts keep no tombstone; rare leftover reappears as draft with same text, teammate discards. Accepted.
314. Conversation merged while draft exists. Draft moves to target unless target already has same mode: then stays orphaned on source hidden, purged by age.
315. Undo after job already started. Lock shows `scheduled_send_at` cleared: normal delete path, toast "Already sent".
316. Undo after teammate reassigned meanwhile. Reversal skips assignee (current differs), restores rest.
317. Undo with `then: close`, teammate reopened. Status left as is.
318. Two replies, first undone. Second job ordering guard sees first `undone_at`: no wait.
319. Undo window and restart of server. River job persists; sends at ScheduledAt.
320. Member override 0. No window on their email replies.
321. Reply to widget conversation with delay true. Delay ignored, 0.
322. Tiptap fails to load chunk. Falls back to plain textarea shell: Markdown typed; send works.
323. Markdown round trip fails to preserve (raw HTML pasted). Pasted HTML reduced to subset, never stored as HTML.
324. Mention token in reply body. Treated as text, no rows.
325. Inline image scan infected after send. Bubble shows removed; email already sent only if scan passed (send waits).
326. Giphy down or key revoked. Picker shows error, no other effect.
327. GIF remote image when key removed later. Old message renders link text.
328. Delete message with active side effects: last message. Counters per edge 83; raw purge via hook.
329. Hook fails repeatedly (04 store down). Job retries; body already cleared; metric alerts.
330. New conversation to address with active hard bounce. 04 send fails with reason; conversation exists with failed message and retry.
331. Send separately with one failure among 5. Results list; four created; retry just failed.
332. Bulk 5,000 assign while restricted member watches. Fan-out coalesced per conversation; `bulk_operation.updated` to requester only.
333. Second bulk job same workspace. Queued; `status: pending` until first ends.
334. Job cancelled mid chunk. Finishes current id; rest untouched; counts true.
335. Bulk reply hits billing block at conversation 40. That id `failed` `spend_cap_reached`; continues (others may be inside open windows).
336. Bulk reply `then: close` with required attributes missing on some. Those `failed` `required_attributes_missing`, no message stored for them.
337. Seen on email. Never shown. Delivered status only.

## AB. Records (records.md)

338. Merge conversations different contacts, setting off. 422 `merge_different_contacts`. On: allowed, source contact `cc` if email known.
339. Setting turned off after merges. Past merges stay.
340. Merge target holds 48 participants, source 5. 422 `participant_limit_reached`, nothing merged.
341. Opening merged source. Client redirects to target, banner. API 200 with `merged_into`.
342. Contact merge both verified different email, setting off. 422. Lead into user: allowed. Different `external_id`: 409 whatever setting.
343. Duplicate suggestion dismissed. No stored dismissal this round: banner stays until merged or identifiers differ. Why: small state, low annoyance, one line text.
344. Bcc address equals To participant. To wins; removed from Bcc, no double mail.
345. Bcc on chat channel. 422 `channel_participants_unsupported`.
346. Teammate reads thread later. Sees event row with Bcc list, not on message resource. API token same.
347. Remove primary contact as participant. 422: primary is implicit. Change contact not in scope.
348. Inbound from removed participant. Stored, flagged `from_non_participant`, no re-add.
349. Participant email suppressed. 04 per-recipient failure; other recipients delivered; warning on message.
350. Attribute required but definition team-limited, conversation on other team. Not required.
351. Automation closes with missing required attribute. Allowed.
352. Attribute type change attempt. 422 immutable; archive and recreate.
353. List option deleted with values in use. Option `archived`, values render label; new writes refused.
354. Attribute sort on 100k rows unindexed. Under 2 s or 503 `list_timeout`; UI suggests filter.
355. Tag on contact via API by token with `contacts:write`. Allowed; auto-create tag by name.
356. E-05: panel lift by member without permission. Button hidden; API 403.
357. Company with 50k contacts. Paginated; panel shows count capped 999+.
358. Export contact CSV with 200k rows. Over limit 422 `export_too_large`.
359. Directory row for erasure-pending contact. Hidden; `GET` 404.

## AC. Safety (safety.md)

360. Scanner offline. Status `pending` until retries end, then `inconclusive`; teammates can grant, customers cannot. Metric alerts.
361. Download in `pending`. 409 `attachment_scan_pending`; UI spinner, `attachment.scanned` refreshes.
362. File large for clamd (unscanned). `inconclusive`, warning flow.
363. GuardDuty `ErrPending` over 15 min. `inconclusive`.
364. Name says `.pdf`, bytes are zip. `mismatch`, warning. Bytes HTML: edge 88 applies too, never inline.
365. Infected attachment on already sent email. Sent only after clear, so case is inbound or later rule; bytes removed, row kept, message stands.
366. Outbound over `scan_wait_max` with `pending`. Send fails `attachment_not_cleared`, retry available.
367. `customer_uploads` false, widget mid-upload. API 403 `uploads_disabled`; widget notice (01).
368. Extra extension in hard block (`exe`). 422 on save.
369. Signed URL after delete. 404. After erase 404. Infected 410.
370. Card inside email subject. Redacted in subject too; thread key unaffected (uses headers).
371. Redaction on number passing Luhn but is order id. Stored masked, irreversible; admin can disable rule. False positive accepted over storing cards.
372. Custom regex matches everything. Allowed; `POST /redaction-tests` exists; change audited.
373. Regex invalid or lookaround (RE2 refuses). 422 with compile message.
374. Rule hit on inbound email. Raw dropped, `raw_email_purged` for view original; 04 reprocess impossible for that mail, stated.
375. Attachment with card inside. Not scanned. Settings copy says so.
376. Redaction at reply: teammate pastes card to customer. Masked before channel; teammate sees masked bubble, badge.
377. Blocked domain in teammate reply. Not checked; policy is for reading customer links.
378. Strip links on, chat messages. Email only.
379. Erase requested, customer writes in grace. New lead, new conversation. Cancel erase: address conflict listed `identities_conflicted`, teammate merges.
380. Erase twice. 202 same row.
381. Restricted member erases. 403 `conversation_access_required`.
382. Retention job meets `RetentionGuard` false. Skips row, metric.
383. Retention turned on with 200k rows. Batched 500 per tx, daily job loops until done, per-run cap 50,000.
384. Restore with thread conflict. 409; merge or close other.
385. Restore after contact erased. Contact tombstone; row still restorable, reply 422 unreachable.
386. Export includes hidden conversations for restricted requester. Never: scope at job time; bulk selection ids silently dropped, `warnings` counts them.
387. PDF with Arabic or CJK. `missing_glyphs` warning; txt suggested.
388. Scheduled export destination credentials revoked. Two fails: `error`, `system_alert`, schedule pauses; test button clears.
389. Marked not spam, trust offered, sender is free mail domain. Trust exact address only (04 rule).
390. Bulk not spam 3,000. Job; `changes.spam: false`.

## AD. Collaboration (collaboration.md)

391. Team mention with team mentions off. Plain text, no rows.
392. Mention deactivated member. Typeahead omits; manual token: plain text.
393. Mention on closed conversation. Allowed; notification normal; view lists.
394. Reaction on deleted note. 404.
395. Reaction by member later removed. Cascades.
396. Side conversation reply from address not on it. Stored, flagged; never reaches customer.
397. Customer replies to forwarded side email chain. Routed by thread ref to side; stored there; teammate sees. Never appears in main thread automatically.
398. Parent closed, side reply arrives. Parent stays closed; unread mark, notification.
399. Bounced side mail. `delivery_status` failed, reason; no customer effect.
400. Similar empty or timed out. Section hidden.
401. Translation with no key. 422 `translation_unavailable`, controls hidden.
402. Translate message later redacted or deleted. Row cascades; cached copy gone.
403. Customer text with instruction "ignore previous". Treated as data, delimited; output only translation; never drives tools. Result shown as untrusted text.
404. Spend cap reached. 07 refuses: 502 `translation_failed` reason; original shown.
405. Reply `original` without translation enabled. 422 `translation_unavailable`.
