-- migrate: phase=expand
-- A messenger's home screen can list a few links of the customer's own,
-- such as booking a call or reading the docs. Each is a title, an address
-- and an icon, kept in order as one list.

-- +goose Up
ALTER TABLE widget_settings ADD COLUMN IF NOT EXISTS home_links jsonb NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE widget_settings DROP COLUMN IF EXISTS home_links;
