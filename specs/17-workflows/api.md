# 17 Workflows: API

Style: caveman. Base `/api/v1`. REST rules `00-foundation/rest.md` (binding): nouns, state as field, child collections for publications, runs, simulations. Every route `A` (`auth`), `workspace`, `throttle:api`; POST routes `idempotent`. Permissions per README "Permissions"; token ability groups `workflows:read`, `workflows:write`, `settings:write`.

## Endpoint table

### Workflows

| Method | Path | Route name | Permission | Purpose |
|---|---|---|---|---|
| GET | /workflows | workflows.index | workflows:read | list in priority order; `filter[trigger]`, `filter[status]`, `filter[kind]`, `filter[trashed]`; `include=stats` (last 30 days counters) |
| POST | /workflows | workflows.store | workflows:manage | create draft `{name, trigger}` or `{template, name?}` or `{duplicate_of: wf_, trigger?}` or full `{name, trigger, spec}` (import) |
| PATCH | /workflows | workflows.update-many | workflows:manage | reorder `{order: [wf_...]}` (rule 9); must list every non-archived workflow (409 `order_stale` otherwise) |
| GET | /workflows/{wf} | workflows.show | workflows:read | `include=live_version,draft,warnings,stats` |
| PATCH | /workflows/{wf} | workflows.update | workflows:manage | `name`, `description`, `spec` (draft, needs `lock_version`), `status` (`live` resume, `paused` + `cancel_runs`), `restore_version_id` + `publish`, `test_on_test_conversations`, `trashed: false` (unarchive as draft) |
| DELETE | /workflows/{wf} | workflows.destroy | workflows:manage | archive; cancels active runs (`workflow_archived`); 409 `workflow_in_use` when a live workflow calls it |
| POST | /workflows/{wf}/publications | workflows.publications.store | workflows:manage | publish draft `{note?, acknowledge_warnings?: [codes], expected_draft_hash}`; 201 version |
| GET | /workflows/{wf}/versions | workflows.versions.index | workflows:read | newest first |
| GET | /workflows/{wf}/versions/{wfv} | workflows.versions.show | workflows:read | full spec; `include=diff` (against previous version or `?against=wfv_`) |
| GET | /workflows/{wf}/runs | workflows.runs.index | workflows:read | `filter[status]`, `filter[version]`, `filter[end_reason]`, `filter[conversation]`, `filter[started_after]`, `filter[mode]`; cursor pagination |
| GET | /workflows/{wf}/stats | workflows.stats.show | workflows:read | `?from&to&version` (default last 30 days, live version); singleton computed resource |
| POST | /workflows/{wf}/simulations | workflows.simulations.store | workflows:read (`live_ai: true` needs workflows:manage) | dry run (engine.md section 16); 201 `workflow_simulation`, not stored |
| GET | /workflow-templates | workflow-templates.index | workflows:read | shipped templates |
| GET | /workflow-triggers | workflow-triggers.index | workflows:read | registry: triggers, groups, allowed steps, condition fields, payload fields (drives the builder and CLI validation) |

### Runs

| Method | Path | Route name | Permission | Purpose |
|---|---|---|---|---|
| GET | /workflow-runs/{wfr} | workflow-runs.show | workflows:read | `include=steps,children,conversation` |
| PATCH | /workflow-runs/{wfr} | workflow-runs.update | workflows:run (conversation visible) | `{status: "canceled"}` only; 422 `invalid_transition` when ended |
| GET | /conversations/{conv}/workflow-runs | conversations.workflow-runs.index | workflows:read + conversations:read | runs on this conversation |
| POST | /conversations/{conv}/workflow-runs | conversations.workflow-runs.store | workflows:run | start a live reusable workflow `{workflow_id, vars?, replace?}`; 201 run with its current status |
| GET | /conversations/{conv}/workflow-matches | conversations.workflow-matches.index | workflows:read + conversations:read | match log; `?workflow_id=` adds a live "would it match now" evaluation |

### Schedules and SLA policies

| Method | Path | Route name | Permission | Purpose |
|---|---|---|---|---|
| GET | /schedules | schedules.index | conversations:read | |
| POST | /schedules | schedules.store | schedules:manage | |
| GET | /schedules/{sch} | schedules.show | conversations:read | `include=status` (`open`, `next_open`, `next_close` now) |
| PATCH | /schedules/{sch} | schedules.update | schedules:manage | `is_default: true` moves default |
| DELETE | /schedules/{sch} | schedules.destroy | schedules:manage | 409 `schedule_in_use`, `schedule_default` |
| GET | /sla-policies | sla-policies.index | conversations:read | |
| POST | /sla-policies | sla-policies.store | sla:manage | |
| GET, PATCH, DELETE | /sla-policies/{sla} | sla-policies.show / update / destroy | read: conversations:read; write: sla:manage | edits apply to future applications only |
| PATCH | /conversations/{conv} | (03) | sla:manage for `sla_policy_id` | 03 PATCH gains `sla_policy_id` (apply or `null` to remove) |

### Signing secret

| Method | Path | Route name | Permission | Purpose |
|---|---|---|---|---|
| GET | /workflow-signing-secret | workflow-signing-secret.show | workflows:manage | reveal (audited) |
| POST | /workflow-signing-secret/rotations | workflow-signing-secret.rotations.store | workflows:manage | rotate with 24 h overlap (same mechanics as 10) |

### Widget (`/widget/v1`, 01 adds)

| Method | Path | Purpose |
|---|---|---|
| GET | /widget/v1/apps/{app}/config | adds `workflow_openers: [{version_id, audience_hash, steps: [{kind: message|reply_time, text}], buttons: {text, options: [{id, label}], allow_typing}}]`; widget picks the first opener whose compiled audience matches the session (server sends only openers whose audience can be decided from session fields; the rest are evaluated server-side at create) |
| POST | /widget/v1/conversations | adds `opener: {version_id, answer: {button_id} or {typed: true}}`; body = button label or typed text. Server re-checks audience and claim; stale opener (version no longer live): conversation created, first-message matching applies instead |
| POST | /widget/v1/conversations/{conv}/messages | adds `workflow_answer: {run_id, step_id, button_id}` for button clicks (body = label); ignored when the run no longer waits on that step (message stored as normal text) |

## Resources

### workflow

```json
{
  "id": "wf_01jbx...",
  "object": "workflow",
  "name": "Fast lane for VIP customers",
  "description": null,
  "trigger": "conversation.first_message",
  "status": "live",
  "kind": "customer_facing",
  "position": 2,
  "live_version": { "id": "wfv_01jby...", "number": 4, "published_at": "2026-11-03T10:12:00Z", "published_by": { "id": "mem_...", "name": "Sam Okafor" } },
  "has_unpublished_changes": true,
  "lock_version": 17,
  "test_on_test_conversations": false,
  "warnings": [ { "code": "reference_missing", "step_id": "s_a1", "field": "member_id", "message": "Assignee Lea Martin was deactivated; the step falls back to the Support team." } ],
  "editors": [ { "id": "mem_...", "name": "Lea Martin" } ],
  "stats": { "period": "30d", "started": 1204, "completed": 1150, "exited": 41, "failed": 2 },
  "created_at": "...", "updated_at": "...", "paused_at": null, "deleted_at": null
}
```

`include=draft` adds `draft: <spec>` (catalog.md section 1) and `draft_errors` / `draft_warnings` (validator run on read, cheap). `editors` from realtime presence on `workflow:{wf}` channel.

### Update draft

```http
PATCH /api/v1/workflows/wf_01jbx...
{ "lock_version": 17, "spec": { ...full spec... } }
```
200 with new `lock_version`. Mismatch: 409 `edit_conflict` `{current_lock_version, updated_by, updated_at}`; builder offers "Load theirs / Overwrite" (overwrite resends with new lock version). Spec saved even when invalid (drafts may be incomplete); `trigger` inside `spec` must equal workflow trigger (422 `trigger_immutable`). Builder autosaves debounced 800 ms.

### Publish

```http
POST /api/v1/workflows/wf_01jbx.../publications
{ "note": "Route refunds to billing", "expected_draft_hash": "sha256:9f...", "acknowledge_warnings": ["path_ends_unowned"] }
```
201:
```json
{ "data": { "id": "wfv_01jbz...", "object": "workflow_version", "number": 5, "kind": "customer_facing",
  "note": "Route refunds to billing", "published_by": { "id": "mem_..." }, "published_via": "session", "created_at": "..." } }
```
Errors: 422 `workflow_invalid` `{errors: [...], warnings: [...]}` (catalog.md section 6); 409 `nothing_to_publish`; 409 `draft_changed` (hash mismatch: someone saved since you looked); 422 `entitlement_limit_reached` (`workflows.max_live`); 403 `human_review_required` when called through MCP without client confirmation (09 gate).

### workflow_run

```json
{
  "id": "wfr_01jc0...",
  "object": "workflow_run",
  "workflow": { "id": "wf_...", "name": "dimmi first, then the team" },
  "version": { "id": "wfv_...", "number": 3 },
  "conversation_id": "conv_...",
  "kind": "customer_facing",
  "mode": "live",
  "status": "waiting",
  "waiting_on": { "kind": "dimmi", "step_id": "s_d1", "since": "...", "wake_at": "2026-11-04T10:40:00Z" },
  "end_reason": null,
  "trigger": { "key": "conversation.first_message", "event_id": "evt_...", "data": { "message_id": "msg_..." } },
  "vars": { "plan": "pro" },
  "started_at": "...", "ended_at": null,
  "steps": [
    { "step_id": "s_8f2k1q", "kind": "buttons", "status": "completed", "path_id": "p_other", "outcome": "button", "detail": { "message_id": "msg_..." }, "started_at": "...", "finished_at": "..." },
    { "step_id": "s_d1", "kind": "dimmi", "status": "waiting", "detail": { "ai_run_ids": ["air_..."] } }
  ]
}
```

`steps` only with `include=steps` (members of `workflows:read`). `vars` values over 200 chars truncated in list views.

### workflow_simulation

Request:
```json
{ "source": "draft", "conversation_id": "conv_...",
  "inputs": [ { "button": "p_other" }, { "dimmi": "handed_over", "reason": "unknown" } ] }
```
Or synthetic: `{ "source": "version:wfv_...", "context": { "channel_type": "email", "contact": { "kind": "user", "custom": { "plan": "enterprise" } }, "message": { "body": "refund please" } } }`.
Response 201:
```json
{ "data": { "object": "workflow_simulation", "matched": true,
  "audience": [ { "field": "company.custom.plan", "op": "in", "value": ["enterprise"], "actual": "enterprise", "result": true } ],
  "state": "completed",
  "steps": [
    { "step_id": "s_8f2k1q", "kind": "buttons", "rendered": { "widget": { "text": "What can we help with?", "buttons": ["Billing", "Something else"] }, "email": "What can we help with?\n\n1. Billing\n2. Something else" }, "path_id": "p_other" },
    { "step_id": "s_d1", "kind": "dimmi", "outcome": "handed_over", "path_id": "p_ho" },
    { "step_id": "s_a2", "kind": "assign", "effect": "Assign to Support team (round robin would pick Lea Martin now)" }
  ],
  "vars": {}, "awaiting": null } }
```
`state`: `completed`, `passed`, `awaiting` (with `awaiting: {step_id, needs: "button" | "text" | "dimmi" | "http" | "classify" | "wait"}`), `not_matched`, `invalid` (draft errors listed).

### workflow_match (list item of `/conversations/{conv}/workflow-matches`)

```json
{ "object": "workflow_match", "trigger": "conversation.first_message", "event_id": "evt_...", "created_at": "...",
  "verdicts": [
    { "workflow": { "id": "wf_a", "name": "Fast lane for VIP" }, "version": 4, "verdict": "skipped", "reason": "audience",
      "failed_rule": { "field": "company.custom.plan", "op": "in", "value": ["enterprise"], "actual": "pro" } },
    { "workflow": { "id": "wf_b", "name": "Route by topic" }, "version": 7, "verdict": "started", "run_id": "wfr_..." },
    { "workflow": { "id": "wf_c", "name": "dimmi first" }, "version": 2, "verdict": "skipped", "reason": "lower_priority" }
  ] }
```

### workflow_stats

```json
{ "object": "workflow_stats", "from": "2026-10-05", "to": "2026-11-04", "version": 4,
  "runs": { "started": 1204, "completed": 1102, "passed": 12, "exited": { "teammate_replied": 61, "conversation_closed": 9 }, "failed": 2, "p50_seconds": 340 },
  "customer_facing": { "engaged": 0.91, "dimmi": { "resolved": 0.42, "handed_over": 0.47, "quiet": 0.11 } },
  "steps": { "s_8f2k1q": { "entered": 1204, "completed": 1150, "paths": { "p_billing": 402, "p_other": 690, "p_typed": 58 } } } }
```

### schedule, sla_policy

```json
{ "id": "sch_01...", "object": "schedule", "name": "Office hours", "timezone": "Europe/Berlin", "is_default": true,
  "weekly": { "mon": [["09:00","17:30"]], "tue": [["09:00","17:30"]] }, "holidays": [ { "date": "2026-12-25", "name": "Christmas" } ],
  "status": { "open": false, "next_open": "2026-11-05T08:00:00Z", "next_close": null } }
```
```json
{ "id": "sla_01...", "object": "sla_policy", "name": "VIP", "first_response_minutes": 30, "next_response_minutes": 120,
  "resolution_minutes": 1440, "schedule_id": "sch_01...", "warn_before_minutes": 15, "count_ai_replies": false, "pause_when_snoozed": true }
```

03 conversation resource gains (only when set): `workflow: {run_id, workflow_id, name, waiting_on}` and `sla: {policy_id, targets: [{target, state, due_at, breached_at}]}`; list sort `sla_due`, filter `filter[sla_state]=breaching|breached|active`.

## Error codes

| Status | Code | When |
|---|---|---|
| 422 | `workflow_invalid` | publish or test enable with errors |
| 422 | `trigger_immutable` | trigger change attempted |
| 409 | `edit_conflict` | stale `lock_version` |
| 409 | `nothing_to_publish`, `draft_changed` | publish |
| 409 | `workflow_in_use` | archive of a called reusable workflow |
| 409 | `conversation_held` | manual run of customer-facing reusable while claimed (`replace: true` to take over) |
| 422 | `not_reusable` | manual run of non-reusable or non-live workflow |
| 422 | `conversation_ineligible` | manual run on spam, trashed, imported or closed (customer-facing) conversation |
| 409 | `order_stale` | reorder list incomplete |
| 409 | `schedule_in_use`, `schedule_default` | schedule delete (SLA policies can always be deleted; applied targets keep their snapshot) |
| 422 | `entitlement_limit_reached` | catalog limits |
| 422 | `invalid_transition` | run cancel when ended; status changes not allowed (draft -> paused) |

## MCP toolset `workflows` (09)

| Tool | Risk | Maps to |
|---|---|---|
| `dimmi_list_workflows` | read | GET /workflows |
| `dimmi_get_workflow` | read | GET /workflows/{wf}?include=draft,warnings,stats |
| `dimmi_get_workflow_triggers` | read | GET /workflow-triggers |
| `dimmi_save_workflow_draft` | write | POST /workflows or PATCH spec (draft only; never live) |
| `dimmi_simulate_workflow` | read | POST /workflows/{wf}/simulations (`live_ai` refused over MCP) |
| `dimmi_publish_workflow` | destructive (always confirms) | POST publications |
| `dimmi_set_workflow_status` | destructive (always confirms) | PATCH status |
| `dimmi_list_workflow_runs` | read | GET runs |
| `dimmi_get_workflow_run` | read | GET /workflow-runs/{wfr}?include=steps |
| `dimmi_explain_conversation_workflows` | read | GET /conversations/{conv}/workflow-matches |
| `dimmi_run_workflow` | send (always confirms; customer may see messages) | POST /conversations/{conv}/workflow-runs |

Agents can draft and simulate freely; anything that changes what customers see needs a human confirmation, same rule as 07 action publishing.

## CLI (09, `dimmi workflows`)

```
dimmi workflows list [--trigger] [--status] [--json]
dimmi workflows show <wf> [--draft] [--json]
dimmi workflows export [<wf>...] --dir ./dimmi/workflows        # one <slug>.json per workflow: {name, trigger, spec}
dimmi workflows apply --dir ./dimmi/workflows [--publish] [--dry-run] [--note "..."]
dimmi workflows simulate <wf|file.json> --conversation 1042 [--input button=p_other ...]
dimmi workflows publish <wf> [--note] [--yes]
dimmi workflows pause|resume <wf> [--cancel-runs]
dimmi workflows runs <wf> [--status failed] [--watch]
dimmi workflows explain <conversation>
```

`apply` matches files to workflows by `id` in the file, else by name; creates missing ones as drafts; updates drafts with `lock_version` from the server (prints a diff first, `--dry-run` stops there); `--publish` publishes each changed workflow and fails the command (exit 1) on validation errors, so CI can gate merges. Exported JSON is canonical (sorted keys, stable ids) so diffs in pull requests show only real changes. Never deletes workflows missing from the directory (prints them).

## Webhooks (10 catalog rows)

| Event type | Object | Notes |
|---|---|---|
| `workflow.published` | `workflow` + `version` | |
| `workflow.status_changed` | `workflow` | `from`, `to`, `canceled_runs` |
| `workflow.failing` | `workflow` | failure rate, window |
| `workflow_run.completed` | `workflow_run` (no steps, no vars unless endpoint opts in with `include_notes`, as vars may hold collected answers) | |
| `workflow_run.ended` | `workflow_run` | exited, canceled, expired, failed with `end_reason`, `error` |
| `conversation.sla_applied`, `conversation.sla_warning`, `conversation.sla_breached` | `conversation` (with `sla` block) | |

## Realtime

| Channel | Events |
|---|---|
| `workspace:` | `workflow.*`, `schedule.*`, `sla_policy.*`, `conversation.sla_*` (list badges) |
| `conversation:` | `workflow_run.started`, `workflow_run.updated`, `workflow_run.completed`, `workflow_run.ended`, `conversation.sla_*` |
| `workflow:wf_...` (new namespace, members with `workflows:read`) | presence only (who is editing), `workflow.updated` for that workflow (builder reload prompt) |
| `widget_contact:` | nothing new; workflow messages arrive as messages with `workflow` block (`buttons`, `input`) |
