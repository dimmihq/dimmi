# Productlane competitor research

Research date: 2026-11-01. Subject: Productlane (productlane.com), Munich, Germany. Lens: Dimmi first users (vibe coders, solo devs, early adopters).

Tags: [unverified] = single source or conflicting sources. [3P] = third-party or competitor-written claim.

## TL;DR

- Productlane start 2021 as "user feedback and changelog for Linear". Add support inbox 2024 (Productlane 2.0). Now sell "autonomous support for modern companies": AI agent that learns product from GitHub repo, answers email/chat/Slack/Teams, files Linear issues, tells customer when fix ships.
- Five people, Munich, self-described profitable. 800,000+ conversations handled since 2021, "hundreds of software companies". Logos: Clerk, Inngest, Whop, HeyGen, Novu, Speakeasy, Directus, Linearity.
- Pricing per seat: Starter $29, Pro $59, Scale $99 per user per month (yearly), Enterprise custom. Agent includes 25 / 50 / 100 resolutions per month, then $0.79 each. Separate AI credit pool ($5 / $10 / $20) for Copilot, docs generation, changelog, translation; overage at token cost plus margin. 7-day trial. No real free plan.
- Killer workflow: Linear-native. Two-way sync of issues, projects, customers, revenue, comments. Engineers reply to customers from Linear comment. Automatic loop closing (Oct 2026) checks GitHub deploy before telling customer fix shipped.
- Self-updating help center (June 2026): merged commit, closed Linear issue, resolved thread each produce draft. Suggestions panel shows line diff (+173 / -39). Full review view: article left, changes highlighted in place, rail of edit cards right. Accept publishes. MCP lets Claude Code or Codex write whole help center from repo plus past conversations.
- Polish: Linear-clone UI, keyboard-first, 50ms inbox on Rocicorp Zero sync engine. Agent ran on Claude Opus 4.6 since Feb 2026.
- Weak spots for our audience: per-seat plus two AI meters, key features gated to $99 Scale (Teams, multi-language, support portal, SLAs, Copilot, branding removal), Linear lock-in [3P], cloud only (AWS Frankfurt), almost no indie or community chatter (zero HN or Reddit threads found).
- Dimmi opening: same "agent learns product from code, docs update themselves" story, but self-hosted, BYOK with zero AI markup, GitHub Issues instead of Linear as default, and help center as docs-as-code in user repo.

## Company and market status

| Item | Value |
|---|---|
| HQ | Munich, Germany. Hybrid, coworking space (careers page) |
| Founder | Raphael Fleckenstein, Founder and CEO (Product Hunt, LinkedIn, careers). Oscar Franco (@ospfranco) tweet calls it "a product I started with @fleckensteyn" [unverified, single source] |
| Founded | 2021 (careers page). First changelog post 2022-01-10. Public launch 2022-06-21 |
| Funding | Pre-seed, total $825K per VCBacked [3P, unverified]. Nordic9 July 2023: raising pre-seed from German angels [3P]. No priced round found |
| Team | Five people. "We are five people in Munich, profitable." (careers) |
| Scale | 800,000+ conversations since 2021; "hundreds of software companies" (llms.txt) |
| Hosting | AWS Frankfurt, data in Germany on every plan (own blog, Oct 2026). Moved to AWS 2024 |
| Compliance | DPA offered. No SOC 2 claim found [unverified] |
| Product Hunt | 2022-01-13 launch (10 upvotes [unverified, unlabeled]); 2024-10-15 "One platform for user feedback and customer support" (56). Rating 5.0 from 6 reviews |
| G2 | 4.9 from 6 reviews [3P search summary; page blocked] |

Pivot timeline (official changelog):
- 2022-2023: feedback inbox, Linear-synced roadmap, changelog, custom domains ($40 per Linear team then, plus free account), Intercom/HubSpot sync, call recordings. Product discovery phase.
- 2024: shared email (May), Slack Connect app (Jul), Productlane 2.0 widget merging support plus feedback (Oct).
- 2025: support portal (Apr), Docs help center (Jul), AI Chat (Sep), Release Intelligence (Oct), Scale Update plus Zero sync (Dec).
- 2026: multi-language and Cursor auto-assign (Jan), broadcasts plus Opus 4.6 agent (Feb), live chat GA (Mar), Copilot, SLAs, API v2, webhooks (May), MCP server and self-updating help center (Jun), reply within Linear (Jul), connectors and Teams (Aug), agent upgrades, GitHub screen map, Spotlight (Sep), Attio/HubSpot, loop closing, OpenAPI docs (Oct).

Cadence: 19 changelog releases in 2026 (homepage). Roughly one substantial ship every two to three weeks from five-person team.

## Pricing

| Plan | Price per user per month (yearly) | Agent resolutions included | AI credits | Key adds |
|---|---|---|---|---|
| Starter | $29 | 25 | $5 | Shared email, live chat, Productlane Agent, self-updating help center, Linear, custom connectors, feature requests |
| Pro | $59 | 50 | $10 | Shared Slack inbox, custom theme, custom views, reporting, macros |
| Scale | $99 | 100 | $20 | Support portal, multiple portals, multi-language, private portal, SSO (JWT), SLAs, Teams, Copilot, branding removal, Attio/HubSpot |
| Enterprise | Custom | Volume rates | Custom | Invoicing, security review, DPA, white-glove migration |

- Extra resolution: $0.79 flat, only when pay-per-use enabled. Otherwise agent pauses (shows "Paused" badge).
- Resolution definition (docs): customer confirms, or customer silent 24 hours after answer, or teammate marks done and review confirms agent solved it. Real follow-up within 72 hours refunds resolution. Silence-equals-resolved favours vendor.
- AI credits cover everything else (Copilot, docs generation, changelog generation, translation). Overage bills at token cost plus "small margin", capped at user-set limit (default 0).
- Trial: 7 days (own blog). Pricing page llms text says "Start free"; docs mention Free plan with 0 resolutions and one-time credit allowance [unverified, conflicting].
- Startup program: under $5M raised, under 15 employees. Scale with 6 seats: year 1 $65/month (89% off), year 2 $451, year 3 $594. 100 resolutions per month.
- Allowance history: saastruecost (May 2026) listed 50 / 100 / 200 included resolutions [3P, unverified]. Today 25 / 50 / 100. Possible silent halving.
- FeatureOS cites $15 to $79 per user [3P, likely stale]. Third-party listings cite $19 / $39 / $99 [3P, stale].

Solo dev math. One seat Starter $29. Agent resolves 200 conversations per month: 25 free, 175 times $0.79 = $138. Total about $167 per month, plus credit overage for docs generation. Two seats doubles base. Dimmi flat per-conversation fee plus own LLM tokens wins clearly here.

## Feature inventory

### Inbox and channels

- Three-panel inbox, keyboard-first: arrows or number keys between threads, Tab cycles tabs, X multi-select, F adds forwarding address, Cmd+K everywhere, Cmd+J assistant. Open / Snoozed / Done dropdown.
- 50ms average response, largest workspaces load in 3 to 4 seconds (Zero partial sync). Earlier on Replicache; broke at scale because inbox synced every email ever received.
- Channels: shared email (forwarding plus DNS verification, multiple inboxes, per-inbox agent switch), live chat widget, Slack Connect (Pro), Microsoft Teams (Scale), API-created threads, portal messages.
- Conversation timeline: status, assignee, Linear links, AI actions in one log. Agent actions folded when three or more in three minutes.
- Auto-replies with working hours, live chat office hours, emailed transcripts, macros, snooze, undo send, blocklist, company-owner auto-assignment.
- Custom views (revenue, company-size filters), SLAs (Scale), reporting tabs with AI resolution rate, in-app notifications, inbox-wide assistant ("who at risk of churning").

### Widget

- Two script tags, UUID key. JS API: open(view), toggle, openDocs, set, signOut, setContext; events loaded/opened/closed. Views: home, changelog, docs, feedback, AI chat, live chat, roadmap, requests.
- Identity: HS256 JWT (secret `wg_sec_...`, exp max 1 hour, session 12 hours). Multi-tenant via `orgExternalId` / `orgName`.
- Page context for agent ("What can I do here?"). Automatic browser and console capture. Inline screenshots.
- Theming: light/dark/auto, position, every color; subtle question-mark launcher, not loud bubble. Branding removal needs Scale.
- Spotlight mode: AI question bar with voice input for marketing pages and trial users.
- No React/Next.js package documented. No bundle size published.

### Linear integration (deepest in market)

- Issue from thread or link to existing issue/project; AI drafts title, description, team, labels, priority. Optional auto-create per thread. Auto-assign to Cursor so PR may exist before human returns.
- Two-way real-time sync: issues, projects, customers (including revenue, tier, owner), internal comments.
- Reply within Linear: pinned anchor comment mirrors conversation; reply there reaches customer in original channel. Needs seat.
- Customer sees todo / in progress / done plus merged PR in chat. Never moves backward, cancelled hidden. Portal-only titles so internal names never leak.
- Private Linear teams excluded. No Jira, no GitHub Issues.

### Feedback, roadmap, portals, changelog

- Requests portal: upvote, written context, importance, "Get beta access" button. Public roadmap mirrors Linear projects and real statuses.
- Support portal (Scale): customer tracks own requests plus colleague requests by email domain. SSO, custom domain, private portal.
- Release Intelligence: completing Linear project drafts changelog (two paragraphs per project, one bullet per issue, area from label). Widget popup, RSS, email, Slack broadcast to upvoters.
- Critics: votes plus volume, no revenue weighting or theme synthesis [3P Enterpret, AIOProductOS].

### AI agent (Productlane Agent)

- On by default, Starter and up. Answers in customer language, anonymous visitors included; asks email only when human requested.
- Sources: help center, roadmap, changelog, sidebar external links, GitHub-derived agent-only screen map, OpenAPI reference, PostgreSQL whitelisted read-only queries (verified customer own data), email attachments (up to five), custom connectors.
- GitHub use: agent never reads repo directly. Generator writes one agent-only article per customer-facing screen (buttons, fields, settings). Excludes admin screens, credentials, other customers' data. Regenerates on merge or Linear completion.
- Silent mode: replies only with substantive answer. If only acknowledgement possible, stays quiet. Ignores bounces, out-of-office, newsletters.
- Guidance: workspace guidance (tone, policy, escalation), "learn writing style from replies".
- Handoff: "Talk with a human" button; thread lands unread, high priority, open. Agent stops replying once human requested. Cannot disable handoff without disabling agent.
- Connectors (Aug 2026): Stripe, Shopify, product DB, CRM, logs. Customer-facing agents only get connectors explicitly enabled. Write actions (refund) can require human confirmation always.
- Image annotation: agent marks up help center screenshots. Suggest coding-agent routine via MCP to recapture screenshots with demo data.
- Claims: 65.3% resolution rate across customers; 12,499 conversations tracked [3P aitools]. Hero Software: "80 to 90% less work", nine in ten AI drafts sent as written.
- Teammate testing never consumes resolution. Model: Opus 4.6 since Feb 2026 (changelog). Not BYOK.
- Copilot (Scale) drafts replies for human send. Editor assistant (Cmd+J, all plans) writes docs/changelog with codebase context.

### API, webhooks, MCP

- API v2 (May 2026): scoped hashed keys shown once, cursor pagination, error envelope with `request_id`, OpenAPI reference. v1 sunset 2026-11-20 via Deprecation/Sunset headers.
- Webhooks: `thread.created`, `message.received`, `changelog.published`, dozens more. HMAC-SHA256, backoff retries, 200-delivery log, email alert on auto-disable.
- MCP server (Jun 2026): OAuth with per-resource consent, or v2 key as bearer. Scopes filter visible tools. Covers threads, contacts, companies, issues, tags, changelogs, help articles, members. Marketed prompts: draft today's replies; reproduce bug and draft fix PR; find doc gaps; triage; rank requests by revenue; translate docs.
- Also: Raycast and Chrome extensions, llms-full.txt (800 KB), Zendesk / Intercom / Productboard importers, HubSpot and Attio sync.

## Self-updating help center deep dive

Launch: 2026-06-24 changelog. Plan: Starter (all paid plans).

Inputs (three signals):
1. GitHub. Merged commit drafts new article or edits affected one. Connect repository under Settings > Integrations > GitHub.
2. Linear. Closed issue arrives with draft ready. Blog: on Done, Productlane checks whether customer-facing behavior changed; only then drafts.
3. Support threads. Resolved thread becomes new entry, or suggested edit when conversation exposes gap.

Each draft carries provenance: "Each draft arrives with the commit, the issue, or the thread behind it."

Review flow:
- Suggestions panel pinned at top of docs sidebar. Row shows line diff count (+173 / -39).
- Full review view: article left, every proposed change highlighted in place. Right rail lists each edit as card; hover card lights exact location.
- Accept publishes to portal. Each nav group shows counter of pending drafts.
- Version history keeps replaced content; restore from three-dot menu.
- Nothing auto-publishes. Hero Software case study lists "agent publishes updates itself, checked against GitHub code" as planned next step, not shipped.

Cold start:
- MCP route: point Claude Code or Codex at repo with prompt "Review the codebase and every past conversation in Productlane, then write our Help Center." Drafts land in same Suggestions panel.
- Linearity case: MCP read every past ticket plus old knowledge base, rebuilt help center in about two minutes, translated, published.
- Import Markdown or CSV under Settings > Import/Export; Intercom and Zendesk importers bring collections, drafts, translations.

Three knowledge tiers (blog "AI help center"):
- Public articles: customers and agent read.
- Agent reference articles: generated from code, agent reads, customers cannot open.
- Internal articles: excluded from customer agent.

Feedback loop: weekly AI report lists questions agent answered badly; open article with Cmd+K and fix, or link product gap to Linear.

Other help center traits: 47 languages (auto-translation on Scale), docs tabs (Documentation / Guides / API), OpenAPI reference pages with cURL / JavaScript / Python samples and daily breaking-change hold, "Ask anything" bar under every article, copy-article-as-Markdown button, Loom auto-embed, iframes, Google Analytics, custom domain (one per workspace, Starter).

Gaps seen: no docs-as-code (content lives in Productlane DB, not user repo); no PR-based review in GitHub; GitHub trigger tied to Productlane-hosted help center only; screenshot freshness pushed onto user's own coding agent.

## Onboarding and DX for solo dev

Quickstart "Essential Setup" (docs):
1. Connect GitHub, about 2 minutes. Reads front end once, writes agent-only articles.
2. Publish portal, about 5 minutes.
3. Install widget, about 15 minutes.
4. Set up email, about 5 minutes.

Total about 27 minutes to first value, before importing docs. Linear not strictly required in quickstart, but product value collapses without it; quickstart headline still "Set up a customer support system around Slack and Linear."

DX strengths: clean JWT identity with multi-tenant org claims, widget context API, scoped API keys, signed webhooks, OAuth MCP, llms.txt, OpenAPI-driven docs.

DX friction for solo dev:
- 7-day trial. Overage cap defaults to 0, so agent pauses (badge only) once allowance spent.
- Script tags only; no npm or framework components documented.
- Seat price plus two AI meters. Hard to predict. Slack needs Pro; Teams and branding removal need Scale.
- No self-host, no BYOK, no model choice.

Design and UX: Linear-clone visual language, Cmd+K, keyboard everything, real-time sync engine (Replicache then Zero, co-authored blog with Rocicorp's Aaron Boodman). Product Hunt praise centers on "intuitive UI/UX". December 2025 redesign: stronger contrast, refined typography, new empty states. Customer portals run on customer subdomains (feedback.clerk.com, roadmap.speakeasy.com), so product doubles as distribution.

## What users love, what they hate

Love:
- UI polish, Linear-feel, speed (Product Hunt reviews; Brandon Bayer of Flightcontrol).
- Linear integration "seamless"; prioritizing from customer feedback easy (Noah Moscovici).
- Shipping speed: reviewer calls feature velocity "crazy" (Oliver Bahne).
- Agent answer quality: "by far the best" of tools tested (Hero Software, vendor case study).
- Responsive team fixing issues quickly (G2 [3P summary]).

Hate / complaints:
- Linear lock-in: "Extremely niche," only for Linear teams (review quoted by FeatureOS [3P]). Enterpret calls it "Linear-locked" [3P].
- No Jira / Trello (Product Hunt question).
- No free plan, per-seat pricing, AI metered on top [3P aggregators].
- Feedback side shallow: votes not revenue, no theme synthesis [3P Enterpret, AIOProductOS].
- Teams agent replies stay draft until human sends [3P aitools].
- Earlier perception: "more oriented toward feature requests than support" (Product Hunt forum [3P]).
- Tiny review base (about 6 on PH, 6 on G2, 0 on OMR). No HN or Reddit discussion found. Low indie awareness.

## How they market

- Positioning shift from "feedback for Linear" to "autonomous support for modern companies". Headline proof: agent learns from code; loop closing; 65.3% resolution.
- Changelog as main marketing channel: long, opinionated posts ("future of customer support is autonomous"), each ending with deep link into settings.
- SEO content machine since May 2026: "Best Intercom / Zendesk / Canny alternatives", "Best ticketing systems", "Best customer support tool 2026" with dated pricing tables (and own EU-hosting column where they win).
- Thought leadership: resolution rate needs definition, denominator, reopen window; competitor headline rates called incomparable.
- Linear ecosystem listing plus coding-agent narrative (Cursor auto-assign, "reproduce bug and draft fix PR").
- Startup program (89% off year one) seeds early logos. llms.txt for AI-answer visibility.

## Dimmi: must-have at launch

Only true launch blockers for vibe coders and solo devs. Each line: why, then world-class bar.

1. **One-snippet widget with live chat plus AI agent.** Entry point for every conversation. Bar: npm package plus React/Next.js component plus plain script tag; install under 5 minutes; signed identity (HS256 JWT or HMAC) with org claims; page context API; light/dark; tiny bundle with published size; no Dimmi branding lock behind top tier.
2. **Dimmi Agent, BYOK, grounded and honest.** Core reason to switch from Productlane/Intercom. Bar: connect OpenAI/Anthropic/OpenRouter key in under one minute; answers only from docs with citations; silent when only acknowledgement possible; "Talk to human" always available; test mode free; per-conversation token cost visible; hard spend cap.
3. **Email channel.** Solo devs live on support@. Bar: forwarding address works in 2 minutes; correct threading; reply from own domain with DKIM/SPF guided setup; agent ignores bounces, out-of-office, newsletters.
4. **Help center with cold-start generator.** Agent useless without docs; empty docs is top blocker. Bar: Markdown import (folder or git repo); MCP plus CLI command that drafts full help center from codebase into review queue; nothing auto-publishes; custom domain; fast search; "ask AI" bar; copy-as-Markdown and llms.txt; agent-only article visibility.
5. **Review queue with inline diffs.** Trust gate for every AI-written doc. Bar: match Productlane: count of +/- lines, in-place highlights, per-edit accept/reject, provenance link (commit, thread), version history.
6. **Fast keyboard-first inbox plus notifications.** Solo dev triages between coding sessions. Bar: sub-100ms navigation, Cmd+K, snooze, done, internal notes, handoff alert by email and push (or Slack DM) within seconds.
7. **MCP server.** Audience works inside Claude Code and Cursor. Bar: OAuth or key, scoped tools for threads (read, draft reply, send with confirmation), docs CRUD, search; documented prompts like "draft replies for today's threads".
8. **Trivial self-host.** Dimmi core promise; Productlane cannot match. Bar: one container, one env file, under 256 MB RAM idle, SQLite/Postgres choice, one-click deploy buttons (Railway, Fly, Render, Coolify), upgrade with one command.
9. **Transparent flat pricing page with calculator vs per-seat tools.** Pricing is wedge. Bar: show cost for 100 / 1,000 conversations next to Productlane, Featurebase, Intercom with dated sources.

Not at launch: Linear sync, Slack, changelog, portal, roadmap, connectors, translations, reporting beyond basic counts.

## Dimmi: must ship after MVP (strict priority)

1. **Self-updating docs from merged PRs and resolved threads.** Biggest Productlane differentiator; our audience merges PRs daily, docs rot instantly.
2. **GitHub Issues link plus automatic loop closing.** Most solo devs use GitHub Issues not Linear; "your bug got fixed" message is delight moment and retention hook.
3. **Knowledge-gap report.** Weekly list of questions agent failed; cheap to build, feeds item 1, proves agent improving.
4. **Changelog auto-drafted from releases / merged PRs, with widget popup and email broadcast to requesters.** Ships marketing for free; pairs with loop closing.
5. **Agent actions and data connectors.** Stripe lookup, read-only SQL whitelist, HTTP actions with mandatory human confirm for writes. Moves resolution rate from FAQ to account questions (billing is top SaaS ticket).
6. **Discord and Slack channels.** Vibe-coder communities live on Discord; Slack for B2B early customers.
7. **Linear integration.** Second tracker; needed to win Linear-native teams from Productlane directly.
8. **Importers (Intercom, Crisp, Help Scout, Zendesk, Featurebase).** Lower switching cost once Dimmi has proof.
9. **Feature requests portal plus public roadmap.** Nice; Featurebase and Productlane both have; not why solo devs switch.
10. **Auto-translation of help center and agent.** Indie products go global day one; LLM makes it cheap under BYOK.
11. **Honest reporting.** Resolution rate with published definition, reopen window, denominator.
12. **Spotlight-style ask bar, CRM sync, Teams, SLAs, SSO portals.** Conversion and B2B upmarket features; last.

## Standout ideas beyond Productlane

- **Docs-as-code.** Help center lives as Markdown in user's own repo. Dimmi opens GitHub PR when docs need update (from merged PR, closed issue, or resolved thread). Review in GitHub, merge, deploy. Zero lock-in, works with existing review habits. Productlane keeps docs in its DB.
- **Zero-markup BYOK with live cost meter.** Show exact token cost per conversation and per resolution next to "Productlane would have charged $0.79". Support local models (Ollama, LM Studio) on self-host for $0 AI.
- **Bug-to-PR loop without Linear.** Agent confirms bug, gathers console errors, route, app version, repro steps; opens GitHub issue; optional hand to Claude Code / Codex / Copilot agent; customer watches status live in widget; deploy webhook (Vercel, Netlify, Fly, Railway) triggers "fix live" message. Productlane needs Linear plus Cursor.
- **Agent CI.** Replay last N real conversations against new prompt, model, or docs change before deploy; diff answers. "Tests for your support agent." No competitor seen marketing this.
- **Deploy-aware answers.** Agent knows current deployed commit; refuses to describe features not yet shipped; flags docs drift when code and article disagree.
- **Public MCP for end-customers.** Each Dimmi help center exposes read-only MCP plus llms.txt so customers' own AI agents (Cursor, Claude) can query product docs. Developer-tool makers love this.
- **One-command everything.** `npx dimmi init`: detects framework, installs widget, drafts help center from repo, sets up email forwarding, prints deploy button. Target: from zero to agent answering in under 10 minutes (Productlane quickstart is about 27 minutes, docs not included).
- **Open agent prompts and published resolution definition.** Ship prompts in repo; publish how resolution counted (no "silence for 24 hours equals resolved"). Trust play against closed vendors.
- **Screenshot refresher built in.** Headless browser job recaptures help center screenshots with seed data on each deploy. Productlane tells users to build this themselves with MCP.

## Open questions and unverified

- Co-founders beyond Raphael Fleckenstein; Oscar Franco role [unverified].
- Total funding, investors [unverified; only VCBacked $825K pre-seed].
- Free plan status: pricing shows none; docs mention Free with 0 resolutions [unverified, conflicting].
- Resolution allowance cut from 50/100/200 to 25/50/100 [3P, unverified].
- Annual vs monthly price difference: page has toggle, no stated monthly price.
- Whether Linear is hard requirement to sign up (quickstart silent).
- GitHub-triggered drafts: what counts as "affects" article; repo size limits; private repo handling; which LLM.
- Self-updating help center adoption numbers: none published.
- SOC 2 status unknown.
- Spotlight, connectors plan gating not stated.
- Dimmi should verify by trial sign-up (not done; read-only research).

## Sources

1. https://productlane.com
2. https://productlane.com/pricing
3. https://productlane.com/changelog
4. https://productlane.com/changelog/2026-06-25-self-updating-help-center
5. https://productlane.com/help-center
6. https://productlane.com/changelog/2026-06-17-mcp-server
7. https://productlane.com/changelog/2026-05-21-api-v2-and-webhooks
8. https://productlane.com/changelog/2026-10-15-automatic-loop-closing
9. https://productlane.com/changelog/2026-09-03-customer-support-agent-upgrades
10. https://productlane.com/changelog/2026-09-14-support-agent-github-context
11. https://productlane.com/changelog/2026-08-07-support-agent-connectors
12. https://productlane.com/changelog/2026-07-28-reply-within-linear
13. https://productlane.com/changelog/2026-10-27-openapi-integration
14. https://productlane.com/changelog/2026-03-12-live-chat
15. https://productlane.com/changelog/2026-02-17-support-inbox
16. https://productlane.com/changelog/2026-05-04-support-copilot
17. https://productlane.com/changelog/2026-01-27-cursor-integration
18. https://productlane.com/changelog/2026-01-14-multi-language
19. https://productlane.com/changelog/2026-02-05-email-and-slack-broadcasts
20. https://productlane.com/changelog/2026-03-27-conversation-timeline
21. https://productlane.com/changelog/2025-12-08-scale-update
22. https://productlane.com/changelog/2025-10-15-release-intelligence
23. https://productlane.com/changelog/2025-09-24-ai-chat
24. https://productlane.com/changelog/2025-07-08-docs
25. https://productlane.com/changelog/2025-04-30-customer-support-portal
26. https://productlane.com/changelog/2022-01-10-announcing-productlane
27. https://productlane.com/llms-full.txt
28. https://productlane.com/docs
29. https://productlane.com/docs/get-started/quickstart
30. https://productlane.com/docs/setup/widget
31. https://productlane.com/docs/ai/ai-agent
32. https://hello.productlane.com/docs/integrations/linear
33. https://productlane.com/docs/administration/startup-program
34. https://productlane.com/startup
35. https://productlane.com/careers
36. https://productlane.com/blog
37. https://productlane.com/blog/best-customer-support-tool-2026
38. https://productlane.com/blog/ai-support-agent
39. https://productlane.com/blog/ai-help-center
40. https://productlane.com/blog/ai-ticketing-system
41. https://productlane.com/blog/zero-migration (via mirror search results)
42. https://productlane.com/customer/linearity
43. https://productlane.com/customer/hero-software
44. https://productlane.com/migrate/intercom
45. https://linear.app/integrations/productlane
46. https://www.producthunt.com/products/productlane/reviews
47. https://www.producthunt.com/products/productlane/launches
48. https://www.producthunt.com/products/productlane/launches/productlane-3
49. https://www.g2.com/products/productlane-productlane/reviews (blocked; search summary only)
50. https://www.enterpret.com/guides/the-6-best-productlane-alternatives-2026 [3P]
51. https://featureos.com/alternatives/productlane [3P]
52. https://aioproductos.com/reviews/productlane [3P]
53. https://aitools.fyi/productlane [3P]
54. https://hellostack.io/customer-service/productlane [3P]
55. https://saastruecost.com/compare/featurebase-vs-productlane/ [3P]
56. https://omr.com/en/reviews/product/productlane [3P]
57. https://www.vcbacked.co/company/productlane [3P]
58. https://nordic9.com/news/productlane-to-raise-pre-seed-funding-with-angel-investors-in-germany/ [3P]
59. https://x.com/ospfranco/status/1846227174841360806
60. https://www.producthunt.com/@fleckensteyn
