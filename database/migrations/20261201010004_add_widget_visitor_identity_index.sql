-- migrate: phase=expand
-- Anonymous visitors are found by their session id, a lookup the messenger
-- does often enough to deserve an index of its own.

-- +goose Up
CREATE INDEX contact_identities_workspace_id_value_widget_visitor_index
    ON contact_identities (workspace_id, value) WHERE type = 'widget_visitor';

-- +goose Down
DROP INDEX contact_identities_workspace_id_value_widget_visitor_index;
