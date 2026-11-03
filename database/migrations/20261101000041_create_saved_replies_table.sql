-- migrate: phase=expand
-- A saved reply is text a teammate inserts instead of typing it again.
-- One without an owner is shared with the whole workspace; one with an
-- owner is that member's own and goes with them. A shortcut names one
-- reply per owner.

-- +goose Up
CREATE TABLE saved_replies (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    owner_member_id uuid REFERENCES workspace_members ON DELETE CASCADE,
    title text NOT NULL,
    body text NOT NULL,
    shortcut citext,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT saved_replies_workspace_id_owner_member_id_shortcut_key UNIQUE (workspace_id, owner_member_id, shortcut)
);

ALTER TABLE saved_replies ENABLE ROW LEVEL SECURITY;
CREATE POLICY saved_replies_tenant ON saved_replies USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE saved_replies;
