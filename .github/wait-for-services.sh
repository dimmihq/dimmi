#!/usr/bin/env bash
# Waits for the services started in the background at the top of a CI job:
# first for docker compose to finish starting the containers, then for each
# service to answer, since a running container may not be listening yet.
set -euo pipefail

timeout 300 bash -c 'until [ -f "$RUNNER_TEMP/services.status" ]; do sleep 1; done'
cat "$RUNNER_TEMP/services.log"
status="$(cat "$RUNNER_TEMP/services.status")"
if [ "$status" != 0 ]; then
  exit "$status"
fi

timeout 120 bash -c '
  until docker compose exec -T postgres pg_isready -q -h 127.0.0.1 -U postgres; do sleep 0.5; done
  until docker compose exec -T valkey valkey-cli ping | grep -q PONG; do sleep 0.5; done
  until curl -s -o /dev/null http://127.0.0.1:18333/; do sleep 0.5; done
  until curl -s -o /dev/null http://127.0.0.1:18025/; do sleep 0.5; done
'
