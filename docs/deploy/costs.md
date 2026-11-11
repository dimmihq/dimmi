# Monthly costs

Prices read on 2026-11-11, before VAT.

## Hetzner

| Item | Size | Monthly |
|---|---|---|
| Dedicated server | Xeon E-2276G, 64 GB ECC, 2 × 960 GB NVMe (server auction) | €120 |
| Primary IPv4 | 1 | ~€1.70 |
| Load balancer | lb11, fsn1, VAT included | €7.89 |
| vSwitch and Cloud network | | €0 |
| **Total** | both environments | **~€130** |

Traffic is unlimited on the server's 1 Gbit port; the load balancer includes 20 TB.

## AWS, per environment

Only mail runs on AWS: SES, SNS, SQS, the inbound bucket and a handful of CloudWatch alarms. At today's volume this is
cents to a few dollars a month. The budget in each environment's `ops` stack mails `ops@trydimmi.com` at 80% of actual
and 100% of forecast spend: $20 for staging, $50 for production. It counts only once the `Environment` cost allocation
tag is active.
