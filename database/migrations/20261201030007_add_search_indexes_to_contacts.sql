-- migrate: phase=expand
-- Finding a customer by name, address or attribute from the command menu
-- and the directory, a typo allowed in the name.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY contacts_search_idx ON contacts USING gin (workspace_id, search);
CREATE INDEX CONCURRENTLY contacts_name_trgm_idx
    ON contacts USING gin (workspace_id, dimmi_unaccent(lower(coalesce(name, ''))) gin_trgm_ops);
CREATE INDEX CONCURRENTLY contacts_email_trgm_idx ON contacts USING gin (workspace_id, (email::text) gin_trgm_ops);
CREATE INDEX CONCURRENTLY contacts_last_seen_idx
    ON contacts (workspace_id, last_seen_at DESC, id DESC)
    WHERE deleted_at IS NULL AND merged_into_id IS NULL;
CREATE INDEX CONCURRENTLY contacts_custom_attributes_idx
    ON contacts USING gin (workspace_id, custom_attributes jsonb_path_ops);

-- +goose Down
DROP INDEX CONCURRENTLY contacts_custom_attributes_idx;
DROP INDEX CONCURRENTLY contacts_last_seen_idx;
DROP INDEX CONCURRENTLY contacts_email_trgm_idx;
DROP INDEX CONCURRENTLY contacts_name_trgm_idx;
DROP INDEX CONCURRENTLY contacts_search_idx;
