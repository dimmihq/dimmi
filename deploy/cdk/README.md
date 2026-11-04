# dimmi infrastructure

AWS CDK in TypeScript. Today it holds one stack, `Mail`: receiving, sending, notifications, alarms and DNS for one stage.
It deploys alone and does not depend on what runs the application.

The package has its own lockfile and is not part of the browser app. Node runs the TypeScript directly, so it needs
Node 22.18 or newer. Run every command below from `deploy/cdk`, after `pnpm install`.

## Settings

Nothing that identifies an account is in this repository. Each setting comes from `--context name=value`, or from the
environment variable beside it.

| Context | Environment | Meaning |
|---|---|---|
| `stage` | `DIMMI_STAGE` | Names and tags every resource, such as `staging` |
| `region` | `DIMMI_REGION` | Where the stack lives. Stages in one account must use different Regions (below) |
| `inboundDomain` | `DIMMI_INBOUND_DOMAIN` | The domain SES receives on and sends default mail from |
| `webhookBaseUrl` | `DIMMI_WEBHOOK_BASE_URL` | Origin of the web host; `/webhooks/ses/inbound` and `/webhooks/ses/events` are added |
| `opsEmail` | `DIMMI_OPS_EMAIL` | Who is mailed when an alarm fires |
| `hostedZoneId` | `DIMMI_HOSTED_ZONE_ID` | Optional Route 53 zone for the DNS records. Without it they become stack outputs named `DnsRecord*` |
| `hostedZoneName` | `DIMMI_HOSTED_ZONE_NAME` | The zone's name, required with the id. The stack does no lookup |

The account comes from your AWS credentials when you deploy. Keep your values in a local `.env*` file or your shell; both
`.env*` and `cdk.context.json` are ignored by git.

**One Region per stage.** SES has one active receipt rule set per account and Region. Two stages in one account and one
Region would take each other's inbound mail, and deleting one stack would stop receiving for both.

## Synth

`make infra-check` from the repository root type-checks, synthesizes on fixture values, runs the template tests and scans
`deploy/` for secrets. It hides AWS credentials from the toolchain, so it cannot reach an account.

For your own values:

```sh
export DIMMI_STAGE=staging DIMMI_REGION=<region> DIMMI_INBOUND_DOMAIN=inbound.staging.trydimmi.com
export DIMMI_WEBHOOK_BASE_URL=https://staging.trydimmi.com DIMMI_OPS_EMAIL=<address>
export DIMMI_HOSTED_ZONE_ID=<zone id> DIMMI_HOSTED_ZONE_NAME=staging.trydimmi.com
pnpm exec cdk synth --lookups=false
```

## Bootstrap

Once per account and Region, with credentials for that account:

```sh
pnpm exec cdk bootstrap aws://<account>/<region> --cloudformation-execution-policies <policy arn>
```

The default execution role is effectively administrator. In an account shared between stages, pass a narrower policy.

## Diff

```sh
pnpm exec cdk diff Mail
```

## Deploy

```sh
pnpm exec cdk deploy Mail
```

Afterwards:

- Confirm the subscription mail sent to the ops address. Alarms reach nobody until then.
- Without a zone id, add the `DnsRecord*` outputs at your DNS host, with no proxy.
- Attach the policy in the `MailApplicationPolicyArn` output to whatever role runs the application.
- Point the application at the other outputs: bucket, topics and configuration set names.

The application must answer on `webhookBaseUrl` for the HTTPS subscriptions to confirm. Until it does they stay pending,
and SNS delivers nothing.

Operating the stack is covered in [`docs/ops/ses-inbound.md`](../../docs/ops/ses-inbound.md).
