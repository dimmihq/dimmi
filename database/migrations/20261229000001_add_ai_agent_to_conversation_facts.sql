-- migrate: phase=expand
-- Which AI agent a conversation's facts belong to: the agent holding it
-- when the facts were worked out, so after a transfer the agent that
-- finished it. Reports compare agents side by side and filter to one.
-- Facts already written take the conversation's agent now.

-- +goose Up
ALTER TABLE conversation_facts ADD COLUMN ai_agent_id uuid;

UPDATE conversation_facts f SET ai_agent_id = c.ai_agent_id
FROM conversations c
WHERE c.id = f.conversation_id AND c.ai_agent_id IS NOT NULL;

-- +goose Down
ALTER TABLE conversation_facts DROP COLUMN ai_agent_id;
