-- migrate: phase=expand
-- The logos of a member's workspaces, read before any workspace is
-- chosen, so the workspace switcher shows each one's logo. Like the other
-- lookups it runs as a security definer and only ever answers for the
-- user's own memberships.

-- +goose Up
CREATE FUNCTION auth_user_workspace_logos(member_user_id uuid)
    RETURNS TABLE (workspace_id uuid, logo_path text)
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT w.id, w.logo_path
        FROM workspace_members m
        JOIN workspaces w ON w.id = m.workspace_id
        WHERE m.user_id = member_user_id
    $$;

-- +goose Down
DROP FUNCTION auth_user_workspace_logos(uuid);
