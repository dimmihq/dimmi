-- migrate: phase=contract
-- A run can end by handing the conversation to another agent. The
-- outcome check only widens, added unvalidated and validated after, so
-- every row the old release writes still passes it.

-- +goose Up
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_outcome_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_outcome_check
    CHECK (outcome IN ('answered', 'unknown', 'handoff', 'silent', 'greeted', 'refused', 'skipped', 'superseded',
        'error', 'drafted', 'transferred')) NOT VALID;
ALTER TABLE ai_runs VALIDATE CONSTRAINT ai_runs_outcome_check;

-- +goose Down
UPDATE ai_runs SET outcome = 'skipped' WHERE outcome = 'transferred';
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_outcome_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_outcome_check
    CHECK (outcome IN ('answered', 'unknown', 'handoff', 'silent', 'greeted', 'refused', 'skipped', 'superseded',
        'error', 'drafted'));
