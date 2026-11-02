# 04 Email channel: outbound, domains, reputation

Style: caveman. Owner: 04-email-channel. Read with `README.md` and `pipeline.md`.

## 1. Send flow

```
03 ReplyToConversation / 07 commit / 03 CreateConversation (email)
  -> message.created (type message, author member|agent|system, conversation channel email)
  -> 03 SendChannelOutbound listener -> InsertTx send_channel_message{message_id} (queue mail, unique by message_id)
  -> 03 send_channel_message worker: guards (03 channel-contract section 2), then
     email Channel.Send(ctx, conn, contracts.OutboundMessage) -> actions/email.SendEmailMessage
       1. load address; skip transport if outbound_emails row already sent for IdempotencyKey (return stored SendResult)
       2. resolve sender (section 2), recipients (section 3), check suppression + limits (sections 8, 12)
       3. compose MIME (section 4, 5)
       4. transport send (section 6): customer-stream Mailer (section 15).Send(ctx, mail.Message{Raw, Tags, ConfigurationSet, TenantName}) -> mail.SendResult{ProviderMessageID, MessageID}
       5. 04 tx: outbound_emails row sent, email_message_refs (outbound, outbound_provider), email_addresses.last_outbound_at
       6. return SendResult{ChannelMessageID: our Message-ID, Status: sent, ExternalThreadKey, SentAt, Metadata}
  -> 03 UpdateMessageDelivery: messages.delivery_status=sent, channel_message_id; event message.delivery_updated
```

- Imported messages (16, `metadata.imported`): never reach Send (03 `SendChannelOutbound` skips; 16 dispatches no `message.created`). Defense: `SendEmailMessage` returns `SendError{Code: "imported_message", Err: contracts.ErrPermanent}` without transport call.
- Rehomed imported conversation (16; 03 `ChannelRehomer` moved it from `import` connection to inbox email connection in reply tx): sender = inbox's active email address (section 2); first outbound has no `In-Reply-To`/`References` (old thread lives at old provider; imported Message-IDs never used as refs), subject `Re: <conversation subject>`; reply token address set as usual so customer reply threads back. Later outbound normal threading on our own Message-IDs.
- `messages.delivery_status = pending` set by 03 at create (email `Transport = push`), pending until transport accepts. 04 never writes `messages.delivery_status` directly: all changes via 03 `UpdateMessageDelivery`.
- Widget conversations: never through this flow (widget is own channel); offline relay is section 9.
- Conversation contact without email (placeholder lead from forward with missing address): 03 reply request rule (supplied by 04) returns 422 `recipient_missing`. If message reaches Send anyway (contact email removed meanwhile): Send returns `SendError{Code: "recipient_missing", Message: "Contact has no email address", Err: contracts.ErrRecipientUnreachable}`; 03 marks failed + emits event.

## 2. Sender resolution

Address used: `email.from_address_id` option, else conversation's channel connection address, else workspace default address.

| Address state | From header | Envelope (Return-Path) | Signed by |
|---|---|---|---|
| Cloud or self-host `ses`, address domain verified | `"<name>" <support@acme.com>` | `bounce.acme.com` (custom MAIL FROM success) else SES default `amazonses.com` subdomain | SES Easy DKIM `d=acme.com` |
| Cloud, domain not verified | `"<name> (Acme)" <acme@inbound.trydimmi.com>` (forwarding address) | SES default | SES DKIM `d=inbound.trydimmi.com` (operator-verified) |
| Self-host `smtp` relay, domain verified | `"<name>" <support@acme.com>` | relay's bounce domain or `bounces+<token>@<inbound_domain>` when relay allows | Dimmi DKIM `d=acme.com s=dimmi202611` + relay's own |
| Self-host `postmark` / `resend` / `mailgun` / `sendgrid`, domain verified (ownership TXT + `provider_verified_domains`) | `"<name>" <support@acme.com>` | provider return-path set up at provider (Postmark `pm-bounces`, Resend `send.`, Mailgun `email.`/`mg.`, SendGrid `em<id>.`) | provider DKIM `d=acme.com` (set up by operator at provider) |
| Self-host, any driver, domain not verified | `mail.from` config address (or `default_sender_domain` forwarding address when set up at provider) with display name; Reply-To token address | provider default | provider only |
| Self-host with no `mail.from`, no verified address, or customer-stream mailer missing | 422 `no_sender_available` at reply time; Send returns `contracts.ErrPermanent` (`no_sender_available`) | | |

No row sends through workspace-owned credentials: provider is instance config (README "Never").

Display name (`from_name_style`):
- `member_and_name`: human reply `"Sam from Acme Support"`; AI reply `"<agent name> from Acme Support"` (agent name from 07 `ai_agents`; empty: address display name only); system `"Acme Support"`.
- `name_only`: always address display name.
- Display name sanitized: strip CR/LF, quotes escaped, max 78 chars, RFC 2047 encoded when non-ASCII. Never contain `@` (anti-spoof look-alike) except as part of address.

## 3. Recipients

- Default To: conversation contact email (Reply-To if section 7.2 of pipeline allows). Default Cc: conversation participants role `cc` whose email not suppressed and not the workspace's own addresses. Bcc: none.
- Request `email.to/cc/bcc` overrides (03 reply request). Validation: syntax (RFC 5322 addr-spec, EAI allowed), max 50 total, no workspace own addresses/inbound domain (loop), no duplicates (dedupe case-insensitive), not suppressed scope all (422 `recipient_suppressed`).
- Inbound was Bcc to us (`metadata.email.bcc`): default To sender only, Cc empty.
- Added Cc in reply: upsert participant contacts role `cc` (visible thread state). Removed Cc: participant row kept (history), address added to `conversations.source.email_cc_excluded` (jsonb array merge) and excluded from future defaults until participant writes again.
- Bcc never stored as participant; stored only in `outbound_emails.bcc_addresses` and message metadata (members only).
- Suppressed Cc (not To): silently dropped from that send, recipient status `suppressed`, system event line; send continues.

## 4. Headers

| Header | Value |
|---|---|
| From | section 2 |
| Reply-To | `reply_to_mode = token`: `"<display name>" <local+token@domain>` where local/domain = forwarding address; `none`: omitted |
| To, Cc | section 3 (Bcc only in envelope) |
| Subject | first reply: `Re: <conversation subject>` (no double `Re:`); new outbound conversation: given subject; widget offline: `Re: <conversation subject or "Your conversation with Acme">`; `subject_tag` on: append ` [#1042]`; never changed mid-thread |
| Date | RFC 5322 now, UTC offset +0000 |
| Message-ID | `<msg_<base32 id>.<8 random b32>@<From domain>>` (From domain = sending domain; default sender domain when unverified) |
| In-Reply-To | Message-ID of latest inbound message in conversation (else latest outbound) |
| References | root id + up to `references_max` (20) most recent ids of thread (keep first + last 19), each folded on own line |
| MIME-Version, Content-Type | multipart/alternative (text/plain + text/html); multipart/related when inline images; multipart/mixed when attachments |
| Auto-Submitted | `auto-replied` on AI replies and widget offline relay (automated delivery of response to visitor); `auto-generated` on `author_type system` messages (15 CSAT request) and contact mail with `Automated` (section 16); human email replies: absent |
| X-Auto-Response-Suppress | `OOF, AutoReply` on AI and automated mail only (Microsoft clients skip OOO to bots) |
| X-Dimmi-Loop | `v1.<base64url(hmac-sha256(app_key derived loop key, workspace_id + message id))[:22]>`: lets any Dimmi instance with same key spot own echo; other instances ignore |
| List-Unsubscribe, List-Unsubscribe-Post | widget offline relay and contact mail with `Unsubscribe` set (section 10) |
| List-Id | contact mail with `ListID` (13 changelog) |
| Precedence | `bulk` only on contact mail kind `changelog` (bulk list; Gmail bulk-sender rules) |
| Feedback-ID | `<workspace short>:<kind>:dimmi` (Gmail FBL grouping, cloud SES) |
| X-Entity-Ref-ID | random per message (stop Gmail collapsing identical-looking automated mails) |

Never set: `Precedence` (except bulk row above), tracking headers, `X-Mailer` version (fingerprinting), `Disposition-Notification-To` (read receipts).

## 5. Body templates

Plain look: no logo, no colored header, system font, 15 px, max width 640 px, single column, left aligned. Goal: looks like person wrote it, lands in Primary tab.

Teammate reply (`conversation_reply.templ` + text twin):

```
<reply body Markdown -> HTML (goldmark safe, links target _blank rel noopener)>

<signature (member signature for this address, else member global signature, else none)>

<attachments over size cap: "Attachments: file.pdf (24 MB) <signed link, expires 7 days>">

---- (thin rule, light grey, only when quote_previous)
On Mon, 2 Nov 2026 at 10:04, Bob <bob@customer.io> wrote:
> last inbound message visible part (max 2,000 chars, Markdown text), blockquote in HTML
```

- Text part: Markdown source lightly normalized (links `text (url)`), wrapped 76 chars soft (format=flowed not used), quote prefixed `> `.
- AI reply: same template; citations footer rendered by channel from `metadata.ai.citations` ("Sources: 1. Title url"); 07 human footer already inside body.
- Saved reply variables resolved by 03 before message created.
- System message (`author_type system`, 15 CSAT request via 03 `SendSystemMessage`): same thread headers (In-Reply-To, References, Reply-To token) and From as conversation reply address (display name `"Acme Support"`); body = message Markdown (question + linked score labels) rendered plain; no signature, no quote block, no citations; text twin lists every URL in full; no tracking pixel, no link rewriting. Kind `system`. Suppression scope `all` only (customer started thread). Customer text reply threads normally (pipeline.md 8).
- Locale of quote header line ("On ... wrote:") = workspace locale; dates in workspace timezone.
- Inline images in body (pasted images in editor are attachments with `inline=true`): sent as `multipart/related` parts with `Content-ID: <att_<id>@dimmi>`, HTML `src="cid:..."`.
- Attachments: total size (base64-inflated estimate x1.37) over `outbound.max_attachment_bytes` (20 MB): largest attachments converted to signed links (7 days, `signed` middleware route `attachments.download`) until under cap. Executable types never attached, always links.
- Body over 64 KB impossible (03 limit). HTML size cap 1 MB after render.

## 6. Transports

One customer-stream mailer per instance (section 15). `transport` = its driver. Never direct-to-MX; never per-workspace credentials.

| Transport | Used when | Send call | Accept = |
|---|---|---|---|
| ses | cloud; self-host `ses` (own AWS) | `Mailer.Send(mail.Message{Raw, ConfigurationSet: ses.configuration_set, Tags: stream, workspace, kind, message, TenantName})`; driver SESv2 `SendEmail` `Content.Raw`, `ConfigurationSetName`, `EmailTags`, `TenantName` | `ProviderMessageID` = SES `MessageId`; `MessageID` = `<MessageId>@<region>.amazonses.com` (SES rewrites) |
| smtp | self-host relay/submission (Postmark SMTP, Mailgun SMTP, Brevo, company relay); never port 25 MX of recipient | framework `mail` `smtp` driver (go-mail) to `mail.smtp.host` (TLS/STARTTLS required unless `mail.smtp.allow_insecure`), DKIM signed by `framework/mail/dkim` when domain verified with own key | 250 after DATA; `ProviderMessageID` = queue id parsed from 250 text when present |
| postmark | self-host | driver decomposes `Raw` into Postmark `/email` JSON, `MessageStream` = `postmark.message_stream`, `Metadata` = tags | `ProviderMessageID` = `MessageID`; Message-ID header kept or rewritten [verify], driver reports in `MessageID` |
| resend | self-host | driver decomposes `Raw` into `/emails` JSON, `tags`, custom `headers` | `id`; Message-ID handling [verify] |
| mailgun | self-host | driver posts `Raw` to `/v3/<domain>/messages.mime`, `o:tag`, `v:` vars | `id` (`<...@domain>`, also Message-ID) |
| sendgrid | self-host | driver decomposes `Raw` into `/v3/mail/send` JSON, `custom_args`, `categories` | `X-Message-Id` response header; SendGrid sets own Message-ID [verify], reported in `MessageID` |
| log | dev/test | framework `mail` `log` driver writes `.eml` to `storage/logs/mail/` | always; boot refuses `log` for customer stream in production |

Errors (Mailer returns foundation `mail` errors; Send maps to 03 channel errors; 03 result table decides message state):

| Mailer error | Examples | Send returns | Behavior (03) |
|---|---|---|---|
| `mail.ErrPermanent` | SMTP 5xx at RCPT/DATA, SES `MessageRejected`, `MailFromDomainNotVerified`, provider 422 invalid recipient / inactive recipient (Postmark 406) | `SendError{Code: smtp_5xx / ses_<reason> / <provider>_<code>, Err: ErrRecipientUnreachable}` for recipient faults, else `ErrPermanent` | failed now, delivery_error, event; no retry |
| `mail.ErrTemporary` | SMTP 4xx, connection reset, timeout, SES 5xx, provider 5xx | `SendError{Err: nil}` (transient) | 03 River retry within email retry window (about 24 h, declared by driver per `contracts.md`; backoff capped 1 h); `delivery_status` stays pending with "retrying" error; after window: failed, final |
| `*mail.RateLimitedError{RetryAfter}` | SES `Throttling`, `TooManyRequests`, provider 429, SMTP 421/451 rate | `*contracts.RateLimitedError{RetryAfter}` (jitter 1 to 60 s when provider gives none) | snooze, no attempt used |
| provider auth | SMTP 535, API 401/403 bad key | `SendError{Err: nil}` (`provider_auth_failed`) | transient (instance fix, not workspace); health check `mail.customer_stream` red, instance admin alert once per hour; after 24 h failed final (edge 108) |
| sender rejected | Postmark `Sender signature not defined`, Resend domain not verified, SES `Email address is not verified`, Mailgun/SendGrid 403 sender | domain `degraded` (`provider_rejected_sender`), `SendError{Err: nil}` | retry resolves default sender (edge 170); default sender rejected: `ErrPermanent` `sender_not_verified` |
| own checks (no transport call) | sending paused / suppressed / daily limit | paused: `ErrPermanent` `email_sending_paused`; suppressed To: `ErrRecipientUnreachable` `recipient_suppressed`; daily limit: `*contracts.RateLimitedError` code `entitlement_limit_reached` (key `email.verified_daily`/`email.unverified_daily`) (RetryAfter min(15 min, until next UTC day 00:05), repeats; status pending, UI "queued until tomorrow") | per 03 table |

At-least-once guard: before transport call, `outbound_emails` row inserted status queued with Message-ID (unique per message); retry after crash between accept and commit could double-send: SES and SMTP have no idempotency key (Postmark/Resend/Mailgun/SendGrid idempotency headers used where offered [verify]), so Send checks `outbound_emails.status` for same `IdempotencyKey` first; window accepted (rare duplicate mail with same Message-ID; recipients' clients dedupe by Message-ID).

## 7. Delivery events (SES)

Configuration set event destination SNS (signature via `framework/aws/sns`) -> `/webhooks/ses/events` -> `ProcessSesNotification`. Lookup `outbound_emails` by `(ses, mail.messageId)` (= `mail.SendResult.ProviderMessageID`). Rows without `message_id` (contact mail, section 16) follow 7.2. Message status changes below go through 03 `UpdateMessageDelivery{MessageID, Status, ErrorCode, ErrorMessage, OccurredAt}` (monotonic, emits `message.delivery_updated`); 04 updates only own rows.

| SES event | Mapping |
|---|---|
| Send | ignored (already sent) |
| Delivery | recipient status delivered; primary To delivered -> `UpdateMessageDelivery` delivered |
| DeliveryDelay | recipient status delayed (UI "delayed: mailbox full"); no status change |
| Bounce Permanent (General, NoEmail) | hard bounce: failed, suppression hard_bounce all, stats, event, system timeline line |
| Bounce Permanent Suppressed / OnAccountSuppressionList / OnTenantSuppressionList | failed, delivery_error "Recipient on suppression list", suppression hard_bounce (source ses) if not present; not counted in reputation rate (SES excludes) |
| Bounce Transient (General) | if `bouncedRecipients[].action = failed` final: failed soft, suppression soft_bounce 7 days after 3 transient fails in 7 days; Transient General from autoresponder: SES doc says counts as bounce notification for OOO; status stays delivered/sent, logged only |
| Bounce Transient MailboxFull, MessageTooLarge, ContentRejected, AttachmentRejected | failed with specific human error ("Recipient mailbox full", "Message too large"); no automatic resend; UI offers resend (03 manual retry re-enqueues `send_channel_message`; MessageTooLarge resend forces attachments to links) |
| Bounce Undetermined | failed only if no Delivery event for same recipient; delivery_error "Undetermined bounce" |
| Complaint (abuse, fraud, virus, other) | suppression complaint all, status complained (delivery_status unchanged: message was delivered), stats, reputation eval |
| Complaint not-spam | ignore |
| Reject (virus) | failed |
| RenderingFailure | n/a (raw send) |

- Out-of-order (Bounce before Delivery, Delivery after Bounce for same recipient): failed is terminal for recipient; later Delivery ignored. Delivered then Bounce (async bounce): failed wins (SES docs: receiving server may accept then bounce).
- Multiple recipients per notification: iterate `bouncedRecipients` / `complainedRecipients`.
- Unknown messageId (sent by other system using same SES account, or row purged): ignore, metric.
- D13: failed primary To: `UpdateMessageDelivery` failed (`error_code` `hard_bounce`) emits `message.delivery_updated` with `final = true`; 12 credits if no other delivered reply in billing window.

### 7.1 Provider events (postmark, resend, mailgun, sendgrid)

Operator registers one webhook per provider (instance config; `dimmi email:doctor` checks it). Routes README "Webhooks"; signature verified before parse; `ProcessMailProviderEvent` normalizes to one `DeliveryEvent{Transport, ProviderMessageID, Recipient, Type, Subtype, Diagnostic, OccurredAt, EventID}` (`providerevents/*.go`), then same actions as SES (lookup `outbound_emails` by `(transport, provider_message_id)`).

| Normalized | Postmark | Resend | Mailgun | SendGrid |
|---|---|---|---|---|
| delivered | `Delivery` | `email.delivered` | `delivered` | `delivered` |
| delayed | `Bounce` type `Transient`/`SoftBounce` | `email.delivery_delayed` | `failed` severity `temporary` | `deferred` |
| hard bounce | `Bounce` type `HardBounce`, `BadEmailAddress` | `email.bounced` (`bounce.type = Permanent`) | `failed` severity `permanent` | `bounce` (`type = bounce`) |
| blocked / suppressed at provider | `Bounce` `Blocked`, `ManuallyDeactivated` | `email.bounced` `Suppressed` | `failed` reason `suppress-bounce` | `dropped` |
| complaint | `SpamComplaint` | `email.complained` | `complained` | `spamreport` |
| ignore | `Open`, `Click`, `SubscriptionChange` (no tracking on: never enabled) | `email.sent`, `email.opened`, `email.clicked` | `accepted`, `opened`, `clicked` | `processed`, `open`, `click` |

- Exact event names and fields [verify] against provider docs at implementation; table is the mapping contract, fixtures per provider in `tests/unit/email/provider_events_test.go`.
- Same rules as SES: out-of-order terminal failed, at most one final failed per message (03 monotonic), suppression `hard_bounce` / `complaint` with source `provider`, stats + reputation counters (section 13).
- Provider-side suppression lists (Postmark inactive recipients, SendGrid suppressions, Mailgun bounces) mirrored into workspace suppression on event; lift (section 8) also calls provider reactivate API when operator key allows [verify], else UI says "also remove at provider (operator)".
- Unknown provider id (other app on same provider account, or event before commit): retry 3x 30 s, then ignore + metric (edges 172, 173).
- Open/click tracking always off at provider (driver sets `TrackOpens=false`, `TrackLinks=None` or equivalent per message).

SMTP relay transport: delivered never known; final state `sent`; failures only via DSN to VERP/return path (pipeline.md 10.1).

### 7.2 Contact mail feedback (rows without message)

Contact mail (`kind` in `changelog`, `feedback_update`, `portal_sign_in`; section 16) has no `messages` row: never calls 03 `UpdateMessageDelivery`.
- Every event (SES, provider, DSN by Message-ID): update `outbound_emails.status` + `recipient_statuses`, stats in row's substream bucket (`changelog`, `feedback`).
- Hard bounce: suppression `hard_bounce` scope `all`. Complaint: suppression `complaint` with kind scope (`changelog` -> `changelog`, `feedback_update`/`portal_sign_in` -> `feedback`). Event `email_suppression.created` carries `outbound_email_id`, `outbound_kind`; 13 and 14 listeners sync subscriber/voter state from it.

### 7.3 System stream events (routed to 06)

04 owns the provider event endpoints (one webhook per provider account, one SES events topic per config set), so it also receives system-stream events (`dimmi-system` config set, Postmark `outbound` stream, tag `stream=system`). Router rule in `ProcessSesNotification` / `ProcessMailProviderEvent`, before any `outbound_emails` lookup: event tag `stream=system` (SES `mail.tags`, Postmark `MessageStream` = system stream, Resend/Mailgun/SendGrid tag/custom arg) -> call 06 action `notifications.RecordSystemMailFeedback(ctx, DeliveryEvent)` and stop. 04 never writes `email_suppressions` or stats for system mail (06 README "System stream feedback"). SES: controller accepts TopicArn `ses.events_topic_arn` (customer) and `mail.mailers.system.ses.events_topic_arn` (system); both routed by tag.

## 8. Suppression

- Checked at send (To/Cc/Bcc) case-insensitive on full address; Gmail dot/plus normalization not applied (different addresses can be real).

| Scope | Created by | Blocks |
|---|---|---|
| `all` | hard bounce (any kind), complaint on conversation mail (reply, AI, system, widget offline, new conversation), manual, invalid | every kind |
| `notifications` | widget offline one-click / mailto unsubscribe | `widget_offline` only; direct replies still allowed (customer wrote to us) |
| `changelog` | complaint on kind `changelog` | `changelog` only |
| `feedback` | complaint on `feedback_update` or `portal_sign_in` | `feedback_update`, `portal_sign_in` |

- Opt-out from 13/14 own unsubscribe links lives in those specs' subscriber tables (not 04 scopes); 04 scopes only record provider-level signals (complaint) so other mail keeps flowing.
- Expiry: soft_bounce 7 days; others never (admin lift).
- Lift: admin DELETE; complaint lift requires owner + `confirm_complaint: true` + audit log; SES account-level suppression (cloud): cloud lift also calls SESv2 `DeleteSuppressedDestination` only for hard_bounce (not complaint) and only within workspace's tenant (if tenant-level) else ops ticket (UI explains).
- Suppression created on contact (any workspace) never shared across workspaces, except SES account-level list (cloud) which SES enforces globally: send then yields `OnAccountSuppressionList` bounce; 04 mirrors into workspace suppression with source ses.

## 9. Widget offline replies

Ownership split: 01 owns listener `ScheduleWidgetOfflineEmail`, job `SendWidgetOfflineReplyEmail` (delay `widget.offline_email_delay`, coalescing, presence check, unread message collection, `WidgetOfflineReplyMail` template incl. continue link and unverified-recipient footer). 01 job calls 04 action `email.SendWidgetOfflineReply{ConversationID, ContactID, MessageIDs, HTML, Text, Subject}`. 04 never schedules relay mail itself (no double send).

`SendWidgetOfflineReply` steps (returns `sent | skipped{reason}`; 01 treats skipped as done):
  1. `widget_offline.enabled` false -> skipped `disabled`.
  2. Contact email suppressed (all or notifications) or `contacts.blocked_at` -> skipped `suppressed`.
  3. Automated per-recipient limit; recipient email unverified and over `limits.unverified_relay_per_address_daily` (config) / entitlement `email.unverified_relay_workspace_daily` -> skipped `limited`, metric.
  4. Sender: default email address (`is_default`) -> its domain status rules (section 2); none (should not happen on cloud: initializer creates it; repaired by `dimmi workspace:repair`): skipped `no_sender`; self-host without sender: skipped with warning log once per hour.
  5. Headers: Reply-To token address of email_threads(conversation, default address) -> customer email reply threads into same widget conversation (thread step 1); `Auto-Submitted: auto-replied`; `List-Unsubscribe` (section 10). Message-ID as usual; refs stored -> In-Reply-To matching also works.
  6. outbound_emails kind widget_offline, `message_id` = first collected message; collected messages `delivery_status` unchanged (widget delivery semantics owned by 01); 01 marks `metadata.offline_emailed_at`.
  7. Idempotent per (conversation, sorted MessageIDs hash): 01 job retry after crash never sends twice once `outbound_emails` row `sent`.
- Inbound email reply on widget conversation: message `metadata.email.via = "email"`, widget shows it normally; conversation channel stays widget; further teammate replies follow same offline logic (visitor answers by email -> next teammate reply emailed after delay unless visitor online).

## 10. Unsubscribe (RFC 2369 / RFC 8058)

- On widget offline relay (automated notification copy) with 04 token (below). On contact mail when caller passes `Unsubscribe{URL, MailtoToken?}` (13 changelog, 14 `feedback_update`): caller URL used as-is, caller owns its one-click endpoint; 04 only writes headers. Never on direct email replies (customer started email thread; transactional 1:1), system messages, or transactional contact mail (sign-in, confirmation).
- Headers: `List-Unsubscribe: <https://<host>/email-unsubscriptions/<token>>, <mailto:<forwarding-local>+unsub-<token>@<inbound_domain>>` and `List-Unsubscribe-Post: List-Unsubscribe=One-Click`. Both covered by DKIM signature (SES signs all headers; smtp signer includes them; API providers sign custom headers [verify per provider], fixture checks `h=` in `.eml` from staging).
- Token = signed (HMAC) `{workspace_id, contact_id, email, scope=notifications}`, no expiry.
- `POST` (one-click, body `List-Unsubscribe=One-Click`): suppression unsubscribed scope notifications, 200 empty, no login, no CSRF. `GET`: page "You will no longer get email copies of chat replies from Acme" + confirm button (POST) + "undo" (removes suppression, same token).
- mailto unsubscribe: inbound pipeline recognizes `+unsub-<token>` before classification -> same action, ignore message.
- Caller mailto tokens (contact mail): `UnsubscribeSpec.MailtoToken` = `<prefix>-<token>` with prefix registered at boot via `email.RegisterMailtoUnsubscribe(prefix, handler)` (13 registers `cunsub` -> `UnsubscribeFromChangelog(reason mailto)`). Header mailto = `<forwarding-local>+<prefix>-<token>@<inbound_domain>` of sending workspace's default forwarding address. Inbound pipeline (pipeline.md section 3) matches `+<prefix>-` before classification, calls handler (job, idempotent), discards message (no conversation, no 03 call), metric `email_mailto_unsubscribes_total{prefix}`. Unknown prefix: treated as plain reply-token miss (routes by local part). Prefix format `[a-z]{2,10}`; `unsub` reserved for 04. Caller passing `MailtoToken` with unregistered prefix: `SendContactMail` panics in tests, omits mailto in prod (URL-only `List-Unsubscribe`, still RFC 8058 compliant).

## 11. Custom sending domain

### 11.1 Records

| Purpose | ses driver | smtp driver (relay) | manual drivers: postmark, resend, mailgun, sendgrid | Required for `verified` |
|---|---|---|---|---|
| ownership | TXT `_dimmi-verification.<domain>` = `dimmi-verification=<token>` | same | same | yes (always: stops one workspace sending as other's domain on shared instance) |
| dkim | 3 x CNAME `<token>._domainkey.<domain>` -> `<token>.<SigningHostedZone>` (from `CreateEmailIdentity`, RSA 2048 Easy DKIM) | TXT `<selector>._domainkey.<domain>` = `v=DKIM1; k=rsa; p=<base64 2048-bit public key>` (split into 255-char strings; UI shows provider-specific single value) | provider record (values come from operator's provider dashboard; Dimmi shows known selector hint only: Postmark `<yyyymmddhhmmss>pm`, Resend `resend`, Mailgun `k1`/`mx`/`smtp`, SendGrid `s1`/`s2`); status `info` (presence at hinted selector shown, never blocks) | ses/smtp: yes. manual: replaced by `provider_trusted_at` (domain in `channels.email.provider_verified_domains`) |
| return_path | MX `<mail_from_sub>.<domain>` 10 `feedback-smtp.<region>.amazonses.com` (exactly one MX on that name) | none (relay-specific; shown as guidance per relay preset: Postmark `pm-bounces` CNAME, Resend, Mailgun, SendGrid, Brevo, generic) | guidance per provider (`pm-bounces` CNAME, Resend `send` MX/TXT, Mailgun `email` CNAME, SendGrid `em<id>` CNAME) | no (recommended; full SPF alignment) |
| return_path_spf | TXT `<mail_from_sub>.<domain>` = `v=spf1 include:amazonses.com ~all` | root SPF include for relay preset (e.g. `include:spf.mtasv.net`) merged into existing SPF | provider include merged into existing SPF | no |
| dmarc | TXT `_dmarc.<domain>` = `v=DMARC1; p=none;` only when absent | same | same | no (warning when absent: Gmail/Yahoo bulk sender rules need DMARC; Outlook.com too) |

- Manual drivers: workspace admin adds domain in Dimmi (ownership TXT); operator sets domain up at provider (provider verifies DKIM + return path) and lists it in `channels.email.provider_verified_domains` (parent entry covers subdomains). Dimmi never holds per-workspace provider keys. Provider domain API automation post-MVP (README Out). Cloud uses `ses` only.

- SES identity created on `POST /email-domains` (`CreateEmailIdentity`, `DkimSigningAttributes.NextSigningKeyLength = RSA_2048_BIT`), tenant association (`CreateTenantResourceAssociation`) when tenant per workspace, `PutEmailIdentityMailFromAttributes` with `BehaviorOnMxFailure = USE_DEFAULT_VALUE` (never reject mail because MX missing).
- SES identity per region unique in account: second workspace adding same domain reuses existing SES identity (same DKIM tokens) but cannot become verified without own ownership TXT (section 11.4).
- Existing records merge: SPF: if root SPF exists, show merged value (insert include before `~all`/`-all`); never propose second SPF record. DMARC: existing kept, shown read-only with policy summary.
- Subdomain sending (`mail.acme.com`) allowed: records relative to subdomain; DMARC lookup falls back to org domain policy (shown).
- IDN domains: A-label in records, U-label display.
- `?format=bind` zone snippet; `provider_hint` from NS lookup (cloudflare, route53, godaddy, namecheap, google, vercel, netlify, porkbun, hover, other) -> UI shows provider steps (frontend), API returns `host` relative form because many providers append domain.

### 11.2 Checker (`dns/checker.go`)

- Resolvers `dns.resolvers` (cloud: 1.1.1.1 and 8.8.8.8 directly, bypass local cache; self-host: system); each lookup timeout 3 s; both resolvers must agree for `verified` status of a record (any one shows `propagating` otherwise).
- Per record status: `verified` (exact match, case-insensitive, whitespace/quote-insensitive TXT join), `mismatch` (record exists, different value; observed shown), `missing`, `propagating`, `error`.
- Mistake detectors -> `warnings[]` codes with plain-language message:
  - `doubled_domain`: record found at `<name>.<domain>` (user pasted full name into provider that appends domain).
  - `cloudflare_proxied`: DKIM CNAME resolves to Cloudflare IPs/flattened A instead of CNAME target (orange cloud): "Set proxy status to DNS only".
  - `duplicate_spf`: more than one `v=spf1` TXT at name (permerror).
  - `spf_too_many_lookups`: over 10 DNS mechanisms after expansion.
  - `spf_missing_include`: smtp driver relay include absent.
  - `dmarc_missing`, `dmarc_multiple`, `dmarc_reject_without_alignment` (p=reject and our records missing: mail would be rejected).
  - `mx_conflict_return_path`: return-path subdomain already has other MX (pick other subdomain: UI offers `dimmi-bounce`).
  - `cname_with_other_records`: CNAME at name alongside TXT (invalid DNS).
  - `wrong_selector_record_type`: TXT where CNAME expected or reverse.
  - `ownership_token_other_workspace`: token value of other workspace found (copied wrong).
- ses driver: also `GetEmailIdentity` -> `DkimAttributes.Status` (PENDING, SUCCESS, FAILED, TEMPORARY_FAILURE, NOT_STARTED), `MailFromAttributes.MailFromDomainStatus` (PENDING, SUCCESS, FAILED, TEMPORARY_FAILURE). DKIM verified requires SES `SUCCESS` (SES signs only then) AND own lookup match.

### 11.3 Status machine

```
pending --(POST checks)--> verifying
verifying --(ownership + dkim verified; manual drivers: ownership + provider_trusted_at)--> verified
verifying --(72 h after first check, required records not all verified)--> failed
failed --(POST checks)--> verifying (attempt counter reset)
verified --(daily recheck: ownership or dkim missing 2 consecutive checks 1 h apart; manual: domain removed from provider_verified_domains; any driver: provider rejected sender at send, edge 170)--> degraded
degraded --(records back)--> verified
degraded --(7 days)--> failed
any --(DELETE)--> deleted (soft)
```

- Polling (`ScheduleEmailDomainChecks` every minute picks `next_check_at <= now`, enqueues `CheckEmailDomain` unique per domain): verifying: every 1 min first 10 min after last manual check, every 5 min until 1 h, every 15 min until 24 h, hourly until 72 h. Verified: daily (spread by hash of id over 6 h window). Degraded: hourly.
- SES side 72 h window for DKIM detection: if SES DKIM `FAILED` but own lookup sees records: call `PutEmailIdentityDkimSigningAttributes` again (restart SES detection) once per day max.
- `verified` effects: addresses whose domain equals domain (or subdomain match exact) get `email_domain_id`; sending switches to own domain immediately (next send). Event `email_domain.status_changed`, notification to admins (06).
- `degraded` effects: sending falls back to default sender immediately (DKIM missing would fail DMARC and land in spam); event + notification "acme.com DNS records removed; replies now sent from acme@inbound.trydimmi.com".
- Driver change (instance switched customer-stream driver, e.g. smtp to ses, ses to postmark): all domains re-enter `verifying` with new driver's records; until verified, default sender (self-host: `mail.from`) used; `dimmi doctor` warns (edges 107, 175).

### 11.4 Ownership conflicts

- Domain verified by workspace A; workspace B adds same domain: allowed as pending; B verifies ownership TXT with B's token while A's token absent from DNS -> B becomes verified, A degraded->failed immediately with reason `ownership_transferred`, A admins notified. Both tokens present: first verified keeps it; B gets warning `domain_claimed` and stays verifying (both cannot hold verified at once; unique index).
- Cloud forbids `trydimmi.com`, inbound domain, and public mailbox domains (gmail.com, outlook.com, yahoo.com, icloud.com, proton.me, etc. list) as custom domains: 422 `invalid_domain` with hint "Forward your Gmail address to your Dimmi forwarding address instead".

### 11.5 DKIM rotation (smtp driver)

- `POST /email-domains/{id}/dkim-rotations`: new selector `dimmi<yyyymm>[a-z]`, new key, new record shown; domain stays verified with old key signing until new record verified, then new key signs; old record can be removed after 7 days (UI reminder). SES Easy DKIM rotates keys itself (no action).

## 12. Default sender

- Cloud: `<forwarding-local>@inbound.trydimmi.com`, sent via Dimmi SES on customer stream (shared IP pools). Operator maintains SPF (`v=spf1 include:amazonses.com -all`), SES Easy DKIM, custom MAIL FROM `bounce.inbound.trydimmi.com`, DMARC `p=reject` on `inbound.trydimmi.com`. Display name includes workspace name to reduce confusion: `"Sam from Acme Support"` (domain shows trydimmi). Default address created by 04 workspace initializer (never lazily) and used for widget offline mail when no other default.
- Self-host: `mail.from` (foundation config) or a verified address marked default. `dimmi doctor` checks SPF/DKIM/DMARC of `mail.from` domain.
- Reply-To token always set when sending from default sender (replies must route back even when From domain differs from customer-facing support address).

## 13. Limits and reputation (entitlement keys; self-host config off by default)

- `email_sending_profiles` per workspace: `daily_limit` = `Entitlements.Limit(ws, "email.verified_daily")` (cloud 2,000) if any verified domain else `email.unverified_daily` (cloud 200); `hourly_limit` = `email.new_workspace_hourly` (cloud 50) for first 7 days after workspace created; null after. -1 = no limit (self-host default). Recomputed on domain verify/unverify and on entitlement cache bust (override set/expire), so staff raise applies within 60 s; no `edition == cloud` branch.
- Counted: every outbound email (per recipient counts 1 per email, not per recipient; recipients cap 50 per email). Widget offline, AI and system (CSAT) count. Test mails count.
- Contact mail per kind (section 16):

| Kind | Stats bucket | Daily/hourly caps | Reputation eval | Pause that blocks it |
|---|---|---|---|---|
| `changelog` | `changelog` | no (13 own caps + `changelog.email.monthly_recipient_cap`) | yes (shared domain reputation) | workspace `status` paused/review |
| `feedback_update` | `feedback` | yes (over daily: skipped `limited`, 14 retries next day) | yes | workspace paused |
| `portal_sign_in` | `feedback` | counted, never deferred (auth-like, priority 1); per-recipient automated limit still applies | yes | workspace paused |
- Over daily: job reschedules to next UTC day 00:05 (pending, UI banner "Daily email limit reached (200). Verify your domain to raise to 2,000."); API reply still accepted (message created) but `delivery_status` pending. Over hourly: reschedule next hour.
- `EvaluateEmailReputation` every 15 min: trailing 500 sends (min 100) per workspace: hard bounce rate over 5% or complaint rate over 0.1% -> `status = paused`, `paused_reason`, event `email_sending.paused`, owners/admins notified (system stream to members, never through paused customer stream), cloud ops alerted. Paused: sends fail fast `email_sending_paused` (status failed, not queued; teammate sees banner; prevents silent backlog). Resume: cloud: ops sets `review -> active` after review (owner requests via `PATCH /email-sending-status {status: "review"}`: `paused -> review`, notifies ops); self-host: owner resumes directly with `PATCH /email-sending-status {status: "active"}`.
- SES account protection (cloud): global kill switch config `channels.email.ses.pause_all` (ops, customer stream only; system stream unaffected), SES tenant auto-pause respected (SES returns error -> treat as paused with reason `ses_tenant_paused`). Shared SES IP pools only; never dedicated IPs or warming (README Never).
- Self-host any driver: same evaluation from SES or provider events when `reputation.*` config set (default off); provider's own account-level pause (Postmark, SendGrid) surfaces as provider errors (section 6), not Dimmi state.
- D7 fair-use (pricing): enforced by 12 InTx `GuardOutboundFairUse` on 03 `conversation.created` (`initiated_by` member/agent/api): over `max(1,000, 10 x previous billable)`: 429 `outbound_fair_use_exceeded` on new conversation creation (not replies). 04 adds no own counter; daily send caps above are separate (reputation, not billing).
- Content safety on default sender (cloud): outbound from unverified domain with more than 10 links or URL shorteners (bit.ly etc.) to new recipient (never wrote in) blocked 422 `outbound_content_blocked` (phishing via our domain). Replies to existing customers unaffected.

## 14. Signatures

- Per member, optional, per address override. Markdown subset (bold, italic, links, line breaks; images not allowed MVP). Max 2,000 chars. Rendered under reply body above quote, separated by blank line; text part uses `-- ` delimiter line (RFC 3676) so receiving clients and our own stripper recognize it.
- AI replies never use member signature; 07 may add agent signature text in body.
- Member deactivated: signature kept for history, not used.

## 15. Streams (system vs customer)

Two logical mail streams on one instance provider (contracts.md Mailer "two-stream rule"). Never mixed.

| Stream | Recipients | Sent by | Mailer instance | Isolation |
|---|---|---|---|---|
| system | Dimmi users/members only, plus billing account `billing_email` (12; may be non-member finance address): auth (magic link, verification, reset, invitations, security notices), 06 member notifications, 12 billing mail, ops/system alerts | `app/mail` Mailables via `a.Mail` default | `mail.mailers.system` (foundation) | SES set `dimmi-system`, no tenant; Postmark stream `outbound`; tag `stream=system` |
| customer | contacts, visitors, subscribers, voters: replies, AI replies, system messages (15 CSAT), widget offline, 13/14 mail via section 16 | 04 actions only (`SendEmailMessage`, `SendWidgetOfflineReply`, `SendContactMail`) | `a.Mail.Mailer(channels.email.mailer)` (default name `customer`) | SES set `dimmi-customer` + `TenantName` per workspace (cloud); Postmark stream `postmark.message_stream`; tag `stream=customer` + `workspace` |

- `mail.mailers.customer` undefined: resolves to `mail.default` mailer with stream overrides (config set, message stream, tags). One set of provider credentials is normal; separate credentials allowed (operator choice), never per workspace.
- Workspace pause, tenant pause, `ses.pause_all` touch customer stream only; password reset and invitations keep flowing.
- Arch test `tests/arch/email_streams_test.go`: only `app/actions/email` resolves customer mailer; `app/mail` Mailables never take a contact/subscriber address (Mailable routing type is `models.User` / member only; one exception: 12 billing Mailables route to `models.BillingAccount` (`billing_email`), allowlisted in the arch test).

## 16. Customer-facing mail API for other specs

One generic entry for any non-conversation mail to a contact address: 13 changelog (posts + confirmations), 14 voter updates + portal sign-in. Conversation mail stays on `Channel.Send` (section 1), incl. 15-announcements-surveys CSAT request (system message in thread, section 5). Callers never import `a.Mail` for these recipients (arch test section 15).

```go
// app/actions/email/send_contact_mail.go
type SendContactMailInput struct {
    WorkspaceID    ids.UUID
    ContactID      *ids.UUID          // nil for subscriber without contact row (13)
    To             string
    Kind           enums.ContactMailKind // changelog | feedback_update | portal_sign_in (= outbound_emails.kind)
    Purpose        string             // caller sub-kind, free text <= 40 (confirmation, entry, update, shipped, sign_in, ...); metadata only
    FromAddressID  *ids.UUID          // ema_; nil = workspace default address
    FromName       string             // optional display override (14 notify.from_name)
    ReplyTo        string             // optional; default = workspace default address without thread token (replies start new conversation)
    Subject, HTML, Text string        // rendered by caller
    Unsubscribe    *UnsubscribeSpec   // {URL, Post bool, MailtoToken string ("<prefix>-<token>", prefix registered, section 10)}; nil for transactional (sign-in, confirmation)
    ListID         string             // optional List-Id (13)
    Tags           map[string]string  // max 5, stored in outbound_emails.metadata.tags (13: entry, mailing; stats only, never reply attribution) + provider tags
    IdempotencyKey string             // caller-unique (13 `changelog:{mailing}:{subscriber}`, 14 `fbn_` id)
    Automated      bool               // Auto-Submitted: auto-generated (all 13/14 kinds set true)
}
type SendContactMailResult struct {
    Status            string          // sent | skipped
    SkipReason        string          // suppressed | limited | paused | no_sender | disabled
    OutboundEmailID   ids.UUID
    ProviderMessageID string
}
func SendContactMail(ctx context.Context, a *app.App, in SendContactMailInput) (SendContactMailResult, error)
```

- Guards in order: idempotency (row with same key `sent`: return stored result, no transport call; crash between Mailer success and caller commit), suppression (scope `all` + kind scope, section 8), pause for kind (section 13), caps for kind (section 13), automated per-recipient limit when `Automated` (sign-in exempt from daily caps only), sender resolution (section 2; cloud default sender allowed, 13 enforces its 50-recipient rule via `HasVerifiedSendingDomain`), MIME (section 4 headers incl. List-*, Precedence), customer-stream Mailer (section 15), `outbound_emails` row (`kind` = `Kind`, `metadata.purpose`, `metadata.tags`, `metadata.idempotency_key`, `metadata.stream`).
- Errors returned only for transport outcomes: `mail.ErrPermanent`, `mail.ErrTemporary`, `*mail.RateLimitedError` (caller job retries; already-sent rows untouched). Own-check outcomes are results (`skipped`), never errors.
- Wrappers keep caller names, no own logic: `SendChangelogEmail` (13; maps `skipped suppressed` -> `email.ErrSuppressed`, `skipped paused` -> `email.ErrSendingPaused`, matching 13 email.md), `SendContactNotification` (14; returns result as-is). New callers use `SendContactMail` directly.
- Read helpers (`resolve_verified_sender.go`): `ResolveVerifiedSender(ctx, ws, addressID) (mail.Address, bool)` (verified domain only, never default forwarding address), `HasVerifiedSendingDomain(ctx, ws) bool` (13), `IsSuppressed(ctx, ws, email string, scopes ...string) bool` (default scope `all`; 15-announcements-surveys pre-send check passes none).
- Inbound replies to contact mail: pipeline.md 8 step "contact mail reply" (auto-replies discarded, humans start plain new conversation; no changelog entry tag).
