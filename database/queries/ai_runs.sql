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
-- CountConversationOutcomes counts the conversation's trailing runs that
-- ended with the outcome, for the streak rules. Skipped and superseded
-- runs neither count nor break the streak.
SELECT count(*) FROM ai_runs r
WHERE r.workspace_id = @workspace_id AND r.conversation_id = @conversation_id
    AND r.outcome = @outcome
    AND NOT EXISTS (
        SELECT 1 FROM ai_runs r2
        WHERE r2.workspace_id = r.workspace_id AND r2.conversation_id = r.conversation_id
            AND r2.outcome IS NOT NULL AND r2.outcome NOT IN ('skipped', 'superseded')
            AND r2.outcome <> r.outcome
            AND (r2.created_at, r2.id) > (r.created_at, r.id)
    );

-- name: ListConversationRuns :many
-- ListConversationRuns returns a conversation's runs newest first.
SELECT * FROM ai_runs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id
ORDER BY created_at DESC
LIMIT @page_limit;

-- name: PruneAiRuns :execrows
-- PruneAiRuns deletes up to a batch of the workspace's runs older than
-- the cutoff, of one kind when a kind is given; their steps go with
-- them.
DELETE FROM ai_runs WHERE id IN (
    SELECT r.id FROM ai_runs AS r
    WHERE r.workspace_id = @workspace_id AND r.created_at < @cutoff
        AND (@kind::text = '' OR r.kind = @kind::text)
    ORDER BY r.created_at LIMIT @batch::bigint
);

-- name: SetRunToolLayer :exec
-- SetRunToolLayer records what the tool layer did during a run.
UPDATE ai_runs SET tool_layer = @tool_layer
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListConversationToolLayers :many
-- ListConversationToolLayers returns what the tool layer did in the
-- conversation's latest runs before the given one, newest first.
SELECT tool_layer FROM ai_runs
WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id <> @before
    AND tool_layer IS NOT NULL
ORDER BY created_at DESC, id DESC
LIMIT 10;

-- name: ListRunToolLayers :many
-- ListRunToolLayers returns what the tool layer did in the given runs.
SELECT tool_layer FROM ai_runs
WHERE workspace_id = @workspace_id AND id = ANY(@ids::uuid[]) AND tool_layer IS NOT NULL;

-- name: AddRunCost :exec
-- AddRunCost adds a step written after the run finished, such as the
-- summary of a long conversation, to the run's tokens and cost.
UPDATE ai_runs SET
    input_tokens = input_tokens + @input_tokens,
    cached_input_tokens = cached_input_tokens + @cached_input_tokens,
    cache_write_tokens = cache_write_tokens + @cache_write_tokens,
    output_tokens = output_tokens + @output_tokens,
    cost_micros = cost_micros + @cost_micros,
    updated_at = @now
WHERE workspace_id = @workspace_id AND id = @id;

-- name: ConversationFollowedProcedure :one
-- ConversationFollowedProcedure reports whether a run of the
-- conversation before the given one carries the flag.
SELECT EXISTS (
    SELECT 1 FROM ai_runs
    WHERE workspace_id = @workspace_id AND conversation_id = @conversation_id AND id <> @before
        AND @flag::text = ANY(flags)
);

-- name: SetRunProcedureTrace :exec
-- SetRunProcedureTrace keeps what a run did with the procedure it
-- followed.
UPDATE ai_runs SET procedure_trace = @procedure_trace WHERE workspace_id = @workspace_id AND id = @id;

-- name: ListProcedureRuns :many
-- ListProcedureRuns returns the conversation runs that followed a
-- procedure since a moment, newest first.
SELECT * FROM ai_runs
WHERE workspace_id = @workspace_id AND kind = 'conversation' AND created_at >= @since
    AND ('procedure:' || (@procedure_id::uuid)::text) = ANY (flags)
ORDER BY created_at DESC, id DESC
LIMIT @row_limit;
