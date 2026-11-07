-- name: CreateWorkflowRun :one
-- CreateWorkflowRun starts a run. A second start for the same workflow
-- and event, or while the workflow already runs on the conversation,
-- does nothing and returns no row.
INSERT INTO workflow_runs (
    id, workspace_id, workflow_id, version_id, conversation_id, contact_id, kind, mode, status, trigger,
    trigger_event_id, trigger_data, spec, parent_run_id, parent_step_id, chain, cursor_step_id, vars,
    started_at, created_at, updated_at
) VALUES (
    @id, @workspace_id, @workflow_id, @version_id, @conversation_id, @contact_id, @kind, @mode, 'running', @trigger,
    @trigger_event_id, @trigger_data, @spec, @parent_run_id, @parent_step_id, @chain, @cursor_step_id, @vars,
    @now, @now, @now
)
ON CONFLICT DO NOTHING
RETURNING *;

-- name: LockWorkflowRun :one
-- LockWorkflowRun returns a run and holds it until the transaction ends.
SELECT * FROM workflow_runs WHERE workspace_id = @workspace_id AND id = @id FOR UPDATE;

-- name: FindWorkflowRun :one
-- FindWorkflowRun returns one run.
SELECT * FROM workflow_runs WHERE workspace_id = @workspace_id AND id = @id;

-- name: SaveWorkflowRun :one
-- SaveWorkflowRun writes a run's progress and moves its sequence on.
UPDATE workflow_runs SET
    status = @status, end_reason = @end_reason, cursor_step_id = @cursor_step_id, wait = @wait, wake_at = @wake_at,
    vars = @vars, steps_executed = @steps_executed, acted = @acted, error = @error, ended_at = @ended_at,
    seq = seq + 1, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ActiveWorkflowRuns :many
-- ActiveWorkflowRuns returns the runs still going on a conversation.
SELECT * FROM workflow_runs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND status IN ('running', 'waiting')
ORDER BY started_at;

-- name: WorkflowRanOnConversation :one
-- WorkflowRanOnConversation reports when the workflow last started a run
-- on the conversation that did something, null when never.
SELECT max(started_at)::timestamptz AS last_started_at FROM workflow_runs
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id AND conversation_id = @conversation_id
  AND status <> 'passed';

-- name: CountConversationStepsSince :one
-- CountConversationStepsSince counts the steps runs executed on a
-- conversation since a moment, the conversation's hourly budget.
SELECT count(*)::integer AS steps FROM workflow_run_steps s
JOIN workflow_runs r ON r.id = s.run_id
WHERE r.workspace_id = @workspace_id AND r.conversation_id = @conversation_id AND s.started_at >= @since;

-- name: CountBotMessagesSince :one
-- CountBotMessagesSince counts the messages workflows sent on a
-- conversation since a moment.
SELECT count(*)::integer AS messages FROM workflow_run_steps s
JOIN workflow_runs r ON r.id = s.run_id
WHERE r.workspace_id = @workspace_id AND r.conversation_id = @conversation_id AND s.started_at >= @since
  AND s.outcome = 'sent';

-- name: ListConversationWorkflowRuns :many
-- ListConversationWorkflowRuns returns a conversation's runs, newest
-- first.
SELECT r.*, w.name AS workflow_name FROM workflow_runs r
JOIN workflows w ON w.id = r.workflow_id
WHERE r.workspace_id = @workspace_id AND r.conversation_id = @conversation_id
ORDER BY r.started_at DESC, r.id DESC
LIMIT @row_limit;

-- name: ListWorkflowRuns :many
-- ListWorkflowRuns returns a workflow's runs, newest first, before a
-- cursor when paging.
SELECT * FROM workflow_runs
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id
  AND (sqlc.narg(before)::uuid IS NULL OR id < sqlc.narg(before)::uuid)
ORDER BY id DESC
LIMIT @row_limit;

-- name: DueWorkflowRuns :many
-- DueWorkflowRuns returns runs whose timer passed a while ago without
-- being resumed, for the reconciler.
SELECT id, workspace_id, seq FROM workflow_runs
WHERE status = 'waiting' AND wake_at IS NOT NULL AND wake_at < @before
ORDER BY wake_at
LIMIT @row_limit;

-- name: ExpiredWorkflowRuns :many
-- ExpiredWorkflowRuns returns runs waiting since before a moment, which
-- have outlived the longest a run may wait.
SELECT id, workspace_id FROM workflow_runs
WHERE status = 'waiting' AND started_at < @before
ORDER BY started_at
LIMIT @row_limit;

-- name: CreateWorkflowRunStep :one
-- CreateWorkflowRunStep records one executed step in the run's trace.
INSERT INTO workflow_run_steps (
    id, workspace_id, run_id, workflow_id, version_id, step_id, kind, status, path_id, outcome, detail,
    started_at, finished_at, created_at, updated_at
) VALUES (
    @id, @workspace_id, @run_id, @workflow_id, @version_id, @step_id, @kind, @status, @path_id, @outcome, @detail,
    @now, @finished_at, @now, @now
)
RETURNING *;

-- name: FinishWorkflowRunStep :exec
-- FinishWorkflowRunStep closes a step that was waiting.
UPDATE workflow_run_steps SET
    status = @status, path_id = @path_id, outcome = @outcome, detail = @detail, finished_at = @now, updated_at = @now
WHERE workspace_id = @workspace_id AND run_id = @run_id AND step_id = @step_id AND status = 'waiting';

-- name: ListWorkflowRunSteps :many
-- ListWorkflowRunSteps returns a run's trace in order.
SELECT * FROM workflow_run_steps WHERE workspace_id = @workspace_id AND run_id = @run_id
ORDER BY started_at, id;

-- name: ClaimConversationForRun :execrows
-- ClaimConversationForRun hands an unclaimed conversation to a
-- customer-facing run, and keeps the automatic agent out of it.
UPDATE conversations SET workflow_run_id = @run_id, ai_mode = 'workflow', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @conversation_id AND workflow_run_id IS NULL;

-- name: ReleaseConversationClaim :exec
-- ReleaseConversationClaim lets go of a conversation a run held.
UPDATE conversations SET workflow_run_id = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @conversation_id AND workflow_run_id = @run_id;

-- name: CreateWorkflowMatchLog :exec
-- CreateWorkflowMatchLog records each live workflow's verdict on an
-- event.
INSERT INTO workflow_match_logs (id, workspace_id, conversation_id, trigger, event_id, verdicts, created_at)
VALUES (@id, @workspace_id, @conversation_id, @trigger, @event_id, @verdicts, @now);

-- name: ListWorkflowMatchLogs :many
-- ListWorkflowMatchLogs returns a conversation's match logs, newest
-- first.
SELECT * FROM workflow_match_logs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;

-- name: PruneWorkflowMatchLogs :execrows
-- PruneWorkflowMatchLogs deletes match logs older than a moment.
DELETE FROM workflow_match_logs AS l WHERE l.id IN (
    SELECT o.id FROM workflow_match_logs AS o WHERE o.created_at < sqlc.arg(before)::timestamptz
    LIMIT sqlc.arg(row_limit)::integer
);
