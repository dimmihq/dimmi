# 19 Cloud staging: tasks

**Draft: awaiting owner approval.** Compute and deployment are open, so packaging, pipeline and CDK tasks wait for that choice. Do not touch any AWS, Vercel, GitHub or DNS
account before approval. The repository is public: no account ids, secret values or `cdk.context.json` in commits.

Size: S under a day, M 1 to 3 days, L more.

## Track A: the application can run in the open

- [ ] A1 (M) HTTPS listener in `framework/foundation`: serve `Domains.TLSConfig()` on a configured port when the default driver terminates TLS, plain HTTP on another, `Domains.HTTPHandler`
      on the plain listener, redirect to HTTPS except ACME challenges and health paths, both stopped gracefully. HTTP/2 enabled, WebSocket over HTTP/1.1 proved. Tests against a local ACME server.
- [ ] A2 (S) Authorizer in the application allowing the `app.url` host, passed as `foundation.Options.Domains`. Certificate obtained after listeners are up, renewed in the background, survives restart with no new order.
      Let's Encrypt allows 5 certificates per exact name per 7 days, so first runs use its staging CA.
- [ ] A3 (S to M) Generic gate middleware, off by default, password cookie, CI key header, exempt paths, `robots.txt`, `X-Robots-Tag`. Framework code, no product words. Tests per exempt path; realtime and health unaffected.
- [ ] A4 (M to L) Run the whole app as a non-owner role in tests, one switch, once in `make test`. List failures, then fix. `PruneAuditLogs`, `PruneSecurityEvents`, purge and maintenance jobs proved to act on rows.
- [ ] A5 (S) Role provisioning script, idempotent: `dimmi_owner`, `dimmi_app`, database, default privileges including River tables.
- [ ] A6 (S) Restate the audit log's append-only rule for the application role, with a test.
- [ ] A7 (S) River under `dimmi_app`: `MAINTAIN` grant or reindexing off; leader, periodic jobs, insert in a transaction tested.
- [ ] A8 (S) Boot guards for the cloud edition: `local` default disk, `log` mailer, non-https or localhost `app.url`, unset `inbound_domain`. Self-hosted install untouched.
- [ ] A9 (S) Redaction review of effective-config output, with a test.

## Track B: infrastructure and pipeline

Waits for the compute choice. Regardless of it: TypeScript CDK at `deploy/cdk`, no lookups, GitHub Actions with OIDC, `docs/deploy` with every manual step, IAM scoped by prefix, tag and Region.

- [ ] B1 (S) `Dns` stack: zone for `staging.trydimmi.com`, outputs NS.
- [ ] B2 (M) `Mail` stack: identities, MAIL FROM, DMARC, configuration sets, topics, receipt rules with scan on, custom resource activating the rule set, DLQ.
- [ ] B3 (M) Data stores: isolated subnets, RDS 18, Valkey with TLS and AUTH, S3 buckets, KMS key, secrets.
- [ ] B4 Compute, packaging, deploy, rollback: to be written after the choice.

## Track C: mail both ways (spec 04)

- [ ] C1 (M) 04 M5: `POST /webhooks/ses/inbound`, recording the reference's disk. Exempt from the gate.
- [ ] C2 (L) 04 M7: `POST /webhooks/ses/events`, suppression, system-stream routing, unsubscribe.
- [ ] C3 (S) Scheduled round trip workflow every 6 hours.
- [ ] C4 (L) 04 M8 custom domains, then M11 limits and tenants, which come before the gate can come off.

## Track D: testing on staging

- [ ] D1 (S) Checklist in `docs/deploy`; first full walk recorded with dates.
- [ ] D2 (M) Playwright against staging: base URL and gate key from environment, a subset needing no `psql`, Mailpit or test inbound.

## Later, nice to have

- [ ] Maintenance state shared between replicas.
- [ ] HSTS on pages.
- [ ] CSP for S3 attachment URLs once attachment routes exist.
