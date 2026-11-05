-- migrate: phase=expand
-- A new workspace walks its owners through setting it up before the app
-- opens: the step they reached is kept so they resume on any device, and
-- the moment they finished opens the app. Workspaces that existed before
-- onboarding count as finished.

-- +goose Up
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS onboarding_step text NOT NULL DEFAULT 'workspace'
    CHECK (onboarding_step ~ '^[a-z][a-z_]{0,39}$');
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS onboarding_completed_at timestamptz;
UPDATE workspaces SET onboarding_completed_at = created_at WHERE onboarding_completed_at IS NULL;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN IF EXISTS onboarding_completed_at;
ALTER TABLE workspaces DROP COLUMN IF EXISTS onboarding_step;
