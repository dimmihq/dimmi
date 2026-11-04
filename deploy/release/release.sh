#!/usr/bin/env bash
# Releases one image to one environment:
#
#   ENVIRONMENT=staging AWS_REGION=us-east-2 deploy/release/release.sh <image tag>
#
# 1. remembers what runs now as `previous`
# 2. points `release` at the new image
# 3. provisions database roles (idempotent), then migrates as the owner
# 4. rolls every service and waits until each is stable
#
# Migrations only add, so the old release keeps working while the new one
# starts, and a rollback never undoes one. If migrating or rolling fails,
# `release` is pointed back at `previous` before the script exits.

source "$(dirname "$0")/lib.sh"

TAG="${1:?usage: release.sh <image tag>}"
NEW="$(digest_of "$TAG")"
if [ -z "$NEW" ] || [ "$NEW" = "None" ]; then
  say "image $TAG is not in $REPOSITORY"
  exit 1
fi

CURRENT="$(digest_of "$RELEASE_TAG")"
if [ -n "$CURRENT" ] && [ "$CURRENT" != "None" ]; then
  point "$PREVIOUS_TAG" "$CURRENT"
fi

restore() {
  if [ -n "$CURRENT" ] && [ "$CURRENT" != "None" ]; then
    point "$RELEASE_TAG" "$CURRENT"
    say "release failed; $RELEASE_TAG points at the image that ran before"
  fi
  exit 1
}

point "$RELEASE_TAG" "$NEW"
run_task provision || restore
run_task migrate || restore
roll || restore

say "released $TAG ($NEW) to $ENVIRONMENT"
