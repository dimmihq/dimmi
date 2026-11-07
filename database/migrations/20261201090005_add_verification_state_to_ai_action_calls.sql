-- migrate: phase=contract
-- An agent action bound to the customer's email can now wait for an
-- anonymous messenger visitor to prove that address with a code before
-- it binds anything. The state list only widens, so every row the
-- previous release writes still passes. The new check is added
-- unvalidated and validated after, so writers are never blocked. Going
-- down restores the narrow check unvalidated, keeping such rows.

-- +goose Up
ALTER TABLE ai_action_calls DROP CONSTRAINT ai_action_calls_state_check;
ALTER TABLE ai_action_calls ADD CONSTRAINT ai_action_calls_state_check CHECK (state IN (
    'awaiting_verification', 'awaiting_confirmation', 'pending_approval', 'approved', 'declined', 'expired',
    'canceled', 'running', 'succeeded', 'failed'
)) NOT VALID;
ALTER TABLE ai_action_calls VALIDATE CONSTRAINT ai_action_calls_state_check;

-- +goose Down
ALTER TABLE ai_action_calls DROP CONSTRAINT ai_action_calls_state_check;
ALTER TABLE ai_action_calls ADD CONSTRAINT ai_action_calls_state_check CHECK (state IN (
    'awaiting_confirmation', 'pending_approval', 'approved', 'declined', 'expired', 'canceled', 'running',
    'succeeded', 'failed'
)) NOT VALID;
