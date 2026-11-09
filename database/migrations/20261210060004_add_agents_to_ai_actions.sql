-- migrate: phase=expand
-- Which of the workspace's agents may use each action: dimmi, which
-- answers customers, Operator, which works for the team, or both. An
-- action connected before this choice existed stays with dimmi.

-- +goose Up
ALTER TABLE ai_actions ADD COLUMN agents text[] NOT NULL DEFAULT '{dimmi}'
    CHECK (agents <@ ARRAY['dimmi', 'operator']::text[]);

-- +goose Down
ALTER TABLE ai_actions DROP COLUMN agents;
