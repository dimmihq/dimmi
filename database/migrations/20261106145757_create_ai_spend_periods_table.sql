-- migrate: phase=expand
-- One row per workspace per month is the running tally the cap check
-- reads, so the limit is one row lock, never a table scan.

-- +goose Up
CREATE TABLE ai_spend_periods (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    period_start date NOT NULL,
    cost_micros bigint NOT NULL DEFAULT 0,
    input_tokens bigint NOT NULL DEFAULT 0,
    cached_input_tokens bigint NOT NULL DEFAULT 0,
    output_tokens bigint NOT NULL DEFAULT 0,
    runs_count integer NOT NULL DEFAULT 0,
    answered_count integer NOT NULL DEFAULT 0,
    unknown_count integer NOT NULL DEFAULT 0,
    handoff_count integer NOT NULL DEFAULT 0,
    silent_count integer NOT NULL DEFAULT 0,
    thresholds_notified integer[] NOT NULL DEFAULT '{}',
    capped_at timestamptz,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, period_start)
);

-- +goose Down
DROP TABLE ai_spend_periods;
