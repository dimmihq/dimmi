-- migrate: phase=expand
-- A dashboard preview's conversations are marked as tests and never billed,
-- and a message past the reopen window starts a conversation that remembers
-- the one it continues. The shared inbox's own migration adds the same
-- columns, so both tolerate the other having run first.

-- +goose Up
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS is_test boolean NOT NULL DEFAULT false;
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS previous_conversation_id uuid
    REFERENCES conversations ON DELETE SET NULL;

-- +goose Down
ALTER TABLE conversations DROP COLUMN IF EXISTS previous_conversation_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS is_test;
