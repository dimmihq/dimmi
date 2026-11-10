# Core data model

Style: caveman. Scope trim 2026-11-02: ownership/prefix/jsonb rows for deferred items removed or marked reserved (`reply-drafts`, `domain-join`, `identity-validations`, `changelog-custom-domain`, `changelog-replies-to-inbox`). Rules from `specs/00-conventions/README.md` apply: UUIDv7 `id`, `workspace_id` on tenant tables, `created_at`/`updated_at` on every table, text + CHECK for enums, prefixed public IDs. Two enum kinds (`00-foundation/extensibility.md` P1): state machines, statuses, visibility = strict `CHECK in (...)`; driver-backed **registry enums** = `text` + format CHECK `^[a-z][a-z0-9_]{1,39}$` + Go registry validation (new provider, factor, channel = new driver, zero migration). Registry enum columns marked `registry enum` below and in owner specs.

This file owns **shared core tables**. Feature specs own their own tables and must not redefine these. Feature may add columns to core table only via its own migration, listed in its spec under "Core table changes".

## Ownership map

| Table | Owner spec |
|---|---|
| organizations, workspaces, users, workspace_members, teams, team_members (schema here); user_auth_factors, oauth_identities, invitations, password_reset_tokens, one_time_tokens, security_events, workspace_domains, audit_logs; product columns on framework `sessions`, `personal_access_tokens` (ALTER, never CREATE) | 00-accounts |
| inboxes, channel_connections, contacts, contact_identities, companies, conversations, conversation_participants, messages, attachments, tags, conversation_tag, conversation_reads, saved_replies | 00-foundation (schema here), behavior in 03-shared-inbox and channel specs |
| message_mentions, contact_merges, contact_exports | 03-shared-inbox |
| encryption_keys (app `encryption.KeysetStore`) | 00-foundation |
| sessions (`framework/session`), personal_access_tokens (`framework/auth/token`), oauth_clients, oauth_authorization_codes, oauth_grants, oauth_tokens (`framework/oauth`), idempotency_keys (`framework/http/idempotency`), certmagic_storage (`framework/customdomain/storage`), notifications (`framework/notifications/channels/database`, migrate step), River tables (`river_*`, migrate step), `framework_migrations` | framework (one framework migration set; "Infrastructure tables" below). Owner specs add product columns by ALTER in app set |
| widget_settings, widget_sessions | 01-messenger-widget |
| identity_secrets | 02-widget-identity (JWT nonce replay set moved to Valkey `SET NX EX`; no `identity_jwt_nonces` table) |
| email_domains, email_addresses, email_threads, email_message_refs, inbound_emails, outbound_emails, email_suppressions, email_sending_profiles, email_sending_stats, email_signatures | 04-email-channel |
| channel_rooms, channel_message_refs, channel_contact_states, channel_member_links, channel_inbound_events, channel_socket_sessions, whatsapp_templates | 05-chat-app-channels |
| notification_settings, notification_preferences, notification_destinations, notification_deliveries, push_subscriptions (user-scoped, no workspace_id); product columns on framework `notifications` (ALTER) | 06-notifications |
| ai_agents, ai_provider_keys, ai_runs, ai_run_steps, ai_spend_periods, ai_action_connections, ai_actions, ai_action_versions, ai_action_calls (agent actions, `07-dimmi-agent/actions.md`) | 07-dimmi-agent |
| help_centers, collections, articles, article_versions, article_suggestions, article_suggestion_batches, help_center_domains, help_center_redirects, help_center_assets, help_center_searches, help_center_article_stats_daily, knowledge_settings, knowledge_sources, knowledge_sync_runs, knowledge_documents, knowledge_chunks, knowledge_embeddings, github_connections, github_repositories (shared GitHub layer: 08 knowledge, 08 docs sync, 13 changelog), docs_sync_sources, docs_sync_runs, article_source_refs, article_freshness (`08-help-center/docs-sync.md`) | 08-help-center |
| product columns on framework `oauth_clients`, `oauth_grants` (ALTER: `auth_methods`, `device_id`, `confirmed_method`, `revoked_reason` registry enum; `auth_time`, `confirmed_at` already framework), oauth_authorization_requests; oauth_device_codes later (device grant deferred). Framework `oauth_authorization_codes` + `oauth_tokens` (access + refresh rows, `kind`) replace own auth-code/access/refresh tables | 09-agent-native (`reply_drafts` later: `reply-drafts`) |
| webhook_endpoints, webhook_deliveries, webhook_delivery_attempts | 10-webhooks |
| system_settings, backups, data_exports, dimmi_releases (node registry = Valkey `dimmi:nodes` index + `dimmi:nodes:<id>` hashes, no table) | 11-self-host |
| billing_accounts, billing_periods, billing_windows, usage_ledger, billing_adjustments, billing_disputes, credit_grants, spend_caps, invoices_cache, stripe_events, billing_program_applications, billing_reconciliation_issues, credit_grant_applications, entitlement_overrides | 12-cloud-billing |
| changelogs, changelog_labels, changelog_entries, changelog_entry_label, changelog_entry_author, changelog_entry_article, changelog_entry_reaction_counts, changelog_reactions, changelog_reads, changelog_subscribers, changelog_mailings, changelog_mailing_recipients, changelog_sources, changelog_generations, changelog_entry_stats_daily, changelog_stats_daily | 13-changelog |
| feedback_settings, feedback_boards, feedback_categories, feedback_statuses, feedback_posts, feedback_post_tag, feedback_votes, feedback_comments, feedback_comment_mentions, feedback_requests, feedback_post_merges, feedback_subscriptions, feedback_post_updates, feedback_notifications, feedback_contact_states, feedback_suggestions, feedback_post_embeddings, feedback_imports, portal_sessions, portal_sign_ins | 14-feedback-roadmap |
| engage_settings, engage_assets, announcements, surveys, survey_questions, survey_responses, survey_answers, engage_receipts, engage_stats_daily | 15-announcements-surveys |
| member_access_teams, team_assignment_cursors, away_reasons, conversation_views, member_inbox_preferences, snooze_lists, conversation_drafts, bulk_operations, message_reactions, contact_tag, company_tag, conversation_attribute_definitions, redaction_rules, contact_erasures, data_exports (workspace-level), export_destinations, export_schedules, side_conversations, side_messages, message_translations; SQL function `dimmi_conversation_visible` | 03-shared-inbox |
| widget_translations, widget_assets, widget_site_imports, widget_domains | 01-messenger-widget |
| identity_verification_stats | 02-widget-identity |
| email_ignored_addresses, email_trusted_senders, email_mailbox_connections, email_mailbox_messages, email_address_tests | 04-email-channel |
| audiences, audience_references, attribute_definitions, attribute_problems, event_definitions, contact_event_summaries, audience_snapshots, audience_snapshot_members | 18-audiences |
| widget_devices, widget_push_credentials | 20-native-sdks |
| roles | 00-accounts |
| sso_connections, organization_domains, scim_tokens, scim_group_links | 21-enterprise-identity |
| imports, import_runs, import_run_stages, import_mappings, import_records, import_errors, import_files, import_erasures | 16-importers |

Exceptions to table rules: `usage_ledger` and billing tables append-only, no `updated_at`, no cascade from workspace purge (tax records). User-scoped (`user_id`, no `workspace_id`): `push_subscriptions` (06; `oauth_grant_id uuid null references oauth_grants on delete cascade`: app push rows die with grant, listener on `oauth_grant.revoked` also deletes; `device_id`), `sessions`, `user_auth_factors`, `oauth_identities`, `security_events`. Global (not tenant, no RLS): `organizations`, `encryption_keys`. Org-scoped (`-- lint:org-scoped`, RLS via `dimmi_visible_billing_accounts()`, 12): `billing_accounts`, `spend_caps`, `billing_periods`, `invoices_cache`, `credit_grants`, `credit_grant_applications`, `billing_adjustments`, `entitlement_overrides` (staff only, no tenant API).

Tenancy rule (P8): every tenant-owned row carries `workspace_id`, join tables included. Nothing scoped only through parent join. Lint: table with FK to tenant table must have `workspace_id` unless migration annotated `-- lint:user-scoped`, `-- lint:org-scoped` or `-- lint:global` (security.md "Tenancy isolation").

## Core table changes made by feature specs

Feature migrations that alter core tables. Builders apply in migration number order.

| Table | Change | Owner |
|---|---|---|
| users | deleted_at, password_changed_at, risk_flags, created_via (registry enum) | 00-accounts |
| users | is_instance_admin | 11-self-host |
| workspaces | created_by_user_id, deleted_by_user_id, purge_after, suspended_reason | 00-accounts |
| workspace_members | last_accessed_at, deactivated_at, deactivated_by_member_id, joined_via (registry enum) | 00-accounts |
| teams | deleted_at, partial unique name | 00-accounts |
| conversations | waiting_since, last_activity_at, last_message_id, previous_conversation_id, merged_into_id, is_test, search, list indexes | 03-shared-inbox |
| conversations | ai_reply_count, ai_cost_micros, ai_language, ai_handed_off_at, ai_handoff_reason | 07-dimmi-agent |
| messages | channel_connection_id, redefined search column | 03-shared-inbox |
| conversation_tag, conversation_reads, conversation_participants | add workspace_id | 03-shared-inbox |
| saved_replies | unique NULLS NOT DISTINCT | 03-shared-inbox |
| attachments | uploaded_at | 01-messenger-widget |
| jsonb keys | workspaces.settings.agent_access, messages.metadata.sent_via | 09-agent-native |
| attachments | feedback_post_id, feedback_comment_id (nullable FKs, cascade), CHECK at most one of `message_id`, `feedback_post_id`, `feedback_comment_id`, index `(workspace_id, feedback_post_id)` (`20261101001420`) | 14-feedback-roadmap |
| jsonb keys | messages.metadata.survey_request `{survey_id, response_id}`, conversations.source.announcement_id, messages.event name `announcement.replied` | 15-announcements-surveys |
| jsonb keys | messages.metadata.ai.confirmation, messages.metadata.ai.action_calls, messages.metadata.widget.identity_verified, messages.event names `ai.action_call_completed`, `ai.action_call_approval_requested` (03 registers) | 07-dimmi-agent (actions) |
| jsonb keys, registry values | conversations.source.imported `{import_id, source, external_id, external_number, external_url, channel}`, messages.metadata.imported (bool) + messages.metadata.import, `messages.dedupe_key` `imp:<connection_key>:<id>`, `channel_connections.type` value `import`, `attachments.uploaded_by_type` value `system`. No column change | 16-importers |
| workspace_members | conversation_access, conversation_access_unassigned (`20261201020001`); away_reassign, away_since, availability_source, away_reason_id, assign_on_reply, conversation_capacity, capacity_by_channel, last_assigned_at (`20261201020004`); undo_send_seconds (`20261201030014`); FK away_reason_id (`20261201120001`) | 03-shared-inbox |
| workspace_members | directory_external_id, directory_managed_at (`20261201080008`); custom_role_id (`20261201080009`, also invitations) | 21, 00-accounts |
| conversations | assignment_wait_reason, assignment_waiting_since (`20261201020006`); priority boolean to smallint 0..4 (`20261201030012`); custom_attributes (`20261201120004`) | 03-shared-inbox |
| messages | scheduled_send_at, undone_at (`20261201030014`) | 03-shared-inbox |
| conversation_participants | role values `to`, `cc`; removed_at, added_by (`20261201040004`) | 03-shared-inbox |
| conversation_tag | message_id (`20261201040006`) | 03-shared-inbox |
| contacts, companies | notes (`20261201040007`) | 03-shared-inbox |
| contacts | last_heard_at, web_sessions, conversation_count, GIN custom_attributes | 18-audiences |
| attachments | scan_status adds inconclusive, mismatch; scan_signature, scanned_at (`20261201040008`); side_message_id + one-target CHECK (`20261201120003`) | 03-shared-inbox |
| users | preferences (`20261201070001`); managed_by_organization_id (`20261201080005`) | 00-accounts, 21 |
| invitations | reminded_at (`20261201070002`) | 00-accounts |
| one_time_tokens | purpose adds two_factor_reset (`20261201070003`) | 00-accounts |
| oauth_identities, sessions, oauth_grants, oauth_authorization_requests | sso_connection_id, auth_domain where listed (`20261201080003`, `004`, `010`) | 21-enterprise-identity |
| jsonb keys | conversations.source.email_throttled, conversations.source.email_spoof_risk, messages.metadata.email.{trusted, unverified_sender, spoof_risk, external_send}, messages.metadata.from_non_participant (04 writes, 03 reads); messages.metadata.dev_context, messages.metadata.ai.run_id (01) | 04, 01 |

Feature tables changed by other features (owner applies in own block; listed for build order): `widget_settings` (01) gains `changelog_tab_enabled` (13), `feedback_tab_enabled` (14, migration `001421`); `knowledge_documents.changelog_entry_id` (13 migration `001314`); `email_suppressions.scope` adds `changelog` and `outbound_emails.kind` adds `changelog`, `feedback_update`, `portal_sign_in` (04 block); `feedback_posts.changelog_entry_id`, `feedback_post_updates.changelog_entry_id` FKs (declared inline by 14 migrations `001406` / `001414`; 13 block runs first); `ai_runs.kind` adds `changelog`, `feedback`, `docs_sync` (07 CHECK); `changelog_sources.github_repository_id` (13 table, FK to 08 `github_repositories`; 08 block runs first).

Migration number ranges: every migration built from 2026-11-04 on takes a track range `20261201TTNNNN`, see `specs/BUILD-TRACKS.md`; `00FFNN` numbers in specs are labels for what is unbuilt. Original ranges (app set, `database/migrations`): 00-accounts `20261101000001-000029` (organizations before workspaces), foundation core `000030-000099`, then one block per feature `00FFNN` = folder number `FF` (`000101-000199` for 01 ... `001201-001299` for 12), 13 `20261101001301-001399`, 14 `001401-001499`, 15 announcements/surveys `001501-001599` (folder renamed from 16; its old `0016NN` numbers move to `0015NN`), 16 importers `001601-001699`. Sub-ranges listed for grep (no rule change): 07 agent actions `000707-000710`, 08 docs sync + GitHub layer `000821-000827`. Cross-feature FK needing later table ships in the later block (13 `001314` adds `knowledge_documents.changelog_entry_id`). Framework set (`foundation.FrameworkMigrations()`: SQL embedded per owning package, one set, own version table `framework_migrations`) runs first, after migrate steps (River schema, notifications table); app set version table `app_migrations`. App migrations needing product columns on framework tables (`sessions`, `personal_access_tokens`, `oauth_*`, `notifications`) ALTER them.

## Public ID prefixes (registry)

One prefix per addressable table (pivots, stats, singletons, keyed-by-parent tables have none). New table, new row here. Encoding = TypeID (prefix + `_` + 26-char lowercase Crockford base32 of UUIDv7) via `framework/ids`; catalog `app/models/public_ids.go` registers every row (duplicate panics at boot). Token secrets (`dmi_live_`, `dmi_oat_`, `dmi_pts_` portal session, `whsec_`) are not IDs.

| Prefix | Table | Prefix | Table |
|---|---|---|---|
| ws_ | workspaces | usr_ | users |
| mem_ | workspace_members | team_ | teams |
| inv_ | invitations | tok_ | personal_access_tokens |
| ses_ | sessions | aud_ | audit_logs |
| idn_ | oauth_identities | dom_ | reserved: workspace_domains (later: `domain-join`) |
| sev_ | security_events | inbox_ | inboxes |
| chn_ | channel_connections | ctc_ | contacts |
| cmp_ | companies | conv_ | conversations |
| msg_ | messages | att_ | attachments |
| tag_ | tags | rpl_ | saved_replies |
| cid_ | contact_identities | | |
| exp_ | contact_exports | evt_ | events |
| wgt_ | widget_settings | wss_ | widget_sessions |
| sig_ | email_signatures | edm_ | email_domains |
| isk_ | identity_secrets (02 exposes as `kid`) | | |
| ema_ | email_addresses | ibm_ | inbound_emails |
| obm_ | outbound_emails | sup_ | email_suppressions |
| chr_ | channel_rooms | cml_ | channel_member_links |
| cie_ | channel_inbound_events | wtp_ | whatsapp_templates |
| ntf_ | notifications | psub_ | push_subscriptions |
| ndst_ | notification_destinations | ndlv_ | notification_deliveries |
| aia_ | ai_agents | aik_ | ai_provider_keys |
| air_ | ai_runs | ais_ | ai_run_steps |
| hc_ | help_centers | col_ | collections |
| art_ | articles | artv_ | article_versions |
| sug_ | article_suggestions | sugb_ | article_suggestion_batches |
| hcd_ | help_center_domains | rdr_ | help_center_redirects |
| ast_ | help_center_assets | ks_ | knowledge_sources |
| ksr_ | knowledge_sync_runs | kd_ | knowledge_documents |
| kc_ | knowledge_chunks | ocl_ | oauth_clients |
| ogr_ | oauth_grants | oar_ | oauth_authorization_requests |
| odr_ | oauth_device_codes | drf_ | conversation_drafts (03; 09 server drafts reuse table) |
| whk_ | webhook_endpoints | whd_ | webhook_deliveries |
| wha_ | webhook_delivery_attempts | bak_ | backups |
| iex_ | data_exports (instance) | | |
| ba_ | billing_accounts | ul_ | usage_ledger |
| ivc_ | invoices_cache | cg_ | credit_grants |
| bd_ | billing_disputes | bpa_ | billing_program_applications |
| bri_ | billing_reconciliation_issues | ott_ | one_time_tokens (email verification, email change; public id for REST redemption) |
| prs_ | password_reset_tokens | org_ | organizations |
| fac_ | user_auth_factors | ent_ | entitlement_overrides |
| clg_ | changelogs (13) | cle_ | changelog_entries (13) |
| cll_ | changelog_labels (13) | csub_ | changelog_subscribers (13) |
| clm_ | changelog_mailings (13) | cls_ | changelog_sources (13) |
| cgn_ | changelog_generations (13) | | |
| fbd_ | feedback_boards (14) | fbct_ | feedback_categories (14) |
| fbs_ | feedback_statuses (14) | fbp_ | feedback_posts (14) |
| fbv_ | feedback_votes (14) | fbc_ | feedback_comments (14) |
| fbr_ | feedback_requests (14) | fbm_ | feedback_post_merges (14) |
| fbsb_ | feedback_subscriptions (14) | fbu_ | feedback_post_updates (14) |
| fbn_ | feedback_notifications (14) | fbsg_ | feedback_suggestions (14) |
| fbi_ | feedback_imports (14) | pts_ | portal_sessions (14) |
| psi_ | portal_sign_ins (14) | | |
| adn_ | audiences (18) | ann_ | announcements (15) |
| svy_ | surveys (15) | svq_ | survey_questions (15) |
| svr_ | survey_responses (15) | erc_ | engage_receipts (15) |
| eas_ | engage_assets (15) | | |
| acn_ | ai_action_connections (07) | aac_ | ai_actions (07) |
| aacv_ | ai_action_versions (07) | aax_ | ai_action_calls (07) |
| ghc_ | github_connections (08) | ghr_ | github_repositories (08) |
| dss_ | docs_sync_sources (08) | dsr_ | docs_sync_runs (08) |
| asr_ | article_source_refs (08; `article_freshness` keyed by article, no prefix) | | |
| imp_ | imports (16) | impr_ | import_runs (16) |
| impm_ | import_mappings (16) | impf_ | import_files (16) |
| wga_ | widget_assets (01) | wsi_ | widget_site_imports (01) |
| wdm_ | widget_domains (01) | wdv_ | widget_devices (20) |
| wpc_ | widget_push_credentials (20) | cvw_ | conversation_views (03) |
| blk_ | bulk_operations (03) | awr_ | away_reasons (03) |
| cad_ | conversation_attribute_definitions (03) | rxr_ | redaction_rules (03) |
| ces_ | contact_erasures (03) | dex_ | data_exports (03, workspace) |
| dxd_ | export_destinations (03) | dxs_ | export_schedules (03) |
| sdc_ | side_conversations (03) | sdm_ | side_messages (03) |
| atd_ | attribute_definitions (18) | edf_ | event_definitions (18) |
| asn_ | audience_snapshots (18) | ssoc_ | sso_connections (21) |
| odom_ | organization_domains (21) | sct_ | scim_tokens (21) |
| scg_ | scim_group_links (21) | rol_ | roles (00-accounts) |
| eig_ | email_ignored_addresses (04) | ets_ | email_trusted_senders (04) |
| eat_ | email_address_tests (04) | emc_ | email_mailbox_connections (04) |
| itg_ | integration_installations (23) | itu_ | integration_member_grants (23) |

Reserved for later tables (do not reuse; `00-foundation/extensibility.md`): `brd_` brands, `lgh_` legal_holds, `ains_` app_installations, `dev_` user_devices. `encryption_keys` has no public id (internal). Prefix uniqueness checked over registry + reserved list (CI: `app/models/public_ids.go` test; 2026-11-02 final pass: zero collisions across registry + reserved list incl. 07 actions, 08 docs sync, 16 importers).

## Accounts

### organizations
Global, not tenant (no `workspace_id`, no RLS). Enterprise things hang off org, never workspace (P4: SSO, SCIM, policies, billing owner, CMK, entitlement overrides). MVP: exactly one per workspace, created by `CreateWorkspace` (00-accounts) in same tx before workspace row, invisible in API (no `organization_id` in resources). Reached only via `workspace -> organization` join in actions or SECURITY DEFINER functions. MVP org admin = owners of its workspace(s) (derived; no `organization_members`).
| Column | Type | Notes |
|---|---|---|
| id | uuid pk default uuidv7() | public `org_` |
| name | text not null | copy of first workspace name; renamed with workspace while 1:1 |
| created_by_user_id | uuid null references users on delete set null | |
| settings | jsonb not null default '{}' | org-scope settings (registry scope `organization`, contracts.md "Settings registry"). Empty at MVP |
| deleted_at | timestamptz | set by `PurgeWorkspace` when org has no live workspace; row kept while billing retention needs it |
| created_at, updated_at | timestamptz | |

Attached to org at MVP (all 1:1, behavior identical): billing account (12), `entitlement_overrides` (12), `encryption_keys` scope `organization` (reserved; CMK later), settings inheritance level. Stay on workspace: members, roles, inboxes, all content, `security.require_two_factor`, `workspace_domains`, `audit_logs`.

### workspaces
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| organization_id | uuid not null references organizations | no cascade; index `(organization_id)`. Not exposed in API at MVP |
| name | text not null | 1 to 80 chars |
| slug | citext unique not null | 3 to 40, `[a-z0-9-]`, used in help center default domain |
| timezone | text not null default 'UTC' | IANA name, validated |
| locale | text not null default 'en' | BCP 47 |
| settings | jsonb not null default '{}' | small, typed in `models.WorkspaceSettings` |
| test_mode | boolean not null default false | sandbox workspace, never billable |
| next_conversation_number | bigint not null default 1 | allocator for `conversations.number` via `UPDATE ... RETURNING` in same transaction |
| suspended_at | timestamptz | abuse or billing; blocks inbound processing except storage |
| deleted_at | timestamptz | soft delete, hard purge after 30 days job |

### users
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| name | text not null | |
| email | citext unique not null | |
| email_verified_at | timestamptz | |
| password_hash | text | argon2id; null when only OAuth/magic link |
| two_factor_confirmed_at | timestamptz | set while user has at least one active second factor in `user_auth_factors` (driver capability `SecondFactor`; recovery codes excluded); cleared when last removed. Maintained in same tx by factor actions (00-accounts). Factor secrets live in `user_auth_factors`, never on `users` |
| avatar_path | text | filesystem path |
| locale, timezone | text | user prefs, fall back to workspace |
| last_seen_at | timestamptz | |

### workspace_members
`id, workspace_id, user_id, role text CHECK in ('owner','admin','member') (role key; permissions from `app/auth/roles.go`, policies never compare role names, P2), availability text CHECK in ('online','away') default 'online', status text CHECK in ('active','deactivated') default 'active', created_at, updated_at`. Unique `(workspace_id, user_id)`. At least one active owner per workspace (enforced in action, plus deferred check job).

### teams, team_members
`teams: id, workspace_id, name, created_at, updated_at`, unique `(workspace_id, lower(name))`.
`team_members: team_id, workspace_id uuid not null, workspace_member_id, created_at`, primary key `(team_id, workspace_member_id)`. `workspace_id` for RLS (P8), equals team's workspace.

Other account tables (user_auth_factors, oauth_identities, invitations, password_reset_tokens, security_events, audit_logs) specified in `specs/00-accounts`; `sessions`, `personal_access_tokens` created by framework set, 00-accounts ALTERs product columns. Registry enums there: `user_auth_factors.type`, `sessions.auth_method`, `users.created_via`, `oauth_identities.provider`, `security_events.type` (dotted), `workspace_members.joined_via`, `audit_logs.actor_type`, `audit_logs.via_kind` (must accept every `contracts.ActorVia.Kind`, incl. `app`).

## Inboxes and channels

### inboxes
Routing container. Every channel connection belongs to one inbox. Workspace gets default inbox "Support" on create.
`id, workspace_id, name, default_team_id null references teams on delete set null, settings jsonb, created_at, updated_at, deleted_at`.

### channel_connections
One connected source: widget install, email address, Slack workspace, Discord server, Telegram bot, WhatsApp number.
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id | uuid | |
| inbox_id | uuid not null | |
| type | text not null, registry enum | MVP registry: `widget`, `email`, `slack`, `discord`, `telegram`, `whatsapp`, `api` (`api` = conversations created through REST), `import` (registered by 16: imported conversations, receive-only; replies rehome via 03 `ChannelRehomer`). Channel specs register types in Go registry, no CHECK migration |
| name | text not null | display |
| status | text CHECK in ('pending','active','error','disconnected') | |
| external_id | text | provider routing key: Slack team id, Discord guild id, Telegram bot id, WhatsApp phone number id, email address |
| config | jsonb not null default '{}' | non-secret settings |
| credentials_encrypted | bytea | tokens, secrets |
| last_error | text | human readable |
| last_error_at, last_activity_at | timestamptz | |
| deleted_at | timestamptz | |
Unique `(type, external_id) WHERE deleted_at IS NULL AND external_id IS NOT NULL` (inbound routing must map to exactly one workspace).

## Customers

### contacts
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id | uuid | |
| kind | text CHECK in ('user','lead') | `user` = identified by host app (verified JWT or API). `lead` = anonymous or unverified |
| external_id | text | host app user id; unique `(workspace_id, external_id)` where not null |
| email | citext | not unique (leads may share), index `(workspace_id, email)` |
| email_verified | boolean not null default false | true only from: verified JWT carrying claim `email_verified: true` (02), API with token, or inbound email passing aligned DMARC (04) |
| name | text | |
| phone | text | E.164 |
| avatar_url | text | |
| company_id | uuid null references companies on delete set null | primary company. JWT `company` claim maps here. Never assume one company per contact in metering or webhooks (later `contact_companies` additive) |
| custom_attributes | jsonb not null default '{}' | max 100 keys, key 1 to 64 chars `[a-z0-9_]`, value scalar or string up to 1024 chars |
| trusted_attribute_keys | text[] not null default '{}' | keys of `custom_attributes` whose current value came from trusted source: 02 verified JWT claims, token API upsert (03), 16 import mappings admin marked `trusted` (`kind user` only; imports untrusted by default), integrations. Widget lead `PATCH /widget/v1/contact` (01) removes key it writes. Merge keeps key only when surviving value came from trusted source. 07 agent actions bind identity only to these keys (`07-dimmi-agent/actions.md` "Identity binding") |
| locale, timezone | text | detected or provided |
| location | jsonb | country, region, city from IP (optional GeoIP file) |
| first_seen_at, last_seen_at | timestamptz | |
| blocked_at | timestamptz | blocked contacts: inbound stored as spam, never notified, never billed |
| merged_into_id | uuid null | after merge, row kept as tombstone for redirects |
| search | tsvector generated | name, email, phone, external_id |
| deleted_at | timestamptz | GDPR erase: see security.md |

### contact_identities
How one person appears on each channel. Drives cross-channel merge.
`id, workspace_id, contact_id, type text not null (registry enum), value text, verified boolean, created_at`. Unique `(workspace_id, type, value)`. MVP registry: `email`, `phone`, `widget_visitor`, `slack_user`, `discord_user`, `telegram_user`, `whatsapp_wa_id`, `external_id` (foundation), `whatsapp_bsuid` (registered by 05 in Go registry, no migration: Meta business-scoped user id, used when Meta omits phone number), `portal_visitor` (registered by 14: anonymous portal session lead, value = `pts_` id). New channel identity types = Go registry entry, never CHECK swap (big table: full validation scan).

Merge rules (03-shared-inbox owns action `MergeContacts`): auto-merge only on verified identity match (verified email, verified external_id, phone from WhatsApp). Never auto-merge on unverified email (prevents impersonation: lead typing someone else's email must not see their history).

### companies
`id, workspace_id, external_id, name, domain citext, custom_attributes jsonb, created_at, updated_at, deleted_at`. Unique `(workspace_id, external_id)` where not null. Created from JWT `company` claim or API.

## Conversations

### conversations
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id | uuid | |
| number | bigint not null | per-workspace sequence, human reference `#1042`; unique `(workspace_id, number)`; allocated from `workspaces.next_conversation_number` |
| inbox_id | uuid not null | |
| channel_connection_id | uuid not null | origin channel |
| contact_id | uuid not null | primary customer |
| subject | text | email subject, or first message excerpt (120 chars) |
| status | text CHECK in ('open','snoozed','closed') | |
| spam | boolean not null default false | separate from status so un-spam restores status |
| priority | smallint not null default 0, CHECK 0..4 | none, low, medium, high, urgent. Was boolean; 03 W1 migrates |
| assignee_member_id | uuid null references workspace_members | |
| assignee_team_id | uuid null references teams | |
| ai_state | text CHECK in ('off','active','handed_off') default 'off' | 07 owns transitions |
| snoozed_until | timestamptz | required when status snoozed |
| external_thread_key | text | channel thread identity (email root Message-ID, Slack channel+thread_ts, Discord thread id, Telegram chat id, WhatsApp wa_id); unique `(channel_connection_id, external_thread_key)` where status <> 'closed' |
| source | jsonb | page URL, referrer, user agent, browser, OS, app version, IP country |
| first_customer_message_at, last_customer_message_at, first_reply_at, last_reply_at, last_message_at | timestamptz | maintained by actions; drive sorting, SLA, metering |
| closed_at | timestamptz | |
| message_count | int not null default 0 | |
| deleted_at | timestamptz | hard delete only via GDPR or spam purge |
Indexes: `(workspace_id, status, last_message_at desc)`, `(workspace_id, assignee_member_id, status)`, `(workspace_id, contact_id, created_at desc)`, `(workspace_id, inbox_id, status)`.

State machine (03 owns):
```
open -> snoozed (snooze, needs snoozed_until in future)
snoozed -> open (snooze expires, customer message, manual)
open|snoozed -> closed (close)
closed -> open (customer message, manual reopen, teammate reply with reopen flag)
any -> spam=true (mark spam), spam=true -> spam=false (unmark)
```
Customer message on closed conversation reopens it. Exception: closed more than `inbox.settings.reopen_window_days` (default 30) ago, then new conversation linked via column `previous_conversation_id` (added by 03). Email threads follow same rule. Teammate reply on closed conversation keeps it closed unless request sets `reopen: true` (03).

### conversation_participants
Extra people on thread (email CC, Slack channel members who wrote, second contact).
`conversation_id, contact_id, role text CHECK in ('to','cc'), removed_at, added_by, created_at`, primary key both.

### messages
One row per thing in timeline: customer message, teammate reply, AI reply, internal note, system event.
| Column | Type | Notes |
|---|---|---|
| id | uuid | |
| workspace_id, conversation_id | uuid | |
| type | text CHECK in ('message','note','event') | `note` internal only: never sent to channel, never in widget, never in webhooks unless endpoint opts in with `include_notes` (10) |
| author_type | text CHECK in ('contact','member','agent','system','integration') | `agent` = Dimmi Agent or external AI agent token |
| author_id | uuid | contact id, member id, ai_agents id, null for system |
| body | text | canonical Markdown (CommonMark subset). Max 64 KB |
| body_html | text | sanitized original HTML (email) for faithful rendering; never trusted |
| event | jsonb | for type event: `{"name":"conversation.assigned","from":...,"to":...}` |
| channel_message_id | text | provider message id (email Message-ID, Slack ts, Discord message id, Telegram message_id, WhatsApp wamid) |
| dedupe_key | text | unique `(workspace_id, dedupe_key)` where not null; inbound retries never create duplicates |
| delivery_status | text CHECK in ('pending','sent','delivered','read','failed','not_applicable') | outbound only |
| delivery_error | text | |
| substantive | boolean not null default true | false for auto-acknowledgements, OOO, CSAT prompts (metering D2) |
| metadata | jsonb | AI citations, token usage, email headers subset, client info |
| edited_at, sent_at, read_at | timestamptz | `edited_at`: note edits only at MVP (reply edit later: `message-edit`) |
| deleted_at | timestamptz | deleted by author or on source channel; body cleared, row kept |
| search | tsvector generated from body | GIN index `(workspace_id, search)` via `btree_gin` |
Indexes: `(workspace_id, conversation_id, created_at)`, GIN `(workspace_id, search)`.

Text search: generated columns cannot call `unaccent()` (not IMMUTABLE). Use custom configuration `dimmi_search` built with `unaccent` dictionary (03 `search.md` defines). English stemmer for all workspaces at MVP.

### attachments
`id, workspace_id, message_id null, uploaded_by_type, uploaded_by_id, disk text, path text, filename text, content_type text, size_bytes bigint, sha256 text, width int, height int, inline boolean, content_id text (email cid), scan_status text CHECK in ('pending','clean','infected','skipped'), created_at`.
Uploads land with `message_id null`; job purges unattached after 24 hours. Max size `filesystems.max_upload_mb` (default 25). Served only through signed URLs, `Content-Disposition: attachment` unless image/pdf allowlist.

### tags, conversation_tag
`tags: id, workspace_id, name citext, color text, created_at, updated_at`. Unique `(workspace_id, name)`.
`conversation_tag: conversation_id, tag_id, created_by_type, created_by_id, created_at`. Primary key both.

### conversation_reads
Unread state per teammate. `conversation_id, workspace_member_id, last_read_message_id, last_read_at`. Primary key both.

### saved_replies
`id, workspace_id, owner_member_id null (null = shared), title, body, shortcut citext, created_at, updated_at`. Unique `(workspace_id, owner_member_id, shortcut)`. Variables: `{{contact.first_name}}`, `{{member.name}}`, `{{conversation.number}}`, unknown variables left blank, never error.

## Infrastructure tables

| Table | Owner | Columns | Purpose |
|---|---|---|---|
| sessions | framework (`framework/session`, user-scoped) | id, token_hash bytea unique (SHA-256 of cookie token), user_id null, data jsonb, flash jsonb, ip, user_agent, authenticated_at, confirmed_at, last_activity_at, expires_at, created_at; idx `(user_id, last_activity_at desc)`, `(expires_at)` | cookie sessions, per-user device list + revoke; 00-accounts ALTERs product columns (`auth_method`, `auth_methods`, `device_id`, `device_label`, `country`, `confirmed_method`) |
| personal_access_tokens | framework (`framework/auth/token`) | id, owner_id, tenant_id text null, name, hint, token_hash bytea unique, abilities text[], expires_at, last_used_at, last_used_ip, revoked_at, created_at; idx `(owner_id, created_at desc)` | PATs; only digest stored; 00-accounts ALTERs product columns (`workspace_id` + RLS, `expiry_notified_at`, `revoked_by_member_id`); `owner_id` = member, `tenant_id` = workspace |
| oauth_clients, oauth_authorization_codes, oauth_grants, oauth_tokens | framework (`framework/oauth`) | clients: id text, secret_hash, name, redirect_uris, scopes, grant_types, first_party, dynamic. codes: code_hash, client_id, user_id, tenant_id, scopes, redirect_uri, code_challenge, auth_time, grant_id, expires_at, used_at. grants: client_id, user_id, tenant_id, first_party (CHECK first_party → tenant_id null), scopes, auth_time, confirmed_at, revoked_at. tokens: grant_id fk cascade, kind `access`/`refresh`, token_hash unique, parent_id (refresh family), scopes, expires_at, used_at, revoked_at | OAuth 2.1 server storage; 09 ALTERs product columns |
| idempotency_keys | framework (`framework/http/idempotency`) | tenant_id text default '' (= workspace id; generic name), scope, key, fingerprint, status, header jsonb, body bytea, created_at, expires_at; pk `(tenant_id, scope, key)`; idx `(expires_at)` | replay same response 24 hours; fingerprint mismatch returns 422 `idempotency_key_reused`, in flight 409 `idempotency_key_in_use`; `Prune` daily |
| notifications | framework (`framework/notifications/channels/database`, migrate step per configured table) | id, notifiable_type, notifiable_id text, type, data jsonb, read_at, created_at; idx `(notifiable_type, notifiable_id, created_at desc)` | database channel rows; 06 ALTERs product columns |
| river_* | framework (`framework/queue`, River migrations as migrate step `queue.Migration()`) | River managed | jobs, periodic jobs, leader election (transactional enqueue: stays in Postgres) |
| framework_migrations, app_migrations | framework / app | goose managed | two version tables: framework set, app set |
| certmagic_storage | framework (`framework/customdomain/storage`) | key text pk, value bytea, modified timestamptz | non-tenant (`-- lint:global`): ACME account + certs shared by replicas; `Locker` = Postgres advisory locks (no lock column); durable (never Valkey: ACME rate limits). 08 wires driver |
| encryption_keys | foundation (app store behind `encryption.KeysetStore`) | see below | wrapped Tink keysets per scope + purpose (security.md "Secrets") |

Framework migration set = `sessions`, `personal_access_tokens`, `oauth_*` (4), `idempotency_keys`, `certmagic_storage`; migrate steps before it = River schema, `notifications`. No `cache`, `rate_limits`, `realtime_events`, `realtime_publications`, `realtime_presence` tables: Valkey owns that state (PO decision 2026-11-02, below).

Bulk load and RLS: COPY cannot load tables with row-level security for the app role. `tx.CopyFrom` only for non-RLS tables or a staging table (COPY into staging, then `INSERT ... SELECT` under tenant setting); tenant tables otherwise batched inserts (16-importers).

### Valkey (required, not a database of record)

Every deployment runs Valkey (app + Postgres + Valkey, incl. $5 VPS). Rule: **Valkey holds only rebuildable state**; loss resets caches, limits, presence and realtime history, never business data.

| Keyspace (prefix) | Owner | What |
|---|---|---|
| `dimmi:rt:` | `framework/broadcasting` (centrifuge Redis broker + presence) | realtime publications, history streams, presence, node control |
| `dimmi:cache:` | `framework/cache` | shared cache, `Remember` entries, `Lock`/`TryLock` (rueidislock; non-critical; correctness locks = Postgres advisory locks); keyspace from `cache.keyspace` |
| `dimmi:rl:` | `framework/ratelimit` | rate-limit counters |
| `dimmi:ps:` | `framework/pubsub` | internal cross-replica bus (cache bust, entitlement bust) |
| `dimmi:once:` | app via `cache.Add` (`SET NX EX`) | replay/dedupe windows: JWT `nonce`/`jti` (02, 14 portal), provider webhook replay (Mailgun, Slack, Telegram), inbound dedupe windows. Durable dedupe keys stay in Postgres where spec stores provider event rows (`messages.dedupe_key`, `stripe_events`, `channel_inbound_events`) |
| `dimmi:ctr:` | app | typing, collision (viewers), short-lived unread/badge counters |
| `dimmi:nodes:` | 11 (`app/support/nodes`) | node heartbeat registry (sorted-set index `dimmi:nodes` scored by heartbeat + hash per process `EX 60`; listing never SCANs) |

Key rules (hold on standalone, Sentinel and cluster; 11 operations.md section 11.3):
- Keys built only through `framework/redis` `Key(keyspace, tag, parts...)`. Spec key patterns like `ai:slot:{ws}:...` write placeholders; the real hash tag comes from `Key`.
- Every multi-key command (Lua script, `MULTI`, multi-key `DEL`, rate-limit window pair) uses keys sharing one hash tag, so all land in one cluster slot (no `CROSSSLOT`).
- Batch reads across tags (`cache.Many`) go through the client's per-slot batching, never one raw `MGET`.
- No `SCAN`/`KEYS` on request or job paths; registries keep an index key. `SCAN` allowed only in operator CLI commands (`dimmi restore`) and `cache.Flush` (own keyspace; operators, tests), run on every primary.
- No logical database index in production config; `FLUSHDB`/`FLUSHALL` never issued (Valkey may be shared or clustered): bulk delete = prefix SCAN + `UNLINK`.
- No replica reads.

Postgres keeps: River queue (transactional enqueue), sessions, tokens, audit, `idempotency_keys`, billing ledger, advisory locks, FTS + pgvector, all durable data.

Valkey unreachable at runtime (binding policy, final pass 2026-11-02; owner specs pick codes):
| Use | Behavior | Examples |
|---|---|---|
| Auth and abuse guards | **fail closed**: 503 + `Retry-After: 5` (`service_unavailable`, or owner code like 00-accounts `auth_busy`) | login/sign-in limiter + lockout counters (00-accounts), magic link / email code issue, portal sign-in start + redeem and anonymous votes (14), widget PoW creates (01), JWT `nonce` replay check (02: nonce tokens only), provider webhook replay check (04: 503, provider retries) |
| Outbound provider limiters | **fail closed by deferral**: `RateLimitedError` snooze, job retries; never exceed provider limits | 04 automated/contact mail, 05 channel sends |
| Agent action limiters (07) | **fail closed, degrade**: no action offered in run (eligibility false, flag `action_blocked`), agent answers from knowledge or hands off; no 503 to customer; never calls customer API unmetered | `ai.actions.calls_hourly` counter, per-action/per-conversation call limiters (07 actions.md) |
| Everything else | **fail open**: proceed, `log.Warn` + metric `dimmi_valkey_fallback_total{subsystem}` | authenticated API rate limits, per-IP limits on public reads, cache (miss reads Postgres), pubsub busts (staleness bounded by L1 TTL), counters, typing, fast-path dedupe backed by durable unique index (05), 15 engage event limiter, 10 per-endpoint delivery limiter (deliveries proceed; River concurrency still bounds) |
| Spend caps with Postgres fallback | enforced from Postgres | 08 Ask AI per-help-center day cap, 07 spend cap, 12 billing (never reads Valkey) |
- Realtime down until Valkey returns; clients reconnect, `recovered: false`, refetch via REST (widget polls).
- Health: `/up` stays 200 (never restart a process for Valkey). `/ready` returns 503 `{"status":"degraded","checks":{"valkey":"unreachable"}}` while Valkey unreachable (11 decision 18): load balancer drains; single node keeps serving degraded under rules above. Boot waits `redis.connect_retry` (60 s) then exits 1.

### encryption_keys
Global (`-- lint:global`), no public id. App table behind `encryption.KeysetStore` (`Find`, `Create`, `Replace` compare-and-swap, `All`). One row per (scope, purpose): a Tink keyset wrapped by the key encryption key (KEK), not one row per data key. Rotation adds a key inside the keyset (new primary, old keys stay to decrypt); ciphertext carries Tink key-id prefix, so decrypt finds the key without guessing.
| Column | Type | Notes |
|---|---|---|
| scope_kind | text not null CHECK in ('system','organization','workspace') | MVP uses `system`, `workspace`; `organization` for CMK later |
| scope_id | text not null | '' for `system`; org or workspace id otherwise (no FK: rows outlive purge until crypto-shred) |
| purpose | text not null CHECK in ('aead','stream','mac') | values, streaming AEAD (backups, exports), deterministic MAC (tombstones, lookup hashes) |
| wrapped | bytea not null | keyset wrapped by current KEK (`encryption.kek`: `local` from app key, `awskms`); AAD binds scope + purpose |
| created_at, updated_at | timestamptz | |
Primary key `(scope_kind, scope_id, purpose)`. Each keyset created on first use. `Rotate(scope)` rotates `aead` + `stream` only; MAC keys never rotated automatically (deterministic lookups must keep matching). KEK change: `RewrapAll` re-wraps every row while old KEK listed in `encryption.previous` (no data re-encryption). Crypto-shred = delete row (`ErrKeysetNotFound` after).

## Global limits (config, defaults)

| Limit | Default |
|---|---|
| message body | 64 KB Markdown |
| attachments per message | 10 (inbound email override 50, 04) |
| attachment size | 25 MB |
| custom attributes per contact/company | 100 keys |
| tags per conversation | 50 |
| participants per conversation | 50 |
| conversations per contact list page | 100 max `limit` |
| API page size | default 25, max 100 |
