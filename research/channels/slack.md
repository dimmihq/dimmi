# Slack support: how the best do it

Research date: 2026-11-08. Lens: what world-class support on Slack looks like for end customer, support team, AI agent, admin. Compare with dimmi plan `specs/05-chat-app-channels` (README + providers.md section 1).

Tags: [unverified] = single source or sources conflict. [3P] = third-party or competitor-written claim (many "reviews" here are written by rival vendors: Unthread, ClearFeed, eesel, Featurebase blogs).

## TL;DR

- Category split in two. **Slack-first B2B helpdesks** (Pylon, Plain, Thena, Unthread, ClearFeed, Atlas) treat Slack Connect channel = customer account, live in Slack, sell per seat $29 to $139. **Omnichannel helpdesks** (Intercom/Fin, Front, Featurebase, Productlane, Zendesk) bolt Slack on as one more channel, mostly 2025-2026 launches, often gated to mid/high plan.
- Table stakes in 2026, every serious player has all: configurable ingestion (every message / time or AI grouping / one per thread / emoji-only / mention-only), pleasantry filter ("thanks" never a ticket), teammate replies typed natively in Slack sync into helpdesk, reply from helpdesk posts in thread as teammate's own Slack identity, emoji shortcuts (✅ close, 👀 assign/investigating, 🎫 ticket), ✅ reaction on close, auto-reopen on customer reply, internal triage channel or internal "discussion" thread, CSAT in thread, SLAs, multi-workspace.
- AI agent on Slack converged on: answer in thread with citations, decide silently when NOT to answer (greetings, people talking to each other, customer still trying fix), stop the moment a human steps in or customer @mentions a teammate, hand off with internal note of what was tried, "Yes solved / No, get a human" buttons, emoji or loading indicator while working. Draft-only (suggest) mode in internal triage channel as alternative.
- Standout ideas worth stealing: Featurebase audience-aware knowledge (AI uses company-private content only in private/Slack Connect channel linked to that company; falls back to public if membership changes). Plain AI message grouping + "hand off when teammate tagged" toggle. Pylon AI follow-up detection (new top-level message within 24 h judged follow-up or new issue; sign-off detection keeps closed). ClearFeed 💬 reaction while AI responding, 🔒/📢 prefix to reply to customer from triage channel. Unthread in-channel ticket tracker (pinned message, "View support requests" modal, customer closes own ticket). Thena CSAT as ephemeral message to requester only.
- Slack platform 2025-2026 shifts that matter: (1) API ToS May 2025: no LLM training on Slack API data, no bulk export/persistent indexes [3P legal reading]; (2) non-Marketplace distributed apps throttled to 1 req/min on `conversations.history/replies`, internal custom apps exempt; (3) `chat.startStream/appendStream/stopStream` streaming (Oct 2025) + feedback buttons; (4) Work Objects unfurl + flexpane (Oct 2025); (5) Slack MCP server + Real-time Search API GA Feb 2026; (6) Slackbot itself now an AI agent (Jan 2026, Business+); (7) Lists API public Sept 2025.
- Complaints thin and mostly vendor-written: setup friction (admin rights, per-channel invite), noise tickets from chit-chat, Slack Connect limits (external mentions, multi-org channels break sync), AI answers exposing account data in multi-member channels (Intercom warns explicitly), price (seat minimums: Pylon 3, Unthread 5).
- Dimmi spec 05 biggest gaps vs table stakes: ignores teammate native replies (so AI and inbox both blind to humans answering in Slack), interactivity disabled (no buttons, modals, shortcuts, ephemeral prompts, CSAT), one ingestion mode only, inbound reactions ignored, no channel-to-company link or audience-aware AI, no triage/discussion thread. Streaming deferred is fine at launch if a working indicator exists.
- Dimmi angle no one has: free Slack channel on all plans (Featurebase, Productlane, Plain cap/gate it), self-host as customer's own internal Slack app (exempt from Marketplace throttle, data never leaves their infra, sidesteps ToS third-party worries), BYOK agent, Socket Mode no public URL, open source manifest. Community (non-Connect) workspace support matters more for vibe-coder audience than for Pylon's enterprise buyers.

## Per-competitor

### Pylon (usepylon.com)

Slack Connect category leader. 1,800+ B2B customers (own facts page): Linear, Cognition, Airtable, Together AI.

| Area | How |
|---|---|
| Message to issue | Every customer message in enabled channel becomes an **Issue** (always, powers account intelligence). Ticketing mode decides which are also **Tickets**: "Require mention or reaction" (default: @Pylon, 🎫 reaction, or customer types `:ticket:`), "Track all threads as tickets" (each top-level message), "Do not allow". Rules per account via conditions (e.g. Community accounts) |
| Grouping | AI decides if new top-level message continues channel's latest conversation or starts new issue. Always new if latest is ticket, closed over 24 h ago, or moved into thread. Thread replies join parent issue even if closed |
| Question detection prompt | Optional "Confirm if contacts are asking a support question": ephemeral prompt visible only to asker, max once per 15 min, "Help" button opens ticket |
| Who is teammate | Email on company domain or active Pylon user in workspace. Single-channel guests = customers. Other bots hidden unless allowlisted (by app ID or bot ID; "counts as customer" toggle) |
| Teammate replies | Reply in Slack natively or in Pylon. Teammate replies in customer threads appear as replies; in internal channels become internal notes. From Pylon: posts **as you** (connected Slack user token, needs channel membership) else as Pylon bot with your name + avatar, or as alias. Reactions follow same rule |
| Edits/deletes | Synced. Delete last message deletes issue |
| Status to customer | Optional in-thread confirmation ("ticket opened by X"), custom follow-up message, "Close Ticket" button (customer or team), becomes "Reopen Ticket"; ✅ reaction on close, removed on reopen. Community channels: ✅ on close |
| Reopen | Customer reply in same thread reopens to "On you", no time limit, unless AI reads reply as sign-off. Teammate reply never reopens. New top-level message within 24 h may be judged follow-up |
| Channel types | Customer (1:1 `#ext-acme`), Community (many orgs; every top-level message = ticket), Mirrored (Teams/Discord/chat mirrored into Slack to reply there), Internal threads (route to `#bugs`, engineers reply in Slack, synced), Notification (`#sla-breach`), Internal helpdesk |
| Triage | Triage channel = single source of all issues when team works in Slack. Or Pylon app with internal threads for other teams |
| Other Slack UX | Off-hours auto-message, structured data collection (priority), auto channel bookmarks, champion join/leave alerts, message shortcuts (View in Pylon, Sync internal thread, Sync DM), Pylon links unfurl only in internal channels, never customer channels |
| AI | Support Agent auto-resolves; Assist Agent investigates for human [3P Dupple]. **Slack Agent (Nov 2026) internal only**: @Pylon in internal channels/DMs, cites sources, can update issues, KB; explicitly disabled in Slack Connect, customers cannot trigger. Whether Support Agent answers inside Slack Connect channel: unclear [unverified] |
| Account mapping | Channel linked to account. Community vs customer account type |
| Limits | No Enterprise Grid org-wide install. Per-channel enablement, admin needed [3P] |
| Pricing | No free plan. Professional $89, Advanced $139 per seat per month, Enterprise custom (own facts page). Older 3P lists Starter $59, 3-seat minimum [3P, stale?]. AI on shared credit pool (min 5,000 credits/yr). Slack Connect, Community, internal Slack on all plans; Teams needs Advanced |
| Complaints | G2 dislikes generic: complexity, MS Teams wonky (opens non-tickets) [3P]. Weak for internal helpdesk [3P Unthread] |

### Plain (plain.com)

API-first B2B support. Slack strong, plus Teams, Discord, email, chat.

| Area | How |
|---|---|
| Channel types | Customer, Discussion (internal), Disabled. Auto-join rules on name prefix/suffix. Multiple Slack workspaces per Plain workspace (Connect workspace + community workspace) |
| Ingestion modes | **Time-based** (default: same user within 2 h, different users within 1 h = one thread). **AI-based** (recommended: semantic grouping). **One-to-one** (each Slack thread = Plain thread). **Manual emoji** (e.g. 👀; Plain users only by default, "Anyone" lets customers trigger; removing reaction keeps thread). **API-only** (`createThreadFromSlackMessage`, no welcome message, no ingestion emoji). After team/agent/workflow replies, next top-level message starts new thread even inside window |
| Teammate replies | From Plain: posts as agent's own Slack profile (permission at setup) |
| In-Slack actions | Notification message buttons: Reply in Slack, I'll take this, status dropdown (custom statuses), snooze until tomorrow, set urgent, open in Plain. Emoji on top-level customer message: ✅ = Done, 👀 = Investigating + assign me (per-channel toggle) |
| AI | **Customer Agent** (ex-Ari) customer-facing. Handoff triggers: asks for human, frustration, sales/billing/security/incident, no knowledge found or draft fails self-check (records knowledge gap), customer @mentions a teammate (toggle off for customers who tag habitually), nothing new to add, out of credits. Mechanics: short handoff message (optional), unassigns itself, never resumes, internal note listing search terms + docs read; incidents Urgent. One clarifying question when docs answer differently by detail. **Sidekick** internal agent: @mention in Discussion channels only, never Customer channels; tools Linear, Datadog, GitHub. **Bring your own agent** via API; third-party agents (Inkeep, Decimal, Duckie, Parahelp) |
| Internal | Slack Discussions: start internal Slack thread from Plain thread; messages, reactions, files sync back; resolving moves thread to "Close the loop". Slack digests (daily standup, daily summary incl. agent handoffs + top knowledge gap, themes) |
| Forms | Slack workflow forms create threads with fields pre-filled |
| CSAT, SLA | CSAT to Slack customers; same labels/SLAs/workflows as other channels |
| Limits | Foundation caps 25 enabled Customer channels; Horizon/Frontier unlimited |
| Pricing | Week 41 2026 change: per user annual, Foundation $45, Horizon $99, Frontier custom [unverified, single aggregator]. Earlier: Foundation $35 incl 1 seat, Horizon $299 incl 3 seats [3P] |

### Thena (thena.ai)

| Area | How |
|---|---|
| Setup | Org installs app, assigns Slack workspaces to Thena teams; each team own Slack config (one workspace can serve several teams) |
| Channel roles | Customer channels (messages become tickets), internal helpdesk channels, triage channels (updates only) |
| Creation strategies | Automatic (every new message), emoji reaction, @Thena mention, slash command opens form, require forms. Conversation grouping window merges quick bursts. Auto-create fails silently if default form has mandatory field (FAQ) |
| AI prompts | Detect ticket-worthy messages, generate title + description, sentiment, urgency |
| Routing | Map Slack user groups (`@support-engineers`) to Thena groups: tagging group assigns ticket |
| Triage | Rules (priority, sentiment, tags, account owner) post to one or many triage channels, separate internal thread each |
| Shortcuts | Message menu: "Assign to" creates ticket + assigns; "Inspect message" shows ticket id, status, assignee, form, link (debug "was this captured?") |
| Emoji actions | ✅ resolve, 🔄 reopen, custom mappings per team; logged on timeline |
| Replies | Slack authorization: reply from Thena appears as you in Slack, per team/workspace |
| CSAT | Rules (filters on ticket/account/contact, sampling 10/20/100%, cooldown, delay in days, 5-star or thumbs). Slack delivery = **ephemeral message in ticket thread mentioning requester** |
| Pricing | Starter $29/user/mo (max 5 seats, 1,000 tickets/mo, AI ticket detection), Standard $79 (AI auto-responses, agent studio, MCP), Enterprise $119 (Teams, custom agents). Annual only [3P] |

### Unthread (unthread.io)

YC. Slack-native helpdesk, now leans IT/HR/ITSM plus customer support.

| Area | How |
|---|---|
| Tracking | Automatic for messages from users outside workspace in external channels, skips pleasantries ("Thanks", "Hello"). Customers never see app at creation. Role overrides per channel (teammate list, customer list) |
| Grouping | Top-level messages grouped by same-user interval (default 5 min). Threads always own conversation. Threading off a top-level message splits it out. Merge and split conversations |
| Intake options | Auto-create ticket; show ticket-type form in thread (AI pre-fills type, "Never mind" cancels); **Engage AI agent** (AI converses, max N follow-ups e.g. 2, then "Create ticket" button or auto handoff; AI extracts required custom fields) |
| Bot DM / private tickets | "Start a conversation" button in channel opens modal, continues in DM with bot; private to requester + assignees. App Home shows request history. "Nevermind" = deflected, thread closed to bot |
| In-channel ticket tracker | Pinned message in customer channel: open count, "Create ticket" button, "View support requests" modal listing open/closed with deep links; customer can close own ticket |
| Triage | Global triage channel cross-posts each conversation; triaged message live-updates status, assignee, notes, tags, linked tasks, deep link. Per-customer internal channel (`ext-acme` + `ext-acme-internal`). Temporary triage channels auto-archive on close |
| In-Slack management | "Manage conversation" modal: triage, merge, snooze (DM reminder), assignee, priority, tags (AI auto-tag), notes |
| CSAT/NPS | In channel, inline buttons or button+modal, emoji or numeric; post-submit ephemeral thanks; auto-send on close via automation |
| Other | Broadcasts to many customer channels, emoji-triggered automations [3P review], white-label bot per project, WebMCP |
| Pricing | Pro from $85/agent/mo, 5-seat minimum (so $425+/mo), conversation limits apply, 14-day trial. Business/Enterprise quote. Older $50/$75 tiers gone [3P Desk365] |

### ClearFeed (clearfeed.ai)

$2.7M raised (PeakXV), 300+ customers, G2 4.6 (165 reviews) (own llms.txt).

| Area | How |
|---|---|
| Model | Request (internal, no customer-visible id) to Ticket (customer-visible id CF-1234 + status) to Task (Jira/Linear/etc). Collections = groups of channels sharing settings |
| User types | Responders (own workspace) vs non-responders (Slack Connect users, guests). External Helpdesk: only non-responder messages create requests. 📌 reaction on first message forces request from any message |
| AI filtering | Greetings, acknowledgments, appreciation, "this works now" never create requests; replies to filtered message can. Messages merged per user burst. Auto-priority High on "prod down", "urgent" |
| Emoji rules | 👀 assign self, ✅ solved, 🚨 urgent (all users or responders only), 💬 auto-added while Virtual Agent responding, removed on handoff/solve. Custom emoji ok. Not in triage channels |
| Triage channel | Each request posted as thread. Reply from triage syncs to customer channel. Visibility default Public: prefix 🔒 keeps internal; or Private default and 📢 prefix sends to customer. Link previews and forwards synced both ways. Multi-workspace: triage stays in primary workspace |
| AI modes per collection | **Virtual Agent** (public replies in request channel, trigger automatic or on emoji; before ticket: "Yes, mark as solved" / "No, create a ticket" buttons, auto-ticket if no answer or no feedback in 1 h; after ticket). **Interactive Agent Mode** (multi-turn, clarifying questions, tool calls Jira/Zendesk; stops when human responder steps in; auto-escalate after 1 h customer silence). **Agent Assistant** (private drafts in triage channel, first message only). Reads images in Slack. Doc Updater agent finds doc gaps. BYOM: any OpenAI-compatible provider incl Anthropic (via support, not UI) |
| CSAT | Slack-native on solved, all requests or tickets only, 5-emoji or thumbs, delay, comment prompt on low score, only non-responders can answer |
| Pricing | Starter $49/mo (own page) vs $24/agent or $40/mo for 10 channels + 100 tickets [3P conflicting]. AI Pack $20/mo/account on Starter, included Pro/Enterprise. 14-day trial. Multiple Slack workspaces = separate sub-accounts. Integrations Edition not for Slack Connect |

### Intercom / Fin (native Slack channel)

| Area | How |
|---|---|
| Status | Beta 2025-09-11; replaced deprecated "Intercom app for Slack". Available on all helpdesk plans |
| Channels | Conversational (creates synced conversations) or notification-only, never both. Slack Connect + community workspaces + private channels (`@fin` to add) |
| Message to conversation | Default every message. Restrict triggers to mention of user/bot, specific emoji (unicode only), keyword. Allow specific bots. Pre-connect threads never ingested |
| Contacts | New lead per sender, or match email to existing user. No channel-to-company mapping documented |
| Replies | Inbox replies post to same thread with teammate name + avatar; connect own Slack account to post as self (needs workspace data permission). Other apps' bot replies not synced by default |
| Fin | Responds to all messages by default (restrict via triggers). Generic loading messages while preparing, cleared on final send. Resolution billed same as other channels ($0.99 per outcome, see `competitors/fin.md`) |
| DMs | Teammates can start 1:1 DM from Inbox; customers cannot start DMs ("New chat" errors) |
| Tickets | Created from Inbox only; ticket status changes posted to Slack thread |
| Limits | Cannot react to Slack messages from Inbox. Workflow reply buttons not clickable (free text only). Custom emoji no. No translation, no attribute collectors. **Public channels expose shared custom attributes and customer email in reply-time messages to all members**; data connectors off by default for privacy. Multi-workspace mode: channels claimed first-come, DMs off, one bot auth shared. Primary Enterprise Grid workspace unsupported. Slack free plan 10-app cap noted |

### Front

- Reimagined Slack integration 2025-09-30. Grouping mode merges same-sender burst into one conversation. Private Slack Connect channels supported (older help article says public only [conflicting]). Replies and reactions from Front appear as you, not bot. Smart QA + CSAT on Slack conversations. Old inboxes not migrated.
- No AI agent in Slack described. Plan gating not stated.

### Zendesk

- Native "Slack for Zendesk Support": @zendesk in Slack Connect channel creates ticket; external users cannot set assignee [3P eesel].
- Side conversations in Slack (internal, from ticket): Professional+ plus Collaboration add-on, off by default. Only comments up to 1,000 chars insertable, multi-workspace shared channel replies never return, no reactions/DMs/typing, lag 5 to 30 s, channel must be owned by own workspace.
- In practice Zendesk shops buy ClearFeed/Unthread bridge [3P, rival vendors say so].

### Featurebase

- Slack Support launched 2026-08-10, Professional ($59/seat) and Enterprise only.
- Channel types: Customer (one channel, one Company link), Community (many orgs, each qualifying top-level message = conversation, no Company link), Discussion (internal, only discussions started from Inbox sync), Disabled. Auto-join rules (starts/ends/contains, can enable Fibi on join), "scan existing channels".
- Ingestion: AI topic grouping, sender + timing, one per top-level message, emoji-only.
- Sync both ways: replies, attachments, edits, deletions, reactions, custom emoji, recognized @mentions. Notes, assignment, tags, priority, SLA never mirrored to Slack. Inbox reactions need personal Slack connect.
- **Fibi**: answers support questions, problem reports, help requests. Silent when message aimed at teammate, people coordinating, customer still trying suggestion, closing ack, greeting, chit-chat; no typing shown; team sees reason in Inbox; silence does not close.
- **Audience-aware content** (best idea in category): Community or unlinked channel = public content only. Company-linked private or Slack Connect channel = public + that company's content. Company-linked public channel = public only. Membership change on linked Slack Connect channel = fall back to public until team confirms audience; queued Fibi replies blocked. Custom actions per tool: Disabled in Slack (default), Public-safe, Company-safe.
- Workflows in Slack: buttons, collect data, expected reply time, rating request, routing. Broadcasts max 500 channels per send. Reports split by channel incl Fibi vs teammate handling.

### Productlane

(Detail in `competitors/productlane.md`.) Slack inbox needs Pro ($59/seat). Bot invited per Slack Connect channel. Grouping: off/manual, one per message, 30 min (default) / 1 h / 24 h window; optional tag requirement. Teammate messages excluded unless pushed. Replies both ways; reactions sync both ways (`reactions:write`). Linear issue done: ✅ on customer message, optional close-loop reply. Office-hours thread auto-replies, welcome messages on join with `{name}` variables, important threads posted once to internal channel, split grouped messages, link previews internal only. Agent answers in Slack only if enabled under Deploy.

### Atlas (atlas.so)

- YC. Slack Connect: auto-ticket from monitored channels or 🎫 reaction by agents. Two modes: triage-only (requests forwarded to triage channel, manage priority/assignee in Slack) or triage + support. Monitor all / Slack Connect / internal / chosen channels. Agents link Slack accounts. AI reply suggestions shown in Slack for agent review. **One Slack workspace only.** Help center dated 2025.

### Notable 2025-2026 movement

- Slackbot rebuilt as AI agent (GA 2026-01-13, Business+ / Enterprise+): Slack itself competes for "ask the workspace" internal use. Support vendors respond by moving agents into internal Slack (Pylon Slack Agent Nov 2026, Plain Sidekick in Slack).
- Hivinq (Product Hunt 2026-01-13): Slack copilot drafting answers teammates correct [unverified, thin].
- Third-party agent layering: Plain lets Inkeep, Decimal, Duckie, Parahelp act as agent; ClearFeed BYOM. Model-agnostic is now expected, not a differentiator. BYOK at zero markup still rare.

## Slack platform facts 2026 that matter

| Fact | Detail | Impact for dimmi |
|---|---|---|
| API ToS update | Effective 2025-05-29 new apps, 2025-06-30 existing. Prohibits bulk export, "persistent copies, archives, indexes", and use of API data to train LLMs [3P Hunton legal reading]. Developer policy (Dec 2024) bans LLM training "under any circumstances", incl unlisted apps | Never train on Slack data (BYOK, we do not train anyway). Storing conversation messages as helpdesk record is what every vendor does, but feeding Slack transcripts into self-updating help center / vector index on cloud is gray zone [unverified]. Self-host custom app = customer's internal app on own infra: cleanest story |
| Non-Marketplace throttle | Commercially distributed apps not in Marketplace: `conversations.history` + `conversations.replies` Tier 1 (1 req/min), `limit` max 15. New apps from 2025-05-29; existing installs unaffected per Slack page (one 3P says hit from 2025-09-02 [conflicting]). Internal customer-built apps exempt (Tier 3, limit 1,000) | Spec already handles (Marketplace at beta, backfill only). Self-host custom app exempt: a real selling point |
| Streaming | `chat.startStream` / `appendStream` / `stopStream` (2025-10-07). Stream max 10 min. Takes `channel`, `thread_ts`, `recipient_team_id`, `recipient_user_id`. Whether allowed in ordinary channel threads (vs agent container) not stated in changelog [unverified]. `feedback_buttons` + `context_actions` + `icon_button` blocks attachable at stop | Post-launch: stream Dimmi Agent replies in Slack Connect thread; verify channel-thread support first |
| Agent surface | Agents sidebar / split view, `assistant:write`, suggested prompts (up to 4), `agents.sessions.setStatus` processing indicator (stop button, 1 h timeout), `app_context_changed` tells what user views. Some features paid plan only | Relevant for internal teammate assistant ("ask dimmi about this customer") and customer DMs, not Slack Connect channels |
| Typing | No bot typing API in channels. Workarounds used by vendors: reaction while working (ClearFeed 💬), placeholder message then `chat.update` (Intercom loading messages) | Spec says "Typing: no". Use reaction or placeholder |
| Work Objects | Oct 2025: rich unfurl + flexpane (right panel) via `chat.unfurl` metadata + `entity.presentDetails`; embeds in sandboxed iframe; search/AI citations need `entity_details_requested` and new Marketplace review | Dimmi conversation links unfurl as live object in internal channels (status, assignee, actions). Later |
| MCP + Real-time Search | GA 2026-02-17. Replaced Data Access API. Query Slack without storing data; granular `search:read.*` scopes with user consent | Dimmi Agent could search customer's internal Slack for answers at query time (ToS-compliant path) instead of indexing it. Later |
| Lists API | Public 2025-09-02 (`slackLists.items.create` etc.), paid plans, `lists:write` | Possible "support queue as Slack List" view for solo dev. Later, low value |
| Slack Connect app limits | Apps live in host org; external users see bot messages; external user mentions/profiles limited [3P Glue, Tightknit]; Zendesk side conversations break in channels owned by other org; workflows by external people limited by admin policy | Fan-out across both orgs already in spec. Expect admin policy errors (`restricted_action`) |
| Free plan | Slack free plan: max 10 apps (Intercom note). Slack Connect needs paid plan on host side, customers join free [3P] | Vibe-coder audience often free plan, community workspace not Connect. Community mode quality matters |
| Interactivity | Buttons, modals, ephemeral messages, message shortcuts need `interactivity.is_enabled` + request URL (Socket Mode delivers interactions too) | Spec manifest has `interactivity: false`. Blocks most table-stakes UX |

## Best-in-class experience (synthesis)

### End customer

1. Posts in shared channel or community channel like talking to a colleague. No form, no ticket syntax. Threads or top-level both fine.
2. Within seconds: subtle signal it was seen (👀 or 💬 reaction, or AI reply streaming in thread). Never a "Your ticket #123 was created" spam message by default.
3. AI answer in thread: short, cited, scoped to what this channel's audience may see. Two buttons: "Solved, thanks" / "Talk to a human". Asks one clarifying question max.
4. AI stays silent on "thanks", "hi", people talking among themselves, messages that @mention a named teammate.
5. Human takes over in same thread, as a real person (own Slack identity, not "Support Bot"). AI never talks again after handoff.
6. Off hours: one line with expected reply time.
7. Close: ✅ on root message; reply in thread reopens; optional "Close" / "Reopen" button customer can press.
8. CSAT: one-tap emoji or thumbs, ephemeral to requester only, delayed, cooldown.
9. Bonus: pinned tracker in channel with open requests; DM the bot for private issue.

### Teammate

1. Works where they like: helpdesk inbox or natively in Slack thread. Both stay in sync (replies, edits, deletes, reactions, files).
2. Reply from inbox shows as own Slack user (connected account) else bot with name + avatar.
3. Emoji actions in Slack: ✅ close, 👀 assign me, 🚨 urgent, 🎫/📌 force-track.
4. Message shortcut on any message: "Create conversation", "View in dimmi" (and "was this captured?" inspect).
5. Triage channel or per-customer internal channel: each conversation posted once, live-updating status/assignee, reply from triage with 🔒/📢 prefix, internal discussion thread syncs back as notes.
6. Notifications with buttons: take it, status, snooze, urgent. Daily digest.
7. Channel = account: linked company shows plan, MRR, open conversations, SLA. SLAs per account tier.
8. Merge/split conversations when grouping guesses wrong.

### AI agent

1. Decide answer / stay silent / hand off per message, record reason in inbox.
2. Audience-aware context: public docs in community channels; account data only in private/Connect channel linked to that account; re-check when channel membership changes. Tools tagged public-safe / account-safe.
3. Handoff triggers: asks human, frustration, billing/security/incident, no knowledge, teammate tagged, human already replied. Handoff note lists searches + docs read. Knowledge gap logged.
4. Shows working state (reaction or placeholder or stream). Streams long answers.
5. Suggest-only mode: drafts in triage channel for review.
6. Internal assistant in Slack for teammates (@dimmi about this customer), never in customer channels.

### Admin setup

1. "Add to Slack" OAuth or self-host manifest. Auto-join rules by name prefix (`#ext-`, `#help-`), scan existing channels.
2. Per channel: type (customer / community / internal discussion / disabled), linked company, ingestion mode, agent on/off, tracker on/off.
3. Teammate detection automatic (own workspace members, domain email) with overrides; bot allowlist.
4. Health: missing scopes, not-in-channel, uninstall, clear fix links (spec already strong here).

## Gaps vs dimmi spec 05

Priority: **launch** = Slack channel feels broken or unsafe without it; **soon** = first few weeks after launch, table stakes for B2B buyers; **later** = differentiator or niche.

| # | Gap | Spec today | Competitors | Priority | Why |
|---|---|---|---|---|---|
| 1 | **Teammate native replies in Slack thread** recorded in conversation (echo) | Ignored, `later: teammate-echo` | Pylon, Plain, Featurebase, Productlane, Front, ClearFeed all sync | **launch** | Without it inbox shows conversation unanswered after human answered in Slack, SLA breaches fire, Dimmi Agent may reply after a human already did, history missing for agent context. Solo devs will answer from Slack mobile constantly |
| 2 | **Human-in-thread stops agent** (handoff when teammate replies or is @mentioned) | Not specified | Plain (tag toggle), ClearFeed (interactive mode stops when responder steps in), Fibi silent when aimed at teammate | **launch** | Bot talking over a human in front of a paying customer is the worst failure in shared channels. Needs #1 detection even if echo storage slips |
| 3 | **Interactivity on** (buttons, ephemeral, modals, shortcuts) | Manifest `interactivity: false` | Everyone | **launch** (enable + "Solved / Talk to human" buttons + "Create conversation from message" shortcut); rest soon | Enabling later forces every self-host user to edit their app and re-approve scopes. Ship manifest with it now even if few features use it |
| 4 | **Ingestion modes** beyond per-author 120 s window: every message, one per thread, emoji-only, mention-only; pleasantry filter | One mode (author window) | Plain 5 modes, Featurebase 4, Pylon/Thena/Intercom triggers, ClearFeed/Unthread pleasantry filter | **launch**: pleasantry filter + emoji-only + one-per-message. **soon**: AI grouping, merge/split | "thanks!" creating billable conversation hurts a per-conversation price model directly. Busy community channels need emoji-only |
| 5 | **Agent silence policy** (answer / stay silent / hand off, reason logged) | Agent answers every conversation (07) | Fibi silence rules, ClearFeed AI filter, Unthread pleasantries | **launch** | In channels many messages are not questions. Ties to 07 |
| 6 | **Audience-aware agent context + channel-to-company link** | Contact identity only; no room company link | Featurebase (best), Intercom warns of attribute leaks, Pylon account per channel | **launch** (minimum: agent uses public knowledge + no account-data tools in rooms; linked company room unlocks). **soon**: full room `company_id`, membership-change fallback | Privacy bug class: agent answering from contact A's account data in channel where B, C (other org in community) read. Cheap to guard now, expensive after an incident |
| 7 | **Working indicator** for agent | None (typing capability false) | ClearFeed 💬 reaction, Intercom loading message, Plain typing (widget) | **launch** | 5 to 20 s silence then wall of text feels dead. Reaction add/remove costs two calls |
| 8 | **Inbound reaction emoji actions** (✅ close, 👀 assign, 📌 track) | Inbound reactions ignored (needs 03 reaction model) | Plain, Thena, ClearFeed, Pylon, Productlane | **soon** | Action mapping does not need full reaction timeline model; only `reaction_added` on root by teammate. Needs `reactions:read` scope in manifest now (#3 logic) |
| 9 | **Edits/deletes sync** | Dropped, `later: message-edit` | Pylon, Featurebase sync both ways | **soon** | Customer edits typo or deletes secret they pasted (API key): inbox keeps it. Delete inbound is a security nice-to-have |
| 10 | **Reply as own Slack identity** (user token connect) | Bot with `chat:write.customize` name + avatar | Pylon, Thena, Front, Plain, Intercom, Featurebase | **soon** | B2B customers notice "APP" badge; Slack Connect buyers sell on "talk to real engineers" |
| 11 | **Triage / internal discussion thread** (post conversation once to internal channel, live status, reply with prefix, notes sync) | 06 notifications only; reply from notifications later | Pylon, Plain, Thena, Unthread, ClearFeed, Featurebase, Atlas | **soon** | Solo dev lives in Slack; replying from internal channel without opening dimmi is the Slack-native promise |
| 12 | **CSAT in Slack** (ephemeral, one-tap) | `later: chat-app-csat` | Thena, ClearFeed, Unthread, Plain, Front, Featurebase | **soon** | Every competitor reports Slack CSAT; needs #3 |
| 13 | **Close/reopen visible to customer** beyond ✅: optional confirmation line, customer Close/Reopen button, sign-off detection keeps closed | ✅ marker only | Pylon, Unthread tracker | **soon** | ✅ parity covers basics; button + AI sign-off avoid reopen churn from "thanks!" |
| 14 | **Auto-join rules + scan existing channels** | Room created when bot invited | Plain, Featurebase | **soon** | Accounts with 50+ `#ext-` channels; invite-each is top setup complaint [3P] |
| 15 | **Office-hours / expected reply time message** | None | Pylon, Productlane, Featurebase, Unthread | **soon** | Depends on 03 business hours; one templated line |
| 16 | **Streaming agent replies** | Deferred | Slack API since Oct 2025; few vendors document use | **later** (verify channel-thread support) | Indicator (#7) covers most of perceived latency |
| 17 | **Broadcasts to many customer channels** | None | Unthread, ClearFeed, Featurebase (500), Intercom, Thena, Productlane | **later** | Release notes and incident notices for B2B; fits 06/outbound not 05 |
| 18 | **In-channel tracker / App Home history / DM private ticket** | DMs supported, no tracker | Unthread | **later** | Nice for multi-requester channels |
| 19 | **Forms / ticket types in modal** | None | Thena, Unthread, Plain workflow forms | **later** | Enterprise intake pattern, low value for indie audience |
| 20 | **Work Objects unfurl** of dimmi links in internal channels only | None | Pylon/Productlane internal-only previews (classic unfurl) | **later** | Never unfurl in customer channels (leaks internal notes) |
| 21 | **Internal Slack assistant** (@dimmi about customer X) | None | Pylon Slack Agent, Plain Sidekick, Slackbot itself | **later** | Slackbot now covers generic "ask"; differentiate via MCP tools later |
| 22 | **Enterprise Grid org install** | Rejected | Pylon and Intercom also unsupported | later | Parity fine |

Cross-cutting notes:
- Billing: decide whether pleasantry-only and AI-silent conversations are billable. Fin bills "assumed resolution" on silence; dimmi should never bill a "thanks" message. Strong marketing line.
- Scopes to add to manifest now (avoid re-approval later): `reactions:read`, `app_mentions:read`, `chat:write.public`? (only if posting to unjoined triage), `commands`, `links:read`/`links:write` (later unfurl), `users:read.email` already. Event subs: `reaction_added`, `reaction_removed`, `app_mention`, `message_changed`/`message_deleted` already arrive with `message.*`.
- Self-host wins to market: internal custom app is exempt from Marketplace throttle and sits under customer's own Slack app policy; data stays on their box; BYOK model. Nobody else offers this for Slack Connect support.

## Sources

Pylon
- https://docs.usepylon.com/pylon-docs/integrations/chat/slack.md
- https://docs.usepylon.com/pylon-docs/integrations/chat/slack/support-ticket-flow
- https://docs.usepylon.com/pylon-docs/integrations/chat/slack/slack-community
- https://kb.help.usepylon.com/articles/8713223951-best-practices-slack-and-pylon
- https://manifest.help.usepylon.com/articles/7631479619-getting-support-via-slack
- https://www.usepylon.com/integrations/slack
- https://www.usepylon.com/ai-agents/slack
- https://www.usepylon.com/blog/tagging-ai-in-slack-for-support-before-a-ticket-exists
- https://www.usepylon.com/pylon-facts
- https://dupple.com/reviews/pylon [3P]
- https://unthread.io/blog/pylon-review/ [3P]
- https://www.g2.com/products/pylon-pylon/reviews_and_filters [3P]

Plain
- https://www.plain.com/docs/product/channels/slack.md
- https://www.plain.com/docs/product/channels/slack-ingestion-modes.md
- https://www.plain.com/docs/product/channels/slack-in-slack-actions.md
- https://www.plain.com/docs/product/channels/slack-discussions.md
- https://www.plain.com/docs/product/channels/slack-digests.md
- https://www.plain.com/docs/product/agents/ari/handoffs.md
- https://www.plain.com/docs/product/agents/bring-your-own-agent.md
- https://changelog.plain.com/changelog/multiple-slack-workspaces-and-ai-message-grouping
- https://pricingsaas.com/companies/plain/diffs/2026W41 [3P]

Thena
- https://docs.thena.ai/guides/sources/slack.md
- https://docs.thena.ai/guides/ticketing/emoji-actions.md
- https://docs.thena.ai/guides/ticketing/csat.md
- https://docs.thena.ai/guides/preferences/slack-authorization.md
- https://thena.ai/pricing

Unthread
- https://docs.unthread.io/docs/how-conversations-are-tracked
- https://docs.unthread.io/docs/80b9e061-cf17-4db0-aa73-55ce152a0f2f
- https://docs.unthread.io/docs/bot-for-slack-private-tickets
- https://docs.unthread.io/docs/in-channel-ticket-tracker
- https://docs.unthread.io/docs/triaging-with-your-team
- https://docs.unthread.io/docs/editing-conversations-in-slack
- https://docs.unthread.io/docs/csat-nps-surveys
- https://unthread.io/pricing
- https://www.desk365.io/?p=77672 [3P]

ClearFeed
- https://docs.clearfeed.ai/clearfeed-help-center/how-it-works/creating-requests/from-slack-channels.md
- https://docs.clearfeed.ai/clearfeed-help-center/how-it-works/request-tickets-tasks.md
- https://docs.clearfeed.ai/clearfeed-help-center/clearfeed-helpdesk/emoji-rules.md
- https://docs.clearfeed.ai/clearfeed-help-center/managing-requests/triage-channel.md
- https://docs.clearfeed.ai/clearfeed-help-center/clearfeed-helpdesk/csat-survey.md
- https://docs.clearfeed.ai/clearfeed-help-center/clearfeed-ai/ai-agents.md
- https://docs.clearfeed.ai/clearfeed-help-center/clearfeed-ai/ai-agents/use-ai-agents.md
- https://docs.clearfeed.ai/clearfeed-help-center/clearfeed-ai/ai-agents/bring-your-own-model.md
- https://clearfeed.ai/llms.txt
- https://clearfeed.ai/pricing/customer-slack-channels
- https://unthread.io/blog/clearfeed-pricing/ [3P]

Intercom / Fin
- https://www.intercom.com/help/en/articles/11534476-connect-your-slack-channel
- https://www.intercom.com/changes/en/107019-enable-fin-to-support-your-customers-in-slack

Front
- https://community.front.com/product-updates/front-s-reimagined-slack-integration-tame-the-chaos-of-slack-support-3089
- https://front.com/integrations/slack
- https://help.front.com/en/articles/2053

Zendesk
- https://support.zendesk.com/hc/en-us/articles/4408844202778
- https://www.eesel.ai/blog/zendesk-slack-integration [3P]

Featurebase
- https://feedback.featurebase.app/changelog/slack-support-one-inbox-for-every-customer-conversation
- https://help.featurebase.app/articles/5536051-slack-support-overview
- https://help.featurebase.app/articles/9984940-add-and-manage-slack-channels
- https://help.featurebase.app/articles/3024324-deploy-fibi-ai-agent-in-slack

Productlane
- https://productlane.com/docs/channels/slack-inbox

Atlas
- https://atlas.so/integrations/slack-connect
- https://help.atlas.so/articles/506572-slack-integration

Slack platform
- https://docs.slack.dev/changelog/2025/05/29/rate-limit-changes-for-non-marketplace-apps
- https://api.slack.com/changelog/2025-05-terms-rate-limit-update-and-faq
- https://docs.slack.dev/developer-policy
- https://docs.slack.dev/changelog/2024/12/10/dev-policy-update
- https://www.hunton.com/insights/legal/salesforce-locks-down-slack-data-time-to-review-your-slack-api-terms [3P]
- https://docs.slack.dev/changelog/2025/10/7/chat-streaming
- https://docs.slack.dev/ai/developing-ai-apps
- https://docs.slack.dev/messaging/work-objects-overview
- https://docs.slack.dev/changelog/2025/10/22/work-objects
- https://docs.slack.dev/changelog/2026/02/17/slack-mcp
- https://docs.slack.dev/changelog/2025/09/02/list-api
- https://techcrunch.com/2026/01/13/slackbot-is-an-ai-agent-now
- https://slack.com/help/articles/1500012572621-Permissions-in-Slack-Connect-channels-
- https://docs.glue.ai/for-admins/slack-connect [3P]
- https://docs.tightknit.ai/slack/slack-connect [3P]
