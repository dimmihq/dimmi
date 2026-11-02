-- migrate: phase=expand
-- A member is one user's place in one workspace. The role is only a key:
-- what a role may do lives in code, so roles never appear in policies.
-- Members are deactivated, never deleted, so their history stays
-- attributed.

-- +goose Up
CREATE TABLE workspace_members (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users,
    role text NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    availability text NOT NULL DEFAULT 'online' CHECK (availability IN ('online', 'away')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'deactivated')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT workspace_members_workspace_id_user_id_key UNIQUE (workspace_id, user_id)
);

ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY workspace_members_tenant ON workspace_members
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE workspace_members;
