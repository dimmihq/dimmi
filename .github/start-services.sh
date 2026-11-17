#!/usr/bin/env bash
# Starts the services from compose.yaml in the background at the top of a CI
# job, so the job's setup runs while they come up. Pulling an image sometimes
# stalls on a shared runner, so each attempt is cut off and tried again; the
# log and the last attempt's status are left for wait-for-services.sh.
set -uo pipefail

(
  status=1
  for attempt in 1 2 3; do
    echo "docker compose up, attempt $attempt" >> "$RUNNER_TEMP/services.log"
    timeout 150 docker compose up -d --wait >> "$RUNNER_TEMP/services.log" 2>&1
    status=$?
    if [ "$status" = 0 ]; then
      break
    fi
  done
  echo "$status" > "$RUNNER_TEMP/services.status"
) &
