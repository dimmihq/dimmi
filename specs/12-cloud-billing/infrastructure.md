# 12 Dimmi Cloud: infrastructure

Style: caveman. Status: outline (PO 2026-11-02: "no need to worry too much about that now"). Internal ops only; never published as self-host guidance. Self-host paths: `specs/11-self-host/packaging.md` (compose, `install.sh` bare binary, platform templates).

## Shape

Same binary as self-host, no Docker. Managed backing services. Built to scale out a lot.

| Layer | Choice |
|---|---|
| App | release binary (`linux/arm64`, Graviton) on EC2, run by the same `dimmi.service` systemd unit `install.sh` ships. No container runtime |
| Fleet | EC2 Auto Scaling groups behind ALB (idle timeout 300 s, health `/ready`, no sticky sessions). Optional split by `DIMMI_APP__ROLES`: `web` group (scales on WebSocket count + CPU), `worker,scheduler` group (scales on River queue depth) |
| Deploy | new binary version = new launch template version (user data pulls signed release archive, verifies checksum + cosign) then ASG instance refresh, rolling; migrations run once under advisory lock (11 operations.md) |
| Postgres | RDS PostgreSQL 18 (pgvector), Multi-AZ, writer endpoint as `database.url`; reader endpoint as `database.read_urls` when replicas added; RDS Proxy optional (`database.pool_mode=transaction` + `database.direct_url`). Backups: RDS PITR (`backups.enabled=false`) |
| Valkey | ElastiCache for Valkey 8, TLS + auth, cluster mode when one shard is not enough (`redis.mode=auto`) |
| Storage | S3 (`filesystems.default=s3`) |
| Mail | SES (two configuration sets, tenant per workspace; 04) |
| Keys | KMS (`encryption.keys.driver=kms`) |
| Secrets | Secrets Manager, read at boot into `_FILE` paths or env by user data |
| IaC | CloudFormation (stacks per cell) |
| Cells | one cell = one VPC + RDS + ElastiCache + ASGs (`app.region` `us`, later `eu`); foundation extensibility.md cell model |

Backing-service scaling (RDS instance class, replicas, ElastiCache shards) is AWS's job; Dimmi only connects (11 decision 20).

## Later (not specced yet)

Stack layout, instance sizes, autoscaling thresholds, observability stack, runbooks, cost model refresh (`research/pricing.md` uses EC2 Graviton compute).
