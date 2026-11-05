-- migrate: phase=expand
-- A reply on a channel that cannot take a message back waits a few
-- seconds before it leaves, so its writer can undo it. The wait ends at
-- scheduled_send_at; an undone reply keeps its row, emptied, for the
-- records that point at it, and is shown nowhere. Each member may choose
-- their own wait instead of the workspace's.

-- +goose Up
ALTER TABLE messages ADD COLUMN IF NOT EXISTS scheduled_send_at timestamptz;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS undone_at timestamptz;
ALTER TABLE workspace_members ADD COLUMN IF NOT EXISTS undo_send_seconds smallint
    CHECK (undo_send_seconds IN (0, 5, 10, 20, 30));

-- +goose Down
ALTER TABLE workspace_members DROP COLUMN IF EXISTS undo_send_seconds;
ALTER TABLE messages DROP COLUMN IF EXISTS undone_at;
ALTER TABLE messages DROP COLUMN IF EXISTS scheduled_send_at;
