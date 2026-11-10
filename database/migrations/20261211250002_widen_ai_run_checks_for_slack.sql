-- migrate: phase=contract
-- Runs gain the cheap verdicts a chat app asks for, request detection
-- and the draft judge, the drafted outcome of an answer a teammate sends,
-- and the feedback teammates and customers give from Slack. Every check
-- only widens: each row the old release writes still passes it. Each
-- wider check is added unvalidated and validated after, so writers are
-- never blocked while existing rows are read.

-- +goose Up
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_kind_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_kind_check
    CHECK (kind IN ('conversation', 'playground', 'probe', 'help_center', 'task', 'classify', 'judge')) NOT VALID;
ALTER TABLE ai_runs VALIDATE CONSTRAINT ai_runs_kind_check;

ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_outcome_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_outcome_check
    CHECK (outcome IN ('answered', 'unknown', 'handoff', 'silent', 'greeted', 'refused', 'skipped', 'superseded',
        'error', 'drafted')) NOT VALID;
ALTER TABLE ai_runs VALIDATE CONSTRAINT ai_runs_outcome_check;

ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_feedback_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_feedback_check
    CHECK (feedback IN ('helped', 'not_helped', 'solved', 'handoff', 'discarded', 'good', 'bad')) NOT VALID;
ALTER TABLE ai_runs VALIDATE CONSTRAINT ai_runs_feedback_check;

-- +goose Down
DELETE FROM ai_runs WHERE kind IN ('classify', 'judge');
UPDATE ai_runs SET outcome = 'skipped' WHERE outcome = 'drafted';
UPDATE ai_runs SET feedback = NULL WHERE feedback IN ('solved', 'handoff', 'discarded', 'good', 'bad');
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_feedback_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_feedback_check CHECK (feedback IN ('helped', 'not_helped'));
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_outcome_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_outcome_check
    CHECK (outcome IN ('answered', 'unknown', 'handoff', 'silent', 'greeted', 'refused', 'skipped', 'superseded',
        'error'));
ALTER TABLE ai_runs DROP CONSTRAINT ai_runs_kind_check;
ALTER TABLE ai_runs ADD CONSTRAINT ai_runs_kind_check
    CHECK (kind IN ('conversation', 'playground', 'probe', 'help_center', 'task'));
