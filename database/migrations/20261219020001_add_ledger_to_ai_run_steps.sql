-- migrate: phase=expand
-- A model call's step keeps what its input was made of, part by part
-- in the order the provider read it, so its reported tokens can be
-- split across the fixed prefix, the history, the newest message and
-- the turn's tool results.

-- +goose Up
ALTER TABLE ai_run_steps ADD COLUMN ledger jsonb;

-- +goose Down
ALTER TABLE ai_run_steps DROP COLUMN ledger;
