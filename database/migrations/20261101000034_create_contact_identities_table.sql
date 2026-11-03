-- migrate: phase=expand
-- An identity is how one contact appears on one channel: an address, a
-- phone number, a chat user id. One identity belongs to one contact in a
-- workspace, which is what lets a message on any channel find its
-- person. The type is a registry value, so a new channel's identity
-- needs no migration on what may be a very large table.

-- +goose Up
CREATE TABLE contact_identities (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    type text NOT NULL CHECK (type ~ '^[a-z][a-z0-9_]{1,39}$'),
    value text NOT NULL,
    verified boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT contact_identities_workspace_id_type_value_key UNIQUE (workspace_id, type, value)
);

CREATE INDEX contact_identities_workspace_id_contact_id_index ON contact_identities (workspace_id, contact_id);

ALTER TABLE contact_identities ENABLE ROW LEVEL SECURITY;
CREATE POLICY contact_identities_tenant ON contact_identities
    USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE contact_identities;
