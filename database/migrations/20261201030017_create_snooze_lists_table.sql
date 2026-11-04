-- migrate: phase=expand
-- The snooze choices a workspace offers, and those a team offers instead
-- for its own conversations. A missing row inherits: a team falls back to
-- the workspace's list, the workspace to the built-in presets.

-- +goose Up
CREATE TABLE snooze_lists (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    team_id uuid REFERENCES teams ON DELETE CASCADE,
    options jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX snooze_lists_workspace_id_team_id_key ON snooze_lists (workspace_id, team_id) NULLS NOT DISTINCT;

ALTER TABLE snooze_lists ENABLE ROW LEVEL SECURITY;
CREATE POLICY snooze_lists_tenant ON snooze_lists USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE snooze_lists;
