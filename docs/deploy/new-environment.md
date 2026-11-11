# An environment's mail on AWS

AWS holds two stacks per environment: `ops` (the alarm topic and a budget) and `mail` (SES identities, receipt rules,
configuration sets, the inbound bucket, SNS topics and webhooks, and the IAM user the application signs in as).

Commands run from `deploy/cdk` with credentials for the account in your shell (`AWS_PROFILE`). Replace `staging` with
the environment's name. Nothing below writes an account id into the repository.

## 1. HUMAN: bootstrap CDK (once per account and Region, 5 minutes)

```sh
pnpm exec cdk bootstrap aws://<account>/us-east-1 aws://<account>/us-east-2
```

## 2. HUMAN: deploy the stacks (5 minutes)

```sh
mkdir -p ../dns/outputs
pnpm exec cdk deploy -c env=staging --all --outputs-file ../dns/outputs/staging.json
```

Expect `dimmi-staging-ops` and `dimmi-staging-mail`. The outputs file holds the mail records the DNS script writes.

## 3. HUMAN: the application's access key (2 minutes)

The application runs outside AWS, so it signs in as the `dimmi-staging-application` user. CloudFormation never sees its
key: create it by hand and put it straight into the cluster's secret.

```sh
aws iam create-access-key --user-name dimmi-staging-application
```

Store `AccessKeyId` and `SecretAccessKey` in the environment's `dimmi-secrets` as `AWS_ACCESS_KEY_ID` and
`AWS_SECRET_ACCESS_KEY`, with `AWS_REGION` set to the environment's Region. Rotate by creating a second key, rolling the
pods, then deleting the first.

## 4. HUMAN: confirm the alarm address (1 minute)

AWS mails `ops@trydimmi.com` once for the ops topic and once for the budget. Click the SNS confirmation link.

## 5. AUTOMATED: DNS and SES verification

Follow [dns.md](dns.md). SES checks the DKIM records by itself, usually within an hour; the SES console then shows both
identities `Verified`. The SNS webhook subscriptions confirm once the host answers over HTTPS.

## 6. HUMAN: billing and SES production access

- Billing console, Cost allocation tags: activate `Environment` and `Project`, so the budget counts.
- Request SES production access for the Region: [ses-production-access.md](ses-production-access.md).

## Production

The same steps with `-c env=production`, Region us-east-1.
