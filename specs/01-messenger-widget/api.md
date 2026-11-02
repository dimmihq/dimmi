# 01 Messenger widget: API detail

Style: caveman. Shapes follow `specs/00-foundation/api.md` (`data`, `object`, prefixed ids, ISO UTC, `meta.next_cursor`). Every widget response carries header `Dimmi-Server-Time` and JSON `server_time` where listed. All `/widget/v1` responses: `Cache-Control: no-store` unless stated.

## Common widget request rules

- `Origin` header checked against `allowed_origins` on every request (preflight included). Missing `Origin` on browser-only handshake routes (`POST /sessions`, `GET`/`PATCH /session`) with non-empty allowlist: 403 `origin_not_allowed`. Every request feeds install tracker (README "Install tracking"). Headless/native clients: allowlist empty, or later mobile app keys (post-MVP: mobile SDKs).
- Auth: `Authorization: Bearer dmi_ws_<43 chars>`. Token never in query string. Exception: none (polling uses header too).
- Body limit 1 MB (JSON). Upload body limit `max_attachment_mb`.
- Strings NFC-normalized; control chars except `\n` `\t` stripped; bidi override chars (U+202A..U+202E, U+2066..U+2069) stripped from `name`.
- `Idempotency-Key` accepted on POST (foundation); `client_id` is primary dedupe for messages.

## GET /widget/v1/apps/{app}/config

Public, CDN cacheable. `Cache-Control: public, max-age=60, s-maxage=60, stale-while-revalidate=600`, `ETag: "wgt_...:14"`, `Vary: Origin`.

```json
{
  "data": {
    "object": "widget_config",
    "app_id": "wgt_01jbd2k7m3q8r5t6v9w0x1y2z3",
    "config_version": 14,
    "workspace": { "name": "Acme", "logo_url": "https://.../logo.png" },
    "appearance": {
      "primary_color": "#5B5BD6", "theme": "auto", "position": "right",
      "padding_x": 20, "padding_y": 20,
      "launcher": { "style": "bubble", "label": "", "icon": "chat" },
      "font": "system"
    },
    "texts": {
      "greeting_title": "Hi there 👋",
      "greeting_body": "Ask us anything. We read every message.",
      "office_hours_text": "Mon to Fri, 9 to 17 CET",
      "offline_message": "We're away. Leave a message and we'll email you.",
      "email_prompt": "Where should we reply?",
      "composer_placeholder": "Write a message..."
    },
    "locale": "auto",
    "features": {
      "attachments": true, "max_attachment_mb": 10,
      "help_tab": true, "ai_agent": true,
      "collect_email": "offline", "require_email_when_offline": true,
      "allow_anonymous": true,
      "changelog": { "enabled": true, "title": "What's new", "latest_announced_at": "2026-10-30T08:00:00Z", "spotlight": { "id": "cle_01jb...", "title": "CSV export is 10x faster", "summary": "...", "cover_url": "https://.../cover.png", "published_at": "2026-10-30T08:00:00Z" } },
      "feedback": { "enabled": true, "boards_count": 2, "roadmap": true },
      "intercom_shim": false
    },
    "branding": { "show": true, "locked": true, "url": "https://trydimmi.com/?utm_source=widget&utm_medium=badge&ref=acme" },
    "realtime": { "url": "https://app.trydimmi.com/connection/uni_sse", "token_url": "https://app.trydimmi.com/widget/v1/realtime-tokens", "polling_interval_ms": 5000 },
    "engage": { "active": true, "anonymous": true, "version": 7, "refresh_interval_ms": 300000 },
    "honeypot_field": "website"
  }
}
```

`ai_agent` true when 07 agent active for widget's inbox. Blocks owned by other specs (README "Config resource blocks"): `features.changelog` (13, only when `changelog_tab_enabled`), `features.feedback` + `texts.feedback_tab_label` (14, only when `feedback_tab_enabled`), `engage` (15; cache key includes `engage_settings.version`). `features.intercom_shim` (01, 16 switchers): loader installs `window.Intercom` shim. Agent action button labels (`action_confirm`, `action_cancel`, 07 lang keys) ship in config `strings` for widget locale. Errors: 404 `app_not_found`, 403 `widget_disabled`, `origin_not_allowed`, `workspace_suspended`.

## GET /widget/v1/apps/{app}/availability

`Cache-Control: public, max-age=30`.

```json
{
  "data": {
    "object": "widget_availability",
    "online": true,
    "reply_time": "minutes",
    "reply_time_text": "Usually replies in a few minutes",
    "team": [{ "name": "Kim", "avatar_url": "https://.../kim.png" }],
    "server_time": "2026-11-01T10:00:00Z"
  }
}
```

`online` = at least one active member in widget inbox's default team (or workspace when none) with availability `online` and realtime connection seen within 60 s (centrifuge presence on Valkey, foundation rule). `reply_time` `auto` resolves: online -> `minutes`, else `hours`. `team` max 3, members with avatar, ordered by last activity; first name or display name only.

## Boot handshake

Decision table and rules: README "Boot handshake" (binding). No `boot` route. Shared response: `widget_session` resource (README example): `id`, `object`, `token` (only when issued by `POST /sessions`), `kind`, `test`, `expires_at`, `contact` (widget-safe summary), `identity {status, error, warnings}`, `unread_count`, `open_conversation_ids`, `realtime {url, channel, token_url}`, `changelog {last_seen_at, unread_count}` (13, tab enabled only), `feedback_unread_count` (14, tab enabled only, capped 99), `config_version` (SDK refetches config when newer than cached), `test_quota_exceeded`, `server_time`. `identity.status` in (`anonymous`, `verified`, `unverified`); `signed_out` and `failed` are SDK-side states only (`identity_changed` event), never in a 2xx body.

### POST /widget/v1/sessions

Starts session. App auth (`app_id` body), origin checked. Three kinds by body:

| Body | Kind | PoW | Response |
|---|---|---|---|
| `{app_id}` (no `user_jwt`, no `test_token`) | anonymous lead. Lazy: first write (open conversation, email capture), never on page view | rules below | 201 |
| `{app_id, user_jwt, previous_token?}` | identified (02 `VerifyIdentityToken` + `IdentifyVisitor`) | never | 201; 200 when `previous_token` is identified session with same `sub` (reused, extended, `token` omitted) |
| `{app_id, test_token}` | test lead (`test = true`) | never | 201 |

`user_jwt` with `test_token`: 422 (`test_token` preview only).

Request fields:

| Field | Type | Notes |
|---|---|---|
| app_id | string required | `wgt_` |
| user_jwt | string or null | identity token from page's identity source (sdk-contract "Identity sources"); wire name only, never an SDK option. 02 verifies. Max 8 KB |
| previous_token | string or null | stored `dmi_ws_` token. Body only. Valid, unexpired, same widget: anonymous -> lead upgrade or merge (02), old revoked `identity_switch`; identified other `sub` -> old revoked `identity_switch`. Invalid, expired, revoked, other widget: ignored silently |
| test_token | string or null | from dashboard preview (`dmi_wtt_...`); one-time |
| user | object or null | unverified hints `{name, email, user_id}` (02 R5) |
| page | `{url, title, referrer}` | |
| pages | array | up to 10 pages buffered client side before session |
| locale, timezone | string | validated BCP 47 / IANA; invalid ignored |
| context | object | page context, max 4 KB |
| pow | object or null | anonymous only |
| website | string | honeypot |
| sdk | `{name, version}` | logged, metrics. `name` in (`dimmi-browser` (script tag + `@dimmi/browser`), `dimmi-react`, `dimmi-nextjs`); unknown names accepted, bucketed `other` in metrics |

Failure never has side effects: invalid JWT returns 401 with 02 code, `previous_token` session untouched, no contact created.

Anonymous request:
```json
{ "app_id": "wgt_...", "page": { "url": "https://acme.com/pricing" }, "locale": "en", "timezone": "America/New_York", "pow": null, "website": "" }
```

When PoW needed (anonymous only): `428 pow_required`. Challenge is ALTCHA shape (altcha-lib-go):
```json
{ "message": "Proof of work required.", "code": "pow_required", "challenge": { "algorithm": "SHA-256", "challenge": "4f1c...hex", "maxnumber": 500000, "salt": "9a2e...?expires=1793527320&ip=203.0.113.0", "signature": "c81b...hex" } }
```
Client finds `number` in `[0, maxnumber]` so `sha256(salt + number)` hex equals `challenge`, retries with `"pow": {"algorithm": "SHA-256", "challenge": "...", "number": 123456, "salt": "...", "signature": "..."}` (ALTCHA payload, JSON object). Server verifies HMAC signature, `expires` and `ip` salt params, then marks single use (Valkey `SET NX EX`). Median solve at `maxnumber` 500000 about 100 to 300 ms in Web Worker on mid phone.

201 (anonymous):
```json
{ "data": { "object": "widget_session", "id": "wss_01jb...", "token": "dmi_ws_...", "kind": "anonymous", "test": false, "expires_at": "2027-04-30T10:00:00Z", "contact": { "id": "ctc_01jb...", "object": "contact", "kind": "lead", "name": null, "email": null }, "identity": { "status": "anonymous", "error": null, "warnings": [] }, "unread_count": 0, "open_conversation_ids": [], "realtime": { "url": "https://app.trydimmi.com/connection/uni_sse", "channel": "widget_contact:ctc_01jb...", "token_url": "https://app.trydimmi.com/widget/v1/realtime-tokens" }, "config_version": 14, "server_time": "2026-11-01T10:00:00Z" } }
```

Honeypot `website` non-empty: 201 normal response, session `bot_score = 100` (silent).

Rate limits: `widget.rate.sessions_per_ip_hour` and PoW thresholds count anonymous creates only. Identified and test creates: per-IP request limit only.

### GET /widget/v1/session

Bearer. Anonymous session: 200 `widget_session` (`token` omitted), slides `expires_at` (write throttled 60 s with `last_seen_at`). Identified session: 409 `identity_proof_required`, nothing changed. 401 `session_expired`/`session_revoked`/`session_invalid` per auth:widget. Read-only otherwise: never revokes (safe method).

### PATCH /widget/v1/session

Bearer. `{ "user_jwt": "..." }`. Resume on page load and refresh mid-session (former refresh route). auth:widget admits expired, not revoked identified token here only. 02 verifies; `sub` must equal session `external_id` else 409 `identity_mismatch` (SDK then `POST /sessions` with `previous_token`). 200 `widget_session` with new `expires_at` (`verified_at + identity.session_ttl`), claims applied (zero writes when claims hash unchanged), token unchanged. Anonymous session: 409 `identity_mismatch` (use `POST /sessions` with `previous_token`). Invalid JWT: 401 02 code, session untouched.

### DELETE /widget/v1/session

204. Revokes reason `logout`, disconnects realtime streams of that session (`widget.session_revoked` then disconnect 4009), then 14 `RevokePortalSessions` (surface `widget`, same widget + contact). Idempotent: already revoked also 204. SDK sends it on `Dimmi.logout()` and when identity source answers `{token: null}` with stored identified session (02 R6).

## GET /widget/v1/contact, PATCH /widget/v1/contact

```json
{ "data": { "id": "ctc_...", "object": "contact", "kind": "lead", "name": "Ada", "email": "ada@example.com", "email_verified": false, "phone": null, "locale": "en", "custom_attributes": {} } }
```

PATCH body (lead only): `name` (1..120), `email` (RFC 5322 addr-spec, IDN domain to punycode, max 254), `phone` (E.164), `custom_attributes` (max 20 keys from widget, rules from data-model). Sets `contact_identities` email row `verified=false` only when no other contact holds same email identity; otherwise email stored on `contacts.email` only (no identity row, no merge). Every `custom_attributes` key written here is removed from `contacts.trusted_attribute_keys` in same tx (widget-set values never trusted; 07 agent actions bind only trusted keys). User contacts: 403 `identity_locked`. Email change rate: 5/day/session.

## GET /widget/v1/conversations

Query: `cursor`, `limit` (default 20, max 50), `filter[status]=open,closed`.

```json
{
  "data": [{
    "id": "conv_01jb...", "object": "conversation", "number": 1042, "status": "open",
    "subject": "Still can't export CSV", "last_message": { "id": "msg_...", "author": { "type": "member", "name": "Kim", "avatar_url": "..." }, "excerpt": "Fixed! Try again.", "created_at": "..." },
    "unread_count": 1, "ai_state": "handed_off", "created_at": "...", "last_message_at": "..."
  }],
  "meta": { "next_cursor": null, "prev_cursor": null, "per_page": 20 }
}
```

Scope: `contact_id = session.contact_id`, origin channel type in (`widget`,`email`,`api`), plus imported (`source.imported`, channel type `import`, 16) for identified sessions only (read-only history; `imported: true` in item), `deleted_at IS NULL`. Spam conversations included (visitor never learns spam flag). `ai_state` exposed (drives "AI answering" chip); `assignee` never.

## POST /widget/v1/conversations

```json
{ "client_id": "uuid-v4", "body": "Hi, export fails", "attachment_ids": [], "page": { "url": "...", "title": "..." }, "context": { "plan": "pro" }, "website": "" }
```

Optional `context` keys from SDK `startConversation(text, {context})` land in `source.context`; reserved key lifted to `source`: `announcement_id` (15, `ann_`; chat-format announcement, contact has receipt, item live or ended within 7 days, else 422 `announcement_not_replyable`; `StartWidgetConversation` passes `source.announcement_id` + event row `announcement.replied` to 03).

201 `{ data: conversation, meta: { message: <widget_message> } }`. Same `client_id` replay returns 200 with same objects. `email_required` 422 when rule applies (no rows created).

## GET /widget/v1/conversations/{conversation}/messages

Query: `before` cursor (older), `after` message id (newer), `limit` default 30 max 100. Returns `type = message` rows only, not deleted (deleted shown as `{id, object, deleted: true, body: null}` placeholders to keep order), oldest first within page. Attachment URLs signed 1 h. System message from 15 CSAT (`author.type = system`) adds `survey_request: {survey_id, response_id}`; widget renders survey card and answers via 15 `POST /widget/v1/surveys/{svy}/responses`. Agent message with 07 write action adds `ai: {confirmation: {action_call_id, title, text, status, expires_at}}` (README "Agent action card"); `ai.action_calls`, citations internals never.

## POST /widget/v1/conversations/{conversation}/messages

See README example. Rules:
- `client_id` uuid required. Dedupe key `widget:{session_id}:{client_id}` in `messages.dedupe_key`. Duplicate returns 200 with original.
- Body 1 to 64 KB after trim, or empty when `attachment_ids` non-empty. Max 10 attachments, each `uploaded_at` set, uploaded by same session, unattached. Otherwise 409 `attachment_not_uploaded` or 404.
- Conversation closed: 03 rules. Within reopen window: reopened, `meta.reopened = true`. Past window: new conversation linked, response `meta.redirected_from = "conv_old"`, `data.conversation_id` new. This route is widget's continue/reopen path for past conversations (listed by `GET /conversations?filter[status]=closed`); no separate `continue` route. Offline email continue link deep-links here via SDK (`#dimmi-conversation=conv_...`).
- Conversation deleted: 404; SDK starts new conversation with same draft.
- Imported conversation (16, channel `import`): never appended; creates linked new widget conversation, `meta.redirected_from` (same as past reopen window).
- Server stamps `metadata.widget {session_id, identity_verified}` (README Core table changes); client cannot set it (unknown body keys ignored).

## POST /widget/v1/conversations/{conversation}/typing-indicators

`{ "state": "started" }` or `"stopped"`. REST because uni-SSE stream is receive-only. 202, ephemeral (published to `conversation:{conv}` without history) (no stored row, no id). Throttled 1 per 2 s per session per conversation (extra calls 202 no-op). Never stored, never preview text.

## PATCH /widget/v1/conversations/{conversation}/read-state

Singleton. `{ "last_read_message_id": "msg_..." }`. Sets `read_at = now()` for member/agent `type=message` rows in conversation with `created_at <=` that message and `read_at IS NULL`; emits one `message.delivery_updated` per conversation batch (`status: read`, `message_ids`). 200 `{ "data": { "object": "widget_read_state", "conversation_id": "conv_...", "last_read_message_id": "msg_...", "unread_count": 0 } }`. Older id than current pointer: no-op, 200 current state. Message of other conversation: 422.

## POST /widget/v1/conversations/{conversation}/handoffs

Controller + action owned by 07 (`HandOffConversation`); 01 registers route `widget.conversations.handoffs.store` in `routes/widget.go`. 202 `{ "data": { "object": "handoff", "status": "requested" } }` (idempotent while `ai_state = handed_off`). No agent configured: 202, no-op (button still honored: conversation stays with humans).

## GET, PATCH /widget/v1/conversations/{conversation}/action-calls/{call}

Controller owned by 07 (`07-dimmi-agent/actions.md` HTTP API); 01 registers `widget.conversations.action-calls.show` / `.update` in `routes/widget.go`. Call must belong to conversation of session contact (else 404 `action_call_not_found`).

GET 200:
```json
{ "data": { "id": "aax_01jb...", "object": "action_call", "title": "Refund last invoice", "status": "pending_confirmation", "confirmation_text": "Refund last invoice: amount: $49.00", "expires_at": "2026-11-01T10:15:00Z" } }
```

PATCH `{ "status": "confirmed" }` or `{ "status": "canceled" }`. Requires session `kind = identified`, verified, not expired, identity equal to call snapshot. 200 same shape, status `confirmed` (or `pending_approval` when action needs teammate/admin approval) or `canceled`. Errors: 409 `action_call_not_pending` (already confirmed, canceled, superseded), 409 `action_call_expired`, 409 `identity_changed` (session user differs from snapshot or session anonymous). Idempotent: repeat of same status on already-confirmed call 200.

## Attachments

Same flow serves 14 feedback post/comment images (14 links the uploaded row; 03 orphan purge treats `feedback_post_id`/`feedback_comment_id` rows as attached).

Step 1 `POST /widget/v1/attachments`:
```json
{ "filename": "error.png", "content_type": "image/png", "size_bytes": 48211 }
```
201:
```json
{
  "data": {
    "id": "att_01jb...", "object": "attachment_upload",
    "status": "pending",
    "upload": { "method": "PUT", "url": "https://bucket.s3.../ws/att_01jb...?X-Amz-...", "headers": { "Content-Type": "image/png", "Content-Length": "48211" }, "expires_at": "2026-11-01T10:15:00Z" },
    "links": { "self": "https://app.trydimmi.com/widget/v1/attachments/att_01jb..." }
  }
}
```
Local disk driver (or S3 driver with `presign: false`): `upload.url` = `/widget/v1/attachments/{id}/content`, client sends same bearer header.

Step 2 upload. Direct route streams to disk with size cap; computes sha256 inline.

Step 3 `PATCH /widget/v1/attachments/{attachment}` `{ "status": "uploaded" }`. `status` derived from `attachments.uploaded_at` (null = `pending`, set = `uploaded`); only legal transition `pending` -> `uploaded` (other values 422 `invalid_transition`; already uploaded: 200 idempotent). Server `Stat` object; size must equal declared (else delete object, 422 `attachment_size_mismatch`); reads first 512 bytes to sniff type; type mismatch with block list: delete, 422 `attachment_type_blocked`; images: decode header for width/height, reject over 40 megapixels (`attachment_image_too_large`). Sets `uploaded_at`. Queues 03 `ScanAttachment` (computes sha256 for presigned uploads, malware scan driver). 200 `widget_attachment`.

Blocked types: executables (`application/x-msdownload`, `x-executable`, `x-mach-binary`, `x-sh`), `.exe .bat .cmd .scr .js .msi .dmg .apk .jar` by extension, `text/html`. SVG allowed as download only.

## POST /widget/v1/pings

```json
{ "page": { "url": "https://app.acme.com/settings", "title": "Settings", "referrer": "https://app.acme.com/" }, "context": { "app_version": "2.3.1" }, "visible": true }
```
202. Throttle 1 per 10 s per session (extra merged into latest). Updates `current_page`, `recent_pages` (dedupe consecutive same URL), `context` (merge, null deletes key), `last_seen_at` (60 s throttle). Without session (anonymous lazy): SDK buffers last 10 pages client side and sends with session create / first conversation.

## POST /widget/v1/realtime-tokens

Bearer. Empty body. 201 `{ "data": { "object": "realtime_token", "token": "eyJ...", "expires_at": "+60 s", "url": "https://app.trydimmi.com/connection/uni_sse", "channel": "widget_contact:ctc_..." } }`. Token = centrifuge connection JWT signed with broadcasting key: `sub` = `wss_` session id, `aud` = realtime connect only (REST guards reject it), `exp` 60 s (`widget.realtime_token_ttl`), `channels`/server-side subscription `widget_contact:{session contact}` only. Rate limit 30/min per session. Revoked or expired session: 401 per auth:widget.

## GET /connection/uni_sse (framework, not `/widget/v1`)

`?cf_connect=<url-encoded JSON {token, subs: {"widget_contact:ctc_...": {recover: true, offset, epoch}}}>`. Own small uni-SSE handler in `framework/broadcasting` built on centrifuge public API (`centrifuge.NewClient`, `ProtocolConnect`). `OnConnecting` verifies token, re-checks session not revoked, applies `widget.max_connections_per_session` (oldest stream disconnected with code 4004), returns server-side subscription; client-sent `subs` used only for recovery position of that same channel (other channels ignored). Frames: centrifuge JSON push (`connect` reply with `recovered`, publications with `offset`, `disconnect`). Recovery: Valkey history 300 publications / 10 min per channel; miss returns `recovered: false`, SDK refetches via REST. Revoke: `widget.session_revoked` publication then disconnect (custom code 4009). Headers: `Cache-Control: no-store`, CORS same as widget group. `cf_connect` redacted in access logs.

## GET /widget/v1/updates

Polling fallback when SSE blocked or buffered (corporate proxy, first-party proxy that buffers responses): SDK switches after 2 failed connects in 30 s or no first frame in 10 s.
Query: `since` opaque cursor (server-issued; first call omits = now).
```json
{ "data": [ { "event": "message.created", "id": "evt_...", "payload": { "...widget message..." } } ], "meta": { "cursor": "c_...", "poll_after_ms": 5000, "server_time": "..." } }
```
Built from DB (messages, conversations `updated_at`) for contact's conversations, not from realtime history (10 min). Max 100 events; more: `meta.has_more = true`, client polls immediately. Typing not in polling. `conversation.seen_by_team` synthesized from max member `conversation_reads.last_read_at` (03 table) past latest team message; same widget-safe payload `{conversation_id, seen_at}` (+ `seen_by` only with workspace `seen_receipt_shows_name`).

## Help endpoints

Owned by 08 (routes, controllers, resources, shapes: `08-help-center/rendering.md` widget section). 01 requirements on 08: accept `app_id` without session (anonymous visitors), resolve help center from widget `help_center_id`, origin check same as widget group, `Cache-Control: public, max-age=60` when no session header, only published public articles.
- `GET /widget/v1/help`, `GET /widget/v1/help/collections/{col}`, `GET /widget/v1/help/articles/{art}`, `GET /widget/v1/help/search?q=&app_id=wgt_...`, `POST /widget/v1/help/articles/{art}/feedback`.
- Intercom shim (16): `GET /widget/v1/help/articles?filter[intercom_id]=<id>` resolves imported article via 16 import records to `art_` (published public only; unknown: 404 `article_not_found`, shim opens Help home).
- `help_tab_enabled = false` or no help center: 404 `help_center_not_linked`.

## Dashboard: widget_settings resource

```json
{
  "data": {
    "id": "wgt_01jbd2...", "object": "widget_settings",
    "name": "Website", "enabled": true,
    "inbox": { "id": "inbox_...", "object": "inbox", "name": "Support" },
    "channel_connection_id": "chn_...",
    "appearance": { "...": "as config" },
    "texts": { "...": "as config" },
    "locale": "auto", "reply_time": "auto",
    "show_team_avatars": true, "show_availability": true,
    "allowed_origins": ["https://acme.com", "https://*.acme.com"],
    "allow_anonymous": true, "collect_email": "offline", "require_email_when_offline": true,
    "attachments_enabled": true, "max_attachment_mb": 10,
    "help_center_id": "hc_...", "help_tab_enabled": true, "changelog_tab_enabled": false, "feedback_tab_enabled": false,
    "intercom_shim": false,
    "show_branding": null, "branding": { "show": false, "locked": false },
    "config_version": 14,
    "last_boot_at": "2026-11-01T09:58:00Z", "last_boot_origin": "https://app.acme.com",
    "created_at": "...", "updated_at": "...",
    "links": { "self": ".../api/v1/widget-settings/wgt_...", "install": ".../install", "config": ".../widget/v1/apps/wgt_.../config" }
  }
}
```

POST body: `name`, `inbox_id` (default workspace default inbox), any settings field. PATCH: any subset; nested `appearance`/`texts` merge per key (null resets key to default). 422 examples: `allowed_origins.2: "Origin must be scheme://host[:port], no path."`, `appearance.primary_color: "Use #RRGGBB."`, `help_tab_enabled: "Link a help center first."`. Over entitlement `widget.max_per_workspace` (default 10): 422 `entitlement_limit_reached` `{key, limit, used}` (contracts.md shared code). Cloud free workspace `show_branding=false`: 403 `branding_locked`.

## Dashboard: install

`GET /api/v1/widget-settings/{widget}/install?framework=nextjs-app`

```json
{
  "data": {
    "object": "widget_install",
    "app_id": "wgt_...",
    "api_base": "https://app.trydimmi.com",
    "framework": "nextjs-app",
    "status": { "installed": true, "last_boot_at": "...", "last_boot_origin": "https://app.acme.com", "origins_seen": ["https://app.acme.com", "http://localhost:3000"], "origins_blocked": ["https://evil.example"] },
    "blocks": [
      { "kind": "command", "content": "npm i @dimmi/nextjs" },
      { "kind": "file", "path": ".env.local", "language": "dotenv", "content": "DIMMI_APP_ID=wgt_...\nDIMMI_IDENTITY_SECRET=" },
      { "kind": "file", "path": "app/layout.tsx", "language": "tsx", "content": "..." }
    ],
    "docs_url": "https://docs.trydimmi.com/messenger/install/nextjs-app"
  }
}
```
`installed`, `last_boot_*`, `origins_*` come from install tracker (README "Install tracking"; any widget request incl. config GET). `framework` required, enum = 09 `dimmi init` ids: `nextjs-app`, `nextjs-pages`, `react`, `react-router`, `sveltekit`, `nuxt`, `vue`, `astro`, `rails`, `laravel`, `django`, `fastapi`, `go`, `express`, `hono`, `html`, `proxy` (unknown: 422 with allowed list). `blocks` rendered from `resources/widget/install/<id>.tmpl`; text exact per [sdk-contract.md](sdk-contract.md) "Install by stack"; `{{.AppID}}` filled in env lines only (code never contains `wgt_`). Secret never included (created via 02 API by `dimmi init`).

## Dashboard: test sessions

`POST /api/v1/widget-settings/{widget}/test-sessions` -> 201 `{ "data": { "object": "widget_test_token", "token": "dmi_wtt_...", "expires_at": "+10 min", "preview_url": "https://app.trydimmi.com/widget-preview/wgt_...#dmi_wtt_..." } }`. One-time, bound to minting member. `POST /widget/v1/sessions {app_id, test_token}` creates session `test = true` (session field; conversations it starts get 03 `conversations.is_test = true` via `InboundMessage.IsTest`), contact = per-member test lead (`custom_attributes.test_member_id`, name "Test: Kim"). Over monthly cap: 429 `test_quota_exceeded`.

`/widget-preview/{wgt}` is SSR page in web routes (dashboard origin) that loads widget with test token from fragment; not a UI design, just host page. 15 preview: fragment `#preview=ann_...` or `#preview=svy_...` forwarded to SDK, which shows that item; test sessions may pass `preview` param on 15 engage routes (draft items allowed, receipts `is_test`).
