#!/usr/bin/env bash
# Makes or updates an environment's secrets on the server. Run by an admin
# whose kubectl points at the cluster:
#
#   deploy/k8s/scripts/secrets.sh staging < values.env
#
# Passwords for the database, Valkey and SeaweedFS and the app key are
# random and made once; running again keeps them. Lines on stdin, in
# KEY=value form, are merged into dimmi-secrets over what it holds, so
# provider keys and AWS settings can be added or rotated at any time.
# Nothing is printed and nothing touches the disk.

set -euo pipefail

ENVIRONMENT="${1:?usage: $0 <environment> < values.env}"
NS="dimmi-$ENVIRONMENT"

random() { openssl rand -hex 24; }
has() { kubectl -n "$NS" get secret "$1" >/dev/null 2>&1; }
protect() { kubectl -n "$NS" label secret "$@" dimmi.com/protected=true --overwrite >/dev/null; }

generated() {
  local owner app valkey s3_key s3_secret
  owner="$(random)"
  app="$(random)"
  valkey="$(random)"
  s3_key="$(random)"
  s3_secret="$(random)"

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
  protect postgres-owner postgres-app-role valkey seaweedfs-s3

  cat <<VALUES
DIMMI_APP__KEY=base64:$(openssl rand -base64 32)
DIMMI_DATABASE__URL=postgres://dimmi_app:$app@postgres-rw:5432/dimmi?sslmode=require
OWNER_DATABASE_URL=postgres://dimmi_owner:$owner@postgres-rw:5432/dimmi?sslmode=require
DIMMI_REDIS__URL=redis://:$valkey@valkey:6379/0
DIMMI_FILESYSTEMS__DISKS__S3__ACCESS_KEY_ID=$s3_key
DIMMI_FILESYSTEMS__DISKS__S3__SECRET_ACCESS_KEY=$s3_secret
VALUES
}

merge() {
  local current
  current="$(kubectl -n "$NS" get secret dimmi-secrets -o json 2>/dev/null | jq '.data // {}' || echo '{}')"
  jq -Rn --argjson current "$current" '
    [inputs | select(test("^[A-Z][A-Z0-9_]*=")) | capture("^(?<k>[^=]+)=(?<v>.*)$")]
    | reduce .[] as $e ($current; .[$e.k] = ($e.v | @base64))
    | {apiVersion: "v1", kind: "Secret", type: "Opaque",
       metadata: {name: "dimmi-secrets", labels: {"dimmi.com/protected": "true"}}, data: .}
  ' | kubectl -n "$NS" apply -f - >/dev/null
}

{
  if ! has postgres-owner; then generated; fi
  cat
} | merge

echo "secrets in $NS are up to date"
