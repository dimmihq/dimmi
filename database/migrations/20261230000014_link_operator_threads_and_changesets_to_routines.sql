-- migrate: phase=expand
-- A routine's run opens a thread of its own, and the suggestions it
-- leaves belong to the routine rather than to a teammate. dedup_key lets
-- a run see it suggested something before; a preview is a dry run's
-- suggestion, shown on its thread and never applied; expires_at is when
-- a routine's suggestion counts as stale.

-- +goose Up
ALTER TABLE operator_threads
    ADD COLUMN routine_id uuid REFERENCES operator_routines ON DELETE SET NULL,
    ADD COLUMN routine_run_id uuid REFERENCES operator_routine_runs ON DELETE SET NULL;

ALTER TABLE operator_changesets
    ADD COLUMN routine_id uuid REFERENCES operator_routines ON DELETE SET NULL,
    ADD COLUMN routine_run_id uuid REFERENCES operator_routine_runs ON DELETE SET NULL,
    ADD COLUMN dedup_key text CHECK (char_length(dedup_key) BETWEEN 1 AND 200),
    ADD COLUMN preview boolean NOT NULL DEFAULT false,
    ADD COLUMN expires_at timestamptz;

-- +goose Down
ALTER TABLE operator_changesets
    DROP COLUMN expires_at, DROP COLUMN preview, DROP COLUMN dedup_key,
    DROP COLUMN routine_run_id, DROP COLUMN routine_id;
ALTER TABLE operator_threads DROP COLUMN routine_run_id, DROP COLUMN routine_id;
