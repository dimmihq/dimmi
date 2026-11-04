# 06 Notifications: channel delivery

Style: caveman. Per-channel payloads, provider facts (verified 2026-11-01), failure mapping. Sources at bottom.

## Copy (lang/en/notifications.json)

| Trigger | Title | Body (show_previews true) | Body (false) |
|---|---|---|---|
| new_conversation | `New conversation from {contact}` | excerpt | `#{number} in {inbox}` |
| conversation_assigned | `{actor} assigned #{number} to you` | `{contact}: {subject}` | `{contact}` |
| customer_replied | `{contact} replied` / `{contact} sent {count} messages` | latest excerpt | `#{number}` |
| mentioned | `{actor} mentioned you in #{number}` | note excerpt | `Internal note` |
| ai_handoff_requested | `{agent} handed #{number} to a human` | AI summary excerpt or last customer message | `{contact} needs a human` |
| team_joined | `{actor} added you to {team}` | `Conversations for {team} now show in your sidebar` | same |
| note_reacted | `{actor} reacted {emoji} to your note` | note excerpt | `#{number}` |
| side_conversation_replied | `{from} replied to a side conversation on #{number}` | excerpt | `#{number}` |
| data_export_ready | `Your export is ready` | `{row_count} conversations, available for {days} days` | same |
| snooze_expired | `#{number} is back` | `Snoozed conversation with {contact} reopened` | same |
| system_alert | per kind, e.g. `Slack destination #support stopped working` | error explanation + fix step | same |
| digest | `{n} new notifications` | top 5 lines `#{number} {contact}: {title}` | same without excerpts |
| system_alert `system_mail.suppressed` | `Emails to {email} are bouncing` | `Update your address or ask an admin. Dimmi stopped emailing it.` | same |

Triggers registered by 14 and 15-announcements-surveys (and 13 destination posts) ship their own copy keys in `lang/en/notifications.json` under their trigger name (owner spec writes copy; 06 renders).

`{contact}`: contact name, else email, else `Visitor #{short id}`. Deleted contact: `Deleted contact`. Former member actor: `Former teammate`. Excerpt: plain text from Markdown body (`markdown.PlainText`, framework), whitespace collapsed, 280 graphemes max (push 140), ellipsis.

Multi-workspace users: prefix title with `[{workspace name}] ` on push, email subject, Slack personal when user active in more than one workspace.

## In-app (database channel)

- Upsert `notifications` row (aggregation rules in README), commit, then broadcast `notification.created` on `member:{mem}` (centrifuge namespace `member`) with NotificationResource + counts.
- `notifications.read` broadcast after any read change (bell sync across tabs and devices).
- Never suppressed except `preference_off` (only non-locked triggers) and `conversation_gone`.

## Web Push

### Protocol

- RFC 8030 HTTP push: `POST {endpoint}` with headers `TTL`, `Urgency`, `Topic`, `Content-Encoding: aes128gcm`, `Authorization: vapid t=<jwt>, k=<public key b64url>` (RFC 8292).
- Payload encryption RFC 8291 (ECDH P-256 + HKDF + AES-128-GCM, record size 4096) and VAPID via `github.com/SherClockHolmes/webpush-go` v1.4.x (MIT), wrapped by framework `notifications/channels/webpush` (`Options`: `HTTPClient` = `a.HTTP.StdClient()`, `TTL`, `Urgency`, `Topic`, `RecordSize`, `VapidExpiration`, `Subscriber`). Own crypto removed (research/build-vs-reuse.md 3.7). RFC 8291 appendix vectors run in framework tests.
- VAPID JWT (library): ES256, `aud` = endpoint origin, `VapidExpiration` = now + 12 h (Apple rejects more than 1 day ahead), `Subscriber` = `notifications.push.vapid_subject` (Apple returns 403 `BadJwtToken` if not `mailto:` or URL). webpush-go signs per request (no JWT cache needed at our volume; Apple's once-per-hour guidance is about refresh frequency per audience, still met because each JWT lives 12 h) [verify per-request signing acceptable to Apple under burst; fallback: cache signed header per audience in Valkey 1 h].
- `Topic`: `c` + first 31 chars base64url of sha256(member id + conversation id) (32 char max, URL-safe base64 alphabet). Push service replaces undelivered message with same topic while device offline: phone wakes to 1 notification per conversation, not 10.
- `Urgency: high` for alerts, `normal` for digests and summaries. `TTL: 14400`.
- Plaintext payload max 3,000 bytes before encryption (Apple limit 4 KB; encryption overhead ~100 bytes; margin for safety). Builder trims body first, then title, then drops `data.excerpt`.
- Payload rule (binding for all push kinds incl. later `apns`/`fcm`, README Security): ids (`notification_id` `ntf_`, `conversation_id` `conv_`, `workspace_id` `ws_`) + `trigger` + `unread_count` + optional title/body only. Client fetches everything else (`GET /notifications/{ntf}`, conversation) with own token. Later APNs/FCM builders reuse `PushPayload` struct (`app/services/push/payload.go`) so rule holds per kind; unit test asserts every builder output under 4 KB with max-length inputs.

### Payload (Declarative Web Push compatible + service worker)

```json
{
  "web_push": 8030,
  "notification": {
    "title": "New conversation from Bob Smith",
    "body": "Checkout fails on Safari after I add a coupon…",
    "navigate": "https://app.trydimmi.com/w/ws_01ja/conversations/conv_01jbc9x7",
    "lang": "en",
    "dir": "auto",
    "tag": "conv_01jbc9x7",
    "silent": false,
    "data": {
      "notification_id": "ntf_01jbdq3x9k2m4",
      "workspace_id": "ws_01ja",
      "conversation_id": "conv_01jbc9x7",
      "trigger": "new_conversation",
      "unread_count": 4
    }
  },
  "mutable": true
}
```

- `web_push: 8030` + `notification.title` + `notification.navigate` = valid declarative message (Safari 18.4+ Home Screen apps show it without service worker).
- `mutable: true`: browsers with service worker get `push` event; SW shows same notification (must always show; Safari revokes permission on invisible pushes) and sets app badge from `data.unread_count`.
- Browsers without declarative support (Chrome, Firefox, Edge) parse same JSON in SW. One payload for all.
- Never send push without visible notification (no badge-only or sync pushes).

### Platform rules

| Platform | Rule | Server behavior |
|---|---|---|
| iOS/iPadOS 16.4+ | web push only for Home Screen web apps; permission needs user gesture | none server side; `GET /push-config` returns `requires_home_screen_on_ios: true` hint for UI |
| iOS 18.4+ | Declarative Web Push | payload above |
| Safari macOS 16+ | works in browser | same |
| Apple endpoints | `*.push.apple.com`, HTTP/1.1 or HTTP/2, SNI required | `a.HTTP` default transport (SNI on) |
| Chrome/Edge/Firefox | FCM, WNS, Mozilla autopush | same protocol |

### Response handling

| Status | Meaning | Action |
|---|---|---|
| 201 / 200 / 202 | accepted | delivery `sent`, subscription `last_success_at`, reset failures |
| 400 | bad request (`BadTtl`, `BadWebPushTopic`...) | `failed` reason `bad_request`, log error with reason, no retry, alert metric (our bug) |
| 403 `BadJwtToken` / `BadAuthorizationHeader` | our VAPID config wrong | `failed` reason `vapid_auth`; no retry; `system_alert` once per day to owners on cloud ops, `dimmi doctor` red |
| 403 `VapidPkHashMismatch` | subscription made with other VAPID key | delete subscription (reason `vapid_mismatch`) |
| 404 / 410 | subscription expired or unsubscribed | delete subscription immediately (reason `gone`) |
| 413 | payload too large | rebuild minimal payload (title + navigate only), retry once |
| 429 | rate limited | retry after `Retry-After` (cap 60 s), max 3 attempts |
| 500 / 502 / 503 / timeout | provider trouble | retry 5 s, 20 s, 60 s; then `failed` reason `provider_error`; `consecutive_failures++` |

## Email

- Mailable `MemberNotificationMail`, templ partial per trigger, plain-text + HTML. Sent only through `mail.mailers.system` (system stream: SES config set `dimmi-system`, Postmark system stream, tag `stream=system`); never through 04 customer stream.
- From: `notifications.email.from` (domain of `mail.mailers.system` from address; SPF/DKIM aligned for system stream) with display name `{workspace name} via Dimmi` (cloud) or `{workspace name}` (self-host). Reply-To: none. Replies go nowhere useful: footer says "Reply in Dimmi" link (reply-by-email notifications is post-MVP).
- Subject: `[{workspace}] {title}` (`#{number}` included when conversation).
- Threading per conversation: `Message-ID: <ntf_{id}@notifications.{host}>`, `References` and `In-Reply-To: <conv_{id}@notifications.{host}>` so Gmail/Apple Mail group notifications per conversation.
- Headers: `List-Unsubscribe: <https://{host}/notification-unsubscriptions?...signed>`, `List-Unsubscribe-Post: List-Unsubscribe=One-Click` (RFC 8058), `Auto-Submitted: auto-generated` (RFC 3834), `X-Auto-Response-Suppress: All`, `X-Dimmi-Notification: 1`, `X-Entity-Ref-ID: ntf_{id}` (stops Gmail collapsing distinct mails).
- Footer: "Open conversation" button (deep link), "Manage notifications" (settings deep link), "Unsubscribe from {trigger} emails" (signed one-click), "Unsubscribe from all email notifications in {workspace}" (signed, channel=email trigger=*).
- Delayed email: job `ScheduledAt = event + delay`; at send re-check `already_read` + `condition_changed`.
- Mailer errors: transient (SMTP 4xx, SES throttling) retry 5x over ~10 min; permanent (5xx recipient) `failed`, no retry.
- Bounce/complaint: system stream, owned by 06 (README "System stream feedback"): `system_mail_suppressions` row -> email channel `skipped` reason `email_suppressed` for every trigger + in-app `system_alert` `system_mail.suppressed` to member. Never 04 `email_suppressions` (that list is per workspace, customer stream).

## Slack

### Workspace channel: incoming webhook

- Pasted URL must match `https://hooks.slack.com/services/T.../B.../...` (also `hooks.slack-gov.com`). Stored encrypted.
- Or from Slack app OAuth (`incoming-webhook` scope): `oauth.v2.access` response `incoming_webhook.url`, `channel`, `channel_id` stored as `slack_webhook` destination automatically.
- Channel, username, icon cannot be overridden on incoming webhooks (fixed by app config). Name destination by channel.
- Message format (Block Kit, no interactive buttons: link buttons on apps without interactivity URL show warnings; use mrkdwn links):

```json
{
  "text": "New conversation from Bob Smith (#1042)",
  "blocks": [
    { "type": "section", "text": { "type": "mrkdwn", "text": "*New conversation* <https://app.trydimmi.com/w/ws_01ja/conversations/conv_01jbc9x7|#1042> from *Bob Smith* · Widget · Support" } },
    { "type": "section", "text": { "type": "mrkdwn", "text": ">Checkout fails on Safari after I add a coupon…" } },
    { "type": "context", "elements": [ { "type": "mrkdwn", "text": "<https://app.trydimmi.com/w/ws_01ja/conversations/conv_01jbc9x7|Open in Dimmi> · bob@example.com" } ] }
  ],
  "unfurl_links": false,
  "unfurl_media": false
}
```

- `text` fallback always set (notifications, accessibility). Escape `&` `<` `>` in all customer text. Never emit `<!here>`, `<!channel>`, `<@U...>` from customer content.
- Rate: Slack documents roughly 1 message per second per incoming webhook with short bursts. Destination burst rule (10/min then digest) keeps far under. 429: honor `Retry-After`.

### Slack app (bot) for DMs

- OAuth v2 install by admin: scopes `incoming-webhook`, `chat:write`, `im:write`, `users:read`, `users:read.email`. Token `xoxb-` stored encrypted in `slack_app` destination.
- Member link (`slack_dm`): `users.lookupByEmail` with member account email; found: store `slack_user_id`. Not found: 422 `slack_user_not_found` (no manual user id; prevents DMing wrong person).
- Send: `conversations.open` (users=U...) once, cache channel id in `config.slack_dm_channel_id`; then `chat.postMessage` with same blocks.
- Web API errors: `ok:false` body. `invalid_auth`, `account_inactive`, `token_revoked`, `missing_scope`: mark `slack_app` `error`, all child `slack_dm` effectively paused, `system_alert` admins. `user_not_found`, `cannot_dm_bot`, `user_disabled`: mark that `slack_dm` `disabled`, in-app note to member. `ratelimited` (HTTP 429): honor `Retry-After`.
- Uninstall: Slack `app_uninstalled` / `tokens_revoked` events arrive via 05's Slack events endpoint; 05 dispatches to 06 action `MarkDestinationErrored` for `slack_app` destinations matching team id.

### Incoming webhook error mapping

| Response | Slack error | Action |
|---|---|---|
| 200 `ok` | | `sent` |
| 400 | `invalid_payload`, `no_text`, `too_many_attachments` | our bug: `failed`, no retry, log error |
| 403 | `action_prohibited`, `posting_to_general_channel_denied`, `invalid_token`, `team_disabled` | destination `error`, `system_alert`, stop sending |
| 404 | `no_service`, `no_service_id`, `no_team`, `channel_not_found`, `user_not_found` | destination `error` (revoked or channel gone), `system_alert` |
| 410 | `channel_is_archived` | destination `error` with hint "channel archived" |
| any | `no_active_hooks` | destination `error` |
| 429 | | retry after `Retry-After` |
| 5xx / timeout | | retry 3x, then `failed`; 20 consecutive failures over 1 hour: destination `error` |

## Workspace destination posts

- Same formats; title prefixed by trigger label. `include_ai_handled=false`: skip `new_conversation` and `customer_replied` while `ai_state='active'`, `ai_handoff_requested` always posts.
- Burst digest: over 10 posts per minute per destination, rest collapse into one post at minute end: `15 more new conversations` + up to 10 lines with links + "Open inbox" link.
- Not affected by member focus, quiet hours, pause.

## Digest

- Burst: first `threshold` items in window send normally; later items `batched`; first batched schedules `SendNotificationDigest` at window end (unique). Digest re-checks each item (`already_read`, `condition_changed`, `conversation_gone`); zero left: skip.
- Push digest: one notification `"{n} new notifications"`, navigate to inbox (or conversation if single), tag `digest`.
- Email digest: list up to 20 items, grouped by trigger, "and {k} more".
- Quiet hours summary: at window end, if any items suppressed by `quiet_hours` remain unread: one email (if email enabled for any of those triggers) listing them. No push summary (avoid morning buzz storm).

## Sources

- Apple, Sending web push notifications in web apps and browsers: https://developer.apple.com/documentation/usernotifications/sending-web-push-notifications-in-web-apps-and-browsers
- WebKit, Meet Declarative Web Push (Safari 18.4): https://webkit.org/blog/16535/meet-declarative-web-push/
- W3C Push API (declarative push message, `web_push: 8030`, `mutable`): https://w3c.github.io/push-api/
- RFC 8030, RFC 8291, RFC 8292 (VAPID), RFC 8058 (one-click unsubscribe), RFC 3834 (Auto-Submitted).
- Slack, Sending messages using incoming webhooks (errors, no channel override, Block Kit): https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks
- SherClockHolmes/webpush-go v1.4.0 (MIT; deps golang-jwt/jwt/v5, x/crypto), research/build-vs-reuse.md 3.7.
