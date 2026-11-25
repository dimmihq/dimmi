-- migrate: phase=expand
-- What an AI agent handles, in a sentence or two the team writes. Every
-- other agent that may pass it a conversation reads this in its own
-- prompt, so it knows when a request is this agent's job. Agents made
-- before this keep an empty one until the team writes it.

-- +goose Up
ALTER TABLE ai_agents ADD COLUMN description text NOT NULL DEFAULT '';
ALTER TABLE ai_agents
    ADD CONSTRAINT ai_agents_description_check CHECK (char_length(description) <= 400) NOT VALID;

-- +goose Down
ALTER TABLE ai_agents DROP CONSTRAINT ai_agents_description_check;
ALTER TABLE ai_agents DROP COLUMN description;
