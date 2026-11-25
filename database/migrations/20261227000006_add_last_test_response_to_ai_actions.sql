-- migrate: phase=expand
-- The result of an action's last successful live test, as the model would
-- read it. The procedure editor offers its fields in rules, and the
-- simulator answers with it when a test gives no mock of its own.

-- +goose Up
ALTER TABLE ai_actions ADD COLUMN last_test_response jsonb
    CHECK (octet_length(last_test_response::text) <= 32768);

-- +goose Down
ALTER TABLE ai_actions DROP COLUMN last_test_response;
