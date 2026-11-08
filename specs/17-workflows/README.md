# 17 Workflows

Style: caveman. Date 2026-11-04. Backend first, plus the builder that already exists as a prototype in `resources/js/features/automation`. Read with `specs/00-conventions/*`, `specs/00-foundation/*`, `specs/03-shared-inbox`, `specs/07-dimmi-agent`, `specs/18-audiences/README.md`. Builds after 18 M4: rule language, evaluator, builder come from 18. Research: `research/competitors/workflows-productlane-intercom-plain.md`.

Files in this folder:

| File | What |
|---|---|
| README.md | this: goal, bar, scope, decisions, data model, code map, events, jobs, config, limits, permissions, other-spec changes, where we beat them, open questions |
| [catalog.md](catalog.md) | every trigger (payload, allowed steps), condition fields, every step kind (settings, behaviour, channel rendering), templates |
| [engine.md](engine.md) | execution: matching, ordering, claims, runs, waits, idempotency, re-entry, loop guard, mid-run changes, failure, versions, office hours, SLAs, assignment |
| [api.md](api.md) | REST, widget additions, MCP, CLI, webhooks, realtime, examples, errors |
| [edge-cases.md](edge-cases.md) | decided behaviour per case |
| [tasks.md](tasks.md) | milestones with acceptance tests |

## Boundary with 03 assignment

Round robin, balanced and capacity also work with no workflow: inbox setting in [03 assignment.md](../03-shared-inbox/assignment.md). This spec's `assign` step calls same `assignment.PickAssignee`, never own logic. `team_assignment_cursors` moved to 03 (track 02 migration; `20261101001710` not built here). `conversation.priority` is level `none|low|medium|high|urgent`, no longer bool (03 views.md).

## Goal

One engine that decides what happens to a conversation without a human: what the customer sees first, which questions get asked, when dimmi (the AI agent, 07) answers and when it steps back, who owns the conversation, what gets tagged, when it closes, when an SLA clock starts, when your own systems hear about it. Built visually, versioned, testable before it goes live, explainable after it ran, and fully driveable by API, CLI and MCP.

Why: roadmap post-MVP item 11 "rules lite" (office hours, auto-assign, auto-tags, auto-replies, HTTP step, SLA timers) and "workflows, SLAs" from "Not at launch" collapse into one feature. Building rules lite first and workflows later means two engines, two UIs, two mental models (Plain still runs auto-responses beside workflows; Intercom splits SLAs into a higher plan). Every competitor we switch customers from has some form of this (Intercom Workflows, Plain workflows, Productlane routines). Importers (16) currently drop Intercom workflows and Zendesk triggers for lack of a target.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| First workflow message after customer's first widget message | p95 under 500 ms server time (claim in ingest tx, advance job picks up under 100 ms) |
| Opener (buttons before the customer types) | rendered from widget config cache, zero extra round trip; no conversation row until the customer acts |
| Background step (assign, tag, note) after triggering event | p95 under 1 s |
| Timer accuracy (waits, quiet triggers, SLA warnings) | fires within 5 s of due time p95; lost timer recovered within 2 min by reconciler |
| Determinism | same event + same data + same live versions = same result, always (background runs serialized per conversation in list order) |
| Explainability | every run has a step-by-step trace; every conversation answers "which workflows looked at me, which ran, why the others did not" |
| Safety | zero duplicate customer messages under retries (dedupe key per run step); zero infinite loops (self-trigger impossible, chain depth 3, per-conversation step budget) |
| Draft to live | edit any workflow (any trigger) as a draft, simulate it against a real past conversation, try it on test conversations, publish, roll back in one click; running conversations keep their version |
| Channel parity | every customer-facing step works on widget, email, Slack, Discord, Telegram, WhatsApp, API (buttons degrade to numbered choices) |
| API parity | everything in the builder doable by REST, CLI (`dimmi workflows`) and MCP; definitions are plain JSON files a repo can review |
| Cost | never metered; workflow bot messages never bill; AI steps spend the workspace's own key under the 07 cap |

## Scope

### In (MVP)

- Workflows CRUD, one immutable trigger per workflow, audience, channels, schedule (office hours), re-entry rule, exclusive flag, ordering.
- Working copy (draft) plus immutable versions for every trigger; publish, pause (optionally cancel running runs), resume, rollback, archive; runs keep their version.
- 16 triggers in 6 groups (catalog.md): conversation start (opener, first message, customer reopens), during conversation (customer message, customer quiet, rated), team (teammate message, note added, assignment changed, status changed, teammate quiet), dimmi (handed over, resolved), service levels (SLA warning, SLA breached), reusable.
- Conditions: 18 rule language for audience and branches. 17 registers conversation, message, trigger, dimmi, office hours and run variable fields (catalog.md section 3).
- Steps (catalog.md): message, reply buttons, collect data, show reply time, let dimmi handle, branch, AI classify, wait, run workflow (reusable, call and return), assign (member, team, round robin, balanced), unassign, tag, untag, priority, note, update contact, snooze, close, apply SLA, HTTP request.
- Execution engine on River (engine.md): claims, ordering, durable runs, timers, idempotent steps, loop guard, failure handling.
- dimmi integration: dimmi joins a conversation a workflow owns only through a "Let dimmi handle" step; outcomes (resolved, handed over, went quiet) are paths; automatic dimmi (07 settings) stays the fallback for conversations no customer-facing workflow claims.
- Office hours: named schedules with holidays, owned here (moves `ai_agents.office_hours` out of 07).
- SLA policies (first response, next response, resolution) applied by workflows, business-hours clocks, warning and breach triggers.
- Team round robin and balanced assignment with availability.
- HTTP request step: signed outbound calls or 07 action connections, response mapped into run variables, success and failure paths.
- Testing: simulation (dry run, no side effects) against synthetic or real conversations, match explainer, drafts live on test conversations only.
- Run history per workflow and per conversation, step trace, per-step and per-path analytics, version comparison.
- Templates (catalog.md): 9 starting points shipped as JSON in the repo.
- REST, widget additions, MCP toolset `workflows`, CLI `dimmi workflows` (list, show, export, apply, publish, runs, simulate), webhooks for runs and publishes.
- Builder (frontend): wire the prototype to the API, plus the changes listed in "Prototype: keep and change".

### Out (later)

| Item | Later |
|---|---|
| Customer visits a page / clicks an element (proactive) | covered by 15 chat-format announcements (targeting, page rules, frequency); the reply starts a conversation that first-message workflows see with `conversation.source.announcement_id`. No second targeting engine |
| Contact-level events from the customer's app ("payment failed" starts a conversation) | later: `workflow-contact-events` (needs proactive conversation creation; standout feature "error-triggered proactive support"). Seam: trigger registry + `POST /workflow-runs` already accepts `contact_id` reserved field (422 `not_supported` at MVP) |
| Paths that rejoin (DAG), loops | later: `workflow-path-joins`. Tree only at MVP; definition stores parent links so joins are additive |
| Native interactive buttons on Slack, Telegram, WhatsApp, Discord | later: `native-channel-buttons`. MVP renders numbered choices as text on every non-widget channel |
| Conversation custom fields as condition and collect target | post-MVP: ticket fields (collect saves to contact attributes or run variables at MVP) |
| Scheduled go-live (`live_from`, `live_until`) | later: `workflow-schedule-window` |
| Member capacity (max open per member) for balanced assignment | later: `assignment-capacity` |
| Escalation paths (ordered levels) | later: `escalation-paths` (assign step + SLA breach trigger cover MVP) |
| Real-time collaborative editing (CRDT) | later; MVP: presence ("Lea is editing") + `lock_version` conflict 409 |
| A/B split step | later: `workflow-split-tests` |
| dimmi procedures inside a workflow step | later with 07 `action-procedures` (step setting `procedure_ids` reserved) |
| Reports beyond per-workflow analytics | post-MVP: reports |
| Importing Intercom workflows / Zendesk triggers (16) | later: `import-workflows` (16 maps a safe subset to drafts, never live) |

## Concepts

- **Workflow** (`workflows`, `wf_`): name, trigger (immutable), working copy (draft definition), status, list position, optional live version.
- **Version** (`workflow_versions`, `wfv_`): immutable snapshot of trigger settings, audience, schedule, re-entry, definition. Created by publish. Runs reference a version forever.
- **Definition**: tree of steps. A step may have **paths** (buttons, branch, classify, dimmi, wait, collect, HTTP request); each path holds its own step list. Top to bottom only.
- **Kind**: computed per version. `customer_facing` when any step can speak to the customer (message, buttons, collect, reply time, dimmi, close with rating) or the trigger is a start trigger; else `background`.
- **Run** (`workflow_runs`, `wfr_`): one execution of one version on one conversation. Cursor, variables, status, trace.
- **Claim**: a conversation is held by at most one customer-facing run at a time (`conversations.workflow_run_id`). While held, other customer-facing workflows do not start and automatic dimmi stays silent.
- **Schedule** (`schedules`, `sch_`): office hours. **SLA policy** (`sla_policies`, `sla_`): response targets.

## Decisions (with why)

1. **One engine, folds rules lite.** Office hours, auto-replies, round robin, auto-tags, SLA timers and the HTTP step are workflow features, not a separate rules system. Why: one mental model, one API, one history view (competitor gap: Plain auto-responses separate, Intercom SLAs gated).
2. **Two kinds, one list.** Customer-facing: first match in list order wins, one per conversation at a time. Background: every match runs, one after another per conversation in list order, so later workflows see earlier changes and the last one wins deterministically. Why: Intercom's model is right; its "no guaranteed winner" for background conflicts is not.
3. **A workflow that did nothing passes the conversation on.** A customer-facing run that ends having executed no customer-facing step and no action (only branches with empty paths) is recorded `passed` and the next matching customer-facing workflow gets the conversation. Why: Intercom's most cited ordering pitfall (dead branch blocks lower workflows).
4. **Exclusive flag.** A workflow may say "when I run, later workflows for this trigger do not". Front's "stop processing", applied to background too.
5. **Tree only, plus reusable workflows that return.** Paths never rejoin at MVP. A reusable workflow is called like a function: the caller waits, then continues. Why: trees read top to bottom, per-step funnels stay exact, loops are impossible by construction; reuse covers duplication. Builder adds copy and paste of steps and whole paths (Intercom's top request).
6. **Trigger immutable.** Kept from prototype and Intercom: steps, allowed kinds and data depend on it. "Duplicate as..." copies into another trigger when compatible.
7. **Drafts and versions for every trigger.** Working copy autosaves; publish makes an immutable version; runs keep their version; rollback = copy old version into working copy and publish. Why: Intercom drafts exist for 3 triggers only; Plain has none.
8. **dimmi joins workflow-owned conversations only through a step.** When a customer-facing workflow claims a conversation, `conversations.ai_mode` becomes `workflow`; 07 answers only while a "Let dimmi handle" step is active. Conversations no customer-facing workflow claims keep the zero-config behaviour from 07 (agent settings), shown in the list as the bottom row "Everything else: dimmi answers" so nobody wonders why the bot spoke. Why: Intercom's "simple deployment overrides workflows" confusion; our solo users still get the bot working without building anything.
9. **Resolved means the customer said so.** dimmi step outcome `resolved` only on explicit confirmation (button "That helped" or a thank-you after an answered reply). Silence is its own path, "went quiet". Why: our published pledge (roadmap standout 15) and the clearest contrast with Fin and Productlane billing silence.
10. **AI is a step, never the engine.** Conditions are deterministic. AI enters through two steps: "Let dimmi handle" and "Let dimmi pick a path" (classify). Both run on the workspace key under the 07 cap, with a fallback path when AI is unavailable. Why: Productlane routines are opaque; Plain meters AI per step.
11. **Workflow changes never trigger the same workflow, and chains stop at depth 3.** Events carry the chain of workflow ids that caused them. Why: Front's rule plus a hard cap; no Zendesk-style "add a nullifying condition".
12. **Humans win.** A teammate's public reply ends every customer-facing run on that conversation (`teammate_replied`), cancels pending bot messages, and leaves background runs alone. Why: the customer must never get a scripted message after a person answered.
13. **Messages from workflows are system messages.** `author_type = system`, `substantive = false`, shown with the workspace name and logo. Never billed (12 D2). Why: routing bots are plumbing; pricing pledge "same price bot or human" stays honest.
14. **Exactly-once where we own the effect.** DB effects run in the same transaction as the run cursor advance. Customer messages carry dedupe key `wf:<run>:<step>`. HTTP requests carry a stable idempotency key; at-least-once by nature, documented.
15. **Channel parity by degradation.** Buttons on widget are buttons; everywhere else numbered choices that accept the number, the label, or free text.
16. **Office hours and SLAs live here.** Named schedules (unlimited, holidays) and SLA policies are owned by 17; 07 reads schedules; 03 exposes SLA state on conversations.
17. **Workflows as code.** Definitions are stable JSON; `dimmi workflows export/apply` round-trips them so teams can review automation in pull requests and publish from CI. Why: our audience is developers; Intercom has no API at all.

## Prototype: keep and change

Keep (right as built): trigger groups with tints, trigger picked first and immutable, tree canvas on React Flow laid out by d3 tidy tree (`flow-layout.ts`), cards that state what they do in a sentence, inspector beside the canvas, statuses draft / live / paused, version number in the header, list order as priority, templates.

Change:

| Prototype | Spec |
|---|---|
| "Customer visits a page" trigger | removed (15 chat announcements own proactive messages) |
| "Customer opens the messenger" | renamed "Customer starts a new conversation" (opener): fires when the widget shows a new conversation, before typing; no conversation row until the customer acts |
| "dimmi closes a conversation" | "dimmi resolves a conversation" (confirmed resolution only) |
| missing triggers | add "Customer writes on a closed conversation", "SLA about to breach", "SLA breached" |
| dimmi step paths Resolved / Hands over / No answer | Resolved / Handed over / Went quiet. "No answer" is a handover reason, branchable via `dimmi.handover_reason` |
| dimmi follow-up fixed at 4 minutes | `quiet_after` setting, default 10 min chat, 1 day email |
| wait step one exit, "stop when the customer replies" toggle | two paths: "Time's up" and "They replied" (second optional), interrupt sources configurable |
| buttons step | adds optional "Typed instead" path and "no answer in" timeout path |
| trigger inspector: channels choice, 3 audience presets, office hours toggle, once per conversation | channels multi-select, shared audience builder `resources/js/features/audiences/` from 18 M4 (replaces presets; same for branch), schedule picker (any, during, outside named schedule), re-entry (once per conversation, every time, cooldown), exclusive toggle, computed kind badge |
| step settings as `Record<string,string>` parsed back from the sentence | typed settings from the server definition (catalog.md); card sentence rendered from settings |
| local state, fake runs and versions | autosaved working copy (`PATCH /workflows/{wf}`), publish via `POST /workflows/{wf}/publications`, live stats |
| missing steps | snooze, unassign, untag, update contact, apply SLA, HTTP request, AI classify, run workflow |
| missing views | Runs tab (list and trace), per-step counters on cards, Simulate panel, "Why didn't it run?" on conversations, version history with diff and rollback, copy and paste of steps and paths, presence of other editors |
| list | grouped by trigger group, ordered within; pinned bottom row "Everything else: dimmi answers" linking to agent settings |

## Data model

All tables: `workspace_id uuid not null references workspaces(id) on delete cascade`, indexes lead with `workspace_id`, RLS in cloud (tenant policy from security.md), `created_at`, `updated_at`. Migration block `20261101001701` .. `001799`. Public ID prefixes (add to `00-foundation/data-model.md` registry and `app/models/public_ids.go`; checked against registry and reserved list, no collision): `wf_` workflows, `wfv_` workflow_versions, `wfr_` workflow_runs, `sch_` schedules, `sla_` sla_policies.

### workflows (`20261101001701`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | `wf_` |
| workspace_id | uuid not null | |
| name | text not null | 1 to 80 chars; unique `(workspace_id, lower(name)) WHERE deleted_at IS NULL` |
| description | text | max 500 |
| trigger | text not null | registry enum (`enums.WorkflowTrigger`, catalog.md); format CHECK `^[a-z][a-z0-9_.]{1,39}$`; immutable (action refuses change, 422 `trigger_immutable`) |
| status | text not null default 'draft' CHECK in ('draft','live','paused') | archived = `deleted_at` |
| position | int not null | list order (priority); gaps allowed; `PATCH /workflows {order}` rewrites |
| draft | jsonb not null | working copy `models.WorkflowSpec` (trigger settings, audience, channels, schedule, reentry, exclusive, definition); max 256 KB |
| draft_hash | text not null | sha256 of canonical JSON; `has_unpublished_changes = draft_hash <> live version spec_hash` |
| live_version_id | uuid null references workflow_versions on delete set null | FK added in `001702` |
| lock_version | int not null default 1 | optimistic concurrency for builder saves (409 `edit_conflict`) |
| kind | text not null default 'background' CHECK in ('customer_facing','background') | of the draft, recomputed on save; versions store their own |
| created_by_member_id, updated_by_member_id | uuid null references workspace_members on delete set null | |
| paused_at, published_at | timestamptz | |
| deleted_at | timestamptz | archive; refused while referenced by a live workflow's `run_workflow` step (409 `workflow_in_use`) |

Indexes: `(workspace_id, deleted_at, position)`, `(workspace_id, trigger) WHERE status = 'live' AND deleted_at IS NULL`.

### workflow_versions (`20261101001702`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `wfv_` |
| workspace_id | uuid not null | |
| workflow_id | uuid not null references workflows on delete cascade | |
| number | int not null | 1, 2, 3 per workflow; unique `(workflow_id, number)` |
| spec | jsonb not null | frozen `models.WorkflowSpec` incl. `trigger` |
| spec_hash | text not null | |
| kind | text not null CHECK in ('customer_facing','background') | |
| compiled | jsonb not null | engine form: step index by id, parent links, path order, referenced ids (teams, members, tags, schedules, SLA policies, workflows, connections), customer-facing step ids; rebuilt never (immutable) |
| note | text | max 280, publish note |
| published_by_member_id | uuid null | |
| published_via | text | `ActorVia.Kind` of publisher (session, cli, mcp, personal_token) |
| restored_from_version_id | uuid null | rollback lineage |
| created_at | timestamptz | no `updated_at` (immutable; lint exception `-- lint:immutable`) |

Index `(workspace_id, workflow_id, number desc)`.

### workflow_runs (`20261101001703`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `wfr_` |
| workspace_id | uuid not null | |
| workflow_id | uuid not null references workflows on delete cascade | |
| version_id | uuid not null references workflow_versions on delete restrict | |
| conversation_id | uuid null references conversations on delete cascade | always set at MVP (openers start their run when the conversation is created; simulations are never stored); nullable so later `workflow-contact-events` needs no migration |
| contact_id | uuid null references contacts on delete set null | |
| kind | text not null CHECK in ('customer_facing','background') | copy of version kind |
| mode | text not null default 'live' CHECK in ('live','test') | `test` = draft running on test conversation (engine.md "Drafts on test conversations") |
| status | text not null CHECK in ('running','waiting','completed','passed','exited','canceled','failed','expired') | engine.md "Run lifecycle" |
| end_reason | text | registry enum `enums.WorkflowRunEndReason` (engine.md) |
| trigger_event_id | text | `evt_` id of the event that started it (idempotency of start); unique `(workflow_id, trigger_event_id)` where not null |
| trigger_data | jsonb not null default '{}' | trigger payload snapshot (catalog.md), max 16 KB |
| parent_run_id | uuid null references workflow_runs on delete set null | reusable call |
| parent_step_id | text | caller step |
| chain | uuid[] not null default '{}' | workflow ids whose changes caused this run (loop guard), max 3 |
| cursor_step_id | text | step being executed or waited on; null when ended |
| seq | bigint not null default 0 | bumped on every advance; jobs carry expected seq |
| wait | jsonb | `{kind: timer|input|dimmi|http_retry|child, until, interrupt: [...], step_id}` while `waiting` |
| wake_at | timestamptz | timer due time (waiting on timer, input timeout, dimmi quiet, http retry) |
| vars | jsonb not null default '{}' | run variables; max 50 keys, 1 KB per value, 16 KB total |
| steps_executed | int not null default 0 | budget guard |
| started_at, ended_at | timestamptz | |
| error | jsonb | `{code, message, step_id}` on failed |

Indexes: partial unique `(workflow_id, conversation_id) WHERE status IN ('running','waiting')` (one active run per workflow per conversation); `(workspace_id, conversation_id, started_at desc)`; `(workspace_id, workflow_id, started_at desc)`; `(wake_at) WHERE status = 'waiting' AND wake_at IS NOT NULL` (reconciler); `(workspace_id, status) WHERE status IN ('running','waiting')`.

### workflow_run_steps (`20261101001704`)

Trace. One row per step execution (a step retried after a transient error keeps one row, `attempts` bumped).

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | no public prefix (exposed nested in run resource with `step_id`) |
| workspace_id | uuid not null | |
| run_id | uuid not null references workflow_runs on delete cascade | |
| workflow_id, version_id | uuid not null | denormalized for analytics |
| step_id | text not null | definition step id |
| kind | text not null | step kind |
| status | text not null CHECK in ('completed','waiting','skipped','failed','interrupted') | |
| path_id | text | path taken (steps with paths) |
| outcome | text | registry enum per kind: `sent`, `assigned`, `no_eligible_member`, `matched`, `else`, `timeout`, `typed`, `resolved`, `handed_over`, `quiet`, `success`, `failure`, `already_known`, `invalid_answer`, ... (catalog.md per step) |
| detail | jsonb not null default '{}' | small facts for the trace: chosen member, tag ids, message id, HTTP status and latency (never bodies), classify label + run id, condition evaluation summary (which rule failed); max 4 KB; PII-bearing keys listed in eraser |
| attempts | smallint not null default 1 | |
| started_at, finished_at | timestamptz | |

Index `(workspace_id, run_id, started_at)`, `(workspace_id, workflow_id, version_id, step_id, started_at)`. Retention: `workflows.trace_retention_days` (90) via `prune_workflow_traces`; runs kept, `steps` include returns `pruned: true`.

### workflow_match_logs (`20261101001705`)

Answers "why did (not) this workflow run". One row per (event, conversation) evaluation listing every live workflow whose trigger matched the event and its verdict. Not one row per workflow (volume).

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | |
| workspace_id | uuid not null | |
| conversation_id | uuid not null references conversations on delete cascade | |
| trigger | text not null | |
| event_id | text | `evt_` |
| verdicts | jsonb not null | `[{workflow_id, version_id, verdict, reason, failed_rule}]`; verdict in `started`, `skipped`; reasons engine.md "Skip reasons"; max 50 entries |
| created_at | timestamptz | |

Index `(workspace_id, conversation_id, created_at desc)`. Retention 30 days (`workflows.match_log_retention_days`). Only written when at least one live workflow has the trigger (no rows for workspaces without workflows).

### workflow_stats_daily (`20261101001706`)

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid not null | |
| workflow_id | uuid not null references workflows on delete cascade | |
| version_id | uuid not null | |
| day | date not null | workspace timezone |
| step_id | text not null | `_run` row holds run totals |
| entered, completed, failed, interrupted | int not null default 0 | per step; `_run`: started, completed, passed, exited, canceled, failed, expired in `outcomes` |
| outcomes | jsonb not null default '{}' | path id or outcome -> count |
| duration_ms_sum, duration_count | bigint | median not stored; p50 computed from trace while retained |

PK `(workflow_id, version_id, day, step_id)`. Built by `rollup_workflow_stats` (every 5 min, idempotent recompute of the last 2 days from `workflow_run_steps`; older days frozen). Kept forever (small).

### schedules (`20261101001707`)

Office hours.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `sch_` |
| workspace_id | uuid not null | |
| name | text not null | unique per workspace, 1 to 60 |
| timezone | text not null | IANA, default workspace timezone |
| weekly | jsonb not null | `{"mon": [["09:00","17:30"]], ...}`; max 4 ranges per day, `24:00` allowed as end; empty week invalid |
| holidays | jsonb not null default '[]' | `[{"date": "2026-12-25", "name": "Christmas"}]` or ranges `{from, to}`; max 100 entries; closed all day |
| is_default | boolean not null default false | partial unique `(workspace_id) WHERE is_default AND deleted_at IS NULL`; created by workspace initializer ("Office hours", Mon-Fri 09:00-17:00 workspace timezone) |
| deleted_at | timestamptz | refused while referenced by live workflow, SLA policy or 07 agent (409 `schedule_in_use`) |

Evaluation in `app/support/officehours` (07 already plans it): `IsOpen(t)`, `NextOpen(t)`, `NextClose(t)`, `BusinessDuration(from, to)`, `AddBusiness(t, d)`; DST rules from foundation (nonexistent local times round forward).

### sla_policies (`20261101001708`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | `sla_` |
| workspace_id | uuid not null | |
| name | text not null | unique per workspace |
| first_response_minutes, next_response_minutes, resolution_minutes | int null | at least one set; 1 to 525,600 |
| schedule_id | uuid null references schedules | null = 24/7 calendar time |
| warn_before_minutes | int not null default 15 | 0 = no warning |
| count_ai_replies | boolean not null default false | dimmi substantive answers count as response (honest default: no) |
| pause_when_snoozed | boolean not null default true | resolution clock paused while snoozed |
| deleted_at | timestamptz | applied targets keep running (policy snapshot on target row) |

### conversation_sla_targets (`20261101001709`)

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid not null | |
| conversation_id | uuid not null references conversations on delete cascade | |
| target | text not null CHECK in ('first_response','next_response','resolution') | |
| sla_policy_id | uuid null references sla_policies on delete set null | |
| policy_snapshot | jsonb not null | minutes, schedule id, flags at apply time |
| state | text not null CHECK in ('active','paused','met','breached','canceled') | `breached` stays breached when met late (`met_at` set) |
| started_at | timestamptz not null | clock start |
| due_at | timestamptz | null while paused |
| remaining_seconds | int | business seconds left when paused |
| warned_at, breached_at, met_at | timestamptz | |
| applied_by_run_id | uuid null references workflow_runs on delete set null | |

PK `(conversation_id, target)`. Index `(workspace_id, state, due_at) WHERE state = 'active'`.

### Core table changes

| Migration | Change | Why |
|---|---|---|
| `20261101001711_add_workflow_columns_to_conversations.sql` | `conversations.workflow_run_id uuid null references workflow_runs on delete set null` (current customer-facing claim); `conversations.ai_mode text not null default 'auto' CHECK in ('auto','workflow')`; `conversations.sla_due_at timestamptz` (earliest active target, sort and filter); index `(workspace_id, sla_due_at) WHERE sla_due_at IS NOT NULL AND status = 'open'` | claim, dimmi ownership, SLA sort |
| `20261101001712_move_agent_office_hours.sql` | expand: `ai_agents.schedule_id uuid null references schedules`; backfill: each agent with non-null `office_hours` gets a schedule "dimmi hours" and the column set; contract (next release): drop `ai_agents.office_hours` | office hours owned here (07 open question 1 closed) |
| `20261101001713_add_schedule_fk_to_widget_settings.sql` | FK `widget_settings.schedule_id` references `schedules` on delete set null (column from 01) | per-widget schedule |
| jsonb keys | `messages.metadata.workflow {workflow_id, version, run_id, step_id, buttons?: [{id,label}], input?: {kind, step_id}}`; `messages.event` names `workflow.run_started`, `workflow.run_ended`, `workflow.step_failed`, `sla.applied`, `sla.warning`, `sla.breached`, `sla.met` | timeline rows (03 registry) |
| registry values | `ActorVia.Kind` adds `workflow` (ID = run id); `audit_logs.via_kind` accepts it (registry, no migration) | actor of every workflow action |

## Code map

```
app/enums/        workflow_trigger.go (registry), workflow_step_kind.go (registry), workflow_status.go,
                  workflow_run_status.go, workflow_run_end_reason.go (registry), workflow_kind.go, sla_target.go
app/models/       workflow_spec.go (typed definition, settings per step kind, canonical JSON + hash), sqlc output
app/support/workflowspec/   parse, validate (per trigger, per step, references, limits), compile, kind, diff
app/workflows/fields.go     registers conversation, message, trigger, dimmi, office hours, run fields into `app/support/audience`
app/support/officehours/    schedule math (shared with 07)
app/support/workflowvars/   variable rendering {{contact.first_name | fallback: "there"}} (reuses 03 saved-reply renderer)
app/workflows/              engine (product package, no HTTP):
  matcher.go                  candidates for (trigger, conversation), audience, schedule, reentry, chain
  claimer.go                  InTx claim for start triggers
  runner.go                   AdvanceRun: lock, load, execute until block/end, persist
  steps/                      one file per step kind: Execute(ctx, tx, run, step) (Result, error)
  waits.go timers.go interrupts.go outcomes.go simulator.go explainer.go
app/actions/workflows/      CreateWorkflow, UpdateWorkflow, PublishWorkflow, SetWorkflowStatus, RestoreWorkflowVersion,
                            ArchiveWorkflow, ReorderWorkflows, StartWorkflowRun (manual/API), CancelWorkflowRun,
                            SimulateWorkflow, ExplainConversationMatches
app/actions/schedules/      CRUD; app/actions/slas/ CRUD, ApplySlaPolicy, RecomputeSlaTargets
app/listeners/              MatchWorkflowsInTx (InTx, start triggers), MatchWorkflows (queued, all other triggers),
                            InterruptWorkflowRuns (InTx: teammate reply, close, spam, merge, delete),
                            ResumeWorkflowOnInput (queued: contact message, button answer),
                            ResumeWorkflowOnDimmiOutcome (queued: ai.state_changed, ai.handoff_requested, ai.run_completed),
                            ArmQuietTimers (queued), TrackSlaTargets (InTx: replies, status changes)
app/jobs/                   advance_workflow_run, resume_workflow_run, fire_quiet_trigger, check_sla_target,
                            reconcile_workflow_timers, rollup_workflow_stats, prune_workflow_traces, notify_workflow_failing
app/http/controllers/api/v1/ workflow_controller, workflow_publication_controller, workflow_version_controller,
                            workflow_run_controller, workflow_simulation_controller, workflow_stats_controller,
                            conversation_workflow_run_controller, conversation_workflow_match_controller,
                            schedule_controller, sla_policy_controller, workflow_template_controller
app/http/controllers/widget/v1/ workflow_opener_controller
app/mcp/tools/workflows/    toolset `workflows`
app/policies/               workflow_policy.go, schedule_policy.go, sla_policy_policy.go
resources/workflows/templates/*.json   shipped templates (validated in CI against the spec parser)
cmd (09 CLI)                `dimmi workflows ...`
```

No new framework package. Engine is product code (uses `framework/queue`, `framework/events`, `framework/clock`, `framework/httpclient`, `framework/webhooks`, `framework/database` advisory locks). Reuse check: River already gives durable scheduled jobs, uniqueness and retries; a workflow library (Temporal, go-workflows, dbos) was rejected: extra service or schema, replay determinism rules on step code, binary size. Our runs are short state machines over our own tables; a cursor plus River is enough.

## Events

### Emitted

| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| workflow.created / workflow.updated / workflow.deleted | yes (`workspace:`) | no | updated carries `lock_version`, `has_unpublished_changes`; audit via `WriteAuditLog` |
| workflow.published | yes | yes | `{workflow_id, version_id, number, note, kind}` |
| workflow.status_changed | yes | yes | live/paused, `canceled_runs` count |
| workflow.failing | yes (`workspace:`) | yes | failure rate over 5% (min 20 runs) in the last hour, or any `failed` run of an HTTP step 10 times in an hour; drives 06 notification to `workflows:manage` holders; at most once per workflow per 6 h |
| workflow_run.started | yes (`conversation:`) | no | timeline + inbox "Workflow X is running" chip |
| workflow_run.updated | yes (`conversation:`) | no | cursor moved, waiting on what; coalesced 250 ms |
| workflow_run.completed | yes | yes | ended in `completed` or `passed` |
| workflow_run.ended | yes | yes | ended `exited`, `canceled`, `expired`, `failed` with `end_reason`, `error` |
| schedule.created / updated / deleted | yes | no | 07 cache bust |
| sla_policy.created / updated / deleted | yes | no | |
| conversation.sla_applied | yes | yes | targets with due times |
| conversation.sla_warning | yes | yes | target, due_at; drives 06 notification to assignee (or team) |
| conversation.sla_breached | yes | yes | target, due_at, breached_at |
| conversation.sla_met | yes | no | |

Add all to `00-foundation/events-and-realtime.md` "Events added by feature specs" in the same PR. Events of actions taken by workflows are the normal 03 events (`conversation.assigned` etc.) with `actor = {type: system, via: {kind: workflow, id: wfr_}}` plus `workflow_chain` (ids) on the struct for the loop guard.

### Consumed

| Event | Listener | Effect |
|---|---|---|
| conversation.created, message.created (contact, first in conversation or after reopen) | `MatchWorkflowsInTx` (InTx) | claim for start triggers (engine.md) |
| message.created, conversation.assigned/closed/reopened/snoozed/unsnoozed, survey_response.completed (15 conversation CSAT), ai.handoff_requested, ai.state_changed, conversation.sla_warning/breached | `MatchWorkflows` (queued `workflows`) | background and in-conversation triggers |
| message.created (member public reply), conversation.closed, marked_spam, deleted, merged, contact.blocked | `InterruptWorkflowRuns` (InTx) | exit or cancel runs, clear claim (engine.md "Mid-run changes") |
| message.created (contact), widget button answer | `ResumeWorkflowOnInput` | resume input waits |
| ai.handoff_requested, ai.state_changed, ai.run_completed | `ResumeWorkflowOnDimmiOutcome` | dimmi step outcomes |
| member.deactivated, team.deleted, tag.deleted | `FlagWorkflowReferences` (queued) | mark live workflows with broken references (`warnings` on resource, 06 notify); runs fall back per step (catalog.md) |

Import rule (16): imported conversations and messages dispatch nothing; listeners also skip `metadata.imported` (defense). Workflows never run on imported data.

## Jobs

| Job (kind) | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| `advance_workflow_run` | `workflows` | run created or resumed | 10, backoff capped 5 min; engine errors classified (engine.md) | `ByArgs` (run_id) while available/scheduled/retryable; carries `expected_seq` |
| `resume_workflow_run` | `workflows` | timer due (`ScheduledAt = wake_at`) | 5 | `ByArgs` (run_id, seq) |
| `fire_quiet_trigger` | `workflows` | `ArmQuietTimers` on public reply / customer message | 3 | `ByArgs` (conversation_id, trigger, anchor_message_id) |
| `check_sla_target` | `workflows` | target applied / recomputed, `ScheduledAt` = warn time and due time | 5 | `ByArgs` (conversation_id, target, due_at, phase) |
| `reconcile_workflow_timers` | `maintenance` | every minute | 1 | `ByPeriod` 1 min; runs `waiting` with `wake_at < now - 2 min`, SLA targets `due_at < now - 2 min` still active, quiet timers missed; metric `dimmi_workflow_timer_lag_seconds` |
| `rollup_workflow_stats` | `maintenance` | every 5 min | 3 | `ByPeriod` 5 min |
| `prune_workflow_traces` | `maintenance` | daily | 3 | `RetentionGuard.MayDestroy` per batch |
| `notify_workflow_failing` | `notifications` | `workflow.failing` | 3 | per workflow per 6 h |
| `send_workflow_http_request` | `workflows` | HTTP step | engine.md (3 attempts in 2 min) | `ByArgs` (run_id, step_id) |

New queue `workflows` in `config/queue.go` (default concurrency 20 per node). Add to `00-conventions` queue list.

## Config keys (`config/workflows.go`)

| Key | Env | Default | Notes |
|---|---|---|---|
| `workflows.queue_concurrency` | `DIMMI_WORKFLOWS_QUEUE_CONCURRENCY` | 20 | per node |
| `workflows.max_steps_per_run` | | 200 | run fails `step_budget` |
| `workflows.max_steps_per_conversation_hour` | | 300 | across runs; new runs skipped `conversation_budget` |
| `workflows.max_bot_messages_per_conversation_hour` | | 20 | customer-facing loop guard |
| `workflows.run_max_lifetime` | | 30d | waiting runs expire (`expired`) |
| `workflows.input_debounce` | | 2s | collect waits for follow-up text before evaluating |
| `workflows.trace_retention_days` | | 90 | |
| `workflows.match_log_retention_days` | | 30 | |
| `workflows.http.timeout` | | 10s | per attempt |
| `workflows.http.max_response_bytes` | | 65536 | |
| `workflows.simulation.max_steps` | | 200 | |

## Limits

Entitlement catalog keys (add to `00-foundation/contracts.md` catalog; pricing promise: limits, never paywalls; every feature in both editions):

| Key | Kind | Self-host | Cloud default |
|---|---|---|---|
| `workflows.max_workflows` | limit | -1 | 200 (non-archived) |
| `workflows.max_live` | limit | -1 | 100 |
| `workflows.workspace_concurrency` | limit (running advance jobs) | config (10) | 10 |
| `workflows.step_executions_hourly` | limit (hourly) | -1 | 50,000 |
| `workflows.http_requests_hourly` | limit (hourly) | -1 | 5,000 |
| `schedules.max_per_workspace` | limit | -1 | 50 |
| `sla.max_policies` | limit | -1 | 50 |

Fixed product limits (not entitlements, same in both editions; 422 `workflow_invalid` with error code): 150 steps per definition, path nesting depth 12, 10 paths per branch / classify step, 6 buttons per step (widget), 5 numbered choices recommended on text channels (6 max), 50 condition leaves and depth 4 per condition tree (18 limits), message 4,000 chars, note 10,000 chars, wait 1 minute to 30 days, reusable call depth 3, chain depth 3, run variables 50 keys, 10 response mappings per HTTP step, 20 tags per tag step, 256 KB definition. Hourly cap over: new runs skipped with reason `rate_limited` (match log), 06 notification once per hour; running runs continue.

## Permissions

Registered in 00-accounts catalog (`app/auth/permissions.go`); policies call `authz.Can`, never role names.

| Permission | Ability group | Roles | Gates |
|---|---|---|---|
| `workflows:read` | workflows:read | owner, admin, member | list, show, versions, runs, stats, match explainer, simulations |
| `workflows:manage` | workflows:write | owner, admin | create, edit drafts, publish, pause, rollback, archive, reorder, templates |
| `workflows:run` | workflows:write | owner, admin, member | start a reusable workflow on a conversation (Cmd+K, API), cancel a run on a conversation they can see |
| `schedules:manage` | settings:write | owner, admin | schedules CRUD |
| `sla:manage` | settings:write | owner, admin | SLA policies CRUD; remove an SLA from a conversation |

Members read everything (they need to understand why a conversation was routed). Publishing from MCP always confirms (09 confirm gate, risk `destructive`: changes customer-facing behaviour). Publishing a workflow with an HTTP step to a raw URL or a new connection needs `workflows:manage` plus `settings:write` (same bar as 10 webhook endpoints). Audit log: every create, update (coalesced per member per 10 min), publish, status change, rollback, archive, reorder, manual run start and cancel.

## Metering (pricing.md, 12)

- Never metered: runs, steps, HTTP calls, simulations.
- Workflow customer messages: `author_type system`, `substantive false` (D2): never activate a billable conversation. 12 `MeterMessageActivity` unchanged (already ignores non-substantive).
- dimmi step and classify step: 07 spend (BYOK tokens) under the 07 cap, recorded as `ai_runs` (kind `answer` for dimmi, `workflow_classify` new 07 run kind for classify). AI answer messages bill exactly as 07 today (D6).
- Workspace activations paused (12): customer-facing steps still send (non-substantive), dimmi step takes the handed-over path with reason `billing_paused` (07 gate 9b).

## Security and privacy

- HTTP step: `a.HTTP` SSRF-safe client (no private IPs on cloud; self-host allowlist from 07 `ai.actions` config), https only on cloud, no redirects, 10 s timeout, 64 KB response cap, signed with Standard Webhooks (`framework/webhooks`) using workspace secret `workflows.signing_secret` (KeyManager scope `workspace`), or 07 connection auth. Response bodies never stored; mapped values stored in run vars (PII rules apply).
- Variables in customer messages render escaped (03 renderer); contact-supplied values never become links.
- Collect step never verifies or merges identity: collected email sets `contacts.email` only when empty and `email_verified = false`, adds unverified identity; never triggers 03 auto-merge (foundation merge rule).
- Contact eraser hook (contracts.md): erase `workflow_runs.vars`, `trigger_data`, `workflow_run_steps.detail` PII keys for the contact's runs.
- Export hook: contact export includes collected answers.
- Customer text in conditions: evaluated as data; AI classify wraps customer text as untrusted (07 prompt rules) and returns only a label from the configured list.

## Dependencies and changes to other specs

| Spec | Change (owner applies in own files when this lands) |
|---|---|
| 00-foundation | events rows; prefixes `wf_ wfv_ wfr_ sch_ sla_`; entitlement keys; `ActorVia.Kind` `workflow`; queue `workflows`; contact eraser + export hooks |
| 00-accounts | permissions `workflows:read`, `workflows:manage`, `workflows:run`, `schedules:manage`, `sla:manage`; ability groups `workflows:read`, `workflows:write`; workspace initializer creates default schedule |
| 01 widget | config gains `workflow_openers` (compiled opener carrying residual from 18 `audience.Partial`, never rule shape); `POST /widget/v1/conversations` accepts `opener`; messages endpoint accepts `workflow_answer`; renders `metadata.workflow.buttons` as quick replies, hides composer when the step disables typing; `office_hours_text` stays free text but widget availability reads the default schedule |
| 03 inbox | `ai_mode`, `workflow_run_id`, `sla_due_at` columns; resource blocks `workflow` (active run summary) and `sla` (targets); sort `sla_due` and filter `sla_state`; timeline event names; `InterruptWorkflowRuns` listener order before `SendChannelOutbound` |
| 04 email | workflow system messages sent in thread with `Auto-Submitted: auto-replied`; inbound auto-replies (`substantive false`) never resume or trigger workflows |
| 05 chat apps | numbered-choice rendering is plain text, nothing new; bot-authored inbound ignored as today |
| 06 notifications | notification types `workflow_failing`, `sla_warning`, `sla_breached` |
| 07 dimmi | gate rule 2c `workflow_controls_ai` (`ai_mode = workflow` and no active dimmi step: skip); `ActivateConversationAi(reason workflow, step settings)`; handoff inside a dimmi step skips agent `handoff.team_id` routing (workflow path routes) and skips the notice when the handed-over path starts with a customer-facing step; resolution confirmation buttons on widget answers when step `confirm_resolution`; run kind `workflow_classify`; reads `schedule_id` instead of `office_hours` |
| 09 agent-native | MCP toolset `workflows`; CLI `dimmi workflows` |
| 10 webhooks | catalog rows `workflow.published`, `workflow.status_changed`, `workflow.failing`, `workflow_run.completed`, `workflow_run.ended`, `conversation.sla_*`; fixtures |
| 12 billing | nothing new (system messages non-substantive) |
| 15 engage | CSAT `survey_response.completed` on conversation surveys feeds trigger `conversation.rated`; close step "ask for rating" calls 15 `RequestConversationCsat` |
| 16 importers | later `import-workflows`; nothing at MVP |
| roadmap | "rules lite" and "workflows, SLAs" are one launch item (decided) |

## Where we beat them

| Gap found | Who | Dimmi |
|---|---|---|
| No API to manage or trigger workflows; "troubleshooting is guesswork" | Intercom | full REST, CLI, MCP; manual and API runs of reusable workflows; run trace per step; match explainer per conversation |
| Drafts only on 3 triggers; others edit live | Intercom | drafts and versions on every trigger, simulate, test conversations, rollback |
| No drafts, versions, test runs or run logs | Plain | all of it |
| Background conflicts have no defined winner | Intercom | serialized in list order per conversation; deterministic |
| Matching workflow with dead branches blocks lower ones | Intercom | `passed` outcome hands the conversation to the next match |
| Workflow runs twice per event set; each event starts its own run | Plain | one active run per workflow per conversation; explicit re-entry rule |
| Steps unavailable on email and WhatsApp | Intercom | every step on every channel; numbered choices |
| No copy and paste of steps or paths | Intercom | copy and paste across paths and workflows (clipboard holds definition JSON) |
| Silence billed as resolution | Intercom, Productlane | resolved only on confirmation; silence is its own path; nothing billed per outcome |
| AI-only automation, opaque; AI steps metered in credits | Productlane, Plain | deterministic engine with two explicit AI steps on your own key at cost |
| No time-based rules, no round robin | Productlane | quiet triggers, waits, SLA triggers, round robin and balanced assignment |
| SLAs on top plans only; breach triggers beta, on request | Intercom (Expert), Plain (Horizon) | SLA policies, warnings and breach triggers for everyone, both editions |
| Auto-pause on billing lapse, manual re-enable | Intercom | workflows never pause for billing; AI steps fall back to handover |
| Fin keeps answering after notes and reassignment | Intercom | dimmi stops on any teammate public reply and on member pause; workflow takes the handed-over path |
| No workflows as code | all | `dimmi workflows export/apply`, JSON in the repo, publish from CI |

## Open questions (recommended answer in bold)

1. Roadmap placement. Decided by the owner on 2026-11-04: workflows are in launch, as one item replacing rules lite, built after audiences core.
2. Should workflow bot messages count as substantive (billable) when they carry real content? **No: always non-substantive.** Abuse is bounded by step limits and the conversation price model.
3. Keep automatic dimmi (07 settings) as the fallback for unclaimed conversations, or require a workflow for dimmi to speak at all (prototype comment says "only through a step")? **Keep automatic as fallback; visible as the bottom row of the list.** Zero-config matters for solo users.
4. Drop "Customer visits a page" and leave proactive messages to 15 chat announcements? **Yes.**
5. Tree only at MVP, rejoining paths later? **Yes; reusable workflows and copy and paste cover duplication.**
6. dimmi "resolved" only on explicit confirmation (button or thank-you after an answer)? **Yes; silence goes down "went quiet".**
7. May workflow changes trigger other workflows? **Yes, never the same workflow, chain depth 3.**
8. Can members (not admins) edit drafts? **No at MVP: members read, run reusable workflows, see traces. Revisit with custom roles.**
9. Own office hours and SLA policies here (and move `ai_agents.office_hours`)? **Yes.**
10. Ship the AI classify step at MVP? **Yes; it replaces most keyword branching and costs one cheap classifier call on the workspace key.**
11. Native buttons on Slack, Telegram, WhatsApp, Discord at MVP? **No; numbered choices everywhere, native later (`native-channel-buttons`).**
12. `dimmi workflows export/apply` (workflows as code) at MVP? **Yes; thin over REST, strong fit for our audience.**
13. SLA `count_ai_replies` default? **false: an AI answer does not satisfy a human SLA unless the admin says so.**

## Decisions 2026-11-07 (first build)

Owner ordered workflows before audiences, so the build starts from the parts of 18 workflows need.

- **Rule language first.** `app/support/audience` (parse, catalog, surfaces, compile, three-valued evaluate, partial, explain, saved audience leaves) and `app/support/urlmatch` ship now. 18's SQL compiler, lint, attribute definitions and saved audience storage come with audiences. Workflow fields register in `workflowspec.Catalog()` (not `app/workflows/fields.go`): the validator and the engine share one catalog without an import cycle.
- **Built:** definitions, validator, CRUD, publish, versions, rollback, pause, archive, reorder (M1 minus schedules CRUD and entitlements); engine core (matcher with verdicts and match logs, claims, runner with advisory lock, sequence and trace, step budget, loop guard via `WorkflowChain` on every event base); steps message, buttons, collect, branch, wait (duration and time of day), assign (member, team, round robin, balanced via `assignment.PickAssignee`), unassign, tag, untag, priority, note, update contact, snooze, close; triggers first message and customer reopened (in transaction), customer message, teammate message, note, assignment and status changes (queued); teammate reply, close, spam and delete interrupt runs in transaction.
- **Background order:** the runs one event starts advance from one job, one after another in list order, each in its own transaction. Same determinism as the spec's batch links, one job instead of a chain.
- **Live cache:** per-server memo with a 5 s lifetime plus local drop on publish, pause, archive and reorder. Pubsub busting later; 5 s is the bound on a stale server.
- **Not ready, refused at publish with `step_not_available`:** classify, reply time, run workflow, apply SLA, HTTP request, close with rating, schedule restriction, office-hours waits and business-time counting. Each lands with its milestone.
- **Buttons render as numbered text on every channel**, the widget included, until the widget draws `metadata.workflow.buttons`.
- **Fix found on the way:** `message.created` never carried `ConversationCreated`; it now marks a conversation's first message, which also lets notifications tell a new conversation from a reply.
- **Builder wired to the API (M9 first cut).** The builder edits the server definition directly: every change autosaves the working copy with `lock_version` (an `edit_conflict` stops saving until reload), Publish sends the saved `draft_hash` with warnings acknowledged, Pause and Resume set the status, Archive deletes. The picker offers only triggers the engine fires today and the add-step menu only ready kinds the trigger allows. Cards and the inspector show the report's errors; a branch path edits one condition (field, comparison, value), grouped conditions stay API-only. Not yet: reorder by drag, versions panel, templates, test runs, audience editor, schedules.
- **dimmi step (first cut, 2026-11-08).** The run waits with `wait.kind = dimmi`, carrying the rendered instructions, reply limit and `confirm_resolution`; the pipeline reads them from the run on each turn (no copy). Entering the step readies `ai_state` (handed_off to off), queues `answer_for_workflow` for a waiting customer message, and arms the quiet timer (`quiet_after`, default 10 m, email 1 d). Outcomes: `ai.handoff_requested` resumes `handed_over` (agent routing and priority skipped inside the step); a short confirmation (`app/support/resolution`) right after an agent answer resumes `resolved` (agent skips it as `resolution_confirmed`), closing when the resolved path is empty and `close_on_resolved`; the timer takes `quiet` once the last agent answer went unanswered for `quiet_after`. Confirmation is the text line "Did that solve it? Reply yes if it did." on every channel until the widget draws buttons. Agent off or AI disabled: `handed_over` at once (reason `agent_disabled`), publish warns `dimmi_not_ready`. Not yet: `handover_notice: workflow` and turning off the email question (refused), the 6 s summary wait, `dimmi.summary` / `dimmi.handover_reason` path variables, `dimmi.resolved` trigger.
- **Queue holds jobs on the app clock.** `queue.At(t)` now holds a job for `t - clock.Now()` from the real now, so timers behave the same under a test's fake clock as in production.

