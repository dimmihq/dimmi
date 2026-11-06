-- name: CreateRunStep :one
-- CreateRunStep records one stage of a run.
INSERT INTO ai_run_steps (
    workspace_id, run_id, seq, kind, provider, provider_key_id, model, response_model, attempt,
    retry_of_step_id, status, error_kind, provider_error_code, http_status, provider_request_id,
    request, response, input_tokens, cached_input_tokens, cache_write_tokens, output_tokens,
    reasoning_tokens, usage_estimated, cost_micros, latency_ms, first_token_ms,
    started_at, completed_at, created_at
) VALUES (
    @workspace_id, @run_id, @seq, @kind, @provider, @provider_key_id, @model, @response_model, @attempt,
    @retry_of_step_id, @status, @error_kind, @provider_error_code, @http_status, @provider_request_id,
    @request, @response, @input_tokens, @cached_input_tokens, @cache_write_tokens, @output_tokens,
    @reasoning_tokens, @usage_estimated, @cost_micros, @latency_ms, @first_token_ms,
    @started_at, @completed_at, @now
)
RETURNING *;

-- name: ListRunSteps :many
-- ListRunSteps returns a run's steps in order.
SELECT * FROM ai_run_steps
WHERE workspace_id = @workspace_id AND run_id = @run_id
ORDER BY seq;

-- name: NullRunStepPayloads :exec
-- NullRunStepPayloads drops the request and response payloads of one
-- run's steps, keeping every number.
UPDATE ai_run_steps SET request = NULL, response = NULL
WHERE run_id = @run_id AND (request IS NOT NULL OR response IS NOT NULL);

-- name: NextRunStepSeq :one
-- NextRunStepSeq is the sequence number for the next step of a run.
SELECT COALESCE(max(seq), 0) + 1 FROM ai_run_steps WHERE run_id = @run_id;

-- name: EraseContactRunData :exec
-- EraseContactRunData drops payloads and queries for a contact's
-- conversations, keeping every number.
UPDATE ai_run_steps SET request = NULL, response = NULL
WHERE run_id IN (
    SELECT id FROM ai_runs WHERE conversation_id = ANY(@conversation_ids::uuid[])
);
UPDATE ai_runs SET query_original = NULL, query_rewritten = NULL
WHERE conversation_id = ANY(@conversation_ids::uuid[]);
