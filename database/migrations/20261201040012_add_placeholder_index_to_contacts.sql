-- migrate: phase=expand
-- A workspace has one placeholder contact, the stand-in for every
-- customer it erased.

-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY contacts_workspace_id_placeholder_key ON contacts (workspace_id) WHERE placeholder;

-- +goose Down
DROP INDEX CONCURRENTLY contacts_workspace_id_placeholder_key;
