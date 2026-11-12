#!/usr/bin/env bash
# Sets up the observability namespace around infrastructure/observability.yaml.
# Run by an admin whose kubectl points at the cluster:
#
#   deploy/k8s/scripts/observability.sh secrets    # before the first apply
#   deploy/k8s/scripts/observability.sh configure  # once OpenObserve runs
#
# secrets makes the OpenObserve root login once, random, and keeps it on
# later runs. configure sets stream retention and the alerts through the
# OpenObserve API over a port-forward; running it again updates them.
# Needs kubectl, curl, jq and openssl.
# Nothing secret is printed and nothing touches the disk.

set -euo pipefail

NS=observability
ORG=default
EMAIL=admin@trydimmi.com
PORT="${PORT:-15080}"

secrets() {
  kubectl get namespace "$NS" >/dev/null 2>&1 || kubectl create namespace "$NS" >/dev/null
  kubectl label namespace "$NS" dimmi.com/protected=true --overwrite >/dev/null
  if kubectl -n "$NS" get secret openobserve-root >/dev/null 2>&1; then
    echo "openobserve-root exists; kept"
    return
  fi

  local password
  password="$(openssl rand -hex 16)Aa1-"
  kubectl -n "$NS" create secret generic openobserve-root \
    --from-literal="email=$EMAIL" \
    --from-literal="password=$password" \
    --from-literal="authorization=Basic $(printf '%s:%s' "$EMAIL" "$password" | base64 | tr -d '\n')" >/dev/null
  kubectl -n "$NS" label secret openobserve-root dimmi.com/protected=true >/dev/null
  echo "openobserve-root created"
}

# api METHOD PATH [JSON] calls OpenObserve as root through the port-forward.
api() {
  curl -fsS -X "$1" "http://127.0.0.1:$PORT/api$2" \
    -H "Authorization: $AUTH" -H 'Content-Type: application/json' ${3:+--data "$3"}
}

configure() {
  AUTH="$(kubectl -n "$NS" get secret openobserve-root -o jsonpath='{.data.authorization}' | base64 -d)"
  kubectl -n "$NS" port-forward svc/openobserve "$PORT:5080" >/dev/null &
  FORWARD=$!
  trap 'kill "$FORWARD" 2>/dev/null' EXIT
  for _ in $(seq 30); do curl -fsS "http://127.0.0.1:$PORT/healthz" >/dev/null 2>&1 && break; sleep 1; done

  retention
  destination
  alerts
}

# Until a real destination (Slack, email) is added, fired alerts are
# written back into OpenObserve as the alerts_fired log stream.
destination() {
  local body
  body="$(jq -n --arg auth "$AUTH" '{
    name: "alerts_fired", type: "http", method: "post", template: "prebuilt_webhook",
    url: "http://openobserve.observability.svc.cluster.local:5080/api/default/alerts_fired/_json",
    headers: {Authorization: $auth}, skip_tls_verify: false
  }')"
  if api GET "/$ORG/alerts/destinations/alerts_fired" >/dev/null 2>&1; then
    api PUT "/$ORG/alerts/destinations/alerts_fired" "$body" >/dev/null
  else
    api POST "/$ORG/alerts/destinations" "$body" >/dev/null
  fi
  echo "destination: alerts_fired"
}

# Metrics keep the global 30 days; logs and traces keep 14. A stream
# exists only once data reached it, so run this again if one is missing.
retention() {
  local type
  for type in logs traces; do
    if api PUT "/$ORG/streams/default/settings?type=$type" '{"data_retention":14}' >/dev/null 2>&1; then
      echo "retention: $type 14 days"
    else
      echo "retention: no $type stream yet; run configure again later" >&2
    fi
  done
}

# alert NAME STREAM TYPE QUERY DESCRIPTION creates or updates a scheduled
# alert that looks at the last 10 minutes every 5. QUERY is jq that builds
# the query_condition. An alert that already exists keeps the destinations
# set on it in the UI.
alert() {
  local body id destinations
  id="$(api GET "/v2/$ORG/alerts" | jq -r --arg name "$1" '.list[]? | select(.name == $name) | .alert_id' | head -n1)"
  destinations='["alerts_fired"]'
  [[ -z "$id" ]] || destinations="$(api GET "/v2/$ORG/alerts/$id" | jq -c '.destinations // ["alerts_fired"]')"
  body="$(jq -n --argjson destinations "$destinations" --arg name "$1" --arg stream "$2" --arg type "$3" --arg description "$5" "{
      name: \$name, stream_name: \$stream, stream_type: \$type, description: \$description,
      is_real_time: false, enabled: true, destinations: $destinations,
      query_condition: ($4),
      trigger_condition: {period: 10, frequency: 5, frequency_type: \"minutes\",
        operator: \">=\", threshold: 1, silence: 30}
    }")"
  if [[ -n "$id" ]]; then
    api PUT "/v2/$ORG/alerts/$id" "$body" >/dev/null
  else
    api POST "/v2/$ORG/alerts" "$body" >/dev/null
  fi
  echo "alert: $1"
}

promql() {
  printf '{type: "promql", promql: %s, promql_condition: {column: "value", operator: ">=", value: %s}}' \
    "$(jq -Rn --arg q "$1" '$q')" "$2"
}

alerts() {
  alert web_5xx http_server_request_duration_seconds_count metrics \
    "$(promql 'sum by (k8s_namespace_name) (increase(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[10m]))' 5)" \
    "Five or more 5xx responses from web in 10 minutes"
  alert pod_restarts k8s_container_restarts metrics \
    "$(promql 'sum by (k8s_namespace_name, k8s_pod_name) (increase(k8s_container_restarts[10m]))' 1)" \
    "A pod's containers restarted in the last 10 minutes"
  alert error_logs default logs \
    "$(jq -Rn --arg q "SELECT k8s_namespace_name, COUNT(*) AS errors FROM \"default\" WHERE level = 'ERROR' AND k8s_namespace_name LIKE 'dimmi-%' GROUP BY k8s_namespace_name HAVING COUNT(*) >= 20" '{type: "sql", sql: $q}')" \
    "Twenty or more error log lines in a dimmi namespace in 10 minutes"
}

case "${1:-}" in
  secrets) secrets ;;
  configure) configure ;;
  *) echo "usage: $0 secrets|configure" >&2; exit 2 ;;
esac
