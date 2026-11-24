-- migrate: phase=expand
-- An agent's analytics and its deletion read the conversations it
-- answers. The index is built without locking conversations.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS conversations_workspace_id_ai_agent_id_index
    ON conversations (workspace_id, ai_agent_id) WHERE ai_agent_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS conversations_workspace_id_ai_agent_id_index;
