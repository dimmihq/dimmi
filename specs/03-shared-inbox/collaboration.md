# 03 Shared inbox: collaboration

Style: caveman. Gaps I-50, I-37, I-51, I-52, I-38. I-50, I-37 track 03 inbox-workbench; I-51, I-52, I-38 track 12 inbox-extras. Notifications delivered by 06 (X10); this file owns write side.

## Mentions (I-50)

Write side in README: `message_mentions`, parse in `AddNote`, `EditNote`, tokens `<@mem_...>`, `<@team_...>`.

- Notes only. Replies never parse (customer visible).
- Team mention: one row per active team member at write (`via_team_id`). Workspace setting `inbox.team_mentions_enabled` default true; off: token renders as text, no rows; typeahead omits teams.
- Scope: mentioned member who cannot see conversation: row written, not delivered, not listed (view joins `VisibleScope`). If access later grows, it appears. Author warned in typeahead. Why not grant access: note author cannot change who sees what.
- Author's own row skipped, also for team mention.
- `GET /conversations/{id}/mentionables?q=`: `conversations:read`. `{members: [{id, name, avatar_url, availability, can_see}], teams: [{id, name, member_count}]}`, limit 10 each, active only. Why own route: `/members` needs `members:read` ability, mention must work under `conversations:write` tokens and reveal only names.
- Composer: Tiptap mention with `Suggestion`, `@` trigger, note mode only. Render chip in `message-body.tsx`.
- Mentions view: `filter[view]=mentions` joins `message_mentions` (index exists), default Unread on, `SegmentedControl` Unread and All; sort newest mention. Replaces empty list in `filters.go`. Mark read in bulk: bulk `changes.unread: false`.
- Counts: `mentions.unread`.
- `MentionedMemberIDs`, `AddedMentionedMemberIDs` on events. 06 trigger `mentioned`.
- Muting: 06 preference cell. Delete note: rows cascade, 06 condition check drops pending.
- e2e `tests/e2e/inbox-mentions.spec.ts`: two browsers, note, count, in-app bell (bell part lands with track 11; test asserts count until then).

## Reactions on notes (I-37)

`message_reactions`: `workspace_id`, `message_id` fk cascade, `member_id` fk cascade, `emoji text`, `created_at`. PK `(message_id, member_id, emoji)`. Migration `20261201030019_create_message_reactions_table.sql`.

- `POST /conversations/{c}/messages/{m}/reactions {emoji}`, `DELETE .../reactions/{emoji}` (url-encoded). `conversations:write`. Own only. Repeat: 200 no-op. Notes only: 422 `message_not_reactable`. Max 20 distinct per message: 422 `reaction_limit_reached`. Emoji: max 32 bytes, no ASCII alphanumerics, no whitespace, else 422; client offers only emojibase list.
- Message resource adds `reactions: [{emoji, count, member_ids}]` for notes.
- Event `message.reactions_changed`, `conversation:{conv}` only, webhook no. 06 trigger `note_reacted` to note author, in-app only by default.
- UI: hover toolbar emoji `Popover`, chips under note, click toggles.

## Side conversations (I-51, track 12)

Email a partner from a conversation; customer never sees. Email only: Slack side threads narrowed, 05 has no outbound thread model for this; reported.

`side_conversations` (`sdc_`): `workspace_id`, `conversation_id` fk cascade, `subject`, `status CHECK in ('open','closed')`, `created_by_member_id`, `channel_connection_id` (email connection), `last_message_at`, `closed_at`, timestamps. Index `(workspace_id, conversation_id)`.

`side_messages` (`sdm_`): `workspace_id`, `side_conversation_id` cascade, `direction CHECK in ('outbound','inbound')`, `author_member_id` null, `from_address`, `to_addresses text[]`, `cc_addresses text[]`, `body`, `body_html`, `dedupe_key`, `channel_message_id`, `delivery_status`, `delivery_error`, `sent_at`, timestamps. Unique `(workspace_id, dedupe_key)`. `attachments.side_message_id` nullable fk added to one-target CHECK, orphan purge counts as attached. Migrations `20261201120002`, `20261201120003`.

Not `messages`: never billed, never in customer resources, webhooks, widget, agent context, search default. Isolation by table beats filter everywhere.

- API: `GET /conversations/{c}/side-conversations`, `POST` `{to, cc, subject, body, attachment_ids}` 201 with first message; `GET .../{sdc}` includes messages; `POST .../{sdc}/messages`; `PATCH .../{sdc} {status}`. Permission `conversations:side` (group `conversations:write`; all roles by default). Scope of parent.
- Send: job `send_side_message` (`mail` queue) calling 04 `SendRelayMail`-style compose with thread headers; 04 records `email_message_refs` row with `side_conversation_id`. No customer signature block, From address of inbox. Suppression applies.
- Inbound: 04 thread lookup hit on that ref calls `conversations.ReceiveSideMessage`, never `ReceiveInboundMessage` (needs 04 milestone; email agent aligns, delta). Also malware scan, redaction.
- Parent effects: inbound adds timeline event row `side_conversation.replied`, marks unread for assignee, wakes snooze (reason `side_reply`), closed parent stays closed but notifies. Assignee and state follow parent. Parent merged: moves. Parent trashed or erased: cascades.
- Events: `side_conversation.created`, `.updated`, `side_message.created`: `conversation:{conv}`, webhook no. 06 trigger `side_conversation_replied` to assignee, in-app and email on.
- UI: panel section + `features/inbox/side-conversations/`: list, thread drawer, compose. Clear banner "Customer cannot see this". e2e with Mailpit round trip.

## Similar conversations (I-52, track 12)

`GET /conversations/{c}/similar`, `conversations:read`. No vectors (semantic search stays out). Terms: first 8 distinct non-stop lexemes of subject, then latest customer message (`tsvector` of `dimmi_search`). OR tsquery of those, ranked by `ts_rank_cd`, closed conversations in last 180 days, newest 500 candidates, excludes current, `VisibleScope`, top 5. No term statistics table: extra state for little gain at this size. Budget p95 150 ms, timeout 1 s, empty on timeout.

Response `{id, number, subject, closed_at, excerpt}`. Panel section "Similar", lazy on open, replaces sample `library.ts`. Fuzzy by nature: label "May be related". e2e in panel spec.

## Translation (I-38, track 12)

Uses 07 BYOK layer: `contracts.LLM` via `managers.LLMManager` from workspace `ai_provider_keys`. No Dimmi key. Tests use `app/drivers/llm/fake` only; never real LLM (X16).

Workspace setting `inbox.translation_enabled` default false; needs usable key, else 422 `translation_unavailable`. Model: 07 agent's answer key and model; 07 spend cap applies, usage recorded as 07 run kind `inbox_translation` (registry).

`message_translations`: `workspace_id`, `message_id` fk cascade, `locale`, `body`, `source_locale`, `model`, `created_at`. PK `(message_id, locale)`. Migration `20261201120005_create_message_translations_table.sql`. Deleted with message; erased with hook.

- `POST /conversations/{c}/messages/{m}/translations {locale}` `conversations:read`: cached row or computes; 200 or 201. Customer messages and replies; not notes, events.
- `POST /conversations/{c}/reply-translations {body, locale}` `conversations:write`: computed, not stored (rule 11), returns `{body, source_locale}`.
- Reply param `original: {body, locale}` stored `metadata.translation`, teammates see both; customer gets translated body.
- Target language: `users.locale` for reading; `contacts.locale`, else `source.locale`, else last detected `source_locale`. Same language detected: no call, returned as is.
- Prompt: system instruction translate only, customer text inside delimiters; never tools. Output sanitized as Markdown. Mentions, code blocks kept.
- Never auto-sends. Member reviews in composer preview. UI: thread per-message "Translate", conversation toggle "Show in my language" (preference), composer "Send in Bob's language" `features/inbox/translation/`. e2e with fake provider driver through test binary route only; no network.
- Failure: provider error 502 `translation_failed`, original still shown.
