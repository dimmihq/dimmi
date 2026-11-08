-- migrate: phase=expand
-- Who sees a messenger's launcher, who may start a conversation, and who
-- sees each tab, as audience rules. Absent means everyone. Rules are
-- decided on the server; a browser only ever gets a yes, a no, or the
-- page conditions left to check on each page.

-- +goose Up
ALTER TABLE widget_settings ADD COLUMN audiences jsonb NOT NULL DEFAULT '{}'
    CHECK (octet_length(audiences::text) <= 32768);

-- +goose Down
ALTER TABLE widget_settings DROP COLUMN audiences;
