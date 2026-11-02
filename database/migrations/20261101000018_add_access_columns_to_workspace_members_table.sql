-- migrate: phase=expand
-- last_accessed_at orders a user's workspaces so the one they used last
-- opens first. That list is read by user, through a security definer
-- function, so its index leads with the user instead of the workspace.
-- joined_via records how the member arrived, a registry value.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE workspace_members
    ADD COLUMN last_accessed_at timestamptz,
    ADD COLUMN deactivated_at timestamptz,
    ADD COLUMN deactivated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    ADD COLUMN joined_via text CHECK (joined_via ~ '^[a-z][a-z0-9_]{1,39}$');

CREATE INDEX CONCURRENTLY IF NOT EXISTS workspace_members_user_id_last_accessed_at_index
    ON workspace_members (user_id, last_accessed_at DESC);

-- +goose Down
DROP INDEX IF EXISTS workspace_members_user_id_last_accessed_at_index;
ALTER TABLE workspace_members
    DROP COLUMN joined_via,
    DROP COLUMN deactivated_by_member_id,
    DROP COLUMN deactivated_at,
    DROP COLUMN last_accessed_at;
