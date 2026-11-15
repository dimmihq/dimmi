-- migrate: phase=expand
-- What the agent remembers of a long conversation: a short summary of
-- the turns folded out of its window, the last message the summary
-- covers, and the help-center passages it cited, which an agent that
-- carries passages reads again instead of searching for them. An agent
-- starts without carrying passages until a benchmark shows answers hold.

-- +goose Up
ALTER TABLE conversations ADD COLUMN ai_memory jsonb;
ALTER TABLE ai_agents ADD COLUMN carry_passages boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE ai_agents DROP COLUMN carry_passages;
ALTER TABLE conversations DROP COLUMN ai_memory;
