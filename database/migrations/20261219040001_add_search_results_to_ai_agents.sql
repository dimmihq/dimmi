-- migrate: phase=expand
-- How many passages one search hands the agent's model. Every agent
-- keeps today's five; fewer cost less on every later call of the turn.

-- +goose Up
ALTER TABLE ai_agents ADD COLUMN search_results smallint NOT NULL DEFAULT 5
    CHECK (search_results BETWEEN 1 AND 8);

-- +goose Down
ALTER TABLE ai_agents DROP COLUMN search_results;
