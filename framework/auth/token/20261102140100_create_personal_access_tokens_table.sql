-- migrate: phase=expand
-- lint:user-scoped
-- A token acts for its owner. When the owner works inside one tenant the
-- token is bound to it, but lookups happen before the tenant is known,
-- so the table is read across tenants by token digest.

-- +goose Up
CREATE TABLE personal_access_tokens (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL,
    tenant_id text,
    name text NOT NULL,
    hint text NOT NULL,
    token_hash bytea NOT NULL UNIQUE,
    abilities text[] NOT NULL DEFAULT '{}',
    expires_at timestamptz,
    last_used_at timestamptz,
    last_used_ip text,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE INDEX personal_access_tokens_owner_id_created_at_index
    ON personal_access_tokens (owner_id, created_at DESC);

-- +goose Down
DROP TABLE personal_access_tokens;
