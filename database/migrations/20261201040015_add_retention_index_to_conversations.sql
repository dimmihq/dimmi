-- migrate: phase=expand
-- The daily retention round finds a workspace's closed conversations by
-- when they were closed, without walking the open ones.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY conversations_retention_idx
    ON conversations (workspace_id, closed_at) WHERE status = 'closed' AND deleted_at IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY conversations_retention_idx;
