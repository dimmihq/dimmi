# Featurebase competitor research

Research date: 2026-11-01. Subject: Featurebase (featurebase.app), Tallinn, Estonia. Lens: Dimmi first users (vibe coders, solo devs, early adopters).

Tags: [unverified] = single source or conflicting sources. [3P] = third-party or competitor-written claim.

## TL;DR

- Featurebase start 2021 as cheap Canny clone (feedback voting board). Add roadmap, changelog, help center. Launch support platform (inbox, Messenger, Fibi AI agent) May 2025. Now sell "AI support across every channel" suite; feedback is supporting act, not headline.
- Bootstrapped, profitable, 8 people, claims 1,600+ customers. Logos hit our audience directly: Lovable, t3.chat, Indie Hackers, Raycast, n8n, Base44, Screen Studio, Typefully.
- Pricing: per seat ($29 / $59 / $99 billed yearly; $37 / $75 / $129 monthly) plus Fibi $0.49 per resolution (raised from $0.29 on 2026-07-01, existing customers included). Free tier: 1 seat, unlimited conversations, no AI.
- Real moat: feedback loop. Conversations auto-linked to feature requests (Feedback Autopilot), MRR-weighted prioritization, two-way Linear/Jira sync, auto email to requesters when shipped.
- Developer story strong: `featurebase-js` npm package, React provider, JWT identity, date-versioned REST API, webhooks, hosted MCP server with 168 tools over OAuth.
- Weak spots for solo devs: no AI on free, many meters (resolutions, Copilot, outbound, branding removal $69/month), per-seat scaling, cloud only, EU hosting only [3P], model hidden, web-only Messenger (no native mobile SDK).
- Dimmi opening: self-host plus BYOK means zero AI markup and no seat tax. Featurebase charges $0.49 per resolution; Dimmi user pays only own LLM tokens.

## Company and market status

| Item | Value |
|---|---|
| HQ | Tallinn, Estonia, fully remote |
| Founders | Bruno Hiis (CEO per Latka [3P]), Robi Rohumaa, Markus Palm |
| Founded | About page says 2022; blog and guides say end of 2020; first MVP early 2021 [unverified, conflict] |
| Funding | None. "Bootstrapped and profitable from day one." 100% founder-owned, three acquisition offers declined (founder blog) |
| Team | 8 people (about page). First 2 years built by two engineers [3P] |
| Customers | 1,600+ (about, customers page); 1,500+ earlier [3P] |
| Revenue | 100K ARR with 163 paying customers on 2023-09-26 (founder blog). Latka says $54.5K revenue 2024 [3P, unverified, likely wrong]. Robi Rohumaa LinkedIn says "$2M+ ARR" [unverified] |
| Compliance | SOC 2 Type II, GDPR, EU data hosting (Netherlands, Germany, Ireland [3P]) |

Origin. Founders previously ran startup to $20K MRR, starting age 14 (founder blog). Built Featurebase as cheaper copy of proven paid tools. MVP flopped on Product Hunt, Indie Hackers, Reddit, Discord in early 2021; shelved over year. Relaunched polished 2.0 early 2022, reached $1,200 MRR by end of 2022. Featurebase 3.0 on Product Hunt: feedback boards, voting, roadmap, changelog, notify upvoters on ship.

Pivot timeline (official changelog unless noted):
- 2023-03-06: Linear integration launch.
- 2025-05-30: Support platform. Inbox, Messenger, Fibi AI agent, email channel, macros. Included in all plans. All-in-one widget deprecated for Messenger. Pitch: cut Intercom and Zendesk spend.
- 2025-12-04: switch from flat plans to per-seat pricing [3P, unverified].
- 2026-01-22: Copilot AI assistant inside inbox.
- 2026-02-25: multi-board Linear and Jira sync with per-board rules.
- 2026-04-14: Tickets and Ticket Portal.
- 2026-05-04: MCP server (Professional at launch; now all paid plans per llms.txt).
- 2026-06-18: Fibi 3.0; resolution price $0.29 to $0.49 from 2026-07-01 for new and existing customers [3P cites changelog].
- 2026-07-07: Outbound (chats, banners, emails, surveys, journeys).
- 2026-07-14 to 2026-10-28: spam filter, Reports, Slack as support channel, Slack notifications, Salesforce/HubSpot sync, native mobile app rebuild, Feedback 2.0 (Requests plus Insights), customer portal redesign with llms.txt, Product Tours, workload management.

Shipping cadence: weekly-ish major entries. Small team ships fast. Expect parity pressure on any Dimmi feature.

Market position: homepage headline "AI support across every channel, built for modern software teams." Claims Fibi "resolves up to 80%." Competes now with Intercom/Fin, Zendesk, Crisp, Plain, plus legacy feedback rivals Canny, Frill, Productboard.

## Pricing

| Plan | Yearly per seat per month | Monthly per seat | Key adds |
|---|---|---|---|
| Free | $0 | $0 | 1 seat, live chat, inbox, tickets, unlimited conversations and end users, help center (50 articles), mobile app, 1 board, 1 roadmap, 500 survey shows per month |
| Growth | $29 | $37 | Fibi, email channel, AI Copilot, macros, banners, surveys, reports, API/MCP/webhooks, integrations, 5 boards |
| Professional | $59 | $75 | Slack, WhatsApp, Discord, workflows, SLAs, round robin, private and multilingual help center, AI translations, HubSpot/Salesforce, Fibi custom actions, 20 free Lite seats, 10 boards |
| Enterprise | $99 | $129 | Multi-brand, Microsoft Teams, SAML SSO, SCIM (min 5 seats), custom roles, Azure DevOps, 50 Lite seats, demo-gated |

Meters on top:
- Fibi: $0.49 per resolution, all paid plans. Charged once per conversation, only when customer confirms or leaves without more help. Refunded if customer returns. Spend alerts and hard cap exist. Handoffs free.
- AI Copilot: 10 conversations per seat per month free, then $19 per seat per month unlimited.
- Remove branding: $69 per month, Professional and Enterprise only. Solo dev wanting unbranded widget pays minimum $59 plus $69 = $128 per month.
- Outbound: 5,000 emails per month included, then from $0.001 per email. Tours and surveys: 500 views per month, then from $0.0045 per view.
- Translation overage above 250,000 characters [3P, unverified].

Lite seats: free collaborator seats (sales, marketing, engineering) on Professional (20) and Enterprise (50). Can manage changelog, people, companies; restrictions not fully documented [unverified]. Formbricks calls them read-only [3P], conflicting with help center.

Discounts:
- Startup program: Professional 86% off year one ($63 per month instead of $450), 40% off year two, 10% off year three. Fixed 6 full seats plus 20 Lite seats, 1 year Fibi with 50 free resolutions per month. Eligible: founded under 2 years, under 6 employees, under $3M raised, new to paid plans.
- Open source and nonprofit: 15% off.
- Trial: 10 days, no card.

Price history: early flat plans Growth $35 then $49, Premium $79 then $99 (founder blog). GetApp still lists flat $49 / $99 / $207 [3P]. Founder wrote raising prices was "highest-impact, lowest-effort change" with no drop in sign-ups.

Cost math [3P]: 300 resolutions per month cost about $147, roughly five Growth seats. MyAskAI models 10 seats plus 10,000 tickets at $3,040 to $4,265 per month.

## Feature inventory

### Help desk / inbox
- Shared inbox: live chat, email, Slack, Microsoft Teams, WhatsApp, Discord (channel availability by plan; MyAskAI disputes some [3P]).
- Tickets: customer, back-office, tracker tickets; Ticket Portal for customers to submit and track.
- Assignment, round robin, balanced assignment (Enterprise), priorities, SLAs, away modes, backup teammates, capacity dashboard.
- Internal notes, macros, AI Copilot drafts, conversation attributes, bulk actions, spam filter, custom inbox views.
- Native iOS and Android agent apps (rebuilt August 2026) with Copilot drafting.
- Reports: 12 templates, custom dashboards, CSAT, SLA, AI performance, CSV export, scheduled reports, query via MCP.

### Messenger
- Modules: Home (cards), Messages, Help (help center search), Changelog feed, Fibi, Product Tours.
- Per-brand themes, separate light and dark themes, auto-translated UI.
- Anonymous visitors treated as leads. Identity via server-signed JWT (`featurebaseJwt`), secure install on by default. Legacy HMAC still works.
- Web only. Help FAQ: Messenger works only on web pages and web apps; mobile embed "planned." Portal can embed in iOS/Android via WebView with auto sign-in. Fibi page lists iOS and Android as channels [unverified, conflict].

### Fibi AI agent
- Knowledge: help center, API reference, changelog, feedback portal, plus files, websites, Q&A.
- Custom actions: HTTP GET/POST/PUT/DELETE to own API (Professional plus).
- Guidance: custom instructions, guardrails (no refunds, no discounts), handoff rules, plan-based routing (free users to AI, paid users to humans).
- Fallback: says when unsure, points to docs; claims never invents links or presents planned features as shipped.
- Self-improvement: flags knowledge gaps, drafts articles, actions, Q&A; human approves. "Improve answer" on any reply.
- Claimed 73% average resolution versus 65% for "leading agents" (vendor, unaudited [3P]).
- Model not named, no model choice, no documented simulation or replay against historical tickets [3P].
- Fibi deploys only on Featurebase channels; cannot run inside other help desks [3P].

### Help center
- Custom domain, branding, editor with custom blocks (steps, callouts, code tabs, accordions), version history with restore.
- AI translation into 70+ languages, per-user language. Multilingual and private help center on Professional.
- Access control per team, user, segment. Ask AI search. Article analytics.
- API docs inside help center with live versions and playground (October 2026; free plan included).
- llms.txt support, positioning docs for citation by ChatGPT and Claude.
- Edit articles from Claude or ChatGPT via MCP.

### Feedback, roadmap, changelog
- Boards with voting, Top/New/Trending, categories, follow posts, in-app widget with screenshot annotation, merge duplicates.
- Statuses: In Review, Planned, In Progress, Completed, Rejected; public roadmap.
- Prioritization by Revenue (sum of requester MRR), Trending, Impact, Value/Effort.
- Feedback 2.0 (September 2026): Requests backed by Insights (quotes) from support, calls, Slack, email, Intercom, Zendesk, HubSpot, Salesforce.
- Feedback Autopilot: AI scans conversations, finds requests, links to existing Request or suggests new post. Call recorder ingestion (Gong, Fathom, Granola, Grain, Fireflies, tl;dv); Fireflies in beta, quotes suggested not auto-posted.
- Close loop: one update posts to request, emails subscribers, replies inside linked conversations.
- Linear: create issue from post, link existing issue or project, two-way status sync rules (bidirectional or one-way), notify upvoters by email when status changes in Linear, project due dates sync. Jira: same model with per-board issue type. Also GitHub, ClickUp, Azure DevOps.
- Changelog ("Updates"): page, in-app widget, email subscribers, preview emails.

### Outbound
- In-app chats, banners, emails from teammate, Slack broadcasts, product tours with tooltips, surveys (NPS, CSAT, custom), Journeys canvas with waits, branches, A/B tests up to 5 paths.
- Targeting by user/company attributes, page, time on page, events, CRM data.

### Platform and DX
- REST API at `do.featurebase.app/v2`, Bearer `sk_` keys or OIDC, date-based versions pinned via `Featurebase-Version` header (example `2026-08-19.orbit`), cursor pagination, Stripe-style error objects.
- Endpoint groups: boards, posts, comments, changelogs, admins, teams, workload, brands, audit logs, contacts, companies, surveys, help centers, conversations, tickets, webhooks, reports, training data.
- Webhooks with event reference.
- MCP: hosted at `mcp.featurebase.app`, OAuth (no key), 168 tools (88 read, 56 write, 24 delete), drafts need confirmation before sending, works with Claude, Claude Code, ChatGPT, Codex, Cursor, VS Code.
- Importers on all plans: Intercom, Zendesk, Freshdesk, Crisp, Canny (no voters or comments), Productboard, Frill, Beamer, HubSpot, Salesforce, CSV.
- Integrations: Slack, Discord, Zapier, Segment, Intercom, Zendesk, HubSpot, Salesforce, Linear, Jira, GitHub, ClickUp.

## Onboarding and time-to-value for solo dev

Path: sign up free, no card. Install:

1. `npm install featurebase-js` (or pnpm, yarn, bun) or legacy script tag.
2. React/Next.js: wrap root in `FeaturebaseProvider` from `featurebase-js/react`. Else call `Featurebase({ appId })`.
3. Logged-in users: sign JWT on server, pass as `featurebaseJwt`. Secure install on by default, so solo dev must write server code before identified chat works.
4. Messenger auto-boots. Feedback, changelog, survey widgets need extra mount call each.

Docs claim "a few minutes of engineering time." Realistic solo-dev time: 15 to 45 minutes including JWT [unverified estimate].

Friction points:
- Free plan has no AI. Trying Fibi needs 10-day trial or $29 to $37 per month plus $0.49 per resolution.
- Branding stays unless $128+ per month.
- Startup program built for 6-seat teams; solo founder overpays for unused seats.
- Suite breadth means much configuration before value [3P].
- No self-host, no data residency outside EU.

Good: unlimited conversations on free, importers on free, MCP and llms.txt friendly, API docs free.

## What users love

- Price versus Canny and Intercom; "criminal" value (My-AskAI founder testimonial).
- Clean modern UI, easy setup, easy customization (Product Hunt, G2).
- Fast founder-led support. Founder blog: 320 live requests, about one-minute average response.
- Linear integration and close-the-loop emails.
- All-in-one: feedback, roadmap, changelog, help center, chat in one bill.
- G2 4.7/5 from about 48 to 49 reviews; Product Hunt 5.0/5 from 19 reviews.

## What users hate

- Cost grows per seat; key features behind higher tiers (G2 main con).
- Multiple meters; headline seat price hides AI, Copilot, branding, outbound costs [3P].
- 69% AI price hike applied to existing customers mid-2026.
- Trustpilot 3.2/5: billing error cancelled plan after nearly a year, forced to re-sign at higher price; one "worst service" review. Company does not invite Trustpilot reviews, small sample.
- No Slack upvoting (Product Hunt review); limited lifecycle messaging (addressed by Outbound July 2026).
- Bundle too heavy for feedback-only users; "solo founder under $50 per month budget should pick neither" Canny nor Featurebase [3P, competitor blogs].
- Support side young (tickets only April 2026) [3P].

"Drifted from feedback" criticism: no direct user complaint found. Only competitor framing (WishKit, Formbricks, Nolt-style vendors) that feedback-only teams pay for unused support tooling. Featurebase also pushes feedback forward (Feedback 2.0) so story is "feedback became input layer for support suite," not abandonment. Treat criticism as competitor narrative [3P].

## Go-to-market toward indie hackers and startups

- Free forever plan with unlimited conversations; "Powered by Featurebase" badge drives "significant traffic" (founder blog). Branding removal priced high on purpose.
- Huge "alternative to X" library: Intercom, Canny, Front, Trengo, Ada, LiveChat, Voiceflow, Chatbase, Gleap, Frill, FeatureOS, UserVoice, Aha!, Bettermode, and more. Backed by Google Ads and SEO blog posts ("16 Best Intercom Alternatives", "Plain pricing", "Missive pricing").
- One-click migration from competitors; importer in onboarding.
- Build in public: founder blog posts on bootstrapping to 100K ARR and $2,500 MRR, Indie Hackers profile, X.
- Product Hunt: 5 launches since 2021; BetaList, AlternativeTo.
- Startup program framed "founder-to-founder," "supporting other startups is one of our core values."
- Social proof from AI-native startups: Lovable, t3.chat, Base44, Hedra, Wispr Flow, Indie Hackers itself.
- Pre-selling niche features to trial users (founder blog). ROI calculator, demo for larger teams.
- Increasingly upmarket: Salesforce, SCIM, Microsoft Teams, workload management. Indie focus now secondary.

## Dimmi must-have at launch

Only items without which vibe coder will not switch or will churn in week one.

1. **One-command self-host plus hosted option.** Why: core promise; Featurebase cannot self-host. Bar: single `docker run` or binary, SQLite default, under 256 MB RAM idle, live in under 5 minutes, one-file backup.
2. **Messenger widget.** Why: first thing user installs. Bar: npm package plus script tag, React provider, under 50 KB gzipped, dark mode, file attachments, anonymous leads, JWT identity verification with copy-paste server snippet for Next.js, Express, Rails, Laravel, offline email capture, mobile-responsive.
3. **Shared inbox with email channel.** Why: solo dev answers chat and email in one place. Bar: inbound email via forwarding address, threaded replies from own domain, keyboard shortcuts, notes, macros, snooze, close, search, sub-second UI.
4. **Dimmi Agent (BYOK).** Why: AI answers are why people leave email-only setup; Featurebase charges $0.49 per resolution and blocks AI on free. Bar: paste key (OpenAI, Anthropic, Gemini, OpenRouter, local OpenAI-compatible), point at docs URL or sitemap, answering in under 10 minutes; citations; "I don't know" fallback; one-click human handoff; per-conversation token cost shown; hard monthly spend cap.
5. **Knowledge base / help center.** Why: agent needs source; public docs reduce tickets. Bar: Markdown articles, URL crawl import, custom domain, search, llms.txt, in-widget help tab.
6. **Notifications.** Why: solo dev not watching inbox. Bar: email, browser push or PWA push, Slack and Discord webhook on new conversation and handoff, under 10 seconds.
7. **API, webhooks, MCP server.** Why: audience lives in Claude Code and Cursor; Featurebase ships 168-tool MCP. Bar: REST plus signed webhooks, MCP with read, draft, send-with-confirm for conversations and articles; token or OAuth auth; works with Claude Code out of box.
8. **Transparent pricing page with calculator against Featurebase and Intercom.** Why: price is main reason users switch; Featurebase meters confuse. Bar: one number per conversation, no seat fee, free self-host clearly stated.

## Post-MVP, strict priority order

1. **Feature request capture from conversations plus notify-on-ship.** Featurebase moat; agent detects request, links to list, emails requesters when shipped.
2. **GitHub Issues sync (then Linear).** Vibe coders track work in GitHub; closing issue or merging PR triggers customer reply.
3. **Agent custom actions (HTTP tools) plus Stripe lookup.** Billing and account questions dominate solo dev support; Featurebase gates actions to $59 tier.
4. **Importers: Crisp, Intercom, Featurebase, Canny.** Removes switching cost; Featurebase uses same tactic to win.
5. **Public feedback board with voting and roadmap.** Indie users expect public board; reuse request data from item 1.
6. **Changelog / updates in Messenger plus email to subscribers.** Cheap reuse of help center editor; closes loop publicly.
7. **Discord and Slack as support channels.** Indie communities live in Discord; Featurebase gates Slack/Discord to Professional.
8. **React Native / Expo and native mobile Messenger SDK.** Featurebase Messenger is web only; vibe coders ship Expo apps.
9. **Auto-translation of conversations and help center via BYOK.** Featurebase charges Professional tier; Dimmi costs only tokens.
10. **Reports: AI resolution rate, response time, CSAT, cost per resolution.** Proves agent value; needed before teams adopt.
11. **Agent self-improvement: gap detection and drafted articles.** Featurebase strength; raises resolution rate over time.
12. **Outbound: banners, surveys, product tours.** Nice for growth, not core support; Featurebase still meters these.
13. **Workflows, SLAs, round robin, multi-brand.** Team features; first users mostly solo.
14. **CRM sync (HubSpot, Salesforce), SSO, SCIM.** Upmarket only.

## Standout ideas beyond Featurebase

- **Agent reads your repo.** Connect GitHub repo; Dimmi Agent answers from code, README, OpenAPI, migrations, not only docs. Vibe coders often have no docs; code is the docs.
- **Bug report to fix PR.** Customer reports bug; agent gathers repro (URL, console errors, browser, user id), opens GitHub issue, offers "Send to Claude Code / Codex" handoff with full context. Marketing line: "Customer complains, PR appears."
- **Auto-reply on merge.** When PR linked to conversation merges and deploys, Dimmi messages every affected user. Featurebase does this via Linear statuses; Dimmi does it from git, zero configuration.
- **Install-with-AI prompt.** Copy one prompt into Cursor, Claude Code, Lovable, Bolt, v0, Replit; it installs widget, writes JWT endpoint, seeds help center from repo. Plus Dimmi llms.txt and MCP docs server.
- **Support from terminal.** `dimmi` CLI and MCP: list open conversations, reply, close, write articles from Claude Code session. "Do support without leaving your editor."
- **Zero-markup AI receipt.** Show exact token cost per conversation and monthly total; compare live with Featurebase $0.49 and Fin pricing. Supports local models (Ollama) for zero AI cost.
- **Docs and changelog from git.** Generate help articles from README and code; draft changelog from merged PRs and release tags.
- **Error-triggered proactive support.** Sentry / error webhook creates conversation with affected user: "saw checkout fail, fixed, here is credit." Feels magical for small apps.
- **Own your data.** SQLite file, one-command export, no lock-in; host on Railway, Fly, Hetzner, Raspberry Pi. Contrast with Featurebase cloud-only, EU-only.
- **No seat tax, ever.** Unlimited teammates and AI-agent "seats"; invite collaborators, contractors, cofounders free. Featurebase charges per seat and gates Lite seats to $59 tier.
- **Free unbranded self-host.** Featurebase charges $69 per month to remove badge; Dimmi self-host ships unbranded (hosted free tier may keep badge for growth loop).
- **Vibe-coder templates.** Pre-built agent actions for Supabase auth lookup, Stripe subscription status, Clerk user lookup, Resend email.

## Open questions and unverified items

- Founding year: 2020 (blog, [3P]) versus 2022 (about page).
- Current ARR: $54.5K 2024 (Latka [3P]) versus "$2M+" (co-founder LinkedIn [unverified]). 1,600+ customers suggests well above 100K ARR.
- Per-seat switch date 2025-12-04 and whether existing flat-plan customers grandfathered [3P].
- Pricing page fetch showed same prices under monthly and yearly; llms.txt and WishKit say monthly is $37 / $75 / $129. Startup page math ($450 for 6 seats) confirms $75 monthly Professional.
- MCP plan: launch post says Professional; llms.txt says all paid plans; pricing lists API/MCP on Growth.
- Fibi channels: Fibi page lists iOS/Android apps, WhatsApp, Teams; Messenger FAQ says web only; MyAskAI says WhatsApp, Discord, Teams, Slack not live [3P]. Pricing diff tracker confirms Slack live August 2026.
- Lite seat exact permissions.
- Feedback Autopilot accuracy and whether auto-linking needs human approval by default.
- Which LLM powers Fibi; data processing subprocessors.
- No Reddit or Hacker News threads found; sentiment beyond review sites thin.

## Sources

- https://www.featurebase.app/
- https://www.featurebase.app/pricing
- https://www.featurebase.app/llms.txt
- https://www.featurebase.app/about
- https://www.featurebase.app/startups
- https://www.featurebase.app/ai-agent
- https://www.featurebase.app/customer-feedback
- https://www.featurebase.app/help-center
- https://www.featurebase.app/outbound
- https://www.featurebase.app/integrations/mcp
- https://www.featurebase.app/switch
- https://www.featurebase.app/customers
- https://www.featurebase.app/blog/bootstrapping-to-100k-arr
- https://www.featurebase.app/blog/growing-stuggling-side-project-to-2500-mrr
- https://www.featurebase.app/alternative/intercom
- https://feedback.featurebase.app/changelog
- https://feedback.featurebase.app/changelog/introducing-the-support-platform-dizzy
- https://feedback.featurebase.app/changelog/multi-board-linear-and-jira-syncing
- https://feedback.featurebase.app/changelog/introducing-featurebase-mcp-server
- https://feedback.featurebase.app/changelog/introducing-tickets-in-featurebase
- https://feedback.featurebase.app/changelog/linear-integration-official-launch
- https://help.featurebase.app/api-reference
- https://help.featurebase.app/en/articles/6506862-messenger-overview
- https://help.featurebase.app/en/help/articles/2796697-installation-overview
- https://help.featurebase.app/articles/5402549-secure-your-installation-highly-recommended
- https://help.featurebase.app/articles/5466771-seats
- https://help.featurebase.app/articles/6144007-featurebase-mcp-server
- https://help.featurebase.app/articles/8810300-early-stage-startup-program
- https://demoorg.featurebase.app/help/articles/1123439-linear-integration
- https://www.producthunt.com/products/featurebase/reviews
- https://www.producthunt.com/p/featurebase/featurebase-3-0
- https://www.producthunt.com/products/featurebase/makers
- https://g2.com/products/featurebase/reviews/featurebase-review-10450168
- https://ie.trustpilot.com/review/featurebase.app
- https://formbricks.com/blog/featurebase-pricing
- https://www.wishkit.io/blog/featurebase-pricing
- https://myaskai.com/blog/featurebase-complete-guide-2026
- https://myaskai.com/blog/featurebase-pricing-explained
- https://saastruecost.com/guides/featurebase-pricing-decoded/
- https://pricingsaas.com/companies/featurebase/diffs/2026W34
- https://getlatka.com/companies/featurebase.app
- https://www.linkedin.com/in/robi-rohumaa-4238241b4/
- https://www.indiehackers.com/BrunoHiis
- https://fdback.io/blog/canny-vs-featurebase
- https://www.triagely.net/blog/canny-alternatives-founder-buyers-guide
- https://getlaunchlist.com/blog/product-launch-tools
- https://openalternative.co/alternatives/featurebase
