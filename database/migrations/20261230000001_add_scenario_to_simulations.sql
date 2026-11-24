-- migrate: phase=expand
-- A saved simulation's scenario: who its customer is, what they want
-- and what they already know, in the team's words. The simulated
-- customer answers from it, and agrees only to what it says the
-- customer wants. Simulations saved before this keep an empty one and
-- run from their saved messages alone.

-- +goose Up
ALTER TABLE ai_agent_test_cases ADD COLUMN scenario text NOT NULL DEFAULT '';
ALTER TABLE ai_agent_test_cases
    ADD CONSTRAINT ai_agent_test_cases_scenario_check CHECK (char_length(scenario) <= 1000) NOT VALID;

ALTER TABLE procedure_test_cases ADD COLUMN scenario text NOT NULL DEFAULT '';
ALTER TABLE procedure_test_cases
    ADD CONSTRAINT procedure_test_cases_scenario_check CHECK (char_length(scenario) <= 1000) NOT VALID;

-- +goose Down
ALTER TABLE procedure_test_cases DROP CONSTRAINT procedure_test_cases_scenario_check;
ALTER TABLE procedure_test_cases DROP COLUMN scenario;
ALTER TABLE ai_agent_test_cases DROP CONSTRAINT ai_agent_test_cases_scenario_check;
ALTER TABLE ai_agent_test_cases DROP COLUMN scenario;
