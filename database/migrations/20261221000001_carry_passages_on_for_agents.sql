-- migrate: phase=expand
-- An agent with this on keeps the knowledge its replies cited in front
-- of the model on later turns. Measured on Haiku 5.5 over the same 52
-- conversations, it searched a third less often and cost about a sixth
-- less, with the same accuracy; on GPT-6 Luna it changed nothing. It
-- was off for every agent because only a benchmark command could set
-- it. Every agent now has it on, and new agents start with it on.

-- +goose Up
ALTER TABLE ai_agents ALTER COLUMN carry_passages SET DEFAULT true;

UPDATE ai_agents SET carry_passages = true WHERE NOT carry_passages;

-- +goose Down
ALTER TABLE ai_agents ALTER COLUMN carry_passages SET DEFAULT false;

UPDATE ai_agents SET carry_passages = false WHERE carry_passages;
