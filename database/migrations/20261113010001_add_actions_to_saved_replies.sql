-- migrate: phase=expand
-- A saved reply may also be a macro: besides its text it carries what
-- happens to the conversation when the reply goes out, such as closing
-- or snoozing it, assigning it, tagging it or setting its priority. An
-- empty object is a plain reply.

-- +goose Up
ALTER TABLE saved_replies ADD COLUMN IF NOT EXISTS actions jsonb NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE saved_replies DROP COLUMN IF EXISTS actions;
