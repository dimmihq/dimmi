# Writing for dimmi

How humans and agents write every customer-facing word in this repository: the marketing site (`site/`), the documentation (`docs/`), the README, changelogs, and in-app microcopy.

dimmi speaks with **one human personality across two distinct modes**:

- **Marketing site (`site/`):** opinionated, direct, peer-to-peer, transparently honest about pricing and limits, with dry wit earned from real builder experience. Read [site/VOICE.md](site/VOICE.md).
- **Documentation (`docs/`):** warm, generous, example-first, built around clear mental models and concrete real-world scenarios, guiding readers smoothly from the happy path to advanced edge cases. Read [docs/WRITING.md](docs/WRITING.md).

Both surfaces run through the same **three-pass writing pipeline** using two local skills: [stop-slop](.claude/skills/stop-slop/SKILL.md) and [make-it-skim](.claude/skills/make-it-skim/SKILL.md).

## Before you write anything

Read the files that apply to the surface you are touching:

| You are changing | Read first |
| --- | --- |
| Marketing site (`site/`) | This file, [site/VOICE.md](site/VOICE.md), [stop-slop](.claude/skills/stop-slop/SKILL.md), [make-it-skim](.claude/skills/make-it-skim/SKILL.md) |
| Documentation (`docs/`) | This file, [docs/WRITING.md](docs/WRITING.md), [stop-slop](.claude/skills/stop-slop/SKILL.md), [make-it-skim](.claude/skills/make-it-skim/SKILL.md), and the template in `docs/templates/` |
| README, changelog, or release notes | This file, [site/VOICE.md](site/VOICE.md), [stop-slop](.claude/skills/stop-slop/SKILL.md), [make-it-skim](.claude/skills/make-it-skim/SKILL.md) |

## What dimmi is (canonical product truth)

Every page you write rests on what dimmi actually builds and stands for. Ground your claims in real product behavior, never vague category slogans.

**The house thesis:** Customer support you own.

dimmi is an **open-source (MIT-licensed)** customer support platform that teams run on **dimmi Cloud** or **self-host on a single Linux server** with one command. Both editions run the exact same codebase with zero features locked behind an enterprise paywall.

What dimmi replaces: the brittle stack of per-seat helpdesks, $0.99-per-resolution AI add-ons, SSO taxes, separate feedback boards, and duct-taped webhook glue.

What dimmi gives a team in one place:

- **Shared inbox:** messenger, email, Slack, WhatsApp, Instagram, Facebook Messenger, SMS, phone, and REST API threads in one queue, with customer context, internal notes, assignment rules, SLAs, and saved replies.
- **dimmi agent:** an AI agent that answers customers from your help center and knowledge sources, follows plain-English **procedures**, calls your backend through **actions** with optional customer confirmation or teammate approval, and hands over with a summary when a human should step in.
- **Operator:** an internal AI teammate that lives beside the inbox and in Slack, briefs teammates on long threads, drafts replies, and proposes workspace edits as reviewable, undoable **changesets**.
- **Tickets and trackers:** structured customer tickets, internal back-office escalations, multi-customer incident trackers, business-hour SLAs, and links to Linear, Jira, GitHub, incident.io, Rootly, and PagerDuty.
- **Workflows:** visual automation triggered by messages, events, tags, SLA risks, or schedules, with API request steps and full step-by-step run logs.
- **Help center, changelog, feedback, and broadcasts:** public or restricted docs on your custom domain, release notes in the messenger and inbox, linked feature request boards and roadmaps, and multi-channel broadcasts and series whose replies land right back in the inbox.
- **Developer platform:** a versioned REST API (`/api/v1`), signed webhooks, OAuth apps, an MCP server (`/mcp`), `llms.txt`, and typed SDKs for TypeScript, Go, PHP, Laravel, Python, browser, React, Next.js, Vue, Angular, and Svelte.
- **Transparent economics:** **unlimited free seats** on dimmi Cloud, **100 free conversations every month**, graduated conversation pricing that drops as volume grows, and **BYO model keys** so AI conversations drop to the human rate. Self-hosting costs $0 to dimmi.

## Who we are talking to

Write for two real people sitting at real desks, never an abstract "organization" or "enterprise buyer":

1. **Technical founders and product engineers.** They inspect pricing calculators, read source code, and spot corporate spin in two seconds. They hate per-seat taxes that punish them for letting engineers answer bug reports, and they distrust AI vendors who charge per resolution while hiding how the model works.
2. **Support leads and operations managers.** They live in the queue every day. They care about response times, clear escalation paths, clean mental models, and an AI agent they can test safely before a customer ever sees a reply.

Both readers are smart, busy, and allergic to fluff. Treat them like capable peers.

## Two surfaces, two voices

Marketing and documentation share the same respect for the reader, the same factual honesty, and the same hatred of AI slop. Their jobs on the page are different, so their voices intentionally diverge:

| Dimension | Marketing site (`site/VOICE.md`) | Documentation (`docs/WRITING.md`) |
| --- | --- | --- |
| **Job on the page** | Earn attention, name the pain, take a stance, and earn trust | Build mental models, get readers to a working result fast, and answer edge cases without guesswork |
| **Tone** | Smart friend who built this and happens to sell it; opinionated, candid, dry wit | Calm senior teammate sitting beside you; warm, patient, generous, reassuring |
| **Humor** | Earned, dry, specific, sparingly placed | Almost none; clarity and calm warmth replace jokes |
| **Structure** | Varied editorial cadence across sections; never clone the same choppy rhythm | Progressive disclosure: real-world intro, happy path with real example, customization, rules and limits |
| **Length** | Tight, high-signal blocks; zero walls of text | As complete as the topic needs to make the reader succeed on the first try |
| **Fact bar** | Honest about capabilities, prices, competitors, and limits | 100% verified against shipped code, contracts, and UI labels |

## The three-pass writing pipeline

Every page goes through three distinct passes, in this exact order. Never compress them into one hurried generation pass.

### Pass 1: Draft for truth and voice

- Gather real facts from the codebase first (`site/src/pricing/`, `routes/*.go`, `config/*.go`, `app/`, `tests/contracts/`, `resources/js/`).
- Draft in the target voice: opinionated, peer-to-peer clarity for `site/`, or warm, example-first teaching for `docs/`.
- Focus on real customer problems, concrete scenarios, and varied human sentence rhythm.

### Pass 2: Human polish (`stop-slop`)

Run the draft through [.claude/skills/stop-slop/SKILL.md](.claude/skills/stop-slop/SKILL.md) and its reference lists (`phrases.md`, `structures.md`, `examples.md`):

- Strip throat-clearing openers ("Here's the thing:", "It turns out", "The truth is").
- Break formulaic AI contrasts ("Not X, it's Y", "X isn't the problem. Y is.", "Not a X. Not a Y. A Z.").
- Eliminate false agency ("the complaint becomes a fix", "the data tells us") and passive voice that hides the actor.
- Kill vague declaratives ("The implications are significant"), meta-commentary ("In this section, we'll..."), and corporate buzzwords.
- Break metronomic staccato rhythm: vary sentence lengths so paragraphs breathe like real speech.
- Score the draft on the 50-point `stop-slop` rubric (Directness, Rhythm, Trust, Authenticity, Density). Anything below **40/50** gets rewritten.

### Pass 3: Scannability (`make-it-skim`)

Run the finished prose through [.claude/skills/make-it-skim/SKILL.md](.claude/skills/make-it-skim/SKILL.md):

- **Break walls:** keep paragraphs to 1 to 3 short sentences, one idea per paragraph, separated by white space.
- **Layer-Cake headings:** write descriptive headings that tell a scanning reader what lives underneath. Never label an opening paragraph with a redundant heading.
- **Front-load bullets:** put the point in the first three words, and never open consecutive bullets with the same word.
- **Bold anchors only:** bold UI labels, coined terms, list lead-ins, and load-bearing numbers or limits (at most one anchor per prose paragraph). Never bold whole sentences or ordinary prose.
- **Digits for weight:** write load-bearing numbers, limits, prices, and timings as digits (`100 free conversations`, `7 days`, `1.5 seconds`, `30 steps`).
- **Scale the touch:** do not bullet a marketing story or drag a persuasive punchline out of order. Leave already-skimmable copy alone.
- **Run all three tests:** the **Skim test** (first lines + bold anchors + bullet openings carry the message), the **Difference test** (zero invented facts or weakened caveats), and the **Leave-alone test**.

## Binding tie-breakers

Whenever voice warmth, editorial style, `stop-slop`, and `make-it-skim` appear to pull in opposite directions, follow these binding rules:

1. **Warmth without filler adverbs.** Warm documentation comes from clear mental models, concrete scenarios, and anticipating the reader's next question, never from filler words like "simply", "easily", "thankfully", or "blazing speed". Keep functional adverbs only when they carry exact technical meaning (`automatically`, `immediately`, `synchronously`, `only`, `by default`).
2. **No em dashes anywhere.** Use a period, comma, colon, or parentheses instead. Never use `—` or `–` as a clause break in prose.
3. **Varied rhythm beats clipped minimalism.** Never split every sentence at a second comma or stack 4-word fragments back to back. Mix short, medium, and occasional longer sentences so the writing sounds like a thoughtful human talking, not a clipped bot.
4. **Bold anchors, not bold walls.** Bold interface labels (**Settings → Webhooks**), list lead-ins, and genuinely load-bearing terms or limits (at most one per prose paragraph). If everything is bold, nothing is.
5. **Digits for concrete facts.** Use digits for every price, limit, timeout, percentage, count, and duration that a reader scans for (`3 steps`, `7 days`, `100 free conversations`, `5% bounce rate`). Spell out casual non-anchor counts from one to nine in running prose only when the number is incidental ("a couple of minutes", "two teams").
6. **Active voice by default, exact truth always.** Put the real actor at the front of the sentence ("dimmi retries 12 times over 24 hours"). Use passive voice in docs only when the actor is genuinely irrelevant and passive voice states the system rule more clearly.

## Canonical terminology

Use the exact term in the first column across both `site/` and `docs/`. When the REST API uses a different field or resource name, introduce the API name once in inline code: "a customer (`contact` in the API)".

| Use | Avoid | Meaning |
| --- | --- | --- |
| dimmi | Dimmi, DIMMI | The product. Always lowercase, even at the start of a sentence. |
| workspace | organization, account, team | One company's dimmi environment: its inboxes, customers, settings, and members. |
| customer | user, visitor, end user | A person who writes to your team (`contact` in the API). A visitor is a customer not yet identified. |
| teammate | agent, operator, staff, user | A person on your team who answers customers. Use **member** only for workspace membership, roles, and invitations. |
| conversation | chat, thread, ticket | One exchange with a customer on any channel. |
| inbox | queue, mailbox | A container conversations route into, with its own team and assignment rules. |
| view | filter, folder | A saved conversation list such as **My inbox**, **Unassigned**, or a custom view. |
| ticket | case, issue | Tracked work with a customer-visible state. Kinds: customer ticket, back-office ticket, tracker. |
| tracker | incident, master ticket | A ticket grouping one shared outage or bug reported by many customers. Not Linear or Jira. |
| linked issue | tracker, external ticket | A ticket's link to Linear, Jira, or GitHub. |
| messenger | widget, chat bubble, live chat | The chat embedded on your website or app. Its HTTP API is the Messenger API. |
| dimmi agent | bot, AI assistant, Fin | The customer-facing AI that answers from your knowledge, runs procedures, and hands over. |
| Operator | copilot, assistant | The internal AI your team chats with inside dimmi and Slack. Proposes edits as changesets. |
| handover | handoff, escalation, transfer | The dimmi agent passing a conversation to a teammate. |
| saved reply | macro, canned response, snippet | Reusable reply text with optional variables and actions. |
| office hours | business hours, opening hours | Workspace schedules used by SLAs and the dimmi agent. |
| SLA policy | SLA rule | Reply and resolution time targets for conversations and tickets. |
| workflow | automation, rule, flow | A visual automation with a trigger, conditions, and steps. |
| procedure | playbook, runbook | Plain-English instructions the dimmi agent follows for a specific kind of request. |
| action | tool, integration, function | An HTTP or MCP call the dimmi agent (or Operator) makes to your own systems. |
| knowledge source | data source, content source | Where the dimmi agent learns answers: help center, websites, files, GitHub, Notion, Intercom, Zendesk. |
| help center | knowledge base, docs site | Your public or restricted customer articles, collections, and search. |
| feature requests | feedback, feedback board, roadmap | The public board where customers post and vote on ideas (`feedback-*` in the API). |
| broadcast | campaign, blast | A one-off outbound message sent to an audience. |
| series | journey, drip, sequence | An automated, multi-step outbound message sequence. |
| API token | API key, personal access token | The credential for the REST API. Permissions are **abilities** (OAuth calls them scopes). |
| identity verification | user hash, HMAC | Server-signed proof of a customer's identity in the messenger. |
| sign in, sign-in | log in, login | The verb and the noun. |
| dimmi Cloud, self-hosted | SaaS, on-prem, enterprise | The two ways to run dimmi. |

## Reading level and paragraph length

Write in plain, everyday English at a **Grade 5 reading level or lower**. Our readers are smart, busy people, and many read English as a second language. Simple words read faster and feel more human than fancy ones.

- **Keep technical nouns exact, simplify everything else.** Use the real product or engineering word when it is the thing itself (`webhook`, `SAML SSO`, `Postgres`, `API token`), and use plain Grade 4 to 5 words around it.
- **Short paragraphs always.** Keep prose paragraphs to **1 to 2 short sentences** (roughly 20 to 35 words, 2 to 3 lines on screen). If a paragraph hits 4 lines, split it or cut words.
- **One idea per sentence.** Long multi-clause sentences with three commas make readers work too hard.
- **Avoid stacked hyphenated modifiers before nouns.** Turning a phrase into a hyphenated adjective before a noun (*"one MIT-licensed codebase"*, *"plain-English rules"*, *"step-by-step playbooks"*) makes text clunky and slow to read. Put the detail naturally after the noun instead (*"all open source under MIT"*, *"rules in plain English"*, *"playbooks step by step"*).

Swap big words for plain words:

| Instead of | Write |
| --- | --- |
| deterministic | fixed, same every time |
| irreversible | cannot undo |
| coherent | clear, single |
| eligibility | whether it can reply |
| lifecycle | what happens, how a message moves |
| guardrails | safety rules, limits |
| hallucinate | guess, make things up |
| grounded (in docs) | backed by your docs, from your docs |
| graduated usage bands | rates that drop as you grow |
| pre-publish validation | checks before it goes live |
| atomic updates | safe edits |
| configure | set up |
| verify, validate | check |
| invoke | call, run |
| disrupting | breaking |
| improvising | guessing |

## Shared mechanics

- **Spelling and punctuation:** American English, Oxford comma, straight quotes (`'` and `"`).
- **Capitalization:** sentence case for every heading, subhead, tab, and button. `dimmi` is always lowercase; `Operator` is capitalized.
- **No emojis:** plain English carries more weight and ages better.
- **No exclamation marks:** except inside a deliberate parody on the marketing site (and almost never even there).
- **Hyphenation:** "open-source" before a noun ("an open-source helpdesk"); "open source" elsewhere ("dimmi is open source").
- **No `i.e.,` or `e.g.,` clutter:** prefer plain English ("such as", "meaning"). When using `e.g.` or `i.e.`, omit the trailing comma.
