-- migrate: phase=expand
-- An import can be undone for a week after it finishes. While it runs,
-- each record it touches leaves one change: whether the import created
-- it, the fields it changed with their values before and after, and the
-- tags it added. Undoing reads those changes back in a background job
-- that counts as it goes: records removed, records put back, and records
-- kept because someone has worked on them since, each kept one with why.
-- The uploaded file is emptied a month after the import finishes; the
-- import and its counts stay. The nightly sweep finds what to expire
-- across workspaces through a security definer function, as row level
-- security hides other workspaces from the application role.

-- +goose Up
CREATE TABLE import_changes (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    workspace_id uuid NOT NULL REFERENCES workspaces ON DELETE CASCADE,
    import_id uuid NOT NULL REFERENCES imports ON DELETE CASCADE,
    record_id uuid NOT NULL,
    created boolean NOT NULL DEFAULT false,
    before jsonb NOT NULL DEFAULT '{}',
    after jsonb NOT NULL DEFAULT '{}',
    added_tag_ids uuid[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX import_changes_import_id_id_index ON import_changes (import_id, id);
CREATE INDEX import_changes_workspace_id_index ON import_changes (workspace_id);

ALTER TABLE import_changes ENABLE ROW LEVEL SECURITY;
CREATE POLICY import_changes_tenant ON import_changes USING (workspace_id = ANY (dimmi_visible_workspaces()));

ALTER TABLE imports
    ADD COLUMN rollback_status text CHECK (rollback_status IN ('queued', 'running', 'completed', 'failed')),
    ADD COLUMN rollback_total integer NOT NULL DEFAULT 0,
    ADD COLUMN rollback_processed integer NOT NULL DEFAULT 0,
    ADD COLUMN rollback_removed integer NOT NULL DEFAULT 0,
    ADD COLUMN rollback_reverted integer NOT NULL DEFAULT 0,
    ADD COLUMN rollback_kept integer NOT NULL DEFAULT 0,
    ADD COLUMN rollback_kept_records jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN rollback_error text,
    ADD COLUMN rollback_by_member_id uuid REFERENCES workspace_members ON DELETE SET NULL,
    ADD COLUMN rollback_requested_at timestamptz,
    ADD COLUMN rollback_finished_at timestamptz,
    ADD COLUMN file_expired_at timestamptz;

CREATE FUNCTION imports_to_expire(expire_before timestamptz, max_rows integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.id FROM imports i
        WHERE i.file_expired_at IS NULL AND i.finished_at <= expire_before
        ORDER BY i.finished_at, i.id
        LIMIT max_rows
    $$;

CREATE FUNCTION import_changes_to_prune(prune_before timestamptz, max_rows integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT i.workspace_id, i.id FROM imports i
        WHERE i.finished_at <= prune_before
          AND (i.rollback_status IS NULL OR i.rollback_status IN ('completed', 'failed'))
          AND EXISTS (SELECT 1 FROM import_changes c WHERE c.import_id = i.id)
        ORDER BY i.finished_at, i.id
        LIMIT max_rows
    $$;

-- +goose Down
DROP FUNCTION import_changes_to_prune(timestamptz, integer);
DROP FUNCTION imports_to_expire(timestamptz, integer);
ALTER TABLE imports
    DROP COLUMN rollback_status, DROP COLUMN rollback_total, DROP COLUMN rollback_processed,
    DROP COLUMN rollback_removed, DROP COLUMN rollback_reverted, DROP COLUMN rollback_kept,
    DROP COLUMN rollback_kept_records, DROP COLUMN rollback_error, DROP COLUMN rollback_by_member_id,
    DROP COLUMN rollback_requested_at, DROP COLUMN rollback_finished_at, DROP COLUMN file_expired_at;
DROP TABLE import_changes;
