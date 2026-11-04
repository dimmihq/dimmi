# 17 Workflows: catalog (triggers, conditions, steps, templates)

Style: caveman. Owner 17. Every trigger and step kind is a registry entry (`enums.WorkflowTrigger`, `enums.WorkflowStepKind`, extensibility.md P1): new kind = Go registration plus validator plus executor, no migration.

## 1. Definition shape

`models.WorkflowSpec` (draft and version `spec`):

```json
{
  "schema": 1,
  "trigger": "conversation.first_message",
  "trigger_settings": {},
  "channels": ["widget", "email"],
  "audience": { "all": [
    { "field": "contact.kind", "op": "eq", "value": "user" },
    { "field": "company.custom.plan", "op": "in", "value": ["enterprise"] }
  ] },
  "schedule": { "mode": "any" },
  "reentry": { "mode": "once_per_conversation" },
  "exclusive": false,
  "steps": [
    { "id": "s_8f2k1q", "kind": "buttons",
      "settings": { "text": "What can we help with?", "allow_typing": true },
      "paths": [
        { "id": "p_billing", "label": "Billing", "steps": [
          { "id": "s_a1", "kind": "assign", "settings": { "mode": "team_round_robin", "team_id": "team_01jb..." } }
        ] },
        { "id": "p_other", "label": "Something else", "steps": [
          { "id": "s_d1", "kind": "dimmi", "settings": { "quiet_after": "10m", "confirm_resolution": true },
            "paths": [
              { "id": "p_res", "outcome": "resolved", "steps": [ { "id": "s_c1", "kind": "close", "settings": {} } ] },
              { "id": "p_ho", "outcome": "handed_over", "steps": [] },
              { "id": "p_q", "outcome": "quiet", "steps": [] }
            ] }
        ] },
        { "id": "p_typed", "outcome": "typed", "steps": [] }
      ] }
  ]
}
```

- Step and path ids: `s_` / `p_` + 6 to 16 chars `[a-z0-9_]`, unique within the definition, assigned by the client (builder) or server (missing id on save gets one). Stable across versions: analytics compare the same step across versions by id. Copy and paste regenerates ids.
- `channels`: subset of `enums.ChannelType` registry; empty = all channels the trigger supports. `api` included.
- `schedule`: `{mode: any}` | `{mode: open, schedule_id}` | `{mode: closed, schedule_id}`; `schedule_id` null = default schedule.
- `reentry`: `{mode: every_time}` | `{mode: once_per_conversation}` | `{mode: cooldown, cooldown: "24h"}` (1 minute to 90 days). Allowed modes per trigger below.
- `exclusive`: when this workflow starts a run for an event, later workflows in the list skip that event (reason `exclusive`).
- Durations: Go-style strings `"10m"`, `"2h"`, `"3d"` (days allowed, `d` = 24 h calendar).
- Variables in text settings: `{{contact.first_name | fallback: "there"}}`, `{{conversation.number}}`, `{{run.vars.order_id}}`, `{{schedule.next_open}}` (localized). Same renderer and escaping as 03 saved replies. Unknown variable: publish error `unknown_variable`.

## 2. Triggers

Groups and tints match the prototype (`triggers.ts`), plus "service levels". CF = customer-facing steps allowed. Every trigger carries the common context (section 3); listed payload is `trigger.*` and extra roots.

### Conversation start (`start`, blue)

| Key | Name | Fires | CF | Channels | Re-entry | Payload |
|---|---|---|---|---|---|---|
| `conversation.opener` | Customer starts a new conversation | widget shows a new-conversation screen; workflow content shown before the customer types; run created when the customer clicks a button or sends text | yes (required) | widget | fixed once per conversation | `trigger.opener_page_url`, `trigger.answer` (`button` + id, or `typed`) |
| `conversation.first_message` | Customer sends their first message | first contact message of a conversation (`message.created`, `ConversationCreated` true, contact author, substantive) | yes | all | fixed once per conversation | `message.*` |
| `conversation.customer_reopened` | Customer writes on a closed conversation | contact message reopens a closed conversation (`conversation.reopened` reason `customer_message`) | yes | all | fixed once per reopen | `message.*`, `trigger.closed_for_minutes` |

Opener rules: definition must start with zero or more `message` / `reply_time` steps followed by exactly one `buttons` step (validation `opener_shape`); everything before the first input is compiled into widget config (`opener` block) and rendered client-side with no request. Conversation is created only when the customer acts (01 `POST /widget/v1/conversations` with `opener`), so empty "hi" conversations never exist and never count. Audience fields limited to what the widget session knows (contact, company, `session.*`, page URL); conversation fields unavailable (publish error `field_unavailable_for_trigger`).

### During conversation (`during`, purple)

| Key | Name | Fires | CF | Channels | Re-entry default (allowed) | Payload |
|---|---|---|---|---|---|---|
| `message.customer` | Customer sends a message | every substantive contact message; first message excluded unless `trigger_settings.include_first` | no | all | every_time (all) | `message.*` |
| `conversation.customer_quiet` | Customer stops replying | `after` elapsed since the last public reply by a member or dimmi (`after_reply_from`: `member`, `dimmi`, default both) with no contact message since and conversation open | yes | all | every_time, re-armed by each qualifying reply (all) | `trigger.quiet_for`, `trigger.last_reply_by` |
| `conversation.rated` | Customer rates the conversation | 15 `survey_response.completed` on a conversation CSAT survey | message only (sent without reopening) | widget, email (15 CSAT channels) | once_per_conversation (all) | `rating.score` 1-5, `rating.comment`, `rating.ai_involved` |

`trigger_settings` for `customer_quiet`: `after` (1 minute to 30 days), `after_reply_from`, `count` (`calendar` | `business`, with `schedule_id`). Business counting: only open hours count.

### Team (`team`, orange)

| Key | Name | Fires | CF | Re-entry default | Payload |
|---|---|---|---|---|---|
| `message.teammate` | Teammate sends a reply | member public reply (any client: dashboard, app, API, CLI, MCP) | no | every_time | `message.*`, `trigger.member` |
| `note.created` | Teammate adds a note | member note (edits do not fire) | no | every_time | `note.body`, `note.mentions`, `trigger.member` |
| `conversation.assignment_changed` | Assignment changes | `conversation.assigned` by a member or API; `trigger_settings.to`: `any`, `member`, `team`, `unassigned` | no | every_time | `trigger.from_member`, `trigger.to_member`, `trigger.from_team`, `trigger.to_team` |
| `conversation.status_changed` | Conversation status changes | closed, snoozed, unsnoozed, reopened (manual or customer); `trigger_settings.to`: subset of `open`, `snoozed`, `closed` | no | every_time | `trigger.from`, `trigger.to`, `trigger.reason` |
| `conversation.teammate_quiet` | Customer waits for the team | `after` elapsed since `waiting_since` (first unanswered customer message) with no public member reply; fires once per waiting period | message and reply time only | once per waiting period (fixed) | `trigger.waiting_for` |

`teammate_quiet` settings: `after` (1 minute to 7 days), `count` calendar | business. Changes made by workflows never fire team triggers for the same workflow (engine.md "Loop guard"); changes by dimmi (actor agent) do not fire team triggers at all (they are not teammate actions), except `status_changed` which fires for any actor except the same workflow chain.

### dimmi (`dimmi`, lime)

| Key | Name | Fires | CF | Payload |
|---|---|---|---|---|
| `dimmi.handed_over` | dimmi hands over | `ai.handoff_requested` (any reason, inside or outside a workflow), after the 07 summary note exists | no | `dimmi.handover_reason`, `dimmi.summary`, `dimmi.replies`, `dimmi.run_id` |
| `dimmi.resolved` | dimmi resolves a conversation | dimmi step outcome `resolved`, or automatic dimmi receives an explicit confirmation (button "That helped") | message, close-with-rating yes | `dimmi.replies`, `dimmi.cited_articles` |

Handover reasons (07 enum): `customer_requested`, `unknown`, `unknown_streak`, `max_replies`, `frustration`, `feedback`, `abusive`, `safety`, `unsupported_language`, `provider_error`, `spend_cap`, `billing_paused`, `schedule`, `human_replied`, `member_paused`, `agent_disabled`, `action_failed`, `action_declined`, `action_expired`. No new reasons: "went quiet" is a dimmi step outcome, not a handover.

### Service levels (`sla`, red)

| Key | Name | Fires | CF | Payload |
|---|---|---|---|---|
| `sla.warning` | SLA about to breach | target reaches `due_at - warn_before` while active | no | `sla.target`, `sla.policy_id`, `sla.due_at` |
| `sla.breached` | SLA breached | target passes `due_at` while active | no | same plus `sla.breached_at` |

`trigger_settings.targets`: subset of `first_response`, `next_response`, `resolution` (default all).

### Reusable (`reuse`, neutral)

| Key | Name | Fires | CF |
|---|---|---|---|
| `reusable` | Run from other workflows | `run_workflow` step, member Cmd+K (`workflows:run`), API `POST /conversations/{conv}/workflow-runs` | yes when caller allows |

No audience, schedule or re-entry of its own (fields ignored, publish warning if set). Inherits caller run variables (copy); variables it sets are copied back to the caller when it ends `completed`. A reusable workflow with customer-facing steps cannot be called from a background run (publish error on caller `customer_facing_call_from_background`). Manual and API starts of customer-facing reusable workflows need the conversation unclaimed, or `replace: true` (cancels the current claim, reason `replaced`).

## 3. Conditions

One language for workflow audience, branch paths and wait interrupts. Reuses 15 "Audience rule language" tree (`all` / `any`, max depth 3, max 30 leaves), operators and missing-value semantics exactly (Go evaluator `app/support/conditions` wraps `app/support/segments`; property test shares the 15 corpus). Adds `not` node (single child) and these fields:

| Field | Type | Ops | Available |
|---|---|---|---|
| contact.*, company.* | as 15 | as 15 | all triggers |
| session.page_url, session.locale, session.country | string | eq, contains, starts_with, ends_with, in | opener |
| conversation.channel_type | enum registry | eq, neq, in, not_in | all except opener |
| conversation.channel_connection_id, inbox_id | id | eq, in, not_in | same |
| conversation.status | enum | eq, in | same |
| conversation.priority | bool | is_true, is_false | same |
| conversation.tags | tag ids | has, has_any, has_none | same |
| conversation.assignee_id, team_id | id or null | eq, neq, in, exists, not_exists | same |
| conversation.ai_state | enum | eq, in | same |
| conversation.subject | string | contains, contains_any, starts_with, exists | same |
| conversation.source.page_url | string | eq, contains, starts_with | same |
| conversation.source.announcement_id | id | eq, exists | same |
| conversation.message_count | number | number ops | same |
| conversation.waiting_minutes | number | number ops | same |
| conversation.created_at | date | date ops | same |
| conversation.is_test | bool | is_true, is_false | same |
| conversation.sla.<target>.state | enum | eq, in | same |
| message.body | string | contains, contains_any (keyword list, word-boundary, case- and accent-insensitive, max 50 keywords), not_contains_any, exists | message triggers |
| message.has_attachments | bool | is_true, is_false | message triggers |
| message.language | BCP 47 primary | eq, in | message triggers (07 classifier value when known, else contact locale) |
| note.body | string | contains, contains_any | `note.created` |
| trigger.* | per trigger | eq, in, number ops | that trigger |
| rating.score | number | number ops | `conversation.rated` |
| dimmi.handover_reason | enum | eq, in, not_in | `dimmi.*` triggers, steps after a dimmi step |
| office_hours.open | bool, with `schedule_id` (null = default) | is_true, is_false | all |
| time.weekday | enum mon..sun (workspace tz) | in | all |
| run.vars.<key> | by JSON type | as custom attributes | branch, wait interrupt (not audience) |

- No regex (ReDoS, explainability). Keyword lists cover the need; AI classify covers fuzzy intent.
- Unknown field for trigger: publish error `field_unavailable_for_trigger`.
- Referenced tag, team, member, schedule deleted: condition evaluates false, workflow resource `warnings` lists it, 06 notifies admins once.

## 4. Steps

Columns: CF = customer-facing (makes the version `customer_facing`). Paths: fixed outcome keys or user-defined. Every step records a trace row (`outcome`, `path_id`, `detail`).

### Customer-facing

#### message: Send a message (blue)

Settings: `text` (1 to 4,000 chars, Markdown subset of 03, variables), `delay` (0 to 30 s, typing indicator shown on widget and channels with `Typing` while delaying; default 0).
Behaviour: 03 `SendSystemMessage` with `author_type system`, `substantive false`, `metadata.workflow`, dedupe key `wf:<run>:<step>`. Email: in-thread reply, `Auto-Submitted: auto-replied` (04). Outcome `sent`. Skipped (`skipped_teammate_replied`) when a member public reply exists after the run's claim (race guard, engine.md).

#### buttons: Reply buttons (blue)

Settings: `text` (question, required), buttons = paths with `label` (1 to 40 chars, 2 to 6 paths), `allow_typing` (default true), `timeout` (null or 1 minute to 30 days), `retry_text` (text channels: sent once when the reply matches no choice; default localized "Please reply with one of the numbers above.").
Paths: one per button, plus optional fixed `typed` (customer typed text instead; only when `allow_typing`), optional fixed `timeout`.
Rendering: widget quick-reply buttons under the message (`metadata.workflow.buttons`), composer hidden while waiting when `allow_typing = false`; email and chat apps: question plus numbered list "1. Billing 2. Something else", matched by number, exact label (case- and accent-insensitive), unique label prefix of 3+ chars, or a reply containing exactly one label as whole words; no match: `typed` path when allowed, else `retry_text` once, then `typed` behaviour falls back to ending the run `exited` reason `no_choice`.
Answer: widget click posts a contact message with the label as body and `workflow_answer` (01); the message is a normal substantive contact message (transcript reads naturally, customer is now a conversation participant). Outcome `button` (path id), `typed`, `timeout`.

#### collect: Collect data (blue)

Settings: `text` (question), `field` (`contact.email`, `contact.name`, `contact.phone`, `contact.custom.<key>`, `run.vars.<key>`), `type` (`text`, `email`, `phone`, `number`, `choice` with options, `yes_no`), `skip_if_known` (default true: field already has a value, step outcome `already_known`, no question), `invalid_text`, `max_attempts` (1 to 3, default 2), `timeout`, `allow_skip` (widget "Skip" chip).
Behaviour: asks, waits for next contact message (debounced `workflows.input_debounce`), validates (email RFC 5322 light + MX not checked, phone E.164 via 02 helper, number parse, choice match like buttons). Valid: writes field (contact attribute writes via 03 `UpdateContact`, actor system via workflow; email never marked verified; never merges), outcome `collected`. Invalid after `max_attempts`: outcome `invalid_answer`, value stored nowhere. Paths: fixed `collected` (implicit continue), `invalid` (optional), `timeout` (optional). When path absent the step continues down the main line.
Widget renders typed inputs (email keyboard, number) from `metadata.workflow.input`.

#### reply_time: Show reply time (blue)

Settings: `schedule_id` (null = default), `team_id` (null = conversation team or inbox default team), `style` (`typical` "The team typically replies in about 2 hours" | `next_open` "We're back Monday at 09:00" | `auto` = typical when open, next open when closed; default `auto`).
Typical reply time = median first public member reply over the last 14 days for that team (or inbox), outside-hours time excluded when a schedule is set, rounded to friendly buckets (a few minutes, under 1 hour, a few hours, about a day); computed hourly, cached in Valkey per team; fewer than 10 samples: "The team will reply as soon as they can." Message sent like `message`.

#### dimmi: Let dimmi handle (lime)

Settings:
- `instructions` (0 to 1,000 chars): appended to 07 guidance for runs while this step is active ("Only answer billing questions; hand over refunds").
- `max_replies` (1 to 50, default agent `max_ai_replies`).
- `quiet_after` (1 minute to 7 days; default 10 m widget and chat apps, 1 d email): no customer message since the last dimmi answer for this long = outcome `quiet`.
- `confirm_resolution` (default true): widget answers carry "That helped" / "I need more help" quick replies (07 renders, `metadata.ai.actions`); email and chat apps: last line "Reply YES if this solved it." matched by 07 confirm phrase list.
- `close_on_resolved` (default true): conversation closed by the engine when outcome is `resolved` and path `resolved` is empty.
- `ask_email_before_handover` (default true on widget when contact has no email: 07 `request_email`).
- `handover_notice` (`dimmi` default | `workflow`): `workflow` = dimmi sends no handover notice; the handed-over path is expected to speak (publish warning when the path starts with no customer-facing step).
Paths (fixed): `resolved`, `handed_over`, `quiet`. All optional; empty path = run ends `completed`.
Behaviour: engine.md "dimmi step". Not available when the trigger is a team, dimmi or SLA trigger (publish error `step_unavailable_for_trigger`). Agent disabled or not ready at publish time: publish warning `dimmi_not_ready` (allowed; at run time the step takes `handed_over` with reason `agent_disabled` immediately).

#### close (with rating): see Actions below; with `ask_rating = true` the step is CF.

### Logic

#### branch: Branch (purple)

Paths: 1 to 10 user paths each with `label` and `condition` (section 3), evaluated top to bottom, first match wins; fixed final path `else` ("Everyone else", always present, may be empty). Outcome `matched` (path id) or `else`. Trace detail records the first failing leaf of each path checked (explainer).

#### classify: Let dimmi pick a path (purple, AI)

Settings: `question` (optional extra context for the classifier, 0 to 500 chars), paths 2 to 10 with `label` and `description` (what belongs there, 1 to 300 chars), `source` (`last_customer_message` default | `conversation`: last 6 customer and public messages), `min_confidence` (`low` | `medium` default | `high`).
Fixed path `none` ("None fits"). Also taken when AI is unavailable (no valid key, 07 cap reached, provider error after retry, billing paused): outcome `ai_unavailable` with reason in trace.
Behaviour: one call on the 07 classifier model via `ai.RunTask` (kind `workflow_classify`, JSON schema `{label, confidence}`), customer text wrapped untrusted, label must be one of the configured ids, else `none`. p95 budget 1.5 s; timeout 8 s. Cost recorded on an `ai_runs` row linked from the trace. Tests use the 07 fake LLM driver only (never a real model).

#### wait: Wait (purple)

Settings: `for` (1 minute to 30 days) or `until` (`office_hours_open` with `schedule_id`, or `time_of_day` "09:00" next occurrence in workspace tz), `count` (calendar | business), `interrupt_on` (subset of `customer_message`, `teammate_reply`; default `customer_message`).
Paths (fixed): `elapsed` (main line, implicit continue), `interrupted` (optional; taken when an `interrupt_on` event happens). Interrupted with no `interrupted` path: run ends `completed` (end reason `interrupted`). Close, spam, delete always end the run (engine.md). Background runs may wait too.

#### run_workflow: Run a workflow (neutral)

Settings: `workflow_id` (must be a `reusable` workflow, live at publish time of caller). Behaviour: starts child run on the child's live version at call time, caller waits (`wait.kind = child`); child `completed` or `passed` copies vars back and the caller continues; child `failed`, `exited`, `canceled`, `expired` ends the caller with the same end reason (prefixed `child_`). Depth max 3; cycles impossible because depth is checked at run time and publish rejects direct self-reference.

### Actions (neutral)

| Kind | Settings | Behaviour and outcomes | Failure fallback |
|---|---|---|---|
| `assign` | `mode`: `member` (`member_id`), `team` (`team_id`, keeps member), `team_round_robin`, `team_balanced` (both `team_id`, `only_available` default true, `fallback`: `team_only` default or `leave`), `keep_if_assigned` (default true: skip when a member is already assigned) | 03 `AssignConversation` (actor system via workflow). Round robin: next active member after cursor in `team_members` order by member id, skipping deactivated and (when `only_available`) members not `online` (00-accounts availability), cursor row locked FOR UPDATE. Balanced: fewest `open` conversations assigned (ties: least recently assigned), same availability rule. Outcomes `assigned`, `kept`, `no_eligible_member` (team set, member null) | member deactivated: assign team of step if any, else inbox default team; trace `fallback_used`; workflow `warnings` |
| `unassign` | `what`: `member`, `team`, `both` | 03 `AssignConversation` with nulls | none needed |
| `tag` | `tag_ids` (1 to 20) | 03 apply tags; outcome `tagged` (count added) | deleted tag skipped, `warnings` |
| `untag` | `tag_ids` | remove | same |
| `priority` | `value` bool (default true) | 03 `UpdateConversation` priority (core column is boolean; 5 levels later with ticket fields) | none |
| `note` | `text` (variables incl. `{{dimmi.summary}}`, `{{run.vars.*}}`), `mention_member_ids`, `mention_team_ids` | 03 note, `author_type system`, mentions notify (06) | none |
| `update_contact` | `set`: list of `{field: contact.custom.<key> or contact.name or contact.phone, value}` (literal or variable), max 10 | 03 `UpdateContact` (actor system via workflow); never `email` or `external_id` (identity) | invalid value: skip that field, trace |
| `snooze` | `for` duration or `until` `office_hours_open` | 03 snooze (actor workflow); waking by customer message as usual | closed conversation: skip `not_open` |
| `close` | `ask_rating` (default false: 15 CSAT if its settings allow this channel; CF), `only_if_no_teammate_reply` (default false) | 03 close (actor workflow). Customer-facing runs end at close anyway | already closed: skip |
| `apply_sla` | `sla_policy_id` | `ApplySlaPolicy`: replaces active targets (engine.md "SLAs") | deleted policy: skip, `warnings` |
| `http_request` | `connection_id` (07 action connection) + `path`, or `url` (https; signed); `method` POST default (GET, POST, PUT, PATCH); `headers` (max 10, values may reference connection secrets only through the connection); `body` (`default` = standard payload: run, workflow, conversation, contact resource subsets) or `template` JSON with variables; `map`: `[{pointer: "/plan", to: "run.vars.plan"}]` max 10 | engine.md "HTTP request". Paths fixed: `success` (2xx; main line), `failure` (optional; else run continues with outcome `failure` recorded). Response status, latency, mapped keys in trace; body never stored | retries then `failure` |

## 5. Allowed steps per trigger

| Trigger group | message, buttons, collect, reply_time | dimmi | classify, branch, wait, run_workflow | actions |
|---|---|---|---|---|
| start (opener, first message, reopened) | yes | yes | yes | yes |
| `message.customer` | no | no | yes (run_workflow background only) | yes |
| `conversation.customer_quiet` | yes | yes | yes | yes |
| `conversation.rated` | message only | no | yes | yes |
| team triggers except teammate_quiet | no | no | yes | yes |
| `conversation.teammate_quiet` | message, reply_time | no | yes | yes |
| `dimmi.handed_over` | no | no | yes | yes |
| `dimmi.resolved` | message, close with rating | no | yes | yes |
| `sla.*` | no | no | yes | yes |
| `reusable` | yes (callers checked) | yes | yes | yes |

Violations: publish error `step_unavailable_for_trigger` with step id.

## 6. Publish validation (422 `workflow_invalid`, `errors: [{code, step_id?, path_id?, field?}]`; warnings need `acknowledge_warnings`)

Errors: `schema`, `opener_shape`, `step_unavailable_for_trigger`, `field_unavailable_for_trigger`, `unknown_variable`, `missing_setting`, `reference_missing` (team, member, tag, schedule, SLA policy, connection, reusable workflow not live), `customer_facing_call_from_background`, `self_call`, `limit_exceeded` (which limit), `duplicate_id`, `invalid_duration`, `channel_unsupported` (trigger cannot run on a selected channel).
Warnings: `dimmi_not_ready`, `path_ends_unowned` (a customer-facing path ends with no dimmi step, no assignment and no close: conversation will sit unassigned in the inbox), `else_empty_on_audience_branch`, `handover_notice_silent`, `audience_matches_nothing` (preview count 0 over last 30 days), `overlaps_higher_workflow` (a higher live customer-facing workflow on the same trigger matched every conversation this one would have matched in the last 30 days: this one would never run), `http_raw_url`.

## 7. Templates

JSON in `resources/workflows/templates/<key>.json` (`schema` 1, ids placeholders, references as `{"$ref": "default_team"}` resolved at instantiation; unresolved references leave the setting empty and the draft shows `missing_setting`). CI validates every template with the publish validator against a seeded workspace. `GET /workflow-templates`, `POST /workflows {template: key}` creates a draft.

| Key | Name | Trigger | Shape |
|---|---|---|---|
| `route_by_topic` | Route by topic | first_message | buttons (Billing, Bug, Something else) -> assign teams; else dimmi |
| `dimmi_first` | dimmi first, then the team | first_message | dimmi -> resolved: close; handed over: collect email, assign team round robin, reply time; quiet: message nudge, wait 1 d, close |
| `opener_menu` | Greet with a menu | opener | message + buttons -> paths as route_by_topic |
| `out_of_hours` | Out-of-hours reply | first_message, schedule closed | reply_time (next open), collect email when missing, tag "after-hours" |
| `close_inactive` | Close inactive conversations | customer_quiet 2 d | message "Still need help?", wait 1 d (interrupt customer), close |
| `vip_fast_lane` | Fast lane for VIP customers | first_message, audience company plan | tag VIP, priority, apply SLA "VIP", assign member; exclusive |
| `bad_rating` | Follow up on a bad rating | rated, branch score <= 2 | message asking what went wrong (conversation stays closed until they answer), note mentioning the team, priority |
| `handover_brief` | Brief the team on handover | dimmi.handed_over | note `{{dimmi.summary}}`, branch reason (frustration, safety -> priority), assign team balanced |
| `sla_escalation` | Escalate before SLA breach | sla.warning | note mention, assign team balanced when unassigned; sla.breached sibling template adds priority |
