-- migrate: phase=contract
-- Messages are searched through the accent-folding configuration, and the
-- index leads with the workspace so one tenant's common words cost another
-- tenant nothing. Generated columns cannot change their expression, so the
-- column is replaced; no release has written to it yet.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE messages DROP COLUMN search;
ALTER TABLE messages
    ADD COLUMN search tsvector GENERATED ALWAYS AS (to_tsvector('dimmi_search', coalesce(body, ''))) STORED;
CREATE INDEX CONCURRENTLY messages_workspace_id_search_index ON messages USING gin (workspace_id, search);

-- +goose Down
ALTER TABLE messages DROP COLUMN search;
ALTER TABLE messages
    ADD COLUMN search tsvector GENERATED ALWAYS AS (to_tsvector('english', coalesce(body, ''))) STORED;
CREATE INDEX CONCURRENTLY messages_search_index ON messages USING gin (search);
