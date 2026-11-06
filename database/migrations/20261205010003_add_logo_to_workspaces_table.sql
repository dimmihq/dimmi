-- migrate: phase=expand
-- A workspace may have a logo, shown to customers in the messenger and
-- in mail and to the team in the app. Like a person's picture it is a
-- small square kept on the default disk under a random name, and the
-- column holds that path.

-- +goose Up
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS logo_path text;

-- +goose Down
ALTER TABLE workspaces DROP COLUMN IF EXISTS logo_path;
