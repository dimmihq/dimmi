-- migrate: phase=expand
-- A conversation waiting for an assignee must not wait for ever because
-- a trigger was missed, so a round every minute looks for waiting
-- conversations across every workspace. Row level security hides other
-- workspaces from the application role until one is set, so the round
-- reads only which workspaces have any, through a security definer
-- function owned by the migration role.

-- +goose Up
CREATE FUNCTION assignment_waiting_workspaces(max_rows integer)
    RETURNS TABLE (workspace_id uuid)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT DISTINCT c.workspace_id
        FROM conversations c
        WHERE c.assignment_wait_reason IS NOT NULL
        ORDER BY c.workspace_id
        LIMIT max_rows
    $$;

-- +goose Down
DROP FUNCTION assignment_waiting_workspaces(integer);
