# 03 Shared inbox: customer records

Style: caveman. Tags, attributes, customer panel, directory, companies, merges, participants. Gaps I-60, I-61, I-63, I-71, I-72, I-80, I-90, I-91, I-92, I-93, E-05 panel. Track 04 customer-records, except I-63 in track 12 inbox-extras.

## Conversation tags (I-60)

Tables exist (`tags`, `conversation_tag`), queries in `database/queries/tags.sql`. API already in api.md; actions `app/actions/tags/`, `TagConversation`, `UntagConversation` get built.

- `20261201040006_add_message_to_conversation_tag.sql`: `conversation_tag.message_id uuid null references messages on delete set null`. Shift+T on focused message tags conversation and records which message prompted it. Filtering stays conversation-level. Why: one vocabulary; message-only tags split reports for no gain.
- Filter `filter[tag]`, operators `tag:`, view field `tag`. Add `filter[tag_any]` (OR) next to AND default.
- UI: `features/inbox/tags/tag-picker.tsx` (`Popover`, type to find or create, `Shift+T`), chips in header and panel, tag bulk action. Member can create and apply; rename, recolor, delete need `tags:manage`.
- Settings: `TagsSettings` in `resources/js/features/settings/pages/inbox.tsx` real: list with counts (`include=counts`), rename, colour, delete confirm with count. Sample data goes. e2e `tests/e2e/tags.spec.ts`.
- Colour: stored hex is user data; chip renders it as inline dot only, text stays semantic token. Dark mode safe.

## People and company tags (I-61)

`20261201040005_create_contact_company_tag_tables.sql`: `contact_tag (workspace_id, contact_id, tag_id, created_by_type, created_by_id, created_at)` PK `(contact_id, tag_id)`, index `(workspace_id, tag_id)`; `company_tag` same shape. Same `tags` vocabulary. RLS. One spec of these tables: here, built by track 04. 18 reads them through rule fields `contact.tags`, `company.tags` and adds nothing to them.

- `PATCH /contacts/{id}`, `PATCH /companies/{id}`: `add_tags`, `remove_tags`. `filter[tag]` on both lists. View fields `contact.tag`, `company.tag`. Max 50 per contact or company, 10,000 tags per workspace, 422 `tag_limit_reached`.
- Events carry in `contact.updated.changes.tags`; no separate event.
- Merge: union, cap 50. Erase: rows removed. Tag delete cascades; count in `tag.deleted`.
- Follows person across conversations: panel chips; list row does not show them.

## Custom attributes (I-63, track 12)

`20261201120004_create_conversation_attribute_definitions_table.sql`:

`conversation_attribute_definitions` (`cad_`): `workspace_id`, `key text` (`[a-z0-9_]`, 1 to 64), `label`, `type text CHECK in ('text','list','number','decimal','boolean','date','reference','file')`, `options jsonb` (list values `{id,label}`), `reference_object text null CHECK in ('contact','company','conversation')`, `required_to_close bool default false`, `required_when jsonb null`, `team_ids uuid[] default '{}'`, `position`, `archived_at`, timestamps. Unique `(workspace_id, key)`. Limit 100, 422 `attribute_definition_limit_reached`.

Contact and company definitions are not here: [18](../18-audiences/README.md) `attribute_definitions` (`atd_`) owns them.

`conversations.custom_attributes jsonb not null default '{}'`, GIN `(workspace_id, custom_attributes jsonb_path_ops)`.

- Contacts and companies keep free-form `custom_attributes`. Track 04 panel and directory show raw values with key as label; no build dependency on 18. When track 13 ships 18 `attribute_definitions`, labels and types come from that table, keys without definition still display raw. Conversation keys must be defined (422 `attribute_unknown`).
- Value checks: list option id; number int64; decimal string; date ISO date; reference public id of that object, same workspace; file `att_` linked to this conversation. Max 1024 chars.
- PATCH `custom_attributes` on single and bulk (`null` removes). Order step 6. Event `conversation.updated` `changes.custom_attributes`, `changed_attribute_keys`. Snooze wake per setting.
- `required_to_close`: `required_when` is definition grammar limited to `channel`, `inbox`, `team`, `tag`, `priority`. Team-limited: only for conversations of `team_ids`. Enforced when member actor (dashboard, member token) closes, single, bulk (`failed`), reply `then: close`: 422 `required_attributes_missing` `{keys}`. System, agent, workflow, integration tokens bypass. Why: requirement is about human hand-off quality; automation would otherwise stall.
- Filter `filter[attr.<key>]`, view field `attr.<key>`, sort `attr:<key>` (text, number, date, list), search `attr.plan:pro`.
- Permission `attributes:manage` (owner, admin), one permission for this table and 18 definitions. Token abilities `attributes:read`, `attributes:write`. Track 12 registers them, 18 reuses. Writing values: `conversations:write`.
- API: `GET /conversation-attribute-definitions`, `POST`, `PATCH /conversation-attribute-definitions/{id}`, `DELETE` (archives; values stay, hidden). `type`, `key` immutable (422). `/attribute-definitions` belongs to 18.
- Events `conversation_attribute_definition.created|updated|deleted`, `workspace:{ws}`, webhook yes.
- UI: settings `features/settings/pages/attributes.tsx` (new page, nav entry, conversation attributes only; contact and company in 18 Data page); panel section "Attributes" inline editors by type; close dialog lists missing. e2e `tests/e2e/attributes.spec.ts`.

## Audience data on contacts (18, track 13)

18 adds `contacts.last_heard_at`, `web_sessions`, `conversation_count`, GIN on `custom_attributes`, `contact_event_summaries`. Contact resource returns three columns once they exist (additive); `include=event_summaries` and `GET /contacts/{ctc}/event-summaries` return rollups (`contacts:read`). Panel section "Events" and attribute trust badge mount with 18 M6, track 04 panel ships without them. Merge and erase of summaries per 18 "Erase and merge".

## Customer panel (I-90, I-91, E-05)

Split `resources/js/features/inbox/details-panel.tsx` into `features/inbox/panel/`: `conversation-section.tsx` (status, assignee, team, inbox, channel, priority, tags, participants, attributes), `contact-section.tsx`, `company-section.tsx`, `past-conversations.tsx`, `email-delivery-section.tsx`. Placeholder Links and Articles sections removed until owners ship. Data: `GET /conversations/{id}/context`, one round trip.

Contact section: name (edit in place), avatar, email, phone, verified mark, kind, external id, company, location, local time, plan and other attributes, first seen, last seen, browser, OS, device, app version, source page (`conversations.source`), identities list, tags, notes. Long values truncate with copy. Pinned attributes: star on row saves key to `member_inbox_preferences.pinned_attributes` (works on raw keys, no definition needed); pinned first. Click name: profile page `/d/customers/$contact`.

Past conversations: 3 newest, "Show more" to 20 (`GET /contacts/{id}/conversations?limit=20`, scope filtered). Row: number, subject, status, time; click opens in place. Context `past_conversations` limit moves from 5 to 3 + `total_estimate`.

E-05 display: panel calls 04's suppression read API for contact's email (04 owns route, permissions, lift action, builds on F51). State shown: "Delivery blocked: hard bounce on 3 Mar" + Lift button when `email_suppressions:manage` (`email_suppressions:lift_complaint` for complaints). Nothing suppressed: section hidden.

Duplicate banner: `duplicates_count > 0`: "Possible duplicate: Bob L." Review opens merge preview (I-72).

Block action in overflow ([safety.md](safety.md)). e2e `tests/e2e/inbox-panel.spec.ts`.

## Contact directory (I-92)

`resources/js/routes/d/customers.tsx` placeholder goes; routes `/d/customers` (People), `/d/customers/companies`, `/d/customers/$contact`, `/d/customers/companies/$company`, `/d/customers/blocked`. Feature folder `resources/js/features/customers/`.

- People: BoardUI `Table`, search (`q`), filters kind, company, tag, blocked, last seen, created, `custom.<key>`; sort; cursor "Load more". Columns chooser saved in preferences. New contact dialog.
- Profile: edit fields, attributes, identities (detach), tags, notes, conversations (scoped), Merge, Block, Export data, Erase (with permissions).
- Export: `POST /data-exports` kind `contacts`, CSV of current filter ([safety.md](safety.md)), permission `contacts:export` (`contacts:read` group; owner, admin).
- Permission: `contacts:read` page; missing shows nothing in nav. All roles have it today.
- e2e `tests/e2e/customers.spec.ts`.

Backend list exists (`app/actions/contacts/list_contacts.go`); add tag filter, `filter[blocked]` hidden-by-default (blocked excluded unless asked, Intercom archive behavior).

## Notes on records

`20261201040007_add_notes_to_contacts_and_companies.sql`: `contacts.notes text`, `companies.notes text`, max 10,000 chars. Member-only: never in widget, webhook omits, resource members only. Erase wipes. Merge: appended with separator.

## Companies (I-93)

`companies` table exists. Actions per code map. Reachable via `contacts:read`, `contacts:write`.

Company page: name, domain, plan and attributes, tags, notes, people (`/companies/{id}/contacts`), past conversations `GET /companies/{id}/conversations` (new; scope filtered; newest first). Panel section on conversation. Filter `filter[company]`, search `company:acme` (name trigram or domain).

Company delete: `companies:delete`, soft, contacts `company_id` null.

## Merge conversations (I-71)

API, moves, rules: api.md "Merge conversations". Decisions here:

- Primary (target) carries on, secondary (source) closes with `merged_into_id`, read-only (writes 409 `conversation_merged`). Timeline moves to target with `metadata.merged_from_conversation_id`, dated rows interleave by `created_at`. Summary event rows both sides linking each other. Opening source redirects client to target with banner.
- Irreversible. No API or support undo. Dialog says so.
- Different customers off by default: workspace setting `inbox.merge_different_customers` bool, default false. Contact differs: 422 `merge_different_contacts` naming both. On: source contact joins target as `cc` participant only if address known, else stays author of moved rows. Setting on security section of `settings/pages/workspace.tsx`, permission `settings:write`; change audited. Why: wrong merge exposes one customer's history to another over email.
- Multi-participant threads: allowed (union), since same-contact rule already guards exposure. Better than Intercom's refusal. Target participant cap 50 overflow 422 `participant_limit_reached`, whole merge aborts.
- Both inside caller's scope, else 404.
- Reply later on target goes on target's thread (edge 124). Email threading refs repoint (04 follows merged chain).
- `merged_into_id` added by track 04 migration; list predicate adds `merged_into_id IS NULL` at runtime, not in partial index predicate (keeps track 03 list indexes independent; merged rows are closed, narrow overlap).
- UI: `features/inbox/merge/merge-dialog.tsx`: `M` from conversation, or command menu. Suggestions: same contact's open conversations; search any. Preview: both subjects, message counts, which wins. e2e `tests/e2e/inbox-merge.spec.ts`.

## Merge contacts, duplicates (I-72)

API api.md "Merge". X8 identity rules unchanged in shape (verified customer one contact across widgets, anonymous per widget, conversation stays on widget where started). Builds on F36 fix: `contacts.email` and identities agree.

- Duplicate finder `FindDuplicateContacts`: same verified or unverified email (case folded), same phone E.164, same WhatsApp id; same name + company weak signal shown second. Never auto-merged unless verified identity rule.
- `inbox.merge_different_customers` also gates: both contacts hold verified identity of same type with different values: 422 `merge_different_contacts`. Lead into user, unverified into verified: always allowed. Different non-null `external_id`: 409 regardless (exists).
- Preview: `GET /contacts/{id}/duplicates` returns reasons `[{contact, reasons: ["same_phone"]}]`. Dialog `features/customers/merge-contact-dialog.tsx` shows field by field outcome (precedence edge 110), conversation counts. No preview endpoint: client computes from two resources; server rules decide on POST.
- Conversations of source hidden from restricted caller still move: action needs `contacts:write`, not sight. Dialog counts only visible. Stated, since person identity is not conversation content.

## Participants (I-80, X2)

03 owns model, composer, API; 04 renders headers, builds on F16 fix (loads participants into `OutboundMessage`).

`20261201040004_alter_conversation_participants_roles.sql`: `role CHECK in ('to','cc')`, existing `participant` becomes `to`; adds `removed_at timestamptz null`, `added_by jsonb`. Unique active per `(conversation_id, contact_id)`. Primary contact is implicit To, no row. `enums.ParticipantRole`.

Bcc: per send only.

- Reply and member-authored create accept `to`, `cc`, `bcc`: arrays of `{email, name?}` or `{contact_id}`.
  - `to`, `cc`: persisted additions; `remove_participants: [ctc_...]` removes on this send. Applied in reply tx so "changes apply on send".
  - `bcc`: max 50, on `OutboundMessage.BCC`, stored only in `messages.metadata.bcc` (member-only subset never rendered by `MessageResource`). Timeline event row `conversation.bcc_sent` `{count, addresses}`, the internal note of who was bcc'd. Never on later sends. No contact row created for Bcc addresses (no thread membership).
- Channel: `ChannelCapabilities.ParticipantsCC` required for `to`, `cc`, `bcc` beyond primary; add `ParticipantsBCC bool` and `OutboundMessage.BCC []Recipient`. Else 422 `channel_participants_unsupported`.
- Direct route `POST /conversations/{id}/participants` `{contact, role}` and `DELETE`, also `PATCH .../participants/{contact} {role}`. Conversations:write. Limit 50 total (global, 422 `participant_limit_reached`); To + Cc on one send over email kept by 04 provider limits.
- Removed person stops getting copies; old messages stay. Their next inbound is stored, flagged.
- Outsiders: inbound from address not primary, not active participant, on known thread: stored normally, `metadata.from_non_participant = true`; 04 sets via `InboundMessage`. Thread shows badge "Not on this conversation" with Add button. No auto-add beyond existing `Cc` capture. Why: forwarded-in people must be seen, not silently copied on future replies.
- Suppressed address: send proceeds to others; 04 per-recipient failure surfaces.
- Erased contact: rows removed.

Composer: recipients row in `features/inbox/composer/recipients.tsx`, collapsed "To Bob, +2"; expand: chips, contact search, Cc and Bcc toggles (`Cmd+Shift+C`, `Cmd+Shift+B`). Header avatar stack opens participant `Popover`. Staged in draft `recipients`. e2e `tests/e2e/inbox-participants.spec.ts` with Mailpit asserting headers.

Events: `conversation.participants_changed` (exists). Webhook excludes bcc.
