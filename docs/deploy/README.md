# Deploying dimmi

dimmi's cloud edition runs on one Hetzner dedicated server with k3s. Staging and production are namespaces on that
server, each with its own PostgreSQL, Valkey and SeaweedFS ([`deploy/k8s`](../../deploy/k8s)). A Hetzner Cloud load
balancer sits in front and reaches the server over a private network ([`deploy/server`](../../deploy/server)).

AWS holds only mail: SES sending and receiving, the SNS topics that carry mail events to the application, the inbound
mail bucket and the alarms on them, defined in [`deploy/cdk`](../../deploy/cdk). DNS for every environment lives at
Vercel and is written by [`deploy/dns`](../../deploy/dns).

| Environment | Mail Region | Host |
|---|---|---|
| staging | us-east-2 | `staging.trydimmi.com` |
| production | us-east-1 | `trydimmi.com` |

## What only a person can do

**Once**

1. Order the server and set it up: [server.md](server.md).
2. Secure the AWS account: root user with MFA and no access keys.
3. Bootstrap CDK in each mail Region (`us-east-1`, `us-east-2`).
4. Create a Vercel token for the DNS script ([dns.md](dns.md#the-token)).

**Once per environment**

5. Deploy its mail stacks and create the application user's access key ([new-environment.md](new-environment.md)).
6. Confirm the alarm subscription mail sent to `ops@trydimmi.com`.
7. Run `pnpm dns apply` with the Vercel token.
8. Request SES production access for the environment's Region ([ses-production-access.md](ses-production-access.md)).
9. Create the GitHub and Google OAuth apps, with callbacks on the environment's host.

**Any change to AWS:** `cdk diff`, read it, then `cdk deploy`. CI never changes infrastructure.

## Pages

- [server.md](server.md): the server, the private network, the load balancer and k3s.
- [new-environment.md](new-environment.md): an environment's mail on AWS, from nothing.
- [dns.md](dns.md): the Vercel token, `plan`, `apply`, `verify`.
- [ses-production-access.md](ses-production-access.md): leaving the SES sandbox, with draft wording.
- [costs.md](costs.md): what each part costs a month.
