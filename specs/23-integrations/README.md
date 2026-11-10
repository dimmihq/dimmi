# 23 Integrations: OAuth connections to other tools

Date: 2026-11-10. Style: caveman. Status: approved direction (owner 2026-11-10), not built. Read with `00-conventions/README.md`, `00-foundation/*`, `05-chat-app-channels/README.md` (first consumer), `06-notifications/delivery.md` "Slack", `22-operator/README.md` "Surfaces". Evidence: `research/channels/slack.md`.

## Goal

dimmi connects to tools a team already uses (Slack first; Linear, Jira, HubSpot, Stripe next) with OAuth 2 authorization code flow. One installation per tool account, shared by every feature that needs that tool. Today 05 (Slack channel) and 06 (Slack notifications) each planned own Slack OAuth: two consent screens, two bots in customer's Slack, two token stores, two uninstall paths. 23 replaces both with one.

Direction: dimmi is OAuth **client** here. Not `framework/oauth` (dimmi as OAuth server for agents, 09) and not `framework/socialite` (sign in with GitHub/Google, 00-accounts).

Not covered (stay where they are):
- 08 shared GitHub layer: GitHub App installs (installation tokens per org/repo, one webhook). May register as 23 provider later; no change now.
- 07 agent action connections (`acn_`): pasted API keys per action server. Later `action-oauth` reuses 23 member grants or installations for OAuth-only APIs.
- 04 mailbox OAuth (Gmail, Microsoft): own flow in `04-email-channel/mailbox-connections.md`. Later move onto 23 (same shape), not at launch.

## Principles

1. **One installation per (workspace, provider, provider account).** Slack: per Slack team. Every feature reads that row. No feature runs its own OAuth.
2. **Two token kinds.** Installation token (bot / app token, installed by admin, acts as the app). Member grant (user token, one per member per installation, acts as that person: Slack replies posted as teammate's own Slack user). A member grant never outlives its member or its installation.
3. **Scopes are declared, never guessed.** Each feature registers scopes it needs at boot (`integrations.RegisterScopes(provider, feature, Scopes{Bot, User})`). Install asks union of enabled features. Installation missing scope for a feature: feature shows "Update Slack permissions" (re-consent with added scopes), never fails silently.
4. **Secrets sealed.** Tokens envelope-encrypted (`a.Encrypter`, purpose `integration_credentials`), never in resources, logs, audit diffs, events or MCP output. Redaction patterns `xoxb-`, `xoxp-`, `xoxe-`, `xapp-`, `lin_oauth_` tested.
5. **Refresh is single-flight.** Expiring tokens refreshed on use when within `integrations.refresh_skew` of expiry, under Valkey lock per installation; losers re-read row. Refresh failure with `invalid_grant`: status `error`, consumers see `ErrIntegrationBroken`.
6. **Provider-side revoke wins.** Uninstall or revoke reported by provider (Slack `app_uninstalled`, `tokens_revoked`) marks installation or grant `disconnected` at once and emits `integration.disconnected`; consumers react through listeners, never poll.
7. **Self-host same code.** Provider app credentials from `config/services.go`. Self-host without shared app: manual installation (`source = manual`) from pasted tokens (Slack custom app from 05 manifest) creates same row; consumers cannot tell difference.

## Framework: `framework/oauthclient`

Generic OAuth 2 authorization code client for connecting accounts (not signing in). Wraps `golang.org/x/oauth2` (already allowed). Shares state + PKCE store with socialite: move `framework/socialite/internal/oauth` to `framework/oauthclient/internal/flow`, socialite imports it.

| API | Behavior |
|---|---|
| `a.OAuthClient.Redirect(ctx, provider, Request{Scopes, UserScopes, RedirectURL, Extra}) (authURL, state, error)` | state single use, TTL `oauth_client.state_ttl` (10 min), bound to browser session id; PKCE S256 when driver supports |
| `a.OAuthClient.Callback(ctx, provider, state, query) (Grant, error)` | verifies state + session, exchanges code. `Grant{Access, Refresh *Token, Scopes, UserScopes []string, Account, App string, Raw map[string]any}`; `Token{Value, Expiry}` |
| `a.OAuthClient.Refresh(ctx, provider, refresh) (Grant, error)` | `ErrInvalidGrant` when provider refuses |
| `a.OAuthClient.Revoke(ctx, provider, token) error` | best effort; `ErrUnsupported` when provider has none |
| `Driver(name)`, `Extend(name, factory)`, `Fake()` | manager per conventions; fake testing env only; `drivertest` contract suite |

Drivers: `generic` (endpoints from config, standard token response), `slack` (`oauth.v2.access`: bot token top level, user token under `authed_user`, `team.id` to `Account`, `app_id` to `App`, `enterprise` + `is_enterprise_install` in `Raw`; scopes comma lists; revoke `auth.revoke`). Later `linear`, `atlassian`, `hubspot` as `generic` config where possible.

Errors: `ErrInvalidState`, `ErrDenied` (user pressed cancel), `ErrExchange`, `ErrInvalidGrant`, `ErrProvider`, `ErrMisconfigured`, `ErrUnsupported`.

## Data model

Migrations stamped by `make gen ARGS="make:migration ..."` in track range assigned when 23 track opens (`BUILD-TRACKS.md`). Prefixes `itg_`, `itu_` (added to `00-foundation/data-model.md`). Tenant tables: RLS in cloud, indexes lead with `workspace_id`.

#### integration_installations (`itg_`)

| Column | Type | Notes |
|---|---|---|
| provider | text not null, registry enum `enums.IntegrationProvider` | `slack`; later `linear`, `jira`, `hubspot` with no migration |
| external_account_id | text | Slack team id `T...`; null while manual install pending |
| external_app_id | text | Slack app id `A...` (shared app or custom app) |
| name | text not null | provider account name, e.g. "Acme" Slack team; refreshed on health check |
| source | text not null CHECK in ('oauth','manual') | manual = pasted tokens (self-host custom app) |
| scopes | text[] not null default '{}' | granted bot/app scopes, from provider response |
| credentials_encrypted | bytea not null | sealed `{access, access_expiry, refresh, refresh_expiry, signing_secret?, app_token?}` |
| status | text not null default 'active' CHECK in ('active','error','disconnected') | |
| last_error_code, last_error | text | `slack_auth_failed`, `refresh_invalid_grant`, ... human text from catalog |
| metadata | jsonb not null default '{}' | provider facts: Slack `bot_user_id`, `bot_id`, `team_domain`, `enterprise_id`, `icon_url` |
| installed_by_member_id | uuid references workspace_members on delete set null | |
| installed_at, last_checked_at, disconnected_at, deleted_at | timestamptz | |

Unique `(provider, external_app_id, external_account_id) WHERE deleted_at IS NULL AND status <> 'disconnected'`: one provider account feeds one dimmi workspace per app (shared Slack app: one dimmi workspace per Slack team, same as 05). Index `(workspace_id, provider, status)`.

#### integration_member_grants (`itu_`)

| Column | Type | Notes |
|---|---|---|
| integration_installation_id | uuid not null references integration_installations on delete cascade | |
| member_id | uuid not null references workspace_members on delete cascade | |
| external_user_id | text not null | Slack user id `U...` (from `authed_user.id`) |
| scopes | text[] not null default '{}' | user scopes granted |
| credentials_encrypted | bytea not null | sealed user token (+ refresh when provider issues one) |
| status | text not null default 'active' CHECK in ('active','revoked','error') | |
| created_at, revoked_at | timestamptz | |

Unique `(integration_installation_id, member_id)`; unique `(integration_installation_id, external_user_id)`. Member grant also writes 05 `channel_member_links` row (`source = oauth`) for Slack: same person, proven by provider.

Consumers point at installation, never copy tokens:
- 05 `channel_connections.integration_installation_id uuid null references integration_installations on delete set null` (Slack shared and custom; other channel types null).
- 06 `notification_destinations.config.integration_installation_id` for `slack_app` destinations.
- 22 Operator reads installation via `integrations.For(ctx, workspace, "slack")`.

## Slack: one app, every feature

| Feature | Owner | Bot scopes | User scopes | Events |
|---|---|---|---|---|
| Customer channels, DMs, rooms, interactivity | 05 | providers.md manifest list | none | 05 list |
| Triage + discussion threads in internal channels | 05 | `chat:write`, `chat:write.public`, `channels:join` | none | same |
| Notifications to channel and teammate DMs | 06 | `chat:write`, `chat:write.public`, `im:write`, `users:read`, `users:read.email` | none | none extra |
| Operator in internal Slack | 22 | `app_mentions:read`, `im:history`, `assistant:write` | none | `app_mention`, `message.im`, `assistant_thread_started` |
| Reply and react as teammate's own Slack user | 05 | none | `chat:write`, `reactions:write` | none |

- Install URL asks union of bot scopes of enabled features. Defaults: all of above (one consent, no re-approval when admin turns on notifications later).
- User scopes asked only in member grant flow (`POST /integrations/{itg}/member-grants`), never at admin install.
- `incoming-webhook` scope dropped: notifications post with bot token to channel picked in 06 settings. Pasted incoming webhook URL stays for teams who never install app (06).
- Events land on one endpoint `/webhooks/slack/events` (05 owns controller, signature, storage). Router dispatches by event kind: messages, reactions, interactions to 05; `app_mention` in `internal` rooms and `message.im` from linked members with Operator enabled to 22; `app_uninstalled`, `tokens_revoked` to 23 `ApplyProviderRevocation`.
- Manifest (custom self-host app) generated by 05 `channels:manifest` from same scope registry: one source of truth.

### Slack Marketplace (cloud)

Shared cloud app is commercially distributed: until Marketplace approval, `conversations.history` and `conversations.replies` limited to 1 request/min, 15 objects (Slack, 2025-05-29). Thread replies whose root predates bot joining cannot be fetched fast (Plain lost these after 2025-06-30). Plan:
- Submit for Marketplace review at first staging deploy with working install, not at launch. Review takes weeks.
- Listing needs: privacy policy, support URL, data retention statement, statement that Slack data never trains models (dimmi never trains; BYOK providers bound by their API terms), uninstall removes tokens at once.
- Retention: dimmi keeps only messages that became conversation messages and room metadata. Raw inbound events pruned per 05 (7 days). Profile cache refreshed, not archived.
- Self-host custom app is internal app: exempt from throttle. Market it.

## Code map

- `framework/oauthclient/{oauthclient.go,manager.go,errors.go,fake.go,drivers/generic,drivers/slack,internal/flow,drivertest}`.
- `app/integrations/oauth/{registry.go (scope registry per provider + feature), providers.go (provider definitions: name, icon, driver, health check), slack.go (auth.test health, metadata mapping)}`.
- Actions `app/actions/integrations/`: `start_installation.go`, `complete_installation.go` (upsert by unique key, reconnect reuses disconnected row of same workspace, rejects Slack enterprise install `enterprise_install_unsupported`, dispatches `integration.connected`), `create_manual_installation.go` (05 custom app path), `start_member_grant.go`, `complete_member_grant.go`, `revoke_member_grant.go`, `disconnect_installation.go` (provider revoke best effort, soft delete, cascades), `apply_provider_revocation.go`, `refresh_installation_token.go`, `check_installation_health.go`, `token_for.go` (`TokenFor(ctx, installation)` and `UserTokenFor(ctx, installation, member)`: only way consumers get tokens).
- Controllers `app/http/controllers/api/v1/`: `integration_controller.go` (Index, Show, Destroy), `integration_provider_controller.go` (Index), `integration_installation_controller.go` (Store), `integration_member_grant_controller.go` (Store, Destroy). Web: `integration_oauth_callback_controller.go` (Invoke).
- Jobs: `refresh_integration_tokens.go` (providers with expiring tokens), `check_integration_health.go` (daily, `auth.test` for Slack), `revoke_integration_at_provider.go`.
- Listeners: `revoke_member_grants_on_member_deactivated.go` (00-accounts `member.deactivated`).
- Events: `integration.connected`, `integration.errored`, `integration.disconnected`, `integration.scopes_changed` (catalog + 10 webhook catalog; payload never holds tokens).

## HTTP API

| Method | Path | Ability | Notes |
|---|---|---|---|
| GET | `/integration-providers` | `settings:read` | available providers, configured on this instance, features and scopes each asks |
| GET | `/integrations` | `settings:read` | installations; `filter[provider]` |
| GET | `/integrations/{itg}` | `settings:read` | status, scopes, missing scopes per feature, consumers (channel connection, destinations), member grant count |
| DELETE | `/integrations/{itg}` | `integrations:manage` | disconnect: consumers go `disconnected`, member grants revoked; 202 |
| POST | `/integrations/{provider}/installations` | `integrations:manage`, browser session (`S`) | body `{features?: ["channels","notifications","operator"], return_to}`; returns `{authorize_url}`; existing installation missing scopes: same call upgrades |
| GET | `/integrations/{provider}/callback` (web) | session | completes; redirects `return_to?connected=itg_...` or `?error=<code>` |
| POST | `/integrations/{itg}/member-grants` | any member, browser session (`S`) | returns `{authorize_url}` for current member only |
| DELETE | `/integrations/{itg}/member-grants/{itu}` | self, or `integrations:manage` | revoke at provider best effort |

05 `POST /channels/slack/installations` stays and delegates to 23 with `features = channels` default set; on `integration.connected` with feature `channels`, 05 creates channel connection and enqueues room sync.

Permissions: register `integrations:manage` (group `settings:write`) in 00-accounts catalog; owner, admin. Connect, disconnect, upgrade audit logged (scopes recorded, never tokens). Cloud: connect requires verified email (same as 05).

## Config

| Key | Default | Notes |
|---|---|---|
| `services.slack.client_id` / `client_secret` / `signing_secret` / `app_id` | empty | ownership moves from 05 to 23, env names unchanged (`DIMMI_SLACK_*`); empty = shared Slack app unavailable, custom app only |
| `oauth_client.state_ttl` | 10m | |
| `integrations.refresh_skew` | 5m | |
| `integrations.health_interval` | 24h | |
| `integrations.enabled` | `slack` | providers offered on this instance |

## Edge cases

1. Admin installs Slack from notifications settings, later enables channels. Same installation; scopes already granted (default union); no second consent.
2. Older installation missing scope a newly enabled feature needs. Feature card "Update Slack permissions"; upgrade re-runs consent; `integration.scopes_changed`; until then feature `blocked_missing_scope` with list.
3. Same Slack team installed by second dimmi workspace on shared app. 409 `integration_account_taken` with name of workspace owner can contact (cloud shows workspace name only to members of it: here "another dimmi workspace").
4. Admin cancels at Slack. Redirect `?error=denied`, no row.
5. Callback state from other browser, expired or reused. `ErrInvalidState`, 400 page "Start again", no token exchange.
6. Slack returns enterprise (Grid org) install. Reject `enterprise_install_unsupported`, revoke returned token, explain per-workspace install.
7. App uninstalled in Slack. `app_uninstalled` → installation `disconnected`, 05 connection `disconnected` reason `uninstalled`, 06 destinations `error`, member grants revoked, admins notified (06 `system_alert`).
8. One teammate revokes own user token in Slack (`tokens_revoked` with user ids). That member grant `revoked`; replies fall back to bot with name + avatar; member sees "Reconnect your Slack" once.
9. Member deactivated in dimmi. Member grants revoked at provider (best effort) and deleted; links unlinked.
10. Two replicas refresh same expiring token. Valkey lock; loser waits up to 5 s, re-reads row; refresh called once.
11. Refresh returns `invalid_grant`. Installation `error` `refresh_invalid_grant`, consumers get `ErrIntegrationBroken` (05 maps to `ErrConnectionBroken`), one alert.
12. Disconnect while jobs in flight. `TokenFor` returns `ErrIntegrationDisconnected`; 03 send jobs fail permanent `connection_disconnected`, no retry storm.
13. Workspace deleted. Installations revoked at provider in eraser hook before rows cascade.
14. Custom self-host app (manual) pasted with wrong team token later. 05 credentials rotate checks `auth.test` team equals `external_account_id`; else 422 `integration_account_mismatch`.
15. Token appears in error from provider or panic message. Redaction patterns strip before log (test per prefix).
16. Member grant started by member A, callback completed in member B's browser session. State bound to A's session: refused.
17. Slack user in member grant differs from Slack user already linked to that member (05 `channel_member_links`). New grant wins, old link replaced, audit entry.

## Tasks

- [ ] **`framework/oauthclient`.** Move socialite flow internals, manager, `generic` + `slack` drivers, fake, `drivertest` (local test provider server). 90% coverage.
- [ ] **Tables, queries, factories, RLS.** `integration_installations`, `integration_member_grants`, 05 `channel_connections.integration_installation_id`; prefix registry rows.
- [ ] **Scope registry + install/upgrade/callback flow.** Actions, controllers, audit, events. Tests: edge cases 1-6, 16.
- [ ] **Token access + refresh + health.** `TokenFor`, `UserTokenFor`, single-flight refresh, daily health. Tests: 10-12, 15.
- [ ] **Revocation + cascade.** `ApplyProviderRevocation`, member deactivation, workspace eraser. Tests: 7-9, 13.
- [ ] **Member grants.** Flow, 05 member link write, revoke. Tests: 8, 16, 17.
- [ ] **Settings UI.** `settings/integrations` page (BoardUI): provider cards, connect, features with scope state, consumers, member "Connect my Slack". e2e test light + dark.
- [ ] **Marketplace submission checklist** in `docs/` (listing copy, retention, privacy), done at first staging deploy.

Done when: one Slack install drives 05 channels, 06 notifications and 22 Operator; uninstall in Slack flips all three in under 5 s; no token in any log, resource or event (test).
