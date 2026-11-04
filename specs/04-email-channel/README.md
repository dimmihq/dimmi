# 04 Email channel

Style: caveman. Owner: 04-email-channel. Read with `specs/00-conventions/README.md`, `specs/00-foundation/*`. Companion files: [pipeline.md](pipeline.md) (inbound pipeline, threading, classification, stripping, auth algorithms), [outbound.md](outbound.md) (composing, sending, domains, DNS, reputation), [sending.md](sending.md) (own address, sender choice, signatures, files, template style, guided DNS, provider domain APIs), [sender-rules.md](sender-rules.md) (ignored, trusted, suppression screen, flags), [setup.md](setup.md) (guides, test button, mistake detection, IT admin mail), [sources.md](sources.md) (inbound providers, IMAP), [mailbox-connections.md](mailbox-connections.md) (Google, Microsoft OAuth), [staging.md](staging.md) (real-world testing, infrastructure), [edge-cases.md](edge-cases.md), [tasks.md](tasks.md). Bug-fix agent still edits `app/actions/email` and `app/integrations/email`: re-read before building.

Status 2026-11-04: UI in scope. Owner order: everything below ships at launch.

## Goal

Email both ways in same inbox as chat. Customer writes to support@ (forwarded to Dimmi forwarding address, SES receiving), lands as conversation in under 10 s, correctly threaded, quotes stripped. Teammate or Dimmi Agent replies from customer's own domain with aligned SPF/DKIM/DMARC. Widget visitor who left page gets reply by email and can answer by email.

Sending model (binding, product owner): Dimmi sends mail for workspaces through managed provider API or relay set once per instance (deploy config), or through customer's own connected Google or Microsoft mailbox API. Dimmi never runs mail servers, never owns IPs or IP reputation (see "Never").

Why: roadmap launch item 4 ([research/roadmap.md](../../research/roadmap.md) "4. Email channel both ways"). support@ still main channel; visitor leaves before reply; reply must reach them. Also roadmap item 1 bar "Offline reply reach visitor by email".

## World-class bar (measurable)

| # | Bar | Target |
|---|---|---|
| 1 | Cloud forwarding address ready | exists at signup: created by 04 initializer in foundation workspace initializer hook (inside `CreateWorkspace` tx, no DNS) |
| 2 | Inbound latency (forward via SES) | p95 under 10 s from provider delivery to `message.created` (foundation budget); p50 under 3 s |
| 3 | Threading accuracy | 100% on corpus replies carrying reply token or known Message-ID; at least 98% on 500-reply real-client corpus (Gmail web/iOS/Android, Outlook web/desktop/mobile/new Outlook, Apple Mail, Yahoo, Proton, Fastmail, Thunderbird, Spark, HEY) |
| 4 | Quote and signature stripping | at least 97% exact on 300-fixture corpus across 12 locales; false strip of customer-written text under 0.5%; full original always one click away |
| 5 | Auto-mail detection | zero Dimmi Agent replies to OOO, bounce, DSN, newsletter, list mail in corpus; human mail misclassified non-substantive under 1% |
| 6 | Domain verification UX | every record copy-paste ready with per-record live status; detected within 2 min of DNS propagation after "Check now"; common mistakes named in plain words (doubled domain, Cloudflare proxied CNAME, duplicate SPF, SPF over 10 lookups) |
| 7 | Deliverability | mail from verified domain passes DKIM aligned, SPF aligned (SES custom MAIL FROM), DMARC on Gmail, Outlook.com, Yahoo; zero tracking pixels, zero link rewriting |
| 8 | Spoof safety | AI never answers spoofed mail claiming verified user identity; unauthenticated mail never auto-merges |
| 9 | Loop safety | zero mail loops in chaos test (two Dimmi workspaces auto-replying to each other, OOO ping-pong, list echo) |
| 10 | Provider outage | provider down never loses inbound (raw stored before 200) and never drops outbound (retries 24 h, `pending` visible) |
| 11 | Memory | email parsing bounded by 64 MB process budget (fits 256 MB self-host box) |

## Gap map

| Gap | Where specified | Tasks |
|---|---|---|
| E-01 | [sending.md](sending.md) "1. Own address and domain authentication"; [outbound.md](outbound.md) 11 | M8 |
| E-02 | [sending.md](sending.md) "2. Choose sending address" | M14 |
| E-03 | [sending.md](sending.md) "3. Signatures"; [outbound.md](outbound.md) 14 | M6, M14 |
| E-04 | [sender-rules.md](sender-rules.md) "1. Ignored addresses" | M15 |
| E-05 | [sender-rules.md](sender-rules.md) "2. See and lift suppression"; [outbound.md](outbound.md) 8 | M7 |
| E-06 | Reference only: folded into inbox S-12 (`03-shared-inbox`); [sender-rules.md](sender-rules.md) "3. Strip links" | 03 track 04 |
| E-07 | [sending.md](sending.md) "4. Files hosted or attached" | M14 |
| E-08 | [sending.md](sending.md) "5. Template style" | M14 |
| E-09 | [sending.md](sending.md) "6. Guided DNS" | M14, M20 |
| I-142 | [sender-rules.md](sender-rules.md) "4. Trusted senders and rate-limit label" | M15 |

Also in this round: SES route (M5), cloud limits and auto-pause (M11, [../12-cloud-billing/README.md](../12-cloud-billing/README.md) "Email sending limits"), setup (M16), providers + IMAP (M17, M18), OAuth (M19), staging (M20), unverified sender flag (M15), provider domain APIs (M14).

## Scope

### In

- Inbound sources (all feed one pipeline):
  - Cloud forwarding address `<local>@inbound.trydimmi.com` (SES receipt rule -> S3 + SNS -> webhook). Default address created by 04 initializer in foundation workspace initializer hook (`00-foundation/contracts.md`), not lazily; self-host same when `inbound_domain` set.
  - Self-host inbound domain (`channels.email.inbound_domain`) fed by any enabled source: SES receipt rule (own AWS), Postmark, Mailgun, SendGrid, Resend, Cloudflare Email Routing ([sources.md](sources.md)). No built-in SMTP receiver (Never list).
  - Mailbox connection per address: IMAP app password ([sources.md](sources.md) 3), Google and Microsoft OAuth with Sent sync and send-as ([mailbox-connections.md](mailbox-connections.md)).
  - Setup guides per mail host, end-to-end test button, mistake detection, send-to-IT-admin ([setup.md](setup.md)).
- Outbound: one instance-level provider (deploy config `mail.*`, framework `mail.Mailer`): cloud = Dimmi AWS SES (shared IP pools, SES tenant per workspace); self-host = admin picks `ses` (own AWS), `smtp` (relay/submission service), `postmark`, `resend`, `mailgun`, `sendgrid`. Same provider carries system mail (to Dimmi users) and customer-facing mail (to contacts) on separate logical streams (named mailers, SES configuration sets / Postmark message streams / tags; outbound.md section 15). Workspaces never connect own provider.
- Custom sending domain per workspace through instance provider: generated DNS records (ownership TXT, DKIM, return-path MX/SPF, DMARC), live per-record checker, verification polling job, daily recheck, degrade fallback. `ses`: SES identity + Easy DKIM + custom MAIL FROM per domain (automated). `smtp`: Dimmi DKIM key per domain. `postmark`/`resend`/`mailgun`/`sendgrid`: through provider domain API when token set ([sending.md](sending.md) 7), else admin sets domain up at provider, declares it in config (outbound.md 11.1). Per-host instructions, deep links, Domain Connect ([sending.md](sending.md) 6).
- Default sender on cloud domain until custom domain verified. Reply-To reply-token address.
- Threading: reply token, In-Reply-To, References, SES rewritten Message-ID mapping, out-of-order reconciliation, subject fallback heuristic with guards.
- Quoted reply + signature stripping (HTML-aware, multi-locale), HTML sanitization, HTML to Markdown, plain-text extraction, charsets, encodings, inline images (cid), attachments.
- Classification: auto-reply/OOO (RFC 3834 + vendor headers + subject patterns), DSN bounce (RFC 3464), ARF complaint (RFC 5965), mailing list (RFC 2369/2919), bulk/newsletter, noreply senders, Gmail forwarding confirmation, loop, spam/virus. Non-substantive never triggers AI, never reopens, never notifies.
- Loop prevention: never auto-reply to auto mail, loop header, own-address detection, per-sender inbound rate limits, per-recipient automated outbound limits.
- CC/BCC handling, participants, reply-all default, editable recipients per reply.
- Forwarded email detection: teammate forwards customer mail into inbox, original sender becomes contact.
- Form-relay detection (Webflow, Typeform, Tally, Framer forms): Reply-To becomes contact.
- Inbound DKIM/SPF/DMARC verification (own DKIM/DMARC + SES verdicts) for identity verification on contact merge, AI spoof guard, visible unverified sender flag ([sender-rules.md](sender-rules.md) 5).
- Bounce, complaint, delivery handling (SES events; Postmark, Resend, Mailgun, SendGrid event webhooks, signature-verified; DSN parsing for `smtp` relay), `delivery_status`, suppression list, D13 credit signal.
- Widget offline replies: 01 owns trigger, delay, presence check, message collection and template (`SendWidgetOfflineReplyEmail` job, `WidgetOfflineReplyMail`); 04 owns delivery action `email.SendWidgetOfflineReply` (sender, Reply-To token, unsubscribe headers, suppression, limits, MIME, Mailer, `outbound_emails`). Replies by email thread into same widget conversation.
- Templates: plain (default) or branded style per address ([sending.md](sending.md) 5), signatures per member and per address (3), AI reply, widget offline reply, test mail.
- Ignored addresses, trusted senders, rate-limit label, suppression screen ([sender-rules.md](sender-rules.md)).
- Settings UI under `resources/js/features/email/`, mounted in `features/settings/pages/email.tsx`.
- Unsubscribe headers (RFC 2369 + RFC 8058 one-click) on automated notification mail only.
- Sending limits and reputation on cloud: per-workspace daily/hourly caps (entitlement keys, overridable per customer), bounce/complaint auto-pause (SES account health is Dimmi's), SES tenant per workspace.
- Customer-facing mail API for other specs (13 changelog, 14 voters + portal sign-in): outbound.md section 16 (`SendContactMail`). 15-announcements-surveys CSAT requests are in-thread system messages on `Channel.Send` (outbound.md 5). Any mail to a contact, visitor, subscriber or voter goes through a 04 action on customer stream; never through system Mailables.
- Large email limits, raw `.eml` retention, reprocess, "view original".
- REST API, MCP tools, CLI commands listed below.

### Never

Product owner rule: no maintaining mail servers, IPs or IP reputation.
- No outbound MTA. No direct-to-MX delivery (no MX lookup for outbound anywhere; `smtp` driver talks only to configured relay host).
- No dedicated IPs, no IP warming, no IP pool management. Cloud SES uses shared pools only.
- No built-in SMTP receiver (no port 25 listener). Inbound = provider webhooks and mailbox sync over HTTPS or IMAP ([sources.md](sources.md)).
- No BYO SMTP and no per-workspace sending provider key (no SMTP host, no Postmark or SES credentials per workspace or per address). Instance provider = deploy config only. Only exception, and only per connected mailbox: OAuth grant for that mailbox's own Gmail API or Microsoft Graph, and IMAP read password; sealed through KeyManager ([mailbox-connections.md](mailbox-connections.md)). IMAP password never sends. Google restricted-scope verification and CASA are a human step; code and tests never depend on them.
- No open/click tracking, no link rewriting (privacy pledge).

### Left out (decided)

| Item | Why |
|---|---|
| OAuth IMAP for Gmail, Microsoft | replaced by Gmail API and Graph: same grant cost, better sync |
| Gmail forwarding automation through domain-wide delegation | rejected: saves little, fails for Groups ([setup.md](setup.md) 6) |
| ARC chain verification | no library in allowlist; connected mailbox answers broken-DKIM forward; trusted authserv list stays |
| IMAP IDLE | poll 60 s instead ([sources.md](sources.md) 3) |
| Outbound campaigns, broadcasts | not support mail; changelog email ships in 13 |
| Rules lite: auto-acknowledgement, office-hours auto-reply | workflow (17) and 03 office hours own it; 04 already marks `Auto-Submitted` |
| HTML email designer, arbitrary HTML | product owner: out; two styles cover it ([sending.md](sending.md) 5) |
| S/MIME, PGP | product owner: out, policy stays (`pipeline.md`) |
| Raw mailbox import (IMAP, mbox) | product owner: out; connections start at now; 16 imports helpdesk history |
| Two-way label or read state with Gmail | we never mutate customer mailbox |
| Undo send | 03 owns (track 03); 04 honors `ScheduledAt` |

## User stories

Workspace member:
- As owner on cloud, I copy `acme@inbound.trydimmi.com`, set Gmail forward, see Gmail confirmation code in Dimmi, paste it, done in 2 minutes.
- As admin, I add `acme.com`, copy 6 records into Cloudflare, click Check now, each record turns green, replies now go out as `support@acme.com`.
- As self-host admin with no port 25, I point inbound domain MX at SES receiving on own AWS and forward support@ to my forwarding address; replies go out through instance provider (Postmark set in config) from support@acme.com once acme.com verified.
- As self-host operator, I set `mail.default` + provider credentials once in config; member mail and customer replies both flow, on separate streams.
- As member, I see email conversations threaded like chat, quotes collapsed, "show original" for full HTML and headers.
- As member, I edit To/Cc before sending; Cc'd colleagues of customer stay on thread.
- As member, I forward a customer email from my own Gmail to support address; conversation shows customer as sender, note says "Forwarded by Sam".
- As member, I see why message not delivered (bounce reason) and admin can lift suppression.
- As member, I set my email signature once.
- As owner on Google Workspace, I follow routing rule steps, press Test, see it pass in a minute; or I send those steps to IT.
- As owner with a Google Group on support@, dimmi tells me and shows the fix.
- As admin, I connect support@ Gmail or Microsoft 365 by OAuth; replies land in its Sent folder; replies sent from Gmail show in dimmi.
- As self-hoster without AWS, I receive through Cloudflare, Postmark, Mailgun, SendGrid, Resend or IMAP.
- As admin, I trust my form relay so it never gets throttled, and I see "Too many emails" when something is.

End customer:
- As customer, I email support@acme.com and get reply from support@acme.com in same Gmail thread.
- As widget visitor who closed tab, I get email with reply and link that reopens chat; replying by email continues chat.
- As customer, my OOO auto-reply does not start bot ping-pong.
- As customer, I can stop email copies of chat replies with one-click unsubscribe.

Developer:
- As developer, I manage domains and addresses through REST API and MCP (`dimmi_get_email_domain` lets Claude Code read DNS records and set them via my DNS provider tool).
- As self-hoster, `dimmi email:doctor acme.com` explains DNS problems.

AI agent:
- Dimmi Agent answers only substantive, authenticated-or-unclaimed mail from conversation contact or existing participants; its replies carry `Auto-Submitted: auto-replied`.

## Data model

All tables: UUIDv7 `id`, `workspace_id uuid not null references workspaces(id) on delete cascade` (except noted), `created_at`, `updated_at`, text + CHECK enums, RLS in cloud, indexes lead with `workspace_id`.

New public ID prefixes (add to `app/models/public_ids.go`, `framework/ids` registry): `edm_` email domain, `ema_` email address, `ibm_` inbound email, `obm_` outbound email, `sup_` suppression, `sig_` signature, `eig_` ignored address, `ets_` trusted sender, `eat_` address test, `emc_` mailbox connection.

### email_domains
| Column | Type | Notes |
|---|---|---|
| domain | citext not null | lowercase, IDNA A-label stored (`xn--`), display U-label in resource; validated public suffix aware (not bare TLD, not `trydimmi.com`, not inbound domain) |
| status | text not null CHECK in ('pending','verifying','verified','degraded','failed') default 'pending' | pending = created, user not clicked check; verifying = polling; degraded = was verified, record now missing |
| driver | text not null CHECK in ('ses','smtp','postmark','resend','mailgun','sendgrid') | driver of customer-stream mailer (`channels.email.mailer`) at creation; change of instance driver triggers re-verification |
| provider_trusted_at | timestamptz | manual drivers (`postmark`, `resend`, `mailgun`, `sendgrid`): set when domain (or parent) listed in `channels.email.provider_verified_domains`; null for ses/smtp |
| ownership_token | text not null | random 24 chars; TXT `_dimmi-verification.<domain>` value `dimmi-verification=<token>` |
| ownership_verified_at | timestamptz | |
| dkim_selector | text | smtp driver: `dimmi<yyyymm>`; ses: null (tokens) |
| dkim_private_key_encrypted | bytea | smtp driver only, RSA 2048 PKCS8, envelope-encrypted |
| dkim_tokens | text[] | ses Easy DKIM tokens (3) |
| dkim_signing_hosted_zone | text | ses `SigningHostedZone` |
| mail_from_domain | citext | default `bounce.<domain>`; ses only |
| ses_identity_status | text | last SES `VerificationStatus` / DKIM status raw |
| records | jsonb not null default '[]' | expected records + last observed values + per-record status (see outbound.md) |
| warnings | jsonb not null default '[]' | code + message list from last check |
| check_attempts | int not null default 0 | |
| last_checked_at, next_check_at, verified_at, degraded_at, failed_at | timestamptz | |
| failure_reason | text | human readable |
| created_by_member_id | uuid null references workspace_members on delete set null | |
| deleted_at | timestamptz | |
Unique `(workspace_id, domain) WHERE deleted_at IS NULL`. Unique `(domain) WHERE status IN ('verified','degraded') AND deleted_at IS NULL` (instance-wide; one workspace owns verified domain). Index `(next_check_at) WHERE status IN ('verifying','verified','degraded') AND deleted_at IS NULL` (scheduler scan, cross-tenant job runs as system role).

### email_addresses
Detail row for `channel_connections` with `type = 'email'` (1:1).
| Column | Type | Notes |
|---|---|---|
| channel_connection_id | uuid not null unique references channel_connections on delete cascade | `channel_connections.external_id` = routing key (forwarding address) |
| address | citext not null | public address customers see (`support@acme.com`); for default cloud address equals forwarding address |
| display_name | text not null | 1 to 80, default workspace name |
| email_domain_id | uuid null references email_domains on delete set null | set when address domain matches verified domain |
| forwarding_address | citext not null unique | `<local>@<inbound_domain>` |
| reply_to_mode | text not null CHECK in ('token','none') default 'token' | token = Reply-To reply-token address |
| from_name_style | text not null CHECK in ('member_and_name','name_only') default 'member_and_name' | "Sam from Acme Support" vs "Acme Support" |
| is_default | boolean not null default false | default sender for widget offline mail and system mail; partial unique `(workspace_id) WHERE is_default AND deleted_at IS NULL` |
| settings | jsonb not null default '{}' | typed `models.EmailAddressSettings`: `quote_previous` bool (true), `subject_tag` bool (false), `bulk_mail_handling` ('close' default, 'open', 'drop'), `outbound_attachments` ('attach' default, 'link'), `template` ('plain' default, 'branded'), `brand_logo_attachment_id`, `brand_color`, `footer_markdown`, `detected_mail_host`, `sent_sync_new_threads` bool (false), `imap_append_sent` bool (true) |
| forwarding_confirmation | jsonb | last Gmail/other confirmation: `{provider, code, confirm_url, from, received_at}` |
| inbound_verified_at | timestamptz | first mail received on this address (forward works) |
| setup_findings | jsonb not null default '[]' | [setup.md](setup.md) 4; migration 20261201060005 |
| last_inbound_at, last_outbound_at | timestamptz | |
| deleted_at | timestamptz | |
Unique `(workspace_id, address) WHERE deleted_at IS NULL`.

### inbound_emails
One row per received email per workspace. Audit log of everything that arrived, including ignored mail.
| Column | Type | Notes |
|---|---|---|
| email_address_id | uuid null references email_addresses on delete set null | matched address |
| channel_connection_id | uuid null references channel_connections on delete set null | |
| source | text not null, registry enum | `ses`, `postmark`, `mailgun`, `sendgrid`, `resend`, `cloudflare`, `imap`, `gmail`, `microsoft`, `test`; migration 20261201060001 |
| provider_message_id | text | SES messageId |
| message_id_header | text | normalized, without angle brackets, max 998 |
| envelope_from | text | may be empty (`<>` for DSN) |
| envelope_to | text[] not null | matched recipients only (never full Bcc list of other workspaces) |
| from_address | citext | header From |
| subject | text | first 255 chars, decoded |
| size_bytes | bigint not null | raw size |
| raw_disk | text not null | filesystem disk name |
| raw_path | text | null after purge |
| raw_deleted_at | timestamptz | |
| status | text not null CHECK in ('received','processing','processed','ignored','failed','quarantined') | |
| classification | text CHECK in ('normal','auto_reply','bounce','complaint','list','bulk','noreply','system_notice','loop','spam','forwarded','test') | |
| auth | jsonb | `{spf, dkim, dmarc, result, aligned_domain, source}`; result in ('pass','fail','none','unknown') |
| provider_verdicts | jsonb | SES spam/virus |
| conversation_id | uuid null references conversations on delete set null | |
| message_id | uuid null references messages on delete set null | |
| ignore_reason | text | e.g. `auto_reply_no_thread`, `duplicate`, `loop_own_address`, `bulk_drop`, `suppressed_sender_rate` |
| error | text | last processing error (no body content) |
| attempts | int not null default 0 | |
| received_at | timestamptz not null | provider receive time |
| processed_at | timestamptz | |
Indexes: `(workspace_id, email_address_id, received_at desc)`, `(workspace_id, status, received_at)`, unique `(workspace_id, source, provider_message_id) WHERE provider_message_id IS NOT NULL` (webhook retry dedupe), `(raw_deleted_at) WHERE raw_path IS NOT NULL` (purge scan).

### email_message_refs
Threading index: RFC Message-ID to conversation.
| Column | Type | Notes |
|---|---|---|
| workspace_id | uuid | |
| rfc_message_id | text not null | exact bytes inside `<>`, trimmed; max 998 |
| conversation_id | uuid not null references conversations on delete cascade | |
| message_id | uuid null references messages on delete set null | null for `referenced` |
| kind | text not null CHECK in ('inbound','outbound','outbound_provider','referenced') | `outbound_provider` = SES-assigned id; `referenced` = id seen in References of stored mail but mail itself not seen yet |
| created_at | timestamptz | |
Primary key `(workspace_id, rfc_message_id, conversation_id)`. Index `(workspace_id, conversation_id)`. No `updated_at` (append-only; exception noted for conventions).

### email_threads
Reply token per conversation per sending address.
| Column | Type | Notes |
|---|---|---|
| conversation_id | uuid not null references conversations on delete cascade | |
| email_address_id | uuid null references email_addresses on delete set null | sending address (null = instance default sender) |
| reply_token | text not null | 16 chars Crockford base32 lowercase (80 bits random); unique instance-wide |
| root_message_id | text | first Message-ID of thread |
| normalized_subject | text | for heuristic |
| revoked_at | timestamptz | after GDPR erase or conversation delete |
Unique `(reply_token)`. Unique `(conversation_id, email_address_id) NULLS NOT DISTINCT`. Index `(workspace_id, conversation_id)`.

### outbound_emails
One row per sent email (teammate reply, AI reply, system message e.g. 15 CSAT request, widget offline relay, contact mail for 13/14, test).
| Column | Type | Notes |
|---|---|---|
| message_id | uuid null references messages on delete set null | source message (first message for coalesced widget mail) |
| conversation_id | uuid null references conversations on delete set null | |
| email_address_id | uuid null references email_addresses on delete set null | |
| kind | text not null CHECK in ('reply','ai_reply','system','widget_offline','new_conversation','test','changelog','feedback_update','portal_sign_in') | `system` = `author_type system` message in thread (15 CSAT request); last three = contact mail kinds (outbound.md 16; `conversation_id`/`message_id` null); caller sub-kind in `metadata.purpose` |
| from_address | text not null | |
| to_addresses, cc_addresses, bcc_addresses | text[] not null default '{}' | |
| subject | text not null | |
| message_id_header | text not null | our generated Message-ID |
| transport | text not null CHECK in ('ses','smtp','postmark','resend','mailgun','sendgrid','log','gmail','microsoft') | driver of customer-stream mailer at send, or connected mailbox API; last two from migration 20261201060001 |
| provider_message_id | text | SES MessageId, Postmark MessageID, Resend id, Mailgun id, SendGrid `X-Message-Id`, or SMTP relay queue id |
| status | text not null CHECK in ('queued','sent','delivered','bounced','complained','failed','suppressed') | |
| recipient_statuses | jsonb not null default '{}' | per address: status, smtp code, diagnostic, at |
| size_bytes | bigint | |
| attempts | int not null default 0 | |
| last_error | text | |
| metadata | jsonb not null default '{}' | `purpose` (contact mail sub-kind, e.g. `confirmation`, `entry`, `sign_in`; no `digest`: later changelog-digest), `stream`, caller `tags` (13 `entry`, `mailing`; stats only, no reply attribution: later changelog-replies-to-inbox), `idempotency_key`, unsubscribe kind (no body) |
| sent_at | timestamptz | |
Unique `(transport, provider_message_id) WHERE provider_message_id IS NOT NULL` (instance-wide for SES and provider event lookup). Unique `(workspace_id, (metadata->>'idempotency_key')) WHERE kind IN ('changelog','feedback_update','portal_sign_in')` (contact mail idempotency). Index `(workspace_id, message_id)`, `(workspace_id, message_id_header)`.

### email_suppressions
| Column | Type | Notes |
|---|---|---|
| email | citext not null | |
| reason | text not null CHECK in ('hard_bounce','soft_bounce','complaint','unsubscribed','manual','invalid') | |
| scope | text not null CHECK in ('all','notifications','changelog','feedback') | `notifications`: widget offline mail only (one-click unsubscribe). `changelog` / `feedback`: complaint on that contact-mail kind (outbound.md 8); blocks only that kind, support mail still flows. Hard bounce always `all` |
| source | text not null CHECK in ('ses','provider','dsn','arf','one_click','link','api','manual') | `provider` = Postmark/Resend/Mailgun/SendGrid event webhook (`details.provider` names it) |
| details | jsonb | diagnostic code, feedback id (no body) |
| outbound_email_id | uuid null references outbound_emails on delete set null | |
| expires_at | timestamptz | soft bounce: 7 days; others null |
| created_by_member_id | uuid null | |
Unique `(workspace_id, email, scope)`. Index `(workspace_id, created_at desc)`.

### email_signatures
`id, workspace_id, workspace_member_id uuid not null references workspace_members on delete cascade, email_address_id uuid null references email_addresses on delete cascade, body text not null (Markdown, max 2,000 chars, links + bold/italic only), enabled boolean not null default true, created_at, updated_at`. Unique `(workspace_member_id, email_address_id) NULLS NOT DISTINCT`. Migration 20261201050001 allows null member (address signature) and adds `logo_attachment_id` ([sending.md](sending.md) 3).

### New tables

| Table | Spec | Migration |
|---|---|---|
| email_ignored_addresses | [sender-rules.md](sender-rules.md) 1 | 20261201050003 |
| email_trusted_senders | [sender-rules.md](sender-rules.md) 4 | 20261201050004 |
| email_address_tests | [setup.md](setup.md) 3 | 20261201060004 |
| email_mailbox_connections | [mailbox-connections.md](mailbox-connections.md) 2 | 20261201060002 |
| email_mailbox_messages | [mailbox-connections.md](mailbox-connections.md) 2 | 20261201060003 |

### email_sending_profiles
One per workspace, created lazily.
`workspace_id uuid primary key, status text CHECK in ('active','paused','review') default 'active', paused_at, paused_reason text, daily_limit int, hourly_limit int, ses_tenant_name text, created_at, updated_at`.

### email_sending_stats
`workspace_id, day date, stream text not null default 'customer' CHECK in ('customer','changelog','feedback'), sent int, delivered int, hard_bounced int, soft_bounced int, complained int, blocked int, created_at, updated_at`. Primary key `(workspace_id, day, stream)`. Substreams tracked separately (security.md): `customer` = conversation mail; `changelog` (13), `feedback` (14 `feedback_update` + `portal_sign_in`). Daily cap check sums `customer` + `feedback`; workspace reputation eval sums all three (shared domain reputation). Counters via `INSERT ... ON CONFLICT DO UPDATE SET x = x + 1`. Retention 400 days.

### Core table changes

No columns. New jsonb keys: `conversations.source.email_throttled` (`{at, sender}`, read by 03 row label), `conversations.source.email_spoof_risk`, `messages.metadata.email.{trusted, unverified_sender, spoof_risk, external_send}`. Uses existing columns:
- `channel_connections` (`type='email'`, `external_id`, `config`, `status`, `last_error`).
- `conversations.external_thread_key` = root Message-ID; `conversations.subject`; `conversations.source` (`{"channel":"email","to":"support@acme.com"}`).
- `messages.channel_message_id` = Message-ID; `messages.dedupe_key` = `email:<sha256(message-id)[:32]>`; `messages.body_html`; `messages.substantive`; `messages.delivery_status`; `messages.metadata.email` (shape in pipeline.md); `messages.metadata.auto_reply` (read by 07 gate rule 6).
- `attachments.inline`, `attachments.content_id`.
- `conversation_participants` role `cc`.
- `contact_identities` type `email`, `verified`.

### Migrations

`database/migrations/`:
- `20261101000401_create_email_domains_table.sql`
- `20261101000402_create_email_addresses_table.sql`
- `20261101000403_create_inbound_emails_table.sql`
- `20261101000404_create_email_message_refs_table.sql`
- `20261101000405_create_email_threads_table.sql`
- `20261101000406_create_outbound_emails_table.sql`
- `20261101000407_create_email_suppressions_table.sql`
- `20261101000408_create_email_signatures_table.sql`
- `20261101000409_create_email_sending_profiles_table.sql`
- `20261101000410_create_email_sending_stats_table.sql`
- `20261101000411_add_email_rls_policies.sql`

Suppression scopes `changelog`, `feedback` and contact-mail `kind` values ship in the create migrations above (04 unreleased); no separate add-scope migration.

Added this round (track ranges; each includes RLS where it creates a table):

- `20261201050001_alter_email_signatures_for_address_level.sql`
- `20261201050002_add_provider_fields_to_email_domains.sql`
- `20261201050003_create_email_ignored_addresses_table.sql`
- `20261201050004_create_email_trusted_senders_table.sql`
- `20261201060001_widen_email_source_and_transport.sql`
- `20261201060002_create_email_mailbox_connections_table.sql`
- `20261201060003_create_email_mailbox_messages_table.sql`
- `20261201060004_create_email_address_tests_table.sql`
- `20261201060005_add_setup_findings_to_email_addresses.sql`

## Implements 03 channel contract + foundation Mailer

03 owns `contracts.Channel` (`03-shared-inbox/channel-contract.md`, canonical per `00-foundation/contracts.md`). 04 implements it unchanged; no own signature.

```go
// app/integrations/email/channel.go: contracts.Channel for enums.ChannelTypeEmail
func (c *Channel) Type() enums.ChannelType { return enums.ChannelTypeEmail }
func (c *Channel) Capabilities(conn *models.ChannelConnection) contracts.ChannelCapabilities
func (c *Channel) Send(ctx context.Context, conn *models.ChannelConnection, msg contracts.OutboundMessage) (contracts.SendResult, error)
// no optional interfaces: not ChannelTyping, ChannelReadMarker
```

Email capability values (`Capabilities(conn)`, no network):

| Field | Value |
|---|---|
| Transport | `TransportPush` |
| StartConversation | `StartAlways` |
| Formatting | `FormatHTML` (Markdown to HTML + text twin) |
| MaxBodyChars | 0 (03 64 KB Markdown limit applies; rendered HTML cap 1 MB checked in Send) |
| Subject | true |
| Attachments | true; `MaxAttachments` 0 (global 10), `MaxAttachmentBytes` 0 (global 25 MB), `MaxTotalAttachmentBytes` 0 (over 20 MB total: largest become signed links, never rejected) |
| AllowedContentTypes | nil (executables sent as links, never attached) |
| Edits, Deletes, Typing, ReadReceipts | false |
| DeliveryReceipts | true when transport `ses` (SES events) or `postmark`, `resend`, `mailgun`, `sendgrid` (provider event webhooks, outbound.md 7.1); false for `smtp` relay, `log` (sent only; DSN bounce can still fail message) |
| ParticipantsCC | true |
| MessagingWindow | 0 |
| Outbound retry window | about 24 h (SMTP norms), declared by driver per `00-foundation/contracts.md` Channel section; `send_channel_message` reads it from driver via 03 `ChannelCapabilities.RetryWindow` (email driver sets 24 h) |

`OutboundMessage` fields used: `Subject`, `BodyHTML`, `BodyText`, `Recipient` (IdentityType `email`), `CC`, `Author` (display name, section outbound.md 2), `Attachments` (`Open` stream; `Inline` + `ContentID` for cid), `ReplyTo` (In-Reply-To/References), `ExternalThreadKey` (root Message-ID), `IdempotencyKey` (outbound_emails row unique per message, retry reuses stored Message-ID), `Options` (raw JSON `channel_options.email`: `{"to":[],"cc":[],"bcc":[],"from_address_id":"ema_..."}`; driver validates), `IsTest`.

`SendResult`: `ChannelMessageID` = our Message-ID header; `Status` = `sent` (delivered later via SES or provider event); `ExternalThreadKey` = our Message-ID when conversation had none (member-started email); `SentAt` = provider accept time; `Metadata` (stored `messages.metadata.email`, max 4 KB) = `{outbound_email_id, from, to, cc, transport, provider_message_id}` (bcc stays in `outbound_emails` only).

Error mapping (Send returns `*contracts.SendError` wrapping 03 sentinels; outbound.md section 6):
- `mail.ErrPermanent` (5xx, SES `MessageRejected`, provider 422 invalid recipient): invalid/unknown recipient `ErrRecipientUnreachable` (code `smtp_550` / `ses_message_rejected` / `provider_invalid_recipient`), else `contracts.ErrPermanent`.
- `mail.ErrTemporary`: `SendError{Err: nil}` (transient, 03 River retry).
- `*mail.RateLimitedError{RetryAfter}`: `*contracts.RateLimitedError{RetryAfter}` (snooze, no attempt used).
- Provider auth failure (SMTP 535, API 401/403 on credentials): instance misconfig, not workspace fault: `SendError{Err: nil}` (transient, retried in 24 h window) + health check `mail.customer_stream` red + instance admin alert (once per hour). Never `ErrConnectionBroken` (no workspace connection to blame).
- Provider rejects From as unverified at provider (edge 170): domain `degraded` reason `provider_rejected_sender`, `SendError{Err: nil}` so retry resolves default sender; default sender itself rejected: `ErrPermanent` `sender_not_verified`.
- Own checks: suppressed To `ErrRecipientUnreachable` (`recipient_suppressed`); paused `ErrPermanent` (`email_sending_paused`); missing sender `ErrPermanent` (`no_sender_available`); daily limit `*contracts.RateLimitedError` (`RetryAfter` = min(15 min, until next UTC day 00:05), repeats until reset).

Inbound: 04 calls 03 action `conversations.ReceiveInboundMessage` with 03 `InboundMessage` (mapping pipeline.md section 10). Delivery changes (SES events, DSN bounces) go through 03 `UpdateMessageDelivery`. Drivers never write conversations/messages/contacts directly.

Mailer: framework `mail.Mailer` (contracts.md Mailer section, framework.md `mail`): `Send(ctx, mail.Message) (mail.SendResult, error)`. 04 sends only on customer stream: `a.Mail.Mailer(channels.email.mailer)` (named instance, default `customer`; outbound.md section 15). Never default/system mailer, never `app/mail` Mailables for contacts. 04 builds full MIME itself (DKIM for `smtp` driver, headers) and passes `mail.Message.Raw`; `Tags` = provider tags (stream=customer, workspace, kind, message), `ConfigurationSet` = `ses.configuration_set` (`dimmi-customer`). Provider id from `mail.SendResult.ProviderMessageID`; rewritten Message-ID from `mail.SendResult.MessageID` when driver knows it. Errors via `errors.Is`: `mail.ErrPermanent`, `mail.ErrTemporary`, `errors.As` `*mail.RateLimitedError{RetryAfter}`. SES driver uses SESv2 `SendEmail` with `Content.Raw`, `ConfigurationSetName`, `EmailTags`, `TenantName` (cloud: `mail.Message.TenantName` = `email_sending_profiles.ses_tenant_name`). Raw-capable drivers: `ses`, `smtp`, `mailgun` (`messages.mime`), `log`. `postmark`, `resend`, `sendgrid` have no raw endpoint: framework driver decomposes `Raw` (go-message) into provider JSON, custom headers carried (Message-ID override support per provider [staging]).
- Inbound sources: framework `mailbox.Source` / `mailbox.Envelope` (framework.md 10.1 replaces former 04 `contracts.InboundMail`); 04 owns pipeline policy only.

## Code map

```
# Framework packages used (not 04 code; framework.md): mail (Mailer, mime.Builder, dkim.Signer, drivers ses/smtp/postmark/resend/mailgun/sendgrid/log),
# mailbox (Source, Envelope, parse on enmime/v2, authcheck, dsn, arf, driver ses, Fake), aws/sns (SNS verify),
# dom (Parse, ToMarkdown on html-to-markdown/v2, DecodeCharset), sanitize, dns (a.DNS), httpclient + safedial, ids, clock.
app/enums/
  email_domain_status.go, email_classification.go,
  email_auth_result.go, email_suppression_reason.go, email_suppression_scope.go, outbound_email_status.go,
  outbound_email_kind.go, contact_mail_kind.go, reply_to_mode.go
app/integrations/email/
  channel.go                           # contracts.Channel impl (Type, Capabilities, Send)
  composer.go                          # 04 header policy (section outbound.md 4) on framework mime.Builder: multipart/alternative, related, attachments
  templates.go                         # render templ views to HTML + text
  addressing.go                        # forwarding local parts, reply token address format/parse, reserved names
  threading.go                         # thread resolution order (pipeline.md section 8)
  subject.go                           # normalize subject (Re/Fwd prefixes in 20 languages, tags)
  classifier.go                        # auto-reply, list, bulk, noreply, system notice, loop
  bounces.go                           # map framework dsn/arf results to 04 actions (classification, suppression scope)
  quotes.go                            # quote + signature stripping (HTML DOM pass + text pass = port of GitHub email_reply_parser, MIT, attribution header)
  forwarded.go                         # inline forwarded block + message/rfc822 extraction
  formrelay.go                         # form service detection, Reply-To promotion
  htmlbody.go                          # email sanitize policy (sanitize.NewPolicy), cid rewrite, dom.ToMarkdown options, text extraction
  auth.go                              # trust + alignment policy over framework mailbox/authcheck results (SPF from trusted verdicts only)
  mailer.go                            # customer-stream mailer resolve (channels.email.mailer), tags, config set, tenant
  providerevents/                      # normalize provider event webhooks to one DeliveryEvent
    postmark.go, resend.go, mailgun.go, sendgrid.go
  gmail.go                             # forwarding confirmation detection
  limits.go                            # inbound sender rate, outbound per-recipient automated limit
  loop.go                              # X-Dimmi-Loop header sign/verify, own-address check
  dns/checker.go                       # compare + mistakes detection on a.DNS lookups
  dns/records.go                       # expected record generation per driver (ses, smtp, manual provider guidance)
app/services/ses/
  identities.go                        # adapter (sesv2): CreateEmailIdentity, GetEmailIdentity, PutEmailIdentityMailFromAttributes, tenants
app/actions/email/
  receive_inbound_email.go             # store raw + inbound_emails row + enqueue (fast path from every driver)
  process_inbound_email.go             # full pipeline (job body)
  reprocess_inbound_email.go
  ensure_default_email_address.go      # idempotent default forwarding address; called by workspace initializer hook (and `dimmi workspace:repair`)
  create_email_address.go, update_email_address.go, delete_email_address.go
  send_email_address_test.go           # outbound test + forwarding round trip test
  create_email_domain.go, check_email_domain.go, delete_email_domain.go, regenerate_email_domain_dkim.go
  send_email_message.go                # body of Channel.Send: resolve sender/recipients, compose MIME, Mailer.Send, outbound_emails + refs; returns SendResult (03 updates delivery)
  send_widget_offline_reply.go         # called by 01 SendWidgetOfflineReplyEmail job with rendered HTML/text; 04 adds sender, Reply-To token, List-Unsubscribe, suppression + limits, sends
  send_contact_mail.go                 # SendContactMail: one generic customer-stream entry for 13/14 (outbound.md 16)
  send_changelog_email.go, send_contact_notification.go # thin named wrappers over SendContactMail (13, 14)
  resolve_verified_sender.go           # ResolveVerifiedSender, HasVerifiedSendingDomain, IsSuppressed (read helpers, outbound.md 16)
  record_email_delivery.go, record_email_bounce.go, record_email_complaint.go
  record_mail_provider_event.go        # normalized provider event -> delivery/bounce/complaint actions
  suppress_email.go, unsuppress_email.go, unsubscribe_email.go
  erase_email_contact_data.go          # EraseEmailContactData: GDPR contact eraser (00-foundation/contracts.md "Contact eraser hooks"), registered on 03 eraser registry in app_service_provider.go
  evaluate_email_reputation.go, resume_email_sending.go, request_email_sending_review.go
  upsert_email_signature.go, delete_email_signature.go
  store_forwarding_confirmation.go
app/http/controllers/api/v1/
  email_domain_controller.go           # Index, Store, Show, Update, Destroy
  email_domain_check_controller.go     # Store (POST /email-domains/{id}/checks)
  email_domain_dkim_rotation_controller.go # Store (POST /email-domains/{id}/dkim-rotations)
  email_address_controller.go          # Index, Store, Show, Update, Destroy
  email_address_test_controller.go     # Store (POST /email-addresses/{id}/tests)
  email_address_inbound_email_controller.go # Index (activity log per address)
  inbound_email_controller.go          # Show
  inbound_email_raw_controller.go      # Show (signed .eml redirect)
  inbound_email_processing_run_controller.go # Store (POST /inbound-emails/{id}/processing-runs)
  message_email_controller.go          # Show (GET /messages/{id}/email: headers, auth, original html, outbound statuses)
  email_suppression_controller.go      # Index, Store, Destroy
  email_signature_controller.go        # Index, Store, Update, Destroy
  email_sending_status_controller.go   # Show, Update (field-dispatch: `status` to ResumeEmailSending / RequestEmailSendingReview)
app/http/controllers/web/
  email_unsubscription_controller.go   # Show (GET page), Store (POST one-click RFC 8058)
app/http/controllers/webhooks/
  ses_inbound_controller.go            # SNS: receipt notifications
  ses_event_controller.go              # SNS: bounce, complaint, delivery, delivery delay
  mail_provider_event_controller.go    # Store per provider: postmark, resend, mailgun, sendgrid outbound events
app/http/requests/
  store_email_domain_request.go, store_email_address_request.go, update_email_address_request.go,
  store_email_address_test_request.go, store_email_suppression_request.go,
  update_email_domain_request.go, update_email_sending_status_request.go,
  store_email_signature_request.go, update_email_signature_request.go
app/http/resources/
  email_domain_resource.go, email_dns_record_resource.go, email_address_resource.go,
  inbound_email_resource.go, message_email_resource.go, outbound_email_resource.go,
  email_suppression_resource.go, email_signature_resource.go, email_sending_status_resource.go
app/policies/
  email_domain_policy.go, email_address_policy.go, inbound_email_policy.go,
  email_suppression_policy.go, email_signature_policy.go
app/jobs/
  process_inbound_email.go,
  process_ses_notification.go, process_mail_provider_event.go, check_email_domain.go, schedule_email_domain_checks.go,
  recheck_verified_email_domains.go, prune_inbound_email_raw.go,
  expire_email_suppressions.go, evaluate_email_reputation.go
app/listeners/
  revoke_conversation_email_threads.go # conversation.deleted
app/events/
  email_domain_status_changed.go, email_address_forwarding_confirmation_received.go,
  email_suppression_created.go, email_suppression_deleted.go, email_sending_paused.go, email_sending_resumed.go
app/mail/
  conversation_reply_mail.go           # teammate/AI reply (not foundation Mailable queue: built by composer)
  email_address_test_mail.go           # (widget offline mailable + template owned by 01)
app/notifications/
  email_domain_verified_notification.go, email_domain_degraded_notification.go,
  email_domain_failed_notification.go, email_sending_paused_notification.go
app/mcp/tools/
  list_email_addresses.go, get_email_domain.go, create_email_domain.go, check_email_domain.go,
  get_message_email.go
app/console/commands/
  email_doctor_command.go              # dimmi email:doctor <domain>
  email_test_inbound_command.go        # dimmi email:test-inbound <file.eml> --address
  email_reprocess_command.go           # dimmi email:reprocess <ibm_id>
  email_suppressions_command.go        # dimmi email:suppressions list|remove
config/channels.go                     # channels.email.* keys (section below)
resources/views/emails/
  conversation_reply.templ, conversation_reply_text.templ,
  email_address_test.templ, partials/quote.templ, partials/signature.templ
resources/views/web/email_unsubscribe.templ
resources/email/subject_prefixes.yaml, quote_headers.yaml, autoreply_subjects.yaml, form_relays.yaml, noreply_patterns.yaml
lang/en/email.json
routes/api.go, routes/web.go, routes/webhooks.go, routes/console.go, routes/mcp.go  # entries added
database/queries/
  email_domains.sql, email_addresses.sql, inbound_emails.sql, email_message_refs.sql,
  email_threads.sql, outbound_emails.sql, email_suppressions.sql, email_signatures.sql,
  email_sending_profiles.sql, email_sending_stats.sql
database/factories/
  email_domain_factory.go, email_address_factory.go, inbound_email_factory.go, outbound_email_factory.go,
  email_suppression_factory.go, email_signature_factory.go
tests/fixtures/email/                  # .eml corpus: clients x locales, OOO, DSN, ARF, lists, forwards, charsets, malformed
# inbound driver contract suite = framework/mailbox/drivertest (framework-owned; ses + Fake)
tests/unit/email/                      # pure parsing/mapping of Channel driver (testing.md: domain driver zone), table-driven, fixtures, no mocks
  threading_test.go, subject_test.go, classifier_test.go, quotes_test.go, forwarded_test.go,
  formrelay_test.go, bounces_map_test.go, auth_test.go, htmlbody_test.go, composer_test.go,
  addressing_test.go, dns_records_test.go, dns_checker_test.go, loop_test.go,
  # parser fuzz, DSN/ARF parse, SNS verify tests live in framework mailbox/parse, mailbox/dsn, mailbox/arf, aws/sns
  provider_events_test.go (normalization per provider; signature checks in feature tests)
tests/feature/email/
  email_domains_test.go, email_addresses_test.go, inbound_webhooks_test.go, inbound_pipeline_test.go,
  outbound_reply_test.go, widget_offline_test.go, bounces_test.go, suppressions_test.go,
  signatures_test.go, unsubscribe_test.go,
  reputation_test.go, provider_drivers_test.go (customer stream per driver, `httpclienttest.New(tc.App.HTTP)`), contact_mail_test.go (kinds, scopes, idempotency, wrappers, feedback hook),
  mcp_email_tools_test.go
tests/arch/email_streams_test.go         # app/mail Mailables never addressed to contacts; only app/actions/email resolves customer mailer
```

### Files added this round

```
framework/mailbox/drivers/{postmark,mailgun,sendgrid,resend,cloudflare,gmail,graph}/   # sources; framework words only
framework/mailbox/imap/                # thin go-imap v2 client wrapper
framework/mail/domains.go              # optional DomainManager; drivers postmark, resend, mailgun, sendgrid implement
app/integrations/email/
  setupfindings.go, setupguides/guides.yaml, dns/hosts.yaml, dns/domainconnect.go, sender_rules.go,
  cloudflareworker/{worker.js,wrangler.toml}, conversation_reply_branded.templ
app/actions/email/
  connect_email_mailbox.go, complete_email_mailbox_oauth.go, disconnect_email_mailbox.go, sync_email_mailbox.go,
  renew_email_mailbox_watches.go, send_through_mailbox.go, record_mailbox_sent_message.go,
  start_email_address_test.go, complete_email_address_test.go, send_email_setup_instructions.go,
  create_ignored_email_address.go, create_trusted_email_sender.go, dismiss_email_throttle.go,
  set_default_email_address.go, start_email_domain_dns_setup.go, run_email_round_trip.go
app/http/controllers/api/v1/
  email_mailbox_connection_controller.go, email_mailbox_connection_check_controller.go, email_address_test_controller.go,
  email_address_instruction_email_controller.go, email_domain_instruction_email_controller.go, email_domain_dns_setup_controller.go,
  email_ignored_address_controller.go, email_trusted_sender_controller.go, conversation_email_throttle_controller.go,
  conversation_email_sender_options_controller.go, email_signature_rendering_controller.go,
  email_template_rendering_controller.go, email_capabilities_controller.go
app/http/controllers/web/
  email_mailbox_callback_controller.go, email_setup_share_controller.go
app/http/controllers/webhooks/
  mailbox_controller.go, google_mailbox_controller.go, microsoft_mailbox_controller.go
routes/email_senders.go, routes/email_mailboxes.go
database/queries/email_ignored_addresses.sql, email_trusted_senders.sql, email_address_tests.sql,
  email_mailbox_connections.sql, email_mailbox_messages.sql
resources/js/features/email/
  domains/, senders/, suppressions/, setup/, mailboxes/, sending/, signature-editor.tsx, sender-select.tsx,
  sender-flag.tsx, template-settings.tsx
resources/js/lib/api/email-domains.ts, email-senders.ts, email-mailboxes.ts
tests/e2e/email-domains.spec.ts, email-senders.spec.ts, email-setup.spec.ts, email-signatures.spec.ts,
  email-mailboxes.spec.ts, email-sending.spec.ts
tests/emailmatrix/                      # tag emailmatrix; staging.md 5.3
deploy/tofu/, deploy/domainconnect/     # staging.md 7
```

## HTTP API

All under `/api/v1`, guard `auth` (`A`: session, first-party app or token; abilities apply to tokens), `workspace`, `throttle:api`. 404 for other workspace IDs.

| Method | Path | Route name | Permission (Permissions matrix) | Purpose |
|---|---|---|---|---|
| GET | /email-domains | email-domains.index | `email:read` | list domains with status |
| POST | /email-domains | email-domains.store | `email:manage` | add domain, generate records, start SES identity |
| GET | /email-domains/{domain} | email-domains.show | `email:read` | records + live statuses + warnings |
| PATCH | /email-domains/{domain} | email-domains.update | `email:manage` | change `mail_from_subdomain` (edge 128; re-calls SES MAIL FROM), re-enters `verifying` for return-path records only |
| DELETE | /email-domains/{domain} | email-domains.destroy | `email:manage` | remove; addresses fall back to default sender |
| POST | /email-domains/{domain}/checks | email-domains.checks.store | `email:manage` | check DNS now (rate 1 per 10 s per domain), enter `verifying`; 202, result lands on domain (not stored as row) |
| POST | /email-domains/{domain}/dkim-rotations | email-domains.dkim-rotations.store | `email:rotate_dkim` | smtp driver: new selector key, old kept 7 days |
| GET | /email-addresses | email-addresses.index | `email:read` | list (default address already created at workspace creation) |
| POST | /email-addresses | email-addresses.store | `email:manage` | create forwarding address (creates channel_connection) |
| GET | /email-addresses/{address} | email-addresses.show | `email:read` | detail, setup steps per provider, forwarding confirmation |
| PATCH | /email-addresses/{address} | email-addresses.update | `email:manage` | display name, inbox, modes, settings |
| DELETE | /email-addresses/{address} | email-addresses.destroy | `email:manage` | disconnect (soft delete connection) |
| POST | /email-addresses/{address}/tests | email-addresses.tests.store | `email:manage` | send outbound test and/or forwarding round trip; sync outcome resource `email_address_test` (not stored) |
| GET | /email-addresses/{address}/inbound-emails | email-addresses.inbound-emails.index | `email:read_raw` | activity log incl. ignored mail, `filter[status]`, `filter[classification]` |
| GET | /inbound-emails/{inbound_email} | inbound-emails.show | `conversations:read` + 03 `VisibleScope` contains conversation; unrouted: `email:read_raw` | headers, auth, classification, outcome |
| GET | /inbound-emails/{inbound_email}/raw | inbound-emails.raw.show | `email:read_raw` | 302 to signed `.eml` URL (5 min) |
| POST | /inbound-emails/{inbound_email}/processing-runs | inbound-emails.processing-runs.store | `email:manage` | rerun pipeline (failed or ignored); 202 `{object: "inbound_email_processing_run", status: "queued"}` |
| GET | /messages/{message}/email | messages.email.show | `conversations:read` + `VisibleScope` | original headers, auth verdict, sanitized full HTML, outbound recipient statuses |
| GET | /email-suppressions | email-suppressions.index | `contacts:read` | `filter[email]`, `filter[reason]` |
| POST | /email-suppressions | email-suppressions.store | `email_suppressions:manage` | manual suppress |
| DELETE | /email-suppressions/{suppression} | email-suppressions.destroy | `email_suppressions:manage` | lift (complaint lift needs `email_suppressions:lift_complaint` + confirm flag) |
| GET | /email-signatures | email-signatures.index | own: `email_signatures:manage_own`; all: `email_signatures:manage_any` | |
| POST | /email-signatures | email-signatures.store | own: `email_signatures:manage_own`; for other member: `email_signatures:manage_any` | |
| PATCH | /email-signatures/{signature} | email-signatures.update | own: `email_signatures:manage_own`; other: `email_signatures:manage_any` | |
| DELETE | /email-signatures/{signature} | email-signatures.destroy | own: `email_signatures:manage_own`; other: `email_signatures:manage_any` | |
| GET | /email-sending-status | email-sending-status.show | `email:read_raw` | limits, usage today, bounce/complaint rates, paused state; `streams` object per substream (`changelog`, `feedback`: sent today, rates) |
| PATCH | /email-sending-status | email-sending-status.update | `email:resume_sending` | `{status: "active"}` resumes after pause (self-host: `paused -> active`). Cloud: owner sends `{status: "review"}` (`paused -> review`, notifies ops); ops clears `review -> active`. Other transitions 422 `invalid_transition`. |

Web (no `/api`): `GET /email-unsubscriptions/{token}` (`email-unsubscriptions.show`, `signed`), `POST /email-unsubscriptions/{token}` (`email-unsubscriptions.store`, `signed`, no CSRF, RFC 8058 `List-Unsubscribe=One-Click` body). Noun path per rest.md rule 1 (route lint denylists `unsubscribe`); same shape as 06 `/notification-unsubscriptions`.

Webhooks (`routes/webhooks.go`, no session, body limit 256 KB for SNS; SES raw read from S3, never in body):

| Method | Path | Route name | Verification |
|---|---|---|---|
| POST | /webhooks/ses/inbound | webhooks.ses.inbound | SNS signature (v1 SHA1 / v2 SHA256), cert URL host `sns.<region>.amazonaws.com` https, TopicArn equals `channels.email.ses.inbound_topic_arn` |
| POST | /webhooks/ses/events | webhooks.ses.events | same, TopicArn equals `channels.email.ses.events_topic_arn` |
| POST | /webhooks/postmark/events | webhooks.postmark.events | HTTP basic auth (`postmark` / `channels.email.postmark.events_webhook_password`), constant-time; Postmark has no signature |
| POST | /webhooks/resend/events | webhooks.resend.events | Svix headers (`svix-id`, `svix-timestamp`, `svix-signature`), HMAC-SHA256 with `channels.email.resend.webhook_secret`, 5 min window, `svix-id` replay check Valkey `SET NX EX 600` (`email:replay:resend:{svix-id}`) |
| POST | /webhooks/mailgun/events | webhooks.mailgun.events | `signature.{timestamp,token,signature}` HMAC-SHA256 with `channels.email.mailgun.signing_key`, 5 min window, token replay check Valkey `SET NX EX 600` key `email:replay:mailgun:{token}` |
| POST | /webhooks/sendgrid/events | webhooks.sendgrid.events | Signed Event Webhook: ECDSA P-256 over `X-Twilio-Email-Event-Webhook-Timestamp` + raw body, public key `channels.email.sendgrid.events_public_key`, 5 min window |

Valkey state (PO rule: Valkey required in every deployment; holds only rebuildable state; durable rows stay in Postgres):
- Replay/dedupe keys above (`SET NX EX`), SNS message dedupe (24 h).
- Rate limits via `a.Limiter` (Valkey driver): inbound `sender_rate`, `sender_quarantine`, `limits.automated_per_recipient_hourly`, `limits.unverified_relay_per_address_daily`, `domain_check_throttled`, `throttle:webhooks`.
- Not in Valkey: `inbound_emails` (raw + audit), `outbound_emails`, suppressions, `email_sending_stats` (reputation evidence), daily cap counters (computed from stats rows: billing-adjacent, must survive restart).
- Valkey unavailable: webhooks return 503 (providers retry; fail closed on replay check, never process unverified duplicates twice); inbound jobs already queued continue (River in Postgres); limiter errors treat as allowed for member replies, denied for automated mail (edge 191).

Provider event routes registered only when that driver backs customer stream (else 404 `inbound_driver_disabled` shape). Body limit 1 MB. Bad signature: 401, metric `dimmi_email_provider_event_rejected_total{provider}`, nothing written (edge 148).

Reply-endpoint extension (03 owns `POST /conversations/{id}/replies` and `POST /conversations`): 04 asks 03 to accept optional `email` object, validated by 04 rule `rules.EmailRecipients`:

```json
{ "body": "Thanks Bob, fixed.", "email": { "to": ["bob@customer.io"], "cc": ["ana@customer.io"], "bcc": [], "from_address_id": "ema_01jc..." } }
```
Omitted: To = conversation contact, Cc = current `cc` participants, From = conversation's email address (or default). Max 50 recipients total. Suppressed recipient: 422 `recipient_suppressed` with `errors["email.to.0"]`.

### Routes added this round

Same guards. `U` = session only.

| Method | Path | Route name | Permission | Purpose |
|---|---|---|---|---|
| GET | /workspace/email-capabilities | workspace.email-capabilities.show | `email:read` | which OAuth, IMAP, inbound sources this instance offers |
| POST | /email-domains/{domain}/dns-setups | email-domains.dns-setups.store | `email:manage` | signed Domain Connect apply URL |
| POST | /email-domains/{domain}/instruction-emails | email-domains.instruction-emails.store | `email:manage` | mail records to DNS manager, 202 |
| GET | /email-addresses/{address}/tests/{test} | email-addresses.tests.show | `email:read` | test status (poll fallback) |
| POST | /email-addresses/{address}/instruction-emails | email-addresses.instruction-emails.store | `email:manage` | mail guide to IT admin, 202 |
| POST | /email-addresses/{address}/template-renderings | email-addresses.template-renderings.store | `email:manage` | preview style |
| GET | /conversations/{conversation}/email-sender-options | conversations.email-sender-options.show | `conversations:read` + `VisibleScope` | From picker |
| DELETE | /conversations/{conversation}/email-throttle | conversations.email-throttle.destroy | `conversations:write` + `VisibleScope` | dismiss label |
| POST | /email-signatures/renderings | email-signatures.renderings.store | `email_signatures:manage_own` | preview |
| GET, POST | /email-ignored-addresses | email-ignored-addresses.index, .store | `email:read`, `email:manage` | list, add |
| DELETE | /email-ignored-addresses/{ignored_address} | email-ignored-addresses.destroy | `email:manage` | remove |
| GET, POST | /email-trusted-senders | email-trusted-senders.index, .store | `email:read`, `email:manage` | list, add (`release`) |
| DELETE | /email-trusted-senders/{trusted_sender} | email-trusted-senders.destroy | `email:manage` | remove |
| GET | /email-mailbox-connections | email-mailbox-connections.index | `email:read` | list |
| POST | /email-mailbox-connections | email-mailbox-connections.store | `email:manage`, `U` for OAuth | OAuth URL or IMAP connect |
| GET, PATCH, DELETE | /email-mailbox-connections/{connection} | .show, .update, .destroy | `email:read`, `email:manage` | detail, settings, disconnect |
| POST | /email-mailbox-connections/{connection}/checks | email-mailbox-connections.checks.store | `email:manage` | sync now, 202 |

`POST /email-addresses/{address}/tests` now returns stored test (201, `status: waiting`), [setup.md](setup.md) 3. `PATCH /email-addresses/{address}` accepts `is_default` and new settings. `GET /email-suppressions` adds `filter[contact]`.

Web (no `/api`): `GET /channels/email/google/callback`, `GET /channels/email/microsoft/callback`, `GET /email-setup/{address}`, `POST /email-setup/{address}/tests` (signed).

Webhooks added: `POST /webhooks/postmark/inbound`, `/webhooks/mailgun/inbound/mime`, `/webhooks/sendgrid/inbound`, `/webhooks/resend/inbound`, `/webhooks/cloudflare/inbound` ([sources.md](sources.md) 1), `/webhooks/google/mailbox`, `/webhooks/microsoft/mailbox`, `/webhooks/microsoft/mailbox-lifecycle` ([mailbox-connections.md](mailbox-connections.md) 5). Body limit per source, not 256 KB.

### Examples

`POST /api/v1/email-domains`
```json
{ "domain": "acme.com", "mail_from_subdomain": "bounce" }
```
201:
```json
{
  "data": {
    "id": "edm_01jcf3...",
    "object": "email_domain",
    "domain": "acme.com",
    "status": "pending",
    "driver": "ses",
    "records": [
      { "purpose": "ownership", "type": "TXT", "name": "_dimmi-verification.acme.com", "host": "_dimmi-verification", "value": "dimmi-verification=q3m8x0v7k2c5r9t1w4y6z8a2", "required": true, "status": "missing", "observed": [] },
      { "purpose": "dkim", "type": "CNAME", "name": "4f7k2...._domainkey.acme.com", "host": "4f7k2...._domainkey", "value": "4f7k2....dkim.amazonses.com", "required": true, "status": "missing", "observed": [] },
      { "purpose": "dkim", "type": "CNAME", "name": "b8a1c...._domainkey.acme.com", "host": "b8a1c...._domainkey", "value": "b8a1c....dkim.amazonses.com", "required": true, "status": "missing", "observed": [] },
      { "purpose": "dkim", "type": "CNAME", "name": "z0e9d...._domainkey.acme.com", "host": "z0e9d...._domainkey", "value": "z0e9d....dkim.amazonses.com", "required": true, "status": "missing", "observed": [] },
      { "purpose": "return_path", "type": "MX", "name": "bounce.acme.com", "host": "bounce", "value": "feedback-smtp.us-east-1.amazonses.com", "priority": 10, "required": false, "status": "missing", "observed": [] },
      { "purpose": "return_path_spf", "type": "TXT", "name": "bounce.acme.com", "host": "bounce", "value": "v=spf1 include:amazonses.com ~all", "required": false, "status": "missing", "observed": [] },
      { "purpose": "dmarc", "type": "TXT", "name": "_dmarc.acme.com", "host": "_dmarc", "value": "v=DMARC1; p=none;", "required": false, "status": "found", "observed": ["v=DMARC1; p=quarantine; rua=mailto:d@acme.com"], "note": "Existing DMARC policy kept. Do not add second record." }
    ],
    "warnings": [],
    "provider_hint": "cloudflare",
    "last_checked_at": null,
    "next_check_at": null,
    "verified_at": null,
    "created_at": "2026-11-01T10:00:00Z",
    "updated_at": "2026-11-01T10:00:00Z",
    "links": { "self": "https://app.trydimmi.com/api/v1/email-domains/edm_01jcf3...", "checks": "https://app.trydimmi.com/api/v1/email-domains/edm_01jcf3.../checks", "zone_file": "https://app.trydimmi.com/api/v1/email-domains/edm_01jcf3...?format=bind" }
  }
}
```
`?format=bind` returns `text/plain` BIND zone snippet (import into Cloudflare/Route 53).

`POST /api/v1/email-addresses`
```json
{ "address": "support@acme.com", "display_name": "Acme Support", "inbox_id": "inbox_01jc..." }
```
Returns `forwarding_address` and `setup_guides` (gmail, google_workspace, microsoft_365, fastmail, icloud, zoho, cpanel, other).

201:
```json
{
  "data": {
    "id": "ema_01jcf9...",
    "object": "email_address",
    "address": "support@acme.com",
    "display_name": "Acme Support",
    "channel_connection": { "id": "chn_01jcf9...", "object": "channel_connection", "status": "active" },
    "inbox": { "id": "inbox_01jc...", "object": "inbox", "name": "Support" },
    "forwarding_address": "acme.support@inbound.trydimmi.com",
    "reply_to_mode": "token",
    "reply_to_preview": "support+k7f3x9q2mz8b4c1d@acme.com",
    "sending_identity": { "from": "Sam from Acme Support <support@acme.com>", "domain_status": "verified", "uses_default_sender": false, "provider": "ses" },
    "from_name_style": "member_and_name",
    "is_default": false,
    "settings": { "quote_previous": true, "subject_tag": false, "bulk_mail_handling": "close" },
    "forwarding_confirmation": null,
    "inbound_verified_at": null,
    "last_inbound_at": null,
    "last_error": null,
    "created_at": "2026-11-01T10:00:00Z",
    "updated_at": "2026-11-01T10:00:00Z"
  }
}
```
`sending_identity.provider` = instance customer-stream driver (read-only; workspaces never set it).

`GET /api/v1/messages/{message}/email` 200:
```json
{
  "data": {
    "object": "message_email",
    "message": { "id": "msg_...", "object": "message" },
    "direction": "inbound",
    "message_id_header": "CAF=x7Q@mail.gmail.com",
    "in_reply_to": "msg_01jc.k2m9@acme.com",
    "from": { "name": "Bob", "email": "bob@customer.io" },
    "to": ["support@acme.com"],
    "cc": ["ana@customer.io"],
    "subject": "Re: Refund for order 1042",
    "auth": { "result": "pass", "spf": "pass", "dkim": "pass", "dmarc": "pass", "aligned_domain": "customer.io", "source": "ses" },
    "classification": "normal",
    "stripped": { "quote": true, "signature": true },
    "html": "<div>...sanitized full html incl. <div data-dimmi-quoted>...</div>...</div>",
    "headers": [{ "name": "Received", "value": "..." }],
    "inbound_email": { "id": "ibm_...", "object": "inbound_email" },
    "outbound": null
  }
}
```

### Error codes

| Status | code | When |
|---|---|---|
| 409 | `domain_claimed` | domain verified by other workspace (ownership TXT path explained in message) |
| 409 | `address_in_use` | address already connected in same workspace |
| 409 | `domain_in_use` | delete verified domain while addresses send from it: returns list; `?force=true` falls back addresses to default sender |
| 422 | `invalid_domain` | public suffix, IP literal, reserved (`trydimmi.com`, inbound domain), over 253 chars |
| 422 | `inbound_not_configured` | self-host without `inbound_domain` and no mailbox connection requested: forwarding-only address cannot be created |
| 422 | `recipient_suppressed` | reply to suppressed address |
| 422 | `no_sender_available` | self-host without `mail.from`, or customer-stream mailer missing / `log` in production, and no verified address |
| 423 | `email_sending_paused` | workspace sending paused (reputation) |
| 429 | `domain_check_throttled` | check more than once per 10 s |
| 429 | `entitlement_limit_reached` | entitlement cap (`key` `email.verified_daily` / `email.unverified_daily`; contracts.md shared code); `Retry-After` until UTC midnight |
| 410 | `raw_email_purged` | raw past retention |
| 404 | `inbound_driver_disabled` | test, inbound or provider event webhook for driver not enabled (404, not 501: conventions status list; hides enabled drivers) |
| 422 | `default_address_required` | `is_default: false` on current default |
| 422 | `invalid_pattern`, `pattern_conflict`, `ignored_address_limit`, `trusted_sender_limit` | ignored and trusted lists |
| 422 | `mailbox_address_mismatch`, `imap_auth_failed`, `imap_tls_required`, `imap_host_blocked`, `imap_unreachable` | mailbox connect |
| 404 | `oauth_provider_disabled` | OAuth client not configured or gated off |
| 409 | `mailbox_reauth_required` | sync or send on broken connection |
| 429 | `email_test_throttled`, `instructions_throttled` | [setup.md](setup.md) 3, 5 |
| 422 | `outbound_content_blocked` | outbound.md 13 content safety on default sender |
| 429 | `outbound_fair_use_exceeded` | 12 InTx guard on `conversation.created` (new member-initiated email conversation, cloud); 04 only surfaces |

## MCP tools

| Tool | Ability | Purpose |
|---|---|---|
| `dimmi_list_email_addresses` | settings:read | addresses, forwarding addresses, status, last inbound |
| `dimmi_get_email_domain` | settings:read | DNS records with live status (agent can set records through user's DNS MCP) |
| `dimmi_create_email_domain` | settings:write | add domain; returns records |
| `dimmi_check_email_domain` | settings:write | run check now (`POST /email-domains/{id}/checks`, then waits on domain up to 10 s); returns per-record status + plain-language fix list |
| `dimmi_get_message_email` | conversations:read | original headers, auth verdict for message |
| `dimmi_list_email_mailbox_connections` | settings:read | connections, status, last synced |
| `dimmi_start_email_address_test` | settings:write | start test, wait up to 10 s, return status and findings |

Registered in 09 toolset `email` (`routes/mcp.go`), not in default core list; header/subject output wrapped in 09 untrusted envelope (attacker-controlled text).

## CLI commands

| Command | Does |
|---|---|
| `dimmi email:doctor <domain>` | live DNS diagnosis (MX for inbound domain, SPF, DKIM, DMARC, provider records for configured driver) plus customer-stream mailer check (driver, credentials reachable, event webhook registered); warns on `smtp` relay at port 25 without AUTH (looks like direct-to-MX, edge 169) |
| `dimmi email:test-inbound <file.eml> --address <addr> [--dry-run]` | feeds `.eml` through pipeline (dry run prints classification, thread decision, stripped body) |
| `dimmi email:reprocess <ibm_id>` | reprocess inbound email |
| `dimmi email:suppressions list|remove <email>` | manage suppression list |
| `dimmi email:cloudflare-worker` | print worker + wrangler config for Cloudflare source |
| `dimmi email:resend-backfill --since <time>` | re-feed received mail held at Resend |
| `dimmi email:round-trip [--path <p>]` | run round-trip check now ([staging.md](staging.md) 6) |
| `dimmi email:mailboxes list|sync <emc_id>` | list connections, force sync |

`dimmi channels:add email` (05 owns generic command) delegates to 04 actions for forwarding setup prompts.

## Events

### Emitted (catalog names)

| Event | When |
|---|---|
| conversation.created, message.created, conversation.reopened, conversation.unsnoozed, contact.created, contact.updated | through 03 `conversations.ReceiveInboundMessage` on inbound |
| message.delivery_updated | outbound status change (sent, delivered, failed) |
| channel.connected / channel.disconnected | address created, deleted (no `channel.errored` for email: outbound provider faults are instance health, not channel errors) |

### New events (add to catalog)

| Event | Realtime | Webhook | Payload notes |
|---|---|---|---|
| email_domain.status_changed | yes (`workspace:{ws}`, admins) | yes | domain id, from, to, failure_reason |
| email_address.forwarding_confirmation_received | yes (`workspace:{ws}`) | no | address id, provider; code only in REST resource (admin) |
| email_suppression.created | yes | yes | email, reason, scope, source, `outbound_email_id`, `outbound_kind` (13/14 listeners sync subscriber/voter state by kind) |
| email_suppression.deleted | yes | yes | email, actor |
| email_sending.paused | yes | yes | reason, rates |
| email_sending.resumed | yes | yes | actor |
| email_domain.records_updated | yes (`workspace:{ws}`, admins) | no | domain id; page re-reads |
| email_address_test.updated | yes (`workspace:{ws}`) | no | test id, status |
| email_address.setup_findings_changed | yes (`workspace:{ws}`) | no | address id |
| email_mailbox_connection.status_changed | yes (`workspace:{ws}`, admins) | yes | connection id, from, to, reason code |

`email_suppression.created` and `.deleted` also go realtime now on `workspace:{ws}` ([sender-rules.md](sender-rules.md) 2).

### Consumed

| Event | Listener | Queue |
|---|---|---|
| message.created (type message, author member/agent/system, conversation channel email) | none in 04: 03 `SendChannelOutbound` inserts `send_channel_message` (queue mail), worker calls email `Channel.Send` | mail |
| message.created (type message, author member/agent, conversation channel widget) | none in 04: 01 `ScheduleWidgetOfflineEmail` listener + `SendWidgetOfflineReplyEmail` job call 04 action `SendWidgetOfflineReply` | mail |
| conversation.deleted | RevokeConversationEmailThreads | default |
| contact.merged | none: threads keyed by conversation, suppressions by address | |

## Jobs

| Job | Queue | Trigger | Retries | Uniqueness |
|---|---|---|---|---|
| ProcessInboundEmail | channels | ReceiveInboundEmail | 8, backoff 5 s to 30 min; final: status failed, admin notified once per hour per address | by `inbound_email_id` |
| ProcessSesNotification | mail | ses_event_controller (after Valkey `SET NX EX 86400` `email:sns:{MessageId}`: SNS redelivery acked without enqueue) | 5 | by SNS `MessageId` (backstop) |
| ProcessMailProviderEvent | mail | mail_provider_event_controller (postmark, resend, mailgun, sendgrid) | 5; unknown provider id retried 3x 30 s before ignore (edge 173) | by `(provider, event id)` |
| CheckEmailDomain | default | ScheduleEmailDomainChecks, `POST checks` endpoint | 3 | by `email_domain_id` |
| ScheduleEmailDomainChecks | maintenance | periodic 1 min | 0 | periodic |
| RecheckVerifiedEmailDomains | maintenance | periodic daily 03:17 UTC, spreads checks over 6 h | 0 | periodic |
| PruneInboundEmailRaw | maintenance | periodic daily | 3 | periodic; per row `RetentionGuard.MayDestroy(ws, {inbound_email, id})` before raw delete / `raw_deleted_at` mark (held: raw kept, `retention_held`, `dimmi_retention_held_total{kind="inbound_email"}`); suspended-workspace rows older than 30 days purged same way |
| ExpireEmailSuppressions | maintenance | periodic hourly | 3 | periodic |
| EvaluateEmailReputation | billing | periodic 15 min (cloud) | 3 | periodic |
| ReprocessSuspendedInboundEmails | channels | periodic hourly | 0 | periodic; re-enqueues `received` rows of unsuspended workspaces (30-day window) |
| ScheduleEmailMailboxSyncs | maintenance | periodic 1 min | 0 | periodic |
| SyncEmailMailbox | channels | schedule, push wake, send through mailbox, `POST checks` | 5 | by connection |
| RenewEmailMailboxWatches | maintenance | periodic daily, 6 h spread | 3 | periodic |
| PruneEmailMailboxMessages, PruneEmailAddressTests | maintenance | periodic daily | 3 | periodic |
| CheckEmailAddressTest | default | test + 10 min | 3 | by test |
| CheckEmailSetupHealth | maintenance | periodic daily (`not_receiving` finding) | 3 | periodic |
| RunEmailRoundTrip | maintenance | periodic hourly when enabled | 0 | periodic |
| DeleteSesRawObject | maintenance | `EraseEmailContactData` fallback when inline SES `DeleteObject` fails transiently (eraser returns error until done) | 5 | by `inbound_email_id` |

Retention rule (`00-foundation/contracts.md` `RetentionGuard`, extensibility.md section 8): `PruneInboundEmailRaw` guarded per row (above). SES bucket lifecycle (31 days on `raw/`) is outside Dimmi: when a non-noop guard driver ships, lifecycle rule moves to tag-based expiry (`dimmi-hold=false` tag) so held objects survive; noted in 11 ops runbook. `EraseEmailContactData` and `DeleteSesRawObject` run inside 03 `EraseContact` (guarded there). `ExpireEmailSuppressions` not guarded (suppression = sending protection, not customer content). Feature test: fake guard false keeps raw.

## Config keys

`config/channels.go`, prefix `channels.email.`, env `DIMMI_CHANNELS_EMAIL_*`.

| Key | Default self-host | Default cloud | Notes |
|---|---|---|---|
| inbound_domain | "" | inbound.trydimmi.com | empty: no inbound email (outbound only) |
| default_sender_domain | = inbound_domain | inbound.trydimmi.com | must have SPF/DKIM/DMARC set by operator at instance provider; `dimmi doctor` checks |
| mailer | customer | customer | named `mail.mailers.<name>` instance for customer-facing stream (outbound.md 15); undefined name: inherits `mail.default` mailer with stream overrides below. Boot fails if it resolves to `log` in production |
| provider_verified_domains | [] | [] | manual drivers (`postmark`, `resend`, `mailgun`, `sendgrid`): domains operator set up at provider; parent entry covers subdomains (outbound.md 11.1) |
| postmark.message_stream | outbound-customer | - | Postmark transactional stream for customer mail (system mail keeps `outbound`) |
| postmark.events_webhook_password | "" | - | basic auth for `/webhooks/postmark/events` |
| resend.webhook_secret | "" | - | Svix `whsec_` secret |
| sendgrid.events_public_key | "" | - | Signed Event Webhook ECDSA key |
| inbound.drivers | [] | [ses] | any of `ses`, `postmark`, `mailgun`, `sendgrid`, `resend`, `cloudflare`, `test`; each needs `mailbox.sources.<name>`. No smtp receiver: Never list. Options per driver: [../11-self-host/operations.md](../11-self-host/operations.md) section 15 |
| inbound.max_message_bytes | 31457280 (30 MB) | 31457280 | over: headers + text kept, attachments dropped |
| inbound.max_attachments | 50 | 50 | email-specific; overrides global 10 for inbound email only |
| inbound.raw_disk | local | s3 | filesystem disk for raw `.eml` |
| inbound.raw_retention_days | 30 | 30 | 0 = keep forever |
| inbound.parse_memory_budget_mb | 64 | 256 | weighted semaphore across parses |
| inbound.sender_rate | 20/10m | 20/10m | over: throttled (stored, no notify, no AI) |
| inbound.sender_quarantine | 100/1h | 100/1h | over: quarantined, spam |
| ses.region | "" | us-east-1 | |
| ses.inbound_bucket, ses.inbound_topic_arn, ses.events_topic_arn | "" | set | |
| ses.configuration_set | dimmi-customer | dimmi-customer | customer stream; event destination SNS. System stream uses own set (`mail.mailers.system`, foundation, e.g. `dimmi-system`) |
| ses.tenant_per_workspace | false | true | SES tenants (reputation isolation, launched 2025-08) |
| ses.mail_from_subdomain | bounce | bounce | |
| mailgun.signing_key | "" | "" | outbound events |
| outbound.max_attachment_bytes | 20971520 (20 MB) | 20971520 | over: signed links |
| outbound.attachment_link_ttl | 168h | 168h | |
| outbound.references_max | 20 | 20 | |
| widget_offline.enabled | true | true | false: `SendWidgetOfflineReply` returns skipped. Delay is 01 key `widget.offline_email_delay` (single source) |
| limits.unverified_relay_per_address_daily | 3 | 3 | widget offline mails to unverified (typed) email per address per 24 h; anti-relay abuse (edge 153) |
| limits.unverified_relay_workspace_daily | 0 (off) | (entitlement) | self-host value of entitlement `email.unverified_relay_workspace_daily` |
| limits.unverified_daily | 0 (off) | (entitlement) | self-host value of entitlement `email.unverified_daily` |
| limits.verified_daily | 0 | (entitlement) | self-host value of entitlement `email.verified_daily` |
| limits.new_workspace_hourly | 0 | (entitlement) | self-host value of entitlement `email.new_workspace_hourly` |
| limits.automated_per_recipient_hourly | 10 | 10 | AI + widget offline + acks per recipient |
| limits.connected_mailboxes | 0 (off) | (entitlement) | self-host value of entitlement `email.connected_mailboxes` |
| postmark.domain_api_token, resend.domain_api_token, mailgun.domain_api_token, sendgrid.domain_api_token | "" | - | provider domain API ([sending.md](sending.md) 7); secret, `_FILE` |
| oauth.google.enabled, .client_id, .client_secret | false | false until verification | [mailbox-connections.md](mailbox-connections.md) |
| oauth.google.pubsub_topic, .pubsub_audience, .pubsub_service_account | "" | set | push; empty: poll |
| oauth.microsoft.enabled, .client_id, .client_secret, .tenant | false, tenant `common` | false until registered | |
| mailbox.poll_interval, mailbox.poll_interval_with_push | 60s, 5m | 60s, 5m | |
| imap.enabled, imap.allow_private_hosts, imap.max_concurrent | true, false, 10 | true, false, 10 | [sources.md](sources.md) 3 |
| domain_connect.enabled, .provider_id, .service_id, .private_key, .key_host | false | true after human step | [sending.md](sending.md) 6.2 |
| round_trip.enabled, .workspace, .simulator, .alert_webhook_url, .heartbeat_url | false | true | [staging.md](staging.md) 6 |
| reputation.bounce_pause_rate | 0 (off) | 0.05 | over trailing 500 sends, min 100 |
| reputation.complaint_pause_rate | 0 | 0.001 | |
| dns.resolvers | [system] | [1.1.1.1:53, 8.8.8.8:53] | domain checker; never cached over 60 s |
| dkim.selector_prefix | dimmi | - | smtp relay driver |
| ops_forward_to | "" | ops address | postmaster@ and abuse@ on inbound domain |

### Entitlement keys (catalog `app/support/entitlements/catalog.go`, contracts.md `Entitlements`)

Cloud sending caps (were `limits.*` cloud defaults + "raise via support"). Read via `Entitlements.Limit(ctx, ws, key)` when computing `email_sending_profiles` limits (outbound.md section 13) and on each send check (cached 60 s by driver). -1 = unlimited. Self-host driver (11) maps `limits.*` config (0 = off -> -1). Cloud driver (12): catalog default, contract, `entitlement_overrides` (staff `dimmi entitlement:set` replaces manual "raise on request" DB edits). Abuse limits per recipient / per address (`limits.automated_per_recipient_hourly`, `limits.unverified_relay_per_address_daily`, sender rates) stay config: protection, not contract.

| Key | Kind | Self-host | Cloud default | Use |
|---|---|---|---|---|
| `email.verified_daily` | limit | config (0 -> -1) | 2000 | daily sends with verified custom domain |
| `email.unverified_daily` | limit | config (0 -> -1) | 200 | daily sends on default sender domain |
| `email.new_workspace_hourly` | limit | config (0 -> -1) | 50 | hourly cap first 7 days after workspace created |
| `email.unverified_relay_workspace_daily` | limit | config (0 -> -1) | 200 | workspace daily automated mail to unverified addresses (edge 153) |
| `email.connected_mailboxes` | limit (count) | config (0 -> -1) | 25 | OAuth + IMAP connections; sync cost guard |

`GET /email-sending-status` returns effective limits from driver (not config). `GET /workspace/entitlements` `used` for daily keys from `email_sending_stats` today row.

## Permissions matrix

Implemented as permissions (`00-foundation/extensibility.md` section 4, P2). Policies (`email_domain_policy.go`, `email_address_policy.go`, `inbound_email_policy.go`, `email_suppression_policy.go`, `email_signature_policy.go`) call `authz.Can(principal, permission)`; role columns derived from 00-accounts `roles.go`, never checked in 04 code. Token effective permissions = role permissions intersect expanded abilities (replaces "owner token"). Conversation-bound reads also need 03 `VisibleScope` to contain conversation.

04 permissions (registered in 00-accounts catalog `app/auth/permissions.go`; each in exactly one ability group):

| Permission | Ability group | Roles (roles.go) | Gates |
|---|---|---|---|
| `email:read` | settings:read | owner, admin, member | domains, addresses, records, setup steps |
| `email:read_raw` | settings:read | owner, admin | activity log, raw `.eml`, unrouted inbound detail, sending status, forwarding confirmation code |
| `email:manage` | settings:write | owner, admin | domains/addresses create/update/delete, checks, tests, reprocess |
| `email:rotate_dkim` | settings:write | owner | DKIM rotation |
| `email:resume_sending` | settings:write | owner | `PATCH /email-sending-status` |
| `email_suppressions:manage` | contacts:write | owner, admin | add/remove bounce or manual suppression |
| `email_suppressions:lift_complaint` | contacts:write | owner | remove complaint suppression (+ `confirm_complaint=true`) |
| `email_signatures:manage_own` | settings:write | owner, admin, member | own signature (token acts for token's member only) |
| `email_signatures:manage_any` | settings:write | owner, admin | other members' signatures, address signatures |
| `email:send_as_any` | conversations:write | owner, admin, member | pick any active address as From ([sending.md](sending.md) 2) |

| Action | owner | admin | member | Permission (token ability group) |
|---|---|---|---|---|
| View addresses, domains, records | yes | yes | yes | `email:read` (settings:read) |
| Create/update/delete address, domain, check, test | yes | yes | no | `email:manage` (settings:write) |
| Rotate DKIM key | yes | no | no | `email:rotate_dkim` (settings:write) |
| View inbound email detail of visible conversation | yes | yes | yes | conversations:read + `VisibleScope` |
| View address activity log, download raw `.eml` | yes | yes | no | `email:read_raw` (settings:read) |
| Reprocess inbound email | yes | yes | no | `email:manage` (settings:write) |
| View message email (headers, auth) | yes | yes | yes | conversations:read + `VisibleScope` |
| Edit To/Cc/Bcc on reply | yes | yes | yes | conversations:write |
| View suppressions | yes | yes | yes | contacts:read |
| Add/remove suppression (bounce, manual) | yes | yes | no | `email_suppressions:manage` (contacts:write) |
| Remove complaint suppression | yes | no | no | `email_suppressions:lift_complaint` (contacts:write) + `confirm_complaint=true` |
| Own signature | yes | yes | yes | `email_signatures:manage_own` (settings:write) |
| Others' signatures | yes | yes | no | `email_signatures:manage_any` (settings:write) |
| Sending status | yes | yes | no | `email:read_raw` (settings:read) |
| Resume sending | yes | no | no | `email:resume_sending` (settings:write) |
| See Gmail forwarding confirmation code | yes | yes | no | `email:read_raw` (settings:read) |
| Choose From on reply | yes | yes | yes | `email:send_as_any` (conversations:write) |
| Connect, change, disconnect mailbox | yes | yes | no | `email:manage` (settings:write) |
| Manage ignored and trusted lists | yes | yes | no | `email:manage` (settings:write) |
| Dismiss "Too many emails" label | yes | yes | yes | conversations:write + `VisibleScope` |
| Send setup instructions | yes | yes | no | `email:manage` (settings:write) |

## Performance budgets

| Metric | Budget |
|---|---|
| Webhook ack (SNS, provider events) | p95 under 200 ms; raw persisted before 200 |
| ProcessInboundEmail, message under 1 MB | p95 under 400 ms CPU+DB (excluding DNS for DKIM) |
| DKIM/SPF/DMARC DNS lookups | 3 s timeout each, total 5 s; cached 5 min in otter |
| Inbound end to end | p95 under 10 s (SES) |
| email `Channel.Send` (inside 03 `send_channel_message`) | p95 under 3 s from job start to provider accept |
| Domain check | under 4 s per domain (parallel lookups) |
| Threading lookup | 1 indexed query per candidate id, max 22 lookups |

## Observability

Metrics (Prometheus):
- `dimmi_email_inbound_total{source,classification,status}`, `dimmi_email_inbound_latency_seconds{source}` (provider received_at to message.created), `dimmi_email_inbound_unroutable_total{source}`, `dimmi_email_inbound_bytes`.
- `dimmi_email_thread_resolution_total{method}` (token, in_reply_to, references, provider_id, reverse_ref, subject, new).
- `dimmi_email_quote_strip_total{result}` (stripped, none, fallback_text).
- `dimmi_email_auth_total{result}`.
- `dimmi_email_outbound_total{transport,status}`, `dimmi_email_provider_events_total{provider,type}`, `dimmi_email_provider_event_rejected_total{provider}`, `dimmi_email_provider_auth_failures_total{transport}`, `dimmi_email_outbound_latency_seconds`, `dimmi_email_bounces_total{type}`, `dimmi_email_complaints_total`.
- `dimmi_email_domain_checks_total{result}`, `dimmi_email_sending_paused_workspaces`.
- `dimmi_email_mailbox_sync_total{provider,result}`, `dimmi_email_mailbox_sync_lag_seconds{provider}`, `dimmi_email_mailbox_connections{provider,status}`, `dimmi_email_mailbox_full_resync_total{provider}`, `dimmi_email_setup_findings_total{code}`, `dimmi_email_test_total{result}`, `dimmi_email_round_trip_*` ([staging.md](staging.md) 6).
Logs: inbound_email id, workspace, source, classification, thread method, auth result. Never body, never subject at info (subject debug only), addresses hashed at info (`sha256[:12]`).
Traces: span per pipeline stage (parse, auth, classify, thread, strip, ingest).

## Security and privacy

- Email HTML: bluemonday strict per foundation; stored `body_html` sanitized; rendering in sandboxed iframe (frontend); remote images blocked until click (proxy); cid images rewritten to signed attachment URLs at resource time.
- Raw `.eml` holds Bcc and full headers: admin-only, signed URL 5 min, retention 30 days, purged on contact erase.
- SNS: never auto-confirm subscription for unknown TopicArn; SubscribeURL fetched via `a.HTTP` (framework httpclient) with host allowlist `sns.*.amazonaws.com`.
- Webhook secrets compared constant-time; Mailgun/Resend/SendGrid timestamp windows 5 min; replay keys in Valkey (`SET NX EX`). Provider event webhooks verified before parse (Svix HMAC, Mailgun HMAC, SendGrid ECDSA, Postmark basic auth).
- No listening mail server, no outbound MTA (Never list): attack surface is HTTPS webhooks only.
- Spoofing: auth verdict on every inbound; unverified sender never auto-merges, never marks identity verified, never gets AI answer when claimed contact verified; outbound reply goes to authenticated identity address only (never attacker Reply-To).
- Reply tokens: 80-bit random, revocable; token reply from non-participant becomes participant with banner, no AI.
- Outbound provider credentials are instance config secrets (`_FILE` env), never in DB, never per workspace. Mailbox grants and IMAP passwords: sealed per row (`encryption.Seal`, scope workspace, AAD table + column + id), never in resources, exports, logs; erased at disconnect.
- IMAP host through `safedial`: no private ranges. Connected scopes minimal: read + send; never modify customer mailbox except IMAP APPEND to Sent.
- Google Pub/Sub JWT and Graph `clientState` checked before enqueue. Webhooks are wake-ups only: they never carry mail trust.
- DKIM private keys (smtp driver) envelope-encrypted, never exported via API.
- Gmail forwarding confirmation link never auto-clicked; code visible to admins only.
- Domain takeover: instance-wide unique verified domain; ownership TXT per workspace; later verifier with proof wins, previous owner notified.
- Abuse (cloud): daily caps, new-workspace hourly cap, auto-pause on bounce 5% / complaint 0.1% (security.md), SES tenant isolation (shared IP pools, no dedicated IPs), customer stream separate from system stream (workspace pause never blocks password reset mail), no outbound to suppressed addresses, no bulk recipients over 50.
- No open/click tracking, no link rewriting.
- PII: `inbound_emails.subject`, addresses are personal data: erased by contact eraser `EraseEmailContactData` (`app/actions/email/erase_email_contact_data.go`, `Erase(ctx, workspaceID, contactID) error`, called by 03 `EraseContact` job). Scope per `contracts.md`: `inbound_emails` and `outbound_emails` rows of contact anonymized (subject, addresses, headers `[erased]`), raw `.eml` deleted (Filesystem + SES object), `email_suppressions` keyed by contact's plain addresses replaced by `sha256` address hash (opt-out still honored: suppression check compares hash too), `email_message_refs` + `email_threads` of contact's conversations deleted and reply tokens revoked. Idempotent (missing raw object = ok).

## Metering impact (research/pricing.md D1 to D15)

- D1/D2: inbound auto-reply, OOO, bounce, list, bulk, noreply messages passed with `InboundMessage.AutoGenerated = true`, stored `substantive = false`, `metadata.auto_reply = true`. Rule: inbound auto-replies never count toward D1 activation (customer-side auto-reply is not inbound customer message for billing); 12 treats non-substantive inbound as not satisfying D1(a). AI/acks from Dimmi carry `substantive` as set by 07.
- D9: SES spam FAIL / virus FAIL / quarantine set `conversations.spam = true` before any reply: never billable.
- D10: mail from active workspace member to own support address (authenticated, not forward) passes `InboundMessage.IsTest = true`; 03 sets `conversations.is_test` (03 column; 12 skips). Forwarded-by-member mail is customer mail: normal billing.
- D13: hard bounce of reply calls 03 `UpdateMessageDelivery` failed, which emits `message.delivery_updated` (failed, final); 12 queued `ApplyBillingCredits` credits when no other reply delivered. Transient bounce after 24 h retry window counts as failed. Suppressed-at-send counts as failed. 04 emits at most one final `failed` per message (`UpdateMessageDelivery` monotonic: repeat failed is no-op, no event), so DSN + SES Bounce for same message never double-credit.
- D7: teammate-initiated new email conversation: billable only after contact replies and workspace answers. `initiated_by = member|agent|api` carried on 03 `conversation.created`; fair-use cap (`max(1,000, 10 x previous billable)`) enforced by 12 InTx `GuardOutboundFairUse` (429 `outbound_fair_use_exceeded`). 04 never counts itself.
- D9(c): inbound over `sender_quarantine` rate classified spam before any reply: not billable.
- Cost model check: per conversation 5 outbound + 2 inbound SES emails (pricing.md section 3) matches design; raw `.eml` in S3 30 days adds about 150 KB per conversation for 30 days only.

## Dependencies on other specs

| Spec | Need |
|---|---|
| 00-foundation + framework | `mail.Mailer` with raw MIME + provider id + rewritten `MessageID` + typed errors; two-stream rule (system vs customer, contracts.md); named mailer instances; built-in drivers `postmark`, `resend`, `mailgun`, `sendgrid` beside `smtp`, `ses`, `log` (framework.md); `mailbox` `ses` source + `Fake`, parse/authcheck/dsn/arf; `aws/sns`; `dom` (html-to-markdown/v2); `sanitize`; `dns`; `httpclient`; filesystem disks; Valkey (`a.Limiter`, replay keys); events; River; `messages.dedupe_key` unique; `Entitlements` contract + catalog (04 registers 4 `email.*` keys); `RetentionGuard` (subject kind `inbound_email`); registry enums `channel_connections.type` / `contact_identities.type` (04 uses core `email`, adds none) |
| 00-accounts | member email + `email_verified_at` (detect member forwards and tests); workspace slug; `authz.Can` + permission catalog (04 registers 10 permissions), roles.go |
| 01-messenger-widget | owns offline relay trigger (`ScheduleWidgetOfflineEmail`, `SendWidgetOfflineReplyEmail`, `WidgetOfflineReplyMail`) and calls 04 `SendWidgetOfflineReply`; skip when 04 returns `suppressed`/`limited`; hosted conversation continue page route `widget.conversations.continue` accepting signed token (NOT yet in 01: 01 to add, else mail omits link); `request_email` capture stores contact email |
| 02-widget-identity | verified email identities for merge and spoof rule |
| 03-shared-inbox | mounts `sender-select`, `sender-flag`, `suppression-chip`, `throttle-banner`; row label from `conversations.source.email_throttled`; new action `RecordExternalOutboundMessage`; participants per X2 (04 renders headers, builds on F16); S-12 link safety (E-06); `contracts.Channel` (implemented exactly, see contract section above); `conversations.ReceiveInboundMessage` + `UpdateMessageDelivery` actions; `send_channel_message` job; reply/new conversation endpoints accept `email` options; Cc participants display; capability-driven UI (no edit for email) |
| 05-chat-app-channels | generic channel connection CRUD + `dimmi channels:add`; 04 registers email type |
| 06-notifications | delivery of 04 notifications to admins; member notification mail uses system mailer (`mail.mailers.system`) not this channel; 06 `RecordSystemMailFeedback` owns system-stream bounces/complaints (04 webhooks route events tagged `stream=system` to it, outbound.md 7.3) |
| 07-dimmi-agent | gate rule 6 reads `metadata.auto_reply`; gate also skips when `metadata.ai_skip` present (`unverified_sender` = AI spoof guard, kept at MVP; `non_participant`, `throttled`, `recipient_rate`, `no_reply_address`). Agent actions (07 `actions.md`) treat email as verified context only when `metadata.email.auth.result = pass` (= DMARC pass with aligned DKIM or SPF, `aligned_domain` set; pipeline.md 5) and contact email identity `verified = true`; 04 confirms this meaning. Write actions over email: later (`email-action-confirmations`); 07 email footer "Reply with HUMAN" survives 04 quote stripping (keyword kept in stripped body) |
| 10-webhooks | delivers new events |
| 11-self-host | `dimmi doctor` email checks call 04 dns checker + customer-stream check; config docs for `mail.default` drivers and `mail.mailers.customer`; inbound provider config and doctor checks ([../11-self-host/operations.md](../11-self-host/operations.md) section 15), no SMTP receiver |
| 12-cloud-billing | D1 non-substantive inbound, D10 test flag, D13 credit on `message.delivery_updated` failed; SES cost tracking; cloud `Entitlements` driver + overrides back `email.*` sending caps (README "Email sending limits") |
| 13, 14 | 13 registers mailto unsubscribe prefix `cunsub` (`email.RegisterMailtoUnsubscribe`, outbound.md 10); call 04 `SendContactMail` (or wrappers `SendChangelogEmail` 13, `SendContactNotification` 14; + `ResolveVerifiedSender`, `HasVerifiedSendingDomain`) for every mail to subscribers, voters, contacts (outbound.md 16); listen to `email_suppression.created{outbound_kind}`; never system Mailables |
| 16-importers | imported email conversations arrive via 03 import helpers (no 04 pipeline, no outbound, no auto-reply, no `outbound_emails` rows); reply on imported conversation rehomes to inbox email address (outbound.md section 1 rehome rules); raw mailbox import not built (README Left out) |
| 15-announcements-surveys | CSAT request = 03 `SendSystemMessage` in thread; 04 sends it via `Channel.Send` (kind `system`, `Auto-Submitted: auto-generated`); `IsSuppressed` pre-send check |

## Contradictions and gaps found in foundation docs

1. `data-model.md` global limit 10 attachments per message conflicts with real inbound email (invoices, photo sets). 04 overrides for inbound email only (`channels.email.inbound.max_attachments` 50); outbound keeps 10. Foundation should mark limit as per-channel overridable.
2. Conventions: every table has `updated_at`. `email_message_refs` is append-only (no updates); 04 omits `updated_at`. Accept as documented exception or add unused column.
3. `conversations.external_thread_key` unique only while not closed: manual reopen of old closed conversation can collide with newer open conversation sharing root key. 04 avoids by keying linked new conversations on new Message-ID; 03 reopen action should still handle unique violation (409 `thread_key_conflict`).
4. `contacts.email_verified` "true only from verified JWT, API, or confirmed email channel": 04 interprets DMARC-aligned pass as confirmed email channel (identity `verified = true`). 02/03 must agree, else auto-merge chat+email ("same conversation when same person") rarely fires.
5. Resolved: default forwarding address created eagerly by 04 initializer in foundation workspace initializer hook (`00-foundation/contracts.md`), inside `CreateWorkspace` tx. No lazy provisioning.
6. Resolved: webhook paths follow `/webhooks/<provider>/<purpose>` (`/webhooks/ses/inbound`, `/webhooks/ses/events`, `/webhooks/{postmark,resend,mailgun,sendgrid}/events`).
7. 07 gate reads only `metadata.auto_reply`; 04 needs extra skip reasons (`metadata.ai_skip`: unverified_sender, non_participant, throttled, recipient_rate, no_reply_address). 07 to add gate rule.
8. pricing D1(a) "inbound message from external contact": 04 rule binding: inbound auto-replies (and all `AutoGenerated` / non-substantive inbound) never count toward D1 activation; customer-side auto-reply is not inbound customer message for billing. 12 to encode (reads `substantive`).
9. Resolved: 04 uses framework `mail.Mailer` as in `contracts.md` (`mail.Message.Raw`, `Tags`, `ConfigurationSet`, `TenantName`, `mail.SendResult.ProviderMessageID`, `mail.ErrPermanent` / `mail.ErrTemporary` / `*mail.RateLimitedError`).
10. Resolved: email driver sets 03 `ChannelCapabilities.RetryWindow = 24h`; `send_channel_message` reads it from driver.
11. 01 and 04 both specified widget offline relay job. Resolved: 01 owns trigger/timing/template, 04 owns delivery action. 01 calls `email.SendWidgetOfflineReply`. Continue link uses existing 01 `widget.conversations.messages.store` (reopens or redirects closed conversation); no `widget.conversations.continue` route.
12. Resolved: 04 writes own rows (`email_message_refs`, `email_threads`, `inbound_emails` linkage) via 03 `InboundMessage.AfterPersist` hook, inside `ReceiveInboundMessage` tx (channel-contract.md section 3 step 9b). Hook error aborts ingest; refs still `ON CONFLICT DO NOTHING`.
13. Resolved: `contracts.md` eraser scope removes suppression entries keyed by address; 04 keeps only sha256 address hash (matches 03 README Security), so opt-out survives erase.
14. 13, 14 and 15-announcements-surveys name 04 actions (`SendChangelogEmail`, `SendContactNotification`, `ResolveVerifiedSender`, `HasVerifiedSendingDomain`, `IsSuppressed`) that 04 did not define. Resolved: outbound.md 16 defines one generic `SendContactMail` (`Kind` = `outbound_emails.kind`: `changelog`, `feedback_update`, `portal_sign_in`; caller sub-kind in `Purpose`) + read helpers; `SendChangelogEmail` / `SendContactNotification` are thin wrappers kept for caller names. 15-announcements-surveys sends no contact mail outside threads (CSAT in thread).
15. Resolved: contact-mail kinds are `changelog`, `feedback_update`, `portal_sign_in` only; suppression scopes `all`, `notifications`, `changelog`, `feedback`; config sets `dimmi-system`, `dimmi-customer`.
16. Resolved, then widened 2026-11-04: framework.md `mailbox` gains `postmark`, `mailgun`, `sendgrid`, `resend`, `cloudflare`, `gmail`, `graph` drivers and `imap` client; `Receiver` takes `Delivery` (delta). go-smtp still blocked by depguard, no SMTP receiver. framework `mail` built-in drivers `smtp`, `ses`, `postmark`, `resend`, `mailgun`, `sendgrid` (raw decompose for non-raw APIs).
17. Suppression scopes per contact-mail kind (13 `changelog`; 14 had none): 04 adds `feedback` so a complaint on voter mail stops voter mail only (outbound.md 8). 14 to note in README item 30.

## Decisions

1. Reply-To token on by default, per-address `reply_to_mode=none`. Why: robust when forward rules or headers break. Connected mailbox sends skip it: replies return to mailbox.
2. Bulk mail without thread: closed conversation tagged `bulk-email`. Why: audit trail, zero noise.
3. Raw `.eml` retention 30 days. Why: debug and reprocess without forever storage cost or privacy risk.
4. Widget offline relay delay 120 s. Why: visitor often still on page.
5. Cloud cap 200/day unverified, 2,000/day verified, staff override `dimmi entitlement:set`. Why: limits abuse on shared domain, rewards verifying.
6. Gmail and Microsoft through their APIs, not OAuth IMAP. Google restricted-scope verification + CASA is human step ([mailbox-connections.md](mailbox-connections.md) 1).
7. ARC not verified. Why: no allowlisted library; connected mailbox solves broken DKIM. Identity stays unverified, no merge, flag rules in [sender-rules.md](sender-rules.md) 5.
8. Provider domains: API when `domain_api_token` set, else config list. Why: per-record status where possible, no forced account token.
9. Push wakes, poll is truth, for Gmail and Graph. Why: both vendors say notifications can drop; self-host may have no public URL or Cloud project.
10. Default send path of connected mailbox is mailbox API. Why: no DNS step, lands in Sent, sender reputation is customer's own. Bulk kinds never use it.
11. IaC OpenTofu ([staging.md](staging.md) 7). Why: MPL-2.0, multi-provider, S3 native lock. 12 infrastructure outline says CloudFormation; owner to confirm whole cloud follows.
12. Domain Connect optional on top of plain records, Entri no ([sending.md](sending.md) 6).
13. `[staging]` mark = provider fact not stated in docs read; fixture pins current belief, driver reports what provider returns, first staging run corrects. Never blocks build.
