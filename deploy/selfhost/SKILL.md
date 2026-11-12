---
name: install-dimmi
description: Install, configure and update a self-hosted dimmi (customer support app) on one Linux server with Docker Compose. Use when someone asks to self-host dimmi, set up its domain, email (Amazon SES or another provider), AI keys or sign-in apps, or to upgrade or troubleshoot an install.
---

# Install dimmi on a server

You are helping someone run dimmi on their own server. The whole stack is one
Docker Compose project: the app (web and worker), Postgres with pgvector,
Valkey, SeaweedFS for files and Caddy for HTTPS. Only one public domain is
needed, the app's own, such as `support.example.com`. Files, help centers and
webhooks are all served from it.

Everything goes through one script, `install.sh`. After the first run it is
installed as `dimmictl` on the server.

## What to ask first

Ask only what you cannot find yourself:

1. **The server.** A fresh Ubuntu 22.04/24.04 or Debian 12 machine with a
   public IPv4, at least 2 vCPU, 4 GB RAM and 40 GB disk (add 2 GB of RAM for
   malware scanning). Root over SSH, or a sudo user.
2. **The domain** dimmi will live on, and who controls its DNS.
3. **An email** for Let's Encrypt notices.
4. **The first account**: name, email and workspace name.
5. Later, only if they want them: email (AWS or another provider), AI keys,
   Google or GitHub sign-in.

## How to work

Pick one mode and say which:

- **You run it over SSH** (preferred when you have a shell and they give you
  `ssh user@host`). Run each command below through `ssh`, show the output,
  and stop on errors.
- **They run it.** Give them one command at a time in a code block, ask them
  to paste back the output, and read it before the next step.

Never print secrets from `/opt/dimmi/.env` back into the chat. Never put AWS or
AI keys on a command line someone else will see; send them on stdin (see
"Changing settings").

## Steps

### 1. DNS first

Find the server's public IPv4 (`curl -4 https://api.ipify.org` on the server).
Ask them to add, at their DNS provider:

```
Type: A    Name: support.example.com    Value: <server IPv4>    TTL: 300
```

If the DNS is on Cloudflare, the record must be "DNS only" (grey cloud) so
Caddy can get its certificate. Check it before moving on:
`dig +short A support.example.com` must print the server's IP. Installing
before DNS is ready works too; HTTPS starts as soon as the record resolves.

### 2. Install

On the server, as root:

```bash
curl -fsSL https://raw.githubusercontent.com/dimmihq/dimmi/main/deploy/selfhost/install.sh \
  | bash -s -- --domain support.example.com --email ops@example.com
```

Over SSH as a sudo user, pipe to `sudo bash -s -- ...` instead. Add
`--clamav` to scan uploads and email attachments for malware.

It takes 3 to 5 minutes. It:

- updates the system, installs Docker, `ufw`, `fail2ban` and automatic
  security updates, adds 2 GB of swap if there is none, and opens only SSH,
  80 and 443. It turns off SSH password logins only when a key can already
  log in. Skip all of this with `--skip-prepare` on a server you manage
  yourself (Docker with the compose plugin must be installed).
- writes `/opt/dimmi` with `compose.yaml`, `Caddyfile` and `.env`. `.env`
  holds random passwords for Postgres, Valkey and SeaweedFS and the app key.
  They are made once and kept on every later run.
- starts the stack. A one-off `migrate` service applies the database
  migrations before the app starts, on every start.
- checks the DNS record and `https://<domain>/up`.

Safe to run again with the same flags at any time.

### 3. Create the first account

New installs are invite-only: nobody can sign up from the web page. Create
the owner, then they invite everyone else from the app:

```bash
dimmictl admin --email owner@example.com --name "Ada Lovelace" --workspace "Acme"
```

It prints a generated password once. Tell them to sign in at
`https://<domain>` and change it. Treat it as a secret: pass it straight on,
don't repeat it later.

### 4. Check it

```bash
dimmictl status      # every service Up, web and worker (healthy)
dimmictl check       # DNS, certificate, /up
```

Done when `https://<domain>` shows the sign-in page with a valid certificate
and the owner can sign in.

Until email is set up, dimmi writes each email with its links to the logs
instead of sending it. Find an invite or sign-in link with
`dimmictl logs web worker | grep -i http`.

## Changing settings

Every setting is an environment variable in `/opt/dimmi/.env`: `DIMMI_`
plus the setting's path in upper case with `__` between levels, so
`mail.from.name` is `DIMMI_MAIL__FROM__NAME`. `.env.example` beside it lists
the common ones.

```bash
dimmictl set DIMMI_MAIL__FROM__NAME="Acme Support"
```

For secrets, send `KEY=value` lines on stdin so they stay out of the shell
history and the process list:

```bash
dimmictl set <<'EOF'
DIMMI_AI__MANAGED__ANTHROPIC__API_KEY=sk-ant-...
EOF
```

`set` saves the values and restarts the app. If it does not come back
healthy, the app explains what is wrong: `dimmictl logs web`.

Optional features and their settings:

- **Email** (sending and receiving support mail): `references/aws-ses.md` for
  Amazon SES, `references/options.md` for SMTP, Postmark, Mailgun, SendGrid,
  Resend and Cloudflare.
- **AI, Google or GitHub sign-in, Slack and other integrations, help-center
  subdomains, malware scanning, backups:** `references/options.md`.

## Running it

```bash
dimmictl update          # newest stack files and image, migrate, restart
dimmictl logs [service]  # follow logs: web, worker, caddy, postgres...
dimmictl status
```

Data lives in Docker volumes (`dimmi_postgres`, `dimmi_seaweedfs`, ...).
`/opt/dimmi/.env` is the other half: without its `DIMMI_APP__KEY` encrypted
data cannot be read. Back up both (see `references/options.md`).

## When something fails

- **`/up` fails, DNS is right:** `dimmictl logs caddy`. Let's Encrypt needs
  ports 80 and 443 open from the internet, including any cloud firewall in
  front of the server, and the A record must not be proxied. Too many failed
  attempts are rate limited for an hour.
- **`migrate` failed:** `docker compose -f /opt/dimmi/compose.yaml logs migrate`.
- **web unhealthy after `set`:** the log names the setting that cannot work.
  Fix it with `set` again.
- **Out of memory:** `free -m`. Drop `--clamav` or use a bigger server.

## Files

- `install.sh`: the installer and `dimmictl`.
- `compose.yaml`, `Caddyfile`, `postgres-init.sh`: the stack.
- `.env.example`: the settings, explained.
- `references/aws-ses.md`: AWS setup for sending and receiving mail.
- `references/options.md`: other mail providers, AI, sign-in, integrations,
  help-center domains, malware scanning, backups.
