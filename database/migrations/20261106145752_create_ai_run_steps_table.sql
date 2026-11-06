-- migrate: phase=expand
-- Every stage of a run: what was asked of which provider, what came
-- back, and what it cost. Payloads are redacted and nulled at
-- retention; the numbers stay with the run.

-- +goose Up
CREATE TABLE ai_run_steps (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    run_id uuid NOT NULL REFERENCES ai_runs ON DELETE CASCADE,
    seq integer NOT NULL,
    kind text NOT NULL CHECK (kind IN ('classify', 'retrieve', 'generate', 'tool_call', 'verify', 'summarize', 'translate', 'probe')),
    provider text,
    provider_key_id uuid REFERENCES ai_provider_keys ON DELETE SET NULL,
    model text,
    response_model text,
    attempt integer NOT NULL DEFAULT 1,
    retry_of_step_id uuid,
    status text NOT NULL CHECK (status IN ('ok', 'error', 'canceled')),
    error_kind text,
    provider_error_code text,
    http_status integer,
    provider_request_id text,
    request jsonb,
    response jsonb,
    input_tokens bigint NOT NULL DEFAULT 0,
    cached_input_tokens bigint NOT NULL DEFAULT 0,
    cache_write_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    reasoning_tokens bigint NOT NULL DEFAULT 0,
    usage_estimated boolean NOT NULL DEFAULT false,
    cost_micros bigint NOT NULL DEFAULT 0,
    latency_ms integer,
    first_token_ms integer,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ai_run_steps_run_id_seq_key ON ai_run_steps (run_id, seq);
CREATE INDEX ai_run_steps_workspace_id_run_id_seq_index
    ON ai_run_steps (workspace_id, run_id, seq);

-- +goose Down
DROP TABLE ai_run_steps;
