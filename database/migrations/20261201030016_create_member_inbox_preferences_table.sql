-- migrate: phase=expand
-- How each member arranges their inbox: the sidebar, the sort and status
-- chosen per view, and the customer attributes they pin. A preference is
-- the member's own and goes with them; it is never data others read.

-- +goose Up
CREATE TABLE member_inbox_preferences (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    member_id uuid PRIMARY KEY REFERENCES workspace_members ON DELETE CASCADE,
    sidebar jsonb NOT NULL DEFAULT '{}',
    views jsonb NOT NULL DEFAULT '{}',
    pinned_attributes jsonb NOT NULL DEFAULT '[]',
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX member_inbox_preferences_workspace_id_index ON member_inbox_preferences (workspace_id);

ALTER TABLE member_inbox_preferences ENABLE ROW LEVEL SECURITY;
CREATE POLICY member_inbox_preferences_tenant ON member_inbox_preferences
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE member_inbox_preferences;
