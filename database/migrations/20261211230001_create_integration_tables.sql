-- migrate: phase=expand
-- A workspace connects each account it holds at another tool once, and
-- every feature that needs that tool reads the same installation: its
-- granted scopes, its sealed tokens and its health. The provider is a
-- registry value, so a new tool is new code and never a migration. One
-- provider account feeds one workspace per app, so a team installed by a
-- second workspace is refused rather than shared across tenants.
-- Teammates may add their own grant on an installation, a token that acts
-- as them; it never outlives them or the installation.

-- +goose Up
CREATE TABLE integration_installations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,39}$'),
    external_account_id text CHECK (char_length(external_account_id) BETWEEN 1 AND 200),
    external_app_id text CHECK (char_length(external_app_id) BETWEEN 1 AND 200),
    name text NOT NULL CHECK (char_length(name) <= 200),
    source text NOT NULL CHECK (source IN ('oauth', 'manual')),
    scopes text[] NOT NULL DEFAULT '{}',
    credentials_encrypted bytea NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'error', 'disconnected')),
    last_error_code text CHECK (char_length(last_error_code) <= 100),
    last_error text CHECK (char_length(last_error) <= 2000),
    metadata jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    installed_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    installed_at timestamptz NOT NULL DEFAULT now(),
    last_checked_at timestamptz,
    disconnected_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX integration_installations_provider_account_key
    ON integration_installations (provider, external_app_id, external_account_id)
    WHERE deleted_at IS NULL AND status <> 'disconnected';
CREATE INDEX integration_installations_workspace_id_provider_status_index
    ON integration_installations (workspace_id, provider, status);
CREATE INDEX integration_installations_installed_by_member_id_index
    ON integration_installations (installed_by_member_id);

ALTER TABLE integration_installations ENABLE ROW LEVEL SECURITY;
CREATE POLICY integration_installations_tenant ON integration_installations
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE TABLE integration_member_grants (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    integration_installation_id uuid NOT NULL REFERENCES integration_installations ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    external_user_id text NOT NULL CHECK (char_length(external_user_id) BETWEEN 1 AND 200),
    scopes text[] NOT NULL DEFAULT '{}',
    credentials_encrypted bytea NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'error')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

CREATE UNIQUE INDEX integration_member_grants_installation_member_key
    ON integration_member_grants (integration_installation_id, member_id);
CREATE UNIQUE INDEX integration_member_grants_installation_user_key
    ON integration_member_grants (integration_installation_id, external_user_id);
CREATE INDEX integration_member_grants_workspace_id_member_id_index
    ON integration_member_grants (workspace_id, member_id);
CREATE INDEX integration_member_grants_member_id_index ON integration_member_grants (member_id);

ALTER TABLE integration_member_grants ENABLE ROW LEVEL SECURITY;
CREATE POLICY integration_member_grants_tenant ON integration_member_grants
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE integration_member_grants;
DROP TABLE integration_installations;
