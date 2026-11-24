-- migrate: phase=expand
-- A broadcast or series that lets AI answer replies can name the agent
-- that answers them. Without one, the agent placed on the channel the
-- reply comes in on answers, as for any other conversation.

-- +goose Up
ALTER TABLE broadcasts ADD COLUMN reply_agent_id uuid REFERENCES ai_agents ON DELETE SET NULL;
ALTER TABLE series ADD COLUMN reply_agent_id uuid REFERENCES ai_agents ON DELETE SET NULL;

-- +goose Down
ALTER TABLE series DROP COLUMN reply_agent_id;
ALTER TABLE broadcasts DROP COLUMN reply_agent_id;
