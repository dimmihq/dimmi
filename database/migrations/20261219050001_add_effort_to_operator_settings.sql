-- migrate: phase=expand
-- How hard Operator thinks on each model call. Every workspace keeps
-- medium; low thinks less and writes fewer reasoning tokens.

-- +goose Up
ALTER TABLE operator_settings ADD COLUMN effort text NOT NULL DEFAULT 'medium'
    CHECK (effort IN ('low', 'medium', 'high'));

-- +goose Down
ALTER TABLE operator_settings DROP COLUMN effort;
