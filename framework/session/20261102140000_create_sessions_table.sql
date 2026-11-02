-- migrate: phase=expand
-- lint:user-scoped
-- A session belongs to the browser that holds its cookie and, once the
-- visitor logs in, to a user. It belongs to no tenant: one login reaches
-- every tenant the user is a member of.
--
-- A signed-in session also records how its user got in: the primary
-- method, every method passed at the last full login, the method of the
-- last confirmation, and the device and country it runs from. None of it
-- can be recovered later, and a visitor's session has none of it, so an
-- auth method only ever sits beside a user. Method names are short
-- lowercase identifiers such as password or totp, and a country is a
-- two-letter ISO 3166 code.

-- +goose Up
CREATE TABLE sessions (
    id uuid PRIMARY KEY,
    token_hash bytea NOT NULL UNIQUE,
    user_id uuid,
    data jsonb NOT NULL DEFAULT '{}',
    flash jsonb NOT NULL DEFAULT '{}',
    ip text,
    user_agent text,
    auth_method text CHECK (auth_method ~ '^[a-z][a-z0-9_]{1,39}$'),
    auth_methods text[] NOT NULL DEFAULT '{}',
    confirmed_method text CHECK (confirmed_method ~ '^[a-z][a-z0-9_]{1,39}$'),
    device_id text,
    device_label text,
    country text CHECK (country ~ '^[A-Z]{2}$'),
    authenticated_at timestamptz,
    confirmed_at timestamptz,
    last_activity_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT sessions_auth_method_signed_in CHECK (auth_method IS NULL OR user_id IS NOT NULL)
);

CREATE INDEX sessions_user_id_last_activity_at_index ON sessions (user_id, last_activity_at DESC);
CREATE INDEX sessions_expires_at_index ON sessions (expires_at);

-- +goose Down
DROP TABLE sessions;
