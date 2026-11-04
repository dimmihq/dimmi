# A new environment, from nothing to its first release

Steps run in order. Each says who does it, what to run, what to expect, how long it takes, how to check it, and what to
do if it fails. **HUMAN** steps need a person; **AUTOMATED** steps are a command or a workflow.

Commands run from `deploy/cdk` unless they say otherwise, with credentials for the account in your shell (for example
`aws sso login` and `AWS_PROFILE`). Replace `staging` with the environment's name. Nothing below writes an account id
into the repository; keep yours in your shell or a password manager.

## 0. Before you start

- Node 24, pnpm, Docker with Buildx, the AWS CLI v2, and `pnpm install` in `deploy/cdk` and `deploy/dns`.
- The environment exists in `deploy/cdk/lib/config/environments.ts`. A new one is one entry there, in its own Region.
- `make infra-check` passes on your machine.

## 1. HUMAN: secure the account (once per account, 30 minutes)

Root user with MFA and no access keys. People use IAM Identity Center or a named role with MFA.

Check: the IAM console's security recommendations show no warnings for the root user.

## 2. HUMAN: bootstrap CDK (once per account and Region, 5 minutes)

```sh
pnpm exec cdk bootstrap aws://<account>/us-east-1 aws://<account>/us-east-2
```

Expect a `CDKToolkit` stack in each Region. If it fails with access denied, the role you used cannot create IAM roles; use
an administrator. The bootstrap execution role is effectively an administrator; only people deploy with it.

## 3. HUMAN: let GitHub in (once per account, 2 minutes)

```sh
pnpm exec cdk deploy dimmi-account -c env=staging
```

Creates the GitHub OIDC identity provider. Check: IAM, Identity providers lists `token.actions.githubusercontent.com`.
If it says the provider already exists, someone created it by hand: delete that one or skip this step.

## 4. HUMAN: deploy everything but the application (25 minutes, mostly RDS)

```sh
mkdir -p ../dns/outputs
pnpm exec cdk deploy -c env=staging -c paused=true --all --outputs-file ../dns/outputs/staging.json
```

`paused=true` starts the services at zero tasks: the database has no roles or schema yet. Expect eight stacks. RDS takes
10 to 15 minutes, Valkey about 10.

Check: `aws cloudformation describe-stacks --region us-east-2 --query 'Stacks[?starts_with(StackName, `dimmi-staging`)].[StackName,StackStatus]'`
shows every stack `CREATE_COMPLETE`.

If a stack fails, CloudFormation rolls it back; read the first `CREATE_FAILED` event in the console, fix, deploy again. A
name already taken (bucket, role) means an earlier attempt left it behind: delete it and retry.

## 5. HUMAN: confirm the alarm address (1 minute)

AWS mails `ops@trydimmi.com` twice, once for the ops topic and once for the budget. Click the confirmation link in the SNS
mail. Until then alarms reach nobody.

## 6. HUMAN: fill in the app secret (10 minutes)

`dimmi-staging/app` holds keys only a person can produce. Generate the app key and keep a copy in the password manager:
losing it loses every encrypted value.

```sh
printf 'base64:%s\n' "$(openssl rand -base64 32)"
```

Create OAuth apps at GitHub and Google with callbacks `https://staging.trydimmi.com/auth/github/callback` and
`https://staging.trydimmi.com/auth/google/callback`. Then write all five keys in the Secrets Manager console (edit the
secret as JSON, keep every key present, empty if unused). The gate password and CI key in `dimmi-staging/gate/*` are
generated; copy the password to the password manager for people who test staging.

## 7. HUMAN: push the first image (10 minutes)

CI pushes every later image; this first one comes from your machine, from the commit you want to run.

```sh
cd ../..   # repository root
REGISTRY=<account>.dkr.ecr.us-east-2.amazonaws.com
aws ecr get-login-password --region us-east-2 | docker login --username AWS --password-stdin "$REGISTRY"
docker buildx build --platform linux/arm64 --build-arg REVISION="$(git rev-parse HEAD)" \
  --tag "$REGISTRY/dimmi-staging:$(git rev-parse HEAD)" --push .
```

Check: `aws ecr describe-images --region us-east-2 --repository-name dimmi-staging` lists it.

## 8. AUTOMATED: provision, migrate, release (5 minutes)

```sh
ENVIRONMENT=staging AWS_REGION=us-east-2 deploy/release/release.sh "$(git rev-parse HEAD)"
```

It tags the image `release`, runs `provision` (creates `dimmi_owner` and `dimmi_app`), runs `migrate --force` as the
owner, and rolls the services, which are still at zero tasks. Expect `provision finished`, `migrate finished`, `released`.

If `provision` or `migrate` fails, read its log in CloudWatch, log group `/dimmi/staging`, stream prefix `provision` or
`migrate`. A `password authentication failed` means a password secret changed after the database was created; run again,
since provisioning resets every role's password.

## 9. HUMAN: start the services (5 minutes)

```sh
cd deploy/cdk
pnpm exec cdk deploy dimmi-staging-app -c env=staging --outputs-file ../dns/outputs/staging.json
```

Without `paused`, the web service starts its task. Check: the ECS console shows the service with one running task and the
load balancer's targets turning healthy within about two minutes. Before DNS exists the binary has no certificate for
the host, so check over plain HTTP, which serves health without redirecting: `curl http://<address>/ready`, with one of
the `LoadBalancerAddresses` outputs, answers 200.

If the task keeps stopping, its log in `/dimmi/staging` says why; a missing or malformed `app_key` is the usual cause. The
deployment circuit breaker stops retrying after a few failures; fix and deploy again.

## 10. HUMAN, then AUTOMATED: DNS (10 minutes plus propagation)

Follow [dns.md](dns.md): create the token, then from `deploy/dns`:

```sh
pnpm dns plan  --env staging --outputs outputs/staging.json
pnpm dns apply --env staging --outputs outputs/staging.json
pnpm dns verify --env staging --outputs outputs/staging.json
```

Expect the host's A records, and per mail domain an MX, SPF, DMARC, MAIL FROM MX and SPF, and three DKIM CNAMEs. Verify
until every record shows `ok`, usually within five minutes.

## 11. AUTOMATED: certificates and SES verification (5 to 60 minutes)

Once the host resolves, the binary orders its certificate from Let's Encrypt on the first HTTPS request and keeps it in
PostgreSQL. Check:

```sh
curl -sv https://staging.trydimmi.com/ready 2>&1 | grep -E 'issuer|HTTP/'
```

Expect a Let's Encrypt issuer and `200`. If the certificate does not come, the task log names the ACME error; the usual
cause is DNS still pointing elsewhere. Let's Encrypt allows five certificates per exact name per week, so do not delete
certificates from the database to retry.

SES checks the DKIM records by itself, usually within an hour. Check: the SES console in us-east-2 shows both identities
`Verified`. The SNS webhook subscriptions confirm once the host answers with a valid certificate.

## 12. HUMAN: GitHub (15 minutes)

In the repository settings, Environments:

- `staging`: deployment branch `main` only. Variables `AWS_REGION=us-east-2`, `AWS_ACCOUNT_ID`, and
  `AWS_DEPLOY_ROLE_ARN` (the `DeployRoleArn` output of `dimmi-staging-ci`).
- `production`: required reviewers, deployment branch `main` only. Variables `AWS_REGION=us-east-1`, `AWS_ACCOUNT_ID`,
  `AWS_DEPLOY_ROLE_ARN` from `dimmi-production-ci`, and `SOURCE_AWS_REGION=us-east-2`.

No secrets are needed: the workflows assume roles through OIDC. Check: the next push to `main` runs the `main` workflow
through `staging`, ending with `smoke: https://staging.trydimmi.com is healthy`.

## 13. HUMAN: billing and SES production access

- Billing console, Cost allocation tags: activate `Environment` and `Project`. Tags appear there a day after the first
  tagged resource exists, and the budget counts only once they are active.
- Request SES production access for the Region: [ses-production-access.md](ses-production-access.md). AWS answers within
  about a day. Until then staging sends only to verified addresses and the SES mailbox simulator.

## Production

The same steps with `-c env=production` and Region us-east-1, with two differences: in step 7 copy staging's image
instead of building one (the production workflow does the same), and in step 12 add required reviewers. Production's data
stack has termination protection and deletion protection; that is on purpose.
