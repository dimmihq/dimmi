-- migrate: phase=expand
-- A routine's run keeps each step Operator finished while it works, so a
-- teammate who opens the run midway sees what it did so far.

-- +goose Up
ALTER TABLE operator_routine_runs ADD COLUMN steps jsonb NOT NULL DEFAULT '[]'
    CHECK (jsonb_typeof(steps) = 'array');

-- +goose Down
ALTER TABLE operator_routine_runs DROP COLUMN steps;
