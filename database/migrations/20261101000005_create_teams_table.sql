-- migrate: phase=expand
-- Teams group members for routing. Deleting a team first hides it and
-- frees its name at once, then purges it after its conversations are
-- unassigned, so the unique name only counts teams still alive.
-- team_members carries its workspace like every tenant row, which lets
-- row level security cover it without a join.

-- +goose Up
CREATE TABLE teams (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX teams_workspace_id_name_key ON teams (workspace_id, lower(name)) WHERE deleted_at IS NULL;

CREATE TABLE team_members (
    team_id uuid NOT NULL REFERENCES teams ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    workspace_member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, workspace_member_id)
);

CREATE INDEX team_members_workspace_id_workspace_member_id_index
    ON team_members (workspace_id, workspace_member_id);

ALTER TABLE teams ENABLE ROW LEVEL SECURITY;
CREATE POLICY teams_tenant ON teams USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE team_members ENABLE ROW LEVEL SECURITY;
CREATE POLICY team_members_tenant ON team_members USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE team_members;
DROP TABLE teams;
