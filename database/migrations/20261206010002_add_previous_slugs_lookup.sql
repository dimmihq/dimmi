-- migrate: phase=expand
-- The slugs a member's workspaces gave up and still hold, read before any
-- workspace is chosen, so a link that names an old slug still finds its
-- workspace. Like the other lookups it runs as a security definer and
-- only ever answers for the user's own memberships.

-- +goose Up
CREATE FUNCTION auth_user_previous_slugs(member_user_id uuid)
    RETURNS TABLE (workspace_id uuid, previous_slugs jsonb)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT w.id, coalesce(w.settings -> 'previous_slugs', '[]'::jsonb)
        FROM workspace_members m
        JOIN workspaces w ON w.id = m.workspace_id
        WHERE m.user_id = member_user_id
    $$;

-- +goose Down
DROP FUNCTION auth_user_previous_slugs(uuid);
