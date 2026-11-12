-- migrate: phase=expand
-- A verified custom domain serves one help center, so no two live help
-- centers may claim the same one. Built concurrently, since help centers
-- are read on every public page.

-- +goose NO TRANSACTION
-- +goose Up
CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS help_centers_custom_domain_key ON help_centers (custom_domain)
    WHERE custom_domain IS NOT NULL AND deleted_at IS NULL;

-- +goose Down
DROP INDEX CONCURRENTLY IF EXISTS help_centers_custom_domain_key;
