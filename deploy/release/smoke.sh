#!/usr/bin/env bash
# Checks a deployed environment from the outside:
#
#   deploy/release/smoke.sh https://staging.trydimmi.com [expect-gate]
#
# The certificate must verify, health must answer, plain HTTP must redirect to
# HTTPS, and the gate must be on exactly where it is expected. GATE_KEY, when
# set, is sent as the gate's CI key header.

set -euo pipefail

BASE="${1:?usage: smoke.sh <https url> [expect-gate]}"
EXPECT_GATE="${2:-}"
HOST="${BASE#https://}"
GATE_HEADER="${GATE_HEADER:-X-Gate-Key}"
fail() { printf 'smoke: %s\n' "$*" >&2; exit 1; }

status() { curl --silent --output /dev/null --write-out '%{http_code}' --max-time 15 "$@"; }

for path in /up /ready; do
  code="$(status "$BASE$path")" || fail "$BASE$path did not answer over verified TLS"
  [ "$code" = "200" ] || fail "$BASE$path answered $code"
done

code="$(status "http://$HOST/")"
case "$code" in 301 | 308) ;; *) fail "http://$HOST/ answered $code, expected a redirect to HTTPS" ;; esac

code="$(status "$BASE/d")"
if [ -n "$EXPECT_GATE" ]; then
  [ "$code" = "401" ] || [ "$code" = "303" ] || [ "$code" = "302" ] || fail "the gate is off: $BASE/d answered $code"
  if [ -n "${GATE_KEY:-}" ]; then
    code="$(status --header "$GATE_HEADER: $GATE_KEY" "$BASE/d")"
    [ "$code" = "200" ] || fail "the CI key did not pass the gate: $code"
  fi
else
  [ "$code" = "200" ] || fail "$BASE/d answered $code; is a gate on where none should be?"
fi

printf 'smoke: %s is healthy\n' "$BASE"
