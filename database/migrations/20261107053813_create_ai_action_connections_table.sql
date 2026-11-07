-- migrate: phase=expand
-- Where and how the agent reaches a workspace's own systems: an HTTP API
-- or a remote MCP server. The secret is sealed with the workspace's key
-- and never read back; variables and default headers are plain values
-- templates may use. Many actions share one connection.

-- +goose Up
CREATE TABLE ai_action_connections (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('http', 'mcp')),
    base_url text NOT NULL,
    auth_type text NOT NULL DEFAULT 'none' CHECK (auth_type ~ '^[a-z][a-z0-9_]{1,39}$'),
    auth_config jsonb NOT NULL DEFAULT '{}',
    credentials_encrypted bytea,
    secret_hint text,
    default_headers jsonb NOT NULL DEFAULT '{}',
    variables jsonb NOT NULL DEFAULT '{}',
    timeout_ms integer NOT NULL DEFAULT 8000 CHECK (timeout_ms BETWEEN 1000 AND 15000),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'error', 'disabled')),
    status_reason text,
    last_error text,
    last_error_at timestamptz,
    last_success_at timestamptz,
    consecutive_auth_failures integer NOT NULL DEFAULT 0,
    mcp_server_info jsonb,
    tools_synced_at timestamptz,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX ai_action_connections_workspace_id_lower_name_key
    ON ai_action_connections (workspace_id, lower(name)) WHERE deleted_at IS NULL;

ALTER TABLE ai_action_connections ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_action_connections_tenant ON ai_action_connections
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE ai_action_connections;
