-- migrate: phase=expand
-- lint:global
-- Data keys are kept as Tink keysets, one per owner and purpose, each
-- wrapped by the key encryption key, never in the clear. A keyset holds
-- every key it ever had, so rotating adds a key and old values still
-- open. Rows have no foreign key on purpose: deleting one is how the
-- values it sealed are destroyed for good, after their owner is gone.

-- +goose Up
CREATE TABLE encryption_keys (
    scope_kind text NOT NULL CHECK (scope_kind IN ('system', 'organization', 'workspace')),
    scope_id text NOT NULL,
    purpose text NOT NULL CHECK (purpose IN ('aead', 'stream', 'mac')),
    wrapped bytea NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope_kind, scope_id, purpose)
);

-- +goose Down
DROP TABLE encryption_keys;
