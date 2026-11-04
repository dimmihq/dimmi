# Deploying dimmi

dimmi's cloud edition runs on AWS: ECS Fargate on ARM behind a Network Load Balancer, RDS PostgreSQL 18, ElastiCache
Valkey, S3, SES, Secrets Manager and KMS, all defined in [`deploy/cdk`](../../deploy/cdk). DNS for every environment lives
at Vercel and is written by [`deploy/dns`](../../deploy/dns). GitHub Actions releases with short-lived credentials.

| Environment | Region | Host | Released |
|---|---|---|---|
| staging | us-east-2 | `staging.trydimmi.com` | On every push to `main`, after checks pass |
| production | us-east-1 | `trydimmi.com` | By hand, with an approval, using the image staging ran |

## What only a person can do

Everything else is automated. These steps need a person, a browser or a decision, and each is described where it occurs.

**Once per account**

1. Secure the AWS account: root user with MFA and no access keys, people sign in through IAM Identity Center or a named role.
2. Bootstrap CDK in each Region (`us-east-1`, `us-east-2`) and deploy `dimmi-account`, which lets GitHub in.
3. Create a Vercel token for the DNS script ([dns.md](dns.md#the-token)).
4. In GitHub, create the `staging` and `production` environments, their variables, and production's required reviewers.

**Once per environment**

5. Deploy the stacks: `pnpm exec cdk deploy --all -c env=<name>`, in the order the runbook gives.
6. Push the first image by hand. CI cannot until the stacks it needs exist.
7. Fill in the app secret: app key and OAuth client ids and secrets.
8. Confirm the alarm subscription mail sent to `ops@trydimmi.com`.
9. Run `pnpm dns apply` with the Vercel token.
10. Activate the `Environment` cost allocation tag in the billing console, so budgets and cost reports split by environment.
11. Request SES production access for the environment's Region ([ses-production-access.md](ses-production-access.md)).
12. Create the GitHub and Google OAuth apps, with callbacks on the environment's host.

**Every production release**

13. Start the `production` workflow with the commit sha, and approve it.

**When things go wrong**

14. Decide to roll back, and start the `rollback` workflow ([releases.md](releases.md#rollback)).
15. Any change to infrastructure: `cdk diff`, read it, then `cdk deploy`. CI never changes infrastructure.

## Pages

- [new-environment.md](new-environment.md): the ordered runbook for a new environment and its first deploy.
- [releases.md](releases.md): later deploys, rollback, pausing staging, teardown.
- [dns.md](dns.md): the Vercel token, `plan`, `apply`, `verify`, and certificates coming up.
- [ses-production-access.md](ses-production-access.md): leaving the SES sandbox, with draft wording.
- [costs.md](costs.md): monthly cost per environment and what is not confirmed.
