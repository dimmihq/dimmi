# 17 Workflows: execution engine

Style: caveman. Owner 17. Package `app/workflows` (product code; River via `framework/queue`, events via `framework/events`, time via `framework/clock`).

## 1. Overview

```
domain event (03, 07, 15, 17 SLA)                      widget opener answer (01)
   |                                                         |
   |- InTx  MatchWorkflowsInTx   (start triggers only) ------+--> claim + run row + advance job (same tx)
   |- InTx  InterruptWorkflowRuns (teammate reply, close, spam, delete, merge, block)
   |- Queued MatchWorkflows      (every other trigger) --> runs + advance jobs
   |- Queued ResumeWorkflowOnInput / OnDimmiOutcome     --> advance job
timers: River scheduled jobs (resume, quiet triggers, SLA checks) + reconciler every minute

advance_workflow_run(run, expected_seq):
  tx { advisory lock conversation; load run (seq check); load version (cache); loop:
         execute step -> Result{continue | path | wait | end}
         write trace row, cursor, seq+1, effects (03 actions in same tx)
       until wait or end or budget; dispatch events }
```

## 2. Trigger mapping

| Trigger | Source event (phase) |
|---|---|
| `conversation.opener` | widget `POST /widget/v1/conversations` with `opener` (InTx in `StartWidgetConversation`) |
| `conversation.first_message` | `message.created` with `ConversationCreated`, contact author, substantive (InTx) |
| `conversation.customer_reopened` | `conversation.reopened` reason `customer_message` (InTx) |
| `message.customer` | `message.created` contact, substantive (queued) |
| `conversation.customer_quiet` | `fire_quiet_trigger` job armed by public member or agent replies |
| `conversation.rated` | `survey_response.completed` with conversation survey (queued) |
| `message.teammate` | `message.created` member, type message (queued) |
| `note.created` | `message.created` member, type note (queued) |
| `conversation.assignment_changed` | `conversation.assigned` (queued) |
| `conversation.status_changed` | `conversation.closed`, `conversation.snoozed`, `conversation.unsnoozed`, `conversation.reopened` (queued) |
| `conversation.teammate_quiet` | `fire_quiet_trigger` armed when `waiting_since` is set |
| `dimmi.handed_over` | `ai.handoff_requested` (queued) |
| `dimmi.resolved` | engine dimmi step outcome, or 07 confirmation on automatic dimmi (`ai.state_changed` with reason `resolved_confirmed`, new 07 transition `active -> off`) |
| `sla.warning`, `sla.breached` | `check_sla_target` job dispatching `conversation.sla_warning` / `conversation.sla_breached` (queued) |
| `reusable` | `run_workflow` step, `POST /conversations/{conv}/workflow-runs` |

Never triggers: spam conversations, trashed conversations, blocked or erased contacts, imported conversations and messages, `substantive = false` inbound (auto-replies, bounces, OOO), bot-authored chat app messages (05 drops them anyway), messages with `metadata.ai_skip` (04 spoofing guards) for customer-facing workflows (background still sees them).

## 3. Matching

`Matcher.Candidates(ctx, tx, trigger, conversation, event)`:

1. Load live versions for the workspace from cache `workflows:live:<ws>:<epoch>` (otter L1 per node, epoch bumped on publish / status / reorder / archive, busted via Valkey pubsub). Filter by `trigger`, then `trigger_settings` (status `to`, assignment `to`, SLA targets).
2. Order by `position` asc, ties by `created_at`.
3. For each candidate, first failing check wins, recorded as verdict in `workflow_match_logs`:

| # | Check | Skip reason |
|---|---|---|
| 1 | channel type in `channels` | `channel` |
| 2 | workspace hourly caps (`workflows.step_executions_hourly`) | `rate_limited` |
| 3 | schedule (`open` / `closed` against schedule at event time) | `schedule` |
| 4 | audience true (conditions over freshly loaded conversation, contact, company, trigger payload) | `audience` (+ `failed_rule` path and field) |
| 5 | loop guard: workflow id not in event `workflow_chain`, chain length < 3 | `loop_guard`, `chain_depth` |
| 6 | no active run of this workflow on the conversation | `already_running` |
| 7 | re-entry rule (once per conversation: any earlier run row of this workflow; cooldown: last start within window) | `reentry` |
| 8 | per-conversation budget (`workflows.max_steps_per_conversation_hour`) | `conversation_budget` |
| 9 | customer-facing only: conversation not claimed by another run | `held` |
| 10 | customer-facing only: an earlier candidate already started for this event | `lower_priority` |
| 11 | an earlier candidate with `exclusive` started for this event | `exclusive` |

Test-mode drafts (section 15) evaluated after live candidates with the same checks.

## 4. Claims (customer-facing ownership)

- A customer-facing run sets `conversations.workflow_run_id = run` and `conversations.ai_mode = 'workflow'` in the tx that creates the run. At most one claim per conversation (single column).
- Start triggers claim **inside the ingest transaction** (`MatchWorkflowsInTx`): the compiled cache is in memory, audience evaluates on rows already loaded by the action (contact, company, conversation); budget p95 2 ms, no network. This guarantees the 07 `TriggerDimmiAgent` pre-check (queued, runs after commit) sees `ai_mode = workflow` and stays silent. No race window.
- The claim clears when the run ends (any status). `ai_mode` stays `workflow` until the conversation closes (then back to `auto`, together with 07 `ai_state -> off`). Reason: after a workflow routed a conversation to a person, automatic dimmi must not jump back in on the next message.
- Background runs never claim.
- Customer-facing runs started by non-start triggers (customer quiet, teammate quiet, rated, dimmi resolved, reusable) claim only if the conversation is unclaimed, else skip `held` (manual and API reusable starts get 409 `conversation_held` unless `replace: true`).

## 5. Ordering and priority

- **Customer-facing**: candidates in list order; first that passes all checks starts; the rest get `lower_priority`. If the started run ends `passed` (section 6), matching re-runs for the same event from the next position (once; max 10 hops per event), so a workflow whose audience matched but whose branches had nowhere to go hands over to the next.
- **Background**: every passing candidate starts, in list order, as one batch per event. Batch member 1 is enqueued at once; member n+1 is enqueued when member n ends or reaches its first wait (`workflow_runs.wait.after_run` links them; the reconciler unblocks a batch whose predecessor is stuck over 2 min). Workflow 2 therefore sees workflow 1's changes, and for conflicting writes the lower workflow wins, always. A waiting background run (timer) does not hold up the rest of its batch.
- Customer-facing claim happens before background candidates for the same event start; background runs of the same event start after the claim commit (same tx for InTx, same job for queued).
- `exclusive` cuts the list for that event at that workflow (both kinds).
- One list, drag to reorder (`PATCH /workflows {order}`); reorder applies to the next event; running runs unaffected.

## 6. Run lifecycle

```
running --(blocking step)--> waiting --(resume)--> running
running --(path ends)--------> completed | passed
running|waiting --(conversation change)--> exited
running|waiting --(member cancel, pause with cancel, archive, replaced)--> canceled
running|waiting --(engine error, budget)--> failed
waiting --(run_max_lifetime)--> expired
```

- `passed`: customer-facing run ended having executed no customer-facing step, no action step and no dimmi step (only branch / classify / wait with nothing after). Recorded, claim released, fall-through (section 5). Background runs never `passed` (they are `completed`).
- End reasons (`enums.WorkflowRunEndReason`, registry): `completed`, `no_action`, `teammate_replied`, `conversation_closed`, `conversation_spam`, `conversation_deleted`, `conversation_merged`, `contact_blocked`, `interrupted`, `no_choice`, `canceled_by_member`, `canceled_by_api`, `workflow_paused`, `workflow_archived`, `replaced`, `step_budget`, `conversation_budget`, `definition_error`, `child_failed`, `child_exited`, `child_canceled`, `child_expired`, `expired`, `internal_error`.
- Timeline: `workflow.run_started` event row for customer-facing runs and for background runs that changed something visible (first effect writes it lazily); `workflow.run_ended` only for unusual endings (failed, expired, canceled). Steps' own effects already write 03 timeline rows (assigned, tagged) with actor shown as "Workflow: <name>".

## 7. Advancing a run

`advance_workflow_run{run_id, expected_seq}` (queue `workflows`, unique by run id while not running):

1. `BEGIN`; `SELECT pg_advisory_xact_lock(hashtextextended('wf:conv:' || conversation_id, 0))`: one advancing run per conversation at a time (all workflows), so effects never interleave.
2. Load run `FOR UPDATE`; `seq <> expected_seq` or status not `running`: commit, no-op (stale job).
3. Load version from immutable cache (otter LRU 2,000 by version id).
4. Loop: execute cursor step via its executor (`steps.Execute(ctx, tx, run, step) (Result, error)`):
   - `Continue`: next step on the same path. In a tree a path's end is the run's end, so an exhausted path means `End(completed)` (or `passed`, section 6).
   - `Path(id)`: first step of that path.
   - `Wait(spec)`: set `status waiting`, `wait`, `wake_at`; schedule resume job when timed (`InsertTx`, `ScheduledAt = wake_at`).
   - `End(reason)`.
   Each iteration: write trace row, set `cursor_step_id`, `steps_executed + 1`, `seq + 1`. Effects (03 actions) run in this tx with actor `{system, via: {workflow, run}}` and the event field `WorkflowChain = run.chain + workflow_id`.
5. Stop when waiting, ended, `steps_executed` hits `workflows.max_steps_per_run` (fail `step_budget`), or 25 steps in one job (commit and re-enqueue self with new seq: keeps transactions short).
6. `COMMIT`. Events dispatched in tx per phase rules (realtime after commit).

Steps whose effect leaves the database (HTTP request, classify LLM call, dimmi activation) split into two phases so no network call sits inside the lock: phase 1 (in tx) records `waiting` with `wait.kind = http_retry | classify | dimmi` and enqueues the worker job; the worker calls out without the lock, then enqueues `advance_workflow_run` with the result stored in `wait.result` via a short tx (`seq` checked). Customer message sends are DB-only (03 outbound job), so they stay in phase 1.

## 8. Waits, timers, inputs

| Wait kind | Set by | Resumed by | Timeout |
|---|---|---|---|
| `timer` | wait step | `resume_workflow_run` at `wake_at` | n/a |
| `input` | buttons, collect | `ResumeWorkflowOnInput` on next contact message (debounce `workflows.input_debounce` for collect: resume job `ScheduledAt = now + 2 s`, unique by run, so "my order" + "is 1042" become one answer) or widget button answer | step `timeout` -> timeout path; else run lifetime |
| `dimmi` | dimmi step | `ResumeWorkflowOnDimmiOutcome` | `quiet_after` timer re-armed after each dimmi answer |
| `child` | run_workflow | child end | child's own |
| `http_retry`, `classify` | phase-1 of external steps | worker result | step timeouts |

- Interrupts for `timer` (wait step `interrupt_on`): `ResumeWorkflowOnInput` (customer message) and the teammate-reply listener resume with `interrupted`; trace outcome `interrupted`.
- Reconciler (`reconcile_workflow_timers`, every minute): runs `waiting` with `wake_at < now - 2 min` get a resume job (unique args make duplicates harmless).
- Business-time waits: `wake_at` computed with `officehours.AddBusiness`; schedule edits after the wait started do not move it (computed once; documented).

## 9. Idempotency

| Effect | Guarantee | How |
|---|---|---|
| Run start | once per (workflow, triggering event) | unique `(workflow_id, trigger_event_id)`; listener retries hit conflict and no-op |
| Step execution | once per seq | `expected_seq` check under row lock |
| DB effects (assign, tag, note, close, snooze, SLA, contact update) | exactly once | same tx as cursor advance |
| Customer message | exactly once | `messages.dedupe_key = 'wf:<run>:<step>'` (03 unique `(workspace_id, dedupe_key)`); outbound job keyed by message |
| HTTP request | at least once | header `webhook-id` / `Idempotency-Key` = `wfr_<run>:<step_id>` stable across retries; receivers dedupe |
| Classify / dimmi LLM calls | at most one billed call per step attempt | result stored in `wait.result` before advance; retry of advance reuses it |
| Timers | at least once, idempotent | resume jobs carry seq; stale no-op |

## 10. Re-entry

- Per workflow `reentry` (catalog.md section 1) checked at match time; once-per-conversation counts runs of any status except `passed` (a pass did nothing, so trying again later is fine).
- One active run per workflow per conversation (partial unique index). A new matching event while active: `already_running`.
- Start triggers are once per conversation (or per reopen) by definition.
- Quiet triggers re-arm per anchor message: one fire per teammate reply (customer quiet) or per waiting period (teammate quiet).

## 11. Loop guard

1. Every event dispatched inside a run carries `WorkflowChain` (ids of workflows in the causal chain, the run's own chain plus its workflow).
2. A workflow never starts from an event whose chain contains it (`loop_guard`).
3. Chain length 3 max (`chain_depth`): A changes -> B runs -> B changes -> C runs -> C's changes trigger nothing.
4. Per run step budget (200), per conversation hourly budget (300 steps), per conversation hourly bot message cap (20; further message steps skipped `message_budget`, run continues).
5. Workflow messages are `author_type system`: never fire `message.customer` or `message.teammate`; email workflow messages carry `Auto-Submitted`, and inbound auto-replies are `substantive false` (never trigger).
6. dimmi actions (actor agent) never fire team triggers.

## 12. Mid-run changes

Handled InTx by `InterruptWorkflowRuns` (runs in the action's transaction, before outbound listeners).

| Change | Customer-facing run | Background run |
|---|---|---|
| Member public reply | exit `teammate_replied`; unsent workflow messages of this run (outbound job not yet sent) canceled; claim released; `ai_mode` stays `workflow` | continues (unless its wait interrupts on teammate reply) |
| Member note, tag, priority, assignment change | continue | continue |
| Conversation closed (by anyone except this run) | exit `conversation_closed` | exit `conversation_closed` unless the run was started by `conversation.status_changed` to closed or by `conversation.rated` / `dimmi.resolved` (they run on closed conversations) |
| Reopened | n/a (ended at close) | continue |
| Snoozed | continue; waits keep running; a message step while snoozed sends (snooze is a team view, not a customer state) | continue |
| Marked spam, trashed | exit `conversation_spam` / `conversation_deleted` | same |
| Merged into another | exit `conversation_merged` on source; target unaffected | same |
| Contact blocked or erased | exit `contact_blocked` | same |
| Contact merged (03) | run follows conversation (conversation id unchanged); `contact_id` updated | same |
| Assignee deactivated | continue; assign steps use fallback | same |
| Workflow paused | continue on its version (default); with `cancel_runs: true` canceled `workflow_paused` | same |
| New version published | continue on old version | same |
| Workflow archived | canceled `workflow_archived` | same |
| Member PATCH `ai_state: handed_off` during dimmi step | dimmi step outcome `handed_over` reason `member_paused` | n/a |

## 13. dimmi step

1. Phase 1 (tx): `ai.ActivateConversationAi(ctx, tx, conv, ActivateInput{Reason: workflow, RunID, StepID, Instructions, MaxReplies, ConfirmResolution, HandoverNotice})` (07): `ai_state off|handed_off -> active`, dispatches `ai.state_changed`. 07 reads the step settings from the claiming run's cursor step on each pipeline run (no copy) and records them in `ai_runs` context. Run waits `dimmi`. If the conversation has an unanswered contact message (first-message trigger, typed path), enqueue 07 `RunDimmiAgent` for it directly (no new message needed).
2. 07 gate rule 2c: when `ai_mode = workflow`, the agent runs only if the claiming run's cursor is a dimmi step (cheap join on `workflow_run_id`). Gate rules 3 to 12 apply as today (schedule, cap, provider, limits) and produce handovers that the engine maps to `handed_over`.
3. Outcomes:
   - `resolved`: customer pressed "That helped" or replied with a confirm phrase / classifier `acknowledgement` intent directly after an `answered` run. 07 sets `ai_state -> off` with reason `resolved_confirmed` and dispatches `ai.state_changed`; engine takes `resolved` path; when empty and `close_on_resolved`, closes the conversation (actor workflow) and dispatches trigger `dimmi.resolved`.
   - `handed_over`: `ai.handoff_requested` for this conversation while the step is active (any reason). Inside a dimmi step 07 skips its own routing (`handoff.team_id`, `set_priority` except `safety`, which always sets priority) because the path routes. Notice: sent by 07 unless `handover_notice = workflow`. Path variables: `dimmi.handover_reason`, `dimmi.summary` (07 summary note text; the engine waits up to 6 s for `SummarizeHandoff` before continuing so the path can use the summary; then continues with the deterministic summary).
   - `quiet`: no contact message for `quiet_after` since the last dimmi answer. Engine sets `ai_state -> off` (reason `workflow_quiet`, no notice) and takes the path.
4. A member public reply ends the run (`teammate_replied`) and 07 already moves `ai_state -> handed_off` (`human_replied`): customer never gets both.
5. dimmi step inside a reusable workflow behaves the same; the claim belongs to the top-level run.

## 14. Concurrency

- Queue `workflows`, per-node concurrency `workflows.queue_concurrency`. Per-workspace fairness: `workflows.workspace_concurrency` running advance jobs (Valkey semaphore `wf:sem:<ws>`, TTL 60 s); over: job snoozes 200 ms (River snooze, not counted as attempt). Valkey down: semaphore fails open (metric), advisory lock still serializes per conversation.
- Hot conversation (many events at once): advisory lock queues advances; each holds the lock under 50 ms p95 (DB-only work).
- Matching listener batches per event; a workspace with 100 live workflows on one trigger evaluates in under 5 ms (compiled conditions).

## 15. Versions, publish, pause, rollback, drafts on test conversations

- Publish (`PublishWorkflow`): validate draft (catalog.md section 6), compute kind, compile, insert version `number = max + 1`, set `live_version_id`, `status live`, bump cache epoch, dispatch `workflow.published` and audit. 409 `nothing_to_publish` when draft hash equals live hash. Entitlement `workflows.max_live` checked when the workflow was not live.
- Pause: `PATCH {status: paused, cancel_runs?}`. Resume: `PATCH {status: live}` keeps the same version.
- Rollback: `PATCH {restore_version_id, publish?}` copies version spec into draft (`lock_version` bump); `publish: true` publishes it as a new version (`restored_from_version_id`). References that no longer exist surface as publish errors, never silently empty (Intercom gap).
- Drafts on test conversations: `PATCH /workflows/{wf} {test_on_test_conversations: true}` (stored on workflow, default false). The draft (validated, compiled on the fly, not stored as version) is a candidate only for conversations with `is_test = true` (widget preview page and test contacts create them, 01/03). Runs `mode test`, trace and effects real (on test data). Test runs never count in stats.
- Version retention: all versions kept while any run references them; otherwise the newest 100 per workflow kept.

## 16. Simulation and explainer

- `SimulateWorkflow` (`POST /workflows/{wf}/simulations`): runs `draft` or a version against `conversation_id` (real, read-only snapshot) or a synthetic context (`contact`, `company`, `conversation`, `message`, `trigger` fields). Inputs scripted: `inputs: [{button: "p_billing"} | {text: "it's about invoice 1042"} | {dimmi: "handed_over", reason: "unknown"} | {wait: "elapsed"} | {http: {status: 200, body: {...}}} | {classify: "p_bug"}]`. Missing input at a blocking step: simulation stops with `awaiting` and lists what it needs.
- No side effects: executors run in `dry` mode (no 03 action, no outbound, no HTTP, no LLM unless `live_ai: true` which requires `workflows:manage` and spends 07 budget; default fakes the AI step via inputs). Output: audience verdict with per-leaf results, ordered step trace with rendered messages (per channel rendering: widget buttons vs numbered text), effects as sentences ("Assign to Billing team (round robin would pick Lea Martin now)"), variables, end state. Synchronous, max `workflows.simulation.max_steps`, p95 under 150 ms without live AI.
- Explainer: `GET /conversations/{conv}/workflow-matches` returns match logs (section 3 verdicts) plus runs; `GET /conversations/{conv}/workflow-matches?workflow_id=` evaluates now ("would it match right now?") with per-leaf audience results.

## 17. Office hours

- Schedules: `app/support/officehours` (pure Go). Used by workflow `schedule`, `office_hours.open` condition, wait `until office_hours_open`, snooze `until office_hours_open`, business-time counting, reply time step, SLA clocks, 07 schedule and handover notice (`next_open_local`), widget availability (default schedule).
- Workspace default schedule created at workspace init; deleting the default refused (`schedule_default`); pick another default first.
- Holidays closed whole day in the schedule timezone; ranges inclusive.

## 18. SLAs

- `ApplySlaPolicy(conv, policy, run)`: cancels active targets (state `canceled`), inserts targets for policy's non-null minutes:
  - `first_response`: starts at conversation `first_customer_message_at` (or now if later applied: clock started at first customer message, so a late apply can be already breached: then `breached` immediately and `sla.breached` fires); met by first public member reply (and dimmi substantive answer when `count_ai_replies`).
  - `next_response`: active when the customer is waiting (`waiting_since` not null) after a first response; due from `waiting_since`; met by next public reply; re-created on each new waiting period.
  - `resolution`: from apply time (or first customer message); met by close; paused while snoozed when `pause_when_snoozed`; reopen resumes with remaining time.
- Due times: `officehours.AddBusiness(start, minutes)` when `schedule_id`, else calendar. Paused targets store `remaining_seconds`.
- `TrackSlaTargets` (InTx on replies, close, snooze, reopen) updates targets and `conversations.sla_due_at` in the same tx; `check_sla_target` jobs at warn and due times dispatch events and workflow triggers; reconciler catches lost jobs.
- One policy per conversation. Re-applying the same policy is a no-op (`skip already_applied`). Removing: `PATCH /conversations/{conv} {sla_policy_id: null}` (`sla:manage`) cancels targets.

## 19. HTTP request

- Phase 1 records `wait.kind http_retry` and enqueues `send_workflow_http_request`.
- Request: connection (07 `ai_action_connections`: base URL, auth, SSRF policy) + `path`, or raw `url` signed with Standard Webhooks headers using the workspace workflow signing secret (`GET /workflow-signing-secret` reveals, rotation like 10). Always headers `Dimmi-Workflow-Run`, `Idempotency-Key`. Default body:
  ```json
  {"type": "workflow.http_request", "run": {"id": "wfr_...", "step_id": "s_x"}, "workflow": {"id": "wf_...", "version": 4},
   "conversation": {"id": "conv_...", "number": 1042, "channel_type": "widget", "tags": [], "assignee_id": null},
   "contact": {"id": "ctc_...", "external_id": "u_42", "email": "a@b.co", "name": "Ada"}, "vars": {}}
  ```
- Attempts: 3 within 2 min (0 s, 10 s, 60 s) on network errors, 429 (honours `Retry-After` up to 60 s), 5xx. 4xx other than 408/429: no retry. Then `failure` path.
- Response: up to 64 KB JSON parsed; `map` pointers copied into run vars (type preserved, strings max 1 KB). Non-JSON with mappings: `failure` with outcome `invalid_response`.
- Customer-facing runs keep the customer waiting at most 2 min for this step; builder warns when an HTTP step precedes a customer message.
- Hourly cap `workflows.http_requests_hourly` per workspace: over cap = `failure` outcome `rate_limited` without calling.

## 20. Analytics

Per workflow (and per version, compare two versions side by side):
- Runs: started, completed, passed, exited (by reason), canceled, failed, expired; median and p90 duration.
- Customer-facing: engaged (customer answered at least one input or wrote while claimed), dimmi resolved / handed over / quiet rates, conversations closed within the run.
- Per step: entered, completed, failed; per path counts and share; drop-off (entered minus continued) at input steps; HTTP success rate and p95 latency; classify label distribution and `none` rate.
- Canvas overlay: counts on each card and each path line for a chosen period and version.
- Source: `workflow_stats_daily` (rollup) plus raw trace for the current hour. Test runs and simulations excluded.

## 21. Observability

Metrics: `dimmi_workflow_runs_started_total{trigger,kind}`, `dimmi_workflow_runs_ended_total{status,reason}`, `dimmi_workflow_step_duration_seconds{kind}`, `dimmi_workflow_advance_lock_wait_seconds`, `dimmi_workflow_timer_lag_seconds`, `dimmi_workflow_match_duration_seconds{phase}`, `dimmi_workflow_http_requests_total{status_class}`, `dimmi_workflow_loop_guard_total`. No workspace or workflow labels (cardinality). Logs: one line per run end (ids, status, reason, steps, duration), never message text or variables.
