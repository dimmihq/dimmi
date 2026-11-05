-- migrate: phase=expand
-- A custom role is a named part of what a built-in role may do. A member
-- holding one keeps the built-in role it narrows, which still decides who
-- owns the workspace, and does exactly what the custom role lists. A role
-- someone holds cannot be deleted, since falling back to the built-in one
-- would quietly give them more. An invitation may name the role its
-- invitee starts with; if the role goes first, the invitation's built-in
-- role applies.

-- +goose NO TRANSACTION
-- +goose Up
CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 50),
    description text CHECK (char_length(description) <= 200),
    base_role text NOT NULL CHECK (base_role IN ('admin', 'member')),
    permissions text[] NOT NULL DEFAULT '{}',
    created_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX roles_workspace_id_name_key ON roles (workspace_id, lower(name));

ALTER TABLE roles ENABLE ROW LEVEL SECURITY;
CREATE POLICY roles_tenant ON roles USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE workspace_members ADD COLUMN custom_role_id uuid REFERENCES roles ON DELETE RESTRICT;
ALTER TABLE invitations ADD COLUMN custom_role_id uuid REFERENCES roles ON DELETE SET NULL;

CREATE INDEX CONCURRENTLY IF NOT EXISTS workspace_members_workspace_id_custom_role_id_index
    ON workspace_members (workspace_id, custom_role_id) WHERE custom_role_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS workspace_members_workspace_id_custom_role_id_index;
ALTER TABLE invitations DROP COLUMN custom_role_id;
ALTER TABLE workspace_members DROP COLUMN custom_role_id;
DROP TABLE roles;
