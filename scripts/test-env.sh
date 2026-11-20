#!/bin/sh
# Prints dimmi.test.yaml as shell exports, for commands that are not Go
# tests: eval "$(scripts/test-env.sh)". Go tests read the file through
# framework/testkit. A variable already set in the environment is kept.
file="${TEST_CONFIG:-$(dirname "$0")/../dimmi.test.yaml}"

awk '/^[a-z0-9_]+:/ {
  at = index($0, ":")
  key = toupper(substr($0, 1, at - 1))
  value = substr($0, at + 1)
  sub(/^[ \t]+/, "", value)
  sub(/[ \t]+$/, "", value)
  printf "export %s=\"${%s:-%s}\"\n", key, key, value
}' "$file"
