# Dimmi Cloud Pricing: Research and Recommendation

Date: 2026-11-01. Scope: competitor pricing, billable unit definition, unit economics, bill comparison, BYOK design, guardrails, billing stack, packaging. Tags: [unverified] = single source or own estimate. [3P] = claim written by competitor of subject.

---

## TL;DR recommendation

**One plan. Pay per conversation. Everything included.**

| Item | Number |
|---|---|
| Free tier | First 100 billable conversations each month, forever, no card, all features incl. Dimmi Agent (BYOK). Card required once workspace passes 100. |
| Band 1 | Conversations 101 to 1,000: **$0.03** each |
| Band 2 | Conversations 1,001 to 10,000: **$0.015** each |
| Band 3 | Conversations 10,001 to 50,000: **$0.01** each |
| Scale | Above 50,000/month: **talk to us**. Annual contract, custom rate set per deal. Reference rate published (about $0.006) so pricing never looks hidden. |
| Margin floor | Price never below 5x measured variable cost per conversation (80% gross margin). Bands 2 and 3 hit floor only after cost cuts (section 3). |
| Style | Graduated (like PostHog). No platform fee or seat fee. |
| Billing | Postpaid monthly, card on file. Invoices under $5 roll to next month (max 3 months). Alternative: $5 minimum once paying. |
| Spend cap | On by default. New paid workspace default cap $50/month. User can raise, lower, or remove. |
| Alerts | 50%, 80%, 100% of cap plus "on pace for $X" forecast. Email, in-app, webhook. |
| Over cap | 20% buffer free (min 20 conversations). Beyond: new conversations captured, replies paused until cap raised. Never drop messages. |
| Prepaid credits | Optional annual commit: $1,000 gets 10% bonus, $10,000 gets 20%. Valid 24 months. |
| AI | BYOK only at launch. Dimmi charge never changes whether human or AI answers. Zero AI markup. |
| Discounts | Open-source projects: free to 2,000 conversations/month. Startups: $300 credits for 12 months. Nonprofit/edu: 50% off. |
| Enterprise | Commit from $3,600/year (about 50,000 conversations/month at $0.006): SLA, invoice/ACH, DPA, EU region, named support. No locked features. |

Resulting bills: hobby (50 conv) **$0**. Indie (500) **$12**. Growing (5,000) **$87**. 10,000 **$162**. 25,000 **$312**. Scale-up (50,000) **$562** at published rates, custom contract below that. Plus user's own LLM tokens, roughly $0.0025 to $0.05 per AI-touched conversation [unverified estimate].

History: first proposal was 100 free then $0.05 / $0.025 / $0.015 (bills $20 / $245 / $1,495). Revised 2026-11-01: $0.05 lost human-only buyers from about 5,000 conversations up. 500 free rejected because most customers would never pay.

Core pitch: "No AI tax. No seat tax. Same price human or AI answers." Fin charges $0.99 per AI outcome. Same work costs cents in tokens.

Surprises:
- Intercom renamed itself Fin (May 2026). Salesforce agreed to buy Fin for about $3.6B (June 2026); close expected early 2027. Migration window open.
- Help Scout tried contact-based pricing (Nov 2024), hit backlash, now shows per-user pricing again.
- Front prices Autopilot AI "from $0.05 per conversation", above Dimmi band 1 ($0.03) and on top of seats.
- Featurebase raised Fibi $0.29 to $0.49 per resolution (July 2026), existing customers included [3P].
- Meta charges WhatsApp service messages after first 1,000 per number per month (since 2026-10-01).
- Human-only: Dimmi now cheapest at 500 and 5,000 conversations. At 50,000, seat tools still win (Crisp $295 flat, 20 seats) unless our cost drops to $0.0012 or less. Custom contract or self-host covers that buyer.

---

## 1. Competitor pricing table (verified on pricing pages, Nov 2026 unless tagged)

| Product | Base / seat (per month) | Meter / free tier | AI charge | Annual | Notes |
|---|---|---|---|---|---|
| Fin (ex-Intercom) | Essential $29 ($19 promo, new customers), Advanced $85, Expert $132 | Trial only | $0.99 per outcome (resolution, 24h assumed resolution, or deliberate handoff Procedure); once per conversation. Standalone Fin $49 incl. 50 | Split unclear | Lite seats free on Advanced/Expert |
| Featurebase | $29 / $59 / $99 yearly ($37/$75/$129 monthly) | Free: 1 seat, no AI | Fibi $0.49 per resolution, handoffs free | 20% | Branding removal $69 |
| Productlane | $29 / $59 / $99 per user yearly | No clear free plan [unverified] | 25/50/100 included, then $0.79; 24h silence = resolved | Toggle | Agent pauses at cap |
| Help Scout | Free (5 users, 100 contacts); $25 / $45 / $75 per user | Contacts limit on Free only | AI Answers $0.75 per resolution, spending cap | 16% | Legacy contact accounts remain |
| Crisp | Per workspace: Free (2 seats), $45 (4), $95 (10), $295 (20+) | Profile caps 100 / 5,000 / 50,000 / 200,000 | Credits incl. $5/$25/$75 (about 90/450/1,350 AI conversations) | In app | Extra agent $10 |
| Plain | $45 (annual; $55 list), $99 per user | 7-day trial | 100 credits per resolution; $10 per 1,000 credits (about $1.00) [inferred] | 20% | Seat mins unclear |
| Pylon | $89 / $139 per seat (annual) | None | Credits, min 5,000 commit | Annual | SAML $125 add-on |
| Gorgias | $10 (50 tickets), $60 (300), $360 (2,000), $900 (5,000) | Overage $0.40 / $0.36 per ticket | AI Agent $0.90 to $1.00 per automated interaction; $1.50 over allowance | ~17% | Conflicting page prices [unverified] |
| Zendesk Suite | $55 / $115 per agent annual ($69 / $149 monthly) | Trial | 5 / 10 ARs per agent incl.; $1.50 committed, $2.00 pay-as-you-go | ~20% | New resolution tiers since 2026-05-18 |
| Front | $25 / $65 / $105 per seat (annual) | Trial | Autopilot from $0.05 per conversation; Copilot $20/seat | 24% | WhatsApp: Meta cost plus 20% |
| HelpDesk.com | $19 / $79 per seat annual ($25 / $99 monthly) | Trial | 10 / 200 resolutions incl.; extra $0.99 each (50-pack $49.50) | up to 24% | Unlimited tickets |
| Chatwoot Cloud | $0 / $19 / $39 / $99 per agent | Free: 2 agents, 500 conv, chat only | Captain credits 300/500/800 incl.; $20 per 1,000 extra | n/a | Free for qualifying OSS |
| Tidio | $24.17, from $49.17 | Billable conversations (human reply); free 50 | Lyro from $32.50 for 50 AI conversations | 2 months free | |
| Tawk.to | $0, unlimited agents | None | AI Assist from $29 | n/a | Branding removal $29; hired agents $1/hour |
| Chatbase | $32 / $120 / $400 (yearly) | Credits 700 / 4,000 / 15,000 | $40 per 1,000 extra credits | 20% | Branding removal $99 |

### Help Scout contact pricing and reversal
- November 2024: plans bill "contacts helped" per month, unlimited users. Tier = average of last three months. Auto-replies excluded; same person across channels counts once.
- Backlash (bitsplitting.org, April 2025): solo user $22 to $50/month; three-person team helping 500 customers $66 to $266/month (about 4x). Complaints: unpredictable bills, "penalty for growth", free tier lost API and export.
- Now: pricing page per-user ($25/$45/$75), contacts unlimited on paid. Reviews say reverted after churn [3P, date unverified]. Legacy contact accounts remain.
- Lesson: forcing seat customers onto usage meter with 4x increase breeds churn. Usage pricing works when unit cheap, capped, and default from day one.

### Gorgias ticket pricing and complaints
- Billable ticket: any helpdesk message sent (human, AI Agent, or Rule auto-reply). Unanswered inbound, notes, tags free. New ticket after 10 days (email) or 3 days (chat/SMS/voice) inactivity. Merges still billed.
- Complaints [3P]: Rule auto-replies inflate tickets; AI on separate meter doubles cost (example $360 to $720/month). Core grievance: unpredictability.

### Tawk.to and Crisp
- Tawk.to: core free forever, unlimited agents. Revenue from branding removal $29/month, AI Assist from $29/month, hired agents $1/hour. Sets human-only chat floor at $0.
- Crisp: flat per workspace, seat bundles (4/10/20), extra agent $10, unlimited conversations, AI via included credits. Cheapest paid option for many seats.

---

## 2. Definition of "conversation"

### How others define billable units
| Vendor | Unit | Starts | Window / reopen | Exclusions |
|---|---|---|---|---|
| Gorgias | Billable ticket | Any helpdesk message (incl. Rule auto-reply) | New ticket after 10 days email, 3 days chat/SMS/voice | Unanswered, internal changes; merges still billed |
| Tidio | Billable conversation | Human agent reply | Multiple messages = one | Bot (Flows/Lyro) replies not counted as human |
| Help Scout (legacy) | Contact helped | User reply or created conversation | Per person per month | Auto-replies, CC/BCC |
| Fin | Outcome | AI resolution or deliberate handoff Procedure | Assumed after 24h with no further help; reversed if customer returns | Default escalations, rule escalations, failures, abandoned clarifications, greeting-only |
| Zendesk (pre May 2026) | Automated resolution | AI generative reply, positive or no feedback, AI-verified relevance | Email 72h; messaging default 2h, max 72h | Internal notes/actions only |
| Productlane | AI resolution | Confirm, 24h silence, or teammate mark | Follow-up within 72h refunds | Teammate testing |
| WhatsApp (Meta) | Per message since 2025-07-01 | Business message delivered | 24h customer service window from last user message | User-to-business free; service free first 1,000/number/month since 2026-10-01 |

Fair pattern: inbound plus real reply, one charge per thread, inactivity reset, refund when wrong. Unfair: billing auto-replies (Gorgias), "silence equals resolved" (Fin, Productlane), billed merges.

### Recommended Dimmi definition (spec)

**D1. Billable conversation.** Thread becomes billable when both true: (a) thread contains at least one inbound message from external contact; (b) workspace sends at least one substantive reply to contact, by human teammate or Dimmi Agent. Charge records at moment of first substantive reply ("activation").

**D2. Substantive reply.** Message written by teammate or Dimmi Agent answering contact. Not substantive: auto-acknowledgments, out-of-office, business-hours notices, CSAT surveys, pre-chat welcome, system notifications.

**D3. One charge per window.** Activation opens billing window. Window closes after 7 days with no message in thread, or 30 days after activation, whichever first. All messages, channels, teammates, AI turns inside window = one charge.

**D4. Reopen.** Contact writes after window closed and workspace replies: new charge. Nobody replies: no charge.

**D5. Channels.** Widget, email, WhatsApp, Slack Connect, Discord, API, mobile: same rule, same price. SMS and voice carrier fees passed through. WhatsApp: Meta bills business directly (Tech Provider model), nothing passed through.

**D6. AI-only.** Billable same as human (activation on first Dimmi Agent answer). No AI surcharge. Tokens billed by user's provider.

**D7. Outbound.** Broadcasts, campaigns, tours: never billable on send. Teammate-initiated 1:1: billable only once contact replies and workspace answers. Outbound fair-use: 10x monthly billable count, min 1,000 emails, blocks free bulk sending.

**D8. Internal.** Threads with only workspace members (back-office tickets, DMs, tasks): never billable. Internal notes: not replies.

**D9. Spam, bots, abuse.** Not billable: (a) threads auto-classified spam before reply; (b) threads teammate marks spam within 14 days of activation (auto-credit); (c) sessions failing Dimmi bot checks or hitting rate limits; (d) contacts on blocklist. Spam-marked threads move to spam folder and stay billable if un-marked. Workspaces with spam-credit ratio above 25% get review.

**D10. Test mode.** Free: conversations by workspace members, widget test mode, sandbox workspaces (cap 1,000/month).

**D11. API-created.** Same rule as D1. Imported history (migration from Fin, Zendesk, Help Scout, CSV): never billable.

**D12. Merges.** Merging threads of same contact within 7 days of each other: credit all but one (Gorgias does not).

**D13. Failed delivery.** Reply hard-bounces or fails delivery and no other reply delivered: credit.

**D14. Month boundary.** Conversation counts in month of activation only. No double count across months.

**D15. Audit.** Append-only ledger per charge: conversation ID, contact, channel, activation time, rule version, actor (human/AI), credit reason. Usage page links each line to thread; CSV and API export. Counting code open source. Rule changes versioned, 60 days notice.

Plain-English version for pricing page: "We charge when your customer writes and your team or AI writes back. One charge covers that thread until it goes quiet for 7 days. Spam, tests, internal chats, auto-replies, imports: free."

---

## 3. Unit economics

### Cost per conversation (AWS us-east-1 list prices)
Per conversation: 10 messages; 60 KB Postgres incl. indexes; 150 KB S3 attachments; 5 outbound + 2 inbound SES emails; 150 websocket frames, 60 connection-minutes; 4.5 MB widget bandwidth (300 page views, about 15 KB each cached); 36-month retention.

| Component | Price used | Cost per conversation |
|---|---|---|
| SES outbound | $0.10 per 1,000 | $0.0005 |
| SES inbound | $0.10 per 1,000 + $0.09 per 1,000 chunks | $0.0004 |
| Websockets (API Gateway price as ceiling) | $1.00 per million messages, $0.25 per million connection-minutes | $0.0002 |
| Postgres storage (Multi-AZ, backups, 36 months) | gp3 about $0.115/GB-month [unverified] | $0.0007 |
| Postgres compute (db.m7g.large $0.168/hour, Multi-AZ about $245/month, at 1M conv/month) | | $0.0003 |
| S3 attachments, 36 months plus egress | $0.023/GB-month [unverified], $0.09/GB egress [unverified] | $0.0002 |
| Widget CDN bandwidth | about $0.085/GB [unverified] | $0.0004 |
| App compute (EC2 Graviton, e.g. t4g/m7g on-demand; binary + systemd, no containers) [verify current prices] | 1 vCPU/2 GB per 100k conv | $0.0004 |
| Logs, metrics, queues, misc | estimate | $0.0005 |
| **Variable total** | | **about $0.004** |
| With 2x safety factor | | **$0.008** |

Fixed baseline: about $1,450/month (Multi-AZ RDS, ElastiCache for Valkey 2 small nodes Multi-AZ about $50 [verify], NAT, load balancer, CDN, staging, monitoring). Valkey required since 2026-11-02 (rebuildable state only; `tech-stack.md`). Adds $0.0145/conv at 100k paid conv/month, $0.00145 at 1M.

Payment costs dominate small invoices. Stripe card 2.9% + $0.30, Billing 0.7%, Tax 0.5%:
- $12 invoice: $0.79 fees (6.6%).
- $87 invoice: $3.87 (4.4%).
- $562 invoice: $23.34 (4.2%).
Reason for $5 roll-over rule.

### Margin by band

Rule: price at least 5x measured variable cost per conversation. Equals 80% gross margin floor.

| Band | Price | Margin at $0.008 (2x safety) | Margin at $0.004 (AWS list) | Margin at $0.002 (after cuts) |
|---|---|---|---|---|
| 101 to 1,000 | $0.03 | 73% | 87% | 93% |
| 1,001 to 10,000 | $0.015 | 47% | 73% | 87% |
| 10,001 to 50,000 | $0.01 | 20% | 60% | 80% |
| 50,000+ custom | about $0.006 | negative | 33% | 67% (80% needs cost $0.0012) |

Reading: band 1 safe today. Bands 2 and 3 need cost cuts below before launch, or start higher (for example $0.02 and $0.015) and drop when measured cost allows. Custom rate set per deal, never below floor.

### Cost reduction plan (needed for 80% floor)

| Lever | Saving per conversation | Notes |
|---|---|---|
| Attachments and widget assets on Cloudflare R2 (no egress fee) | about $0.0006 | S3 egress plus CDN bandwidth gone |
| Cold storage: conversations older than 12 months moved to compressed object storage, searchable on demand | most of $0.0007 Postgres storage | Postgres storage is biggest line |
| Cells: thousands of small workspaces per Postgres | lower compute and fixed cost share | Same codebase, `workspace_id` everywhere |
| Cloud compute on reserved Graviton or Hetzner/OVH dedicated | 3x to 10x on compute and database | Adapters let our cloud run where cheapest; self-host keeps AWS default |
| Postgres + small Valkey only (no ClickHouse, Kafka, search cluster) | fixed baseline down | Already in stack (Valkey about $50/month cloud) |
| Prepaid credits, invoice roll-over, annual | payment fees down | $0.30 fixed fee hurts small invoices |
| No sales team, self-serve, community support, own support agent on cheap tokens | overhead | |

Target: fully loaded variable cost $0.002 or less. Measure real numbers in first 3 months of cloud before committing bands 2 and 3 publicly.

Free tier cost: maxed free workspace about $0.20 to $0.80/month.

Not modeled: human support, deliverability ops, abuse handling. Band 1 margin funds these.

### LLM cost per conversation (paid by user, BYOK)
Assumption: AI-touched conversation = 6 model calls, 30K input tokens (half cached), 1.8K output tokens. Claude list prices (Nov 2026):
| Model | Price per MTok in/out | Est. cost per conversation |
|---|---|---|
| Haiku 5.5 | $0.10 / $0.50 | about $0.0025 |
| Sonnet 5.5 | $2 / $10 | about $0.05 |
| Opus 5.5 | $4 / $20 | about $0.10 |
[unverified estimate; real usage varies with RAG size and tool calls]

Fin charges $0.99 per outcome. At 40% resolution rate, Sonnet-class tokens per resolution about $0.125. Fin markup over raw tokens roughly 8x (Sonnet) to 150x (Haiku).

---

## 4. Bill comparison: four customer profiles

Assumptions: list prices; cheapest plan fitting seat count; AI scenario = AI touches every conversation and fully resolves 40%. Per-resolution vendors bill 40%. Crisp bills credits per AI conversation (about $0.056 each from "$5 about 90" [unverified extrapolation]). Dimmi LLM cost at Sonnet-class $0.05/conv paid to provider. Help Scout contacts treated as equal to conversations.

### Human-only (no AI)
| Monthly | Hobby 50 conv, 1 seat | Indie 500, 2 seats | Growing 5,000, 5 seats | Scale-up 50,000, 15 seats |
|---|---|---|---|---|
| **Dimmi Cloud** | **$0** | **$12** | **$87** | **$562** published, custom below |
| Fin/Intercom Essential $29 | $29 | $58 | $145 | $435 |
| Featurebase Growth $29 | $0 (Free) | $58 | $145 | $435 |
| Productlane Starter $29 | $29 | $58 | $145 | $435 |
| Help Scout | $0 (Free) | $50 | $125 | $375 |
| Crisp | $0 (Free) | $45 (Mini) | $95 (Essentials) | $295 (Plus) |
| Plain Foundation/Horizon | $45 | $90 | $225 | $1,485 (Horizon) |
| Chatwoot Cloud | $0 | $0 (Hacker) | $95 | $285 to $585 |

### With AI agent
| Monthly | Hobby | Indie | Growing | Scale-up |
|---|---|---|---|---|
| **Dimmi Cloud + own Sonnet-class tokens** | **$2.50** ($0 + $2.50) | **$37** ($12 + $25) | **$337** ($87 + $250) | **$3,062** ($562 + $2,500) |
| **Dimmi Cloud + own Haiku-class tokens** | **$0.13** | **$13.25** | **$99.50** | **$687** |
| Fin/Intercom Essential + $0.99 | $48.80 | $256 | $2,125 | $20,235 |
| Featurebase Growth + $0.49 | $38.80 | $156 | $1,125 | $10,235 |
| Productlane Starter + $0.79 (25 incl.) | $29 | $196 | $1,705 | $16,215 |
| Help Scout + AI Answers $0.75 | $15 | $200 | $1,625 | $15,375 |
| Crisp + credits | $45 | $68 | $348 | $3,000 |
| Plain + about $1.00/resolution | $55 | $280 | $2,215 | $21,460 |

Reading:
- With AI, Dimmi cheapest everywhere; 5x to 7x cheaper than Fin, Plain, Help Scout, Productlane at scale-up with Sonnet-class tokens, far more with Haiku-class. Crisp closest rival.
- Human-only: Dimmi cheapest at 500 ($12 vs Crisp $45) and 5,000 ($87 vs Crisp $95). At 50,000, seat tools still cheaper (Crisp $295). Custom contract (target under $300 if cost reaches $0.0012) or self-host covers that buyer.
- Real Intercom scale-ups run Advanced/Expert ($1,275 to $1,980 for 15 seats), so table flatters rivals.

---

## 5. BYOK design

### Precedents
| Product | Key handling | Lesson |
|---|---|---|
| Cursor | BYOK covers chat only; Agent/Edit/Tab/cloud agents stay on Cursor billing; paid plan needed [3P forum] | Partial BYOK confuses users |
| Zed | OS keychain, never settings file; env vars override | Clean local model |
| Raycast | Anthropic, Google, OpenAI, OpenRouter; tells user to watch provider console; BYOK now Pro-only (v2) | BYOK can still sit behind paywall |
| Warp | Per-user keys, no team management; fallback to Warp credits off by default; Auto mode always uses credits | Opt-in fallback respected |
| TypingMind | Browser local storage, optional AES, no backend; one-time license | Zero-trust option |
| Cline | VS Code SecretStorage (OS keychain); plain JSON in CLI mode [3P] | Key only goes to chosen provider |
| n8n | Credentials encrypted in DB with instance key (auto or env); lose key, lose credentials | Self-host pattern |
| Chatwoot | Self-host Captain: OpenAI key plus custom endpoint, Enterprise Edition required; cloud sells credits | AI gated behind paid license |
| Vercel AI Gateway | Team-scoped, 10 keys per provider, test button, replace without downtime; zero markup; **fallback to paid credits by default**; BYOK spend outside budgets | Silent fallback = surprise bills |
| OpenRouter | "Securely encrypted"; 5% fee on BYOK, $25,000/month free allowance on pay-as-you-go [verify]; per-key "never use shared capacity" | Fee on BYOK possible |

### Dimmi BYOK spec
- **Providers:** Anthropic, OpenAI, Google, OpenRouter, Azure OpenAI, Bedrock, any OpenAI-compatible endpoint (Ollama, vLLM on self-host). OpenRouter or Vercel AI Gateway key unlocks every model.
- **Model per task:** cheap model for triage, spam, language; strong model for answers. Presets: Cheap (Haiku-class), Balanced (Sonnet-class), Best.
- **Cloud storage:** envelope encryption, per-workspace data key from AWS KMS, AES-256-GCM. Key shown once, then masked (last 4). Decrypt only in inference worker memory. Never logged. Every decrypt audited. Admins only manage keys.
- **Self-host storage:** instance master key from env (like n8n). Boot fails if missing. Docs: back up key apart from database.
- **Validate on save:** tiny test call ("Test key", like Vercel). **Rotate** in place without downtime.
- **No silent fallback.** Key fails or rate-limited: hand off to human, banner, email alert. Never spend Dimmi money on user's behalf. Contrast Vercel default fallback.
- **Spend visibility:** tokens per call; dollars via maintained price table; cost per conversation, per resolution, per day; forecast. Compare line: "Fin would charge $X for these outcomes."
- **AI budget caps:** daily and monthly dollar caps; per-visitor limit (default 30 AI messages per session, 100 per day); turn limit per conversation. Cap hit: AI pauses, human handoff.
- **Abuse:** spam and bot check before AI answers; tool permission guard against prompt injection; onboarding nudges provider-side spend limit and scoped key.
- **Privacy:** data goes to user's chosen provider under user's own terms; ZDR depends on that contract.
- **Later: "Dimmi AI credits at cost."** Prepaid only, provider list price plus 5% for card fees and fraud. OpenRouter or Vercel AI Gateway behind it. Never per-resolution. Ship only on demand; prepaid AI credits attract stolen-card fraud.

Positioning: "Bring your key. Pay provider list price. Nobody marks up your AI."

---

## 6. Usage pricing: pitfalls and guardrails

### Cases
- **Vercel / Cara:** about $96K bill from one viral week of bandwidth; no cap caught it [3P]. Vercel now offers Spend Management alerts and optional hard cap, manual setup on paid.
- **Netlify:** $104,500 bill (Feb 2024) from 3.4 MB file, likely DDoS; waived after Hacker News outrage [3P].
- **Supabase:** Pro $25 incl. $10 compute credits. Spend cap on by default; over-quota usage stops, not billed. Compute outside cap.
- **Railway:** hard limit shuts workloads; alerts 75/90/100%. Limit set after overspend does not help [forum].
- **PostHog:** over 90% of companies pay $0; 1M events free monthly; graduated rates from $0.00005; per-product limits drop data at cap.
- **Neon:** usage billing with $5 minimum, minimum removed Dec 2025 [3P].
- **Clerk:** free tier raised to 50,000 monthly retained users (Feb 2026).
- **Resend:** free 3,000 emails/month (100/day); Pro $20 for 50,000.

### Indie reaction pattern
- Love: generous free tier, $0 at hobby scale, visible unit price, no sales call.
- Hate: no cap, surprise bills, meters driven by abuse they cannot control, multiple meters, minimums, hikes on existing users.
- One viral surprise-bill post outweighs hundreds of happy users.

### Dimmi guardrails (ship before launch)
1. Cap on by default for every paid workspace ($50 starting default).
2. Live meter in app header: "312 conversations, $10.60, on pace for $31."
3. Alerts 50/80/100% plus anomaly alert (daily count 3x trailing 7-day average).
4. Spam and bot conversations never billed; auto-credit when marked spam.
5. Single meter. No second AI meter.
6. Graduated bands only lower unit price as volume grows. Never cliff.
7. Price calculator on pricing page with own-LLM cost estimate.
8. Price changes: existing customers grandfathered 12 months minimum; 60 days notice.
9. Self-serve dispute button; auto-credit under $50.

### Open-source monetization patterns
- PostHog: MIT core plus ee folder [unverified]; cloud wins on scale and free tier.
- Supabase: Apache 2.0; cloud = managed Postgres and ops.
- Plausible: AGPLv3 CE released twice yearly; funnels, journeys, revenue goals, SSO, sites API cloud-only.
- Cal.com: AGPLv3 plus commercial license for /ee (SSO, orgs, payments).
- Chatwoot: MIT plus separate license for enterprise/ folder.
- Sentry: FSL, non-compete, converts to Apache 2.0 or MIT after 2 years.
- n8n: Sustainable Use License (not OSI) plus .ee enterprise license.

Cloud value when self-host free: email deliverability (MX, DKIM, reputation), Meta/WhatsApp verification, mobile push certs, widget CDN, uptime, backups, upgrades, network spam intelligence, SOC 2, residency, zero ops.

Decision (PO 2026-11-02): **MIT** whole repo, no withheld "ee" features. Matches "unlimited everything" and maximizes adoption by developers and agents; accepts hosted-clone risk (moat = cloud value above + speed). Earlier recommendation (AGPLv3) superseded.

---

## 7. Billing infrastructure

| Option | Type | Cost | Fit |
|---|---|---|---|
| Stripe Billing (meters) | Hosted billing | 0.7% of billing volume (pay-as-you-go) plus card fees | Best default; graduated tiers native |
| Stripe Tax | Tax calc | 0.5% per transaction (no-code) or $0.50 per transaction (API) where registered | Pair with Billing |
| Stripe Managed Payments | MoR | about 3.5% on top of processing; preview status unclear [3P] | Watch; same Stripe customer object |
| Metronome, Orb | Enterprise metering | Contact sales; Stripe closed Metronome deal Jan 2026 | Overkill |
| Lago | OSS billing, AGPLv3 | Self-host free; premium via sales | Extra service to run |
| OpenMeter | OSS metering, Apache 2.0 | Acquired by Kong (Sep 2025); cloud folding into Kong Konnect | Uncertain cloud future |
| Autumn | OSS layer over Stripe (credits, limits) | Hosted pricing not found | Young, Stripe-only |
| Polar | MoR, Apache 2.0 | Starter 5% + $0.50; Pro $20/month + 3.8% + $0.40; +1.5% non-US cards | OSS-friendly MoR, usage engine |
| Paddle | MoR | 5% + $0.50 | Mature MoR; less dev-native |
| Lemon Squeezy | MoR | Stripe-owned since 2024 [unverified] | Roadmap folding into Stripe |

**Pick:**
1. **Own usage ledger in Postgres** (append-only, idempotency key = conversation ID + window index) in open-source repo. Truth for caps, alerts, dashboard, disputes.
2. **Stripe Billing**, Dimmi computes graduated price from own ledger and posts one-off invoice items monthly (0.4% Invoicing fee instead of 0.7% Billing meters; pricing logic stays in open-source code). Customer Portal for cards and invoices. Decided in `specs/12-cloud-billing/stripe.md`.
3. **Stripe Tax** day one. Mostly B2B: EU VAT reverse charge; US sales tax only past state thresholds.
4. Caps, buffer, credits logic live in Dimmi, not Stripe.
5. Revisit MoR (Stripe Managed Payments at GA, or Polar) if tax registrations multiply. Polar first choice if team wants zero tax ops day one; costs 1.5 to 2 points more.

Fee on $20 invoice: Stripe stack $1.12; Polar Starter $1.50 (+$0.30 non-US card); Paddle $1.50.

---

## 8. Packaging

- **No feature tiers.** Every workspace: unlimited seats, teams, inboxes, help centers, integrations, API, webhooks, Dimmi Agent BYOK, SSO/SAML, SCIM, audit log, export, unlimited retention. Developers resent "SSO tax"; free SSO costs nothing.
- **Branding:** "Powered by Dimmi" badge on free tier only; gone once workspace pays anything. Viral loop. Rivals charge for removal (Tawk.to $29, Featurebase $69, Chatbase $99).
- **Paid extras:** SMS and voice at carrier cost plus small margin (WhatsApp billed by Meta directly) (about 10%; Front adds 20%). Dedicated infrastructure. EU residency when region ships. Only things that cost us real money; no core feature locked.
- **Free users:** convert automatically as they grow (card asked past 100). Badge markets Dimmi. Cost to us about $0.20 to $0.80/month each.
- **Annual commit:** prepaid credits with bonus (10% at $1,000, 20% at $10,000), valid 24 months.
- **Scale and enterprise:** above 50,000 conversations/month, talk to us. Commit from $3,600/year. Adds 99.95% SLA, invoice/ACH, custom DPA, security review, named support, residency. Rate custom, never below 80% margin floor. No locked features.
- **Discounts:** OSS projects free to 2,000 conversations/month (Chatwoot free for OSS). Startups under 2 years and under $5M raised: $300 credits, 12 months (Featurebase 86% off, Productlane 89%, Plain 50%). Nonprofit/edu 50% (Crisp same).
- **Migration:** free importer from Fin/Intercom, Zendesk, Help Scout; history never billed (D11). Consider 3 months free up to 20,000 conversations for Fin/Intercom switchers during Salesforce transition.

---

## 9. Marketing angles

- **"No AI tax."** Fin $0.99, Productlane $0.79, Help Scout $0.75, Featurebase $0.49 per resolution. Dimmi $0; tokens at provider price.
- **"Same price, human or AI."**
- **"No seat tax."** Invite whole company.
- **"Read our meter."** Billing rule open source; every charge links to thread.
- **"Caps on by default."** Contrast Vercel/Netlify horror stories.
- **"Self-host free. Same code."**
- **"Intercom is now Salesforce."** Target nervous Fin customers with importer and calculator (paste Fin invoice, see Dimmi plus tokens).

---

## 10. Risks

- **Human-only price gap.** Gap now only at 50,000+ (Crisp $295 flat). Answer: custom contract once cost under $0.0012, self-host, target AI-forward teams.
- **Margin floor vs low bands.** Bands 2 and 3 below 80% margin at today's AWS list cost. Answer: cost reduction plan (section 3), or launch bands higher and lower after measuring.
- **"Penalty for growth" perception** (Help Scout lesson). Answer: low unit price, caps, graduated bands, never per message.
- **Definition gaming:** endless threads (30-day max), fake spam marks (25% review), bulk outbound (fair-use).
- **AI answering spam** costs user tokens plus Dimmi charge. Answer: classifier and bot check before AI; auto-credit.
- **BYOK friction** for non-technical buyers. Fine for developer launch; at-cost credits later.
- **AI price collapse.** Front already $0.05 per AI conversation (still above Dimmi band 1). "No AI tax" weakens if market hits cents; open source and no seats remain.
- **Prompt injection draining user key.** Budget caps, per-visitor limits, tool permissions.
- **Carrier cost shifts.** SMS/voice pass through. WhatsApp billed by Meta directly to business.
- **Fixed-cost burn** until about 90,000 billable conversations/month at blended $0.02 minus variable cost ($1,400 infra) [estimate]. Lower prices push break-even out; cost cuts pull it back.

---

## 11. Open questions and unverified items

- Unverified: Fin annual vs monthly seat split and $19 promo length; Productlane allowance per workspace or per user; Plain seat minimums and credit dollar value; Crisp extra AI credit price; Help Scout reversal date; Zendesk resolution tier platform (since 2026-05-18); Gorgias conflicting page prices ($77/$471/$1,227); RDS gp3, S3, CloudFront prices (memory, not page tables); Stripe Managed Payments fee and GA [3P]; OpenRouter BYOK fee wording.
- Measure: real LLM tokens per conversation; spam share of inbound.
- Decide: 7-day reopen window (vs Gorgias 3-day chat / 10-day email); free-tier badge.
- Test: $0.025 vs $0.03 vs $0.04 band 1. Free tier fixed at 100 (decided).
- Measure: fully loaded variable cost per conversation in first 3 months of cloud. Gates bands 2 and 3 and custom rate.

---

## 12. Sources

Competitor pricing and definitions
- https://www.intercom.com/pricing
- https://fin.ai/help/en/articles/13975800-fin-ai-agent-outcomes
- https://intercom.com/help/en/articles/8205718-fin-ai-agent-resolutions
- https://flexprice.io/pricing-index/intercom-fin [3P]
- https://dev.to/omar_bni_f6856a8bb0e021e9/intercom-fin-pricing-in-2026-the-real-math-behind-099-62j [3P]
- https://martech.org/salesforce-acquires-fin-formerly-known-as-intercom/
- https://www.sec.gov/Archives/edgar/data/0001108524/000110852426000190/crm-20260731.htm
- https://www.featurebase.app/pricing
- https://productlane.com/pricing
- https://crisp.chat/en/pricing/
- https://www.helpscout.com/pricing/
- https://docs.helpscout.com/article/1595-contact-based-fair-billing-policy
- https://bitsplitting.org/2025/04/30/whither-help-scout/
- https://kayako.com/tools/help-scout-review/ [3P]
- https://www.plain.com/pricing
- https://usepylon.com/pricing
- https://www.gorgias.com/pricing
- https://docs.gorgias.com/en-US/billable-tickets-199385
- https://www.getmacha.com/blog/gorgias-common-complaints-and-problems [3P]
- https://www.tawk.to/pricing/
- https://www.zendesk.com/pricing/
- https://support.zendesk.com/hc/en-us/articles/4408831235738
- https://front.com/pricing
- https://www.helpdesk.com/pricing/
- https://www.chatwoot.com/pricing
- https://www.tidio.com/pricing/
- https://www.chatbase.co/pricing
- https://developers.facebook.com/docs/whatsapp/pricing

Infrastructure and LLM cost
- https://aws.amazon.com/ses/pricing/
- https://aws.amazon.com/api-gateway/pricing/
- https://aws.amazon.com/ec2/pricing/on-demand/
- https://aws.amazon.com/rds/postgresql/pricing/
- https://instances.vantage.sh/aws/rds/db.m7g.large
- https://aws.amazon.com/s3/pricing/
- https://claude.com/pricing

BYOK
- https://vercel.com/docs/ai-gateway/authentication-and-byok/byok
- https://openrouter.ai/docs/use-cases/byok
- https://zed.dev/docs/ai/use-api-access
- https://forum.cursor.com/t/byok-got-blocked-in-0-limit-left/160612
- https://manual.raycast.com/ai/bring-your-own-key
- https://manual.raycast.com/ai/byoai
- https://docs.warp.dev/support-and-billing/plans-and-pricing/bring-your-own-api-key
- https://docs.typingmind.com/general-faqs
- https://www.mintlify.com/cline/cline/getting-started/authorizing-with-cline [3P mirror]
- https://docs.n8n.io/deploy/host-n8n/configure-n8n/basic-configuration/configuration-examples/set-a-custom-encryption-key
- https://www.chatwoot.com/hc/user-guide/articles/1755284287-how-to-enable-captain-on-self_hosted-installations

Usage pricing and OSS models
- https://costgoat.com/bill-shock/stories/vercel-cara-96k-week-bandwidth [3P]
- https://rosetta.to/u/t3dotgg/surprise-104-500-bill-breaking-down-a-big-mistake [3P]
- https://supabase.com/docs/guides/platform/cost-control
- https://supabase.com/docs/guides/platform/billing-faq
- https://posthog.com/pricing
- https://docs.railway.com/pricing/cost-control
- https://www.plushcap.com/content/neon/blog/neon-neons-new-pricing-explained-usage-based-with-a-5-minimum
- https://clerk.com/changelog/2026-02-05-new-plans-more-value
- https://resend.com/pricing
- https://plausible.io/self-hosted-web-analytics
- https://cal.com/blog/changing-to-agplv3-and-introducing-enterprise-edition
- https://raw.githubusercontent.com/chatwoot/chatwoot/develop/LICENSE
- https://blog.sentry.io/introducing-the-functional-source-license-freedom-without-free-riding
- https://www.activepieces.com/blog/n8n-license-fair-code-community-enterprise-2026 [3P]

Billing stack
- https://stripe.com/billing/pricing
- https://stripe.com/tax/pricing
- https://stripe.com/newsroom/news/stripe-completes-metronome-acquisition
- https://dodopayments.com/blogs/stripe-managed-payments-fees-explained [3P]
- https://polar.sh/resources/pricing
- https://www.paddle.com/pricing
- https://github.com/getlago/lago
- https://konghq.com/blog/news/kong-acquires-openmeter
- https://github.com/useautumn/autumn

Sibling research (same repo): research/competitors/featurebase.md, research/competitors/productlane.md, research/competitors/helpdesk.md.
