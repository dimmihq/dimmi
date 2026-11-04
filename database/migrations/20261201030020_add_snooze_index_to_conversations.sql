-- migrate: phase=expand
-- The snoozed list sorts by when each conversation wakes, and the job that
-- wakes them reads the same order.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY conversations_snooze_idx
    ON conversations (workspace_id, snoozed_until, id)
    WHERE status = 'snoozed' AND deleted_at IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY conversations_snooze_idx;
