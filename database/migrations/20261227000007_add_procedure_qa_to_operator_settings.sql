-- migrate: phase=expand
-- Whether Operator reviews the procedures' recent runs once a week and
-- proposes fixes and tests for what went wrong. Off for every workspace
-- until the team turns it on; it spends from the monthly AI budget.

-- +goose Up
ALTER TABLE operator_settings ADD COLUMN procedure_qa_weekly boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE operator_settings DROP COLUMN procedure_qa_weekly;
