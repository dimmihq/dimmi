-- migrate: phase=expand
-- The side a conversation took at each condition its procedure words in
-- plain language, by step id: "then" or "else". Code cannot work such a
-- condition out from data, so the side is decided once from the
-- conversation and kept, and the steps of the other side stay off the
-- path on every later turn.

-- +goose Up
ALTER TABLE conversation_procedure_states ADD COLUMN branches jsonb NOT NULL DEFAULT '{}'
    CHECK (octet_length(branches::text) <= 8192);

-- +goose Down
ALTER TABLE conversation_procedure_states DROP COLUMN branches;
