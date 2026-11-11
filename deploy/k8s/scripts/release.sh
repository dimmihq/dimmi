#!/usr/bin/env bash
# Releases one commit's image to an environment on the server, in the order
# local.sh uses: data first, then migrations, then the app, then the edge.
#
#   deploy/k8s/scripts/release.sh staging <commit sha>
#
# kubectl must already point at the cluster. CI runs this as the
# environment's deployer account; the namespace, that account and the
# secrets are made once by an admin (see apps/<env>/access).

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENVIRONMENT="${1:?usage: $0 <environment> <commit sha>}"
SHA="${2:?usage: $0 <environment> <commit sha>}"
NS="dimmi-$ENVIRONMENT"
IMAGE=ghcr.io/dimmihq/dimmi

[[ "$SHA" =~ ^[0-9a-f]{40}$ ]] || { echo "the commit must be a full sha" >&2; exit 2; }
[[ -d "$ROOT/apps/$ENVIRONMENT" ]] || { echo "no environment named $ENVIRONMENT" >&2; exit 2; }

render() {
  kubectl kustomize "$ROOT/apps/$ENVIRONMENT/$1" | sed "s#image: $IMAGE\$#image: $IMAGE:$SHA#"
}

data() {
  kubectl -n "$NS" delete job seaweedfs-buckets --ignore-not-found
  render data | kubectl apply -f -
  kubectl -n "$NS" wait cluster/postgres --for=condition=Ready --timeout=5m
  kubectl -n "$NS" rollout status statefulset/valkey --timeout=3m
  kubectl -n "$NS" rollout status statefulset/seaweedfs --timeout=3m
  kubectl -n "$NS" wait job/seaweedfs-buckets --for=condition=Complete --timeout=3m
}

migrate() {
  kubectl -n "$NS" delete job migrate --ignore-not-found --wait
  render migrate | kubectl apply -f -
  if ! kubectl -n "$NS" wait job/migrate --for=condition=Complete --timeout=10m; then
    kubectl -n "$NS" logs job/migrate --tail=100
    exit 1
  fi
}

app() {
  render app | kubectl apply -f -
  if ! kubectl -n "$NS" rollout status deployment/web --timeout=5m ||
    ! kubectl -n "$NS" rollout status deployment/worker --timeout=5m; then
    kubectl -n "$NS" get pods
    kubectl -n "$NS" logs deployment/web --tail=100 || true
    exit 1
  fi
}

edge() {
  render edge | kubectl apply -f -
  kubectl -n "$NS" rollout status deployment/edge --timeout=3m
}

data
migrate
app
edge
echo "released $SHA to $ENVIRONMENT"
