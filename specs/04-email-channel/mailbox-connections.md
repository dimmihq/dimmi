# 04 Email channel: connected mailboxes

Style: caveman. Owner: 04-email-channel. Connect Google and Microsoft mailboxes by OAuth (and IMAP, [sources.md](sources.md) 3): read Inbox + Sent, send as mailbox. Tasks: [tasks.md](tasks.md) M18, M19. Track 06 email-inbound. Facts verified on Google and Microsoft docs 2026-11-04.

BYO SMTP stays out (README Never). Allowed credential is OAuth grant or IMAP read password, per connected mailbox, sealed.

## 1. Why, and the human step

Forward has limits: Sent folder blind, DKIM breaks on forward, admin blocks. Connection fixes them. Forward stays default and works with none of this.

**Google restricted-scope verification + CASA is human work.** `gmail.readonly` is restricted; storing or transmitting restricted data on servers requires security assessment, recurring every 12 months from Letter of Validation. Assessor is third party. `gmail.send` is sensitive. Code never depends on it:
- Tests use fakes at HTTP boundary, `httpclienttest.New(tc.App.HTTP)`; no Google call, no client id needed.
- Cloud option gated by `channels.email.oauth.google.enabled`; false until human step done.
- Self-host: operator's own Google Cloud client. Workspace-only: user type Internal, verification not required. External unverified: warning screen, hard cap 100 users. Trusted app by Workspace admin lifts cap and 7-day testing-token expiry. README self-host page says it.

Microsoft: no CASA. Publisher verification recommended, human, multitenant consent otherwise shows unverified; tenant admin may need to consent. Same gate `oauth.microsoft.enabled`.

## 2. Data model

`email_mailbox_connections` (migration `20261201060002 create_email_mailbox_connections_table`, RLS, prefix `emc_`):

| Column | Type | Notes |
|---|---|---|
| email_address_id | uuid not null unique references email_addresses on delete cascade | one per address |
| provider | text not null, registry enum | `google`, `microsoft`, `imap` |
| account_email | citext not null | signed-in account, from provider |
| mailbox_email | citext not null | mailbox read; differs from account for M365 shared |
| status | text CHECK in (`active`,`degraded`,`reauth_required`,`disconnected`) | |
| send_mode | text CHECK in (`mailbox`,`provider`) default `mailbox` | `imap` forced `provider` |
| filter_mode | text CHECK in (`all`,`address_only`) | `address_only`: only mail delivered to this address (alias on personal mailbox) |
| sent_sync | boolean default true | |
| credentials_encrypted | bytea not null | `encryption.Sealed`, scope workspace, AAD table + column + id. google, microsoft: `{refresh_token, access_token, expiry, scopes}`; imap: `{host, port, username, password}` |
| client_state_encrypted | bytea | microsoft `clientState` |
| send_as | jsonb not null default '[]' | google accepted aliases |
| cursor | jsonb not null default '{}' | google `{history_id}`; microsoft `{inbox_delta, sent_delta}`; imap per folder `{uidvalidity, last_uid}` |
| watch | jsonb not null default '{}' | google `{expires_at}`; microsoft `{inbox: {id, expires_at}, sent: {...}}` |
| next_sync_at, last_synced_at, last_error_at | timestamptz | `(next_sync_at) WHERE status IN ('active','degraded')` scan |
| last_error | text | plain lang key, no token, no body |
| consecutive_failures | int default 0 | |
| connected_by_member_id | uuid null references workspace_members on delete set null | |
| disconnected_at | timestamptz | row kept 30 days without credentials, for audit |

Unique `(workspace_id, mailbox_email) WHERE status <> 'disconnected'`. Same mailbox in two workspaces: allowed, each proves own grant and syncs own. Credentials never in resources, exports, logs. `data_exports` secret-exclusion list: add column (delta to 11).

`email_mailbox_messages` (migration `20261201060003 create_email_mailbox_messages_table`, RLS, no public id):

`workspace_id, connection_id, provider_message_id text, provider_thread_id text, folder text (inbox|sent), rfc_message_id text, inbound_email_id uuid null, outbound_email_id uuid null, created_at`. Primary key `(connection_id, provider_message_id)`. Index `(workspace_id, rfc_message_id)`. Keeps sync idempotent and holds Gmail `threadId`. Pruned 400 days. Eraser: rows by contact via `inbound_email_id`, `outbound_email_id` deleted in `EraseEmailContactData`.

`email_threads`: unchanged; thread id looked up through refs.

Entitlement `email.connected_mailboxes` (count limit; cloud 25, self-host -1) checked at create: 422 `entitlement_limit_reached`.

## 3. OAuth flow

Reuse check: `golang.org/x/oauth2` (BSD, in go.mod, in allowlist) with `a.HTTP` transport. `framework/socialite` is sign-in only: different scopes, no offline tokens; not reused. No Gmail or Graph SDK: `google.golang.org/api` and `msgraph-sdk-go` are huge (binary size rule); small REST calls in `framework/mailbox/drivers/{gmail,graph}`, fixtures hit `httpclienttest`.

```
POST /email-mailbox-connections {provider: "google"|"microsoft", email_address_id?, address?, mailbox_email?}
  -> 201 {object: "email_mailbox_connection_authorization", authorization_url}
  state: random, Valkey SET NX EX 600 email:oauth:{state} -> {workspace, member, address, provider, pkce verifier}
browser -> provider consent -> GET /channels/email/{google,microsoft}/callback (web, session + state check)
  exchange code, read profile, verify rules (section 4)
  store sealed, start at current cursor (no history), create subscription/watch, first sync
  redirect /d/settings/email?connected=emc_...
```

Callback routes follow `/channels/slack/callback` pattern (rest.md web list; delta adds them). `state` session mismatch: 403 page. Consent denied: back to settings with plain message.

| Provider | Scopes | Why |
|---|---|---|
| google | `openid email https://www.googleapis.com/auth/gmail.readonly https://www.googleapis.com/auth/gmail.send`; `access_type=offline`, `prompt=consent` | read + send only; no `gmail.modify`: we never mutate mailbox |
| microsoft | `offline_access openid email User.Read Mail.Read Mail.Send`; shared mailbox adds `Mail.Read.Shared Mail.Send.Shared`; authority `common` (or `channels.email.oauth.microsoft.tenant`) | no `Mail.ReadWrite`: sendMail with MIME needs no draft |

Refresh: `SyncEmailMailbox` and Send take access token via `oauth2.TokenSource`; new tokens resealed under row lock (Microsoft rotates refresh tokens, must store newest). Lock: advisory per connection during refresh.

Permissions: `email:manage`. Read `email:read` (resource: provider, emails, status, send mode, last synced, last error). Token (API) cannot start OAuth flow: `POST` requires session guard `U`. API tokens may `GET`, `PATCH`, `DELETE`.

## 4. Account rules at connect

| Case | Behavior |
|---|---|
| Account email = address | `filter_mode = all` |
| Account email differs, address in Gmail `sendAs` with `verificationStatus = accepted` | allowed, `filter_mode = address_only` (filters `Delivered-To`, To, Cc contain address) |
| Differs, not alias | 422-style page: "You signed in as sam@acme.com. Sign in as support@acme.com, or use a routing rule." |
| No address given | creates address from account email (+ channel connection, inbox default) |
| Address is Google Group | sign-in as Group impossible; message points to routing guide ([setup.md](setup.md) 2) |
| M365 shared mailbox `mailbox_email` set | sign in as member with access; poll only (section 6) |
| Personal Outlook.com | supported, same scopes, no shared mailbox |
| Personal-looking mailbox (`filter_mode = all`, Gmail consumer) | consent warning before redirect: "dimmi will read every new message in this mailbox" |

## 5. Sync

`SyncEmailMailbox(connection)` queue `channels`, unique by connection. Order: **Sent first, then Inbox**, so reply to our own just-sent mail finds refs. Each new item: `email_mailbox_messages` `ON CONFLICT DO NOTHING`; raw fetched; `ReceiveInboundEmail` with source `gmail`/`microsoft`/`imap`, envelope recipient = address's `forwarding_address` route skipped: connection routes directly to its address. Cursor saved per page after commit.

Skip: Gmail labels `SPAM`, `TRASH`, `DRAFT`, `CHATS`. Graph: folder scoped already.

Verdicts: mailbox host is receiving MTA. Top `Authentication-Results` header only (`mx.google.com` for gmail; first header for microsoft), parsed by driver. Below it sender controlled, ignored. Own DKIM verify runs too. IMAP: not trusted.

### 5.1 Gmail

| Item | Decision |
|---|---|
| Change feed | `users.history.list`, `startHistoryId`, `historyTypes=messageAdded`. Fetch `messages.get format=minimal` for labels, then `format=raw` for INBOX or SENT |
| Push vs poll | **Both: push wakes, poll is truth.** Push on when `channels.email.oauth.google.pubsub_topic` set: cloud on. Self-host default off (needs Cloud project, public URL): poll 60 s. With push: poll every 5 min; Google says notifications may delay or drop, per-user over 1/s are dropped |
| Watch | `users.watch` `{topicName, labelIds: [INBOX, SENT], labelFilterBehavior: include}`. Must renew under 7 days; `RenewEmailMailboxWatches` daily. `users.stop` at disconnect |
| Push auth | `POST /webhooks/google/mailbox`: Pub/Sub OIDC bearer verified (issuer, `aud` = config, `email` = config service account) with `framework/jwt`, JWKS through `a.HTTP`. Decoded `{emailAddress, historyId}`: look up connections, enqueue sync; never trusts `historyId` beyond wake. Unknown address: 200 (ack) |
| Stale cursor | `history.list` 404: `messages.list q="after:<last_synced - 1 day>"`, dedupe, new `historyId` from newest, metric, no user effect. History typically lasts about a week at least |
| Topic grant | operator grants `gmail-api-push@system.gserviceaccount.com` publish on topic; doctor checks config only |

### 5.2 Microsoft Graph

| Item | Decision |
|---|---|
| Change feed | Delta per folder: `/users/{mailbox}/mailFolders/inbox/messages/delta`, `.../sentitems/messages/delta`, `$select=id,internetMessageId,conversationId`; `deltaLink` in `cursor`; `Prefer: IdType="ImmutableId"` on every request. Raw: `GET /messages/{id}/$value` |
| Push vs poll | Push wakes, poll is truth. Basic notifications only (no resource data, no cert work). `changeType: created`. Poll every 5 min with subscription; 60 s when `app.url` not public or subscription failed |
| Subscription | `POST /subscriptions`, resources inbox and sent, `notificationUrl` `/webhooks/microsoft/mailbox`, `lifecycleNotificationUrl` `/webhooks/microsoft/mailbox-lifecycle`, secret `clientState` per connection. Max lifetime 10,080 minutes (under 7 days). `RenewEmailMailboxWatches` daily: `PATCH expirationDateTime` to +6 days, also refreshes reauthorization. Duplicate 409 reuse |
| Validation | `?validationToken=` on POST: answer 200 `text/plain` decoded token within 10 s, escaped. No DB touch |
| Notification | Answer 202 within 3 s (over 10% slow marks endpoint slow): check `clientState` by constant-time against sealed value cached 5 min, enqueue, return. Bad state: 202 + drop + metric (never reveal) |
| Lifecycle | `reauthorizationRequired`: `PATCH` renew (never `/reauthorize` plus `PATCH` in same 10 min). `subscriptionRemoved`: create again, then delta. `missed`: delta. Delta always recovers |
| Shared mailbox | **Poll only**, no subscription: delegated subscription coverage not stated in docs; poll always works. 60 s |

### 5.3 Dedupe with forwarding

Customer may keep forward on while connected. Second arrival of same mail is new `inbound_emails` row under other source; pipeline rule 2 (`messages.dedupe_key`, sha of Message-ID) marks `ignored duplicate`; builds on F54 fix (assume: dedupe also compares sender so reused Message-ID from another sender stays separate; same mail still same key). No loss either order. UI after connect: "You can remove forwarding now". Not enforced.

Sent copy of our own send: section 6 ref match, `ignored loop_own_outbound`, linked.

### 5.4 Sent mail written outside dimmi

Builds on F24 fix (assume: authenticated member on thread appended as member message) and 03 action delta below.

Sent item, not ours:

| Thread match (headers, Gmail thread, refs) | Result |
|---|---|
| Yes | 03 `RecordExternalOutboundMessage(conv, InboundMessage{..., AuthorMember})`: message `author_type member`, author = member whose `users.email` equals From, else connector, else `system` with mailbox name; `delivery_status = sent`; `metadata.email.external_send = {provider, mailbox}`; no `send_channel_message`; clears waiting like a reply; no notify to author; UI "Sent from Gmail" |
| No, `settings.sent_sync_new_threads` true | new conversation `initiated_by member`, first To = contact (F23 rule) |
| No, default false | `ignored sent_no_thread`: mailbox may hold personal mail |

Not counted in daily caps, not `outbound_emails` (we did not send). Billing: counts as reply per 12 reply rule; 04 keeps `substantive = true`.

## 6. Send through mailbox

`send_mode = mailbox`, `kind` in (`reply`, `ai_reply`, `system`, `new_conversation`, `test`): `SendEmailMessage` uses mailbox driver. Everything else (`widget_offline`, 13, 14 contact mail): instance provider, never customer's mailbox: their account would take bulk reputation and Google/Microsoft send caps. `send_mode = provider`: as `outbound.md` 2.

Composer same MIME (Message-ID, In-Reply-To, References, signature, `Auto-Submitted` for AI), plus HMAC header `X-Dimmi-Outbound: <stamp of obm id>` (uses `stamps.go`). No Reply-To token (`reply_to_mode` treated `none`): replies return to real mailbox and are synced. No `List-Unsubscribe`.

| | Gmail | Graph |
|---|---|---|
| Call | `POST /upload/gmail/v1/users/me/messages/send?uploadType=multipart` (metadata `{threadId?}` + `message/rfc822`) | `POST /users/{mailbox}/sendMail`, `Content-Type: text/plain`, base64 MIME; 202 no body |
| From | mailbox or accepted `sendAs` | mailbox; shared needs `Mail.Send.Shared` |
| Thread | `threadId` when a message of conversation is in `email_mailbox_messages` and subject matches (Google requires id, RFC refs, same subject); else omit | `conversationId` from MIME headers, automatic |
| Real Message-ID | response `id`; `messages.get format=metadata metadataHeaders=Message-Id` right after; differs from ours: write `outbound_provider` ref | Sent sync finds item carrying `X-Dimmi-Outbound`: `internetMessageId` becomes ref, `provider_message_id` filled. Immediate sync enqueued, retry 3x 30 s |
| Sent folder | automatic | automatic |
| `SendResult.Status` | `sent` | `sent` |
| Errors | 429/`rateLimitExceeded`, `userRateLimitExceeded`: `RateLimitedError`; 5xx temporary; 401 after refresh or `invalid_grant`: broken; 403 `failedPrecondition`/domain policy: permanent `mailbox_send_refused` | 429 + `Retry-After`; 5xx; 401/403 token; `ErrorMessageSizeExceeded`, `ErrorSendAsDenied`: permanent |

Capabilities: `DeliveryReceipts` false on mailbox connections. Bounces arrive in mailbox as DSN, go through classification + F48 recipient check. Complaints invisible; no ARF.

Limits: Dimmi daily caps and reputation pause skip mailbox sends (their account, no shared reputation); stats bucket `customer` still counts sent. Provider send caps surface as rate limit, UI "Gmail sending limit reached; will retry".

Idempotency: neither API has key. `outbound_emails` row guard as today; crash window duplicate carries same Message-ID.

Sender row: add to `outbound.md` 2 table. Fallback never silent: `reauth_required` while send pending: `ErrConnectionBroken` (real workspace connection this time), message failed + F35 retry badge; address card red; From never quietly switches. After disconnect: resolution moves to `own_domain` or `default_sender`, card says what customers will now see.

## 7. Disconnect and revocation

`DELETE /email-mailbox-connections/{connection}` (`email:manage`): stop watch (`users.stop`, `DELETE /subscriptions/{id}`, best effort), Google revoke `POST https://oauth2.googleapis.com/revoke`, Microsoft: no single-grant revoke API short of tenant-wide sessions: drop tokens, UI links `https://myaccount.microsoft.com` apps page. Overwrite `credentials_encrypted` with sealed empty, `status = disconnected`, audit. Conversations stay. Address stays. Event + notify.

`PATCH`: `send_mode`, `filter_mode`, `sent_sync`, `settings.sent_sync_new_threads`.

Revocation detection:

| Signal | Effect |
|---|---|
| Refresh `invalid_grant` (Google: revoked, password change with Gmail scopes, 6 months idle, testing app 7 days; Microsoft: `AADSTS70008`, `50173`, `700082`) | `reauth_required` |
| Scope dropped at consent (`scope` missing gmail.readonly/send) | connect refused; existing: `reauth_required` |
| Mailbox removed, license gone (404, `MailboxNotEnabledForRESTAPI`, `failedPrecondition`) | `degraded`, then `reauth_required` after 24 h |
| Admin blocks app (`admin_policy_enforced`, `AADSTS65004`) | `reauth_required` + copy "your admin blocked dimmi" + send to admin mail |

On `reauth_required`: polling stops, watch left to lapse, event `email_mailbox_connection.status_changed`, 06 notifies `email:manage` holders once per day. "Reconnect" reuses `POST` and keeps cursor when same account; else start now. Mail received during gap: Gmail history or delta recovers when cursor valid; else list since `last_synced_at`.

`RetentionGuard`, erase: `EraseEmailContactData` handles `email_mailbox_messages`. Customer mailbox never modified by erase; says so in privacy docs.

## 8. API, events, UI

API (`routes/email_mailboxes.go`): `GET /email-mailbox-connections`, `POST`, `GET /{connection}`, `PATCH`, `DELETE`; `POST /{connection}/checks` (202, sync now, rate 1/10 s). Route names `email-mailbox-connections.*`, controllers `EmailMailboxConnectionController`, `EmailMailboxConnectionCheckController`. Errors: `mailbox_address_mismatch` 422, `oauth_provider_disabled` 404, `mailbox_reauth_required` 409 on check, `imap_*` 422.

Events: `email_mailbox_connection.status_changed` (realtime `workspace:{ws}` admins; webhook yes).

Jobs: `ScheduleEmailMailboxSyncs` (1 min), `SyncEmailMailbox`, `RenewEmailMailboxWatches` (daily, 6 h spread), `PruneEmailMailboxMessages` (daily). Google/Graph per-mailbox rate limits: on 429 respect `Retry-After`, push `next_sync_at`.

MCP: `dimmi_list_email_mailbox_connections` read only.

UI:
- `resources/js/features/email/mailboxes/connect-mailbox.tsx`: in Add address flow and `AddressCard`. BoardUI `social-button` Google, Microsoft (brand marks through button's supported icons), "Other mail (IMAP)" form with `select` preset, `input` host, port, user, password. Disabled options hidden by `GET /workspace/email-capabilities` (`{oauth: {google, microsoft}, imap, inbound_sources}`, `email:read`).
- `mailbox-status.tsx`: account, last synced, Sent sync `switch`, send mode `Choice`, Reconnect, Disconnect `modal`. Red state for `reauth_required` with what customers see meanwhile.
- e2e `tests/e2e/email-mailboxes.spec.ts`: fake OAuth server route in e2e server (test-only `dev` build: serves authorize + token + Gmail REST subset, built on `httpclienttest` fixtures), connect, new mail shows conversation, reply marked sent, revoke shows Reconnect. IMAP path against in-process server helper.

## 9. Tests

Integration through real pipeline, `httpclienttest` at Google and Microsoft hosts, clock fake:
- gmail: first sync sets cursor no import; history page; history 404 full resync; spam label skipped; own sent linked; outside sent appended as member; alias `address_only` ignores other mail; watch renewed day 6 ; Pub/Sub bad JWT 401, unknown mailbox 200.
- graph: validation token echoed as text; notification under 3 s budget (no outbound call in handler); wrong `clientState` no job; lifecycle three kinds; delta resync; shared mailbox never subscribes; rotated refresh token stored.
- both: token sealed (column bytes lack refresh token); AAD moved row fails open; disconnect clears, revokes, stops; `invalid_grant` sets `reauth_required` and failed send `ErrConnectionBroken`; dedupe forward + sync either order one message; `widget_offline` never uses mailbox; daily cap skips mailbox.
- Contract: both drivers pass `framework/mailbox/drivertest` account suite.

No test contacts Google or Microsoft. Real calls only in `staging.md` harness, by human-approved run.
