# deploy

How dimmi's cloud edition reaches AWS. The runbook, with every step a person must take, is
[`docs/deploy`](../docs/deploy/README.md).

| Folder | Holds |
|---|---|
| [`cdk`](cdk) | TypeScript CDK for every environment, from one typed config |
| [`dns`](dns) | Writes each environment's records to Vercel DNS: `plan`, `apply`, `verify`, `remove` |
| [`release`](release) | Release, rollback and smoke test scripts the workflows run |
| [`domainconnect`](domainconnect) | The Domain Connect template customers' DNS hosts use for sending domains |

The image is built from the repository's [`Dockerfile`](../Dockerfile); the workflows are in
[`.github/workflows`](../.github/workflows).

Nothing here holds an account id, a token or a secret. `make infra-check` proves the code synthesizes and passes its tests
with every AWS credential hidden, and scans this folder for anything that looks like a secret.
