-- migrate: phase=expand
-- lint:global
-- Certificates, keys and ACME accounts are shared by every server and
-- belong to no tenant.

-- +goose Up
CREATE TABLE certmagic_storage (
    key text PRIMARY KEY,
    value bytea NOT NULL,
    modified timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE certmagic_storage;
