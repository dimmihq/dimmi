# Discord support research

Research date: 2026-11-08. Subject: how best support products and dev-tool communities run customer support on Discord. Lens: dimmi first users (vibe coders, solo devs, dev-tool startups whose community lives in Discord). Read with `specs/05-chat-app-channels/README.md` and `providers.md` section 2.

Tags: [unverified] = single source or conflicting sources. [3P] = third-party or competitor-written claim.

## TL;DR

- Dev communities do not do support in DMs. They do it in public **forum channels**: one post = one question, tags for status, "mark solution", archive. Next.js, Drizzle, Prisma (`#ask-ai`), OpenAI dev Discord (`#ask-ai`) all public. DM to a bot is rare path, and "staff never DM you" anti-scam culture makes it worse [unverified, common server rule].
- Every serious helpdesk Discord integration in 2026 starts from **forum posts or channel threads**, not DMs: Plain (forum only), Pylon (forum + channel + `:ticket:` emoji), Thena (forum + channel `all|emoji|none`), Unthread (forum + `/support`), Intercom (connected channels, threads mirror, right-click "Create Private Conversation"), Fin (mention / emoji / keyword trigger, public thread, sensitive goes private thread). AI-only bots (kapa, Inkeep, RunLLM, CrawlChat) all do mention + forum auto-reply.
- **Nobody found does DM-only support.** dimmi spec 05 MVP (DMs only, forum/threads/`/support` deferred) is below table stakes for exactly the audience it targets.
- Private path = **private thread** from a button panel or command (Mava, Ticket Tool, Tickets, Fin "sensitive" move, Intercom right-click). Not DM. Private threads dodge 500-channel cap; 1,000 active threads per guild cap applies.
- AI pattern that wins: **answer first in thread, then buttons**: "Solved" / "Ask a human" (Inkeep "Ask for help" pings role, Mava "Chat to a human", Forum Assistant solved button). Reactions 👍/👎 as feedback (CrawlChat, Fin trigger by emoji). Confidence threshold rarely exposed; "say I don't know, then escalate" is the norm.
- **Knowledge capture is the moat nobody owns end to end.** Answer Overflow (MIT, ~2k stars) turns threads into SEO pages + MCP; Ferndesk reads up to 3,000 closed forum threads as source; Mava trains on Discord replies (Growth $99+); CrawlChat "learn" command; RunLLM ingests channel history filtered by trusted roles. No one ships: solved thread -> reviewed help article -> AI cites it next time -> public page, in one product.
- Big 2026 platform shifts: (1) privileged intents (MESSAGE_CONTENT) now gated by **10,000 users** not 100 servers (2026-06-10), annual re-review; below 10k toggle freely. Self-hosted own bot almost always under. (2) Discord **Server Web Pages from 2026-11-16, Google indexing of selected public channels mid-Dec 2026** for Discoverable servers [3P] — Discord itself starts eating Answer Overflow's SEO lane. (3) Channel obfuscation for bots from 2026-11-16. (4) Components V2 + modals with selects, checkboxes, file upload: rich ticket intake possible without web forms.
- Pricing landscape: ticket bots $0-12/mo (Ticket Tool premium $8/mo, Tickets whitelabel $6.99); Mava $0-599/mo by request volume (Free 30 req); Plain Discord only on top plan; Pylon $89+/seat; Intercom seats + $0.99/outcome; Featurebase Discord = feedback bot only. Open-source self-hostable helpdesk with full Discord support: none (Chatwoot none native, Crisp `!reply` commands).
- dimmi opening: forum + thread + private-thread modes, AI answer with solved/human buttons, native teammate/helper replies synced, solved thread -> knowledge, free and self-hostable, own bot under 10k users means MESSAGE_CONTENT free toggle. That beats everyone on this list for the dev-tool community case.

## Competitors

### Intercom / Fin (Discord channel)

| Item | Detail |
|---|---|
| Status | Alpha 2025 ("native discord channel alpha" article id), changelog 2026-03-24 "Connect Discord to your Intercom inbox"; Fin over Discord available to all March 2026 |
| Entry point | Connected channels (public or private). Customer message in connected channel = new conversation. Fin triggers: **mention, emoji reaction, specific keywords** (fin.ai update). Right-click message -> Apps -> Intercom -> **Create Private Conversation** (private thread) |
| Public vs private | Fin answers straightforward questions in **public thread**; "sensitive details" auto-move to **private thread** |
| Threads | "Intercom threads mirror Discord threads". Replies land in same Discord thread |
| Teammate reply | From Intercom inbox. Posts under **teammate name + avatar** (uses channel webhooks: docs warn channel must have under 10 webhooks, Discord cap). No reply-from-Discord sync documented |
| Fin escalation | Handoff to moderators or team via "configurable triggers", full context (connector page). Uses Workflows like any channel |
| Setup | Settings > Channels > Discord, OAuth, server admin required, "Add channels". Private channels: add Intercom member + role, View Channel |
| Limits | Fin bot hit Discord verification threshold: "can't join new servers" until Intercom verifies (help article). No nested lists |
| Pricing | Seats ($29-132) + $0.99/Fin outcome; Discord not separately priced |
| Standout | Trigger choice (mention / emoji / keyword), auto public -> private move for sensitive data, teammate avatar via webhook |
| Gaps | No forum-specific docs, no tags/solved, no CSAT on Discord found, no knowledge capture from threads, closed SaaS |

### Mava (mava.app)

| Item | Detail |
|---|---|
| Who | Community-first support tool, web3 roots (1inch, Wormhole, Immunefi, Layer3), now SaaS/AI/gaming |
| Entry point | **"Create Ticket" button panel** -> opens **private thread**. Plus **public channel AI**: detects questions in public channels, answers before ticket needed |
| AI | Trained on docs, FAQs, past conversations (websites, Google Docs, Notion, GitBook). In tickets: AI Answers per chatbot-builder step; drafts for agents; user's language. Escalation = user presses **"Chat to a human"** CTA, ticket lands in inbox. No confidence threshold documented |
| Intake | Chatbot builder: buttons, forms, custom fields, auto tags (billing / account / technical) |
| Teammate reply | Shared inbox (web) across Discord, Telegram, email, web chat; assignment. Users tagged in each answer |
| Close / status | Resolved -> **CSAT**; thread **auto-archived after CSAT**; transcript sent on resolve. Option: archive resolved after 6 h when over 1,000 tickets in 7 days (active-thread cap) |
| Knowledge capture | "Automatically train AI on Discord replies" (Growth+) |
| Pricing (mava.app/pricing, monthly / yearly) | Free $0: 30 req, 1 seat, 2 sources. Founder $39/$33: 100 req, 3 seats. Growth $99/$83: 500 req, unlimited seats. Scale $199/$166: 2,000. Enterprise $599/$499: 10,000. Request = one private ticket or one public AI answer. Over limit: **new tickets hidden and public bot stops** |
| Conflicts | Older listings: Free 100 req, "Bullish $83", "Whale $599", Diamond $199 [3P, stale] |
| Standout | Private threads not channels (no 500-channel cap), public-channel AI + ticket in one bot, CSAT closes thread |
| Gaps | Request cap kills public bot when exceeded; mostly Discord/Telegram; eesel says knowledge shallow [3P]; closed source |

### kapa.ai

| Item | Detail |
|---|---|
| Who | AI answer engine for dev docs. Prisma (`#ask-ai`, 10k+ questions/mo across all deployments, 20%+ non-English), OpenAI dev Discord `#ask-ai` (testimonial) |
| Entry point | **Mention mode** (default, text channels). **Forum auto-reply** checkbox (off by default): answers every new forum post without tag |
| Setup | Single Channel or Global Server integration; guild id; install link perms `274877908992` (= Send Messages + Send Messages in Threads); enable with `!kapa_enable_channel <id>` / `!kapa_enable_server <id>` (admin-only text command, visible in channel) |
| Config | First reply text (normal + forum), follow-up text, footer, **PII error text** (refuses when PII detected), anonymized user id tracking |
| Human handoff | Not in Discord docs. Feedback API (upvote/downvote) exists; Discord buttons not documented |
| Status signals | None (no solved, tags, close) |
| Pricing | Enterprise sales, not public |
| Standout | Forum auto-reply as one checkbox; PII guard; big dev logos |
| Gaps | Answer-only bot. No inbox, no teammate flow, no ticket, no solved |

### Inkeep

- Mention bot (`@Ask Inkeep`) anywhere; **Auto-reply Channel IDs** answer every new thread without tag, incl. forum channels. Suggests dedicated `#ask-ai` channel.
- **"Ask for help" button on every AI message**: tags configured users / roles / bots.
- **AI Draft mode**: bot posts draft on new threads for team to use as starting point. Loop-In mode: tag only when needed, uses thread context.
- Custom bot name on Discord (logo Slack only). Pricing enterprise.
- Standout: escalation button pinging role inside Discord, draft-for-humans mode.

### RunLLM

- Discord deployment: server id + channel id, pick response type and reply mode, install app. Private channels need explicit invite.
- **Ingests Discord message history** as knowledge: filter by channel, days, **trusted role IDs** (only learn from staff/helpers answers). Admin perms to install.
- Pricing enterprise. Standout: trusted-role filter for learning.

### Answer Overflow

| Item | Detail |
|---|---|
| What | Open source (MIT, ~2.0k stars, Bun/Convex/Next.js 16/discord.js). Bot indexes chosen channels, builds web page per thread, sitemap to Google. MCP access for AI agents (README). Large servers: Valorant, Cloudflare, C#, Nuxt; Drizzle Team mirror pages live |
| Consent | Users' messages public only after consent prompt (membership screening, verification channel or forum guidelines "Read the rules") |
| Mark solution | OP, admins, or members with ManageChannels / ManageThreads / ManageGuild. Adds solved indicator (configurable **solved tag** per channel) + bot posts jump-to-solution message. Enabled via `/channel-settings` "Help channel utilities". Forum channels supported |
| Indexing | ~every 24 h; Google decides |
| Pricing | Freemium, ad-supported free [3P]; pricing page rate-limited, not verified [unverified] |
| Threat | Discord's own Server Web Pages + Google indexing (Dec 2026) overlaps core value [3P] |
| Lesson | Solved threads are SEO asset. Consent + solved tag + jump link = minimum. dimmi can do same into own help center with review |

### Ticket Tool (tickettool.xyz)

- Classic **button panel** in a channel -> creates **ticket channel** (not thread) per ticket. Panels, categories, forms, claim, transcripts.
- Premium $8/mo or $66/yr via website (FAQ), sub tied to purchase email not Discord account. Buying in Discord app store: no annual, no transfer.
- Mava claims [3P]: no AI, no shared inbox, free tier 10 tickets/min, channel limits (500 channels / 50 per category).
- Separate newer "Ticket Tool" (ticket-tool.app) listing: free 5 categories, 3 panels, 30-day transcripts; Community $5, Pro $12 with AI assist [3P, unverified, possibly different product].
- Lesson: panel button is what Discord users expect for "private help".

### Tickets (tickets.bot)

- Free: unlimited tickets, panels, forms, claiming, archives, transcripts, **thread mode** (tickets as private threads), ratings. Whitelabel $6.99/mo: own name/avatar/status. Thread mode cannot be claimed (Discord threads have no owner transfer) [3P].
- Premium: insights, per-staff performance, **satisfaction ratings** [3P].
- Lesson: whitelabel bot identity matters to communities; self-host gives it free.

### Helper.gg

- "AI-powered and automatic whitelabeled ticket bot", ~41,900 servers (discordbotlist) [3P]. AI agent based on server info is premium. Little official doc found [unverified].

### Featurebase

- Discord integration = **feedback capture only**: `/feedback` command, right-click Apps -> Send to Featurebase. All plans (counts as Free's one integration).
- Public request "Discord integration for support module" open; May 2025 changelog line "Sync your Discord channel to Featurebase Inbox" ambiguous [unverified]. Pricing page lists Discord on Professional ($59) per our featurebase.md — conflicts with help docs [unverified].
- Lesson: not a support competitor on Discord yet.

### Plain

- Launched 2025-01-30 "first Discord integration for B2B support". **Forum channels only**; text channels and private threads unsupported. Add bot to each forum.
- Replies from Plain post via Plain bot; teammate clicks **"Connect your profile"** once -> replies under own name.
- Reporting by channel (volume, first response, time to close), auto-responders (time, channel, content). Webhooks `thread.discord_message_received/sent/updated`.
- Limits (docs): no mentions/reactions, Discord avatars not shown, **cannot match Discord users to existing customers by email**. Frontier plan only.
- Lesson: forum-first is enough for B2B dev tools; identity mapping is open problem.

### Pylon

- Forum posts auto-create issues; regular channels: every message = ticket, or **`:ticket:` emoji** opt-in. **Bidirectional sync of messages, files, reactions.** Channels linked to Accounts (account = customer company). Support Agent (AI) answers on Discord with account context.
- $89 / $139 per seat (annual), agent credits. Discord every plan.
- Their evaluation checklist (useful): native vs notifications; per-channel ticket control; forum support; two-way replies/files/reactions; same queue/SLA/reports; tie Discord user to account; AI that hands off; native setup vs own bot; pricing model.

### Thena

- Regular + forum channels. Regular modes `all | emoji (default, 🎫) | none`; forum: every post = ticket. Comments, reactions, attachments sync both ways.
- Setup only via API + `!admin` text commands (`auth`, `map-team`, `set-ticket-mode`), no UI. AI "insights" vague.
- Lesson: emoji-to-ticket default in chatty channels avoids noise.

### Unthread

- Community-maintained bot (wgtechlabs, Docker/Railway): **`/support` command** or forum post -> ticket; real-time sync. Not first-party [unverified].

### Zendesk, Chatwoot, Crisp

- Zendesk: 2018 Discord integration for gaming; Pylon says retired Dec 2022 [3P]. Today Zapier/marketplace glue (one-way).
- Chatwoot: **no native Discord channel**. Third-party `discord-tickets/chatwoot-modmail` only. Open-source gap dimmi fills.
- Crisp: plugin mirrors conversations **into** a Discord channel for team; team replies via `!reply <msg>` / `!resolve` as Discord replies. Essentials+. Team-side notification channel, not customer channel.

### Ferndesk

- Discord forum channels as **knowledge source**, up to 3,000 closed threads. Does not answer in Discord (see `research/competitors/ferndesk.md`).

### Newcomers and long tail (2025-2026)

| Tool | Notable |
|---|---|
| Forum Assistant (top.gg / discordbotlist, ~62 servers) | Auto-reply on new forum posts from FAQ docs; **auto-tags Waiting / Discussing / Solved**; one-click Solved for OP; auto-close marks Solved after N days idle [3P] |
| CrawlChat | Reply-as-thread option; **"@bot learn"** in thread adds whole thread as knowledge item; 👍/👎 reactions = rating; answer-correction flow from low ratings |
| TicketCord (PH 2026-02-24) | Ticket bot + AI auto-responses from KB, routing, staff analytics |
| Threadline | Two-way Discord forum thread <-> GitHub issue sync, AI labels bug/feature/question/spam. Free 1 repo, Pro $9/mo |
| GitHub-Issues-Discord-Threads-Bot (several OSS forks) | Forum post -> GitHub issue, comments mirrored |
| Quickchat | "Answer first, then escalate": AI answers in channel; if missing, opens **private non-invitable thread**, adds member, pings support role with AI summary; dedupe one ticket per conversation; `allowed_mentions` blocks `@everyone` |
| Helpshift, ClearFeed | Helpshift: Discord bot -> agent dashboard, CSAT, forum support limited [3P]. ClearFeed: Discord messages/forum posts -> Slack triage channel, replies sync back [3P] |
| Wallu | AI layer on top of Ticket Tool / Tickets / Helper.gg tickets [3P] |

## How top dev communities actually run it

| Community | Pattern | Evidence strength |
|---|---|---|
| Next.js (Vercel) | Help **forum**; helpers mark accepted answer; accepted answers count toward **Helper role**; open-source bot + web app `nextjs-forum.com` (MIT) indexes forum posts to public site; helper/mod roles can select answer on behalf of OP; points milestone gives "regular" role; scope rule: Next.js only, not Vercel hosting | GitHub repos (joulev/nextjs-faq, rafaelalmeidatk/nextjs-forum) |
| Prisma | `#ask-ai` channel with kapa; humans in topic channels | kapa case study [3P vendor] |
| OpenAI dev Discord | `#ask-ai` powered by kapa | kapa testimonial [3P vendor] |
| Drizzle | Help forum mirrored on Answer Overflow (public, indexed) | answeroverflow.com pages |
| Supabase | Discord for code questions ("many answered in minutes"), `#help` + per-product channels; paid support separate (dashboard tickets). No AI bot confirmed | supabase.com/support; channel list [3P] |
| Cursor | Moved center of gravity to Discourse forum `forum.cursor.com`; mod: focus forum "to reduce duplicate questions / make things easier to find"; users: Discord hard to search | forum.cursor.com threads |
| Bun, Clerk | Not verified in this pass [unverified] | - |

Common shape:
- Public forum channel `#help` with tags (product area + status). Required tag on post.
- Community helpers answer most; staff answer hard ones; roles reward helpers.
- "Mark solution" by OP or helper -> solved tag -> archive/lock.
- AI in separate `#ask-ai` or as first reply in forum, never replacing helpers.
- Paid / account-specific issues pushed **out** of Discord to email/dashboard (privacy). This is where a private thread or DM -> inbox path fits.
- Pain: unsearchable, duplicates, knowledge lost -> Answer Overflow / own indexers / move to Discourse.

## Discord platform facts 2026 that matter

| Fact | Detail | Impact for dimmi |
|---|---|---|
| Privileged intents by user count | 2026-06-10: review needed above **10,000 unique users with access** across installs; below toggle freely; 90 days to apply after notice; can keep growing during review; **annual reapply**; separate from App Verification | Self-host own bot: MESSAGE_CONTENT free. Cloud shared bot: one mid community (10k members) triggers review; apply early, design fallback modes |
| MESSAGE_CONTENT exemptions | Content visible without intent for: DMs, messages **mentioning the bot**, bot's own messages; replies to bot [3P]. Forum post title + tags visible via THREAD_CREATE without content [inferred] | Mention, slash command, context menu, button flows work with zero privileged intent. Forum auto-answer needs intent |
| App Verification | Unverified bot capped at 100 servers (Intercom hit it) [3P for 2026 numbers] | Cloud shared bot must verify before launch |
| Forum channels | Thread-only (type 15); post = thread + first message; `available_tags` (20 max), `applied_tags` (5 max) [limits from memory, docs page silent: unverified]; `REQUIRE_TAG` flag; pinned posts never archive; list or gallery layout | Status via tags (Open / Answered / Solved / Needs human) |
| Threads | Auto-archive by inactivity (1 h / 24 h / 3 d / 1 w), early archive near cap; **1,000 active threads per guild**; lock without archive (`MANAGE_THREADS`); sending message unarchives unless locked | Close = archive (+ lock optional). Re-open on new message. Watch active cap |
| Private threads | Only in text channels (not forums); visible to invited + `MANAGE_THREADS`; non-invitable option | Private ticket = private thread in a `#support` text channel |
| Channels cap | 500 channels / server, 50 per category | Never channel-per-ticket |
| Webhooks | Max 10 per channel; can set username + avatar per message; not usable in DMs | Teammate name/avatar in channels/threads (Intercom approach) |
| Interactions | Respond in 3 s (ack then follow up 15 min token); ephemeral replies; buttons, selects | "Solved" / "Ask human" / CSAT buttons; ephemeral private answers via `/ask` |
| Modals | 2025-26: text input, string/user/role/channel selects, Label, Text Display, **File Upload** (2025-10), **radio, checkbox groups** (2026-02) | Rich ticket intake form inside Discord (plan, email, repro, screenshot) |
| Components V2 | 2025-04: containers, sections, text display, media gallery, up to 40 components; `content`/`embeds` disabled with flag. 2026-10: component embeds in link previews | Answer cards with citation buttons |
| Context menu commands | Message context menu limit raised to 15 per type (2026-03-03) | "Ask dimmi" / "Send to dimmi" / "Mark as solution" on any message |
| User-installable apps | GA 2024-06; usable in DMs and servers where not installed; 5 follow-up limit | Users can `/ask product` anywhere; low priority |
| Linked roles | App connection metadata to grant roles (2022) | "Verified customer" / plan role from dimmi account link |
| DMs | Bot can DM only users sharing a guild who allow server DMs; 50007 otherwise. Anti-scam culture: "staff never DM you" [unverified] | DM-only = low discoverability, blocked often |
| Upload limit | 20 MiB default (2026-09-03) | Already in spec |
| Channel obfuscation | Bots see unviewable channels with hidden names; HTTP omits from 2026-11-16 | Channel picker must handle missing names |
| Search Guild Messages | Endpoint documented 2026-03-19; 202 + code 110000 while indexing; gated by MESSAGE_CONTENT [3P] | Backfill / knowledge import option |
| Server Web Pages + Google indexing | Pages for Discoverable servers from 2026-11-16; selected public channel messages indexed from mid-Dec 2026; admin per-channel under Server Settings > Web Presence; user opt-out; opt-in in EU/UK/CA/BR/AU/KR [3P, not from Discord primary] | Public help gets SEO for free on big servers; dimmi value shifts to curated answers + own help center |
| Content scanning flags | Attachments/embeds flags (explicit, gore, self-harm) 2026-10-28 | Use to drop flagged attachments before AI/inbox |
| AutoMod | Server keyword/spam filters run before bot sees message [general knowledge] | Respect; don't re-post blocked content |

## Best-in-class experience (synthesis)

### Community member
- Posts in `#help` forum, picks tag. Within seconds: AI reply **in the post's thread**, cited (doc links), short. Two buttons: **"This solved it"** and **"Ask a human"**. Optional 👍/👎.
- Solved: tag flips to Solved, title prefix ✅ optional, thread archived (not locked) after X h. Jump-to-solution message if human/helper answer marked.
- Human asked: tag "Needs team", role pinged (configurable, rate-limited), AI says expected response time. Community helpers can still answer; their answer counts.
- Account-specific / sensitive (billing, keys, logs): button **"Continue privately"** -> private thread with same user + support role (Fin, Mava pattern). Never paste secrets in public: AI detects secrets/PII in public posts and offers to move (kapa PII text; Fin sensitive move).
- `/support` or panel button in `#support` opens private ticket with modal intake (email, plan, description, screenshot upload).
- Mention `@dimmi` or right-click message -> Apps -> "Ask dimmi" anywhere: answer in thread off that message.
- DMs still work for people who want them.
- CSAT as one-tap buttons after resolve (Mava, Tickets), never a link to a web form.

### Teammate
- One inbox: forum posts, threads, private tickets, DMs alongside widget/email. Shows Discord context (server, channel, tags, OP roles, linked customer).
- Reply from inbox -> appears in Discord as teammate (name + avatar via webhook in channels; bold prefix in DMs). Or reply **natively in Discord**: synced back into inbox (Pylon, Thena, Plain webhooks). No double answers: AI stands down once a human (teammate or trusted helper role) replied.
- Status in inbox <-> tags in Discord both ways. Close in inbox = solved tag + archive. Mod marks solution in Discord = conversation resolved.
- Internal notes never posted. Escalation to private thread from inbox one click.
- Notifications in team's own Discord channel optional (Crisp-style) — spec 06 owns.

### AI agent
- Triggers per channel: forum auto-reply / mention / emoji / keyword / slash / context menu (Fin + kapa + Inkeep union).
- Wait-then-answer option (e.g. 2 min) in busy servers so helpers get first shot; "draft only" mode (Inkeep) for teams that want human-first.
- Answers only when grounded; otherwise says so and offers human (Quickchat). PII/secret guard. `allowed_mentions` none.
- Learns: solved threads (only answers by teammates or trusted roles, RunLLM filter) -> proposed knowledge snippets / help-center drafts, reviewed before use. 👎 -> review queue.
- Reads whole thread context, not just first message. Multilingual.
- Billing honesty: AI answer in public forum is a conversation like any other (dimmi pricing D5), no "assumed resolution" games.

### Admin setup
- Cloud: Add to Discord -> pick server -> pick channels + mode per channel (forum auto-answer / mention-only / emoji-ticket / private-ticket panel) -> post panel -> test. Under 3 min.
- Bot creates suggested tags (Open, Answered, Solved, Needs team) on chosen forum if missing (needs Manage Channels) or maps existing ones.
- Roles: map Discord roles to teammate / trusted helper / ping-on-escalation.
- Self-host: own bot (whitelabel name/avatar free, MESSAGE_CONTENT free under 10k users), manifest-like checklist of intents + permissions.
- Consent + public reuse toggle per channel if solved threads become public pages.

## Gaps vs dimmi spec 05

Spec 05 MVP: DMs to bot only; forum posts, thread-on-ingest, private `/support`, close tag/archive, MESSAGE_CONTENT, teammate echo, chat-app CSAT all "later". Evidence says reverse priority: DM is the least-used path in dev communities; forum/thread support is what every competitor ships.

| # | Gap | Who has it | Priority | Why |
|---|---|---|---|---|
| G1 | **Forum channel mode**: post = conversation (key `discord:t:{thread}`), AI auto-answer in post, teammate reply into post | Plain, Pylon, Thena, Unthread, kapa, Inkeep, Forum Assistant, Ferndesk (source) | **launch** | Dev-tool help lives in forums. DM-only means dimmi invisible in exactly the communities targeted. Needs MESSAGE_CONTENT: free for self-host own bot; cloud shared bot free under 10k users, apply for review now (90-day window, keep growing during review) |
| G2 | **Mention / context-menu / `/ask` in any text channel -> thread off message** | Fin (mention/emoji/keyword), kapa, Inkeep, Intercom right-click | **launch** | Works with **zero privileged intent** (mentions + interactions carry content). Cheap fallback for cloud bot above 10k users before approval |
| G3 | **Status signals**: forum tags (Open / Answered / Solved / Needs team) mapped to conversation state both ways; close = archive; "Mark solution" context menu | Answer Overflow, Next.js forum bot, Forum Assistant, Mava (archive on CSAT) | **launch** | Community and mods read state from tags. Without it, inbox and Discord disagree; mods keep separate bot. Spec `close_marker` exists for Slack only |
| G4 | **AI answer buttons**: "Solved" / "Ask a human" (pings mapped role) + 👍/👎 | Inkeep, Mava, Forum Assistant, CrawlChat | **launch** | Public AI answers without an exit feel spammy; explicit solved also gives honest resolution signal for dimmi analytics |
| G5 | **Teammate/helper echo**: native Discord replies by teammates and trusted-role helpers ingested; AI stands down after any human reply | Pylon, Thena (bidirectional), Plain (message events), RunLLM (trusted roles) | **launch for forum/thread mode** (DM mode can stay ignore) | In a community, humans answer in Discord, not in dimmi. Ignoring them = AI talks over helpers, inbox shows answered posts as unanswered. Spec X2 Echo already designed; pull forward for rooms |
| G6 | **Private ticket**: panel button + `/support` -> private thread in `#support` text channel, modal intake; "Continue privately" from public thread | Mava, Ticket Tool, Tickets, Fin (auto move sensitive), Intercom right-click, Quickchat | **soon** (DM covers privacy at launch) | Users expect panel button, not DM-a-bot. Private threads avoid DM privacy blocks (50007) and scam fear. Watch 1,000 active-thread cap (archive resolved after N h) |
| G7 | **Secret/PII guard in public**: detect keys/emails/logs in public post, offer move to private, AI refuses to echo | kapa (PII text), Fin (sensitive -> private) | **soon** | Vibe coders paste `.env` into help posts. Real risk, cheap differentiator |
| G8 | **Knowledge capture**: solved threads (teammate/trusted answers) -> knowledge source + help-center draft for review; "@dimmi learn" | Mava (Growth+), RunLLM, CrawlChat, Ferndesk, Answer Overflow (public pages) | **soon** | Turns Discord from cost into docs. No one does solved -> reviewed article -> cited next time, end to end. Fits dimmi self-updating help center story |
| G9 | **CSAT as Discord buttons** on resolve (spec: later chat-app-csat) | Mava (then archive), Tickets premium | **soon** | One-tap; link-out CSAT gets ignored |
| G10 | **Teammate identity in Discord**: name + avatar via channel webhook (threads support `thread_id`) instead of bold prefix | Intercom (webhooks), Plain ("Connect your profile") | **soon** | Looks like a person, not a bot. Mind 10-webhook-per-channel cap: reuse one dimmi webhook per parent channel |
| G11 | **Identity link Discord user -> customer/account**: "Link account" button (OAuth with signed state from widget/app), optional linked role "Customer"/plan | Pylon (channel->account), Plain lacks it (stated limitation) | **soon** | Needed for account-aware AI answers and private tickets; differentiator vs Plain |
| G12 | **Per-channel trigger config**: auto (forum), mention, emoji (🎫), keyword, draft-only, delay-before-AI | Fin, Pylon, Thena, Inkeep | **launch (minimal: auto / mention / off)**, rest soon | Communities differ; noisy channels need opt-in triggers |
| G13 | **Role mapping**: Discord roles -> teammate / trusted helper / escalation ping | Next.js bot, RunLLM, Inkeep | **launch** (spec has member links per user only) | Mods and helpers are roles, not individually linked users |
| G14 | **Public solved-thread pages / SEO** with consent | Answer Overflow, nextjs-forum.com; Discord Web Pages Dec 2026 | **later** | Discord now indexing natively for Discoverable servers [3P]; dimmi better spends effort on curated help center |
| G15 | **GitHub issue from thread** (bug -> issue, status back to thread) | Threadline, OSS bots | **later** (08 GitHub owns) | High value for dev tools; reuse dimmi GitHub integration |
| G16 | **Backfill / import history** of forum threads as knowledge (Search Guild Messages, archived threads) | RunLLM, Ferndesk (3,000 threads) | **later** | Cold start for AI on day one; spec says never backfill conversations, but knowledge import is different |
| G17 | **Verification + intent review ops** for cloud shared bot | Intercom blocked at threshold | **launch (process)** | Apply App Verification before 100 servers and intent review before 10k users, or cloud Discord stalls at launch |

Data-model note: spec `channel_rooms.kind` CHECK allows only `slack_channel`; forum/text rooms need `discord_forum`, `discord_text`, `discord_ticket_panel` kinds, per-room `mode`, tag map in `settings`. Thread key `discord:t:{thread_id}`. X5 (thread created by driver before ingest) already anticipated.

Spec scope challenge, one line: ship G1-G5 + G12-G13 + G17 at launch; keep DMs; move private tickets, CSAT, knowledge capture right after. DM-only launch would lose to free kapa-style bots and Mava's free tier for the exact audience dimmi courts.

## Sources

1. https://www.intercom.com/help/en/articles/14136689-connect-your-discord-channel
2. https://www.intercom.com/help/en/articles/11868746-native-discord-channel-alpha
3. https://www.intercom.com/changes/en/135844-connect-discord-to-your-intercom-inbox
4. https://fin.ai/updates/fin-over-discord
5. https://fin.ai/integrations/connectors/discord-channel
6. https://mava.app/pricing
7. https://www.mava.app/llm-info (vendor)
8. https://www.mava.app/mava-vs-ticket-tool [3P on Ticket Tool]
9. https://mava.gitbook.io/mava-docs/ticket-behavior-and-settings/discord-threads-and-limits
10. https://mava.gitbook.io/mava-docs/ai-support/private-support-ticket-ai
11. https://mava.gitbook.io/mava-docs/getting-started/integration-setup/discord-setup/discord-ticket-setup
12. https://www.getapp.com/emerging-technology-software/a/mava/ [3P]
13. https://docs.kapa.ai/integrations/discord-bot/
14. https://docs.kapa.ai/integrations/discord-bot/configuration
15. https://docs.kapa.ai/integrations/discord-bot/quickstart
16. https://docs.kapa.ai/api-feedback
17. https://www.kapa.ai/customer-stories/prisma (vendor)
18. https://inkeep.com/blog/slack-and-discord-copilots
19. https://docs.inkeep.com/integrations/discord
20. https://runllm.com/deployments
21. https://docs.runllm.com/data-sources/knowledge/discord/
22. https://github.com/AnswerOverflow/AnswerOverflow
23. https://docs.answeroverflow.com/channel-settings/mark-solution
24. https://docs.answeroverflow.com/channel-settings/indexing
25. https://www.answeroverflow.com/c/1043890932593987624/1084576896039260202 (Drizzle mirror)
26. https://docs.tickettool.xyz/general/faqs
27. https://top.gg/bot/1506999581722804274 [3P]
28. https://tickets.bot/premium
29. https://docs.tickets.bot/premium/perks
30. https://discordbotlist.com/bots/helper-gg [3P]
31. https://help.featurebase.app/articles/7019078-discord-integration
32. https://feedback.featurebase.app/p/discord-integration-for-support-module
33. https://feedback.featurebase.app/changelog/introducing-the-support-platform-dizzy
34. https://www.plain.com/changelog/the-first-discord-integration-for-b2b-support
35. https://www.plain.com/docs/product/channels/discord
36. https://docs.usepylon.com/pylon-docs/integrations/chat/discord
37. https://www.usepylon.com/blog/discord-customer-support-2026 [3P on others]
38. https://docs.thena.ai/guides/sources/discord
39. https://github.com/wgtechlabs/unthread-discord-bot
40. https://www.zendesk.com/company/press/zendesk-launches-discord-integration-enabling-seamless-customer-support-within-worlds-largest-gaming-community
41. https://github.com/discord-tickets/chatwoot-modmail
42. https://help.crisp.chat/en/article/how-to-connect-discord-with-crisp-1ercioa/
43. https://ferndesk.com/help/discord-integration-us1cg
44. https://discordbotlist.com/bots/forum-assistant [3P]
45. https://crawlchat.app/changelog/14-learn-by-reaction-discord-slack
46. https://crawlchat.app/changelog/12-discord-reaction-rating
47. https://docs.crawlchat.app/analysis/answer-correction
48. https://www.producthunt.com/products/ticketcord/launches/ticketcord
49. https://www.producthunt.com/p/threadline/threadline
50. https://github.com/EIRTeam/GitHub-Issues-Discord-Threads-Bot
51. https://quickchat.ai/post/discord-ai-support-ticket-bot
52. https://www.eesel.ai/blog/discord-ai-chatbot [3P, vendor-written]
53. https://github.com/joulev/nextjs-faq
54. https://github.com/rafaelalmeidatk/nextjs-forum
55. https://supabase.com/support
56. https://forum.cursor.com/t/discord-community/14619
57. https://forum.cursor.com/t/thank-you-for-having-an-actual-forum/1040
58. https://dev.to/bdbchgg/why-discord-sucks-for-developer-communities-2fg1 [3P]
59. https://docs.discord.com/developers/change-log
60. https://docs.discord.com/developers/gateway/getting-started-with-privileged-intent-review
61. https://docs.discord.com/developers/topics/threads
62. https://support-dev.discord.com/hc/articles/40281523410967 (blocked; via search summary)
63. https://docs.jda.wiki/net/dv8tion/jda/api/requests/GatewayIntent.html [3P, MESSAGE_CONTENT exemptions]
64. https://discord-api-types.dev/api/discord-api-types-v9/interface/RESTGetAPIGuildMessagesSearchQuery [3P]
65. https://www.mava.app/blog/verify-discord-bot [3P]
66. https://gadgetreview.com/discord-is-making-public-server-messages-searchable-on-google [3P]
67. https://www.saasrise.com/news/discord-to-index-discoverable-server-messages-on-google-starting-dec-2026-f53d836a-bed8-4c19-a17a-eda568ed6aa3 [3P]
68. https://www.galaxus.de/en/page/content-from-public-discord-servers-will-soon-appear-in-google-search-results-44238 [3P]
