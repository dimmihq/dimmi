# Dimmi roadmap: launch, post-MVP, standout

Date: 2026-11-01. Scope trim 2026-11-02 (`scope-audit-4-competitors.md`, PO): audit cut and borderline items deferred to "Later" section; self-updating docs, importers, agent actions moved into launch (16-18). Synthesis of `competitors/fin.md`, `competitors/helpdesk.md`, `competitors/featurebase.md`, `competitors/productlane.md`, `tech-stack.md`, `pricing.md`, `landscape-2026-11.md`. Style: caveman.

Target users: vibe coders, solo devs, super early adopters. Build with Claude Code, Cursor, Lovable, Bolt. Ship SaaS and apps fast. Hate seats, hate meters, hate setup. Live in editor, Slack, Discord, phone.

Rule: fewer things at launch, each world class. Cut scope, never quality.

## Why now

- Salesforce buying Fin (ex-Intercom) for about $3.6B. Close status disputed: `fin.md` sources say closed 2026-09-10, `pricing.md` sources say pending until early 2027. Either way, renewal-time churn window open.
- Fin bills $0.99 per outcome. Since 2026-03-12 customer silence for 24 hours count as billable outcome. Productlane same trick at $0.79. Featurebase raised AI price $0.29 to $0.49 on 2026-07-01, hit existing customers too. Text (HelpDesk) $0.99 per AI resolution plus billed API calls with no cap.
- Every competitor charge per seat. Every competitor charge AI tax. None self-hostable except weak open-source (Chatwoot 4GB+ RAM, paywalled enterprise folder).
- Fin already target our audience: Fin CLI (April 2026) built for Claude Code and Cursor, "live in under 2 minutes" demo. But US workspaces only, tokens expire about 14 days. Must beat this on day one.

## Launch must-haves (MVP)

Only items without which first users will not switch or will churn week one. Each: why, then world-class bar.

### 1. Messenger widget

- Why: first thing user installs. Every conversation start here.
- Bar:
  - Install by one script tag or `npm i`. Packages: browser, React, Next.js. Other stacks copy-paste snippets (later: identity-helper-packages). Automatic SPA route tracking.
  - Loader 3KB or less, widget core 35KB or less gzipped, panel lazy-loaded. Preact in Shadow DOM. Size published, CI size budget.
  - Live chat, attachments, dark mode, mobile-responsive, page context API.
  - Anonymous visitor plus email capture when nobody online. Offline reply reach visitor by email.
  - Loud console error on wrong key, wrong domain, bad token. Never fail silent.
  - No Dimmi branding lock behind paid tier on self-host.
  - First message in inbox under 5 minutes from signup or `docker run`.

### 2. Secure identity by default

- Why: vibe coders ship insecure by default. Intercom leaked data on shared devices. SaaS dev need know which user wrote.
- Bar:
  - JWT (HS256) identity verification on by default. Intercom deprecated HMAC in Aug 2026, so JWT is primary.
  - `@dimmi/server` helper (Node/Next.js). Copy-paste snippets for Python, Go, Rails, Laravel.
  - User ID, email, plan, custom attributes, organization claims. Visible in inbox sidebar.
  - `dimmi.logout()` clear everything.
  - Bad token: stable error code in plain words in console and API (validations endpoint later: identity-validations).

### 3. Shared inbox

- Why: core job. HelpDesk users complain search cannot find old tickets.
- Bar:
  - Real-time. Sub-100ms navigation. Optimistic updates.
  - Assign, open, snoozed, closed, unlimited tags, internal notes, saved replies, collision indicator.
  - Cmd+K command palette. Single-key shortcuts.
  - Full-text search with filters that finds old conversations (Postgres FTS + trigram).
  - Context sidebar: user attributes, page URL, browser, past conversations.
  - Installable PWA, good on phone.
  - Unlimited teammates, roles owner, admin, member.

### 4. Email channel both ways

- Why: support@ still main channel. Visitor leave page before reply; reply must reach them.
- Bar:
  - Forwarding address working in 2 minutes. Inbound through SES receiving (cloud forwarding address; self-host SES receipt rules on own AWS). IMAP and other inbound adapters later (imap-adapter, extra-mail-adapters).
  - Reply from own domain with guided SPF/DKIM check. Sending through one instance-level provider set in deploy config: SES, Postmark, Resend, Mailgun, SendGrid, or any SMTP relay. Two streams never mixed: system (teammate mail) and customer (contact mail). No SMTP receiver, no per-workspace provider keys.
  - Correct threading (Message-ID, In-Reply-To, References, reply token address). Attachments. Quoted-reply stripping.
  - Chat and email in same inbox, same conversation when same person.
  - Agent ignores bounces, out-of-office, newsletters.

### 5. Chat app channels: Slack, Discord, Telegram, WhatsApp

- Why: vibe-coder communities live in Discord and Telegram. Early B2B customers live in Slack. Most of world lives on WhatsApp. One inbox for all. Fin added Discord and Telegram in 2026; Featurebase gates Slack/Discord to paid tier.
- Design:
  - `Channel` interface in core: receive inbound (webhook or gateway connection), send outbound, map external user to contact, map external thread to conversation, declare capabilities (attachments, typing, reactions, messaging windows; edits later: message-edit).
  - Every channel is first-party adapter in main repo. Same conversation model, same Dimmi Agent, same billing rule, same API/MCP/webhooks.
  - Cloud: shared Dimmi Slack app, Discord bot, Telegram bot flow, Meta tech provider. Self-host: `dimmi channels:add <name>` generates app manifest and walks user through creating own app or bot.

| Channel | How | User setup | Gotchas |
|---|---|---|---|
| Telegram | Bot API, webhook. Private chats (groups, forum topics later: telegram-groups). | Paste BotFather token. About 1 minute. | Easiest. Bot cannot message user first. |
| Discord | Bot: Gateway WebSocket plus REST. IC mode set (other modes later: discord-ticket-modes). | Click invite link (cloud) or create bot (self-host). | Gateway connection held by one replica (per-shard lease table). Large public bots need Discord verification and message content intent approval. |
| Slack | Slack app: Events API plus Web API. Customer channels and Slack Connect shared channels. Each thread is one conversation. | Install app, pick channels. | Slack Connect depends on customer's Slack plan [unverified]. Self-host uses own app manifest. |
| WhatsApp | Meta WhatsApp Cloud API, direct (no Twilio markup). | Meta Business account, phone number, business verification. | 24-hour customer service window; outside it only approved templates. Meta bills business directly under Tech Provider model; Dimmi passes nothing through. Verification can take days: biggest setup friction. |

- Bar:
  - Connect Telegram, Discord, Slack in under 5 minutes. WhatsApp as fast as Meta allows, with checklist showing verification status.
  - Reply from inbox with attachments. Delivery and read status where channel supports it.
  - Dimmi Agent answers on every channel, same procedures and citations.
  - Contact merge across channels by email or phone. Customer history shows every channel.
  - Typing indicators where supported. Clear error when message window closed (WhatsApp) with one-click template send.
- Wait for post-MVP (setup gated by third parties, not code): SMS (US A2P 10DLC carrier registration takes weeks), Instagram and Facebook Messenger (Meta app review), Microsoft Teams (Microsoft app store review).

### 6. Notifications where solo dev lives

- Why: solo dev not sit in dashboard.
- Bar: email, browser push, PWA push, Slack. Fire on new conversation and on AI handoff, under 10 seconds. Discord later: discord-notifications. Reply-from-Slack comes post-MVP.

### 7. Dimmi Agent (BYOK)

- Why: AI first answer is table stakes 2026. Every competitor charge per resolution. We charge nothing extra.
- Bar:
  - Connect key in one step: OpenAI, Anthropic, Gemini, OpenRouter, Bedrock, any OpenAI-compatible endpoint (Ollama, LM Studio, vLLM).
  - Knowledge sources: help center, docs URL or sitemap crawl, Markdown folder or Git repo, pasted text.
  - From key pasted to agent answering: under 10 minutes.
  - Answers only from sources, with citations. Says "I don't know" when not grounded. Silent when only acknowledgement possible.
  - "Talk to human" always visible and always honored. Handoff carry AI summary.
  - Playground to test questions before going live. Test mode free.
  - Token cost shown per conversation and per month. Hard monthly spend cap per workspace.
  - Keys envelope-encrypted (KeyManager interface: local key default, AWS KMS adapter). Never logged.

### 8. Help center

- Why: agent useless without docs. Public docs cut tickets, bring SEO. Empty docs is top blocker for new users.
- Bar:
  - Markdown articles, Tiptap editor, collections.
  - Import from URL crawl, Markdown folder, Git repo.
  - Server-rendered from monolith (Go templ + goldmark). Zero JS by default. Fast, SEO-clean.
  - Custom domain. Fast search. "Ask AI" bar. In-widget Help tab.
  - `llms.txt` plus `.md` version of every page.
  - Agent-only articles (visible to agent, hidden from public).
  - **Cold-start generator:** CLI or MCP drafts full help center from codebase plus README into review queue. Nothing auto-publishes.
  - **Review queue with inline diffs:** +/- line counts, in-place highlights, accept or reject per edit, provenance link, version history. Productlane is bar.

### 9. Agent-native: CLI, MCP, API

- Why: audience lives in Claude Code and Cursor. Fin ships CLI. HelpDesk MCP read-only. Featurebase MCP has 168 tools.
- Bar:
  - `npx dimmi init`: detect framework, install widget, write JWT endpoint, seed help center from repo, print deploy button. Works for cloud, any region, and self-host URL.
  - Install by prompt: ship Claude Code skill, Cursor rule, `llms.txt` docs. "Add Dimmi support chat with secure login" works first try in Claude Code, Cursor, Lovable, Bolt.
  - CLI auth by OAuth device flow. Non-expiring scoped tokens. `--json`, `--dry-run`.
  - MCP server (stdio via `dimmi mcp`, plus remote HTTP with OAuth): read conversations, reply with confirmation (always confirm; later: agent-send-modes, reply-drafts), tag, assign, close, articles CRUD, search.
  - REST API, OpenAPI 3.1 generated from Go types (huma). Same spec feeds docs, CLI, MCP tool schemas.
  - API unmetered. Rate limits documented.

### 10. Webhooks done right

- Why: devs wire support into own app.
- Bar: HMAC-SHA256 with timestamp, retries with exponential backoff over 24 hours, delivery log with redeliver button, subscriptions via UI and API, documented event catalog with payload examples.

### 11. One-command self-host

- Why: core promise. Salesforce lock-in fear. No competitor in this set self-hosts.
- Bar:
  - `docker compose up`: Dimmi binary + Postgres 18 (tuned small-memory configuration) + Valkey (rebuildable state only: realtime, cache, rate limits). Optional Caddy for auto TLS.
  - Bare binary, no Docker: `install.sh` downloads binary, installs Postgres 18 + pgvector and Valkey (skipped when `--database-url` / `--redis-url` given), runs it under systemd. No all-in-one image, ever (PO 2026-11-02).
  - Whole stack (app + Postgres + Valkey) under 256MB RAM idle on $5 VPS. Dimmi binary target under 50MB idle, Valkey about 10MB. Measure and publish.
  - One-click deploy: Railway, Render, Fly, DigitalOcean, Coolify, Dokploy, each on platform-managed Postgres + Valkey/Redis.
  - Upgrade by pulling image. Migrations auto-run under advisory lock.
  - `dimmi backup`, `dimmi doctor`. Full JSON export. Unlimited history.
  - Postgres only. No SQLite mode (see `tech-stack.md`: dual database is trap). Competitor docs suggested "SQLite or Postgres"; tech research overrules.

### 12. Hosted cloud with honest metering

- Why: revenue. Pricing is top complaint against every competitor.
- Bar:
  - One plan, every feature. Unlimited seats, teams, inboxes, integrations. Audit log included at launch. SSO ships post-MVP, never paywalled.
  - First 100 conversations per month free, no card. Card asked once past 100.
  - Graduated: $0.03 per conversation 101 to 1,000, $0.015 to 10,000, $0.01 to 50,000. Above 50,000: talk to us, annual contract, custom rate.
  - Margin floor: price never below 5x measured cost per conversation (80% gross margin). Bands 2 and 3 need cost cuts in `pricing.md` section 3.
  - Example bills: 50 conversations $0, 500 $12, 5,000 $87, 10,000 $162, 25,000 $312.
  - Same price whether bot or human answered. No AI fee ever (BYOK).
  - Billable conversation: customer writes and human or AI replies. One charge per thread until 7 days quiet, max 30 days. Spam, tests, internal threads, auto-replies, imports, duplicate merges free. Full spec in `pricing.md`.
  - Live usage meter. $50 spend cap on by default, alerts at 50%, 80%, 100%.
  - Billing stack: own Postgres usage ledger plus Stripe Billing and Stripe Tax. Polar if merchant of record wanted.
  - Pricing page with calculator vs Fin, Featurebase, Productlane, HelpDesk, using dated sources.

### 13. Changelog

- Why: anything we use ourselves as a business ships at launch so we showcase it. Dimmi runs own public changelog on Dimmi (help center host `/changelog`). First page visitors of dev tools read after landing page. Featurebase bundles changelog + email subscribers; Productlane drafts it from Linear.
- Bar:
  - Markdown entries, labels (New, Improved, Fixed), cover, version tag, schedule, drafts. Lives on help center host (`/changelog`); custom domain = help center domain.
  - Zero JS, Lighthouse 100. RSS. `.md` twin per entry, listed in `llms.txt`; public MCP `dimmi_search_docs` returns changelog chunks.
  - Drafts from GitHub releases and merged PRs, BYOK rewrite in customer language, or user's coding agent via MCP. Nothing auto-publishes.
  - Widget "What's new" tab with unread badge per contact. Emoji reactions without login.
  - Email subscribers: double opt-in, one email per announced entry, 5-minute undo, one-click unsubscribe, no tracking pixels.
  - Dimmi Agent cites changelog: "CSV export shipped on 2026-10-30".
  - Free: no conversation, no meter. Spec `specs/13-changelog`.

### 14. Feedback board and public roadmap

- Why: we run our own feedback board and roadmap on Dimmi (`feedback.trydimmi.com`, `/roadmap`), so it ships at launch. Featurebase moat is feedback loop; Canny, Productlane, Linear Customer Requests charge for it. Inside support tool, capture from conversations is native, not integration.
- Bar:
  - One click from inbox message to linked request (vote + quote + subscription, one API call). Similar posts while typing under 50 ms.
  - Boards, statuses, public roadmap on help center host. Zero JS required to read, vote, post; Lighthouse 100.
  - Widget feedback tab (lazy chunk 15 KB gz or less), sign-in with widget identity, no extra step.
  - Revenue weighting from company attributes. Trending sort.
  - Notify-on-ship: status change emails every voter and requester after undo window; changelog entry (13) links shipped posts. One-click unsubscribe.
  - BYOK AI: capture requests from closed conversations, duplicate detection. AI only suggests, human accepts.
  - Free: no conversation, no meter. Spec `specs/14-feedback-roadmap`.

### 15. In-app announcements and surveys

- Why: we announce launches, warn about outages, run NPS and rate support inside our own app. Intercom Posts/Banners/Surveys, Chameleon, Userpilot, Refiner charge per MAU. CSAT on close moves into launch here.
- Bar:
  - Announcements: banner, modal, chat-style message from teammate, targeted by segments (custom attributes, identity, page rules), scheduled, frequency capped (max 2 interruptive per day per contact).
  - Surveys: NPS, CSAT, CES, choice, text, simple branching. In widget; CSAT also by email (minimal rating page).
  - CSAT after conversation close on widget and email; results split by AI vs human.
  - Zero cost when unused: no extra widget request, core bundle grows 1.5 KB gz or less.
  - Never metered. Spec `specs/15-announcements-surveys`.

### 16. Self-updating docs (extends 08)

- Why: PO moved into launch 2026-11-02 (audit gap G3). Productlane and Ferndesk core pitch. Stale docs make agent wrong; vibe coders ship daily.
- Bar:
  - Merged PR (one GitHub webhook per repo, token shared by 08 git source, 08 docs sync, 13 changelog) -> suggested article edits in 08 review queue. Provenance link to PR. Freshness: articles whose source files changed flagged stale.
  - Per-edit accept/reject, +/- counts (08 review queue as is). Nothing auto-publishes.
  - BYOK AI via 07 `ai.RunTask`, counted in spend cap. Sensitivity setting: skip PRs with no user-facing change. Customer code never stored or logged.
  - Deferred by PO 2026-11-02 (Later): drafts from resolved conversations (`conversation-drafts`), PR into docs repo (`docs-as-code-pr`), pre-merge PR check (`docs-pr-check`), GitHub App install (`github-app`).
- Owner: `specs/08-help-center/docs-sync.md` (extension; provenance kind `commit` live, `conversation` reserved). AI calls 07.

### 17. Importers (new spec `16-importers`)

- Why: PO moved into launch 2026-11-02 (audit gap G2). Salesforce/Fin churn window open now. Featurebase, Productlane, Ferndesk (22 platforms) and Intercom all import; switching cost is first objection.
- Bar:
  - `npx dimmi migrate intercom`: contacts, companies, conversations, articles (keep URLs via 08 redirects). `window.Intercom` shim keeps `Intercom('boot')` calls working.
  - Imports never billed (12 metering: imports free), idempotent re-run, resumable job with progress, dry-run report first.
  - Launch sources (decided in `16-importers`): Intercom, Zendesk, Help Scout, Featurebase, Productlane, Canny, any public help center URL, CSV. Others later (`more-import-sources`). Feedback CSV import stays in 14 (fold later: `feedback-csv-unify`).
  - Preview, mapping, delta sync, rollback; no per-record events, no notifications, no outbound.
- Owner: `specs/16-importers`. Touches 03 (contacts, conversations), 08 (articles, redirects), 12 (not billable), 13 (entries), 14 (posts, votes), 01 (`window.Intercom` shim), 09 (`dimmi migrate`).

### 18. Agent actions (extends 07)

- Why: PO moved into launch 2026-11-02 (audit gap G1). Fin Procedures main moat; Productlane connectors, Featurebase custom actions. Billing and account questions dominate SaaS support; answers alone cannot fix them.
- Bar:
  - HTTP tools (method, URL template, auth from encrypted secret, JSON schema params), OpenAPI import, MCP client (agent calls user's MCP server).
  - Read actions run inline; write actions need confirmation (customer clicks Dimmi card in widget; teammate/admin approval for risky). Writes widget only at launch (email confirmations later: `email-action-confirmations`). Identity bound only from verified context. SSRF guard on every call (`a.HTTP`).
  - Templates: Stripe subscription lookup, Supabase auth user, Clerk user, Resend.
  - Every call in run trace with inputs/outputs redacted, latency, cost. Playground tests actions with fake or live mode.
- Owner: `specs/07-dimmi-agent` (extension). 09 exposes action CRUD over REST/MCP/CLI.

### Not at launch

Linear, SMS, Instagram, Facebook Messenger, Microsoft Teams, translations, workflows, SLAs, mobile SDKs, reports beyond basic counts, voice, email/broadcast outbound campaigns, product tours and checklists, status page. Scope-audit deferrals: "Later" section.

## Must ship after MVP (strict priority)

Specs reference these by **name** ("post-MVP: mobile SDKs"), never by number. Numbers are display order only and stay stable: items pulled into launch keep their row as a pointer; specs then cite "launch: <name>".

| # | Name (stable reference) | Item |
|---|---|---|
| 1 | importers | Moved to launch 2026-11-02: launch 17, spec `16-importers`. |
| 2 | agent actions | Moved to launch 2026-11-02: launch 18, extends `07-dimmi-agent`. |
| 3 | GitHub loop | **GitHub Issues link and loop closing.** Most solo devs use GitHub Issues, not Linear. Tracker ticket pattern: one bug, many users. Merge or deploy triggers "fixed" message to every affected user. |
| 4 | self-updating docs | Moved to launch 2026-11-02: launch 16, extends `08-help-center`. |
| 5 | knowledge-gap report | **Knowledge-gap report.** Weekly list of questions agent failed, grouped. One click to draft article. Recent data only (Fin's suggestions use year-old conversations). |
| 6 | agent evals | **Agent evals.** `dimmi test` replays past conversations against new prompt, model, or docs. GitHub Action. Diff answers, cost per model. Shows BYOK value: pick cheapest model that passes. |
| 7 | chat replies | **Team reply from Slack and Discord notifications.** Answer conversation from any channel inside internal Slack or Discord thread. Solo dev never opens dashboard. |
| 8 | more channels | **More channels: SMS, Instagram, Facebook Messenger, Microsoft Teams.** Same `Channel` interface. Waiting on carrier registration and app reviews, start those early. |
| 9 | reports | **Reports.** Volume, first response, resolution time, AI resolution rate with published definition (reopen window, denominator), CSAT trends (CSAT collection itself ships at launch in 15), cost per resolution. HelpDesk paywalls reports; ours free. |
| 10 | mobile SDKs | **React Native / Expo SDK, then native iOS and Android.** Many vibe-coded apps mobile. Featurebase widget web-only. |
| 11 | rules lite | **Rules lite.** Office hours, auto-assign, auto-tags, auto-replies, HTTP step, SLA timers. Needed once second teammate joins. |
| 12 | agent assist | **Agent assist for humans.** Draft reply, summarize, translate, tag suggestion. Fin charges $29 to $35 per agent; BYOK make free. |
| 13 | feature requests | Moved to launch: 14 feedback board and public roadmap (capture, notify-on-ship); API importers (Canny, Featurebase, Productlane) in 16. Later extras: Linear/GitHub sync of posts. |
| 14 | Linear | **Linear integration.** Win Linear-native teams from Productlane. Linear project completion drafts changelog entry. |
| 15 | auto-translation | **Auto-translation** of help center, changelog and conversations via BYOK. |
| 16 | ticket fields | **Ticket types, custom fields, views, macros.** |
| 17 | upmarket set | **Upmarket set:** QA scoring, SSO/SCIM, CRM sync (HubSpot, Salesforce), product tours and checklists, email/broadcast outbound, voice. In-app banners and announcements moved to launch (15). Never paywalled in open source. |

## Later (deferred from MVP on 2026-11-02, scope audit)

Source: `scope-audit-4-competitors.md` sections 1 (C1-C16) and 2 (B1-B20). PO: every C and B item deferred, except C3, C4, C5, B17 (stay MVP). Rows marked "PO (round 3)" deferred while specifying launch items 16-18. Specs reference by **name**: "later: <name>". Names stable; owner spec keeps an Out (later) row.

| Name | Audit | Owner | Item | Competitor note |
|---|---|---|---|---|
| imap-adapter | C1 | 04 | IMAP mailbox adapter (Gmail, Fastmail, iCloud, Zoho, Yahoo; IDLE + poll) | IC, FB, PL forwarding only; FD no email channel |
| message-edit | C2 | 03, 05 | Customer-visible message edit + edit/delete propagation to Slack/Discord/Telegram/WhatsApp | IC replies delete only; FB/PL unclear; FD no inbox |
| dimmi-listen | C6 | 10, 09 | `dimmi listen --forward-to localhost` (SSE stream of signed events to local dev) | none of the 4 (Stripe CLI pattern) |
| bulk-recover | C7 | 10 | Bulk webhook "recover all failed since timestamp" | PL single replay only; IC none; FB backfill via API. Single redeliver stays |
| changelog-digest | C8 | 13 | Weekly changelog digest cadence, per-subscriber override | IC/PL/FD no, FB unclear |
| atom-json-feeds | C9 | 13 | Atom and JSON Feed (CORS headless JSON); RSS stays | FB, PL, FD RSS only; IC no feed |
| portal-md-twins | C10 | 14 (08 llms.txt) | `.md` twins of feedback portal pages and roadmap, portal llms.txt sections | FB, PL, FD do `.md` for docs only |
| survey-ai-summary | C11 | 15 | AI summary of open-text survey answers (themes + quotes) | IC/FB unclear; PL/FD no surveys |
| hosted-survey-page | C12 | 15 (12 seam) | Dimmi-hosted survey page, `/survey-forms/*`, public/signed links, unpaid daily cap | IC partial (own page + Messenger); FB/PL/FD no. MVP keeps minimal CSAT email rating page |
| anonymous-survey-responses | C13 | 15 | `anonymize_responses` survey setting | none found in IC/FB survey docs |
| pmf-survey-template | C14 | 15 | Product-market-fit survey template | not in IC survey types; FB templates NPS/CSAT/CES only |
| teammate-typing | C15 | 03 | Teammate "is typing" relay between teammates (collision) | IC shows viewing only. "Is viewing" + 409 guard stay |
| identity-helper-packages | C16 | 02, 09 | SDK/identity framework packages beyond Next.js, React, browser, `@dimmi/server` (SvelteKit, Nuxt, Astro, Vue, Svelte, Laravel, gem, pip, Go) | all 4 publish copy-paste snippets only |
| discord-notifications | B1 | 06 | Discord team notifications (personal webhook + workspace destination) | FB partial (new posts only); IC/PL no. Slack stays |
| telegram-groups | B2 | 05 | Telegram groups (mention/reply chains) and forum topics | IC Telegram, group support unclear. Private chats stay |
| discord-ticket-modes | B3 | 05 | Discord private `/support` tickets, forum posts, text-channel threads beyond IC mode set | IC Discord, sub-modes unclear |
| teammate-echo | B4 | 05 (03) | Teammates replying natively in Slack/Discord thread recorded as replies | PL partial; IC unclear |
| unverified-sender-flag | B5 | 04 | Inbox "unverified sender" flag from inbound DKIM/SPF/DMARC | IC partial (impersonation warning) |
| extra-mail-adapters | B6 | 04, 11 | Self-host inbound adapters (Cloudflare Email Routing worker, Postmark, Mailgun) beyond SES receiving | none (no competitor self-hosts) |
| identity-validations | B7 | 02 | Identity validations endpoint, mistake detector, rollout stats, `off/optional/required` modes | IC partial (enforcement toggle) |
| agent-send-modes | B8 | 09 | Agent send modes `confirm` / `drafts_only` / `direct`; MVP always confirms | FB partial (MCP drafts) |
| reply-drafts | B9 | 09 | Server-side reply drafts (`reply_drafts`) over REST/MCP/CLI | FB partial (MCP composer drafts) |
| feedback-eta | B10 | 14 | ETA on feedback posts (month/quarter, public toggle) | FB/PL unclear |
| feedback-ai-summary | B11 | 14 | On-demand AI summary of requests per feedback post | FB partial (Insights quotes); PL no |
| changelog-custom-domain | B12 | 13, 08 | Dedicated changelog custom domain (`purpose = changelog`) separate from help center domain | FB/PL portal domain covers changelog; per-purpose domain none |
| public-mcp-changelog | B13 | 13, 08 | Public MCP tool `dimmi_list_updates` | FD public MCP covers help center; changelog tool unclear |
| changelog-replies-to-inbox | B14 | 13 (01, 03, 04) | Changelog email replies + widget "Send feedback" land in inbox tagged with entry | none documented |
| chat-app-csat | B15 | 15, 05 | CSAT over Slack/Discord/Telegram/WhatsApp | IC/FB unclear. Widget + email CSAT stay |
| domain-join | B16 | 00-accounts | Workspace domain join (verified domain, users see and join) | IC/FB/PL unclear |
| review-rebase | B18 | 08 | 3-way rebase of pending review suggestions onto edited article | PL accept/reject per edit (stays); rebase none |
| delivery-inspection-api | B19 | 06 | Notification delivery inspection API (`/notifications/{id}/deliveries`) | none documented |
| conversation-restore | B20 | 03 | Conversation soft delete + admin restore | IC delete yes, restore unclear |
| conversation-drafts | PO (round 3) | 08 | Article drafts from resolved conversations (`source_kind conversation` reserved); ships with post-MVP: knowledge-gap report | PO deferral (08 docs-sync.md Out) |
| docs-as-code-pr | PO (round 3) | 08 | Dimmi opens PR in user's docs repo instead of in-place suggestion (needs Contents + Pull requests write) | PO deferral (08 docs-sync.md Out) |
| docs-pr-check | PO (round 3) | 08 | Pre-merge GitHub check "this PR makes 3 articles wrong", drafts attached (needs Checks write, GitHub App) | PO deferral (08 docs-sync.md Out) |
| github-app | PO (round 3) | 08 (13) | GitHub App connection kind (`github_connections.kind = app`): one-click install, org-owned, auto webhooks, short-lived tokens; lands with post-MVP: GitHub loop | PO deferral (08 docs-sync.md D1) |
| more-import-sources | PO (round 3) | 16 (13, 14) | Crisp, Front, Freshdesk, Gorgias, HubSpot inbox, LiveChat/HelpDesk, Plain, Pylon, Kustomer; Beamer, Headway, LaunchNotes; Productboard, Frill, UserVoice (driver = registry entry, zero migration) | PO deferral (16 README Out); FD imports 22 platforms |
| feedback-csv-unify | PO (round 3) | 16, 14 | Fold 14 feedback CSV import into 16 pipeline | PO deferral (16 README decision) |
| email-action-confirmations | PO (round 3) | 07 (04) | Agent write actions confirmed over email ("reply YES"); needs spoof and thread-hijack review | PO deferral (07 actions.md Out) |

Stay MVP (PO): C3 RS256/EdDSA identity keys (02), C4 JWT nonce replay cache (02), C5 ALTCHA proof-of-work (01), B17 help center path mode `/hc/{sub}` (08).

## Standout features (after post-MVP list, for marketing)

Ranked by marketing punch for our audience.

1. **"Customer complains, PR appears."** Agent confirms bug, gathers repro (console errors, failed network calls, route, app version, user), opens GitHub issue, offers "send to Claude Code / Codex / Copilot agent". Customer watches status in widget. Deploy webhook (Vercel, Netlify, Fly, Railway) sends "fix is live".
2. **Developer context on every conversation.** Widget auto-captures console errors, failed requests, URL, app version, browser. Links to Sentry or PostHog session. Fix bug from one screen.
3. **Error-triggered proactive support.** Sentry or error webhook opens conversation with affected user: "saw checkout fail, fixed, here is credit."
4. **Agent reads your repo.** Vibe coders often have no docs. Code is docs. Answer from README, OpenAPI, code, migrations.
5. **Deploy-aware answers.** Agent knows deployed commit. Never describes unshipped features. Flags drift when code and article disagree.
6. **Agent CI.** PR preview shows how agent answers change. "Tests for your support agent." No competitor markets this.
7. **Zero-markup AI receipt.** Monthly receipt: "Fin would have charged $312. Your tokens cost $4.10."
8. **"We do not profit when your bot deflects" pledge.** Same price bot or human. Human button always visible. Optional 15-minute human SLA toggle (California AB 1609 coming).
9. **Local model mode.** Self-host plus Ollama equals fully private AI support, zero third-party data flow. EU and privacy angle. Fin MCP and CLI US-only.
10. **Support from terminal.** CLI and MCP: list, reply, close, write articles from Claude Code session.
11. **Personal-agent channel.** Help center read-only public MCP plus `llms.txt` already ship at launch (08). Add Personal Agent Protocol (Sierra + Meta) and PACT (Decagon) for verified agent-initiated tickets.
12. **Self-healing screenshots.** Headless browser recaptures help center screenshots with seed data on each deploy.
13. **Headless widget SDK.** Hooks (`useConversation`) for own UI.
14. **Published widget weight.** Size and Lighthouse impact badge vs Intercom.
15. **Open agent prompts and published resolution definition.** Prompts in repo. No "silence equals resolved".
16. **Vibe-coder templates.** Ready agent actions for Supabase, Stripe, Clerk, Resend, Lemon Squeezy, Polar.
17. **Data ownership pledge.** Full export any time, no history caps, open-source license, no acquisition rug-pull.

## Build notes from tech research

- Go 1.26 backend: chi + huma, pgx + sqlc, River (jobs, cron, leader), goose migrations, koanf configuration, embedded centrifuge for realtime (`specs/00-conventions/framework.md`, `build-vs-reuse.md`).
- PostgreSQL 18 only database of record. Required extensions: pgvector, pg_trgm, unaccent. Valkey required for rebuildable state: realtime broker/presence/history (centrifuge Redis broker), cache, rate limits, internal pubsub (PO decision 2026-11-02; replaces Postgres LISTEN/NOTIFY fan-out, which hits commit lock contention).
- Frontend: React 19 + Vite + TanStack Router/Query + shadcn/ui (Base UI) + Tailwind v4, embedded via `go:embed`. Tiptap 3 editor. Preact widget in Shadow DOM.
- Adapters compiled in: Filesystem (local disk default, S3), Mailer (one instance provider: SES, Postmark, Resend, Mailgun, SendGrid, SMTP relay), Mailbox sources (SES receiving at MVP; IMAP and provider webhooks later; no SMTP receiver), LLM (official SDKs), KeyManager (local, KMS), Valkey (cache, limits, pubsub, realtime), Channel (widget, email, Slack, Discord, Telegram, WhatsApp at launch). MinIO archived 2026; recommend SeaweedFS, RustFS, Garage, R2 for self-host S3.
- Same codebase multi-tenant (`workspace_id` everywhere). Cloud billing behind configuration flag. No enterprise fork.

## Open decisions

1. License: decided MIT (PO 2026-11-02). Dependencies MIT-compatible only.
2. Trademark check on Dimmi Health conflict before filing.
3. Pricing: decided 100 free, then $0.03 / $0.015 / $0.01, talk to us above 50,000. Open: measure real cost per conversation before publishing bands 2 and 3. At 50,000 human-only, Crisp ($295 flat) still cheaper unless our cost reaches $0.0012.
4. Cold-start generator and review queue stay in launch scope: decided (self-updating docs, launch 16, builds on review queue; empty docs kill agent).
