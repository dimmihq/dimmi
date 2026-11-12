-- migrate: phase=expand
-- A conversation brought in from another tool keeps the id it had there
-- in its source, so sending the same history twice finds the first copy
-- instead of making a second one. The id is unique within a workspace.

-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS conversations_workspace_id_import_external_id_key
    ON conversations (workspace_id, (source ->> 'external_id'))
    WHERE source ? 'external_id';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS conversations_workspace_id_import_external_id_key;
