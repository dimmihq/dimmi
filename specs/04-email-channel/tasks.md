# 04 Email channel: tasks

Style: caveman. Order matters. Each task about 1 day, merged with tests. `[ ]` open. Edge case numbers refer to `edge-cases.md`.

Test rule (00-conventions/testing.md): actions, jobs, controllers, listeners tested as integration tests on real Postgres + real App (`testkit`); `tc.Mail`, `httpclienttest.New(tc.App.HTTP)`, `a.DNS.Fake()` at process boundary only (fakes panic outside `app.env=testing`). Pure parsing/mapping in `app/integrations/email` (Channel driver zone) may have table-driven unit tests with fixtures, no mocks. Test names carry edge ids (`E04-17`).

Prereqs: framework (`mail` named `system`/`customer` mailers + drivers `ses`, `smtp`, `postmark`, `resend`, `mailgun`, `sendgrid`, `log`; `mailbox` `ses` source + `Fake` + parse/authcheck/dsn/arf + drivertest; `dom`, `sanitize`, `dns`, `aws/sns`, `filesystem`, `events`, `queue`), 03 `contracts.Channel` + `conversations.ReceiveInboundMessage` + `UpdateMessageDelivery` (exact `03-shared-inbox/channel-contract.md` signatures; fake until 03 lands), 05 channel connection CRUD.

## M1. Schema and scaffolding

- [ ] Migrations 0401 to 0411 (tables, indexes, RLS). Acceptance: `migrate` up/down clean; sqlc generates; tenant-query lint passes. Tests: migration test, unique constraints (verified domain instance-wide, token unique).
- [ ] Enums (incl. `outbound_emails.kind`, suppression scope, `ContactMailKind`), public ID prefixes (`edm_`, `ema_`, `ibm_`, `obm_`, `sup_`, `sig_`) in `app/models/public_ids.go`, factories for all 04 tables. Tests: prefix round trip, wrong prefix 404.
- [ ] `config/channels.go` email keys with cloud/self-host defaults, validation (SES inbound keys present when `inbound.drivers` has `ses`; provider event secrets present when driver backs customer stream). Tests: config load matrix, `config:show` redacts secrets.
- [ ] Wire framework `mailbox` source `ses` to `ReceiveInboundEmail`; `framework/mailbox/drivertest` green for `ses` (built). Acceptance: test-signed SNS payload (cert stubbed via `httpclienttest`, raw message in per-test bucket) feeds pipeline in feature test.

## M2. Parsing and body

- [ ] Fixture corpus v1 in `tests/fixtures/email/`: 60 real-client replies (Gmail web/iOS/Android, Outlook web/desktop/mobile/new, Apple Mail, Yahoo, Proton, Fastmail, Thunderbird, Spark, HEY) x EN plus 11 locales quote headers; charsets (case 17-21); malformed (22, 23, 26). Anonymized. Acceptance: README per fixture with expected stripped output.
- [ ] Parse wiring on `framework/mailbox/parse` (enmime/v2; size limit, partial parse, charset fallback in `parse.Limits`; fuzz lives in framework): 04 memory semaphore, NFC, NUL strip, bidi strip, TNEF, S/MIME/PGP policy. Tests: edges 13-31 as pipeline integration tests over `.eml` fixtures.
- [ ] `htmlbody.go`: email policy on `sanitize.NewPolicy()`, tracking pixel removal, hidden-element drop (`dom`), cid rewrite hook, data URI extraction, HTML -> Markdown via `dom.ToMarkdown` (html-to-markdown/v2) with 04 options, plain-text format=flowed. Tests: XSS corpus (94), hidden text (95), cid (96, 97), emoji/RTL (98).
- [ ] `quotes.go` HTML DOM pass (all markers in pipeline.md 9.3) + signature wrappers. Tests: corpus fixtures, inline reply (87), fallback (89).
- [ ] `quotes.go` text pass (port of GitHub `email_reply_parser`, MIT: attribution header + `THIRD_PARTY_NOTICES.md` entry with upstream commit; `quote_headers.yaml` 12 locales, Outlook header block, wrapped Gmail header, signature rules). Tests: 88, 90, 91, 92; corpus score report printed (target 97%).
- [ ] Attachments extraction: limits (count 50, size, blocked types), inline vs attachment, dedupe sha256, signature asset flag, skipped list. Tests: 14, 15, 25.

## M3. Authentication and classification

- [ ] `auth.go`: policy over `framework/mailbox/authcheck` (DKIM verify, DMARC lookup via go-msgauth): relaxed/strict alignment + org domain, SPF from SES verdict only (no local SPF, A-R headers never trusted), SES verdict mapping, `ai_skip = unverified_sender` spoof guard (no visible flag), result rules. `a.DNS` fake in tests. Tests: 72-78, forwarded DKIM pass, DNS timeout unknown.
- [ ] `classifier.go` ordered rules 1-13, YAML pattern files (autoreply subjects 15 languages, noreply patterns, form relays). Tests: 55-58, 63-65, 68, 69; precision report on corpus (human misclassified under 1%).
- [ ] `bounces.go`: map `framework/mailbox/dsn` + `arf` results (RFC 3464 incl. non-standard Gmail, Outlook, Postfix, Exim, qmail fixtures; RFC 5965) to bounce/complaint actions, contact-mail kind scopes. Tests: 59-62, 181, 182.
- [ ] `formrelay.go` + `forwarded.go` (Gmail, Apple, Outlook localized, Thunderbird, rfc822 attachment). Tests: 66, 67, 84, 85, edge with missing original address.
- [ ] `gmail.go` forwarding confirmation detection + Zoho variant + `StoreForwardingConfirmation` action + event. Tests: code/link extraction, never fetches link.

## M4. Routing, threading, ingest

- [ ] `addressing.go`: forwarding locals (`<slug>`, `<slug>.<label>`, reserved names), reply token generation/format/parse, VERP and unsubscribe locals. Tests: reserved slugs, token in other workspace (45).
- [ ] `ReceiveInboundEmail` action: route recipients, per-workspace rows, dedupe, raw persist (disk or RawRef), InsertTx job. Tests: 1, 4, 6-10; latency under 200 ms with 1 MB raw.
- [ ] `subject.go` normalization (20+ prefixes, tags, ticket tag). Tests: table of 80 subjects.
- [ ] `threading.go` resolution order 1-8 (incl. 7b contact mail reply) + refs writing (inbound, referenced, cap) + merged/deleted handling + reopen-window integration. Tests: 32-54, 183, 184.
- [ ] `ProcessInboundEmail` job + action: full pipeline wiring, advisory lock per sender, 03 `conversations.ReceiveInboundMessage` call (`InboundMessage` mapping pipeline.md section 10), participants (cc, new participant, bcc), email_threads ensure, metadata shape, `ai_skip` rules, member test flag (`InboundMessage.IsTest`), rate limits (sender_rate, quarantine). Tests: 50, 83, 86, 154, 156; end-to-end fixture to message; events `conversation.created`, `message.created`; metering fixtures: OOO/bulk inbound stored `substantive=false` + `metadata.auto_reply`, member test sets `conversations.is_test` (12 reads these keys); cross-tenant: token/refs of workspace A never touch A when routed to B.
- [ ] Reprocess action + `POST /inbound-emails/{id}/processing-runs` (`InboundEmailProcessingRunController.Store`, 202) + `dimmi email:reprocess`; `ReprocessSuspendedInboundEmails` + stuck-row sweep. Tests: 5, 8.

## M5. Inbound (SES)

- [ ] SNS: `framework/aws/sns` verify wired in SES controllers; 04 TopicArn check + subscription confirm via `a.HTTP` (host allowlist). Tests: 138, 139 with fixture certs.
- [ ] SES inbound controller on framework `mailbox` `ses` driver + S3 raw opener. Tests: feature test with SNS fixture, verdict mapping, headersTruncated, unknown recipient metric.
- [ ] `dimmi email:test-inbound` (dry run prints classification, thread decision, stripped body). Tests: CLI golden output.

## M6. Outbound

- [ ] `composer.go` (04 header policy on framework `mime.Builder`): headers table (outbound.md section 4), Message-ID format, References trimming, multipart structure, inline cid, attachments + link conversion over 20 MB, EAI/IDN handling. Tests: golden `.eml` outputs; 104, 119, 120; headers never include tracking; Precedence only on bulk contact mail.
- [ ] Templates `conversation_reply.templ` + text twin, quote previous (localized header, timezone), signature rendering with `-- ` delimiter, AI citations footer. Tests: golden HTML/text; litmus-style manual check list in PR (Gmail, Outlook, Apple Mail screenshots).
- [ ] `integrations/email/channel.go` implementing `contracts.Channel` (Capabilities, Send) + sender resolution table (outbound.md section 2) + recipient resolution/validation rule `rules.EmailRecipients` for 03 requests. Tests: each sender row; 70, 104, 105, 114.
- [ ] Transports: `mailer.go` resolves customer stream `a.Mail.Mailer(channels.email.mailer)` (fallback `mail.default` + stream overrides; boot refuses `log` in production); `mail.Message.Raw`, `ConfigurationSet` (`dimmi-customer`), `Tags` (stream=customer), `TenantName` on cloud; `smtp` relay with `framework/mail/dkim` signer; `postmark`/`resend`/`mailgun`/`sendgrid` via framework drivers (raw decompose where needed), tracking off; `mail.SendResult.MessageID` stored as `outbound_provider` ref; map `mail.ErrPermanent` / `mail.ErrTemporary` / `*mail.RateLimitedError` / provider auth / sender rejected to 03 channel errors (outbound.md 6); driver declares 24 h retry window; retry + ordering owned by 03 worker. Tests: `tc.Mail` capture + `httpclienttest.New(tc.App.HTTP)` per API provider fixture, Mailpit-backed `smtp` driver covered by framework: 100, 101 (clock freeze: still pending at 23 h, failed final after 24 h), 108, 109, 115, 116, 157, 158, 169, 170, 174, 179.
- [ ] `SendEmailMessage` action as body of email `Channel.Send` (called by 03 `send_channel_message`; no 04 job or listener) + outbound_emails + refs (outbound + `mail.SendResult.ProviderMessageID`) + `SendResult` + `last_outbound_at`; delivery state via 03 `UpdateMessageDelivery`. Tests: 03 channel contract suite `tests/contracts/channel_test.go`; 102 metric, 111, 112, double-run idempotent (same `IdempotencyKey`).
- [ ] Signatures API (`email-signatures` CRUD, member own vs admin), sanitized Markdown subset. Tests: permissions, 144.
- [ ] `POST /email-addresses/{id}/tests` (`EmailAddressTestController.Store`, sync outcome `email_address_test`): outbound test mail + forwarding round-trip (X-Dimmi-Test token, marks `inbound_verified_at`, no conversation). Tests: both paths.

## M7. Delivery status, suppression, unsubscribe

- [ ] `ses_event_controller` + `ProcessSesNotification`: Delivery, Bounce (all types/subtypes), Complaint, DeliveryDelay, Reject; out-of-order rules; stats counters; system timeline lines. Tests: table per subtype; out-of-order; D13 event payload.
- [ ] DSN path for `smtp` relay transport (VERP `bounces+<token>` to forwarding inbound domain) -> `RecordEmailBounce`. Tests: 59, 60 with VERP.
- [ ] Provider event webhooks (`/webhooks/{postmark,resend,mailgun,sendgrid}/events`): signature verify (basic auth, Svix, HMAC, ECDSA), replay windows (Valkey `SET NX EX` keys, SNS MessageId dedupe 24 h), `providerevents/*` normalizers (outbound.md 7.1 table), `ProcessMailProviderEvent` with unknown-id retry, provider suppression mirror, tracking events ignored. Tests: fixture per provider and type; 148, 172, 173, 191 (Valkey down: 503, no write), 192 (system-stream event routed to 06, outbound.md 7.3); double event idempotent.
- [ ] Suppression actions + API (`email-suppressions` index/store/destroy, complaint lift owner + confirm, SES `DeleteSuppressedDestination` for hard bounce on cloud) + `ExpireEmailSuppressions` + CLI. Tests: scopes, expiry, permissions, events `email_suppression.created/deleted`.
- [ ] Unsubscribe: token signer, `List-Unsubscribe` + `List-Unsubscribe-Post` on relay mail (and caller URL passthrough on contact mail), web GET/POST (RFC 8058 one-click), mailto `+unsub-` handling, undo. Tests: POST without CSRF works only with valid token; 123.

## M8. Custom domains

- [ ] `dns/records.go`: expected records per driver (ownership, SES DKIM CNAMEs from SigningHostedZone, smtp DKIM TXT with 255 splitting, return path MX/SPF, DMARC; manual drivers: ownership + provider guidance + selector hints as `info`), SPF merge proposal, BIND export, IDN, provider hint from NS. Tests: golden record sets per driver; merge cases.
- [ ] `app/services/ses/identities.go` (adapter, sesv2): CreateEmailIdentity (RSA 2048), tenant create + resource association, MailFrom attributes (USE_DEFAULT_VALUE), GetEmailIdentity status mapping, DKIM detection restart. Tests: fake SES responses; 136.
- [ ] `dns/checker.go` on `a.DNS`: dual resolver, record statuses, propagating, all mistake detectors. Tests: fake resolver matrices for 124-128, 130, 131, 137.
- [ ] Domain actions + API (`email-domains` index/store/show/update (`mail_from_subdomain`)/destroy, `POST checks` (202), `POST dkim-rotations`), status machine incl. manual-driver trust (`provider_verified_domains` -> `provider_trusted_at`) and `provider_rejected_sender` degrade, polling schedule (`ScheduleEmailDomainChecks`, `CheckEmailDomain`, `RecheckVerifiedEmailDomains`), effects (address linking, sending switch, degraded fallback), events + notifications. Tests: state transitions with frozen clock, 106, 107, 128, 129, 132, 135, 155, 160, 170, 171, 175; rate limit 429 on check; other-workspace domain/address ids 404.
- [ ] `dimmi email:doctor <domain>` (also used by 11 `dimmi doctor`): plain-language report incl. inbound domain MX, provider records for configured driver, customer-stream mailer check (driver, credentials, event webhook), direct-to-MX warning (169), inbound = SES receipt, no SMTP receiver (168). Tests: golden output with fake resolver.

## M9. Addresses and setup flow

- [ ] Email address actions + API (`EnsureDefaultEmailAddress` registered as 04 initializer in foundation workspace initializer hook: idempotent, runs inside `CreateWorkspace` tx, only when `inbound_domain` set; index never provisions; store forwarding, show with setup guides per provider incl. Microsoft 365 auto-forward policy note, update incl. regenerate forwarding address with 30-day alias, destroy), channel events. Tests: 7, 150, 151; permission matrix; `address_in_use`.
- [ ] Activity log (`email-addresses/{id}/inbound-emails`), `inbound-emails/{id}`, raw download (signed, audit), `messages/{id}/email`. Tests: member vs admin visibility, 143, 410 after purge.

## M10. Widget offline relay and contact mail

- [ ] `SendContactMail` (one generic action; `SendChangelogEmail`, `SendContactNotification` thin wrappers; `ResolveVerifiedSender`, `HasVerifiedSendingDomain`, `IsSuppressed(ws, email, scopes...)`) per outbound.md 16: guards order, per-kind caps/pause table (section 13), kind suppression scopes, sender, customer stream, substream stats buckets (`changelog`, `feedback`), idempotency key unique, `outbound_emails.kind` = contact-mail kind, `metadata.tags`, List-Id/List-Unsubscribe/Precedence headers. Arch test `tests/arch/email_streams_test.go` (only `app/actions/email` resolves customer mailer; Mailables member-only). Tests: each skip reason; 176, 180, 185, 186, 187, 188; double call same key sends once.
- [ ] Contact-mail feedback (outbound.md 7.2): SES/provider/DSN events on rows without message -> stats bucket, kind scope suppression, `email_suppression.created{outbound_email_id, outbound_kind}`. Tests: 181, 182, 189.
- [ ] System message outbound (15-announcements-surveys CSAT request): `author_type system` via `Channel.Send`, kind `system`, `Auto-Submitted: auto-generated`, thread headers, no signature/quote, text twin URLs. Tests: 190; golden `.eml`.
- [ ] `SendWidgetOfflineReply` action (called by 01 `SendWidgetOfflineReplyEmail` job; 04 registers no listener/job): suppression, automated + unverified-relay limits, default sender, Reply-To token, `Auto-Submitted`, List-Unsubscribe, idempotency per message set, `sent|skipped{reason}` result. Tests: 121-123, 53, 153; double call same message set sends once; integration with 01 job stub.
- [ ] Relay bounce feedback to 01 (`metadata.email_relay_failed` on contact-visible message state). Tests: 121.
- [ ] Mailto unsubscribe prefix registry `email.RegisterMailtoUnsubscribe(prefix, handler)` (04 `unsub`, 13 `cunsub`), routing before classification, `SendContactMail` MailtoToken prefix check. Tests: 196, 197 (fake handler registered by test provider until 13 lands).
- [ ] 16 seams: rehome outbound rules (no refs on first mail after rehome, `Re:` subject, token Reply-To), `imported_message` defense in `SendEmailMessage`, auto-respond guard on `metadata.imported`; system-stream allowlist for 12 `billing_email` Mailables in `email_streams_test.go`. Tests: 193, 194, 195, 198.

## M11. Limits and reputation (cloud)

- [ ] Register 04 entitlement keys (`email.verified_daily`, `email.unverified_daily`, `email.new_workspace_hourly`, `email.unverified_relay_workspace_daily`; self-host maps `limits.*`, 0 -> -1) and `used` providers. Tests: fake driver values flow into profile limits; override bust recomputes profile; no `app.edition` branch in 04 cap code.
- [ ] Register 04 permissions in 00-accounts catalog (9, README Permissions matrix) + roles.go; policies `authz.Can` only. Tests: role matrix; admin denied `email:rotate_dkim`, `email:resume_sending`, `email_suppressions:lift_complaint`; member token with `settings:write` denied `email:manage`; lint no role names.
- [ ] `email_sending_profiles` + daily/hourly caps (from entitlements) with reschedule to next window, banner data in `email-sending-status`, content safety rule for default domain (D7 fair-use owned by 12 guard; 04 only maps 429). Tests: cap reached reschedule (clock freeze), 140, 159.
- [ ] `EvaluateEmailReputation`: trailing window rates over `customer` + `changelog` + `feedback` substreams, pause/review/resume flow (`PATCH /email-sending-status` field-dispatch: `status: active` to `ResumeEmailSending`, `status: review` to `RequestEmailSendingReview`, else 422 `invalid_transition`), SES tenant paused error mapping, events, notifications. Tests: 141, 162, 189; self-host disabled by default.
- [ ] Automated per-recipient limit (AI + relay) and inbound sender rate/quarantine wired with `ai_skip`. Tests: 68, 69 chaos test (two workspaces, OOO bots) proves loop stops.

## M12. MCP, CLI, observability, privacy

- [ ] MCP tools `dimmi_list_email_addresses`, `dimmi_get_email_domain`, `dimmi_create_email_domain`, `dimmi_check_email_domain`, `dimmi_get_message_email` (schemas from OpenAPI). Tests: ability checks, output shape.
- [ ] Metrics, logs (hashed addresses, no bodies), traces per stage; Grafana dashboard JSON in `docs/ops/`. Tests: metrics registered; log redaction test (no subject/body at info).
- [ ] GDPR: contact eraser `EraseEmailContactData` (`app/actions/email/erase_email_contact_data.go`, registered on 03 eraser registry: raw delete incl. SES object, anonymize inbound/outbound rows, hash suppression addresses, delete thread refs, revoke tokens), `RevokeConversationEmailThreads`, `PruneInboundEmailRaw` (`RetentionGuard` per row). Tests: 142, 46, fake guard false keeps raw; eraser second run no-op; other contact rows untouched; failing raw delete returns error (erase job retries); 03 registry test lists it.
- [ ] Ingest via 03 `InboundMessage.AfterPersist`: `email_message_refs`, `email_threads`, `inbound_emails` linkage written in 03 tx. Tests: 161 (forced hook error rolls back message, job retry ingests once).

## M13. Quality gates and docs

- [ ] Corpus v2: 500 replies for threading score, 300 for stripping, 200 auto/bulk/bounce mails; CI job prints scores, fails under bar (98% threading, 97% stripping, under 1% human misclassified, 0 AI-trigger on auto mail).
- [ ] Load test: 50 inbound/s sustained for 10 min on 1 replica (SES webhook + processing), p95 under 10 s, RSS under budget.
- [ ] Docs: forwarding guides (Gmail, Google Workspace, Microsoft 365 incl. outbound spam policy, Fastmail, iCloud, Zoho, cPanel; Sent folder note 178), self-host inbound guide (SES receipt on own AWS, no SMTP receiver), self-host outbound provider guide (`mail.default` per driver, `mail.mailers.customer` stream overrides, event webhook registration, `provider_verified_domains`), Never list page, SES ops runbook (receipt rules, bucket lifecycle 31 days, configuration sets `dimmi-system` / `dimmi-customer`, tenants, shared IP pools only), DNS provider record guides.

## Done when

Demo script (cloud staging + self-host box, real mailboxes, no LLM calls needed except optional agent step with user approval):

1. Cloud: sign up, open Email settings: `acme@inbound.trydimmi.com` shown within 1 s.
2. Gmail: set forward to that address; Dimmi shows Gmail confirmation code live; confirm in Gmail.
3. From personal Gmail, email support@ (forwarded). Conversation appears in under 10 s, subject correct, signature "Sent from my iPhone" stripped.
4. Reply from inbox: arrives in Gmail from `acme@inbound.trydimmi.com` with "Sam from Acme Support", in same Gmail thread; reply from Gmail lands in same conversation with quote stripped.
5. Add `acme.com`: copy records to Cloudflare, Check now: records go green within 2 min; next reply comes from `support@acme.com`; Gmail "Show original" shows SPF, DKIM, DMARC PASS.
6. Customer sets OOO and replies: OOO appears greyed, conversation stays closed, no AI reply, no notification.
7. Send reply to `nonexistent@gmail.com` Cc: bounce shows "mailbox does not exist", address suppressed, delivery failed in timeline.
8. Spoofed mail (swaks with From of verified widget user, no DKIM): attaches to real contact, agent does not answer (`ai_skip = unverified_sender`), teammate reply goes to real address.
9. Widget visitor writes, closes tab; teammate replies twice; one email arrives after 2 min with both replies and "Continue the conversation" link; replying by email continues widget conversation; one-click unsubscribe works.
10. Self-host box (no port 25), `mail.default=postmark` in config: SES receipt rule on own AWS for inbound domain; Fastmail forward to forwarding address arrives within 10 s; add `acme.com` (ownership TXT + listed in `provider_verified_domains`), reply arrives from `support@acme.com` via Postmark customer stream; Postmark bounce webhook marks a bad recipient failed.
11. `dimmi email:doctor acme.com` explains a deliberately broken duplicate SPF record.
12. `go test ./...` green; corpus scores meet bar; chaos loop test passes.
