-- migrate: phase=contract
-- Runs gain the procedure simulator's turns, which go through the agent's
-- whole pipeline with mocked actions and count toward the monthly cap
-- like any other run. The check only widens, added unvalidated and
-- validated after.

-- +goose Up
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_kind_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_kind_check
    CHECK (kind IN ('conversation', 'playground', 'probe', 'help_center', 'task', 'classify', 'judge',
        'ticket_fill', 'ticket_match', 'ticket_summary', 'simulation')) NOT VALID;
ALTER TABLE ai_runs VALIDATE CONSTRAINT ai_runs_kind_check;

-- +goose Down
DELETE FROM ai_runs WHERE kind = 'simulation';
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_kind_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_kind_check
    CHECK (kind IN ('conversation', 'playground', 'probe', 'help_center', 'task', 'classify', 'judge',
        'ticket_fill', 'ticket_match', 'ticket_summary'));
