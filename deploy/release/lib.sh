# Shared by release.sh and rollback.sh. Everything reads the environment's
# stack outputs, so nothing about an account is written here.
#
# Needs: ENVIRONMENT, AWS_REGION, and AWS credentials for that environment's
# deploy role.

set -euo pipefail

: "${ENVIRONMENT:?set ENVIRONMENT, such as staging}"
: "${AWS_REGION:?set AWS_REGION to the Region of the environment}"

REPOSITORY="dimmi-${ENVIRONMENT}"
RELEASE_TAG="release"
PREVIOUS_TAG="previous"

say() { printf '%s\n' "$*" >&2; }

output() {
  aws cloudformation describe-stacks --stack-name "dimmi-${ENVIRONMENT}-$1" \
    --query "Stacks[0].Outputs[?OutputKey=='$2'].OutputValue" --output text
}

# digest_of TAG prints the digest TAG points at, or nothing when it does not exist.
digest_of() {
  aws ecr describe-images --repository-name "$REPOSITORY" --image-ids "imageTag=$1" \
    --query 'imageDetails[0].imageDigest' --output text 2>/dev/null || true
}

# point TAG DIGEST moves TAG to the image DIGEST without pulling it.
point() {
  local manifest
  manifest="$(aws ecr batch-get-image --repository-name "$REPOSITORY" --image-ids "imageDigest=$2" \
    --query 'images[0].imageManifest' --output text)"
  if [ "$(digest_of "$1")" = "$2" ]; then
    return 0
  fi
  aws ecr put-image --repository-name "$REPOSITORY" --image-tag "$1" --image-manifest "$manifest" >/dev/null
  say "tag $1 now points at $2"
}

# roll forces every service onto the image the release tag points at and
# follows each new deployment until ECS settles it. ECS pins the digest per
# deployment, so the circuit breaker's own rollback returns to exactly the
# image that ran before.
roll() {
  local cluster services service deployments=() i
  cluster="$(output app ClusterName)"
  IFS=',' read -r -a services <<<"$(output app ServiceNames)"

  for service in "${services[@]}"; do
    deployments+=("$(aws ecs update-service --cluster "$cluster" --service "$service" --force-new-deployment \
      --query 'service.deployments[?status==`PRIMARY`].id | [0]' --output text)")
    say "rolling $service"
  done

  for i in "${!services[@]}"; do
    settle "$cluster" "${services[$i]}" "${deployments[$i]}" || return 1
  done
}

# settle waits up to half an hour for one deployment of a service to finish.
# A deployment still draining the old tasks reports itself in progress, and
# one the circuit breaker replaced leaves the list or reports failed. A
# paused service runs no task, so its deployment never finishes and counts
# as settled.
settle() {
  local cluster="$1" service="$2" deployment="$3" state desired attempt
  for attempt in $(seq 1 120); do
    read -r state desired <<<"$(aws ecs describe-services --cluster "$cluster" --services "$service" \
      --query "services[0].[deployments[?id=='$deployment'].rolloutState | [0], desiredCount]" --output text)"
    case "$state" in
      COMPLETED) say "$service released"; return 0 ;;
      FAILED | None) say "$service failed its rollout: the circuit breaker rolled it back"; return 1 ;;
    esac
    if [ "$desired" = "0" ]; then
      say "$service is paused"
      return 0
    fi
    sleep 15
  done
  say "$service was still rolling after $attempt checks"
  return 1
}

# run_task NAME runs a one-off task definition to the end and fails when its
# container exits with anything but 0.
run_task() {
  local cluster task code
  cluster="$(output app ClusterName)"
  task="$(aws ecs run-task --cluster "$cluster" --task-definition "dimmi-${ENVIRONMENT}-$1" --launch-type FARGATE \
    --network-configuration "awsvpcConfiguration={subnets=[$(output app TaskSubnetIds)],securityGroups=[$(output app TaskSecurityGroupId)],assignPublicIp=$(output app TaskAssignPublicIp)}" \
    --query 'tasks[0].taskArn' --output text)"
  say "started $1: $task"

  aws ecs wait tasks-stopped --cluster "$cluster" --tasks "$task"
  code="$(aws ecs describe-tasks --cluster "$cluster" --tasks "$task" \
    --query 'tasks[0].containers[0].exitCode' --output text)"
  if [ "$code" != "0" ]; then
    say "$1 failed with exit code $code; its log is in $(output app LogGroupName), stream prefix $1"
    return 1
  fi
  say "$1 finished"
}
