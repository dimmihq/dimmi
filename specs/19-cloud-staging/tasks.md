# 19 Cloud staging: tasks

**Draft: awaiting owner approval.** Order matters inside a track. `[ ]` open. Owner's manual steps are in
[README.md](README.md), "Rollout plan". Do not touch any AWS, Vercel, GitHub or DNS account before approval. The repository
is public: no account ids, secret values or `cdk.context.json` in commits.

Size: S under a day, M 1 to 3 days, L more.

## Track A: the application can be deployed

- [ ] A1 (M to L) Run the whole app as a non-owner role in tests. `tests.New` connects as a role like `dimmi_app`, selected by one switch,
      with the suite run once that way in `make test`. List failures, then fix: cross-tenant jobs and queries need a security-definer function or
      a tenant set per workspace. Acceptance: full suite green under the role; `PruneAuditLogs`, `PruneSecurityEvents`, purge and maintenance jobs
      proved to act on rows.
- [ ] A2 (S) Role provisioning script `deploy/db/provision.sql`, idempotent, run by `psql` as master: creates `dimmi_owner`, `dimmi_app`, database, default
      privileges including River tables. Acceptance: run twice on a fresh PG18; migrate as owner; app connects as `dimmi_app`.
- [ ] A3 (S) Restate the audit log's append-only rule for the application role. Test: application role cannot update an entry.
- [ ] A4 (S) River under `dimmi_app`: decide `MAINTAIN` grant or disable reindexing; leader, periodic jobs and insert in a transaction tested.
- [ ] A5 (S) `deploy/docker/Dockerfile` per 11 packaging: Debian slim, `postgresql-client-18`, uid 65532, RDS CA bundle, `CGO_ENABLED=0`, build argument for
      tags, ARM and x86. Stamp version and commit through ldflags. Acceptance: `docker run ... serve` against compose, `/ready` passes, `psql --version` shows 18.
- [ ] A6 (S) Boot guards for the cloud edition in `production`: refuse a `local` default disk, the `log` mailer, a non-https `app.url` or one on localhost, empty
      `trusted_proxies`, and an `inbound_domain` unset by the operator. Self-hosted install untouched. Tests per guard. `make e2e` server config updated.
- [ ] A7 (S) Check and mask credentials inside connection URLs in effective-config output. Test with a URL holding a password.
- [ ] A8 (S) Update `docs/testing/manual-e2e.md` for the image.

## Track B: infrastructure and pipeline

- [ ] B1 (S) `deploy/cdk` TypeScript package in the pnpm workspace; Biome and `pnpm typecheck` cover it; stacks take account and Region from environment; no lookups;
      `cdk.context.json` ignored. `cdk synth` runs in CI with fake account.
- [ ] B2 (S) `Dns` stack: zone, outputs NS. `Ci` stack: ECR with lifecycle keep 20, OIDC provider, deploy role bound to the `staging` environment on this repository.
- [ ] B3 (S) `Network` stack: VPC, two AZs, public and isolated subnets, no NAT, S3 gateway endpoint, security groups.
- [ ] B4 (M) `Data` stack: RDS 18, parameter group, Valkey replication group with TLS and AUTH, S3 buckets, KMS key, secrets including composed URLs. Prove `secretObjectValue`
      composing; else `deploy/scripts/write-url-secrets`.
- [ ] B5 (M) `Mail` stack: identities with MAIL FROM, DMARC, configuration sets, topics, receipt rules with scan on, custom resource activating the rule set, DLQ.
- [ ] B6 (M) `App` stack: cluster, task definitions (web, worker, migrate, provision), services with circuit breaker, ACM, ALB, gate rules, records, SNS subscriptions with delivery
      policy, alarms, log groups. Image tag is a parameter. Task role scoped: SES send on identities and sets, S3 on two buckets, KMS on one key.
- [ ] B7 (M) `.github/workflows/ci.yml`: `make lint test`, `make e2e`, `cdk synth` on pull requests and `main`; `deploy-staging.yml` on `main` after CI: build ARM image, push, provision
      (idempotent), migrate as one-off and fail on non-zero, `cdk deploy App`, wait stable, smoke test; on smoke failure redeploy the previous tag. Actions pinned by SHA. One deploy at a
      time, a queued one replaces a queued one, a running one is never cancelled.
- [ ] B8 (S) Smoke script: `/ready`, version equals commit, sign-in page, CSP header present, WebSocket connects, test source refused, wrong SNS signature refused, `/robots.txt` disallows.
- [ ] B9 (M) `docs/deploy/` per README outline, every manual step in order, including the SES production access text. Reviewed for account-specific data.
- [ ] B10 (S) Update specs: `12-cloud-billing/infrastructure.md` (Fargate, CDK, idle 60 s), `04-email-channel` config keys to match the code, `README.md` folder table.

## Track C: mail both ways (spec 04)

- [ ] C1 (M) 04 M5: `POST /webhooks/ses/inbound` on the `mailbox` source, wired from `channels.email.inbound.drivers`. Record the reference's disk, not the configured one. Feature test with SNS fixture;
      S3 read tested on SeaweedFS.
- [ ] C2 (L) 04 M7: `POST /webhooks/ses/events`, `ProcessSesNotification`, suppression actions, system-stream routing. Unsubscribe follows.
- [ ] C3 (S) Scheduled round trip workflow, every 6 hours, through `POST /email-addresses/{id}/tests`, failing when no arrival within 2 minutes. Token in a GitHub secret.
- [ ] C4 (L) 04 M8 custom domains, then M11 limits and tenants. Limits come before the gate can open.

## Track D: testing on staging

- [ ] D1 (S) Checklist in `docs/deploy/09-testing-checklist.md`; first full walk recorded with dates.
- [ ] D2 (M) Playwright against staging: base URL and gate header from environment, a subset tagged for staging that needs no `psql`, no Mailpit, no test inbound; run after deploy. Mail-dependent tests read
      a staging inbound mailbox later.
- [ ] D3 (S) Deliberate failure drills: broken image rolls back, worker stopped and drained, Valkey reboot.

## Later, nice to have

- [ ] Maintenance state shared between replicas rather than per file.
- [ ] `dimmi healthcheck` and worker container health check (11).
- [ ] HSTS on pages.
- [ ] CSP for S3 attachment URLs once attachment routes exist.
- [ ] Overnight stop schedule.

## Done when

Pushing to `main` on GitHub runs checks, migrates and deploys to `https://app.staging.trydimmi.com` with no long-lived keys and no downtime. A bad release rolls back on its own. The app connects as a role
that owns nothing. A person, through a browser, passes every row of the README's testing table, with real mail through Gmail and Outlook, and the scheduled round trip stays green for a week. An operator can
rebuild the environment from `docs/deploy` alone.
