-- migrate: phase=expand
-- Participants are the extra people on a thread beside its contact: an
-- email CC, a chat member who wrote, a second customer. A contact is on
-- a conversation at most once. The row carries its workspace like every
-- tenant row, so row level security covers it without a join.

-- +goose Up
CREATE TABLE conversation_participants (
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    conversation_id uuid NOT NULL REFERENCES conversations ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    role text NOT NULL CHECK (role IN ('cc', 'participant')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (conversation_id, contact_id)
);

CREATE INDEX conversation_participants_workspace_id_contact_id_index
    ON conversation_participants (workspace_id, contact_id);

ALTER TABLE conversation_participants ENABLE ROW LEVEL SECURITY;
CREATE POLICY conversation_participants_tenant ON conversation_participants
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE conversation_participants;
