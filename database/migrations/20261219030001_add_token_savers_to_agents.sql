-- migrate: phase=expand
-- Two more ways to spend fewer tokens, each with its own switch so its
-- saving can be measured: the agent searches the knowledge with the
-- customer's message before its first model call, and tool results are
-- written compactly. Rows that exist keep today's behavior; rows created
-- from now on start with both on.

-- +goose Up
ALTER TABLE ai_agents ADD COLUMN prefetch_search boolean NOT NULL DEFAULT false;
ALTER TABLE ai_agents ALTER COLUMN prefetch_search SET DEFAULT true;
ALTER TABLE ai_agents ADD COLUMN compact_results boolean NOT NULL DEFAULT false;
ALTER TABLE ai_agents ALTER COLUMN compact_results SET DEFAULT true;

ALTER TABLE operator_settings ADD COLUMN compact_results boolean NOT NULL DEFAULT false;
ALTER TABLE operator_settings ALTER COLUMN compact_results SET DEFAULT true;

-- +goose Down
ALTER TABLE operator_settings DROP COLUMN compact_results;
ALTER TABLE ai_agents DROP COLUMN compact_results;
ALTER TABLE ai_agents DROP COLUMN prefetch_search;
