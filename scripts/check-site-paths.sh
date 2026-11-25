#!/usr/bin/env bash
# Checks that the edge sends every top-level path of the built site to the
# site: the site shares its host with the app, and the edge's Caddyfile
# lists the site's paths, so a new page missing from that list would
# reach the app instead.
#
#   scripts/check-site-paths.sh [site/dist]

set -euo pipefail

DIST="${1:-site/dist}"
CADDYFILE="$(dirname "$0")/../deploy/k8s/apps/base/edge/Caddyfile"
[[ -d "$DIST" ]] || { echo "no built site at $DIST; run pnpm site:build" >&2; exit 2; }

matcher=" $(grep -E '^\s*@site path ' "$CADDYFILE" | sed -E 's/^\s*@site path//') "
missing=0
for entry in "$DIST"/*; do
  name="$(basename "$entry")"
  if [[ -d "$entry" ]]; then
    [[ "$name" == "index.html" ]] && continue
    if [[ -f "$entry/index.html" ]]; then want=("/$name" "/$name/*"); else want=("/$name/*"); fi
  else
    [[ "$name" == "index.html" ]] && want=("/") || want=("/$name")
  fi
  for path in "${want[@]}"; do
    if [[ "$matcher" != *" $path "* ]]; then
      echo "the edge does not send $path to the site; add it to @site in $CADDYFILE" >&2
      missing=1
    fi
  done
done
exit "$missing"
