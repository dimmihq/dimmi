-- migrate: phase=expand
-- Which agent answers a conversation. A Slack channel, a channel
-- connection (a widget, an email address, a WhatsApp number) or an inbox
-- can name the agent that takes its new conversations; anything none of
-- them names goes to the workspace's default agent. Once an agent
-- answers a conversation it stays on it, unless it hands it to another
-- agent; the conversation remembers who it came from so it never goes
-- back and forth.

-- +goose Up
ALTER TABLE channel_rooms ADD COLUMN ai_agent_id uuid REFERENCES ai_agents ON DELETE SET NULL;
ALTER TABLE channel_connections ADD COLUMN ai_agent_id uuid REFERENCES ai_agents ON DELETE SET NULL;
ALTER TABLE inboxes ADD COLUMN ai_agent_id uuid REFERENCES ai_agents ON DELETE SET NULL;
ALTER TABLE conversations
    ADD COLUMN ai_agent_id uuid REFERENCES ai_agents ON DELETE SET NULL,
    ADD COLUMN ai_transferred_from_ids uuid[] NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE conversations DROP COLUMN ai_agent_id, DROP COLUMN ai_transferred_from_ids;
ALTER TABLE inboxes DROP COLUMN ai_agent_id;
ALTER TABLE channel_connections DROP COLUMN ai_agent_id;
ALTER TABLE channel_rooms DROP COLUMN ai_agent_id;
