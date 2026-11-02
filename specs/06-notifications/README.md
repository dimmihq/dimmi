# 06 Notifications

Style: caveman. Date 2026-11-01. Read with `specs/00-conventions/README.md` and `specs/00-foundation/*`. Backend only.

Files: [README.md](README.md) (this), [delivery.md](delivery.md) (per-channel payload formats, provider facts, failure handling), [edge-cases.md](edge-cases.md), [tasks.md](tasks.md).

## Goal

Tell workspace members (not customers) about conversation activity where they live: in-app bell, email, web push (desktop + installed PWA incl. iOS), Slack. Under 10 seconds, never spammy, never to wrong person.

Why: roadmap launch item 6 "Notifications where solo dev lives" (`research/roadmap.md`). Solo dev not sit in dashboard. Competitor gap: HelpDesk users report no email on assignment and missed notifications (`research/competitors/helpdesk.md`).

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Event commit to push service accepted (push, Slack, in-app) | p95 under 5 s, p99 under 10 s |
| Event commit to Mailer accepted (immediate email triggers) | p95 under 10 s |
| In-app bell update over realtime | p95 under 1 s after event commit |
| Zero-config default | new workspace owner gets email + in-app for new conversation and AI handoff with no setup |
| Push setup | one click "Enable notifications" + permission prompt; test push arrives under 5 s |
| Wrong-person deliveries | zero: recipients resolved at send time; deactivated members, other workspaces never receive |
| No self-notify | actor never notified of own action (assign self, own note mention) |
| No notify when already handled | conversation assigned, replied, or read before send: push/email skipped (`condition_changed`, `already_read`) |
| No push while watching | member focused on that conversation in dashboard: no push/email/chat for it |
| Burst control | 50 new conversations in 1 minute produce at most 5 pushes + 1 digest per member per channel |
| Dead subscription cleanup | 404/410 push subscription deleted on first response |
| Debuggability | every delivery outcome stored with reason (`notification_deliveries`) + one log line; `POST /notification-tests` returns per-target outcome |
| Unsubscribe | every email has one-click unsubscribe (RFC 8058) + manage link |

## Scope

### In (MVP)

- Channels per member: `in_app` (database + realtime `member:{mem}`), `email`, `push` (Web Push with VAPID, RFC 8030/8291/8292, Declarative Web Push compatible payload), `slack` (DM via Slack app bot, or personal incoming webhook).
- Triggers: `new_conversation`, `conversation_assigned`, `customer_replied`, `mentioned`, `ai_handoff_requested`, `snooze_expired`, plus non-configurable `system_alert` (integration failures incl. 05 channel errored/disconnected, 07 agent action connection failed, 08 GitHub connection/repo webhook problems, webhook endpoint disabled from 10, changelog failures from 13). Registered by other specs (registry, no migration): 07 `ai_action_approval_requested`; 14 `feedback_post_submitted`, `feedback_post_pending`, `feedback_comment_created`, `feedback_mentioned`; 15-announcements-surveys `csat_negative`. Destination-only triggers: 13 `changelog_entry_published`, 08 `docs_suggestions_ready`.
- Imported data (16) never notifies: import writers dispatch no record events; `SendMemberNotifications` also skips any event whose message has `metadata.imported` (defense). Live messages on imported conversations notify normally.
- System stream feedback owner: bounces/complaints on mail to members/users (all `app/mail` Mailables on `mail.mailers.system`), section "System stream feedback".
- Per-member preferences: trigger x channel matrix, sparse overrides on top of defaults.
- Per-member settings: quiet hours (member timezone), pause until, previews on/off, AI-handled conversations on/off, email delay.
- Workspace destinations: Slack channel (incoming webhook, pasted or from Slack app OAuth). "Post every new conversation to #support". Per-destination trigger list + inbox filter.
- Smart suppression: focus heartbeat, read-state re-check, condition re-check at send, quiet hours, pause, burst digest, per-conversation cooldown.
- Deep links into dashboard. Email threading per conversation. Unsubscribe and manage links.
- Delivery failure handling: push 404/410 cleanup, Slack revoked webhooks mark destination `error` + `system_alert` to admins.
- APIs (REST per `00-foundation/rest.md`): notifications list, `PATCH` read/unread, bulk `PATCH /notifications` (read all, seen), preferences (+ `DELETE` reset), settings, push config, push subscriptions register/unregister (`kind` web; `apns`/`fcm` reserved), destinations CRUD + `tests`, focus heartbeat, `notification-tests`, signed `notification-unsubscriptions`.
- MCP tools for notifications list and mark read.

### Out (later)

- Reply from Slack notification thread: post-MVP: chat replies.
- Native mobile push (APNs/FCM) for Dimmi mobile app (first-party, members): post-MVP driver. Column `push_subscriptions.kind` (`web`, `apns`, `fcm`) and nullable web-only columns ship now so no migration break later; `POST /push-subscriptions` with `kind` `apns`/`fcm` returns 422 `push_kind_unsupported` until drivers ship. Customer mobile SDK push: post-MVP: mobile SDKs.
- Telegram, SMS, Microsoft Teams notifications: post-MVP: more channels timeframe.
- later: discord-notifications. Personal Discord webhook, workspace Discord channel destination, Discord DM via bot. Adds `discord` channel value + `discord_webhook` destination type back (registry/CHECK widen, additive).
- later: delivery-inspection-api. `GET /notifications/{id}/deliveries` (per-channel outcome + reason). Rows already stored in `notification_deliveries`; MVP reads them via logs/metrics and `POST /notification-tests` outcome.
- Rules-based routing notifications (SLA breach, office hours): post-MVP: rules lite.
- Notification for article suggestions, billing threshold to members: billing threshold handled by 12 directly (email); article suggestions by 08 later. `system_alert` trigger available to them.
- Per-tag or per-VIP-contact notification filters: post-MVP: ticket types, custom fields, views.
- Translations beyond `en` (copy): i18n itself is in (00-conventions "Localization (i18n)"): copy lives in `lang/<locale>/notifications.json`, every mail and notification renders in the recipient's locale (queued with it; `mail` `PendingMail.Locale(l)`), `en` ships first; adding a locale = adding a `lang/<locale>/` folder, no code.

## User stories

- Solo dev (owner, only member): installs widget, gets email within 10 s of first customer message without touching settings. Enables push on phone PWA, gets lock-screen push. Silences 23:00 to 08:00.
- Teammate: gets push when conversation assigned to them, when assigned customer replies, when mentioned in note. Not pushed about conversations they are looking at.
- Team lead (admin): posts every new conversation and AI handoff to Slack `#support`; `#support-alerts` channel for handoffs only.
- Member on shared device: logs out, device gets no more pushes.
- Developer: sends test notification, reads per-target outcome with reason (`suppressed: quiet_hours`) via `POST /notification-tests`.
- AI agent (MCP, acting as member): lists unread notifications, marks read after handling.
- End customer: never receives these notifications. Customer content inside notifications escaped and size-limited.

## Triggers

| Trigger | Source event (catalog) | Condition at fan-out | Recipients |
|---|---|---|---|
| `new_conversation` | `conversation.created`; `conversation.assigned` where `to.team` set, `to.member` null, team changed | not spam, contact not blocked, `assignee_member_id` null, `ai_state <> 'active'` unless recipient `include_ai_handled` | audience(conversation) |
| `conversation_assigned` | `conversation.assigned` where `to.member` set | `to.member <> actor` | `to.member` |
| `customer_replied` | `message.created` with `type='message'`, `author_type='contact'` | not first customer message of conversation (that is `new_conversation`), not spam, `ai_state <> 'active'` unless `include_ai_handled` | assignee member; if none and message reopened conversation: audience(conversation); if none and not reopened: audience only for members with `customer_replied` preference explicitly on for unassigned (`notify_unassigned_replies` setting, default false) |
| `mentioned` | `message.created` with `type='note'` and `MentionedMemberIDs` non-empty; `message.updated` with `AddedMentionedMemberIDs` non-empty (note edit) | mentioned member active, not author | each member from 03 `message_mentions` rows for message (team mentions already expanded; edit: only added ids) |
| `ai_handoff_requested` | `ai.handoff_requested` | conversation `ai_state='handed_off'` | assignee member else audience(conversation) |
| `snooze_expired` | `conversation.unsnoozed` with `reason='expired'` | status open | assignee member else audience(conversation) |
| `system_alert` | `notification_destination.errored`, `webhook_endpoint.failing` / `webhook_endpoint.disabled` (10), direct `Notify` calls; sources by owner: 05 `channel.errored`, `channel.disconnected` (deduped per connection per 24 h; `channel.connected` reason `recovered` marks unread alert resolved, no new notification); 07 `ai.action_connection_failed` (class `ai_action_connection_failed` owned by 07, once per connection per 24 h); 08 `github_connection.errored` (token expiring, invalid, expired) and `github_repository.webhook_status_changed` to `failing` (optional, once per repo per 24 h); 13 `changelog_source.errored`, changelog mailing `blocked`/`failed`, scheduled publish failed; 06 own `system_mail.suppressed` (member's own address bounced, to that member only) | none | active members holding `notification_alerts:receive` (MVP: owners and admins via `roles.go`); `system_mail.suppressed`: affected member only |

Triggers registered by other specs (registry `enums.NotificationTrigger.Register(key, defaults)` at boot in owner's provider; notification class owned by registering spec; 06 runs fan-out, suppression, delivery):

| Trigger (owner) | Source event | Condition at fan-out | Recipients |
|---|---|---|---|
| `ai_action_approval_requested` (07) | `ai.action_call_approval_requested` | call still `pending_approval` at send (approved/declined/expired meanwhile: `condition_changed`) | conversation assignee when holding `ai_action_calls:approve` (or `ai_actions:manage` for `approval: admin`); else handoff team members holding it; else admins/owners. Deep link to conversation |
| `feedback_post_submitted` (14) | `feedback_post.created` (approved, source customer) | post not spam/trashed | active members with `feedback:read` |
| `feedback_post_pending` (14) | `feedback_post.created` / `feedback_comment.created` pending approval | still pending at send (email delay 5 min; moderated meanwhile: `condition_changed`) | members with `feedback:write` |
| `feedback_comment_created` (14) | `feedback_comment.created` (public) | member subscribed to post (14 `feedback_subscriptions`), not author | subscribed members |
| `feedback_mentioned` (14) | `feedback_comment.created` internal with rows in `feedback_comment_mentions` | mentioned member active, not author | mentioned members |
| `csat_negative` (15-announcements-surveys) | `survey_response.completed` CSAT score <= 2 of 5 (or NPS 0..6) | response not `is_test` | conversation assignee; none: assignee team members (audience fallback minus actor) |
| `changelog_entry_published` (13, destination only) | `changelog_entry.published` with `announce = true` | entry still published | workspace destinations listing it (never members) |
| `docs_suggestions_ready` (08, destination only) | `docs_sync_run.completed` with `suggestion_count > 0` | run not superseded | workspace destinations listing it: Slack post with PR ref, suggestion count, review queue link (never members) |

Subject column: feedback triggers set `notifications.feedback_post_id`; `csat_negative` sets `conversation_id` (+ `data.survey_response_id`); `ai_action_approval_requested` sets `conversation_id` (+ `data.action_call_id`, `data.action_title`).

audience(conversation): active members of `conversations.assignee_team_id` team if set; else members of `inboxes.default_team_id` team if set; else all active members. Always minus actor. Deactivated members and members of `status='deactivated'` never included.

Actor: from event `Actor`. Actor `system` or `agent` never excluded member (no member to exclude).

## Defaults (trigger x channel)

`on` unless listed. Member-level `slack` only applies once member has active personal destination.

| Trigger | in_app | push | email | slack |
|---|---|---|---|---|
| new_conversation | on | on | on | on |
| conversation_assigned | on | on | on | on |
| customer_replied | on | on | on (delayed 120 s, only if still unread) | on |
| mentioned | on | on | on | on |
| ai_handoff_requested | on | on | on | on |
| snooze_expired | on | on | off | off |
| system_alert | on (locked) | off | on (locked for owners) | off |
| feedback_post_submitted (14) | on | off | off | on |
| feedback_post_pending (14) | on | off | on (delayed 300 s, skipped if moderated) | off |
| feedback_comment_created (14) | on | off | off | off |
| feedback_mentioned (14) | on | on | on | off |
| csat_negative (15-announcements-surveys) | on | on | off | off |
| ai_action_approval_requested (07) | on | on | on | off |

Registered defaults live in owner spec's provider; this table mirrors them.

Locked = cannot be disabled (`in_app` for `system_alert`; `email` for owners on `system_alert` so webhook auto-disable and revoked integration always reach someone). API returns `locked: true`; PATCH to disable returns 422 `preference_locked`.

Defaults defined in code (`app/notifications/defaults.go`), never stored. Changing a default in release changes behavior for members without override. Changelog entry required.

## Suppression pipeline (per recipient, per channel, at send time)

Order matters. First match wins, recorded as delivery `status='suppressed'` + `reason`, except `preference_off` (no row written).

1. `preference_off`: channel disabled for trigger.
2. `member_inactive`: member deactivated or removed since fan-out.
3. `conversation_gone`: conversation deleted, spam, or contact blocked.
4. `condition_changed`: trigger condition no longer true (see triggers table; e.g. `new_conversation` now assigned to someone, `ai_handoff_requested` but member already replied after handoff, `customer_replied` but teammate or AI replied after this message).
5. `already_read`: member `conversation_reads.last_read_at >= message.created_at` (only checked for delayed email and digest items).
6. `focused`: member focus heartbeat (Valkey key `ntf:focus:{mem}:{device}` EX 75 s, last 75 s) shows dashboard visible on this conversation. For `new_conversation` and `system_alert`: dashboard visible on any page. Applies to push, email, slack. Never to in_app.
7. `paused`: `notification_settings.paused_until > now`. Not in_app.
8. `quiet_hours`: inside member quiet hours in member timezone. Not in_app. Trigger in `quiet_hours_bypass` skips this rule.
9. `cooldown`: push/slack for same member + conversation + trigger sent in last 30 s (Valkey `SET NX EX 30` key `ntf:cooldown:{mem}:{subject}:{trigger}:{channel}`).
10. `unverified_email`: email channel and user `email_verified_at` null.
11. `no_target`: push with zero subscriptions, slack with no active personal destination. Recorded once, no error.
12. `burst`: channel burst threshold hit (Valkey sliding window via `a.Limiter`, key `ntf:burst:{mem|ndst}:{channel}`); item becomes `batched` and joins digest (not lost).

Valkey state (PO rule: Valkey in every deployment; rebuildable state only): focus keys, cooldown keys, burst windows, event-id dedupe for `RecordSystemMailFeedback`, throttles (`notification-tests` 6/min). Durable in Postgres: `notifications`, `notification_deliveries` (outcome log), preferences, settings, subscriptions, destinations, `system_mail_suppressions`. Valkey loss: worst case one extra push/email (focus/cooldown forgotten) or one digest-eligible item sent individually; never lost notifications (edge 95).

Workspace destinations apply only rule 3 (`conversation_gone`), rule 4 (`condition_changed`; exception: `new_conversation` still posts when assigned meanwhile, channel post is a log), and rule 12 (`burst`).

## System stream feedback (owner 06)

All mail to Dimmi users/members (`app/mail` Mailables on `mail.mailers.system`: 06 notifications, 00-accounts auth mail, invitations, 12 billing, system alerts) shares one reputation on system config set `dimmi-system` / Postmark system stream. 04 owns the webhook endpoints and routes events tagged `stream=system` to 06 (04 outbound.md 7.3).

- Action `notifications.RecordSystemMailFeedback(ctx, DeliveryEvent{Transport, ProviderMessageID, Recipient, Type, Subtype, Diagnostic, OccurredAt, EventID})`, idempotent by `(transport, EventID)` (Valkey `SET NX EX 86400` + table unique).
- Hard bounce (SES `Permanent`, provider hard bounce, `OnAccountSuppressionList`): upsert `system_mail_suppressions` reason `hard_bounce`. Complaint (`abuse`, `fraud`, `virus`, `other`; not `not-spam`): reason `complaint`. Transient/delayed: metric only.
- Effects:
  - 06 email channel: suppression rule `email_suppressed` (delivery `skipped`), all triggers, all workspaces of that user. In-app `system_alert` kind `system_mail.suppressed` to that member in each active workspace ("Emails to you@x bounced. Update your address or contact admin.").
  - Other system Mailables: guard `app/mail/system_suppression_guard.go` (06) wraps `a.Mail.Mailer("system")` send for non-critical Mailables (12 billing notices, invitations to already-suppressed address, digests): skip + log. Critical user-initiated auth mail (magic link, email code, password reset, email verification, security notices) bypasses `complaint` (user is asking now) and `hard_bounce` for at most 3 sends per 24 h per address (address may be fixed; rate keeps reputation safe).
  - Lift: automatic on `user.email_verified` or `user.updated` with new email (00-accounts events); instance admin `dimmi mail:suppressions --stream system list|remove <email>`; member UI "I fixed my mailbox" after re-verifying email (reuses 00-accounts `POST /email-verifications`).
- Reputation: no auto-pause of system stream (password reset must flow); `dimmi_system_mail_bounce_rate` and `dimmi_system_mail_complaint_rate` gauges (trailing 1,000 sends); alert ops at bounce over 2% or complaint over 0.05% (half SES review thresholds); `dimmi doctor` shows both.
- Never touches 04 `email_suppressions` (customer stream, per workspace) and vice versa: member also a contact in some workspace keeps independent states.

### system_mail_suppressions (instance-level, no `workspace_id`, no RLS)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | |
| email | citext not null unique | user address (lowercased) |
| user_id | uuid null fk users on delete set null | resolved at insert |
| reason | text not null CHECK in ('hard_bounce','complaint','manual') | |
| source | text not null CHECK in ('ses','provider','dsn','manual') | |
| details | jsonb not null default '{}' | diagnostic, provider, event id (no body) |
| critical_sends_window_start | timestamptz | rolling 24 h window for critical bypass count |
| critical_sends | int not null default 0 | |
| created_at, updated_at | timestamptz | |

Retention: row deleted on lift; user erase deletes row (00-accounts eraser hook).

## Data model

All new tables tenant-scoped (`workspace_id`, RLS) except `push_subscriptions` (user-scoped, like `sessions`; see Security). New ID prefixes: `ntf_` notifications, `psub_` push subscriptions, `ndst_` notification destinations, `ndlv_` notification deliveries. Member public ID `mem_` (users `usr_`; 03 uses `mem_` for assignee, author, mentions).

### notifications (in-app inbox)

Base table owned by framework (`framework/notifications/channels/database` migrate step, runs before app set): `id`, `notifiable_type`, `notifiable_id` text, `type`, `data`, `read_at`, `created_at` (00-foundation data-model.md). 06 migration 0601 ALTERs it: adds product columns below, `notifiable_type = 'workspace_member'` + `notifiable_id` = member id mirror `workspace_member_id`, `type` = `trigger`. Never CREATE.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | `ntf_` |
| workspace_id | uuid not null fk workspaces cascade | |
| workspace_member_id | uuid not null fk workspace_members cascade | recipient |
| trigger | text not null CHECK (trigger ~ '^[a-z][a-z0-9_]{1,39}$') | registry enum (`00-foundation/extensibility.md` P1): Go registry `enums.NotificationTrigger` (`app/support/notifications/triggers.go`), validated in request rules and `Notify`. MVP values: `new_conversation`, `conversation_assigned`, `customer_replied`, `mentioned`, `ai_handoff_requested`, `snooze_expired`, `system_alert`. Other specs register at boot with default channel row, no migration: 14 `feedback_post_submitted`, `feedback_post_pending`, `feedback_comment_created`, `feedback_mentioned`; 15-announcements-surveys `csat_negative`; later 08 `article_suggestion`, 12 `billing_threshold`. No CHECK list anywhere (14 "CHECK lists" request = registry entries) |
| conversation_id | uuid null fk conversations cascade | null for system_alert and non-conversation triggers |
| feedback_post_id | uuid null fk feedback_posts on delete cascade | 14 feedback triggers; column added by 14 migration (FK target is 14 table; ordering like 09 FK, edge 118) |
| message_id | uuid null fk messages on delete set null | latest message that caused it |
| actor_type | text CHECK in ('member','contact','agent','system','integration') | `contracts.Actor.Type`; token/OAuth/MCP are `Actor.Via`, never type |
| actor_id | uuid null | |
| data | jsonb not null default '{}' | render data: contact name, conversation number, subject, excerpt (max 280 chars), inbox name, channel type, alert kind + subject ids. Max 4 KB |
| count | int not null default 1 | aggregated occurrences while unread |
| source_event_ids | uuid[] not null | last 50 event ids, idempotency for aggregation |
| seen_at | timestamptz | bell opened (badge cleared) |
| read_at | timestamptz | |
| created_at, updated_at | timestamptz | `updated_at` = last occurrence, drives sort |

Indexes: `(workspace_id, workspace_member_id, updated_at desc, id desc)`; partial `(workspace_id, workspace_member_id) WHERE read_at IS NULL`; unique partial `(workspace_id, workspace_member_id, conversation_id, trigger) WHERE read_at IS NULL AND conversation_id IS NOT NULL` (aggregation target); unique partial `(workspace_id, workspace_member_id, feedback_post_id, trigger) WHERE read_at IS NULL AND feedback_post_id IS NOT NULL` (14 aggregation, added by 14 migration).

Aggregation: new occurrence with unread row for same member + subject (conversation, or feedback post) + trigger updates that row (`count = count + 1`, `message_id`, `data`, `updated_at`, `seen_at = null`, append event id) only if event id not in `source_event_ids`. Otherwise insert. Read rows never reopened; new occurrence creates new row.

### notification_preferences

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | |
| workspace_id | uuid not null | |
| workspace_member_id | uuid not null fk cascade | |
| trigger | text not null CHECK (same registry format) | registry enum, same registry |
| channel | text not null CHECK in ('in_app','email','push','slack') | |
| enabled | boolean not null | |
| created_at, updated_at | timestamptz | |

Unique `(workspace_id, workspace_member_id, trigger, channel)`. Row exists only when differs from default (PATCH back to default deletes row).

### notification_settings (1:1 member)

| Column | Type | Notes |
|---|---|---|
| workspace_member_id | uuid pk fk workspace_members cascade | |
| workspace_id | uuid not null | |
| timezone | text null | IANA; null = `users.timezone`, then `workspaces.timezone` |
| quiet_hours_enabled | boolean not null default false | |
| quiet_hours_start | time not null default '22:00' | local wall clock |
| quiet_hours_end | time not null default '08:00' | end < start means overnight |
| quiet_hours_days | smallint not null default 127 | bitmask Mon=1..Sun=64; day of window start |
| quiet_hours_bypass | text[] not null default '{}' | triggers allowed through (e.g. `ai_handoff_requested`) |
| quiet_hours_summary | boolean not null default true | email summary of unread suppressed items at window end |
| paused_until | timestamptz | do not disturb; max now + 30 days |
| show_previews | boolean not null default true | message excerpt in push, email, slack |
| include_ai_handled | boolean not null default false | notify `new_conversation`/`customer_replied` while Dimmi Agent active |
| notify_unassigned_replies | boolean not null default false | customer replies on unassigned conversations |
| email_delay_seconds | int null CHECK 0..3600 | null = per-trigger default; else applies to every email trigger, email skipped if read meanwhile |
| created_at, updated_at | timestamptz | |

Row created lazily (absent = all defaults). No workspace initializer needed: contracts.md hook entry "06 default notification settings" satisfied by lazy defaults (06 registers none).

### push_subscriptions (user-scoped)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `psub_` |
| user_id | uuid not null fk users cascade | |
| kind | text not null default 'web' CHECK in ('web','apns','fcm') | `web` = Web Push (MVP, only kind accepted by API). `apns`, `fcm` reserved for Dimmi mobile app (first-party) native push, post-MVP driver; in enum now so adding drivers needs no migration |
| session_id | uuid null fk sessions on delete cascade | browser session that registered; logout, revoke, expiry prune delete session row and its subscriptions (shared-device safety: no pushes after logout) |
| oauth_grant_id | uuid null fk oauth_grants on delete cascade | depends on 09: `oauth_grants` table (FK target, 09 migration `20261101000902` must run before 06 `20261101000604`) and 09 event `oauth_grant.revoked` (payload `grant_id`, `user_id`, `first_party`, `device_id`, `reason`). First-party app grant (`ogr_`, `auth:app`) that registered; grant revoke (`DELETE /sessions/{ses}` on app entry, password change, logout-all) deletes its subscriptions via listener on `oauth_grant.revoked` (grant rows may be soft-revoked; FK cascade covers hard delete). CHECK `num_nonnulls(session_id, oauth_grant_id) = 1` (tokens cannot register) |
| device_id | text null | stable device identity (P3, extensibility.md section 5), max 64. Session rows: server copies `sessions.device_id` (SHA-256 hex of `__Host-dimmi_device` cookie; body value ignored). App rows: body `device_id` = app install id (same value app sent as 09 authorize `device_id`); grant has `device_id` and body differs: 422 `device_id_mismatch`; grant null: body value stored. Joins push rows with session and grant for later "revoke this device" (one click kills session/grant + push) and `user_devices` backfill |
| endpoint | text null | `web`: required, max 2048, https, host allowlisted. Null for `apns`/`fcm` |
| endpoint_hash | bytea not null unique | `web`: sha256(endpoint). `apns`/`fcm` (later): sha256(kind + device token). Lookup and dedupe |
| p256dh | text null | `web`: required, base64url, 65-byte uncompressed P-256 point |
| auth_secret_encrypted | bytea null | `web`: required, 16-byte auth secret, envelope encrypted (user data key) |
| vapid_key_id | text null | `web`: required; which VAPID key subscribed; mismatch with current key = stale |
| device_token_encrypted | bytea null | reserved `apns`/`fcm` device/registration token, envelope encrypted; always null at MVP |
| app_id | text null | reserved `apns` bundle id / `fcm` app id (prod vs dev builds); always null at MVP |
| platform | text CHECK in ('apple','google','mozilla','microsoft','other') | derived from host (web) or kind (later) |
| user_agent | text | truncated 512 |
| device_label | text | client supplied, max 80, e.g. "iPhone PWA" |
| expires_at | timestamptz | from `expirationTime` if provided |
| last_success_at, last_failure_at | timestamptz | |
| consecutive_failures | int not null default 0 | |
| created_at, updated_at | timestamptz | |

CHECK `kind <> 'web' OR (endpoint IS NOT NULL AND p256dh IS NOT NULL AND auth_secret_encrypted IS NOT NULL AND vapid_key_id IS NOT NULL)`. Index `(user_id)`; partial `(user_id, device_id) WHERE device_id IS NOT NULL` (device revoke later). Max 20 per user across kinds (oldest by `last_success_at` evicted on insert). Push channel sends only `kind = 'web'` rows at MVP (other kinds never exist yet; sender skips them defensively, reason `push_kind_unsupported`).

### notification_destinations

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `ndst_` |
| workspace_id | uuid not null | |
| scope | text not null CHECK in ('workspace','member') | |
| workspace_member_id | uuid null fk cascade | required when scope member, null when workspace |
| type | text not null CHECK in ('slack_webhook','slack_app','slack_dm') | `slack_app` = bot install record (workspace scope only); `slack_dm` = member link to Slack user via `slack_app` |
| name | text not null | display, 1..80 |
| credentials_encrypted | bytea | webhook URL or bot token |
| parent_id | uuid null fk notification_destinations cascade | `slack_dm` points to `slack_app` |
| config | jsonb not null default '{}' | `slack_team_id`, `slack_team_name`, `slack_channel_id`, `slack_channel_name`, `slack_user_id`, `bot_scopes` |
| triggers | text[] not null default '{}' | workspace scope only; allowed: new_conversation, customer_replied, conversation_assigned, ai_handoff_requested, snooze_expired, `feedback_post_submitted` (14), `changelog_entry_published` (13: title, labels, summary, link; community announcements). Allowed list = registry entries flagged `destination_eligible` |
| inbox_ids | uuid[] null | workspace scope filter; null = all inboxes |
| include_ai_handled | boolean not null default false | workspace scope |
| show_previews | boolean not null default true | workspace scope |
| status | text not null CHECK in ('active','error','disabled') default 'active' | |
| last_error, last_error_code | text | |
| last_error_at, last_delivered_at | timestamptz | |
| consecutive_failures | int not null default 0 | |
| created_by_member_id | uuid null fk on delete set null | |
| created_at, updated_at | timestamptz | |

Indexes: `(workspace_id, scope, status)`, `(workspace_id, workspace_member_id)`. Unique `(workspace_id, (config->>'slack_team_id')) WHERE type='slack_app'`. Unique `(workspace_id, workspace_member_id, type) WHERE scope='member'` (one personal destination per type). Max 25 workspace-scope destinations per workspace.

### notification_deliveries (outcome log, 14-day retention)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `ndlv_` |
| workspace_id | uuid not null | |
| notification_id | uuid null fk notifications cascade | null for workspace destination posts and digests |
| destination_id | uuid null fk notification_destinations cascade | |
| workspace_member_id | uuid null | |
| event_id | uuid not null | source event (or digest id) |
| trigger | text not null | |
| channel | text not null CHECK in ('push','email','slack','in_app') | |
| target_key | text not null | push subscription id, destination id, member id (email/in_app) |
| status | text not null CHECK in ('pending','sent','failed','suppressed','batched','skipped') | |
| reason | text | suppression reason or error code (`gone`, `revoked`, `rate_limited`, `timeout`, `payload_too_large`) |
| provider_status | int | HTTP status from provider |
| attempts | int not null default 0 | |
| latency_ms | int | event `occurred_at` to provider accepted |
| digest_id | uuid null | set when `batched` item later sent in digest |
| scheduled_for | timestamptz | delayed email, digest |
| created_at, sent_at | timestamptz | |

Unique `(workspace_id, event_id, channel, target_key)` (idempotent fan-out). Index `(workspace_id, notification_id)`, partial `(workspace_id, workspace_member_id, channel, created_at) WHERE status IN ('sent','batched')` for burst counting.

### Core table changes

None. Reads `conversation_reads`, `conversations`, `workspace_members`, `team_members`, `inboxes`, `users`, `sessions`. Reads 03 `message_mentions` (canonical mention store: `(message_id, workspace_member_id, via_team_id)`, written by 03 in note tx) for `mentioned` recipients; never reads `messages.metadata.mentions` (render metadata only).

### Migrations

| File | Creates |
|---|---|
| `database/migrations/20261101000601_alter_notifications_table.sql` | notifications: ALTER framework table (product columns, indexes); not CREATE |
| `database/migrations/20261101000602_create_notification_preferences_table.sql` | notification_preferences |
| `database/migrations/20261101000603_create_notification_settings_table.sql` | notification_settings |
| `database/migrations/20261101000604_create_push_subscriptions_table.sql` | push_subscriptions |
| `database/migrations/20261101000605_create_notification_destinations_table.sql` | notification_destinations |
| `database/migrations/20261101000606_create_notification_deliveries_table.sql` | notification_deliveries |
| `database/migrations/20261101000607_add_notification_rls_policies.sql` | RLS for tenant tables above (cloud) |
| `database/migrations/20261101000608_create_system_mail_suppressions_table.sql` | system_mail_suppressions (instance-level) |

14 adds `notifications.feedback_post_id` + its partial unique index (aggregation key `(workspace_member_id, feedback_post_id, trigger)` where unread) in 14 migration `20261101001423_add_feedback_post_id_to_notifications_table.sql` (FK target `feedback_posts`); 06 documents it, ships no migration for it.

## Code map

```
app/
├── enums/
│   ├── notification_trigger.go            # registry enum (P1): Register(key, defaults) at boot; MVP NotificationTriggerNewConversation ...
│   ├── notification_channel.go            # in_app, email, push, slack
│   ├── notification_delivery_status.go
│   ├── notification_suppression_reason.go
│   └── notification_destination_type.go
├── events/
│   ├── notification_created.go
│   ├── notifications_read.go
│   └── notification_destination_errored.go
├── notifications/                         # Laravel Notification classes, one per trigger
│   ├── notifier.go                        # Notify(ctx, recipients, Notification): prefs, suppression, persistence, channel dispatch
│   ├── defaults.go                        # default matrix + locked cells
│   ├── audience.go                        # audience(conversation) resolver
│   ├── suppression.go                     # ordered rule pipeline, pure functions + loaders
│   ├── quiet_hours.go                     # tz-aware window math
│   ├── burst.go                           # sliding window counter (a.Limiter, Valkey) + digest scheduling
│   ├── deeplinks.go                       # route("dashboard.conversations.show") helpers
│   ├── new_conversation_notification.go
│   ├── conversation_assigned_notification.go
│   ├── customer_replied_notification.go
│   ├── mentioned_in_note_notification.go
│   ├── ai_handoff_requested_notification.go
│   ├── snooze_expired_notification.go
│   ├── system_alert_notification.go
│   ├── digest_notification.go
│   ├── test_notification.go
│   └── channels/
│       ├── database_channel.go            # notifications row + realtime broadcast
│       ├── mail_channel.go
│       ├── push_channel.go                # on framework `notifications/channels/webpush` (wraps SherClockHolmes/webpush-go v1.4.x: RFC 8291 + VAPID, HTTPClient = a.HTTP.StdClient()); 06 maps responses (404/410/413/429) only
│       └── slack_channel.go               # incoming webhook + chat.postMessage DM
├── mail/
│   ├── member_notification_mail.go        # one mailable, per-trigger templ partials
│   ├── system_suppression_guard.go        # wraps system mailer: skip suppressed (critical auth bypass rules)
│   ├── notification_digest_mail.go
│   └── quiet_hours_summary_mail.go
├── services/
│   ├── push/payload.go                    # PushPayload builder (payload rule, 4 KB cap) shared by web now, apns/fcm later
│   └── slack/                             # shared with 05: add incoming_webhook.go, oauth_v2_access.go, users_lookup.go, conversations_open.go, chat_post_message.go
├── actions/notifications/
│   ├── dispatch_notifications_for_event.go   # event -> triggers -> recipients -> Notify
│   ├── deliver_notification.go               # one member + channel, re-checks suppression, sends
│   ├── deliver_destination_notification.go
│   ├── send_notification_digest.go
│   ├── send_quiet_hours_summary.go
│   ├── mark_notification_read.go             # PATCH `read: true`
│   ├── mark_notification_unread.go           # PATCH `read: false`
│   ├── mark_notifications_seen.go            # PATCH `seen: true`
│   ├── bulk_update_notifications.go          # PATCH /notifications (ids or filter), batches 5,000
│   ├── mark_conversation_notifications_read.go  # on conversation.read
│   ├── update_notification_preferences.go
│   ├── reset_notification_preferences.go     # DELETE /notification-preferences
│   ├── update_notification_settings.go
│   ├── record_notification_focus.go
│   ├── register_push_subscription.go         # kind web only at MVP; apns/fcm -> push_kind_unsupported
│   ├── delete_push_subscription.go
│   ├── delete_session_push_subscriptions.go
│   ├── delete_grant_push_subscriptions.go    # on oauth_grant.revoked (first-party app)
│   ├── create_notification_destination.go
│   ├── update_notification_destination.go
│   ├── delete_notification_destination.go
│   ├── start_slack_notification_install.go
│   ├── complete_slack_notification_install.go
│   ├── link_slack_dm.go
│   ├── test_notification_destination.go     # POST .../tests
│   ├── send_test_notification.go             # POST /notification-tests
│   ├── mark_destination_errored.go
│   ├── record_system_mail_feedback.go        # system-stream bounce/complaint (called by 04 router)
│   ├── lift_system_mail_suppression.go       # on user.email_verified / email change / admin CLI
│   └── unsubscribe_from_email.go             # POST /notification-unsubscriptions
├── http/
│   ├── controllers/api/v1/
│   │   ├── notification_controller.go                 # Index, Show, Update (field-dispatch `read`, `seen`), UpdateMany (bulk)
│   │   ├── notification_test_controller.go            # Store (POST /notification-tests)
│   │   ├── notification_preference_controller.go      # Index, Update, Destroy (reset)
│   │   ├── notification_setting_controller.go         # Show, Update
│   │   ├── notification_focus_controller.go           # Store
│   │   ├── push_config_controller.go                  # Show
│   │   ├── push_subscription_controller.go            # Index, Store, Destroy
│   │   ├── notification_destination_controller.go     # Index, Store, Show, Update, Destroy
│   │   ├── notification_destination_test_controller.go # Store (POST .../tests)
│   │   └── slack_notification_install_controller.go   # Store (returns authorize URL)
│   ├── controllers/web/
│   │   ├── slack_notification_callback_controller.go  # Invoke (OAuth redirect)
│   │   └── notification_unsubscription_controller.go  # Create (confirm page), Store (RFC 8058 one-click POST)
│   ├── requests/
│   │   ├── index_notifications_request.go
│   │   ├── update_notification_request.go           # read bool, seen true
│   │   ├── update_many_notifications_request.go     # ids xor filter {conversation_id, trigger, before}, changes {read|seen}
│   │   ├── update_notification_preferences_request.go
│   │   ├── update_notification_settings_request.go
│   │   ├── store_notification_focus_request.go
│   │   ├── store_push_subscription_request.go       # kind (default web) checked first
│   │   ├── store_notification_destination_request.go
│   │   ├── update_notification_destination_request.go
│   │   ├── store_notification_test_request.go
│   │   └── store_slack_notification_install_request.go
│   └── resources/
│       ├── notification_resource.go
│       ├── notification_preferences_resource.go
│       ├── notification_settings_resource.go
│       ├── push_subscription_resource.go
│       ├── push_config_resource.go
│       └── notification_destination_resource.go
├── listeners/
│   ├── send_member_notifications.go            # queued: inserts DispatchNotifications job
│   ├── mark_notifications_read_on_conversation_read.go  # queued
│   ├── delete_push_subscriptions_on_logout.go  # InTx on `session.revoked` (belt; FK cascade primary), `oauth_grant.revoked` (first-party app rows) and `user.deleted` (all user rows)
│   ├── broadcast_notification_created.go       # sync after commit (via BroadcastToRealtime)
│   └── lift_system_mail_suppression_listener.go # user.email_verified, user.updated (email)
├── jobs/
│   ├── dispatch_notifications.go
│   ├── deliver_notification.go
│   ├── deliver_destination_notification.go
│   ├── send_notification_digest.go
│   ├── send_quiet_hours_summary.go
│   ├── prune_notifications.go
│   └── prune_push_subscriptions.go
├── policies/
│   ├── notification_policy.go
│   ├── push_subscription_policy.go
│   └── notification_destination_policy.go
├── mcp/tools/
│   ├── list_notifications.go                   # dimmi_list_notifications
│   └── mark_notifications_read.go              # dimmi_mark_notifications_read
├── console/commands/
│   ├── push_vapid_generate_command.go          # dimmi push:vapid
│   ├── mail_suppressions_command.go            # dimmi mail:suppressions --stream system
│   └── notifications_test_command.go           # dimmi notifications:test --member --channel
config/notifications.go
database/
├── migrations/20261101000601..607 (above)
├── queries/
│   ├── notifications.sql
│   ├── notification_preferences.sql
│   ├── notification_settings.sql
│   ├── push_subscriptions.sql
│   ├── notification_destinations.sql
│   ├── notification_deliveries.sql
│   └── system_mail_suppressions.sql
└── factories/
    ├── notification_factory.go
    ├── notification_preference_factory.go
    ├── push_subscription_factory.go
    └── notification_destination_factory.go
resources/views/mail/notifications/
├── layout.templ                                 # shared header/footer with manage + unsubscribe links
├── new_conversation.templ ... snooze_expired.templ, system_alert.templ, digest.templ, quiet_hours_summary.templ
resources/views/web/notifications/unsubscription.templ
lang/en/notifications.json
tests/
├── feature/notifications/
│   ├── notifications_api_test.go
│   ├── notification_preferences_api_test.go
│   ├── notification_settings_api_test.go
│   ├── push_subscriptions_api_test.go
│   ├── notification_destinations_api_test.go
│   ├── slack_notification_install_test.go
│   ├── unsubscription_test.go
│   ├── dispatch_triggers_test.go            # every trigger, recipients, actor exclusion
│   ├── suppression_test.go                  # every rule
│   ├── digest_test.go
│   └── delivery_failures_test.go            # 404/410/413/429, Slack revoked
│   ├── quiet_hours_test.go                  # DST, overnight, day mask via frozen clock through dispatch (integration, testing.md)
│   ├── audience_test.go                     # real teams/inboxes rows
│   ├── burst_test.go                        # Valkey windows, digest scheduling
│   └── system_mail_feedback_test.go         # 04-routed events, guard, critical bypass, lift
# no app unit tests (testing.md); RFC 8291 vectors live in framework notifications/channels/webpush tests
```

Routes: `routes/api.go` (api v1 group), `routes/web.go` (callback, unsubscribe), `routes/console.go` (periodic jobs), `routes/channels.go` (`member:{mem}` authorizer, owned by foundation, verified here), `routes/mcp.go`.

## HTTP API

All under `/api/v1`, `throttle:api`. REST rules: `00-foundation/rest.md`. Notification routes are per-member: actor must be workspace member (browser session, first-party app, or token owned by member); else 403 `member_actor_required`. Every endpoint acts on current member only, except destinations with workspace scope. Guard letters (`00-foundation/api.md`): `A` = `auth` (session, first-party app, token; abilities apply to tokens only, session and first-party app use member role) + `workspace`; `U` = `auth:user` (session or first-party app; tokens 403 `first_party_only`).

| Method | Path | Route name | Auth + ability | Purpose |
|---|---|---|---|---|
| GET | /notifications | notifications.index | A; `notifications:read` | list mine; `filter[status]=unread\|read\|all` (default all), `filter[trigger]`, `filter[conversation]`; sort `-updated_at` only; meta `unread_count`, `unseen_count` (capped 100) |
| PATCH | /notifications | notifications.update-many | A; `notifications:write` | bulk (rest.md rule 8): `{ids, changes}` (max 100, per-id results) or `{filter: {conversation_id?, trigger?, before?}, changes: {read: true\|false} \| {seen: true}}` (meta only); see Bulk below |
| GET | /notifications/{notification} | notifications.show | A; `notifications:read` | one |
| PATCH | /notifications/{notification} | notifications.update | A; `notifications:write` | `{read: true\|false}`, `{seen: true}`; returns resource |
| POST | /notification-tests | notification-tests.store | A; `notifications:write` | body `{channel}`; sends test to me now, returns outcome resource synchronously (max 10 s); not stored beyond normal delivery rows (rest.md rule 5) |
| GET | /notification-preferences | notification-preferences.index | A; `notifications:read` | full matrix with `default`, `locked`, `source` |
| PATCH | /notification-preferences | notification-preferences.update | A; `notifications:write` | partial matrix `{ "preferences": [{trigger, channel, enabled}] }`, max 50 cells |
| DELETE | /notification-preferences | notification-preferences.destroy | A; `notifications:write` | reset: delete all my overrides (defaults apply); 204 |
| GET | /notification-settings | notification-settings.show | A; `notifications:read` | |
| PATCH | /notification-settings | notification-settings.update | A; `notifications:write` | |
| POST | /notification-focus | notification-focus.store | U + `workspace` | heartbeat `{visible, conversation_id?}`, every 30 s while visible and on visibility change (dashboard and first-party app foreground); 204 |
| GET | /push-config | push-config.show | U | `{vapid_public_key, key_id, allowed: true, kinds: ["web"]}` |
| GET | /push-subscriptions | push-subscriptions.index | U | my devices (across workspaces, user-scoped) |
| POST | /push-subscriptions | push-subscriptions.store | U | upsert by `endpoint_hash`; body `{kind?: "web", endpoint, keys:{p256dh, auth}, expiration_time?, device_label?, device_id?, old_endpoint?}` (`kind` default `web`; `device_id` used only for first-party app rows, see table); 201 new, 200 existing; `kind` `apns`/`fcm`: 422 `push_kind_unsupported` (reserved, post-MVP driver) |
| DELETE | /push-subscriptions/{push_subscription} | push-subscriptions.destroy | U | 204; other user's id 404 |
| GET | /notification-destinations | notification-destinations.index | A; `notifications:read` | workspace destinations (all members can read, credentials never shown) + my personal ones |
| POST | /notification-destinations | notification-destinations.store | A; workspace scope: permission `notification_destinations:manage` (ability group `settings:write`); member scope: `notifications:write` | create `slack_webhook`, `slack_dm` |
| GET | /notification-destinations/{destination} | notification-destinations.show | A; `notifications:read` | |
| PATCH | /notification-destinations/{destination} | notification-destinations.update | same as store | name, triggers, inbox_ids, include_ai_handled, show_previews, `enabled` (true re-activates `error`), `url` (replace webhook) |
| DELETE | /notification-destinations/{destination} | notification-destinations.destroy | same as store | deleting `slack_app` cascades `slack_dm` links; best-effort `auth.revoke` |
| POST | /notification-destinations/{destination}/tests | notification-destinations.tests.store | same as store | posts test message now, returns outcome resource synchronously (rest.md rule 5) |
| POST | /slack-notification-installs | slack-notification-installs.store | `notification_destinations:manage`, `S` (browser session only; bearer 403 `session_required`) | returns `{authorize_url, expires_at}` (state bound to session, 10 min) |

Push subscriptions and focus are `U`: browser session or first-party Dimmi app (mobile/desktop), never PATs or third-party OAuth (403 `first_party_only`). Rows registered by session carry `session_id`; by first-party app carry `oauth_grant_id`.

### Bulk (`PATCH /notifications`)

- `changes` accepts exactly single-PATCH fields: `read` (bool), `seen` (`true` only). Empty or unknown: 422.
- By ids: max 100; response 200 `{data: [{id, status: "updated"|"skipped"|"failed", error?}], meta: {updated, skipped, failed}}`. Other member's or workspace's id: `failed` `not_found`. Already in requested state: `skipped`.
- By filter (allowed here: own data, reversible, no confirmation): `filter` keys `conversation_id` (`conv_`), `trigger`, `before` (ISO 8601; rows with `updated_at <= before`; omitted = now). `{}` = all mine. Always scoped to acting member. Response 200 meta only: `{meta: {updated, unread_count, unseen_count}}`. Batched 5,000 rows per statement in one action; one `notifications.read` broadcast (`all: true` or `conversation_id`).
- "Mark all read": `{filter: {}, changes: {read: true}}`. "Bell opened": `{filter: {before: <newest seen updated_at>}, changes: {seen: true}}`.

Web routes (`routes/web.go`; listed in rest.md Exceptions, not under `/api/v1`):

| Method | Path | Route name | Auth | Purpose |
|---|---|---|---|---|
| GET | /integrations/slack/notifications/callback | slack-notifications.callback | session | OAuth v2 code exchange, creates `slack_app` (+ `slack_webhook` from `incoming_webhook`), redirects to dashboard settings with `?result=` |
| GET | /notification-unsubscriptions | notification-unsubscriptions.create | `signed` | confirm page (link scanners safe), button POSTs same signed URL |
| POST | /notification-unsubscriptions | notification-unsubscriptions.store | `signed`, CSRF exempt | RFC 8058 one-click; body `List-Unsubscribe=One-Click` or form; idempotent; 200 |

Signed URL carries member + trigger + channel scope and HMAC in query (RFC 8058 needs self-contained POST URL; HMAC, not a stored secret token, so rest.md rule 12 not applicable). `List-Unsubscribe` header points at `https://{host}/notification-unsubscriptions?…signed`.

Dashboard deep link route (SPA, served by web catch-all, owned by 03/web): `dashboard.conversations.show` = `/w/{workspace}/conversations/{conversation}` with public IDs (`ws_`, `conv_`), optional `?message=msg_...`. Uses workspace public ID, not slug, so renames never break links. Settings link: `dashboard.settings.notifications` = `/w/{workspace}/settings/notifications`.

### Examples

`GET /api/v1/notifications?filter[status]=unread&limit=2`

```json
{
  "data": [
    {
      "id": "ntf_01jbdq3x9k2m4",
      "object": "notification",
      "trigger": "customer_replied",
      "title": "Bob Smith replied",
      "body": "Still broken after clearing cache. Screenshot attached.",
      "count": 3,
      "conversation": { "id": "conv_01jbc9x7", "object": "conversation", "number": 1042, "subject": "Checkout fails on Safari" },
      "message": { "id": "msg_01jbdq3w", "object": "message" },
      "actor": { "type": "contact", "id": "ctc_01jbc9", "name": "Bob Smith" },
      "url": "https://app.trydimmi.com/w/ws_01ja/conversations/conv_01jbc9x7?message=msg_01jbdq3w",
      "seen_at": null,
      "read_at": null,
      "created_at": "2026-11-01T10:00:00Z",
      "updated_at": "2026-11-01T10:02:11Z"
    }
  ],
  "meta": { "next_cursor": "eyJ...", "prev_cursor": null, "per_page": 2, "unread_count": 7, "unseen_count": 2 }
}
```

`PATCH /api/v1/notification-preferences`

```json
{ "preferences": [
  { "trigger": "new_conversation", "channel": "email", "enabled": false },
  { "trigger": "snooze_expired", "channel": "push", "enabled": false }
] }
```

Response (`GET` shape):

```json
{
  "data": {
    "object": "notification_preferences",
    "channels": ["in_app", "email", "push", "slack"],
    "available_channels": { "push": true, "slack": false },
    "matrix": [
      { "trigger": "new_conversation", "channel": "email", "enabled": false, "default": true, "locked": false, "source": "custom" },
      { "trigger": "system_alert", "channel": "in_app", "enabled": true, "default": true, "locked": true, "source": "default" }
    ]
  }
}
```

`PATCH /api/v1/notification-settings`

```json
{
  "timezone": "Europe/Berlin",
  "quiet_hours": { "enabled": true, "start": "22:30", "end": "07:30", "days": ["mon","tue","wed","thu","fri","sat","sun"], "bypass": ["ai_handoff_requested"], "summary": true },
  "paused_until": null,
  "show_previews": true,
  "include_ai_handled": false,
  "notify_unassigned_replies": false,
  "email_delay_seconds": null
}
```

Response adds `"effective_timezone": "Europe/Berlin"`, `"in_quiet_hours": false`, `"next_quiet_hours_change_at": "2026-11-01T21:30:00Z"`.

`POST /api/v1/push-subscriptions`

```json
{
  "kind": "web",
  "endpoint": "https://web.push.apple.com/QGuQyavXutnMH...",
  "keys": { "p256dh": "BOr8...65-byte-b64url", "auth": "k8Jx...16-byte-b64url" },
  "expiration_time": null,
  "device_label": "iPhone (Home Screen)"
}
```

```json
{ "data": { "id": "psub_01jbe0", "object": "push_subscription", "kind": "web", "platform": "apple", "device_label": "iPhone (Home Screen)", "vapid_key_id": "vk_2026_11", "created_at": "2026-11-01T10:00:00Z", "last_success_at": null } }
```

`POST /api/v1/notification-destinations` (workspace Slack channel by pasted URL)

```json
{ "scope": "workspace", "type": "slack_webhook", "name": "#support", "url": "https://hooks.slack.com/services/T000/B000/XXXX", "triggers": ["new_conversation", "ai_handoff_requested"], "inbox_ids": null, "show_previews": true }
```

```json
{ "data": { "id": "ndst_01jbe4", "object": "notification_destination", "scope": "workspace", "type": "slack_webhook", "name": "#support", "url_hint": "hooks.slack.com/services/T000/B000/…XXXX", "triggers": ["new_conversation","ai_handoff_requested"], "inbox_ids": null, "status": "active", "last_error": null, "last_delivered_at": null, "created_at": "2026-11-01T10:00:00Z" } }
```

Create validates host + path shape only, never posts (no channel noise). `POST /notification-destinations/{id}/tests` posts on demand:

```json
{ "data": { "object": "notification_destination_test", "destination_id": "ndst_01jbe4", "status": "sent", "provider_status": 200, "error": null, "latency_ms": 412 } }
```

`POST /api/v1/notification-tests` `{ "channel": "push" }` returns 201 `{ "data": { "object": "notification_test", "channel": "push", "deliveries": [ { "target": { "id": "psub_01jbe0", "device_label": "iPhone (Home Screen)" }, "status": "sent", "provider_status": 201, "latency_ms": 840 } ] } }`. Destination test same status (201).

`PATCH /api/v1/notifications/ntf_01jbdq3x9k2m4` `{ "read": true }` returns 200 notification resource (`read_at` set); same value again: 200 no-op, no broadcast.

`PATCH /api/v1/notifications` `{ "filter": { "conversation_id": "conv_01jbc9x7" }, "changes": { "read": true } }` returns 200 `{ "meta": { "updated": 3, "unread_count": 4, "unseen_count": 0 } }`.

`POST /api/v1/notification-focus`: `{ "visible": true, "conversation_id": "conv_01jbc9x7" }` returns 204.

### Error codes

| Status | Code | When |
|---|---|---|
| 403 | `member_actor_required` | token not tied to member |
| 403 | `first_party_only` | push/focus endpoints with PAT or third-party OAuth token (`U` routes; `00-foundation/api.md`) |
| 403 | `session_required` | any bearer (incl. first-party app) on `POST /slack-notification-installs` (`S`: OAuth callback relies on browser cookie) |
| 403 | `missing_ability` | token lacks `notifications:*` |
| 403 | `forbidden` | member manages workspace destination |
| 404 | `not_found` | other member's notification, other user's subscription, other workspace |
| 409 | `slack_app_required` | `slack_dm` without workspace `slack_app` |
| 409 | `destination_limit_reached` | 25 workspace destinations |
| 410 | `slack_install_expired` | OAuth state older than 10 min |
| 422 | validation | Laravel shape |
| 422 | `preference_locked` | disable locked cell |
| 422 | `trigger_not_member_eligible` | member preference for destination-only trigger (`changelog_entry_published`) |
| 422 | `push_endpoint_not_allowed` | endpoint host not in allowlist or not https |
| 422 | `push_key_invalid` | p256dh not 65-byte P-256 point or auth not 16 bytes |
| 422 | `push_kind_unsupported` | `kind` `apns` or `fcm` (reserved for Dimmi mobile app native push; driver post-MVP). Checked before other fields |
| 422 | validation | PATCH `seen: false`, bulk `ids` and `filter` both or neither, empty `changes`, more than 100 ids |
| 422 | `vapid_key_mismatch` | client subscribed with old key (body `key_id` differs) |
| 422 | `invalid_webhook_url` | Slack URL host/path shape wrong |
| 422 | `slack_user_not_found` | member email not found in Slack team |
| 429 | `throttled` | `notification-tests` and destination `tests` over 6/minute per member |
| 503 | `push_unavailable` | VAPID keys missing and could not be generated |

## MCP tools

| Tool | Ability | Does |
|---|---|---|
| `dimmi_list_notifications` | `notifications:read` | unread by default, includes conversation compact + url; args `status`, `trigger`, `limit` |
| `dimmi_mark_notifications_read` | `notifications:write` | args `ids[]` or `conversation_id` or `all: true`; returns count. REST equivalent `PATCH /notifications` (`ids` or `filter`, `changes: {read: true}`); tool keeps task name (rest.md rule 15) |

No MCP tools for preferences or destinations (human settings, low agent value, avoids prompt-injection reconfiguring alerts).

## CLI

Owned by 09 for user CLI. 06 adds admin commands:
- `dimmi push:vapid [--force]`: print current VAPID public key; `--force` generates new pair (warns: invalidates every push subscription; clients re-subscribe on next dashboard load).
- `dimmi notifications:test --member=<email> --channel=push|email|slack|in_app`: sends test notification, prints per-target outcome.

## Events

Consumed:

| Event | Use |
|---|---|
| conversation.created | `new_conversation` |
| conversation.assigned | `conversation_assigned`, `new_conversation` (team routing) |
| message.created | `customer_replied` (contact message), `mentioned` (note with `MentionedMemberIDs`; recipients loaded from `message_mentions`) |
| message.updated | `mentioned` for `AddedMentionedMemberIDs` only (note edit added mentions) |
| ai.handoff_requested | `ai_handoff_requested` |
| conversation.unsnoozed | `snooze_expired` (reason expired) |
| conversation.read | mark that member's unread notifications for conversation read; broadcast `notifications.read` |
| conversation.deleted, conversation.marked_spam | delete unread notifications for conversation (in-app), cancel pending delayed emails/digest items |
| contact.merged | rewrite `data.contact` name on unread notifications (best effort, async) |
| (03 contact eraser hook, not event) | `EraseNotificationContactData` (`app/actions/notifications/erase_notification_contact_data.go`, registered on 03 eraser registry): blank `data` contact name, email, excerpt on notifications of erased contact's conversations; cancel pending deliveries |
| webhook_endpoint.failing, webhook_endpoint.disabled (10) | not consumed: 10 calls `Notify(system_alert)` directly with its notification classes (owners + admins); 06 never also listens (no double alert) |
| notification_destination.errored | `system_alert` |
| member.deactivated (00-accounts) | delete pending deliveries for member; member personal destinations set `disabled` |
| session.revoked (00-accounts, also on logout) | delete push subscriptions with that `session_id` (FK cascade covers row delete) |
| oauth_grant.revoked (09; first-party app grant) | delete push subscriptions with that `oauth_grant_id` (grant may be soft-revoked; FK cascade covers hard delete) |
| user.deleted (00-accounts) | delete all push subscriptions of user; pending deliveries skipped `member_inactive` |
| user.email_verified, user.updated (email changed) (00-accounts) | lift `system_mail_suppressions` for old/new address (`LiftSystemMailSuppression`) |
| feedback_post.created, feedback_comment.created (14) | 14 feedback triggers (registered by 14) |
| survey_response.completed (15-announcements-surveys) | `csat_negative` (registered by 15-announcements-surveys) |
| ai.action_call_approval_requested (07) | `ai_action_approval_requested` (registered by 07) |
| ai.action_connection_failed (07) | `system_alert` (07 class) |
| channel.errored, channel.disconnected, channel.connected (reason `recovered`) (05) | `system_alert` to owners/admins; recovered resolves |
| github_connection.errored, github_repository.webhook_status_changed (08) | `system_alert` (optional sources) |
| docs_sync_run.completed (08) | destination trigger `docs_suggestions_ready` when `suggestion_count > 0` |
| changelog_entry.published (13) | destination trigger `changelog_entry_published` when `announce = true` |

New events (add to catalog in `events-and-realtime.md`):

| Event | Emitted by | Realtime | Webhook | Notes |
|---|---|---|---|---|
| notification.created | 06 | yes, `member:{mem}` only | no | payload NotificationResource + `unread_count`, `unseen_count`; aggregated update re-emits with same id (client upserts) |
| notifications.read | 06 | yes, `member:{mem}` only | no | `{ids: [...] or conversation_id, unread_count}`; syncs bell across tabs/devices |
| notification_destination.errored | 06 | yes, `workspace:{ws}` (admins act) | no | destination id, type, error code |

Broadcast `notification.created` on `member:{mem}` never on workspace channel. Payload under 10 KB guaranteed (data max 4 KB).

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| DispatchNotifications | `notifications` | listener `SendMemberNotifications` per source event | 5, backoff 2 s, 10 s, 30 s, 2 m, 5 m | unique by `event_id` (River UniqueOpts ByArgs) |
| DeliverNotification | `notifications` | per member x channel (push, email, slack) after in-app row persisted; delayed email `ScheduledAt` | push/slack 3 (5 s, 20 s, 60 s); email 5 (mail errors) | unique by `(notification_id, channel, event_id)` |
| DeliverDestinationNotification | `notifications` | per workspace destination x event | 3 (5 s, 30 s, 2 m), honor Retry-After up to 60 s | unique by `(destination_id, event_id)` |
| SendNotificationDigest | `notifications` | first `batched` item schedules at window end | 3 | unique by `(member_id or destination_id, channel, window_start)` |
| SendQuietHoursSummary | `notifications` | scheduled at member quiet-hours end when items suppressed | 3 | unique by `(member_id, window_end)` |
| PruneNotifications | `maintenance` | periodic daily 03:10 UTC: delete notifications older than 90 days, deliveries older than 14 days, batches of 5,000. Not `RetentionGuard`-guarded (decided: member alerts are derived copies of conversation data, not source records; legal hold applies to conversations) | default | periodic (leader) |
| PrunePushSubscriptions | `maintenance` | periodic daily 03:20 UTC: delete `expires_at < now`, `consecutive_failures >= 10 AND last_success_at < now - 30 days`, unused 120 days | default | periodic (leader) |

Stale guard: DeliverNotification drops (status `skipped`, reason `stale`) push/slack older than 10 minutes since event (pointless late alert); email allowed up to 2 hours.

Queue: dedicated `notifications` queue (concurrency default 20, added by foundation) isolates 10 s budget from bulk `default` jobs. Every 06 job except prune runs there; `SendMemberNotifications` listener inserts on `notifications`, never `default`.

## Delivery flow

```
action tx: a.Events.Dispatch(ctx, tx, conversation.created)
  -> SendMemberNotifications (queued listener) InsertTx DispatchNotifications{event}
commit
DispatchNotifications worker (target start < 1 s after commit):
  load event subject, compute trigger(s), resolve recipients (audience/assignee/mentions)
  tx: upsert notifications rows (in_app), insert notification_deliveries (pending) per enabled channel,
      InsertTx DeliverNotification per member x channel (+ ScheduledAt for delayed email),
      InsertTx DeliverDestinationNotification per matching workspace destination,
      Dispatch notification.created per member (realtime after commit)
DeliverNotification worker:
  reload conversation + member + settings; run suppression pipeline; burst check;
  send (push to all member subscriptions in parallel, max 5 concurrent; email via Mailer; slack HTTP)
  update delivery rows, subscription health, destination health
```

## Config keys

`config/notifications.go`, env prefix `DIMMI_NOTIFICATIONS_`.

| Key | Default | Notes |
|---|---|---|
| notifications.enabled | true | false: only in_app rows, no external channels |
| notifications.queue_concurrency | 20 | `notifications` queue workers |
| notifications.push.enabled | true | |
| notifications.push.vapid_public_key / vapid_private_key | "" | env `_FILE` supported; empty: auto-generate on first boot, stored in `system_settings` (11) encrypted |
| notifications.push.vapid_subject | `mailto:` + `mail.from` address | must be `mailto:` or `https:` URL (Apple rejects else); boot validates |
| notifications.push.ttl_seconds | 14400 | 4 hours; `TTL` header |
| notifications.push.urgency | high | `Urgency` header for alerts; digests `normal` |
| notifications.push.allowed_hosts | `fcm.googleapis.com`, `updates.push.services.mozilla.com`, `*.push.apple.com`, `*.notify.windows.com` | registration allowlist; self-host may add |
| notifications.push.max_subscriptions_per_user | 20 | |
| notifications.focus.ttl_seconds | 75 | heartbeat every 30 s |
| notifications.cooldown_seconds | 30 | per member + conversation + trigger, push/slack |
| notifications.burst.push | `{threshold: 5, window: 60s}` | |
| notifications.burst.email | `{threshold: 3, window: 600s}` | |
| notifications.burst.chat | `{threshold: 5, window: 60s}` | slack personal |
| notifications.burst.destination | `{threshold: 10, window: 60s}` | workspace destinations |
| notifications.email.from | `notifications@` + `mail.mailers.system` from domain | sent only via `mail.mailers.system` (system stream, contracts.md two-stream rule); must differ from every inbox email address (doctor check) |
| notifications.system_mail.critical_bypass_per_day | 3 | critical auth mail sends allowed per suppressed address per 24 h |
| notifications.system_mail.alert_bounce_rate / alert_complaint_rate | 0.02 / 0.0005 | ops alert thresholds for system stream (no auto-pause) |
| notifications.email.customer_replied_delay_seconds | 120 | |
| notifications.stale.push_seconds | 600 | |
| notifications.stale.email_seconds | 7200 | |
| notifications.retention_days | 90 | |
| notifications.deliveries_retention_days | 14 | |
| entitlement `notifications.email_daily_per_workspace` | self-host -1 (unlimited), cloud 5000 | not a config key: entitlement catalog key (extensibility.md section 6), read via `Entitlements.Limit(ws, key)` (raise per customer with `entitlement_overrides`, no deploy). Over cap: digest only, `system_alert` to owners. Replaces `notifications.cloud.email_daily_cap_per_workspace` |
| services.slack.* | from 05 | client id/secret/signing secret; notification OAuth scopes listed in delivery.md |

## Permissions matrix

Policies call `authz.Can(principal, permission)` (extensibility.md section 4, P2); role columns derived from 00-accounts `roles.go`, never checked in 06 code. 06 registers permission `notification_destinations:manage` (group `settings:write`; owner, admin) in 00-accounts catalog. Recipient rules "owners and admins" (`system_alert`) resolve through permission `notification_alerts:receive` (group `notifications:read`; owner, admin), not role names; owner-locked email row resolves through `workspace:delete` holders (owner-only permission in catalog).

| Action | owner | admin | member | token ability |
|---|---|---|---|---|
| Read/mark own notifications | yes | yes | yes | `notifications:read` / `notifications:write` (new) |
| Own preferences + settings | yes | yes | yes | `notifications:read` / `notifications:write` |
| Push subscriptions, focus | yes | yes | yes | browser session or first-party app only (`U`); tokens never |
| Personal destinations (slack_webhook, slack_dm) | yes | yes | yes | `notifications:write` |
| Workspace destinations create/update/delete/test | yes | yes | no | `notification_destinations:manage` (group `settings:write`) |
| Workspace destinations read (no secrets) | yes | yes | yes | `notifications:read` |
| Install Slack app (notifications) | yes | yes | no | `notification_destinations:manage`, browser session only (`S`) |
| Read other member's notifications/preferences | no | no | no | never |
| `dimmi push:vapid`, `notifications:test` | server operator | | | CLI on host |

## Performance budgets

| Metric | Budget |
|---|---|
| DispatchNotifications start after commit | p95 under 1 s (River LISTEN-triggered fetch) |
| DispatchNotifications duration, 50-member audience | p95 under 150 ms |
| Push send per subscription | timeout 10 s (connect 5 s), p95 provider accept under 1.5 s |
| Slack post | timeout 10 s, p95 under 1.5 s |
| Notifications index p95 | under 30 ms (index-backed, member partial index) |
| Unread count | single partial-index count capped `LIMIT 101` |
| Focus heartbeat write | under 5 ms (Valkey `SET EX`) |
| Rows per event | at most (recipients x enabled channels) delivery rows; preference-off rows not written |

## Observability

Metrics (Prometheus):
- `dimmi_notifications_created_total{trigger}`
- `dimmi_notification_deliveries_total{channel, status, reason}`
- `dimmi_notification_latency_seconds{channel, trigger}` histogram, event `occurred_at` to provider accept. Alert when p95 over 8 s for 10 minutes.
- `dimmi_push_responses_total{platform, status}`
- `dimmi_push_subscriptions_pruned_total{reason}` (`gone`, `expired`, `vapid_mismatch`, `failing`, `logout`)
- `dimmi_notification_destination_errors_total{type, code}`
- `dimmi_notification_digests_total{channel}`

Logs: one line per DeliverNotification with workspace, member, notification id, channel, status, reason, latency. Never log push endpoint full URL (log host + sha256 prefix), webhook URLs, bot tokens, message excerpts at info.

Traces: span per dispatch and per delivery, linked to source event id.

`dimmi doctor` checks: VAPID keys present and valid P-256, `vapid_subject` valid, notification From not equal to inbox address, push hosts reachable through `a.HTTP` (framework httpclient) (HEAD on `fcm.googleapis.com`), Slack scopes when slack app configured.

## Security and privacy

- Recipients resolved from current membership at fan-out and re-checked at send. Removed member gets nothing after removal.
- `push_subscriptions` user-scoped, not tenant table (one browser subscription serves all workspaces of user, like `sessions`). Queries always filter `user_id`; CI tenant lint has explicit allowlist entry. Payload only includes workspace data for workspaces user is active member of at send time.
- Push endpoint is client-supplied URL: SSRF vector. Only `https`, host allowlist, sent through `a.HTTP`, no redirects followed.
- `auth` secret encrypted at rest; `p256dh` public. Payload encrypted per RFC 8291 (aes128gcm), so push service never sees content.
- Push payload rule (every kind: `web` now, `apns`/`fcm` later; extensibility.md section 12): payload carries ids (`ntf_`, `conv_`, `ws_`), `trigger`, `unread_count`, `navigate` URL + optional `title`/`body` (omitted or generic when `show_previews = false`). Never message bodies beyond 140-char excerpt, never contact email/phone, never attachments. App/SW fetches details via API with own credentials. Plaintext never over 3,000 bytes (APNs 4 KB hard limit, FCM 4 KB data). Content level becomes user preference later (`notification_settings` column, additive).
- Logout deletes subscriptions registered by that session (shared device); first-party app grant revoke deletes that grant's subscriptions. Password change and "log out all sessions" delete all user subscriptions.
- Push registration only via `U` guard (session or first-party app). PATs and third-party OAuth cannot register devices (`first_party_only`): stolen integration token cannot redirect member alerts to attacker endpoint.
- Slack webhook URLs and bot tokens are secrets: `credentials_encrypted`, shown as `url_hint` only, redaction patterns `hooks.slack.com/services/`, `xoxb-`.
- Customer content in notifications is untrusted: Slack mrkdwn escaped (`&`, `<`, `>`; blocks `<!channel>`, `<@U..>` injection), email HTML-escaped via templ, push plain text. Excerpts truncated to 280 chars on grapheme boundary.
- `show_previews=false`: push/email/chat say "New message from Bob Smith" without body (lock screen privacy).
- Unsubscribe URLs (`/notification-unsubscriptions`) signed (HMAC, no expiry), scoped to member + trigger + channel, only ever disable (never enable), safe against link prefetchers (GET shows page, POST acts).
- Notification emails carry `Auto-Submitted: auto-generated`, `X-Auto-Response-Suppress: All`, `X-Dimmi-Notification: 1`, Message-IDs under `@notifications.<host>`; 04 must drop inbound mail replying to these (loop guard).
- Slack OAuth `state` random 32 bytes bound to session + workspace, single use, 10 min.
- Audit log: workspace destination create/update/delete, Slack install/uninstall. Personal preferences not audited.

## Metering impact

None. Notifications to members are not messages to contacts: never substantive replies (D2), never activate billing (D1), never count toward outbound fair use (D7). Notifications keep flowing when spend cap hit (owner must learn about traffic). Suspended workspace (`suspended_at`): no notifications at all.

## Dependencies

| Spec | Need |
|---|---|
| 00-foundation + framework | events, realtime member channel `member:{mem}` (centrifuge namespace `member`, events-and-realtime.md), `httpclient` (`a.HTTP`), `mail` (`mail.mailers.system`), `encryption.KeyManager`, `routing` signed URLs, Valkey (`cache.Store`, `ratelimit.Limiter`), `notifications/channels/webpush` (webpush-go), River, registry enums (`notifications.trigger`), `Entitlements` contract + catalog (key `notifications.email_daily_per_workspace`); add `notifications` queue; add `mem_`, `ntf_`, `psub_`, `ndst_`, `ndlv_` prefixes; add abilities `notifications:read`, `notifications:write` |
| 00-accounts | sessions (`session.revoked` on logout/revoke, FK cascade; `sessions.device_id` copied to push rows), `member.deactivated`, `user.deleted`, `users.timezone`, `email_verified_at`; `auth:user` guard (session or first-party app); permission catalog + `authz.Can` (06 registers `notification_destinations:manage`, `notification_alerts:receive`) |
| 09-agent-native | hard dependency: `oauth_grants` table (FK from `push_subscriptions.oauth_grant_id`; 09 migration before 06), `oauth_grant.revoked` event (listener `DeleteGrantPushSubscriptions`; payload carries `grant_id`, `device_id`), grant `device_id` (authorize param) checked against push body `device_id`. Without 09 shipped, app rows cannot exist (session rows only) |
| 03-shared-inbox | `message_mentions` rows written in note tx (create and edit); `message.created.MentionedMemberIDs`, `message.updated.AddedMentionedMemberIDs`; `mem_` member ids; `conversation.read` event; dashboard route `/w/{ws}/conversations/{conv}` |
| 04-email-channel | drop inbound replies to notification Message-IDs; doctor check From vs inbox addresses; route system-stream provider/SES events (tag `stream=system`) to `RecordSystemMailFeedback` (04 outbound.md 7.3) |
| 05-chat-app-channels | `channel.errored`, `channel.disconnected`, `channel.connected` (recovered) events; `app/services/slack` client; cloud shared Slack app manifest adds notification scopes (`incoming-webhook`, `chat:write`, `im:write`, `users:read`, `users:read.email`) and redirect URL `/integrations/slack/notifications/callback`; self-host manifest generator same |
| 07-dimmi-agent | `ai_state='active'` set in same tx as conversation creation when agent will answer; `ai.handoff_requested` carries summary excerpt; registers `ai_action_approval_requested` trigger + class; `ai.action_connection_failed` system alert class |
| 08-help-center | destination trigger `docs_suggestions_ready` (`docs_sync_run.completed`), optional `system_alert` sources `github_connection.errored`, repo webhook `failing` |
| 16-importers | never dispatches record events; 06 guard skips `metadata.imported` |
| 10-webhooks | emits `webhook_endpoint.disabled`; uses `system_alert` via `Notify` |
| 11-self-host | `system_settings` storage for auto-generated VAPID keys; doctor hooks |
| 12-cloud-billing | may call `Notify(system_alert)` for threshold alerts (optional); cloud `Entitlements` driver serves `notifications.email_daily_per_workspace`; billing Mailables pass `system_suppression_guard` |
| 13-changelog | `changelog_entry.published{announce}` for destination trigger; `system_alert` via `Notify` (source errored, mailing blocked/failed, scheduled publish failed) |
| 14-feedback-roadmap | registers 4 feedback triggers + defaults; migration adds `notifications.feedback_post_id` + partial unique; events `feedback_post.created`, `feedback_comment.created` (+ mentions), moderation state for `condition_changed` |
| 15-announcements-surveys | registers `csat_negative`; `survey_response.completed` event with score, scale, `is_test`, conversation id |

## Contradictions with foundation (reported, not edited)

1. Resolved: member realtime channel `member:{mem}` (centrifuge namespace `member`, public `mem_` id; 03 aligned). Naming owned by events-and-realtime.md.
2. Foundation listener table: `SendMemberNotifications` on `default`, and lists `message.created (customer)` only. Mentions need notes (+ `message.updated` for edit-added mentions). Resolved: foundation adds dedicated `notifications` queue; listener row should read `notifications`.
3. Ability list lacks notification abilities; added `notifications:read`, `notifications:write`.
4. Ownership map lists 3 tables for 06; spec adds `notification_settings`, `notification_destinations`, `notification_deliveries`.
5. Ownership map implies `push_subscriptions` tenant table; push subscriptions are per browser per user, made user-scoped.

## Open questions (with recommended default)

1. Declarative Web Push `app_badge` placement not in W3C draft (WebKit-only). Default: set `mutable: true`, service worker sets badge via `navigator.setAppBadge(data.unread_count)`; revisit when spec settles.
2. Slack DMs on self-host need own Slack app. Default: pasted incoming webhook is the zero-setup path; DM via app documented as optional.
3. `new_conversation` email for workspaces with many members could be noisy. Default stays on (first users are solo); revisit when member count over 5 by showing "turn off email for new conversations" hint (frontend).
