# 03 Shared inbox: workbench

Style: caveman. The screen a teammate works in. Gaps I-20, I-21, I-22, I-30, I-31, I-32, I-33, I-34, I-35, I-36, I-39, I-40, I-70, I-110, I-111, I-112, I-130. Track 03 inbox-workbench.

Files sit under `resources/js/features/inbox/<area>/`. Existing `composer.tsx`, `thread.tsx`, `case.ts`, `conversation-view.tsx` edited in place.

## Action registry

`resources/js/features/inbox/actions/registry.ts`: every inbox move declared once `{id, label, icon, shortcut, when(ctx), run(ctx)}`. Shortcuts, command menu, bulk bar, cheat sheet read it. A track adds moves by adding one file under `actions/` (keeps merges small). Tag, merge, block actions arrive with track 04 files; side conversation, translate with track 12.

## Keyboard (I-110)

Reuse check: `tinykeys` (MIT, under 1 KB, sequences, no dependencies), wrapped in `resources/js/features/inbox/keyboard/use-shortcuts.ts`. Never fires in input, textarea, contenteditable, open modal, except those marked `allowInInput`.

| Key | Move | Key | Move |
|---|---|---|---|
| `J` / `K` | next / previous conversation (list and open conversation) | `C` | new conversation |
| `R` / `N` | reply / note (exists) | `X` | select row |
| `A` | assign (teammate or team) | `Shift+A` | assign to me |
| `Z` | snooze picker | `Shift+T` | tag |
| `P` | priority | `L` | list or table layout |
| `Cmd+Shift+Y` | close | `Cmd+Shift+O` | reopen |
| `Cmd+Enter` | send (allowInInput; Enter variant reads T-02 preference, 00-accounts) | `Cmd+Shift+Enter` | send and close |
| `Cmd+Alt+Enter` | send and snooze | `Cmd+E` | hide or show events |
| `[` / `]` | toggle list / details panel | `G` then `I`, `M`, `U`, `A`, `S` | go to inbox, mine, unassigned, all, spam |
| `Cmd+K` | command menu (allowInInput) | `?` | shortcut sheet |
| `/` | focus search | `Esc` | close menu, clear selection |
| `Cmd+Shift+I` | insert image (composer) | `Cmd+Shift+G` | GIF (composer) |

No browser conflict: `Cmd+Shift+I` opens devtools only in some browsers when page does not `preventDefault`; handler prevents only with composer focused.

`J`/`K` use `GET /conversations/{id}/neighbors` with current view params. Client prefetches next with `include=messages,context` so no spinner. After close or snooze with auto advance: go to `next`. Budget: 20 ms server, no visible loader on prefetch hit.

## Command menu (I-111)

`Cmd+K` anywhere in dashboard. `resources/js/features/command-menu/`. BoardUI has no installed command block: build from BoardUI `Modal` + `react-aria-components` `Autocomplete`, `Menu` (already dependency), tokens and type utilities per BoardUI rules; install BoardUI block instead if registry `get_component command-menu` returns one at build time.

Sections, filtered by typed text:

1. Actions in context from registry: reply, note, close, snooze, assign, priority, tag, merge, spam, delete, upload.
2. Go to: views, teams, settings pages, new conversation, theme, language.
3. Results from `GET /search` (debounce 150 ms, I-101): jump first, conversations, contacts, companies. Each row keyboard selectable.

Shows shortcut with `Kbd`. Recent actions first (local storage, try/catch). e2e `tests/e2e/command-menu.spec.ts` light and dark.

`dashboard-sidebar.tsx` `Cmd+L` stays.

## Shortcut sheet and practice (I-112)

`?` or command menu: `features/inbox/keyboard/shortcut-sheet.tsx`, grouped from registry, BoardUI `Modal`, `Kbd`. Practice: route `/d/shortcuts`, `features/inbox/keyboard/practice.tsx`, client only, sample rows (no API), prompts one move at a time, tracks progress in local storage. Linked from sheet. e2e one path.

## Snooze picker (I-20)

`Z`, header button, bulk bar, command menu. `features/inbox/snooze/snooze-picker.tsx`: `Popover` with options from `GET /snooze-options?conversation=` resolved (label + exact time in viewer's zone), plus "Pick date and time" (BoardUI `date-picker`). `case.ts` stops hard-coding `tomorrow`. Time zone: `users.timezone` (00-accounts writes, X6), fallback workspace. Rules in api.md unchanged.

## Snooze lists (I-21)

`snooze_lists`: `workspace_id`, `team_id uuid null` fk cascade, `options jsonb not null`, `updated_at`. Unique `(workspace_id, team_id) NULLS NOT DISTINCT`. Keyed by parent, no public id. Migration `20261201030017_create_snooze_lists_table.sql`.

Option `{key, label, kind, ...}`, max 10:

| kind | Fields | Resolves |
|---|---|---|
| `preset` | `preset` (later_today, tomorrow, next_week, one_week) | api.md rules |
| `in` | `minutes` 5 to 525600 | now + minutes |
| `day_at` | `days_ahead` 1 to 365, `at` "09:00" | local date + days at time |
| `weekday_at` | `weekday` 1 to 7, `at` | next that weekday at time, never today |

Resolution order: team list of conversation's team, workspace list, built-in presets. Row missing = next. Empty array invalid (422): delete row to inherit.

API:

| Method | Path | Ability, permission |
|---|---|---|
| GET | /snooze-lists | conversations:read |
| PATCH | /snooze-lists/{scope} | settings:write, `inboxes:manage`; `{scope}` = `workspace` or `team_...`; body `{options}`; `{options: null}` removes |
| GET | /snooze-options | conversations:read; `?conversation=` computed with `until` per option for caller's zone |

Applying: PATCH side param `snooze_option` (key), third exclusive way next to `snooze_preset`, `snoozed_until`. Stale key: 422 `snooze_option_unknown`. Also in `then` object `{option}`. Bulk: per conversation resolution.

UI: section on inbox settings page; teams settings shows team override (`settings/pages/teams.tsx`). Event `snooze_list.updated` on `workspace:{ws}`, webhook no.

## Waking a snooze (I-22)

Builds on F31 fix (timer). Assumes per-minute job through status writer exists; replaces `reconcile_snoozes`-only plan: if F31 ships per-minute job, `unsnooze_conversation` scheduled job and reconcile are not added; reason `expired` same.

Workspace settings `snooze.*` (registry, `workspaces.settings`):

| Key | Default | Wakes on |
|---|---|---|
| `snooze.wake_on_reply` | true | member or agent reply with `then: keep` |
| `snooze.wake_on_others_note` | false | note by someone other than assignee |
| `snooze.wake_on_assignment` | false | assignment by someone other than new assignee |
| `snooze.wake_on_attribute_change` | true | custom attribute (I-63) changed by someone other than assignee |

Customer message always wakes (exists; F37 covers blocked and spam senders). `then: snooze` on snoozed conversation re-snoozes, never wakes. `conversation.unsnoozed.reason` adds `member_reply`, `others_note`, `assigned`, `attribute_changed`, `side_reply`. 06 `snooze_expired` still only `expired`. Controls on inbox settings page.

## Send and close, send and snooze (I-30)

`composer.tsx`: split Send button (`Dropdown`): Send, Send and close, Send and snooze. Shortcuts above. `then` already on API. Send and snooze uses first option of resolved list; dropdown submenu picks any. Send on closed conversation sends `then: "open"` (existing default). Auto advance per member preference `inbox.advance_after_send` (`member_inbox_preferences`, default true). Reply with required attributes missing and `then: close`: 422 `required_attributes_missing` rolled back, draft kept.

## Drafts (I-31)

Decision: server persists; browser caches; per conversation, per mode, per member. Replaces earlier client-only plan and old conflict rows. Agent send modes stay 09's.

### conversation_drafts (`drf_`, reserved prefix now used)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | |
| workspace_id | uuid not null | fk cascade, RLS |
| member_id | uuid not null | fk `workspace_members` cascade |
| conversation_id | uuid null | fk cascade; null only for mode `new` |
| mode | text not null CHECK in ('reply','note','new') | |
| body | text not null default '' | max 64 KB Markdown |
| attachment_ids | uuid[] not null default '{}' | max 10 |
| recipients | jsonb not null default '{}' | `{to, cc, bcc, subject, channel_connection_id}` staged |
| client_updated_at | timestamptz not null | last write wins |
| created_at, updated_at | timestamptz | |

Unique `(member_id, conversation_id, mode) NULLS NOT DISTINCT`. CHECK `(mode = 'new') = (conversation_id IS NULL)`. Index `(workspace_id, member_id, updated_at desc)`. Not searchable. One new-conversation draft per member. Migration `20261201030013_create_conversation_drafts_table.sql`.

### API

| Method | Path | Ability | Purpose |
|---|---|---|---|
| GET | /drafts | conversations:read | mine; `filter[conversation]`, `filter[mode]` |
| POST | /drafts?upsert=true | conversations:write | match `conversation_id` + `mode`; 201 or 200 `meta.upserted` |
| PATCH | /drafts/{draft} | conversations:write | body, attachment_ids, recipients, client_updated_at |
| DELETE | /drafts/{draft} | conversations:write | 204 |

- Others' draft 404 for every role. Never exposed via webhook. Conversation outside scope 404.
- `client_updated_at` older than stored: 200 with current row, nothing written. Client reconciles.
- Empty body, no attachments, no recipients: client DELETEs.
- Reply or note success deletes matching draft in same tx (no `draft_id` needed; reply body keys stay strict, E209 unchanged). `POST /conversations` member-authored deletes `new`.
- Event `draft.updated` `{conversation_id, mode, id|null}` on `member:{mem}`, webhook no. Other tabs refresh only when composer not dirty.

### Client

`features/inbox/drafts/use-draft.ts`: keystroke writes local cache `dimmi:draft:{ws}:{conv}:{mode}` (300 ms debounce, try/catch: storage may be blocked), server save 1 s debounce, flush on route change, mode switch, `visibilitychange`, `pagehide` (`fetch keepalive`). Load: newest of local vs server by timestamp. Offline: retries on reconnect. Reply and note kept apart. Composer keying by route id stops discarding state. Conversation with draft: row shows "Draft" chip; view "Drafts" under More (`GET /drafts`).

### Retention

Job `purge_stale_drafts` (`maintenance`, daily): `updated_at` older than `inbox.drafts.retention_days` (default 30). `purge_orphan_attachments` skips ids in any draft's `attachment_ids` (query `NOT EXISTS`, GIN on `attachment_ids`). Eraser deletes drafts on contact's conversations.

## Undo send (I-32)

Decision: window default 5 s for channels that cannot recall (email, chat apps), 0 for widget and api (widget can hide after, api has no send). Workspace setting `inbox.undo_send_seconds` (0, 5, 10, 20, 30; default 5). Member override `workspace_members.undo_send_seconds smallint null` (null inherits; same values), field on `PATCH /members/me`, control on Preferences (track 07). Token and MCP replies never delayed: dashboard client sends `delay: true`; API default false. Why: API callers expect prompt send, and have own confirm gate.

Migration `20261201030014_add_scheduled_send_to_messages.sql`: `messages.scheduled_send_at timestamptz null`, `messages.undone_at timestamptz null`, `workspace_members.undo_send_seconds`, partial index `(scheduled_send_at) WHERE scheduled_send_at IS NOT NULL` (maintenance, documented exception).

Reply tx with delay: message `delivery_status = pending`, `scheduled_send_at = now + window`, `metadata.undo {waiting_since, first_reply_at, last_reply_at, status, snoozed_until, assignee_member_id}` snapshot; `send_channel_message` inserted with River `ScheduledAt`. 04 honors ScheduledAt: nothing to do in driver. Everything else as normal reply (then, auto-assign, read, billing listener, `message.created`). Bar "2 s to leave" counts from window end.

Job start: short tx `SELECT FOR UPDATE`; `undone_at` set: stop; else clear `scheduled_send_at` then call driver.

Undo: `DELETE /conversations/{c}/messages/{m}` by author:

- Under lock `scheduled_send_at > now`: set `undone_at`, clear body, reverse each `metadata.undo` field only when current value is what this reply set, emit `message.delivery_updated` `failed` `final: true` reason `cancelled` (12 credits, existing D13 path), `message.deleted`. Response 200 `{data: {object: "message", undone: true, body: "<original>"}}` so composer refills.
- Window over: normal delete (below); composer toast gone by then.
- Undone rows: excluded from all timeline, search, counters queries; kept for ledger links.

UI: composer shows toast (BoardUI `notification`) "Sending in 5 s. Undo" `Cmd+Z`; message bubble shows pending clock to all. Tab closed during window: send proceeds.

## Formatting (I-33)

Reuse check: Tiptap v3 (`@tiptap/react`, `@tiptap/starter-kit`, `@tiptap/extension-mention`, `@tiptap/markdown`; MIT, ProseMirror beneath). Why not textarea toolbar: teammates who do not know Markdown still see `**`. Why not Lexical: no first-party Markdown serialization of mention nodes. `@tiptap/markdown` documented as early release: pin exact version, composer schema limited to subset below, round-trip test table in `composer-markdown.test.ts` guards each upgrade. Lazy-loaded chunk; composer shell renders immediately (`components/application/composer-loader`).

- Storage stays Markdown, API unchanged. Composer loads and saves Markdown only.
- Subset: paragraph, hard break, bold, italic, strike, inline code, code block, link, bullet list, ordered list, quote, inline image, mention token `<@mem_...>`, `<@team_...>`.
- Underline, size, colour, alignment: not offered. Narrowed on purpose: Markdown cannot carry them, chat channels cannot render them, and `body_html` is never hand-authored.
- Markdown shortcuts as typed, `Cmd+B`, `Cmd+I`, `Cmd+Shift+X` strike, `Cmd+K` in composer stays command menu; link via toolbar and `Cmd+Shift+U`.
- Toolbar: BoardUI `Button` icon group, `@remixicon/react`. Channel hints: `ChannelCapabilities` rich false shows plain preview note.
- `add-menu.tsx` sample `library.ts` removed: past conversations from context, articles via 08 when built, else section hidden.

## Emoji and GIF (I-34)

Emoji: `:` suggestion via Tiptap `Suggestion`, picker in add menu. Data `emojibase-data` (MIT) `en/compact.json` dynamic import, self-hosted in bundle chunk (CSP stays `'self'`). Emoji are unicode text in body.

GIF: Giphy. Tenor API closed 2026-06-30, so not an option. Giphy API rules: search called from client, no proxying of API or media, media URLs used as returned, visible "Powered By GIPHY", 100 calls per hour on beta keys, production key by application. Hence:

- Instance config `inbox.gif.giphy_api_key` (`DIMMI_INBOX_GIF_GIPHY_API_KEY`), default empty. Empty: no GIF tab, no CSP change, no call. Workspace setting `inbox.gif_picker` default true, off hides it.
- Key given: CSP adds `connect-src https://api.giphy.com`, `img-src https://*.giphy.com`, dashboard only. Picker `features/inbox/composer/gif-picker.tsx` calls Giphy from browser with `rating=g`, generated `random_id` per member, attribution mark, pingback on send.
- Inserted Markdown `![title](https://media1.giphy.com/...)`, `metadata.gif {provider, id}`. Server accepts remote image in member Markdown only when URL host matches `*.giphy.com` and key configured; else stripped at render as today.
- Delivery: email `<img>` remote URL with alt; chat apps plain URL; widget needs `img-src https://*.giphy.com` when key set (01 aligns), else plain link.
- Privacy note in settings copy: "Searching GIFs sends your search to GIPHY".
- Tests: Playwright routes `api.giphy.com` to stub; never real. Human step: cloud operator applies for production key.

## Inline images (I-35)

Builds on F15 fix (real uploads reach `OutboundMessage.Attachments`) and F14 download route.

`Cmd+Shift+I`, paste, drop in composer: `POST /attachments` with `inline=true`, inserted as `![name](att_01...)` token in Markdown. `LinkAttachments` on send sets message link, `attachments.inline = true`, generates `content_id`.

| Channel | Delivered as |
|---|---|
| widget | `<img>` signed URL, re-signed each render |
| email | `multipart/related`, `cid:` reference (04 render) |
| chat apps | sent as file attachment after text |

Images only (`image/png,jpeg,gif,webp`). Dimensions stored. Thread renderer `message-body.tsx` resolves `att_` tokens to attachment urls. Unknown or other-workspace token renders alt text only. Scan pending: placeholder with spinner; send waits via `scan_wait_max`.

## Delete reply or note, edit note (I-36)

API exists in api.md. Additions: others' replies deletable with `conversations:moderate` (the pasted card number by a colleague). Attachments of deleted message: files deleted after commit; `body_html`, `metadata.email` scrub. New hook `contracts.MessageEraser.EraseMessage(ctx, ws, messageID)` registry; 04 purges raw `.eml` for that message, 07 scrubs prompt copies, 10 scrubs stored delivery bodies; job `erase_message_copies` retried until every hook nil. Why: "remove the card number" fails if raw mail keeps it.

UI: hover toolbar on each message in `thread.tsx`: Copy link, Edit (own note), Delete (when allowed). `Modal` confirm: "Deleted for your team. Customer already received this email and keeps their copy." varies by channel. Note edit inline in composer component, `Esc` cancels. Tombstone "Message deleted by Ada". Realtime `message.updated`, `message.deleted`. e2e `tests/e2e/inbox-messages.spec.ts`.

## Thread extras (I-39)

In `thread.tsx`, `message-body.tsx`, new `features/inbox/thread/`:

- Image lightbox: reuse check `yet-another-react-lightbox` (MIT, zoom plugin, small). Click inline image.
- Code blocks: `rehype-highlight` (MIT, lowlight) with common-languages set, dynamic chunk; copy button; full screen via `Modal`. Inline code untouched.
- Customer time zone toggle: header control swaps timestamps to `contact.timezone`; saved in preferences. Tooltip "Bob's local time". Hidden when contact timezone unknown.
- Hide events: `Cmd+E`, saved in preferences; adjacent event rows collapse to "3 events".
- Date on hover: `Tooltip` with full date on every time. All client only; no API.

## Seen indicator (I-130, X12)

Widget and chat channels with `ChannelReadMarker`: bubble shows "Seen 14:02" from existing `read_at`, `delivery_status = read`. Scanner distinction not relevant (no email reads). Email: provider delivery status only ("Delivered", "Bounced"). Never tracking pixel: 04 Never list, privacy pledge. Narrowed from Intercom, reported.

## Start conversation (I-40)

`C`, sidebar +, command menu. `features/inbox/new-conversation/new-conversation-dialog.tsx` (`Modal`, draft mode `new`).

Fields: From (`Select` of connections with `StartConversation` allowed, default inbox's email), To (contact search `GET /contacts?q=`, create on Enter from address), Cc, Bcc, Subject, body (same composer), then Send, Send and close, Send and snooze.

- Multiple To + "Send separately": client issues one `POST /conversations` per recipient with own `Idempotency-Key`, max 20 in UI, sequential, per-recipient result list; else 422 surfaced. Why no batch endpoint: one at a time by design, not campaigns; 12 fair-use guard applies per call. Without separately: first To is contact, others `to` participants.
- API additions on member-authored create: `cc`, `bcc`, `to` arrays ([records.md](records.md) participants), `delay`.
- Custom From permission: which addresses listed comes from 04 (own address, E-01 track). New conversation lands assigned to author unless changed.
- e2e `tests/e2e/inbox-new-conversation.spec.ts`.

## Bulk actions (I-70)

Decision against Intercom numbers (1,000 selected, 10,000 by filter), spec's earlier 100/1,000 and rest rule 8:

| Selection | Runs | Response |
|---|---|---|
| ids 1 to 100 | sync, as rule 8 | 200 per id |
| ids 101 to 1,000 | job | 202 `bulk_operation` |
| filter, matches 1 to 100 | sync, meta only | 200 meta |
| filter, matches 101 to 10,000 | job | 202 `bulk_operation` |
| filter over 10,000 | refused | 422 `bulk_too_many` `meta.matched` |
| bulk reply, bulk note | job always, ids or filter, max 1,000 | 202 |

Rule 8's "max 100 ids" stays for sync shape; 202 path is reported to coordinator as rule 8 exception (delta). `confirm_count` two-step for every filter call unchanged. Filter never takes `q` (ranking not stable enough to confirm a count). "Select all matching" sends the list filter without `q`; with active `q` UI offers only visible ids.

### bulk_operations (`blk_`)

| Column | Type | Notes |
|---|---|---|
| id | uuid pk | |
| workspace_id | uuid not null | RLS |
| requested_by_member_id | uuid null | fk set null |
| actor | jsonb not null | `Actor` incl. via |
| kind | text CHECK in ('update','reply','note') | |
| payload | jsonb not null | `changes`, or `{body, attachment_ids, then, snooze}` |
| conversation_ids | uuid[] not null | snapshot, keyset order, scoped at creation |
| status | text CHECK in ('pending','running','completed','failed','cancelled') | |
| matched, updated_count, skipped_count, failed_count | int | |
| errors | jsonb not null default '[]' | `{id, code}`, cap 1,000 |
| cursor | int not null default 0 | next index |
| filter | jsonb null | audit |
| started_at, finished_at, created_at, updated_at | timestamptz | |

Index `(workspace_id, status, created_at desc)`. Migration `20261201030018_create_bulk_operations_table.sql`. Rows purged after 30 days.

Job `run_bulk_operation` (`default`): takes 100 ids, each own tx through `ApplyConversationChanges` or reply/note action, rechecks `VisibleScope` of requester per id, saves counters, re-enqueues itself. Resume after crash from `cursor`; re-run idempotent (`skipped`). One running per workspace, others queue, so spam clean-up cannot starve inbox. Reply uses `client_message_id = blk_id:conv_id`, body variables rendered per conversation, no undo window, no `expected_last_message_id`.

### API

| Method | Path | Ability | Purpose |
|---|---|---|---|
| PATCH | /conversations | conversations:write | table above |
| POST | /bulk-operations | conversations:write | `{kind: "reply"|"note", ids|filter, confirm_count?, body, attachment_ids?, then?, snooze?}` |
| GET | /bulk-operations/{bulk_operation} | conversations:read | progress; requester or `conversations:moderate` |
| PATCH | /bulk-operations/{bulk_operation} | conversations:write | `{status: "cancelled"}`; finished ones 422 `invalid_transition` |

Event `bulk_operation.updated` on `member:{mem}` throttled 1 per s, webhook no. Audit all job runs. 12: each reply passes own gate, block on one conversation: `failed` `spend_cap_reached`, others go on. 06: assignment fan-out aggregated by 06 burst rules.

### UI

`features/inbox/bulk/`: row checkbox, `X`, `Shift+click` range, Select all on page, then "Select all N matching" (count from `confirmation_required`). Bar `bulk-bar.tsx` from registry: Reply, Note, Assign, Tag, Snooze, Close, Priority, Spam, Delete (when allowed), Export (`conversations:export`). Under job: progress toast, cancel, final summary with failed ids link. e2e `tests/e2e/inbox-bulk.spec.ts`.
