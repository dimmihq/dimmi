-- name: CreateOperatorRoutine :one
-- CreateOperatorRoutine stores a new routine.
INSERT INTO operator_routines (
    workspace_id, name, description, template_key, prompt, cadence, cron_expr, run_at, timezone, next_run_at,
    created_by_member_id, owner_member_id, permissions, tool_groups, inbox_ids, write_policies, scope, model,
    effort, max_model_calls, max_tool_calls, monthly_budget_micros, notify_policy, delivery, status,
    created_at, updated_at
) VALUES (
    @workspace_id, @name, @description, sqlc.narg('template_key'), @prompt, @cadence, sqlc.narg('cron_expr'),
    sqlc.narg('run_at'), @timezone, sqlc.narg('next_run_at'), sqlc.narg('created_by_member_id'),
    sqlc.narg('owner_member_id'), @permissions, @tool_groups, @inbox_ids, @write_policies, @scope,
    sqlc.narg('model'), sqlc.narg('effort'), @max_model_calls, @max_tool_calls, sqlc.narg('monthly_budget_micros'),
    @notify_policy, @delivery, @status, @now, @now
)
RETURNING *;

-- name: UpdateOperatorRoutine :one
-- UpdateOperatorRoutine writes a routine's settings whole.
UPDATE operator_routines SET
    name = @name, description = @description, prompt = @prompt, cadence = @cadence,
    cron_expr = sqlc.narg('cron_expr'), run_at = sqlc.narg('run_at'), timezone = @timezone,
    next_run_at = sqlc.narg('next_run_at'), owner_member_id = sqlc.narg('owner_member_id'),
    permissions = @permissions, tool_groups = @tool_groups, inbox_ids = @inbox_ids,
    write_policies = @write_policies, scope = @scope, model = sqlc.narg('model'), effort = sqlc.narg('effort'),
    max_model_calls = @max_model_calls, max_tool_calls = @max_tool_calls,
    monthly_budget_micros = sqlc.narg('monthly_budget_micros'), notify_policy = @notify_policy,
    delivery = @delivery, status = @status, pause_reason = sqlc.narg('pause_reason'),
    consecutive_failures = @consecutive_failures, state = @state, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
RETURNING *;

-- name: FindOperatorRoutine :one
-- FindOperatorRoutine returns one routine that was not deleted.
SELECT * FROM operator_routines WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: FindAnyOperatorRoutine :one
-- FindAnyOperatorRoutine returns one routine, deleted or not, for the
-- threads and suggestions it left behind.
SELECT * FROM operator_routines WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockOperatorRoutine :one
-- LockOperatorRoutine returns one routine, held until the transaction
-- ends; a routine another round holds is skipped.
SELECT * FROM operator_routines
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL
FOR UPDATE SKIP LOCKED;

-- name: ListOperatorRoutines :many
-- ListOperatorRoutines returns the workspace's routines, newest first,
-- of one status when asked.
SELECT * FROM operator_routines
WHERE workspace_id = @workspace_id AND deleted_at IS NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: DeleteOperatorRoutine :execrows
-- DeleteOperatorRoutine stops a routine for good; its threads and
-- suggestions stay.
UPDATE operator_routines SET deleted_at = @now, next_run_at = NULL, status = 'paused', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND deleted_at IS NULL;

-- name: OperatorRoutinesDue :many
-- OperatorRoutinesDue returns the routines due to run, across workspaces.
SELECT workspace_id::uuid AS workspace_id, id::uuid AS id FROM dimmi_operator_routines_due(@at::timestamptz, @batch::integer);

-- name: SaveOperatorRoutineLifecycle :exec
-- SaveOperatorRoutineLifecycle records when a routine runs next and how
-- its last run went.
UPDATE operator_routines SET
    next_run_at = sqlc.narg('next_run_at'), status = @status, pause_reason = sqlc.narg('pause_reason'),
    consecutive_failures = @consecutive_failures, last_run_at = sqlc.narg('last_run_at'),
    last_success_at = sqlc.narg('last_success_at'), last_outcome = sqlc.narg('last_outcome'), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: SetOperatorRoutineState :exec
-- SetOperatorRoutineState stores what a routine keeps between runs.
UPDATE operator_routines SET state = @state, updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: CreateOperatorRoutineRun :one
-- CreateOperatorRoutineRun queues one run of a routine.
INSERT INTO operator_routine_runs (
    workspace_id, routine_id, trigger, triggered_by_member_id, scheduled_for, status, skip_reason, outcome,
    completed_at, created_at, updated_at
) VALUES (
    @workspace_id, @routine_id, @trigger, sqlc.narg('triggered_by_member_id'), sqlc.narg('scheduled_for'), @status,
    sqlc.narg('skip_reason'), sqlc.narg('outcome'), sqlc.narg('completed_at'), @now, @now
)
RETURNING *;

-- name: FindOperatorRoutineRun :one
-- FindOperatorRoutineRun returns one run of a routine.
SELECT * FROM operator_routine_runs WHERE workspace_id = @workspace_id AND routine_id = @routine_id AND id = @id;

-- name: FindOperatorRoutineRunByID :one
-- FindOperatorRoutineRunByID returns one run, whichever routine it
-- belongs to.
SELECT * FROM operator_routine_runs WHERE workspace_id = @workspace_id AND id = @id;

-- name: LockOperatorRoutineRun :one
-- LockOperatorRoutineRun returns one run, held until the transaction ends.
SELECT * FROM operator_routine_runs WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: FindActiveOperatorRoutineRun :one
-- FindActiveOperatorRoutineRun returns the routine's live run still
-- queued or running.
SELECT * FROM operator_routine_runs
WHERE workspace_id = @workspace_id AND routine_id = @routine_id
  AND status IN ('queued', 'running') AND trigger <> 'dry_run'
LIMIT 1;

-- name: SaveOperatorRoutineRun :one
-- SaveOperatorRoutineRun records how a run is going or how it ended.
UPDATE operator_routine_runs SET
    status = @status, outcome = sqlc.narg('outcome'), noop_reason = sqlc.narg('noop_reason'),
    skip_reason = sqlc.narg('skip_reason'), error_code = sqlc.narg('error_code'),
    error_message = sqlc.narg('error_message'), thread_id = sqlc.narg('thread_id'),
    ai_run_id = sqlc.narg('ai_run_id'), changesets_count = @changesets_count, cost_micros = @cost_micros,
    notified = @notified, delivery_results = @delivery_results, started_at = sqlc.narg('started_at'),
    completed_at = sqlc.narg('completed_at'), updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: SaveOperatorRoutineRunSteps :exec
-- SaveOperatorRoutineRunSteps keeps the steps a run has finished so far.
UPDATE operator_routine_runs SET steps = @steps, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListOperatorRoutineRuns :many
-- ListOperatorRoutineRuns returns a routine's runs, newest first.
SELECT * FROM operator_routine_runs
WHERE workspace_id = @workspace_id AND routine_id = @routine_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: SumOperatorRoutineCost :one
-- SumOperatorRoutineCost is what a routine's runs cost since a moment.
SELECT COALESCE(SUM(cost_micros), 0)::bigint FROM operator_routine_runs
WHERE workspace_id = @workspace_id AND routine_id = @routine_id AND created_at >= @since;

-- name: ListOperatorRoutineCosts :many
-- ListOperatorRoutineCosts is what each routine's runs cost in a range
-- and how many ran.
SELECT r.id, r.name, COUNT(x.id)::bigint AS runs_count, COALESCE(SUM(x.cost_micros), 0)::bigint AS cost_micros
FROM operator_routines r
JOIN operator_routine_runs x ON x.routine_id = r.id AND x.workspace_id = r.workspace_id
WHERE r.workspace_id = @workspace_id AND x.created_at >= @since AND x.created_at < @until
  AND x.status IN ('completed', 'failed')
GROUP BY r.id, r.name
ORDER BY cost_micros DESC, r.id
LIMIT @row_limit;

-- name: CreateOperatorRoutineThread :one
-- CreateOperatorRoutineThread opens the shared thread of one run.
INSERT INTO operator_threads (workspace_id, member_id, routine_id, routine_run_id, title, created_at, updated_at)
VALUES (@workspace_id, NULL, @routine_id, @routine_run_id, @title, @now, @now)
RETURNING *;

-- name: FindAnyOperatorThread :one
-- FindAnyOperatorThread returns one thread whoever owns it; the caller
-- decides who may read it.
SELECT * FROM operator_threads WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListOperatorRoutineThreads :many
-- ListOperatorRoutineThreads returns a routine's threads, latest first.
SELECT * FROM operator_threads
WHERE workspace_id = @workspace_id AND routine_id = @routine_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: FindOperatorChangesetByDedupKey :one
-- FindOperatorChangesetByDedupKey returns the latest suggestion with the
-- key that still waits, or that the team declined since a moment. One
-- declined only because it went stale does not count.
SELECT * FROM operator_changesets
WHERE workspace_id = @workspace_id AND dedup_key = @dedup_key AND NOT preview
  AND (status = 'pending' OR (status = 'declined' AND updated_at >= @declined_since
    AND (expires_at IS NULL OR updated_at < expires_at)))
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListOperatorArticleSuggestions :many
-- ListOperatorArticleSuggestions returns the suggestions about one
-- article that a new one could repeat: those that still wait, a dry
-- run's previews only for that same run, and those the team declined
-- since a moment. One declined only because it went stale does not count.
SELECT * FROM operator_changesets
WHERE workspace_id = @workspace_id
  AND operations @> jsonb_build_array(jsonb_build_object('article_id', @article_id::text))
  AND ((status = 'pending' AND (NOT preview OR routine_run_id = sqlc.narg('routine_run_id')))
    OR (NOT preview AND status = 'declined' AND updated_at >= @declined_since
      AND (expires_at IS NULL OR updated_at < expires_at)))
ORDER BY created_at DESC, id DESC
LIMIT 50;

-- name: ListOperatorRunChangesets :many
-- ListOperatorRunChangesets returns the suggestions one routine run left.
SELECT * FROM operator_changesets
WHERE workspace_id = @workspace_id AND routine_run_id = @routine_run_id
ORDER BY created_at, id;

-- name: MarkOperatorRoutineRunNotified :exec
-- MarkOperatorRoutineRunNotified records that a run's people were told,
-- and what reached whom.
UPDATE operator_routine_runs SET notified = true, delivery_results = @delivery_results, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: CountOperatorRoutineChangesets :many
-- CountOperatorRoutineChangesets counts a workspace's routine suggestions
-- by routine and status, dry run previews left out.
SELECT routine_id::uuid AS routine_id, status, COUNT(*)::bigint AS changesets_count
FROM operator_changesets
WHERE workspace_id = @workspace_id AND routine_id IS NOT NULL AND NOT preview
GROUP BY routine_id, status;

-- name: ExpireOperatorChangesets :execrows
-- ExpireOperatorChangesets declines the workspace's routine suggestions
-- nobody decided before they went stale.
UPDATE operator_changesets
SET status = 'declined', error = 'Nobody decided it before it went stale.', updated_at = @now
WHERE workspace_id = @workspace_id AND status = 'pending' AND expires_at IS NOT NULL AND expires_at <= @now;
