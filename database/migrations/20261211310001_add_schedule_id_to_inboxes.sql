-- migrate: phase=expand
-- Each inbox keeps its own office hours: the schedule it names, or the
-- workspace's default schedule when it names none. Schedules are only
-- ever soft deleted, so the application refuses to delete one an inbox
-- still names; the key only keeps a removed row from leaving a dangling
-- id behind.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE inboxes ADD COLUMN IF NOT EXISTS schedule_id uuid;
ALTER TABLE inboxes ADD CONSTRAINT inboxes_schedule_id_fkey
    FOREIGN KEY (schedule_id) REFERENCES schedules ON DELETE SET NULL NOT VALID;
ALTER TABLE inboxes VALIDATE CONSTRAINT inboxes_schedule_id_fkey;

CREATE INDEX CONCURRENTLY IF NOT EXISTS inboxes_schedule_id_index
    ON inboxes (workspace_id, schedule_id) WHERE schedule_id IS NOT NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS inboxes_schedule_id_index;
ALTER TABLE inboxes DROP COLUMN IF EXISTS schedule_id;
