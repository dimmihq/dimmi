-- migrate: phase=expand
-- Which version of the renderer made an entry's published HTML, so a
-- change to the rendering, such as highlighting code, reaches entries
-- published before it without anyone publishing them again.

-- +goose Up
ALTER TABLE changelog_entries ADD COLUMN published_renderer_version smallint NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE changelog_entries DROP COLUMN published_renderer_version;
