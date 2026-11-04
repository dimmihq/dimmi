# 03 Shared inbox

Style: caveman. Date 2026-11-01, revised 2026-11-04 ("everything now" round: UI in scope, gap map below). Read with `specs/00-conventions/README.md` and `specs/00-foundation/*`.

Files in this folder:

| File | What |
|---|---|
| README.md | this: scope, data model, code map, events, jobs, config, permissions, budgets, metering |
| [api.md](api.md) | every endpoint, filters, sorts, request/response examples, error codes |
| [channel-contract.md](channel-contract.md) | `contracts.Channel` interface, inbound `ReceiveInboundMessage` contract, outbound pipeline, delivery status |
| [search.md](search.md) | search query language, SQL shape, ranking, indexes |
| [access.md](access.md) | team-based conversation access, `VisibleScope`, realtime split (S-04) |
| [assignment.md](assignment.md) | round robin, balanced, capacity, away status, inbox settings page (I-10 to I-14, T-01) |
| [views.md](views.md) | sidebar views, status filter, sort, priority levels, custom views, folders, table layout, list row (I-01 to I-07, I-62) |
| [workbench.md](workbench.md) | keyboard, command menu, snooze, composer, drafts, undo send, emoji/GIF, inline images, delete and edit, thread extras, start conversation, bulk (I-20s, I-30s, I-40, I-70, I-110s, I-130) |
| [records.md](records.md) | tags, attributes, customer panel, directory, companies, merges, participants (I-60s, I-71, I-72, I-80, I-90s) |
| [safety.md](safety.md) | malware, attachment policy, expiring links, redaction, link warnings, spam, block, trash, erase, export, retention, exports (S-*, I-120s, I-140s, P-03) |
| [collaboration.md](collaboration.md) | mentions, reactions, side conversations, similar, translation (I-50s, I-37, I-38) |
| [edge-cases.md](edge-cases.md) | decided behavior per edge case |
| [tasks.md](tasks.md) | build checklist |

## Gap map

Every Intercom gap assigned to this spec. Tasks in [tasks.md](tasks.md).

| Gap | Where specified | Milestone |
|---|---|---|
| I-01 | views.md "Sidebar and views" | W2 |
| I-02 | views.md "Status filter" | W2 |
| I-03 | views.md "Sort" | W2 |
| I-04 | views.md "Custom views" | W3 |
| I-05 | views.md "Personal folders" | X3 |
| I-06 | views.md "Table layout" | X3 |
| I-07 | views.md "List row" | W2 |
| I-10 | assignment.md "Modes", "Picker" | A3 |
| I-11 | assignment.md "Away status" | A2 |
| I-12 | assignment.md "Away reasons and automatic away" | X1 |
| I-13 | assignment.md "Modes", "Waiting and explanation", "Member capacity" | A3 |
| I-14 | assignment.md "Per-member self assign" | A2 |
| I-20 | workbench.md "Snooze picker" | W5 |
| I-21 | workbench.md "Snooze lists" | W5 |
| I-22 | workbench.md "Waking a snooze" | W5 |
| I-30 | workbench.md "Send and close" | W6 |
| I-31 | workbench.md "Drafts" | W6 |
| I-32 | workbench.md "Undo send" | W7 |
| I-33 | workbench.md "Formatting" | W6 |
| I-34 | workbench.md "Emoji and GIF" | W8 |
| I-35 | workbench.md "Inline images" | W8 |
| I-36 | workbench.md "Delete reply or note" | W8 |
| I-37 | collaboration.md "Reactions on notes" | W9 |
| I-38 | collaboration.md "Translation" | X5 |
| I-39 | workbench.md "Thread extras" | W8 |
| I-40 | workbench.md "Start conversation" | W10 |
| I-50 | collaboration.md "Mentions" | W9 |
| I-51 | collaboration.md "Side conversations" | X4 |
| I-52 | collaboration.md "Similar conversations" | X4 |
| I-60 | records.md "Conversation tags" | R1 |
| I-61 | records.md "People and company tags" | R1 |
| I-62 | views.md "Priority levels" | W1 |
| I-63 | records.md "Custom attributes" | X2 |
| I-70 | workbench.md "Bulk actions" | W11 |
| I-71 | records.md "Merge conversations" | R5 |
| I-72 | records.md "Merge contacts, duplicates" | R5 |
| I-80 | records.md "Participants" | R6 |
| I-90 | records.md "Customer panel" | R2 |
| I-91 | records.md "Customer panel" | R2 |
| I-92 | records.md "Contact directory" | R3 |
| I-93 | records.md "Companies" | R4 |
| I-100 | search.md "Query language", "Filter screen" | W12 |
| I-101 | search.md "Search everywhere", workbench.md "Command menu" | W4 |
| I-102 | search.md "Sorts" | W12 |
| I-110 | workbench.md "Keyboard" | W4 |
| I-111 | workbench.md "Command menu" | W4 |
| I-112 | workbench.md "Shortcut sheet" | W4 |
| I-120 | safety.md "Conversation export" | R11 |
| I-121 | safety.md "Scheduled export" | X6 |
| I-130 | workbench.md "Seen indicator" (narrowed, X12) | W8 |
| I-140 | safety.md "Spam folder" | R7 |
| I-141 | safety.md "Block" | R7 |
| S-01 | safety.md "Malware scan" | R8 |
| S-02 | safety.md "Attachment policy" | R8 |
| S-03 | safety.md "Expiring links" | R8 |
| S-04 | access.md | A1 |
| S-11 | safety.md "Redaction" | R9 |
| S-12 | safety.md "Link safety" | R9 |
| S-15 | safety.md "Erase and export one customer" | R10 |
| S-16 | safety.md "Retention" | R11 |
| E-05 | records.md "Customer panel" (panel display; API 04) | R2 |
| E-06 | safety.md "Link safety" (strip option) | R9 |
| P-03 | safety.md "Trash and restore" | R7 |
| P-04 | search.md "Structured search" | W12 |
| T-01 | assignment.md "Inbox settings page" | A4 |
| T-03 | 06 README "Settings page and notifications page" | 06 M9 |

## Goal

Core domain of Dimmi: inboxes, conversations, messages, contacts, companies, tags, saved replies, read state, search. Every channel (widget, email, chat apps, API) and Dimmi Agent write into this model through actions defined here. Dashboard, CLI, MCP, webhooks read it through API defined here.

Why: roadmap launch item 3 (`research/roadmap.md` "3. Shared inbox"). Core job. HelpDesk users complain search cannot find old tickets (`research/competitors/helpdesk.md`). Every other MVP spec (01, 02, 04, 05, 06, 07, 09, 10, 12) depends on this one.

## World-class bar (measurable)

| Bar | Target |
|---|---|
| Conversation list API (any built-in view, any sort, 25 rows) | p95 under 50 ms server time at workspace with 100k conversations / 1M messages |
| Conversation open (`GET /conversations/{id}?include=messages,context`) | p95 under 50 ms, one round trip gives timeline page + sidebar |
| Next/previous conversation in view | p95 under 20 ms (`/neighbors`), client prefetches so keyboard `j`/`k` feels instant |
| Write (reply, note, close, assign, snooze, tag) | p95 under 100 ms; realtime event on other teammate screen p95 under 300 ms |
| Search (message text, 1M messages) | p95 under 150 ms; selective queries (under 2,000 hits) under 50 ms; finds 3-year-old conversation by one rare word |
| Search typo tolerance | contact names, emails, subjects match with 1 typo (trigram) |
| Optimistic updates | every write returns full resource + `updated_at`; every realtime event echoes `client_mutation_id` so client drops own echo |
| Collision | teammate sees "Jane is viewing" within 1 s; send blocked with 409 `new_messages_since` if someone replied since composer opened |
| Outbound | reply leaves Dimmi for push channel within 2 s p95 (queue pickup + adapter); failed delivery visible on message with reason and retry button |
| Zero lost messages | inbound dedupe by `dedupe_key`; outbound job inserted in same tx as message; per-conversation send order preserved |
| Unread counts | correct per member per view, pushed over `member:{mem}` under 1 s after change |
| API parity | dashboard uses only endpoints in api.md; everything doable in UI doable with token, CLI, MCP |

## Scope

### In (launch)

UI in scope wherever a gap has a screen. Everything in new files above is launch scope; list below keeps backend summary.

- Inboxes CRUD, default inbox, per-inbox settings (reopen window, assign-on-reply, default team).
- Conversations: list with views (mine, unassigned, all, team, mentions, spam, deleted) and filters (status, priority, tag, channel type, channel connection, inbox, contact, company, assignee, team, unread, created range), sorts (newest activity, oldest, waiting longest, created, priority, snooze ends, custom attribute); custom views; get; create via API (contact-authored on `api` channel, member-authored outbound 1:1 on channel that can initiate); update via one `PATCH` (attributes subject, priority, inbox, plus state fields: status open/closed/snoozed with presets and custom time, assignee member and/or team, spam, trashed, AI state, tags, unread; field-dispatch to business actions in fixed order, one tx); auto-assign on reply; bulk `PATCH /conversations` (sync to 100, job to 1,000 ids or 10,000 filter matches, confirmation for filters); bulk reply and note; soft delete to trash and restore; conversation merge (many sources into target); participants (`to`, `cc`) add/remove, per-send `bcc`.
- State machine exactly as `00-foundation/data-model.md`, reopen window rule, conversation numbering.
- Messages: reply, internal note with `@member` / `@team` mentions, contact message via API (API channel only), note edit, delete, timeline cursor both directions + `around`, attachments upload and linking, retry failed delivery, collision guard `expected_last_message_id`.
- `contracts.Channel` interface, outbound pipeline (`SendChannelOutbound` listener, `SendChannelMessage` job), delivery status updates, inbound contract (`ReceiveInboundMessage` action) used by 01, 04, 05.
- Read state per member, unread counts per view, `conversation-counts` endpoint.
- Collision detection: centrifuge presence on `conversation:{conv}` (who is viewing, Valkey presence). Typing via REST `typing-indicators` for every client (dashboard, app, API, MCP) relayed to customer side only (widget, push channels with `Typing`).
- Contacts CRUD, list with filters, search, identify/upsert by `external_id` or `email`, identities, block/unblock, duplicates suggestion, manual merge, auto-merge on verified identity, GDPR export + erase.
- Companies CRUD + upsert (`external_id` then `domain`), contacts of company.
- Tags CRUD, apply/remove (by id or name, auto-create).
- Saved replies CRUD (personal + shared), variables rendering with fallbacks.
- Global search (Cmd+K): conversations by message text + subject, contacts, companies; operators (`from:`, `is:`, `tag:`, `assignee:`, `#1042`), Postgres FTS + trigram.
- Sidebar context endpoint: contact attributes, company, identities (channels), source page, browser, OS, past conversations.
- Metering: `substantive` flag rules, `is_test` flag (03 owns `conversations.is_test`), event fields billing listeners (12) need, reply billing gate (12 InTx listener, 423).
- MCP tools for conversations, contacts, search, tags, saved replies.
- Timeline helpers for other specs (`inbox.RecordTimelineEvent`, `inbox.SetMessageMetadataKey`; 07 agent actions, 12).
- Import write helpers for 16 importers (`app/actions/conversations/importing/`: no events, no outbound, no billing), optional `contracts.ChannelRehomer` (reply on imported conversation moves it to live email connection), `imported` resource block, filters and search operator, unread rule for imported messages.

### Out (later)

Rows here need things unbuilt and no assigned gap implies them. Everything else moved in.

| Item | Why stays out |
|---|---|
| Macros (multi-action) | post-MVP: ticket types, custom fields, views, macros. Saved replies and macros excluded from gap round |
| Ticket types, back-office tickets | same set; custom conversation attributes moved in (I-63), ticket types did not |
| Rules beyond assignment: auto-tag, auto-close, SLA timers, office hours | owned by 17 workflows. Round robin, balanced, capacity moved in (assignment.md) |
| Agent assist: draft, summarize | 07 later. Translation moved in (I-38) |
| Agent send modes `drafts_only`/`direct` over REST, MCP, CLI | 09 (`agent-send-modes`). MCP reply still confirms in client. Composer drafts moved in (I-31) |
| Reply via different channel than routing picks (manual channel switch) | post-MVP; no gap |
| Semantic (vector) search over conversations | post-MVP; FTS + trigram only. Similar conversations uses FTS |
| Followers / watching conversations | post-MVP; mentions view covers it |
| Reports beyond counts | post-MVP: reports |
| later: message-edit | customer-visible reply edit, edit window, edit propagation (`ChannelEditor`, `apply_channel_edit`). Notes editable, replies delete only |
| later: message-edit (channel sync) | outbound delete propagation (`ChannelDeleter`), inbound customer edits and deletes from channels. Delete is local; channel copy stays |
| later: teammate-typing | "Jane is typing" between teammates. Presence, list viewers (I-07), 409 guard cover collisions |
| later: teammate-echo (X2 of 05) | teammate native reply inside Slack or Discord thread |

CSAT ships at launch in 15 (system message via `SendSystemMessage`, `include=csat` here). Importers and `imported` messages: launch: importers (16-importers, in; helpers below).

## User stories

Workspace member:
1. See my open conversations, newest activity first, unread bold, and move with `j`/`k` without spinner.
2. Reply, add note mentioning `@Ada`, send and close in one action (`then: "close"`).
3. Snooze "tomorrow" (09:00 my timezone); conversation returns to inbox when time hits or customer writes.
4. See Ada is viewing same conversation; get blocked from sending duplicate answer when she replied first.
5. Assign to teammate or team; replying to unassigned conversation assigns it to me.
6. Find conversation from 2023 by typing one word from customer's message.
7. See contact attributes, plan, page they wrote from, browser, past conversations in sidebar.
8. Merge duplicate contact (same person on email and Telegram) and duplicate conversation.
9. Mark spam; spam never notifies, never bills.
10. Use saved reply `/refund` with `{{contact.first_name | fallback: "there"}}`.
10a. Work my team's queue oldest first, from a team view, by keyboard only; `Cmd+K` reaches every move.
10b. Leave a half written answer, come back on another device, find it (server draft).
10c. Select 3,000 spam conversations matching a filter; job finishes while I keep working.
10d. Take five seconds to undo an email I just sent.
10e. Set myself away; customer replies go to the team, not sit on me.

Team lead:
10f. Round robin new conversations among my team, capped at 10 open each, and see why one waits.
10g. Limit a contractor to conversations of one team.
10h. Answer a privacy request: export, then erase one customer, recoverable for 7 days.

End customer:
11. Write on any channel; reply reaches me on channel I last wrote from.
12. Write again after conversation closed: same conversation reopens (within 30 days) so I do not repeat myself.

Developer (API):
13. `POST /contacts?upsert=true` with `external_id` from my app; idempotent upsert.
14. Create conversation from my own in-app form on `api` channel; receive replies by webhook (10).
15. Filter and paginate conversations, stable cursors, idempotent POSTs.

AI agent (MCP / Dimmi Agent):
16. Dimmi Agent (07) replies through same action; its acknowledgement-only messages flagged non-substantive.
17. Claude Code via MCP lists unassigned conversations, proposes reply, sends only after human confirms (09 confirm gate).

## Data model

Core tables (inboxes, conversations, messages, contacts, contact_identities, companies, attachments, tags, conversation_tag, conversation_reads, conversation_participants, saved_replies) defined in `00-foundation/data-model.md`. Not redefined here. Behavior defined here.

Doc rules on core tables (`00-foundation/extensibility.md` sections 1 P1, 10):
- `contacts.company_id` = **primary company**. JWT `company` claim (02) and `POST /companies?upsert=true` + contact `company` field set primary. Code never assumes contact has exactly one company: metering (12), webhook payloads (10) and filters read `company_id` as "primary", resource field name stays `company`. Later `contact_companies (workspace_id, contact_id, company_id, primary)` + resource `companies[]` additive.
- `contact_identities.type` and `channel_connections.type` = registry enums (P1): `text` + format CHECK `^[a-z][a-z0-9_]{1,39}$`, value validated against Go registry (`enums.IdentityType`, `enums.ChannelType` backed by `app/support/registry`) in request rule and action. MVP identity registry: `email`, `phone`, `widget_visitor`, `external_id` (03, 01, 02) + channel specs register theirs at boot (05: `slack_user`, `discord_user`, `telegram_user`, `whatsapp_wa_id`, `whatsapp_bsuid`). Channel type registry adds `import` (16 driver; channel connection lists hide it unless `filter[type]=import`, 05 CRUD). New type = new Go registration, never CHECK migration. Unknown value from DB (driver removed) renders but cannot be written.

### Typed JSON shapes on core tables (no schema change)

`inboxes.settings` (`models.InboxSettings`):
```json
{ "reopen_window_days": 30, "assign_on_reply": true, "color": "#6366f1" }
```
- `reopen_window_days` int 0 to 365. 0 = closed conversations never reopen; customer message always starts new conversation.
- `assign_on_reply` bool default true.

`workspaces.settings` keys owned by 03 (`models.WorkspaceSettings`): `default_inbox_id` (uuid), `seen_receipt_shows_name` (bool, default false; widget "Seen" shows replying member name + avatar, see `conversation.seen_by_team`).

`workspaces.settings` keys added this round (registry, delta): `inbox.merge_different_customers`, `inbox.undo_send_seconds`, `inbox.team_mentions_enabled`, `inbox.show_viewers_on_list`, `inbox.gif_picker`, `inbox.translation_enabled`, `snooze.wake_*`, `availability.*`, `attachments.customer_uploads`, `attachments.allowed_extra_extensions`, `redaction.card_numbers`, `redaction.us_ssn`, `link_safety.*`, `retention.*`. Meaning in owning file.

`conversations.source` (`models.ConversationSource`), written once at creation by channel adapter:
```json
{ "page_url": "https://app.acme.dev/billing", "page_title": "Billing", "referrer": "https://google.com",
  "user_agent": "Mozilla/5.0 ...", "browser": "Chrome 141", "os": "macOS 15", "device": "desktop",
  "app_version": "2.4.1", "ip_country": "DE", "locale": "de-DE", "timezone": "Europe/Berlin" }
```
`browser`, `os`, `device` parsed server-side from `user_agent` at ingest (`framework/http/useragent` on medama-io/go-useragent: trie, zero allocs; replaces uap-core regexes). IP itself never stored in `source`.

Other documented `source` keys: widget keys (01 README), `announcement_id` (15: reply to chat-format announcement), `imported` (`{import_id, source, external_id, external_number, external_url}`, 16; drives resource `imported` block).

`messages.metadata` keys owned by 03: `mentions` (array of `{type: "member"|"team", id}`; render only, as typed; consumers read `message_mentions`), `client_message_id`, `edited_by`, `redacted_by` (contact message deleted by admin), `merged_from_conversation_id`, `email` subset written by 04, `ai` written by 07 (07-owned subtree; card status via `inbox.SetMessageMetadataKey`), `survey_request` (`{survey_id, response_id}`, 15 via `SendSystemMessage` allowlist), `widget` (`{session_id, identity_verified}`, 01 stamp), `imported` (bool) + `import` (`{import_id, source, external_id, author: {kind: member|former_teammate|bot|unknown, name}}`, 16 helpers only).

`messages.event` for `type = event` (system timeline rows), names reuse catalog: `conversation.assigned`, `conversation.closed`, `conversation.reopened`, `conversation.snoozed`, `conversation.unsnoozed`, `conversation.priority_changed`, `conversation.tagged`, `conversation.untagged`, `conversation.marked_spam`, `conversation.unmarked_spam`, `conversation.merged`, `conversation.participants_changed`, `conversation.restored`, `conversation.assignment_waiting`, `conversation.bcc_sent`, `side_conversation.replied`, `contact.merged`, `announcement.replied` (15: first message of conversation started from announcement), `ai.action_call_completed`, `ai.action_call_approval_requested` (07 agent actions via `inbox.RecordTimelineEvent`). Shape `{"name": "...", "actor": {"type","id"}, "from": ..., "to": ..., "reason": ...}`. Event rows do not change `last_message_at`, `last_activity_at`, `message_count`, unread.

### New tables

#### message_mentions
Canonical mention store. Drives "Mentions" view and mention notifications (06 reads this table, not metadata). Written in note create/edit tx.

| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid not null | fk workspaces cascade |
| message_id | uuid not null | fk messages cascade |
| conversation_id | uuid not null | fk conversations cascade |
| workspace_member_id | uuid not null | fk workspace_members cascade; team mention expanded to members at write time |
| via_team_id | uuid null | fk teams set null; set when expanded from `@team` |
| created_at | timestamptz not null default now() | |

PK `(message_id, workspace_member_id)`. Index `(workspace_id, workspace_member_id, created_at desc, conversation_id)`.

#### contact_merges
Append-only log of merges. Used for audit, undo investigation, D12/D15 evidence, contact id redirect.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | |
| workspace_id | uuid not null | |
| source_contact_id | uuid not null | tombstone row (kept, `merged_into_id` set) |
| target_contact_id | uuid not null | |
| reason | text not null CHECK in ('manual','verified_email','verified_external_id','verified_phone','identify_api','widget_identify') | `widget_identify`: 02 lead-to-user on login |
| actor_type | text not null CHECK in ('member','agent','system','integration') | `Actor.Type` (contracts.md); token/OAuth/CLI/MCP never a type |
| actor_id | uuid null | |
| actor_via | jsonb | `Actor.Via` `{kind, id}` (personal_token, oauth_client, mcp ...) |
| moved | jsonb not null default '{}' | counts: conversations, identities, participants; dropped identities list (types only) |
| created_at | timestamptz not null default now() | |

Index `(workspace_id, target_contact_id, created_at desc)`, `(workspace_id, source_contact_id)`.

#### contact_exports
GDPR export jobs.

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | public `exp_` |
| workspace_id | uuid not null | |
| contact_id | uuid not null | fk contacts cascade |
| requested_by_member_id | uuid null | fk workspace_members set null |
| status | text not null CHECK in ('pending','running','ready','failed','expired') default 'pending' | |
| disk | text | |
| path | text | zip path |
| size_bytes | bigint | |
| error | text | |
| expires_at | timestamptz | ready + 24 h |
| created_at, updated_at | timestamptz | |

Index `(workspace_id, contact_id, created_at desc)`. Partial unique `(workspace_id, contact_id) WHERE status IN ('pending','running')` (one export at time).

#### Tables added this round

Columns in owning file. Track numbers per [tasks.md](tasks.md).

| Table | Prefix | File |
|---|---|---|
| member_access_teams | none | access.md |
| team_assignment_cursors (moved from 17) | none | assignment.md |
| away_reasons | `awr_` | assignment.md |
| conversation_views | `cvw_` | views.md |
| member_inbox_preferences | none | views.md |
| snooze_lists | none | workbench.md |
| conversation_drafts | `drf_` | workbench.md |
| bulk_operations | `blk_` | workbench.md |
| contact_tag, company_tag | none | records.md |
| conversation_attribute_definitions | `cad_` | records.md |
| redaction_rules | `rxr_` | safety.md |
| contact_erasures | `ces_` | safety.md |
| data_exports | `dex_` | safety.md |
| export_destinations, export_schedules | `dxd_`, `dxs_` | safety.md |
| message_reactions | none | collaboration.md |
| side_conversations, side_messages | `sdc_`, `sdm_` | collaboration.md |
| message_translations | none | collaboration.md |

Core column changes this round: `conversations.priority` boolean to smallint level (views.md); `workspace_members` access, availability, capacity (access.md, assignment.md); `conversations.assignment_wait_reason`, `assignment_waiting_since`, `custom_attributes`; `messages.scheduled_send_at`, `undone_at`; `conversation_participants.role` values `to`, `cc`, `removed_at`; `conversation_tag.message_id`; `contacts.notes`, `companies.notes`; `attachments.scan_status` values + `scan_signature`, `scanned_at`, `side_message_id`.

### Core table changes

All in migrations owned by this spec. Expand-only (no drops in MVP except duplicate foundation indexes, dropped in later contract migration after bench proves replacements).

| Migration | Change | Why |
|---|---|---|
| `20261101000301_create_search_configuration.sql` | `CREATE EXTENSION IF NOT EXISTS btree_gin`; `CREATE TEXT SEARCH CONFIGURATION dimmi_search (COPY = english)`; `ALTER MAPPING FOR hword, hword_part, word WITH unaccent, english_stem`; function `dimmi_unaccent(text)` IMMUTABLE wrapper over `public.unaccent('public.unaccent', $1)` | `unaccent()` is STABLE, cannot be used in generated column or index directly; text search config with `unaccent` dictionary can (`to_tsvector(regconfig, text)` immutable). `btree_gin` lets GIN index lead with `workspace_id` (convention) |
| `20261101000302_alter_messages_search_column.sql` | redefine generated `messages.search` as `to_tsvector('dimmi_search', coalesce(body,''))` (drop + add generated column; table empty at this point in fresh install); replace GIN `search` with GIN `(workspace_id, search)` | foundation index lacks `workspace_id` lead and `unaccent` |
| `20261101000303_add_inbox_columns_to_conversations.sql` | `waiting_since timestamptz`, `last_activity_at timestamptz`, `last_message_id uuid` (no fk; message delete keeps pointer, resource tolerates), `previous_conversation_id uuid references conversations on delete set null`, `merged_into_id uuid references conversations on delete set null`, `is_test boolean not null default false`, `search tsvector generated always as (to_tsvector('dimmi_search', coalesce(subject,''))) stored` | waiting-longest sort, unread, list preview, reopen-window link (data-model mentions `metadata.previous_conversation_id` but conversations has no `metadata`: explicit column instead), conversation merge, metering D10, subject search |
| `20261101000304_add_list_indexes_to_conversations.sql` | see "Conversation list indexes" below (all `CREATE INDEX CONCURRENTLY`, goose `-- +goose NO TRANSACTION`) | p95 under 50 ms |
| `20261101000305_add_channel_connection_to_messages.sql` | `messages.channel_connection_id uuid null references channel_connections on delete set null`; index `(workspace_id, conversation_id, created_at, id)`; unique-ish lookup index `(workspace_id, channel_connection_id, channel_message_id) WHERE channel_message_id IS NOT NULL` | reply routing follows customer's last channel; delivery receipt lookup; stable timeline tiebreak |
| `20261101000306_add_workspace_id_to_pivots.sql` | add `workspace_id uuid not null` (fk cascade) to `conversation_tag`, `conversation_reads`, `conversation_participants`; indexes `conversation_tag (workspace_id, tag_id, conversation_id)`, `conversation_reads (workspace_id, workspace_member_id, conversation_id)`, `conversation_participants (workspace_id, contact_id)` | convention: every tenant table has `workspace_id`, RLS needs it; tag filter and unread queries need indexes |
| `20261101000307_add_search_indexes_to_contacts.sql` | GIN `(workspace_id, search)`; GIN trigram `(workspace_id, dimmi_unaccent(lower(coalesce(name,''))) gin_trgm_ops)`; GIN trigram `(workspace_id, (email::text) gin_trgm_ops)`; btree `(workspace_id, last_seen_at desc, id desc) WHERE deleted_at IS NULL AND merged_into_id IS NULL`; GIN `(workspace_id, custom_attributes jsonb_path_ops)`; btree `(workspace_id, company_id)` | contact search, Cmd+K, attribute filters |
| `20261101000308_add_search_indexes_to_companies.sql` | GIN trigram `(workspace_id, dimmi_unaccent(lower(name)) gin_trgm_ops)`; btree `(workspace_id, domain)`; GIN `(workspace_id, custom_attributes jsonb_path_ops)` | company search |
| `20261101000309_alter_saved_replies_unique.sql` | replace unique `(workspace_id, owner_member_id, shortcut)` with `UNIQUE NULLS NOT DISTINCT` | default NULLS DISTINCT lets two shared replies share shortcut |
| `20261101000310_add_attachment_indexes.sql` | `(workspace_id, message_id)`; partial `(created_at) WHERE message_id IS NULL` | timeline attachment load; orphan purge job (cross-tenant maintenance index, documented exception to `workspace_id`-first rule) |
| `20261101000311_create_message_mentions_table.sql` | new table | |
| `20261101000312_create_contact_merges_table.sql` | new table | |
| `20261101000313_create_contact_exports_table.sql` | new table | |
| `20261101000314_add_snooze_index_to_conversations.sql` | `(workspace_id, snoozed_until, id) WHERE status = 'snoozed' AND deleted_at IS NULL` | snoozed view sort, reconcile job |

Conversation list indexes (`20261101000304`, renumbered track 03). Common predicate `P = deleted_at IS NULL`. Query adds `merged_into_id IS NULL` at runtime once track 04 ships column (keeps tracks independent).

| Name | Columns | Partial | Serves |
|---|---|---|---|
| conversations_all_activity_idx | `(workspace_id, status, last_message_at desc, id desc)` | `P AND NOT spam` | All view, default sort |
| conversations_member_activity_idx | `(workspace_id, assignee_member_id, status, last_message_at desc, id desc)` | `P AND NOT spam` | Mine, member, Unassigned (`assignee_member_id IS NULL` uses index) |
| conversations_team_activity_idx | `(workspace_id, assignee_team_id, status, last_message_at desc, id desc)` | `P AND NOT spam` | Team view |
| conversations_inbox_activity_idx | `(workspace_id, inbox_id, status, last_message_at desc, id desc)` | `P AND NOT spam` | Inbox view |
| conversations_waiting_idx | `(workspace_id, status, waiting_since, id)` | `P AND NOT spam AND waiting_since IS NOT NULL` | Waiting longest sort |
| conversations_created_idx | `(workspace_id, created_at desc, id desc)` | `P` | Created sort |
| conversations_priority_idx | `(workspace_id, status, priority desc, last_message_at desc, id desc)` | `P AND NOT spam AND priority > 0` | Priority filter and sort (views.md) |
| conversations_spam_idx | `(workspace_id, last_message_at desc, id desc)` | `deleted_at IS NULL AND spam` | Spam view |
| conversations_deleted_idx | `(workspace_id, deleted_at desc, id desc)` | `deleted_at IS NOT NULL` | Deleted (trash) view |
| conversations_search_idx | GIN `(workspace_id, search)` | | subject search |
| conversations_subject_trgm_idx | GIN `(workspace_id, dimmi_unaccent(lower(coalesce(subject,''))) gin_trgm_ops)` | | typo-tolerant subject |
| conversations_thread_lookup_idx | `(channel_connection_id, external_thread_key, created_at desc)` | `external_thread_key IS NOT NULL` | inbound routing incl. closed (foundation unique index covers only non-closed) |

Foundation indexes `(workspace_id, status, last_message_at desc)`, `(workspace_id, assignee_member_id, status)`, `(workspace_id, inbox_id, status)` become redundant; contract migration drops them after bench (task M11).

### Column semantics owned by 03 (maintained only by actions in this spec)

| Column | Rule |
|---|---|
| `conversations.number` | `UPDATE workspaces SET next_conversation_number = next_conversation_number + 1 WHERE id = $1 RETURNING next_conversation_number - 1` in creating tx. No gaps except rolled-back nothing (same tx). Never reused. |
| `subject` | email subject (04) else first contact or member message plain-text excerpt, 120 chars, cut at word boundary, `…` appended. Editable via PATCH (1 to 255 chars). |
| `last_message_at` | `created_at` of latest `type = message` row (any author). Drives default sort. |
| `last_activity_at` | latest `type IN (message, note)` row. Drives unread. |
| `last_message_id` | latest `type = message`, not deleted. Preview in list. |
| `waiting_since` | set to message `created_at` when substantive contact message arrives and `waiting_since IS NULL`; cleared (null) when member or agent sends `type = message` reply (substantive or not); cleared on close; kept on snooze. Means "customer waiting for us since". |
| `first_customer_message_at`, `last_customer_message_at` | contact messages with `substantive = true` |
| `first_reply_at`, `last_reply_at` | member or agent `type = message` with `substantive = true` |
| `message_count` | count of `type IN (message, note)` not deleted (decremented on delete) |
| `closed_at` | set on close, null on reopen |
| `is_test` | true when: workspace `test_mode`; channel adapter passes `InboundMessage.IsTest` (widget test mode, 01; 01 adds no column, this is only test flag on conversations); contact `email_verified = true` and email equals active member's user email; Dimmi Agent playground (07). Never settable by API clients. |
| `messages.substantive` | see Metering |
| `messages.channel_connection_id` | inbound: connection message arrived on. Outbound: connection chosen by routing rule. Notes/events: null. |
| `contacts.trusted_attribute_keys` | keys whose value came from trusted source (07 binding rule). Added by: 02 `ApplyIdentityClaims` (JWT claims), member or token contact writes (`UpsertIdentifiedContact`, `UpdateContact`: trusted actors), 16 `ImportContacts` only for mappings marked `trusted` on `kind user`. Removed by: widget lead `PATCH /widget/v1/contact` (01) for keys it writes, `null` value via any path. Merge: survivor keeps own list; keys copied from source keep source trust |
| unread (imported) | messages with `metadata.imported` never count toward unread; conversation with only imported messages unread for nobody (no `conversation_reads` rows written) |

## Code map

Laravel-style paths. Tests listed per milestone in tasks.md.

### Enums (`app/enums/`)
`conversation_status.go` (open, snoozed, closed), `message_type.go`, `author_type.go`, `delivery_status.go` (with `CanTransition(from, to)`), `channel_type.go` (registry enum: channel specs register at boot), `contact_kind.go`, `identity_type.go` (registry enum: channel specs register at boot), `participant_role.go`, `snooze_preset.go` (later_today, tomorrow, next_week, one_week), `conversation_priority.go` (none, low, medium, high, urgent to 0..4), `conversation_access.go`, `assignment_mode.go`, `draft_mode.go`, `bulk_operation_status.go`, `conversation_view.go` (mine, unassigned, all, team, member, inbox, mentions, spam, deleted, snoozed; custom views by `cvw_` id), `conversation_sort.go`, `reopen_reason.go`, `unsnooze_reason.go`, `merge_reason.go`, `export_status.go`, `bulk_result_status.go` (updated, skipped, failed), `reply_then.go` (keep, open, close, snooze), `initiated_by.go` (contact, member, agent, api, import; `import` reserved: 16 helpers dispatch no `conversation.created`).

### Contracts (`app/contracts/`)
- `channel.go` – `Channel`, `ChannelCapabilities`, `OutboundMessage`, `SendResult`, optional `ChannelTyping`, `ChannelReadMarker`, `ChannelRehomer` (16 `import` driver), sentinel errors. Full text: [channel-contract.md](channel-contract.md).

### Managers (`app/managers/`)
- `channel_manager.go` – `ChannelManager.Driver(enums.ChannelType) (contracts.Channel, error)`; drivers registered by 01/04/05 in `app_service_provider.go`. Missing driver returns `ErrChannelDriverMissing` (outbound job marks message failed `channel_driver_missing`).

### Actions (`app/actions/`)
`inboxes/`: `create_inbox.go`, `update_inbox.go`, `delete_inbox.go` (moves conversations + connections to target inbox), `ensure_default_inbox.go` (registered as workspace initializer, contracts.md "Workspace initializer hook", first in order, runs inside `CreateWorkspace` tx; idempotent for `dimmi workspace:repair`; sets `workspaces.settings.default_inbox_id`; also creates `api` channel connection per inbox). Never created lazily.

`conversations/`:
- `receive_inbound_message.go` (`conversations.ReceiveInboundMessage`, channel ingest; contract in channel-contract.md)
- `create_conversation.go` (API: contact-authored on `api` channel or member outbound 1:1), `start_conversation_for_inbound.go` (internal, used by ReceiveInboundMessage), `allocate_conversation_number.go`
- `apply_conversation_changes.go` (`ApplyConversationChanges`: PATCH field-dispatch shared by `ConversationController.Update` and bulk; locks row once, hands each changed field to action below in fixed order (api.md "Order of application"), skips no-op fields, returns changed field list; `ai_state` handed to 07 `PauseConversationAi`/`ResumeConversationAi`)
- `update_conversation.go` (subject, priority, inbox)
- `close_conversation.go`, `reopen_conversation.go`, `snooze_conversation.go`, `unsnooze_conversation.go`, `resolve_snooze_time.go` (presets + DST)
- `assign_conversation.go`, `assign_on_reply.go` (rule helper; never fires for `metadata.imported` messages), `unassign_member_conversations.go` (listener on `member.deactivated`, honors `reassign_to_member_id`), `unassign_team_conversations.go` (listener on `team.deleted`, then `teams.PurgeTeam`)
- `mark_conversation_spam.go`, `unmark_conversation_spam.go`
- `delete_conversation.go`, `purge_conversation.go` (hard delete; calls `RetentionGuard.MayDestroy(ws, {conversation, id})` first, false = skip + `retention_held`)
- `merge_conversations.go`
- `tag_conversation.go`, `untag_conversation.go`
- `add_conversation_participant.go`, `remove_conversation_participant.go`
- `bulk_update_conversations.go` (ids or filter snapshot + `confirm_count` check; per-id tx via `ApplyConversationChanges`)
- `mark_conversation_read.go`, `mark_conversation_unread.go`
- `visible_scope.go` (`conversations.VisibleScope(membership)`; full rule and arch test: [access.md](access.md)): single visibility builder. Every conversation list, view, neighbor, count, search (conversation section + `#1042` jump), bulk filter snapshot, contact/company conversation list, export, MCP/CLI list, and realtime `conversation:{conv}` subscribe authorizer (incl. presence) goes through it; show endpoints use `conversations.Find(ctx, q, membership, ref)` (`VisibleScope(membership).Contains(conv)`) (miss = 404). Member modes `all`, `assigned`, `teams`, `all_except_teams`; trash adds `conversations:delete`. Arch test fails raw conversation reads outside scope
- `list_conversations.go` (view + filters + sort to query builder, scoped by `VisibleScope`), `find_neighbor_conversations.go`, `count_conversations.go` (capped counts, cached), `build_conversation_context.go`
- `record_typing.go` (`RecordTyping`: REST `typing-indicators`, only typing path for every client; throttle 1 per 2 s per member per conversation in Valkey; relay to widget / push channel only, see Collision)

`messages/`:
- `reply_to_conversation.go` (member/agent/system reply; connection driver implements `contracts.ChannelRehomer` (16 `import`): `Rehome` first, same tx, updates `channel_connection_id` (`conversation.updated`), 422 `import_channel_cannot_send` when no email connection or contact has no email; validation vs channel capabilities; collision guard; auto-assign; `then`), `add_note.go` (mentions), `post_contact_message.go` (API channel contact message), `send_system_message.go` (`conversations.SendSystemMessage(ctx, tx, input)` for 15 CSAT: inserts `type message`, `author_type system`, `substantive false`, metadata passthrough allowlist (`survey_request`); runs outbound pipeline; never reopens, never touches `waiting_since`, `last_message_at`, `last_activity_at`, unread counts, assignment; never triggers 07; emits `message.created`)
- inbound entry `ReceiveInboundMessage` lives in `conversations/` (contracts.md canonical `conversations.ReceiveInboundMessage`)
- `edit_note.go` (notes only), `delete_message.go`, `retry_message_delivery.go` (`delivery-attempts` endpoint)
- `update_message_delivery.go` (by message id or by `(connection, channel_message_id)`; monotonic)
- `parse_mentions.go`, `route_outbound_channel.go` (reply-follows-customer rule), `build_outbound_message.go`
- `list_messages.go` (cursor before/after/around)

`inbox/` (package `app/actions/inbox`; cross-spec timeline helpers, no HTTP): `record_timeline_event.go` (`inbox.RecordTimelineEvent(ctx, tx, RecordTimelineEventInput{ConversationID, Name, Actor, Data})`: inserts `type = event` row, name must be in `messages.event` list (registry), no counters, no unread, dispatches nothing (caller owns its own event)), `set_message_metadata_key.go` (`inbox.SetMessageMetadataKey(ctx, tx, messageID, key, value)`: `jsonb_set` on one owned top-level key (`ai` 07; allowlist), no `message.updated`, no `updated_at` bump beyond row).

`conversations/importing/` (16 import write helpers; 16 calls, 03 owns invariants; no events, no outbound, no billing listeners, no auto-assign, no 07 trigger): `import_conversations.go` (`ImportConversations(ctx, tx, []ImportedConversationRow)`: number block via one `UPDATE workspaces ... RETURNING`, preserved timestamps, counters `message_count`, `first_customer_message_at`, `last_message_at`, `closed_at`, `waiting_since = null`, `ai_state = 'off'`, `is_test = false`, `source.imported`), `import_messages.go` (`ImportMessages`: `tx.CopyFrom` into temp staging table + `INSERT ... SELECT ... ON CONFLICT (workspace_id, dedupe_key) DO NOTHING` (COPY cannot load RLS tenant tables for the app role); `author_id` null allowed for `member`/`agent` when `metadata.imported`; `substantive` as source says, never metered), `import_contacts.go` (`ImportContacts`: match `external_id`, then verified email identity, then exactly one contact with same email; identities `verified = false` unless attested (`email_trust`); `trusted_attribute_keys` appended only for keys in `TrustedKeys` (mappings `trusted = true`) on `kind user` rows where helper wrote value), `import_companies.go` (`ImportCompanies`: match `external_id` then `domain`), `import_tags.go`, `import_saved_replies.go`, `attach_imported_file.go` (`AttachImportedFile`: attachment row linked to imported message, scan queued).

`assignment/`: `pick_assignee.go`, `assign_on_arrival.go`, `drain_assignment_queue.go`, `set_member_availability.go`, `assignment_status.go`, `auto_away_members.go` (track 12); [assignment.md](assignment.md).

`conversation_views/`, `inbox_preferences/`, `snooze_lists/`, `drafts/`, `bulk_operations/`, `reactions/`, `side_conversations/`, `translations/`, `conversation_attribute_definitions/`, `redaction/`, `data_exports/`, `export_schedules/`, `erasures/`: one action per file, per owning spec file; `restore_conversation.go`, `resolve_snooze_option.go`, `undo_reply.go`, `set_conversation_access.go` under `conversations/` and `workspaces/` respectively.

`attachments/`: `store_upload.go`, `link_attachments.go` (targets `message`, `feedback_post`, `feedback_comment` (14; caller passes ownership, limits and image-only check); ownership + limits + channel capability check for messages; rejects `uploaded_at IS NULL` rows, 01 core change), `sign_attachment_url.go`, `purge_orphan_attachments.go` (attached = `message_id` or `feedback_post_id` or `feedback_comment_id` set), `scan_attachment.go`. Same package hosts 01-owned `create_pending_upload.go`, `complete_upload.go` (presigned/direct widget uploads); both end by queuing `ScanAttachment`.

`contacts/`: `create_contact.go`, `update_contact.go`, `upsert_identified_contact.go` (`contacts.UpsertIdentifiedContact`, `POST /contacts?upsert=true`; renamed from `IdentifyContact` to avoid clash with 02 `identity.IdentifyContact`; written `custom_attributes` keys added to `trusted_attribute_keys`), `resolve_inbound_contact.go` (identities to contact; used by ReceiveInboundMessage, 02 and 14; verified-claim takeover rule, channel-contract.md section 4), `attach_identity.go`, `merge_contacts.go`, `auto_merge_on_verified_identity.go`, `find_duplicate_contacts.go`, `block_contact.go`, `unblock_contact.go` (PATCH `blocked` field), `delete_contact.go`, `erase_contact.go` (calls `RetentionGuard.MayDestroy(ws, {contact, id})` before wipe; false = erasure `pending_hold`), `request_contact_export.go`, `build_contact_export.go` (core sections + registered export hooks), `normalize_custom_attributes.go`.

`companies/`: `create_company.go`, `update_company.go`, `identify_company.go` (`POST /companies?upsert=true`), `delete_company.go`.

`tags/`: `create_tag.go`, `update_tag.go`, `delete_tag.go`, `resolve_tags.go` (ids or names, auto-create).

`saved_replies/`: `create_saved_reply.go`, `update_saved_reply.go`, `delete_saved_reply.go`, `render_saved_reply.go`.

`search/`: `search_everything.go` (Cmd+K), `search_conversations.go`, `search_contacts.go`, `search_companies.go`.

### Support (`app/support/`)
- `searchquery/` – parser for query language (operators, phrases, `-term`, `OR`, last-term prefix) to `tsquery` lexeme list + filters. Pure Go, fuzz tested.
- `querybuilder/conversations.go` – parameterized filter/sort builder for list (only place with built SQL, per conventions).
- `templating/` – saved reply variable renderer (`{{path | fallback: "x"}}`), no logic, no loops, HTML-escaped when rendered to HTML.
- `excerpt/` – Markdown to plain-text excerpt (subjects, previews, notifications).
- `mentions/` – `<@mem_...>` / `<@team_...>` token parse/render.
- `redaction/` – built-in and custom rules applied before save ([safety.md](safety.md)).
- `attachments/policy.go` – default allowlist, hard block.
- `querybuilder/conversation_definition.go` – custom view and structured search compiler ([views.md](views.md)).
- `spam/` – inbound heuristics (link count, known patterns, honeypot flag from widget, repeated body hash per contact).

### HTTP
Controllers (`app/http/controllers/api/v1/`):
`inbox_controller.go`; `conversation_controller.go` (Index, Store, Show, Update [field-dispatch via `ApplyConversationChanges`], Destroy, UpdateMany [bulk `PATCH /conversations`]); `conversation_merge_controller.go` (Store); `conversation_tag_controller.go` (Store, Destroy); `conversation_participant_controller.go` (Store, Destroy); `conversation_read_state_controller.go` (Show, Update); `conversation_typing_indicator_controller.go` (Store); `conversation_neighbor_controller.go`; `conversation_context_controller.go`; `conversation_count_controller.go`; `conversation_message_controller.go` (Index, Store [contact message], Show, Update, Destroy); `conversation_reply_controller.go`; `conversation_note_controller.go`; `message_delivery_attempt_controller.go` (Store); `attachment_controller.go` (Store, Show, Destroy); `contact_controller.go` (Store handles `?upsert=true`; Update field-dispatches `blocked`); `contact_conversation_controller.go`; `contact_identity_controller.go` (Index, Destroy); `contact_merge_controller.go` (Store); `contact_duplicate_controller.go`; `contact_export_controller.go` (Store, Show); `company_controller.go` (Store handles `?upsert=true`); `company_contact_controller.go`; `tag_controller.go`; `saved_reply_controller.go`; `saved_reply_rendering_controller.go` (Store); `search_controller.go`. No action-named controllers (rest.md rule 14): close, reopen, snooze, assign, spam, trash, AI pause/resume are `Update` fields.

Web controller: `app/http/controllers/web/attachment_download_controller.go` (`GET /files/{attachment}/{filename}`, `signed` middleware; streams from disk; `Content-Disposition` rules; `X-Content-Type-Options: nosniff`).

Requests (`app/http/requests/`): one per write endpoint, e.g. `store_conversation_request.go`, `update_conversation_request.go` (all PATCH fields + side params, cross-field rules: `trashed: true` alone, side param needs its field, snooze param exactly one), `update_many_conversations_request.go` (`ids` xor `filter`, `changes` validated with same rules as single PATCH, `confirm_count`), `store_reply_request.go` (strict keys: `body`, `attachment_ids`, `client_message_id`, `expected_last_message_id`, `then`, `snooze`, `channel_options`), `store_note_request.go`, `store_contact_message_request.go`, `update_note_request.go`, `store_conversation_merge_request.go`, `update_conversation_read_state_request.go`, `store_typing_indicator_request.go`, `store_tags_request.go`, `store_participant_request.go`, `store_inbox_request.go`, `update_inbox_request.go`, `destroy_inbox_request.go`, `store_contact_request.go` (upsert match key rule), `update_contact_request.go` (incl. `blocked`), `store_contact_merge_request.go`, `destroy_contact_request.go`, `store_company_request.go` (upsert match key rule), `update_company_request.go`, `store_tag_request.go`, `update_tag_request.go`, `store_saved_reply_request.go`, `update_saved_reply_request.go`, `store_saved_reply_rendering_request.go`, `store_attachment_request.go`, plus list query structs `list_conversations_query.go` (filter keys reused by bulk `filter`), `list_messages_query.go`, `list_contacts_query.go`, `search_query.go`.

Resources (`app/http/resources/`): `inbox_resource.go`, `conversation_resource.go` (full + compact + list-row variants), `conversation_context_resource.go`, `message_resource.go` (member variant + `WidgetMessageResource` widget-safe variant used by 01), `attachment_resource.go`, `contact_resource.go`, `contact_identity_resource.go`, `company_resource.go`, `tag_resource.go`, `saved_reply_resource.go`, `search_result_resource.go`, `conversation_counts_resource.go`, `bulk_result_resource.go` (rule 8 shape), `conversation_read_state_resource.go`, `conversation_merge_resource.go`, `contact_merge_resource.go`, `delivery_attempt_resource.go`, `typing_indicator_resource.go`, `contact_export_resource.go`, `author_resource.go` (member/contact/agent/system; deleted member renders "Former teammate"; imported `metadata.import.author.kind = former_teammate|bot|unknown` renders `{object: "former_teammate", name}` with no member link). `conversation_resource.go` adds `imported` block from `source.imported`.

Policies (`app/policies/`): `inbox_policy.go`, `conversation_policy.go` (`UpdateField(actor, conversation, field)` per PATCH field, rest.md rule 3), `message_policy.go`, `contact_policy.go`, `company_policy.go`, `tag_policy.go`, `saved_reply_policy.go`, `attachment_policy.go`. Every policy calls `authz.Can(principal, permission)` (Permissions matrix); no role names in 03 code (P2).

Broadcasting (`app/broadcasting/`): `conversation_channel.go` (subscribe authorizer for `conversation:{conv}` (namespace with presence + join/leave on, history on, client publish off); checks `VisibleScope(membership).Contains(conv)`; returns `ClientInfo {user: "mem_...", info: {name, avatar_url}}`), `member_channel.go` (`member:{mem}`: own member only). Registered in `routes/channels.go`. No client-event hooks; no typing publications (typing is REST, relayed to customer side only).

### Events (`app/events/`)
Existing catalog structs implemented here: `conversation_created.go`, `conversation_assigned.go`, `conversation_closed.go`, `conversation_reopened.go`, `conversation_snoozed.go`, `conversation_unsnoozed.go`, `conversation_priority_changed.go`, `conversation_tagged.go`, `conversation_untagged.go`, `conversation_marked_spam.go`, `conversation_unmarked_spam.go`, `conversation_deleted.go`, `conversation_read.go`, `message_created.go`, `message_updated.go`, `message_deleted.go`, `message_delivery_updated.go`, `contact_created.go`, `contact_updated.go`, `contact_merged.go`, `contact_deleted.go`. New: see Events section.

### Listeners (`app/listeners/`)
- `send_channel_outbound.go` (queued, inserts `SendChannelMessage`; catalog listener; skips messages with `metadata.imported` (defense: 16 never dispatches them))
- `schedule_unsnooze.go` (sync in tx: inserts scheduled `UnsnoozeConversation` job)
- `unassign_deactivated_member.go` (queued: `member.deactivated`), `unassign_deleted_team.go` (queued: `team.deleted`)
- `broadcast_unread_counts.go` (sync after commit: recompute counts for affected members (Valkey counts cache, rebuildable from Postgres), push `member:{mem}` `conversation.counts_changed`; coalesced 250 ms per member)
- `bust_conversation_counts_cache.go` (sync after commit)
- `revoke_merged_widget_sessions.go` lives in 01; this spec emits needed data on `contact.merged`.

### Jobs (`app/jobs/`)
`send_channel_message.go`, `send_channel_typing.go`, `unsnooze_conversation.go`, `reconcile_snoozes.go`, `purge_deleted_conversations.go`, `purge_spam_conversations.go`, `purge_orphan_attachments.go`, `build_contact_export.go`, `expire_contact_exports.go`, `erase_contact.go`. Details in Jobs section.

### MCP (`app/mcp/tools/`)
Core tool files owned by 09 (`app/mcp/tools/`), calling 03 actions listed in MCP tools section. 03 owns `inbox_extra` toolset files: `untag_conversation.go`, `list_tags.go`, `list_saved_replies.go`, `render_saved_reply.go`.

### Routes, config, database
- `routes/api.go` (all routes in api.md), `routes/web.go` (`files.show`), `routes/channels.go` (`conversation:{conv}`, `member:{mem}` authorizers), `routes/console.go` (periodic jobs).
- `config/inbox.go`.
- Migrations listed above. Queries: `database/queries/inboxes.sql`, `conversations.sql`, `messages.sql`, `message_mentions.sql`, `attachments.sql`, `contacts.sql`, `contact_identities.sql`, `contact_merges.sql`, `contact_exports.sql`, `companies.sql`, `tags.sql`, `conversation_tag.sql`, `conversation_reads.sql`, `conversation_participants.sql`, `saved_replies.sql`, `search.sql`.
- Factories (`database/factories/`): `inbox.go`, `channel_connection.go` (fake type), `conversation.go` (`WithMessages(n)`, `Closed()`, `Snoozed(until)`, `Spam()`, `AssignedTo(m)`), `message.go`, `contact.go` (`Lead()`, `User(externalID)`, `WithIdentity(t, v, verified)`), `company.go`, `tag.go`, `saved_reply.go`, `attachment.go`.
- Seeder: `database/seeders/inbox_demo_seeder.go` (3 inboxes, 200 conversations, all states, tags, notes, mentions) and `database/seeders/inbox_benchmark_seeder.go` (100k conversations, 1M messages, realistic word distribution) for M11.
- Test fake driver: `app/drivers/channel/fake/` (records sends; scripted errors) used by all feature tests; contract test suite `tests/contracts/channel_test.go` every channel driver (01/04/05) must pass.

## HTTP API

Full table, filters, examples, error codes: [api.md](api.md). REST rules: `00-foundation/rest.md`. Summary of route groups (all under `/api/v1`, middleware `auth` (session, first-party app, token; guard letter `A`), `workspace`, `throttle:api`; POST also `idempotent`):

| Group | Routes |
|---|---|
| Inboxes | 5 (CRUD) |
| Conversations | list, create, show, update (`PATCH`: attributes + `status`, `assignee_id`/`team_id`, `spam`, `trashed`, `ai_state`, tags, `unread`), bulk update (`PATCH /conversations`), delete (trash), merges, tags (add/remove), participants (add/remove), read-state (show/update), typing-indicators, neighbors, context, counts |
| Messages | list, show, contact message, reply, note, note edit, delete, delivery-attempts |
| Attachments | upload, show, delete; web `GET /files/{att}/{filename}` signed |
| Contacts | list, create (`?upsert=true`), show, update (incl. `blocked`), delete/erase, conversations, identities (list/delete), merges, duplicates, exports (create/show) |
| Companies | list, create (`?upsert=true`), show, update, delete, contacts |
| Tags | list, create, update, delete |
| Saved replies | list, create, show, update, delete, renderings |
| Search | `GET /search`, `POST /conversation-searches` |
| Added this round | custom views, inbox preferences, snooze lists and options, drafts, bulk operations, reactions, mentionables, assignment status, away reasons, member activity, attribute definitions, redaction rules and tests, contact erasures, data exports, export destinations and schedules, side conversations, similar, translations, download grants. api.md "Added endpoints" |

## MCP tools

Canonical tool list, names, confirmation flow (elicitation; always confirms sends; send modes later: agent-send-modes, 09): 09 (`09-agent-native/mcp.md`). 03 supplies actions, resources, abilities. Ability required = same as REST.

| 09 tool | 03 action | Ability |
|---|---|---|
| `dimmi_list_conversations` / `dimmi_get_conversation` | `ListConversations`, show + `ListMessages` + context | conversations:read |
| `dimmi_search_conversations` | `SearchConversations` (same query language as `/search`) | conversations:read |
| `dimmi_send_reply` | `ReplyToConversation` after 09 confirm gate (`app/mcp/confirm`) | conversations:write |
| `dimmi_add_note` | `AddNote` | conversations:write |
| `dimmi_tag_conversation` | `TagConversation` (names auto-create) | conversations:write |
| `dimmi_assign_conversation`, `dimmi_close_conversation`, `dimmi_reopen_conversation`, `dimmi_snooze_conversation` | same-named actions; REST equivalent `PATCH /conversations/{id}` with `assignee_id`/`team_id`, `status: closed`, `status: open`, `status: snoozed` (rest.md rule 15: tools keep task names) | conversations:write |
| `dimmi_search_contacts` / `dimmi_get_contact` / `dimmi_update_contact` | contacts actions | contacts:read / contacts:write |

03 feature toolset `inbox_extra` (09 "Feature toolsets" rules; files in `app/mcp/tools/`, off by default): `dimmi_untag_conversation`, `dimmi_list_tags`, `dimmi_list_saved_replies`, `dimmi_render_saved_reply`, `dimmi_set_priority`, `dimmi_merge_conversations`, `dimmi_add_participant`, `dimmi_list_views`, `dimmi_structured_search`. All internal or read-only, no confirm gate, except merge (irreversible: 09 confirm gate). All follow caller's `VisibleScope`.

No CLI commands owned (09 generates CLI from OpenAPI).

## Events

### Emitted (catalog names)
`conversation.created`, `conversation.assigned`, `conversation.closed`, `conversation.reopened` (reason: customer_message, manual, reply), `conversation.snoozed`, `conversation.unsnoozed` (reason: expired, customer_message, manual, member_reply, others_note, assigned, attribute_changed, side_reply), `conversation.priority_changed`, `conversation.tagged`, `conversation.untagged`, `conversation.marked_spam`, `conversation.unmarked_spam`, `conversation.deleted`, `conversation.read`, `message.created`, `message.updated`, `message.deleted`, `message.delivery_updated` (from `UpdateMessageDelivery`, invoked by channels), `contact.created`, `contact.updated`, `contact.merged`, `contact.deleted`.

Payload additions (all events): `client_mutation_id` (from request header `Dimmi-Client-Mutation-Id`, max 64 chars, echoed into broadcast `meta`), `actor`.

`contact.updated` and `company.updated` carry `changes: {field: [from, to]}` plus `changed_attribute_keys []string` (custom attribute keys added, changed or removed in this write; 14 revenue recompute and 18 dependency filter read it without diffing).

`message.created` struct fields billing (12), notifications (06), agent (07), outbound need, so no re-query storm:
```go
type MessageCreated struct {
    Base                                  // ID, WorkspaceID, OccurredAt, Actor, ClientMutationID
    MessageID, ConversationID, ContactID  ids.UUID   // framework/ids
    ChannelConnectionID  *ids.UUID
    ChannelType          enums.ChannelType
    Type                 enums.MessageType
    AuthorType           enums.AuthorType
    AuthorID             *ids.UUID
    Substantive          bool
    ConversationIsTest   bool
    ConversationSpam     bool
    ContactBlocked       bool
    HasInboundMessage    bool      // conversation.first_customer_message_at != nil (D1a)
    DeliveryStatus       enums.DeliveryStatus
    MentionedMemberIDs   []ids.UUID
    ConversationCreated  bool      // first message of new conversation
}
```

`conversation.created` carries `InitiatedBy enums.InitiatedBy` (`contact|member|agent|api|import`; 12 billing + fair-use input; semantics in api.md Create conversation). `message.updated` carries `AddedMentionedMemberIDs []ids.UUID` (note edit added mentions; 06 notifies only these).

Dispatch phases (contracts.md "Event dispatch phases"): `message.created` and `conversation.created` dispatched inside action tx, so 12 InTx listeners (`MeterMessageActivity`, `GuardOutboundFairUse`) can abort reply / create. Reply action maps `ErrActivationBlocked{Reason}` to 423 (`spend_cap_reached` etc., `billing` block) and `ErrOutboundFairUse` to 429. Inbound (`ReceiveInboundMessage`, contact messages) never blocked: 12 listener only gates workspace-authored activations.

### Consumed
| Event | Listener here | Effect |
|---|---|---|
| message.created (type message, author not contact, channel push) | `SendChannelOutbound` | outbound job |
| conversation.snoozed | `ScheduleUnsnooze` | scheduled job |
| all conversation.* and message.created/deleted, conversation.read | `BroadcastUnreadCounts`, `BustConversationCountsCache` | counts |
| member.deactivated (00-accounts) | queued listener `UnassignDeactivatedMember` (default queue) calls `UnassignMemberConversations` | open/snoozed only: reassign to event `reassign_to_member_id` when still active, else unassign member, keep team; one `conversation.assigned` per conversation (batched 500/tx); closed keep assignee |
| team.deleted (00-accounts) | queued listener `UnassignDeletedTeam` calls `UnassignTeamConversations` | unassign team (events), null `inboxes.default_team_id`, then call 00-accounts `teams.PurgeTeam` |

### New events (add to catalog in same PR)

| Event | Realtime | Webhook | Notes |
|---|---|---|---|
| conversation.updated | yes | yes | subject or inbox change; `changes: {field: [from, to]}` |
| conversation.merged | yes | yes | `source_id`, `target_id`, moved message count; D12 input for 12 |
| conversation.participants_changed | yes | yes | added/removed contact ids, role |
| conversation.counts_changed | yes (`member:{mem}` only) | no | per-member view counts; ephemeral, not stored in audit |
| conversation.seen_by_team | yes (`widget_contact:{ctc}` only) | no | widget-safe "Seen" relay. Emitted when member read (`conversation.read`) moves past latest contact-visible workspace message on widget conversation. Payload `{conversation_id, seen_at}` only; adds `seen_by: {name, avatar_url}` only when workspace setting `seen_receipt_shows_name = true` (default false). Never member id. Debounced 1 per conversation per 5 s. `conversation.read` itself stays member only |
| company.created / company.updated / company.deleted | yes | yes | |
| tag.created / tag.updated / tag.deleted | yes | yes | delete carries affected conversation count (optional `feedback_post_count`, 14, display only; pivot cascades); no per-conversation untag events |
| saved_reply.created / saved_reply.updated / saved_reply.deleted | yes (personal: owner `member:{mem}` only; shared: `workspace:{ws}`) | no | palette cache |
| inbox.created / inbox.updated / inbox.deleted | yes | no | settings change |
| contact.blocked / contact.unblocked | yes | yes | separate from contact.updated so webhooks can filter |
| contact.export_ready | yes (`member:{mem}` of requester) | no | download link via REST |
| conversation.restored | yes | yes | trash path reactivated; safety.md |
| conversation.hidden | yes (`member:{mem}`) | no | ephemeral, access.md |
| conversation.viewers_changed | yes (fan-out chokepoint) | no | ephemeral, coalesced 1 s, views.md |
| member.access_changed | yes (`member:{mem}`) | no | access.md; owner 03, row in 00-accounts catalog |
| member.availability_changed | yes (`workspace:{ws}`) | no | assignment.md |
| conversation_view.created / updated / deleted | yes (shared `workspace:{ws}`, personal `member:{mem}`) | no | views.md |
| inbox_preferences.updated | yes (`member:{mem}`) | no | views.md |
| snooze_list.updated | yes (`workspace:{ws}`) | no | workbench.md |
| draft.updated | yes (`member:{mem}`) | no | workbench.md |
| bulk_operation.updated | yes (`member:{mem}`) | no | workbench.md |
| message.reactions_changed | yes (`conversation:{conv}`) | no | collaboration.md |
| attachment.scanned | yes (`conversation:{conv}`) | no | safety.md |
| data_export.ready | yes (`member:{mem}`) | no | safety.md |
| conversation_attribute_definition.created / updated / deleted | yes (`workspace:{ws}`) | yes | records.md |
| side_conversation.created / updated, side_message.created | yes (`conversation:{conv}`) | no | collaboration.md |

Conversation list events move to new channel `workspace_conversations:{ws}` plus per-member copies ([access.md](access.md) Realtime).

## Jobs

| Job (kind) | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| `send_channel_message` | `mail` for email, `channels` others | `SendChannelOutbound` listener (InsertTx), manual retry | River backoff capped at 30 min until driver `Capabilities.RetryWindow` elapsed (default about 3 h, email about 24 h; channel-contract.md); rate limit snooze does not count; then `failed` | `ByArgs` (message_id) while pending/scheduled/running |
| `send_channel_typing` | `channels` | typing relay for channels with `Typing` capability | 0 (ephemeral) | `ByArgs` (conversation_id) + `ByPeriod` 4 s |
| `unsnooze_conversation` | `default` | `ScheduleUnsnooze`, ScheduledAt = snoozed_until | 5 | `ByArgs` (conversation_id, snoozed_until) |
| `assign_on_arrival` | `default` | listener on `conversation.created`, `ai.handoff_requested`, team assignment, reopened | 5 | `ByArgs` (conversation_id, reason); assignment.md |
| `drain_assignment_queue` | `default` | capacity change, periodic 1 min | 3 | `ByArgs` (inbox, pool) + `ByPeriod` 5 s |
| `auto_away_members` | `maintenance` | periodic 1 min | 1 | `ByPeriod` |
| `run_bulk_operation` | `default` | 202 bulk | 10 | `ByArgs` (bulk_operation_id); one running per workspace |
| `erase_message_copies` | `default` | message delete, redact | 10 | `ByArgs` (message_id); runs `MessageEraser` hooks |
| `purge_stale_drafts` | `maintenance` | daily | 3 | `ByPeriod` |
| `apply_conversation_retention` | `maintenance` | daily | 3 | guard per row; safety.md |
| `build_data_export`, `expire_data_exports` | `default`, `maintenance` | `POST /data-exports`, hourly | 3 | `ByArgs` (export_id) |
| `run_export_schedule` | `maintenance` | hourly check | 5 | `ByArgs` (schedule, period) |
| `send_side_message` | `mail` | side message created | as `send_channel_message` | `ByArgs` (side_message_id) |
| `reconcile_snoozes` | `maintenance` | periodic every 10 min | 1 | `ByPeriod` 10 min; per workspace batch, unsnoozes rows with `snoozed_until < now() - 2 min` (lost-job safety net) |
| `purge_deleted_conversations` | `maintenance` | daily 03:00 UTC | 3 | `ByPeriod` 24 h; hard delete conversations `deleted_at < now() - inbox.deleted_retention_days` via `PurgeConversation` (guard per row), 500 per tx, attachments files deleted after commit |
| `purge_spam_conversations` | `maintenance` | daily | 3 | spam conversations with no activity for `inbox.spam_retention_days`; `RetentionGuard.MayDestroy(ws, {conversation, id})` per row (false = stays in spam); soft delete (go to trash), then normal purge (guarded again) |
| `purge_orphan_attachments` | `maintenance` | hourly | 3 | `message_id IS NULL AND feedback_post_id IS NULL AND feedback_comment_id IS NULL AND created_at < now() - 24h` (14 feedback targets count as attached); `RetentionGuard.MayDestroy(ws, {attachment, id})` per row |
| `scan_attachment` (`ScanAttachment`) | `default` | upload complete (03 multipart, 01 `CompleteUpload`, 04/05 inbound) | 5 | `ByArgs` (attachment_id); computes `sha256` when missing, runs `attachments.scan_driver` (`none` default sets `skipped`; ClamAV/GuardDuty set `clean`/`infected`); `infected` deletes file bytes, keeps row; outbound waits per channel-contract `scan_wait_max` |
| `build_contact_export` | `default` | `POST /contacts/{id}/exports` | 3 | `ByArgs` (export_id); core sections then every registered export hook (Security: "Contact export hooks") |
| `expire_contact_exports` | `maintenance` | hourly | 3 | delete zips past `expires_at`, status `expired` |
| `erase_contact` | `default` | `DELETE /contacts/{id}?erase=true`, scheduled at `contact_erasures.scheduled_for` (grace 7 days, safety.md); row marked in request tx, heavy work in job | 10 | `ByArgs` (contact_id); `RetentionGuard` first (false = erasure `pending_hold`, audit `retention_held`, nothing wiped, contact stays hidden; re-dispatched when hold released); after core wipe runs contact eraser hooks (below), then dispatches `contact.deleted` (`erased: true`) |

Retention rule (`00-foundation/contracts.md` `RetentionGuard`, extensibility.md section 8): every job and action above that destroys data calls `RetentionGuard.MayDestroy` before destroying. MVP driver noop (always true). Held row skipped, counted `dimmi_retention_held_total{kind}`, logged with `retention_held`. `expire_contact_exports` not guarded (export zip is derived copy, not source data). Feature test per job with fake guard returning false.

Queued listeners from catalog run as jobs owned by their specs (`QueueWebhookDeliveries` 10, `SendMemberNotifications` 06, `TriggerDimmiAgent` 07, `ApplyBillingCredits` 12). Billing gate `RecordBillableActivity` is InTx, not queued.

## Config keys (`config/inbox.go`)

| Key | Env | Default | Notes |
|---|---|---|---|
| `inbox.reopen_window_days` | `DIMMI_INBOX_REOPEN_WINDOW_DAYS` | 30 | default for new inboxes |
| `inbox.snooze.later_today_hours` | `DIMMI_INBOX_SNOOZE_LATER_TODAY_HOURS` | 3 | |
| `inbox.snooze.morning_hour` | `DIMMI_INBOX_SNOOZE_MORNING_HOUR` | 9 | local hour for tomorrow/next_week/one_week |
| `inbox.snooze.max_days` | `DIMMI_INBOX_SNOOZE_MAX_DAYS` | 365 | |
| `inbox.counts.cache_ttl` | `DIMMI_INBOX_COUNTS_CACHE_TTL` | 5s | |
| `inbox.counts.cap` | `DIMMI_INBOX_COUNTS_CAP` | 999 | counts above return `999` + `capped: true` |
| `inbox.search.text_config` | `DIMMI_INBOX_SEARCH_TEXT_CONFIG` | dimmi_search | Postgres text search config name |
| `inbox.search.rank_candidates` | `DIMMI_INBOX_SEARCH_RANK_CANDIDATES` | 2000 | newest matching messages ranked |
| `inbox.search.max_query_length` | `DIMMI_INBOX_SEARCH_MAX_QUERY_LENGTH` | 256 | |
| `inbox.search.statement_timeout` | `DIMMI_INBOX_SEARCH_STATEMENT_TIMEOUT` | 2s | `SET LOCAL statement_timeout` |
| `inbox.outbound.max_attempts` | `DIMMI_INBOX_OUTBOUND_MAX_ATTEMPTS` | 10 | |
| `inbox.outbound.retry_window` | `DIMMI_INBOX_OUTBOUND_RETRY_WINDOW` | 3h | default transient-retry budget when driver `Capabilities.RetryWindow` is 0 (email driver 24 h) |
| `inbox.outbound.send_timeout` | `DIMMI_INBOX_OUTBOUND_SEND_TIMEOUT` | 30s | per adapter call |
| `inbox.outbound.ordering_wait_max` | `DIMMI_INBOX_OUTBOUND_ORDERING_WAIT_MAX` | 2m | wait for earlier pending message in same conversation before sending anyway |
| `inbox.outbound.scan_wait_max` | `DIMMI_INBOX_OUTBOUND_SCAN_WAIT_MAX` | 10m | wait for attachment scan |
| `inbox.deleted_retention_days` | `DIMMI_INBOX_DELETED_RETENTION_DAYS` | 30 | trash |
| `inbox.spam_retention_days` | `DIMMI_INBOX_SPAM_RETENTION_DAYS` | 30 | 0 = keep forever |
| `inbox.bulk.sync_max` | `DIMMI_INBOX_BULK_SYNC_MAX` | 100 | at or below: sync (rest.md rule 8); above: job |
| `inbox.bulk.max_ids` | `DIMMI_INBOX_BULK_MAX_IDS` | 1000 | ids per request |
| `inbox.bulk.max_filter_matches` | `DIMMI_INBOX_BULK_MAX_FILTER_MATCHES` | 10000 | filter cap; over cap 422 `bulk_too_many` |
| `inbox.drafts.retention_days` | `DIMMI_INBOX_DRAFTS_RETENTION_DAYS` | 30 | stale draft purge |
| `inbox.gif.giphy_api_key` | `DIMMI_INBOX_GIF_GIPHY_API_KEY` | empty | empty hides GIF picker, no CSP change |
| `inbox.exports.pdf_font_path` | `DIMMI_INBOX_EXPORTS_PDF_FONT_PATH` | empty | optional TTF for scripts outside embedded font |
| `inbox.exports.max_csv_rows` | `DIMMI_INBOX_EXPORTS_MAX_CSV_ROWS` | 50000 | |
| `contacts.erase_grace_days` | `DIMMI_CONTACTS_ERASE_GRACE_DAYS` | 7 | 0 = wipe at once |
| `inbox.typing.relay_to_widget` | `DIMMI_INBOX_TYPING_RELAY_TO_WIDGET` | true | |
| `attachments.scan_driver` | `DIMMI_ATTACHMENTS_SCAN_DRIVER` | none | `none`, `clamav` (`attachments.clamav.address`), `guardduty` (S3 tag read); `ScanAttachment` job |
| `contacts.export_ttl` | `DIMMI_CONTACTS_EXPORT_TTL` | 24h | |
| `contacts.merge.max_chain` | | 5 | redirect depth for merged ids |

Fixed product limits this round (not catalog keys): 200 shared and 50 personal views, 100 away reasons, 100 attribute definitions per object, 10 redaction rules, 10 snooze options, 20 reactions per message, 50 Bcc. No new entitlement keys: every gap on for every workspace.

Limits reuse foundation global limits (message body 64 KB, 10 attachments, 25 MB, 50 tags, 50 participants, 100 custom attributes, page size 25/100).

## Permissions matrix

Implemented as permissions (`00-foundation/extensibility.md` section 4, P2). Policies call `authz.Can(principal, permission)`; role columns below are derived from 00-accounts `roles.go`, never checked in 03 code. Token effective permissions = role permissions intersect expanded abilities (same outcome as old "ability + admin role"). Visibility of conversations always via `VisibleScope`.

03 permissions (registered in 00-accounts catalog `app/auth/permissions.go`; each in exactly one ability group):

| Permission | Ability group | Roles (roles.go) | Gates |
|---|---|---|---|
| `conversations:read`, `conversations:write`, `contacts:read`, `contacts:write` | same-named | owner, admin, member | base rows below |
| `conversations:delete` | conversations:write | owner, admin | DELETE / PATCH `trashed`, deleted view, trashed show, trash search |
| `conversations:moderate` | conversations:write | owner, admin | delete others' notes, redact contact messages, link attachment uploaded by other actor |
| `contacts:erase` | contacts:write | owner, admin | GDPR erase, export |
| `contacts:manage_external_id` | contacts:write | owner, admin | PATCH `external_id` change/clear |
| `companies:delete` | contacts:write | owner, admin | company delete |
| `inboxes:manage` | settings:write | owner, admin | inbox create/update/delete |
| `tags:manage` | settings:write | owner, admin | tag rename/recolor/delete |
| `saved_replies:manage_shared` | settings:write | owner, admin | shared saved replies CRUD |
| `members:manage_access` | members:write | owner, admin | set `conversation_access` (access.md) |
| `members:manage_availability` | members:write | owner, admin | set another member's away status, capacity |
| `views:manage_shared` | conversations:write | owner, admin | shared custom views |
| `conversations:side` | conversations:write | owner, admin, member | side conversations |
| `conversations:export` | conversations:read | owner, admin | `POST /data-exports` conversations |
| `contacts:export` | contacts:read | owner, admin | `POST /data-exports` contacts |
| `attributes:manage` | attributes:write | owner, admin | conversation attribute definitions here; contact and company definitions in 18 |
| `redaction:manage` | settings:write | owner, admin | redaction rules and tests |

| Action | Owner | Admin | Member | Permission (token ability group) |
|---|---|---|---|---|
| View conversations, messages, notes, contacts, companies | yes | yes | yes | conversations:read / contacts:read |
| Reply, note, PATCH `status`/`assignee_id`/`team_id`/`spam`/`ai_state`/`priority`/`subject`/`inbox_id`/tags/`unread`, participants, typing, merge conversations | yes | yes | yes | conversations:write |
| PATCH `spam` with `block_contact: true` | yes | yes | yes | conversations:write + contacts:write |
| Read state (`read-state` show/update) | yes | yes | yes | conversations:read |
| Create conversation via API | yes | yes | yes | conversations:write |
| Edit own note, delete own message | yes | yes | yes | conversations:write |
| Delete others' notes and replies, redact contact messages | yes | yes | no | `conversations:moderate` (conversations:write) |
| Delete to trash and restore (`DELETE`, PATCH `trashed`) | yes | yes | no | `conversations:delete` (conversations:write) |
| Bulk update (ids or filter); per-field rules same as single PATCH (`trashed`: `conversations:delete`) | yes | yes | yes | conversations:write |
| Inboxes create/update/delete | yes | yes | no | `inboxes:manage` (settings:write) |
| Inboxes list/show | yes | yes | yes | conversations:read |
| Tags create, apply | yes | yes | yes | conversations:write |
| Tags rename/recolor/delete | yes | yes | no | `tags:manage` (settings:write) |
| Saved replies personal CRUD (own) | yes | yes | yes | conversations:write |
| Saved replies shared CRUD | yes | yes | no | `saved_replies:manage_shared` (settings:write) |
| Contacts create/update/upsert/block (`blocked`)/merge | yes | yes | yes | contacts:write |
| Contact delete (no conversations) | yes | yes | yes | contacts:write |
| Contact erase (GDPR), export | yes | yes | no | `contacts:erase` (contacts:write) |
| Contact `external_id` change/clear | yes | yes | no | `contacts:manage_external_id` (contacts:write) |
| Companies CRUD | yes | yes | yes | contacts:write (delete: `companies:delete`) |
| Search | yes | yes | yes | conversations:read (contacts section needs contacts:read; missing ability omits section, never 403) |
| Own drafts, inbox preferences | yes | yes | yes | conversations:write / conversations:read |
| Custom views personal | yes | yes | yes | conversations:write |
| Custom views shared | yes | yes | no | `views:manage_shared` |
| Set own away status | yes | yes | yes | conversations:write |
| Set others' away status, capacity | yes | yes | no | `members:manage_availability` |
| Side conversations | yes | yes | yes | `conversations:side` |
| Export conversations or contacts | yes | yes | no | `conversations:export`, `contacts:export` |
| Redaction, attributes, attachment policy, retention, link safety | yes | yes | no | `redaction:manage`, `attributes:manage`, settings:write, attributes:write |
| Subscribe `conversation:{conv}` (incl. presence) | yes | yes | yes | session or token; `VisibleScope` contains conversation |

Deactivated member: no access (00-accounts disconnects realtime connections). Agent actor (07) uses its own internal actor, not tokens; allowed: reply, note, assign to human/team, close (if 07 policy allows), tag.

## Collision detection and typing relay

Realtime = embedded centrifuge (`framework/broadcasting`, Redis broker + presence on Valkey). Channel names (events-and-realtime.md): `workspace:{ws}`, `conversation:{conv}`, `member:{mem}`, `widget_contact:{ctc}`.

- Presence ("who is viewing"): dashboard and app subscribe `conversation:{conv}` when conversation open. Namespace has presence + join/leave on; authorizer returns `ClientInfo {user: "mem_...", info: {name, avatar_url}}`. Member list = centrifuge presence (Valkey, TTL refreshed by node). Server stores nothing in Postgres.
- Typing (every client: dashboard, app, API, MCP): REST `POST /conversations/{id}/typing-indicators` `{state, composer}` (one auth path; client publications off). `RecordTyping` throttles 1 per 2 s per member per conversation (Valkey key, `SET NX EX`). Nothing published to teammates (teammate-to-teammate typing: later). Dashboard re-sends every 4 s while typing.
- Relay (inside `RecordTyping`, `composer = "reply"`, `state = "start"`): (a) conversation channel type `widget` and `inbox.typing.relay_to_widget`: publish `typing.started` to `widget_contact:{ctc}` with `{name, avatar_url}` of teammate (no email, no id); (b) conversation routes outbound to push channel with `Typing` capability: insert `send_channel_typing` job (unique per 4 s). `composer = "note"`: 202 no-op, never relayed. Stop events relayed only to widget; widget auto-expires 6 s.
- Send guard: reply request `expected_last_message_id` (last `type = message` id client saw). Mismatch: 409 `new_messages_since` with `data` = newer messages (max 10). Client shows "Ada replied 5 s ago. Send anyway?" and resends without field or with new id. MCP reply tool always sends field.

## Keyboard-driven UX support

- `GET /conversations/{id}/neighbors?<same view/filter/sort params>` returns `{prev: {id, number}, next: {id, number}}` via two keyset queries (one row each). Used for `j`/`k` and auto-advance after close/snooze (`then` param + client goes to `next`).
- `GET /conversations?…&include=last_message,contact,assignee,tags` gives list rows ready to render; client prefetches neighbor conversation with `include=messages,context`.
- `ETag` on `GET /conversations/{id}` and `/context`; `If-None-Match` returns 304 (warm cache revalidate free).
- Every write returns full resource. Realtime events carry same resource (or `{id, object, updated_at}` when over 10 KB) and `meta.client_mutation_id`.
- Number jump: `GET /conversations/1042` accepted (bare number resolves within workspace) and search `#1042`.
- Cmd+K: `GET /search?q=` returns top 5 conversations, 5 contacts, 3 companies, plus `commands` hint is client-side only.

## Performance budgets

| Operation | Budget p95 (server) | How |
|---|---|---|
| List conversations any built-in view/sort | 50 ms | partial composite indexes, keyset pagination, preview batch fetch by `last_message_id` (25 PK lookups), tags/assignees batch-loaded with `= ANY($ids)` |
| Get conversation + 50 messages + context | 50 ms | index `(workspace_id, conversation_id, created_at, id)`, attachments by `(workspace_id, message_id)`, past conversations limit 5 by `(workspace_id, contact_id, created_at desc)` |
| Neighbors | 20 ms | two `LIMIT 1` keyset queries |
| Counts endpoint | 30 ms cold, 2 ms cached | capped `count(*) FROM (... LIMIT 1000)` per view, 5 s cache per member, busted by events |
| Reply/note/close/assign | 100 ms | single tx, events via InsertTx |
| Search | 150 ms (50 ms selective) | see search.md |
| Inbound ingest action | 50 ms excluding attachment upload | |
| Contact upsert (`?upsert=true`) | 30 ms | unique index on external_id, email index |

Bench task M11 seeds 100k conversations / 1M messages / 50k contacts / 200 tags and runs k6-like Go bench in CI benchmark job; regression over 10% fails (foundation rule). Write amplification: conversations has about 14 indexes; every message updates `last_message_at` (non-HOT). Measured budget: reply write p95 under 100 ms still holds; if not, drop foundation-duplicate indexes first.

## Observability

Metrics (Prometheus):
- `dimmi_outbound_messages_total{channel,result}` (sent, failed, retried), `dimmi_outbound_send_seconds{channel}` histogram, `dimmi_outbound_pending_age_seconds{channel}` gauge (oldest pending).
- `dimmi_inbound_messages_total{channel,result}` (created, deduped, spam, blocked), `dimmi_inbound_ingest_seconds`.
- `dimmi_conversation_list_seconds{view,sort}`, `dimmi_search_seconds{kind}`, `dimmi_search_candidates_capped_total`.
- `dimmi_conversations_state_transitions_total{from,to,reason}`.
- `dimmi_contact_merges_total{reason}`, `dimmi_contact_merge_blocked_total{why}`.
- `dimmi_unsnooze_lag_seconds` (now minus snoozed_until at unsnooze).
Logs: every action logs `action`, `conversation_id`, `message_id`, actor, duration at debug; outbound failures at warn with provider error code (never body). Search logs query length and operators used, never query text at info.
Traces: span per action, per adapter `Send`, per search phase.

## Security and privacy notes

- Notes never leave member surfaces: excluded from widget resources, channel outbound, customer webhooks unless endpoint sets `include_notes = true` (10), MCP only for member/admin tokens.
- Mentions validated: only active members of same workspace; unknown `<@mem_x>` rendered as plain text, no notification.
- Saved reply variables: no code execution; only allowlisted paths; values HTML-escaped when rendered into HTML channels; custom attributes rendered as strings.
- Attachments: ownership check (uploaded by same workspace, unattached, uploaded by same actor unless admin); signed URLs valid 5 min for API/dashboard, 30 min for widget ([safety.md](safety.md)). Email outbound attaches file bytes (04), never links. SVG/HTML never served inline.
- Contact messages redaction by admin: body cleared, attachments deleted from disk, `metadata.redacted_by`; channel copy unaffected (stated in UI copy by frontend).
- Identify API marks `email_verified = true` (trusted server) and adds written `custom_attributes` keys to `trusted_attribute_keys` (07 agent actions bind only trusted keys; widget writes never trusted). Widget unverified email never auto-merges (data-model rule). Manual merge of unverified lead drops lead's `widget_visitor` identities (see edge cases) so impostor browser cannot read merged history.
- Search runs with `SET LOCAL statement_timeout` (2 s) and query length cap (256) to stop pathological tsquery.
- Merged/erased contact ids: other-workspace ids return 404 (tenancy rule).
- GDPR erase keeps conversation shells, `contact_id` repointed to per-workspace "Deleted contact" tombstone (created lazily, `kind = lead`, `deleted_at` set, name null).
- Bulk trash (`changes.trashed`), any filter-based bulk, and erase written to audit log (`WriteAuditLog`). Erase held by `RetentionGuard` audited as `contact_erasure` status `pending_hold`.
- Message eraser hooks (`contracts.MessageEraser`): delete and redaction reach copies; [workbench.md](workbench.md).
- Contact eraser hooks (`app/providers` registry, like workspace initializers; each idempotent, chunked, re-run safe on job retry; names `Erase<Domain>ContactData`): 01 `EraseWidgetContactData` revoke widget sessions + scrub `current_page`/`recent_pages`; 04 `EraseEmailContactData` delete raw `.eml`, `inbound_emails`/`outbound_emails` bodies, thread refs, suppressions keep hashed address only; 05 `EraseChannelContactData` `channel_message_refs`, `channel_contact_states`, inbound event payloads; 06 `EraseNotificationContactData` scrub `notifications.data` excerpts/names for contact's conversations; 07 `EraseAiContactData` delete `ai_runs`/`ai_run_steps` prompt and output text for contact's conversations; 08 `EraseHelpCenterContactData` `help_center_searches` rows linked to contact's visitor; 10 `EraseWebhookContactData` purge stored delivery payloads; 13 `EraseChangelogContactData` changelog subscribers (by contact or contact emails), reactions, reads; 14 `EraseFeedbackContactData` votes, posts, comments, requests, portal sessions; 15 `EraseEngageContactData` survey responses (answers cascade), receipts, `contact_id` scrub on requested CSAT rows. Registration order = spec number; no cross-hook dependencies. Hook missing for spec with contact PII = CI test failure (table list in test includes 13, 14, 15 tables). Erase completes (`contact_erasure.status = completed` in audit row) only after every hook returns nil.
- Contact export hooks (`contacts.ExportContributor` registry in `app/providers`, same shape as erasers: `Export<Domain>ContactData(ctx, workspace, contact) (section string, data any, files []ExportFile, err)`): `build_contact_export` adds one top-level section per hook. Registered: 13 `ExportChangelogContactData` (`changelog`: subscriptions, optional), 14 `ExportFeedbackContactData` (`feedback`: votes, posts, comments, requests), 15 `ExportEngageContactData` (`engage`: `survey_responses`, `engage_receipts`). Hook error fails export (`failed`, retry).

## Metering impact (pricing.md section 2)

03 does not write ledger (12 owns `RecordBillableActivity`). 03 guarantees correct inputs:

| Rule | 03 responsibility |
|---|---|
| D1 activation | `message.created` carries `HasInboundMessage`, `Substantive`, `AuthorType`. Billing activates on first substantive member/agent `type = message` when `HasInboundMessage` |
| D2 substantive | `substantive` set by action, never by HTTP client: member reply true; agent reply as 07 passes (acknowledgement-only false); system-authored false (auto-ack, notices); notes and events false; contact message true unless adapter flags `AutoGenerated` (OOO, bounce, auto-reply headers) |
| D3/D4 window | 12 computes from timestamps; 03 emits `message.created` for every message incl. reopen |
| D5 channels | `ChannelType` on event |
| D6 AI | `AuthorType = agent` same as member |
| D7 outbound | member-created outbound conversation: first message has `HasInboundMessage = false`; no activation until contact replies and workspace answers again |
| D8 internal | notes never activate; conversation whose contact is a member (verified email match) gets `is_test = true` |
| D9 spam | `ConversationSpam`, `ContactBlocked` on event; `conversation.marked_spam` / `unmarked_spam` events (12 credits within 14 days, re-bills on unmark) |
| D10 test | `ConversationIsTest` on event (workspace test_mode, widget test mode, member-as-contact, playground) |
| D11 API/import | `conversation.created.InitiatedBy` (`contact|member|agent|api|import`); API conversations same as D1. Import path (16, launch: importers) sets `source.imported` + `metadata.imported`, dispatches no `conversation.created`/`message.created`: never billed |
| D12 merges | `conversation.merged` with source/target, source activation info left to 12 |
| D13 failed delivery | `message.delivery_updated` with `failed` and final flag (`final: true` after last attempt or permanent error) |
| D14/D15 | timestamps UTC; message/conversation ids stable for ledger links; merged ids resolvable |

## Dependencies on other specs

| Spec | Needs from it | Gives to it |
|---|---|---|
| 00-foundation | core tables, events infra, realtime (`framework/broadcasting`: embedded centrifuge, Redis broker + presence on Valkey, history + recovery), Valkey (counts cache, typing throttle, pubsub), idempotency, signed URLs, querybuilder, Filesystem, `framework/httpclient`, `framework/http/useragent` (medama), `RetentionGuard`, registry enums (`contact_identities.type`, `channel_connections.type`) | core table changes above, new events, `btree_gin` extension |
| 00-accounts | members, roles, teams, users (timezone), workspace initializer hook (contracts.md), member deactivation, team deletion, `authz.Can` + permission catalog (03 permissions registered there) | `EnsureDefaultInbox` initializer, `UnassignMemberConversations`, `UnassignTeamConversations` |
| 01-messenger-widget | widget channel driver, widget visitor identities, offline email fallback, `InboundMessage.IsTest` from widget test sessions | `ReceiveInboundMessage`, `WidgetMessageResource`, typing relay to widget channel, `contact.merged` payload for session revoke, `conversations.is_test` column (01 adds none), `conversation.seen_by_team` widget event |
| 02-widget-identity | JWT verified claims, `ApplyIdentityClaims` (trusted keys) | `ResolveInboundContact`, contacts upsert (`contacts.UpsertIdentifiedContact`; 02 owns `identity.IdentifyContact` for portal), auto-merge rules |
| 04-email-channel | email driver (`Send` with threading), inbound parse, auto-reply detection, bounces | `contracts.Channel`, `ReceiveInboundMessage`, `UpdateMessageDelivery` |
| 05-chat-app-channels | Slack/Discord/Telegram/WhatsApp drivers, channel connection CRUD (creates connections in inbox) | same contract; `channel_options` passthrough for WhatsApp templates |
| 06-notifications | consumes `message.created` (`MentionedMemberIDs`, sourced from `message_mentions`), `message.updated` (`AddedMentionedMemberIDs`, note edit), `conversation.assigned`, unsnoozed, created | `message_mentions` table is canonical mention store (06 never reads `messages.metadata.mentions`) |
| 07-dimmi-agent | calls `ReplyToConversation` (actor agent, `Substantive`), `AssignConversation`, `CloseConversation`; reads timeline; `PauseConversationAi` / `ResumeConversationAi` actions for PATCH `ai_state` field; consumes `contact.merged`; `GET /conversations/{conversation}/ai` (sidebar AI block incl. pending action calls; dashboard calls 07 directly, not via 03 context) | 03 never writes `ai_state` column (07 owns transitions; `ConversationController.Update` hands `ai_state` field to 07 action in same tx; 07 listens to `message.created` from members and `conversation.closed`) |
| 09-agent-native | MCP transport, OAuth, CLI, confirm gate | MCP tools above |
| 10-webhooks | catalog webhook fan-out, `include_notes` opt-in | resources |
| 13-changelog | eraser + optional export hook | none |
| 14-feedback-roadmap | eraser + export hook, `feedback.RequestsForConversation` query | `ResolveInboundContact` verified-claim takeover, `LinkAttachments` feedback targets, orphan purge rule, `include=feedback_requests`, `changed_attribute_keys` |
| 15-announcements-surveys | eraser + export hook, latest CSAT query | `SendSystemMessage`, `announcement.replied` event row, `source.announcement_id`, `include=csat` |
| 18-audiences | SQL compiler for `filter[audience]`, new contact columns, `contact_event_summaries`, 18 `attribute_definitions` labels | `contact_tag`, `company_tag`, `changed_attribute_keys`, eraser hook, merge event, `include=event_summaries`, `filter[audience]` on contacts and companies |
| 16-importers | `import` channel driver implementing `ChannelRehomer`; calls import write helpers | `conversations/importing` helpers, `imported` resource block, filters `filter[imported]`/`filter[import_id]`, search `imported:`, unread rule, outbound/auto-assign skip |
| 17-workflows | calls `assignment.PickAssignee`, `AssignOnArrival` on run end; adds claim check in `AssignOnArrival` | one picker, cursor table, no round robin inside engine |
| 12-cloud-billing | InTx listener `MeterMessageActivity` on `message.created` (reply/create gate, `ErrActivationBlocked` -> 423), InTx `GuardOutboundFairUse` on `conversation.created`, queued `ApplyBillingCredits` | event fields per Metering table, `initiated_by`, `conversation.merged`, 423 mapping with `billing` block |

## Decisions

| # | Topic | Decided | Why |
|---|---|---|---|
| 1 | Text search language | `english` stemmer + unaccent base, F57 fix adjusts tokenization for non-Latin scripts; per-workspace config not in this round | Parser is one column; second generated column costs write amplification |
| 2 | Reply to closed conversation without `then` | Keeps closed. Dashboard sends `then: "open"` by default | Data-model rule: reopen by explicit flag |
| 3 | Members public id | `mem_` everywhere member referenced; `usr_` for accounts | One person can be in several workspaces |
| 4 | Prefixes not in foundation | `rpl_`, `exp_`, plus delta list | Registry rule |
| 5 | Spam retention | 30 days then trash, then 30 more days then purge. Per workspace settable | Rescue window before irreversible |
| 6 | Undo window | 5 s default, email and chat apps; widget and api 0; member override | workbench.md |
| 7 | Priority | 5 levels, smallint, old true = high | views.md |
| 8 | Bulk limits | Sync 100; job 1,000 ids or 10,000 filter matches | workbench.md |
| 9 | Different-customer merge | Off by default | records.md |
| 10 | Erase grace | 7 days, immediate on request with confirmation | safety.md |
| 11 | Spam mark and status | Status untouched, unlike Intercom | Unmark restores exactly |
| 12 | Blocked sender mail | Kept as spam, not refused | Never lose mail; no bounce backscatter |
