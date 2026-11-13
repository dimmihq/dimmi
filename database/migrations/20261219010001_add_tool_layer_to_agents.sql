-- migrate: phase=expand
-- The tool layer offers the model a few tools and loads the rest on
-- request, so each call carries fewer tool definitions. Agents and
-- Operator settings that exist keep every tool offered as before; rows
-- created from now on start with the layer on. A run keeps what the
-- layer did: the tools it loaded, so the next turn can name them, and
-- its counts.

-- +goose Up
ALTER TABLE ai_agents ADD COLUMN tool_layer boolean NOT NULL DEFAULT false;
ALTER TABLE ai_agents ALTER COLUMN tool_layer SET DEFAULT true;

ALTER TABLE operator_settings ADD COLUMN tool_layer boolean NOT NULL DEFAULT false;
ALTER TABLE operator_settings ALTER COLUMN tool_layer SET DEFAULT true;

ALTER TABLE ai_runs ADD COLUMN tool_layer jsonb;

-- +goose Down
ALTER TABLE ai_runs DROP COLUMN tool_layer;
ALTER TABLE operator_settings DROP COLUMN tool_layer;
ALTER TABLE ai_agents DROP COLUMN tool_layer;
