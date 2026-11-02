-- migrate: phase=expand
-- An invitation asks one email address into a workspace with a role.
-- There is at most one pending invitation per address and workspace;
-- inviting again sends the same invitation with a new token. Owners are
-- never invited directly: ownership is handed over by an owner.

-- +goose Up
CREATE TABLE invitations (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    email citext NOT NULL,
    role text NOT NULL CHECK (role IN ('admin', 'member')),
    team_ids uuid[] NOT NULL DEFAULT '{}',
    invited_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    token_hash bytea NOT NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'revoked', 'expired')),
    expires_at timestamptz NOT NULL,
    last_sent_at timestamptz NOT NULL,
    send_count smallint NOT NULL DEFAULT 1,
    accepted_at timestamptz,
    accepted_by_user_id uuid REFERENCES users ON DELETE SET NULL,
    revoked_at timestamptz,
    revoked_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT invitations_token_hash_key UNIQUE (token_hash)
);

CREATE UNIQUE INDEX invitations_workspace_id_email_pending_key ON invitations (workspace_id, email)
    WHERE status = 'pending';
CREATE INDEX invitations_workspace_id_status_created_at_index
    ON invitations (workspace_id, status, created_at DESC);
CREATE INDEX invitations_email_pending_index ON invitations (email) WHERE status = 'pending';

ALTER TABLE invitations ENABLE ROW LEVEL SECURITY;
CREATE POLICY invitations_tenant ON invitations USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE invitations;
