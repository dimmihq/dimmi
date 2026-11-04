-- migrate: phase=expand
-- Whether the messenger shows its What's new tab.

-- +goose Up
ALTER TABLE widget_settings ADD COLUMN changelog_tab_enabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE widget_settings DROP COLUMN changelog_tab_enabled;
