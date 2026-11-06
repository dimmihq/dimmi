-- migrate: phase=expand
-- A provider connection: one way the workspace's agent can reach a
-- model. Secrets are sealed with the workspace's key and are
-- write-only; the row remembers status, models and provider facts.

-- +goose Up
CREATE TABLE ai_provider_keys (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL,
    provider text NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_]{1,39}$'),
    base_url text,
    region text,
    auth_type text NOT NULL DEFAULT 'api_key'
        CHECK (auth_type IN ('api_key', 'aws_iam', 'bedrock_api_key', 'none', 'managed', 'oauth')),
    credentials_encrypted bytea,
    key_hint text,
    status text NOT NULL DEFAULT 'unverified' CHECK (status IN ('unverified', 'valid', 'invalid', 'error')),
    status_reason text,
    last_error text,
    last_validated_at timestamptz,
    last_error_at timestamptz,
    last_used_at timestamptz,
    models jsonb NOT NULL DEFAULT '[]',
    provider_info jsonb NOT NULL DEFAULT '{}',
    config jsonb NOT NULL DEFAULT '{}',
    source text NOT NULL DEFAULT 'dashboard' CHECK (source IN ('dashboard', 'api', 'env', 'system')),
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ai_provider_keys_workspace_id_lower_name_key
    ON ai_provider_keys (workspace_id, lower(name)) WHERE deleted_at IS NULL;
CREATE INDEX ai_provider_keys_workspace_id_provider_index
    ON ai_provider_keys (workspace_id, provider) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE ai_provider_keys;
