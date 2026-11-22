-- migrate: phase=expand
-- The moment a saved simulation runs at. A procedure can turn on dates,
-- such as a refund window or how old a scan is, so a simulation keeps
-- the day its chat happened and every run sees that day, not the real
-- one. Empty runs at the real time.

-- +goose Up
ALTER TABLE procedure_test_cases ADD COLUMN simulated_at timestamptz;

-- +goose Down
ALTER TABLE procedure_test_cases DROP COLUMN simulated_at;
