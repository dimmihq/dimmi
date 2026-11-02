# Reconciliation log

Style: caveman. Raw notes from spec writing (2026-11-01/02): what each writer reported, conflicts found, how resolved. History only; current truth lives in 00-foundation and feature specs.


## 07-dimmi-agent
- New events: ai.reply_started, ai.reply_delta (ephemeral, no replay), ai.reply_discarded, ai.state_changed, ai.spend_threshold_reached, ai.spend_cap_reached, ai.spend_cap_cleared, ai.provider_key_failed, ai.provider_key_changed (audit only), ai.agent_updated.
- Core: conversations + ai_reply_count, ai_cost_micros, ai_language, ai_handed_off_at, ai_handoff_reason (migration 20261101070001). Prefixes aia_, aik_, air_, ais_.
- Foundation fixes: TriggerDimmiAgent only on message.created (conversation.created no-op); realtime needs ephemeral flag (no resume replay); abilities add ai:read, ai:write, ai:keys.
- 08 must implement contracts.Knowledge (Search, Stats, EmbeddingKeyInUse, PublicHosts) per 07 README "Retrieval contract"; 08 calls ai.ResolveProviderKey + RecordAiSpend; reuses AnswerQuestion for Ask AI.
- 01: /widget/v1/conversations/{conversation}/handoffs route, streaming UI.
- Open: office hours workspace-level (03) vs ai_agents; default AI cap $25/month; UTC month periods; greeting non-billable; empty knowledge blocks enabling; embedding spend counts toward cap.

## 00-accounts
- New events: workspace.created/updated/deleted/restored/suspended/unsuspended, member.joined/role_changed/deactivated/reactivated, team.created/updated/deleted/member_added/member_removed, invitation.created/accepted/revoked, token.created/revoked, user.registered/email_verified/updated/deleted, session.revoked (flags in 00-accounts README).
- Core: users + deleted_at, password_changed_at, two_factor_last_used_step, risk_flags, created_via; workspaces + created_by_user_id, deleted_by_user_id, purge_after, suspended_reason; workspace_members + last_accessed_at, deactivated_at, deactivated_by_member_id, joined_via; teams + deleted_at + partial unique name. New tables one_time_tokens, security_events, workspace_domains. Migrations 000001-000019 reserved for accounts.
- Prefixes add: mem_, inv_, ses_, aud_, idn_, dom_, sev_.
- Hooks: workspace-initializer hook (03 default inbox, 01 widget settings). Unassign on member.deactivated/team.deleted (03). 06 stop notifying deactivated.
- Foundation fixes: roadmap roles say admin/member, data model has owner (keep owner; fix roadmap). Audit log in MVP (security.md) vs roadmap upmarket #18 lists audit log: fix roadmap (audit log MVP, SSO later). __Host- cookie fallback for plain http self-host. Log mail driver prints auth links in dev: allowed exception, document. 423 conflict: spend cap 423 vs password.confirm 423 -> use 423 only for password confirm? decide: spend cap use 402? Choose: spend cap 402 payment_required? hmm decide later. RLS pre-tenant lookups via SECURITY DEFINER. System KeyManager key for user-level secrets (2FA). 
- Open: service tokens not MVP; password min 10; audit ability reuse settings:read; self-host signup toggle config only.

## 03-shared-inbox
- Files: README, api.md, channel-contract.md (contracts.Channel: Send, Capabilities, optional Editor/Deleter/Typing/ReadMarker; ReceiveInboundMessage), search.md, edge-cases (183), tasks.
- New events: conversation.updated/.merged/.restored/.participants_changed/.counts_changed; company.*, tag.*, saved_reply.*, inbox.*; contact.blocked/.unblocked/.export_ready.
- Core: conversations + waiting_since, last_activity_at, last_message_id, previous_conversation_id, merged_into_id, is_test, search + list indexes; messages + channel_connection_id, redefined search; workspace_id on conversation_tag, conversation_reads, conversation_participants; saved_replies unique NULLS NOT DISTINCT. New tables message_mentions, contact_merges, contact_exports.
- Foundation fixes: metadata.previous_conversation_id -> column previous_conversation_id; add workspace_id to 3 pivots; btree_gin extension for GIN index leading workspace_id; unaccent not IMMUTABLE -> custom text search config (dimmi_en with unaccent dictionary); saved_replies NULLS NOT DISTINCT; prefixes (members usr_? -> accounts proposed mem_; pick mem_), rpl_, exp_; search budget 150ms; member.deactivated event (accounts has it); merge event conversation.merged exists now.
- Open: English stemmer + unaccent all workspaces; reply to closed keeps closed (conflicts with data-model "teammate reply with reopen flag" — fine, flag optional); spam purge after 30 days.

## 01-messenger-widget + 02-widget-identity
- New events: widget.created/settings_updated/deleted; identity_secret.created/retired/revoked; identity_settings.updated (realtime yes, webhook no). (superseded by scope trim: `identity_settings.updated` later `identity-validations`)
- Core: conversations.test (20261101000105) CONFLICT with 03 conversations.is_test -> pick is_test (03), fix 01. attachments.uploaded_at (20261101000106). widget_sessions.identity_secret_id FK (20261101000204). 02 adds 3 tables not in ownership map -> add.
- Foundation fixes: conversation.read widget-safe relay ("Seen"); data-model email_verified only when JWT claim email_verified:true; tech-stack SSE fallback vs long-poll only (accept long-poll); size budget: loader <=3KB + core <=35KB (update roadmap); workspace.created exists now in accounts -> identity settings can init via workspace-initializer hook (lazy ok).
- Open: default identity mode required (anonymous still works); RS256/EdDSA at MVP; identified session 24h; anonymous 180 days; ad-blockers first-party proxy only.

## 08-help-center
- Files: README, knowledge.md, review-queue.md, rendering.md, api.md, edge-cases (162), tasks.
- CONFLICT: 08 defines knowledge.Searcher.Search in app/actions/knowledge; 07 expects contracts.Knowledge {Search, Stats, EmbeddingKeyInUse, PublicHosts}. Reconcile: one interface contracts.Knowledge in app/contracts implemented by app/actions/knowledge (08) with all 4 methods; update both specs.
- Decisions: multiple help centers (cloud cap 3), 2-level collections, Markdown; public MCP read-only 3 tools at MVP (roadmap had it as standout -> note pulled into MVP); pasted text = agent-only article; MCP cannot accept/publish suggestions.
- New events: article.created, article.deleted, article_suggestion.reviewed, knowledge_source.sync_completed/sync_failed, help_center.updated, help_center.domain_status_changed, knowledge.embeddings_switched.
- Ownership map add: article_suggestion_batches, help_center_domains, help_center_redirects, help_center_assets, knowledge_settings, knowledge_sync_runs, knowledge_embeddings, help_center_article_stats_daily, help_center_searches. btree_gin required ext. New prefixes.
- Deps: 07 voyage provider type, AnswerQuestion stream; 09 client_kind claim on OAuth tokens, CLI dimmi docs *; 11 Caddy on-demand TLS ask endpoint.
- Open: separate cloud help domain yes; SVG rejected; Voyage in 07 yes.

## 06-notifications + 10-webhooks
- Standard Webhooks headers adopted (webhook-id, webhook-timestamp, webhook-signature, whsec_).
- New events: notification.created, notifications.read, notification_destination.errored, webhook_endpoint.failing, webhook_endpoint.disabled, webhook.test (synthetic). None webhook-deliverable.
- Tables add to ownership map: notification_settings, notification_destinations, notification_deliveries, webhook_delivery_attempts. push_subscriptions user-scoped (not workspace).
- Foundation fixes: mem_ prefix (agrees with accounts); new queue `notifications`; abilities add notifications:read/write? and webhooks:read; clarify notes in webhooks only with include_notes opt-in (fix data-model sentence); prefixes ntf_, psub_, ndst_, ndlv_, whd_, wha_.
- Deps: 03 messages.metadata.mentions (03 has message_mentions table — reconcile: use message_mentions table); 05 share Slack/Discord clients + notification scopes; 07 ai_state set in same tx as conversation create; 11 VAPID keys storage; 09 dimmi listen/trigger.
- Open: badge from service worker; auto-disable after 72h; global webhook sequence ok; no static egress IPs.

## 09-agent-native
- MCP spec 2026-07-28 (agent verified): DCR deprecated for CIMD (Client ID Metadata Documents); no sessions, no GET stream; elicitation via "input required" results (MRTR). Spec keeps backward compat.
- Decisions: reply_drafts server-side (09 owns); workspace send_mode confirm|drafts_only|direct (default confirm); 25 MCP tools; CLI = same Go codebase `-tags cli`, npm `dimmi` per-platform binaries; Homebrew own tap. (superseded by scope trim: reply drafts later `reply-drafts`, send modes later `agent-send-modes`; MVP always confirms)
- New tables: oauth_clients, oauth_grants, oauth_authorization_requests, oauth_auth_codes, oauth_access_tokens, oauth_refresh_tokens, oauth_device_codes, reply_drafts. Events: reply_draft.created/updated/sent/discarded, oauth_grant.created/revoked. (`reply_drafts` later: `reply-drafts`)
- Core jsonb keys: workspaces.settings.agent_access, messages.metadata.sent_via.
- Foundation fixes: Actor gets `Via` field (oauth client / token) — author_type stays; member events exist in accounts; ownership map add tables; conventions add app/client/ for CLI client code; command naming: server/admin commands colon style (make:migration), client resource commands space style (dimmi conversations list) — document; prefixes ocl_, ogr_, oar_, odr_, drf_; api.md endpoint ownership: drafts sub-resource owned by 09.
- Open: AI-assisted label no; refresh 90-day sliding; device grant first-party only.

## 04-email-channel
- Files: README, pipeline.md, outbound.md, edge-cases (170), tasks (14 milestones).
- Assumed Channel: Type(), Capabilities(), Send(ctx, OutboundMessage) (SendResult, error); inbound calls 03 action `ReceiveChannelMessage` -> 03 named it `ReceiveInboundMessage`: align to 03.
- New events: email_domain.status_changed, email_address.forwarding_confirmation_received, email_suppression.created/deleted, email_sending.paused/resumed.
- 10 new tables (migrations 0401-0411), 6 prefixes. Ownership map update.
- Foundation fixes: attachment cap 10 per message, inbound email override 50 (document per-channel override); external_thread_key unique collides on reopen -> partial unique where status <> closed already... note: reopen of closed thread while new open one exists -> rule: lookup prefers open, else most recent; DMARC pass = verified email (accept: email identity verified when DMARC aligned pass); workspace.created now exists (accounts) -> default address via initializer hook; webhook paths sub-paths /webhooks/{provider}/{connection?}; D1 ignore auto-replies (inbound auto-reply not starting billable) add to pricing D1 note; Mailer contract shape (raw MIME, provider message id, typed errors) add to foundation.
- Open: Reply-To token on; newsletters closed conversations; raw .eml 30 days; offline reply delay 120s; cloud cap 200/day unverified, 2,000 verified; Gmail/MS OAuth + ARC post-MVP.

## 05-chat-app-channels
- Uses 03 channel-contract unchanged + X1-X5 additions to 03: X1 identity type whatsapp_bsuid; X2 InboundMessage.Echo (teammate native thread replies recorded as replies); X3 optional ChannelWindowResolver (WhatsApp 24h); X4 extra provider ids in 05 channel_message_refs; X5 03 accepts driver-created thread key.
- New events: channel.updated (realtime only), whatsapp_template.status_changed (realtime + webhook).
- Core: 20261101000501 contact_identities.type CHECK add whatsapp_bsuid; 20261101000502 lookup indexes. Prefixes chr_, cml_, wtp_, cie_.
- Foundation fixes: WhatsApp key by bsuid when phone omitted (data-model note); Discord gateway per-shard lease table (fix roadmap wording); command spelling channels:add (fix roadmap); security.md Discord messages via Gateway, only interactions signed; abilities: channels use settings:* (ok) ; status 424 add to conventions list (provider failure on connect); Meta Tech Provider: Meta bills customer directly -> fix pricing.md/roadmap pass-through note.
- Open: Slack emails unverified; cap 3 custom Discord bots per cloud workspace; Slack marketplace at beta; Discord message content intent at launch; WhatsApp read receipts on.

## 11-self-host + 12-cloud-billing
- New events 11: system.backup_succeeded/backup_failed/update_available/maintenance_changed, export.completed. 12: billing.conversation_credited, conversation_credit_reversed, forecast_exceeds_cap, usage_anomaly_detected, activations_paused/resumed, spend_cap_updated, invoice_finalized, invoice_paid, payment_failed, account_status_changed, meter_updated.
- Core: users.is_instance_admin (20261101001101).
- Foundation: in-tx listeners that can abort tx + tx.AfterRollback hook; abilities billing:write, system:*, exports:*; queue backups; RecordBillableActivity must decide cap in reply tx; ledger must survive workspace purge (no cascade, no updated_at); Stripe one-off invoices not meters (fix pricing.md); compose image pgvector/pgvector:pg18 + PG18 volume path; backups need -pgtools image; app key auto-generate for one-command; River UI instance admins.
- 03 blocked reply 423.
- Open: GitHub org dimmihq; publish bands as-is; Terraform before Helm.

## Final cross-spec sweep (2026-11-02, foundation + conventions)
- Realtime: Pusher protocol + laravel-echo replaced by embedded centrifuge (`framework/broadcasting`). Endpoints `/connection/websocket`, `/connection/uni_sse` (widget `EventSource`, 60 s connect token from `POST /widget/v1/realtime-tokens`). Channels `workspace:ws_`, `conversation:conv_`, `member:mem_`, `widget_contact:ctc_`. History + recovery replaces `last_event_id` resume. Typing via REST for widget.
- PO decision: Valkey required everywhere. Owns realtime broker/presence/history (centrifuge Redis broker), cache, rate limits, internal pubsub, replay/dedupe windows, counters. Dropped tables: `realtime_events`, `cache`, `rate_limits`, `identity_jwt_nonces` (02 to apply). Rule: Valkey holds only rebuildable state.
- Framework layer: `app/contracts` domain only; framework contract pointers in contracts.md; `ids.UUID`, `mail.Message`, `*database.Tx`, `a.Events.Dispatch`.
- Mail final model (04): one instance-level provider, two streams (`system`, `customer`), no SMTP receiver, no per-workspace providers.
- CSRF: `http.CrossOriginProtection`; `GET /auth/csrf` and `XSRF-TOKEN` removed (00-accounts applies).
- Entitlements: full MVP catalog in contracts.md; one over-limit code `entitlement_limit_reached` (422 count, 429 per period). Feature specs using own codes for catalog keys switch (01 `widget_limit_reached`, 04 `email_daily_limit_reached`, 08 `help_center_limit_reached`/`article_limit_reached`/`source_limit_reached`, 10 `endpoint_limit_reached`, 14 `board_limit_reached`, 15 engage `live_limit_reached` where catalog-backed).
- PO: status page removed from product (old folder 15-status-page). Announcements/surveys folder renamed `16-` -> `15-announcements-surveys`; migration block `0015NN`.
- 13-15 added: prefixes (zero collisions), ownership rows, migration blocks `0013NN`-`0015NN`, events, abilities `changelog:*`, `feedback:*`, `engage:*`, rest exceptions, initializers (13, 14), erasers (13-15), Feedback shipping contract.
- License: MIT (PO). Dependencies must be MIT-compatible (MIT, BSD, ISC, Apache-2.0, MPL-2.0 unmodified); no GPL/AGPL.
- Post-MVP items referenced by name (research/roadmap.md table), not number.

## Final consistency pass (2026-11-02, round 3: agent actions, docs sync, importers)
- 07 agent actions: prefixes `acn_ aac_ aacv_ aax_`, tables in 07 row, events `ai.action_*`, entitlement keys `ai.actions.*`, ability group `ai:actions`, `contacts.trusted_attribute_keys` core column (only trusted sources add keys; imports untrusted unless admin marks mapping `trusted`), SSRF + identity-binding rules in security.md. Narrow depguard exceptions for `app/support/{openapiimport,actionschema,actiontemplate,actionresult}`; MCP client in `framework/mcp`.
- 08 docs sync + shared GitHub layer: prefixes `ghc_ ghr_ dss_ dsr_ asr_`, five events, `docs_sync.*` keys, Knowledge `Surface docs_sync`, one webhook route `/webhooks/github/repositories/{ghr}` (replaces 13 `/webhooks/github/changelog/{cls}` and 08 `/webhooks/github/knowledge/{ks}`), source-code privacy rule. `github.ClientFor` = only way to get a GitHub client.
- 16 importers: prefixes `imp_ impr_ impm_ impf_`, migrations `001601-001699`, `import.*` lifecycle events only (no per-record events), `imports.*` keys, permissions `imports:read|manage|rollback`, `ImportSource` + `ChannelRehomer` contracts, eraser with HMAC tombstones, framework helpers `ids.NewV7At`, `Tx.CopyFrom`, `encryption.MAC`.
- Framework catalog: `customdomain` (+ `certmagic_storage` in framework set), `image`, `support/diff` (textdiff stays app over it), `httpclient/robots`, `view/feed`, `markdown.FrontMatter`; adapter majors go-github v93, stripe-go v87; `nodes` table gone (Valkey `dimmi:nodes:`).
- Valkey unreachable policy decided (data-model.md): auth/abuse guards fail closed, provider limiters defer, rest fail open with metric; `/ready` 503, `/up` 200.
- Mail config shape: `mail.default` = mailer name; driver keys flat under `mail.mailers.<name>`. 06 critical auth mail bypass of system suppression (3/day) kept.
