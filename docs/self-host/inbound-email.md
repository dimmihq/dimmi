# Receiving email without AWS

dimmi turns mail sent to its inbound domain into conversations. Something has to accept that mail over SMTP and
hand it to dimmi over HTTPS: dimmi never listens on port 25. Pick one provider for the inbound domain, point the
domain's MX records at it, and give dimmi the provider's secret.

Every provider ends in the same pipeline, so threading, quote stripping, spam handling and the activity log work
the same whichever you choose.

| You already have | Use |
|---|---|
| An AWS account | `ses` (receipt rule to S3 and SNS) |
| The domain on Cloudflare | `cloudflare` (Email Routing and a Worker, free) |
| Postmark, Mailgun, SendGrid or Resend for sending | the same provider for receiving |
| Only a mailbox, no domain to spare | an IMAP or Google or Microsoft mailbox connection instead |

## Common settings

```bash
DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=inbound.example.com
DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=postmark
DIMMI_MAILBOX__SOURCES__POSTMARK__DRIVER=postmark
DIMMI_MAILBOX__SOURCES__POSTMARK__USERNAME=hook
DIMMI_MAILBOX__SOURCES__POSTMARK__PASSWORD_FILE=/run/secrets/postmark_inbound_password
```

- `inbound.drivers` lists the providers whose webhook routes are open. A route for a provider not listed answers
  404 `inbound_driver_disabled`.
- Options live under `mailbox.sources.<name>`. Every secret also takes a `_FILE` variant.
- A listed provider without its secret stops `dimmi serve` at boot and names the missing key.
- `app.url` must be the public HTTPS address, because the provider posts to it.
- Webhook bodies reach 64 MiB. Let your reverse proxy pass bodies that large on `/webhooks/*`, unmodified.
- `channels.email.inbound.parse_memory_budget_mb` (default 64) bounds how much mail is held in memory at once.
  When it is full, dimmi answers 503 and the provider retries.

Run `dimmi email:inbound-doctor` after setup. It checks the MX records, the keys of each provider, the last mail
each provider delivered, and the last setting a provider reported missing, and it says what to fix.

## Postmark

1. In your Postmark server, open the inbound stream and set the webhook URL to
   `https://hook:<password>@<your host>/webhooks/postmark/inbound`.
2. Tick "Include raw email content in JSON payload". Without it dimmi answers 503 so Postmark keeps the mail, and
   the doctor names the setting.
3. Point the inbound domain's MX at `inbound.postmarkapp.com` (priority 10) and add the domain as the stream's
   inbound domain.
4. Set `DIMMI_MAILBOX__SOURCES__POSTMARK__USERNAME` and `__PASSWORD` to the user and password of the URL.

Wrong credentials answer 403, which tells Postmark to stop retrying.

## Mailgun

1. Create a route: expression `match_recipient(".*@inbound.example.com")`, actions
   `forward("https://<your host>/webhooks/mailgun/inbound/mime")` and `stop()`. The URL must end in `mime`, or
   Mailgun does not send the raw message.
2. Point the MX at `mxa.mailgun.org` and `mxb.mailgun.org` (EU accounts: `mxa.eu.mailgun.org`, `mxb.eu.mailgun.org`).
3. Set `DIMMI_MAILBOX__SOURCES__MAILGUN__SIGNING_KEY` to the HTTP webhook signing key.

A bad or replayed signature answers 406, which Mailgun does not retry. Anything else it retries for eight hours.

## SendGrid

1. Add an Inbound Parse host for the inbound domain with URL `https://<your host>/webhooks/sendgrid/inbound`, and
   tick "POST the raw, full MIME message".
2. Attach a signature security policy to the parse setting and copy its public key.
3. Point the MX at `mx.sendgrid.net` (priority 10). SendGrid only receives on a subdomain.
4. Set `DIMMI_MAILBOX__SOURCES__SENDGRID__PUBLIC_KEY` to the public key.

The signature covers the body exactly as sent: a proxy that rewrites bodies breaks it.

## Resend

1. Add a receiving domain in Resend and point its MX at the value Resend shows.
2. Add a webhook for the event `email.received` to `https://<your host>/webhooks/resend/inbound` and copy its
   signing secret. Create an API key that can read received emails.
3. Set `DIMMI_MAILBOX__SOURCES__RESEND__WEBHOOK_SECRET` and `__API_KEY`.

The webhook carries only metadata; dimmi fetches the raw message from Resend's API. Resend keeps the mail it
received, so after an outage run `dimmi email:resend-backfill --since 2026-11-01` to feed it through
again. Mail dimmi already holds is kept once.

## Cloudflare

1. Turn on Email Routing for the inbound domain. Cloudflare sets the MX records (`route1`, `route2`, `route3`
   `.mx.cloudflare.net`).
2. Run `dimmi email:cloudflare-worker`. It prints `worker.js` and `wrangler.toml` with your webhook URL filled in.
3. Deploy the Worker with `wrangler deploy`, then set its secrets: `wrangler secret put SECRET` (the same value as
   `DIMMI_MAILBOX__SOURCES__CLOUDFLARE__SECRET`) and, recommended, `FALLBACK_ADDRESS`, a verified destination
   address.
4. Add a catch-all routing rule on the inbound domain that sends to the Worker. Cloudflare allows 200 rules per
   domain, so do not add one per address.

The Worker retries three times. When dimmi stays unreachable it forwards the mail to `FALLBACK_ADDRESS`, or, with
none set, rejects it so the sender gets a bounce instead of silent loss. Cloudflare rejects mail over 25 MiB.

## Amazon SES

See the SES receipt rule steps in the operations guide: a receipt rule stores the message in S3 and notifies an
SNS topic that posts to `/webhooks/ses/inbound`.
