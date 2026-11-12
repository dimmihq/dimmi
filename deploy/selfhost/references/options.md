# Optional settings

Set any of these with `dimmictl set` (secrets on stdin, see SKILL.md). Each
key is an environment variable in `/opt/dimmi/.env`. `APP` below is the app's
domain, such as `support.example.com`.

## Sending mail without SES

Two mailers exist: `system` (sign-in links, invitations, notifications) and
`customer` (replies and broadcasts). Set at least `system`; without
`customer`, customer mail uses it too. Every mailer needs a `DRIVER`.
Always set `DIMMI_MAIL__FROM__EMAIL` and `DIMMI_MAIL__FROM__NAME`.

Prefix for each key below: `DIMMI_MAIL__MAILERS__SYSTEM__` (or `__CUSTOMER__`).

| Driver     | Keys                                                                   |
|------------|------------------------------------------------------------------------|
| `smtp`     | `HOST`, `PORT` (587), `USERNAME`, `PASSWORD`, `TLS` (true for 465)     |
| `postmark` | `TOKEN` (server token), `MESSAGE_STREAM` (`outbound`)                  |
| `mailgun`  | `API_KEY`, `DOMAIN`, `ENDPOINT` (`https://api.eu.mailgun.net` for EU)  |
| `sendgrid` | `API_KEY`                                                              |
| `resend`   | `API_KEY`                                                              |
| `ses`      | `REGION`, `ACCESS_KEY_ID`, `SECRET_ACCESS_KEY`, `CONFIGURATION_SET`    |

Example, any SMTP relay:

```bash
dimmictl set <<'EOF'
DIMMI_MAIL__FROM__EMAIL=no-reply@mail.example.com
DIMMI_MAIL__FROM__NAME=Acme Support
DIMMI_MAIL__MAILERS__SYSTEM__DRIVER=smtp
DIMMI_MAIL__MAILERS__SYSTEM__HOST=smtp.example.net
DIMMI_MAIL__MAILERS__SYSTEM__PORT=587
DIMMI_MAIL__MAILERS__SYSTEM__USERNAME=(your relay username)
DIMMI_MAIL__MAILERS__SYSTEM__PASSWORD=(your relay password)
EOF
```

Verify the sending domain at the provider (SPF, DKIM, DMARC) first, as the
provider describes. `dimmi email:doctor <domain>` explains what is missing:
`docker compose -f /opt/dimmi/compose.yaml exec web /dimmi email:doctor mail.example.com`.

## Receiving mail without SES

Pick one inbound domain, such as `inbound.example.com`, point its MX records
at the provider, and have the provider post each message to dimmi. Every
option needs:

```
DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN=inbound.example.com
DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS=<driver>
DIMMI_MAILBOX__DEFAULT=<driver>
DIMMI_MAILBOX__SOURCES__<DRIVER>__DRIVER=<driver>
```

plus the driver's own keys (prefix `DIMMI_MAILBOX__SOURCES__<DRIVER>__`):

- **Postmark** (`postmark`): an inbound stream with "Include raw email
  content" ticked, webhook
  `https://<user>:<password>@APP/webhooks/postmark/inbound`. Make up a long
  random user and password (`openssl rand -hex 16`) and set them as
  `USERNAME` and `PASSWORD`. MX: the inbound domain to `inbound.postmarkapp.com`.
- **Mailgun** (`mailgun`): a receiving route for the inbound domain whose
  action forwards to `https://APP/webhooks/mailgun/inbound/mime` (the `mime`
  ending matters). `SIGNING_KEY` is the account's HTTP webhook signing key.
  MX: as Mailgun shows for the domain.
- **SendGrid** (`sendgrid`): Inbound Parse for the inbound domain with "POST
  the raw, full MIME message" ticked, URL
  `https://APP/webhooks/sendgrid/inbound`, and signed webhooks on.
  `PUBLIC_KEY` is the verification key SendGrid shows. MX: `mx.sendgrid.net`.
- **Resend** (`resend`): enable receiving on the domain, add a webhook for
  `email.received` to `https://APP/webhooks/resend/inbound`. Set
  `WEBHOOK_SECRET` (the webhook's signing secret, `whsec_...`) and `API_KEY`
  (dimmi fetches each message through the API). MX: as Resend shows.
- **Cloudflare Email Routing** (`cloudflare`): free when the domain's DNS is
  on Cloudflare. Make a secret (`openssl rand -hex 32`), set it as `SECRET`,
  then print the Worker that posts mail to dimmi:
  `docker compose -f /opt/dimmi/compose.yaml exec web /dimmi email:cloudflare-worker`.
  Deploy it as a Worker with the secret, and add a catch-all routing rule for
  the inbound domain that sends to the Worker.

Check with `... exec web /dimmi email:inbound-doctor`.

## AI

AI replies and suggestions need a model provider. Without any key, the
built-in provider is hidden and workspaces can still connect their own keys
in the app's settings. Install-wide keys:

```
DIMMI_AI__MANAGED__ANTHROPIC__API_KEY=sk-ant-...
DIMMI_AI__MANAGED__OPENAI__API_KEY=sk-...
DIMMI_AI__MANAGED__EMBEDDINGS__API_KEY=...       # semantic search of help articles
```

Embeddings default to DeepInfra's OpenAI-compatible API
(`DIMMI_AI__MANAGED__EMBEDDINGS__BASE_URL`, model `Qwen/Qwen3-Embedding-8B`,
1024 dimensions). Any OpenAI-compatible endpoint with a 1024-dimension model
works; set `__BASE_URL` and `__MODEL`. `DIMMI_AI__DEFAULT_MONTHLY_CAP_USD`
caps each workspace's spend (25 by default). `DIMMI_AI__ENABLED=false` turns
AI off. Check with `... exec web /dimmi ai:doctor`.

## Sign in with Google or GitHub

Create an OAuth app at the provider with the callback URL below, then set
its client id and secret. The button shows once a client id is set.

| Provider | Where                                        | Callback URL                      |
|----------|----------------------------------------------|-----------------------------------|
| Google   | Google Cloud console > APIs > Credentials > OAuth client (Web) | `https://APP/auth/google/callback` |
| GitHub   | GitHub > Settings > Developer settings > OAuth Apps            | `https://APP/auth/github/callback` |

```
DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_ID=...
DIMMI_AUTH__OAUTH__GOOGLE__CLIENT_SECRET=...
DIMMI_AUTH__OAUTH__GITHUB__CLIENT_ID=...
DIMMI_AUTH__OAUTH__GITHUB__CLIENT_SECRET=...
```

Sign-in still needs an invitation unless `DIMMI_AUTH__SIGNUP__MODE=open`,
which lets anyone with an account at the provider (or any email) sign up.

## Integrations

Each needs an OAuth app at the service with redirect
`https://APP/integrations/<name>/callback`, such as
`https://APP/integrations/slack/callback`:

```
DIMMI_SERVICES__SLACK__CLIENT_ID / __CLIENT_SECRET / __SIGNING_SECRET / __APP_ID
DIMMI_SERVICES__NOTION__CLIENT_ID / __CLIENT_SECRET
DIMMI_SERVICES__INTERCOM__CLIENT_ID / __CLIENT_SECRET
DIMMI_SERVICES__ZENDESK__CLIENT_ID / __CLIENT_SECRET
DIMMI_SERVICES__GITHUB__CLIENT_ID / __CLIENT_SECRET
```

Slack also needs its signing secret, and events and interactivity pointed at
`https://APP/webhooks/slack/events` and `https://APP/webhooks/slack/interactions`.

## Help centers

By default each help center is served at `https://APP/hc/<name>`; nothing to
set up. To give each one its own subdomain, such as `acme.help.example.com`:

1. DNS: `*.help.example.com` A record to the server.
2. Settings:

```
DIMMI_HELP_CENTERS__BASE_DOMAIN=help.example.com
DIMMI_CUSTOM_DOMAINS__DEFAULT=edge
DIMMI_CUSTOM_DOMAINS__TLS__EDGE__DRIVER=caddyask
```

Caddy then gets a certificate for each help center on its first visit, after
asking dimmi that the name belongs to a live help center.

## Malware scanning

Install with `--clamav`, or later:

```
dimmictl set COMPOSE_PROFILES=clamav DIMMI_MALWARE__DEFAULT=clamav \
  DIMMI_MALWARE__SCANNERS__CLAMAV__DRIVER=clamav DIMMI_MALWARE__SCANNERS__CLAMAV__ADDRESS=clamav:3310
docker compose -f /opt/dimmi/compose.yaml up -d clamav
```

ClamAV needs about 1.5 GB of RAM and takes a few minutes to load its
signatures. Without it, files are delivered marked as not scanned.

## Backups

Back up, off the server:

- `/opt/dimmi/.env`, once, and after each `set`. Without `DIMMI_APP__KEY` the
  encrypted data in the database cannot be read.
- The database, daily:

```bash
docker compose -f /opt/dimmi/compose.yaml exec -T postgres \
  pg_dump -U postgres -Fc dimmi > dimmi-$(date +%F).dump
```

- Files: the `dimmi_seaweedfs` volume (`/var/lib/docker/volumes/dimmi_seaweedfs`),
  or copy the bucket with any S3 tool through
  `docker compose ... exec seaweedfs weed shell`.

Restore the database with `pg_restore -U postgres -d dimmi --clean`, run
inside the postgres container, then `dimmictl start`.

## Help centers on customers' own domains

A workspace can serve a help center on its own domain, such as
`help.acme.com`. The app shows the team a CNAME (the domain points at the
target below) and a TXT record (`_dimmi.help.acme.com`, proving ownership),
checks both in the background, and serves the help center there once they
hold. Caddy orders the certificate on the first visit, after the app's ask
endpoint allows the domain; the bundled Caddyfile already asks.

Turn the ask endpoint on and name the CNAME target:

```bash
dimmictl set <<'EOF'
DIMMI_CUSTOM_DOMAINS__DEFAULT=edge
DIMMI_CUSTOM_DOMAINS__TLS__EDGE__DRIVER=caddyask
DIMMI_HELP_CENTERS__CUSTOM_DOMAIN_TARGET=APP
EOF
```

- The target defaults to `custom.<help domain>` with a help domain, else
  the app's own host. It must resolve to this server. A domain counts as
  pointed here when its CNAME is the target or it resolves to the
  target's addresses, so an A record on the target works best.
- Ports 80 and 443 must reach Caddy for every hostname, not only `APP`.
- Only subdomains can be added; a bare domain cannot carry a CNAME.
- Each help center also serves `/sitemap.xml`, `/robots.txt`, `/llms.txt`
  and `/llms-full.txt` on its own address.

## Other settings

- `DIMMI_AUTH__SIGNUP__MODE`: `invite_only` (default) or `open`.
- `DIMMI_APP__LOCALE`, `DIMMI_APP__LOCALES`: languages.
- `DIMMI_LOG__LEVEL`: `debug`, `info`, `warn`, `error`.
- `docker compose -f /opt/dimmi/compose.yaml exec web /dimmi config:show`
  prints the settings in effect, secrets masked.
