-- migrate: phase=expand
-- A user's own memberships span every workspace they belong to, so the
-- list and the lookup of one of them run before any workspace is set,
-- through security definer functions owned by the migration role. They
-- return only the user's own rows, with how each membership began and
-- the workspace's role-independent state, and pin search_path so nobody
-- can swap in their own tables.

-- +goose Up
CREATE FUNCTION auth_list_user_memberships(member_user_id uuid)
    RETURNS TABLE (
        member_id uuid, workspace_id uuid, workspace_name text, workspace_slug citext, role text, status text,
        joined_via text, last_accessed_at timestamptz, created_at timestamptz, deleted_at timestamptz,
        purge_after timestamptz
    )
    LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog, public
    AS $$
        SELECT m.id, w.id, w.name, w.slug, m.role, m.status, m.joined_via, m.last_accessed_at, m.created_at,
            w.deleted_at, w.purge_after
        FROM workspace_members m
        JOIN workspaces w ON w.id = m.workspace_id
        WHERE m.user_id = member_user_id
        ORDER BY m.last_accessed_at DESC NULLS LAST, m.created_at
    $$;

-- +goose Down
DROP FUNCTION auth_list_user_memberships(uuid);
