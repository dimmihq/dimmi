#!/usr/bin/env bash
# Tells the error tracker (errors.trydimmi.com, Sentry-compatible) that a
# commit went live, once a release passed its smoke test:
#
#   deploy/release/tracker.sh staging <commit sha>
#   deploy/release/tracker.sh production <commit sha> v1.2.3
#
# A release exists in the tracker only once something names it, and the app
# rarely reports an error, so without this its releases page stays empty.
# The release is the commit's sha in every project the deploy carries (the
# app, the dashboard and the website), because that is the release their
# reports carry. It is created where it is missing (an existing one is left
# as it is), marked released now, and given a deploy to the environment.
# Its ref and link name the commit, or for production the version tag and
# its GitHub release, which is what the tracker shows beside the sha.
#
# TRACKER_TOKEN is an API token of the tracker; without one nothing is sent,
# so forks and self-hosted copies of the workflows skip it. TRACKER_URL and
# TRACKER_PROJECTS (slugs, space-separated) change where it goes. Any
# answer but a success fails the script, which a deploy runs without
# waiting on it or failing because of it.

set -euo pipefail

ENVIRONMENT="${1:?usage: tracker.sh <environment> <commit sha> [version]}"
SHA="${2:?usage: tracker.sh <environment> <commit sha> [version]}"
VERSION="${3:-}"
TRACKER_URL="${TRACKER_URL:-https://errors.trydimmi.com}"
PROJECTS="${TRACKER_PROJECTS:-api dashboard website}"
REPOSITORY="${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-dimmihq/dimmi}"
fail() { printf 'tracker: %s\n' "$*" >&2; exit 1; }

if [ -z "${TRACKER_TOKEN:-}" ]; then
  echo "tracker: no TRACKER_TOKEN, so the release is not registered"
  exit 0
fi

[[ "$SHA" =~ ^[0-9a-f]{40}$ ]] || fail "the commit must be a full sha"
[[ "$ENVIRONMENT" =~ ^[a-z][a-z0-9-]*$ ]] || fail "not an environment: $ENVIRONMENT"
[[ -z "$VERSION" || "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "not a version: $VERSION"

projects=""
for project in $PROJECTS; do
  [[ "$project" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || fail "not a project slug: $project"
  projects="$projects${projects:+,}\"$project\""
done
[ -n "$projects" ] || fail "no projects to register the release in"

now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
commit_url="$REPOSITORY/commit/$SHA"
if [ -n "$VERSION" ]; then
  ref="$VERSION" url="$REPOSITORY/releases/tag/$VERSION" name="$VERSION"
else
  ref="$SHA" url="$commit_url" name="${SHA:0:12}"
fi
run_url=""
if [ -n "${GITHUB_RUN_ID:-}" ]; then
  run_url="$REPOSITORY/actions/runs/$GITHUB_RUN_ID"
fi

# send <what> <method> <path> <json>: one call, which must answer 2xx.
send() {
  local what="$1" method="$2" path="$3" body="$4" code
  code="$(curl --silent --show-error --max-time 15 --request "$method" \
    --header "Authorization: Bearer $TRACKER_TOKEN" \
    --header "Content-Type: application/json" \
    --data "$body" --output "$answer" --write-out '%{http_code}' "$TRACKER_URL$path")" ||
    fail "$what: $TRACKER_URL did not answer"
  case "$code" in
    2??) ;;
    *) fail "$what: $method $path answered $code: $(head -c 300 "$answer")" ;;
  esac
}

answer="$(mktemp)"
trap 'rm -f "$answer"' EXIT

releases="/api/0/organizations/_/releases"

# 201 when it is new in a project, 208 when every project already has it.
send "create the release" POST "$releases/" \
  "{\"version\":\"$SHA\",\"projects\":[$projects],\"ref\":\"$SHA\",\"url\":\"$commit_url\",\"dateReleased\":\"$now\"}"
send "mark it released" PUT "$releases/$SHA/" \
  "{\"ref\":\"$ref\",\"url\":\"$url\",\"dateReleased\":\"$now\"}"
send "record the deploy" POST "$releases/$SHA/deploys/" \
  "{\"environment\":\"$ENVIRONMENT\",\"name\":\"$name\",\"url\":\"${run_url:-$url}\",\"dateFinished\":\"$now\",\"projects\":[$projects]}"

echo "tracker: registered $SHA${VERSION:+ ($VERSION)} as deployed to $ENVIRONMENT in $PROJECTS"
