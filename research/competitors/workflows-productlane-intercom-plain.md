# Workflows and automation: Intercom, Plain, Productlane (plus Zendesk, Front, Help Scout, Chatwoot, Crisp)

Research date: 2026-11-04. Input for `specs/17-workflows`. Lens: what a support automation engine must do, where the three closest competitors fall short, and what that means for Dimmi.

Tags: [unverified] = single source or sources disagree. [3P] = third-party claim. Everything summarised in our own words; follow the URL for the source text.

## TL;DR

- **Intercom** has the richest builder: about 20 triggers, a canvas with reply buttons, branches, data collection, Fin, waits, SLAs, reusable workflows. Weak where engineers care: no public Workflows API, no run logs ("troubleshooting is guesswork"), drafts only on three triggers, background workflows conflict without a defined winner, a matching workflow whose branches all miss silently blocks lower workflows, many steps do not work on email or WhatsApp, no copy and paste of paths. Builder from the $85 Advanced seat; SLAs Expert only ($132).
- **Plain** rebuilt automation in 2026 as one visual graph builder: nine event triggers plus manual, condition / action / wait steps, AI conditions ("match if..."), AI-picked labels and priority, HTTP request step, MCP tool steps, Sidekick agent step. Strong primitives (business hours, SLA policies, escalation paths, least-loaded assignment). Missing: drafts and version history, test runs (only Ari "Test mode"), run logs, analytics per workflow, scheduled or SLA-breach triggers, loops.
- **Productlane** has no rule builder at all. Automation = AI "Routines" (plain-language instructions, 7 fixed actions, two triggers), a few toggles (auto-assign to Linear owner, auto-create issues), first-match SLA policies, and the strong Linear "close the loop" flow with pre-send checks. Deterministic time rules (auto-close after N days, unassigned for N hours) do not exist.
- **Gaps nobody fills**: run history you can read per conversation and per step, deterministic conflict rules, an API and CLI to manage and run workflows, drafts plus version history plus rollback on every trigger, a simulator that explains why a workflow matched or not, one engine that drives the AI agent and the humans with the same rules, and an honest definition of "resolved".

## 1. Intercom Workflows

Sources: help center articles linked inline; changelog `intercom.com/changes`; community forum.

### Model

- One workflow = one trigger + audience + schedule + a canvas of steps. Triggers are a fixed list; arbitrary backend events cannot start a workflow (Series does that). https://www.intercom.com/help/en/articles/7836459-workflows-explained
- Trigger type cannot change after creation; duplicate into a compatible trigger instead. https://www.intercom.com/help/en/articles/6611595-using-the-workflows-builder
- Two kinds: **customer-facing** (one at a time per conversation, first match in the ordered list wins) and **background** (every match runs, in parallel). https://www.intercom.com/help/en/articles/11783255-how-can-i-see-if-a-workflow-triggered-in-a-conversation, https://intercom.com/help/en/articles/7857645-managing-the-order-of-your-workflows

### Trigger catalogue

Source unless noted: https://www.intercom.com/help/en/articles/7434613-how-to-trigger-a-workflow

| Trigger | Notes |
|---|---|
| Customer visits a page | Messenger only; time on page or URL rules; only trigger with goals |
| Customer clicks a website element | Messenger only |
| Customer opens a new conversation | Before typing; first step must be a message (no branch first) |
| Customer sends first message | Once per conversation; Messenger plus email, WhatsApp, others |
| Customer sends any message | Every message; meant for background tagging; must not contain Fin or Fin keeps answering after a human joins (https://intercom.com/help/en/articles/10032299-use-fin-ai-agent-in-workflows) |
| Customer unresponsive | 30 s to 14 days; only after a teammate (not bot) message; never for API-created or outbound conversations (https://www.intercom.com/help/en/articles/7155449-customer-or-teammate-has-been-unresponsive) |
| Teammate unresponsive | 30 s to 14 days from first unanswered customer message; once per customer message; limited actions |
| Teammate sends any message, adds a note, changes state, changes assignment | Background only; bulk assignment from inbox does not fire |
| Ticket created, ticket state changed | Background only |
| CX score rating received | Arrives hours after close |
| Customer calls | Phone; reusable phone workflows return to the caller (https://www.intercom.com/help/en/articles/11688331-how-to-use-phone-reusable-workflows) |
| Reusable | No audience; started by a "pass to reusable workflow" step or by hand from the inbox (Cmd+K) |
| Fin closes a conversation | Filter by close reason |
| Before SLA breach / SLA breached | Expert plan, beta, on request; checked about once a minute (https://www.intercom.com/help/en/articles/15362781-trigger-a-workflow-before-or-when-an-sla-breaches) |

Channel parity: only first message, any message, state change, and the two unresponsive triggers work on WhatsApp, email, SMS, Slack. https://intercom.com/help/en/articles/7851710-configure-workflow-trigger-settings

### Steps

https://www.intercom.com/help/en/articles/6611595-using-the-workflows-builder

- Customer-facing: bot message, collect data (to person attribute or conversation data), let Fin answer, show expected reply time, collect customer reply (blocks until typed), send ticket form, send app, pass to reusable workflow.
- Routing: reply buttons (route only, no links), buttons from custom objects, branches.
- Background: apply rules, tag / untag conversation or person, assign, snooze, wait, priority (5 levels), apply SLA, custom action (HTTP data connector), disable customer reply, close, set conversation data, integration action, note, AI summary note, wait for webhook (only replies to its own request), convert to ticket.
- No explicit end step; a path just ends.

### Conditions and branches

- Audience: AND/OR predicates on person, company, conversation, message, availability, topics, tags; negation; estimated match count shown. Company rules false when person has no company. https://intercom.com/help/en/articles/7851710-configure-workflow-trigger-settings
- Branches: ordered "if" paths plus else, first match wins; exact-match strings break on whitespace; 320-predicate limit per branch, over the limit the branch silently counts as no match. https://www.intercom.com/help/en/articles/7846212-using-branches-in-workflows
- Schedule: any time, office hours, outside office hours, custom weekly windows, workspace timezone.

### Execution

- Running conversations finish on the version they started with, waits included; pausing only blocks new entries. https://www.intercom.com/help/en/articles/6611595-using-the-workflows-builder
- Wait and snooze are interrupted by teammate and/or customer messages (configurable) or any closure; an interrupt **ends** the workflow; tags and notes do not interrupt. https://www.intercom.com/help/en/articles/7155449-customer-or-teammate-has-been-unresponsive
- After a customer message at an input step Intercom waits about 2 s for more text. https://community.intercom.com/questions/5198
- Global "auto-close abandoned bot conversations" after 3 to 15 minutes at an input step. https://www.intercom.com/help/en/articles/7836459-workflows-explained
- Background conflicts (two assignments) have no defined winner (same source as ordering).
- No re-entry setting; staff suggest tags or "teammate reply is false" to stop repeats. https://community.intercom.com/workflows-7/why-is-my-workflow-triggering-multiple-times-for-the-same-user-how-can-i-fix-this-8289
- Billing lapse auto-pauses workflows; they must be set live again by hand.
- Fin "simple deployment" overrides a first-message workflow with overlapping audience. https://intercom.com/help/en/articles/10032299-use-fin-ai-agent-in-workflows

### Fin inside workflows

https://intercom.com/help/en/articles/10032299-use-fin-ai-agent-in-workflows

- Fin answers only typed messages; a button click bypasses it.
- Settings: expectation text, collect info before handover, inactivity follow-up (chat fixed 4 minutes; email 1 h to 30 days), CSAT, auto-close by reason, email CC behaviour.
- Fin keeps answering until a teammate sends a customer-visible reply; notes, reassignment, status changes do not stop it.
- Office hours always apply on Fin handover. Fin "outcomes" billed $0.99, including assumed resolutions after silence and Procedure handoffs (see `research/competitors/fin.md`).

### Versioning, testing, reporting

- Drafts only for Messenger-open, reusable and Fin-task triggers; every other trigger (email included) edits live. https://www.intercom.com/help/en/articles/6611595-using-the-workflows-builder
- Version history and rollback since March 2026; rollback creates a draft; blocked while a draft exists; old versions may point at deleted tags. https://www.intercom.com/changes/en/134017-workflow-version-history-and-rollback
- Preview in a Messenger modal as a synthetic user; not usable to test audience matching; advice is to target your own email. https://www.intercom.com/help/en/articles/7836467-preview-and-set-workflows-live
- Reports: triggered, engaged, completed, per path drop-off (path analysis), version comparison; no export; live workflows only; preview users may or may not count (docs disagree). https://intercom.com/help/en/articles/7837262-optimizing-workflows-with-path-analysis, https://www.intercom.com/help/en/articles/7836469-workflows-report
- Debugging: "show conversation events" in the inbox. https://www.intercom.com/help/en/articles/11783255-how-can-i-see-if-a-workflow-triggered-in-a-conversation

### API, permissions, limits, price

- No public API to list, create, edit or trigger workflows; request open since 2023. https://community.intercom.com/ideas/trigger-a-reusable-workflow-via-the-api-12742, https://community.intercom.com/api-webhooks-23/ability-to-trigger-reusable-workflows-via-rest-api-7927. The Fin CLI lists a `workflows` subcommand (see `fin.md`) [unverified: scope unclear].
- Builders need "message more than one person" plus automation permissions.
- Published limits: branch complexity 320, unresponsive timers 30 s to 14 days. No documented cap on steps or live workflows (https://community.intercom.com/inbox-9/workflow-4006).
- Builder from Advanced ($85 per seat annual); SLAs Expert ($132). Overview article says Workflows are available to all, pricing page says Advanced [unverified]. https://www.intercom.com/pricing

### Complaints

- No execution log, no API, "guesswork" debugging (Oct 2025). https://community.intercom.com/ideas/workflow-builder-needs-major-improvements-logging-troubleshooting-api-support-13017
- Workflows sometimes do not fire. https://community.intercom.com/helpdesk-9/intercom-automation-sometimes-fails-to-trigger-for-new-conversations-15369
- First match ends at a dead branch and lower workflows never run. https://community.intercom.com/getting-started-with-intercom-43/guidance-on-how-to-set-up-different-workflow-triggers-with-rules-and-conditions-and-understand-their-prioritisation-6905
- Duplicate firing from "any message". (thread 8289 above)
- No copy and paste of blocks or paths, asked since 2024. https://community.intercom.com/ideas/the-ability-to-duplicate-or-copy-paste-workflow-blocks-7421, https://community.intercom.com/ideas/allow-duplication-of-workflow-paths-8689
- No WhatsApp preview, no custom text when replies are disabled. https://community.intercom.com/ideas/be-able-to-preview-workflows-for-whatsapp-in-intercom-8857
- G2: complexity, unpredictable per-outcome cost. https://www.g2.com/products/fin-by-intercom/reviews

## 2. Plain

Sources: `plain.com/docs/*.md` pages (changelog entry pages render client-side, so changelog facts come from the index and search snippets).

### Model

- Visual graph builder (Feb 2026) replaced "workflow rules". Three step types: condition (if / else-if / else, top to bottom, and/or groups), action, wait (completed or canceled exits). Forward only, no loops. https://www.plain.com/docs/product/workflows.md
- Waits 1 s to 1 year with optional cancel condition. https://www.plain.com/docs/product/workflows/workflows-wait-for.md
- Manual "run workflow" from Cmd+K (Dec 2025) and in bulk on a queue (Oct 2026); a customer-card `WorkflowButton` runs a workflow from your own UI. https://www.plain.com/changelog, https://www.plain.com/docs/ui-components/workflow-button.md

### Triggers

Thread created, message added, labels changed, tier updated, thread escalated, priority changed, status changed, thread field created / updated, plus manual. Each selected event starts its own run, so one workflow can run twice on one thread. No scheduled, quiet-customer or SLA triggers. A "Customer Agent unassigned" trigger is mentioned only on the Ari handoff page [unverified]. https://www.plain.com/docs/product/workflows/workflows-triggers.md

### Conditions and actions

- Conditions: thread state, labels, tier, channel; message contains keywords (bots and autoresponders skipped); **AI prompt** condition over up to 600 messages; business hours (outside one or all schedules). https://www.plain.com/docs/product/workflows/workflows-conditions.md
- Actions: assign user / machine user / team (round robin in the action doc, least-loaded in the assignment doc [unverified]), unassign, priority (manual or AI), tier, status, labels (manual or AI), lock, send message, note, HTTPS POST with variables (webhook retry policy), escalation path, SLA policy, Sidekick agent action, start agent discussion, custom MCP tool steps. First failing action stops the run and marks it failed. https://www.plain.com/docs/product/workflows/workflows-actions.md
- Ordering: matching workflows run one after another in page order; a run paused at a wait lets the next one start; reorders apply to the next event. Workflow label changes never fire "labels changed" (anti-loop). https://www.plain.com/docs/product/workflows.md

### Primitives around workflows

- Assignment: team label driven, skips away / offline / at-capacity members, least loaded; re-evaluated on create, label, done, snooze, reopen, back online. https://www.plain.com/docs/product/platform/threads/assignment.md
- SLA policies: first response, next response, first resolution, total resolution; pause outside business hours; paused statuses; warning window ("imminent breach"); applied by workflows; no auto-escalation on breach. https://www.plain.com/docs/product/platform/slas.md
- Business hours: named schedules, overrides up to 92 days; Foundation 1 schedule, higher plans unlimited. https://www.plain.com/docs/product/platform/business-hours.md
- Auto-responses are a separate, simpler system (new threads, first match, up to 15 min delay, canceled when someone replies). https://www.plain.com/docs/product/platform/auto-responses.md
- Statuses: Todo (needs first / next response, investigating, close the loop), Snoozed (waiting for customer, paused), Done (done, ignored). https://www.plain.com/docs/product/platform/threads/statuses.md

### AI

- Ari ("Customer Agent" since Oct 2026) works only on threads assigned to it, usually by a workflow step; per-step Live or Test mode (test drafts privately) is Plain's only automation testing. https://www.plain.com/docs/product/agents/ari/test-mode.md
- Handoff: unassigns itself, writes reason note, sets urgent for incidents, SLA restarts from customer's last message. https://www.plain.com/docs/product/agents/ari/handoffs.md
- Credits: AI label pick 1 to 2 credits, Sidekick 1 to 600 credits per run, resolution 100 credits. https://www.plain.com/docs/product/billing/credits.md

### API, limits, price, complaints

- GraphQL API with machine users and per-key permissions; webhooks (25 targets, 12 retries over about 5 days, versioned schemas, HMAC without timestamp). https://www.plain.com/docs/graphql/authentication.md, https://www.plain.com/docs/webhooks.md
- No documented drafts, versions, rollback, run log, dry run, per-workflow analytics, or step limits.
- Foundation $45 to $55, Horizon $99 to $139 per seat; SLAs and escalation paths Horizon+. https://www.plain.com/docs/product/billing/pricing.md
- Complaints: cost climbs with tiers and credits, API-first setup needs engineers. https://www.featurebase.app/blog/plain-alternatives.md [3P], https://unthread.io/blog/plain-review/ [3P]

## 3. Productlane

Docs moved to `hello.productlane.com/docs`; several inbox pages are placeholders.

- **Routines** (Pro+): plain-language instructions, one per line; two triggers (new thread; customer reply after a week of quiet); seven actions (tag, importance, assign if unclaimed, note, file Linear issue, close/reopen, post to Slack); never replies to customers; skips machine replies; logged as "AI agent" on the timeline; ordering between routines undocumented. https://hello.productlane.com/docs/ai/routines
- **Auto-assignment**: three toggles (Linear owner on Scale, follow latest sender, contact-to-company by domain). No round robin. https://hello.productlane.com/docs/inbox/auto-assignments
- **Linear flows**: two-way issue sync, replies from Linear comments, automatic issue creation, close-the-loop follow-ups with pre-send checks (teammate already told them, customer wrote back, issue canceled, deploy verified) and a logged reason when held back. https://hello.productlane.com/docs/ai/close-the-loop-drafts, https://productlane.com/changelog/2026-10-15-automatic-loop-closing
- **AI agent handoff**: "Talk with a human" button, cannot be disabled; after handoff agent stops; thread comes back unread, high priority. Resolution = confirmed, 24 h silence, or teammate done; refunded on real follow-up within 72 h. https://hello.productlane.com/docs/ai/ai-agent
- **SLAs** (Scale): policies checked top to bottom, first match wins; first / next response and time to close; conditions on thread origin, priority, company fields; clock only while the team owes a reply. https://hello.productlane.com/docs/administration/slas
- **Webhooks**: pattern subscriptions, `message.sent.author_type` (user, ai, auto_reply, close_loop) lets receivers avoid loops, 3 retries, replay for 30 days. https://hello.productlane.com/docs/webhooks
- Gaps (our reading of docs): no deterministic rules, no time-based rules, no round robin, macros cannot act, routine ordering undefined.

## 4. Others: ordering and loop protection patterns

| Product | Ordering | Loop protection | Source |
|---|---|---|---|
| Zendesk triggers | list order in categories; any change restarts the cycle, fired triggers skipped | each trigger once per update; nullifying conditions advised; automations hourly with mandatory nullifying action; 100 automation updates per ticket | https://support.zendesk.com/hc/en-us/articles/4408886797466, https://support.zendesk.com/hc/en-us/articles/4408832701850 |
| Front rules | company rules first, then list order; all match unless "stop processing" | a rule's action may trigger a later rule, never itself | https://help.front.com/t/m22445 |
| Help Scout | list order (secondary source) | automatic workflows run once per conversation | https://docs.helpscout.com/article/22-using-workflows |
| Chatwoot | definition order | undocumented [unverified] | https://www.chatwoot.com/hc/user-guide/articles/1677689800-how-to-use-automation |
| Crisp | undocumented | proactive triggers have once-per-user | https://help.crisp.chat/en/article/understanding-and-mastering-the-crisp-workflows-1boypme.md |

## 5. What this means for Dimmi

1. One ordered list, two kinds (customer-facing, background) is the right mental model; Intercom proved it. Fix its two holes: background runs execute in list order one after another per conversation (deterministic), and a customer-facing workflow that ends without doing anything passes the conversation to the next match.
2. Run history per conversation and per step, plus a "why did it (not) match" explainer, is the most requested missing feature across Intercom and Plain.
3. Drafts, versions, rollback on every trigger. Running conversations keep their version (Intercom does this; keep it).
4. API, CLI and MCP for workflows: Intercom has none, Plain has GraphQL only. Our audience wants workflows reviewed in pull requests.
5. Loop rule: a workflow never re-triggers itself; chains to other workflows bounded (Front pattern plus depth cap).
6. AI where it helps (classify into paths, let dimmi answer), deterministic everywhere else. Productlane's all-AI routines and Plain's credit-metered AI steps both make cost and behaviour unpredictable; BYOK with zero markup removes the cost half.
7. Honest outcomes: "resolved" only when the customer confirms (Intercom and Productlane bill silence as resolution).
8. Channel parity: buttons and data collection must degrade to numbered text on email and chat apps instead of being unavailable.
9. Fold "rules lite" (office hours, round robin, auto-replies, SLA timers, HTTP step) into workflows instead of building a second system (Plain runs auto-responses separately; Intercom splits SLAs into Expert).
