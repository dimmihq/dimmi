# 06 Notifications: edge cases

Style: caveman. Each: case. Decided behavior.

## Recipients and triggers

1. Solo workspace, owner is only member, owner replies via dashboard then customer replies. `customer_replied` to owner if assigned; unassigned conversation reopened by customer goes to audience (owner). Owner never notified of own reply.
2. Member assigns conversation to self. Actor equals assignee: no `conversation_assigned` notification.
3. Agent token (07) or API token assigns conversation to member. Actor type `agent`/`token`, member notified (actor not a member, nothing to exclude). If token owned by same user being assigned: excluded (actor resolves to that member).
4. Conversation created already assigned (rule or API sets assignee in create tx). `new_conversation` skipped (assignee set); `conversation_assigned` fires from `conversation.assigned` event if emitted, else create handler treats create-with-assignee as assignment to that member.
5. Conversation reassigned A to B to A within seconds. Each event produces its own notification; at send `condition_changed` check (assignee still recipient) suppresses stale push to B. In-app rows kept as history.
6. Unassigned (`to.member = null`). No notification. Previous assignee not notified (MVP).
7. Team routing: conversation moved from team X to team Y with no member. `new_conversation` to Y audience. X members get nothing.
8. Team with zero active members. Audience falls back to inbox default team, then all active members. Never empty unless workspace has zero active members (then nothing).
9. Mention of deactivated or removed member. Skipped silently. Mention of member in other workspace (forged id). 03 validates; 06 ignores ids not active in workspace.
10. Note mentions same member twice. One notification (dedupe recipients).
11. Note edited (`message.updated`) to add new mention. 03 inserts new `message_mentions` rows and sets `AddedMentionedMemberIDs`; 06 notifies only added members (event id differs, so fan-out unique key allows it). Removed mentions: no notification, unread in-app row kept.
12. Customer sends 10 messages in 20 s. In-app: one row, `count=10`. Push: first sent, rest `cooldown` within 30 s; 31st second next message pushes "Bob sent 4 messages". Email: one delayed email at 120 s, only if unread.
13. Customer reply while AI active (`ai_state='active'`) and member `include_ai_handled=false`. Skipped (`condition_changed`); handoff later fires `ai_handoff_requested`.
14. AI run fails mid-conversation without handoff event. 07 must emit `ai.handoff_requested` on failure (dependency). If 07 sets `ai_state='off'` without event, next customer message triggers `customer_replied` normally.
15. 07 sets `ai_state='active'` after `conversation.created` tx commits (race). Dispatch reads state at fan-out (not event time); if still `off` at fan-out, members notified. Dependency asks 07 to set state in same tx.
16. `snooze_expired` for conversation closed meanwhile. Re-check status: not open, `condition_changed`.
17. `conversation.unsnoozed` reason `customer_message`. Not `snooze_expired`; `customer_replied` covers it.
18. Spam conversation. No notifications. Marked spam after notification sent: unread in-app rows deleted, pending delayed emails canceled via `conversation_gone`.
19. Blocked contact writes. `blocked_at` set: no notifications (foundation rule).
20. Conversation in `test_mode` workspace. Notifications sent normally (testing notifications is the point). Push title not prefixed.
21. Conversation deleted after notification row created. FK cascade deletes notification; pending jobs find no row, exit `skipped`.
22. Contact merged. Unread notification data rewritten async to new name; links use conversation id (unchanged by contact merge).
23. Conversation merge (03, later): source conversation gone; notifications cascade; target conversation notifications unaffected.
24. Member role changes admin to member mid-flight. Trigger notifications unaffected; `system_alert` recipients re-resolved at send (demoted admin skipped).
25. Member removed while job pending. `member_inactive` at send. Realtime sockets kicked by foundation.
26. User belongs to two workspaces, both notify. Separate rows, separate preferences; push titles prefixed `[Workspace]`.

## Preferences and settings

27. PATCH preference to same as default. Override row deleted, `source: default`.
28. PATCH unknown trigger or channel. 422 with field path `preferences.0.trigger`.
29. Disable locked cell. 422 `preference_locked`, other cells in same request not applied (all-or-nothing tx).
30. Release changes default. Members without override get new default; changelog entry required (tasks).
31. Slack preference on but member has no personal Slack destination. `no_target` recorded once per notification, no error.
32. `email_delay_seconds=0` and trigger `customer_replied`. Member override wins: immediate email.
33. Pause `paused_until` in past. Treated as null. Max 30 days ahead else 422.

## Quiet hours and time

34. Overnight window 22:00 to 08:00 crossing midnight. In window when local time >= start or < end; `days` mask checked against day the window started.
35. DST spring forward, window starts 02:30 on skipped hour. Window starts at first valid instant after (03:00), same as foundation snooze rule.
36. DST fall back, end 01:30 occurs twice. Window ends at first occurrence.
37. Start equals end. 422 `quiet_hours_empty`.
38. Invalid timezone string. 422; member timezone null falls back to `users.timezone`, then workspace timezone, then UTC.
39. Member travels, browser timezone differs. Server uses stored setting only; frontend may suggest update. No IP-based guessing.
40. Quiet hours summary when member read everything before window end. Skip summary.
41. `ai_handoff_requested` in bypass list during quiet hours. Push and email sent; others suppressed.

## Focus and suppression

42. Member has dashboard open on conversation in background tab. Heartbeat sends `visible=false`: not focused; push sent.
43. Two tabs, one focused on conversation A, other hidden. Last heartbeat per member wins per device; focus = any device visible on A in last 75 s. Push for A suppressed, for B sent.
44. Laptop focused, phone in pocket. Push suppressed on all devices (member is looking). Intended: no double alert.
45. Heartbeat stops (laptop sleeps without visibilitychange). Focus expires after 75 s; next notification sent.
46. Heartbeat forged with other member conversation id. Only affects own suppression; harmless. Conversation id from other workspace ignored (404 not returned, silently ignored, 204).
47. Focus check vs in-app: in-app always created and broadcast (member sees toast in dashboard).
48. Notification read in dashboard before delayed email. `already_read` skip.
49. Teammate replies before delayed `customer_replied` email to assignee. `condition_changed` skip.

## Push subscriptions

50. Same browser registers twice. Upsert by `endpoint_hash`; 200 with existing id; keys updated.
51. User A logs out, user B logs in same browser, registers same endpoint. Row reassigned to B (delete A's row with that hash, insert B); A no longer receives on device.
52. Logout without client unregister (network down). Session row delete cascades to its push subscriptions in same tx; no push reaches shared device after logout.
53. Session rotated (login, password change). Old session row deleted, subscription deleted via FK cascade (never outlives its session). Dashboard re-registers on every boot (upsert by `endpoint_hash`), so gap lasts until next page load.
54. Endpoint host not in allowlist (e.g. attacker posts `https://169.254.169.254/`). 422 `push_endpoint_not_allowed`. `a.HTTP` re-check at send anyway.
55. VAPID key regenerated (`dimmi push:vapid --force`). Old subscriptions get 403 `VapidPkHashMismatch` (Apple) or 401/403 (FCM): deleted. Dashboard sees `push-config.key_id` differs from local, unsubscribes and resubscribes.
56. Subscription limit 20 reached. Evict least recently successful; never reject (new device wins).
57. Push service returns 413 even after minimal payload. `failed` reason `payload_too_large`, metric, no further retry.
58. iOS user enables push in Safari tab (not Home Screen). Browser API unavailable; server never sees subscription. `push-config` hint lets UI explain "Add to Home Screen first".
59. Safari revokes permission (user disabled in Settings). Next send 410 or 404: row deleted.
60. `pushsubscriptionchange` with no open client. Old endpoint eventually 410 and deleted; next dashboard load re-registers via `old_endpoint` replace.
61. Push provider slow (10 s timeout). Retry 5 s, 20 s, 60 s; stale after 10 minutes skipped.

## Slack

62. Slack webhook revoked (404 `no_service`). Destination `error`, one `system_alert` per destination per error transition (not per message). Further events skip destination until admin re-enables or replaces URL.
63. Slack channel archived (410). Same as 62 with hint.
64. Slack app uninstalled. 05 receives `app_uninstalled`, calls `MarkDestinationErrored` on `slack_app`; child DMs pause; members see "Slack disconnected" in preferences (`available_channels.slack=false`).
65. Member email differs in Slack. `slack_user_not_found`; suggest personal incoming webhook.
66. Customer writes `<!channel>` or `<!here>` in message. Slack escaping makes literal text. Never pings.
67. Customer message contains Slack link markup `<https://evil|Click>`. Escaped `<` `>`: shows literally.
68. 1,000 new conversations in minute (spam wave). Destination: 10 posts + 1 digest per minute. Members: burst digests. Spam classifier (03) marking spam before fan-out stops all.
69. Admin replaces webhook URL on `error` destination. PATCH `url` validates, resets status `active`, `consecutive_failures=0`.
70. Same Slack webhook URL added twice in workspace. Allowed (different trigger sets); dedupe not enforced. Note in UI only.

## Email

71. Member email unverified (cloud). Email channel `unverified_email`; in-app + push still work.
72. Member email hard-bounces. 04 suppression list; email channel `skipped` reason `email_suppressed`; in-app `system_alert` to member to fix email.
73. Member's autoresponder replies to notification. Reply goes to `notifications@` (not inbox address) and 04 drops replies to notification Message-IDs. No loop.
74. Self-host `notifications.email.from` equals support inbox address. Boot warning + `dimmi doctor` red; still send (operator choice) but `Auto-Submitted` header lets 04 drop auto-replies.
75. Unsubscribe link (`/notification-unsubscriptions`) clicked by corporate link scanner (GET). GET shows page only; nothing changes until POST.
76. Unsubscribe link for removed member. 200 page "You no longer receive emails from this workspace". Idempotent.
77. Unsubscribe from all in workspace. Sets `email` channel disabled for every non-locked trigger; owner `system_alert` email stays (locked).
78. Cloud workspace email cap (entitlement `notifications.email_daily_per_workspace`, cloud default 5,000/day; override raises it per customer) hit. Further emails batched into hourly digest per member; `system_alert` to owners once.

## Concurrency, retries, failures

79. DispatchNotifications retried after partial commit. Single tx: nothing partial. Retry after commit: unique `(event_id, channel, target_key)` and `source_event_ids` check prevent duplicates.
80. DeliverNotification retried after push accepted but before DB update (crash). Duplicate push possible; `tag` + `Topic` make OS replace same notification. Accepted at-least-once.
81. Two replicas run same digest. River unique job; digest marks items `digest_id` in tx with `FOR UPDATE SKIP LOCKED`.
82. Notifications queue backlog (e.g. 10k events). Stale rules drop late push/chat (10 min), email keeps 2 hours; metric alert fires.
83. Postgres down. Events not committed, nothing to notify. River resumes after.
84. Mail provider down. Email retries 5x ~10 min then `failed`; other channels unaffected.
85. `notifications.enabled=false` (self-host). Only in-app rows + realtime; external jobs not inserted.
86. Workspace suspended. Dispatch skips entire workspace; pending deliveries `skipped` reason `workspace_suspended`.
87. Workspace soft-deleted. Same as suspended; prune with workspace purge.
88. Mark all read (`PATCH /notifications {filter: {}, changes: {read: true}}`) with 50,000 unread rows. Batched updates 5,000 per statement in one action; returns `meta.updated`; single `notifications.read` broadcast with `all: true`.
89. Unread count above 99. API returns `unread_count: 100` cap and `unread_count_capped: true`.

## Upgrades and self-host

90. Upgrade adds new trigger. Defaults apply instantly; existing rows unaffected; registry enum (format CHECK only), no migration.
91. Self-host without HTTPS on dashboard. Push API requires secure context; `push-config` returns `allowed: false, reason: "insecure_origin"` when `app.url` not https (localhost exempt).
92. Self-host behind egress firewall. Push fails `provider_error`; `dimmi doctor` shows push hosts unreachable; docs list `*.push.apple.com`, `fcm.googleapis.com`, `updates.push.services.mozilla.com`, `*.notify.windows.com`.
93. Self-host with `http_client.allow_private_networks` false and Slack on public internet. Fine. Private Mattermost-style Slack-compatible URL: rejected by URL shape validation (MVP only official hosts).
94. Multiple replicas, focus heartbeat on replica A, job on replica B. Focus stored in Valkey (`ntf:focus:{mem}:{device}` EX 75 s), shared by all replicas, never in-memory only.
95. Valkey restarted or flushed. Focus, cooldown and burst windows lost: worst case one extra push/email or one item sent outside digest. Acceptable (rebuildable state only; notifications rows durable in Postgres).

## Test conversations

96. Conversation with `is_test = true` (widget preview, member testing own widget). Behavior: notifications still sent (members verify setup works), titles prefixed "[Test]", never posted to workspace-level Slack channel destinations, never counted in digests.

## REST shape, first-party app, push kinds

97. PAT or third-party OAuth token calls `POST /push-subscriptions`. 403 `first_party_only` (`U` route). Stops integration token from pointing member alerts at attacker endpoint.
98. Dimmi mobile app (first-party, `auth:app`) registers `kind: "apns"` device. 422 `push_kind_unsupported` until APNs driver ships; app falls back to in-app + email. No row written. Same for `fcm`.
99. Body omits `kind`. Defaults `web`; existing dashboard clients unchanged.
100. First-party app registers `kind: "web"` (desktop app wrapping Web Push). Row stored with `oauth_grant_id`, `session_id` null. App grant revoked (sessions list revoke, password change): `oauth_grant.revoked` listener deletes row; no further pushes.
101. Later release adds APNs driver. Rows `kind = 'apns'` use `device_token_encrypted`, `app_id`; web-only columns already nullable, CHECK already scoped to `kind = 'web'`: no table rewrite, only driver + validation change.
102. `PATCH /notifications/{id} {read: true}` on already read row. 200 no-op, `read_at` unchanged, no `notifications.read` broadcast.
103. `PATCH /notifications/{id} {seen: false}`. 422 (seen only settable true; badge reset is server-side by new occurrence).
104. Bulk `PATCH /notifications` with `ids` of other member's notification. That id `failed` `not_found` (never leaks existence); others updated.
105. Bulk with both `ids` and `filter`, or empty `changes`. 422, nothing updated.
106. Bulk filter `{before: T}` with `{seen: true}` while new notification arrives at T + 1 s. New row stays unseen (filter bound); badge shows 1.
107. `DELETE /notification-preferences` with no overrides. 204 no-op. Locked cells unaffected (never stored).
108. `POST /notification-tests {channel: "slack"}` without personal Slack destination. 201 outcome with delivery `suppressed` reason `no_target` (test reports, never errors).
109. Focus heartbeat from first-party app (foreground on conversation). Same suppression as dashboard tab (`focused`); app sends `visible: false` on background.

## Extensibility seams (`00-foundation/extensibility.md`)

110. Browser registers push via session. Row `device_id` = `sessions.device_id` (hashed device cookie); body `device_id` ignored. Session row without device cookie (old session): `device_id` null, push works.
111. First-party app registers with `device_id` equal to grant `device_id`. Stored. Different value: 422 `device_id_mismatch`, no row. Grant without `device_id` (old app build): body value stored; absent: null.
112. Two push rows (browser + desktop app) on same machine. Different `device_id` values (cookie hash vs install id); no merge at MVP. Later `user_devices` links them.
113. Push payload with 64 KB customer message and long contact name. Body excerpt 140 graphemes, title trimmed; plaintext under 3,000 bytes; no contact email, no attachment URLs. Integration test (real App, push service stubbed via `httpclienttest`) for every trigger with max inputs.
114. `show_previews = false`. Payload title generic ("New message in #1042"), body omitted; ids still present; app fetches details.
115. 08 registers trigger `article_suggestion` at boot (later). Preferences matrix gains row with registered defaults; no migration; DB format CHECK passes. Unknown trigger in `PATCH /notification-preferences`: 422 (registry miss). Stored row with trigger later unregistered: hidden from matrix, never dispatched.
116. Staff raises one cloud workspace email cap via `entitlement_overrides` (`notifications.email_daily_per_workspace` = 20,000). Takes effect within 60 s (entitlement cache), no deploy. Self-host: limit -1, cap never applies.
117. Custom role later without `notification_alerts:receive`. Members with role get no `system_alert`; MVP owners/admins via `roles.go` unchanged. 06 code never names roles.
118. 09 not migrated yet on fresh install order bug (06 migration before 09). FK creation fails loudly at migrate (ordering test in CI asserts 09 `000902` precedes 06 `000604`).

## Triggers from other specs, system stream (final sweep)

119. `feedback_post_pending` email delayed 300 s; moderator approves at 200 s. Email skipped `condition_changed`; in-app row stays (read state untouched).
120. Ten votes/comments on same feedback post while member's notification unread. One row per (member, `feedback_post_id`, trigger), `count` 10 (aggregation index added by 14 migration).
121. `csat_negative` on unassigned conversation with no team. Audience fallback (all active members minus actor) per audience rule; response `is_test = true`: no notification.
122. `changelog_entry_published` with `announce = false`. No destination post. Member preference API never lists this trigger (destination only); PATCH preference for it 422 `trigger_not_member_eligible`.
123. Destination lists `changelog_entry_published`, entry unpublished before job runs. `condition_changed` (entry no longer published); no post.
124. Member email hard-bounces on system stream (SES `dimmi-system` set). 04 router hands event to `RecordSystemMailFeedback`; `system_mail_suppressions` row; next member notification email `skipped email_suppressed`; in-app `system_alert` `system_mail.suppressed` to that member in every active workspace.
125. Same user later requests magic link. Critical auth mail bypasses suppression, max 3 per 24 h per address; 4th skipped and logged (login page still says "check your email": no account enumeration).
126. User changes email or re-verifies. `user.updated` / `user.email_verified` lifts row; notification email resumes.
127. Member is also a contact in another workspace with customer-stream complaint (04 scope `all`). Independent lists: member notifications still flow (system stream); conversation mail to that contact still blocked by 04.
128. System stream complaint rate over 0.05%. No pause (password reset must flow); ops alert + `dimmi doctor` warning with top Mailable kinds by complaints.

## Agent actions (07), channels (05), docs sync (08), imports (16)

129. `ai_action_approval_requested` while conversation assigned to member without `ai_action_calls:approve` (custom role later; MVP members hold it). Fallback to handoff team members holding it, else owners/admins. `approval: admin` action: only holders of `ai_actions:manage`.
130. Teammate approves call 30 s after request; delayed or burst-batched push/email still pending. `condition_changed` (call no longer `pending_approval`); in-app row stays, marked read by `conversation.read` when opened.
131. Connection test fails repeatedly (`ai.action_connection_failed` 5 times in 1 h). One `system_alert` per connection per 24 h (dedupe key `ntf:alert:{source}:{id}`, Valkey `SET NX EX 86400`, durable fallback: unread alert row for same subject exists); later failures update `count` on unread row.
132. Slack channel `channel.errored` then `channel.connected` reason `recovered` 10 minutes later. Alert posted once; recovery marks unread alert `data.resolved = true` (realtime `notification.created` upsert), no new notification; email already sent stays.
133. `channel.disconnected` (uninstall) while Slack is also a notification destination via same Slack app. Destination marked `error` separately (`notification_destination.errored`); both alerts deduped per subject, two rows (different subjects).
134. `docs_sync_run.completed` with `suggestion_count = 0`. No destination post. Count > 0 on destination not listing `docs_suggestions_ready`: no post; member preference API never lists it (destination only).
135. 16 import of 50k conversations. Zero notifications: no `message.created`/`conversation.created` dispatched. Bug dispatches one with `metadata.imported`: `SendMemberNotifications` skips (counter `dimmi_notifications_skipped_total{reason="imported"}`).
136. Live customer reply on imported conversation after import. Normal `customer_replied` rules (message not imported); assignee from import mapping or audience fallback.

