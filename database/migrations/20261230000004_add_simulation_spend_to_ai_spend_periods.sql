-- migrate: phase=expand
-- What a workspace spends rehearsing its agent is kept apart from what
-- the agent spends answering customers. The spend cap reads cost_micros
-- alone, so testing an agent can never stop it answering, and the two
-- can be read side by side: how many simulations ran in the month and
-- what they cost.

-- +goose Up
ALTER TABLE ai_spend_periods
    ADD COLUMN simulation_cost_micros bigint NOT NULL DEFAULT 0,
    ADD COLUMN simulation_runs_count integer NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE ai_spend_periods
    DROP COLUMN simulation_cost_micros,
    DROP COLUMN simulation_runs_count;
