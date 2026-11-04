# 04 Email channel: inbound sources

Style: caveman. Owner: 04-email-channel. Self-hosters receive without AWS. Every source ends in same pipeline ([pipeline.md](pipeline.md)). `framework/mailbox` owns drivers; no product words there. Tasks: [tasks.md](tasks.md) M5, M17, M18. Track 06 email-inbound.

Facts verified on provider docs 2026-11-04. Marked `[staging]` = not stated in docs read; pinned by fixture; first staging run confirms ([staging.md](staging.md) 5).

## 1. Framework change

Today `Receiver.Receive(ctx, body []byte)`. Provider webhooks need headers, query, and bodies up to tens of MB. Change (delta to framework.md 3.2):

```go
type Delivery struct {
    Header http.Header
    Query  url.Values
    Body   io.Reader // capped by controller at source's MaxWebhookBytes
    Size   int64     // Content-Length, -1 unknown
}
type Receiver interface {
    Source
    Receive(ctx context.Context, d Delivery) (*Envelope, error)
    MaxWebhookBytes() int64
}
// optional: provider-specific answer for an error; default mapping below
type StatusMapper interface { Status(err error) int }
```

Default status: nil 200; `ErrRejected` 401; `ErrTooLarge` 200 with envelope marked truncated when raw was stored partly, else 413; other 503. Provider needing another code implements `StatusMapper`.

Drivers buffer raw body only under weighted semaphore shared with `parse_memory_budget_mb` (weight = 2 x `Size`, wait 2 s, else 503 so provider retries). Never hold over 64 MB. Signature checked before parse. Replay key written after raw is persisted, so a failed attempt can retry.

Generic controller `app/http/controllers/webhooks/mailbox_controller.go` `Store(source)` for all webhook sources: resolve `a.Mailbox.Receiver(name)` (disabled: 404 `inbound_driver_disabled`), call `Receive`, then `actions/email.ReceiveInboundEmail`. Raw saved to `inbound.raw_disk` before 200 (bar 10): Envelope with no `RawRef` is copied during request, under 20 s deadline; failure 503. SES keeps `RawRef`.

Routes (`routes/webhooks.go`), one per enabled driver:

| Path | Source | Body |
|---|---|---|
| `POST /webhooks/ses/inbound` | ses | SNS JSON |
| `POST /webhooks/postmark/inbound` | postmark | JSON |
| `POST /webhooks/mailgun/inbound/mime` | mailgun | form-urlencoded (Mailgun sends raw only to URLs ending `mime` or `raw-mime`) |
| `POST /webhooks/sendgrid/inbound` | sendgrid | multipart |
| `POST /webhooks/resend/inbound` | resend | JSON metadata |
| `POST /webhooks/cloudflare/inbound` | cloudflare | `message/rfc822` |

`/webhooks/{provider}/...` is rest.md exception; Mailgun's extra segment is provider-defined.

`inbound_emails.source`: migration `20261201060001 widen_email_source_and_transport`: drop CHECK, add format CHECK (registry enum): `ses`, `postmark`, `mailgun`, `sendgrid`, `resend`, `cloudflare`, `imap`, `gmail`, `microsoft`, `test`. Same migration widens `outbound_emails.transport` CHECK with `gmail`, `microsoft`.

Provider message id: when provider has none stable across retries: `sha256(raw)[:32]`, so retry dedupes and same mail to two workspaces still gets own row.

Recipients: envelope when provider gives one; else To + Cc + Bcc filtered to inbound domain. Router matches inbound domain only; other recipients ignored.

Verdict trust (`mailbox.Verdicts`): provider saw the connection, so trust its fields, parsed by driver only. Own DKIM + DMARC (`authcheck`) still run; A-R headers from senders never trusted.

## 2. Providers

Common: operator points inbound domain MX at provider, sets webhook URL, adds secret to `mailbox.sources.<name>`. `dimmi doctor` checks ([../11-self-host/operations.md](../11-self-host/operations.md) section 15). Same provider may also be outbound driver; independent config.

### 2.1 postmark

| Item | Value |
|---|---|
| Setup | Inbound stream webhook URL `https://user:pass@host/webhooks/postmark/inbound`; tick "Include raw email content in JSON payload" |
| MX | `inbound.postmarkapp.com` priority 10 `[staging]` |
| Auth | HTTP basic in URL (Postmark has no inbound signature); constant-time compare |
| Raw | `RawEmail` field; `RawEmail` absent: 503 (keep mail retrying) + `mailbox.misconfigured` health red + doctor names setting |
| Envelope | `OriginalRecipient` then `ToFull`, `CcFull`, `BccFull`; `MessageID` Postmark id |
| Verdicts | `Headers[]` `X-Spam-Status` (Spam), `Received-SPF` (SPF) `[staging]`; DKIM own |
| Size | attachments 35 MB total; base64 in JSON so `MaxWebhookBytes` 64 MiB |
| Retry | any non-200 retried 10 times over about 10 h (1, 5, 10, 10, 10, 15, 30, 60, 120, 360 min); **403 stops retries**. Bad auth: 403 via `StatusMapper` |

### 2.2 mailgun

| Item | Value |
|---|---|
| Setup | Route `match_recipient(".*@<inbound_domain>")` action `forward("https://host/webhooks/mailgun/inbound/mime")`, `stop()` |
| MX | `mxa.mailgun.org`, `mxb.mailgun.org` (EU `.eu.`) `[staging]` |
| Auth | form `timestamp`, `token`, `signature`: HMAC-SHA256 of timestamp + token with signing key; 5 min window; replay key `mailbox:replay:mailgun:{token}` |
| Raw | `body-mime` (form-urlencoded, no separate attachments) |
| Envelope | `recipient`, `sender`; id = hash of raw |
| Verdicts | raw headers `X-Mailgun-Spf`, `X-Mailgun-Dkim-Check-Result`, `X-Mailgun-Sflag`, first occurrence only `[staging]` |
| Retry | 200 ok; **406 no retry**; other retried 8 h. Bad signature 406 via `StatusMapper` |

### 2.3 sendgrid

| Item | Value |
|---|---|
| Setup | Inbound Parse host `<inbound_domain>`, URL `/webhooks/sendgrid/inbound`, tick "POST the raw, full MIME message"; security policy attached with `PATCH` on parse setting (`security_policy`) |
| MX | `mx.sendgrid.net` priority 10; subdomain only |
| Auth | Security policy signature: ECDSA, headers `X-Twilio-Email-Event-Webhook-Signature`, `X-Twilio-Email-Event-Webhook-Timestamp`, over raw body exactly as received; public key `mailbox.sources.sendgrid.public_key`. Whether timestamp is in signed content: `[staging]`; driver tries `timestamp + body` per event webhook pattern, fixture pins it. OAuth option unused |
| Raw | multipart field `email`; controller spools raw body then verifies then walks parts |
| Envelope | `envelope` JSON `{to:[...], from}` |
| Verdicts | `SPF`, `dkim`; `spam_score` only when spam check on and message under 2.5 MB |
| Size | SendGrid guidance 30 MB; `MaxWebhookBytes` 64 MiB |
| Retry | non-2xx retried by SendGrid `[staging]` window |

### 2.4 resend

| Item | Value |
|---|---|
| Setup | Receiving domain or `.resend.app` address in Resend; webhook event `email.received` to `/webhooks/resend/inbound`; API key with read access |
| MX | value shown in Resend dashboard (doctor checks only presence) |
| Auth | Svix headers, HMAC-SHA256, 5 min window, `svix-id` replay key; webhook secret per source (not outbound events secret) |
| Raw | webhook carries metadata only. Driver calls `GET /emails/receiving/{id}`: `raw.download_url` (signed, `raw.expires_at`), then GET through `a.HTTP`. `raw` null: 503 so Svix retries |
| Envelope | webhook `data.to`, `cc`, `bcc`, `from`; `email_id` id; To is header-derived so filter by inbound domain |
| Verdicts | none documented; own DKIM + DMARC; SPF unknown |
| Note | Resend stores mail, so outage on our side recoverable; `dimmi email:resend-backfill --since` lists and re-feeds |

### 2.5 cloudflare

Cloudflare Email Routing hands mail to a Worker, no webhook of its own.

- `dimmi email:cloudflare-worker` prints `worker.js` plus `wrangler.toml` (embedded `app/integrations/email/cloudflareworker/`), URL filled. Operator deploys; routing rule catch-all on inbound domain to Worker. Cloudflare limit: inbound over 25 MiB rejected at SMTP; 200 rules per domain, so catch-all not per address.
- Worker: stream `message.raw` to `ArrayBuffer`, POST `/webhooks/cloudflare/inbound` with `Content-Type: message/rfc822`, `X-Dimmi-Timestamp`, `X-Dimmi-Signature` = HMAC-SHA256(secret, timestamp + "." + sha256(body)), `X-Dimmi-Envelope-From` = `message.from`, `X-Dimmi-Envelope-To` = `message.to`. Web Crypto only, no dependency. Retry 3 times, 1 s apart.
- All retries failed: `message.forward(env.FALLBACK_ADDRESS)` when set (verified destination); else `message.setReject("temporarily unavailable, try again later")`. Sender gets bounce, never silent loss. Documented risk: permanent SMTP error; fallback address recommended, `dimmi email:doctor` warns when unset.
- MX: `route1/2/3.mx.cloudflare.net` `[staging]`.
- Verdicts: first `Authentication-Results` header only when it names `mx.cloudflare.net`: DMARC + others. Worker sends raw unchanged. Driver rejects extra leading A-R of other authserv.
- Secret: `mailbox.sources.cloudflare.secret`. 5 min window, replay key on signature.

Size 25 MiB means message over limit bounces at Cloudflare; guide says so.

### 2.6 Choosing

| Box has | Pick |
|---|---|
| AWS | ses |
| Domain on Cloudflare, no budget | cloudflare |
| Already on Postmark, Mailgun, SendGrid, Resend for outbound | same provider inbound |
| None; own mailbox only | IMAP (section 3) or OAuth connection, no inbound domain needed |

Inbound domain empty and only mailbox connections: allowed. `forwarding_address` stays not null unique: such address stores `<local>@no-inbound.invalid` (RFC 2606 reserved, never routes). Resource `forwarding_available: false`; UI hides forward path, test uses mailbox. `inbound_not_configured` now applies only to forwarding-only create.

### 2.7 Tests

`framework/mailbox/drivertest` every driver: identical envelope + raw bytes from fixture; bad signature rejected; replay refused (where keyed); oversize; same id on retry. App feature test per driver through route with real Valkey, real disk, real pipeline to message. Fixture payloads in `framework/mailbox/drivers/<name>/testdata`, built from provider doc examples, refreshed from staging capture. Resend raw fetch via `httpclienttest`. Worker: golden signed request test; JS not run in Go tests; `tests/e2e/support` not involved.

Reuse check: none new for webhooks (stdlib, `framework/httpclient`, Svix check already in `mail` provider events: move to `framework/webhooks` shared helper if not there).

## 3. IMAP adapter

Per-address connection, not instance. Row in `email_mailbox_connections` (`provider = imap`) ([mailbox-connections.md](mailbox-connections.md) 2), `email:manage`. Works on cloud and self-host; config `channels.email.imap.enabled` default true self-host, true cloud.

Reuse check: `github.com/emersion/go-imap/v2` (MIT; latest seen v2.0.0-beta.8; only maintained Go IMAP client; client has `Idle`, `Status`, `Fetch` by UID). Beta: pin exact version, thin wrapper `framework/mailbox/imap`, hides library types; add to depguard framework allowlist. go-sasl already allowed.

Decisions:

- **Poll every 60 s, no IDLE.** River job `SyncEmailMailbox` per connection, unique by id; `ScheduleEmailMailboxSyncs` each minute. Why: no long-lived socket per mailbox, no supervisor or leases, any replica runs it, 256 MB box stays small. Cost: up to 60 s extra latency; bar for IMAP is p95 under 90 s. IDLE left out (reported).
- Auth: app password only, sealed by `encryption.Seal` scope workspace. Presets: Gmail (`imap.gmail.com`), Fastmail, iCloud (`imap.mail.me.com`), Zoho, Yahoo, custom. Microsoft: no IMAP password path; UI sends to OAuth.
- TLS: 993 implicit or 143 STARTTLS required. Plain refused. Host through `safedial`: private and loopback ranges refused unless `channels.email.imap.allow_private_hosts` (self-host only). DNS rebinding covered by safedial.
- Read: `SELECT INBOX` read-only `EXAMINE`; `UID FETCH` with `BODY.PEEK[]`: never sets `\Seen`, never deletes, never moves. Sent via `SPECIAL-USE \Sent` (`LIST`), fallback names `Sent`, `Sent Items`, `Sent Messages`; none: Sent sync off, shown.
- Cursor per folder: `{uidvalidity, last_uid}`. Start at `UIDNEXT` on connect: no history import. `UIDVALIDITY` change: rescan `SINCE` last sync minus 1 day, dedupe on `email_mailbox_messages`.
- Batch 50 per run, oversize over `max_message_bytes`: header + `BODYSTRUCTURE` only, truncated path (`pipeline.md` 4).
- Verdicts: none trusted. Own DKIM + DMARC. SPF unknown. So `auth.result` pass only with aligned DKIM.
- Write-back: `APPEND` dimmi's sent MIME to `\Sent`, flag `\Seen`, after send succeeds (`imap_append_sent` setting default true). Gmail IMAP keeps one copy by Message-ID. APPEND failure: logged, metric, no effect on message. Only write ever made.
- Sending: never through IMAP host SMTP (BYO SMTP stays out). Replies go through instance provider under `own_domain` or `default_sender` rule. UI says it up front.
- Failure: 3 auth failures in a row: `reauth_required`, stop polling (avoid account lockout), notify admins. Network errors: backoff doubling to 30 min, status stays `active`, `last_error`, `degraded` after 24 h.
- Concurrency: one connection per mailbox per run, closed at end. Provider limits respected by 60 s floor. `channels.email.imap.max_concurrent` 10 per node (semaphore).

API: `POST /email-mailbox-connections` `{provider: "imap", email_address_id, host, port, username, password, preset?}`. Live check first (login, list folders) sync, under 10 s: 422 with `imap_auth_failed`, `imap_tls_required`, `imap_host_blocked`, `imap_unreachable`, plain copy per host (Gmail: "Use an app password, not your normal password" + link).

Tests: real IMAP server available? Testing rule: no fake where can run real; external program. Add `greenmail` service? Decision: use go-imap's in-process `imapserver` + `imapmemserver` from same module in test helper (real protocol over loopback, no extra container). Covers read-only, cursor, UIDVALIDITY change, APPEND, TLS refused, auth failure lockout stop, safedial refusal.
