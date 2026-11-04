-- migrate: phase=expand
-- Whether the loader keeps an old Intercom snippet's calls working while a
-- customer moves over.

-- +goose Up
ALTER TABLE widget_settings ADD COLUMN intercom_shim boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE widget_settings DROP COLUMN intercom_shim;
