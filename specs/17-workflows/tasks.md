# 17 Workflows: tasks

Style: caveman. Ordered. Each about 1 day, merged with tests. Business logic integration-tested only (`tests.New(t)`: real Postgres, Valkey, River, fake clock `tc.Clock`), tests named in plain words, every edge case covered. LLM steps use the 07 `fake` driver only; nothing here calls a real model. Requires 03 core, 07 core (pipeline, handoff, `RunTask`), 18 M1 to M4 (language, evaluator, builder; 17 starts after them), 01 widget messages. Each milestone ends green on `make lint test`.

## M1: Definitions, versions, schedules

- [ ] Migrations `001701`..`001713` (`001710` not used), sqlc queries, factories (`WorkflowFactory` with spec builder, `ScheduleFactory`, `SlaPolicyFactory`), prefixes `wf_ wfv_ wfr_ sch_ sla_`, permissions and ability groups, entitlement keys, queue `workflows`.
  - Accept: migrate up/down; tenant lint; RLS test (workspace B cannot read A's workflows, runs, schedules); prefix registry test.
- [ ] `models.WorkflowSpec` + `app/support/workflowspec` parse, canonical JSON, hash, id assignment, kind computation, compile (index, parents, references).
  - Accept: round-trip test (parse, canonicalize, hash stable across key order); kind matrix per step kind; every template in `resources/workflows/templates` parses.
- [ ] Validator: per trigger allowed steps, opener shape, fields per trigger, variables, references, limits, warnings (catalog.md section 6).
  - Accept: one subtest per error and warning code; edge cases 75, 78 (self-call), 85, 112.
- [ ] Register 17 fields into `app/support/audience` (conversation, message, trigger, dimmi, office hours, run vars); surfaces `workflow.audience`, `workflow.branch`; `audience_references` written on publish.
  - Accept: 18 parser and evaluator used, no second parser (`tests/arch`); missing-value cases 13, 14; `collect` on non-writable key refused at publish; explainer returns per-leaf result.
- [ ] Workflow CRUD API, reorder, archive/unarchive, draft save with `lock_version`, `workflow.created/updated/deleted` events, audit.
  - Accept: feature tests: create from trigger, template, duplicate; 409 `edit_conflict` (79); 422 `trigger_immutable` (85); 403 member (108); reorder `order_stale`; realtime event received by subscribed member client.
- [ ] Publish, versions, diff, pause/resume, rollback, `nothing_to_publish`, `draft_changed`, `workflow_in_use`, entitlement `max_live`.
  - Accept: edge cases 80, 81, 83, 87, 111; version immutability (update attempt fails at DB level); webhook fixture `workflow.published`.
- [ ] Schedules CRUD + `app/support/officehours` (shared with 07) + default schedule initializer + 07 expand migration (`ai_agents.schedule_id`, backfill).
  - Accept: DST tests Berlin, New York, Sydney (`tc.Clock`); holidays; `IsOpen`, `NextOpen`, `AddBusiness`, `BusinessDuration` table tests; edge 55, 56, 103; 07 reads schedule (07 gate 8 test still green).

## M2: Engine core (background)

- [ ] Matcher + live cache (epoch, pubsub bust) + match logs + skip reasons.
  - Accept: edge cases 1, 7, 8, 9, 11, 15, 16, 19; cache bust visible on second app instance within 1 s (two-app harness).
- [ ] Runner: advisory lock, seq, step loop, trace rows, budgets, 25-step chunking, run lifecycle, end reasons, `workflow_run.*` events.
  - Accept: concurrency test (100 events on one conversation, no interleaved effects); stale job no-op; `step_budget` failure; benchmark advance p95 under 50 ms lock hold.
- [ ] Background batch ordering, exclusive flag, `MatchWorkflows` queued listener for team, status, assignment, note, customer message, rated, dimmi handed over triggers.
  - Accept: edge cases 4, 5, 6, 10; deterministic winner test repeated 50 times.
- [ ] Action steps: assign (member, team), unassign, tag, untag, priority, note (mentions -> 06), update contact, snooze, close; actor `system` via `workflow`; fallbacks and warnings.
  - Accept: per step happy path + failure fallback (68, 69, 70); timeline shows "Workflow: <name>"; `FlagWorkflowReferences` on member deactivation sets warnings and notifies once.
- [ ] Loop guard: `WorkflowChain` on 03 event structs, chain depth, per-conversation budgets.
  - Accept: edge cases 72, 73, 74, 77; budget skip `conversation_budget`.
- [ ] Branch step + `passed` semantics.
  - Accept: first-match order, else path, failing-leaf trace detail.
- [ ] Interrupts: `InterruptWorkflowRuns` InTx for teammate reply, close, spam, delete, merge, block, erase.
  - Accept: edge cases 47, 64, 67, 71, 18; listener runs before `SendChannelOutbound` (order test).

## M3: Customer-facing steps and channels

- [ ] Claims + `ai_mode` + `MatchWorkflowsInTx` for first message and customer reopened; 07 gate rule 2c.
  - Accept: edge cases 21, 22, 23, 24; no dimmi run row created when claimed; ingest latency regression test (claim adds under 2 ms p95 with 50 live workflows).
- [ ] Fall-through on `passed` (max 10 hops), `held`, `lower_priority`.
  - Accept: edge cases 2, 3.
- [ ] Message step (system message, dedupe key, delay with typing), reply time step (typical reply cache, buckets, sample floor).
  - Accept: retry of advance never double-sends (kill worker after message insert, rerun); edge 52; email outbound has `Auto-Submitted` (Mailpit header check) (76).
- [ ] Buttons step: widget quick replies (`metadata.workflow.buttons`, `workflow_answer`), numbered text on email and chat apps, matcher (number, label, prefix, whole-word), retry text, typed and timeout paths.
  - Accept: edge cases 37, 38, 39, 40, 41, 42; channel rendering golden files per channel type.
- [ ] Collect step: types, validation, debounce, attempts, skip if known, contact writes without verification or merge.
  - Accept: edge cases 43, 44, 45, 46.
- [ ] Opener: compile into widget config, `POST /widget/v1/conversations` with `opener`, server re-check, stale opener.
  - Accept: edge cases 50, 51; widget config size test (openers add under 2 KB for 5 openers); zero conversations created by viewing.
- [ ] Bot message cap and cancellation of unsent workflow messages on teammate reply.
  - Accept: edge cases 48, 49.

## M4: Time

- [ ] Wait step (duration, until open, time of day, business counting, interrupts, two paths), resume jobs, reconciler.
  - Accept: edge cases 53, 54, 57, 58, 62, 63 with `tc.Clock.Travel`; reconciler recovers deleted River job within 2 min.
- [ ] Quiet triggers: `ArmQuietTimers`, `fire_quiet_trigger` (customer quiet, teammate quiet), anchors, business counting.
  - Accept: edge cases 59, 60, 61.
- [ ] Workflow schedule restriction + `office_hours.open` condition + snooze until open.
  - Accept: edge 16; snooze lands at next open time.
- [ ] Run lifetime expiry and `rollup_workflow_stats`, `prune_workflow_traces` (RetentionGuard), match log retention.
  - Accept: stats recompute idempotent; held rows skipped with fake guard false.

## M5: dimmi and AI

- [ ] 07 changes: `ActivateConversationAi`, gate 2c reads cursor step, step settings into pipeline (instructions, max replies), routing skip inside step, notice suppression, `resolved_confirmed` transition, confirmation quick replies and email footer, run kind `workflow_classify`.
  - Accept: 07 suite green; new 07 tests for each change (fake LLM driver).
- [ ] dimmi step executor + `ResumeWorkflowOnDimmiOutcome` + quiet timer + summary wait.
  - Accept: edge cases 25 to 36 (fake LLM: scripted classify/answer outputs).
- [ ] `dimmi.resolved` and `dimmi.handed_over` triggers.
  - Accept: automatic dimmi (unclaimed) confirmation also fires `dimmi.resolved`.
- [ ] Classify step via `ai.RunTask` (schema, untrusted wrapping, label check, confidence, fallback path, cost on `ai_runs`).
  - Accept: edge cases 93, 94, 95 with fake driver; spend recorded under 07 cap.

## M6: SLAs, assignment, HTTP, reusable

- [ ] SLA policies CRUD, `ApplySlaPolicy`, `TrackSlaTargets`, `check_sla_target`, `sla_due_at`, conversation `sla` block, sort and filter, events, 06 notifications, SLA triggers.
  - Accept: edge cases 96 to 102; warning and breach fire within 5 s of due time with `tc.Clock`; webhook fixtures.
- [ ] Round robin and balanced assignment with availability and cursor locking.
  - Accept: edge cases 104 to 107; two-goroutine race test alternates exactly.
- [ ] HTTP request step: connection or signed raw URL, retries, mapping, idempotency headers, SSRF, hourly cap, signing secret reveal and rotation.
  - Accept: edge cases 88 to 92 with `httpclienttest`; signature verifies with Standard Webhooks lib; no body persisted (DB scan in test).
- [ ] Reusable workflows: `run_workflow` step (call and return, vars copy), manual and API start (`replace`), depth guard.
  - Accept: edge 78; `conversation_held` 409; vars returned to caller.

## M7: Testing, history, analytics

- [ ] Simulation endpoint (dry executors, scripted inputs, per-channel rendering, audience leaf results, `live_ai` gated).
  - Accept: no rows written (row count assert across all tables), p95 under 150 ms on 50-step workflow; simulated round robin pick equals real pick for the same state.
- [ ] Drafts on test conversations (`test_on_test_conversations`, mode `test`, excluded from stats).
  - Accept: edge 86.
- [ ] Runs API (per workflow, per conversation, show with steps), cancel, match explainer with live evaluation.
  - Accept: explainer lists every candidate with reason; cursor pagination; `VisibleScope` 404 (109).
- [ ] Stats API (runs, customer-facing rates, per step, per path, version filter) + `workflow.failing` detection and notification.
  - Accept: stats match a seeded fixture of 1,000 runs exactly; failing fires once per 6 h.

## M8: Surfaces

- [ ] MCP toolset `workflows` with risk levels and confirm gate.
  - Accept: publish and run tools refused without confirmation (110); draft and simulate allowed.
- [ ] CLI `dimmi workflows` (list, show, export, apply with diff and `--dry-run`, simulate, publish, pause, runs, explain).
  - Accept: export -> apply round trip produces zero diff; edge 113; CI exit codes.
- [ ] Webhook catalog rows + fixtures (`workflow.*`, `workflow_run.*`, `conversation.sla_*`), `vars` only with `include_notes` (115).
  - Accept: 10 catalog test green.
- [ ] Templates shipped and instantiated with `$ref` resolution.
  - Accept: every template publishes on a seeded workspace with default team and schedule; edge 87.

## M9: Builder (frontend, `resources/js/features/automation`)

- [ ] Replace local data with TanStack Query over the API; typed settings from `GET /workflow-triggers` and the spec; autosave with `lock_version`, conflict dialog; publish dialog with errors and warnings; version history with diff and rollback.
  - Accept: unit tests for spec <-> canvas mapping (existing `flow-layout`, `edit-steps` tests extended); Biome clean; e2e: create from template, edit, publish, pause, roll back (light and dark).
- [ ] Trigger picker and inspector changes from README "Prototype: keep and change" (new triggers, removed page trigger, shared 18 builder in place of presets, schedule, re-entry, exclusive, new step kinds, typed path and timeout paths, dimmi outcomes).
  - Accept: e2e builds the `dimmi_first` template by hand and publishes it.
- [ ] Copy and paste of steps and paths (clipboard JSON with fresh ids), per-step stats overlay, Runs tab with trace, Simulate panel, editor presence.
  - Accept: e2e copy a path between workflows; simulate shows rendered email and widget variants.
- [ ] Inbox: workflow chip on conversation, "Why didn't a workflow run?" panel, SLA countdown and sort; Cmd+K "Run workflow"; list pinned row "Everything else: dimmi answers".
  - Accept: e2e: VIP conversation shows run trace and SLA due; member runs a reusable workflow from Cmd+K.
- [ ] Manual end-to-end pass on release binary (`docs/testing/manual-e2e.md` rows for every endpoint above).

## Demo script (end of M9)

1. Fresh workspace, paste key (07), knowledge indexed. Workflows list shows only "Everything else: dimmi answers".
2. Start from "dimmi first, then the team". Simulate against a past conversation: see dimmi outcome choices and rendered email vs widget.
3. Enable on test conversations, open widget preview, click "Something else", ask a question, press "That helped": conversation closes, run trace shows each step.
4. Publish. Real widget conversation: customer asks for a human; handed-over path collects email, assigns round robin, shows "back Monday 09:00" (outside hours).
5. Add "Fast lane for VIP" above it (exclusive, SLA VIP 30 min). Enterprise contact writes: VIP workflow wins, SLA countdown in inbox, warning notification at 15 minutes.
6. `dimmi workflows export --dir ./dimmi/workflows`, edit a button label in the JSON, `dimmi workflows apply --publish --note "rename"`: version 2 live, running conversations stay on version 1.
7. Open a conversation where nothing ran: "Why didn't a workflow run?" shows `audience` failed on `company.custom.plan`.
