# Competitor: HelpDesk (helpdesk.com) and Text suite

Research date: 2026-11-01. Style: caveman. Tags: [unverified] = single source or not confirmed on primary page. [3P] = third-party or competitor-written claim.

## TL;DR

- Identification confirmed. HelpDesk = ticketing product at helpdesk.com by Text S.A. (formerly LiveChat Software S.A., Wrocław, Warsaw exchange ticker TXT). Launched June 2019. No other notable "HelpDesk" product took over name in 2026; "HelpDesk+ for Microsoft Teams" (Appfire) and Jitbit Helpdesk exist but unrelated, smaller.
- HelpDesk no longer standalone product in practice. Pricing pages on helpdesk.com, livechat.com, chatbot.com all show same "Text" plans: Essential $19, Growth $79 per user per month (annual). HelpDesk now "ticketing platform inside Text.com". Old help center still shows Team $29 / Business $50 per agent. Rebrand mid-flight; docs contradict pricing page.
- Model: per seat plus meters. AI resolutions $0.99 each (50-pack $49.50, auto-refill). Workflow executions capped. API calls metered, even Personal Access Token calls. Essential caps brutal: 60-day history, 3 workflow rules, 3 ticketing rules, 5 tags, 10 AI resolutions per month.
- Company: flat, mature, highly profitable. FY2025/26 revenue PLN 329.1M (-7.1%), net profit PLN 116.6M. ARR $89.52M June 2026 (record, from killing LiveChat price grandfathering). Sales team disbanded, product-led growth pivot June 2026. LiveChat = 83.7% of revenue; HelpDesk plus KnowledgeBase only 7.5%.
- Strength: clean simple UI, collision detection, decent no-code Workflows with HTTP step, hosted MCP server (read-only), OpenAPI REST API, 200+ app marketplace, SOC 2 Type 2.
- Weakness for our audience: seat pricing plus many meters, paywalled reporting, weak search, auth tied to LiveChat accounts, metered API, no BYOK, no self-host, billing and refund complaints, brand confusion across 6 domains.
- Dimmi angle: everything Text meters (AI resolutions, API calls, workflow runs, history, tags, rules) Dimmi gives unlimited. BYOK AI at token cost vs $0.99 per resolution. Self-host plus open source plus developer-first API, MCP with write tools.

## Company and market status

**Identity.** Text S.A. (US subsidiary Text, Inc., Boston). Founded 2002 as LiveChat. Renamed Text September 2023. Products: Text (suite, formerly "Text App"), LiveChat, ChatBot, HelpDesk, KnowledgeBase, OpenWidget. 35,000+ customers claimed. Around 268 employees [unverified, Wikipedia].

**HelpDesk history.** Launched June 6, 2019 (Product Hunt). LiveChat plus HelpDesk integration May 2020. LiveChat native ticketing retired January 2025; LiveChat tickets now migrate into HelpDesk via marketplace integration (tickets, agents, groups, canned responses, tags copied; LiveChat agents invited as free Viewers). Spam folder not importable [3P, Capterra reviewer].

**Text platform.** Soft launch August 2025 at text.com. Combines LiveChat, HelpDesk, ChatBot functions plus AI agents. Go-to-market started May 2026: AI selling agents, custom skills, "AI Supervisor" training program, Shopify focus. Native WhatsApp integration June 2026. Microsoft marketplace listing February 2026. Meta Business Partner January 2026. SOC 2 Type 1 December 2025, Type 2 May 2026.

**Financials (fiscal year ends March 31).**

| Metric | Value | Note |
|---|---|---|
| FY2025/26 revenue | PLN 329.1M (-7.1%); USD 88.2M (-0.5%) | currency drag |
| FY2025/26 net profit | PLN 116.6M (-29.1%) | EBITDA margin around 46% |
| MRR March 2026 | $6.93M (-2.7% year over year) | low point |
| MRR June 2026 | $7.46M (+7.6% quarter over quarter) | record; grandfathering end |
| ARR June 2026 | $89.52M | target $100M |
| Revenue split FY2025/26 | LiveChat 83.7%, ChatBot 8.7%, HelpDesk plus KnowledgeBase 7.5% | |
| HelpDesk revenue | PLN 24.9M, +81.7% [unverified, AI-assisted Investing.com article] | another Investing.com article says ChatBot and HelpDesk declined year over year; conflict |
| Accounts paying $500+ MRR | 52-55% of MRR | moving upmarket |
| Multi-product customers | about 40% of MRR | bundling working |
| API usage revenue | about $250K per quarter | outside MRR |

**Strategy signals.**
- Ended LiveChat price grandfathering April 2026. All monthly customers moved to current list price in one pass. Churn spiked next month [3P, Blind Spot Substack]. MRR still hit record.
- Classic sales department eliminated; functions moved into support. CEO Mariusz Ciepły: transition "toward a fully product-led growth approach" (June 2026).
- Company expects 2027 = migration period to Text product [unverified, single source via search snippet].
- From Q2 2026/27 Text reports Text MRR as separate line. Q2 report (September 2026) not found during research.
- Positioning: "customer service from cost center to revenue driver". Heavy ecommerce/Shopify selling-agent pitch. VP Brand: "pivoting from a defensive to an offensive stance."

**Read for Dimmi.** Text = cash cow under pressure, moving upmarket ($500+ MRR accounts) and toward ecommerce selling. Small accounts and developers = not their focus. Price hikes plus refund disputes create churn pool of small customers. Opportunity.

## Pricing

**Current (helpdesk.com/pricing = text.com/pricing = livechat.com/pricing = chatbot.com/pricing).**

| | Essential | Growth | Enterprise |
|---|---|---|---|
| Per user per month, annual | $19 | $79 | Custom |
| Per user per month, monthly | $25 | $99 | Custom |
| AI Agents | 1 | 10 | Custom (FAQ says 50) |
| AI resolutions included | 10/month | 200/month | Custom |
| Extra AI resolutions | $49.50 per 50 ($0.99 each), auto-refill | same | Custom |
| Reply suggestions | 5/user/month | 50/user/month | Custom |
| AI text edits | 5/user/month | 100/user/month | Custom |
| Custom skills | 10 per agent | 10 per agent | Custom |
| Teams | 1 | 10 | Custom |
| Workflow rules / executions | 3 / 300 per month | 10 / 1,000 per month | Custom |
| Ticketing rules | 3 | 10 | Custom |
| Custom fields | 5 | 20 | Custom |
| Ticket tags | 5 default | Included | Included |
| Forwarding addresses | 1 | 10 | Custom |
| Email templates | 1 | 10 | Custom |
| Conversation and file history | 60 days | Unlimited | Unlimited |
| Storage | 2 GB + 50 MB/user | 15 GB + 100 MB/user | Custom |
| API calls | 10,000/month | 20,000/month | Custom |
| Extra API calls | $29.50 per 100,000 | same | Custom |
| Live visitors tracked | 50 | 400 | Custom |
| Proactive campaigns / impressions | 3 / 1,000 | 30 / 10,000 | Custom |

- Trial 14 days, no card. No permanent free plan. Tickets unlimited on all plans.
- Growth-only: AI ticket and chat summaries, tag suggestions, automated tags, chat routing rules, working hours, shared canned responses, chat supervision, agent performance reports, multilingual groups.
- Enterprise-only: SLAs, SSO, audit logs, white-label widget, scheduled reports, custom roles and multiple brands ("coming soon"), HIPAA ("coming soon"), crypto and purchase order payment.
- Resolution definition: AI Agent answer "directly solves at least one customer question". One charge per conversation. Handoff without answer not charged. No BYOK. Model provider not disclosed.
- Pricing page internal contradictions: "save up to 24%" true for Essential only (Growth about 20%); ChatBot FAQ says Essential gets 20,000 API calls vs table 10,000; HIPAA listed both included and "coming soon" [3P, eesel].

**Legacy HelpDesk pricing (help center, still live).** Team $29/agent/month annual ($34 monthly); Business $50 annual ($59 monthly); 15% annual discount. Team: 5 Viewers, 5 teams, 5 active rules, 5 custom domains, 20 shared macros. Business: 20 Viewers, 100 teams, 50 rules, 100 macros. Viewer seats free. Removing agent does not lower bill until seat count reduced.

**Real-bill math [3P, eesel, competitor].** Solo, 40 resolutions per month: Essential $68.50/month. 5 seats, 200 resolutions: $293 Essential vs $395 Growth. 1,000 tickets per month at 80% AI resolution: about $792 AI vs $152 seats. Eleventh resolution on Essential costs $49.50 due to pack minimum.

**Siblings.**
- KnowledgeBase: still standalone. $49/month annual, $59 monthly, unlimited seats and articles, 1 knowledge base; extra base $21.99-$24.99/month. AI article editor.
- OpenWidget: free widgets (contact form, FAQ, WhatsApp/Messenger buttons, product cards, visitor counter). Lead-gen funnel into Text.
- ChatBot: folded into Text plans (AI Agent). Older reviews quote standalone ChatBot from $52/month [3P].
- LiveChat legacy plans: existing customers keep plan and login per pricing page; but grandfathering ended April 2026, so "keep plan" means keep plan name, not old price. CostBench reports top LiveChat tier $59 to $89 July 2026 [unverified, 3P].

**Developer API pricing (Text Platform).** Private apps and Personal Access Token calls billed: $0.27 per 1,000 requests (0-1M), $0.22 (1-10M), $0.17 (10M+). Public marketplace apps free. One-time free credit 1,000 requests (docs) or $25 (other page) [conflict]. Failed requests billed except HTTP 500. No way to cap spend. Packages $29.99/100K, $149.99/500K.

## Feature inventory

| Area | What HelpDesk/Text has | Notes |
|---|---|---|
| Ticketing | Tickets with status, priority, assignee, team; merge; move between silos (tickets, archive, spam, trash); ticket rating; bulk actions | Unlimited tickets |
| Channels | Email (forwarding), contact forms, live chat widget, Messenger, Instagram, SMS, WhatsApp (June 2026), Shopify, chat page | Chat via LiveChat engine |
| Mailboxes | Forwarding addresses, reply addresses, custom email domains, mailboxes API, trusted/blocked senders, unsubscriptions, failed outgoing email report | Per-provider forwarding guides (Gmail, Microsoft 365, Outlook, Yahoo) |
| Views | Personal and shared saved filters | VIP customers float to top |
| Canned responses | Yes; shared ones Growth only | |
| Macros | Private and shared, one-click multi-action | |
| Tags | Yes; Essential capped 5; AI tag suggestions and auto-tags Growth | |
| Custom fields | 5 / 20 | |
| Teams and roles | Admin (owner), agent (normal), viewer (read-only, free in legacy plans); teams by language, location, skill | Custom roles Enterprise "coming soon" |
| Collaboration | Private notes, mentions, collision detection, team member takeover | |
| Automation: rules | Condition/action ticketing rules, templates | 3 / 10 cap |
| Automation: Workflows | Visual no-code canvas. Triggers: ticket created, low chat rating, keyword, schedule, manual. Actions: assign, priority, tags, fields, Slack message, Salesforce record, HTTP request, AI summarize, OpenAI step. If/else branches, delays, variables, runs log | 300 / 1,000 executions per month; vendor claims 2M+ executions per month platform-wide |
| SLA | Enterprise only | Not in lower tiers |
| Working hours | Growth | |
| Reports | Ticket volume, new tickets, response and resolution times, status durations, ratings, per agent/team, 24-hour distribution, weekly summaries, export; custom reports API | Agent performance Growth only; reviewers call reporting limited |
| AI Agent | Answers chat, knowledge sources (website, pricing pages, files up to 50 MB), 48 languages, tone, human takeover, lead qualification, product recommendations, Shopify catalog, cart recovery | Answers on website chat only |
| AI custom skills | Up to 10 per agent; "describe workflow in own words"; offers discounts, collect info, escalate | |
| AI Copilot | Reply drafts from knowledge base, text enhance (tone, grammar, length), similar ticket suggestions, language detection, summaries (Growth) | Per-user monthly caps |
| Knowledge base | Separate KnowledgeBase product; also "knowledge sources" for AI | |
| Proactive | Campaigns triggered by URL, time on page, location; visitor tracking | Chat-side |
| Apps | Web, Mac and Windows desktop; LiveChat mobile app pushes HelpDesk ticket notifications | Standalone HelpDesk mobile app not confirmed |
| Integrations | LiveChat, ChatBot, KnowledgeBase, Shopify, Salesforce, HubSpot, Slack, WordPress, WooCommerce, Webflow, Klaviyo, Mailchimp, BigCommerce, OpenAI, Zapier-style via webhooks | No Zendesk/Freshdesk/Front/Gorgias connector [3P] |
| API | REST v1, OpenAPI 1.0.0, base `https://api.helpdesk.com/v1` | See DX |
| MCP | Hosted `https://mcp.text.com/`, OAuth, read-only tools | Write not supported |
| Security | SOC 2 Type 2, GDPR, CCPA, PCI DSS SAQ A, 2FA, encryption, audit logs (Enterprise), SSO (Enterprise), WCAG 2.2 AA widget | |
| Migration | Free migration assistance; Help Desk Migration marketplace app | |

## Onboarding and DX

**Solo dev time-to-value.**
- Sign up: 14-day trial, no card. Sign-up and login go through accounts.livechat.com, redirect to text.com/app. Confusing for user who came to helpdesk.com.
- Email: forward support inbox to HelpDesk address, per-provider guide. Custom domain and reply address need extra setup; help landing page silent on SPF/DKIM.
- Chat: paste snippet or install plugin (WordPress, Shopify, Webflow). Text claims test AI Agent on own site "in under a minute" with no signup demo.
- AI: point at website and files, review answers, go live. Only 10 resolutions per month on Essential; trial user hits wall fast.
- Estimated first ticket under 15 minutes; first AI answer under 10 minutes [unverified, vendor claims plus our estimate].
- Friction: six brand domains (text.com, livechat.com, helpdesk.com, chatbot.com, knowledgebase.com, openwidget.com) with overlapping pricing pages and contradictory help center docs.

**API.**
- REST, OpenAPI spec published. Auth: Personal Access Token via Basic auth (account ID plus token) or OAuth 2 authorization code. Shares auth system with LiveChat (Developer Console on Text Platform).
- `User-Agent` header required or request may be blocked.
- Resources: licenses, agents, subscriptions, teams, templates, audit log, tickets, imported tickets, tags, transactions, custom fields, email domains, mailboxes, reply addresses, spam lists, canned responses, rules, macros, views, webhooks, predefined and custom reports.
- Cursor pagination (`next.value` plus `next.ID`), `pageSize` up to 100, `X-Total-Results` header. VIP filter capped at 10,000 tickets.
- Agents and viewers read-only on admin resources; only owner writes settings.
- Webhooks: CRUD endpoints; event docs thin. Third-party connector shows event `tickets.create` [unverified]. No public payload schema found. Rate limits not documented in docs fetched.
- Cost: every PAT call billed per 1,000 after small free credit, no spend cap. Developer scripting own helpdesk pays tax. Plan includes 10,000-20,000 calls but platform docs describe separate per-request billing; relationship unclear [unverified].

**Platform and marketplace.** Text Platform (platform.text.com, formerly developers.livechat.com): Agent App widgets (extend agent UI), Open Chat Widget (Moments, rich messages), omnichannel messaging APIs, Configuration API v3.5 (bot agents, webhooks, properties), Reports API, Billing API (charge for paid apps, supports LiveChat and HelpDesk). Marketplace 200+ apps; vendor says marketplace reaches 40,000+ brands. Discord community, API status page. Revenue share not published on fetched pages.

**MCP.** Remote server `mcp.text.com`, OAuth, permissions follow user role. Tools: `search`, `fetch`, `find-ticket`, `list-tickets`, `list-ticket-tags`, `get-chat-transcript`, `list-archived-chats`, `list-chat-tags`. No create or update. Help article says custom ticket operations need Ticketing API or Skills. Docs claim MCP "built upon Workflows" yet expose read only.

**No self-host. No open source. No BYOK.**

## What users love

- Clean, intuitive UI; "Very simple to use and does exactly what we need" (Capterra reviewer). Capterra 4.6 from about 180 reviews; G2 9.1/10 from 26 reviews (small sample).
- Fast setup for email ticketing; good for teams leaving shared Gmail.
- Collision detection, macros, views reduce double work.
- Workflows with HTTP request step and Slack alerts flexible without code.
- 24/7 support chat.
- Tight LiveChat pairing: chat and tickets in one queue.
- Text AI Agent: vendor claims 74% resolution rate, 266% conversion lift [vendor claim, unverified].

## What users hate

- Reporting limited and paywalled; one reviewer says build own dashboard instead [3P review sites].
- Search: hard to find old tickets; missing search operators; no pinned tags.
- Limited customization of ticket forms and workflows.
- Separate HelpDesk site from LiveChat felt redundant; product split confusing.
- Migration gaps from LiveChat (spam folder not importable, needs paid third-party).
- Notification gaps (no email to assignee on assignment), startup lag, missed notifications [3P, single sources].
- Billing: Trustpilot LiveChat 2/5 (10 reviews) and 2.8/5 (96 reviews, secondary source). Complaints: charge after cancellation, refund credit clawed back over "internal billing changes", renewal refunds refused per Terms of Service. Company says subscriptions "generally non-refundable".
- Pricing drift: end of grandfathering, LiveChat top tier jump, AI resolutions too few on Essential (10 per month), many meters to watch.
- AI quality: needs tinkering, chatbot "not fully trained yet" [3P review roundups].

## Dimmi: must-have at launch

Viewpoint: vibe coder or solo dev shipping SaaS, wants support live in one sitting, hates per-seat plus meters, wants AI on own key.

1. **One-line chat widget install.** Why: first channel for web app builders; HelpDesk demo claims under one minute. Bar: script tag plus npm/React component, async load, small bundle, works in Next.js/Vite/plain HTML, first message in inbox under 5 minutes from signup or `docker run`.
2. **Email channel with custom domain.** Why: support@ still main channel; HelpDesk core is email ticketing. Bar: inbound via forwarding or MX, outbound from own domain with guided SPF/DKIM check, threading correct, attachments, chat and email in same inbox.
3. **Shared inbox core.** Why: table stakes. Bar: assign, open/snoozed/closed, tags (unlimited), internal notes, saved replies, collision indicator, keyboard shortcuts, full-text search with filters that actually finds old conversations (HelpDesk weak spot).
4. **Dimmi Agent on BYOK.** Why: core promise; Text charges $0.99 per resolution. Bar: paste key (OpenAI, Anthropic, others), ingest docs URL/markdown/files, cited answers, test playground before go-live, clean handoff to human with summary, no per-resolution fee, show token cost per conversation.
5. **User identity from host app.** Why: SaaS devs need to know which user wrote. Bar: pass user ID, email, plan, custom attributes via widget; HMAC identity verification; attributes visible in sidebar.
6. **Public REST API plus signed webhooks, unmetered.** Why: Text bills every token call; devs script everything. Bar: OpenAPI spec, API keys in UI, documented event catalog with payload examples, HMAC signatures, retries with backoff, delivery log, documented rate limits.
7. **Notifications where dev lives.** Why: solo dev not watching dashboard. Bar: email plus browser push plus Slack or Discord webhook on new conversation and on AI handoff, under 10 seconds.
8. **Self-host in one command, unlimited history.** Why: Dimmi identity; Text caps Essential history at 60 days. Bar: single Docker image, SQLite or Postgres, runs on small VPS within stated memory budget, full JSON export, no feature caps by plan.
9. **Transparent flat pricing page.** Why: Text pricing spans six domains with contradictions. Bar: one number per conversation, calculator, no hidden meters, cancel in UI, prorated refunds policy plain.

## Post-MVP priority list (strict order)

1. **MCP server with write tools.** Text MCP read-only; vibe coders live in Claude Code/Cursor; reply, tag, close from agent = instant marketing win.
2. **Workflows/rules lite with HTTP step.** Text's most-used automation (2M executions per month claimed); start with triggers (new conversation, tag added, rating) plus actions (assign, tag, webhook, Slack).
3. **Agent tools/skills as code.** Text custom skills capped 10 per agent; let devs register HTTP or TypeScript tools (refund, lookup order, reset password) the BYOK agent calls.
4. **Help center (public knowledge base).** Text sells separately at $49/month; doubles as AI knowledge source and SEO.
5. **Basic reports free.** HelpDesk paywalls reports; ship volume, first response, resolution time, AI resolution rate, CSAT.
6. **CSAT rating on close.** Cheap; feeds reports and rating-triggered workflows.
7. **Importers (HelpDesk, Intercom, Crisp, Zendesk, CSV).** Catch Text churn pool after grandfathering end; migration gaps are HelpDesk complaint.
8. **Business hours, SLA timers, auto-replies.** HelpDesk SLA Enterprise-only; give everyone.
9. **Teams and routing, roles, free viewer seats.** Matters once solo dev hires first help.
10. **WhatsApp, Messenger, Instagram channels.** Text has them; less urgent for SaaS devs.
11. **Mobile PWA with push.** Reply from phone; Text mobile story weak.
12. **AI extras on BYOK: summaries, tag suggestions, reply drafts, text edit.** Text caps per user per month (5 on Essential); Dimmi free beyond token cost.
13. **Custom fields, macros, views.**
14. **SSO, audit log, white-label widget.** Enterprise checkbox later.

## Standout ideas beyond HelpDesk

- **"Install with prompt".** Copy one prompt into Cursor/Claude Code/Lovable/Bolt; agent installs widget, sets identity, wires webhooks. Marketing: "support live in one prompt".
- **Support-to-PR loop.** Agent detects bug report, opens GitHub issue with repro, browser info, user attributes, conversation link; optional hand to coding agent; customer auto-notified when fix merged and deployed.
- **Meter-free comparison calculator.** Public page: same workload priced on Text (seats plus $0.99 resolutions plus API packs) vs Dimmi (per conversation plus own token cost). Text math eesel published makes good template.
- **Config as code and CLI.** `dimmi` CLI plus YAML/TS config for saved replies, workflows, agent prompt, tools; versioned in repo, deploy via CI. Nobody in this segment does it.
- **Agent evals and replay.** Replay past conversations against new prompt or model, diff answers, cost per model. Shows BYOK value: pick cheapest model that passes.
- **Live token cost per conversation.** Show model, tokens, cents on each AI reply. Radical transparency vs opaque resolution meter.
- **Stripe and auth context sidebar.** Auto-pull plan, MRR, last invoice, failed payments by user email; agent tool to answer billing questions safely.
- **Free unmetered API and webhooks forever.** Direct jab: Text bills Personal Access Token calls $0.27 per 1,000 with no spend cap.
- **One-click deploy buttons.** Railway, Render, Fly, Vercel-plus-database; "runs on $5 VPS" benchmark published.
- **Headless widget SDK.** Own UI with hooks (`useConversation`), for devs who hate stock widgets.
- **Data ownership pledge.** Full export any time, no history caps, open-source license; contrast with 60-day history on Text Essential.

## Open questions and unverified

- Is helpdesk.com still selling standalone HelpDesk Team/Business, or only Text plans? Help center vs pricing page conflict.
- HelpDesk revenue: +81.7% to PLN 24.9M (one AI-assisted article) vs "declined year over year" (another). Annual report not read.
- 2027 forced migration of legacy LiveChat/HelpDesk customers to Text: single-source.
- Webhook event catalog, payload shapes, rate limits: not found in fetched docs.
- Whether plan-included API calls (10,000/20,000) offset Text Platform per-request billing.
- AI model provider behind Text AI Agent: undisclosed.
- Standalone HelpDesk mobile app status: unconfirmed.
- Q2 2026/27 report (first separate Text MRR line) not yet found; likely published around early October 2026. Worth checking for adoption numbers.
- Review sites (Capterra, G2, GetApp, Software Advice) blocked direct fetch; review data via search snippets only.

## Sources

1. https://www.helpdesk.com/pricing/
2. https://www.helpdesk.com/
3. https://www.helpdesk.com/features/
4. https://www.helpdesk.com/workflows/
5. https://www.helpdesk.com/help/
6. https://www.helpdesk.com/help/pricing-and-plans-in-helpdesk/
7. https://www.helpdesk.com/help/forward-your-emails-to-helpdesk-and-convert-them-into-tickets/
8. https://www.helpdesk.com/help/connect-ai-assistants-to-tickets-with-mcp/
9. https://api.helpdesk.com/docs
10. https://www.text.com/
11. https://www.text.com/pricing/
12. https://www.livechat.com/pricing/
13. https://www.chatbot.com/pricing/
14. https://www.knowledgebase.com/pricing/
15. https://www.openwidget.com/
16. https://platform.text.com/
17. https://platform.text.com/docs/guides/mcp
18. https://platform.text.com/docs/getting-started/api-pricing
19. https://platform.text.com/pricing
20. https://www.labs.helpdesk.com/help/livechat-integration-manage-tickets-and-chats-in-one-application/
21. https://www.chatbot.com/help/bot-responses/create-ticket
22. https://www.eesel.ai/blog/helpdesk-com-pricing [3P, competitor]
23. https://www.investing.com/news/company-news/text-sa-q4-202526-slides-mrr-dips-as-firm-targets-premium-customers-93CH-4600739
24. https://ph.investing.com/news/company-news/text-q1-202627-slides-mrr-hits-record-as-ai-strategy-takes-shape-93CH-2477981
25. https://finance.yahoo.com/markets/stocks/articles/texts-arr-exceeds-89-5-125600062.html
26. https://www.biznesradar.pl/a/148900,text-w-2025-26-329-1-mln-zl-przychodow-116-6-mln-zl-zysku-netto-i-83-12-mln-usd-arr
27. https://investor.text.com/news/text-group-in-fy-2025-26-pln-329-1-million-revenue-pln-116-6-million-net-profit-and-usd-83-12-million-arr
28. https://theblindspotintel.substack.com/p/text-ended-price-protection-then [3P]
29. https://biznes.pap.pl/wiadomosci/firmy/text-software-expects-end-livechat-price-grandfathering-be-key-operating-results
30. https://www.customerexperiencedive.com/spons/text-turns-customer-service-into-a-profit-engine-with-new-agentic-ai-capabi/819832/ (sponsored)
31. https://en.wikipedia.org/wiki/Text_(company)
32. https://www.producthunt.com/products/helpdesk/launches/helpdesk-com
33. https://www.capterra.com/p/185973/HelpDesk/reviews/ (via search snippets)
34. https://www.g2.com/compare/helpdesk-vs-zendesk-for-customer-service (via search snippets)
35. https://www.softwareadvice.com/issue-tracking/helpdesk-profile/reviews/ (via search snippets)
36. https://thecxlead.com/tools/helpdesk-review/ (via search snippets)
37. https://www.trustpilot.com/review/livechat.com (via search snippets)
38. https://costbench.com/software/help-desk/livechat/ [3P]
39. https://www.capterra.com/p/10031687/Text/ (via search snippets)
40. https://www.livechat.com/updates/product-updates/handle-tickets-livechat-mobile/
