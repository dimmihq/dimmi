# Automation revamp: workflows and series

**Status:** approved 2026-11-24, in progress on `feat/automation-revamp`.
**Area:** `app/workflows`, `app/support/workflowspec`, `app/actions/workflows`, `app/actions/series`, `resources/js/features/automation`, `resources/js/features/series`, docs and site pages for both.

## Why

Agents and procedures were rebuilt on 2026-11-22. A workspace now runs many agents, each with its own places, hours, handoff team and transfers. Agents and procedures have a studio: drafts, versions, simulations, draft-vs-live compare, analytics and an Operator panel.

Workflows and series were built when there was one "dimmi". They still assume one agent, and they have none of the studio tools. Three engines (workflows, series, routines) now automate work side by side and share none of their building blocks.

## Who owns what

One line per concept, used in the product, docs and site:

- **Workflows** decide *when* something happens to a conversation and *who* owns it: routing, triage, SLAs, follow-ups, tickets.
- **Series** decide *when* to reach out to people and accounts over days or weeks.
- **Agents** decide *what to say*.
- **Procedures** decide *how* an agent does a job.
- **Routines** put Operator on a clock to review and improve all of the above.

This replaces today's line, "workflows are fixed rules without AI, procedures are AI" (`docs/src/content/docs/ai/procedures.mdx:11-16`, `site/src/pages/procedures.tsx:133-135`). The `dimmi` and `classify` steps already contradict it.

## What is wrong today

### Bugs

1. **A greeting workflow mutes the agent for the whole conversation.**
   - Any customer-facing workflow sets `conversations.ai_mode = 'workflow'` (`database/queries/workflow_runs.sql:129`).
   - Only closing the conversation sets it back (`app/workflows/listen.go:171`).
   - Outside an active agent step the gate skips every turn (`app/ai/pipeline/gate.go:36`).
   - So an opener with buttons that ends without an agent step leaves the placed agent silent until close.
   - Intercom's equivalent only stops the agent when a teammate replies or the agent escalates.
2. **Deleting an agent ignores the workflows that name it.**
   - `app/actions/ai/agents.go:322` deletes it while a live workflow's step still holds its `agent_id`.
   - The step then resolves to whichever agent comes next in the order.
3. **The docs describe a setting the UI lacks.**
   - `docs/src/content/docs/ai/multiple-agents.mdx` tells users to set `agent_id` on the step.
   - The builder has no picker, and still says "dimmi is off… Agent settings" (`step-fields.tsx:482`).

### Conflicts with agents and procedures

- **Handoff is overridden without saying so.**
  - Inside a workflow step, the agent's handoff team and priority are cleared (`app/ai/pipeline/workflow.go:72`).
  - Neither the agent studio nor the builder says so.
- **A step-picked agent can never transfer.**
  - Code: `app/ai/pipeline/transfer.go:73`.
  - The multi-agent spec said a step could allow it.
- **There is a third layer of guidance.** The step's "Extra instructions" and reply cap sit beside the agent's guidance and its procedures.
- **Two systems route by topic.**
  - Transfers use each agent's "What it handles".
  - Workflow `classify` and series `ai_path` each use their own categories and prompts.
- **Agents have no view of their workflows.** "Where it answers" says "unless a workflow picks another", but never lists those workflows.

### Duplicated building blocks

| Job | Workflows | Series | Agents and procedures |
| --- | --- | --- | --- |
| Call an outside system | `http_request` (own URL and connection) | `webhook` (raw URL, no connection) | Workspace actions, with connections, definitions and tests |
| Sort with AI | `classify` (conversation, one category) | `ai_path` (profile only) | Decision model judgments |
| Test | `test_on_test_conversations` flag | "Try it on me" with skippable waits | Simulations, test cases, compare |
| Results | Run list only | Per-step counts, split winner, goals | Analytics tabs, reports compare |
| Build with AI | None (MCP can draft) | None | Operator panel and tools |

The agent behind a series reply is unclear too. Replies go to the team or teammate the broadcast picked (`app/actions/broadcasts/replies.go:205`), and the series has no say in which agent answers.

## What competitors do

These findings come from public docs (marked "doc") and marketing posts (marked "mkt"), researched on 2026-11-24.

- **Intercom (doc)**
  - "Let Fin handle" with explicit handover branches.
  - A "Fin closes conversation" trigger filtered by confirmed resolution or inactivity.
  - Fin Attributes: AI-detected fields, re-checked during the conversation, that drive branches, reports and inbox views.
  - Workflow path analysis with drop-off per step and version-vs-version compare.
  - Version history with rollback to a draft, and control groups on Series.
  - Simulations only for procedures, not workflows.
- **Zendesk (doc)**
  - Intelligent triage writes intent, sentiment and language as fields, updated during the conversation.
  - Action flows are shared integrations that AI agents call.
  - Admin Copilot flags duplicate and stale triggers.
- **Front (doc):** "Branch with AI" with a "no answer matched" branch, and "Extract with AI" into typed variables that never invents a value.
- **Braze (doc):** the Canvas Agent step returns typed output with fallbacks, a daily cap and invocation logs. Preview runs on a chosen user.
- **Gorgias (doc):** rules run before the AI. An `ai_ignore` tag stops the AI, which avoids two replies.
- **Decagon, Sierra (mkt)**
  - Versions that show which version handled a given conversation.
  - Tests generated from real conversations, and release snapshots with rollback.
  - Drafting from a description of the outcome.
- **Freshworks, Customer.io (mkt):** describe a workflow or journey and the AI drafts it for approval.

Nobody documents simulation for visual workflows, or AI building of visual workflows. Both are open ground, and we already own the engines for them.

## Proposal

Changes are additive (step kinds and ids stay) except the agent-step settings the owner chose to remove; see Owner decisions. Run `make api-snapshot`, `make api-compat`, `make sdk-check` and `make sdk-compat` after each contract change.

### Phase 0: fix the bugs

- **Release the conversation** when a customer-facing run ends without an active agent step: set `ai_mode` back to `auto`. Keep holding it while a run waits on a customer-facing step (buttons, collect).
- **Guard agent archive** as the owner decisions describe.
- **List each agent's workflows** under "Where it answers".
- **Fix the docs** for the agent picker and the hand-back.

### Phase 1: fit agents into workflows and series

- **"Hand to agent" step.** This is the `dimmi` kind, renamed in the UI.
  - Agent picker: a named agent, "the agent placed here", or "let the agents sort it". The last routes by each agent's "What it handles", the same way transfers do.
  - An "Allow transfers" toggle.
  - A visible note when the step's handover path replaces the agent's own handoff team.
  - "Extra instructions", the reply cap, ask-email and handover notice are removed.
- **Outcome branches after the step:** resolved (confirmed or assumed), handed over, went quiet, transferred, failed.
- **Agent outcome triggers:**
  - Keep `dimmi.handed_over` and `dimmi.resolved`, and add an agent filter to both.
  - Add `agent.transferred`.
  - Add the confirmed or assumed resolution filter (Intercom parity).
- **"Assign AI agent" action:** pins a conversation to an agent, from background workflows and series.
- **Series replies:** a "Who answers replies" setting per message step, with the same three choices as the agent picker.
- **One actions layer.**
  - New steps call workspace actions, with connections, definitions and tests, in workflows and series.
  - `http_request` and `webhook` keep running and are hidden from the add-step menu.
- **Turn off AI / Turn on AI** actions (Gorgias's `ai_ignore`): keep every agent off a conversation until a teammate or a workflow turns it back on.

### Phase 2: studio parity

Workflows and series get the agent studio's shell: Build, Analytics and History tabs, plus a side panel with Preview, Simulations and Operator.

- **Simulations**
  - Teach the AI-played customer to tap buttons, answer `collect` steps and let timeouts pass. Each run shows the trigger verdict, every step and path, and the agent's turns inside agent steps.
  - Series reuse "Try it on me" for real sends, and add a dry simulation on a chosen or made-up person with skippable waits.
  - A saved simulation library with "Run all", and draft-vs-live compare as agents have it.
  - New simulations can come from a past conversation, as for agents.
- **Analytics**
  - Workflows get per-step counts on the canvas, copied from series `step-results.tsx`: reached, left by each path, waiting now.
  - Drop-off and a version-vs-version compare.
  - Workflows join the reports compare beside agents.
- **Failure inbox:** one list of the conversations where an action, classify step or agent step failed, linked from Analytics.
- **History**
  - Versions with author, a diff and restore as a draft, reusing `agent-diff.ts`.
  - Each run records which version handled it.
- **Operator**
  - Workflow and series tools in `app/ai/operator`, proposing changesets that show live in the open builder.
  - "Describe it and Operator drafts it" for new workflows and series.

### Phase 3: shared AI steps, goals and templates

- **One AI decision step for workflows and series.**
  - It reads the whole conversation (or the person's profile and events for series).
  - It answers a question with paths and a "no match" path, or extracts typed values (text, number, date, boolean, choice) into attributes, with fallbacks.
  - `classify` and `ai_path` stay as kinds and run on it.
- **AI conversation attributes**
  - Defined once, detected and re-checked during the conversation.
  - Usable in branches, triggers, reports, inbox views and SLA rules.
  - This also covers the "auto conversation fields" in the parity spec's third wave.
- **Goals and control groups**
  - Series get a control group that receives nothing, so lift can be measured.
  - Workflows get an optional goal (resolved without a handover, CSAT) and A/B paths.
- **Templates:** triage before the agent, out-of-hours, agent per channel, chat grading, QA alerts, onboarding series, and win-back series.
- **Routine templates** for automation hygiene:
  - Find overlapping customer-facing workflows. The first one wins, so overlaps hide each other.
  - Find steps that fail often.
  - Find unused workflows.
- **Docs and site:** rewrite the workflows, series and procedures pages around "who owns what". Update `site/src/pages/workflows.tsx` for multiple agents.

## Owner decisions (2026-11-24)

- **One PR.** Every section above ships together; the phases are only the order of work.
- **Agents are never hard-deleted.** "Delete" becomes archive: the agent is turned off and hidden, and its threads, runs and history stay. Archiving is refused while anything ties the agent, and the refusal lists each tie:
  - workflows (live or draft) whose agent step or "Assign AI agent" action names it;
  - series whose replies go to it;
  - channels or inboxes it is placed on;
  - other agents that list it under "May transfer to".
- **Hand back after a workflow.** When a customer-facing run ends, the conversation goes back to the placed agent. A new **Turn off AI** action keeps every agent off the conversation until a teammate (or a **Turn on AI** action) turns it back on. A handover to a person still stops the agent, as today.
- **Extra instructions, reply cap, ask-email and handover-notice settings are removed** from the agent step outright, not hidden. There is no production data, so this break is accepted.
- **Studio layout.** Workflows and series move to the agent studio layout: Build, Analytics and History tabs, with a Preview, Simulations and Operator side panel.
- **Routines stay Operator-only.** Workflows start only from events, never on a schedule.

## What shipped on this branch (2026-11-24)

- **Agents:** archive replaces delete and is refused while workflows, series, places or other agents rely on the agent; the dialog lists each tie. The agent's page lists the workflows that hand conversations to it.
- **Hand back:** a finished customer-facing run hands the conversation back to the placed agent, which answers from the next customer message. A message the workflow took never gets a second answer.
- **Steps:** "Hand to an AI agent" (agent picker, May transfer, no extra instructions or reply cap), Assign an AI agent, Turn off AI, Turn on AI, Extract with AI, Run an action. The HTTP request and series webhook steps still run but leave the menus.
- **Triggers and conditions:** AI agent transfers to another agent; AI attribute changed; the AI agent, transferred from and transferred to fields.
- **Teammates:** Turn off AI and Turn on AI in the conversation's menu (`ai_enabled` on the conversation update).
- **Replies:** broadcasts and series pick the agent that answers replies.
- **Studio:** workflows and series use the agent studio layout, with Analytics (per-step counts, drop-off, versions, failed runs, goal), History (compare and restore), Simulations (workflows: saved, run all, draft vs live, from a conversation; series: dry runs) and Operator (reads, edits on the canvas with Accept or Decline, drafts from a description, explains a workflow run and a person's series journeys, sees the selected series step).
- **AI:** one decision primitive behind Sort by topic, AI path and Extract; AI conversation attributes with a settings page.
- **Series:** control groups with lift in the report.
- **Templates:** six workflow templates in the new-workflow dialog; a Workflow check-up Operator routine.
- **Reports:** a Workflows table in the AI report; an AI attribute filter on every report and a Conversations by AI attribute table in the AI report.
- **Inbox:** an AI attributes section in the conversation's details panel, refreshed when a value changes.

### Left for later

- A/B paths inside workflows.
- Simulating the Extract and Run an action steps beyond recording them, a customer interrupting a wait, and a called workflow's own steps.
