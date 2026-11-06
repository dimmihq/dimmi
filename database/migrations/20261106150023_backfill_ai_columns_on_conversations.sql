-- migrate: phase=expand
-- The agent's marks on a conversation: how many answers it gave, what
-- they cost, the customer's language, and when and why it handed over.
-- Guarded, because an install may already carry the columns.

-- +goose Up
ALTER TABLE conversations
    ADD COLUMN IF NOT EXISTS ai_reply_count integer NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ai_cost_micros bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS ai_language text,
    ADD COLUMN IF NOT EXISTS ai_handed_off_at timestamptz,
    ADD COLUMN IF NOT EXISTS ai_handoff_reason text;

-- +goose Down
SELECT 1;
