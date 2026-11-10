# 22 Operator

Date: 2026-11-09. Style: caveman. Status: building.

Operator is the team's own agent. It works for teammates and admins, never for customers. Nothing Operator says
reaches a customer unless a teammate sends it. One agent, two postures:

- **Teammate posture** (inside a conversation): briefs the case, answers with citations, drafts replies into the
  composer, proposes triage (tags, assignee, team, priority, status, notes), looks things up across the workspace.
- **Operator posture** (whole workspace): answers questions about volume, response times, resolution and the AI
  agent's outcomes, finds knowledge gaps, drafts and edits help articles, reads why the AI agent did what it did,
  edits procedures' working copies, proposes changes in bulk.

Both postures share one engine, one tool registry, one changeset model, one audit trail and the workspace's AI
budget.

## Principles (enforced in code, not prompts)

1. **Borrowed identity.** An interactive run acts as the member who asked: their role, permissions and conversation
   visibility. Every tool goes through the same action the REST API calls, so a member can never do through Operator
   what they could not do by hand.
2. **Every write is a changeset.** Tools never write directly. A write tool records a changeset: an ordered list of
   operations, each with the value before and after. A changeset is applied as a unit, reverted as a unit, and audited.
3. **Policy per tool group, set by admins.** `auto` (apply at once, still revertible), `ask` (a teammate approves the
   card), `off` (tool not offered). Defaults: every write group `ask`.
4. **Rule of two.** Once a run has read customer-written text (conversation messages, contact fields), its
   changesets never auto-apply, whatever the policy. Customer text is wrapped and labelled untrusted in the prompt.
5. **Approval is not publication.** Article tools write drafts and working copies only; publishing stays a human
   action in the editor. Procedure tools edit the working copy only.
6. **Cite and show the work.** Answers link the conversations, articles and runs behind them. Each run streams its
   steps (tool name, short summary) and its text; the panel shows them.
7. **Cost is a feature.** Every run is an `ai_runs` row (kind `task`) with tokens and cost, counted in the month's
   `ai_spend_periods`, stopped by the agent's monthly cap. Features that spend without a teammate asking are off by
   default.

## Settings (`operator_settings`, one row per workspace, defaults when absent)

| Field | Default | Meaning |
|---|---|---|
| `enabled` | true | Operator answers teammates. Spends only when asked. |
| `brief_on_open` | false | Brief a conversation as soon as its pane opens. Off: the pane offers "Brief me". Costs per open. |
| `triage_on_arrival` | false | When a conversation arrives, Operator proposes tags, team, priority and a summary note as a pending changeset. Costs per conversation. |
| `instructions` | empty | Workspace instructions every run reads (tone, policies, who owns what). Max 8000 chars. |
| `policies` | `{}` | Map tool group to `auto`, `ask` or `off`. Missing group means `ask`. |
| `model` | null | Model Operator answers with, on the agent's answer connection. Null: GPT-6 Luna when the connection offers it (the cheapest that does the job; GPT-6.1 Terra costs ten times as much), else the agent's own model. |

Groups: `conversations` (tags, assign, priority, status, title), `notes`, `contacts`, `articles`, `procedures`,
`actions`. `articles`, `procedures` and `actions` can never be `auto`: drafts are still reviewed and actions cannot be
undone.

## Workspace actions

- A workspace connects its systems once (`ai_action_connections`, `ai_actions`). Each action has `agents`
  (`dimmi`, `operator`, or both; default `{dimmi}`), part of the published version like `channels`.
- dimmi keeps its own rules: channels, customer confirmation, teammate or admin approval.
- Operator is offered published actions given to it, as tools named `action_<name>`; channels do not apply.
  - In a thread about a conversation, the customer's details bind as verified (a teammate is acting); elsewhere only
    actions that bind no customer detail are offered.
  - Read: runs at once as the teammate (`ai_action_calls` row, run = the Operator run). Result is untrusted.
  - Write: never runs from the chat. It becomes a one-operation changeset `action.run` (action, version, arguments).
    Applying it is the approval: the call runs at once with `decided_by_member_id` = the applier. An action with
    `approval = admin` needs `ai_actions:manage` to apply. The version must still be published and given to
    Operator, or applying is refused. Reverting skips it: it cannot be undone.

Permission: reading Operator settings needs `ai:read`; changing them `ai:write` (`ai_agent:manage`). Using Operator
needs conversations read access; each tool checks its own permission through the action it calls.

## Data model

- `operator_settings (workspace_id pk, enabled, brief_on_open, triage_on_arrival, instructions, policies jsonb,
  updated_at)`.
- `operator_threads (id, workspace_id, member_id, conversation_id null, title, created_at, updated_at,
  last_message_at)`. Any number of threads per member and conversation; the pane opens the latest and lists
  the rest. A thread that began with a brief is titled "Brief". A thread is private to its member.
- `operator_messages (id, workspace_id, thread_id, role user|assistant, body, steps jsonb, status
  done|stopped|failed, run_id null, created_at)`. `steps` is the run's visible activity:
  `[{tool, summary, ok}]`, plus `draft` entries for reply drafts.
- `operator_changesets (id, workspace_id, thread_id null, message_id null, member_id null, conversation_id null,
  title, reason, operations jsonb, status pending|applied|declined|reverted|failed, auto bool, error,
  decided_by_member_id, applied_at, reverted_at, created_at)`. `member_id` null means Operator proposed it on its
  own (triage on arrival).
- `operator_memories (id, workspace_id, member_id, body, created_at)`. Personal memory, written only when the member
  asks Operator to remember something. Listed and deleted in settings.

RLS on every table by workspace.

## Operations (changeset)

| Op | Fields | Apply through | Revert |
|---|---|---|---|
| `conversation.tag` | conversation, tags[] | `TagConversation` | untag the tags that were new |
| `conversation.untag` | conversation, tags[] | `UntagConversation` | tag them again |
| `conversation.assign` | conversation, member?, team? | `ApplyConversationChanges` | previous member/team |
| `conversation.priority` | conversation, priority | `ApplyConversationChanges` | previous priority |
| `conversation.status` | conversation, status (open, closed) | `ApplyConversationChanges` | previous status |
| `conversation.title` | conversation, title | `ApplyConversationChanges` | previous title |
| `conversation.note` | conversation, body | `AddNote` (author: the approving member, note marked Operator) | delete the note |
| `contact.update` | contact, name?, attributes? | `UpdateContact` | previous values |
| `article.create_draft` | help center, collection?, title, body | `CreateArticle` (draft, never published) | delete the draft if still unpublished |
| `article.edit_draft` | article, title?, body | article working copy | previous working copy |
| `procedure.edit_draft` | procedure, instructions | procedure working copy | previous working copy |
| `action.run` | action, version, arguments | the workspace action, pre-approved by the applier | none: cannot be undone |

Before values are read when the changeset is applied, not when it is proposed. Revert checks each field still holds
the value Operator wrote; a field someone changed since is skipped and reported, never overwritten.

## Tools

Read (always offered): `get_conversation`, `search_conversations`, `get_contact`, `search_knowledge`,
`get_article`, `list_articles`, `list_tags`, `list_teammates`, `workspace_report`, `agent_runs`,
`list_procedures`, `get_procedure`.

Draft: `draft_reply` (text into the composer; never sent).

Workspace actions: `action_<name>` for each published action given to Operator.

Write: `propose_changes` (operations above, with a title and a reason). Memory: `remember`.

`workspace_report` metrics over a date range (max 366 days): conversations opened/closed by day and channel, median
first response and resolution time, AI runs by outcome, handoff reasons, top tags, busiest hours, article helpfulness.

## Surfaces

- Conversation pane (inbox right pane, `O`): the latest thread for this conversation, past threads in a menu, a
  new thread on request. Brief, chat, steps, draft cards ("Use in reply"), changeset cards (Apply / Decline /
  Undo). "Brief me" also keeps the brief on the conversation as an internal note by the teammate, so the whole
  team reads it; a brief made on open (`brief_on_open`) stays in the thread only.
- Agent page, Operator tab (`/d/{workspace}/agent?tab=operator`; `/d/{workspace}/operator` lands there): whether it
  is on and its model, this month's runs, spend against the shared cap, changes applied of those proposed, failed
  runs, runs by day, runs by purpose (ask, brief, triage), by teammate, by model, and the latest 25 runs.
- Settings (`/d/{workspace}/settings/ai/operator`): the fields above plus memories.
- Streaming: member channel, ephemeral `operator.delta` and `operator.step`; the POST answers with the stored message.
- Slack (`23-integrations` install, events routed by 05): @dimmi in `internal` Slack rooms and DMs to the bot, plus Slack assistant threads (`assistant:write`). Asker must be a linked member (05 `channel_member_links`); others get an ephemeral "Link your Slack account to use dimmi" with grant link. Run borrows that member's identity exactly as in the app. Reply in thread, streamed with `chat.startStream`/`appendStream`/`stopStream` where Slack allows, else placeholder then `chat.update`; steps shown as context line. Changeset cards render as Block Kit with Apply / Decline / Undo buttons, approval checked against clicker's identity. Never answers in `shared`, `community` or `helpdesk` rooms (customers read there). Each Slack thread maps to one Operator thread (`operator_threads.metadata.slack {channel, thread_ts}`); a dimmi conversation link in the message scopes the run to that conversation.

## API

| Method | Path | Ability |
|---|---|---|
| GET/PATCH | `/operator/settings` | `ai:read` / `ai:write` |
| GET/POST | `/operator/threads` (`filter[conversation]`, `filter[scope]=workspace`; POST `fresh`) | `conversations:read` |
| GET/DELETE | `/operator/threads/{thread}` | `conversations:read` |
| POST | `/operator/threads/{thread}/messages` | `conversations:read` |
| POST | `/conversations/{conversation}/operator/brief` (`thread_id`, `attach`) | `conversations:read` |
| GET | `/operator/usage?period=YYYY-MM` | `ai:read` |
| GET | `/operator/changesets?conversation=&status=` | `conversations:read` |
| POST | `/operator/changesets/{changeset}/applications` | `conversations:write` |
| POST | `/operator/changesets/{changeset}/declines` | `conversations:read` |
| POST | `/operator/changesets/{changeset}/reversions` | `conversations:write` |
| GET/DELETE | `/operator/memories[/{memory}]` | `conversations:read` |

## Audit

`audit_logs` entries `operator.changes_applied` and `operator.changes_reverted`, actor the member, `via_kind`
`operator`, changes holding each operation's before and after.

## Limits

- 12 model calls per run, 40 tool calls per run, run timeout from `ai.timeouts`.
- 30 questions per member per minute.
- Thread history sent to the model: last 30 messages.
- `search_conversations` and `workspace_report` read at most 500 rows per call.

## Later

Routines (scheduled runs that return changesets), Operator actions outside a conversation that bind a customer by
id, replay of past conversations against a proposed procedure change,
embeddings for semantic search over conversations, topic clustering, QA monitors, MCP exposure of the same tools.
