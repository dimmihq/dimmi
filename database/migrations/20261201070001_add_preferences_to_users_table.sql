-- migrate: phase=expand
-- lint:global
-- How the app behaves for one person, wherever they work: small choices
-- such as whether Enter sends a reply. Only keys the person changed are
-- stored; every other key reads as its default, so adding a preference
-- never needs a backfill.

-- +goose Up
ALTER TABLE users ADD COLUMN preferences jsonb NOT NULL DEFAULT '{}';

-- +goose Down
ALTER TABLE users DROP COLUMN preferences;
