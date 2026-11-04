# Monthly costs

On-demand prices in USD, 730 hours a month, read on 2026-11-04. us-east-1 prices are used for both Regions; us-east-2 is
usually the same. `~` marks a figure from a third party or from memory rather than AWS's own page. Traffic and mail
volume add to these; at staging's volume they are cents.

## Staging (us-east-2)

| Item | Size | Monthly |
|---|---|---|
| Fargate, ARM | 1 task, 0.25 vCPU, 1 GB | $8.51 |
| RDS PostgreSQL | db.t4g.micro, 20 GB gp3, 1-day backups | ~$14 |
| ElastiCache Valkey | cache.t4g.micro, 1 node | ~$9.34 |
| Network Load Balancer | 1, idle | $16.43 plus about $0.50 of NLCU |
| Public IPv4 | 2 Elastic IPs and 1 task address | $10.95 |
| Secrets Manager | 8 secrets | $3.20 |
| KMS | 1 key | $1 |
| CloudWatch | logs, 9 alarms | ~$1 |
| S3, ECR, SES, SNS, SQS | | ~$2 |
| **Total** | | **~$67** |

Pausing staging saves only the Fargate line. The load balancer and its addresses are the largest part: they buy the same
edge as production. Running one task with a public address and DNS straight at it would save about $24 a month, at the
price of staging no longer matching production.

## Production (us-east-1)

| Item | Size | Monthly |
|---|---|---|
| Fargate web, ARM | 2 tasks, 0.5 vCPU, 1 GB | $28.84 |
| Fargate worker, ARM | 2 tasks, 0.5 vCPU, 2 GB | $34.04 |
| RDS PostgreSQL | db.t4g.medium Multi-AZ, 50 GB gp3, 14-day backups | ~$106 |
| ElastiCache Valkey | cache.t4g.small, 2 nodes | ~$46.72 |
| Network Load Balancer | 1 | $16.43 plus NLCU by traffic |
| Public IPv4 | 2 Elastic IPs and 4 task addresses | $21.90 |
| Secrets Manager, KMS | 8 secrets, 1 key | $4.20 |
| CloudWatch | logs (3 months), Container Insights, alarms | ~$10 |
| S3, ECR, SES, SNS, SQS | | by volume, ~$5 at launch |
| **Total** | | **~$273 plus traffic and mail** |

A NAT gateway instead of task addresses would add about $33 a month per zone; set `network.natGateways` when the
security review asks for private tasks.

## Not confirmed

- RDS and ElastiCache instance prices: AWS's own price tables did not render; third-party listings were used.
- gp3 storage at $0.115 per GB-month, and Multi-AZ doubling it: from memory.
- Container Insights enhanced observability: depends on metric count; estimated.
- us-east-2 prices are assumed equal to us-east-1.
- Credits a new account may receive are not counted.

The budget in each environment's `ops` stack mails `ops@trydimmi.com` at 80% of actual and 100% of forecast spend:
$100 for staging, $600 for production. It counts only once the `Environment` cost allocation tag is active.
