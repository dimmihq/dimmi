# 03 Shared inbox: HTTP API

Style: caveman. Base `/api/v1`. REST rules: `00-foundation/rest.md` (binding). Shapes, pagination, idempotency, errors per `00-foundation/api.md`. Auth: every route `A` (`auth`: browser session, first-party app, or token; abilities apply to tokens only, session and first-party app use member permissions), plus `workspace`, `throttle:api`; POST routes also `idempotent`. Ability column = token ability group; "admin" = caller needs the 03 permission named in README Permissions matrix (`conversations:delete`, `conversations:moderate`, `contacts:erase`, `contacts:manage_external_id`, `companies:delete`, `inboxes:manage`, `tags:manage`, `saved_replies:manage_shared`, and README additions), held by owner/admin via 00-accounts `roles.go`; policies call `authz.Can`, never role names. Every conversation read (list, show, counts, neighbors, search, exports, realtime authorizers) scoped by `conversations.VisibleScope(principal)` ([access.md](access.md)); conversation outside scope = 404. Member references use `mem_` ids (workspace member public id; `usr_` = user, never used for members here, see README Decisions 3); `"me"` accepted wherever member id accepted.

Optional request header on every write: `Dimmi-Client-Mutation-Id: <string up to 64>` echoed in realtime event `meta.client_mutation_id`.

## Endpoint table

### Inboxes
| Method | Path | Route name | Ability / role | Purpose |
|---|---|---|---|---|
| GET | /inboxes | inboxes.index | conversations:read | list (no pagination needed, max 200; still `meta`) |
| POST | /inboxes | inboxes.store | settings:write, `inboxes:manage` | create (also creates `api` connection) |
| GET | /inboxes/{inbox} | inboxes.show | conversations:read | |
| PATCH | /inboxes/{inbox} | inboxes.update | settings:write, `inboxes:manage` | name, default_team_id, settings, `default: true` (make workspace default) |
| DELETE | /inboxes/{inbox} | inboxes.destroy | settings:write, `inboxes:manage` | `?move_to=inbox_...` required when non-empty |

### Conversations
| Method | Path | Route name | Ability / role | Purpose |
|---|---|---|---|---|
| GET | /conversations | conversations.index | conversations:read | list with view, filters, sort, `q` |
| POST | /conversations | conversations.store | conversations:write | create (API channel contact message, or member outbound) |
| PATCH | /conversations | conversations.update-many | conversations:write (per-field rules as single PATCH) | bulk: `{ids, changes}` (max 1,000) or `{filter, changes, confirm_count}` (max 10,000 matches); over 100 runs as job, 202. See Bulk |
| GET | /conversations/{conversation} | conversations.show | conversations:read | `{conversation}` = `conv_…` or number |
| PATCH | /conversations/{conversation} | conversations.update | conversations:write (per-field rules, see Update) | attributes (`subject`, `priority` level, `inbox_id`, `custom_attributes`) and state fields (`status`, `assignee_id`, `team_id`, `spam`, `trashed` both directions, `ai_state`, `add_tags`, `remove_tags`, `unread`) |
| DELETE | /conversations/{conversation} | conversations.destroy | conversations:write, `conversations:delete` | soft delete (trash); same as PATCH `{trashed: true}` |
| POST | /conversations/{conversation}/merges | conversations.merges.store | conversations:write | merge `from: [conv_…]` into this (target) conversation |
| POST | /conversations/{conversation}/tags | conversations.tags.store | conversations:write | add tags (ids or names) |
| DELETE | /conversations/{conversation}/tags/{tag} | conversations.tags.destroy | conversations:write | remove tag |
| POST | /conversations/{conversation}/participants | conversations.participants.store | conversations:write | add `to` or `cc` participant |
| PATCH | /conversations/{conversation}/participants/{contact} | conversations.participants.update | conversations:write | change `role` |
| DELETE | /conversations/{conversation}/participants/{contact} | conversations.participants.destroy | conversations:write | remove participant |
| GET | /conversations/{conversation}/read-state | conversations.read-state.show | conversations:read | my read pointer + `unread` |
| PATCH | /conversations/{conversation}/read-state | conversations.read-state.update | conversations:read | `{last_read_message_id}` (read up to) or `{unread: true}` (mark unread); empty body = read to latest |
| POST | /conversations/{conversation}/typing-indicators | conversations.typing-indicators.store | conversations:write | typing relay to customer side (widget, push channel) for every client; 202, ephemeral, not stored |
| GET | /conversations/{conversation}/neighbors | conversations.neighbors.index | conversations:read | prev/next in same view |
| GET | /conversations/{conversation}/context | conversations.context.show | conversations:read, contacts:read | sidebar data |
| GET | /conversation-counts | conversation-counts.index | conversations:read | sidebar counts + unread |

### Messages
| Method | Path | Route name | Ability / role | Purpose |
|---|---|---|---|---|
| GET | /conversations/{conversation}/messages | conversations.messages.index | conversations:read | timeline, cursor `before`/`after`/`around` |
| GET | /conversations/{conversation}/messages/{message} | conversations.messages.show | conversations:read | |
| POST | /conversations/{conversation}/messages | conversations.messages.store | conversations:write | contact-authored message (only `api` channel conversations) |
| POST | /conversations/{conversation}/replies | conversations.replies.store | conversations:write | customer-visible reply |
| POST | /conversations/{conversation}/notes | conversations.notes.store | conversations:write | internal note |
| PATCH | /conversations/{conversation}/messages/{message} | conversations.messages.update | conversations:write | edit own note (replies not editable) |
| DELETE | /conversations/{conversation}/messages/{message} | conversations.messages.destroy | conversations:write | undo inside window (returns body), delete own, delete others' note or reply or redact (`conversations:moderate`) |
| POST | /conversations/{conversation}/messages/{message}/delivery-attempts | conversations.messages.delivery-attempts.store | conversations:write | retry failed delivery; 202 |

### Attachments
| Method | Path | Route name | Ability / role | Purpose |
|---|---|---|---|---|
| POST | /attachments | attachments.store | conversations:write | multipart upload (`file`), returns `att_` |
| GET | /attachments/{attachment} | attachments.show | conversations:read | metadata + `url` (signed 5 min) |
| DELETE | /attachments/{attachment} | attachments.destroy | conversations:write | only unattached, own upload |
| GET | /files/{attachment}/{filename} (web) | files.show | `signed` | download stream |

### Contacts
| Method | Path | Route name | Ability / role | Purpose |
|---|---|---|---|---|
| GET | /contacts | contacts.index | contacts:read | list, filters, `q` |
| POST | /contacts | contacts.store | contacts:write | create (409 if external_id exists); `?upsert=true` upserts by `external_id` then `email` |
| GET | /contacts/{contact} | contacts.show | contacts:read | merged ids resolve to survivor |
| PATCH | /contacts/{contact} | contacts.update | contacts:write | partial; `custom_attributes` merged by key, null deletes key; `external_id` change or clear: `contacts:manage_external_id`; `blocked` (bool) blocks/unblocks |
| DELETE | /contacts/{contact} | contacts.destroy | contacts:write (`erase=true`: `contacts:erase`) | delete (no conversations) or GDPR erase |
| GET | /contacts/{contact}/conversations | contacts.conversations.index | conversations:read | past conversations, newest first |
| GET | /contacts/{contact}/identities | contacts.identities.index | contacts:read | channels/identities |
| DELETE | /contacts/{contact}/identities/{identity} | contacts.identities.destroy | contacts:write | detach wrong identity (becomes new lead on next inbound) |
| POST | /contacts/{contact}/merges | contacts.merges.store | contacts:write | merge `from: [ctc_…]` into this (target) contact |
| GET | /contacts/{contact}/duplicates | contacts.duplicates.index | contacts:read | suggested duplicates |
| POST | /contacts/{contact}/exports | contacts.exports.store | contacts:write, `contacts:erase` | GDPR export job; 202 |
| GET | /contacts/{contact}/exports/{export} | contacts.exports.show | contacts:write, `contacts:erase` | status + signed url |

### Companies
| Method | Path | Route name | Ability / role | Purpose |
|---|---|---|---|---|
| GET | /companies | companies.index | contacts:read | list, `q`, `filter[tag]`, `filter[audience]=adn_` (18) |
| POST | /companies | companies.store | contacts:write | create; `?upsert=true` upserts by `external_id` then `domain` |
| GET | /companies/{company} | companies.show | contacts:read | |
| PATCH | /companies/{company} | companies.update | contacts:write | |
| DELETE | /companies/{company} | companies.destroy | contacts:write, `companies:delete` | soft delete; contacts `company_id` set null |
| GET | /companies/{company}/contacts | companies.contacts.index | contacts:read | |

### Tags, saved replies, search
| Method | Path | Route name | Ability / role | Purpose |
|---|---|---|---|---|
| GET | /tags | tags.index | conversations:read | all tags (`q` prefix filter), `include=counts` |
| POST | /tags | tags.store | conversations:write | create |
| PATCH | /tags/{tag} | tags.update | settings:write, `tags:manage` | rename/recolor |
| DELETE | /tags/{tag} | tags.destroy | settings:write, `tags:manage` | delete (removes from conversations) |
| GET | /saved-replies | saved-replies.index | conversations:read | shared + own personal; `q` |
| POST | /saved-replies | saved-replies.store | conversations:write (shared: settings:write, `saved_replies:manage_shared`) | |
| GET | /saved-replies/{saved_reply} | saved-replies.show | conversations:read | |
| PATCH | /saved-replies/{saved_reply} | saved-replies.update | owner of personal, or `saved_replies:manage_shared` for shared | |
| DELETE | /saved-replies/{saved_reply} | saved-replies.destroy | same | |
| POST | /saved-replies/{saved_reply}/renderings | saved-replies.renderings.store | conversations:read | render variables for conversation (computed, not stored; rest.md rule 11) |
| GET | /search | search.index | conversations:read (+ contacts:read for contacts/companies sections) | Cmd+K |

## Conversation list

`GET /conversations`

| Param | Values | Notes |
|---|---|---|
| `filter[view]` | `mine` (default), `unassigned`, `all`, `team:<team_id>`, `member:<mem_id>`, `inbox:<inbox_id>`, `cvw_…`, `mentions`, `spam`, `deleted` | spam view ignores status filter default; deleted view needs `conversations:delete`; custom view: definition ANDed with other params (views.md) |
| `filter[status]` | `open` (default), `snoozed`, `closed`, comma list, `any` | |
| `filter[priority]` | levels comma list `none,low,medium,high,urgent` | `true`/`false` 400 |
| `filter[tag]` | tag ids comma list (AND semantics: has all) | max 5 |
| `filter[tag_any]` | tag ids comma list (OR) | max 20 |
| `filter[attr.<key>]` | value | defined conversation attribute; max 3 |
| `filter[assignment_waiting]` | `true` | waiting for capacity (assignment.md) |
| `filter[channel]` | channel types comma list (`email,telegram`) | resolved to connection ids |
| `filter[channel_connection]` | `chn_` ids | |
| `filter[inbox]` | `inbox_` ids | |
| `filter[contact]` | `ctc_` id | |
| `filter[company]` | `cmp_` id | |
| `filter[assignee]` | `mem_`, `me`, `none` | overrides view member part |
| `filter[team]` | `team_`, `none` | |
| `filter[unread]` | `true` | unread for current member |
| `filter[created_after]`, `filter[created_before]` | ISO 8601 | |
| `filter[waiting]` | `true` | `waiting_since IS NOT NULL` (needs reply) |
| `filter[imported]` | `true`/`false` | `source.imported` present (16 importers) |
| `filter[import_id]` | `imp_` id | conversations written by that import |
| `filter[trashed]` | `only` | rest.md rule 7 form; same as `filter[view]=deleted` (needs `conversations:delete`). `with` 400 (trash never mixed into views) |
| `q` | search query (search.md) | switches default sort to `relevance` |
| `sort` | `-last_message_at` (default), `last_message_at`, `waiting_since` (longest first), `-created_at`, `created_at`, `snoozed_until` (snoozed view default), `relevance` (only with q), `-priority` (highest first, then `-last_message_at`), `-priority,waiting_since`, `attr:<key>`, `-attr:<key>` | |
| `include` | `contact`, `assignee`, `team`, `tags`, `last_message`, `inbox`, `unread`, `csat`, `feedback_requests`, `viewers`, `company`, `custom_attributes` | list default includes `contact,assignee,tags,last_message,unread` compact. `csat` (15) and `feedback_requests` (14) batch-loaded per page (one query each) |
| `limit`, `cursor` | | default 25 max 100 |

Unknown filter 400 `invalid_filter`; unknown sort 400 `invalid_sort`; tampered cursor 400 `invalid_cursor`; cursor from different sort/filter set 400 `cursor_mismatch` (cursor embeds hash of filter+sort).

Example `GET /api/v1/conversations?filter[view]=mine&filter[status]=open&sort=waiting_since&limit=2`

```json
{
  "data": [
    {
      "id": "conv_01jbd0k3a8x9t2m4q5r6s7v8w9",
      "object": "conversation",
      "number": 1042,
      "subject": "Card declined on upgrade",
      "status": "open",
      "spam": false,
      "priority": "high",
      "unread": true,
      "waiting_since": "2026-11-01T09:12:44Z",
      "last_message_at": "2026-11-01T09:12:44Z",
      "last_activity_at": "2026-11-01T09:14:02Z",
      "snoozed_until": null,
      "channel": { "id": "chn_01jb…", "object": "channel_connection", "type": "widget", "name": "Website" },
      "inbox": { "id": "inbox_01jb…", "object": "inbox", "name": "Support" },
      "contact": { "id": "ctc_01jb…", "object": "contact", "name": "Bob Lee", "email": "bob@acme.dev", "avatar_url": null, "kind": "user" },
      "assignee": { "id": "mem_01jb…", "object": "member", "name": "Ada", "avatar_url": "https://…" },
      "team": null,
      "tags": [ { "id": "tag_01jb…", "object": "tag", "name": "billing", "color": "#f59e0b" } ],
      "last_message": { "id": "msg_01jb…", "object": "message", "type": "message", "author": { "type": "contact", "name": "Bob Lee" }, "excerpt": "Tried twice, Visa ending 4242 still declined…", "created_at": "2026-11-01T09:12:44Z", "attachments_count": 1 },
      "ai_state": "handed_off",
      "updated_at": "2026-11-01T09:14:02Z",
      "links": { "self": "https://app.trydimmi.com/api/v1/conversations/conv_01jbd0k3a8x9t2m4q5r6s7v8w9" }
    }
  ],
  "meta": { "next_cursor": "eyJ…", "prev_cursor": null, "per_page": 2 }
}
```

## Get conversation

`GET /conversations/{id}?include=messages,context` returns full resource plus `messages` (latest 50 page, ascending, with `meta.messages.prev_cursor`) and `context` (same as context endpoint). `ETag` header; `If-None-Match` 304.

Extra includes owned by other specs (query functions they provide, 03 serializes):
- `include=csat` (15): latest completed CSAT `{response_id, score, scale, comment_excerpt (140 chars), completed_at}` or `null`.
- `include=feedback_requests` (14, `feedback.RequestsForConversation`): `[{id, post: {id, number, title, status}}]` compact, for inbox side panel without extra round trip.

`source.announcement_id` (15) as `announcement: {id, title}`. `source.imported` (16) as `imported: {import_id, source, external_id, external_number, external_url}` (list rows and full resource; `null` for live conversations). Imported conversation unread: never (imported messages excluded, README column semantics).

Full resource adds: `created_at`, `closed_at`, `first_reply_at`, `first_customer_message_at`, `last_customer_message_at`, `message_count`, `source`, `participants`, `previous_conversation` (compact or null), `merged_into` (compact or null), `deleted_at`, `is_test`, `external_thread_key` (members only).

Deleted conversation: callers without `conversations:delete` 404 `not_found`; with it 200 with `deleted_at` set (trash). Outside `VisibleScope`: 404. Merged source: 200 with `merged_into`; writes return 409 `conversation_merged`.

## Create conversation

`POST /conversations`

Contact-authored (developer relays customer message, API channel):
```json
{
  "inbox_id": "inbox_01jb…",
  "contact": { "external_id": "user_42", "email": "bob@acme.dev", "name": "Bob Lee" },
  "author": "contact",
  "subject": "Export failing",
  "body": "My CSV export spins forever.",
  "attachment_ids": ["att_01jb…"],
  "source": { "page_url": "https://app.acme.dev/export", "app_version": "2.4.1" },
  "tags": ["bug"],
  "assignee_id": null
}
```
Member-authored outbound 1:1:
```json
{
  "channel_connection_id": "chn_01jb…email",
  "contact": { "id": "ctc_01jb…" },
  "author": "member",
  "subject": "Following up on your trial",
  "body": "Hi {{contact.first_name | fallback: \"there\"}}, …",
  "then": "snooze",
  "snooze": { "preset": "one_week" }
}
```
Rules:
- `contact`: `id`, or `external_id`/`email` (upsert semantics, see Contacts "Upsert"). Required.
- `author = contact`: requires `inbox_id` (uses inbox `api` connection) or `channel_connection_id` of type `api`. Message `author_type = contact`, `substantive = true`, conversation `waiting_since` set.
- `author = member` (default when omitted and token owner is member): `channel_connection_id` required; channel `StartConversation` must be `always` or `with_template` (+ `channel_options.template`); else 422 `channel_cannot_initiate`. Body variables rendered (same engine as saved replies). Auto-assign to author unless `assignee_id` given (`null` = explicitly unassigned).
- `conversation.created` event carries `initiated_by`: `contact` (author contact), `member` (author member, session or member token), `agent` (Dimmi Agent proactive), `api` (integration token or OAuth client without member), `import` (reserved; 16 import helpers dispatch no `conversation.created`). Workspace-initiated for 12 fair-use = `member|agent|api`.
- Member-authored create runs same billing gate as reply (423 shape above); contact-authored never gated.
- Response 201 conversation resource (`include=messages` implied first page).
- Idempotency: `Idempotency-Key` recommended; duplicates without key create duplicates (documented).

## Update conversation (attributes + state fields)

`PATCH /conversations/{id}`. Partial: only sent keys change. One request may change several fields. `ConversationController.Update` validates (`UpdateConversationRequest`), checks per-field policy, then calls action `ApplyConversationChanges`, which locks row once (`SELECT … FOR UPDATE`) and hands each changed field to its business action in fixed order, all in one tx, one response (conventions "Routes" field-dispatch). Header `If-Unmodified-Since` optional; stale 409 `stale_resource`.

### Fields

| Field | Values | Side params | Action (business name) | Event(s) | Role / ability |
|---|---|---|---|---|---|
| `trashed` | `true` trash, `false` restore (409 `thread_conflict` when thread key held) | none | `DeleteConversation` / `RestoreConversation` | `conversation.deleted` / `conversation.restored` | `conversations:delete`; `conversations:write` |
| `spam` | bool | `block_contact` (bool, only with `spam: true`) | `MarkConversationSpam` / `UnmarkConversationSpam` (+ `BlockContact`) | `conversation.marked_spam` / `conversation.unmarked_spam` (+ `contact.blocked`) | member; `conversations:write` (+ `contacts:write` when `block_contact: true`) |
| `assignee_id`, `team_id` | `mem_…` / `"me"` / `null`; `team_…` / `null` | none | `AssignConversation` (both keys handled together) | one `conversation.assigned` with `from`/`to` for member and team | member; `conversations:write` |
| `status` | `open`, `closed`, `snoozed` | `snoozed_until`, `snooze_preset` or `snooze_option` (exactly one, only with `snoozed`); `note` (only with `closed`) | `CloseConversation`, `ReopenConversation` (from closed), `UnsnoozeConversation` (from snoozed), `SnoozeConversation` | `conversation.closed` / `reopened` (reason `manual`) / `unsnoozed` (reason `manual`) / `snoozed` | member; `conversations:write` |
| `ai_state` | `handed_off`, `active` | none | 07 `PauseConversationAi` (reason `member_paused`) / 07 `ResumeConversationAi` | `ai.state_changed` (07) | member; `conversations:write` |
| `subject`, `priority`, `inbox_id`, `custom_attributes` | 1 to 255 chars; `none\|low\|medium\|high\|urgent`; `inbox_…`; object, null removes key | none | `UpdateConversation` (one call for all) | `conversation.updated` (`changes: {field: [from, to]}` for subject/inbox), `conversation.priority_changed` | member; `conversations:write` |
| `add_tags`, `remove_tags` | ids or names (add; unknown names auto-created); tag ids (remove) | none | `TagConversation` / `UntagConversation` | `conversation.tagged` / `conversation.untagged` | member; `conversations:write` |
| `unread` | bool (viewer field: acting member's read state) | none | `MarkConversationUnread` / `MarkConversationRead` (to latest) | `conversation.read` (`member:{mem}` only) | member; `conversations:write` (read-only tokens use `read-state`) |

`ai_state`: 03 never writes column; field handed to 07 action in same tx (07 owns transitions, rules, errors). `add_tags`/`remove_tags`/`unread` exist so bulk `changes` covers old bulk actions (rest.md rule 8: bulk accepts exactly single-PATCH fields). Child routes `POST /conversations/{id}/tags`, `PATCH …/read-state` stay for single precise ops.

### Order of application (deterministic)

Fields applied in this order inside one tx, regardless of JSON key order:

1. `trashed`
2. `spam` (+ `block_contact`)
3. `assignee_id` / `team_id`
4. `status` (+ `snoozed_until` | `snooze_preset`, `note`)
5. `ai_state`
6. `subject`, `priority`, `inbox_id`
7. `add_tags`, then `remove_tags`
8. `unread`

Why: `trashed` first (sent alone anyway); spam before status so status events already carry final spam flag (06/12 listeners read it); assignment before status so `conversation.closed` carries new assignee and assign-then-close reads as one handover; status before `ai_state` so 07 rules judge final status (pause on just-closed ok; resume on still-closed or spam 409); attributes and tags last (no state rules depend on them).

### Rules

- `trashed` must be only field in body (422 field error `trashed`: "must be sent alone").
- Writes on trashed conversation other than `trashed: false`: 409 `conversation_trashed` (only `conversations:delete` sees trash; others get 404).
- Writes on merged source: 409 `conversation_merged` with `data.merged_into`.
- Permission checked per field (policy `ConversationPolicy.UpdateField`) for every sent field before any work. One forbidden field: whole request 403 (`forbidden`, or `missing_ability` with `required_ability`), nothing changes.
- Atomic: any field failing (validation, transition, conflict) rolls back every field. Error names failing field (`errors.<field>` on 422).
- Idempotent per field: value equal to current state (close closed, spam spam, assign same member, `ai_state` same) is no-op: no action call, no event, no timeline row. All fields no-op: 200, `updated_at` unchanged, `meta.changed: []`.
- One event per changed field (catalog unchanged). Exceptions by design: assignment pair emits one `conversation.assigned`; `subject` + `inbox_id` share one `conversation.updated` with both keys in `changes`.
- Response 200 full conversation resource + `meta.changed` (fields that changed, in application order). Non-fatal notice in `meta.warning` (07 resume after max replies: `max_replies_reached`).
- Unknown field: 422 `unknown_field`. Side param without its field (`snoozed_until` without `status: snoozed`, `block_contact` without `spam: true`, `note` without `status: closed`): 422 field error.

### Status transitions

| From \ To | open | closed | snoozed |
|---|---|---|---|
| open | no-op | close | snooze |
| closed | reopen (reason `manual`; 409 `thread_conflict` when other non-closed conversation holds thread key, `data.conversation` compact of other) | no-op | 422 `invalid_transition` `{from: "closed", to: "snoozed"}` (client sends `status: open` first, or reply `then`) |
| snoozed | unsnooze (reason `manual`) | close (`snoozed_until` cleared, pending job no-ops) | re-snooze (new `snoozed_until`, new `conversation.snoozed`); same `snoozed_until`: no-op |

`invalid_transition` body: `{ "message": "Cannot change status from closed to snoozed.", "code": "invalid_transition", "errors": { "status": ["…"] }, "from": "closed", "to": "snoozed" }`.

Snooze params:
- `snooze_preset`: resolved in actor's timezone (`users.timezone`, fallback workspace timezone; token without member: workspace timezone): `later_today` = now + 3 h rounded up to next 15 min; `tomorrow` = next day 09:00; `next_week` = next Monday 09:00 (from Monday: following Monday); `one_week` = now + 7 days at 09:00.
- `snoozed_until`: at least 1 minute in future (422 `snooze_in_past`), at most 365 days (422 `snooze_too_far`).
- Nonexistent local time (DST gap) shifts forward (02:30 to 03:00); ambiguous (fall back) picks first occurrence.
- `note` with `status: closed`: internal note added in same tx. Spam conversations closable.

`ai_state` transitions (07 rules; 07 `README.md` "HTTP API"): `active|off -> handed_off` (reason `member_paused`, no customer notice); `handed_off -> active` (resume; 409 `ai_not_active` when conversation closed or spam after status step); `off -> active` 422 `invalid_transition` (AI activates only through gate); `off` not settable (422 validation).

Close by member actor with required attribute missing: 422 `required_attributes_missing` `{keys}` (records.md).

Assignment: omitted key unchanged; `null` unassigns that part; `"me"` = actor. Assignee must be active member (422 `member_inactive`). Team must exist in workspace (422 `team_not_found` field error).

Spam: `spam: true` sets flag (status untouched), timeline event, optional contact block (safety.md). Pending outbound messages stay pending (reply sent before spam mark is legitimate); replies created after return 409 `conversation_spam`. `spam: false`: status unchanged.

### Example: close and assign in one call

```http
PATCH /api/v1/conversations/conv_01jbd0k3a8x9t2m4q5r6s7v8w9
{ "assignee_id": "mem_01jb…bob", "status": "closed", "note": "Handing billing follow-up to Bob." }
```
Order: assign (`conversation.assigned` from Ada to Bob), then close (`conversation.closed`, note row). Response 200:
```json
{ "data": { "id": "conv_01jbd0k3a8x9t2m4q5r6s7v8w9", "object": "conversation", "status": "closed", "assignee": { "id": "mem_01jb…bob", "object": "member", "name": "Bob" }, "…": "…" },
  "meta": { "changed": ["assignee_id", "status"] } }
```
Same request repeated: 200, `meta.changed: []`, no events.

## Merge conversations

`POST /conversations/{target}/merges` `{ "from": ["conv_a", "conv_b"] }` (max 10). Target in path; each source merged into it. Moves messages, tags union (cap 50), participants union (source contact becomes `cc` participant of target if different), each source closed with `merged_into_id = target`. Rules:
- Same workspace; target and sources not trashed (404 without `conversations:delete`; 409 `conversation_trashed` with it).
- Target in `from`: 422 `merge_self`. Duplicate ids deduped.
- Target already merged: 409 `conversation_merged` (`data.merged_into`).
- Source already merged into this target: skipped (`skipped_ids`). Source merged into other conversation: 409 `conversation_merged`.
- All-or-nothing: rows locked in UUID order (target + sources), one tx; any source failing aborts whole merge.
- Contacts differ: 422 `merge_different_contacts` unless workspace `inbox.merge_different_customers` true (default false; records.md). Target inbox, status, assignee win.
- Both inside caller's `VisibleScope`, else 404. Irreversible.
- Response 201 (computed result, not stored; event + timeline rows are record):
```json
{ "data": { "object": "conversation_merge", "target": { "id": "conv_…", "object": "conversation", "…": "full" },
  "merged_ids": ["conv_a", "conv_b"], "skipped_ids": [], "moved_messages": 14, "dropped_tags": [] } }
```
- Emits one `conversation.merged` per merged source (`source_id`, `target_id`, moved count; D12 input) + timeline rows in source and target.
- Imported conversations (16) merge like any other; target keeps its channel connection (live wins over `import` when target is imported and source live: 422 `merge_imported_target`, pick live conversation as target).

## Tags, participants, read state, typing

`POST /conversations/{id}/tags` `{ "tags": ["tag_01…", "billing"] }`: ids or names; unknown names auto-created (members allowed). Over 50 tags: 422 `tag_limit_reached`. Already present: skipped silently. Response 200 conversation `tags`.

`DELETE /conversations/{id}/tags/{tag}`: not present: 200 no-op.

`POST /conversations/{id}/participants` `{ "contact": { "email": "cfo@acme.dev", "name": "Ann" }, "role": "cc" }` (`to` or `cc`; contact upsert semantics; 50 max 422 `participant_limit_reached`). Only channels with `ParticipantsCC` accept (else 422 `channel_participants_unsupported`). Reply-level `to`, `cc`, `bcc`, `remove_participants`: below, records.md.

Read state (singleton per member per conversation, `ConversationReadStateController`):
- `GET /conversations/{id}/read-state` returns `{ "data": { "object": "conversation_read_state", "conversation_id": "conv_…", "last_read_message_id": "msg_…", "last_read_at": "…", "unread": false } }` for acting member.
- `PATCH /conversations/{id}/read-state` body one of: `{ "last_read_message_id": "msg_…" }` (read up to message; moves forward only, older id no-op), `{ "unread": true }` (mark unread), `{ "unread": false }` or `{}` (read to latest). Both keys: 422. Message from other conversation: 422 `message_not_in_conversation`. Response 200 read-state resource.
- Mark unread: sets `last_read_message_id` to message before latest non-own message (so exactly latest shows unread) and `last_read_at` to that message's `created_at`. Only own messages: no-op.
- Emits `conversation.read` to `member:{mem}` only (catalog: member only). When read moves past latest contact-visible workspace message on widget conversation, also emits widget-safe `conversation.seen_by_team` on `widget_contact:{ctc}` (README Events; 01 consumes). No-op: no event.

Typing: `POST /conversations/{id}/typing-indicators` `{ "state": "start"|"stop", "composer": "reply"|"note" }` (defaults `start`, `reply`). 202 `{ "data": { "object": "typing_indicator", "state": "start", "expires_at": "…" } }`. Only typing path for every client (dashboard, app, API, MCP; no client publications). Ephemeral, never stored, never published to teammates. Throttle and relay per README Collision; `composer: note`: 202 no-op, never relayed.

## Neighbors

`GET /conversations/{id}/neighbors?filter[view]=mine&filter[status]=open&sort=-last_message_at`

```json
{ "data": { "object": "conversation_neighbors",
  "prev": { "id": "conv_…", "object": "conversation", "number": 1041 },
  "next": { "id": "conv_…", "object": "conversation", "number": 1039 } } }
```
Current conversation no longer matches view (just closed): neighbors computed from its last sort key position anyway (so `j` after close goes to next item). End of list: `null`.

## Context

`GET /conversations/{id}/context`
```json
{ "data": {
  "object": "conversation_context",
  "contact": { "id": "ctc_…", "object": "contact", "kind": "user", "name": "Bob Lee", "email": "bob@acme.dev", "email_verified": true,
    "external_id": "user_42", "phone": null, "locale": "en-US", "timezone": "America/New_York",
    "location": { "country": "US", "region": "NY", "city": "New York" },
    "custom_attributes": { "plan": "pro", "mrr_cents": 4900 },
    "first_seen_at": "2026-02-01T10:00:00Z", "last_seen_at": "2026-11-01T09:12:40Z", "blocked": false },
  "company": { "id": "cmp_…", "object": "company", "name": "Acme", "domain": "acme.dev", "custom_attributes": { "seats": 12 } },
  "identities": [ { "type": "email", "value": "bob@acme.dev", "verified": true }, { "type": "telegram_user", "value": "8812", "verified": true } ],
  "source": { "page_url": "https://app.acme.dev/billing", "browser": "Chrome 141", "os": "macOS 15", "device": "desktop", "app_version": "2.4.1", "ip_country": "US" },
  "past_conversations": { "data": [ { "id": "conv_…", "object": "conversation", "number": 977, "subject": "Invoice VAT", "status": "closed", "last_message_at": "2026-09-02T…", "channel": { "type": "email" } } ], "total_estimate": 7 },
  "duplicates_count": 1
} }
```
`past_conversations` = 3 newest of same contact excluding current, within `VisibleScope`, (panel expands to 20 through `GET /contacts/{id}/conversations`), deleted excluded, `total_estimate` capped at 100. AI sidebar block (agent state, citations, pending agent action calls) comes from 07 `GET /conversations/{conversation}/ai`, not from context.

## Bulk

`PATCH /conversations` (rest.md rule 8). `changes` accepts exactly single-PATCH fields and side params (Update above), validated once up front (unknown field or bad value: 422, nothing runs). Per-field permission checked once for whole request before any work (member sending `trashed`: 403, nothing runs). Each conversation then runs `ApplyConversationChanges` in own tx (same order, same rules, same events as single PATCH). Partial success allowed, never silent.

By ids (`inbox.bulk.max_ids` 1,000; 1,001: 422 `bulk_too_many`; duplicates deduped; 101 or more: job, see below):
```json
{ "ids": ["conv_a", "conv_b", "conv_c", "conv_d"], "changes": { "status": "snoozed", "snooze_preset": "tomorrow" } }
```
Response 200:
```json
{ "data": [
    { "id": "conv_a", "status": "updated" },
    { "id": "conv_b", "status": "failed", "error": { "code": "invalid_transition", "message": "Cannot change status from closed to snoozed.", "from": "closed", "to": "snoozed" } },
    { "id": "conv_c", "status": "failed", "error": { "code": "not_found", "message": "Conversation not found." } },
    { "id": "conv_d", "status": "skipped" } ],
  "meta": { "updated": 1, "skipped": 1, "failed": 2 } }
```
`skipped` = every field no-op for that conversation (already in requested state). Other workspace or trashed (member) ids: `failed` `not_found`.

By filter (allowed for conversations, capped, explicit count confirmation):
```json
{ "filter": { "view": "unassigned", "status": "open", "channel": "widget", "created_after": "2026-11-01T00:00:00Z" },
  "changes": { "spam": true },
  "confirm_count": 412 }
```
- `filter` keys = `GET /conversations` `filter[…]` keys (same allowlist and validation, 400 `invalid_filter`). `q` not allowed (ranking unstable).
- Two-step confirmation: request without `confirm_count` changes nothing, returns 422 `confirmation_required` with `meta.matched`. Client shows "Mark 412 conversations as spam?" and resends with `confirm_count`. Server re-counts inside request: differs from `confirm_count`: 409 `bulk_count_mismatch` with `meta.matched`, nothing changed.
- Cap: `inbox.bulk.max_filter_matches` (default 10,000). More matches: 422 `bulk_too_many` with `meta.matched`.
- Over `inbox.bulk.sync_max` (100) matched or ids: 202 `{ "data": { "id": "blk_…", "object": "bulk_operation", "status": "pending", "matched": 412, "updated": 0, "skipped": 0, "failed": 0 } }`. Progress `GET /bulk-operations/{id}`, `bulk_operation.updated`. Workbench.md "Bulk actions".
- Snapshot: ids selected once at start (keyset order of filter); each processed per own tx with current-state validation (conversation closed meanwhile: `skipped` or `failed` per rules). New matches arriving mid-run not included.
- Response 200 meta only (rule 8): `{ "meta": { "matched": 412, "updated": 409, "skipped": 2, "failed": 1, "errors": [ { "id": "conv_…", "code": "conversation_merged" } ] } }`; `errors` lists every failed id (max 1,000, never silent).
- Bulk by filter audit-logged (`WriteAuditLog`, filter + changes + counts), like bulk trash.

Events emitted per conversation (catalog), workspace channel coalesced by foundation. Bulk reply and note: `POST /bulk-operations` (workbench.md).

## Counts

`GET /conversation-counts`
```json
{ "data": { "object": "conversation_counts",
  "views": {
    "mine": { "open": 12, "unread": 3, "snoozed": 4 },
    "members": { "mem_01…": { "open": 7 } },
    "views": { "cvw_01…": { "open": 18 } },
    "unassigned": { "open": 41, "unread": 41 },
    "all": { "open": 999, "capped": true },
    "mentions": { "unread": 1 },
    "spam": { "total": 7 },
    "teams": { "team_01…": { "open": 18 } },
    "inboxes": { "inbox_01…": { "open": 230 } } },
  "computed_at": "2026-11-01T09:15:00Z" } }
```
Unread computed only for `mine`, `unassigned`, `mentions` (bounded sets). Others open counts only, capped at 999. Hidden conversations never counted; teams and members only as reachable by caller's scope. `views`: pinned only.

## Messages

### List timeline
`GET /conversations/{id}/messages?limit=50` (latest page). `?before=<cursor>` older, `?after=<cursor>` newer, `?around=msg_…` (25 before + target + 24 after, `meta.anchor`). Order always ascending by `(created_at, id)` in response. Filters: `filter[type]=message,note,event`, `filter[updated_since]=ts` (resync: returns messages created or edited or deleted since, ascending by `updated_at`, max 500, `meta.truncated`). `after` queries rescan 10 s overlap before cursor; clients dedupe by `id` (protects against late-committed concurrent inserts).

Message resource:
```json
{
  "id": "msg_01jb…", "object": "message", "conversation_id": "conv_…",
  "type": "message",
  "author": { "type": "member", "id": "mem_…", "name": "Ada", "avatar_url": "https://…" },
  "body": "Fixed. Try again?",
  "body_html": null,
  "attachments": [ { "id": "att_…", "object": "attachment", "filename": "receipt.pdf", "content_type": "application/pdf", "size_bytes": 48211, "url": "https://app.trydimmi.com/files/att_…/receipt.pdf?expires=…&signature=…", "inline": false, "width": null, "height": null, "scan_status": "skipped" } ],
  "channel": { "id": "chn_…", "type": "email" },
  "delivery_status": "sent", "delivery_error": null,
  "substantive": true,
  "mentions": [],
  "client_message_id": "c3f0e7c2-…",
  "edited_at": null, "deleted_at": null, "sent_at": "2026-11-01T09:20:01Z", "read_at": null,
  "event": null,
  "created_at": "2026-11-01T09:20:00Z", "updated_at": "2026-11-01T09:20:01Z"
}
```
`body_html` only with `include=body_html` (email, sanitized). Deleted messages: `body: null`, `attachments: []`, `deleted_at` set. Deleted member author: `{"type":"member","id":null,"name":"Former teammate"}`. Imported message (16): `imported: true`; author from `metadata.import.author`: mapped member renders as member, `former_teammate`/`bot`/`unknown` render `{"object":"former_teammate","type":"member","id":null,"name":"Kim (Intercom)"}` (bot: `type agent`, no link).

### Reply
`POST /conversations/{id}/replies`
```json
{
  "body": "Hi Bob, fixed now. Can you retry?",
  "attachment_ids": ["att_01jb…"],
  "client_message_id": "c3f0e7c2-5d0a-4a43-9a1e-2f2f8c9d1e10",
  "expected_last_message_id": "msg_01jb…",
  "then": "close",
  "snooze": null,
  "channel_options": null,
  "to": [],
  "cc": [{ "email": "cfo@acme.dev" }],
  "bcc": [],
  "remove_participants": [],
  "delay": true
}
```
- Added fields: `to`, `cc`, `bcc`, `remove_participants` (records.md), `delay` (undo window, workbench.md), `original` (translation, collaboration.md). `snooze` adds `{option}`. Success deletes caller's matching draft in same tx.
- Body keys strict: unknown key 422 `unknown_field` (same as PATCH). MCP/CLI sugar `reopen` / `close_after` is not a reply field: tools translate to `then` (09). 03 never emits `conversation_closed`: reply on closed conversation follows `then` (default `keep`).
- Order inside reply tx: (1) lock conversation; (2) rehome when connection driver implements `ChannelRehomer` (16 `import`: moves conversation to inbox's live email connection, `conversation.updated`; 422 `import_channel_cannot_send` when inbox has no email connection or contact has no email); (3) collision guard, validation vs (new) connection, insert message, dispatch `message.created` (billing gate); (4) auto-assign, `then`. Any failure rolls back all. No `draft_id`: reply consumes draft by conversation + mode. Send modes stay 09.
- `body` 1 to 64 KB Markdown (required unless attachments present). NFC normalized. Saved reply variables already rendered by client via render endpoint; server renders any remaining `{{…}}` with same engine (unknown blank).
- `client_message_id` (UUID string): dedupe within conversation per author; repeat returns 200 with existing message (not 201) and header `Idempotent-Replayed: true`.
- `expected_last_message_id`: collision guard (README). Mismatch 409 `new_messages_since`:
```json
{ "message": "New messages arrived since you started typing.", "code": "new_messages_since",
  "data": [ { "id": "msg_…", "object": "message", "author": { "type": "member", "name": "Ada" }, "excerpt": "Refunded, closing.", "created_at": "…" } ] }
```
- `then`: `keep` (default), `open`, `close`, `snooze` (requires `snooze` object `{preset}`, `{until}` or `{option}`, same rules as PATCH). Closed conversation + `keep`: stays closed, message sent. Snoozed + `keep`: wakes, reason `member_reply`, when `snooze.wake_on_reply` true. `open` on closed: reopen reason `reply`. `snooze` on closed: 422 `invalid_transition` (whole reply rolled back).
- Response 201 `{ "data": message, "meta": { "conversation": compact } }`.
- Side effects in tx: auto-assign (inbox `assign_on_reply`, unassigned member, actor member), clear `waiting_since`, `first_reply_at`, mark read for author, typing stop relay.
- Billing gate (cloud): `events.Dispatch(message.created)` runs 12 InTx listener `MeterMessageActivity` (contracts.md "Event dispatch phases") inside reply tx. Activation blocked: listener returns `ErrActivationBlocked{Reason}`, whole tx rolls back (no message, no assign, no `then`), 12 records pause via `tx.AfterRollback`. Controller maps to 423 with 12's `billing` block:
```json
{ "message": "Workspace reached its monthly spend cap. …", "code": "spend_cap_reached",
  "billing": { "state": "paused", "spend_cap_cents": 5000, "manage_url": "https://app.trydimmi.com/settings/billing", "can_manage": false } }
```
  Other block reasons same shape: `payment_method_required`, `billing_restricted`, `billing_locked`, `test_mode_limit_reached`. Notes, contact messages, inbound never gated. Follow-up inside open billing window not activation, never blocked. Self-host: listener no-op.

### Note
`POST /conversations/{id}/notes` `{ "body": "Looping <@mem_01jb…> and <@team_01jb…>", "attachment_ids": [], "client_message_id": "…" }`. Mentions parsed server-side; invalid ids ignored; valid ones written to `message_mentions` (team expanded to members) in same tx, ids carried on `message.created.MentionedMemberIDs`. Response 201. No status change, no assign, no outbound, `substantive = false`. Allowed on closed and spam conversations.

### Contact message (API channel)
`POST /conversations/{id}/messages` `{ "body": "…", "attachment_ids": [], "dedupe_key": "myapp:evt_991" }`. Only when conversation channel type `api` (else 409 `channel_not_api`). Runs `ReceiveInboundMessage` path (reopen/unsnooze rules apply). `dedupe_key` optional, prefixed server-side `api:`.

### Note edit, delete, delivery attempts
`PATCH /conversations/{c}/messages/{m}` `{ "body": "…" }`:
- Own note only, any time. Replies, contact messages, events: 422 `message_not_editable` (reply edit: later).
- Emits `message.updated`. Sets `edited_at`, `metadata.edited_by`. Note edit re-parses mentions: added members get `message_mentions` rows, `message.updated.AddedMentionedMemberIDs` set (06 notifies only those); removed rows deleted, no unmention notification.

`DELETE /conversations/{c}/messages/{m}`:
- Own reply inside undo window (`scheduled_send_at` in future): undo, 200 with `undone: true`, `body` (workbench.md).
- Own reply: any time. Others' reply: `conversations:moderate`. Still `pending`: cancels send. Sent: deleted locally (widget hides it from visitor); push channel copy (email, chat apps) stays, no provider call (channel delete sync: later).
- Every delete clears body, html, attachments, runs `MessageEraser` hooks in job.
- Own note: any time. Others' notes: `conversations:moderate`.
- Contact message: `conversations:moderate` only, local redaction (`metadata.redacted_by`), never touches channel.
- Event rows: 422 `message_not_deletable`.
- Emits `message.deleted`; counters/preview recomputed.

`POST /conversations/{c}/messages/{m}/delivery-attempts` (empty body): failed only (409 `message_not_failed`), re-validation, sets message `pending`, inserts send job. 202 job-like resource (not stored; River job is record):
```json
{ "data": { "object": "delivery_attempt", "message_id": "msg_…", "status": "pending", "queued_at": "…" },
  "meta": { "message": { "id": "msg_…", "object": "message", "delivery_status": "pending", "…": "…" } } }
```

## Attachments

`POST /attachments` multipart `file` (+ optional `inline=true`). Limits: 25 MB (413 `payload_too_large`), blocked types (executables) 422 `attachment_type_blocked`. Scan states and download rules: safety.md. Real type sniffed. Response 201:
```json
{ "data": { "id": "att_01jb…", "object": "attachment", "filename": "receipt.pdf", "content_type": "application/pdf", "size_bytes": 48211, "sha256": "…", "scan_status": "skipped", "url": "https://…signed", "created_at": "…" } }
```
Linking: `attachment_ids` in reply/note/conversation create. Rules: same workspace; unattached (`message_id`, `feedback_post_id`, `feedback_comment_id` all null; one-target CHECK); uploaded by same actor (`conversations:moderate` may link any unattached); max 10. Already attached: 409 `attachment_already_attached`. Infected: 422 `attachment_infected`. `LinkAttachments` also serves 14 targets `feedback_post` / `feedback_comment` (caller passes ownership, limits, image-only check). Orphan purge treats any of the three target columns as attached.

## Contacts

`GET /contacts` filters: `filter[tag]`, `filter[kind]=user|lead`, `filter[email]`, `filter[external_id]`, `filter[company]`, `filter[blocked]=true` (default false: blocked hidden unless asked), `filter[last_seen_after]`, `filter[last_seen_before]`, `filter[created_after]`, `filter[custom.<key>]=<value>` (equality, max 3, JSON containment), `filter[audience]=adn_...` (18 SQL compiler builds predicate; statement timeout 5 s, over: 422 `audience_too_slow`; unknown or other workspace audience: 400 `invalid_filter`), `q`. Sort: `-last_seen_at` (default), `-created_at`, `name`. Excludes merged and deleted.

### Upsert (developer identify)
`POST /contacts?upsert=true` (rest.md rule 10). Same controller as create (`ContactController.Store`), calls action `UpsertIdentifiedContact` (02 owns separate `identity.IdentifyContact`). Match keys in order: `external_id`, then `email`. Neither given: 422 `upsert_key_required`. `upsert` accepts `true`/`false` only (else 400 `invalid_parameter`). Without `upsert=true`: plain create, existing `external_id` 409 `contact_exists`.
```json
{
  "external_id": "user_42",
  "email": "bob@acme.dev",
  "name": "Bob Lee",
  "phone": "+14155550100",
  "custom_attributes": { "plan": "pro", "signed_up_at": "2026-01-03T00:00:00Z" },
  "company": { "external_id": "org_7", "name": "Acme", "domain": "acme.dev" }
}
```
Algorithm:
1. `external_id` given: match contact by `(workspace_id, external_id)`. Found: update. 
2. Not found and `email` given: candidates = contacts with same email, `external_id IS NULL`, not merged, not deleted, AND (`kind = user` OR `email_verified = true`). Pick kind `user` first, then newest `last_seen_at`. Found: set `external_id` (if given), `kind = user`, `email_verified = true`, update fields. Unverified leads with same email never chosen, never merged (security rule); returned in `meta.possible_duplicates` for manual merge.
3. None: create `kind = user` (when external_id) or `kind = lead` with `email_verified = true` (email only from trusted API).
4. Identities upserted: `external_id`, `email` (verified). Verified identity owned by other contact: auto-merge rules (channel-contract.md section 4).
5. Written `custom_attributes` keys added to `contacts.trusted_attribute_keys` (trusted server; `null` removes key). Same for `PATCH /contacts/{id}` by member or token.
Response 201 (created) or 200 (updated), body contact, `meta.upserted: "created"|"updated"`. Fields omitted: unchanged; `null`: cleared. Upsert never changes existing `external_id` (409 `external_id_immutable`); only PATCH with `contacts:manage_external_id` can change or clear it. `blocked` not accepted on upsert (use PATCH).

Companies: `POST /companies?upsert=true`, action `IdentifyCompany`, match keys `external_id`, then `domain` (normalized lowercase, no `www.`). Neither given: 422 `upsert_key_required`. Same 201/200 + `meta.upserted`.

### Block
`PATCH /contacts/{id}` `{ "blocked": true }` / `{ "blocked": false }`. Field-dispatch in `ContactController.Update`: plain attributes first (`UpdateContact`, `contact.updated`), then `blocked` (`BlockContact` / `UnblockContact`, `contact.blocked` / `contact.unblocked`), one tx. Same value as current: no-op, no event. Effects: edge-cases.md 124.

### Merge
`POST /contacts/{target}/merges` `{ "from": ["ctc_a", "ctc_b"] }` (max 10). Target in path. Rules: both verified with different identity values: 422 `merge_different_contacts` unless `inbox.merge_different_customers` (records.md); different non-null `external_id` on target and source: 409 `merge_conflict_external_id`. Target in list: 422 `merge_self`. Source already merged into target: skipped. Source merged elsewhere or target merged: 409 `contact_merged` naming survivor. All-or-nothing per request (rows locked in UUID order). Response 201 (one `contact_merges` row per source written; not exposed by id at MVP):
```json
{ "data": { "object": "contact_merge", "target": { "id": "ctc_…", "object": "contact", "…": "full" }, "merged_ids": ["ctc_a"], "skipped_ids": ["ctc_b"] } }
```
`trusted_attribute_keys`: survivor keeps own list; attribute copied from source (survivor lacked key) keeps source's trust; survivor value wins otherwise (07 binding rule). Over 50k conversations to move: 202 `{ "data": { "object": "contact_merge", "status": "pending", "target": { "id": "ctc_…" } } }`, job finishes, `contact.merged` on completion. See edge cases for field precedence and widget identity drop.

### Export
`POST /contacts/{id}/exports` (`contacts:erase` + `conversation_access = all`, else 403 `conversation_access_required`) body `{include_notes?}`: 202 `{ "data": { "id": "exp_…", "object": "contact_export", "status": "pending" } }`. One running per contact: 409 `export_in_progress`. `GET /contacts/{id}/exports/{export}`: status, signed `url` when `ready` (24 h). `contact.export_ready` to requester `member:{mem}`. Zip sections: core (`contact`, `identities`, `conversations`, `messages`, attachments) plus one per registered export hook (`changelog` 13, `feedback` 14, `engage` 15; README Security).

### Delete / erase
`DELETE /contacts/{id}`: contact with zero conversations: soft delete, 204. Has conversations: 409 `contact_has_conversations` (use `?erase=true`).
`DELETE /contacts/{id}?erase=true[&immediate=true]` (`contacts:erase` + `conversation_access = all`; immediate also needs `password.confirm`): 202 `{ "data": { "id": "ces_…", "object": "contact_erasure", "status": "pending", "scheduled_for": "…" } }`; contact immediately hidden, `erase_contact` job wipes after grace (default 7 days, safety.md). Cancel: `PATCH /contact-erasures/{ces} {status: "canceled"}` inside grace. Audit logged. Job calls `RetentionGuard` first: held = status `pending_hold` (nothing wiped, contact stays hidden; resumes when hold released). Status values: `pending`, `pending_hold`, `completed`, `canceled`.

## Added endpoints

Details in owning file. Route names follow rest.md rule 14. Auth `A` unless stated.

| Method | Path | Ability, permission | File |
|---|---|---|---|
| GET, POST | /conversation-views | conversations:read; conversations:write (`views:manage_shared` for shared) | views.md |
| GET, PATCH, DELETE | /conversation-views/{view} | same | views.md |
| PATCH | /conversation-views | `views:manage_shared` (`order`) | views.md |
| GET, PATCH | /inbox-preferences | conversations:read | views.md |
| POST | /conversation-searches | conversations:read | search.md |
| GET | /snooze-lists, /snooze-options | conversations:read | workbench.md |
| PATCH | /snooze-lists/{scope} | settings:write, `inboxes:manage` | workbench.md |
| GET, POST (`?upsert=true`) | /drafts | conversations:read; conversations:write | workbench.md |
| PATCH, DELETE | /drafts/{draft} | conversations:write | workbench.md |
| POST | /bulk-operations | conversations:write | workbench.md |
| GET, PATCH | /bulk-operations/{bulk_operation} | conversations:read; conversations:write | workbench.md |
| GET | /inboxes/{inbox}/assignment-status | settings:read | assignment.md |
| GET, POST, PATCH | /away-reasons | conversations:read; settings:write, `inboxes:manage` | assignment.md |
| PATCH, DELETE | /away-reasons/{away_reason} | settings:write, `inboxes:manage` | assignment.md |
| POST | /member-activity | `U` + workspace; 204 | assignment.md |
| GET | /conversations/{conversation}/mentionables | conversations:read | collaboration.md |
| POST | /conversations/{conversation}/messages/{message}/reactions | conversations:write | collaboration.md |
| DELETE | /conversations/{conversation}/messages/{message}/reactions/{emoji} | conversations:write | collaboration.md |
| GET, POST | /conversations/{conversation}/side-conversations | conversations:read; `conversations:side` | collaboration.md |
| GET, PATCH | /conversations/{conversation}/side-conversations/{side_conversation} | same | collaboration.md |
| POST | /conversations/{conversation}/side-conversations/{side_conversation}/messages | `conversations:side` | collaboration.md |
| GET | /conversations/{conversation}/similar | conversations:read | collaboration.md |
| POST | /conversations/{conversation}/messages/{message}/translations | conversations:read | collaboration.md |
| POST | /conversations/{conversation}/reply-translations | conversations:write | collaboration.md |
| GET, POST | /conversation-attribute-definitions | attributes:read; attributes:write, `attributes:manage` | records.md |
| PATCH, DELETE | /conversation-attribute-definitions/{conversation_attribute_definition} | attributes:write, `attributes:manage` | records.md |
| GET | /companies/{company}/conversations | conversations:read | records.md |
| POST | /attachments/{attachment}/download-grants | conversations:read | safety.md |
| GET, POST, PATCH | /redaction-rules | settings:read; settings:write, `redaction:manage` | safety.md |
| PATCH, DELETE | /redaction-rules/{redaction_rule} | settings:write, `redaction:manage` | safety.md |
| POST | /redaction-tests | settings:write, `redaction:manage` | safety.md |
| GET | /contact-erasures, /contact-erasures/{contact_erasure} | contacts:write, `contacts:erase` | safety.md |
| PATCH | /contact-erasures/{contact_erasure} | contacts:write, `contacts:erase` | safety.md |
| GET, POST | /data-exports | conversations:read, `conversations:export` or `contacts:export` | safety.md |
| GET, DELETE | /data-exports/{data_export} | same; requester | safety.md |
| GET, POST | /export-destinations | exports:read; exports:write | safety.md |
| PATCH, DELETE | /export-destinations/{export_destination} | exports:write | safety.md |
| POST | /export-destinations/{export_destination}/tests | exports:write | safety.md |
| GET, POST | /export-schedules | exports:read; exports:write | safety.md |
| PATCH, DELETE | /export-schedules/{export_schedule} | exports:write | safety.md |

`PATCH /members/{mem}`, `PATCH /members/me` (00-accounts routes) gain `conversation_access*` (access.md), `availability`, `away_reassign`, `away_reason_id`, `assign_on_reply`, `undo_send_seconds`, `conversation_capacity`, `capacity_by_channel` (assignment.md, workbench.md).

`PATCH /contacts/{id}`, `PATCH /companies/{id}` gain `add_tags`, `remove_tags`, `notes`. `POST /conversations` member-authored gains `to`, `cc`, `bcc`, `delay`.

## Saved replies

Resource: `{ id: "rpl_…", object: "saved_reply", title, body, shortcut, scope: "personal"|"shared", owner: compact member|null, variables: ["contact.first_name"], created_at, updated_at }`.
Create `{ "title": "Refund", "shortcut": "refund", "body": "Hi {{contact.first_name | fallback: \"there\"}}, …", "scope": "personal" }`. `shortcut` 1 to 32 chars `[a-z0-9_-]`, unique per owner scope (personal vs shared may share shortcut; personal wins in palette).

Render `POST /saved-replies/{id}/renderings` `{ "conversation_id": "conv_…" }` (200, computed, not stored):
```json
{ "data": { "object": "rendered_saved_reply", "body": "Hi Bob, …", "missing_variables": ["contact.custom.plan"] } }
```
Variables allowlist: `contact.name`, `contact.first_name`, `contact.last_name`, `contact.email`, `contact.phone`, `contact.external_id`, `contact.custom.<key>`, `company.name`, `company.custom.<key>`, `member.name`, `member.first_name`, `member.email`, `conversation.number`, `conversation.subject`, `workspace.name`. Filter: `| fallback: "text"` only. Unknown or empty: fallback or blank. Never error. `first_name` = first whitespace token of name (CJK names: whole name).

## Search
See [search.md](search.md). `GET /search?q=refund&limit=5`:
```json
{ "data": { "object": "search_results",
  "jump": null,
  "conversations": [ { "conversation": { "id": "conv_…", "object": "conversation", "number": 977, "subject": "Refund for duplicate charge", "status": "closed" },
    "highlight": "…asked for a <mark>refund</mark> after the duplicate…", "message_id": "msg_…", "score": 0.82 } ],
  "contacts": [ { "id": "ctc_…", "object": "contact", "name": "Refund Bot", "email": null } ],
  "companies": [] },
  "meta": { "search": { "capped": false, "fuzzy": false } } }
```

## Error codes (03-specific)

| Status | Code | When |
|---|---|---|
| 400 | invalid_filter, invalid_sort, invalid_cursor, cursor_mismatch, invalid_include, invalid_parameter | query params (`upsert` not `true`/`false`) |
| 404 | not_found | any id from other workspace, outside `VisibleScope`, trashed without `conversations:delete` |
| 403 | forbidden, missing_ability | per-field PATCH permission (`trashed` needs `conversations:delete`; `block_contact` needs `contacts:write`); whole request rejected |
| 409 | conversation_merged | write on merged source (`data.merged_into`); merge target already merged |
| 409 | conversation_trashed | write (caller with `conversations:delete`) on trashed conversation; merge with trashed conversation |
| 409 | conversation_spam | reply on spam conversation |
| 409 | thread_conflict | reopen (`status: open` from closed) while other conversation holds thread |
| 409 | bulk_count_mismatch | filter bulk: matches differ from `confirm_count` (`meta.matched`) |
| 409 | ai_not_active | `ai_state: active` on closed or spam conversation (07) |
| 409 | new_messages_since | collision guard |
| 409 | attachment_scan_pending, attachment_scan_warning | download before scan or without grant (safety.md) |
| 403 | conversation_access_required | contact export or erase by restricted member |
| 410 | attachment_infected | download of removed file |
| 409 | channel_window_closed | messaging window over (`data.window_closed_at`) |
| 409 | channel_disconnected | routed connection disconnected and no fallback |
| 409 | channel_not_api | contact message on non-api conversation |
| 409 | message_not_failed | delivery attempt on message not `failed` |
| 409 | attachment_already_attached | |
| 409 | stale_resource | `If-Unmodified-Since` |
| 409 | cannot_delete_default_inbox, inbox_not_empty | |
| 409 | contact_has_conversations | delete without erase |
| 409 | merge_conflict_external_id, external_id_immutable, contact_exists, contact_merged | contacts |
| 409 | export_in_progress | |
| 422 | validation errors (Laravel shape) incl. field codes below | |
| 422 | channel_cannot_initiate, contact_unreachable_on_channel, channel_participants_unsupported | |
| 422 | import_channel_cannot_send | reply on imported conversation (16) when inbox has no email connection or contact has no email |
| 422 | merge_imported_target | merge live conversation into imported target |
| 422 | invalid_transition | illegal `status` / `ai_state` change; body `from`, `to` (rest.md rule 3) |
| 422 | audience_too_slow | `GET /contacts?filter[audience]` over 5 s statement timeout (18) |
| 422 | unknown_field, confirmation_required, upsert_key_required, message_not_in_conversation | PATCH unknown key; filter bulk without `confirm_count` (`meta.matched`); upsert without match key; read-state message of other conversation |
| 422 | merge_different_contacts, required_attributes_missing, attribute_unknown, snooze_option_unknown, invalid_definition, view_limit_reached, away_reason_required, away_reason_limit_reached, owner_access_unrestricted, attribute_definition_limit_reached, redaction_rule_limit_reached, reaction_limit_reached, message_not_reactable, export_too_large, translation_unavailable | this round |
| 422 | snooze_in_past, snooze_too_far, member_inactive, tag_limit_reached, participant_limit_reached, bulk_too_many, merge_self, attachment_infected, attachment_type_blocked, message_not_editable, message_not_deletable, search_query_too_long | |
| 423 | spend_cap_reached, payment_method_required, billing_restricted, billing_locked, test_mode_limit_reached | 12 InTx listener `MeterMessageActivity` aborts reply / member-authored create tx (cloud); body carries `billing` block. Inbound always captured |
| 429 | outbound_fair_use_exceeded | 12 InTx guard on `conversation.created` with workspace `initiated_by` (cloud) |
| 502 | translation_failed | LLM provider error (07) |
| 503 | search_timeout, list_timeout | search or attribute sort statement timeout |
