-- migrate: phase=expand
-- What a member may see of the workspace's conversations. Everyone keeps
-- seeing everything until an admin says otherwise, and a member's own
-- assignments stay visible in every mode.

-- +goose Up
ALTER TABLE workspace_members
    ADD COLUMN conversation_access text NOT NULL DEFAULT 'all'
        CHECK (conversation_access IN ('all', 'assigned', 'teams', 'all_except_teams')),
    ADD COLUMN conversation_access_unassigned boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE workspace_members
    DROP COLUMN conversation_access_unassigned,
    DROP COLUMN conversation_access;
