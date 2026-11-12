-- migrate: phase=expand
-- A scan can be lost: the job that carried it was dropped, or the worker
-- stopped before it ran. The sweep that queues the scans again has to find
-- waiting files in every workspace, while row level security hides all but
-- one. The lookup runs as a security definer function owned by the
-- migration role and returns only the ids the sweep needs.

-- +goose Up
CREATE FUNCTION attachments_pending_scan(cutoff timestamptz, row_limit integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT a.workspace_id, a.id
        FROM attachments a
        WHERE a.scan_status = 'pending' AND (a.uploaded_at IS NOT NULL OR a.message_id IS NOT NULL)
          AND a.updated_at <= cutoff
        ORDER BY a.updated_at
        LIMIT row_limit
    $$;

-- +goose Down
DROP FUNCTION attachments_pending_scan(timestamptz, integer);
