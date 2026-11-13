-- migrate: phase=contract
-- Runs gain the cheap verdicts tickets ask for: filling a ticket's
-- fields, matching a conversation to trackers, and summarizing what a
-- tracker's customers report. The check only widens, added unvalidated and validated after.

-- +goose Up
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_kind_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_kind_check
    CHECK (kind IN ('conversation', 'playground', 'probe', 'help_center', 'task', 'classify', 'judge',
        'ticket_fill', 'ticket_match', 'ticket_summary')) NOT VALID;
ALTER TABLE ai_runs VALIDATE CONSTRAINT ai_runs_kind_check;

-- +goose Down
DELETE FROM ai_runs WHERE kind IN ('ticket_fill', 'ticket_match', 'ticket_summary');
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_kind_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_kind_check
    CHECK (kind IN ('conversation', 'playground', 'probe', 'help_center', 'task', 'classify', 'judge'));
