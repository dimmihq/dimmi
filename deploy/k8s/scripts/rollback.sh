#!/usr/bin/env bash
# Puts an environment's web and worker back on an earlier commit's image
# after a release failed, and its site too when that commit has a site image:
#
#   deploy/k8s/scripts/rollback.sh staging <commit sha>
#
# Migrations are not undone. A migration either only expands the schema or
# is a contract migration shipped a release after the code stopped using
# what it removes, so the previous release runs against the new schema.

set -euo pipefail

ENVIRONMENT="${1:?usage: $0 <environment> <commit sha>}"
SHA="${2:?usage: $0 <environment> <commit sha>}"
NS="dimmi-$ENVIRONMENT"
IMAGE=ghcr.io/dimmihq/dimmi

[[ "$SHA" =~ ^[0-9a-f]{40}$ ]] || { echo "the commit must be a full sha" >&2; exit 2; }

for deployment in web worker; do
  kubectl -n "$NS" set image "deployment/$deployment" "dimmi=$IMAGE:$SHA"
done
for deployment in web worker; do
  kubectl -n "$NS" rollout status "deployment/$deployment" --timeout=5m
done
if kubectl -n "$NS" get deployment/site >/dev/null 2>&1 &&
  docker buildx imagetools inspect "$IMAGE:site-$SHA" >/dev/null 2>&1; then
  kubectl -n "$NS" set image deployment/site "site=$IMAGE:site-$SHA"
  kubectl -n "$NS" rollout status deployment/site --timeout=3m
fi
echo "rolled $ENVIRONMENT back to $SHA"
