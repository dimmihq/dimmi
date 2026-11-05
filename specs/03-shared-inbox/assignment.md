# 03 Shared inbox: assignment and availability

Style: caveman. Gaps I-10, I-11, I-12, I-13, I-14, T-01. X3, X11. Track 02 inbox-access, except away reasons and automatic away in track 12 inbox-extras. No workflow engine needed. 17 `assign` step calls same picker.

## Modes (per inbox, I-10, I-13)

`inboxes.settings.assignment`:

```json
{ "mode": "manual", "include_away": false, "overflow_team_id": null, "queue_order": "oldest_waiting",
  "default_capacity": null, "capacity_by_channel": {} }
```

| Key | Values | Meaning |
|---|---|---|
| `mode` | `manual` (default), `round_robin`, `balanced` | manual: nobody auto-assigned; `assign_on_reply` still applies |
| `include_away` | bool, default false | pool includes away members (Intercom per-team toggle). Deactivated never |
| `overflow_team_id` | team or null | secondary pool: tried when primary has nobody eligible. One level |
| `queue_order` | `oldest_waiting`, `priority_then_oldest` | order draining queue |
| `default_capacity` | int 1 to 500 or null | member limit when member has none. Null = unlimited |
| `capacity_by_channel` | `{channel_type: int}` | per channel limit for all members of pool, 1 to 500 |

Pool = active members of `conversation.assignee_team_id`, else `inbox.default_team_id`, else all active members of workspace. Restricted members stay eligible: assignment grants sight ([access.md](access.md)).

## When picker runs

Queued listener `AssignOnArrival` (queue `default`), one action `assignment.AssignOnArrival(ctx, conversationID, reason)`:

| Source | Reason | Condition |
|---|---|---|
| `conversation.created` by contact | `arrival` | member null, not spam, `ai_state <> 'active'` |
| `ai.handoff_requested` (07) | `handoff` | member null |
| `conversation.assigned` to team, member null, actor member or API | `team_assigned` | inbox mode not manual |
| `conversation.reopened` | `reopened` | member null |
| Reassign on away (below) | `away_reassign` | |

Never for: spam, blocked contact, closed, imported, conversation claimed by workflow run (17 adds column check; on run end with member null, 17 calls `AssignOnArrival`). Runs once per reason per conversation: River unique `ByArgs` + re-check member null under row lock. Why queued: ingest budget 50 ms stays clean, and inbound never fails because of pick.

## Picker

`app/actions/assignment/pick_assignee.go`, sole implementation, also called by 17 `assign` step:

```go
type PickInput struct {
    WorkspaceID, InboxID ids.UUID
    TeamID      *ids.UUID
    Mode        string // round_robin | balanced
    ChannelType enums.ChannelType
    IncludeAway bool
    Tx          database.Tx
}
type PickResult struct { MemberID *ids.UUID; Outcome string } // assigned | no_available_member | all_at_capacity
func PickAssignee(ctx, in PickInput) (PickResult, error)
```

- Eligible: active, member of pool, `availability = 'online'` unless `IncludeAway`, under capacity.
- Round robin: next eligible after cursor by member id, wraps. Cursor row `team_assignment_cursors` locked `FOR UPDATE`. Pool with no team: cursor keyed by inbox.
- Balanced: fewest open assigned conversations, tie least recent `last_assigned_at`, then member id. Same cursor lock serializes picks in pool, so two arrivals cannot both take same slot.
- Open count: `status = 'open' AND NOT spam AND deleted_at IS NULL`. Snoozed not counted (Intercom same; returns hit capacity harmlessly). Index `(workspace_id, assignee_member_id, status...)` exists.
- Capacity: member `conversation_capacity`, else inbox `default_capacity`, else none. Channel: member `capacity_by_channel[type]`, else inbox's. Both must hold.
- No one in primary: try `overflow_team_id` pool.
- `AssignConversation` with actor system (`via: {kind: assignment}`), event reason. Team stays. Sets `workspace_members.last_assigned_at`.

### Waiting and explanation

Nobody eligible in `balanced` or `round_robin`: conversation keeps team, member null, and:

- `conversations.assignment_wait_reason` text null CHECK in (`no_available_member`, `all_at_capacity`), `assignment_waiting_since` timestamptz null. Cleared on any assignment, close, spam, trash.
- Timeline event row `conversation.assignment_waiting` once per wait, shown in thread: "Waiting: everyone on Billing is at capacity".
- Details panel shows reason. `GET /inboxes/{inbox}/assignment-status` explains per member (below).

Drain: job `drain_assignment_queue` (`default`, unique per inbox + pool, debounce 5 s), triggered by close, snooze, reassign, spam, member back online, capacity change, plus periodic every minute `maintenance` for missed triggers. Takes waiting rows in `queue_order`, `FOR UPDATE SKIP LOCKED`, assigns until nobody eligible. Index `(workspace_id, inbox_id, assignment_waiting_since, id) WHERE assignment_wait_reason IS NOT NULL`.

Round robin with capacity unset: only `no_available_member` occurs.

### Team cursor ownership

`team_assignment_cursors` moves here from 17 (`20261101001710` dropped there). Shape widens: `pool_kind text CHECK in ('team','inbox')`, `pool_id uuid`, PK `(pool_kind, pool_id)`, `workspace_id`, `last_member_id`, `last_assigned_at`, `updated_at`. No fk on `pool_id`; team and inbox delete listeners remove row.

## Away status (I-11, X3)

Existing: `workspace_members.availability` CHECK (`online`, `away`), read by member resources, written by nothing. Keep values, 17 reads them.

New columns (migration below): `away_reassign boolean not null default false`, `away_since timestamptz`, `availability_source text not null default 'manual' CHECK in ('manual','auto')`, `away_reason_id uuid null`.

User-facing three states: Active, Away, Away and reassigning.

- `PATCH /members/me {availability, away_reassign, away_reason_id}` self (ability `conversations:write`; 00-accounts route, 03 action `SetMemberAvailability`). Others: `PATCH /members/{mem}` same fields, permission `members:manage_availability` (`members:write`; owner, admin).
- `availability: online` clears `away_reassign`, reason, `away_since`. `away_reassign: true` with online: 422.
- Emits `member.availability_changed`, realtime `workspace:{ws}`, webhook no. Audit only when set by someone else.
- Online again: triggers drain.

Reassign on reply: inbound substantive contact message, assignee `away_reassign = true`: in `ReceiveInboundMessage` tx after message insert, unassign member (keep team), event row reason `away_reassign`, then queue `AssignOnArrival`. Inbox mode manual: stays with team, or Unassigned if none. Notes, auto-generated mail, imported: no effect. Away without reassign: stays on them, still skipped by picker.

UI: toggle in user menu `resources/js/components/application/dashboard/dashboard-user-menu.tsx` (edit block, real data), status dot on sidebar avatar. Preferences page (track 07, `settings/pages/account.tsx`) shows same control calling same API; never its own state. Sidebar teammate list dot from `workspace:{ws}` presence + availability.

## Away reasons and automatic away (I-12, track 12)

`away_reasons`: `id` (`awr_`), `workspace_id`, `label` 1 to 60, `emoji` null, `position int`, `archived_at`, timestamps. Unique active label citext per workspace. Limit 100, 422 `away_reason_limit_reached` (fixed product limit). FK `workspace_members.away_reason_id` set null added here.

Workspace settings: `availability.away_reason_required` bool default false (away without reason 422 `away_reason_required`, skipped for auto), `availability.auto_away_minutes` int null, 5 to 1440, default null (off).

- API: `GET /away-reasons`, `POST`, `PATCH /away-reasons/{id}`, `DELETE` (archives, members keep label until they change); manage permission `inboxes:manage`. Order via `PATCH /away-reasons {order: [...]}` (rest rule 9).
- Activity: dashboard calls `POST /member-activity` (`U` guard + workspace, 204) at most every 60 s, only with real input in last minute. Writes Valkey `member_active:{mem}` TTL 25 h. No Postgres write.
- Job `auto_away_members` (`maintenance`, every minute, workspaces with setting on): online members with key older than threshold set away, `availability_source = 'auto'`, reason null.
- Return: activity call on member with source `auto` flips online. Manual away never auto-returns.
- Valkey flushed: no key means unknown, not idle; job skips until key reappears. Why: flush must not mark whole team away.

UI: settings section on `settings/pages/inbox.tsx` "Availability"; reason picker in user menu.

## Per-member self assign (I-14)

`workspace_members.assign_on_reply text not null default 'inherit' CHECK in ('inherit','always','never')`.

`AssignOnReply`: `never` no; `always` yes; `inherit` inbox `assign_on_reply`. `always` also assigns on manual reopen and manual unsnooze of unassigned conversation by that member (Intercom rule). Field on `PATCH /members/me`; control on Preferences (track 07).

## Member capacity

`workspace_members.conversation_capacity int null CHECK (1 to 500)`, `capacity_by_channel jsonb not null default '{}'`, `last_assigned_at timestamptz`. Set through `PATCH /members/{mem}` with `members:manage_availability`. Lowering under current load: nothing moved, member just gets no new.

## API summary

| Method | Path | Ability, permission | Purpose |
|---|---|---|---|
| PATCH | /inboxes/{inbox} | settings:write, `inboxes:manage` | `name`, `default_team_id` (null clears), `settings` as a merge patch; `settings.assignment` keys, validated as table above; `overflow_team_id` equal to default team 422 |
| DELETE | /inboxes/{inbox} | settings:write, `inboxes:manage` | `?move_to=inbox_...` required when it holds conversations; moves conversations and channels; default inbox 422 `default_inbox` |
| GET | /inboxes/{inbox}/assignment-status | settings:read | `?channel_type=widget|email|api` (default widget) picks which channel limits apply; computed: `{mode, members: [{member, availability, open_count, capacity, eligible, why_not: away|at_capacity|at_channel_capacity|null}], waiting: {count, oldest_at, reasons}}`. Route name `inboxes.assignment-status.show` |

Conversation resource adds `assignment_wait: {reason, since}|null`. Filter `filter[assignment_waiting]=true`.

## Inbox settings page (T-01)

Replace sample in `resources/js/features/settings/pages/inbox.tsx` `InboxesSettings` (fake Billing inbox goes). Query layer `resources/js/lib/api/inboxes.ts`.

- Section Inboxes: real list, New inbox dialog (BoardUI `Modal`, `Input`), Default chip, Edit, Delete. Delete asks target inbox when non-empty (`?move_to=`), default inbox: control disabled with reason.
- Inbox detail `/d/settings/inboxes/$inbox` route: name, colour (`color-picker`), default team (`Select`), reopen window (Never, 7, 30, 90, custom 1 to 365), assign to whoever replies, assignment mode (segmented: Manual, Round robin, Balanced), include away, capacity defaults, per channel limits, overflow team, queue order; table of `assignment-status`.
- Section Messenger: "Name in seen receipts" wired to `workspaces.settings.seen_receipt_shows_name` (I-130).
- Section Snooze: snooze list editor ([workbench.md](workbench.md) I-21, track 03 fills in).
- Member without `inboxes:manage`: read only, controls disabled.
- `TagsSettings` real: [records.md](records.md), track 04. `SavedRepliesSettings` stays sample until its own work; saved replies excluded from this round.
- e2e: `tests/e2e/inbox-settings.spec.ts`.

## Migrations (track 02)

| Migration | Change |
|---|---|
| `20261201020004_add_availability_columns_to_workspace_members.sql` | `away_reassign`, `away_since`, `availability_source`, `away_reason_id` (no fk yet), `assign_on_reply`, `conversation_capacity`, `capacity_by_channel`, `last_assigned_at` |
| `20261201020005_create_team_assignment_cursors_table.sql` | as above |
| `20261201020006_add_assignment_wait_to_conversations.sql` | `assignment_wait_reason`, `assignment_waiting_since`; partial index CONCURRENTLY |

Track 12: `20261201120001_create_away_reasons_table.sql` (+ fk).

`inboxes.settings` typed struct `models.InboxSettings` gains `assignment`; absent reads as manual.
