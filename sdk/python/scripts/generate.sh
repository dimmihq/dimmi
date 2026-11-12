#!/usr/bin/env bash
# Generates the REST client in src/dimmi/_generated from the dimmi
# OpenAPI spec.
#
#   scripts/generate.sh [SPEC_FILE_OR_URL]
#
# The default spec is the committed snapshot
# tests/contracts/api_v1.openapi.json at the repo root; when that file is
# absent it fetches http://localhost:8080/api/v1/openapi.json. The generator is pinned to openapi-python-client
# 0.29.0: later releases emit httpx2 and typing.Self, which would drop
# httpx and Python 3.10.
set -euo pipefail

GENERATOR="openapi-python-client==0.29.0"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SNAPSHOT="$ROOT/../../tests/contracts/api_v1.openapi.json"
if [[ $# -ge 1 ]]; then
  SPEC="$1"
elif [[ -f "$SNAPSHOT" ]]; then
  SPEC="$SNAPSHOT"
else
  SPEC="http://localhost:8080/api/v1/openapi.json"
fi
TARGET="$ROOT/src/dimmi/_generated"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

if [[ -f "$SPEC" ]]; then
  cp "$SPEC" "$WORK/openapi.raw.json"
else
  curl -fsSL "$SPEC" -o "$WORK/openapi.raw.json"
fi

# Preprocessing: the generator reads OpenAPI 3.1 as dimmi serves it.
# This step only pins the server URL to the API root so the generated
# paths stay relative ("/contacts"), and drops nothing else. Add fixes
# here if a future spec trips the generator.
python3 - "$WORK/openapi.raw.json" "$WORK/openapi.json" <<'PY'
import json, sys
spec = json.load(open(sys.argv[1]))
spec["servers"] = [{"url": "/api/v1"}]
json.dump(spec, open(sys.argv[2], "w"))
PY

cat > "$WORK/config.yml" <<'YML'
project_name_override: dimmi-generated
package_name_override: _generated
post_hooks:
  - "ruff check . --isolated --target-version py310 --select I,F401 --fix-only --quiet --exit-zero"
  - "ruff format . --isolated --target-version py310 --line-length 120 --quiet"
YML

if command -v uvx >/dev/null 2>&1; then
  run() { uvx --from "$GENERATOR" --with ruff openapi-python-client "$@"; }
else
  python3 -m venv "$WORK/venv"
  "$WORK/venv/bin/pip" install -q "$GENERATOR" ruff
  export PATH="$WORK/venv/bin:$PATH"
  run() { openapi-python-client "$@"; }
fi

run generate --path "$WORK/openapi.json" --config "$WORK/config.yml" \
  --output-path "$WORK/out" --meta none

# Deprecated names the spec no longer has stay importable: copy each
# shim in scripts/compat/models unless the spec brought the module back.
for shim in "$ROOT/scripts/compat/models/"*.py; do
  [[ -e "$shim" ]] || continue
  module="$(basename "$shim" .py)"
  [[ -e "$WORK/out/models/$module.py" ]] && continue
  cp "$shim" "$WORK/out/models/"
  classes="$(sed -nE 's/^class ([A-Za-z0-9_]+).*/\1/p; s/^from .* import [A-Za-z0-9_]+ as ([A-Za-z0-9_]+)$/\1/p' "$shim")"
  for cls in $classes; do
    printf '\nfrom .%s import %s  # deprecated\n__all__ += ("%s",)\n' "$module" "$cls" "$cls" >> "$WORK/out/models/__init__.py"
  done
done

rm -rf "$WORK/out/.ruff_cache" "$TARGET"
mv "$WORK/out" "$TARGET"
echo "Generated $(find "$TARGET/api" -name '*.py' ! -name '__init__.py' | wc -l | tr -d ' ') operations into $TARGET"
