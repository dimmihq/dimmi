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
-- on the conversation that did something; no row when it never did.
SELECT started_at AS last_started_at FROM workflow_runs
WHERE workspace_id = @workspace_id AND workflow_id = @workflow_id AND conversation_id = @conversation_id
  AND status <> 'passed'
ORDER BY started_at DESC
LIMIT 1;

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

-- name: ResetConversationAiMode :exec
-- ResetConversationAiMode gives a closed conversation back to the
-- automatic agent's own rules.
UPDATE conversations SET ai_mode = 'auto', updated_at = @now WHERE workspace_id = @workspace_id AND id = @id;

-- name: ScrubWorkflowRunsForContact :execrows
-- ScrubWorkflowRunsForContact forgets what an erased customer's runs
-- learned about them; the runs stay as a record that they happened.
UPDATE workflow_runs SET vars = '{}', trigger_data = '{}', contact_id = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND contact_id = @contact_id;

-- name: PublicAuthorBefore :one
-- PublicAuthorBefore names who wrote the conversation's last public
-- message before the given one, so a short "yes" is read as confirming
-- the agent's answer only when that answer came right before it.
SELECT author_type FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message'
  AND deleted_at IS NULL AND id < @message_id
ORDER BY id DESC
LIMIT 1;

-- name: LastAgentAndCustomerMessages :one
-- LastAgentAndCustomerMessages returns when the agent and the customer
-- last wrote publicly on the conversation since a moment; the Unix
-- epoch stands for not at all.
SELECT
    coalesce(max(created_at) FILTER (WHERE author_type = 'agent'), 'epoch'::timestamptz)::timestamptz AS agent_at,
    coalesce(max(created_at) FILTER (WHERE author_type = 'contact'), 'epoch'::timestamptz)::timestamptz AS contact_at
FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message'
  AND deleted_at IS NULL AND created_at >= @since;

-- name: StandDownConversationAi :exec
-- StandDownConversationAi stops the agent answering once a workflow
-- step that let it handle the conversation is over.
UPDATE conversations SET ai_state = 'off', updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND ai_state = 'active';

-- name: ReadyConversationAi :exec
-- ReadyConversationAi lets the agent answer again on a conversation it
-- was handed away from, as a workflow step hands it back.
UPDATE conversations SET ai_state = 'off', ai_handed_off_at = NULL, ai_handoff_reason = NULL, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id AND ai_state = 'handed_off';

-- name: LastCustomerMessage :one
-- LastCustomerMessage returns the conversation's newest message when
-- the customer wrote it, so the agent can answer what is waiting.
SELECT id, author_type FROM messages
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND type = 'message'
  AND deleted_at IS NULL
ORDER BY id DESC
LIMIT 1;

-- name: QuietAnchors :one
-- QuietAnchors returns what a quiet trigger checks before it fires: the
-- newest public reply by a teammate or the agent, the nil id for none,
-- and whether the customer wrote after the anchor message.
SELECT
    coalesce((
        SELECT m.id FROM messages m
        WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id AND m.type = 'message'
          AND m.author_type IN ('member', 'agent') AND m.deleted_at IS NULL
        ORDER BY m.id DESC LIMIT 1
    ), '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS last_reply_id,
    EXISTS (
        SELECT 1 FROM messages m
        WHERE m.workspace_id = @workspace_id AND m.conversation_id = @conversation_id AND m.type = 'message'
          AND m.author_type = 'contact' AND m.deleted_at IS NULL AND m.id > @anchor_id
    )::boolean AS customer_wrote;
