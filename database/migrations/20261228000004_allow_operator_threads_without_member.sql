-- migrate: phase=contract
-- A routine's thread belongs to no one teammate: every teammate who may
-- read what the routine reads opens it. So a thread names a teammate or
-- a routine, and the teammate becomes optional.

-- +goose Up
ALTER TABLE operator_threads ALTER COLUMN member_id DROP NOT NULL;
ALTER TABLE operator_threads
    ADD CONSTRAINT operator_threads_owner_check CHECK (member_id IS NOT NULL OR routine_id IS NOT NULL);

-- +goose Down
DELETE FROM operator_threads WHERE member_id IS NULL;
ALTER TABLE operator_threads DROP CONSTRAINT operator_threads_owner_check;
ALTER TABLE operator_threads ALTER COLUMN member_id SET NOT NULL;
