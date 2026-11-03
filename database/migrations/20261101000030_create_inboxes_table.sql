-- migrate: phase=expand
-- An inbox is a routing container: every channel connection and every
-- conversation belongs to one. An inbox is only ever soft deleted, since
-- its conversations keep pointing at it, and its default team is simply
-- forgotten when that team goes.

-- +goose Up
CREATE TABLE inboxes (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    name text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 80),
    default_team_id uuid REFERENCES teams ON DELETE SET NULL,
    settings jsonb NOT NULL DEFAULT '{}',
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX inboxes_workspace_id_index ON inboxes (workspace_id);

ALTER TABLE inboxes ENABLE ROW LEVEL SECURITY;
CREATE POLICY inboxes_tenant ON inboxes USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE inboxes;
