# 03 Shared inbox: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Requires foundation tasks (core migrations, events, `framework/broadcasting` on centrifuge + Valkey, querybuilder, idempotency) and 00-accounts members/teams.

Existing milestones M1 to M12 partly built: builder checks code first, builds only what is missing. Items superseded this round say so. New milestones A, W, R, X below. Browser flows: Playwright in `tests/e2e` light and dark, fails on console error or CSP violation. `pnpm lint`, typecheck, test pass.

### Old migrations to track numbers

Not built yet, so renumber (goose rejects older pending). Already-present parts (e.g. pivot `workspace_id`) skip.

| Old | New | Plain name | Track |
|---|---|---|---|
| 000301 | 20261201030001 | create_search_configuration | 03 |
| 000302 | 20261201030002 | alter_messages_search_column (reconcile with F57 fix) | 03 |
| 000303 (minus `merged_into_id`) | 20261201030003 | add_inbox_columns_to_conversations (`waiting_since` backfill, `last_activity_at`, `last_message_id`, `previous_conversation_id`, `is_test`, `search`) | 03 |
| 000304 | 20261201030004 | add_list_indexes_to_conversations (predicate without `merged_into_id`) | 03 |
| 000305 | 20261201030005 | add_channel_connection_to_messages | 03 |
| 000306 | 20261201030006 | add_workspace_id_to_pivots (indexes only where missing) | 03 |
| 000307 | 20261201030007 | add_search_indexes_to_contacts | 03 |
| 000308 | 20261201030008 | add_search_indexes_to_companies | 03 |
| 000309 | 20261201030009 | alter_saved_replies_unique | 03 |
| 000310 | 20261201030010 | add_attachment_indexes | 03 |
| 000311 | 20261201030011 | create_message_mentions_table | 03 |
| 000314 | 20261201030020 | add_snooze_index_to_conversations | 03 |
| 000303 `merged_into_id` | 20261201040001 | add_merged_into_to_conversations | 04 |
| 000312 | 20261201040002 | create_contact_merges_table | 04 |
| 000313 | 20261201040003 | create_contact_exports_table | 04 |

Wave-1 dependency stated: tracks 03 and 04 need track 02 milestone A1 (`dimmi_conversation_visible`, `VisibleScope`) merged first for any list query they add. A1 is small and goes first. No other cross-track migration dependency. Track 03 bulk bar and registry show tag, merge, block actions only as track 04 adds their action files.

Tests: business logic = integration only (real App, Postgres, Valkey, River, centrifuge node; testing.md): HTTP feature tests first, action/job entry with real App second. No unit tests with mocks for app code. Pure parsers in `app/support` (searchquery, templating) get fuzz tests in addition, never mocks. Every edge case has a test, named in plain words.

## M1. Schema and models (track 03 inbox-workbench)

- [ ] Migration `20261101000301` search config (`btree_gin`, `dimmi_search`, `dimmi_unaccent`). Accept: `SELECT to_tsvector('dimmi_search','Rückerstattungen')` equals unaccented stem; `dimmi doctor` checks `btree_gin`. Test: migration test (up, down, up) + doctor check integration test.
- [ ] Migrations `302`, `303`, `305`, `306`, `309`, `310`, `314` (core table changes). Accept: up/down on empty DB; `sqlc generate` clean; tenant lint passes for pivots with `workspace_id`. Test: schema snapshot test.
- [ ] Migration `304` list indexes (CONCURRENTLY, NO TRANSACTION) + `307`, `308`. Accept: `EXPLAIN` of each list query in fixture uses intended index (test asserts plan node name on seeded 10k rows). 
- [ ] Migrations `311` to `313` new tables + queries files skeleton (`database/queries/*.sql` for every table in code map). Accept: sqlc compiles; every query has `workspace_id` predicate.
- [ ] Enums, prefixes `rpl_`, `exp_` in `app/models/public_ids.go` (`framework/ids`), typed JSON (`InboxSettings`, `ConversationSource` incl. `announcement_id` 15, workspace settings keys declared in foundation settings registry). Test: integration round trip through endpoints (encode/decode, settings defaults).
- [ ] Registry enums `enums.IdentityType`, `enums.ChannelType` over foundation registry (P1): 03 registers `email`, `phone`, `widget_visitor`, `external_id`, `api`; request rules and actions validate against registry. Test: unknown type 422 in request, action rejects unknown type, registering new type at boot needs no migration (test registers fake type, inserts row).
- [ ] 03 permissions in 00-accounts catalog (`conversations:delete`, `conversations:moderate`, `contacts:erase`, `contacts:manage_external_id`, `companies:delete`, `inboxes:manage`, `tags:manage`, `saved_replies:manage_shared`) with ability groups; every 03 policy uses `authz.Can`. Test: table test role x permission matches Permissions matrix; lint no `Role ==` in `app/policies/*` 03 files.
- [ ] Factories for all tables in code map incl. `fake` channel connection; `inbox_demo_seeder`. Accept: `dimmi db:seed --class=InboxDemoSeeder` produces 200 conversations in all states.

## M2. Inboxes, contacts, companies (track 04 customer-records)

- [ ] `EnsureDefaultInbox` (+ `api` connection) registered as first workspace initializer (contracts.md hook, inside `CreateWorkspace` tx, idempotent for `dimmi workspace:repair`); inboxes CRUD endpoints, policy, resource, `inbox.*` events. Test: feature (CRUD, default delete 409, non-empty delete needs move_to, other workspace 404, member 403).
- [ ] Contacts CRUD + list filters + custom attribute validation + phone/email normalization; `contact.updated`/`company.updated` carry `changes` + `changed_attribute_keys`. Test: feature per endpoint, validation (key format, 100 keys, 1024 chars), cross-tenant; changed keys only (E247).
- [ ] `GET /contacts?filter[audience]=adn_...`, `GET /companies?filter[audience]=` via 18 compiler; built by 18 M3 (track 13), not here. 5 s statement timeout, 422 `audience_too_slow`. Test: audience filter combines with others, slow audience 422, other workspace audience 400 (E244).
- [ ] `UpsertIdentifiedContact` (renamed from `IdentifyContact`; 02 owns `identity.IdentifyContact`) + `POST /contacts?upsert=true` per api.md algorithm (match `external_id` then `email`, 201/200 + `meta.upserted`); written `custom_attributes` keys added to `contacts.trusted_attribute_keys` (also member/token `PATCH /contacts`), merge keeps source trust for copied keys. Test: feature table test for every branch (external_id hit, email hit user, unverified lead skipped, create, immutable external_id 409, merged id follows chain); feature: no match key 422 `upsert_key_required`, `upsert=yes` 400, plain create 409 `contact_exists`; trusted keys added on upsert and PATCH, null removes (E254); merge trust rule (E255).
- [ ] Identities list/delete, block/unblock as PATCH `blocked` field (field-dispatch in `ContactController.Update`, `contact.blocked`/`unblocked`), duplicates finder (same email/phone/name+company heuristics). Test: feature (no-op on same value, events order with attributes).
- [ ] Companies CRUD + `POST /companies?upsert=true` (`external_id` then normalized `domain`) + company contacts. Test: feature.

## M3. Conversations core (track 03 inbox-workbench)

- [ ] Number allocation + `CreateConversation` (contact-authored api channel, member outbound with `StartConversation` check) + `POST /conversations`. Test: concurrent creation 50 goroutines yields unique gapless numbers; channel_cannot_initiate.
- [ ] `ConversationResource` (full, compact, list row), show endpoint with `include=messages,context`, number lookup, ETag/304; includes `csat` (15) and `feedback_requests` (14) via owner query functions (fakes until owners land); context renders `announcement` (15) from `source`. `imported` block (16) from `source.imported`, imported message author `former_teammate`. Test: snapshot JSON, 304 path, include batching (one query per include per page), E245, E246, E259.
- [ ] State machine actions (business names kept): close, reopen (thread_conflict), snooze (presets, DST, limits), unsnooze, transition table returning 422 `invalid_transition` `{from, to}`, `ScheduleUnsnooze` + `unsnooze_conversation` job + `reconcile_snoozes`. Test: feature for `snooze_preset` resolution with frozen `tc.Clock` (DST spring/fall, Sunday/Monday, no timezone), feature for each transition + no-op cases, job no-op when stale.
- [ ] Assign + assign-on-reply helper + `UnassignMemberConversations` + `UnassignTeamConversations` + queued listeners on `member.deactivated` (reassign hint) and `team.deleted` (null `inboxes.default_team_id`, then `teams.PurgeTeam`). Test: concurrency (two replies, single assign), batch 2,000 unassign events count, reassign target deactivated meanwhile falls back to unassign, closed conversations keep assignee.
- [ ] Update attributes (subject, priority, inbox move), spam mark/unmark (+ `block_contact`), soft delete (`DELETE` and PATCH `trashed: true`; restore in R7), purge jobs calling `RetentionGuard` (`PurgeConversation`, spam purge, orphan attachments). Test: feature + job tests with `tc.Clock.Freeze`; fake guard returning false per job keeps rows, metric incremented (edge cases 217, 218).
- [ ] `ApplyConversationChanges` field-dispatch + `UpdateConversationRequest` + `ConversationPolicy.UpdateField` + `ConversationController.Update`: fixed order (trashed, spam, assignment, status, ai_state, attributes, tags, unread), one row lock, one tx, no-op skip, `meta.changed`, `ai_state` handed to 07 actions (fake until 07 lands). Test: table test per field (event emitted, no-op no event), combined close + assign event order, key order irrelevant, one failing field rolls back all, forbidden field 403 before work, `trashed` alone rule, side params without field 422, merged source 409, trashed 409/404, `invalid_transition` body has `from`/`to`.
- [ ] Timeline event rows for every state change (`messages.type = event`). Test: each action asserts event row shape.

## M4. List, neighbors, counts, read state (track 03 inbox-workbench)

- [ ] Superseded by A1. `conversations.VisibleScope(principal)` single visibility builder (trash needs `conversations:delete`) used by list, show, neighbors, counts, search, bulk filter, contact/company conversation lists, exports, realtime authorizers. Test lives in A1.
- [ ] `querybuilder/conversations.go`: views, filters, sorts, keyset cursor with filter hash. Test: feature for each filter/sort combo on seeded data (rows + order, `EXPLAIN` uses intended index); pagination stable under inserts, `cursor_mismatch`, invalid filter 400.
- [ ] List endpoint with batch includes (contact, assignee, tags, last_message preview, unread). Test: query count assertion (max 5 queries per page).
- [ ] Neighbors endpoint. Test: prev/next at edges, after close still resolves.
- [ ] Read state actions + `GET`/`PATCH /conversations/{id}/read-state` singleton (`last_read_message_id` or `unread`) + PATCH `unread` field + author auto-read + `conversation.read` broadcast (`member:{mem}` only) + widget-safe `conversation.seen_by_team` (`widget_contact:{ctc}`, `{conversation_id, seen_at}`, name only with `seen_receipt_shows_name`). Test: forward-only, mark unread semantics, both keys 422, other conversation message 422, widget payload never has member id.
- [ ] Counts endpoint (capped counts, Valkey cache with generation counter) + `BroadcastUnreadCounts` coalesced 250 ms to `member:{mem}`. Test: cap at 999 (E95), cache bust on event across two App instances (E94), member-scoped channel payload, Valkey flush recomputes.
- [ ] Superseded by W9.

## M5. Messages and channel contract (track 03 inbox-workbench)

- [ ] `contracts.Channel` + `ChannelManager` + `fake` driver + `api` driver + contract test suite `tests/contracts/channel_test.go` (capability flags match interfaces, success returns ChannelMessageID, errors wrap sentinels, honors ctx deadline). Accept: fake and api pass suite.
- [ ] Attachments: upload (sniff, limits, blocked types), show (signed URL), delete, web download controller, `LinkAttachments` targets `message` / `feedback_post` / `feedback_comment` (14), orphan purge job (any target column = attached), `ScanAttachment` job + `attachments.scan_driver` (`none`, fake). Test: feature + signed URL expiry + SVG served as attachment; infected attachment never downloadable, outbound with infected attachment fails `attachment_infected`; other workspace `att_` cannot be linked to message (404); feedback-linked upload survives purge (E242); double link 409 (E243).
- [ ] `ReplyToConversation`: routing rule (skips `import` connections), `ChannelRehomer` call for 16 `import` connections (422 `import_channel_cannot_send`), strict body keys (`draft_id` 422 `unknown_field`, E209), capability validation, collision guard, `client_message_id` dedupe, `then`, auto-assign, counters, `waiting_since`, substantive rules, `message.created` fields, `conversation.created.InitiatedBy`, 12 billing gate (InTx `ErrActivationBlocked` -> 423 with `billing` block, `ErrOutboundFairUse` -> 429). Test: feature per rule; 201/200 replay/409/422 cases; InTx listener returning block gives 423 and rolls back message + assign; inbound with same listener still stored; rehome via fake `import` driver (E210, E211); `then: snooze` on closed 422 `invalid_transition`.
- [ ] `SendSystemMessage` (15 CSAT): system author, non-substantive, metadata allowlist `survey_request`, outbound pipeline, no state effects, no 07 trigger. Test: action entry with real App + fake channel driver: E234 to E237; billing listener sees `Substantive = false`.
- [ ] `AddNote` + `EditNote` with mentions parse/render (`support/mentions`, `<@mem_…>`), `message_mentions` rows in same tx, edit diff -> `message.updated.AddedMentionedMemberIDs`. Test: invalid mentions ignored, notes never in widget resource, edit adds only new ids, reply edit 422 `message_not_editable` (E79).
- [ ] `SendChannelOutbound` listener + `send_channel_message` worker (guards 1 to 6, result handling table, ordering wait, snooze on rate limit) + `UpdateMessageDelivery` monotonic. Test: fake driver scripted: success, transient x3 then success, permanent, rate limit, connection broken (connection status error), ordering of two messages, deleted before send, retry window end final failure event `final = true` (`Capabilities.RetryWindow` 0 = default about 3 h; fake driver declaring 24 h still pending at 23 h, failed after 24 h, frozen clock).
- [ ] Timeline list (before/after/around, `updated_since`, 10 s overlap), message show. Test: cursor both directions, around anchor, late-commit overlap.
- [ ] Delete (own reply: pending cancels send, sent local only; notes; admin redact), `POST …/messages/{id}/delivery-attempts` (202 `delivery_attempt`). Test: feature per rule; sent push-channel reply delete makes no driver call (E80); non-failed 409 `message_not_failed`.
- [ ] Contact message endpoint for api channel. Test: 409 on non-api, reopen path.

## M6. Inbound (track 03 inbox-workbench)

- [ ] `ReceiveInboundMessage` per channel-contract.md algorithm (dedupe, resolve contact, pick conversation incl. reopen window, merged chain, spam decision, state effects, participants, `AfterPersist` hook step 9b). Test: table-driven integration tests per step (action entry with real App, fake channel driver); concurrency test dedupe race; window boundary equal; `AfterPersist` receives same tx (row visible inside, rolled back on hook error, no message/dedupe row left, retry ingests once); hook not called on `Duplicate`.
- [ ] `ResolveInboundContact` + `AutoMergeOnVerifiedIdentity` + verified-claim takeover (channel-contract.md section 4; 14 portal, 04 DMARC, 02 JWT). Test: verified vs unverified, external_id conflict blocked, survivor choice, takeover moves unverified row without merge (E239), verified row never taken over (E240), unverified claim no takeover (E241).
- [ ] `FindConversationByChannelMessage` helper, `app/support/spam` heuristics, UA parse via `framework/http/useragent` (medama) at ingest. Test: integration through fake-channel ingest (spam score, `source.browser/os/device`).
- [ ] Fake-channel end-to-end test: inbound, reply, delivery receipt, customer reopen after close, new conversation after window.

## M7. Collision and realtime (track 03 inbox-workbench)

- [ ] `conversation:{conv}` subscribe authorizer (presence + join/leave, history on, client publish off; `ClientInfo` `mem_` id, name, avatar) + `member:{mem}` authorizer, both via `VisibleScope`. Test (real centrifuge node + Valkey): other workspace denied (E99), outside scope denied (E226), deactivated disconnected (E156), two tabs one presence member (E230).
- [ ] `RecordTyping` + `POST /conversations/{id}/typing-indicators` (202): Valkey throttle, relay to `widget_contact:{ctc}` + `send_channel_typing` job, nothing published on `conversation:{conv}`. Test: note composer not relayed (E101), throttle (E232), job uniqueness 4 s, email conversation no-op (E208).
- [ ] `client_mutation_id` echo on all events; workspace list payloads use list-row resource. Test: centrifuge-js Node script receives `message.created` on `conversation:{conv}` with `meta.client_mutation_id`; reconnect with offset recovers (E104); Valkey restart gives `recovered: false` (E231).

## M8. Tags, saved replies, bulk, participants (track 04 customer-records)

- [ ] Tags CRUD, apply/remove by id or name, limit 50, `tag.*` events, delete cascade single event. Test: feature, case-insensitive collision.
- [ ] Saved replies CRUD (personal/shared, policy), `support/templating` with fallback filter, `POST /saved-replies/{id}/renderings`, `missing_variables`. Test: feature via `POST /saved-replies/{id}/renderings` (unknown var blank, unbalanced braces literal, HTML escape mode, CJK first_name) + fuzz on renderer (no panic).
- [ ] Superseded by W11 (keep shape). Bulk `PATCH /conversations` (`ConversationController.UpdateMany`, `BulkUpdateConversations`): `changes` = single-PATCH fields validated once, per-field permission once, per-id tx via `ApplyConversationChanges`, rule 8 result shape (`updated`/`skipped`/`failed`). Test: mixed success, other workspace ids `not_found`, member `trashed` 403, 1,001 ids 422, duplicate ids deduped, re-run gives `skipped`.
- [ ] Superseded by W11. Filter-based bulk: list filter keys reused, `q` rejected, two-step `confirm_count` (422 `confirmation_required`, 409 `bulk_count_mismatch`), cap `inbox.bulk.max_filter_matches`, id snapshot, meta-only response with `errors`, audit log. Test: count drift between steps, over cap 422, snapshot item closed mid-run `skipped`, audit row written.
- [ ] Superseded by R6.

## M9. Merge and GDPR (track 04 customer-records)

- [ ] `MergeContacts` (manual, `POST /contacts/{target}/merges`, 201 `contact_merge`, 202 over 50k conversations) + `contact_merges` log + widget identity drop for unverified lead + chain flatten. Test: field precedence table, 409 external_id conflict, concurrent opposite merges no deadlock.
- [ ] `MergeConversations` + `POST /conversations/{target}/merges {from: [...]}` (max 10, all-or-nothing, UUID lock order) + one `conversation.merged` per source + inbound follow chain. Test: messages moved, tags capped 50, target closed stays closed, pending outbound still sends, `merge_self` 422, live into imported target 422 `merge_imported_target` (E262), source merged elsewhere 409 aborts all, already-merged-into-target `skipped_ids`, 201 `conversation_merge` body.
- [ ] Contact export job (`POST /contacts/{id}/exports` 202; zip JSON + attachments, signed URL 24 h, expiry job) + erase job (security.md rules, tombstone contact, `RetentionGuard` first: held = `pending_hold`) + contact eraser hook registry (README Security; registered 01, 04 to 10, 13 `EraseChangelogContactData`, 14 `EraseFeedbackContactData`, 15 `EraseEngageContactData`) + contact export hook registry (`ExportContributor`: 13 optional, 14, 15). Test: export contents, erase wipes bodies/attachments/identities and runs every registered hook (fake hooks; one failing hook retries job, erase not marked complete), registry test lists every spec with contact PII, conversation shells kept, audit row; fake guard false leaves data, audit `pending_hold`, no hooks run (edge case 219); export of other workspace contact 404; export signed URL expires; 13/14/15 hooks run and fail-retry (E248); export sections per hook (E249).

## M9b. Cross-spec helpers (07 agent actions, 16 importers) (track 04 customer-records)

- [ ] `inbox.RecordTimelineEvent` + `inbox.SetMessageMetadataKey` (07, 12 callers); `messages.event` names `ai.action_call_completed`, `ai.action_call_approval_requested`. Test: event row shape, no counters/unread change on closed conversation (E253), unknown name rejected (E251), metadata key allowlist + no `message.updated` (E252).
- [ ] `conversations/importing` helpers for 16: `ImportConversations` (number block, preserved timestamps, counters, `ai_state off`, `source.imported`), `ImportMessages` (`tx.CopyFrom` into temp staging table + `INSERT ... SELECT ... ON CONFLICT DO NOTHING`; never COPY into RLS tenant tables, null author for imported member/agent), `ImportContacts` (match order, unverified identities, `TrustedKeys` on `kind user` only), `ImportCompanies`, `ImportTags`, `ImportSavedReplies`, `AttachImportedFile`. Arch test: package imports no `app/events`. Test: re-run idempotent (E256), ambiguous email no match (E257), parallel number blocks (E258), null author only with `metadata.imported` (E259), zero events/jobs/ledger rows after import of fixture tenant.
- [ ] Imported read rules: unread excludes `metadata.imported` (E212); `SendChannelOutbound` + `AssignOnReply` skip imported (E261); filters `filter[imported]`, `filter[import_id]`, `filter[trashed]=only`; search `imported:` + `is:imported` (E260); `channel_connections.type` registry value `import`.

## M10. Search (track 03 inbox-workbench)

- [ ] `support/searchquery` parser (operators, phrases, OR, exclusion, prefix, number/id jump) with fuzz test (no panic, bounded output).
- [ ] Conversation search (candidates, filters, relevance sort, highlight, fuzzy fallback, timeout 503). Test: finds rare word in oldest of 10k seeded, accent-insensitive, stemming, capped meta, highlight escapes HTML.
- [ ] Contact and company search (FTS + trigram + email exact + phone). Test: one-typo name match, email prefix.
- [ ] `GET /search` Cmd+K concurrent sections with partial failure. Test: one section timeout returns others.

## M11. MCP and performance (track 03 inbox-workbench)

- [ ] Expose actions 09 MCP tools call (README MCP table); action-level tests with agent/MCP actor (`Via.Kind = mcp`). Test: actor recorded in `messages.metadata.sent_via`; abilities enforced same as REST.
- [ ] `inbox_benchmark_seeder` (100k conversations, 1M messages, 50k contacts, 200 tags) + Go bench suite for budgets table (list each view/sort, show+include, neighbors, counts, search rare/medium/common, reply write). Accept: all budgets met on CI benchmark runner (4 vCPU, Postgres 18 default config tuned small); report published in CI artifact.
- [ ] Contract migration dropping redundant foundation conversation indexes after bench confirms. Accept: bench unchanged or better.
- [ ] Metrics and logs from Observability section. Test: metric names registered; log redaction test (no bodies at info).

## M12. Docs and API polish (track 03 inbox-workbench)

- [ ] OpenAPI examples for every endpoint (operationId = route name), error code docs page, webhook payload examples for new events (hand to 10). Accept: oasdiff baseline committed; `dimmi route:list` shows all routes named.
- [ ] Add new events to `specs/00-foundation/events-and-realtime.md` catalog in same PR as implementation (catalog rule); document `messages.event` names `announcement.replied` (15), `ai.action_call_completed`, `ai.action_call_approval_requested` (07), `source.announcement_id`, `source.imported` (16) in OpenAPI.

## A1. Conversation access core (track 02 inbox-access)

- [ ] Migrations `20261201020001` to `003`, `Scope` builder, `Scope.Contains` equals SQL function (table test every mode x assignment shape). Replace `VisibleScope{}` in `loaded.go`; existing queries `FilterConversations`, counts, neighbors, search, show go through it. Test: restricted member per mode list, show 404, neighbors, counts, search, number jump, contact conversations.
- [ ] Arch test `tests/arch/visible_scope_test.go` (query naming rule, allowlist with reasons). Test: planted query fails.
- [ ] `PATCH /members/{mem}` fields + `members:manage_access` + member resource fields + audit. Test: self 403, owner 422, team ids rule, no-op no event, ability enforced, token of restricted member, integration token sees all.
- [ ] Realtime split: `workspace_conversations:{ws}`, `conversation_fanout.go` chokepoint, `FanOutToRestrictedMembers`, `conversation.hidden`, `member.access_changed` disconnect. Test (real centrifuge + Valkey): restricted member denied new channel, gets `member:` copies only for visible, reassigned away gets `conversation.hidden` and loses subscription, access change disconnects under 5 s. Update `resources/js/features/realtime` subscription by `conversation_access`. Existing e2e `realtime.spec.ts` still green.
- [ ] Bench: list budgets with `all` unchanged, restricted within 50 ms at 100k rows. Inbox UI reaction: sidebar, 404 page. e2e `tests/e2e/inbox-access.spec.ts`: admin restricts via API, restricted browser sees only team conversations, hidden link no-access page.

Edge 263 to 274.

## A2. Away status, self assign (track 02 inbox-access)

- [ ] Migration `20261201020004`; `SetMemberAvailability`, `PATCH /members/me|{mem}` fields, `members:manage_availability`, `member.availability_changed`. Test: three states, online clears, others' change needs permission and audit, 17 still reads `availability`.
- [ ] Reassign on reply in `ReceiveInboundMessage`. Test: edge 282, 283; auto-generated mail no effect, spam no effect.
- [ ] `assign_on_reply` per member in `AssignOnReply`, reopen and unsnooze rule. Test: edge 287.
- [ ] User menu control `dashboard-user-menu.tsx`, status dot. e2e `tests/e2e/availability.spec.ts`.

## A3. Picker, round robin, balanced, capacity (track 02 inbox-access)

- [ ] Migrations `005`, `006`. `PickAssignee`, cursor, capacity, overflow. Test: fair rotation over 30 arrivals across 3, away skipped, `include_away`, deactivated, 50 concurrent arrivals no double, balanced fewest with ties, channel limit, edge 275 to 281, 288, 289.
- [ ] `AssignOnArrival` listener and `drain_assignment_queue` + periodic, wait reason, timeline row, resource `assignment_wait`, `filter[assignment_waiting]`. Test: capacity freed drains in `queue_order`, `SKIP LOCKED` two workers, explanation row once.
- [ ] `GET /inboxes/{inbox}/assignment-status`. Test: `why_not` each.

Edge 290, 291.

## A4. Inbox settings page (track 02 inbox-access)

- [ ] Replace sample `InboxesSettings`, inbox detail route, assignment controls, status table, seen receipt toggle wired, read only without `inboxes:manage`. Also backend `DELETE /inboxes/{inbox}` with `move_to` (route missing today), `default`. e2e `tests/e2e/inbox-settings.spec.ts`: create, edit reopen window, set round robin, delete moves conversations; member sees read only.

## W1. Schema renumber, priority (track 03 inbox-workbench)

- [ ] Track 03 renumbered migrations above. Backfill `waiting_since`. Test: schema, plan assertions, backfill rows.
- [ ] Migration `20261201030012` priority levels, enum, resource, PATCH, filter, sort, search, events, `case.ts`, row chip, `P` menu. Test: old true becomes high, level validation 422, `true` 422, filter comma, sort order with keyset across pages, event strings, edge 298. e2e in `inbox.spec.ts`.

## W2. Views on screen, status, sort, row (track 03 inbox-workbench)

- [ ] `filter[view]` forms `team:`, `member:`, `inbox:`, `view-route.ts`, sidebar sections, counts additions, team `Select` in details panel. Test: API each view, restricted variant; vitest `view-route`. e2e `tests/e2e/inbox-views.spec.ts`: team view lists team's, assign team in panel moves it.
- [ ] Migration `016`, `/inbox-preferences`. Status control, sort control, persist per view. Test: all sorts + cursor, 64 KB limit, realtime tab sync. e2e: chosen sort survives reload.
- [ ] Row: channel, assignee, priority, tags, waiting, viewers. `PresenceMany` in `framework/broadcasting` (generic, covered per framework rule), `include=viewers`, `conversation.viewers_changed`, setting. Test: two members real centrifuge; deadline omit; setting off. e2e two browsers.

## W3. Custom views (track 03 inbox-workbench)

- [ ] Migration `015`, definition compiler + registry, CRUD, order, `views:manage_shared`, `filter[view]=cvw_`. Test: every field and op on seeded data, depth rule, 20 conditions, injection attempts stay data, deleted reference warning, cursor hash, limits, other's personal 404. Fuzz compiler: no panic.
- [ ] View editor UI, Save as view, pin in preferences, duplicate. e2e.

## W4. Keyboard, command menu, sheet (track 03 inbox-workbench)

- [ ] Action registry + `tinykeys` wrapper + `J`/`K` with neighbors and prefetch + all table keys. Vitest: input guard. e2e `tests/e2e/inbox-keyboard.spec.ts`: walk list, snooze, close by keys, no spinner on prefetched.
- [ ] Command menu with search results (I-101): sections, debounce, jump. e2e `tests/e2e/command-menu.spec.ts`: find contact by email, open conversation by `#number`, run close.
- [ ] Shortcut sheet, practice page. e2e one path.

## W5. Snooze (track 03 inbox-workbench)

- [ ] Picker, `/snooze-options`, `case.ts`. Migration `017`, `/snooze-lists`, `snooze_option` on PATCH, reply, bulk, resolution order, settings section. Test: each kind with frozen clock + DST, stale key 422, 10 options, empty 422, team override. e2e pick custom time, wakes (builds on F31 job).
- [ ] Wake rules and settings, new reasons. Test: each key on and off; `then: snooze` stays; assignee note no wake (edge 309).

## W6. Composer: send, drafts, formatting (track 03 inbox-workbench)

- [ ] Send split button, `then`, shortcuts, auto advance. e2e: send and close moves to next.
- [ ] Migration `013`, drafts API + `draft.updated`, reply and create consume, orphan purge exclusion, `purge_stale_drafts`. Test: per mode, per member, other member 404, last write wins, scope loss 404, edges 310, 312, 314. E209 still 422.
- [ ] `use-draft`, local cache, flush points. e2e `tests/e2e/inbox-drafts.spec.ts`: type, navigate, reload, second context sees it; reply and note kept apart; send clears.
- [ ] Tiptap composer: lazy chunk, fallback textarea, toolbar, shortcuts, round-trip vitest table for every subset node. e2e bold via `Cmd+B` reaches stored Markdown; pasted HTML reduced. Bundle size note in PR.

## W7. Undo send (track 03 inbox-workbench)

- [ ] Migration `014`, settings + member field, `delay`, scheduled job, `metadata.undo`, undo path, exclusions from timeline, search, counters, billing `final` failed. Test (frozen clock, fake driver): email delayed, widget not, token no delay, undo reverses per field (edges 315 to 318), job after undo no send, restart sends. e2e with Mailpit: undo means no mail; no undo means mail after window.

## W8. Thread and message moves (track 03 inbox-workbench)

- [ ] Delete and edit UI, moderate on others' replies, `MessageEraser` registry + `erase_message_copies` + tombstone. Test: raw copy hook called, retries, attachments gone, realtime. e2e `inbox-messages.spec.ts`.
- [ ] Inline images (builds on F14, F15): tokens, `LinkAttachments`, renderer, fake driver asserts `Inline` + `ContentID`; 04 renders `cid`. e2e: paste image, Mailpit shows related part.
- [ ] Emoji, GIF: emojibase chunk, instance key, CSP conditional, picker, host rule. Test: key absent no CSP entry, remote image other host stripped. e2e with routed stub for `api.giphy.com`, no real call.
- [ ] Thread extras: lightbox, code block, time zone toggle, hide events, date hover. Vitest + e2e.
- [ ] Seen indicator from `read_at`. Test: widget read shows; email never (X12).

## W9. Mentions, reactions (track 03 inbox-workbench)

- [ ] Mentions write side built: parse, `message_mentions`, team expand, setting, `/mentionables`, view join with scope, unread filter, counts. Test: team expand, setting off, hidden member no row in view, deactivated ignored, edit adds only new (E84).
- [ ] Mention typeahead, chip render. e2e `tests/e2e/inbox-mentions.spec.ts`.
- [ ] Migration `019`, reactions API, resource, event, UI. Test: notes only 422, limit, repeat no-op, emoji rule.

## W10. Start conversation (track 03 inbox-workbench)

- [ ] Dialog, draft `new`, From list, To search and create, send separately loop, then options. Backend `to`, `cc`, `bcc` accepted once R6 lands (dialog hides Cc, Bcc until then). e2e `inbox-new-conversation.spec.ts` with Mailpit.

## W11. Bulk (track 03 inbox-workbench)

- [ ] Migration `018`, sync and 202 branch, `run_bulk_operation` chunks, resume, one per workspace, cancel, recheck scope, `/bulk-operations`, bulk reply and note. Test: 100 sync, 101 job, 1,001 422, 10,001 filter 422, crash resume, cancel, billing block continues, edges 332 to 336, audit.
- [ ] Bulk bar, selection, select all matching, progress. e2e `inbox-bulk.spec.ts`: filter 150 seeded, confirm, job completes, list updates.

## W12. Search (track 03 inbox-workbench)

- [ ] Parser operators, resolution, `any:`, fuzz. Filter popover. Relevance built, sort control. Test: operators against seeded, relevance beats recency on rare phrase, combination with status. e2e `inbox-search.spec.ts`.
- [ ] `POST /conversation-searches`. Test: same grammar errors, scope, cursor, timeout 503.

## R1. Tags (track 04 customer-records)

- [ ] Tag actions, PATCH fields, routes, events, filter (`tag`, `tag_any`), migration `006`, picker, `Shift+T`, `TagsSettings` real. Test: per M8 tags item + message link. e2e `tags.spec.ts`.
- [ ] Migration `005`, contact and company tags, filters, merge union, erase. Test: follow person, cap 50.

## R2. Customer panel (track 04 customer-records)

- [ ] Context endpoint complete, panel split, pinned attributes, past conversations 3 to 20, edit in place. Test: context scope filter, one query per section. e2e `inbox-panel.spec.ts` light and dark.
- [ ] E-05 section against 04 API (needs track 05 suppression API; section hidden until route answers 404). Duplicate banner.

## R3. Directory (track 04 customer-records)

- [ ] Routes, table, filters, profile, new contact, blocked hidden by default. Migration `007` notes. e2e `customers.spec.ts`: search, edit attribute, view conversations.

## R4. Companies (track 04 customer-records)

- [ ] `companies` actions and routes per M2 item + `/companies/{id}/conversations`, pages, panel, `company:` filter. Test: scope, delete nulls contacts. e2e.

## R5. Merges (track 04 customer-records)

- [ ] Migrations `001` to `003`, `MergeConversations` per M9 item + setting + scope + redirect, merge dialog. Test: edges 338 to 341, 122 to 126, threading follows chain with fake channel. e2e `inbox-merge.spec.ts`.
- [ ] `MergeContacts` per M9 item (builds on F36) + setting + finder reasons + dialog. Test: edges 342, 343, 105 to 113, X8 widget rule unchanged.

## R6. Participants (track 04 customer-records)

- [ ] Migration `004`, API, reply fields, `OutboundMessage.To`, `CC`, `BCC`, `ParticipantsBCC`, event row, non-participant flag. Test: changes apply on send, bcc never in resource, limit, edges 344 to 349; fake driver asserts recipients. 04 builds on F16 for headers.
- [ ] Recipients row, header popover, draft staging. e2e with Mailpit To, Cc, Bcc envelope.

## R7. Spam, block, trash (track 04 customer-records)

- [ ] Spam folder UI, not spam, trust offer (hidden until 04 trusted sender API), block dialog, blocked tab. Test: builds on F37, F61; e2e `inbox-spam.spec.ts`.
- [ ] `RestoreConversation`, `conversation.restored`, `DELETE` route, deleted view, thread conflict. Test: edges 384, 385, bulk restore. Reactivates rest.md `conversation-restore`.

## R8. Attachment safety (track 04 customer-records)

- [ ] Migration `008`; `ScanAttachment` calls `a.Malware`; verdict table; retries; download gate + grants; outbound gate; widget filter; event; badges. Test with `Malware.Fake()` (true boundary, framework drivertest proves drivers); EICAR through real path with fake verdict; edges 360 to 366. Lands with F14 route.
- [ ] Policy settings, `policy.Allowed`, `conversation-safety.tsx` page. Test: customer upload off, extra extension, hard block 422, edge 367, 368.
- [ ] Link lifetimes: 5 min, widget 30 min, dead after delete, member check on `GET /attachments/{att}` by scope. Test: frozen clock expiry, edge 369.

## R9. Redaction, link safety (track 04 customer-records)

- [ ] `app/support/redaction`, built-ins, migration `009`, CRUD, tests endpoint, hooks in every write path, metadata badge, raw drop signal to 04. Test: Luhn table, last 4, SSN off default, custom regex, subject, html, reply, note, edge 370 to 376; no pre-redaction text in logs (log capture). Fuzz: no panic.
- [ ] Settings, `safe-link.tsx`, warning, block, strip option. Vitest heuristics; e2e `conversation-safety.spec.ts`.

## R10. Erase, export one customer (track 04 customer-records)

- [ ] Migration `010`, grace flow, stash encrypted, cancel, inbound in grace, immediate confirm, hooks per M9 item (+ `MessageEraser` scope, drafts, reactions), scope refusal. Test: edges 379 to 381, 115, 116, 219, registry test.
- [ ] Export zip per safety.md, profile UI. e2e `customer-privacy.spec.ts`: export ready, erase with immediate, thread shows Deleted contact, cancel path.

## R11. Retention, conversation export (track 04 customer-records)

- [ ] Retention settings, `apply_conversation_retention`, per-workspace values read by purge jobs and 04 pruner. Test: frozen clock, guard false, batching, edge 382, 383.
- [ ] Migration `011`, `/data-exports`, txt, csv, json, `framework/pdf` wrapper + fpdf + font (depguard added by coordinator), notes rule, redaction rerun, scope. Test: formats golden, limits 422, scope drop, missing glyph warning, expiry. Framework unit tests 90%. e2e `exports.spec.ts` from view and selection.

## X1. Away reasons, automatic away (track 12 inbox-extras)

- [ ] Migration `20261201120001`, CRUD, required setting, activity endpoint, `auto_away_members`, return rule, UI. Test: edges 284 to 286 frozen clock + real Valkey. e2e in `availability.spec.ts`.

## X2. Custom attributes (track 12 inbox-extras)

- [ ] Migration `004` (`conversation_attribute_definitions`, `cad_`), definitions CRUD, value validation, PATCH + bulk, required to close incl. `required_when`, filter, sort, search, snooze wake, panel editors, settings page. Test: each type, edges 350 to 354. e2e `attributes.spec.ts`: required blocks close, fill, close.

## X3. Folders, table layout (track 12 inbox-extras)

- [ ] Sidebar editor, folders, hide, pin. e2e.
- [ ] Table layout, columns, preview, `L`, includes. e2e `inbox-table.spec.ts`, phone width falls back to list.

## X4. Side conversations, similar (track 12 inbox-extras)

- [ ] Migrations `002`, `003`, actions, API, send job, `ReceiveSideMessage`, parent effects, UI. Needs track 06 inbound ref lookup. Test: never in `MessageResource`, widget, webhook, agent context, edges 396 to 399. e2e Mailpit round trip via `dimmi email:test-inbound`.
- [ ] `/similar`, panel section, sample `library.ts` removed. Test: rare words, 180 day limit, scope, edge 400.

## X5. Translation (track 12 inbox-extras)

- [ ] Migration `005`, endpoints, `metadata.translation`, `app/drivers/llm/fake` scripted, spend cap, delimiters, UI. Test: cached, same language no call, no key 422, provider error 502, edges 401 to 405. Never real LLM; CI has no key.

## X6. Scheduled export (track 12 inbox-extras)

- [ ] Migration `006`, destinations with encrypted credentials, tests endpoint, schedules, JSONL gz incremental, alert via 06. Test against real SeaweedFS: objects, manifest, cursor, edge 388. e2e settings flow.

## Done when

- `go test ./...` green: feature tests for every endpoint (happy, validation, auth, other workspace 404, ability, idempotency), contract suite passes for fake + api drivers, bench budgets met.
- Demo script (fresh `docker compose up` (app + Postgres + Valkey), seeded demo workspace, two browser sessions as Ada and Bob using REST via `curl` + centrifuge-js test client or dashboard):
  1. `POST /contacts?upsert=true` with `external_id=user_42`; `POST /conversations` contact-authored on api channel: conversation `#1` appears in Unassigned via realtime within 300 ms.
  2. Bob opens it (presence on `conversation:{conv}` shows Bob to Ada).
  3. Ada replies with `then: "keep"`: conversation auto-assigned to Ada; Bob's stale reply with old `expected_last_message_id` gets 409 `new_messages_since`.
  4. Ada adds note `<@mem_bob>`: Bob's Mentions count goes to 1 via `member:{mem}`.
  5. Ada sends `PATCH /conversations/{id} {status: "snoozed", snooze_preset: "tomorrow"}` (`clock` advanced in test mode); conversation returns to open with `conversation.unsnoozed` reason `expired`.
  6. Ada closes and reassigns to Bob in one `PATCH {status: "closed", assignee_id: "mem_bob"}` (events assigned then closed); developer posts contact message via `/messages`: conversation reopens (`customer_message`). Advance clock 31 days after close: next message creates `#2` with `previous_conversation`.
  7. Fake channel conversation: reply goes out, fake driver scripted 429 then success; message shows `pending` then `sent`; scripted permanent error on next reply shows `failed` + retry works.
  8. `GET /search?q=rückerstatung` (typo, accent) finds seeded conversation via fuzzy subject; `#1` jumps to conversation.
  9. Merge duplicate contacts (`POST /contacts/{user}/merges {from: [lead]}`, lead with same email): conversations move, lead widget identity dropped, `contact.merged` emitted.
  10. `PATCH /conversations {ids: [3 ids], changes: {status: "closed"}}`, 1 from other workspace: per-id results show 2 `updated`, 1 `failed` `not_found`. Filter bulk spam on widget conversations: first call 422 `confirmation_required` with count, second with `confirm_count` updates all.
  11. `dimmi_send_reply` MCP call (09) without confirm returns preview, nothing sent; with confirm sends.
  12. Browser only: from `/d/inbox`, `G` `U` opens Unassigned, `J`, `J`, `R`, type, leave and return (draft there), `Cmd+Shift+Enter` sends and closes after 5 s undo window; `Cmd+K` finds contact by email.
  13. Restrict Bob to team Billing: Bob sees only Billing, deep link elsewhere no-access, counts exclude, socket stops carrying others.
  14. Set inbox round robin capacity 1: two arrivals go to Ada, Bob; third waits with reason; Ada closes one, third assigned.
  15. Inbound with card number and EICAR attachment (fake verdict): body masked; file removed. Erase contact with 7-day grace, cancel; erase immediately; panel shows Deleted contact.
