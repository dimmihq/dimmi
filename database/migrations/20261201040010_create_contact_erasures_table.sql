-- migrate: phase=expand
-- A request to erase one customer. The contact is hidden at once and
-- wiped when the grace period ends, unless the request is canceled or a
-- retention hold keeps it. The contact id carries no foreign key, since
-- the row outlives the contact it names. Identities are kept encrypted
-- while the request can still be canceled, and wiped when it completes.
-- One open request per contact.
--
-- Erased conversations stay as shells pointed at the workspace's
-- placeholder contact, which no list shows; the next migration keeps it
-- to one per workspace.

-- +goose Up
CREATE TABLE contact_erasures (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL,
    status text NOT NULL CHECK (status IN ('pending', 'pending_hold', 'completed', 'canceled')),
    requested_by_member_id uuid,
    scheduled_for timestamptz NOT NULL,
    completed_at timestamptz,
    counts jsonb NOT NULL DEFAULT '{}',
    identities_stash bytea,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX contact_erasures_open_key ON contact_erasures (workspace_id, contact_id)
    WHERE status IN ('pending', 'pending_hold');
CREATE INDEX contact_erasures_workspace_id_created_at_index ON contact_erasures (workspace_id, created_at DESC);

ALTER TABLE contact_erasures ENABLE ROW LEVEL SECURITY;
CREATE POLICY contact_erasures_tenant ON contact_erasures USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE contacts ADD COLUMN placeholder boolean NOT NULL DEFAULT false;

-- +goose Down
ALTER TABLE contacts DROP COLUMN placeholder;
DROP TABLE contact_erasures;
