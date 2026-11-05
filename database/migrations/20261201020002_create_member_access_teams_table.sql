-- migrate: phase=expand
-- The teams a member's conversation access names, kept apart from team
-- membership so joining a team never widens what someone sees. A team
-- that goes away leaves every list it was on.

-- +goose Up
CREATE TABLE member_access_teams (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    team_id uuid NOT NULL REFERENCES teams ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (member_id, team_id)
);

CREATE INDEX member_access_teams_workspace_id_team_id_index ON member_access_teams (workspace_id, team_id);

ALTER TABLE member_access_teams ENABLE ROW LEVEL SECURITY;
CREATE POLICY member_access_teams_tenant ON member_access_teams
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE member_access_teams;
