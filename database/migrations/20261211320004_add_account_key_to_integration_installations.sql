-- migrate: phase=expand
-- One live installation per workspace, provider, app and external
-- account, so a second connection of the same account updates the first
-- instead of adding a duplicate. Built concurrently so writes to the
-- table never wait for it.

-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS integration_installations_workspace_account_key
    ON integration_installations (workspace_id, provider, external_app_id, external_account_id) NULLS NOT DISTINCT
    WHERE deleted_at IS NULL AND status <> 'disconnected' AND external_account_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS integration_installations_workspace_account_key;
