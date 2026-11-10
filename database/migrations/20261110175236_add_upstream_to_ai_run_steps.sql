-- migrate: phase=expand

-- +goose Up
ALTER TABLE ai_run_steps ADD COLUMN upstream text;

-- +goose Down
ALTER TABLE ai_run_steps DROP COLUMN upstream;
