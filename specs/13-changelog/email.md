# 13 Changelog email: subscribers, mailings, deliverability

Style: caveman. Owner 13. Delivery owned by 04 (`04-email-channel/outbound.md` sections 15-16, defined): `email.SendChangelogEmail` = thin wrapper over `email.SendContactMail(ctx, a, SendContactMailInput{Kind: changelog, Purpose: entry|confirmation, ...})`; wrapper maps `skipped suppressed` -> `email.ErrSuppressed`, `skipped paused` -> `email.ErrSendingPaused`. Instance-level mail provider only (deploy config), customer stream, never `app/mail` Mailables, never `a.Mail` directly (04 arch test `tests/arch/email_streams_test.go`). `outbound_emails.kind = changelog`, `metadata.purpose` = entry | confirmation. One email per announced entry; no digest cadence (later: changelog-digest). Facts checked 2026-11-02: Gmail/Yahoo bulk sender rules (about 5,000 messages/day to Gmail = bulk; SPF + DKIM + DMARC; RFC 8058 one-click unsubscribe honored within 2 days; spam rate under 0.3%, aim under 0.1%). Sources at bottom.

## 1. Subscriber model decision

Separate `changelog_subscribers` table, optional `contact_id` link. Not contacts.

Why:
- Anonymous page subscriber gives unverified email. Creating contact per subscribe pollutes inbox contacts and invites impersonation merges (foundation rule: never merge on unverified email).
- Subscription state (pending, consent proof, unsubscribe reason) is changelog-specific; contacts table is core, owned by foundation.
- Link to contact when trustworthy: widget subscribe by contact with verified email, or later confirmation by identified contact (same email, `email_verified = true`). Link drives eraser and inbox sidebar "Subscribed to changelog".
- One subscriber row per (changelog, email). Workspace with 3 help centers: up to 3 rows per email (separate lists, separate unsubscribe).

## 2. States

```
(none) -> pending        subscribe via page/widget without verified identity, or API consent double_opt_in
pending -> active        confirm (POST on confirmation page)
(none) -> active         widget subscribe with verified contact email (consent verified_identity), API/import consent attested, dashboard add with attested
active -> unsubscribed   unsubscribe page, one-click, mailto, admin, API
active -> bounced        04 hard bounce suppression
active -> complained     04 complaint suppression (scope `changelog`: blocks changelog mail only, support mail still flows)
pending -> (deleted)     not confirmed in 7 days (purge)
unsubscribed|bounced|complained -> pending   only by person: new subscribe through page/widget (double opt-in again, even for verified identity when previous state complained)
```
Never back to `active` via API, import, or dashboard: 409 `subscriber_unsubscribed`. Import skips these rows (counted `skipped_unsubscribed`). Law and trust: unsubscribe is final unless person opts in again.

`bounced` re-subscribe: allowed through double opt-in; 04 hard-bounce suppression (scope `all`) still blocks send until 04 suppression removed (owner action in 04) or address delivers confirmation (confirmation send blocked too, so page shows generic success; documented in edge cases).

## 3. Subscribe flows

Page (no JS): `POST /changelog/subscriptions` form fields `email`, `website` (honeypot, must be empty), `ts` (render timestamp HMAC; submit under 2 s or over 24 h -> silently treated as bot: same success page, nothing stored). JSON variant `POST /api/changelog/subscriptions` (hc.js) always 202 `{status: "check_email"}`.

Server (`SubscribeToChangelog`):
1. Normalize email (trim, NFC, lowercase domain, IDNA A-label domain, reject invalid syntax 422 only on JSON API; form shows inline error). Reject role-less obviously invalid (`@example.com`, `.test`, no MX check at submit: no network on request path).
2. Rate limits: per IP `changelog.subscribe.per_ip_per_hour` (10) on Valkey limiter (`a.Limiter`, rebuildable; Valkey flush = window resets, acceptable); per address 3 confirmations per 24 h from durable `confirmation_count` + `confirmation_sent_at` (Postgres, survives Valkey flush).
3. Existing row: `active` -> nothing sent, same success response (no enumeration; optional "you are already subscribed" email? no: would let attacker spam). `pending` -> resend confirmation if limits allow. `unsubscribed|bounced|complained` -> set `pending`, new `token_nonce`, send confirmation.
4. New row `pending`, consent `double_opt_in`, `consent_evidence {ip_prefix, user_agent, page_url}`.
5. Queue `SendChangelogSubscriptionConfirmation`.

Widget: `POST /widget/v1/changelog/subscriptions {email?}`. Contact with verified email (`contacts.email_verified`) and no `email` param (or same email): active immediately, consent `verified_identity`, `contact_id` set, `changelog_subscriber.subscribed`. Otherwise pending + confirmation, `contact_id` set only if email matches contact email.

API/dashboard: `POST /changelog-subscribers {help_center_id, email, name?, consent: "attested"|"double_opt_in"}`. `attested` records `attested_by` actor + time; caller asserts lawful basis (owner responsibility, dashboard copy explains). Imports: `POST /changelog-subscriber-imports` max 1,000 rows, `consent: "attested"` required, result counts `{created, updated, skipped_unsubscribed, skipped_invalid, skipped_duplicate_in_file, skipped_limit}`. Cloud import guard: workspace over 1,000 imported subscribers in 24 h without verified sending domain -> 423 `sending_domain_required` (cheap list-washing deterrent).

## 4. Confirmation

Confirmation email (templ view `resources/views/mail/changelog/confirmation.templ` + text twin, rendered by 13 `RenderChangelogConfirmationEmail`; not a Mailable): subject "Confirm your subscription to {changelog title}", body: one sentence, button link `https://{canonical host}{base}/subscription-confirmations/{token}`, "Did not ask? Ignore this email." No other user content. Sent via 04 `email.SendContactMail{Kind: changelog, Purpose: confirmation, Unsubscribe: nil (transactional), Automated: true, IdempotencyKey: changelog_confirm:{csub}:{confirmation_count}}` from changelog from-address. Result `skipped{suppressed}` (scope `all`/`changelog`): page still shows generic success (no enumeration).

`GET .../subscription-confirmations/{token}`: verify token (HMAC, purpose `confirm`, nonce match, issued within 7 days), render page "Confirm subscription to {title}" + button (POST same path). Invalid or expired: page "Link expired, subscribe again" with form. GET never changes state (link scanners).

`POST`: `pending` -> `active`, `confirmed_at`, `subscriber_count + 1` (same tx), event `changelog_subscriber.subscribed`. Already active: idempotent success page. Unsubscribed meanwhile (rare): treat as fresh confirm (person acted).

## 5. Mailings

### Creation

- Publish entry with `announce = true` and `notify_subscribers = true` -> `CreateChangelogMailing` in publish tx: `kind entry`, `status scheduled`, `send_after = published_at(now) + email_delay_minutes`. Backdated publish (`published_at` explicitly in past): mailing not created unless request sets `notify_subscribers: true` explicitly (default false for backdated).
- Scheduled entry: mailing created when scheduled publish runs (send_after = actual publish + delay).
- Later email for published entry: `POST /changelog-mailings {entry_id}` (once; 409 `already_mailed` if non-canceled mailing exists).
- Unpublish, trash, or `notify_subscribers` set false while mailing `scheduled`: mailing `canceled` in same tx.
- `PATCH /changelog-mailings/{clm} {status: "canceled"}` during `scheduled`: canceled. During `sending`: remaining pending recipients `skipped` (reason `canceled`), status `partially_sent`.

### Start (`StartChangelogMailing` at `send_after`)

1. Re-fetch mailing and entry: entry still published and not trashed, changelog enabled, help center active. Else `canceled`.
2. Guards (cloud unless noted):
   - 04 pause for kind `changelog` or whole workspace (outbound.md 13): `blocked` `email_sending_paused` (retry when resumed: owner re-sends via `POST /changelog-mailings` after cancel; blocked mailing never auto-resumes after 24 h to avoid stale news; within 24 h listener on `email_sending.resumed` re-queues start).
   - Active subscriber count over entitlement `changelog.email.default_sender_max_recipients` (`Entitlements.Limit`, cloud 50, self-host -1) and no verified sending domain (`email.HasVerifiedSendingDomain`) or from-address on unverified domain: `blocked` `sending_domain_required`.
   - Month recipients so far (sum `recipient_count` of this month's mailings, Postgres) + this mailing over entitlement `changelog.email.monthly_recipient_cap` (cloud 10,000, self-host -1; staff override per customer via 12): `blocked` `monthly_cap_reached` (mailing status only, not an HTTP error; no partial send; owner raises cap via support or waits).
   - Zero eligible recipients: `sent` with `recipient_count 0` (`blocked_reason no_recipients` informational).
3. Render snapshot: subject (entry `title`; prefix none), preheader (`summary`), HTML via `support/emailhtml` (inline styles, 600 px table layout, absolute URLs, images with width/height and alt, code blocks wrapped, alerts as bordered boxes, dark-mode friendly colors), text twin (Markdown stripped to readable text with link list). Size check: HTML over `html_budget_kb` (90) or `email_content = summary`: summary + cover + "Read the full update" link. Store `rendered_html`, `rendered_text` with placeholders `{{unsubscribe_url}}`, `{{manage_url}}`.
4. Snapshot recipients: `INSERT INTO changelog_mailing_recipients SELECT ... FROM changelog_subscribers WHERE changelog_id = $1 AND status = 'active'` in chunks of 5,000 ordered by id; `recipient_count`.
   - Feedback dedupe (14): entry linked to feedback posts (`feedback_post_ids` non-empty, `notify_voters = true`): 14 sends voter `feedback_update` mail. Snapshot calls 14 read action `feedback.ShippedRecipientContactIDs(ctx, entryID)`; subscribers whose `contact_id` is in set, or whose email equals an email of those contacts, inserted as `skipped` reason `feedback_notified` (one email per person per ship, voter mail wins: more specific). 14 not notifying (no posts, `notify_voters = false`): empty set.
5. `status sending`, `started_at`, enqueue first batch.

### Batch (`SendChangelogMailingBatch`, 100 recipients)

Per recipient (status `pending` only, so retries idempotent):
- Subscriber still `active` (re-check: unsubscribed after snapshot -> `skipped` reason `unsubscribed`).
- Per-recipient input to `email.SendChangelogEmail` (`SendContactMailInput{Kind: changelog, Purpose: entry, Automated: true}`): `Unsubscribe: &UnsubscribeSpec{URL: https://{host}{base}/unsubscriptions/{token}, Post: true, MailtoToken: cunsub-{token}}` (prefix `cunsub` registered by 13 at boot via 04 `email.RegisterMailtoUnsubscribe`) (04 writes `List-Unsubscribe` + `List-Unsubscribe-Post: List-Unsubscribe=One-Click`; 13 owns one-click endpoint), `ListID: "{title}" <{clg}.changelog.{host}>`, no `ReplyTo` (replies reach From address, no entry tagging; later: changelog-replies-to-inbox), `Tags {entry: cle_, mailing: clm_}` (04 `metadata.tags`), `IdempotencyKey changelog:{clm}:{csub}`. 04 adds `Precedence: bulk` for kind `changelog` and `Auto-Submitted` from `Automated` (outbound.md 4, 16).
- Result mapping (`SendContactMailResult{Status, SkipReason, OutboundEmailID}` via wrapper): `sent` -> `sent`, `outbound_email_id`, `sent_at`, subscriber `last_emailed_at`. `email.ErrSuppressed` (skip `suppressed`) -> `skipped` (suppressed). `email.ErrSendingPaused` (skip `paused`) or skip `disabled` -> stop: remaining `skipped` (reason `paused`), mailing `partially_sent`. Skip `limited` -> batch rescheduled (04 caps for kind). Skip `no_sender` -> stop, mailing `blocked` `sending_domain_required`. Errors: `mail.ErrPermanent` -> `failed` + `error_code`; `mail.ErrTemporary` / `*mail.RateLimitedError` -> batch job retries (already-sent rows untouched; idempotency key makes retry after crash return stored result, no resend).
- Counters updated per batch; `changelog_mailing.updated` coalesced 1/s.
- Rate: per-workspace limiter `rate_per_second` (20) on Valkey (`a.Limiter`); next batch scheduled accordingly. Valkey flush mid-mailing: limiter resets, 04 provider backoff still applies; no double send (recipient status + idempotency key in Postgres).

### Finish

All recipients non-pending -> `sent` (or `partially_sent` when any skipped by cancel/pause, `failed` when every recipient failed), `finished_at`, event `changelog_mailing.completed`. Watchdog: no progress 30 min -> requeue batch once, then `failed` + 06 `system_alert`.

Bounces and complaints arrive later through 04 `email_suppression.created {email, reason, scope, outbound_email_id, outbound_kind}` -> `SyncSubscriberSuppression` (handles `outbound_kind = changelog`, plus any hard bounce scope `all` by email): subscriber `bounced` (hard bounce) or `complained`; recipient row by `outbound_email_id` -> status, mailing `bounced_count`/`complained_count` incremented; `subscriber_count` decremented; `changelog_subscriber.unsubscribed` reason `bounce`/`complaint`.

## 6. Unsubscribe

- Token = HMAC purpose `unsubscribe`, no expiry, bound to `token_nonce` (rotated only on re-subscribe after unsubscribe, so old links of old subscription stop working cleanly).
- One-click: `POST .../unsubscriptions/{token}` body `List-Unsubscribe=One-Click` -> `unsubscribed` (`one_click`), 200 empty body, no CSRF, no session. Gmail/Yahoo call this from their UI.
- Page `GET`: "Unsubscribe from {title} emails" button (POST, empty form = unsubscribe). After unsubscribe: "Unsubscribed. Changed your mind? Subscribe again" (double opt-in form).
- mailto: 13 registers prefix `cunsub` at boot (`email.RegisterMailtoUnsubscribe("cunsub", handler)`, 04 outbound.md 10). `MailtoToken cunsub-<token>` in `List-Unsubscribe` mailto; 04 inbound recognizes it before classification and calls 13 `UnsubscribeFromChangelog` (`mailto`), message discarded.
- Manage link in footer (`{{manage_url}}`) = same unsubscribe page.
- Footer always contains: changelog title, help center URL, physical address line when workspace sets `settings.company_address` (CAN-SPAM; dashboard warns when empty and subscribers over 50; not blocking for self-host).

## 7. Deliverability rules

- Cloud: changelog mail sent from verified customer domain (04 `email_domains` verified: SPF include, DKIM, DMARC check advisory). Default sender (`inbound.trydimmi.com`) only up to 50 recipients (try-out). Protects shared domain reputation.
- 04 reputation evaluation counts changelog sends (`email_sending_stats` bucket `changelog`, own caps, not against 04 conversational daily cap); complaint rate over 0.1% pauses kind `changelog` per 04 section 13 (shared domain reputation). Complaint on changelog mail = 04 suppression scope `changelog` (that person keeps getting support replies). Dashboard shows changelog-specific complaint rate per mailing.
- Double opt-in default for anonymous; attested imports guarded (section 3).
- No link tracking, no pixels: links point to real URLs (better spam scores, privacy).
- `Message-ID` per recipient (04), no BCC batching, one recipient per email.
- Self-host: instance mail provider from deploy config (04 drivers `ses`, `postmark`, `resend`, `mailgun`, `sendgrid`, `smtp` relay; customer-stream mailer per 04 section 15); no per-workspace provider; `rate_per_second` respected; 04 limits off by default; same headers.

## 8. Test and preview

- `POST /changelog-entries/{cle}/email-previews`: returns `{subject, preheader, html, text, size_bytes, over_budget, warnings}`; html with placeholder unsubscribe URL `#`.
- `POST /changelog-entries/{cle}/email-tests`: sends to caller member email only (session or first-party app; tokens 403 `session_required`-style code `member_required`), subject prefixed `[Test] `, no List-Unsubscribe One-Click (placeholder link), counts toward 04 stats, 10/hour per member (Valkey limiter). Same path as real send (`SendChangelogEmail`, `Purpose: entry`, customer stream, `Tags {test: true}`) so test proves sender + DKIM of real mailing; not a system-stream Mailable.

## Sources

- Gmail/Yahoo bulk sender requirements summaries: https://www.resend.com/blog/gmail-and-yahoo-bulk-sending-requirements-for-2024 , https://bird.com/docs/knowledge-base/deliverability/gmail-yahoo-requirements
- RFC 8058 One-Click unsubscribe: https://www.rfc-editor.org/rfc/rfc8058
- Featurebase subscribers (default auto-subscribe, CSV import): https://help.featurebase.app/articles/2705284-subscribing-users-to-changelogs
