-- migrate: phase=expand
-- The purge of unused uploads finds the files conversation attributes
-- still hold by this index.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS conversation_ai_values_attachment_ids_index
    ON conversation_ai_values USING gin (attachment_ids);

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS conversation_ai_values_attachment_ids_index;
