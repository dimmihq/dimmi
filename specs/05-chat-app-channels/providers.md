# Providers: verified facts and decided behavior

Style: caveman. Facts checked 2026-11-01 against provider docs (sources at end of each section). Where docs silent, behavior marked **decided**.

Libraries (research/build-vs-reuse.md 3.6; adapter zone `app/drivers/channel/*`, framework.md 7.3). Every SDK gets `a.HTTP.StdClient()` (SSRF guard) as its HTTP client; websockets dial through `framework/websocket` / `httpclient/safedial` where the SDK allows a custom dialer:

| Provider | Library | We keep |
|---|---|---|
| Slack | `github.com/slack-go/slack` v0.33.x (BSD-2): Web API client, `socketmode`, `slackevents`, `slack.NewSecretsVerifier` | `format.go` (Markdown to mrkdwn/blocks), `manifest.go`, error mapping, room/identity rules |
| Discord | `github.com/disgoorg/disgo` v0.19.x (Apache-2.0): REST + rate limiter, `gateway` (zstd-stream), `sharding` ShardManager configured with lease-owned shard ids only, `httpserver` Ed25519 verify | lease ownership of shard ids per replica, close-code policy, format, invalid-request breaker. Never import `disgo/voice` (cgo `godave`); CI builds `CGO_ENABLED=0`, depguard denies `github.com/disgoorg/disgo/voice` |
| Telegram | `github.com/go-telegram/bot` v1.27.x (MIT, zero deps, Bot API 10.3; `WithHTTPClient`, webhook handler, `getUpdates`) | `entities.go` (UTF-16 offsets for Markdown conversion), `format.go`, poller lease + offset persistence |
| WhatsApp | none: thin own Graph client on `a.HTTP` (only Go lib is alpha v0.1.x) | whole `whatsapp/client.go` (messages, media, templates, phone registration, subscribed_apps) |

Shared pipeline (all four):
```
provider -> /webhooks/... | socket/gateway/poller
  -> verify signature (middleware, raw body)
  -> dedupe fast path: Valkey `SET NX EX` key `ch:dedupe:{provider}:{dedupe_key}` (window: Slack 1 h, Discord 1 h, Telegram 24 h, WhatsApp 7 d = provider retry horizon); hit -> 200, no DB write
  -> StoreInboundEvent (tx: insert channel_inbound_events ON CONFLICT DO NOTHING (durable backstop + audit) + InsertTx process job) -> 200
process_channel_inbound_event (per-thread advisory lock)
  -> driver.Normalize(event) -> []Op (message | status | lifecycle | room | ignore); provider edit/delete events -> ignore except Slack (edit -> 03 EditInboundMessage, delete -> 03 MarkInboundMessageDeleted; later: message-edit for others)
  -> classify author: bot/self -> ignore; teammate or helper in room thread -> InboundMessage with Echo (03 X2); teammate elsewhere -> ignore (DM: IsTest rules); contact -> trigger check (room `trigger`) -> pleasantry filter -> InboundMessage
     flags: `public_room`, `addressed_to_other` (reply-to / mention of non-bot person), `audience_company_id` (Slack Connect room with company link)
  -> interactions (buttons, shortcuts, slash commands, context menus) -> apply_channel_interaction (solved, handoff, create conversation, /ask)
  -> attachments: stream from provider (allowlisted host, size cap) to Filesystem
  -> 03 conversations.ReceiveInboundMessage / UpdateMessageDelivery
     InboundMessage.AfterPersist hook (same 03 tx): RecordContactState (last_inbound_at, window, profile) + channel_message_refs (media groups, extra ids)
outbound: 03 send_channel_message -> driver.Send -> provider -> SendResult (+ channel_message_refs parts)
```

Common markdown pipeline: `markdown.Parse` (framework, goldmark underneath) of canonical Markdown into neutral AST; per-provider renderer walks AST. Tables rendered as monospace code block (all four lack tables). Images in Markdown (`![a](u)`) rendered as link (Dimmi attachments go as real files). Headings rendered as bold line. Nested lists flattened with 2-space indent + bullet `•` where provider lacks lists. Citations footer from `metadata.ai.citations` appended by driver: `Sources:` line then numbered links in provider syntax.

Splitter: if rendered length over provider per-message limit, split at (in order) paragraph boundary, line boundary, sentence end, word; never inside code fence (fence closed at end of part and reopened in next with same language); never inside link or entity. Parts sent sequentially; part 0 id is `messages.channel_message_id`; all ids in `channel_message_refs`.

---

## 1. Slack

### Connect
| Mode | Flow |
|---|---|
| shared (cloud, or self-host with `services.slack.*`) | Runs through `23-integrations` (`framework/oauthclient` `slack` driver); scopes = 23 registry union for 05, 06, 22; result is `integration_installations` row, connection holds `integration_installation_id`, never own token copy. `POST /channels/slack/installations` delegates to 23 and returns `https://slack.com/oauth/v2/authorize?client_id&scope&redirect_uri&state`. Callback: `oauth.v2.access` returns `access_token` (`xoxb-`), `team.id`, `team.name`, `bot_user_id`, `app_id`, `enterprise`, `is_enterprise_install`. `is_enterprise_install = true`: reject `enterprise_install_unsupported`. On `integration.connected` (feature `channels`): create connection `external_id = app_id:team_id`, status `active`, enqueue `sync_channel_rooms`, run auto-join scan. Token rotation off |
| custom (self-host default) | `POST /channels/slack` creates `pending` connection, returns manifest + `create_app_url = https://api.slack.com/apps?new_app=1&manifest_json=<urlencoded JSON>`. User creates app, installs to workspace, copies Bot User OAuth Token + Signing Secret (+ App-Level Token with `connections:write` for Socket Mode). `POST /channels/{chn}/credentials`: `auth.test` (gets `team_id`, `user_id`, `bot_id`), `apps.connections.open` when socket, scope check via `x-oauth-scopes` response header vs required list; stores tokens as 23 manual installation (`source = manual`) so 06 and 22 use same app. Reply-as-teammate on custom app: manifest also declares user scopes `chat:write`, `reactions:write` and redirect URL `{{ .AppURL }}/integrations/slack/callback`; member grant flow uses custom app's client id/secret (pasted, optional) |

Slack Marketplace: shared cloud app submitted at first staging deploy (`23-integrations` "Slack Marketplace"). Not required for self-host or custom apps (single-workspace, not commercially distributed; rate-limit tiers unchanged). Shared cloud app is commercially distributed: Marketplace approval removes `conversations.history`/`conversations.replies` throttle (non-Marketplace distributed apps limited to 1 request/min, max 15 objects since 2025-05-29; Tier 3 for Marketplace and internal apps). 05 uses those methods only for gap backfill.

Generated manifest (`resources/stubs/slack_manifest.yaml.tmpl`, URL values from `app.url` and connection id):
```yaml
_metadata: { major_version: 2, minor_version: 1 }
display_information:
  name: "{{ .Name }}"            # max 35 chars, default "Dimmi Support"
  description: "Customer support by Dimmi"
features:
  app_home: { home_tab_enabled: false, messages_tab_enabled: true, messages_tab_read_only_enabled: false }
  bot_user: { display_name: "{{ .BotName }}", always_online: true }
oauth_config:
  scopes:
    bot: [channels:history, groups:history, im:history, channels:read, groups:read, im:read, im:write,
          chat:write, chat:write.customize, files:read, files:write, users:read, users:read.email,
          reactions:read, reactions:write, team:read, channels:join, app_mentions:read, commands,
          chat:write.public, assistant:write]
    user: [chat:write, reactions:write]   # asked only in member grant flow (23)
  redirect_urls: ["{{ .AppURL }}/integrations/slack/callback"]
settings:
  event_subscriptions:
    request_url: "{{ .AppURL }}/webhooks/slack/events/{{ .ChannelID }}"   # omitted when socket mode
    bot_events: [message.channels, message.groups, message.im, app_uninstalled, tokens_revoked,
                 member_joined_channel, channel_left, group_left, channel_archive, group_archive,
                 channel_deleted, group_deleted, channel_rename, group_rename, channel_id_changed,
                 channel_shared, channel_unshared, app_mention, reaction_added, reaction_removed,
                 channel_created, assistant_thread_started]
  interactivity:
    is_enabled: true
    request_url: "{{ .AppURL }}/webhooks/slack/interactions/{{ .ChannelID }}"   # omitted when socket mode
  shortcuts:
    - { name: "Track in dimmi", type: message, callback_id: dimmi_create_conversation, description: "Track this message as a support conversation" }
  socket_mode_enabled: {{ .Socket }}
  token_rotation_enabled: false
  org_deploy_enabled: false
```
`app_uninstalled` and `tokens_revoked` need no scope. `member_joined_channel` with `user == bot_user_id` detects bot invited. Interactivity, `reactions:read`, `app_mentions:read`, `commands` ship from day one even where MVP uses little of them: adding scopes later forces every custom app to be edited and reinstalled.

Url verification: custom `pending` connection without signing secret answers `url_verification` challenge only (Slack verifies request URL during manifest creation, before user can copy secret). Any other event before secret stored: 401. Shared endpoint always verifies.

### Inbound
- Events API HTTP: verify `X-Slack-Signature` = `v0=` + hex HMAC-SHA256(signing secret, `v0:{X-Slack-Request-Timestamp}:{raw body}`), timestamp within 300 s. Respond 2xx within 3 s (we answer in under 150 ms). Slack retries failed deliveries up to 3 times (about immediately, 1 min, 5 min) with `x-slack-retry-num`, `x-slack-retry-reason`; dedupe on `event_id` then message key. Subscriptions disabled automatically if over 95% failures in 60 min (apps with 1,000+ events/hour): health job alerts at 20% failure rate. Delivery cap 30,000 events per workspace per app per 60 min; `app_rate_limited` event logged + `channel.updated` warning.
- Socket Mode (custom, self-host without public URL): `apps.connections.open` with `xapp-` token returns WSS URL; ack each envelope with `{"envelope_id": ...}` within 3 s after storing raw event; reconnect on `disconnect` message (`refresh_requested`, `link_disabled`). Lease kind `slack_socket`, key connection id. Same `StoreInboundEvent`.
- Routing: shared: `api_app_id` + envelope `team_id` (installed workspace) to `external_id`. Event visible to several installations of shared app (both sides of Slack Connect channel are Dimmi customers): `authorizations` lists one; call `apps.event.authorizations.list` (cached 10 min per event context) and fan out to every connection whose room includes channel. Custom: path connection id.

Message events handled (`event.type = message`):
| Subtype / shape | Behavior |
|---|---|
| none, `file_share`, `thread_broadcast`, `me_message` | message (thread_broadcast treated as thread reply) |
| `message_changed` | contact or echo message already stored (lookup `channel_message_refs` / `channel_message_id`): 03 `EditInboundMessage` (body replaced, previous in `metadata.edits[]`, `message.updated`); edit of message not stored: ignore. Edit never triggers agent, never reopens. Own bot edits: ignore |
| `message_deleted` | stored message: 03 mark deleted (`metadata.source_deleted = {by, reason, at}`); words, files, edit history kept; timeline shows message with "Deleted by the customer" label; unanswered conversation whose customer messages are all deleted: closed reason `source_deleted`. Not stored: ignore |
| `bot_message`, any event with `bot_id` | ignore (own bot: confirms send; other bots: ignored unless room setting `allow_bot_authors`) |
| `channel_join`, `channel_leave`, `channel_topic`, `channel_purpose`, `pinned_item`, others | ignore |

Room rules:
- Message in DM (`channel_type = im`): thread key `slack:d:{channel}`; always active (App Home messages tab).
- Message in channel without room row or room `paused`/`removed`: ignore. Room auto-created `active` on `member_joined_channel` (bot) with mode `shared` when `is_ext_shared` else `community`.
- Top-level contact message (no `thread_ts` or `thread_ts == ts`), by room `trigger`:
  - `author_window` (default): new conversation `slack:r:{room}:{ts}` unless same author posted top-level message in same room within `group_window_seconds` (default 120) and that conversation still has no workspace reply: append to it (Dimmi replies stay in first thread).
  - `every_message`: each top-level message own conversation.
  - `mention`: only when message mentions bot (`app_mention`); others ignored (`not_triggered`).
  - `emoji`: nothing at post time; `reaction_added` of `trigger_emoji` (default `ticket`) on a top-level message by anyone in room creates conversation from that message (fetch via `conversations.history` `latest=ts inclusive limit=1`; Marketplace throttle: 1 call per trigger is within limit). Later thread replies follow normal rules.
  - `off`: no new conversations; echo and state still tracked for existing ones.
  - Pleasantry filter runs before all modes: pleasantry top-level message never creates conversation.
- Message shortcut "Track in dimmi" (any member of Slack workspace with app): creates conversation from that message regardless of trigger; ephemeral confirmation with link for linked members.
- Thread reply (`thread_ts != ts`): key `slack:r:{room}:{thread_ts}`. No conversation for that key (thread started by teammate or before connect): contact message in that thread creates conversation keyed on thread root; teammate message ignored.
- Mode `shared`: author is teammate when `user_team`/`user_profile.team` equals installing team and user not `is_restricted`/`is_ultra_restricted` (guests are contacts). Mode `community`: teammate only when `channel_member_links` has user; everyone else contact.
- Room mode `internal`: no conversations. Messages handled only as: triage thread replies (note, or customer reply with `>>` prefix), discussion thread messages (notes), `app_mention` (22 Operator), dimmi conversation link pasted (ephemeral "Start discussion"). Everything else ignored, never stored.
- Room mode `helpdesk`: own-workspace channel. Author is teammate only when linked member of workspace; every other own-workspace user is contact (identity `slack_user`, profile email `verified = true` because same Slack org as installer). Triggers as other rooms. App Home tab (`app_home_opened`, `views.publish`) lists requester's open and recent conversations with links to threads.
- Trigger `ai`: top-level contact message (after pleasantry filter) sent to 07 `ClassifyRequest` (cheap model, max 300 input tokens: message + last 3 room messages for context, untrusted-wrapped) returning `request|chatter|followup` + confidence. `request` above threshold (conservative 0.75, aggressive 0.45): new conversation. `followup` within 24 h of author's last conversation in room: append to it (reopens if closed and not sign-off). `chatter`: ignored, metric `kind=chatter`; teammate can still use shortcut or trigger emoji. Classifier error or timeout (3 s): fall back to `author_window`.
- Teammate messages: top level ignored (metric `kind=teammate`). Inside thread of existing conversation: echo (03 X2) with `MemberID` when linked member, else ignored with room warning "Link your Slack account to record replies" (once per user per 7 days, ephemeral). Teammate @mentioned by customer in thread: agent stand-down (`human_replied`). Auto-link by email (Slack profile email equals active member's verified user email, `source=email_match`) feeds `community` classification.

### Outbound
- Interactions endpoint: `application/x-www-form-urlencoded` `payload` JSON; same signature scheme as events; ack 200 within 3 s after `StoreInboundEvent` (transport `interaction`); Socket Mode delivers same payload as envelope type `interactive`.
- Text: `chat.postMessage` with `channel`, `thread_ts` (root ts; DMs: none), `markdown_text` (standard Markdown accepted, max 12,000 chars, cannot combine with `text`/`blocks`), `username` + `icon_url` = teammate name/avatar (or agent name) via `chat:write.customize`, `unfurl_links: false`, `unfurl_media: true`. Escaping: `<`, `>`, `&` escaped to `&lt;` `&gt;` `&amp;` (blocks `<!here>` injection).
- First reply to top-level message: `thread_ts = root ts`, `reply_broadcast = false`.
- Attachments: per file `files.getUploadURLExternal` (`filename`, `length`) then POST bytes to `upload_url`, then one `files.completeUploadExternal` with `files: [{id, title}]`, `channel_id`, `thread_ts` (files share into thread). Text message posted first, files after. `files.upload` sunset 2025-11-12: never used. Refs kind `file`, external id `file:{F...}`.
- Typing: no typing event for apps. Working state: `agents.sessions.setStatus` in any thread (README "Slack AI controls"); capability `typing` stays false.
- Receipts: none; `sent` on `ok: true`.
- State marker (`ChannelStateMarker`): closed `reactions.add` `white_check_mark` on thread root; reopen `reactions.remove`. Errors `already_reacted` / `no_reaction` ignored.
- Working indicator: `reactions.add` `eyes` on triggering message at `ai.reply_started`, removed at commit/discard.
- Reply as teammate: sender has active 23 member grant and that Slack user is member of channel (`conversations.members` cached 10 min): `chat.postMessage` with user token, no `username`/`icon_url`. `not_in_channel` or user token error: retry once with bot + name/avatar, mark grant `error` on auth errors. DMs to bot always bot token.
- Emoji actions (inbound): `reaction_added` on conversation root by linked member: mapping (`white_check_mark` close, `eyes` assign reactor + open, `rotating_light` urgent); `reaction_removed` of `white_check_mark` reopens. Our own state-marker reactions (bot user) ignored. Contact reactions: no action.
- Triage post: `chat.postMessage` to triage room (`chat:write.public` for public rooms without bot; private needs invite), blocks: header section, context (company, plan, SLA due), actions (Take it, status `static_select`, Snooze, Urgent, Open). Live update `chat.update` debounced 2 s per conversation; ts stored in `channel_message_refs` kind `triage`. Thread replies under triage post: notes; `>>` prefix or "Reply to customer" shortcut: customer reply via normal outbound as that member.
- Auto-reply: `chat.postMessage` as bot in conversation thread after `delay_seconds` if no reply exists; `AutoGenerated`, no agent trigger.
- Answer controls (X6): message sent with `blocks` instead of `markdown_text` when controls present: `markdown` block (same Markdown, 12,000 chars) + `actions` block with buttons `Solved` (`primary`) and `Talk to a human`; `action_id` = control id (`controls.go`). Click: interaction payload `block_actions`, verify clicker is thread participant contact (else ephemeral "Only the person who asked can mark this solved"); then `chat.update` removes actions block. `ClearControls` also on human reply.

### Formatting
- Out: Markdown passed as `markdown_text` after: escaping above, tables to code block, images to links, citation footer. Mention of linked teammate never generated (plain names).
- In: prefer `blocks` `rich_text` (structured: sections, lists, quotes, preformatted, styled text, links, user/channel/emoji elements) to Markdown; fallback mrkdwn `text` parser: `*b*` to `**b**`, `_i_` to `*i*`, `~s~` to `~~s~~`, `<url|label>` to `[label](url)`, `<@U123>` to `@Display Name` (users.info cache), `<#C1|name>` to `#name`, `<!here>` to `@here` text, entity decode `&amp;` `&lt;` `&gt;`. Emoji `:smile:` kept as shortcode text.

### Identity
- `slack_user` value `{team_id}:{user_id}` (user's home team, from `user_team` or `users.info` `team_id`), `verified = true` (provider id).
- `users.info` (with `users:read.email`): name (`profile.display_name` or `real_name`), avatar `image_192`, `tz` to contact timezone if empty, email to `email` identity **verified = false** (open question 1). External Slack Connect users: `users.info` works for cross-team ids; email may be hidden by their org.

### Rate limits
- `chat.postMessage`: about 1 message/s per channel, workspace limit several hundred/min; burst tolerated. Bucket per (team, channel) 1/s burst 3 (`a.Limiter`, Valkey; shared across replicas).
- Web API tiers per method per workspace per app: Tier 1 1+/min, Tier 2 20+/min, Tier 3 50+/min, Tier 4 100+/min. `users.info` Tier 4, `conversations.list` Tier 2, `files.*External` Tier 4, `reactions.add` Tier 3.
- 429: `Retry-After` seconds; driver returns `RateLimitedError{RetryAfter}`. Other methods/workspaces unaffected.

### Lifecycle
| Signal | Effect |
|---|---|
| `app_uninstalled` | 23 `ApplyProviderRevocation`: installation `disconnected`; connection `disconnected` reason `uninstalled`; rooms `removed`; 06 Slack destinations `error` |
| `tokens_revoked` (bot) | status `disconnected` reason `token_revoked` (via 23) |
| `tokens_revoked` (user ids listed) | 23 member grants for those users `revoked`; replies fall back to bot with name + avatar |
| API `invalid_auth`, `account_inactive`, `token_revoked`, `token_expired`, `not_authed` | `ErrConnectionBroken`: status `error` code `slack_auth_failed` |
| `not_in_channel` on send | room `error` "Invite @bot to #channel"; message failed `slack_not_in_channel` |
| `channel_not_found`, `is_archived`, `channel_archive`, `channel_deleted`, `channel_left`/`group_left` | room `removed`; sends fail `slack_channel_gone` |
| `channel_id_changed` | update room `external_id` (thread keys use room id, unaffected) |
| `channel_shared` / `channel_unshared` | update `is_shared_external`; suggest mode change via `channel.updated` (never auto-switch mode) |
| `ekm_access_denied`, `team_access_not_granted`, `restricted_action` | permanent failure, message error text explains admin policy |

Sources: docs.slack.dev events-api, verifying-requests-from-slack, rate-limits, chat.postMessage reference, app-manifest reference, configuring-apps-with-app-manifests, changelog 2024-04 files.upload retirement (sunset 2025-11-12), 2025-05 rate limit FAQ, slack-connect docs.

---

## 2. Discord

### Connect
| Mode | Flow |
|---|---|
| shared | `POST /channels/discord/installations` returns `https://discord.com/oauth2/authorize?client_id={app}&scope=bot+applications.commands&permissions={channels.discord.permissions bitset}&response_type=code&redirect_uri=...&state=...` (optional `guild_id` + `disable_guild_select=true` on reconnect). Callback gets `code`, `guild_id`; exchange code at `/oauth2/token` (includes `guild` object). Create connection `external_id = app_id:guild_id`. Bot receives `GUILD_CREATE` over gateway. Setup step: pick rooms (forum channels default `auto`, text channels default `mention`), map roles. Global commands `/ask`, message context menu `Ask dimmi` registered once per application |
| custom | `POST /channels/discord {bot_token, guild_id?}`: `GET /users/@me` + `GET /applications/@me` (application id, `verify_key` public key). `GET /users/@me/guilds`: 0 guilds: `pending`, return invite URL; 1: bind; more: 422 `discord_guild_required` unless `guild_id` given. Optional Interactions Endpoint URL `/webhooks/discord/interactions/{chn}` (setup step; without it interactions arrive over gateway) |

Intents (2026): GUILDS, GUILD_MESSAGES, DIRECT_MESSAGES, MESSAGE_CONTENT (privileged). Since 2026-06-10 review is by user count: free toggle below 10,000 users across all servers, review above it, renewed yearly. Custom (self-host) bots: toggle in developer portal, setup checks it. Cloud shared bot: App Verification before 100 servers (apply at 75), intent review before 10,000 users; until granted above threshold, limited mode (config `channels.discord.intents`). DM content, bot mentions and interactions carry content without the intent.

### Inbound
- Gateway: `GET /gateway/bot` (url, recommended `shards`, `session_start_limit` with `max_concurrency`). Identify with intents 4097 (GUILDS 1, DIRECT_MESSAGES 4096), `compress` zstd-stream transport. Sharding required at 2,500 guilds per shard; shards distributed by lease (`discord_gateway` key `{app}:{shard}/{count}`). Identify budget 1,000 per 24 h across shards (exceed: all sessions terminated + token reset): supervisor checks `session_start_limit.remaining` before identify, prefers RESUME (does not count). Send limit 120 gateway events per 60 s per connection. Heartbeat at `heartbeat_interval` (first with jitter), missing ACK: close non-1000 code, resume. Resume with `resume_gateway_url`, `session_id`, last `seq` (persisted in `channel_socket_sessions.state` every 2 s and on close) so new lease holder can resume after failover. Op 7 reconnect: resume. Op 9 `d=false`: re-identify then backfill.
- Close codes: 4004 auth failed (token reset): connection `error` `discord_token_invalid`, no reconnect until credentials rotated. 4013/4014 (intents): connection `error` `discord_intents_invalid` (config bug; no privileged intent requested). 4010/4011 sharding: refetch `/gateway/bot`, reshard (all leases released, recomputed). Others: reconnect with backoff 1 s to 60 s.
- Events used: `GUILD_CREATE` (bind custom pending, mark `available`), `GUILD_DELETE` (`unavailable = true`: outage, keep; absent: bot removed, connection `disconnected` reason `guild_removed`), `MESSAGE_CREATE` (DMs; guild messages only in watched rooms and their threads, rest dropped before store), `THREAD_CREATE` (forum post created; `newly_created`), `THREAD_UPDATE` (tags, archived, locked changed in Discord: synced to conversation state), `THREAD_DELETE` (conversation stays, room thread gone note), `CHANNEL_UPDATE` (forum available tags refresh `tag_map`), `GUILD_ROLE_UPDATE`/`DELETE` (role link refresh/remove), `INTERACTION_CREATE` (commands, components; only when no HTTP endpoint). `MESSAGE_UPDATE`/`MESSAGE_DELETE`: ignored (later: message-edit).
- Interactions HTTP: verify Ed25519 signature `X-Signature-Ed25519` over `X-Signature-Timestamp + raw body` with application public key (disgo `httpserver` verifier); PING (type 1) answered `{"type":1}`; respond within 3 s (component select answered with update message).
- Gap recovery: after re-identify or lease takeover with failed resume, `backfill_channel_room` for DMs active in last 24 h: `GET /channels/{dm}/messages?after={last_seen_message_id}&limit=100`, feed through same dedupe path.

Threading (private `/support` tickets: later: discord-private-tickets):
| Place | Conversation |
|---|---|
| Forum post (room `discord_forum`, trigger `auto`) | key `discord:t:{thread_id}`; starter message = first inbound; title = subject. Trigger `mention`: only when post or reply mentions bot. Bot applies tag Open on create (missing tags: setup step "Create tags Open, Answered, Solved, Needs team" or one-click `PATCH /channels/{forum}` `available_tags` when bot has Manage Channels; else tag sync disabled with health warning). Max 5 applied tags per post: dimmi only toggles its own state tags, keeps others |
| Text channel (room `discord_text`, trigger `mention`) | bot mention, `/ask question:`, or context menu `Ask dimmi` on any message: driver starts thread off message (`POST /channels/{c}/messages/{m}/threads`, name = first 80 chars) then ingests with key `discord:t:{thread}` (X5). `/ask` answers ephemeral ack "Answering in thread" |
| Thread of watched room, later messages | contact: normal inbound; linked member or `teammate` role with link: echo member; `helper` role: echo helper; reply-to or mention of other non-bot user: `addressed_to_other` |
| DM | key `discord:d:{dm_channel}`. Shared bot DM routing: user must share a guild with bot; candidates = connections whose guild has this user (`GET /guilds/{g}/members/{u}` probe, cached 24 h). 1 candidate: route. Several: bot replies with select menu (component) "Which community do you need help with?"; choice stored in `channel_contact_states.profile.dm_route` (sticky 30 days, asked again after). 0: bot replies once per 24 h "Please ask in your community's support channel" (not stored as conversation). Custom bot: its connection(s) only |

Teammates: DM from user linked in `channel_member_links`: conversation flagged `IsTest` (D10). Webhook messages (`webhook_id`) and `author.bot` ignored (own bot: confirm send).

### Outbound
- Rooms: same call on thread id (`/channels/{thread}/messages`); archived thread: unarchive first (`PATCH archived=false`) when conversation reopened by reply. Locked thread: send fails `discord_thread_locked`, message failed with explanation.
- Answer controls (X6): `components` action row with buttons `Solved` (style success) and `Talk to a human` (secondary), `custom_id` = control id. Click: interaction type 3; only original poster (thread owner or conversation contact) may press Solved, others get ephemeral notice; response type 7 (update message) removes row. Thumbs feedback: later.
- Handoff in room: "A teammate will follow up here." plus one allowed role mention (`allowed_mentions.roles = [escalation role]`), tag Needs team.
- State marker: closed: tag Solved (remove Open/Answered/Needs team) then `archived=true` when `archive_on_close`; reopened: unarchive, tag Open.
- `POST /channels/{dm}/messages` with `content` (max 2,000 chars per message, split max 4 parts), `allowed_mentions: {"parse": []}`; files as multipart `files[n]` (max 10 per message, each within upload limit; default 20 MiB for all users since 2026-09-03, higher with server boost; `attachment_size_limit` unknown for non-interaction sends, so driver uses 20 MiB): bigger file: signed link line appended. Request total kept under 25 MiB by batching files into extra messages.
- Bot posts as itself (no per-message username for bot messages); teammate attribution: first line `**{Teammate name}**` prefix setting `show_author_name` (default true) for human replies; agent replies prefixed with agent name only when `show_author_name`.
- Typing: `POST /channels/{c}/typing` (expires 10 s). 03 job every 4 s while typing: driver throttles to 1 per 8 s per channel.
- Receipts: none.

### Formatting
- Out: Discord markdown close to CommonMark: `**b**`, `*i*`, `__u__`, `~~s~~`, `` `c` ``, fenced blocks with language, `>` quotes, `#`/`##`/`###` headings, `-` lists, masked links `[t](url)` allowed. Escape `@everyone`/`@here` text and `<@`, `<#`, `<@&` sequences (zero-width joiner after `@`) as defense in depth beside `allowed_mentions`. Tables to code block. Link previews: wrap bare URLs in `<url>` when more than 2 links (avoid embed spam).
- In: `<@id>`/`<@!id>` to `@display_name` (resolved from message `mentions` array), `<#id>` to `#channel-name`, `<@&id>` to `@role`, custom emoji `<:name:id>` to `:name:`, `||spoiler||` kept as text, timestamps `<t:unix:f>` to ISO date text, sticker items to `[sticker: name]`.

### Identity
- `discord_user` value user snowflake, verified true. Name from `global_name` or `username`; avatar CDN URL; no email (bots never get email without OAuth `email` scope; not used).

### Rate limits
- Per-route buckets: read `X-RateLimit-Bucket`, `-Remaining`, `-Reset-After`; buckets keyed per top-level resource (channel, guild, webhook). Per-route buckets handled by disgo REST rate limiter (in-memory per bot per replica), plus shared global limiter 50 requests/s per bot across replicas (`a.Limiter` on Valkey, key `discord:global:{app}`). Pre-emptive wait when `Remaining = 0`.
- 429 body `retry_after` (seconds, float), `global`, header `X-RateLimit-Scope` (`user`, `global`, `shared`). `RateLimitedError{RetryAfter}`.
- Invalid requests (401, 403, 429 except `shared` scope) limit 10,000 per 10 min per IP else Cloudflare ban: circuit breaker counts per bot; over `invalid_request_budget` (5,000): pause all non-gateway REST for that bot 10 min, connection `error` `discord_invalid_request_storm`, alert.

### Lifecycle
| Signal | Effect |
|---|---|
| 401 on REST | `ErrConnectionBroken`, `discord_token_invalid` |
| 403 `50007` Cannot send messages to this user | `ErrRecipientUnreachable`, state `dm_closed` |
| 404 `10003` Unknown Channel | DM channel gone: message failed `discord_channel_gone` |
| GUILD_DELETE not unavailable | `disconnected` reason `guild_removed` |

Sources: docs.discord.com developers/events/gateway, topics/rate-limits, resources/channel, reference (uploading files: default 20 MiB), change-log (2026-09-03 upload limit 20 MiB; 2026-06-10 privileged intent review by user count; 2026-08-12 channel obfuscation from 2026-11-16).

---

## 3. Telegram

### Connect
- Only custom mode (each business has own bot from @BotFather). Cloud and self-host identical.
- `POST /channels/telegram {bot_token}`: `getMe` (bot id, username, `can_join_groups`, `can_read_all_group_messages` = privacy off). Then transport:
  - webhook: `setWebhook` url `{app.url}/webhooks/telegram/updates/{chn}`, `secret_token` (64 random chars from `[A-Za-z0-9_-]`, allowed 1 to 256), `allowed_updates: ["message","my_chat_member","callback_query"]` (callback_query = answer buttons), `max_connections` 40 (allowed 1 to 100), `drop_pending_updates: false`. Ports allowed by Telegram: 443, 80, 88, 8443 (else polling auto).
  - polling (self-host without public URL): `deleteWebhook`, lease `telegram_polling` key bot id, `getUpdates` long poll timeout 50 s, `offset` persisted in `state` after each batch commit. Webhook and polling exclusive (getUpdates fails while webhook set).
- `setMyCommands` registers `/start`, `/help`, `/human` in private chats. Bot description not touched.
- Setup shows `t.me/{username}` link + QR code (rendered client side) for sharing.
- Token valid format checked before network. Same bot connected to another workspace: 409 `channel_connected_elsewhere` (never steal webhook).

### Inbound
- Header `X-Telegram-Bot-Api-Secret-Token` must equal connection secret (constant time). Non-2xx makes Telegram retry, then give up; updates kept by Telegram at most 24 h. Dedupe `update_id` per bot.
- `message` in private chat: key `telegram:p:{chat_id}`. `/start` alone: stored `AutoGenerated` (no AI trigger, no billing) only when no open conversation. `/start {nonce}`: nonce looked up in Valkey `ch:link:{nonce}` (05 `deep_links.go`, single use, TTL 15 min): hit binds `telegram_user` identity to widget contact (`InboundSender.ContactID`, verified pair), greeting uses contact name; miss or expired: treated as plain `/start`, payload never shown to agent. `/human`: 07 `RequestHumanHandoff`.
- `callback_query`: buttons (X6 controls). `answerCallbackQuery` within 10 s always (spinner stops); `data` = control id (under 64 bytes). Then `editMessageReplyMarkup` to remove keyboard.
- Groups/supergroups/forum topics: messages ignored, metric `kind=group`; bot added to group: no room, nothing stored (later: telegram-groups).
- Private chats with topics (Bot API 9.3+): `message_thread_id` echoed back on replies.
- Edits: `edited_message` not subscribed (later: message-edit). Deletes: Bot API has no update for user deletions in normal chats: never synced.
- `my_chat_member` in private chat with `kicked`: user blocked bot: state `reachable=false` `blocked_bot`; `member` again: reachable.
- Media: photo (largest size), document, video, audio, voice, video_note, sticker (static only as image; animated/video sticker as `[sticker]` text), location/venue (Markdown map link), contact (vCard summary text). `getFile` download limit 20 MB: larger: placeholder `[file.pdf, 40 MB, too large to import from Telegram]`.
- `from.is_bot` true: ignored. Channel posts: ignored (not in allowed_updates).

### Outbound
- `sendMessage` `parse_mode: HTML`, text max 4,096 chars after entity parsing, split max 4; `link_preview_options: {is_disabled: true}` when more than 1 link; private-chat topics: `message_thread_id`.
- Attachments: images up to 10 MB `sendPhoto`, else `sendDocument` (up to 50 MB upload; our cap 25 MB); 2 to 10 attachments of photo/video: `sendMediaGroup` (caption on first, max 1,024 chars; longer text sent as separate message first). Refs `media_group` per item.
- Typing: `sendChatAction` `typing` (shows 5 s or until message); 03 4 s cadence fits.
- Draft streaming (private chats, `channels.telegram.stream_drafts`): on `ai.reply_started` `sendMessageDraft` with `draft_id` = run sequence (non-zero int), empty text = "thinking" state; on `ai.reply_delta` resend accumulated text rendered to HTML (throttle 1 per 500 ms per chat; rendered text over 4,096: stop streaming, final split send). On commit normal `sendMessage` with controls (draft replaced by message). On `ai.reply_discarded` draft cleared by empty update. Errors on draft calls never fail the answer: fall back to typing.
- Reaction: `setMessageReaction` 👀 on inbound message when agent will not answer now (agent off for channel, handed off, outside schedule) as honest "seen"; never when agent answers (draft/typing shows instead).
- Answer controls (X6): `reply_markup` `inline_keyboard` one row: `✅ Solved`, `🙋 Talk to a human`.
- Receipts: none for bots (read only for business connections). `sent` on `ok: true`.
- Cannot message first: no prior private chat (`403 Forbidden: bot can't initiate conversation with a user`): `ErrRecipientUnreachable` code `telegram_cannot_initiate`. `403 Forbidden: bot was blocked by the user`: `telegram_403_blocked`, state unreachable.

### Formatting
- Out (Telegram HTML subset): `<b>`, `<i>`, `<u>`, `<s>`, `<code>`, `<pre><code class="language-x">`, `<a href>`, `<blockquote>`, `<tg-spoiler>`. Escape `&`, `<`, `>` in text. Headings to `<b>`; lists to `• ` / `1. ` lines; tables to `<pre>`. HTML chosen over MarkdownV2 (MarkdownV2 requires escaping 18 characters `_*[]()~`>#+-=|{}.!` everywhere; error-prone).
- In: text + `entities` (offsets in UTF-16 code units) to Markdown: bold, italic, underline (to `<u>` dropped, plain), strikethrough, code, pre (with language), text_link, url, mention, text_mention (to `@Name`), spoiler (plain), blockquote, custom_emoji (fallback char), date_time (to text). Offsets converted carefully (emoji surrogate pairs, combining marks).

### Identity
- `telegram_user` value user id (int64 as text), verified true. Name `first_name + last_name`, `username`, `language_code` to contact locale if empty. No phone/email (unless user shares contact card: `contact` message with `user_id == from.id` adds `phone` identity **verified=false**).

### Rate limits
- Per chat about 1 message/s (bursts tolerated), bulk about 30 messages/s per bot (paid broadcasts not used). Buckets: `tg:{bot}:chat:{chat}` 1/s burst 3, `tg:{bot}:global` 30/s.
- 429 `{"ok":false,"error_code":429,"parameters":{"retry_after":N}}`: `RateLimitedError{N s}`.

### Lifecycle
| Signal | Effect |
|---|---|
| 401 Unauthorized (token revoked in BotFather) | `ErrConnectionBroken`, `telegram_token_revoked` |
| health: `getWebhookInfo` url mismatch | re-register (`register_telegram_webhook`); `last_error_message` non-empty with `pending_update_count` growing: connection `error` with Telegram text |
| Bot username changed in BotFather | `getMe` on health refreshes `external_name`, test link |

Sources: core.telegram.org/bots/api (setWebhook, ReplyParameters, update types), bots/faq (limits: 1 msg/s per chat, 20/min groups, 30/s bulk; 20 MB download, 50 MB upload; privacy mode), bots/api-changelog (9.0 to 10.3, latest 10.3 on 2026-08-24).

---

## 4. WhatsApp (Meta Cloud API, direct, no BSP)

### Connect
| Mode | Flow |
|---|---|
| shared (cloud: Dimmi registered as Meta Tech Provider) | Dashboard runs Facebook JS SDK `FB.login` with `config_id` (Embedded Signup v4; v2/v3 deprecated 2026-10-15). SDK returns `code` + session info `waba_id`, `phone_number_id`. `POST /channels/whatsapp/embedded-signups`: exchange code for business token (customer-scoped business token), `POST /{waba_id}/subscribed_apps` (webhooks), `POST /{phone_number_id}/register` with `messaging_product=whatsapp` and 6-digit `pin` (two-step verification), fetch phone (`display_phone_number`, `verified_name`, `name_status`, `quality_rating`, `code_verification_status`, `whatsapp_business_manager_messaging_limit`), WABA owner business id. Customer adds payment method in WhatsApp Manager (Tech Provider customers pay Meta directly). Onboarding cap: 10 new customers per rolling 7 days until Dimmi completes Business Verification + App Review + Access Verification (then 200) |
| custom (self-host or cloud power users) | User creates Meta app (Business type) + WhatsApp product, system user with `whatsapp_business_messaging` + `whatsapp_business_management`, permanent system user token, notes WABA id + phone number id + app secret. `POST /channels/whatsapp {access_token, app_secret, waba_id, phone_number_id}` validates (`GET /{phone_number_id}`, `debug_token` scopes), creates connection `pending` with generated `verify_token`; setup returns callback URL `{app.url}/webhooks/whatsapp/events/{chn}` + verify token; user pastes into Meta app Webhooks config and subscribes field `messages` (plus optional template/account fields). Dimmi calls `POST /{waba_id}/subscribed_apps`. Status `active` after verification GET succeeded and subscription confirmed (or first webhook arrives) |

Requires public HTTPS reachable by Meta (no polling alternative). `channels:check` tests reachability with self-GET.

### Verification checklist (`refresh_whatsapp_checklist`)
| Step key | Source | Required |
|---|---|---|
| business_portfolio | WABA `owner_business_info` | yes |
| business_verification | business `verification_status` (`verified`, `pending`, `not_verified`, `rejected`) | no (raises messaging limit 250 to 2,000; unlocks display name with official checks) |
| display_name | phone `name_status` (`APPROVED`, `PENDING_REVIEW`, `DECLINED`, `NONE`) + `phone_number_name_update` webhook | declined blocks |
| phone_registered | register call result, `code_verification_status` | yes |
| two_step_pin | set during register | yes (stored encrypted) |
| webhooks_subscribed | `GET /{waba_id}/subscribed_apps` contains our app | yes |
| payment_method | not reliably exposed: `unknown` until error 131042 seen (`action_required`) or paid message delivered (`complete`) | no |
| messaging_limit | `whatsapp_business_manager_messaging_limit` (`TIER_250`, `TIER_2K`, `TIER_10K`, `TIER_100K`, `TIER_UNLIMITED`); portfolio-level, counts unique users messaged outside open windows per moving 24 h | info |
| quality_rating | `quality_rating` + `phone_number_quality_update` | RED = action_required |
| account_status | `account_update` events (`DISABLED_UPDATE`, `ACCOUNT_VIOLATION`, `ACCOUNT_RESTRICTION`) | blocking events set connection `error` |
| first_message_received | `channel_contact_states` count > 0 | info |

### Inbound
- Verify GET: `hub.mode=subscribe`, `hub.verify_token` equals expected, respond `hub.challenge` body text 200; mismatch 403.
- POST: `X-Hub-Signature-256: sha256=<hex HMAC-SHA256(app_secret, raw body)>`. Payloads up to 3 MB. Meta retries non-200 with decreasing frequency up to 7 days; duplicates possible (multiple apps, retries). Each `entry[].changes[]` stored as its own inbound event (dedupe sha256 of change JSON; message-level dedupe `whatsapp:{wamid}`). Meta expects median under 250 ms, under 1% over 1 s.
- Routing: `value.metadata.phone_number_id` to connection; `entry[].id` (WABA) cross-checked against connection config.
- `messages[]` (field `messages`):
  - Sender: `from` (phone, may be omitted), `from_user_id` (BSUID, format `CC.<up to 128 alnum>`), `contacts[].wa_id` (may be omitted), `contacts[].user_id`, `contacts[].profile.name`, `profile.username`. BSUIDs in webhooks since early April 2026; phone omitted when user enabled username unless within 30 days business messaged/called that number or user messaged from it, or user in business contact book.
  - Identity claims: `whatsapp_bsuid` (verified), `whatsapp_wa_id` (when present, verified), `phone` `+{wa_id}` E.164 (when present, verified: auto-merge per data model). Parent BSUID (`parent_user_id`) stored in profile only.
  - Thread key `whatsapp:u:{bsuid}` (fallback `whatsapp:p:{wa_id}`); one open conversation per user per number.
  - Types: `text`; `image`/`video`/`audio`/`document`/`sticker` (media id valid 7 days in webhooks; `GET /{media_id}` gives URL valid 5 min, download with Bearer token; incoming media up to 100 MB, ours capped 25 MB: placeholder above cap); `location` (Markdown with map link + name/address); `contacts` (vCard summary); `interactive` (`button_reply.title` / `list_reply.title` as text, `metadata.whatsapp.reply_id`); `button` (template quick-reply payload text); `reaction` (**ignored**, not window-extending, metric); `order` (summary text, post-MVP detail); `system` (`user_changed_number` / `user_changed_user_id`: rekey via `previous_user_id`, no message); `unsupported` / errors (stored as message `[Unsupported WhatsApp message]` so teammate sees something arrived, `AutoGenerated=false`); `edit` / `revoke` (Coexistence only): ignored (later: message-edit).
  - `context.id` (reply-to) passed as `ReplyToChannelMessageID`.
  - Every user message updates `channel_contact_states.last_inbound_at` and `window_expires_at = timestamp + 24 h` (customer service window).
- `statuses[]`: `sent`, `delivered`, `read` (and `played` treated as read), `failed` with `errors[].code`. Mapped via `UpdateMessageDelivery` by `(connection, id)` through `channel_message_refs`. `pricing` (`billable`, `pricing_model` "PMP", `category`, `type` `regular`/`free_customer_service`/`free_entry_point`) stored in message metadata.
- Other fields: `message_template_status_update` (template status), `message_template_quality_update`, `phone_number_name_update`, `phone_number_quality_update`, `account_update`, `account_review_update`, `business_capability_update`: refresh checklist / templates, `channel.updated`, `whatsapp_template.status_changed`.

### Outbound
- `POST /{phone_number_id}/messages` `messaging_product: "whatsapp"`, recipient `to` = wa_id when known, else `recipient` = BSUID (sending by BSUID supported from 2026-07; if both, `to` wins). `context.message_id` = latest customer wamid when replying to specific message (default off; on when room setting `quote_reply`).
- Text: `type: text`, `text.body` max 4,096, `preview_url: true` when exactly one URL. Split max 4.
- Media: upload `POST /{phone_number_id}/media` (multipart, returns media id valid 30 days) then send by id; one media per message; caption max 1,024 on image/video/document. Limits: image JPEG/PNG 5 MB, video MP4/3GP 16 MB, audio AAC/AMR/MP3/M4A/OGG(opus mono) 16 MB, document (TXT, XLS(X), DOC(X), PPT(X), PDF) 100 MB, sticker WebP 100 KB static / 500 KB animated. Image over 5 MB or non-JPEG/PNG image: sent as document if type in document list, else signed link in text. Other file types: signed link.
- Template (`channel_options.template`): `type: template`, `template.name`, `language.code`, `components` with body/header text parameters in order. Only `approved` templates. Allowed outside window.
- Window check in driver (`WindowState`, X3): `channel_contact_states.window_expires_at > now` else `ErrWindowClosed` (`whatsapp_window_closed`). Race with Meta: error 131047 (re-engagement message: more than 24 h since customer reply) mapped to same.
- Read receipts: `MarkRead`: `{"messaging_product":"whatsapp","status":"read","message_id":"<latest inbound wamid>"}` (marks earlier messages read too). Typing: same request with `"typing_indicator":{"type":"text"}`; dismissed on reply or after 25 s; sent only when `send_read_receipts` true (typing implies read).
- Throughput: 80 messages/s per number default (inbound + outbound), auto-upgrade to 1,000 for unlimited-tier numbers; bucket `wa:{phone}` 70/s. Error 130429 (throughput): `RateLimitedError{1 s}` growing. Pair rate limit 131056 (too many messages to same user): `RateLimitedError{6 s}` then exponential to 60 s. 131048 spam rate limit: permanent fail, connection warning. 80007 (WABA rate limit): `RateLimitedError{60 s}`.
- Error mapping (`errors.go`, extended as Meta adds): 131047 window closed; 131026 undeliverable (not on WhatsApp / old app / terms): `ErrRecipientUnreachable`; 131049 marketing per-user limit: permanent "Meta limited marketing messages to this user"; 131042 payment issue: permanent + checklist `payment_method` action_required; 131031 account locked: `ErrConnectionBroken`; 190 token invalid/expired: `ErrConnectionBroken`; 132000-132016 template errors (params mismatch, paused, disabled, not found): `ErrContentRejected`; 133010 number not registered: `ErrConnectionBroken` + checklist; 131052/131053 media download/upload error: transient once then permanent; 131051 unsupported type: `ErrContentRejected`; 368 temporarily blocked for policy: `ErrConnectionBroken` with Meta text.

### Formatting
- Out: `*bold*`, `_italic_`, `~strike~`, ```` ```mono``` ````, `` `inline code` ``, `> quote`, `- ` / `* ` bullet lists, `1. ` lists. Markdown links `[t](u)` rendered `t (u)`; headings to `*bold*`; tables to monospace block; nested emphasis flattened (WhatsApp does not nest reliably).
- In: reverse: `*b*` to `**b**`, `_i_` to `*i*`, `~s~` to `~~s~~`, mono kept; ambiguous asterisks (math `2*3*4`) left literal when not word-bounded.

### Templates (minimal MVP)
- Sync: `GET /{waba_id}/message_templates?fields=id,name,language,status,category,components,quality_score,rejected_reason` paged.
- Create: name (`^[a-z0-9_]{1,512}$`), language, category `utility` or `marketing` (authentication read-only), header text (optional, max 60 chars, max 1 param), body text (max 1,024 chars, `{{1}}`..`{{n}}` sequential, example values required by Meta: request field `examples`), footer (optional, 60 chars), quick-reply buttons (max 3, text 25 chars). No media headers, no URL/phone buttons at MVP. `POST /{waba_id}/message_templates`; status `pending` until webhook.
- Delete: `DELETE /{waba_id}/message_templates?name=` (deletes all languages) or `&hsm_id=` (one). Name reuse blocked by Meta for 30 days after delete: 409 `template_name_taken` explains.
- Pricing note (Meta, 2026-10-01): utility templates inside open window now charged; service (free-form) messages free first 1,000 per number per month then charged. Composer shows "Paid by Meta per message" hint on templates.

### Lifecycle
| Signal | Effect |
|---|---|
| 190 / token invalid | `error` `whatsapp_token_invalid`; shared mode reconnect re-runs Embedded Signup |
| `account_update` `PARTNER_REMOVED` / app unsubscribed / Meta deauthorize callback | `disconnected` reason `meta_deauthorized` |
| `account_update` `ACCOUNT_DELETED` / phone deleted | `disconnected` reason `account_deleted` |
| `account_update` `DISABLED_UPDATE` / violation | `error` with Meta reason; checklist account_status action_required |
| quality RED / messaging limit decrease | `channel.updated` warning, checklist |
| `user_changed_user_id` / `user_changed_number` system message | `RekeyWhatsappUser`: add new BSUID/phone identities to same contact, rekey open conversation `external_thread_key`, keep old identity (verified) for history |

Sources: developers.facebook.com WhatsApp docs: pricing (per-message since 2025-07-01; service free 2024-11-01 to 2026-09-30; from 2026-10-01 1,000 free service messages per number per month, utility in window charged), webhooks overview (3 MB payload, retries up to 7 days), webhooks components, business-scoped-user-ids, typing-indicators, business-phone-numbers/media, messaging-limits, throughput (80 mps, 1,000 upgrade, 130429), embedded-signup overview (v4; v2/v3 deprecated 2026-10-15; onboarding 10 per 7 days, 200 after verification), edit and revoke webhook references (Coexistence only); Graph API versions (v26.0 released 2026-07-29).
