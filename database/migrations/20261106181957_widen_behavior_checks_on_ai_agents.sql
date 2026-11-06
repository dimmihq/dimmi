-- migrate: phase=contract
-- A workspace may let the agent keep answering while a teammate holds
-- the conversation (assigned_behavior always), and may have it simply
-- say it does not know without offering a teammate (unknown_behavior
-- say_unknown). Both checks only widen: every row the old release
-- writes still passes them. Each wider check is added unvalidated and
-- validated after, so writers are never blocked while existing rows are
-- read.

-- +goose Up
ALTER TABLE ai_agents DROP CONSTRAINT ai_agents_assigned_behavior_check;
ALTER TABLE ai_agents ADD CONSTRAINT ai_agents_assigned_behavior_check
    CHECK (assigned_behavior IN ('skip_when_human_active', 'never_when_assigned', 'always')) NOT VALID;
ALTER TABLE ai_agents VALIDATE CONSTRAINT ai_agents_assigned_behavior_check;

ALTER TABLE ai_agents DROP CONSTRAINT ai_agents_unknown_behavior_check;
ALTER TABLE ai_agents ADD CONSTRAINT ai_agents_unknown_behavior_check
    CHECK (unknown_behavior IN ('offer_handoff', 'handoff', 'say_unknown')) NOT VALID;
ALTER TABLE ai_agents VALIDATE CONSTRAINT ai_agents_unknown_behavior_check;

-- +goose Down
UPDATE ai_agents SET unknown_behavior = 'offer_handoff' WHERE unknown_behavior = 'say_unknown';
ALTER TABLE ai_agents DROP CONSTRAINT ai_agents_unknown_behavior_check;
ALTER TABLE ai_agents ADD CONSTRAINT ai_agents_unknown_behavior_check
    CHECK (unknown_behavior IN ('offer_handoff', 'handoff'));

UPDATE ai_agents SET assigned_behavior = 'skip_when_human_active' WHERE assigned_behavior = 'always';
ALTER TABLE ai_agents DROP CONSTRAINT ai_agents_assigned_behavior_check;
ALTER TABLE ai_agents ADD CONSTRAINT ai_agents_assigned_behavior_check
    CHECK (assigned_behavior IN ('skip_when_human_active', 'never_when_assigned'));
