-- migrate: phase=expand
-- A copy of everything a workspace holds about one customer, built in the
-- background for a privacy request. The zip lives on a disk until it
-- expires a day after it is ready. One export per customer runs at a time.

-- +goose Up
CREATE TABLE contact_exports (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts ON DELETE CASCADE,
    requested_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'ready', 'failed', 'expired')),
    include_notes boolean NOT NULL DEFAULT false,
    disk text,
    path text,
    size_bytes bigint,
    error text,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX contact_exports_running_key ON contact_exports (workspace_id, contact_id)
    WHERE status IN ('pending', 'running');
CREATE INDEX contact_exports_expires_at_index ON contact_exports (expires_at) WHERE status = 'ready';

ALTER TABLE contact_exports ENABLE ROW LEVEL SECURITY;
CREATE POLICY contact_exports_tenant ON contact_exports USING (workspace_id = ANY (dimmi_visible_workspaces()));

-- +goose Down
DROP TABLE contact_exports;
