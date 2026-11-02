# 04 Email channel: inbound pipeline

Style: caveman. Owner: 04-email-channel. Every inbound source ends in same pipeline. Decisions here are binding; edge cases numbered in `edge-cases.md`.

## 1. Overview

```
source driver (ses | test)
  -> mailbox.Envelope{source, provider_message_id, envelope_from, recipients, size, received_at, verdicts, raw opener}
  -> actions/email.ReceiveInboundEmail            (fast, in request)
       1. route recipients -> workspaces + email_addresses   (no match: unroutable metric, drop)
       2. per workspace: dedupe (workspace, source, provider_message_id)
       3. persist raw to disk (or reference SES S3 object), insert inbound_emails (received)
       4. InsertTx ProcessInboundEmail
  -> jobs.ProcessInboundEmail -> actions/email.ProcessInboundEmail
       a. workspace suspended -> keep received, stop (resumes on unsuspend via reprocess sweep)
       b. parse MIME (`framework/mailbox/parse`, enmime/v2) under memory budget
       c. authenticate (SPF/DKIM/DMARC or trusted verdicts)
       d. classify (loop, system notice, bounce, complaint, auto-reply, list/bulk, noreply, spam, forwarded, test, normal)
       e. per-sender rate limit
       f. resolve sender identity (form relay, member forward, member test, Reply-To rules)
       g. resolve thread (token, In-Reply-To, References, provider ids, reverse refs, subject heuristic)
       h. build body: sanitize HTML, strip quotes + signature, Markdown, text fallback, attachments, cid
       i. ingest via 03 conversations.ReceiveInboundMessage(InboundMessage) (one tx); email_message_refs + email_threads + inbound_emails linkage written by InboundMessage.AfterPersist hook in same tx
       j. inbound_emails status processed | ignored | quarantined
```

Idempotency: every step re-runnable. `messages.dedupe_key` unique stops double message. `email_message_refs` insert `ON CONFLICT DO NOTHING`.

## 2. Sources

Framework `mailbox.Source` + `mailbox.Envelope` (framework.md `mailbox`; driver `framework/mailbox/drivers/ses`; no mailbox fake: tests feed the real `ses` source test-signed SNS payloads, signing cert served via `httpclienttest`, raw message in a per-test bucket on SeaweedFS). Shape (framework-owned, shown for reference):

```go
type Source interface {
    Name() string // ses, test
}

type Envelope struct {
    Source            string
    ProviderMessageID string
    EnvelopeFrom      string    // "" for null sender
    Recipients        []string  // envelope RCPT TO (or best equivalent)
    SizeBytes         int64
    ReceivedAt        time.Time
    Verdicts          ProviderVerdicts // spf/dkim/dmarc/spam/virus when provider computed them; trusted only per source rules
    OpenRaw           func(ctx context.Context) (io.ReadCloser, error)
    RawRef            *RawRef   // set when raw already stored elsewhere (SES S3)
}
```

Contract test suite (`framework/mailbox/drivertest`): each driver given fixture payload produces identical envelope + identical raw bytes; bad signature rejected; oversize rejected; retry of same payload yields same `ProviderMessageID`.

### 2.1 SES (cloud; self-host with own AWS)

- Setup (cloud ops, documented in `docs/ops/ses-inbound.md`): MX `inbound.trydimmi.com` -> `inbound-smtp.<region>.amazonaws.com` priority 10; receipt rule set: recipient `inbound.trydimmi.com`, actions: S3 (bucket `channels.email.ses.inbound_bucket`, prefix `raw/`, SNS topic `inbound_topic_arn`), spam + virus scanning on, TLS required off (accept plain from legacy senders).
- SES S3 action limit 40 MB per message including headers. Our `max_message_bytes` 30 MB applies after.
- Webhook `POST /webhooks/ses/inbound` receives SNS `Notification` with `Message` JSON: `notificationType = Received`, `receipt.recipients` (envelope matched), `receipt.{spfVerdict,dkimVerdict,dmarcVerdict,spamVerdict,virusVerdict}.status` (PASS, FAIL, GRAY, PROCESSING_FAILED), `receipt.dmarcPolicy` (only when DMARC fails), `receipt.action.{bucketName,objectKey}`, `mail.messageId` (= S3 object key), `mail.source` (envelope MAIL FROM), `mail.commonHeaders`, `mail.headersTruncated` (headers over 10 KB).
- Controller never reads S3. Envelope `RawRef = {disk: "ses_inbound", path: objectKey}`. Job opens S3 object via aws-sdk-go-v2 (`framework/aws` config, `a.HTTP.StdClient()` transport).
- SNS signature verified by `framework/aws/sns` (`sns.Verify`). `SubscriptionConfirmation`: confirm only when TopicArn equals config; GET SubscribeURL via `a.HTTP` (host allowlist). `UnsubscribeConfirmation`: log warn, metric, alert.
- SES verdicts trusted (SES is receiving MTA, sees real connecting IP). Mapping: PASS -> pass, FAIL -> fail, GRAY -> none, PROCESSING_FAILED -> unknown. Own DKIM verify still runs when SES DKIM is GRAY (multiple signatures, alignment calc).
- Self-host with own SES: same controller; config `ses.inbound_topic_arn`; IAM role/keys from standard AWS chain.

No built-in SMTP receiver (README "Never": no port 25 listener, no mail server). IMAP and other inbound adapters (Postmark, Mailgun, Cloudflare worker) deferred (README Out).

### 2.2 Test source

`dimmi email:test-inbound` and feature tests: source `test`, verdicts from flags, never on production webhook.

## 3. Routing (recipient to workspace)

Address formats on inbound domain `D` (`channels.email.inbound_domain`):

| Format | Example | Meaning |
|---|---|---|
| `<slug>@D` | `acme@inbound.trydimmi.com` | first (default) forwarding address of workspace |
| `<slug>.<label>@D` | `acme.sales@inbound.trydimmi.com` | additional forwarding addresses; label `[a-z0-9-]{1,30}`; slugs never contain dots so no collisions |
| `<local>+<token>@D` | `acme.sales+k7f3x9q2mz8b4c1d@inbound.trydimmi.com` | reply token address for that forwarding address |
| `bounces+<obm-token>@D` | | VERP return path for `smtp` relay outbound (self-host, when relay keeps envelope sender) |
| `<local>+unsub-<token>@D`, `<local>+<prefix>-<token>@D` | `acme+cunsub-k7f3...@inbound.trydimmi.com` | mailto unsubscribe: 04 `unsub` (widget offline) or registered caller prefix (13 `cunsub`); handled before classification, message discarded (outbound.md section 10) |
| `postmaster@D`, `abuse@D` | | forwarded to `ops_forward_to` (raw resend via Mailer), never routed to workspace |

- Reserved locals (never assignable): postmaster, abuse, hostmaster, webmaster, admin, administrator, root, mailer-daemon, noreply, no-reply, bounces, bounce, support, security, dmarc, info, mail, www, dimmi. Workspace slug reserved: default local becomes `<slug>-team`.
- Forwarding address fixed at creation; slug change never changes it.
- Regenerated forwarding address (inbound domain change, edge 150): old address kept 30 days in `channel_connections.config.previous_external_ids`; router checks exact `external_id` first, then alias lookup (expression index on that jsonb array, migration 0402).
- Matching: local part lowercase, plus split at first `+`. Token mismatch to local (token belongs to other address of same workspace): still routes by token (thread wins), metric `token_local_mismatch`. Token belongs to other workspace: route by local part only, ignore token (prevents cross-tenant injection).
- Recipient discovery order for forwarded mail (envelope recipient is forwarding address; we also want original customer-facing address): `Delivered-To`, `X-Original-To`, `X-Forwarded-To`, `X-Forwarded-For`, To, Cc. Stored in `metadata.email.original_recipient`.
- One email to multiple addresses of same workspace (support@ + sales@ both forwarded): one inbound_emails row per workspace, address = first matching in priority: reply token match, To order, Cc order, envelope order. Other matched addresses recorded in `metadata.email.also_delivered_to`.
- Multiple workspaces in recipients: independent rows per workspace; envelope_to stores only that workspace's addresses.
- Unroutable: SES already accepted; drop + metric. SES raw objects (matched and unmatched) stay in inbound bucket under `raw/`; bucket lifecycle expires `raw/` after 31 days (= raw retention + 1 day). `PruneInboundEmailRaw` only marks SES rows `raw_deleted_at` after 30 days (after `RetentionGuard.MayDestroy(ws, {inbound_email, id})`; held row keeps `raw_path`, lifecycle exemption via object tag when non-noop guard driver ships, README Jobs); explicit `DeleteObject` only for GDPR erase.

## 4. Parse

- `parse.Message(raw, parse.Limits)` (`framework/mailbox/parse`, wraps `github.com/jhillyerd/enmime/v2` v2.5.x `ReadEnvelope`) on `io.LimitReader(max_message_bytes + 1)`. Over limit: re-read headers only + first `text/plain` and `text/html` parts up to 1 MB each (streaming MIME walk), `metadata.email.truncated = true`, attachments listed by name/size with `dropped_reason = too_large`.
- Memory: weighted semaphore `parse_memory_budget_mb`; weight = max(1 MB, 2 x size). Job waits (context deadline 2 min) else retry.
- enmime errors (malformed MIME) collected into `metadata.email.parse_warnings` (max 10); never fail job on warnings. Fatal parse (not MIME at all): treat whole raw as `text/plain` body, UTF-8 with replacement.
- Charsets: enmime decodes declared charset; HTML parts via `dom.DecodeCharset` (`framework/dom`); unknown/absent charset: try UTF-8 valid, else Windows-1252, else ISO-8859-1; HTML `<meta charset>` respected when part header missing. Encoded-word headers (RFC 2047) decoded, malformed encoded words left literal. RFC 2231 parameter continuation for filenames. Result NFC-normalized. NUL bytes stripped. Bidi control chars in display names stripped (spoof guard `U+202E`).
- Addresses: parse with `net/mail` lenient fallback (enmime AddressList); internationalized local parts (EAI) kept as UTF-8; domains converted to A-label for routing and identity compare, U-label for display.
- Dates: `Date` header parsed lenient; missing/unparseable or more than 7 days in future / 10 years past: use `received_at`. Stored as `messages.sent_at`.
- TNEF (`winmail.dat`): decoded via enmime-compatible TNEF reader when possible, attachments extracted; else attached as file.
- Calendar invites (`text/calendar`): kept as attachment `invite.ics`; body text extracted from other parts.
- Nested `message/rfc822`: treated as attachment `.eml` unless forwarded-detection (section 7.3) uses it.

## 5. Authentication

Output `auth = {spf, dkim, dmarc, result, aligned_domain, source}`; `result` in pass, fail, none, unknown. Primitives from `framework/mailbox/authcheck` (go-msgauth dkim/dmarc/authres); trust and alignment policy below stays 04 (`app/integrations/email/auth.go`).

Order:
1. Source verdicts trusted (SES receipt verdicts only; Authentication-Results headers never trusted at MVP).
2. DKIM: go-msgauth `dkim.Verify` on raw (max 5 signatures, 3 s DNS timeout each). Pass list of `d=` domains.
3. SPF: never computed locally (Dimmi never sees connecting IP: no receiver). From SES verdict or `unknown`.
4. DMARC: go-msgauth `dmarc.Lookup` on From domain (org domain fallback via public suffix list). Alignment relaxed unless record `adkim=s`/`aspf=s`. DMARC pass = aligned DKIM pass OR aligned SPF pass.
5. `result`:
   - `pass`: DMARC pass (aligned DKIM or SPF), regardless of published policy (no record still counts when aligned DKIM passes).
   - `fail`: DMARC record exists with p=quarantine/reject and no aligned pass; or SES dmarcVerdict FAIL.
   - `none`: no aligned pass and no enforcing DMARC record (typical small domains, forwarded SPF-only mail).
   - `unknown`: DNS temp errors, PROCESSING_FAILED.
- Forwarded mail (Gmail/Workspace/M365 forward to our address): SPF breaks, DKIM usually survives; aligned DKIM pass still yields pass. ARC not verified in MVP.
- Effects:
  - `pass`: sender email identity `verified = true` (contact_identities) when created or matched; enables 03 auto-merge with verified identities of same email.
  - not `pass`: identity unverified; never auto-merge into verified contact. No visible flag at MVP (later: unverified-sender-flag); verdict shown in "view original" (`GET /messages/{id}/email`).
  - not `pass` AND matched existing contact has verified email identity or `kind = user`: message attaches to that contact (teammate sees history) BUT `metadata.ai_skip = "unverified_sender"`, outbound replies use contact's verified address, Reply-To ignored.
  - `fail` with `dmarcPolicy = reject` from SES (SES still delivered): classify spam (quarantine) unless sender is existing participant of matched thread (then keep).

## 6. Classification

First match wins (order matters). Each yields `classification`, `substantive`, and action.

| # | Class | Detection | Action |
|---|---|---|---|
| 1 | loop | `X-Dimmi-Loop` header with valid HMAC for this instance; or From address equals one of workspace's own addresses / forwarding addresses / default sender and Message-ID matches our outbound; or Message-ID in `email_message_refs` kind outbound | ignore `loop_own_outbound`; never message |
| 2 | duplicate | `messages.dedupe_key` exists | ignore `duplicate` |
| 3 | system_notice | Gmail forwarding confirmation: From `forwarding-noreply@google.com` AND subject matches `^\(#\d+\) .*` (localized subjects: code pattern only); Dimmi test mail with `X-Dimmi-Test` token | store confirmation (section 11) / mark address `inbound_verified_at`; ignore |
| 4 | complaint | `Content-Type: multipart/report; report-type=feedback-report` (ARF RFC 5965) | section 10.2; no message |
| 5 | bounce | `multipart/report; report-type=delivery-status` (RFC 3464); or envelope from `<>` / MAILER-DAEMON / postmaster with subject patterns (Undeliverable, Delivery Status Notification, Mail delivery failed, Returned mail, Unzustellbar, Non remis) and attached original headers | section 10.1; event message on original conversation if matched; else ignore |
| 6 | auto_reply | `Auto-Submitted` present and not `no` (RFC 3834); `X-Autoreply`, `X-Autorespond`, `X-Autogenerated: Reply`; `Precedence: auto_reply`; `X-Auto-Response-Suppress` + `X-MS-Exchange-Inbox-Rules-Loop`; `X-Precedence: auto_reply`; `X-POST-MessageClass: 9; Autoresponder`; subject patterns from `autoreply_subjects.yaml` (Out of Office, Automatic reply, Auto-Reply, Autoreply, Abwesenheitsnotiz, Réponse automatique, Respuesta automática, Risposta automatica, Fuori sede, Automatisch antwoord, Afwezig, Resposta automática, Odpowiedź automatyczna, 自動返信, 自动回复, 자동 응답) only combined with at least one header signal OR body under 1,000 chars with no prior thread customer message in last 1 min | substantive false, `metadata.auto_reply = true`; thread match: append, never reopen, never unsnooze, no notify; no thread: ignore `auto_reply_no_thread` |
| 7 | spam | SES spamVerdict FAIL; virusVerdict FAIL (attachments dropped, `scan_status=infected`); `X-Spam-Flag: YES` from trusted source; DMARC fail + reject policy (section 5); contact blocked (`blocked_at`); sender rate over quarantine limit | ingest with `conversations.spam = true` (new conversation) or message flagged spam on existing thread (`metadata.email.spam = true`, no notify, no AI); status quarantined |
| 8 | forwarded | sender is active workspace member (From email equals `users.email` of active member, auth pass), AND forwarded block detected (section 7.3) | section 7.3 |
| 9 | test | sender is active workspace member, auth pass, not forwarded, no thread match | normal ingest + `InboundMessage.IsTest = true` (03 sets `conversations.is_test`, D10) |
| 10 | list | `List-Id`, `List-Post`, or `Precedence: list` (RFC 2919/2369), mailing list software headers (`X-Mailman-Version`, `X-Google-Group-Id`, `Mailing-List`) | thread match: normal (human list member reply, substantive true); no thread: per `bulk_mail_handling` |
| 11 | bulk | `Precedence: bulk|junk`; `List-Unsubscribe` + (`Feedback-ID` or ESP headers `X-Mailgun-Tag`, `X-SG-EID`, `X-SES-Outgoing`, `X-Mailchimp-*`, `X-Campaign`, `X-MC-User`, `X-Postmark-*` broadcast stream) | thread match: substantive false, `auto_reply = true`; no thread: per `bulk_mail_handling` |
| 12 | noreply | sender local part matches `noreply_patterns.yaml` (no-reply, noreply, donotreply, do-not-reply, notifications, notification, mailer, bounce, alerts, automated) and NOT form-relay (section 7.1) | treat as bulk |
| 13 | normal | else | substantive true |

`bulk_mail_handling` (per address): `close` (default): new conversation created with status closed, tag `bulk-email`, substantive false, no notify, no AI; `open`: open, substantive false, no AI, normal notify; `drop`: ignore `bulk_drop`.

Classification stored in `inbound_emails.classification` and `messages.metadata.email.classification`. Members can reclassify by "Not auto-reply" action (03 UI, 04 endpoint not needed: 03 message update sets `substantive=true`, clears `auto_reply`, re-dispatch AI trigger is 07's call). Reclassification feedback not learned in MVP.

Never auto-respond (07 and future rules) when: classification not normal/forwarded/test, `Auto-Submitted` non-no, envelope from `<>`, `metadata.ai_skip` set, or message `metadata.imported` (16; imported mail never enters this pipeline, guard is defense).

### Rate limits (step e)

- Counters via `a.Limiter` (Valkey sliding window, key `email:sender:{ws}:{sha256(sender)}`); rebuildable, lost counters only loosen limits briefly.
- Per workspace per sender address: over `sender_rate` (20 per 10 min): message stored normally but `metadata.ai_skip = "throttled"`, notifications suppressed (`metadata.email.throttled = true`). Over `sender_quarantine` (100 per hour): classified spam (quarantined).
- Per workspace total inbound: no cap (customer mail never refused); cloud abuse alert at 50,000/day.
- Automated outbound per recipient (`automated_per_recipient_hourly` 10): AI replies, widget offline mail over limit not sent; AI run skips with handoff (07 consults `email.limits.CanAutoReply` via message metadata `ai_skip = "recipient_rate"` on next inbound).

## 7. Sender identity

### 7.1 Form relays

`form_relays.yaml`: From domains/addresses of form services (webflow.com, typeform.com, tally.so, framer.com/framerusercontent, formspree.io, jotform.com, netlify.com forms, wix-forms, squarespace, hubspot forms, google forms notifications, basin, getform, formsubmit.co) plus generic rule: From local part noreply-like AND exactly one Reply-To address on different domain.
- Match: contact = Reply-To address (identity unverified, never `pass`), display name from Reply-To. `metadata.email.form_relay = "<service>"`. Classification stays normal (not noreply). Replies go to Reply-To.
- Form relay with no Reply-To: contact = email parsed from body field `Email:` line (regex, first match, validated); none found: contact = relay address, `metadata.ai_skip = "no_reply_address"`, replies blocked in UI with reason (03 shows "no customer address").

### 7.2 Normal

- Contact address = From (first address; multiple From: first, warning). `Sender` header ignored for identity.
- Reply-To honored as outbound To only when: auth pass, or contact is lead without verified identity. Stored `metadata.email.reply_to`.
- Contact lookup by 03 (`ReceiveInboundMessage` resolves contact): 04 passes `InboundSender{Identities: [IdentityClaim{Type: email, Value: lowercased, Verified: auth pass}], Name, Email}`. Display name updates contact `name` only when contact name empty.

### 7.3 Forwarded by member

- Detect inline forward blocks (first match in HTML DOM then text): Gmail `---------- Forwarded message ---------` + `From:`/`Date:`/`Subject:`/`To:` lines; Apple `Begin forwarded message:`; Outlook `________________________________` + `From:`/`Sent:`/`To:`/`Subject:` (localized: Von/De/Da/Van/Från, Gesendet/Envoyé/Enviado/Inviato/Verzonden, Betreff/Objet/Asunto/Oggetto/Onderwerp); Thunderbird `-------- Forwarded Message --------`; Yahoo/Proton variants. Attached `message/rfc822` (forward as attachment) preferred when present.
- Requires forwarder = active member (section 6 #8). Non-member forwards (customer forwards thread from colleague): normal message from forwarder; forwarded block kept in body (not stripped as quote).
- Result: contact = original From (identity unverified: never pass, original auth unverifiable inline; `message/rfc822` attachment: DKIM verify original, may pass). Message author = contact; body = forwarded block content (text above the block = member comment). Member comment non-empty: internal note (type note, author member) "Forwarded by Sam: <comment>" else note "Forwarded by Sam". `metadata.email.forwarded_by_member_id`. Subject = original subject (Fwd prefix removed). Thread: original's Message-ID (from rfc822 attachment) used for refs if present; else new conversation. `sent_at` = original Date.
- Original From unparseable (Outlook "From: Bob" with no address): conversation created with contact = new lead `name: "Bob"`, no email; note explains "Original sender address missing; add email to reply". Forwarder never becomes contact.

## 8. Threading

Inputs: reply token (from any recipient incl. Delivered-To), In-Reply-To ids, References ids (all normalized: trim, strip `<>`, collapse whitespace, max 50 ids, each max 998), normalized subject, sender, address.

Resolution order (first hit wins, all scoped to workspace):

1. **Reply token** -> `email_threads` (not revoked) -> conversation. Token for other workspace: ignored (section 3).
2. **In-Reply-To** ids -> `email_message_refs` (any kind). Multiple conversations hit (merged history, shared refs): pick most recent open; else most recent.
3. **References** newest to oldest (last id first), max 20 lookups.
4. **Provider id**: ids rewritten by provider (SES `@<region>.amazonses.com` / `@email.amazonses.com`; others when driver reports `mail.SendResult.MessageID`) looked up as kind `outbound_provider` (covered by 2/3 since stored as refs; listed for clarity). Provider rewrote and did not report: steps 1 (token) and 7 carry threading (edge 174).
5. **Own Message-ID shape** `<msg_<b32>.<rand>@<sending domain>>`: decode message public id -> conversation (covers refs table loss after restore from partial backup; same workspace check).
6. **Reverse refs** (out-of-order arrival): this email's own Message-ID found in `email_message_refs` kind `referenced` -> that conversation (earlier-arrived reply already referenced it).
7. **Subject heuristic** only when ALL hold: message has `Re:`/`Fwd:`-family prefix OR has References/In-Reply-To (unknown ids); normalized subject length at least 8 chars and not in generic list (`hello`, `hi`, `help`, `question`, `support`, `(no subject)`, `inquiry`, `issue`, `problem`, `test`, `urgent`, localized equivalents); same contact (by email identity); same email address connection; conversation status open or snoozed; last message within 14 days; exactly one candidate. Method `subject` logged.
7b. **Contact mail reply** (no conversation hit above): In-Reply-To or References id equals `outbound_emails.message_id_header` (or provider id) of row with contact-mail kind (`changelog`, `feedback_update`, `portal_sign_in`; outbound.md 16). Classification auto_reply / bulk / noreply / bounce-like: discard, `inbound_emails.status` ignored, `ignore_reason` `contact_mail_auto_reply`, stat `dimmi_email_contact_mail_replies_total{kind, result="discarded"}` (13: OOO to changelog mail never opens conversation). Human reply: step 8 new conversation, `source.contact_mail_kind` = row kind, 14 `feedback_post_id` from tags when present (changelog replies get no entry tag); contact resolved normally (section 7).
8. **New conversation**: `external_thread_key` = root id (first References id, else In-Reply-To, else own Message-ID). Unique constraint (channel_connection_id, external_thread_key) where not closed: conflict means concurrent creation -> re-run resolution inside lock (section 8.2).

Subject normalization (`subject.go`): repeatedly strip prefixes `re`, `fw`, `fwd`, `aw`, `wg`, `sv`, `vs`, `antw`, `r`, `rif`, `tr`, `enc`, `res`, `odp`, `pd`, `ynt`, `ilt`, `回复`, `转发`, `答复`, `RE`, `FW`, with optional `[n]`/`(n)` counters and `:` or full-width `：`; strip bracket tags `[EXTERNAL]`, `[External]`, `[SPAM]`, `[#1042]` (ticket tag, also used to match when `subject_tag` on: `[#<number>]` -> conversation number lookup, treated as step 1.5 with same-contact guard); collapse whitespace; casefold; NFC.

Closed conversation hit (steps 1 to 7): 03 reopen rule: closed within `reopen_window_days` -> reopen (substantive only); older -> new conversation with column `previous_conversation_id` (03 sets), new email_threads row (new token), refs of new message point to new conversation. New conversation `external_thread_key` = new message's own Message-ID (never old root), so manual reopen of old conversation never violates unique `(channel_connection_id, external_thread_key) WHERE status <> 'closed'`.

Merged conversation hit (03 merge leaves source as tombstone with column `merged_into_id`): follow pointer to target (03 `ReceiveInboundMessage` also follows chain).

Deleted conversation hit: treat as no hit (continue resolution; tokens revoked anyway).

### 8.1 Refs written

- Inbound message: own Message-ID (kind inbound), each References/In-Reply-To id not already known (kind referenced, message_id null) for reverse lookup. Cap 20 referenced rows per message.
- Outbound: our Message-ID (outbound) at send; provider-assigned id as `outbound_provider` after send when it differs: SES `<MessageId>@<region>.amazonses.com` (`email.amazonses.com` for us-east-1; SES overrides Message-ID header; region suffix from configured SES region, both forms stored when uncertain); other drivers `mail.SendResult.MessageID` when non-empty and different.

### 8.2 Concurrency

- `ProcessInboundEmail` takes `pg_advisory_xact_lock(hashtext(workspace_id || ':' || lower(sender)))` around steps g to i. Two rapid mails from same sender serialize; second sees first's refs.
- Different senders same thread (Cc participants replying simultaneously): both may resolve by token/refs to same conversation; no conflict. Both new-thread with same root: unique constraint conflict -> retry once inside lock keyed by root id.

## 9. Body

1. Pick HTML part (multipart/alternative prefers HTML), else text.
2. HTML: sanitize via `framework/sanitize` email policy built in `app/integrations/email/htmlbody.go` with `sanitize.NewPolicy()` (bluemonday strict plus email-safe tags/attrs: tables, inline style allowlist, `img src` http(s)/cid/data image only). Remove `<style>` blocks, tracking pixels (1x1 or `display:none` images) dropped from stored HTML.
3. Quote + signature stripping, HTML pass (DOM, `dom.Parse` / `doc.Find`, `framework/dom`): mark first top-level quote container: `div.gmail_quote`, `blockquote[type=cite]` (Apple/Thunderbird), `div#divRplyFwdMsg` + following siblings (Outlook web/new Outlook), `div#appendonsend` siblings, `hr#stopSpelling` (Outlook.com legacy), `div.yahoo_quoted`, `div.protonmail_quote`, `blockquote.protonmail_quote`, `div.moz-cite-prefix` + next blockquote, `div[data-marker="__QUOTED_TEXT__"]` (Zimbra), Outlook desktop `div[style*="border-top:solid #E1E1E1"]` header block, `#zwchr` (Zoho). Everything from marker to end wrapped `<div data-dimmi-quoted>`. Signature: `div.gmail_signature`, `div[data-smartmail=gmail_signature]`, `#Signature` (Outlook), `div.moz-signature`, `#ms-outlook-mobile-signature` wrapped `<div data-dimmi-signature>`.
4. Text pass (no HTML or HTML pass found nothing): port of GitHub `email_reply_parser` (Ruby, MIT, github/email_reply_parser; attribution: MIT notice + upstream commit hash in `app/integrations/email/quotes.go` header and `THIRD_PARTY_NOTICES.md`; no maintained Go library exists, research/build-vs-reuse.md 3.5) + extensions: quote header lines from `quote_headers.yaml` (English "On <date>, <name> wrote:" incl. line-wrapped 2-line form; "Le ... a écrit :", "Am ... schrieb ...:", "El ... escribió:", "Il ... ha scritto:", "Op ... schreef ...:", "Em ... escreveu:", "W dniu ... pisze:", "... написал(а):", "<date>，<name> 写道：", "<date> <name> <email>のメッセージ:"), Outlook header block (From/Sent/To/Subject 4 lines, localized), `-----Original Message-----` (and localized), `>` quoted line runs at end; signature: RFC 3676 delimiter `-- \n`, "Sent from my iPhone/iPad/Android/Outlook for iOS/Samsung", "Get Outlook for iOS/Android", localized; trailing lines after delimiter only when under 12 lines.
5. Safety rules: never strip to empty (if stripped body empty or only whitespace: keep full text, `metadata.email.strip_fallback = true`); never strip content above first marker; quoted text in middle (inline replies, interleaved `>` blocks followed by new text) kept in full (detect: non-quoted text after quote block -> no strip); keyword `HUMAN` (07 footer instruction) in stripped part ignored, only visible part counts.
6. Markdown: visible part converted HTML -> Markdown via `dom.ToMarkdown` (wraps `github.com/JohannesKaufmann/html-to-markdown/v2` v2.5.x; 04 rules as converter options: links kept with http/https/mailto, images become `![alt](attachment-or-url)` only for cid/attachments, remote images become links `[image: alt](url)`), whitespace normalized, max 64 KB (cut at paragraph boundary, append "\n\n[Message truncated. View original.]", `metadata.email.body_truncated = true`).
7. `messages.body` = Markdown of visible part; `messages.body_html` = full sanitized HTML with quoted/signature wrappers (cap 512 KB; over cap: null + `metadata.email.html_too_large = true`, original via raw).
8. Plain-text-only mail: body = text (Markdown-escaped except URLs autolinked), `format=flowed` (RFC 3676) unwrapped, `body_html` null.
9. Attachments: each part with disposition attachment or non-text inline part -> filesystem store via 03 attachment action (`uploaded_by_type = contact`), sha256, sniffed type, `scan_status` pending (or infected from SES virus FAIL). Inline images with Content-ID: `inline = true`, `content_id` stored; HTML `src="cid:x"` kept in stored HTML, rewritten by resource layer to signed URL. Limits: over `max_attachments` (50) or per-file over `filesystems.max_upload_mb` or blocked type (executables per foundation): skipped, listed in `metadata.email.skipped_attachments` [{filename, size, reason}] and system event line in timeline. Duplicate inline images (same sha256) stored once per message. Signature logos (inline image under 10 KB inside signature wrapper) stored but `metadata.signature_asset = true` (UI hides).
10. Empty body + attachments only: body "" allowed (03 allows attachment-only); 07 skips `no_text`.

## 10. Ingest

04 calls 03 action `conversations.ReceiveInboundMessage(ctx, contracts.InboundMessage)` (03 `channel-contract.md` section 3; one tx) with `AfterPersist` hook set; hook writes 04 rows (`email_message_refs`, `email_threads`, `inbound_emails.message_id`/`conversation_id` linkage and status) inside 03 tx (channel-contract.md section 3 step 9b). Hook error rolls back whole ingest; `process_inbound_email` job retries. All 04 writes idempotent; retry after commit gets `Duplicate` result with existing ids. Field mapping:

| `InboundMessage` field | Email value |
|---|---|
| ConnectionID | routed `email_addresses.channel_connection_id` |
| DedupeKey | `email:` + hex(sha256(message_id_header))[:32] (missing Message-ID: sha256 of From + Date + Subject + first 4 KB body; reused id edge case 3: + body hash suffix) |
| ExternalThreadKey | thread root id (section 8 step 8); existing conversation key when matched and conversation open, snoozed, or closed within `reopen_window_days`; matched conversation closed past window: this message's own Message-ID (03 creates linked new conversation with that key, so old root never collides on manual reopen) |
| ConversationID | conversation from threading steps 1 to 7 (token, refs, subject); nil when new |
| ForceNewConversation | false (closed-past-window handled by 03 reopen rule) |
| Sender | `InboundSender{Identities: [{Type: email, Value, Verified: auth pass}], Name, Email}`; participant reply: that participant's address |
| Participants | To/Cc addresses (filter below), max 50 |
| Subject | decoded subject (03 trims 255) |
| Body | Markdown of visible part (section 9) |
| BodyHTML | sanitized HTML with quote/signature wrappers (03 sanitizes again) |
| Attachments | `InboundAttachment{Filename, ContentType, SizeBytes, Inline, ContentID, Open}` (03 stores bytes before tx) |
| ChannelMessageID | Message-ID header |
| ReplyToChannelMessageID | In-Reply-To |
| SentAt | parsed `Date` (section 4) |
| AutoGenerated | true when classification gives `substantive = false` (auto_reply, bulk, noreply, list-without-thread bulk path): 03 sets `substantive=false`, no reopen, no unsnooze, no `waiting_since`, no notify; never counts toward D1 activation |
| SpamVerdict | `spam` when class spam (section 6 #7), else SES verdict / `unknown` |
| Source | `{channel: email, address_id, inbound_email_id}` |
| IsTest | true for class test (section 6 #9, D10); 03 sets `conversations.is_test`. Never from headers alone |
| Metadata | `email` object below (max 8 KB; overflow in `inbound_emails`) |

Result message (03 writes): `type=message`, `author_type=contact` (or participant contact), `channel_message_id`, `dedupe_key`, `sent_at`, `substantive = !AutoGenerated`, `body`, `body_html`, `metadata`:

```json
{
  "auto_reply": false,
  "ai_skip": null,
  "email": {
    "inbound_email_id": "ibm_...",
    "from": {"name": "Bob", "email": "bob@customer.io"},
    "to": ["support@acme.com"], "cc": ["ana@customer.io"],
    "reply_to": null,
    "subject": "Re: Refund for order 1042",
    "in_reply_to": "msg_01jc.k2m9@acme.com",
    "original_recipient": "support@acme.com",
    "classification": "normal",
    "thread_method": "in_reply_to",
    "auth": {"result": "pass", "aligned_domain": "customer.io"},
    "stripped": {"quote": true, "signature": true, "fallback": false},
    "truncated": false,
    "skipped_attachments": [],
    "form_relay": null,
    "forwarded_by_member_id": null
  }
}
```
- Participants: To/Cc addresses (minus workspace addresses, forwarding addresses, inbound domain, default sender, suppressed-invalid, sender) upserted as contacts (lead, unverified email identity) + `conversation_participants` role `cc`, max 50 (rest in metadata only). Existing participant replying: author = that participant contact. Participant reply never changes conversation `contact_id`.
- Sender not contact and not participant on token/ref-matched thread: added as participant role `participant`, `metadata.ai_skip = "non_participant"`, `metadata.email.new_participant = true` (UI banner).
- Bcc'd address (our address only in envelope, not To/Cc): `metadata.email.bcc = true`; replies default To = sender only (no Cc of visible recipients) to avoid revealing Bcc.
- `email_threads` row ensured for (conversation, address) with new token if missing.
- `conversations.subject` set on create (decoded, prefix-normalized display keeps original casing, 255 chars).
- `channel_connections.last_activity_at`, `email_addresses.last_inbound_at`, `inbound_verified_at` (first time) updated.
- `inbound_emails` status `processed`, `conversation_id`, `message_id`.

### 10.1 Bounces (DSN)

- Parse RFC 3464 `message/delivery-status`: per recipient `Final-Recipient`, `Action` (failed, delayed, delivered, relayed, expanded), `Status` (x.y.z), `Diagnostic-Code`. Original message identified by `text/rfc822-headers` or `message/rfc822` part Message-ID; fallback VERP token in envelope recipient (`bounces+<token>`); fallback In-Reply-To.
- Non-standard bounces (no report part): regex for recipient + 5xx in body, original Message-ID in quoted headers; else ignore `bounce_unmatched`.
- Action `failed` with status 5.x.x: hard bounce -> `RecordEmailBounce` (permanent). 4.x.x / `delayed`: transient, recorded on outbound recipient status, no failure unless final.
- Matched outbound: `outbound_emails.recipient_statuses` updated; if recipient is primary To: 03 `UpdateMessageDelivery` failed, `ErrorMessage` = "550 5.1.1 User unknown (gmail.com)" (diagnostic trimmed 300 chars), emits `message.delivery_updated`, system event message in conversation "Email to bob@x bounced: mailbox does not exist". Suppression hard_bounce (scope all).
- Matched outbound without `message_id` (contact mail, outbound.md 7.2): recipient status + suppression per kind scope rules, no 03 call, no timeline line.
- Bounces for event-capable transports (`ses`, `postmark`, `resend`, `mailgun`, `sendgrid`) come via events (outbound.md 7, 7.1); DSN to forwarding address for those ignored as duplicate if outbound already failed (03 monotonic). DSN is primary only for `smtp` relay.

### 10.2 Complaints (ARF)

- `multipart/report; report-type=feedback-report`: `Feedback-Type` (abuse, fraud, virus, other, not-spam), original headers part. Match outbound by Message-ID. `abuse|fraud|virus`: suppression complaint (scope `all`; contact mail kinds use kind scope, outbound.md 8), outbound status complained, stats++, event `email_suppression.created`. `not-spam`: log only.

## 11. Gmail forwarding confirmation

- Detection rule section 6 #3. Extract code `\(#(\d+)\)` from subject; confirm URL first `https://mail-settings.google.com/mail/` or `https://mail.google.com/mail/` link in body (validated host allowlist). Store in `email_addresses.forwarding_confirmation`, set `inbound_verified_at`, event `email_address.forwarding_confirmation_received` (realtime admins).
- Never auto-click. Dashboard shows code + "Open Gmail confirmation" link + explanation. Admins only.
- Other providers (Fastmail, Zoho) with confirmation mail: generic rule: From in `forwarding_confirmations.yaml` (Zoho `noreply@zohoaccounts.com` subject "Confirm forwarding", iCloud none, Outlook none). Same storage.

## 12. Failure handling

| Failure | Behavior |
|---|---|
| Raw read error (S3 missing, disk) | retry; after 8 attempts status failed, admin activity log shows "Raw email unavailable" |
| 03 ingest error (validation) | status failed with error code, reprocess available |
| DNS timeouts during auth | auth unknown, continue (never block ingest on DNS) |
| Attachment store failure | retry job (attachments before message commit; orphan attachments purged by foundation 24 h job) |
| Workspace suspended | stays received; periodic `ReprocessSuspendedInboundEmails` (hourly) re-enqueues rows of workspaces unsuspended within 30 days; older rows purged with raw |
| Workspace deleted | rows cascade; raw purged by workspace purge job |
