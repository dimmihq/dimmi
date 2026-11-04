-- migrate: phase=expand
-- Whether requests without an Origin header, which is what native apps send,
-- are accepted despite a list of allowed sites.

-- +goose Up
ALTER TABLE widget_settings ADD COLUMN native_apps_enabled boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE widget_settings DROP COLUMN native_apps_enabled;
