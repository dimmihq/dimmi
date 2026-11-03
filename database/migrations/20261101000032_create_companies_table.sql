-- migrate: phase=expand
-- A company is the organization a contact works for, as the host app or
-- the API describes it. The host app's own id for it is unique within
-- the workspace, so the same company sent twice is one row.

-- +goose Up
CREATE TABLE companies (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    external_id text,
    name text NOT NULL,
    domain citext,
    custom_attributes jsonb NOT NULL DEFAULT '{}',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX companies_workspace_id_external_id_key ON companies (workspace_id, external_id)
    WHERE external_id IS NOT NULL;

ALTER TABLE companies ENABLE ROW LEVEL SECURITY;
CREATE POLICY companies_tenant ON companies USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE companies;
