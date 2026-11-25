# Writing the dimmi documentation

How to plan, structure, and write every page in `docs/` (served at `trydimmi.com/docs`, plus `/docs/<page>.md`, `/docs/llms.txt`, and `/docs/llms-full.txt`). Before writing or editing any page, read the root [WRITING.md](../WRITING.md), [stop-slop](../.claude/skills/stop-slop/SKILL.md), and [make-it-skim](../.claude/skills/make-it-skim/SKILL.md).

Our documentation is warm, calm, pragmatic, example-rich, and structured so a newcomer gets to a working result in minutes while an experienced engineer can scan straight to an edge case in seconds.

## Why our previous docs fell short (and what changed)

Our first docs guideline pushed extreme minimalism (150 to 400 word ceilings, one-sentence intros, zero scenarios, zero context). That made pages read like terse database specs rather than guides written for humans trying to ship customer support.

We are keeping the strict factual rigor of the old guide while replacing its clipped austerity with generous, example-first technical teaching:

- **Keep:** verify every single limit, default, endpoint, and UI label in the codebase; never narrate obvious UI buttons field by field; never lecture readers on generic industry concepts like "what is OAuth"; keep every page self-contained for AI coding agents.
- **Change:** drop artificial 150-to-400-word caps; open pages with a warm, grounded introduction that builds the mental model; show realistic, complete examples (real code, real procedures, real workflows) before listing limits; guide readers smoothly from the 80% happy path into customization and edge cases.

## Documentation philosophy

> **Write like a calm, experienced senior teammate sitting beside the reader. Help them understand the mental model, get something real working fast, and avoid the traps you already know about.**

Five principles define great dimmi documentation:

### 1. Documentation is part of the product

Developers and support leads judge dimmi by how quickly our docs make things click. Clear, generous docs turn first-time evaluators into power users and let teams self-serve without filing support tickets.

### 2. Mental models before mechanics

Never drop a reader straight into a table of limits or settings without explaining _why_ the feature exists and _how_ to think about it. Ground every introduction in a recognizable problem:

- On **Procedures**: explain why plain-English step-by-step instructions work better than stuffing edge cases into a giant system prompt, and clarify when to use a **procedure** (AI-guided judgment) versus a **workflow** (deterministic routing).
- On **Tickets**: explain the clean split between a **conversation** (the chat with the customer) and a **ticket** (the tracked work behind it that can link to Linear or Jira).
- On **Actions**: explain how dimmi resolves the customer's verified identity on the server so the language model can never spoof another customer's account ID.

### 3. Optimize for the happy path first

Structure every page using **progressive disclosure** from top to bottom:

1. **80% Happy Path:** show the cleanest, most common working setup immediately. A reader who wants the default behavior should succeed without scrolling past advanced knobs.
2. **Customization & Real-World Patterns:** walk through the common variations (`##` and `###` sections) one capability at a time.
3. **Rules, Limits & Edge Cases:** put exact timeouts, caps, retry schedules, and boundary conditions in clean tables or scannable lists near the bottom (or under their own descriptive heading) so anyone looking up a hard limit finds it instantly via the table of contents without overwhelming beginners at line 5.

### 4. Show real examples, not abstract placeholders

Abstract descriptions make readers guess; concrete examples make features click in five seconds.

- For **developer pages**, show complete, runnable snippets with realistic payloads (`cus_1042`, `ada@example.com`, `Acme Inc`), imports included, and realistic JSON responses where helpful.
- For **product pages** (Procedures, Workflows, SLAs, Broadcasts, Assignment), show a realistic concrete example: a real 5-step duplicate-charge refund procedure, a real Enterprise SLA policy, or a concrete walkthrough of what happens when a teammate goes away with 4 open conversations.

### 5. Warmth without fluff or condescension

Great docs feel welcoming because they anticipate the reader's questions and reassure them at the exact moment they might hesitate (_"You don't need to configure an outbound mailer before your first sign-in. Until you connect one, dimmi writes outbound links to the server logs."_).

Deliver that exact warmth **without** borrowing filler adverbs (`simply`, `just`, `easily`, `thankfully`, `blazingly`, `seamlessly`) or hype adjectives (`robust`, `elegant`, `powerful`). Calm precision and genuine empathy for the reader's job create warmth on their own.

## What belongs in the docs (and what stays out)

Write what makes a reader confident and accurate. Skip what wastes their time.

**Always include:**

- **Mental models and tradeoffs:** why a feature works the way it does, how sibling features relate, and which mode to choose.
- **Concrete, runnable examples:** SDK calls, webhook signature checks, API payloads, sample procedures, workflow structures, and server commands.
- **Behavior the screen cannot show:** exact timings, retry schedules, capacity rules, SLA clock pauses, security guarantees, and background jobs.
- **Practical gotchas and safe testing:** how to dry-run an action, test the agent without emailing customers, or inspect logs before going live.
- **Complete reference data:** events, configuration keys, environment variables, abilities, and CLI flags.

**Still leave out:**

- **Internal plumbing readers do not care about:** leave out behind-the-scenes details that do not change how someone sets up or uses the feature (such as message debounce milliseconds, internal reply character caps, token-trimming mechanics, or internal pipeline flags). Ask: _"Does knowing this change what the reader configures, expects, or debugs?"_ If not, cut it.
- **Click-by-click form narration:** never write five steps to click "New", type a name, and click "Save". Name the screen (**Settings → Webhooks**) and explain only the fields or choices whose consequences matter.
- **Obvious UI controls:** a toggle labeled **Show launcher** needs no sentence explaining what it shows.
- **Self-explanatory error messages:** document an error only when the fix is non-obvious.
- **Generic industry primers:** do not explain what HTTP webhooks, OAuth 2.0, or DNS CNAME records are; link to an authoritative source if needed.
- **Self-hosting configuration on cloud/product pages:** keep `.env` keys and Docker details strictly inside `/self-hosting/`.
- **Internal or unshipped features:** never document `x-internal: true` routes, internal flags, or planned features that are not live in the codebase.

## Anatomy of a dimmi doc page

Every feature guide follows a predictable, scannable top-to-bottom rhythm. Readers learn where to look on any page in seconds.

### 1. Frontmatter

- **Sidebar label** (`label` in `docs/src/nav.ts`): 1 to 3 words. The sidebar group provides context (under `Webhooks`: `Receive`, `Signatures`, `Retries`, `Events`).
- **Title:** clear and standalone for search and AI agents (`Procedures`, `Verify webhook signatures`, `Automatic assignment`).
- **Description:** one concrete sentence under 160 characters stating what the page helps the reader understand or accomplish. Never open with `"Learn about..."`.

### 2. Introduction (no heading required above it)

Start directly under the page title with **2 to 3 short paragraphs** (1 to 3 sentences each):

- Ground the feature in the real problem a support team or developer faces.
- State clearly what dimmi provides and how it fits with the rest of the workspace.
- If a reader commonly confuses two features (e.g., **procedures** vs. **workflows**, **dimmi agent** vs. **Operator**, **tickets** vs. **conversations**), draw that distinction cleanly right here or in an immediate `##` subsection.

### 3. Happy path (working example first)

Show the simplest real-world setup or usage right after the introduction. Give the reader something concrete they can copy, adapt, or picture immediately before introducing optional parameters or edge cases.

### 4. Core capabilities (`##` and `###` Layer-Cake sections)

Walk through each major capability in logical order. Use the **prose-example rhythm** inside every section:

1. **Setup (1 to 3 sentences):** state _when_ and _why_ you would use this capability.
2. **Concrete example:** a runnable code block, realistic procedure/rule example, or structured table.
3. **Clarification (1 to 2 sentences):** explain the non-obvious behavior, default, or return value.
4. **Optional `<Callout>`:** surface a genuine gotcha, security warning, or helpful shortcut (`note`, `tip`, `caution`, `danger`). Keep to 1 to 2 callouts per page so they stay high-signal.

### 5. Behavior, rules, and limits

Place exact system rules, caps, retry schedules, and limits under a clear heading (such as `## Rules and limits` or `## How replies are filtered`) using tables or front-loaded bullet lists. Every number must come straight from the codebase.

### 6. Testing and contextual next steps (only when genuinely useful)

- When a feature supports safe testing (dry runs, test mode, `mail_fallback` logs), explain how to verify it before touching live customers.
- Close with 1 to 2 natural, contextual inline links to what the reader typically sets up next (e.g., _"Once your messenger is live, set up [identity verification](/developers/messenger/identity-verification/) so the dimmi agent can securely read signed-in customer data."_). Never add a padded generic "Next steps" list just to fill space.

## Page shapes and templates

Choose the shape that fits the reader's intent. Start from the matching template in `docs/templates/`.

| Shape                        | When to use                                                                                                     | Template                         |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------- | -------------------------------- |
| **Feature guide**            | Teaching how a dimmi feature works and how to configure it end to end (the default guide chapter)               | `docs/templates/explanation.mdx` |
| **Task / integration guide** | Reaching one specific technical goal (installing the messenger, verifying webhook signatures, connecting Slack) | `docs/templates/how-to.mdx`      |
| **Quickstart / tutorial**    | Building one working integration from scratch step by step                                                      | `docs/templates/tutorial.mdx`    |
| **Reference**                | Looking up structured facts: webhook events, configuration keys, CLI flags, abilities                           | `docs/templates/reference.mdx`   |

The REST API reference is generated automatically from `tests/contracts/api_v1.openapi.json`. Never hand-write endpoint schemas or duplicate full parameter tables on guide pages; link directly to the endpoint instead.

## Combining human warmth with `stop-slop` and `make-it-skim`

Great docs read smoothly top to bottom _and_ reward a reader who only skims headings and code blocks. Apply both skills deliberately:

### Applying `stop-slop` in docs

- **Active, concrete prose:** `"dimmi waits 1.5 seconds in the messenger so rapid follow-up messages get a single reply"` beats `"A 1.5 second delay is applied to consolidate messages."`
- **No AI clichés or filler:** cut `"simply"`, `"just"`, `"easily"`, `"obviously"`, `"thankfully"`, `"blazing"`, `"seamless"`, `"robust"`, `"in this section, we'll"`, `"it's worth noting"`, `"keep in mind"`, binary contrasts (`"Not X. Y."`), and em dashes (`—`).
- **Keep functional precision adverbs:** words like `automatically`, `immediately`, `synchronously`, `only`, and `by default` carry real technical meaning and stay.
- **Keep passive voice only where exact:** when the actor does not matter and passive voice is clearer (`"Drafts are never read by the agent"` or `"Secrets are encrypted at rest"`).

### Applying `make-it-skim` and plain English in docs

- **Grade 5 English or lower:** use exact code and product terms (`webhook`, `SAML`, `Postgres`), and keep every surrounding word plain and everyday. Write _"check"_ instead of _"verify"_, _"set up"_ instead of _"configure"_, _"same every time"_ instead of _"deterministic"_, and _"what happens"_ instead of _"lifecycle"_. See the plain-word table in [WRITING.md](../WRITING.md).
- **Short, focused paragraphs:** 1 to 2 short sentences (**max 30 to 40 words**, 2 to 3 lines on screen), strictly one idea per paragraph. Break anything longer with a blank line, bullet list, table, or code block.
- **Layer-Cake headings:** write descriptive sentence-case headings that mirror what a reader scans the right-hand table of contents for (`## Check the signature`, `## Retry schedule`, `## When nobody is free`). Never put a redundant `## Introduction` heading over a 2-sentence opening unless the page has a long multi-section outline where a clear top anchor helps navigation.
- **Front-load every bullet:** put the key subject or rule in the first 3 words, and vary the opening word across consecutive bullets so scanners do not glaze over.
- **Bold anchors with discipline:** bold interface labels (**Settings → Actions**), list lead-ins, coined terms on first definition, and genuinely load-bearing defaults or limits (at most one prose anchor per paragraph). Never bold full sentences.
- **Use digits for concrete values:** write `1.5 seconds`, `7 days`, `30 steps`, `6,000 characters`, `5% bounce rate` as digits so scanning eyes catch them immediately.
- **Tables for structured comparisons:** whenever items share 2 or more attributes (kinds, states, limits, HTTP codes), use a table instead of nested prose.

## Real dimmi before/after rewrites

These examples take real pages from `docs/src/content/docs/` and show how to elevate them from clipped specs into warm, example-first dimmi docs.

### 1. Procedures (`docs/src/content/docs/ai/procedures.mdx`)

**Before (clipped spec, zero mental model, no real example):**
> A procedure tells the dimmi agent how your team handles one kind of request. Admins write them under **Procedures** in the sidebar.
> ## How the agent uses a procedure
> The agent sees the name and **When dimmi uses this** of every published procedure. When a customer's request matches one, it reads the steps and follows them in order.
> ## Limits
> - 1 to 30 steps, each up to 1,000 characters.

**After (grounded intro, real mental model, concrete example, limits cleanly placed below):**
> When customers ask how a feature works, your help center articles are enough. When they ask to refund a duplicate charge, cancel a subscription, or verify an account change, the dimmi agent needs your team's exact process.
>
> **Procedures** let you write that process in plain English under **Procedures** in the sidebar. Instead of cramming every edge case into one giant prompt, you give the agent focused, step-by-step playbooks for specific requests.
>
> ### Procedures vs. workflows
>
> Use a **procedure** when the dimmi agent needs judgment, conversation, and natural-language instructions to resolve a request. Use a [workflow](/workflows/how-workflows-work/) when you want deterministic routing, tagging, or SLA rules that run identically every time without AI.
>
> ### Writing your first procedure
>
> Every procedure has three parts: when it applies, the ordered steps to follow, and when to hand the conversation to a human. Here is how a billing team handles duplicate charges:
>
> - **When dimmi uses this:** `The customer reports being charged twice for the same invoice or billing period.`
> - **Step 1 (Instruction):** Ask for the invoice number or billing email if it is not already on the customer's profile.
> - **Step 2 (Action):** Call `Look up recent charges` to check whether two identical payments settled within 48 hours.
> - **Step 3 (Condition):** If two matching charges exist and the amount is under $200, continue to Step 4. Otherwise, hand over to the Billing team with the charge IDs.
> - **Step 4 (Action):** Call `Issue refund` (configured to require teammate approval) and let the customer know a specialist is confirming the refund.
> - **Hand over to the team when:** The customer disputes a charge older than 30 days, or threatens a chargeback.
>
> _(Followed by `## How matching works`, `## Step kinds`, `## Drafts, publishing, and pausing`, and `## Limits`.)_

### 2. How the dimmi agent works (`docs/src/content/docs/ai/how-the-agent-works.mdx`)

**Before (cold rule dump full of internal plumbing):**
> The dimmi agent replies to customer messages from your knowledge, and hands the conversation to a teammate when it cannot help.
> ## What happens to a message
> 1. A customer sends a message. dimmi waits 1.5 seconds in the messenger and 2.5 seconds in Slack and API conversations, so several quick messages get one reply. Email has no wait.

**After (clear mental model, focused only on what matters to the reader):**
> The **dimmi agent** works in the same shared inbox as your team. When you turn it on for a channel, it reads the chat, searches your help center and [knowledge sources](/ai/knowledge-sources/), follows matching [procedures](/ai/procedures/), and hands off when a human should take over.
>
> In a new workspace, the agent starts **off**. You can add your docs, write playbooks, and test it on past chats before any customer sees it.
>
> ## How a reply works
>
> 1. **Checks if it should reply:** confirms the agent is on for that channel, inside its [office hours](/ai/set-up-the-agent/), and not already being handled by a teammate.
> 2. **Checks playbooks and docs:** reads the thread and customer info, checks for a matching [procedure](/ai/procedures/), searches your help center and [knowledge sources](/ai/knowledge-sources/), and runs any [actions](/ai/actions/) needed.
> 3. **Replies with sources:** answers in the customer's language, attaches links to the exact articles it used, and titles and tags the chat.
> 4. **Hands over when needed:** if it cannot find an answer or hits a rule, it [hands over](/ai/handover/) with a private summary for your team and stops replying.

## Get every fact from the code

Docs describe what the code ships today. Read the source; never write from memory, old design docs, or the marketing site. Every route, default, limit, config key, and UI label must be verified.

| Fact                                              | Source of truth                                                                                                                            |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| REST endpoints, fields, types, required abilities | `tests/contracts/api_v1.openapi.json` (generated from `routes/*.go`)                                                                       |
| Webhook events and payloads                       | `tests/contracts/webhooks_v1.json`                                                                                                         |
| Messenger HTTP API                                | `tests/contracts/widget_v1.openapi.json`; messenger code in `resources/widget/`                                                            |
| Messenger JavaScript API                          | `resources/widget/core.ts` (commands), `sdk/browser/src/`                                                                                  |
| SDK usage                                         | `sdk/<language>/` and each SDK's README (run `make sdk-generate` to inspect generated types)                                               |
| Config keys and env vars                          | `config/*.go`, `framework/foundation/config.go` (`koanf` tags and defaults), `deploy/selfhost/.env.example`, `deploy/selfhost/references/` |
| Request behavior                                  | `routes/*.go` → `app/http/controllers/` → `app/actions/`                                                                                   |
| Background jobs and timings                       | `app/jobs/`, `app/listeners/`                                                                                                              |
| Dashboard UI labels                               | `resources/js/features/shell/navigation.ts`, `resources/js/features/`, `resources/js/lang/en/*.json`                                       |
| Roles, permissions, abilities                     | `app/auth/permissions.go`, `app/auth/abilities.go`                                                                                         |
| Self-hosting installer & stack                    | `deploy/selfhost/` (`install.sh` / `dimmictl`, `compose.yaml`, `Caddyfile`, `references/`)                                                 |
| Cloud vs. self-hosted differences                 | `config/edition_*.go` and `settle()` in `config/*.go`                                                                                      |
| API versioning and compatibility                  | `contracts/VERSIONING.md`                                                                                                                  |

- Operations marked `x-internal: true` exist only for dimmi's own app. Never document them.
- Copy interface labels verbatim from the dashboard.
- When code and a comment or README disagree, the code wins. Fix or report the stale text.
- On self-hosting pages, a config key like `auth.sessions.idle_lifetime` maps to the environment variable `DIMMI_AUTH__SESSIONS__IDLE_LIFETIME`.

## Code samples

- Set the language on every code fence, and add a file name when relevant: ` ```ts title="app/layout.tsx" `.
- Make every snippet complete enough to paste and run: include imports, client setup, and error handling where it matters.
- Use `<Tabs>` when the same task differs across languages or frameworks. Put shared explanation _before_ the tabs.
- Put placeholders in `<angle_brackets>` and explain each one briefly below the block. Use realistic fake data (`Acme Inc`, `ada@example.com`, `cus_1042`).
- Omit shell prompts (`$`) so readers can copy commands cleanly. Show output in its own block only when it helps the reader recognize success.
- Never include a real secret, token, or customer record.

## Making pages work for AI coding agents

AI agents fetch individual pages as Markdown (`/docs/<page>.md`, `/docs/llms.txt`, `/docs/llms-full.txt`) and act on them directly. Clear, progressive structure serves humans and agents equally well when you keep these habits:

- **Every page stands alone:** link prerequisites inline rather than assuming the reader read the previous sidebar item.
- **Every section stands alone:** name the feature under each heading instead of opening with an ambiguous `"It"`.
- **Exact names and units:** spell endpoints, config keys, headers, and limits exactly as they appear in code.
- **Text carries every critical fact:** never hide a step or value exclusively inside a screenshot; give every `<Screenshot>` descriptive `alt` text.
- **Shared context before `<Tabs>`:** agents read every tab, so avoid duplicating shared explanations across tabs.
- **Redirects on moves:** whenever a page URL changes, add a redirect in `docs/astro.config.mjs`.
- **Public only:** everything in `docs/src/content/` is public. Keep internal notes out.

## Screenshots and MDX components

Use screenshots sparingly: only where seeing a screen saves a reader from hunting, or shows an outcome they need to recognize. Captures are real screenshots generated by `docs/shots/capture.ts` (see `docs/shots/README.md`) and embedded with `<Screenshot name="..." alt="..." />`. Mark a screenshot still to capture with `{/* SHOT: name | path | what to show | alt */}`.

Import components from `~docs/components/mdx` only when they genuinely clarify the structure:

| Component                            | Use for                                                                                                 | Avoid                                                      |
| ------------------------------------ | ------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| `<Steps>`                            | Sequential numbered procedures in a quickstart or tutorial                                              | Non-sequential lists or form narration                     |
| `<Tabs>` / `<TabItem label>`         | The same task across languages or frameworks                                                            | Splitting unrelated information across tabs                |
| `<Callout type>`                     | `note` (helpful context), `tip` (better practice), `caution` (gotcha), `danger` (security or data loss) | More than 2 per page; hiding core instructions in callouts |
| `<Card>`, `<CardGrid>`, `<LinkCard>` | Landing and overview pages that route readers onward                                                    | Inside detailed guides or reference tables                 |
| `<Endpoint method path>`, `<Param>`  | Highlighting a specific endpoint inside a guide                                                         | Rebuilding the auto-generated API reference                |
| `<FileTree>`                         | Directory layouts the reader needs to create or locate                                                  | Ordinary non-file lists                                    |
| `<Accordion>`                        | Optional FAQs                                                                                           | Required instructions or core rules                        |
| `<Badge>`                            | Status markers such as "Beta" inline                                                                    | Visual emphasis                                            |

## Pre-publish checklist

- [ ] **Code-verified truth:** Every endpoint, config key, limit, default, command, and UI label was checked in the codebase.
- [ ] **Progressive flow:** The page opens with a clear mental model, delivers the happy path with a concrete example early, and places edge cases and limits cleanly below.
- [ ] **No filler narration:** No click-by-click form walkthroughs, no generic concept lectures, no self-hosting config on product pages.
- [ ] **Terminology & labels:** Terms match [WRITING.md](../WRITING.md) (`dimmi` lowercase, `Operator` capitalized) and UI labels match the dashboard.
- [ ] **Runnable code:** Every code block includes imports, realistic fake values, and zero real secrets.
- [ ] **`stop-slop` pass:** Warm and human with zero filler adverbs (`simply`, `easily`, `just`, `thankfully`), zero AI clichés, zero binary contrasts, and zero em dashes. Rubric score 40/50 or higher.
- [ ] **`make-it-skim` & plain-English pass:** English stays below Grade 6 (plain words around exact technical terms), paragraphs are 1 to 2 short sentences (max ~35 words), headings pass the Layer-Cake scan, bullets are front-loaded with varied openings, load-bearing numbers use digits, and the Skim and Difference tests pass.
- [ ] **Navigation and build:** The page is registered in `docs/src/nav.ts` (with a 1-to-3-word sidebar label) and `pnpm docs:build` passes.

## Keeping the docs true

Any code change that alters what a reader needs to know (an endpoint, field, event, limit, default, UI label, or install step) updates the docs in the same commit. When a feature is removed, delete its page and add a redirect in `docs/astro.config.mjs`.

To test docs against real customer questions with AI agents, maintain a saved evaluation set of real questions and expected facts and run it only when the owner explicitly approves live LLM runs.
