-- migrate: phase=expand
-- lint:global
-- A stored response belongs to the tenant of the request that made it,
-- or to no tenant when the request had none, so the tenant column
-- defaults to empty instead of being required.

-- +goose Up
CREATE TABLE idempotency_keys (
    tenant_id text NOT NULL DEFAULT '',
    scope text NOT NULL,
    key text NOT NULL,
    fingerprint text NOT NULL,
    status integer,
    header jsonb,
    body bytea,
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, scope, key)
);

CREATE INDEX idempotency_keys_expires_at_index ON idempotency_keys (expires_at);

-- +goose Down
DROP TABLE idempotency_keys;
