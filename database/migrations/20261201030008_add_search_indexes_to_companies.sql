-- migrate: phase=expand
-- Finding a company by a name with a typo, or by what its attributes say.

-- +goose NO TRANSACTION
-- +goose Up
CREATE INDEX CONCURRENTLY companies_name_trgm_idx
    ON companies USING gin (workspace_id, dimmi_unaccent(lower(name)) gin_trgm_ops);
CREATE INDEX CONCURRENTLY companies_custom_attributes_idx
    ON companies USING gin (workspace_id, custom_attributes jsonb_path_ops);

-- +goose Down
DROP INDEX CONCURRENTLY companies_custom_attributes_idx;
DROP INDEX CONCURRENTLY companies_name_trgm_idx;
