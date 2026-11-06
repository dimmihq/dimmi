-- migrate: phase=expand
-- One run of the agent's machinery: who triggered it, what it decided,
-- what it read and wrote, and what it cost, so every reply can be
-- traced and every cent accounted.

-- +goose Up
CREATE TABLE ai_runs (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    agent_id uuid NOT NULL REFERENCES ai_agents,
    kind text NOT NULL CHECK (kind IN ('conversation', 'playground', 'probe', 'help_center', 'task')),
    conversation_id uuid REFERENCES conversations ON DELETE CASCADE,
    trigger_message_id uuid REFERENCES messages ON DELETE SET NULL,
    included_message_ids uuid[] NOT NULL DEFAULT '{}',
    actor_type text,
    actor_id uuid,
    channel_type text,
    status text NOT NULL CHECK (status IN ('running', 'completed', 'failed', 'skipped')),
    outcome text CHECK (outcome IN ('answered', 'unknown', 'handoff', 'silent', 'greeted', 'refused', 'skipped', 'superseded', 'error')),
    skip_reason text,
    handoff_reason text,
    intent text,
    language text,
    query_original text,
    query_rewritten text,
    retrieval jsonb NOT NULL DEFAULT '[]',
    answer_message_id uuid REFERENCES messages ON DELETE SET NULL,
    prompt_version text NOT NULL DEFAULT '',
    prompt_hashes jsonb NOT NULL DEFAULT '{}',
    config_snapshot jsonb NOT NULL DEFAULT '{}',
    flags text[] NOT NULL DEFAULT '{}',
    input_tokens bigint NOT NULL DEFAULT 0,
    cached_input_tokens bigint NOT NULL DEFAULT 0,
    cache_write_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    cost_micros bigint NOT NULL DEFAULT 0,
    price_table_version text,
    first_token_ms integer,
    latency_ms integer,
    queue_wait_ms integer,
    error_code text,
    error_message text,
    feedback text CHECK (feedback IN ('helped', 'not_helped')),
    action_calls_count integer NOT NULL DEFAULT 0,
    started_at timestamptz,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ai_runs_workspace_id_conversation_id_created_at_index
    ON ai_runs (workspace_id, conversation_id, created_at);
CREATE INDEX ai_runs_workspace_id_created_at_index
    ON ai_runs (workspace_id, created_at DESC);
CREATE INDEX ai_runs_workspace_id_kind_outcome_created_at_index
    ON ai_runs (workspace_id, kind, outcome, created_at DESC);
CREATE INDEX ai_runs_workspace_id_trigger_message_id_index
    ON ai_runs (workspace_id, trigger_message_id);

-- +goose Down
DROP TABLE ai_runs;
