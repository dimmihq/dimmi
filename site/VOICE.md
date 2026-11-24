# Writing the dimmi marketing site

How to write every page on `trydimmi.com` (`site/`). Before drafting or editing any page, read the root [WRITING.md](../WRITING.md), [stop-slop](../.claude/skills/stop-slop/SKILL.md), and [make-it-skim](../.claude/skills/make-it-skim/SKILL.md).

Our marketing voice is radically candid, opinionated, technically literate, transparently generous about pricing, and genuinely human. Every page should read like a thoughtful builder talking plainly to a peer, never like a SaaS template.

## Why our previous site copy fell flat (and what to stop doing)

Before writing a single new headline, understand the trap our first draft fell into. Reading the old copy in `site/src/` reveals four clear failure modes:

1. **Hemingway-bot staccato rhythm.** Nearly every headline and card followed the exact same two-beat AI cadence: _"Snooze it. It comes back."_ / _"It does the work. You set the rules."_ / _"Write it down once. The agent does it."_ / _"Pick, write, send."_ / _"Trigger, check, act."_ / _"You edit it. You send it."_ Stacking short clipped fragments across 12 pages sounds robotic and exhausted, not confident.
2. **Generic feature labels posing as headlines.** Lines like _"One queue for every conversation."_, _"Automate the dull support work."_, _"Turn a conversation into tracked work."_, and _"See how support is actually going."_ could sit on Intercom, Zendesk, Help Scout, or any generic helpdesk without changing a word. They have no point of view and no tension.
3. **Zero customer pain or enemy.** The old copy described buttons and features in a vacuum instead of naming what actually drives founders and support teams crazy: paying $85 a seat so an engineer can read one bug report, getting hit with $0.99 per AI resolution on top of seat fees, paying an SSO tax for basic security, or stitching five separate tools together just to close a feedback loop.
4. **Mechanical template tics.** Over 80% of FAQ answers opened with `"No."` or `"Yes."`, and every product page followed the exact same cadence from top to bottom. When a reader notices the template, the human disappears.

## The core philosophy

> **Write the way you would explain dimmi to a smart friend who runs a software company, not a prospect you are trying to close.**

Five principles anchor every marketing page we write:

### 1. Taste over polish

"Polish" is surface-level SaaS sheen: smooth gradients, trendy verbs, and buzzword-compliant headlines that say nothing. **Taste** means having a real point of view, caring about whether a sentence is true and sharp rather than merely "done", and writing copy that looks like a thoughtful human chose every word on purpose.

### 2. Be opinionated

We would rather have half of visitors love our stance and the other half realize dimmi is not for them than have everyone nod politely and forget our name ten seconds later. Take clear positions on how support software should work:

- Charging per seat punishes teams for putting engineers and founders close to customers.
- Charging $0.99 per AI resolution on top of seat fees turns automation into a margin trap.
- Locking SAML SSO, audit logs, and basic workflows behind an "Enterprise" sales call is hostile.
- Open source means MIT-licensed code with every feature included, not a crippled "Community Edition" bait-and-switch.
- AI agents should show their sources, follow plain-English procedures, and ask a human before refunding money or mutating accounts.

### 3. Compete with the internet, not boring B2B SaaS

When someone lands on `trydimmi.com`, you are competing with Hacker News, X, Slack, and their open IDE, not just Zendesk's landing page. If a headline puts you to sleep, delete it. Lead with concrete stakes, recognizable Tuesday-morning pain, or an unexpected truth.

### 4. Pull, don't push (no sneaky shit)

Our audience builds software for a living. They distrust marketing by default, view source code for fun, and smell hype from three scrolls away.

- Put prices and the free tier on the page before any pitch.
- Tell readers how to pay us less (BYO model key, MCP instead of Operator, or self-hosting for $0).
- Admit who should _not_ use dimmi. Honesty about tradeoffs builds more trust than ten testimonials.
- Name competitors (Intercom, Zendesk, Help Scout, Front, Freshdesk) and treat them fairly. Concede what they do well, then show the math.
- Keep everything self-serve. Never gate pricing, docs, or sign-up behind a "Book a demo" wall.

### 5. Pass the Hacker News test

Before you ship any headline, section, or pricing claim, ask: **How would this land on the front page of Hacker News?** Skeptical engineers will tear apart vague claims, hidden catches, and try-hard humor in seconds. If a line would get called out in the comments, rewrite it until it holds up under scrutiny.

## Brand personality

| Feel like this                                                                               | Never like this                                                |
| -------------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| **Opinionated** (takes a clear stance on support software)                                   | Diplomatic to the point of saying nothing                      |
| **Human** (writes in natural spoken rhythms with contractions)                               | A corporate brochure or clipped Hemingway-bot                  |
| **Dry and slightly roguish** (understated wit grounded in truth)                             | Try-hard "fellow kids" memes or wacky SaaS metaphors           |
| **Specific** (names real tools, numbers, and Tuesday-morning problems)                       | Abstract claims ("actionable insights", "seamless experience") |
| **Honest** (admits limits, edges, and when another tool fits better)                         | Spin, evasion, or pretending to be everything for everyone     |
| **Generous** (gives away the MIT codebase, 100 free conversations, and tips to shrink bills) | Nickel-and-diming or hiding the catch in footnotes             |
| **Peer-to-peer** (engineer-to-engineer, builder-to-support-lead)                             | Vendor pitching downward or enterprise sales-speak             |

## Positioning playbook (unique beliefs per product)

Do not describe products as feature checklists. That is what `docs/` is for. Every marketing page starts from a **unique belief** about why the status quo is broken and what dimmi does differently.

| Page                               | The broken status quo                                                                                                                                                                                                                       | dimmi's unique belief and angle                                                                                                                                                                                                              | Honest boundary (who it isn't for)                                                                                           |
| ---------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- |
| **Home (`/`)**                     | Support stacks punish growth: per-seat taxes keep engineers out of the inbox, AI costs $0.99 a resolution, and your data lives in a closed box.                                                                                             | One MIT-licensed support platform you can run on our cloud or your own Linux box. Unlimited seats, transparent conversation pricing, and an AI stack you control.                                                                            | Enterprise call centers needing power dialers, shift scheduling, or 200-seat phone floors.                                   |
| **Inbox (`/inbox`)**               | Support lives in one tab, customer billing in another, product events in a third, and engineering is locked out because seats cost $85 each.                                                                                                | Every channel lands in one fast queue with full customer context beside the thread, and everyone on your team can jump in for free.                                                                                                          | Teams running a traditional high-volume call center with wallboards and shift rosters.                                       |
| **Agents (`/agents`)**             | Teams either cram frontline support, billing refunds, technical docs, and multiple brands into one overloaded agent, pay Chatbase extra per widget without a helpdesk, or pay Intercom $132/seat for multi-brand plus $0.99 per resolution. | Dedicated support agents for every queue, policy, and brand locked to their own docs, procedures, and actions, transferring threads quietly inside one shared inbox.                                                                         | Regulated workflows where zero AI should ever touch a customer thread.                                                       |
| **Agent Studio (`/agent-studio`)** | Teams tweak support prompts in production and hope nothing breaks, or trust a single green test run that hides flaky replies.                                                                                                               | One workspace to write guidance linked with `@` chips, catch flakiness across multi-run simulations, compare Draft against Live side by side, and let Operator diagnose failures with one-click draft fixes.                                 | Teams that do not run customer-facing support agents yet.                                                                    |
| **Procedures (`/procedures`)**     | Teams either stuff 12-page prompts into a single text box or build fragile 40-node flowcharts every time a refund policy changes.                                                                                                           | Support policies your AI agent cannot break: resolve multi-step refunds, cancellations, and account changes while enforcing hard limits, testing safely before launch, keeping human sign-off on risky steps, and rolling back in one click. | Fixed automations that should run with zero AI involvement (use Workflows for those).                                        |
| **Operator (`/operator`)**         | Support tools bolt on pricey per-seat "copilots" that only spit out generic reply text and cannot touch your workspace safely.                                                                                                              | A backstage AI teammate that briefs you on 40-message threads, answers workspace questions in the inbox or Slack, and proposes real changes as undoable changesets.                                                                          | Teams looking for a customer-facing agent (that is Agents).                                                                  |
| **Routines (`/routines`)**         | Help centers rot after every product release, failing policies go unnoticed, and rigid background bots spam Slack every morning with empty alerts.                                                                                          | Operator on a clock: reads merged GitHub PRs, unanswered chats, and slipping policies, leaves inline diffs inside your Help Center and Procedure editors, stays quiet when nothing changed, and never publishes without your click.          | One-off real-time ticket triggers (use Workflows for instant event rules).                                                   |
| **Compare (`/compare`)**           | Comparison pages cherry-pick competitor tiers and hide real monthly bills behind sales calls.                                                                                                                                               | Candid side-by-side customer support comparison against Helply, Chatbase, and Intercom Fin showing capabilities and real monthly support math.                                                                                               | Buyers wanting a feature-by-feature RFP spreadsheet for 500-seat call centers.                                               |
| **Tickets (`/tickets`)**           | Conversations blur into untracked backlogs, bugs vanish into Slack threads, and 50 customers hit by one outage get 50 manual updates.                                                                                                       | Separate the conversation from the tracked work behind it. Link engineering issues in Linear, Jira, or GitHub, group outages under one tracker, and update every affected customer once.                                                     | Teams expecting a full engineering project tracker to replace Linear or Jira.                                                |
| **Workflows (`/workflows`)**       | Routine routing, SLA warnings, and follow-ups depend on teammates remembering checkboxes, or require upgrading to a $132/seat enterprise tier.                                                                                              | Visual automation included for every workspace, from simple templates to API-calling branches, with a full step-by-step run log when you need to debug what ran at 3am.                                                                      | General cross-app glue like syncing a CRM to Google Sheets (keep Zapier or Make for that).                                   |
| **Help center (`/help-center`)**   | Help centers rot in a separate tool that drifts out of sync with what the AI bot says and forces customers to guess exact search keywords.                                                                                                  | Publish docs on your own domain that serve both humans and the AI agent from one source of truth, with Markdown copies ready for AI tools and stale-translation alerts.                                                                      | Versioned developer API docs with OpenAPI generators (use a dedicated docs framework).                                       |
| **Broadcasts (`/broadcasts`)**     | Outbound customer emails go out from `no-reply@` on a shared domain, turning a simple incident notice into a dead end.                                                                                                                      | Send announcements and behavioral series from your own verified domain across email, messenger, Slack, SMS, and WhatsApp, where every reply opens a real conversation in your inbox.                                                         | Top-of-funnel marketing automation with landing pages, ad audiences, and lead scoring.                                       |
| **Changelog (`/changelog`)**       | Release notes get buried on a forgotten blog nobody checks, so customers keep asking if a feature shipped three months ago.                                                                                                                 | Publish once and reach customers on your help center, inside the messenger tab with an unread dot, and by email, without ever emailing subscribers twice when you fix a typo.                                                                | Teams that want a standalone changelog without setting up a help center.                                                     |
| **Feedback (`/feedback`)**         | Feature requests live in a spreadsheet nobody updates, and customers who begged for a feature never hear when it finally ships.                                                                                                             | Connect customer votes, live support conversations, your public roadmap, and your changelog so every voter automatically hears the moment you ship what they asked for.                                                                      | Teams needing automatic two-way sync of raw feature ideas into Jira without webhooks or API glue.                            |
| **Reports (`/reports`)**           | Dashboards throw opaque averages at managers who cannot click through to see which conversations actually broke the SLA.                                                                                                                    | Nine practical support reports plus custom builders where every single number opens the exact conversations behind it, respecting role permissions and business hours.                                                                       | SQL-heavy BI warehouses joining billing tables against product telemetry (export CSV or use the REST API).                   |
| **Pricing (`/pricing`)**           | Helpdesk pricing pages hide true costs behind four tiers, per-seat multipliers, gated workflows, and surprise AI resolution fees.                                                                                                           | One transparent price book: unlimited seats, 100 free conversations every month, graduated volume bands, spend caps, and three honest ways to pay us even less.                                                                              | Very small teams (e.g., 2 seats answering 2,000 manual threads with basic needs) who may pay less on an entry per-seat plan. |

## Writing mechanics and rhythm

### Keep blocks short and below Grade 6

A landing page is scanned, never studied like a textbook. Too much text in one box kills the page.

- **Hero and section subheads:** 1 to 2 short sentences (**max 25 to 30 words**, 2 to 3 lines on desktop). Never let a centered subhead stretch to 4 or 5 lines.
- **Feature cards, steps, and narrow rows:** 1 to 2 short sentences (**max 18 to 25 words**, 2 to 3 lines max). Match line counts across sibling items in a horizontal row so labels and values sit on the same visual baseline.
- **FAQ answers:** 1 to 3 short sentences (**max 45 words**).
- **Grade 5 English or lower:** use short, everyday words. Write _"make things up"_ instead of _"hallucinate"_, _"rates that drop as you grow"_ instead of _"graduated usage bands"_, and _"things you cannot undo"_ instead of _"irreversible actions"_. See the plain-word table in [WRITING.md](../WRITING.md).
- **Keep spec limits in the docs:** never mention character limits (`2,000 characters`), step caps, or internal settings in marketing copy. Numbers on the marketing site must matter to a buyer's wallet, time, or choice (prices, free tier, `%` savings, setup time, SDK count).

### Kill the staccato reflex

Short does not mean choppy. Vary sentence lengths naturally so a 2-sentence block reads like a real person talking, not a clipped bot.

- **Mix lengths inside every block:** pair a crisp 7-word sentence with a smooth 14-word sentence.
- **Never clone the same headline pattern twice on one screen:** alternate direct claims, blunt buyer questions, and sharp observations so the page feels written by a person, not stamped from a template.

### Paint real Tuesdays, not abstract capabilities

Engineers ignore vague benefit claims and remember concrete situations.

- Show the real moment: _"An engineer shouldn't cost $85 a month just to read one bug report and leave an internal note."_
- Name the real number: _"The first 100 conversations every month are free, with unlimited seats and every feature unlocked."_
- Name the real mechanism: _"When the Linear issue closes, dimmi drafts the update for every customer linked to that tracker. Your team reviews it and clicks send."_

### Where wit belongs (and where it hurts)

Humor in dimmi copy works only when it is **dry, specific, earned from real industry absurdity, and immediately followed by the real fact**.

- **Good places for a dry aside:** a section heading (_"How to give us even less money"_), a caption or subtext, a pricing footnote, or a single honest line in an FAQ.
- **Bad places:** primary navigation, error states, critical pricing math, or stacked back-to-back across every card on a page.
- **Budget:** at most one dry wink per section. When in doubt, cut the joke and say the true thing more vividly. Clear beats clever every time.

### Headlines, subheads, features, and FAQs

- **Hero headlines:** earn attention in 5 to 11 words. State the core truth, tension, or payoff, never a bland category label. Follow with a 1-to-2 sentence subhead that names what the product actually is and why our approach is different.
- **Section titles:** alternate between concrete outcomes, blunt buyer questions (_"What actually counts as a conversation?"_, _"Why not just self-host it?"_), and sharp positioning statements.
- **Feature cards and rows:** pair a concrete action or mechanism with the real-world consequence. Never pad with `"helps you to"` or `"allows you to"`.
- **FAQs:** write the exact skeptical questions a smart buyer would ask in Slack before signing up. Vary your openings naturally: open with a direct answer, a nuance, or a straight verdict (_"Not from us."_, _"Only when you turn it on."_, _"Sometimes, on paper."_), never 6 robotic `"No."` openers in a row. Include at least one honest boundary question per product page naming who should pick another tool.
- **CTAs:** literal, zero-pressure, and specific about cost or friction (_"Start free, no card"_, _"Self-host on GitHub"_, _"Read the docs"_, _"Calculate your bill"_).

### Applying `make-it-skim` to marketing pages

Follow `make-it-skim`'s **"Scale the touch"** guidance:

- Use **digits** for every price, count, tier, and limit (`100 free conversations`, `$0.99`, `11 SDKs`, `7 days`) so scanning eyes lock onto the proof immediately.
- **Front-load** every bullet and feature card with the load-bearing noun or verb in the first 3 words, and never start consecutive items with the same opening word.
- Keep cards and rows to **1 to 3 short sentences**, one clear idea per block.
- **Do not** flatten a persuasive hero, a pricing story, or a witty aside into robotic bullets. Preserve the rhythm and build where tone carries the point.

## Real dimmi before/after rewrites

Study these transformations taken directly from our existing `site/src/` pages. Notice how the "After" versions keep 100% of the factual truth while replacing clipped AI staccato with human voice, real stakes, and natural rhythm.

### 1. Homepage Inbox section (`site/src/sections/inbox.tsx`)

**Before (clipped, generic feature brochure):**
> **One queue for every conversation.**
> Your team and the AI agent work the same threads, with the customer and their history beside every reply.
> - _The customer beside the thread:_ Company, plan, product events, and past conversations sit next to every reply. No second tab.
> - _Workflows you draw:_ Routing, SLAs, and follow-ups on a canvas. Start from a template or a blank one.

**After (human, grounded in real pain, varied rhythm):**
> **Every customer channel in one inbox your whole company can actually open.**
> Messenger, email, Slack, WhatsApp, SMS, and phone threads land in the same queue, with the customer's plan, product events, and past conversations sitting right next to the composer. And because seats are free, you can tag an engineer into a bug thread without upgrading your billing tier.
> - **Full customer context, zero tab-hopping:** Plan tier, company details, recent product events, and past tickets load beside every reply automatically.
> - **Visual workflows without the enterprise tier:** Draw routing rules, SLA escalations, and follow-ups on a canvas, or start from 8 ready-made templates.

### 2. Agents Hero & Procedures (`site/src/pages/agents.tsx`, `site/src/pages/procedures.tsx`)

**Before (metronomic two-beat AI cadence):**
> **Write it down once. The agent does it.**
> A procedure is your process in plain words, like how to refund a duplicate charge. The agent follows it step by step and asks before the risky parts.
> **It does the work. You set the rules.**

**After (clear POV, real mechanism, human cadence):**
> **Teach the AI agent your real support playbooks in plain English.**
> Most support bots fall apart the moment a customer asks for something beyond a help article. With procedures, you write out how your team handles a request, such as refunding a duplicate charge, step by step. The agent checks your conditions, calls your API, and pauses for teammate approval before anything irreversible runs.

### 3. Workflows Hero & FAQ (`site/src/pages/workflows.tsx`)

**Before (generic headline + robotic FAQ):**
> **Automate the dull support work.**
> A workflow waits for a trigger, checks your conditions, then acts. It routes, replies, escalates, or hands the conversation to the agent.
> _Q: So can I cancel Zapier?_
> _A: No. Workflows are about conversations and customers. They route, reply, escalate, and call your API. If you need your CRM to update a spreadsheet, keep the tool built for that._

**After (opinionated, specific, natural FAQ):**
> **Automate triage, SLAs, and follow-ups without paying for an Enterprise tier.**
> Build routing and escalation flows on a visual canvas, trigger them from messages, product events, or schedules, and inspect a full step-by-step run log whenever you want to see what fired at 3am.
> _Q: So can I cancel Zapier?_
> _A: Probably not, unless you only used Zapier to glue your helpdesk together. dimmi workflows route conversations, trigger SLAs, create tickets, and call your API, but they aren't built to sync your CRM with a spreadsheet. Keep the right tool for that._

### 4. Tickets Hero (`site/src/pages/tickets.tsx`)

**Before (flat category description):**
> **Turn a conversation into tracked work.**
> Every request gets a type, a state, and an owner. Send it to the right team, link the engineering work, and tell the customer where it stands.

**After (specific tension + vivid payoff):**
> **Stop losing bug reports and billing escalations inside long chat threads.**
> Convert any conversation into a structured ticket with AI-filled fields, link the underlying Linear, Jira, or GitHub issue, and group everyone hit by the same outage under one tracker. When engineering closes the issue, dimmi drafts the update so every affected customer hears back in one go.

### 5. Self-hosting section (`site/src/sections/self-host.tsx`)

**Before (staccato fragments):**
> **Open source. Actually open.**
> MIT license, no enterprise edition held back. Run it on our cloud or your servers, and build on it with the SDKs and the API.

**After (opinionated, concrete, HN-ready):**
> **MIT-licensed open source, with zero "Enterprise Edition" paywalls.**
> The self-hosted build runs the exact same code as dimmi Cloud: SAML SSO, AI procedures, workflows, tickets, and all 11 SDKs included. Spin it up on a single Linux server with one command, keep every byte in your own Postgres database, and pay us $0.

## Never write or do

- **Banned SaaS buzzwords:** leverage, utilize, streamline, robust, seamless, best-in-class, holistic, synergy, unlock, empower, supercharge, revolutionize, game-changing, cutting-edge, next-generation, world-class, "AI-powered" as a standalone pitch, "helps you to", "allows you to", "all-in-one platform for modern teams".
- **Banned AI structures (see `stop-slop`):** "Not X, it's Y" binary contrasts, "X isn't the problem. Y is.", dramatic 2-word fragment stacks, throat-clearing openers ("Here's the thing:"), em dashes (`—`), and vague declaratives.
- **Never fabricate:** do not invent customers, benchmarks, percentages, guarantees, or unshipped features to make a line punchier. Never alter a price or limit to land a joke.
- **Never hardcode growing catalog counts:** avoid putting fixed counts on things we continuously add to (such as workflow templates) so shipping new ones never stales the marketing copy.
- **Never punch down:** treat competitors fairly (acknowledge what they do well) and never mock customers or support teams.
- **Never mention internal margins or costs.**
- **Never borrow another company's slogans or tropes.** Write dimmi's own words from our own product and perspective.

## Pre-ship quality gate

Before finishing any marketing page or section, check every item:

- [ ] **Hacker News test:** Would a skeptical engineer respect every claim, or call out spin or try-hard humor?
- [ ] **Smart-friend test:** Read the copy out loud to a friend who runs a support or product team. Does it sound like natural human speech?
- [ ] **Short and simple check:** Hero and section subheads stay under 30 words (2 to 3 lines max), cards stay under 25 words, English stays below Grade 6, and zero internal spec limits (like character caps) leaked onto the page.
- [ ] **Anti-staccato check:** Are sentence lengths genuinely varied? Did you eliminate repetitive `[Short clause]. [Short clause].` headline/card clones?
- [ ] **Point-of-view check:** Could Zendesk or Intercom paste this headline onto their homepage unchanged? If yes, rewrite it until it unmistakably belongs to dimmi.
- [ ] **Honesty check:** Are prices, limits, and competitor numbers verified against `site/src/pricing/` and the codebase? Does every product page honestly state who it is _not_ for?
- [ ] **`stop-slop` pass:** Zero banned phrases, binary contrasts, false agency, or em dashes; rubric score 40/50 or higher.
- [ ] **`make-it-skim` pass:** Digits on load-bearing numbers, front-loaded bullets with varied openings, short scannable blocks, zero walls of text.
- [ ] **Terminology check:** Product terms match [WRITING.md](../WRITING.md) (`dimmi` lowercase, `Operator` capitalized, American English, Oxford comma).
