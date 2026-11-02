# Fin (formerly Intercom) competitor research

Research date: 2026-11-01. Subject: Fin (fin.ai), company formerly named Intercom, now Salesforce subsidiary. Covers Fin AI agent and Intercom helpdesk product. Lens: Dimmi first users (vibe coders, solo devs, early adopters).

Tags: [unverified] = single source or conflicting sources. [3P] = third-party or competitor-written claim.

## TL;DR

- Intercom company renamed Fin on 2026-05-12. "Intercom" survive only as helpdesk product name ("Intercom 2" rebuild). Salesforce signed deal 2026-06-15, closed 2026-09-10, about $3.6B. Fin now sit inside Salesforce AI Labs, positioned next to Agentforce.
- Scale: company ARR over $400M, Fin ARR near $100M, 30,000+ customers, ~1,400 staff [3P, SalesforceBen]. Fin claim 76% average resolution rate, 2M resolutions per week, own model family "Apex".
- Pricing: seat ($29 / $85 / $132 per seat per month annual; Essential A/B test show $19) plus $0.99 per Fin "outcome", plus $9.99 per sales qualification. Outcome replaced "resolution" on 2026-03-12 and widened billable set (Procedure handoffs now billed). Silence after 24 hours count as billable "assumed resolution".
- Fin already court our audience: Fin CLI (`npx @intercom/cli`, April 2026) built for Claude Code, Cursor, Codex. Demo: Fin live on site in under 2 minutes. Remote MCP server (14 tools), Fin Agent API MCP (5 tools). Weak spots: US region only for CLI, tokens expire after ~14 days, no OAuth browser flow.
- Top complaints: unpredictable bill, bill grow as bot improve, upsell pressure, steep learning curve, slow vendor support, Fin trap customers away from humans (Anthropic billing story, SF Standard, 2026-10-15).
- Dimmi opening: BYOK means zero AI markup and no incentive to fake resolutions. Self-host plus open source answer Salesforce lock-in fear. Must beat Fin CLI on agent-native setup, not only on price.

## Company and market status

| Item | Value |
|---|---|
| Founded | 2011, San Francisco and Dublin, founders Eoghan McCabe, Des Traynor, Ciaran Lee, David Barrett |
| Rename | 2026-05-12, "Today Intercom becomes Fin" post by McCabe. All 1,400 employees now employed by Fin |
| Product naming | Company = Fin. AI agent = Fin. Helpdesk = Intercom (rebuilt as "Intercom 2", launched late April/early May 2026) |
| Acquirer | Salesforce. Agreement 2026-06-15, announced 2026-06-16, closed 2026-09-10 (Salesforce investor release). Price ~$3.6B |
| Post-close structure | Fin "will continue to serve its customers" inside Salesforce AI Labs. McCabe stays CEO, Traynor leads R&D [3P] |
| Revenue | Company ARR over $400M; Fin ARR approaching $100M [3P, SalesforceBen] |
| Customers | 30,000+ companies (Salesforce release); Fin page claim 12,000+ Fin customers |
| Debt | $250M venture debt from Hercules Capital, March 2026 (Wikipedia) [unverified] |
| Model | Apex 1.0 (March 2026) generate every final answer; Apex 2.0 claim +8.6% confirmed resolutions; "Apex for Procedures" claim +15% vs Sonnet 5 (fin.ai) |

Salesforce fallout:
- Sources disagree on close. Older pages (Plain, Richpanel) say pending, close early 2027. Salesforce investor release confirm close 2026-09-10. Treat as closed.
- On 2026-09-11 Salesforce listed Fin as separate generally available agent inside Agentforce lineup [3P, datafloq].
- No public roadmap for: Fin on non-Salesforce helpdesks (Zendesk, HubSpot, Freshdesk), pricing protection, data handling. Competitors (Plain, Richpanel, Kommunicate, SleekFlow, Macha, Aissist) all publish "alternatives after Salesforce acquisition" pages [3P]. Signal: churn-bait market exists.
- Fears voiced [3P, Plain]: re-platforming pain once folded into Agentforce, roadmap shift toward enterprise, "implementing anything in Salesforce is a nightmare", metered pricing rarely get kinder post-acquisition.
- No layoff reports found. No Hacker News thread found.
- Regulatory angle: California AB 1609 (signed ~September 2026) require businesses over $500M revenue to connect customer to human within 15 minutes, starting 2027. Bill author said rule would apply to Anthropic and its contracted Fin chatbot (SF Standard).

## Pricing

### Intercom helpdesk seats (intercom.com/pricing, 2026-11-01)

| Plan | Annual per seat per month | Monthly | Lite seats | Key adds |
|---|---|---|---|---|
| Essential | $29 list; page show $19 promo "New customers save 35%" | $39 | none | Fin, Messenger, shared inbox, tickets, pre-built reports, public help center |
| Advanced | $85 | $99 | 20 free | multiple team inboxes, workflow builder, round robin, private and multilingual help center |
| Expert | $132 | $139 | 50 free | SSO, HIPAA, SLAs, multibrand Messenger and help center |

- Essential price flip-flop: 2026-09-21 calculator and fin.ai moved Essential from $19 back to $29, but A/B test `team-self-serve-essential-price-reduction` still show $19 to some visitors [3P, usagepricing.com]. Our fetch saw $19 on 2026-11-01. Price depend on who look.
- Minimum one Full seat. Lite seats cannot send messages.

### Fin outcomes

- $0.99 per outcome. One outcome max per conversation.
- Billable outcomes: resolution (confirmed or assumed), Procedure handoff, disqualification, self-serve routing. Qualification (Fin for Sales) $9.99.
- Assumed resolution = customer leave without asking more help; 24 hours inactive after Fin's last answer count. Silence bill.
- Not billed: customer ask for human, failed Procedure, default escalation (frustration detected), unanswered clarifying question, greeting-only reply. Reopened resolution reversed.
- Metric change 2026-03-12: "Resolutions" became "Outcomes". Price unchanged, billable set wider. Procedure handoff (bot run configured Procedure then hand to human) now cost $0.99 even though human finish work.
- Fin on another helpdesk (Salesforce, Zendesk, HubSpot, Freshdesk, generic): $0.99 per outcome, 50-outcome monthly minimum, "$49/month base plan including 50 resolutions" per help article. Unlimited teammates, no seat fee.
- Fin Voice: custom pricing; rebuilt pricing page briefly showed $1.99 per voice outcome [unverified].

### Add-ons

| Add-on | Price |
|---|---|
| Copilot (agent assist) | $29 per agent per month annual, $35 monthly; 10 free Copilot conversations per agent per month |
| Pro (Operator, Workforce Management, conversation analysis) | from $100 per month (some captures $99); 6,000 credits; extra credit $0.01 |
| Proactive Support Plus (Series, Checklists, News, A/B tests, webhooks in Series) | $99 per month, 500 messages included, then per message |
| Usage-based | email campaigns, SMS, WhatsApp, phone, Posts, Product Tours, Surveys, mobile push and carousels billed per message; rates not shown publicly |

Free on all plans: live chat, support email, in-app chats, banners, tooltips.

### Trial and startup deals

- 14-day trial, no card, unlimited Fin outcomes.
- Early Stage program: under $10M raised, fewer than 15 employees, new customer. Advanced plan plus Proactive Support Plus plus Pro, 6 full seats, 6 Copilot seats, 20 Lite seats. List $903/month. Year 1 93% off ($65/month), year 2 50% off ($451.50), year 3 25% off ($677.25). Free Fin outcomes 315/month year 1, then 165, then 90. Cliff: year 2 bill jump ~7x.
- Billing USD only. ACH/wire via sales.

### Real bills [3P]

- 2 agents, 300 AI conversations per month: $206 to $266 per month with Essential seats (dev.to, competitor author).
- 8 agents Advanced, 2,100 outcomes: seats $680, Fin $2,079, add-ons push to ~$3,090 per month (Macha).
- Solo dev, Fin standalone floor: ~$49.50 per month before any human seat. Plus one Essential seat: ~$78.50.
- Viral claim "Intercom bill jumped from $4k to $9k/month" cited across competitor blogs [unverified]. Another: bot cost "~€75 to ~€700 a month" as volume grew [3P, Plain].

## Feature inventory

### Fin AI agent

| Area | Features |
|---|---|
| Knowledge | Help center articles, website sync (with images), PDFs, snippets, Notion/Confluence/Guru sync or import, Content Guidance (pick source per question type), reads customer PDF/HEIC attachments, image answers (up to 3 images) |
| Behavior control | Guidance (tone, policy rules, versioned, office-hours scheduling), Personalisation (name, email, location), Fin Memory across channels (July 2026), Identities per brand on email |
| Procedures | Multi-step natural-language SOPs with deterministic if/else, Python code blocks, data connectors, Ask-a-teammate approval, Wait step, Wait-for-Webhook, hand off to workflow, Proactive Procedures (triggered by click or system signal), AI-generated from description, versioning, reporting |
| Actions | Data connectors (HTTP), automatic Salesforce connectors, Zapier MCP connector (thousands of apps), Shopify actions |
| Testing | Simulations (AI-suggested scenarios, mock connector output, pass/fail, run-all before publish; monthly cap), Fin over Email preview, Voice simulations |
| Quality | Monitors: custom scorecards, AI scoring, review queue, Issues, QA permissions, phone call scoring. CX Score: AI 1-5 score on every conversation, benchmarking percentile vs peers |
| Insights | Topic trends, Incident Detection (flags emerging incidents within minutes), Escalation Reporting, Content recommendations, revenue attribution (ecommerce) |
| Operator | Back-office agent that tunes Fin, proposes content fixes, applies config changes; Connections to Slack and GitHub (November 2026); credit-billed |
| Channels | Messenger, email, WhatsApp, SMS, Slack, Discord, Telegram, social, phone (Fin Voice 2 on Apex Flash) |
| Verticals | Fin for Sales (qualify, book via Chili Piper and HubSpot Meetings, Marketo), Fin for Ecommerce (product search, Shopify subscriptions) |
| Deploy targets | Intercom, Salesforce Service Cloud, Zendesk, HubSpot, Freshdesk, "Generic Platforms" via Messenger |
| Languages | "dozens" (no count given) |
| Vendor claims | 76% average resolution, ~0.1% hallucination rate, 99.8% uptime SLA, 226 updates shipped in 2025 |

### Intercom helpdesk

| Area | Features |
|---|---|
| Messenger (web) | JS snippet, npm `@intercom/messenger-js-sdk`, anonymous visitors via cookie, Messenger Home, help search, brand font matching (October 2026), in-conversation search, multibrand (Expert) |
| Mobile SDKs | iOS (15+), Android (API 23+), React Native wrapper with Expo config plugin (v10.2.0, May 2026). Flutter not confirmed official [unverified] |
| Inbox | Shared and team inboxes, views from saved search, follow conversations, flexible snooze lists, macros with usage analytics, round robin, skill-based routing, spam view, Copilot, AI translation |
| Tickets | Customer tickets (customer-visible), back-office tickets (internal, linked to conversation), tracker tickets (one bug, many customers). Customer portal with full-page views and Insights tab |
| Help center | Public, private, multilingual, multi-brand, content scheduling, audience targeting, redesign July 2026 |
| Workflows | Visual builder (Advanced+): triggers, branches, assignment, bots, SLAs (Expert), Fin closure trigger |
| Outbound | Series (multi-step campaigns), Product Tours, Checklists, News, Posts, Banners, Tooltips, Surveys, mobile carousels, email campaigns, SMS |
| Reporting | Pre-built reports (Essential), custom reports, CX Score, Workforce Management (forecasting, scheduling; moved to Pro add-on October 2026) |
| Phone | Native phone, skill-based routing, Fin Voice |
| Integrations | App Store, 250+ apps [3P, unverified count]; Slack, Jira, GitHub, Shopify, Salesforce, HubSpot, Segment, Zapier |
| Security | Messenger JWT (HS256), SSO, HIPAA (Expert), data hosting US/EU/AU |

### Developer platform

| Area | Detail |
|---|---|
| REST API | Versioned (2.14 current, plus "Unstable" version for preview). OpenAPI spec published. Official Node/TypeScript SDK `intercom-client` generated with Fern; reported to lag API versions [3P] |
| Rate limits | 10,000 calls per minute per app (private apps share per workspace), 25,000 per minute per workspace, enforced in 10-second windows (limit divided by 6). 429 `rate_limit_exceeded`; headers sometimes missing on 429 |
| Webhooks | Subscriptions managed only in Developer Hub UI, not API. Signature HMAC-SHA1 in `X-Hub-Signature` keyed on app client secret, no timestamp so no replay protection. ~5s timeout, one retry after 1 minute, suspension after 7 days of errors [3P, Hookdeck, Macha] |
| Remote MCP server | `mcp.intercom.com/mcp` (US), `mcp.eu.intercom.com/mcp` (EU), AU not supported. 14 tools: search, fetch, conversations, contacts, companies, articles (create/update), add_internal_note. Cannot send customer-visible reply. 5 Operator tools via OAuth only |
| Fin Agent API | Fin as node in multi-agent systems. Enterprise only via account team (fin.ai update). MCP endpoint `api.intercom.io/fin/mcp`: list_fin_capabilities, ask_fin, get_fin_response, reply_to_fin_conversation, run_fin_procedure |
| Fin CLI | `@intercom/cli`. `intercom setup` create workspace, install Messenger, fill help center from `--content-dir ./docs`. Subcommands: articles, contacts, conversations, help-center, messenger, fin (procedures, guidance, audiences, data connectors, simulations, workflows), raw `intercom api`. `--json`, `--jq`, `--plan`, `--dry-run`, no prompts. Limits: US region only, tokens expire ~14 days with no refresh, no OAuth browser flow, Node 18+ |

## Onboarding and DX for solo dev

Path A: classic web install.
1. Sign up, 14-day trial, no card.
2. Enable Messenger for Web and "enable user traffic" first. Skip this and requests fail.
3. Paste snippet before `</body>` or `npm i @intercom/messenger-js-sdk`, call `Intercom({ app_id })`. Wrong app ID give no error; widget silent.
4. Set regional `api_base` for EU/AU. Wrong region silent fail.
5. SPA: call `Intercom('update')` on every route change, `shutdown()` on logout, then delete `intercom-` cookies or previous user's history leak on shared device.
6. Messenger security: server sign JWT HS256 with `user_id` claim, pass `intercom_user_jwt` in boot. Company data must nest under `company` object; flat claims ignored. Invalid JWT block Messenger even when enforcement off. Old `user_hash` HMAC deprecated. Logged-in conversation history capped at 14 days unless security enabled.
7. Train Fin: sync site or help center, set audiences, preview, go live.

Estimate time-to-first-message: 10 to 30 minutes for widget, 1 to 3 hours for secure identity plus Fin tuned [unverified, our estimate from docs].

Path B: Fin CLI with coding agent. Demo shows Fin live on website in under 2 minutes from Claude Code (createwith.com, April 2026) [3P]. Real friction: US-only workspaces, 14-day token expiry, `auth bootstrap` and `auth signup` use internal endpoints that may need WAF exceptions.

DX verdict:
- Good: mature versioned API, OpenAPI spec, generous rate limits, React Native plus Expo plugin, agent-ready CLI, two MCP servers, Developer Hub.
- Bad: many silent-failure modes in install, SHA1 webhooks with single retry, webhook subscriptions not API-manageable, region splits (US/EU/AU) leak into every config, SDK lag, MCP cannot reply to customers, Agent API locked to Enterprise.

## What users love

- Polished, clean UI; Messenger look premium. "the interface is clean" (Capterra via Macha [3P]).
- Fin resolution quality high relative to rivals; time-saving most common G2 praise theme (198 reviews tagged) [3P].
- All-in-one: chat, email, tickets, help center, outbound, phone in one product.
- Shipping speed: 226 updates in 2025, 191 by June 2026.
- Ratings: G2 ~4.5 (3,500+ reviews), Capterra ~4.5 (~1,100) [3P]. Note: G2 profile heavy on incentivized seller-invite reviews [3P, unverified].

## What users hate

- Price, and pricing complexity. Capterra pricing theme 64% negative [3P]. "pricing was a little steep, especially as a small business" [3P].
- Unpredictable bill. Outcome model make bill grow as bot improve; assumed resolutions bill silence; Procedure handoffs billed though human finish job.
- Upsells: G2 tag "too expensive, especially with frequent upsells" (80 reviews) [3P]. Workforce Management moved behind Pro add-on after beta.
- Fin limits: needs specific phrasing, outdated info (130 G2 reviews), inaccuracy (74) [3P].
- Steep learning curve (115 G2 mentions), setup complexity (85) [3P].
- Vendor support slow, frustrating handoffs (88 G2 complaints) [3P]. Trustpilot reportedly ~1.9 or 3.3 depending on source [unverified].
- Customers trapped with bot. SF Standard: Anthropic customers looped through Fin for weeks on billing errors; Fin "empathized" and closed case, said it could not transfer to human. Only 2 of 6 interviewed reached person.
- Startup cliff: Early Stage deal jump from $65 to $451.50 per month in year 2.
- Region gaps: CLI and MCP US/EU only; AU excluded.
- Indie hackers: per-seat plus per-resolution fees "grow faster than founder revenue"; under ~$1,000 MRR support tool "look like a competitor" [3P, devtoolpicks]. They pick Crisp, Tawk.to, Chatwoot, Help Scout instead.

## Dimmi must-have at launch

Only items without which vibe coder will not switch or will not stay.

1. **Web Messenger widget, installable in under 2 minutes.** First impression; Fin CLI already demo 2 minutes. Bar: one script tag or `npm i`, React/Next/Vue/Svelte helpers, automatic SPA route handling, loud console error on wrong key or domain, launcher under 30KB gzipped with lazy-loaded panel, attachments, dark mode, mobile responsive, anonymous visitor plus email capture when no agent online.
2. **Secure identity by default (JWT).** Intercom leak history on shared devices and silent-fail on bad JWT; vibe coders ship insecure by default. Bar: HS256 JWT, copy-paste server helper for Node/Next/Python/Go/Rails, `dimmi.logout()` clear everything, test page that validates token and explains errors in plain words.
3. **Shared inbox that feels instant.** Core job. Bar: real-time, assign, close, snooze, internal notes, saved replies, keyboard shortcuts, search, conversation context sidebar (user attributes, page URL, browser), works well as PWA on phone.
4. **Email channel both ways.** Visitor leave page before answer; reply must reach them. Bar: offline replies sent by email, customer reply by email threads back, forwarding address for support@ inbox, custom sending domain with DKIM/SPF guide (SES adapter default).
5. **Notifications where solo dev lives.** Solo dev not sit in inbox. Bar: email, browser push, mobile PWA push, Slack and Discord notify with reply-from-thread.
6. **Dimmi Agent (BYOK) answering from docs.** AI first answer is table stakes in 2026. Bar: connect OpenAI/Anthropic/OpenRouter/Ollama key in 1 step, ingest URL crawl plus Markdown folder plus pasted text, cited answers, refuse when not grounded, clean handoff with summary, "talk to human" always honored, playground to test questions before live, per-conversation token cost shown.
7. **Agent-native setup: CLI, MCP, API.** Audience build with Claude Code and Cursor; Fin already ship CLI. Bar: `npx dimmi init` detect framework and wire widget plus JWT in codebase; works for any region and self-host URL; OAuth device flow, non-expiring scoped tokens; `--json`, `--dry-run`; MCP server that can read AND reply to conversations (Fin's cannot); REST API with OpenAPI spec; `llms.txt` docs.
8. **Webhooks done right.** Devs wire support into own app. Bar: HMAC-SHA256 with timestamp, retries with exponential backoff over 24 hours, delivery log with redeliver button, subscriptions via API and UI.
9. **One-command self-host plus hosted cloud.** Core promise; Salesforce lock-in fear. Bar: single Docker image or binary, SQLite or Postgres, runs on $5 VPS in under 256MB RAM, upgrade by pulling image, full data export.
10. **Public flat per-conversation price with calculator.** Pricing is top Fin complaint. Bar: one number on page, no seats, no add-ons, no promo A/B tests, hard spend cap, bill same whether bot or human answered.

## Post-MVP, strict priority order

1. **Intercom importer plus `window.Intercom` compatibility shim.** Salesforce churn window open now; lowest-friction switch win deals.
2. **Public help center from Markdown/Git.** Grounds agent, cuts tickets, SEO; Fin CLI already fill help center from `./docs`.
3. **Agent actions (HTTP tools, OpenAPI import, MCP client).** Lookup order or reset password is where AI earn keep; Procedures is Fin's main moat.
4. **Simulations and evals in CI.** Devs trust tests; `dimmi test` run question set against agent on every docs change.
5. **React Native/Expo SDK, then iOS/Android.** Many vibe-coded apps are mobile; Intercom has Expo plugin.
6. **Basic reporting plus AI conversation score.** Founders need volume, response time, bot handoff rate, top topics; CX Score style scoring cheap with BYOK.
7. **Tickets plus GitHub/Linear issue linking.** Bug reports are main support load for dev products; tracker ticket pattern (one bug, many users, notify on fix).
8. **Rules and automation (office hours, auto-assign, auto-tags, SLA timers).** Needed once second teammate joins.
9. **More channels: Discord, Slack Connect, WhatsApp, Telegram.** Dev communities live in Discord; Fin added Discord and Telegram 2026.
10. **Agent assist for humans (draft reply, summarize, translate).** Fin charges $29 to $35 per agent; BYOK make it free.
11. **In-app announcements and changelog widget.** Lightweight News/Banners; full Series/Tours later.
12. **QA monitors, outbound campaigns, voice.** Enterprise-grade; low value for first users.

## Standout ideas beyond Fin

- **"No incentive to trap your customers" pledge.** Fin earns $0.99 when bot "resolves", including silence. Dimmi charges same per conversation whether bot or human answers. Market: "We do not profit when your bot deflects." Pair with always-visible human button and AB 1609-style 15-minute human SLA toggle.
- **Zero AI markup, visible token cost.** Show exact LLM cost per conversation and per month next to Fin-equivalent price ("this month Fin would have charged $312; your tokens cost $4.10").
- **Switch from Intercom in one command.** `npx dimmi migrate intercom` imports contacts, conversations, articles; shim keeps `Intercom('boot')` calls working. Campaign: "Leave Salesforce before renewal."
- **Install by prompt.** Ship Claude Code skill, Cursor rule, and MCP so "add Dimmi support chat with secure login" works first try. Publish recorded benchmark: time from prompt to first message vs Fin CLI.
- **Developer context on every conversation.** Widget auto-captures console errors, failed network calls, URL, app version, browser; optional links to Sentry/PostHog session. Solo dev fix bug from one screen. Fin has nothing comparable.
- **Docs-as-code knowledge.** Help center and agent knowledge synced from repo `/docs`; PR preview shows how agent answers change; evals in GitHub Action.
- **Local model mode.** Self-host plus Ollama equals fully private AI support, zero third-party data flow. Strong for EU and privacy-minded founders; Fin MCP and CLI exclude AU and lean US.
- **Lightweight widget badge.** Publish widget size and Lighthouse impact; "lighter than Intercom" measurable claim.
- **Reply from anywhere.** Answer customers from Slack, Discord, Telegram, or email thread without opening dashboard.
- **Open-source trust.** Code public, data portable, no acquisition rug-pull. Contrast: Intercom changed pricing metric, flip-flopped Essential price, moved beta features behind add-ons, then sold to Salesforce within one year.

## Open questions and unverified items

- Essential seat real price: $19 promo or $29 list. A/B test active; varies by visitor.
- Fin Voice $1.99 per outcome: single source, sales-only.
- Future of Fin on Zendesk/HubSpot/Freshdesk under Salesforce: not stated publicly.
- Intercom 2 exact scope and launch date: secondary sources only ("60+ updates", "6x faster inbox" [unverified]).
- Webhook retry policy and SHA1 signature: third-party guides; official page silent.
- G2/Capterra/Trustpilot counts: G2 and Capterra blocked fetch; numbers from third-party aggregators.
- Fin "~0.1% hallucination rate" and "76% resolution": vendor claims, no independent test.
- Early Stage free outcomes 300 vs 315: official article conflicts with itself.
- Official Flutter SDK existence: not confirmed.
- Fin Agent API plan gate: fin.ai update says Enterprise; help article says only API key needed.
- Whether "Procedure handoff" and "self-serve routing" outcomes apply on non-Intercom helpdesks: help article covers resolution and Procedure handoff only.

## Sources

1. https://fin.ai/pricing
2. https://www.intercom.com/pricing
3. https://fin.ai/help/en/articles/13975800-fin-pricing-outcomes
4. https://fin.ai/updates/fin-pricing-metric
5. https://www.intercom.com/blog/from-resolutions-to-outcomes-evolving-how-fin-delivers-value/
6. https://fin.ai/updates
7. https://www.intercom.com/changes
8. https://fin.ai/
9. https://ideas.fin.ai/p/today-intercom-becomes-fin
10. https://en.wikipedia.org/wiki/Fin_(company)
11. https://investor.salesforce.com/news/news-details/2026/Salesforce-Completes-Acquisition-of-Fin/default.aspx
12. https://www.salesforceben.com/salesforce-acquires-fin-formerly-intercom-adding-30k-ai-customers/
13. https://datafloq.com/salesforce-closes-3-6-billion-fin-deal-adding-a-fast-deploy-ai-agent-to-agentforce/
14. https://www.cxtoday.com/contact-center/intercom-rebrands-to-fin/
15. https://www.plain.com/blog/intercom-alternatives-salesforce-acquisition-2026 [3P]
16. https://www.richpanel.com/learn/fin-salesforce-acquisition-what-it-means-for-support-teams [3P]
17. https://www.usagepricing.com/blueprint/activity/intercom-2026-09-21-essential-seat-split-closes [3P]
18. https://www.intercom.com/help/en/articles/15359850-early-stage-program
19. https://www.intercom.com/help/en/articles/8344190-pricing-faqs
20. https://fin.ai/updates/fin-cli
21. https://www.intercom.com/help/en/articles/14497543-getting-started-with-the-fin-cli
22. https://www.createwith.com/tool/intercom/updates/intercom-launches-cli-tool-for-claude-code-to-deploy-fin-in-minutes [3P]
23. https://developers.intercom.com/docs/guides/mcp
24. https://www.intercom.com/help/en/articles/15481203-fin-agent-api-mcp-server
25. https://fin.ai/updates/fin-agent-api
26. https://developers.intercom.com/installing-intercom/web/installation
27. https://www.intercom.com/help/en/articles/10589769-authenticating-users-in-the-messenger-with-json-web-tokens-jwts
28. https://developers.intercom.com/docs/references/rest-api/errors/rate-limiting
29. https://developers.intercom.com/docs/webhooks
30. https://hookdeck.com/webhooks/platforms/guide-to-intercom-webhooks-features-and-best-practices [3P]
31. https://www.getmacha.com/blog/intercom-webhooks-guide [3P]
32. https://developers.intercom.com/intercom-api-reference/reference/
33. https://www.npmjs.com/package/@intercom/messenger-js-sdk
34. https://developers.intercom.com/installing-intercom/docs/react-native-installation
35. https://www.intercom.com/help/en/articles/6436600-tickets-explained
36. https://fin.ai/help/en/articles/13975974-understand-customer-experience-at-scale-with-the-cx-score
37. https://fin.ai/help/en/articles/12628771-run-simulations-for-fin-procedures
38. https://fin.ai/help/en/articles/13976234-sync-or-import-internal-articles-from-external-sources
39. https://createwith.com/tool/intercom/updates/intercom-unveils-intercom-2-with-60-updates-for-ai-first-customer-support [3P]
40. https://sfstandard.com/2026/10/15/anthropic-customer-service-human/
41. https://dev.to/omar_bni_f6856a8bb0e021e9/intercom-fin-pricing-in-2026-the-real-math-behind-099-62j [3P]
42. https://www.getmacha.com/blog/is-intercom-worth-it [3P]
43. https://www.getmacha.com/blog/intercom-pricing-explained [3P]
44. https://devtoolpicks.com/blog/best-intercom-alternatives-indie-hackers-2026 [3P]
45. https://www.g2.com/products/fin/reviews (blocked; themes via search snippets) [3P]
46. https://www.capterra.com/p/134347/Intercom/reviews/ (blocked; themes via aggregators) [3P]
47. https://www.intercom.com/blog/reducing-intercom-messenger-bundle-size/
48. https://www.bbb.org/us/il/chicago/profile/artificial-intelligence/intercom-inc-0654-1000118625/complaints
