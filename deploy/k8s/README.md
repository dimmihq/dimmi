# k8s

How dimmi's cloud edition runs on a single k3s server. Each environment is one namespace holding its own
PostgreSQL, Valkey and SeaweedFS, so staging and production share the machine and nothing else. Mail stays
on Amazon SES; everything else lives on the server.

Releases reach the cluster through Flux. The repository is the source of truth, and nothing outside the
cluster holds credentials to it.

## Layout

| Path | Holds |
|---|---|
| `infrastructure/` | Cluster-wide operators, installed once: CloudNativePG |
| `apps/base/data/` | Settings, PostgreSQL, Valkey, SeaweedFS and the job that creates the bucket |
| `apps/base/migrate/` | The `migrate` job, run as the schema owner before every release |
| `apps/base/app/` | The `web` deployment and service, and the `worker` deployment |
| `apps/<env>/{data,migrate,app}/` | One environment's overlays: namespace, image, sizes, settings |
| `k3d.yaml` | The local cluster that stands in for the server |
| `scripts/local.sh` | Brings the whole stack up locally, in the order a release runs |

A release applies the three layers in order: data, then migrate, then app. Migrations only ever add, so
the running pods keep working while the job runs, and the new pods start only after the schema is ready.

## Database roles

The database belongs to `dimmi_owner`, and `migrate` connects as that role. The application connects as
`dimmi_app`, which owns nothing and cannot bypass row-level security, so every tenant policy applies to
it. Default privileges give `dimmi_app` read and write on every table the owner creates. CloudNativePG
sets both roles' passwords from the `postgres-owner` and `postgres-app-role` secrets.

## Secrets

Each environment has these secrets:

| Secret | Holds |
|---|---|
| `dimmi-secrets` | `DIMMI_` variables: app key, database and Valkey URLs, S3 keys. `OWNER_DATABASE_URL` is used by migrate only |
| `postgres-owner`, `postgres-app-role` | Basic-auth secrets CloudNativePG uses for the two roles |
| `valkey` | The Valkey password |
| `seaweedfs-s3` | `s3.json`, the only identity SeaweedFS accepts |

The key encryption key is the local driver, derived from `DIMMI_APP__KEY`. **Losing the app key loses
every encrypted value**, so keep a copy off the server.

## Nothing that holds data is deleted by mistake

Three layers stand between a mistake and lost data:

1. **The API server refuses the delete.** The `protect-data` admission policy rejects deleting a namespace,
   a database cluster, a volume claim or a secret labelled `dimmi.com/protected=true`, and any volume of
   the `data-retain` class. It applies to Flux and kubectl alike. Deleting one on purpose takes two steps:
   `kubectl annotate <kind> <name> dimmi.com/delete=allow`, then the delete.
2. **Flux never prunes data.** The namespace, the database cluster and the stateful sets carry
   `kustomize.toolkit.fluxcd.io/prune: disabled`, so removing or renaming them in git leaves them running.
3. **Deleted claims keep their files.** Every data volume uses the `data-retain` class. A deleted claim only
   releases its volume; the directory stays on disk until a person removes it.

Stateless objects (deployments, services, jobs, settings) are not protected: git recreates them exactly.

None of this survives the disk or the server dying. Only off-site backups do: PostgreSQL's continuous WAL
archive and base backups, and a copy of the files bucket, both on another machine, plus the app key kept
outside the cluster. A backup counts only once a restore from it has been tested.

## Run it locally

You need Docker, k3d, kubectl, flux and openssl.

```sh
deploy/k8s/scripts/local.sh up      # build the image, create the cluster, apply everything
BUILD=0 deploy/k8s/scripts/local.sh up   # reuse the image already built
deploy/k8s/scripts/local.sh down    # delete the cluster and all its data
```

The app answers on http://localhost:18080. The local overlay runs with `DIMMI_APP__ENV=local` and the
log mailer, because the cloud binary refuses to boot in production without a public https URL and a real
mailer. Everything else matches the server.
