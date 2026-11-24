-- migrate: phase=expand
-- Reports narrowed to one AI agent read its facts by start date. The
-- index is built without locking the facts.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS conversation_facts_workspace_id_ai_agent_id_started_at_index
    ON conversation_facts (workspace_id, ai_agent_id, started_at) WHERE ai_agent_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS conversation_facts_workspace_id_ai_agent_id_started_at_index;
