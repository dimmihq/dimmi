-- migrate: phase=expand
-- A saved audience is a named rule the workspace writes once and uses
-- wherever the product asks "who": workflows, and later the messenger,
-- help articles and the agent. Membership is never stored; it is worked
-- out from the rule each time, so an audience always means the people
-- who match it now. lock_version stops two editors from overwriting each
-- other.

-- +goose Up
CREATE TABLE audiences (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    description text CHECK (char_length(description) <= 500),
    rules jsonb NOT NULL CHECK (octet_length(rules::text) <= 8192),
    rules_schema smallint NOT NULL DEFAULT 1,
    last_estimate jsonb,
    lock_version integer NOT NULL DEFAULT 1,
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    updated_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX audiences_workspace_id_name_key ON audiences (workspace_id, lower(name)) WHERE deleted_at IS NULL;

ALTER TABLE audiences ENABLE ROW LEVEL SECURITY;
CREATE POLICY audiences_tenant ON audiences USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE audiences;
