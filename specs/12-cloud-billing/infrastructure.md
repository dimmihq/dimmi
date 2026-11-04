# 12 Dimmi Cloud: infrastructure

Style: caveman. Status: approved for launch (owner, 2026-11-04). Internal ops only; never published as self-host guidance. Self-host paths: `specs/11-self-host/packaging.md`. Built and run: `deploy/` and `docs/deploy`; detail in `specs/19-cloud-staging`.

## Shape

Same binary as self-host, cloud edition (`-tags cloud`), in a distroless ARM container. Managed backing services. Staging and production defined in one CDK app.

| Layer | Choice |
|---|---|
| App | ECS Fargate, `linux/arm64`, image from `Dockerfile`. Tasks disposable: fresh container per deploy, nothing on disk |
| Edge | NLB, TCP passthrough on 443 and 80, Elastic IP per zone, client IP preservation on IP targets. Binary terminates TLS (certmagic, certificates in Postgres). Health `/ready` over plain HTTP. No ALB, no sticky sessions |
| Services | `web` behind the NLB; production adds `worker,scheduler` as its own service (`DIMMI_APP__ROLES`). Staging runs all roles in one task |
| Deploy | image per commit in ECR; `release` tag moved, `provision` and `migrate --force` one-off tasks, then forced rolling deployment (100/200, circuit breaker rollback). Production gets the exact image staging ran. GitHub Actions with OIDC, no long-lived keys |
| Postgres | RDS PostgreSQL 18, Multi-AZ in production. App connects as `dimmi_app` (owns nothing, no `BYPASSRLS`); migrations as `dimmi_owner`. Backups: RDS automated (`backups.enabled=false`) |
| Valkey | ElastiCache for Valkey 8, node based, TLS + AUTH. Serverless out: no `CLIENT TRACKING` |
| Storage | S3 (`filesystems.default=s3`); inbound mail bucket owned by the mail stack |
| Mail | SES, Region per environment (staging us-east-2, production us-east-1), two configuration sets, tenant per workspace (04) |
| Keys | KMS (`encryption.kek=awskms`) |
| Secrets | Secrets Manager, injected by ECS at task start |
| DNS | Vercel, every record, written by `deploy/dns`. No Route 53 |
| IaC | AWS CDK in TypeScript, `deploy/cdk`, one typed config per environment |

Backing-service scaling (RDS instance class, replicas, ElastiCache shards) is AWS's job; Dimmi only connects (11 decision 20).

## Later (not specced yet)

Autoscaling thresholds (web on CPU and connections, worker on queue depth), cells per region (`app.region` `us`, later `eu`), observability stack beyond CloudWatch, cost model refresh in `research/pricing.md`.
