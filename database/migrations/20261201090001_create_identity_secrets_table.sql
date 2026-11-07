-- migrate: phase=expand
-- An identity secret is the key a host app signs its customers' identity
-- tokens with. A shared secret is sealed and shown only once; for RS256
-- and EdDSA the host keeps the private key and only the public key is
-- stored. Revoked rows stay, so a token signed with one gets a plain
-- answer instead of "unknown key". The secret that verified a widget
-- session is remembered on it, so revoking a secret can end its sessions.

-- +goose Up
CREATE TABLE identity_secrets (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL,
    algorithm text NOT NULL CHECK (algorithm IN ('HS256', 'RS256', 'EdDSA')),
    secret_encrypted bytea,
    public_key_jwk jsonb,
    fingerprint text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'retiring', 'revoked')),
    retires_at timestamptz,
    revoked_at timestamptz,
    revoked_reason text CHECK (revoked_reason IN ('rotated', 'compromised', 'manual', 'retired')),
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    last_used_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK ((algorithm = 'HS256') = (secret_encrypted IS NOT NULL)),
    CHECK ((algorithm <> 'HS256') = (public_key_jwk IS NOT NULL)),
    CHECK ((status = 'retiring') = (retires_at IS NOT NULL)),
    CHECK ((status = 'revoked') = (revoked_at IS NOT NULL))
);

CREATE INDEX identity_secrets_workspace_id_status_index ON identity_secrets (workspace_id, status);

ALTER TABLE identity_secrets ENABLE ROW LEVEL SECURITY;
CREATE POLICY identity_secrets_tenant ON identity_secrets USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE widget_sessions ADD CONSTRAINT widget_sessions_identity_secret_id_fkey
    FOREIGN KEY (identity_secret_id) REFERENCES identity_secrets ON DELETE SET NULL;

-- +goose Down
ALTER TABLE widget_sessions DROP CONSTRAINT widget_sessions_identity_secret_id_fkey;
DROP TABLE identity_secrets;
