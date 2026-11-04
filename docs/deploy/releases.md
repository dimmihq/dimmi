# Releases, rollback, pausing and teardown

## Staging: every push to main (AUTOMATED, 25 to 40 minutes)

The `main` workflow runs `make lint test infra-check` and `make e2e`, builds the ARM image once and pushes it to
`dimmi-staging` tagged with the commit sha, then runs [`deploy/release/release.sh`](../../deploy/release/release.sh):

1. tag the running image `previous`, and the new one `release`;
2. run `provision` (idempotent role setup) and `migrate --force` as the schema owner;
3. force a new deployment of every service and wait until each is stable;
4. smoke test: verified TLS, `/up` and `/ready`, HTTP redirecting to HTTPS, the gate closed without the CI key and open
   with it.

Releases to one environment never overlap. Migrations only ever add, so the old tasks keep working while new ones start.
Services start new tasks before stopping old ones (100% minimum, 200% maximum), so nothing drops.

Check: the workflow ends with `smoke: https://staging.trydimmi.com is healthy`.

If it fails:

| Where | What happened | What to do |
|---|---|---|
| `check` or `e2e` | Code is broken | Nothing was released. Fix on `main` |
| `migrate` | A migration failed | `release` points back at the old image, services were never touched. Read the `migrate` log stream in `/dimmi/staging`, fix the migration |
| Rolling | New tasks failed health checks | The circuit breaker already put the old tasks back, and the script points `release` back too. Read the task log |
| Smoke test | The release is up but wrong | Roll back (below), then investigate |

## Production (HUMAN starts and approves, 15 minutes)

1. Pick a commit staging has run and smoke tested.
2. Actions, `production`, Run workflow, paste the full commit sha.
3. A required reviewer approves the `production` environment.
4. The workflow copies that exact image from staging's registry to production's, releases it and smoke tests
   `https://trydimmi.com`.

Failures behave as for staging. Nothing is rebuilt: production runs bytes staging ran.

## Rollback

**HUMAN decides, AUTOMATED does, 5 to 10 minutes.** Actions, `rollback`, choose the environment, and leave the tag empty
to return to the image before the last release, or give a commit sha. From a terminal with the deploy role:

```sh
ENVIRONMENT=production AWS_REGION=us-east-1 deploy/release/rollback.sh            # previous
ENVIRONMENT=production AWS_REGION=us-east-1 deploy/release/rollback.sh <sha>      # a given image
```

Rollback never migrates. The schema stays as the newer release left it, which the older code reads because migrations
only add. A migration marked as contract ships one release after the code stopped using what it removes, so rolling back
one release is always safe; rolling back further than a contract migration is not, and needs a person to judge.

The registry keeps the last 30 images.

## Infrastructure changes (HUMAN, minutes to an hour)

CI never changes infrastructure. After a change to `deploy/cdk` is merged:

```sh
cd deploy/cdk
pnpm exec cdk diff --all -c env=staging     # read it: replacements of data resources are never fine
pnpm exec cdk deploy --all -c env=staging --outputs-file ../dns/outputs/staging.json
```

Then `pnpm dns plan` in `deploy/dns` in case addresses or mail records changed. Repeat for production once staging is
right. A secret value set by a person is never overwritten by a deploy unless its key list in `data-stack.ts` changes.

## Pausing staging

**HUMAN, 5 minutes.** Stop the compute but keep everything else:

```sh
pnpm exec cdk deploy dimmi-staging-app -c env=staging -c paused=true
```

Services go to zero tasks, which saves about $8.50 a month; the database, Valkey, load balancer and addresses keep
costing. RDS can also be stopped from its console for up to seven days, after which AWS starts it again. SES keeps
receiving into S3 while paused, and SNS retries webhooks for an hour and then parks them in the dead-letter queue, from
which raw mail can be replayed. Resume by deploying without `paused`.

## Teardown

**HUMAN, 30 minutes.** Staging deletes everything, data included:

```sh
pnpm exec cdk destroy --all -c env=staging
```

Then remove the environment's DNS records, which deletes only records carrying staging's comment, and delete the GitHub
`staging` environment:

```sh
cd ../dns && pnpm dns remove --env staging
```

Production refuses: its data stack has termination protection, RDS has deletion protection, and the bucket, key, logs
and registry are retained. Tearing production down takes a person turning each of those off on purpose, and RDS leaves a
final snapshot.

`dimmi-account` stays while any environment exists.
