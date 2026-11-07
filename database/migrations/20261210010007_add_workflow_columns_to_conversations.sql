-- migrate: phase=expand
-- A conversation a customer-facing workflow holds points at that run, so
-- other workflows and the automatic agent keep out of it. ai_mode stays
-- 'workflow' after the run ends, until the conversation closes: once a
-- workflow routed a conversation to a person, the automatic agent must
-- not jump back in on the next message.

-- +goose Up
ALTER TABLE conversations
    ADD COLUMN workflow_run_id uuid REFERENCES workflow_runs ON DELETE SET NULL,
    ADD COLUMN ai_mode text NOT NULL DEFAULT 'auto' CHECK (ai_mode IN ('auto', 'workflow'));

-- +goose Down
ALTER TABLE conversations DROP COLUMN workflow_run_id, DROP COLUMN ai_mode;
