# AWS staging deployment: research

Research date: 2026-11-04. Backs the plan in [`specs/19-cloud-staging/`](../specs/19-cloud-staging/README.md), which is
the document to read for decisions. This file holds the options, the reasoning and the sources.

**Updated after owner review: see section 13 first.** It supersedes the accounts, compute, edge, deploy and gate recommendations below.

Method: repository code and specs were read; AWS, Vercel, GitHub and PostgreSQL documents were read on the web. No
AWS, Vercel, GitHub or DNS account was read or changed, and nothing was deployed, so nothing here has been proved by
running it on AWS. Web pages were treated as data. Prices are US East (N. Virginia), on-demand, in USD, as read on
2026-11-04, and 730 hours make a month. Section 12 lists what could not be confirmed.

## 1. What the application needs

Facts from the code, each checked at the path given.

| Topic | Finding | Evidence |
|---|---|---|
| Binary | One Go binary. `-tags cloud` selects the cloud edition, at build time, never by config. Browser app embedded | `Makefile` `build`, `config/edition_cloud.go` |
| Commands | `serve`, `migrate`, `migrate:status`, `migrate:plan`, `migrate:rollback`, `down`, `up`, `config:show`, `route:list`, `workspace:repair`, `user:disable-2fa`. No `healthcheck`, no seed | `framework/foundation/commands*.go`, `app/console/commands/` |
| Roles | `web`, `worker`, `scheduler`; any mix in one process. Scheduler joins River's leader election, so several are safe | `framework/foundation/environment.go`, `boot_services.go` |
| Environments | Only `local`, `testing`, `production`. Staging runs as `production` and names itself through `telemetry.environment` | `framework/foundation/environment.go`, `load.go` |
| Settings | YAML or `DIMMI_` variables, `__` between levels. Lists are comma-separated. A variable ending in `_FILE` reads a file | `framework/config/env.go`, `lists.go` |
| Health | `/up` and `/ready` on the public and admin listeners. Maintenance is a warning, so `/ready` stays 200 while the app is down for maintenance. `/metrics` only on admin, loopback by default | `framework/foundation/serve.go`, `framework/health/http.go`, `boot_web.go:167` |
| Non-web health | A process without `web` opens no listener at all | `framework/foundation/serve.go:63` |
| Shutdown | Handles SIGTERM. Listeners, realtime, then workers, within `http.shutdown_timeout`, 25 s by default | `framework/foundation/commands.go:65`, `serve.go`, `config.go:131` |
| Migrations | goose files under one advisory lock, plus River's own step. `migrations.auto` defaults to true, so every booting replica migrates. `migrate` wants `--force` in production. A lint keeps migrations expand-only unless marked contract | `migrations.go:51`, `commands_migrate.go`, `database/migrate/lint.go` |
| Database role | Pool uses `database.url`. Migrator and queue use `direct_url` when set, else `url` | `framework/database/config.go`, `queue/queue.go`, `migrations.go` |
| Row-level security | 29 product tables enable row-level security. Tests connect as table owner, where policies do not apply. Only a few queries run through a role that owns nothing | `tests/application_role.go`, `tests/feature/inbox/queries_test.go` |
| PostgreSQL | `uuidv7()` needs 18. `citext` created by a migration. Specs add `btree_gin`, `pg_trgm`, `vector` later | `database/migrations/`, `specs/08-help-center/README.md:475` |
| Valkey | rueidis with `rediss://` support, single, sentinel or cluster. Cache uses client-side caching (`rueidisaside`) | `framework/redis/client.go`, `framework/cache/store.go:87` |
| Realtime | centrifuge with Redis broker on Valkey. Any replica serves any client, no sticky sessions. WebSocket and SSE at `/connection/*`, server pings every 25 s, sockets told to reconnect on shutdown | `framework/broadcasting/server.go` |
| Local disk | Default disk is `local` under `app.data_dir`. Maintenance state is a file there, per replica, announced over Valkey | `load.go:84`, `boot_services.go:79`, `maintenance.go` |
| App key | From `app.key`, else a file, else generated only when `key_auto_generate` is on. Off by default, so a missing key stops the boot | `framework/foundation/appkey.go` |
| Key encryption | `encryption.kek` `local` by default, `awskms` driver built | `framework/encryption/drivers/awskms` |
| AWS credentials | Standard chain, so a task role works. Service calls use the guarded HTTP client that refuses private addresses, which matters for interface VPC endpoints. The S3 driver uses the SDK's own client | `framework/aws/config.go`, `httpclient/client.go`, `filesystem/drivers/s3/s3.go` |
| Cookies | `__Host-` and `Secure` follow the scheme of `app.url`, not the request, so TLS ending at a load balancer is fine | `app/app.go:198`, `app/auth/cookies.go` |
| Client address | `http.trusted_proxies` empty means every visitor shares the proxy's address for rate limits. Uses the rightmost untrusted address of `X-Forwarded-For` | `boot_web.go`, `http/middleware/trustproxies.go` |
| CSP, CSRF | CSP is same-origin plus a nonce. Trusted origin is `app.url` plus `http.trusted_origins` | `app/http/middleware/content_security_policy.go`, `route_service_provider.go` |
| Passkeys | Relying party ID is the hostname of `app.url`, so passkeys belong to one environment | `app/providers/factor_drivers.go` |
| SES outbound | Driver built: raw MIME through SESv2, configuration set per mailer or message, tenant per message, region per mailer section | `framework/mail/drivers/ses/ses.go` |
| SES inbound | Driver built and tested, SNS verification built. No route reaches it; `routes/webhooks.go` has only the test source | `framework/mailbox/drivers/ses/ses.go`, `routes/webhooks.go` |
| Cloud-only code | No `app/cloud`. The only tagged non-test file is `config/edition_cloud.go`, so no billing and no marketing site exist yet. `/` redirects to `/d` | `routes/web.go:48` |
| Portal | `/hc/{center}` on bundled sample content | `routes/web.go`, `app/support/helpsample` |
| Repository | `origin` is set to GitHub. No `.github/`, no `deploy/`, no Dockerfile | `git remote -v`, tree |

Specs disagree with the request on two points. `specs/12-cloud-billing/infrastructure.md`, marked an outline, says EC2
with no containers, and CloudFormation; the owner asked for CDK. `specs/11-self-host/operations.md` and spec 12 use an
ALB idle timeout of 300 s, which section 4 changes. `specs/BUILD-TRACKS.md` does not exist. This plan takes number 19, after `specs/17-workflows` and `specs/18-audiences`.

## 2. Accounts

| Option | For | Against |
|---|---|---|
| One account, staging and later production together | Nothing to set up | SES sandbox, sending quota, reputation, suppression list and the one active rule set are per account and Region, so staging could touch production. Secrets and blast radius shared |
| Organization with member accounts | Isolation is the AWS boundary, one bill, no extra charge expected | A few console steps |

Pick: AWS Organizations, with the existing account as management account when it holds nothing, and member accounts
`dimmi-staging`, later `dimmi-production`. AWS says to keep workloads out of the management account because service
control policies do not apply to it, and to attach a control stopping member accounts leaving or closing; organizations
created in the console after 2026-07-10 get that control by default
([management account best practices](https://docs.aws.amazon.com/organizations/latest/userguide/orgs_best-practices_mgmt-acct.html)).
If the existing account already runs other things, create a fresh account as management account instead, because moving
workloads later is harder. Sandbox status and quotas are per Region and account
([SES regions](https://docs.aws.amazon.com/ses/latest/dg/regions.html)), which is the strongest reason to split.

Region: SES receiving exists in 22 Regions including us-east-1, us-east-2, us-west-1, us-west-2, eu-west-1, eu-west-2,
eu-west-3, eu-central-1, eu-north-1, eu-south-1, ap-south-1, ap-southeast-1, ap-southeast-2, ap-southeast-3,
ap-northeast-1, ap-northeast-2, ap-northeast-3, ca-central-1, af-south-1, il-central-1, me-south-1 and sa-east-1, but not
in ap-south-2, ap-southeast-5, ca-west-1, eu-central-2, me-central-1 or GovCloud
([endpoints table](https://docs.aws.amazon.com/general/latest/gr/ses.html#ses_inbound_endpoints)). Pick us-east-1: it
receives mail, specs and driver defaults already assume it, `app.region` is `us`, and most AWS prices quoted are for it.
SNS topics and KMS keys for receiving must share the SES Region
([SES regions](https://docs.aws.amazon.com/ses/latest/dg/regions.html)).

## 3. Domain and DNS

Vercel is registrar and DNS for `trydimmi.com`.

| Option | For | Against |
|---|---|---|
| Move nameservers to Route 53 | One DNS system | Vercel loses automatic records and wildcards; the coming marketing site must then be pointed by hand; production and staging share one zone, so a staging role can touch production records unless carefully split |
| Delegate `staging.trydimmi.com` to a Route 53 zone in the staging account | The apex and production are untouched. CDK owns every staging record, so SES DKIM, MAIL FROM, MX and ACM validation are automatic. Credentials for staging can never edit production DNS | Four NS records entered by hand at Vercel |

Vercel lists `NS` among record types, and allows adding records from the dashboard or `vercel dns add`
([Vercel DNS records](https://vercel.com/docs/domains/managing-dns-records)). Custom nameservers for the whole domain
are also possible for domains registered there, up to four
([Vercel nameservers](https://vercel.com/docs/domains/working-with-nameservers)). The pages do not describe subdomain
delegation in those words, so step one of the rollout proves it with `dig` before anything depends on it.

Keeping Vercel nameservers means a marketing site on Vercel works without effort later. Route 53 costs $0.50 per zone per
month and alias queries to an ALB are free ([Route 53 pricing](https://aws.amazon.com/route53/pricing/)).

Subdomains are in the spec. Two details drive them. AWS says a MAIL FROM domain must be a subdomain of the verified
identity's parent domain and should not be used to send from or to receive mail, and that exactly one MX record may exist
on it ([custom MAIL FROM](https://docs.aws.amazon.com/ses/latest/dg/mail-from.html)). And under RFC 7489 section 6.6.3 a
receiver checks the DMARC record of the exact From domain and then of the organizational domain, not the levels between,
so each sending domain carries its own record ([RFC 7489](https://www.rfc-editor.org/rfc/rfc7489#section-6.6.3), cited
from memory, not re-read for this work).

Assets: the app serves `/build/*` from the binary on its own origin, and the CSP allows only `'self'`, so no asset domain
or CDN is needed. CloudFront would be one more thing that differs from local and adds nothing at this traffic.

## 4. Compute

| Option | Monthly, 3 tasks or 1 box | For | Against |
|---|---|---|---|
| ECS on Fargate, ARM, CDK | about $23 | No hosts to patch. Rolling updates, ALB integration, automatic rollback, one-off migration tasks, ECS Exec. Roles are separate services | Slightly more per unit than EC2 |
| EC2 with systemd, per spec 12 | about $12 for a t4g.small (from memory, not re-read) | Matches current spec text. Cheapest | Patching, AMIs, slower instance refresh, more CDK, one box cannot prove two replicas or a zero-downtime deploy |
| App Runner | not priced further | Simple | Reported closed to new customers since 2026-04-30 and in maintenance ([report](https://encore.dev/articles/end-of-app-runner), [report](https://dev.to/parag477/aws-app-runner-is-dead-heres-what-you-should-use-instead-2026-1hp7); AWS's own page shows no notice ([pricing](https://aws.amazon.com/apprunner/pricing/))). Costs more, $0.064 per vCPU-hour. WebSocket support not shown |
| ECS Express Mode | not priced | AWS's suggested App Runner replacement, per the reports above | Less control over rules and rollout than a plain service; not needed when CDK writes the service |

Pick: ECS on Fargate. Fargate rates derived from the page's examples: ARM $0.03238 per vCPU-hour and $0.00356 per GB-hour;
x86 $0.04048 and $0.00445 ([Fargate pricing](https://aws.amazon.com/fargate/pricing/)). ARM is about 20% cheaper, Go
cross-compiles with `CGO_ENABLED=0`, GitHub offers ARM runners, and spec 12 already chose Graviton. The smallest size is
0.25 vCPU with 512 MiB ([Fargate sizes](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-tasks-services.html)):
$7.21 per month on ARM. The app targets under 50 MB idle and uses Argon2id at 19 MiB per hash
(`framework/hashing/drivers/argon2id`), so this is real if a little slow at hashing. The worker gets 1 GB because inbound
parsing can hold large messages.

Timeouts. The ALB default idle timeout is 60 s, range 1 to 4000, and AWS advises the application idle timeout be larger
than the load balancer's, or a closing connection can yield a 502
([ALB attributes](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/edit-load-balancer-attributes.html)).
The app's `http.idle_timeout` is 120 s and realtime pings every 25 s, so the default 60 s is correct. Spec 12's 300 s would
invert the rule and should change. The Go server sets no write timeout, so SSE streams are not cut. Deregistration delay
30 s, because realtime clients are told to reconnect and API calls are short.

Deploys: rolling updates with the circuit breaker and rollback. It returns to the last `COMPLETED` deployment and needs
3 failures at this size ([circuit breaker](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/deployment-circuit-breaker.html)).
With 2 web tasks, 100% minimum and 200% maximum gives zero downtime. `stopTimeout` should exceed `http.shutdown_timeout`;
the worker's `queue.job_timeout` of 1 m exceeds the default 25 s shutdown, so worker tasks get 60 s and a 55 s shutdown
budget. The Fargate maximum for `stopTimeout` was not read for this work (120 s from memory).

Migrations: see section 9.

## 5. Data

### PostgreSQL

RDS for PostgreSQL 18 has been available since 18.1
([announcement](https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-rds-postgresql-major-version-18)). The extension
page lists 18.1 to 18.6 with `citext` 1.8, `pg_trgm` 1.6, `btree_gin` 1.3, `pgcrypto` 1.4 and `pgvector` 0.8.2 at 18.4 and
later ([extensions](https://docs.aws.amazon.com/AmazonRDS/latest/PostgreSQLReleaseNotes/postgresql-extensions.html)).
`uuidv7()` is a core function of 18, needing no extension
([PostgreSQL 18](https://www.postgresql.org/docs/18/functions-uuid.html)). `citext` is a trusted extension, so a
non-superuser with `CREATE` on the database can install it ([citext](https://www.postgresql.org/docs/18/citext.html)).

Size: `db.t4g.micro`, 2 vCPU burst, 1 GiB, listed at $0.016 per hour or $11.68 per month by third parties
([Aiven](https://aiven.io/tools/instances/db.t4g.micro)); AWS's table did not render, so check before relying on it. Same
listing source shows PostgreSQL 18 on that class ([Sparecores](https://sparecores.com/database/aws/db.t4g.micro)). CPU
credits past the burst cost $0.075 per vCPU-hour ([RDS pricing](https://aws.amazon.com/rds/postgresql/pricing/)).
`max_connections` defaults to `LEAST(DBInstanceClassMemory/9531392, 5000)`, computed on memory left after what RDS reserves
([RDS limits](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_Limits.html)), so under 112 here. Budget: web
2 x 5, worker 5 + 5, leaving room for a migration and a person. Set `database.max_conns=5` and `queue.max_conns=5` (each
defaults to 10). No RDS Proxy: it adds cost and needs `direct_url`.

Roles. PostgreSQL says table owners normally bypass row security and that superusers and `BYPASSRLS` roles always do
([row security](https://www.postgresql.org/docs/18/ddl-rowsecurity.html)). The RDS master user must therefore never be
what the app connects as. Three roles: master for provisioning only; `dimmi_owner`, owner of database and schema, for
migrations; `dimmi_app`, `NOSUPERUSER NOBYPASSRLS`, for the running app and queue, with DML through
`ALTER DEFAULT PRIVILEGES FOR ROLE dimmi_owner`. Web and worker tasks get only `dimmi_app`'s URL, and `migrations.auto` is
false, so no running task can change the schema. Migrator runs as a separate task definition holding the owner's URL.

Open points:
- Nothing has run the whole app as `dimmi_app`. Jobs and queries that read across tenants without a tenant set would see
  no rows rather than fail. `PruneAuditLogs` is one example, where the listing step would return nothing and the job would
  silently do nothing. A full-suite run under that role finds these.
- The append-only rule for audit entries is a `REVOKE` aimed at the role running the migration, so it must be restated for
  the application role.
- River's leader reindexes on a schedule; the River maintenance page shows a daily default
  ([River](https://pkg.go.dev/github.com/riverqueue/river@v0.34.0/internal/maintenance)), and `REINDEX` needs ownership or,
  per newer manuals, the `MAINTAIN` privilege. How River behaves under a non-owner was not found. Plan: grant `MAINTAIN` on
  `river_*` tables or turn that task off, and verify.

### Valkey

| Option | Monthly | For | Against |
|---|---|---|---|
| `cache.t4g.micro`, one node | about $9.34 at $0.0128 per hour ([Upstash, 2026-07](https://upstash.com/blog/aws-elasticache-pricing-explained-2026-full-cost-breakdown), [CloudPrice](https://cloudprice.net/aws/elasticache?region=us-east-1)) | Same engine behaviour as compose | Not free when idle; 0.5 GiB |
| Serverless | from about $6.13: $0.084 per GB-hour on a 100 MB minimum, plus $0.0023 per million ECPUs ([pricing](https://aws.amazon.com/elasticache/pricing/)) | Scales | Does not support `CLIENT TRACKING`, `PSUBSCRIBE` or `WAIT` ([commands](https://docs.aws.amazon.com/AmazonElastiCache/latest/dg/SupportedCommands.html)), and the cache relies on client-side caching. Forces TLS and cluster behaviour |

Pick: one node-based `cache.t4g.micro`, in-transit encryption on, AUTH token. AUTH needs encryption in transit, tokens are
16 to 128 printable characters, and the micro node supports Valkey from 7.2
([AUTH](https://docs.aws.amazon.com/AmazonElastiCache/latest/dg/auth.html),
[node types](https://docs.aws.amazon.com/AmazonElastiCache/latest/dg/CacheNodes.SupportedTypes.html)). Versions 8.0, 8.1,
8.2 and 9.x appear in the docs ([versions](https://docs.aws.amazon.com/AmazonElastiCache/latest/dg/engine-versions.html));
pick an 8.x matching `compose.yaml`'s `valkey:8`. `CONFIG` is restricted on all ElastiCache
([commands](https://docs.aws.amazon.com/AmazonElastiCache/latest/dg/SupportedCommands.html)); whether a library calls it
at start was not checked, so the first boot proves it. Same for client tracking, which the page lists as unavailable only
for serverless.

### S3

Two private buckets: files (default disk) and inbound mail (`ses_inbound` disk). SES stores at most 40 MB per message, and
does not support buckets with object lock default retention
([S3 action](https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-s3.html)). Use bucket default SSE-S3 and leave
the SES message encryption option off: that option is client-side encryption needing the S3 encryption client, which AWS
names for Java and Ruby, and the Go driver reads objects directly
([S3 action](https://docs.aws.amazon.com/ses/latest/dg/receiving-email-action-s3.html)). A bucket policy lets SES write,
scoped by source account and rule ARN
([permissions](https://docs.aws.amazon.com/ses/latest/dg/receiving-email-permissions.html)). Lifecycle: expire `raw/` after
31 days, per `specs/04-email-channel/pipeline.md`.

### Keys and secrets

One KMS customer key as `encryption.kek` so staging proves that path: $1 per month, 20,000 requests free
([KMS pricing](https://aws.amazon.com/kms/pricing/)). RDS and S3 use AWS-managed encryption, which has no key charge.

Secrets Manager: $0.40 per secret per month, $0.05 per 10,000 calls
([pricing](https://aws.amazon.com/secrets-manager/pricing/)). ECS can inject a whole secret or a JSON key into an
environment variable at task start; a change needs a new deployment
([ECS secrets](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/secrets-envvar-secrets-manager.html)). Since the
app reads a whole connection string, secret values need composing. CDK can build one secret from others
(`secretObjectValue`, [CDK secrets](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.aws_secretsmanager-readme.html)),
though the docs also warn secret values work only in select properties. So: try that at first synth; fall back to one
documented script that writes the URL secrets once. RDS passwords may not contain `/`, `'`, `"`, `@` or a space
([RDS limits](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/CHAP_Limits.html)); generate without punctuation so
URLs need no escaping. No new app setting is needed.

## 6. Network and edge

Prices: public IPv4 $0.005 per hour in use or idle; gateway endpoints free; NAT figure on the page is for Ohio, $0.045 per
hour and per GB ([VPC pricing](https://aws.amazon.com/vpc/pricing/)). ALB $0.0225 per hour plus $0.008 per LCU-hour; an LCU
is 25 new connections per second, 3,000 active per minute, 1 GB per hour, or 1,000 rule evaluations per second with 10 rules
free ([ALB pricing](https://aws.amazon.com/elasticloadbalancing/pricing/)).

| Egress option | Monthly | Notes |
|---|---|---|
| NAT gateway | at least $32.85 plus data, one AZ, Ohio rate | The proper production shape. Most expensive line in staging |
| NAT instance such as fck-nat | about $3 to $7 (from memory) | One more host to run |
| Tasks in public subnets with a public IP | $3.65 per task | Security group admits only the ALB. Pick for staging |
| Interface endpoints | not used | Do not cover SES, OAuth providers or SNS certificates; and the guarded client refuses private addresses, needing `http_client.allow_hosts` |

Tasks must reach the internet: SES API, SNS certificates, OAuth providers, DNS. Fargate needs a public IP or NAT to pull
images ([Fargate networking](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/fargate-tasks-services.html)). An
S3 gateway endpoint is free; its addresses stay public so the guard is unaffected.

WAF: $5 per ACL, $1 per rule, $0.60 per million requests ([WAF pricing](https://aws.amazon.com/waf/pricing/)). Not worth it
behind the gate in section 10. CloudFront: no.

Logs and alarms: logs $0.50 per GB ingested from examples; the free tier covers 5 GB of logs and 10 alarm metrics
([CloudWatch pricing](https://aws.amazon.com/cloudwatch/pricing/)). The app logs JSON by default
(`framework/log/log.go:52`). Traces stay off. `/metrics` stays on loopback; ALB, ECS, RDS and ElastiCache metrics cover
staging.

Admin and debugging: ECS Exec runs commands through Session Manager with no open port, works on Fargate, is audited, needs
a writable root filesystem, and needs a shell in the image
([ECS Exec](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs-exec.html)). The planned Debian image with
`postgresql-client-18` (`specs/11-self-host/packaging.md`) gives `psql` inside a task. This means no read-only root on
staging, and no bastion.

## 7. Email

Sandbox: send only to verified addresses or the simulator, 200 messages per 24 hours, 1 per second, per Region. Production
access asks for mail type, website URL, up to 4 contacts, language, and an acknowledgement that you send only to people who
asked and handle bounces and complaints. AWS gives an initial response within 96 hours, and having a verified domain first
helps ([production access](https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html)). The page does not
say that receiving is limited in the sandbox.

Pricing: since 2026-07-21 new SES accounts start on the Essentials plan, $0 fixed and $0.16 per 1,000 outbound; a la carte
is $0.10 per 1,000. Inbound $0.10 per 1,000 plus $0.09 per 1,000 256 KB chunks. Attachments $0.12 per GB. 1,000 tenants
included on plans ([SES pricing](https://aws.amazon.com/ses/pricing/)). Staging volume costs cents. Mail Manager is not
needed; its open ingress endpoints cost $50 each, and classic receipt rules do the job.

Simulator: `success@`, `bounce@`, `ooto@`, `complaint@`, `suppressionlist@simulator.amazonses.com`; labels such as
`bounce+x@` work. Works in the sandbox, does not count against quota or reputation, but is billed as normal
([simulator](https://docs.aws.amazon.com/ses/latest/dg/send-an-email-from-console.html)). Reject events can be tested with an
EICAR attachment per the same page, and those count against quota.

Receiving: `SetActiveReceiptRuleSet` sets the active set, and a null name disables receiving
([API](https://docs.aws.amazon.com/ses/latest/APIReference/API_SetActiveReceiptRuleSet.html)). A per-Region active set is
described in the docs, though the fetched pages do not state "one only" in those words. `ScanEnabled` defaults to false, so it
must be turned on to get spam and virus verdicts
([rule API](https://docs.aws.amazon.com/ses/latest/APIReference/API_ReceiptRule.html), via search, not read in full). Rules
with no recipients match all verified domains, so list recipients explicitly.

SNS: all 5XX and 429 are retryable; other errors are permanent. Without a custom HTTP policy a delivery is retried only a few
times, and the total retry time for HTTP/S cannot exceed 3,600 s. A dead-letter queue keeps what exhausts the policy
([SNS retries](https://docs.aws.amazon.com/sns/latest/dg/sns-message-delivery-retries.html)). So a custom policy of about an
hour plus an SQS DLQ is needed, because the raw mail stays in S3 and can be replayed. This also rules against stopping the app
overnight without accepting lost notifications.

Reputation isolation: separate account means separate quota, suppression and account reputation
([SES regions](https://docs.aws.amazon.com/ses/latest/dg/regions.html)). Sub-domain isolation is less certain. Google's own
statement was not found, and secondary sources disagree on whether subdomain signals reach the parent
([Suped, separate](https://suped.com/email-deliverability/answers/does-google-postmaster-tools-domain-reputation-include-subdomains),
[Suped, aggregate](https://suped.com/email-deliverability/answers/does-subdomain-data-roll-up-to-the-root-domain-in-google-postmaster-tools)).
A claim that staging "can never" hurt production cannot be made. Low volume, known recipients, a gate, and simulator use make
it unlikely. A wholly separate domain is the only certain answer, and is offered as a decision.

Tenants: `AWS::SES::Tenant` exists and CDK has `CfnTenant` L1 only
([CloudFormation](https://docs.aws.amazon.com/AWSCloudFormation/latest/TemplateReference/aws-resource-ses-tenant.html),
[CDK readme](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.aws_ses-readme.html)). Default quota 10,000 per Region
([quotas](https://docs.aws.amazon.com/general/latest/gr/ses.html#limits_ses_quota)). Three policies: Standard, Strict, None
([announcement](https://aws.amazon.com/about-aws/whats-new/2025/08/amazon-ses-tenant-isolation-automated-reputation-policies/)).
Tenants belong to workspaces, so the app creates them, not CDK. Not built (spec 04 M8, M11).

## 8. CDK

| Language | For | Against |
|---|---|---|
| TypeScript | pnpm, Biome and `tsc` already in the repo. CDK's home language; every example uses it | Another package in the repo |
| Go | Same language as the app | CDK Go still needs Node through jsii. Its dependencies would sit in or beside `go.mod`, and would meet depguard and the comment rules, which fit poorly |

Pick: TypeScript in `deploy/cdk`, inside the pnpm workspace, Biome clean, typechecked by `pnpm typecheck`.

Gaps found in `aws-cdk-lib` 2.275 docs:
- `EmailIdentity` with `Identity.publicHostedZone` creates DKIM and MAIL FROM records automatically. Good.
- `ConfigurationSet.addEventDestination` supports SNS. Good.
- `ReceiptRuleSet` has no activation option ([docs](https://docs.aws.amazon.com/cdk/api/v2/docs/aws-cdk-lib.aws_ses.ReceiptRuleSet.html)).
  Fill with `AwsCustomResource` calling `setActiveReceiptRuleSet`.
- `dropSpam` adds a Lambda rule; do not use it, because the app keeps spam verdicts and classifies.
- Production access: API or console only. Manual, deliberately.
- ElastiCache: L1 `CfnReplicationGroup`.
- SNS subscription delivery policy: L1 `CfnSubscription`.
- ALB rules with fixed responses: L2 covers them.

Avoid `fromLookup` and commit no `cdk.context.json`, because it caches account ids and the repository is public. Account id
and Region arrive by environment. `cdk bootstrap` is required per account and Region, and termination protection is advised
([bootstrap](https://docs.aws.amazon.com/cdk/v2/guide/bootstrapping-env.html)).

## 9. CI/CD

OIDC: provider `https://token.actions.githubusercontent.com`, audience `sts.amazonaws.com`, trust restricted by `sub`, with
`id-token: write` and `aws-actions/configure-aws-credentials`. Repositories created after 2026-07-15, or opted in, get
immutable subject claims with owner and repository ids
([GitHub OIDC](https://docs.github.com/en/actions/how-tos/secure-your-work/security-harden-deployments/oidc-in-aws)), so read
the actual claim first. With a GitHub environment the `sub` carries it, which is tighter than the branch form.

Runners: `ubuntu-24.04-arm` has 2 vCPU and 8 GB, private-repo availability came 2026-01-29, public repos have been free since
2025 ([runners](https://docs.github.com/en/actions/reference/runners/github-hosted-runners),
[changelog](https://github.blog/changelog/2025-01-16-linux-ARM64-hosted-runners-now-available-for-free-in-public-repositories-public-preview/)).
So native ARM image builds cost nothing while the repo is public. Private-repo prices were not settled by sources found.

Migration order: expand-only migrations are enforced by lint and mean the old release keeps working on the new schema, so:
migrate first, then roll tasks. A rollback never undoes a migration. Contract migrations ship one release later and are
marked. `migrate --force` is the automation form.

ECR: $0.10 per GB-month from the page's example ([ECR pricing](https://aws.amazon.com/ecr/pricing/)).

## 10. Keeping the public out

| Option | Cost | For | Against |
|---|---|---|---|
| ALB listener rules: source IP or secret header, with SNS webhook path open | free | No app change. Health checks unaffected | Changing IPs need a rule update. Limits: 100 rules, 5 condition values per rule ([quotas](https://docs.aws.amazon.com/elasticloadbalancing/latest/application/load-balancer-limits.html)) |
| WAF IP set | about $6 plus requests | Clean | Costs more for the same thing |
| ALB OIDC authentication | free | Real sign-in | Cookies and redirects that production does not have; contradicts "used exactly like production" |
| No gate, invite-only signups | free | Most like production | Sign-up flow cannot be tested, and public can reach it. Combined with noindex only |

Pick: listener rules, with `/robots.txt` as a fixed response. The SPA already sends `noindex` (`resources/views/spa/shell.templ:18`).
Signups stay open behind the gate so the flow can be tested. Sending limits are not built (spec 04 M11), so opening the gate
later needs them first.

## 11. Costs and production

See the cost table in the spec. Credits: accounts created from 2025-07-15 may receive up to $200 in credits, used within 12
months ([SES pricing](https://aws.amazon.com/ses/pricing/), [ALB pricing](https://aws.amazon.com/elasticloadbalancing/pricing/));
whether a new member account of an existing organization qualifies was not confirmed. `research/pricing.md` holds a production
baseline of about $1,450 per month; it was not recomputed here.

## 12. Not confirmed

- Whether Vercel accepts NS records on a subdomain in practice. Documented record type only. Step one proves it.
- RDS `db.t4g.micro` price and gp3 price from AWS itself: tables did not render. Third-party figure used; gp3 $0.115 per GB is
  from memory.
- PostgreSQL 18 on `db.t4g.micro` from an AWS page: third-party listing only.
- ElastiCache micro price from AWS itself: third-party figures, one outlier at about $12.
- NAT and interface endpoint prices for N. Virginia: only Ohio's NAT rate was shown.
- ElastiCache node-based behaviour with rueidis client tracking, and whether any library calls a restricted command.
- River under a non-owner role.
- Whether SES receiving is unrestricted in the sandbox: not stated. Long-standing behaviour says yes.
- That one receipt rule set only may be active per Region: not literally stated on pages read, implied by the API.
- Google's position on subdomain reputation.
- CDK composing one secret from another for a connection string.
- App Runner closure: secondary sources; AWS's pricing page has no notice.
- Fargate `stopTimeout` maximum; EC2 and fck-nat prices: from memory.
- Anything about the owner's existing account, Vercel team or GitHub settings: not read, by instruction.
- Whether a build track covers a blocker: `specs/BUILD-TRACKS.md` does not exist, and task boxes in spec 04 are all open though
  much is built, so "covered by" names spec tasks, not who is doing them.

## 13. Owner review, 2026-11-04

The owner changed the plan after review. These stand under any compute choice and replace the matching parts above. **Sections 2 (accounts), 4 (compute),
6 (network and edge), 9 (deploy order) and 10 (gate) are superseded where they recommend Organizations, Fargate, an ALB, public-subnet tasks or ALB rules.** Compute and
deployment are being chosen by the owner and are not recorded here as decided.

### One account

Sandbox status is per Region and production access must name each Region ([production access](https://docs.aws.amazon.com/ses/latest/dg/request-production-access.html)). Quotas are
separate for each Region ([limits](https://docs.aws.amazon.com/ses/latest/dg/manage-sending-quotas.html)). Identities, Easy DKIM, suppression list, feedback settings and receiving resources are per Region
([SES regions](https://docs.aws.amazon.com/ses/latest/dg/regions.html)). `GetAccount` describes status for the current Region
([API](https://docs.aws.amazon.com/ses/latest/APIReference-V2/API_GetAccount.html), via search), and pausing is per Region
([pausing](https://docs.aws.amazon.com/ses/latest/dg/monitoring-sender-reputation-pausing-account.html)). No page read states that a review in one Region leaves another Region unaffected: likely, not confirmed.
Bounce above 5% or complaints above 0.1% place an account under review, per that pausing page; another AWS page gives further pause thresholds, and they are not fully consistent
([alarms](https://docs.aws.amazon.com/ses/latest/dg/reputationdashboard-cloudwatch-alarm.html)).

### One host

`staging.trydimmi.com` serves everything. Code check: `/hc/{center}` is the only help center mode built; `help_center.base_domain` is in spec 08 only, and empty means path mode
(`specs/08-help-center/README.md:1018`). So no wildcard is needed. Spec 08 line 1178 wants a separate registrable domain on the cloud for cookie isolation, which single-host staging does not exercise.

### TLS in the binary

Confirmed: `TLSConfig()` is defined (`framework/customdomain/tls.go:169`, `drivers/certmagic/certmagic.go:167`) and no listener uses it. A search for `ServeTLS`, `tls.NewListener` and `tls.Listen` in `framework/`, `app/`
and `bootstrap/` finds nothing, and `framework/foundation/serve.go` serves plain TCP. `foundation.Options.Domains` is never set, so the default authorizer refuses every hostname (`boot_web.go:209`), including `app.url`'s.
The driver sets TLS 1.2 minimum, `h2`, `http/1.1` and the ACME protocol, and stores everything in PostgreSQL under a lock.

certmagic: instances sharing storage act as one cluster, with "distributed solving of all challenges (works behind load balancers)"; HTTP challenge needs port 80, TLS-ALPN 443
([certmagic](https://pkg.go.dev/github.com/caddyserver/certmagic)). Let's Encrypt: 5 certificates per exact set of names per 7 days, no overrides; 50 per registered domain per 7 days, which staging and production share since both are under `trydimmi.com`;
use its staging environment for tests ([limits](https://letsencrypt.org/docs/rate-limits/)). SNS needs a certificate from a CA it trusts and times out in about 15 s; by default it makes up to 3 retries, 20 s apart
([SNS HTTPS](https://docs.aws.amazon.com/sns/latest/dg/SendMessageToHttp.prepare.html)).

### NLB with passthrough, production design only

- Client IP: preserved by default for instance targets; for IP targets with TCP it is off by default and can be turned on. Hairpinning is not supported with preservation
  ([attributes](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/edit-target-group-attributes.html)). Matters if containers register by IP.
- Idle: 350 s default for TCP, 60 to 6000 s ([idle timeout](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/update-idle-timeout.html)).
- A TCP connection is routed to a single target for its life ([overview](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/introduction.html)).
- Deregistration default 300 s, range 0 to 3600, at least 120 s advised; existing connections stay unless termination is enabled. TCP target groups accept HTTP, HTTPS or TCP health checks
  ([target groups](https://docs.aws.amazon.com/elasticloadbalancing/latest/network/load-balancer-target-groups.html)).
- $0.0225 per hour and $0.006 per NLCU-hour; an NLCU is 800 new TCP connections per second, 100,000 active, or 1 GB per hour ([pricing](https://aws.amazon.com/elasticloadbalancing/pricing/)).
- AWS does not mention ACME, WebSocket or SSE on those pages. That they work is an inference from the NLB being layer 4 with bytes passed through.

### Gate in the app

Public by necessity. Reasoning and rules are in the spec. Note: `/up`, `/ready` and `/connection/*` sit on the mux before the kernel (`framework/foundation/serve.go`), so kernel middleware does not cover them.

### Not confirmed, added

- SES review isolation between Regions in one account.
- EC2 prices: AWS's table did not render, and third-party figures conflict. T4g surplus credits are $0.04 per vCPU-hour ([EC2 pricing](https://aws.amazon.com/ec2/pricing/on-demand/)).
- Whether SNS trusts Let's Encrypt: no list on the page read; proved at first subscription.
- HTTP/2 beside WebSocket on the new listener: to be proved by test.
