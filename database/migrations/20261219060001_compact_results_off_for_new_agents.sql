-- migrate: phase=expand
-- Compact results saved the customer agent nothing on the WixQA
-- benchmark (its results are mostly passages, already plain text) and
-- scored lower, so new agents start with them off. Operator keeps them:
-- its tool results are records, where they cut cost the most.

-- +goose Up
ALTER TABLE ai_agents ALTER COLUMN compact_results SET DEFAULT false;

-- +goose Down
ALTER TABLE ai_agents ALTER COLUMN compact_results SET DEFAULT true;
