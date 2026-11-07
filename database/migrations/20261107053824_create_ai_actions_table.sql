-- migrate: phase=expand
-- One tool the agent may call: the working copy an admin edits. The
-- agent only ever runs a published version; editing the working copy
-- changes nothing live until it is published again. Who must agree
-- before a write runs is the admin's choice per action: the customer,
-- a teammate, an admin, or nobody.

-- +goose Up
CREATE TABLE ai_actions (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    connection_id uuid NOT NULL REFERENCES ai_action_connections ON DELETE RESTRICT,
    name text NOT NULL CHECK (name ~ '^[a-z][a-z0-9_]{2,47}$'),
    title text NOT NULL,
    description text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('http', 'mcp_tool')),
    side_effect text NOT NULL CHECK (side_effect IN ('read', 'write')),
    confirmation text NOT NULL DEFAULT 'none' CHECK (confirmation IN ('none', 'customer')),
    approval text NOT NULL DEFAULT 'none' CHECK (approval IN ('none', 'teammate', 'admin')),
    approval_timeout_minutes integer NOT NULL DEFAULT 60 CHECK (approval_timeout_minutes BETWEEN 1 AND 10080),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'disabled')),
    published_version_id uuid,
    method text CHECK (method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE')),
    path_template text,
    query jsonb NOT NULL DEFAULT '{}',
    headers jsonb NOT NULL DEFAULT '{}',
    body_template jsonb,
    body_encoding text NOT NULL DEFAULT 'json' CHECK (body_encoding IN ('json', 'form')),
    mcp_tool_name text,
    bound_arguments jsonb NOT NULL DEFAULT '{}',
    input_schema jsonb NOT NULL DEFAULT '{"type":"object","properties":{},"additionalProperties":false}',
    response_fields jsonb NOT NULL DEFAULT '[]',
    error_message_path text,
    response_max_bytes integer NOT NULL DEFAULT 65536 CHECK (response_max_bytes BETWEEN 1024 AND 262144),
    timeout_ms integer CHECK (timeout_ms BETWEEN 1000 AND 15000),
    channels text[] NOT NULL DEFAULT '{widget}',
    spec_hash text NOT NULL DEFAULT '',
    source text NOT NULL DEFAULT 'dashboard' CHECK (source IN ('dashboard', 'api', 'mcp_sync')),
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    updated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ai_actions_reads_run_inline CHECK (side_effect = 'write' OR (confirmation = 'none' AND approval = 'none'))
);

CREATE UNIQUE INDEX ai_actions_workspace_id_name_key ON ai_actions (workspace_id, name) WHERE deleted_at IS NULL;
CREATE INDEX ai_actions_workspace_id_status_index ON ai_actions (workspace_id, status) WHERE deleted_at IS NULL;
CREATE INDEX ai_actions_workspace_id_connection_id_index ON ai_actions (workspace_id, connection_id);

ALTER TABLE ai_actions ENABLE ROW LEVEL SECURITY;
CREATE POLICY ai_actions_tenant ON ai_actions USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE ai_actions;
