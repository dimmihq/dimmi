-- migrate: phase=expand
-- An agent in a routine's run reads a batch and searches around it in a
-- few steps, which takes more tool calls than the first limit allowed.
-- Routines still on that first limit move to the new one; a limit a
-- team set itself stays.

-- +goose Up
ALTER TABLE operator_routines ALTER COLUMN max_tool_calls SET DEFAULT 75;
UPDATE operator_routines SET max_tool_calls = 75 WHERE max_tool_calls = 50;

-- +goose Down
UPDATE operator_routines SET max_tool_calls = 50 WHERE max_tool_calls = 75;
ALTER TABLE operator_routines ALTER COLUMN max_tool_calls SET DEFAULT 50;
