-- migrate: phase=expand
-- A member link says which chat app user is which teammate, so their
-- messages in a room count as the team's replies and never as a
-- customer's. One user is one teammate per connection and one teammate
-- one user, so an answer is never filed under the wrong person. A link
-- the provider proved by sign-in wins over one matched by email.

-- +goose Up
CREATE TABLE channel_member_links (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    channel_connection_id uuid NOT NULL REFERENCES channel_connections ON DELETE CASCADE,
    workspace_member_id uuid NOT NULL REFERENCES workspace_members ON DELETE CASCADE,
    external_user_id text NOT NULL,
    source text NOT NULL CHECK (source IN ('email_match', 'manual', 'oauth')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX channel_member_links_channel_connection_id_external_user_id_key
    ON channel_member_links (channel_connection_id, external_user_id);
CREATE UNIQUE INDEX channel_member_links_channel_connection_id_workspace_member_id_key
    ON channel_member_links (channel_connection_id, workspace_member_id);
CREATE INDEX channel_member_links_workspace_id_workspace_member_id_index
    ON channel_member_links (workspace_id, workspace_member_id);

ALTER TABLE channel_member_links ENABLE ROW LEVEL SECURITY;
CREATE POLICY channel_member_links_tenant ON channel_member_links
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE channel_member_links;
