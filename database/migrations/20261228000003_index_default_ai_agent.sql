-- migrate: phase=expand
-- A workspace may now have many agents but still exactly one default.
-- The new index holds that; the old one, one agent per workspace, goes.
-- Both are built and dropped without locking ai_agents.

-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS ai_agents_default_per_workspace_key
    ON ai_agents (workspace_id) WHERE is_default AND deleted_at IS NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS ai_agents_workspace_id_index
    ON ai_agents (workspace_id) WHERE deleted_at IS NULL;

DROP INDEX CONCURRENTLY IF EXISTS ai_agents_workspace_id_key;

-- +goose Down
DROP INDEX IF EXISTS ai_agents_workspace_id_index;
DROP INDEX IF EXISTS ai_agents_default_per_workspace_key;
CREATE UNIQUE INDEX ai_agents_workspace_id_key ON ai_agents (workspace_id) WHERE deleted_at IS NULL;
