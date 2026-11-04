# 04 Email channel: staging and production-grade testing

Style: caveman. Owner: 04-email-channel. Tasks: [tasks.md](tasks.md) M20. Track 05 email-sending. AWS account and DNS are human step (section 8). Code and infrastructure files ship ready. Local tests stay fakes + Mailpit + SeaweedFS exactly as now: nothing here runs in `make test`.

## 1. Layers

| Layer | Runs | Real what | Where |
|---|---|---|---|
| Unit + integration | every commit | Postgres, Valkey, S3, Mailpit; fakes at HTTP boundary | `make test` |
| e2e | every commit | browser, binaries; `/webhooks/test/inbound` | `make e2e` |
| IaC check | every commit touching `deploy/tofu` | `tofu fmt -check`, `tofu validate` (no credentials, `-backend=false`) | `make infra-check` |
| SES simulator | hourly on staging | SES send + events | `RunEmailRoundTrip` |
| Probe round trip | hourly on staging | real mail both ways, one real mailbox per host | `RunEmailRoundTrip` |
| Mailbox matrix | nightly + before release | six mailbox hosts, every direction, headers read | `make email-matrix` (tag `emailmatrix`) |
| Provider adapters | before release, on change | Postmark, Mailgun, SendGrid, Resend, Cloudflare, IMAP, OAuth | same harness, `-adapter` |

Staging runs cost SES and storage only. No LLM anywhere (X16). Each manual real run: owner approval per project rule on third-party spend.

## 2. Separate AWS account

`dimmi-email-staging`, member of AWS Organization, own billing alarm at 20 USD/month. Never shares SES reputation, identities or suppression list with production. Region `us-east-1` (SES receiving available, same as production). Production account unchanged by staging applies.

SES production access requested for staging too: sandbox blocks unverified real recipients. Simulator works in sandbox, so simulator checks work from day one.

## 3. Domains

Delegated zone `staging.trydimmi.com` (NS in parent, Route 53 in staging account).

| Name | Use |
|---|---|
| `inbound.staging.trydimmi.com` | MX `inbound-smtp.us-east-1.amazonaws.com`, SES receiving, default sender |
| `acme.staging.trydimmi.com` | custom sending domain for SES path, DKIM + MAIL FROM + DMARC `p=reject`, proves section 11 |
| `broken.staging.trydimmi.com` | deliberately bad: duplicate SPF, doubled name, for doctor and checker |
| `pm.staging.trydimmi.com`, `mg.`, `sg.`, `rs.`, `cf.` | inbound and outbound per provider adapter. `cf.` zone on Cloudflare for Email Routing and Domain Connect check |
| `app.staging.trydimmi.com` | staging app |

## 4. Real mailboxes

| Host | Account | Role | How harness reads |
|---|---|---|---|
| Gmail consumer | `dimmi.staging.probe@gmail.com` | customer; also Gmail forward path | IMAP app password |
| Google Workspace | `support@wsp.staging...` (1 seat, own test domain `dimmi-wsp-test` human-chosen) | routing rule path; Group `group@` in front case; OAuth connect | Gmail API token in secrets |
| Outlook.com | `dimmi-staging-probe@outlook.com` | customer | Graph device-code token |
| Microsoft 365 | Business Basic seat, shared mailbox `support@` | external forward policy; OAuth + shared poll | Graph token |
| Yahoo | app password | customer | IMAP |
| iCloud | app-specific password | customer | IMAP |

Credentials only in AWS Secrets Manager, staging account, `dimmi/email-matrix/*`. Harness reads them at start. Never env files, never repo.

## 5. Scenarios

### 5.1 SES mailbox simulator (hourly, `RunEmailRoundTrip`)

Sent through real customer stream from `acme.staging`. Sends labeled `+<run id>` so events match run. No effect on reputation or quota, billed normally.

| Address | Expect within 5 min |
|---|---|
| `success+<id>@simulator.amazonses.com` | SNS Delivery, `outbound_emails.status = delivered` |
| `bounce+<id>@simulator.amazonses.com` | Bounce 5.1.1, suppression row created, message failed |
| `complaint+<id>@simulator.amazonses.com` | Complaint, suppression scope `all` |
| `ooto+<id>@simulator.amazonses.com` | RFC 3834 auto-response arrives at return path, classified `auto_reply`, no notify |
| `suppressionlist+<id>@simulator.amazonses.com` | hard bounce, `OnAccountSuppressionList` mapping |

Reject event: EICAR test attachment to verified address, nightly only (counts quota). Cleanup: probe workspace rows removed at end; suppressions lifted so next run works.

### 5.2 Probe round trip (hourly)

Reuses setup test ([setup.md](setup.md) 3). Staging workspace `probe`, one `email_addresses` row per path. Job posts test per address; test row result is the check.

| Path | Route | Bar |
|---|---|---|
| direct | app -> SES -> `probe@inbound.staging` -> S3 + SNS -> webhook -> pipeline | p95 under 60 s |
| gmail | app -> Gmail -> filter forward -> SES inbound | under 5 min |
| workspace | app -> Workspace -> routing rule -> inbound | under 5 min |
| m365 | app -> 365 -> forward -> inbound | under 5 min |

Alert: section 6.

### 5.3 Mailbox matrix (nightly; `tests/emailmatrix`, build tag `emailmatrix`, never in `make test`)

For each of six mailboxes:

1. dimmi replies from `acme.staging` to mailbox. Read message there: inbox not spam; `Authentication-Results` shows SPF, DKIM, DMARC pass and alignment; no tracking; signature; attachment; thread header.
2. Mailbox replies from its own client path (SMTP app password or API): quoted reply lands in same conversation under 60 s; quote stripped; no new contact.
3. Mailbox sends new mail to forwarding address; conversation; contact verified when DMARC aligned.
4. dimmi reply with Cc to second mailbox: both receive, reply-all from Cc lands in thread (F16 fix).

Output: JSON + table artifact, per-host pass/fail, spam-folder count. Fails build of release when any host spam-folders twice in a row.

Harness: Go, uses `framework/mailbox/imap`, gmail, graph drivers, go-mail for SMTP. Same code as product.

### 5.4 Adapter runs

`make email-matrix ARGS="-adapter postmark"`: send to `x@pm.staging...`, confirm webhook, signature, raw, verdict fields; save sanitized payload to `framework/mailbox/drivers/postmark/testdata` when `-record`. Closes every `[staging]` mark in [sources.md](sources.md) and verify paths in [sending.md](sending.md) 7. OAuth run: real consent once by human on Workspace + 365, then sync, send, renew, revoke, reconnect, shared mailbox poll.

## 6. Scheduled check and alerts

`dimmi email:round-trip` command + periodic `RunEmailRoundTrip` (queue `maintenance`, hourly). Registered only when `channels.email.round_trip.enabled` (default false; cloud production: on with `direct` and simulator paths only, so production mail also proven).

Config `channels.email.round_trip.{enabled, workspace, simulator, alert_webhook_url, heartbeat_url}`.

Metrics: `dimmi_email_round_trip_success{path}`, `dimmi_email_round_trip_seconds{path}`, `dimmi_email_round_trip_last_success_timestamp{path}`.

Alerts:
- 2 consecutive failures on a path: POST `alert_webhook_url` (Slack-shaped JSON) + system mail to `ops_forward_to`. Text names path, stage reached, last good time. System mail itself is a stream the check does not prove: webhook is primary.
- Dead man: each success GET `heartbeat_url` (any cron monitor); monitor alerts when silent 2 h, covers app dead or SES fully down.
- CloudWatch alarms from IaC: SES `Reputation.BounceRate` over 5%, `Reputation.ComplaintRate` over 0.1%, SNS `NumberOfNotificationsFailed` over 0, S3 inbound object count zero for 6 h, to SNS topic `ops` with email subscriber (human confirms).
- `make test` covers job with fake clock, `tc.Mail`, `httpclienttest` for webhook and heartbeat: stage timeout, two-failure gate, success resets, disabled means not scheduled.

## 7. Infrastructure as code

**Tool: OpenTofu.** MPL-2.0 (Terraform is BUSL: outside repo license list). HCL same, AWS provider has every resource needed, also drives Cloudflare and Route 53 so `cf.` zone lives in same plan. Native S3 state lock (`use_lockfile`, 1.10+), no DynamoDB. Pin `required_version = "~> 1.10"`. Conflict: `12-cloud-billing/infrastructure.md` says CloudFormation for cloud stacks; that doc is outline. Reported to owner; email stacks use OpenTofu. Should other stacks follow, one tool remains.

Layout:

```
deploy/tofu/
  README.md                     # plan and apply steps, bootstrap, no secret values
  modules/email-ses/
    main.tf variables.tf outputs.tf versions.tf
    receiving.tf                # receipt rule set, active set, rule: S3 + scan, recipient = inbound domain
    storage.tf                  # S3 inbound bucket, lifecycle raw/ 31 days, block public, SSE, bucket policy ses.amazonaws.com + aws:SourceAccount + aws:SourceArn
    notifications.tf            # SNS inbound + events topics, HTTPS subscriptions, topic policy
    sending.tf                  # identity for inbound domain, DKIM, MAIL FROM, config sets dimmi-system, dimmi-customer, event destinations
    iam.tf                      # app role: ses:SendEmail on identities, sesv2 identity + tenant actions, s3:GetObject raw/, s3:DeleteObject raw/ (erase)
    alarms.tf                   # section 6
    dns.tf                      # optional route53 records when var.route53_zone_id set; else outputs records for human
  envs/staging/
    main.tf backend.tf terraform.tfvars.example
  envs/production/
    main.tf backend.tf terraform.tfvars.example
  modules/mail-probe-cloudflare/ # cf. zone: Email Routing rule to worker, optional
deploy/domainconnect/trydimmi.com.email.json
```

Resources: `aws_ses_receipt_rule_set`, `aws_ses_active_receipt_rule_set`, `aws_ses_receipt_rule` (`s3_action` with `topic_arn`, `scan_enabled = true`, `tls_policy = "Optional"`), `aws_s3_bucket*`, `aws_sns_topic*`, `aws_sesv2_email_identity`, `aws_sesv2_email_identity_mail_from_attributes`, `aws_sesv2_configuration_set`, `aws_sesv2_configuration_set_event_destination`, `aws_iam_role*`, `aws_cloudwatch_metric_alarm`. Customer domains and tenants are never IaC: app creates them.

State: S3 bucket `dimmi-tofu-state-<env>` in same account, versioning on, SSE-KMS, `use_lockfile = true`, public block. Bootstrap once by hand (`deploy/tofu/README.md` lists the commands). State holds outputs only: no secret resources in Tofu; DB URLs, API keys and mailbox credentials go to Secrets Manager by human or by app config, values never in `.tf`, `.tfvars`.

Repo rules: `.gitignore` `*.tfstate*`, `*.tfvars` (only `.example` committed), `.terraform/`. `make infra-check` runs fmt + validate + `tests/arch/deploy_secrets_test.go` (fails on AWS key pattern, `password =` literals, private key blocks under `deploy/`). Lock file `.terraform.lock.hcl` committed.

Binary unaffected; `deploy/` not embedded.

## 8. Human checklist

No code path waits on these. Order matters.

AWS and DNS:
- [ ] Create AWS account `dimmi-email-staging` in Organization; billing alarm.
- [ ] Bootstrap state bucket (`deploy/tofu/README.md`).
- [ ] Delegate `staging.trydimmi.com` NS to Route 53 zone.
- [ ] `tofu apply` staging; confirm SNS `ops` email.
- [ ] Request SES production access in staging (support case). Same for production when ready.
- [ ] Set staging app config from Tofu outputs; confirm SNS HTTPS subscription auto-confirms.
- [ ] Production: repeat in production account, inbound on `inbound.trydimmi.com`.

Mailboxes:
- [ ] Create Gmail, Outlook.com, Yahoo, iCloud accounts; 2FA; app passwords; store in Secrets Manager.
- [ ] Buy one Google Workspace seat on test domain; set routing rule per guide; add Google Group case.
- [ ] Buy one Microsoft 365 Business Basic seat; shared mailbox; forward policy; external forwarding scoped.
- [ ] Set Gmail filter and forward on probe account; confirm code in dimmi.

OAuth:
- [ ] Google Cloud project; OAuth client; redirect `https://app.staging.trydimmi.com/channels/email/google/callback`; test users; Pub/Sub topic, grant `gmail-api-push@system.gserviceaccount.com`, push subscription with OIDC service account.
- [ ] For cloud launch: Google brand + restricted-scope verification, CASA assessment (third party, paid, annual). Until then `oauth.google.enabled = false` in production.
- [ ] Azure app registration, multitenant, secret or cert, redirect, Publisher verification.

Providers:
- [ ] Test accounts Postmark (inbound-capable plan), Mailgun, SendGrid, Resend, Cloudflare; MX on subdomains; webhooks; secrets in Secrets Manager.
- [ ] Cloudflare: deploy generated worker on `cf.` zone.

Domain Connect:
- [ ] Generate RSA key; publish public key TXT at `syncPubKeyDomain`; private key in Secrets Manager.
- [ ] PR template to `Domain-Connect/Templates`; after merge mail `domain-connect@cloudflare.com` with template links, FQDNs, SVG logo, proxy defaults (DNS only). Then other hosts as they accept.

Upkeep:
- [ ] Quarterly: click DNS host deep links ([sending.md](sending.md) 6.1) and setup guide steps against current host UIs.
- [ ] Yearly: CASA recertification (Google mails Owner and Editor about 90 days ahead).

## 9. Local stays local

- `make test`, `make e2e` need no AWS, no real mailbox, no provider, no OAuth client. Asserted: CI runs them with no such secret defined.
- Fakes unchanged: test-signed SNS + SeaweedFS for SES, `httpclienttest` for providers and OAuth, `tc.Mail`, `a.DNS.Fake()`, in-process IMAP server.
- `emailmatrix` tag never built by `make lint test` except compile check (`go vet -tags emailmatrix`), so harness cannot rot.
