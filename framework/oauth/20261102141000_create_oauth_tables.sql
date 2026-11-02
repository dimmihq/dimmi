-- migrate: phase=expand
-- lint:user-scoped
-- A grant acts for one user. A third-party grant is bound to one tenant,
-- a first-party grant reaches every tenant the user belongs to, and
-- tokens are looked up before any tenant is known, so these tables are
-- read across tenants by digest.

-- +goose Up
CREATE TABLE oauth_clients (
    id text PRIMARY KEY,
    secret_hash bytea,
    name text NOT NULL,
    redirect_uris text[] NOT NULL,
    scopes text[] NOT NULL DEFAULT '{}',
    grant_types text[] NOT NULL,
    first_party boolean NOT NULL DEFAULT false,
    dynamic boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL
);

CREATE TABLE oauth_authorization_codes (
    id uuid PRIMARY KEY,
    code_hash bytea NOT NULL UNIQUE,
    client_id text NOT NULL,
    user_id uuid NOT NULL,
    tenant_id text,
    scopes text[] NOT NULL DEFAULT '{}',
    redirect_uri text NOT NULL,
    code_challenge text NOT NULL,
    auth_time timestamptz,
    grant_id uuid,
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE TABLE oauth_grants (
    id uuid PRIMARY KEY,
    client_id text NOT NULL,
    user_id uuid NOT NULL,
    tenant_id text,
    first_party boolean NOT NULL DEFAULT false,
    scopes text[] NOT NULL DEFAULT '{}',
    auth_time timestamptz,
    confirmed_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    CONSTRAINT oauth_grants_first_party_tenant_check CHECK (NOT first_party OR tenant_id IS NULL)
);

CREATE INDEX oauth_grants_user_id_index ON oauth_grants (user_id);

CREATE TABLE oauth_tokens (
    id uuid PRIMARY KEY,
    grant_id uuid NOT NULL REFERENCES oauth_grants (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('access', 'refresh')),
    token_hash bytea NOT NULL UNIQUE,
    parent_id uuid,
    scopes text[] NOT NULL DEFAULT '{}',
    expires_at timestamptz NOT NULL,
    used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL
);

CREATE INDEX oauth_tokens_grant_id_index ON oauth_tokens (grant_id);

-- +goose Down
DROP TABLE oauth_tokens;
DROP TABLE oauth_grants;
DROP TABLE oauth_authorization_codes;
DROP TABLE oauth_clients;
