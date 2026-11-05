-- migrate: phase=expand
-- Replies waiting out their undo window are found by when they leave.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY messages_scheduled_send_at_index ON messages (scheduled_send_at)
    WHERE scheduled_send_at IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY messages_scheduled_send_at_index;
