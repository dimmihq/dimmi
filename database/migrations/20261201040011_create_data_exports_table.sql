-- migrate: phase=expand
-- A file of conversations or customers a member asked for: a transcript,
-- a spreadsheet or the data itself. The selection says which rows, read
-- again when the file is built so only what the member may still see is
-- written. The file lives on a disk until the workspace's export period
-- runs out. The hourly sweep finds expired files across workspaces
-- through a security definer function, since row level security hides
-- them until a workspace is set.

-- +goose Up
CREATE TABLE data_exports (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    requested_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    kind text NOT NULL CHECK (kind IN ('conversations', 'contacts')),
    format text NOT NULL CHECK (format IN ('txt', 'pdf', 'csv', 'json')),
    selection jsonb NOT NULL,
    include_notes boolean NOT NULL DEFAULT false,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'ready', 'failed', 'expired')),
    row_count bigint,
    warnings jsonb NOT NULL DEFAULT '[]',
    disk text,
    path text,
    size_bytes bigint,
    expires_at timestamptz,
    error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX data_exports_member_index ON data_exports (workspace_id, requested_by_member_id, created_at DESC);
CREATE INDEX data_exports_expires_at_index ON data_exports (expires_at) WHERE status = 'ready';

ALTER TABLE data_exports ENABLE ROW LEVEL SECURITY;
CREATE POLICY data_exports_tenant ON data_exports USING (workspace_id = ANY (dimmi_visible_workspaces()));

CREATE FUNCTION data_exports_expired(expired_at timestamptz, max_rows integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT e.workspace_id, e.id FROM data_exports e
        WHERE e.status = 'ready' AND e.expires_at <= expired_at
        ORDER BY e.expires_at, e.id
        LIMIT max_rows
    $$;

-- +goose Down
DROP FUNCTION data_exports_expired(timestamptz, integer);
DROP TABLE data_exports;
