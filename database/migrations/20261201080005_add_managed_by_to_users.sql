-- migrate: phase=expand
-- lint:user-scoped
-- An account the company's identity provider created, or whose address
-- sits under a domain the company verified, is managed by that company:
-- its email stays put and it cannot be deleted while it belongs there.

-- +goose NO TRANSACTION
-- +goose Up
ALTER TABLE users ADD COLUMN managed_by_organization_id uuid REFERENCES organizations ON DELETE SET NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS users_managed_by_organization_id_index
    ON users (managed_by_organization_id) WHERE managed_by_organization_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS users_managed_by_organization_id_index;
ALTER TABLE users DROP COLUMN managed_by_organization_id;
