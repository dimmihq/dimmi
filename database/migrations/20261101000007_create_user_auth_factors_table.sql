-- migrate: phase=expand
-- lint:user-scoped
-- A factor is one way a user proves who they are beyond their password:
-- an authenticator app, a set of recovery codes, later a passkey. The
-- type is a registry value, so a new kind of factor is a new driver and
-- no migration. A user has at most one preferred factor and one live set
-- of recovery codes. Secrets are sealed with the system key; the rest is
-- plain data a driver keeps about its factor.

-- +goose Up
CREATE TABLE user_auth_factors (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE,
    type text NOT NULL CHECK (type ~ '^[a-z][a-z0-9_]{1,39}$'),
    name text NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'active')),
    secret_encrypted bytea,
    credential jsonb NOT NULL DEFAULT '{}',
    lookup_hash bytea,
    last_counter bigint,
    is_preferred boolean NOT NULL DEFAULT false,
    confirmed_at timestamptz,
    last_used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX user_auth_factors_user_id_status_index ON user_auth_factors (user_id, status);
CREATE UNIQUE INDEX user_auth_factors_preferred_key ON user_auth_factors (user_id) WHERE is_preferred;
CREATE UNIQUE INDEX user_auth_factors_recovery_code_key ON user_auth_factors (user_id)
    WHERE type = 'recovery_code' AND status = 'active';
CREATE UNIQUE INDEX user_auth_factors_lookup_hash_key ON user_auth_factors (lookup_hash)
    WHERE lookup_hash IS NOT NULL;

-- +goose Down
DROP TABLE user_auth_factors;
