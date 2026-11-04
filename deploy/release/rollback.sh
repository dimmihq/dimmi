#!/usr/bin/env bash
# Puts an earlier image back on one environment:
#
#   ENVIRONMENT=staging AWS_REGION=us-east-2 deploy/release/rollback.sh [image tag]
#
# Without a tag it returns to `previous`, the image that ran before the last
# release. It never migrates: the schema stays, and the older code runs on it
# because migrations only ever add.

source "$(dirname "$0")/lib.sh"

TARGET="$(digest_of "${1:-$PREVIOUS_TAG}")"
if [ -z "$TARGET" ] || [ "$TARGET" = "None" ]; then
  say "no image tagged ${1:-$PREVIOUS_TAG} in $REPOSITORY"
  exit 1
fi

point "$RELEASE_TAG" "$TARGET"
roll
say "rolled $ENVIRONMENT back to $TARGET"
