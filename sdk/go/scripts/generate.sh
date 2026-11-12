#!/usr/bin/env sh
# Regenerates the typed REST client in api/ from dimmi's OpenAPI spec.
#
#   scripts/generate.sh [spec path or URL]
#
# Default spec: tests/contracts/api_v1.openapi.json at the repo root,
# else http://localhost:8080/api/v1/openapi.json from a running server.
set -eu

spec="${1:-}"
case "$spec" in
"" | http://* | https://* | /*) ;;
*) spec="$(pwd)/$spec" ;;
esac
cd "$(dirname "$0")/.."
OAPI_CODEGEN_VERSION=v2.5.1
spec="${spec:-$(pwd)/../../tests/contracts/api_v1.openapi.json}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

case "$spec" in
http://* | https://*) curl -fsSL "$spec" -o "$tmp/spec31.json" ;;
*)
	if [ -f "$spec" ]; then
		cp "$spec" "$tmp/spec31.json"
	else
		echo "spec $spec not found; fetching from http://localhost:8080" >&2
		curl -fsSL http://localhost:8080/api/v1/openapi.json -o "$tmp/spec31.json"
	fi
	;;
esac

go run ./scripts/openapi30 <"$tmp/spec31.json" >"$tmp/openapi30.json"
go run "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$OAPI_CODEGEN_VERSION" \
	-config api/oapi-codegen.yaml "$tmp/openapi30.json"
gofmt -w api/api.gen.go
go build ./...
echo "regenerated api/api.gen.go"
