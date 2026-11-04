-- migrate: phase=expand
-- Anonymous visitors are found by their session id, a lookup the messenger
-- does often enough to deserve an index of its own. The index is built
-- without locking the table, which already holds every contact's identities.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY IF NOT EXISTS contact_identities_workspace_id_value_widget_visitor_index
    ON contact_identities (workspace_id, value) WHERE type = 'widget_visitor';

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS contact_identities_workspace_id_value_widget_visitor_index;
