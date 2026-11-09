# Telegram support channel research

Research date: 2026-11-08. Subject: how best support products run on Telegram, Telegram platform 2026, gaps vs spec 05 (`specs/05-chat-app-channels/`). Lens: dimmi first users (vibe coders, solo devs, crypto/web3, dev tools; many live in Telegram groups).

Tags: [unverified] = single source or conflicting sources. [3P] = third-party or competitor-written claim.

## TL;DR

- Market split in two. **Helpdesks** (Intercom, Chatwoot, Crisp, respond.io, Zendesk via app) do bot DM only: paste BotFather token, DM lands in inbox, AI answers whole message. **Community tools** (Mava, Pylon, Interchat, LiveChat app, Chat Data, SUCH, Rose/Combot) do groups. Nobody does both well plus modern Bot API.
- Intercom/Fin Telegram (June 2026): bot DM only, Fin auto-answers, quick replies and CSAT as inline buttons, ticket forms behind "Log in with Telegram", multi-bot with brand routing. Gaps: no groups (3P Interchat $49/mo fills it), no Telegram Business (community idea Aug 2026, no reply), new users land as Leads with manual merge, one conversation per user per bot, admins cannot unlink Telegram identity themselves, replies only from Intercom.
- Groups is where web3/dev communities live. Mava built a company on it: public group AI (answers only confident support questions, ignores replies and small talk, silent when unsure) plus private ticket bot via `/start`. Pylon uses teammates' personal Telegram accounts (MTProto session, phone code) to turn shared B2B groups into issues.
- Telegram shipped an AI-agent platform in 2026. Bot API 9.3 to 10.3 (Dec 2025 to Aug 2026): `sendMessageDraft` streaming for all bots (9.5) with "Thinking..." placeholder and user Stop button (10.3); private-chat topics (9.3/9.4); Guest Mode (10.0: bot answers when @mentioned in any group without being member); ephemeral group messages visible to one user (10.2); Rich Messages in GitHub-flavored Markdown with tables and buttons (10.1); Managed Bots (9.6: one link creates customer's own bot, manager gets token); Business bots no longer need Premium (10.0); colored buttons (9.4).
- No helpdesk found using any of these yet. Intercom ships whole messages, no streaming, no topics, no guest mode. Window open for dimmi to be first "AI-native Telegram" helpdesk.
- Identity is weak spot everywhere. Telegram gives numeric id + name only. Fixes available: `?start=` deep link (64 chars) from widget, `login_url` button / Login Widget for verified link to web account. Intercom only uses Login for ticket forms.
- Spec 05 MVP (private chat, whole messages, no buttons, no groups, no identity handoff, no CSAT) equals Intercom minus buttons/CSAT. Matches incumbent, beats nobody. Cheapest standouts: streaming, inline buttons (handoff + CSAT), deep-link identity, group mention answers via Guest Mode / privacy-mode mention. All small given go-telegram/bot already on Bot API 10.3.

## Market map

| Product | Entry point | AI on Telegram | Teammate reply | Groups | Business account | Price for Telegram |
|---|---|---|---|---|---|---|
| Intercom / Fin | Bot DM (token) | Fin auto, whole msg, inline-button quick replies + CSAT | Inbox only | No (3P Interchat/Octopods) | No | Included; Fin $0.99/outcome (see `competitors/fin.md`) |
| Zendesk | 3P Marketplace app (AssureSoft) or Sunshine Conversations API | Not native [unverified] | Zendesk ticket | App: privacy off -> all msgs | No | App $10 to $40/mo [3P] |
| Chatwoot | Bot DM (token) | Captain: auto-answer, suggest, translate, handoff | Inbox | No | Yes since v4.3.0 (2025-06-18) | OSS; cloud plans |
| Crisp | Bot DM (token) | Not documented for Telegram | Inbox | No | No | Mini plan and up |
| respond.io | Bot DM (token), QR growth widget | Workflows/AI agent | Inbox | No (roadmap request) | No (explicitly rejected) | Platform plan |
| Trengo, Freshchat/Freshdesk, Tidio, Featurebase, Plain | No native Telegram found. Zapier/viaSocket glue only | - | - | - | - | - |
| LiveChat / Text | 3P app (FEDYK): bot or regular account | Via ChatBot product | LiveChat | Yes, groups + topics, chat rating | No | 3P app price n/a |
| Mava | Bot DM `/start` (ticket), group bot | Public group AI + private ticket AI | Mava inbox | Yes (core) | No | Free 30 req, $39 / $99 / $199 / $599 |
| Pylon | Teammates' personal accounts (user session) | Agents generic, not Telegram-specific | Pylon, two-way incl. reactions | Linked chats -> issues | n/a | All plans |
| Interchat (Intercom add-on) | Bot, regular account, Business chatbot | Relays Fin | Intercom | Yes, per-group conversation, topics as conversations | Yes | $49/mo for 5 accounts, +$10 each |
| Chat Data | Bot | GPT bot, group mention/reply/`/ask` (2026-04-17) | Handoff | Yes | n/a | n/a |
| SUCH (such.chat) | Bot | ChatGPT integration | Telegram-native panel | PRO feature | n/a | Free, PRO paid |
| 14.ai, MetaCRM, Converge | Crypto-targeted AI / ticket bots | Claims groups + DMs [3P] | Own inbox | Claimed [3P] | n/a | Converge flat $49 [3P] |
| Rose, Combot | Group moderation bots | Keyword filters/notes (static FAQ) | n/a | Yes, `/report` and `@admin` to admins | n/a | Free / from $10 [3P] |
| Livegram / OSS "topic per user" bots | Bot DM relay | None | **Inside Telegram**: each user = forum topic in admin supergroup | n/a | n/a | Free / OSS |

## Intercom / Fin

Launch June 2026 ("Fin over Telegram", tagged available to all). Help articles 15690842 (setup) and 15690843 (how it works).

- Entry: bot DM only. Admin pastes BotFather token in Settings > Channels > Telegram. Intercom validates, registers webhook, logs install in teammate activity. Test: DM bot, conversation in inbox in seconds.
- Multi-bot: many bots per workspace, each with own brand (multibrand plan): sets Fin persona + help center. Read-only `Telegram bot` conversation attribute for routing/reporting (only conversations after 2026-08-12, no backfill, not in REST API). Broadcasts get "From bot" picker.
- One bot = one workspace.
- Fin: auto-answers DMs. Workflows at parity with other messaging channels. Quick replies and CSAT render as inline keyboard buttons (payload cap 64 bytes = Telegram `callback_data` limit).
- Ticket forms: hosted page behind Telegram Login Widget (verifies Telegram account). Admin must `/setdomain` in BotFather to `app.intercom.com` / EU / AU per bot. Clever: reuses Telegram login as identity proof, but only for forms.
- Inbound types: text with formatting, photo, video, document, audio, voice, video note, animation. Outbound: text, photo, document only. No voice/video notes. Stickers outbound fail silently.
- Customer edit: not applied; shows as operator note.
- Replies "must always be sent from within Intercom". Native Telegram replies (from bot account tools) do not sync.
- Identity: new user = Lead with numeric id + display name. Manual merge after first message. One Telegram account per contact per bot. Unlink only via Intercom support ticket. Old account messaging again creates new contact.
- One conversation per user per bot ("Telegram doesn't support separate conversations" — false since Bot API 9.3 private-chat topics).
- No read receipts; email notification fallback always fires after teammate reply.
- Limits listed: 4,096 chars, 1,024 caption, 30 msg/s global, 1/s per chat. Oversize inbound files skipped silently.
- Groups: not documented. Setup article mentions two bots in same group each open own conversation (duplicate messages) [unverified: implies some group ingestion]. 3P connectors (Interchat, Octopods) sell group + Business support on top.
- Telegram Business: not supported. Community idea 2026-08-18, 2 votes, no staff reply; 3P vendor replied pitching own app.
- Disconnect deletes token + webhook secret permanently; reconnect needs new token from BotFather.

Takeaway: polished table stakes (buttons, CSAT, login-verified forms, multi-brand). Nothing Telegram-native (no streaming, no topics, no groups, no Business). Identity UX poor.

## Zendesk

- No native Telegram channel in Zendesk messaging found. Sunshine Conversations (ex-Smooch) has Telegram integration: text, image, GIF, location full; files partial (arrive as link); link/postback/reply buttons; carousels; typing + read via activity API; files over 20 MB alert. Standalone Sunshine dashboard only for pre-April-2023 legacy plans [3P eesel].
- Practical route: Marketplace app "Telegram Channel" by AssureSoft: bot -> tickets, privacy off for groups. $10/mo (3 bots), $20 (6), $40 unlimited [3P, listings disagree].
- Takeaway: Zendesk ignores Telegram. Legacy buyers glue it.

## Chatwoot (main OSS rival)

- Bot token inbox. Webhook auto-registered. Agents need no Telegram account. Media both ways incl. voice, stickers, location, contact.
- Captain AI: answer from knowledge base, suggest replies, translate, hand off.
- Telegram Business mode since v4.3.0 (2025-06-18): BotFather `/business_mode`, paste same token, Chatwoot auto-detects. Limits: 24 h reply window, no typing, fewer message types; customer who chatted with bot before sees replies from bot not business. Keep business bot in own inbox.
- Groups: not documented.
- Complaints: GitHub #9306 Telegram messages missing until page reload; discussion #2353 "no docs"; #9735 Business messages not showing [unverified, open threads].
- Takeaway: Chatwoot beats Intercom on Business mode. Dimmi must at least match.

## Crisp

- Bot token, from Mini plan. Inbox + canned responses + bots + knowledge articles. No groups, no AI specifics documented. Integration updated 2026-07-10, nothing new visible.

## respond.io

- BotFather bot only. Explicitly rejects Telegram Business and Premium bots. Groups not supported (roadmap request).
- Status: only "Sent" (no delivered/read/failed). Unlimited broadcasts (no window). Files: image 10 MB, others 20 MB; unsupported/oversize -> link.
- Growth tools: `t.me` chat link, downloadable QR code per channel, Telegram website widget.
- Quick replies render on Telegram Web as single button in input box (Telegram Web quirk) — reply-keyboard pitfall; inline keyboards avoid it.
- Takeaway: marketing-grade, not community-grade.

## Trengo, Freshchat/Freshdesk, Tidio, Featurebase, Plain

- No native Telegram found in official docs. Freshchat channel lists: web, WhatsApp, Instagram, Messenger, SMS, LINE, Google, email [3P]. Tidio help center lists Messenger, Instagram, WhatsApp; Telegram only via Zapier. Featurebase: no Telegram (only viaSocket glue). Plain: none. Trengo: Telegram via bot token per migration guide [unverified].
- Takeaway: most of the field skips Telegram. Dimmi Telegram = differentiator vs Featurebase/Plain/Tidio audience.

## LiveChat / Text (3P app by FEDYK)

- Bots and regular Telegram accounts (up to 10). Private chats, groups, topics in groups (v1.8.0, 2024-09-05). Customer chat rating. Block user. Messaging mode (async). ChatBot product for automation, carousels + quick replies. Says no chat data kept on its servers.

## Mava (web3 community support, closest to our ICP)

- Channels: Discord, Telegram, Slack, email, web chat. Positioned for web3 communities.
- Telegram ticket bot: BotFather bot, set name/logo, disable privacy, add to group, share `t.me/<bot>` in group bio. Private ticket: user opens bot, `/start` creates ticket. Custom chat flows collect data first. Auto tag/assign.
- Public Telegram group AI (docs): privacy mode off. Answers any message it judges a support question it can answer from uploaded docs. Always answers when tagged. Never answers replies to other people's messages (no interrupting). Classifies greetings as social, ignores. Silent when unsure: no ticket, no notification. One shared toggle for Discord/Telegram/Slack public AI.
- Billing: one support request = one private ticket or one public AI answer. Over limit: new tickets hidden, public AI stops.
- Pricing (mava.app/pricing): Free $0 (30 requests, 1 seat), Founder $39 ($33 yearly, 100, 3 seats), Growth $99 ($83, 500, unlimited seats), Scale $199 ($166, 2,000; multilingual bot), Enterprise $599 ($499, 10,000; automations). Aggregators disagree [3P].
- Blog advice: group over few hundred people makes in-thread support impractical; VIP support as separate private flows; topics make groups usable.
- Weak: silent failures in groups (no escalation from unanswered public question), no group-to-private handoff documented, no streaming, multilingual gated to $199.

## Pylon (B2B shared groups)

- Model: org-level connect, then each teammate connects own personal Telegram account (phone + login code or QR "Add device", 2FA password). User sessions, not bot. "Show only personal channels" setting.
- Link Telegram chats to accounts (or create account from chat). Every new message in linked chat -> issue. Messages, files, reactions sync both ways.
- Bot ordering rule: connect personal account before adding bot to channels, else bot removed.
- Fits crypto B2B pattern: vendor and customer share a Telegram group (like Slack Connect). Personal-account sessions risk Telegram ToS/ban and are fragile (sessions expire).
- Available on all plans (Pylon facts page).

## Interchat and Octopods (Intercom add-ons)

- Interchat: bot (admin in group) or regular account (phone login, "Forward Telegram group chats"). Each group = own Intercom conversation; topics -> separate conversations. Telegram Business via chatbot (Settings > Telegram Business > Chatbots). Relays Fin + Workflows. $49/mo for 5 accounts, +$10 each, unlimited teammates. Advises against regular accounts (chat limits).
- Octopods: forwards all group messages; warns groups not for complex rules/automations.
- Takeaway: Intercom customers pay extra $49+/mo for groups + Business. Dimmi native = clear wedge.

## Crypto / Telegram-native long tail

- Chat Data (2026-04-17): group support via @mention, reply to bot message (quoted text passed as context), `/ask` command. Privacy mode on. Clean trigger model.
- SUCH: free bot builder, quick replies, team chat, broadcasts; groups and broadcasts PRO. No KB, no full ticketing (per Mava) [3P].
- CRMChat: Telegram user accounts (multiple), auto-lead per inbound message, pipeline stages, follow-up sequences, "account warmup" against bans. No SLA/CSAT [3P].
- 14.ai: "first AI-native support platform with Telegram first-class", crypto exchanges, groups + DMs, account actions via exchange tools [3P, unverified; domain now redirects to gloglo.com].
- MetaCRM: Telegram ticket bot (2024) for web3 [3P].
- Rose: group management; filters (keyword auto-reply), notes, `/report` and `@admin` to alert admins, log channels. Rose own FAQ May 2026: 55 M MAU, 2 M monthly active groups [3P via Metricgram, unverified]. Community admins already use keyword FAQ + report-to-admin patterns; AI should feel like smarter Rose.
- Livegram + OSS clones (pytdevs/support-bot, apkuzmin/telegram-support-bot, chatopic, XTV-SupportBot): every user DM -> forum topic in private admin supergroup; admins reply in topic, bot relays. Close topic = close/ban. XTV adds SLA timers and roles. Solo founders love it: support never leaves Telegram.

## Telegram platform facts 2026 that matter

Latest: Bot API 10.3 (2026-08-24). go-telegram/bot v1.27.x covers 10.3 (spec build-vs-reuse).

| Feature | Version / date | Fact | Use for dimmi |
|---|---|---|---|
| Streaming drafts `sendMessageDraft` | 9.3 (2025-12-31), all bots 9.5 (2026-03-01), empty text 10.0, `can_stop`/`keep_on_stop` + `MessageGenerationStopped` update 10.3 | Private chats only. Draft = ephemeral 30 s preview; final `sendMessage` persists. Empty text shows "Thinking..." placeholder. Per peer max 20 calls / 5 s, 40 / 30 s (same budget as `sendChatAction`) | Stream agent answers; Stop button cancels LLM call (saves BYOK tokens) |
| Rich Messages `sendRichMessage` / `sendRichMessageDraft` | 10.1 (2026-06-11), blocks 10.2, buttons/docs/expandable quotes 10.3 | Rich Markdown (GFM) or Rich HTML: headings, lists, task lists, tables, code, LaTeX, footnotes, media blocks, embedded buttons. Business accounts only if user can send rich | Send agent Markdown nearly as-is; citations as footnotes/buttons. Client support on older apps unknown [unverified] |
| Topics in private chats ("threaded mode") | 9.3, user-created topics flag 9.4 | Enabled in BotFather. `createForumTopic` works in private chat. Each topic = separate thread. **Subject to extra fee on Telegram Star purchases** (Bot Dev ToS 6.2.6) | Many conversations per customer (Intercom says impossible). Fee irrelevant unless bot sells via Stars |
| Guest Mode | 10.0 (2026-05-08) | Enable in BotFather Mini App. User @mentions bot or replies to its message in **any group or private chat, bot not member** -> `guest_message` update with summoning + replied message -> one reply via `answerGuestQuery`. No history, no participant list. Up to 3 guest bots per message | AI answers in community groups with zero admin setup or privacy-off. Any customer can summon support bot anywhere |
| Ephemeral messages | 10.2 (2026-07-14), params 10.3 | Group message visible only to one user + bot. Ephemeral commands invisible to others. Replace callback message per user. Replies within 15 s. Delivery not guaranteed if offline | Private answer or "continue in DM" nudge inside public group; ask for order id privately |
| Group privacy mode | long-standing | Default on: bot sees commands to it, replies to its messages, mentions, service msgs. Admin bots see everything. Changing needs re-add to group | Mention/reply model works with privacy on (Chat Data). Avoid asking admins to disable privacy |
| Business bots ("Secretary Mode") | 7.2 (2024), management 9.0, no Premium needed 10.0 | Owner picks chats bot can access. Rights: `can_reply` (only chats with incoming msg in last 24 h), `can_read_messages` (mark read via `readBusinessMessage`), delete. Messages look sent by owner. `business_message`, `edited_business_message`, `deleted_business_messages` updates. "Manage Bot" bar in each chat -> `/start bizChat<id>`. `user_chat_id` lets bot DM owner. Account still needs Business subscription per grammY docs [unverified post-10.0] | Founder connects own Telegram; AI answers as founder; real read receipts; edit/delete sync |
| Managed Bots | 9.6 (2026-04-03) | Manager bot with "Bot Management Mode". Link `t.me/newbot/{manager}/{suggested_username}?name=...` -> user confirms -> manager gets `managed_bot` update -> `getManagedBotToken`, `replaceManagedBotToken`, access settings | Cloud onboarding: one tap creates "AcmeSupportBot" owned by user, token flows to dimmi. No BotFather, no token paste |
| Bot-to-bot | 10.0 | DM other bots by username if both enabled; business bot can reply to bots. Loop safeguards mandatory | Loop protection in driver; future: dimmi bot as tool for other agents |
| Inline keyboards | 7.x+, colors 9.4, disabled buttons + `force_reply` 10.3 | `callback_data` 64 bytes. `style`: danger / success / primary. `login_url` button logs user into site seamlessly. Copy-text button 7.11 | Quick replies, "Talk to a human", "Solved", CSAT stars, verify identity |
| Reactions | 7.0, delete 10.0 | `setMessageReaction` by bot. `message_reaction` updates need admin in groups + explicit allowed_updates; private chat reactions arrive [unverified] | 👀 on receipt = "seen" (bots have no read receipts); 👍 on answer = resolution signal |
| Deep links | long-standing | `t.me/<bot>?start=<param>`: `[A-Za-z0-9_-]`, max 64 chars, arrives as `/start param`. `?startgroup=` adds bot to group with param | Signed short token from widget -> verified contact link |
| Web Login | long-standing | Login widget (`/setdomain`), `login_url` inline button | Verified link Telegram user <-> web account; Intercom uses for forms |
| Checklists | 9.1 (2025-07-03) | `sendChecklist` only on behalf of business account | Low value |
| Mini Apps | 6.x+, origin hardening enforced 2026-07-20 | Full web app inside Telegram | Later: forms, ticket history, help center inside Telegram |
| Communities | 10.2 | Supergroups + channels + bots linked around one topic | Watch: community-wide support bot |
| Channel direct messages | 9.2 | Channels have DM inbox as topics; bot with rights answers | Crypto projects with announcement channels get DM inbox |
| Member tags | 9.5 | `setChatMemberTag`, shown as `sender_tag` | Tag "Team" on staff in groups |
| Admin right `can_send_welcome_messages` | 10.3 | New admin right | Group onboarding message |
| Rate limits | FAQ | 1 msg/s per chat (bursts), **20 msg/min per group**, ~30 msg/s bulk; paid broadcasts up to 1,000/s at 0.1 Star each (needs 100k Stars + 100k MAU) | Group answers must batch; group bucket 20/min missing from spec buckets |
| Files | FAQ / local server | Download via `getFile` 20 MB, upload 50 MB. Local Bot API server: upload 2,000 MB and bigger downloads | Self-host option: point driver at local `telegram-bot-api` |
| Scale | Durov 2025-03 | 1 B+ MAU; Premium 15 M (2025-05) [3P aggregators for 2026] | |
| Telegram Business (app features) | 2024-03-31 | Hours, location, quick replies (`/` panel), greeting, away messages, chat links with prefilled text | Founders already set greeting/away; dimmi must not double-greet (Chatwoot/Interchat advise disabling native greeting) |

## Best-in-class experience (synthesis)

### Customer
- Finds support where they are: `t.me/acmesupportbot` link, QR, "Continue in Telegram" button in web widget (deep link carries signed session -> already identified), or @mention bot in project's group.
- DM: bot greets once with 2 to 3 colored buttons (common intents). Types question. Sees "Thinking..." then streamed answer, Stop button. Final message rich-formatted (code blocks, steps, source links as buttons). Buttons under answer: "Solved" / "Talk to a human".
- Multiple issues = separate topics in same bot chat (billing topic, bug topic), each a dimmi conversation.
- Human handoff: clear status line ("Frantz from Acme usually replies in ~2 h"), typing while teammate writes, 👀 reaction when teammate opens.
- Group: @mention or reply -> short public answer in thread; sensitive stuff -> ephemeral "only you see this: continue privately" button -> DM with context carried. Never interrupts members talking to each other; silent on small talk.
- After resolution: one inline CSAT row (1 to 5 or 👍/👎), optional comment via `force_reply`.
- Business account mode: talks to founder's own account, AI answers look like founder, read ticks real.

### Teammate
- Main path: dimmi inbox, same as other channels; Telegram badge, username, language, verified link status.
- Solo-dev path: reply without leaving Telegram. Option A: private admin supergroup, topic per conversation (Livegram pattern), reply in topic -> customer; `/close`, `/assign` commands. Option B: Business mode, reply natively in own Telegram; echo recorded in dimmi.
- See edits (as note or diff), group context (previous N messages, replied-to message).
- Merge/unlink identity themselves (Intercom needs support ticket).

### AI agent
- Streams via `sendMessageDraft` (throttle under 20 calls / 5 s), handles `MessageGenerationStopped` (cancel model, bill nothing new).
- Emits buttons (clarify choices, handoff, solved); reads `callback_query` as user turn.
- Knows channel context: group vs DM, replied-to message, `language_code`, verified identity (via deep link/login) to unlock account-specific tools.
- Group policy: answer only confident support questions; mention always gets reply (even "I'll get a human"); escalate unanswered group question to private ticket instead of silence (Mava gap).
- Respects 4,096 / rich limits, splits cleanly, never pings everyone.

### Admin setup
- Cloud: one tap "Create Telegram bot" via Managed Bots link, or paste BotFather token (spec under 60 s bar). Self-host: paste token, polling if no public URL.
- dimmi sets commands, description, short description, profile hints; shows test link + QR.
- Toggles: groups on/off, Guest Mode instructions, Business mode detection, admin supergroup for native replies, streaming on/off, CSAT on/off.
- Health: webhook info, pending updates, privacy mode status, missing rights in groups (topics need `can_manage_topics`).

## Gaps vs dimmi spec 05

Spec 05 MVP: private chats, whole messages, typing, HTML formatting, media, blocked-bot detection, `callback_query` subscribed but no button feature. Deferred: groups/forum topics, deep-link identity, `sendMessageDraft`, CSAT on chat apps, edits, teammate echo, reactions.

| # | Gap | Spec today | Competitors | Recommend | Why |
|---|---|---|---|---|---|
| 1 | Streaming AI answers (`sendMessageDraft`, empty-text "Thinking...", Stop -> cancel) | Out (post-MVP) | Nobody in helpdesks does it; Telegram markets it as native AI UX | **Launch** | Available to all bots since 9.5; private chats = MVP scope; tiny driver surface (draft calls + final send, 20/5 s throttle). Stop saves BYOK tokens. Visible "AI-native" proof in demo. Fallback: typing + whole message |
| 2 | Inline buttons: quick replies, "Talk to human", "Solved", CSAT | Not in scope; `chat-app-csat` later | Intercom has quick replies + CSAT as inline keyboard; LiveChat app has chat rating | **Launch** (Telegram at least) | Table stakes vs Intercom. `callback_query` already in allowed_updates. 64-byte `callback_data` -> store payload server side, send short id. Gives resolution signal for billing/analytics |
| 3 | Deep-link identity handoff from widget + verified login | Out (post-MVP) | Intercom: Leads + manual merge, support-ticket unlink (pain); Login Widget only for forms | **Launch** (deep link) / **Soon** (`login_url` verify button) | Identity is #1 Telegram weakness. Widget "Continue in Telegram" with `?start=<≤64 char signed nonce>` stored server side links contact instantly. Lets AI use account tools safely. Self-serve unlink/merge in contact UI |
| 4 | Group support: @mention / reply-to-bot answers in groups, Guest Mode, ephemeral private follow-up | Later (`telegram-groups`), group messages ignored | Mava (core product), LiveChat app, Interchat ($49/mo add-on to Intercom), Chat Data; Intercom native lacks | **Soon** (first post-launch; launch if a design partner is web3) | ICP communities are Telegram groups. Mention/reply works with privacy mode on; Guest Mode needs zero admin rights. Start narrow: mention or reply -> AI answers in thread -> unanswered/escalated -> private conversation via deep link or ephemeral button. Full forum-topic rooms later. Add 20 msg/min group bucket |
| 5 | Telegram Business connection (answer as founder's own account) | Not mentioned | Chatwoot since 2025-06; Interchat; Intercom refuses (open idea) | **Soon** | Solo founders/crypto BD already run support from personal account. No Premium needed since 10.0. Gives real read receipts (`readBusinessMessage`), edits, deletes. Reuse WhatsApp X3 window resolver for 24 h `can_reply` window. Must detect business vs bot mode per connection; owner messages = teammate echo |
| 6 | Reply from Telegram (admin supergroup, topic per conversation) | Teammate echo later; Slack notification replies later | Livegram + OSS clones; Pylon via personal accounts | **Soon** | Solo devs live in Telegram; strongest "never open a dashboard" story. Bot is admin with `can_manage_topics` in team supergroup; topic per conversation; messages in topic -> teammate reply via member link. Same bot, no user sessions (avoids Pylon ToS risk) |
| 7 | Managed Bots one-tap bot creation | Paste token only | Nobody uses it yet | **Soon** (cloud) | Kills BotFather step; beats "under 60 s" bar; user still owns bot. Self-host keeps token paste |
| 8 | 👀 reaction on receipt, 👍 reaction as signal | Inbound reactions ignored | Pylon syncs reactions | **Launch** (outbound 👀), later inbound | Bots lack read receipts; reaction is honest "seen" status. One API call |
| 9 | Edits: `edited_message` as note | Dropped (later) | Intercom shows edit as operator note | **Soon** | Customers fix typos/add order ids; AI may answer stale text. Cheap: subscribe + note |
| 10 | Multiple conversations per user (private-chat topics) | Topics echoed `message_thread_id` only; thread key per chat | Intercom: one conversation per user per bot | **Later** | Real fix for "one thread forever" chats. Star-purchase fee only matters if bot sells via Stars. Map topic -> conversation key `telegram:p:{chat}:{thread}` |
| 11 | Rich Messages (GFM) instead of hand-written HTML converter | Telegram HTML converter + 250 golden cases | Nobody | **Later** (evaluate) | Could delete most of `format.go` and render tables/footnotes. Risk: old clients, business-account restriction. Keep HTML fallback |
| 12 | Growth surfaces: QR code, widget "Continue in Telegram" button, `startgroup` add-to-group link | `setup.test_link` only | respond.io QR + widget; Mava bio link | **Launch** (QR + widget button), cheap | Customers need a way in; deep link ties to #3 |
| 13 | Local Bot API server for self-host (2 GB files, no 20 MB download cap) | 20 MB download placeholder | Nobody | **Later** | Config switch: base URL. Self-host perk |
| 14 | Group rate bucket and loop safeguards for bot-to-bot | Buckets per chat 1/s + global 30/s | n/a | With #4 | Telegram FAQ: 20 msg/min per group; 10.0 bot-to-bot requires loop guards |
| 15 | Channel direct messages, Communities, Mini App ticket history | Not mentioned | Nobody | **Later** | Watch list for crypto projects with broadcast channels |

Launch delta size estimate: #1, #2, #3 (deep link), #8, #12 = driver + small 07/15 hooks, roughly days not weeks, all inside private-chat scope already chosen.

## Where dimmi beats everyone

- Only helpdesk with streamed, stoppable AI answers on Telegram (Telegram's own showcase feature, unused by Intercom/Chatwoot/Mava).
- Groups + DMs + Business mode in one free, self-hostable product. Intercom needs $49/mo Interchat for the same; Mava gates multilingual at $199 and bills per public answer.
- Identity that works: widget-to-Telegram deep link and login button, self-serve merge/unlink.
- Solo-dev mode: answer customers from Telegram itself, no user-session hacks.
- One-tap bot creation on cloud (Managed Bots); token paste + polling on self-host laptop.

## Sources

- Intercom setup: https://www.intercom.com/help/en/articles/15690842-setting-up-telegram-in-intercom
- Intercom how it works: https://www.intercom.com/help/en/articles/15690843-how-telegram-conversations-work-in-intercom
- Fin over Telegram: https://fin.ai/updates/fin-over-telegram
- Intercom community idea, Telegram Business: https://community.intercom.com/ideas/add-telegram-business-functionality-15293
- Interchat groups: https://interchat.io/telegram-groups
- Interchat Business: https://interchat.io/telegram-business
- Interchat FAQ: https://interchat.io/faq
- Octopods Telegram for Intercom: https://intercom.help/octopods/en/articles/3603991-telegram-intercom
- Sunshine Conversations Telegram: https://docs.smooch.io/guide/telegram/
- Zendesk Marketplace Telegram Channel app: https://www.zendesk.com/marketplace/apps/support/190271/telegram-channel/
- Sunshine Conversations guide [3P]: https://www.eesel.ai/blog/zendesk-sunshine-conversations
- Chatwoot Telegram docs: https://www.chatwoot.com/docs/product/channels/telegram
- Chatwoot Telegram feature page: https://www.chatwoot.com/features/telegram-integration
- Chatwoot issue #9306: https://www.github.com/chatwoot/chatwoot/issues/9306
- Chatwoot discussion #2353: https://www.github.com/chatwoot/chatwoot/discussions/2353
- Chatwoot discussion #9735: https://github.com/orgs/chatwoot/discussions/9735
- Crisp Telegram plugin: https://crisp.chat/en/integrations/telegram/
- Crisp help article: https://help.crisp.chat/en/article/x2nkse
- respond.io Telegram overview: https://respond.io/help/telegram/telegram-overview
- respond.io Telegram channel: https://respond.io/help/channels/telegram
- Freshchat review [3P]: https://dupple.com/reviews/freshchat
- Featurebase via viaSocket: https://viasocket.com/integrations/featurebase/telegram
- LiveChat Telegram app: https://www.livechat.com/marketplace/apps/telegram/
- Mava Telegram product: https://mava.app/product/telegram
- Mava pricing: https://mava.app/pricing
- Mava public Telegram group AI: https://mava.gitbook.io/mava-docs/ai-support/public-telegram-group-ai
- Mava Telegram setup: https://mava.gitbook.io/mava-docs/getting-started/integration-setup/telegram-setup
- Mava best Telegram support bot [3P]: https://www.mava.app/blog/best-telegram-support-bot
- Mava groups blog: https://mava.app/blog/how-to-provide-customer-support-via-telegram-groups-and-communities
- Mava llms.txt: https://www.mava.app/llms.txt
- Pylon Telegram integration: https://www.usepylon.com/integrations/telegram
- Pylon Telegram docs: https://docs.usepylon.com/pylon-docs/integrations/chat/telegram
- Pylon Telegram KB: https://support.usepylon.com/articles/7047547549-how-do-i-integrate-telegram-with-pylon
- Chat Data group support: https://www.chat-data.com/changelog/telegram-group-chat-support
- SUCH: https://such.chat
- CRMChat [3P]: https://crmchat.ai/blog/best-telegram-crm-support-ticketing
- 14.ai [3P]: https://14.ai/articles/missing-ai-layer-telegram-customer-support
- Rose docs: https://missrose.org/docs/moderation/log-channels/
- Rose stats via Metricgram [3P]: https://metricgram.com/alternatives/rose-bot
- OSS topic-per-user bots: https://github.com/pytdevs/support-bot , https://www.mintlify.com/apkuzmin/telegram-support-bot , https://github.com/yehuda-lev/chatopic , https://davdxpx.github.io/XTV-SupportBot/
- Bot API: https://core.telegram.org/bots/api
- Bot API changelog: https://core.telegram.org/bots/api-changelog
- Bot features (guest mode, ephemeral, streaming, topics, business, managed bots, rich messages, privacy, deep links, login): https://core.telegram.org/bots/features
- Bot FAQ (limits): https://core.telegram.org/bots/faq
- AI features for bots (threaded mode fee, draft limits): https://core.telegram.org/api/bots/ai
- grammY Business guide: https://grammy.dev/advanced/business
- Business bots without Premium [3P]: https://sendpulse.com/en/knowledge-base/chatbot/telegram/telegram-business
- Telegram Business announcement: https://telegram.org/blog/telegram-business
- aiogram sendMessageDraft: https://docs.aiogram.dev/en/latest/api/methods/send_message_draft.html
- OpenClaw streaming notes [3P]: https://ai-muninn.com/en/blog/openclaw-telegram-sendmessagedraft-streaming
- Telegram 1 B MAU: https://kun.uz/en/news/2025/03/20/telegram-surpasses-1-billion-active-users-closes-in-on-whatsapps-dominance
- Telegram stats [3P]: https://backlinko.com/telegram-users
