# deploy

How dimmi's cloud edition is deployed: one Hetzner server running k3s for every environment, and AWS for mail only. The
runbook, with every step a person must take, is [`docs/deploy`](../docs/deploy/README.md).

| Folder | Holds |
|---|---|
| [`server`](server) | Prepares the Hetzner server and configures its private network and k3s |
| [`k8s`](k8s) | What runs on the cluster: data services, migrations and the application, per environment |
| [`cdk`](cdk) | TypeScript CDK for each environment's mail on AWS, from one typed config |
| [`dns`](dns) | Writes each environment's records to Vercel DNS: `plan`, `apply`, `verify`, `remove` |
| [`release`](release) | The smoke test that checks a deployed environment from the outside |
| [`domainconnect`](domainconnect) | The Domain Connect template customers' DNS hosts use for sending domains |

The image is built from the repository's [`Dockerfile`](../Dockerfile).

Nothing here holds an account id, a token or a secret. `make infra-check` proves the code synthesizes and passes its tests
with every AWS credential hidden, and scans this folder for anything that looks like a secret.
