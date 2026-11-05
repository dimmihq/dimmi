-- migrate: phase=expand
-- Why a conversation waits for an assignee and since when: nobody in
-- its pool is available, or everyone is at capacity. Waiting rows are
-- few, so a partial index keeps draining them in order cheap.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE conversations
    ADD COLUMN assignment_wait_reason text
        CHECK (assignment_wait_reason IN ('no_available_member', 'all_at_capacity')),
    ADD COLUMN assignment_waiting_since timestamptz;

CREATE INDEX CONCURRENTLY IF NOT EXISTS conversations_assignment_waiting_index
    ON conversations (workspace_id, inbox_id, assignment_waiting_since, id)
    WHERE assignment_wait_reason IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS conversations_assignment_waiting_index;
ALTER TABLE conversations
    DROP COLUMN assignment_waiting_since,
    DROP COLUMN assignment_wait_reason;
