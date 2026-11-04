# REST rules and route map (canonical)

Style: caveman. Scope trim 2026-11-02: rows for deferred items removed (domain join, identity validations, reply drafts, survey forms, changelog domain); conversation restore later. Binding for every HTTP route under `/api/v1`, `/widget/v1`, help center host `/api` (incl. `/api/feedback/*`, 14). Wins over feature specs (conflict rule in `specs/README.md`). Goal: anyone who knows one Dimmi resource can guess every other.

## Rules

1. **Paths are nouns.** Plural kebab-case collections (`/saved-replies`), members by public id (`/saved-replies/{rpl}`). No verbs in paths, ever: no `/close`, `/approve`, `/login`, `/sync`, `/test`, `/resend`, `/accept`.
2. **Methods.** `GET` read, `POST` create in a collection, `PATCH` partial update, `DELETE` remove. `PUT` only replaces a singleton's whole binary content (`PUT /user/avatar`, `PUT /widget/v1/attachments/{att}/content`). No other `PUT`.
3. **State is a field.** Status, assignment, flags and soft-delete change through `PATCH` on the resource itself. Examples: `{"status": "closed"}`, `{"assignee_id": "mem_..."}`, `{"spam": true}`, `{"read": true}`. Side parameters ride along (`snoozed_until`, `reason`). Illegal transition: 422 `invalid_transition` with `from`, `to`. Each changed field fires its own domain event (`status: closed` fires `conversation.closed`); event catalog unchanged. Permissions checked per field (policy `UpdateField`).
4. **Things with identity, history or a job are child collections.** `POST` to a noun: `replies`, `notes`, `merges`, `exports`, `deliveries`, `syncs`, `checks`, `tests`, `rotations`, `replays`, `delivery-attempts`, `tool-syncs`, `publications`, `signing-secret-rotations` (07 actions), `runs`, `files` (16 imports), `freshness-checks` (08). Returns the created resource (201) or, for async work, a job-like resource with `status` (202).
5. **`tests` and `checks` collections run something now.** `POST .../tests` performs a synchronous test (email, webhook, notification destination) and returns the outcome resource. `POST .../checks` starts a re-verification (DNS, domains) and returns 202. Not stored unless the owner spec says so. Status codes: synchronous result object (`tests`, `validations`, `probes`, `renderings`) 201; async job 202.
6. **Singletons.** One per parent: singular noun, no id (`/user`, `/workspace`, `/workspace/entitlements`, `/user/confirmation`, `/conversations/{conv}/read-state`, `/knowledge/settings`). Only real singletons: thing a user may hold several of later is a collection (auth factors: collection `/user/auth-factors`). `GET`, `PATCH`; `POST` creates when it has a lifecycle; `DELETE` removes.
7. **Soft delete and restore.** `DELETE` trashes. Restore is `PATCH {"trashed": false}` on the same path. Trashed items readable with `filter[trashed]=with|only`. Applies to articles, workspaces (grace period) and any future soft-deleted resource. Conversations: `DELETE` trashes, purge after retention; restore later (`conversation-restore`).
8. **Bulk = `PATCH` on the collection.** Convenience fields that only make sense as changes (`add_tags`, `remove_tags`, viewer's own `unread`) are allowed on single `PATCH` too, so bulk stays a strict subset. Body `{"ids": [...], "changes": {...}}` or `{"filter": {...}, "changes": {...}}` (filter only where owner spec allows it). `changes` accepts exactly the fields of single-resource `PATCH`; bulk delete is `{"changes": {"trashed": true}}`. Max 100 ids. Response 200 `{data: [{id, status: "updated"|"skipped"|"failed", error?}], meta: {updated, skipped, failed}}`. Filter-based bulk returns `meta` only. Partial success allowed, never silent.
9. **Ordering** of children: `PATCH` on the child collection with `{"order": ["id1", "id2", ...]}`.
10. **Upsert.** `POST /collection?upsert=true`. Match keys documented per resource (contacts: `external_id` then `email`; companies: `external_id` then `domain`). 201 when created, 200 when updated, `meta.upserted: "created"|"updated"`.
11. **Computed results with no stored row** use a noun for the output: `POST /saved-replies/{rpl}/renderings`, `POST /help-centers/{hc}/previews`. Response `object` names the output.
12. **Secret-token redemption.** Links in emails carry a public id plus a secret token. Redeem with `PATCH /{collection}/{id}` and `{"token": "..."}` in the body. Tokens never go in paths or query strings (access logs).
13. **Logging in creates a session.** `POST /sessions` with a `grant` field (`password`, `two_factor`, `magic_link`, `email_code`, `sso`; grant names are a registry). `two_factor` body `{grant: "two_factor", method, code}` (`method` required: factor type `totp` or `recovery_code`; later `{method: "webauthn", credential}`). Logout is `DELETE /sessions/current`. Same collection lists and revokes sessions.
14. **Route names** follow the noun path, Laravel resource style: `conversations.merges.store`, `conversations.read-state.update`, `email-domains.checks.store`. Collection-level writes: `PATCH` collection = `{resource}.update-many` (`conversations.update-many`, `notifications.update-many`), `DELETE` collection = `{resource}.destroy-many` (`sessions.destroy-many`, `notification-preferences.destroy-many`). Route name = OpenAPI `operationId`. Controllers: one per resource or sub-resource (`ConversationMergeController.Store`), never action-named controllers (`CloseConversationController`). Actions keep business names (`CloseConversation`), so code still reads as business.
15. **MCP tools and CLI keep task names.** `close_conversation` (MCP) and `dimmi conversations close` (CLI) stay verbs for humans and agents. Each maps to the REST call (`PATCH /conversations/{conv}` with `status: closed`). REST is the contract; tools are sugar.

Rule notes added 2026-11-04: rule 8 limit is 100 ids answered inline; owner spec may run larger as job, 202 `bulk_operation` (03: ids to 1,000, filter to 10,000). Rule 11 covers read-only computed POST `/conversation-searches`, `/audience-previews`, `/audience-evaluations`, `/identity-verification/validations`. Rule 5: `POST /email-addresses/{address}/tests` stored by owner spec, 201 `waiting`, completes async. `/drafts?upsert=true` rule 10, match `conversation_id` + `mode`. Singletons: `/workspace/access-policy`, `/identity-verification/settings`. Collection delete: `DELETE /contacts/{contact}/widget-sessions`.

### Exceptions (not REST, protocol-defined)

Never under `/api/v1`. Listed here so nothing else claims an exception.

| Path | Why |
|---|---|
| `/oauth/authorize`, `/oauth/token`, `/oauth/revoke`, `/oauth/register`, `/oauth/device/code`, `/.well-known/*` | OAuth 2.1, RFC 7009, RFC 7591, RFC 8628, RFC 8414, RFC 9728 fixed shapes |
| `/mcp` | MCP JSON-RPC |
| `GET /connection/websocket`, `GET /connection/uni_sse` | centrifuge realtime transports (events-and-realtime.md). Connect/subscribe auth via app callbacks, no separate auth endpoint. `uni_sse` carries 60 s connect token in query `cf_connect` (EventSource cannot set headers; redacted in logs). Widget mints token via REST `POST /widget/v1/realtime-tokens` (noun, normal rules) |
| `/webhooks/{provider}/...` | provider-defined payloads. Includes `/webhooks/{ses,postmark,resend,mailgun,sendgrid}/events` (mail delivery/bounce/complaint events, 04, both streams), SES inbound mail (04), `/webhooks/github/repositories/{ghr}` (08 shared GitHub layer: one route per repo, handler registry for 08 knowledge, 08 docs sync, 13 changelog releases; replaces `/webhooks/github/changelog/{cls}` and `/webhooks/github/knowledge/{ks}`, which must not exist) |
| `/webhooks/{postmark,sendgrid,resend,cloudflare}/inbound`, `/webhooks/mailgun/inbound/mime`, `/webhooks/google/mailbox`, `/webhooks/microsoft/mailbox`, `/webhooks/microsoft/mailbox-lifecycle` | provider-defined inbound and mailbox push (04 sources.md, mailbox-connections.md) |
| `/auth/sso/{ssoc}/redirect`, `/auth/sso/{ssoc}/callback`, `POST /auth/saml/{ssoc}/acs`, `GET /auth/saml/{ssoc}/metadata` | SAML and OIDC fixed shapes (21); ACS exempt from CSRF middleware, guarded by state cookie, RelayState, InResponseTo |
| `/scim/v2/*` | RFC 7644 fixed shapes (21); own bearer `dmi_scim_` |
| `GET /messenger/{app}`, `/channels/email/google/callback`, `/channels/email/microsoft/callback`, `/email-setup/{address}` + `POST /email-setup/{address}/tests` (signed HMAC query, no login) | web pages (01 hosted page, 04 setup) |
| `/auth/{provider}/redirect`, `/auth/{provider}/callback`, `/device`, `/oauth/consent/{oar}`, `/oauth/callback/app` | browser redirect pages, not API |
| `/notification-unsubscriptions`, `/feedback-unsubscriptions` (14) body | RFC 8058 one-click body `List-Unsubscribe=One-Click` (path is still a noun); app host, signed HMAC query |
| Web pages in `routes/web.go` (`/setup`, `/device`, consent page, `/email-unsubscriptions/{token}`, `/channels/slack/callback`, `/channels/discord/callback`, `/integrations/slack/notifications/callback`, `/notification-unsubscriptions`, signed downloads `/files/{att}/{filename}`, help center pages, changelog pages incl. `/changelog/subscription-confirmations/{token}` and `/changelog/unsubscriptions/{token}` (13), feedback portal pages `/feedback/*`, `/roadmap` (14), CSAT rating page `/csat-ratings/{svr}` (15: signed CSAT rating page; GET renders, POST stores; HMAC-signed query + form token; hosted survey pages later: `hosted-survey-page`), `/widget-preview/{widget}` (01)) | browser pages and provider install redirects, not API. Unsubscribe and confirmation pages: GET renders only, POST acts (RFC 8058 one-click POST, link scanners never act). Paths still nouns; tokens in path allowed only where RFC 8058, double opt-in links (13) or provider redirect requires; URL signatures (HMAC query params, not stored tokens) allowed on signed routes |
| Admin port `/internal/*` (`/internal/tls/ask`), `/metrics`, `/up`, `/ready` | operator endpoints, not public API |

## Route map (old to new)

All paths relative to `/api/v1` unless shown. Owner spec applies the change in its own files; consumers update references. Old paths must not appear anywhere after this change (checked by grep).

### 00-accounts

| Old | New | Notes |
|---|---|---|
| POST /auth/register | POST /users | guest; creates user + session (+ workspace or membership) |
| POST /auth/login | POST /sessions `{grant: "password"}` | 2FA needed: 200 `{object: "two_factor_challenge", methods, expires_at}` + challenge cookie, no session. `methods` = array of objects `{method, factor_id, name, preferred, options}` |
| POST /auth/two-factor-challenge `{code \| recovery_code}` | POST /sessions `{grant: "two_factor", method, code}` | challenge cookie; `method` = `totp` \| `recovery_code` |
| POST /auth/logout | DELETE /sessions/current | |
| POST /auth/magic-link | POST /magic-links | always 202 |
| POST /auth/magic-link/consume | POST /sessions `{grant: "magic_link", token}` or `{grant: "email_code", email, code}` | may sign up when signup allowed |
| POST /auth/forgot-password | POST /password-resets | always 202 |
| POST /auth/reset-password | PATCH /password-resets/{prs} `{token, password}` | link carries `prs_` id + token |
| POST /auth/email/verification-notification | POST /email-verifications | 202 |
| POST /auth/email/verify | PATCH /email-verifications/{ott} `{token}` | no auth; `one_time_tokens` purpose `email_verification` (24 h) replaces signed payload |
| GET /auth/confirmed-password-status | GET /user/confirmation | `{confirmed_until}`, 404 when none |
| POST /auth/confirm-password | POST /user/confirmation `{method: password \| email_code \| <factor type with Confirmation capability, MVP totp>}` | stores `sessions.confirmed_method` |
| POST /auth/confirm-code | POST /user/confirmation-codes | 202 |
| POST /user/email | POST /email-changes `{email}` | creates `ott_` purpose `email_change` |
| POST /user/email/confirm | PATCH /email-changes/{ott} `{token}` | no auth required |
| POST /user/avatar | PUT /user/avatar | multipart |
| GET /user/sessions | GET /sessions | own sessions and first-party app grants |
| DELETE /user/sessions/{session} | DELETE /sessions/{ses} | `current` alias = logout |
| DELETE /user/sessions | DELETE /sessions | all except current, `password.confirm` |
| POST /user/two-factor | POST /user/auth-factors `{type: "totp", name?}` | 201 `auth_factor` status `pending` + one-time `setup` (`secret`, `otpauth_url`, `qr_svg`); `password.confirm` |
| POST /user/two-factor/confirm | PATCH /user/auth-factors/{fac} `{code}` | pending to active; one-time `recovery_codes` when this activation enabled 2FA |
| POST /user/two-factor/recovery-codes | POST /user/auth-factors `{type: "recovery_code"}` | regenerates set, 201 with one-time `codes`; `password.confirm` |
| DELETE /user/two-factor | DELETE /user/auth-factors (all) or DELETE /user/auth-factors/{fac} | `password.confirm`; removing last second factor = 2FA disabled (409 `two_factor_required_by_workspace` kept) |
| GET /user/workspaces | GET /user/memberships | membership resources with `workspace` included |
| POST /invitations/accept | POST /user/memberships `{invitation_token}` | |
| POST /workspace/leave | DELETE /user/memberships/{mem} | |
| POST /workspaces/{ws}/restore | PATCH /workspaces/{ws} `{trashed: false}` | owner, `password.confirm` |
| POST /members/{mem}/deactivate | PATCH /members/{mem} `{status: "deactivated"}` | |
| POST /members/{mem}/reactivate | PATCH /members/{mem} `{status: "active"}` | |
| POST /members/{mem}/transfer-ownership | PATCH /members/{mem} `{role: "owner"}` | owner only, `password.confirm`; caller becomes admin in same tx |
| POST /invitations/{inv}/resend | POST /invitations/{inv}/deliveries | |
| PATCH /members/me/availability | PATCH /members/me `{availability}` | rule 3 |

### 01-messenger-widget (`/widget/v1`)

| Old | New | Notes |
|---|---|---|
| POST /boot | GET /session (resume) and POST /sessions (start) | see 01 README "Boot handshake"; anonymous page load sends no new request |
| POST /session/refresh | PATCH /session `{user_jwt}` | |
| POST /conversations/{conv}/typing | POST /conversations/{conv}/typing-indicators | 202, ephemeral |
| POST /conversations/{conv}/read | PATCH /conversations/{conv}/read-state `{last_read_message_id}` | |
| POST /conversations/{conv}/handoff | POST /conversations/{conv}/handoffs | controller + action owned by 07 |
| POST /attachments/{att}/complete | PATCH /attachments/{att} `{status: "uploaded"}` | |

### 02-widget-identity

| Old | New |
|---|---|
| POST /identity-secrets/{isk}/rotate | POST /identity-secrets/{isk}/rotations |
| POST /identity-secrets/{isk}/retire | PATCH /identity-secrets/{isk} `{status: "retiring", retires_at}` |
| POST /identity-secrets/{isk}/revoke | PATCH /identity-secrets/{isk} `{status: "revoked", reason, revoke_sessions}` |

### 03-shared-inbox

| Old | New |
|---|---|
| POST /conversations/{conv}/restore | PATCH /conversations/{conv} `{trashed: false}` |
| POST /conversations/{conv}/close | PATCH /conversations/{conv} `{status: "closed"}` |
| POST /conversations/{conv}/reopen | PATCH /conversations/{conv} `{status: "open"}` |
| POST /conversations/{conv}/snooze | PATCH /conversations/{conv} `{status: "snoozed", snoozed_until \| snooze_preset}` |
| DELETE /conversations/{conv}/snooze | PATCH /conversations/{conv} `{status: "open"}` |
| POST /conversations/{conv}/assign | PATCH /conversations/{conv} `{assignee_id, team_id}` |
| POST /conversations/{conv}/spam | PATCH /conversations/{conv} `{spam: true, block_contact?}` |
| DELETE /conversations/{conv}/spam | PATCH /conversations/{conv} `{spam: false}` |
| POST /conversations/{conv}/merge (`into`) | POST /conversations/{target}/merges `{from: [conv_...]}` (target in path) |
| POST /conversations/{conv}/read | PATCH /conversations/{conv}/read-state `{last_read_message_id}` |
| DELETE /conversations/{conv}/read | PATCH /conversations/{conv}/read-state `{unread: true}` |
| POST /conversations/{conv}/typing | POST /conversations/{conv}/typing-indicators |
| POST /conversations/bulk | PATCH /conversations `{ids, changes}` |
| POST /conversations/{conv}/messages/{msg}/retry | POST /conversations/{conv}/messages/{msg}/delivery-attempts |
| POST /contacts/identify | POST /contacts?upsert=true |
| POST /companies/identify | POST /companies?upsert=true |
| POST /contacts/{ctc}/merge | POST /contacts/{ctc}/merges `{from}` |
| POST /contacts/{ctc}/block | PATCH /contacts/{ctc} `{blocked: true}` |
| DELETE /contacts/{ctc}/block | PATCH /contacts/{ctc} `{blocked: false}` |
| POST /contacts/{ctc}/export | POST /contacts/{ctc}/exports |

### 04-email-channel

| Old | New |
|---|---|
| POST /email-domains/{edm}/check | POST /email-domains/{edm}/checks |
| POST /email-domains/{edm}/dkim-rotation | POST /email-domains/{edm}/dkim-rotations |
| POST /email-addresses/{ema}/test | POST /email-addresses/{ema}/tests |
| POST /inbound-emails/{ibm}/reprocessing | POST /inbound-emails/{ibm}/processing-runs |
| POST /email-sending-status/resumption | PATCH /email-sending-status `{status: "active"}` |

### 05-chat-app-channels

| Old | New |
|---|---|
| POST /channels/{chn}/reconnect | POST /channels/{chn}/reconnections |
| POST /channels/{chn}/rooms/sync | POST /channels/{chn}/room-syncs |
| POST /channels/slack/install | POST /channels/slack/installations |
| POST /channels/discord/install | POST /channels/discord/installations |
| POST /channels/whatsapp/embedded-signup | POST /channels/whatsapp/embedded-signups |
| POST /channels/{chn}/whatsapp/checklist/refresh | GET /channels/{chn}/whatsapp-checklist?refresh=true (throttled 1/min) |
| POST /channels/{chn}/whatsapp-templates/sync | POST /channels/{chn}/whatsapp-template-syncs |
| POST /channels/{chn}/inbound-events/{cie}/replay | POST /channels/{chn}/inbound-events/{cie}/replays |

### 06-notifications

| Old | New |
|---|---|
| POST /notifications/{ntf}/read | PATCH /notifications/{ntf} `{read: true}` |
| DELETE /notifications/{ntf}/read | PATCH /notifications/{ntf} `{read: false}` |
| POST /notifications/read-all | PATCH /notifications `{filter: {conversation_id?, trigger?, before?}, changes: {read: true}}` |
| POST /notifications/seen | PATCH /notifications `{filter: {before}, changes: {seen: true}}` |
| POST /notifications/test | POST /notification-tests `{channel}` |
| POST /notification-preferences/reset | DELETE /notification-preferences |
| POST /notification-destinations/{ndst}/test | POST /notification-destinations/{ndst}/tests |
| POST /notifications/unsubscribe | GET/POST `/notification-unsubscriptions` at web root (signed, RFC 8058 body; not under `/api/v1`) |

### 07-dimmi-agent

| Old | New |
|---|---|
| POST /ai/provider-keys/{aik}/validation | POST /ai/provider-keys/{aik}/validations |
| POST /ai/provider-keys/{aik}/probe | POST /ai/provider-keys/{aik}/probes |
| POST /ai/playground | POST /ai/playground-runs |
| POST /conversations/{conv}/ai/pause | PATCH /conversations/{conv} `{ai_state: "handed_off"}` (reason `member_paused`; 03 delegates field to 07 action) |
| POST /conversations/{conv}/ai/resume | PATCH /conversations/{conv} `{ai_state: "active"}` |

### 08-help-center

| Old | New |
|---|---|
| POST /articles/{art}/publish | POST /articles/{art}/publications `{note}` (publishes working copy as new version) |
| POST /articles/{art}/unpublish | PATCH /articles/{art} `{status: "draft"}` |
| POST /articles/{art}/archive | PATCH /articles/{art} `{status: "archived"}` |
| POST /articles/{art}/unarchive | PATCH /articles/{art} `{status: "draft"}` |
| POST /articles/{art}/restore | PATCH /articles/{art} `{trashed: false}` |
| POST /articles/{art}/move | PATCH /articles/{art} `{collection_id, position}` |
| POST /articles/{art}/versions/{artv}/restore | PATCH /articles/{art} `{restore_version_id, publish?}` |
| POST /collections/{col}/articles/reorder | PATCH /collections/{col}/articles `{order}` |
| POST /help-centers/{hc}/collections/reorder | PATCH /help-centers/{hc}/collections `{order}` |
| POST /help-centers/{hc}/domains/{hcd}/verify | POST /help-centers/{hc}/domains/{hcd}/checks |
| POST /knowledge/sources/{ks}/sync | POST /knowledge/sources/{ks}/syncs |
| DELETE /knowledge/sources/{ks}/sync | DELETE /knowledge/sources/{ks}/syncs/{ksr} (cancel run) |
| POST /article-suggestions/{sug}/accept, /reject | PATCH /article-suggestions/{sug} `{status: "accepted" \| "rejected"}` |
| POST /article-suggestion-batches/{sugb}/accept, /reject | PATCH /article-suggestion-batches/{sugb} `{status: "accepted" \| "rejected", publish?}` |
| POST {help host}/api/ask | POST {help host}/api/answers (SSE) |

Exact field values (`status` enums) follow 08 data model; 08 owner adjusts names if they differ, keeping the rule.

### 09-agent-native

| Old | New |
|---|---|
| POST /oauth/authorization-requests/{oar}/approve, /deny | PATCH /oauth/authorization-requests/{oar} `{status: "approved" \| "denied", workspace_id?, scopes?}` |
| POST /oauth/device-requests/lookup | GET /oauth/device-requests?filter[user_code]=... (`user_code` redacted in logs) |
| POST /oauth/device-requests/{odr}/approve, /deny | PATCH /oauth/device-requests/{odr} `{status: "approved" \| "denied", workspace_id?, scopes?}` |

### 10-webhooks

| Old | New |
|---|---|
| POST /webhook-endpoints/{whk}/test | POST /webhook-endpoints/{whk}/tests |

### 05 extras, 12-cloud-billing

| Old | New |
|---|---|
| GET /channels/whatsapp/embedded-signup | GET /channels/whatsapp/embedded-signup-config |
| GET /oauth/slack/callback, /oauth/discord/callback (web) | GET /channels/slack/callback, /channels/discord/callback (web; `/oauth/*` belongs to 09) |
| GET /billing/usage-lines/export | GET /billing/usage-lines?format=csv (or `Accept: text/csv`) |
| GET /email/unsubscribe/{token} (04 web) | GET/POST /email-unsubscriptions/{token} |
| POST /email-sending-status/resumption | PATCH /email-sending-status `{status: "active"}` (self-host) or `{status: "review"}` (cloud owner asks ops review) |

11-self-host already nouns.

## First-party apps and guards

See `api.md` "Who calls what". Summary: one API for everyone. Guards, not URL prefixes, decide access.
