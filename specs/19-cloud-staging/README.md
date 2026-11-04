# 19 Cloud staging

**Draft: awaiting owner approval.** Nothing has been created in any account. Research, options and sources:
[`research/aws-staging-deployment.md`](../../research/aws-staging-deployment.md). Work list: [tasks.md](tasks.md).

Prices are us-east-1, read 2026-11-04. The repository is public, so account ids, secret values and environment-specific
identifiers never go in the repository.

## Goal

A staging environment used exactly like production, with everything real, on tiny instances: the cloud edition only, real
deployments on every push to `main`, real Valkey, real DNS on `trydimmi.com`, real mail both ways through Amazon SES, all
defined with AWS CDK in this repository and fully documented, so everything we have built can be tested by hand in a browser.

## Recommended architecture

```
 Vercel (registrar, DNS for trydimmi.com, later the marketing site)
   | 4 NS records: staging.trydimmi.com
   v
 Route 53 zone staging.trydimmi.com, in the dimmi-staging AWS account, us-east-1
   |
   |  app.staging.trydimmi.com          inbound.staging.trydimmi.com (MX)
   v                                      v
 ALB, TLS by ACM                        SES receiving, spam and virus scan on
   | gate rules: IP / CI key / SES path    | rule: S3 raw/ + SNS topic
   | idle 60 s                             v
   |                                     S3 inbound bucket (31 days)
   v                                       |
 ECS on Fargate, ARM, one image            | SNS HTTPS, signed, retried 1 h, DLQ
   web x2     0.25 vCPU 512 MiB   <--------+  POST /webhooks/ses/inbound
              roles=web           <---------- POST /webhooks/ses/events
   worker x1  0.25 vCPU 1 GiB                 (SES config sets: bounces, complaints...)
              roles=worker,scheduler
   migrate    one-off, owner role, before each rollout
   | outbound through public IP, no NAT: SES API, OAuth, SNS certificates
   v
 Isolated subnets, no internet route
   RDS PostgreSQL 18  db.t4g.micro, single AZ, 7 days of backups, app connects as dimmi_app
   ElastiCache Valkey 8 cache.t4g.micro, TLS + AUTH

 S3 files bucket, KMS key for encryption.kek, Secrets Manager into DIMMI_* env
 GitHub Actions --OIDC, no keys--> ECR + cdk deploy, on push to main
```

## Decisions and why

Each row is a recommendation; items needing the owner's answer are under "Decisions for you".

| # | Area | Decision | Why | Rough monthly |
|---|---|---|---|---|
| 1 | Accounts | AWS Organizations. Existing account becomes management, empty of workloads. New `dimmi-staging`, later `dimmi-production` | SES sandbox, quota, reputation and suppression are per account. Credentials cannot cross | $0 |
| 2 | Safety | Root MFA, no root keys, IAM Identity Center for people, budgets at $100 and $150 with email, CloudTrail default | Bills and takeovers are the real risks of a new account | $0 |
| 3 | Region | us-east-1 | Supports SES receiving. Specs and defaults assume it | |
| 4 | DNS | Keep Vercel. Delegate `staging.trydimmi.com` to Route 53 by NS records | Production and apex untouched. CDK owns all staging records | $0.50 |
| 5 | Compute | ECS Fargate on ARM. Web and worker as separate services | No hosts. Real rolling deploys, rollback, one-off tasks. Replaces EC2 in spec 12 | $23 |
| 6 | Deploy | Rolling, 100% to 200%, circuit breaker with rollback, deregistration 30 s, ALB idle 60 s | Zero downtime. App idle 120 s must exceed ALB's | |
| 7 | Migrations | One-off task as owner role before rollout. `migrations.auto=false` | Running tasks cannot change schema. Expand-only keeps old tasks working | |
| 8 | Database | RDS PostgreSQL 18.x, `db.t4g.micro`, 20 GB gp3, single AZ, encrypted, 7-day backups, deletion protection | Real engine and version. `uuidv7()` core, `citext` listed | $14 |
| 9 | DB roles | master, `dimmi_owner`, `dimmi_app` (`NOSUPERUSER NOBYPASSRLS`, owns nothing) | Row-level security applies only to non-owners | |
| 10 | Valkey | Node-based `cache.t4g.micro`, TLS, AUTH, no snapshots | Serverless lacks `CLIENT TRACKING`, which the cache uses | $9.34 |
| 11 | Storage | Two private buckets, SSE-S3, no SES message encryption | SES's option is client-side, which our driver cannot read | under $1 |
| 12 | Secrets | Secrets Manager, injected as env at task start. App key in a secret, never on disk. KMS key as `encryption.kek` | Same shape as production. ECS handles it with no code | $3.40 |
| 13 | Network | VPC two AZs. Public subnets for ALB and tasks, security group admits ALB only. Isolated subnets for data. S3 gateway endpoint. No NAT | NAT alone would be over $32 | $18.25 IPs |
| 14 | Edge | ALB, ACM. No CloudFront, no WAF | Assets are same-origin from the binary. The gate does WAF's job here | $17.60 |
| 15 | Observability | CloudWatch logs (JSON, 30 days), default metrics, under 10 alarms to email. No traces. Sentry optional | Enough for staging, inside free tiers | $1 |
| 16 | Debugging | ECS Exec with `psql` in the image. Admin port stays loopback | No bastion, no open port, audited | $0 |
| 17 | IaC | TypeScript CDK in `deploy/cdk` | pnpm, Biome and `tsc` are here. Go would still need Node | $0 |
| 18 | CI/CD | GitHub Actions with OIDC, ARM runner, ECR, smoke tests | No long-lived keys. Free while the repo is public | under $1 |

### DNS names

| Name | Record | Purpose |
|---|---|---|
| `staging.trydimmi.com` | NS at Vercel, zone in Route 53. Alias to ALB, redirect to app | Delegation; spec 09's demo uses this host |
| `app.staging.trydimmi.com` | Alias to ALB | `app.url`, cookies, passkey relying party |
| `inbound.staging.trydimmi.com` | MX 10 `inbound-smtp.us-east-1.amazonaws.com`, DKIM CNAMEs, `_dmarc` | Forwarding addresses, default sender for customer mail |
| `bounce.inbound.staging.trydimmi.com` | MX 10 `feedback-smtp.us-east-1.amazonses.com`, SPF TXT | Return path of customer mail |
| `mail.staging.trydimmi.com` | MX to SES inbound, DKIM CNAMEs, `_dmarc` | System mail, `no-reply@`. Replies stored, never bounced |
| `bounce.mail.staging.trydimmi.com` | MX `feedback-smtp`, SPF TXT | Return path of system mail |
| assets | none | Served from the app on its own origin |

Production will mirror this one level up: `app.`, `inbound.`, `mail.`. DMARC starts at `p=quarantine`; each sending domain has
its own record, since receivers look at the exact domain and then the organizational domain.

### Configuration

Set by CDK. `(s)` comes from Secrets Manager.

| Setting | Value |
|---|---|
| `DIMMI_APP__ENV` | `production`; there is no staging environment and there should not be |
| `DIMMI_TELEMETRY__ENVIRONMENT` | `staging` |
| `DIMMI_APP__URL`, `DIMMI_APP__REGION` | `https://app.staging.trydimmi.com`, `us` |
| `DIMMI_APP__ROLES` | `web`, or `worker,scheduler` |
| `DIMMI_APP__KEY` (s) | generated once; never `key_auto_generate` |
| `DIMMI_APP__DATA_DIR` | writable path in the image |
| `DIMMI_MIGRATIONS__AUTO` | `false` |
| `DIMMI_HTTP__TRUSTED_PROXIES` | CIDRs of the public subnets; without it all visitors share one address for rate limits |
| `DIMMI_DATABASE__URL` (s) | `dimmi_app`, TLS verified against the RDS CA. Migrate task: `dimmi_owner` |
| `DIMMI_DATABASE__MAX_CONNS`, `DIMMI_QUEUE__MAX_CONNS` | `5`, `5` |
| `DIMMI_REDIS__URL` (s) | `rediss://` with AUTH token |
| `DIMMI_FILESYSTEMS__DEFAULT` | `s3`, with `__DISKS__S3__*` and `__DISKS__SES_INBOUND__*` |
| `DIMMI_ENCRYPTION__KEK` | `awskms`, with `__KEKS__AWSKMS__KEY_ARN` |
| `DIMMI_MAIL__FROM__EMAIL` | `no-reply@mail.staging.trydimmi.com`; the default would use the app host |
| `DIMMI_MAIL__MAILERS__SYSTEM__*` | `ses`, region, set `dimmi-staging-system` |
| `DIMMI_MAIL__MAILERS__CUSTOMER__*` | `ses`, region, set `dimmi-staging-customer` |
| `DIMMI_CHANNELS__EMAIL__INBOUND_DOMAIN` | `inbound.staging.trydimmi.com`; the cloud default is the production domain |
| `DIMMI_CHANNELS__EMAIL__INBOUND__DRIVERS` | `ses` only |
| `DIMMI_MAILBOX__SOURCES__SES__*` | `ses`, topic ARN, disk `ses_inbound` |
| `DIMMI_AUTH__SIGNUP__MODE` | `open`, behind the gate |
| `DIMMI_AUTH__OAUTH__*` (s) | staging's own GitHub and Google clients |
| `DIMMI_AUTH__FACTORS__TOTP__ISSUER`, `__PASSKEY__NAME` | `dimmi staging` |

### Email

- SES in the staging account: domain identities for `inbound.staging` and `mail.staging`, Easy DKIM through Route 53, custom MAIL
  FROM with "reject on MX failure" so a mistake is loud.
- Configuration sets `dimmi-staging-system` and `dimmi-staging-customer`, reputation metrics on, events `SEND`, `DELIVERY`, `BOUNCE`,
  `COMPLAINT`, `DELIVERY_DELAY`, `REJECT` to one SNS topic, as spec 04 wants.
- Receipt rule set: `inbound.staging` to S3 `raw/` with SNS, scan on, TLS optional per the spec; `mail.staging` to S3 `system/`.
  CDK cannot activate a rule set, so a small custom resource does. Rule recipients are explicit.
- SNS HTTPS subscriptions with a custom delivery policy of about one hour and an SQS dead-letter queue, since raw mail stays in S3
  and can be replayed. The subscription follows the app stack, so the app is there to confirm it.
- Tenants: none in staging until spec 04 M8 and M11 build them; the app creates them, not CDK.
- Reputation: separate account and subdomains, low volume, known recipients, simulator for failures. Separate subdomains protect
  account-level standing and make sharing unlikely to matter, but whether mailbox providers count subdomain signals against the
  parent domain is not settled. See decision 8.
- Scheduled round trip: a GitHub Actions cron every 6 hours calls the existing `POST /email-addresses/{id}/tests` on a test workspace
  with an API token, and checks the arrival by polling. The test address's public address is the Gmail test mailbox, forwarding to
  dimmi, so it proves SES send, real Gmail delivery, MX, receipt rule, S3, SNS, webhook and worker. On failure the workflow fails,
  which emails the owner. Confirm when building that the API exposes the arrival for polling.

## Rollout plan

Owner's manual steps are marked **Owner**. Everything else is built by agents; detail in [tasks.md](tasks.md).

| Step | Who | What | Depends on |
|---|---|---|---|
| 0 | **Owner** | Approve this plan, answer "Decisions for you" | |
| 1 | **Owner** | Root MFA, Organization, `dimmi-staging` account, Identity Center user, budgets | 0 |
| 2 | Agents | Build track A: image, role provisioning, non-owner role run, boot guards | 0 |
| 3 | Agents | Build track B: `deploy/cdk`, workflows, `docs/deploy` | 0 |
| 4 | **Owner** | `cdk bootstrap` staging with Identity Center credentials; deploy `Dns` stack | 1, 3 |
| 5 | **Owner** | Add 4 NS records at Vercel; `dig NS staging.trydimmi.com` shows Route 53 | 4 |
| 6 | **Owner** | Deploy `Ci`, `Network`, `Data`, `Mail`. Paste OAuth secrets. Create GitHub environment `staging` and its variables | 5 |
| 7 | **Owner** | Request SES production access once identities show verified; text in `docs/deploy` | 6 |
| 8 | **Owner** | Create GitHub and Google OAuth clients, test mailboxes | 6 |
| 9 | Agents and **Owner** | Push `main`: provision roles, migrate, deploy, smoke test. Owner watches the first run | 2, 3, 6 |
| 10 | **Owner** | Walk the checklist for non-mail features and sign-up mail | 9 |
| 11 | Agents | Build track C: SES inbound route, SES events, then scheduled round trip | 9 |
| 12 | **Owner** | Walk the mail checklist with Gmail and Outlook | 11, 7 |
| 13 | Agents | Staging Playwright subset, custom domains (spec 04 M8), limits (M11) | 11 |

Staging can go live at step 9 without mail receiving. Sign-up, invites and replies go out through SES then.

### `docs/deploy` outline

`README` (map, costs, who to call), then in order: `01-aws-accounts`, `02-dns-vercel-delegation`, `03-bootstrap-cdk`, `04-ses`
(production access text, what each record does), `05-github`, `06-oauth-clients`, `07-first-deploy`, `08-operating` (deploy, rollback,
migrate, ECS Exec, `psql`, rotate secrets, replay mail from S3), `09-testing-checklist`, `10-tear-down`.

What stays manual, and why: account and root setup (AWS requires it), NS records at Vercel (another provider, once), SES production
access (a review, by AWS), OAuth clients and their secrets (other providers), test mailboxes, bootstrap (it precedes everything).

### What must never be enabled on staging

The `test` inbound driver and its secret, `key_auto_generate`, `app.env=local`, `session.insecure`, the `log` mailer, `frontend.hot_path`,
`migrations.auto`, `http_client.allow_private_networks`, production OAuth clients or keys, real customer data. A smoke test checks the
ones visible from outside, and `config:show` output can be reviewed through ECS Exec.

### Keeping search engines and the public out

ALB listener rules, in order: `/robots.txt` fixed response disallowing all; `/webhooks/ses/*` forwarded (SNS is signature-checked); up to
5 source CIDRs per rule for people; a secret `X-Dimmi-Staging-Key` header for CI; everything else a fixed 403. The SPA already sends
`noindex`. Health checks reach targets directly.

## Testing staging end to end

Built features only. Test mailboxes: one Gmail, one Outlook.com, and one on a custom domain the owner controls later for M8. Seed data:
there is no seed command, and none is needed at first; real sign-up creates a real workspace. Add a Playwright script only if manual setup
becomes tedious. Sandbox only: verify the test mailboxes as SES identities.

| Feature | Check |
|---|---|
| Sign-up mail | Sign up with Gmail: mail arrives, not spam; "Show original" passes SPF, DKIM, DMARC, aligned on `mail.staging` |
| Verify, magic link, password reset | Each link works, once only, from another browser where expected |
| Invites | Invite Outlook address, accept, role applied, withdrawn invite refused |
| OAuth | GitHub and Google to `https://app.staging.trydimmi.com/auth/{provider}/callback` |
| 2FA | TOTP enroll in a real app, shows issuer `dimmi staging`, recovery code works, workspace rule enforces it |
| Passkeys | Register on real domain with Touch ID and a phone; `__Host-` cookies present; sign in; nothing logged in console |
| Sessions | List, revoke another browser; revoked socket closes |
| Inbox | Create, reply, note, assign, close, search |
| Realtime, two browsers | Reply shows without reload; typing shows; with `web x2`, connect sockets to different tasks, confirm by task logs; deploy during a session and see reconnect |
| Email in | Gmail and Outlook to `<slug>@inbound.staging...`: conversation under 10 s, subject right, quote stripped |
| Email out | Reply from inbox: arrives in the same Gmail thread; reply from Gmail lands in the same conversation |
| Forwarding | Gmail forward to forwarding address; confirmation code shows; address test passes |
| Bounces | Reply to `bounce@simulator.amazonses.com`: delivery failed in timeline, suppressed. `complaint@`, `suppressionlist@`, `ooto@` likewise. Needs SES events |
| Spam verdicts | GTUBE from a real mailbox: spam verdict recorded, no notification; EICAR for virus |
| Audit log | Actions above appear, right actor and IP (proves `trusted_proxies`) |
| API tokens | Mint, call API, revoke, refused; role narrowing holds |
| Workspace delete, restore | Grace, reminder mail, restore; purge job after grace in a test workspace |
| Scheduler | Exactly one scheduled run per job across restarts; nightly prune in logs |
| Portal | `/hc/...` renders over HTTPS with the CSP, no console errors |
| Theme, phone | Light, dark, phone width |
| Deploy | Push a trivial commit under load of a click-through: no errors; break a commit on purpose: rollback happens |
| Failure | Stop the worker: mail queues, then drains. Valkey reboot: clients reconnect and refetch |

Spec 04's demo script covers the same mail flows in more depth, and its steps 5, 8 and 9 wait on M8 and widget work.

## Blockers

Severity: **deploy** blocks a real deployment; **feature** blocks one feature's test; **nice** can wait. Size: S under a day, M 1 to 3 days,
L more. Task boxes in specs are unreliable and `specs/BUILD-TRACKS.md` does not exist, so "Covered by" names a spec task, not an owner.

| # | Blocker | Evidence | Severity | Size | Covered by |
|---|---|---|---|---|---|
| 1 | No container image | No Dockerfile or `deploy/` in tree | deploy | S | 11 tasks.md:49 specifies it, without the cloud tag. New: 19 |
| 2 | No CI, no deploy workflow | No `.github/` | deploy | M | 00-foundation tasks.md:7 for CI. Deploy: 19 |
| 3 | No CDK, no `docs/deploy` | Tree | deploy | L | 19 |
| 4 | App never run as a non-owner of tables. Cross-tenant jobs would quietly do nothing, such as pruning | `tests/application_role.go`, used by 2 tests; `app/jobs/prune_audit_logs.go:101` | deploy | M to L | None. New: 19 |
| 5 | No grants or default privileges in migrations; nothing creates the roles. Audit append-only rule covers only the migrating role | `database/migrations/20261101000013_create_audit_logs_table.sql:40`; no `GRANT` in migrations | deploy | S | None. New: 19 |
| 6 | River under non-owner role unverified | `framework/queue/queue.go`; research section 5 | deploy | S | None. New: 19 |
| 7 | RDS TLS needs the CA bundle in the image | `framework/database` takes URL; image absent | deploy | S | With 1 |
| 8 | Effective-config output needs checking for connection-string credentials before real secrets are used | `framework/config/redact.go`, `framework/database/config.go:73`, `framework/redis/client.go:68` | deploy | S | None. New: 19 |
| 9 | Defaults that fail quietly in the cloud: `local` disk, `log` mailer, no `trusted_proxies`, production inbound domain. A missed variable gives a staging that looks fine | `framework/foundation/load.go:84,87`; `config/channels.go:39`; `framework/foundation/config.go:107` | deploy, as boot guards | S | Partly 04 M6 ("boot refuses log"). New: 19 |
| 10 | SES inbound route missing | `routes/webhooks.go:24`; `Mailbox` unused in `app/` | feature: email in | M | 04 M5 |
| 11 | SES raw mail recorded under the configured disk, not the reference's | `app/actions/email/receive_inbound_email.go:195,244` | feature: email in | S | 04 M5, should name it |
| 12 | SES event webhook, bounce and complaint handling, suppression | No `/webhooks/ses/events`; no suppression actions | feature: bounces, complaints | L | 04 M7 |
| 13 | Unsubscribe flow | No route | feature | M | 04 M7 |
| 14 | Custom sending domains: identities, records, API, checker | No `app/services/ses`, no `email-domains` routes | feature: own domain | L | 04 M8 |
| 15 | Sending limits, reputation pause, tenants | `email_sending_profiles` table only; `TenantName` never set | feature; deploy before any open public staging or production | L | 04 M11 |
| 16 | E2E suite assumes localhost, Mailpit, direct `psql`, test inbound | `playwright.config.ts:10`; `tests/e2e/support/mail.ts`, `database.ts`, `inbound.ts`; 5, 5 and 2 of 22 specs use them | feature: Playwright on staging | M | New: 19. Start with a subset |
| 17 | Maintenance state is a local file, so a new task in maintenance serves traffic | `framework/foundation/boot_services.go:79`, `maintenance.go` | nice | S | None |
| 18 | Worker has no health signal; no `healthcheck` command | `framework/foundation/serve.go:63` | nice | S | 11 tasks.md:49 |
| 19 | No HSTS | `app/http/middleware/content_security_policy.go:48` | nice | S | None |
| 20 | Release not stamped, so smoke tests cannot read the live commit | `bootstrap/app.go:45` | nice | S | 11 packaging ldflags |
| 21 | Attachments: no download or upload route yet. S3 presigned URLs will need the CSP's `img-src` or a proxy | `routes/`, `content_security_policy.go:82,84` | nice now | S | 03 |
| 22 | Malware scanner never called; inbound relies on SES verdicts | No use in `app/` | nice | M | 03, 04 |
| 23 | Spec 12 infrastructure says EC2 and 300 s idle; config keys in spec 04 differ from code | `specs/12-cloud-billing/infrastructure.md`; `framework/mailbox/drivers/ses/ses.go:45` | nice | S | Update once approved |

Checked and fine: graceful shutdown on SIGTERM; realtime across replicas by design (unproved on AWS); `__Host-` cookies and CSRF behind TLS at the
ALB; passkey relying party from `app.url`; JSON logs by default; `/ready` during maintenance; app key required rather than generated by default;
task-role credentials through the SDK chain. Not blockers: billing, marketing site, widget API, notifications, webhooks, agent, none of which are
built; custom help-center domains need more than an ALB when spec 08 arrives.

## Costs

Monthly, us-east-1, read 2026-11-04. "Confirmed" means from an AWS page; `~` means third-party or from memory, see research section 12.

| Item | Detail | Monthly | Confirmed |
|---|---|---|---|
| Fargate web | 2 x 0.25 vCPU, 512 MiB, ARM | $14.42 | yes, derived |
| Fargate worker | 1 x 0.25 vCPU, 1 GiB, ARM | $8.51 | yes, derived |
| Public IPv4 | 3 tasks + 2 ALB | $18.25 | yes |
| ALB | hours + about 0.2 LCU | $17.60 | yes |
| RDS | `db.t4g.micro`, 20 GB gp3, backups in free allowance | ~$14 | no |
| Valkey | `cache.t4g.micro` | ~$9.34 | no |
| Route 53 | 1 zone | $0.50 | yes |
| Secrets Manager | 6 secrets | $2.40 | yes |
| KMS | 1 key | $1.00 | yes |
| S3, ECR | under 5 GB | ~$0.50 | partly |
| CloudWatch | inside free tier, some overflow | ~$1 | partly |
| SES, SNS | hundreds of mails | under $1 | yes |
| GitHub Actions | public repo | $0 | yes |
| **Total** | | **about $89** | |

- `web x1` saves $10.86: about **$78**.
- Stop overnight and at weekends, on 60 hours a week: Fargate and its IPs to zero saves about $22; RDS stop saves about $7, but restarts itself after 7 days
  (from memory). ALB and Valkey cannot stop. About **$60**. Mail notifications sent while stopped are lost after SNS gives up; raw mail stays in S3.
- New account credits of up to $200 may apply; not confirmed for a member account.

Production changes: Multi-AZ RDS on a larger class and `read_urls` later; Valkey with a replica; private task subnets with NAT per AZ; more and bigger tasks with
autoscaling; WAF; longer logs; backups tested; tenants per workspace; limits on; its own account and SES review. `research/pricing.md` has about $1,450 per month as
a baseline, not recomputed here.

## Risks

| Risk | Mitigation |
|---|---|
| Non-owner run finds more than expected and delays the first deploy | Do it first; list failures before estimating. Do not ship as owner by default |
| AWS refuses or delays SES production access (first reply within 96 hours) | Domains verified first. Start in sandbox with verified mailboxes and simulator; it blocks only sending to strangers |
| Vercel refuses subdomain NS | Proved in step 5, before anything else depends on it. Fallback: move all DNS to Route 53 |
| Staging mail hurts `trydimmi.com`'s standing | Separate account and subdomains, low volume, gate on. Certainty needs a separate domain: decision 8 |
| Public repo leaks an account id or staging detail | No lookups, no `cdk.context.json`, ids in GitHub variables. Review `docs/deploy` on every change |
| Deploy credentials abused from a workflow | OIDC bound to one environment on `main`, actions pinned by SHA, no deploy on `pull_request` |
| Costs creep | Budget alerts at $100 and $150. Staging's account holds nothing else |
| Open sign-up plus real SES becomes a spam relay | Gate stays until M11 limits exist |
| 0.25 vCPU too small for hashing or inbound parse | CPU alarm; step to 0.5 vCPU, about $7 per task more |
| Valkey behaviour differs from compose: restricted commands, client tracking | First boot proves it; `/ready` and realtime smoke test |
| Burstable database runs out of credits | Alarm on CPU credit balance |
| Migration that is not expand-only breaks rollback | Lint exists; CI runs `migrate:plan` and prints findings |

## Decisions for you

1. **Region?** Recommended: us-east-1. eu-west-1 if you want staging near European users; specs and defaults assume us-east-1.
2. **Existing AWS account?** Recommended: make it the management account if it holds nothing; if it runs anything, tell me and we create a fresh management account.
3. **DNS?** Recommended: keep Vercel, delegate `staging.trydimmi.com`.
4. **Fargate rather than spec 12's EC2 without containers?** Recommended: yes, Fargate, and update spec 12. EC2 saves about $11 but cannot prove two replicas and needs host care.
5. **Two web tasks always, or one?** Recommended: two, $11 more, since multi-replica realtime and zero-downtime deploys are the point.
6. **Must the app run as a non-owner before the first deploy?** Recommended: yes. The alternative, a first deploy as owner, is faster but tests something production must not be.
7. **Gate?** Recommended: IP allowlist plus CI key at the ALB, sign-up open behind it. You would add a rule when your IP changes.
8. **Staging mail on `trydimmi.com` subdomains, or on a separate domain?** Recommended: subdomains as you asked, knowing no one can promise zero effect on the parent. A separate domain would remove the doubt for about $12 a year.
9. **SES production access now?** Recommended: yes, as soon as domains verify, so real recipients and the request itself get tested early. Sandbox works until then.
10. **Run `make lint test` and e2e before each deploy?** Recommended: yes, all three, blocking. If it takes over about 20 minutes, e2e moves to run alongside, not blocking.
11. **Always on?** Recommended: yes at first, about $89. Add the overnight stop later if the bill matters, about $60, accepting mail gaps at night.
12. **First deploy before SES receiving exists?** Recommended: yes. Deploy at step 9 and test everything else, then add M5 and M7.
