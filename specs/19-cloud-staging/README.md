# 19 Cloud staging

**Approved by the owner, 2026-11-04.** Built in `deploy/` and run by hand through [`docs/deploy`](../../docs/deploy/README.md).
Nothing has been created in any account yet. Research, options and sources:
[`research/aws-staging-deployment.md`](../../research/aws-staging-deployment.md). Work list: [tasks.md](tasks.md).

Compute is ECS Fargate on ARM behind a Network Load Balancer, for staging and production alike. All DNS is at Vercel, written
by a script. There is no ALB, no EC2 fleet, no Kubernetes, no Kamal and no Route 53.

The repository is public, so account ids, secret values and environment-specific identifiers never go in it.

## Goal

A staging environment used exactly like production, with everything real on tiny instances: the cloud edition only, deployed on
every push to `main`, real Valkey, real DNS on `trydimmi.com`, real mail both ways through Amazon SES, defined with AWS CDK in
TypeScript at `deploy/cdk`, documented in `docs/deploy` including manual steps, so everything we have built can be tested by hand.

## Decided by the owner

### One AWS account

No Organization and no separate staging account. Staging and production share one account. What that costs, plainly:

- **SES must be split by Region.** Sandbox status, quotas, verified identities, Easy DKIM, the suppression list, receipt rules and
  feedback settings are per account and Region, so staging's SES runs in a different Region from production's. AWS describes
  account status and a sending pause as applying to the current Region, but no page read says a review in one Region leaves
  another untouched, so that isolation is likely rather than promised.
- **Reputation is less isolated than with two accounts.** It is the same customer to AWS. A serious abuse finding could reach beyond
  one Region. Staging volume stays low, and recipients stay known.
- **Production access is requested per Region,** so staging's request does not carry over to production.
- **IAM is global.** Roles and policies are scoped by resource prefix (`dimmi-staging-*`), tag (`Environment=staging`) and
  `aws:RequestedRegion`. The default `cdk bootstrap` execution role is effectively administrator, so it is given a narrower policy.
  A mistake made as an administrator can still reach production.
- **Bill and alarms are shared.** Tags separate cost; budgets filter on the tag once it is activated for billing. Alarm names carry the environment.
- **Quotas are shared** where they are per account rather than per Region.

The existing account is used. The root user gets MFA and no access keys, people sign in through IAM Identity Center or a named role.

### One host

`staging.trydimmi.com` serves everything from one binary: dashboard, help center, marketing, changelog. There is no `app.staging`.
`app.url`, cookies (`__Host-`) and the passkey relying party all use `staging.trydimmi.com`, so passkeys belong to staging only.

Vercel stays registrar and DNS for `trydimmi.com`, and holds every record of every environment. There is no NS delegation and
no Route 53 zone. `deploy/dns` reads an environment's stack outputs and writes its records through Vercel's API (`plan`,
`apply`, `verify`, `remove`), touching only records whose comment marks them as its own for that environment.

| Name | Records | Is it a website? |
|---|---|---|
| `staging.trydimmi.com` | A records to the load balancer's two Elastic IPs | **Yes, the only one** |
| `inbound.staging.trydimmi.com` | MX to SES receiving, DKIM CNAMEs, `_dmarc` | No, DNS only |
| `bounce.inbound.staging.trydimmi.com` | MX to SES feedback, SPF TXT | No, DNS only |
| `mail.staging.trydimmi.com` | MX to SES receiving, DKIM CNAMEs, `_dmarc` | No, DNS only |
| `bounce.mail.staging.trydimmi.com` | MX to SES feedback, SPF TXT | No, DNS only |

Nothing in the code needs another host today:

- The help center, changelog and portal are built only in path mode, `/hc/{center}` (`routes/web.go`). `help_center.base_domain`
  exists in spec 08 only, where empty means path mode. Staging leaves it empty, so **no wildcard record is needed**.
- Assets (`/build/*`, `/widget/*`) come from the same origin. Marketing is not built; `/` redirects to `/d` (`routes/web.go:48`) and
  will move to the marketing page on this same host.
- One thing to know: spec 08 line 1178 keeps cloud help centers on a separate registrable domain so user content never shares
  cookies with the dashboard. Staging's single host does not test that. If host mode is built and needs testing, it takes
  `*.help.staging.trydimmi.com` pointing at the compute, with on-demand certificates. Decision 3 settles production.

### The binary terminates TLS

The binary serves HTTPS on 443 and HTTP on 80 itself, through `framework/customdomain` (certmagic), with certificates in PostgreSQL.
The same path runs on staging and production. An ALB is ruled out for production because it terminates TLS; `r.TLS` is also what the
portal and help center read to build origins and set `Secure` (`help_center_controller.go:188`, `portal_controller.go:211`), which
stays correct when the binary holds the connection.

### Compute and edge: Fargate behind an NLB with TCP passthrough

Both environments run ECS Fargate tasks on ARM behind a Network Load Balancer, TCP on 443 and 80, so the binary still sees TLS and
answers with certificates from the database. The binary listens on 8443 and 8080 inside the container; the target groups map the
public ports onto them, so nothing binds a low port. Each zone's load balancer node has an Elastic IP, so DNS is plain A records,
which also work at production's apex. Staging runs one task with every role; production runs several web tasks and a separate
worker and scheduler service. Tasks sit in public subnets with public addresses (no NAT) and keep nothing on disk.

Deploys: one image per commit, tagged in ECR; `release` is the tag every task definition runs. A release points it at the new
image, runs a one-off `provision` task (database roles, idempotent) and `migrate --force` as the schema owner, then forces a new
deployment of each service: 100% minimum and 200% maximum, deployment circuit breaker with rollback. ECS pins the image digest per
deployment, so a rollback returns to exactly what ran. CI never changes infrastructure; people deploy CDK. Checked against AWS docs,
research section 13:

| Question | Answer |
|---|---|
| Client IP, instance targets | Preserved by default, so no `trusted_proxies` |
| Client IP, IP targets (Fargate registers this way) | **Off by default for TCP**. Decided: turn on `preserve_client_ip`, so the binary reads the real address from the connection; no proxy protocol, no `trusted_proxies` |
| Hairpin | Not supported with preservation: a target calling its own NLB can fail, so the app calls its own URL only with care |
| ACME TLS-ALPN and HTTP-01 | TCP passthrough carries both to whichever replica; certmagic solves across replicas through shared storage. AWS does not mention ACME; this follows from layer 4 |
| WebSocket and SSE | A TCP connection stays on one target for life. AWS does not name them; no HTTP handling is in the path |
| Idle timeout | 350 s by default, 60 to 6000 s. Realtime pings every 25 s |
| Draining | Deregistration default 300 s, AWS advises at least 120 s. Open connections are not closed unless termination is enabled, so shutdown must tell clients to reconnect, which it does |
| Health checks | HTTP, HTTPS or TCP are allowed on TCP target groups. Use HTTP `/ready` on the plain port, which must not redirect |
| Cost | $0.0225 per hour, about $16.43 per month, plus $0.006 per NLCU-hour |

Deregistration delay is 120 s. Hairpinning is not supported with client IP preservation, so a task never calls its own host through
the load balancer.

### Data stores

Real RDS PostgreSQL 18 on `db.t4g.micro`, and real ElastiCache Valkey on `cache.t4g.micro` with TLS and AUTH, in isolated subnets with no NAT.
The app connects as `dimmi_app`, which owns nothing and has neither superuser nor `BYPASSRLS`, because row-level security does not apply to
table owners. Migrations run as a one-off Fargate task as `dimmi_owner`, with `migrations.auto=false` everywhere else. A `provision` task
creates both roles from Secrets Manager passwords with `deploy/cdk/lib/database/provision.sql`. Connection strings are composed by
CloudFormation from secret references and verify the server against the RDS bundle in the image.

### Gate

The host must be public, since SNS and Let's Encrypt have to reach it, and there is no load balancer rule to lean on. So the gate lives in
the app: a small generic middleware, enabled by configuration, off by default.

- `app.env` is `production` on staging, so the switch is its own setting, not the environment. A production smoke test asserts it is off.
- Runs as global kernel middleware. A person enters a shared password once and gets a signed `__Host-` cookie.
- CI passes with a key header, compared in constant time. Password and key are secrets.
- Exempt: `/webhooks/ses/*` (SNS signature), `/widget/*` (loaded from other sites, and the widget test needs it), `/robots.txt`.
- Already outside it: `/up`, `/ready`, `/connection/*` are mounted before the kernel (`framework/foundation/serve.go`) and realtime authenticates
  itself; ACME challenges are answered first by the certificate handler.
- Search engines: `/robots.txt` disallows all and an `X-Robots-Tag: noindex` header goes on every response. The app shell already sends `noindex`. Certificate
  logs make the name public anyway, so the gate, not secrecy of the name, is the protection.
- Sign-up stays open behind the gate, so that flow can be tested. Sending limits (spec 04 M11) must exist before the gate is ever removed.

## Email

Region-independent except for the endpoints, which follow decision 1.

- Domain identities for `inbound.staging` and `mail.staging`, Easy DKIM with records written to Vercel by `deploy/dns`, custom MAIL FROM with reject on MX failure so a mistake is loud.
- Configuration sets `dimmi-system-staging` and `dimmi-customer-staging`, events to one SNS topic, as spec 04 wants.
- Receipt rules: `inbound.staging` to S3 `raw/` with SNS, scanning on (the API default is off); `mail.staging` to S3 `system/`. CDK cannot activate a rule set, so a
  small custom resource does. Leave SES message encryption off: it is client-side and our driver cannot read it. Use bucket encryption.
- SNS to `https://staging.trydimmi.com/webhooks/ses/inbound` and `/events`, about one hour of retries (the default is 3 retries, 20 s apart) and a dead-letter queue. Raw mail stays in S3 and can be replayed.
- The app must answer SNS within about 15 s, and SNS needs a certificate from a CA it trusts. AWS publishes no list on the page read; Let's Encrypt is proved at first subscription.
- Scheduled round trip every 6 hours through `POST /email-addresses/{id}/tests`, with the Gmail test mailbox forwarding to dimmi, and a failing workflow alerts. Confirm the API exposes the arrival for polling.
- Tenants: none until spec 04 M8 and M11; the app creates them.
- Reputation: separate Region and subdomains, low volume, known recipients, simulator for failures. No one can promise staging never affects `trydimmi.com`; secondary sources disagree on subdomain signals.

## Testing staging end to end

Built features only. Test mailboxes: one Gmail, one Outlook.com. No seed command exists or is needed at first; real sign-up creates a real workspace.

| Feature | Check |
|---|---|
| TLS | First visit gets a Let's Encrypt certificate; survives a restart with no new order; port 80 redirects; ACME renewal path works on the staging CA first |
| Gate | Blocked without password; SNS, widget and health pass; CI key passes; crawler sees disallow and `noindex` |
| Sign-up mail | Arrives, not spam; SPF, DKIM, DMARC pass, aligned on `mail.staging` |
| Verify, magic link, password reset | Each link works once |
| Invites | Invite Outlook address, accept, role applied, withdrawn invite refused |
| OAuth | GitHub and Google to `https://staging.trydimmi.com/auth/{provider}/callback`, staging's own clients |
| 2FA | TOTP in a real app, recovery code, workspace rule |
| Passkeys | Register on the real host with Touch ID and a phone |
| Sessions | Revoke another browser; its socket closes |
| Inbox | Create, reply, note, assign, close, search |
| Realtime | Two browsers: reply shows without reload, wss over HTTP/1.1 works beside HTTP/2 |
| Email in | Gmail and Outlook to `<slug>@inbound.staging...`: under 10 s, quote stripped. Needs spec 04 M5 |
| Email out | Same Gmail thread both ways |
| Bounces | Simulator `bounce@`, `complaint@`, `suppressionlist@`, `ooto@`. Needs spec 04 M7 |
| Spam verdicts | GTUBE recorded as spam; EICAR for virus |
| Audit log | Right actor and real client address |
| API tokens | Mint, call, revoke, refused |
| Workspace delete and restore | Grace, reminder, restore |
| Scheduler | One run per job across restarts |
| Portal | `/hc/...` over HTTPS with CSP, no console errors |
| Theme, phone | Light, dark, phone width |

Never enabled on staging: the `test` inbound driver, `key_auto_generate`, `app.env=local`, `session.insecure`, the `log` mailer, `frontend.hot_path`, `migrations.auto`,
`http_client.allow_private_networks`, production OAuth clients or keys, real customer data.

## Blockers

Severity: **deploy**, **feature**, **nice**. Size: S under a day, M 1 to 3 days, L more. The infrastructure already sets the settings
these items will read: `http.tls_addr` (`:8443`) beside `http.addr` (`:8080`), `gate.enabled`, `gate.password` and `gate.ci_key`.

| # | Blocker | Evidence | Severity | Size | Covered by |
|---|---|---|---|---|---|
| 1 | **No HTTPS listener.** `TLSConfig()` exists but nothing serves it. Needs: 443 and 80 listeners, `HTTPHandler` on 80 for ACME, redirect to HTTPS except challenges and health, graceful stop of both, low-port binding by capability, a default driver config | `framework/customdomain/tls.go:169`; `framework/foundation/serve.go` `listen` serves plain TCP; no `ServeTLS` or `tls.NewListener` in `framework/`, `app/`, `bootstrap/` | deploy | M | New: 19. 08 assumes it |
| 2 | **The `app.url` certificate would be refused.** No authorizer is set, so every hostname is refused. Needs an authorizer allowing the `app.url` host, and that certificate kept renewed, obtained after the listeners are up | `bootstrap/app.go` sets no `Domains`; `framework/foundation/boot_web.go:209` | deploy | S | New: 19. 08 adds custom domains later |
| 3 | Staging gate middleware | None exists | deploy | S to M | New: 19 |
| 4 | App never run as a non-owner role; cross-tenant jobs would quietly do nothing | `tests/application_role.go`, used by 2 tests; `app/jobs/prune_audit_logs.go:101` | deploy | M to L | New: 19 |
| 5 | No grants or role provisioning; audit append-only rule covers only the migrating role | `database/migrations/20261101000013_create_audit_logs_table.sql:40` | deploy | S | New: 19 |
| 6 | River under a non-owner role unverified | `framework/queue/queue.go` | deploy | S | New: 19 |
| 7 | Effective-config output needs a redaction review before real secrets are used | `framework/config/redact.go` | deploy | S | New: 19 |
| 8 | Quietly wrong defaults: `local` disk, `log` mailer, localhost `app.url`, production inbound domain. Boot guards | `framework/foundation/load.go:84,87`; `config/channels.go:39`; `framework/foundation/config.go:107` | deploy | S | Partly 04 M6 |
| 9 | RDS CA bundle must be present where the binary runs | Built: the image carries it at `/etc/ssl/certs/rds-global-bundle.pem`, connection strings use `sslmode=verify-full` | deploy | S | Done: 19 |
| 10 | Packaging, pipeline, CDK, `docs/deploy` | Built: `Dockerfile`, `.github/workflows`, `deploy/`, `docs/deploy` | deploy | L | Done: 19 |
| 11 | SES inbound route missing; raw mail recorded under the configured disk rather than the reference's | `routes/webhooks.go:24`; `app/actions/email/receive_inbound_email.go:195,244` | feature | M | 04 M5 |
| 12 | SES events, bounces, suppression, unsubscribe | No `/webhooks/ses/events` | feature | L | 04 M7 |
| 13 | Custom sending domains | No `email-domains` routes | feature | L | 04 M8 |
| 14 | Sending limits, reputation pause, tenants; needed before the gate comes off | `TenantName` never set | feature | L | 04 M11 |
| 15 | E2E suite tied to localhost, Mailpit, `psql`, test inbound | `playwright.config.ts:10`; `tests/e2e/support/` | feature | M | New: 19 |
| 16 | Maintenance state is a local file, lost with a disposable replica | `framework/foundation/boot_services.go:79` | nice | S | None |
| 17 | No HSTS. The binary is now the edge, so this is ours | `app/http/middleware/content_security_policy.go:48` | nice | S | None |
| 18 | Release not stamped, so smoke tests cannot read the live commit | `bootstrap/app.go:45` | nice | S | 11 |
| 19 | Attachments: S3 URLs will need CSP changes or a proxy | `content_security_policy.go:82,84` | nice | S | 03 |
| 20 | Spec 04 config keys differ from code | `framework/mailbox/drivers/ses/ses.go:45` | nice | S | 04 |

Withdrawn as the design changed: `trusted_proxies` as a guard (the binary sees the client directly), ALB rules and idle timeout.

## Costs

Per month, read 2026-11-04; details and what is not confirmed in [`docs/deploy/costs.md`](../../docs/deploy/costs.md).

| Environment | Monthly |
|---|---|
| Staging: 1 task, db.t4g.micro, cache.t4g.micro, NLB, 3 public addresses | ~$67 |
| Production: 2 web and 2 worker tasks, db.t4g.medium Multi-AZ, 2 cache.t4g.small, NLB | ~$273 plus traffic and mail |

Staging is above the earlier "about $38" because the owner chose the NLB for staging too: the load balancer and its two addresses
are about $28 of it.

## Decisions taken

1. **Region:** staging us-east-2, production us-east-1, both in `deploy/cdk/lib/config/environments.ts`.
2. **Compute and deployment:** ECS Fargate on ARM behind an NLB with TCP passthrough, rolling deploys with circuit-breaker rollback.
3. **DNS:** every record at Vercel, written by `deploy/dns`; no Route 53, no delegation.
4. **Help center host in production:** still open, spec 08. Staging stays in path mode.
5. **Staging mail:** `trydimmi.com` subdomains.
6. **SES production access for staging:** yes, once domains verify; a manual step in the runbook.
