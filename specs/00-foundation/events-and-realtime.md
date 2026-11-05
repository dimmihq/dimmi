# Events and realtime

Style: caveman. Scope trim 2026-11-02: dropped `conversation.restored` (later: `conversation-restore`), `reply_draft.*` (later: `reply-drafts`), `survey.summary_generated` (later: `survey-ai-summary`); `message.updated` = note edits; teammate typing relay later. One event catalog. Every state change is a domain event. Events feed realtime, webhooks, notifications, audit log, metering, AI agent triggers. Nothing else invents its own change feed.

## Dispatch model

```
action (in tx) -> events.Dispatch(ctx, tx, evt)
   |- InTx listeners: run now inside tx, may abort action (billing gate, audit log)
   |- Queued listeners: River jobs inserted in same tx (InsertTx)
   |- AfterCommit listeners: run after commit (realtime broadcast, cache bust)
```
Details in `contracts.md` "Event dispatch phases".

- Rolled-back transaction: no listener runs, no job exists.
- Each event struct carries: `ID` (evt_ UUIDv7), `WorkspaceID`, `OccurredAt`, `Actor` (type + id), subject IDs, and change data (`from`/`to` for state changes). Never full models. Listeners re-fetch current state.
- Webhook payload snapshot: built once by fan-out job (seconds after event) from Resource, stored on delivery row. Retries send identical body (see 10-webhooks).
- Event names dotted lowercase, past tense: `conversation.assigned`.

## Catalog (MVP)

| Event | Emitted by | Realtime | Webhook | Notes |
|---|---|---|---|---|
| conversation.created | 03, channels | yes | yes | |
| conversation.assigned | 03 | yes | yes | includes from/to member and team; unassign = to null |
| conversation.closed | 03 | yes | yes | |
| conversation.reopened | 03 | yes | yes | reason: customer_message, manual, reply |
| conversation.snoozed | 03 | yes | yes | |
| conversation.unsnoozed | 03 | yes | yes | reason: expired, customer_message, manual |
| conversation.priority_changed | 03 | yes | yes | |
| conversation.tagged / conversation.untagged | 03 | yes | yes | |
| conversation.marked_spam / conversation.unmarked_spam | 03 | yes | yes | |
| conversation.deleted | 03 | yes | yes | |
| conversation.read | 03 | yes (member only) | no | per-member unread; widget gets `conversation.seen_by_team` relay instead |
| message.created | 03, channels, 07 | yes | yes | type message or note; notes only to member realtime channels and webhooks only for endpoints with `include_notes` |
| message.updated | 03 | yes | yes | note edits only (reply edit later: `message-edit`) |
| message.deleted | 03 | yes | yes | |
| message.delivery_updated | channels | yes | yes | sent, delivered, read, failed |
| contact.created / contact.updated | 02, 03, channels | yes | yes | |
| contact.merged | 03 | yes | yes | from ids, into id |
| contact.deleted | 03 | yes | yes | GDPR erase |
| typing.started / typing.stopped | widget, inbox (REST `typing-indicators`) | yes (ephemeral) | no | never stored, no history |
| member.availability_changed | 00-accounts | yes (presence) | no | |
| ai.handoff_requested | 07 | yes | yes | AI asked for human |
| ai.run_completed | 07 | no | yes | tokens, cost, outcome |
| article.published / article.updated / article.unpublished | 08 | no | yes | |
| article_suggestion.created | 08 | yes | yes | review queue |
| channel.connected / channel.disconnected / channel.errored | 04, 05 | yes | yes | |
| billing.conversation_activated | 12 | no | yes | metering ledger line |
| billing.threshold_reached | 12 | yes | yes | 50/80/100% of cap |

### Events added by feature specs

Realtime/webhook flags defined in owner spec. Listed here so catalog stays complete.

| Owner | Events |
|---|---|
| 00-accounts | workspace.created, workspace.updated, workspace.deleted, workspace.restored, workspace.suspended, workspace.unsuspended, member.joined, member.role_changed, member.deactivated, member.reactivated, team.created, team.updated, team.deleted, team.member_added, team.member_removed, invitation.created, invitation.accepted, invitation.revoked, token.created, token.revoked, user.registered, user.email_verified, user.updated, user.deleted, session.revoked |
| 01-messenger-widget | widget.created, widget.settings_updated, widget.deleted (realtime yes, webhook no) |
| 02-widget-identity | identity_secret.created, identity_secret.retired, identity_secret.revoked (realtime yes, webhook no; `identity_settings.updated` later: `identity-validations`) |
| 03-shared-inbox | conversation.updated, conversation.merged, conversation.participants_changed, conversation.counts_changed, conversation.seen_by_team (widget-safe relay of read state: timestamp only), company.created, company.updated, company.deleted, tag.created, tag.updated, tag.deleted, saved_reply.created, saved_reply.updated, saved_reply.deleted, inbox.created, inbox.updated, inbox.deleted, contact.blocked, contact.unblocked, contact.export_ready |
| 04-email-channel | email_domain.status_changed, email_address.forwarding_confirmation_received, email_suppression.created, email_suppression.deleted, email_sending.paused, email_sending.resumed |
| 05-chat-app-channels | channel.updated (realtime only), whatsapp_template.status_changed |
| 06-notifications | notification.created, notifications.read, notification_destination.errored (none webhook-deliverable) |
| 07-dimmi-agent | ai.reply_started, ai.reply_delta (ephemeral), ai.reply_discarded, ai.state_changed, ai.spend_threshold_reached, ai.spend_cap_reached, ai.spend_cap_cleared, ai.provider_key_failed, ai.provider_key_changed (audit only), ai.agent_updated. Agent actions (`actions.md`): ai.action_call_updated (realtime only; `conversation:` + widget-safe `{id, title, status}` on `widget_contact:`), ai.action_call_completed (webhook only, object `ai_action_call`, no payloads), ai.action_call_approval_requested (realtime + webhook; drives 06 notification), ai.action_changed (realtime only), ai.action_connection_failed (realtime + webhook, object `ai_action_connection` + `reason`) |
| 08-help-center | article.created, article.deleted, article_suggestion.reviewed, knowledge_source.sync_completed, knowledge_source.sync_failed, help_center.updated, help_center.domain_status_changed, knowledge.embeddings_switched. Docs sync (`docs-sync.md`): docs_sync_run.completed, docs_sync_run.failed, article.freshness_changed (realtime + webhook), github_repository.webhook_status_changed, github_connection.errored (realtime only) |
| 09-agent-native | oauth_grant.created, oauth_grant.revoked |
| 10-webhooks | webhook_endpoint.failing, webhook_endpoint.disabled, webhook.test (synthetic, test API only) |
| 11-self-host | system.backup_succeeded, system.backup_failed, system.update_available, system.maintenance_changed, export.completed, system.app_url_changed |
| 12-cloud-billing | billing.conversation_credited, billing.conversation_credit_reversed, billing.forecast_exceeds_cap, billing.usage_anomaly_detected, billing.activations_paused, billing.activations_resumed, billing.spend_cap_updated, billing.invoice_finalized, billing.invoice_paid, billing.payment_failed, billing.account_status_changed, billing.meter_updated |
| 13-changelog | changelog_entry.created, changelog_entry.updated, changelog_entry.scheduled, changelog_entry.published, changelog_entry.unpublished, changelog_entry.deleted, changelog.updated, changelog_mailing.updated, changelog_mailing.completed, changelog_subscriber.subscribed, changelog_subscriber.unsubscribed, changelog_generation.completed, changelog_generation.failed, changelog_source.errored. Webhook yes: `changelog_entry.*` except `scheduled`, `changelog_mailing.completed`, `changelog_subscriber.*`. `changelog_entry.published` carries `{entry_id, url, title, published_at, notify_subscribers, feedback_post_ids[] (max 50), notify_voters}`; `changelog_entry.deleted` carries `{entry_id, purged}` (14 consumes both, "Feedback shipping" in contracts.md) |
| 14-feedback-roadmap | feedback_post.created, feedback_post.updated, feedback_post.status_changed, feedback_post.moderated, feedback_post.merged, feedback_post.unmerged, feedback_post.deleted, feedback_post.restored, feedback_post.counts_changed (realtime only, coalesced 5 s), feedback_vote.created, feedback_vote.deleted (webhook only), feedback_comment.created/updated/deleted (internal comments webhook only to `include_notes` endpoints), feedback_request.created, feedback_request.deleted, feedback_suggestion.created, feedback_suggestion.reviewed, feedback_board.created/updated/deleted, feedback_status.created/updated/deleted, feedback_settings.updated, feedback_post_update.sent (webhook only), feedback_import.completed (`member:` channel of creator) |
| 15-announcements-surveys | announcement.created/updated/deleted, announcement.status_changed, survey.created/updated/deleted, survey.status_changed, survey_response.created, survey_response.completed (only webhook=yes event of 15; CSAT also on `conversation:` channel), survey_response.deleted, engage_settings.updated |
| 18-audiences | audience.created/updated/deleted, attribute_definition.created/updated/archived/deleted, event_definition.created/updated, contact_event.tracked (excluded from webhook wildcards), surface.changed (widget only). Exact flags in 18 README |
| 16-importers | import.created, import.updated, import.deleted, import.started, import.progressed (realtime only, coalesced 2 s per run, history kept), import.paused, import.resumed, import.completed (only webhook=yes event of 16, object `import_run`), import.failed, import.cancelled, import.rolled_back. Realtime on `workspace:`; `import.started`/`completed` also `member:` of starter |
| 03-shared-inbox (2026-11-04 round) | conversation.restored (realtime + webhook), conversation.hidden (member), conversation.viewers_changed, member.access_changed, member.availability_changed, conversation_view.created/updated/deleted, inbox_preferences.updated, snooze_list.updated, draft.updated, bulk_operation.updated, message.reactions_changed, attachment.scanned, data_export.ready, side_conversation.created/updated, side_message.created (realtime only); conversation_attribute_definition.created/updated/deleted (+ webhook). `conversation.unsnoozed.reason` adds member_reply, others_note, assigned, attribute_changed, side_reply. `conversation.priority_changed` carries level strings |
| 03-shared-inbox redaction | redaction_rule.created/updated/deleted, redaction_settings.updated (internal: audit log, no realtime, no webhook); message.redacted (internal: rule names and match count, never text; email channel drops the raw copy); contact_erasure.requested/canceled/held (internal: audit log), contact.deleted (internal, after an erasure wipes everything, with counts); contact.export_ready (realtime on the requester's member channel) |
| 04-email-channel (2026-11-04 round) | email_domain.records_updated, email_address_test.updated, email_address.setup_findings_changed (realtime only), email_mailbox_connection.status_changed (realtime + webhook). email_suppression.created/deleted now realtime too |
| 01, 02, 20 (2026-11-04 round) | widget.site_import_finished, widget_domain.status_changed, identity_settings.updated, widget_push_credential.failing (realtime only). Every conversation publication on `widget_contact:` carries `widget_id`; `customer_status` on message.created, conversation.closed, conversation.reopened widget payloads |
| 00-accounts, 21 (2026-11-04 round) | role.created/updated/deleted, member.custom_role_changed (+ webhook), workspace.access_policy_changed (realtime, no values); sso_connection.*, organization_domain.*, scim_token.*, scim.* (no realtime, no webhook). `team.member_added` gains `added_by_member_id`; `team.deleted` gains `reassign_to` |

Payload notes for events consumed across specs:
- `oauth_grant.revoked` (09): `{grant_id, user_id, first_party, device_id, reason}`; `reason` registry enum (`oauth_grants.revoked_reason`, 09 owns list: `user`, `admin`, `user_deleted`, `member_deactivated`, `role_changed`, `refresh_reuse`, `client_disabled`, `code_reuse`, `workspace_deleted`, `idle`, `logout_others`, `password_changed`, `password_reset`, `two_factor_disabled`, `pre_hijack_defense`). 00-accounts and 06 consume (drop app sessions, push subscriptions with that `oauth_grant_id`); realtime listener disconnects that grant's sockets.

- `changelog_entry.published` / `changelog_entry.deleted` (13): see 13 row; 14 listener links posts and notifies voters.
- `contact.updated` / `company.updated` (03): `changes` keys include changed `custom_attributes` keys (14 revenue recompute, 07 trusted-key bookkeeping).

Rule (16): import writers dispatch no per-record events (`conversation.created`, `message.created`, `contact.created`, `article.*`, `feedback_*`, `changelog_entry.*`); lifecycle `import.*` only. Listeners that could see imported rows anyway (`SendMemberNotifications`, `TriggerDimmiAgent`, `QueueWebhookDeliveries`, `SendChannelOutbound`, 03 auto-assign) skip `metadata.imported` (defense in depth).

Features may add events. New event: add row here in same PR.

Not domain events (no fan-out, never webhooks or realtime):
- User security events: rows in `security_events` (00-accounts), `type` registry enum (dotted). Auth factor types (`00-foundation/extensibility.md` section 1): `two_factor.enabled`, `two_factor.disabled` (account state, kept); new `auth_factor.added`, `auth_factor.removed`, `auth_factor.renamed`, `auth_factor.recovery_code_used` (was `two_factor.recovery_code_used`), `auth_factor.recovery_codes_regenerated` (was `two_factor.recovery_codes_regenerated`); metadata `{factor_id, type}`.
- Account audit names (00-accounts): entries in `audit_logs` only, never fanned out. The audit listener writes them from in-process events: `member.left` (a member deactivating themselves), `ownership.transferred` (a role change to owner), `member.two_factor_reset_requested`, `member.two_factor_reset_completed`, `member.sign_in_lock_cleared` (sign-in help events). Mirrored from `security_events`: `member.signed_in`, `member.sign_in_failed`, `member.sign_in_locked`, `member.two_factor_enabled`, `member.two_factor_disabled`, `member.auth_factor_added`, `member.auth_factor_removed`, `member.recovery_code_used`, `member.password_changed`, `member.password_reset`, `member.email_changed`, `member.identity_linked`, `member.identity_unlinked`, `member.sessions_revoked`. Written directly by their action: `audit_logs.exported`, `members.exported`, and `workspace.suspended` / `workspace.unsuspended` from operator commands (actor `system` via `cli`; no domain event yet despite the catalog row).
- `UserSignedOutEverywhere` (00-accounts): in-process only, no name, no webhook. Dispatched when a password reset, an operator sign-out or an account claim ends every credential; the realtime listener closes the user's connections.
- Organizations: created and soft-deleted inside `CreateWorkspace` / `PurgeWorkspace` (00-accounts), covered by `workspace.created` / workspace purge. No `organization.*` events until org API exists (later additive).
- Entitlement overrides (12, staff `dimmi entitlement:set`): audit logged, cache bust via PubSub control message. No domain event.

### Ephemeral events

Flag `Ephemeral: true` (typing, `ai.reply_delta`): published to realtime without history (`HistorySize` 0), never recovered on reconnect, never webhooks, never audit.

## Standard listeners

| Listener | Events | Queue |
|---|---|---|
| BroadcastToRealtime | all with realtime=yes | sync after commit: `a.Broadcast.Broadcast(ctx, channel, event, resource)`; broker owns cross-node fan-out |
| QueueWebhookDeliveries | all with webhook=yes | `webhooks` (fan-out job per endpoint) |
| WriteAuditLog | member/token actions on settings, members, tokens, channels, deletes, exports (incl. 14 feedback settings/bans/merges, 15 engage status/settings/exports/response deletes) | sync in tx |
| ForwardAuditEntries | every audit row written by WriteAuditLog | `default` (queued, inserted same tx): calls `AuditSink.Write` (contracts.md; noop at MVP) |
| SendMemberNotifications | conversation.created, conversation.assigned, message.created (customer, and notes with mentions), ai.handoff_requested, conversation.unsnoozed | `notifications` |
| TriggerDimmiAgent | message.created (customer) only. conversation.created is no-op (first message already fires message.created) | `ai` |
| RecordBillableActivity (cloud) | message.created (substantive reply) | `InTx`: calls `MeterMessageActivity` inside 03 reply transaction (activation + spend cap); abort returns 423 `spend_cap_reached` (12) |
| ApplyBillingCredits (cloud) | conversation.marked_spam, conversation.unmarked_spam (reverse credit), conversation.merged, message.delivery_updated (permanent failure) | `billing` (queued) |
| SendChannelOutbound | message.created (type message, author not contact, channel needs push) | `channels` or `mail` |
| ReindexSearch | none needed (generated tsvector) | |

## Realtime: centrifuge, embedded

Decision (`research/build-vs-reuse.md` section 1): embed `github.com/centrifugal/centrifuge` (MIT, pinned) in `framework/broadcasting`. No Pusher protocol, no laravel-echo, no own protocol server. One binary, no sidecar.

| Client | Library | Transport | Endpoint |
|---|---|---|---|
| Dashboard SPA | centrifuge-js | WebSocket (fallback: centrifuge SSE / HTTP-streaming emulation) | `GET /connection/websocket` |
| First-party mobile apps | centrifuge-swift, centrifuge-java (Android) | WebSocket | `GET /connection/websocket` |
| CLI `--watch`, integrations | centrifuge-go / centrifuge-js | WebSocket | `GET /connection/websocket` |
| Widget | plain browser `EventSource` (about 1 KB gz, no SDK) | unidirectional SSE (receive only) | `GET /connection/uni_sse` |

- Uni-SSE handler: thin `framework/broadcasting` handler on centrifuge public API (`centrifuge.NewClient`, `ProtocolConnect`), pattern from library `_examples` unidirectional SSE. Own code, no copied Centrifugo server code.
- Not at MVP: client-side subscribe for widget (uni transport is receive-only), encrypted payloads, delta compression, server API to third parties.

### Connect and subscribe auth (app callbacks)

- Connect: `node.OnConnecting` resolves caller from HTTP request with same guards as REST (`app/auth`).
  - Dashboard: `__Host-dimmi_session` cookie on same-origin upgrade + `Origin` check (`http.CrossOriginProtection` rules).
  - Apps, CLI, integrations: bearer (`dmi_oat_`, `dmi_live_`) in connect `token` field (never URL). Abilities/permissions as REST.
  - Widget: 60-second single-purpose connect token from `POST /widget/v1/realtime-tokens` (01; auth `auth:widget`; returns `{token, expires_at, channels}`). Visitor session token never in a URL. `EventSource` cannot send headers, so connect token rides query param `cf_connect`; logs redact `cf_connect`. On `error`: close, mint fresh token, reopen with last `offset`/`epoch` (browser auto-retry would reuse expired URL).
- Subscribe: `client.OnSubscribe` calls authorizers in `routes/channels.go` (Laravel `Broadcast::channel` shape: `broadcast.Channel("conversation:{conv}", fn)`). Widget gets server-side subscriptions in connect reply (`widget_contact:` only).
- Authorizers check permissions (`authz.Can`), never role names (P2). Conversation channels go through same `conversations.VisibleScope(membership)` (03) as REST lists, search, counts, exports.
- Client publish: off on every namespace (`OnPublish` rejects). Everything server-published, incl. typing (REST, below).
- Member removed or deactivated, token or grant revoked, widget session revoked: `node.Disconnect(userID)` (cross-node via Controller) within 5 s.

### Channels (centrifuge namespaces)

Format: `<namespace>:<public id>`. Namespace = singular snake_case resource; id = public prefixed id (never raw UUID). All namespaces private (subscribe auth required); no public channels.

| Channel | Who can subscribe | Presence | History (size / TTL) | Carries |
|---|---|---|---|---|
| `workspace_conversations:ws_...` | members with `conversation_access = all` (others get copies on `member:`; 03 access.md) | no | 300 / 10 min | conversation list changes (created, assigned, status, tags, priority, new message summary) |
| `workspace:ws_...` | active members | yes (online members, availability) | 300 / 10 min | feature events with realtime=yes (07 actions, 08 docs sync, 13-16 unless stated) |
| `conversation:conv_...` | members passing `VisibleScope` (03 access.md) | yes (who is viewing: collision indicator), join/leave on | 300 / 10 min, client publish off | full message events, delivery updates, notes, contact typing (ephemeral, no history; teammate-to-teammate typing later: `teammate-typing`), CSAT response (15), agent action calls (07) |
| `member:mem_...` | that member only | no | 100 / 10 min | personal notifications, unread counts, import completion |
| `widget_contact:ctc_...` | widget session of that contact (server-side subscription) | no | 100 / 10 min | messages (no notes), teammate typing (name + avatar only), delivery/read, `conversation.seen_by_team`, widget-safe `ai.action_call_updated` (07) |

- `presence-*` / `private-*` prefixes gone: presence is a namespace option (`EmitPresence`, `EmitJoinLeave`).
- Broadcast payload: `{id: evt_..., event, server_time, data: Resource JSON}` (same Resource as REST). Widget gets widget-safe resource (no notes, no internal fields, teammate email hidden).
- Max payload 10 KB; larger events broadcast `{id, object, updated_at}` and client fetches via REST.
- Workspace channel coalesced per 100 ms window per conversation (list updates); message events not coalesced.

### History and recovery (replaces last-event-id resume; history in Valkey streams)

- Each publish gets stream position (`offset`, `epoch`). Clients subscribe with `recover: true` + last position (centrifuge-js automatic; widget passes it in `cf_connect` subs).
- Reconnect within history TTL: server replays missed publications, reply `recovered: true`. Gap too big, epoch changed, or TTL passed: `recovered: false`, client refetches lists via REST.
- Ephemeral events published with `HistorySize` 0: never recovered.
- Client dedupes by event `id` (replay plus live overlap).

### Broker, presence, history: Valkey (always)

PO decision 2026-11-02: Valkey required in every deployment (incl. $5 VPS: app + Postgres + Valkey). Realtime uses centrifuge built-in Redis broker + Redis presence manager against Valkey (RESP compatible). No custom broker, no realtime tables in Postgres.

- Broker: `centrifuge.NewRedisBroker` (publish, history streams with size/TTL, recovery positions, cross-node fan-out). Presence: `centrifuge.NewRedisPresenceManager`. Controller (node discovery, cross-node `Disconnect`): built into Redis broker.
- Same Valkey instance as cache, rate limits, internal pubsub (`redis.url`, env `DIMMI_REDIS_URL`); key prefix `broadcasting.redis.prefix` (default `dimmi:rt:`).
- Valkey holds only rebuildable state. Valkey loss = clients reconnect, `recovered: false`, refetch via REST; presence rebuilt within one heartbeat. Nothing durable lost.
- Tests: no broadcasting fake, no `memory` broker; tests subscribe a real client and run against real Valkey from root `compose.yaml` (`REDIS_URL`, key prefix per test; testing.md).
- Internal cross-replica bus (`framework/pubsub`: cache bust, auth cache bust, control topics) also on Valkey PUBLISH/SUBSCRIBE, separate key space from realtime.

### Presence and typing

- Presence from centrifuge Redis presence manager on Valkey (above). Member availability: `online` only when member set available AND at least one connected client in last 60 seconds. Drives widget "team online" indicator and routing.
- Typing: REST for all clients (one auth path). Members `POST /api/v1/conversations/{conv}/typing-indicators`, widget `POST /widget/v1/conversations/{conv}/typing-indicators` (202, ephemeral). Server throttles 1 per 2 s per user per conversation, publishes contact typing on `conversation:` (members) and teammate typing on `widget_contact:` (name + avatar only) without history; teammate typing to other members later (`teammate-typing`); clients auto-expire after 6 s.

## Edge cases

- Event for deleted conversation reaches listener: listener re-fetches, skips if gone, logs debug. Never error-loop.
- Same event delivered twice (recovery replay plus live publish): client dedupes by event `id`.
- Member removed from workspace while connected: `node.Disconnect` within 5 seconds (listener on `member.deactivated`, `user.deleted`; `token.revoked`, `session.revoked`, `oauth_grant.revoked` disconnect that credential's clients); client gets disconnect code `4009` (`member_removed`, custom range 4000-4499, no reconnect) and drops workspace state.
- Contact merged while widget open: `widget_contact:` channel of old contact receives `contact.merged`; widget mints new connect token (server subscribes it to new contact channel).
- Widget connect token expired before `EventSource` opened (slow network): 401 disconnect, widget mints new token once, then backs off (1 s, 2 s, 4 s ... 30 s).
- Proxy buffers SSE (corporate proxy): no frames within 20 s of connect, widget falls back to polling `GET /widget/v1/conversations/{conv}/messages?after=` every 10 s while panel open.
- Thousands of sockets per workspace: workspace channel coalesced per 100 ms window per conversation; message events not coalesced.
- Node restart / deploy: `Node.Shutdown` closes clients with reconnect advice; clients recover from Valkey history on any node.
- Valkey restart or flush: history gone, epoch changes, clients get `recovered: false` and refetch lists via REST. Valkey unreachable: `/ready` 503 (load balancer drains node), `/up` 200; node keeps serving under data-model.md "Valkey unreachable" policy (auth/abuse guards fail closed, everything else fails open with log + metric); realtime down until Valkey returns.
- Clock skew: server time in every payload (`server_time`), client uses for relative times.
