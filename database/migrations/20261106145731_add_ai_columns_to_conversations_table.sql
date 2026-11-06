-- migrate: phase=expand
-- The agent's marks on a conversation: how many answers it gave, what
-- they cost, the customer's language, and when and why it handed over.

-- +goose Up
ALTER TABLE conversations
    ADD COLUMN ai_reply_count integer NOT NULL DEFAULT 0,
    ADD COLUMN ai_cost_micros bigint NOT NULL DEFAULT 0,
    ADD COLUMN ai_language text,
    ADD COLUMN ai_handed_off_at timestamptz,
    ADD COLUMN ai_handoff_reason text;

-- +goose Down
ALTER TABLE conversations
    DROP COLUMN ai_handoff_reason,
    DROP COLUMN ai_handed_off_at,
    DROP COLUMN ai_language,
    DROP COLUMN ai_cost_micros,
    DROP COLUMN ai_reply_count;
