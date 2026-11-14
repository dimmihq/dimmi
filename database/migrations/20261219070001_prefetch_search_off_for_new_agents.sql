-- migrate: phase=expand
-- Searching before the first model call cut the WixQA benchmark's cost
-- per answer by about a quarter but left a few more answers wrong than
-- the noise between identical runs explains, so new agents start with
-- it off until that is fixed. The switch stays per agent.

-- +goose Up
ALTER TABLE ai_agents ALTER COLUMN prefetch_search SET DEFAULT false;

-- +goose Down
ALTER TABLE ai_agents ALTER COLUMN prefetch_search SET DEFAULT true;
