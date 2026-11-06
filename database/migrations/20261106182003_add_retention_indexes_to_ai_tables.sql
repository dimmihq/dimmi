-- migrate: phase=expand
-- The nightly retention round finds the steps that still carry
-- payloads past their time, workspace by workspace, without scanning
-- every step; the inbox finds the conversations the agent is part of
-- the same way.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS ai_run_steps_workspace_id_created_at_payload_index
    ON ai_run_steps (workspace_id, created_at) WHERE request IS NOT NULL OR response IS NOT NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS ai_runs_workspace_id_kind_created_at_index
    ON ai_runs (workspace_id, kind, created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS conversations_workspace_id_ai_state_index
    ON conversations (workspace_id, ai_state) WHERE ai_state <> 'off';

-- +goose Down
DROP INDEX IF EXISTS conversations_workspace_id_ai_state_index;
DROP INDEX IF EXISTS ai_runs_workspace_id_kind_created_at_index;
DROP INDEX IF EXISTS ai_run_steps_workspace_id_created_at_payload_index;
