-- name: CreateRun :one
-- CreateRun starts a run row.
INSERT INTO ai_runs (
    workspace_id, agent_id, kind, conversation_id, trigger_message_id, included_message_ids,
    actor_type, actor_id, channel_type, status, prompt_version, prompt_hashes, config_snapshot,
    started_at, created_at, updated_at
) VALUES (
    @workspace_id, @agent_id, @kind, @conversation_id, @trigger_message_id, @included_message_ids,
    @actor_type, @actor_id, @channel_type, @status, @prompt_version, @prompt_hashes, @config_snapshot,
    @started_at, @now, @now
)
RETURNING *;

-- name: FindRun :one
-- FindRun returns the run with the id in the workspace.
SELECT * FROM ai_runs WHERE workspace_id = @workspace_id AND id = @id;

-- name: FinishRun :one
-- FinishRun records how a run ended and what it cost.
UPDATE ai_runs SET
    status = @status, outcome = @outcome, skip_reason = @skip_reason, handoff_reason = @handoff_reason,
    intent = @intent, language = @language, query_original = @query_original,
    query_rewritten = @query_rewritten, retrieval = @retrieval, answer_message_id = @answer_message_id,
    flags = @flags, input_tokens = @input_tokens, cached_input_tokens = @cached_input_tokens,
    cache_write_tokens = @cache_write_tokens, output_tokens = @output_tokens,
    cost_micros = @cost_micros, price_table_version = @price_table_version,
    first_token_ms = @first_token_ms, latency_ms = @latency_ms, queue_wait_ms = @queue_wait_ms,
    error_code = @error_code, error_message = @error_message,
    action_calls_count = @action_calls_count,
    completed_at = @completed_at, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: ListRuns :many
-- ListRuns returns runs newest first, filtered.
SELECT * FROM ai_runs
WHERE workspace_id = @workspace_id
    AND (@conversation_id::uuid IS NULL OR conversation_id = @conversation_id)
    AND (@kind::text = '' OR kind = @kind)
    AND (@outcome::text = '' OR outcome = @outcome)
    AND (@created_after::timestamptz IS NULL OR created_at >= @created_after)
    AND (@created_before::timestamptz IS NULL OR created_at <= @created_before)
ORDER BY created_at DESC, id DESC
LIMIT @page_limit;

-- name: SetRunFeedback :one
-- SetRunFeedback records the customer's verdict on a run.
UPDATE ai_runs SET feedback = @feedback, updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id
RETURNING *;

-- name: CountConversationOutcomes :one
-- CountConversationOutcomes counts consecutive trailing runs of one
-- outcome kind for the streak rules.
SELECT count(*) FROM ai_runs r
WHERE r.workspace_id = @workspace_id AND r.conversation_id = @conversation_id
    AND r.outcome = @outcome AND r.id > COALESCE((
        SELECT max(r2.id) FROM ai_runs r2
        WHERE r2.workspace_id = r.workspace_id AND r2.conversation_id = r.conversation_id
            AND r2.outcome IS DISTINCT FROM r.outcome AND r2.outcome IS NOT NULL
    ), '00000000-0000-0000-0000-000000000000'::uuid);

-- name: ListRunsForPayloadPrune :many
-- ListRunsForPayloadPrune returns step-holding runs past payload
-- retention, in batches.
SELECT DISTINCT r.* FROM ai_runs r
JOIN ai_run_steps s ON s.run_id = r.id AND (s.request IS NOT NULL OR s.response IS NOT NULL)
WHERE r.workspace_id = @workspace_id AND r.completed_at < @before
ORDER BY r.created_at
LIMIT @batch_limit;

-- name: ListConversationRuns :many
-- ListConversationRuns returns a conversation's runs newest first.
SELECT * FROM ai_runs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id
ORDER BY created_at DESC
LIMIT @page_limit;

-- name: ResolveLatestRunOutcome :execrows
-- ResolveLatestRunOutcome stamps a resolution outcome on the
-- conversation's newest finished run.
UPDATE ai_runs AS ar SET outcome = @outcome, completed_at = @now
WHERE ar.workspace_id = @workspace_id
  AND ar.id = (
      SELECT r.id FROM ai_runs r
      WHERE r.workspace_id = ar.workspace_id AND r.conversation_id = @conversation_id
        AND r.status = 'completed'
      ORDER BY r.id DESC LIMIT 1
  );
