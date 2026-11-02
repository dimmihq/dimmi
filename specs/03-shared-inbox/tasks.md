# 03 Shared inbox: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. Requires foundation tasks (core migrations, events, `framework/broadcasting` on centrifuge + Valkey, querybuilder, idempotency) and 00-accounts members/teams.

Tests: business logic = integration only (real App, Postgres, Valkey, River, centrifuge node; testing.md): HTTP feature tests first, action/job entry with real App second. No unit tests with mocks for app code. Pure parsers in `app/support` (searchquery, templating) get fuzz tests in addition, never mocks. Every edge case has a test, named in plain words.

## M1. Schema and models

- [ ] Migration `20261101000301` search config (`btree_gin`, `dimmi_search`, `dimmi_unaccent`). Accept: `SELECT to_tsvector('dimmi_search','Rückerstattungen')` equals unaccented stem; `dimmi doctor` checks `btree_gin`. Test: migration test (up, down, up) + doctor check integration test.
- [ ] Migrations `302`, `303`, `305`, `306`, `309`, `310`, `314` (core table changes). Accept: up/down on empty DB; `sqlc generate` clean; tenant lint passes for pivots with `workspace_id`. Test: schema snapshot test.
- [ ] Migration `304` list indexes (CONCURRENTLY, NO TRANSACTION) + `307`, `308`. Accept: `EXPLAIN` of each list query in fixture uses intended index (test asserts plan node name on seeded 10k rows). 
- [ ] Migrations `311` to `313` new tables + queries files skeleton (`database/queries/*.sql` for every table in code map). Accept: sqlc compiles; every query has `workspace_id` predicate.
- [ ] Enums, prefixes `rpl_`, `exp_` in `app/models/public_ids.go` (`framework/ids`), typed JSON (`InboxSettings`, `ConversationSource` incl. `announcement_id` 15, workspace settings keys declared in foundation settings registry). Test: integration round trip through endpoints (encode/decode, settings defaults).
- [ ] Registry enums `enums.IdentityType`, `enums.ChannelType` over foundation registry (P1): 03 registers `email`, `phone`, `widget_visitor`, `external_id`, `api`; request rules and actions validate against registry. Test: unknown type 422 in request, action rejects unknown type, registering new type at boot needs no migration (test registers fake type, inserts row).
- [ ] 03 permissions in 00-accounts catalog (`conversations:delete`, `conversations:moderate`, `contacts:erase`, `contacts:manage_external_id`, `companies:delete`, `inboxes:manage`, `tags:manage`, `saved_replies:manage_shared`) with ability groups; every 03 policy uses `authz.Can`. Test: table test role x permission matches Permissions matrix; lint no `Role ==` in `app/policies/*` 03 files.
- [ ] Factories for all tables in code map incl. `fake` channel connection; `inbox_demo_seeder`. Accept: `dimmi db:seed --class=InboxDemoSeeder` produces 200 conversations in all states.

## M2. Inboxes, contacts, companies

- [ ] `EnsureDefaultInbox` (+ `api` connection) registered as first workspace initializer (contracts.md hook, inside `CreateWorkspace` tx, idempotent for `dimmi workspace:repair`); inboxes CRUD endpoints, policy, resource, `inbox.*` events. Test: feature (CRUD, default delete 409, non-empty delete needs move_to, other workspace 404, member 403).
- [ ] Contacts CRUD + list filters + custom attribute validation + phone/email normalization; `contact.updated`/`company.updated` carry `changes` + `changed_attribute_keys`. Test: feature per endpoint, validation (key format, 100 keys, 1024 chars), cross-tenant; changed keys only (E247).
- [ ] `GET /contacts?filter[segment]=seg_...` via 15 segment compiler (fake compiler until 15 lands), 5 s statement timeout, 422 `segment_too_slow`. Test: segment filter combines with others, slow segment 422, other workspace segment 400 (E244).
- [ ] `UpsertIdentifiedContact` (renamed from `IdentifyContact`; 02 owns `identity.IdentifyContact`) + `POST /contacts?upsert=true` per api.md algorithm (match `external_id` then `email`, 201/200 + `meta.upserted`); written `custom_attributes` keys added to `contacts.trusted_attribute_keys` (also member/token `PATCH /contacts`), merge keeps source trust for copied keys. Test: feature table test for every branch (external_id hit, email hit user, unverified lead skipped, create, immutable external_id 409, merged id follows chain); feature: no match key 422 `upsert_key_required`, `upsert=yes` 400, plain create 409 `contact_exists`; trusted keys added on upsert and PATCH, null removes (E254); merge trust rule (E255).
- [ ] Identities list/delete, block/unblock as PATCH `blocked` field (field-dispatch in `ContactController.Update`, `contact.blocked`/`unblocked`), duplicates finder (same email/phone/name+company heuristics). Test: feature (no-op on same value, events order with attributes).
- [ ] Companies CRUD + `POST /companies?upsert=true` (`external_id` then normalized `domain`) + company contacts. Test: feature.

## M3. Conversations core

- [ ] Number allocation + `CreateConversation` (contact-authored api channel, member outbound with `StartConversation` check) + `POST /conversations`. Test: concurrent creation 50 goroutines yields unique gapless numbers; channel_cannot_initiate.
- [ ] `ConversationResource` (full, compact, list row), show endpoint with `include=messages,context`, number lookup, ETag/304; includes `csat` (15) and `feedback_requests` (14) via owner query functions (fakes until owners land); context renders `announcement` (15) from `source`. `imported` block (16) from `source.imported`, imported message author `former_teammate`. Test: snapshot JSON, 304 path, include batching (one query per include per page), E245, E246, E259.
- [ ] State machine actions (business names kept): close, reopen (thread_conflict), snooze (presets, DST, limits), unsnooze, transition table returning 422 `invalid_transition` `{from, to}`, `ScheduleUnsnooze` + `unsnooze_conversation` job + `reconcile_snoozes`. Test: feature for `snooze_preset` resolution with frozen `tc.Clock` (DST spring/fall, Sunday/Monday, no timezone), feature for each transition + no-op cases, job no-op when stale.
- [ ] Assign + assign-on-reply helper + `UnassignMemberConversations` + `UnassignTeamConversations` + queued listeners on `member.deactivated` (reassign hint) and `team.deleted` (null `inboxes.default_team_id`, then `teams.PurgeTeam`). Test: concurrency (two replies, single assign), batch 2,000 unassign events count, reassign target deactivated meanwhile falls back to unassign, closed conversations keep assignee.
- [ ] Update attributes (subject, priority, inbox move), spam mark/unmark (+ `block_contact`), soft delete (`DELETE` and PATCH `trashed: true`; `trashed: false` 422), purge jobs calling `RetentionGuard` (`PurgeConversation`, spam purge, orphan attachments). Test: feature + job tests with `tc.Clock.Freeze`; fake guard returning false per job keeps rows, metric incremented (edge cases 217, 218).
- [ ] `ApplyConversationChanges` field-dispatch + `UpdateConversationRequest` + `ConversationPolicy.UpdateField` + `ConversationController.Update`: fixed order (trashed, spam, assignment, status, ai_state, attributes, tags, unread), one row lock, one tx, no-op skip, `meta.changed`, `ai_state` handed to 07 actions (fake until 07 lands). Test: table test per field (event emitted, no-op no event), combined close + assign event order, key order irrelevant, one failing field rolls back all, forbidden field 403 before work, `trashed: true` alone rule, side params without field 422, merged source 409, trashed 409/404, `invalid_transition` body has `from`/`to`.
- [ ] Timeline event rows for every state change (`messages.type = event`). Test: each action asserts event row shape.

## M4. List, neighbors, counts, read state

- [ ] `conversations.VisibleScope(principal)` single visibility builder (MVP: workspace; trash needs `conversations:delete`) used by list, show, neighbors, counts, search, bulk filter, contact/company conversation lists, exports, realtime authorizers. Test: fake scope hiding one conversation hides it from every surface (table test over surfaces); show outside scope 404.
- [ ] `querybuilder/conversations.go`: views, filters, sorts, keyset cursor with filter hash. Test: feature for each filter/sort combo on seeded data (rows + order, `EXPLAIN` uses intended index); pagination stable under inserts, `cursor_mismatch`, invalid filter 400.
- [ ] List endpoint with batch includes (contact, assignee, tags, last_message preview, unread). Test: query count assertion (max 5 queries per page).
- [ ] Neighbors endpoint. Test: prev/next at edges, after close still resolves.
- [ ] Read state actions + `GET`/`PATCH /conversations/{id}/read-state` singleton (`last_read_message_id` or `unread`) + PATCH `unread` field + author auto-read + `conversation.read` broadcast (`member:{mem}` only) + widget-safe `conversation.seen_by_team` (`widget_contact:{ctc}`, `{conversation_id, seen_at}`, name only with `seen_receipt_shows_name`). Test: forward-only, mark unread semantics, both keys 422, other conversation message 422, widget payload never has member id.
- [ ] Counts endpoint (capped counts, Valkey cache with generation counter) + `BroadcastUnreadCounts` coalesced 250 ms to `member:{mem}`. Test: cap at 999 (E95), cache bust on event across two App instances (E94), member-scoped channel payload, Valkey flush recomputes.
- [ ] Mentions view (`message_mentions`) filter. Test: team mention expands to members.

## M5. Messages and channel contract

- [ ] `contracts.Channel` + `ChannelManager` + `fake` driver + `api` driver + contract test suite `tests/contracts/channel_test.go` (capability flags match interfaces, success returns ChannelMessageID, errors wrap sentinels, honors ctx deadline). Accept: fake and api pass suite.
- [ ] Attachments: upload (sniff, limits, blocked types), show (signed URL), delete, web download controller, `LinkAttachments` targets `message` / `feedback_post` / `feedback_comment` (14), orphan purge job (any target column = attached), `ScanAttachment` job + `attachments.scan_driver` (`none`, fake). Test: feature + signed URL expiry + SVG served as attachment; infected attachment never downloadable, outbound with infected attachment fails `attachment_infected`; other workspace `att_` cannot be linked to message (404); feedback-linked upload survives purge (E242); double link 409 (E243).
- [ ] `ReplyToConversation`: routing rule (skips `import` connections), `ChannelRehomer` call for 16 `import` connections (422 `import_channel_cannot_send`), strict body keys (`draft_id` 422 `unknown_field`, E209), capability validation, collision guard, `client_message_id` dedupe, `then`, auto-assign, counters, `waiting_since`, substantive rules, `message.created` fields, `conversation.created.InitiatedBy`, 12 billing gate (InTx `ErrActivationBlocked` -> 423 with `billing` block, `ErrOutboundFairUse` -> 429). Test: feature per rule; 201/200 replay/409/422 cases; InTx listener returning block gives 423 and rolls back message + assign; inbound with same listener still stored; rehome via fake `import` driver (E210, E211); `then: snooze` on closed 422 `invalid_transition`.
- [ ] `SendSystemMessage` (15 CSAT): system author, non-substantive, metadata allowlist `survey_request`, outbound pipeline, no state effects, no 07 trigger. Test: action entry with real App + fake channel driver: E234 to E237; billing listener sees `Substantive = false`.
- [ ] `AddNote` + `EditNote` with mentions parse/render (`support/mentions`, `<@mem_…>`), `message_mentions` rows in same tx, edit diff -> `message.updated.AddedMentionedMemberIDs`. Test: invalid mentions ignored, notes never in widget resource, edit adds only new ids, reply edit 422 `message_not_editable` (E79).
- [ ] `SendChannelOutbound` listener + `send_channel_message` worker (guards 1 to 6, result handling table, ordering wait, snooze on rate limit) + `UpdateMessageDelivery` monotonic. Test: fake driver scripted: success, transient x3 then success, permanent, rate limit, connection broken (connection status error), ordering of two messages, deleted before send, retry window end final failure event `final = true` (`Capabilities.RetryWindow` 0 = default about 3 h; fake driver declaring 24 h still pending at 23 h, failed after 24 h, frozen clock).
- [ ] Timeline list (before/after/around, `updated_since`, 10 s overlap), message show. Test: cursor both directions, around anchor, late-commit overlap.
- [ ] Delete (own reply: pending cancels send, sent local only; notes; admin redact), `POST …/messages/{id}/delivery-attempts` (202 `delivery_attempt`). Test: feature per rule; sent push-channel reply delete makes no driver call (E80); non-failed 409 `message_not_failed`.
- [ ] Contact message endpoint for api channel. Test: 409 on non-api, reopen path.

## M6. Inbound

- [ ] `ReceiveInboundMessage` per channel-contract.md algorithm (dedupe, resolve contact, pick conversation incl. reopen window, merged chain, spam decision, state effects, participants, `AfterPersist` hook step 9b). Test: table-driven integration tests per step (action entry with real App, fake channel driver); concurrency test dedupe race; window boundary equal; `AfterPersist` receives same tx (row visible inside, rolled back on hook error, no message/dedupe row left, retry ingests once); hook not called on `Duplicate`.
- [ ] `ResolveInboundContact` + `AutoMergeOnVerifiedIdentity` + verified-claim takeover (channel-contract.md section 4; 14 portal, 04 DMARC, 02 JWT). Test: verified vs unverified, external_id conflict blocked, survivor choice, takeover moves unverified row without merge (E239), verified row never taken over (E240), unverified claim no takeover (E241).
- [ ] `FindConversationByChannelMessage` helper, `app/support/spam` heuristics, UA parse via `framework/http/useragent` (medama) at ingest. Test: integration through fake-channel ingest (spam score, `source.browser/os/device`).
- [ ] Fake-channel end-to-end test: inbound, reply, delivery receipt, customer reopen after close, new conversation after window.

## M7. Collision and realtime

- [ ] `conversation:{conv}` subscribe authorizer (presence + join/leave, history on, client publish off; `ClientInfo` `mem_` id, name, avatar) + `member:{mem}` authorizer, both via `VisibleScope`. Test (real centrifuge node + Valkey): other workspace denied (E99), outside scope denied (E226), deactivated disconnected (E156), two tabs one presence member (E230).
- [ ] `RecordTyping` + `POST /conversations/{id}/typing-indicators` (202): Valkey throttle, relay to `widget_contact:{ctc}` + `send_channel_typing` job, nothing published on `conversation:{conv}`. Test: note composer not relayed (E101), throttle (E232), job uniqueness 4 s, email conversation no-op (E208).
- [ ] `client_mutation_id` echo on all events; workspace list payloads use list-row resource. Test: centrifuge-js Node script receives `message.created` on `conversation:{conv}` with `meta.client_mutation_id`; reconnect with offset recovers (E104); Valkey restart gives `recovered: false` (E231).

## M8. Tags, saved replies, bulk, participants

- [ ] Tags CRUD, apply/remove by id or name, limit 50, `tag.*` events, delete cascade single event. Test: feature, case-insensitive collision.
- [ ] Saved replies CRUD (personal/shared, policy), `support/templating` with fallback filter, `POST /saved-replies/{id}/renderings`, `missing_variables`. Test: feature via `POST /saved-replies/{id}/renderings` (unknown var blank, unbalanced braces literal, HTML escape mode, CJK first_name) + fuzz on renderer (no panic).
- [ ] Bulk `PATCH /conversations` (`ConversationController.UpdateMany`, `BulkUpdateConversations`): `changes` = single-PATCH fields validated once, per-field permission once, per-id tx via `ApplyConversationChanges`, rule 8 result shape (`updated`/`skipped`/`failed`). Test: mixed success, other workspace ids `not_found`, member `trashed` 403, 101 ids 422, duplicate ids deduped, re-run gives `skipped`.
- [ ] Filter-based bulk: list filter keys reused, `q` rejected, two-step `confirm_count` (422 `confirmation_required`, 409 `bulk_count_mismatch`), cap `inbox.bulk.max_filter_matches` 1,000, id snapshot, meta-only response with `errors`, audit log. Test: count drift between steps, 1,001 matches 422, snapshot item closed mid-run `skipped`, audit row written.
- [ ] Participants add/remove + `conversation.participants_changed`. Test: limit 50, channel without CC 422.

## M9. Merge and GDPR

- [ ] `MergeContacts` (manual, `POST /contacts/{target}/merges`, 201 `contact_merge`, 202 over 50k conversations) + `contact_merges` log + widget identity drop for unverified lead + chain flatten. Test: field precedence table, 409 external_id conflict, concurrent opposite merges no deadlock.
- [ ] `MergeConversations` + `POST /conversations/{target}/merges {from: [...]}` (max 10, all-or-nothing, UUID lock order) + one `conversation.merged` per source + inbound follow chain. Test: messages moved, tags capped 50, target closed stays closed, pending outbound still sends, `merge_self` 422, live into imported target 422 `merge_imported_target` (E262), source merged elsewhere 409 aborts all, already-merged-into-target `skipped_ids`, 201 `conversation_merge` body.
- [ ] Contact export job (`POST /contacts/{id}/exports` 202; zip JSON + attachments, signed URL 24 h, expiry job) + erase job (security.md rules, tombstone contact, `RetentionGuard` first: held = `pending_hold`) + contact eraser hook registry (README Security; registered 01, 04 to 10, 13 `EraseChangelogContactData`, 14 `EraseFeedbackContactData`, 15 `EraseEngageContactData`) + contact export hook registry (`ExportContributor`: 13 optional, 14, 15). Test: export contents, erase wipes bodies/attachments/identities and runs every registered hook (fake hooks; one failing hook retries job, erase not marked complete), registry test lists every spec with contact PII, conversation shells kept, audit row; fake guard false leaves data, audit `pending_hold`, no hooks run (edge case 219); export of other workspace contact 404; export signed URL expires; 13/14/15 hooks run and fail-retry (E248); export sections per hook (E249).

## M9b. Cross-spec helpers (07 agent actions, 16 importers)

- [ ] `inbox.RecordTimelineEvent` + `inbox.SetMessageMetadataKey` (07, 12 callers); `messages.event` names `ai.action_call_completed`, `ai.action_call_approval_requested`. Test: event row shape, no counters/unread change on closed conversation (E253), unknown name rejected (E251), metadata key allowlist + no `message.updated` (E252).
- [ ] `conversations/importing` helpers for 16: `ImportConversations` (number block, preserved timestamps, counters, `ai_state off`, `source.imported`), `ImportMessages` (`tx.CopyFrom` into temp staging table + `INSERT ... SELECT ... ON CONFLICT DO NOTHING`; never COPY into RLS tenant tables, null author for imported member/agent), `ImportContacts` (match order, unverified identities, `TrustedKeys` on `kind user` only), `ImportCompanies`, `ImportTags`, `ImportSavedReplies`, `AttachImportedFile`. Arch test: package imports no `app/events`. Test: re-run idempotent (E256), ambiguous email no match (E257), parallel number blocks (E258), null author only with `metadata.imported` (E259), zero events/jobs/ledger rows after import of fixture tenant.
- [ ] Imported read rules: unread excludes `metadata.imported` (E212); `SendChannelOutbound` + `AssignOnReply` skip imported (E261); filters `filter[imported]`, `filter[import_id]`, `filter[trashed]=only`; search `imported:` + `is:imported` (E260); `channel_connections.type` registry value `import`.

## M10. Search

- [ ] `support/searchquery` parser (operators, phrases, OR, exclusion, prefix, number/id jump) with fuzz test (no panic, bounded output).
- [ ] Conversation search (candidates, filters, relevance sort, highlight, fuzzy fallback, timeout 503). Test: finds rare word in oldest of 10k seeded, accent-insensitive, stemming, capped meta, highlight escapes HTML.
- [ ] Contact and company search (FTS + trigram + email exact + phone). Test: one-typo name match, email prefix.
- [ ] `GET /search` Cmd+K concurrent sections with partial failure. Test: one section timeout returns others.

## M11. MCP and performance

- [ ] Expose actions 09 MCP tools call (README MCP table); action-level tests with agent/MCP actor (`Via.Kind = mcp`). Test: actor recorded in `messages.metadata.sent_via`; abilities enforced same as REST.
- [ ] `inbox_benchmark_seeder` (100k conversations, 1M messages, 50k contacts, 200 tags) + Go bench suite for budgets table (list each view/sort, show+include, neighbors, counts, search rare/medium/common, reply write). Accept: all budgets met on CI benchmark runner (4 vCPU, Postgres 18 default config tuned small); report published in CI artifact.
- [ ] Contract migration dropping redundant foundation conversation indexes after bench confirms. Accept: bench unchanged or better.
- [ ] Metrics and logs from Observability section. Test: metric names registered; log redaction test (no bodies at info).

## M12. Docs and API polish

- [ ] OpenAPI examples for every endpoint (operationId = route name), error code docs page, webhook payload examples for new events (hand to 10). Accept: oasdiff baseline committed; `dimmi route:list` shows all routes named.
- [ ] Add new events to `specs/00-foundation/events-and-realtime.md` catalog in same PR as implementation (catalog rule); document `messages.event` names `announcement.replied` (15), `ai.action_call_completed`, `ai.action_call_approval_requested` (07), `source.announcement_id`, `source.imported` (16) in OpenAPI.

## Done when

- `go test ./...` green: feature tests for every endpoint (happy, validation, auth, other workspace 404, ability, idempotency), contract suite passes for fake + api drivers, bench budgets met.
- Demo script (fresh `docker compose up` (app + Postgres + Valkey), seeded demo workspace, two browser sessions as Ada and Bob using REST via `curl` + centrifuge-js test client until dashboard exists):
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
