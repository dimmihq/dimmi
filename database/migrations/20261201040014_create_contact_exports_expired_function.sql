-- migrate: phase=expand
-- The hourly sweep finds expired customer exports across every
-- workspace, which row level security hides from the application role
-- until a workspace is set, so it reads them through a security definer
-- function owned by the migration role. It returns only which export of
-- which workspace; the sweep then works on each inside its workspace.

-- +goose Up
CREATE FUNCTION contact_exports_expired(expired_at timestamptz, max_rows integer)
    RETURNS TABLE (workspace_id uuid, id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT e.workspace_id, e.id FROM contact_exports e
        WHERE e.status = 'ready' AND e.expires_at <= expired_at
        ORDER BY e.expires_at, e.id
        LIMIT max_rows
    $$;

-- +goose Down
DROP FUNCTION contact_exports_expired(timestamptz, integer);
