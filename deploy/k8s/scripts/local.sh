#!/usr/bin/env bash
# Runs the cluster on this machine in k3d, the way the server runs it:
# operators first, then data, then migrations, then the app.
#
#   deploy/k8s/scripts/local.sh up      create or update everything
#   deploy/k8s/scripts/local.sh down    delete the cluster and its data
#
# Secrets are random and made once per cluster; they never leave it.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CLUSTER=dimmi
NS=dimmi-local
IMAGE=dimmi:local

random() { openssl rand -hex 24; }

cluster() {
  if ! k3d cluster list -o json | grep -q "\"name\":\"$CLUSTER\""; then
    k3d cluster create --config "$ROOT/k3d.yaml" --wait
  fi
  kubectl config use-context "k3d-$CLUSTER" >/dev/null
}

image() {
  if [[ "${BUILD:-1}" == 1 ]]; then
    docker build -t "$IMAGE" "$ROOT/../.."
  fi
  k3d image import "$IMAGE" -c "$CLUSTER"
}

operators() {
  flux install --components=source-controller,helm-controller --network-policy=false
  kubectl apply -k "$ROOT/infrastructure"
  kubectl -n cnpg-system wait helmrelease/cloudnative-pg --for=condition=Ready --timeout=5m
  kubectl wait crd/clusters.postgresql.cnpg.io --for=condition=Established --timeout=1m
}

secrets() {
  kubectl apply -f "$ROOT/apps/local/data/namespace.yaml"
  if kubectl -n "$NS" get secret dimmi-secrets >/dev/null 2>&1; then
    return
  fi

  local owner app valkey s3_key s3_secret app_key
  owner="$(random)"
  app="$(random)"
  valkey="$(random)"
  s3_key="$(random)"
  s3_secret="$(random)"
  app_key="base64:$(openssl rand -base64 32)"

  kubectl -n "$NS" create secret generic postgres-owner --type=kubernetes.io/basic-auth \
    --from-literal=username=dimmi_owner --from-literal="password=${owner}"
  kubectl -n "$NS" create secret generic postgres-app-role --type=kubernetes.io/basic-auth \
    --from-literal=username=dimmi_app --from-literal="password=${app}"
  kubectl -n "$NS" label secret postgres-owner postgres-app-role cnpg.io/reload=true
  kubectl -n "$NS" create secret generic valkey --from-literal="password=${valkey}"
  kubectl -n "$NS" create secret generic seaweedfs-s3 --from-literal=s3.json="$(cat <<JSON
{"identities":[{"name":"dimmi","credentials":[{"accessKey":"$s3_key","secretKey":"$s3_secret"}],"actions":["Admin","Read","List","Tagging","Write"]}]}
JSON
)"
  kubectl -n "$NS" create secret generic dimmi-secrets \
    --from-literal=DIMMI_APP__KEY="$app_key" \
    --from-literal=DIMMI_DATABASE__URL="postgres://dimmi_app:$app@postgres-rw:5432/dimmi?sslmode=require" \
    --from-literal=OWNER_DATABASE_URL="postgres://dimmi_owner:$owner@postgres-rw:5432/dimmi?sslmode=require" \
    --from-literal=DIMMI_REDIS__URL="redis://:$valkey@valkey:6379/0" \
    --from-literal=DIMMI_FILESYSTEMS__DISKS__S3__ACCESS_KEY_ID="$s3_key" \
    --from-literal=DIMMI_FILESYSTEMS__DISKS__S3__SECRET_ACCESS_KEY="$s3_secret"
  kubectl -n "$NS" label secret postgres-owner postgres-app-role valkey seaweedfs-s3 dimmi-secrets dimmi.com/protected=true
}

data() {
  kubectl -n "$NS" delete job seaweedfs-buckets --ignore-not-found
  kubectl apply -k "$ROOT/apps/local/data"
  kubectl -n "$NS" wait cluster/postgres --for=condition=Ready --timeout=5m
  kubectl -n "$NS" rollout status statefulset/valkey --timeout=3m
  kubectl -n "$NS" rollout status statefulset/seaweedfs --timeout=3m
  kubectl -n "$NS" wait job/seaweedfs-buckets --for=condition=Complete --timeout=3m
}

migrate() {
  kubectl -n "$NS" delete job migrate --ignore-not-found
  kubectl apply -k "$ROOT/apps/local/migrate"
  if ! kubectl -n "$NS" wait job/migrate --for=condition=Complete --timeout=5m; then
    kubectl -n "$NS" logs job/migrate --tail=50
    exit 1
  fi
}

app() {
  kubectl apply -k "$ROOT/apps/local/app"
  kubectl -n "$NS" rollout restart deployment/web deployment/worker
  kubectl -n "$NS" rollout status deployment/web --timeout=3m
  kubectl -n "$NS" rollout status deployment/worker --timeout=3m
  curl -fsS http://localhost:18080/ready && echo
  echo "dimmi is up at http://localhost:18080"
}

case "${1:-}" in
  up) cluster; image; operators; secrets; data; migrate; app ;;
  down) k3d cluster delete "$CLUSTER" ;;
  *) echo "usage: $0 up|down" >&2; exit 2 ;;
esac
