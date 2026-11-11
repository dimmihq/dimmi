# dimmi infrastructure

AWS CDK in TypeScript for each environment's mail: SES, the SNS topics that carry mail events to the application, the
inbound bucket and their alarms. Everything else runs on the k3s server. One app holds all of them; `-c env=<name>` picks one. The runbook for people
is [`docs/deploy`](../../docs/deploy/README.md); this page is for changing the code.

The package has its own lockfile and is not part of the browser app. Node runs the TypeScript directly, so it needs Node
22.18 or newer. Run commands from `deploy/cdk` after `pnpm install`.

## Layout

| Path                         | Holds                                                                            |
| ---------------------------- | -------------------------------------------------------------------------------- |
| `bin/dimmi.ts`               | Entry point: reads `-c env=` and builds that environment                         |
| `lib/config/environments.ts` | **Every difference between environments**: Regions, hosts, mail domains, budgets |
| `lib/config/environment.ts`  | The config type and its validation                                               |
| `lib/environment-stacks.ts`  | Wires one environment's stacks together, names and tags them                     |
| `lib/stacks/`                | One file per stack                                                               |
| `lib/constructs/`            | Pieces a stack is built from: activating the receipt rule set                    |
| `test/`                      | Template assertions, run with Vitest, no AWS credentials needed                  |

## Stacks

Each environment has these, named `dimmi-<env>-<role>` and tagged `Environment=<env>`:

| Stack  | Owns                                                                                             |
| ------ | ------------------------------------------------------------------------------------------------ |
| `ops`  | Alarm topic mailed to the alarm address, monthly budget on the environment tag                   |
| `mail` | SES identities, receipt rules, configuration sets, webhooks, mail alarms, the application's user |

The application signs in as the IAM user `dimmi-<env>-application`. Its access key is created by hand and stored in the
cluster; no key ever appears in a template or an output.

## Adding an environment

Add one entry to `lib/config/environments.ts`. Give it its own Region if another environment shares the account: SES
keeps one active receipt rule set, one sandbox and one reputation per Region. Then follow the runbook from the top.

## Commands

```sh
pnpm run typecheck
pnpm run test
pnpm run synth -c env=staging            # one environment
pnpm run synth:all                       # every environment, as CI does
pnpm exec cdk diff --all -c env=staging  # needs credentials
```

`make infra-check` from the repository root runs typecheck, synth of both environments and the tests with every AWS
credential hidden, then scans `deploy/` for secrets.

Synth never looks anything up and `cdk.context.json` is ignored by git, because lookups cache account ids and the
repository is public. The account comes from the deploying credentials.
