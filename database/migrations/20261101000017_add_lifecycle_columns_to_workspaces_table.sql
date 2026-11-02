-- migrate: phase=expand
-- A deleted workspace waits in the trash until purge_after, so an owner
-- can restore it; the index finds the ones due for purging. Who created
-- and who deleted it are kept for limits, restore and reminder mail.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE workspaces
    ADD COLUMN created_by_user_id uuid REFERENCES users ON DELETE SET NULL,
    ADD COLUMN deleted_by_user_id uuid REFERENCES users ON DELETE SET NULL,
    ADD COLUMN purge_after timestamptz,
    ADD COLUMN suspended_reason text;

CREATE INDEX CONCURRENTLY IF NOT EXISTS workspaces_purge_after_index
    ON workspaces (purge_after) WHERE deleted_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS workspaces_purge_after_index;
ALTER TABLE workspaces
    DROP COLUMN suspended_reason,
    DROP COLUMN purge_after,
    DROP COLUMN deleted_by_user_id,
    DROP COLUMN created_by_user_id;
