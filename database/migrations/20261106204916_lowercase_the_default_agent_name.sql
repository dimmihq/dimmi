-- migrate: phase=expand
-- The brand is written in lowercase everywhere a customer or teammate
-- reads it, and the agent's name is what both see on its replies and
-- in the timeline. New agents start as "dimmi"; agents still carrying
-- the old default are renamed, while a name a workspace chose stays.

-- +goose Up
ALTER TABLE ai_agents ALTER COLUMN name SET DEFAULT 'dimmi';
UPDATE ai_agents SET name = 'dimmi' WHERE name = 'Dimmi';

-- +goose Down
ALTER TABLE ai_agents ALTER COLUMN name SET DEFAULT 'Dimmi';
