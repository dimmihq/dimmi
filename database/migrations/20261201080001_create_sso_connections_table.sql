-- migrate: phase=expand
-- lint:org-scoped
-- A company's identity provider, connected once for its organization.
-- The protocol fixes which code path signs people in, so it is a closed
-- set. A connection moves from draft to tested to active, and only an
-- active one may be enforced; a change to its protocol settings sends it
-- back to draft, so a typo never locks everyone out. The provider's
-- secret, a signing key or a client secret, is stored sealed and never
-- read back out. Sessions remember the connection they came through,
-- and forget it when the connection goes. Like organizations, the table
-- has no row level security: code reaches it only through a workspace.

-- +goose Up
CREATE TABLE sso_connections (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    organization_id uuid NOT NULL REFERENCES organizations ON DELETE CASCADE,
    protocol text NOT NULL CHECK (protocol IN ('saml', 'oidc')),
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'tested', 'active', 'disabled')),
    config jsonb NOT NULL DEFAULT '{}',
    secret_encrypted bytea,
    jit_enabled boolean NOT NULL DEFAULT false,
    default_role text NOT NULL DEFAULT 'member' CHECK (default_role IN ('admin', 'member')),
    default_team_ids uuid[] NOT NULL DEFAULT '{}',
    group_role_map jsonb NOT NULL DEFAULT '[]',
    enforced boolean NOT NULL DEFAULT false CHECK (NOT enforced OR status = 'active'),
    tested_at timestamptz,
    last_login_at timestamptz,
    metadata_refreshed_at timestamptz,
    metadata_error text,
    last_error jsonb,
    created_by_user_id uuid REFERENCES users ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sso_connections_organization_id_status_index ON sso_connections (organization_id, status);

ALTER TABLE sessions ADD CONSTRAINT sessions_sso_connection_id_fkey
    FOREIGN KEY (sso_connection_id) REFERENCES sso_connections ON DELETE SET NULL NOT VALID;

-- +goose Down
ALTER TABLE sessions DROP CONSTRAINT sessions_sso_connection_id_fkey;
DROP TABLE sso_connections;
