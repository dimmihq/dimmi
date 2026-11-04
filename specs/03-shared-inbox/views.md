# 03 Shared inbox: views, list, sort, priority

Style: caveman. Gaps I-01, I-02, I-03, I-04, I-05, I-06, I-07, I-62. Track 03 inbox-workbench, except I-05, I-06 in track 12 inbox-extras. All lists go through `VisibleScope` ([access.md](access.md)).

## Sidebar and views (I-01)

Route stays `/d/inbox/$view`. `$view` values:

| `$view` | `filter[view]` | Where |
|---|---|---|
| `mine`, `unassigned`, `all`, `mentions` | same | fixed items, `resources/js/features/shell/navigation.ts` |
| `snoozed` | `mine` + `filter[status]=snoozed` | fixed item (builds on F31 fix: timer wakes) |
| `spam`, `deleted` | same | fixed items under "More"; `deleted` only with `conversations:delete` |
| `team_01...` | `team:<id>` | Teams section |
| `mem_01...` | `member:<id>` | Teammates section (collapsed folder) |
| `inbox_01...` | `inbox:<id>` | Inboxes section, shown when workspace has 2 or more |
| `cvw_01...` | `<cvw id>` | Views section |

- API: `filter[view]` accepts all above. Today's query reads `member` plus separate id; code moves to `member:<id>` form per api.md.
- New module `resources/js/features/inbox/views/view-route.ts` maps `$view` to list params, one function, unit tested. `navigation.ts` keeps static items; dynamic sections from `resources/js/features/inbox/views/use-sidebar-views.ts` (`/teams`, `/members`, `/inboxes`, `/conversation-views`, `/conversation-counts`).
- Counts: `GET /conversation-counts` adds `members: {mem: {open}}`, `views: {cvw: {open}}` (pinned views only, capped, cache 30 s). Team, member item counts open only.
- Team assignment on screen: `resources/js/features/inbox/details-panel.tsx` gets team `Select` beside member. Command menu and `A` open combined picker (teammates and teams). Assigning team keeps member unless cleared.
- Restricted member: sections filtered ([access.md](access.md)).

## Status filter (I-02)

`SegmentedControl` above list: Open, Snoozed, Closed, All. Sets `filter[status]`, kept in URL `?status=`, saved per view in `member_inbox_preferences`. Search no longer implicit status switch: search keeps chosen status except number and id jumps. Snoozed rows show wake time; closed rows closed time.

## Sort (I-03)

Control beside status (`Dropdown`). Saved per view.

| Label | `sort` | Rule |
|---|---|---|
| Latest activity (default) | `-last_message_at` | |
| Oldest activity | `last_message_at` | |
| Waiting longest | `waiting_since` | nulls last; keyset on `(waiting_since, id)`. Needs `waiting_since` column |
| Newest started, oldest started | `-created_at`, `created_at` | |
| Priority | `-priority` | then `-last_message_at` |
| Priority, waiting longest | `-priority,waiting_since` | how queues are worked |
| Snooze ends | `snoozed_until` | snoozed status only, else 400 `invalid_sort` |
| Custom attribute | `attr:<key>`, `-attr:<key>` | definition must exist, sortable types (I-63, track 12); unindexed, statement timeout 2 s then 503 `list_timeout` |
| Relevance | `relevance` | only with `q` ([search.md](search.md)) |

SLA sorts belong to 17 (adds value when SLAs ship; registry `conversation_sort`).

`waiting_since` semantics already in README. Migration adds column and backfills: open or snoozed, `last_customer_message_at > coalesce(last_reply_at, 'epoch')` sets `last_customer_message_at`. Close clears. Row shows "Waiting 2 h".

## Priority levels (I-62)

Five levels: `none`, `low`, `medium`, `high`, `urgent`. Replaces boolean. Nothing launched, so plain migration, no compatibility shim.

`20261201030012_change_conversations_priority_to_level.sql`:

```sql
ALTER TABLE conversations ALTER COLUMN priority DROP DEFAULT;
ALTER TABLE conversations ALTER COLUMN priority TYPE smallint
  USING (CASE WHEN priority THEN 3 ELSE 0 END);
ALTER TABLE conversations ALTER COLUMN priority SET DEFAULT 0;
ALTER TABLE conversations ADD CONSTRAINT conversations_priority_check CHECK (priority BETWEEN 0 AND 4);
```

Old true becomes `high`. smallint so sort is plain index order. Marked contract phase with reason "pre-launch".

- `enums.ConversationPriority`: string to int in one place. Int never crosses API.
- Resource: `"priority": "high"`. PATCH `{"priority": "urgent"}`; `true`/`false` 422 validation. Same in bulk, create, reply none.
- `filter[priority]=high,urgent` (comma list); `true`/`false` gone: 400 `invalid_filter`.
- Index `conversations_priority_idx`: `(workspace_id, status, priority desc, last_message_at desc, id desc) WHERE deleted_at IS NULL AND NOT spam AND priority > 0`.
- Event `conversation.priority_changed` `{from, to}` strings.
- Search: `is:priority` = any level above none. New `priority:high`, `priority:>=high`.
- Channels never see priority. Dashboard: `case.ts` drops `priority` bool, `P` opens menu, `1`..`5` in menu. Chip on row and header, BoardUI `Chip` with tokens: urgent `text-text-error-primary`, none hidden.
- Aligned elsewhere by coordinator (delta): 17 catalog bool, 10 payload example.

## Custom views (I-04)

### conversation_views (`cvw_`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `cvw_` |
| workspace_id | uuid not null | fk cascade, RLS |
| name | text not null | 1 to 60 |
| icon | text null | remix icon key from fixed list |
| scope | text not null CHECK in ('shared','personal') | |
| owner_member_id | uuid null | fk `workspace_members` cascade; required when personal, null when shared. CHECK |
| definition | jsonb not null | below |
| sort | text null | any list sort |
| status_filter | text null | open, snoozed, closed, any |
| created_by_member_id | uuid null | fk set null |
| position | int not null default 0 | shared order |
| created_at, updated_at, deleted_at | timestamptz | soft delete rest rule 7 |

Unique `(workspace_id, scope, owner_member_id, name) NULLS NOT DISTINCT WHERE deleted_at IS NULL`. Index `(workspace_id, scope, owner_member_id)`.

Limits fixed product: 200 shared, 50 personal per member; 422 `view_limit_reached`.

### Definition grammar

Shared with `POST /conversation-searches` (P-04).

```json
{ "op": "and", "groups": [
  { "op": "or", "conditions": [
    { "field": "tag", "op": "in", "value": ["tag_01..."] },
    { "field": "priority", "op": "gte", "value": "high" } ] },
  { "op": "and", "conditions": [
    { "field": "waiting_since", "op": "older_than", "value": "PT2H" } ] } ] }
```

Depth 2 only (Intercom-like And/Or groups). Max 5 groups, 20 conditions.

| Field | Ops |
|---|---|
| `status`, `channel`, `channel_connection`, `inbox`, `assignee`, `team`, `tag`, `contact`, `company` | `in`, `not_in`, `exists`, `not_exists` (`assignee`, `team`, `company`, `tag`) |
| `priority` | `eq`, `gte`, `lte`, `in` |
| `unread`, `waiting`, `imported` | `eq` bool |
| `number`, `message_count` | `eq`, `gt`, `lt` |
| `created_at`, `last_message_at`, `waiting_since`, `snoozed_until`, `closed_at` | `before`, `after`, `older_than` (ISO 8601 duration), `newer_than` |
| `subject` | `contains` (trigram) |
| `text` | `matches` (search language; FTS phase, only at top level group, max 1) |
| `attr.<key>` | by type: `eq`, `in`, `gt`, `lt`, `contains`, `exists` |
| `contact.attr.<key>`, `company.attr.<key>` | `eq`, `exists` (GIN containment) |

`me` allowed in `assignee`. Compiler `app/support/querybuilder/conversation_definition.go`: validates against registry, emits parameterized fragment, only place besides `conversations.go` that builds SQL. Unknown field 422 `invalid_definition` with path. Reference to deleted tag, team, inbox: condition stays, matches nothing; resource `warnings: [{path, code: "reference_missing"}]`, editor flags row.

### API

| Method | Path | Ability, permission | Purpose |
|---|---|---|---|
| GET | /conversation-views | conversations:read | shared + own personal; `q` |
| POST | /conversation-views | conversations:write; shared: `views:manage_shared` | create. Duplicate = client posts copy (name "Copy of ...") |
| GET | /conversation-views/{view} | conversations:read | other's personal 404 |
| PATCH | /conversation-views/{view} | owner, or `views:manage_shared` | name, icon, definition, sort, status_filter |
| PATCH | /conversation-views | `views:manage_shared` | `{order: [...]}` shared order (rest rule 9) |
| DELETE | /conversation-views/{view} | same as PATCH | soft delete |

`views:manage_shared`: new permission, ability group `conversations:write`, owner and admin. Pin, hide per member: `member_inbox_preferences`.

Use: `GET /conversations?filter[view]=cvw_...` adds other filters, `q`, cursor. Search inside view = `q`. Results live (query, no materialization). Definition hash in cursor, edit invalidates: 400 `cursor_mismatch`, client refetches.

Events: `conversation_view.created|updated|deleted`, realtime: shared `workspace:{ws}`, personal `member:{mem}`; webhook no.

### UI

`resources/js/features/inbox/views/`: `view-editor.tsx` (Drawer: name, icon, condition rows, group add, live preview list + count), `view-list-section.tsx`. Open from sidebar + or from current filters ("Save as view"). e2e `tests/e2e/inbox-views.spec.ts`.

## Member inbox preferences

`member_inbox_preferences`: `workspace_id`, `member_id` PK, `sidebar jsonb`, `views jsonb` (per view `{sort, status, layout, columns}`), `pinned_attributes jsonb`, `updated_at`. Keyed by parent, no public id. Singleton:

`GET /inbox-preferences`, `PATCH /inbox-preferences` (`conversations:read` both: own preference, not data). Max 64 KB (422). Client saves debounced 1 s. `inbox_preferences.updated` on `member:{mem}` keeps tabs same. Unknown view keys pruned on read when target gone.

## Personal folders and sidebar editing (I-05, track 12)

`sidebar`: `{ "items": [ {"type":"builtin","key":"mine"}, {"type":"folder","id":"f1","name":"Billing","items":[{"type":"team","id":"team_..."},{"type":"view","id":"cvw_..."}]} ], "hidden": ["inbox-all"] }`.

Max 20 folders, 100 items. Drag to reorder via `react-aria-components` `GridList` drag and drop, already a dependency; no new library. Items gone (team deleted, view lost): skipped on render, pruned on next save. Fixed items may be hidden but "My inbox" always stays. File `resources/js/features/inbox/views/sidebar-editor.tsx`. e2e in `inbox-views.spec.ts`.

## Table layout (I-06, track 12)

Toggle `L`, saved per view `layout: list|table`, `columns: [...]`.

- `resources/js/features/inbox/table/`: on BoardUI `components/base/table/table.tsx`; no table library, no virtualization: cursor pages of 25 to 100 rows, "Load more".
- Columns: number, subject, contact, company, assignee, team, inbox, channel, status, priority, tags, waiting since, last activity, created, snooze ends, `attr.<key>`. Add, hide, drag order, click header sorts where sort exists.
- Selecting row: preview pane (read-only `thread.tsx` + composer) via `react-resizable-panels`, existing dependency. `Enter` opens full.
- API: `include=company,team,custom_attributes` batch-loaded.
- Works with bulk selection and J/K. e2e `tests/e2e/inbox-table.spec.ts`.

## List row (I-07)

`resources/js/features/inbox/conversation-list.tsx` row: channel icon (remix by `channel.type`), assignee avatar, priority chip, tags up to 2, waiting time, viewers.

Viewers: up to 3 avatars then `+N`. Hidden when workspace setting `inbox.show_viewers_on_list` false (default true, `workspaces.settings`, registry).

- `include=viewers`: `framework/broadcasting` adds generic `PresenceMany(ctx, channels) map[string][]ClientInfo` (pipelined Valkey). Page max 100 channels. Over 100 ms deadline: `viewers` omitted, never fails list.
- Realtime: join and leave on `conversation:{conv}` namespace hook publish `conversation.viewers_changed {id, viewers: [{id, name, avatar_url}]}` through fan-out chokepoint ([access.md](access.md)), coalesced 1 s per conversation.
- Me excluded on my own screen.

## Migrations (track 03)

`20261201030015_create_conversation_views_table.sql`, `20261201030016_create_member_inbox_preferences_table.sql`, plus renumbered old list migrations; table in [tasks.md](tasks.md).
