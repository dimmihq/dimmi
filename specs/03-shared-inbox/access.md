# 03 Shared inbox: conversation access

Style: caveman. Gap S-04. Binding decision X1. Track 02 inbox-access. Every query added by any spec uses this.

## Why now

Retrofit cost grows with each list, count, channel. One builder, one SQL function, one arch test.

## Rule

| `conversation_access` | Member sees |
|---|---|
| `all` (default) | every conversation in workspace |
| `assigned` | `assignee_member_id = me` |
| `teams` | `assignee_member_id = me` OR `assignee_team_id` in `member_access_teams` list |
| `all_except_teams` | `assignee_member_id = me` OR `assignee_team_id IS NULL` OR team not in list |

- `conversation_access_unassigned` (bool, default false): modes `assigned`, `teams` also see `assignee_member_id IS NULL AND assignee_team_id IS NULL`. Ignored for `all`, `all_except_teams` (they see those already).
- List explicit, not team membership. Joining team never widens access silently. Settings UI preselects member's own teams when mode picked.
- Own assignment always visible, any mode. Assigning member to conversation grants sight; reassign removes it.
- Owners always `all` (422 `owner_access_unrestricted`). Promotion to owner sets `all`, clears list. Why: last person able to reach everything.
- Token, OAuth grant, MCP, CLI, first-party app: follow owning member. Integration token with no member, agent (07), workflow (17), system jobs: `all`. Why: workspace-level actors have no person to restrict.
- Trash and spam views: scope applies too (trash also needs `conversations:delete`).
- Contacts and companies are not hidden. Directory stays workspace-wide under `contacts:read`. Only their conversation lists, past conversations and counts scoped. Why: same customer often spans teams; bodies are the secret.

## Data model

Migrations (track 02):

| Migration | Change |
|---|---|
| `20261201020001_add_conversation_access_to_workspace_members.sql` | `conversation_access text not null default 'all' CHECK in ('all','assigned','teams','all_except_teams')`, `conversation_access_unassigned boolean not null default false` |
| `20261201020002_create_member_access_teams_table.sql` | `member_access_teams (workspace_id uuid not null fk cascade, member_id uuid not null fk workspace_members cascade, team_id uuid not null fk teams cascade, created_at)`, PK `(member_id, team_id)`, index `(workspace_id, team_id)`, RLS tenant policy |
| `20261201020003_create_conversation_visible_function.sql` | SQL function below |

No public id (pivot). Team deleted: rows cascade; `teams` member with empty list sees own only (decided, safe direction); `all_except_teams` widens (team gone, nothing to exclude).

```sql
CREATE FUNCTION dimmi_conversation_visible(
  assignee_member uuid, assignee_team uuid,
  mode text, viewer uuid, teams uuid[], unassigned boolean
) RETURNS boolean LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
  SELECT mode = 'all'
      OR assignee_member = viewer
      OR (mode = 'teams' AND assignee_team = ANY(teams))
      OR (mode = 'all_except_teams' AND (assignee_team IS NULL OR NOT assignee_team = ANY(teams)))
      OR (unassigned AND mode IN ('assigned','teams') AND assignee_member IS NULL AND assignee_team IS NULL)
$$;
```

Single statement, no `SECURITY DEFINER`, so planner inlines it and `mode = 'all'` folds away on custom plans. Bench task proves list budgets at `all` unchanged and restricted within 50 ms.

## Builder

`app/actions/conversations/visible_scope.go` is entry point. Rule itself lives in `app/auth/authz/conversation_scope.go`, because `app/broadcasting` needs it and `conversations` imports `broadcasting`; `conversations.Scope` is alias. `conversations.Find`, `conversations.Lock` take `Membership` and return `exceptions.ErrNotFound` outside scope.

```go
type Scope struct {
    Mode       enums.ConversationAccess // all, assigned, teams, all_except_teams
    MemberID   *ids.UUID  // nil for workspace-level actors
    TeamIDs    []ids.UUID
    Unassigned bool
    Trash      bool       // holds conversations:delete
}
func VisibleScope(m authz.Membership) Scope                 // no I/O: ResolveWorkspace loads list once per request into Membership.AccessTeams
func authz.WorkspaceScope() Scope                           // jobs, agent, anyone with no member: sees all
func (s Scope) Contains(c models.Conversation) bool         // same logic in Go, table test equals SQL function
func (s Scope) Sees(member, team *ids.UUID) bool            // same, for a holding without a row (before state)
func (s Scope) Params() VisibleParams                       // sqlc args: Mode, Viewer, Teams, Unassigned
func Find(ctx, q *models.Queries, m authz.Membership, ref Ref) (models.Conversation, error)
func Lock(ctx, q *models.Queries, m authz.Membership, ref Ref) (models.Conversation, error)
```

Every new listing query calls function with same four arg names, so sqlc fields match:

```sql
AND dimmi_conversation_visible(c.assignee_member_id, c.assignee_team_id, @access_mode::text,
    sqlc.narg(access_viewer)::uuid, @access_teams::uuid[], @access_unassigned::boolean)
```

Lists built by `app/support/querybuilder` set `Conversations.Access` (from `VisibleScope(m).Params()`); builder always renders the function, zero `Access` shows nothing (fail closed). `Teams` never nil (null array hides everything); `Params()` and the builder guarantee it. Why no ctx and error: list is read when workspace resolves, so scope costs no query and cannot fail mid-action.

Surfaces that must use it (table test `tests/feature/access/scope_test.go` hides one conversation, walks each):

list, every view, custom views, show, neighbors, counts, `/search` (conversation section, number jump, id jump), `POST /conversation-searches`, bulk (at snapshot and again per id), exports (`data_exports` at job run with requester's scope), contact conversations, company conversations, context panel (`past_conversations`, `total_estimate`), mentions view, similar, side conversations, notifications audience and mention delivery (06), realtime subscribe, list fan-out, typing, drafts, attachments show and download, merges, reactions.

Exception, stated once: GDPR contact export and erase work on whole person, so they refuse restricted callers (403 `conversation_access_required`), never run partial. See [safety.md](safety.md).

Outside scope always 404 `not_found`, never 403. Bulk per id: `failed` `not_found`. Search: dropped in phase 2, no hit count leak.

Arch test `tests/arch/visible_scope_test.go`: every sqlc query in `database/queries/` whose name starts `Filter`, `List`, `Count`, `Search`, `Find` and reads `conversations` either calls `dimmi_conversation_visible` or sits on allowlist with reason (inbound routing, jobs, purge).

## Counts

Hidden conversations never counted. `GET /conversation-counts` per member cache key includes `access_version` (`workspace_members.updated_at` epoch) so change takes effect at once. `teams` block lists only teams with visible conversations possible (`all_except_teams` omits excluded). Capped rules unchanged.

## Realtime

Problem: `workspace:{ws}` carries conversation list changes to all members.

Decision: split.

| Channel | Who | Carries |
|---|---|---|
| `workspace:{ws}` | active members | non-conversation events only |
| `workspace_conversations:{ws}` (new namespace; history 300 / 10 min; presence off) | members with `conversation_access = 'all'` | `conversation.*`, list summaries of `message.created`, `conversation.viewers_changed` |
| `member:{mem}` | self | restricted members get per-member copies of list events |

- Single chokepoint: `app/broadcasting/conversation_fanout.go` decides channel for every list event. No feature publishes to these directly.
- Listener `FanOutToRestrictedMembers` (sync after commit): loads workspace's restricted members (Valkey cache keyed by workspace, busted on `member.access_changed`; usually empty, one `GET`). For each: event's conversation before or after state visible. Visible after: publish list payload on `member:{mem}`. Visible before, not after: publish `conversation.hidden {id}` and `node.Unsubscribe(user, "conversation:conv_...")`.
- `conversation:{conv}` authorizer: `Scope.Contains`. Presence and messages follow.
- `member.access_changed`: `node.Disconnect(member)` so every subscription re-authorizes; client reconnects, refetches list and counts. Connection lost under 5 s.
- `conversation.assigned` carries `from`, `to` so before state needs no query.

## API

Owned here, exposed by 00-accounts `PATCH /members/{mem}`, permission `members:manage_access` (ability group `members:write`; owner, admin):

```http
PATCH /api/v1/members/mem_01jb...
{ "conversation_access": "teams", "access_team_ids": ["team_01jb..."], "conversation_access_unassigned": true }
```

- Team ids replace list (max 200). Unknown team 422 `team_not_found`. Mode `assigned` or `all` with non-empty ids: 422 field error.
- Mode `assigned` or `all` with ids is `access_teams_not_used`, over 200 ids `too_many_access_teams`, both 422 on field `access_team_ids`.
- Self change 403 `forbidden`, any role. Why: restricted admin must not lift own limit.
- Owner target 422 `owner_access_unrestricted`.
- Same value no-op, no event. Change writes audit `member.access_changed` `{from, to}`.
- Member resource adds `conversation_access`, `conversation_access_unassigned`, `access_team_ids`: self always; others only with `members:read`.
- Member with restricted access may hold `conversations:delete`, `contacts:erase`; scope still narrows what they reach.

## UI

Controls: members settings (track 07, `resources/js/features/settings/pages/team.tsx`), calling above. BoardUI `Select` for mode, multi-select teams, `Switch` for unassigned. Copy: "Can see all conversations", "Only assigned to them", "Assigned to them and selected teams", "All except selected teams".

Inbox reaction (track 02, `resources/js/features/inbox/`):

- Sidebar hides Unassigned for member lacking it; team items only where reachable.
- Deep link to hidden conversation: 404 page "You do not have access to this conversation", link back to My inbox. Never says if exists.
- Receiving `conversation.hidden` while open: same page, draft kept on server, not shown.
- Mention typeahead marks teammate "Cannot see this conversation" ([collaboration.md](collaboration.md)).

## Events

| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| `member.access_changed` | `member:{mem}` | no | `{from, to}`; triggers disconnect, cache bust |
| `conversation.hidden` | `member:{mem}` | no | `{id}`; ephemeral, not stored |

## Edge cases

[edge-cases.md](edge-cases.md) section X.
